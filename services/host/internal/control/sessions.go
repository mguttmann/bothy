// SPDX-License-Identifier: Apache-2.0

package control

import (
	"context"
)

// The close reason and detail RefuseSessions reports (24 section 10.3).
const (
	reasonOpenFailed = "open_failed"
	detailPTYFailed  = "pty_failed"
)

// Notifier sends the session reports of the host service on the control connection.
type Notifier interface {
	// Started sends session.started.
	Started(ctx context.Context, sessionID string) error
	// Ended sends session.ended.
	Ended(ctx context.Context, e SessionEnded) error
}

// Sessions is the session manager of the host service (M2-45): the sessions it knows, the
// session.open and session.close of the server, and the end of all PTYs.
type Sessions interface {
	// Live returns the sessions whose PTY runs or whose end was not reported yet (live_sessions of
	// identify).
	Live() []string
	// Active returns the number of running PTYs (heartbeat.active_sessions).
	Active() int
	// Open handles session.open; n sends the reports of the session.
	Open(ctx context.Context, o SessionOpen, n Notifier)
	// Close handles session.close.
	Close(ctx context.Context, c SessionClose)
	// EndAll ends every PTY (4110, 4111, 4112, 4114).
	EndAll(ctx context.Context)
}

// RefuseSessions is the session manager until M2-45 binds its own: it knows no session, answers
// every session.open with session.ended {open_failed, pty_failed} and ignores session.close, for
// there is no PTY.
type RefuseSessions struct{}

// Compile-time check: the session port.
var _ Sessions = RefuseSessions{}

// Live implements Sessions: none.
func (RefuseSessions) Live() []string { return []string{} }

// Active implements Sessions: none.
func (RefuseSessions) Active() int { return 0 }

// Open implements Sessions: the session ends at once with open_failed and pty_failed.
func (RefuseSessions) Open(ctx context.Context, o SessionOpen, n Notifier) {
	_ = n.Ended(ctx, SessionEnded{SessionID: o.SessionID, CloseReason: reasonOpenFailed, CloseDetail: detailPTYFailed})
}

// Close implements Sessions: nothing runs.
func (RefuseSessions) Close(context.Context, SessionClose) {
	// RefuseSessions never opens a session, so there is none to close.
}

// EndAll implements Sessions: nothing runs.
func (RefuseSessions) EndAll(context.Context) {
	// RefuseSessions never opens a session, so there is none to end.
}
