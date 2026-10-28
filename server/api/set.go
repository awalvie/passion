package api

import (
	"errors"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model setRequest
type setRequest struct {
	// A step's sets, in order. An empty list removes them all.
	Sets []setFieldsBody `json:"sets"`
}

// What one set did. Every number is optional.
//
// swagger:model setFieldsBody
type setFieldsBody struct {
	// example: 5
	Reps *int `json:"reps"`

	// How long it was held or ran.
	//
	// example: 10
	Seconds *int `json:"seconds"`

	// Each rep's time, when they differ, as a ladder's. seconds is then empty.
	//
	// example: [3, 6, 9]
	RepSeconds []int `json:"rep_seconds"`

	// Added weight. Below zero is assistance, and zero is bodyweight.
	//
	// example: 12.5
	WeightKG *float64 `json:"weight_kg"`
}

// swagger:model setBody
type setBody struct {
	// The step's id.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Step string `json:"step"`

	// From 1.
	//
	// example: 1
	Number int `json:"number"`

	// The exercise the step pointed at when the set was written.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Exercise string `json:"exercise"`

	setFieldsBody
}

// swagger:model setListResponse
type setListResponse struct {
	Sets []setBody `json:"sets"`
}

// swagger:route PUT /api/v1/runs/{id}/steps/{step}/sets runs replaceSets
//
// # Replace a step's sets
//
// Writes the whole list, so a retry writes the same sets. A step with sets
// counts as done. A skipped step keeps no sets, and a climbing step logs
// climbs instead. A step the run does not hold answers 404.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: setListResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) replaceSets(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req setRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	sets := make([]db.SetFields, 0, len(req.Sets))
	for _, set := range req.Sets {
		sets = append(sets, db.SetFields(set))
	}
	if problems := db.CheckSets(sets); len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	written, err := db.ReplaceSets(r.Context(), s.pool, who.AccountID, r.PathValue("id"), r.PathValue("step"), sets)
	var problem db.StepProblem
	switch {
	case errors.Is(err, db.ErrNoRun), errors.Is(err, db.ErrNoStep):
		writeNotFound(w)
		return
	case errors.As(err, &problem):
		writeFieldErrors(w, map[string]string{"step": string(problem)})
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, setListResponse{Sets: toSetBodies(written)})
}

func toSetBodies(sets []db.Set) []setBody {
	out := make([]setBody, 0, len(sets))
	for _, set := range sets {
		out = append(out, toSetBody(set))
	}
	return out
}

func toSetBody(set db.Set) setBody {
	return setBody{
		Step:          set.Step,
		Number:        set.Number,
		Exercise:      set.Exercise,
		setFieldsBody: setFieldsBody{Reps: set.Reps, Seconds: set.Seconds, RepSeconds: set.RepSeconds, WeightKG: set.WeightKG},
	}
}

// swagger:model datedSetBody
type datedSetBody struct {
	setBody

	// The run it was logged in.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Run string `json:"run"`

	// The run's day.
	//
	// example: 2026-03-07
	LocalDate string `json:"local_date"`

	// The step's name as the run held it.
	//
	// example: Max hangs
	Name string `json:"name"`

	// The step's kind as the run held it.
	//
	// example: reps_and_sets
	Kind string `json:"kind"`

	// The step's tags as the run held it.
	//
	// example: ["fingers"]
	Tags []string `json:"tags"`
}

// swagger:model datedSetListResponse
type datedSetListResponse struct {
	Sets []datedSetBody `json:"sets"`
}

// swagger:route GET /api/v1/sets runs listSets
//
// # List your sets
//
// Every set in your finished runs from one day to another, both included,
// by day, then by run, then by step and set, each with its step's name, kind
// and tags.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: datedSetListResponse
//	  401: unauthenticated
//	  422: validationFailed
func (s *Server) listSets(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	from, to, ok := parseRange(w, r)
	if !ok {
		return
	}

	list, err := db.ListSets(r.Context(), s.pool, who.AccountID, from, to)
	if s.failed(w, err) {
		return
	}
	out := datedSetListResponse{Sets: make([]datedSetBody, 0, len(list))}
	for _, set := range list {
		out.Sets = append(out.Sets, datedSetBody{
			setBody:   toSetBody(set.Set),
			Run:       set.Run,
			LocalDate: set.LocalDate.Format(time.DateOnly),
			Name:      set.Name,
			Kind:      set.Kind,
			Tags:      set.Tags,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
