// SPDX-License-Identifier: Apache-2.0

package domain

import "encoding/json"

// SessionMode is session.mode of 09 9.1, the member session_mode of AgentInvocationPayload, plus
// SessionGuest for a run without a session, which the payload never shows.
type SessionMode string

// The session modes (09 9.1; M2-12 design note 9).
const (
	// SessionGuest is the run of an agent that does not operate the thread: it has no session
	// (09 3.6, R-AG-04); the payload carries neither session_mode nor agent_session.
	SessionGuest SessionMode = "guest"
	// SessionNew creates the external session, with the reference of the row when there is one.
	SessionNew SessionMode = "new"
	// SessionResume continues a started session that a run already used.
	SessionResume SessionMode = "resume"
	// SessionBind takes over a started session no run used yet (rebinding, import binding).
	SessionBind SessionMode = "bind"
)

// SessionRow is what the mode needs of the operator's current session row: its representation
// (the schema AgentSession, encoded by agents.SessionJSON), whether the runtime started it, and
// whether some run already used it (a run with runs.agent_session_id = the row, which runs owns).
type SessionRow struct {
	JSON    json.RawMessage
	Started bool
	Used    bool
}

// SessionDecision is the session part of an invocation: the mode and the row the payload carries
// as agent_session (nil for none).
type SessionDecision struct {
	Mode SessionMode
	Row  json.RawMessage
}

// DecideSession returns the session of a new run (09 9.1 mode table; M2-12 design note 9):
// a run of an agent that is not the thread's operator is a guest run without session; for the
// operator no row gives new without agent_session, an unstarted row new with the row (the adapter
// reuses its reference, REUSE L14), a started row some run used resume, a started row no run used
// bind.
//
// Example:
//
//	d := domain.DecideSession(true, &domain.SessionRow{JSON: raw, Started: true}) // bind
func DecideSession(operator bool, row *SessionRow) SessionDecision {
	switch {
	case !operator:
		return SessionDecision{Mode: SessionGuest}
	case row == nil:
		return SessionDecision{Mode: SessionNew}
	case !row.Started:
		return SessionDecision{Mode: SessionNew, Row: row.JSON}
	case row.Used:
		return SessionDecision{Mode: SessionResume, Row: row.JSON}
	}
	return SessionDecision{Mode: SessionBind, Row: row.JSON}
}

// check refuses a decision the payload cannot carry: no mode (the decision is required), a guest
// run with a row, resume or bind without a row, or an unknown mode.
func (d SessionDecision) check() error {
	switch d.Mode {
	case SessionGuest:
		if d.Row != nil {
			return errPayload
		}
	case SessionNew:
	case SessionResume, SessionBind:
		if len(d.Row) == 0 {
			return errPayload
		}
	default:
		return errPayload
	}
	return nil
}
