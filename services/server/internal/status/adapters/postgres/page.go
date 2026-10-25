// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// The SQLSTATE codes the page store maps.
const (
	sqlstateUniqueViolation     = "23505"
	sqlstateForeignKeyViolation = "23503"
)

// Page implements app.PageStore over status_components and the incident tables (M7-09). Every
// method runs in the ambient unit of work of ctx when there is one. It is safe for concurrent use.
type Page struct {
	pool *pg.Pool
}

// Compile-time check: Page is the store of the status page.
var _ app.PageStore = (*Page)(nil)

// NewPage returns the store over pool.
func NewPage(pool *pg.Pool) *Page { return &Page{pool: pool} }

// q returns the queries on the ambient transaction of ctx or on the pool.
func (p *Page) q(ctx context.Context) *sqlcgen.Queries { return sqlcgen.New(pg.Querier(ctx, p.pool)) }

// Components implements app.PageStore.
func (p *Page) Components(ctx context.Context) ([]domain.Component, error) {
	rows, err := p.q(ctx).ListComponents(ctx)
	if err != nil {
		return nil, err
	}
	return toComponents(rows), nil
}

// LockComponents implements app.PageStore.
func (p *Page) LockComponents(ctx context.Context) ([]domain.Component, error) {
	rows, err := p.q(ctx).LockComponents(ctx)
	if err != nil {
		return nil, err
	}
	return toComponents(rows), nil
}

// InsertComponent implements app.PageStore. Errors: app.ErrUnknownAgent for an agent_id that
// names no agent.
func (p *Page) InsertComponent(ctx context.Context, c domain.Component) (bool, error) {
	n, err := p.q(ctx).InsertComponent(ctx, sqlcgen.InsertComponentParams{
		ID: uuid(c.ID), Name: c.Name, Description: c.Description, SortKey: c.SortKey, State: string(c.State),
		HealthSource: text(c.HealthSource), AgentID: optUUID(c.AgentID), UpdatedAt: ts(c.UpdatedAt),
	})
	if err != nil {
		return false, mapWriteError(err)
	}
	return n == 1, nil
}

// UpdateComponent implements app.PageStore. Errors: app.ErrNameTaken, app.ErrUnknownAgent.
func (p *Page) UpdateComponent(ctx context.Context, c domain.Component) error {
	err := p.q(ctx).UpdateComponent(ctx, sqlcgen.UpdateComponentParams{
		ID: uuid(c.ID), Name: c.Name, Description: c.Description, State: string(c.State),
		HealthSource: text(c.HealthSource), AgentID: optUUID(c.AgentID), UpdatedAt: ts(c.UpdatedAt),
	})
	return mapWriteError(err)
}

// mapWriteError turns a unique violation into app.ErrNameTaken and a foreign key violation into
// app.ErrUnknownAgent; the component table has no other unique key or foreign key a write can
// break.
func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case sqlstateUniqueViolation:
			return app.ErrNameTaken
		case sqlstateForeignKeyViolation:
			return app.ErrUnknownAgent
		}
	}
	return err
}

// SetSortKeys implements app.PageStore.
func (p *Page) SetSortKeys(ctx context.Context, keys map[ids.ID]string) error {
	params := sqlcgen.SetComponentSortKeysParams{}
	for id, k := range keys {
		params.Ids = append(params.Ids, uuid(id))
		params.SortKeys = append(params.SortKeys, k)
	}
	return p.q(ctx).SetComponentSortKeys(ctx, params)
}

// SetComponentState implements app.PageStore.
func (p *Page) SetComponentState(ctx context.Context, id ids.ID, state domain.ComponentState, at time.Time) error {
	return p.q(ctx).SetComponentState(ctx, sqlcgen.SetComponentStateParams{ID: uuid(id), State: string(state), UpdatedAt: ts(at)})
}

// DeleteComponent implements app.PageStore.
func (p *Page) DeleteComponent(ctx context.Context, id ids.ID) (bool, error) {
	n, err := p.q(ctx).DeleteComponent(ctx, uuid(id))
	return n == 1, err
}

// DeleteAgentComponents implements app.PageStore.
func (p *Page) DeleteAgentComponents(ctx context.Context, agent ids.ID) ([]ids.ID, error) {
	rows, err := p.q(ctx).DeleteAgentComponents(ctx, uuid(agent))
	if err != nil {
		return nil, err
	}
	out := make([]ids.ID, len(rows))
	for i, r := range rows {
		out[i] = r.Bytes
	}
	return out, nil
}

// Incidents implements app.PageStore.
func (p *Page) Incidents(ctx context.Context, since time.Time) ([]domain.Incident, error) {
	rows, err := p.q(ctx).ListIncidents(ctx, ts(since))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Incident, len(rows))
	for i, r := range rows {
		out[i] = toIncident(r)
	}
	return out, p.fill(ctx, out)
}

// LockIncident implements app.PageStore.
func (p *Page) LockIncident(ctx context.Context, id ids.ID) (domain.Incident, bool, error) {
	row, err := p.q(ctx).LockIncident(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Incident{}, false, nil
	}
	if err != nil {
		return domain.Incident{}, false, err
	}
	out := []domain.Incident{toIncident(row)}
	if err := p.fill(ctx, out); err != nil {
		return domain.Incident{}, false, err
	}
	return out[0], true, nil
}

// fill reads the components and the updates of incs in two queries.
func (p *Page) fill(ctx context.Context, incs []domain.Incident) error {
	if len(incs) == 0 {
		return nil
	}
	at := make(map[ids.ID]int, len(incs))
	list := make([]pgtype.UUID, len(incs))
	for i, inc := range incs {
		at[inc.ID] = i
		list[i] = uuid(inc.ID)
	}
	q := p.q(ctx)
	links, err := q.ListIncidentComponents(ctx, list)
	if err != nil {
		return err
	}
	for _, l := range links {
		i := at[l.IncidentID.Bytes]
		incs[i].ComponentIDs = append(incs[i].ComponentIDs, l.ComponentID.Bytes)
	}
	updates, err := q.ListIncidentUpdates(ctx, list)
	if err != nil {
		return err
	}
	for _, u := range updates {
		i := at[u.IncidentID.Bytes]
		incs[i].Updates = append(incs[i].Updates, domain.IncidentUpdate{
			ID: u.ID.Bytes, IncidentID: u.IncidentID.Bytes, State: domain.IncidentState(u.State), BodyMD: u.BodyMd,
			CreatedBy: idOf(u.CreatedBy), CreatedAt: u.CreatedAt.Time,
		})
	}
	return nil
}

// InsertIncident implements app.PageStore.
func (p *Page) InsertIncident(ctx context.Context, inc domain.Incident, first domain.IncidentUpdate) error {
	q := p.q(ctx)
	if err := q.InsertIncident(ctx, sqlcgen.InsertIncidentParams{
		ID: uuid(inc.ID), Title: inc.Title, Kind: string(inc.Kind), Impact: string(inc.Impact), State: string(inc.State),
		StartedAt: ts(inc.StartedAt), ResolvedAt: optTS(inc.ResolvedAt), CreatedBy: optUUID(inc.CreatedBy),
	}); err != nil {
		return err
	}
	if len(inc.ComponentIDs) > 0 {
		list := make([]pgtype.UUID, len(inc.ComponentIDs))
		for i, id := range inc.ComponentIDs {
			list[i] = uuid(id)
		}
		if err := q.InsertIncidentComponents(ctx, sqlcgen.InsertIncidentComponentsParams{IncidentID: uuid(inc.ID), ComponentIds: list}); err != nil {
			return err
		}
	}
	return p.insertUpdate(ctx, q, first)
}

// AddIncidentUpdate implements app.PageStore.
func (p *Page) AddIncidentUpdate(ctx context.Context, u domain.IncidentUpdate, resolvedAt time.Time) error {
	q := p.q(ctx)
	if err := p.insertUpdate(ctx, q, u); err != nil {
		return err
	}
	return q.SetIncidentState(ctx, sqlcgen.SetIncidentStateParams{ID: uuid(u.IncidentID), State: string(u.State), ResolvedAt: optTS(resolvedAt)})
}

// insertUpdate inserts u with q.
func (p *Page) insertUpdate(ctx context.Context, q *sqlcgen.Queries, u domain.IncidentUpdate) error {
	return q.InsertIncidentUpdate(ctx, sqlcgen.InsertIncidentUpdateParams{
		ID: uuid(u.ID), IncidentID: uuid(u.IncidentID), State: string(u.State), BodyMd: u.BodyMD,
		CreatedBy: optUUID(u.CreatedBy), CreatedAt: ts(u.CreatedAt),
	})
}

// OpenAutomaticIncidents implements app.PageStore.
func (p *Page) OpenAutomaticIncidents(ctx context.Context) (map[ids.ID][]ids.ID, error) {
	rows, err := p.q(ctx).OpenAutomaticIncidents(ctx)
	if err != nil {
		return nil, err
	}
	out := map[ids.ID][]ids.ID{}
	for _, r := range rows {
		c := ids.ID(r.ComponentID.Bytes)
		out[c] = append(out[c], r.ID.Bytes)
	}
	return out, nil
}

// AutomaticIncidentSince implements app.PageStore.
func (p *Page) AutomaticIncidentSince(ctx context.Context, component ids.ID, since time.Time) (bool, error) {
	return p.q(ctx).AutomaticIncidentSince(ctx, sqlcgen.AutomaticIncidentSinceParams{ComponentID: uuid(component), Since: ts(since)})
}

// toComponents maps rows to the domain.
func toComponents(rows []sqlcgen.StatusComponent) []domain.Component {
	out := make([]domain.Component, len(rows))
	for i, r := range rows {
		out[i] = domain.Component{
			ID: r.ID.Bytes, Name: r.Name, Description: r.Description, SortKey: r.SortKey, State: domain.ComponentState(r.State),
			HealthSource: r.HealthSource.String, AgentID: idOf(r.AgentID), UpdatedAt: r.UpdatedAt.Time,
		}
	}
	return out
}

// toIncident maps a row to the domain without components and updates.
func toIncident(r sqlcgen.StatusIncident) domain.Incident {
	out := domain.Incident{
		ID: r.ID.Bytes, Title: r.Title, Kind: domain.IncidentKind(r.Kind), Impact: domain.Impact(r.Impact),
		State: domain.IncidentState(r.State), StartedAt: r.StartedAt.Time, CreatedBy: idOf(r.CreatedBy),
	}
	if r.ResolvedAt.Valid {
		out.ResolvedAt = r.ResolvedAt.Time
	}
	return out
}

// optUUID is NULL for the zero id.
func optUUID(id ids.ID) pgtype.UUID {
	if id.IsZero() {
		return pgtype.UUID{}
	}
	return uuid(id)
}

// optTS is NULL for the zero time.
func optTS(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return ts(t)
}

// text is NULL for "".
func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
