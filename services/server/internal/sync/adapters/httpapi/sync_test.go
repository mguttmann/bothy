// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// noSubscriber registers no test subscriber of MessagePosted.
func noSubscriber(*txn.Registry, *mw.Recorder) error { return nil }

// pageSize is sync.catchup_page_size of the catch-up worlds; partitionSize their
// sync.event_log_partition_size.
const (
	pageSize      = 10
	partitionSize = 20
)

// syncEnv is the world of the catch-up tests (brief M1-63): Alex reads Support, Sam reads Support
// and Marketing, Robin reads nothing; a text channel with its main thread in each space.
type syncEnv struct {
	*mw.World
	t                          *testing.T
	support, marketing         ids.ID
	supportChan, marketingChan ids.ID
	supportMain, marketingMain ids.ID
	alex, sam, robin           ids.ID
}

// newSyncEnv builds the world with sync.catchup_max_events maxEvents.
func newSyncEnv(t *testing.T, maxEvents int64) *syncEnv {
	t.Helper()
	w := mw.NewSync(t, noSubscriber, mw.SyncOptions{CatchUp: syncapp.CatchUpConfig{PageSize: pageSize, MaxEvents: maxEvents},
		PartitionSize: partitionSize})
	e := &syncEnv{World: w, t: t}
	e.support, e.marketing = w.Space("Support"), w.Space("Marketing")
	e.supportChan, e.marketingChan = w.Channel(e.support, "text", "tickets"), w.Channel(e.marketing, "text", "campaigns")
	e.supportMain, e.marketingMain = w.MainThread(e.supportChan), w.MainThread(e.marketingChan)
	e.alex, e.sam, e.robin = w.Person("alex", false), w.Person("sam", false), w.Person("robin", false)
	w.Grant("user", e.alex, e.support, "read")
	w.Grant("user", e.sam, e.support, "read")
	w.Grant("user", e.sam, e.marketing, "read")
	return e
}

// envelope is the part of an event the tests read.
type envelope struct {
	Seq      int64          `json:"seq"`
	Type     string         `json:"type"`
	Audience string         `json:"audience"`
	Redacted bool           `json:"redacted"`
	Scope    map[string]any `json:"scope"`
	Payload  map[string]any `json:"payload"`
}

// eventPage is the answer of listEvents.
type eventPage struct {
	Events     []envelope `json:"events"`
	NextCursor int64      `json:"next_cursor"`
	HeadSeq    int64      `json:"head_seq"`
	SyncEpoch  string     `json:"sync_epoch"`
}

// eventsPath is the path of listEvents after the cursor with the optional query.
func eventsPath(after int64, query string) string {
	return "/api/v1/events?after=" + strconv.FormatInt(after, 10) + query
}

// page reads one page of listEvents as token name and checks 200; the harness validated it
// against openapi.yaml.
func (e *syncEnv) page(name string, after int64, query string) eventPage {
	e.t.Helper()
	r := e.Do(name, http.MethodGet, eventsPath(after, query), "")
	e.Want(r, http.StatusOK, "")
	var p eventPage
	if err := json.Unmarshal(r.Body, &p); err != nil {
		e.t.Fatal(err)
	}
	return p
}

// all pages listEvents as token name from after with limit until next_cursor reaches head_seq.
func (e *syncEnv) all(name string, after int64, limit int) ([]envelope, eventPage) {
	e.t.Helper()
	var out []envelope
	for i := 0; ; i++ {
		p := e.page(name, after, "&limit="+strconv.Itoa(limit))
		out = append(out, p.Events...)
		if p.NextCursor <= after && p.NextCursor != p.HeadSeq {
			e.t.Fatalf("cursor did not advance: %d after %d", p.NextCursor, after)
		}
		after = p.NextCursor
		if after == p.HeadSeq {
			return out, p
		}
		if i > 1000 {
			e.t.Fatal("catch-up does not end")
		}
	}
}

// head returns the committed counter event_seq.
func (e *syncEnv) head() int64 {
	return int64(e.Count(`SELECT value::int FROM sequence_counters WHERE name = 'event_seq'`))
}

// channelOf returns scope.channel_id of an envelope, "" without one.
func channelOf(ev envelope) string {
	s, _ := ev.Scope["channel_id"].(string)
	return s
}

// TestListEventsSkipsInvisible: Support and Marketing events interleaved; Alex gets only the
// Support ones, paging with a small limit reaches head_seq, and the cursor passes over the
// Marketing ranges.
func TestListEventsSkipsInvisible(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	for i := range 6 {
		e.Send("owner", e.supportMain, "support "+strconv.Itoa(i), http.StatusCreated)
		e.Send("owner", e.marketingMain, "marketing "+strconv.Itoa(i), http.StatusCreated)
	}
	events, last := e.all("alex", 0, 3)
	if last.HeadSeq != e.head() || last.NextCursor != last.HeadSeq {
		t.Fatalf("last page %+v, head %d", last, e.head())
	}
	support := 0
	for _, ev := range events {
		if channelOf(ev) == e.marketingChan.String() || ev.Scope["space_id"] == e.marketing.String() {
			t.Fatalf("Alex got the Marketing event %+v", ev)
		}
		if ev.Type == "message.created" && channelOf(ev) == e.supportChan.String() {
			support++
		}
	}
	if support != 6 {
		t.Fatalf("Alex got %d Support messages, want 6", support)
	}
	// Robin reads nothing: no channel event, and the cursor still reaches the head.
	events, last = e.all("robin", 0, 5)
	for _, ev := range events {
		if channelOf(ev) != "" {
			t.Fatalf("Robin got %+v", ev)
		}
	}
	if last.NextCursor != e.head() {
		t.Fatalf("Robin's cursor %d", last.NextCursor)
	}
}

// TestListEventsCursorIsLastChecked: a full page ends at the seq of its last row.
func TestListEventsCursorIsLastChecked(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	before := e.head()
	for i := range 3 {
		e.Send("owner", e.supportMain, "m"+strconv.Itoa(i), http.StatusCreated)
	}
	p := e.page("alex", before, "&limit=2")
	if len(p.Events) != 2 || p.NextCursor != p.Events[1].Seq || p.NextCursor >= p.HeadSeq {
		t.Fatalf("full page %+v", p)
	}
	p = e.page("alex", p.NextCursor, "&limit=2")
	if len(p.Events) != 1 || p.NextCursor != p.HeadSeq {
		t.Fatalf("last page %+v", p)
	}
	// An empty page still has next_cursor (the head).
	if p = e.page("alex", p.HeadSeq, ""); len(p.Events) != 0 || p.NextCursor != p.HeadSeq {
		t.Fatalf("empty page %+v", p)
	}
}

// TestListEventsPersonalAudience: Sam's read_state.updated in Support is not delivered to Alex;
// Sam receives it.
func TestListEventsPersonalAudience(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	e.Send("owner", e.supportMain, "hello", http.StatusCreated)
	e.Want(e.Do("sam", http.MethodPut, readStatePath(e.supportMain), `{"last_read_seq":1}`), http.StatusOK, "")
	count := func(name string) int {
		events, _ := e.all(name, 0, pageSize)
		n := 0
		for _, ev := range events {
			if ev.Type == "read_state.updated" {
				if ev.Audience != "self" {
					t.Fatalf("personal event without audience: %+v", ev)
				}
				n++
			}
		}
		return n
	}
	if n := count("alex"); n != 0 {
		t.Fatalf("Alex got %d read states of Sam", n)
	}
	if n := count("sam"); n != 1 {
		t.Fatalf("Sam got %d own read states", n)
	}
}

// resync reads listEvents as Alex and checks 410 resync_required with reason.
func (e *syncEnv) resync(t *testing.T, after int64, query, reason string) {
	t.Helper()
	p := e.Want(e.Do("alex", http.MethodGet, eventsPath(after, query), ""), http.StatusGone, "resync_required")
	if p["reason"] != reason {
		t.Fatalf("reason %v, want %s: %v", p["reason"], reason, p)
	}
}

// TestListEventsResync: retention (a partition removed by the maintenance of M1-16 with the
// world's clock), too_far_behind (a small sync.catchup_max_events), cursor_ahead and
// epoch_changed each give 410 with their reason.
func TestListEventsResync(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	for e.head() <= 2*partitionSize {
		e.Send("owner", e.supportMain, "fill", http.StatusCreated)
	}
	head := e.head()
	var epoch string
	if err := e.Owner.QueryRow(context.Background(), `SELECT sync_epoch::text FROM instance`).Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	e.page("alex", head, "&epoch="+epoch)
	e.resync(t, head+1, "", "cursor_ahead")
	e.resync(t, head+1, "&epoch="+epoch, "cursor_ahead")
	e.resync(t, 0, "&epoch="+ids.New().String(), "epoch_changed")
	e.resync(t, head+5, "&epoch="+ids.New().String(), "epoch_changed")
	// The first partition holds events older than sync.event_retention: the maintenance drops it.
	e.SQL(`UPDATE event_log SET occurred_at = now() - interval '90 days' WHERE seq < $1`, partitionSize+1)
	if _, err := e.Maintenance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := e.Count(`SELECT min(seq)::int FROM event_log`); n != partitionSize+1 {
		t.Fatalf("min(seq) %d after the maintenance", n)
	}
	e.resync(t, 0, "&epoch="+epoch, "retention")
	e.resync(t, partitionSize-1, "", "retention")
	e.page("alex", partitionSize, "&epoch="+epoch)

	far := newSyncEnv(t, 5)
	far.Send("owner", far.supportMain, "one", http.StatusCreated)
	far.resync(t, 0, "", "too_far_behind")
	far.page("alex", far.head()-5, "")
}

// TestListEventsNewChannelInVisibleSpace: a channel created in Support after Alex's rights
// snapshot was cached (no new rights version): its events are delivered.
func TestListEventsNewChannelInVisibleSpace(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	before := e.head()
	e.page("alex", before, "")
	fresh := e.Channel(e.support, "text", "fresh")
	e.Send("owner", e.MainThread(fresh), "in the new channel", http.StatusCreated)
	events, _ := e.all("alex", before, pageSize)
	got := map[string]bool{}
	for _, ev := range events {
		if channelOf(ev) == fresh.String() {
			got[ev.Type] = true
		}
	}
	if !got["channel.created"] || !got["message.created"] {
		t.Fatalf("events of the new channel: %v", got)
	}
}

// TestListEventsAfterRevoke: after Alex's grant on Support is revoked, no Support event is
// delivered, also none from before the revoke that Alex had not fetched yet.
func TestListEventsAfterRevoke(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	e.Send("owner", e.supportMain, "before the revoke", http.StatusCreated)
	seen := false
	events, _ := e.all("alex", 0, pageSize)
	for _, ev := range events {
		seen = seen || (ev.Type == "message.created" && channelOf(ev) == e.supportChan.String())
	}
	if !seen {
		t.Fatal("Alex got no Support message before the revoke")
	}
	e.SQL(`UPDATE grants SET revoked_at = now() WHERE subject_principal_id = $1`, e.alex.String())
	e.SQL(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	events, _ = e.all("alex", 0, pageSize)
	for _, ev := range events {
		if channelOf(ev) != "" || ev.Scope["space_id"] != nil {
			t.Fatalf("Alex got %+v after the revoke", ev)
		}
	}
}

// TestListEventsRedacted: the event of a deleted message comes with redacted true and the
// redacted payload.
func TestListEventsRedacted(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	before := e.head()
	m := e.Send("owner", e.supportMain, "secret words", http.StatusCreated)
	e.Want(e.Do("owner", http.MethodDelete, mw.MessagePath(ids.MustParse(m["id"].(string))), ""), http.StatusNoContent, "")
	events, _ := e.all("alex", before, pageSize)
	found := false
	for _, ev := range events {
		if ev.Type != "message.created" {
			continue
		}
		found = true
		raw, _ := json.Marshal(ev.Payload)
		if !ev.Redacted || strings.Contains(string(raw), "secret words") {
			t.Fatalf("created event of the deleted message: redacted %v, payload %s", ev.Redacted, raw)
		}
	}
	if !found {
		t.Fatal("no message.created")
	}
}

// TestListEventsLimitCapped: a limit above sync.catchup_page_size returns at most that many
// events.
func TestListEventsLimitCapped(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	for e.head() <= 2*pageSize {
		e.Send("owner", e.supportMain, "fill", http.StatusCreated)
	}
	p := e.page("owner", 0, "&limit=1000")
	if len(p.Events) != pageSize || p.NextCursor != p.Events[pageSize-1].Seq {
		t.Fatalf("%d events, cursor %d", len(p.Events), p.NextCursor)
	}
}

// TestListEventsAppNeedsScope: an app without events:subscribe gets the refusal authorize gives
// on the instance resource (403 scope_missing); with the scope it gets a page.
func TestListEventsAppNeedsScope(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	e.App("tracker", "messages:read")
	e.App("watcher", "events:subscribe")
	e.Want(e.Do("tracker", http.MethodGet, eventsPath(0, ""), ""), http.StatusForbidden, "scope_missing")
	e.page("watcher", 0, "")
}

// TestListEventsPinOfAMessage: pin.added names a message but is received with thread.read; the
// page decides it on the thread instead of failing (review finding of M1-63), so Alex gets it and
// Robin does not.
func TestListEventsPinOfAMessage(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	before := e.head()
	m := e.Send("owner", e.supportMain, "pin me", http.StatusCreated)
	e.Want(e.Do("owner", http.MethodPut, mw.MessagePath(ids.MustParse(m["id"].(string)))+"/pin", ""), http.StatusNoContent, "")
	count := func(name string) int {
		events, _ := e.all(name, before, pageSize)
		n := 0
		for _, ev := range events {
			if ev.Type == "pin.added" {
				n++
			}
		}
		return n
	}
	if a, r := count("alex"), count("robin"); a != 1 || r != 0 {
		t.Fatalf("pin.added: Alex %d, Robin %d", a, r)
	}
}
