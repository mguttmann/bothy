// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// fakeMetrics records the decisions the Authorizer counts.
type fakeMetrics struct {
	mu        gosync.Mutex
	decisions []string
}

func (m *fakeMetrics) Decision(_ context.Context, act authorize.Action, o authorize.Outcome) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisions = append(m.decisions, string(act)+" "+string(o))
}

func (m *fakeMetrics) got() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.decisions)
}

// TestAuthzMetrics_DeliveryChecksNotCounted: bothy.authz.decisions counts the decisions of
// requests and ops (allow and deny alike); checks in filter mode (lists) and in delivery mode
// (the hub and catch-up) count nothing (signals.md 4.1).
func TestAuthzMetrics_DeliveryChecksNotCounted(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	m := &fakeMetrics{}
	w.metrics = m
	w.build(t)
	ctx := context.Background()
	w.decide(t, ctx, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	w.decide(t, ctx, w.user(w.robin), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	w.decide(t, authorize.Filter(ctx), w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	w.decide(t, authorize.Delivery(ctx), w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	want := []string{"message.read allow", "message.read not_found"}
	if got := m.got(); !slices.Equal(got, want) {
		t.Errorf("counted %v, want %v", got, want)
	}
}
