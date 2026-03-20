package web

import "testing"

func TestNormalizeTemplateColor(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"#fff", "#fff"},
		{"#ABCDEF", "#abcdef"},
		{"#12ab34", "#12ab34"},
		{"12ab34", ""},
		{"#12ab3", ""},
		{"#12ab3z", ""},
	}

	for _, tc := range cases {
		got := normalizeTemplateColor(tc.in)
		if got != tc.want {
			t.Errorf("normalizeTemplateColor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
