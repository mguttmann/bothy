// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The outgoing ports of computer sessions (CU-04, docs/spec/27-computer-use.md, D-CU-01-15).

// ComputerRow is a computer session with the name of its host, as the list reads it.
type ComputerRow struct {
	domain.ComputerSession
	Desktop string
}

// ComputerStateWrite is what computer.state writes: the state with its pause, the paused time to
// add (the end of a pause), the action count and the frontmost app ("" for NULL).
type ComputerStateWrite struct {
	State, PauseReason string
	PausedAt           time.Time
	AddPausedMS        int64
	ActionCount        int64
	FrontApp           string
}

// ComputerEndWrite is what the one closing statement writes besides the end: the paused time to
// add for a pause in progress, the paused time and the action count the host reported (the higher
// value wins).
type ComputerEndWrite struct {
	AddPausedMS, ReportedPausedMS int64
	ActionCount                   int64
}

// ComputerStore is the outgoing port of the computer sessions over computer_sessions and the
// enablement columns of hosts (implemented by adapters/postgres). Reads run in the ambient
// transaction of ctx when there is one and on the pool otherwise; writes and locks only inside a
// unit of work. Errors are kernel/errs errors without SQL text or values.
type ComputerStore interface {
	// InsertComputerSession inserts s in opening. A second session that is not closed on the host
	// is 409 conflict with reason computer_busy, of the run computer_run_busy (the unique indexes).
	InsertComputerSession(ctx context.Context, s domain.ComputerSession) error
	// ComputerSession returns the session id without a lock; found is false for an unknown id.
	ComputerSession(ctx context.Context, id ids.ID) (domain.ComputerSession, bool, error)
	// LockComputerSession returns the session id and locks its row.
	LockComputerSession(ctx context.Context, id ids.ID) (domain.ComputerSession, bool, error)
	// LockOpenComputerSessionsOfHost, OfUser and OfRun return the sessions that are not closed of
	// a host, a person or a run, locked in ascending id.
	LockOpenComputerSessionsOfHost(ctx context.Context, hostID ids.ID) ([]domain.ComputerSession, error)
	LockOpenComputerSessionsOfUser(ctx context.Context, userID ids.ID) ([]domain.ComputerSession, error)
	LockOpenComputerSessionsOfRun(ctx context.Context, runID ids.ID) ([]domain.ComputerSession, error)
	// OpenComputerSessionOfRun reports a session of the run that is not closed.
	OpenComputerSessionOfRun(ctx context.Context, runID ids.ID) (bool, error)
	// RunHadComputerSession reports whether the run opened a computer session at all.
	RunHadComputerSession(ctx context.Context, runID ids.ID) (bool, error)
	// ComputerSessionsByIDs returns the computer sessions among list, ascending, without a lock.
	ComputerSessionsByIDs(ctx context.Context, list []ids.ID) ([]domain.ComputerSession, error)
	// CountOpenComputerSessions returns the sessions that are not closed of the host and of the run.
	CountOpenComputerSessions(ctx context.Context, hostID, runID ids.ID) (ofHost, ofRun int, err error)
	// OpenComputerHosts returns the hosts with a session that is not closed, ascending.
	OpenComputerHosts(ctx context.Context) ([]ids.ID, error)
	// ListComputerSessions returns up to limit sessions of userID, newest first, in state ("": any).
	ListComputerSessions(ctx context.Context, userID ids.ID, state string, limit int) ([]ComputerRow, error)
	// HostName returns the name of the host id ("" for an unknown host).
	HostName(ctx context.Context, id ids.ID) (string, error)
	// AttachComputerSession writes attached_at = at once for a session in opening; attached is false
	// when the session is no longer in opening or was attached before.
	AttachComputerSession(ctx context.Context, id ids.ID, at time.Time) (attached bool, err error)
	// StartComputerSession turns the opening session id active or paused (locked) at at.
	StartComputerSession(ctx context.Context, id ids.ID, state string, at time.Time) error
	// UpdateComputerState writes w for the active or paused session id.
	UpdateComputerState(ctx context.Context, id ids.ID, w ComputerStateWrite) error
	// EndComputerSession closes the session id with end at at (the one closing statement); a
	// session that is already closed changes no row: kind internal.
	EndComputerSession(ctx context.Context, id ids.ID, end domain.ComputerEnd, w ComputerEndWrite, at time.Time) error
	// StaleOpeningComputerSessions returns the sessions in opening opened before before, ascending.
	StaleOpeningComputerSessions(ctx context.Context, before time.Time) ([]ids.ID, error)
	// SetComputerDisabledGeneration writes hosts.computer_disabled_generation and the version.
	SetComputerDisabledGeneration(ctx context.Context, hostID ids.ID, generation, version int64) error
}

// ComputerRunFacts are the facts of the run of a computer action that SR13 reads (07 SR13, 27 5.1
// step 5), read by runs in the transaction of the check (CU-04 Design note 3): the run, its thread
// and agent, the triggering person, whether it is active, the tool class of its agent, the trigger
// with the origin and author of a triggering message, the triggering command, every row of
// run_feeders, and the display name of the agent for computer.open.
type ComputerRunFacts struct {
	RunID, ThreadID, AgentID, TriggeredBy ids.ID
	Active                                bool
	ToolClass, TriggerKind                string
	TriggerOrigin                         string
	TriggerAuthorID                       ids.ID
	TriggeringCommand                     bool
	Feeders                               []authorize.ComputerFeeder
	AgentName                             string
}

// ComputerRuns reads the run facts of SR13 from the module runs (bound in internal/wire; terminal
// never imports runs internals, CU-04 Design note 2).
type ComputerRuns interface {
	// ComputerRun returns the facts of runID; found is false for an unknown run.
	ComputerRun(ctx context.Context, runID ids.ID) (ComputerRunFacts, bool, error)
}

// ComputerOpenOp is the message computer.open (schema HostComputerOpen).
type ComputerOpenOp struct {
	SessionID, RunID, AgentID, UserID ids.ID
	AgentName, DataPath               string
	ExpiresAt                         time.Time
	Settings                          ComputerSettings
}

// ComputerSettings are the server values of a session (schema HostComputerSettings, 27 18).
type ComputerSettings struct {
	ImageMaxEdge, JPEGQuality, KeyRepeatMax, AuditAppsMax int
	Settle, WaitMax, StatusInterval                       time.Duration
	AutoScreenshot                                        bool
}

// ComputerControl sends the computer ops of the server on the control connection of a host on
// this node (implemented by adapters/wsrelay.Links next to ControlLinks). Every method is safe for
// concurrent use and never waits on the network; without an eligible link of the host it returns
// ErrHostNotConnected or ErrHostNotReady and sends nothing.
type ComputerControl interface {
	OpenComputer(ctx context.Context, hostID ids.ID, op ComputerOpenOp) error
	CloseComputer(ctx context.Context, hostID, sessionID ids.ID, reason domain.ComputerCloseReason) error
	DisableComputer(ctx context.Context, hostID ids.ID, generation int64) error
}

// ComputerExpectation is what one opening tells the computer relay of a node: the session, its
// host and the deadline of the pairing (computer.open_timeout from the receipt of the signal).
type ComputerExpectation struct {
	SessionID, HostID ids.ID
	Deadline          time.Time
}

// ComputerRelay is the outgoing port to the computer relay of this node (adapters/wsrelay).
type ComputerRelay interface {
	// ExpectComputer creates the slot of e.SessionID once; a slot of the session stays.
	ExpectComputer(e ComputerExpectation)
}

// noComputerRelay is the relay of a process without the computer stream routes.
type noComputerRelay struct{}

// ExpectComputer implements ComputerRelay: nothing to expect.
func (noComputerRelay) ExpectComputer(ComputerExpectation) {}

// computerBindings carries the late bound ports of computer use.
type computerBindings struct {
	runs  atomic.Pointer[computerRunsHolder]
	relay atomic.Pointer[computerRelayHolder]
}

// computerRunsHolder and computerRelayHolder carry the bound ports through the atomic pointers.
type (
	computerRunsHolder  struct{ r ComputerRuns }
	computerRelayHolder struct{ r ComputerRelay }
)

// BindComputerRuns binds the run facts of SR13 (internal/wire, after the module runs is built);
// before it every computer action fails closed.
func (s *Service) BindComputerRuns(r ComputerRuns) { s.computer.runs.Store(&computerRunsHolder{r: r}) }

// BindComputerRelay binds the computer relay of this process.
func (s *Service) BindComputerRelay(r ComputerRelay) {
	if r == nil {
		r = noComputerRelay{}
	}
	s.computer.relay.Store(&computerRelayHolder{r: r})
}

// computerRuns returns the bound run facts, nil before BindComputerRuns.
func (s *Service) computerRuns() ComputerRuns {
	if h := s.computer.runs.Load(); h != nil {
		return h.r
	}
	return nil
}

// computerRelay returns the bound computer relay, a relay without slots before BindComputerRelay.
func (s *Service) computerRelay() ComputerRelay {
	if h := s.computer.relay.Load(); h != nil {
		return h.r
	}
	return noComputerRelay{}
}

// computerControl returns the computer ops of the bound control links; nil when the links do not
// send them (a process without the route /hosts/connect).
func (s *Service) computerControl() ComputerControl {
	if c, ok := s.control().(ComputerControl); ok {
		return c
	}
	return nil
}
