// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The trigger kind interaction of 03 runs.trigger_kind (M2-15).
const triggerKindInteraction = "interaction"

// The interaction states of 03 interactions.
const (
	interactionReceived     = "received"
	interactionDispatched   = "dispatched"
	interactionAcknowledged = "acknowledged"
	interactionFailed       = "failed"
	interactionRejected     = "rejected"
)

// resourceInteraction and resourceComponent are the resource kinds of the events and the audit.
const (
	resourceInteraction = "interaction"
	resourceComponent   = "component"
)

// The audit attributes of an allowed approval (M2-15 design note 17).
const (
	attrInteractionID      = "interaction_id"
	attrComponentType      = "component_type"
	attrRequiredCapability = "required_capability"
	attrRestrictedTo       = "restricted_to"
	attrChoice             = "choice"
)

// fieldNotUUIDv7 is the errors[].code of a client_interaction_id that is no UUIDv7 (as M1-56 D2).
const fieldNotUUIDv7 = "not_uuid_v7"

// emptyObject is the values document {} of an interaction without values.
var emptyObject = json.RawMessage("{}")

// Create implements runs.Interactions (createInteraction, interaction.create; M2-15 design note 10):
// the component, its card and thread are read without locks (an unknown component is 404); one unit
// of work takes the locks of design note 16 (thread, runs in ascending id, card, the single-use
// group or the component), answers a repetition of client_interaction_id under them, decides
// interaction.create with the facts of the locked rows (denials are audited by authz and mapped by
// authorize.Decision.Err), checks the values against the definition, writes the single-use rule,
// the interaction, message.updated through messaging, a new run when none waits, interaction.created
// for the card's agent and, for a new run, agent.invocation last, and the allow audit of an
// approval. A unique violation of the idempotency key (only when the locks were bypassed) is
// answered like a repetition in a new unit of work.
func (s *Service) Create(ctx context.Context, cmd runs.CreateInteraction) (runs.InteractionResult, error) {
	if err := s.componentsReady(); err != nil {
		return runs.InteractionResult{}, err
	}
	if !isUUIDv7(cmd.ClientInteractionID) {
		return runs.InteractionResult{}, fieldError("/client_interaction_id", fieldNotUUIDv7, "client_interaction_id is not a UUIDv7")
	}
	values := normalizedValues(cmd.Values)
	a := caller(ctx)
	row, card, found, err := s.cardOf(ctx, cmd.ComponentID)
	if err != nil {
		return runs.InteractionResult{}, err
	}
	if !found {
		return runs.InteractionResult{}, invisible()
	}
	var out runs.InteractionResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.createInteraction(ctx, a, cmd, values, row, card)
		return err
	})
	if errors.Is(err, ErrInteractionTaken) {
		return s.interactionAfterRace(ctx, a, cmd, values, row, card)
	}
	return out, err
}

// isUUIDv7 reports a UUIDv7 with the RFC variant; the time inside is not checked (04 5.1 point 5).
func isUUIDv7(id ids.ID) bool { return id[6]>>4 == 7 && id[8]>>6 == 0b10 }

// normalizedValues is {} for absent values.
func normalizedValues(v json.RawMessage) json.RawMessage {
	t := bytes.TrimSpace(v)
	if len(t) == 0 || bytes.Equal(t, []byte("null")) {
		return emptyObject
	}
	return v
}

// interactionAfterRace answers a request whose insert met the unique key of a concurrent twin, in
// a new unit of work that takes the locks again and finds the committed twin as a repetition
// (design note 10 d); inside an outer unit of work the transaction is lost, so the request fails
// with 409 conflict instead.
func (s *Service) interactionAfterRace(ctx context.Context, a actor.Actor, cmd runs.CreateInteraction, values json.RawMessage,
	row ComponentRow, card messaging.TriggerMessage) (runs.InteractionResult, error) {
	if txn.InUnit(ctx) {
		return runs.InteractionResult{}, errs.New(errs.CodeConflict, "a concurrent request with this client_interaction_id won")
	}
	var out runs.InteractionResult
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.createInteraction(ctx, a, cmd, values, row, card)
		return err
	})
	if errors.Is(err, ErrInteractionTaken) {
		return runs.InteractionResult{}, errs.New(errs.CodeInternalError, "runs: the winning interaction of a client_interaction_id is missing")
	}
	return out, err
}

// interactionReplay looks up the interaction of a with the client_interaction_id of cmd (design
// note 10 d): a principal who may no longer read the card gets the 404 of an unknown component; the
// same component and the same values as JSON (key order ignored) are the original with
// Idempotent-Replay; anything else is 422 idempotency_key_reuse. found is false without a stored
// interaction.
func (s *Service) interactionReplay(ctx context.Context, a actor.Actor, cmd runs.CreateInteraction, values json.RawMessage,
	card messaging.TriggerMessage) (runs.InteractionResult, bool, error) {
	ix, found, err := s.d.Components.InteractionByClientID(ctx, a.ID, cmd.ClientInteractionID)
	if err != nil || !found {
		return runs.InteractionResult{}, false, err
	}
	// The stored interaction names its own card, which may differ from the requested one.
	m, err := s.d.Messages.TriggerMessageOf(ctx, ix.MessageID)
	if err != nil {
		return runs.InteractionResult{}, false, err
	}
	d, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), a, authorize.ActionMessageRead, authorize.Resource{
		Kind: authorize.ResourceMessage, ID: m.ID, ThreadID: m.ThreadID, OwnerID: m.Author.ID,
	})
	if err != nil {
		return runs.InteractionResult{}, false, err
	}
	if !d.Allowed() {
		return runs.InteractionResult{}, false, invisible()
	}
	if ix.ComponentID != cmd.ComponentID || !domain.SameValues(ix.Values, values) {
		return runs.InteractionResult{}, false, errs.New(errs.CodeIdempotencyKeyReuse, "client_interaction_id was used with another request")
	}
	view, err := s.interactionView(ctx, ix, m.ThreadID)
	if err != nil {
		return runs.InteractionResult{}, false, err
	}
	return runs.InteractionResult{Interaction: view, Replay: true}, true, nil
}

// createInteraction is the unit of work of Create.
func (s *Service) createInteraction(ctx context.Context, a actor.Actor, cmd runs.CreateInteraction, values json.RawMessage,
	row ComponentRow, card messaging.TriggerMessage) (runs.InteractionResult, error) {
	if err := s.d.ThreadLocks.LockForMessageChange(ctx, card.ThreadID); err != nil {
		return runs.InteractionResult{}, err
	}
	compRun, activeRun, err := s.lockInteractionRuns(ctx, row.RunID, card.ThreadID)
	if err != nil {
		return runs.InteractionResult{}, err
	}
	facts, err := s.d.Cards.LockCard(ctx, row.MessageID)
	if errs.KindOf(err) == errs.KindNotFound || (err == nil && facts.Deleted) {
		return runs.InteractionResult{}, invisible()
	}
	if err != nil {
		return runs.InteractionResult{}, err
	}
	cur, group, err := s.lockForInteraction(ctx, row)
	if err != nil {
		return runs.InteractionResult{}, err
	}
	if res, found, err := s.interactionReplay(ctx, a, cmd, values, card); err != nil || found {
		if err == nil {
			// The repetition is decided by message.read (design note 10 d); the proof of the HTTP
			// chain (02 9.3) still needs a check of the route's action, which filter mode records
			// without an audit entry and whose outcome the repetition does not use.
			err = s.proveInteraction(ctx, a, cur, facts, compRun, activeRun)
		}
		return res, err
	}
	if err := s.authorizeInteraction(ctx, a, cur, facts, compRun, activeRun); err != nil {
		return runs.InteractionResult{}, err
	}
	if err := domain.CheckValues(cur.Type, cur.Definition, values); err != nil {
		return runs.InteractionResult{}, err
	}
	now := s.now()
	if err := s.payloadFits(cur, card, values, now, a.ID); err != nil {
		return runs.InteractionResult{}, err
	}
	changed, err := s.applySingleUse(ctx, a, cur, group, now)
	if err != nil {
		return runs.InteractionResult{}, err
	}
	ix, err := s.d.Components.InsertInteraction(ctx, NewInteraction{
		ID: ids.New(), ComponentID: cur.ID, MessageID: cur.MessageID, PrincipalID: a.ID, ClientInteractionID: cmd.ClientInteractionID,
		Values: values, At: now,
	})
	if err != nil {
		return runs.InteractionResult{}, err
	}
	if changed {
		if _, err := s.d.Cards.ComponentsChanged(ctx, cur.MessageID); err != nil {
			return runs.InteractionResult{}, err
		}
	}
	runID, opened, mode, err := s.interactionRun(ctx, a, cur, facts, compRun, activeRun, ix.ID)
	if err != nil {
		return runs.InteractionResult{}, err
	}
	view := interactionPortView(ix, cur.CustomID, card.ThreadID, runID)
	if err := s.emitInteractionCreated(ctx, a, view, facts); err != nil {
		return runs.InteractionResult{}, err
	}
	if opened != nil {
		if err := s.emitInteractionInvocation(ctx, *opened, ix.ID, a.ID, mode); err != nil {
			return runs.InteractionResult{}, err
		}
	}
	if err := s.auditApproval(ctx, a, cur, ix.ID, runID); err != nil {
		return runs.InteractionResult{}, err
	}
	return runs.InteractionResult{Interaction: view}, nil
}

// lockInteractionRuns locks the component's run and the thread's active run in ascending id
// (design note 16) and returns both rows (zero rows when absent; a component run that is gone
// counts as none).
func (s *Service) lockInteractionRuns(ctx context.Context, compRunID, threadID ids.ID) (RunRow, RunRow, error) {
	active, hasActive, err := s.d.Repo.ActiveRunOfThread(ctx, threadID)
	if err != nil {
		return RunRow{}, RunRow{}, err
	}
	var list []ids.ID
	if !compRunID.IsZero() {
		list = append(list, compRunID)
	}
	if hasActive && active.ID != compRunID {
		list = append(list, active.ID)
	}
	slices.SortFunc(list, func(x, y ids.ID) int { return bytes.Compare(x[:], y[:]) })
	locked := map[ids.ID]RunRow{}
	for _, id := range list {
		r, found, err := s.d.Repo.LockRun(ctx, id)
		if err != nil {
			return RunRow{}, RunRow{}, err
		}
		if found {
			locked[id] = r
		}
	}
	var compRun, activeRun RunRow
	if r, ok := locked[compRunID]; ok && !compRunID.IsZero() {
		compRun = r
	}
	// The active run as locked: one that ended while this unit waited is no longer active.
	for _, r := range locked {
		if r.ThreadID == threadID && r.State.Active() {
			activeRun = r
		}
	}
	return compRun, activeRun, nil
}

// lockForInteraction locks the chosen component: for a single_use component the statement of 03
// (the active single_use components of the card in ascending id), else the component row. It
// returns the current row of the component and the locked group (empty for a component without
// single_use). Errors: not_found when the component is gone.
func (s *Service) lockForInteraction(ctx context.Context, row ComponentRow) (ComponentRow, []ComponentRow, error) {
	if !row.SingleUse {
		cur, found, err := s.d.Components.LockComponent(ctx, row.ID)
		if err != nil {
			return ComponentRow{}, nil, err
		}
		if !found {
			return ComponentRow{}, nil, invisible()
		}
		return cur, nil, nil
	}
	group, err := s.d.Components.LockSingleUseGroup(ctx, row.MessageID)
	if err != nil {
		return ComponentRow{}, nil, err
	}
	for _, c := range group {
		if c.ID == row.ID {
			return c, group, nil
		}
	}
	// Not active any more: the winner's rows are committed, the state decides (component_inactive).
	cur, found, err := s.d.Components.Component(ctx, row.ID)
	if err != nil {
		return ComponentRow{}, nil, err
	}
	if !found {
		return ComponentRow{}, nil, invisible()
	}
	return cur, group, nil
}

// authorizeInteraction decides interaction.create for a on the locked component cur (design note
// 8 b) and returns the mapped error of a denial; authz audits the denial (R-AZ-04).
func (s *Service) authorizeInteraction(ctx context.Context, a actor.Actor, cur ComponentRow, card messaging.CardFacts,
	compRun, activeRun RunRow) error {
	return s.authorizeErr(ctx, a, authorize.ActionInteractionCreate, interactionResource(cur, card, compRun, activeRun))
}

// proveInteraction decides interaction.create in filter mode and ignores the outcome: the proof
// of the check for a repetition (see createInteraction). Errors: those of the authorizer.
func (s *Service) proveInteraction(ctx context.Context, a actor.Actor, cur ComponentRow, card messaging.CardFacts,
	compRun, activeRun RunRow) error {
	_, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), a, authorize.ActionInteractionCreate,
		interactionResource(cur, card, compRun, activeRun))
	return err
}

// interactionResource is the descriptor of interaction.create for the locked component cur
// (design note 8 b): the component on the card's thread, the card's author as owner and as the
// agent of agent.invoke, the human origin, the agent of the thread's active run and the facts of
// the component and its run.
func interactionResource(cur ComponentRow, card messaging.CardFacts, compRun, activeRun RunRow) authorize.Resource {
	ix := &authorize.InteractionFacts{
		Stage: authorize.InteractionStageAccept, OwnerKind: card.AuthorKind, ComponentState: cur.State, ExpiresAt: cur.ExpiresAt,
		RequiredCapability: cur.RequiredCapability, RestrictedTo: cur.RestrictedTo, ComponentRunID: cur.RunID,
	}
	if !cur.RunID.IsZero() && compRun.ID == cur.RunID {
		ix.ComponentRunActive, ix.RunStarterID = compRun.State.Active(), compRun.TriggeredBy
	}
	return authorize.Resource{
		Kind: authorize.ResourceComponent, ID: cur.ID, ThreadID: card.ThreadID, OwnerID: card.AuthorID,
		Invoke:      &authorize.InvokeFacts{AgentID: card.AuthorID, TriggerOrigin: originHuman, ActiveRunAgentID: activeRun.AgentID},
		Interaction: ix,
	}
}

// payloadFits refuses values whose interaction.created payload would not fit
// sync.event_payload_max_bytes (design note 9; 04 2.2 rule 2: the appender rejects instead of
// cutting, and the agent has no other way to read the values): 400 too_long on /values. The IDs
// of the new interaction and run take their full length.
func (s *Service) payloadFits(cur ComponentRow, card messaging.TriggerMessage, values json.RawMessage, now time.Time, principal ids.ID) error {
	placeholder := ids.New()
	p, err := domain.InteractionCreatedPayload(domain.InteractionView{
		ID: placeholder, ComponentID: cur.ID, MessageID: cur.MessageID, PrincipalID: principal, ClientInteractionID: placeholder,
		CustomID: cur.CustomID, ThreadID: card.ThreadID, RunID: placeholder, Values: values, State: interactionReceived, CreatedAt: now,
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of interaction.created")
	}
	if len(p) >= s.cfg.EventPayloadMaxBytes {
		return fieldError("/values", "too_long", "the values do not fit sync.event_payload_max_bytes")
	}
	return nil
}

// applySingleUse writes the single-use rule (03 "Einmal-Regel je Nachricht", design note 10 g): the
// chosen single_use component becomes used with used_by and used_at, every other locked active row
// disabled, each with version + 1; a component without single_use stays unchanged (ruling
// D-M2-15-13). It reports whether a row changed.
func (s *Service) applySingleUse(ctx context.Context, a actor.Actor, cur ComponentRow, group []ComponentRow, now time.Time) (bool, error) {
	if !cur.SingleUse {
		return false, nil
	}
	for _, c := range group {
		w := ComponentWrite{ID: c.ID, State: domain.ComponentDisabled, Definition: c.Definition, UsedBy: c.UsedBy, UsedAt: c.UsedAt}
		if c.ID == cur.ID {
			w.State, w.UsedBy, w.UsedAt = domain.ComponentUsed, a.ID, now
		}
		if _, err := s.d.Components.SetComponent(ctx, w); err != nil {
			return false, err
		}
	}
	return len(group) > 0, nil
}

// interactionRun is the run of design note 10 h: the component's own active run, or, when the
// component has none and no run is active in the thread, a new run through Open with trigger_kind
// interaction. opened is the summary of a new run with the configured billing mode. Under the
// thread lock no other run can appear; should Open still meet one, the refusal of design note 8 b
// is answered.
func (s *Service) interactionRun(ctx context.Context, a actor.Actor, cur ComponentRow, card messaging.CardFacts, compRun, activeRun RunRow,
	interaction ids.ID) (ids.ID, *runs.RunSummary, string, error) {
	route := domain.RouteInteraction(domain.RouteFacts{
		CardAgentID: card.AuthorID, ComponentRunID: cur.RunID, ComponentRunActive: compRun.ID == cur.RunID && compRun.State.Active(),
		ActiveRunAgentID: activeRun.AgentID,
	})
	switch route.Kind {
	case domain.RouteComponentRun:
		return cur.RunID, nil, "", nil
	case domain.RouteOpenRun:
	case domain.RouteRefused:
		return ids.ID{}, nil, "", routeRefusal(route.Reason)
	default:
		return ids.ID{}, nil, "", errs.New(errs.CodeInternalError, "runs: an interaction without a route")
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{card.AuthorID})
	if err != nil {
		return ids.ID{}, nil, "", err
	}
	mode := facts[card.AuthorID].BillingMode
	if mode == "" {
		return ids.ID{}, nil, "", errs.New(errs.CodeInternalError, "runs: the agent of a card has no billing mode")
	}
	sum, err := s.Open(ctx, OpenCmd{
		ThreadID: card.ThreadID, AgentID: card.AuthorID, TriggeredBy: a.ID, TriggeringInteractionID: interaction,
		TriggerKind: triggerKindInteraction, BillingModeExpected: mode,
	})
	var exists *ActiveRunExistsError
	if errors.As(err, &exists) {
		if exists.Active.AgentID == card.AuthorID {
			return ids.ID{}, nil, "", routeRefusal(string(domain.ReasonRunActive))
		}
		return ids.ID{}, nil, "", routeRefusal(string(domain.ReasonRunActiveOtherAgent))
	}
	if err != nil {
		return ids.ID{}, nil, "", err
	}
	return sum.ID, &sum, mode, nil
}

// routeRefusal is the answer of a route that authorize should have refused already (design note
// 10 h): the mapping of authorize.Decision.Err for the reason.
func routeRefusal(reason string) error {
	return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: authorize.Reason(reason)}.Err()
}

// interactionPortView converts a stored interaction for the port; the response never comes back
// from Create.
func interactionPortView(ix InteractionRow, customID string, thread, run ids.ID) runs.Interaction {
	v := runs.Interaction{
		ID: ix.ID, ComponentID: ix.ComponentID, MessageID: ix.MessageID, CustomID: customID, ThreadID: thread, RunID: run,
		PrincipalID: ix.PrincipalID, ClientInteractionID: ix.ClientInteractionID, Values: json.RawMessage(ix.Values), State: ix.State,
		CreatedAt: ix.CreatedAt,
	}
	if len(v.Values) == 0 {
		v.Values = emptyObject
	}
	if ix.Response != nil {
		var resp struct {
			Text *string `json:"text"`
		}
		if json.Unmarshal(ix.Response, &resp) == nil {
			v.HasResponse, v.ResponseText = true, resp.Text
		}
	}
	return v
}

// interactionView returns the representation of a stored interaction: custom_id from its
// component, the thread of its card, run_id the component's run or the run it opened.
func (s *Service) interactionView(ctx context.Context, ix InteractionRow, thread ids.ID) (runs.Interaction, error) {
	comp, found, err := s.d.Components.Component(ctx, ix.ComponentID)
	if err != nil {
		return runs.Interaction{}, err
	}
	if !found {
		return runs.Interaction{}, invisible()
	}
	run := comp.RunID
	if run.IsZero() {
		if run, _, err = s.d.Components.RunOfInteraction(ctx, ix.ID); err != nil {
			return runs.Interaction{}, err
		}
	}
	return interactionPortView(ix, comp.CustomID, thread, run), nil
}

// emitInteractionCreated appends interaction.created for the card's agent (design note 19): scope
// the card's place, resource the interaction without version (03 interactions has none), audience
// the agent, receive action agent.events.consume, payload InteractionCreatedPayload with the values
// and without response, the person as actor.
func (s *Service) emitInteractionCreated(ctx context.Context, a actor.Actor, v runs.Interaction, card messaging.CardFacts) error {
	p, err := domain.InteractionCreatedPayload(domainInteractionView(v))
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of interaction.created")
	}
	kind, actorID := eventActor(actor.WithActor(ctx, a))
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventInteractionCreated), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: card.SpaceID, ChannelID: card.ChannelID, ThreadID: card.ThreadID, ResourceKind: resourceInteraction, ResourceID: v.ID,
		AudiencePrincipalID: card.AuthorID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}

// domainInteractionView converts the port view for the payload builder.
func domainInteractionView(v runs.Interaction) domain.InteractionView {
	return domain.InteractionView{
		ID: v.ID, ComponentID: v.ComponentID, MessageID: v.MessageID, PrincipalID: v.PrincipalID, ClientInteractionID: v.ClientInteractionID,
		CustomID: v.CustomID, ThreadID: v.ThreadID, RunID: v.RunID, Values: v.Values, State: v.State, ResponseText: v.ResponseText,
		HasResponse: v.HasResponse, CreatedAt: v.CreatedAt,
	}
}

// emitInteractionInvocation appends agent.invocation for a run an interaction opened, as the last
// event of the unit (M2-08 design note 10 f; design note 10 i): trigger_kind interaction, the
// interaction, the person as triggered_by, no message_id.
func (s *Service) emitInteractionInvocation(ctx context.Context, sum runs.RunSummary, interaction, person ids.ID, mode string) error {
	info, err := s.d.Threads.Info(ctx, sum.ThreadID)
	if err != nil {
		return err
	}
	ref, err := s.d.Cards.PrincipalOf(ctx, person)
	if err != nil {
		return err
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil {
		return err
	}
	p, err := domain.InvocationPayload(domain.InvocationPayloadInput{
		RunID: sum.ID, ThreadID: sum.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, InteractionID: interaction,
		TriggerKind: string(domain.TriggerInteraction),
		TriggeredBy: domain.PrincipalRefPayload{
			ID: ref.ID.String(), Kind: ref.Kind, Handle: ref.Handle, DisplayName: ref.DisplayName, AvatarFileID: optionalID(ref.AvatarFileID),
		},
		BillingMode: mode, SettingsVersion: ch.SettingsVersion,
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.invocation")
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventAgentInvocation), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: sum.ThreadID, ResourceKind: resourceRun, ResourceID: sum.ID,
		AudiencePrincipalID: sum.AgentID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}

// auditApproval writes the allow entry of an interaction on an approve_actions component in its
// unit of work (design note 17, 05 4.10): the IDs, the component type, capability and restriction,
// and choice only for a button whose value is once, session, always or deny; never custom_id, a
// label or a form value (SEC-AUD-5).
func (s *Service) auditApproval(ctx context.Context, a actor.Actor, cur ComponentRow, interaction, run ids.ID) error {
	if cur.RequiredCapability != domain.CapabilityApproveActions {
		return nil
	}
	if s.d.Audit == nil {
		return errs.New(errs.CodeInternalError, "runs: an approval without an audit recorder")
	}
	d := domain.InteractionAuditDetails(domain.AuditFacts{
		InteractionID: interaction, MessageID: cur.MessageID, RunID: run, ComponentType: cur.Type,
		RequiredCapability: cur.RequiredCapability, RestrictedTo: cur.RestrictedTo, Definition: cur.Definition,
	})
	attrs := auditing.Attrs{
		attrInteractionID: auditing.ID(d.InteractionID), attrMessageID: auditing.ID(d.MessageID),
		attrComponentType: auditing.Code(d.ComponentType), attrRequiredCapability: auditing.Code(d.RequiredCapability),
		attrRestrictedTo: auditing.Code(d.RestrictedTo),
	}
	if !d.RunID.IsZero() {
		attrs[attrRunID] = auditing.ID(d.RunID)
	}
	if d.Choice != "" {
		attrs[attrChoice] = auditing.Code(d.Choice)
	}
	return s.d.Audit.Record(actor.WithActor(ctx, a), auditing.Entry{
		Action: string(authorize.ActionInteractionCreate), Decision: auditing.DecisionAllow, ResourceKind: resourceComponent,
		ResourceID: cur.ID, Attrs: attrs,
	})
}

// Respond implements runs.Interactions (respondToInteraction, interaction.respond; M2-15 design
// note 12): response.text at most message.max_content_bytes (400 too_long; textcheck is not
// applied, the hint is content), authorize on the interaction path with the card's author as
// owner, then under the row lock of the interaction: from received or dispatched the final state
// and the response; the same final state again answers the current interaction without a write;
// every other transition is 409 invalid_state_transition. No event (05 6 has no type for it).
func (s *Service) Respond(ctx context.Context, cmd runs.RespondInteraction) (runs.Interaction, error) {
	if err := s.componentsReady(); err != nil {
		return runs.Interaction{}, err
	}
	switch cmd.State {
	case interactionAcknowledged, interactionFailed, interactionRejected:
	default:
		return runs.Interaction{}, fieldError("/state", "invalid_enum", "the state is not a final state of an interaction")
	}
	if cmd.ResponseText != nil && len(*cmd.ResponseText) > s.cfg.MaxContentBytes {
		return runs.Interaction{}, fieldError("/response/text", "too_long", "response.text is larger than message.max_content_bytes")
	}
	a := caller(ctx)
	ix, found, err := s.d.Components.Interaction(ctx, cmd.InteractionID)
	if err != nil {
		return runs.Interaction{}, err
	}
	if !found {
		return runs.Interaction{}, invisible()
	}
	card, err := s.d.Messages.TriggerMessageOf(ctx, ix.MessageID)
	if errs.KindOf(err) == errs.KindNotFound {
		return runs.Interaction{}, invisible()
	}
	if err != nil {
		return runs.Interaction{}, err
	}
	if err := s.authorizeErr(ctx, a, authorize.ActionInteractionRespond, authorize.Resource{
		Kind: authorize.ResourceInteraction, ID: ix.ID, ThreadID: card.ThreadID, OwnerID: card.Author.ID,
	}); err != nil {
		return runs.Interaction{}, err
	}
	var response []byte
	if cmd.HasResponse {
		if response, err = json.Marshal(struct {
			Text *string `json:"text,omitempty"`
		}{cmd.ResponseText}); err != nil {
			return runs.Interaction{}, errs.Wrap(errs.CodeInternalError, err, "runs: encoding a response")
		}
	}
	var out runs.Interaction
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Components.LockInteraction(ctx, ix.ID)
		if err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		switch {
		case cur.State == interactionReceived || cur.State == interactionDispatched:
			if cur, err = s.d.Components.SetInteractionState(ctx, cur.ID, cmd.State, response, s.now()); err != nil {
				return err
			}
		case cur.State == cmd.State:
		default:
			return errs.New(errs.CodeInvalidStateTransition, "an answered interaction keeps its state")
		}
		out, err = s.interactionView(ctx, cur, card.ThreadID)
		return err
	})
	return out, err
}
