// Package grades holds the scales a climb's grade is logged in. The lists
// are V1's, in order, easiest first.
package grades

import "slices"

// Scale is one grading system. Boulder scales grade boulders, and route
// scales grade sport and trad routes.
type Scale struct {
	System  string
	Boulder bool
	Grades  []string
}

// Scales lists every system. A grade's rank is its place in its list, so a
// grade must never be removed or moved: old climbs sort by the rank they were
// logged with (rule 40). Add a new grade at the end, or recompute the ranks
// in a migration.
var Scales = []Scale{
	{System: "font", Boulder: true, Grades: []string{
		"3", "3+", "4", "4+", "5", "5+", "5a", "5b", "5c",
		"6a", "6a+", "6b", "6b+", "6c", "6c+",
		"7a", "7a+", "7b", "7b+", "7c", "7c+",
		"8a", "8a+", "8b", "8b+", "8c", "8c+", "9a",
	}},
	{System: "v", Boulder: true, Grades: []string{
		"VB", "V0", "V1", "V2", "V3", "V4", "V5", "V6", "V7", "V8",
		"V9", "V10", "V11", "V12", "V13", "V14", "V15", "V16",
	}},
	{System: "french", Grades: []string{
		"3", "3+", "4", "4+", "5", "5+", "5a", "5b", "5c",
		"6a", "6a+", "6b", "6b+", "6c", "6c+",
		"7a", "7a+", "7b", "7b+", "7c", "7c+",
		"8a", "8a+", "8b", "8b+", "8c", "8c+",
		"9a", "9a+", "9b", "9b+", "9c",
	}},
	{System: "yds", Grades: []string{
		"5.0", "5.1", "5.2", "5.3", "5.4", "5.5", "5.6", "5.7", "5.8", "5.9",
		"5.10a", "5.10b", "5.10c", "5.10d",
		"5.11a", "5.11b", "5.11c", "5.11d",
		"5.12a", "5.12b", "5.12c", "5.12d",
		"5.13a", "5.13b", "5.13c", "5.13d",
		"5.14a", "5.14b", "5.14c", "5.14d",
		"5.15a", "5.15b", "5.15c", "5.15d",
	}},
}

// Ungraded are V1's labels for climbing that has no grade, such as a lap. A
// climb with one of them is never a send (rule 39).
var Ungraded = []string{"Rainbow", "Traverse"}

// Find returns the scale for a system.
func Find(system string) (Scale, bool) {
	i := slices.IndexFunc(Scales, func(s Scale) bool { return s.System == system })
	if i < 0 {
		return Scale{}, false
	}
	return Scales[i], true
}

// Rank sorts a grade within its system, from 1.
func (s Scale) Rank(grade string) (int, bool) {
	i := slices.Index(s.Grades, grade)
	return i + 1, i >= 0
}
