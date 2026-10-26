package api

import (
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model timeOffRequest
type timeOffRequest struct {
	// trip, injury, illness or other.
	//
	// required: true
	// example: trip
	Kind string `json:"kind"`

	// required: true
	// example: Fontainebleau
	Name string `json:"name"`

	// The first day, written YYYY-MM-DD.
	//
	// required: true
	// example: 2026-11-07
	Starts string `json:"starts"`

	// The last day, inclusive, written YYYY-MM-DD. Null while the end is not
	// known yet.
	//
	// example: 2026-11-15
	Ends *string `json:"ends"`

	// Whether the planned sessions inside it are paused. True when left out.
	//
	// example: true
	Pauses *bool `json:"pauses"`
}

// swagger:model timeOffResponse
type timeOffResponse struct {
	// The id the client chose.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Starts string `json:"starts"`

	// Null while the end is not known yet.
	Ends *string `json:"ends"`

	Pauses bool `json:"pauses"`
}

// swagger:model timeOffListResponse
type timeOffListResponse struct {
	// Latest start first.
	TimeOff []timeOffResponse `json:"time_off"`
}

// swagger:route GET /api/v1/time-off time-off listTimeOff
//
// # List your time off
//
//	Security:
//	  bearer:
//	Responses:
//	  200: timeOffListResponse
//	  401: unauthenticated
func (s *Server) listTimeOff(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	list, err := db.ListTimeOff(r.Context(), s.pool, who.AccountID)
	if s.failed(w, err) {
		return
	}
	out := timeOffListResponse{TimeOff: make([]timeOffResponse, 0, len(list))}
	for _, t := range list {
		out.TimeOff = append(out.TimeOff, toTimeOffResponse(t))
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route GET /api/v1/time-off/{id} time-off readTimeOff
//
// # Read a time off
//
//	Security:
//	  bearer:
//	Responses:
//	  200: timeOffResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) readTimeOff(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	t, err := db.GetTimeOff(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	if s.failed(w, err, db.ErrNoTimeOff) {
		return
	}
	writeJSON(w, http.StatusOK, toTimeOffResponse(t))
}

// swagger:route PUT /api/v1/time-off/{id} time-off putTimeOff
//
// # Create or replace a time off
//
// Creates the time off under the id you chose, or replaces it, so a retry
// makes one.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: timeOffResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) putTimeOff(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req timeOffRequest
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
	pauses := req.Pauses == nil || *req.Pauses
	f, more := db.TimeOffFields{Kind: req.Kind, Name: req.Name, Starts: starts, Ends: ends, Pauses: pauses}.Clean()
	for k, v := range more {
		problems[k] = v
	}
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	t, err := db.PutTimeOff(r.Context(), s.pool, who.AccountID, r.PathValue("id"), f)
	if s.failed(w, err, db.ErrNoTimeOff) {
		return
	}
	writeJSON(w, http.StatusOK, toTimeOffResponse(t))
}

// swagger:route DELETE /api/v1/time-off/{id} time-off deleteTimeOff
//
// # Delete a time off
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Deleted
//	  401: unauthenticated
//	  404: notFound
func (s *Server) deleteTimeOff(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.DeleteTimeOff(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	if s.failed(w, err, db.ErrNoTimeOff) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toTimeOffResponse(t db.TimeOff) timeOffResponse {
	out := timeOffResponse{ID: t.ID, Kind: t.Kind, Name: t.Name, Starts: t.Starts.Format(time.DateOnly), Pauses: t.Pauses}
	if t.Ends != nil {
		ends := t.Ends.Format(time.DateOnly)
		out.Ends = &ends
	}
	return out
}
