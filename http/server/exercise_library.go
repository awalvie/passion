package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"passion/db"
	"passion/pages"
)

func (s *Server) handleExerciseLibraryIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	ownerID := s.mustUserID(r)

	// Search, filter, sort params
	searchQ := strings.TrimSpace(r.URL.Query().Get("q"))
	sourceFilter := strings.TrimSpace(r.URL.Query().Get("source"))
	tagFilter := strings.TrimSpace(r.URL.Query().Get("tag"))
	kindFilter := r.URL.Query().Get("kind")
	// Validate against NormalizeKind (the single source of truth for the kind set)
	// rather than a hand-maintained map that drifts — it previously omitted "climbing".
	if kindFilter == "" || db.NormalizeKind(kindFilter) != kindFilter {
		kindFilter = ""
	}
	sortParam := r.URL.Query().Get("sort")
	validSorts := map[string]bool{"name": true, "name_desc": true, "newest": true, "oldest": true}
	if !validSorts[sortParam] {
		sortParam = "name"
	}

	const pageSize = 25
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 1 {
		page = p
	}

	base := s.store.DB.Model(&db.LibraryExercise{}).
		Where("owner_id = ? AND parent_library_exercise_id IS NULL", ownerID)
	if searchQ != "" {
		base = base.Where("name LIKE ?", "%"+searchQ+"%")
	}
	if kindFilter != "" {
		base = base.Where("kind = ?", kindFilter)
	}
	if sourceFilter != "" {
		base = base.Where("source = ?", sourceFilter)
	}
	if tagFilter != "" {
		base = base.Where(db.LabelTagCondition(tagFilter))
	}

	var total int64
	base.Count(&total)

	totalPages := int((total + pageSize - 1) / pageSize)
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	var orderClause string
	switch sortParam {
	case "name_desc":
		orderClause = "name DESC"
	case "newest":
		orderClause = "created_at DESC"
	case "oldest":
		orderClause = "created_at ASC"
	default:
		orderClause = "name ASC"
	}

	var rows []db.LibraryExercise
	err := base.Order(orderClause).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// Build query string for pagination links (preserve search/filter/sort)
	qs := func(p int) string {
		params := fmt.Sprintf("page=%d", p)
		if searchQ != "" {
			params += "&q=" + searchQ
		}
		if kindFilter != "" {
			params += "&kind=" + kindFilter
		}
		if sourceFilter != "" {
			params += "&source=" + url.QueryEscape(sourceFilter)
		}
		if tagFilter != "" {
			params += "&tag=" + url.QueryEscape(tagFilter)
		}
		if sortParam != "name" {
			params += "&sort=" + sortParam
		}
		return "/exercise-library?" + params
	}

	var pagination *pages.PaginationView
	if totalPages > 1 {
		pv := pages.PaginationView{
			Page:       page,
			TotalPages: totalPages,
			HasPrev:    page > 1,
			HasNext:    page < totalPages,
		}
		if pv.HasPrev {
			pv.PrevURL = qs(page - 1)
		}
		if pv.HasNext {
			pv.NextURL = qs(page + 1)
		}
		pagination = &pv
	}

	distinctSources, err := db.DistinctLibrarySources(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	distinctTags, err := db.DistinctLibraryTags(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.pages.LibraryList(w, pages.LibraryListParams{
		Base:             pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		LibraryExercises: rows,
		Pagination:       pagination,
		LibrarySearch:    searchQ,
		LibraryKind:      kindFilter,
		LibrarySort:      sortParam,
		LibrarySource:    sourceFilter,
		LibraryTag:       tagFilter,
		DistinctSources:  distinctSources,
		DistinctTags:     distinctTags,
	})
}

func (s *Server) handleExerciseLibraryNew(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	switch r.Method {
	case http.MethodGet:
		lib, _ := s.listLibraryExercises(ownerID)
		s.pages.NewLibraryExercise(w, pages.NewLibraryExerciseParams{
			Base:             pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
			LibraryExercises: lib,
		})
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}
		row, errMsg := s.libraryExerciseFromForm(r, nil)
		if errMsg != "" {
			lib, _ := s.listLibraryExercises(ownerID)
			s.pages.NewLibraryExercise(w, pages.NewLibraryExerciseParams{
				LibraryExerciseFormErr: errMsg,
				LibraryExercises:       lib,
			})
			return
		}
		row.OwnerID = ownerID
		if err := s.store.DB.Create(row).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		// Sync media for the library exercise.
		if err := s.syncLibraryExerciseMediaFromForm(r, row.ID, ownerID); err != nil {
			s.serverError(w, r, err)
			return
		}

		// Create child library exercises for exercise_catalog.
		if row.Kind == "exercise_catalog" {
			if err := s.syncLibraryCatalogChildren(r, row.ID, ownerID); err != nil {
				s.serverError(w, r, err)
				return
			}
		}

		w.Header().Set("HX-Redirect", "/exercise-library")
		w.WriteHeader(http.StatusOK)
		return
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleExerciseLibraryByID(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	idStr := chi.URLParam(r, "libraryExerciseID")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	id := uint(id64)

	action := chi.URLParam(r, "action")
	switch action {
	case "edit":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		var row db.LibraryExercise
		if err := s.store.DB.
			Where("owner_id = ? AND id = ?", ownerID, id).
			Preload("Media").
			First(&row).Error; err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var children []db.LibraryExercise
		if row.Kind == "exercise_catalog" {
			s.store.DB.Where("owner_id = ? AND parent_library_exercise_id = ?", ownerID, id).
				Order("order_index asc").Find(&children)
		}
		lib, _ := s.listLibraryExercises(ownerID)
		s.pages.EditLibraryExercise(w, pages.EditLibraryExerciseParams{
			Base:                    pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
			LibraryExercise:         &row,
			LibraryExerciseChildren: children,
			LibraryExercises:        lib,
			CatalogImportEnabled:    s.yamlImport != nil,
		})
		return
	case "update":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}
		var existing db.LibraryExercise
		if err := s.store.DB.
			Where("owner_id = ? AND id = ?", ownerID, id).
			First(&existing).Error; err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var children []db.LibraryExercise
		s.store.DB.Where("owner_id = ? AND parent_library_exercise_id = ?", ownerID, id).
			Order("order_index asc").Find(&children)
		row, errMsg := s.libraryExerciseFromForm(r, &existing)
		if errMsg != "" {
			lib, _ := s.listLibraryExercises(ownerID)
			s.pages.EditLibraryExercise(w, pages.EditLibraryExerciseParams{
				Base:                    pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
				LibraryExercise:         &existing,
				LibraryExerciseChildren: children,
				LibraryExerciseFormErr:  errMsg,
				LibraryExercises:        lib,
				CatalogImportEnabled:    s.yamlImport != nil,
			})
			return
		}
		existing.Name = row.Name
		existing.Label = row.Label
		existing.Source = row.Source
		existing.Notes = row.Notes
		existing.Kind = row.Kind
		existing.SessionDurationSeconds = row.SessionDurationSeconds
		existing.Sets = row.Sets
		existing.Reps = row.Reps
		existing.RepSeconds = row.RepSeconds
		existing.RepRestSeconds = row.RepRestSeconds
		existing.SetRestSeconds = row.SetRestSeconds
		existing.PrepSeconds = row.PrepSeconds
		existing.RungSeconds = row.RungSeconds
		existing.WeightKg = row.WeightKg
		if err := s.store.DB.Save(&existing).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		// Sync media for the library exercise.
		if err := s.syncLibraryExerciseMediaFromForm(r, existing.ID, ownerID); err != nil {
			s.serverError(w, r, err)
			return
		}

		// Sync catalog children: delete old, re-create from form.
		if existing.Kind == "exercise_catalog" {
			if err := s.syncLibraryCatalogChildren(r, existing.ID, ownerID); err != nil {
				s.serverError(w, r, err)
				return
			}
		} else {
			// Kind changed away from catalog — remove any leftover children.
			s.store.DB.Where("owner_id = ? AND parent_library_exercise_id = ?", ownerID, existing.ID).
				Delete(&db.LibraryExercise{})
		}

		s.markLibraryExerciseEdited(ownerID, existing.ID)

		w.Header().Set("HX-Redirect", "/exercise-library")
		w.WriteHeader(http.StatusOK)
		return
	case "reset-catalog":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		if err := s.resetCatalogRow(&db.LibraryExercise{}, ownerID, id); err != nil {
			s.catalogResetError(w, r, err)
			return
		}
		w.Header().Set("HX-Redirect", "/exercise-library/"+strconv.FormatUint(uint64(id), 10)+"/edit")
		w.WriteHeader(http.StatusOK)
		return
	case "export":
		s.handleExportLibraryExercise(w, r, ownerID, id)
		return
	case "delete":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		// Delete children first (catalog options), then the parent.
		s.store.DB.Where("owner_id = ? AND parent_library_exercise_id = ?", ownerID, id).
			Delete(&db.LibraryExercise{})
		if err := s.store.DB.
			Where("owner_id = ? AND id = ?", ownerID, id).
			Delete(&db.LibraryExercise{}).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("HX-Redirect", "/exercise-library")
		w.WriteHeader(http.StatusOK)
		return
	default:
		http.NotFound(w, r)
	}
}

// handleSaveToLibraryFromActivity creates a new LibraryExercise from a minimal form
// posted inside the template editor, then re-renders the exercises fragment so the
// library dropdown refreshes immediately.
func (s *Server) handleSaveToLibraryFromActivity(w http.ResponseWriter, r *http.Request, activityID uint, ownerID uint) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}
	row, errMsg := s.libraryExerciseFromForm(r, nil)
	if errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}
	row.OwnerID = ownerID
	if err := s.store.DB.Create(row).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	act, err := s.loadActivityExercises(activityID, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderExercisesWithPreview(w, r, act, ownerID)
}

// libraryExerciseFromForm parses POST fields into a LibraryExercise. If base is non-nil, missing name uses base.Name.
func (s *Server) libraryExerciseFromForm(r *http.Request, base *db.LibraryExercise) (*db.LibraryExercise, string) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		if base != nil && strings.TrimSpace(base.Name) != "" {
			name = base.Name
		}
	}
	if name == "" {
		return nil, "Name is required."
	}

	row := &db.LibraryExercise{
		Name:   name,
		Label:  strings.TrimSpace(r.FormValue("label")),
		Source: strings.TrimSpace(r.FormValue("source")),
		Notes:  strings.TrimSpace(r.FormValue("notes")),
	}

	kind := db.NormalizeKind(r.FormValue("kind"))
	row.Kind = kind
	if kind == "session" {
		row.SessionDurationSeconds = parseSessionDurationSeconds(r)
	} else if kind == "exercise_catalog" {
		// Keep zero defaults for non-reps fields.
	} else if kind == "timed_reps" {
		row.Sets = formInt(r, "sets")
		row.Reps = formInt(r, "reps")
		row.RepSeconds = formInt(r, "rep_seconds")
		row.RepRestSeconds = formInt(r, "rep_rest_seconds")
		row.SetRestSeconds = formInt(r, "set_rest_seconds")
		row.PrepSeconds = formInt(r, "prep_seconds")
		row.RungSeconds = strings.TrimSpace(r.FormValue("rung_seconds"))
		row.WeightKg = formFloat(r, "weight_kg")
	} else {
		// reps_and_sets: counter only
		row.Sets = formInt(r, "sets")
		row.Reps = formInt(r, "reps")
		row.WeightKg = formFloat(r, "weight_kg")
	}

	return row, ""
}

// syncLibraryCatalogChildren deletes existing children of a catalog library
// exercise and re-creates them from the submitted option_name[] / option_library_id[] form fields.
func (s *Server) syncLibraryCatalogChildren(r *http.Request, parentID uint, ownerID uint) error {
	// Remove old children first.
	s.store.DB.Where("owner_id = ? AND parent_library_exercise_id = ?", ownerID, parentID).
		Delete(&db.LibraryExercise{})

	names := r.Form["option_name"]
	libIDs := r.Form["option_library_id"]

	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		child := db.LibraryExercise{
			OwnerID:                 ownerID,
			ParentLibraryExerciseID: &parentID,
			Name:                    name,
			Kind:                    "reps_and_sets",
			OrderIndex:              i,
		}

		// If a library ID was provided, copy fields from that exercise.
		if i < len(libIDs) {
			libIDStr := strings.TrimSpace(libIDs[i])
			if libIDStr != "" {
				if lid, err := strconv.ParseUint(libIDStr, 10, 64); err == nil {
					var src db.LibraryExercise
					if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, uint(lid)).First(&src).Error; err == nil {
						child.Kind = src.Kind
						child.Notes = src.Notes
						child.SessionDurationSeconds = src.SessionDurationSeconds
						child.Sets = src.Sets
						child.Reps = src.Reps
						child.RepSeconds = src.RepSeconds
						child.RepRestSeconds = src.RepRestSeconds
						child.SetRestSeconds = src.SetRestSeconds
						child.PrepSeconds = src.PrepSeconds
						child.RungSeconds = src.RungSeconds
						child.WeightKg = src.WeightKg
					}
				}
			}
		}

		if err := s.store.DB.Create(&child).Error; err != nil {
			return err
		}
	}
	return nil
}
