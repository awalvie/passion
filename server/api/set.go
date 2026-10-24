package api

import (
	"errors"
	"net/http"

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
		out = append(out, setBody{
			Step:          set.Step,
			Number:        set.Number,
			Exercise:      set.Exercise,
			setFieldsBody: setFieldsBody{Reps: set.Reps, Seconds: set.Seconds, RepSeconds: set.RepSeconds, WeightKG: set.WeightKG},
		})
	}
	return out
}
