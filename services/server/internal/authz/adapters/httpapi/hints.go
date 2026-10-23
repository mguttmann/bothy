// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ListPermissionHints implements rest.StrictServerInterface (GET /access/hints,
// access.hints.read, M6-07).
func (h *Access) ListPermissionHints(ctx context.Context, req rest.ListPermissionHintsRequestObject) (rest.ListPermissionHintsResponseObject, error) {
	prm := req.Params
	q := authz.HintsQuery{PageQuery: pageOf(prm.Cursor, prm.Limit)}
	if prm.SpaceId != nil {
		q.SpaceID = ids.ID(*prm.SpaceId)
	}
	if prm.Kind != nil {
		q.Kind = string(*prm.Kind)
	}
	p, err := h.admin.ListPermissionHints(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.PermissionHintPage{Items: make([]rest.PermissionHint, 0, len(p.Items)), Labels: labelsREST(p.Labels),
		NextCursor: optional(p.NextCursor)}
	for _, x := range p.Items {
		out.Items = append(out.Items, hintREST(x))
	}
	return rest.ListPermissionHints200JSONResponse{Body: out}, nil
}

// ListExpiringAccess implements rest.StrictServerInterface (GET /access/expiring,
// access.hints.read, M6-07).
func (h *Access) ListExpiringAccess(ctx context.Context, req rest.ListExpiringAccessRequestObject) (rest.ListExpiringAccessResponseObject, error) {
	prm := req.Params
	q := authz.ExpiringQuery{PageQuery: pageOf(prm.Cursor, prm.Limit)}
	if prm.SpaceId != nil {
		q.SpaceID = ids.ID(*prm.SpaceId)
	}
	p, err := h.admin.ListExpiringAccess(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.ExpiringAccessPage{Items: make([]rest.ExpiringAccessItem, 0, len(p.Items)), Labels: labelsREST(p.Labels),
		NextCursor: optional(p.NextCursor)}
	for _, x := range p.Items {
		out.Items = append(out.Items, expiringREST(x))
	}
	return rest.ListExpiringAccess200JSONResponse{Body: out}, nil
}

// hintREST converts one hint (schema PermissionHint); absent fields are left out (ADR 0053).
func hintREST(x authz.PermissionHint) rest.PermissionHint {
	out := rest.PermissionHint{Kind: rest.PermissionHintKind(x.Kind), AgentId: optionalID(x.AgentID), PersonId: optionalID(x.PersonID),
		SpaceId: optionalID(x.SpaceID), ChannelId: optionalID(x.ChannelID), CredentialHolderId: optionalID(x.CredentialHolderID)}
	if x.Grant != nil {
		g := grantREST(*x.Grant)
		out.Grant = &g
	}
	if len(x.Rules) > 0 {
		rules := make([]rest.PermissionHintRules, 0, len(x.Rules))
		for _, r := range x.Rules {
			rules = append(rules, rest.PermissionHintRules(r))
		}
		out.Rules = &rules
	}
	if x.TriggerSet != nil {
		set := make([]rest.Uuid, 0, len(x.TriggerSet))
		for _, id := range x.TriggerSet {
			set = append(set, rest.Uuid(id))
		}
		out.TriggerSet = &set
	}
	return out
}

// expiringREST converts one entry of "Expiring soon" (schema ExpiringAccessItem).
func expiringREST(x authz.ExpiringItem) rest.ExpiringAccessItem {
	out := rest.ExpiringAccessItem{Kind: rest.ExpiringAccessItemKind(x.Kind), ExpiresAt: x.ExpiresAt, MembershipId: optionalID(x.MembershipID),
		GroupId: optionalID(x.GroupID), UserId: optionalID(x.UserID), TokenId: optionalID(x.TokenID), PrincipalId: optionalID(x.PrincipalID)}
	if x.Grant != nil {
		g := grantREST(*x.Grant)
		out.Grant = &g
	}
	return out
}

// labelsREST converts the names of a page (schema AccessLabels); every list is present, also
// when empty.
func labelsREST(l authz.AccessLabels) rest.AccessLabels {
	out := rest.AccessLabels{Principals: make([]rest.PrincipalRef, 0, len(l.Principals)), Groups: make([]rest.AccessLabel, 0, len(l.Groups)),
		Spaces: make([]rest.AccessLabel, 0, len(l.Spaces)), Channels: make([]rest.ChannelLabel, 0, len(l.Channels))}
	for _, p := range l.Principals {
		out.Principals = append(out.Principals, refREST(p))
	}
	for _, g := range l.Groups {
		out.Groups = append(out.Groups, rest.AccessLabel{Id: rest.Uuid(g.ID), Name: g.Name})
	}
	for _, s := range l.Spaces {
		out.Spaces = append(out.Spaces, rest.AccessLabel{Id: rest.Uuid(s.ID), Name: s.Name})
	}
	for _, c := range l.Channels {
		out.Channels = append(out.Channels, rest.ChannelLabel{Id: rest.Uuid(c.ID), SpaceId: rest.Uuid(c.SpaceID), Name: c.Name})
	}
	return out
}
