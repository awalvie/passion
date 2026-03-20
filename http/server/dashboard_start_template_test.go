package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"passion/db"
)

func TestHandleDashboardStartFromTemplateMethodNotAllowed(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/start-template", nil)
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, uint(7)))
	rr := httptest.NewRecorder()

	s.handleDashboardStartFromTemplate(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleDashboardStartFromTemplateBadTemplateID(t *testing.T) {
	s := &Server{}
	form := url.Values{"template_id": {"abc"}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/start-template", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, uint(7)))
	rr := httptest.NewRecorder()

	s.handleDashboardStartFromTemplate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleDashboardStartFromTemplateStartsRun(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dashboard-start-template.db")
	store, err := db.NewSqlite(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	ownerID := uint(7)
	tpl := &db.SessionTemplate{
		OwnerID: ownerID,
		Name:    "Leg Day",
	}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatal(err)
	}

	s := &Server{store: store}
	form := url.Values{"template_id": {fmt.Sprintf("%d", tpl.ID)}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/start-template", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, ownerID))
	rr := httptest.NewRecorder()

	s.handleDashboardStartFromTemplate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	if redirect := rr.Header().Get("HX-Redirect"); !strings.HasPrefix(redirect, "/runs/") {
		t.Fatalf("HX-Redirect = %q, want prefix /runs/", redirect)
	}

	var scheduledCount int64
	if err := store.DB.Model(&db.ScheduledSession{}).
		Where("owner_id = ? AND is_trial = ?", ownerID, true).
		Count(&scheduledCount).Error; err != nil {
		t.Fatal(err)
	}
	if scheduledCount != 1 {
		t.Fatalf("trial scheduled sessions = %d, want 1", scheduledCount)
	}

	var run db.SessionRun
	if err := store.DB.
		Where("owner_id = ? AND is_trial = ?", ownerID, true).
		First(&run).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "running" {
		t.Fatalf("run status = %q, want running", run.Status)
	}
}
