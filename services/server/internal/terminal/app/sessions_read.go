// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// sessionsList is the list name of the cursors of listTerminalSessions.
const sessionsList = "terminal_sessions"

// sessionsCursor is the keyset position of listTerminalSessions: the last id of the page.
type sessionsCursor struct {
	ID string `json:"id"`
}

// sessionStates are the values of TerminalSessionState.
var sessionStates = []string{domain.SessionOpening, domain.SessionActive, domain.SessionDetached, domain.SessionClosed}

// getSession implements Sessions.Get (Design note 17 a of M2-44): terminal.read_own; unknown and
// invisible give the same 404 (AZ-P4).
func (s *Service) getSession(ctx context.Context, p actor.Actor, id ids.ID) (terminal.SessionView, error) {
	ctx, err := withCaller(ctx, p)
	if err != nil {
		return terminal.SessionView{}, err
	}
	ses, found, err := s.d.Store.Session(ctx, id)
	if err != nil {
		return terminal.SessionView{}, err
	}
	if err := s.check(ctx, p, authorize.ActionTerminalReadOwn, sessionResource(id, ses, found, false)); err != nil {
		return terminal.SessionView{}, err
	}
	if !found {
		return terminal.SessionView{}, invisible()
	}
	return sessionView(ses), nil
}

// listSessions implements Sessions.List (Design note 17 b of M2-44): terminal.read_own on the host of
// the filter or the instance; an active owner sees every session, anyone else only the own; each
// row passes authorize in filter mode; descending id with a keyset cursor (ruling D-M2-44-10).
func (s *Service) listSessions(ctx context.Context, p actor.Actor, q terminal.ListSessions) (terminal.SessionPage, error) {
	ctx, err := withCaller(ctx, p)
	if err != nil {
		return terminal.SessionPage{}, err
	}
	if err := s.checkList(ctx, p, q.HostID); err != nil {
		return terminal.SessionPage{}, err
	}
	if q.State != "" && !slices.Contains(sessionStates, q.State) {
		return terminal.SessionPage{}, domain.FieldError("#query/state", "invalid_enum", "unknown state")
	}
	binding := []byte(p.ID.String() + "\x00" + q.HostID.String() + "\x00" + q.State)
	var before ids.ID
	if q.Cursor != "" {
		var k sessionsCursor
		if err := s.d.Cursors.Decode(sessionsList, binding, q.Cursor, &k); err != nil {
			return terminal.SessionPage{}, err
		}
		if before, err = ids.Parse(k.ID); err != nil || before.IsZero() {
			return terminal.SessionPage{}, errs.New(errs.CodeCursorInvalid, "")
		}
	}
	owner, err := s.d.People.IsActiveOwner(ctx, p.ID)
	if err != nil {
		return terminal.SessionPage{}, err
	}
	user := p.ID
	if owner {
		user = ids.ID{}
	}
	limit := q.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	return s.page(ctx, p, user, q, before, limit, binding)
}

// checkList is the collection check of listTerminalSessions (ruling D-M2-44-10): with host_id
// terminal.read_own on that host (a host the caller cannot see is 404); without it the catalogue
// has no instance resource for the action (M2-40: terminal_session and host), so the check runs on
// a descriptor of an own session no host holds, in filter mode: the gate HumanSession and the
// liveness of the principal answer as for any id (agents, apps and personal tokens 403
// subject_kind_not_allowed, a revoked session 401), and the not_found of the unknown host only
// says that the rows decide, each through its own check.
func (s *Service) checkList(ctx context.Context, p actor.Actor, hostID ids.ID) error {
	if !hostID.IsZero() {
		return s.check(ctx, p, authorize.ActionTerminalReadOwn, hostResource(hostID))
	}
	probe := authorize.Resource{Kind: authorize.ResourceTerminalSession, ID: ids.New(), HostID: ids.New(), OwnerID: p.ID,
		Terminal: &authorize.TerminalFacts{}}
	d, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), p, authorize.ActionTerminalReadOwn, probe)
	if err != nil {
		return err
	}
	if d.Allowed() || d.Outcome == authorize.OutcomeNotFound {
		return nil
	}
	return d.Err()
}

// page reads rows until limit visible ones and one more are found or the rows end.
func (s *Service) page(ctx context.Context, p actor.Actor, user ids.ID, q terminal.ListSessions, before ids.ID, limit int, binding []byte) (terminal.SessionPage, error) {
	out := terminal.SessionPage{Items: []terminal.SessionView{}}
	var last ids.ID
	for {
		rows, err := s.d.Store.ListSessions(ctx, user, q.HostID, q.State, before, limit+1)
		if err != nil {
			return terminal.SessionPage{}, err
		}
		for _, ses := range rows {
			before = ses.ID
			d, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), p, authorize.ActionTerminalReadOwn, sessionResource(ses.ID, ses, true, false))
			if err != nil {
				return terminal.SessionPage{}, err
			}
			if !d.Allowed() {
				continue
			}
			if len(out.Items) == limit {
				c, err := s.d.Cursors.Encode(sessionsList, binding, sessionsCursor{ID: last.String()})
				if err != nil {
					return terminal.SessionPage{}, errs.Wrap(errs.CodeInternalError, err, "terminal: list cursor")
				}
				out.NextCursor = c
				return out, nil
			}
			out.Items = append(out.Items, sessionView(ses))
			last = ses.ID
		}
		if len(rows) < limit+1 {
			return out, nil
		}
	}
}
