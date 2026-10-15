// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// visiblePredicateAllowlist names the statements on messaging tables that legitimately lack
// channel_id = ANY(...), each with its reason (brief M1-57 note 8). M1-70 replaces this test with
// the repository-wide lint and its exception list.
var visiblePredicateAllowlist = map[string]string{
	"GetMessage": "internal lookup by id of sending (reply target, parent message) and of getMessage after the visible " +
		"read missed; the caller authorizes message.create, thread.create or message.read on the row and returns it to no reader",
	"GetMessageByClientID": "the idempotency lookup of the author's own (author, client_msg_id), 04 section 5.1 point 4; " +
		"the use case authorizes message.create on the thread before it answers with the row",
	"ListMentions": "the mentions of one message whose row the caller has already read with the visible predicate " +
		"or written itself, and authorized with message.read or message.create",
}

// messagingTables are the tables of the module (03 Grundsatz 12).
var messagingTables = regexp.MustCompile(`\b(messages|mentions|reactions|pins|message_revisions|message_templates)\b`)

// readOrChange matches the statements the rule covers.
var readOrChange = regexp.MustCompile(`(?i)^\s*(WITH|SELECT|UPDATE|DELETE)\b`)

// TestQueriesCarryVisibleChannelPredicate (07 section 7.4 rule 1, preparation for AZ-P12): every
// SELECT, UPDATE or DELETE statement of queries/*.sql that names a messaging table contains
// channel_id = ANY( or is in the allowlist; a stale allowlist entry fails too.
func TestQueriesCarryVisibleChannelPredicate(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no queries: %v", err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		b, err := os.ReadFile(f) //nolint:gosec // G304: f comes from the Glob of the queries directory above
		if err != nil {
			t.Fatal(err)
		}
		for name, stmt := range statements(string(b)) {
			seen[name] = true
			if !readOrChange.MatchString(stmt) || !messagingTables.MatchString(stmt) {
				continue
			}
			_, allowed := visiblePredicateAllowlist[name]
			has := strings.Contains(stmt, "channel_id = ANY(")
			if !has && !allowed {
				t.Errorf("%s: statement %s names a messaging table without channel_id = ANY( and is not in the allowlist", f, name)
			}
			if has && allowed {
				t.Errorf("%s: statement %s carries the predicate; remove it from the allowlist", f, name)
			}
		}
	}
	for name := range visiblePredicateAllowlist {
		if !seen[name] {
			t.Errorf("allowlist entry %s names no statement", name)
		}
	}
}

// statements splits a sqlc query file into its named statements without comment lines.
func statements(src string) map[string]string {
	out := map[string]string{}
	name := ""
	var body strings.Builder
	flush := func() {
		if name != "" {
			out[name] = body.String()
		}
		body.Reset()
	}
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "-- name: "); ok {
			flush()
			name = strings.Fields(after)[0]
			continue
		}
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		body.WriteString(line + "\n")
	}
	flush()
	return out
}
