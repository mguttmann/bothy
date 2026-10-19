// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// codeLine is the only stdout of a setup command: the code in display form and a newline.
var codeLine = regexp.MustCompile(`^en_[0-9A-HJKMNP-TV-Z]{4}(-[0-9A-HJKMNP-TV-Z]{4}){7}\n$`)

// testPepper is the token pepper of serveTestEnv (32 bytes 0x7c as hex).
var testPepper = bytes.Repeat([]byte{0x7c}, 32)

// runSetup runs bothy setup in process with args and returns exit code, stdout and stderr.
func runSetup(t *testing.T, env []string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := setupCommand(context.Background(), args, env, &out, &errb, defaultSetupDeps())
	return code, out.String(), errb.String()
}

// count returns the integer result of query as the schema owner.
func count(t *testing.T, db *pgtest.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Pool(t, pg.RoleOwner).QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// storedCode reports whether the printed line is a code whose HMAC is stored with purpose.
func storedCode(t *testing.T, db *pgtest.DB, line, purpose string) bool {
	t.Helper()
	canonical, ok := domain.NormalizeEnrollmentCode(strings.TrimSpace(line))
	if !ok {
		return false
	}
	return count(t, db, `SELECT count(*) FROM enrollment_codes WHERE code_hash = $1 AND purpose = $2`, domain.EnrollmentCodeHash(testPepper, canonical), purpose) == 1
}

func TestBootstrapPrintsCodeOnce(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveTestEnv(t, db)
	c := startChild(t, env, "setup", "bootstrap-owner", "--handle", "alex", "--display-name", "Alex")
	if code := c.wait(t); code != 0 {
		t.Fatalf("exit %d; stderr %s", code, c.stderr)
	}
	out, errText := c.stdout.String(), c.stderr.String()
	if !codeLine.MatchString(out) || !storedCode(t, db, out, "first_login") {
		t.Fatalf("stdout %q, want exactly one line with the stored code", out)
	}
	canonical, _ := domain.NormalizeEnrollmentCode(strings.TrimSpace(out))
	if strings.Contains(errText, strings.TrimSpace(out)) || strings.Contains(errText, canonical) {
		t.Fatal("the code is on stderr")
	}
	if !strings.Contains(errText, `"msg":"enrollment code issued"`) || !strings.Contains(errText, "valid until ") || strings.Contains(errText, `"alex"`) {
		t.Fatalf("stderr %s: want the info line without the handle", errText)
	}
	r := startChild(t, env, "setup", "issue-enrollment-code", "--user", "alex")
	if code := r.wait(t); code != 0 {
		t.Fatalf("recovery exit %d; stderr %s", code, r.stderr)
	}
	rout := r.stdout.String()
	if !codeLine.MatchString(rout) || !storedCode(t, db, rout, "recovery") || strings.Contains(r.stderr.String(), strings.TrimSpace(rout)) {
		t.Fatalf("recovery stdout %q stderr %s", rout, r.stderr)
	}
}

// tableCounts returns the row counts the bootstrap writes and the instance row as text.
func tableCounts(t *testing.T, db *pgtest.DB) string {
	t.Helper()
	var s string
	if err := db.Pool(t, pg.RoleOwner).QueryRow(context.Background(), `SELECT concat_ws(',',
		(SELECT count(*) FROM principals), (SELECT count(*) FROM owners), (SELECT count(*) FROM user_identities),
		(SELECT count(*) FROM identity_providers), (SELECT count(*) FROM enrollment_codes), (SELECT count(*) FROM audit_events),
		(SELECT row(instance.*)::text FROM instance))`).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIDT07_SecondBootstrapAbortsWithoutChange(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveTestEnv(t, db)
	if code, out, errText := runSetup(t, env, "bootstrap-owner", "--handle", "alex", "--display-name", "Alex"); code != 0 || !codeLine.MatchString(out) {
		t.Fatalf("first run: exit %d, stdout %q, stderr %s", code, out, errText)
	}
	before := tableCounts(t, db)
	code, out, errText := runSetup(t, env, "bootstrap-owner", "--handle", "sam", "--display-name", "Sam")
	if code != exitSetupComplete || out != "" || !strings.Contains(errText, "already complete") {
		t.Fatalf("second run: exit %d, stdout %q, stderr %s", code, out, errText)
	}
	if after := tableCounts(t, db); after != before {
		t.Fatalf("rows changed: %s, before %s", after, before)
	}
}

func TestBootstrapConcurrent(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveTestEnv(t, db)
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i, h := range []string{"alex", "sam"} {
		wg.Go(func() {
			codes[i], _, _ = runSetup(t, env, "bootstrap-owner", "--handle", h, "--display-name", h)
		})
	}
	wg.Wait()
	if (codes[0] != 0 || codes[1] != exitSetupComplete) && (codes[1] != 0 || codes[0] != exitSetupComplete) {
		t.Fatalf("exit codes %v, want one 0 and one %d", codes, exitSetupComplete)
	}
	if n := count(t, db, `SELECT count(*) FROM owners`); n != 1 {
		t.Fatalf("%d owners, want 1", n)
	}
}

func TestRecoveryCommand(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveTestEnv(t, db)
	if code, out, errText := runSetup(t, env, "issue-enrollment-code", "--user", "alex"); code != exitSetupPending || out != "" {
		t.Fatalf("before setup: exit %d, stdout %q, stderr %s", code, out, errText)
	}
	if code, _, errText := runSetup(t, env, "bootstrap-owner", "--handle", "alex", "--display-name", "Alex"); code != 0 {
		t.Fatalf("bootstrap: exit %d, %s", code, errText)
	}
	owner := db.Pool(t, pg.RoleOwner)
	if _, err := owner.Exec(context.Background(), `INSERT INTO principals (id, kind, handle, display_name) VALUES ('0192f4a0-7b1c-7f89-8d6b-6960453241bc', 'user', 'sam', 'Sam')`); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(context.Background(), `INSERT INTO users (principal_id) VALUES ('0192f4a0-7b1c-7f89-8d6b-6960453241bc')`); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"sam", "nobody"} {
		if code, out, errText := runSetup(t, env, "issue-enrollment-code", "--user", h); code != exitUsage || out != "" || !strings.Contains(errText, "not an active owner") {
			t.Fatalf("%s: exit %d, stdout %q, stderr %s", h, code, out, errText)
		}
	}
	code, out, errText := runSetup(t, env, "issue-enrollment-code", "--user", "alex")
	if code != 0 || !storedCode(t, db, out, "recovery") {
		t.Fatalf("owner: exit %d, stdout %q, stderr %s", code, out, errText)
	}
	if n := count(t, db, `SELECT count(*) FROM audit_events WHERE action = 'owner.recover' AND actor_principal_id IS NULL AND details->>'purpose' = 'recovery'`); n != 1 {
		t.Fatalf("%d owner.recover entries, want 1", n)
	}
	// The older first_login code ended with the new one.
	if n := count(t, db, `SELECT count(*) FROM enrollment_codes WHERE used_at IS NULL AND expires_at > now()`); n != 1 {
		t.Fatalf("%d valid codes, want 1", n)
	}
}

func TestSetupCommandUsage(t *testing.T) {
	t.Parallel()
	env := []string{testPublicURL}
	for _, c := range []struct {
		args []string
		code int
		text string
	}{
		{nil, exitUsage, "Usage: bothy setup"},
		{[]string{"nope"}, exitUsage, "unknown subcommand"},
		{[]string{"bootstrap-owner", "--display-name", "Alex"}, exitUsage, "--handle is required"},
		{[]string{"bootstrap-owner", "--handle", "alex"}, exitUsage, "--display-name is required"},
		{[]string{"issue-enrollment-code"}, exitUsage, "--user is required"},
		{[]string{"bootstrap-owner", "--handle", "alex", "--display-name", "Alex", "extra"}, exitUsage, "unexpected argument"},
		{[]string{"bootstrap-owner", "--handle", "alex", "--display-name", "Alex", "--config="}, exitUsage, "--config needs a file path"},
	} {
		code, out, errText := runSetup(t, env, c.args...)
		if code != c.code || out != "" || !strings.Contains(errText, c.text) {
			t.Errorf("%v: exit %d, stdout %q, stderr %s", c.args, code, out, errText)
		}
	}
	for _, help := range [][]string{{"--help"}, {"bootstrap-owner", "-h"}} {
		if code, out, _ := runSetup(t, env, help...); code != exitOK || !strings.HasPrefix(out, "Usage: bothy setup") {
			t.Errorf("%v: exit %d", help, code)
		}
	}
}

func TestSetupInvalidArgumentsAndInfrastructure(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveTestEnv(t, db)
	for _, args := range [][]string{
		{"bootstrap-owner", "--handle", "Alex", "--display-name", "Alex"},
		{"bootstrap-owner", "--handle", "alex", "--display-name", "Alex", "--time-zone", "Local"},
		{"bootstrap-owner", "--handle", "alex", "--display-name", "Alex", "--locale", "not a tag"},
	} {
		if code, out, errText := runSetup(t, env, args...); code != exitUsage || out != "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %s", args, code, out, errText)
		}
	}
	if n := count(t, db, `SELECT count(*) FROM principals`); n != 0 {
		t.Fatal("invalid arguments created a principal")
	}
	// Without the token pepper the command fails before any change (exit 1).
	bad := serveTestEnv(t, db, "BOTHY_AUTH__TOKEN_PEPPER_REF=file:<secrets.dir>/missing_pepper")
	if code, out, errText := runSetup(t, bad, "bootstrap-owner", "--handle", "alex", "--display-name", "Alex"); code != exitFailure || out != "" {
		t.Fatalf("missing pepper: exit %d, stdout %q, stderr %s", code, out, errText)
	}
	if code, _, _ := runSetup(t, []string{testPublicURL, "BOTHY_STARTUP__DEPENDENCY_WAIT=nonsense"}, "bootstrap-owner", "--handle", "alex", "--display-name", "Alex"); code != exitFailure {
		t.Fatalf("bad configuration: exit %d, want %d", code, exitFailure)
	}
}

func TestBootstrapStdoutFailureExits1(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveTestEnv(t, db)
	var errb bytes.Buffer
	code := setupCommand(context.Background(), []string{"bootstrap-owner", "--handle", "alex", "--display-name", "Alex"}, env, failingWriter{}, &errb, defaultSetupDeps())
	if code != exitFailure || !strings.Contains(errb.String(), "issue-enrollment-code") {
		t.Fatalf("exit %d, stderr %s", code, errb.String())
	}
}
