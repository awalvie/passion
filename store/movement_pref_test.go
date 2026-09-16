package store

import (
	"context"
	"testing"
)

func TestTheSlotBeatsYourOwnNumberAndYoursBeatsTheDefault(t *testing.T) {
	m := Content{DSets: num(3), DReps: num(8), DWeightKg: ptrF(20)}
	pref := &MovementPref{WeightKg: ptrF(25)}
	slot := &Dose{WeightKg: ptrF(30)}

	if got := ResolveDose(m, nil, nil); *got.WeightKg != 20 {
		t.Errorf("with nothing set, want the movement's 20, got %v", *got.WeightKg)
	}
	if got := ResolveDose(m, pref, nil); *got.WeightKg != 25 {
		t.Errorf("with a pref, want 25, got %v", *got.WeightKg)
	}
	// A session that ramps 60/70/80% must not collapse into one number.
	if got := ResolveDose(m, pref, slot); *got.WeightKg != 30 {
		t.Errorf("with a slot, want the slot's 30, got %v", *got.WeightKg)
	}
}

func TestAPrefOnOneNumberLeavesTheRest(t *testing.T) {
	m := Content{DSets: num(3), DReps: num(8), DWeightKg: ptrF(20)}
	got := ResolveDose(m, &MovementPref{WeightKg: ptrF(25)}, nil)

	if got.Sets == nil || *got.Sets != 3 {
		t.Error("a pref on the weight wiped the movement's sets")
	}
	if got.Reps == nil || *got.Reps != 8 {
		t.Error("a pref on the weight wiped the movement's reps")
	}
}

// A ladder replaces reps, so a slot carrying per-set numbers has to clear them.
func TestASlotLadderReplacesTheReps(t *testing.T) {
	m := Content{DSets: num(3), DReps: num(8)}
	got := ResolveDose(m, nil, &Dose{PerSet: []SetEntry{{Reps: num(3)}, {Reps: num(6)}}})

	if got.Reps != nil {
		t.Errorf("reps survived a per-set slot: %v", *got.Reps)
	}
	if len(got.PerSet) != 2 {
		t.Errorf("the ladder did not carry through: %d entries", len(got.PerSet))
	}
}

func TestAPrefOnTheAppsMovementIsKeptAndReadBack(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		m := firstOfKind(t, s, 1, KindMovement)
		if m.Mine() {
			t.Fatal("this test needs a movement the app ships")
		}
		if err := s.SetMovementPref(ctx, 1, m.ID, Dose{WeightKg: ptrF(25)}); err != nil {
			t.Fatal(err)
		}

		prefs, err := s.MovementPrefs(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		p, ok := prefs[m.ID]
		if !ok {
			t.Fatal("the pref was not stored")
		}
		if p.WeightKg == nil || *p.WeightKg != 25 {
			t.Errorf("the pref reads back as %v", p.WeightKg)
		}

		// The movement itself is untouched, which is the whole point of an override.
		again, err := s.ContentByID(ctx, 1, m.ID)
		if err != nil {
			t.Fatal(err)
		}
		if again.AuthorID != nil {
			t.Error("setting a pref took ownership of the app's row")
		}
	})
}

func TestClearingEveryNumberRemovesThePref(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		m := firstOfKind(t, s, 1, KindMovement)
		if err := s.SetMovementPref(ctx, 1, m.ID, Dose{WeightKg: ptrF(25)}); err != nil {
			t.Fatal(err)
		}
		if err := s.SetMovementPref(ctx, 1, m.ID, Dose{}); err != nil {
			t.Fatal(err)
		}

		prefs, err := s.MovementPrefs(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := prefs[m.ID]; ok {
			t.Error("a pref with no numbers was kept")
		}
	})
}

func ptrF(f float64) *float64 { return &f }
