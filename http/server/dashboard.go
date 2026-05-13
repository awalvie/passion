package web

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

func (s *Server) handleDashboardStartFromTemplate(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	templateID, err := strconv.ParseUint(r.FormValue("template_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid template_id", http.StatusBadRequest)
		return
	}

	runID, err := s.startTrialRun(uint(templateID), ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(runID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleStartSessionPicker(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	var templates []db.SessionTemplate
	if err := s.store.DB.Where("owner_id = ?", ownerID).Order("name asc").Find(&templates).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	s.pages.StartSessionPicker(w, pages.StartSessionPickerParams{Templates: templates})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	now := time.Now()

	var templates []db.SessionTemplate
	err := s.store.DB.
		Where("owner_id = ? AND is_system = ?", ownerID, false).
		Order("name asc").
		Find(&templates).Error
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// --- Active (in-progress) runs — exclude manual log drafts ---
	var activeRuns []db.SessionRun
	err = s.store.DB.
		Where("owner_id = ? AND status = ? AND is_draft = ?", ownerID, db.RunStatusRunning, false).
		Order("started_at desc").
		Find(&activeRuns).Error
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// --- Draft manual log entries ---
	var draftRuns []db.SessionRun
	err = s.store.DB.
		Where("owner_id = ? AND is_draft = ?", ownerID, true).
		Order("started_at desc").
		Find(&draftRuns).Error
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	draftLogViews := make([]pages.DraftLogEntryView, 0, len(draftRuns))
	for _, dr := range draftRuns {
		draftLogViews = append(draftLogViews, pages.DraftLogEntryView{
			RunID:     dr.ID,
			DateLabel: relativeDateLabel(dr.StartedAt),
		})
	}

	activeRunViews := make([]pages.ActiveRunView, 0, len(activeRuns))
	if len(activeRuns) > 0 {
		arSSIDs := make([]uint, 0, len(activeRuns))
		for _, ar := range activeRuns {
			arSSIDs = append(arSSIDs, ar.ScheduledSessionID)
		}
		var arSessions []db.ScheduledSession
		err = s.store.DB.
			Preload("SessionTemplate").
			Where("id IN ?", arSSIDs).
			Find(&arSessions).Error
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		arSSMap := map[uint]db.ScheduledSession{}
		for _, ss := range arSessions {
			arSSMap[ss.ID] = ss
		}
		for _, ar := range activeRuns {
			ss := arSSMap[ar.ScheduledSessionID]
			activeRunViews = append(activeRunViews, pages.ActiveRunView{
				RunID:         ar.ID,
				TemplateName:  ss.SessionTemplate.Name,
				Color:         normalizeTemplateColor(ss.SessionTemplate.Color),
				StartedLabel:  ar.StartedAt.Format("Mon 3:04 PM"),
				StartedAtUnix: ar.StartedAt.Unix(),
			})
		}
	}

	// --- Week navigation ---
	weekStart := mondayOfLocalDate(now)
	if wq := r.URL.Query().Get("week"); wq != "" {
		if parsed, perr := time.ParseInLocation("2006-01-02", wq, now.Location()); perr == nil {
			weekStart = mondayOfLocalDate(parsed)
		}
	}
	weekEnd := weekStart.AddDate(0, 0, 6)
	var weekLabel string
	switch {
	case weekStart.Month() != weekEnd.Month():
		weekLabel = dayWithSuffix(weekStart) + " – " + dayWithSuffix(weekEnd)
	default:
		sd, ed := weekStart.Day(), weekEnd.Day()
		weekLabel = weekStart.Format("Jan") + " " + fmt.Sprintf("%d%s", sd, daySuffix(sd)) + "–" + fmt.Sprintf("%d%s", ed, daySuffix(ed))
	}

	// --- Month navigation ---
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	if mq := r.URL.Query().Get("month"); mq != "" {
		if parsed, perr := time.ParseInLocation("2006-01", mq, now.Location()); perr == nil {
			monthStart = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, now.Location())
		}
	}
	monthCalendarStart := mondayOfLocalDate(monthStart)
	monthCalendarEnd := monthCalendarStart.AddDate(0, 0, 41) // 6-week grid

	// Build navigation URLs preserving the other parameter
	weekPrev := weekStart.AddDate(0, 0, -7)
	weekNext := weekStart.AddDate(0, 0, 7)
	monthPrev := monthStart.AddDate(0, -1, 0)
	monthNext := monthStart.AddDate(0, 1, 0)

	monthParam := monthStart.Format("2006-01")
	weekParam := weekStart.Format("2006-01-02")
	weekPrevURL := "/dashboard?week=" + weekPrev.Format("2006-01-02") + "&month=" + monthParam
	weekNextURL := "/dashboard?week=" + weekNext.Format("2006-01-02") + "&month=" + monthParam
	monthPrevURL := "/dashboard?month=" + monthPrev.Format("2006-01") + "&week=" + weekParam
	monthNextURL := "/dashboard?month=" + monthNext.Format("2006-01") + "&week=" + weekParam

	weekSessions, err := db.ListScheduledSessionsInRange(s.store.DB, ownerID, weekStart, weekEnd)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	monthSessions, err := db.ListScheduledSessionsInRange(s.store.DB, ownerID, monthCalendarStart, monthCalendarEnd)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// Count exercises per template for week sessions
	exerciseCountByTemplate := map[uint]int{}
	if len(weekSessions) > 0 {
		tplIDs := map[uint]bool{}
		for _, ss := range weekSessions {
			tplIDs[ss.SessionTemplateID] = true
		}
		uniqueTplIDs := make([]uint, 0, len(tplIDs))
		for id := range tplIDs {
			uniqueTplIDs = append(uniqueTplIDs, id)
		}
		var tplsWithExercises []db.SessionTemplate
		err = s.store.DB.
			Preload("Activities", func(tx *gorm.DB) *gorm.DB {
				return tx.Where("owner_id = ?", ownerID)
			}).
			Preload("Activities.Exercises", func(tx *gorm.DB) *gorm.DB {
				return tx.Where("owner_id = ? AND parent_exercise_id IS NULL", ownerID)
			}).
			Where("id IN ? AND owner_id = ?", uniqueTplIDs, ownerID).
			Find(&tplsWithExercises).Error
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		for _, tpl := range tplsWithExercises {
			count := 0
			for _, act := range tpl.Activities {
				count += len(act.Exercises)
			}
			exerciseCountByTemplate[tpl.ID] = count
		}
	}

	// Load cycle names for sessions that belong to a training cycle
	cycleNameMap := map[uint]string{}
	allSessions := append(weekSessions, monthSessions...)
	{
		cycleIDs := map[uint]bool{}
		for _, ss := range allSessions {
			if ss.TrainingCycleID != nil {
				cycleIDs[*ss.TrainingCycleID] = true
			}
		}
		if len(cycleIDs) > 0 {
			uniqueCycleIDs := make([]uint, 0, len(cycleIDs))
			for id := range cycleIDs {
				uniqueCycleIDs = append(uniqueCycleIDs, id)
			}
			var cycles []db.TrainingCycle
			err = s.store.DB.Where("id IN ?", uniqueCycleIDs).Find(&cycles).Error
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			for _, c := range cycles {
				cycleNameMap[c.ID] = c.Name
			}
		}
	}

	// --- Completed runs for the week (to mark Done on session cards) ---
	completedWeekSSIDs := map[uint]bool{}
	if len(weekSessions) > 0 {
		ssIDs := make([]uint, 0, len(weekSessions))
		for _, ss := range weekSessions {
			ssIDs = append(ssIDs, ss.ID)
		}
		var completedWeekRuns []db.SessionRun
		if err = s.store.DB.
			Where("owner_id = ? AND status = ? AND scheduled_session_id IN ?", ownerID, db.RunStatusCompleted, ssIDs).
			Find(&completedWeekRuns).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
		for _, r := range completedWeekRuns {
			completedWeekSSIDs[r.ScheduledSessionID] = true
		}
	}

	// --- Completed runs for the month calendar ---
	completedByDate := map[string]int{}   // dateKey -> completed scheduled count
	unscheduledByDate := map[string]int{} // dateKey -> trial run count
	completedSSIDSet := map[uint]bool{}   // scheduled_session_id -> completed
	completedMonthRuns, err := db.ListCompletedRunDatesInRange(s.store.DB, ownerID, monthCalendarStart, monthCalendarEnd)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	for _, r := range completedMonthRuns {
		key := localDateKey(r.ScheduledDate)
		if r.IsTrial {
			unscheduledByDate[key]++
		} else {
			completedByDate[key]++
			completedSSIDSet[r.ScheduledSessionID] = true
		}
	}

	weekSessionViews := make([]pages.DashboardSession, 0, len(weekSessions))
	dayGroupMap := map[string]*pages.DashboardDayGroup{}
	var dayGroupOrder []string
	for _, ss := range weekSessions {
		d := ss.ScheduledDate.Day()
		dayLabel := ss.ScheduledDate.Format("Mon") + " " + fmt.Sprintf("%d%s", d, daySuffix(d))
		dayKey := localDateKey(ss.ScheduledDate)

		cycleName := ""
		if ss.TrainingCycleID != nil {
			cycleName = cycleNameMap[*ss.TrainingCycleID]
		}

		view := pages.DashboardSession{
			ID:            ss.ID,
			DateLabel:     dayLabel,
			TemplateName:  ss.SessionTemplate.Name,
			ExerciseCount: exerciseCountByTemplate[ss.SessionTemplateID],
			CycleName:     cycleName,
			Color:         normalizeTemplateColor(ss.SessionTemplate.Color),
			Done:          completedWeekSSIDs[ss.ID],
		}
		weekSessionViews = append(weekSessionViews, view)

		if _, ok := dayGroupMap[dayKey]; !ok {
			dayGroupMap[dayKey] = &pages.DashboardDayGroup{DayLabel: dayLabel}
			dayGroupOrder = append(dayGroupOrder, dayKey)
		}
		dayGroupMap[dayKey].Sessions = append(dayGroupMap[dayKey].Sessions, view)
	}
	weekDayGroups := make([]pages.DashboardDayGroup, 0, len(dayGroupOrder))
	for _, key := range dayGroupOrder {
		weekDayGroups = append(weekDayGroups, *dayGroupMap[key])
	}

	byDate := map[string][]db.ScheduledSession{}
	for _, ss := range monthSessions {
		key := localDateKey(ss.ScheduledDate)
		byDate[key] = append(byDate[key], ss)
	}

	// Load calendar events for the month grid.
	monthCalEvents, err := db.ListCalendarEventsInRange(s.store.DB, ownerID, monthCalendarStart, monthCalendarEnd)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	monthEventsByDateKey := buildEventsByDateKey(monthCalEvents)

	cells := make([]pages.CalendarCell, 0, 42)
	for dayIdx := 0; dayIdx < 42; dayIdx++ {
		d := monthCalendarStart.AddDate(0, 0, dayIdx)
		key := localDateKey(d)
		sessions := byDate[key]
		cell := pages.CalendarCell{
			Day:              d.Day(),
			InMonth:          d.Month() == monthStart.Month(),
			DateKey:          key,
			CompletedCount:   completedByDate[key],
			UnscheduledCount: unscheduledByDate[key],
			Events:           monthEventsByDateKey[key],
		}
		if len(sessions) > 0 {
			cell.FirstSessionColor = normalizeTemplateColor(sessions[0].SessionTemplate.Color)
		}
		for _, ss := range sessions {
			cn := ""
			if ss.TrainingCycleID != nil {
				cn = cycleNameMap[*ss.TrainingCycleID]
			}
			cell.Sessions = append(cell.Sessions, pages.CalendarCellSession{
				Name:      ss.SessionTemplate.Name,
				Color:     normalizeTemplateColor(ss.SessionTemplate.Color),
				CycleName: cn,
				Done:      completedSSIDSet[ss.ID],
			})
		}
		cells = append(cells, cell)
	}

	// Conflict warnings: blocking events in the next 30 days that overlap scheduled sessions.
	today := localDate(now)
	upcoming := today.AddDate(0, 0, 30)
	upcomingEvents, err := db.ListCalendarEventsInRange(s.store.DB, ownerID, today, upcoming)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	// Build a map of dateKey → scheduled sessions for the next 30 days.
	upcomingSessions, err := db.ListScheduledSessionsInRange(s.store.DB, ownerID, today, upcoming)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	upcomingByDate := map[string][]db.ScheduledSession{}
	for _, ss := range upcomingSessions {
		key := localDateKey(ss.ScheduledDate)
		upcomingByDate[key] = append(upcomingByDate[key], ss)
	}
	var conflictWarnings []pages.ConflictWarningView
	for _, e := range upcomingEvents {
		if !e.Blocks {
			continue
		}
		count := 0
		cycleID := uint(0)
		for d := e.StartDate; !d.After(e.EndDate); d = d.AddDate(0, 0, 1) {
			sss := upcomingByDate[localDateKey(d)]
			count += len(sss)
			if cycleID == 0 && len(sss) > 0 && sss[0].TrainingCycleID != nil {
				cycleID = *sss[0].TrainingCycleID
			}
		}
		if count == 0 {
			continue
		}
		cycleName := ""
		if cycleID != 0 {
			cycleName = cycleNameMap[cycleID]
		}
		conflictWarnings = append(conflictWarnings, pages.ConflictWarningView{
			EventTitle:   e.Title,
			EventColor:   pages.CalendarEventColor(e.Kind),
			StartLabel:   e.StartDate.Format("Jan 2"),
			EndLabel:     e.EndDate.Format("Jan 2"),
			SessionCount: count,
			CycleName:    cycleName,
			CycleID:      cycleID,
		})
	}

	weekdayLabels := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	s.pages.Dashboard(w, pages.DashboardParams{
		Base:             pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		Templates:        templates,
		ActiveRuns:       activeRunViews,
		DraftLogEntries:  draftLogViews,
		WeekSessions:     weekSessionViews,
		WeekDayGroups:    weekDayGroups,
		WeekLabel:        weekLabel,
		WeekPrevURL:      weekPrevURL,
		WeekNextURL:      weekNextURL,
		CalendarCells:    cells,
		CalendarMonth:    monthStart.Format("January"),
		CalendarYear:     monthStart.Format("2006"),
		CalendarWeekday:  weekdayLabels,
		MonthPrevURL:     monthPrevURL,
		MonthNextURL:     monthNextURL,
		ConflictWarnings: conflictWarnings,
	})
}
