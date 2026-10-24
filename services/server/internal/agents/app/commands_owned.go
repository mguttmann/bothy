// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Compile-time check: the owned-command port of apps (M2-25).
var _ agents.OwnedCommands = (*CommandService)(nil)

// ownerKindApp is the owner kind of the commands written through OwnedCommands: only the module
// apps calls it, for the commands of an app.
const ownerKindApp = "app"

// needCommandUnit refuses an owned change outside the caller's unit of work: the commands of a
// manifest commit with the owner action that writes them (11 section 4.1).
func needCommandUnit(ctx context.Context) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "agents: an owned command change outside a unit of work")
	}
	return nil
}

// ownedByName returns the live instance-wide command of owner named name (case-insensitive).
func (s *CommandService) ownedByName(ctx context.Context, owner ids.ID, name string) (domain.Command, bool, error) {
	list, err := s.d.Store.CoveringCommands(ctx, CoveringQuery{OwnerID: owner, Name: name})
	if err != nil {
		return domain.Command{}, false, err
	}
	for _, c := range list {
		if c.Def.ScopeKind == domain.ScopeInstance && strings.EqualFold(c.Def.Name, name) {
			return c, true, nil
		}
	}
	return domain.Command{}, false, nil
}

// UpsertOwned implements agents.OwnedCommands (M2-25 design note 2 d): the rules of
// registerCommand (definition, reserved names, command.max_per_principal) without authorize and
// without an audit entry; version + 1 and command.changed only on a real change.
func (s *CommandService) UpsertOwned(ctx context.Context, ownerID ids.ID, def agents.RegisterCommand) (agents.Command, bool, error) {
	if err := needCommandUnit(ctx); err != nil {
		return agents.Command{}, false, err
	}
	if def.ScopeKind != "" && def.ScopeKind != domain.ScopeInstance || !def.ScopeID.IsZero() {
		return agents.Command{}, false, errs.New(errs.CodeInternalError, "agents: an owned command is instance-wide")
	}
	d, err := s.validate(def)
	if err != nil {
		return agents.Command{}, false, err
	}
	if domain.IsReservedCommandName(d.Name) {
		return agents.Command{}, false, reserved()
	}
	if err := s.d.Principals.LockPrincipal(ctx, ownerID); err != nil {
		return agents.Command{}, false, err
	}
	cur, found, err := s.ownedByName(ctx, ownerID, d.Name)
	if err != nil {
		return agents.Command{}, false, err
	}
	if found {
		if d.Equal(cur.Def) {
			return commandView(cur), false, nil
		}
		c, err := s.replaceOwned(ctx, cur, d)
		return commandView(c), err == nil, err
	}
	n, err := s.d.Store.CountLiveCommands(ctx, ownerID)
	if err != nil {
		return agents.Command{}, false, err
	}
	if n >= s.cfg.MaxPerPrincipal {
		return agents.Command{}, false, domain.CommandField("", domain.CodeLimitReached, "the owner has the most commands allowed")
	}
	c, err := s.d.Store.InsertCommand(ctx, domain.Command{ID: ids.New(), OwnerID: ownerID, OwnerKind: ownerKindApp, Def: d})
	if errors.Is(err, ErrCommandTaken) {
		return agents.Command{}, false, taken()
	}
	if err != nil {
		return agents.Command{}, false, err
	}
	return commandView(c), true, s.announce(ctx, "", c, changeRegistered)
}

// replaceOwned writes def over the owned command c with the compare-and-swap of its version and
// appends command.changed without an audit entry.
func (s *CommandService) replaceOwned(ctx context.Context, c domain.Command, def domain.CommandDefinition) (domain.Command, error) {
	v, changed, err := s.d.Store.UpdateCommand(ctx, c.ID, c.Version, def)
	if errors.Is(err, ErrCommandTaken) {
		return c, taken()
	}
	if err != nil {
		return c, err
	}
	if !changed {
		return c, s.lostRace(ctx, c.ID)
	}
	c.Def, c.Version = def, v
	return c, s.announce(ctx, "", c, changeUpdated)
}

// RemoveOwned implements agents.OwnedCommands (M2-25 design note 2 d).
func (s *CommandService) RemoveOwned(ctx context.Context, ownerID ids.ID, names []string) ([]ids.ID, error) {
	if err := needCommandUnit(ctx); err != nil {
		return nil, err
	}
	var out []ids.ID
	for _, name := range names {
		c, found, err := s.ownedByName(ctx, ownerID, name)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if err := s.removeOwned(ctx, c); err != nil {
			return nil, err
		}
		out = append(out, c.ID)
	}
	return out, nil
}

// RemoveAllOwned implements agents.OwnedCommands (M2-25 design note 2 d).
func (s *CommandService) RemoveAllOwned(ctx context.Context, ownerID ids.ID) ([]ids.ID, error) {
	if err := needCommandUnit(ctx); err != nil {
		return nil, err
	}
	list, err := s.d.Store.LiveCommandsOfOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]ids.ID, 0, len(list))
	for _, c := range list {
		if err := s.removeOwned(ctx, c); err != nil {
			return nil, err
		}
		out = append(out, c.ID)
	}
	return out, nil
}

// removeOwned sets removed_at of c with the compare-and-swap of its version and appends
// command.changed without an audit entry.
func (s *CommandService) removeOwned(ctx context.Context, c domain.Command) error {
	v, changed, err := s.d.Store.RemoveCommand(ctx, c.ID, c.Version)
	if err != nil {
		return err
	}
	if !changed {
		return s.lostRace(ctx, c.ID)
	}
	c.Version = v
	return s.announce(ctx, "", c, changeRemoved)
}
