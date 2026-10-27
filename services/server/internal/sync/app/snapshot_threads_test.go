// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	syncmod "github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// The tests of the active threads of the snapshot (M2-52, 04 section 6.3 item 4, R-SY-28) run
// with a window of one day and a bound of three threads per channel; the activity of a thread is
// set relative to the clock of the world, which the use case measures the window from.
const (
	atWindow = 24 * time.Hour
	atBound  = 3
)

// atWorld is a sync world with the bounds above, a space Support with a forum and Alex, who
// reads the space; readers are the people who read every new post at once.
type atWorld struct {
	*mw.World
	space, forum, alex ids.ID
	readers            []string
}

// newATWorld builds the world.
func newATWorld(t *testing.T) *atWorld {
	t.Helper()
	w := mw.NewSync(t, noSubscriber, mw.SyncOptions{CatchUp: app.CatchUpConfig{PageSize: 50, MaxEvents: 1000}, PartitionSize: 50,
		Snapshot: app.SnapshotConfig{ThreadWindow: atWindow, ThreadsPerChannel: atBound}})
	space := w.Space("Support")
	forum := w.Channel(space, "forum", "tickets")
	alex := w.Person("alex", false)
	w.Grant("user", alex, space, "read")
	return &atWorld{World: w, space: space, forum: forum, alex: alex, readers: []string{"alex"}}
}

// post creates a forum post in forum as the owner with its first message, which every reader
// reads at once (so nothing of it is unread), and sets its last activity to age before the clock
// of the world.
func (w *atWorld) post(forum ids.ID, title string, age time.Duration) ids.ID {
	body := `{"title":"` + title + `","first_message":` + mw.Body(ids.New(), "First message of "+title) + `}`
	m := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), body, mw.Key()...), http.StatusCreated, "")
	id := ids.MustParse(m["thread"].(map[string]any)["id"].(string))
	if forum != w.forum {
		w.age(id, age)
		return id
	}
	for _, name := range w.readers {
		w.read(name, id, 1)
	}
	w.age(id, age)
	return id
}

// read sets the read position of name in thread to seq.
func (w *atWorld) read(name string, thread ids.ID, seq int) {
	w.Want(w.Do(name, http.MethodPut, "/api/v1/threads/"+thread.String()+"/read-state", `{"last_read_seq":`+strconv.Itoa(seq)+`}`),
		http.StatusOK, "")
}

// age sets the last activity of thread to age before the clock of the world.
func (w *atWorld) age(thread ids.ID, age time.Duration) {
	w.SQL(`UPDATE threads SET last_activity_at = $2 WHERE id = $1`, thread.String(), w.Clock.Now().Add(-age))
}

// snapshot builds the snapshot of the principal registered under name.
func (w *atWorld) snapshot(t *testing.T, name string) syncmod.SnapshotView {
	t.Helper()
	v, err := w.Sync.Snapshot.Build(actor.WithActor(context.Background(), w.Actor(name)))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// active returns the ids of the active threads of channel in v, in their order.
func active(v syncmod.SnapshotView, channel ids.ID) []ids.ID {
	var out []ids.ID
	for _, th := range v.ActiveThreads {
		if th.ChannelID == channel {
			out = append(out, th.ID)
		}
	}
	return out
}

// TestActiveThreads_BoundAndOrder: a forum with more open threads within the window than
// sync.snapshot_threads_per_channel yields exactly the bound, newest activity first, each as the
// thread document (title, state open, tags never nil).
func TestActiveThreads_BoundAndOrder(t *testing.T) {
	t.Parallel()
	w := newATWorld(t)
	var posts []ids.ID
	for i := range atBound + 2 {
		posts = append(posts, w.post(w.forum, "post "+strconv.Itoa(i), time.Duration(i+1)*time.Hour))
	}
	v := w.snapshot(t, "alex")
	if got := active(v, w.forum); !slices.Equal(got, posts[:atBound]) {
		t.Fatalf("active threads %v, want the %d newest %v", got, atBound, posts[:atBound])
	}
	for _, th := range v.ActiveThreads {
		if th.Kind != "thread" || th.State != "open" || th.Title == "" || th.TagIDs == nil || th.Version < 1 {
			t.Fatalf("thread %+v", th)
		}
	}
}

// TestActiveThreads_ActiveRunAlwaysIncluded: an open thread with an active run and activity older
// than the window is included although the channel already holds the bound of newer threads; it
// counts toward the bound, so the newest threads fill only the rest.
func TestActiveThreads_ActiveRunAlwaysIncluded(t *testing.T) {
	t.Parallel()
	w := newATWorld(t)
	var posts []ids.ID
	for i := range atBound {
		posts = append(posts, w.post(w.forum, "post "+strconv.Itoa(i), time.Duration(i+1)*time.Hour))
	}
	old := w.post(w.forum, "long run", 3*atWindow)
	helper := w.Agent("helper")
	run := ids.New()
	w.SQL(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, billing_mode_expected, invocation_jti)
		VALUES ($1, $2, $3, 'message', $4, 'organization', $5)`, run.String(), old.String(), helper.String(), w.OwnerID.String(), ids.New().String())
	v := w.snapshot(t, "alex")
	if !slices.ContainsFunc(v.ActiveRuns, func(r syncmod.RunSummary) bool { return r.ID == run }) {
		t.Fatalf("the run is not active for Alex: %+v", v.ActiveRuns)
	}
	if got, want := active(v, w.forum), append(slices.Clone(posts[:atBound-1]), old); !slices.Equal(got, want) {
		t.Fatalf("active threads %v, want %v", got, want)
	}
}

// TestActiveThreads_UnreadAlwaysIncluded: an open thread with an unread mention of the caller and
// one with only unread messages, both older than the window and the oldest of a channel that
// already holds the bound of newer threads, are both included and count toward the bound; for
// Robin, who has read them, they are absent and the newest threads fill the bound.
func TestActiveThreads_UnreadAlwaysIncluded(t *testing.T) {
	t.Parallel()
	w := newATWorld(t)
	robin := w.Person("robin", false)
	w.Grant("user", robin, w.space, "read")
	w.readers = append(w.readers, "robin")
	var posts []ids.ID
	for i := range atBound {
		posts = append(posts, w.post(w.forum, "post "+strconv.Itoa(i), time.Duration(i+1)*time.Hour))
	}
	mention := w.post(w.forum, "mention", 0)
	plain := w.post(w.forum, "plain", 0)
	w.Send("owner", mention, "@alex please look", http.StatusCreated)
	w.Send("owner", plain, "a plain update", http.StatusCreated)
	for _, th := range []ids.ID{mention, plain} {
		w.read("robin", th, 2)
	}
	w.age(mention, 3*atWindow)
	w.age(plain, 4*atWindow)

	v := w.snapshot(t, "alex")
	if len(v.UnreadThreads) != 2 {
		t.Fatalf("unread threads of Alex %+v", v.UnreadThreads)
	}
	if got, want := active(v, w.forum), append(slices.Clone(posts[:atBound-2]), mention, plain); !slices.Equal(got, want) {
		t.Fatalf("active threads of Alex %v, want %v", got, want)
	}
	r := w.snapshot(t, "robin")
	if len(r.UnreadThreads) != 0 {
		t.Fatalf("unread threads of Robin %+v", r.UnreadThreads)
	}
	if got := active(r, w.forum); !slices.Equal(got, posts) {
		t.Fatalf("active threads of Robin %v, want %v", got, posts)
	}
}

// TestActiveThreads_Excludes: done threads, main threads (also an unread one), threads older than
// the window without an active run and threads of channels the caller may not read are absent;
// only the one recent open post remains.
func TestActiveThreads_Excludes(t *testing.T) {
	t.Parallel()
	w := newATWorld(t)
	keep := w.post(w.forum, "recent", time.Hour)
	done := w.post(w.forum, "done", time.Hour)
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/threads/"+done.String()+"/close", "", mw.Key()...), http.StatusOK, "")
	w.post(w.forum, "stale", 2*atWindow)
	text := w.Channel(w.space, "text", "general")
	main := w.MainThread(text)
	w.Send("owner", main, "news for everybody", http.StatusCreated)
	hidden := w.Space("Finance")
	other := w.Channel(hidden, "forum", "budget")
	w.post(other, "secret", time.Hour)

	v := w.snapshot(t, "alex")
	if !slices.ContainsFunc(v.UnreadThreads, func(u syncmod.UnreadThread) bool { return u.ThreadID == main }) {
		t.Fatalf("the main thread is not unread for Alex: %+v", v.UnreadThreads)
	}
	if len(v.ActiveThreads) != 1 || v.ActiveThreads[0].ID != keep {
		t.Fatalf("active threads %+v, want only %s", v.ActiveThreads, keep)
	}
	if o := w.snapshot(t, "owner"); !slices.ContainsFunc(o.ActiveThreads, func(th syncmod.SnapshotThread) bool { return th.ChannelID == other }) {
		t.Fatalf("the owner, who reads Finance, misses its post: %+v", o.ActiveThreads)
	}
}
