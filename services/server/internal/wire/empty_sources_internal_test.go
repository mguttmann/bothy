// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestEmptySourcesKnowNothing holds that the empty sources report an instance without spaces,
// channels, threads, served targets, agent policies and personal agents, so authorize decides
// fail closed until the owning modules replace them (ruling D2 of the M1-28 brief).
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
	p := emptyPolicies{}
	if _, ok, err := p.AgentPolicy(ctx, id); ok || err != nil {
		t.Errorf("AgentPolicy = %v, %v", ok, err)
	}
	if ok, err := p.DevEnvironmentReady(ctx, id, id); ok || err != nil {
		t.Errorf("DevEnvironmentReady = %v, %v", ok, err)
	}
	if got, err := p.PersonalAgents(ctx); got != nil || err != nil {
		t.Errorf("PersonalAgents = %v, %v", got, err)
	}
	var _ authz.StructureSource = s
}
