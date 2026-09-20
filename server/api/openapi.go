package api

import (
	_ "embed"
	"net/http"
)

// The spec is generated from the annotations below by `make openapi`, and CI
// fails when the committed copy is out of date.
//
//go:embed swagger.json
var swaggerJSON []byte

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
//	      "Bearer <token>". It lasts thirty days and slides forward on use.
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
