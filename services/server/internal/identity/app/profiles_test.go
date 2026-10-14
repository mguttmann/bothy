// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// profileSeed is one principal of the contract world of Profiles and OwnerIDs.
type profileSeed struct {
	kind            actor.Kind
	handle          string
	owner, disabled bool
	revokedOwner    bool
	locale, tz      string
}

// profileWorld builds an authz.PrincipalSource that knows the principals of seeds and returns
// their ids in the order of seeds.
type profileWorld func(t *testing.T, seeds []profileSeed) (authz.PrincipalSource, []ids.ID)

// fakeProfiles is the authz fake (adapters/fake.Principals).
func fakeProfiles(_ *testing.T, seeds []profileSeed) (authz.PrincipalSource, []ids.ID) {
	src := fake.NewPrincipals()
	out := make([]ids.ID, 0, len(seeds))
	for _, s := range seeds {
		id := ids.New()
		status := authz.PrincipalActive
		if s.disabled {
			status = authz.PrincipalDisabled
		}
		src.Put(authz.PrincipalFacts{ID: id, Kind: s.kind, Status: status, IsOwner: s.owner && !s.revokedOwner})
		p := authz.PrincipalProfile{PrincipalRef: authz.PrincipalRef{ID: id, Kind: s.kind, Handle: s.handle, DisplayName: "Name of " + s.handle}, Status: status}
		if s.kind == actor.KindUser {
			p.Locale, p.TimeZone = s.locale, s.tz
		}
		src.PutProfile(p)
		out = append(out, id)
	}
	return src, out
}

// realProfiles is the identity implementation over PostgreSQL.
func realProfiles(t *testing.T, seeds []profileSeed) (authz.PrincipalSource, []ids.ID) {
	t.Helper()
	f := newFixture(t)
	out := make([]ids.ID, 0, len(seeds))
	for _, s := range seeds {
		p := f.add(domain.Kind(s.kind), s.handle)
		if s.kind == actor.KindUser {
			f.sql(`UPDATE users SET locale = $2, time_zone = $3 WHERE principal_id = $1`, p.ID.String(), s.locale, s.tz)
		}
		if s.owner {
			o := f.grant(p.ID, ids.ID{})
			if s.revokedOwner {
				f.sql(`UPDATE owners SET revoked_at = now() WHERE id = $1`, o.ID.String())
			}
		}
		if s.disabled {
			f.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, p.ID.String())
		}
		out = append(out, p.ID)
	}
	return app.NewPrincipalSource(f.repo), out
}

// TestPrincipalSourceProfilesContract is the shared contract of PrincipalSource.Profiles and
// OwnerIDs (M1-49), run against the authz fake and the identity implementation: Profiles names
// every known principal whatever its status, with locale and time zone for people only, and
// leaves unknown ids out; OwnerIDs lists the owners rows that are not revoked, a disabled owner
// included, ascending.
func TestPrincipalSourceProfilesContract(t *testing.T) {
	t.Parallel()
	seeds := []profileSeed{
		{kind: actor.KindUser, handle: "owner", owner: true, locale: "de-DE", tz: "Europe/Berlin"},
		{kind: actor.KindUser, handle: "robin", owner: true, disabled: true, locale: "en", tz: "UTC"},
		{kind: actor.KindUser, handle: "sam", owner: true, revokedOwner: true, locale: "fr", tz: "UTC"},
		{kind: actor.KindAgent, handle: "texter"},
	}
	for name, build := range map[string]profileWorld{"fake": fakeProfiles, "identity": realProfiles} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, list := build(t, seeds)
			ctx := context.Background()
			got, err := src.Profiles(ctx, append(slices.Clone(list), ids.New()))
			if err != nil || len(got) != len(list) {
				t.Fatalf("Profiles = %v, %v", got, err)
			}
			owner, robin, texter := got[list[0]], got[list[1]], got[list[3]]
			if owner.Handle != "owner" || owner.DisplayName != "Name of owner" || owner.Kind != actor.KindUser ||
				owner.Status != authz.PrincipalActive || owner.Locale != "de-DE" || owner.TimeZone != "Europe/Berlin" {
				t.Fatalf("profile of the owner %+v", owner)
			}
			if robin.Status != authz.PrincipalDisabled || robin.Locale != "en" {
				t.Fatalf("profile of a disabled person %+v", robin)
			}
			if texter.Kind != actor.KindAgent || texter.Handle != "texter" || texter.Locale != "" || texter.TimeZone != "" {
				t.Fatalf("profile of an agent %+v", texter)
			}
			owners, err := src.OwnerIDs(ctx)
			want := []ids.ID{list[0], list[1]}
			slices.SortFunc(want, compare)
			if err != nil || !slices.Equal(owners, want) {
				t.Fatalf("OwnerIDs = %v, %v, want %v", owners, err, want)
			}
		})
	}
}
