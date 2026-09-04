package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

// catalogIDs are the rows every case in the table below starts from: one catalog session
// with one activity holding one exercise, one catalog block holding one exercise, and one
// catalog library exercise. All are flagged ManagedByCatalog with no edit stamp.
type catalogIDs struct {
	sessionTemplateID  uint
	activityID         uint
	activityExerciseID uint
	blockID            uint
	blockExerciseID    uint
	libraryExerciseID  uint
}

func newCatalogFixture(t *testing.T) (*Server, *db.Store, catalogIDs) {
	t.Helper()
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "catalog-edited.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&db.User{Email: "e@example.com", PasswordHash: "h"}).Error; err != nil {
		t.Fatal(err)
	}

	st := db.SessionTemplate{OwnerID: testOwnerID, Name: "Catalog Session", ManagedByCatalog: true}
	if err := store.DB.Create(&st).Error; err != nil {
		t.Fatal(err)
	}
	act := db.Activity{OwnerID: testOwnerID, SessionTemplateID: st.ID, Type: "activity", OrderIndex: 1}
	if err := store.DB.Create(&act).Error; err != nil {
		t.Fatal(err)
	}
	actEx := db.Exercise{OwnerID: testOwnerID, ActivityID: &act.ID, Name: "Session Row", Kind: "reps_and_sets", Sets: 3, Reps: 5, OrderIndex: 1}
	if err := store.DB.Create(&actEx).Error; err != nil {
		t.Fatal(err)
	}

	at := db.ActivityTemplate{OwnerID: testOwnerID, Name: "Catalog Block", ManagedByCatalog: true}
	if err := store.DB.Create(&at).Error; err != nil {
		t.Fatal(err)
	}
	atEx := db.Exercise{OwnerID: testOwnerID, ActivityTemplateID: &at.ID, Name: "Block Row", Kind: "reps_and_sets", Sets: 3, Reps: 5, OrderIndex: 1}
	if err := store.DB.Create(&atEx).Error; err != nil {
		t.Fatal(err)
	}

	lib := db.LibraryExercise{OwnerID: testOwnerID, Name: "Catalog Exercise", Kind: "reps_and_sets", Sets: 3, Reps: 5, ManagedByCatalog: true}
	if err := store.DB.Create(&lib).Error; err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	return srv, store, catalogIDs{
		sessionTemplateID:  st.ID,
		activityID:         act.ID,
		activityExerciseID: actEx.ID,
		blockID:            at.ID,
		blockExerciseID:    atEx.ID,
		libraryExerciseID:  lib.ID,
	}
}

const testOwnerID uint = 1

// postForm builds an authenticated POST carrying form values, plus any chi URL params the
// handler reads itself.
func postForm(values url.Values, chiParams map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, testOwnerID))
	if len(chiParams) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range chiParams {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	return req
}

func uintStr(v uint) string { return strconv.FormatUint(uint64(v), 10) }

// target names which model the handler is expected to stamp.
type target int

const (
	targetSession target = iota
	targetBlock
	targetLibrary
)

// TestEveryCatalogMutationStampsTheRow is the guard the Edited flag depends on. The
// importer overwrites the row it matches by name and recreates a block's or session's
// child rows, so a mutating handler that forgets to stamp CatalogEditedAt leaves that
// particular edit still being silently reverted on the next restart.
//
// One case per mutating route. A new route that touches a catalog row belongs in this
// table — an unlisted route is a gap, and the point of the table is that the gap is
// visible next to the route list in core.go.
func TestEveryCatalogMutationStampsTheRow(t *testing.T) {
	cases := []struct {
		name   string
		want   target
		invoke func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder
	}{
		{
			name: "session template update",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleUpdateSessionTemplate(rr, postForm(url.Values{"name": {"Renamed"}}, nil), ids.sessionTemplateID, testOwnerID)
				return rr
			},
		},
		{
			name: "session activity added",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleAddActivity(rr, postForm(url.Values{"type": {"cooldown"}}, nil), ids.sessionTemplateID, testOwnerID)
				return rr
			},
		},
		{
			name: "session activity added from a block",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleAddActivityFromTemplate(rr, postForm(url.Values{"activity_template_id": {uintStr(ids.blockID)}}, nil), ids.sessionTemplateID, testOwnerID)
				return rr
			},
		},
		{
			name: "session activities reordered",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleReorderActivities(rr, postForm(url.Values{"ordered_ids": {uintStr(ids.activityID)}}, nil), ids.sessionTemplateID, testOwnerID)
				return rr
			},
		},
		{
			name: "session activity deleted",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleDeleteActivity(rr, postForm(nil, nil), ids.activityID, testOwnerID)
				return rr
			},
		},
		{
			name: "exercise added to a session activity",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleAddExercise(rr, postForm(url.Values{"name": {"New Row"}, "kind": {"reps_and_sets"}, "sets": {"3"}, "reps": {"5"}}, nil), ids.activityID, testOwnerID)
				return rr
			},
		},
		{
			name: "exercise added to a session activity from the library",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleAddExerciseFromLibrary(rr, postForm(url.Values{"library_exercise_id": {uintStr(ids.libraryExerciseID)}}, nil), ids.activityID, testOwnerID)
				return rr
			},
		},
		{
			name: "session activity exercises reordered",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleReorderExercises(rr, postForm(url.Values{"ordered_ids": {uintStr(ids.activityExerciseID)}}, nil), ids.activityID, testOwnerID)
				return rr
			},
		},
		{
			name: "session exercise updated",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleUpdateExercise(rr, postForm(url.Values{"name": {"Edited Row"}, "kind": {"reps_and_sets"}, "sets": {"4"}, "reps": {"6"}}, nil), ids.activityExerciseID, testOwnerID)
				return rr
			},
		},
		{
			name: "session exercise deleted",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleDeleteExercise(rr, postForm(nil, nil), ids.activityExerciseID, testOwnerID)
				return rr
			},
		},
		{
			name: "session exercise planned set added",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleExercisePlannedSets(rr, postForm(nil, map[string]string{"exerciseID": uintStr(ids.activityExerciseID)}))
				return rr
			},
		},
		{
			name: "session exercise planned set saved",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleExercisePlannedSetSave(rr, postForm(url.Values{"reps": {"5"}, "weight_kg": {"10"}},
					map[string]string{"exerciseID": uintStr(ids.activityExerciseID), "setIndex": "1"}))
				return rr
			},
		},
		{
			name: "session exercise planned set deleted",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleExercisePlannedSetDelete(rr, postForm(nil,
					map[string]string{"exerciseID": uintStr(ids.activityExerciseID), "setIndex": "1"}))
				return rr
			},
		},
		{
			name: "session exercise planned sets cleared",
			want: targetSession,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleExercisePlannedSetsClear(rr, postForm(nil, map[string]string{"exerciseID": uintStr(ids.activityExerciseID)}))
				return rr
			},
		},
		{
			name: "block update",
			want: targetBlock,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleUpdateActivityTemplate(rr, postForm(url.Values{"name": {"Renamed Block"}}, nil), ids.blockID, testOwnerID)
				return rr
			},
		},
		{
			name: "exercise added to a block",
			want: targetBlock,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleAddActivityTemplateExercise(rr, postForm(url.Values{"name": {"New Block Row"}, "kind": {"reps_and_sets"}, "sets": {"3"}, "reps": {"5"}}, nil), ids.blockID, testOwnerID)
				return rr
			},
		},
		{
			name: "exercise added to a block from the library",
			want: targetBlock,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleAddATExerciseFromLibrary(rr, postForm(url.Values{"library_exercise_id": {uintStr(ids.libraryExerciseID)}}, nil), ids.blockID, testOwnerID)
				return rr
			},
		},
		{
			name: "block exercises reordered",
			want: targetBlock,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleReorderActivityTemplateExercises(rr, postForm(url.Values{"ordered_ids": {uintStr(ids.blockExerciseID)}}, nil), ids.blockID, testOwnerID)
				return rr
			},
		},
		{
			name: "block exercise updated",
			want: targetBlock,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleUpdateExercise(rr, postForm(url.Values{"name": {"Edited Block Row"}, "kind": {"reps_and_sets"}, "sets": {"4"}, "reps": {"6"}}, nil), ids.blockExerciseID, testOwnerID)
				return rr
			},
		},
		{
			name: "block exercise deleted",
			want: targetBlock,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleDeleteExercise(rr, postForm(nil, nil), ids.blockExerciseID, testOwnerID)
				return rr
			},
		},
		{
			name: "library exercise update",
			want: targetLibrary,
			invoke: func(t *testing.T, srv *Server, ids catalogIDs) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				srv.handleExerciseLibraryByID(rr, postForm(
					url.Values{"name": {"Edited Exercise"}, "kind": {"reps_and_sets"}, "sets": {"4"}, "reps": {"6"}},
					map[string]string{"libraryExerciseID": uintStr(ids.libraryExerciseID), "action": "update"}))
				return rr
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, store, ids := newCatalogFixture(t)
			rr := tc.invoke(t, srv, ids)
			if rr.Code >= 400 {
				t.Fatalf("handler returned %d: %s", rr.Code, strings.TrimSpace(rr.Body.String()))
			}

			var edited *string
			var query string
			switch tc.want {
			case targetSession:
				query = "SELECT catalog_edited_at FROM session_templates WHERE id = " + uintStr(ids.sessionTemplateID)
			case targetBlock:
				query = "SELECT catalog_edited_at FROM activity_templates WHERE id = " + uintStr(ids.blockID)
			case targetLibrary:
				query = "SELECT catalog_edited_at FROM library_exercises WHERE id = " + uintStr(ids.libraryExerciseID)
			}
			if err := store.DB.Raw(query).Scan(&edited).Error; err != nil {
				t.Fatal(err)
			}
			if edited == nil {
				t.Fatal("catalog_edited_at is still NULL — this handler does not stamp the row, so the edit is reverted on the next import")
			}
		})
	}
}

// TestCatalogStampSkipsUserCreatedRows guards the other direction: a row the user made
// themselves is not catalog-managed, so stamping it would be meaningless and would show
// an Edited chip on something that was never in the catalog.
func TestCatalogStampSkipsUserCreatedRows(t *testing.T) {
	srv, store, _ := newCatalogFixture(t)

	own := db.SessionTemplate{OwnerID: testOwnerID, Name: "My Own Session"}
	if err := store.DB.Create(&own).Error; err != nil {
		t.Fatal(err)
	}
	srv.markSessionTemplateEdited(testOwnerID, own.ID)
	var edited *string
	if err := store.DB.Raw("SELECT catalog_edited_at FROM session_templates WHERE id = ?", own.ID).
		Scan(&edited).Error; err != nil {
		t.Fatal(err)
	}
	if edited != nil {
		t.Fatal("a user-created row must never be stamped")
	}
}

// TestCatalogStampIsIdempotent checks the stamp keeps its first timestamp, so "Edited on"
// reports when the user first changed the row rather than when they last touched it.
func TestCatalogStampIsIdempotent(t *testing.T) {
	srv, store, ids := newCatalogFixture(t)

	srv.markSessionTemplateEdited(testOwnerID, ids.sessionTemplateID)
	var first *string
	if err := store.DB.Raw("SELECT catalog_edited_at FROM session_templates WHERE id = ?", ids.sessionTemplateID).
		Scan(&first).Error; err != nil {
		t.Fatal(err)
	}
	if first == nil {
		t.Fatal("first stamp did not land")
	}
	srv.markSessionTemplateEdited(testOwnerID, ids.sessionTemplateID)
	var second *string
	if err := store.DB.Raw("SELECT catalog_edited_at FROM session_templates WHERE id = ?", ids.sessionTemplateID).
		Scan(&second).Error; err != nil {
		t.Fatal(err)
	}
	if *second != *first {
		t.Fatalf("stamp changed on a second edit: %q -> %q", *first, *second)
	}
}
