// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestAZM48_MentionWithoutReadRightGivesNothing (PLAN row M3-19, AZ-M48, 13 4.2): Sam writes in
// Marketing/Kampagnen and mentions Alex, who cannot read the channel: the mention row is stored
// (M1-60); for Alex there is no notify.push_send, no window job, no flush job and no log record of
// the push path or the modules naming him; bothy.push.skipped with no_access rose by one.
func TestAZM48_MentionWithoutReadRightGivesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	before := w.skipped("no_access")
	mark := w.logMark()
	m := w.post("sam", w.mainThread(w.kampagnen), "@alex the campaign numbers are in")
	if n := w.count(`SELECT count(*) FROM mentions WHERE message_id = $1 AND principal_id = $2`, m.ID.String(), w.alex.String()); n != 1 {
		t.Fatalf("%d mention rows of Alex, want 1 (M1-60)", n)
	}
	if n := w.consume(); n == 0 {
		t.Fatal("the consumer processed no event")
	}
	if jobs := w.pushJobsFor(w.alex); len(jobs) != 0 {
		t.Fatalf("push jobs for Alex: %+v", jobs)
	}
	for _, kind := range []string{notifyjobs.KindCoalesceWindow, notifyjobs.KindCoalesceFlush} {
		if jobs := w.windowsOf(kind, w.alex, ids.ID{}); len(jobs) != 0 {
			t.Fatalf("%s jobs for Alex: %+v", kind, jobs)
		}
	}
	for _, r := range w.logSince(mark) {
		texts := []string{r.Message}
		for _, v := range r.Attrs {
			texts = append(texts, v)
		}
		for _, s := range texts {
			if strings.Contains(s, w.alex.String()) {
				t.Fatalf("a log record names Alex: %+v", r)
			}
		}
	}
	for _, r := range w.pushLog.Records() {
		for _, v := range append([]string{r.Message}, values(r.Attrs)...) {
			if strings.Contains(v, "alex") {
				t.Fatalf("a record of the push path names Alex: %+v", r)
			}
		}
	}
	if got := w.skipped("no_access") - before; got != 1 {
		t.Fatalf("bothy.push.skipped no_access rose by %d, want 1", got)
	}
}

// values returns the values of m.
func values(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// TestRevocationBetweenEventAndSend (13 section 14 "Revocation between event and sending", 04 7.6,
// R-AZ-11): Sam (level all on T) gets a hint; before the job runs the Owner removes Sam from the
// group Leser; the job sends nothing and counts no_access.
func TestRevocationBetweenEventAndSend(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	w.setLevel("sam", th, "all")
	m := w.post("owner", th, "The export is back")
	w.consume()
	jobs := w.pushJobsFor(w.sam)
	if len(jobs) != 1 || jobs[0].Kind() != domain.KindMessage || jobs[0].Args.MessageID != m.ID || jobs[0].Args.RegistrationID != w.samPhone.Registration {
		t.Fatalf("Sam's push jobs %+v, want one message hint for his phone", jobs)
	}
	w.removeMember(w.leser, w.sam)
	before := w.skipped("no_access")
	if err := w.send(jobs[0]); err != nil {
		t.Fatalf("the job failed: %v", err)
	}
	if calls := w.fcm.Calls(); len(calls) != 0 {
		t.Fatalf("the provider was called after the revocation: %v", calls)
	}
	if got := w.skipped("no_access") - before; got != 1 {
		t.Fatalf("bothy.push.skipped no_access rose by %d, want 1", got)
	}
}

// TestDMHint (13 section 2, row dm): in a fixture dm channel of Alex and Sam, Sam's message gives
// Alex one dm hint, also when it mentions Alex; nobody else gets a hint.
func TestDMHint(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.dmChannel(w.alex, w.sam)
	plain := w.post("sam", th, "Lunch at noon?")
	w.consume()
	w.advance(2 * time.Minute) // past notify.coalesce_window, so the second message is judged on its own
	w.flushDue()
	mention := w.post("sam", th, "@alex are you there?")
	w.consume()
	jobs := w.pushJobs()
	if len(jobs) != 2 {
		t.Fatalf("push jobs %+v, want two for Alex", jobs)
	}
	for i, want := range []msg{plain, mention} {
		j := jobs[i]
		if j.Args.RecipientID != w.alex || j.Kind() != domain.KindDM || j.Args.MessageID != want.ID || j.Args.ThreadID != th ||
			j.Args.RegistrationID != w.alexPhone.Registration {
			t.Fatalf("job %d %+v, want a dm hint of Alex for message %s", i, j.Args, want.ID)
		}
	}
	if hints := w.hintsFor(w.alex); len(hints) != 2 {
		t.Fatalf("Alex has %d hints, want 2", len(hints))
	}
}

// prefRow is a row of notification_preferences.
type prefRow struct {
	Level   string
	Version int64
}

// pref reads the preference of user on the thread; ok is false without a row.
func (w *world) pref(user, thread ids.ID) (prefRow, bool) {
	w.t.Helper()
	rs, err := w.db.Query(w.t.Context(), `SELECT level, version FROM notification_preferences WHERE user_id = $1 AND target_kind = 'thread'
		AND target_id = $2`, user.String(), thread.String())
	must(w.t, err)
	defer rs.Close()
	var p prefRow
	if !rs.Next() {
		return p, false
	}
	must(w.t, rs.Scan(&p.Level, &p.Version))
	return p, true
}

// prefEvents returns the number of notification_pref.updated events with audience user on the
// thread, and how many of them share the transaction of the creation event of message (equal
// occurred_at, the transaction time).
func (w *world) prefEvents(user, thread, message ids.ID) (all, sameTx int64) {
	all = w.count(`SELECT count(*) FROM event_log WHERE type = 'notification_pref.updated' AND audience_principal_id = $1
		AND payload->'preference'->>'target_id' = $2`, user.String(), thread.String())
	sameTx = w.count(`SELECT count(*) FROM event_log p JOIN event_log m ON m.type = 'message.created' AND m.resource_id = $3
		AND m.occurred_at = p.occurred_at
		WHERE p.type = 'notification_pref.updated' AND p.audience_principal_id = $1 AND p.payload->'preference'->>'target_id' = $2`,
		user.String(), thread.String(), message.String())
	return all, sameTx
}

// TestAutomaticFollowing (13 section 14 "Automatic following", 13 3.2, R-PN-04, OQ-05): the Owner
// creates thread T2; Alex's first answer in T2 creates his row with all and one
// notification_pref.updated (audience Alex) in the same transaction; his second answer writes
// nothing; his message in the main thread of Allgemein writes nothing; with Alex's stored mute on
// a thread nothing changes; Lotse's message writes nothing; createThread by Alex makes him follow
// the new thread; a subscriber of messaging.MessagePosted registered after notify that vetoes once
// leaves neither row nor event.
func TestAutomaticFollowing(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withVeto())
	t2 := w.newThread(w.allgemein)
	first := w.post("alex", t2, "I can reproduce it")
	p, ok := w.pref(w.alex, t2)
	if !ok || p.Level != "all" || p.Version != 1 {
		t.Fatalf("Alex's row on T2 %+v %v, want all with version 1", p, ok)
	}
	if all, same := w.prefEvents(w.alex, t2, first.ID); all != 1 || same != 1 {
		t.Fatalf("%d notification_pref.updated of Alex on T2 (%d in the transaction of the message), want 1 and 1", all, same)
	}
	w.post("alex", t2, "Here is the log")
	if p2, _ := w.pref(w.alex, t2); p2 != p {
		t.Fatalf("the second answer changed the row: %+v", p2)
	}
	if all, _ := w.prefEvents(w.alex, t2, first.ID); all != 1 {
		t.Fatalf("the second answer wrote an event: %d", all)
	}
	main := w.mainThread(w.allgemein)
	w.post("alex", main, "Status update")
	if _, ok := w.pref(w.alex, main); ok {
		t.Fatal("a message in the main thread wrote a preference")
	}
	if all, _ := w.prefEvents(w.alex, main, ids.ID{}); all != 0 {
		t.Fatal("a message in the main thread wrote notification_pref.updated")
	}
	t3 := w.newThread(w.allgemein)
	w.setLevel("alex", t3, "mute")
	muted, _ := w.pref(w.alex, t3)
	eventsBefore, _ := w.prefEvents(w.alex, t3, ids.ID{})
	w.post("alex", t3, "Muted, but answering")
	if got, _ := w.pref(w.alex, t3); got != muted || got.Level != "mute" {
		t.Fatalf("the answer changed the stored mute: %+v, was %+v", got, muted)
	}
	if all, _ := w.prefEvents(w.alex, t3, ids.ID{}); all != eventsBefore {
		t.Fatal("the answer under a stored mute wrote notification_pref.updated")
	}
	w.post("lotse", t2, "Looking into it")
	if n := w.count(`SELECT count(*) FROM notification_preferences WHERE user_id = $1`, w.lotse.String()); n != 0 {
		t.Fatalf("%d preference rows of Lotse", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'notification_pref.updated' AND audience_principal_id = $1`, w.lotse.String()); n != 0 {
		t.Fatal("Lotse's message wrote notification_pref.updated")
	}
	own, firstOwn := w.createThread("alex", w.allgemein, "New topic")
	if p, ok := w.pref(w.alex, own); !ok || p.Level != "all" || p.Version != 1 {
		t.Fatalf("createThread by Alex: row %+v %v, want all", p, ok)
	}
	if all, same := w.prefEvents(w.alex, own, firstOwn.ID); all != 1 || same != 1 {
		t.Fatalf("createThread by Alex: %d events (%d in its transaction), want 1 and 1", all, same)
	}
	// The vetoing subscriber after notify refuses this one message (clear_draft): the unit rolls
	// back the row and the event of the following with the message.
	t4 := w.newThread(w.allgemein)
	headBefore := w.head()
	r := w.postBody("alex", t4, "Vetoed answer", `"clear_draft":true`)
	if r.StatusCode < http.StatusBadRequest {
		t.Fatalf("the vetoed message was accepted: %d %s", r.StatusCode, r.Body)
	}
	if _, ok := w.pref(w.alex, t4); ok {
		t.Fatal("the vetoed message left a preference row")
	}
	if all, _ := w.prefEvents(w.alex, t4, ids.ID{}); all != 0 {
		t.Fatal("the vetoed message left notification_pref.updated")
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND content_md = 'Vetoed answer'`, t4.String()); n != 0 {
		t.Fatal("the vetoed message was stored")
	}
	if got := w.head(); got != headBefore {
		t.Fatalf("the vetoed unit appended events: head %d, was %d", got, headBefore)
	}
}
