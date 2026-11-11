package migrationcheck_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DujinA/spendulum/backend/internal/platform/db/migrationcheck"
)

func files(names ...string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for _, name := range names {
		fsys[name] = &fstest.MapFile{Data: []byte("-- " + name)}
	}

	return fsys
}

func TestCheckAcceptsANewMigrationAfterTheLastOneOnMaster(t *testing.T) {
	base := files("00001_baseline.sql", "00002_entries.sql")
	head := files("00001_baseline.sql", "00002_entries.sql", "00003_vendors.sql")

	if err := migrationcheck.Check(base, head); err != nil {
		t.Errorf("Check = %v, want nil", err)
	}
}

func TestCheckIgnoresFilesThatAreNotSQL(t *testing.T) {
	base := files("00001_baseline.sql", "migrations.go")
	head := files("00001_baseline.sql", "migrations.go", "README.md")
	head["migrations.go"] = &fstest.MapFile{Data: []byte("package migrations // changed")}

	if err := migrationcheck.Check(base, head); err != nil {
		t.Errorf("Check = %v, want nil", err)
	}
}

func TestCheckRejectsAChangedMigrationFromMaster(t *testing.T) {
	base := files("00001_baseline.sql")
	head := files("00001_baseline.sql")
	head["00001_baseline.sql"] = &fstest.MapFile{Data: []byte("-- edited after merge")}

	assertRejects(t, migrationcheck.Check(base, head), "00001_baseline.sql", "changed")
}

func TestCheckRejectsADeletedMigrationFromMaster(t *testing.T) {
	base := files("00001_baseline.sql", "00002_entries.sql")
	head := files("00001_baseline.sql")

	assertRejects(t, migrationcheck.Check(base, head), "00002_entries.sql", "deleted")
}

func TestCheckRejectsADuplicateVersion(t *testing.T) {
	base := files("00001_baseline.sql")
	head := files("00001_baseline.sql", "00002_entries.sql", "00002_vendors.sql")

	assertRejects(t, migrationcheck.Check(base, head), "00002_vendors.sql", "duplicate version 2")
}

func TestCheckRejectsANewMigrationOlderThanTheLastOneOnMaster(t *testing.T) {
	base := files("00001_baseline.sql", "00003_vendors.sql")
	head := files("00001_baseline.sql", "00002_entries.sql", "00003_vendors.sql")

	assertRejects(t, migrationcheck.Check(base, head), "00002_entries.sql", "out of order")
}

func TestCheckRejectsAFileNameWithoutAVersion(t *testing.T) {
	base := files("00001_baseline.sql")
	head := files("00001_baseline.sql", "entries.sql")

	assertRejects(t, migrationcheck.Check(base, head), "entries.sql", "version")
}

func assertRejects(t *testing.T, err error, wantInMessage ...string) {
	t.Helper()

	if err == nil {
		t.Fatal("Check = nil, want an error")
	}

	for _, want := range wantInMessage {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Check = %q, want it to mention %q", err, want)
		}
	}
}
