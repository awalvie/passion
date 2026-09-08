package store

// Writing a checked tree into the database. The rules are set out in docs/V2_PLAN.md 1.10.
//
// Three of them shape this file:
//
//   - A row remembers which tree it came from, in source_tree. A re-import rewrites exactly
//     the rows whose source_tree matches the tree being imported, and touches nothing else.
//     Without that, a file would stop meaning anything after its first import, and an edit
//     made in the app would be undone by the next start.
//   - A second import of an unchanged tree writes nothing at all, so every write here is
//     preceded by a comparison. Delete-then-reinsert would be far shorter, and it would
//     issue new row ids and move every updated_at. content_key in particular must survive,
//     because finished runs point at it.
//   - The shipped tree lands with author_id NULL, so no account's deletion can reach it. A
//     private tree lands owned by one account, named by email rather than by id: an id in
//     configuration goes stale when the account is deleted, and an email cannot.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNoSuchOwner means a private tree names an owner email that no account holds. The
// caller logs it and carries on: a first boot has no accounts at all, and refusing to boot
// would leave nobody able to sign up.
var ErrNoSuchOwner = errors.New("store: no account holds that owner email")

// ShippedTree is the source_tree value for content the app ships.
const ShippedTree = "shipped"

// ImportResult is what one import did, for the log line. On a second run of an unchanged
// tree, Unchanged is the whole count and everything else is zero.
type ImportResult struct {
	Tree      string
	Inserted  int
	Updated   int
	Unchanged int
	Retired   int
}

func (r ImportResult) String() string {
	return fmt.Sprintf("tree=%s inserted=%d updated=%d unchanged=%d retired=%d",
		r.Tree, r.Inserted, r.Updated, r.Unchanged, r.Retired)
}

// ImportShipped writes a tree as content the app ships: no author, so no account's deletion
// can reach it.
func (s *Store) ImportShipped(ctx context.Context, t *Tree) (ImportResult, error) {
	return s.importTree(ctx, t, nil)
}

// ImportOwned writes a tree as one account's own content. The email is resolved here rather
// than in configuration, so a name that matches nothing is an error with something useful
// in it instead of rows under an id nothing points at.
func (s *Store) ImportOwned(ctx context.Context, t *Tree, ownerEmail string) (ImportResult, error) {
	var a Account
	err := s.read(ctx).Where("email = ?", normalizeEmail(ownerEmail)).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ImportResult{Tree: t.Name}, fmt.Errorf("%w: %q", ErrNoSuchOwner, ownerEmail)
	}
	if err != nil {
		return ImportResult{Tree: t.Name}, err
	}
	return s.importTree(ctx, t, &a.ID)
}

func (s *Store) importTree(ctx context.Context, t *Tree, authorID *int64) (ImportResult, error) {
	res := ImportResult{Tree: t.Name}
	err := s.WithTx(ctx, func(tx *gorm.DB) error {
		if err := syncTags(tx, t.Tags); err != nil {
			return err
		}

		// Movements first, then menus, blocks and sessions. A child must exist before an
		// edge can point at it, and the four kinds nest in exactly that order.
		//
		ids := map[string]int64{}
		for _, m := range t.Movements {
			if err := upsertOne(tx, t, authorID, contentFromMovement(m), m.Tags, m.Media, m.PerSet, ids, &res); err != nil {
				return err
			}
		}
		for _, m := range t.Menus {
			if err := upsertOne(tx, t, authorID, contentFromMenu(m), m.Tags, nil, nil, ids, &res); err != nil {
				return err
			}
		}
		for _, b := range t.Blocks {
			if err := upsertOne(tx, t, authorID, contentFromBlock(b), b.Tags, nil, nil, ids, &res); err != nil {
				return err
			}
		}
		for _, sn := range t.Sessions {
			if err := upsertOne(tx, t, authorID, contentFromSession(sn), sn.Tags, nil, nil, ids, &res); err != nil {
				return err
			}
		}

		// Edges last, once every slug in the tree has a row.
		for _, m := range t.Menus {
			if err := syncEdges(tx, authorID, ids[m.Slug], KindMenu, m.Options, ids); err != nil {
				return fmt.Errorf("menus/%s.yaml: %w", m.Slug, err)
			}
		}
		for _, b := range t.Blocks {
			if err := syncEdges(tx, authorID, ids[b.Slug], KindBlock, b.Items, ids); err != nil {
				return fmt.Errorf("blocks/%s.yaml: %w", b.Slug, err)
			}
		}
		for _, sn := range t.Sessions {
			if err := syncEdges(tx, authorID, ids[sn.Slug], KindSession, sn.Items, ids); err != nil {
				return fmt.Errorf("sessions/%s.yaml: %w", sn.Slug, err)
			}
		}

		n, err := retireMissing(tx, t, authorID, ids)
		res.Retired = n
		return err
	})
	return res, err
}

// syncTags writes the vocabulary. A tag is matched by slug, and only a changed display
// name is written, so an unchanged tree leaves the table alone.
func syncTags(tx *gorm.DB, tags []TagDef) error {
	for _, td := range tags {
		var existing Tag
		err := tx.Where("slug = ?", td.Slug).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&Tag{Slug: td.Slug, Name: td.Name}).Error; err != nil {
				return fmt.Errorf("tags.yaml: creating %q: %w", td.Slug, err)
			}
			continue
		}
		if err != nil {
			return err
		}
		if existing.Name != td.Name {
			if err := tx.Model(&Tag{}).Where("id = ?", existing.ID).
				Update("name", td.Name).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// upsertOne is the whole per-row decision. It records the id it settled on in ids, so the
// edge pass can resolve a ref without a second query.
func upsertOne(tx *gorm.DB, t *Tree, authorID *int64, want Content,
	tags []string, media []Media, perSet []SetEntry,
	ids map[string]int64, res *ImportResult) error {

	where := fmt.Sprintf("%ss/%s.yaml", want.Kind, want.Slug)
	want.AuthorID = authorID
	tree := t.Name
	want.SourceTree = &tree

	var have Content
	q := tx.Where("kind = ? AND slug = ?", want.Kind, want.Slug)
	if authorID == nil {
		q = q.Where("author_id IS NULL")
	} else {
		q = q.Where("author_id = ?", *authorID)
	}
	err := q.First(&have).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		want.ContentKey = uuid.NewString()
		want.CreatedAt = time.Now()
		want.UpdatedAt = want.CreatedAt
		if err := tx.Create(&want).Error; err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		ids[want.Slug] = want.ID
		res.Inserted++

	case err != nil:
		return err

	// A row a person made, or a fork, holding the slug this file wants. Refused rather than
	// worked around. The importer must not overwrite it, and it must not quietly leave it
	// either: every other file pointing at this slug would then get that row instead of the
	// one described here, so the tree would import "successfully" and mean something else.
	case have.SourceTree == nil:
		return fmt.Errorf("%s: you already have a %s called %q that you made yourself. "+
			"Rename yours, or rename this file", where, want.Kind, want.Slug)

	// Two trees claiming one slug for one owner. Nothing can resolve that, so it stops here
	// rather than letting the last tree imported win.
	case *have.SourceTree != tree:
		return fmt.Errorf("%s: slug %q already belongs to tree %q, and tree %q also claims it",
			where, want.Slug, *have.SourceTree, tree)

	default:
		ids[have.Slug] = have.ID
		want.ID = have.ID
		want.ContentKey = have.ContentKey // survives a refresh; history hangs off it
		want.CreatedAt = have.CreatedAt
		if sameContent(have, want) {
			res.Unchanged++
		} else {
			want.UpdatedAt = time.Now()
			if err := tx.Model(&Content{}).Where("id = ?", have.ID).
				Select(contentColumns).Updates(&want).Error; err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
			res.Updated++
		}
	}

	id := ids[want.Slug]
	if err := syncTagLinks(tx, id, tags); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	if err := syncMedia(tx, id, media); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	return syncContentSets(tx, id, perSet)
}

// contentColumns is what an import owns on a row. Named so Updates cannot reach a column
// the file has no opinion about — content_key and created_at especially, and retired_on,
// which is cleared by name below rather than by accident.
var contentColumns = []string{
	"name", "notes", "source", "source_tree", "retired_on",
	"movement_kind", "per_side",
	"d_sets", "d_reps", "d_weight_kg", "d_rep_seconds", "d_rep_rest_seconds",
	"d_set_rest_seconds", "d_prep_seconds", "d_seconds",
	"block_kind", "pick_count", "color", "needs", "updated_at",
}

// sameContent compares only what a file decides. A difference here is the only reason to
// write, which is what makes a second import a no-op rather than a rewrite.
func sameContent(a, b Content) bool {
	return a.Name == b.Name &&
		a.Notes == b.Notes &&
		a.Source == b.Source &&
		eqStr(a.RetiredOn, b.RetiredOn) &&
		eqStr(a.MovementKind, b.MovementKind) &&
		a.PerSide == b.PerSide &&
		eqInt(a.DSets, b.DSets) &&
		eqInt(a.DReps, b.DReps) &&
		eqFloat(a.DWeightKg, b.DWeightKg) &&
		eqInt(a.DRepSeconds, b.DRepSeconds) &&
		eqInt(a.DRepRestSeconds, b.DRepRestSeconds) &&
		eqInt(a.DSetRestSeconds, b.DSetRestSeconds) &&
		eqInt(a.DPrepSeconds, b.DPrepSeconds) &&
		eqInt(a.DSeconds, b.DSeconds) &&
		eqStr(a.BlockKind, b.BlockKind) &&
		eqInt(a.PickCount, b.PickCount) &&
		a.Color == b.Color &&
		a.Needs == b.Needs
}

func eqStr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func eqInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func eqFloat(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// ---------------------------------------------------------------------------
// One file becomes one row
// ---------------------------------------------------------------------------
//
// Nothing here reads or writes. It is the mapping from the format to the columns, kept in
// one place so a new key has exactly one home.

func doseToContent(c *Content, d Dose) {
	c.DSets = d.Sets
	c.DReps = d.Reps
	c.DWeightKg = d.WeightKg
	c.DRepSeconds = d.RepSeconds
	c.DRepRestSeconds = d.RepRestSeconds
	c.DSetRestSeconds = d.SetRestSeconds
	c.DPrepSeconds = d.PrepSeconds
	c.DSeconds = d.Seconds
}

func contentFromMovement(m MovementFile) Content {
	kind := m.Kind
	c := Content{
		Kind:         KindMovement,
		Slug:         m.Slug,
		Name:         m.Name,
		Notes:        m.Notes,
		Source:       m.Source,
		MovementKind: &kind,
		PerSide:      m.PerSide,
	}
	doseToContent(&c, m.Dose)
	return c
}

func contentFromMenu(m MenuFile) Content {
	return Content{
		Kind:      KindMenu,
		Slug:      m.Slug,
		Name:      m.Name,
		Notes:     m.Notes,
		PickCount: m.Pick,
	}
}

func contentFromBlock(b BlockFile) Content {
	c := Content{
		Kind:   KindBlock,
		Slug:   b.Slug,
		Name:   b.Name,
		Notes:  b.Notes,
		Source: b.Source,
	}
	if b.Role != "" {
		role := b.Role
		c.BlockKind = &role
	}
	return c
}

func contentFromSession(s SessionFile) Content {
	return Content{
		Kind:   KindSession,
		Slug:   s.Slug,
		Name:   s.Name,
		Notes:  s.Notes,
		Source: s.Source,
		Color:  s.Color,
		Needs:  s.Needs,
	}
}

// ---------------------------------------------------------------------------
// The child tables
// ---------------------------------------------------------------------------
//
// Each of these compares before it writes, for the same reason as the content row: an
// unchanged tree must leave the tables byte for byte alone. Deleting and reinserting would
// be shorter and would issue new row ids every time.

func syncTagLinks(tx *gorm.DB, contentID int64, tags []string) error {
	var want []int64
	for _, slug := range tags {
		var t Tag
		if err := tx.Where("slug = ?", slug).First(&t).Error; err != nil {
			return fmt.Errorf("tag %q is not in tags.yaml: %w", slug, err)
		}
		want = append(want, t.ID)
	}

	var have []ContentTag
	if err := tx.Where("content_id = ?", contentID).Find(&have).Error; err != nil {
		return err
	}
	haveSet := map[int64]bool{}
	for _, ct := range have {
		haveSet[ct.TagID] = true
	}
	wantSet := map[int64]bool{}
	for _, id := range want {
		wantSet[id] = true
	}

	for id := range wantSet {
		if !haveSet[id] {
			if err := tx.Create(&ContentTag{ContentID: contentID, TagID: id}).Error; err != nil {
				return err
			}
		}
	}
	for id := range haveSet {
		if !wantSet[id] {
			if err := tx.Where("content_id = ? AND tag_id = ?", contentID, id).
				Delete(&ContentTag{}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// syncMedia keeps the list in file order. Matched by position, so an unchanged list keeps
// its row ids and a reordered one is rewritten in place rather than deleted and recreated.
func syncMedia(tx *gorm.DB, contentID int64, media []Media) error {
	var have []ContentMedia
	if err := tx.Where("content_id = ?", contentID).Order("position").Find(&have).Error; err != nil {
		return err
	}

	for i, m := range media {
		if i < len(have) {
			if have[i].URL == m.URL && have[i].ThumbURL == m.ThumbURL {
				continue
			}
			if err := tx.Model(&ContentMedia{}).Where("id = ?", have[i].ID).
				Updates(map[string]any{"url": m.URL, "thumb_url": m.ThumbURL}).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Create(&ContentMedia{
			ContentID: contentID, URL: m.URL, ThumbURL: m.ThumbURL, Position: i,
		}).Error; err != nil {
			return err
		}
	}
	for i := len(media); i < len(have); i++ {
		if err := tx.Where("id = ?", have[i].ID).Delete(&ContentMedia{}).Error; err != nil {
			return err
		}
	}
	return nil
}

// syncContentSets writes a movement's own ladder. The key is (content, set, rep) and a
// ladder's entries are reps inside ONE set, so set_index is 1 and rep_index is the rung.
func syncContentSets(tx *gorm.DB, contentID int64, perSet []SetEntry) error {
	var have []ContentSet
	if err := tx.Where("content_id = ?", contentID).Order("set_index, rep_index").
		Find(&have).Error; err != nil {
		return err
	}
	haveAt := map[int]ContentSet{}
	for _, cs := range have {
		haveAt[cs.RepIndex] = cs
	}

	for i, e := range perSet {
		row := ContentSet{
			ContentID: contentID, SetIndex: 1, RepIndex: i,
			Reps: e.Reps, WeightKg: e.WeightKg, Seconds: e.Seconds,
		}
		old, ok := haveAt[i]
		if ok {
			if eqInt(old.Reps, row.Reps) && eqFloat(old.WeightKg, row.WeightKg) &&
				eqInt(old.Seconds, row.Seconds) {
				continue
			}
			if err := tx.Model(&ContentSet{}).
				Where("content_id = ? AND set_index = 1 AND rep_index = ?", contentID, i).
				Select("reps", "weight_kg", "seconds").Updates(&row).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	for _, cs := range have {
		if cs.RepIndex >= len(perSet) {
			if err := tx.Where("content_id = ? AND set_index = ? AND rep_index = ?",
				contentID, cs.SetIndex, cs.RepIndex).Delete(&ContentSet{}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// syncEdges writes one parent's children. An edge is matched by (parent, child), which is
// what ux_item_edge makes unique, so position and the per-use numbers are updated in place.
func syncEdges(tx *gorm.DB, authorID *int64, parentID int64, parentKind string,
	items []Item, ids map[string]int64) error {

	var have []ContentItem
	if err := tx.Where("parent_id = ?", parentID).Order("position").Find(&have).Error; err != nil {
		return err
	}
	haveByChild := map[int64]ContentItem{}
	for _, e := range have {
		haveByChild[e.ChildID] = e
	}

	keep := map[int64]bool{}
	for i, it := range items {
		childID, childKind, err := resolveRef(tx, authorID, it.Ref, ids)
		if err != nil {
			return err
		}
		keep[childID] = true

		want := ContentItem{
			ParentID: parentID, ParentKind: parentKind,
			ChildID: childID, ChildKind: childKind,
			Position: i, Notes: it.Notes,
		}
		itemDose(&want, it.Dose)

		old, exists := haveByChild[childID]
		if !exists {
			if err := tx.Create(&want).Error; err != nil {
				return err
			}
			if err := syncItemSets(tx, want.ID, it.PerSet); err != nil {
				return err
			}
			continue
		}
		want.ID = old.ID
		if !sameItem(old, want) {
			if err := tx.Model(&ContentItem{}).Where("id = ?", old.ID).
				Select(itemColumns).Updates(&want).Error; err != nil {
				return err
			}
		}
		if err := syncItemSets(tx, old.ID, it.PerSet); err != nil {
			return err
		}
	}

	for _, e := range have {
		if !keep[e.ChildID] {
			if err := tx.Where("id = ?", e.ID).Delete(&ContentItem{}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveRef turns a slug into the row an edge should point at, and its kind. The kind is
// needed as well as the id because the foreign key in content_item names both, which is
// what pins an edge to one kind and makes a cycle impossible.
//
// It looks in this tree first, then at content already in the database. A private tree
// leans heavily on the second case: it references shipped movements rather than carrying
// copies of them.
//
// Order within the database is your own content, then the app's. So a movement you own
// shadows a shipped one of the same slug, which is the same order the loader validates in.
func resolveRef(tx *gorm.DB, authorID *int64, ref string, ids map[string]int64) (int64, string, error) {
	if id, ok := ids[ref]; ok && id != 0 {
		var c Content
		if err := tx.Select("kind").Where("id = ?", id).First(&c).Error; err != nil {
			return 0, "", err
		}
		return id, c.Kind, nil
	}

	var c Content
	if authorID != nil {
		err := tx.Select("id", "kind").
			Where("slug = ? AND author_id = ?", ref, *authorID).First(&c).Error
		if err == nil {
			return c.ID, c.Kind, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", err
		}
	}
	err := tx.Select("id", "kind").
		Where("slug = ? AND author_id IS NULL", ref).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, "", fmt.Errorf("ref %q names nothing in this tree and nothing already imported", ref)
	}
	if err != nil {
		return 0, "", err
	}
	return c.ID, c.Kind, nil
}

var itemColumns = []string{
	"parent_kind", "child_kind", "position", "notes",
	"sets", "reps", "weight_kg", "rep_seconds", "rep_rest_seconds",
	"set_rest_seconds", "prep_seconds", "seconds",
}

func itemDose(c *ContentItem, d Dose) {
	c.Sets = d.Sets
	c.Reps = d.Reps
	c.WeightKg = d.WeightKg
	c.RepSeconds = d.RepSeconds
	c.RepRestSeconds = d.RepRestSeconds
	c.SetRestSeconds = d.SetRestSeconds
	c.PrepSeconds = d.PrepSeconds
	c.Seconds = d.Seconds
}

func sameItem(a, b ContentItem) bool {
	return a.ParentKind == b.ParentKind && a.ChildKind == b.ChildKind &&
		a.Position == b.Position && a.Notes == b.Notes &&
		eqInt(a.Sets, b.Sets) && eqInt(a.Reps, b.Reps) &&
		eqFloat(a.WeightKg, b.WeightKg) &&
		eqInt(a.RepSeconds, b.RepSeconds) && eqInt(a.RepRestSeconds, b.RepRestSeconds) &&
		eqInt(a.SetRestSeconds, b.SetRestSeconds) && eqInt(a.PrepSeconds, b.PrepSeconds) &&
		eqInt(a.Seconds, b.Seconds)
}

// syncItemSets is syncContentSets for an edge rather than a movement.
func syncItemSets(tx *gorm.DB, itemID int64, perSet []SetEntry) error {
	var have []ContentItemSet
	if err := tx.Where("content_item_id = ?", itemID).Order("set_index, rep_index").
		Find(&have).Error; err != nil {
		return err
	}
	haveAt := map[int]ContentItemSet{}
	for _, cs := range have {
		haveAt[cs.RepIndex] = cs
	}
	for i, e := range perSet {
		row := ContentItemSet{
			ContentItemID: itemID, SetIndex: 1, RepIndex: i,
			Reps: e.Reps, WeightKg: e.WeightKg, Seconds: e.Seconds,
		}
		if old, ok := haveAt[i]; ok {
			if eqInt(old.Reps, row.Reps) && eqFloat(old.WeightKg, row.WeightKg) &&
				eqInt(old.Seconds, row.Seconds) {
				continue
			}
			if err := tx.Model(&ContentItemSet{}).
				Where("content_item_id = ? AND set_index = 1 AND rep_index = ?", itemID, i).
				Select("reps", "weight_kg", "seconds").Updates(&row).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	for _, cs := range have {
		if cs.RepIndex >= len(perSet) {
			if err := tx.Where("content_item_id = ? AND set_index = ? AND rep_index = ?",
				itemID, cs.SetIndex, cs.RepIndex).Delete(&ContentItemSet{}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// retireMissing marks a row whose file has left the tree, rather than deleting it. A plan
// or a finished run may still point at it. The library hides a retired row, and everything
// else keeps resolving.
func retireMissing(tx *gorm.DB, t *Tree, authorID *int64, ids map[string]int64) (int, error) {
	q := tx.Model(&Content{}).Where("source_tree = ?", t.Name)
	if authorID == nil {
		q = q.Where("author_id IS NULL")
	} else {
		q = q.Where("author_id = ?", *authorID)
	}
	var rows []Content
	if err := q.Find(&rows).Error; err != nil {
		return 0, err
	}

	today := time.Now().Format("2006-01-02")
	n := 0
	for _, r := range rows {
		_, present := ids[r.Slug]
		switch {
		case !present && r.RetiredOn == nil:
			d := Date(today)
			if err := tx.Model(&Content{}).Where("id = ?", r.ID).
				Update("retired_on", &d).Error; err != nil {
				return n, err
			}
			n++
		case present && r.RetiredOn != nil:
			// The file came back. Un-retire it so the library shows it again.
			if err := tx.Model(&Content{}).Where("id = ?", r.ID).
				Update("retired_on", nil).Error; err != nil {
				return n, err
			}
		}
	}
	return n, nil
}
