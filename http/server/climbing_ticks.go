package web

import (
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

// tickStyleDisplay converts a stored style key to a display string.
func tickStyleDisplay(s string) (display, cssClass string) {
	switch s {
	case "onsight":
		return "Onsight", "onsight"
	case "flash":
		return "Flash", "flash"
	case "redpoint":
		return "Redpoint", "redpoint"
	case "project":
		return "Project", "project"
	case "repeat":
		return "Repeat", "repeat"
	case "top_rope":
		return "Top Rope", "top-rope"
	}
	return "", ""
}

func ticksToViews(ticks []db.ClimbingTick) []pages.ClimbingTickView {
	views := make([]pages.ClimbingTickView, 0, len(ticks))
	for _, t := range ticks {
		kind := "Boulder"
		if t.Kind == "route" {
			kind = "Route"
		}
		style, styleClass := tickStyleDisplay(t.Style)
		views = append(views, pages.ClimbingTickView{
			ID:         t.ID,
			RunID:      t.RunID,
			ExerciseID: t.ExerciseID,
			Kind:       kind,
			KindRaw:    t.Kind,
			Grade:      t.Grade,
			Focus:      t.Focus,
			Thoughts:   t.Thoughts,
			Style:      style,
			StyleClass: styleClass,
			StyleRaw:   t.Style,
			Attempts:   t.Attempts,
			Sent:       t.Sent,
			Stars:      t.Stars,
		})
	}
	return views
}

// handleExerciseTicks serves GET|POST /runs/{runID}/exercises/{exerciseID}/ticks.
func (s *Server) handleExerciseTicks(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)

	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise ID", http.StatusBadRequest)
		return
	}

	// Validate that the exercise belongs to this run and owner.
	var ex db.Exercise
	if err := s.store.DB.Where("owner_id = ? AND id = ? AND session_run_id = ?",
		ownerID, exerciseID, runID).First(&ex).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Also accept exercises from activity-based runs (via ActivityID).
			if err2 := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, exerciseID).
				First(&ex).Error; err2 != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
		} else {
			s.serverError(w, r, err)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		s.serveExerciseTicks(w, r, ownerID, runID, exerciseID)
	case http.MethodPost:
		s.createExerciseTick(w, r, ownerID, runID, exerciseID)
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) serveExerciseTicks(w http.ResponseWriter, r *http.Request, ownerID, runID, exerciseID uint) {
	ticks, err := db.ListClimbingTicksByExercise(s.store.DB, ownerID, runID, exerciseID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.pages.RenderExerciseTicks(w, pages.ExerciseTicksParams{
		RunID:      runID,
		ExerciseID: exerciseID,
		Ticks:      ticksToViews(ticks),
	})
}

func (s *Server) createExerciseTick(w http.ResponseWriter, r *http.Request, ownerID, runID, exerciseID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	parseInt := func(key string, def int) int {
		v, err := strconv.Atoi(strings.TrimSpace(r.FormValue(key)))
		if err != nil {
			return def
		}
		return v
	}

	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind != "boulder" && kind != "route" {
		kind = "boulder"
	}

	attempts := parseInt("attempts", 1)
	if attempts < 1 {
		attempts = 1
	}
	stars := parseInt("stars", 0)
	if stars < 0 || stars > 3 {
		stars = 0
	}

	tick := &db.ClimbingTick{
		OwnerID:    ownerID,
		RunID:      runID,
		ExerciseID: exerciseID,
		Kind:       kind,
		Grade:      strings.TrimSpace(r.FormValue("grade")),
		Focus:      strings.TrimSpace(r.FormValue("focus")),
		Thoughts:   strings.TrimSpace(r.FormValue("thoughts")),
		Style:      strings.TrimSpace(r.FormValue("style")),
		Attempts:   attempts,
		Sent:       r.FormValue("sent") == "1",
		Stars:      stars,
	}

	if err := db.CreateClimbingTick(s.store.DB, tick); err != nil {
		s.serverError(w, r, err)
		return
	}

	s.serveExerciseTicks(w, r, ownerID, runID, exerciseID)
}

// handleExerciseTickDelete serves POST /runs/{runID}/exercises/{exerciseID}/ticks/{tickID}/delete.
func (s *Server) handleExerciseTickDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)

	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise ID", http.StatusBadRequest)
		return
	}
	tickID, err := parseUintParam(r, "tickID")
	if err != nil {
		http.Error(w, "invalid tick ID", http.StatusBadRequest)
		return
	}

	if err := db.DeleteClimbingTick(s.store.DB, ownerID, uint(tickID)); err != nil {
		s.serverError(w, r, err)
		return
	}

	s.serveExerciseTicks(w, r, ownerID, runID, exerciseID)
}

// handleExerciseTickUpdate serves POST /runs/{runID}/exercises/{exerciseID}/ticks/{tickID}/update.
func (s *Server) handleExerciseTickUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)

	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise ID", http.StatusBadRequest)
		return
	}
	tickID, err := parseUintParam(r, "tickID")
	if err != nil {
		http.Error(w, "invalid tick ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	parseInt := func(key string, def int) int {
		v, err := strconv.Atoi(strings.TrimSpace(r.FormValue(key)))
		if err != nil {
			return def
		}
		return v
	}

	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind != "boulder" && kind != "route" {
		kind = "boulder"
	}
	attempts := parseInt("attempts", 1)
	if attempts < 1 {
		attempts = 1
	}
	stars := parseInt("stars", 0)
	if stars < 0 || stars > 3 {
		stars = 0
	}

	if err := db.UpdateClimbingTick(s.store.DB, ownerID, tickID,
		kind,
		strings.TrimSpace(r.FormValue("grade")),
		strings.TrimSpace(r.FormValue("focus")),
		strings.TrimSpace(r.FormValue("thoughts")),
		strings.TrimSpace(r.FormValue("style")),
		attempts, stars,
		r.FormValue("sent") == "1",
	); err != nil {
		s.serverError(w, r, err)
		return
	}

	s.serveExerciseTicks(w, r, ownerID, runID, exerciseID)
}

// ---------------------------------------------------------------------------
// Profile: venue and board management
// ---------------------------------------------------------------------------

func venueToView(v db.ClimbingVenue) pages.ClimbingVenueView {
	kind := "Commercial"
	if v.Kind == "outdoor" {
		kind = "Outdoor"
	}
	return pages.ClimbingVenueView{ID: v.ID, Name: v.Name, Kind: kind}
}

func boardToView(b db.ClimbingBoard) pages.ClimbingBoardView {
	typeDisplay := map[string]string{
		"kilter":  "Kilter",
		"moon":    "Moon",
		"tension": "Tension",
		"spray":   "Spray",
		"custom":  "Custom",
	}
	td := typeDisplay[b.BoardType]
	if td == "" {
		td = b.BoardType
	}
	label := b.Name
	if label == "" {
		label = td + " Board"
	}
	return pages.ClimbingBoardView{
		ID:        b.ID,
		BoardType: td,
		Name:      b.Name,
		Label:     label,
	}
}

func (s *Server) loadVenuesAndBoards(ownerID uint) ([]pages.ClimbingVenueView, []pages.ClimbingBoardView, error) {
	venues, err := db.ListClimbingVenues(s.store.DB, ownerID)
	if err != nil {
		return nil, nil, err
	}
	boards, err := db.ListClimbingBoards(s.store.DB, ownerID)
	if err != nil {
		return nil, nil, err
	}
	vv := make([]pages.ClimbingVenueView, 0, len(venues))
	for _, v := range venues {
		vv = append(vv, venueToView(v))
	}
	bv := make([]pages.ClimbingBoardView, 0, len(boards))
	for _, b := range boards {
		bv = append(bv, boardToView(b))
	}
	return vv, bv, nil
}

// handleProfileVenues serves GET|POST /profile/venues.
func (s *Server) handleProfileVenues(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form data", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		kind := strings.TrimSpace(r.FormValue("kind"))
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if kind != "commercial" && kind != "outdoor" {
			kind = "commercial"
		}
		if err := db.CreateClimbingVenue(s.store.DB, &db.ClimbingVenue{
			OwnerID: ownerID, Name: name, Kind: kind,
		}); err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	vv, _, err := s.loadVenuesAndBoards(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.pages.RenderVenuesList(w, pages.ProfileParams{Venues: vv})
}

// handleProfileVenueDelete serves POST /profile/venues/{venueID}/delete.
func (s *Server) handleProfileVenueDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	id, err := parseUintParam(r, "venueID")
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	if err := db.DeleteClimbingVenue(s.store.DB, ownerID, uint(id)); err != nil {
		s.serverError(w, r, err)
		return
	}
	vv, _, err := s.loadVenuesAndBoards(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.pages.RenderVenuesList(w, pages.ProfileParams{Venues: vv})
}

// handleProfileBoards serves GET|POST /profile/boards.
func (s *Server) handleProfileBoards(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form data", http.StatusBadRequest)
			return
		}
		boardType := strings.TrimSpace(r.FormValue("board_type"))
		if boardType == "" {
			http.Error(w, "board type required", http.StatusBadRequest)
			return
		}
		if err := db.CreateClimbingBoard(s.store.DB, &db.ClimbingBoard{
			OwnerID:   ownerID,
			BoardType: boardType,
			Name:      strings.TrimSpace(r.FormValue("name")),
		}); err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	_, bv, err := s.loadVenuesAndBoards(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.pages.RenderBoardsList(w, pages.ProfileParams{Boards: bv})
}

// handleProfileBoardDelete serves POST /profile/boards/{boardID}/delete.
func (s *Server) handleProfileBoardDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	id, err := parseUintParam(r, "boardID")
	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	if err := db.DeleteClimbingBoard(s.store.DB, ownerID, uint(id)); err != nil {
		s.serverError(w, r, err)
		return
	}
	_, bv, err := s.loadVenuesAndBoards(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.pages.RenderBoardsList(w, pages.ProfileParams{Boards: bv})
}
