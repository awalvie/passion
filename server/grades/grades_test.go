package grades

import "testing"

func TestRank(t *testing.T) {
	font, ok := Find("font")
	if !ok {
		t.Fatal("no font scale")
	}
	easy, _ := font.Rank("6c")
	hard, _ := font.Rank("7a")
	if easy >= hard || easy < 1 {
		t.Fatalf("6c ranks %d and 7a %d, want 6c lower and both from 1", easy, hard)
	}
	if _, ok := font.Rank("V4"); ok {
		t.Fatal("V4 ranked on the font scale")
	}
	if _, ok := Find("ewbank"); ok {
		t.Fatal("found a system that does not exist")
	}
}

// A repeated grade would give two ranks to one name.
func TestEveryGradeIsListedOnce(t *testing.T) {
	for _, s := range Scales {
		seen := map[string]bool{}
		for _, g := range s.Grades {
			if seen[g] {
				t.Errorf("%s lists %s twice", s.System, g)
			}
			seen[g] = true
		}
	}
}
