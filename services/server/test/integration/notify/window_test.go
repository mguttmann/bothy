// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// windowJob is the decoded window or flush job of a person and a thread.
type windowJob struct {
	jobRow
	User, Thread ids.ID
}

// windowsOf returns the jobs of kind (notify.coalesce_window or notify.coalesce_flush) of user and
// thread.
func (w *world) windowsOf(kind string, user, thread ids.ID) []windowJob {
	w.t.Helper()
	var out []windowJob
	for _, j := range w.jobsOf(kind) {
		var a struct {
			UserID   ids.ID `json:"user_id"`
			ThreadID ids.ID `json:"thread_id"`
		}
		must(w.t, json.Unmarshal(j.Args, &a))
		if a.UserID == user && (thread.IsZero() || a.ThreadID == thread) {
			out = append(out, windowJob{jobRow: j, User: a.UserID, Thread: a.ThreadID})
		}
	}
	return out
}

// TestTenMessagesInOneWindow_AtMostTwoHints (PLAN row M3-19, 13 section 14 "Ten messages in one
// thread within one window", R-PN-06): Alex creates thread T in Support/Allgemein, so he follows it
// with all, and marks T read up to his own first message; the Owner writes ten messages in T at
// fake clock offsets 0 to 45 seconds within one notify.coalesce_window. The consumer creates one
// hint for Alex (one notify.push_send with Count 1 for his phone), one notify.coalesce_window and
// one notify.coalesce_flush at the window end; the window and flush handlers at the end create the
// second and last hint with Count 10. Variant read: Alex reads T up to the tenth message before
// the end, so the flush creates nothing and counts read. Variant mention: a mention of Alex inside
// the window goes at once and leaves the window unchanged.
func TestTenMessagesInOneWindow_AtMostTwoHints(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"flush", "read", "mention"} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			th, first := w.createThread("alex", w.allgemein, "The export fails")
			if got := w.text(`SELECT level FROM notification_preferences WHERE user_id = $1 AND target_kind = 'thread' AND target_id = $2`,
				w.alex.String(), th.String()); got != "all" {
				t.Fatalf("Alex follows T with %q, want all", got)
			}
			w.readUpTo("alex", th, first.ThreadSeq)
			w.consume()
			if n := len(w.pushJobs()); n != 0 {
				t.Fatalf("%d push jobs before the ten messages", n)
			}
			t0 := w.clk.Now()
			end := t0.Add(60 * time.Second) // notify.coalesce_window of pushEnv
			var msgs []msg
			for i := range 10 {
				w.clk.Set(t0.Add(time.Duration(i*5) * time.Second))
				text := fmt.Sprintf("Update %d", i+1)
				if variant == "mention" && i == 4 {
					text = "@alex please check update 5"
				}
				msgs = append(msgs, w.post("owner", th, text))
				w.consume()
				if variant == "mention" && i == 4 {
					jobs := w.pushJobsFor(w.alex)
					if len(jobs) != 2 || jobs[1].Kind() != domain.KindMention || jobs[1].Args.MessageID != msgs[4].ID ||
						jobs[1].Args.RegistrationID != w.alexPhone.Registration {
						t.Fatalf("the mention did not go at once: %+v", jobs)
					}
				}
			}
			jobs := w.pushJobsFor(w.alex)
			wantJobs := 1
			if variant == "mention" {
				wantJobs = 2
			}
			if len(jobs) != wantJobs {
				t.Fatalf("Alex has %d push jobs after the ten messages, want %d: %+v", len(jobs), wantJobs, jobs)
			}
			if j := jobs[0]; j.Kind() != domain.KindMessage || j.Args.Count != 1 || j.Args.MessageID != msgs[0].ID ||
				j.Args.RegistrationID != w.alexPhone.Registration || j.Args.ThreadID != th {
				t.Fatalf("first hint %+v", j.Args)
			}
			windows, flushes := w.windowsOf(notifyjobs.KindCoalesceWindow, w.alex, th), w.windowsOf(notifyjobs.KindCoalesceFlush, w.alex, th)
			if len(windows) != 1 || !windows[0].ScheduledAt.Equal(end) {
				t.Fatalf("windows %+v, want one at %s", windows, end)
			}
			if len(flushes) != 1 || !flushes[0].ScheduledAt.Equal(end) {
				t.Fatalf("flushes %+v, want one at %s", flushes, end)
			}
			for _, j := range w.pushJobs() {
				if j.Args.RecipientID != w.alex {
					t.Fatalf("a push job for someone else than Alex: %+v", j.Args)
				}
			}
			if variant == "read" {
				w.clk.Set(t0.Add(50 * time.Second))
				w.readUpTo("alex", th, msgs[9].ThreadSeq)
				w.consume()
			}
			readBefore := w.skipped("read")
			w.clk.Set(end)
			w.flushDue()
			hints := w.hintsFor(w.alex)
			switch variant {
			case "read":
				if len(hints) != 1 {
					t.Fatalf("Alex has %d hints after reading, want 1", len(hints))
				}
				if got := w.skipped("read") - readBefore; got != 1 {
					t.Fatalf("bothy.push.skipped read rose by %d, want 1", got)
				}
			default:
				jobs = w.pushJobsFor(w.alex)
				var messageHints []pushJob
				for _, j := range jobs {
					if j.Kind() == domain.KindMessage {
						messageHints = append(messageHints, j)
					}
				}
				if len(hints) != wantJobs+1 || len(messageHints) != 2 {
					t.Fatalf("Alex has %d hints (%d of kind message), want %d and 2: %+v", len(hints), len(messageHints), wantJobs+1, jobs)
				}
				last := messageHints[1]
				if last.Args.Count != 10 || last.Args.MessageID != msgs[9].ID || last.Args.RegistrationID != w.alexPhone.Registration ||
					last.Args.DueAtUnixMicro != end.UnixMicro() {
					t.Fatalf("flush hint %+v, want Count 10 at the tenth message with t0 at the window end", last.Args)
				}
			}
		})
	}
}
