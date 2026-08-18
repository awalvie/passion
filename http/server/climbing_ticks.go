package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

// tickStyleDisplay converts a stored ascent style key to display text, CSS class, and icon.
func tickStyleDisplay(s string) (display, cssClass, icon string) {
	switch s {
	case "onsight":
		return "Onsight", "onsight", "eye"
	case "flash":
		return "Flash", "flash", "zap"
	case "redpoint":
		return "Redpoint", "redpoint", "check-circle"
	case "hangdog":
		return "Hangdog", "hangdog", "anchor"
	case "repeat":
		return "Repeat", "repeat", "repeat-2"
	case "attempt":
		return "Attempt", "attempt", "x-circle"
	case "project": // backwards compat alias
		return "Attempt", "attempt", "x-circle"
	}
	return "", "", ""
}

// tickRopeStyleDisplay converts a stored method key to display text, CSS class, and icon.
func tickRopeStyleDisplay(s string) (display, cssClass, icon string) {
	switch s {
	case "lead":
		return "Lead", "lead", "link-2"
	case "top_rope":
		return "Top Rope", "top-rope", "arrow-down-to-line"
	case "auto_belay":
		return "Auto-belay", "auto-belay", "rotate-ccw"
	case "follow", "second": // "second" is legacy
		return "Top Rope", "follow", "users"
	}
	return "", "", ""
}

// tickSubtypeDisplay converts a subtype key to display text (board kind for boulder+indoor).
func tickSubtypeDisplay(s string) string {
	switch s {
	case "kilter":
		return "Kilter"
	case "moon":
		return "Moon"
	case "tension":
		return "Tension"
	case "spray":
		return "Spray Wall"
	case "custom", "board": // "board" is legacy
		return "Board"
	case "commercial": // legacy gym — no chip needed
		return ""
	}
	return ""
}

func isBoardSubtype(s string) bool {
	switch s {
	case "kilter", "moon", "tension", "spray", "custom", "board":
		return true
	}
	return false
}

// parseTickDuration reads the optional time-on-the-wall inputs (minutes + seconds)
// and returns the total in seconds, or 0 when left blank. Capped at six hours so a
// stray keystroke can't record a nonsense lap.
func parseTickDuration(r *http.Request) int {
	mins, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("duration_minutes")))
	secs, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("duration_seconds")))
	if mins < 0 {
		mins = 0
	}
	if secs < 0 {
		secs = 0
	}
	total := mins*60 + secs
	if total > 6*60*60 {
		total = 6 * 60 * 60
	}
	return total
}

// formatTickDuration renders a lap time as m:ss (h:mm:ss for very long ones), or ""
// when no duration was recorded.
func formatTickDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	h, m, s := seconds/3600, (seconds%3600)/60, seconds%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// isUngradedClimb reports whether a grade label is one of the ungraded drill
// options (Rainbow/Traverse), for which no send/outcome applies.
func isUngradedClimb(grade string) bool {
	return grade == "Rainbow" || grade == "Traverse"
}

// styleImpliesSent reports whether an ascent-style key means the climb was sent.
// A send is a clean ascent: onsight (first go, no beta), flash (first go, with
// beta), or redpoint (later go, no falls). Hangdog (fell/hung) and working (not
// topped) are not sends.
func styleImpliesSent(style string) bool {
	switch style {
	case "onsight", "flash", "redpoint":
		return true
	}
	return false
}

// resolveResult takes the posted ascent-style and grade and returns the stored
// (sent, style). Ungraded drill climbs (Rainbow/Traverse) never count as a send
// and carry no style.
func resolveResult(style, grade string) (sentOut bool, styleOut string) {
	if isUngradedClimb(grade) {
		return false, ""
	}
	return styleImpliesSent(style), style
}

func ticksToViews(ticks []db.ClimbingTick) []pages.ClimbingTickView {
	views := make([]pages.ClimbingTickView, 0, len(ticks))
	for _, t := range ticks {
		// Normalise kind: "route" (legacy) → "sport".
		kindRaw := t.Kind
		if kindRaw == "route" {
			kindRaw = "sport"
		}
		kindDisplay := map[string]string{
			"boulder": "Boulder",
			"sport":   "Sport",
			"trad":    "Trad",
		}[kindRaw]
		if kindDisplay == "" {
			kindDisplay = kindRaw
		}

		settingDisplay := map[string]string{
			"indoor":  "Indoor",
			"outdoor": "Outdoor",
		}[t.Setting]

		styleRaw := t.Style
		ropeStyleRaw := t.RopeStyle
		// Backwards compat: old ticks with Style="top_rope" become rope style.
		if t.Style == "top_rope" {
			styleRaw = ""
			ropeStyleRaw = "top_rope"
		}

		style, styleClass, styleIcon := tickStyleDisplay(styleRaw)
		ropeStyle, ropeStyleClass, ropeStyleIcon := tickRopeStyleDisplay(ropeStyleRaw)

		views = append(views, pages.ClimbingTickView{
			ID:                  t.ID,
			RunID:               t.RunID,
			ExerciseID:          t.ExerciseID,
			Kind:                kindDisplay,
			KindRaw:             kindRaw,
			Setting:             settingDisplay,
			SettingRaw:          t.Setting,
			Subtype:             tickSubtypeDisplay(t.Subtype),
			SubtypeRaw:          t.Subtype,
			IsBoard:             isBoardSubtype(t.Subtype),
			Grade:               t.Grade,
			Focus:               t.Focus,
			Thoughts:            t.Thoughts,
			Style:               style,
			StyleClass:          styleClass,
			StyleRaw:            styleRaw,
			StyleIcon:           styleIcon,
			RopeStyle:           ropeStyle,
			RopeStyleClass:      ropeStyleClass,
			RopeStyleRaw:        ropeStyleRaw,
			RopeStyleIcon:       ropeStyleIcon,
			Attempts:            t.Attempts,
			DurationSeconds:     t.DurationSeconds,
			DurationLabel:       formatTickDuration(t.DurationSeconds),
			DurationMinutesPart: t.DurationSeconds / 60,
			DurationSecondsPart: t.DurationSeconds % 60,
			Sent:                t.Sent,
			Stars:               t.Stars,
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
	s.serveExerciseTicksSeeded(w, r, ownerID, runID, exerciseID, 0)
}

// serveExerciseTicksSeeded renders the tick list. The new-tick form is seeded
// from a specific tick (seedTickID > 0, used by "Log again") or, failing that,
// from the latest tick in the run (inheritance), or user defaults.
func (s *Server) serveExerciseTicksSeeded(w http.ResponseWriter, r *http.Request, ownerID, runID, exerciseID, seedTickID uint) {
	ticks, err := db.ListClimbingTicksByExercise(s.store.DB, ownerID, runID, exerciseID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	var user db.User
	if err := s.store.DB.Where("id = ?", ownerID).First(&user).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	bgs := user.BoulderGradeSystem
	if bgs == "" {
		bgs = "font"
	}
	rgs := user.RouteGradeSystem
	if rgs == "" {
		rgs = "french"
	}

	// Seed the new-tick form: explicit tick (Log again) > latest in run > defaults.
	seed := db.ClimbingTick{Kind: "boulder"}
	if seedTickID > 0 {
		if t, err := db.GetClimbingTick(s.store.DB, ownerID, runID, seedTickID); err == nil {
			seed = t
		}
	} else if t, ok := db.GetLatestClimbingTickInRun(s.store.DB, ownerID, runID); ok {
		seed = t
	}

	header, err := db.GetClimbingSessionHeader(s.store.DB, ownerID, runID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.pages.RenderExerciseTicks(w, pages.ExerciseTicksParams{
		RunID:              runID,
		ExerciseID:         exerciseID,
		Ticks:              ticksToViews(ticks),
		BoulderGradeSystem: bgs,
		RouteGradeSystem:   rgs,
		SeedKind:           seed.Kind,
		SeedSetting:        seed.Setting,
		SeedSubtype:        seed.Subtype,
		SeedIsBoard:        isBoardSubtype(seed.Subtype),
		SeedRopeStyle:      seed.RopeStyle,
		SeedGrade:          seed.Grade,
		HeaderClimbs:       header.TotalClimbs,
		HeaderSends:        header.TotalSends,
		HeaderHardest:      header.HardestGrade,
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
	if kind != "boulder" && kind != "sport" && kind != "trad" {
		kind = "boulder"
	}

	stars := parseInt("stars", 0)
	if stars < 0 || stars > 3 {
		stars = 0
	}

	setting := strings.TrimSpace(r.FormValue("setting"))
	if setting != "indoor" && setting != "outdoor" {
		setting = ""
	}
	subtype := ""
	ropeStyle := ""
	if kind == "boulder" && setting == "indoor" {
		if r.FormValue("is_board") == "1" {
			subtype = strings.TrimSpace(r.FormValue("board_kind"))
		}
	} else if kind == "sport" || kind == "trad" {
		ropeStyle = strings.TrimSpace(r.FormValue("rope_style"))
	}

	grade := strings.TrimSpace(r.FormValue("grade"))
	sent, style := resolveResult(strings.TrimSpace(r.FormValue("style")), grade)
	attempts := parseInt("attempts", 1)
	if attempts < 1 {
		attempts = 1
	}

	tick := &db.ClimbingTick{
		OwnerID:         ownerID,
		RunID:           runID,
		ExerciseID:      exerciseID,
		Kind:            kind,
		Setting:         setting,
		Subtype:         subtype,
		Grade:           grade,
		Focus:           strings.TrimSpace(r.FormValue("focus")),
		Thoughts:        strings.TrimSpace(r.FormValue("thoughts")),
		Style:           style,
		RopeStyle:       ropeStyle,
		Attempts:        attempts,
		Sent:            sent,
		DurationSeconds: parseTickDuration(r),
		Stars:           stars,
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
	if kind != "boulder" && kind != "sport" && kind != "trad" {
		kind = "boulder"
	}
	stars := parseInt("stars", 0)
	if stars < 0 || stars > 3 {
		stars = 0
	}

	setting := strings.TrimSpace(r.FormValue("setting"))
	if setting != "indoor" && setting != "outdoor" {
		setting = ""
	}
	subtype := ""
	ropeStyle := ""
	if kind == "boulder" && setting == "indoor" {
		if r.FormValue("is_board") == "1" {
			subtype = strings.TrimSpace(r.FormValue("board_kind"))
		}
	} else if kind == "sport" || kind == "trad" {
		ropeStyle = strings.TrimSpace(r.FormValue("rope_style"))
	}

	grade := strings.TrimSpace(r.FormValue("grade"))
	sent, style := resolveResult(strings.TrimSpace(r.FormValue("style")), grade)
	attempts := parseInt("attempts", 1)
	if attempts < 1 {
		attempts = 1
	}

	if err := db.UpdateClimbingTick(s.store.DB, ownerID, tickID,
		kind, setting, subtype,
		grade,
		strings.TrimSpace(r.FormValue("focus")),
		strings.TrimSpace(r.FormValue("thoughts")),
		style,
		ropeStyle,
		attempts, stars, parseTickDuration(r),
		sent,
	); err != nil {
		s.serverError(w, r, err)
		return
	}

	s.serveExerciseTicks(w, r, ownerID, runID, exerciseID)
}

// handleExerciseTickLogAgain serves POST /runs/{runID}/exercises/{exerciseID}/ticks/{tickID}/again.
// Re-renders the tick list with the single new-tick form pre-seeded from the
// given tick's constants + grade (outcome and notes blank), so a repeat needs
// no re-entry. No new tick is created here.
func (s *Server) handleExerciseTickLogAgain(w http.ResponseWriter, r *http.Request) {
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

	s.serveExerciseTicksSeeded(w, r, ownerID, runID, exerciseID, uint(tickID))
}

// ---------------------------------------------------------------------------
// Profile: venue and board management
// ---------------------------------------------------------------------------

func venueToView(v db.ClimbingVenue) pages.ClimbingVenueView {
	kind := "Commercial"
	if v.Kind == "outdoor" {
		kind = "Outdoor"
	}
	return pages.ClimbingVenueView{ID: v.ID, Name: v.Name, Kind: kind, Location: v.Location}
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
		location := strings.TrimSpace(r.FormValue("location"))
		if err := db.CreateClimbingVenue(s.store.DB, &db.ClimbingVenue{
			OwnerID: ownerID, Name: name, Kind: kind, Location: location,
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

// handleProfileVenueUpdate serves POST /profile/venues/{venueID}/update.
func (s *Server) handleProfileVenueUpdate(w http.ResponseWriter, r *http.Request) {
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind != "commercial" && kind != "outdoor" {
		kind = "commercial"
	}
	location := strings.TrimSpace(r.FormValue("location"))
	if err := db.UpdateClimbingVenue(s.store.DB, ownerID, uint(id), name, kind, location); err != nil {
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
