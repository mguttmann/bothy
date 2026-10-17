// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestEmptySourcesKnowNothing holds that the empty structure source reports an instance without
// spaces, channels, threads and served targets, so authorize decides fail closed until the owning
// modules replace it (ruling D2 of the M1-28 brief). The agent policy comes from agents since M1-51.
func TestEmptySourcesKnowNothing(t *testing.T) {
	t.Parallel()
	ctx, id := context.Background(), ids.New()
	s := emptyStructure{}
	if ok, err := s.Space(ctx, id); ok || err != nil {
		t.Errorf("Space = %v, %v", ok, err)
	}
	if got, err := s.Spaces(ctx); got != nil || err != nil {
		t.Errorf("Spaces = %v, %v", got, err)
	}
	if ok, err := s.SpaceArchived(ctx, id); ok || err != nil {
		t.Errorf("SpaceArchived = %v, %v", ok, err)
	}
	if _, ok, err := s.Channel(ctx, id); ok || err != nil {
		t.Errorf("Channel = %v, %v", ok, err)
	}
	if _, ok, err := s.Thread(ctx, id); ok || err != nil {
		t.Errorf("Thread = %v, %v", ok, err)
	}
	if got, err := s.Channels(ctx); got != nil || err != nil {
		t.Errorf("Channels = %v, %v", got, err)
	}
	if got, err := s.ChannelsOperatedBy(ctx, id); got != nil || err != nil {
		t.Errorf("ChannelsOperatedBy = %v, %v", got, err)
	}
	if got, err := s.DMChannelsOf(ctx, id); got != nil || err != nil {
		t.Errorf("DMChannelsOf = %v, %v", got, err)
	}
	if got, err := s.ServedTargets(ctx, id); got != nil || err != nil {
		t.Errorf("ServedTargets = %v, %v", got, err)
	}
	var _ authz.StructureSource = s
}
