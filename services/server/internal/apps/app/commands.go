// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The commands and components of an app across its lifecycle (docs/spec/11-extensions.md sections
// 4.1 to 4.3, brief M2-25 design note 11). The owner's action is audited by the use case with
// step-up; its details gain the command IDs and the number of disabled components. No
// command.register or command.remove entry of their own: the app did not act. No command.changed
// when only the reachability changes (suspend, resume, scopes): the rows do not change.

// registerOf converts a manifest command into the definition of agents: instance-wide, because a
// manifest names no target (11 section 4.1).
func registerOf(c domain.Command) agents.RegisterCommand {
	out := agents.RegisterCommand{Name: c.Name, Description: c.Description, RequiredLevel: c.RequiredLevel,
		Control: c.Control != nil && *c.Control}
	for _, o := range c.Options {
		opt := agents.CommandOption{Name: o.Name, Description: o.Description, Type: o.Type, Required: o.Required,
			Autocomplete: o.Autocomplete != nil && *o.Autocomplete}
		for _, ch := range o.Choices {
			opt.Choices = append(opt.Choices, agents.CommandChoice{Name: ch.Name, Value: ch.Value})
		}
		out.Options = append(out.Options, opt)
	}
	return out
}

// upsertCommands writes cmds for the app through agents and returns the IDs of the commands it
// changed (created or replaced).
func (s *Service) upsertCommands(ctx context.Context, appID ids.ID, cmds []domain.Command) ([]ids.ID, error) {
	var out []ids.ID
	for _, c := range cmds {
		got, changed, err := s.d.Commands.UpsertOwned(ctx, appID, registerOf(c))
		if err != nil {
			return nil, err
		}
		if changed {
			out = append(out, got.ID)
		}
	}
	return out, nil
}

// installCommands writes the commands of the manifest of a new installation (11 section 4.1: in
// the install transaction, before grants and tokens) and names them in attrs.
func (s *Service) installCommands(ctx context.Context, appID ids.ID, m domain.Manifest, attrs auditing.Attrs) error {
	added, err := s.upsertCommands(ctx, appID, m.Commands)
	if err != nil {
		return err
	}
	attrs[attrCommandsAdded] = idList(added)
	return nil
}

// upgradeCommands applies the difference of the commands of old and next (11 section 4.2): removed
// commands through RemoveOwned first, so they free their place under command.max_per_principal,
// then changed and added ones through UpsertOwned; commands the app registered through the API and
// that neither manifest names stay. It names the IDs in attrs.
func (s *Service) upgradeCommands(ctx context.Context, appID ids.ID, old, next domain.Manifest, attrs auditing.Attrs) error {
	d := domain.DiffCommands(old, next)
	removed, err := s.d.Commands.RemoveOwned(ctx, appID, d.Removed)
	if err != nil {
		return err
	}
	changed, err := s.upsertCommands(ctx, appID, d.Changed)
	if err != nil {
		return err
	}
	added, err := s.upsertCommands(ctx, appID, d.Added)
	if err != nil {
		return err
	}
	attrs[attrCommandsAdded] = idList(added)
	attrs[attrCommandsChanged] = idList(changed)
	attrs[attrCommandsRemoved] = idList(removed)
	return nil
}

// suspendParts disables the components of the app's cards (11 section 4.3); its commands are
// hidden through the app-owner guard of authz because the installation is not active.
func (s *Service) suspendParts(ctx context.Context, appID ids.ID, attrs auditing.Attrs) error {
	n, err := s.d.Components.DisableAuthoredBy(ctx, appID)
	if err != nil {
		return err
	}
	attrs[attrComponentsDisabled] = auditing.Int(int64(n))
	return nil
}

// uninstallCommands removes every command of the app and disables the components of its cards (11
// section 4.3); its messages keep their author.
func (s *Service) uninstallCommands(ctx context.Context, appID ids.ID, attrs auditing.Attrs) error {
	removed, err := s.d.Commands.RemoveAllOwned(ctx, appID)
	if err != nil {
		return err
	}
	attrs[attrCommandsRemoved] = idList(removed)
	return s.suspendParts(ctx, appID, attrs)
}
