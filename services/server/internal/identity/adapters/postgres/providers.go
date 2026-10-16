// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time checks: Repository is the provider and identity store of the application layer.
var (
	_ app.ProviderStore = (*Repository)(nil)
	_ app.IdentityStore = (*Repository)(nil)
)

// sqlstateRestrictViolation is a delete that an ON DELETE RESTRICT foreign key refuses.
const sqlstateRestrictViolation = "23001"

// inUnit reports whether ctx carries a unit of work, where the login reads take share locks.
func inUnit(ctx context.Context) bool {
	_, err := pg.Tx(ctx)
	return err == nil
}

// InsertProvider implements app.ProviderStore.
func (r *Repository) InsertProvider(ctx context.Context, p identity.Provider) (identity.Provider, error) {
	q, err := r.write(ctx)
	if err != nil {
		return identity.Provider{}, err
	}
	row, err := q.InsertProvider(ctx, sqlcgen.InsertProviderParams{
		ID: uuid(p.ID), Kind: string(p.Kind), Name: p.Name, Config: p.Config, Enabled: p.Enabled,
		AllowProvisioning: p.AllowProvisioning, At: ts(p.UpdatedAt),
	})
	if err != nil {
		return identity.Provider{}, mapError(err, "inserting an identity provider")
	}
	return toProvider(row), nil
}

// providerRow maps the result of a single-row provider query.
func providerRow(row sqlcgen.IdentityProvider, err error, what string) (identity.Provider, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Provider{}, false, nil
	}
	if err != nil {
		return identity.Provider{}, false, mapError(err, what)
	}
	return toProvider(row), true, nil
}

// Provider implements app.ProviderStore.
func (r *Repository) Provider(ctx context.Context, id ids.ID) (identity.Provider, bool, error) {
	row, err := r.read(ctx).GetProvider(ctx, uuid(id))
	return providerRow(row, err, "reading an identity provider")
}

// LockProvider implements app.ProviderStore.
func (r *Repository) LockProvider(ctx context.Context, id ids.ID) (identity.Provider, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return identity.Provider{}, false, err
	}
	row, err := q.LockProvider(ctx, uuid(id))
	return providerRow(row, err, "locking an identity provider")
}

// EnabledProvider implements app.ProviderStore: with a share lock inside a unit of work.
func (r *Repository) EnabledProvider(ctx context.Context, id ids.ID) (identity.Provider, bool, error) {
	q := r.read(ctx)
	var row sqlcgen.IdentityProvider
	var err error
	if inUnit(ctx) {
		row, err = q.ShareEnabledProvider(ctx, uuid(id))
	} else {
		row, err = q.GetEnabledProvider(ctx, uuid(id))
	}
	return providerRow(row, err, "reading an enabled identity provider")
}

// Providers implements app.ProviderStore.
func (r *Repository) Providers(ctx context.Context, enabledOnly bool) ([]identity.Provider, error) {
	q := r.read(ctx)
	var rows []sqlcgen.IdentityProvider
	var err error
	if enabledOnly {
		rows, err = q.ListEnabledProviders(ctx)
	} else {
		rows, err = q.ListProviders(ctx)
	}
	if err != nil {
		return nil, mapError(err, "listing identity providers")
	}
	out := make([]identity.Provider, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProvider(row))
	}
	return out, nil
}

// UpdateProvider implements app.ProviderStore.
func (r *Repository) UpdateProvider(ctx context.Context, p identity.Provider) (identity.Provider, error) {
	q, err := r.write(ctx)
	if err != nil {
		return identity.Provider{}, err
	}
	row, err := q.UpdateProvider(ctx, sqlcgen.UpdateProviderParams{
		ID: uuid(p.ID), Name: p.Name, Config: p.Config, Enabled: p.Enabled, AllowProvisioning: p.AllowProvisioning,
		UpdatedAt: ts(p.UpdatedAt),
	})
	if err != nil {
		return identity.Provider{}, mapError(err, "updating an identity provider")
	}
	return toProvider(row), nil
}

// DeleteProvider implements app.ProviderStore: the restrict violation of user_identities or groups
// is conflict with the hint of 08 section 3.3, without the constraint name.
func (r *Repository) DeleteProvider(ctx context.Context, id ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.DeleteProvider(ctx, uuid(id))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == sqlstateRestrictViolation || pgErr.Code == sqlstateForeignKeyViolation) {
		return false, errs.Wrap(errs.CodeConflict, err, "Identities or groups still reference the identity provider.").
			WithReason("provider_in_use").WithHint("disable the provider instead")
	}
	if err != nil {
		return false, mapError(err, "deleting an identity provider")
	}
	return n > 0, nil
}

// LiveSessionsOfProvider implements app.ProviderStore.
func (r *Repository) LiveSessionsOfProvider(ctx context.Context, id ids.ID, now time.Time) ([]ids.ID, error) {
	rows, err := r.read(ctx).LiveSessionsOfProvider(ctx, sqlcgen.LiveSessionsOfProviderParams{ProviderID: uuid(id), Now: ts(now)})
	if err != nil {
		return nil, mapError(err, "reading the sessions of an identity provider")
	}
	return idsOf(rows), nil
}

// InsertIdentity implements app.IdentityStore.
func (r *Repository) InsertIdentity(ctx context.Context, rec identity.Identity) (identity.Identity, error) {
	q, err := r.write(ctx)
	if err != nil {
		return identity.Identity{}, err
	}
	row, err := q.InsertIdentity(ctx, sqlcgen.InsertIdentityParams{
		ID: uuid(rec.ID), UserID: uuid(rec.UserID), ProviderID: uuid(rec.ProviderID), Subject: rec.Subject,
		Issuer: optionalText(rec.Issuer), EmailAtLink: optionalText(rec.EmailAtLink), LinkedAt: ts(rec.LinkedAt),
	})
	if err != nil {
		return identity.Identity{}, mapError(err, "linking an identity")
	}
	out := toIdentity(row)
	out.ProviderKind = rec.ProviderKind
	return out, nil
}

// ActiveIdentityBySubject implements app.IdentityStore: with a share lock inside a unit of work.
func (r *Repository) ActiveIdentityBySubject(ctx context.Context, providerID ids.ID, subject string) (identity.Identity, bool, error) {
	q := r.read(ctx)
	var row sqlcgen.UserIdentity
	var err error
	if inUnit(ctx) {
		row, err = q.ShareActiveIdentityBySubject(ctx, sqlcgen.ShareActiveIdentityBySubjectParams{ProviderID: uuid(providerID), Subject: subject})
	} else {
		row, err = q.GetActiveIdentityBySubject(ctx, sqlcgen.GetActiveIdentityBySubjectParams{ProviderID: uuid(providerID), Subject: subject})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Identity{}, false, nil
	}
	if err != nil {
		return identity.Identity{}, false, mapError(err, "reading an identity")
	}
	return toIdentity(row), true, nil
}

// IdentityOfUser implements app.IdentityStore.
func (r *Repository) IdentityOfUser(ctx context.Context, userID, id ids.ID) (app.IdentityRecord, bool, error) {
	row, err := r.read(ctx).GetIdentityOfUser(ctx, sqlcgen.GetIdentityOfUserParams{UserID: uuid(userID), ID: uuid(id)})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.IdentityRecord{}, false, nil
	}
	if err != nil {
		return app.IdentityRecord{}, false, mapError(err, "reading an identity")
	}
	return identityRecord(row.UserIdentity, row.ProviderKind, row.ProviderEnabled), true, nil
}

// ActiveIdentitiesOfUser implements app.IdentityStore.
func (r *Repository) ActiveIdentitiesOfUser(ctx context.Context, userID ids.ID) ([]app.IdentityRecord, error) {
	rows, err := r.read(ctx).ListActiveIdentitiesOfUser(ctx, uuid(userID))
	if err != nil {
		return nil, mapError(err, "listing identities")
	}
	out := make([]app.IdentityRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, identityRecord(row.UserIdentity, row.ProviderKind, row.ProviderEnabled))
	}
	return out, nil
}

// LockActiveIdentitiesOfUser implements app.IdentityStore.
func (r *Repository) LockActiveIdentitiesOfUser(ctx context.Context, userID ids.ID) ([]ids.ID, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockActiveIdentitiesOfUser(ctx, uuid(userID))
	if err != nil {
		return nil, mapError(err, "locking identities")
	}
	return idsOf(rows), nil
}

// CountOtherIdentitiesAtEnabledProviders implements app.IdentityStore.
func (r *Repository) CountOtherIdentitiesAtEnabledProviders(ctx context.Context, userID, except ids.ID) (int, error) {
	n, err := r.read(ctx).CountOtherIdentitiesAtEnabledProviders(ctx, sqlcgen.CountOtherIdentitiesAtEnabledProvidersParams{
		UserID: uuid(userID), ExceptID: uuid(except),
	})
	if err != nil {
		return 0, mapError(err, "counting identities")
	}
	return int(n), nil
}

// CountPairingDevices implements app.IdentityStore.
func (r *Repository) CountPairingDevices(ctx context.Context, userID ids.ID) (int, error) {
	n, err := r.read(ctx).CountPairingDevicesOfUser(ctx, uuid(userID))
	if err != nil {
		return 0, mapError(err, "counting devices")
	}
	return int(n), nil
}

// UnlinkIdentity implements app.IdentityStore.
func (r *Repository) UnlinkIdentity(ctx context.Context, id ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.UnlinkIdentity(ctx, sqlcgen.UnlinkIdentityParams{ID: uuid(id), At: ts(at)})
	if err != nil {
		return false, mapError(err, "unlinking an identity")
	}
	return n > 0, nil
}

// MarkIdentityLogin implements app.IdentityStore.
func (r *Repository) MarkIdentityLogin(ctx context.Context, id ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.MarkIdentityLogin(ctx, sqlcgen.MarkIdentityLoginParams{ID: uuid(id), At: ts(at)})
	if err != nil {
		return false, mapError(err, "marking a login")
	}
	return n > 0, nil
}

// LiveSessionsOfIdentity implements app.IdentityStore.
func (r *Repository) LiveSessionsOfIdentity(ctx context.Context, id ids.ID, now time.Time) ([]ids.ID, error) {
	rows, err := r.read(ctx).LiveSessionsOfIdentity(ctx, sqlcgen.LiveSessionsOfIdentityParams{IdentityID: uuid(id), Now: ts(now)})
	if err != nil {
		return nil, mapError(err, "reading the sessions of an identity")
	}
	return idsOf(rows), nil
}

// toProvider converts a row of identity_providers.
func toProvider(p sqlcgen.IdentityProvider) identity.Provider {
	return identity.Provider{
		ID: idOf(p.ID), Kind: identity.ProviderKind(p.Kind), Name: p.Name, Config: append([]byte(nil), p.Config...),
		Enabled: p.Enabled, AllowProvisioning: p.AllowProvisioning, CreatedAt: timeOf(p.CreatedAt), UpdatedAt: timeOf(p.UpdatedAt),
	}
}

// toIdentity converts a row of user_identities (without the provider kind).
func toIdentity(i sqlcgen.UserIdentity) identity.Identity {
	return identity.Identity{
		ID: idOf(i.ID), UserID: idOf(i.UserID), ProviderID: idOf(i.ProviderID), Subject: i.Subject, Issuer: i.Issuer.String,
		EmailAtLink: i.EmailAtLink.String, LinkedAt: timeOf(i.LinkedAt), LastLoginAt: timeOf(i.LastLoginAt),
	}
}

// identityRecord converts a row of user_identities with its provider's kind and state.
func identityRecord(i sqlcgen.UserIdentity, kind string, enabled bool) app.IdentityRecord {
	out := app.IdentityRecord{Identity: toIdentity(i), ProviderEnabled: enabled, UnlinkedAt: timeOf(i.UnlinkedAt)}
	out.ProviderKind = identity.ProviderKind(kind)
	return out
}

// optionalText converts "" to NULL.
func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// idsOf converts a list of uuids.
func idsOf(rows []pgtype.UUID) []ids.ID {
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out
}
