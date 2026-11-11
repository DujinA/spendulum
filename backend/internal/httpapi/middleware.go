package httpapi

import (
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

// A cross-site form cannot send application/json, so this also stops form-based CSRF.
func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPI(r.URL.Path) && isMutating(r.Method) {
			if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
				problem.Write(w, http.StatusUnsupportedMediaType, "Send the request as application/json.")

				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func rejectCrossOrigin(next http.Handler) http.Handler {
	c := http.NewCrossOriginProtection()
	c.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		problem.Write(w, http.StatusForbidden, "Cross-origin requests are not allowed.")
	}))

	return c.Handler(next)
}

// Neither the browser nor the service worker may keep API or probe responses.
func noStoreOutsideSPA(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPI(r.URL.Path) || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			w.Header().Set("Cache-Control", "no-store")
		}

		next.ServeHTTP(w, r)
	})
}

func isAPI(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Method, path and status only: a body would put amounts, vendor names and notes in the logs.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPI(r.URL.Path) {
			next.ServeHTTP(w, r)

			return
		}

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		logger.InfoContext(r.Context(), "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
