// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// acTimeout is command.autocomplete_timeout of the unit tests; the waits are driven by the manual
// timer, so the value only has to be recognisable.
const acTimeout = 1500 * time.Millisecond

// register is the fake command register: the commands by id; an unknown id is 404.
type register map[ids.ID]agents.Command

func (r register) Resolve(_ context.Context, commandID, _ ids.ID) (agents.Command, error) {
	if c, ok := r[commandID]; ok {
		return c, nil
	}
	return agents.Command{}, errs.New(errs.CodeNotFound, "")
}

// asked is one call of the fake relay.
type asked struct {
	owner   ids.ID
	ev      eventlog.EphemeralEvent
	token   string
	request ids.ID
}

// relay is the fake AutocompleteRelay: it records every Ask, answers sent and runs onAsk.
type relay struct {
	mu    sync.Mutex
	calls []asked
	sent  bool
	onAsk func(a asked)
}

func (r *relay) Ask(_ context.Context, owner ids.ID, ev eventlog.EphemeralEvent, token string) (bool, error) {
	var p struct {
		RequestID string `json:"request_id"`
	}
	raw, _ := ev.Payload.(json.RawMessage)
	_ = json.Unmarshal(raw, &p)
	id, _ := ids.Parse(p.RequestID)
	a := asked{owner: owner, ev: ev, token: token, request: id}
	r.mu.Lock()
	r.calls = append(r.calls, a)
	on := r.onAsk
	r.mu.Unlock()
	if on != nil {
		on(a)
	}
	return r.sent, nil
}

func (r *relay) list() []asked {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]asked{}, r.calls...)
}

// timers is the manual timer of the wait: every started timer is a channel the test fires. The
// use case starts the timer after the relay returned, so a test that looks at the timer waits for
// its start on starts first.
type timers struct {
	mu      sync.Mutex
	started []time.Duration
	starts  chan time.Duration
	fire    chan time.Time
	stopped int
}

func (m *timers) after(d time.Duration) (<-chan time.Time, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = append(m.started, d)
	select {
	case m.starts <- d:
	default:
	}
	return m.fire, func() {
		m.mu.Lock()
		m.stopped++
		m.mu.Unlock()
	}
}

// list returns the durations of the started timers.
func (m *timers) list() []time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]time.Duration{}, m.started...)
}

// acWorld is the app world with the command /ticket of Lotse in C, bound tokens and the fakes of
// the relay and the timer.
type acWorld struct {
	*world
	relay   *relay
	timers  *timers
	cmd     agents.Command
	reg     register
	forum   ids.ID
	trackID ids.ID
	inv     *invEnv
}

func newACWorld(t *testing.T) *acWorld {
	t.Helper()
	w := newWorld(t)
	inv := newInvEnv(t)
	if err := w.svc.BindTokens(inv.svc); err != nil {
		t.Fatal(err)
	}
	aw := &acWorld{world: w, relay: &relay{sent: true}, timers: &timers{fire: make(chan time.Time, 1), starts: make(chan time.Duration, 16)}, forum: ids.New(), trackID: ids.New(),
		inv: inv}
	aw.cmd = agents.Command{ID: ids.New(), OwnerPrincipalID: w.lotse, OwnerKind: "agent", Name: "ticket", ScopeKind: "instance",
		Options: []agents.CommandOption{
			{Name: "ticket", Type: "string", Autocomplete: true},
			{Name: "priority", Type: "choice", Choices: []agents.CommandChoice{{Name: "High", Value: "high"}}},
		}, RequiredLevel: "trigger"}
	aw.reg = register{aw.cmd.ID: aw.cmd}
	w.channels[w.channel] = chat.TriggerChannel{ChannelID: w.channel, SpaceID: w.space, Kind: "text", EffectiveOperatorID: w.lotse,
		MainThreadID: w.main}
	w.channels[aw.forum] = chat.TriggerChannel{ChannelID: aw.forum, SpaceID: w.space, Kind: "forum", EffectiveOperatorID: w.lotse}
	if err := w.svc.BindCommands(aw.reg); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.BindAutocomplete(app.AutocompleteConfig{Timeout: acTimeout, MaxChoices: 3},
		app.AutocompleteDeps{Relay: aw.relay, After: aw.timers.after}); err != nil {
		t.Fatal(err)
	}
	return aw
}

// alexCtx is the context of a request of Alex.
func alexCtx(w *acWorld) context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: w.alex, Kind: actor.KindUser})
}

// lotseCtx is the context of a request of Lotse.
func lotseCtx(w *acWorld) context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: w.lotse, Kind: actor.KindAgent})
}

// query is /ticket ticket=47 in C without thread.
func (w *acWorld) query() runs.AutocompleteQuery {
	return runs.AutocompleteQuery{ChannelID: w.channel, CommandID: w.cmd.ID, Option: "ticket", Value: "47"}
}

// result is the outcome of one Autocomplete in its own goroutine.
type result struct {
	res runs.AutocompleteResult
	err error
}

// start runs Autocomplete in a goroutine and waits until the relay was asked.
func (w *acWorld) start(ctx context.Context, q runs.AutocompleteQuery) (<-chan result, asked) {
	w.t.Helper()
	asks := make(chan asked, 1)
	w.relay.mu.Lock()
	w.relay.onAsk = func(a asked) { asks <- a }
	w.relay.mu.Unlock()
	out := make(chan result, 1)
	go func() {
		r, err := w.svc.Autocomplete(ctx, q)
		out <- result{r, err}
	}()
	select {
	case a := <-asks:
		return out, a
	case r := <-out:
		w.t.Fatalf("Autocomplete returned before asking: %+v %v", r.res, r.err)
	case <-time.After(testkit.Patience):
		w.t.Fatal("the relay was never asked")
	}
	return nil, asked{}
}

// wait returns the result of out, at most testkit.Patience.
func (w *acWorld) wait(out <-chan result) result {
	w.t.Helper()
	return testkit.Receive(w.t, "the result of Autocomplete", out)
}

// notYet fails when out already has a result. It does not wait: the manual timer has not fired,
// so a correct wait cannot have ended, and a quiet window would prove nothing more.
func (w *acWorld) notYet(out <-chan result) {
	w.t.Helper()
	select {
	case r := <-out:
		w.t.Fatalf("Autocomplete returned early: %+v %v", r.res, r.err)
	default:
	}
}

// TestAutocomplete_AG23_EmptyAfterTimeout: the relay takes the request, nobody answers; the result
// is an empty list once the timer of command.autocomplete_timeout fires, not earlier (AG-23).
func TestAutocomplete_AG23_EmptyAfterTimeout(t *testing.T) {
	w := newACWorld(t)
	out, a := w.start(alexCtx(w), w.query())
	if d := testkit.Receive(t, "the start of the timer", w.timers.starts); d != acTimeout {
		t.Fatalf("timer of %s, want command.autocomplete_timeout", d)
	}
	w.notYet(out)
	if st := w.timers.list(); len(st) != 1 || st[0] != acTimeout {
		t.Fatalf("timers %v, want one of command.autocomplete_timeout", st)
	}
	w.timers.fire <- w.clk.Now()
	r := w.wait(out)
	if r.err != nil || r.res.Choices == nil || len(r.res.Choices) != 0 {
		t.Fatalf("result %+v %v, want an empty non-nil list", r.res, r.err)
	}
	if a.owner != w.lotse || a.token == "" || a.ev.Type != "command.autocomplete_requested" || a.ev.Audience != w.lotse ||
		a.ev.Scope.ThreadID != w.main || a.ev.Resource.Kind != "command" || a.ev.Resource.ID != w.cmd.ID || a.ev.Resource.Version != 0 {
		t.Fatalf("ask %+v", a)
	}
	// The token is an autocomplete token without run on the main thread, like the payload.
	c := claimsOf(t, a.token)
	if c["trigger_kind"] != "autocomplete" || c["run_id"] != nil || c["thread_id"] != w.main.String() || c["command_id"] != w.cmd.ID.String() ||
		c["level"] != "trigger" || c["aud"] != w.lotse.String() || c["sub"] != w.alex.String() {
		t.Fatalf("claims %v", c)
	}
	// Nothing durable: no run, no event, no audit entry.
	if len(w.repo.runs) != 0 || w.j.count("event") != 0 {
		t.Fatalf("autocomplete wrote %v", w.j.whats())
	}
	// A later answer learns that it came too late.
	err := w.svc.AnswerAutocomplete(lotseCtx(w), runs.AutocompleteAnswer{RequestID: a.request})
	if errs.KindOf(err) != errs.KindGone {
		t.Fatalf("late answer: %v", err)
	}
}

// TestAutocomplete_NotSentGivesEmptyAtOnce: no live connection means an empty list at once and the
// request is forgotten.
func TestAutocomplete_NotSentGivesEmptyAtOnce(t *testing.T) {
	w := newACWorld(t)
	w.relay.sent = false
	r, err := w.svc.Autocomplete(alexCtx(w), w.query())
	if err != nil || r.Choices == nil || len(r.Choices) != 0 {
		t.Fatalf("result %+v %v", r, err)
	}
	if len(w.timers.list()) != 0 {
		t.Fatal("a request that nobody got waits for nothing")
	}
	calls := w.relay.list()
	if len(calls) != 1 {
		t.Fatalf("asks %d", len(calls))
	}
	if err := w.svc.AnswerAutocomplete(lotseCtx(w), runs.AutocompleteAnswer{RequestID: calls[0].request}); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("answer to a removed request: %v", err)
	}
}

// TestAutocomplete_AppOwnerAndForumWithoutThread: an app owner is never asked (until M2-25); an
// agent owner in a forum without thread_id is checked as for a new thread and never asked
// (R-AZ-26); a denial still gives its error.
func TestAutocomplete_AppOwnerAndForumWithoutThread(t *testing.T) {
	w := newACWorld(t)
	app := agents.Command{ID: w.trackID, OwnerPrincipalID: ids.New(), OwnerKind: "app", Name: "track", ScopeKind: "instance",
		Options: []agents.CommandOption{{Name: "ticket", Type: "string", Autocomplete: true}}, RequiredLevel: "write"}
	w.reg[app.ID] = app
	q := w.query()
	q.CommandID = app.ID
	r, err := w.svc.Autocomplete(alexCtx(w), q)
	if err != nil || r.Choices == nil || len(r.Choices) != 0 || len(w.relay.list()) != 0 {
		t.Fatalf("app owner: %+v %v asks %d", r, err, len(w.relay.list()))
	}
	forum := w.query()
	forum.ChannelID = w.forum
	r, err = w.svc.Autocomplete(alexCtx(w), forum)
	if err != nil || r.Choices == nil || len(r.Choices) != 0 || len(w.relay.list()) != 0 || len(w.timers.list()) != 0 {
		t.Fatalf("forum: %+v %v", r, err)
	}
	var seen authorize.Resource
	for _, c := range w.j.list() {
		if c.what == "authz:command.autocomplete:request" {
			seen, _ = c.arg.(authorize.Resource)
		}
	}
	if seen.ID != w.forum || seen.Invoke == nil || !seen.Invoke.NewThread || !seen.ThreadID.IsZero() || !seen.Invoke.MentionsAgent ||
		seen.Invoke.AgentID != w.lotse || !seen.Invoke.ActiveRunAgentID.IsZero() {
		t.Fatalf("the forum descriptor %+v", seen)
	}
	w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
		return decision{allowed: act != authorize.ActionCommandAutocomplete, reason: "billing_mode_personal_not_holder"}
	}
	_, err = w.svc.Autocomplete(alexCtx(w), forum)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeAgentInvocationDenied || e.Reason != "billing_mode_personal_not_holder" {
		t.Fatalf("denied forum: %v", err)
	}
}

// TestAutocomplete_FirstAnswerWins: the first valid answer is the result; a second one is 204
// without effect.
func TestAutocomplete_FirstAnswerWins(t *testing.T) {
	w := newACWorld(t)
	out, a := w.start(alexCtx(w), w.query())
	first := []runs.Choice{{Name: "Ticket 47", Value: "47"}, {Name: "Ticket 470", Value: "470"}}
	if err := w.svc.AnswerAutocomplete(lotseCtx(w), runs.AutocompleteAnswer{RequestID: a.request, Choices: first}); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.AnswerAutocomplete(lotseCtx(w), runs.AutocompleteAnswer{RequestID: a.request, Choices: []runs.Choice{{Name: "x", Value: "x"}}}); err != nil {
		t.Fatalf("a second answer: %v", err)
	}
	r := w.wait(out)
	if r.err != nil || len(r.res.Choices) != 2 || r.res.Choices[0] != first[0] || r.res.Choices[1] != first[1] {
		t.Fatalf("result %+v %v", r.res, r.err)
	}
}

// TestAutocomplete_TimeoutVersusAnswerRace: an answer and the end of the wait at the same moment
// give exactly one outcome (run with -race).
func TestAutocomplete_TimeoutVersusAnswerRace(t *testing.T) {
	for range 50 {
		w := newACWorld(t)
		out, a := w.start(alexCtx(w), w.query())
		choice := []runs.Choice{{Name: "a", Value: "a"}}
		answered := make(chan error, 1)
		go func() {
			answered <- w.svc.AnswerAutocomplete(lotseCtx(w), runs.AutocompleteAnswer{RequestID: a.request, Choices: choice})
		}()
		w.timers.fire <- w.clk.Now()
		r := w.wait(out)
		aerr := <-answered
		switch {
		case r.err != nil:
			t.Fatal(r.err)
		case len(r.res.Choices) == 1 && aerr == nil:
		case len(r.res.Choices) == 0 && errs.KindOf(aerr) == errs.KindGone:
		default:
			t.Fatalf("two outcomes: %+v and %v", r.res, aerr)
		}
	}
}

// TestAutocomplete_ContextEndExpires: the end of the request context expires the request; a later
// answer gets 410.
func TestAutocomplete_ContextEndExpires(t *testing.T) {
	w := newACWorld(t)
	ctx, cancel := context.WithCancel(alexCtx(w))
	out, a := w.start(ctx, w.query())
	cancel()
	r := w.wait(out)
	if !errors.Is(r.err, context.Canceled) {
		t.Fatalf("result %+v %v", r.res, r.err)
	}
	if err := w.svc.AnswerAutocomplete(lotseCtx(w), runs.AutocompleteAnswer{RequestID: a.request}); errs.KindOf(err) != errs.KindGone {
		t.Fatalf("answer after the end: %v", err)
	}
}

// TestAnswer_UnknownGives404WithoutAuthorize: an unknown request is 404 and never reaches authorize.
func TestAnswer_UnknownGives404WithoutAuthorize(t *testing.T) {
	w := newACWorld(t)
	err := w.svc.AnswerAutocomplete(lotseCtx(w), runs.AutocompleteAnswer{RequestID: ids.New()})
	if errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("unknown: %v", err)
	}
	if w.j.count("authz:command.autocomplete.respond:request") != 0 {
		t.Fatal("an unknown request reached authorize")
	}
}

// TestAnswer_Order: 404, then authorize, then 400 (the request stays open), then 410 or 204.
func TestAnswer_Order(t *testing.T) {
	w := newACWorld(t)
	out, a := w.start(alexCtx(w), w.query())
	ans := func(c ...runs.Choice) error {
		return w.svc.AnswerAutocomplete(lotseCtx(w), runs.AutocompleteAnswer{RequestID: a.request, Choices: c})
	}
	// authorize before the validation: a denied principal never learns the rules of the body.
	w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
		return decision{allowed: act != authorize.ActionCommandAutocompleteRespond, reason: "not_self"}
	}
	if err := ans(runs.Choice{}, runs.Choice{}, runs.Choice{}, runs.Choice{}); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("denied: %v", err)
	}
	var seen authorize.Resource
	for _, c := range w.j.list() {
		if c.what == "authz:command.autocomplete.respond:request" {
			seen, _ = c.arg.(authorize.Resource)
		}
	}
	if seen.Kind != authorize.ResourceCommand || seen.ID != w.cmd.ID || seen.ChannelID != w.channel || seen.ThreadID != w.main ||
		seen.OwnerID != w.lotse {
		t.Fatalf("respond descriptor %+v", seen)
	}
	w.authz.decide = nil
	err := ans(runs.Choice{Name: "a", Value: "a"}, runs.Choice{Name: "b", Value: "b"}, runs.Choice{Name: "c", Value: "c"}, runs.Choice{Name: "d", Value: "d"})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed || e.Fields[0].Code != "too_many_items" {
		t.Fatalf("too many: %v", err)
	}
	if err := ans(runs.Choice{Name: "a", Value: "a"}); err != nil {
		t.Fatalf("a corrected answer within the window: %v", err)
	}
	if r := w.wait(out); r.err != nil || len(r.res.Choices) != 1 {
		t.Fatalf("result %+v %v", r.res, r.err)
	}
	if err := ans(runs.Choice{Name: "b", Value: "b"}); err != nil {
		t.Fatalf("answered: %v", err)
	}
}

// TestAutocomplete_Validation404AndPayload: a thread of another channel is 404 before authorize, an
// option without autocomplete is 400, and a payload above sync.event_payload_max_bytes is 400
// too_long with an empty pointer.
func TestAutocomplete_Validation404AndPayload(t *testing.T) {
	w := newACWorld(t)
	other := ids.New()
	w.threads.infos[other] = chat.ThreadInfo{ID: other, ChannelID: ids.New(), SpaceID: w.space, ChannelKind: "text", Kind: "thread", State: "open"}
	q := w.query()
	q.ThreadID = other
	if _, err := w.svc.Autocomplete(alexCtx(w), q); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("thread of another channel: %v", err)
	}
	q.ThreadID = ids.New()
	if _, err := w.svc.Autocomplete(alexCtx(w), q); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("unknown thread: %v", err)
	}
	if w.j.count("authz:command.autocomplete:request") != 0 {
		t.Fatal("a contradictory thread reached authorize")
	}
	q = w.query()
	q.Option = "priority"
	if _, err := w.svc.Autocomplete(alexCtx(w), q); errs.KindOf(err) != errs.KindBadRequest {
		t.Fatalf("option without autocomplete: %v", err)
	}
	q = w.query()
	big := make([]byte, 70000)
	for i := range big {
		big[i] = 'a'
	}
	q.Value = string(big)
	_, err := w.svc.Autocomplete(alexCtx(w), q)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed || e.Fields[0].Code != "too_long" || e.Fields[0].Pointer != "" {
		t.Fatalf("too long: %v", err)
	}
	if len(w.relay.list()) != 0 {
		t.Fatal("an invalid request reached the relay")
	}
}

// TestAutocomplete_TokenIsNotARunToken: the server Verify with the agent as audience and a run
// answers invalid for an autocomplete token, so no startRun accepts it (09 4.5 "run wrong").
func TestAutocomplete_TokenIsNotARunToken(t *testing.T) {
	w := newACWorld(t)
	w.relay.sent = false
	if _, err := w.svc.Autocomplete(alexCtx(w), w.query()); err != nil {
		t.Fatal(err)
	}
	tok := w.relay.list()[0].token
	_, err := w.inv.svc.Verify(context.Background(), tok, app.VerifyInvocation{Audience: w.lotse, RunID: ids.New()})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvocationTokenInvalid || e.Reason != "invalid" {
		t.Fatalf("Verify with a run: %v", err)
	}
	if _, err := w.inv.svc.Verify(context.Background(), tok, app.VerifyInvocation{Audience: w.lotse}); err != nil {
		t.Fatalf("Verify without a run: %v", err)
	}
}
