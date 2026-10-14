// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ExplainSpaceAccess implements rest.StrictServerInterface (GET /spaces/{space_id}/access,
// access.explain, M1-49).
func (h *Access) ExplainSpaceAccess(ctx context.Context, req rest.ExplainSpaceAccessRequestObject) (rest.ExplainSpaceAccessResponseObject, error) {
	x, err := h.admin.ExplainAccess(ctx, principal(ctx), authz.AccessTargetRef{Kind: "space", ID: ids.ID(req.SpaceId)})
	if err != nil {
		return nil, err
	}
	return rest.ExplainSpaceAccess200JSONResponse{Body: explanationREST(x)}, nil
}

// ExplainChannelAccess implements rest.StrictServerInterface (GET /channels/{channel_id}/access,
// access.explain, M1-49).
func (h *Access) ExplainChannelAccess(ctx context.Context, req rest.ExplainChannelAccessRequestObject) (rest.ExplainChannelAccessResponseObject, error) {
	x, err := h.admin.ExplainAccess(ctx, principal(ctx), authz.AccessTargetRef{Kind: "channel", ID: ids.ID(req.ChannelId)})
	if err != nil {
		return nil, err
	}
	return rest.ExplainChannelAccess200JSONResponse{Body: explanationREST(x)}, nil
}

// PreviewGrant implements rest.StrictServerInterface (POST /grants/preview, access.preview,
// M1-49).
func (h *Access) PreviewGrant(ctx context.Context, req rest.PreviewGrantRequestObject) (rest.PreviewGrantResponseObject, error) {
	p, err := h.admin.PreviewGrant(ctx, principal(ctx), grantCmd(req.Body))
	if err != nil {
		return nil, err
	}
	return rest.PreviewGrant200JSONResponse{Body: previewREST(p)}, nil
}

// ViewAsUser implements rest.StrictServerInterface (GET /users/{user_id}/view-as,
// access.view_as, M1-49).
func (h *Access) ViewAsUser(ctx context.Context, req rest.ViewAsUserRequestObject) (rest.ViewAsUserResponseObject, error) {
	v, err := h.admin.ViewAs(ctx, principal(ctx), ids.ID(req.UserId))
	if err != nil {
		return nil, err
	}
	return rest.ViewAsUser200JSONResponse{Body: viewAsREST(v)}, nil
}

// refREST converts a principal view (schema PrincipalRef).
func refREST(r authz.PrincipalRef) rest.PrincipalRef {
	return rest.PrincipalRef{Id: rest.Uuid(r.ID), Kind: rest.PrincipalKind(r.Kind), Handle: r.Handle, DisplayName: r.DisplayName,
		AvatarFileId: optionalID(r.AvatarFileID)}
}

// capsREST converts capability names, never nil.
func capsREST(list []string) []rest.Capability {
	out := make([]rest.Capability, 0, len(list))
	for _, c := range list {
		out = append(out, rest.Capability(c))
	}
	return out
}

// explanationREST converts an explanation (schema AccessExplanation); absent fields are left out
// (ADR 0053).
func explanationREST(x authz.AccessExplanation) rest.AccessExplanation {
	out := rest.AccessExplanation{TargetKind: rest.AccessTargetKind(x.TargetKind), TargetId: rest.Uuid(x.TargetID),
		Entries: make([]rest.AccessEntry, 0, len(x.Entries))}
	for _, e := range x.Entries {
		entry := rest.AccessEntry{Principal: refREST(e.Principal), Level: rest.LevelOrNone(e.Level), Capabilities: capsREST(e.Capabilities),
			Sources: make([]rest.AccessSource, 0, len(e.Sources))}
		if e.Inactive {
			inactive := true
			entry.Inactive = &inactive
		}
		for _, s := range e.Sources {
			entry.Sources = append(entry.Sources, sourceREST(s))
		}
		out.Entries = append(out.Entries, entry)
	}
	return out
}

// sourceREST converts one source (schema AccessSource).
func sourceREST(s authz.AccessSource) rest.AccessSource {
	out := rest.AccessSource{Kind: rest.AccessSourceKind(s.Kind), GrantId: optionalID(s.GrantID), GroupId: optionalID(s.GroupID),
		FromTargetId: optionalID(s.FromTargetID), AgentId: optionalID(s.AgentID), ExpiresAt: optionalTime(s.ExpiresAt),
		MembershipExpiresAt: optionalTime(s.MembershipExpiresAt), Reason: optional(s.Reason)}
	if s.FromTargetKind != "" {
		k := rest.TargetKind(s.FromTargetKind)
		out.FromTargetKind = &k
	}
	if s.Rule != "" {
		r := rest.AccessSourceRule(s.Rule)
		out.Rule = &r
	}
	return out
}

// previewREST converts a preview (schema GrantPreview).
func previewREST(p authz.GrantPreview) rest.GrantPreview {
	out := rest.GrantPreview{Allowed: p.Allowed, DenialReason: optional(p.DenialReason), Text: p.Text,
		TextLocale: rest.GrantPreviewTextLocale(p.TextLocale), Effects: make([]rest.AccessEffect, 0, len(p.Effects)),
		InvariantViolations: make([]rest.InvariantViolation, 0, len(p.InvariantViolations)), Hints: make([]rest.PreviewHint, 0, len(p.Hints))}
	for _, e := range p.Effects {
		out.Effects = append(out.Effects, rest.AccessEffect{Principal: refREST(e.Principal), TargetKind: rest.TargetKind(e.TargetKind),
			TargetId: optionalID(e.TargetID), BeforeLevel: rest.LevelOrNone(e.BeforeLevel), AfterLevel: rest.LevelOrNone(e.AfterLevel),
			CapabilitiesAdded: capsREST(e.CapabilitiesAdded)})
	}
	for _, v := range p.InvariantViolations {
		iv := rest.InvariantViolation{Code: v.Code, AgentId: optionalID(v.AgentID), HostId: optionalID(v.HostID),
			CredentialHolderId: optionalID(v.CredentialHolderID), Hint: optional(v.Hint), PrincipalIds: make([]rest.Uuid, 0, len(v.PrincipalIDs))}
		for _, id := range v.PrincipalIDs {
			iv.PrincipalIds = append(iv.PrincipalIds, rest.Uuid(id))
		}
		if !v.AgentID.IsZero() {
			ready := v.OrganizationReady
			iv.OrganizationReady = &ready
		}
		out.InvariantViolations = append(out.InvariantViolations, iv)
	}
	for _, h := range p.Hints {
		hint := rest.PreviewHint{Code: rest.PreviewHintCode(h.Code), AgentId: optionalID(h.AgentID), PrincipalId: optionalID(h.PrincipalID),
			GroupId: optionalID(h.GroupID), SpaceId: optionalID(h.SpaceID), ChannelId: optionalID(h.ChannelID)}
		if h.HasCount {
			n := h.Count
			hint.Count = &n
		}
		out.Hints = append(out.Hints, hint)
	}
	return out
}

// viewAsREST converts a view (schema ViewAsSnapshot).
func viewAsREST(v authz.ViewAsSnapshot) rest.ViewAsSnapshot {
	out := rest.ViewAsSnapshot{User: refREST(v.User), ExpiresAt: ms(v.ExpiresAt),
		Spaces: make([]rest.SpaceView, 0, len(v.Spaces)), Channels: make([]rest.ChannelView, 0, len(v.Channels))}
	for _, s := range v.Spaces {
		out.Spaces = append(out.Spaces, rest.SpaceView{Space: spaceREST(s.Space), Level: rest.Level(s.Level), Capabilities: capsREST(s.Capabilities)})
	}
	for _, c := range v.Channels {
		out.Channels = append(out.Channels, rest.ChannelView{Channel: channelREST(c.Channel), Level: rest.Level(c.Level), Capabilities: capsREST(c.Capabilities)})
	}
	return out
}

// spaceREST converts a space (schema Space).
func spaceREST(d authz.SpaceDetail) rest.Space {
	return rest.Space{Id: rest.Uuid(d.ID), Name: d.Name, Icon: optional(d.Icon), Color: optional(d.Color), SortKey: d.SortKey,
		DefaultOperatorAgentId: optionalID(d.DefaultOperatorID), ArchivedAt: optionalTime(d.ArchivedAt), Version: d.Version,
		CreatedAt: ms(d.CreatedAt)}
}

// channelREST converts a channel (schema Channel).
func channelREST(d authz.ChannelDetail) rest.Channel {
	out := rest.Channel{Id: rest.Uuid(d.ID), SpaceId: optionalID(d.SpaceID), Kind: rest.ChannelKind(d.Kind), Name: d.Name, Topic: d.Topic,
		SortKey: d.SortKey, Operator: rest.ChannelOperator{Mode: rest.OperatorMode(d.OperatorMode), AgentId: optionalID(d.OperatorAgentID),
			EffectiveAgentId: optionalID(d.EffectiveOperatorID)},
		ReadOnly: d.ReadOnly, MainThreadId: optionalID(d.MainThreadID), ArchivedAt: optionalTime(d.ArchivedAt), Version: d.Version,
		SettingsVersion: d.SettingsVersion, CreatedAt: ms(d.CreatedAt)}
	if d.ForumDefaultSort != "" {
		s := rest.ForumSort(d.ForumDefaultSort)
		out.ForumDefaultSort = &s
	}
	if d.DefaultNotificationLevel != "" {
		n := rest.NotificationLevel(d.DefaultNotificationLevel)
		out.DefaultNotificationLevel = &n
	}
	if len(d.DMParticipantIDs) > 0 {
		list := make([]rest.Uuid, 0, len(d.DMParticipantIDs))
		for _, id := range d.DMParticipantIDs {
			list = append(list, rest.Uuid(id))
		}
		out.DmParticipantIds = &list
	}
	return out
}
