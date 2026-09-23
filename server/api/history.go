package api

import (
	"errors"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model historySessionBody
type historySessionBody struct {
	// The run's id.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Run string `json:"run"`

	// example: Power
	Name string `json:"name"`

	// example: 2026-03-07
	LocalDate string `json:"local_date"`

	// The exercise's sets in this run, by step.
	Sets []setBody `json:"sets"`

	// The exercise's climbs in this run, in the order they were climbed.
	Climbs []climbBody `json:"climbs"`
}

// swagger:model exerciseHistoryResponse
type exerciseHistoryResponse struct {
	Sessions []historySessionBody `json:"sessions"`
}

// swagger:route GET /api/v1/exercises/{id}/history exercises exerciseHistory
//
// # An exercise's history
//
// Every finished run that logged the exercise, newest day first, with what
// was logged for it. An unfinished run is left out, and a skipped step logs
// nothing. The id need not be in your library: an exercise typed into a run
// has its own id and its own history.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: exerciseHistoryResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) exerciseHistory(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	history, err := db.ExerciseHistory(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoExercise):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	out := exerciseHistoryResponse{Sessions: make([]historySessionBody, 0, len(history))}
	for _, h := range history {
		out.Sessions = append(out.Sessions, historySessionBody{
			Run:       h.Run,
			Name:      h.Name,
			LocalDate: h.LocalDate.Format(time.DateOnly),
			Sets:      toSetBodies(h.Sets),
			Climbs:    toClimbBodies(h.Climbs),
		})
	}
	writeJSON(w, http.StatusOK, out)
}
