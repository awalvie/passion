package web

import "testing"

func TestYoutubeEmbedURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"https://example.com/x.mp4", ""},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "https://www.youtube.com/embed/dQw4w9WgXcQ?rel=0"},
		{"https://youtube.com/watch?v=abc123def45&feature=share", "https://www.youtube.com/embed/abc123def45?rel=0"},
		{"https://youtu.be/dQw4w9WgXcQ", "https://www.youtube.com/embed/dQw4w9WgXcQ?rel=0"},
		{"https://www.youtube.com/shorts/xyzABC123", "https://www.youtube.com/embed/xyzABC123?rel=0"},
		{"https://m.youtube.com/watch?v=ZZZ", "https://www.youtube.com/embed/ZZZ?rel=0"},
		{"https://www.youtube.com/embed/AAA", "https://www.youtube.com/embed/AAA?rel=0"},
	}
	for _, tc := range cases {
		got := youtubeEmbedURL(tc.in)
		if got != tc.want {
			t.Errorf("youtubeEmbedURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
