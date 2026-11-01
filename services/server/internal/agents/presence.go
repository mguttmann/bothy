// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The presence states of an agent (schema AgentRuntimeState, 04 section 12 item 2): unknown
// before the first heartbeat, online or degraded from the latest one, offline once
// agent.presence_timeout has passed without a heartbeat. People have no presence (R-PR-02).
const (
	StateUnknown  = "unknown"
	StateOnline   = "online"
	StateDegraded = "degraded"
	StateOffline  = "offline"
)

// The warning codes of AgentRuntimeStatus.warnings (10 section 8, 09 section 2.4,
// docs/DECISIONS.md D-M2-20-7). They are evidence for display, never a permission.
const (
	// WarningBillingModeMismatch: the reported mode has differed from the configured one for
	// longer than billing.mismatch_alert_after.
	WarningBillingModeMismatch = "billing_mode_mismatch"
	// WarningBillingModeUnsupported: no reported mode, and the configured one is not among the
	// reported supported modes.
	WarningBillingModeUnsupported = "billing_mode_unsupported"
	// WarningBillingEvidenceUnknown: the runtime reports key_type unknown.
	WarningBillingEvidenceUnknown = "billing_evidence_unknown"
	// WarningToolClassMismatch: the reported tool class differs from the configured one.
	WarningToolClassMismatch = "tool_class_mismatch"
)

// BillingEvidence is billing_evidence.v1 as reported (schema BillingEvidence): how the runtime
// proves its mode. It never holds a key value or prefix.
type BillingEvidence struct {
	// Provider and Method are codes; "" when not reported.
	Provider, Method string
	// KeyType is api_key, oauth, cloud, none or unknown.
	KeyType string
}

// PresenceStatus is the presence and reported state of one agent (schema AgentRuntimeStatus,
// and AgentPresence for the snapshot). Empty strings, nil slices and the zero time mean "not
// reported".
type PresenceStatus struct {
	// AgentID is the agent.
	AgentID ids.ID
	// State is one of the State constants.
	State string
	// ReportedBillingMode is the mode of the runtime configuration; "" when the adapter follows
	// the configured mode per run. It stays while offline (the last reported mode).
	ReportedBillingMode string
	// RuntimeVersion is the reported version of the runtime.
	RuntimeVersion string
	// LastHeartbeatAt is the time of the last heartbeat this process saw or stored.
	LastHeartbeatAt time.Time
	// BillingModesSupported are the modes the adapter can prove.
	BillingModesSupported []string
	// ReportedToolClass is the class the runtime loaded.
	ReportedToolClass string
	// BillingModeMatches is false only when a reported mode exists and differs from the
	// configured one (M1-51).
	BillingModeMatches bool
	// BillingEvidence is the reported evidence; nil when not reported.
	BillingEvidence *BillingEvidence
	// Warnings are the Warning codes that hold now; only while online or degraded.
	Warnings []string
	// ProjectTrash are the folder moves the runner owes (HeartbeatAck.project_trash, 09 18.15,
	// D-DEL-11); set only in the answer to a heartbeat, nil otherwise.
	ProjectTrash []ProjectTrashRequest
}

// ProjectTrashRequest is a folder move the serving runner owes (schema ProjectTrashRequest,
// 09 18.15); agents/app copies it from chat.ProjectTrashRequest.
type ProjectTrashRequest struct {
	ChannelID    ids.ID
	WorkspaceRef string
}

// Presence is the incoming port of the heartbeat and of the presence of agents (M2-20,
// docs/spec/09-agents.md section 4.8). One process-wide tracker in the roles api and all
// implements it; the presence lives in memory and is stored at most once per
// agent.heartbeat_persist_interval. The reported mode is evidence for display, warning, audit and
// alarm and never a permission.
type Presence interface {
	// Heartbeat takes one heartbeat of the agent in ctx (kernel/actor): body is the JSON object
	// of schema Heartbeat, the REST body of sendHeartbeat or the member agent of the gateway op
	// heartbeat, so both transports share one decoder. It authorizes agent.heartbeat on the
	// agent itself, validates the body, applies project readiness, stores and audits a change of
	// the reported mode or tool class at once and publishes agent.presence_changed when state or
	// reported mode change. It returns the presence after the heartbeat.
	//
	// Errors: forbidden (subject_kind_not_allowed for people and apps), validation_failed with
	// every field error, internal_error for a database or readiness failure (the presence is then
	// unchanged).
	//
	// Example:
	//
	//	st, err := presence.Heartbeat(ctx, []byte(`{"state":"online","reported_billing_mode":"organization"}`))
	Heartbeat(ctx context.Context, body json.RawMessage) (PresenceStatus, error)
	// Statuses returns the presence of agentIDs in that order: tracked agents from memory, the
	// others from their stored row; unknown ids are left out. It does not authorize: callers pass
	// only agents the caller may read. Errors: a database error.
	Statuses(ctx context.Context, agentIDs []ids.ID) ([]PresenceStatus, error)
	// ProjectFolders returns the folders the runner agentID reported under its projects root
	// (heartbeat member project_folders, 09 section 18.13) without the names a project binding
	// names, compared without case (listAgentProjectFolders, M4-44). It authorizes
	// agent.project_folders.read for a (owner).
	//
	// Errors: forbidden or not_found from authorize, not_found for an unknown agent,
	// internal_error for a database or chat failure.
	//
	// Example:
	//
	//	f, err := presence.ProjectFolders(ctx, owner, runnerID)
	//	// f.Names == []string{"docs-site", "website"}, f.Reported == true
	ProjectFolders(ctx context.Context, a actor.Actor, agentID ids.ID) (ProjectFolders, error)
}

// ProjectFolders is the answer of Presence.ProjectFolders (schema ProjectFolderList).
type ProjectFolders struct {
	// Names are the reported folder names without a binding, in the reported order; never nil.
	Names []string
	// Reported is false while the runner never sent project_folders.
	Reported bool
}
