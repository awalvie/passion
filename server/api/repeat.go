package api

import (
	"errors"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model repeatRequest
type repeatRequest struct {
	// A session template you can see.
	//
	// required: true
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Template string `json:"template"`

	// The first day, written YYYY-MM-DD.
	//
	// required: true
	// example: 2026-10-27
	Starts string `json:"starts"`

	// The last day, inclusive, written YYYY-MM-DD. Null repeats for ever.
	//
	// example: 2026-12-03
	Ends *string `json:"ends"`

	// Repeats every this many days or weeks, from 1 to 99.
	//
	// required: true
	// example: 1
	Every int `json:"every"`

	// day or week.
	//
	// required: true
	// example: week
	Unit string `json:"unit"`

	// For a weekly repeat, the days it falls on, 1 for Monday to 7 for Sunday.
	// Every few weeks counts from the week of starts.
	//
	// example: [2, 4]
	Weekdays []int `json:"weekdays"`
}

// swagger:model repeatResponse
type repeatResponse struct {
	// The id the client chose.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	repeatRequest
}

// swagger:model repeatPutResponse
type repeatPutResponse struct {
	repeatResponse

	// The days the repeat did not place a session on, because that day already
	// held it.
	LeftOut []slotBody `json:"left_out"`
}

// swagger:model repeatListResponse
type repeatListResponse struct {
	// Latest start first.
	Repeats []repeatResponse `json:"repeats"`
}

// swagger:model endRepeatRequest
type endRepeatRequest struct {
	// The first day to take off, written YYYY-MM-DD.
	//
	// required: true
	// example: 2026-11-10
	From string `json:"from"`
}

// swagger:route GET /api/v1/repeats repeats listRepeats
//
// # List your repeats
//
//	Security:
//	  bearer:
//	Responses:
//	  200: repeatListResponse
//	  401: unauthenticated
func (s *Server) listRepeats(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	list, err := db.ListRepeats(r.Context(), s.pool, who.AccountID)
	if s.failed(w, err) {
		return
	}
	out := repeatListResponse{Repeats: make([]repeatResponse, 0, len(list))}
	for _, rep := range list {
		out.Repeats = append(out.Repeats, toRepeatResponse(rep))
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route PUT /api/v1/repeats/{id} repeats putRepeat
//
// # Create or replace a repeat
//
// Creates the repeat under the id you chose, or replaces it, so a retry makes
// one. It places the session again from today on, up to a year ahead: a past
// day, and a day a run was started from, stay as they are. A day that already
// holds the session is left out and listed.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: repeatPutResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) putRepeat(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req repeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	problems := map[string]string{}
	starts, _ := parseDay(req.Starts, "starts", problems)
	var ends *time.Time
	if req.Ends != nil {
		if day, ok := parseDay(*req.Ends, "ends", problems); ok {
			ends = &day
		}
	}
	f, more := db.RepeatFields{Template: req.Template, Starts: starts, Ends: ends, Every: req.Every, Unit: req.Unit, Weekdays: req.Weekdays}.Clean()
	for k, v := range more {
		problems[k] = v
	}
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	rep, leftOut, err := db.PutRepeat(r.Context(), s.pool, who.AccountID, r.PathValue("id"), f)
	switch {
	case errors.Is(err, db.ErrNoSessionTemplate):
		writeFieldErrors(w, map[string]string{"template": "is not a session template you can see"})
		return
	case errors.Is(err, db.ErrNoRepeat):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	out := repeatPutResponse{repeatResponse: toRepeatResponse(rep), LeftOut: make([]slotBody, 0, len(leftOut))}
	for _, slot := range leftOut {
		out.LeftOut = append(out.LeftOut, slotBody{LocalDate: slot.LocalDate.Format(time.DateOnly), Template: slot.Template})
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route POST /api/v1/repeats/{id}/end repeats endRepeat
//
// # End a repeat
//
// Takes off this day and every later one, except a day a run was started
// from. The earlier days, and the ones you moved or removed, stay as they are.
// Ending it on its first day deletes it.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Ended
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) endRepeat(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req endRepeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	problems := map[string]string{}
	from, _ := parseDay(req.From, "from", problems)
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}
	err := db.EndRepeat(r.Context(), s.pool, who.AccountID, r.PathValue("id"), from)
	if s.failed(w, err, db.ErrNoRepeat) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// swagger:route DELETE /api/v1/repeats/{id} repeats deleteRepeat
//
// # Delete a repeat
//
// Removes it and every day it placed. The runs done from them stay.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Deleted
//	  401: unauthenticated
//	  404: notFound
func (s *Server) deleteRepeat(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.DeleteRepeat(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	if s.failed(w, err, db.ErrNoRepeat) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toRepeatResponse(r db.Repeat) repeatResponse {
	out := repeatResponse{ID: r.ID, repeatRequest: repeatRequest{
		Template: r.Template,
		Starts:   r.Starts.Format(time.DateOnly),
		Every:    r.Every,
		Unit:     r.Unit,
		Weekdays: r.Weekdays,
	}}
	if r.Ends != nil {
		ends := r.Ends.Format(time.DateOnly)
		out.Ends = &ends
	}
	return out
}
