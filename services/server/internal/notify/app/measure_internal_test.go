// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestMeasureSent holds sli.push_acceptance of the own measurement (25 3.2, slos.md 2, M6-13):
// every provider answer except rejected_token counts with the provider as dimension, accepted is
// good, every other outcome bad.
func TestMeasureSent(t *testing.T) {
	t.Parallel()
	rec := &measure.Recording{}
	p := &Push{d: PushDeps{Measure: rec}}
	for _, o := range []string{domain.OutcomeAccepted, domain.OutcomeRejectedToken, "rejected_other", "throttled", "provider_error"} {
		p.measureSent("apns", o)
	}
	p.measureSent("fcm", domain.OutcomeAccepted)
	got := rec.Calls(measure.SLIPushAcceptance)
	if len(got) != 5 || len(rec.Invalid()) != 0 {
		t.Fatalf("calls %+v, invalid %v", got, rec.Invalid())
	}
	good := map[string]int{}
	for _, c := range got {
		if c.Value.OK {
			good[c.Key.Dimension]++
		}
	}
	if good["apns"] != 1 || good["fcm"] != 1 {
		t.Fatalf("good per provider %v, want one each", good)
	}
}
