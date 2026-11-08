package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/DujinA/spendulum/backend/internal/httpapi"
	"github.com/DujinA/spendulum/backend/internal/webui"
)

const indexHTML = `<!doctype html><title>Spendulum</title><div id="root"></div>`

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func newHandler(dbErr error) http.Handler {
	spa := fstest.MapFS{
		"index.html":        {Data: []byte(indexHTML)},
		"assets/app-abc.js": {Data: []byte("console.log('app')")},
	}

	return httpapi.NewHandler(httpapi.Deps{
		DB:     fakeDB{err: dbErr},
		WebUI:  webui.Handler(spa),
		Logger: slog.New(slog.DiscardHandler),
	})
}

func TestHealthzAnswers200EvenWhenTheDatabaseIsDown(t *testing.T) {
	res := do(t, newHandler(errors.New("connection refused")), "/healthz")

	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)
}

func TestReadyzAnswers200WhenTheDatabasePingSucceeds(t *testing.T) {
	res := do(t, newHandler(nil), "/readyz")

	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)
}

func TestReadyzAnswers503WhenTheDatabasePingFails(t *testing.T) {
	res := do(t, newHandler(errors.New("connection refused")), "/readyz")

	assertStatus(t, res, http.StatusServiceUnavailable)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusServiceUnavailable)
}

func TestUnknownAPIPathAnswers404ProblemJSON(t *testing.T) {
	for _, path := range []string{"/api", "/api/", "/api/nope", "/api/entries/123"} {
		t.Run(path, func(t *testing.T) {
			res := do(t, newHandler(nil), path)

			assertStatus(t, res, http.StatusNotFound)
			assertProblem(t, res, http.StatusNotFound)

			if got := res.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
		})
	}
}

func TestRootServesTheSPA(t *testing.T) {
	res := do(t, newHandler(nil), "/")

	assertStatus(t, res, http.StatusOK)
	assertBody(t, res, indexHTML)
}

func TestUnknownPathOutsideAPIServesTheSPAIndex(t *testing.T) {
	for _, path := range []string{"/entries", "/more/settings", "/apiary"} {
		t.Run(path, func(t *testing.T) {
			res := do(t, newHandler(nil), path)

			assertStatus(t, res, http.StatusOK)
			assertBody(t, res, indexHTML)

			if got := res.Header.Get("Cache-Control"); got != "no-cache" {
				t.Errorf("Cache-Control = %q, want no-cache", got)
			}
		})
	}
}

func TestStaticAssetIsServedAsIs(t *testing.T) {
	res := do(t, newHandler(nil), "/assets/app-abc.js")

	assertStatus(t, res, http.StatusOK)
	assertBody(t, res, "console.log('app')")

	if got := res.Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want an immutable cache", got)
	}
}

func TestMissingStaticAssetAnswers404(t *testing.T) {
	res := do(t, newHandler(nil), "/assets/gone-123.js")

	assertStatus(t, res, http.StatusNotFound)
}

type response struct {
	*http.Response

	body []byte
	req  *http.Request
}

func do(t *testing.T, h http.Handler, path string) response {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	t.Cleanup(func() { _ = res.Body.Close() })

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return response{Response: res, body: body, req: req}
}

func assertStatus(t *testing.T, res response, want int) {
	t.Helper()

	if res.StatusCode != want {
		t.Fatalf("status = %d, want %d (body %s)", res.StatusCode, want, res.body)
	}
}

func assertBody(t *testing.T, res response, want string) {
	t.Helper()

	if string(res.body) != want {
		t.Errorf("body = %q, want %q", res.body, want)
	}
}

func assertProblem(t *testing.T, res response, wantStatus int) {
	t.Helper()

	if got := res.Header.Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}

	var p struct {
		Title  string `json:"title"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(res.body, &p); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, res.body)
	}

	if p.Status != wantStatus || p.Title == "" {
		t.Errorf("problem = %+v, want status %d and a title", p, wantStatus)
	}
}

func assertMatchesSpec(t *testing.T, res response) {
	t.Helper()

	router := specRouter(t)

	route, params, err := router.FindRoute(res.req)
	if err != nil {
		t.Fatalf("operation for %s %s not in the spec: %v", res.req.Method, res.req.URL, err)
	}

	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: res.req, PathParams: params, Route: route},
		Status:                 res.StatusCode,
		Header:                 res.Header,
		Body:                   io.NopCloser(bytes.NewReader(res.body)),
	}
	if err := openapi3filter.ValidateResponse(t.Context(), in); err != nil {
		t.Errorf("response does not match the spec: %v", err)
	}
}

func specRouter(t *testing.T) routers.Router {
	t.Helper()

	spec, err := openapi3.NewLoader().LoadFromFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("spec is invalid: %v", err)
	}

	router, err := legacy.NewRouter(spec)
	if err != nil {
		t.Fatalf("router: %v", err)
	}

	return router
}
