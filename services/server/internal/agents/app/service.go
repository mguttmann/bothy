// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Config holds the configuration of the agents use cases, bound in internal/wire
// (wire.AgentsConfig).
type Config struct {
	// PageSizeDefault is api.page_size_default, the page size of listAgents without limit.
	PageSizeDefault int `config:"api.page_size_default"`
	// PageSizeMax is api.page_size_max; larger limits are capped.
	PageSizeMax int `config:"api.page_size_max"`
	// ReasonMaxChars is audit.reason_max_chars, the longest reason of agent.billing_mode.set,
	// agent.billing_mode.set_all and agent.credential_holder.set (SEC-AUD-5, M2-21).
	ReasonMaxChars int `config:"audit.reason_max_chars"`
}

// Deps are the collaborators of the agents use cases.
type Deps struct {
	// Store reads and writes the agents records.
	Store Store
	// Principals creates the agent principal and changes its status and display name.
	Principals identity.PrincipalWriter
	// Directory reads principals and checks credential holders.
	Directory identity.Directory
	// Tokens revokes the API tokens of a disabled agent.
	Tokens TokenRevoker
	// Rights names agents as affected inside a rights change (the rights hook of authz).
	Rights authz.RightsChanges
	// UoW runs every write; its registry holds the rights hooks of authz.
	UoW txn.UnitOfWork
	// Authorizer decides every call before any write.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries inside the unit of work.
	Audit auditing.Recorder
	// Events appends agent.updated.
	Events eventlog.Appender
	// Cursors encodes and checks the cursors of listAgents.
	Cursors Cursors
	// Clock gives the time of a policy switch; nil means clock.System().
	Clock clock.Clock
	// Presence is the presence tracker of the process (M2-20): getAgent and listAgents show its
	// presence, and the changes of the configured values notify it after the commit. nil shows the
	// stored columns and notifies nothing.
	Presence *Tracker
	// Measure is the own measurement (kernel/measure, M6-13): billing_mode_switches after the
	// commit; nil records nothing.
	Measure measure.Recorder
}

// Service implements agents.Agents. It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
}

// Compile-time check: the incoming port.
var _ agents.Agents = (*Service)(nil)

// New returns the use cases. Errors: a missing collaborator, a page size below 1 or an
// audit.reason_max_chars below 1.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Store == nil || d.Principals == nil || d.Directory == nil || d.Tokens == nil || d.Rights == nil || d.UoW == nil ||
		d.Authorizer == nil || d.Audit == nil || d.Events == nil || d.Cursors == nil {
		return nil, errors.New("agents: New needs every collaborator")
	}
	if cfg.PageSizeDefault < 1 || cfg.PageSizeMax < cfg.PageSizeDefault {
		return nil, errors.New("agents: page sizes must be at least 1, the default not above the maximum")
	}
	if cfg.ReasonMaxChars < 1 {
		return nil, errors.New("agents: audit.reason_max_chars must be at least 1")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	d.Measure = measure.OrNop(d.Measure)
	return &Service{cfg: cfg, d: d}, nil
}

// Values of the audit entries and of agent.updated (05 sections 4.10 and 6, 10 section 14).
const (
	resourceAgent    = "agent"
	resourceInstance = "instance"
	eventUpdated     = "agent.updated"
	receiveRead      = "agent.read"

	attrReason        = "reason"
	attrNoChange      = "no_change"
	attrChangedFields = "changed_fields"
	attrAdapterKind   = "adapter_kind"
	attrFromStatus    = "from_status"
	attrRevokedTokens = "revoked_tokens"
	attrFromMode      = "from_billing_mode"
	attrToMode        = "to_billing_mode"
	attrFromHolder    = "from_credential_holder_id"
	attrToHolder      = "to_credential_holder_id"
	attrFromToolClass = "from_tool_class"
	attrToToolClass   = "to_tool_class"
	attrAgentIDs      = "agent_ids"
	attrFromModes     = "from_billing_modes"
	attrHeldAgentIDs  = "held_agent_ids"
	attrFromHolders   = "from_credential_holder_ids"
)

// withCaller puts caller into ctx as the principal of the request, which the recorder and the
// events read; a different principal already in ctx is a programming error.
func withCaller(ctx context.Context, caller actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != caller.ID || have.Kind != caller.Kind) {
		return nil, errs.New(errs.CodeInternalError, "agents: the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, caller), nil
}

// agentResource describes the agent id: kind agent, owned by itself, so agent-self holds for it.
func agentResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceAgent, ID: id, OwnerID: id}
}

// check authorizes act on r and returns the decision as an error (nil for allow).
func (s *Service) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// invisible is the not_found of a missing agent, the same body as an invisible one.
func invisible() error { return errs.New(errs.CodeNotFound, "") }

// load reads the record id and authorizes act on it before it answers a missing agent: a caller
// that authorize refuses gets its refusal for an existing and an unknown id alike (02 section 9.2).
func (s *Service) load(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) (domain.Agent, error) {
	rec, found, err := s.d.Store.Agent(ctx, id)
	if err != nil {
		return domain.Agent{}, err
	}
	if err := s.check(ctx, a, act, agentResource(id)); err != nil {
		return domain.Agent{}, err
	}
	if !found {
		return domain.Agent{}, invisible()
	}
	return rec, nil
}

// lock locks the record id inside the unit of work.
func (s *Service) lock(ctx context.Context, id ids.ID) (domain.Agent, error) {
	rec, found, err := s.d.Store.LockAgent(ctx, id)
	if err != nil {
		return domain.Agent{}, err
	}
	if !found {
		return domain.Agent{}, invisible()
	}
	return rec, nil
}

// view joins the record with its principal into the port DTO.
func (s *Service) view(ctx context.Context, rec domain.Agent) (agents.Agent, error) {
	p, err := s.d.Directory.Principal(ctx, rec.ID)
	if err != nil {
		return agents.Agent{}, err
	}
	return s.agentView(rec, p), nil
}

// presence returns the presence of rec: from the tracker when one is bound (M2-20 ends the
// placeholder of M1-63/3), otherwise from the stored columns.
func (s *Service) presence(rec domain.Agent) agents.PresenceStatus {
	if s.d.Presence != nil {
		return s.d.Presence.recordStatus(rec)
	}
	return recordPresence(rec, s.d.Clock.Now(), 0)
}

// agentView converts a record and its principal.
func (s *Service) agentView(rec domain.Agent, p identity.PrincipalView) agents.Agent {
	presence := s.presence(rec)
	// The tracker learns a desired mode changed in another unit of work (the guided flow of
	// M2-21) only with the next heartbeat, so the comparison takes the desired mode of this read
	// (M2-21 design note 8, docs/DECISIONS.md D-M2-21-7).
	presence.BillingModeMatches = presence.ReportedBillingMode == "" || presence.ReportedBillingMode == string(rec.Policy.Mode)
	return agents.Agent{
		ID: rec.ID, Handle: p.Handle, DisplayName: p.DisplayName, AvatarFileID: p.AvatarFileID, Status: string(p.Status),
		AdapterKind: rec.AdapterKind, Description: rec.Description, BillingMode: string(rec.Policy.Mode),
		CredentialHolderID: rec.Policy.HolderID, BillingModeChangedAt: rec.BillingModeChangedAt, ToolClass: string(rec.ToolClass),
		Presence: presence, OperatorConfig: rec.Config.Map(), Version: rec.Version,
	}
}

// precondition checks the If-Match of a request against the current record (06 section 8): no
// header is 428 precondition_required, a stale or malformed one 412 version_conflict with the
// current agent.
func (s *Service) precondition(ctx context.Context, p agents.Precondition, rec domain.Agent) error {
	if !p.Present {
		return errs.PreconditionRequired()
	}
	if p.Valid && p.Version == rec.Version {
		return nil
	}
	v, err := s.view(ctx, rec)
	if err != nil {
		return err
	}
	return errs.VersionConflict(agentBody(v))
}

// record writes the allow entry of act on the agent id inside the unit of work.
func (s *Service) record(ctx context.Context, act authorize.Action, id ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceAgent, ResourceID: id, Attrs: attrs,
	})
}

// noChange records a call without effect (06 section 2 "No effect") with the reason when given.
func (s *Service) noChange(ctx context.Context, act authorize.Action, id ids.ID, reason string) error {
	attrs := auditing.Attrs{attrNoChange: auditing.Bool(true)}
	if reason != "" {
		attrs[attrReason] = auditing.ReasonText(reason)
	}
	return s.record(ctx, act, id, attrs)
}

// presenceJSON is the schema AgentRuntimeStatus; optional members are left out (ADR 0053).
type presenceJSON struct {
	State                 string   `json:"state"`
	ReportedBillingMode   string   `json:"reported_billing_mode,omitempty"`
	RuntimeVersion        string   `json:"runtime_version,omitempty"`
	LastHeartbeatAt       string   `json:"last_heartbeat_at,omitempty"`
	BillingModesSupported []string `json:"billing_modes_supported,omitempty"`
	ReportedToolClass     string   `json:"reported_tool_class,omitempty"`
	BillingModeMatches    bool     `json:"billing_mode_matches"`
	// BillingEvidence and Warnings are the members of M2-20 (D-M2-20-12).
	BillingEvidence *evidenceJSON `json:"billing_evidence,omitempty"`
	Warnings        []string      `json:"warnings,omitempty"`
}

// evidenceJSON is the schema BillingEvidence.
type evidenceJSON struct {
	Provider string `json:"provider,omitempty"`
	KeyType  string `json:"key_type"`
	Method   string `json:"method,omitempty"`
}

// presenceBody converts a presence into the JSON form of AgentRuntimeStatus.
func presenceBody(p agents.PresenceStatus) presenceJSON {
	out := presenceJSON{
		State: p.State, ReportedBillingMode: p.ReportedBillingMode, RuntimeVersion: p.RuntimeVersion,
		LastHeartbeatAt: stamp(p.LastHeartbeatAt), BillingModesSupported: p.BillingModesSupported,
		ReportedToolClass: p.ReportedToolClass, BillingModeMatches: p.BillingModeMatches, Warnings: p.Warnings,
	}
	if e := p.BillingEvidence; e != nil {
		out.BillingEvidence = &evidenceJSON{Provider: e.Provider, KeyType: e.KeyType, Method: e.Method}
	}
	return out
}

// agentJSON is the schema Agent of docs/api/openapi.yaml: the payload member agent of
// agent.updated and the member current of 412 version_conflict.
type agentJSON struct {
	ID                   string         `json:"id"`
	Handle               string         `json:"handle"`
	DisplayName          string         `json:"display_name"`
	AvatarFileID         string         `json:"avatar_file_id,omitempty"`
	Status               string         `json:"status"`
	AdapterKind          string         `json:"adapter_kind"`
	Description          string         `json:"description"`
	BillingMode          string         `json:"billing_mode"`
	CredentialHolderID   string         `json:"credential_holder_id,omitempty"`
	BillingModeChangedAt string         `json:"billing_mode_changed_at,omitempty"`
	ToolClass            string         `json:"tool_class"`
	Presence             presenceJSON   `json:"presence"`
	OperatorConfig       map[string]any `json:"operator_config"`
	Version              int64          `json:"version"`
}

// timestampLayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// stamp formats t; the zero time is absent.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(timestampLayout)
}

// optionalID formats id; the zero ID is absent.
func optionalID(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// agentBody converts the DTO into the JSON form of the schema Agent.
func agentBody(v agents.Agent) agentJSON {
	config := v.OperatorConfig
	if config == nil {
		config = map[string]any{}
	}
	return agentJSON{
		ID: v.ID.String(), Handle: v.Handle, DisplayName: v.DisplayName, AvatarFileID: optionalID(v.AvatarFileID), Status: v.Status,
		AdapterKind: v.AdapterKind, Description: v.Description, BillingMode: v.BillingMode,
		CredentialHolderID: optionalID(v.CredentialHolderID), BillingModeChangedAt: stamp(v.BillingModeChangedAt), ToolClass: v.ToolClass,
		Presence:       presenceBody(v.Presence),
		OperatorConfig: config, Version: v.Version,
	}
}

// emitUpdated appends agent.updated for v (05 section 6): resource agent with its version, empty
// scope, the actor of the request, payload AgentPayload with the agent after the change, receive
// action agent.read, so it reaches owners, readers of a served channel and the agent itself.
func (s *Service) emitUpdated(ctx context.Context, v agents.Agent) error {
	payload, err := json.Marshal(struct {
		Agent agentJSON `json:"agent"`
	}{agentBody(v)})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "agents: agent.updated payload")
	}
	kind, id := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: eventUpdated, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: id,
		ResourceKind: resourceAgent, ResourceID: v.ID, ResourceVersion: v.Version, ReceiveAction: receiveRead, Payload: payload,
	})
}

// eventActor returns the actor of the request in ctx; none outside a request.
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() {
		return "", ids.ID{}
	}
	switch a.Kind {
	case actor.KindUser:
		return eventlog.ActorUser, a.ID
	case actor.KindAgent:
		return eventlog.ActorAgent, a.ID
	case actor.KindApp:
		return eventlog.ActorApp, a.ID
	case actor.KindHost, actor.KindAnonymous:
	}
	return "", ids.ID{}
}

// changedAndEmit stores rec, records the allow entry of act with attrs and appends agent.updated.
func (s *Service) changedAndEmit(ctx context.Context, act authorize.Action, rec domain.Agent, attrs auditing.Attrs) (agents.Agent, error) {
	if err := s.d.Store.UpdateAgent(ctx, rec); err != nil {
		return agents.Agent{}, err
	}
	return s.recordAndEmit(ctx, act, rec, attrs)
}

// recordAndEmit records the allow entry of act and appends agent.updated for the stored rec.
func (s *Service) recordAndEmit(ctx context.Context, act authorize.Action, rec domain.Agent, attrs auditing.Attrs) (agents.Agent, error) {
	v, err := s.view(ctx, rec)
	if err != nil {
		return agents.Agent{}, err
	}
	if err := s.record(ctx, act, rec.ID, attrs); err != nil {
		return agents.Agent{}, err
	}
	return v, s.emitUpdated(ctx, v)
}

// rightsChange declares the rights change act of the agents list (05 rule 1.7, 10 section 5.1):
// the rights hook raises permission_version, records the views of the agents and checks I1.
func rightsChange(act authorize.Action, res txn.Ref, list ...ids.ID) txn.Option {
	return txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(act), Resource: res, Principals: list})
}

// agentRef is the deny resource of a change of one agent.
func agentRef(id ids.ID) txn.Ref { return txn.Ref{Kind: resourceAgent, ID: id} }

// codes converts field names into audit codes.
func codes(names []string) auditing.Value {
	vs := make([]auditing.Value, 0, len(names))
	for _, n := range names {
		vs = append(vs, auditing.Code(n))
	}
	return auditing.List(vs...)
}

// presenceChanged tells the tracker after a committed change of the configured values or the
// status of the agent out (M2-20 note 13) and shows the reassessed presence in out. A failure
// only logs: the change is committed, and the next heartbeat reads the values again.
func (s *Service) presenceChanged(ctx context.Context, out *agents.Agent) {
	if s.d.Presence == nil {
		return
	}
	s.notifyPresence(ctx, []ids.ID{out.ID})
	if st, ok := s.d.Presence.memory(out.ID, out.Presence.LastHeartbeatAt, s.d.Clock.Now()); ok {
		out.Presence = st
	}
}

// notifyPresence tells the tracker about the committed change of the agents list.
func (s *Service) notifyPresence(ctx context.Context, list []ids.ID) {
	if s.d.Presence == nil || len(list) == 0 {
		return
	}
	if err := s.d.Presence.Changed(ctx, list); err != nil {
		s.d.Presence.d.Log.LogAttrs(ctx, slog.LevelError, "agent presence not reassessed")
	}
}

// BindPresence binds the presence tracker of the process (internal/wire, before the routes serve):
// from then on getAgent and listAgents show its presence and the changes of the configured values
// notify it.
func (s *Service) BindPresence(t *Tracker) { s.d.Presence = t }
