// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// racyStore runs between once after the first GetGrant: a concurrent call that revokes the grant
// between the two units of RevokeGrant.
type racyStore struct {
	app.Store
	once    sync.Once
	between func()
}

func (s *racyStore) GetGrant(ctx context.Context, id ids.ID) (domain.Grant, bool, error) {
	g, ok, err := s.Store.GetGrant(ctx, id)
	s.once.Do(s.between)
	return g, ok, err
}

// TestRevokeGrant_RacedRevocationIsWithoutEffect (red team M1-29 minor 1): when another call
// revokes the grant between the two units, the call ends without effect: no second rights
// version, no event, one allow row with no_change.
func TestRevokeGrant_RacedRevocationIsWithoutEffect(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	var v, seq, aseq int64
	store := &racyStore{Store: w.store, between: func() {
		mustOK(t, w.change(txn.Detach(ctx), "grant.revoke", ref("grant", w.g2), []ids.ID{w.sam}, func(ctx context.Context) error {
			_, _, err := w.store.RevokeGrant(ctx, w.g2, w.owner, "first", w.clk.Now())
			return err
		}))
		v, seq, aseq = w.version(t), w.lastSeq(t), w.auditSeq(t)
	}}
	admin, err := app.NewAdminService(app.AdminDeps{
		UoW: w.uow, Authorizer: w.az, Store: store, Principals: w.principals, Structure: w.structure, Rights: w.hook,
		Audit: w.rec, Clock: w.clk, Limits: domain.GroupLimits{NameMaxChars: 100},
	})
	mustOK(t, err)
	g, err := admin.RevokeGrant(ctx, w.user(w.owner), w.g2, "second")
	mustOK(t, err)
	if g.RevokeReason != "first" || w.version(t) != v || len(w.events(t, seq)) != 0 {
		t.Fatalf("grant %+v, version %d (was %d), events %d", g, w.version(t), v, len(w.events(t, seq)))
	}
	rows := w.allows(t, aseq)
	if len(rows) != 1 || rows[0].Action != "grant.revoke" || !strings.Contains(strings.ReplaceAll(rows[0].Details, " ", ""), `"no_change":true`) {
		t.Fatalf("audit %+v", rows)
	}
}

// racyGroups runs between once: after the first read of the group (onGroup), or after the
// first read of the memberships of a person (onMembers), so the first unit decides on a state
// that another call changes before the second unit.
type racyGroups struct {
	app.Store
	once      sync.Once
	between   func()
	onMembers bool
}

func (s *racyGroups) GetGroup(ctx context.Context, id ids.ID) (domain.Group, bool, error) {
	g, ok, err := s.Store.GetGroup(ctx, id)
	if !s.onMembers {
		s.once.Do(s.between)
	}
	return g, ok, err
}

func (s *racyGroups) MembershipsOfUserInGroup(ctx context.Context, groupID, userID ids.ID) ([]domain.Membership, error) {
	ms, err := s.Store.MembershipsOfUserInGroup(ctx, groupID, userID)
	if s.onMembers {
		s.once.Do(s.between)
	}
	return ms, err
}

// racedAdmin returns an AdminService over store.
func (w *pgWorld) racedAdmin(t *testing.T, store app.Store) *app.AdminService {
	t.Helper()
	admin, err := app.NewAdminService(app.AdminDeps{
		UoW: w.uow, Authorizer: w.az, Store: store, Principals: w.principals, Structure: w.structure, Rights: w.hook,
		Audit: w.rec, Clock: w.clk, Limits: domain.GroupLimits{NameMaxChars: 100},
	})
	mustOK(t, err)
	return admin
}

// TestDeleteGroupAndRemoveMember_RacedCallsAreWithoutEffect: a group deleted and a membership
// removed by another call between the two units end without effect, like RevokeGrant.
func TestDeleteGroupAndRemoveMember_RacedCallsAreWithoutEffect(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"group.delete", "group.member.remove"} {
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			w := newPGWorld(t)
			ctx := context.Background()
			var v, seq, aseq int64
			store := &racyGroups{Store: w.store, between: func() {
				mustOK(t, w.change(txn.Detach(ctx), c, ref("group", w.leser), []ids.ID{w.sam}, func(ctx context.Context) error {
					if c == "group.delete" {
						_, err := w.store.SoftDeleteGroup(ctx, w.leser, w.clk.Now())
						return err
					}
					ms, err := w.store.MembershipsOfUserInGroup(ctx, w.leser, w.sam)
					if err != nil {
						return err
					}
					_, err = w.store.RemoveMembership(ctx, ms[0].ID, w.owner, w.clk.Now())
					return err
				}))
				v, seq, aseq = w.version(t), w.lastSeq(t), w.auditSeq(t)
			}, onMembers: c == "group.member.remove"}
			admin := w.racedAdmin(t, store)
			if c == "group.delete" {
				mustOK(t, admin.DeleteGroup(ctx, w.user(w.owner), w.leser, ""))
			} else {
				mustOK(t, admin.RemoveGroupMember(ctx, w.user(w.owner), w.leser, w.sam))
			}
			if w.version(t) != v || len(w.events(t, seq)) != 0 {
				t.Fatalf("version %d (was %d), events %d", w.version(t), v, len(w.events(t, seq)))
			}
			rows := w.allows(t, aseq)
			if len(rows) != 1 || rows[0].Action != c || !strings.Contains(strings.ReplaceAll(rows[0].Details, " ", ""), `"no_change":true`) {
				t.Fatalf("audit %+v", rows)
			}
		})
	}
}
