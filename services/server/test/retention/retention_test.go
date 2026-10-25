// SPDX-License-Identifier: Apache-2.0

// Package retention_test holds the integration tests of brief M7-05: the real server of
// internal/wire in role all through test/harness on a fake clock, PostgreSQL from pgtest. The test
// takes the participant registry and the runner of the jobs retention.<class> from the job
// modules and runs the classes itself with a controlled clock (DP-T05, DP-T06); River never runs
// them within a test, because their interval is retention.job_interval.
package retention_test

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	auditjobs "github.com/OWNER/bothy/services/server/internal/audit/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// day is one day; the periods below are the defaults of 20 section 13 named by their keys.
const day = 24 * time.Hour

// world is one test server with the participants of the retention jobs.
type world struct {
	t   *testing.T
	s   *harness.Server
	clk *clock.Fake
	db  *pgxpool.Pool

	mu    sync.Mutex
	reg   *retention.Registry
	jobs  *wire.RetentionJobs
	chain auditjobs.Verifier
}

// newWorld starts the server with cfg on a fake clock.
func newWorld(t *testing.T, cfg map[string]string) *world {
	t.Helper()
	all := harness.RaisedRateLimits()
	for k, v := range cfg {
		all[k] = v
	}
	// The clock moves by days in the audit test; the clients sign their proofs with the real time.
	all["auth.clock_leeway"], all["auth.dpop_proof_max_age"] = "96h", "96h"
	all["audit.checkpoint_sink"] = "file://" + t.TempDir()
	w := &world{t: t, clk: clock.NewFake(time.Now().UTC().Truncate(time.Millisecond))}
	w.s = harness.Start(t, harness.Options{Config: all, Clock: w.clk, EgressDenyAll: true, JobModules: w.take})
	w.db = w.s.AdminDB()
	return w
}

// take keeps the participant registry, the runner and the chain verification of the server and
// gives River a no-op for files.purge_objects, so the purge a deleted attachment queues never
// runs during an assertion.
func (w *world) take(m *wire.JobModules) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.reg, w.jobs, w.chain = m.RetentionParticipants, m.Retention, m.ChainVerify
	m.FilesPurge = noPurge{}
}

// noPurge is a files.purge_objects that does nothing.
type noPurge struct{}

func (noPurge) Run(context.Context, ids.ID) error { return nil }

// run runs the job of class c with the clock at now, batches of two rows, and returns the rows
// it removed.
func (w *world) run(c retention.Class, now time.Time) int {
	w.t.Helper()
	w.mu.Lock()
	regs := w.reg.Of(c)
	w.mu.Unlock()
	res, err := retention.Run(context.Background(), regs, clock.NewFake(now), 2, time.Hour)
	if err != nil {
		w.t.Fatalf("retention.%s: %v", c, err)
	}
	if res.Exhausted {
		w.t.Fatalf("retention.%s ran out of its budget", c)
	}
	return res.Deleted
}

func (w *world) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.db.Exec(w.t.Context(), sql, args...); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
}

func (w *world) exists(table string, id any) bool {
	w.t.Helper()
	var ok bool
	if err := w.db.QueryRow(w.t.Context(), "SELECT EXISTS (SELECT 1 FROM "+table+" WHERE id = $1)", id).Scan(&ok); err != nil {
		w.t.Fatal(err)
	}
	return ok
}

func (w *world) count(sql string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.db.QueryRow(w.t.Context(), sql, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// mainThread returns a main thread the owner may write in: the first of the setup.
func (w *world) mainThread() string {
	w.t.Helper()
	mw := harness.BuildMatrixWorld(w.t, w.s)
	var th string
	if err := w.db.QueryRow(w.t.Context(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`,
		mw.Channels["Support/Allgemein"]).Scan(&th); err != nil {
		w.t.Fatal(err)
	}
	return th
}

// post posts text as the owner into thread and returns the message id.
func (w *world) post(thread, text string) string {
	w.t.Helper()
	var m struct {
		ID string `json:"id"`
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages",
		Body: map[string]any{"client_msg_id": ids.New().String(), "content_md": text}}, http.StatusCreated).JSON(w.t, &m)
	return m.ID
}

// verify runs verifyAuditChain as the owner with a fresh step-up.
func (w *world) verify() (ok bool, from, broken int64, reason string) {
	w.t.Helper()
	resp, err := harness.DoWithStepUp(w.t.Context(), w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/audit/verify", Body: map[string]any{}})
	if err != nil {
		w.t.Fatal(err)
	}
	var v struct {
		OK             bool   `json:"ok"`
		CheckedFromSeq int64  `json:"checked_from_seq"`
		FirstBrokenSeq int64  `json:"first_broken_seq"`
		FailureReason  string `json:"failure_reason"`
	}
	if resp.Status != http.StatusOK {
		w.t.Fatalf("verifyAuditChain: %d %s", resp.Status, resp.Body)
	}
	resp.JSON(w.t, &v)
	return v.OK, v.CheckedFromSeq, v.FirstBrokenSeq, v.FailureReason
}

// TestRetention_DPT05 (20 DP-T05): one scenario per class with a controlled clock; after the job
// only expired rows are gone. security: API tokens of the owner ended just before and just after
// now minus retention.security_after_end; content: a message created before and one after now
// minus retention.content; transient: dead webhook deliveries; audit: nothing is old enough, so
// the log stays whole; cost: closed price rows.
func TestRetention_DPT05(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"retention.content": "30d"})
	now := w.clk.Now()
	owner := w.s.Owner.PrincipalID

	token := func(end time.Time) ids.ID {
		id := ids.New()
		w.exec(`INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, scopes, expires_at, created_by)
			VALUES ($1, $2, 'ci', 'bothy_pat_EXAMPL', $3, '{}', $4, $2)`, id, owner, []byte(id.String()), end)
		return id
	}
	tokGone, tokKept := token(now.Add(-90*day-time.Second)), token(now.Add(-90*day+time.Second))

	thread := w.mainThread()
	msgGone, msgKept := w.post(thread, "canary-dpt05-old"), w.post(thread, "canary-dpt05-new")
	w.exec(`UPDATE messages SET created_at = $2 WHERE id = $1`, msgGone, now.Add(-30*day-time.Second))
	w.exec(`UPDATE messages SET created_at = $2 WHERE id = $1`, msgKept, now.Add(-30*day+time.Second))

	agent, sub := ids.New(), ids.New()
	w.exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', 'retention-helper', 'Helper')`, agent)
	w.exec(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'generic')`, agent)
	w.exec(`INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref, state)
		VALUES ('retention-test-wh', 'webhook_out', 'EdDSA', '{}', 'file:/run/keys/retention-test-wh', 'retiring')`)
	w.exec(`INSERT INTO webhook_subscriptions (id, agent_id, url, event_types, signing_key_kid, created_by)
		VALUES ($1, $2, 'https://hooks.example.com/in', '{message.created}', 'retention-test-wh', $2)`, sub, agent)
	delivery := func(seq int64, created time.Time) ids.ID {
		id := ids.New()
		w.exec(`INSERT INTO webhook_deliveries (id, subscription_id, event_seq, event_id, attempt, status, created_at)
			VALUES ($1, $2, $3, $4, 1, 'dead', $5)`, id, sub, seq, ids.New(), created)
		return id
	}
	delGone, delKept := delivery(1, now.Add(-30*day-time.Second)), delivery(2, now.Add(-30*day+time.Second))

	price := func(model string, to time.Time) ids.ID {
		id := ids.New()
		w.exec(`INSERT INTO price_table (id, provider, model, unit, price_micros, currency, valid_from, valid_to, source_url, retrieved_at, created_by)
			VALUES ($1, 'example', $2, 'input_mtok', 1, 'EUR', $3, $4, 'https://prices.example.com', $3, $5)`, id, model, to.Add(-day), to, owner)
		return id
	}
	priceGone, priceKept := price("model-a", now.Add(-395*day-time.Second)), price("model-b", now.Add(-395*day+time.Second))

	// The registry the server built holds one participant per table of the coverage test and
	// every class of the five jobs.
	w.mu.Lock()
	regs := w.reg.All()
	w.mu.Unlock()
	tables := map[string]bool{}
	for _, reg := range regs {
		for _, tb := range reg.Participant.Tables() {
			tables[tb] = true
		}
	}
	want := []string{"refresh_tokens", "refresh_token_families", "auth_sessions", "device_keys", "devices",
		"passkey_credentials", "api_tokens", "enrollment_codes", "server_signing_keys", "push_registrations", "terminal_sessions",
		"host_session_users", "hosts", "webhook_deliveries", "cost_entries", "price_table", "messages", "audit_events",
		"audit_checkpoints", "audit_ip_keys",
		// M7-09: the class ops of the status page and the backup runs.
		"status_incidents", "status_incident_updates", "status_incident_components", "backup_runs"}
	for _, tb := range want {
		if !tables[tb] {
			t.Errorf("the server registers no participant for %s", tb)
		}
	}
	if len(tables) != len(want) {
		t.Errorf("the server registers %d tables, want %d", len(tables), len(want))
	}

	auditRows := w.count(`SELECT count(*)::int FROM audit_events`)
	for _, c := range retention.JobClasses() {
		w.run(c, now)
	}
	for _, c := range []struct {
		table string
		id    any
		want  bool
	}{
		{"api_tokens", tokGone, false}, {"api_tokens", tokKept, true},
		{"webhook_deliveries", delGone, false}, {"webhook_deliveries", delKept, true},
		{"price_table", priceGone, false}, {"price_table", priceKept, true},
	} {
		if got := w.exists(c.table, c.id); got != c.want {
			t.Errorf("%s row present = %v, want %v", c.table, got, c.want)
		}
	}
	if n := w.count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NOT NULL AND content_md = ''`, msgGone); n != 1 {
		t.Error("the expired message is no tombstone")
	}
	if n := w.count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NULL AND content_md = 'canary-dpt05-new'`, msgKept); n != 1 {
		t.Error("the message inside the period changed")
	}
	if n := w.count(`SELECT count(*)::int FROM audit_events`); n < auditRows {
		t.Errorf("audit rows %d after the run, %d before: nothing was old enough", n, auditRows)
	}
}

// TestRetention_ContentNoneDeletesNothing (20 4.1): with retention.content = none the participant
// of messaging is registered without a period, so no message is deleted, however old.
func TestRetention_ContentNoneDeletesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"retention.content_none_reason": "kept as the team record"})
	msg := w.post(w.mainThread(), "canary-none")
	w.exec(`UPDATE messages SET created_at = created_at - interval '3650 days' WHERE id = $1`, msg)
	if n := w.run(retention.ClassContent, w.clk.Now()); n != 0 {
		t.Fatalf("content job removed %d rows with retention.content = none", n)
	}
	if n := w.count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NULL AND content_md = 'canary-none'`, msg); n != 1 {
		t.Fatal("the message changed")
	}
}

// TestRetention_ContentPeriodUsesDeletionPath (20 4.1, 4.2, 4.3): with a period an expired message
// is deleted through the deletion path: tombstone, no revisions, the payloads of its events
// redacted, one message.deleted without deleted_by, its attachment's file marked deleted and the
// purge job files.purge_objects queued.
func TestRetention_ContentPeriodUsesDeletionPath(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"retention.content": "30d"})
	thread := w.mainThread()
	msg := w.post(thread, "canary-period")
	var channel string
	if err := w.db.QueryRow(t.Context(), `SELECT channel_id::text FROM messages WHERE id = $1`, msg).Scan(&channel); err != nil {
		t.Fatal(err)
	}
	file := ids.New()
	w.exec(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, upload_state, scan_status, scope_channel_id)
		VALUES ($1::uuid, 'objects/'||$1::text, '\x00', 4, 'text/plain', 'notes.txt', $2, 'attachment', 'complete', 'clean', $3)`, file, w.s.Owner.PrincipalID, channel)
	w.exec(`INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, 0, $3, $4)`, msg, file, thread, channel)
	w.exec(`UPDATE messages SET has_attachments = true, created_at = created_at - interval '31 days' WHERE id = $1`, msg)

	if n := w.run(retention.ClassContent, w.clk.Now()); n != 1 {
		t.Fatalf("content job removed %d messages, want 1", n)
	}
	if n := w.count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NOT NULL AND deleted_by IS NULL
		AND content_md = '' AND NOT has_attachments`, msg); n != 1 {
		t.Error("the message is no tombstone without deleted_by")
	}
	if n := w.count(`SELECT count(*)::int FROM event_log WHERE resource_id = $1 AND payload::text LIKE '%canary-period%'`, msg); n != 0 {
		t.Errorf("%d events still hold the content", n)
	}
	if n := w.count(`SELECT count(*)::int FROM event_log WHERE resource_id = $1 AND type = 'message.deleted' AND NOT payload ? 'deleted_by'`, msg); n != 1 {
		t.Errorf("message.deleted events without deleted_by: %d, want 1", n)
	}
	if n := w.count(`SELECT count(*)::int FROM files WHERE id = $1 AND deleted_at IS NOT NULL`, file); n != 1 {
		t.Error("the attached file is not marked deleted")
	}
	if n := w.count(`SELECT count(*)::int FROM jobs.river_job WHERE kind = 'files.purge_objects' AND args->>'file_id' = $1`, file.String()); n != 1 {
		t.Errorf("files.purge_objects jobs for the file: %d, want 1", n)
	}
	if n := w.count(`SELECT count(*)::int FROM audit_events WHERE action = 'message.delete'`); n != 0 {
		t.Errorf("retention deletion audited %d times", n)
	}
}

// TestRetention_AuditTrimVerifies (03 audit_events, 20 4.5, SEC-AUD-2): the trim runs as
// bothy_maint, writes the anchor entry and removes the rows before the newest checkpoint at
// audit.checkpoint_sink; the chain still verifies from the anchor against the sink, and a rewrite
// of a row after the anchor is still detected.
func TestRetention_AuditTrimVerifies(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"retention.audit": "1d"})
	if ok, _, _, reason := w.verify(); !ok {
		t.Fatalf("chain broken before the trim: %s", reason)
	}
	w.mu.Lock()
	chain := w.chain
	w.mu.Unlock()
	if err := chain.RunCheckpoint(t.Context()); err != nil {
		t.Fatal(err)
	}
	cp := int64(w.count(`SELECT max(seq)::int FROM audit_checkpoints WHERE exported_at IS NOT NULL`))
	if cp == 0 {
		t.Fatal("no checkpoint at the sink")
	}
	if ok, _, _, reason := w.verify(); !ok {
		t.Fatalf("chain broken after the checkpoint: %s", reason)
	}
	// retention.audit (1d here) passes for every row so far; the server's clock moves too, so a
	// checkpoint of the sink below the anchor is older than retention.audit when verify runs.
	w.clk.Advance(2 * day)
	removed := 0
	for {
		n := w.run(retention.ClassAudit, w.clk.Now())
		removed += n
		if n == 0 {
			break
		}
	}
	if removed == 0 {
		t.Fatal("the trim removed nothing")
	}
	first := int64(w.count(`SELECT min(seq)::int FROM audit_events`))
	if first != cp {
		t.Fatalf("first retained row %d, want the row of the checkpoint %d", first, cp)
	}
	var anchorSeq int64
	var anchorHash, hashBefore string
	if err := w.db.QueryRow(t.Context(), `SELECT (details->>'anchor_seq')::bigint, details->>'anchor_hash' FROM audit_events
		WHERE action = 'audit.retention_trim' ORDER BY seq DESC LIMIT 1`).Scan(&anchorSeq, &anchorHash); err != nil {
		t.Fatal(err)
	}
	if err := w.db.QueryRow(t.Context(), `SELECT encode(prev_hash, 'hex') FROM audit_events WHERE seq = $1`, first).Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	if anchorSeq != first-1 || anchorHash != hashBefore {
		t.Fatalf("anchor %d %s, want %d %s", anchorSeq, anchorHash, first-1, hashBefore)
	}
	if n := w.count(`SELECT count(*)::int FROM audit_checkpoints WHERE seq < $1`, first); n != 0 {
		t.Errorf("%d checkpoints below the anchor left", n)
	}
	ok, from, _, reason := w.verify()
	if !ok || from != first {
		t.Fatalf("verify after the trim: ok %v from %d (%s), want ok from %d", ok, from, reason, first)
	}

	conn, err := w.db.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	for _, sql := range []string{
		`ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete`,
		`UPDATE audit_events SET details = '{"rewritten":true}' WHERE seq = (SELECT min(seq) + 1 FROM audit_events)`,
		`ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update_delete`,
	} {
		if _, err := conn.Exec(t.Context(), sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	ok, _, broken, reason := w.verify()
	if ok || broken != first+1 || reason != "hash_mismatch" {
		t.Fatalf("verify after a rewrite: ok %v broken %d %s, want hash_mismatch at %d", ok, broken, reason, first+1)
	}
}

// TestRetention_DPT06 (20 DP-T06, 03 audit_ip_keys): after privacy.audit_ip_key_retention the
// keys of the IP pseudonyms are destroyed: the key is gone, so no ip_hmac of its period can be
// recomputed for any IP, the row stays with destroyed_at, and the chain still verifies.
func TestRetention_DPT06(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	if ok, _, _, reason := w.verify(); !ok {
		t.Fatalf("chain broken: %s", reason)
	}
	keys := w.count(`SELECT count(*)::int FROM audit_ip_keys`)
	used := w.count(`SELECT count(DISTINCT details->>'ip_key_id')::int FROM audit_events WHERE details ? 'ip_hmac'`)
	if keys == 0 || used == 0 {
		t.Fatalf("no IP pseudonym to destroy: %d keys, %d used", keys, used)
	}
	// The periods of the keys end long ago (the server's clock stays, so the sessions of the
	// test stay valid): privacy.audit_ip_key_retention has passed for every key.
	w.exec(`UPDATE audit_ip_keys SET valid_from = valid_from - interval '400 days', valid_until = valid_until - interval '400 days'`)
	if n := w.run(retention.ClassAudit, w.clk.Now()); n < keys {
		t.Fatalf("the audit job removed or destroyed %d rows, want at least the %d keys", n, keys)
	}
	if n := w.count(`SELECT count(*)::int FROM audit_ip_keys WHERE key_ciphertext IS NULL AND destroyed_at IS NOT NULL`); n != keys {
		t.Fatalf("destroyed keys %d of %d; the rows must stay", n, keys)
	}
	if n := w.count(`SELECT count(*)::int FROM audit_events e WHERE e.details ? 'ip_hmac'
		AND EXISTS (SELECT 1 FROM audit_ip_keys k WHERE k.id = (e.details->>'ip_key_id')::uuid AND k.key_ciphertext IS NOT NULL)`); n != 0 {
		t.Fatalf("%d pseudonyms still have their key", n)
	}
	if ok, _, _, reason := w.verify(); !ok {
		t.Fatalf("chain broken after the destruction: %s", reason)
	}
}
