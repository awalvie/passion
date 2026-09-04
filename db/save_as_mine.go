package db

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Nobody edits a catalog row. Saving your own version copies it to you, and from that
// moment it is an ordinary row you own — every existing owner-scoped path applies to it
// unchanged, which is what keeps this cheap.
//
// The copy is deep, and it has to be. An Activity belongs to a SessionTemplate and has no
// reference to the ActivityTemplate it was expanded from, so a session's children are its
// own rows. There is nothing to point at.

// CopiedFrom records which catalog row a copy came from. It is not used to decide what a
// list shows — both rows appear, and the copy carries its own name — but it lets the UI
// say "based on Boulder Session" and makes the origin recoverable.

// copySuffix names a copy so the two are distinguishable in a list without any hiding
// rule. A user renames it immediately if they care.
const copySuffix = " (mine)"

// uniqueSlugFor returns a slug free for this owner, appending a counter if needed. Slug is
// identity, so a copy cannot share one with the row it came from.
func uniqueSlugFor(tx *gorm.DB, model any, ownerID uint, base string) (string, error) {
	if base == "" {
		base = "copy"
	}
	for i := 0; i < 100; i++ {
		candidate := base + "_mine"
		if i > 0 {
			candidate = fmt.Sprintf("%s_mine_%d", base, i+1)
		}
		var n int64
		if err := tx.Model(model).Where("owner_id = ? AND slug = ?", ownerID, candidate).
			Count(&n).Error; err != nil {
			return "", err
		}
		if n == 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a free slug for %q", base)
}

// SaveLibraryExerciseAsMine copies a catalog library exercise to the caller, with its
// media and its option rows.
func SaveLibraryExerciseAsMine(gdb *gorm.DB, ownerID, id uint) (LibraryExercise, error) {
	var copyRow LibraryExercise
	err := gdb.Transaction(func(tx *gorm.DB) error {
		var src LibraryExercise
		if err := tx.Scopes(Visible(ownerID)).Where("id = ?", id).First(&src).Error; err != nil {
			if isNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		slug, err := uniqueSlugFor(tx, &LibraryExercise{}, ownerID, src.Slug)
		if err != nil {
			return err
		}

		copyRow = src
		copyRow.ID = 0
		copyRow.CreatedAt, copyRow.UpdatedAt = zeroTime(), zeroTime()
		copyRow.OwnerID = ownerID
		copyRow.Shared = false
		// A copy is the user's own work from here. The importer never touches it, so it
		// needs neither the catalog-managed flag nor the edited stamp.
		copyRow.ManagedByCatalog = false
		copyRow.CatalogEditedAt = nil
		copyRow.Slug = slug
		copyRow.Name = src.Name + copySuffix
		copyRow.CopiedFromID = &src.ID
		copyRow.Media = nil
		copyRow.Children = nil
		copyRow.ParentLibraryExercise = nil
		if err := tx.Create(&copyRow).Error; err != nil {
			return err
		}
		if err := copyLibraryMedia(tx, ownerID, src.ID, copyRow.ID); err != nil {
			return err
		}
		return copyLibraryChildren(tx, ownerID, src.ID, copyRow.ID)
	})
	return copyRow, err
}

// copyLibraryMedia duplicates the media attached to a library exercise.
func copyLibraryMedia(tx *gorm.DB, ownerID, fromID, toID uint) error {
	var media []ExerciseMedia
	if err := tx.Where("library_exercise_id = ?", fromID).Order("order_index asc").Find(&media).Error; err != nil {
		return err
	}
	for _, m := range media {
		m.ID = 0
		m.CreatedAt, m.UpdatedAt = zeroTime(), zeroTime()
		m.OwnerID = ownerID
		m.LibraryExerciseID = &toID
		m.ExerciseID = nil
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
	}
	return nil
}

// copyLibraryChildren duplicates the option rows under a catalog parent.
func copyLibraryChildren(tx *gorm.DB, ownerID, fromID, toID uint) error {
	var kids []LibraryExercise
	if err := tx.Where("parent_library_exercise_id = ?", fromID).Order("order_index asc").Find(&kids).Error; err != nil {
		return err
	}
	for _, kid := range kids {
		srcID := kid.ID
		slug, err := uniqueSlugFor(tx, &LibraryExercise{}, ownerID, kid.Slug)
		if err != nil {
			return err
		}
		kid.ID = 0
		kid.CreatedAt, kid.UpdatedAt = zeroTime(), zeroTime()
		kid.OwnerID = ownerID
		kid.Shared = false
		kid.ManagedByCatalog = false
		kid.CatalogEditedAt = nil
		kid.Slug = slug
		kid.CopiedFromID = &srcID
		kid.ParentLibraryExerciseID = &toID
		kid.Media = nil
		kid.Children = nil
		kid.ParentLibraryExercise = nil
		if err := tx.Create(&kid).Error; err != nil {
			return err
		}
		if err := copyLibraryMedia(tx, ownerID, srcID, kid.ID); err != nil {
			return err
		}
	}
	return nil
}

// SaveActivityTemplateAsMine copies a catalog block and its exercises to the caller.
func SaveActivityTemplateAsMine(gdb *gorm.DB, ownerID, id uint) (ActivityTemplate, error) {
	var copyRow ActivityTemplate
	err := gdb.Transaction(func(tx *gorm.DB) error {
		var src ActivityTemplate
		if err := tx.Scopes(Visible(ownerID)).Where("id = ?", id).First(&src).Error; err != nil {
			if isNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		slug, err := uniqueSlugFor(tx, &ActivityTemplate{}, ownerID, src.Slug)
		if err != nil {
			return err
		}
		copyRow = src
		copyRow.ID = 0
		copyRow.CreatedAt, copyRow.UpdatedAt = zeroTime(), zeroTime()
		copyRow.OwnerID = ownerID
		copyRow.Shared = false
		copyRow.ManagedByCatalog = false
		copyRow.CatalogEditedAt = nil
		copyRow.Slug = slug
		copyRow.Name = src.Name + copySuffix
		copyRow.CopiedFromID = &src.ID
		copyRow.Exercises = nil
		if err := tx.Create(&copyRow).Error; err != nil {
			return err
		}
		return copyExercisesInto(tx, ownerID, "activity_template_id = ?", src.ID, func(e *Exercise) {
			e.ActivityTemplateID = &copyRow.ID
			e.ActivityID = nil
		})
	})
	return copyRow, err
}

// SaveSessionTemplateAsMine copies a catalog session to the caller, with its blocks and
// their exercises. The copy is deep because an Activity belongs to one SessionTemplate and
// keeps no reference to the block it was expanded from — there is nothing to share.
func SaveSessionTemplateAsMine(gdb *gorm.DB, ownerID, id uint) (SessionTemplate, error) {
	var copyRow SessionTemplate
	err := gdb.Transaction(func(tx *gorm.DB) error {
		var src SessionTemplate
		if err := tx.Scopes(Visible(ownerID)).Where("id = ?", id).First(&src).Error; err != nil {
			if isNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		slug, err := uniqueSlugFor(tx, &SessionTemplate{}, ownerID, src.Slug)
		if err != nil {
			return err
		}
		copyRow = src
		copyRow.ID = 0
		copyRow.CreatedAt, copyRow.UpdatedAt = zeroTime(), zeroTime()
		copyRow.OwnerID = ownerID
		copyRow.Shared = false
		copyRow.ManagedByCatalog = false
		copyRow.CatalogEditedAt = nil
		copyRow.Slug = slug
		copyRow.Name = src.Name + copySuffix
		copyRow.CopiedFromID = &src.ID
		copyRow.Activities = nil
		if err := tx.Create(&copyRow).Error; err != nil {
			return err
		}

		var acts []Activity
		if err := tx.Where("session_template_id = ?", src.ID).Order("order_index asc").Find(&acts).Error; err != nil {
			return err
		}
		for _, act := range acts {
			srcActID := act.ID
			act.ID = 0
			act.CreatedAt, act.UpdatedAt = zeroTime(), zeroTime()
			act.OwnerID = ownerID
			act.SessionTemplateID = copyRow.ID
			act.Exercises = nil
			if err := tx.Create(&act).Error; err != nil {
				return err
			}
			newActID := act.ID
			if err := copyExercisesInto(tx, ownerID, "activity_id = ?", srcActID, func(e *Exercise) {
				e.ActivityID = &newActID
				e.ActivityTemplateID = nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return copyRow, err
}

// copyExercisesInto duplicates the exercises matched by cond, reparenting each with attach.
// Option rows come across under their new parent, and media follows each exercise.
func copyExercisesInto(tx *gorm.DB, ownerID uint, cond string, arg any, attach func(*Exercise)) error {
	var exs []Exercise
	if err := tx.Where(cond, arg).Order("order_index asc").Find(&exs).Error; err != nil {
		return err
	}
	// Parents before their options, so a child can point at the new parent.
	newIDFor := map[uint]uint{}
	for _, pass := range []bool{false, true} {
		for _, src := range exs {
			isChild := src.ParentExerciseID != nil
			if isChild != pass {
				continue
			}
			srcID := src.ID
			src.ID = 0
			src.CreatedAt, src.UpdatedAt = zeroTime(), zeroTime()
			src.OwnerID = ownerID
			src.Media = nil
			src.ParentExercise = nil
			src.SessionRunID = nil
			if isChild {
				if mapped, ok := newIDFor[*src.ParentExerciseID]; ok {
					src.ParentExerciseID = &mapped
				} else {
					src.ParentExerciseID = nil
				}
			}
			attach(&src)
			if err := tx.Create(&src).Error; err != nil {
				return err
			}
			newIDFor[srcID] = src.ID
			var media []ExerciseMedia
			if err := tx.Where("exercise_id = ?", srcID).Order("order_index asc").Find(&media).Error; err != nil {
				return err
			}
			for _, m := range media {
				m.ID = 0
				m.CreatedAt, m.UpdatedAt = zeroTime(), zeroTime()
				m.OwnerID = ownerID
				m.ExerciseID = &src.ID
				m.LibraryExerciseID = nil
				if err := tx.Create(&m).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// TrimCopySuffix removes the suffix a copy is named with, for a UI that wants the original
// name back.
func TrimCopySuffix(name string) string {
	return strings.TrimSuffix(name, copySuffix)
}
