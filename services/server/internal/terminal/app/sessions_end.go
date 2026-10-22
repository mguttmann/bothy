// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"net/netip"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// SessionSelector names the sessions EndSessions ends (M2-39, M2-40, M2-46): the open sessions that
// match every member that is set. SessionID names one session; DeviceID the sessions whose current
// device it is; AuthSessionIDs the sessions of these login sessions; HostID the sessions of a host;
// UserID the sessions of a person; ExceptUserID excepts that person's sessions (M2-40, the end of
// foreign sessions under I6). Zero ids and an empty list are not used; a selector without any of
// SessionID, DeviceID, AuthSessionIDs, HostID and UserID is an internal error, never "every
// session".
type SessionSelector struct {
	HostID, UserID ids.ID
	ExceptUserID   ids.ID
	SessionID      ids.ID
	DeviceID       ids.ID
	AuthSessionIDs []ids.ID
}

// matches reports whether ses matches every member of sel that is set.
func (sel SessionSelector) matches(ses domain.Session) bool {
	switch {
	case !sel.SessionID.IsZero() && ses.ID != sel.SessionID,
		!sel.DeviceID.IsZero() && ses.DeviceID != sel.DeviceID,
		len(sel.AuthSessionIDs) > 0 && !slices.Contains(sel.AuthSessionIDs, ses.AuthSessionID),
		!sel.HostID.IsZero() && ses.HostID != sel.HostID,
		!sel.UserID.IsZero() && ses.UserID != sel.UserID,
		!sel.ExceptUserID.IsZero() && ses.UserID == sel.ExceptUserID:
		return false
	}
	return true
}

// EndSessions ends the open sessions of sel with end, inside the caller's unit of work: the one end
// function of the module (M2-39 Design note 7, completed by M2-46 Design note 1). Every kind of
// revocation of 24 section 8.3 calls it in the transaction that causes it. It locks the selected
// rows that are not closed in ascending id (03 section 6.3) and ends each through finish: the pair
// of reason and detail checked (domain.ValidateEnd), closed_at, close_reason, close_detail, exit
// code and closer only where the source allows them, the counts not yet written added once,
// detached_at cleared, version raised, one audit entry (terminal.terminate as the system, or
// terminal.close for closeTerminalSession), terminal_session.changed to the person, and
// bothy.terminal.sessions.closed after the commit. A session that is already closed is skipped:
// the first committed end wins. It returns the number of ended sessions. Errors: kind internal
// outside a unit of work, for an empty selector and for a pair outside the closed lists; the
// errors of the store, the recorder and the appender.
//
// The connections learn of the end after the commit from the event log tail of every api process
// (adapters/wsrelay ObserveLogEvent: 4100 with the reason, session.close to the host service).
//
// Example:
//
//	n, err := s.EndSessions(ctx, SessionSelector{HostID: h.ID, UserID: u}, domain.ServerEnd(domain.CloseAccessRevoked))
func (s *Service) EndSessions(ctx context.Context, sel SessionSelector, end domain.End) (int, error) {
	ended, err := s.endSessions(ctx, sel, end)
	return len(ended), err
}

// endSessions is EndSessions returning the ids of the ended sessions in ascending id.
func (s *Service) endSessions(ctx context.Context, sel SessionSelector, end domain.End) ([]ids.ID, error) {
	if _, ok := txn.UnitFrom(ctx); !ok {
		return nil, errs.New(errs.CodeInternalError, "terminal: EndSessions runs only inside a unit of work")
	}
	open, err := s.lockSelected(ctx, sel)
	if err != nil {
		return nil, err
	}
	var ended []ids.ID
	for _, ses := range open {
		if ses.State == domain.SessionClosed || !sel.matches(ses) {
			continue
		}
		if err := s.finish(ctx, ses, end, ""); err != nil {
			return nil, err
		}
		ended = append(ended, ses.ID)
	}
	return ended, nil
}

// lockSelected locks the open sessions the most specific member of sel names, in ascending id.
func (s *Service) lockSelected(ctx context.Context, sel SessionSelector) ([]domain.Session, error) {
	switch {
	case !sel.SessionID.IsZero():
		ses, found, err := s.d.Store.LockSession(ctx, sel.SessionID)
		if err != nil || !found {
			return nil, err
		}
		return []domain.Session{ses}, nil
	case !sel.DeviceID.IsZero():
		return s.d.Store.LockOpenSessionsOfDevice(ctx, sel.DeviceID)
	case len(sel.AuthSessionIDs) > 0:
		return s.d.Store.LockOpenSessionsOfAuthSessions(ctx, sel.AuthSessionIDs)
	case !sel.HostID.IsZero():
		return s.d.Store.LockOpenSessions(ctx, sel.HostID, sel.UserID)
	case !sel.UserID.IsZero():
		return s.d.Store.LockOpenSessionsOfUsers(ctx, []ids.ID{sel.UserID})
	}
	return nil, errs.New(errs.CodeInternalError, "terminal: EndSessions without a selector")
}

// EndSession ends the sessions of sel with end in its own unit of work as the system (M2-46): the
// relay ends a session at a deadline through it, with source server and its counts. It locks the
// host of a named session before the session (03 section 6.3). Errors: those of EndSessions and
// of the unit of work.
func (s *Service) EndSession(ctx context.Context, sel SessionSelector, end domain.End) error {
	if !sel.SessionID.IsZero() {
		return s.lockForRelay(ctx, sel.SessionID, func(ctx context.Context, ses domain.Session) error {
			if ses.State == domain.SessionClosed || !sel.matches(ses) {
				return nil
			}
			return s.finish(ctx, ses, end, "")
		})
	}
	return s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		_, err := s.endSessions(ctx, sel, end)
		return err
	})
}

// endSession ends the session ses the caller locked with end (the paths of M2-43 and M2-44 that
// already hold the row).
func (s *Service) endSession(ctx context.Context, ses domain.Session, end domain.End) error {
	return s.finish(ctx, ses, end, "")
}

// finish is the one end path (M2-46 Design note 1): the end is normalized and its pair checked,
// the counts the relay of this node has not written yet (RelayControl.Counters) join the counts of
// the end, the one closing statement writes the row, an end by a person (source close_request)
// writes the allow entry terminal.close with session_id, closed_by, close_reason and the reason
// text, any other end terminal.terminate as the system, then terminal_session.changed to the
// person, and after the commit bothy.terminal.sessions.closed with the reason. The connections and
// the host service learn of the end from the event log tail.
func (s *Service) finish(ctx context.Context, ses domain.Session, end domain.End, reasonText string) error {
	end = end.Normalized()
	if err := domain.ValidateEnd(end.Reason, end.Detail); err != nil {
		return err
	}
	if end.Source == domain.EndByCloseRequest && end.ClosedBy.IsZero() {
		return errs.New(errs.CodeInternalError, "terminal: a close by a person without the person")
	}
	t := s.d.Clock.Now()
	end.Progress = s.withLiveCounts(ses.ID, end.Progress)
	if err := s.d.Store.EndSession(ctx, ses.ID, end, t); err != nil {
		return err
	}
	if p := end.Progress; p != nil {
		ses.BytesIn += max(p.BytesIn, 0)
		ses.BytesOut += max(p.BytesOut, 0)
	}
	if err := s.recordEnd(ctx, ses, end, reasonText, t); err != nil {
		return err
	}
	ses.State, ses.ClosedAt, ses.CloseReason, ses.CloseDetail, ses.ExitCode = domain.SessionClosed, t, end.Reason, end.Detail, end.ExitCode
	ses.DetachedAt, ses.ClosedBy = time.Time{}, end.ClosedBy
	ses.Version++
	if err := s.emitSessionChangedBy(ctx, ses, end.ClosedBy); err != nil {
		return err
	}
	reason, m := end.Reason, s.sessionMetrics()
	txn.AfterCommit(ctx, func() { m.SessionClosed(context.WithoutCancel(ctx), reason) })
	return nil
}

// withLiveCounts adds the live counts of the session's pairing on this node to p (nil for none);
// it returns nil when there is nothing to add.
func (s *Service) withLiveCounts(id ids.ID, p *domain.Progress) *domain.Progress {
	c := s.relayControl().Counters(id)
	if c.In == 0 && c.Out == 0 && c.LastInput.IsZero() {
		return p
	}
	out := domain.Progress{BytesIn: c.In, BytesOut: c.Out, LastInputAt: c.LastInput}
	if p != nil {
		out.BytesIn += p.BytesIn
		out.BytesOut += p.BytesOut
		if p.LastInputAt.After(out.LastInputAt) {
			out.LastInputAt = p.LastInputAt
		}
	}
	return &out
}

// recordEnd writes the audit entry of an end (24 section 10.1): terminal.close with the person of
// ctx for a close by a person, terminal.terminate as the system otherwise, with exactly
// session_id, close_reason, close_detail (only when set), bytes_in, bytes_out and duration_ms
// (closed_at minus opened_at); never content.
func (s *Service) recordEnd(ctx context.Context, ses domain.Session, end domain.End, reasonText string, t time.Time) error {
	if end.Source == domain.EndByCloseRequest {
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionTerminalClose), Decision: auditing.DecisionAllow, ResourceKind: resourceSession, ResourceID: ses.ID,
			Attrs: auditing.Attrs{attrSessionID: auditing.ID(ses.ID), attrClosedBy: auditing.ID(end.ClosedBy), attrCloseReason: auditing.Code(string(end.Reason)),
				attrReason: auditing.ReasonText(reasonText)},
		})
	}
	// The server ends the session: the audit entry is a system action (05 rule 1.10) without an
	// actor and without the client IP of a request that caused it, so its details hold exactly the
	// keys of 24 section 10.1; the correlation of the request stays.
	sys := systemAudit{actor.WithActor(ctx, actor.Anonymous())}
	attrs := auditing.Attrs{
		attrSessionID: auditing.ID(ses.ID), attrCloseReason: auditing.Code(string(end.Reason)),
		attrBytesIn: auditing.Int(ses.BytesIn), attrBytesOut: auditing.Int(ses.BytesOut),
		attrDurationMS: auditing.Int(max(t.Sub(ses.OpenedAt).Milliseconds(), 0)),
	}
	if end.Detail != "" {
		attrs[attrCloseDetail] = auditing.Code(string(end.Detail))
	}
	return s.d.Audit.Record(sys, auditing.Entry{
		Action: string(authorize.ActionTerminalTerminate), Decision: auditing.DecisionAllow, ResourceKind: resourceSession,
		ResourceID: ses.ID, Attrs: attrs,
	})
}

// systemAudit is ctx without the client IP of the request (kernel/reqmeta), for the audit entry of
// a system action: the recorder then finds no IP to pseudonymise (ip_hmac, ip_key_id), as authz
// does for folded expiries. The key of reqmeta is unexported, so the value is hidden by its type;
// no other netip.Addr travels in a context. Every other value, the unit of work and the
// correlation id included, stays.
type systemAudit struct{ context.Context }

// Value implements context.Context.
func (c systemAudit) Value(key any) any {
	v := c.Context.Value(key)
	if _, ip := v.(netip.Addr); ip {
		return nil
	}
	return v
}

// sessionJSON is the payload of terminal_session.changed (schema TerminalSession): metadata only,
// optional members absent (ADR 0053).
type sessionJSON struct {
	ID          string `json:"id"`
	HostID      string `json:"host_id"`
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
	Profile     string `json:"profile"`
	State       string `json:"state"`
	OpenedAt    string `json:"opened_at"`
	StartedAt   string `json:"started_at,omitempty"`
	DetachedAt  string `json:"detached_at,omitempty"`
	ClosedAt    string `json:"closed_at,omitempty"`
	CloseReason string `json:"close_reason,omitempty"`
	CloseDetail string `json:"close_detail,omitempty"`
	ExitCode    *int   `json:"exit_code,omitempty"`
	BytesIn     int64  `json:"bytes_in"`
	BytesOut    int64  `json:"bytes_out"`
	Version     int64  `json:"version"`
}

// emitSessionChanged appends terminal_session.changed for ses (05 section 6, 24 section 10.2):
// durable, personal to the person of the session (receive action self), resource terminal_session
// with its version, no actor (the system), payload TerminalSession; never content.
func (s *Service) emitSessionChanged(ctx context.Context, ses domain.Session) error {
	return s.emitSessionChangedBy(ctx, ses, ids.ID{})
}

// emitSessionChangedBy is emitSessionChanged with the person by as actor (open, attach and close
// by a person, M2-44); zero by means no actor.
func (s *Service) emitSessionChangedBy(ctx context.Context, ses domain.Session, by ids.ID) error {
	payload, err := json.Marshal(sessionJSON{
		ID: ses.ID.String(), HostID: ses.HostID.String(), UserID: ses.UserID.String(), DeviceID: ses.DeviceID.String(),
		Profile: ses.Profile, State: ses.State, OpenedAt: stamp(ses.OpenedAt), StartedAt: stamp(ses.StartedAt),
		DetachedAt: stamp(ses.DetachedAt), ClosedAt: stamp(ses.ClosedAt), CloseReason: string(ses.CloseReason),
		CloseDetail: string(ses.CloseDetail), ExitCode: ses.ExitCode, BytesIn: ses.BytesIn, BytesOut: ses.BytesOut, Version: ses.Version,
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "terminal: terminal_session.changed payload")
	}
	ev := eventlog.Event{
		Type: eventSessionChanged, SchemaVersion: 1, ResourceKind: resourceSession, ResourceID: ses.ID, ResourceVersion: ses.Version,
		AudiencePrincipalID: ses.UserID, ReceiveAction: receiveSelf, Payload: payload,
	}
	if !by.IsZero() {
		ev.ActorKind, ev.ActorPrincipalID = eventlog.ActorUser, by
	}
	return s.d.Events.Append(ctx, ev)
}
