// Package web serves the built Svelte client out of the binary.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// The client is built by `make client`. A fresh clone holds only dist/.gitkeep,
// which is enough for the pattern to match — an embed pattern matching no files
// is a compile error. The all: prefix is required because Vite writes into
// _app, and embed skips names beginning with an underscore without it.
//
//go:embed all:dist
var files embed.FS

// Handler serves the embedded client.
func Handler() http.Handler {
	dist, err := fs.Sub(files, "dist")
	if err != nil {
		// The directory is embedded at compile time, so this cannot fail.
		panic(err)
	}
	return HandlerFS(dist)
}

// HandlerFS serves a client from any filesystem, which is how it is tested
// without running a build first.
func HandlerFS(fsys fs.FS) http.Handler {
	server := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if exists(fsys, r.URL.Path) {
			server.ServeHTTP(w, r)
			return
		}

		// Every other path belongs to the client's own router, which needs the
		// app shell to start before it can read the URL.
		shell := r.Clone(r.Context())
		shell.URL.Path = "/"
		server.ServeHTTP(w, shell)
	})
}

// exists reports whether the request names a real file. A directory does not
// count: serving one lists its contents.
func exists(fsys fs.FS, urlPath string) bool {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" {
		name = "index.html"
	}

	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
