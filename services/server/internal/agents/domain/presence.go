// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The warning codes of AgentRuntimeStatus.warnings (10 section 8, 09 section 2.4,
// docs/DECISIONS.md D-M2-20-7).
const (
	// WarningBillingModeMismatch: the reported mode differs from the configured one for longer
	// than billing.mismatch_alert_after (10 section 8 row 1).
	WarningBillingModeMismatch = "billing_mode_mismatch"
	// WarningBillingModeUnsupported: no reported mode, and the configured one is not among the
	// modes the adapter can prove (row 2).
	WarningBillingModeUnsupported = "billing_mode_unsupported"
	// WarningBillingEvidenceUnknown: the runtime cannot determine its mode (row 3).
	WarningBillingEvidenceUnknown = "billing_evidence_unknown"
	// WarningToolClassMismatch: the reported tool class differs from the configured one (09
	// section 2.4).
	WarningToolClassMismatch = "tool_class_mismatch"
)

// Present reports whether s is a state of a running adapter (online or degraded).
func (s RuntimeState) Present() bool { return s == RuntimeOnline || s == RuntimeDegraded }

// StateAt derives the presence state at now (04 section 12 item 2): unknown without any
// heartbeat, offline once now has reached last + timeout (agent.presence_timeout; the boundary
// is inclusive, D-M2-20-4), otherwise the reported state.
func StateAt(last time.Time, reported RuntimeState, now time.Time, timeout time.Duration) RuntimeState {
	switch {
	case last.IsZero():
		return RuntimeUnknown
	case !now.Before(last.Add(timeout)):
		return RuntimeOffline
	}
	return reported
}

// StoredPresence is the stored presence of an agent that the start rule reads.
type StoredPresence struct {
	// State is reported_runtime_state.
	State RuntimeState
	// LastHeartbeatAt is last_heartbeat_at; zero for NULL.
	LastHeartbeatAt time.Time
}

// StartRule decides the presence of an agent stored online or degraded when a process starts at
// start (D-M2-20-6). The stored heartbeat lags the real one by at most persistInterval
// (agent.heartbeat_persist_interval), so a stored heartbeat older than timeout
// (agent.presence_timeout) plus persistInterval, or none, means no heartbeat came within timeout:
// offlineNow. Otherwise the agent stays as stored until deadline, start plus timeout, which gives
// adapters time to reconnect after 4013.
func StartRule(stored StoredPresence, start time.Time, timeout, persistInterval time.Duration) (offlineNow bool, deadline time.Time) {
	if stored.LastHeartbeatAt.IsZero() || stored.LastHeartbeatAt.Add(timeout+persistInterval).Before(start) {
		return true, time.Time{}
	}
	return false, start.Add(timeout)
}

// Target is the configured mode and tool class of an agent (agents.billing_mode and tool_class).
type Target struct {
	BillingMode BillingMode
	ToolClass   ToolClass
}

// Assessment compares the configured and the reported values while the agent is present (10
// sections 7 and 8, D-M2-20-7); every flag is false while it is offline or unknown.
type Assessment struct {
	// ModeDiffers is kind (a): a reported mode unequal to the configured one.
	ModeDiffers bool
	// Unsupported is kind (b): no reported mode, reported supported modes without the configured
	// one.
	Unsupported bool
	// EvidenceUnknown is kind (c): billing_evidence.key_type unknown, which counts as unequal to
	// every configured mode (10 section 7).
	EvidenceUnknown bool
	// ToolClassDiffers: a reported tool class unequal to the configured one.
	ToolClassDiffers bool
}

// Differs reports a difference of any of the kinds (a), (b) and (c), the condition of the gauge
// bothy.agent.billing_mode.mismatch_age.
func (a Assessment) Differs() bool { return a.ModeDiffers || a.Unsupported || a.EvidenceUnknown }

// Assess compares target with the report r in state (D-M2-20-7).
func Assess(target Target, r Report, state RuntimeState) Assessment {
	if !state.Present() {
		return Assessment{}
	}
	return Assessment{
		ModeDiffers:      r.BillingMode != "" && r.BillingMode != target.BillingMode,
		Unsupported:      r.BillingMode == "" && r.BillingModesSupported != nil && !slices.Contains(r.BillingModesSupported, target.BillingMode),
		EvidenceUnknown:  r.Evidence != nil && r.Evidence.KeyType == KeyTypeUnknown,
		ToolClassDiffers: r.ToolClass != "" && r.ToolClass != target.ToolClass,
	}
}

// Warnings returns the warning codes of a at now in a fixed order. billing_mode_mismatch needs
// kind (a) to have held for longer than alertAfter (billing.mismatch_alert_after) since
// modeSince; the other warnings hold at once (D-M2-20-7).
func (a Assessment) Warnings(modeSince, now time.Time, alertAfter time.Duration) []string {
	var out []string
	if a.ModeDiffers && !modeSince.IsZero() && now.Sub(modeSince) > alertAfter {
		out = append(out, WarningBillingModeMismatch)
	}
	if a.Unsupported {
		out = append(out, WarningBillingModeUnsupported)
	}
	if a.EvidenceUnknown {
		out = append(out, WarningBillingEvidenceUnknown)
	}
	if a.ToolClassDiffers {
		out = append(out, WarningToolClassMismatch)
	}
	return out
}

// PresenceRow is the presence part of a row of agents: the configured values and the stored
// reported state.
type PresenceRow struct {
	// ID is the agent.
	ID ids.ID
	// Target is the configured mode and class.
	Target Target
	// Stored is the stored report: the reported_* columns with reported_details; State is
	// reported_runtime_state.
	Stored Report
	// LastHeartbeatAt is last_heartbeat_at; zero for NULL.
	LastHeartbeatAt time.Time
}
