package api

import (
	"errors"
	"maps"
	"net/http"
	"strings"
	"time"

	"passion/server/db"
)

// swagger:model runStartRequest
type runStartRequest struct {
	// A session template to copy now. Leave it out for an open run.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Template *string `json:"template"`

	// Required for an open run, up to 200 characters. A planned run takes its
	// template's name.
	//
	// example: Open session
	Name string `json:"name"`

	// When it started. Defaults to now.
	StartedAt *time.Time `json:"started_at"`

	// The day it counts on, written YYYY-MM-DD, for a write-up of a day already
	// gone. Defaults to the day of started_at in your time zone.
	//
	// example: 2026-03-07
	LocalDate *string `json:"local_date"`
}

// swagger:model runRequest
type runRequest struct {
	// Up to 200 characters.
	//
	// required: true
	// example: Power
	Name string `json:"name"`

	// The working list, in order.
	Sections []runSectionBody `json:"sections"`

	// required: true
	StartedAt *time.Time `json:"started_at"`

	// The day it counts on, written YYYY-MM-DD.
	//
	// required: true
	// example: 2026-03-07
	LocalDate string `json:"local_date"`

	// What the session timer showed, in seconds.
	//
	// example: 4200
	ElapsedSeconds *int `json:"elapsed_seconds"`

	// Where it happened. A copy of the name, so renaming a gym later changes no
	// past run.
	//
	// example: The Castle
	Place *string `json:"place"`

	Notes *string `json:"notes"`

	journalBody
}

// V1's end-of-run journal. Every field is optional.
//
// swagger:model journalBody
type journalBody struct {
	// Sleep quality, 1 to 5.
	//
	// example: 3
	Sleep *int `json:"sleep"`

	// Energy before the session, 1 to 5.
	//
	// example: 4
	Energy *int `json:"energy"`

	// How hard the whole session felt, 1 to 10.
	//
	// example: 7
	RPE *int `json:"rpe"`

	// strength, endurance, technique, projects or general.
	//
	// example: technique
	Focus *string `json:"focus"`

	// indoor or outdoor.
	//
	// example: indoor
	Setting *string `json:"setting"`

	// example: Left hand felt solid.
	WentWell *string `json:"went_well"`

	// example: Start the second set less tired.
	NextFocus *string `json:"next_focus"`
}

// swagger:model runSectionBody
type runSectionBody struct {
	// Up to 200 characters.
	//
	// required: true
	// example: Fingers
	Name string `json:"name"`

	Notes *string `json:"notes"`

	// The section's steps and open choices, in order.
	Items []runItemBody `json:"items"`
}

// An item holds a step or a choice. Send exactly one.
//
// swagger:model runItemBody
type runItemBody struct {
	Step   *runStepBody   `json:"step,omitempty"`
	Choice *runChoiceBody `json:"choice,omitempty"`
}

// A choice nobody has picked from yet. A pick replaces it with steps.
//
// swagger:model runChoiceBody
type runChoiceBody struct {
	// A uuid, unique in the run.
	//
	// required: true
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	choiceBody
}

// A step in a run: an exercise's copy, and what happened to it.
//
// swagger:model runStepBody
type runStepBody struct {
	// A uuid, unique in the run. The server writes them for a template's steps.
	// Write one for a step you add.
	//
	// required: true
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	stepBody

	// null until the run reaches it, then done or skipped. Finishing the run
	// skips every step still null.
	//
	// example: done
	Status *string `json:"status"`

	// Notes written during the run.
	//
	// example: Right shoulder tight.
	RunNotes *string `json:"run_notes"`

	// How long the step took.
	//
	// example: 412
	ElapsedSeconds *int `json:"elapsed_seconds"`

	// The choice a picked step came from, so the pick can be changed. The steps
	// of one pick share it.
	FromChoice *runChoiceBody `json:"from_choice,omitempty"`
}

// swagger:model runResponse
type runResponse struct {
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	// The session template it was copied from. null for an open run.
	Template *string `json:"template"`

	// example: Power
	Name string `json:"name"`

	// The template's sections as they were when the run started. null for an
	// open run. Nothing changes it.
	Plan []sectionBody `json:"plan"`

	// The working list.
	Sections []runSectionBody `json:"sections"`

	StartedAt time.Time `json:"started_at"`

	// The time zone the run's day was worked out in.
	//
	// example: Europe/London
	Timezone string `json:"timezone"`

	// example: 2026-03-07
	LocalDate string `json:"local_date"`

	// null until the run is finished. An unfinished run counts for nothing.
	FinishedAt *time.Time `json:"finished_at"`

	ElapsedSeconds *int    `json:"elapsed_seconds"`
	Place          *string `json:"place"`
	Notes          *string `json:"notes"`

	journalBody

	// Every set logged in the run, by step.
	Sets []setBody `json:"sets"`

	// Every climb logged in the run, in the order they were climbed.
	Climbs []climbBody `json:"climbs"`
}

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

	// For a route: lead, top_rope, auto_belay or follow.
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

	// onsight, flash, redpoint, hangdog or working.
	//
	// example: flash
	Outcome *string `json:"outcome"`

	// 1 or more.
	//
	// example: 3
	Attempts *int `json:"attempts"`

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

	climbRequest

	// True for a graded onsight, flash or redpoint.
	Sent bool `json:"sent"`
}

// swagger:model runSummaryBody
type runSummaryBody struct {
	ID             string     `json:"id"`
	Template       *string    `json:"template"`
	Name           string     `json:"name"`
	LocalDate      string     `json:"local_date"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	ElapsedSeconds *int       `json:"elapsed_seconds"`
	Place          *string    `json:"place"`
}

// swagger:model runListResponse
type runListResponse struct {
	Runs []runSummaryBody `json:"runs"`
}

// swagger:route GET /api/v1/runs runs listRuns
//
// # List your runs
//
// Finished or not, newest day first.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: runListResponse
//	  401: unauthenticated
func (s *Server) listRuns(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	list, err := db.ListRuns(r.Context(), s.pool, who.AccountID)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}

	out := runListResponse{Runs: make([]runSummaryBody, 0, len(list))}
	for _, run := range list {
		out.Runs = append(out.Runs, runSummaryBody{
			ID:             run.ID,
			Template:       run.Template,
			Name:           run.Name,
			LocalDate:      run.LocalDate.Format(time.DateOnly),
			StartedAt:      run.StartedAt,
			FinishedAt:     run.FinishedAt,
			ElapsedSeconds: run.ElapsedSeconds,
			Place:          run.Place,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route POST /api/v1/runs runs startRun
//
// # Start a run
//
// With a template, the run copies it now, so editing the template later never
// reaches the run. Without one, it is an open run that starts empty. A
// write-up of a day already gone sends that day as local_date.
//
//	Security:
//	  bearer:
//	Responses:
//	  201: runResponse
//	  401: unauthenticated
//	  422: validationFailed
func (s *Server) startRun(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req runStartRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}

	problems := map[string]string{}
	start := db.RunStart{Template: req.Template, Name: req.Name, StartedAt: time.Now()}
	if req.StartedAt != nil {
		start.StartedAt = *req.StartedAt
	}
	if req.LocalDate != nil {
		day, ok := parseDay(*req.LocalDate, "local_date", problems)
		if ok {
			start.LocalDate = &day
		}
	}
	start, more := start.Clean()
	maps.Copy(problems, more)
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	run, err := db.StartRun(r.Context(), s.pool, who.AccountID, start)
	switch {
	case errors.Is(err, db.ErrNoSessionTemplate):
		writeFieldErrors(w, map[string]string{"template": "is not a session template you can see"})
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRunResponse(run))
}

// swagger:route GET /api/v1/runs/{id} runs readRun
//
// # Read a run
//
//	Security:
//	  bearer:
//	Responses:
//	  200: runResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) readRun(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	run, err := db.GetRun(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoRun):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toRunResponse(run))
}

// swagger:route PUT /api/v1/runs/{id} runs updateRun
//
// # Replace a run
//
// Sets everything but the plan, the template and the finish. A field left out
// or sent as null is cleared. A finished run can still be changed.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: runResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) updateRun(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req runRequest
	if err := decodeJSONUpTo(w, r, &req, maxSessionBodyBytes); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}

	problems := map[string]string{}
	f := req.fields()
	if req.StartedAt == nil {
		problems["started_at"] = "is required"
	} else {
		f.StartedAt = *req.StartedAt
	}
	if day, ok := parseDay(req.LocalDate, "local_date", problems); ok {
		f.LocalDate = day
	}
	f, more := f.Clean()
	maps.Copy(problems, more)
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	updated, err := db.UpdateRun(r.Context(), s.pool, who.AccountID, r.PathValue("id"), f)
	switch {
	case errors.Is(err, db.ErrNoRun):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toRunResponse(updated))
}

// swagger:route POST /api/v1/runs/{id}/finish runs finishRun
//
// # Finish a run
//
// Every step still null becomes skipped. Finishing twice keeps the first
// time.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: runResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) finishRun(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	run, err := db.FinishRun(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoRun):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toRunResponse(run))
}

// swagger:route DELETE /api/v1/runs/{id} runs deleteRun
//
// # Delete a run
//
// Removes one of your runs with everything logged in it.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Deleted
//	  401: unauthenticated
//	  404: notFound
func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.DeleteRun(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	switch {
	case errors.Is(err, db.ErrNoRun):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}
	sets := make([]db.SetFields, 0, len(req.Sets))
	for _, set := range req.Sets {
		sets = append(sets, db.SetFields{Reps: set.Reps, Seconds: set.Seconds, WeightKG: set.WeightKG})
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
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
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
	switch {
	case errors.Is(err, db.ErrNoClimb):
		writeNotFound(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		Outcome:     req.Outcome,
		Attempts:    req.Attempts,
		Seconds:     req.Seconds,
		Stars:       req.Stars,
		Focus:       req.Focus,
		Notes:       req.Notes,
	}
}

func toClimbBody(c db.Climb) climbBody {
	return climbBody{
		ID: c.ID,
		climbRequest: climbRequest{
			Step:        c.Step,
			Position:    c.Position,
			Discipline:  c.Discipline,
			Setting:     c.Setting,
			Board:       c.Board,
			RopeStyle:   c.RopeStyle,
			Grade:       c.Grade,
			GradeSystem: c.GradeSystem,
			Outcome:     c.Outcome,
			Attempts:    c.Attempts,
			Seconds:     c.Seconds,
			Stars:       c.Stars,
			Focus:       c.Focus,
			Notes:       c.Notes,
		},
		Sent: c.Sent(),
	}
}

func toSetBodies(sets []db.Set) []setBody {
	out := make([]setBody, 0, len(sets))
	for _, set := range sets {
		out = append(out, setBody{
			Step:          set.Step,
			Number:        set.Number,
			Exercise:      set.Exercise,
			setFieldsBody: setFieldsBody{Reps: set.Reps, Seconds: set.Seconds, WeightKG: set.WeightKG},
		})
	}
	return out
}

// parseDay reads a date written YYYY-MM-DD, and records a problem under key.
func parseDay(s, key string, problems map[string]string) (time.Time, bool) {
	day, err := time.Parse(time.DateOnly, strings.TrimSpace(s))
	if err != nil {
		problems[key] = "must be a date, written 2026-03-07"
		return time.Time{}, false
	}
	return day, true
}

func (req runRequest) fields() db.RunFields {
	return db.RunFields{
		Name:           req.Name,
		Body:           toRunBody(req.Sections),
		ElapsedSeconds: req.ElapsedSeconds,
		Place:          req.Place,
		Notes:          req.Notes,
		Journal: db.Journal{
			Sleep:     req.Sleep,
			Energy:    req.Energy,
			RPE:       req.RPE,
			Focus:     req.Focus,
			Setting:   req.Setting,
			WentWell:  req.WentWell,
			NextFocus: req.NextFocus,
		},
	}
}

func toRunBody(sections []runSectionBody) db.RunBody {
	out := db.RunBody{Sections: make([]db.RunSection, 0, len(sections))}
	for _, s := range sections {
		items := make([]db.RunItem, 0, len(s.Items))
		for _, item := range s.Items {
			var next db.RunItem
			if item.Step != nil {
				step := item.Step.runStep()
				next.Step = &step
			}
			if item.Choice != nil {
				choice := item.Choice.runChoice()
				next.Choice = &choice
			}
			items = append(items, next)
		}
		out.Sections = append(out.Sections, db.RunSection{Name: s.Name, Notes: s.Notes, Items: items})
	}
	return out
}

func (s runStepBody) runStep() db.RunStep {
	step := db.RunStep{
		ID:             s.ID,
		Step:           s.step(),
		Status:         s.Status,
		RunNotes:       s.RunNotes,
		ElapsedSeconds: s.ElapsedSeconds,
	}
	if s.FromChoice != nil {
		choice := s.FromChoice.runChoice()
		step.FromChoice = &choice
	}
	return step
}

func (c runChoiceBody) runChoice() db.RunChoice {
	return db.RunChoice{ID: c.ID, Choice: c.choice()}
}

func toRunChoiceBody(c db.RunChoice) runChoiceBody {
	return runChoiceBody{ID: c.ID, choiceBody: toChoiceBody(c.Choice)}
}

func toRunSectionBodies(b db.RunBody) []runSectionBody {
	sections := make([]runSectionBody, 0, len(b.Sections))
	for _, s := range b.Sections {
		items := make([]runItemBody, 0, len(s.Items))
		for _, item := range s.Items {
			var out runItemBody
			if st := item.Step; st != nil {
				step := runStepBody{
					ID:             st.ID,
					stepBody:       toStepBody(st.Step),
					Status:         st.Status,
					RunNotes:       st.RunNotes,
					ElapsedSeconds: st.ElapsedSeconds,
				}
				if st.FromChoice != nil {
					choice := toRunChoiceBody(*st.FromChoice)
					step.FromChoice = &choice
				}
				out.Step = &step
			}
			if item.Choice != nil {
				choice := toRunChoiceBody(*item.Choice)
				out.Choice = &choice
			}
			items = append(items, out)
		}
		sections = append(sections, runSectionBody{Name: s.Name, Notes: s.Notes, Items: items})
	}
	return sections
}

func toRunResponse(run db.Run) runResponse {
	var plan []sectionBody
	if run.Plan != nil {
		plan = toSectionBodies(*run.Plan)
	}
	j := run.Journal
	return runResponse{
		ID:             run.ID,
		Template:       run.Template,
		Name:           run.Name,
		Plan:           plan,
		Sections:       toRunSectionBodies(run.Body),
		StartedAt:      run.StartedAt,
		Timezone:       run.Timezone,
		LocalDate:      run.LocalDate.Format(time.DateOnly),
		FinishedAt:     run.FinishedAt,
		ElapsedSeconds: run.ElapsedSeconds,
		Place:          run.Place,
		Notes:          run.Notes,
		journalBody: journalBody{
			Sleep:     j.Sleep,
			Energy:    j.Energy,
			RPE:       j.RPE,
			Focus:     j.Focus,
			Setting:   j.Setting,
			WentWell:  j.WentWell,
			NextFocus: j.NextFocus,
		},
		Sets:   toSetBodies(run.Sets),
		Climbs: toClimbBodies(run.Climbs),
	}
}

func toClimbBodies(climbs []db.Climb) []climbBody {
	out := make([]climbBody, 0, len(climbs))
	for _, c := range climbs {
		out = append(out, toClimbBody(c))
	}
	return out
}
