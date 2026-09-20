// Package api serves the JSON API. It writes no SQL, and db writes no JSON.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// Every failure the API returns carries this shape.
//
//	{ "error": { "code": "...", "message": "...", "fields": { ... } } }
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

const (
	CodeValidationFailed   = "validation_failed"
	CodeInvalidCredentials = "invalid_credentials"
	CodeUnauthenticated    = "unauthenticated"
	CodeNotFound           = "not_found"
	CodeInternal           = "internal"
)

// A body big enough for any request this API takes, and small enough that
// nobody can make the server hash a megabyte.
const maxBodyBytes = 64 << 10

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{errorDetail{Code: code, Message: message}})
}

// writeFieldErrors names what the person must fix.
func writeFieldErrors(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity, errorBody{errorDetail{
		Code:    CodeValidationFailed,
		Message: "Some of what you typed needs fixing.",
		Fields:  fields,
	}})
}

// writeInvalidCredentials is the one answer to a wrong password, an unknown
// address, and an address already registered. Telling them apart would let
// anyone ask the API who has an account.
func writeInvalidCredentials(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, CodeInvalidCredentials,
		"That email address and password do not match an account.")
}

func writeUnauthenticated(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "Sign in first.")
}

func writeNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, CodeNotFound, "There is nothing at that address.")
}

// writeInternal keeps the reason in the log and out of the response.
func writeInternal(w http.ResponseWriter, log *slog.Logger, err error) {
	log.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, CodeInternal, "Something went wrong.")
}

// decodeJSON reads one JSON object, capped, and refuses anything it does not
// recognise so a typo in a field name is an error rather than silence.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("body carries more than one json value")
	}
	return nil
}
