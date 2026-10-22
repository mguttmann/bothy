// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestEndingSQLOnlyInEndSessions (M2-46 Tests, unit; Design note 1): exactly one statement of
// queries/*.sql sets close_reason, EndTerminalSession in ending.sql, the closing statement of the
// one end function EndSessions. A second writer of close_reason would bypass the pair rule, the
// audit entry and the event of that function.
func TestEndingSQLOnlyInEndSessions(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no query files: %v", err)
	}
	var got []string
	for _, f := range files {
		b, err := os.ReadFile(f) //nolint:gosec // G304: the query files of the package, found by the glob above
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range closeReasonWriters(string(b)) {
			got = append(got, filepath.Base(f)+":"+name)
		}
	}
	if want := []string{"ending.sql:EndTerminalSession"}; !slices.Equal(got, want) {
		t.Fatalf("statements that set close_reason: %v, want %v", got, want)
	}

	// The detector itself: assignments in UPDATE SET, ON CONFLICT DO UPDATE SET and INSERT column
	// lists count; a comparison in WHERE or a SELECT does not.
	sample := `-- name: A :exec
UPDATE terminal_sessions SET state = 'closed',
    close_reason = 'x' WHERE id = $1;

-- name: B :exec
-- close_reason = 'only a comment'
UPDATE terminal_sessions SET version = version + 1 WHERE close_reason = 'logout';

-- name: C :exec
INSERT INTO terminal_sessions (id, state, close_reason) VALUES ($1, 'closed', 'logout');

-- name: D :many
SELECT close_reason FROM terminal_sessions WHERE close_reason = 'logout';

-- name: E :exec
INSERT INTO terminal_sessions (id) VALUES ($1) ON CONFLICT (id) DO UPDATE SET close_reason = 'x';
`
	if got, want := closeReasonWriters(sample), []string{"A", "C", "E"}; !slices.Equal(got, want) {
		t.Fatalf("detector on the sample: %v, want %v", got, want)
	}
}

// The parts of a statement the detector reads (lower case, comments removed, blanks folded).
var (
	sqlNameRe    = regexp.MustCompile(`(?m)^--\s*name:\s*(\w+)`)
	sqlCommentRe = regexp.MustCompile(`--[^\n]*`)
	sqlBlankRe   = regexp.MustCompile(`\s+`)
	sqlSetRe     = regexp.MustCompile(`\bset\b(.*?)(\bwhere\b|\breturning\b|\bfrom\b|;|$)`)
	sqlInsertRe  = regexp.MustCompile(`\binsert\s+into\s+[\w.]+\s*\(([^)]*)\)`)
	sqlAssignRe  = regexp.MustCompile(`\bclose_reason\s*=`)
	sqlColumnRe  = regexp.MustCompile(`\bclose_reason\b`)
)

// closeReasonWriters returns the sqlc names of the statements of src (one query file) that assign
// close_reason: in a SET clause of an UPDATE or an ON CONFLICT DO UPDATE, or in the column list of
// an INSERT. A statement is the text from its "-- name:" line to the next one.
func closeReasonWriters(src string) []string {
	var out []string
	marks := sqlNameRe.FindAllStringSubmatchIndex(src, -1)
	for i, m := range marks {
		end := len(src)
		if i+1 < len(marks) {
			end = marks[i+1][0]
		}
		name := src[m[2]:m[3]]
		body := sqlCommentRe.ReplaceAllString(src[m[1]:end], " ")
		body = strings.ToLower(sqlBlankRe.ReplaceAllString(body, " "))
		writes := false
		for _, set := range sqlSetRe.FindAllStringSubmatch(body, -1) {
			writes = writes || sqlAssignRe.MatchString(set[1])
		}
		for _, cols := range sqlInsertRe.FindAllStringSubmatch(body, -1) {
			writes = writes || sqlColumnRe.MatchString(cols[1])
		}
		if writes {
			out = append(out, name)
		}
	}
	return out
}
