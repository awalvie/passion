package store

// Writing a checked tree into the database. Three rules shape this file:
//
//   - A row whose source_tree is NULL was edited in the app. It is skipped, not refused:
//     one edit must not stop the whole tree importing on every start from then on.
//   - A second import of an unchanged tree writes nothing, so every write is preceded by a
//     comparison. Row ids have to survive a re-import; movement_pref points at one.
//   - The whole tree is one transaction.

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNoSuchOwner means a private tree names an owner email that no account holds. The
// caller logs it and carries on: a first boot has no accounts at all.
var ErrNoSuchOwner = errors.New("store: no account holds that owner email")

// ShippedTree is the source_tree value for content the app ships.
const ShippedTree = "shipped"

// ImportResult is what one import did, for the log line.
type ImportResult struct {
	Tree      string
	Inserted  int
	Updated   int
	Unchanged int
	Retired   int

	// Skipped names every file whose row was edited in the app, so the file no longer has
	// any effect. Reported, because otherwise the edit is silently ignored.
	Skipped []string
}

func (r ImportResult) String() string {
	s := fmt.Sprintf("tree=%s inserted=%d updated=%d unchanged=%d retired=%d",
		r.Tree, r.Inserted, r.Updated, r.Unchanged, r.Retired)
	if len(r.Skipped) > 0 {
		s += fmt.Sprintf(" skipped=%d", len(r.Skipped))
	}
	return s
}

// ImportShipped writes a tree with author_id NULL, so no account's deletion can reach it.
func (s *Store) ImportShipped(ctx context.Context, t *Tree) (ImportResult, error) {
	return s.importTree(ctx, t, nil)
}

// ImportOwned writes a tree as one account's own content. Configuration names the owner by
// email, not id: an id goes stale when the account is deleted.
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

// TreeMenu is one menu in a tree, with the slug it will hold. A menu has no file, so its
// slug is derived from where it sits. It is never typed or shown, only stored.
type TreeMenu struct {
	Slug  string
	Block string
	Body  MenuBody
}

// Menus is every menu in the tree, in a settled order.
func (t *Tree) Menus() []TreeMenu {
	var out []TreeMenu
	for _, b := range t.Blocks {
		for i, it := range b.Items {
			if it.Menu != nil {
				out = append(out, TreeMenu{
					Slug:  b.Slug + "_" + strconv.Itoa(i),
					Block: b.Slug,
					Body:  *it.Menu,
				})
			}
		}
	}
	return out
}

// importer carries the state one import needs across its passes.
type importer struct {
	tx       *gorm.DB
	tree     *Tree
	authorID *int64
	res      *ImportResult

	// ids is every row this tree settled on, so the edge pass resolves without a query.
	ids map[Ref]int64

	// detached is every slug this tree owns whose row was edited in the app. Its edges and
	// its menus are left alone too, or the edit would be half kept.
	detached map[Ref]bool

	// keep is every row this import decided about, including the ones it skipped. What is
	// NOT in here is what the tree no longer holds, which is what reapMissing acts on.
	keep map[Ref]bool

	// keepID is keep addressed by row id, which reapMissing needs: a (kind, slug) pair
	// moves when a file is renamed, a row id does not.
	keepID map[int64]bool
}

func (s *Store) importTree(ctx context.Context, t *Tree, authorID *int64) (ImportResult, error) {
	res := ImportResult{Tree: t.Name}

	// A slot's weight beats a person's own saved number, so a shipped file carrying one
	// would override every account.
	if authorID == nil {
		if err := refuseShippedWeights(t); err != nil {
			return res, err
		}
	}

	err := s.WithTx(ctx, func(tx *gorm.DB) error {
		im := &importer{
			tx: tx, tree: t, authorID: authorID, res: &res,
			ids:      map[Ref]int64{},
			detached: map[Ref]bool{},
			keep:     map[Ref]bool{},
			keepID:   map[int64]bool{},
		}
		// Rows first, in nesting order, so a child exists before an edge points at it.
		for _, m := range t.Movements {
			if err := im.upsert("movements/"+m.Slug+".yaml", contentFromMovement(m),
				m.Tags, m.Media, m.PerSet); err != nil {
				return err
			}
		}
		for _, mn := range t.Menus() {
			if im.detached[Ref{Kind: KindBlock, Slug: mn.Block}] {
				im.keep[Ref{Kind: KindMenu, Slug: mn.Slug}] = true
				continue
			}
			where := fmt.Sprintf("blocks/%s.yaml: the menu at %s", mn.Block, mn.Slug)
			if err := im.upsert(where, contentFromMenu(mn), mn.Body.Tags, nil, nil); err != nil {
				return err
			}
		}
		for _, b := range t.Blocks {
			if err := im.upsert("blocks/"+b.Slug+".yaml", contentFromBlock(b),
				b.Tags, nil, nil); err != nil {
				return err
			}
		}
		for _, sn := range t.Sessions {
			if err := im.upsert("sessions/"+sn.Slug+".yaml", contentFromSession(sn),
				sn.Tags, nil, nil); err != nil {
				return err
			}
		}

		// Blocks come before menus here only because a menu's slug names its block, and a
		// detached block takes its menus with it.
		for _, mn := range t.Menus() {
			ref := Ref{Kind: KindMenu, Slug: mn.Slug}
			if im.detached[Ref{Kind: KindBlock, Slug: mn.Block}] || im.detached[ref] {
				continue
			}
			items := make([]Item, 0, len(mn.Body.Of))
			for _, o := range mn.Body.Of {
				items = append(items, Item{Movement: o.Movement, Notes: o.Notes, Dose: o.Dose})
			}
			if err := im.syncEdges(ref, items); err != nil {
				return fmt.Errorf("blocks/%s.yaml: the menu at %s: %w", mn.Block, mn.Slug, err)
			}
		}
		for _, b := range t.Blocks {
			ref := Ref{Kind: KindBlock, Slug: b.Slug}
			if im.detached[ref] {
				continue
			}
			if err := im.syncEdges(ref, b.Items); err != nil {
				return fmt.Errorf("blocks/%s.yaml: %w", b.Slug, err)
			}
		}
		for _, sn := range t.Sessions {
			ref := Ref{Kind: KindSession, Slug: sn.Slug}
			if im.detached[ref] {
				continue
			}
			if err := im.syncEdges(ref, sn.Items); err != nil {
				return fmt.Errorf("sessions/%s.yaml: %w", sn.Slug, err)
			}
		}

		return im.reapMissing()
	})
	return res, err
}

// refuseShippedWeights is the rule that keeps movement_pref meaningful. See the comment on
// the movement_pref table in docs/SCHEMA_V2.sql.
func refuseShippedWeights(t *Tree) error {
	bad := func(where string, d Dose) error {
		if d.WeightKg != nil {
			return fmt.Errorf("%s: a shipped file may not set weight_kg on a slot. A slot "+
				"beats a person's own saved weight, so this would override every account. "+
				"Put the weight on the movement's own defaults instead", where)
		}
		for i, e := range d.PerSet {
			if e.WeightKg != nil {
				return fmt.Errorf("%s: per_set[%d] sets weight_kg, and a shipped file may not "+
					"set a weight on a slot", where, i)
			}
		}
		return nil
	}
	for _, b := range t.Blocks {
		for i, it := range b.Items {
			where := fmt.Sprintf("blocks/%s.yaml: items[%d]", b.Slug, i)
			if it.Menu != nil {
				for j, o := range it.Menu.Of {
					if err := bad(fmt.Sprintf("%s: of[%d]", where, j), o.Dose); err != nil {
						return err
					}
				}
				continue
			}
			if err := bad(where, it.Dose); err != nil {
				return err
			}
		}
	}
	for _, sn := range t.Sessions {
		for i, it := range sn.Items {
			if err := bad(fmt.Sprintf("sessions/%s.yaml: items[%d]", sn.Slug, i), it.Dose); err != nil {
				return err
			}
		}
	}
	return nil
}

// tagName is the label a new tag gets. A person can rename it afterwards, and nothing here
// overwrites a name that is already there.
func tagName(slug string) string {
	words := strings.Split(slug, "_")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// scope narrows a query to the rows this import may touch: the app's, or one account's.
func (im *importer) scope(q *gorm.DB) *gorm.DB {
	if im.authorID == nil {
		return q.Where("author_id IS NULL")
	}
	return q.Where("author_id = ?", *im.authorID)
}

// upsert is the whole per-row decision. The match key is the uuid the file carries, scoped
// to the author, which is what makes a rename free. A menu has no file, so it is matched by
// its derived slug and keeps the uuid it was minted at insert.
func (im *importer) upsert(where string, want Content,
	tags []string, media []Media, perSet []SetEntry) error {

	tree := im.tree.Name
	want.AuthorID = im.authorID
	want.SourceTree = &tree
	ref := Ref{Kind: want.Kind, Slug: want.Slug}

	var have Content
	var err error
	if want.Kind == KindMenu {
		err = im.scope(im.tx.Where("kind = ? AND slug = ?", want.Kind, want.Slug)).
			First(&have).Error
	} else {
		err = im.scope(im.tx.Where("uuid = ?", want.UUID)).First(&have).Error
	}

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if want.Kind == KindMenu {
			want.UUID = uuid.NewString()
			want.Family = want.UUID
		}
		if err := im.refuseSlugClash(where, want, 0); err != nil {
			return err
		}
		want.CreatedAt = time.Now()
		want.UpdatedAt = want.CreatedAt
		if err := im.tx.Create(&want).Error; err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		im.ids[ref] = want.ID
		im.keep[ref] = true
		im.keepID[want.ID] = true
		im.res.Inserted++
		return im.syncChildren(where, want.ID, tags, media, perSet)

	case err != nil:
		return err

	// Edited in the app, which detached it from this file.
	case have.SourceTree == nil:
		im.ids[ref] = have.ID
		im.keep[ref] = true
		im.keepID[have.ID] = true
		im.detached[ref] = true
		im.res.Skipped = append(im.res.Skipped, where)
		return nil

	// Two trees claiming one row for one owner. Only a person can resolve that.
	case *have.SourceTree != tree:
		return fmt.Errorf("%s: id %s already belongs to tree %q, and tree %q also claims it",
			where, have.UUID, *have.SourceTree, tree)
	}

	im.ids[ref] = have.ID
	im.keep[ref] = true
	im.keepID[have.ID] = true
	want.ID = have.ID
	want.UUID = have.UUID
	want.CreatedAt = have.CreatedAt
	if want.Kind == KindMenu {
		want.Family = have.Family
	}
	// A file that came back un-retires its row.
	want.RetiredOn = nil
	if sameContent(have, want) {
		im.res.Unchanged++
	} else {
		if have.Slug != want.Slug {
			if err := im.refuseSlugClash(where, want, have.ID); err != nil {
				return err
			}
		}
		want.UpdatedAt = time.Now()
		if err := im.tx.Model(&Content{}).Where("id = ?", have.ID).
			Select(contentColumns).Updates(&want).Error; err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		im.res.Updated++
	}
	return im.syncChildren(where, have.ID, tags, media, perSet)
}

// refuseSlugClash names the two things that wanted one slug, rather than letting the unique
// index fail with a constraint name and nothing else. Only a person can say which keeps it.
func (im *importer) refuseSlugClash(where string, want Content, exceptID int64) error {
	q := im.scope(im.tx.Model(&Content{}).
		Where("kind = ? AND slug = ?", want.Kind, want.Slug))
	if exceptID != 0 {
		q = q.Where("id <> ?", exceptID)
	}
	var other Content
	err := q.First(&other).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	held := "a row edited in the app"
	if other.SourceTree != nil {
		held = fmt.Sprintf("a row from tree %q", *other.SourceTree)
	}
	return fmt.Errorf("%s: the %s slug %q is already held by %s (id %s). "+
		"Rename one of them", where, want.Kind, want.Slug, held, other.UUID)
}

func (im *importer) syncChildren(where string, id int64,
	tags []string, media []Media, perSet []SetEntry) error {

	if err := syncTagLinks(im.tx, id, tags); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	if err := syncMedia(im.tx, id, media); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	return syncContentSets(im.tx, id, perSet)
}

// contentColumns is what an import owns on a row. Named so Updates cannot reach a column
// the file has no opinion about, created_at especially.
var contentColumns = []string{
	// slug is here because the match key is the uuid: a rename is exactly the case where
	// the two differ, and leaving slug out would keep the old one.
	"slug", "family",
	"name", "notes", "source", "source_tree", "retired_on",
	"movement_style", "per_side",
	"d_sets", "d_reps", "d_weight_kg", "d_rep_seconds", "d_rep_rest_seconds",
	"d_set_rest_seconds", "d_prep_seconds", "d_seconds",
	"block_kind", "pick_count", "color", "needs", "updated_at",
}

// sameContent compares only what a file decides. A difference here is the only reason to
// write.
func sameContent(a, b Content) bool {
	return a.Slug == b.Slug &&
		a.Family == b.Family &&
		a.Name == b.Name &&
		a.Notes == b.Notes &&
		a.Source == b.Source &&
		eqStr(a.RetiredOn, b.RetiredOn) &&
		eqStr(a.MovementStyle, b.MovementStyle) &&
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

// familyOf reads the family a file declares. A file with no family line is the head of its
// own series.
func familyOf(f FileID) string {
	if f.Family != "" {
		return f.Family
	}
	return f.ID
}

func contentFromMovement(m MovementFile) Content {
	style := m.Style
	c := Content{
		Kind:          KindMovement,
		UUID:          m.ID,
		Family:        familyOf(m.FileID),
		Slug:          m.Slug,
		Name:          m.Name,
		Notes:         m.Notes,
		Source:        m.Source,
		MovementStyle: &style,
		PerSide:       m.PerSide,
	}
	doseToContent(&c, m.Dose)
	return c
}

// contentFromMenu falls back to "Pick N" for a menu with no name of its own.
func contentFromMenu(mn TreeMenu) Content {
	name := mn.Body.Name
	if name == "" {
		name = "Pick " + strconv.Itoa(*mn.Body.Pick)
	}
	return Content{
		Kind:      KindMenu,
		Slug:      mn.Slug,
		Name:      name,
		Notes:     mn.Body.Notes,
		PickCount: mn.Body.Pick,
	}
}

func contentFromBlock(b BlockFile) Content {
	c := Content{
		Kind:   KindBlock,
		UUID:   b.ID,
		Family: familyOf(b.FileID),
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
		UUID:   s.ID,
		Family: familyOf(s.FileID),
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
// Each of these compares before it writes, so an unchanged tree leaves the tables alone.

func syncTagLinks(tx *gorm.DB, contentID int64, tags []string) error {
	var want []int64
	for _, slug := range tags {
		var t Tag
		err := tx.Where("slug = ?", slug).First(&t).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			t = Tag{Slug: slug, Name: tagName(slug)}
			if err := tx.Create(&t).Error; err != nil {
				return fmt.Errorf("creating tag %q: %w", slug, err)
			}
		} else if err != nil {
			return err
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

// syncMedia keeps the list in file order, matched by position so row ids survive.
func syncMedia(tx *gorm.DB, contentID int64, media []Media) error {
	var have []ContentMedia
	if err := tx.Where("content_id = ?", contentID).Order("position, id").Find(&have).Error; err != nil {
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
func (im *importer) syncEdges(parent Ref, items []Item) error {
	parentID := im.ids[parent]
	var have []ContentItem
	if err := im.tx.Where("parent_id = ?", parentID).Order("position, id").Find(&have).Error; err != nil {
		return err
	}
	haveByChild := map[int64]ContentItem{}
	for _, e := range have {
		haveByChild[e.ChildID] = e
	}

	keep := map[int64]bool{}
	for i, it := range items {
		kind, written, err := it.what()
		if err != nil {
			return err
		}

		var (
			childID   int64
			childKind = kind
			dose      = it.Dose
			notes     = it.Notes
		)
		if kind == KindMenu {
			// The menu row's slug names its parent and this position, so it is already
			// known and needs no reference to resolve.
			childID = im.ids[Ref{Kind: KindMenu, Slug: parent.Slug + "_" + strconv.Itoa(i)}]
			if childID == 0 {
				return fmt.Errorf("items[%d]: the menu row is missing", i)
			}
			dose, notes = Dose{}, ""
		} else {
			childID, err = im.resolve(splitRef(kind, written))
			if err != nil {
				return fmt.Errorf("items[%d]: %w", i, err)
			}
		}
		keep[childID] = true

		want := ContentItem{
			ParentID: parentID, ParentKind: parent.Kind,
			ChildID: childID, ChildKind: childKind,
			Position: i, Notes: notes,
		}
		itemDose(&want, dose)

		old, exists := haveByChild[childID]
		if !exists {
			if err := im.tx.Create(&want).Error; err != nil {
				return err
			}
			if err := syncItemSets(im.tx, want.ID, dose.PerSet); err != nil {
				return err
			}
			continue
		}
		want.ID = old.ID
		if !sameItem(old, want) {
			if err := im.tx.Model(&ContentItem{}).Where("id = ?", old.ID).
				Select(itemColumns).Updates(&want).Error; err != nil {
				return err
			}
		}
		if err := syncItemSets(im.tx, old.ID, dose.PerSet); err != nil {
			return err
		}
	}

	for _, e := range have {
		if !keep[e.ChildID] {
			if err := im.tx.Where("id = ?", e.ID).Delete(&ContentItem{}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// resolve turns a reference into the row an edge should point at. Two namespaces, with no
// fallback between them: bare is this account's, app: is the app's.
func (im *importer) resolve(r Ref) (int64, error) {
	if id, ok := im.ids[r]; ok && id != 0 {
		return id, nil
	}

	q := im.tx.Select("id").Where("kind = ? AND slug = ?", r.Kind, r.Slug)
	if r.App {
		q = q.Where("author_id IS NULL")
	} else if im.authorID != nil {
		q = q.Where("author_id = ?", *im.authorID)
	} else {
		q = q.Where("author_id IS NULL")
	}

	var c Content
	err := q.First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if r.App {
			return 0, fmt.Errorf("no %s %q in the app's catalog", r.Kind, r.Slug)
		}
		return 0, fmt.Errorf("no %s %q in this tree or already in the database", r.Kind, r.Slug)
	}
	if err != nil {
		return 0, err
	}
	return c.ID, nil
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

// reapMissing handles a row whose file has left the tree. A movement, block or session is
// retired, not deleted: a plan or a finished run may still point at it. A menu is deleted,
// because nothing can reference one, and a retired menu would hold its derived slug against
// the next import of a reordered block.
func (im *importer) reapMissing() error {
	var rows []Content
	if err := im.scope(im.tx.Where("source_tree = ?", im.tree.Name)).Find(&rows).Error; err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	for _, r := range rows {
		if im.keepID[r.ID] {
			continue
		}
		if r.Kind == KindMenu {
			if err := im.tx.Where("parent_id = ? OR child_id = ?", r.ID, r.ID).
				Delete(&ContentItem{}).Error; err != nil {
				return err
			}
			if err := im.tx.Where("id = ?", r.ID).Delete(&Content{}).Error; err != nil {
				return err
			}
			continue
		}
		if r.RetiredOn == nil {
			d := Date(today)
			if err := im.tx.Model(&Content{}).Where("id = ?", r.ID).
				Update("retired_on", &d).Error; err != nil {
				return err
			}
			im.res.Retired++
		}
	}
	return nil
}
