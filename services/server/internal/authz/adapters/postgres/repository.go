// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the authz module: the sqlc queries in queries/,
// the generated code in sqlcgen/ and Repository, which implements app.Store over the tables
// groups, group_memberships and grants (docs/spec/03-data-model.md section 4.3). Only this
// package runs SQL on those tables (03 "Principles" rule 12). It has no delete query and no general
// update of grants: grants are only revoked (R-DM-09).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	grants, err := repo.ActiveGrantsOfPrincipal(ctx, sam)
package postgres

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Store with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the authz application layer.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads
// and writes inside one use its transaction (pg.Querier, pg.Tx).
func NewRepository(pool *pg.Pool) *Repository {
	return &Repository{pool: pool}
}

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write: only the ambient transaction of ctx, because a write
// outside a unit of work would skip its phases (02 section 8.1). at is the time the write
// stores; a zero time is a programming error of the caller, not a reason to fall back to the
// database clock, because one clock decides validity.
func (r *Repository) write(ctx context.Context, at time.Time) (*sqlcgen.Queries, error) {
	if at.IsZero() {
		return nil, errs.New(errs.CodeInternalError, "authz: a write without a time")
	}
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// rightsWrite is write for a statement that changes rights (memberships, grants, the deletion of
// a group): it also needs a unit of work declared as a rights change, so the rights begin hook
// has locked permission_version first and the rights check hook verifies I1 and SR8 (M1-29).
func (r *Repository) rightsWrite(ctx context.Context, at time.Time) (*sqlcgen.Queries, error) {
	q, err := r.write(ctx, at)
	if err != nil {
		return nil, err
	}
	if !txn.InRightsChange(ctx) {
		return nil, errs.New(errs.CodeInternalError, "authz: a rights write outside a unit of work declared as a rights change")
	}
	return q, nil
}

// InsertGroup implements app.Store.
func (r *Repository) InsertGroup(ctx context.Context, g domain.Group) error {
	q, err := r.write(ctx, g.CreatedAt)
	if err != nil {
		return err
	}
	err = q.InsertGroup(ctx, sqlcgen.InsertGroupParams{
		ID:          uuid(g.ID),
		Name:        g.Name,
		Description: g.Description,
		Source:      g.Source,
		ProviderID:  optionalUUID(g.ProviderID),
		ExternalRef: optionalText(g.ExternalRef),
		CreatedBy:   uuid(g.CreatedBy),
		CreatedAt:   timestamp(g.CreatedAt),
	})
	if err != nil {
		return mapError(err, "inserting a group")
	}
	return nil
}

// GetGroup implements app.Store.
func (r *Repository) GetGroup(ctx context.Context, id ids.ID) (domain.Group, bool, error) {
	row, err := r.read(ctx).GetGroup(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Group{}, false, nil
	}
	if err != nil {
		return domain.Group{}, false, mapError(err, "reading a group")
	}
	return toGroup(row.Group), true, nil
}

// SoftDeleteGroup implements app.Store.
func (r *Repository) SoftDeleteGroup(ctx context.Context, id ids.ID, at time.Time) (bool, error) {
	q, err := r.rightsWrite(ctx, at)
	if err != nil {
		return false, err
	}
	n, err := q.SoftDeleteGroup(ctx, sqlcgen.SoftDeleteGroupParams{ID: uuid(id), DeletedAt: timestamp(at)})
	if err != nil {
		return false, mapError(err, "deleting a group")
	}
	return n > 0, nil
}

// InsertMembership implements app.Store.
func (r *Repository) InsertMembership(ctx context.Context, m domain.Membership) error {
	q, err := r.rightsWrite(ctx, m.AddedAt)
	if err != nil {
		return err
	}
	err = q.InsertMembership(ctx, sqlcgen.InsertMembershipParams{
		ID:        uuid(m.ID),
		GroupID:   uuid(m.GroupID),
		UserID:    uuid(m.UserID),
		AddedBy:   uuid(m.AddedBy),
		AddedAt:   timestamp(m.AddedAt),
		ExpiresAt: optionalTimestamp(m.ExpiresAt),
	})
	if err != nil {
		return mapError(err, "inserting a group membership")
	}
	return nil
}

// RemoveMembership implements app.Store.
func (r *Repository) RemoveMembership(ctx context.Context, id, by ids.ID, at time.Time) (bool, error) {
	q, err := r.rightsWrite(ctx, at)
	if err != nil {
		return false, err
	}
	n, err := q.RemoveMembership(ctx, sqlcgen.RemoveMembershipParams{
		ID: uuid(id), RemovedBy: optionalUUID(by), RemovedAt: timestamp(at),
	})
	if err != nil {
		return false, mapError(err, "removing a group membership")
	}
	return n > 0, nil
}

// ActiveMembershipsOfUser implements app.Store.
func (r *Repository) ActiveMembershipsOfUser(ctx context.Context, userID ids.ID) ([]domain.MembershipWithGroup, error) {
	rows, err := r.read(ctx).ListActiveMembershipsOfUser(ctx, uuid(userID))
	if err != nil {
		return nil, mapError(err, "reading the group memberships of a person")
	}
	out := make([]domain.MembershipWithGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.MembershipWithGroup{Membership: toMembership(row.GroupMembership), Group: toGroup(row.Group)})
	}
	return out, nil
}

// InsertGrant implements app.Store.
func (r *Repository) InsertGrant(ctx context.Context, g domain.Grant) error {
	q, err := r.rightsWrite(ctx, g.GrantedAt)
	if err != nil {
		return err
	}
	err = q.InsertGrant(ctx, sqlcgen.InsertGrantParams{
		ID:                 uuid(g.ID),
		SubjectKind:        string(g.Subject.Kind),
		SubjectPrincipalID: optionalUUID(g.Subject.PrincipalID),
		SubjectGroupID:     optionalUUID(g.Subject.GroupID),
		TargetKind:         string(g.Target.Kind),
		TargetSpaceID:      optionalUUID(g.Target.SpaceID),
		TargetChannelID:    optionalUUID(g.Target.ChannelID),
		TargetHostID:       optionalUUID(g.Target.HostID),
		Level:              optionalText(g.Level.String()),
		Capabilities:       g.Caps.Names(),
		ExpiresAt:          optionalTimestamp(g.ExpiresAt),
		Reason:             g.Reason,
		GrantedBy:          uuid(g.GrantedBy),
		GrantedAt:          timestamp(g.GrantedAt),
	})
	if err != nil {
		return mapError(err, "inserting a grant")
	}
	return nil
}

// GetGrant implements app.Store.
func (r *Repository) GetGrant(ctx context.Context, id ids.ID) (domain.Grant, bool, error) {
	row, err := r.read(ctx).GetGrant(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Grant{}, false, nil
	}
	if err != nil {
		return domain.Grant{}, false, mapError(err, "reading a grant")
	}
	g, err := toGrant(row.Grant)
	if err != nil {
		return domain.Grant{}, false, err
	}
	return g, true, nil
}

// RevokeGrant implements app.Store.
func (r *Repository) RevokeGrant(ctx context.Context, id, by ids.ID, reason string, at time.Time) (domain.Grant, bool, error) {
	q, err := r.rightsWrite(ctx, at)
	if err != nil {
		return domain.Grant{}, false, err
	}
	row, err := q.RevokeGrant(ctx, sqlcgen.RevokeGrantParams{
		ID: uuid(id), RevokedAt: timestamp(at), RevokedBy: optionalUUID(by), RevokeReason: optionalText(reason),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Grant{}, false, nil
	}
	if err != nil {
		return domain.Grant{}, false, mapError(err, "revoking a grant")
	}
	g, err := toGrant(row)
	if err != nil {
		return domain.Grant{}, false, err
	}
	return g, true, nil
}

// ActiveGrantsOfPrincipal implements app.Store.
func (r *Repository) ActiveGrantsOfPrincipal(ctx context.Context, principalID ids.ID) ([]domain.Grant, error) {
	rows, err := r.read(ctx).ListActiveGrantsOfPrincipal(ctx, uuid(principalID))
	if err != nil {
		return nil, mapError(err, "reading the grants of a principal")
	}
	out := make([]domain.Grant, 0, len(rows))
	for _, row := range rows {
		g, err := toGrant(row.Grant)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// ActiveGrantsOfGroups implements app.Store.
func (r *Repository) ActiveGrantsOfGroups(ctx context.Context, groupIDs []ids.ID) ([]domain.Grant, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	keys := make([]pgtype.UUID, 0, len(groupIDs))
	for _, id := range slices.Compact(slices.SortedFunc(slices.Values(groupIDs), compareIDs)) {
		keys = append(keys, uuid(id))
	}
	rows, err := r.read(ctx).ListActiveGrantsOfGroups(ctx, keys)
	if err != nil {
		return nil, mapError(err, "reading the grants of groups")
	}
	out := make([]domain.Grant, 0, len(rows))
	for _, row := range rows {
		g, err := toGrant(row.Grant)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// UpdateGroup implements app.Store.
func (r *Repository) UpdateGroup(ctx context.Context, id ids.ID, name, description string) (bool, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(tx).UpdateGroup(ctx, sqlcgen.UpdateGroupParams{ID: uuid(id), Name: name, Description: description})
	if err != nil {
		return false, mapError(err, "updating a group")
	}
	return n > 0, nil
}

// ActiveMembershipsOfGroups implements app.Store.
func (r *Repository) ActiveMembershipsOfGroups(ctx context.Context, groupIDs []ids.ID) ([]domain.MembershipWithGroup, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	rows, err := r.read(ctx).ListActiveMembershipsOfGroups(ctx, uuids(groupIDs))
	if err != nil {
		return nil, mapError(err, "reading the memberships of groups")
	}
	out := make([]domain.MembershipWithGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.MembershipWithGroup{Membership: toMembership(row.GroupMembership), Group: toGroup(row.Group)})
	}
	return out, nil
}

// MembershipsOfUserInGroup implements app.Store.
func (r *Repository) MembershipsOfUserInGroup(ctx context.Context, groupID, userID ids.ID) ([]domain.Membership, error) {
	rows, err := r.read(ctx).ListMembershipsOfUserInGroup(ctx, sqlcgen.ListMembershipsOfUserInGroupParams{GroupID: uuid(groupID), UserID: uuid(userID)})
	if err != nil {
		return nil, mapError(err, "reading the memberships of a person in a group")
	}
	out := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMembership(row.GroupMembership))
	}
	return out, nil
}

// ActiveTriggerGrants implements app.Store.
func (r *Repository) ActiveTriggerGrants(ctx context.Context) ([]domain.Grant, error) {
	rows, err := r.read(ctx).ListActiveTriggerGrants(ctx)
	if err != nil {
		return nil, mapError(err, "reading the trigger grants")
	}
	out := make([]domain.Grant, 0, len(rows))
	for _, row := range rows {
		g, err := toGrant(row.Grant)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// ActiveGrantsOnTargets implements app.Store.
func (r *Repository) ActiveGrantsOnTargets(ctx context.Context, spaceIDs, channelIDs []ids.ID) ([]domain.Grant, error) {
	rows, err := r.read(ctx).ListActiveGrantsOnTargets(ctx, sqlcgen.ListActiveGrantsOnTargetsParams{
		SpaceIds: uuids(spaceIDs), ChannelIds: uuids(channelIDs),
	})
	if err != nil {
		return nil, mapError(err, "reading the grants on targets")
	}
	out := make([]domain.Grant, 0, len(rows))
	for _, row := range rows {
		g, err := toGrant(row.Grant)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// uuids converts ids to sorted unique parameters; nil becomes an empty array, never NULL.
func uuids(list []ids.ID) []pgtype.UUID {
	keys := make([]pgtype.UUID, 0, len(list))
	for _, id := range slices.Compact(slices.SortedFunc(slices.Values(list), compareIDs)) {
		keys = append(keys, uuid(id))
	}
	return keys
}

// toGroup converts a row of groups. The database checks hold the shape, so it is wrapped
// without a second validation.
func toGroup(g sqlcgen.Group) domain.Group {
	return domain.Group{
		ID:          idOf(g.ID),
		Name:        g.Name,
		Description: g.Description,
		Source:      g.Source,
		ProviderID:  idOf(g.ProviderID),
		ExternalRef: g.ExternalRef.String,
		CreatedBy:   idOf(g.CreatedBy),
		CreatedAt:   timeOf(g.CreatedAt),
		DeletedAt:   timeOf(g.DeletedAt),
	}
}

// toMembership converts a row of group_memberships.
func toMembership(m sqlcgen.GroupMembership) domain.Membership {
	return domain.Membership{
		ID:               idOf(m.ID),
		GroupID:          idOf(m.GroupID),
		UserID:           idOf(m.UserID),
		AddedBy:          idOf(m.AddedBy),
		AddedAt:          timeOf(m.AddedAt),
		ExpiresAt:        timeOf(m.ExpiresAt),
		ExpiryRemindedAt: timeOf(m.ExpiryRemindedAt),
		ExpiryReportedAt: timeOf(m.ExpiryReportedAt),
		RemovedAt:        timeOf(m.RemovedAt),
		RemovedBy:        idOf(m.RemovedBy),
	}
}

// toGrant converts a row of grants. A level or capability the domain does not know means the
// schema and the code disagree, which is internal.
func toGrant(g sqlcgen.Grant) (domain.Grant, error) {
	level := domain.LevelNone
	if g.Level.Valid {
		l, err := domain.ParseLevel(g.Level.String)
		if err != nil {
			return domain.Grant{}, errs.Wrap(errs.CodeInternalError, err, "authz: a stored grant has an unknown level")
		}
		level = l
	}
	caps, err := domain.ParseCaps(g.Capabilities)
	if err != nil {
		return domain.Grant{}, errs.Wrap(errs.CodeInternalError, err, "authz: a stored grant has an unknown capability")
	}
	return domain.Grant{
		ID: idOf(g.ID),
		Subject: domain.Subject{
			Kind: domain.SubjectKind(g.SubjectKind), PrincipalID: idOf(g.SubjectPrincipalID), GroupID: idOf(g.SubjectGroupID),
		},
		Target: domain.Target{
			Kind: domain.TargetKind(g.TargetKind), SpaceID: idOf(g.TargetSpaceID),
			ChannelID: idOf(g.TargetChannelID), HostID: idOf(g.TargetHostID),
		},
		Level:            level,
		Caps:             caps,
		ExpiresAt:        timeOf(g.ExpiresAt),
		ExpiryRemindedAt: timeOf(g.ExpiryRemindedAt),
		ExpiryReportedAt: timeOf(g.ExpiryReportedAt),
		Reason:           g.Reason,
		GrantedBy:        idOf(g.GrantedBy),
		GrantedAt:        timeOf(g.GrantedAt),
		RevokedAt:        timeOf(g.RevokedAt),
		RevokedBy:        idOf(g.RevokedBy),
		RevokeReason:     g.RevokeReason.String,
	}, nil
}

// uuid converts an id for a parameter.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// optionalUUID converts an optional id: the zero ID becomes NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// optionalText converts an optional text: the empty string becomes NULL.
func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// timestamp converts a time for a parameter.
func timestamp(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// optionalTimestamp converts an optional time: the zero time becomes NULL.
func optionalTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// idOf converts a column; NULL becomes the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// timeOf converts a column; NULL becomes the zero time. Times are UTC (03 "Principles" rule 2).
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}

// compareIDs orders ids by their bytes, the creation order of UUIDv7.
func compareIDs(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }
