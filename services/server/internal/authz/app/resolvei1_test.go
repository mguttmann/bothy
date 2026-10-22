// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// resolveLog records the calls of the fakes of the resolver tests in their order.
type resolveLog struct {
	calls []string
	// authorized are the agents authorize saw, with their actions and previewed actions.
	authorized []ids.ID
	acts       []authorize.Action
	previewOf  []authorize.Action
	affected   []ids.ID
	switched   [][]ids.ID
	reasons    []string
}

// fakeResolveAuthorizer allows every agent except those in deny, which get their decision.
type fakeResolveAuthorizer struct {
	// Authorizer is nil; only Authorize is called.
	authorize.Authorizer
	log  *resolveLog
	deny map[ids.ID]authorize.Decision
}

func (f *fakeResolveAuthorizer) Authorize(_ context.Context, _ authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	f.log.calls = append(f.log.calls, "authorize")
	f.log.authorized = append(f.log.authorized, r.ID)
	f.log.acts = append(f.log.acts, act)
	f.log.previewOf = append(f.log.previewOf, r.PreviewOf)
	if d, ok := f.deny[r.ID]; ok {
		return d, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

// fakeResolvePolicies knows the agents of known.
type fakeResolvePolicies struct {
	known map[ids.ID]bool
}

func (f fakeResolvePolicies) PersonalAgents(context.Context) ([]authz.PersonalAgent, error) {
	return nil, nil
}

func (f fakeResolvePolicies) AgentPolicy(_ context.Context, id ids.ID) (authz.AgentPolicy, bool, error) {
	return authz.AgentPolicy{BillingMode: "personal"}, f.known[id], nil
}

func (f fakeResolvePolicies) DevEnvironmentReady(context.Context, ids.ID, ids.ID) (bool, error) {
	return false, nil
}

// fakeResolveRights records AddAffected.
type fakeResolveRights struct{ log *resolveLog }

func (f fakeResolveRights) AddAffected(_ context.Context, principals ...ids.ID) error {
	f.log.calls = append(f.log.calls, "affected")
	f.log.affected = append(f.log.affected, principals...)
	return nil
}

// fakeSwitch records SwitchToOrganization.
type fakeSwitch struct{ log *resolveLog }

func (f fakeSwitch) SwitchToOrganization(_ context.Context, _ actor.Actor, list []ids.ID, reason string) error {
	f.log.calls = append(f.log.calls, "switch")
	f.log.switched = append(f.log.switched, slices.Clone(list))
	f.log.reasons = append(f.log.reasons, reason)
	return nil
}

// sortedIDs returns n new ids in ascending order.
func sortedIDs(n int) []ids.ID {
	out := make([]ids.ID, n)
	for i := range out {
		out[i] = ids.New()
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	return out
}

// newTestResolver returns a resolver over the fakes of log.
func newTestResolver(t *testing.T, log *resolveLog, deny map[ids.ID]authorize.Decision, known []ids.ID) *app.Resolver {
	t.Helper()
	k := map[ids.ID]bool{}
	for _, id := range known {
		k[id] = true
	}
	r, err := app.NewResolver(app.ResolverDeps{
		Authorizer: &fakeResolveAuthorizer{log: log, deny: deny}, Policies: fakeResolvePolicies{known: k},
		Rights: fakeResolveRights{log: log}, ReasonMaxChars: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	r.BindSwitch(fakeSwitch{log: log})
	return r
}

var resolveCaller = actor.Actor{ID: ids.New(), Kind: actor.KindUser}

// TestResolveI1Order (M2-21 design note 3 e): authorize with agent.billing_mode.set once per agent
// in ascending id, AddAffected with all ids before the switch, the switch once with the sorted ids
// and the trimmed reason; the first denial is returned unchanged and nothing after it runs; an
// unknown agent is 404 not_found and nothing after it runs.
func TestResolveI1Order(t *testing.T) {
	a := sortedIDs(3)
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		log := &resolveLog{}
		r := newTestResolver(t, log, nil, a)
		err := r.ResolveI1(ctx, resolveCaller, authz.ResolveI1{AgentIDs: []ids.ID{a[2], a[0], a[1]}, Reason: "  Second owner  "})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(log.authorized, a) || !slices.Equal(log.calls, []string{"authorize", "authorize", "authorize", "affected", "switch"}) {
			t.Fatalf("calls %v on %v", log.calls, log.authorized)
		}
		for _, act := range log.acts {
			if act != authorize.ActionAgentBillingModeSet {
				t.Fatalf("action %s", act)
			}
		}
		if !slices.Equal(log.affected, a) || len(log.switched) != 1 || !slices.Equal(log.switched[0], a) || log.reasons[0] != "Second owner" {
			t.Fatalf("affected %v switched %v reasons %q", log.affected, log.switched, log.reasons)
		}
	})
	t.Run("first denial decides", func(t *testing.T) {
		log := &resolveLog{}
		deny := authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "owner_only"}
		r := newTestResolver(t, log, map[ids.ID]authorize.Decision{a[1]: deny, a[2]: {Outcome: authorize.OutcomeNotFound}}, a)
		err := r.ResolveI1(ctx, resolveCaller, authz.ResolveI1{AgentIDs: []ids.ID{a[2], a[1], a[0]}, Reason: "x"})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeForbidden || e.Reason != "owner_only" {
			t.Fatalf("got %v, want the forbidden of the second agent", err)
		}
		if !slices.Equal(log.authorized, a[:2]) || slices.Contains(log.calls, "affected") || slices.Contains(log.calls, "switch") {
			t.Fatalf("calls %v on %v", log.calls, log.authorized)
		}
	})
	t.Run("unknown agent", func(t *testing.T) {
		log := &resolveLog{}
		r := newTestResolver(t, log, nil, a[1:])
		err := r.ResolveI1(ctx, resolveCaller, authz.ResolveI1{AgentIDs: a, Reason: "x"})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeNotFound || e.Detail != "" {
			t.Fatalf("got %v, want the bare not_found", err)
		}
		if len(log.authorized) != 1 || slices.Contains(log.calls, "affected") || slices.Contains(log.calls, "switch") {
			t.Fatalf("calls %v", log.calls)
		}
	})
	t.Run("invalid request", func(t *testing.T) {
		log := &resolveLog{}
		r := newTestResolver(t, log, nil, a)
		err := r.ResolveI1(ctx, resolveCaller, authz.ResolveI1{AgentIDs: []ids.ID{a[0], a[0]}, Reason: "x"})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed || len(log.calls) != 0 {
			t.Fatalf("got %v and calls %v", err, log.calls)
		}
	})
	t.Run("preview", func(t *testing.T) {
		log := &resolveLog{}
		r := newTestResolver(t, log, nil, a)
		if err := r.AuthorizePreview(ctx, resolveCaller, []ids.ID{a[1], a[0]}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(log.authorized, a[:2]) || slices.Contains(log.calls, "switch") {
			t.Fatalf("calls %v on %v", log.calls, log.authorized)
		}
		for i, act := range log.acts {
			if act != authorize.ActionAccessPreview || log.previewOf[i] != authorize.ActionAgentBillingModeSet {
				t.Fatalf("preview authorized %s of %s", act, log.previewOf[i])
			}
		}
	})
}

// TestEmptyAgentPolicySwitchFailsClosed: before internal/wire binds the agents module the switch
// is an internal error, and so is a resolver without a bound switch.
func TestEmptyAgentPolicySwitchFailsClosed(t *testing.T) {
	err := authz.EmptyAgentPolicySwitch{}.SwitchToOrganization(context.Background(), resolveCaller, []ids.ID{ids.New()}, "x")
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("got %v", err)
	}
	log := &resolveLog{}
	a := sortedIDs(1)
	r, err := app.NewResolver(app.ResolverDeps{Authorizer: &fakeResolveAuthorizer{log: log}, Policies: fakeResolvePolicies{known: map[ids.ID]bool{a[0]: true}},
		Rights: fakeResolveRights{log: log}, ReasonMaxChars: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !r.UsesEmptySwitch() {
		t.Fatal("a new resolver must start with the empty switch")
	}
	err = r.ResolveI1(context.Background(), resolveCaller, authz.ResolveI1{AgentIDs: a, Reason: "x"})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("got %v", err)
	}
}
