// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestPurgeOnlyExpired (M7-05, DP-T05 and 20 section 4.2): the participant of retention.content
// deletes the message created before the cutoff through the deletion path (empty content,
// deleted_at without deleted_by, revisions gone, the payload of message.created redacted, one
// message.deleted without deleted_by, no audit entry) and leaves the messages created at and after
// the cutoff untouched.
func TestPurgeOnlyExpired(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, old := world(t)
	w.Want(patch(w, "alex", old.id, v(old.version), content("canary-retention-edit")), http.StatusOK, "")
	w.Clock.Advance(time.Hour)
	cutoff := w.Clock.Now()
	at := post(w, "alex", main, "canary-retention-at")
	w.Clock.Advance(time.Hour)
	after := post(w, "alex", main, "canary-retention-after")

	p := w.Svc.Retention()
	n, more, err := p.Purge(context.Background(), cutoff, 10)
	if err != nil || n != 1 || more {
		t.Fatalf("Purge = %d, %v, %v; want 1, false, nil", n, more, err)
	}
	if n, more, err := p.Purge(context.Background(), cutoff, 10); err != nil || n != 0 || more {
		t.Fatalf("second Purge = %d, %v, %v; want nothing", n, more, err)
	}
	var contentMD string
	var deleted, deletedBy bool
	if err := w.Owner.QueryRow(t.Context(), `SELECT content_md, deleted_at IS NOT NULL, deleted_by IS NOT NULL FROM messages WHERE id = $1`,
		old.id).Scan(&contentMD, &deleted, &deletedBy); err != nil {
		t.Fatal(err)
	}
	if contentMD != "" || !deleted || deletedBy {
		t.Fatalf("expired message: content %q, deleted %v, deleted_by set %v", contentMD, deleted, deletedBy)
	}
	if n := w.Count(`SELECT count(*)::int FROM message_revisions WHERE message_id = $1`, old.id); n != 0 {
		t.Errorf("revisions left: %d", n)
	}
	if n := w.Count(`SELECT count(*)::int FROM event_log WHERE resource_id = $1 AND payload::text LIKE '%canary-retention%'`, old.id); n != 0 {
		t.Errorf("event payloads of the expired message still hold its content: %d", n)
	}
	evs := w.Events("message.deleted")
	if len(evs) != 1 || evs[0].ResourceID != old.id {
		t.Fatalf("message.deleted events %+v", evs)
	}
	if _, ok := evs[0].Payload["deleted_by"]; ok {
		t.Errorf("message.deleted of the retention job names deleted_by: %v", evs[0].Payload)
	}
	validate(t, "MessageDeletedPayload", evs[0].Raw)
	w.FlushAudit()
	if n := w.Count(`SELECT count(*)::int FROM audit_events WHERE action = 'message.delete'`); n != 0 {
		t.Errorf("retention deletion audited: %d", n)
	}
	for _, m := range []sent{at, after} {
		var c string
		if err := w.Owner.QueryRow(t.Context(), `SELECT content_md FROM messages WHERE id = $1 AND deleted_at IS NULL`, m.id).Scan(&c); err != nil {
			t.Fatalf("message %s: %v", m.id, err)
		}
		if !strings.HasPrefix(c, "canary-retention-") {
			t.Errorf("message %s content %q", m.id, c)
		}
	}
}
