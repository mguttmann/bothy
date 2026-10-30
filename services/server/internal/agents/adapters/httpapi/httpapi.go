// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the agents module (M1-51): the ten operations of the
// group Agents of docs/api/openapi.yaml as part of rest.StrictServerInterface. The use cases of
// agents.Agents authorize; the handlers only convert, set ETag from the version and pass errors
// to the problem writer of the chain unchanged.
//
// Example:
//
//	h := httpapi.New(svc, consumers)
//	built, err := routes.Build(wire.Handlers{Agents: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Agents implements listAgents, createAgent, getAgent, updateAgent, disableAgent, enableAgent,
// setAgentBillingMode, setAllAgentsBillingMode, setCredentialHolder and setAgentToolClass, and the
// consumer operations getConsumerState, pollAgentEvents and ackAgentEvents (M2-07),
// sendHeartbeat (M2-20) and getAgentTriggerers.
type Agents struct {
	agents    agents.Agents
	consumers agents.ConsumerAPI
	// presence and interval serve sendHeartbeat (M2-20, WithPresence).
	presence agents.Presence
	interval time.Duration
}

// New returns the handlers over the use cases of the records a and of the consumers c.
func New(a agents.Agents, c agents.ConsumerAPI) *Agents { return &Agents{agents: a, consumers: c} }

// Cursors is the codec of the list cursors (platform/httpserver), the outgoing port app.Cursors.
type Cursors struct{}

// Compile-time check: the codec is the port of the use cases.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}

// principal returns the actor of the request; anonymous when the chain set none.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// precondition parses the If-Match header value of a request.
func precondition(v string) agents.Precondition {
	h := http.Header{}
	if v != "" {
		h.Set("If-Match", v)
	}
	im := httpserver.ParseIfMatch(h)
	return agents.Precondition{Present: im.Present, Valid: im.Valid, Version: im.Version}
}

// etag returns the ETag header value of a.
func etag(a agents.Agent) *string {
	s := httpserver.FormatETag(a.Version)
	return &s
}

// ListAgents implements rest.StrictServerInterface (GET /agents).
func (h *Agents) ListAgents(ctx context.Context, req rest.ListAgentsRequestObject) (rest.ListAgentsResponseObject, error) {
	var p agents.Page
	if req.Params.Cursor != nil {
		p.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		p.Limit = *req.Params.Limit
	}
	page, err := h.agents.List(ctx, principal(ctx), p)
	if err != nil {
		return nil, err
	}
	out := rest.AgentPage{Items: make([]rest.Agent, 0, len(page.Items))}
	for _, a := range page.Items {
		out.Items = append(out.Items, toREST(a))
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListAgents200JSONResponse{Body: out}, nil
}

// CreateAgent implements rest.StrictServerInterface (POST /agents).
func (h *Agents) CreateAgent(ctx context.Context, req rest.CreateAgentRequestObject) (rest.CreateAgentResponseObject, error) {
	b := req.Body
	cmd := agents.CreateCmd{Handle: b.Handle, DisplayName: b.DisplayName, AdapterKind: b.AdapterKind}
	if b.Description != nil {
		cmd.Description = *b.Description
	}
	if b.OperatorConfig != nil {
		cmd.OperatorConfig = map[string]any(*b.OperatorConfig)
	}
	a, err := h.agents.Create(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateAgent201JSONResponse{Body: toREST(a)}, nil
}

// GetAgent implements rest.StrictServerInterface (GET /agents/{agent_id}).
func (h *Agents) GetAgent(ctx context.Context, req rest.GetAgentRequestObject) (rest.GetAgentResponseObject, error) {
	a, err := h.agents.Get(ctx, principal(ctx), ids.ID(req.AgentId))
	if err != nil {
		return nil, err
	}
	return rest.GetAgent200JSONResponse{Body: toREST(a), Headers: rest.GetAgent200ResponseHeaders{ETag: etag(a)}}, nil
}

// UpdateAgent implements rest.StrictServerInterface (PATCH /agents/{agent_id}).
func (h *Agents) UpdateAgent(ctx context.Context, req rest.UpdateAgentRequestObject) (rest.UpdateAgentResponseObject, error) {
	b := req.Body
	cmd := agents.UpdateCmd{DisplayName: b.DisplayName, Description: b.Description, IfMatch: precondition(req.Params.IfMatch)}
	if b.OperatorConfig != nil {
		cmd.OperatorConfig = map[string]any(*b.OperatorConfig)
	}
	a, err := h.agents.Update(ctx, principal(ctx), ids.ID(req.AgentId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.UpdateAgent200JSONResponse{Body: toREST(a), Headers: rest.UpdateAgent200ResponseHeaders{ETag: etag(a)}}, nil
}

// DisableAgent implements rest.StrictServerInterface (POST /agents/{agent_id}/disable).
func (h *Agents) DisableAgent(ctx context.Context, req rest.DisableAgentRequestObject) (rest.DisableAgentResponseObject, error) {
	a, err := h.agents.Disable(ctx, principal(ctx), ids.ID(req.AgentId), req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return rest.DisableAgent200JSONResponse{Body: toREST(a)}, nil
}

// EnableAgent implements rest.StrictServerInterface (POST /agents/{agent_id}/enable).
func (h *Agents) EnableAgent(ctx context.Context, req rest.EnableAgentRequestObject) (rest.EnableAgentResponseObject, error) {
	a, err := h.agents.Enable(ctx, principal(ctx), ids.ID(req.AgentId), req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return rest.EnableAgent200JSONResponse{Body: toREST(a)}, nil
}

// SetAgentBillingMode implements rest.StrictServerInterface (PUT /agents/{agent_id}/billing-mode).
func (h *Agents) SetAgentBillingMode(ctx context.Context, req rest.SetAgentBillingModeRequestObject) (rest.SetAgentBillingModeResponseObject, error) {
	b := req.Body
	cmd := agents.BillingModeCmd{BillingMode: string(b.BillingMode), Reason: b.Reason, IfMatch: precondition(req.Params.IfMatch)}
	if b.CredentialHolderId != nil {
		cmd.CredentialHolderID = ids.ID(*b.CredentialHolderId)
	}
	a, err := h.agents.SetBillingMode(ctx, principal(ctx), ids.ID(req.AgentId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.SetAgentBillingMode200JSONResponse{Body: toREST(a), Headers: rest.SetAgentBillingMode200ResponseHeaders{ETag: etag(a)}}, nil
}

// SetAllAgentsBillingMode implements rest.StrictServerInterface (PUT /instance/agent-billing-mode).
func (h *Agents) SetAllAgentsBillingMode(ctx context.Context, req rest.SetAllAgentsBillingModeRequestObject) (rest.SetAllAgentsBillingModeResponseObject, error) {
	b := req.Body
	cmd := agents.BillingModeAllCmd{BillingMode: string(b.BillingMode), Reason: b.Reason}
	if b.CredentialHolderId != nil {
		cmd.CredentialHolderID = ids.ID(*b.CredentialHolderId)
	}
	if b.AgentIds != nil {
		cmd.AgentIDsGiven = true
		for _, id := range *b.AgentIds {
			cmd.AgentIDs = append(cmd.AgentIDs, ids.ID(id))
		}
	}
	changed, err := h.agents.SetBillingModeAll(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	out := rest.BillingModeSetAllResult{ChangedAgentIds: make([]rest.Uuid, 0, len(changed))}
	for _, id := range changed {
		out.ChangedAgentIds = append(out.ChangedAgentIds, rest.Uuid(id))
	}
	return rest.SetAllAgentsBillingMode200JSONResponse{Body: out}, nil
}

// SetCredentialHolder implements rest.StrictServerInterface (PUT /agents/{agent_id}/credential-holder).
func (h *Agents) SetCredentialHolder(ctx context.Context, req rest.SetCredentialHolderRequestObject) (rest.SetCredentialHolderResponseObject, error) {
	b := req.Body
	a, err := h.agents.SetCredentialHolder(ctx, principal(ctx), ids.ID(req.AgentId), agents.CredentialHolderCmd{
		CredentialHolderID: ids.ID(b.CredentialHolderId), Reason: b.Reason, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetCredentialHolder200JSONResponse{Body: toREST(a), Headers: rest.SetCredentialHolder200ResponseHeaders{ETag: etag(a)}}, nil
}

// SetAgentToolClass implements rest.StrictServerInterface (PUT /agents/{agent_id}/tool-class).
func (h *Agents) SetAgentToolClass(ctx context.Context, req rest.SetAgentToolClassRequestObject) (rest.SetAgentToolClassResponseObject, error) {
	b := req.Body
	a, err := h.agents.SetToolClass(ctx, principal(ctx), ids.ID(req.AgentId), agents.ToolClassCmd{
		ToolClass: string(b.ToolClass), Reason: b.Reason, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetAgentToolClass200JSONResponse{Body: toREST(a), Headers: rest.SetAgentToolClass200ResponseHeaders{ETag: etag(a)}}, nil
}

// toREST converts an agent (schema Agent); empty values become absent members, never null (ADR
// 0053).
func toREST(a agents.Agent) rest.Agent {
	presence := presenceREST(a.Presence)
	config := rest.JsonObject(a.OperatorConfig)
	if config == nil {
		config = rest.JsonObject{}
	}
	return rest.Agent{
		Id: rest.Uuid(a.ID), Handle: a.Handle, DisplayName: a.DisplayName, AvatarFileId: optionalID(a.AvatarFileID),
		Status: rest.AgentStatus(a.Status), AdapterKind: a.AdapterKind, Description: a.Description,
		BillingMode: rest.BillingMode(a.BillingMode), CredentialHolderId: optionalID(a.CredentialHolderID),
		BillingModeChangedAt: optionalTime(a.BillingModeChangedAt), ToolClass: rest.ToolClass(a.ToolClass),
		Presence: presence, OperatorConfig: config, Version: a.Version,
	}
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalID(id ids.ID) *rest.Uuid {
	if id.IsZero() {
		return nil
	}
	u := rest.Uuid(id)
	return &u
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t.UTC().Truncate(time.Millisecond)
	return &v
}
