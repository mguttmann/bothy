// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestStructureScopeReadsThreadAndChannelOnce: within one structure scope (the hub uses one per
// event, M1-72) the checks of several principals on one message read its thread and channel once;
// without a scope every check reads them. The decisions are the same either way.
func TestStructureScopeReadsThreadAndChannelOnce(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	r := w.msg(w.allgemein, w.sam)
	ctx := authorize.Delivery(context.Background())
	people := []struct {
		name string
		id   func() authorize.Principal
	}{
		{"alex", func() authorize.Principal { return w.user(w.alex) }},
		{"sam", func() authorize.Principal { return w.user(w.sam) }},
		{"robin", func() authorize.Principal { return w.user(w.robin) }},
	}
	var plain []authorize.Outcome
	threads0, channels0 := w.structure.Calls()
	for _, p := range people {
		plain = append(plain, w.decide(t, ctx, p.id(), authorize.ActionMessageRead, r).Outcome)
	}
	threads1, channels1 := w.structure.Calls()
	if threads1-threads0 < len(people) || channels1-channels0 < len(people) {
		t.Fatalf("without a scope: %d thread and %d channel reads for %d checks", threads1-threads0, channels1-channels0, len(people))
	}
	scoped := authorize.WithStructureScope(ctx)
	for i, p := range people {
		if got := w.decide(t, scoped, p.id(), authorize.ActionMessageRead, r).Outcome; got != plain[i] {
			t.Errorf("%s: %s in the scope, %s without", p.name, got, plain[i])
		}
	}
	threads2, channels2 := w.structure.Calls()
	if threads2-threads1 != 1 || channels2-channels1 != 1 {
		t.Fatalf("in one scope: %d thread and %d channel reads, want 1 each", threads2-threads1, channels2-channels1)
	}
}
