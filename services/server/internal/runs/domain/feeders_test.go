// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestFeedSourceOf_EveryInputEvent: every invocation-bearing event type, the inputs an agent
// receives for a run, records a source of run_feeders at its delivery, and every source is one of
// FeedSources; a new input event without a source fails here (FEAT-TEAM-CHANNEL-SERVER).
func TestFeedSourceOf_EveryInputEvent(t *testing.T) {
	seen := map[string]bool{}
	for _, typ := range domain.InvocationBearingTypes() {
		src := domain.FeedSourceOf(typ)
		if !slices.Contains(domain.FeedSources(), src) {
			t.Errorf("event %s records source %q, which is no source of run_feeders", typ, src)
		}
		if seen[src] {
			t.Errorf("source %s is recorded by two event types", src)
		}
		seen[src] = true
		if !domain.InvocationBearing(typ) {
			t.Errorf("%s is listed but not invocation-bearing", typ)
		}
	}
	if got := domain.FeedSourceOf("message.created"); got != "" {
		t.Errorf("an event without an invocation records %q", got)
	}
	if !seen[domain.FeedTrigger] || seen[domain.FeedContext] {
		t.Errorf("the deliveries record %v; the context is recorded by the run context only", seen)
	}
}
