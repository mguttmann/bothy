// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// closeSession implements Sessions.Close (Design note 17 c of M2-44): the mandatory reason (at most
// audit.reason_max_chars, else 400 validation_failed too_long at /reason), terminal.close, then in
// one unit of work the locked row ends through the one end path, client_closed for an own session
// and closed_by_owner for another person's, with closed_by and the entry terminal.close; a closed
// session answers unchanged with an entry no_change. The connections and the host service learn of
// the end from the event log tail.
func (s *Service) closeSession(ctx context.Context, p actor.Actor, cmd terminal.CloseSession) (terminal.SessionView, error) {
	ctx, err := withCaller(ctx, p)
	if err != nil {
		return terminal.SessionView{}, err
	}
	reason, err := domain.CheckReason(cmd.Reason, s.cfg.ReasonMaxChars)
	if err != nil {
		return terminal.SessionView{}, err
	}
	ses, found, err := s.d.Store.Session(ctx, cmd.SessionID)
	if err != nil {
		return terminal.SessionView{}, err
	}
	if err := s.check(ctx, p, authorize.ActionTerminalClose, sessionResource(cmd.SessionID, ses, found, false)); err != nil {
		return terminal.SessionView{}, err
	}
	var out domain.Session
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Store.LockSession(ctx, cmd.SessionID)
		if err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		if cur.State == domain.SessionClosed {
			out = cur
			return s.d.Audit.Record(ctx, auditing.Entry{
				Action: string(authorize.ActionTerminalClose), Decision: auditing.DecisionAllow, ResourceKind: resourceSession, ResourceID: cur.ID,
				Attrs: auditing.Attrs{attrSessionID: auditing.ID(cur.ID), attrReason: auditing.ReasonText(reason), attrNoChange: auditing.Bool(true)},
			})
		}
		end := domain.CloseClientClosed
		if cur.UserID != p.ID {
			end = domain.CloseClosedByOwner
		}
		if err := s.finish(ctx, cur, domain.End{Reason: end, ClosedBy: p.ID, Source: domain.EndByCloseRequest}, reason); err != nil {
			return err
		}
		out, _, err = s.d.Store.LockSession(ctx, cur.ID)
		return err
	})
	if err != nil {
		return terminal.SessionView{}, err
	}
	return sessionView(out), nil
}
