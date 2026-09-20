package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the error shape: %v (%s)", err, rec.Body)
	}
	return body
}

func TestEveryErrorCarriesTheSameShape(t *testing.T) {
	cases := map[string]struct {
		write  func(w http.ResponseWriter)
		code   string
		status int
	}{
		"validation": {
			func(w http.ResponseWriter) { writeFieldErrors(w, map[string]string{"password": "too short"}) },
			CodeValidationFailed, http.StatusUnprocessableEntity,
		},
		"credentials":     {writeInvalidCredentials, CodeInvalidCredentials, http.StatusUnauthorized},
		"unauthenticated": {writeUnauthenticated, CodeUnauthenticated, http.StatusUnauthorized},
		"internal": {
			func(w http.ResponseWriter) {
				writeInternal(w, slog.New(slog.NewTextHandler(io.Discard, nil)), io.EOF)
			},
			CodeInternal, http.StatusInternalServerError,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c.write(rec)

			if rec.Code != c.status {
				t.Fatalf("status %d, want %d", rec.Code, c.status)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("content type %q", got)
			}

			body := decodeBody(t, rec)
			if body.Error.Code != c.code {
				t.Fatalf("code %q, want %q", body.Error.Code, c.code)
			}
			if body.Error.Message == "" {
				t.Fatal("no message")
			}
		})
	}
}

func TestOnlyValidationCarriesFields(t *testing.T) {
	rec := httptest.NewRecorder()
	writeInvalidCredentials(rec)
	if decodeBody(t, rec).Error.Fields != nil {
		t.Fatal("a non-validation error carried fields")
	}

	rec = httptest.NewRecorder()
	writeFieldErrors(rec, map[string]string{"email": "already in use"})
	if got := decodeBody(t, rec).Error.Fields["email"]; got != "already in use" {
		t.Fatalf("fields did not survive: %q", got)
	}
}

// The reason belongs in the log, never in the response.
func TestInternalHidesTheReason(t *testing.T) {
	var logged strings.Builder
	rec := httptest.NewRecorder()

	writeInternal(rec, slog.New(slog.NewTextHandler(&logged, nil)), io.ErrUnexpectedEOF)

	if strings.Contains(rec.Body.String(), "unexpected EOF") {
		t.Fatal("the response leaked the reason")
	}
	if !strings.Contains(logged.String(), "unexpected EOF") {
		t.Fatal("the log did not record the reason")
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Email string `json:"email"`
	}

	cases := map[string]struct {
		body string
		ok   bool
	}{
		"an object":        {`{"email":"ada@example.com"}`, true},
		"an unknown field": {`{"email":"a@b.c","admin":true}`, false},
		"two values":       {`{"email":"a@b.c"}{"email":"x@y.z"}`, false},
		"not json":         {`hunter2`, false},
		"empty":            {``, false},
		"too big":          {`{"email":"` + strings.Repeat("a", maxBodyBytes) + `"}`, false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(c.body))
			err := decodeJSON(httptest.NewRecorder(), r, &payload{})

			if c.ok && err != nil {
				t.Fatalf("rejected a good body: %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("accepted a bad body")
			}
		})
	}
}
