package api

import (
	"net/http"

	"passion/server/db"
	"passion/server/grades"
)

// swagger:model gradeScaleBody
type gradeScaleBody struct {
	// example: font
	System string `json:"system"`

	// True for a scale that grades boulders. The others grade sport and trad
	// routes.
	Boulder bool `json:"boulder"`

	// Easiest first.
	//
	// example: ["6a", "6a+", "6b"]
	Grades []string `json:"grades"`
}

// swagger:model gradeListResponse
type gradeListResponse struct {
	Scales []gradeScaleBody `json:"scales"`

	// Labels for climbing with no grade, such as a lap. A climb with one has
	// no grade_system and is never a send.
	//
	// example: ["Rainbow", "Traverse"]
	Ungraded []string `json:"ungraded"`
}

// swagger:route GET /api/v1/grades grades listGrades
//
// # List the grade scales
//
// Every scale a climb can be logged in, so the client offers the grades the
// server accepts.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: gradeListResponse
//	  401: unauthenticated
func (s *Server) listGrades(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	out := gradeListResponse{Scales: make([]gradeScaleBody, 0, len(grades.Scales)), Ungraded: grades.Ungraded}
	for _, scale := range grades.Scales {
		out.Scales = append(out.Scales, gradeScaleBody{System: scale.System, Boulder: scale.Boulder, Grades: scale.Grades})
	}
	writeJSON(w, http.StatusOK, out)
}
