// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestEmptySourcesKnowNothing holds that the empty channel part of the structure source reports
// an instance without channels, threads, details and served targets, so authorize decides fail
// closed until the owning modules replace it (ruling D2 of the M1-28 brief). The agent policy
// comes from agents since M1-51 and the spaces from chat since M1-52.
func TestEmptySourcesKnowNothing(t *testing.T) {
	t.Parallel()
	ctx, id := context.Background(), ids.New()
	s := emptyChannels{}
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
	if got, err := s.ChannelDetails(ctx, []ids.ID{id}); len(got) != 0 || err != nil {
		t.Errorf("ChannelDetails = %v, %v", got, err)
	}
}
