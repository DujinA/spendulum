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

func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // dist is embedded at compile time, so this cannot fail.
	}

	return sub
}

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

		// A stale hashed asset must get a 404, not index.html.
		if strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, withPath(r, "/index"), spa, "index.html")
	})
}

func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)

	return err == nil && !info.IsDir()
}

// Without a new path, ServeFileFS redirects requests for index.html.
func withPath(r *http.Request, p string) *http.Request {
	r2 := r.Clone(r.Context())
	r2.URL.Path = p

	return r2
}
