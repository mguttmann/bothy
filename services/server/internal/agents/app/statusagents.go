// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// statusPage is the number of agent records StatusSource reads per query.
const statusPage = 500

// AgentLister is the part of Store StatusSource reads.
type AgentLister interface {
	// ListAgents returns up to limit agents with an id above after, ascending.
	ListAgents(ctx context.Context, after ids.ID, limit int) ([]domain.Agent, error)
}

// StatusSource implements agents.StatusAgents over the stored records and the principals
// (M7-09). It is safe for concurrent use.
type StatusSource struct {
	store AgentLister
	dir   Principals
	clock clock.Clock
	stale time.Duration
}

// Compile-time check: StatusSource is the presence port of the status page.
var _ agents.StatusAgents = (*StatusSource)(nil)

// NewStatusSource returns the source over store and dir with cfg (agent.presence_timeout and
// agent.heartbeat_persist_interval give the age after which a stored presence is stale); c nil
// means clock.System(). Errors: a missing collaborator or a period that is not positive.
func NewStatusSource(store AgentLister, dir Principals, cfg PresenceConfig, c clock.Clock) (*StatusSource, error) {
	if store == nil || dir == nil {
		return nil, errors.New("agents: the status source needs the store and the principals")
	}
	if cfg.PresenceTimeout <= 0 || cfg.PersistInterval <= 0 {
		return nil, errors.New("agents: agent.presence_timeout and agent.heartbeat_persist_interval must be positive")
	}
	if c == nil {
		c = clock.System()
	}
	return &StatusSource{store: store, dir: dir, clock: c, stale: cfg.PresenceTimeout + cfg.PersistInterval}, nil
}

// ActiveAgents implements agents.StatusAgents: the records page by page, one principal read per
// page, the people of other states left out.
func (s *StatusSource) ActiveAgents(ctx context.Context) ([]agents.StatusAgent, error) {
	now := s.clock.Now()
	var out []agents.StatusAgent
	after := ids.ID{}
	for {
		recs, err := s.store.ListAgents(ctx, after, statusPage)
		if err != nil {
			return nil, err
		}
		if len(recs) == 0 {
			return out, nil
		}
		list := make([]ids.ID, len(recs))
		for i, r := range recs {
			list[i] = r.ID
		}
		principals, err := s.dir.Principals(ctx, list)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			p, ok := principals[r.ID]
			if !ok || p.Status != identity.StatusActive {
				continue
			}
			out = append(out, agents.StatusAgent{ID: r.ID, Handle: p.Handle, DisplayName: p.DisplayName, State: s.state(r.Reported, now)})
		}
		if len(recs) < statusPage {
			return out, nil
		}
		after = recs[len(recs)-1].ID
	}
}

// state is the stored presence, offline once the last heartbeat is older than the stale age.
func (s *StatusSource) state(r domain.Reported, now time.Time) string {
	st := r.State
	if st == "" {
		st = domain.RuntimeUnknown
	}
	if (st == domain.RuntimeOnline || st == domain.RuntimeDegraded) && now.Sub(r.LastHeartbeatAt) > s.stale {
		return agents.StateOffline
	}
	return string(st)
}
