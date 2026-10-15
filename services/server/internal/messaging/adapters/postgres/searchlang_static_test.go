// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// insertInto finds the start of an INSERT INTO <table> statement in a query file.
var insertInto = regexp.MustCompile(`(?i)\bINSERT\s+INTO\s+(\w+)`)

// missingSearchLanguage returns the start of every INSERT INTO table in sql whose statement (up
// to the next ";") does not name search_language.
func missingSearchLanguage(sql, table string) []string {
	var out []string
	for _, m := range insertInto.FindAllStringSubmatchIndex(sql, -1) {
		if !strings.EqualFold(sql[m[2]:m[3]], table) {
			continue
		}
		stmt := sql[m[0]:]
		if end := strings.Index(stmt, ";"); end >= 0 {
			stmt = stmt[:end]
		}
		if !regexp.MustCompile(`\bsearch_language\b`).MatchString(stmt) {
			out = append(out, strings.SplitN(stmt, "\n", 2)[0])
		}
	}
	return out
}

// TestMessageInsertsSetSearchLanguage holds 14 3.1 for every writer of messages in this module
// (docs/DECISIONS.md D-M3-13-1): each INSERT INTO messages in queries/*.sql names search_language,
// so no message is created without its language; and no generated read carries the
// vector (D-M3-13-10).
func TestMessageInsertsSetSearchLanguage(t *testing.T) {
	t.Parallel()
	if got := missingSearchLanguage("-- name: X :one\nINSERT INTO messages (id, content_md)\nVALUES (@id, @content_md);\n", "messages"); len(got) != 1 {
		t.Fatalf("the check accepts an insert without search_language: %v", got)
	}
	if got := missingSearchLanguage("INSERT INTO messages (id, search_language) VALUES (@id, @l);", "messages"); len(got) != 0 {
		t.Fatalf("the check refuses an insert with search_language: %v", got)
	}
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no query files: %v", err)
	}
	inserts := 0
	for _, f := range files {
		data, err := os.ReadFile(f) // #nosec G304 -- a file of this package's queries directory.
		if err != nil {
			t.Fatal(err)
		}
		inserts += len(insertInto.FindAllString(string(data), -1))
		for _, stmt := range missingSearchLanguage(string(data), "messages") {
			t.Errorf("%s: %q does not set search_language", f, stmt)
		}
	}
	if inserts == 0 {
		t.Fatal("no INSERT found; the check reads the wrong files")
	}
	generated, err := filepath.Glob(filepath.Join("sqlcgen", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range generated {
		// The table models hold every column; since M1-57 the file is models.gen.go (INT-W22/5).
		if b := filepath.Base(f); b == "models.go" || b == "models.gen.go" {
			continue
		}
		data, err := os.ReadFile(f) // #nosec G304 -- a generated file of this package.
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "TitleSearch") || strings.Contains(string(data), "SearchVector") {
			t.Errorf("%s reads a search vector", f)
		}
	}
}
