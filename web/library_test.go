package web

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"passion/store"
)

// signIn makes an account and returns its cookie and id.
func signIn(t *testing.T, srv *Server, email string) (*http.Cookie, int64) {
	t.Helper()
	rec := post(t, srv.Routes(), "/signup", url.Values{
		"email":    {email},
		"password": {"a-long-enough-password"},
	}, nil)
	c := authCookie(t, rec)

	a, err := srv.store.Authenticate(context.Background(), email, "a-long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	return c, a.ID
}

// importTrees puts the app's real catalog in, and gives one account a tree of its own.
func importTrees(t *testing.T, st *store.Store, owner string) {
	t.Helper()
	ctx := context.Background()

	root, err := os.OpenRoot("../catalog")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	tree, err := store.Load(root.FS(), store.ShippedTree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ImportShipped(ctx, tree); err != nil {
		t.Fatal(err)
	}
}

func TestTheLibraryListsTheAppsSessions(t *testing.T) {
	srv, st := newTestServer(t)
	cookie, _ := signIn(t, srv, "a@b.c")
	importTrees(t, st, "a@b.c")

	rec := get(t, srv.Routes(), "/library", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Library") {
		t.Error("the page does not say Library")
	}
	if strings.Contains(rec.Body.String(), "Nothing here yet") {
		t.Error("the library is empty after importing the shipped tree")
	}
}

func TestTheLibraryNeedsAnAccount(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv.Routes(), "/library", nil)
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("want a redirect to the login page, got %d", rec.Code)
	}
}

func TestASessionPageShowsItsBlocksAndMovements(t *testing.T) {
	srv, st := newTestServer(t)
	cookie, id := signIn(t, srv, "a@b.c")
	importTrees(t, st, "a@b.c")

	sessions, err := st.ListContent(context.Background(), id, store.KindSession)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) == 0 {
		t.Fatal("the shipped tree has no sessions")
	}

	rec := get(t, srv.Routes(), "/library/"+strconv.FormatInt(sessions[0].ID, 10), cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, html.EscapeString(sessions[0].Name)) {
		t.Errorf("the page does not name the session %q", sessions[0].Name)
	}

	tree, err := st.ContentTree(context.Background(), id, sessions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range tree.Children {
		if !strings.Contains(body, html.EscapeString(block.Name)) {
			t.Errorf("the page does not name the block %q", block.Name)
		}
	}
}

// The app's content carries no badge, so a page showing "Yours" on it would be wrong.
func TestTheAppsContentIsNotBadgedAsYours(t *testing.T) {
	srv, st := newTestServer(t)
	cookie, _ := signIn(t, srv, "a@b.c")
	importTrees(t, st, "a@b.c")

	rec := get(t, srv.Routes(), "/library", cookie)
	if strings.Contains(rec.Body.String(), "Yours") {
		t.Error("the app's own content is badged as Yours")
	}
}

func TestAMissingItemIs404(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie, _ := signIn(t, srv, "a@b.c")

	rec := get(t, srv.Routes(), "/library/999999", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestCopyingASessionLandsOnYourOwn(t *testing.T) {
	srv, st := newTestServer(t)
	cookie, id := signIn(t, srv, "a@b.c")
	importTrees(t, st, "a@b.c")

	sessions, err := st.ListContent(context.Background(), id, store.KindSession)
	if err != nil {
		t.Fatal(err)
	}
	src := sessions[0]

	rec := post(t, srv.Routes(), "/library/"+strconv.FormatInt(src.ID, 10)+"/copy", nil, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("want a redirect, got %d: %s", rec.Code, rec.Body.String())
	}

	loc := rec.Header().Get("Location")
	page := get(t, srv.Routes(), loc, cookie)
	if page.Code != http.StatusOK {
		t.Fatalf("the copy does not render: %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), "Yours") {
		t.Error("the copy is not badged as yours")
	}
	if !strings.Contains(page.Body.String(), html.EscapeString(src.Name)) {
		t.Errorf("the copy does not keep the name %q", src.Name)
	}
}

// A row you already own has nothing to copy to.
func TestYourOwnRowOffersNoCopyButton(t *testing.T) {
	srv, st := newTestServer(t)
	cookie, id := signIn(t, srv, "a@b.c")
	importTrees(t, st, "a@b.c")

	sessions, err := st.ListContent(context.Background(), id, store.KindSession)
	if err != nil {
		t.Fatal(err)
	}
	copied, err := st.CopyToMine(context.Background(), id, sessions[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	rec := get(t, srv.Routes(), "/library/"+strconv.FormatInt(copied.ID, 10), cookie)
	if strings.Contains(rec.Body.String(), "Copy to my content") {
		t.Error("a row you own offers a copy button")
	}
}
