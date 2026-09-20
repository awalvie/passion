package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"passion/server/db"
	"passion/server/password"
	"passion/server/token"
)

const maxDisplayName = 64

// swagger:model signUpRequest
type signUpRequest struct {
	// required: true
	// example: ada@example.com
	Email string `json:"email"`

	// Between 8 and 128 characters.
	//
	// required: true
	// example: correct horse battery
	Password string `json:"password"`

	// The name shown in the app.
	//
	// required: true
	// example: Ada
	DisplayName string `json:"display_name"`

	// A zone name Postgres knows. It decides which local date a record counts
	// as, so a session finished before midnight stays on that day.
	//
	// required: true
	// example: Europe/Oslo
	Timezone string `json:"timezone"`
}

// swagger:model accountResponse
type accountResponse struct {
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	// example: ada@example.com
	Email string `json:"email"`

	// example: Ada
	DisplayName string `json:"display_name"`

	// example: Europe/Oslo
	Timezone string `json:"timezone"`
}

// swagger:model tokenResponse
type tokenResponse struct {
	// Send as "Authorization: Bearer <token>". Shown once and never again.
	Token string `json:"token"`

	// Thirty days out. Using the token pushes this forward.
	ExpiresAt time.Time `json:"expires_at"`
}

// swagger:model signUpResponse
type signUpResponse struct {
	Account accountResponse `json:"account"`
	Token   tokenResponse   `json:"token"`
}

// swagger:route POST /api/v1/accounts accounts signUp
//
// # Create an account
//
// Answers with the account and a token, so the client signs up and signs in
// with one call.
//
// An address that is already registered answers invalid_credentials, the same
// as a wrong password. Telling them apart would let anyone ask the API who has
// an account.
//
//	Security: []
//	Responses:
//	  201: signUpResponse
//	  401: invalidCredentials
//	  422: validationFailed
func (s *Server) signUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if fields := validateSignUp(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	// Hashing happens before the insert, so a taken address and a free one
	// take the same time. Otherwise the clock answers "does this person have
	// an account" however careful the response is.
	hash, err := s.hash(func() (string, error) { return password.Hash(req.Password) })
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}

	account, err := db.CreateAccount(r.Context(), s.pool, req.Email, hash, req.DisplayName, req.Timezone)
	switch {
	case errors.Is(err, db.ErrEmailTaken):
		writeInvalidCredentials(w)
		return
	case errors.Is(err, db.ErrUnknownZone):
		writeFieldErrors(w, map[string]string{"timezone": "is not a time zone this server knows"})
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}

	issued, err := s.issueToken(r, account.ID)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}

	writeJSON(w, http.StatusCreated, signUpResponse{
		Account: toAccountResponse(account),
		Token:   issued,
	})
}

func (s *Server) issueToken(r *http.Request, accountID string) (tokenResponse, error) {
	raw, hash, err := token.New()
	if err != nil {
		return tokenResponse{}, err
	}

	expires := time.Now().Add(tokenLife)
	if _, err := db.CreateAuthToken(r.Context(), s.pool, accountID, hash, expires); err != nil {
		return tokenResponse{}, err
	}
	return tokenResponse{Token: raw, ExpiresAt: expires}, nil
}

func toAccountResponse(a db.Account) accountResponse {
	return accountResponse{
		ID:          a.ID,
		Email:       a.Email,
		DisplayName: a.DisplayName,
		Timezone:    a.Timezone,
	}
}

func validateSignUp(req signUpRequest) map[string]string {
	fields := map[string]string{}

	if _, err := mail.ParseAddress(strings.TrimSpace(req.Email)); err != nil {
		fields["email"] = "is not an email address"
	}

	switch {
	case len(req.Password) < password.MinLength:
		fields["password"] = "must be at least 8 characters"
	case len(req.Password) > password.MaxLength:
		fields["password"] = "must be at most 128 characters"
	}

	switch {
	case req.DisplayName == "":
		fields["display_name"] = "is required"
	case len(req.DisplayName) > maxDisplayName:
		fields["display_name"] = "is too long"
	}

	if req.Timezone == "" {
		fields["timezone"] = "is required"
	}

	return fields
}

// swagger:model signInRequest
type signInRequest struct {
	// required: true
	// example: ada@example.com
	Email string `json:"email"`

	// required: true
	// example: correct horse battery
	Password string `json:"password"`
}

// unknownAddressHash is a real hash of a value nobody has. Verifying against
// it makes an unknown address cost the same as a wrong password, so the clock
// does not answer "does this person have an account".
var unknownAddressHash = func() string {
	h, err := password.Hash("no account has this password")
	if err != nil {
		panic(err)
	}
	return h
}()

// swagger:route POST /api/v1/tokens tokens signIn
//
// # Sign in
//
// Each sign-in mints a separate token, so a phone and a laptop hold different
// ones and signing out of either leaves the other working.
//
//	Security: []
//	Responses:
//	  201: tokenResponse
//	  401: invalidCredentials
//	  422: validationFailed
func (s *Server) signIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeFieldErrors(w, map[string]string{"body": "could not be read as JSON"})
		return
	}

	account, err := db.AccountByEmail(r.Context(), s.pool, req.Email)
	switch {
	case errors.Is(err, db.ErrNoAccount):
		s.verify(unknownAddressHash, req.Password)
		writeInvalidCredentials(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}

	err = s.verify(account.PasswordHash, req.Password)
	switch {
	case errors.Is(err, password.ErrMismatch):
		writeInvalidCredentials(w)
		return
	case err != nil:
		writeInternal(w, s.log, err)
		return
	}

	issued, err := s.issueToken(r, account.ID)
	if err != nil {
		writeInternal(w, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, issued)
}

// swagger:route GET /api/v1/accounts/me accounts readAccount
//
// Read the signed-in account
//
//	Security:
//	  bearer:
//	Responses:
//	  200: accountResponse
//	  401: unauthenticated
func (s *Server) me(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	writeJSON(w, http.StatusOK, accountResponse{
		ID:          who.AccountID,
		Email:       who.Email,
		DisplayName: who.DisplayName,
		Timezone:    who.Timezone,
	})
}

// signOut deletes the token that authenticated this request, so it signs out
// the one device and leaves the others alone.
// swagger:route DELETE /api/v1/tokens/current tokens signOut
//
// # Sign out this device
//
// Deletes the token that authenticated the request. Signing out twice with the
// same token answers 401 the second time, because the route authenticates
// before it deletes.
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Signed out
//	  401: unauthenticated
func (s *Server) signOut(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	if err := db.DeleteAuthToken(r.Context(), s.pool, who.TokenID); err != nil {
		writeInternal(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// verify runs one comparison, waiting for a free hashing slot first.
func (s *Server) verify(encoded, plain string) error {
	s.hashing <- struct{}{}
	defer func() { <-s.hashing }()
	return password.Verify(encoded, plain)
}
