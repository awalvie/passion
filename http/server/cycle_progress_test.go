package web

import (
	"strings"
	"testing"
	"time"

	"passion/db"
	"passion/pages"
)

// buildProgressRows makes week rows with a given plan/done shape, marking the week that
// contains today as current — the shape cycleProgressLine reads.
func buildProgressRows(gridStart, today time.Time, weeks int, plan [][]bool, done [][]bool) []pages.CycleWeekRowView {
	rows := make([]pages.CycleWeekRowView, 0, weeks)
	for w := 0; w < weeks; w++ {
		weekStart := gridStart.AddDate(0, 0, w*7)
		cells := make([]pages.CycleDayCellView, 0, 7)
		planned, completed := 0, 0
		for d := 0; d < 7; d++ {
			day := weekStart.AddDate(0, 0, d)
			c := pages.CycleDayCellView{DateKey: localDateKey(day)}
			if w < len(plan) && d < len(plan[w]) && plan[w][d] {
				c.HasSession = true
				planned++
				if w < len(done) && d < len(done[w]) && done[w][d] {
					c.HasCompletedRun = true
					completed++
				} else if day.Before(today) {
					c.IsMissed = true
				}
			}
			cells = append(cells, c)
		}
		rows = append(rows, pages.CycleWeekRowView{
			WeekNumber: w + 1, Cells: cells, Planned: planned, Done: completed,
			IsCurrent: !today.Before(weekStart) && today.Before(weekStart.AddDate(0, 0, 7)),
		})
	}
	return rows
}

func totals(rows []pages.CycleWeekRowView) (planned, done int) {
	for _, r := range rows {
		planned += r.Planned
		done += r.Done
	}
	return
}

// TestCycleProgressLine_NotStarted guards the state the old static chip never had to
// handle: a cycle that has not begun must not claim to be in week 1.
func TestCycleProgressLine_NotStarted(t *testing.T) {
	today := localDate(time.Date(2026, 8, 27, 12, 0, 0, 0, time.Local))
	gridStart := mondayOfLocalDate(today).AddDate(0, 0, 7) // next Monday
	cycle := db.TrainingCycle{Weeks: 4, StartDate: gridStart}
	gridEnd := gridStart.AddDate(0, 0, 4*7-1)

	plan := [][]bool{{true, false, true, false, false, false, false}}
	rows := buildProgressRows(gridStart, today, 4, plan, nil)
	p, d := totals(rows)

	got := cycleProgressLine(cycle, gridStart, gridEnd, today, rows, p, d)
	if !strings.HasPrefix(got, "Starts ") {
		t.Errorf("progress = %q, want it to start with %q", got, "Starts ")
	}
	if strings.Contains(got, "Week 1 of") {
		t.Errorf("progress = %q, must not claim a week for a cycle that has not begun", got)
	}
	if !strings.Contains(got, "2 sessions planned") {
		t.Errorf("progress = %q, want the planned count", got)
	}
}

// TestCycleProgressLine_Finished guards the tail state: past the last week, report the
// tally rather than pinning the owner to week 4 forever.
func TestCycleProgressLine_Finished(t *testing.T) {
	today := localDate(time.Date(2026, 8, 27, 12, 0, 0, 0, time.Local))
	gridStart := mondayOfLocalDate(today).AddDate(0, 0, -5*7)
	gridEnd := gridStart.AddDate(0, 0, 4*7-1)
	cycle := db.TrainingCycle{Weeks: 4, StartDate: gridStart}

	plan := [][]bool{{true, false, true, false, false, false, false}}
	done := [][]bool{{true, false, false, false, false, false, false}}
	rows := buildProgressRows(gridStart, today, 4, plan, done)
	p, d := totals(rows)

	got := cycleProgressLine(cycle, gridStart, gridEnd, today, rows, p, d)
	if !strings.HasPrefix(got, "Finished · ") {
		t.Errorf("progress = %q, want it to start with %q", got, "Finished · ")
	}
	if !strings.Contains(got, "1 of 2 sessions done") {
		t.Errorf("progress = %q, want the final tally", got)
	}
	if strings.Contains(got, "left this week") {
		t.Errorf("progress = %q, a finished cycle has nothing left", got)
	}
}

// TestCycleProgressLine_InFlight covers the everyday case, including that a past day
// with no run counts as missed rather than "left this week" — calling it left would
// overstate what is still doable.
func TestCycleProgressLine_InFlight(t *testing.T) {
	// A Thursday, so the current week has Mon/Wed behind and Fri/Sat ahead.
	today := localDate(time.Date(2026, 8, 27, 12, 0, 0, 0, time.Local))
	if today.Weekday() != time.Thursday {
		t.Fatalf("test setup: %v is not a Thursday", today.Weekday())
	}
	gridStart := mondayOfLocalDate(today)
	gridEnd := gridStart.AddDate(0, 0, 4*7-1)
	cycle := db.TrainingCycle{Weeks: 4, StartDate: gridStart}

	// Week 1: Mon, Wed, Fri, Sat planned. Mon done; Wed missed; Fri and Sat still ahead.
	plan := [][]bool{{true, false, true, false, true, true, false}}
	done := [][]bool{{true, false, false, false, false, false, false}}
	rows := buildProgressRows(gridStart, today, 4, plan, done)
	p, d := totals(rows)

	got := cycleProgressLine(cycle, gridStart, gridEnd, today, rows, p, d)
	if !strings.Contains(got, "Week 1 of 4") {
		t.Errorf("progress = %q, want %q", got, "Week 1 of 4")
	}
	if !strings.Contains(got, "1 of 4 sessions done") {
		t.Errorf("progress = %q, want %q", got, "1 of 4 sessions done")
	}
	if !strings.Contains(got, "2 left this week") {
		t.Errorf("progress = %q, want 2 left (Fri, Sat) — the missed Wed must not count", got)
	}
}

// TestCycleProgressLine_SingularWording guards the copy at n=1, where naive pluralising
// produces "1 sessions".
func TestCycleProgressLine_SingularWording(t *testing.T) {
	today := localDate(time.Date(2026, 8, 27, 12, 0, 0, 0, time.Local))
	gridStart := mondayOfLocalDate(today)
	gridEnd := gridStart.AddDate(0, 0, 6)
	cycle := db.TrainingCycle{Weeks: 1, StartDate: gridStart}

	plan := [][]bool{{true, false, false, false, false, false, false}}
	done := [][]bool{{true, false, false, false, false, false, false}}
	rows := buildProgressRows(gridStart, today, 1, plan, done)
	p, d := totals(rows)

	got := cycleProgressLine(cycle, gridStart, gridEnd, today, rows, p, d)
	if strings.Contains(got, "1 sessions") {
		t.Errorf("progress = %q, want singular wording", got)
	}
	if !strings.Contains(got, "1 of 1 session done") {
		t.Errorf("progress = %q, want %q", got, "1 of 1 session done")
	}
}

// TestCycleProgressLine_NoneLeftThisWeekOmitsClause guards that a fully-done week does
// not append "0 left this week".
func TestCycleProgressLine_NoneLeftThisWeekOmitsClause(t *testing.T) {
	today := localDate(time.Date(2026, 8, 27, 12, 0, 0, 0, time.Local))
	gridStart := mondayOfLocalDate(today)
	gridEnd := gridStart.AddDate(0, 0, 4*7-1)
	cycle := db.TrainingCycle{Weeks: 4, StartDate: gridStart}

	plan := [][]bool{{true, false, false, false, false, false, false}}
	done := [][]bool{{true, false, false, false, false, false, false}}
	rows := buildProgressRows(gridStart, today, 4, plan, done)
	p, d := totals(rows)

	got := cycleProgressLine(cycle, gridStart, gridEnd, today, rows, p, d)
	if strings.Contains(got, "left this week") {
		t.Errorf("progress = %q, want no leftover clause when nothing is left", got)
	}
}
