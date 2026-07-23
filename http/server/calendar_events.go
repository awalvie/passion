package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"passion/db"
	"passion/pages"
)

func calendarEventToView(e db.CalendarEvent) pages.CalendarEventView {
	color := pages.CalendarEventColor(e.Kind)
	return pages.CalendarEventView{
		ID:         e.ID,
		Title:      e.Title,
		Kind:       e.Kind,
		Color:      color,
		StartKey:   localDateKey(e.StartDate),
		EndKey:     localDateKey(e.EndDate),
		StartLabel: e.StartDate.Format("Jan 2"),
		EndLabel:   e.EndDate.Format("Jan 2"),
		Notes:      e.Notes,
		Blocks:     e.Blocks,
	}
}

// buildEventsByDateKey maps every calendar date within each event's range to the event view.
func buildEventsByDateKey(events []db.CalendarEvent) map[string][]pages.CalendarEventView {
	m := map[string][]pages.CalendarEventView{}
	for _, e := range events {
		v := calendarEventToView(e)
		for d := e.StartDate; !d.After(e.EndDate); d = d.AddDate(0, 0, 1) {
			key := localDateKey(d)
			m[key] = append(m[key], v)
		}
	}
	return m
}

func (s *Server) handleCalendarEventCreate(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	kind := r.FormValue("kind")
	if kind == "" {
		kind = "other"
	}
	notes := r.FormValue("notes")
	blocks := r.FormValue("blocks") == "on" || r.FormValue("blocks") == "true"

	startDate, err := time.ParseInLocation("2006-01-02", r.FormValue("start_date"), time.Local)
	if err != nil {
		http.Error(w, "invalid start_date", http.StatusBadRequest)
		return
	}
	endDate, err := time.ParseInLocation("2006-01-02", r.FormValue("end_date"), time.Local)
	if err != nil {
		http.Error(w, "invalid end_date", http.StatusBadRequest)
		return
	}
	if endDate.Before(startDate) {
		http.Error(w, "end_date must be >= start_date", http.StatusBadRequest)
		return
	}

	event := &db.CalendarEvent{
		OwnerID:   ownerID,
		Title:     title,
		Kind:      kind,
		StartDate: localDate(startDate),
		EndDate:   localDate(endDate),
		Notes:     notes,
		Blocks:    blocks,
	}
	if err := db.CreateCalendarEvent(s.store.DB, event); err != nil {
		s.serverError(w, r, err)
		return
	}

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/training-cycles"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func (s *Server) handleCalendarEventUpdate(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	eventID, err := strconv.ParseUint(chi.URLParam(r, "eventID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	kind := r.FormValue("kind")
	if kind == "" {
		kind = "other"
	}
	notes := r.FormValue("notes")
	blocks := r.FormValue("blocks") == "on" || r.FormValue("blocks") == "true"

	startDate, err := time.ParseInLocation("2006-01-02", r.FormValue("start_date"), time.Local)
	if err != nil {
		http.Error(w, "invalid start_date", http.StatusBadRequest)
		return
	}
	endDate, err := time.ParseInLocation("2006-01-02", r.FormValue("end_date"), time.Local)
	if err != nil {
		http.Error(w, "invalid end_date", http.StatusBadRequest)
		return
	}
	if endDate.Before(startDate) {
		http.Error(w, "end_date must be >= start_date", http.StatusBadRequest)
		return
	}

	if err := db.UpdateCalendarEvent(s.store.DB, ownerID, uint(eventID),
		title, kind, notes, localDate(startDate), localDate(endDate), blocks,
	); err != nil {
		s.serverError(w, r, err)
		return
	}

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/training-cycles"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func (s *Server) handleCalendarEventDelete(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	eventID, err := strconv.ParseUint(chi.URLParam(r, "eventID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	if err := db.DeleteCalendarEvent(s.store.DB, ownerID, uint(eventID)); err != nil {
		s.serverError(w, r, err)
		return
	}

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/training-cycles"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}
