package db

import "testing"

func TestCheckVersion(t *testing.T) {
	cases := []struct {
		name string
		num  int
		ok   bool
	}{
		{"postgres 18.6", 180006, true},
		{"postgres 18.0", 180000, true},
		{"postgres 19.0", 190000, true},
		{"postgres 17.11", 170011, false},
		{"postgres 14.24", 140024, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkVersion(c.num)
			if c.ok && err != nil {
				t.Fatalf("want accepted, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("want refused, got no error")
			}
		})
	}
}
