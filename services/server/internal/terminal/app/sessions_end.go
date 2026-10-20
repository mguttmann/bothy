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
	return s.finish(ctx, ses, endSpec{end: end, reported: reported})
}

// endSpec says how a session ends: the reason with detail and exit code (kept only when reported),
// and for an end by a person (closeTerminalSession, M2-44) the person and the mandatory reason.
type endSpec struct {
	end      domain.HostSessionEnd
	reported bool
	by       ids.ID
	reason   string
}

// finish is the one end path (M2-44 Design note 11): the live counts of the relay's pairing on
// this node are taken and added to the row first (bytes_in, bytes_out, last_input_at), then the row
// closes; an end by a person writes closed_by and the allow entry terminal.close with session_id,
// closed_by and the reason, any other end terminal.terminate as the system; then
// terminal_session.changed to the person, with the person as actor of a close by a person. The
// connections and the host service learn of the end from the event log tail (ObserveLogEvent of
// adapters/wsrelay).
func (s *Service) finish(ctx context.Context, ses domain.Session, sp endSpec) error {
	t := s.d.Clock.Now()
	if err := s.takeCounts(ctx, &ses); err != nil {
		return err
	}
	end := sp.end
	var err error
	switch {
	case !sp.by.IsZero():
		end.Detail, end.ExitCode = "", nil
		err = s.d.Store.CloseSessionByPerson(ctx, ses.ID, end.Reason, sp.by, t)
	case sp.reported:
		err = s.d.Store.CloseSessionReported(ctx, ses.ID, end, t)
	default:
		end.Detail, end.ExitCode = "", nil
		err = s.d.Store.CloseSession(ctx, ses.ID, end.Reason, t)
	}
	if err != nil {
		return err
	}
	if err := s.recordEnd(ctx, ses, end, sp, t); err != nil {
		return err
	}
	ses.State, ses.ClosedAt, ses.CloseReason, ses.CloseDetail, ses.ExitCode = domain.SessionClosed, t, end.Reason, end.Detail, end.ExitCode
	ses.DetachedAt, ses.ClosedBy = time.Time{}, sp.by
	ses.Version++
	return s.emitSessionChangedBy(ctx, ses, sp.by)
}

// takeCounts adds the live counts of the session's pairing on this node to the row and to ses.
func (s *Service) takeCounts(ctx context.Context, ses *domain.Session) error {
	c := s.relayControl().Counters(ses.ID)
	if c.In == 0 && c.Out == 0 && c.LastInput.IsZero() {
		return nil
	}
	if err := s.d.Store.AddCounters(ctx, ses.ID, c.In, c.Out, c.LastInput); err != nil {
		return err
	}
	ses.BytesIn += c.In
	ses.BytesOut += c.Out
	return nil
}

// recordEnd writes the audit entry of an end: terminal.close with the person of ctx for a close by
// a person, terminal.terminate as the system otherwise.
func (s *Service) recordEnd(ctx context.Context, ses domain.Session, end domain.HostSessionEnd, sp endSpec, t time.Time) error {
	if !sp.by.IsZero() {
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionTerminalClose), Decision: auditing.DecisionAllow, ResourceKind: resourceSession, ResourceID: ses.ID,
			Attrs: auditing.Attrs{attrSessionID: auditing.ID(ses.ID), attrClosedBy: auditing.ID(sp.by), attrCloseReason: auditing.Code(string(end.Reason)),
				attrReason: auditing.ReasonText(sp.reason)},
		})
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
	return s.d.Audit.Record(sys, auditing.Entry{
		Action: string(authorize.ActionTerminalTerminate), Decision: auditing.DecisionAllow, ResourceKind: resourceSession,
		ResourceID: ses.ID, Attrs: attrs,
	})
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
