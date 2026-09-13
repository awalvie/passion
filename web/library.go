package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"passion/store"
)

type libraryParams struct {
	BaseParams
	Tab   string
	Tabs  []libraryTab
	Items []store.Content
}

type libraryTab struct {
	Kind   string
	Label  string
	Active bool
}

type libraryItemParams struct {
	BaseParams
	Node store.Node
}

var libraryTabs = []libraryTab{
	{Kind: store.KindSession, Label: "Sessions"},
	{Kind: store.KindBlock, Label: "Blocks"},
	{Kind: store.KindMovement, Label: "Movements"},
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	actor, _ := actorFrom(r.Context())

	kind := r.URL.Query().Get("kind")
	if !knownKind(kind) {
		kind = store.KindSession
	}

	items, err := s.store.ListContent(r.Context(), actor.ID, kind)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	tabs := make([]libraryTab, len(libraryTabs))
	copy(tabs, libraryTabs)
	for i := range tabs {
		tabs[i].Active = tabs[i].Kind == kind
	}

	s.render.page(w, http.StatusOK, "library", libraryParams{
		BaseParams: BaseParams{Title: "Library", Authenticated: true},
		Tab:        kind,
		Tabs:       tabs,
		Items:      items,
	})
}

func (s *Server) handleLibraryItem(w http.ResponseWriter, r *http.Request) {
	actor, _ := actorFrom(r.Context())

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	node, err := s.store.ContentTree(r.Context(), actor.ID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	s.render.page(w, http.StatusOK, "library_item", libraryItemParams{
		BaseParams: BaseParams{Title: node.Name, Authenticated: true},
		Node:       node,
	})
}

func knownKind(kind string) bool {
	for _, t := range libraryTabs {
		if t.Kind == kind {
			return true
		}
	}
	return false
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, msg := statusFor(err)
	if status == http.StatusInternalServerError {
		s.log.Error("library", "path", r.URL.Path, "error", err)
	}
	http.Error(w, msg, status)
}
