// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestClaimQueries (M1-42): the claim groups of a provider by claim value (live ones only), the
// person's claim memberships of that provider (not removed, expired ones included, local groups
// and other providers left out) and the extension, which clears both expiry markers and needs a
// rights change.
func TestClaimQueries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex := e.principal(t, "user", "alex")
	sam := e.principal(t, "user", "sam")
	prov, other := ids.New(), ids.New()
	for _, p := range []ids.ID{prov, other} {
		e.mustExec(t, `INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', $2, '{}')`, p.String(), p.String())
	}
	claim := func(name string, p ids.ID, ref string) domain.Group {
		t.Helper()
		g, err := domain.NewClaimGroup(ids.New(), name, "", limits, p, ref, alex, t0)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.write(func(ctx context.Context) error { return e.repo.InsertGroup(ctx, g) }); err != nil {
			t.Fatal(err)
		}
		return g
	}
	support, research, foreign := claim("Support", prov, "support"), claim("Research", prov, "research"), claim("Foreign", other, "support")
	gone := claim("Gone", prov, "gone")
	local := e.group(t, "Local", alex)
	if err := e.write(func(ctx context.Context) error { _, err := e.repo.SoftDeleteGroup(ctx, gone.ID, t0); return err }); err != nil {
		t.Fatal(err)
	}
	got, err := e.repo.ClaimGroupsByRefs(ctx, prov, []string{"support", "gone", "support", "unknown"})
	if err != nil || len(got) != 1 || got[0].ID != support.ID {
		t.Fatalf("ClaimGroupsByRefs %+v %v", got, err)
	}
	if none, err := e.repo.ClaimGroupsByRefs(ctx, prov, nil); err != nil || none != nil {
		t.Fatalf("ClaimGroupsByRefs without values %+v %v", none, err)
	}
	add := func(g domain.Group, exp time.Time) domain.Membership {
		t.Helper()
		m, err := domain.NewMembership(ids.New(), g.ID, sam, sam, exp, t0)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.write(func(ctx context.Context) error { return e.repo.InsertMembership(ctx, m) }); err != nil {
			t.Fatal(err)
		}
		return m
	}
	ms := add(support, t0.Add(time.Hour))
	add(research, t0.Add(2*time.Hour))
	add(foreign, t0.Add(time.Hour))
	add(local, time.Time{})
	var locked []domain.MembershipWithGroup
	if err := e.plain(func(ctx context.Context) (err error) {
		locked, err = e.repo.LockClaimMembershipsOfUser(ctx, sam, prov)
		return err
	}); err != nil || len(locked) != 2 {
		t.Fatalf("LockClaimMembershipsOfUser %+v %v", locked, err)
	}
	e.mustExec(t, `UPDATE group_memberships SET expiry_reminded_at = $2, expiry_reported_at = $2 WHERE id = $1`, ms.ID.String(), t0)
	extend := func(ctx context.Context) error {
		ok, err := e.repo.ExtendMembership(ctx, ms.ID, t0.Add(48*time.Hour), t0)
		if err == nil && !ok {
			t.Error("ExtendMembership changed no row")
		}
		return err
	}
	wantError(t, e.plain(extend), errs.KindInternal, errs.CodeInternalError, "")
	if err := e.write(extend); err != nil {
		t.Fatal(err)
	}
	var reminded, reported *time.Time
	var expires time.Time
	if err := e.pool.QueryRow(ctx, `SELECT expires_at, expiry_reminded_at, expiry_reported_at FROM group_memberships WHERE id = $1`,
		ms.ID.String()).Scan(&expires, &reminded, &reported); err != nil {
		t.Fatal(err)
	}
	if !expires.Equal(t0.Add(48*time.Hour)) || reminded != nil || reported != nil {
		t.Fatalf("after extension: %v %v %v", expires, reminded, reported)
	}
	if err := e.write(func(ctx context.Context) error {
		if _, err := e.repo.RemoveMembership(ctx, ms.ID, sam, t0); err != nil {
			return err
		}
		ok, err := e.repo.ExtendMembership(ctx, ms.ID, t0.Add(96*time.Hour), t0)
		if ok {
			t.Error("a removed membership was extended")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
