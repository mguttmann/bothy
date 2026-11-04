// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/search"
	"github.com/OWNER/bothy/services/server/internal/search/app"
)

// fakeSearch answers Query per result type with fixed hits and records the requests.
type fakeSearch struct {
	mu       sync.Mutex
	threads  map[string][]search.ResultHit // by query text
	messages map[string][]search.ResultHit
	asked    []search.Request
	err      error
}

func (f *fakeSearch) Query(_ context.Context, _ actor.Actor, r search.Request) (search.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, r)
	if f.err != nil {
		return search.Result{}, f.err
	}
	src := f.messages
	if r.Type == search.TypeThreads {
		src = f.threads
	}
	hits := src[r.Q]
	if r.Limit > 0 && len(hits) > r.Limit {
		hits = hits[:r.Limit]
	}
	return search.Result{Hits: slices.Clone(hits)}, nil
}

func (f *fakeSearch) ThreadTitles(context.Context, actor.Actor, search.PrefixRequest) ([]searchkit.TitleHit, error) {
	return nil, errors.New("not used")
}

// assistAuthorizer allows search.assist for people only, search.assist.respond for the agent the
// resource names, and the read actions in filter mode unless the principal's deny set holds the
// resource id.
type assistAuthorizer struct {
	mu    sync.Mutex
	deny  map[ids.ID]map[ids.ID]bool // principal -> resource ids
	reads []string
}

func (a *assistAuthorizer) Authorize(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	allow := authorize.Decision{Outcome: authorize.OutcomeAllow}
	if act == authorize.ActionSearchAssist {
		if p.PrincipalKind() != string(actor.KindUser) {
			return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "subject_kind_not_allowed"}, nil
		}
		return allow, nil
	}
	if act == authorize.ActionSearchAssistRespond {
		if p.PrincipalID() != r.ID {
			// The Authorizer hides another agent like an unknown resource (ADR 0017).
			return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "not_self"}, nil
		}
		return allow, nil
	}
	if act == authorize.ActionThreadRead || act == authorize.ActionMessageRead {
		if !authorize.IsFilter(ctx) {
			return authorize.Decision{}, errors.New("the agent filter outside filter mode")
		}
		a.reads = append(a.reads, string(act)+" "+p.PrincipalID().String())
		if a.deny[p.PrincipalID()][r.ID] {
			return authorize.Decision{Outcome: authorize.OutcomeNotFound}, nil
		}
		return allow, nil
	}
	return authorize.Decision{}, errors.New("unexpected action " + string(act))
}

func (a *assistAuthorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, errors.New("not used")
}

func (a *assistAuthorizer) Observe(context.Context, authorize.LogEvent) {}

// fakeRelay records the events and reports sent unless down; answer, when set, runs after Ask.
type fakeRelay struct {
	mu     sync.Mutex
	down   bool
	events []eventlog.EphemeralEvent
	agents []ids.ID
	answer func(ev eventlog.EphemeralEvent)
}

func (r *fakeRelay) AskAssistant(_ context.Context, agent ids.ID, ev eventlog.EphemeralEvent) (bool, error) {
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.agents = append(r.agents, agent)
	answer, down := r.answer, r.down
	r.mu.Unlock()
	if down {
		return false, nil
	}
	if answer != nil {
		go answer(ev)
	}
	return true, nil
}

// manualTimer is a timer the test fires.
type manualTimer struct {
	fire    chan time.Time
	started chan time.Duration
}

func newManualTimer() *manualTimer {
	return &manualTimer{fire: make(chan time.Time, 1), started: make(chan time.Duration, 1)}
}

func (m *manualTimer) after(d time.Duration) (<-chan time.Time, func()) {
	m.started <- d
	return m.fire, func() {}
}

// assistEnv is one assistant with its fakes.
type assistEnv struct {
	as      *app.Assistant
	search  *fakeSearch
	az      *assistAuthorizer
	relay   *fakeRelay
	timer   *manualTimer
	alex    actor.Actor
	helper  actor.Actor
	general ids.ID
	private ids.ID
}

func assistConfig(agent ids.ID) app.AssistConfig {
	c := app.AssistConfig{
		Timeout: 10 * time.Second, MaxCandidates: 10, MaxHits: 8, MaxTerms: 6, MaxTurns: 3, ExcerptMaxChars: 120,
		ReasonMaxChars: 200, QueryMaxChars: 256, PayloadMax: 65536,
	}
	if !agent.IsZero() {
		c.AgentID = agent.String()
	}
	return c
}

func newAssistEnv(t *testing.T, configured bool) *assistEnv {
	t.Helper()
	e := &assistEnv{
		search: &fakeSearch{threads: map[string][]search.ResultHit{}, messages: map[string][]search.ResultHit{}},
		az:     &assistAuthorizer{deny: map[ids.ID]map[ids.ID]bool{}}, relay: &fakeRelay{}, timer: newManualTimer(),
		alex: actor.Actor{ID: ids.New(), Kind: actor.KindUser}, helper: actor.Actor{ID: ids.New(), Kind: actor.KindAgent},
		general: ids.New(), private: ids.New(),
	}
	agent := ids.ID{}
	if configured {
		agent = e.helper.ID
	}
	as, err := app.NewAssistant(assistConfig(agent), app.AssistDeps{
		Search: e.search, Authorizer: e.az, Clock: clock.NewFake(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)),
		Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := as.BindRelay(e.relay, e.timer.after); err != nil {
		t.Fatal(err)
	}
	e.as = as
	return e
}

// threadHit is a hit of the thread search; messageHit one of the message search.
func threadHit(thread, channel ids.ID, title string, at time.Time) search.ResultHit {
	return search.ResultHit{Kind: "thread", ID: thread, ThreadID: thread, ChannelID: channel, ThreadTitle: title, ChannelName: "larkspur",
		SpaceName: "Projects", CreatedAt: at, Segments: []searchkit.Segment{{Text: title, Match: true}}}
}

func messageHit(thread, channel ids.ID, title, text string, at time.Time) search.ResultHit {
	return search.ResultHit{Kind: "message", ID: ids.New(), ThreadID: thread, ChannelID: channel, ThreadTitle: title, ChannelName: "larkspur",
		SpaceName: "Projects", CreatedAt: at, Segments: []searchkit.Segment{{Text: text, Match: false}}}
}

// raw returns the encoded payload of ev.
func raw(ev eventlog.EphemeralEvent) []byte {
	b, _ := ev.Payload.(json.RawMessage)
	return b
}

var day = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func TestAssist_WithoutAssistantAnswersFusedFullText(t *testing.T) {
	t.Parallel()
	e := newAssistEnv(t, false)
	takeover, other := ids.New(), ids.New()
	q := "takeover or larkspur or hermes"
	e.search.threads[q] = []search.ResultHit{threadHit(takeover, e.general, "Takeover of Larkspur", day)}
	e.search.messages[q] = []search.ResultHit{
		messageHit(other, e.general, "Release", "larkspur release", day.Add(time.Hour)),
		messageHit(takeover, e.general, "Takeover of Larkspur", "Hermes takes over", day),
	}
	res, err := e.as.Assist(context.Background(), e.alex, search.AssistRequest{Q: "I am looking for the takeover of Larkspur on Hermes"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Mode != search.ModeFulltext || res.FallbackReason != search.FallbackNotConfigured || res.Cost != nil {
		t.Fatalf("result %+v, want full text without an assistant", res)
	}
	if !slices.Equal(res.Terms, []string{"takeover", "larkspur", "hermes"}) {
		t.Fatalf("terms %q", res.Terms)
	}
	if len(res.Hits) != 2 || res.Hits[0].ThreadID != takeover || res.Hits[1].ThreadID != other {
		t.Fatalf("hits %+v, want the takeover thread first", res.Hits)
	}
	first := res.Hits[0]
	if first.Source != search.SourceFulltext || first.Reason != "" || first.MessageID.IsZero() || first.ThreadTitle != "Takeover of Larkspur" {
		t.Fatalf("first hit %+v, want the matching message as jump target", first)
	}
	if len(e.relay.events) != 0 {
		t.Fatal("asked an assistant that is not configured")
	}
	for _, r := range e.search.asked {
		if r.Q != q || r.Sort != searchkit.SortRelevance || r.Limit != 10 {
			t.Fatalf("search request %+v", r)
		}
	}
}

func TestAssist_OnlyPeopleMayAsk(t *testing.T) {
	t.Parallel()
	e := newAssistEnv(t, true)
	_, err := e.as.Assist(context.Background(), e.helper, search.AssistRequest{Q: "takeover"})
	if errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("err = %v, want 403", err)
	}
	if len(e.search.asked) != 0 {
		t.Fatal("an agent's question ran a search")
	}
}

func TestAssist_ValidatesTheQuestion(t *testing.T) {
	t.Parallel()
	e := newAssistEnv(t, false)
	ctx := context.Background()
	for _, tc := range []struct {
		r             search.AssistRequest
		code, pointer string
	}{
		{search.AssistRequest{Q: strings.Repeat("x", 257)}, "too_long", "/q"},
		{search.AssistRequest{Q: "the of and"}, "empty", "/q"},
		{search.AssistRequest{Q: "larkspur", History: make([]search.AssistTurn, 4)}, "too_many_items", "/history"},
	} {
		_, err := e.as.Assist(ctx, e.alex, tc.r)
		code, pointer := field(t, err)
		if code != tc.code || pointer != tc.pointer {
			t.Fatalf("%q: %s %s, want %s %s", tc.r.Q, code, pointer, tc.code, tc.pointer)
		}
	}
}

// TestAssist_TheAssistantSeesOnlyWhatBothMayRead is the rights test of the ticket: a candidate
// the person may read but the assistant agent may not never reaches the payload, a pick of a
// reference that was not sent is refused, and the person still gets that candidate as full text.
func TestAssist_TheAssistantSeesOnlyWhatBothMayRead(t *testing.T) {
	t.Parallel()
	e := newAssistEnv(t, true)
	shared, secret := ids.New(), ids.New()
	q := "takeover"
	e.search.threads[q] = []search.ResultHit{threadHit(secret, e.private, "Secret takeover plan", day.Add(time.Hour))}
	e.search.messages[q] = []search.ResultHit{messageHit(shared, e.general, "Takeover of Larkspur", "the takeover", day)}
	e.az.deny[e.helper.ID] = map[ids.ID]bool{secret: true}
	answered := make(chan error, 2)
	e.relay.answer = func(ev eventlog.EphemeralEvent) {
		var p struct {
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal(raw(ev), &p)
		req := ids.MustParse(p.RequestID)
		// c2 was never sent: the agent cannot name the candidate it may not read.
		answered <- e.as.AnswerAssist(context.Background(), e.helper, search.AssistAnswer{RequestID: req,
			Picks: []search.AssistPick{{Ref: "c2", Reason: "Guess."}}})
		answered <- e.as.AnswerAssist(context.Background(), e.helper, search.AssistAnswer{RequestID: req,
			Picks: []search.AssistPick{{Ref: "c1", Reason: "The move of the steering session."}}})
	}
	res, err := e.as.Assist(context.Background(), e.alex, search.AssistRequest{Q: "takeover"})
	if err != nil {
		t.Fatal(err)
	}
	if code, pointer := field(t, <-answered); code != "unknown_ref" || pointer != "/picks/0/ref" {
		t.Fatalf("pick of an unsent reference: %s %s", code, pointer)
	}
	if err := <-answered; err != nil {
		t.Fatalf("valid answer: %v", err)
	}
	if len(e.relay.events) != 1 || e.relay.agents[0] != e.helper.ID {
		t.Fatalf("events %d to %v", len(e.relay.events), e.relay.agents)
	}
	ev := e.relay.events[0]
	if ev.Type != string(authorize.EventSearchAssistRequested) || ev.Audience != e.helper.ID || ev.Resource.ID != e.helper.ID {
		t.Fatalf("event %+v", ev)
	}
	payload := string(raw(ev))
	for _, leak := range []string{"Secret", secret.String(), shared.String(), e.private.String()} {
		if strings.Contains(payload, leak) {
			t.Fatalf("payload leaks %q: %s", leak, payload)
		}
	}
	if !strings.Contains(payload, `"ref":"c1"`) || strings.Contains(payload, `"ref":"c2"`) {
		t.Fatalf("payload candidates: %s", payload)
	}
	if res.Mode != search.ModeAssisted || len(res.Hits) != 2 {
		t.Fatalf("result %+v", res)
	}
	if h := res.Hits[0]; h.ThreadID != shared || h.Source != search.SourceAssistant || h.Reason != "The move of the steering session." {
		t.Fatalf("pick %+v", h)
	}
	if h := res.Hits[1]; h.ThreadID != secret || h.Source != search.SourceFulltext || h.Reason != "" {
		t.Fatalf("the person's own hit %+v, want it as full text", h)
	}
}

func TestAssist_TermsOfTheAssistantRunASecondPass(t *testing.T) {
	t.Parallel()
	e := newAssistEnv(t, true)
	move, takeover := ids.New(), ids.New()
	e.search.messages["umzug or discord"] = []search.ResultHit{messageHit(move, e.general, "Discord bots", "discord bot", day)}
	e.search.threads["migration or takeover"] = []search.ResultHit{threadHit(takeover, e.general, "Takeover of Larkspur", day)}
	amount := int64(1500)
	e.relay.answer = func(ev eventlog.EphemeralEvent) {
		var p struct {
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal(raw(ev), &p)
		_ = e.as.AnswerAssist(context.Background(), e.helper, search.AssistAnswer{RequestID: ids.MustParse(p.RequestID),
			Terms: []string{"Migration", "takeover", "discord"},
			Costs: []search.AssistCostReport{{Provider: "anthropic", Model: "m", Currency: "USD", AmountMicros: &amount}}})
	}
	res, err := e.as.Assist(context.Background(), e.alex, search.AssistRequest{Q: "Umzug von Discord"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Mode != search.ModeAssisted || !slices.Equal(res.Terms, []string{"umzug", "discord", "migration", "takeover"}) {
		t.Fatalf("result %+v", res)
	}
	if len(res.Hits) != 2 || res.Hits[0].ThreadID != move || res.Hits[1].ThreadID != takeover || res.Hits[1].Source != search.SourceFulltext {
		t.Fatalf("hits %+v", res.Hits)
	}
	if res.Cost == nil || res.Cost.AmountMicros != 1500 || res.Cost.Currency != "USD" {
		t.Fatalf("cost %+v", res.Cost)
	}
}

func TestAssist_NoCandidatesStillAsksForTerms(t *testing.T) {
	t.Parallel()
	e := newAssistEnv(t, true)
	takeover := ids.New()
	e.search.threads["takeover"] = []search.ResultHit{threadHit(takeover, e.general, "Takeover of Larkspur", day)}
	e.relay.answer = func(ev eventlog.EphemeralEvent) {
		var p struct {
			RequestID  string `json:"request_id"`
			Candidates []any  `json:"candidates"`
		}
		_ = json.Unmarshal(raw(ev), &p)
		if p.Candidates == nil || len(p.Candidates) != 0 {
			t.Errorf("candidates %v, want an empty list", p.Candidates)
		}
		_ = e.as.AnswerAssist(context.Background(), e.helper, search.AssistAnswer{RequestID: ids.MustParse(p.RequestID), Terms: []string{"takeover"}})
	}
	res, err := e.as.Assist(context.Background(), e.alex, search.AssistRequest{Q: "Umzug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 || res.Hits[0].ThreadID != takeover {
		t.Fatalf("hits %+v", res.Hits)
	}
}

func TestAssist_FallsBackWhenTheAssistantIsAwayOrSlow(t *testing.T) {
	t.Parallel()
	away := newAssistEnv(t, true)
	away.relay.down = true
	res, err := away.as.Assist(context.Background(), away.alex, search.AssistRequest{Q: "takeover"})
	if err != nil || res.Mode != search.ModeFulltext || res.FallbackReason != search.FallbackUnavailable {
		t.Fatalf("away: %+v, %v", res, err)
	}
	slow := newAssistEnv(t, true)
	thread := ids.New()
	slow.search.threads["takeover"] = []search.ResultHit{threadHit(thread, slow.general, "Takeover", day)}
	done := make(chan search.AssistResult, 1)
	go func() {
		r, err := slow.as.Assist(context.Background(), slow.alex, search.AssistRequest{Q: "takeover"})
		if err != nil {
			t.Error(err)
		}
		done <- r
	}()
	if d := <-slow.timer.started; d != 10*time.Second {
		t.Fatalf("waited %v, want search.assist.timeout", d)
	}
	slow.timer.fire <- time.Now()
	r := <-done
	if r.Mode != search.ModeFulltext || r.FallbackReason != search.FallbackTimeout || len(r.Hits) != 1 || r.Hits[0].ThreadID != thread {
		t.Fatalf("slow: %+v", r)
	}
	// The late answer learns that it came too late.
	var p struct {
		RequestID string `json:"request_id"`
	}
	_ = json.Unmarshal(raw(slow.relay.events[0]), &p)
	err = slow.as.AnswerAssist(context.Background(), slow.helper, search.AssistAnswer{RequestID: ids.MustParse(p.RequestID)})
	if errs.KindOf(err) != errs.KindGone {
		t.Fatalf("late answer: %v, want 410", err)
	}
}

func TestAnswerAssist_UnknownRequestAndOtherAgents(t *testing.T) {
	t.Parallel()
	e := newAssistEnv(t, true)
	if err := e.as.AnswerAssist(context.Background(), e.helper, search.AssistAnswer{RequestID: ids.New()}); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("unknown request: %v", err)
	}
	other := actor.Actor{ID: ids.New(), Kind: actor.KindAgent}
	results := make(chan error, 3)
	e.relay.answer = func(ev eventlog.EphemeralEvent) {
		var p struct {
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal(raw(ev), &p)
		req := ids.MustParse(p.RequestID)
		results <- e.as.AnswerAssist(context.Background(), other, search.AssistAnswer{RequestID: req})
		results <- e.as.AnswerAssist(context.Background(), e.helper, search.AssistAnswer{RequestID: req})
		results <- e.as.AnswerAssist(context.Background(), e.helper, search.AssistAnswer{RequestID: req, Terms: []string{"x"}})
	}
	if _, err := e.as.Assist(context.Background(), e.alex, search.AssistRequest{Q: "takeover"}); err != nil {
		t.Fatal(err)
	}
	if err := <-results; errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("other agent: %v, want 404", err)
	}
	if err := <-results; err != nil {
		t.Fatalf("first answer: %v", err)
	}
	if err := <-results; err != nil {
		t.Fatalf("second answer: %v, want nil without effect", err)
	}
}

func TestAssist_HistoryMarksShownThreadsAndKeepsTerms(t *testing.T) {
	t.Parallel()
	e := newAssistEnv(t, true)
	older, newer := ids.New(), ids.New()
	q := "older or takeover or larkspur"
	e.search.messages[q] = []search.ResultHit{
		messageHit(newer, e.general, "Takeover again", "takeover", day.Add(48*time.Hour)),
		messageHit(older, e.general, "Takeover of Larkspur", "takeover", day),
	}
	got := make(chan string, 1)
	e.relay.answer = func(ev eventlog.EphemeralEvent) {
		got <- string(raw(ev))
		var p struct {
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal(raw(ev), &p)
		_ = e.as.AnswerAssist(context.Background(), e.helper, search.AssistAnswer{RequestID: ids.MustParse(p.RequestID),
			Picks: []search.AssistPick{{Ref: "c2", Reason: "The older one."}}})
	}
	res, err := e.as.Assist(context.Background(), e.alex, search.AssistRequest{Q: "no, the older one",
		History: []search.AssistTurn{{Q: "takeover of larkspur", Terms: []string{"takeover", "larkspur"}, ThreadIDs: []ids.ID{newer}}}})
	if err != nil {
		t.Fatal(err)
	}
	payload := <-got
	if !strings.Contains(payload, `"history":["takeover of larkspur"]`) {
		t.Fatalf("history missing: %s", payload)
	}
	var p struct {
		Candidates []struct {
			Ref   string `json:"ref"`
			Shown bool   `json:"shown_before"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Candidates) != 2 || !p.Candidates[0].Shown || p.Candidates[1].Shown {
		t.Fatalf("candidates %+v, want the shown thread marked", p.Candidates)
	}
	if res.Hits[0].ThreadID != older || res.Hits[0].Reason != "The older one." {
		t.Fatalf("hits %+v", res.Hits)
	}
}
