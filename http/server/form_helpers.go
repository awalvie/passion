package web

import (
	"net/http"
	"strconv"
	"strings"
)

// formInt parses a form value as an integer, returning 0 for empty or invalid values.
func formInt(r *http.Request, key string) int {
	v := strings.TrimSpace(r.FormValue(key))
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

// formFloat parses a form value as a float64, returning 0 for empty or invalid values.
func formFloat(r *http.Request, key string) float64 {
	v := strings.TrimSpace(r.FormValue(key))
	if v == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(v, 64)
	return f
}
