// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// ErrNameTaken reports a component name that another component already has (unique
// lower(name), docs/spec/03-data-model.md 4.12).
var ErrNameTaken = errors.New("status: the component name is taken")

// PageStore is the outgoing port of the status page to status_components and the incident
// tables; adapters/postgres implements it. Every method runs in the ambient unit of work of ctx
// when there is one.
type PageStore interface {
	// Components returns every component in sort_key order.
	Components(ctx context.Context) ([]domain.Component, error)
	// LockComponents returns every component in sort_key order under its row lock until the
	// commit; it serializes the writers of the page (the owner and the job status.probe).
	LockComponents(ctx context.Context) ([]domain.Component, error)
	// InsertComponent inserts c; inserted is false when another component has its name (nothing
	// was written then).
	InsertComponent(ctx context.Context, c domain.Component) (inserted bool, err error)
	// UpdateComponent writes name, description, state, health_source, agent_id and updated_at of
	// c. Errors: ErrNameTaken.
	UpdateComponent(ctx context.Context, c domain.Component) error
	// SetSortKeys writes the sort keys of the components named in keys.
	SetSortKeys(ctx context.Context, keys map[ids.ID]string) error
	// SetComponentState writes state and updated_at of the component id.
	SetComponentState(ctx context.Context, id ids.ID, state domain.ComponentState, at time.Time) error
	// DeleteComponent removes the component id with its incident links; false when there is none.
	DeleteComponent(ctx context.Context, id ids.ID) (bool, error)
	// DeleteAgentComponents removes the components of agent and returns their ids.
	DeleteAgentComponents(ctx context.Context, agent ids.ID) ([]ids.ID, error)
	// Incidents returns the incidents that are not resolved or were resolved at or after since,
	// newest start first, with their components and updates.
	Incidents(ctx context.Context, since time.Time) ([]domain.Incident, error)
	// LockIncident returns the incident id with its components and updates under its row lock;
	// false when there is none.
	LockIncident(ctx context.Context, id ids.ID) (domain.Incident, bool, error)
	// InsertIncident inserts inc with its component links and its first update.
	InsertIncident(ctx context.Context, inc domain.Incident, first domain.IncidentUpdate) error
	// AddIncidentUpdate inserts u and sets state and resolved_at of its incident (zero resolvedAt
	// writes NULL).
	AddIncidentUpdate(ctx context.Context, u domain.IncidentUpdate, resolvedAt time.Time) error
	// OpenAutomaticIncidents returns, per component, the automatic incidents (created_by NULL)
	// that are not resolved.
	OpenAutomaticIncidents(ctx context.Context) (map[ids.ID][]ids.ID, error)
	// LatestAutomaticIncidents returns, per component, its newest automatic incident by
	// started_at; a component without one is absent.
	LatestAutomaticIncidents(ctx context.Context) (map[ids.ID]domain.LatestIncident, error)
}
