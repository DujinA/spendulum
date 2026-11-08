package db_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

func TestPingSucceedsOnAMigratedDatabase(t *testing.T) {
	d := open(t, dbtest.NewDatabase(t), time.Second)

	if err := d.Ping(t.Context()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestPingOnAStuckDatabaseFailsWithinTheTimeout(t *testing.T) {
	d := open(t, "postgres://app:secret@"+blackhole(t)+"/spendulum?sslmode=disable", 200*time.Millisecond)

	start := time.Now()
	err := d.Ping(t.Context())

	if err == nil {
		t.Fatal("Ping succeeded against a database that never answers")
	}

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Ping took %s, want it bounded by the 200ms timeout", elapsed)
	}
}

func TestSlowQueryIsCancelledByTheTimeout(t *testing.T) {
	d := open(t, dbtest.NewDatabase(t), 200*time.Millisecond)

	for name, run := range map[string]func(context.Context) error{
		"exec":      func(ctx context.Context) error { return d.ExecForTest(ctx, "SELECT pg_sleep(5)") },
		"query":     func(ctx context.Context) error { return d.QueryForTest(ctx, "SELECT pg_sleep(5)") },
		"query row": func(ctx context.Context) error { return d.QueryRowForTest(ctx, "SELECT pg_sleep(5)") },
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			err := run(t.Context())

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("error = %v, want context.DeadlineExceeded", err)
			}

			if elapsed := time.Since(start); elapsed > 2*time.Second {
				t.Errorf("query took %s, want it bounded by the 200ms timeout", elapsed)
			}
		})
	}
}

func TestQueryWithinTheTimeoutReadsAllRows(t *testing.T) {
	d := open(t, dbtest.NewDatabase(t), time.Second)

	if err := d.QueryForTest(t.Context(), "SELECT generate_series(1, 1000)"); err != nil {
		t.Fatalf("query: %v", err)
	}
}

func TestMigrateOnAFreshDatabaseAppliesEveryMigrationOnce(t *testing.T) {
	d := open(t, dbtest.NewEmptyDatabase(t), time.Second)

	first, err := d.Migrate(t.Context())
	if err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	if first == 0 {
		t.Error("first Migrate applied nothing, want the embedded migrations")
	}

	second, err := d.Migrate(t.Context())
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	if second != 0 {
		t.Errorf("second Migrate applied %d migrations, want 0", second)
	}
}

func TestHasPendingReportsUnappliedMigrations(t *testing.T) {
	d := open(t, dbtest.NewEmptyDatabase(t), time.Second)

	pending, err := d.HasPending(t.Context())
	if err != nil || !pending {
		t.Fatalf("HasPending before migrate = %v, %v; want true, nil", pending, err)
	}

	if _, err := d.Migrate(t.Context()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	pending, err = d.HasPending(t.Context())
	if err != nil || pending {
		t.Fatalf("HasPending after migrate = %v, %v; want false, nil", pending, err)
	}
}

func TestMigrateOnAnUnreachableDatabaseFails(t *testing.T) {
	d := open(t, "postgres://app:secret@"+blackhole(t)+"/spendulum?sslmode=disable", 200*time.Millisecond)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	if _, err := d.Migrate(ctx); err == nil {
		t.Fatal("Migrate succeeded against an unreachable database")
	}
}

func open(t *testing.T, url string, timeout time.Duration) *db.DB {
	t.Helper()

	d, err := db.Open(t.Context(), url, timeout)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(d.Close)

	return d
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
