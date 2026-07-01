package db

import (
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
)

// ErrNotFound is returned by query helpers when a record is not found.
// Callers can distinguish it from real DB failures with errors.Is(err, ErrNotFound).
var ErrNotFound = errors.New("not found")

// isNotFound reports whether err is a GORM record-not-found error.
func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// LocalDate truncates t to midnight in its own location (strips time-of-day).
func LocalDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// GetScheduledSessionWithTemplate loads a ScheduledSession with its full template graph
// (Activities ordered by order_index → Exercises ordered by order_index → Media)
// for the given owner.
func GetScheduledSessionWithTemplate(gdb *gorm.DB, ownerID, ssID uint) (ScheduledSession, error) {
	var ss ScheduledSession
	err := gdb.
		Preload("SessionTemplate", func(tx *gorm.DB) *gorm.DB {
			return tx.
				Preload("Activities", func(tx2 *gorm.DB) *gorm.DB {
					return tx2.Where("owner_id = ?", ownerID).Order("order_index asc")
				}).
				Preload("Activities.Exercises", func(tx2 *gorm.DB) *gorm.DB {
					return tx2.Where("owner_id = ?", ownerID).Order("order_index asc")
				}).
				Preload("Activities.Exercises.Media")
		}).
		Where("owner_id = ? AND id = ?", ownerID, ssID).
		First(&ss).Error
	if isNotFound(err) {
		return ss, ErrNotFound
	}
	return ss, err
}

// GetTemplateWithGraph loads a SessionTemplate with all Activities (ordered), Exercises
// (ordered), and Media for the given owner.
func GetTemplateWithGraph(gdb *gorm.DB, ownerID, templateID uint) (*SessionTemplate, error) {
	var tpl SessionTemplate
	err := gdb.
		Preload("Activities", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Activities.Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Activities.Exercises.Media").
		Where("id = ? AND owner_id = ?", templateID, ownerID).
		First(&tpl).Error
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tpl, nil
}

// ListTemplates returns all non-system session templates for a user, ordered by id descending.
// If labelFilter is non-empty, only templates with that label are returned.
func ListTemplates(gdb *gorm.DB, ownerID uint, labelFilter string) ([]SessionTemplate, error) {
	var templates []SessionTemplate
	q := gdb.Preload("Activities").Where("owner_id = ? AND is_system = ?", ownerID, false)
	if labelFilter != "" {
		q = q.Where("label = ?", labelFilter)
	}
	err := q.Order("id desc").Find(&templates).Error
	return templates, err
}

// DistinctTemplateLabels returns the sorted distinct non-empty labels across all session templates for a user.
func DistinctTemplateLabels(gdb *gorm.DB, ownerID uint) ([]string, error) {
	var labels []string
	err := gdb.Model(&SessionTemplate{}).
		Where("owner_id = ? AND is_system = ? AND label != ''", ownerID, false).
		Distinct("label").
		Order("label asc").
		Pluck("label", &labels).Error
	return labels, err
}

// ListLibraryExercises returns all root (no parent) library exercises for a user,
// ordered by name ascending.
func ListLibraryExercises(gdb *gorm.DB, ownerID uint) ([]LibraryExercise, error) {
	var rows []LibraryExercise
	err := gdb.
		Where("owner_id = ? AND parent_library_exercise_id IS NULL", ownerID).
		Order("name asc").
		Find(&rows).Error
	return rows, err
}

// ListActivityTemplates returns all activity templates for a user, ordered by name ascending.
// If labelFilter is non-empty, only templates with that label are returned.
func ListActivityTemplates(gdb *gorm.DB, ownerID uint, labelFilter string) ([]ActivityTemplate, error) {
	var rows []ActivityTemplate
	q := gdb.Where("owner_id = ?", ownerID)
	if labelFilter != "" {
		q = q.Where("label = ?", labelFilter)
	}
	err := q.Order("name asc").Find(&rows).Error
	return rows, err
}

// DistinctActivityTemplateLabels returns the sorted distinct non-empty labels across all activity templates for a user.
func DistinctActivityTemplateLabels(gdb *gorm.DB, ownerID uint) ([]string, error) {
	var labels []string
	err := gdb.Model(&ActivityTemplate{}).
		Where("owner_id = ? AND label != ''", ownerID).
		Distinct("label").
		Order("label asc").
		Pluck("label", &labels).Error
	return labels, err
}

// GetActivityTemplateWithExercises loads an ActivityTemplate with its root exercises and their
// catalog children, all ordered by order_index.
func GetActivityTemplateWithExercises(gdb *gorm.DB, ownerID, templateID uint) (*ActivityTemplate, error) {
	var tpl ActivityTemplate
	err := gdb.
		Preload("Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Exercises.Media").
		Where("id = ? AND owner_id = ?", templateID, ownerID).
		First(&tpl).Error
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tpl, nil
}

// ListActivityTemplatesWithExercises returns all activity templates for a user with their
// exercises preloaded, ordered by name ascending.
func ListActivityTemplatesWithExercises(gdb *gorm.DB, ownerID uint) ([]ActivityTemplate, error) {
	var rows []ActivityTemplate
	err := gdb.
		Preload("Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Where("owner_id = ?", ownerID).
		Order("name asc").
		Find(&rows).Error
	return rows, err
}

// ListScheduledSessionsInRange returns non-trial scheduled sessions (with their templates)
// for a user within the given date range, ordered by scheduled_date ascending.
func ListScheduledSessionsInRange(gdb *gorm.DB, ownerID uint, start, end time.Time) ([]ScheduledSession, error) {
	var rows []ScheduledSession
	err := gdb.
		Preload("SessionTemplate").
		Where("owner_id = ? AND is_trial = ? AND scheduled_date >= ? AND scheduled_date <= ?",
			ownerID, false, start, end).
		Order("scheduled_date asc").
		Find(&rows).Error
	return rows, err
}

// CompletedRunDate groups completed-run counts by date and trial status.
type CompletedRunDate struct {
	ScheduledDate      time.Time
	IsTrial            bool
	ScheduledSessionID uint
}

// ListCompletedRunDatesInRange returns one row per completed SessionRun whose
// associated ScheduledSession.ScheduledDate falls within [start, end].
func ListCompletedRunDatesInRange(gdb *gorm.DB, ownerID uint, start, end time.Time) ([]CompletedRunDate, error) {
	var rows []CompletedRunDate
	err := gdb.
		Table("session_runs").
		Select("scheduled_sessions.scheduled_date, session_runs.is_trial, session_runs.scheduled_session_id").
		Joins("JOIN scheduled_sessions ON scheduled_sessions.id = session_runs.scheduled_session_id").
		Where("session_runs.owner_id = ? AND session_runs.status = ? AND session_runs.deleted_at IS NULL AND scheduled_sessions.scheduled_date >= ? AND scheduled_sessions.scheduled_date <= ?",
			ownerID, RunStatusCompleted, start, end).
		Scan(&rows).Error
	return rows, err
}

// GetSessionJournalByRunID returns the journal for a run, or nil if none exists.
func GetSessionJournalByRunID(gdb *gorm.DB, ownerID, runID uint) (*SessionJournal, error) {
	var j SessionJournal
	err := gdb.Where("owner_id = ? AND run_id = ?", ownerID, runID).First(&j).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &j, err
}

// UpsertSessionJournal creates or updates the journal for a run.
// Pass j.ID = 0 to create, non-zero to update.
func UpsertSessionJournal(gdb *gorm.DB, j *SessionJournal) error {
	return gdb.Save(j).Error
}

// ListSessionJournals returns all journals for a user, newest first.
func ListSessionJournals(gdb *gorm.DB, ownerID uint) ([]SessionJournal, error) {
	var journals []SessionJournal
	err := gdb.Where("owner_id = ?", ownerID).Order("id desc").Find(&journals).Error
	return journals, err
}

// ListJournalIDsByRun returns a map of runID → journal ID for all runs in runIDs that have a journal.
func ListJournalIDsByRun(gdb *gorm.DB, ownerID uint, runIDs []uint) (map[uint]uint, error) {
	if len(runIDs) == 0 {
		return map[uint]uint{}, nil
	}
	type row struct {
		RunID uint
		ID    uint
	}
	var rows []row
	err := gdb.Model(&SessionJournal{}).
		Select("run_id, id").
		Where("owner_id = ? AND run_id IN ?", ownerID, runIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[uint]uint, len(rows))
	for _, r := range rows {
		m[r.RunID] = r.ID
	}
	return m, nil
}

// GetSessionJournalByID returns a single journal by primary key, scoped to the owner.
func GetSessionJournalByID(gdb *gorm.DB, ownerID, id uint) (*SessionJournal, error) {
	var j SessionJournal
	err := gdb.Where("owner_id = ? AND id = ?", ownerID, id).First(&j).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &j, err
}

// DeleteSessionJournal hard-deletes a journal entry owned by the given user.
func DeleteSessionJournal(gdb *gorm.DB, ownerID, id uint) error {
	return gdb.Where("owner_id = ? AND id = ?", ownerID, id).Delete(&SessionJournal{}).Error
}

// ListCycleExerciseWeekOverrides returns all week overrides for a cycle, ordered by week asc.
func ListCycleExerciseWeekOverrides(gdb *gorm.DB, ownerID, cycleID uint) ([]CycleExerciseWeekOverride, error) {
	var rows []CycleExerciseWeekOverride
	err := gdb.
		Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
		Order("week asc").
		Find(&rows).Error
	return rows, err
}

// UpsertCycleExerciseWeekOverride creates or updates a week override.
func UpsertCycleExerciseWeekOverride(gdb *gorm.DB, o *CycleExerciseWeekOverride) error {
	var existing CycleExerciseWeekOverride
	q := gdb.Where("owner_id = ? AND training_cycle_id = ? AND week = ?", o.OwnerID, o.TrainingCycleID, o.Week)
	if o.LibraryExerciseID != nil && *o.LibraryExerciseID != 0 {
		q = q.Where("library_exercise_id = ?", *o.LibraryExerciseID)
	} else {
		q = q.Where("library_exercise_id IS NULL AND exercise_name = ?", o.ExerciseName)
	}
	if err := q.First(&existing).Error; err != nil {
		return gdb.Create(o).Error
	}
	existing.Sets = o.Sets
	existing.Reps = o.Reps
	existing.WeightKg = o.WeightKg
	existing.RepSeconds = o.RepSeconds
	return gdb.Save(&existing).Error
}

// DeleteCycleExerciseWeekOverridesForExercise deletes all week overrides for one exercise in a cycle.
func DeleteCycleExerciseWeekOverridesForExercise(gdb *gorm.DB, ownerID, cycleID uint, libID *uint, exerciseName string) error {
	q := gdb.Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID)
	if libID != nil && *libID != 0 {
		q = q.Where("library_exercise_id = ?", *libID)
	} else {
		q = q.Where("library_exercise_id IS NULL AND exercise_name = ?", exerciseName)
	}
	return q.Delete(&CycleExerciseWeekOverride{}).Error
}

// ---------------------------------------------------------------------------
// Climbing ticks
// ---------------------------------------------------------------------------

// ListClimbingTicksByExercise returns all ticks for a specific exercise step, ordered by order_index asc.
func ListClimbingTicksByExercise(gdb *gorm.DB, ownerID, runID, exerciseID uint) ([]ClimbingTick, error) {
	var rows []ClimbingTick
	err := gdb.
		Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).
		Order("order_index asc, id asc").
		Find(&rows).Error
	return rows, err
}

// CreateClimbingTick inserts a new tick, assigning the next order_index within the exercise.
func CreateClimbingTick(gdb *gorm.DB, t *ClimbingTick) error {
	var maxIdx int
	if err := gdb.Model(&ClimbingTick{}).
		Where("owner_id = ? AND run_id = ? AND exercise_id = ?", t.OwnerID, t.RunID, t.ExerciseID).
		Select("COALESCE(MAX(order_index), -1)").
		Scan(&maxIdx).Error; err != nil {
		return err
	}
	t.OrderIndex = maxIdx + 1
	return gdb.Create(t).Error
}

// DeleteClimbingTick hard-deletes a tick (validates ownerID).
func DeleteClimbingTick(gdb *gorm.DB, ownerID, id uint) error {
	return gdb.Where("owner_id = ? AND id = ?", ownerID, id).Delete(&ClimbingTick{}).Error
}

// GetClimbingTick fetches a single tick, scoped to owner and run (prevents IDOR
// and cross-run seeding via an attacker-controllable id). Used by "Log again".
func GetClimbingTick(gdb *gorm.DB, ownerID, runID, id uint) (ClimbingTick, error) {
	var t ClimbingTick
	err := gdb.Where("owner_id = ? AND run_id = ? AND id = ?", ownerID, runID, id).First(&t).Error
	return t, err
}

// GetLatestClimbingTickInRun returns the most recently created tick across all
// drills in a run, for inheriting constants into a new-tick form. Ordered by
// created_at then id (NOT order_index, which is assigned per-exercise and
// collides across drills). Returns (zero, false) when the run has no ticks.
func GetLatestClimbingTickInRun(gdb *gorm.DB, ownerID, runID uint) (ClimbingTick, bool) {
	var t ClimbingTick
	err := gdb.
		Where("owner_id = ? AND run_id = ?", ownerID, runID).
		Order("created_at desc, id desc").
		First(&t).Error
	if err != nil {
		return ClimbingTick{}, false
	}
	return t, true
}

// UpdateClimbingTick replaces all editable fields on an existing tick.
func UpdateClimbingTick(gdb *gorm.DB, ownerID, id uint, kind, setting, subtype, grade, focus, thoughts, style, ropeStyle string, attempts, stars int, sent bool) error {
	return gdb.Model(&ClimbingTick{}).
		Where("owner_id = ? AND id = ?", ownerID, id).
		Updates(map[string]interface{}{
			"kind":       kind,
			"setting":    setting,
			"subtype":    subtype,
			"grade":      grade,
			"focus":      focus,
			"thoughts":   thoughts,
			"style":      style,
			"rope_style": ropeStyle,
			"attempts":   attempts,
			"stars":      stars,
			"sent":       sent,
		}).Error
}

// ClimbingTickSummary is a compact summary of ticks for a run, used in the training log.
type ClimbingTickSummary struct {
	TotalBoulders int
	TotalRoutes   int
	TotalSends    int
	MinGrade      string
	MaxGrade      string
}

// GetClimbingTickSummaryForRun builds a summary of all ticks in a run.
func GetClimbingTickSummaryForRun(gdb *gorm.DB, ownerID, runID uint) (ClimbingTickSummary, error) {
	var ticks []ClimbingTick
	if err := gdb.Where("owner_id = ? AND run_id = ?", ownerID, runID).Find(&ticks).Error; err != nil {
		return ClimbingTickSummary{}, err
	}
	var s ClimbingTickSummary
	grades := make([]string, 0)
	for _, t := range ticks {
		if t.Kind == "boulder" {
			s.TotalBoulders++
		} else {
			s.TotalRoutes++
		}
		if t.Sent {
			s.TotalSends++
		}
		if t.Grade != "" {
			grades = append(grades, t.Grade)
		}
	}
	if len(grades) > 0 {
		s.MinGrade = grades[0]
		s.MaxGrade = grades[len(grades)-1]
		for _, g := range grades {
			if g < s.MinGrade {
				s.MinGrade = g
			}
			if g > s.MaxGrade {
				s.MaxGrade = g
			}
		}
	}
	return s, nil
}

// gradeRanks maps every grade across all systems to a comparable ordinal.
// Built from the canonical scales (mirrors GRADE_LISTS in run_ticks.html).
// Ungraded labels (Rainbow/Traverse) are deliberately absent — they don't rank.
var gradeRanks = buildGradeRanks()

// Each list must mirror the corresponding GRADE_LISTS entry in run_ticks.html
// (same order, same ceiling) so a grade the chip strip offers always ranks and
// no out-of-range grade can be stored. font stops at 9a (boulder ceiling);
// french continues to 9c (route ceiling).
func buildGradeRanks() map[string]int {
	lists := [][]string{
		{"3", "3+", "4", "4+", "5", "5+", "6a", "6a+", "6b", "6b+", "6c", "6c+", "7a", "7a+", "7b", "7b+", "7c", "7c+", "8a", "8a+", "8b", "8b+", "8c", "8c+", "9a"},
		{"3", "3+", "4", "4+", "5", "5+", "6a", "6a+", "6b", "6b+", "6c", "6c+", "7a", "7a+", "7b", "7b+", "7c", "7c+", "8a", "8a+", "8b", "8b+", "8c", "8c+", "9a", "9a+", "9b", "9b+", "9c"},
		{"VB", "V0", "V1", "V2", "V3", "V4", "V5", "V6", "V7", "V8", "V9", "V10", "V11", "V12", "V13", "V14", "V15", "V16"},
		{"5.0", "5.1", "5.2", "5.3", "5.4", "5.5", "5.6", "5.7", "5.8", "5.9", "5.10a", "5.10b", "5.10c", "5.10d", "5.11a", "5.11b", "5.11c", "5.11d", "5.12a", "5.12b", "5.12c", "5.12d", "5.13a", "5.13b", "5.13c", "5.13d", "5.14a", "5.14b", "5.14c", "5.14d", "5.15a", "5.15b", "5.15c", "5.15d"},
	}
	ranks := make(map[string]int)
	for _, list := range lists {
		for i, g := range list {
			ranks[g] = i
		}
	}
	return ranks
}

// ClimbingSessionHeader is the live one-liner shown above the tick list:
// total climbs, sends, and the hardest graded climb in the run.
type ClimbingSessionHeader struct {
	TotalClimbs int
	TotalSends  int
	HardestGrade string
}

// GetClimbingSessionHeader aggregates the run's ticks for the in-session header.
// "Hardest" uses gradeRanks (not lexicographic) and ignores ungraded climbs.
func GetClimbingSessionHeader(gdb *gorm.DB, ownerID, runID uint) (ClimbingSessionHeader, error) {
	var ticks []ClimbingTick
	if err := gdb.Where("owner_id = ? AND run_id = ?", ownerID, runID).Find(&ticks).Error; err != nil {
		return ClimbingSessionHeader{}, err
	}
	var h ClimbingSessionHeader
	bestRank := -1
	for _, t := range ticks {
		h.TotalClimbs++
		if t.Sent {
			h.TotalSends++
		}
		if r, ok := gradeRanks[t.Grade]; ok && r > bestRank {
			bestRank = r
			h.HardestGrade = t.Grade
		}
	}
	return h, nil
}

// ---------------------------------------------------------------------------
// Climbing analytics (History page)
// ---------------------------------------------------------------------------

type gradeTally struct{ total, sent int }

// ClimbingGradeTally is one grade row in a discipline's pyramid.
type ClimbingGradeTally struct {
	Grade string
	Total int
	Sent  int
	Rank  int
}

// ClimbingDisciplineStats aggregates one discipline (boulder or route).
type ClimbingDisciplineStats struct {
	Ticks       int
	Sends       int
	SendRate    int // percent
	HardestSent string
	Pyramid     []ClimbingGradeTally // hardest -> easiest, capped at 10
	MoreGrades  int
	MaxTotal    int // busiest grade's tick count, for bar scaling
}

// ClimbingAnalyticsResult aggregates climbing ticks across a set of runs.
type ClimbingAnalyticsResult struct {
	HasData      bool
	TotalClimbs  int
	TotalSends   int
	SessionCount int
	SendRate     int
	Boulder      ClimbingDisciplineStats
	Route        ClimbingDisciplineStats

	HasIndoorOutdoor bool
	IndoorPct        int
	OutdoorPct       int
	HasBoardSplit    bool
	CommercialPct    int
	BoardPct         int
}

func percentOf(n, d int) int {
	if d <= 0 {
		return 0
	}
	return n * 100 / d
}

// ClimbingAnalytics aggregates every climbing tick belonging to the given runs
// into per-discipline grade pyramids, send rates, hardest sends, and setting
// splits. Boulder and route grades are ranked within their own scales via
// gradeRanks; grades absent from the canonical scales (e.g. Rainbow) are
// excluded from pyramids but still counted toward volume and send rate.
func ClimbingAnalytics(gdb *gorm.DB, ownerID uint, runIDs []uint) (ClimbingAnalyticsResult, error) {
	var a ClimbingAnalyticsResult
	if len(runIDs) == 0 {
		return a, nil
	}
	var ticks []ClimbingTick
	if err := gdb.Where("owner_id = ? AND run_id IN ?", ownerID, runIDs).Find(&ticks).Error; err != nil {
		return a, err
	}
	if len(ticks) == 0 {
		return a, nil
	}
	a.HasData = true

	boulderGrades := map[string]*gradeTally{}
	routeGrades := map[string]*gradeTally{}
	sessions := map[uint]bool{}
	var indoor, outdoor, commercial, board int
	bestBoulder, bestRoute := -1, -1

	for _, t := range ticks {
		sessions[t.RunID] = true
		a.TotalClimbs++
		if t.Sent {
			a.TotalSends++
		}
		isBoulder := t.Kind == "boulder"
		disc, grades := &a.Route, routeGrades
		if isBoulder {
			disc, grades = &a.Boulder, boulderGrades
		}
		disc.Ticks++
		if t.Sent {
			disc.Sends++
		}
		if rank, ok := gradeRanks[t.Grade]; ok {
			if grades[t.Grade] == nil {
				grades[t.Grade] = &gradeTally{}
			}
			grades[t.Grade].total++
			if t.Sent {
				grades[t.Grade].sent++
				if isBoulder && rank > bestBoulder {
					bestBoulder, a.Boulder.HardestSent = rank, t.Grade
				} else if !isBoulder && rank > bestRoute {
					bestRoute, a.Route.HardestSent = rank, t.Grade
				}
			}
		}
		switch t.Setting {
		case "indoor":
			indoor++
		case "outdoor":
			outdoor++
		}
		if isBoulder && t.Setting == "indoor" {
			switch t.Subtype {
			case "commercial":
				commercial++
			case "board":
				board++
			}
		}
	}

	a.SessionCount = len(sessions)
	a.SendRate = percentOf(a.TotalSends, a.TotalClimbs)
	a.Boulder.SendRate = percentOf(a.Boulder.Sends, a.Boulder.Ticks)
	a.Route.SendRate = percentOf(a.Route.Sends, a.Route.Ticks)
	a.Boulder.Pyramid, a.Boulder.MoreGrades, a.Boulder.MaxTotal = buildClimbingPyramid(boulderGrades)
	a.Route.Pyramid, a.Route.MoreGrades, a.Route.MaxTotal = buildClimbingPyramid(routeGrades)

	if indoor+outdoor > 0 {
		a.HasIndoorOutdoor = true
		a.IndoorPct = percentOf(indoor, a.TotalClimbs)
		a.OutdoorPct = percentOf(outdoor, a.TotalClimbs)
	}
	if commercial+board > 0 {
		a.HasBoardSplit = true
		a.CommercialPct = percentOf(commercial, a.TotalClimbs)
		a.BoardPct = percentOf(board, a.TotalClimbs)
	}
	return a, nil
}

// buildClimbingPyramid sorts grade tallies hardest-first and caps at 10 rows.
func buildClimbingPyramid(m map[string]*gradeTally) ([]ClimbingGradeTally, int, int) {
	rows := make([]ClimbingGradeTally, 0, len(m))
	for g, t := range m {
		rows = append(rows, ClimbingGradeTally{Grade: g, Total: t.total, Sent: t.sent, Rank: gradeRanks[g]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Rank > rows[j].Rank })
	more := 0
	if len(rows) > 10 {
		more = len(rows) - 10
		rows = rows[:10]
	}
	maxTotal := 0
	for _, r := range rows {
		if r.Total > maxTotal {
			maxTotal = r.Total
		}
	}
	return rows, more, maxTotal
}

// UserHasClimbingTicks reports whether the user has ever logged any climbing
// tick, used to distinguish "no climbs in this range" from "never climbed".
func UserHasClimbingTicks(gdb *gorm.DB, ownerID uint) (bool, error) {
	var tick ClimbingTick
	err := gdb.Select("id").Where("owner_id = ?", ownerID).First(&tick).Error
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

// ---------------------------------------------------------------------------
// Climbing venues and boards
// ---------------------------------------------------------------------------

// ListClimbingVenues returns all venues for a user, ordered by name.
func ListClimbingVenues(gdb *gorm.DB, ownerID uint) ([]ClimbingVenue, error) {
	var rows []ClimbingVenue
	err := gdb.Where("owner_id = ?", ownerID).Order("name asc").Find(&rows).Error
	return rows, err
}

// CreateClimbingVenue inserts a new venue.
func CreateClimbingVenue(gdb *gorm.DB, v *ClimbingVenue) error {
	return gdb.Create(v).Error
}

// UpdateClimbingVenue updates a venue's name, kind, and location.
func UpdateClimbingVenue(gdb *gorm.DB, ownerID, id uint, name, kind, location string) error {
	return gdb.Model(&ClimbingVenue{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Updates(map[string]any{"name": name, "kind": kind, "location": location}).Error
}

// DeleteClimbingVenue hard-deletes a venue and nulls SessionJournal.VenueID for affected entries.
func DeleteClimbingVenue(gdb *gorm.DB, ownerID, id uint) error {
	if err := gdb.Model(&SessionJournal{}).
		Where("owner_id = ? AND venue_id = ?", ownerID, id).
		Updates(map[string]interface{}{"venue_id": nil, "board_id": nil}).Error; err != nil {
		return err
	}
	return gdb.Where("owner_id = ? AND id = ?", ownerID, id).Delete(&ClimbingVenue{}).Error
}

// ListClimbingBoards returns all standalone boards for a user, ordered by name.
func ListClimbingBoards(gdb *gorm.DB, ownerID uint) ([]ClimbingBoard, error) {
	var rows []ClimbingBoard
	err := gdb.Where("owner_id = ?", ownerID).Order("name asc, board_type asc").Find(&rows).Error
	return rows, err
}

// CreateClimbingBoard inserts a standalone board.
func CreateClimbingBoard(gdb *gorm.DB, b *ClimbingBoard) error {
	return gdb.Create(b).Error
}

// DeleteClimbingBoard hard-deletes a board and nulls SessionJournal.BoardID for affected entries.
func DeleteClimbingBoard(gdb *gorm.DB, ownerID, id uint) error {
	if err := gdb.Model(&SessionJournal{}).
		Where("owner_id = ? AND board_id = ?", ownerID, id).
		Update("board_id", nil).Error; err != nil {
		return err
	}
	return gdb.Where("owner_id = ? AND id = ?", ownerID, id).Delete(&ClimbingBoard{}).Error
}

// ---------------------------------------------------------------------------
// Draft runs (for manual log entries)
// ---------------------------------------------------------------------------

// CreateDraftSessionRun creates a draft manual run using the given scheduled session anchor.
// The caller must create the ScheduledSession anchor first (same pattern as open sessions).
func CreateDraftSessionRun(gdb *gorm.DB, ownerID, scheduledSessionID uint) (*SessionRun, error) {
	run := &SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: scheduledSessionID,
		IsTrial:            true,
		IsManual:           true,
		IsDraft:            true,
		Status:             RunStatusRunning,
		StartedAt:          time.Now(),
	}
	return run, gdb.Create(run).Error
}

// FinaliseDraftRun promotes a draft run to a completed manual entry.
func FinaliseDraftRun(gdb *gorm.DB, ownerID, runID uint, customName string, date time.Time) error {
	return gdb.Model(&SessionRun{}).
		Where("owner_id = ? AND id = ? AND is_draft = ?", ownerID, runID, true).
		Updates(map[string]interface{}{
			"is_draft":     false,
			"status":       RunStatusCompleted,
			"custom_name":  customName,
			"started_at":   date,
			"completed_at": date,
		}).Error
}

// DeleteDraftRun hard-deletes a draft run and all its exercises, completions, and ticks.
func DeleteDraftRun(gdb *gorm.DB, ownerID, runID uint) error {
	return gdb.Transaction(func(tx *gorm.DB) error {
		var exerciseIDs []uint
		if err := tx.Model(&Exercise{}).
			Where("owner_id = ? AND session_run_id = ?", ownerID, runID).
			Pluck("id", &exerciseIDs).Error; err != nil {
			return err
		}
		if len(exerciseIDs) > 0 {
			if err := tx.Where("owner_id = ? AND exercise_id IN ?", ownerID, exerciseIDs).Delete(&ClimbingTick{}).Error; err != nil {
				return err
			}
			if err := tx.Where("owner_id = ? AND exercise_id IN ?", ownerID, exerciseIDs).Delete(&RunExerciseCompletion{}).Error; err != nil {
				return err
			}
			if err := tx.Where("owner_id = ? AND id IN ?", ownerID, exerciseIDs).Delete(&Exercise{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("owner_id = ? AND run_id = ?", ownerID, runID).Delete(&ClimbingTick{}).Error; err != nil {
			return err
		}
		return tx.Where("owner_id = ? AND id = ? AND is_draft = ?", ownerID, runID, true).
			Delete(&SessionRun{}).Error
	})
}

// AddManualExercise creates an Exercise attached directly to a SessionRun (no ActivityID).
func AddManualExercise(gdb *gorm.DB, ownerID, runID uint, name string, libraryExerciseID *uint, kind string) (*Exercise, error) {
	var orderIndex int
	if err := gdb.Model(&Exercise{}).
		Where("owner_id = ? AND session_run_id = ?", ownerID, runID).
		Select("COALESCE(MAX(order_index), -1)").
		Scan(&orderIndex).Error; err != nil {
		return nil, err
	}
	ex := &Exercise{
		OwnerID:           ownerID,
		SessionRunID:      &runID,
		LibraryExerciseID: libraryExerciseID,
		Name:              name,
		Kind:              kind,
		OrderIndex:        orderIndex + 1,
	}
	return ex, gdb.Create(ex).Error
}

// DeleteManualExercise removes an exercise from a run, along with ticks, completions, and set logs.
func DeleteManualExercise(gdb *gorm.DB, ownerID, runID, exerciseID uint) error {
	if err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).Delete(&ClimbingTick{}).Error; err != nil {
		return err
	}
	if err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).Delete(&RunExerciseCompletion{}).Error; err != nil {
		return err
	}
	if err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).Delete(&ManualExerciseSetLog{}).Error; err != nil {
		return err
	}
	return gdb.Where("owner_id = ? AND session_run_id = ? AND id = ?", ownerID, runID, exerciseID).
		Delete(&Exercise{}).Error
}

// ListExercisesForRun returns exercises attached directly to a session run, ordered by order_index.
func ListExercisesForRun(gdb *gorm.DB, ownerID, runID uint) ([]Exercise, error) {
	var rows []Exercise
	err := gdb.
		Where("owner_id = ? AND session_run_id = ?", ownerID, runID).
		Order("order_index asc").
		Find(&rows).Error
	return rows, err
}

// ListExerciseCountsByRun returns a map of runID → exercise count for the given run IDs.
// Replaces per-run ListExercisesForRun calls in list views.
func ListExerciseCountsByRun(gdb *gorm.DB, ownerID uint, runIDs []uint) (map[uint]int, error) {
	if len(runIDs) == 0 {
		return map[uint]int{}, nil
	}
	type row struct {
		SessionRunID uint
		Count        int
	}
	var rows []row
	err := gdb.Model(&Exercise{}).
		Select("session_run_id, COUNT(*) as count").
		Where("owner_id = ? AND session_run_id IN ?", ownerID, runIDs).
		Group("session_run_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[uint]int, len(rows))
	for _, r := range rows {
		m[r.SessionRunID] = r.Count
	}
	return m, nil
}

// ListTickSummariesByRun returns a map of runID → ClimbingTickSummary for the given run IDs.
// Replaces per-run GetClimbingTickSummaryForRun calls in list views.
func ListTickSummariesByRun(gdb *gorm.DB, ownerID uint, runIDs []uint) (map[uint]ClimbingTickSummary, error) {
	if len(runIDs) == 0 {
		return map[uint]ClimbingTickSummary{}, nil
	}
	var ticks []ClimbingTick
	if err := gdb.Where("owner_id = ? AND run_id IN ?", ownerID, runIDs).Find(&ticks).Error; err != nil {
		return nil, err
	}
	m := make(map[uint]ClimbingTickSummary)
	for _, t := range ticks {
		s := m[t.RunID]
		if t.Kind == "boulder" {
			s.TotalBoulders++
		} else {
			s.TotalRoutes++
		}
		if t.Sent {
			s.TotalSends++
		}
		if t.Grade != "" {
			if s.MinGrade == "" || t.Grade < s.MinGrade {
				s.MinGrade = t.Grade
			}
			if t.Grade > s.MaxGrade {
				s.MaxGrade = t.Grade
			}
		}
		m[t.RunID] = s
	}
	return m, nil
}

// UpsertManualExerciseCompletion creates or updates the completion record for a manual exercise.
func UpsertManualExerciseCompletion(gdb *gorm.DB, ownerID, runID, exerciseID uint, sets, reps int, weightKg float64, notes string, elapsedSeconds int) error {
	var existing RunExerciseCompletion
	err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return gdb.Create(&RunExerciseCompletion{
			OwnerID:        ownerID,
			RunID:          runID,
			ExerciseID:     exerciseID,
			Status:         RunStatusCompleted,
			CompletedAt:    time.Now(),
			ActualSets:     sets,
			ActualReps:     reps,
			ActualWeightKg: weightKg,
			RunNotes:       notes,
			ElapsedSeconds: elapsedSeconds,
		}).Error
	}
	if err != nil {
		return err
	}
	existing.ActualSets = sets
	existing.ActualReps = reps
	existing.ActualWeightKg = weightKg
	existing.RunNotes = notes
	existing.ElapsedSeconds = elapsedSeconds
	return gdb.Save(&existing).Error
}

// UpsertClimbingExerciseMeta creates or updates session-level climbing metadata for an exercise.
func UpsertClimbingExerciseMeta(gdb *gorm.DB, ownerID, runID, exerciseID uint, climbType, boardKind string, boardID *uint) error {
	var existing ClimbingExerciseMeta
	err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return gdb.Create(&ClimbingExerciseMeta{
			OwnerID:    ownerID,
			RunID:      runID,
			ExerciseID: exerciseID,
			Type:       climbType,
			BoardKind:  boardKind,
			BoardID:    boardID,
		}).Error
	}
	if err != nil {
		return err
	}
	existing.Type = climbType
	existing.BoardKind = boardKind
	existing.BoardID = boardID
	return gdb.Save(&existing).Error
}

// GetClimbingExerciseMeta returns the climbing meta for an exercise in a run, or nil if not set.
func GetClimbingExerciseMeta(gdb *gorm.DB, ownerID, runID, exerciseID uint) (*ClimbingExerciseMeta, error) {
	var m ClimbingExerciseMeta
	err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &m, err
}

// UpsertManualExerciseSetLog creates or updates a per-set log entry.
func UpsertManualExerciseSetLog(gdb *gorm.DB, ownerID, runID, exerciseID uint, setIndex, reps int, weightKg float64) error {
	var existing ManualExerciseSetLog
	err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ? AND set_index = ?", ownerID, runID, exerciseID, setIndex).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return gdb.Create(&ManualExerciseSetLog{
			OwnerID:    ownerID,
			RunID:      runID,
			ExerciseID: exerciseID,
			SetIndex:   setIndex,
			Reps:       reps,
			WeightKg:   weightKg,
		}).Error
	}
	if err != nil {
		return err
	}
	existing.Reps = reps
	existing.WeightKg = weightKg
	return gdb.Save(&existing).Error
}

// ListManualExerciseSetLogs returns all set logs for an exercise ordered by set_index.
func ListManualExerciseSetLogs(gdb *gorm.DB, ownerID, runID, exerciseID uint) ([]ManualExerciseSetLog, error) {
	var logs []ManualExerciseSetLog
	err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).
		Order("set_index asc").Find(&logs).Error
	return logs, err
}

// DeleteManualExerciseSetLog removes a single set log entry.
func DeleteManualExerciseSetLog(gdb *gorm.DB, ownerID, runID, exerciseID uint, setIndex int) error {
	return gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ? AND set_index = ?", ownerID, runID, exerciseID, setIndex).
		Delete(&ManualExerciseSetLog{}).Error
}

// DeleteAllManualExerciseSetLogs removes all set logs for an exercise.
func DeleteAllManualExerciseSetLogs(gdb *gorm.DB, ownerID, runID, exerciseID uint) error {
	return gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).
		Delete(&ManualExerciseSetLog{}).Error
}

// ListExercisePlannedSets returns all planned sets for an exercise ordered by set_index.
func ListExercisePlannedSets(gdb *gorm.DB, exerciseID uint) ([]ExercisePlannedSet, error) {
	var rows []ExercisePlannedSet
	err := gdb.Where("exercise_id = ?", exerciseID).Order("set_index asc").Find(&rows).Error
	return rows, err
}

// UpsertExercisePlannedSet creates or updates a planned set for an exercise.
func UpsertExercisePlannedSet(gdb *gorm.DB, ownerID, exerciseID uint, setIndex, reps int, weightKg float64) error {
	var existing ExercisePlannedSet
	err := gdb.Where("exercise_id = ? AND set_index = ?", exerciseID, setIndex).First(&existing).Error
	if err != nil {
		return gdb.Create(&ExercisePlannedSet{
			OwnerID:    ownerID,
			ExerciseID: exerciseID,
			SetIndex:   setIndex,
			Reps:       reps,
			WeightKg:   weightKg,
		}).Error
	}
	existing.Reps = reps
	existing.WeightKg = weightKg
	return gdb.Save(&existing).Error
}

// DeleteExercisePlannedSet removes a single planned set entry.
func DeleteExercisePlannedSet(gdb *gorm.DB, ownerID, exerciseID uint, setIndex int) error {
	return gdb.Where("owner_id = ? AND exercise_id = ? AND set_index = ?", ownerID, exerciseID, setIndex).
		Delete(&ExercisePlannedSet{}).Error
}

// DeleteAllExercisePlannedSets removes all planned sets for an exercise.
func DeleteAllExercisePlannedSets(gdb *gorm.DB, ownerID, exerciseID uint) error {
	return gdb.Where("owner_id = ? AND exercise_id = ?", ownerID, exerciseID).
		Delete(&ExercisePlannedSet{}).Error
}

// SyncExerciseSetsCount updates Exercise.Sets to match the number of planned sets.
// Call after every add/delete/clear on ExercisePlannedSet so the playlist sidebar stays accurate.
func SyncExerciseSetsCount(gdb *gorm.DB, exerciseID uint) error {
	rows, err := ListExercisePlannedSets(gdb, exerciseID)
	if err != nil {
		return err
	}
	return gdb.Model(&Exercise{}).Where("id = ?", exerciseID).Update("sets", len(rows)).Error
}

// ListCalendarEventsInRange returns all calendar events that overlap [start, end] (inclusive),
// ordered by start_date ascending.
func ListCalendarEventsInRange(gdb *gorm.DB, ownerID uint, start, end time.Time) ([]CalendarEvent, error) {
	var events []CalendarEvent
	err := gdb.Where("owner_id = ? AND start_date <= ? AND end_date >= ?", ownerID, end, start).
		Order("start_date asc").Find(&events).Error
	return events, err
}

// CreateCalendarEvent inserts a new calendar event.
func CreateCalendarEvent(gdb *gorm.DB, event *CalendarEvent) error {
	return gdb.Create(event).Error
}

// UpdateCalendarEvent updates a calendar event owned by ownerID.
func UpdateCalendarEvent(gdb *gorm.DB, ownerID, id uint, title, kind, notes string, startDate, endDate time.Time, blocks bool) error {
	return gdb.Model(&CalendarEvent{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Updates(map[string]any{
			"title":      title,
			"kind":       kind,
			"notes":      notes,
			"start_date": startDate,
			"end_date":   endDate,
			"blocks":     blocks,
		}).Error
}

// DeleteCalendarEvent soft-deletes a calendar event owned by ownerID.
func DeleteCalendarEvent(gdb *gorm.DB, ownerID, id uint) error {
	return gdb.Where("id = ? AND owner_id = ?", id, ownerID).Delete(&CalendarEvent{}).Error
}

// MaterialiseTemplateExercises copies template exercises from a ScheduledSession into
// RunExercise rows for the given run, carrying over any existing completion data.
// Sets ExercisesMaterialised = true so this only runs once.
func MaterialiseTemplateExercises(gdb *gorm.DB, ownerID, runID uint, ss ScheduledSession) error {
	return gdb.Transaction(func(tx *gorm.DB) error {
		var comps []RunExerciseCompletion
		tx.Where("run_id = ? AND owner_id = ?", runID, ownerID).Find(&comps)
		compByExID := make(map[uint]RunExerciseCompletion, len(comps))
		for _, c := range comps {
			compByExID[c.ExerciseID] = c
		}
		orderIdx := 0
		for _, act := range ss.SessionTemplate.Activities {
			for _, ex := range act.Exercises {
				if ex.ParentExerciseID != nil {
					continue
				}
				runEx := Exercise{
					OwnerID:      ownerID,
					SessionRunID: &runID,
					Name:         ex.Name,
					Kind:         ex.Kind,
					OrderIndex:   orderIdx,
				}
				if err := tx.Create(&runEx).Error; err != nil {
					return err
				}
				if comp, ok := compByExID[ex.ID]; ok {
					newComp := RunExerciseCompletion{
						OwnerID:        ownerID,
						RunID:          runID,
						ExerciseID:     runEx.ID,
						Status:         comp.Status,
						RunNotes:       comp.RunNotes,
						ElapsedSeconds: comp.ElapsedSeconds,
						CompletedAt:    comp.CompletedAt,
						ActualSets:     comp.ActualSets,
						ActualReps:     comp.ActualReps,
						ActualWeightKg: comp.ActualWeightKg,
					}
					tx.Create(&newComp)
				}
				orderIdx++
			}
		}
		return tx.Model(&SessionRun{}).
			Where("id = ? AND owner_id = ?", runID, ownerID).
			Update("exercises_materialised", true).Error
	})
}
