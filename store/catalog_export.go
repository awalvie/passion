package store

// Turning rows back into files. The same structs the loader parses, so a key cannot exist
// on the way in and be forgotten on the way out.

import (
	"context"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// ExportTree returns one tree's rows as files, keyed by path. Nothing is written to disk.
// authorID is nil for the shipped tree, the owning account for a private one.
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
		case KindBlock:
			dir = "blocks"
			file, err = blockFileFrom(db, r)
		case KindSession:
			dir = "sessions"
			file, err = sessionFileFrom(db, r)
		case KindMenu:
			// blockFileFrom writes it inside its block.
			continue
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

// fileIDOf writes a row's identity back out. Family only when it differs from the id: the
// importer reads a file with no family line as its own family.
func fileIDOf(r Content) FileID {
	f := FileID{ID: r.UUID}
	if r.Family != r.UUID {
		f.Family = r.Family
	}
	return f
}

func movementFileFrom(db *gorm.DB, r Content) (MovementFile, error) {
	f := MovementFile{
		FileID:  fileIDOf(r),
		Name:    r.Name,
		Notes:   r.Notes,
		Source:  r.Source,
		PerSide: r.PerSide,
	}
	if r.MovementStyle != nil {
		f.Style = *r.MovementStyle
	}
	tags, err := tagSlugs(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Tags = tags

	var media []ContentMedia
	if err := db.Where("content_id = ?", r.ID).Order("position, id").Find(&media).Error; err != nil {
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

func blockFileFrom(db *gorm.DB, r Content) (BlockFile, error) {
	f := BlockFile{FileID: fileIDOf(r), Name: r.Name, Notes: r.Notes, Source: r.Source}
	if r.BlockKind != nil {
		f.Role = *r.BlockKind
	}
	tags, err := tagSlugs(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Tags = tags
	items, err := itemsFrom(db, r)
	if err != nil {
		return f, err
	}
	f.Items = items
	return f, nil
}

func sessionFileFrom(db *gorm.DB, r Content) (SessionFile, error) {
	f := SessionFile{
		FileID: fileIDOf(r),
		Name:   r.Name, Notes: r.Notes,
		Source: r.Source, Color: r.Color, Needs: r.Needs,
	}
	tags, err := tagSlugs(db, r.ID)
	if err != nil {
		return f, err
	}
	f.Tags = tags
	items, err := itemsFrom(db, r)
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

// itemsFrom reads one parent's children back as items, in position order.
func itemsFrom(db *gorm.DB, parent Content) ([]Item, error) {
	var edges []ContentItem
	if err := db.Where("parent_id = ?", parent.ID).Order("position, id").
		Find(&edges).Error; err != nil {
		return nil, err
	}

	out := make([]Item, 0, len(edges))
	for _, e := range edges {
		var child Content
		if err := db.Where("id = ?", e.ChildID).First(&child).Error; err != nil {
			return nil, err
		}

		// No spelling in the format means "another account's row": bare means this tree,
		// app: means the app's.
		if child.AuthorID != nil && (parent.AuthorID == nil || *child.AuthorID != *parent.AuthorID) {
			return nil, fmt.Errorf("%s %q holds %s %q, which belongs to another account. "+
				"The format cannot name it", parent.Kind, parent.Slug, child.Kind, child.Slug)
		}

		if child.Kind == KindMenu {
			body, err := menuBodyFrom(db, child)
			if err != nil {
				return nil, err
			}
			out = append(out, Item{Menu: body})
			continue
		}

		it := Item{
			Notes: e.Notes,
			Dose: Dose{
				Sets: e.Sets, Reps: e.Reps, WeightKg: e.WeightKg,
				RepSeconds: e.RepSeconds, RepRestSeconds: e.RepRestSeconds,
				SetRestSeconds: e.SetRestSeconds, PrepSeconds: e.PrepSeconds,
				Seconds: e.Seconds,
			},
		}
		switch child.Kind {
		case KindMovement:
			it.Movement = writeRef(parent, child)
		case KindBlock:
			it.Block = writeRef(parent, child)
		default:
			return nil, fmt.Errorf("%s %q holds a %s, which the format cannot express",
				parent.Kind, parent.Slug, child.Kind)
		}

		sets, err := itemSets(db, e.ID)
		if err != nil {
			return nil, err
		}
		it.PerSet = sets
		if len(it.PerSet) > 0 {
			it.Reps = nil
		}
		out = append(out, it)
	}
	return out, nil
}

// writeRef spells a child the way a file has to name it. Bare means the tree this file is
// in, so the app: prefix goes on only when the reference leaves its tree.
func writeRef(parent, child Content) string {
	if parent.AuthorID != nil && child.AuthorID == nil {
		return AppPrefix + child.Slug
	}
	return child.Slug
}

func menuBodyFrom(db *gorm.DB, menu Content) (*MenuBody, error) {
	body := &MenuBody{Name: menu.Name, Notes: menu.Notes, Pick: menu.PickCount}
	tags, err := tagSlugs(db, menu.ID)
	if err != nil {
		return nil, err
	}
	body.Tags = tags

	var edges []ContentItem
	if err := db.Where("parent_id = ?", menu.ID).Order("position, id").
		Find(&edges).Error; err != nil {
		return nil, err
	}
	for _, e := range edges {
		var child Content
		if err := db.Where("id = ?", e.ChildID).First(&child).Error; err != nil {
			return nil, err
		}
		if child.AuthorID != nil && (menu.AuthorID == nil || *child.AuthorID != *menu.AuthorID) {
			return nil, fmt.Errorf("menu %q holds movement %q, which belongs to another "+
				"account. The format cannot name it", menu.Slug, child.Slug)
		}
		o := Option{
			Movement: writeRef(menu, child),
			Notes:    e.Notes,
			Dose: Dose{
				Sets: e.Sets, Reps: e.Reps, WeightKg: e.WeightKg,
				RepSeconds: e.RepSeconds, RepRestSeconds: e.RepRestSeconds,
				SetRestSeconds: e.SetRestSeconds, PrepSeconds: e.PrepSeconds,
				Seconds: e.Seconds,
			},
		}
		sets, err := itemSets(db, e.ID)
		if err != nil {
			return nil, err
		}
		o.PerSet = sets
		if len(o.PerSet) > 0 {
			o.Reps = nil
		}
		body.Of = append(body.Of, o)
	}
	return body, nil
}

func itemSets(db *gorm.DB, itemID int64) ([]SetEntry, error) {
	var sets []ContentItemSet
	if err := db.Where("content_item_id = ?", itemID).Order("set_index, rep_index").
		Find(&sets).Error; err != nil {
		return nil, err
	}
	out := make([]SetEntry, 0, len(sets))
	for _, cs := range sets {
		out = append(out, SetEntry{Reps: cs.Reps, WeightKg: cs.WeightKg, Seconds: cs.Seconds})
	}
	return out, nil
}

// MarshalYAML writes an option as a bare slug when it carries nothing else, which keeps a
// menu readable.
func (o Option) MarshalYAML() (any, error) {
	bare := o.Notes == "" && o.Sets == nil && o.Reps == nil && o.WeightKg == nil &&
		o.RepSeconds == nil && o.RepRestSeconds == nil && o.SetRestSeconds == nil &&
		o.PrepSeconds == nil && o.Seconds == nil && len(o.PerSet) == 0
	if bare {
		return o.Movement, nil
	}
	type plain Option
	return plain(o), nil
}
