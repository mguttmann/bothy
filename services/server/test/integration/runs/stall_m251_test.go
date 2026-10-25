// SPDX-License-Identifier: Apache-2.0

package runs_test

// Tests of M2-51: runs.last_progress_at, the stall marking of runs.stall_check, the remove of
// /stall_warned_at in the next patch and stall_warned_at in Run and RunSummary (09 5.5, 04 11.3
// item 7, R-DM-53). The world's clock is the fake clock, so run.stall_after (10m in the world) is
// crossed by advancing it, never by waiting.

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// stallAfter is run.stall_after of the world (world_test.go).
const stallAfter = 10 * time.Minute

// updates records the run.updated events the runs service publishes.
type updates struct {
	mu   sync.Mutex
	list []runs.RunUpdated
}

// PublishEphemeral implements eventlog.EphemeralPublisher.
func (u *updates) PublishEphemeral(_ context.Context, ev eventlog.EphemeralEvent) error {
	if p, ok := ev.Payload.(runs.RunUpdated); ok && ev.Type == "run.updated" {
		u.mu.Lock()
		u.list = append(u.list, p)
		u.mu.Unlock()
	}
	return nil
}

// of returns the recorded run.updated of run, in publish order.
func (u *updates) of(run string) []runs.RunUpdated {
	u.mu.Lock()
	defer u.mu.Unlock()
	var out []runs.RunUpdated
	for _, p := range u.list {
		if p.RunID.String() == run {
			out = append(out, p)
		}
	}
	return out
}

// recordUpdates binds a recording publisher to the runs service of w.
func (w *world) recordUpdates() *updates {
	u := &updates{}
	w.runs.BindPublisher(u)
	return u
}

// progressOf reads last_progress_at and stall_warned_at of run (zero for NULL).
func (w *world) progressOf(run string) (lastProgress, warned time.Time) {
	w.t.Helper()
	var lp, sw *time.Time
	if err := w.db.QueryRow(context.Background(),
		`SELECT last_progress_at, stall_warned_at FROM runs WHERE id = $1`, run).Scan(&lp, &sw); err != nil {
		w.t.Fatal(err)
	}
	if lp != nil {
		lastProgress = lp.UTC()
	}
	if sw != nil {
		warned = sw.UTC()
	}
	return lastProgress, warned
}

// patchStatus sends patchRun as Lotse with replace /status_line at base and returns the status.
func (w *world) patchStatus(run string, base int64, line string) int {
	return w.do("lotse", http.MethodPatch, "/api/v1/runs/"+run,
		`{"base_version":`+strconv.FormatInt(base, 10)+`,"ops":[{"op":"replace","path":"/status_line","value":"`+line+`"}]}`).StatusCode
}

// stallCheck runs one round of runs.stall_check.
func (w *world) stallCheck() {
	w.t.Helper()
	must(w.t, w.runs.StallCheck(context.Background()))
}

// TestLastProgress_SetOnStartAndPatch: run.start and a stored patch set last_progress_at to the
// clock time; a refused patch (stale base_version) does not.
func TestLastProgress_SetOnStartAndPatch(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.started()
	startedAt := w.clk.Now().UTC().Truncate(time.Microsecond)
	if lp, _ := w.progressOf(r.ID); !lp.Equal(startedAt) {
		t.Fatalf("after start: last_progress_at %v, want %v", lp, startedAt)
	}
	w.clk.Advance(time.Minute)
	v := w.runVersion(r.ID)
	if got := w.patchStatus(r.ID, v, "Reading"); got != http.StatusOK {
		t.Fatalf("patch: %d", got)
	}
	patchedAt := w.clk.Now().UTC().Truncate(time.Microsecond)
	if lp, _ := w.progressOf(r.ID); !lp.Equal(patchedAt) {
		t.Fatalf("after patch: last_progress_at %v, want %v", lp, patchedAt)
	}
	w.clk.Advance(time.Minute)
	if got := w.patchStatus(r.ID, v, "Stale"); got != http.StatusConflict {
		t.Fatalf("stale patch: %d", got)
	}
	if lp, _ := w.progressOf(r.ID); !lp.Equal(patchedAt) {
		t.Fatalf("a refused patch moved last_progress_at to %v", lp)
	}
}

// TestStallCheck_MarksRunningWithoutProgress: exactly run.stall_after after the last progress
// nothing happens; past it one round sets stall_warned_at with one version step and publishes one
// run.updated with add /stall_warned_at; a second round changes nothing.
func TestStallCheck_MarksRunningWithoutProgress(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	u := w.recordUpdates()
	r, _, _ := w.started()
	v := w.runVersion(r.ID)
	w.clk.Advance(stallAfter)
	w.stallCheck()
	if _, warned := w.progressOf(r.ID); !warned.IsZero() || w.runVersion(r.ID) != v {
		t.Fatalf("marked at exactly run.stall_after: %v", warned)
	}
	w.clk.Advance(time.Second)
	w.stallCheck()
	at := w.clk.Now().UTC().Truncate(time.Microsecond)
	if _, warned := w.progressOf(r.ID); !warned.Equal(at) {
		t.Fatalf("stall_warned_at %v, want %v", warned, at)
	}
	if got := w.runVersion(r.ID); got != v+1 {
		t.Fatalf("version %d, want %d", got, v+1)
	}
	ev := u.of(r.ID)
	if len(ev) != 1 || ev[0].BaseVersion != v || ev[0].Version != v+1 || len(ev[0].Ops) != 1 ||
		ev[0].Ops[0].Op != "add" || ev[0].Ops[0].Path != "/stall_warned_at" {
		t.Fatalf("run.updated %+v", ev)
	}
	var stamp string
	if err := json.Unmarshal(ev[0].Ops[0].Value, &stamp); err != nil || stamp != at.Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z") {
		t.Fatalf("value %s (%v)", ev[0].Ops[0].Value, err)
	}
	if got := w.lifecycle(r.ID); got.State != "running" {
		t.Fatalf("the marking changed the state: %+v", got)
	}

	// Within run.orphan_timeout of the start, so the run stays running and only the marking counts.
	w.clk.Advance(time.Minute)
	w.stallCheck()
	if got := w.runVersion(r.ID); got != v+1 || len(u.of(r.ID)) != 1 {
		t.Fatalf("second round: version %d, %d events", got, len(u.of(r.ID)))
	}
}

// TestStallCheck_SkipsWaitingInput: a run waiting for a person is never marked.
func TestStallCheck_SkipsWaitingInput(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.started()
	w.wantOK(w.await("lotse", r.ID, "Waiting"))
	v := w.runVersion(r.ID)
	w.clk.Advance(stallAfter + time.Second)
	w.stallCheck()
	if _, warned := w.progressOf(r.ID); !warned.IsZero() || w.runVersion(r.ID) != v {
		t.Fatalf("a waiting run was marked: %v", warned)
	}
}

// TestStallCheck_PatchBetweenReadAndLockWins: a patch stored after the round read the runs but
// before it locked the run is progress; the compare-and-swap on version skips the run.
func TestStallCheck_PatchBetweenReadAndLockWins(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	u := w.recordUpdates()
	r, _, _ := w.started()
	w.clk.Advance(stallAfter + time.Second)
	v := w.runVersion(r.ID)
	w.heartbeats.onNextRead(func() {
		if got := w.patchStatus(r.ID, v, "Still working"); got != http.StatusOK {
			t.Errorf("patch in between: %d", got)
		}
	})
	w.stallCheck()
	if _, warned := w.progressOf(r.ID); !warned.IsZero() || w.runVersion(r.ID) != v+1 {
		t.Fatalf("marked although a patch came in between: %v, version %d", warned, w.runVersion(r.ID))
	}
	for _, ev := range u.of(r.ID) {
		for _, o := range ev.Ops {
			if o.Path == "/stall_warned_at" {
				t.Fatalf("published %+v", ev)
			}
		}
	}
}

// TestPatch_ClearsStallWithRemoveOp: the next stored patch clears stall_warned_at and its
// normalized ops carry remove /stall_warned_at; a later patch carries no such op.
func TestPatch_ClearsStallWithRemoveOp(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	u := w.recordUpdates()
	r, _, _ := w.started()
	w.clk.Advance(stallAfter + time.Second)
	w.stallCheck()
	v := w.runVersion(r.ID)
	if got := w.patchStatus(r.ID, v, "Back"); got != http.StatusOK {
		t.Fatalf("patch: %d", got)
	}
	if _, warned := w.progressOf(r.ID); !warned.IsZero() {
		t.Fatalf("stall_warned_at %v after a patch", warned)
	}
	hasRemove := func(ev runs.RunUpdated) bool {
		for _, o := range ev.Ops {
			if o.Op == "remove" && o.Path == "/stall_warned_at" && o.Value == nil {
				return true
			}
		}
		return false
	}
	ev := u.of(r.ID)
	if len(ev) != 2 || ev[1].BaseVersion != v || !hasRemove(ev[1]) {
		t.Fatalf("run.updated %+v", ev)
	}
	if got := w.patchStatus(r.ID, v+1, "Again"); got != http.StatusOK {
		t.Fatalf("second patch: %d", got)
	}
	if ev := u.of(r.ID); len(ev) != 3 || hasRemove(ev[2]) {
		t.Fatalf("a patch without a notice removed it: %+v", ev)
	}
}

// TestListActiveRuns_StallWarnedAt: listActiveRuns and getRun return stall_warned_at of a marked
// run, getRun returns metrics.progress_events, and both answers pass the contract validation of
// the world (ValidateResponses).
func TestListActiveRuns_StallWarnedAt(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.started()
	body := `{"base_version":` + strconv.FormatInt(w.runVersion(r.ID), 10) +
		`,"ops":[{"op":"replace","path":"/metrics/progress_events","value":7}]}`
	w.wantOK(w.do("lotse", http.MethodPatch, "/api/v1/runs/"+r.ID, body))
	w.clk.Advance(stallAfter + time.Second)
	w.stallCheck()
	want := w.clk.Now().UTC().Truncate(time.Millisecond)
	same := func(v any) bool {
		s, _ := v.(string)
		got, err := time.Parse(time.RFC3339Nano, s)
		return err == nil && got.Equal(want)
	}

	doc := w.getRun("owner", r.ID)
	if !same(doc["stall_warned_at"]) {
		t.Fatalf("getRun stall_warned_at %v, want %v", doc["stall_warned_at"], want)
	}
	if m, _ := doc["metrics"].(map[string]any); m["progress_events"] != float64(7) {
		t.Fatalf("getRun metrics %v", doc["metrics"])
	}
	page := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/runs", ""))
	items, _ := page["items"].([]any)
	found := false
	for _, it := range items {
		if m := it.(map[string]any); m["id"] == r.ID {
			found = true
			if !same(m["stall_warned_at"]) {
				t.Fatalf("listActiveRuns stall_warned_at %v, want %v", m["stall_warned_at"], want)
			}
		}
	}
	if !found {
		t.Fatalf("run not listed: %v", items)
	}
}
