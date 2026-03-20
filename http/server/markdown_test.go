package web

import "testing"

func TestMarkdownToHTML(t *testing.T) {
	if got := string(markdownToHTML("   ")); got != "" {
		t.Fatalf("empty markdown should be empty HTML, got %q", got)
	}

	got := string(markdownToHTML("**bold**"))
	if got == "" {
		t.Fatal("markdownToHTML returned empty for non-empty markdown")
	}
	if got != "<p><strong>bold</strong></p>\n" {
		t.Fatalf("unexpected markdown HTML: %q", got)
	}
}

func TestMarkdownToHTMLString(t *testing.T) {
	got := markdownToHTMLString("# Title")
	if got != "<h1>Title</h1>\n" {
		t.Fatalf("markdownToHTMLString mismatch: %q", got)
	}
}
