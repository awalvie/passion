package web

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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
	ownerID := s.mustUserID(r)

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
		dateLabel := fmt.Sprintf("%s, %s %d%s %d", t.Format("Mon"), t.Format("Jan"), t.Day(), daySuffix(t.Day()), t.Year())
		monthGroup := t.Format("January 2006")
		monday := mondayOfLocalDate(t)
		sunday := monday.AddDate(0, 0, 6)
		weekGroup := fmt.Sprintf("%s %d – %s %d", monday.Format("Jan"), monday.Day(), sunday.Format("Jan"), sunday.Day())

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
			WeekGroup:        weekGroup,
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
			entry.SleepPct = j.SleepScore * 20
			entry.EnergyPct = j.Energy * 20
			entry.RPEPct = j.RPE * 10
			entry.Focus = focusDisplayName(j.Focus)
			entry.Location = locationDisplayName(j.Location)
			entry.WentWellHTML = markdownToHTML(j.WentWell)
			entry.NextFocusHTML = markdownToHTML(j.NextFocus)
			entry.JournalTeaser = journalTeaser(j.WentWell)
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
		dateLabel := fmt.Sprintf("%s, %s %d%s %d", t.Format("Mon"), t.Format("Jan"), t.Day(), daySuffix(t.Day()), t.Year())
		monday := mondayOfLocalDate(t)
		sunday := monday.AddDate(0, 0, 6)
		weekGroup := fmt.Sprintf("%s %d – %s %d", monday.Format("Jan"), monday.Day(), sunday.Format("Jan"), sunday.Day())
		entries = append(entries, pages.TrainingLogEntryView{
			JournalEntryID: j.ID,
			SortTime:       t,
			DateLabel:      dateLabel,
			TemplateName:   title,
			MonthGroup:     t.Format("January 2006"),
			WeekGroup:      weekGroup,
			IsStandalone:   true,
			HasJournal:     true,
			SleepScore:     j.SleepScore,
			Energy:         j.Energy,
			RPE:            j.RPE,
			SleepPct:       j.SleepScore * 20,
			EnergyPct:      j.Energy * 20,
			RPEPct:         j.RPE * 10,
			Focus:          focusDisplayName(j.Focus),
			Location:       locationDisplayName(j.Location),
			WentWellHTML:   markdownToHTML(j.WentWell),
			NextFocusHTML:  markdownToHTML(j.NextFocus),
			JournalTeaser:  journalTeaser(j.WentWell),
		})
	}

	// Sort all entries newest-first so standalone and run entries interleave correctly.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SortTime.After(entries[j].SortTime)
	})

	stats := buildTrainingLogStats(entries, time.Now())

	// Build adherence block from the most recent training cycle.
	adherence, cycleName := s.buildAdherenceView(ownerID)

	s.pages.TrainingLogPage(w, pages.TrainingLogPageParams{
		Entries:   entries,
		Adherence: adherence,
		CycleName: cycleName,
		Stats:     stats,
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


// handleTrainingLogEdit serves GET|POST /training-log/{journalID}/edit.
func (s *Server) handleTrainingLogEdit(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)

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

	// Build the run info string and load exercise data for manual run-linked entries.
	runInfo := ""
	isRunLinked := j.RunID != nil
	params := pages.TrainingLogNewParams{
		JournalID:  uint(journalID),
		SleepScore: j.SleepScore,
		Energy:     j.Energy,
		RPE:        j.RPE,
		Focus:      j.Focus,
		Location:   j.Location,
		WentWell:   j.WentWell,
		NextFocus:  j.NextFocus,
	}

	if isRunLinked {
		var run db.SessionRun
		if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, *j.RunID).First(&run).Error; err == nil {
			ss, ssErr := db.GetScheduledSessionWithTemplate(s.store.DB, ownerID, run.ScheduledSessionID)
			if ssErr == nil {
				name := ss.SessionTemplate.Name
				if run.CustomName != "" {
					name = run.CustomName
				}
				t := run.StartedAt
				runInfo = fmt.Sprintf("%s · %s %d%s", name, t.Format("Jan"), t.Day(), daySuffix(t.Day()))
			}

			// Load exercise management for all run-linked entries.
			libExercises, err := db.ListLibraryExercises(s.store.DB, ownerID)
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			activityTemplates, err := db.ListActivityTemplates(s.store.DB, ownerID, "")
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			venues, boards, err := s.loadVenuesAndBoards(ownerID)
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			// Exercises added on-the-fly (manual or added after the fact).
			exs, err := db.ListExercisesForRun(s.store.DB, ownerID, run.ID)
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			views := make([]pages.ManualExerciseView, 0, len(exs))
			for _, ex := range exs {
				mev := pages.ManualExerciseView{
					ExerciseID: ex.ID,
					RunID:      run.ID,
					Name:       ex.Name,
					Kind:       ex.Kind,
				}
				var comp db.RunExerciseCompletion
				if err2 := s.store.DB.Where("owner_id = ? AND run_id = ? AND exercise_id = ?",
					ownerID, run.ID, ex.ID).First(&comp).Error; err2 == nil {
					mev.ActualSets = comp.ActualSets
					mev.ActualReps = comp.ActualReps
					mev.ActualWeightKg = comp.ActualWeightKg
					mev.Notes = comp.RunNotes
					mev.ElapsedMinutes = comp.ElapsedSeconds / 60
				}
				setLogs, _ := db.ListManualExerciseSetLogs(s.store.DB, ownerID, run.ID, ex.ID)
				if len(setLogs) > 0 {
					mev.PerSetMode = true
					mev.SetLogs = make([]pages.ManualExerciseSetLogView, len(setLogs))
					for i, sl := range setLogs {
						mev.SetLogs[i] = pages.ManualExerciseSetLogView{
							SetIndex: sl.SetIndex,
							Reps:     sl.Reps,
							WeightKg: sl.WeightKg,
						}
					}
				}
				if ex.Kind == "climbing" {
					if meta, _ := db.GetClimbingExerciseMeta(s.store.DB, ownerID, run.ID, ex.ID); meta != nil {
						mev.ClimbingMeta = pages.ClimbingExerciseMetaView{
							Type:      meta.Type,
							BoardKind: meta.BoardKind,
						}
					}
				}
				views = append(views, mev)
			}
			params.DraftRunID = run.ID
			params.LibraryExercises = libExercises
			params.ActivityTemplates = activityTemplates
			params.Venues = venues
			params.Boards = boards
			params.Exercises = views
			if j.VenueID != nil {
				params.VenueID = *j.VenueID
			}
			if j.BoardID != nil {
				params.BoardID = *j.BoardID
			}

			// For template-based runs, load activities with their completions as read-only.
			if ssErr == nil && !run.IsManual && !run.IsOpen {
				var completions []db.RunExerciseCompletion
				s.store.DB.Where("run_id = ? AND owner_id = ?", run.ID, ownerID).Find(&completions)
				compByID := make(map[uint]db.RunExerciseCompletion, len(completions))
				for _, c := range completions {
					compByID[c.ExerciseID] = c
				}
				for _, act := range ss.SessionTemplate.Activities {
					sa := pages.RunSummaryActivity{Name: act.Name}
					for _, ex := range act.Exercises {
						if ex.ParentExerciseID != nil {
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
							se.Notes = c.RunNotes
							se.ElapsedSeconds = c.ElapsedSeconds
						}
						sa.Exercises = append(sa.Exercises, se)
					}
					if len(sa.Exercises) > 0 {
						params.TemplateActivities = append(params.TemplateActivities, sa)
					}
				}
			}
		}
		params.IsRunLinked = true
		params.RunInfo = runInfo
	} else {
		dateVal := j.Date.Format("2006-01-02")
		if j.Date.IsZero() {
			dateVal = j.CreatedAt.Format("2006-01-02")
		}
		params.DateValue = dateVal
		params.Title = j.Title
	}

	s.pages.TrainingLogNewPage(w, params)
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
	j.WentWell = strings.TrimSpace(r.FormValue("went_well"))
	j.NextFocus = strings.TrimSpace(r.FormValue("next_focus"))

	// Venue / board / location (present when editing a manual run-linked entry).
	venueIDStr := strings.TrimSpace(r.FormValue("venue_id"))
	boardIDStr := strings.TrimSpace(r.FormValue("board_id"))
	if v, err2 := strconv.ParseUint(venueIDStr, 10, 64); err2 == nil && v > 0 {
		vv := uint(v)
		j.VenueID = &vv
		// Derive location from venue kind.
		var venue db.ClimbingVenue
		if err2 := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, vv).First(&venue).Error; err2 == nil {
			if venue.Kind == "outdoor" {
				j.Location = "outdoor"
			} else {
				j.Location = "indoor"
			}
		}
	} else {
		j.VenueID = nil
		j.Location = strings.TrimSpace(r.FormValue("location"))
	}
	if b, err2 := strconv.ParseUint(boardIDStr, 10, 64); err2 == nil && b > 0 {
		bv := uint(b)
		j.BoardID = &bv
	} else {
		j.BoardID = nil
	}

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
	ownerID := s.mustUserID(r)

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

// handleTrainingLogForRun serves GET /training-log/for-run/{runID}.
// It ensures a SessionJournal exists for the given run, then redirects to its edit page.
func (s *Server) handleTrainingLogForRun(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	j, err := db.GetSessionJournalByRunID(s.store.DB, ownerID, runID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if j == nil {
		j = &db.SessionJournal{OwnerID: ownerID, RunID: &runID}
		if err := db.UpsertSessionJournal(s.store.DB, j); err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/training-log/%d/edit", j.ID), http.StatusSeeOther)
}

// buildTrainingLogStats derives aggregate stats from the already-computed entry list.
func buildTrainingLogStats(entries []pages.TrainingLogEntryView, now time.Time) pages.TrainingLogStatsView {
	var sleepSum, sleepN, energySum, energyN, rpeSum, rpeN int
	focusCounts := map[string]int{}
	var indoorCount, outdoorCount, thisMonth, thisWeek int
	activeDays := map[string]bool{}

	weekStart := mondayOfLocalDate(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	for _, e := range entries {
		if e.SleepScore > 0 {
			sleepSum += e.SleepScore
			sleepN++
		}
		if e.Energy > 0 {
			energySum += e.Energy
			energyN++
		}
		if e.RPE > 0 {
			rpeSum += e.RPE
			rpeN++
		}
		if e.Focus != "" {
			focusCounts[e.Focus]++
		}
		switch e.Location {
		case "Indoor":
			indoorCount++
		case "Outdoor":
			outdoorCount++
		}
		if !e.SortTime.Before(monthStart) {
			thisMonth++
		}
		if !e.SortTime.Before(weekStart) {
			thisWeek++
		}
		activeDays[localDateKey(e.SortTime)] = true
	}

	avg := func(sum, n int, denom string) string {
		if n == 0 {
			return "—"
		}
		whole := sum / n
		frac := (sum*10/n) % 10
		if frac == 0 {
			return fmt.Sprintf("%d / %s", whole, denom)
		}
		return fmt.Sprintf("%d.%d / %s", whole, frac, denom)
	}

	topFocus := ""
	topCount := 0
	for f, c := range focusCounts {
		if c > topCount {
			topCount = c
			topFocus = f
		}
	}

	currentStreak, _ := computeStreaks(activeDays, now)

	return pages.TrainingLogStatsView{
		TotalSessions: len(entries),
		ThisMonth:     thisMonth,
		ThisWeek:      thisWeek,
		CurrentStreak: currentStreak,
		AvgSleep:      avg(sleepSum, sleepN, "5"),
		AvgEnergy:     avg(energySum, energyN, "5"),
		AvgRPE:        avg(rpeSum, rpeN, "10"),
		TopFocus:      topFocus,
		IndoorCount:   indoorCount,
		OutdoorCount:  outdoorCount,
	}
}

// journalTeaser returns a plain-text single-line preview of a journal field,
// stripped of markdown syntax and truncated to 120 characters.
func journalTeaser(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Take only the first line.
	if idx := strings.IndexByte(raw, '\n'); idx >= 0 {
		raw = raw[:idx]
	}
	// Strip common markdown: headers, bold, italic, code, links.
	raw = strings.TrimLeft(raw, "#> ")
	replacer := strings.NewReplacer("**", "", "__", "", "*", "", "_", "", "`", "")
	raw = replacer.Replace(raw)
	raw = strings.TrimSpace(raw)
	if len([]rune(raw)) > 120 {
		runes := []rune(raw)
		raw = string(runes[:120]) + "…"
	}
	return raw
}
