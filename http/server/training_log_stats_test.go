package web

import (
	"testing"
	"time"

	"passion/pages"
)

// TestBuildTrainingLogStats_ExcludesStandaloneNotesFromEveryBucket guards the
// stats-partition behavior: a standalone quick note (IsStandalone == true) must
// not move TotalSessions, ThisWeek, ThisMonth, CurrentStreak, or the wellness
// averages — those numbers must reflect real training sessions only. Without the
// `if e.IsStandalone { continue }` guard, this test fails because the lone
// standalone entry would count as 1 session, contribute to the streak, and pull
// the averages toward its journal fields.
func TestBuildTrainingLogStats_ExcludesStandaloneNotesFromEveryBucket(t *testing.T) {
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.Local) // Saturday
	today := localDate(now)

	entries := []pages.TrainingLogEntryView{
		{
			IsStandalone: true,
			SortTime:     today,
			SleepScore:   5,
			Energy:       5,
			RPE:          10,
			Focus:        "Strength",
			Location:     "Indoor",
		},
	}

	stats := buildTrainingLogStats(entries, now)

	if stats.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0 (standalone note must not count as a session)", stats.TotalSessions)
	}
	if stats.ThisWeek != 0 {
		t.Errorf("ThisWeek = %d, want 0", stats.ThisWeek)
	}
	if stats.ThisMonth != 0 {
		t.Errorf("ThisMonth = %d, want 0", stats.ThisMonth)
	}
	if stats.CurrentStreak != 0 {
		t.Errorf("CurrentStreak = %d, want 0 (standalone note must not seed the streak)", stats.CurrentStreak)
	}
	if stats.AvgSleep != "—" {
		t.Errorf("AvgSleep = %q, want %q (no session data to average)", stats.AvgSleep, "—")
	}
	if stats.AvgEnergy != "—" {
		t.Errorf("AvgEnergy = %q, want %q", stats.AvgEnergy, "—")
	}
	if stats.AvgRPE != "—" {
		t.Errorf("AvgRPE = %q, want %q", stats.AvgRPE, "—")
	}
	if stats.TopFocus != "" {
		t.Errorf("TopFocus = %q, want empty (standalone note's focus must not be counted)", stats.TopFocus)
	}
	if stats.IndoorCount != 0 || stats.OutdoorCount != 0 {
		t.Errorf("IndoorCount/OutdoorCount = %d/%d, want 0/0", stats.IndoorCount, stats.OutdoorCount)
	}
}

// TestBuildTrainingLogStats_SessionsCountedAlongsideExcludedNotes guards that
// mixing real sessions with standalone notes still counts sessions correctly:
// TotalSessions must equal only the non-standalone entries, and the averages must
// derive solely from the session's journal fields, not the note's.
func TestBuildTrainingLogStats_SessionsCountedAlongsideExcludedNotes(t *testing.T) {
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.Local) // Saturday
	today := localDate(now)

	entries := []pages.TrainingLogEntryView{
		{
			IsStandalone: false,
			SortTime:     today,
			SleepScore:   4,
			Energy:       3,
			RPE:          7,
			Focus:        "Endurance",
			Location:     "Outdoor",
		},
		{
			IsStandalone: true,
			SortTime:     today,
			SleepScore:   1, // would drag the average down if counted
			Energy:       1,
			RPE:          1,
			Focus:        "Strength",
			Location:     "Indoor",
		},
	}

	stats := buildTrainingLogStats(entries, now)

	if stats.TotalSessions != 1 {
		t.Fatalf("TotalSessions = %d, want 1 (only the non-standalone entry counts)", stats.TotalSessions)
	}
	if stats.ThisWeek != 1 {
		t.Errorf("ThisWeek = %d, want 1", stats.ThisWeek)
	}
	if stats.CurrentStreak != 1 {
		t.Errorf("CurrentStreak = %d, want 1 (streak seeded by the real session only)", stats.CurrentStreak)
	}
	if stats.AvgSleep != "4 / 5" {
		t.Errorf("AvgSleep = %q, want %q (standalone note's SleepScore=1 must not be averaged in)", stats.AvgSleep, "4 / 5")
	}
	if stats.TopFocus != "Endurance" {
		t.Errorf("TopFocus = %q, want %q (standalone note's focus must not compete)", stats.TopFocus, "Endurance")
	}
	if stats.IndoorCount != 0 || stats.OutdoorCount != 1 {
		t.Errorf("IndoorCount/OutdoorCount = %d/%d, want 0/1", stats.IndoorCount, stats.OutdoorCount)
	}
}

// TestBuildTrainingLogStats_TotalSessionsIsSessionCountNotEntryLen guards the
// contract change from len(entries) to a dedicated sessionCount: with several
// standalone notes and zero real sessions, TotalSessions must be 0, not
// len(entries).
func TestBuildTrainingLogStats_TotalSessionsIsSessionCountNotEntryLen(t *testing.T) {
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.Local)

	entries := []pages.TrainingLogEntryView{
		{IsStandalone: true, SortTime: now},
		{IsStandalone: true, SortTime: now.AddDate(0, 0, -1)},
		{IsStandalone: true, SortTime: now.AddDate(0, 0, -2)},
	}

	stats := buildTrainingLogStats(entries, now)
	if stats.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0; got len(entries)=%d instead of a real session count", stats.TotalSessions, len(entries))
	}
}
