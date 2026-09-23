package api

import (
	"embed"
	"io/fs"
	"net/http"
)

// The spec is generated from the annotations below by `make openapi`, and CI
// fails when the committed copy is out of date.
//
//go:embed swagger.json
var swaggerJSON []byte

// The docs page and the Scalar bundle it loads. Both are served from this
// binary so an install with no internet still gets browsable, runnable docs.
//
//go:embed docs
var docsFS embed.FS

// docsHandler serves the page at /api/docs and its script beside it.
func docsHandler() http.Handler {
	docs, err := fs.Sub(docsFS, "docs")
	if err != nil {
		// The directory is embedded at compile time, so this cannot fail.
		panic(err)
	}
	return http.StripPrefix("/api/docs", http.FileServer(http.FS(docs)))
}

func (s *Server) openapi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(swaggerJSON)
}

// Passion
//
// Training app for climbers. Sign up, sign in on several devices at once, and
// sign each one out on its own.
//
// Every failure answers with the same shape: an error object carrying a code,
// a human message, and on a validation failure a fields map naming what to
// fix.
//
//	Version: 1
//	BasePath: /
//	Schemes: http, https
//	Consumes:
//	- application/json
//	Produces:
//	- application/json
//
//	SecurityDefinitions:
//	  bearer:
//	    type: apiKey
//	    name: Authorization
//	    in: header
//	    description: >-
//	      The token from a sign-up or sign-in response, sent as
//	      "Bearer <token>". It lasts thirty days, unless the server sets another
//	      length, and slides forward on use.
//
// swagger:meta
type swaggerMeta struct{}

// The body of a sign-up.
// swagger:parameters signUp
type signUpParams struct {
	// in:body
	// required:true
	Body signUpRequest `json:"body"`
}

// The body of a sign-in.
// swagger:parameters signIn
type signInParams struct {
	// in:body
	// required:true
	Body signInRequest `json:"body"`
}

// The account and its first token.
// swagger:response signUpResponse
type signUpResponseWrapper struct {
	// in:body
	Body signUpResponse
}

// A new token.
// swagger:response tokenResponse
type tokenResponseWrapper struct {
	// in:body
	Body tokenResponse
}

// An account.
// swagger:response accountResponse
type accountResponseWrapper struct {
	// in:body
	Body accountResponse
}

// The address and password do not match an account.
// swagger:response invalidCredentials
type invalidCredentialsWrapper struct {
	// in:body
	Body errorBody
}

// No live bearer token.
// swagger:response unauthenticated
type unauthenticatedWrapper struct {
	// in:body
	Body errorBody
}

// Something in the body needs fixing.
// swagger:response validationFailed
type validationFailedWrapper struct {
	// in:body
	Body errorBody
}

// Nothing you can see or change has that id.
// swagger:response notFound
type notFoundWrapper struct {
	// in:body
	Body errorBody
}

// The body of a new or replaced exercise.
// swagger:parameters createExercise updateExercise
type exerciseBodyParams struct {
	// in:body
	// required:true
	Body exerciseRequest `json:"body"`
}

// swagger:parameters readExercise updateExercise retireExercise
type exerciseIDParams struct {
	// The exercise's id.
	//
	// in: path
	// required: true
	ID string `json:"id"`
}

// An exercise.
// swagger:response exerciseResponse
type exerciseResponseWrapper struct {
	// in:body
	Body exerciseResponse
}

// Your library.
// swagger:response exerciseListResponse
type exerciseListResponseWrapper struct {
	// in:body
	Body exerciseListResponse
}

// The body of a new or replaced session template.
// swagger:parameters createSessionTemplate updateSessionTemplate
type sessionTemplateBodyParams struct {
	// in:body
	// required:true
	Body sessionTemplateRequest `json:"body"`
}

// swagger:parameters readSessionTemplate updateSessionTemplate retireSessionTemplate
type sessionTemplateIDParams struct {
	// The session template's id.
	//
	// in: path
	// required: true
	ID string `json:"id"`
}

// The body of a new run.
// swagger:parameters startRun
type runStartParams struct {
	// in:body
	// required:true
	Body runStartRequest `json:"body"`
}

// The body of a replaced run.
// swagger:parameters updateRun
type runBodyParams struct {
	// in:body
	// required:true
	Body runRequest `json:"body"`
}

// swagger:parameters readRun updateRun finishRun deleteRun replaceSets putClimb deleteClimb
type runIDParams struct {
	// The run's id.
	//
	// in: path
	// required: true
	ID string `json:"id"`
}

// swagger:parameters replaceSets
type setParams struct {
	// The step's id in the run's body.
	//
	// in: path
	// required: true
	Step string `json:"step"`

	// in:body
	// required:true
	Body setRequest `json:"body"`
}

// swagger:parameters setGrades
type gradesParams struct {
	// in:body
	// required:true
	Body gradesBody `json:"body"`
}

// swagger:parameters putClimb deleteClimb
type climbIDParams struct {
	// The id the client chose for the climb.
	//
	// in: path
	// required: true
	Climb string `json:"climb"`
}

// swagger:parameters putClimb
type climbParams struct {
	// in:body
	// required:true
	Body climbRequest `json:"body"`
}

// swagger:parameters readCycle putCycle deleteCycle
type cycleIDParams struct {
	// The cycle's id. The client chooses it when it creates the cycle.
	//
	// in: path
	// required: true
	ID string `json:"id"`
}

// swagger:parameters putCycle
type cycleParams struct {
	// in:body
	// required:true
	Body cycleRequest `json:"body"`
}

// swagger:parameters listScheduledSessions
type scheduleRangeParams struct {
	// The first day, written YYYY-MM-DD.
	//
	// in: query
	// required: true
	From string `json:"from"`

	// The last day, included.
	//
	// in: query
	// required: true
	To string `json:"to"`
}

// swagger:parameters scheduleSession
type scheduleParams struct {
	// in:body
	// required:true
	Body scheduleRequest `json:"body"`
}

// swagger:parameters moveScheduledSession deleteScheduledSession
type scheduledSessionIDParams struct {
	// in: path
	// required: true
	ID string `json:"id"`
}

// swagger:parameters moveScheduledSession
type moveParams struct {
	// in:body
	// required:true
	Body moveRequest `json:"body"`
}
