package web

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

	// Load all completed runs, newest first.
	var runs []db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND status = ?", ownerID, "completed").
		Order("started_at desc").
		Find(&runs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, ss := range ssList {
			ssMap[ss.ID] = ss
		}
	}

	// Load all journals for this user and index by RunID.
	journals, err := db.ListSessionJournals(s.store.DB, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	journalByRun := map[uint]db.SessionJournal{}
	for _, j := range journals {
		journalByRun[j.RunID] = j
	}

	// Build entry views.
	entries := make([]pages.TrainingLogEntryView, 0, len(runs))
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

		entry := pages.TrainingLogEntryView{
			RunID:         run.ID,
			DateLabel:     dateLabel,
			TemplateName:  templateName,
			Color:         ss.SessionTemplate.Color,
			DurationLabel: dur,
			MonthGroup:    monthGroup,
		}

		if j, ok := journalByRun[run.ID]; ok {
			entry.HasJournal = true
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
			ownerID, "completed", cycle.ID).
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
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	runID, err := strconv.ParseUint(chi.URLParam(r, "runID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	// Validate ownership of the run.
	var run db.SessionRun
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, uint(runID)).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.serveJournalForm(w, r, ownerID, uint(runID), false)
	case http.MethodPost:
		s.saveJournal(w, r, ownerID, uint(runID))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) serveJournalForm(w http.ResponseWriter, r *http.Request, ownerID, runID uint, saved bool) {
	j, err := db.GetSessionJournalByRunID(s.store.DB, ownerID, runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		if saved {
			// Render the pre-filled read view.
		}
	}

	s.pages.RenderJournalForm(w, params)
}

func (s *Server) saveJournal(w http.ResponseWriter, r *http.Request, ownerID, runID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	parseInt := func(key string) int {
		v, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(key)))
		return v
	}

	// Look up existing journal (upsert).
	existing, err := db.GetSessionJournalByRunID(s.store.DB, ownerID, runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	j := db.SessionJournal{
		OwnerID:   ownerID,
		RunID:     runID,
		SleepScore: parseInt("sleep_score"),
		Energy:    parseInt("energy"),
		RPE:       parseInt("rpe"),
		Focus:     strings.TrimSpace(r.FormValue("focus")),
		Location:  strings.TrimSpace(r.FormValue("location")),
		WentWell:  strings.TrimSpace(r.FormValue("went_well")),
		NextFocus: strings.TrimSpace(r.FormValue("next_focus")),
	}
	if existing != nil {
		j.Model = existing.Model // preserve ID + timestamps for update
	}

	if err := db.UpsertSessionJournal(s.store.DB, &j); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the saved (read-only) fragment so HTMX can swap it in.
	s.serveJournalForm(w, r, ownerID, runID, true)
}

