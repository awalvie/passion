package store

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// visible is the only place that decides what an account may read. Every content read goes
// through it, so a missed scope is a missing call rather than a subtly wrong WHERE clause.
func visible(q *gorm.DB, accountID int64) *gorm.DB {
	return q.Where("author_id IS NULL OR author_id = ?", accountID)
}

// ListContent returns one kind of content, the app's and the account's own, newest name
// order. Retired rows are left out; they still resolve by id so old plans keep working.
func (s *Store) ListContent(ctx context.Context, accountID int64, kind string) ([]Content, error) {
	var rows []Content
	err := visible(s.read(ctx).Where("kind = ? AND retired_on IS NULL", kind), accountID).
		Order("name").Find(&rows).Error
	return rows, err
}

func (s *Store) ContentByID(ctx context.Context, accountID, id int64) (Content, error) {
	var c Content
	err := visible(s.read(ctx).Where("id = ?", id), accountID).First(&c).Error
	return c, err
}

func (s *Store) ContentBySlug(ctx context.Context, accountID int64, kind, slug string) (Content, error) {
	var c Content
	// Own rows first, so a copy shadows the app's row of the same slug.
	err := visible(s.read(ctx).Where("kind = ? AND slug = ?", kind, slug), accountID).
		Order("author_id IS NULL").First(&c).Error
	return c, err
}

// Node is one row as its parent asks for it: the row itself, plus the numbers and notes that
// belong to this use of it rather than to the row.
type Node struct {
	Content
	EdgeNotes string
	Dose
	PerSet   []SetEntry
	Children []Node
}

// ContentTree reads a session, block or menu with everything under it. Three levels at most,
// because the chain is fixed: session holds blocks, a block holds movements or menus, a menu
// holds movements.
func (s *Store) ContentTree(ctx context.Context, accountID, id int64) (Node, error) {
	root, err := s.ContentByID(ctx, accountID, id)
	if err != nil {
		return Node{}, err
	}
	n := Node{Content: root}
	n.Children, err = s.childrenOf(ctx, accountID, root.ID)
	if err != nil {
		return Node{}, err
	}
	for i := range n.Children {
		n.Children[i].Children, err = s.childrenOf(ctx, accountID, n.Children[i].ID)
		if err != nil {
			return Node{}, err
		}
		for j := range n.Children[i].Children {
			gc := &n.Children[i].Children[j]
			gc.Children, err = s.childrenOf(ctx, accountID, gc.ID)
			if err != nil {
				return Node{}, err
			}
		}
	}
	return n, nil
}

func (s *Store) childrenOf(ctx context.Context, accountID, parentID int64) ([]Node, error) {
	db := s.read(ctx)

	var edges []ContentItem
	if err := db.Where("parent_id = ?", parentID).Order("position, id").
		Find(&edges).Error; err != nil {
		return nil, err
	}
	if len(edges) == 0 {
		return nil, nil
	}

	out := make([]Node, 0, len(edges))
	for _, e := range edges {
		child, err := s.ContentByID(ctx, accountID, e.ChildID)
		if err != nil {
			// An edge into another account's content. The picker is meant to make this
			// impossible, so say which edge rather than rendering a hole.
			return nil, fmt.Errorf("store: content %d holds %d, which this account cannot read",
				parentID, e.ChildID)
		}
		n := Node{
			Content:   child,
			EdgeNotes: e.Notes,
			Dose: Dose{
				Sets: e.Sets, Reps: e.Reps, WeightKg: e.WeightKg,
				RepSeconds: e.RepSeconds, RepRestSeconds: e.RepRestSeconds,
				SetRestSeconds: e.SetRestSeconds, PrepSeconds: e.PrepSeconds,
				Seconds: e.Seconds,
			},
		}
		if n.PerSet, err = itemSets(db, e.ID); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// Mine reports whether the account owns this row rather than the app shipping it. It is what
// the library badges: everything else about where a row came from is the importer's business.
func (c Content) Mine() bool { return c.AuthorID != nil }
