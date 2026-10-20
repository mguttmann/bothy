// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The terminal across several api nodes (M2-47, docs/spec/24-terminal.md sections 3.4 and 9):
// the hub receives the signals of every node, keeps the presence view of the control connections
// of other nodes, probes an older control connection of this node when a newer one is announced
// elsewhere, holds back session.open during the probation of a new connection and reconciles
// after its LISTEN connection came back. With cluster.mode single it only hands the signals of the
// use cases on, so one node runs the same handler with a local transport.

// The log record of a second control connection across nodes: code only (Design note 14).
const msgDuplicate = "terminal duplicate control connection"

// ClusterConfig holds the configuration of the hub.
type ClusterConfig struct {
	// Multi is cluster.mode multi; without it the hub has no presence view, no probe and no
	// probation.
	Multi bool
	// HeartbeatInterval and HeartbeatMissLimit are terminal.host_heartbeat_interval and
	// terminal.host_heartbeat_miss_limit: their product is the window a remote entry counts.
	HeartbeatInterval  time.Duration
	HeartbeatMissLimit int
	// DuplicateProbeTimeout is terminal.host_duplicate_probe_timeout: the probe of an older
	// connection, and half the probation of a new one.
	DuplicateProbeTimeout time.Duration
}

// ClusterUseCases are the use cases the hub calls (app.Service).
type ClusterUseCases interface {
	HandleSignal(ctx context.Context, s domain.Signal)
	SendSignal(ctx context.Context, s domain.Signal) error
	HostCloseCodes(ctx context.Context, list []ids.ID) (map[ids.ID]domain.CloseCode, error)
	SessionOpenValid(ctx context.Context, hostID, sessionID ids.ID) (bool, error)
	ResolveDuplicate(ctx context.Context, hostID ids.ID) error
	ConnectionLost(ctx context.Context, hostID ids.ID) error
	StaleReplaced(ctx context.Context, hostID ids.ID) error
}

// Compile-time checks: the use cases of the module; the hub is the handler of the signals.
var (
	_ ClusterUseCases   = (*app.Service)(nil)
	_ app.SignalHandler = (*Hub)(nil)
)

// Hub is the handler of the terminal signals of one process (Design notes 3 to 9 of M2-47). It is
// safe for concurrent use.
type Hub struct {
	cfg      ClusterConfig
	uc       ClusterUseCases
	links    *Links
	relay    *Relay
	clk      clock.Clock
	gen      *ids.Generator
	presence *domain.PresenceView
	log      *slog.Logger
}

// NewHub returns the hub over the registry of control connections and the relay of this process
// and binds itself to the registry. clk nil means clock.System(), log nil slog.Default(). Errors:
// a missing collaborator or, with Multi, a window or probe below 1.
func NewHub(cfg ClusterConfig, uc ClusterUseCases, links *Links, relay *Relay, clk clock.Clock, log *slog.Logger) (*Hub, error) {
	if uc == nil || links == nil || relay == nil {
		return nil, errors.New("wsrelay: NewHub needs the use cases, the registry and the relay")
	}
	if cfg.Multi && (cfg.HeartbeatInterval <= 0 || cfg.HeartbeatMissLimit < 1 || cfg.DuplicateProbeTimeout <= 0) {
		return nil, errors.New("wsrelay: terminal.host_heartbeat_interval, terminal.host_heartbeat_miss_limit and " +
			"terminal.host_duplicate_probe_timeout must be positive")
	}
	if clk == nil {
		clk = clock.System()
	}
	if log == nil {
		log = slog.Default()
	}
	h := &Hub{cfg: cfg, uc: uc, links: links, relay: relay, clk: clk, gen: ids.NewGenerator(clk, rand.Reader),
		presence: domain.NewPresenceView(), log: log}
	links.bindHub(h)
	if cfg.Multi {
		relay.BindSender(uc.SendSignal)
	}
	return h, nil
}

// window is the time a remote entry counts.
func (h *Hub) window() time.Duration {
	return h.cfg.HeartbeatInterval * time.Duration(h.cfg.HeartbeatMissLimit)
}

// HandleSignal implements app.SignalHandler: session.open and the host transitions go to the use
// cases; presence, withdrawal and pairing are the hub's own.
func (h *Hub) HandleSignal(ctx context.Context, s domain.Signal) {
	switch s.Kind {
	case domain.SignalSessionOpen, domain.SignalHostClose:
		h.uc.HandleSignal(ctx, s)
	case domain.SignalHostPresent:
		if h.cfg.Multi {
			h.present(ctx, s.HostID, s.ConnID)
		}
	case domain.SignalHostGone:
		h.presence.Gone(s.HostID, s.ConnID)
	case domain.SignalSessionPaired:
		h.relay.MarkPairedElsewhere(s.SessionID)
	}
}

// present handles terminal.host_present of conn (Design notes 6 and 7): an own connection is
// ignored (rule a); a remote one counts in the view; an older local connection of the host is
// probed (rule b); a newer local one announces itself at once (rule c), so the node of the older
// one probes it.
func (h *Hub) present(ctx context.Context, host, conn ids.ID) {
	if h.links.ownConn(host, conn) {
		return
	}
	h.presence.Seen(host, conn, h.clk.Now())
	d := h.links.get(host)
	if d == nil || d.isClosing() {
		return
	}
	switch {
	case domain.OlderConnection(d.conn, conn):
		h.probe(d)
	case domain.OlderConnection(conn, d.conn) && d.isReported():
		h.announce(ctx, d)
	}
}

// probe runs M2-43's probe on the older local connection d once at a time: no answer within
// terminal.host_duplicate_probe_timeout closes d with 4009, detaches its sessions, audits the
// replacement and withdraws d; an answer is the duplicate verdict in one unit of work, which
// stages terminal.host_close, so every node closes the host's control connections with 4112.
func (h *Hub) probe(d *link) {
	if !d.startProbe() {
		return
	}
	go func() {
		defer d.endProbe()
		ctx := context.Background()
		host := d.login.HostID
		pctx, cancel := context.WithTimeout(ctx, h.cfg.DuplicateProbeTimeout)
		err := d.ws.Ping(pctx)
		cancel()
		if err == nil {
			h.log.LogAttrs(ctx, slog.LevelWarn, msgDuplicate, slog.Int(attr.CloseCode, int(domain.CodeHostDuplicate)))
			if err := h.uc.ResolveDuplicate(ctx, host); err != nil {
				h.log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
			}
			return
		}
		h.log.LogAttrs(ctx, slog.LevelWarn, msgDuplicate, slog.Int(attr.CloseCode, int(domain.CodeHeartbeatTimeout)))
		d.markHandled()
		if h.links.remove(d) && d.isReported() {
			h.links.publish(host, false)
		}
		d.close(domain.CodeHeartbeatTimeout)
		h.relay.ControlLost(host, domain.CodeHeartbeatTimeout)
		h.gone(ctx, d)
		if err := h.uc.ConnectionLost(ctx, host); err != nil {
			h.log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
		}
		if err := h.uc.StaleReplaced(ctx, host); err != nil {
			h.log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
		}
	}()
}

// announce sends terminal.host_present of the eligible local connection k (after its first report
// and after each heartbeat, Design note 6); the first announcement starts its probation.
func (h *Hub) announce(ctx context.Context, k *link) {
	if !h.cfg.Multi {
		return
	}
	k.announced(h.clk.Now())
	h.send(ctx, domain.Signal{Kind: domain.SignalHostPresent, HostID: k.login.HostID, ConnID: k.conn})
}

// gone sends terminal.host_gone of k once, when k was announced.
func (h *Hub) gone(ctx context.Context, k *link) {
	if h.cfg.Multi && k.withdraw() {
		h.send(ctx, domain.Signal{Kind: domain.SignalHostGone, HostID: k.login.HostID, ConnID: k.conn})
	}
}

// send delivers s; a failure is logged with its type only (the next heartbeat repeats presence).
func (h *Hub) send(ctx context.Context, s domain.Signal) {
	if err := h.uc.SendSignal(context.WithoutCancel(ctx), s); err != nil {
		h.log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
	}
}

// connID returns a new connection ID: UUIDv7 on the clock of this node (Design note 7).
func (h *Hub) connID() ids.ID { return h.gen.New() }

// remoteConnected reports a counting remote entry of host (Design note 6).
func (h *Hub) remoteConnected(host ids.ID) bool {
	return h.cfg.Multi && h.presence.Connected(host, h.clk.Now(), h.window())
}

// holdOpen reports whether k is in probation and, if so, sends cmd at its end after checking the
// database again: the session still not closed, the host still active, k still the open link of
// the host (Design note 7 d). The session's own terminal.open_timeout keeps running.
func (h *Hub) holdOpen(k *link, hostID ids.ID, cmd app.SessionOpen) bool {
	if !h.cfg.Multi {
		return false
	}
	end := domain.ProbationEnd(k.firstAnnounce(h.clk.Now()), h.cfg.DuplicateProbeTimeout)
	wait := end.Sub(h.clk.Now())
	if wait <= 0 {
		return false
	}
	time.AfterFunc(wait, func() {
		if !h.links.current(k) || k.isClosing() {
			return
		}
		ok, err := h.uc.SessionOpenValid(context.Background(), hostID, cmd.SessionID)
		if err == nil && ok {
			k.send(sessionOpenFrame(cmd))
		}
	})
	return true
}

// Reconcile implements the reconciliation after the LISTEN connection came back (Design note 9):
// the hosts of every control connection and every relay side of this node are read; what may no
// longer be connected closes with the code of its state; every eligible control connection
// announces itself at once. Sessions that ended meanwhile need nothing here: the log tail reads on
// from its own position.
func (h *Hub) Reconcile(ctx context.Context) {
	hosts := h.links.hosts()
	seen := map[ids.ID]bool{}
	for _, id := range hosts {
		seen[id] = true
	}
	for _, id := range h.relay.hostsWithSides() {
		if !seen[id] {
			hosts = append(hosts, id)
		}
	}
	if len(hosts) > 0 {
		codes, err := h.uc.HostCloseCodes(ctx, hosts)
		if err != nil {
			h.log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
		}
		for id, code := range codes {
			h.links.CloseHost(id, code)
		}
	}
	for _, k := range h.links.eligible() {
		h.announce(ctx, k)
	}
}
