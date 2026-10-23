// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"math"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository is the store of the expiry jobs too.
var _ app.ExpiryStore = (*Repository)(nil)

// locking returns the queries of the ambient transaction for a locking read: FOR UPDATE outside
// a unit of work would lock nothing beyond its own statement.
func (r *Repository) locking(ctx context.Context, limit int) (*sqlcgen.Queries, int32, error) {
	if limit < 1 {
		return nil, 0, errs.New(errs.CodeInternalError, "authz: an expiry batch needs a limit of at least 1")
	}
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, 0, err
	}
	return sqlcgen.New(tx), int32(min(limit, math.MaxInt32)), nil
}

// marked checks that a mark changed every row: fewer means a row was marked concurrently, which
// the lock of the selection rules out, so it is internal and rolls the unit back.
func marked(n int64, want int, what string) error {
	if n != int64(want) {
		return errs.New(errs.CodeInternalError, "authz: "+what+" changed fewer rows than selected")
	}
	return nil
}

// DueExpiredGrants implements app.ExpiryStore.
func (r *Repository) DueExpiredGrants(ctx context.Context, t time.Time, limit int) ([]domain.Grant, error) {
	q, n, err := r.locking(ctx, limit)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListDueExpiredGrants(ctx, sqlcgen.ListDueExpiredGrantsParams{At: timestamp(t), MaxRows: n})
	if err != nil {
		return nil, mapError(err, "reading the expired grants")
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

// MarkGrantsExpiryReported implements app.ExpiryStore.
func (r *Repository) MarkGrantsExpiryReported(ctx context.Context, grantIDs []ids.ID, at time.Time) error {
	q, err := r.rightsWrite(ctx, at)
	if err != nil {
		return err
	}
	n, err := q.MarkGrantsExpiryReported(ctx, sqlcgen.MarkGrantsExpiryReportedParams{At: timestamp(at), Ids: uuids(grantIDs)})
	if err != nil {
		return mapError(err, "marking grants as reported")
	}
	return marked(n, len(uuids(grantIDs)), "marking grants as reported")
}

// DueExpiredMemberships implements app.ExpiryStore.
func (r *Repository) DueExpiredMemberships(ctx context.Context, t time.Time, limit int) ([]domain.MembershipWithGroup, error) {
	q, n, err := r.locking(ctx, limit)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListDueExpiredMemberships(ctx, sqlcgen.ListDueExpiredMembershipsParams{At: timestamp(t), MaxRows: n})
	if err != nil {
		return nil, mapError(err, "reading the expired memberships")
	}
	out := make([]domain.MembershipWithGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.MembershipWithGroup{Membership: toMembership(row.GroupMembership), Group: toGroup(row.Group)})
	}
	return out, nil
}

// MarkMembershipsExpiryReported implements app.ExpiryStore.
func (r *Repository) MarkMembershipsExpiryReported(ctx context.Context, membershipIDs []ids.ID, at time.Time) error {
	q, err := r.rightsWrite(ctx, at)
	if err != nil {
		return err
	}
	n, err := q.MarkMembershipsExpiryReported(ctx, sqlcgen.MarkMembershipsExpiryReportedParams{At: timestamp(at), Ids: uuids(membershipIDs)})
	if err != nil {
		return mapError(err, "marking memberships as reported")
	}
	return marked(n, len(uuids(membershipIDs)), "marking memberships as reported")
}

// DueReminderGrants implements app.ExpiryStore.
func (r *Repository) DueReminderGrants(ctx context.Context, t, until time.Time, limit int) ([]domain.Grant, error) {
	q, n, err := r.locking(ctx, limit)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListDueReminderGrants(ctx, sqlcgen.ListDueReminderGrantsParams{At: timestamp(t), Until: timestamp(until), MaxRows: n})
	if err != nil {
		return nil, mapError(err, "reading the grants to remind")
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

// MarkGrantsExpiryReminded implements app.ExpiryStore.
func (r *Repository) MarkGrantsExpiryReminded(ctx context.Context, grantIDs []ids.ID, at time.Time) error {
	q, err := r.write(ctx, at)
	if err != nil {
		return err
	}
	n, err := q.MarkGrantsExpiryReminded(ctx, sqlcgen.MarkGrantsExpiryRemindedParams{At: timestamp(at), Ids: uuids(grantIDs)})
	if err != nil {
		return mapError(err, "marking grants as reminded")
	}
	return marked(n, len(uuids(grantIDs)), "marking grants as reminded")
}

// DueReminderMemberships implements app.ExpiryStore.
func (r *Repository) DueReminderMemberships(ctx context.Context, t, until time.Time, limit int) ([]domain.MembershipWithGroup, error) {
	q, n, err := r.locking(ctx, limit)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListDueReminderMemberships(ctx, sqlcgen.ListDueReminderMembershipsParams{At: timestamp(t), Until: timestamp(until), MaxRows: n})
	if err != nil {
		return nil, mapError(err, "reading the memberships to remind")
	}
	out := make([]domain.MembershipWithGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.MembershipWithGroup{Membership: toMembership(row.GroupMembership), Group: toGroup(row.Group)})
	}
	return out, nil
}

// MarkMembershipsExpiryReminded implements app.ExpiryStore.
func (r *Repository) MarkMembershipsExpiryReminded(ctx context.Context, membershipIDs []ids.ID, at time.Time) error {
	q, err := r.write(ctx, at)
	if err != nil {
		return err
	}
	n, err := q.MarkMembershipsExpiryReminded(ctx, sqlcgen.MarkMembershipsExpiryRemindedParams{At: timestamp(at), Ids: uuids(membershipIDs)})
	if err != nil {
		return mapError(err, "marking memberships as reminded")
	}
	return marked(n, len(uuids(membershipIDs)), "marking memberships as reminded")
}

// ExpiringGrants implements app.Store (M6-07): a read without lock, inside the ambient
// transaction of ctx when there is one.
func (r *Repository) ExpiringGrants(ctx context.Context, t, until time.Time) ([]domain.Grant, error) {
	rows, err := r.read(ctx).ListExpiringGrants(ctx, sqlcgen.ListExpiringGrantsParams{At: timestamp(t), Until: timestamp(until)})
	if err != nil {
		return nil, mapError(err, "reading the expiring grants")
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

// ExpiringMemberships implements app.Store (M6-07): a read without lock, inside the ambient
// transaction of ctx when there is one.
func (r *Repository) ExpiringMemberships(ctx context.Context, t, until time.Time) ([]domain.MembershipWithGroup, error) {
	rows, err := r.read(ctx).ListExpiringMemberships(ctx, sqlcgen.ListExpiringMembershipsParams{At: timestamp(t), Until: timestamp(until)})
	if err != nil {
		return nil, mapError(err, "reading the expiring memberships")
	}
	out := make([]domain.MembershipWithGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.MembershipWithGroup{Membership: toMembership(row.GroupMembership), Group: toGroup(row.Group)})
	}
	return out, nil
}
