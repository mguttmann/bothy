// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/wire"
)

// journalRetentionProblem is the start of the problem of the cross-check deletion_journal_retention.
const journalRetentionProblem = "privacy.deletion_journal_retention must be at least the longest backup period"

// TestDPT08_StartAbort (DP-T08, brief M7-07 requirement 15): a privacy.deletion_journal_retention
// shorter than the longest backup period fails bothy config check (exit 1) and the start of api
// and worker (exit 2, before any listener or database), whichever of the three periods is the
// longest: backup.logical.retention, backup.objects.retention, or the pgBackRest count of full
// backups times the interval of backup.pg.full_schedule. The problem names the keys and never a
// value; a retention of exactly the longest period passes.
func TestDPT08_StartAbort(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		short, exact []string
	}{
		{"logical longest", []string{"BOTHY_BACKUP__LOGICAL__RETENTION=500d", "BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=499d"},
			[]string{"BOTHY_BACKUP__LOGICAL__RETENTION=500d", "BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=500d"}},
		{"objects longest", []string{"BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=89d"},
			[]string{"BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=90d"}},
		{"full backups longest", []string{"BOTHY_BACKUP__PG__RETENTION_FULL=60", "BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=419d"},
			[]string{"BOTHY_BACKUP__PG__RETENTION_FULL=60", "BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=420d"}},
		{"local full count", []string{"BOTHY_BACKUP__PG__LOCAL_RETENTION_FULL=60", "BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=419d"},
			[]string{"BOTHY_BACKUP__PG__LOCAL_RETENTION_FULL=60", "BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=420d"}},
		{"monthly full schedule", []string{"BOTHY_BACKUP__PG__FULL_SCHEDULE=0 2 1 * *", "BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=123d"},
			[]string{"BOTHY_BACKUP__PG__FULL_SCHEDULE=0 2 1 * *", "BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION=124d"}},
	}
	for _, c := range cases {
		code, stdout, stderr := runConfig(t, wire.ConfigSchema, c.short, "check")
		if code != 1 || stdout != "" || !strings.Contains(stderr, journalRetentionProblem) || strings.Contains(stderr, "499d") ||
			strings.Contains(stderr, "419d") {
			t.Fatalf("%s: config check exit %d, stdout %q, stderr %q", c.name, code, stdout, stderr)
		}
		for _, role := range []string{"api", "worker"} {
			var out, errOut bytes.Buffer
			got := serveCommand(context.Background(), []string{"--role=" + role}, withPublicURL(c.short), &out, &errOut, nil)
			if got != 2 || !strings.Contains(errOut.String(), journalRetentionProblem) {
				t.Fatalf("%s: serve --role=%s exit %d, stderr %q", c.name, role, got, errOut.String())
			}
		}
		if code, stdout, stderr := runConfig(t, wire.ConfigSchema, c.exact, "check"); code != 0 || stdout != "configuration ok\n" {
			t.Fatalf("%s exact: exit %d, stdout %q, stderr %q", c.name, code, stdout, stderr)
		}
	}
	// The defaults pass.
	if code, stdout, stderr := runConfig(t, wire.ConfigSchema, nil, "check"); code != 0 || stdout != "configuration ok\n" {
		t.Fatalf("defaults: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
