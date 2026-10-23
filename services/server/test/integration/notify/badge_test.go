// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fakepush"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// badgeSyncInterval is notify.badge_sync_min_interval of pushEnv.
const badgeSyncInterval = 30 * time.Second

// badge returns BadgeCounter.For of user at the clock of the world.
func (w *world) badge(user ids.ID) int64 {
	w.t.Helper()
	n, err := w.push.Push.Badge().For(context.Background(), user)
	must(w.t, err)
	return n
}

// markUnread sends markThreadUnread as name for thread with last_read_seq seq and the current
// version of the person's read state, and expects 200.
func (w *world) markUnread(name string, user, thread ids.ID, seq int64) {
	w.t.Helper()
	version := w.count(`SELECT version FROM read_states WHERE user_id = $1 AND thread_id = $2`, user.String(), thread.String())
	w.wantOK(w.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/read-state/mark-unread",
		`{"last_read_seq":`+strconv.FormatInt(seq, 10)+`}`, ifMatch(version)...))
}

// TestBadgeCount (13 section 8, design note 16 a): a table over BadgeCounter.For with Alex, each
// row adding one thread and the change it must make to the number: an unread mention in a thread
// with level mentions counts; an unread thread with level all counts; an unread thread with level
// mentions and no mention does not; an unread DM thread counts; a read thread does not; a thread
// whose messages lie below unread_floor_seq does not; a thread in Marketing where Sam mentioned
// Alex (stored by M1-60, Alex cannot read it) never counts. The Badge of the Notification that
// FakeProvider records equals For at the fake send time and HasBadge is true. After the Owner
// removes Alex's read right on Support a counted thread there stops counting.
func TestBadgeCount(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	if n := w.badge(w.alex); n != 0 {
		t.Fatalf("Alex's badge in the fresh world is %d, want 0", n)
	}
	var mentionThread ids.ID
	var mentionMsg msg
	rows := []struct {
		name  string
		setup func()
		delta int64
	}{
		{"unread mention with level mentions", func() {
			mentionThread = w.newThread(w.allgemein)
			mentionMsg = w.post("owner", mentionThread, "@alex please check the export")
		}, 1},
		{"unread with level all", func() {
			th := w.newThread(w.allgemein)
			w.setLevel("alex", th, "all")
			w.post("owner", th, "Export numbers are in")
		}, 1},
		{"unread with level mentions and no mention", func() {
			th := w.newThread(w.allgemein)
			w.post("owner", th, "Nothing for Alex here")
		}, 0},
		{"unread dm thread", func() {
			th := w.dmChannel(w.alex, w.sam)
			w.post("sam", th, "Lunch at noon?")
		}, 1},
		{"read thread with level all", func() {
			th := w.newThread(w.allgemein)
			w.setLevel("alex", th, "all")
			m := w.post("owner", th, "Already seen")
			w.readUpTo("alex", th, m.ThreadSeq)
		}, 0},
		{"messages below unread_floor_seq", func() {
			th := w.newThread(w.allgemein)
			w.setLevel("alex", th, "all")
			w.post("owner", th, "Below the floor")
			w.sql(`UPDATE threads SET unread_floor_seq = last_message_seq WHERE id = $1`, th.String())
		}, 0},
		{"mention in a channel Alex cannot read", func() {
			m := w.post("sam", w.mainThread(w.kampagnen), "@alex the campaign numbers are in")
			if n := w.count(`SELECT count(*) FROM mentions WHERE message_id = $1 AND principal_id = $2`, m.ID.String(), w.alex.String()); n != 1 {
				w.t.Fatalf("%d mention rows of Alex, want 1 (M1-60)", n)
			}
		}, 0},
	}
	var want int64
	for _, r := range rows {
		r.setup()
		want += r.delta
		if got := w.badge(w.alex); got != want {
			t.Fatalf("after %q Alex's badge is %d, want %d", r.name, got, want)
		}
	}

	// The badge a push carries is For at the send time.
	w.consume()
	var job pushJob
	for _, j := range w.pushJobsFor(w.alex) {
		if j.Args.MessageID == mentionMsg.ID {
			job = j
		}
	}
	if job.ID == 0 || job.Kind() != domain.KindMention {
		t.Fatalf("no mention job of Alex for the message in thread %s: %+v", mentionThread, w.pushJobsFor(w.alex))
	}
	w.advance(time.Minute)
	must(t, w.send(job))
	calls := w.apns.CallsFor(w.alexPhone.Registration)
	if len(calls) != 1 {
		t.Fatalf("%d provider calls for Alex's phone, want 1", len(calls))
	}
	if n := calls[0].Notification; !n.HasBadge || n.Badge != w.badge(w.alex) || n.Badge != want {
		t.Fatalf("the push carries badge %d (HasBadge %v), want For at the send time %d", n.Badge, n.HasBadge, want)
	}

	// Without the read right on Support only the DM thread counts.
	w.revoke(w.g1)
	if got := w.badge(w.alex); got != 1 {
		t.Fatalf("after the revocation of G-1 Alex's badge is %d, want 1 (the DM thread)", got)
	}
}

// syncJobs returns the jobs notify.push_send of kind sync of recipient.
func (w *world) syncJobs(recipient ids.ID) []pushJob {
	return slices.DeleteFunc(w.pushJobsFor(recipient), func(j pushJob) bool { return j.Kind() != domain.KindSync })
}

// wantSyncCall sends the one new sync job of recipient for registration reg through provider p
// and checks the recorded call: kind sync, badge as For at the send time and want, Cleared, and no
// thread, message, target, quiet flag or count.
func (w *world) wantSyncCall(p *fakepush.FakeProvider, recipient, reg ids.ID, done int, badge int64, cleared ...ids.ID) {
	w.t.Helper()
	jobs := w.syncJobs(recipient)
	if len(jobs) != done+1 {
		w.t.Fatalf("%d sync jobs, want %d: %+v", len(jobs), done+1, jobs)
	}
	j := jobs[done]
	if j.Args.RegistrationID != reg {
		w.t.Fatalf("sync job %+v, want registration %s", j.Args, reg)
	}
	before := len(p.CallsFor(reg))
	must(w.t, w.send(j))
	calls := p.CallsFor(reg)
	if len(calls) != before+1 {
		w.t.Fatalf("%d provider calls, want %d", len(calls), before+1)
	}
	n := calls[len(calls)-1].Notification
	if n.Kind != notifyapp.KindSync || !n.HasBadge || n.Badge != badge || n.Badge != w.badge(recipient) || !slices.Equal(n.Cleared, cleared) ||
		!n.ThreadID.IsZero() || !n.MessageID.IsZero() || !n.TargetID.IsZero() || n.Quiet || n.Count != 0 || n.ID != j.Args.NotificationID {
		w.t.Fatalf("sync notification %+v, want kind sync with badge %d and Cleared %v", n, badge, cleared)
	}
}

// TestBadgeSync (13 section 8, design note 16 b and c, fake clock): Alex has two counted unread
// threads in Support/Allgemein and reads the first through putReadState: one notify.badge_sync at
// once, then one notify.push_send of kind sync for his phone, recorded with the lower Badge,
// Cleared = that thread and no thread, message or quiet flag. A second falling read before
// notify.badge_sync_min_interval ends creates one sync at the end of the interval and none
// before. A read that does not lower the number (a thread with level mentions and no mention, or
// a second read of the read thread) and markThreadUnread give no sync. Sam's fcm registration
// gets the sync with Cleared when Sam reads.
func TestBadgeSync(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var threads []ids.ID
	var msgs []msg
	for range 2 {
		th := w.newThread(w.allgemein)
		w.setLevel("alex", th, "all")
		threads, msgs = append(threads, th), append(msgs, w.post("owner", th, "Export numbers are in"))
	}
	w.consume()
	if n := w.badge(w.alex); n != 2 {
		t.Fatalf("Alex's badge is %d, want 2", n)
	}
	if n := len(w.jobsOf(notifyjobs.KindBadgeSync)); n != 0 {
		t.Fatalf("%d badge syncs before any read", n)
	}

	t0 := w.clk.Now()
	w.readUpTo("alex", threads[0], msgs[0].ThreadSeq)
	w.consume()
	syncs := w.pendingOf(notifyjobs.KindBadgeSync)
	if len(syncs) != 1 || !syncs[0].ScheduledAt.Equal(t0) {
		t.Fatalf("badge syncs %+v, want one at once (%s)", syncs, t0)
	}
	w.badgeSyncsDue()
	w.wantSyncCall(w.apns, w.alex, w.alexPhone.Registration, 0, 1, threads[0])
	holds := w.pendingOf(notifyjobs.KindBadgeSyncHold)
	if len(holds) != 1 || !holds[0].ScheduledAt.Equal(t0.Add(badgeSyncInterval)) {
		t.Fatalf("holds %+v, want one until %s", holds, t0.Add(badgeSyncInterval))
	}

	// The second falling read inside the interval waits for its end.
	w.clk.Set(t0.Add(10 * time.Second))
	w.readUpTo("alex", threads[1], msgs[1].ThreadSeq)
	w.consume()
	syncs = w.pendingOf(notifyjobs.KindBadgeSync)
	if len(syncs) != 1 || !syncs[0].ScheduledAt.Equal(t0.Add(badgeSyncInterval)) {
		t.Fatalf("badge syncs %+v, want one at the end of the interval %s", syncs, t0.Add(badgeSyncInterval))
	}
	w.badgeSyncsDue()
	if n := len(w.syncJobs(w.alex)); n != 1 {
		t.Fatalf("%d sync pushes before the interval ended, want 1", n)
	}
	w.clk.Set(t0.Add(badgeSyncInterval))
	w.badgeSyncsDue()
	w.wantSyncCall(w.apns, w.alex, w.alexPhone.Registration, 1, 0, threads[1])

	// Reads that do not lower the number, and a mark as unread, give no sync.
	w.clk.Set(t0.Add(3 * badgeSyncInterval))
	w.badgeSyncsDue()
	quiet := w.newThread(w.allgemein)
	q := w.post("owner", quiet, "Nothing for Alex here")
	w.consume()
	syncsBefore := len(w.jobsOf(notifyjobs.KindBadgeSync))
	w.readUpTo("alex", quiet, q.ThreadSeq)
	w.readUpTo("alex", threads[0], msgs[0].ThreadSeq)
	w.markUnread("alex", w.alex, threads[0], msgs[0].ThreadSeq-1)
	w.consume()
	if n := len(w.jobsOf(notifyjobs.KindBadgeSync)); n != syncsBefore {
		t.Fatalf("%d badge syncs after reads that do not lower the number, want %d", n, syncsBefore)
	}
	if n := w.badge(w.alex); n != 1 {
		t.Fatalf("Alex's badge after the mark as unread is %d, want 1", n)
	}

	// Sam's fcm registration gets the sync when Sam reads.
	own := w.newThread(w.allgemein)
	w.setLevel("sam", own, "all")
	m := w.post("owner", own, "For Sam")
	w.consume()
	if n := w.badge(w.sam); n != 1 {
		t.Fatalf("Sam's badge is %d, want 1", n)
	}
	w.readUpTo("sam", own, m.ThreadSeq)
	w.consume()
	w.badgeSyncsDue()
	w.wantSyncCall(w.fcm, w.sam, w.samPhone.Registration, 0, 0, own)
}
