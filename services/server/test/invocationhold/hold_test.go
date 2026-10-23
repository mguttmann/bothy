// SPDX-License-Identifier: Apache-2.0

// Package invocationhold_test holds the integration tests of the held agent invocation of brief
// M3-07 ("Tests / Integration", the TestHold_* rows, TestAgentAttachments_FilesOutSwitch and the
// M2-11 rows): the real server of internal/wire in role all through test/harness on a fake clock,
// PostgreSQL 18 from pgtest, uploads through createUpload, uploadPart and completeUpload as Alex,
// the scan pass of files.scan called directly against the fake clamd of clamdtest (or, in the
// test marked real clamd, its throwaway container), and Lotse's side through pollAgentEvents with
// its agent token. River runs no-ops for files.scan, runs.invocation_hold_timeout and
// runs.queued_expiry; the tests move the fake clock and call these handlers themselves.
package invocationhold_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestHold_ReleasedOnClean (plan, 12 15 row "Agent invocation with an attachment while clamd is
// down"): with clamd stopped Alex mentions Lotse with F; the run is held until the due files.scan
// makes F clean, which writes exactly one agent.invocation in its transaction; the next poll
// delivers it with a token and trigger_state becomes queued; the hold job after the limit writes
// nothing.
func TestHold_ReleasedOnClean(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f := w.file("export.txt")
	sentAt := w.clk.Now()
	m := w.mention(f)
	if m.TriggerState != "received" {
		t.Fatalf("201 trigger_state %q", m.TriggerState)
	}
	r := w.runOf(m.ID)
	if r.State != "queued" || r.Thread == w.main || !r.HeldUntil.Equal(sentAt.Add(holdMax)) {
		t.Fatalf("run %+v, want queued in the new thread held until %v", r, sentAt.Add(holdMax))
	}
	if jobs := w.holdJobs(r.ID); len(jobs) != 1 || !jobs[0].Equal(r.HeldUntil) {
		t.Fatalf("hold jobs %v, want one at %v", jobs, r.HeldUntil)
	}
	if n := len(w.invocations(r.ID)); n != 0 {
		t.Fatalf("%d agent.invocation while held", n)
	}
	// clamd is stopped: the due round of files.scan schedules the next round, F stays pending.
	w.mustScan(f)
	if up, scan := w.fileState(f); up != "pending" || scan != "pending" {
		t.Fatalf("file %s %s while clamd is stopped", up, scan)
	}
	if got := delivered(w.poll(), r.ID); len(got) != 0 {
		t.Fatalf("delivered while held: %v", got)
	}
	// A clock step below the limit: the hold job is early, schedules itself again at the release
	// time and releases nothing (design note 13 b).
	w.clk.Advance(holdMax - 1)
	w.holdTimeout(r.ID)
	if jobs := w.holdJobs(r.ID); len(jobs) != 2 || !jobs[1].Equal(r.HeldUntil) {
		t.Fatalf("hold jobs after the early run %v", jobs)
	}
	if got := delivered(w.poll(), r.ID); len(got) != 0 || w.triggerState(m.ID) != "received" || len(w.invocations(r.ID)) != 0 {
		t.Fatalf("released below the limit: %v, trigger_state %s", got, w.triggerState(m.ID))
	}
	held := w.run(r.ID)
	w.clamd.start(clamdtest.Script{})
	w.mustScan(f)
	if up, scan := w.fileState(f); up != "complete" || scan != "clean" {
		t.Fatalf("file %s %s after the start", up, scan)
	}
	inv := w.wantReleased(r.ID)
	if after := w.run(r.ID); !after.HeldUntil.IsZero() || after.Version != held.Version+1 || after.State != "queued" {
		t.Fatalf("run after the release %+v, before %+v", after, held)
	}
	// The file.updated of the clean outcome and the release commit together: no event of another
	// unit lies between them.
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'file.updated' AND resource_id = $1 AND seq < $2`, f, inv.Seq); n != 1 {
		t.Fatalf("%d file.updated of F before the invocation", n)
	}
	got := delivered(w.poll(), r.ID)
	if len(got) != 1 || token(got[0]) == "" {
		t.Fatalf("delivered %v", got)
	}
	if st := w.triggerState(m.ID); st != "queued" {
		t.Fatalf("trigger_state %s after the delivery", st)
	}
	// After the limit the hold job finds the run released and writes nothing.
	before, version := w.maxSeq(), w.run(r.ID).Version
	w.clk.Advance(holdMax)
	w.holdTimeout(r.ID)
	if w.maxSeq() != before || w.run(r.ID).Version != version || len(w.holdJobs(r.ID)) != 2 {
		t.Fatal("the hold job after the release wrote something")
	}
	if status, scan := w.getFile(w.lotse, f); status != http.StatusOK || scan != "clean" {
		t.Fatalf("Lotse getFile %d %s", status, scan)
	}
}

// wantReleased fails unless the run has exactly one agent.invocation now and it is a release: no
// actor and no correlation id (no principal wrote it), audience the agent, receive action
// agent.events.consume, resource the run, the scope of the run's thread now and a payload valid
// against AgentInvocationPayload.
func (w *world) wantReleased(runID string) invocation {
	w.t.Helper()
	inv := w.invocations(runID)
	if len(inv) != 1 {
		w.t.Fatalf("%d agent.invocation of run %s, want 1", len(inv), runID)
	}
	r := w.run(runID)
	space, channel := w.threadScope(r.Thread)
	got := inv[0]
	if got.ActorKind != "" || got.ActorID != "" || got.CorrelationID != "" {
		w.t.Fatalf("released invocation with actor %q %q correlation %q", got.ActorKind, got.ActorID, got.CorrelationID)
	}
	if got.SpaceID != space || got.ChannelID != channel || got.ThreadID != r.Thread || got.Audience != w.lotse.PrincipalID ||
		got.ReceiveAction != "agent.events.consume" || got.Resource != "run" {
		w.t.Fatalf("released invocation %+v, want scope %s %s %s of Lotse", got, space, channel, r.Thread)
	}
	validInvocationPayload(w.t, got.Payload)
	if !r.HeldUntil.IsZero() {
		w.t.Fatalf("run %+v still held after the release", r)
	}
	return got
}

// TestHold_TimeoutReleasesPending (plan): clamd stays stopped; after file.invocation_hold_max the
// hold job appends exactly one agent.invocation without actor in the scope of the run's thread,
// valid against AgentInvocationPayload, while F is still pending: Lotse's getFile shows pending
// and downloadFile answers 409 file_not_ready (M3-05). When F becomes clean later, no second
// agent.invocation appears. getRunContext (M2-13) is not merged and not checked.
func TestHold_TimeoutReleasesPending(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f := w.file("report.txt")
	m := w.mention(f)
	r := w.runOf(m.ID)
	w.mustScan(f)
	w.clk.Advance(holdMax)
	w.holdTimeout(r.ID)
	w.wantReleased(r.ID)
	if up, scan := w.fileState(f); up != "pending" || scan != "pending" {
		t.Fatalf("file %s %s after the timeout", up, scan)
	}
	if got := delivered(w.poll(), r.ID); len(got) != 1 || token(got[0]) == "" {
		t.Fatalf("delivered %v", got)
	}
	if status, scan := w.getFile(w.lotse, f); status != http.StatusOK || scan != "pending" {
		t.Fatalf("Lotse getFile %d %s", status, scan)
	}
	dl := w.lotse.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + f + "/content"})
	if dl.Status != http.StatusConflict || problemCode(dl.Body) != "file_not_ready" {
		t.Fatalf("Lotse downloadFile %d %s", dl.Status, dl.Body)
	}
	// A second run of the job writes nothing.
	before := w.maxSeq()
	w.holdTimeout(r.ID)
	if w.maxSeq() != before {
		t.Fatal("the second hold job wrote")
	}
	w.clamd.start(clamdtest.Script{})
	w.mustScan(f)
	if _, scan := w.fileState(f); scan != "clean" {
		t.Fatalf("scan %s", scan)
	}
	if n := len(w.invocations(r.ID)); n != 1 {
		t.Fatalf("%d agent.invocation after F became clean", n)
	}
}

// TestHold_RealClamdStoppedThenClean (real clamd, the clamdtest container with the pinned image):
// the sequence of TestHold_ReleasedOnClean with the container stopped and started again (M3-04
// TestScanClamdStoppedStaysPending).
func TestHold_RealClamdStoppedThenClean(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()
	clamd, err := clamdtest.Start(ctx, clamdtest.Options{Restartable: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clamd.Terminate(context.Background()) })
	w := newWorld(t, worldOptions{address: clamd.Address()})
	if err := clamd.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	f := w.file("container.txt")
	sentAt := w.clk.Now()
	m := w.mention(f)
	r := w.runOf(m.ID)
	if m.TriggerState != "received" || r.State != "queued" || !r.HeldUntil.Equal(sentAt.Add(holdMax)) || len(w.holdJobs(r.ID)) != 1 {
		t.Fatalf("message %+v run %+v", m, r)
	}
	w.mustScan(f)
	if up, scan := w.fileState(f); up != "pending" || scan != "pending" || len(w.invocations(r.ID)) != 0 {
		t.Fatalf("file %s %s while the container is stopped", up, scan)
	}
	if err := clamd.StartAgain(ctx); err != nil {
		t.Fatal(err)
	}
	// The container needs a moment until it answers; every pass before that is one more retry
	// round and keeps F pending and the run held.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		w.mustScan(f)
		if _, scan := w.fileState(f); scan != "pending" {
			break
		}
		if len(w.invocations(r.ID)) != 0 {
			t.Fatal("released while F is pending")
		}
		if time.Now().After(deadline) {
			t.Fatal("F still pending after the start of the container")
		}
		time.Sleep(time.Second)
	}
	if up, scan := w.fileState(f); up != "complete" || scan != "clean" {
		t.Fatalf("file %s %s", up, scan)
	}
	w.wantReleased(r.ID)
	if got := delivered(w.poll(), r.ID); len(got) != 1 || token(got[0]) == "" || w.triggerState(m.ID) != "queued" {
		t.Fatalf("delivered %v, trigger_state %s", got, w.triggerState(m.ID))
	}
	w.clk.Advance(holdMax)
	before := w.maxSeq()
	w.holdTimeout(r.ID)
	if w.maxSeq() != before {
		t.Fatal("the hold job after the release wrote")
	}
}

// TestHold_NoHoldWhenFinal: F is clean before the send: agent.invocation in the message's
// transaction (with Alex as actor, as M2-08 writes it), the column NULL and no hold job.
func TestHold_NoHoldWhenFinal(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	w.clamd.start(clamdtest.Script{})
	f := w.file("clean.txt")
	w.mustScan(f)
	if _, scan := w.fileState(f); scan != "clean" {
		t.Fatalf("scan %s", scan)
	}
	m := w.mention(f)
	r := w.runOf(m.ID)
	inv := w.invocations(r.ID)
	if !r.HeldUntil.IsZero() || len(inv) != 1 || len(w.holdJobs(r.ID)) != 0 {
		t.Fatalf("run %+v, %d invocations, %d hold jobs", r, len(inv), len(w.holdJobs(r.ID)))
	}
	if inv[0].ActorKind != "user" || inv[0].ActorID != w.alex.PrincipalID {
		t.Fatalf("invocation actor %q %q", inv[0].ActorKind, inv[0].ActorID)
	}
	validInvocationPayload(t, inv[0].Payload)
	// A message without attachments is never held either.
	plain := w.send(w.alex, w.main, "@lotse a question without a file")
	if plain.Status != http.StatusCreated {
		t.Fatalf("createMessage: %d %s", plain.Status, plain.Body)
	}
	var pm message
	plain.JSON(t, &pm)
	if pr := w.runOf(pm.ID); !pr.HeldUntil.IsZero() || len(w.invocations(pr.ID)) != 1 {
		t.Fatalf("run without files %+v", pr)
	}
}

// TestHold_LastOfTwoAttachments: F1 and F2 pending; F1 clean keeps the run held; F2 clean
// releases it once. Variant: both outcome writes in parallel give exactly one agent.invocation,
// before the limit.
func TestHold_LastOfTwoAttachments(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	w.clamd.start(clamdtest.Script{})

	t.Run("sequential", func(t *testing.T) {
		f1, f2 := w.file("one.txt"), w.file("two.txt")
		m := w.mention(f1, f2)
		r := w.runOf(m.ID)
		if r.HeldUntil.IsZero() {
			t.Fatalf("run %+v not held", r)
		}
		w.mustScan(f1)
		if _, scan := w.fileState(f1); scan != "clean" {
			t.Fatalf("F1 %s", scan)
		}
		if still := w.run(r.ID); still.HeldUntil.IsZero() || still.Version != r.Version || len(w.invocations(r.ID)) != 0 {
			t.Fatalf("released with F2 pending: %+v", still)
		}
		w.mustScan(f2)
		w.wantReleased(r.ID)
		if got := w.run(r.ID); got.Version != r.Version+1 {
			t.Fatalf("version %d, want %d", got.Version, r.Version+1)
		}
	})

	t.Run("parallel", func(t *testing.T) {
		f1, f2 := w.file("left.txt"), w.file("right.txt")
		m := w.mention(f1, f2)
		r := w.runOf(m.ID)
		var wg sync.WaitGroup
		errc := make(chan error, 2)
		for _, f := range []string{f1, f2} {
			wg.Go(func() { errc <- w.scanFile(f) })
		}
		wg.Wait()
		close(errc)
		for err := range errc {
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, f := range []string{f1, f2} {
			if _, scan := w.fileState(f); scan != "clean" {
				t.Fatalf("%s %s", f, scan)
			}
		}
		w.wantReleased(r.ID)
		if got := w.run(r.ID); got.Version != r.Version+1 {
			t.Fatalf("version %d, want %d", got.Version, r.Version+1)
		}
		w.clk.Advance(holdMax)
		before := w.maxSeq()
		w.holdTimeout(r.ID)
		if w.maxSeq() != before {
			t.Fatal("the hold job after the release wrote")
		}
	})
}

// TestHold_BlockedOrFailedAttachmentReleases: an EICAR reply makes F infected, a final state: the
// run is released at once and Lotse's getFile shows infected. An upload whose bytes do not match
// the declared SHA-256 ends failed (checksum_mismatch), also final: released at once.
func TestHold_BlockedOrFailedAttachmentReleases(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})

	t.Run("infected", func(t *testing.T) {
		w.clamd.start(clamdtest.Script{Reply: eicarReply})
		f := w.file("eicar.txt")
		m := w.mention(f)
		r := w.runOf(m.ID)
		w.mustScan(f)
		if _, scan := w.fileState(f); scan != "infected" {
			t.Fatalf("scan %s", scan)
		}
		w.wantReleased(r.ID)
		if status, scan := w.getFile(w.lotse, f); status != http.StatusOK || scan != "infected" {
			t.Fatalf("Lotse getFile %d %s", status, scan)
		}
	})

	t.Run("failed", func(t *testing.T) {
		w.clamd.start(clamdtest.Script{})
		content := []byte("the bytes that arrived at the server")
		declared := []byte("the bytes the client announced first")
		f := w.upload(w.alex, w.allgemein, "broken.txt", content, declared)
		m := w.mention(f)
		r := w.runOf(m.ID)
		if r.HeldUntil.IsZero() {
			t.Fatalf("run %+v not held", r)
		}
		w.mustScan(f)
		if up, _ := w.fileState(f); up != "failed" {
			t.Fatalf("upload_state %s", up)
		}
		w.wantReleased(r.ID)
	})
}

// TestHold_SameTransaction: a test subscriber of files.FileFinalized registered after
// runs.OnFileFinalized vetoes once: neither the clean state of F, nor its file.updated, nor
// agent.invocation is stored; the retry stores all three.
func TestHold_SameTransaction(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	w.clamd.start(clamdtest.Script{})
	f := w.file("veto.txt")
	m := w.mention(f)
	r := w.runOf(m.ID)
	updates := func() int64 {
		return w.count(`SELECT count(*) FROM event_log WHERE type = 'file.updated' AND resource_id = $1`, f)
	}
	w.veto.Arm(ids.MustParse(f))
	if err := w.scanFile(f); err == nil {
		t.Fatal("the vetoed scan unit succeeded")
	}
	if w.veto.Hits() != 1 {
		t.Fatalf("veto hits %d", w.veto.Hits())
	}
	if up, scan := w.fileState(f); up != "pending" || scan != "pending" || updates() != 0 || len(w.invocations(r.ID)) != 0 {
		t.Fatalf("after the veto: file %s %s, %d file.updated, %d invocations", up, scan, updates(), len(w.invocations(r.ID)))
	}
	if still := w.run(r.ID); still.HeldUntil.IsZero() || still.Version != r.Version {
		t.Fatalf("run after the veto %+v", still)
	}
	w.mustScan(f)
	if up, scan := w.fileState(f); up != "complete" || scan != "clean" || updates() != 1 {
		t.Fatalf("after the retry: file %s %s, %d file.updated", up, scan, updates())
	}
	w.wantReleased(r.ID)
	if n := w.veto.Seen(ids.MustParse(f)); n != 2 {
		t.Fatalf("FileFinalized published %d times, want the vetoed one and the stored one", n)
	}
}

// TestHold_RunEndedWhileHeld: with run.queued_max_age below file.invocation_hold_max,
// runs.queued_expiry (M2-08) fails the held run with agent_unavailable; the run keeps its column,
// and later finalization and the hold job write no agent.invocation.
func TestHold_RunEndedWhileHeld(t *testing.T) {
	t.Parallel()
	queuedMax := holdMax / 2
	w := newWorld(t, worldOptions{config: map[string]string{"run.queued_max_age": seconds(queuedMax)}})
	f := w.file("late.txt")
	m := w.mention(f)
	r := w.runOf(m.ID)
	w.clk.Advance(queuedMax)
	w.queuedExpiry(r.ID)
	ended := w.run(r.ID)
	if ended.State != "failed" || ended.Reason != "agent_unavailable" || !ended.HeldUntil.Equal(r.HeldUntil) {
		t.Fatalf("run after runs.queued_expiry %+v", ended)
	}
	w.clamd.start(clamdtest.Script{})
	w.mustScan(f)
	if _, scan := w.fileState(f); scan != "clean" {
		t.Fatalf("scan %s", scan)
	}
	w.clk.Advance(holdMax)
	w.holdTimeout(r.ID)
	if n := len(w.invocations(r.ID)); n != 0 {
		t.Fatalf("%d agent.invocation of an ended run", n)
	}
	if after := w.run(r.ID); after.Version != ended.Version || !after.HeldUntil.Equal(r.HeldUntil) {
		t.Fatalf("the ended run changed: %+v, was %+v", after, ended)
	}
}

// TestHold_ThreadMovedWhileHeld: the thread of a held run moves to Support/Eskalation; the
// released agent.invocation has the scope of Eskalation.
func TestHold_ThreadMovedWhileHeld(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f := w.file("moved.txt")
	m := w.mention(f)
	r := w.runOf(m.ID)
	harness.MustDo(t, w.owner, harness.Request{Method: http.MethodPost, Path: "/threads/" + r.Thread + "/move",
		Body: map[string]any{"target_channel_id": w.eskalation, "reason": "Belongs to escalation"}}, http.StatusOK)
	if _, channel := w.threadScope(r.Thread); channel != w.eskalation {
		t.Fatalf("the thread is in %s", channel)
	}
	w.clamd.start(clamdtest.Script{})
	w.mustScan(f)
	inv := w.wantReleased(r.ID)
	var p struct {
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(inv.Payload), &p); err != nil {
		t.Fatal(err)
	}
	if inv.ChannelID != w.eskalation || inv.SpaceID != w.w.Spaces["Support"] || p.ChannelID != w.eskalation {
		t.Fatalf("released invocation %+v", inv)
	}
}
