package api

import (
	"errors"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model sessionTemplateRequest
type sessionTemplateRequest struct {
	// Up to 200 characters.
	//
	// required: true
	// example: Power
	Name string `json:"name"`

	// example: Warm up well. The hangs come first.
	Notes *string `json:"notes"`

	// Who or what the session is known by.
	Source *string `json:"source"`

	// A colour for the session, written #rrggbb.
	//
	// example: #5d86c9
	Color *string `json:"color"`

	// What the session needs to run.
	//
	// example: a hangboard and a bar
	Needs *string `json:"needs"`

	// example: ["power", "fingers"]
	Tags []string `json:"tags"`

	// The session's parts, in order.
	Sections []sectionBody `json:"sections"`
}

// swagger:model sectionBody
type sectionBody struct {
	// Up to 200 characters.
	//
	// required: true
	// example: Warm-up
	Name string `json:"name"`

	// example: Go slow, it is cold.
	Notes *string `json:"notes"`

	// The section's steps and choices, in order.
	Items []itemBody `json:"items"`
}

// An item holds a step or a choice. Send exactly one.
//
// swagger:model itemBody
type itemBody struct {
	Step   *stepBody   `json:"step,omitempty"`
	Choice *choiceBody `json:"choice,omitempty"`
}

// swagger:model choiceBody
type choiceBody struct {
	// Up to 200 characters.
	//
	// required: true
	// example: Shoulders
	Name string `json:"name"`

	// example: Pick what feels good today.
	Notes *string `json:"notes"`

	// The fewest options to do. 0 lets the whole choice be skipped. It cannot
	// be more than the options.
	//
	// minimum: 0
	// example: 1
	Pick int `json:"pick"`

	// At least one.
	Options []stepBody `json:"options"`
}

// A step is a library exercise and its own copy of that exercise's fields.
// The session shows the copy, so edit it to change the numbers for this
// session only.
//
// swagger:model stepBody
type stepBody struct {
	// A shipped exercise or one of yours. A retired one is allowed.
	//
	// required: true
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Exercise string `json:"exercise"`

	exerciseRequest
}

// swagger:model sessionTemplateResponse
type sessionTemplateResponse struct {
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	// True when the app ships it. Nobody can change a shipped session template.
	Shipped bool `json:"shipped"`

	// example: Power
	Name string `json:"name"`

	Notes    *string       `json:"notes"`
	Source   *string       `json:"source"`
	Color    *string       `json:"color"`
	Needs    *string       `json:"needs"`
	Tags     []string      `json:"tags"`
	Sections []sectionBody `json:"sections"`

	// When it left the list. A retired session template still reads.
	RetiredAt *time.Time `json:"retired_at"`
}

// swagger:model sessionTemplateListResponse
type sessionTemplateListResponse struct {
	SessionTemplates []sessionTemplateResponse `json:"session_templates"`
}

// swagger:route GET /api/v1/session-templates session-templates listSessionTemplates
//
// # List your session templates
//
// The ones the app ships and the ones you wrote, sorted by name. Retired ones
// are left out.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: sessionTemplateListResponse
//	  401: unauthenticated
func (s *Server) listSessionTemplates(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	list, err := db.ListSessionTemplates(r.Context(), s.pool, who.AccountID)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}

	out := sessionTemplateListResponse{SessionTemplates: make([]sessionTemplateResponse, 0, len(list))}
	for _, t := range list {
		out.SessionTemplates = append(out.SessionTemplates, toSessionTemplateResponse(t))
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route POST /api/v1/session-templates session-templates createSessionTemplate
//
// # Create a session template
//
// A field left out or sent as null is not set. A problem inside a section is
// named by its path, such as sections[0].items[2].choice.pick.
//
//	Security:
//	  bearer:
//	Responses:
//	  201: sessionTemplateResponse
//	  401: unauthenticated
//	  422: validationFailed
func (s *Server) createSessionTemplate(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	fields, ok := readSessionTemplateRequest(w, r)
	if !ok {
		return
	}

	created, err := db.CreateSessionTemplate(r.Context(), s.pool, who.AccountID, fields)
	if writeUnknownExercises(w, err) {
		return
	}
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toSessionTemplateResponse(created))
}

// swagger:route GET /api/v1/session-templates/{id} session-templates readSessionTemplate
//
// # Read a session template
//
// One the app ships or one of yours. A retired one still reads.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: sessionTemplateResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) readSessionTemplate(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	t, err := db.GetSessionTemplate(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoSessionTemplate):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toSessionTemplateResponse(t))
}

// swagger:route PUT /api/v1/session-templates/{id} session-templates updateSessionTemplate
//
// # Replace a session template
//
// Sets every field of one of your own, sections included. A field left out or
// sent as null is cleared. A shipped session template, or someone else's,
// answers 404.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: sessionTemplateResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) updateSessionTemplate(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	fields, ok := readSessionTemplateRequest(w, r)
	if !ok {
		return
	}

	updated, err := db.UpdateSessionTemplate(r.Context(), s.pool, who.AccountID, r.PathValue("id"), fields)
	if writeUnknownExercises(w, err) {
		return
	}
	switch {
	case errors.Is(err, db.ErrNoSessionTemplate):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toSessionTemplateResponse(updated))
}

// swagger:route POST /api/v1/session-templates/{id}/retire session-templates retireSessionTemplate
//
// # Retire a session template
//
// Takes one of your own out of your list. It still reads by id. Retiring it
// twice keeps the first date. A shipped session template, or someone else's,
// answers 404.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Retired
//	  401: unauthenticated
//	  404: notFound
func (s *Server) retireSessionTemplate(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.RetireSessionTemplate(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoSessionTemplate):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// readSessionTemplateRequest decodes and checks a body, and answers the
// request itself when the body is wrong.
func readSessionTemplateRequest(w http.ResponseWriter, r *http.Request) (db.SessionTemplateFields, bool) {
	var req sessionTemplateRequest
	if err := decodeJSONUpTo(w, r, &req, maxSessionBodyBytes); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return db.SessionTemplateFields{}, false
	}

	fields, problems := req.fields().Clean()
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return db.SessionTemplateFields{}, false
	}
	return fields, true
}

// writeUnknownExercises answers a body whose steps name exercises outside
// the person's library, and reports whether it did.
func writeUnknownExercises(w http.ResponseWriter, err error) bool {
	var unknown *db.UnknownExercisesError
	if !errors.As(err, &unknown) {
		return false
	}
	writeFieldErrors(w, unknown.Problems)
	return true
}

func (req sessionTemplateRequest) fields() db.SessionTemplateFields {
	sections := make([]db.Section, 0, len(req.Sections))
	for _, s := range req.Sections {
		items := make([]db.Item, 0, len(s.Items))
		for _, item := range s.Items {
			var out db.Item
			if item.Step != nil {
				step := item.Step.step()
				out.Step = &step
			}
			if item.Choice != nil {
				choice := item.Choice.choice()
				out.Choice = &choice
			}
			items = append(items, out)
		}
		sections = append(sections, db.Section{Name: s.Name, Notes: s.Notes, Items: items})
	}

	return db.SessionTemplateFields{
		Name:   req.Name,
		Notes:  req.Notes,
		Source: req.Source,
		Color:  req.Color,
		Needs:  req.Needs,
		Tags:   req.Tags,
		Body:   db.SessionBody{Sections: sections},
	}
}

func (s stepBody) step() db.Step {
	return db.Step{Exercise: s.Exercise, ExerciseFields: s.exerciseRequest.fields()}
}

func (c choiceBody) choice() db.Choice {
	options := make([]db.Step, 0, len(c.Options))
	for _, option := range c.Options {
		options = append(options, option.step())
	}
	return db.Choice{Name: c.Name, Notes: c.Notes, Pick: c.Pick, Options: options}
}

func toChoiceBody(c db.Choice) choiceBody {
	options := make([]stepBody, 0, len(c.Options))
	for _, option := range c.Options {
		options = append(options, toStepBody(option))
	}
	return choiceBody{Name: c.Name, Notes: c.Notes, Pick: c.Pick, Options: options}
}

func toStepBody(s db.Step) stepBody {
	f := s.ExerciseFields
	return stepBody{
		Exercise: s.Exercise,
		exerciseRequest: exerciseRequest{
			Name:            f.Name,
			Kind:            f.Kind,
			Notes:           f.Notes,
			Source:          f.Source,
			Tags:            f.Tags,
			Sets:            f.Sets,
			Reps:            f.Reps,
			SetRestSeconds:  f.SetRestSeconds,
			RepSeconds:      f.RepSeconds,
			RepRestSeconds:  f.RepRestSeconds,
			PrepSeconds:     f.PrepSeconds,
			DurationSeconds: f.DurationSeconds,
			PerSide:         f.PerSide,
			Media:           toMediaBodies(f.Media),
		},
	}
}

func toSectionBodies(b db.SessionBody) []sectionBody {
	sections := make([]sectionBody, 0, len(b.Sections))
	for _, s := range b.Sections {
		items := make([]itemBody, 0, len(s.Items))
		for _, item := range s.Items {
			var out itemBody
			if item.Step != nil {
				step := toStepBody(*item.Step)
				out.Step = &step
			}
			if item.Choice != nil {
				choice := toChoiceBody(*item.Choice)
				out.Choice = &choice
			}
			items = append(items, out)
		}
		sections = append(sections, sectionBody{Name: s.Name, Notes: s.Notes, Items: items})
	}
	return sections
}

func toSessionTemplateResponse(t db.SessionTemplate) sessionTemplateResponse {
	return sessionTemplateResponse{
		ID:        t.ID,
		Shipped:   t.Owner == nil,
		Name:      t.Name,
		Notes:     t.Notes,
		Source:    t.Source,
		Color:     t.Color,
		Needs:     t.Needs,
		Tags:      t.Tags,
		Sections:  toSectionBodies(t.Body),
		RetiredAt: t.RetiredAt,
	}
}
