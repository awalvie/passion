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
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

// Reads say "mine, plus the catalog". Writes say "mine only". That asymmetry is the whole
// design, and it needs a guard rather than a wider filter: GORM's Save writes whatever row
// it is handed, and a widened read now hands a handler a catalog row.
//
// Before this, the write was scoped to owner_id and simply matched nothing. The user
// pressed Delete on a catalog row, got a success redirect, and found the row still there
// on reload. Silent, and the same shape as the empty-children bug on the read side.

func guardTestServer(t *testing.T, name string) (*Server, *db.Store) {
	t.Helper()
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(store, "test-secret-at-least-32-characters!!", time.Hour, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv, store
}

func postAs(t *testing.T, ownerID uint, path string, params map[string]string, urlParams map[string]string) *http.Request {
	t.Helper()
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	for k, v := range urlParams {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	return req.WithContext(ctx)
}

// A catalog row must survive every write path, and the user must be told why.
func TestCatalogRowsRefuseEveryWrite(t *testing.T) {
	const app, user uint = 1, 2

	cases := []struct {
		name  string
		seed  func(t *testing.T, store *db.Store) uint
		call  func(t *testing.T, srv *Server, id uint) *httptest.ResponseRecorder
		alive func(t *testing.T, store *db.Store, id uint) bool
	}{
		{
			name: "session template edit",
			seed: func(t *testing.T, store *db.Store) uint {
				row := db.SessionTemplate{OwnerID: app, Name: "Boulder Session", Slug: "boulder", Shared: true}
				mustCreateWeb(t, store, &row)
				return row.ID
			},
			call: func(t *testing.T, srv *Server, id uint) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleTemplatesByID(rr, postAs(t, user, fmt.Sprintf("/templates/%d/update", id),
					map[string]string{"name": "Hijacked"},
					map[string]string{"templateID": fmt.Sprint(id), "action": "update"}))
				return rr
			},
			alive: func(t *testing.T, store *db.Store, id uint) bool {
				var row db.SessionTemplate
				return store.DB.Where("id = ?", id).First(&row).Error == nil && row.Name == "Boulder Session"
			},
		},
		{
			name: "session template delete",
			seed: func(t *testing.T, store *db.Store) uint {
				row := db.SessionTemplate{OwnerID: app, Name: "Boulder Session", Slug: "boulder2", Shared: true}
				mustCreateWeb(t, store, &row)
				return row.ID
			},
			call: func(t *testing.T, srv *Server, id uint) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleTemplatesByID(rr, postAs(t, user, fmt.Sprintf("/templates/%d/delete", id), nil,
					map[string]string{"templateID": fmt.Sprint(id), "action": "delete"}))
				return rr
			},
			alive: func(t *testing.T, store *db.Store, id uint) bool {
				var n int64
				store.DB.Model(&db.SessionTemplate{}).Where("id = ?", id).Count(&n)
				return n == 1
			},
		},
		{
			name: "activity template delete",
			seed: func(t *testing.T, store *db.Store) uint {
				row := db.ActivityTemplate{OwnerID: app, Name: "Warm Up", Slug: "warm_up", Shared: true}
				mustCreateWeb(t, store, &row)
				return row.ID
			},
			call: func(t *testing.T, srv *Server, id uint) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleActivityTemplatesByID(rr, postAs(t, user, fmt.Sprintf("/activity-templates/%d/delete", id), nil,
					map[string]string{"activityTemplateID": fmt.Sprint(id), "action": "delete"}))
				return rr
			},
			alive: func(t *testing.T, store *db.Store, id uint) bool {
				var n int64
				store.DB.Model(&db.ActivityTemplate{}).Where("id = ?", id).Count(&n)
				return n == 1
			},
		},
		{
			name: "library exercise delete",
			seed: func(t *testing.T, store *db.Store) uint {
				row := db.LibraryExercise{OwnerID: app, Name: "Max Hangs", Slug: "max_hangs", Shared: true}
				mustCreateWeb(t, store, &row)
				return row.ID
			},
			call: func(t *testing.T, srv *Server, id uint) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleExerciseLibraryByID(rr, postAs(t, user, fmt.Sprintf("/exercise-library/%d/delete", id), nil,
					map[string]string{"libraryExerciseID": fmt.Sprint(id), "action": "delete"}))
				return rr
			},
			alive: func(t *testing.T, store *db.Store, id uint) bool {
				var n int64
				store.DB.Model(&db.LibraryExercise{}).Where("id = ?", id).Count(&n)
				return n == 1
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, store := guardTestServer(t, "guard-"+strings.ReplaceAll(tc.name, " ", "-")+".db")
			id := tc.seed(t, store)

			rr := tc.call(t, srv, id)

			if !tc.alive(t, store, id) {
				t.Fatal("the catalog row was changed or removed")
			}
			// A success redirect is the failure this exists to stop: it tells the user the
			// write worked when nothing happened.
			if rr.Header().Get("HX-Redirect") != "" {
				t.Errorf("the handler reported success, status %d", rr.Code)
			}
			if rr.Code < 400 {
				t.Errorf("want a client error, got %d", rr.Code)
			}
			if !strings.Contains(rr.Body.String(), "shared catalog") {
				t.Errorf("the message should say the item is shared, got %q", strings.TrimSpace(rr.Body.String()))
			}
		})
	}
}

// The guard must not stop anyone editing their own rows.
func TestOwnRowsAreStillWritable(t *testing.T) {
	srv, store := guardTestServer(t, "guard-own.db")
	const user uint = 2

	row := db.SessionTemplate{OwnerID: user, Name: "My Session", Slug: "mine"}
	mustCreateWeb(t, store, &row)

	rr := httptest.NewRecorder()
	srv.handleTemplatesByID(rr, postAs(t, user, fmt.Sprintf("/templates/%d/update", row.ID),
		map[string]string{"name": "My Renamed Session"},
		map[string]string{"templateID": fmt.Sprint(row.ID), "action": "update"}))

	var after db.SessionTemplate
	if err := store.DB.Where("id = ?", row.ID).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Name != "My Renamed Session" {
		t.Fatalf("the user could not rename their own template, name is %q (status %d)", after.Name, rr.Code)
	}
}

func mustCreateWeb(t *testing.T, store *db.Store, v any) {
	t.Helper()
	if err := store.DB.Create(v).Error; err != nil {
		t.Fatal(err)
	}
}
