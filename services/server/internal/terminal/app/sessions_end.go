// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// SessionSelector names the sessions EndSessions ends: those of HostID, of UserID or, with a zero
// UserID, of every person.
type SessionSelector struct {
	HostID, UserID ids.ID
}

// EndSessions ends the open sessions of sel with reason as the server, inside the caller's unit of
// work (24 section 8.3: the end of an assignment and every transition out of active end the
// sessions in the same transaction that raises permission_version). It locks the rows that are not
// closed in ascending id (03 section 6.3, after the host and the session user rows), closes each
// (closed_at, close_reason, detached_at cleared, no detail, no closer, version raised) and writes
// one audit entry terminal.terminate per session as the system (05 rule 1.10): session_id,
// close_reason, bytes_in, bytes_out and duration_ms, never content. It returns the number of ended
// sessions. Errors: kind internal outside a unit of work; the errors of the store and the recorder.
//
// Extension point: every later end goes through this function. M2-44 adds the event
// terminal_session.changed, the close 4100 of the device stream and session.close to the host
// service (after the commit); M2-46 adds the metric bothy.terminal.sessions.closed and its
// subscribers. Until a session can be opened (M2-44) the rows exist only as test fixtures.
func (s *Service) EndSessions(ctx context.Context, sel SessionSelector, reason domain.CloseReason) (int, error) {
	if _, ok := txn.UnitFrom(ctx); !ok {
		return 0, errs.New(errs.CodeInternalError, "terminal: EndSessions runs only inside a unit of work")
	}
	open, err := s.d.Store.LockOpenSessions(ctx, sel.HostID, sel.UserID)
	if err != nil {
		return 0, err
	}
	t := s.d.Clock.Now()
	// The server ends the session: the audit entry has no actor (system, 05 rule 1.10), the
	// correlation of the request stays.
	sys := actor.WithActor(ctx, actor.Anonymous())
	for _, ses := range open {
		if err := s.d.Store.CloseSession(ctx, ses.ID, reason, t); err != nil {
			return 0, err
		}
		duration := max(t.Sub(ses.OpenedAt).Milliseconds(), 0)
		if err := s.d.Audit.Record(sys, auditing.Entry{
			Action: string(authorize.ActionTerminalTerminate), Decision: auditing.DecisionAllow, ResourceKind: resourceSession,
			ResourceID: ses.ID, Attrs: auditing.Attrs{
				attrSessionID: auditing.ID(ses.ID), attrCloseReason: auditing.Code(string(reason)),
				attrBytesIn: auditing.Int(ses.BytesIn), attrBytesOut: auditing.Int(ses.BytesOut), attrDurationMS: auditing.Int(duration),
			},
		}); err != nil {
			return 0, err
		}
	}
	return len(open), nil
}
