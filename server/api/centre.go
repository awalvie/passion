package api

import (
	"errors"
	"net/http"

	"passion/server/db"
)

// swagger:model centreRequest
type centreRequest struct {
	// Up to 200 characters.
	//
	// required: true
	// example: The Arch
	Name string `json:"name"`

	// The session templates you can do here. Repeats are dropped.
	Sessions []string `json:"sessions"`
}

// swagger:model centreResponse
type centreResponse struct {
	// The id the client chose.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	centreRequest
}

// swagger:model centreListResponse
type centreListResponse struct {
	Centres []centreResponse `json:"centres"`
}

// swagger:route GET /api/v1/centres centres listCentres
//
// # List your climbing centres
//
// By name.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: centreListResponse
//	  401: unauthenticated
func (s *Server) listCentres(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	list, err := db.ListCentres(r.Context(), s.pool, who.AccountID)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}
	out := centreListResponse{Centres: make([]centreResponse, 0, len(list))}
	for _, c := range list {
		out.Centres = append(out.Centres, toCentreResponse(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route PUT /api/v1/centres/{id} centres putCentre
//
// # Create or replace a climbing centre
//
// Creates the centre under the id you chose, or replaces it, so a retry makes
// one centre.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: centreResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) putCentre(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req centreRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}
	f, problems := db.CentreFields{Name: req.Name, Sessions: req.Sessions}.Clean()
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	centre, err := db.PutCentre(r.Context(), s.pool, who.AccountID, r.PathValue("id"), f)
	var unknown *db.UnknownTemplatesError
	switch {
	case errors.As(err, &unknown):
		writeFieldErrors(w, unknown.Problems)
		return
	case errors.Is(err, db.ErrNoCentre):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toCentreResponse(centre))
}

// swagger:route DELETE /api/v1/centres/{id} centres deleteCentre
//
// # Delete a climbing centre
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Deleted
//	  401: unauthenticated
//	  404: notFound
func (s *Server) deleteCentre(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.DeleteCentre(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoCentre):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toCentreResponse(c db.Centre) centreResponse {
	return centreResponse{ID: c.ID, centreRequest: centreRequest{Name: c.Name, Sessions: c.Sessions}}
}
