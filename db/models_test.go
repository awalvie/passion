package db

import "testing"

func TestPrimaryMedia(t *testing.T) {
	if got := PrimaryMedia(nil); got != nil {
		t.Fatalf("PrimaryMedia(nil) = %#v, want nil", got)
	}

	media := []ExerciseMedia{
		{VideoURL: "v2", ThumbnailURL: "t2", OrderIndex: 2},
		{VideoURL: "v0", ThumbnailURL: "t0", OrderIndex: 0},
		{VideoURL: "v1", ThumbnailURL: "t1", OrderIndex: 1},
	}
	got := PrimaryMedia(media)
	if got == nil {
		t.Fatal("PrimaryMedia returned nil")
	}
	if got.OrderIndex != 0 || got.VideoURL != "v0" {
		t.Fatalf("PrimaryMedia picked wrong media: %+v", *got)
	}
}

func TestPrimaryMediaTieKeepsFirst(t *testing.T) {
	media := []ExerciseMedia{
		{VideoURL: "first", OrderIndex: 1},
		{VideoURL: "second", OrderIndex: 1},
	}
	got := PrimaryMedia(media)
	if got == nil {
		t.Fatal("PrimaryMedia returned nil")
	}
	if got.VideoURL != "first" {
		t.Fatalf("PrimaryMedia tie-break mismatch: got %q", got.VideoURL)
	}
}
