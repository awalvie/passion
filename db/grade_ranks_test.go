package db

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The Go ranks and the chip strip in run_ticks.html are two copies of one scale. A grade
// the strip offers but the ranks do not know cannot be compared, so it silently drops out
// of hardest-sent and the grade pyramid. This test is the only thing keeping them honest.
func TestGradeRanksMirrorTheChipStrip(t *testing.T) {
	raw, err := os.ReadFile("../templates/fragments/run_ticks.html")
	if err != nil {
		t.Skipf("cannot read the template: %v", err)
	}
	src := string(raw)

	listRe := regexp.MustCompile(`(?s)(font|v_scale|french|yds):\s*\[(.*?)\]`)
	quoted := regexp.MustCompile(`'([^']+)'`)

	found := 0
	for _, m := range listRe.FindAllStringSubmatch(src, -1) {
		system, body := m[1], m[2]
		found++
		for _, g := range quoted.FindAllStringSubmatch(body, -1) {
			grade := g[1]
			if _, ok := gradeRanks[grade]; !ok {
				t.Errorf("%s offers %q but gradeRanks does not know it, so it cannot be ranked",
					system, grade)
			}
		}
	}
	if found != 4 {
		t.Fatalf("expected 4 grade lists in the template, found %d", found)
	}
}

// The grades the owner has already logged must keep ranking, whatever the scale gains.
func TestLegacyGradesStillRank(t *testing.T) {
	for _, g := range []string{"5", "5+", "4", "4+"} {
		if _, ok := gradeRanks[g]; !ok {
			t.Errorf("%q no longer ranks; climbs logged on it drop out of hardest-sent", g)
		}
	}
}

func TestFrenchScaleHasTheLetteredFives(t *testing.T) {
	for _, g := range []string{"5a", "5b", "5c"} {
		if _, ok := gradeRanks[g]; !ok {
			t.Errorf("%q does not rank", g)
		}
	}
	// Ordered, and above the legacy grades they replace.
	if !(gradeRanks["5+"] < gradeRanks["5a"] &&
		gradeRanks["5a"] < gradeRanks["5b"] &&
		gradeRanks["5b"] < gradeRanks["5c"] &&
		gradeRanks["5c"] < gradeRanks["6a"]) {
		t.Errorf("the fives are out of order: 5+=%d 5a=%d 5b=%d 5c=%d 6a=%d",
			gradeRanks["5+"], gradeRanks["5a"], gradeRanks["5b"], gradeRanks["5c"], gradeRanks["6a"])
	}
}

// A guard on the mirroring rule itself: the two French and font lists in the template must
// still be the ones the ranks were built from, so a future edit to one is caught.
func TestBothScalesCarryTheLetteredFives(t *testing.T) {
	raw, err := os.ReadFile("../templates/fragments/run_ticks.html")
	if err != nil {
		t.Skipf("cannot read the template: %v", err)
	}
	for _, system := range []string{"font", "french"} {
		i := strings.Index(string(raw), system+": [")
		if i < 0 {
			t.Fatalf("%s list not found", system)
		}
		end := strings.Index(string(raw)[i:], "]")
		body := string(raw)[i : i+end]
		for _, g := range []string{"'5a'", "'5b'", "'5c'"} {
			if !strings.Contains(body, g) {
				t.Errorf("the %s chip strip is missing %s", system, g)
			}
		}
	}
}
