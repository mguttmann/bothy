// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// reasonKindNotUser is the reason of a sidebar call by an agent or app: the sidebar is a row of a
// person (03 user_space_states).
const reasonKindNotUser = "subject_kind_not_allowed"

// sidebarResource describes the sidebar row of the caller for space: owned by the caller, so the
// self rule of sidebar.update holds, and visible with read on the space (05 section 4.5).
func sidebarResource(space, caller ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceSpace, ID: space, OwnerID: caller}
}

// PutSidebarSpace implements chat.Structure in one unit of work (04 section 14): only people
// (agents and apps get 403 subject_kind_not_allowed before any lookup), sidebar.update on the
// space (an invisible space is 404), the key rule of domain.SidebarKey, then the upsert with
// version + 1 and the caller's device, and sidebar.updated for the caller. There is no audit
// (05: Audit nein) and no If-Match: the last write wins.
func (s *Service) PutSidebarSpace(ctx context.Context, a actor.Actor, spaceID ids.ID, cmd chat.SidebarPutCmd) (chat.SidebarState, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.SidebarState{}, err
	}
	if a.Kind == actor.KindAgent || a.Kind == actor.KindApp {
		return chat.SidebarState{}, errs.New(errs.CodeForbidden, "").WithReason(reasonKindNotUser)
	}
	var out domain.SidebarState
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		_, found, err := s.d.Store.Space(ctx, spaceID)
		if err != nil {
			return err
		}
		if err := s.check(ctx, a, authorize.ActionSidebarUpdate, sidebarResource(spaceID, a.ID)); err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		key, err := domain.SidebarKey("", cmd.SortKey, cmd.UseGlobalOrder)
		if err != nil {
			return err
		}
		keep := cmd.SortKey == nil && !cmd.UseGlobalOrder
		out, err = s.d.Store.PutSidebarState(ctx, domain.SidebarState{
			UserID: a.ID, SpaceID: spaceID, Collapsed: cmd.Collapsed, SortKey: key, UpdatedByDeviceID: a.DeviceID,
		}, keep)
		if err != nil {
			return err
		}
		return s.emitSidebar(ctx, out)
	})
	return sidebarView(out), err
}

// ListSidebarStates implements chat.Structure: the rows of the person a for spaces that are not
// archived and in the visible spaces of Scope (07 section 7.4 rule 1), each kept only when a may
// read the space (authorize in filter mode). Agents and apps have no rows.
func (s *Service) ListSidebarStates(ctx context.Context, a actor.Actor) ([]chat.SidebarState, error) {
	if a.Kind != actor.KindUser || a.ID.IsZero() {
		return []chat.SidebarState{}, nil
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionSpaceRead)
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Store.SidebarStates(ctx, a.ID, sc.InstanceWide, sc.SpaceIDs)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	out := make([]chat.SidebarState, 0, len(rows))
	for _, st := range rows {
		d, err := s.d.Authorizer.Authorize(fctx, a, authorize.ActionSpaceRead, spaceResource(st.SpaceID))
		if err != nil {
			return nil, err
		}
		if d.Allowed() {
			out = append(out, sidebarView(st))
		}
	}
	return out, nil
}
