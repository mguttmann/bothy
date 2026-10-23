// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestResetAfterConcurrentReplace (09 9.2, design note 7 b): a reset that waits on the lock of
// the current row S while a concurrent change ends S and makes N current ends N; it never answers
// 409 no_current_session while a row is current. A writer as database owner holds S locked, ends it
// and inserts N, the reset is seen waiting, then the writer commits.
func TestResetAfterConcurrentReplace(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run := w.a6StartedRun(th, `"external_session_ref":"sess-replace-1"`)
	w.finishOK(run)
	s, ok := w.current(th)
	if !ok {
		t.Fatal("no current row")
	}
	ctx := context.Background()
	tx, err := w.db.Begin(ctx, pgx.TxOptions{})
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE agent_sessions SET is_current = false, ended_at = now(), end_reason = 'rebind',
		version = version + 1 WHERE id = $1`, s.ID)
	must(t, err)
	n := ids.New().String()
	_, err = tx.Exec(ctx, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, started)
		VALUES ($1, $2, $3, 'sess-replace-2', true)`, n, s.Thread, s.Agent)
	must(t, err)
	var writer int64
	must(t, tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&writer))
	done := make(chan *httpservertest.Response, 1)
	go func() { done <- w.reset("alex", th) }()
	w.waitBlockedBy(writer)
	must(t, tx.Commit(ctx))
	r := testkit.Receive(t, "the answer of the reset", done)
	body := w.want(r, http.StatusCreated, "")
	if body["id"] != n || body["end_reason"] != "reset" {
		t.Fatalf("reset %v, want the replacing row %s ended with reset", body, n)
	}
	if _, ok := w.current(th); ok {
		t.Fatal("a row is still current after the reset")
	}
}

// waitBlockedBy waits until a backend waits on a lock that the backend pid holds.
func (w *world) waitBlockedBy(pid int64) {
	w.t.Helper()
	testkit.Until(w.t, "a session waits on a lock of the backend", func() bool {
		return w.count(`SELECT count(*) FROM pg_locks WHERE NOT granted AND $1::int = ANY(pg_blocking_pids(pid))`, pid) > 0
	})
}

// TestStartRunModelValidated (design note 13, 09 section 21): the model of run.start passes the
// pattern and the bound agent.session_ref_max_bytes before the run changes, with and without a
// reference: 400 validation_failed on /model, never a 500, and the run stays queued.
func TestStartRunModelValidated(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run := w.trigger(th)
	token := w.deliver(run)
	for _, extra := range []string{
		`"model":"m\u0000x"`,
		`"model":"m\u0000x","external_session_ref":"sess-model-1"`,
		`"model":"bad model"`,
		`"model":"m\u202ex"`,
		`"model":"` + strings.Repeat("m", 70000) + `"`,
	} {
		p := w.want(w.start("lotse", run, token, extra), http.StatusBadRequest, "validation_failed")
		w.b6Pointer(p, "/model")
		if got := w.lifecycle(run); got.State != "queued" {
			t.Fatalf("%.40s: run %+v, want queued", extra, got)
		}
	}
	w.wantOK(w.start("lotse", run, token, `"model":"model-large"`))
}
