// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// opAutocompleteResult is the op of the owner's answer (04 section 6.2).
const opAutocompleteResult = "command.autocomplete_result"

// AutocompleteRequest is one request the relay hands to the owner (brief M2-17 design note 5): the
// ephemeral event, the revealed invocation token (the delivery field invocation_token) and the
// correlation of the person's request (correlation_id, and trace_context when it was recorded).
type AutocompleteRequest struct {
	OwnerID         ids.ID
	Event           eventlog.EphemeralEvent
	InvocationToken string
	CorrelationID   ids.ID
	TraceContext    string
}

// AskAutocomplete offers r to exactly one live connection of the owner in this process (brief M2-17
// design note 5): consumer default if it is live, otherwise the most recently attached live
// connection; connections of other principals never get the frame. The connection is admitted
// with agent.events.consume on the owner in delivery mode (filter, no audit), the frame is queued
// without blocking and dropped when the queue is full (never 4014). sent is false when no
// connection was live, admitted or had room. Errors only report programming faults: another type,
// a schema version below 1, a payload that is no JSON object, contains null or reaches
// sync.event_payload_max_bytes.
func (h *Hub) AskAutocomplete(ctx context.Context, r AutocompleteRequest) (bool, error) {
	ev := r.Event
	if ev.Type != string(authorize.EventCommandAutocompleteRequested) || ev.SchemaVersion < 1 || ev.Audience != r.OwnerID {
		return false, fmt.Errorf("gateway: AskAutocomplete takes only %s for its owner", authorize.EventCommandAutocompleteRequested)
	}
	payload, err := h.ephemeralPayload(ev)
	if err != nil {
		return false, err
	}
	members := h.membersOf(r.OwnerID)
	cands := make([]domain.AutocompleteCandidate, len(members))
	for i, m := range members {
		s, a := m.c.current()
		consumer := ""
		if s != nil {
			consumer = s.Consumer
		}
		cands[i] = domain.AutocompleteCandidate{Consumer: consumer, Live: a.ID == r.OwnerID && h.live(m), Attached: m.attached.Load()}
	}
	i, ok := domain.PickAutocompleteConnection(cands)
	if !ok {
		return false, nil
	}
	m := members[i]
	_, a := m.c.current()
	d, err := h.d.Authorizer.Authorize(authorize.Delivery(ctx), a, authorize.ActionAgentEventsConsume,
		authorize.Resource{Kind: authorize.ResourceAgent, ID: r.OwnerID})
	if err != nil || !d.Allowed() {
		// A refused or failed admission drops the frame like a full queue: the person gets the
		// empty list, the durable path closes or revokes the connection on its next event.
		return false, nil
	}
	occurred := ev.OccurredAt
	if occurred.IsZero() {
		occurred = h.d.Clock.Now()
	}
	env := ephemeralEnvelope(sync.Event{
		Type: ev.Type, SchemaVersion: ev.SchemaVersion, OccurredAt: occurred, SpaceID: ev.Scope.SpaceID, ChannelID: ev.Scope.ChannelID,
		ThreadID: ev.Scope.ThreadID, ResourceKind: ev.Resource.Kind, ResourceID: ev.Resource.ID, AudiencePrincipalID: ev.Audience,
		ActorKind: actorKind(ev.Actor), ActorPrincipalID: actorID(ev.Actor), Payload: payload,
	})
	env.CorrelationID, env.TraceContext, env.InvocationToken = r.CorrelationID, r.TraceContext, r.InvocationToken
	frame, err := json.Marshal(outFrame{Op: opDispatch, D: env})
	if err != nil {
		return false, fmt.Errorf("gateway: %s cannot be encoded: %w", ev.Type, err)
	}
	entry := domain.Entry{Frame: frame, Class: domain.ClassDispatch, SpaceID: ev.Scope.SpaceID, ChannelID: ev.Scope.ChannelID,
		OccurredAt: occurred, Live: true, Ephemeral: true}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.phase == domain.PhaseLive && m.c.conn.Offer(entry), nil
}

// actorKind is the envelope kind of a, "" without an actor.
func actorKind(a *eventlog.ActorRef) string {
	if a == nil {
		return ""
	}
	return string(a.Kind)
}

// actorID is the principal of a, zero without an actor.
func actorID(a *eventlog.ActorRef) ids.ID {
	if a == nil {
		return ids.ID{}
	}
	return a.PrincipalID
}

// AutocompleteAnswerer is the part of runs.CommandExec the op command.autocomplete_result uses
// (M2-17); runs authorizes command.autocomplete.respond.
type AutocompleteAnswerer interface {
	AnswerAutocomplete(ctx context.Context, a runs.AutocompleteAnswer) error
}

// AutocompleteResult is the body of command.autocomplete_result (schema GatewayAutocompleteResult).
type AutocompleteResult struct {
	RequestID ids.ID
	Choices   []runs.Choice
	// Ref is the client reference echoed in an error frame.
	Ref string
}

// autocompleteResult is the handler of command.autocomplete_result (brief M2-17 design note 8):
// rl.agent_runtime keyed by the principal before anything else, then CommandExec.AnswerAutocomplete.
// Success sends no frame; a failure answers error {op, code, detail, ref} and keeps the
// connection, where detail names the pointer and the code of the first field error, never a value;
// a credential revoked since identify closes with 4012.
func (g *Gateway) autocompleteResult(ctx context.Context, c *Connection, d any) error {
	in, ok := d.(AutocompleteResult)
	if !ok {
		return closeWith(domain.CloseDecodeError)
	}
	_, a := c.current()
	ctx = actor.WithActor(ctx, a)
	if g.d.AgentLimiter != nil {
		allowed, retry, err := g.d.AgentLimiter.AllowAgent(a)
		if err != nil {
			return err
		}
		if !allowed {
			return &OpError{Code: string(errs.CodeRateLimited), Detail: "retry_after=" + strconv.Itoa(retry), Ref: in.Ref}
		}
	}
	err := g.d.Autocomplete.AnswerAutocomplete(ctx, runs.AutocompleteAnswer{RequestID: in.RequestID, Choices: in.Choices})
	if err == nil {
		return nil
	}
	e, ok := errs.As(err)
	switch {
	case ok && e.Kind() == errs.KindUnauthenticated:
		return closeWith(domain.CloseRevoked)
	case ok && autocompleteOpErrors[e.Kind()]:
		return &OpError{Code: string(e.PublicCode()), Detail: fieldDetail(e), Ref: in.Ref}
	}
	return err
}

// autocompleteOpErrors are the kinds the op answers with an error frame (the codes of design note 7
// and rate_limited); every other error ends the connection with 4000.
var autocompleteOpErrors = map[errs.Kind]bool{
	errs.KindBadRequest: true, errs.KindForbidden: true, errs.KindNotFound: true, errs.KindGone: true, errs.KindRateLimited: true,
}

// fieldDetail is "<pointer> <code>" of the first field error, "" without one; never a value.
func fieldDetail(e *errs.Error) string {
	if len(e.Fields) == 0 {
		return ""
	}
	return e.Fields[0].Pointer + " " + e.Fields[0].Code
}
