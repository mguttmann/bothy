// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// catchAuthz allows events.catch_up and every receive action on the channels in allowed; Scope
// returns allowed as the visible channels.
type catchAuthz struct {
	allowed map[ids.ID]bool
	checks  int
	filters int
	// memos records the structure memo of every receive check (M1-72).
	memos []*authorize.RequestMemo
}

func (f *catchAuthz) Authorize(ctx context.Context, _ authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if a == authorize.ActionEventsCatchUp {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	f.checks++
	if authorize.IsFilter(ctx) {
		f.filters++
	}
	m, _ := authorize.StructureMemoFrom(ctx)
	f.memos = append(f.memos, m)
	if f.allowed[r.ChannelID] {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}, nil
}

func (f *catchAuthz) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	sc := authorize.Scope{PermissionVersion: 7}
	for id := range f.allowed {
		sc.ChannelIDs = append(sc.ChannelIDs, id)
	}
	return sc, nil
}

// Observe implements authorize.Authorizer; the fake keeps no cache.
func (*catchAuthz) Observe(context.Context, authorize.LogEvent) {}

// catchLog returns the stored events after the cursor up to the head; it applies no visibility
// predicate, so the use case alone decides.
type catchLog struct {
	events []sync.Event
	min    int64
	query  app.VisibleQuery
}

func (l *catchLog) MinRetainedSeq(context.Context) (int64, error) { return l.min, nil }

func (l *catchLog) VisibleEvents(_ context.Context, q app.VisibleQuery) ([]sync.Event, error) {
	l.query = q
	var out []sync.Event
	for _, e := range l.events {
		if e.Seq > q.After && e.Seq <= q.Head && len(out) < q.Limit {
			out = append(out, e)
		}
	}
	return out, nil
}

type catchHead int64

func (h catchHead) EventSeq(context.Context) (int64, error) { return int64(h), nil }

type catchEpoch ids.ID

func (e catchEpoch) Load(context.Context) (instance.Record, error) {
	return instance.Record{SyncEpoch: ids.ID(e)}, nil
}

// newCatchUp returns the use case over the fakes with page size pageSize.
func newCatchUp(t *testing.T, log *catchLog, az authorize.Authorizer, head int64, pageSize int) *app.CatchUpService {
	t.Helper()
	s, err := app.NewCatchUp(app.CatchUpConfig{PageSize: pageSize, MaxEvents: 1000},
		app.CatchUpDeps{Log: log, Head: catchHead(head), Epoch: catchEpoch(ids.New()), Authorizer: az})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// seqsOf returns the seq of every envelope of a page.
func seqsOf(t *testing.T, page sync.PageResult) []int64 {
	t.Helper()
	out := make([]int64, 0, len(page.Events))
	for _, raw := range page.Events {
		var env struct {
			Seq int64 `json:"seq"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		out = append(out, env.Seq)
	}
	return out
}

// TestPersonalEventsOnlyToAudience: a personal event goes only to its audience, without asking
// authorize; an event of a readable channel passes authorize in filter mode, one of an unreadable
// channel does not, and the cursor passes over everything checked.
func TestPersonalEventsOnlyToAudience(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	support, marketing := ids.New(), ids.New()
	ev := func(seq int64, channel, audience ids.ID) sync.Event {
		// A file event: its descriptor carries the channel the fake decides on.
		e := sync.Event{Seq: seq, ID: ids.New(), Type: "file.updated", SchemaVersion: 1, ChannelID: channel,
			ResourceKind: "file", ResourceID: ids.New(), ReceiveAction: "file.read", Payload: json.RawMessage(`{}`)}
		if !audience.IsZero() {
			e.Type, e.ResourceKind, e.ReceiveAction, e.AudiencePrincipalID = "read_state.updated", "thread", "self", audience
			e.ThreadID = ids.New()
			e.ResourceID = e.ThreadID
		}
		return e
	}
	log := &catchLog{min: 1, events: []sync.Event{
		ev(1, support, ids.ID{}), ev(2, support, sam), ev(3, support, alex), ev(4, marketing, ids.ID{}), ev(5, support, ids.ID{}),
	}}
	az := &catchAuthz{allowed: map[ids.ID]bool{support: true}}
	s := newCatchUp(t, log, az, 5, 10)
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: alex, Kind: actor.KindUser})
	page, err := s.Page(ctx, sync.PageInput{After: 0, Head: 5})
	if err != nil {
		t.Fatal(err)
	}
	got := seqsOf(t, page)
	if want := []int64{1, 3, 5}; len(got) != len(want) || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("delivered %v, want %v", got, want)
	}
	if page.NextCursor != 5 {
		t.Fatalf("next cursor %d", page.NextCursor)
	}
	if az.checks != 3 || az.filters != 3 {
		t.Fatalf("authorize ran %d times (%d in filter mode), want 3 for the non-personal events", az.checks, az.filters)
	}
	if log.query.Principal != alex || len(log.query.ChannelIDs) != 1 {
		t.Fatalf("query %+v", log.query)
	}
	// Sam receives its own read state and none of Alex.
	ctx = actor.WithActor(context.Background(), actor.Actor{ID: sam, Kind: actor.KindUser})
	page, err = s.Page(ctx, sync.PageInput{After: 0, Head: 5})
	if err != nil {
		t.Fatal(err)
	}
	if got := seqsOf(t, page); len(got) != 3 || got[1] != 2 {
		t.Fatalf("Sam got %v", got)
	}
}

// TestCatchUpCheck: Check maps the domain decision and reads the minimum of an empty log as head
// + 1.
func TestCatchUpCheck(t *testing.T) {
	t.Parallel()
	log := &catchLog{min: 0}
	s := newCatchUp(t, log, &catchAuthz{}, 40, 10)
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
	res, err := s.Check(ctx, sync.CheckInput{After: 40})
	if err != nil || res.Reason != "" || res.MinRetainedSeq != 41 || res.HeadSeq != 40 {
		t.Fatalf("at head: %+v %v", res, err)
	}
	if res, err = s.Check(ctx, sync.CheckInput{After: 39}); err != nil || res.Reason != sync.ResyncRetention {
		t.Fatalf("behind an emptied log: %+v %v", res, err)
	}
	other := ids.New()
	if res, err = s.Check(ctx, sync.CheckInput{After: 40, Epoch: &other}); err != nil || res.Reason != sync.ResyncEpochChanged {
		t.Fatalf("other epoch: %+v %v", res, err)
	}
}

// TestCatchUpPageLimit: the limit defaults to and is capped at the page size; a full page ends at
// its last row.
func TestCatchUpPageLimit(t *testing.T) {
	t.Parallel()
	ch := ids.New()
	log := &catchLog{min: 1}
	for i := int64(1); i <= 10; i++ {
		log.events = append(log.events, sync.Event{Seq: i, ID: ids.New(), Type: "message.created", SchemaVersion: 1, ChannelID: ch,
			ResourceKind: "message", ResourceID: ids.New(), ReceiveAction: "message.read"})
	}
	s := newCatchUp(t, log, &catchAuthz{allowed: map[ids.ID]bool{}}, 10, 4)
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
	for _, limit := range []int{0, 100} {
		page, err := s.Page(ctx, sync.PageInput{After: 0, Head: 10, Limit: limit})
		if err != nil || log.query.Limit != 4 || page.NextCursor != 4 || len(page.Events) != 0 {
			t.Fatalf("limit %d: %+v %v (query limit %d)", limit, page, err, log.query.Limit)
		}
	}
}

// TestCatchUpPageOneStructureScope: the receive checks of one page share one structure memo and
// the next page gets a new one, so a page reads the thread and channel of its events once each
// instead of once per event (perf.catchup_page_p99, M1-72).
func TestCatchUpPageOneStructureScope(t *testing.T) {
	t.Parallel()
	ch := ids.New()
	log := &catchLog{min: 1}
	for i := int64(1); i <= 6; i++ {
		log.events = append(log.events, sync.Event{Seq: i, ID: ids.New(), Type: "message.created", SchemaVersion: 1, ChannelID: ch,
			ResourceKind: "message", ResourceID: ids.New(), ReceiveAction: "message.read"})
	}
	az := &catchAuthz{allowed: map[ids.ID]bool{ch: true}}
	s := newCatchUp(t, log, az, 6, 3)
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
	for _, after := range []int64{0, 3} {
		if _, err := s.Page(ctx, sync.PageInput{After: after, Head: 6, Limit: 3}); err != nil {
			t.Fatal(err)
		}
	}
	if len(az.memos) != 6 || az.memos[0] == nil {
		t.Fatalf("%d checks, first memo %p; want 6 checks in a structure scope", len(az.memos), az.memos[0])
	}
	if az.memos[0] != az.memos[2] || az.memos[3] != az.memos[5] || az.memos[0] == az.memos[3] {
		t.Error("want one structure memo per page")
	}
}

// TestCatchUp_ActingForWithholdsOwnAccessChanged (07 guard rule SR9, brief M2-14 requirement 14):
// the own access.changed of an agent names the targets of its grants; a request bound to a run
// of a person does not get it, so the catch-up names no channel the person may not read; without
// the binding, and for the agent consumer, the event is delivered.
// Every personal self event is judged by the agent's own rights (access.changed and
// access.expiring name the targets of the agent's grants, sidebar.updated and read_state.updated
// name its spaces and threads), so a catch-up bound to a run leaves out all of them.
func TestCatchUp_ActingForWithholdsOwnAccessChanged(t *testing.T) {
	t.Parallel()
	agent := ids.New()
	var events []sync.Event
	for i, typ := range []string{"access.changed", "access.expiring", "sidebar.updated", "read_state.updated"} {
		events = append(events, sync.Event{Seq: int64(i + 1), ID: ids.New(), Type: typ, SchemaVersion: 1, ResourceKind: "principal",
			ResourceID: agent, ReceiveAction: "self", AudiencePrincipalID: agent, Payload: json.RawMessage(`{}`)})
	}
	head := int64(len(events))
	log := &catchLog{min: 1, events: events}
	s := newCatchUp(t, log, &catchAuthz{allowed: map[ids.ID]bool{}}, 1, 10)
	own := actor.Actor{ID: agent, Kind: actor.KindAgent}
	bound := own
	bound.ActingFor = actor.ActingFor{Status: actor.ActingForBound, UserID: ids.New(), RunID: ids.New(), ThreadID: ids.New(), JTI: "jti"}
	for _, c := range []struct {
		name     string
		a        actor.Actor
		consumer bool
		want     int
	}{{"without the header", own, false, len(events)}, {"bound", bound, false, 0}, {"bound agent consumer", bound, true, len(events)}} {
		page, err := s.PageEvents(actor.WithActor(context.Background(), c.a), sync.PageInput{After: 0, Head: head, AgentConsumer: c.consumer})
		if err != nil || len(page.Events) != c.want || page.NextCursor != head {
			t.Errorf("%s: %d events, cursor %d, %v; want %d", c.name, len(page.Events), page.NextCursor, err, c.want)
		}
	}
}
