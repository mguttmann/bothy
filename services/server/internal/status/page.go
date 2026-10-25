// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Component is one component of the status page (schema StatusComponent).
type Component struct {
	ID          ids.ID
	Name        string
	Description string
	SortKey     string
	// State is operational, degraded, partial_outage, major_outage or maintenance.
	State string
	// HealthSource is the key of the automatic check; "" for a manual component.
	HealthSource string
	// AgentID is the agent of a presence component; zero for none.
	AgentID   ids.ID
	UpdatedAt time.Time
}

// IncidentUpdate is one entry of the history of an incident (schema StatusIncidentUpdate).
type IncidentUpdate struct {
	ID        ids.ID
	State     string
	BodyMD    string
	CreatedAt time.Time
}

// Incident is an incident or a maintenance window (schema StatusIncident).
type Incident struct {
	ID     ids.ID
	Title  string
	Kind   string
	Impact string
	State  string
	// StartedAt is the start; ResolvedAt is zero while the incident is not resolved.
	StartedAt, ResolvedAt time.Time
	// ComponentIDs are the affected components the reader may see, ascending.
	ComponentIDs []ids.ID
	// Updates are the history, oldest first.
	Updates []IncidentUpdate
}

// Page is the status page of one reader (schema StatusPage).
type Page struct {
	// OverallState is the worst state of the components that belong to no agent.
	OverallState string
	// Components are the visible components in sort_key order.
	Components []Component
	// Incidents are the open incidents and the ones resolved within status.history_window,
	// newest first.
	Incidents []Incident
}

// ComponentInput is the body of createStatusComponent and updateStatusComponent (schema
// StatusComponentInput). A nil pointer leaves the field as it is (update) or at its default
// (create).
type ComponentInput struct {
	Name         string
	Description  *string
	State        *string
	HealthSource *string
	AgentID      *ids.ID
	// AfterComponentID places the component right after that one; nil places a new component at
	// the end and leaves an existing one where it is.
	AfterComponentID *ids.ID
}

// IncidentCreate is the body of createStatusIncident (schema StatusIncidentCreate).
type IncidentCreate struct {
	Title, Kind, Impact string
	// State is "" for the default: scheduled for a maintenance, investigating for an incident.
	State        string
	StartedAt    time.Time
	ComponentIDs []ids.ID
	// BodyMD is the text of the first update.
	BodyMD string
}

// IncidentUpdateCreate is the body of addStatusIncidentUpdate (schema
// StatusIncidentUpdateCreate).
type IncidentUpdateCreate struct {
	State  string
	BodyMD string
}

// Status is the inbound port of the status page (docs/spec/16-operations.md 12, R-OP-15): the
// seven operations of the tag Status. Every write runs in one unit of work with its audit entry
// and appends status.updated; errors are kernel/errs errors.
type Status interface {
	// Read returns the page for the logged-in principal a (status.read): agent components only
	// with agent.read on their agent, incidents only with at least one visible component or none.
	// Errors: the refusal of status.read.
	//
	// Example:
	//
	//	page, err := st.Read(ctx, a)
	Read(ctx context.Context, a actor.Actor) (Page, error)
	// ReadPublic returns the page without login (status.public.read): the components without
	// agent components. Errors: 404 not_found unless status.public_enabled.
	ReadPublic(ctx context.Context) (Page, error)
	// CreateComponent adds a component (status.manage). Errors: 422 validation_failed, 409
	// conflict with reason name_taken, the refusal of status.manage.
	CreateComponent(ctx context.Context, a actor.Actor, in ComponentInput) (Component, error)
	// UpdateComponent changes a component (status.manage). Errors: as CreateComponent, 404
	// not_found.
	UpdateComponent(ctx context.Context, a actor.Actor, id ids.ID, in ComponentInput) (Component, error)
	// DeleteComponent removes a component and its incident links (status.manage). Errors: 404
	// not_found.
	DeleteComponent(ctx context.Context, a actor.Actor, id ids.ID) error
	// CreateIncident opens an incident or a maintenance with its first update (status.manage).
	// Errors: 422 validation_failed, among them an unknown component.
	CreateIncident(ctx context.Context, a actor.Actor, in IncidentCreate) (Incident, error)
	// AddIncidentUpdate adds an update and sets the state of the incident (status.manage).
	// Errors: 422 validation_failed, 404 not_found.
	AddIncidentUpdate(ctx context.Context, a actor.Actor, id ids.ID, in IncidentUpdateCreate) (Incident, error)
}
