// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
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

// SessionSelector names the sessions EndSessions ends: those of HostID, of UserID or, with a zero
// UserID, of every person, except those of ExceptUserID (zero: nobody excepted; M2-40, the end of
// foreign sessions under I6).
type SessionSelector struct {
	HostID, UserID ids.ID
	ExceptUserID   ids.ID
}

// EndSessions ends the open sessions of sel with reason as the server, inside the caller's unit of
// work (24 section 8.3: the end of an assignment and every transition out of active end the
// sessions in the same transaction that raises permission_version). It locks the rows that are not
// closed in ascending id (03 section 6.3, after the host and the session user rows) and ends each
// through the one end path of the module (endSession): closed_at, close_reason, detached_at
// cleared, no detail, no closer, version raised, one audit entry terminal.terminate as the system
// and terminal_session.changed to the person. It returns the number of ended sessions. Errors: kind
// internal outside a unit of work; the errors of the store, the recorder and the appender.
//
// Extension point: M2-44 adds the close 4100 of the device stream and session.close to the host
// service through its observer of terminal_session.changed (D-W-105); M2-46 adds the metric
// bothy.terminal.sessions.closed.
func (s *Service) EndSessions(ctx context.Context, sel SessionSelector, reason domain.CloseReason) (int, error) {
	ended, err := s.endSessions(ctx, sel, reason)
	return len(ended), err
}

// endSessions is EndSessions returning the ids of the ended sessions in ascending id.
func (s *Service) endSessions(ctx context.Context, sel SessionSelector, reason domain.CloseReason) ([]ids.ID, error) {
	if _, ok := txn.UnitFrom(ctx); !ok {
		return nil, errs.New(errs.CodeInternalError, "terminal: EndSessions runs only inside a unit of work")
	}
	open, err := s.d.Store.LockOpenSessions(ctx, sel.HostID, sel.UserID)
	if err != nil {
		return nil, err
	}
	var ended []ids.ID
	for _, ses := range open {
		if !sel.ExceptUserID.IsZero() && ses.UserID == sel.ExceptUserID {
			continue
		}
		if err := s.endSession(ctx, ses, domain.HostSessionEnd{Reason: reason}, false); err != nil {
			return nil, err
		}
		ended = append(ended, ses.ID)
	}
	return ended, nil
}

// endSession is the one end path of a session the caller locked (Design note 10 of M2-43): it
// closes the row as the server (reported false: no detail, no exit code) or as the host service
// reported it (reported true: end.Detail and end.ExitCode), records terminal.terminate as the
// system (05 rule 1.10) with session_id, close_reason, close_detail, bytes_in, bytes_out and
// duration_ms, never content, and appends terminal_session.changed to the person.
func (s *Service) endSession(ctx context.Context, ses domain.Session, end domain.HostSessionEnd, reported bool) error {
	t := s.d.Clock.Now()
	var err error
	if reported {
		err = s.d.Store.CloseSessionReported(ctx, ses.ID, end, t)
	} else {
		end.Detail, end.ExitCode = "", nil
		err = s.d.Store.CloseSession(ctx, ses.ID, end.Reason, t)
	}
	if err != nil {
		return err
	}
	// The server ends the session: the audit entry has no actor (system, 05 rule 1.10), the
	// correlation of the request stays.
	sys := actor.WithActor(ctx, actor.Anonymous())
	attrs := auditing.Attrs{
		attrSessionID: auditing.ID(ses.ID), attrCloseReason: auditing.Code(string(end.Reason)),
		attrBytesIn: auditing.Int(ses.BytesIn), attrBytesOut: auditing.Int(ses.BytesOut),
		attrDurationMS: auditing.Int(max(t.Sub(ses.OpenedAt).Milliseconds(), 0)),
	}
	if end.Detail != "" {
		attrs[attrCloseDetail] = auditing.Code(string(end.Detail))
	}
	if err := s.d.Audit.Record(sys, auditing.Entry{
		Action: string(authorize.ActionTerminalTerminate), Decision: auditing.DecisionAllow, ResourceKind: resourceSession,
		ResourceID: ses.ID, Attrs: attrs,
	}); err != nil {
		return err
	}
	ses.State, ses.ClosedAt, ses.CloseReason, ses.CloseDetail, ses.ExitCode = domain.SessionClosed, t, end.Reason, end.Detail, end.ExitCode
	ses.DetachedAt = time.Time{}
	ses.Version++
	return s.emitSessionChanged(ctx, ses)
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
	payload, err := json.Marshal(sessionJSON{
		ID: ses.ID.String(), HostID: ses.HostID.String(), UserID: ses.UserID.String(), DeviceID: ses.DeviceID.String(),
		Profile: ses.Profile, State: ses.State, OpenedAt: stamp(ses.OpenedAt), StartedAt: stamp(ses.StartedAt),
		DetachedAt: stamp(ses.DetachedAt), ClosedAt: stamp(ses.ClosedAt), CloseReason: string(ses.CloseReason),
		CloseDetail: string(ses.CloseDetail), ExitCode: ses.ExitCode, BytesIn: ses.BytesIn, BytesOut: ses.BytesOut, Version: ses.Version,
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "terminal: terminal_session.changed payload")
	}
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: eventSessionChanged, SchemaVersion: 1, ResourceKind: resourceSession, ResourceID: ses.ID, ResourceVersion: ses.Version,
		AudiencePrincipalID: ses.UserID, ReceiveAction: receiveSelf, Payload: payload,
	})
}
