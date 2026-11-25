// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
)

// TestRetention_EmptiesRunTexts (AUD-PKG-R2-owner-server, #685): retention.content empties the
// status line, excerpt, summary and plan of a run that finished before the cutoff, the title and
// detail of its steps and the label, condition and result of its ended wake-up; the rows stay. A
// cutoff at the end time itself empties nothing, and a second run finds nothing more.
func TestRetention_EmptiesRunTexts(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	w := newPDWorld(t, exec)
	step, wake := ids.New(), ids.New()
	exec(`INSERT INTO run_steps (id, run_id, ordinal, kind, name, title, status, detail, finished_at)
		VALUES ($1, $2, 1, 'tool_call', 'read', 'read the notes of Sam', 'completed', '{"output": "notes of Sam"}', $3)`,
		step, w.alexRun, w.finished)
	exec(`INSERT INTO run_wakes (id, agent_id, thread_id, source_run_id, triggered_by, kind, condition, label, reason,
		not_before, deadline, state, result_text, created_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, 'command', '{"command": "check the order of Sam"}', 'order of Sam', 'agent',
		$6, $6, 'fired', 'order of Sam shipped', $6, $6)`,
		wake, w.agent, w.thread, w.alexRun, w.alex, w.finished)
	r := postgres.NewRetention(db.Pool(t, pg.RoleApp))

	if n, _, err := r.Purge(ctx, w.finished, 10); err != nil || n != 0 {
		t.Fatalf("Purge at the end time = %d, %v; want nothing", n, err)
	}
	n, more, err := r.Purge(ctx, w.finished.Add(time.Minute), 10)
	if err != nil || n != 3 || more {
		t.Fatalf("Purge = %d, %v, %v; want the run, the step and the wake-up", n, more, err)
	}
	var status, excerpt, summary *string
	var plan, stepTitle, stepDetail, label, condition, result string
	if err := owner.QueryRow(ctx, `SELECT status_line, last_text_excerpt, summary, plan::text FROM runs WHERE id = $1`, w.alexRun).
		Scan(&status, &excerpt, &summary, &plan); err != nil {
		t.Fatal(err)
	}
	if status != nil || excerpt != nil || summary != nil || plan != "{}" {
		t.Errorf("run texts after retention: %v %v %v %s", status, excerpt, summary, plan)
	}
	if err := owner.QueryRow(ctx, `SELECT title, detail::text FROM run_steps WHERE id = $1`, step).Scan(&stepTitle, &stepDetail); err != nil {
		t.Fatal(err)
	}
	if stepTitle != "" || stepDetail != "{}" {
		t.Errorf("step after retention: %q %s", stepTitle, stepDetail)
	}
	if err := owner.QueryRow(ctx, `SELECT label, condition::text, result_text FROM run_wakes WHERE id = $1`, wake).
		Scan(&label, &condition, &result); err != nil {
		t.Fatal(err)
	}
	if label != "" || condition != "{}" || result != "" {
		t.Errorf("wake-up after retention: %q %s %q", label, condition, result)
	}
	if n, _, err := r.Purge(ctx, w.finished.Add(time.Minute), 10); err != nil || n != 0 {
		t.Fatalf("second Purge = %d, %v; want nothing", n, err)
	}
}
