// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// The field error codes of the operator rules (D1 of M1-54).
const (
	// FieldNotAllowedForMode marks an agent_id sent with a mode other than agent.
	FieldNotAllowedForMode = "not_allowed_for_mode"
	// FieldInheritOnlyForChannels marks mode inherit on a space, which has nothing to inherit from.
	FieldInheritOnlyForChannels = "inherit_only_for_channels"
	// FieldAgentNotActive marks an agent_id that names no active agent.
	FieldAgentNotActive = "agent_not_active"
)

// OperatorAssignment is the operator a request sets (OpenAPI OperatorSet without the reason):
// Mode inherit, none or agent, and AgentID the agent of mode agent, zero otherwise.
type OperatorAssignment struct {
	Mode    OperatorMode
	AgentID ids.ID
}

// CheckForChannel validates a for a channel: a known mode, agent_id exactly with mode agent.
// Errors: validation_failed on /mode or /agent_id.
func (a OperatorAssignment) CheckForChannel() error {
	return a.check(true)
}

// CheckForSpace validates a for the default operator of a space: like CheckForChannel, and
// inherit is refused because a space has nothing to inherit from. Errors: validation_failed on
// /mode (inherit_only_for_channels) or /agent_id.
func (a OperatorAssignment) CheckForSpace() error {
	return a.check(false)
}

// check is the shared rule; channel allows inherit.
func (a OperatorAssignment) check(channel bool) error {
	switch a.Mode {
	case OperatorInherit:
		if !channel {
			return fieldError("/mode", FieldInheritOnlyForChannels, "inherit applies to channels only")
		}
	case OperatorNone, OperatorAgent:
	default:
		return fieldError("/mode", FieldInvalid, "the mode must be inherit, none or agent")
	}
	switch {
	case a.Mode == OperatorAgent && a.AgentID.IsZero():
		return fieldError("/agent_id", FieldRequired, "mode agent requires agent_id")
	case a.Mode != OperatorAgent && !a.AgentID.IsZero():
		return fieldError("/agent_id", FieldNotAllowedForMode, "agent_id is allowed with mode agent only")
	}
	return nil
}

// EffectiveOperator returns the effective operator of a channel from its columns and the default
// operator of its space (03 channels: operator_inherit ? default : operator_agent_id); zero for
// none.
func EffectiveOperator(inherit bool, own, spaceDefault ids.ID) ids.ID {
	if inherit {
		return spaceDefault
	}
	return own
}

// Assignment returns the operator columns of the channel as an assignment.
func (c Channel) Assignment() OperatorAssignment {
	switch {
	case c.OperatorInherit:
		return OperatorAssignment{Mode: OperatorInherit}
	case !c.OperatorAgentID.IsZero():
		return OperatorAssignment{Mode: OperatorAgent, AgentID: c.OperatorAgentID}
	}
	return OperatorAssignment{Mode: OperatorNone}
}

// SetOperator writes the checked assignment a into the operator columns and bumps the version
// once; it reports false and changes nothing when a equals the current assignment.
func (c *Channel) SetOperator(a OperatorAssignment) bool {
	if c.Assignment() == a {
		return false
	}
	c.OperatorInherit = a.Mode == OperatorInherit
	c.OperatorAgentID = a.AgentID
	c.Version++
	return true
}

// Assignment returns the default operator of the space as an assignment: agent or none.
func (s Space) Assignment() OperatorAssignment {
	if s.DefaultOperatorAgentID.IsZero() {
		return OperatorAssignment{Mode: OperatorNone}
	}
	return OperatorAssignment{Mode: OperatorAgent, AgentID: s.DefaultOperatorAgentID}
}

// SetOperator writes the checked assignment a (none or agent) as the default operator and bumps
// the version once; it reports false and changes nothing when a equals the current default.
func (s *Space) SetOperator(a OperatorAssignment) bool {
	if s.Assignment() == a {
		return false
	}
	s.DefaultOperatorAgentID = a.AgentID
	s.Version++
	return true
}
