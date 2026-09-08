// Package web serves the HTML. It holds no database handle: every handler reaches the
// data through a *store.Store and cannot write a query of its own.
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"

	"gorm.io/gorm"

	"passion/store"
)

// BaseParams is what every page needs and no page has to think about. A page's own Params
// struct embeds it.
//
// There is no CSRF token field: CSRF is an Origin check on unsafe methods, so nothing has
// to be stored server-side or threaded through a template.
type BaseParams struct {
	Title         string
	Authenticated bool
	IsAuthPage    bool
	WideLayout    bool
}

// Renderer owns the parsed templates. They are parsed once at boot, so a malformed
// template fails the process instead of the first request that reaches it.
type Renderer struct {
	pages  map[string]*template.Template
	assets fs.FS
	sums   map[string]string
	mu     sync.RWMutex
	log    *slog.Logger
}

// NewRenderer parses the named pages against the layouts. Layouts and fragments are parsed
// alongside each page, because html/template resolves {{ template }} within a set rather
// than across sets.
//
// The page list is explicit, and deliberately not "every .html in the tree". The 50
// shipped templates need 29 template functions between them — splitTags, markdownHTML,
// exerciseSummary and the rest — and those arrive with the phases that need them. Naming
// the pages means a page this build claims to serve either parses or fails the build,
// while a page no handler has been written for yet is simply absent.
func NewRenderer(templatesFS, staticFS fs.FS, log *slog.Logger, pages []string) (*Renderer, error) {
	r := &Renderer{
		pages:  map[string]*template.Template{},
		assets: staticFS,
		sums:   map[string]string{},
		log:    log,
	}

	shared, err := sharedTemplateNames(templatesFS)
	if err != nil {
		return nil, err
	}

	if len(pages) == 0 {
		return nil, errors.New("web: no pages were named")
	}
	for _, name := range pages {
		file := name + ".html"
		files := append([]string{file}, shared...)
		files = append(files, pageFragments[name]...)
		t, err := template.New(file).Funcs(r.funcs()).ParseFS(templatesFS, files...)
		if err != nil {
			return nil, fmt.Errorf("web: parsing %s: %w", file, err)
		}
		r.pages[name] = t
	}
	return r, nil
}

// pageNames is every page this build serves. A phase adds its pages here, and by phase 12
// the list covers the whole tree.
func pageNames() []string {
	return []string{
		// phase 1
		"login", "signup",
	}
}

// pageFragments names the fragments each page needs, for the HTMX swaps it drives. Empty
// until a phase adds a page with fragments; login and signup have none.
var pageFragments = map[string][]string{}

func sharedTemplateNames(templatesFS fs.FS) ([]string, error) {
	var out []string
	// Only the layout shell. fragments/ is deliberately excluded: those templates call the
	// 29 template functions that arrive with later phases, and parsing them alongside every
	// page would make the whole tree a dependency of the login screen. A page that needs a
	// fragment names it in pageFragments.
	for _, dir := range []string{"layouts", "layouts/fragments"} {
		entries, err := fs.ReadDir(templatesFS, dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("web: reading %s: %w", dir, err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".html") {
				out = append(out, path.Join(dir, e.Name()))
			}
		}
	}
	return out, nil
}

func (r *Renderer) funcs() template.FuncMap {
	return template.FuncMap{
		"asset": r.assetURL,
	}
}

// assetURL appends a content hash so a changed stylesheet is not served from a cache. The
// hash is computed once per file and remembered.
func (r *Renderer) assetURL(p string) string {
	clean := strings.TrimPrefix(p, "/static/")

	r.mu.RLock()
	sum, ok := r.sums[clean]
	r.mu.RUnlock()
	if ok {
		return p + "?v=" + sum
	}

	data, err := fs.ReadFile(r.assets, clean)
	if err != nil {
		// A missing asset is not worth failing a page render over; the browser will show
		// the 404 and the log line says which file.
		r.log.Warn("asset not found", "path", clean)
		return p
	}
	h := sha256.Sum256(data)
	sum = hex.EncodeToString(h[:])[:12]

	r.mu.Lock()
	r.sums[clean] = sum
	r.mu.Unlock()
	return p + "?v=" + sum
}

// page renders one template into a buffer first, so a template error becomes a 500 with
// nothing written rather than a half-finished page with a 200 already sent.
func (r *Renderer) page(w http.ResponseWriter, status int, name string, data any) {
	t, ok := r.pages[name]
	if !ok {
		r.log.Error("no such page template", "name", name)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var buf strings.Builder
	if err := t.ExecuteTemplate(&buf, "layouts/base", data); err != nil {
		r.log.Error("rendering page", "name", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, buf.String())
}

// fragment renders a named template without the layout, for HTMX swaps.
func (r *Renderer) fragment(w http.ResponseWriter, pageName, tmplName string, data any) {
	t, ok := r.pages[pageName]
	if !ok {
		r.log.Error("no such page template", "name", pageName)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var buf strings.Builder
	if err := t.ExecuteTemplate(&buf, tmplName, data); err != nil {
		r.log.Error("rendering fragment", "name", tmplName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, buf.String())
}

// statusFor is the one place a store error becomes an HTTP status. Nothing below the
// handler knows about HTTP, so this is where the translation happens and there is exactly
// one of it.
//
// "Not yours" maps to 404, never 403. A 403 confirms the row exists, which tells an
// unauthorised caller something it should not learn.
func statusFor(err error) (int, string) {
	switch {
	case err == nil:
		return http.StatusOK, ""
	case errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound, "Not found."
	case errors.Is(err, gorm.ErrDuplicatedKey), errors.Is(err, store.ErrEmailTaken):
		return http.StatusConflict, "That already exists."
	case errors.Is(err, store.ErrShipped):
		return http.StatusConflict, "This comes with the app. Make your own copy to change it."
	case errors.Is(err, store.ErrBadCredentials):
		return http.StatusUnauthorized, "Email or password is wrong."
	default:
		return http.StatusInternalServerError, "Something went wrong."
	}
}
