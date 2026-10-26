// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	gosync "sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/api/rest"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	statusdomain "github.com/OWNER/bothy/services/server/internal/status/domain"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
)

// emptyJournal is a deletion journal without objects: the atomicity test of M7-04 injects its
// failure after the journal.
type emptyJournal struct{}

func (emptyJournal) Apply(context.Context) (instance.JournalApplied, error) {
	return instance.JournalApplied{Next: 1}, nil
}

func (emptyJournal) StoreNext(context.Context, int64) error { return nil }

// restoreRow is the part of the instance row bothy restore finalize changes.
type restoreRow struct {
	epoch, system string
	timeline      int64
}

// superConns holds one superuser connection per database of these tests: db.SuperConn opens a
// new connection per call that lives until the test ends, and the shared test container has a
// bounded number of slots.
var superConns gosync.Map

// superOf returns the superuser connection of db for t, opened once per database.
func superOf(t *testing.T, db *pgtest.DB) *pgx.Conn {
	t.Helper()
	if c, ok := superConns.Load(db.Name()); ok {
		return c.(*pgx.Conn)
	}
	c := db.SuperConn(t)
	superConns.Store(db.Name(), c)
	t.Cleanup(func() { superConns.Delete(db.Name()) })
	return c
}

// readRestoreRow reads epoch and stored identity of db as the superuser.
func readRestoreRow(t *testing.T, db *pgtest.DB) restoreRow {
	t.Helper()
	var r restoreRow
	if err := superOf(t, db).QueryRow(context.Background(), `SELECT sync_epoch::text, coalesce(pg_system_identifier, ''),
		coalesce(pg_timeline_id, 0) FROM instance`).Scan(&r.epoch, &r.system, &r.timeline); err != nil {
		t.Fatal(err)
	}
	return r
}

// clusterOf reads the identity of the test cluster.
func clusterOf(t *testing.T, db *pgtest.DB) restoreRow {
	t.Helper()
	var r restoreRow
	if err := superOf(t, db).QueryRow(context.Background(), `SELECT s.system_identifier::text, c.timeline_id::bigint
		FROM pg_control_system() s, pg_control_checkpoint() c`).Scan(&r.system, &r.timeline); err != nil {
		t.Fatal(err)
	}
	return r
}

// superExec runs a statement as the superuser.
func superExec(t *testing.T, db *pgtest.DB, stmt string, args ...any) {
	t.Helper()
	if _, err := superOf(t, db).Exec(context.Background(), stmt, args...); err != nil {
		t.Fatal(err)
	}
}

// simulateRestore makes the stored identity differ from the cluster as a point-in-time restore
// does: the restored database still names the timeline of its backup, the cluster a newer one.
func simulateRestore(t *testing.T, db *pgtest.DB) {
	t.Helper()
	superExec(t, db, `UPDATE instance SET pg_timeline_id = pg_timeline_id + 1`)
}

// ownerEnv is the environment of bothy restore finalize against db: the configuration of the
// server with database.user = bothy_owner, as the migrate container of the Compose stack runs it.
func ownerEnv(t *testing.T, db *pgtest.DB) []string {
	t.Helper()
	return serveEnv(t, db, "BOTHY_DATABASE__USER=bothy_owner",
		"BOTHY_DATABASE__PASSWORD_REF=file:"+filepath.Join(db.SecretsDir(), "db_owner_password"))
}

// waitNoServerSessions waits until no session of bothy_app is connected to db: a closed pool or a
// stopped process does not wait for its backends to end, and bothy restore finalize counts them.
func waitNoServerSessions(t *testing.T, db *pgtest.DB) {
	t.Helper()
	testkit.Until(t, "no bothy_app session connected", func() bool {
		var n int
		if err := superOf(t, db).QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity
			WHERE usename = 'bothy_app' AND datname = current_database()`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 0
	})
}

// finalizeRestore runs bothy restore finalize through wire.NewRestoreFinalize, the assembly of the
// command; manifest nil is a restore, else a relocation. It first waits until the bothy_app
// sessions of earlier processes and pools have ended.
func finalizeRestore(t *testing.T, db *pgtest.DB, manifest []byte) (wire.RestoreResult, error) {
	t.Helper()
	waitNoServerSessions(t, db)
	ctx := context.Background()
	env := ownerEnv(t, db)
	r, err := wire.NewRestoreFinalize(ctx, loadEnv(t, env), env, nil)
	if err != nil {
		t.Fatalf("NewRestoreFinalize: %v", err)
	}
	defer r.Close(ctx)
	return r.Finalize(ctx, manifest)
}

// finalizeWithoutWait runs bothy restore finalize like finalizeRestore while the sessions of the
// caller stay connected.
// The refusal of a connected server comes from NewRestoreFinalize already, before any module
// starts (M7-07), or from Finalize when a server connects after the start.
func finalizeWithoutWait(t *testing.T, db *pgtest.DB) (wire.RestoreResult, error) {
	t.Helper()
	ctx := context.Background()
	env := ownerEnv(t, db)
	r, err := wire.NewRestoreFinalize(ctx, loadEnv(t, env), env, nil)
	if err != nil {
		return wire.RestoreResult{}, err
	}
	defer r.Close(ctx)
	return r.Finalize(ctx, nil)
}

// stopProcess stops a serve process like docker compose stop and waits for its exit.
func stopProcess(t *testing.T, p *process) {
	t.Helper()
	p.sigs <- syscall.SIGTERM
	if code := p.exit(t); code != 0 {
		t.Fatalf("stop: exit %d; logs:\n%s", code, p.logs)
	}
}

// seedCursors writes the cursor rows of every kind as the superuser, each at another value (two
// agent consumers, both worker consumers, a webhook subscription), and sets event_seq to head.
func seedCursors(t *testing.T, db *pgtest.DB, head int64) {
	t.Helper()
	agent, subscription, robin := ids.New(), ids.New(), ids.New()
	superExec(t, db, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, 'Robin')`, robin, "robin"+robin.String()[24:])
	superExec(t, db, `INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', $2, $2)`, agent, "agent"+agent.String()[24:])
	superExec(t, db, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent)
	superExec(t, db, `INSERT INTO agent_consumer_cursors (agent_id, consumer_name, acked_seq) VALUES ($1, 'default', 3), ($1, 'archive', 99)`, agent)
	superExec(t, db, `INSERT INTO event_consumer_cursors (name, last_seq) VALUES ('push', 5), ('webhooks', 50)
		ON CONFLICT (name) DO UPDATE SET last_seq = EXCLUDED.last_seq`)
	kid := "whk-" + subscription.String()[24:]
	superExec(t, db, `INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref, state)
		VALUES ($1, 'webhook_out', 'EdDSA', '{}', 'file:/run/whk', 'retired')`, kid)
	superExec(t, db, `INSERT INTO webhook_subscriptions (id, agent_id, url, event_types, signing_key_kid, created_by, last_delivered_seq)
		VALUES ($1, $2, 'https://hooks.example.com/chat', '{message.created}', $3, $4, 77)`, subscription, agent, kid, robin)
	superExec(t, db, `UPDATE sequence_counters SET value = $1 WHERE name = 'event_seq'`, head)
}

// cursors returns every cursor of the seed as "<kind>/<name>=<value>" in a fixed order.
func cursors(t *testing.T, db *pgtest.DB) []string {
	t.Helper()
	rows, err := superOf(t, db).Query(context.Background(), `
		SELECT 'agent/' || consumer_name || '=' || acked_seq FROM agent_consumer_cursors
		UNION ALL SELECT 'worker/' || name || '=' || last_seq FROM event_consumer_cursors
		UNION ALL SELECT 'webhook=' || last_delivered_seq FROM webhook_subscriptions
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// auditEntries returns the details of the audit rows of action, oldest first.
func auditEntries(t *testing.T, db *pgtest.DB, action string) []map[string]any {
	t.Helper()
	rows, err := superOf(t, db).Query(context.Background(), `SELECT details FROM audit_events WHERE action = $1 ORDER BY seq`, action)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var d map[string]any
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// failingCursors is a cursor port that always fails, the injected failure of the atomic test.
type failingCursors struct{}

// errInjected is the failure of failingCursors.
var errInjected = errors.New("injected failure of the last cursor port")

func (failingCursors) MoveCursorsTo(context.Context, int64) error { return errInjected }

// TestStart_RestoreNotFinalized_NotReady: a database whose identity differs from the stored one
// (a restore without bothy restore finalize) keeps every role running but not ready with only
// restore_not_finalized, serving and processing nothing (no public listener, log tail or worker
// consumers), and logs the runbook step; with database.restore_guard = off the server
// becomes ready; after bothy restore finalize it becomes ready with the guard on (R-DM-50,
// brief M7-04 requirement 14).
func TestStart_RestoreNotFinalized_NotReady(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	// An active terminal session of the previous process gives the reconcile of the start work:
	// it would detach the session with an event (M2-44); the lock holds it too (D-M7-04-10).
	session := seedActiveTerminalSession(t, db)
	simulateRestore(t, db)
	var heads [2]int64
	// presenceStarts counts the starts of the presence tracker (M2-20), whose start rule and flush
	// loop write the agents table: the lock holds them too (D-M7-04-7).
	var presenceStarts atomic.Int32
	gauges := func(func() []agentsapp.GaugeSample) error { presenceStarts.Add(1); return nil }
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
		p := start(t, role, serveEnv(t, db), wire.Deps{PresenceGauges: gauges})
		internal := p.waitInternal(t)
		testkit.Until(t, string(role)+": the start ended not ready", func() bool {
			return strings.Contains(p.logs.String(), `"msg":"server started, not ready"`)
		})
		// The database check may fail for a moment on a loaded host; the lock fails for good.
		want := `{"status":"unavailable","failed":["restore_not_finalized"]}` + "\n"
		testkit.Until(t, string(role)+": /readyz names only restore_not_finalized", func() bool {
			code, body := get(t, internal, "/readyz")
			return code == http.StatusServiceUnavailable && body == want
		})
		if logs := p.logs.String(); strings.Count(logs, "database identity differs from the stored one") != 1 ||
			!strings.Contains(logs, "bothy restore finalize") || strings.Contains(logs, `"msg":"server ready"`) {
			t.Fatalf("%s: want one log line with the runbook step and no ready line:\n%s", role, logs)
		}
		// The lock holds every step that serves or processes: no public listener, no log tail, no
		// worker consumers, no River client (the River leader would hold a connection).
		if p.logs.port("public listener started") != 0 {
			t.Fatalf("%s: the public listener opened while the restore is not finalized", role)
		}
		var listeners int
		if err := superOf(t, db).QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()
			AND application_name IN ('bothy-event-tail', 'bothy-consumer-wake')`).Scan(&listeners); err != nil {
			t.Fatal(err)
		}
		if listeners != 0 {
			t.Fatalf("%s: %d log tail or consumer connections while the restore is not finalized", role, listeners)
		}
		if n := presenceStarts.Load(); n != 0 {
			t.Fatalf("%s: the presence tracker started %d times while the restore is not finalized", role, n)
		}
		if state := terminalSessionState(t, db, session); state != "active" {
			t.Fatalf("%s: the terminal session is %s after a start under the lock, want active", role, state)
		}
		// No start under the lock writes an event or an audit entry; the first start may create
		// the signing keys of the database, so the heads count from its end.
		got := logHeads(t, db)
		if role != wire.RoleAPI && got != heads {
			t.Fatalf("%s: a start under the lock moved event_seq and audit_seq from %v to %v", role, heads, got)
		}
		heads = got
		stopProcess(t, p)
	}
	off := start(t, wire.RoleAll, serveEnv(t, db, "BOTHY_DATABASE__RESTORE_GUARD=off"), wire.Deps{})
	waitReadyz(t, off.waitInternal(t), http.StatusOK)
	stopProcess(t, off)
	// Without the lock the reconcile ran: the seeded session would have been detached by any start.
	if state := terminalSessionState(t, db, session); state != "detached" {
		t.Fatalf("the terminal session is %s after a start without the lock, want detached", state)
	}

	if _, err := finalizeRestore(t, db, nil); err != nil {
		t.Fatal(err)
	}
	p := start(t, wire.RoleAll, serveEnv(t, db), wire.Deps{PresenceGauges: gauges})
	waitReadyz(t, p.waitInternal(t), http.StatusOK)
	if p.logs.port("public listener started") == 0 {
		t.Fatal("no public listener after finalize")
	}
	if presenceStarts.Load() != 1 {
		t.Fatalf("presence tracker starts after finalize = %d, want 1", presenceStarts.Load())
	}
}

// seedActiveTerminalSession inserts a person, an active host assigned to them, a session user and
// an active terminal session as the schema owner, and returns the session.
func seedActiveTerminalSession(t *testing.T, db *pgtest.DB) ids.ID {
	t.Helper()
	ctx := context.Background()
	owner := db.Pool(t, pg.RoleOwner)
	person := ids.New()
	if _, err := owner.Exec(ctx, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`,
		person.String(), "alex"+strings.ReplaceAll(person.String()[24:], "-", "")); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO users (principal_id) VALUES ($1)`, person.String()); err != nil {
		t.Fatal(err)
	}
	host, err := terminaltest.InsertHost(ctx, owner, terminaltest.Host{Name: "Lab", State: "active", AssignedUserID: person,
		CreatedBy: person, ReportedSessionUsers: []string{"bothy-s-alex"}})
	if err != nil {
		t.Fatal(err)
	}
	su, err := terminaltest.InsertSessionUser(ctx, owner, terminaltest.SessionUser{HostID: host, UserID: person, OSUser: "bothy-s-alex", CreatedBy: person})
	if err != nil {
		t.Fatal(err)
	}
	session, err := terminaltest.InsertSession(ctx, owner, terminaltest.Session{HostID: host, UserID: person, SessionUserID: su, State: "active"})
	if err != nil {
		t.Fatal(err)
	}
	// The partition of the next events, which the maintenance of a running server would create:
	// the reconcile runs before it.
	superExec(t, db, "SELECT sync_event_log_ensure_partitions(1000, 1, 1)")
	return session
}

// terminalSessionState reads the state of a terminal session as the superuser.
func terminalSessionState(t *testing.T, db *pgtest.DB, id ids.ID) string {
	t.Helper()
	var state string
	if err := superOf(t, db).QueryRow(context.Background(), `SELECT state FROM terminal_sessions WHERE id = $1`, id.String()).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

// logHeads reads event_seq and audit_seq, the heads of the event log and of the audit chain.
func logHeads(t *testing.T, db *pgtest.DB) [2]int64 {
	t.Helper()
	var h [2]int64
	if err := superOf(t, db).QueryRow(context.Background(), `SELECT
		(SELECT value FROM sequence_counters WHERE name = 'event_seq'),
		(SELECT value FROM sequence_counters WHERE name = 'audit_seq')`).Scan(&h[0], &h[1]); err != nil {
		t.Fatal(err)
	}
	return h
}

// TestStart_EmptyStoredIdentityNeverBlocks: an instance older than M7-04 has no stored identity
// until the next bothy migrate up; it becomes ready.
func TestStart_EmptyStoredIdentityNeverBlocks(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	superExec(t, db, `UPDATE instance SET pg_system_identifier = NULL, pg_timeline_id = NULL`)
	p := start(t, wire.RoleAll, serveEnv(t, db), wire.Deps{})
	waitReadyz(t, p.waitInternal(t), http.StatusOK)
}

// TestFinalizeRestore_Atomic (brief M7-04 design note 2): with a failure injected in the last
// cursor port nothing changed, neither the epoch nor a cursor nor the identity nor the audit log;
// then bothy restore finalize sets a new UUIDv7 epoch, every cursor kind to head_seq, the
// identity of the cluster and exactly one audit entry.
func TestFinalizeRestore_Atomic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newServeDB(t)
	const head = 40
	seedCursors(t, db, head)
	simulateRestore(t, db)
	before, beforeCursors := readRestoreRow(t, db), cursors(t, db)
	if want := []string{"agent/archive=99", "agent/default=3", "webhook=77", "worker/push=5", "worker/webhooks=50"}; strings.Join(beforeCursors, " ") != strings.Join(want, " ") {
		t.Fatalf("seed %v, want %v", beforeCursors, want)
	}

	owner := db.Pool(t, pg.RoleOwner)
	a, err := wire.NewAudit(owner, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute},
		app.Config{DenyAggregateWindow: time.Minute, DenyAggregateMaxKeys: 10, IPKeyPeriod: 24 * time.Hour, IPKeyRetention: 48 * time.Hour,
			IPKEKRef: "file:<secrets.dir>/audit_ip_kek"}, testKEK{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	failing, err := wire.NewRestoreService(owner, a, append(wire.RestoreCursors(owner), instanceapp.CursorMover(failingCursors{})), emptyJournal{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failing.Finalize(ctx, instance.FinalizeRestore{}); !errors.Is(err, errInjected) {
		t.Fatalf("Finalize with a failing last port = %v, want the injected error", err)
	}
	if got := readRestoreRow(t, db); got != before {
		t.Fatalf("instance row changed after the failure: %+v, was %+v", got, before)
	}
	if got := cursors(t, db); strings.Join(got, " ") != strings.Join(beforeCursors, " ") {
		t.Fatalf("cursors changed after the failure: %v, were %v", got, beforeCursors)
	}
	if n := len(auditEntries(t, db, instance.ActionRestoreFinalize)); n != 0 {
		t.Fatalf("%d audit entries after the failure", n)
	}

	res, err := finalizeRestore(t, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	after, cluster := readRestoreRow(t, db), clusterOf(t, db)
	epoch, err := ids.Parse(after.epoch)
	if err != nil || epoch.String() == before.epoch || epoch[6]>>4 != 7 || res.SyncEpoch != epoch || res.PreviousSyncEpoch.String() != before.epoch ||
		res.Relocation || res.HeadSeq != head {
		t.Fatalf("epoch %s (was %s), result %+v", after.epoch, before.epoch, res)
	}
	if after.system != cluster.system || after.timeline != cluster.timeline || res.Identity.SystemIdentifier != cluster.system {
		t.Fatalf("stored identity %+v, cluster %+v", after, cluster)
	}
	want := []string{"agent/archive=40", "agent/default=40", "webhook=40", "worker/push=40", "worker/webhooks=40"}
	if got := cursors(t, db); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("cursors %v, want %v", got, want)
	}
	entries := auditEntries(t, db, instance.ActionRestoreFinalize)
	if len(entries) != 1 || entries[0]["head_seq"] != float64(head) || entries[0]["sync_epoch"] != after.epoch ||
		entries[0]["previous_sync_epoch"] != before.epoch || entries[0]["pg_system_identifier"] != cluster.system {
		t.Fatalf("audit entries %v", entries)
	}
}

// TestFinalizeRestore_RefusesWithConnectedApp: an open bothy_app session on the database makes
// bothy restore finalize refuse without a change (brief M7-04 requirement 16).
func TestFinalizeRestore_RefusesWithConnectedApp(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	simulateRestore(t, db)
	before := readRestoreRow(t, db)
	appPool := db.Pool(t, pg.RoleApp)
	if _, err := appPool.Exec(context.Background(), `SELECT 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := finalizeWithoutWait(t, db); !errors.Is(err, wire.ErrServerConnected) {
		t.Fatalf("Finalize = %v, want ErrServerConnected", err)
	}
	if got := readRestoreRow(t, db); got != before {
		t.Fatalf("instance row changed: %+v, was %+v", got, before)
	}
	if n := len(auditEntries(t, db, instance.ActionRestoreFinalize)); n != 0 {
		t.Fatalf("%d audit entries after the refusal", n)
	}
	appPool.Close()
	if _, err := finalizeRestore(t, db, nil); err != nil {
		t.Fatalf("after the session ended: %v", err)
	}
}

// restart stops the api process of w and closes the bothy_app pool of its side identity, the
// state in which bothy restore finalize runs; restartAPI starts a new api process.
func (w *gwWorld) stopForRestore(t *testing.T) {
	t.Helper()
	stopProcess(t, w.p)
	w.app.Close()
}

// startAPI starts a new api process of w on its database and returns it once /readyz answers code.
func (w *gwWorld) startAPI(t *testing.T, code int) {
	t.Helper()
	w.p = start(t, wire.RoleAPI, w.env, wire.Deps{})
	waitReadyz(t, w.p.waitInternal(t), code)
	w.base = "http://127.0.0.1:" + strconv.Itoa(w.p.logs.port("public listener started"))
}

// appendEvents appends n events after the head as the superuser, raises event_seq with them and
// returns the new head (the writes of the new timeline).
func appendEvents(t *testing.T, db *pgtest.DB, n int) int64 {
	t.Helper()
	var head int64
	if err := superOf(t, db).QueryRow(context.Background(), `SELECT value FROM sequence_counters WHERE name = 'event_seq'`).Scan(&head); err != nil {
		t.Fatal(err)
	}
	super := superOf(t, db)
	ctx := context.Background()
	for range n {
		head++
		if _, err := super.Exec(ctx, "SELECT sync_event_log_ensure_partitions(1000, 1, $1::bigint)", head); err != nil {
			t.Fatal(err)
		}
		if _, err := super.Exec(ctx, `INSERT INTO event_log (seq, id, type, resource_kind, receive_action, payload, permission_version)
			VALUES ($1::bigint, gen_random_uuid(), 'settings.updated', 'user', 'settings.read', '{}'::jsonb, 0)`, head); err != nil {
			t.Fatal(err)
		}
		if _, err := super.Exec(ctx, "SELECT pg_notify('event_log', $1::bigint::text)", head); err != nil {
			t.Fatal(err)
		}
	}
	superExec(t, db, `UPDATE sequence_counters SET value = $1 WHERE name = 'event_seq'`, head)
	return head
}

// listEvents calls GET /api/v1/events as p with a fresh DPoP proof and returns status and body.
func (w *gwWorld) listEvents(t *testing.T, p gwPerson, after int64, epoch string) (int, map[string]any) {
	t.Helper()
	path := "/api/v1/events"
	req, err := http.NewRequestWithContext(testkit.Context(t), http.MethodGet, w.base+path+"?after="+strconv.FormatInt(after, 10)+"&epoch="+epoch, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "DPoP "+p.access)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{HTM: http.MethodGet, HTU: gwPublicURL + path,
		IAT: time.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(p.access)}))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatalf("listEvents %d %s: %v", resp.StatusCode, b, err)
	}
	return resp.StatusCode, body
}

// nextOp returns the next frame of c within d, "" when none came.
func nextOp(c *wstest.Client, d time.Duration) (string, []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	op, data, err := c.Next(ctx)
	if err != nil {
		return "", nil
	}
	return op, data
}

// TestEpochChanged_AfterOvertake (ADR 0059, R-SY-18): a client holds a cursor of the old timeline
// ahead of the restored head; after bothy restore finalize new writes make head_seq larger than
// the cursor, so R-SY-04 alone would no longer catch it. The gateway (resume through identify with
// cursor and epoch) and the REST catch-up both answer resync_required with epoch_changed.
func TestEpochChanged_AfterOvertake(t *testing.T) {
	t.Parallel()
	w := newGatewayWorld(t)
	alex := w.newPerson("alex", identity.DeviceStateActive)
	c, r := w.ready(t, w.params(alex))
	c.Close()
	oldEpoch, cursor := r.SyncEpoch.String(), r.HeadSeq+3

	w.stopForRestore(t)
	res, err := finalizeRestore(t, w.db, nil)
	if err != nil {
		t.Fatal(err)
	}
	head := appendEvents(t, w.db, 5)
	if head <= cursor || res.SyncEpoch.String() == oldEpoch {
		t.Fatalf("head %d, cursor %d, epoch %s after %s", head, cursor, res.SyncEpoch, oldEpoch)
	}
	w.startAPI(t, http.StatusOK)

	params := w.params(alex)
	params.Cursor, params.SyncEpoch = &cursor, oldEpoch
	c2, r2 := w.ready(t, params)
	if r2.SyncEpoch.String() != res.SyncEpoch.String() || r2.HeadSeq < head {
		t.Fatalf("ready %+v, want epoch %s and head %d", r2, res.SyncEpoch, head)
	}
	op, data := nextOp(c2, testkit.Patience)
	var rr rest.GatewayResyncRequired
	if op != "resync_required" || wstest.Strict(data, &rr) != nil || rr.Reason != "epoch_changed" || rr.SyncEpoch != r2.SyncEpoch {
		t.Fatalf("after ready: %s %s", op, data)
	}

	code, body := w.listEvents(t, alex, cursor, oldEpoch)
	if code != http.StatusGone || body["code"] != "resync_required" || body["reason"] != "epoch_changed" {
		t.Fatalf("listEvents = %d %v", code, body)
	}
	if code, body := w.listEvents(t, alex, head, res.SyncEpoch.String()); code != http.StatusOK || body["sync_epoch"] != res.SyncEpoch.String() {
		t.Fatalf("listEvents with the new epoch = %d %v", code, body)
	}
}

// relocationManifest returns the manifest of a final logical backup of db as bothy-backupd writes
// it, with the head of the log, the audit head and the epoch read now; change edits it first.
func relocationManifest(t *testing.T, db *pgtest.DB, change func(m *statusdomain.Manifest)) []byte {
	t.Helper()
	ctx := context.Background()
	super := superOf(t, db)
	m := statusdomain.Manifest{
		Schema: statusdomain.ManifestSchema, BackupID: ids.New().String(), Kind: statusdomain.KindLogical,
		StartedAt: time.Now().Add(-time.Minute).UTC(), FinishedAt: time.Now().UTC(), ServerVersion: "0.0.0",
		RowCounts: map[string]int64{}, Restic: &statusdomain.ResticRef{SnapshotID: strings.Repeat("a", 64)},
	}
	if err := super.QueryRow(ctx, `SELECT instance_id::text, sync_epoch::text, schema_version FROM instance`).Scan(&m.InstanceID, &m.SyncEpoch, &m.SchemaVersion); err != nil {
		t.Fatal(err)
	}
	if err := super.QueryRow(ctx, `SELECT COALESCE(max(seq), 0)::bigint, COALESCE(min(seq), 0)::bigint FROM event_log`).Scan(&m.EventLog.HeadSeq, &m.EventLog.MinRetainedSeq); err != nil {
		t.Fatal(err)
	}
	if err := super.QueryRow(ctx, `SELECT COALESCE(max(seq), 0), COALESCE(encode((SELECT hash FROM audit_events ORDER BY seq DESC LIMIT 1), 'hex'), '')
		FROM audit_events`).Scan(&m.Audit.LastSeq, &m.Audit.LastHash); err != nil {
		t.Fatal(err)
	}
	if change != nil {
		change(&m)
	}
	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestRelocationKeepsEpoch (16 11, brief M7-04 design note 3): a relocation restores the final
// backup into another cluster, so the server is not ready; restore finalize --relocation with a
// manifest whose head_seq differs exits with the field and changes nothing; with the matching
// manifest it keeps epoch and cursors, stores the new identity, the server becomes ready and a
// client continues with its cursor without a resync.
func TestRelocationKeepsEpoch(t *testing.T) {
	t.Parallel()
	w := newGatewayWorld(t)
	alex := w.newPerson("alex", identity.DeviceStateActive)
	appendEvents(t, w.db, 3)
	c, r := w.ready(t, w.params(alex))
	c.Close()
	w.stopForRestore(t)
	seedCursors(t, w.db, r.HeadSeq)
	superExec(t, w.db, `UPDATE agent_consumer_cursors SET acked_seq = 2`)
	// The final backup of the old instance came from another cluster.
	superExec(t, w.db, `UPDATE instance SET pg_system_identifier = '1'`)
	beforeCursors := cursors(t, w.db)
	w.startAPI(t, http.StatusServiceUnavailable)
	w.stopForRestore(t)

	before := readRestoreRow(t, w.db)
	wrong := relocationManifest(t, w.db, func(m *statusdomain.Manifest) { m.EventLog.HeadSeq++ })
	_, err := finalizeRestore(t, w.db, wrong)
	var mismatch *wire.RelocationMismatchError
	if !errors.As(err, &mismatch) || mismatch.Field != "event_log.head_seq" {
		t.Fatalf("Finalize with another head_seq = %v", err)
	}
	if got := readRestoreRow(t, w.db); got != before {
		t.Fatalf("instance row changed: %+v, was %+v", got, before)
	}

	res, err := finalizeRestore(t, w.db, relocationManifest(t, w.db, nil))
	if err != nil {
		t.Fatal(err)
	}
	after, cluster := readRestoreRow(t, w.db), clusterOf(t, w.db)
	if !res.Relocation || after.epoch != before.epoch || res.SyncEpoch.String() != r.SyncEpoch.String() || after.system != cluster.system || after.timeline != cluster.timeline {
		t.Fatalf("result %+v, row %+v, cluster %+v", res, after, cluster)
	}
	if got := cursors(t, w.db); strings.Join(got, " ") != strings.Join(beforeCursors, " ") {
		t.Fatalf("cursors %v, were %v", got, beforeCursors)
	}
	if n := len(auditEntries(t, w.db, instance.ActionRelocationFinalize)); n != 1 || len(auditEntries(t, w.db, instance.ActionRestoreFinalize)) != 0 {
		t.Fatalf("%d relocation entries", n)
	}

	w.startAPI(t, http.StatusOK)
	params := w.params(alex)
	cursor := r.HeadSeq
	params.Cursor, params.SyncEpoch = &cursor, r.SyncEpoch.String()
	c2, r2 := w.ready(t, params)
	if r2.SyncEpoch != r.SyncEpoch {
		t.Fatalf("ready after the relocation %+v, want epoch %s", r2, r.SyncEpoch)
	}
	if op, data := nextOp(c2, 2*time.Second); op == "resync_required" {
		t.Fatalf("resync after a relocation: %s", data)
	}
}
