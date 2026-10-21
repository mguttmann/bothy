// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Reported is the actual state the adapter reported in its last stored heartbeat (03 agents,
// reported_* columns and reported_details); the heartbeat of M2-20 writes it. The zero value is
// "no heartbeat".
type Reported struct {
	// State is reported_runtime_state; unknown without a heartbeat.
	State RuntimeState
	// BillingMode is reported_billing_mode; empty when the adapter follows the target per run.
	BillingMode BillingMode
	// ToolClass is reported_tool_class; empty for none.
	ToolClass ToolClass
	// RuntimeVersion is reported_runtime_version; empty for none.
	RuntimeVersion string
	// LastHeartbeatAt is last_heartbeat_at; zero for none.
	LastHeartbeatAt time.Time
	// BillingModesSupported comes from reported_details.billing_modes_supported.
	BillingModesSupported []BillingMode
	// Evidence comes from reported_details.billing_evidence; nil when not reported.
	Evidence *Evidence
	// Followups is reported_details.capabilities.followups (steer or queue); "" when not
	// reported. The effective capability also needs the owner's supports_followups (09 2.2).
	Followups string
}

// Agent is the operating record of an agent principal, a row of agents (03 section 4.4). The
// identity (handle, display name, status) belongs to identity; this entity holds the operation,
// the backend policy and the tool class. Every change bumps Version by one (03 principle 10).
type Agent struct {
	// ID is principals.id of the agent.
	ID ids.ID
	// AdapterKind is the free identifier of the adapter (CheckAdapterKind).
	AdapterKind string
	// Description may be empty.
	Description string
	// Config is agents.operator_config.
	Config OperatorConfig
	// Policy is the target backend policy.
	Policy Policy
	// BillingModeChangedAt and BillingModeChangedBy record the last switch; zero before one.
	BillingModeChangedAt time.Time
	BillingModeChangedBy ids.ID
	// ToolClass is the target tool class.
	ToolClass ToolClass
	// Reported is the actual state of the last heartbeat.
	Reported Reported
	// Version is agents.version, the ETag of the agent.
	Version int64
}

// NewAgent returns a new record with the defaults of 03: organization, tool class chat, state
// unknown, version 1. The caller validated adapterKind, description and config.
func NewAgent(id ids.ID, adapterKind, description string, config OperatorConfig) Agent {
	return Agent{
		ID: id, AdapterKind: adapterKind, Description: description, Config: config,
		Policy: Policy{Mode: Organization}, ToolClass: ToolChat, Reported: Reported{State: RuntimeUnknown}, Version: 1,
	}
}

// Bump raises Version by one; a change of the agent's display name in identity calls it, because
// the display name is part of the representation whose ETag is Version.
func (a *Agent) Bump() { a.Version++ }

// Update applies the given fields of agent.update; nil keeps a field. changed lists the JSON names
// of the fields whose value differs, in schema order; Version is bumped once when one does.
func (a *Agent) Update(description *string, config *OperatorConfig) (changed []string) {
	if description != nil && *description != a.Description {
		a.Description = *description
		changed = append(changed, "description")
	}
	if config != nil && !config.Equal(a.Config) {
		a.Config = *config
		changed = append(changed, "operator_config")
	}
	if len(changed) > 0 {
		a.Bump()
	}
	return changed
}

// SetPolicy switches the backend policy to target (agent.billing_mode.set) by the principal by at
// time at; changed is false for the same target, which writes nothing. Errors: those of
// Policy.To.
func (a *Agent) SetPolicy(target Policy, by ids.ID, at time.Time) (changed bool, err error) {
	changed, err = a.Policy.To(target)
	if err != nil || !changed {
		return false, err
	}
	a.Policy, a.BillingModeChangedAt, a.BillingModeChangedBy = target, at, by
	a.Bump()
	return true, nil
}

// SetHolder changes the credential holder of a personal agent (agent.credential_holder.set);
// changed is false for the same holder. Errors: NotPersonal for an organization agent.
func (a *Agent) SetHolder(holder ids.ID) (changed bool, err error) {
	if a.Policy.Mode != Personal {
		return false, NotPersonal()
	}
	changed, err = a.Policy.To(Policy{Mode: Personal, HolderID: holder})
	if err != nil || !changed {
		return false, err
	}
	a.Policy.HolderID = holder
	a.Bump()
	return true, nil
}

// SetToolClass sets the target tool class (agent.tool_class.set); changed is false for the same
// class. The caller validated c (ParseToolClass).
func (a *Agent) SetToolClass(c ToolClass) (changed bool) {
	if c == a.ToolClass {
		return false
	}
	a.ToolClass = c
	a.Bump()
	return true
}

// EffectiveToolClass is the higher of the target class and the reported class.
func (a Agent) EffectiveToolClass() ToolClass {
	return EffectiveToolClass(a.ToolClass, a.Reported.ToolClass)
}

// PresenceView is the reported state as the API shows it (schema AgentRuntimeStatus).
type PresenceView struct {
	State                 RuntimeState
	ReportedBillingMode   BillingMode
	RuntimeVersion        string
	LastHeartbeatAt       time.Time
	BillingModesSupported []BillingMode
	ReportedToolClass     ToolClass
	// BillingModeMatches is false only when a reported mode exists and differs from the target.
	BillingModeMatches bool
}

// Presence returns the presence view of a.
func (a Agent) Presence() PresenceView {
	r := a.Reported
	state := r.State
	if state == "" {
		state = RuntimeUnknown
	}
	return PresenceView{
		State: state, ReportedBillingMode: r.BillingMode, RuntimeVersion: r.RuntimeVersion, LastHeartbeatAt: r.LastHeartbeatAt,
		BillingModesSupported: append([]BillingMode(nil), r.BillingModesSupported...), ReportedToolClass: r.ToolClass,
		BillingModeMatches: r.BillingMode == "" || r.BillingMode == a.Policy.Mode,
	}
}

// ReportedDetailsFromStore reads billing_modes_supported from agents.reported_details (schema
// agent_heartbeat.v1); unknown values are left out, because the server shows only modes it knows.
// Errors: a value that is not a JSON object.
func ReportedDetailsFromStore(raw []byte) ([]BillingMode, error) {
	var d struct {
		BillingModesSupported []string `json:"billing_modes_supported"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	var out []BillingMode
	for _, s := range d.BillingModesSupported {
		if m := BillingMode(s); m.Valid() {
			out = append(out, m)
		}
	}
	return out, nil
}
