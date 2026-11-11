package dbtest

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

const Image = "postgres:18"

const templateName = "spendulum_template"

var adminURL string //nolint:gochecknoglobals // set once by Main, shared by the tests of one binary

func Main(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	container, err := postgres.Run(ctx, Image,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("spendulum"),
		postgres.WithPassword("spendulum"),
		postgres.BasicWaitStrategies(),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			log.Printf("dbtest: terminate container: %v", err)
		}
	}()

	if err != nil {
		log.Printf("dbtest: start %s: %v", Image, err)

		return 1
	}

	adminURL, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("dbtest: connection string: %v", err)

		return 1
	}

	if err := createTemplate(ctx); err != nil {
		log.Printf("dbtest: %v", err)

		return 1
	}

	return m.Run()
}

func createTemplate(ctx context.Context) error {
	if err := exec(ctx, "CREATE DATABASE "+templateName); err != nil {
		return err
	}

	d, err := db.Open(ctx, withDatabase(adminURL, templateName), 30*time.Second)
	if err != nil {
		return fmt.Errorf("open template: %w", err)
	}
	// Close before cloning: PostgreSQL refuses a template that has open connections.
	defer d.Close()

	if _, err := d.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate template: %w", err)
	}

	return nil
}

func NewDatabase(tb testing.TB) string {
	tb.Helper()

	return newDatabase(tb, "TEMPLATE "+templateName)
}

func NewEmptyDatabase(tb testing.TB) string {
	tb.Helper()

	return newDatabase(tb, "")
}

func newDatabase(tb testing.TB, options string) string {
	tb.Helper()

	// Unquoted identifiers fold to lower case, so lower the random part up front.
	name := "test_" + strings.ToLower(rand.Text()[:16])

	if err := exec(tb.Context(), "CREATE DATABASE "+name+" "+options); err != nil {
		tb.Fatalf("dbtest: %v", err)
	}

	tb.Cleanup(func() {
		// tb.Context is already cancelled during cleanup.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			tb.Errorf("dbtest: %v", err)
		}
	})

	return withDatabase(adminURL, name)
}

func exec(ctx context.Context, sql string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("%s: %w", sql, err)
	}

	return nil
}

func withDatabase(rawURL, name string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err) // testcontainers returns a valid URL.
	}

	u.Path = "/" + name

	return u.String()
}

// NewUser inserts a user with a fake email, for tests of tables that reference users.
func NewUser(tb testing.TB, q *gen.Queries) uuid.UUID {
	tb.Helper()

	id := uuid.Must(uuid.NewV7())
	email := "user-" + strings.ToLower(rand.Text()[:8]) + "@example.test"

	if err := q.CreateUser(tb.Context(), gen.CreateUserParams{ID: id, Email: email, PasswordHash: "not-a-hash"}); err != nil {
		tb.Fatalf("dbtest: create user: %v", err)
	}

	return id
}

// Open opens a fresh migrated database and closes it when the test ends.
func Open(tb testing.TB) *db.DB {
	tb.Helper()

	d, err := db.Open(tb.Context(), NewDatabase(tb), 5*time.Second)
	if err != nil {
		tb.Fatalf("dbtest: open: %v", err)
	}

	tb.Cleanup(d.Close)

	return d
}
