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
	// DeliveryOf is the receive action of the event a webhook_out.deliver check delivers
	// (event_log.receive_action); empty for a personal event, whose receive action is "self" and
	// whose Audience decides. Used only by webhook_out.deliver (M2-26), which decides it on the
	// same resource.
	DeliveryOf Action
	// Audience is the audience principal of a personal event to deliver, zero otherwise; used
	// only by webhook_out.deliver (M2-26).
	Audience ids.ID
	// File describes a file resource (kind file) for the visibility of file.read and file.delete
	// (M3-03): required in request mode, where a missing value is an internal error, so no caller
	// widens access by leaving it out; nil only in filter and delivery mode, where the event scope
	// is the path (ResourceFromEvent).
	File *FileFacts
	// Event marks the descriptor of a stored event (ResourceFromEvent) and carries its full
	// scope. When the stored resource kind is not a resource of the receive action (pin.added
	// names a message, thread.read takes a thread), authz decides on the object of the scope the
	// action takes instead, and on the target of a grant for grant.read (M1-63).
	Event *EventScope
}

// FileFacts describes a file for the visibility rule of kind file (docs/spec/05-aktionen.md
// section 2.2, docs/DECISIONS.md D-M3-03-17): an attachment that is not attached yet is visible
// only to its uploader (Resource.OwnerID) with read on its scope channel (Resource.ChannelID); an
// attached one to every reader of the path of its message (Resource.ThreadID or ChannelID); the
// purposes avatar, branding and gdpr_export are not visible until their tickets (M3-10, M7-06)
// define their read rules.
//
// Example:
//
//	r := authorize.Resource{Kind: authorize.ResourceFile, ID: f.ID, ChannelID: f.ScopeChannelID, OwnerID: f.UploadedBy,
//		File: &authorize.FileFacts{Purpose: "attachment", Attached: false}}
type FileFacts struct {
	// Purpose is files.purpose: attachment, avatar, branding or gdpr_export.
	Purpose string
	// Attached reports that the file hangs on a message (set by M3-07).
	Attached bool
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

// EventScope is the scope of a stored event (event_log.space_id, channel_id, thread_id).
type EventScope struct {
	SpaceID, ChannelID, ThreadID ids.ID
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

// ResourceFromEvent returns the descriptor of a stored event for the check of its receive action
// at delivery (docs/spec/04-sync-protocol.md "Flow" point 2, 07 section 8.1): REST catch-up
// (M1-63) and the gateway hub (M1-65) build it from the columns resource_kind, resource_id,
// space_id, channel_id and thread_id of event_log. A thread, channel or space names itself in ID
// and carries no hint; any other kind carries only its most specific scope (the thread, else the
// channel, else the space). Reason: the scope of an event is a copy from the time of the event,
// and a thread moved since then would contradict it, which authz treats as an internal error;
// the most specific hint lets authz read the current path from the structure, so the rights at
// delivery decide (04 "Basic rule"). Event carries the whole scope, so authz can decide an
// event whose resource kind the receive action does not take on the object it does take.
//
// Example:
//
//	r := authorize.ResourceFromEvent(ev.ResourceKind, ev.ResourceID, ev.SpaceID, ev.ChannelID, ev.ThreadID)
//	d, err := az.Authorize(authorize.Filter(ctx), p, authorize.Action(ev.ReceiveAction), r)
func ResourceFromEvent(kind string, id, spaceID, channelID, threadID ids.ID) Resource {
	// An unknown kind stays empty, which Authorize refuses as an internal error (fail closed).
	k, _ := ParseResourceKind(kind)
	r := Resource{Kind: k, ID: id, Event: &EventScope{SpaceID: spaceID, ChannelID: channelID, ThreadID: threadID}}
	if k == ResourceThread || k == ResourceChannel || k == ResourceSpace {
		return r
	}
	switch {
	case !threadID.IsZero():
		r.ThreadID = threadID
	case !channelID.IsZero():
		r.ChannelID = channelID
	default:
		r.SpaceID = spaceID
	}
	return r
}
