package db

import (
	"gorm.io/gorm"
)

// The catalog belongs to the app, not to a user. Everyone reads it, nobody edits it, and
// saving your own version of something copies it to you. Three models can be part of it:
// LibraryExercise, ActivityTemplate and SessionTemplate.
//
// Two rules, and they are not symmetric:
//
//	reads   →  mine, plus the catalog
//	writes  →  mine only
//
// That asymmetry is the whole design. It is why the read side can ship on its own, and
// why every write needs a guard rather than a wider filter.

// Visible scopes a read of a catalog model to the caller's own rows plus the catalog.
// Every read of the three models goes through this. Nothing hand-writes an owner filter
// for them again — with 117 such filters in the codebase, one place to get right is the
// only version of this that stays correct.
func Visible(ownerID uint) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("owner_id = ? OR shared = ?", ownerID, true)
	}
}

// Mine scopes a read to the caller's own rows only, excluding the catalog. For the places
// that genuinely mean "what have I made" — an export, a count of your own work — where
// including the catalog would be wrong or would double-count.
func Mine(ownerID uint) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("owner_id = ? AND shared = ?", ownerID, false)
	}
}

// ErrSharedReadOnly is returned when a write targets a catalog row. Handlers turn it into
// a message telling the user to save their own copy.
var ErrSharedReadOnly = errNotEditable{}

type errNotEditable struct{}

func (errNotEditable) Error() string {
	return "this item belongs to the shared catalog and cannot be changed"
}

// GuardWritable reports whether the caller may change the given catalog row, by id.
// Returns ErrSharedReadOnly for a catalog row and ErrNotFound when the row is not theirs
// and not shared — the two cases a handler must tell apart, because one deserves "make a
// copy to change this" and the other is simply not found.
func GuardWritable(gdb *gorm.DB, model any, ownerID, id uint) error {
	var count int64
	if err := gdb.Model(model).Where("id = ?", id).
		Where("owner_id = ? AND shared = ?", ownerID, false).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 1 {
		return nil
	}
	var shared int64
	if err := gdb.Model(model).Where("id = ? AND shared = ?", id, true).
		Count(&shared).Error; err != nil {
		return err
	}
	if shared > 0 {
		return ErrSharedReadOnly
	}
	return ErrNotFound
}
