package web

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	"passion/db"
	"passion/pages"
)

func (s *Server) handleTrainingCycles(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	var cycles []db.TrainingCycle
	err := s.store.DB.
		Where("owner_id = ?", ownerID).
		Order("id desc").
		Find(&cycles).Error
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	// Count scheduled sessions per cycle for the index (the column previously
	// mislabeled the week count as a session count).
	scheduledCounts := map[uint]int{}
	type cycleCount struct {
		TrainingCycleID uint
		N               int
	}
	var counts []cycleCount
	if err := s.store.DB.Model(&db.ScheduledSession{}).
		Select("training_cycle_id, count(*) as n").
		Where("owner_id = ? AND training_cycle_id IS NOT NULL", ownerID).
		Group("training_cycle_id").
		Scan(&counts).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	for _, c := range counts {
		scheduledCounts[c.TrainingCycleID] = c.N
	}

	s.pages.TrainingCycleList(w, pages.TrainingCycleListParams{
		Base:            pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		TrainingCycles:  cycles,
		ScheduledCounts: scheduledCounts,
	})
}

// cycleScheduleSpec describes the sessions a cycle would generate, before any of them
// exist. Both creation paths build one so conflict detection is shared rather than
// copied — the copy is why the guided builder silently had no conflict check at all.
type cycleScheduleSpec struct {
	StartDate time.Time
	Weeks     int
	Weekdays  []int // Mon=1..Sun=7
}

// endDate is the last day of the cycle's final week.
func (spec cycleScheduleSpec) endDate() time.Time {
	return mondayOfLocalDate(spec.StartDate).AddDate(0, 0, spec.Weeks*7-1)
}

// sessionDateKeys returns the local date keys the spec would schedule sessions on.
// Week 1 anchors on the Monday of the start week, so mapped days falling before
// StartDate are excluded — matching the rule both creation loops apply.
func (spec cycleScheduleSpec) sessionDateKeys() map[string]bool {
	keys := map[string]bool{}
	week1Monday := mondayOfLocalDate(spec.StartDate)
	for weekIdx := 0; weekIdx < spec.Weeks; weekIdx++ {
		for _, wd := range spec.Weekdays {
			d := localDate(week1Monday.AddDate(0, 0, weekIdx*7+(wd-1)))
			if !d.Before(spec.StartDate) {
				keys[localDateKey(d)] = true
			}
		}
	}
	return keys
}

// findCycleConflicts returns the training-blocking calendar events that overlap the
// sessions spec would create, along with every date key those events block (used when
// the owner chooses to skip the conflicting days).
func (s *Server) findCycleConflicts(ownerID uint, spec cycleScheduleSpec) ([]pages.CycleConflictView, map[string]bool, error) {
	events, err := db.ListCalendarEventsInRange(s.store.DB, ownerID, spec.StartDate, spec.endDate())
	if err != nil {
		return nil, nil, err
	}
	wouldBe := spec.sessionDateKeys()
	blockedKeys := map[string]bool{}
	var conflicts []pages.CycleConflictView
	for _, e := range events {
		if !e.Blocks {
			continue
		}
		var affected []string
		for d := e.StartDate; !d.After(e.EndDate); d = d.AddDate(0, 0, 1) {
			key := localDateKey(d)
			blockedKeys[key] = true
			if wouldBe[key] {
				affected = append(affected, d.Format("Mon Jan 2"))
			}
		}
		if len(affected) > 0 {
			conflicts = append(conflicts, pages.CycleConflictView{
				EventTitle:    e.Title,
				EventColor:    pages.CalendarEventColor(e.Kind),
				AffectedDates: affected,
				AffectedLabel: strings.Join(affected, ", "),
				AffectedCount: len(affected),
			})
		}
	}
	return conflicts, blockedKeys, nil
}

// handleTrainingCyclesNew is retired. The guided builder is the single creation
// surface; this stays as a redirect for bookmarks and any stale links.
func (s *Server) handleTrainingCyclesNew(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/training-cycles/new/guided", http.StatusFound)
}

// handleTrainingCyclesGuided renders and processes the guided cycle builder — a short
// smart form that asks a few questions (focus, timeframe, days, sessions) and drafts a
// cycle by round-robining the chosen sessions across the chosen days. Output is a normal,
// fully-editable cycle: it redirects into the standard detail-page editor.
func (s *Server) handleTrainingCyclesGuided(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	templates, err := s.listTemplates(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	if r.Method == http.MethodGet {
		s.pages.NewTrainingCycleGuided(w, pages.NewTrainingCycleGuidedParams{
			Base:             pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
			Templates:        templates,
			DefaultStartDate: localDateKey(nextWeekMondayOfLocalDate(time.Now())),
		})
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	focus := strings.TrimSpace(r.FormValue("focus"))
	switch focus {
	case "", "strength", "endurance", "technique", "projects", "general":
	default:
		focus = ""
	}
	// Goals: parallel goal_before[]/goal_after[] arrays (a before→after pair per row,
	// up to 5). Kept as first-class CycleGoal rows, created after the cycle exists.
	goalBefores := r.Form["goal_before"]
	goalAfters := r.Form["goal_after"]
	goalHows := r.Form["goal_how"]

	// Weeks: an explicit count, or counted back from a target date.
	weeks := 0
	if td := strings.TrimSpace(r.FormValue("target_date")); r.FormValue("time_mode") == "date" && td != "" {
		if parsed, perr := time.ParseInLocation("2006-01-02", td, time.Now().Location()); perr == nil {
			days := int(localDate(parsed).Sub(localDate(time.Now())).Hours() / 24)
			weeks = (days + 6) / 7
		}
	} else {
		weeks, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("weeks")))
	}
	if weeks <= 0 {
		weeks = 4
	}
	if weeks > 52 {
		weeks = 52
	}

	allowed := map[uint]bool{}
	for _, t := range templates {
		allowed[t.ID] = true
	}
	var days []int
	for _, d := range r.Form["day"] {
		if n, derr := strconv.Atoi(d); derr == nil && n >= 1 && n <= 7 {
			days = append(days, n)
		}
	}
	sort.Ints(days)
	// Dedupe: a repeated weekday would create two mappings (and two scheduled sessions
	// on the same date), unlike the manual form's one-field-per-weekday layout.
	var uniqDays []int
	for i, d := range days {
		if i == 0 || d != days[i-1] {
			uniqDays = append(uniqDays, d)
		}
	}
	days = uniqDays
	if len(days) == 0 {
		http.Error(w, "pick at least one training day", http.StatusBadRequest)
		return
	}

	// Per-day session assignment: each chosen day carries its own session_day_<weekday>
	// select. Build the weekday→template mappings directly (no round-robin here — the
	// form's defaults already rotated the sessions across days client-side).
	dayTemplate := map[int]uint{}
	for _, dw := range days {
		sv := strings.TrimSpace(r.FormValue("session_day_" + strconv.Itoa(dw)))
		if id, ierr := strconv.ParseUint(sv, 10, 64); ierr == nil && allowed[uint(id)] {
			dayTemplate[dw] = uint(id)
		}
	}
	if len(dayTemplate) == 0 {
		http.Error(w, "pick a session for at least one training day", http.StatusBadRequest)
		return
	}

	// Label and notes are plain fields now that nothing else is competing to write
	// into Notes (equipment and per-day energy used to be folded in as text lines).
	label := strings.TrimSpace(r.FormValue("label"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = strconv.Itoa(weeks) + "-week cycle"
	}

	// An explicit date always wins. Left blank, the cycle starts on next week's Monday:
	// week 1 then contains every mapped weekday, where a mid-week start silently drops
	// the days that fall before it.
	startDate := nextWeekMondayOfLocalDate(time.Now())
	if raw := strings.TrimSpace(r.FormValue("start_date")); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, time.Now().Location())
		if err != nil {
			http.Error(w, "invalid start_date", http.StatusBadRequest)
			return
		}
		startDate = localDate(parsed)
	}
	week1Monday := mondayOfLocalDate(startDate)

	// Conflict check, shared with the quick path. The guided builder previously had
	// none at all, so it would happily schedule a cycle straight through a trip or an
	// injury week that the calendar had marked as blocking training.
	var weekdays []int
	for _, dw := range days {
		if _, ok := dayTemplate[dw]; ok {
			weekdays = append(weekdays, dw)
		}
	}
	confirmed := r.FormValue("confirmed")
	blockedKeys := map[string]bool{}
	if confirmed == "" || confirmed == "skip" {
		conflicts, blocked, err := s.findCycleConflicts(ownerID, cycleScheduleSpec{
			StartDate: startDate, Weeks: weeks, Weekdays: weekdays,
		})
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if confirmed == "" && len(conflicts) > 0 {
			// Carry the whole submission forward so the review step can resubmit it
			// verbatim; the guided form has too many fields to enumerate by hand, and
			// repeats `day` once per chosen weekday.
			var fields []pages.CycleFormField
			for key, values := range r.Form {
				if key == "confirmed" {
					continue
				}
				for _, v := range values {
					fields = append(fields, pages.CycleFormField{Key: key, Value: v})
				}
			}
			sort.Slice(fields, func(i, j int) bool {
				if fields[i].Key != fields[j].Key {
					return fields[i].Key < fields[j].Key
				}
				return fields[i].Value < fields[j].Value
			})
			s.pages.NewTrainingCycleGuided(w, pages.NewTrainingCycleGuidedParams{
				Base:             pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
				Templates:        templates,
				DefaultStartDate: localDateKey(startDate),
				Conflicts:        conflicts,
				FormFields:       fields,
			})
			return
		}
		if confirmed == "skip" {
			blockedKeys = blocked
		}
	}

	cycle := &db.TrainingCycle{
		OwnerID: ownerID, Name: name, StartDate: startDate, Weeks: weeks,
		Focus: focus, Notes: notes, Label: label,
	}
	if err := s.store.DB.Create(cycle).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	cycleID := cycle.ID

	// First-class goals (before → after pairs, up to 5). Skip fully-empty rows.
	var goalRows []db.CycleGoal
	for i := range goalBefores {
		var after, how string
		if i < len(goalAfters) {
			after = strings.TrimSpace(goalAfters[i])
		}
		if i < len(goalHows) {
			how = strings.TrimSpace(goalHows[i])
		}
		before := strings.TrimSpace(goalBefores[i])
		if before == "" && after == "" && how == "" {
			continue
		}
		goalRows = append(goalRows, db.CycleGoal{
			OwnerID: ownerID, TrainingCycleID: cycleID,
			Before: before, After: after, How: how, OrderIndex: len(goalRows),
		})
		if len(goalRows) >= 5 {
			break
		}
	}
	if len(goalRows) > 0 {
		if err := s.store.DB.Create(&goalRows).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	var mappingRows []db.TrainingCycleWeekdayMapping
	for _, dw := range days {
		if id, ok := dayTemplate[dw]; ok {
			mappingRows = append(mappingRows, db.TrainingCycleWeekdayMapping{
				OwnerID: ownerID, TrainingCycleID: cycleID,
				Weekday: dw, SessionTemplateID: id,
			})
		}
	}
	if err := s.store.DB.Create(&mappingRows).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	for weekIdx := 0; weekIdx < weeks; weekIdx++ {
		for _, mr := range mappingRows {
			scheduled := localDate(week1Monday.AddDate(0, 0, weekIdx*7+(mr.Weekday-1)))
			if scheduled.Before(startDate) {
				continue
			}
			if blockedKeys[localDateKey(scheduled)] {
				continue
			}
			ss := &db.ScheduledSession{
				OwnerID: ownerID, TrainingCycleID: &cycleID,
				ScheduledDate: scheduled, SessionTemplateID: mr.SessionTemplateID,
			}
			if err := s.store.DB.Create(ss).Error; err != nil {
				s.serverError(w, r, err)
				return
			}
		}
	}

	// Optional deload: a non-blocking rest event over the final week.
	if r.FormValue("deload") == "1" && weeks >= 2 {
		ds := localDate(week1Monday.AddDate(0, 0, (weeks-1)*7))
		ev := &db.CalendarEvent{
			OwnerID: ownerID, Title: "Deload week", Kind: "rest",
			StartDate: ds, EndDate: localDate(ds.AddDate(0, 0, 6)),
			TrainingCycleID: &cycleID,
		}
		if s.store.DB.Create(ev).Error == nil {
			// Blocks has a DB default of true and GORM omits the false zero-value on
			// insert; force it off so the deload week is informational, not blocking.
			s.store.DB.Model(ev).Update("blocks", false)
		}
	}

	// Optional rest period: an informational (non-blocking) rest event over a
	// user-chosen date range (trip, injury, time off). Silently skipped if the
	// range is missing or invalid — it's an optional field, not a hard error.
	if r.FormValue("rest_enabled") == "1" {
		rs := strings.TrimSpace(r.FormValue("rest_start"))
		re := strings.TrimSpace(r.FormValue("rest_end"))
		start, serr := time.ParseInLocation("2006-01-02", rs, time.Now().Location())
		end, eerr := time.ParseInLocation("2006-01-02", re, time.Now().Location())
		if serr == nil && eerr == nil && !end.Before(start) {
			ev := &db.CalendarEvent{
				OwnerID: ownerID, Title: "Rest period", Kind: "rest",
				StartDate: localDate(start), EndDate: localDate(end),
				TrainingCycleID: &cycleID,
			}
			if s.store.DB.Create(ev).Error == nil {
				s.store.DB.Model(ev).Update("blocks", false)
			}
		}
	}

	// End on the targets page so per-cycle targets are configurable at creation, not
	// only afterwards. The list is derived from the templates just scheduled, so it
	// cannot be a field earlier in the builder. /targets redirects straight through to
	// the cycle when the cycle has nothing targetable, so this never dead-ends.
	http.Redirect(w, r,
		"/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10)+"/targets?new=1",
		http.StatusSeeOther)
}

func (s *Server) handleTrainingCyclesByID(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	cycleID, err := parseUintParam(r, "cycleID")
	if err != nil {
		http.Error(w, "invalid training cycle id", http.StatusBadRequest)
		return
	}

	action := chi.URLParam(r, "action")
	// Detail page: GET /training-cycles/{id}
	if action == "" {
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		s.renderTrainingCycleDetail(w, r, cycleID, ownerID)
		return
	}

	// Exercise targets page: GET /training-cycles/{id}/targets
	if action == "targets" {
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		s.renderCycleTargets(w, r, cycleID, ownerID)
		return
	}

	// Edit endpoints:
	// POST /training-cycles/{id}/move
	// POST /training-cycles/{id}/add
	// POST /training-cycles/{id}/remove
	if r.Method == http.MethodPost {
		switch action {
		case "move":
			s.handleTrainingCycleMove(w, r, cycleID, ownerID)
			return
		case "add":
			s.handleTrainingCycleAdd(w, r, cycleID, ownerID)
			return
		case "remove":
			s.handleTrainingCycleRemove(w, r, cycleID, ownerID)
			return
		case "override-save":
			s.handleCycleOverrideSave(w, r, cycleID, ownerID)
			return
		case "override-clear":
			s.handleCycleOverrideClear(w, r, cycleID, ownerID)
			return
		case "details-save":
			s.handleCycleDetailsSave(w, r, cycleID, ownerID)
			return
		case "delete":
			s.handleTrainingCycleDelete(w, r, cycleID, ownerID)
			return
		}
	}

	http.NotFound(w, r)
}

// handleTrainingCycleDelete removes a cycle while preserving logged history: scheduled
// sessions that already have runs are detached (kept in history, unlinked from the
// cycle); unrun sessions, the weekday map, and the exercise overrides are removed.
func (s *Server) handleTrainingCycleDelete(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	var cycle db.TrainingCycle
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, cycleID).First(&cycle).Error; err != nil {
		s.notFound(w)
		return
	}
	err := s.store.DB.Transaction(func(tx *gorm.DB) error {
		var scheduled []db.ScheduledSession
		if err := tx.Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).Find(&scheduled).Error; err != nil {
			return err
		}
		for _, ss := range scheduled {
			var runCount int64
			if err := tx.Model(&db.SessionRun{}).Where("scheduled_session_id = ?", ss.ID).Count(&runCount).Error; err != nil {
				return err
			}
			if runCount > 0 {
				// Keep the session and its runs; just unlink from the cycle.
				if err := tx.Model(&db.ScheduledSession{}).Where("id = ?", ss.ID).
					Update("training_cycle_id", nil).Error; err != nil {
					return err
				}
			} else if err := tx.Unscoped().Delete(&db.ScheduledSession{}, ss.ID).Error; err != nil {
				return err
			}
		}
		if err := tx.Unscoped().Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
			Delete(&db.CycleExerciseOverride{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
			Delete(&db.CycleExerciseWeekOverride{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
			Delete(&db.TrainingCycleWeekdayMapping{}).Error; err != nil {
			return err
		}
		// Goals are plan metadata (no history to preserve) — hard-delete with the
		// cycle. The CASCADE tag isn't relied on: soft-deletes don't trigger it.
		if err := tx.Unscoped().Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
			Delete(&db.CycleGoal{}).Error; err != nil {
			return err
		}
		// The deload and rest events the builder created belong to the cycle, and used
		// to outlive it as unexplained entries on the calendar. Events added by hand
		// have a nil TrainingCycleID and are left alone.
		if err := tx.Unscoped().Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
			Delete(&db.CalendarEvent{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&db.TrainingCycle{}, cycleID).Error
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("HX-Redirect", "/training-cycles")
	w.WriteHeader(http.StatusOK)
}

// handleCycleDetailsSave autosaves the optional cycle metadata (notes / focus / tag / goal).
func (s *Server) handleCycleDetailsSave(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}
	var cycle db.TrainingCycle
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, cycleID).First(&cycle).Error; err != nil {
		s.notFound(w)
		return
	}
	// Every write is guarded on the field actually being present, so a form carrying
	// only some of the cycle's metadata updates only those fields. This handler used to
	// assume one form carrying everything: it overwrote Notes/Focus/Label from the
	// request and hard-deleted every CycleGoal row before recreating them. Once goals
	// and notes live in separate forms on the page, that assumption silently destroys
	// data — a notes-only save would wipe the goals. FormValue cannot tell "absent"
	// from "empty", so presence is checked with Form.Has.
	if name := strings.TrimSpace(r.FormValue("name")); name != "" {
		cycle.Name = name
	}
	if r.Form.Has("notes") {
		cycle.Notes = strings.TrimSpace(r.FormValue("notes"))
	}
	if r.Form.Has("focus") {
		focus := strings.TrimSpace(r.FormValue("focus"))
		switch focus {
		case "", "strength", "endurance", "technique", "projects", "general":
		default:
			focus = ""
		}
		cycle.Focus = focus
	}
	if r.Form.Has("label") {
		cycle.Label = strings.TrimSpace(r.FormValue("label"))
	}
	// Goals are first-class CycleGoal rows. Only touch them — and only clear the legacy
	// Goal column that would otherwise shadow them — when goals were actually submitted.
	submittedGoals := r.Form.Has("goal_before")
	if submittedGoals {
		cycle.Goal = ""
	}
	if err := s.store.DB.Save(&cycle).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	if !submittedGoals {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Replace the cycle's goals with the submitted before→after pairs (up to 5).
	if err := s.store.DB.Unscoped().Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
		Delete(&db.CycleGoal{}).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	befores := r.Form["goal_before"]
	afters := r.Form["goal_after"]
	hows := r.Form["goal_how"]
	var goalRows []db.CycleGoal
	for i := range befores {
		var after, how string
		if i < len(afters) {
			after = strings.TrimSpace(afters[i])
		}
		if i < len(hows) {
			how = strings.TrimSpace(hows[i])
		}
		before := strings.TrimSpace(befores[i])
		if before == "" && after == "" && how == "" {
			continue
		}
		goalRows = append(goalRows, db.CycleGoal{
			OwnerID: ownerID, TrainingCycleID: cycleID,
			Before: before, After: after, How: how, OrderIndex: len(goalRows),
		})
		if len(goalRows) >= 5 {
			break
		}
	}
	if len(goalRows) > 0 {
		if err := s.store.DB.Create(&goalRows).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// openWeekIndex picks the single week the phone layout expands: the one containing
// today, else the first week for a cycle that has not started and the last for one
// already finished. Returns -1 when there are no weeks.
func openWeekIndex(rows []pages.CycleWeekRowView, today, gridStart time.Time) int {
	if len(rows) == 0 {
		return -1
	}
	for i, row := range rows {
		if row.IsCurrent {
			return i
		}
	}
	if today.Before(gridStart) {
		return 0
	}
	return len(rows) - 1
}

// cycleProgressLine describes where the owner is in the cycle. Counts rather than a
// percentage: at the ~16-20 sessions a cycle holds, a count says what to do next and a
// percentage does not.
//
// Three states, because the old static chip never had to distinguish them:
//   - not started yet  → say when it starts, not "week 1 of 4"
//   - finished         → say so, with the final tally
//   - in flight        → week, tally, and what is left this week
func cycleProgressLine(
	cycle db.TrainingCycle,
	gridStart, gridEnd, today time.Time,
	rows []pages.CycleWeekRowView,
	totalPlanned, totalDone int,
) string {
	weeks := cycle.Weeks
	plural := func(n int, one, many string) string {
		if n == 1 {
			return one
		}
		return many
	}
	tally := fmt.Sprintf("%d of %d session%s done", totalDone, totalPlanned, plural(totalPlanned, "", "s"))

	if today.Before(localDate(cycle.StartDate)) {
		return fmt.Sprintf("Starts %s · %d week%s · %d session%s planned",
			cycle.StartDate.Format("Mon Jan 2"), weeks, plural(weeks, "", "s"),
			totalPlanned, plural(totalPlanned, "", "s"))
	}
	if today.After(gridEnd) {
		return "Finished · " + tally
	}

	week := int(today.Sub(gridStart).Hours()/24)/7 + 1
	if week < 1 {
		week = 1
	}
	if week > weeks {
		week = weeks
	}
	line := fmt.Sprintf("Week %d of %d · %s", week, weeks, tally)

	// "Left this week" counts only sessions still ahead. A past day with no run is
	// missed, not pending, and calling it "left" would quietly overstate what is doable.
	left := 0
	for _, row := range rows {
		if !row.IsCurrent {
			continue
		}
		for _, c := range row.Cells {
			if !c.HasSession || c.HasCompletedRun {
				continue
			}
			if d, err := time.ParseInLocation("2006-01-02", c.DateKey, today.Location()); err == nil && !d.Before(today) {
				left++
			}
		}
	}
	if left > 0 {
		line += fmt.Sprintf(" · %d left this week", left)
	}
	return line
}

// renderCycleTargets serves the standalone exercise-targets page. Reached from the
// cycle header, and from the end of the guided builder with ?new=1 so targets can be
// set at creation — the list is derived from the templates scheduled into the cycle, so
// it cannot exist before the cycle does.
func (s *Server) renderCycleTargets(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	var cycle db.TrainingCycle
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, cycleID).First(&cycle).Error; err != nil {
		s.notFound(w)
		return
	}

	overrides := s.buildCycleExerciseOverrides(cycleID, ownerID, cycle.Weeks)
	if len(overrides) == 0 {
		// Nothing targetable in this cycle's templates, so there is no page to show.
		// Matters most for the creation redirect: landing on an empty page after every
		// build would be worse than not stopping here at all.
		http.Redirect(w, r, "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10), http.StatusFound)
		return
	}

	s.pages.CycleTargets(w, pages.CycleTargetsParams{
		Base:              pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		CycleID:           cycleID,
		CycleName:         cycle.Name,
		CycleWeeks:        cycle.Weeks,
		ExerciseOverrides: overrides,
		IsNewCycle:        r.URL.Query().Get("new") == "1",
	})
}

func (s *Server) renderTrainingCycleDetail(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	var cycle db.TrainingCycle
	res := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, cycleID).
		Find(&cycle)
	if res.Error != nil {
		http.Error(w, res.Error.Error(), http.StatusInternalServerError)
		return
	}
	if res.RowsAffected == 0 {
		http.Error(w, "Training cycle not found", http.StatusNotFound)
		return
	}

	gridStart := mondayOfLocalDate(cycle.StartDate)
	gridEnd := gridStart.AddDate(0, 0, cycle.Weeks*7-1)

	// Load available templates for "Add" controls.
	templates, err := s.listTemplates(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	var scheduled []db.ScheduledSession
	if err := s.store.DB.
		Preload("SessionTemplate").
		Where("owner_id = ? AND training_cycle_id = ? AND scheduled_date >= ? AND scheduled_date <= ?",
			ownerID, cycleID, gridStart, gridEnd).
		Order("scheduled_date asc").
		Find(&scheduled).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	sessionsByDateKey := map[string]db.ScheduledSession{}
	for _, ss := range scheduled {
		key := localDateKey(ss.ScheduledDate)
		sessionsByDateKey[key] = ss
	}

	// Which of the cycle's sessions were actually run. The page showed only the plan
	// before this, so there was no way to see progress without going to the history.
	completedSSID := map[uint]bool{}
	if len(scheduled) > 0 {
		ssIDs := make([]uint, 0, len(scheduled))
		for _, ss := range scheduled {
			ssIDs = append(ssIDs, ss.ID)
		}
		var completedRuns []db.SessionRun
		if err := s.store.DB.
			Where("owner_id = ? AND status = ? AND scheduled_session_id IN ?",
				ownerID, db.RunStatusCompleted, ssIDs).
			Find(&completedRuns).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
		for _, run := range completedRuns {
			completedSSID[run.ScheduledSessionID] = true
		}
	}

	// Load calendar events covering the cycle range and index by date key.
	calEvents, err := db.ListCalendarEventsInRange(s.store.DB, ownerID, gridStart, gridEnd)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	eventsByDateKey := buildEventsByDateKey(calEvents)

	// Build full event view list for the add/edit/delete dialogs.
	eventViews := make([]pages.CalendarEventView, len(calEvents))
	for i, e := range calEvents {
		eventViews[i] = calendarEventToView(e)
	}

	today := localDate(time.Now())
	weekdayLabels := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	rows := make([]pages.CycleWeekRowView, 0, cycle.Weeks)
	for weekIdx := 0; weekIdx < cycle.Weeks; weekIdx++ {
		cells := make([]pages.CycleDayCellView, 0, 7)
		for dayIdx := 0; dayIdx < 7; dayIdx++ {
			d := gridStart.AddDate(0, 0, weekIdx*7+dayIdx)
			key := localDateKey(d)
			cell := pages.CycleDayCellView{
				DateKey:      key,
				DateLabel:    d.Format("Mon 02"),
				DayNum:       d.Format("2"),
				DayNumber:    d.Day(),
				WeekdayShort: d.Format("Mon"),
				IsWeekend:    d.Weekday() == time.Saturday || d.Weekday() == time.Sunday,
				HasSession:   false,
				Events:       eventsByDateKey[key],
			}
			if ss, ok := sessionsByDateKey[key]; ok {
				cell.HasSession = true
				cell.SessionID = ss.ID
				cell.SessionTemplateName = ss.SessionTemplate.Name
				cell.SessionTemplateColor = ss.SessionTemplate.Color
				cell.HasCompletedRun = completedSSID[ss.ID]
				// Dim, never red: the plan is a suggestion and the run is what happened.
				cell.IsMissed = !cell.HasCompletedRun && d.Before(today)
			}
			cells = append(cells, cell)
		}
		weekStart := gridStart.AddDate(0, 0, weekIdx*7)
		planned, done := 0, 0
		// The phone layout renders a row per session and collapses the rest, so the
		// week is split up here rather than filtered in the template.
		var sessionCells []pages.CycleDayCellView
		var restDays []string
		var freeDays []pages.CycleFreeDay
		var weekEvents []pages.CalendarEventView
		seenEvent := map[uint]bool{}
		for _, c := range cells {
			if c.HasSession {
				planned++
				if c.HasCompletedRun {
					done++
				}
				sessionCells = append(sessionCells, c)
			} else {
				freeDays = append(freeDays, pages.CycleFreeDay{DateKey: c.DateKey, Label: c.DateLabel})
				// A day carrying an event is not a rest day: its chip belongs in the
				// band, and calling it rest would hide a trip or an injury week.
				if len(c.Events) == 0 {
					restDays = append(restDays, c.WeekdayShort)
				}
			}
			for _, ev := range c.Events {
				if !seenEvent[ev.ID] {
					seenEvent[ev.ID] = true
					weekEvents = append(weekEvents, ev)
				}
			}
		}
		// A week with nothing planned reads better as one sentence than as all seven
		// weekdays listed out as rest.
		restLabel := ""
		if planned == 0 {
			restLabel = ""
		} else if len(restDays) > 0 {
			restLabel = strings.Join(restDays, ", ") + " — rest"
		}
		rows = append(rows, pages.CycleWeekRowView{
			WeekNumber:   weekIdx + 1,
			Cells:        cells,
			Planned:      planned,
			Done:         done,
			IsCurrent:    !today.Before(weekStart) && today.Before(weekStart.AddDate(0, 0, 7)),
			SessionCells: sessionCells,
			RestLabel:    restLabel,
			WeekEvents:   weekEvents,
			FreeDays:     freeDays,
		})
	}

	if i := openWeekIndex(rows, today, gridStart); i >= 0 {
		rows[i].IsOpen = true
	}

	progressLine := cycleProgressLine(cycle, gridStart, gridEnd, today, rows, len(scheduled), len(completedSSID))

	// Only the count: the targets themselves render on /targets now.
	targetCount := len(s.buildCycleExerciseOverrides(cycleID, ownerID, cycle.Weeks))

	// Load goals; fall back to the legacy single Goal column for cycles created
	// before multi-goals shipped (shown as one target, no "before").
	var goals []db.CycleGoal
	s.store.DB.Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
		Order("order_index asc, id asc").Find(&goals)
	goalViews := make([]pages.CycleGoalView, 0, len(goals))
	for _, g := range goals {
		goalViews = append(goalViews, pages.CycleGoalView{Before: g.Before, After: g.After, How: g.How})
	}
	if len(goalViews) == 0 && strings.TrimSpace(cycle.Goal) != "" {
		goalViews = append(goalViews, pages.CycleGoalView{After: cycle.Goal})
	}

	s.pages.TrainingCycleDetail(w, pages.TrainingCycleDetailParams{
		Base:               pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		CycleID:            cycleID,
		CycleName:          cycle.Name,
		CycleWeeks:         cycle.Weeks,
		CycleNotes:         cycle.Notes,
		CycleFocus:         cycle.Focus,
		CycleLabel:         cycle.Label,
		CycleGoals:         goalViews,
		CycleWeekdayLabels: weekdayLabels,
		CycleTemplates:     templates,
		CycleRows:          rows,
		TotalScheduled:     len(scheduled),
		ProgressLine:       progressLine,
		TargetCount:        targetCount,
		Events:             eventViews,
	})
}

func (s *Server) handleTrainingCycleMove(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	scheduledSessionIDStr := strings.TrimSpace(r.FormValue("scheduled_session_id"))
	targetDateStr := strings.TrimSpace(r.FormValue("scheduled_date"))
	if scheduledSessionIDStr == "" || targetDateStr == "" {
		http.Error(w, "scheduled_session_id and scheduled_date are required", http.StatusBadRequest)
		return
	}

	scheduledSessionID, err := strconv.ParseUint(scheduledSessionIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid scheduled_session_id", http.StatusBadRequest)
		return
	}

	loc := time.Now().Location()
	parsed, err := time.ParseInLocation("2006-01-02", targetDateStr, loc)
	if err != nil {
		http.Error(w, "invalid scheduled_date", http.StatusBadRequest)
		return
	}
	parsed = localDate(parsed)

	var ss db.ScheduledSession
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND training_cycle_id = ?", ownerID, scheduledSessionID, cycleID).
		First(&ss).Error; err != nil {
		s.notFound(w)
		return
	}

	ss.ScheduledDate = parsed
	if err := s.store.DB.Save(&ss).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTrainingCycleAdd(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	targetDateStr := strings.TrimSpace(r.FormValue("scheduled_date"))
	templateIDStr := strings.TrimSpace(r.FormValue("session_template_id"))
	if targetDateStr == "" || templateIDStr == "" {
		http.Error(w, "scheduled_date and session_template_id are required", http.StatusBadRequest)
		return
	}

	templateID, err := strconv.ParseUint(templateIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid session_template_id", http.StatusBadRequest)
		return
	}

	var tpl db.SessionTemplate
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, templateID).
		First(&tpl).Error; err != nil {
		s.notFound(w)
		return
	}

	loc := time.Now().Location()
	parsed, err := time.ParseInLocation("2006-01-02", targetDateStr, loc)
	if err != nil {
		http.Error(w, "invalid scheduled_date", http.StatusBadRequest)
		return
	}
	parsed = localDate(parsed)

	// Replace if one already exists for that day.
	var existing db.ScheduledSession
	err = s.store.DB.
		Where("owner_id = ? AND training_cycle_id = ? AND scheduled_date = ?", ownerID, cycleID, parsed).
		First(&existing).Error
	if err == nil {
		existing.SessionTemplateID = uint(tpl.ID)
		if err := s.store.DB.Save(&existing).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
		w.WriteHeader(http.StatusOK)
		return
	}

	ss := &db.ScheduledSession{
		OwnerID:           ownerID,
		TrainingCycleID:   &cycleID,
		ScheduledDate:     parsed,
		SessionTemplateID: uint(tpl.ID),
	}
	if err := s.store.DB.Create(ss).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTrainingCycleRemove(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	scheduledSessionIDStr := strings.TrimSpace(r.FormValue("scheduled_session_id"))
	if scheduledSessionIDStr == "" {
		http.Error(w, "scheduled_session_id is required", http.StatusBadRequest)
		return
	}

	scheduledSessionID, err := strconv.ParseUint(scheduledSessionIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid scheduled_session_id", http.StatusBadRequest)
		return
	}

	// Ensure it belongs to the cycle.
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND training_cycle_id = ?", ownerID, scheduledSessionID, cycleID).
		Delete(&db.ScheduledSession{}).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
	w.WriteHeader(http.StatusOK)
}

// buildCycleExerciseOverrides collects all unique targetable exercises across
