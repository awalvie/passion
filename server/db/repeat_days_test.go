package db

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"
)

// The client works out a repeat's days with its own copy of the rule, for the
// preview, and reads the same cases.
func TestRepeatDays(t *testing.T) {
	raw, err := os.ReadFile("testdata/repeat_days.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Starts, From, Until, Unit string
		Ends                            *string
		Every                           int
		Weekdays                        []int
		Days                            []string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	day := func(s string) time.Time {
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			r := Repeat{Starts: day(c.Starts), Every: c.Every, Unit: c.Unit, Weekdays: c.Weekdays}
			if c.Ends != nil {
				ends := day(*c.Ends)
				r.Ends = &ends
			}
			var got []string
			for _, s := range r.slots(day(c.From), day(c.Until)) {
				got = append(got, s.LocalDate.Format(time.DateOnly))
			}
			if !slices.Equal(got, c.Days) {
				t.Fatalf("days %v, want %v", got, c.Days)
			}
		})
	}
}
