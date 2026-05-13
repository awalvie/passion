package web

import (
	"net/http"
	"sort"
	"time"

	"passion/db"
	"passion/pages"
)

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	now := time.Now()

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	if mq := r.URL.Query().Get("month"); mq != "" {
		if parsed, err := time.ParseInLocation("2006-01", mq, now.Location()); err == nil {
			monthStart = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, now.Location())
		}
	}
	monthCalendarStart := mondayOfLocalDate(monthStart)
	monthCalendarEnd := monthCalendarStart.AddDate(0, 0, 41)

	monthPrev := monthStart.AddDate(0, -1, 0)
	monthNext := monthStart.AddDate(0, 1, 0)
	monthPrevURL := "/calendar?month=" + monthPrev.Format("2006-01")
	monthNextURL := "/calendar?month=" + monthNext.Format("2006-01")

	monthSessions, err := db.ListScheduledSessionsInRange(s.store.DB, ownerID, monthCalendarStart, monthCalendarEnd)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	completedByDate := map[string]int{}
	unscheduledByDate := map[string]int{}
	completedSSIDSet := map[uint]bool{}
	completedRuns, err := db.ListCompletedRunDatesInRange(s.store.DB, ownerID, monthCalendarStart, monthCalendarEnd)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	for _, run := range completedRuns {
		key := localDateKey(run.ScheduledDate)
		if run.IsTrial {
			unscheduledByDate[key]++
		} else {
			completedByDate[key]++
			completedSSIDSet[run.ScheduledSessionID] = true
		}
	}

	cycleNameMap := map[uint]string{}
	{
		cycleIDs := map[uint]bool{}
		for _, ss := range monthSessions {
			if ss.TrainingCycleID != nil {
				cycleIDs[*ss.TrainingCycleID] = true
			}
		}
		if len(cycleIDs) > 0 {
			ids := make([]uint, 0, len(cycleIDs))
			for id := range cycleIDs {
				ids = append(ids, id)
			}
			var cycles []db.TrainingCycle
			if err := s.store.DB.Where("id IN ?", ids).Find(&cycles).Error; err != nil {
				s.serverError(w, r, err)
				return
			}
			for _, c := range cycles {
				cycleNameMap[c.ID] = c.Name
			}
		}
	}

	byDate := map[string][]db.ScheduledSession{}
	for _, ss := range monthSessions {
		key := localDateKey(ss.ScheduledDate)
		byDate[key] = append(byDate[key], ss)
	}

	calEvents, err := db.ListCalendarEventsInRange(s.store.DB, ownerID, monthCalendarStart, monthCalendarEnd)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	eventsByDateKey := buildEventsByDateKey(calEvents)

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
			Events:           eventsByDateKey[key],
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

	allEventViews := make([]pages.CalendarEventView, 0, len(calEvents))
	for _, e := range calEvents {
		allEventViews = append(allEventViews, calendarEventToView(e))
	}
	sort.Slice(allEventViews, func(i, j int) bool {
		return allEventViews[i].StartKey < allEventViews[j].StartKey
	})

	s.pages.CalendarPage(w, pages.CalendarPageParams{
		Base:            pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		Cells:           cells,
		AllEvents:       allEventViews,
		CalendarMonth:   monthStart.Format("January"),
		CalendarYear:    monthStart.Format("2006"),
		CalendarWeekday: []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		MonthPrevURL:    monthPrevURL,
		MonthNextURL:    monthNextURL,
	})
}
