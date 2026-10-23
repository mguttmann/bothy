// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	gosync "sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// AutocompleteConfig holds the keys of autocompletion (09 section 21), bound in internal/wire
// (wire.AutocompleteConfigFrom).
type AutocompleteConfig struct {
	// Timeout is command.autocomplete_timeout: the longest wait for the owner's answer, and the
	// time an answered or expired request is remembered after its expires_at (410 gone and 204).
	Timeout time.Duration `config:"command.autocomplete_timeout"`
	// MaxChoices is command.autocomplete_max_choices, the most choices of one answer.
	MaxChoices int `config:"command.autocomplete_max_choices"`
	// InteractionTimeout is app.interaction_timeout: an app asked through its interaction address
	// answers within it, and never later than the end of the person's wait (M2-25 design note 10).
	InteractionTimeout time.Duration `config:"app.interaction_timeout"`
}

// AutocompleteDeps are the collaborators of autocompletion.
type AutocompleteDeps struct {
	// Relay hands a request to one live gateway connection of the owner (the gateway, M2-17).
	Relay runs.AutocompleteRelay
	// After starts the timer of one wait and returns its channel and its stop function; nil means
	// a timer of the Go runtime. Tests drive the wait with a manual timer.
	After func(d time.Duration) (<-chan time.Time, func())
}

// autocompleteBox is the bound autocompletion: configuration, relay, timer and the in-process
// registry of open requests (design note 6). mu guards open and every request in it.
type autocompleteBox struct {
	cfg   AutocompleteConfig
	relay runs.AutocompleteRelay
	after func(d time.Duration) (<-chan time.Time, func())

	mu   gosync.Mutex
	open map[ids.ID]*openRequest
}

// openRequest is one request in the registry: the place and owner the answer is checked against,
// its state and the channel that wakes its waiter. The state lives only in this process: a restart
// forgets every request (04 section 9 point 4).
type openRequest struct {
	owner, command, channel, thread ids.ID
	req                             domain.AutocompleteRequest
	done                            chan struct{}
}

// runtimeTimer is the default timer of a wait.
func runtimeTimer(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTimer(d)
	return t.C, func() { t.Stop() }
}

// BindAutocomplete sets the relay and the keys of autocompletion; internal/wire calls it once the
// gateway exists. Errors: a nil relay, a timeout that is not positive or fewer than one choice.
func (s *Service) BindAutocomplete(cfg AutocompleteConfig, d AutocompleteDeps) error {
	if d.Relay == nil {
		return errors.New("runs: BindAutocomplete needs the relay")
	}
	if cfg.Timeout <= 0 || cfg.MaxChoices < 1 || cfg.InteractionTimeout <= 0 {
		return errors.New("runs: command.autocomplete_timeout and app.interaction_timeout must be positive and " +
			"command.autocomplete_max_choices at least 1")
	}
	if d.After == nil {
		d.After = runtimeTimer
	}
	s.autocomplete.Store(&autocompleteBox{cfg: cfg, relay: d.Relay, after: d.After, open: map[ids.ID]*openRequest{}})
	return nil
}

// autocompleteState is the atomic holder of the bound autocompletion in Service.
type autocompleteState = atomic.Pointer[autocompleteBox]

// sweep forgets every request from its expires_at plus command.autocomplete_timeout on (design
// note 6): every call of Autocomplete and AnswerAutocomplete sweeps, there is no goroutine.
func (b *autocompleteBox) sweep(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, e := range b.open {
		if e.req.Forgotten(now, b.cfg.Timeout) {
			delete(b.open, id)
		}
	}
}

// Autocomplete implements runs.CommandExec (brief M2-17 design note 2). It holds no transaction and
// no lock across a call, writes nothing and appends no durable event: resolve the command, check
// the thread, read the channel, decide command.autocomplete at the pinned permission version in
// request mode, validate the query, answer at once for an app owner or without a target thread,
// build the payload, issue the token, register the request, ask the relay and wait.
func (s *Service) Autocomplete(ctx context.Context, q runs.AutocompleteQuery) (runs.AutocompleteResult, error) {
	b, cmds, issuer := s.autocomplete.Load(), s.commands.Load(), s.tokens.Load()
	if b == nil || cmds == nil || issuer == nil || s.d.Versions == nil {
		return runs.AutocompleteResult{}, errs.New(errs.CodeInternalError, "runs: Autocomplete without its relay, commands, tokens or versions")
	}
	b.sweep(s.d.Clock.Now())
	c, err := cmds.register.Resolve(ctx, q.CommandID, q.ChannelID)
	if err != nil {
		return runs.AutocompleteResult{}, err
	}
	ch, err := s.autocompletePlace(ctx, q)
	if err != nil {
		return runs.AutocompleteResult{}, err
	}
	target := domain.AutocompleteTarget(q.ThreadID, ch.Kind, ch.MainThreadID)
	v, err := s.d.Versions.PermissionVersion(ctx)
	if err != nil {
		return runs.AutocompleteResult{}, err
	}
	ctx = authorize.WithPermissionVersion(ctx, v)
	if err := s.authorizeAutocomplete(ctx, c, q.ChannelID, target); err != nil {
		return runs.AutocompleteResult{}, err
	}
	if err := domain.ValidateAutocompleteQuery(optionDefs(c), q.Option, optionValues(q.Options)); err != nil {
		return runs.AutocompleteResult{}, err
	}
	if c.OwnerKind == ownerKindApp {
		// An app is asked over its way, without a token (M2-25 design note 10).
		return s.askApp(ctx, b, c, q, ch.SpaceID, target)
	}
	if c.OwnerKind != ownerKindAgent || target.IsZero() {
		// A forum without a thread has no main thread for a token (R-AZ-26). The check above ran,
		// so the empty list reveals nothing.
		return emptyResult(), nil
	}
	return s.askOwner(ctx, b, issuer.issuer, c, q, ch.SpaceID, target, v)
}

// autocompletePlace checks the thread of the request against the channel, before authorize,
// because contradictory path hints are an internal error there (M1-28), and reads the channel
// facts. An unknown thread or channel, or a thread of another channel, is 404 like an unknown ID.
func (s *Service) autocompletePlace(ctx context.Context, q runs.AutocompleteQuery) (chat.TriggerChannel, error) {
	if !q.ThreadID.IsZero() {
		info, err := s.d.Threads.Info(ctx, q.ThreadID)
		if errs.KindOf(err) == errs.KindNotFound || err == nil && info.ChannelID != q.ChannelID {
			return chat.TriggerChannel{}, invisible()
		}
		if err != nil {
			return chat.TriggerChannel{}, err
		}
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, q.ChannelID)
	if errs.KindOf(err) == errs.KindNotFound {
		return chat.TriggerChannel{}, invisible()
	}
	return ch, err
}

// authorizeAutocomplete decides command.autocomplete on the channel with the target thread as path
// hint (design note 3), in request mode so authz audits every denial; a denial is mapped as for
// invokeCommand (06 section 4 point 6).
func (s *Service) authorizeAutocomplete(ctx context.Context, c agents.Command, channel, target ids.ID) error {
	r := authorize.Resource{
		Kind: authorize.ResourceChannel, ID: channel, ThreadID: target,
		Command: &authorize.CommandFacts{OwnerKind: c.OwnerKind, OwnerID: c.OwnerPrincipalID, RequiredLevel: c.RequiredLevel},
	}
	if c.OwnerKind == ownerKindAgent {
		// A slash command addresses its owner explicitly, like a mention (09 section 10.3); no
		// active run, because autocompletion runs without one (05 section 4.10).
		r.Invoke = &authorize.InvokeFacts{AgentID: c.OwnerPrincipalID, TriggerOrigin: originHuman, MentionsAgent: true, NewThread: target.IsZero()}
	}
	d, err := s.d.Authorizer.Authorize(ctx, caller(ctx), authorize.ActionCommandAutocomplete, r)
	if err != nil {
		return err
	}
	if !d.Allowed() {
		return mapInvocationDenial(d)
	}
	return nil
}

// askOwner builds payload and token of an allowed request for an agent owner (design note 4),
// registers it, asks the relay and waits (design notes 2 h to j).
func (s *Service) askOwner(ctx context.Context, b *autocompleteBox, issuer TokenIssuer, c agents.Command, q runs.AutocompleteQuery,
	space, target ids.ID, version int64) (runs.AutocompleteResult, error) {
	person := caller(ctx).ID
	now := s.d.Clock.Now().UTC()
	reqID := ids.New()
	expires := now.Add(b.cfg.Timeout)
	payload, err := domain.AutocompletePayload(domain.AutocompletePayloadInput{
		RequestID: reqID, CommandID: c.ID, ChannelID: q.ChannelID, ThreadID: target, UserID: person, Option: q.Option, Value: q.Value,
		Options: optionValues(q.Options), ExpiresAt: expires,
	})
	if err != nil {
		return runs.AutocompleteResult{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of command.autocomplete_requested")
	}
	if len(payload) >= s.cfg.EventPayloadMaxBytes {
		// Truncating a partial text makes no sense (04 section 2.2 rule 2; docs/DECISIONS.md D-M2-17).
		return runs.AutocompleteResult{}, domain.AutocompleteOptionError("", domain.FieldTooLong, "the request does not fit sync.event_payload_max_bytes")
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{c.OwnerPrincipalID})
	if err != nil {
		return runs.AutocompleteResult{}, err
	}
	agent, ok := facts[c.OwnerPrincipalID]
	if !ok {
		return runs.AutocompleteResult{}, errs.New(errs.CodeInternalError, "runs: the owner of an allowed command is no agent")
	}
	tok, err := issuer.Issue(ctx, domain.AutocompleteGrant(domain.AutocompleteGrantInput{
		AgentID: c.OwnerPrincipalID, TriggeredBy: person, CommandID: c.ID, ThreadID: target, ChannelID: q.ChannelID, SpaceID: space,
		BillingMode: agent.BillingMode, ToolClass: agent.ToolClass, PermissionVersion: version,
	}))
	if err != nil {
		return runs.AutocompleteResult{}, err
	}
	ev := eventlog.EphemeralEvent{
		Type: string(authorize.EventCommandAutocompleteRequested), SchemaVersion: 1,
		Actor:    &eventlog.ActorRef{Kind: eventlog.ActorUser, PrincipalID: person},
		Scope:    eventlog.Scope{SpaceID: space, ChannelID: q.ChannelID, ThreadID: target},
		Resource: eventlog.ResourceRef{Kind: resourceCommand, ID: c.ID}, Audience: c.OwnerPrincipalID, OccurredAt: now,
		Payload: json.RawMessage(payload),
	}
	e := &openRequest{owner: c.OwnerPrincipalID, command: c.ID, channel: q.ChannelID, thread: target,
		req: domain.NewAutocompleteRequest(expires), done: make(chan struct{})}
	b.mu.Lock()
	b.open[reqID] = e
	b.mu.Unlock()
	sent, err := b.relay.Ask(ctx, c.OwnerPrincipalID, ev, tok.Token.Reveal())
	if err != nil || !sent {
		// Nobody got the request: the server does not wait (R-AG-14 "never hangs").
		b.mu.Lock()
		delete(b.open, reqID)
		b.mu.Unlock()
		if err != nil {
			return runs.AutocompleteResult{}, errs.Wrap(errs.CodeInternalError, err, "runs: the relay of command.autocomplete_requested")
		}
		return emptyResult(), nil
	}
	return b.wait(ctx, e, b.cfg.Timeout)
}

// wait waits for the first accepted answer, the end of d (command.autocomplete_timeout, or what is
// left of it) or the end of ctx (design note 6); every transition is a compare-and-swap under the
// mutex, so a timer that fires after an answer won returns the answer's choices.
func (b *autocompleteBox) wait(ctx context.Context, e *openRequest, d time.Duration) (runs.AutocompleteResult, error) {
	fire, stop := b.after(d)
	defer stop()
	ctxEnded := false
	select {
	case <-e.done:
	case <-fire:
	case <-ctx.Done():
		ctxEnded = true
	}
	b.mu.Lock()
	if e.req.Expire() {
		close(e.done)
	}
	state, choices := e.req.State(), e.req.Choices()
	b.mu.Unlock()
	switch {
	case state == domain.AutocompleteAnswered:
		return runs.AutocompleteResult{Choices: toChoices(choices)}, nil
	case ctxEnded:
		// The client is gone, http.request_timeout passed or the server stops: nothing is written.
		return runs.AutocompleteResult{}, ctx.Err()
	}
	return emptyResult(), nil
}

// AnswerAutocomplete implements runs.CommandExec (brief M2-17 design note 7): find the request
// (unknown or forgotten is 404 without authorize), decide command.autocomplete.respond in request
// mode, validate the choices (the request stays open), then 410 for an expired request, nil
// without effect for an answered one, and otherwise the compare-and-swap open -> answered that
// wakes the waiter. No audit entry for an allowed answer; authz audits denials.
func (s *Service) AnswerAutocomplete(ctx context.Context, a runs.AutocompleteAnswer) error {
	b := s.autocomplete.Load()
	if b == nil {
		return errs.New(errs.CodeInternalError, "runs: AnswerAutocomplete without its relay")
	}
	b.sweep(s.d.Clock.Now())
	b.mu.Lock()
	e := b.open[a.RequestID]
	b.mu.Unlock()
	if e == nil {
		return invisible()
	}
	r := authorize.Resource{Kind: authorize.ResourceCommand, ID: e.command, ChannelID: e.channel, ThreadID: e.thread, OwnerID: e.owner}
	d, err := s.d.Authorizer.Authorize(ctx, caller(ctx), authorize.ActionCommandAutocompleteRespond, r)
	if err != nil {
		return err
	}
	if !d.Allowed() {
		return d.Err()
	}
	choices := fromChoices(a.Choices)
	if err := domain.ValidateAutocompleteAnswer(choices, b.cfg.MaxChoices); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch e.req.State() {
	case domain.AutocompleteExpired:
		return errs.New(errs.CodeGone, "")
	case domain.AutocompleteAnswered:
		// The first answer counts (06 section 7 point 3); a retry after a lost response is no error.
		return nil
	case domain.AutocompleteOpen:
		if e.req.Answer(choices) {
			close(e.done)
		}
	}
	return nil
}

// emptyResult is the answer without suggestions; Choices encodes as [].
func emptyResult() runs.AutocompleteResult { return runs.AutocompleteResult{Choices: []runs.Choice{}} }

// toChoices converts accepted choices; never nil.
func toChoices(in []domain.Choice) []runs.Choice {
	out := make([]runs.Choice, 0, len(in))
	for _, c := range in {
		out = append(out, runs.Choice(c))
	}
	return out
}

// fromChoices converts submitted choices.
func fromChoices(in []runs.Choice) []domain.Choice {
	out := make([]domain.Choice, 0, len(in))
	for _, c := range in {
		out = append(out, domain.Choice(c))
	}
	return out
}
