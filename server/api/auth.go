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

type signUpRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
}

type accountResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
}

type tokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type signUpResponse struct {
	Account accountResponse `json:"account"`
	Token   tokenResponse   `json:"token"`
}

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
