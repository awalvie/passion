package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

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
	run.Status = db.RunStatusCompleted
	run.CompletedAt = &now
	if err := s.store.DB.Save(&run).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	redirect := "/dashboard"
	if run.IsOpen {
		redirect = "/runs/" + chi.URLParam(r, "runID") + "/summary"
	}
	w.Header().Set("HX-Redirect", redirect)
	w.WriteHeader(http.StatusOK)
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
	}

	if run.IsOpen {
		var openExercises []db.Exercise
		s.store.DB.
			Where("session_run_id = ? AND owner_id = ? AND parent_exercise_id IS NULL", run.ID, ownerID).
			Order("order_index asc").
			Find(&openExercises)
		sa := pages.RunSummaryActivity{Name: "Exercises"}
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
		for _, act := range ss.SessionTemplate.Activities {
			sa := pages.RunSummaryActivity{Name: act.Name}
			for _, ex := range act.Exercises {
				if ex.ParentExerciseID != nil && *ex.ParentExerciseID != 0 {
					continue
				}
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
