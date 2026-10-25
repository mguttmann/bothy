// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// The dimensions of invocations_denied (25 3.2) and the billing mode whose amounts are shown only
// with billing.show_personal_estimates (10 section 12.2).
const (
	dimensionI2Denied       = "i2_denied"
	dimensionModeMismatch   = "billing_mode_mismatch"
	billingModePersonal     = "personal"
	billingModeOrganization = "organization"
)

// Backends implements stats.Views (getStatsBackends, docs/spec/25-visualization.md 6.2, ruling T8,
// M6-14 design note 3, D-M6-14-3 and D-M6-14-4), in this order:
//
//  1. stats.read on the instance or the space (403, 404 for an unreadable space);
//  2. the range: from before to and not before now minus retention.stats_minutes (400);
//  3. the channel set of M6-05 and cost.read.aggregate on the same target in filter mode;
//  4. the agents: those with a run in a visible channel whose interval intersects the range and
//     the effective operators of the visible channels;
//  5. per agent the configured and reported mode, the supported modes and the stored rate_limit
//     (agents.BackendSource), the switches of the range (no channel: counted for the agents of the
//     result only), the denials of I2 and I3 in the visible channels and, with
//     cost.read.aggregate, the costs per mode and currency in the visible channels.
//
// Costs come from the own measurement, which is no billing record (25 3.4 item 4) and does not
// follow reconciliation, so every amount is an estimate; amounts of mode personal appear only with
// billing.show_personal_estimates (10 section 12.2), as in getCostStats. No member names a runtime,
// an adapter kind, a version, evidence, a subscription or a credential (I5).
func (v *Views) Backends(ctx context.Context, q stats.BackendQuery) ([]stats.BackendAgent, error) {
	st := v.d.Statistics
	a, err := st.authorize(ctx, authorize.ActionStatsRead, q.SpaceID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateBackendRange(q.From, q.To, v.d.Clock.Now(), v.cfg.Retention); err != nil {
		return nil, err
	}
	t, err := st.visible(ctx, a, q.SpaceID)
	if err != nil {
		return nil, err
	}
	costs, err := v.costsAllowed(ctx, a, q.SpaceID)
	if err != nil {
		return nil, err
	}
	list, err := v.backendAgents(ctx, t, q)
	if err != nil {
		return nil, err
	}
	states, err := v.d.Agents.BackendStates(ctx, list)
	if err != nil {
		return nil, err
	}
	out := backendItems(states, list, costs)
	if len(out) == 0 {
		return out, nil
	}
	agentIDs := make([]ids.ID, len(out))
	byID := make(map[ids.ID]int, len(out))
	for i, b := range out {
		agentIDs[i], byID[b.AgentID] = b.AgentID, i
	}
	rows, err := v.d.Store.BackendRows(ctx, BackendRowsQuery{From: q.From, To: q.To, AgentIDs: agentIDs, ChannelIDs: t.channels, WithCost: costs})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if i, ok := byID[r.AgentID]; ok {
			v.addBackendRow(&out[i], r, costs)
		}
	}
	for i := range out {
		slices.SortFunc(out[i].Costs, func(a, b stats.BackendCost) int {
			return cmp.Or(strings.Compare(a.BillingMode, b.BillingMode), strings.Compare(a.Currency, b.Currency))
		})
	}
	return out, nil
}

// backendItems converts the states of the agents of list (defence in depth: no other agent),
// ascending by agent ID; costs, when shown, start as an empty list.
func backendItems(states []agents.BackendState, list []ids.ID, costs bool) []stats.BackendAgent {
	out := make([]stats.BackendAgent, 0, len(states))
	for _, s := range states {
		if !slices.Contains(list, s.AgentID) {
			continue
		}
		b := stats.BackendAgent{AgentID: s.AgentID, BillingMode: s.BillingMode, ReportedBillingMode: s.ReportedBillingMode,
			BillingModesSupported: append([]string{}, s.BillingModesSupported...), CostsShown: costs}
		if costs {
			b.Costs = []stats.BackendCost{}
		}
		if s.RateLimit != nil {
			b.RateLimit = &stats.RateLimit{State: s.RateLimit.State, ResetsAt: s.RateLimit.ResetsAt}
		}
		out = append(out, b)
	}
	slices.SortFunc(out, func(a, b stats.BackendAgent) int { return bytes.Compare(a.AgentID[:], b.AgentID[:]) })
	return out
}

// backendAgents returns the agents of the view, ascending and distinct: those with a run in a
// visible channel that intersects the range, and the effective operators of the visible channels.
func (v *Views) backendAgents(ctx context.Context, t target, q stats.BackendQuery) ([]ids.ID, error) {
	threads, _, err := v.threadsOf(ctx, t.channels, q.To)
	if err != nil {
		return nil, err
	}
	withRuns, err := v.d.Runs.TimelineAgents(ctx, runs.TimelineQuery{From: q.From, To: q.To, ThreadIDs: threads})
	if err != nil {
		return nil, err
	}
	places, err := v.d.Places.ChannelPlaces(ctx)
	if err != nil {
		return nil, err
	}
	in := make(map[ids.ID]bool, len(t.channels))
	for _, c := range t.channels {
		in[c] = true
	}
	out := slices.Clone(withRuns)
	for _, p := range places {
		if in[p.ChannelID] && !p.OperatorID.IsZero() {
			out = append(out, p.OperatorID)
		}
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	return slices.Compact(out), nil
}

// addBackendRow adds one sum of stats_minutes to b. A cost row counts only with costs (defence in
// depth), and one of mode personal only with billing.show_personal_estimates.
func (v *Views) addBackendRow(b *stats.BackendAgent, r BackendRow, costs bool) {
	m := measure.Metric(r.Metric)
	switch {
	case m == measure.BillingModeSwitches:
		b.Switches += r.Count
	case m == measure.InvocationsDenied && r.Dimension == dimensionI2Denied:
		b.I2Denied += r.Count
	case m == measure.InvocationsDenied && r.Dimension == dimensionModeMismatch:
		b.BillingModeMismatch += r.Count
	case m == measure.Cost:
		if !costs || (r.Dimension != billingModeOrganization && r.Dimension != billingModePersonal) ||
			(r.Dimension == billingModePersonal && !v.cfg.ShowPersonalEstimates) {
			return
		}
		i := slices.IndexFunc(b.Costs, func(c stats.BackendCost) bool { return c.BillingMode == r.Dimension && c.Currency == r.Currency })
		if i < 0 {
			b.Costs = append(b.Costs, stats.BackendCost{BillingMode: r.Dimension, Currency: r.Currency, IsEstimate: true})
			i = len(b.Costs) - 1
		}
		b.Costs[i].AmountMicros += r.Sum
	}
}
