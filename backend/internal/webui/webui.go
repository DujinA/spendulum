// Package webui serves the single-page app embedded in the binary.
//
// The committed dist/index.html is a stub so the backend builds and tests without
// Node; the Docker build copies web/dist over it.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the embedded SPA build.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // dist is embedded at compile time, so this cannot fail.
	}

	return sub
}

// Handler serves the files in spa. A path with no file serves index.html, so client-side
// routes load the app; a missing file under /assets/ answers 404, because a stale
// hashed asset must not be answered with HTML.
func Handler(spa fs.FS) http.Handler {
	files := http.FileServerFS(spa)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")

		if name != "" && name != "index.html" && isFile(spa, name) {
			if strings.HasPrefix(name, "assets/") {
				// Vite puts a content hash in every file name under assets/.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}

			files.ServeHTTP(w, r)

			return
		}

		if strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, withPath(r, "/"), spa, "index.html")
	})
}

func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)

	return err == nil && !info.IsDir()
}

// withPath returns r with its URL path replaced, so ServeFileFS does not redirect
// requests for index.html.
func withPath(r *http.Request, p string) *http.Request {
	r2 := r.Clone(r.Context())
	r2.URL.Path = p

	return r2
}
