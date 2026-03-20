package web

import "strings"

// normalizeTemplateColor returns a safe lowercase #RGB or #RRGGBB string, or "" if invalid/empty.
func normalizeTemplateColor(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return ""
	}
	if len(s) != 7 && len(s) != 4 {
		return ""
	}
	if s[0] != '#' {
		return ""
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return ""
	}
	return s
}
