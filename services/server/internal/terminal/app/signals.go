// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The signals of the terminal between api nodes (M2-47 Design notes 3 and 4,
// docs/spec/24-terminal.md section 9). A use case that acts on a live connection after its commit
// stages its signal in its unit of work and hands it over after the commit; the handler of the
// relay acts on it on every node. The end of a session has no signal: it reaches every node
// through the event log tail (M2-44 Design note 11).

// Signals is the outgoing port of the signals. Stage runs inside the unit of work (phase work),
// Committed after UnitOfWork.Do returned nil, Send outside any unit.
type Signals interface {
	// Stage registers s in the transaction of ctx: with several nodes the notification is
	// delivered only after the commit and never after a rollback. Errors: those of the store.
	Stage(ctx context.Context, s domain.Signal) error
	// Committed hands s over after the commit: with one node to the local handler.
	Committed(ctx context.Context, s domain.Signal)
	// Send delivers s outside any unit (presence, pairing): with several nodes in its own
	// statement. Errors: those of the store.
	Send(ctx context.Context, s domain.Signal) error
}

// SignalHandler acts on one signal on this node; the wsrelay hub implements it. It never blocks
// the caller on the network: frames go to the writer of a connection, and every database read is
// bounded by database.statement_timeout.
type SignalHandler interface {
	HandleSignal(ctx context.Context, s domain.Signal)
}

// LocalSignals are the Signals of cluster.mode single: Stage and Send do nothing, Committed calls
// the handler of this process once. One handler serves both modes, so one node behaves like
// several nodes with a local transport (ruling D-M2-47-4).
type LocalSignals struct {
	h atomic.Pointer[handlerHolder]
}

// handlerHolder carries the bound handler through the atomic pointer.
type handlerHolder struct{ h SignalHandler }

// Compile-time check.
var _ Signals = (*LocalSignals)(nil)

// NewLocalSignals returns LocalSignals without a handler (a process without the relay: Committed
// does nothing until Bind).
func NewLocalSignals() *LocalSignals { return &LocalSignals{} }

// Bind sets the handler of this process; internal/wire binds the hub of the relay.
func (l *LocalSignals) Bind(h SignalHandler) { l.h.Store(&handlerHolder{h: h}) }

// Stage implements Signals: nothing to stage with one node.
func (*LocalSignals) Stage(context.Context, domain.Signal) error { return nil }

// Committed implements Signals: the local handler, once.
func (l *LocalSignals) Committed(ctx context.Context, s domain.Signal) {
	if hh := l.h.Load(); hh != nil && hh.h != nil {
		hh.h.HandleSignal(ctx, s)
	}
}

// Send implements Signals: no other node to tell.
func (*LocalSignals) Send(context.Context, domain.Signal) error { return nil }

// signalsHolder carries the bound Signals through the atomic pointer of Service.
type signalsHolder struct{ s Signals }

// BindSignals binds the signals of the process (M2-47): LocalSignals with cluster.mode single,
// the NOTIFY implementation of adapters/postgres with multi. internal/wire binds them right after
// the module is built.
func (s *Service) BindSignals(sig Signals) {
	if sig == nil {
		sig = NewLocalSignals()
	}
	s.signals.Store(&signalsHolder{s: sig})
}

// clusterSignals returns the bound signals (New binds LocalSignals over the Service itself).
func (s *Service) clusterSignals() Signals { return s.signals.Load().s }

// stage stages a signal of kind for host (and session) in the unit of ctx.
func (s *Service) stage(ctx context.Context, sig domain.Signal) error {
	return s.clusterSignals().Stage(ctx, sig)
}

// committed hands a staged signal over after the commit.
func (s *Service) committed(ctx context.Context, sig domain.Signal) {
	s.clusterSignals().Committed(ctx, sig)
}

// sendReady sends ready again on the control connection of hostID once the change is durable:
// inside a unit of work (the Idempotency-Key stage runs the whole request in one) after its
// outermost commit, because the ready re-reads the row and would otherwise carry the state before
// the change (M2-48); outside one at once.
func (s *Service) sendReady(ctx context.Context, hostID ids.ID) {
	if !txn.AfterCommit(ctx, func() { s.control().SendReady(hostID) }) {
		s.control().SendReady(hostID)
	}
}

// SendSignal delivers s outside any unit of work (the relay's presence and pairing signals).
// Errors: those of the store.
func (s *Service) SendSignal(ctx context.Context, sig domain.Signal) error {
	return s.clusterSignals().Send(ctx, sig)
}

// hostCloseSignal is terminal.host_close of hostID.
func hostCloseSignal(hostID ids.ID) domain.Signal {
	return domain.Signal{Kind: domain.SignalHostClose, HostID: hostID}
}

// sessionOpenSignal is terminal.session_open of the session.
func sessionOpenSignal(hostID, sessionID ids.ID) domain.Signal {
	return domain.Signal{Kind: domain.SignalSessionOpen, HostID: hostID, SessionID: sessionID}
}

// grantNotes keeps what a grant knows that the row does not: the working directory index of an
// opening of a claude profile (ruling D-M2-44-4). The handler of terminal.session_open on the node
// that committed the grant takes it; other nodes have none, as M2-44 rules for another node.
type grantNotes struct {
	mu  sync.Mutex
	wdi map[ids.ID]*int
}

// put remembers the index of session (nil removes it).
func (g *grantNotes) put(session ids.ID, wdi *int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.wdi == nil {
		g.wdi = map[ids.ID]*int{}
	}
	if wdi == nil {
		delete(g.wdi, session)
		return
	}
	v := *wdi
	g.wdi[session] = &v
}

// take returns and forgets the index of session.
func (g *grantNotes) take(session ids.ID) *int {
	g.mu.Lock()
	defer g.mu.Unlock()
	v := g.wdi[session]
	delete(g.wdi, session)
	return v
}
