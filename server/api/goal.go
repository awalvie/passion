package api

import (
	"errors"
	"maps"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model goalRequest
type goalRequest struct {
	// required: true
	// example: Train fingers twice
	Text string `json:"text"`

	// week, month, year or custom. It cannot change once the goal is made.
	//
	// required: true
	// example: week
	Period string `json:"period"`

	Span *goalSpanBody `json:"span"`
}

// A custom goal's days. Required for a custom goal, and left out for the
// others. Every goal in the span shares them, so a change here changes them
// all.
//
// swagger:model goalSpan
type goalSpanBody struct {
	// The id the client chose for the span.
	//
	// required: true
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a4
	ID string `json:"id"`

	// Empty when the days are known by their dates.
	//
	// example: Font trip
	Name string `json:"name"`

	// The first day, written YYYY-MM-DD. Moving it moves the span's ticks.
	//
	// required: true
	// example: 2026-11-07
	Starts string `json:"starts"`

	// The last day, included.
	//
	// required: true
	// example: 2026-11-15
	Ends string `json:"ends"`
}

// swagger:model goalTick
type goalTickResponse struct {
	// The first day of the period that was met: a Monday, the 1st of a month,
	// 1 January, or a custom goal's span's first day.
	//
	// example: 2026-10-26
	PeriodStart string `json:"period_start"`

	// The goal's text when it was ticked.
	Text string `json:"text"`
}

// swagger:model goalResponse
type goalResponse struct {
	// The id the client chose.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID     string `json:"id"`
	Text   string `json:"text"`
	Period string `json:"period"`

	// The day it was set, written YYYY-MM-DD. The period holding it is its
	// first.
	//
	// example: 2026-10-31
	Starts string `json:"starts"`

	// The day it was removed, or null. It still counts in the periods that
	// ended before then.
	//
	// example: 2026-12-02
	Removed *string `json:"removed"`

	Span *goalSpanBody `json:"span"`

	// Each period met, oldest first.
	Ticks []goalTickResponse `json:"ticks"`
}

// swagger:model goalListResponse
type goalListResponse struct {
	// Oldest first, removed goals included.
	Goals []goalResponse `json:"goals"`
}

// swagger:route GET /api/v1/goals goals listGoals
//
// # List your goals
//
//	Security:
//	  bearer:
//	Responses:
//	  200: goalListResponse
//	  401: unauthenticated
func (s *Server) listGoals(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	list, err := db.ListGoals(r.Context(), s.pool, who.AccountID)
	if s.failed(w, err) {
		return
	}
	out := goalListResponse{Goals: make([]goalResponse, 0, len(list))}
	for _, g := range list {
		out.Goals = append(out.Goals, toGoalResponse(g))
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route PUT /api/v1/goals/{id} goals putGoal
//
// # Create a goal or change it
//
// Creates the goal under the id you chose, set today, or changes its text, so
// a retry makes one. A custom goal's span can change too.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: goalResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) putGoal(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req goalRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	problems := map[string]string{}
	var span *db.GoalSpanFields
	if req.Span != nil {
		from, _ := parseDay(req.Span.Starts, "span.starts", problems)
		to, _ := parseDay(req.Span.Ends, "span.ends", problems)
		span = &db.GoalSpanFields{ID: req.Span.ID, Name: req.Span.Name, Starts: from, Ends: to}
	}
	f, more := db.PeriodGoalFields{Text: req.Text, Period: req.Period, Span: span}.Clean()
	maps.Copy(problems, more)
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	g, err := db.PutGoal(r.Context(), s.pool, who.AccountID, r.PathValue("id"), f)
	if errors.Is(err, db.ErrPeriodLocked) {
		writeFieldErrors(w, map[string]string{"period": "cannot change once the goal is made"})
		return
	}
	if errors.Is(err, db.ErrSpanLocked) {
		writeFieldErrors(w, map[string]string{"span.id": "cannot change once the goal is made"})
		return
	}
	if s.failed(w, err, db.ErrNoGoal) {
		return
	}
	writeJSON(w, http.StatusOK, toGoalResponse(g))
}

// swagger:route DELETE /api/v1/goals/{id} goals removeGoal
//
// # Remove a goal
//
// It leaves your list from today. It still counts in the periods that ended
// before then, but its ticks can no longer change.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Removed
//	  401: unauthenticated
//	  404: notFound
func (s *Server) removeGoal(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.RemoveGoal(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	if s.failed(w, err, db.ErrNoGoal) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// swagger:route PUT /api/v1/goals/{id}/ticks/{period_start} goals tickGoal
//
// # Mark a period met
//
// period_start is the period's first day: a Monday, the 1st of a month,
// 1 January, or a custom goal's span's first day. It cannot be after today,
// though a tick there can be taken off. Ticking it again keeps the first tick.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: goalResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) tickGoal(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	s.setTick(w, r, who, true)
}

// swagger:route DELETE /api/v1/goals/{id}/ticks/{period_start} goals untickGoal
//
// # Mark a period not met
//
//	Security:
//	  bearer:
//	Responses:
//	  200: goalResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) untickGoal(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	s.setTick(w, r, who, false)
}

func (s *Server) setTick(w http.ResponseWriter, r *http.Request, who db.Authenticated, ticked bool) {
	problems := map[string]string{}
	day, ok := parseDay(r.PathValue("period_start"), "period_start", problems)
	if !ok {
		writeFieldErrors(w, problems)
		return
	}
	g, err := db.TickGoal(r.Context(), s.pool, who.AccountID, r.PathValue("id"), day, ticked)
	if errors.Is(err, db.ErrNotAPeriod) {
		writeFieldErrors(w, map[string]string{"period_start": "must be the first day of one of the goal's periods, up to today"})
		return
	}
	if s.failed(w, err, db.ErrNoGoal) {
		return
	}
	writeJSON(w, http.StatusOK, toGoalResponse(g))
}

func toGoalResponse(g db.PeriodGoal) goalResponse {
	out := goalResponse{ID: g.ID, Text: g.Text, Period: g.Period, Starts: g.Starts.Format(time.DateOnly), Ticks: make([]goalTickResponse, 0, len(g.Ticks))}
	if g.Removed != nil {
		removed := g.Removed.Format(time.DateOnly)
		out.Removed = &removed
	}
	if g.Span != nil {
		span := goalSpanBody(*g.Span)
		out.Span = &span
	}
	for _, t := range g.Ticks {
		out.Ticks = append(out.Ticks, goalTickResponse(t))
	}
	return out
}
