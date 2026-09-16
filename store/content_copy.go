package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotCopyable means the kind asked for has no structure to copy. Changing a movement's
// numbers is what MovementPref is for.
var ErrNotCopyable = errors.New("store: only a block or a session can be copied")

// CopyToMine gives an account its own copy of a session or block.
//
// Structure is copied and movements are not: a copied session holds new block rows, and
// those blocks point at the same movement rows as before. So the copy costs three rows, not
// three hundred, and a later release improving a movement still reaches you.
//
// The copy keeps its name and slug. Your slugs and the app's live in separate unique indexes
// and never meet, so there is nothing to work around, and keeping the name is what keeps
// history in one series.
func (s *Store) CopyToMine(ctx context.Context, accountID, id int64) (Content, error) {
	var out Content
	err := s.WithTx(ctx, func(tx *gorm.DB) error {
		var src Content
		if err := visible(tx.Where("id = ?", id), accountID).First(&src).Error; err != nil {
			return err
		}
		if src.Kind != KindSession && src.Kind != KindBlock {
			return fmt.Errorf("%w, and this is a %s", ErrNotCopyable, src.Kind)
		}

		copied, err := copyRow(tx, src, accountID)
		if err != nil {
			return err
		}
		if err := copyChildren(tx, src.ID, copied.ID, src.Kind, accountID); err != nil {
			return err
		}
		out = copied
		return nil
	})
	return out, err
}

// copyRow writes one row as the account's own. A new uuid, because two rows are two things;
// the family is inherited, which is what keeps their history one series.
func copyRow(tx *gorm.DB, src Content, accountID int64) (Content, error) {
	c := src
	c.ID = 0
	c.UUID = uuid.NewString()
	c.Family = src.Family
	c.AuthorID = &accountID
	c.SourceTree = nil
	c.RetiredOn = nil
	c.CreatedAt = time.Now()
	c.UpdatedAt = c.CreatedAt

	if err := tx.Create(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return Content{}, fmt.Errorf("you already have a %s called %q", src.Kind, src.Slug)
		}
		return Content{}, err
	}

	var links []ContentTag
	if err := tx.Where("content_id = ?", src.ID).Find(&links).Error; err != nil {
		return Content{}, err
	}
	for _, l := range links {
		if err := tx.Create(&ContentTag{ContentID: c.ID, TagID: l.TagID}).Error; err != nil {
			return Content{}, err
		}
	}
	return c, nil
}

// copyChildren rebuilds one parent's edges under the copy. A block or a menu under it is
// copied, because neither can be shared; a movement is pointed at.
func copyChildren(tx *gorm.DB, srcParent, newParent int64, parentKind string, accountID int64) error {
	var edges []ContentItem
	if err := tx.Where("parent_id = ?", srcParent).Order("position, id").
		Find(&edges).Error; err != nil {
		return err
	}

	for _, e := range edges {
		var child Content
		if err := tx.Where("id = ?", e.ChildID).First(&child).Error; err != nil {
			return err
		}

		childID := child.ID
		if child.Kind == KindBlock || child.Kind == KindMenu {
			copied, err := copyRow(tx, child, accountID)
			if err != nil {
				return err
			}
			if err := copyChildren(tx, child.ID, copied.ID, child.Kind, accountID); err != nil {
				return err
			}
			childID = copied.ID
		}

		edge := e
		edge.ID = 0
		edge.ParentID = newParent
		edge.ParentKind = parentKind
		edge.ChildID = childID
		if err := tx.Create(&edge).Error; err != nil {
			return err
		}
		if err := copyItemSets(tx, e.ID, edge.ID); err != nil {
			return err
		}
	}
	return nil
}

func copyItemSets(tx *gorm.DB, srcItem, newItem int64) error {
	var sets []ContentItemSet
	if err := tx.Where("content_item_id = ?", srcItem).Order("set_index, rep_index").
		Find(&sets).Error; err != nil {
		return err
	}
	for _, cs := range sets {
		cs.ContentItemID = newItem
		if err := tx.Create(&cs).Error; err != nil {
			return err
		}
	}
	return nil
}
