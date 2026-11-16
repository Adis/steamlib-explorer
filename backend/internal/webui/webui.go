// Package webui serves the built frontend, embedded into the binary.
//
// `make build` (or `cd frontend && npm run build`) writes the Vite build into
// dist/. A binary built without it still runs and shows a page saying so.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the embedded frontend build.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is always embedded, so this cannot happen
	}
	return New(sub)
}

// New serves the frontend build in files (index.html plus assets/).
func New(files fs.FS) http.Handler {
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return http.HandlerFunc(notBuilt)
	}
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			// Vite puts a content hash in every asset name, so they never change.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// index.html names the current assets, so always check for a new one.
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func notBuilt(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`<!doctype html><title>Steamlib Explorer</title>
<p>This binary was built without the frontend. Run <code>make build</code> to build both,
or use <code>make dev</code> and open <a href="http://localhost:5173">http://localhost:5173</a>.</p>`))
}
