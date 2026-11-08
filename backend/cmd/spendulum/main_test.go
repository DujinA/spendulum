package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

func TestServeWithAMissingVariableExitsAndNamesIt(t *testing.T) {
	var out syncBuffer

	code := run(t.Context(), []string{"serve"}, map[string]string{}, &out)

	if code == 0 {
		t.Fatal("serve exited 0 without SPENDULUM_DATABASE_URL")
	}

	if !strings.Contains(out.String(), "SPENDULUM_DATABASE_URL") {
		t.Errorf("output does not name SPENDULUM_DATABASE_URL:\n%s", out.String())
	}
}

func TestServeWithAnInvalidVariableExitsAndNamesIt(t *testing.T) {
	var out syncBuffer

	env := map[string]string{
		"SPENDULUM_DATABASE_URL": "postgres://app:secret@" + closedPort(t) + "/spendulum",
		"SPENDULUM_DB_TIMEOUT":   "soon",
	}
	code := run(t.Context(), []string{"serve"}, env, &out)

	if code == 0 {
		t.Fatal("serve exited 0 with an invalid SPENDULUM_DB_TIMEOUT")
	}

	if !strings.Contains(out.String(), "SPENDULUM_DB_TIMEOUT") {
		t.Errorf("output does not name SPENDULUM_DB_TIMEOUT:\n%s", out.String())
	}
}

func TestUnknownCommandExitsNonZero(t *testing.T) {
	var out syncBuffer

	if code := run(t.Context(), []string{"fly"}, map[string]string{}, &out); code == 0 {
		t.Fatal("unknown command exited 0")
	}
}

func TestMigrateWithAnUnreachableDatabaseExitsNonZero(t *testing.T) {
	var out syncBuffer

	env := map[string]string{
		"SPENDULUM_DATABASE_URL": "postgres://app:secret@" + closedPort(t) + "/spendulum?sslmode=disable",
		"SPENDULUM_DB_TIMEOUT":   "500ms",
	}

	if code := run(t.Context(), []string{"migrate"}, env, &out); code == 0 {
		t.Fatalf("migrate exited 0 against an unreachable database:\n%s", out.String())
	}
}

func TestMigrateTwiceSucceedsAndTheSecondRunAppliesNothing(t *testing.T) {
	env := map[string]string{"SPENDULUM_DATABASE_URL": dbtest.NewEmptyDatabase(t)}

	var first syncBuffer
	if code := run(t.Context(), []string{"migrate"}, env, &first); code != 0 {
		t.Fatalf("first migrate exited %d:\n%s", code, first.String())
	}

	var second syncBuffer
	if code := run(t.Context(), []string{"migrate"}, env, &second); code != 0 {
		t.Fatalf("second migrate exited %d:\n%s", code, second.String())
	}

	if !strings.Contains(second.String(), `"applied":0`) {
		t.Errorf("second migrate did not report 0 applied migrations:\n%s", second.String())
	}
}

func TestServeExitsWhenMigrationsArePending(t *testing.T) {
	var out syncBuffer

	env := map[string]string{
		"SPENDULUM_DATABASE_URL": dbtest.NewEmptyDatabase(t),
		"SPENDULUM_HTTP_ADDR":    "127.0.0.1:0",
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	if code := run(ctx, []string{"serve"}, env, &out); code == 0 {
		t.Fatalf("serve exited 0 with pending migrations:\n%s", out.String())
	}

	if !strings.Contains(out.String(), "spendulum migrate") {
		t.Errorf("output does not tell the operator to run spendulum migrate:\n%s", out.String())
	}
}

func TestServeOnAMigratedDatabaseIsReady(t *testing.T) {
	addr := freeAddr(t)
	env := map[string]string{
		"SPENDULUM_DATABASE_URL": dbtest.NewDatabase(t),
		"SPENDULUM_HTTP_ADDR":    addr,
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := startServe(ctx, env)

	waitUntilServing(t, addr)

	if status := get(t, addr, "/readyz"); status != http.StatusOK {
		t.Errorf("GET /readyz = %d, want 200", status)
	}

	cancel()

	if res := <-done; res.code != 0 {
		t.Errorf("serve exited %d after shutdown:\n%s", res.code, res.out)
	}
}

// SIGTERM: stop accepting connections, finish in-flight requests, close the pool, exit 0.
// The database never answers, so /readyz stays in flight for the whole DB timeout.
// That makes it the in-flight request; it also shows serve starts without a database.
func TestSIGTERMDrainsInFlightRequestsAndExitsZero(t *testing.T) {
	addr := freeAddr(t)
	env := map[string]string{
		"SPENDULUM_DATABASE_URL": "postgres://app:secret@" + blackhole(t) + "/spendulum?sslmode=disable",
		"SPENDULUM_HTTP_ADDR":    addr,
		"SPENDULUM_DB_TIMEOUT":   "1500ms",
	}

	done := startServe(t.Context(), env)

	waitUntilServing(t, addr)

	inFlight := make(chan int, 1)

	go func() { inFlight <- get(t, addr, "/readyz") }()

	time.Sleep(200 * time.Millisecond) // let /readyz reach its database ping

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	time.Sleep(200 * time.Millisecond) // let shutdown close the listener

	if _, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", addr); err == nil {
		t.Error("a new connection was accepted after SIGTERM")
	}

	if status := <-inFlight; status != http.StatusServiceUnavailable {
		t.Errorf("in-flight GET /readyz = %d, want it finished with 503", status)
	}

	select {
	case res := <-done:
		if res.code != 0 {
			t.Errorf("serve exited %d after SIGTERM, want 0:\n%s", res.code, res.out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not exit after SIGTERM")
	}
}

type result struct {
	code int
	out  string
}

func startServe(ctx context.Context, env map[string]string) <-chan result {
	done := make(chan result, 1)

	go func() {
		var out syncBuffer

		code := run(ctx, []string{"serve"}, env, &out)
		done <- result{code: code, out: out.String()}
	}()

	return done
}

func get(t *testing.T, addr, path string) int {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		t.Errorf("request: %v", err)

		return 0
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}

	_ = res.Body.Close()

	return res.StatusCode
}

func waitUntilServing(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if get(t, addr, "/healthz") == http.StatusOK {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("server on %s never answered /healthz", addr)
}

// freeAddr returns a local address nothing listens on, for serve to bind.
func freeAddr(t *testing.T) string {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr := ln.Addr().String()
	_ = ln.Close()

	return addr
}

// closedPort returns an address that refuses connections.
func closedPort(t *testing.T) string {
	t.Helper()

	return freeAddr(t)
}

// blackhole returns the address of a listener that accepts connections and never answers.
func blackhole(t *testing.T) string {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		var conns []net.Conn

		for {
			conn, err := ln.Accept()
			if err != nil {
				for _, c := range conns {
					_ = c.Close()
				}

				return
			}

			conns = append(conns, conn)
		}
	}()

	return ln.Addr().String()
}

// syncBuffer is a bytes.Buffer safe for the logger and the test to share.
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
