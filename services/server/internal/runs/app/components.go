// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// CardMessages is the part of messaging.Messages the components of runs use (M2-15): the lock of
// a card message, the announcement of a component change through message.updated and the
// public reference of a principal for the payload of agent.invocation. None authorizes; runs
// decided.
type CardMessages interface {
	LockCard(ctx context.Context, messageID ids.ID) (messaging.CardFacts, error)
	ComponentsChanged(ctx context.Context, messageID ids.ID) (int64, error)
	CardFits(ctx context.Context, messageID ids.ID) error
	PrincipalOf(ctx context.Context, principalID ids.ID) (messaging.PrincipalRef, error)
}

// ThreadLocker locks the thread row before an interaction may open a run in it (chat.Threads,
// M1-58; the lock order of the README starts with threads).
type ThreadLocker interface {
	LockForMessageChange(ctx context.Context, threadID ids.ID) error
}

// ComponentExpiryScheduler inserts the job runs.component_expiry of a card message in the unit of
// work of ctx, to run at or after at (M2-15 design note 15 a); adapters/jobs implements it over
// jobs.Inserter.InsertTx.
type ComponentExpiryScheduler interface {
	ScheduleComponentExpiry(ctx context.Context, messageID ids.ID, at time.Time) error
}

// Compile-time check: Service is the incoming port of the components and interactions.
var _ runs.Interactions = (*Service)(nil)

// cardLimits returns the configuration keys of 09 21 for the domain checks of components.
func (s *Service) cardLimits() domain.CardLimits {
	return domain.CardLimits{
		MaxPerMessage: s.cfg.ComponentMaxPerMessage, CustomIDMaxChars: s.cfg.ComponentCustomIDMaxChars,
		FormMaxFields: s.cfg.ComponentFormMaxFields, DefaultTTL: s.cfg.ComponentDefaultTTL,
	}
}

// componentsReady refuses a component operation of a service built without the component
// collaborators.
func (s *Service) componentsReady() error {
	if s.d.Components == nil || s.d.Cards == nil || s.d.ThreadLocks == nil {
		return errs.New(errs.CodeInternalError, "runs: the components have no repository, card port or thread lock")
	}
	return nil
}

// CheckCard checks the components of a card request without database access (M2-15 design notes 4
// and 5): component.v1 and the server fields through domain.CheckCard. It is the Check of the
// source port messaging.ComponentWriter (adapters/messagingsource). Errors: 400 validation_failed
// with one errors[] entry per finding; internal for a service without components.
func (s *Service) CheckCard(_ context.Context, in messaging.CardInput) (messaging.CheckedCard, error) {
	if err := s.componentsReady(); err != nil {
		return messaging.CheckedCard{}, err
	}
	checked, canonical, err := domain.CheckCard(domainInputs(in.Components), in.Now, s.cardLimits())
	if err != nil {
		return messaging.CheckedCard{}, err
	}
	out := messaging.CheckedCard{Canonical: canonical, Input: in, Components: make([]messaging.Component, len(checked))}
	for i, c := range checked {
		out.Components[i] = messaging.Component{
			CustomID: c.CustomID, Type: c.Type, Definition: c.Definition, Position: c.Position, SingleUse: c.SingleUse,
			RequiredCapability: c.RequiredCapability, RestrictedTo: c.RestrictedTo, RunID: in.RunID, State: domain.ComponentActive,
			ExpiresAt: c.ExpiresAt, Version: 1,
		}
	}
	return out, nil
}

// domainInputs converts the components of a request.
func domainInputs(list []messaging.ComponentInput) []domain.ComponentInput {
	out := make([]domain.ComponentInput, len(list))
	for i, c := range list {
		out[i] = domain.ComponentInput{
			CustomID: c.CustomID, Type: c.Type, Definition: c.Definition, SingleUse: c.SingleUse,
			RequiredCapability: c.RequiredCapability, RestrictedTo: c.RestrictedTo, ExpiresAt: c.ExpiresAt,
		}
	}
	return out
}

// AttachCard stores the components of a new card message in the ambient unit of work (M2-15
// design note 6 b), after messaging inserted the message row: the request is checked again (the
// checked form is not trusted across the port), the run of run_id is share-locked and must be an
// active run of the author in the message's thread (the answers of M2-19, so
// nothing about other agents' runs leaks), the owner's switch supports_components = false gives
// 403 feature_disabled, then the rows are inserted and runs.component_expiry is scheduled at the
// earliest expires_at. It is the Attach of messaging.ComponentWriter. messages.run_id stays NULL:
// the run of a card lives in message_components.run_id (ruling D-M2-15-6).
//
// Errors: internal outside a unit of work or for an author that is no agent; 400 and 403 as above;
// the errors of the database and the scheduler.
func (s *Service) AttachCard(ctx context.Context, messageID ids.ID, card messaging.CheckedCard) ([]messaging.Component, error) {
	if err := needUnit(ctx, "AttachCard"); err != nil {
		return nil, err
	}
	if err := s.componentsReady(); err != nil {
		return nil, err
	}
	in := card.Input
	if in.AuthorKind != string(actor.KindAgent) || in.AuthorID.IsZero() || in.ThreadID.IsZero() {
		return nil, errs.New(errs.CodeInternalError, "runs: a card of a principal that is no agent, or without its thread")
	}
	checked, _, err := domain.CheckCard(domainInputs(in.Components), in.Now, s.cardLimits())
	if err != nil {
		return nil, err
	}
	if err := s.checkCardRun(ctx, in); err != nil {
		return nil, err
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{in.AuthorID})
	if err != nil {
		return nil, err
	}
	if !facts[in.AuthorID].ComponentsAllowed {
		// The owner switched the capability off (09 2.2, R-AG-01; ruling D-M2-15-5).
		return nil, errs.New(errs.CodeFeatureDisabled, "the owner switched components off for this agent")
	}
	rows, earliest := componentRows(messageID, in.RunID, checked)
	if err := s.d.Components.InsertComponents(ctx, rows); err != nil {
		return nil, err
	}
	if err := s.scheduleComponentExpiry(ctx, messageID, earliest); err != nil {
		return nil, err
	}
	out := make([]messaging.Component, len(rows))
	for i, r := range rows {
		out[i] = messagingComponent(r)
	}
	return out, nil
}

// checkCardRun share-locks the run of a card's run_id and checks that it is an active run of the
// author in the message's thread with the verdict and the answers of M2-19 (400 run_mismatch on
// /run_id for an unknown run, a run of another thread or agent; 409 run_not_active for an ended
// own run; ruling D-M2-15-29). A card without run_id passes.
func (s *Service) checkCardRun(ctx context.Context, in messaging.CardInput) error {
	if in.RunID.IsZero() {
		return nil
	}
	run, found, err := s.d.Components.LockRunShared(ctx, in.RunID)
	if err != nil {
		return err
	}
	return answerError(domain.AnswerVerdict(domain.AnswerRun{AgentID: run.AgentID, ThreadID: run.ThreadID, State: run.State}, found,
		in.AuthorID, in.ThreadID))
}

// componentRows returns the rows of the checked components of a new card (design note 5: state
// active, version 1, the card's run) and the earliest expires_at.
func componentRows(messageID, run ids.ID, checked []domain.CheckedComponent) ([]ComponentRow, time.Time) {
	rows := make([]ComponentRow, len(checked))
	var earliest time.Time
	for i, c := range checked {
		rows[i] = ComponentRow{
			ID: ids.New(), MessageID: messageID, CustomID: c.CustomID, Type: c.Type, Definition: c.Definition, Position: c.Position,
			SingleUse: c.SingleUse, RequiredCapability: c.RequiredCapability, RestrictedTo: c.RestrictedTo, RunID: run,
			State: domain.ComponentActive, ExpiresAt: c.ExpiresAt, Version: 1,
		}
		if earliest.IsZero() || c.ExpiresAt.Before(earliest) {
			earliest = c.ExpiresAt
		}
	}
	return rows, earliest
}

// scheduleComponentExpiry inserts runs.component_expiry for the card at at; without a scheduler
// nothing is inserted (authorize still refuses a passed expires_at).
func (s *Service) scheduleComponentExpiry(ctx context.Context, messageID ids.ID, at time.Time) error {
	if s.d.ComponentExpiry == nil || at.IsZero() {
		return nil
	}
	return s.d.ComponentExpiry.ScheduleComponentExpiry(ctx, messageID, at)
}

// ForMessages returns the components of the given card messages ordered by position (M2-15 design
// note 7), in one statement and in the ambient transaction when there is one. It is the
// messaging.ComponentReader of the source port. A service without components returns none.
func (s *Service) ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]messaging.Component, error) {
	out := map[ids.ID][]messaging.Component{}
	if s.d.Components == nil || len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := s.d.Components.ComponentsOfMessages(ctx, messageIDs)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.MessageID] = append(out[r.MessageID], messagingComponent(r))
	}
	return out, nil
}

// messagingComponent converts a row for the representation of messaging.
func messagingComponent(r ComponentRow) messaging.Component {
	return messaging.Component{
		ID: r.ID, CustomID: r.CustomID, Type: r.Type, Definition: json.RawMessage(r.Definition), Position: r.Position,
		SingleUse: r.SingleUse, RequiredCapability: r.RequiredCapability, RestrictedTo: r.RestrictedTo, RunID: r.RunID,
		State: r.State, ExpiresAt: r.ExpiresAt, UsedBy: r.UsedBy, UsedAt: r.UsedAt, Version: r.Version,
	}
}

// portComponent converts a row for the port runs.Interactions.
func portComponent(r ComponentRow) runs.Component {
	return runs.Component{
		ID: r.ID, MessageID: r.MessageID, CustomID: r.CustomID, Type: r.Type, Definition: json.RawMessage(r.Definition),
		Position: r.Position, SingleUse: r.SingleUse, RequiredCapability: r.RequiredCapability, RestrictedTo: r.RestrictedTo,
		RunID: r.RunID, State: r.State, ExpiresAt: r.ExpiresAt, UsedBy: r.UsedBy, UsedAt: r.UsedAt, Version: r.Version,
	}
}

// cardOf reads the component id and the facts of its card for the check before the unit of work:
// an unknown component is not found.
func (s *Service) cardOf(ctx context.Context, id ids.ID) (ComponentRow, messaging.TriggerMessage, bool, error) {
	row, found, err := s.d.Components.Component(ctx, id)
	if err != nil || !found {
		return ComponentRow{}, messaging.TriggerMessage{}, false, err
	}
	m, err := s.d.Messages.TriggerMessageOf(ctx, row.MessageID)
	if errs.KindOf(err) == errs.KindNotFound {
		return ComponentRow{}, messaging.TriggerMessage{}, false, nil
	}
	if err != nil {
		return ComponentRow{}, messaging.TriggerMessage{}, false, err
	}
	return row, m, true, nil
}

// UpdateComponent implements runs.Interactions (updateComponent, component.update; M2-15 design
// note 13): authorize on the component path with the card's author as owner (agent-self, app-self),
// then in one unit of work the card row, for a state change of a single_use component the whole
// single-use set of the card, else the component row; If-Match on the component version (412
// version_conflict with the current component); the transitions of domain.StateTransition; the
// definition checked as component.v1 of the component's type. No change answers the current
// component without a write (06 2 "No effect"); a change raises the component version and, through
// messaging, the message version with message.updated; a reactivation schedules the expiry job.
// 428 for a missing If-Match is the HTTP chain's.
func (s *Service) UpdateComponent(ctx context.Context, cmd runs.UpdateComponent) (runs.Component, error) {
	if err := s.componentsReady(); err != nil {
		return runs.Component{}, err
	}
	a := caller(ctx)
	row, card, found, err := s.cardOf(ctx, cmd.ComponentID)
	if err != nil {
		return runs.Component{}, err
	}
	r := authorize.Resource{Kind: authorize.ResourceComponent, ID: cmd.ComponentID, ThreadID: card.ThreadID, OwnerID: card.Author.ID}
	if !found {
		// An unknown component reads like an invisible one (02 9.2).
		return runs.Component{}, invisible()
	}
	if err := s.authorizeErr(ctx, a, authorize.ActionComponentUpdate, r); err != nil {
		return runs.Component{}, err
	}
	var def json.RawMessage
	if cmd.Definition != nil {
		if def, err = domain.CheckDefinition(row.Type, cmd.Definition, s.cardLimits()); err != nil {
			return runs.Component{}, err
		}
	}
	var out runs.Component
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.updateComponent(ctx, row, cmd, def)
		return err
	})
	return out, err
}

// updateComponent is the unit of work of UpdateComponent.
func (s *Service) updateComponent(ctx context.Context, row ComponentRow, cmd runs.UpdateComponent, def json.RawMessage) (runs.Component, error) {
	facts, err := s.d.Cards.LockCard(ctx, row.MessageID)
	if errs.KindOf(err) == errs.KindNotFound || (err == nil && facts.Deleted) {
		return runs.Component{}, invisible()
	}
	if err != nil {
		return runs.Component{}, err
	}
	cur, set, err := s.lockForUpdate(ctx, row, cmd.State != nil)
	if err != nil {
		return runs.Component{}, err
	}
	if cur.Version != cmd.IfMatch {
		return runs.Component{}, errs.VersionConflict(componentBody(portComponent(cur)))
	}
	w, changed, err := componentChange(cur, set, cmd.State, def, s.now())
	if err != nil || !changed {
		return portComponent(cur), err
	}
	updated, err := s.writeComponent(ctx, row.MessageID, cur, w, def != nil)
	if err != nil {
		return runs.Component{}, err
	}
	return portComponent(updated), nil
}

// writeComponent stores the change w of the locked component cur of the card message: with a new
// definition the card must keep its size reserve (D-M2-15-26; the unit of work rolls the write
// back), then the message version rises with message.updated, and a reactivated component is
// scheduled to expire again (design note 15 a).
func (s *Service) writeComponent(ctx context.Context, message ids.ID, cur ComponentRow, w ComponentWrite, newDefinition bool) (ComponentRow, error) {
	updated, err := s.d.Components.SetComponent(ctx, w)
	if err != nil {
		return ComponentRow{}, err
	}
	if newDefinition {
		if err := s.d.Cards.CardFits(ctx, message); err != nil {
			return ComponentRow{}, err
		}
	}
	if _, err := s.d.Cards.ComponentsChanged(ctx, message); err != nil {
		return ComponentRow{}, err
	}
	if w.State == domain.ComponentActive && cur.State != domain.ComponentActive {
		if err := s.scheduleComponentExpiry(ctx, message, updated.ExpiresAt); err != nil {
			return ComponentRow{}, err
		}
	}
	return updated, nil
}

// componentChange decides the write of updateComponent for the locked component cur and the
// locked single-use set (design note 13 c and d): the state through domain.StateTransition (a
// single_use component may not come back while another one of its card is used), the definition
// when it differs as JSON. changed is false for no change. Errors: 409 invalid_state_transition.
func componentChange(cur ComponentRow, set []ComponentRow, state *string, def json.RawMessage, now time.Time) (ComponentWrite, bool, error) {
	to := ""
	if state != nil {
		to = *state
	}
	groupUsed := slices.ContainsFunc(set, func(c ComponentRow) bool { return c.ID != cur.ID && c.State == domain.ComponentUsed })
	stateChanged, err := domain.StateTransition(cur.State, cur.ExpiresAt, now, to, cur.SingleUse, groupUsed)
	if err != nil {
		return ComponentWrite{}, false, err
	}
	defChanged := def != nil && !domain.SameDefinition(def, cur.Definition)
	w := ComponentWrite{ID: cur.ID, State: cur.State, Definition: cur.Definition, UsedBy: cur.UsedBy, UsedAt: cur.UsedAt}
	if stateChanged {
		w.State = to
	}
	if defChanged {
		w.Definition = def
	}
	return w, stateChanged || defChanged, nil
}

// lockForUpdate locks the component of row for updateComponent: for a state change of a single_use
// component the whole single-use set of its card in ascending id (the single-use rule must not
// allow a second decision, 09 11.3), otherwise the component row. It returns the locked component
// and the locked set (empty without one). Errors: not_found when the component is gone.
func (s *Service) lockForUpdate(ctx context.Context, row ComponentRow, stateChange bool) (ComponentRow, []ComponentRow, error) {
	if stateChange && row.SingleUse {
		set, err := s.d.Components.LockSingleUseSet(ctx, row.MessageID)
		if err != nil {
			return ComponentRow{}, nil, err
		}
		for _, c := range set {
			if c.ID == row.ID {
				return c, set, nil
			}
		}
		return ComponentRow{}, nil, invisible()
	}
	cur, found, err := s.d.Components.LockComponent(ctx, row.ID)
	if err != nil {
		return ComponentRow{}, nil, err
	}
	if !found {
		return ComponentRow{}, nil, invisible()
	}
	return cur, nil, nil
}

// authorizeErr authorizes act on r for a and returns the decision as an error (nil for allow).
func (s *Service) authorizeErr(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// componentJSON is the schema Component in the current member of a version_conflict problem: optional
// members left out, never null (ADR 0053).
type componentJSON struct {
	ID                 string          `json:"id"`
	CustomID           string          `json:"custom_id"`
	Type               string          `json:"type"`
	Definition         json.RawMessage `json:"definition"`
	Position           int             `json:"position"`
	SingleUse          bool            `json:"single_use"`
	RequiredCapability string          `json:"required_capability,omitempty"`
	RestrictedTo       string          `json:"restricted_to"`
	RunID              string          `json:"run_id,omitempty"`
	State              string          `json:"state"`
	ExpiresAt          string          `json:"expires_at,omitempty"`
	UsedBy             string          `json:"used_by,omitempty"`
	UsedAt             string          `json:"used_at,omitempty"`
	Version            int64           `json:"version"`
}

// componentBody converts a component into the JSON form of the schema Component.
func componentBody(c runs.Component) componentJSON {
	def := c.Definition
	if len(def) == 0 {
		def = json.RawMessage("{}")
	}
	return componentJSON{
		ID: c.ID.String(), CustomID: c.CustomID, Type: c.Type, Definition: def, Position: c.Position, SingleUse: c.SingleUse,
		RequiredCapability: c.RequiredCapability, RestrictedTo: c.RestrictedTo, RunID: optionalID(c.RunID), State: c.State,
		ExpiresAt: stampOf(c.ExpiresAt), UsedBy: optionalID(c.UsedBy), UsedAt: stampOf(c.UsedAt), Version: c.Version,
	}
}

// stampOf formats t as an API time (06 section 2: UTC, milliseconds, suffix Z); the zero time is
// absent.
func stampOf(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z")
}
