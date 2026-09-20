package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"passion/server/db/dbtest"
)

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux(dbtest.Pool(t)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthzReportsALostDatabase(t *testing.T) {
	pool := dbtest.Pool(t)
	pool.Close()

	rec := httptest.NewRecorder()
	newMux(pool).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestUnknownPathIs404(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux(dbtest.Pool(t)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
