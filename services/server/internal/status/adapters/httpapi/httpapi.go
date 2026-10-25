// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the status module as part of
// rest.StrictServerInterface (M7-09): getStatus, getPublicStatus, createStatusComponent,
// updateStatusComponent, deleteStatusComponent, createStatusIncident and addStatusIncidentUpdate
// of docs/api/openapi.yaml. The use cases authorize; the handlers only convert and pass errors to
// the problem writer of the chain unchanged. The Idempotency-Key of the writes is handled by the
// middleware of the chain (M1-20).
//
// Example:
//
//	h := httpapi.New(page)
//	built, err := routes.Build(wire.Handlers{Status: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/status"
)

// Status implements the REST operations of the status page over the port status.Status.
type Status struct {
	page status.Status
}

// New returns the handlers over the port.
func New(p status.Status) *Status { return &Status{page: p} }

// principal returns the actor of the request, the anonymous actor outside one.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// GetStatus implements rest.StrictServerInterface (GET /status).
func (h *Status) GetStatus(ctx context.Context, _ rest.GetStatusRequestObject) (rest.GetStatusResponseObject, error) {
	p, err := h.page.Read(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	return rest.GetStatus200JSONResponse{Body: PageREST(p)}, nil
}

// GetPublicStatus implements rest.StrictServerInterface (GET /status/public, without login).
func (h *Status) GetPublicStatus(ctx context.Context, _ rest.GetPublicStatusRequestObject) (rest.GetPublicStatusResponseObject, error) {
	p, err := h.page.ReadPublic(ctx)
	if err != nil {
		return nil, err
	}
	return rest.GetPublicStatus200JSONResponse{Body: PageREST(p)}, nil
}

// CreateStatusComponent implements rest.StrictServerInterface (POST /status/components): 201
// with the new component.
func (h *Status) CreateStatusComponent(ctx context.Context, req rest.CreateStatusComponentRequestObject) (rest.CreateStatusComponentResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	c, err := h.page.CreateComponent(ctx, principal(ctx), componentInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return rest.CreateStatusComponent201JSONResponse{Body: ComponentREST(c)}, nil
}

// UpdateStatusComponent implements rest.StrictServerInterface (PATCH
// /status/components/{component_id}).
func (h *Status) UpdateStatusComponent(ctx context.Context, req rest.UpdateStatusComponentRequestObject) (rest.UpdateStatusComponentResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	c, err := h.page.UpdateComponent(ctx, principal(ctx), ids.ID(req.ComponentId), componentInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return rest.UpdateStatusComponent200JSONResponse{Body: ComponentREST(c)}, nil
}

// DeleteStatusComponent implements rest.StrictServerInterface (DELETE
// /status/components/{component_id}): 204.
func (h *Status) DeleteStatusComponent(ctx context.Context, req rest.DeleteStatusComponentRequestObject) (rest.DeleteStatusComponentResponseObject, error) {
	if err := h.page.DeleteComponent(ctx, principal(ctx), ids.ID(req.ComponentId)); err != nil {
		return nil, err
	}
	return rest.DeleteStatusComponent204Response{}, nil
}

// CreateStatusIncident implements rest.StrictServerInterface (POST /status/incidents): 201 with
// the incident and its first update.
func (h *Status) CreateStatusIncident(ctx context.Context, req rest.CreateStatusIncidentRequestObject) (rest.CreateStatusIncidentResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	b := req.Body
	in := status.IncidentCreate{Title: b.Title, Kind: string(b.Kind), Impact: string(b.Impact), StartedAt: b.StartedAt, BodyMD: b.BodyMd}
	if b.State != nil {
		in.State = string(*b.State)
	}
	for _, id := range b.ComponentIds {
		in.ComponentIDs = append(in.ComponentIDs, ids.ID(id))
	}
	inc, err := h.page.CreateIncident(ctx, principal(ctx), in)
	if err != nil {
		return nil, err
	}
	return rest.CreateStatusIncident201JSONResponse{Body: IncidentREST(inc)}, nil
}

// AddStatusIncidentUpdate implements rest.StrictServerInterface (POST
// /status/incidents/{incident_id}/updates): 201 with the whole incident.
func (h *Status) AddStatusIncidentUpdate(ctx context.Context, req rest.AddStatusIncidentUpdateRequestObject) (rest.AddStatusIncidentUpdateResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	inc, err := h.page.AddIncidentUpdate(ctx, principal(ctx), ids.ID(req.IncidentId),
		status.IncidentUpdateCreate{State: string(req.Body.State), BodyMD: req.Body.BodyMd})
	if err != nil {
		return nil, err
	}
	return rest.AddStatusIncidentUpdate201JSONResponse{Body: IncidentREST(inc)}, nil
}

// componentInput converts the schema StatusComponentInput.
func componentInput(b rest.StatusComponentInput) status.ComponentInput {
	in := status.ComponentInput{Name: b.Name, Description: b.Description, HealthSource: b.HealthSource}
	if b.State != nil {
		s := string(*b.State)
		in.State = &s
	}
	if b.AgentId != nil {
		id := ids.ID(*b.AgentId)
		in.AgentID = &id
	}
	if b.AfterComponentId != nil {
		id := ids.ID(*b.AfterComponentId)
		in.AfterComponentID = &id
	}
	return in
}

// PageREST converts the page to the schema StatusPage; the lists are never null.
func PageREST(p status.Page) rest.StatusPage {
	out := rest.StatusPage{OverallState: rest.StatusComponentState(p.OverallState),
		Components: make([]rest.StatusComponent, 0, len(p.Components)), Incidents: make([]rest.StatusIncident, 0, len(p.Incidents))}
	for _, c := range p.Components {
		out.Components = append(out.Components, ComponentREST(c))
	}
	for _, i := range p.Incidents {
		out.Incidents = append(out.Incidents, IncidentREST(i))
	}
	return out
}

// ComponentREST converts a component to the schema StatusComponent; health_source and agent_id
// are absent when not set.
func ComponentREST(c status.Component) rest.StatusComponent {
	out := rest.StatusComponent{Id: rest.Uuid(c.ID), Name: c.Name, Description: c.Description, SortKey: c.SortKey,
		State: rest.StatusComponentState(c.State), UpdatedAt: timestamp(c.UpdatedAt)}
	if c.HealthSource != "" {
		hs := c.HealthSource
		out.HealthSource = &hs
	}
	if !c.AgentID.IsZero() {
		id := rest.Uuid(c.AgentID)
		out.AgentId = &id
	}
	return out
}

// IncidentREST converts an incident to the schema StatusIncident; resolved_at is absent while it
// is open.
func IncidentREST(i status.Incident) rest.StatusIncident {
	out := rest.StatusIncident{Id: rest.Uuid(i.ID), Title: i.Title, Kind: rest.StatusIncidentKind(i.Kind),
		Impact: rest.StatusIncidentImpact(i.Impact), State: rest.StatusIncidentState(i.State), StartedAt: timestamp(i.StartedAt),
		ComponentIds: make([]rest.Uuid, 0, len(i.ComponentIDs)), Updates: make([]rest.StatusIncidentUpdate, 0, len(i.Updates))}
	if !i.ResolvedAt.IsZero() {
		t := timestamp(i.ResolvedAt)
		out.ResolvedAt = &t
	}
	for _, id := range i.ComponentIDs {
		out.ComponentIds = append(out.ComponentIds, rest.Uuid(id))
	}
	for _, u := range i.Updates {
		out.Updates = append(out.Updates, rest.StatusIncidentUpdate{Id: rest.Uuid(u.ID), State: rest.StatusIncidentState(u.State),
			BodyMd: u.BodyMD, CreatedAt: timestamp(u.CreatedAt)})
	}
	return out
}

// timestamp is t in UTC at the millisecond precision of the schema Timestamp.
func timestamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// missingBody is the 400 of a request without its required body; the schema validation of the
// chain normally answers first.
func missingBody() error {
	return errs.New(errs.CodeValidationFailed, "the request body is required").
		WithFields(errs.FieldError{Pointer: "", Code: "required", Detail: "the request body is required"})
}
