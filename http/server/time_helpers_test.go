package web

import (
	"testing"
	"time"
)

func TestLocalDateAndKey(t *testing.T) {
	loc := time.FixedZone("UTC+2", 2*60*60)
	in := time.Date(2026, time.April, 28, 15, 4, 5, 0, loc)

	got := localDate(in)
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Fatalf("localDate did not clear time: got %v", got)
	}
	if !got.Equal(time.Date(2026, time.April, 28, 0, 0, 0, 0, loc)) {
		t.Fatalf("localDate mismatch: got %v", got)
	}
	if key := localDateKey(in); key != "2026-04-28" {
		t.Fatalf("localDateKey = %q, want 2026-04-28", key)
	}
}

func TestMondayOfLocalDate(t *testing.T) {
	loc := time.FixedZone("UTC", 0)
	wed := time.Date(2026, time.April, 29, 9, 0, 0, 0, loc)
	sun := time.Date(2026, time.May, 3, 22, 0, 0, 0, loc)

	want := time.Date(2026, time.April, 27, 0, 0, 0, 0, loc)
	if got := mondayOfLocalDate(wed); !got.Equal(want) {
		t.Fatalf("mondayOfLocalDate(wed) = %v, want %v", got, want)
	}
	if got := mondayOfLocalDate(sun); !got.Equal(want) {
		t.Fatalf("mondayOfLocalDate(sun) = %v, want %v", got, want)
	}
}
