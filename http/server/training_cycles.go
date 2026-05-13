package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

		// Weekday params map to Mon=1..Sun=7
		mappings := []struct {
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

		var mappingRows []db.TrainingCycleWeekdayMapping
		for _, m := range mappings {
			raw := strings.TrimSpace(r.FormValue(m.key))
			if raw == "" {
				continue
			}
			id, err := strconv.ParseUint(raw, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			if !allowedTemplateIDs[uint(id)] {
				continue
			}
			mappingRows = append(mappingRows, db.TrainingCycleWeekdayMapping{
				OwnerID:           ownerID,
				TrainingCycleID:   cycle.ID,
				Weekday:           m.weekday,
				SessionTemplateID: uint(id),
			})
		}

		if len(mappingRows) == 0 {
			http.Error(w, "select at least one template for a weekday", http.StatusBadRequest)
			return
		}

		if err := s.store.DB.Create(&mappingRows).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		// Generate scheduled sessions.
		week1Monday := mondayOfLocalDate(startDate)
		cycleID := cycle.ID
		for weekIdx := 0; weekIdx < weeks; weekIdx++ {
			for _, mr := range mappingRows {
				scheduled := week1Monday.AddDate(0, 0, weekIdx*7+(mr.Weekday-1))
				scheduled = localDate(scheduled)
				if scheduled.Before(startDate) {
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
		}
	}

	http.NotFound(w, r)
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
		CycleWeekdayLabels: weekdayLabels,
		CycleTemplates:     templates,
		CycleRows:          rows,
		TotalScheduled:     len(scheduled),
		ExerciseOverrides:  exerciseOverrides,
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
