package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

func (s *Server) handleRunStop(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, runID).First(&run).Error; err != nil {
		s.notFound(w)
		return
	}
	if run.Status != db.RunStatusRunning {
		http.Error(w, "run is not in progress", http.StatusBadRequest)
		return
	}

	now := time.Now()

	// Finishing early leaves the remaining steps with no completion row, which the
	// summary and history then render as "pending" — neither done nor skipped, and
	// absent from the counts. Mark them skipped so the run is fully accounted for.
	if err := s.skipRemainingSteps(runID, ownerID, run, now); err != nil {
		s.serverError(w, r, err)
		return
	}

	run.Status = db.RunStatusCompleted
	run.CompletedAt = &now
	if err := s.store.DB.Save(&run).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	// Ensure an empty journal exists so the run always has one.
	if existing, _ := db.GetSessionJournalByRunID(s.store.DB, ownerID, runID); existing == nil {
		j := db.SessionJournal{OwnerID: ownerID, RunID: &runID}
		if err := db.UpsertSessionJournal(s.store.DB, &j); err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	w.Header().Set("HX-Redirect", "/runs/"+chi.URLParam(r, "runID")+"/summary")
	w.WriteHeader(http.StatusOK)
}

// skipRemainingSteps records a skipped completion for every step of the run that
// has no completion row yet. Idempotent: existing rows are left untouched, so a
// double-submit cannot overwrite a real "completed" with a "skipped".
func (s *Server) skipRemainingSteps(runID, ownerID uint, run db.SessionRun, at time.Time) error {
	var steps []pages.RunStep
	if run.IsOpen {
		steps = s.loadOpenSteps(runID)
	} else {
		ss, err := db.GetScheduledSessionWithTemplate(s.store.DB, ownerID, run.ScheduledSessionID)
		if err != nil {
			return err
		}
		ss = s.useRunOwnedGraph(ss, run, ownerID)
		steps = s.buildRunSteps(ss, runID, ownerID)
	}
	if len(steps) == 0 {
		return nil
	}

	return s.store.DB.Transaction(func(tx *gorm.DB) error {
		var existing []db.RunExerciseCompletion
		if err := tx.Where("owner_id = ? AND run_id = ?", ownerID, runID).Find(&existing).Error; err != nil {
			return err
		}
		done := make(map[uint]struct{}, len(existing))
		for _, c := range existing {
			done[c.ExerciseID] = struct{}{}
		}
		for _, st := range steps {
			if _, ok := done[st.ExerciseID]; ok {
				continue
			}
			row := &db.RunExerciseCompletion{
				OwnerID:     ownerID,
				RunID:       runID,
				ExerciseID:  st.ExerciseID,
				Status:      db.RunStatusSkipped,
				CompletedAt: at,
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
			done[st.ExerciseID] = struct{}{}
		}
		return nil
	})
}

func (s *Server) handleRunDelete(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, runID).First(&run).Error; err != nil {
		s.notFound(w)
		return
	}

	// Cascade delete related data
	s.store.DB.Where("owner_id = ? AND run_id = ?", ownerID, run.ID).Delete(&db.RunExerciseChoice{})
	s.store.DB.Where("owner_id = ? AND run_id = ?", ownerID, run.ID).Delete(&db.RunExerciseCompletion{})
	s.store.DB.Delete(&run)

	// If trial run, also delete the associated scheduled session
	if run.IsTrial {
		s.store.DB.Where("owner_id = ? AND id = ? AND is_trial = ?", ownerID, run.ScheduledSessionID, true).Delete(&db.ScheduledSession{})
	}

	returnTo := strings.TrimSpace(r.FormValue("return_to"))
	if returnTo == "" || !strings.HasPrefix(returnTo, "/") {
		returnTo = "/history"
	}
	w.Header().Set("HX-Redirect", returnTo)
	w.WriteHeader(http.StatusOK)
}

// summaryTickViews loads the logged climbs for one climbing exercise in a run,
// as read-only views for the run summary. Returns nil if none or on error —
// the summary degrades gracefully rather than failing.
func (s *Server) summaryTickViews(ownerID, runID, exerciseID uint) []pages.ClimbingTickView {
	ticks, err := db.ListClimbingTicksByExercise(s.store.DB, ownerID, runID, exerciseID)
	if err != nil || len(ticks) == 0 {
		return nil
	}
	return ticksToViews(ticks)
}

func (s *Server) handleRunSummary(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, runID).First(&run).Error; err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ss, err := db.GetScheduledSessionWithTemplate(s.store.DB, ownerID, run.ScheduledSessionID)
	if err != nil {
		s.dbError(w, r, err)
		return
	}

	var completions []db.RunExerciseCompletion
	s.store.DB.Where("run_id = ? AND owner_id = ?", runID, ownerID).Find(&completions)
	compByID := make(map[uint]db.RunExerciseCompletion, len(completions))
	for _, c := range completions {
		compByID[c.ExerciseID] = c
	}

	name := ss.SessionTemplate.Name
	if run.CustomName != "" {
		name = run.CustomName
	}

	durationLabel := ""
	if run.CompletedAt != nil {
		durationLabel = formatDuration(run.CompletedAt.Sub(run.StartedAt))
	}

	view := pages.RunSummaryView{
		RunID:         runID,
		TemplateName:  name,
		Color:         normalizeTemplateColor(ss.SessionTemplate.Color),
		DateLabel:     run.StartedAt.Format("Mon, Jan 2, 2006"),
		DurationLabel: durationLabel,
		IsOpen:        run.IsOpen,
		Completed:     run.Status == db.RunStatusCompleted,
	}

	if run.IsOpen {
		var openExercises []db.Exercise
		s.store.DB.
			Where("session_run_id = ? AND owner_id = ? AND parent_exercise_id IS NULL", run.ID, ownerID).
			Order("order_index asc").
			Find(&openExercises)
		// Unnamed: the summary page already renders an "Exercises" section heading,
		// so a group label here would duplicate it.
		sa := pages.RunSummaryActivity{Name: ""}
		for _, ex := range openExercises {
			se := pages.RunSummaryExercise{
				Name:            ex.Name,
				Kind:            ex.Kind,
				Sets:            ex.Sets,
				Reps:            ex.Reps,
				WeightKg:        ex.WeightKg,
				RepSeconds:      ex.RepSeconds,
				SessionDuration: ex.SessionDurationSeconds,
				Status:          "pending",
			}
			if c, ok := compByID[ex.ID]; ok {
				se.Status = c.Status
				se.ElapsedSeconds = c.ElapsedSeconds
				se.Notes = c.RunNotes
			}
			if ex.Kind == "climbing" {
				se.Ticks = s.summaryTickViews(ownerID, run.ID, ex.ID)
			}
			switch se.Status {
			case db.RunStatusCompleted:
				view.CompletedCount++
			case db.RunStatusSkipped:
				view.SkippedCount++
			}
			view.TotalCount++
			sa.Exercises = append(sa.Exercises, se)
		}
		if len(sa.Exercises) > 0 {
			view.Activities = append(view.Activities, sa)
		}
	} else {
		// An exercise_catalog is only a picker: the run records the completion and any
		// climbing ticks against the *chosen child*, not the menu. Rendering only
		// top-level rows therefore showed the menu as forever "pending" and hid the
		// climbing entirely, so resolve each menu to what was actually picked.
		var choices []db.RunExerciseChoice
		s.store.DB.Where("owner_id = ? AND run_id = ?", ownerID, runID).Find(&choices)
		chosenByParent := map[uint][]uint{}
		for _, c := range choices {
			chosenByParent[c.ParentExerciseID] = append(chosenByParent[c.ParentExerciseID], c.ChosenExerciseID)
		}

		for _, act := range ss.SessionTemplate.Activities {
			byID := map[uint]db.Exercise{}
			for _, ex := range act.Exercises {
				byID[ex.ID] = ex
			}

			sa := pages.RunSummaryActivity{Name: act.Name}
			addRow := func(ex db.Exercise) {
				se := pages.RunSummaryExercise{
					Name:            ex.Name,
					Kind:            ex.Kind,
					Sets:            ex.Sets,
					Reps:            ex.Reps,
					WeightKg:        ex.WeightKg,
					RepSeconds:      ex.RepSeconds,
					SessionDuration: ex.SessionDurationSeconds,
					Status:          "pending",
				}
				if c, ok := compByID[ex.ID]; ok {
					se.Status = c.Status
					se.ElapsedSeconds = c.ElapsedSeconds
					se.Notes = c.RunNotes
				}
				if ex.Kind == "climbing" {
					se.Ticks = s.summaryTickViews(ownerID, run.ID, ex.ID)
				}
				switch se.Status {
				case db.RunStatusCompleted:
					view.CompletedCount++
				case db.RunStatusSkipped:
					view.SkippedCount++
				}
				view.TotalCount++
				sa.Exercises = append(sa.Exercises, se)
			}

			for _, ex := range act.Exercises {
				if ex.ParentExerciseID != nil && *ex.ParentExerciseID != 0 {
					continue
				}
				if ex.Kind == "exercise_catalog" {
					if chosen := chosenByParent[ex.ID]; len(chosen) > 0 {
						for _, cid := range chosen {
							if child, ok := byID[cid]; ok {
								addRow(child)
							}
						}
						continue
					}
					// Nothing was picked — show the menu itself so it still appears
					// in the counts as skipped rather than vanishing.
				}
				addRow(ex)
			}
			if len(sa.Exercises) > 0 {
				view.Activities = append(view.Activities, sa)
			}
		}
	}

	// Fragment path (HTMX dialog).
	if r.Header.Get("HX-Request") != "" {
		s.pages.RenderFragment(w, "fragments/run_summary", view)
		return
	}

	// Full page — redirect in-progress runs to the run page (open sessions can view summary while running).
	if run.Status != db.RunStatusCompleted && !run.IsOpen {
		http.Redirect(w, r, "/runs/"+chi.URLParam(r, "runID"), http.StatusSeeOther)
		return
	}

	// Load previous runs of the same template via ScheduledSession.SessionTemplateID.
	var scheduledSess db.ScheduledSession
	s.store.DB.Select("session_template_id").Where("id = ?", run.ScheduledSessionID).First(&scheduledSess)

	var prevRuns []db.SessionRun
	if scheduledSess.SessionTemplateID != 0 {
		s.store.DB.
			Joins("JOIN scheduled_sessions ON scheduled_sessions.id = session_runs.scheduled_session_id").
			Where("session_runs.owner_id = ? AND scheduled_sessions.session_template_id = ? "+
				"AND session_runs.status = 'completed' AND session_runs.id != ?",
				ownerID, scheduledSess.SessionTemplateID, run.ID).
			Order("session_runs.started_at DESC").
			Limit(8).
			Find(&prevRuns)
	}

	var prevRunRows []pages.PrevRunRow
	for _, pr := range prevRuns {
		var comps []db.RunExerciseCompletion
		s.store.DB.Where("run_id = ? AND owner_id = ?", pr.ID, ownerID).Find(&comps)
		done := 0
		for _, c := range comps {
			if c.Status == db.RunStatusCompleted {
				done++
			}
		}
		pct := 0
		if view.TotalCount > 0 {
			pct = done * 100 / view.TotalCount
		}
		dur := ""
		if pr.CompletedAt != nil {
			dur = formatDuration(pr.CompletedAt.Sub(pr.StartedAt))
		}
		prevRunRows = append(prevRunRows, pages.PrevRunRow{
			DateLabel:     pr.StartedAt.Format("Jan 2"),
			DoneCount:     done,
			TotalCount:    view.TotalCount,
			DurationLabel: dur,
			Pct:           pct,
		})
	}

	s.pages.RunSummaryPage(w, pages.RunSummaryPageParams{
		Base:     pages.Base{CurrentUserEmail: s.currentUserEmail(r), Title: view.TemplateName + " · Summary"},
		Summary:  view,
		PrevRuns: prevRunRows,
	})
}
