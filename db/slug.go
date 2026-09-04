package db

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// The importer used to match catalog rows by display name, so renaming an entry read as
// delete-then-create: the old row was pruned and a new one built. That is what deleted
// rows on rename and what left cycle overrides pointing at nothing.
//
// Slug is identity from here. The YAML carries an explicit `slug:`, and every existing row
// is backfilled from its current name so the two line up. Both must happen before the
// importer matches on slug — a switch with an empty column makes every row match nothing,
// and prune then treats all 1,619 of them as orphans.

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
var slugApostrophe = regexp.MustCompile(`['’` + "`" + `]`)

// Slugify derives a slug from a display name. It must match the generator used on the
// YAML trees exactly, or a backfilled row and its YAML entry will not find each other.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugApostrophe.ReplaceAllString(s, "")
	s = slugNonAlnum.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// SlugBackfillReport says what the backfill found and what it set.
type SlugBackfillReport struct {
	ByTable    map[string]int64
	Total      int64
	Collisions []string
	DryRun     bool
}

// slugTables are the three models that carry a slug, with the table name for reporting.
func slugTables() []struct {
	name  string
	model any
} {
	return []struct {
		name  string
		model any
	}{
		{"library_exercises", &LibraryExercise{}},
		{"activity_templates", &ActivityTemplate{}},
		{"session_templates", &SessionTemplate{}},
	}
}

// BackfillSlugs fills the slug on every row that has none, derived from its name. Rows that
// already carry one are left alone, so it is safe to run repeatedly.
//
// A slug is unique per owner, so a collision within one account gets a counter. Collisions
// are reported either way: two rows an account named the same thing is worth knowing about
// before the importer starts matching on it.
func BackfillSlugs(gdb *gorm.DB, dryRun bool) (SlugBackfillReport, error) {
	rep := SlugBackfillReport{ByTable: map[string]int64{}, DryRun: dryRun}

	for _, tbl := range slugTables() {
		type row struct {
			ID      uint
			OwnerID uint
			Name    string
		}
		var rows []row
		if err := gdb.Model(tbl.model).
			Where("slug = ''").
			Select("id, owner_id, name").
			Order("id").Scan(&rows).Error; err != nil {
			return rep, err
		}
		if len(rows) == 0 {
			continue
		}

		// Slugs already in use for this owner, so the backfill cannot collide with a row
		// that was created with one.
		taken := map[uint]map[string]bool{}
		type existing struct {
			OwnerID uint
			Slug    string
		}
		var have []existing
		if err := gdb.Model(tbl.model).Where("slug <> ''").
			Select("owner_id, slug").Scan(&have).Error; err != nil {
			return rep, err
		}
		for _, e := range have {
			if taken[e.OwnerID] == nil {
				taken[e.OwnerID] = map[string]bool{}
			}
			taken[e.OwnerID][e.Slug] = true
		}

		for _, r := range rows {
			base := Slugify(r.Name)
			if base == "" {
				base = fmt.Sprintf("row_%d", r.ID)
			}
			if taken[r.OwnerID] == nil {
				taken[r.OwnerID] = map[string]bool{}
			}
			slug := base
			for i := 2; taken[r.OwnerID][slug]; i++ {
				slug = fmt.Sprintf("%s_%d", base, i)
				if i == 2 {
					rep.Collisions = append(rep.Collisions,
						fmt.Sprintf("%s: owner %d has more than one %q", tbl.name, r.OwnerID, base))
				}
			}
			taken[r.OwnerID][slug] = true
			rep.ByTable[tbl.name]++
			rep.Total++
			if dryRun {
				continue
			}
			if err := gdb.Model(tbl.model).Where("id = ?", r.ID).
				Update("slug", slug).Error; err != nil {
				return rep, err
			}
		}
	}
	return rep, nil
}
