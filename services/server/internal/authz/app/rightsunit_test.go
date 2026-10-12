// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// rightsUnitCtx returns a context inside a unit of work declared as a rights change.
func rightsUnitCtx(ctx context.Context) context.Context {
	reg := txn.NewRegistry()
	reg.Seal()
	u, setPhase := txn.NewUnit(reg, txn.ApplyOptions(txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "grant.create"})))
	setPhase(txn.PhaseWork)
	return txn.WithUnit(ctx, u)
}

// TestAuthorize_NoCacheInsideARightsChange (M1-29): inside a rights change the Authorizer neither
// reads nor fills the snapshot cache nor the version memo, because the unit sees its own
// uncommitted rights version; outside it the cache works as before.
func TestAuthorize_NoCacheInsideARightsChange(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	res := w.msg(w.allgemein, w.alex)
	in := rightsUnitCtx(authorize.WithRequestScope(context.Background()))
	before := w.store.Reads()
	w.decide(t, in, alex, authorize.ActionMessageRead, res)
	w.decide(t, in, alex, authorize.ActionMessageRead, res)
	if got := w.store.Reads() - before; got != 2 {
		t.Fatalf("inside a rights change the snapshot was built %d times, want 2 (no cache)", got)
	}
	reads := w.versions.Reads()
	w.decide(t, in, alex, authorize.ActionMessageRead, res)
	if w.versions.Reads() != reads+1 {
		t.Fatal("inside a rights change the version came from the memo")
	}
	if _, err := w.az.Scope(in, alex, authorize.ActionMessageRead); err != nil {
		t.Fatal(err)
	}
	out := context.Background()
	before = w.store.Reads()
	w.decide(t, out, alex, authorize.ActionMessageRead, res)
	w.decide(t, out, alex, authorize.ActionMessageRead, res)
	if got := w.store.Reads() - before; got != 1 {
		t.Fatalf("outside a rights change the snapshot was built %d times, want 1 (the in-unit calls left no entry)", got)
	}
}

// TestGrantRevoke_RevokedGrantResolves (M1-29): grant.revoke of a revoked grant is judged like the
// revocation of the stored grant, so the use case can answer "without change"; an unknown grant
// stays not_found, and a person without rights on the target still gets not_found.
func TestGrantRevoke_RevokedGrantResolves(t *testing.T) {
	w := newWorld(t)
	if _, _, err := w.store.RevokeGrant(context.Background(), w.g2, w.owner, "", worldStart); err != nil {
		t.Fatal(err)
	}
	w.versions.Bump()
	g := authorize.Resource{Kind: authorize.ResourceGrant, ID: w.g2}
	w.allow(t, w.user(w.owner), authorize.ActionGrantRevoke, g)
	w.expect(t, w.user(w.owner), authorize.ActionGrantRevoke, authorize.Resource{Kind: authorize.ResourceGrant, ID: ids.New()},
		authorize.OutcomeNotFound, "no_grant")
	w.expect(t, w.user(w.alex), authorize.ActionGrantRevoke, g, authorize.OutcomeNotFound, "no_grant")
}
