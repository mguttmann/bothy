// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestEmptySourcesKnowNothing holds that the empty thread part of the structure source reports
// an instance without threads, dm participation and served targets, so authorize decides fail
// closed until the owning modules replace it (ruling D2 of the M1-28 brief). The agent policy
// comes from agents since M1-51 and the spaces and channels from chat since M1-52 and M1-53.
func TestEmptySourcesKnowNothing(t *testing.T) {
	t.Parallel()
	ctx, id := context.Background(), ids.New()
	s := emptyThreads{}
	if _, ok, err := s.Thread(ctx, id); ok || err != nil {
		t.Errorf("Thread = %v, %v", ok, err)
	}
	if got, err := s.DMChannelsOf(ctx, id); got != nil || err != nil {
		t.Errorf("DMChannelsOf = %v, %v", got, err)
	}
	if got, err := s.ServedTargets(ctx, id); got != nil || err != nil {
		t.Errorf("ServedTargets = %v, %v", got, err)
	}
}
