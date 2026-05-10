package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"passion/db"
)

func withRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})
}

func TestHandleProfileGet(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "profile-get.db"))
	if err != nil {
		t.Fatal(err)
	}
	user := &db.User{Email: "climber@example.com", PasswordHash: "hash"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, user.ID))
	rr := httptest.NewRecorder()

	srv.handleProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Your statistics") {
		t.Fatalf("profile page did not render expected content: %q", rr.Body.String())
	}
}

func TestHandleProfilePostUpdatesStats(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "profile-post.db"))
	if err != nil {
		t.Fatal(err)
	}
	user := &db.User{Email: "climber@example.com", PasswordHash: "hash"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{
		"height_cm":     {"178"},
		"weight_kg":     {"70.5"},
		"ape_index_cm":  {"4"},
		"max_pull_ups":  {"18"},
		"max_hang_kg":   {"32.5"},
		"boulder_grade": {"7A"},
		"route_grade":   {"7b+"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, user.ID))
	rr := httptest.NewRecorder()

	srv.handleProfile(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	var updated db.User
	if err := store.DB.First(&updated, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.HeightCm != 178 || updated.WeightKg != 70.5 || updated.ApeIndexCm != 4 || updated.MaxPullUps != 18 || updated.MaxHangKg != 32.5 {
		t.Fatalf("updated stats mismatch: %+v", updated)
	}
	if updated.BoulderGrade != "7A" || updated.RouteGrade != "7b+" {
		t.Fatalf("updated grades mismatch: %+v", updated)
	}
}

func TestHandleProfilePostBadNumberShowsError(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "profile-bad.db"))
	if err != nil {
		t.Fatal(err)
	}
	user := &db.User{Email: "climber@example.com", PasswordHash: "hash"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"weight_kg": {"heavy"}}
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, user.ID))
	rr := httptest.NewRecorder()

	srv.handleProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Weight must be a number.") {
		t.Fatalf("expected validation error, got %q", rr.Body.String())
	}
}
