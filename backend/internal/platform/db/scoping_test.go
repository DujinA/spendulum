package db_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	queryName   = regexp.MustCompile(`(?m)^-- name: (\w+)`)
	tableRef    = regexp.MustCompile(`(?i)\b(?:from|join|into|update)\s+(\w+)`)
	userIDWhere = regexp.MustCompile(`(?is)\bwhere\b.*\buser_id\s*=`)
	userIDCol   = regexp.MustCompile(`(?i)\buser_id\b`)
)

// A query may skip the user_id predicate only with a "-- unscoped: <reason>" line above its name.
func TestEveryQueryOnAUserOwnedTableIsScopedByUser(t *testing.T) {
	files, err := filepath.Glob("../../../queries/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no query files found: %v", err)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		for _, q := range splitQueries(string(content)) {
			if msg := unscoped(q); msg != "" {
				t.Errorf("%s: query %s %s (ADR 0009)", filepath.Base(file), q.name, msg)
			}
		}
	}
}

type query struct {
	name, sql string
	exempt    bool
}

func splitQueries(content string) []query {
	locs := queryName.FindAllStringSubmatchIndex(content, -1)
	queries := make([]query, 0, len(locs))
	for i, loc := range locs {
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}

		start := 0
		if i > 0 {
			start = locs[i-1][1]
		}

		queries = append(queries, query{
			name:   content[loc[2]:loc[3]],
			sql:    content[loc[1]:end],
			exempt: strings.Contains(content[start:loc[0]], "-- unscoped:"),
		})
	}

	return queries
}

func unscoped(q query) string {
	if q.exempt {
		return ""
	}

	// Tables with no user_id: shared data, and users, whose id is the user id (ADR 0009).
	unscopedTables := map[string]bool{"users": true}

	for _, m := range tableRef.FindAllStringSubmatch(q.sql, -1) {
		table := strings.ToLower(m[1])
		if unscopedTables[table] {
			continue
		}

		insert := regexp.MustCompile(`(?i)\binsert\s+into\s+` + table + `\b`).MatchString(q.sql)
		if insert && !userIDCol.MatchString(q.sql) {
			return "inserts into " + table + " without user_id"
		}

		if !insert && !userIDWhere.MatchString(q.sql) {
			return "reads or writes " + table + " without a user_id predicate"
		}
	}

	return ""
}
