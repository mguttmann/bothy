// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// OpenSession is the body of openTerminalSession (schema TerminalSessionOpen) with its host:
// WorkdirIndex nil when absent; CEK is client_ephemeral_key_hash as sent.
type OpenSession struct {
	HostID       ids.ID
	Profile      string
	WorkdirIndex *int
	Cols, Rows   int
	CEK          string
	PinChanged   bool
}

// AttachSession is the body of attachTerminalSession (schema TerminalSessionAttach) with its
// session.
type AttachSession struct {
	SessionID  ids.ID
	Cols, Rows int
	CEK        string
	PinChanged bool
}

// ListSessions asks for one page of listTerminalSessions: the filters host_id (zero: every host)
// and state ("": every state), the cursor and the page size (0 for api.page_size_default).
type ListSessions struct {
	HostID ids.ID
	State  string
	Cursor string
	Limit  int
}

// CloseSession is the body of closeTerminalSession (schema ReasonBody) with its session.
type CloseSession struct {
	SessionID ids.ID
	Reason    string
}

// SessionView is a terminal session as the API shows it (schema TerminalSession): metadata only,
// never content; zero times, empty codes and a nil exit code are absent members.
type SessionView struct {
	ID, HostID, UserID, DeviceID    ids.ID
	Profile, State                  string
	OpenedAt                        time.Time
	StartedAt, DetachedAt, ClosedAt time.Time
	CloseReason, CloseDetail        string
	ExitCode                        *int
	BytesIn, BytesOut, Version      int64
}

// SessionPage is one page of sessions in descending id; NextCursor is "" on the last page.
type SessionPage struct {
	Items      []SessionView
	NextCursor string
}

// Grant is the answer of openTerminalSession and attachTerminalSession (schema
// TerminalSessionGrant): everything the device needs for the Noise handshake.
type Grant struct {
	Session        SessionView
	Token          SessionToken
	TokenExpiresAt time.Time
	StreamPath     string
	// NoisePublicKey, KeyFingerprint and KeyGeneration are the keys of the host as getHost
	// renders them.
	NoisePublicKey  []byte
	KeyFingerprint  string
	KeyGeneration   int64
	NoiseProtocol   string
	PrologueVersion string
}

// SessionToken is a session token of the terminal (docs/spec/24-terminal.md section 5.3). It
// renders as "[redacted]" through fmt, slog, encoding/json and encoding/text, so it never reaches
// a log, an audit entry, an event or an error by accident (SEC-LOG-2); only Reveal hands out the
// compact JWS, which the HTTP adapter calls once when it writes session_token.
type SessionToken struct {
	v *string
}

// NewSessionToken wraps the compact JWS s.
func NewSessionToken(s string) SessionToken { return SessionToken{v: &s} }

// Reveal returns the compact JWS; "" for the zero token.
func (t SessionToken) Reveal() string {
	if t.v == nil {
		return ""
	}
	return *t.v
}

// String returns "[redacted]".
func (SessionToken) String() string { return redacted }

// GoString returns "[redacted]".
func (SessionToken) GoString() string { return redacted }

// Format writes "[redacted]" for every verb.
func (SessionToken) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, redacted) }

// LogValue implements slog.LogValuer.
func (SessionToken) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns the JSON string "[redacted]".
func (SessionToken) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText returns "[redacted]".
func (SessionToken) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// Sessions is the incoming port of the terminal sessions (M2-44, docs/spec/24-terminal.md section
// 6, 05 section 4.14). Every method authorizes the caller p before it reads anything else of the
// session; agents, apps and personal tokens get 403 forbidden with reason
// subject_kind_not_allowed for any id; an unknown and an invisible session give the same 404
// not_found. Errors are kernel/errs errors; a session token never appears in one.
type Sessions interface {
	// Open opens a session on the host (terminal.open with a fresh step-up, SR10 to SR12, the host
	// active and connected, the profile known): it consumes the step-up, writes the session in
	// opening, the audit entry and terminal_session.changed in one unit of work and returns the
	// grant with a one-time session token; after the commit it asks the host service for its data
	// connection. Errors: validation_failed, not_found, forbidden, step_up_required,
	// terminal_denied (with reason), host_unavailable, unavailable without a signing key.
	Open(ctx context.Context, p actor.Actor, cmd OpenSession) (Grant, error)
	// Attach issues a new session token for an own session that is not closed (terminal.attach:
	// a fresh step-up unless quick reattach); an older device's stream closes with 4103. Errors:
	// as Open, plus invalid_state_transition for a closed session.
	Attach(ctx context.Context, p actor.Actor, cmd AttachSession) (Grant, error)
	// List returns one page of the sessions p may read (terminal.read_own: the own, owners all).
	// Errors: cursor_invalid, validation_failed for an unknown state.
	List(ctx context.Context, p actor.Actor, q ListSessions) (SessionPage, error)
	// Get returns the session id (terminal.read_own).
	Get(ctx context.Context, p actor.Actor, id ids.ID) (SessionView, error)
	// Close ends an own session (client_closed) or, for an owner, any session (closed_by_owner)
	// with a mandatory reason (terminal.close); a closed session answers unchanged (no_change).
	// Errors: validation_failed on /reason.
	Close(ctx context.Context, p actor.Actor, cmd CloseSession) (SessionView, error)
}
