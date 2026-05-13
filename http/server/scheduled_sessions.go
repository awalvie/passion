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

func (s *Server) handleScheduledSessionsByID(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	scheduledID, err := parseUintParam(r, "scheduledID")
	if err != nil {
		http.Error(w, "invalid scheduled session id", http.StatusBadRequest)
		return
	}

	action := chi.URLParam(r, "action")
	switch action {
	case "preview":
		var ss db.ScheduledSession
		if err := s.store.DB.
			Where("owner_id = ? AND id = ?", ownerID, scheduledID).
			First(&ss).Error; err != nil {
			s.notFound(w)
			return
		}
		tpl, err := s.loadTemplateWithGraph(ss.SessionTemplateID, ownerID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		s.pages.RenderFragment(w, "fragments/scheduled_session_preview", pages.TemplateFragmentData{Template: tpl})
		return
	case "start":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		var ss db.ScheduledSession
		if err := s.store.DB.
			Where("owner_id = ? AND id = ?", ownerID, scheduledID).
			First(&ss).Error; err != nil {
			s.notFound(w)
			return
		}

		run := &db.SessionRun{
			OwnerID:            ownerID,
			ScheduledSessionID: uint(ss.ID),
			IsTrial:            ss.IsTrial,
			Status:             db.RunStatusRunning,
			StartedAt:          time.Now(),
		}
		if err := s.store.DB.Create(run).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(run.ID), 10))
		w.WriteHeader(http.StatusOK)
		return
	case "move":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}

		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}

		dateStr := strings.TrimSpace(r.FormValue("scheduled_date"))
		if dateStr == "" {
			http.Error(w, "scheduled_date is required", http.StatusBadRequest)
			return
		}

		loc := time.Now().Location()
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			http.Error(w, "invalid scheduled_date", http.StatusBadRequest)
			return
		}
		parsed = localDate(parsed)

		var ss db.ScheduledSession
		if err := s.store.DB.
			Where("owner_id = ? AND id = ?", ownerID, scheduledID).
			First(&ss).Error; err != nil {
			s.notFound(w)
			return
		}

		ss.ScheduledDate = parsed
		if err := s.store.DB.Save(&ss).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		// For drag/drop we generally reload the page from the client.
		w.WriteHeader(http.StatusOK)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s *Server) handleAddScheduledSession(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
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

	dateStr := strings.TrimSpace(r.FormValue("scheduled_date"))
	templateIDStr := strings.TrimSpace(r.FormValue("session_template_id"))
	if dateStr == "" || templateIDStr == "" {
		http.Error(w, "scheduled_date and session_template_id are required", http.StatusBadRequest)
		return
	}

	templateID, err := strconv.ParseUint(templateIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid session_template_id", http.StatusBadRequest)
		return
	}

	loc := time.Now().Location()
	parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		http.Error(w, "invalid scheduled_date", http.StatusBadRequest)
		return
	}
	parsed = localDate(parsed)

	// Ensure template belongs to owner.
	var tpl db.SessionTemplate
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, templateID).
		First(&tpl).Error; err != nil {
		s.notFound(w)
		return
	}

	ss := &db.ScheduledSession{
		OwnerID:           ownerID,
		TrainingCycleID:   nil,
		IsTrial:           false,
		ScheduledDate:     parsed,
		SessionTemplateID: uint(tpl.ID),
	}
	if err := s.store.DB.Create(ss).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}
