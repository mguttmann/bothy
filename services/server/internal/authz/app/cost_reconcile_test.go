// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// costInstance is the resource of the three reconciliation actions.
var costInstance = authorize.Resource{Kind: authorize.ResourceInstance}

// reconcilerApp is the app Tracker holding scope in its installation and its token, as the
// reconciliation app does with costs:reconcile.
func reconcilerApp(w *world, scope string) actor.Actor {
	a := w.app()
	a.GrantedScopes, a.Scopes = []string{scope}, []string{scope}
	return a
}

// personalToken is a personal API token of the owner: a person without a session.
func personalToken(w *world) actor.Actor {
	w.addToken(w.owner)
	return actor.Actor{ID: w.owner, Kind: actor.KindUser, TokenID: w.tokens[w.owner]}
}

// TestCostReconcileRequirement (M2-28 design note 8, R-AZ-03): cost.reconcile is owner only with
// step-up; an app token with costs:reconcile, an agent, the owner's personal token and a person
// who is no owner fail the requirement before the step-up.
func TestCostReconcileRequirement(t *testing.T) {
	w := newWorld(t)
	act := authorize.ActionCostReconcile
	w.allow(t, w.user(w.owner), act, costInstance)
	old := w.user(w.owner)
	old.StepUpAt = w.clk.Now().Add(-time.Hour)
	w.expect(t, old, act, costInstance, stepUpNeeded, "step_up_required")
	for name, p := range map[string]actor.Actor{
		"app with costs:reconcile": reconcilerApp(w, domain.ScopeCostsReconcile),
		"agent":                    w.agent(w.lotse),
		"personal token":           personalToken(w),
		"not an owner":             w.user(w.sam),
	} {
		t.Run(name, func(t *testing.T) { w.expect(t, p, act, costInstance, forbidden, "owner_only") })
	}
}

// TestCostReconcileSubmitRequirement (M2-28 design note 8): only an app with costs:reconcile; an
// app without it gets scope_missing; the owner's session and an agent are no app.
func TestCostReconcileSubmitRequirement(t *testing.T) {
	w := newWorld(t)
	act := authorize.ActionCostReconcileSubmit
	w.allow(t, reconcilerApp(w, domain.ScopeCostsReconcile), act, costInstance)
	w.expect(t, w.app(), act, costInstance, forbidden, "scope_missing")
	w.expect(t, w.user(w.owner), act, costInstance, forbidden, "subject_kind_not_allowed")
	w.expect(t, w.agent(w.lotse), act, costInstance, forbidden, "subject_kind_not_allowed")
}

// TestCostReconcileReadRequirement (M2-28 design note 8, R-AZ-28): an owner in a session without
// step-up and an app with costs:reconcile read; an app without it gets scope_missing; an agent,
// the owner's personal token and a person who is no owner fail the first alternative owner.
func TestCostReconcileReadRequirement(t *testing.T) {
	w := newWorld(t)
	act := authorize.ActionCostReconcileRead
	noStepUp := w.user(w.owner)
	noStepUp.StepUpAt = time.Time{}
	w.allow(t, noStepUp, act, costInstance)
	w.allow(t, reconcilerApp(w, domain.ScopeCostsReconcile), act, costInstance)
	w.expect(t, w.app(), act, costInstance, forbidden, "scope_missing")
	for name, p := range map[string]actor.Actor{
		"agent":          w.agent(w.lotse),
		"personal token": personalToken(w),
		"not an owner":   w.user(w.sam),
	} {
		t.Run(name, func(t *testing.T) { w.expect(t, p, act, costInstance, forbidden, "owner_only") })
	}
}
