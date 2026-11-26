// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The incoming port of computer sessions (CU-04, docs/spec/27-computer-use.md 12.2): the five REST
// operations. The attach of bothy-computer-mcp, the host ops and the revocations are no port of
// other modules; they run inside the module (adapters/wsrelay, app.Subscribers) or through the
// ports runs calls (app.ComputerRunEnds).

// OpenComputer is the body of openComputerSession (schema ComputerSessionOpen).
type OpenComputer struct {
	HostID     ids.ID
	CEK        string
	PinChanged bool
}

// Desktop is one entry of listComputerDesktops (schema ComputerDesktop).
type Desktop struct {
	HostID         ids.ID
	Name, Platform string
	Online, Locked bool
	Busy           bool
}

// ComputerSessionView is a computer session as the API shows it (schema ComputerSession): metadata
// only, never an image, the token or its jti; zero times, zero ids and empty codes are absent
// members.
type ComputerSessionView struct {
	ID, HostID, UserID, AgentID, RunID ids.ID
	Desktop                            string
	State, PauseReason                 string
	Generation, KeyGeneration          int64
	AttachedAt, OpenedAt, StartedAt    time.Time
	PausedAt, ClosedAt                 time.Time
	PausedMS                           int64
	StoppedBy                          ids.ID
	CloseReason, CloseDetail           string
	ActionCount                        int64
	FrontApp                           string
	Version                            int64
}

// ComputerGrant is the answer of openComputerSession (schema ComputerSessionGrant): the single-use
// attach token is a redacting SessionToken, revealed only by the handler of the answer.
type ComputerGrant struct {
	SessionID      ids.ID
	Token          SessionToken
	AttachPath     string
	NoisePublicKey []byte
	KeyFingerprint string
	KeyGeneration  int64
	ExpiresAt      time.Time
}

// Computers is the incoming port of the five operations (27 12.2), implemented by
// app.Service.Computers. Every method authorizes first; errors are kernel/errs errors.
type Computers interface {
	// Desktops lists the desktops of the bound run's owner (computer.desktops.list). Errors:
	// forbidden with computer_not_owner_run or computer_tool_class, invocation_token_invalid.
	Desktops(ctx context.Context, a actor.Actor) ([]Desktop, error)
	// Open opens a computer session of the bound run on a desktop (computer.open). Errors:
	// not_found, forbidden (computer_not_owner_run, computer_tool_class, computer_not_enabled),
	// invocation_token_invalid, host_unavailable, conflict (computer_busy, computer_run_busy),
	// unavailable without a signing key.
	Open(ctx context.Context, a actor.Actor, cmd OpenComputer) (ComputerGrant, error)
	// List lists the caller's own computer sessions, newest first, optionally in one state
	// (computer.session.read). Errors: forbidden, validation_failed for an unknown state.
	List(ctx context.Context, a actor.Actor, state string) ([]ComputerSessionView, error)
	// Stop ends a computer session with stopped_by_user and client_stop (computer.stop); a closed
	// session answers unchanged. Errors: not_found, forbidden.
	Stop(ctx context.Context, a actor.Actor, sessionID ids.ID) (ComputerSessionView, error)
	// DisableDevice switches the enablement of a host off (computer.device.disable) and returns
	// the host. Errors: not_found, forbidden.
	DisableDevice(ctx context.Context, a actor.Actor, hostID ids.ID) (HostView, error)
}
