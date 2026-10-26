// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository gives the participant of M7-06 the access history of a person.
var _ app.PersonalDataStore = (*Repository)(nil)

// PersonalAccess implements app.PersonalDataStore: every grant whose subject is person and every
// membership of person with its group, revoked and removed rows included, ascending by id. Both
// lists come from one read-only snapshot (pg.ReadSnapshot), so a grant and a membership written
// between the two reads cannot make the export disagree with itself. ctx carries no unit of work.
//
// Errors: storage errors as kernel/errs errors; an unknown stored level or capability is internal.
func (r *Repository) PersonalAccess(ctx context.Context, person ids.ID) ([]domain.Grant, []domain.MembershipWithGroup, error) {
	var grants []domain.Grant
	var members []domain.MembershipWithGroup
	err := pg.ReadSnapshot(ctx, r.pool, func(ctx context.Context, db pg.DBTX) error {
		q := sqlcgen.New(db)
		grows, err := q.ListGrantsOfPrincipalHistory(ctx, uuid(person))
		if err != nil {
			return mapError(err, "reading the grant history of a person")
		}
		for _, row := range grows {
			g, err := toGrant(row.Grant)
			if err != nil {
				return err
			}
			grants = append(grants, g)
		}
		mrows, err := q.ListMembershipsOfUserHistory(ctx, uuid(person))
		if err != nil {
			return mapError(err, "reading the membership history of a person")
		}
		for _, row := range mrows {
			members = append(members, domain.MembershipWithGroup{Membership: toMembership(row.GroupMembership), Group: toGroup(row.Group)})
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return grants, members, nil
}
