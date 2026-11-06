// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"math"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the incoming port of the definitions.
var _ agents.Definitions = (*Service)(nil)

// The values of the definitions in audit entries, events and refusals (26 4.1, 4.4 item 5).
const (
	attrDefinitionVersion = "definition_version"
	attrChangedParts      = "changed_parts"
	receiveDefinitionRead = "agent.definition.read"
	reasonNotBuiltin      = "not_a_builtin_agent"
	reasonNeedsHost       = "definition_needs_host"
	definitionVersions    = "agent_definition_versions"
)

// notBuiltin is the refusal of a definition for an agent that is no built-in agent (26 4.1 item
// 4).
func notBuiltin() error {
	return errs.New(errs.CodeConflict, "only a built-in agent has a definition").WithReason(reasonNotBuiltin)
}

// PutDefinition implements agents.Definitions in one unit of work (agent.update is no rights
// change): it authorizes before the row lock, so a refused caller never waits on it, locks the
// agents row first (03 section 6.3), checks If-Match against the current definition version and
// the body against 26 4.2 and 4.3 for the agent's tool class, then inserts the next version,
// raises agents.version with agent.updated, audits the version and the changed parts and appends
// agent.definition_changed to the agent.
func (s *Service) PutDefinition(ctx context.Context, a actor.Actor, id ids.ID, cmd agents.PutDefinitionCmd) (agents.DefinitionVersion, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.DefinitionVersion{}, err
	}
	var out agents.DefinitionVersion
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.putDefinition(ctx, a, id, cmd)
		return err
	})
	return out, err
}

// putDefinition is PutDefinition inside its unit of work.
func (s *Service) putDefinition(ctx context.Context, a actor.Actor, id ids.ID, cmd agents.PutDefinitionCmd) (agents.DefinitionVersion, error) {
	if _, err := s.load(ctx, a, authorize.ActionAgentUpdate, id); err != nil {
		return agents.DefinitionVersion{}, err
	}
	rec, err := s.lock(ctx, id)
	if err != nil {
		return agents.DefinitionVersion{}, err
	}
	if rec.AdapterKind != agents.AdapterKindBuiltin {
		return agents.DefinitionVersion{}, notBuiltin()
	}
	cur, found, err := s.d.Definitions.CurrentDefinition(ctx, id)
	if err != nil {
		return agents.DefinitionVersion{}, err
	}
	if err := definitionPrecondition(cmd.IfMatch, cur, found); err != nil {
		return agents.DefinitionVersion{}, err
	}
	def, findings := definition.Parse(cmd.Definition, s.cfg.DefinitionLimits(), definition.ToolClass(rec.ToolClass))
	if len(findings) > 0 {
		return agents.DefinitionVersion{}, refusal(findings)
	}
	var old definition.Definition
	if found {
		if old, err = definition.Decode(cur.Definition); err != nil {
			return agents.DefinitionVersion{}, errs.Wrap(errs.CodeInternalError, err, "agents: a stored definition outside its schema")
		}
	}
	parts := definition.AllParts()
	if found {
		parts = definition.Parts(old, def)
	}
	if found && len(parts) == 0 {
		return versionView(cur), s.noChange(ctx, authorize.ActionAgentUpdate, id, "")
	}
	return s.storeDefinition(ctx, a, rec, def, cur.Version+1, parts)
}

// storeDefinition inserts def as version of rec and writes agents.version, the audit entry and
// both events.
func (s *Service) storeDefinition(ctx context.Context, a actor.Actor, rec domain.Agent, def definition.Definition, version int64, parts []string) (agents.DefinitionVersion, error) {
	raw, err := def.JSON()
	if err != nil {
		return agents.DefinitionVersion{}, errs.Wrap(errs.CodeInternalError, err, "agents: encoding a definition")
	}
	row := DefinitionRow{AgentID: rec.ID, Version: version, Definition: raw, OriginKind: def.Origin.Kind, CreatedAt: s.d.Clock.Now().UTC(), CreatedBy: a.ID}
	if err := s.d.Definitions.InsertDefinition(ctx, row); err != nil {
		return agents.DefinitionVersion{}, err
	}
	rec.Bump()
	if _, err := s.changedAndEmit(ctx, authorize.ActionAgentUpdate, rec, auditing.Attrs{
		attrDefinitionVersion: auditing.Int(version), attrChangedParts: codes(parts),
	}); err != nil {
		return agents.DefinitionVersion{}, err
	}
	if err := s.emitDefinitionChanged(ctx, rec.ID, version); err != nil {
		return agents.DefinitionVersion{}, err
	}
	return versionView(row), nil
}

// definitionPrecondition checks If-Match against the current definition version (06 section 8,
// R-API-29): absent is allowed only before the first version; without a stored version any
// If-Match is stale (412 without current); otherwise absent is 428 and a stale or malformed one
// 412 with the current version.
func definitionPrecondition(p agents.Precondition, cur DefinitionRow, found bool) error {
	switch {
	case !found && !p.Present:
		return nil
	case !found:
		return errs.New(errs.CodeVersionConflict, "")
	case !p.Present:
		return errs.PreconditionRequired()
	case p.Valid && p.Version == cur.Version:
		return nil
	}
	return errs.VersionConflict(versionBody(versionView(cur)))
}

// refusal is 400 validation_failed with one field error per finding; the detail never repeats a
// value of the definition.
func refusal(findings []definition.FieldError) error {
	const detail = "the definition breaks a rule of agent_definition.v1"
	e := errs.New(errs.CodeValidationFailed, detail)
	for _, f := range findings {
		e = e.WithFields(errs.FieldError{Pointer: f.Pointer, Code: f.Code, Detail: "The member breaks the rule " + f.Code + "."})
	}
	return e
}

// emitDefinitionChanged appends agent.definition_changed to the agent itself (26 9): resource the
// agent without version, receive action agent.definition.read, payload definition_version only,
// never the definition.
func (s *Service) emitDefinitionChanged(ctx context.Context, id ids.ID, version int64) error {
	payload, err := json.Marshal(struct {
		DefinitionVersion int64 `json:"definition_version"`
	}{version})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "agents: agent.definition_changed payload")
	}
	kind, by := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventAgentDefinitionChanged), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: by,
		ResourceKind: resourceAgent, ResourceID: id, ReceiveAction: receiveDefinitionRead, AudiencePrincipalID: id, Payload: payload,
	})
}

// GetDefinition implements agents.Definitions: agent.definition.read on the agent, then 404 for
// an unknown agent, one that is no built-in agent, one without a definition and an unknown
// version. The agent itself reads only its current version (26 4.1 item 3, A1-02 design note 7).
func (s *Service) GetDefinition(ctx context.Context, a actor.Actor, id ids.ID, version int64) (agents.DefinitionVersion, error) {
	rec, err := s.load(ctx, a, authorize.ActionAgentDefinitionRead, id)
	if err != nil {
		return agents.DefinitionVersion{}, err
	}
	if rec.AdapterKind != agents.AdapterKindBuiltin {
		return agents.DefinitionVersion{}, invisible()
	}
	row, found, err := s.d.Definitions.CurrentDefinition(ctx, id)
	if err != nil {
		return agents.DefinitionVersion{}, err
	}
	if found && version > 0 && version != row.Version {
		if a.Kind != actor.KindUser {
			return agents.DefinitionVersion{}, invisible()
		}
		row, found, err = s.d.Definitions.DefinitionAt(ctx, id, version)
		if err != nil {
			return agents.DefinitionVersion{}, err
		}
	}
	if !found {
		return agents.DefinitionVersion{}, invisible()
	}
	return versionView(row), nil
}

// versionCursor is the keyset position of a page of versions: the last version of the page.
type versionCursor struct {
	Version int64 `json:"version"`
}

// ListDefinitionVersions implements agents.Definitions: agent.definition.read on the agent, 404
// for an agent that is no built-in agent, then the versions newest first in pages of the API page
// size, without the definition text.
func (s *Service) ListDefinitionVersions(ctx context.Context, a actor.Actor, id ids.ID, p agents.Page) (agents.DefinitionVersionPage, error) {
	rec, err := s.load(ctx, a, authorize.ActionAgentDefinitionRead, id)
	if err != nil {
		return agents.DefinitionVersionPage{}, err
	}
	if rec.AdapterKind != agents.AdapterKindBuiltin {
		return agents.DefinitionVersionPage{}, invisible()
	}
	binding := []byte(a.ID.String() + "\x00" + id.String())
	before := int64(math.MaxInt64)
	if p.Cursor != "" {
		var k versionCursor
		if err := s.d.Cursors.Decode(definitionVersions, binding, p.Cursor, &k); err != nil {
			return agents.DefinitionVersionPage{}, err
		}
		if k.Version < 1 {
			return agents.DefinitionVersionPage{}, errs.New(errs.CodeCursorInvalid, "")
		}
		before = k.Version
	}
	limit := p.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	rows, err := s.d.Definitions.DefinitionVersions(ctx, id, before, limit+1)
	if err != nil {
		return agents.DefinitionVersionPage{}, err
	}
	out := agents.DefinitionVersionPage{Items: make([]agents.DefinitionVersion, 0, min(len(rows), limit))}
	for _, row := range rows[:min(len(rows), limit)] {
		row.Definition = nil
		out.Items = append(out.Items, versionView(row))
	}
	if len(rows) > limit {
		if out.NextCursor, err = s.d.Cursors.Encode(definitionVersions, binding, versionCursor{Version: rows[limit-1].Version}); err != nil {
			return agents.DefinitionVersionPage{}, errs.Wrap(errs.CodeInternalError, err, "agents: definition versions cursor")
		}
	}
	return out, nil
}

// versionView converts a row into the port DTO.
func versionView(row DefinitionRow) agents.DefinitionVersion {
	return agents.DefinitionVersion{
		AgentID: row.AgentID, Version: row.Version, OriginKind: row.OriginKind, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy,
		Definition: json.RawMessage(row.Definition),
	}
}

// versionJSON is the schema AgentDefinitionVersion: the member current of the 412 of
// putAgentDefinition, which only the owner who passed agent.update receives.
type versionJSON struct {
	AgentID    string          `json:"agent_id"`
	Version    int64           `json:"version"`
	OriginKind string          `json:"origin_kind"`
	CreatedAt  string          `json:"created_at"`
	CreatedBy  string          `json:"created_by"`
	Definition json.RawMessage `json:"definition,omitempty"`
}

// versionBody converts the DTO into the JSON form of AgentDefinitionVersion.
func versionBody(v agents.DefinitionVersion) versionJSON {
	return versionJSON{
		AgentID: v.AgentID.String(), Version: v.Version, OriginKind: v.OriginKind, CreatedAt: stamp(v.CreatedAt),
		CreatedBy: v.CreatedBy.String(), Definition: v.Definition,
	}
}

// needsHost is the check of agent.tool_class.set (26 4.4 item 5, R-BA-06): a class below host
// while the current definition of rec has a stdio MCP server is refused with 409 conflict, reason
// definition_needs_host, and an audit deny that survives the rollback.
func (s *Service) needsHost(ctx context.Context, rec domain.Agent, target domain.ToolClass) error {
	if !definition.ToolClass(target).Below(definition.Host) || rec.AdapterKind != agents.AdapterKindBuiltin {
		return nil
	}
	row, found, err := s.d.Definitions.CurrentDefinition(ctx, rec.ID)
	if err != nil || !found {
		return err
	}
	def, err := definition.Decode(row.Definition)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "agents: a stored definition outside its schema")
	}
	if !def.NeedsHost() {
		return nil
	}
	if err := s.d.Audit.RecordDenial(ctx, auditing.Entry{
		Action: string(authorize.ActionAgentToolClassSet), Decision: auditing.DecisionDeny, Reason: auditing.ReasonDefinitionNeedsHost,
		ResourceKind: resourceAgent, ResourceID: rec.ID, Attrs: auditing.Attrs{attrToToolClass: auditing.Code(string(target))},
	}); err != nil {
		return err
	}
	return errs.New(errs.CodeConflict, "the definition has a stdio MCP server, which needs the tool class host").WithReason(reasonNeedsHost)
}
