package httpapi_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/entry"
	"github.com/DujinA/spendulum/backend/internal/fx"
	"github.com/DujinA/spendulum/backend/internal/httpapi"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
	"github.com/DujinA/spendulum/backend/internal/settings"
	"github.com/DujinA/spendulum/backend/internal/vendors"
	"github.com/DujinA/spendulum/backend/internal/webui"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

const (
	indexHTML = `<!doctype html><title>Spendulum</title><div id="root"></div>`
	email     = "ana@example.test"
	password  = "correct horse battery"
)

type app struct {
	handler http.Handler
	db      *db.DB
	auth    *auth.Service
	logs    *syncBuffer
	now     time.Time
	mu      sync.Mutex
}

type appOption func(*httpapi.Deps)

func withPinger(p httpapi.Pinger) appOption {
	return func(d *httpapi.Deps) { d.DB = p }
}

func newApp(t *testing.T, opts ...appOption) *app {
	t.Helper()

	d := dbtest.Open(t)
	a := &app{db: d, logs: &syncBuffer{}, now: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)}
	logger := slog.New(slog.NewJSONHandler(a.logs, nil))
	settingsSvc := settings.NewService(settings.NewPostgres(d.Queries()))
	categoryRepo := category.NewPostgres(d.Queries())
	categorySvc := category.NewService(category.Deps{Repo: categoryRepo, Keys: categoryRepo, Tx: d, Now: a.clock})
	vendorSvc := vendors.NewService(vendors.NewPostgres(d.Queries()))

	a.auth = auth.NewService(auth.Deps{
		Repo:     auth.NewPostgres(d.Queries()),
		Tx:       d,
		Hasher:   auth.NewHasher(),
		Limiter:  auth.NewLimiter(),
		Defaults: []auth.AccountDefaults{settingsSvc, categorySvc},
		Now:      a.clock,
	})

	deps := httpapi.Deps{
		DB:         d,
		WebUI:      webui.Handler(fstest.MapFS{"index.html": {Data: []byte(indexHTML)}, "assets/app-abc.js": {Data: []byte("console.log('app')")}}),
		Logger:     logger,
		Auth:       auth.NewHandler(a.auth, logger, true, []netip.Prefix{netip.MustParsePrefix("10.42.0.0/16")}),
		Settings:   settings.NewHandler(settingsSvc),
		Categories: category.NewHandler(categorySvc),
		Vendors:    vendors.NewHandler(vendorSvc),
		Entries: entry.NewHandler(entry.NewService(entry.Deps{
			Repo: entry.NewPostgres(d.Queries()), Vendors: vendorSvc, Tx: d, Settings: settingsSvc, Now: a.clock,
		}), logger),
		Currencies: fx.NewHandler(fx.NewService(fx.Deps{Repo: fx.NewPostgres(d.Queries())})),
	}
	for _, o := range opts {
		o(&deps)
	}

	a.handler = httpapi.NewHandler(deps)

	return a
}

func (a *app) clock() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.now
}

func (a *app) advance(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.now = a.now.Add(d)
}

func (a *app) account(t *testing.T) uuid.UUID {
	t.Helper()

	return a.accountFor(t, email)
}

func (a *app) accountFor(t *testing.T, email string) uuid.UUID {
	t.Helper()

	id, err := a.auth.CreateAccount(t.Context(), email, password)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	return id
}

// login logs in and returns the session cookie.
func (a *app) login(t *testing.T) *http.Cookie {
	t.Helper()

	return a.loginAs(t, email)
}

func (a *app) loginAs(t *testing.T, email string) *http.Cookie {
	t.Helper()

	res := a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials(email, password)})
	assertStatus(t, res, http.StatusNoContent)

	return sessionCookie(t, res)
}

func (a *app) addCategory(t *testing.T, userID uuid.UUID, c category.Category) category.Category {
	t.Helper()

	c.ID = uuid.Must(uuid.NewV7())
	if err := category.NewPostgres(a.db.Queries()).Create(t.Context(), userID, c); err != nil {
		t.Fatalf("create category: %v", err)
	}

	return c
}

type request struct {
	method, path, json string
	contentType        string
	cookie             *http.Cookie
	header             map[string]string
	remoteAddr         string
}

func (a *app) do(t *testing.T, r request) response {
	t.Helper()

	if r.method == "" {
		r.method = http.MethodGet
	}

	var body io.Reader
	if r.json != "" {
		body = strings.NewReader(r.json)
	}

	req := httptest.NewRequestWithContext(t.Context(), r.method, r.path, body)

	switch {
	case r.contentType != "":
		req.Header.Set("Content-Type", r.contentType)
	case r.method != http.MethodGet:
		req.Header.Set("Content-Type", "application/json")
	}

	if r.cookie != nil {
		req.AddCookie(r.cookie)
	}

	for k, v := range r.header {
		req.Header.Set(k, v)
	}

	if r.remoteAddr != "" {
		req.RemoteAddr = r.remoteAddr
	}

	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)

	res := rec.Result()
	t.Cleanup(func() { _ = res.Body.Close() })

	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	// The validator reads the request body again.
	if r.json != "" {
		req.Body = io.NopCloser(strings.NewReader(r.json))
	}

	return response{Response: res, body: b, req: req}
}

func credentials(email, password string) string {
	return `{"email":"` + email + `","password":"` + password + `"}`
}

func sessionCookie(t *testing.T, res response) *http.Cookie {
	t.Helper()

	for _, c := range res.Cookies() {
		if c.Name == "spendulum_session" {
			return c
		}
	}

	t.Fatalf("no spendulum_session cookie in %v", res.Header.Values("Set-Cookie"))

	return nil
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }
