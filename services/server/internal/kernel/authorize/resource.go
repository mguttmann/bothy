// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Resource describes the resource of one Authorize call. The calling use case loads the resource
// first (02 section 9.2) and fills what it knows; authz resolves the path through the structure
// itself and treats SpaceID, ChannelID and ThreadID only as hints that must agree with it (a
// contradiction is an internal error).
//
// Example (reading a message):
//
//	r := authorize.Resource{Kind: authorize.ResourceMessage, ID: msg.ID, ThreadID: msg.ThreadID, OwnerID: msg.AuthorID}
//	d, err := az.Authorize(ctx, p, authorize.ActionMessageRead, r)
type Resource struct {
	// Kind is the resource kind of the catalogue.
	Kind ResourceKind
	// ID is the resource; zero for the instance.
	ID ids.ID
	// SpaceID, ChannelID and ThreadID are path hints. A resource below a channel (message,
	// reaction, run and the like) names its thread or channel; a channel or thread may name itself
	// in ID instead.
	SpaceID   ids.ID
	ChannelID ids.ID
	ThreadID  ids.ID
	// OwnerID is the principal the resource belongs to: author of a message or reaction, creator
	// of a thread, holder of a device, session, token, draft or setting, starter of a run, the
	// agent or app itself, the principal itself for user and principal resources.
	OwnerID ids.ID
	// TargetChannelID is the destination channel of thread.move.
	TargetChannelID ids.ID
	// HasAttachments marks a message.create with attachments (needs upload).
	HasAttachments bool
	// RestrictedTags marks a thread.tags.set that touches restricted tags (needs manage).
	RestrictedTags bool
	// Grant describes the new grant of grant.create and the existing grant of grant.revoke.
	Grant *GrantFacts
	// Invoke describes the trigger of agent.invoke.
	Invoke *InvokeFacts
	// PreviewOf is the action that access.preview previews.
	PreviewOf Action
	// HostID is the path hint of a terminal_session: its host (terminal_sessions.host_id, M2-40).
	// authz resolves the host through it; a terminal_session without it is an internal error.
	HostID ids.ID
	// Terminal describes the session of terminal.attach for the quick reattach (M2-40); required
	// for terminal.attach, ignored otherwise.
	Terminal *TerminalFacts
}

// TerminalFacts describes a terminal session for the step-up rule of terminal.attach (08 section
// 6.3 point 5): its state, the device and login session that opened it and the time it was
// detached (zero when it is not detached).
//
// Example:
//
//	r := authorize.Resource{Kind: authorize.ResourceTerminalSession, ID: s.ID, HostID: s.HostID, OwnerID: s.UserID,
//		Terminal: &authorize.TerminalFacts{SessionState: "detached", SessionDeviceID: s.DeviceID,
//			SessionAuthSessionID: s.AuthSessionID, DetachedAt: s.DetachedAt}}
type TerminalFacts struct {
	// SessionState is terminal_sessions.state.
	SessionState string
	// SessionDeviceID and SessionAuthSessionID are the device and the login session of the session.
	SessionDeviceID, SessionAuthSessionID ids.ID
	// DetachedAt is terminal_sessions.detached_at; zero for none.
	DetachedAt time.Time
}

// GrantFacts is the shape of a grant for the delegation rules (07 section 2.6). The strings are
// the column values of grants (subject_kind, level, capabilities, target_kind).
type GrantFacts struct {
	// SubjectKind is user, group, agent or app.
	SubjectKind string
	// Level is read, write or trigger; empty on a host.
	Level string
	// Caps are the capabilities.
	Caps []string
	// TargetKind is instance, space, channel or host.
	TargetKind string
	// TargetSpaceID, TargetChannelID and TargetHostID name the target of the kind.
	TargetSpaceID, TargetChannelID, TargetHostID ids.ID
}

// InvokeFacts describes one trigger of agent.invoke (05 section 5).
type InvokeFacts struct {
	// AgentID is the agent to trigger.
	AgentID ids.ID
	// TriggerOrigin is messages.origin of the trigger: human, agent, app, import or system.
	TriggerOrigin string
	// MentionsAgent reports whether the trigger mentions the agent.
	MentionsAgent bool
	// ActiveRunAgentID is the agent of the active run of the thread; zero for none (set by runs
	// from M2).
	ActiveRunAgentID ids.ID
}
