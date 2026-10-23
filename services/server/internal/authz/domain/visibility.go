// SPDX-License-Identifier: Apache-2.0

package domain

// Visibility per resource kind (07 section 4.3; docs/DECISIONS.md M1-28 D4 for the kinds outside
// the structure tree). Without visibility the answer is not_found (R-AZ-01).

// Resource kinds of the catalogue that the rules name.
const (
	kindInstance  = "instance"
	kindSpace     = "space"
	kindChannel   = "channel"
	kindAgentRes  = "agent"
	kindUserRes   = "user"
	kindPrincipal = "principal"
	kindHost      = "host"
	kindGroup     = "group"
	kindGrant     = "grant"
	kindDevice    = "device"
	// kindTerminalSession is a terminal session (M2-40): visible to its person and to effective
	// owners, whatever the requirement.
	kindTerminalSession = "terminal_session"
	// kindFile is a file (M3-03): its visibility depends on its purpose and attachment.
	kindFile = "file"
	// purposeAttachment is files.purpose of a file uploaded into a channel; purposeAvatar and
	// purposeBranding are the files of a profile picture and of the instance branding (M3-05).
	purposeAttachment = "attachment"
	// purposeAvatar and purposeBranding are files.purpose of an avatar and of the instance logo
	// (M3-10); purposeGDPRExport of a data export (M7).
	purposeAvatar     = "avatar"
	purposeBranding   = "branding"
	purposeGDPRExport = "gdpr_export"
	// actionFileRead is the one action that reads avatar and logo files.
	actionFileRead = "file.read"
)

// channelPathKinds are the kinds that live in a channel: a dm channel is visible to participants
// only, any other channel with at least read on the path.
//
//nolint:goconst // resource kinds of the generated catalogue, listed once per rule
var channelPathKinds = map[string]bool{
	kindChannel: true, "thread": true, "message": true, "reaction": true, "pin": true, "run": true,
	"run_step": true, "component": true, "interaction": true, "agent_session": true, kindFile: true,
	"incoming_webhook": true,
}

// ownedKinds belong to one principal: visible to it, and to an effective owner when the
// requirement names the owner.
//
//nolint:goconst // resource kinds of the generated catalogue, listed once per rule
var ownedKinds = map[string]bool{
	kindDevice: true, "auth_session": true, "api_token": true, "passkey": true, "push_registration": true,
	"draft": true, "gdpr_request": true, "dev_environment": true,
}

// ownerOnlyKinds are visible to effective owners only.
//
//nolint:goconst // resource kinds of the generated catalogue, listed once per rule
var ownerOnlyKinds = map[string]bool{KindApp: true, "webhook_delivery": true, "import_source": true}

// IsChannelPathKind reports whether kind lives in a channel (05 section 2.2).
func IsChannelPathKind(kind string) bool { return channelPathKinds[kind] }

// Visible applies the visibility table to the facts of in with its effective rights. reason is
// the reason of an invisible resource (no_grant, or not_self for principal-owned kinds and terminal
// sessions). The error reports a kind outside the table.
func Visible(in Input) (visible bool, reason Reason, err error) {
	if in.Target != nil && !in.Target.Visible {
		return false, ReasonNoGrant, nil
	}
	if in.Req.Contains(TargetHasCap(CapManage)) && in.Target == nil {
		return false, "", internalError("thread.move without its target channel")
	}
	kind := in.Resource.Kind
	switch {
	case kind == kindFile:
		return fileVisible(in), ReasonNoGrant, nil
	case channelPathKinds[kind]:
		if in.Path.DM {
			// The rights after "view as" decide too: no view opens a dm the viewed person is not in.
			return in.Path.Participant(in.Principal.ID) && in.Eff.Level >= LevelRead, ReasonNoGrant, nil
		}
		return in.Eff.Level >= LevelRead, ReasonNoGrant, nil
	case ownedKinds[kind]:
		return isSelf(in) || (in.Eff.Owner && in.Req.Contains(Owner)), ReasonNotSelf, nil
	case kind == KindApp && in.Resource.ID.IsZero():
		// The collection of apps (listApps, validateManifest) names no app, so it hides nothing:
		// its visibility is that of the instance, and the requirement owner answers a non-owner
		// with owner_only (06 section 4 point 2, M2-24 design note 13).
		return in.Principal.Kind != KindAnonymous, ReasonNoGrant, nil
	case ownerOnlyKinds[kind]:
		return in.Eff.Owner, ReasonNoGrant, nil
	case kind == kindCommand:
		return commandVisible(in)
	}
	rule, ok := kindRules[kind]
	if !ok {
		return false, "", internalError("no visibility rule for resource kind " + kind)
	}
	return rule(in)
}

// principalKinds are the resource kinds whose ID is a principal: naming the principal in ID makes
// the resource its own.
var principalKinds = map[string]bool{kindUserRes: true, kindPrincipal: true, kindAgentRes: true, KindApp: true}

// isSelf reports a resource of the principal itself: OwnerID is the principal, or ID is the
// principal for the kinds whose ID is a principal. The atom Self and the visibility rules use it
// alike.
func isSelf(in Input) bool {
	r, p := in.Resource, in.Principal.ID
	if p.IsZero() {
		return false
	}
	return r.OwnerID == p || (principalKinds[r.Kind] && r.ID == p)
}

// kindRules are the visibility rules of the remaining kinds.
var kindRules = map[string]func(in Input) (bool, Reason, error){
	kindInstance: func(in Input) (bool, Reason, error) {
		return in.Principal.Kind != KindAnonymous || in.Req.op == opPublic, ReasonNoGrant, nil
	},
	kindSpace: func(in Input) (bool, Reason, error) {
		return in.Eff.Level >= LevelRead || in.SpaceHasVisibleChannel, ReasonNoGrant, nil
	},
	kindGrant: grantVisible,
	kindAgentRes: func(in Input) (bool, Reason, error) {
		return in.Eff.Owner || (in.Principal.Kind == KindAgent && isSelf(in)) || in.ReadOnOperatedChannel, ReasonNoGrant, nil
	},
	kindUserRes:   personVisible,
	kindPrincipal: personVisible,
	kindGroup: func(in Input) (bool, Reason, error) {
		return in.Eff.Owner || in.GroupMember, ReasonNoGrant, nil
	},
	kindHost: hostVisible,
	kindTerminalSession: func(in Input) (bool, Reason, error) {
		return isSelf(in) || in.Eff.Owner, ReasonNotSelf, nil
	},
}

// hostVisible: an effective owner sees every host, its holder the own host, a person with the
// effective right terminal that host (07 section 2.12 point 5); with "view as" both views must see
// it. Agents and apps never reach a host action (the gate of HumanSession). Any principal sees the
// host for an action whose requirement is exactly Public: only host.pair, whose pairing code the
// use case proved before (M2-41, Design note 10).
func hostVisible(in Input) (bool, Reason, error) {
	if in.Host == nil {
		return false, "", internalError("host resource without its facts")
	}
	return in.Host.Visible || in.Req.op == opPublic, ReasonNoGrant, nil
}

// personVisible: people and apps see the directory; an agent sees only itself.
func personVisible(in Input) (bool, Reason, error) {
	return in.Principal.Kind == KindUser || in.Principal.Kind == KindApp || isSelf(in), ReasonNoGrant, nil
}

// grantVisible: owners see every grant; others see the grants on a space or channel where they
// hold manage, a channel inheriting it from its space through the effective rights on the path
// (07 section 2.6 item 6, brief M1-48 "Visibility"). Reading the target is not enough: a grant
// is administration data, and a grant outside the caller's manage scope answers like a random id.
func grantVisible(in Input) (bool, Reason, error) {
	switch in.GrantTarget {
	case TargetInstance, TargetHost:
		return in.Eff.Owner, ReasonNoGrant, nil
	case TargetSpace, TargetChannel:
		return in.Eff.Owner || in.Eff.Caps.Has(CapManage), ReasonNoGrant, nil
	}
	return false, "", internalError("grant resource without a target kind")
}

// channelReadable is the visibility of a resource in a channel: a participant of a dm channel,
// or at least read on the path of any other channel.
func channelReadable(in Input) bool {
	if in.Path.DM {
		return in.Path.Participant(in.Principal.ID) && in.Eff.Level >= LevelRead
	}
	return in.Eff.Level >= LevelRead
}

// fileVisible is the visibility of kind file (05 section 2.2, docs/DECISIONS.md D-M3-03-17, brief
// M3-10 design note 6): an attachment not attached yet only to its uploader with read on its scope
// channel, an attached one to every reader of the path of its message; an avatar or a logo to
// every principal that passed step 1 (active, signed in, not pending), but only for file.read, so
// no other file action reaches them; gdpr_export and any other purpose to nobody (its path is
// gdpr.export.download, M7). Without facts (an event decided in filter or delivery mode, where the
// application layer allows it) the path of the event decides, and without a path only the
// audience of the event (OwnerID, authorize.ResourceFromEvent) sees it; anything else fails closed.
func fileVisible(in Input) bool {
	f := in.Resource.File
	switch {
	case f == nil && in.Path.Kind == PathNone:
		return isSelf(in)
	case f == nil:
		return channelReadable(in)
	case f.Purpose == purposeAvatar || f.Purpose == purposeBranding:
		return in.Action == actionFileRead
	case f.Purpose != purposeAttachment:
		return false
	case f.Attached:
		return channelReadable(in)
	}
	return isSelf(in) && channelReadable(in)
}
