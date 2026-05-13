package web

import (
	"bytes"
	"html"
	"html/template"
	"net/http"
	"strings"

	"github.com/yuin/goldmark"
)

var markdownEngine = goldmark.New()

// markdownToHTML converts Markdown to trusted HTML for templates (goldmark; no raw HTML by default).
func markdownToHTML(s string) template.HTML {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := markdownEngine.Convert([]byte(s), &buf); err != nil {
		return template.HTML(html.EscapeString(s))
	}
	return template.HTML(buf.String())
}

func markdownToHTMLString(s string) string {
	var buf bytes.Buffer
	if err := markdownEngine.Convert([]byte(s), &buf); err != nil {
		return html.EscapeString(s)
	}
	return buf.String()
}

func (s *Server) handleMarkdownPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}
	content := r.FormValue("content")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	inner := strings.TrimSpace(markdownToHTMLString(content))
	if inner == "" {
		w.Write([]byte(`<p class="text-xs muted m-0">Nothing to preview.</p>`))
		return
	}
	w.Write([]byte(`<div class="markdown-body">` + inner + `</div>`))
}
