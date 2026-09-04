package db

import (
	"fmt"

	"gorm.io/gorm"
)

// The catalog belongs to the app, but the rows are owned by whichever account ran the
// import. Publishing flags them shared, so every account reads them and nobody edits them
// in place.
//
// Only rows the importer created are eligible. A row a user made is theirs, and a row
// carrying an edit stamp is theirs too — publishing either would hand someone's own work
// to everyone.

// PublishReport says what publishing found and what it flagged.
type PublishReport struct {
	OwnerID       uint
	ByTable       map[string]int64
	Total         int64
	SkippedEdited map[string]int64
	SkippedUser   map[string]int64
	AlreadyShared int64
	DryRun        bool
}

// PublishCatalog flags the importer-created rows owned by ownerID as shared. Pass dryRun
// to see the counts without changing anything.
//
// The system open-session template is never published: it is a hidden per-user anchor, and
// one shared copy would give every account the same one.
func PublishCatalog(gdb *gorm.DB, ownerID uint, dryRun bool) (PublishReport, error) {
	rep := PublishReport{
		OwnerID:       ownerID,
		ByTable:       map[string]int64{},
		SkippedEdited: map[string]int64{},
		SkippedUser:   map[string]int64{},
	}
	if ownerID == 0 {
		return rep, fmt.Errorf("publish needs a real owner id")
	}
	var owner User
	if err := gdb.Where("id = ?", ownerID).First(&owner).Error; err != nil {
		return rep, fmt.Errorf("no account with id %d: %w", ownerID, err)
	}

	for _, tbl := range slugTables() {
		base := gdb.Model(tbl.model).Where("owner_id = ?", ownerID)
		if _, isSession := tbl.model.(*SessionTemplate); isSession {
			base = base.Where("is_system = ?", false)
		}

		var eligible, edited, user, already int64
		if err := base.Session(&gorm.Session{}).
			Where("managed_by_catalog = ? AND catalog_edited_at IS NULL AND shared = ?", true, false).
			Count(&eligible).Error; err != nil {
			return rep, err
		}
		if err := base.Session(&gorm.Session{}).
			Where("managed_by_catalog = ? AND catalog_edited_at IS NOT NULL", true).
			Count(&edited).Error; err != nil {
			return rep, err
		}
		if err := base.Session(&gorm.Session{}).
			Where("managed_by_catalog = ?", false).Count(&user).Error; err != nil {
			return rep, err
		}
		if err := base.Session(&gorm.Session{}).
			Where("shared = ?", true).Count(&already).Error; err != nil {
			return rep, err
		}

		if eligible > 0 {
			rep.ByTable[tbl.name] = eligible
			rep.Total += eligible
		}
		if edited > 0 {
			rep.SkippedEdited[tbl.name] = edited
		}
		if user > 0 {
			rep.SkippedUser[tbl.name] = user
		}
		rep.AlreadyShared += already

		if dryRun || eligible == 0 {
			continue
		}
		if err := base.Session(&gorm.Session{}).
			Where("managed_by_catalog = ? AND catalog_edited_at IS NULL AND shared = ?", true, false).
			Update("shared", true).Error; err != nil {
			return rep, err
		}
	}
	rep.DryRun = dryRun
	return rep, nil
}

// UnpublishCatalog takes every shared row back into private ownership. The way out, if
// publishing turns out to be wrong: it restores the state before PublishCatalog ran,
// because publishing changes nothing else about a row.
func UnpublishCatalog(gdb *gorm.DB, ownerID uint) (int64, error) {
	var total int64
	for _, tbl := range slugTables() {
		res := gdb.Model(tbl.model).Where("owner_id = ? AND shared = ?", ownerID, true).
			Update("shared", false)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	return total, nil
}
