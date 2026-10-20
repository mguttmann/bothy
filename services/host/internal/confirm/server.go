// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"
)

// Peer is the user of a connected agent as the operating system reports it.
type Peer struct {
	// UID is the uid (Linux, macOS) or the user SID of the process token (Windows).
	UID string
	// PID is the process ID of the agent.
	PID int
	// Session is the session ID of the token of the agent (Windows, read from the pipe client;
	// zero on Linux and macOS, which have no such check).
	Session uint32
}

// Rule decides which peers may answer (brief M2-45 Design note 15 a): not the service user, not
// a configured session user, not a former session user of bindings.json, and the owner of the
// active console session (Linux: the user of the active session on seat0 from loginctl; macOS:
// the owner of /dev/console; Windows: the user of the active console session, and the peer
// process runs in that session). Every missing function and every lookup error refuses.
type Rule struct {
	// ServiceUID is the uid or SID of the service user.
	ServiceUID string
	// Excluded returns the uids or SIDs of the configured and former session users; it is called
	// at each accept, so a change of the configuration or of bindings.json applies at once.
	Excluded func() []string
	// ConsoleOwner returns the uid or SID of the owner of the active console session
	// (production: ConsoleOwner).
	ConsoleOwner func(ctx context.Context) (string, error)
	// ConsoleSession reports whether peer p runs in the active console session. Nil
	// skips the check (Linux and macOS, where the console owner decides); on Windows DefaultRule
	// sets it, because the person at the machine may also have a disconnected remote session.
	ConsoleSession func(ctx context.Context, p Peer) (bool, error)
}

// DefaultRule returns the Rule of this platform with the production lookups ConsoleOwner and,
// on Windows, the console session check of the peer process.
//
// serviceUID is the uid or SID of the service user; excluded returns the configured and former
// session users (see Rule.Excluded).
func DefaultRule(serviceUID string, excluded func() []string) Rule {
	return Rule{ServiceUID: serviceUID, Excluded: excluded, ConsoleOwner: ConsoleOwner, ConsoleSession: consoleSession}
}

// Allow reports whether peer p may answer confirm requests. It returns false for an empty user,
// the service user, an excluded user, a user that does not own the console session, a process
// outside the console session, and on every lookup error.
func (r Rule) Allow(ctx context.Context, p Peer) bool {
	if p.UID == "" || p.UID == r.ServiceUID || r.Excluded == nil || r.ConsoleOwner == nil {
		return false
	}
	if slices.Contains(r.Excluded(), p.UID) {
		return false
	}
	owner, err := r.ConsoleOwner(ctx)
	if err != nil || owner != p.UID {
		return false
	}
	if r.ConsoleSession != nil {
		in, err := r.ConsoleSession(ctx, p)
		if err != nil || !in {
			return false
		}
	}
	return true
}

// agentQueue bounds the messages waiting for one agent; an agent that does not read is closed
// instead of blocking a request. A protocol constant, not a product limit.
const agentQueue = 64

// ServerOptions configures NewServer.
type ServerOptions struct {
	// Listener is the endpoint from Listen (tests pass their own).
	Listener net.Listener
	// Rule decides which peers are accepted.
	Rule Rule
	// PeerOf returns the peer of an accepted connection (production: PeerOf; tests replace it).
	// Nil refuses every connection.
	PeerOf func(net.Conn) (Peer, error)
	// Log receives codes only (confirm_peer_refused, confirm_agent_closed); nil discards them.
	Log *slog.Logger
}

// Server is the service side: it accepts agents on the listener, checks every peer with the rule
// at accept and again before every request (refused: close without sending and log the code
// confirm_peer_refused), and asks the accepted agents.
type Server struct {
	o   ServerOptions
	log *slog.Logger

	mu      sync.Mutex
	agents  map[*agentConn]struct{}
	pending map[string]*pending
	// closed is set when Serve returns; later agents are closed at once.
	closed bool
	wg     sync.WaitGroup
}

// agentConn is one accepted agent.
type agentConn struct {
	// peer is the user of the agent, checked again by Ask at every request.
	peer Peer
	c    net.Conn
	out  chan []byte
	done chan struct{}
	once sync.Once
}

// pending is one open request: the agents it was sent to and the slot of the first answer.
type pending struct {
	asked  map[*agentConn]struct{}
	result chan bool
}

// NewServer returns a Server for o; Serve starts it.
func NewServer(o ServerOptions) *Server {
	log := o.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Server{o: o, log: log, agents: map[*agentConn]struct{}{}, pending: map[string]*pending{}}
}

// Serve accepts agents until ctx ends; then it closes the listener and every agent and returns
// nil. It returns an error when the listener fails otherwise; every later Ask then gives false.
func (s *Server) Serve(ctx context.Context) error {
	if s.o.Listener == nil {
		return errors.New("confirm: no listener")
	}
	stop := context.AfterFunc(ctx, func() { _ = s.o.Listener.Close() })
	defer stop()
	defer s.closeAll()
	for {
		c, err := s.o.Listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		s.wg.Go(func() { s.admit(ctx, c) })
	}
}

// closeAll closes every agent and waits for their goroutines.
func (s *Server) closeAll() {
	s.mu.Lock()
	s.closed = true
	agents := make([]*agentConn, 0, len(s.agents))
	for a := range s.agents {
		agents = append(agents, a)
	}
	s.mu.Unlock()
	for _, a := range agents {
		a.close()
	}
	s.wg.Wait()
}

// admit checks the peer of c and serves it as an agent, or closes it without sending anything.
func (s *Server) admit(ctx context.Context, c net.Conn) {
	if s.o.PeerOf == nil {
		s.refuse(ctx, c)
		return
	}
	p, err := s.o.PeerOf(c)
	if err != nil || !s.o.Rule.Allow(ctx, p) {
		s.refuse(ctx, c)
		return
	}
	a := &agentConn{peer: p, c: c, out: make(chan []byte, agentQueue), done: make(chan struct{})}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = c.Close()
		return
	}
	s.agents[a] = struct{}{}
	s.mu.Unlock()
	s.wg.Go(func() { a.write() })
	s.read(ctx, a)
}

// refuse logs the code first and then closes c, so the log holds every refusal a peer has seen.
func (s *Server) refuse(ctx context.Context, c net.Conn) {
	s.log.WarnContext(ctx, "confirm_peer_refused")
	_ = c.Close()
}

// read handles the answers of a until it closes or breaks the protocol.
func (s *Server) read(ctx context.Context, a *agentConn) {
	defer s.drop(a)
	r := bufio.NewReaderSize(a.c, maxLine)
	for {
		line, err := readLine(r)
		if err != nil {
			if errors.Is(err, errLineTooLong) {
				s.log.WarnContext(ctx, "confirm_agent_closed")
			}
			return
		}
		id, allow, err := decodeAnswer(line)
		if err != nil {
			s.log.WarnContext(ctx, "confirm_agent_closed")
			return
		}
		s.answer(a, id, allow)
	}
}

// write sends the queued lines of a until it closes.
func (a *agentConn) write() {
	for {
		select {
		case b := <-a.out:
			if _, err := a.c.Write(b); err != nil {
				a.close()
				return
			}
		case <-a.done:
			return
		}
	}
}

// send queues line for a; a full queue closes a.
func (a *agentConn) send(line []byte) {
	select {
	case a.out <- line:
	case <-a.done:
	default:
		a.close()
	}
}

func (a *agentConn) close() {
	a.once.Do(func() {
		close(a.done)
		_ = a.c.Close()
	})
}

// drop removes a: every request that waits only for a ends as a denial.
func (s *Server) drop(a *agentConn) {
	a.close()
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, a)
	for id, p := range s.pending {
		if _, ok := p.asked[a]; !ok {
			continue
		}
		delete(p.asked, a)
		if len(p.asked) == 0 {
			delete(s.pending, id)
			p.result <- false
		}
	}
}

// answer takes the answer of a to id when a was asked and the request is still open; the other
// agents asked get cancel.
func (s *Server) answer(a *agentConn, id string, allow bool) {
	s.mu.Lock()
	p := s.pending[id]
	if p == nil {
		s.mu.Unlock()
		return
	}
	if _, ok := p.asked[a]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.pending, id)
	delete(p.asked, a)
	others := keys(p.asked)
	s.mu.Unlock()
	p.result <- allow
	cancelAll(others, id)
}

// Agents returns the number of accepted agents that are connected.
func (s *Server) Agents() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.agents)
}

// Ask sends r to every accepted agent and returns true only when the first answer within timeout
// (host_service.local_confirm_timeout, passed by the caller) is allow=true. No agent, a denial,
// the timeout, the end of ctx, an invalid request or the loss of every asked agent give false.
// The other agents get cancel once one answered or the request ended. Ask is safe for concurrent
// use; several requests may be open at once.
func (s *Server) Ask(ctx context.Context, r Request, timeout time.Duration) bool {
	if timeout <= 0 || (r.Kind != KindOpen && r.Kind != KindAttach) || r.Profile == "" || r.SessionUser == "" {
		return false
	}
	id := newID()
	line := encodeLine(confirmMsg{Op: opConfirm, ID: id, Kind: r.Kind, Profile: r.Profile, SessionUser: r.SessionUser,
		TimeoutMS: max(1, timeout.Milliseconds())})
	asked := s.allowed(ctx)
	if len(asked) == 0 {
		return false
	}
	p := &pending{asked: map[*agentConn]struct{}{}, result: make(chan bool, 1)}
	for _, a := range asked {
		p.asked[a] = struct{}{}
	}
	s.mu.Lock()
	s.pending[id] = p
	s.mu.Unlock()
	for _, a := range asked {
		a.send(line)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case allow := <-p.result:
		return allow
	case <-timer.C:
	case <-ctx.Done():
	}
	s.mu.Lock()
	if s.pending[id] != p {
		// An answer or the loss of the last agent came at the same moment and was taken.
		s.mu.Unlock()
		return <-p.result
	}
	delete(s.pending, id)
	rest := keys(p.asked)
	s.mu.Unlock()
	cancelAll(rest, id)
	return false
}

// allowed returns the connected agents whose peer the rule still accepts, and closes the others:
// the console owner can change and a user can become a session user while an agent stays
// connected, so the check at accept alone would let a stale agent answer.
func (s *Server) allowed(ctx context.Context) []*agentConn {
	s.mu.Lock()
	agents := keys(s.agents)
	s.mu.Unlock()
	out := make([]*agentConn, 0, len(agents))
	for _, a := range agents {
		if s.o.Rule.Allow(ctx, a.peer) {
			out = append(out, a)
			continue
		}
		s.log.WarnContext(ctx, "confirm_peer_refused")
		a.close()
	}
	return out
}

func keys(m map[*agentConn]struct{}) []*agentConn {
	out := make([]*agentConn, 0, len(m))
	for a := range m {
		out = append(out, a)
	}
	return out
}

func cancelAll(agents []*agentConn, id string) {
	if len(agents) == 0 {
		return
	}
	line := encodeLine(cancelMsg{Op: opCancel, ID: id})
	for _, a := range agents {
		a.send(line)
	}
}
