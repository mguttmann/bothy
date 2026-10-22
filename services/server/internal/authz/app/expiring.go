// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// cursorExpiring is the list name of the cursor of "Expiring soon".
const cursorExpiring = "authz.expiring"

// expiringCursor is the position of a page of "Expiring soon": expiry, kind and id of its last
// entry.
type expiringCursor struct {
	At   time.Time `json:"t"`
	Kind string    `json:"k"`
	ID   string    `json:"id"`
}

// expiringEntry is one entry with its position.
type expiringEntry struct {
	pos  domain.ExpiringPos
	item authz.ExpiringItem
	// grant and group are the rows behind the entry, for the labels.
	grant *domain.Grant
	group ids.ID
	user  ids.ID
}

// ListExpiringAccess implements authz.Admin (access.hints.read, 07 section 9.4, M6-07): the
// entries in (now, now + authz.expiry_reminder_lead] read from the store and the token source,
// merged by expiry, kind and id, paged by that position. A space lists only grants on the space
// and its channels and the memberships of groups holding such a grant in effect; tokens only
// for the instance, which only owners read (R-AZ-34).
func (s *AdminService) ListExpiringAccess(ctx context.Context, caller actor.Actor, q authz.ExpiringQuery) (authz.ExpiringPage, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.ExpiringPage{}, err
	}
	eng, err := s.engine()
	if err != nil {
		return authz.ExpiringPage{}, err
	}
	if s.d.Tokens == nil || s.d.Config.ExpiryReminderLead <= 0 {
		return authz.ExpiringPage{}, internal("the expiring list needs the token source and authz.expiry_reminder_lead")
	}
	if err := s.checkHints(ctx, caller, q.SpaceID); err != nil {
		return authz.ExpiringPage{}, err
	}
	bind := binding(caller.ID.String(), q.SpaceID.String())
	after, err := s.expiringPosition(q.PageQuery, bind)
	if err != nil {
		return authz.ExpiringPage{}, err
	}
	t := s.d.Clock.Now()
	entries, err := s.expiringEntries(ctx, eng, q.SpaceID, t, t.Add(s.d.Config.ExpiryReminderLead))
	if err != nil {
		return authz.ExpiringPage{}, err
	}
	limit := s.pageLimit(q.PageQuery)
	var picked []expiringEntry
	for _, e := range entries {
		if after != nil && domain.ComparePos(e.pos, *after) <= 0 {
			continue
		}
		if picked = append(picked, e); len(picked) > limit {
			break
		}
	}
	full := len(picked) > limit
	picked = picked[:min(len(picked), limit)]
	out := authz.ExpiringPage{Items: make([]authz.ExpiringItem, 0, len(picked))}
	var refs labelRefs
	for _, e := range picked {
		out.Items = append(out.Items, e.item)
		if e.grant != nil {
			refs.addGrant(*e.grant)
		}
		refs.groups = append(refs.groups, e.group)
		refs.principals = append(refs.principals, e.user, e.item.PrincipalID)
	}
	if out.Labels, err = pageLabels(ctx, eng, s.d.Store, refs); err != nil {
		return authz.ExpiringPage{}, err
	}
	if full {
		last := picked[len(picked)-1].pos
		out.NextCursor, err = s.d.Cursors.Encode(cursorExpiring, bind, expiringCursor{At: last.ExpiresAt, Kind: string(last.Kind), ID: last.ID.String()})
		if err != nil {
			return authz.ExpiringPage{}, errs.Wrap(errs.CodeInternalError, err, "authz: expiring cursor")
		}
	}
	return out, nil
}

// expiringPosition decodes the cursor of q; nil for the start.
func (s *AdminService) expiringPosition(q authz.PageQuery, binding []byte) (*domain.ExpiringPos, error) {
	if q.Cursor == "" {
		return nil, nil
	}
	var c expiringCursor
	if err := s.d.Cursors.Decode(cursorExpiring, binding, q.Cursor, &c); err != nil {
		return nil, errs.New(errs.CodeCursorInvalid, "")
	}
	id, err := ids.Parse(c.ID)
	if err != nil || c.At.IsZero() {
		return nil, errs.New(errs.CodeCursorInvalid, "")
	}
	return &domain.ExpiringPos{ExpiresAt: c.At, Kind: domain.ExpiringKind(c.Kind), ID: id}, nil
}

// expiringEntries reads every entry of the window (t, until] in scope, sorted.
func (s *AdminService) expiringEntries(ctx context.Context, eng *Authorizer, space ids.ID, t, until time.Time) ([]expiringEntry, error) {
	var inSpace func(domain.Target) bool
	if !space.IsZero() {
		chans, err := eng.structure.Channels(ctx)
		if err != nil {
			return nil, sourceError("structure source", err)
		}
		own := map[ids.ID]bool{}
		for _, c := range chans {
			if c.SpaceID == space && c.Kind != authz.ChannelDM {
				own[c.ID] = true
			}
		}
		inSpace = func(tg domain.Target) bool {
			return (tg.Kind == domain.TargetSpace && tg.SpaceID == space) || (tg.Kind == domain.TargetChannel && own[tg.ChannelID])
		}
	}
	grants, err := s.d.Store.ExpiringGrants(ctx, t, until)
	if err != nil {
		return nil, err
	}
	var out []expiringEntry
	for _, g := range grants {
		if inSpace != nil && !inSpace(g.Target) {
			continue
		}
		row, port := g, grantOut(g)
		out = append(out, expiringEntry{pos: domain.ExpiringPos{ExpiresAt: g.ExpiresAt, Kind: domain.ExpiringGrant, ID: g.ID},
			item: authz.ExpiringItem{Kind: string(domain.ExpiringGrant), ExpiresAt: g.ExpiresAt, Grant: &port}, grant: &row})
	}
	members, err := s.expiringMembers(ctx, space, inSpace, t, until)
	if err != nil {
		return nil, err
	}
	out = append(out, members...)
	if space.IsZero() {
		tokens, err := s.d.Tokens.ExpiringTokens(ctx, t, until)
		if err != nil {
			return nil, sourceError("token source", err)
		}
		for _, tok := range tokens {
			if !domain.ExpiresWithin(tok.ExpiresAt, t, until) {
				continue
			}
			out = append(out, expiringEntry{pos: domain.ExpiringPos{ExpiresAt: tok.ExpiresAt, Kind: domain.ExpiringToken, ID: tok.ID},
				item: authz.ExpiringItem{Kind: string(domain.ExpiringToken), ExpiresAt: tok.ExpiresAt, TokenID: tok.ID, PrincipalID: tok.PrincipalID}})
		}
	}
	slices.SortFunc(out, func(a, b expiringEntry) int { return domain.ComparePos(a.pos, b.pos) })
	return out, nil
}

// expiringMembers returns the memberships of the window; for a space only those of groups that
// hold a grant in effect at t on a target in the space (07 section 2.6: the manager administers
// the access the group gives there).
func (s *AdminService) expiringMembers(ctx context.Context, space ids.ID, inSpace func(domain.Target) bool, t, until time.Time) ([]expiringEntry, error) {
	rows, err := s.d.Store.ExpiringMemberships(ctx, t, until)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	var allowed map[ids.ID]bool
	if !space.IsZero() {
		var groups []ids.ID
		for _, m := range rows {
			groups = append(groups, m.Group.ID)
		}
		grants, err := s.d.Store.ActiveGrantsOfGroups(ctx, uniqueIDs(groups))
		if err != nil {
			return nil, err
		}
		allowed = map[ids.ID]bool{}
		for _, g := range grants {
			if g.InEffect(t) && inSpace(g.Target) {
				allowed[g.Subject.GroupID] = true
			}
		}
	}
	var out []expiringEntry
	for _, m := range rows {
		if allowed != nil && !allowed[m.Group.ID] {
			continue
		}
		mb := m.Membership
		out = append(out, expiringEntry{pos: domain.ExpiringPos{ExpiresAt: mb.ExpiresAt, Kind: domain.ExpiringMembership, ID: mb.ID},
			item: authz.ExpiringItem{Kind: string(domain.ExpiringMembership), ExpiresAt: mb.ExpiresAt, MembershipID: mb.ID, GroupID: mb.GroupID,
				UserID: mb.UserID},
			group: mb.GroupID, user: mb.UserID})
	}
	return out, nil
}
