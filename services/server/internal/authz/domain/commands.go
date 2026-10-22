// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The rules of slash commands (docs/spec/09-agents.md section 10, brief M2-16 design note 8):
// the visibility of the resource kind command, the target guard of command.register and the
// requirement and guard of command.invoke. The application layer gathers CommandFacts.

// The values of CommandFacts.OwnerKind and ScopeKind (03 commands).
const (
	commandOwnerAgent = KindAgent
	commandOwnerApp   = KindApp
	scopeInstance     = "instance"
	scopeSpace        = "space"
	scopeChannel      = "channel"
	kindCommand       = "command"
)

// The actions with command rules.
const (
	actionCommandRead     = "command.read"
	actionCommandRegister = "command.register"
	actionCommandInvoke   = "command.invoke"
)

// CommandFacts are the facts of a command resource and of command.invoke, gathered by the
// application layer from kernel/authorize.CommandFacts, the principal and policy sources and the
// structure.
type CommandFacts struct {
	// OwnerID, OwnerKind and the scope are the stored or the submitted definition.
	OwnerID                      ids.ID
	OwnerKind                    string
	ScopeKind                    string
	ScopeSpaceID, ScopeChannelID ids.ID
	// Offered marks command.read for one item of a channel list (OfferedInChannelID): the path is
	// that channel, and the rule of 09 section 10.3 decides.
	Offered bool
	// OwnerActive is the status active of the owner principal; OwnerLevel its own effective level
	// on the offered channel (operator rule included); OwnerBillingMode its billing mode (agents).
	OwnerActive      bool
	OwnerLevel       Level
	OwnerBillingMode string
	// AllowMentionedAgents is the switch of the offered channel.
	AllowMentionedAgents bool
	// SpaceDefaultOperator reports, for command.register on a space, that the principal is the
	// default operator of the space.
	SpaceDefaultOperator bool
	// RequiredLevel, Control and ActiveRun are the facts of command.invoke.
	RequiredLevel string
	Control       bool
	ActiveRun     bool
}

// extraResourceKinds are resource kinds an action takes besides the column "Ressource" of 05:
// command.read on a single command (getCommand, updateCommand, removeCommand, the items of
// listChannelCommands and the receive check of command.changed), whose visibility rule decides
// (brief M2-16 design note 8 d; docs/DECISIONS.md D-M2-16-8).
var extraResourceKinds = map[string][]string{
	actionCommandRead: {kindCommand},
}

// ActionResourceKinds returns the resource kinds of spec: its catalogue column and the extra kinds of
// this package.
func ActionResourceKinds(spec ActionSpec) []string {
	out := slices.Clone(spec.Resources)
	for _, k := range extraResourceKinds[spec.ID] {
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// CommandLevel is the requirement of command.invoke: Level(required_level) of the command (05
// section 4.10 "required_level of the command"). A missing command or an unknown level is an
// internal error.
var CommandLevel = Expr{op: opCommandLevel}

// commandLevel evaluates CommandLevel.
func (v evaluator) commandLevel() (bool, Reason, error) {
	c := v.in.Command
	if c == nil {
		return false, "", internalError("command.invoke without its command")
	}
	switch c.RequiredLevel {
	case levelNames[LevelWrite]:
		return v.level(LevelWrite), ReasonLevelInsufficient, nil
	case levelNames[LevelTrigger]:
		return v.level(LevelTrigger), ReasonLevelInsufficient, nil
	}
	return false, "", internalError("command with an unknown required level")
}

// commandVisible is the visibility of kind command (07 section 4.3, brief M2-16 design note 8 b).
// Without Offered: the owner, an effective owner, or a principal that sees the scope target: read
// on the channel, the space visible, any principal for the instance. The path is the scope target
// (or the scope of the event). With Offered the path is the channel C of the list: read on C, C not
// a dm channel, the command's scope covers C, the owner active, and the owner is C's effective
// operator, or an agent guest (allow_mentioned_agents, own level at least write, billing mode
// organization), or an app with level at least write.
func commandVisible(in Input) (bool, Reason, error) {
	c := in.Command
	if c == nil || !c.Offered {
		return isSelf(in) || in.Eff.Owner || scopeReadable(in), ReasonNoGrant, nil
	}
	if in.Path.Kind != PathChannel || in.Path.DM || in.Eff.Level < LevelRead || !c.OwnerActive || !commandCovers(*c, in.Path) {
		return false, ReasonNoGrant, nil
	}
	return offeredBy(in.Path, *c), ReasonNoGrant, nil
}

// scopeReadable reports that the principal sees the scope target that the path describes.
func scopeReadable(in Input) bool {
	switch in.Path.Kind {
	case PathChannel:
		return channelReadable(in)
	case PathSpace:
		return in.Eff.Level >= LevelRead || in.SpaceHasVisibleChannel
	case PathInstance:
		return in.Principal.Kind != KindAnonymous
	case PathNone:
	}
	return false
}

// commandCovers reports that the scope of c covers the channel of path (09 section 10.3).
func commandCovers(c CommandFacts, path Path) bool {
	switch c.ScopeKind {
	case scopeInstance:
		return true
	case scopeSpace:
		return !c.ScopeSpaceID.IsZero() && c.ScopeSpaceID == path.SpaceID
	case scopeChannel:
		return !c.ScopeChannelID.IsZero() && c.ScopeChannelID == path.ChannelID
	}
	return false
}

// offeredBy is the owner rule of 09 section 10.3 on the channel of path.
func offeredBy(path Path, c CommandFacts) bool {
	if !path.ChannelOperatorID.IsZero() && path.ChannelOperatorID == c.OwnerID {
		return true
	}
	switch c.OwnerKind {
	case commandOwnerAgent:
		return c.AllowMentionedAgents && c.OwnerLevel >= LevelWrite &&
			c.OwnerBillingMode == BillingOrganization
	case commandOwnerApp:
		return c.OwnerLevel >= LevelWrite
	}
	return false
}

// commandRegisterGuard is the target rule of command.register (09 section 10.2, brief M2-16 design
// note 8 c): instance has no further condition; a space must be visible (else not_found) and the
// principal its default operator or with write on it (else level_insufficient); a channel must not
// be a dm channel and be readable (else not_found), and the principal needs write on it, the
// operator rule included (else level_insufficient).
func commandRegisterGuard(in *Input) (Decision, error) {
	c := in.Command
	if c == nil {
		return Decision{}, internalError("command.register without its command")
	}
	switch c.ScopeKind {
	case scopeInstance:
		return Allow, nil
	case scopeSpace:
		if in.Path.Kind != PathSpace || in.Path.SpaceID != c.ScopeSpaceID {
			return Decision{}, internalError("command.register on a space without its path")
		}
		return targetDecision(in.Eff.Level >= LevelRead || in.SpaceHasVisibleChannel, c.SpaceDefaultOperator || in.Eff.Level >= LevelWrite), nil
	case scopeChannel:
		if in.Path.Kind != PathChannel || in.Path.ChannelID != c.ScopeChannelID {
			return Decision{}, internalError("command.register on a channel without its path")
		}
		return targetDecision(!in.Path.DM && in.Eff.Level >= LevelRead, in.Eff.Level >= LevelWrite), nil
	}
	return Decision{}, internalError("command.register with an unknown scope kind")
}

// targetDecision is the outcome of the target rule: an invisible target is not_found, a visible
// one without the right level_insufficient.
func targetDecision(visible, served bool) Decision {
	switch {
	case !visible:
		return deny(OutcomeNotFound, ReasonNoGrant)
	case !served:
		return deny(OutcomeForbidden, ReasonLevelInsufficient)
	}
	return Allow
}

// commandInvokeGuard is the guard of command.invoke (05 section 4.10, 09 section 10.4, brief M2-16
// design note 8 e): an app owner is an internal error until the app path exists (fail closed); a
// model command in a thread with an active run is forbidden with run_active (07 section 4.1).
func commandInvokeGuard(in *Input) (Decision, error) {
	c := in.Command
	if c == nil {
		return Decision{}, internalError("command.invoke without its command")
	}
	if c.OwnerKind != commandOwnerAgent {
		return Decision{}, internalError("command.invoke of a command whose owner is not an agent")
	}
	if !c.Control && c.ActiveRun {
		return deny(OutcomeForbidden, ReasonRunActive), nil
	}
	return Allow, nil
}
