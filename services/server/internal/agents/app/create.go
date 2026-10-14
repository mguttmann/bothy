// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// instanceResource is the resource of agent.create and agent.billing_mode.set_all.
var instanceResource = authorize.Resource{Kind: authorize.ResourceInstance}

// Create implements agents.Agents. A first unit authorizes agent.create once (owner, step-up)
// and validates the body, so a refused call never raises the rights version; a second unit,
// declared as the rights change agent.create for the new agent, creates the principal through
// identity (handle and display name rules of M1-26, case-insensitive uniqueness) and the record
// with the defaults of 03, records the allow entry and appends agent.updated.
func (s *Service) Create(ctx context.Context, a actor.Actor, cmd agents.CreateCmd) (agents.Agent, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.Agent{}, err
	}
	var rec domain.Agent
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionAgentCreate, instanceResource); err != nil {
			return err
		}
		var err error
		rec, err = newRecord(cmd)
		return err
	})
	if err != nil {
		return agents.Agent{}, err
	}
	var out agents.Agent
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if _, err := s.d.Principals.CreatePrincipal(ctx, identity.CreatePrincipalCmd{
			ID: rec.ID, Kind: actor.KindAgent, Handle: cmd.Handle, DisplayName: cmd.DisplayName,
		}); err != nil {
			return err
		}
		if err := s.d.Store.InsertAgent(ctx, rec); err != nil {
			return err
		}
		var err error
		out, err = s.recordAndEmit(ctx, authorize.ActionAgentCreate, rec, auditing.Attrs{attrAdapterKind: auditing.Code(rec.AdapterKind)})
		return err
	}, rightsChange(authorize.ActionAgentCreate, agentRef(rec.ID), rec.ID))
	return out, err
}

// newRecord validates the body of createAgent and returns the new record with a fresh id.
func newRecord(cmd agents.CreateCmd) (domain.Agent, error) {
	if err := domain.CheckAdapterKind(cmd.AdapterKind); err != nil {
		return domain.Agent{}, err
	}
	if err := domain.CheckDescription(cmd.Description); err != nil {
		return domain.Agent{}, err
	}
	cfg, err := domain.ParseOperatorConfig(cmd.OperatorConfig)
	if err != nil {
		return domain.Agent{}, err
	}
	return domain.NewAgent(ids.New(), cmd.AdapterKind, cmd.Description, cfg), nil
}

// Update implements agents.Agents in one unit of work (agent.update is no rights change): it
// authorizes before the row lock, so a refused caller never waits on it, checks If-Match against
// the locked version, changes the display name through identity and the record, and bumps the
// version once. An update without change records no_change and appends nothing.
func (s *Service) Update(ctx context.Context, a actor.Actor, id ids.ID, cmd agents.UpdateCmd) (agents.Agent, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.Agent{}, err
	}
	var out agents.Agent
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.update(ctx, a, id, cmd)
		return err
	})
	return out, err
}

// update is Update inside its unit of work.
func (s *Service) update(ctx context.Context, a actor.Actor, id ids.ID, cmd agents.UpdateCmd) (agents.Agent, error) {
	if _, err := s.load(ctx, a, authorize.ActionAgentUpdate, id); err != nil {
		return agents.Agent{}, err
	}
	rec, err := s.lock(ctx, id)
	if err != nil {
		return agents.Agent{}, err
	}
	if err := s.precondition(ctx, cmd.IfMatch, rec); err != nil {
		return agents.Agent{}, err
	}
	cfg, err := parseUpdate(cmd)
	if err != nil {
		return agents.Agent{}, err
	}
	changed, err := s.rename(ctx, id, cmd.DisplayName)
	if err != nil {
		return agents.Agent{}, err
	}
	version := rec.Version
	changed = append(changed, rec.Update(cmd.Description, cfg)...)
	if len(changed) == 0 {
		v, err := s.view(ctx, rec)
		if err != nil {
			return agents.Agent{}, err
		}
		return v, s.noChange(ctx, authorize.ActionAgentUpdate, id, "")
	}
	if rec.Version == version {
		rec.Bump()
	}
	return s.changedAndEmit(ctx, authorize.ActionAgentUpdate, rec, auditing.Attrs{attrChangedFields: codes(changed)})
}

// rename changes the display name through identity when name is given and returns
// ["display_name"] when it changed.
func (s *Service) rename(ctx context.Context, id ids.ID, name *string) ([]string, error) {
	if name == nil {
		return nil, nil
	}
	_, renamed, err := s.d.Principals.SetDisplayName(ctx, id, *name)
	if err != nil || !renamed {
		return nil, err
	}
	return []string{"display_name"}, nil
}

// parseUpdate validates the description and the operator configuration of agent.update; the
// result is nil when the request keeps the stored configuration.
func parseUpdate(cmd agents.UpdateCmd) (cfg *domain.OperatorConfig, err error) {
	if cmd.Description != nil {
		if err := domain.CheckDescription(*cmd.Description); err != nil {
			return nil, err
		}
	}
	if cmd.OperatorConfig != nil {
		c, err := domain.ParseOperatorConfig(cmd.OperatorConfig)
		if err != nil {
			return nil, err
		}
		cfg = &c
	}
	return cfg, nil
}
