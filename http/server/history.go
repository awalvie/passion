package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"time"

	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

type HeatmapSession struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type HeatmapDay struct {
	Date     string           `json:"date"`
	Count    int              `json:"count"`
	Sessions []HeatmapSession `json:"sessions,omitempty"`
}

type WeeklyDataPoint struct {
	WeekLabel string `json:"label"`
	Count     int    `json:"count"`
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)

	now := time.Now()

	// Time range filter
	rangeParam := r.URL.Query().Get("range")
	validRanges := map[string]bool{"30d": true, "3mo": true, "6mo": true, "1y": true, "all": true}
	if !validRanges[rangeParam] {
		rangeParam = "all"
	}

	query := s.store.DB.Where("owner_id = ?", ownerID).Order("started_at desc")
	switch rangeParam {
	case "30d":
		query = query.Where("started_at >= ?", now.AddDate(0, 0, -30))
	case "3mo":
		query = query.Where("started_at >= ?", now.AddDate(0, -3, 0))
	case "6mo":
		query = query.Where("started_at >= ?", now.AddDate(0, -6, 0))
	case "1y":
		query = query.Where("started_at >= ?", now.AddDate(-1, 0, 0))
	}

	var runs []db.SessionRun
	err := query.Find(&runs).Error
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// Gather all run IDs and scheduled session IDs
	runIDs := make([]uint, 0, len(runs))
	ssIDs := make([]uint, 0, len(runs))
	for _, r := range runs {
		runIDs = append(runIDs, r.ID)
		ssIDs = append(ssIDs, r.ScheduledSessionID)
	}

	// Load scheduled sessions with their templates (including exercises for TotalCount)
	var scheduledSessions []db.ScheduledSession
	if len(ssIDs) > 0 {
		err = s.store.DB.
			Preload("SessionTemplate").
			Preload("SessionTemplate.Activities", func(tx *gorm.DB) *gorm.DB {
				return tx.Where("owner_id = ?", ownerID)
			}).
			Preload("SessionTemplate.Activities.Exercises", func(tx *gorm.DB) *gorm.DB {
				return tx.Where("owner_id = ? AND parent_exercise_id IS NULL", ownerID)
			}).
			Where("id IN ?", ssIDs).
			Find(&scheduledSessions).Error
		if err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	ssMap := map[uint]db.ScheduledSession{}
	for _, ss := range scheduledSessions {
		ssMap[ss.ID] = ss
	}

	// Count root exercises per template
	exerciseCountByTemplate := map[uint]int{}
	for _, ss := range scheduledSessions {
		if _, ok := exerciseCountByTemplate[ss.SessionTemplateID]; !ok {
			count := 0
			for _, act := range ss.SessionTemplate.Activities {
				count += len(act.Exercises)
			}
			exerciseCountByTemplate[ss.SessionTemplateID] = count
		}
	}

	// Load completions for all runs in one query
	var completions []db.RunExerciseCompletion
	if len(runIDs) > 0 {
		err = s.store.DB.
			Where("owner_id = ? AND run_id IN ?", ownerID, runIDs).
			Find(&completions).Error
		if err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	// Group completions by run ID
	completionsByRun := map[uint][]db.RunExerciseCompletion{}
	for _, c := range completions {
		completionsByRun[c.RunID] = append(completionsByRun[c.RunID], c)
	}

	// Bulk-load journal IDs for all runs.
	journalIDByRun, err := db.ListJournalIDsByRun(s.store.DB, ownerID, runIDs)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// Climbing analytics over ticks in the selected range.
	climbing, err := db.ClimbingAnalytics(s.store.DB, ownerID, runIDs)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	everClimbed, err := db.UserHasClimbingTicks(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// Build views and compute stats
	weekStart := mondayOfLocalDate(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var totalCompletedDuration time.Duration
	completedCount := 0
	thisWeekCount := 0
	thisMonthCount := 0

	// For streaks and weekly chart
	completedDays := map[string]bool{}
	completedDayCounts := map[string]int{}
	templateCounts := map[string]int{}
	templateColors := map[string]string{}
	weeklyBuckets := map[string]int{}
	weeklyColorVotes := map[string]map[string]int{}
	weeklyNameVotes := map[string]map[string]string{} // weekKey -> color -> templateName
	daySessions := map[string][]HeatmapSession{}

	views := make([]pages.HistoryRunView, 0, len(runs))
	for _, run := range runs {
		ss := ssMap[run.ScheduledSessionID]
		tplName := ss.SessionTemplate.Name
		if run.CustomName != "" {
			tplName = run.CustomName
		}
		color := normalizeTemplateColor(ss.SessionTemplate.Color)
		totalExercises := exerciseCountByTemplate[ss.SessionTemplateID]

		durationLabel := "In progress"
		if run.CompletedAt != nil {
			d := run.CompletedAt.Sub(run.StartedAt)
			durationLabel = formatDuration(d)
		}

		rc := completionsByRun[run.ID]
		completed := 0
		for _, c := range rc {
			if c.Status == db.RunStatusCompleted {
				completed++
			}
		}

		views = append(views, pages.HistoryRunView{
			ID:             run.ID,
			RunID:          run.ID,
			DateLabel:      run.StartedAt.Format("Jan 2, 2006"),
			RelativeDate:   relativeDate(run.StartedAt, now),
			MonthGroup:     run.StartedAt.Format("January 2006"),
			TemplateName:   tplName,
			Color:          color,
			DurationLabel:  durationLabel,
			Status:         run.Status,
			JournalEntryID: journalIDByRun[run.ID],
			CompletedCount: completed,
			TotalCount:     totalExercises,
		})

		if run.Status == db.RunStatusCompleted {
			completedCount++
			if run.CompletedAt != nil {
				totalCompletedDuration += run.CompletedAt.Sub(run.StartedAt)
			}
			if !run.StartedAt.Before(weekStart) {
				thisWeekCount++
			}
			if !run.StartedAt.Before(monthStart) {
				thisMonthCount++
			}

			dayKey := localDateKey(run.StartedAt)
			completedDays[dayKey] = true
			completedDayCounts[dayKey]++
			templateCounts[tplName]++
			if _, ok := templateColors[tplName]; !ok {
				templateColors[tplName] = color
			}

			// Weekly bucket (ISO week Monday)
			monday := mondayOfLocalDate(run.StartedAt)
			weekKey := monday.Format("2006-01-02")
			weeklyBuckets[weekKey]++
			if weeklyColorVotes[weekKey] == nil {
				weeklyColorVotes[weekKey] = map[string]int{}
			}
			weeklyColorVotes[weekKey][color]++
			if weeklyNameVotes[weekKey] == nil {
				weeklyNameVotes[weekKey] = map[string]string{}
			}
			weeklyNameVotes[weekKey][color] = tplName
			daySessions[dayKey] = append(daySessions[dayKey], HeatmapSession{Name: tplName, Color: color})
		}
	}

	// Compute streaks
	currentStreak, longestStreak := computeStreaks(completedDays, now)

	// Most used template
	mostUsedTemplate := ""
	mostUsedColor := ""
	mostUsedCount := 0
	for name, count := range templateCounts {
		if count > mostUsedCount {
			mostUsedCount = count
			mostUsedTemplate = name
			mostUsedColor = templateColors[name]
		}
	}

	// Build template breakdown sorted by count desc
	breakdown := make([]pages.TemplateBreakdownItem, 0, len(templateCounts))
	for name, count := range templateCounts {
		breakdown = append(breakdown, pages.TemplateBreakdownItem{
			TemplateName: name,
			Color:        templateColors[name],
			Count:        count,
		})
	}
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].Count > breakdown[j].Count
	})

	// Compute percentage bars relative to most-used template
	if len(breakdown) > 0 {
		maxCount := breakdown[0].Count
		for i := range breakdown {
			breakdown[i].Pct = breakdown[i].Count * 100 / maxCount
		}
	}

	// Build weekly chart data (last 12 weeks)
	weeklyData := make([]WeeklyDataPoint, 12)
	chartStart := mondayOfLocalDate(now).AddDate(0, 0, -11*7)
	maxWeekly := 0
	for i := 0; i < 12; i++ {
		monday := chartStart.AddDate(0, 0, i*7)
		key := monday.Format("2006-01-02")
		d := monday.Day()
		weeklyData[i] = WeeklyDataPoint{
			WeekLabel: monday.Format("Jan") + " " + fmt.Sprintf("%d", d),
			Count:     weeklyBuckets[key],
		}
		if weeklyBuckets[key] > maxWeekly {
			maxWeekly = weeklyBuckets[key]
		}
	}
	chartJSON, _ := json.Marshal(weeklyData)

	// Build CSS-based weekly trend bars
	weeklyTrend := make([]pages.WeeklyTrendItem, 12)
	for i, wp := range weeklyData {
		pct := 0
		if maxWeekly > 0 && wp.Count > 0 {
			pct = wp.Count * 100 / maxWeekly
			if pct < 5 {
				pct = 5
			}
		}
		weekKey := chartStart.AddDate(0, 0, i*7).Format("2006-01-02")

		// Build stacked color segments (sorted by count desc for stable ordering)
		var segments []pages.WeeklyColorSegment
		if votes := weeklyColorVotes[weekKey]; len(votes) > 0 && wp.Count > 0 {
			// Collect and sort colors by count descending
			type colorCount struct {
				color string
				n     int
			}
			cc := make([]colorCount, 0, len(votes))
			for c, n := range votes {
				cc = append(cc, colorCount{c, n})
			}
			sort.Slice(cc, func(a, b int) bool { return cc[a].n > cc[b].n })
			remaining := 100
			for j, cv := range cc {
				var segPct int
				if j == len(cc)-1 {
					segPct = remaining
				} else {
					segPct = cv.n * 100 / wp.Count
					remaining -= segPct
				}
				segName := ""
				if names := weeklyNameVotes[weekKey]; names != nil {
					segName = names[cv.color]
				}
				segments = append(segments, pages.WeeklyColorSegment{Color: cv.color, Name: segName, Count: cv.n, HeightPct: segPct})
			}
		}

		weeklyTrend[i] = pages.WeeklyTrendItem{
			Label:     wp.WeekLabel,
			Count:     wp.Count,
			HeightPct: pct,
			Segments:  segments,
		}
	}

	// Build heatmap data (last 365 days from all completed runs)
	heatmapStart := localDate(now).AddDate(0, 0, -364)
	heatmapDays := make([]HeatmapDay, 365)
	for i := 0; i < 365; i++ {
		d := heatmapStart.AddDate(0, 0, i)
		key := localDateKey(d)
		heatmapDays[i] = HeatmapDay{Date: key, Count: completedDayCounts[key], Sessions: daySessions[key]}
	}
	heatmapJSON, _ := json.Marshal(heatmapDays)

	totalTimeLabel := formatDuration(totalCompletedDuration)
	avgLabel := "—"
	if completedCount > 0 {
		avgLabel = formatDuration(totalCompletedDuration / time.Duration(completedCount))
	}

	// Serialize weekly trend with segment details for JS tooltips
	type weeklySegJSON struct {
		Name  string `json:"name"`
		Color string `json:"color"`
		Count int    `json:"count"`
	}
	type weeklyItemJSON struct {
		Label    string          `json:"label"`
		Count    int             `json:"count"`
		Segments []weeklySegJSON `json:"segments"`
	}
	weeklyItems := make([]weeklyItemJSON, len(weeklyTrend))
	for i, wt := range weeklyTrend {
		segs := make([]weeklySegJSON, len(wt.Segments))
		for j, s := range wt.Segments {
			segs[j] = weeklySegJSON{Name: s.Name, Color: s.Color, Count: s.Count}
		}
		weeklyItems[i] = weeklyItemJSON{Label: wt.Label, Count: wt.Count, Segments: segs}
	}
	weeklyTrendJSON, _ := json.Marshal(weeklyItems)

	s.pages.History(w, pages.HistoryParams{
		Base:         pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		HistoryRuns:  views,
		HistoryRange: rangeParam,
		WeeklyTrend:  weeklyTrend,
		Climbing:     buildClimbingView(climbing, everClimbed),
		HistoryStats: pages.HistoryStatsView{
			TotalRuns:         completedCount,
			TotalTimeLabel:    totalTimeLabel,
			AvgDurationLabel:  avgLabel,
			ThisWeekCount:     thisWeekCount,
			ThisMonthCount:    thisMonthCount,
			CurrentStreak:     currentStreak,
			LongestStreak:     longestStreak,
			MostUsedTemplate:  mostUsedTemplate,
			MostUsedColor:     mostUsedColor,
			WeeklyChartJSON:   template.JS(chartJSON),
			WeeklyTrendJSON:   template.JS(weeklyTrendJSON),
			TemplateBreakdown: breakdown,
			HeatmapJSON:       template.JS(heatmapJSON),
		},
	})
}

func buildClimbingView(a db.ClimbingAnalyticsResult, everClimbed bool) pages.ClimbingAnalyticsView {
	v := pages.ClimbingAnalyticsView{HasEverClimbed: everClimbed}
	if !a.HasData {
		return v
	}
	v.HasData = true
	v.TotalClimbs = a.TotalClimbs
	v.SessionCount = a.SessionCount
	v.SendRate = a.SendRate
	v.HardestBoulder = a.Boulder.HardestSent
	v.HardestRoute = a.Route.HardestSent
	if a.Boulder.Ticks > 0 {
		v.Disciplines = append(v.Disciplines, climbingDisciplineView("Boulder", a.Boulder))
	}
	if a.Route.Ticks > 0 {
		v.Disciplines = append(v.Disciplines, climbingDisciplineView("Routes", a.Route))
	}
	v.HasSplits = a.HasIndoorOutdoor
	v.IndoorPct = a.IndoorPct
	v.OutdoorPct = a.OutdoorPct
	v.HasBoardSplit = a.HasBoardSplit
	v.CommercialPct = a.CommercialPct
	v.BoardPct = a.BoardPct
	return v
}

func climbingDisciplineView(label string, d db.ClimbingDisciplineStats) pages.ClimbingDisciplineView {
	dv := pages.ClimbingDisciplineView{Label: label, MoreGrades: d.MoreGrades}
	for _, row := range d.Pyramid {
		r := pages.ClimbingGradeRow{Grade: row.Grade, Sent: row.Sent, Total: row.Total}
		if d.MaxTotal > 0 {
			r.SentPct = row.Sent * 100 / d.MaxTotal
			r.AttemptPct = (row.Total - row.Sent) * 100 / d.MaxTotal
		}
		dv.Rows = append(dv.Rows, r)
	}
	return dv
}

func computeStreaks(completedDays map[string]bool, now time.Time) (current int, longest int) {
	if len(completedDays) == 0 {
		return 0, 0
	}

	// Start from today, walk backward for current streak
	d := localDate(now)
	// Allow starting from today or yesterday
	if !completedDays[localDateKey(d)] {
		d = d.AddDate(0, 0, -1)
	}
	for completedDays[localDateKey(d)] {
		current++
		d = d.AddDate(0, 0, -1)
	}

	// Find longest streak: collect all days, sort, walk forward
	days := make([]time.Time, 0, len(completedDays))
	for key := range completedDays {
		if t, err := time.ParseInLocation("2006-01-02", key, now.Location()); err == nil {
			days = append(days, t)
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })

	streak := 1
	for i := 1; i < len(days); i++ {
		diff := days[i].Sub(days[i-1]).Hours() / 24
		if diff <= 1.5 { // same or next day (accounts for DST)
			streak++
		} else {
			if streak > longest {
				longest = streak
			}
			streak = 1
		}
	}
	if streak > longest {
		longest = streak
	}
	return current, longest
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func relativeDate(t time.Time, now time.Time) string {
	tDate := localDate(t)
	today := localDate(now)
	yesterday := today.AddDate(0, 0, -1)
	switch {
	case tDate.Equal(today):
		return "Today"
	case tDate.Equal(yesterday):
		return "Yesterday"
	default:
		return t.Format("Mon, Jan 2")
	}
}
