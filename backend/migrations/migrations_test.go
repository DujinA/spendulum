package migrations_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
	"github.com/DujinA/spendulum/backend/internal/platform/db/migrationcheck"
	"github.com/DujinA/spendulum/backend/migrations"
)

// CI sets it to the master commit the change applies to; locally the merge base with origin/master is used.
const baseRefVar = "MIGRATIONS_BASE_REF"

func TestMain(m *testing.M) { dbtest.Main(m) }

func TestGooseUpOnAnEmptyDatabaseAppliesEveryMigration(t *testing.T) {
	d, err := db.Open(t.Context(), dbtest.NewEmptyDatabase(t), 30*time.Second)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(d.Close)

	applied, err := d.Migrate(t.Context())
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	sqlFiles, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}

	if applied != len(sqlFiles) {
		t.Errorf("Migrate applied %d migrations, want all %d files", applied, len(sqlFiles))
	}
}

func TestMigrationsOnMasterAreUnchangedAndNewOnesComeAfterThem(t *testing.T) {
	base := masterMigrations(t, baseRef(t))

	if err := migrationcheck.Check(base, migrations.FS); err != nil {
		t.Errorf("migrations break ADR 0008:\n%v", err)
	}
}

func baseRef(t *testing.T) string {
	t.Helper()

	if ref := os.Getenv(baseRefVar); ref != "" {
		if _, err := git(t, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
			t.Fatalf("%s=%s does not name a commit: %v", baseRefVar, ref, err)
		}

		return ref
	}

	ref, err := git(t, "merge-base", "origin/master", "HEAD")
	if err != nil {
		t.Skipf("no merge base with origin/master (%v); set %s to run this check", err, baseRefVar)
	}

	return strings.TrimSpace(ref)
}

func masterMigrations(t *testing.T, ref string) fstest.MapFS {
	t.Helper()

	names, err := git(t, "ls-tree", "--name-only", ref, ".")
	if err != nil {
		t.Fatalf("list migrations at %s: %v", ref, err)
	}

	fsys := fstest.MapFS{}

	for name := range strings.FieldsSeq(names) {
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		// The ./ prefix makes git resolve the path from this directory.
		content, err := git(t, "show", ref+":./"+name)
		if err != nil {
			t.Fatalf("read %s at %s: %v", name, ref, err)
		}

		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}

	return fsys
}

func git(t *testing.T, args ...string) (string, error) {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "git", args...).Output()

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return "", errors.New(strings.TrimSpace(string(exitErr.Stderr)))
	}

	return string(out), err
}
