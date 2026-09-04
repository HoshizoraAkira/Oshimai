package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// distFS embeds all production React assets from the dist directory.
//
//go:embed all:dist
var distFS embed.FS

// Assets provides fs.FS interface to the embedded dist directory.
var Assets, _ = fs.Sub(distFS, "dist")

// Handler serves static assets from the React dist directory, falling back to index.html for SPA routing.
func Handler() http.Handler {
	fileServer := http.FileServer(http.FS(Assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if file exists in embedded assets
		if f, err := Assets.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for client-side SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
