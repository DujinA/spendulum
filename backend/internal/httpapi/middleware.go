package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// noStoreOutsideSPA marks API and probe responses uncacheable, so neither the browser
// nor the service worker keeps them. The SPA handler sets its own Cache-Control.
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

// logRequests logs one line per API request. It logs the method, path and status
// only: never a body, so amounts, vendor names and notes stay out of the logs.
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
