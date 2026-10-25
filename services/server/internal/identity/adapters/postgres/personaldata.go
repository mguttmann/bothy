// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository is the store of the privacy participant of identity (M7-06).
var _ app.PersonalDataStore = (*Repository)(nil)

// PersonalExport implements app.PersonalDataStore: every read in one read-only snapshot, so the
// sections of the export agree with each other.
func (r *Repository) PersonalExport(ctx context.Context, userID ids.ID) (app.PersonalRows, bool, error) {
	var (
		out   app.PersonalRows
		found bool
	)
	err := pg.ReadSnapshot(ctx, r.pool, func(ctx context.Context, db pg.DBTX) error {
		q, id := sqlcgen.New(db), uuid(userID)
		p, err := q.PersonalPrincipal(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		out.Principal = app.ExportPrincipal{
			ID: idOf(p.ID), Kind: p.Kind, Handle: p.Handle, DisplayName: p.DisplayName, Status: p.Status,
			AvatarFileID: idOf(p.AvatarFileID), CreatedAt: timeOf(p.CreatedAt), DisabledAt: timeOf(p.DisabledAt),
			DeletedAt: timeOf(p.DeletedAt),
		}
		out.User = app.ExportUser{Email: p.Email.String, Locale: p.Locale, TimeZone: p.TimeZone, LastActiveAt: timeOf(p.LastActiveAt)}
		return readPersonalLists(ctx, q, id, &out)
	})
	if err != nil {
		return app.PersonalRows{}, false, mapError(err, "reading the personal data of a person")
	}
	return out, found, nil
}

// readPersonalLists reads the lists of the export of the person id into out.
func readPersonalLists(ctx context.Context, q *sqlcgen.Queries, id pgtype.UUID, out *app.PersonalRows) error {
	owners, err := q.PersonalOwners(ctx, id)
	if err != nil {
		return err
	}
	for _, o := range owners {
		out.Owners = append(out.Owners, app.ExportOwner{
			ID: idOf(o.ID), Role: app.RoleOwner, Reason: o.Reason, GrantedAt: timeOf(o.GrantedAt), RevokedAt: timeOf(o.RevokedAt),
		})
	}
	identities, err := q.PersonalIdentities(ctx, id)
	if err != nil {
		return err
	}
	for _, i := range identities {
		out.Identities = append(out.Identities, app.ExportIdentity{
			ID: idOf(i.ID), ProviderID: idOf(i.ProviderID), Subject: i.Subject, Issuer: i.Issuer.String,
			EmailAtLink: i.EmailAtLink.String, LinkedAt: timeOf(i.LinkedAt), LastLoginAt: timeOf(i.LastLoginAt),
			UnlinkedAt: timeOf(i.UnlinkedAt),
		})
	}
	passkeys, err := q.PersonalPasskeys(ctx, id)
	if err != nil {
		return err
	}
	for _, k := range passkeys {
		out.Passkeys = append(out.Passkeys, app.ExportPasskey{
			ID: idOf(k.ID), Name: k.Name, Transports: k.Transports, CreatedAt: timeOf(k.CreatedAt),
			LastUsedAt: timeOf(k.LastUsedAt), RevokedAt: timeOf(k.RevokedAt),
		})
	}
	creds, err := q.PersonalLocalCredentials(ctx, id)
	if err != nil {
		return err
	}
	for _, c := range creds {
		out.LocalCredentials = append(out.LocalCredentials, app.ExportLocalCredential{
			IdentityID: idOf(c.IdentityID), PasswordChangedAt: timeOf(c.PasswordChangedAt),
			TOTPEnrolled: c.TotpEnabledAt.Valid, TOTPEnabledAt: timeOf(c.TotpEnabledAt),
		})
	}
	return readPersonalAccess(ctx, q, id, out)
}

// readPersonalAccess reads devices, sessions and personal API tokens of the person id into out.
func readPersonalAccess(ctx context.Context, q *sqlcgen.Queries, id pgtype.UUID, out *app.PersonalRows) error {
	devices, err := q.PersonalDevices(ctx, id)
	if err != nil {
		return err
	}
	for _, d := range devices {
		out.Devices = append(out.Devices, app.ExportDevice{
			ID: idOf(d.ID), Name: d.Name, Platform: d.Platform, AppVersion: d.AppVersion.String, State: d.State,
			CreatedAt: timeOf(d.CreatedAt), ApprovedAt: timeOf(d.ApprovedAt), LastSeenAt: timeOf(d.LastSeenAt),
			RevokedAt: timeOf(d.RevokedAt), RevokeReason: d.RevokeReason.String,
		})
	}
	sessions, err := q.PersonalSessions(ctx, id)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		e := app.ExportSession{
			ID: idOf(s.ID), DeviceID: idOf(s.DeviceID), ClientComponent: s.ClientComponent, AuthMethod: s.AuthMethod,
			CreatedAt: timeOf(s.CreatedAt), ExpiresAt: timeOf(s.ExpiresAt), LastSeenAt: timeOf(s.LastSeenAt),
			RevokedAt: timeOf(s.RevokedAt), RevokeReason: s.RevokeReason.String,
		}
		if s.LastIp != nil {
			e.LastIP = s.LastIp.String()
		}
		out.Sessions = append(out.Sessions, e)
	}
	tokens, err := q.PersonalAPITokens(ctx, id)
	if err != nil {
		return err
	}
	for _, t := range tokens {
		out.APITokens = append(out.APITokens, app.ExportAPIToken{
			ID: idOf(t.ID), Name: t.Name, CreatedAt: timeOf(t.CreatedAt), LastUsedAt: timeOf(t.LastUsedAt),
			ExpiresAt: timeOf(t.ExpiresAt), RevokedAt: timeOf(t.RevokedAt),
		})
	}
	return nil
}

// eraseStatements are the deletes of EraseSecurityRows, children before their parents: tokens
// before families, challenges before the sessions and codes they name, local credentials before
// their identities.
var eraseStatements = []func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID) (int64, error){
	(*sqlcgen.Queries).EraseRefreshTokensOfUser,
	(*sqlcgen.Queries).EraseRefreshFamiliesOfUser,
	(*sqlcgen.Queries).EraseChallengesOfUser,
	(*sqlcgen.Queries).ErasePairingsOfUser,
	(*sqlcgen.Queries).EraseLoginFlowsOfUser,
	(*sqlcgen.Queries).EraseEnrollmentCodesOfUser,
	(*sqlcgen.Queries).EraseAPITokensOfUser,
	(*sqlcgen.Queries).ErasePasskeysOfUser,
	(*sqlcgen.Queries).EraseLocalCredentialsOfUser,
	(*sqlcgen.Queries).EraseIdentitiesOfUser,
	(*sqlcgen.Queries).EraseDeviceKeysOfUser,
}

// EraseSecurityRows implements app.PersonalDataStore.
func (r *Repository) EraseSecurityRows(ctx context.Context, userID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	for _, del := range eraseStatements {
		if _, err := del(q, ctx, uuid(userID)); err != nil {
			return mapError(err, "deleting the security rows of a person")
		}
	}
	return nil
}

// EraseHeldRows implements app.PersonalDataStore with pg.PurgeEach: each session and then each
// device in its own savepoint, so the foreign key of another module's row decides what stays
// (03 principle 12: identity never reads the other table). Sessions go first, because a session
// holds its device (ON DELETE RESTRICT).
func (r *Repository) EraseHeldRows(ctx context.Context, userID ids.ID) (int, int, error) {
	q := sqlcgen.New(r.pool)
	sessions, err := q.SessionIDsOfUser(ctx, uuid(userID))
	if err != nil {
		return 0, 0, mapError(err, "listing the sessions of a person")
	}
	if _, err := pg.PurgeEach(ctx, r.pool, sessions, func(ctx context.Context, db pg.DBTX, id pgtype.UUID) (int64, error) {
		return sqlcgen.New(db).EraseSessionOfUser(ctx, sqlcgen.EraseSessionOfUserParams{ID: id, UserID: uuid(userID)})
	}); err != nil {
		return 0, 0, mapError(err, "deleting the sessions of a person")
	}
	// A child session goes with its parent (ON DELETE CASCADE), so count what is left.
	keptSessions, err := q.SessionIDsOfUser(ctx, uuid(userID))
	if err != nil {
		return 0, 0, mapError(err, "listing the sessions of a person")
	}
	devices, err := q.DeviceIDsOfUser(ctx, uuid(userID))
	if err != nil {
		return 0, 0, mapError(err, "listing the devices of a person")
	}
	deleted, err := pg.PurgeEach(ctx, r.pool, devices, func(ctx context.Context, db pg.DBTX, id pgtype.UUID) (int64, error) {
		return sqlcgen.New(db).EraseDeviceOfUser(ctx, sqlcgen.EraseDeviceOfUserParams{ID: id, UserID: uuid(userID)})
	})
	if err != nil {
		return 0, 0, mapError(err, "deleting the devices of a person")
	}
	return len(keptSessions), len(devices) - deleted, nil
}

// ScrubKeptRows implements app.PersonalDataStore.
func (r *Repository) ScrubKeptRows(ctx context.Context, userID ids.ID, deviceName string) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if _, err := q.ScrubKeptSessionsOfUser(ctx, uuid(userID)); err != nil {
		return mapError(err, "clearing the kept sessions of a person")
	}
	if _, err := q.ScrubKeptDevicesOfUser(ctx, sqlcgen.ScrubKeptDevicesOfUserParams{UserID: uuid(userID), Name: deviceName}); err != nil {
		return mapError(err, "clearing the kept devices of a person")
	}
	return nil
}

// Pseudonymize implements app.PersonalDataStore.
func (r *Repository) Pseudonymize(ctx context.Context, userID ids.ID, handle, displayName string, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.PseudonymizePrincipal(ctx, sqlcgen.PseudonymizePrincipalParams{
		UserID: uuid(userID), Handle: handle, DisplayName: displayName, DeletedAt: ts(at),
	})
	if err != nil {
		return false, mapError(err, "pseudonymizing a principal")
	}
	if n == 0 {
		return false, nil
	}
	if _, err := q.PseudonymizeUser(ctx, uuid(userID)); err != nil {
		return false, mapError(err, "pseudonymizing a person")
	}
	return true, nil
}
