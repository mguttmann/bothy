// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// session writes a current session row of agent in thread as bothy_owner (agents owns the table;
// the runs side only names its id) and returns its id.
func (e env) session(t *testing.T, thread, agent ids.ID, ref string) ids.ID {
	t.Helper()
	id := ids.New()
	e.exec(t, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref) VALUES ($1, $2, $3, $4)`,
		id.String(), thread.String(), agent.String(), ref)
	return id
}

// isCode reports whether err is the kernel/errs error with code.
func isCode(err error, code errs.Code) bool {
	e, ok := errs.As(err)
	return ok && e.Code == code
}

// TestSetRunSessionAndSessionUsed (M2-12 design notes 9 and 10): SetRunSession writes only
// runs.agent_session_id (no version change), the run reads it back, and SessionUsed tells a row a
// run of the thread used from one no run used. An unknown run is not_found; outside a unit of
// work the write is internal.
func TestSetRunSessionAndSessionUsed(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	run := e.insert(t, newRun(w, w.thread, w.robin, time.Now()), true)
	used, unused := e.session(t, w.thread, w.robin, "sess-1"), e.session(t, w.other, w.robin, "sess-2")
	e.do(t, func(ctx context.Context) error { return e.repo.SetRunSession(ctx, run.ID, used) })
	var got domain.State
	e.do(t, func(ctx context.Context) error {
		row, found, err := e.repo.Run(ctx, run.ID)
		if err == nil && (!found || row.AgentSessionID != used || row.Version != run.Version) {
			t.Errorf("run after SetRunSession: %+v", row)
		}
		got = row.State
		for id, want := range map[ids.ID]bool{used: true, unused: false} {
			u, err := e.repo.SessionUsed(ctx, w.thread, id)
			if err != nil {
				return err
			}
			if u != want {
				t.Errorf("SessionUsed(%s) = %v, want %v", id, u, want)
			}
		}
		return err
	})
	if got != domain.StateQueued {
		t.Fatalf("state %s", got)
	}
	err := e.uow.Do(context.Background(), func(ctx context.Context) error { return e.repo.SetRunSession(ctx, ids.New(), used) })
	if !isCode(err, errs.CodeNotFound) {
		t.Fatalf("unknown run: %v", err)
	}
	if err := e.repo.SetRunSession(context.Background(), run.ID, used); !isCode(err, errs.CodeInternalError) {
		t.Fatalf("outside a unit of work: %v", err)
	}
}

// TestRunSource_ActiveRun (agents.RunSource, R-DM-14): the run of the thread in queued, running or
// waiting_input with its agent, session and state; none for a thread without an active run; the
// row stays locked for the rest of the unit (a second unit cannot lock it); outside a unit of
// work it is internal.
func TestRunSource_ActiveRun(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	src := postgres.NewRunSource(e.owner)
	run := e.insert(t, newRun(w, w.thread, w.robin, time.Now()), true)
	sess := e.session(t, w.thread, w.robin, "sess-1")
	e.do(t, func(ctx context.Context) error { return e.repo.SetRunSession(ctx, run.ID, sess) })
	e.do(t, func(ctx context.Context) error {
		a, found, err := src.ActiveRun(ctx, w.thread)
		if err == nil && (!found || a != (agents.ActiveRun{RunID: run.ID, AgentID: w.robin, SessionID: sess, State: "queued"})) {
			t.Errorf("ActiveRun %+v %v", a, found)
		}
		if err != nil {
			return err
		}
		_, found, err = src.ActiveRun(ctx, w.other)
		if err == nil && found {
			t.Error("a thread without an active run has one")
		}
		return err
	})
	e.transition(t, app.TransitionWrite{ID: run.ID, From: domain.StateQueued, Version: 1, To: domain.StateRunning, At: time.Now()}, true)
	locked := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- e.uow.Do(context.Background(), func(ctx context.Context) error {
			a, found, err := src.ActiveRun(ctx, w.thread)
			if err == nil && (!found || a.State != "running") {
				t.Errorf("ActiveRun of a running run %+v %v", a, found)
			}
			close(locked)
			<-release
			return err
		})
	}()
	<-locked
	tx, err := e.owner.Begin(context.Background(), pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `SET LOCAL lock_timeout = '200ms'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `SELECT 1 FROM runs WHERE id = $1 FOR UPDATE`, run.ID.String()); err == nil {
		t.Error("the row of the active run is not locked")
	}
	_ = tx.Rollback(context.Background())
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	e.transition(t, app.TransitionWrite{ID: run.ID, From: domain.StateRunning, Version: 2, To: domain.StateCompleted, At: time.Now()}, true)
	e.do(t, func(ctx context.Context) error {
		_, found, err := src.ActiveRun(ctx, w.thread)
		if err == nil && found {
			t.Error("a completed run is active")
		}
		return err
	})
	if _, _, err := src.ActiveRun(context.Background(), w.thread); !isCode(err, errs.CodeInternalError) {
		t.Fatalf("outside a unit of work: %v", err)
	}
}
