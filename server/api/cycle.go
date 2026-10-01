package api

import (
	"errors"
	"maps"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model cycleRequest
type cycleRequest struct {
	// Up to 200 characters.
	//
	// required: true
	// example: Spring fingers
	Name string `json:"name"`

	// The first day, written YYYY-MM-DD.
	//
	// required: true
	// example: 2026-03-03
	Starts string `json:"starts"`

	// The last day, included. Within a year of starts.
	//
	// required: true
	// example: 2026-04-13
	Ends string `json:"ends"`

	// How long the repeating block is. Seven lands the sessions on the same
	// weekdays every time.
	//
	// required: true
	// example: 7
	BlockDays int `json:"block_days"`

	// Which session falls on which day of the block.
	Days []cycleDayBody `json:"days"`

	// What the person sets out to reach. Blank lines are dropped.
	Goals []goalBody `json:"goals"`

	// Free-text entries written at the start. Blank ones are dropped.
	//
	// example: ["Max hang, 20 mm, 10 s: +18 kg"]
	Before []string `json:"before"`

	// Free-text entries written at the end. Blank ones are dropped.
	After []string `json:"after"`

	// example: Left shoulder felt tight in week 1.
	Notes *string `json:"notes"`
}

// swagger:model goalBody
type goalBody struct {
	// Up to 200 characters.
	//
	// required: true
	// example: Flash 7a in the gym
	Text string `json:"text"`

	Done bool `json:"done"`
}

// swagger:model cycleDayBody
type cycleDayBody struct {
	// From 1 to block_days.
	//
	// required: true
	// example: 1
	Day int `json:"day"`

	// A session template you can see.
	//
	// required: true
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Template string `json:"template"`
}

// swagger:model cycleResponse
type cycleResponse struct {
	// The id the client chose.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	cycleRequest
}

// swagger:model slotBody
type slotBody struct {
	// example: 2026-03-10
	LocalDate string `json:"local_date"`

	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Template string `json:"template"`
}

// swagger:model cyclePutResponse
type cyclePutResponse struct {
	cycleResponse

	// The days the cycle did not place a session on, because that day already
	// held it.
	LeftOut []slotBody `json:"left_out"`
}

// swagger:model cycleListResponse
type cycleListResponse struct {
	Cycles []cycleResponse `json:"cycles"`
}

// swagger:route GET /api/v1/cycles cycles listCycles
//
// # List your cycles
//
// Latest start first.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: cycleListResponse
//	  401: unauthenticated
func (s *Server) listCycles(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	list, err := db.ListCycles(r.Context(), s.pool, who.AccountID)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}
	out := cycleListResponse{Cycles: make([]cycleResponse, 0, len(list))}
	for _, c := range list {
		out.Cycles = append(out.Cycles, toCycleResponse(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route GET /api/v1/cycles/{id} cycles readCycle
//
// # Read a cycle
//
//	Security:
//	  bearer:
//	Responses:
//	  200: cycleResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) readCycle(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	cycle, err := db.GetCycle(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoCycle):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toCycleResponse(cycle))
}

// swagger:route PUT /api/v1/cycles/{id} cycles putCycle
//
// # Create or replace a cycle
//
// Creates the cycle under the id you chose, or replaces it, so a retry makes
// one cycle. When the dates, the block or the days change, it places the
// sessions again from today on: a past day, and a day a run was started from,
// stay as they are, and so do the days you moved unless the shape changed. A
// day that already holds the session is left out and listed. Once the cycle
// has begun, its start date cannot change.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: cyclePutResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) putCycle(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req cycleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}

	problems := map[string]string{}
	f := db.CycleFields{Name: req.Name, BlockDays: req.BlockDays, Before: req.Before, After: req.After, Notes: req.Notes}
	for _, g := range req.Goals {
		f.Goals = append(f.Goals, db.Goal{Text: g.Text, Done: g.Done})
	}
	starts, okStarts := parseDay(req.Starts, "starts", problems)
	ends, okEnds := parseDay(req.Ends, "ends", problems)
	f.Starts, f.Ends = starts, ends
	for _, d := range req.Days {
		f.Body.Days = append(f.Body.Days, db.CycleDay{Day: d.Day, Template: d.Template})
	}
	f, more := f.Clean()
	if !okStarts || !okEnds {
		// Without both dates the length checks mean nothing.
		delete(more, "ends")
		delete(more, "block_days")
	}
	maps.Copy(problems, more)
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	cycle, leftOut, err := db.PutCycle(r.Context(), s.pool, who.AccountID, r.PathValue("id"), f)
	var unknown *db.UnknownTemplatesError
	switch {
	case errors.As(err, &unknown):
		writeFieldErrors(w, unknown.Problems)
		return
	case errors.Is(err, db.ErrStartsLocked):
		writeFieldErrors(w, map[string]string{"starts": "cannot change once the cycle has begun"})
		return
	case errors.Is(err, db.ErrNoCycle):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	out := cyclePutResponse{cycleResponse: toCycleResponse(cycle), LeftOut: make([]slotBody, 0, len(leftOut))}
	for _, slot := range leftOut {
		out.LeftOut = append(out.LeftOut, slotBody{LocalDate: slot.LocalDate.Format(time.DateOnly), Template: slot.Template})
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route DELETE /api/v1/cycles/{id} cycles deleteCycle
//
// # Delete a cycle
//
// Removes the plan and every day it placed. The runs done under it stay, and
// so do the days you scheduled by hand.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Deleted
//	  401: unauthenticated
//	  404: notFound
func (s *Server) deleteCycle(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.DeleteCycle(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoCycle):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toCycleResponse(c db.Cycle) cycleResponse {
	days := make([]cycleDayBody, 0, len(c.Body.Days))
	for _, d := range c.Body.Days {
		days = append(days, cycleDayBody{Day: d.Day, Template: d.Template})
	}
	goals := make([]goalBody, 0, len(c.Goals))
	for _, g := range c.Goals {
		goals = append(goals, goalBody{Text: g.Text, Done: g.Done})
	}
	return cycleResponse{
		ID: c.ID,
		cycleRequest: cycleRequest{
			Name:      c.Name,
			Starts:    c.Starts.Format(time.DateOnly),
			Ends:      c.Ends.Format(time.DateOnly),
			BlockDays: c.BlockDays,
			Days:      days,
			Goals:     goals,
			Before:    c.Before,
			After:     c.After,
			Notes:     c.Notes,
		},
	}
}
