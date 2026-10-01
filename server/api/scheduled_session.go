package api

import (
	"errors"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model scheduleRequest
type scheduleRequest struct {
	// A session template you can see.
	//
	// required: true
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Template string `json:"template"`

	// The day, written YYYY-MM-DD.
	//
	// required: true
	// example: 2026-03-10
	LocalDate string `json:"local_date"`
}

// swagger:model moveRequest
type moveRequest struct {
	// The new day, written YYYY-MM-DD.
	//
	// required: true
	// example: 2026-03-11
	LocalDate string `json:"local_date"`
}

// swagger:model scheduledSessionBody
type scheduledSessionBody struct {
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	// The cycle that placed it. null for a session you scheduled by hand.
	Cycle *string `json:"cycle"`

	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Template string `json:"template"`

	// example: 2026-03-10
	LocalDate string `json:"local_date"`
}

// swagger:model scheduledDayBody
type scheduledDayBody struct {
	scheduledSessionBody

	// The template's name today. A future session follows its template.
	//
	// example: Power
	TemplateName string `json:"template_name"`

	// The template's icon today.
	//
	// example: dumbbell
	TemplateIcon *string `json:"template_icon"`

	// The run started from it, a finished one first. null before it starts.
	Run *string `json:"run"`

	// done (a finished run), started (a run not finished yet), missed (a day
	// gone with no run), or planned.
	//
	// example: planned
	Status string `json:"status"`
}

// swagger:model scheduledDayListResponse
type scheduledDayListResponse struct {
	Days []scheduledDayBody `json:"days"`
}

// swagger:route GET /api/v1/scheduled-sessions scheduled-sessions listScheduledSessions
//
// # List your schedule
//
// Every session planned from one day to another, both included, in date
// order. A day before today in your time zone with no run is missed.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: scheduledDayListResponse
//	  401: unauthenticated
//	  422: validationFailed
func (s *Server) listScheduledSessions(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	problems := map[string]string{}
	query := r.URL.Query()
	from, okFrom := parseDay(query.Get("from"), "from", problems)
	to, okTo := parseDay(query.Get("to"), "to", problems)
	if okFrom && okTo && to.Before(from) {
		problems["to"] = "must be on or after from"
	}
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	list, err := db.ListScheduledSessions(r.Context(), s.pool, who.AccountID, from, to)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}
	out := scheduledDayListResponse{Days: make([]scheduledDayBody, 0, len(list))}
	for _, d := range list {
		out.Days = append(out.Days, scheduledDayBody{
			scheduledSessionBody: toScheduledSessionBody(d.ScheduledSession),
			TemplateName:         d.TemplateName,
			TemplateIcon:         d.TemplateIcon,
			Run:                  d.Run,
			Status:               d.Status,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route POST /api/v1/scheduled-sessions scheduled-sessions scheduleSession
//
// # Schedule a session
//
// A one-off, with no cycle behind it. A day holds each session once.
//
//	Security:
//	  bearer:
//	Responses:
//	  201: scheduledSessionBody
//	  401: unauthenticated
//	  422: validationFailed
func (s *Server) scheduleSession(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req scheduleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}
	problems := map[string]string{}
	day, _ := parseDay(req.LocalDate, "local_date", problems)
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	scheduled, err := db.ScheduleSession(r.Context(), s.pool, who.AccountID, req.Template, day)
	switch {
	case errors.Is(err, db.ErrNoSessionTemplate):
		writeFieldErrors(w, map[string]string{"template": "is not a session template you can see"})
		return
	case errors.Is(err, db.ErrAlreadyScheduled):
		writeFieldErrors(w, map[string]string{"local_date": "already holds this session"})
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toScheduledSessionBody(scheduled))
}

// swagger:route PUT /api/v1/scheduled-sessions/{id} scheduled-sessions moveScheduledSession
//
// # Move a scheduled session
//
// Puts it on another day and leaves every other day alone.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: scheduledSessionBody
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) moveScheduledSession(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req moveRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}
	problems := map[string]string{}
	day, _ := parseDay(req.LocalDate, "local_date", problems)
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	moved, err := db.MoveScheduledSession(r.Context(), s.pool, who.AccountID, r.PathValue("id"), day)
	switch {
	case errors.Is(err, db.ErrNoScheduledSession):
		writeNotFound(w)
		return
	case errors.Is(err, db.ErrAlreadyScheduled):
		writeFieldErrors(w, map[string]string{"local_date": "already holds this session"})
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toScheduledSessionBody(moved))
}

// swagger:route DELETE /api/v1/scheduled-sessions/{id} scheduled-sessions deleteScheduledSession
//
// # Take a session off its day
//
// A run started from it stays.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Deleted
//	  401: unauthenticated
//	  404: notFound
func (s *Server) deleteScheduledSession(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.DeleteScheduledSession(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoScheduledSession):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toScheduledSessionBody(s db.ScheduledSession) scheduledSessionBody {
	return scheduledSessionBody{
		ID:        s.ID,
		Cycle:     s.Cycle,
		Template:  s.Template,
		LocalDate: s.LocalDate.Format(time.DateOnly),
	}
}
