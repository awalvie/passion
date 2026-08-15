package web

import (
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

	s.pages.TrainingCycleList(w, pages.TrainingCycleListParams{
		Base:           pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		TrainingCycles: cycles,
	})
}

func (s *Server) handleTrainingCyclesNew(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	switch r.Method {
	case http.MethodGet:
		templates, err := s.listTemplates(ownerID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}

		s.pages.NewTrainingCycle(w, pages.NewTrainingCycleParams{
			Base:      pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
			Templates: templates,
		})
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		startDateStr := strings.TrimSpace(r.FormValue("start_date"))
		if startDateStr == "" {
			http.Error(w, "start_date is required", http.StatusBadRequest)
			return
		}

		weeks, err := strconv.Atoi(strings.TrimSpace(r.FormValue("weeks")))
		if err != nil || weeks <= 0 {
			http.Error(w, "weeks must be a positive integer", http.StatusBadRequest)
			return
		}

		loc := time.Now().Location()
		startDate, err := time.ParseInLocation("2006-01-02", startDateStr, loc)
		if err != nil {
			http.Error(w, "invalid start_date", http.StatusBadRequest)
			return
		}
		startDate = localDate(startDate)

		templates, err := s.listTemplates(ownerID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		allowedTemplateIDs := map[uint]bool{}
		for _, t := range templates {
			allowedTemplateIDs[t.ID] = true
		}

		// Weekday params map to Mon=1..Sun=7
		weekdayKeys := []struct {
			weekday int
			key     string
		}{
			{1, "template_mon"},
			{2, "template_tue"},
			{3, "template_wed"},
			{4, "template_thu"},
			{5, "template_fri"},
			{6, "template_sat"},
			{7, "template_sun"},
		}

		type pendingMapping struct {
			weekday    int
			key        string
			templateID uint
		}
		var pendingMappings []pendingMapping
		for _, m := range weekdayKeys {
			raw := strings.TrimSpace(r.FormValue(m.key))
			if raw == "" {
				continue
			}
			id, err := strconv.ParseUint(raw, 10, 64)
			if err != nil || id == 0 || !allowedTemplateIDs[uint(id)] {
				continue
			}
			pendingMappings = append(pendingMappings, pendingMapping{m.weekday, m.key, uint(id)})
		}

		if len(pendingMappings) == 0 {
			http.Error(w, "select at least one template for a weekday", http.StatusBadRequest)
			return
		}

		// Compute the set of would-be session dates so we can check for conflicts.
		week1Monday := mondayOfLocalDate(startDate)
		cycleEnd := week1Monday.AddDate(0, 0, weeks*7-1)

		confirmed := r.FormValue("confirmed")

		// Conflict detection: only run on first submission (confirmed == "").
		if confirmed == "" {
			blockingEvents, err := db.ListCalendarEventsInRange(s.store.DB, ownerID, startDate, cycleEnd)
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			// Filter to blocking-only events.
			var blocking []db.CalendarEvent
			for _, e := range blockingEvents {
				if e.Blocks {
					blocking = append(blocking, e)
				}
			}

			if len(blocking) > 0 {
				// Compute the set of session dates that would be generated.
				wouldBeKeys := map[string]bool{}
				for weekIdx := 0; weekIdx < weeks; weekIdx++ {
					for _, pm := range pendingMappings {
						d := localDate(week1Monday.AddDate(0, 0, weekIdx*7+(pm.weekday-1)))
						if !d.Before(startDate) {
							wouldBeKeys[localDateKey(d)] = true
						}
					}
				}

				// Match would-be session dates against each blocking event.
				var conflicts []pages.CycleConflictView
				for _, e := range blocking {
					var affected []string
					for d := e.StartDate; !d.After(e.EndDate); d = d.AddDate(0, 0, 1) {
						if wouldBeKeys[localDateKey(d)] {
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

				if len(conflicts) > 0 {
					// Preserve form values for hidden inputs in the conflict review step.
					formValues := map[string]string{
						"name":       name,
						"start_date": startDateStr,
						"weeks":      strconv.Itoa(weeks),
					}
					for _, pm := range pendingMappings {
						formValues[pm.key] = strconv.FormatUint(uint64(pm.templateID), 10)
					}

					s.pages.NewTrainingCycle(w, pages.NewTrainingCycleParams{
						Base:       pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
						Templates:  templates,
						Conflicts:  conflicts,
						FormValues: formValues,
					})
					return
				}
			}
		}

		// Build blocked date set when skipping.
		blockedKeys := map[string]bool{}
		if confirmed == "skip" {
			blockingEvents, err := db.ListCalendarEventsInRange(s.store.DB, ownerID, startDate, cycleEnd)
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			for _, e := range blockingEvents {
				if e.Blocks {
					for d := e.StartDate; !d.After(e.EndDate); d = d.AddDate(0, 0, 1) {
						blockedKeys[localDateKey(d)] = true
					}
				}
			}
		}

		cycle := &db.TrainingCycle{
			OwnerID:   ownerID,
			Name:      name,
			StartDate: startDate,
			Weeks:     weeks,
		}
		if err := s.store.DB.Create(cycle).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		var mappingRows []db.TrainingCycleWeekdayMapping
		for _, pm := range pendingMappings {
			mappingRows = append(mappingRows, db.TrainingCycleWeekdayMapping{
				OwnerID:           ownerID,
				TrainingCycleID:   cycle.ID,
				Weekday:           pm.weekday,
				SessionTemplateID: pm.templateID,
			})
		}
		if err := s.store.DB.Create(&mappingRows).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		// Generate scheduled sessions, skipping blocked dates when requested.
		cycleID := cycle.ID
		for weekIdx := 0; weekIdx < weeks; weekIdx++ {
			for _, mr := range mappingRows {
				scheduled := week1Monday.AddDate(0, 0, weekIdx*7+(mr.Weekday-1))
				scheduled = localDate(scheduled)
				if scheduled.Before(startDate) {
					continue
				}
				if blockedKeys[localDateKey(scheduled)] {
					continue
				}
				ss := &db.ScheduledSession{
					OwnerID:           ownerID,
					TrainingCycleID:   &cycleID,
					ScheduledDate:     scheduled,
					SessionTemplateID: mr.SessionTemplateID,
				}
				if err := s.store.DB.Create(ss).Error; err != nil {
					s.serverError(w, r, err)
					return
				}
			}
		}

		http.Redirect(w, r, "/training-cycles/"+strconv.FormatUint(uint64(cycle.ID), 10), http.StatusSeeOther)
		return
	default:
		s.methodNotAllowed(w)
	}
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
			Base:      pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
			Templates: templates,
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
	goal := strings.TrimSpace(r.FormValue("goal"))

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
	var sessionIDs []uint
	for _, sv := range r.Form["session"] {
		if id, ierr := strconv.ParseUint(sv, 10, 64); ierr == nil && allowed[uint(id)] {
			sessionIDs = append(sessionIDs, uint(id))
		}
	}
	if len(days) == 0 || len(sessionIDs) == 0 {
		http.Error(w, "pick at least one training day and one session", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = strconv.Itoa(weeks) + "-week block"
	}

	startDate := localDate(time.Now())
	week1Monday := mondayOfLocalDate(startDate)

	cycle := &db.TrainingCycle{
		OwnerID: ownerID, Name: name, StartDate: startDate, Weeks: weeks,
		Focus: focus, Goal: goal,
	}
	if err := s.store.DB.Create(cycle).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	cycleID := cycle.ID

	// Round-robin the chosen sessions across the chosen days.
	var mappingRows []db.TrainingCycleWeekdayMapping
	for i, dw := range days {
		mappingRows = append(mappingRows, db.TrainingCycleWeekdayMapping{
			OwnerID: ownerID, TrainingCycleID: cycleID,
			Weekday: dw, SessionTemplateID: sessionIDs[i%len(sessionIDs)],
		})
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
		}
		if s.store.DB.Create(ev).Error == nil {
			// Blocks has a DB default of true and GORM omits the false zero-value on
			// insert; force it off so the deload week is informational, not blocking.
			s.store.DB.Model(ev).Update("blocks", false)
		}
	}

	http.Redirect(w, r, "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10), http.StatusSeeOther)
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
	focus := strings.TrimSpace(r.FormValue("focus"))
	switch focus {
	case "", "strength", "endurance", "technique", "projects", "general":
	default:
		focus = ""
	}
	if name := strings.TrimSpace(r.FormValue("name")); name != "" {
		cycle.Name = name
	}
	cycle.Notes = strings.TrimSpace(r.FormValue("notes"))
	cycle.Focus = focus
	cycle.Label = strings.TrimSpace(r.FormValue("label"))
	cycle.Goal = strings.TrimSpace(r.FormValue("goal"))
	if err := s.store.DB.Save(&cycle).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
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

	weekdayLabels := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	rows := make([]pages.CycleWeekRowView, 0, cycle.Weeks)
	for weekIdx := 0; weekIdx < cycle.Weeks; weekIdx++ {
		cells := make([]pages.CycleDayCellView, 0, 7)
		for dayIdx := 0; dayIdx < 7; dayIdx++ {
			d := gridStart.AddDate(0, 0, weekIdx*7+dayIdx)
			key := localDateKey(d)
			cell := pages.CycleDayCellView{
				DateKey:    key,
				DateLabel:  d.Format("Mon 02"),
				DayNum:     d.Format("2"),
				DayNumber:  d.Day(),
				IsWeekend:  d.Weekday() == time.Saturday || d.Weekday() == time.Sunday,
				HasSession: false,
				Events:     eventsByDateKey[key],
			}
			if ss, ok := sessionsByDateKey[key]; ok {
				cell.HasSession = true
				cell.SessionID = ss.ID
				cell.SessionTemplateName = ss.SessionTemplate.Name
				cell.SessionTemplateColor = ss.SessionTemplate.Color
			}
			cells = append(cells, cell)
		}
		rows = append(rows, pages.CycleWeekRowView{
			WeekNumber: weekIdx + 1,
			Cells:      cells,
		})
	}

	// Load exercise targets for this cycle.
	exerciseOverrides := s.buildCycleExerciseOverrides(cycleID, ownerID, cycle.Weeks)

	s.pages.TrainingCycleDetail(w, pages.TrainingCycleDetailParams{
		Base:               pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		CycleID:            cycleID,
		CycleName:          cycle.Name,
		CycleWeeks:         cycle.Weeks,
		CycleNotes:         cycle.Notes,
		CycleFocus:         cycle.Focus,
		CycleLabel:         cycle.Label,
		CycleGoal:          cycle.Goal,
		CycleWeekdayLabels: weekdayLabels,
		CycleTemplates:     templates,
		CycleRows:          rows,
		TotalScheduled:     len(scheduled),
		ExerciseOverrides:  exerciseOverrides,
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
