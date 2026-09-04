package web

import (
	"errors"
	"log/slog"
	"time"

	"passion/db"
)

// The YAML importer overwrites every field of the row it matches, and for blocks and
// sessions it deletes and recreates the child rows. So an edit made in the UI used to
// disappear on the next restart, and a rename used to be deleted outright by
// pruneCatalogOrphans. Stamping CatalogEditedAt is what stops both: the importer skips a
// stamped row entirely.
//
// Every handler that mutates a LibraryExercise, ActivityTemplate or SessionTemplate — or
// any child row belonging to one — must call one of these. A handler that forgets leaves
// that particular edit still being silently reverted.
//
// None of them fail the request. By the time they run the user's edit has already
// committed, so returning an error here would show them a 500 for a change that worked —
// and over HTMX the fragment would never re-render. A failed stamp is logged and the
// request carries on, the same way the planned-set cleanup in handleDeleteExercise does.

// errCatalogImportDisabled is returned by resetCatalogRow when there is no catalog
// configured to reset to.
var errCatalogImportDisabled = errors.New("catalog import is disabled")

// stampCatalogEdited marks one catalog row as user-edited. The WHERE clause does the
// filtering: a row the user created themselves (managed_by_catalog = 0) is never stamped,
// and a row already stamped keeps its original timestamp, so "Edited" reports when the
// user first changed the row rather than when they last touched it.
func (s *Server) stampCatalogEdited(model any, ownerID, id uint) {
	err := s.store.DB.Model(model).
		Where("owner_id = ? AND id = ? AND managed_by_catalog = ? AND catalog_edited_at IS NULL",
			ownerID, id, true).
		Update("catalog_edited_at", time.Now()).Error
	if err != nil {
		s.logger.Error("failed to stamp catalog row as edited",
			slog.Uint64("owner_id", uint64(ownerID)),
			slog.Uint64("id", uint64(id)),
			slog.String("error", err.Error()),
		)
	}
}

func (s *Server) markSessionTemplateEdited(ownerID, id uint) {
	s.stampCatalogEdited(&db.SessionTemplate{}, ownerID, id)
}

func (s *Server) markActivityTemplateEdited(ownerID, id uint) {
	s.stampCatalogEdited(&db.ActivityTemplate{}, ownerID, id)
}

func (s *Server) markLibraryExerciseEdited(ownerID, id uint) {
	s.stampCatalogEdited(&db.LibraryExercise{}, ownerID, id)
}

// markActivityEdited stamps the session template an activity belongs to.
func (s *Server) markActivityEdited(ownerID, activityID uint) {
	var act db.Activity
	if err := s.store.DB.Select("session_template_id").
		Where("owner_id = ? AND id = ?", ownerID, activityID).
		First(&act).Error; err != nil {
		s.logger.Error("failed to find the session template for an activity",
			slog.Uint64("activity_id", uint64(activityID)),
			slog.String("error", err.Error()),
		)
		return
	}
	s.markSessionTemplateEdited(ownerID, act.SessionTemplateID)
}

// markExerciseOwnerEdited stamps whichever catalog row an exercise belongs to. An exercise
// hangs off an Activity (inside a session template), an ActivityTemplate (a block), or a
// SessionRun. Only the first two are catalog rows; a run's exercises are the user's own
// copies and are never imported, so those are left alone.
func (s *Server) markExerciseOwnerEdited(ownerID uint, ex *db.Exercise) {
	if ex.ActivityID != nil {
		s.markActivityEdited(ownerID, *ex.ActivityID)
		return
	}
	if ex.ActivityTemplateID != nil {
		s.markActivityTemplateEdited(ownerID, *ex.ActivityTemplateID)
	}
}

// markExerciseIDOwnerEdited is markExerciseOwnerEdited for a handler that has only the id.
func (s *Server) markExerciseIDOwnerEdited(ownerID, exerciseID uint) {
	var ex db.Exercise
	if err := s.store.DB.Select("id", "activity_id", "activity_template_id").
		Where("owner_id = ? AND id = ?", ownerID, exerciseID).
		First(&ex).Error; err != nil {
		s.logger.Error("failed to find the owner of an exercise",
			slog.Uint64("exercise_id", uint64(exerciseID)),
			slog.String("error", err.Error()),
		)
		return
	}
	s.markExerciseOwnerEdited(ownerID, &ex)
}

// resetCatalogRow clears the edited stamp and re-runs the import for the owner, so the row
// goes back to what the YAML says.
//
// Only the account that holds the catalog may do this. The import writes the whole catalog
// into whichever account it runs for, so calling it as anyone else would copy the catalog —
// including the licensed part — into their account. That was the leak this project spent a
// day closing.
//
// Under the shared catalog nobody edits a catalog row in place, so there is nothing left to
// reset: a user's own copies are not importer-created and never match the WHERE below. The
// feature is dead for everyone but the catalog owner, and this makes that explicit rather
// than incidental.
func (s *Server) resetCatalogRow(model any, ownerID, id uint) error {
	if s.yamlImport == nil {
		return errCatalogImportDisabled
	}
	if s.catalogOwnerID == 0 || ownerID != s.catalogOwnerID {
		return db.ErrNotFound
	}
	// Report a miss instead of a success. This matched zero rows for a catalog row and
	// still redirected as if it had worked, so the row stayed exactly as it was.
	res := s.store.DB.Model(model).
		Where("owner_id = ? AND id = ? AND managed_by_catalog = ?", ownerID, id, true).
		Update("catalog_edited_at", nil)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return db.ErrNotFound
	}
	opts := *s.yamlImport
	opts.OwnerID = ownerID
	return s.store.ImportYAML(opts)
}
