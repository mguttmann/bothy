// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// cursorRow is one row of the fake consumer store.
type cursorRow struct {
	acked  int64
	holder ids.ID
}

// fakeConsumerStore is agent_consumer_cursors in memory with the semantics of the queries.
type fakeConsumerStore struct {
	mu   gosync.Mutex
	rows map[agents.ConsumerKey]*cursorRow
}

func newFakeConsumerStore() *fakeConsumerStore {
	return &fakeConsumerStore{rows: map[agents.ConsumerKey]*cursorRow{}}
}

func (f *fakeConsumerStore) TakeConsumer(_ context.Context, key agents.ConsumerKey, holder ids.ID, head int64) (int64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rows[key]; ok {
		r.holder = holder
		return r.acked, false, nil
	}
	f.rows[key] = &cursorRow{acked: head, holder: holder}
	return head, true, nil
}

func (f *fakeConsumerStore) SaveConsumerAck(_ context.Context, key agents.ConsumerKey, holder ids.ID, acked int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[key]
	if !ok || r.holder != holder {
		return false, nil
	}
	r.acked = max(r.acked, acked)
	return true, nil
}

func (f *fakeConsumerStore) RestartConsumer(_ context.Context, key agents.ConsumerKey, holder ids.ID, seq int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[key]
	if !ok || r.holder != holder {
		return false, nil
	}
	r.acked = seq
	return true, nil
}

func (f *fakeConsumerStore) AckConsumer(_ context.Context, key agents.ConsumerKey, seq int64) (int64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[key]
	if !ok {
		return 0, false, nil
	}
	r.acked = max(r.acked, seq)
	return r.acked, true, nil
}

func (f *fakeConsumerStore) GetConsumer(_ context.Context, key agents.ConsumerKey) (int64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[key]
	if !ok {
		return 0, false, nil
	}
	return r.acked, true, nil
}

// selfAuthorizer is agent-self for agent.events.consume: an agent on itself passes, everyone else
// gets forbidden with subject_kind_not_allowed. It counts the checks.
type selfAuthorizer struct {
	mu     gosync.Mutex
	checks int
}

func (z *selfAuthorizer) Authorize(_ context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	z.mu.Lock()
	z.checks++
	z.mu.Unlock()
	act, ok := p.(actor.Actor)
	if a == authorize.ActionAgentEventsConsume && ok && act.Kind == actor.KindAgent && r.Kind == authorize.ResourceAgent && r.ID == act.ID {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "subject_kind_not_allowed"}, nil
}

func (z *selfAuthorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

func (z *selfAuthorizer) Observe(context.Context, authorize.LogEvent) {}

// fakeFeed is the gateway engine as the REST use cases see it.
type fakeFeed struct {
	head  int64
	polls []string
	// seen is the actor of the context of every Poll.
	seen  []actor.Actor
	acked map[agents.ConsumerKey]int64
	page  agents.FeedPage
}

func (f *fakeFeed) Poll(ctx context.Context, consumer string, _ int) (agents.FeedPage, error) {
	f.polls = append(f.polls, consumer)
	a, _ := actor.FromContext(ctx)
	f.seen = append(f.seen, a)
	return f.page, nil
}

func (f *fakeFeed) Acked(_ context.Context, key agents.ConsumerKey, acked int64) {
	if f.acked == nil {
		f.acked = map[agents.ConsumerKey]int64{}
	}
	f.acked[key] = acked
}

func (f *fakeFeed) Head(context.Context) (int64, ids.ID, error) { return f.head, ids.ID{}, nil }

// consumerEnv is the consumer use cases over the fakes, with an agent and a person.
type consumerEnv struct {
	svc   *app.ConsumerService
	store *fakeConsumerStore
	az    *selfAuthorizer
	feed  *fakeFeed
	agent context.Context
	sam   context.Context
	id    ids.ID
}

func newConsumerEnv(t *testing.T) consumerEnv {
	t.Helper()
	store, az, feed := newFakeConsumerStore(), &selfAuthorizer{}, &fakeFeed{head: 100}
	svc, err := app.NewConsumers(store, az)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFeed(feed)
	id := ids.New()
	return consumerEnv{svc: svc, store: store, az: az, feed: feed, id: id,
		agent: actor.WithActor(context.Background(), actor.Actor{ID: id, Kind: actor.KindAgent}),
		sam:   actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})}
}

// wantErrCode checks the public code of err.
func wantErrCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	if e, ok := errs.As(err); !ok || e.Code != code {
		t.Fatalf("error %v, want %s", err, code)
	}
}

// TestTakeCreatesAtHead: the first take creates the cursor at the head; a person is refused and
// creates no row.
func TestTakeCreatesAtHead(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t)
	st, err := e.svc.Take(e.agent, "default", ids.New(), 40)
	if err != nil || !st.Created || st.AckedSeq != 40 || st.Consumer != "default" {
		t.Fatalf("Take = %+v, %v", st, err)
	}
	_, err = e.svc.Take(e.sam, "default", ids.New(), 40)
	wantErrCode(t, err, errs.CodeForbidden)
	if len(e.store.rows) != 1 {
		t.Fatalf("rows %d", len(e.store.rows))
	}
	_, err = e.svc.Take(e.agent, "Bad Name", ids.New(), 40)
	wantErrCode(t, err, errs.CodeValidationFailed)
}

// TestTakeKeepsAckedAndWritesHolder: a later take keeps acked_seq and writes the new holder, so
// the save of the old holder is fenced out.
func TestTakeKeepsAckedAndWritesHolder(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t)
	first, second := ids.New(), ids.New()
	if _, err := e.svc.Take(e.agent, "default", first, 40); err != nil {
		t.Fatal(err)
	}
	key := agents.ConsumerKey{AgentID: e.id, Consumer: "default"}
	if err := e.svc.Save(context.Background(), key, first, 44); err != nil {
		t.Fatal(err)
	}
	st, err := e.svc.Take(e.agent, "default", second, 90)
	if err != nil || st.Created || st.AckedSeq != 44 {
		t.Fatalf("second Take = %+v, %v", st, err)
	}
	if got := e.store.rows[key].holder; got != second {
		t.Fatalf("holder %v", got)
	}
}

// TestSaveFencedReturnsReplaced: a save or restart of a holder that lost the row is
// ErrConsumerReplaced (kind conflict); the stored value stays.
func TestSaveFencedReturnsReplaced(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t)
	first, second := ids.New(), ids.New()
	key := agents.ConsumerKey{AgentID: e.id, Consumer: "default"}
	if _, err := e.svc.Take(e.agent, "default", first, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Take(e.agent, "default", second, 10); err != nil {
		t.Fatal(err)
	}
	err := e.svc.Save(context.Background(), key, first, 20)
	if !errors.Is(err, agents.ErrConsumerReplaced) || errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("Save of the replaced holder = %v", err)
	}
	if err := e.svc.Restart(context.Background(), key, first, 50); !errors.Is(err, agents.ErrConsumerReplaced) {
		t.Fatalf("Restart of the replaced holder = %v", err)
	}
	if e.store.rows[key].acked != 10 {
		t.Fatalf("acked %d", e.store.rows[key].acked)
	}
}

// TestRestPollDelegatesAfterAuthorize: pollAgentEvents authorizes before it reaches the feed; a
// person never reaches it; a resync page answers 410 resync_required with the reason.
func TestRestPollDelegatesAfterAuthorize(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t)
	_, err := e.svc.Poll(e.sam, "default", 0)
	wantErrCode(t, err, errs.CodeForbidden)
	if len(e.feed.polls) != 0 || e.az.checks != 1 {
		t.Fatalf("polls %v checks %d", e.feed.polls, e.az.checks)
	}
	e.feed.page = agents.FeedPage{NextCursor: 7, HeadSeq: 100}
	page, err := e.svc.Poll(e.agent, "worker-1", 3)
	if err != nil || page.NextCursor != 7 || len(e.feed.polls) != 1 || e.feed.polls[0] != "worker-1" {
		t.Fatalf("Poll = %+v, %v, polls %v", page, err, e.feed.polls)
	}
	e.feed.page = agents.FeedPage{Resync: agents.ResyncRetention, HeadSeq: 100}
	_, err = e.svc.Poll(e.agent, "worker-1", 3)
	wantErrCode(t, err, errs.CodeResyncRequired)
	if ee, _ := errs.As(err); ee.Reason != "retention" {
		t.Fatalf("reason %q", ee.Reason)
	}
}

// TestAckEventsUnknownConsumerNotFound: ackAgentEvents of an unknown consumer is not_found, a seq
// above the head or below 0 is validation_failed on /seq, a valid one is stored monotonically and
// reaches the window of the process; State of an unknown consumer is not_found.
func TestAckEventsUnknownConsumerNotFound(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t)
	_, err := e.svc.Ack(e.agent, "default", 5)
	wantErrCode(t, err, errs.CodeNotFound)
	_, err = e.svc.State(e.agent, "default")
	wantErrCode(t, err, errs.CodeNotFound)
	if _, err := e.svc.Take(e.agent, "default", ids.New(), 10); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []int64{-1, 101} {
		_, err := e.svc.Ack(e.agent, "default", bad)
		wantErrCode(t, err, errs.CodeValidationFailed)
	}
	v, err := e.svc.Ack(e.agent, "default", 30)
	if err != nil || v.AckedSeq != 30 || v.HeadSeq != 100 {
		t.Fatalf("Ack = %+v, %v", v, err)
	}
	if v, err = e.svc.Ack(e.agent, "default", 20); err != nil || v.AckedSeq != 30 {
		t.Fatalf("lower Ack = %+v, %v", v, err)
	}
	key := agents.ConsumerKey{AgentID: e.id, Consumer: "default"}
	if e.feed.acked[key] != 30 {
		t.Fatalf("feed saw %v", e.feed.acked)
	}
	if v, err := e.svc.State(e.agent, "default"); err != nil || v.AckedSeq != 30 || v.HeadSeq != 100 {
		t.Fatalf("State = %+v, %v", v, err)
	}
	if err := e.svc.AcceptAck(e.agent, "default", 101, 100); errs.KindOf(err) != errs.KindBadRequest {
		t.Fatalf("AcceptAck above the head = %v", err)
	}
	if err := e.svc.AcceptAck(e.sam, "default", 1, 100); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("AcceptAck of a person = %v", err)
	}
}

// TestRestPollFeedIgnoresActingFor (07 section 8.1, brief M2-14 design note 6): the feed of
// pollAgentEvents reads with the agent's own grants; the Invocation-Token binding of the request
// is checked by the authorization of agent.events.consume and never reaches the feed, so it can
// neither narrow the consumer nor skip its events.
func TestRestPollFeedIgnoresActingFor(t *testing.T) {
	t.Parallel()
	e := newConsumerEnv(t)
	bound := actor.Actor{ID: e.id, Kind: actor.KindAgent, ActingFor: actor.ActingFor{Status: actor.ActingForBound, UserID: ids.New(),
		RunID: ids.New(), ThreadID: ids.New(), JTI: "jti"}}
	if _, err := e.svc.Poll(actor.WithActor(context.Background(), bound), "worker-1", 3); err != nil {
		t.Fatal(err)
	}
	if len(e.feed.seen) != 1 || e.feed.seen[0].ID != e.id || e.feed.seen[0].ActingFor.Present() {
		t.Fatalf("the feed saw %+v", e.feed.seen)
	}
}
