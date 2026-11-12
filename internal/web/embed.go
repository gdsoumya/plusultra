// Package web serves the Expo web export embedded at build time (make web),
// or the placeholder page when the binary was built without one.
package web

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

//go:embed index.html
var placeholder []byte

// reserved paths belong to the API and never fall back to the app shell.
var reserved = []string{"/api/", "/mcp", "/healthz", "/llms"}

func init() {
	// Go's built-in table has no fonts and distroless has no /etc/mime.types.
	_ = mime.AddExtensionType(".ttf", "font/ttf")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
}

// Handler serves the web app at / and its index for client-side routes.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return newHandler(sub)
}

func newHandler(fsys fs.FS) http.Handler {
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		index = placeholder
	}
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		for _, pre := range reserved {
			if strings.HasPrefix(p, pre) {
				http.NotFound(w, r)
				return
			}
		}
		name := strings.TrimPrefix(path.Clean(p), "/")
		if st, err := fs.Stat(fsys, name); err == nil && !st.IsDir() {
			// Expo names bundles and assets by content hash.
			if strings.HasPrefix(p, "/_expo/") || strings.HasPrefix(p, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index) //nolint:gosec // embedded page, not request data
	})
}
