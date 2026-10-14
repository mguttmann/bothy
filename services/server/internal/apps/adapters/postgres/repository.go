// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the module apps: the sqlc queries in queries/, the
// generated code in sqlcgen/ and Repository, which implements app.Repository over the tables apps
// and app_installations (docs/spec/03-datenmodell.md section 4.4). Only this package runs SQL on
// those tables (03 Grundsatz 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	svc, err := app.New(cfg, app.Deps{Repo: repo, ...})
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Repository with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the use cases.
var _ app.Repository = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads,
// locks and writes inside one use its transaction.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write or a lock: only the ambient transaction of ctx.
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// sqlstateUniqueViolation is a unique violation (a concurrent install of the same slug).
const sqlstateUniqueViolation = "23505"

// mapError turns a database error of the operation what into a kernel/errs error: a unique
// violation is conflict, a kernel/errs error passes, anything else is internal. The text never
// contains SQL or values.
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateUniqueViolation {
		return errs.Wrap(errs.CodeConflict, err, "The app is being installed concurrently.")
	}
	return errs.Wrap(errs.CodeInternalError, err, "apps: "+what+" failed")
}

// one maps the result of a query for one row: no row is found false.
func one[R, T any](row R, err error, conv func(R) T, what string) (T, bool, error) {
	var zero T
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, mapError(err, what)
	}
	return conv(row), true, nil
}

// App implements app.Repository.
func (r *Repository) App(ctx context.Context, id ids.ID) (app.StoredApp, bool, error) {
	row, err := r.read(ctx).GetApp(ctx, uuid(id))
	return one(row, err, func(x sqlcgen.GetAppRow) app.StoredApp { return toApp(x.App) }, "reading an app")
}

// AppBySlug implements app.Repository.
func (r *Repository) AppBySlug(ctx context.Context, slug string) (app.StoredApp, bool, error) {
	row, err := r.read(ctx).GetAppBySlug(ctx, slug)
	return one(row, err, func(x sqlcgen.GetAppBySlugRow) app.StoredApp { return toApp(x.App) }, "reading an app by slug")
}

// LockAppBySlug implements app.Repository.
func (r *Repository) LockAppBySlug(ctx context.Context, slug string) (app.StoredApp, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.StoredApp{}, false, err
	}
	row, err := q.LockAppBySlug(ctx, slug)
	return one(row, err, func(x sqlcgen.LockAppBySlugRow) app.StoredApp { return toApp(x.App) }, "locking an app by slug")
}

// LockApp implements app.Repository.
func (r *Repository) LockApp(ctx context.Context, id ids.ID) (app.StoredApp, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.StoredApp{}, false, err
	}
	row, err := q.LockApp(ctx, uuid(id))
	return one(row, err, func(x sqlcgen.LockAppRow) app.StoredApp { return toApp(x.App) }, "locking an app")
}

// InsertApp implements app.Repository.
func (r *Repository) InsertApp(ctx context.Context, a app.StoredApp) (app.StoredApp, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.StoredApp{}, err
	}
	row, err := q.InsertApp(ctx, sqlcgen.InsertAppParams{
		PrincipalID: uuid(a.PrincipalID), Slug: a.Slug, Manifest: a.Manifest, ManifestVersion: a.ManifestVersion,
		ManifestSha256: a.ManifestSHA256, Publisher: a.Publisher, HomepageUrl: optionalText(a.HomepageURL),
	})
	if err != nil {
		return app.StoredApp{}, mapError(err, "inserting an app")
	}
	return toApp(row), nil
}

// UpdateApp implements app.Repository.
func (r *Repository) UpdateApp(ctx context.Context, a app.StoredApp, at time.Time) (app.StoredApp, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.StoredApp{}, err
	}
	row, err := q.UpdateApp(ctx, sqlcgen.UpdateAppParams{
		PrincipalID: uuid(a.PrincipalID), Manifest: a.Manifest, ManifestVersion: a.ManifestVersion, ManifestSha256: a.ManifestSHA256,
		Publisher: a.Publisher, HomepageUrl: optionalText(a.HomepageURL), UpdatedAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
	if err != nil {
		return app.StoredApp{}, mapError(err, "updating an app")
	}
	return toApp(row), nil
}

// LatestInstallation implements app.Repository.
func (r *Repository) LatestInstallation(ctx context.Context, appID ids.ID) (app.InstallationRecord, bool, error) {
	row, err := r.read(ctx).LatestInstallation(ctx, uuid(appID))
	return one(row, err, func(x sqlcgen.LatestInstallationRow) app.InstallationRecord { return toInstallation(x.AppInstallation) },
		"reading the latest installation")
}

// LockLiveInstallation implements app.Repository.
func (r *Repository) LockLiveInstallation(ctx context.Context, appID ids.ID) (app.InstallationRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.InstallationRecord{}, false, err
	}
	row, err := q.LockLiveInstallation(ctx, uuid(appID))
	return one(row, err, func(x sqlcgen.LockLiveInstallationRow) app.InstallationRecord {
		return toInstallation(x.AppInstallation)
	},
		"locking the installation")
}

// ActiveInstallation implements app.Repository.
func (r *Repository) ActiveInstallation(ctx context.Context, appID ids.ID) (app.InstallationRecord, bool, error) {
	row, err := r.read(ctx).ActiveInstallation(ctx, uuid(appID))
	return one(row, err, func(x sqlcgen.ActiveInstallationRow) app.InstallationRecord { return toInstallation(x.AppInstallation) },
		"reading the active installation")
}

// ShareActiveInstallation implements app.Repository.
func (r *Repository) ShareActiveInstallation(ctx context.Context, appID ids.ID) (app.InstallationRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.InstallationRecord{}, false, err
	}
	row, err := q.ShareActiveInstallation(ctx, uuid(appID))
	return one(row, err, func(x sqlcgen.ShareActiveInstallationRow) app.InstallationRecord {
		return toInstallation(x.AppInstallation)
	},
		"locking the active installation")
}

// InsertInstallation implements app.Repository.
func (r *Repository) InsertInstallation(ctx context.Context, i app.InstallationRecord) (app.InstallationRecord, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.InstallationRecord{}, err
	}
	row, err := q.InsertInstallation(ctx, sqlcgen.InsertInstallationParams{
		ID: uuid(i.ID), AppID: uuid(i.AppID), GrantedScopes: nonNil(i.GrantedScopes), EventSubscriptions: nonNil(i.EventSubscriptions),
		State: string(i.State), InstalledBy: uuid(i.InstalledBy),
	})
	if err != nil {
		return app.InstallationRecord{}, mapError(err, "inserting an installation")
	}
	return toInstallation(row), nil
}

// UpdateInstallationGrants implements app.Repository.
func (r *Repository) UpdateInstallationGrants(ctx context.Context, id ids.ID, g domain.Grants) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.UpdateInstallationGrants(ctx, sqlcgen.UpdateInstallationGrantsParams{
		ID: uuid(id), GrantedScopes: nonNil(g.Scopes), EventSubscriptions: nonNil(g.Events),
	})
	return affected(n, err, "updating the grants of an installation")
}

// SetInstallationState implements app.Repository.
func (r *Repository) SetInstallationState(ctx context.Context, id ids.ID, state domain.State, at time.Time, by ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	p := sqlcgen.SetInstallationStateParams{ID: uuid(id), State: string(state)}
	if state == domain.StateUninstalled {
		p.UninstalledAt, p.UninstalledBy = pgtype.Timestamptz{Time: at, Valid: true}, uuid(by)
	}
	n, err := q.SetInstallationState(ctx, p)
	return affected(n, err, "setting the state of an installation")
}

// ListLive implements app.Repository.
func (r *Repository) ListLive(ctx context.Context, afterSlug string, afterID ids.ID, limit int) ([]app.ListedApp, error) {
	rows, err := r.read(ctx).ListLiveApps(ctx, sqlcgen.ListLiveAppsParams{AfterSlug: afterSlug, AfterID: pgtype.UUID{Bytes: afterID, Valid: true},
		Lim: int32(min(limit, 1<<30))}) //nolint:gosec // G115: bounded above
	if err != nil {
		return nil, mapError(err, "listing apps")
	}
	out := make([]app.ListedApp, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.ListedApp{App: toApp(row.App), Installation: toInstallation(row.AppInstallation)})
	}
	return out, nil
}

// affected maps the result of an update of one row: no row is internal (the caller locked it).
func affected(n int64, err error, what string) error {
	if err != nil {
		return mapError(err, what)
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, "apps: "+what+" found no row")
	}
	return nil
}

// toApp converts a row of apps.
func toApp(a sqlcgen.App) app.StoredApp {
	return app.StoredApp{
		PrincipalID: ids.ID(a.PrincipalID.Bytes), Slug: a.Slug, Manifest: a.Manifest, ManifestVersion: a.ManifestVersion,
		ManifestSHA256: a.ManifestSha256, Publisher: a.Publisher, HomepageURL: a.HomepageUrl.String,
		CreatedAt: a.CreatedAt.Time, UpdatedAt: a.UpdatedAt.Time,
	}
}

// toInstallation converts a row of app_installations.
func toInstallation(i sqlcgen.AppInstallation) app.InstallationRecord {
	r := app.InstallationRecord{
		ID: ids.ID(i.ID.Bytes), AppID: ids.ID(i.AppID.Bytes), GrantedScopes: nonNil(i.GrantedScopes),
		EventSubscriptions: nonNil(i.EventSubscriptions), State: domain.State(i.State), InstalledBy: ids.ID(i.InstalledBy.Bytes),
		InstalledAt: i.InstalledAt.Time,
	}
	if i.UninstalledAt.Valid {
		r.UninstalledAt = i.UninstalledAt.Time
	}
	if i.UninstalledBy.Valid {
		r.UninstalledBy = ids.ID(i.UninstalledBy.Bytes)
	}
	return r
}

// uuid converts an id.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// optionalText is s as nullable text; "" is NULL.
func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// nonNil returns list or an empty list for nil, so text[] is never NULL.
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
