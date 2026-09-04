package pages

import "testing"

// A logged session should read the way it was run — warm-up, main work, cooldown — not as
// one flat list. The block travels as text on each exercise, because an Activity belongs to
// a template and cannot belong to a run.
func TestGroupExercisesIntoBlocks(t *testing.T) {
	t.Run("groups consecutive exercises under their block", func(t *testing.T) {
		got := GroupExercisesIntoBlocks([]SessionExerciseSummaryView{
			{Name: "Row", BlockType: "warmup", BlockName: "Warm Up"},
			{Name: "Band Prep", BlockType: "warmup", BlockName: "Warm Up"},
			{Name: "Pull-ups", BlockType: "activity", BlockName: "Main"},
		})
		if len(got) != 2 {
			t.Fatalf("want 2 blocks, got %d", len(got))
		}
		if got[0].Name != "Warm Up" || len(got[0].Exercises) != 2 {
			t.Errorf("first block wrong: %q with %d exercises", got[0].Name, len(got[0].Exercises))
		}
		if got[1].Name != "Main" || len(got[1].Exercises) != 1 {
			t.Errorf("second block wrong: %q with %d exercises", got[1].Name, len(got[1].Exercises))
		}
	})

	// An open session or a manual entry has no blocks. Returning nil lets the page fall
	// back to the flat list rather than render one nameless group.
	t.Run("returns nil when nothing carries a block", func(t *testing.T) {
		got := GroupExercisesIntoBlocks([]SessionExerciseSummaryView{{Name: "Ad hoc"}, {Name: "Also ad hoc"}})
		if got != nil {
			t.Fatalf("want nil so the caller falls back, got %d blocks", len(got))
		}
	})

	t.Run("returns nil for an empty list", func(t *testing.T) {
		if GroupExercisesIntoBlocks(nil) != nil {
			t.Fatal("want nil for no exercises")
		}
	})

	// The same block name appearing again later is a real second block, not a continuation.
	// Order is what the athlete did, so it is never rearranged.
	t.Run("keeps a repeated block name as two blocks", func(t *testing.T) {
		got := GroupExercisesIntoBlocks([]SessionExerciseSummaryView{
			{Name: "a", BlockName: "Climbing"},
			{Name: "b", BlockName: "Rest"},
			{Name: "c", BlockName: "Climbing"},
		})
		if len(got) != 3 {
			t.Fatalf("want 3 blocks in run order, got %d", len(got))
		}
		if got[0].Name != "Climbing" || got[1].Name != "Rest" || got[2].Name != "Climbing" {
			t.Errorf("order changed: %q %q %q", got[0].Name, got[1].Name, got[2].Name)
		}
	})

	// A block with a type but no name still groups; the page falls back to the type as a
	// heading. Half the owner's backfilled rows are in this state, because prune had
	// already hard-deleted the activity they came from.
	t.Run("groups on type alone", func(t *testing.T) {
		got := GroupExercisesIntoBlocks([]SessionExerciseSummaryView{
			{Name: "a", BlockType: "activity"},
			{Name: "b", BlockType: "activity"},
		})
		if len(got) != 1 || got[0].Type != "activity" || len(got[0].Exercises) != 2 {
			t.Fatalf("want one activity block of 2, got %+v", got)
		}
	})
}
