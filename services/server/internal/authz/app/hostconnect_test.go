// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// hostPrincipal is the principal of the host service of host id (07 section 3, D-T25).
func hostPrincipal(id ids.ID) actor.Actor { return actor.Actor{ID: id, Kind: actor.KindHost} }

// hostAuth is the resource of host.connect on host id with the verdict v of the proof (M2-43
// Design note 4).
func hostAuth(id ids.ID, v authorize.HostAuthVerdict) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceHost, ID: id, HostAuth: &authorize.HostAuthFacts{Verdict: v}}
}

// TestHostConnectRequirement (M2-43, 05 section 4.14, 07 section 3): host.connect passes only for
// the host principal of its own row in confirming or active; another host's id is not_found;
// people, agents and apps get subject_kind_not_allowed; a proof that failed gives
// unauthenticated host_auth_failed; a host in disabled, retired or pairing gives unauthenticated
// principal_inactive; a missing HostAuth is an internal error.
func TestHostConnectRequirement(t *testing.T) {
	t.Parallel()
	w := newHostWorld(t)
	connect := authorize.ActionHostConnect
	confirming, disabled, retired, pairing := ids.New(), ids.New(), ids.New(), ids.New()
	w.hosts.Put(authz.HostFacts{ID: confirming, State: "confirming", LLMCredentials: "none"})
	w.hosts.Put(authz.HostFacts{ID: disabled, State: "disabled", LLMCredentials: "none"})
	w.hosts.Put(authz.HostFacts{ID: retired, State: "retired", LLMCredentials: "none"})
	w.hosts.Put(authz.HostFacts{ID: pairing, State: "pairing", LLMCredentials: "none"})

	w.allow(t, hostPrincipal(w.r1), connect, hostAuth(w.r1, authorize.HostAuthValid))
	w.allow(t, hostPrincipal(confirming), connect, hostAuth(confirming, authorize.HostAuthValid))
	w.expect(t, hostPrincipal(w.r1), connect, hostAuth(w.r2, authorize.HostAuthValid), authorize.OutcomeNotFound, "no_grant")

	for name, p := range map[string]actor.Actor{"owner": w.user(w.owner), "agent": w.agent(w.lotse), "app": w.app()} {
		d := w.decide(t, context.Background(), p, connect, hostAuth(w.r1, authorize.HostAuthValid))
		if d.Outcome != authorize.OutcomeForbidden || d.Reason != "subject_kind_not_allowed" {
			t.Errorf("%s: %s/%s, want forbidden/subject_kind_not_allowed", name, d.Outcome, d.Reason)
		}
	}
	for _, v := range []authorize.HostAuthVerdict{authorize.HostAuthUnknownKey, authorize.HostAuthInvalid} {
		w.expect(t, hostPrincipal(w.r1), connect, hostAuth(w.r1, v), authorize.OutcomeUnauthenticated, "host_auth_failed")
	}
	for _, h := range []ids.ID{disabled, retired, pairing} {
		w.expect(t, hostPrincipal(h), connect, hostAuth(h, authorize.HostAuthValid), authorize.OutcomeUnauthenticated, "principal_inactive")
	}
	// A host without a row: the proof cannot be valid, so the verdict decides.
	unknown := ids.New()
	w.expect(t, hostPrincipal(unknown), connect, hostAuth(unknown, authorize.HostAuthUnknownKey), authorize.OutcomeUnauthenticated, "host_auth_failed")

	_, err := w.az.Authorize(context.Background(), hostPrincipal(w.r1), connect, authorize.Resource{Kind: authorize.ResourceHost, ID: w.r1})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("missing HostAuth: %v, want an internal error", err)
	}
	// A host principal reaches no other action: it is not authenticated for any.
	w.expect(t, hostPrincipal(w.r1), authorize.ActionHostRead, host(w.r1), authorize.OutcomeUnauthenticated, "unauthenticated")
}

// countingPrincipals counts the reads of the principal source.
type countingPrincipals struct {
	authz.PrincipalSource
	mu sync.Mutex
	n  int
}

func (c *countingPrincipals) Principal(ctx context.Context, id ids.ID) (authz.PrincipalFacts, bool, error) {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return c.PrincipalSource.Principal(ctx, id)
}

// TestHostPrincipalSkipsPrincipalSource (M2-43 Design note 4): a host principal is never looked up
// in principals; its snapshot is empty.
func TestHostPrincipalSkipsPrincipalSource(t *testing.T) {
	t.Parallel()
	w := newHostWorld(t)
	counted := &countingPrincipals{PrincipalSource: w.principals}
	az, err := app.NewAuthorizer(w.cfg, app.AuthorizerDeps{
		Catalog: app.NewCatalog(slog.New(slog.NewTextHandler(w.logs, nil))), Store: w.store, Principals: counted,
		Credentials: w.creds, Structure: w.structure, Policies: w.policies, Hosts: w.hosts, Versions: w.versions, Recorder: w.rec,
		Clock: w.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	w.az = az
	w.allow(t, hostPrincipal(w.r1), authorize.ActionHostConnect, hostAuth(w.r1, authorize.HostAuthValid))
	if counted.n != 0 {
		t.Fatalf("principal source read %d times for a host principal", counted.n)
	}
}
