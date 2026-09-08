package store

// Turning rows back into files.
//
// The export uses the same structs the loader parses, on purpose. One definition of the
// format means a key cannot exist on the way in and be forgotten on the way out. What the
// round trip then proves is the part that can actually go wrong: that no column is dropped
// between the database and a file.
//
// It is also how something built in the app becomes a file that can be kept in a tree.

import (
	"context"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// ExportTree writes every row belonging to one tree back out as files, keyed by the path
// each belongs at. Nothing is written to disk here, so a caller can compare the result or
// save it wherever it wants.
//
// authorID must match the import that wrote the tree: nil for the shipped tree, the owning
// account for a private one.
func (s *Store) ExportTree(ctx context.Context, name string, authorID *int64) (map[string][]byte, error) {
	db := s.read(ctx)

	q := db.Where("source_tree = ?", name)
	if authorID == nil {
		q = q.Where("author_id IS NULL")
	} else {
		q = q.Where("author_id = ?", *authorID)
	}
	var rows []Content
	if err := q.Order("kind, slug").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("store: tree %q holds no rows", name)
	}

	out := map[string][]byte{}

	meta, err := yaml.Marshal(catalogMeta{FormatVersion: FormatVersion})
	if err != nil {
		return nil, err
	}
	out["catalog.yaml"] = meta

	// The vocabulary belongs to the shipped tree. A private tree that carries no tags of
	// its own gets no tags.yaml, which is what the loader expects.
	if authorID == nil {
		var tags []Tag
		if err := db.Order("slug").Find(&tags).Error; err != nil {
			return nil, err
		}
		if len(tags) > 0 {
			defs := make([]TagDef, 0, len(tags))
			for _, t := range tags {
				defs = append(defs, TagDef{Slug: t.Slug, Name: t.Name})
			}
			b, err := yaml.Marshal(defs)
			if err != nil {
				return nil, err
			}
			out["tags.yaml"] = b
		}
	}

	for _, r := range rows {
		var (
			file any
			dir  string
			err  error
		)
		switch r.Kind {
		case KindMovement:
			dir = "movements"
			file, err = movementFileFrom(db, r)
		case KindMenu:
			dir = "menus"
			file, err = menuFileFrom(db, r)
		case KindBlock:
			dir = "blocks"
			file, err = blockFileFrom(db, r)
		case KindSession:
			dir = "sessions"
			file, err = sessionFileFrom(db, r)
		default:
			return nil, fmt.Errorf("store: row %d has kind %q", r.ID, r.Kind)
		}
		if err != nil {
			return nil, fmt.Errorf("exporting %s: %w", r.Slug, err)
		}
		b, err := yaml.Marshal(file)
		if err != nil {
			return nil, err
		}
		out[dir+"/"+r.Slug+".yaml"] = b
	}
	return out, nil
}

func movementFileFrom(db *gorm.DB, r Content) (MovementFile, error) {
	f := MovementFile{
		Name:    r.Name,
		Slug:    r.Slug,
		Notes:   r.Notes,
		Source:  r.Source,
		PerSide: r.PerSide,
	}
	if r.MovementKind != nil {
		f.Kind = *r.MovementKind
	}
	tags, err := tagSlugs(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Tags = tags

	var media []ContentMedia
	if err := db.Where("content_id = ?", r.ID).Order("position").Find(&media).Error; err != nil {
		return f, err
	}
	for _, m := range media {
		f.Media = append(f.Media, Media{URL: m.URL, ThumbURL: m.ThumbURL})
	}

	f.Dose = Dose{
		Sets: r.DSets, Reps: r.DReps, WeightKg: r.DWeightKg,
		RepSeconds: r.DRepSeconds, RepRestSeconds: r.DRepRestSeconds,
		SetRestSeconds: r.DSetRestSeconds, PrepSeconds: r.DPrepSeconds,
		Seconds: r.DSeconds,
	}

	var sets []ContentSet
	if err := db.Where("content_id = ?", r.ID).Order("set_index, rep_index").
		Find(&sets).Error; err != nil {
		return f, err
	}
	for _, cs := range sets {
		f.PerSet = append(f.PerSet, SetEntry{Reps: cs.Reps, WeightKg: cs.WeightKg, Seconds: cs.Seconds})
	}
	// A ladder replaces reps, so the column is not written back as a key.
	if len(f.PerSet) > 0 {
		f.Reps = nil
	}
	return f, nil
}

func menuFileFrom(db *gorm.DB, r Content) (MenuFile, error) {
	f := MenuFile{Name: r.Name, Slug: r.Slug, Notes: r.Notes, Pick: r.PickCount}
	tags, err := tagSlugs(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Tags = tags
	items, err := itemsFrom(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Options = items
	return f, nil
}

func blockFileFrom(db *gorm.DB, r Content) (BlockFile, error) {
	f := BlockFile{Name: r.Name, Slug: r.Slug, Notes: r.Notes, Source: r.Source}
	if r.BlockKind != nil {
		f.Role = *r.BlockKind
	}
	tags, err := tagSlugs(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Tags = tags
	items, err := itemsFrom(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Items = items
	return f, nil
}

func sessionFileFrom(db *gorm.DB, r Content) (SessionFile, error) {
	f := SessionFile{
		Name: r.Name, Slug: r.Slug, Notes: r.Notes,
		Source: r.Source, Color: r.Color, Needs: r.Needs,
	}
	tags, err := tagSlugs(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Tags = tags
	items, err := itemsFrom(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Items = items
	return f, nil
}

// tagSlugs returns a row's tags in a settled order, so exporting twice gives the same file.
func tagSlugs(db *gorm.DB, contentID int64) ([]string, error) {
	var slugs []string
	err := db.Raw(`SELECT t.slug FROM content_tag ct JOIN tag t ON t.id = ct.tag_id
	               WHERE ct.content_id = ?`, contentID).Scan(&slugs).Error
	sort.Strings(slugs)
	return slugs, err
}

// itemsFrom reads one parent's children back as references, in position order.
func itemsFrom(db *gorm.DB, parentID int64) ([]Item, error) {
	var edges []ContentItem
	if err := db.Where("parent_id = ?", parentID).Order("position").Find(&edges).Error; err != nil {
		return nil, err
	}

	out := make([]Item, 0, len(edges))
	for _, e := range edges {
		var child Content
		if err := db.Select("slug").Where("id = ?", e.ChildID).First(&child).Error; err != nil {
			return nil, err
		}
		it := Item{
			Ref:   child.Slug,
			Notes: e.Notes,
			Dose: Dose{
				Sets: e.Sets, Reps: e.Reps, WeightKg: e.WeightKg,
				RepSeconds: e.RepSeconds, RepRestSeconds: e.RepRestSeconds,
				SetRestSeconds: e.SetRestSeconds, PrepSeconds: e.PrepSeconds,
				Seconds: e.Seconds,
			},
		}
		var sets []ContentItemSet
		if err := db.Where("content_item_id = ?", e.ID).Order("set_index, rep_index").
			Find(&sets).Error; err != nil {
			return nil, err
		}
		for _, cs := range sets {
			it.PerSet = append(it.PerSet, SetEntry{Reps: cs.Reps, WeightKg: cs.WeightKg, Seconds: cs.Seconds})
		}
		if len(it.PerSet) > 0 {
			it.Reps = nil
		}
		out = append(out, it)
	}
	return out, nil
}
