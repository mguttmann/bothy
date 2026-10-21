// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	syncpkg "github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// pastWindow is a step of the fake clock beyond notify.coalesce_window of pushEnv (60s).
const pastWindow = 61 * time.Second

// hint is one hint of a person: the arguments of the first job of its NotificationID.
type hint struct {
	Kind              domain.Kind
	Thread, Message   ids.ID
	Target            ids.ID
	Registration      ids.ID
	NotificationID    ids.ID
	Count, DueAtMicro int64
}

// hintList returns the hints of recipient in the order of their first job (one entry per
// NotificationID, as hintsFor counts them).
func (w *world) hintList(recipient ids.ID) []hint {
	w.t.Helper()
	var out []hint
	seen := map[ids.ID]bool{}
	for _, j := range w.pushJobsFor(recipient) {
		if seen[j.Args.NotificationID] {
			continue
		}
		seen[j.Args.NotificationID] = true
		out = append(out, hint{Kind: j.Kind(), Thread: j.Args.ThreadID, Message: j.Args.MessageID, Target: j.Args.TargetID,
			Registration: j.Args.RegistrationID, NotificationID: j.Args.NotificationID,
			Count: j.Args.Count, DueAtMicro: j.Args.DueAtUnixMicro})
	}
	return out
}

// wantHints fails the test unless the hints of recipient have exactly the kinds and messages of
// want, in order, all in thread.
func (w *world) wantHints(who string, recipient, thread ids.ID, want ...hint) []hint {
	w.t.Helper()
	got := w.hintList(recipient)
	if len(got) != len(want) {
		w.t.Fatalf("%s has %d hints, want %d: %+v", who, len(got), len(want), got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Message != want[i].Message || got[i].Thread != thread {
			w.t.Fatalf("%s's hint %d is %s for message %s in thread %s, want %s for message %s in thread %s",
				who, i, got[i].Kind, got[i].Message, got[i].Thread, want[i].Kind, want[i].Message, thread)
		}
	}
	return got
}

// runScene is the scene of the run tests (brief M3-19 "Tests", TestRunWithThreeAgentMessages):
// thread T in Support/Allgemein where Sam set all; Alex's message mentioning Lotse, which opened
// the run; Sam's message hint for it, whose window the clock has already passed.
type runScene struct {
	thread  ids.ID
	trigger msg
	run     string
}

// runScene builds the run scene: the Owner opens T, Sam sets all on T (putNotificationPreference),
// Alex mentions Lotse in T and the consumer runs. Sam gets one message hint for Alex's message,
// which opens Sam's window; the clock moves past notify.coalesce_window and the window and flush
// handlers run.
func (w *world) runScene() runScene {
	w.t.Helper()
	th := w.newThread(w.allgemein)
	w.setLevel("sam", "thread", th, "all")
	m := w.post("alex", th, "@lotse please look at the export")
	run, ok := w.runOf(m.ID.String())
	if !ok {
		w.t.Fatal("Alex's mention of Lotse opened no run")
	}
	w.consume()
	w.wantHints("Sam", w.sam, th, hint{Kind: domain.KindMessage, Message: m.ID})
	if windows := w.windowsOf(notifyjobs.KindCoalesceWindow, w.sam, th); len(windows) != 1 || !windows[0].pending() {
		w.t.Fatalf("Sam's windows on T %+v, want one open window", windows)
	}
	if hints := w.hintList(w.alex); len(hints) != 0 {
		w.t.Fatalf("Alex got hints for his own message: %+v", hints)
	}
	w.advance(pastWindow)
	w.flushDue()
	return runScene{thread: th, trigger: m, run: run}
}

// startRun delivers the invocation of run (in place of pollAgentEvents) and starts it as Lotse
// (startRun).
func (w *world) startRun(run string) {
	w.t.Helper()
	w.wantOK(w.start("lotse", run, w.deliver(run)))
}

// endSeq returns the seq of the last run.state_changed of run.
func (w *world) endSeq(run string) int64 {
	return w.count(`SELECT max(seq) FROM event_log WHERE type = 'run.state_changed' AND resource_id = $1`, run)
}

// creationSeq returns the seq of message.created of message.
func (w *world) creationSeq(message ids.ID) int64 {
	return w.count(`SELECT seq FROM event_log WHERE type = 'message.created' AND resource_id = $1`, message.String())
}

// consumeTo runs rounds of one event each on a second worker of the consumer push (consumer.batch_size
// 1) until its cursor reaches seq, so the test can move the clock between two events.
func (w *world) consumeTo(seq int64) {
	w.t.Helper()
	cfg := w.consumerCfg
	cfg.BatchSize = 1
	repo := syncpg.NewRepository(w.pool, slog.New(w.pushLog))
	svc, err := syncapp.NewConsumerService(cfg, w.uow, repo, repo, slog.New(w.pushLog))
	must(w.t, err)
	must(w.t, svc.Register(syncpkg.ConsumerPush, w.push.Consumer()))
	for w.cursor() < seq {
		res, err := svc.Round(context.Background(), syncpkg.ConsumerPush)
		must(w.t, err)
		if res.Outcome != syncapp.RoundProcessed {
			w.t.Fatalf("the round ended with %v before seq %d (cursor %d)", res.Outcome, seq, w.cursor())
		}
	}
}

// TestRunWithThreeAgentMessages_ExactlyOneRunDone (PLAN row M3-19, 13 section 14 "Run with three
// agent messages", 13 2.1, R-PN-02, design notes 4 b and 5): in the run scene Lotse starts the run
// (startRun), writes three messages in T with its agent token and ends the run; the consumer runs
// only after all of it committed. Alex, who started the run, gets exactly one hint, run_done,
// pointing at the third agent message; Sam gets exactly one further hint, message, pointing at the
// same message; no other hint for the three messages exists. Variants: failed gives run_failed to
// Alex and message to Sam; a cancel through cancelRun gives message to Alex and Sam; a fourth
// message of Lotse after the end gives normal message hints once the clock has moved past
// notify.coalesce_window again and does not move the pointer, even though it was committed before
// the consumer reached the end of the run; a run that waits for input (awaitRunInput) gives one
// input_needed to Alex, also when Alex stored mute on T.
func TestRunWithThreeAgentMessages_ExactlyOneRunDone(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"completed", "failed", "cancelled", "fourth_message", "await_input"} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			s := w.runScene()
			if variant == "await_input" {
				// Alex follows T with all since his message; his own mute replaces it.
				w.setLevel("alex", "thread", s.thread, "mute")
			}
			w.startRun(s.run)
			var agent []msg
			for _, text := range []string{"Looking at the export", "The job stops at row 400", "Fixed the mapping, the export runs"} {
				agent = append(agent, w.post("lotse", s.thread, text))
			}
			last := agent[2]
			var fourth msg
			switch variant {
			case "completed", "fourth_message":
				w.wantOK(w.finish("lotse", s.run, `"state":"completed","summary":"Export fixed"`))
			case "failed":
				w.wantOK(w.finish("lotse", s.run, `"state":"failed","reason":"adapter_error"`))
			case "cancelled":
				w.want(w.cancel("alex", s.run), http.StatusAccepted, "")
				w.wantOK(w.finish("lotse", s.run, `"state":"cancelled","reason":"user_cancelled"`))
			case "await_input":
				w.wantOK(w.await("lotse", s.run, "Waiting for approval"))
			}
			if variant == "fourth_message" {
				// The fourth message commits after the end of the run, before the consumer
				// reaches the end: the pointer of the end hints must stay at the third message.
				fourth = w.post("lotse", s.thread, "One more note after the run")
				if end := w.endSeq(s.run); w.creationSeq(fourth.ID) <= end {
					t.Fatalf("the fourth message (seq %d) is not after the end of the run (seq %d)", w.creationSeq(fourth.ID), end)
				}
				w.consumeTo(w.endSeq(s.run))
			} else {
				w.consume()
			}
			first := hint{Kind: domain.KindMessage, Message: s.trigger.ID}
			switch variant {
			case "completed", "fourth_message":
				w.wantHints("Alex", w.alex, s.thread, hint{Kind: domain.KindRunDone, Message: last.ID})
				w.wantHints("Sam", w.sam, s.thread, first, hint{Kind: domain.KindMessage, Message: last.ID})
			case "failed":
				w.wantHints("Alex", w.alex, s.thread, hint{Kind: domain.KindRunFailed, Message: last.ID})
				w.wantHints("Sam", w.sam, s.thread, first, hint{Kind: domain.KindMessage, Message: last.ID})
			case "cancelled":
				w.wantHints("Alex", w.alex, s.thread, hint{Kind: domain.KindMessage, Message: last.ID})
				w.wantHints("Sam", w.sam, s.thread, first, hint{Kind: domain.KindMessage, Message: last.ID})
			case "await_input":
				w.wantHints("Alex", w.alex, s.thread, hint{Kind: domain.KindInputNeeded, Message: last.ID})
				w.wantHints("Sam", w.sam, s.thread, first)
			}
			// No hint for the three agent messages other than the run's own hints on the last one.
			for _, j := range w.pushJobs() {
				if j.Args.MessageID == agent[0].ID || j.Args.MessageID == agent[1].ID {
					t.Fatalf("a hint for an agent message during the run: %+v", j.Args)
				}
				if j.Args.RecipientID != w.alex && j.Args.RecipientID != w.sam {
					t.Fatalf("a hint for someone else than Alex and Sam: %+v", j.Args)
				}
			}
			if variant != "fourth_message" {
				return
			}
			// Past notify.coalesce_window the fourth message is judged on its own: normal message
			// hints at once to Alex (who follows T with all) and Sam, and the end hints keep their
			// pointer.
			w.advance(pastWindow)
			w.flushDue()
			w.consume()
			// A hint that exists right after the round, before any flush, went at once: a
			// collected hint becomes a job only at the window end.
			alex := w.wantHints("Alex", w.alex, s.thread, hint{Kind: domain.KindRunDone, Message: last.ID},
				hint{Kind: domain.KindMessage, Message: fourth.ID})
			sam := w.wantHints("Sam", w.sam, s.thread, first, hint{Kind: domain.KindMessage, Message: last.ID},
				hint{Kind: domain.KindMessage, Message: fourth.ID})
			for _, h := range []hint{alex[1], sam[2]} {
				if h.Count != 1 {
					t.Fatalf("the hint of the fourth message %+v did not go at once", h)
				}
			}
		})
	}
}

// TestAgentMentionDuringRun_GoesAtOnce (13 2.1, R-PN-02): in the run scene Lotse's second message
// of the run mentions Sam and the consumer runs: Sam gets mention at once, which opens Sam's
// window; after the clock has moved past notify.coalesce_window the run ends and Sam still gets
// exactly one message hint at the end, pointing at the last agent message.
func TestAgentMentionDuringRun_GoesAtOnce(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	s := w.runScene()
	w.startRun(s.run)
	a1 := w.post("lotse", s.thread, "Looking at the export")
	a2 := w.post("lotse", s.thread, "@sam can you confirm the numbers?")
	w.consume()
	now := w.clk.Now()
	first := hint{Kind: domain.KindMessage, Message: s.trigger.ID}
	got := w.wantHints("Sam", w.sam, s.thread, first, hint{Kind: domain.KindMention, Message: a2.ID})
	// The job exists right after the round, before any flush: the mention went at once.
	if got[1].Registration != w.samPhone.Registration {
		t.Fatalf("the mention %+v did not go at once to Sam's phone", got[1])
	}
	var open []windowJob
	for _, j := range w.windowsOf(notifyjobs.KindCoalesceWindow, w.sam, s.thread) {
		if j.pending() {
			open = append(open, j)
		}
	}
	if len(open) != 1 || !open[0].ScheduledAt.Equal(now.Add(60*time.Second)) {
		t.Fatalf("Sam's open windows %+v, want one until %s", open, now.Add(60*time.Second))
	}
	if hints := w.hintList(w.alex); len(hints) != 0 {
		t.Fatalf("Alex got hints during the run: %+v", hints)
	}
	w.advance(pastWindow)
	w.flushDue()
	a3 := w.post("lotse", s.thread, "Confirmed and fixed")
	w.wantOK(w.finish("lotse", s.run, `"state":"completed"`))
	w.consume()
	w.wantHints("Sam", w.sam, s.thread, first, hint{Kind: domain.KindMention, Message: a2.ID}, hint{Kind: domain.KindMessage, Message: a3.ID})
	w.wantHints("Alex", w.alex, s.thread, hint{Kind: domain.KindRunDone, Message: a3.ID})
	if slices.ContainsFunc(w.pushJobs(), func(j pushJob) bool { return j.Args.MessageID == a1.ID }) {
		t.Fatal("a hint for the first agent message of the run")
	}
}
