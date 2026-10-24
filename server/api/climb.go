package api

import (
	"errors"
	"net/http"
	"time"

	"passion/server/db"
)

// What one climb was. A grade from a scale needs its grade_system: font or v
// for a boulder, french or yds for a route.
//
// swagger:model climbRequest
type climbRequest struct {
	// The climbing step it was climbed under.
	//
	// required: true
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Step string `json:"step"`

	// The order it was climbed in, from 0.
	//
	// example: 0
	Position int `json:"position"`

	// boulder, sport or trad.
	//
	// required: true
	// example: boulder
	Discipline string `json:"discipline"`

	// indoor or outdoor.
	//
	// required: true
	// example: indoor
	Setting string `json:"setting"`

	// A board, for a boulder: kilter, moon, tension, spray or custom.
	//
	// example: moon
	Board *string `json:"board"`

	// For a route: lead, top_rope, auto_belay, auto_belay_rope or follow.
	//
	// example: lead
	RopeStyle *string `json:"rope_style"`

	// As logged. With no grade_system it can be an ungraded label, Rainbow or
	// Traverse.
	//
	// example: 6c
	Grade *string `json:"grade"`

	// font, v, french or yds.
	//
	// example: font
	GradeSystem *string `json:"grade_system"`

	// Each go in order, attempt or send. A climb done in loops holds many
	// sends.
	//
	// example: ["attempt","attempt","send"]
	Goes []string `json:"goes"`

	// How long it took.
	//
	// example: 240
	Seconds *int `json:"seconds"`

	// 1 to 3.
	//
	// example: 2
	Stars *int `json:"stars"`

	// What you meant to work on, set before the climb.
	//
	// example: Heel hooks
	Focus *string `json:"focus"`

	Notes *string `json:"notes"`
}

// swagger:model climbBody
type climbBody struct {
	// The id the client chose.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	// The exercise the step pointed at when the climb was written.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Exercise string `json:"exercise"`

	climbRequest

	// True for a graded climb with at least one send.
	Sent bool `json:"sent"`
}

// swagger:route PUT /api/v1/runs/{id}/climbs/{climb} runs putClimb
//
// # Write a climb
//
// Creates the climb under the id you chose, or replaces it, so a retry writes
// one climb. Its step must be a climbing step the run holds and has not
// skipped. A step with a climb counts as done.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: climbBody
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) putClimb(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req climbRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	f, problems := req.fields().Clean()
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	climb, err := db.PutClimb(r.Context(), s.pool, who.AccountID, r.PathValue("id"), r.PathValue("climb"), f)
	var problem db.StepProblem
	switch {
	case errors.Is(err, db.ErrNoRun), errors.Is(err, db.ErrNoClimb):
		writeNotFound(w)
		return
	case errors.As(err, &problem):
		writeFieldErrors(w, map[string]string{"step": string(problem)})
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toClimbBody(climb))
}

// swagger:route DELETE /api/v1/runs/{id}/climbs/{climb} runs deleteClimb
//
// # Delete a climb
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Deleted
//	  401: unauthenticated
//	  404: notFound
func (s *Server) deleteClimb(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.DeleteClimb(r.Context(), s.pool, who.AccountID, r.PathValue("id"), r.PathValue("climb"))
	if s.failed(w, err, db.ErrNoClimb) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// swagger:model datedClimbBody
type datedClimbBody struct {
	climbBody

	// The run it was logged in.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Run string `json:"run"`

	// The run's day.
	//
	// example: 2026-03-07
	LocalDate string `json:"local_date"`
}

// swagger:model climbListResponse
type climbListResponse struct {
	Climbs []datedClimbBody `json:"climbs"`
}

// swagger:route GET /api/v1/climbs runs listClimbs
//
// # List your climbs
//
// Every climb in your finished runs from one day to another, both included,
// by day and then in the order they were climbed.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: climbListResponse
//	  401: unauthenticated
//	  422: validationFailed
func (s *Server) listClimbs(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	from, to, ok := parseRange(w, r)
	if !ok {
		return
	}

	list, err := db.ListClimbs(r.Context(), s.pool, who.AccountID, from, to)
	if s.failed(w, err) {
		return
	}
	out := climbListResponse{Climbs: make([]datedClimbBody, 0, len(list))}
	for _, c := range list {
		out.Climbs = append(out.Climbs, datedClimbBody{
			climbBody: toClimbBody(c.Climb),
			Run:       c.Run,
			LocalDate: c.LocalDate.Format(time.DateOnly),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (req climbRequest) fields() db.ClimbFields {
	return db.ClimbFields{
		Step:        req.Step,
		Position:    req.Position,
		Discipline:  req.Discipline,
		Setting:     req.Setting,
		Board:       req.Board,
		RopeStyle:   req.RopeStyle,
		Grade:       req.Grade,
		GradeSystem: req.GradeSystem,
		Goes:        req.Goes,
		Seconds:     req.Seconds,
		Stars:       req.Stars,
		Focus:       req.Focus,
		Notes:       req.Notes,
	}
}

func toClimbBody(c db.Climb) climbBody {
	if c.Goes == nil {
		c.Goes = []string{}
	}
	return climbBody{
		ID:       c.ID,
		Exercise: c.Exercise,
		climbRequest: climbRequest{
			Step:        c.Step,
			Position:    c.Position,
			Discipline:  c.Discipline,
			Setting:     c.Setting,
			Board:       c.Board,
			RopeStyle:   c.RopeStyle,
			Grade:       c.Grade,
			GradeSystem: c.GradeSystem,
			Goes:        c.Goes,
			Seconds:     c.Seconds,
			Stars:       c.Stars,
			Focus:       c.Focus,
			Notes:       c.Notes,
		},
		Sent: c.Sent(),
	}
}

func toClimbBodies(climbs []db.Climb) []climbBody {
	out := make([]climbBody, 0, len(climbs))
	for _, c := range climbs {
		out = append(out, toClimbBody(c))
	}
	return out
}
