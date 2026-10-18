// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// TestDecisions_ActionAndDecision: a decision counts once in bothy.authz.decisions with exactly
// bothy.action and bothy.decision from their closed sets.
func TestDecisions_ActionAndDecision(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.Decision(ctx, authorize.ActionMessageRead, authorize.OutcomeAllow)
	m.Decision(ctx, authorize.ActionMessageRead, authorize.OutcomeAllow)
	m.Decision(ctx, authorize.ActionMessageCreate, authorize.OutcomeForbidden)
	got := metricstest.Find(t, reader, "bothy.authz.decisions")
	if n := metricstest.Sum(got, map[string]string{"bothy.action": "message.read", "bothy.decision": "allow"}); n != 2 {
		t.Errorf("message.read allow = %v, want 2", n)
	}
	if n := metricstest.Sum(got, map[string]string{"bothy.action": "message.create", "bothy.decision": "forbidden"}); n != 1 {
		t.Errorf("message.create forbidden = %v, want 1", n)
	}
	for _, p := range metricstest.Points(got) {
		if len(p.Attrs) != 2 {
			t.Errorf("attributes %v, want bothy.action and bothy.decision", p.Attrs)
		}
	}
}
