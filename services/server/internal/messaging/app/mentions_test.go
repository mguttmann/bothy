// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// fakeMentionStore serves a fixed mention list; every other Store method is unused.
type fakeMentionStore struct {
	bareStore
	rows []app.MentionRow
}

func (f fakeMentionStore) MyMentions(_ context.Context, q app.MentionQuery) ([]app.MentionRow, error) {
	var out []app.MentionRow
	for _, r := range f.rows {
		if q.HasAfter && !r.MentionedAt.Before(q.AfterAt) {
			continue
		}
		out = append(out, r)
		if len(out) == q.Limit {
			break
		}
	}
	return out, nil
}

func (fakeMentionStore) Mentions(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }

// denyOne allows everything except message.read on one message, the way a SQL filter and
// authorize could disagree.
type denyOne struct {
	deny    ids.ID
	channel ids.ID
}

func (d denyOne) Authorize(_ context.Context, _ authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if a == authorize.ActionMessageRead && r.ID == d.deny {
		return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (d denyOne) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{ChannelIDs: []ids.ID{d.channel}}, nil
}

// fakeDirectory knows every principal as Sam.
type fakeDirectory struct{ identity.Directory }

func (fakeDirectory) Principal(_ context.Context, id ids.ID) (identity.PrincipalView, error) {
	return identity.PrincipalView{ID: id, Kind: actor.KindUser, Handle: "sam", DisplayName: "Sam"}, nil
}

// plainCursors encodes the key as JSON without binding checks.
type plainCursors struct{}

func (plainCursors) Encode(_ string, _ []byte, key any) (string, error) {
	b, err := json.Marshal(key)
	return string(b), err
}

func (plainCursors) Decode(_ string, _ []byte, s string, key any) error {
	return json.Unmarshal([]byte(s), key)
}

// counter records the reasons of bothy.authz.errors.
type counter struct {
	mu      sync.Mutex
	reasons []string
}

func (c *counter) CountError(_ context.Context, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reasons = append(c.reasons, reason)
}

// TestListMyMentions_DropsItemDeniedByAuthorize (07 section 7.4 rule 3): an item the SQL filter
// kept and authorize refuses is dropped, logged without ids and counted with bothy.reason =
// filter_mismatch; the page is filled from the following rows.
func TestListMyMentions_DropsItemDeniedByAuthorize(t *testing.T) {
	t.Parallel()
	alex, channel, thread := ids.New(), ids.New(), ids.New()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	var rows []app.MentionRow
	for i := range 5 {
		m := domain.Message{ID: ids.New(), ThreadID: thread, ChannelID: channel, ThreadSeq: int64(i + 1), AuthorID: ids.New(),
			Origin: domain.OriginHuman, Kind: domain.KindText, ContentMD: "@alex " + strconv.Itoa(i), ContentFormat: 1, Version: 1}
		rows = append(rows, app.MentionRow{Message: m, MentionedAt: at.Add(-time.Duration(i) * time.Second)})
	}
	denied := rows[1].Message.ID
	logs := &httpservertest.LogCapture{}
	cnt := &counter{}
	svc, err := app.New(app.Config{MaxContentBytes: 100, MaxMentions: 4, EventPayloadMaxBytes: 1000, TemplateNameMaxChars: 10,
		TemplateMaxPerScope: 5, ListContentMaxBytes: 100, PageSizeDefault: 3, PageSizeMax: 3, EmojiMaxBytes: 64, MaxDistinctReactions: 20, MaxPinsPerThread: 50}, app.Deps{
		Store: fakeMentionStore{rows: rows}, UoW: struct{ txn.UnitOfWork }{}, Authorizer: denyOne{deny: denied, channel: channel},
		Events: struct{ eventlog.Appender }{}, Threads: struct{ chat.Threads }{}, Directory: fakeDirectory{},
		Cursors: plainCursors{}, Errors: cnt, Log: slog.New(logs),
		Redactor: struct{ eventlog.Redactor }{}, Audit: struct{ auditing.Recorder }{}, Symbols: anySymbol{},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: alex, Kind: actor.KindUser})
	page, err := svc.Mentions().ListMine(ctx, messaging.MentionListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	var got []ids.ID
	for _, it := range page.Items {
		got = append(got, it.Message.ID)
	}
	want := []ids.ID{rows[0].Message.ID, rows[2].Message.ID, rows[3].Message.ID}
	if !slices.Equal(got, want) || page.NextCursor == "" {
		t.Fatalf("items %v, want %v; next %q", got, want, page.NextCursor)
	}
	if !slices.Equal(cnt.reasons, []string{"filter_mismatch"}) {
		t.Fatalf("counted %v", cnt.reasons)
	}
	// The next page starts after the cursor; a cursor without a position is cursor_invalid.
	next, err := svc.Mentions().ListMine(ctx, messaging.MentionListQuery{Cursor: page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].Message.ID != rows[4].Message.ID || next.NextCursor != "" {
		t.Fatalf("second page %+v %v", next, err)
	}
	if _, err := svc.Mentions().ListMine(ctx, messaging.MentionListQuery{Cursor: `{"at":"0001-01-01T00:00:00Z","id":"00000000-0000-0000-0000-000000000000"}`}); err == nil {
		t.Fatal("zero cursor accepted")
	}
	if _, err := svc.Mentions().ListMine(ctx, messaging.MentionListQuery{Cursor: "not json"}); err == nil {
		t.Fatal("broken cursor accepted")
	}
	recs := logs.Records()
	if len(recs) != 1 || recs[0].Attrs["bothy.reason"] != "filter_mismatch" || recs[0].Attrs["bothy.action"] != "message.read" {
		t.Fatalf("logs %+v", recs)
	}
	for k, v := range recs[0].Attrs {
		for _, id := range []ids.ID{alex, channel, thread, denied} {
			if v == id.String() {
				t.Fatalf("log attribute %s carries an id", k)
			}
		}
	}
}

// seed sends "@alex hi" from the owner into thread and returns the message's thread_seq.
func seed(w *mw.World, thread ids.ID) int64 {
	m := w.Send("owner", thread, "@alex hi", http.StatusCreated)
	return int64(m["thread_seq"].(float64))
}

// counts calls CountUnread and returns the result keyed by thread.
func counts(t *testing.T, w *mw.World, q messaging.UnreadMentionQuery) map[ids.ID]int {
	t.Helper()
	list, err := w.Svc.Mentions().CountUnread(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	out := map[ids.ID]int{}
	for i, c := range list {
		if c.Count < 1 {
			t.Errorf("zero count for %s", c.ThreadID)
		}
		if i > 0 && list[i-1].ThreadID.String() >= c.ThreadID.String() {
			t.Errorf("not ascending by thread")
		}
		out[c.ThreadID] = c.Count
	}
	return out
}

// TestCountUnread_FloorsAndDeleted (brief note 4): counts respect LastReadSeq, the default floor
// is 0, tombstones are ignored and threads with no unread mention are omitted.
func TestCountUnread_FloorsAndDeleted(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	alex := w.Person("alex", false)
	ch1, ch2, ch3 := w.Channel(w.Space("One"), "text", "c"), w.Channel(w.Space("Two"), "text", "c"), w.Channel(w.Space("Three"), "text", "c")
	main1, main2, main3 := w.MainThread(ch1), w.MainThread(ch2), w.MainThread(ch3)
	s1 := seed(w, main1)
	seed(w, main1)
	seed(w, main1)
	w.Send("owner", main1, "no mention", http.StatusCreated)
	seed(w, main2)
	m := w.Send("owner", main2, "@alex deleted", http.StatusCreated)
	w.SQL(`UPDATE messages SET deleted_at = now(), deleted_by = author_principal_id, content_md = '', version = version + 1 WHERE id = $1`, m["id"])
	last3 := seed(w, main3)
	visible := []ids.ID{ch1, ch2, ch3}
	got := counts(t, w, messaging.UnreadMentionQuery{PrincipalID: alex, VisibleChannelIDs: visible})
	if got[main1] != 3 || got[main2] != 1 || got[main3] != 1 || len(got) != 3 {
		t.Fatalf("floor 0: %v", got)
	}
	got = counts(t, w, messaging.UnreadMentionQuery{PrincipalID: alex, VisibleChannelIDs: visible,
		LastReadSeq: map[ids.ID]int64{main1: s1, main3: last3, ids.New(): 99}})
	if got[main1] != 2 || got[main2] != 1 || len(got) != 2 {
		t.Fatalf("with read states: %v", got)
	}
	if got := counts(t, w, messaging.UnreadMentionQuery{PrincipalID: alex}); len(got) != 0 {
		t.Fatalf("no visible channel: %v", got)
	}
	if got := counts(t, w, messaging.UnreadMentionQuery{PrincipalID: alex, VisibleChannelIDs: []ids.ID{ch2}}); len(got) != 1 || got[main2] != 1 {
		t.Fatalf("one channel: %v", got)
	}
}

// TestAZP5_CountUnreadRandomWorlds (07 AZ-P5, mentions): in seeded random worlds of grants,
// mentions and read states, CountUnread with the person's Scope equals a naive reference that
// filters every mention with authorize(message.read), and never returns a thread of a channel
// the person cannot read.
func TestAZP5_CountUnreadRandomWorlds(t *testing.T) {
	t.Parallel()
	for _, seedValue := range []uint64{1, 7, 42} {
		t.Run(strconv.FormatUint(seedValue, 10), func(t *testing.T) {
			t.Parallel()
			r := rand.New(rand.NewPCG(seedValue, seedValue^0x9e3779b97f4a7c15)) //nolint:gosec // G404: a seeded, repeatable test world, no secret
			w := mw.New(t, subscribe)
			people := []string{"alex", "sam", "robin"}
			pid := map[string]ids.ID{}
			for _, p := range people {
				pid[p] = w.Person(p, false)
			}
			type thread struct{ id, channel ids.ID }
			var threads []thread
			for s := range 3 {
				space := w.Space("Space " + strconv.Itoa(s))
				for _, p := range people {
					if r.IntN(2) == 0 {
						w.Grant("user", pid[p], space, "read")
					}
				}
				for c := range 2 {
					ch := w.Channel(space, "text", "c"+strconv.Itoa(c))
					threads = append(threads, thread{id: w.MainThread(ch), channel: ch})
				}
			}
			for range 30 {
				th := threads[r.IntN(len(threads))]
				who := people[r.IntN(len(people))]
				w.Send("owner", th.id, "@"+who+" ping", http.StatusCreated)
			}
			for _, p := range people {
				a := actor.Actor{ID: pid[p], Kind: actor.KindUser}
				sc, err := w.Authorizer.Scope(context.Background(), a, authorize.ActionMessageRead)
				if err != nil {
					t.Fatal(err)
				}
				read := map[ids.ID]int64{}
				for _, th := range threads {
					if r.IntN(2) == 0 {
						read[th.id] = int64(r.IntN(8))
					}
				}
				got := counts(t, w, messaging.UnreadMentionQuery{PrincipalID: pid[p], VisibleChannelIDs: sc.ChannelIDs, LastReadSeq: read})
				want := reference(t, w, a, read)
				if len(got) != len(want) {
					t.Fatalf("%s: got %v, want %v", p, got, want)
				}
				for th, n := range want {
					if got[th] != n {
						t.Fatalf("%s: thread %s got %d, want %d", p, th, got[th], n)
					}
				}
				for th := range got {
					ok := false
					for _, x := range threads {
						if x.id == th && slices.Contains(sc.ChannelIDs, x.channel) {
							ok = true
						}
					}
					if !ok {
						t.Fatalf("%s: thread %s of an invisible channel", p, th)
					}
				}
			}
		})
	}
}

// reference counts the unread mentions of a naively: every mention row of a, its message checked
// with authorize(message.read) one by one.
func reference(t *testing.T, w *mw.World, a actor.Actor, read map[ids.ID]int64) map[ids.ID]int {
	t.Helper()
	rs, err := w.Owner.Query(context.Background(), `SELECT m.id::text, m.thread_id::text, m.channel_id::text, m.thread_seq, m.author_principal_id::text
		FROM mentions mn JOIN messages m ON m.id = mn.message_id WHERE mn.principal_id = $1 AND m.deleted_at IS NULL`, a.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id, thread, channel, author string
		seq                         int64
	}
	var all []row
	for rs.Next() {
		var x row
		if err := rs.Scan(&x.id, &x.thread, &x.channel, &x.seq, &x.author); err != nil {
			t.Fatal(err)
		}
		all = append(all, x)
	}
	rs.Close()
	out := map[ids.ID]int{}
	ctx := authorize.Filter(context.Background())
	for _, x := range all {
		th := ids.MustParse(x.thread)
		if x.seq <= read[th] {
			continue
		}
		d, err := w.Authorizer.Authorize(ctx, a, authorize.ActionMessageRead, authorize.Resource{
			Kind: authorize.ResourceMessage, ID: ids.MustParse(x.id), ThreadID: th, ChannelID: ids.MustParse(x.channel), OwnerID: ids.MustParse(x.author),
		})
		if err != nil {
			t.Fatal(err)
		}
		if d.Allowed() {
			out[th]++
		}
	}
	return out
}
