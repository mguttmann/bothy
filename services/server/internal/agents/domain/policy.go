// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// BillingMode is the backend policy of an agent, the column agents.billing_mode (10 section 1).
type BillingMode string

const (
	// Personal runs on the subscription of exactly one person, the credential holder (I1).
	Personal BillingMode = "personal"
	// Organization runs on an organisation key from the secret provider of the adapter.
	Organization BillingMode = "organization"
)

// Valid reports whether m is a value of the check on agents.billing_mode.
func (m BillingMode) Valid() bool { return m == Personal || m == Organization }

// The JSON pointers of the policy fields (BillingModeSet, CredentialHolderSet).
const (
	pointerBillingMode = "/billing_mode"
	pointerHolder      = "/credential_holder_id"
)

// Policy is the target backend policy of an agent: the mode and, exactly for personal, the
// credential holder (03 agents check, 10 section 9).
type Policy struct {
	// Mode is personal or organization.
	Mode BillingMode
	// HolderID is the credential holder; zero for organization.
	HolderID ids.ID
}

// To decides the switch from p to target (10 section 9.1): organization is always allowed and
// takes no holder; personal needs one. Whether the holder is an active person and whether I1
// holds afterwards is for the use case and the rights hook. changed is false when target equals
// p (a call without effect, 06 section 2).
//
// Errors: validation_failed on /billing_mode for an unknown mode and on /credential_holder_id for
// personal without a holder or organization with one.
func (p Policy) To(target Policy) (changed bool, err error) {
	switch target.Mode {
	case Organization:
		if !target.HolderID.IsZero() {
			return false, fieldError(pointerHolder, FieldNotAllowed, "organization takes no credential holder")
		}
	case Personal:
		if target.HolderID.IsZero() {
			return false, fieldError(pointerHolder, FieldRequired, "personal requires a credential holder")
		}
	default:
		return false, fieldError(pointerBillingMode, FieldInvalid, "unknown billing mode")
	}
	return target != p, nil
}

// ParseBillingMode validates s. Errors: validation_failed on /billing_mode.
func ParseBillingMode(s string) (BillingMode, error) {
	m := BillingMode(s)
	if !m.Valid() {
		return "", fieldError(pointerBillingMode, FieldInvalid, "unknown billing mode")
	}
	return m, nil
}

// NotPersonal is the conflict of agent.credential_holder.set on an agent that is not personal
// (brief M1-51: only for personal agents).
func NotPersonal() error {
	return errs.New(errs.CodeConflict, "Only a personal agent has a credential holder.").WithReason("billing_mode_not_personal")
}

// ToolClass is the tool class of an agent runtime (07 section 2.11), ordered chat < scoped < host.
type ToolClass string

const (
	// ToolChat answers in the thread only.
	ToolChat ToolClass = "chat"
	// ToolScoped uses named tools with minimal credentials.
	ToolScoped ToolClass = "scoped"
	// ToolHost has a shell, the file system or free network access.
	ToolHost ToolClass = "host"
)

// rank is the order of the classes; 0 for no class.
func (c ToolClass) rank() int {
	switch c {
	case ToolChat:
		return 1
	case ToolScoped:
		return 2
	case ToolHost:
		return 3
	}
	return 0
}

// Valid reports whether c is a value of the check on agents.tool_class.
func (c ToolClass) Valid() bool { return c.rank() > 0 }

// ParseToolClass validates s. Errors: validation_failed on /tool_class.
func ParseToolClass(s string) (ToolClass, error) {
	c := ToolClass(s)
	if !c.Valid() {
		return "", fieldError("/tool_class", FieldInvalid, "unknown tool class")
	}
	return c, nil
}

// EffectiveToolClass is the class authorize reckons with: the higher of the target class and the
// reported class; an empty reported class counts as none (09 section 2.4).
func EffectiveToolClass(target, reported ToolClass) ToolClass {
	if reported.rank() > target.rank() {
		return reported
	}
	return target
}

// RuntimeState is the reported state of the runtime, agents.reported_runtime_state.
type RuntimeState string

// The reported states; unknown until a heartbeat arrives (M2-20).
const (
	RuntimeUnknown  RuntimeState = "unknown"
	RuntimeOnline   RuntimeState = "online"
	RuntimeDegraded RuntimeState = "degraded"
	RuntimeOffline  RuntimeState = "offline"
)
