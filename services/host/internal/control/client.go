// SPDX-License-Identifier: Apache-2.0

package control

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	gosync "sync"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// Path is the path of the control connection on the server.
const Path = "/api/v1/hosts/connect"

// Errors of New and Login.
var (
	// ErrNotHTTPS is a server URL that is no absolute https URL: the client dials only wss://.
	ErrNotHTTPS = errors.New("control: the server URL must be an absolute https URL")
	// ErrClosed is a call after Close.
	ErrClosed = errors.New("control: closed")
)

// Config configures the client: the pairing record and keys of the state directory (M2-42) and
// the host configuration.
type Config struct {
	// ServerURL is the https URL of the server from the pairing record.
	ServerURL string
	// HostID is the host id from the pairing record.
	HostID string
	// AuthKey is the Ed25519 login key from the state directory.
	AuthKey ed25519.PrivateKey
	// HostVersion is the version of the host service (SemVer).
	HostVersion string
	// BackoffBase and BackoffCap are host_service.reconnect_backoff_base and _cap.
	BackoffBase, BackoffCap time.Duration
	// RootCAs replaces the system roots when set (tests); never a skip of the verification.
	RootCAs *x509.CertPool
	// Clock drives every timer; nil means clock.Real().
	Clock clock.Clock
	// Log receives codes only; nil means slog.Default().
	Log *slog.Logger
	// Random gives the jitter in [0, 1); nil means math/rand/v2.
	Random func() float64
	// OwnAddresses lists the addresses of this machine for PeerIsLocal; nil means OwnAddresses.
	OwnAddresses func() ([]net.IP, error)
	// Proxy chooses the proxy of a request; cmd/bothy-host passes http.ProxyFromEnvironment (only
	// it reads the environment); nil means no proxy.
	Proxy func(*http.Request) (*url.URL, error)
	// UpdatePath is the update route of the package kind of this binary
	// (buildinfo.UpdatePath); the client logs it in one line when the server refuses the version
	// with 4114 or shares no protocol version (docs/spec/18-release.md 9.1).
	UpdatePath string
}

// Client is the control connection of the host service. It implements startup.Control. It is
// safe for concurrent use.
type Client struct {
	cfg      Config
	url      string
	http     *http.Client
	sessions Sessions
	reports  ReportSource
	keys     KeyStore
	backoff  *BackoffTimer
	log      *slog.Logger

	mu      gosync.Mutex
	started bool
	cancel  context.CancelFunc
	live    *conn
	onReady func(context.Context, startup.Ready)
	first   chan startup.Ready
	// delivered is set once the first ready went to Login or Login gave up waiting for it.
	delivered bool
	// pending is the newest later ready the ready handler has not taken yet; applying is set
	// while the one worker of the handler runs, so readies are applied one at a time and the
	// newest last.
	pending  *startup.Ready
	applying bool
	stopped  chan struct{}
	stopErr  error
	loopDone chan struct{}
	closed   bool
}

// Compile-time check: the port of M2-42.
var _ startup.Control = (*Client)(nil)

// New returns the client over sessions. Errors: ErrNotHTTPS, a missing host id or key, a backoff
// out of range.
func New(cfg Config, sessions Sessions) (*Client, error) {
	u, err := url.Parse(cfg.ServerURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, ErrNotHTTPS
	}
	if cfg.HostID == "" || len(cfg.AuthKey) != ed25519.PrivateKeySize || cfg.BackoffBase <= 0 || cfg.BackoffCap < cfg.BackoffBase {
		return nil, errors.New("control: a host id, the login key and a valid backoff are required")
	}
	if sessions == nil {
		sessions = RefuseSessions{}
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.Real()
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Random == nil {
		cfg.Random = rand.Float64
	}
	if cfg.OwnAddresses == nil {
		cfg.OwnAddresses = OwnAddresses
	}
	roots := cfg.RootCAs
	if roots == nil {
		if roots, err = x509.SystemCertPool(); err != nil {
			roots = x509.NewCertPool()
		}
	}
	ws := url.URL{Scheme: "wss", Host: u.Host, Path: strings.TrimSuffix(u.Path, "/") + Path}
	c := &Client{cfg: cfg, url: ws.String(), sessions: sessions, backoff: NewBackoff(cfg.BackoffBase, cfg.BackoffCap, cfg.Random),
		log: cfg.Log, first: make(chan startup.Ready, 1), stopped: make(chan struct{}), loopDone: make(chan struct{})}
	c.http = &http.Client{Transport: &http.Transport{ // nosemgrep: bothy-go-egress-only -- the host module has no egress package; the control connection dials only the https server of the pairing record (24 3.5)
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		Proxy:           cfg.Proxy,
	}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return c, nil
}

// Keys returns the key store of purpose terminal (M2-45 checks session tokens with it).
func (c *Client) Keys() *KeyStore { return &c.keys }

// SetReadyHandler sets the function every later ready reaches (startup.Service.OnReady): the
// server sends ready again on every change of the declaration (M2-42 Design note 15).
func (c *Client) SetReadyHandler(f func(context.Context, startup.Ready)) {
	c.mu.Lock()
	c.onReady = f
	// A ready that arrived before the handler (Login had given up) is the current declaration.
	c.startApplyLocked()
	c.mu.Unlock()
}

// startApplyLocked starts the worker of the ready handler when a ready waits and none runs;
// c.mu is held.
func (c *Client) startApplyLocked() {
	if c.onReady == nil || c.pending == nil || c.applying {
		return
	}
	c.applying = true
	go c.applyReadies(c.onReady)
}

// applyReadies hands the waiting readies to f one at a time until none waits: a ready that
// arrives while f runs replaces an older waiting one, so the newest declaration is applied last
// (the server sends ready on every change of llm_credentials, 24 7.4).
func (c *Client) applyReadies(f func(context.Context, startup.Ready)) {
	for {
		c.mu.Lock()
		r := c.pending
		c.pending = nil
		if r == nil {
			c.applying = false
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
		f(context.Background(), *r)
	}
}

// Stopped is closed when the connection stopped for good (StopError); Err returns why.
func (c *Client) Stopped() <-chan struct{} { return c.stopped }

// Err returns the StopError of a stopped client, nil otherwise.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopErr
}

// Login implements startup.Control: it starts the connection loop (once) and waits for the first
// ready, at most host_service.reconnect_backoff_cap in total (Design note 12 g of M2-43). Errors:
// startup.ErrUnreachable after that time (the loop keeps trying in the background), the StopError
// of a stop, ErrClosed, the end of ctx.
func (c *Client) Login(ctx context.Context) (startup.Ready, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return startup.Ready{}, ErrClosed
	}
	if !c.started {
		c.started = true
		loop, cancel := context.WithCancel(context.WithoutCancel(ctx))
		c.cancel = cancel
		go c.run(loop)
	}
	c.mu.Unlock()
	t := c.cfg.Clock.NewTimer(c.cfg.BackoffCap)
	defer t.Stop()
	select {
	case r := <-c.first:
		return r, nil
	case <-c.stopped:
		return startup.Ready{}, c.Err()
	case <-t.C():
		// From now on every ready goes to the ready handler: nobody waits for the first one.
		c.mu.Lock()
		gaveUp := !c.delivered
		c.delivered = true
		c.mu.Unlock()
		if !gaveUp {
			if r, ok := <-c.first; ok {
				return r, nil
			}
		}
		return startup.Ready{}, startup.ErrUnreachable
	case <-ctx.Done():
		return startup.Ready{}, ctx.Err()
	}
}

// Report implements startup.Control: r becomes the current report; on a connection after ready it
// is sent at once, otherwise as the first frame after the next ready.
func (c *Client) Report(ctx context.Context, r startup.Report) error {
	c.reports.Set(r)
	c.mu.Lock()
	k := c.live
	c.mu.Unlock()
	if k == nil {
		return nil
	}
	return k.sendReport(ctx)
}

// Close implements startup.Control: the connection closes with 1000 and the loop ends. The close
// frame follows every frame sent before, so a refused start reports before it closes.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	k, cancel, started := c.live, c.cancel, c.started
	c.mu.Unlock()
	if k != nil {
		k.close(CodeNormal)
	}
	if cancel != nil {
		cancel()
	}
	if started {
		<-c.loopDone
	}
	return nil
}

// stop ends the client for good with err.
func (c *Client) stop(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopErr == nil {
		c.stopErr = err
		close(c.stopped)
	}
}

// isClosed reports a client after Close.
func (c *Client) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// Run runs the connection loop until ctx ends, Close or a stop (Design note 12 of M2-43); it
// returns the StopError of a stop, else nil. Login starts it in the background; Run is for callers
// that do not log in through startup.
func (c *Client) Run(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return errors.New("control: the loop already runs")
	}
	c.started = true
	ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()
	c.run(ctx)
	return c.Err()
}

// run is the loop: connect, react to the close, wait, connect again.
func (c *Client) run(ctx context.Context) {
	defer close(c.loopDone)
	for {
		r := c.connect(ctx)
		if ctx.Err() != nil || c.isClosed() {
			return
		}
		var delay time.Duration
		switch r.Kind {
		case Stop:
			c.sessions.EndAll(ctx)
			if r.code == CodeHostVersionUnsupported {
				c.logUpdate(ctx, r.reason)
			}
			c.stop(&StopError{Code: r.code})
			return
		case Soon:
			delay = c.backoff.Soon()
		case Wait:
			delay = r.Delay
		case WaitEndPTYs:
			c.sessions.EndAll(ctx)
			delay = r.Delay
		case Backoff:
			delay = c.backoff.Next()
		}
		t := c.cfg.Clock.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C():
		}
	}
}

// outcome is the end of one connection: the reaction, the close code it came from and, for a
// stop this side decided, the reason of the log line.
type outcome struct {
	Reaction
	code   int
	reason string
}

// logUpdate writes the one line of a refused version (4114 from the server, or no common protocol
// version): the update route of the package kind, because the host service never updates itself
// and the service manager does not restart it after this stop (24 2.2, 18 9.1).
func (c *Client) logUpdate(ctx context.Context, reason string) {
	attrs := []slog.Attr{slog.Int(logCode, CodeHostVersionUnsupported), slog.String(logUpdatePath, c.cfg.UpdatePath)}
	if reason != "" {
		attrs = append(attrs, slog.String(logReason, reason))
	}
	c.log.LogAttrs(ctx, slog.LevelError, msgUpdate, attrs...)
}

// dial opens the WebSocket and returns it with the remote address of the TCP connection.
func (c *Client) dial(ctx context.Context) (*websocket.Conn, net.IP, error) {
	var remote net.IP
	var mu gosync.Mutex
	tr := c.http.Transport.(*http.Transport).Clone()
	dialer := &net.Dialer{} // nosemgrep: bothy-go-egress-only -- the host module has no egress package; this dialer only records the peer address of the control connection (24 2.3)
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		nc, err := dialer.DialContext(ctx, network, addr)
		if err == nil {
			if a, ok := nc.RemoteAddr().(*net.TCPAddr); ok {
				mu.Lock()
				remote = a.IP
				mu.Unlock()
			}
		}
		return nc, err
	}
	hc := &http.Client{Transport: tr, CheckRedirect: c.http.CheckRedirect} // nosemgrep: bothy-go-egress-only -- the transport of New with the address recorded for PeerIsLocal (24 2.3)
	ws, resp, err := websocket.Dial(ctx, c.url, &websocket.DialOptions{HTTPClient: hc, CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	return ws, remote, nil
}

// connect runs one connection to its end (Design note 12 a to f of M2-43).
func (c *Client) connect(ctx context.Context) outcome {
	ws, remote, err := c.dial(ctx)
	if err != nil {
		c.log.LogAttrs(ctx, slog.LevelWarn, "server unreachable")
		return outcome{Reaction: Reaction{Kind: Backoff}}
	}
	ws.SetReadLimit(-1)
	own, _ := c.cfg.OwnAddresses()
	k := &conn{c: c, ws: ws, peerLocal: PeerIsLocal(remote, own), frames: make(chan frameOrEnd, 1), done: make(chan struct{})}
	go k.read(ctx)
	defer c.drop(k)
	defer close(k.done)
	return k.serve(ctx)
}

// attach makes k the live connection.
func (c *Client) attach(k *conn) {
	c.mu.Lock()
	c.live = k
	c.mu.Unlock()
}

// drop forgets k as the live connection.
func (c *Client) drop(k *conn) {
	c.mu.Lock()
	if c.live == k {
		c.live = nil
	}
	c.mu.Unlock()
	k.close(CodeNormal)
}

// deliver hands a ready to Login (the first) or to the ready handler (every later one).
func (c *Client) deliver(r startup.Ready, k *conn) {
	c.mu.Lock()
	c.live = k
	first := !c.delivered
	c.delivered = true
	if first {
		c.mu.Unlock()
		c.first <- r
		return
	}
	c.pending = &r
	c.startApplyLocked()
	c.mu.Unlock()
}
