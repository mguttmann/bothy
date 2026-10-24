// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// askApp is the app branch of Autocomplete after the check (brief M2-25 design note 10): the
// payload and the open request as for an agent, without a token (04 section 2.2), then the way of
// the app. The interaction address is asked with the earlier of the end of the person's wait and
// now plus app.interaction_timeout as its deadline: an answer gives the choices, 202 keeps waiting
// for answerAutocomplete until command.autocomplete_timeout, every other outcome an empty list at
// once. The gateway way hands the request to one live connection of the app (M2-17 relay, empty
// token); an inactive app gets an empty list at once. The use case holds no transaction, so the
// request may run inside the person's request (02 section 8.5 concerns transactions).
func (s *Service) askApp(ctx context.Context, b *autocompleteBox, c agents.Command, q runs.AutocompleteQuery, space, target ids.ID) (
	runs.AutocompleteResult, error) {
	d, err := s.appDispatcher()
	if err != nil {
		return runs.AutocompleteResult{}, err
	}
	way, err := d.Way(ctx, c.OwnerPrincipalID)
	if err != nil {
		return runs.AutocompleteResult{}, err
	}
	if way == runs.AppWayInactive {
		return emptyResult(), nil
	}
	now := s.d.Clock.Now().UTC()
	reqID := ids.New()
	expires := now.Add(b.cfg.Timeout)
	payload, err := domain.AutocompletePayload(domain.AutocompletePayloadInput{
		RequestID: reqID, CommandID: c.ID, ChannelID: q.ChannelID, ThreadID: target, UserID: caller(ctx).ID, Option: q.Option, Value: q.Value,
		Options: optionValues(q.Options), ExpiresAt: expires,
	})
	if err != nil {
		return runs.AutocompleteResult{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of command.autocomplete_requested")
	}
	if len(payload) >= s.cfg.EventPayloadMaxBytes {
		return runs.AutocompleteResult{}, domain.AutocompleteOptionError("", domain.FieldTooLong, "the request does not fit sync.event_payload_max_bytes")
	}
	ev := eventlog.EphemeralEvent{
		Type: string(authorize.EventCommandAutocompleteRequested), SchemaVersion: 1,
		Actor:    &eventlog.ActorRef{Kind: eventlog.ActorUser, PrincipalID: caller(ctx).ID},
		Scope:    eventlog.Scope{SpaceID: space, ChannelID: q.ChannelID, ThreadID: target},
		Resource: eventlog.ResourceRef{Kind: resourceCommand, ID: c.ID}, Audience: c.OwnerPrincipalID, OccurredAt: now,
		Payload: json.RawMessage(payload),
	}
	e := &openRequest{owner: c.OwnerPrincipalID, command: c.ID, channel: q.ChannelID, thread: target,
		req: domain.NewAutocompleteRequest(expires), done: make(chan struct{})}
	b.mu.Lock()
	b.open[reqID] = e
	b.mu.Unlock()
	if way == runs.AppWayGateway {
		sent, err := b.relay.Ask(ctx, c.OwnerPrincipalID, ev, "")
		if err != nil || !sent {
			b.forget(reqID)
			if err != nil {
				return runs.AutocompleteResult{}, errs.Wrap(errs.CodeInternalError, err, "runs: the relay of command.autocomplete_requested")
			}
			return emptyResult(), nil
		}
		return b.wait(ctx, e, b.cfg.Timeout)
	}
	deadline := now.Add(b.cfg.InteractionTimeout)
	if expires.Before(deadline) {
		deadline = expires
	}
	out, err := d.Ask(ctx, runs.AppAsk{AppID: c.OwnerPrincipalID, Event: ev, Deadline: deadline})
	if err != nil {
		b.forget(reqID)
		return runs.AutocompleteResult{}, err
	}
	return b.settleAsk(ctx, e, out, expires.Sub(s.d.Clock.Now().UTC()))
}

// forget removes the request id from the registry: nobody got it, so nobody may answer it.
func (b *autocompleteBox) forget(id ids.ID) {
	b.mu.Lock()
	delete(b.open, id)
	b.mu.Unlock()
}

// settleAsk applies the outcome of a request to the interaction address of an app: answered gives
// its choices (the first answer counts, so an answerAutocomplete that won keeps its own), deferred
// waits for answerAutocomplete for what is left of command.autocomplete_timeout, and every other
// outcome expires the request and answers an empty list.
func (b *autocompleteBox) settleAsk(ctx context.Context, e *openRequest, out runs.AppOutcome, left time.Duration) (runs.AutocompleteResult, error) {
	switch {
	case out.Answered:
		b.mu.Lock()
		if e.req.Answer(fromChoices(out.Choices)) {
			close(e.done)
		}
		state, choices := e.req.State(), e.req.Choices()
		b.mu.Unlock()
		if state == domain.AutocompleteAnswered {
			return runs.AutocompleteResult{Choices: toChoices(choices)}, nil
		}
		return emptyResult(), nil
	case out.Deferred && left > 0:
		return b.wait(ctx, e, left)
	}
	b.mu.Lock()
	if e.req.Expire() {
		close(e.done)
	}
	state, choices := e.req.State(), e.req.Choices()
	b.mu.Unlock()
	if state == domain.AutocompleteAnswered {
		return runs.AutocompleteResult{Choices: toChoices(choices)}, nil
	}
	return emptyResult(), nil
}
