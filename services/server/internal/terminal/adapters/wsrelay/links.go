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
type ReadySource interface { // NOSONAR: names the source of ready it stands for; see D-M2-43-24.
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

// Connected implements app.ControlLinks: a link whose first report after ready was applied.
func (l *Links) Connected(hostID ids.ID) bool {
	k := l.get(hostID)
	return k != nil && k.isReported()
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
	r, code, err := src.Ready(context.Background(), k.login)
	switch {
	case err != nil:
		k.close(domain.CodeUnknownError)
	case code != 0:
		k.close(code)
	default:
		k.send(readyFrame(r))
	}
}

// publish sends host.presence_changed {host_id, connected}: resource host, no scope, no actor,
// no seq. A failure is logged with the type only; presence is at most once.
func (l *Links) publish(hostID ids.ID, connected bool) {
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
	ended    chan struct{}
	once     gosync.Once
	mu       gosync.Mutex
	reported bool
	code     domain.CloseCode
	// handled marks a link whose sessions another path detached or ended (duplicate, shutdown).
	handled bool
	// lastJWKS is the key set last sent on the link (hello or heartbeat_ack).
	lastJWKS     string
	writeTimeout time.Duration
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
