package web

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

// focusDisplayName converts a stored focus key to a capitalised display string.
func focusDisplayName(f string) string {
	switch f {
	case "strength":
		return "Strength"
	case "endurance":
		return "Endurance"
	case "technique":
		return "Technique"
	case "projects":
		return "Projects"
	case "general":
		return "General"
	}
	return ""
}

// locationDisplayName converts a stored location key to a display string.
func locationDisplayName(l string) string {
	switch l {
	case "indoor":
		return "Indoor"
	case "outdoor":
		return "Outdoor"
	}
	return ""
}

// handleTrainingLog serves GET /training-log.
func (s *Server) handleTrainingLog(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}

	// Load all completed non-draft runs, newest first.
	var runs []db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND status = ? AND is_draft = ?", ownerID, db.RunStatusCompleted, false).
		Order("started_at desc").
		Find(&runs).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	// Collect scheduled session IDs so we can load templates.
	ssIDs := make([]uint, 0, len(runs))
	for _, run := range runs {
		ssIDs = append(ssIDs, run.ScheduledSessionID)
	}

	ssMap := map[uint]db.ScheduledSession{}
	if len(ssIDs) > 0 {
		var ssList []db.ScheduledSession
		if err := s.store.DB.
			Preload("SessionTemplate").
			Where("id IN ?", ssIDs).
			Find(&ssList).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
		for _, ss := range ssList {
			ssMap[ss.ID] = ss
		}
	}

	// Bulk-load per-run data for manual runs to avoid N+1 queries.
	manualRunIDs := make([]uint, 0)
	for _, run := range runs {
		if run.IsManual {
			manualRunIDs = append(manualRunIDs, run.ID)
		}
	}
	exerciseCountsByRun, err := db.ListExerciseCountsByRun(s.store.DB, ownerID, manualRunIDs)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	tickSummariesByRun, err := db.ListTickSummariesByRun(s.store.DB, ownerID, manualRunIDs)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// Load all journals for this user; split into run-linked and standalone.
	journals, err := db.ListSessionJournals(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	journalByRun := map[uint]db.SessionJournal{}
	var standaloneJournals []db.SessionJournal
	for _, j := range journals {
		if j.RunID != nil {
			journalByRun[*j.RunID] = j
		} else {
			standaloneJournals = append(standaloneJournals, j)
		}
	}

	// Build entry views for completed runs.
	entries := make([]pages.TrainingLogEntryView, 0, len(runs)+len(standaloneJournals))
	for _, run := range runs {
		ss := ssMap[run.ScheduledSessionID]
		templateName := ss.SessionTemplate.Name
		if run.CustomName != "" {
			templateName = run.CustomName
		}

		var dur string
		if run.CompletedAt != nil {
			dur = formatDuration(run.CompletedAt.Sub(run.StartedAt))
		}

		t := run.StartedAt
		dateLabel := fmt.Sprintf("%s %d%s %d", t.Format("Jan"), t.Day(), daySuffix(t.Day()), t.Year())
		monthGroup := t.Format("January 2006")

		tickSummaryLabel := ""
		exerciseCount := exerciseCountsByRun[run.ID]
		if run.IsManual {
			ts := tickSummariesByRun[run.ID]
			if ts.TotalBoulders > 0 || ts.TotalRoutes > 0 {
				label := ""
				if ts.TotalBoulders > 0 {
					label += fmt.Sprintf("%d boulder", ts.TotalBoulders)
					if ts.TotalBoulders != 1 {
						label += "s"
					}
				}
				if ts.TotalRoutes > 0 {
					if label != "" {
						label += " · "
					}
					label += fmt.Sprintf("%d route", ts.TotalRoutes)
					if ts.TotalRoutes != 1 {
						label += "s"
					}
				}
				if ts.TotalSends > 0 {
					label += fmt.Sprintf(" · %d send", ts.TotalSends)
					if ts.TotalSends != 1 {
						label += "s"
					}
				}
				tickSummaryLabel = label
			}
		}

		entry := pages.TrainingLogEntryView{
			RunID:            run.ID,
			SortTime:         run.StartedAt,
			DateLabel:        dateLabel,
			TemplateName:     templateName,
			Color:            ss.SessionTemplate.Color,
			DurationLabel:    dur,
			MonthGroup:       monthGroup,
			IsManual:         run.IsManual,
			TickSummaryLabel: tickSummaryLabel,
			ExerciseCount:    exerciseCount,
		}

		if j, ok := journalByRun[run.ID]; ok {
			entry.HasJournal = true
			entry.JournalEntryID = j.ID
			entry.SleepScore = j.SleepScore
			entry.Energy = j.Energy
			entry.RPE = j.RPE
			entry.Focus = focusDisplayName(j.Focus)
			entry.Location = locationDisplayName(j.Location)
			entry.WentWellHTML = markdownToHTML(j.WentWell)
			entry.NextFocusHTML = markdownToHTML(j.NextFocus)
		}

		entries = append(entries, entry)
	}

	// Append standalone journal entries.
	for _, j := range standaloneJournals {
		t := j.Date
		if t.IsZero() {
			t = j.CreatedAt
		}
		title := j.Title
		if title == "" {
			title = "Log entry"
		}
		dateLabel := fmt.Sprintf("%s %d%s %d", t.Format("Jan"), t.Day(), daySuffix(t.Day()), t.Year())
		entries = append(entries, pages.TrainingLogEntryView{
			JournalEntryID: j.ID,
			SortTime:       t,
			DateLabel:      dateLabel,
			TemplateName:   title,
			MonthGroup:     t.Format("January 2006"),
			IsStandalone:   true,
			HasJournal:     true,
			SleepScore:     j.SleepScore,
			Energy:         j.Energy,
			RPE:            j.RPE,
			Focus:          focusDisplayName(j.Focus),
			Location:       locationDisplayName(j.Location),
			WentWellHTML:   markdownToHTML(j.WentWell),
			NextFocusHTML:  markdownToHTML(j.NextFocus),
		})
	}

	// Sort all entries newest-first so standalone and run entries interleave correctly.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SortTime.After(entries[j].SortTime)
	})

	// Build adherence block from the most recent training cycle.
	adherence, cycleName := s.buildAdherenceView(ownerID)

	s.pages.TrainingLogPage(w, pages.TrainingLogPageParams{
		Entries:   entries,
		Adherence: adherence,
		CycleName: cycleName,
	})
}

// buildAdherenceView computes planned vs. completed sessions per week for the
// user's most recent training cycle (up to 8 past weeks).
func (s *Server) buildAdherenceView(ownerID uint) ([]pages.AdherenceWeekView, string) {
	// Find most recent cycle.
	var cycle db.TrainingCycle
	err := s.store.DB.
		Preload("WeekdayMappings").
		Where("owner_id = ?", ownerID).
		Order("start_date desc").
		First(&cycle).Error
	if err != nil {
		return nil, ""
	}

	// Build a set of planned weekdays (1=Mon..7=Sun).
	plannedDays := map[int]bool{}
	for _, m := range cycle.WeekdayMappings {
		plannedDays[m.Weekday] = true
	}
	if len(plannedDays) == 0 {
		return nil, cycle.Name
	}

	now := time.Now()
	thisMonday := mondayOfLocalDate(now)

	// Collect completed run dates for this cycle.
	var completedDates []time.Time
	var completedRuns []db.SessionRun
	s.store.DB.
		Joins("JOIN scheduled_sessions ON scheduled_sessions.id = session_runs.scheduled_session_id").
		Where("session_runs.owner_id = ? AND session_runs.status = ? AND scheduled_sessions.training_cycle_id = ? AND session_runs.deleted_at IS NULL",
			ownerID, db.RunStatusCompleted, cycle.ID).
		Find(&completedRuns)

	// Load the scheduled dates for these runs.
	runSSIDs := make([]uint, 0, len(completedRuns))
	for _, r := range completedRuns {
		runSSIDs = append(runSSIDs, r.ScheduledSessionID)
	}
	var compSessions []db.ScheduledSession
	if len(runSSIDs) > 0 {
		s.store.DB.Where("id IN ?", runSSIDs).Find(&compSessions)
	}
	for _, ss := range compSessions {
		completedDates = append(completedDates, ss.ScheduledDate)
	}

	// Group completed dates into a map keyed by monday-of-week.
	completedByWeek := map[string]int{}
	for _, d := range completedDates {
		key := localDateKey(mondayOfLocalDate(d))
		completedByWeek[key]++
	}

	// Walk back up to 8 complete weeks from last Monday.
	weeks := make([]pages.AdherenceWeekView, 0, 8)
	cycleStart := localDate(cycle.StartDate)
	for i := 0; i < 8; i++ {
		monday := thisMonday.AddDate(0, 0, -7*i)
		sunday := monday.AddDate(0, 0, 6)

		// Don't show weeks before the cycle started.
		if sunday.Before(cycleStart) {
			break
		}

		// Count planned days in this week (Mon–Sun), capped by cycle bounds.
		planned := 0
		cycleEnd := cycleStart.AddDate(0, 0, cycle.Weeks*7)
		for day := 0; day < 7; day++ {
			d := monday.AddDate(0, 0, day)
			if d.Before(cycleStart) || !d.Before(cycleEnd) {
				continue
			}
			wd := int(d.Weekday())
			if wd == 0 {
				wd = 7 // Sunday = 7
			}
			if plannedDays[wd] {
				planned++
			}
		}
		if planned == 0 {
			continue
		}

		completed := completedByWeek[localDateKey(monday)]
		if completed > planned {
			completed = planned
		}

		pct := 0
		if planned > 0 {
			pct = (completed * 100) / planned
		}

		weekLabel := fmt.Sprintf("%s – %s", dayWithSuffix(monday), dayWithSuffix(sunday))
		weeks = append(weeks, pages.AdherenceWeekView{
			WeekLabel: weekLabel,
			Planned:   planned,
			Completed: completed,
			Pct:       pct,
			PctLabel:  fmt.Sprintf("%d / %d", completed, planned),
		})
	}

	// Most-recent week first (already in order since we walked back).
	sort.Slice(weeks, func(i, j int) bool { return i < j })

	return weeks, cycle.Name
}

// handleRunJournal serves GET|POST /runs/{runID}/journal.
func (s *Server) handleRunJournal(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}

	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	// Validate ownership of the run.
	var run db.SessionRun
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, runID).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		s.serverError(w, r, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.serveJournalForm(w, r, ownerID, runID, false)
	case http.MethodPost:
		s.saveJournal(w, r, ownerID, runID)
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) serveJournalForm(w http.ResponseWriter, r *http.Request, ownerID, runID uint, saved bool) {
	j, err := db.GetSessionJournalByRunID(s.store.DB, ownerID, runID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	params := pages.JournalFormParams{
		RunID: runID,
		Saved: saved,
	}
	if j != nil {
		params.SleepScore = j.SleepScore
		params.Energy = j.Energy
		params.RPE = j.RPE
		params.Focus = j.Focus
		params.Location = j.Location
		params.WentWell = j.WentWell
		params.NextFocus = j.NextFocus
	}

	s.pages.RenderJournalForm(w, params)
}

func (s *Server) saveJournal(w http.ResponseWriter, r *http.Request, ownerID, runID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	// Look up existing journal (upsert).
	existing, err := db.GetSessionJournalByRunID(s.store.DB, ownerID, runID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	j := db.SessionJournal{
		OwnerID:    ownerID,
		RunID:      &runID,
		SleepScore: formInt(r, "sleep_score"),
		Energy:     formInt(r, "energy"),
		RPE:        formInt(r, "rpe"),
		Focus:      strings.TrimSpace(r.FormValue("focus")),
		Location:   strings.TrimSpace(r.FormValue("location")),
		WentWell:   strings.TrimSpace(r.FormValue("went_well")),
		NextFocus:  strings.TrimSpace(r.FormValue("next_focus")),
	}
	if existing != nil {
		j.Model = existing.Model // preserve ID + timestamps for update
	}

	if err := db.UpsertSessionJournal(s.store.DB, &j); err != nil {
		s.serverError(w, r, err)
		return
	}

	// Return the saved (read-only) fragment so HTMX can swap it in.
	s.serveJournalForm(w, r, ownerID, runID, true)
}

// handleTrainingLogNew serves GET|POST /training-log/new.
// GET: creates a draft SessionRun so exercise/tick HTMX routes are live from page load.
// POST: finalises the draft run, creates a SessionJournal, redirects to /training-log.
func (s *Server) handleTrainingLogNew(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}

	if r.Method == http.MethodPost {
		s.finaliseManualEntry(w, r, ownerID)
		return
	}

	// GET: create a draft run anchored to the open-session system template.
	tpl, err := s.getOrCreateOpenSessionTemplate(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	now := time.Now()
	scheduled := &db.ScheduledSession{
		OwnerID:           ownerID,
		IsTrial:           true,
		ScheduledDate:     localDate(now),
		SessionTemplateID: tpl.ID,
	}
	if err := s.store.DB.Create(scheduled).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	draft, err := db.CreateDraftSessionRun(s.store.DB, ownerID, scheduled.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	libExercises, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	venues, boards, err := s.loadVenuesAndBoards(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.pages.TrainingLogNewPage(w, pages.TrainingLogNewParams{
		DateValue:        now.Format("2006-01-02"),
		DraftRunID:       draft.ID,
		LibraryExercises: libExercises,
		Venues:           venues,
		Boards:           boards,
	})
}

// finaliseManualEntry is called on POST /training-log/new.
// It finalises the draft run and creates a journal entry linked to it.
func (s *Server) finaliseManualEntry(w http.ResponseWriter, r *http.Request, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	dateStr := strings.TrimSpace(r.FormValue("date"))
	date, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		date = time.Now()
	}

	runIDStr := strings.TrimSpace(r.FormValue("draft_run_id"))
	runID64, parseErr := strconv.ParseUint(runIDStr, 10, 64)
	runID := uint(runID64)

	// If we have a valid draft run ID, finalise it.
	if parseErr == nil && runID > 0 {
		customName := strings.TrimSpace(r.FormValue("title"))
		if err := db.FinaliseDraftRun(s.store.DB, ownerID, runID, customName, date); err != nil {
			s.serverError(w, r, err)
			return
		}
	} else {
		// No draft run (shouldn't normally happen, but fall back gracefully).
		runID = 0
	}

	// Parse optional venue and board IDs.
	venueIDStr := strings.TrimSpace(r.FormValue("venue_id"))
	boardIDStr := strings.TrimSpace(r.FormValue("board_id"))
	var venueID, boardID *uint
	if v, err2 := strconv.ParseUint(venueIDStr, 10, 64); err2 == nil && v > 0 {
		vv := uint(v)
		venueID = &vv
	}
	if b, err2 := strconv.ParseUint(boardIDStr, 10, 64); err2 == nil && b > 0 {
		bv := uint(b)
		boardID = &bv
	}

	// Determine location from venue kind if a venue was selected.
	location := strings.TrimSpace(r.FormValue("location"))
	if venueID != nil {
		var venue db.ClimbingVenue
		if err2 := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, *venueID).First(&venue).Error; err2 == nil {
			if venue.Kind == "outdoor" {
				location = "outdoor"
			} else {
				location = "indoor"
			}
		}
	}

	j := db.SessionJournal{
		OwnerID:    ownerID,
		Title:      strings.TrimSpace(r.FormValue("title")),
		Date:       date,
		SleepScore: formInt(r, "sleep_score"),
		Energy:     formInt(r, "energy"),
		RPE:        formInt(r, "rpe"),
		Focus:      strings.TrimSpace(r.FormValue("focus")),
		Location:   location,
		VenueID:    venueID,
		BoardID:    boardID,
		WentWell:   strings.TrimSpace(r.FormValue("went_well")),
		NextFocus:  strings.TrimSpace(r.FormValue("next_focus")),
	}
	if runID > 0 {
		j.RunID = &runID
	}

	if err := db.UpsertSessionJournal(s.store.DB, &j); err != nil {
		s.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/training-log", http.StatusSeeOther)
}

// handleTrainingLogDraftDiscard serves POST /training-log/draft/{runID}/discard.
func (s *Server) handleTrainingLogDraftDiscard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	if err := db.DeleteDraftRun(s.store.DB, ownerID, runID); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/training-log", http.StatusSeeOther)
}

// handleTrainingLogAddExercise serves POST /training-log/draft/{runID}/exercises.
func (s *Server) handleTrainingLogAddExercise(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	var libExerciseID *uint
	libIDStr := strings.TrimSpace(r.FormValue("library_exercise_id"))
	if libIDStr != "" {
		if lid, err2 := strconv.ParseUint(libIDStr, 10, 64); err2 == nil && lid > 0 {
			lv := uint(lid)
			libExerciseID = &lv
		}
	}

	name := strings.TrimSpace(r.FormValue("name"))
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		kind = "reps_and_sets"
	}

	// If a library exercise was selected, pull name and kind from it.
	if libExerciseID != nil {
		var le db.LibraryExercise
		if err2 := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, *libExerciseID).First(&le).Error; err2 == nil {
			if name == "" {
				name = le.Name
			}
			kind = le.Kind
		}
	}
	if name == "" {
		name = "Exercise"
	}

	if _, err := db.AddManualExercise(s.store.DB, ownerID, runID, name, libExerciseID, kind); err != nil {
		s.serverError(w, r, err)
		return
	}

	s.renderManualExercises(w, r, ownerID, runID)
}

// handleTrainingLogSaveExerciseCompletion serves POST /training-log/draft/{runID}/exercises/{exerciseID}/save.
func (s *Server) handleTrainingLogSaveExerciseCompletion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}
	if err := db.UpsertManualExerciseCompletion(
		s.store.DB, ownerID, runID, exerciseID,
		formInt(r, "sets"), formInt(r, "reps"), formFloat(r, "weight_kg"),
		strings.TrimSpace(r.FormValue("notes")),
	); err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleTrainingLogDeleteExercise serves POST /training-log/draft/{runID}/exercises/{exerciseID}/delete.
func (s *Server) handleTrainingLogDeleteExercise(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise ID", http.StatusBadRequest)
		return
	}
	if err := db.DeleteManualExercise(s.store.DB, ownerID, runID, exerciseID); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderManualExercises(w, r, ownerID, runID)
}

// renderManualExercises renders the manual exercises container fragment.
func (s *Server) renderManualExercises(w http.ResponseWriter, r *http.Request, ownerID, runID uint) {
	exs, err := db.ListExercisesForRun(s.store.DB, ownerID, runID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	libExercises, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	views := make([]pages.ManualExerciseView, 0, len(exs))
	for _, ex := range exs {
		mev := pages.ManualExerciseView{
			ExerciseID: ex.ID,
			RunID:      runID,
			Name:       ex.Name,
			Kind:       ex.Kind,
		}
		// Load completion if it exists.
		var comp db.RunExerciseCompletion
		if err2 := s.store.DB.Where("owner_id = ? AND run_id = ? AND exercise_id = ?",
			ownerID, runID, ex.ID).First(&comp).Error; err2 == nil {
			mev.ActualSets = comp.ActualSets
			mev.ActualReps = comp.ActualReps
			mev.ActualWeightKg = comp.ActualWeightKg
			mev.Notes = comp.RunNotes
		}
		views = append(views, mev)
	}
	s.pages.RenderManualExercisesContainer(w, pages.TrainingLogNewParams{
		DraftRunID:       runID,
		LibraryExercises: libExercises,
		Exercises:        views,
	})
}

// handleTrainingLogEdit serves GET|POST /training-log/{journalID}/edit.
func (s *Server) handleTrainingLogEdit(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}

	journalID, err := parseUintParam(r, "journalID")
	if err != nil {
		http.Error(w, "invalid journal ID", http.StatusBadRequest)
		return
	}

	j, err := db.GetSessionJournalByID(s.store.DB, ownerID, uint(journalID))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if j == nil {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodPost {
		s.updateJournal(w, r, ownerID, j)
		return
	}

	// Build the run info string for run-linked entries.
	runInfo := ""
	isRunLinked := j.RunID != nil
	if isRunLinked {
		var run db.SessionRun
		if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, *j.RunID).First(&run).Error; err == nil {
			ss := db.ScheduledSession{}
			s.store.DB.Preload("SessionTemplate").Where("id = ?", run.ScheduledSessionID).First(&ss)
			name := ss.SessionTemplate.Name
			if run.CustomName != "" {
				name = run.CustomName
			}
			t := run.StartedAt
			runInfo = fmt.Sprintf("%s · %s %d%s", name, t.Format("Jan"), t.Day(), daySuffix(t.Day()))
		}
	}

	dateVal := j.Date.Format("2006-01-02")
	if j.Date.IsZero() {
		dateVal = j.CreatedAt.Format("2006-01-02")
	}

	s.pages.TrainingLogNewPage(w, pages.TrainingLogNewParams{
		JournalID:   uint(journalID),
		IsRunLinked: isRunLinked,
		RunInfo:     runInfo,
		DateValue:   dateVal,
		Title:       j.Title,
		SleepScore:  j.SleepScore,
		Energy:      j.Energy,
		RPE:         j.RPE,
		Focus:       j.Focus,
		Location:    j.Location,
		WentWell:    j.WentWell,
		NextFocus:   j.NextFocus,
	})
}

func (s *Server) updateJournal(w http.ResponseWriter, r *http.Request, ownerID uint, j *db.SessionJournal) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	j.SleepScore = formInt(r, "sleep_score")
	j.Energy = formInt(r, "energy")
	j.RPE = formInt(r, "rpe")
	j.Focus = strings.TrimSpace(r.FormValue("focus"))
	j.Location = strings.TrimSpace(r.FormValue("location"))
	j.WentWell = strings.TrimSpace(r.FormValue("went_well"))
	j.NextFocus = strings.TrimSpace(r.FormValue("next_focus"))

	// Only update title/date for standalone entries.
	if j.RunID == nil {
		j.Title = strings.TrimSpace(r.FormValue("title"))
		dateStr := strings.TrimSpace(r.FormValue("date"))
		if d, err := time.ParseInLocation("2006-01-02", dateStr, time.Local); err == nil {
			j.Date = d
		}
	}

	if err := db.UpsertSessionJournal(s.store.DB, j); err != nil {
		s.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/training-log", http.StatusSeeOther)
}

// handleTrainingLogDelete serves POST /training-log/{journalID}/delete.
func (s *Server) handleTrainingLogDelete(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}

	journalID, err := parseUintParam(r, "journalID")
	if err != nil {
		http.Error(w, "invalid journal ID", http.StatusBadRequest)
		return
	}

	if err := db.DeleteSessionJournal(s.store.DB, ownerID, uint(journalID)); err != nil {
		s.serverError(w, r, err)
		return
	}

	// HTMX: return empty body so hx-swap="outerHTML" removes the card.
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/training-log", http.StatusSeeOther)
}
