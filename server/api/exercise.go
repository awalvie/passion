package api

import (
	"errors"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model exerciseRequest
type exerciseRequest struct {
	// Up to 200 characters.
	//
	// required: true
	// example: Repeaters 7:3
	Name string `json:"name"`

	// Which player screen runs it: reps_and_sets, timed_reps, climbing or open.
	//
	// required: true
	// example: timed_reps
	Kind string `json:"kind"`

	// example: 7 seconds on, 3 off. Stop when the grip goes.
	Notes *string `json:"notes"`

	// Who or what the method is known by.
	//
	// example: Eva López
	Source *string `json:"source"`

	// example: ["hangboard", "fingers"]
	Tags []string `json:"tags"`

	// example: 4
	Sets *int `json:"sets"`

	// example: 6
	Reps *int `json:"reps"`

	// example: 180
	SetRestSeconds *int `json:"set_rest_seconds"`

	// example: 7
	RepSeconds *int `json:"rep_seconds"`

	// example: 3
	RepRestSeconds *int `json:"rep_rest_seconds"`

	// example: 10
	PrepSeconds *int `json:"prep_seconds"`

	// For an open block: how long it runs.
	//
	// example: 180
	DurationSeconds *int `json:"duration_seconds"`

	// True when the numbers count for each side, as in 6 reps per side.
	PerSide bool `json:"per_side"`

	// Clips of the exercise, in order. An entry with neither link is dropped.
	Media []mediaBody `json:"media"`
}

// swagger:model mediaBody
type mediaBody struct {
	// An http or https link to a video.
	//
	// example: https://www.youtube.com/watch?v=9FImGAOPysY
	URL *string `json:"url"`

	// An http or https link to a still image, for the video or on its own.
	//
	// example: https://i.ytimg.com/vi/9FImGAOPysY/maxresdefault.jpg
	ThumbURL *string `json:"thumb_url"`
}

// swagger:model exerciseResponse
type exerciseResponse struct {
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	// True when the app ships it. Nobody can change a shipped exercise.
	Shipped bool `json:"shipped"`

	// example: Repeaters 7:3
	Name string `json:"name"`

	// example: timed_reps
	Kind string `json:"kind"`

	Notes  *string  `json:"notes"`
	Source *string  `json:"source"`
	Tags   []string `json:"tags"`

	Sets            *int `json:"sets"`
	Reps            *int `json:"reps"`
	SetRestSeconds  *int `json:"set_rest_seconds"`
	RepSeconds      *int `json:"rep_seconds"`
	RepRestSeconds  *int `json:"rep_rest_seconds"`
	PrepSeconds     *int `json:"prep_seconds"`
	DurationSeconds *int `json:"duration_seconds"`
	PerSide         bool `json:"per_side"`

	Media []mediaBody `json:"media"`

	// When it left the library. A retired exercise still reads, because
	// sessions that used it still point at it.
	RetiredAt *time.Time `json:"retired_at"`
}

// swagger:model exerciseListResponse
type exerciseListResponse struct {
	Exercises []exerciseResponse `json:"exercises"`
}

// swagger:route GET /api/v1/exercises exercises listExercises
//
// # List your library
//
// The exercises the app ships and the ones you wrote, sorted by name. Retired
// ones are left out.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: exerciseListResponse
//	  401: unauthenticated
func (s *Server) listExercises(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	list, err := db.ListExercises(r.Context(), s.pool, who.AccountID)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}

	out := exerciseListResponse{Exercises: make([]exerciseResponse, 0, len(list))}
	for _, e := range list {
		out.Exercises = append(out.Exercises, toExerciseResponse(e))
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route POST /api/v1/exercises exercises createExercise
//
// # Create an exercise
//
// Adds one to your library. A field left out or sent as null is not set.
//
//	Security:
//	  bearer:
//	Responses:
//	  201: exerciseResponse
//	  401: unauthenticated
//	  422: validationFailed
func (s *Server) createExercise(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	fields, ok := readExerciseRequest(w, r)
	if !ok {
		return
	}

	created, err := db.CreateExercise(r.Context(), s.pool, who.AccountID, fields)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toExerciseResponse(created))
}

// swagger:route GET /api/v1/exercises/{id} exercises readExercise
//
// # Read an exercise
//
// One the app ships or one of yours. A retired one still reads.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: exerciseResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) readExercise(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	e, err := db.GetExercise(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoExercise):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toExerciseResponse(e))
}

// swagger:route PUT /api/v1/exercises/{id} exercises updateExercise
//
// # Replace an exercise
//
// Sets every field of one of your own. A field left out or sent as null is
// cleared. A shipped exercise, or someone else's, answers 404.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: exerciseResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) updateExercise(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	fields, ok := readExerciseRequest(w, r)
	if !ok {
		return
	}

	updated, err := db.UpdateExercise(r.Context(), s.pool, who.AccountID, r.PathValue("id"), fields)
	switch {
	case errors.Is(err, db.ErrNoExercise):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toExerciseResponse(updated))
}

// swagger:route POST /api/v1/exercises/{id}/retire exercises retireExercise
//
// # Retire an exercise
//
// Takes one of your own out of your library. It still reads by id. Retiring it
// twice keeps the first date. A shipped exercise, or someone else's, answers
// 404.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Retired
//	  401: unauthenticated
//	  404: notFound
func (s *Server) retireExercise(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.RetireExercise(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoExercise):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// readExerciseRequest decodes and checks a body, and answers the request
// itself when the body is wrong.
func readExerciseRequest(w http.ResponseWriter, r *http.Request) (db.ExerciseFields, bool) {
	var req exerciseRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return db.ExerciseFields{}, false
	}

	// Clean names each problem by its column, and every column is spelt the
	// same as its JSON field.
	fields, problems := req.fields().Clean()
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return db.ExerciseFields{}, false
	}
	return fields, true
}

func (req exerciseRequest) fields() db.ExerciseFields {
	media := make([]db.Media, 0, len(req.Media))
	for _, m := range req.Media {
		media = append(media, db.Media{URL: m.URL, ThumbURL: m.ThumbURL})
	}
	return db.ExerciseFields{
		Name:            req.Name,
		Kind:            req.Kind,
		Notes:           req.Notes,
		Source:          req.Source,
		Tags:            req.Tags,
		Sets:            req.Sets,
		Reps:            req.Reps,
		SetRestSeconds:  req.SetRestSeconds,
		RepSeconds:      req.RepSeconds,
		RepRestSeconds:  req.RepRestSeconds,
		PrepSeconds:     req.PrepSeconds,
		DurationSeconds: req.DurationSeconds,
		PerSide:         req.PerSide,
		Media:           media,
	}
}

func toMediaBodies(media []db.Media) []mediaBody {
	out := make([]mediaBody, 0, len(media))
	for _, m := range media {
		out = append(out, mediaBody{URL: m.URL, ThumbURL: m.ThumbURL})
	}
	return out
}

func toExerciseResponse(e db.Exercise) exerciseResponse {
	return exerciseResponse{
		ID:              e.ID,
		Shipped:         e.Owner == nil,
		Name:            e.Name,
		Kind:            e.Kind,
		Notes:           e.Notes,
		Source:          e.Source,
		Tags:            e.Tags,
		Sets:            e.Sets,
		Reps:            e.Reps,
		SetRestSeconds:  e.SetRestSeconds,
		RepSeconds:      e.RepSeconds,
		RepRestSeconds:  e.RepRestSeconds,
		PrepSeconds:     e.PrepSeconds,
		DurationSeconds: e.DurationSeconds,
		PerSide:         e.PerSide,
		Media:           toMediaBodies(e.Media),
		RetiredAt:       e.RetiredAt,
	}
}
