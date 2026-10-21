// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"log/slog"
	gosync "sync"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The ephemeral event of the presence of a host (05 section 6, 24 section 10.2).
const (
	eventPresenceChanged = "host.presence_changed"
	resourceHost         = "host"
	logEventType         = "event.type"
	errorDetach          = "detach"
)

// ReadySource re-reads the content of ready of a host (app.Service.Ready).
type ReadySource interface {
	Ready(ctx context.Context, l app.Login) (app.Ready, domain.CloseCode, error)
}

// LossHandler detaches the sessions of a host whose link ended (app.Service.ConnectionLost).
type LossHandler interface {
	ConnectionLost(ctx context.Context, hostID ids.ID) error
}

// Links is the per-process registry of the control connections (Design note 7 of M2-43): host
// ID to one link. It implements app.ControlLinks and publishes
// host.presence_changed through kernel/eventlog.EphemeralPublisher on every change of
// Connected. M2-47 adds the path across nodes behind app.ControlLinks. It is safe for concurrent
// use.
type Links struct {
	publisher eventlog.EphemeralPublisher
	log       *slog.Logger
	mu        gosync.Mutex
	byHost    map[ids.ID]*link
	// hostLocks serialize the duplicate check and the registration per host (Design note 5).
	hostLocks map[ids.ID]*gosync.Mutex
	ready     ReadySource
	loss      LossHandler
	closing   bool
	// relay is the relay of this process (M2-44): it learns of session.started and of the end of
	// a control connection; nil before BindRelay.
	relay *Relay
	// hub is the handler of the signals across nodes (M2-47): connection IDs, presence and
	// probation; nil before NewHub, which a process without the relay never builds.
	hub *Hub
	// metrics records bothy.terminal.hosts.connected (M2-46); nil records nothing.
	metrics app.SessionMetrics
}

// BindMetrics sets the instrument bothy.terminal.hosts.connected (M2-46): +1 when a link of this
// process becomes usable for sessions, -1 when it stops being usable. internal/wire calls it once
// before the public listener opens.
func (l *Links) BindMetrics(m app.SessionMetrics) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.metrics = m
}

// bindHub sets the hub of the process (NewHub).
func (l *Links) bindHub(h *Hub) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hub = h
}

// hubOf returns the bound hub, nil before NewHub.
func (l *Links) hubOf() *Hub {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.hub
}

// newConnID returns the connection ID of a new control connection: from the hub's generator on the
// node's clock, else ids.New.
func (l *Links) newConnID() ids.ID {
	if h := l.hubOf(); h != nil {
		return h.connID()
	}
	return ids.New()
}

// ownConn reports whether conn is the ID of the registered link of host on this node.
func (l *Links) ownConn(host, conn ids.ID) bool {
	k := l.get(host)
	return k != nil && k.conn == conn
}

// hosts returns the hosts with a registered link.
func (l *Links) hosts() []ids.ID {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]ids.ID, 0, len(l.byHost))
	for id := range l.byHost {
		out = append(out, id)
	}
	return out
}

// eligible returns the registered links whose first report was applied.
func (l *Links) eligible() []*link {
	l.mu.Lock()
	list := make([]*link, 0, len(l.byHost))
	for _, k := range l.byHost {
		list = append(list, k)
	}
	l.mu.Unlock()
	out := list[:0]
	for _, k := range list {
		if k.isReported() && !k.isClosing() {
			out = append(out, k)
		}
	}
	return out
}

// BindRelay sets the relay the control connections tell of session.started and of their end
// (Design note 10 of M2-44). internal/wire calls it once before the public listener opens.
func (l *Links) BindRelay(r *Relay) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.relay = r
}

// relayOf returns the bound relay, nil before BindRelay.
func (l *Links) relayOf() *Relay {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.relay
}

// Compile-time check: the outgoing port of the use cases (the presence of host reads goes through
// it, app.Service.BindControl).
var _ app.ControlLinks = (*Links)(nil)

// NewLinks returns an empty registry that publishes the presence through publisher (nil: no
// publisher) and logs to log (nil: slog.Default()).
func NewLinks(publisher eventlog.EphemeralPublisher, log *slog.Logger) *Links {
	if log == nil {
		log = slog.Default()
	}
	return &Links{publisher: publisher, log: log, byHost: map[ids.ID]*link{}, hostLocks: map[ids.ID]*gosync.Mutex{}}
}

// Bind sets the use cases the registry calls: the re-read of ready for SendReady and the
// detachment at shutdown. internal/wire calls it once after the module is built.
func (l *Links) Bind(ready ReadySource, loss LossHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ready, l.loss = ready, loss
}

// lockHost returns the unlock of the per-host lock of hostID.
func (l *Links) lockHost(hostID ids.ID) func() {
	l.mu.Lock()
	m, ok := l.hostLocks[hostID]
	if !ok {
		m = &gosync.Mutex{}
		l.hostLocks[hostID] = m
	}
	l.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// get returns the link of hostID, nil without one.
func (l *Links) get(hostID ids.ID) *link {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.byHost[hostID]
}

// put registers k as the link of its host; false while the registry shuts down.
func (l *Links) put(k *link) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing {
		return false
	}
	l.byHost[k.login.HostID] = k
	return true
}

// remove unregisters k when it is still the link of its host and reports whether it was; a newer
// link of the host stays.
func (l *Links) remove(k *link) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byHost[k.login.HostID] != k {
		return false
	}
	delete(l.byHost, k.login.HostID)
	return true
}

// current reports whether k is the registered link of its host.
func (l *Links) current(k *link) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.byHost[k.login.HostID] == k
}

// OpenSession implements app.ControlLinks.
func (l *Links) OpenSession(_ context.Context, hostID ids.ID, cmd app.SessionOpen) error {
	k := l.get(hostID)
	if k == nil {
		return app.ErrHostNotConnected
	}
	if !k.isReported() {
		return app.ErrHostNotReady
	}
	if h := l.hubOf(); h != nil && h.holdOpen(k, hostID, cmd) {
		// A new connection in probation waits (M2-47 Design note 7 d).
		return nil
	}
	k.send(sessionOpenFrame(cmd))
	return nil
}

// CloseSession implements app.ControlLinks.
func (l *Links) CloseSession(_ context.Context, hostID, sessionID ids.ID, reason domain.CloseReason) error {
	if k := l.get(hostID); k != nil {
		k.send(sessionCloseFrame(domain.SessionClose{SessionID: sessionID, Reason: reason}))
	}
	return nil
}

// CloseHost implements app.ControlLinks: the link closes with code; a code that comes with a
// state change of the host detaches nothing (Design note 11).
func (l *Links) CloseHost(hostID ids.ID, code domain.CloseCode) {
	if k := l.get(hostID); k != nil {
		k.close(code)
	}
}

// Connected implements app.ControlLinks: a link whose first report after ready was applied, or
// with cluster.mode multi a counting entry of another node's presence (M2-47 Design note 6).
func (l *Links) Connected(hostID ids.ID) bool {
	if k := l.get(hostID); k != nil && k.isReported() {
		return true
	}
	h := l.hubOf()
	return h != nil && h.remoteConnected(hostID)
}

// SendReady implements app.ControlLinks: ready again with the row read now (M2-42 Design note
// 15); a host that changed into a state without a connection closes with its code.
func (l *Links) SendReady(hostID ids.ID) {
	k := l.get(hostID)
	l.mu.Lock()
	src := l.ready
	l.mu.Unlock()
	if k == nil || src == nil {
		return
	}
	code, err := k.sendReady(func() (app.Ready, domain.CloseCode, error) { return src.Ready(context.Background(), k.login) })
	switch {
	case err != nil:
		k.close(domain.CodeUnknownError)
	case code != 0:
		k.close(code)
	}
}

// sendReady reads ready with read and queues it, both under readyMu, so the ready queued last on
// the link is the one read last. A close code or an error of read is returned and nothing is sent.
func (k *link) sendReady(read func() (app.Ready, domain.CloseCode, error)) (domain.CloseCode, error) {
	k.readyMu.Lock()
	defer k.readyMu.Unlock()
	r, code, err := read()
	if err != nil || code != 0 {
		return code, err
	}
	k.send(readyFrame(r))
	return 0, nil
}

// publish sends host.presence_changed {host_id, connected}: resource host, no scope, no actor,
// no seq, and moves bothy.terminal.hosts.connected (every call is a change of a link of this
// process). A failure is logged with the type only; presence is at most once.
func (l *Links) publish(hostID ids.ID, connected bool) {
	l.mu.Lock()
	m := l.metrics
	l.mu.Unlock()
	if m != nil {
		delta := int64(-1)
		if connected {
			delta = 1
		}
		m.HostsConnected(context.Background(), delta)
	}
	if l.publisher == nil {
		return
	}
	err := l.publisher.PublishEphemeral(context.Background(), eventlog.EphemeralEvent{
		Type: eventPresenceChanged, SchemaVersion: 1, Resource: eventlog.ResourceRef{Kind: resourceHost, ID: hostID},
		Payload: struct {
			HostID    string `json:"host_id"`
			Connected bool   `json:"connected"`
		}{hostID.String(), connected},
	})
	if err != nil {
		l.log.LogAttrs(context.Background(), slog.LevelError, "presence not published", slog.String(logEventType, eventPresenceChanged))
	}
}

// Shutdown is the stop step of the control connections (Design note 11 of M2-43, 02 section 15.3
// step 3): no new link registers; every link detaches its sessions and closes with 4013, within
// ctx.
func (l *Links) Shutdown(ctx context.Context) error {
	l.mu.Lock()
	l.closing = true
	loss := l.loss
	list := make([]*link, 0, len(l.byHost))
	for _, k := range l.byHost {
		list = append(list, k)
	}
	l.mu.Unlock()
	for _, k := range list {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		k.markHandled()
		if loss != nil {
			if err := loss.ConnectionLost(ctx, k.login.HostID); err != nil {
				l.log.LogAttrs(ctx, slog.LevelError, "control connection not detached at shutdown", slog.String(logErrorType, errorDetach))
			}
		}
		k.close(domain.CodeServerRestart)
	}
	for _, k := range list {
		select {
		case <-k.ended:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// link is one authenticated control connection: the WebSocket, its bounded writer queue, the
// login and whether its first report after ready was applied.
type link struct {
	ws    *websocket.Conn
	login app.Login
	// queue holds the encoded frames for the writer (sync.outbound_queue_max).
	queue chan []byte
	done  chan struct{}
	// ended is closed when the reader of the link returned.
	ended chan struct{}
	once  gosync.Once
	mu    gosync.Mutex
	// readyMu orders every read and send of ready on the link (the one of the login and those of
	// SendReady): each ready is read and queued under it, so the last ready the host receives holds
	// the row read last. Without it the ready of the login, read before a confirmHost committed,
	// could be queued after the ready SendReady read after the commit, and the host would keep the
	// older declaration (M2-48).
	readyMu  gosync.Mutex
	reported bool
	code     domain.CloseCode
	// handled marks a link whose sessions another path detached or ended (duplicate, shutdown).
	handled bool
	// lastJWKS is the key set last sent on the link (hello or heartbeat_ack).
	lastJWKS     string
	writeTimeout time.Duration
	// conn is the connection ID of a control connection (M2-47 Design note 7); firstPresent the
	// time of its first terminal.host_present (the start of its probation); probing marks a
	// running probe; withdrawn marks the terminal.host_gone sent.
	conn         ids.ID
	firstPresent time.Time
	probing      bool
	withdrawn    bool
}

// announced records the time of the first announcement of k.
func (k *link) announced(at time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.firstPresent.IsZero() {
		k.firstPresent = at
	}
}

// firstAnnounce returns the time of the first announcement of k; a link not yet announced starts
// its probation at now.
func (k *link) firstAnnounce(now time.Time) time.Time {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.firstPresent.IsZero() {
		k.firstPresent = now
	}
	return k.firstPresent
}

// withdraw reports once whether an announced link is to be withdrawn.
func (k *link) withdraw() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.firstPresent.IsZero() || k.withdrawn {
		return false
	}
	k.withdrawn = true
	return true
}

// startProbe marks a probe of k and reports whether none was running.
func (k *link) startProbe() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.probing {
		return false
	}
	k.probing = true
	return true
}

// endProbe clears the mark of a probe.
func (k *link) endProbe() {
	k.mu.Lock()
	k.probing = false
	k.mu.Unlock()
}

// newLink returns a link over ws with a queue of queueMax frames.
func newLink(ws *websocket.Conn, queueMax int, writeTimeout time.Duration) *link {
	return &link{ws: ws, queue: make(chan []byte, queueMax), done: make(chan struct{}), ended: make(chan struct{}), writeTimeout: writeTimeout}
}

// isReported reports a link whose first report after ready was applied.
func (k *link) isReported() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.reported
}

// markReported sets the flag and reports whether it changed.
func (k *link) markReported() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	was := k.reported
	k.reported = true
	return !was
}

// markHandled marks the link as detached by another path.
func (k *link) markHandled() {
	k.mu.Lock()
	k.handled = true
	k.mu.Unlock()
}

// result returns the close code and whether another path handled the link.
func (k *link) result() (domain.CloseCode, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.code, k.handled
}

// send queues an encoded frame; a full queue closes the link with 4014 (the frame is dropped).
func (k *link) send(b []byte) {
	select {
	case <-k.done:
		return
	default:
	}
	select {
	case k.queue <- b:
	default:
		k.close(domain.CodeSlowConsumer)
	}
}

// close starts the close handshake with code and its name once, in its own goroutine, so no
// caller waits on the network; frames still queued are dropped.
func (k *link) close(code domain.CloseCode) { k.closeReason(code, code.Name()) }

// closeReason is close with the reason text reason (machine data, never content).
func (k *link) closeReason(code domain.CloseCode, reason string) {
	k.once.Do(func() {
		k.mu.Lock()
		k.code = code
		k.mu.Unlock()
		close(k.done)
		go func() { _ = k.ws.Close(websocket.StatusCode(code), reason) }()
	})
}

// quiet stops the text writer of the link without a close frame: the data connection hands the
// WebSocket to the relay after its login (M2-44).
func (k *link) quiet() { k.once.Do(func() { close(k.done) }) }

// isClosing reports a link whose close started.
func (k *link) isClosing() bool {
	select {
	case <-k.done:
		return true
	default:
		return false
	}
}

// write is the writer goroutine: the queued frames in order, each within gateway.write_timeout; a
// missed deadline closes with 4014 (coder/websocket tears the connection down then).
func (k *link) write(ctx context.Context) {
	for {
		select {
		case <-k.done:
			return
		case b := <-k.queue:
			wctx, cancel := context.WithTimeout(ctx, k.writeTimeout)
			err := k.ws.Write(wctx, websocket.MessageText, b)
			timedOut := wctx.Err() != nil
			cancel()
			if err != nil {
				if timedOut {
					k.close(domain.CodeSlowConsumer)
				}
				return
			}
		}
	}
}
