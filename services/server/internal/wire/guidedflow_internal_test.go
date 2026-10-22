// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/authzsource"
	"github.com/OWNER/bothy/services/server/internal/authz"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// noPolicies is an empty agent policy source; noRights an empty rights hook.
type (
	noPolicies struct{}
	noRights   struct{}
)

func (noPolicies) PersonalAgents(context.Context) ([]authz.PersonalAgent, error) { return nil, nil }
func (noPolicies) AgentPolicy(context.Context, ids.ID) (authz.AgentPolicy, bool, error) {
	return authz.AgentPolicy{}, false, nil
}
func (noPolicies) DevEnvironmentReady(context.Context, ids.ID, ids.ID) (bool, error) {
	return false, nil
}
func (noRights) AddAffected(context.Context, ...ids.ID) error { return nil }

// TestWiringUsesRealAgentPolicySwitch (M2-21): the agents step binds the switch of the agents
// module into the resolver of authz, and the start fails when the resolver would still hold the
// empty switch; startAgents calls bindGuidedFlow, so every started graph has the real switch.
func TestWiringUsesRealAgentPolicySwitch(t *testing.T) {
	t.Parallel()
	if _, err := authzapp.NewResolver(authzapp.ResolverDeps{Policies: noPolicies{}, Rights: noRights{}, ReasonMaxChars: 10}); err == nil {
		t.Fatal("a resolver without an Authorizer must be refused")
	}
	resolver, err := authzapp.NewResolver(authzapp.ResolverDeps{Authorizer: allowNothing{}, Policies: noPolicies{}, Rights: noRights{}, ReasonMaxChars: 10})
	if err != nil {
		t.Fatal(err)
	}
	admin := &AuthzAdmin{Resolver: resolver}
	if !resolver.UsesEmptySwitch() {
		t.Fatal("the resolver must start with the empty switch")
	}
	if err := bindGuidedFlow(admin, &Agents{}); err == nil {
		t.Fatal("binding without the switch of the agents module must fail the start")
	}
	if err := bindGuidedFlow(admin, &Agents{Switch: authzsource.NewSwitch(nil)}); err != nil {
		t.Fatal(err)
	}
	if resolver.UsesEmptySwitch() {
		t.Fatal("the production graph still uses the empty switch")
	}
}

// allowNothing is an Authorizer the test never calls.
type allowNothing struct{ authorize.Authorizer }
