// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Commands is the register of slash commands (docs/spec/09-agents.md section 10, 03 commands;
// M2-16). The caller is the principal of ctx (kernel/actor). Every change authorizes
// command.register or command.remove and writes one audit entry and one command.changed in its
// unit of work; the reads authorize command.read. Errors are kernel/errs errors; an unknown, a
// removed and an invisible command give the same 404 not_found (02 section 9.2).
//
// Example:
//
//	c, err := cmds.Register(ctx, agents.RegisterCommand{Name: "escalate", Description: "Escalate the ticket"})
type Commands interface {
	// Register adds a command of the caller (registerCommand, command.register). Errors: 400
	// validation_failed for a definition outside the rules (limit_reached beyond
	// command.max_per_principal), 403 forbidden for a person or a target without write, 404 for an
	// invisible target, 409 conflict with reason command_name_reserved for a built-in name and
	// without reason for a taken name.
	Register(ctx context.Context, cmd RegisterCommand) (Command, error)
	// Update replaces the definition of a live command of the caller (updateCommand,
	// command.register, If-Match). Errors: those of Register, 404 for an unknown or invisible
	// command, 403 for a visible command of another principal, 428 without and 412
	// version_conflict with a stale If-Match. An identical definition answers the command
	// unchanged without a write.
	Update(ctx context.Context, cmd UpdateCommand) (Command, error)
	// Remove removes a command of the caller (removeCommand, command.remove); a removed command
	// answers nil without a write. Errors: 404 for an unknown or invisible command, 403 for a
	// visible command of another principal.
	Remove(ctx context.Context, commandID ids.ID) error
	// Get reads a live command (getCommand, command.read). Errors: 404.
	Get(ctx context.Context, commandID ids.ID) (Command, error)
	// ListForChannel lists the registered commands offered in a channel (listChannelCommands,
	// command.read on the channel; 09 section 10.3): the commands of the channel's effective
	// operator first, then the others, each group by lower-case name, owner and ID; per owner and
	// name only the most specific scope. A dm channel lists nothing. Errors: 404 for an unknown or
	// invisible channel.
	ListForChannel(ctx context.Context, channelID ids.ID) ([]Command, error)
	// Resolve returns the live command that is usable in the channel (the scope covers it and no
	// more specific command of the same owner and name shadows it). It does not authorize: the
	// caller authorizes command.invoke. Unknown, removed, not covered or shadowed give the same
	// not_found error, and so does an unknown channel.
	Resolve(ctx context.Context, commandID, channelID ids.ID) (Command, error)
}

// Command is a slash command as the API shows it (schema Command).
type Command struct {
	ID, OwnerPrincipalID ids.ID
	// OwnerKind is the principal kind of the owner: agent or app.
	OwnerKind         string
	Name, Description string
	// ScopeKind is instance, space or channel; ScopeID its target, zero for instance.
	ScopeKind string
	ScopeID   ids.ID
	// Options is never nil.
	Options []CommandOption
	// RequiredLevel is write or trigger.
	RequiredLevel string
	Control       bool
	Version       int64
}

// CommandOption is one option of a command (schema CommandOption, command_options.v1).
type CommandOption struct {
	Name, Description, Type string
	Required                bool
	// Choices are the static values; nil for none.
	Choices      []CommandChoice
	Autocomplete bool
}

// CommandChoice is one static value of an option (schema CommandChoice).
type CommandChoice struct {
	Name, Value string
}

// RegisterCommand is a submitted definition (schema CommandRegister). Zero values are absent
// members: ScopeKind "" is instance, Options nil an empty list, RequiredLevel "" trigger.
type RegisterCommand struct {
	Name, Description, ScopeKind string
	ScopeID                      ids.ID
	Options                      []CommandOption
	RequiredLevel                string
	Control                      bool
}

// UpdateCommand is the input of Commands.Update: the command, the If-Match of the request and the
// new definition. The owner and the ID never change.
type UpdateCommand struct {
	CommandID  ids.ID
	IfMatch    Precondition
	Definition RegisterCommand
}

// OwnedCommands changes the commands of an owner for an owner action that another module already
// authorized (M2-25 design note 2 d): the module apps writes the commands of an app's manifest at
// app.install and app.upgrade and removes them at app.uninstall, decided and audited there with
// step-up. The methods call no authorize, write no audit entry of command.register or
// command.remove (the app did not act), and write command.changed exactly when a row changes. They
// join the ambient unit of work of ctx; outside one they fail with internal. Errors are
// kernel/errs errors: 400 validation_failed for a definition outside the rules (limit_reached
// beyond command.max_per_principal), 409 conflict with reason command_name_reserved for a built-in
// name.
//
// Example:
//
//	c, changed, err := owned.UpsertOwned(ctx, appID, agents.RegisterCommand{Name: "ticket", Description: "Open a ticket"})
type OwnedCommands interface {
	// UpsertOwned creates the instance-wide command def of ownerID, or replaces the fields of the
	// owner's live instance-wide command with the same name compared case-insensitively. changed
	// is false when the stored definition already equals def (no write, no event).
	UpsertOwned(ctx context.Context, ownerID ids.ID, def RegisterCommand) (c Command, changed bool, err error)
	// RemoveOwned removes the live instance-wide commands of ownerID named in names (compared
	// case-insensitively) and returns the IDs it removed; a name without a live command is
	// skipped.
	RemoveOwned(ctx context.Context, ownerID ids.ID, names []string) ([]ids.ID, error)
	// RemoveAllOwned removes every live command of ownerID, whatever its scope, and returns their
	// IDs in ascending order.
	RemoveAllOwned(ctx context.Context, ownerID ids.ID) ([]ids.ID, error)
}
