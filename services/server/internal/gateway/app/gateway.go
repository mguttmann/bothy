// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	gosync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Log keys of the gateway; constants for sloglint, names of the signals catalogue.
const (
	logKeyAction    = "bothy.action"
	logKeyReason    = "bothy.reason"
	logKeyErrorType = "error.type"
)

// Config holds the configuration keys of the gateway use cases.
type Config struct {
	// HTU is the htu every proof must name: the HTTPS form of the origin of server.public_url
	// plus /api/v1/gateway, also behind TLS termination (04 section 6.3 step 3).
	HTU string
	// IdentifyTimeout is sync.identify_timeout.
	IdentifyTimeout time.Duration
	// HeartbeatInterval is sync.heartbeat_interval.
	HeartbeatInterval time.Duration
	// HeartbeatMissLimit is sync.heartbeat_miss_limit.
	HeartbeatMissLimit int
	// ResumeWindow is sync.resume_window.
	ResumeWindow time.Duration
	// ReauthLeadTime is sync.reauth_lead_time.
	ReauthLeadTime time.Duration
	// ProtocolVersions are the protocol versions this server speaks (info.x-protocol-versions).
	ProtocolVersions []int
	// MaxFocusedThreads is sync.max_focused_threads.
	MaxFocusedThreads int
	// TypingMinInterval is sync.typing_min_interval.
	TypingMinInterval time.Duration
	// TypingTTL is sync.typing_ttl.
	TypingTTL time.Duration
}

// validate names every invalid value.
func (c Config) validate() error {
	var problems []error
	if c.HTU == "" {
		problems = append(problems, errors.New("the gateway htu is empty"))
	}
	for name, d := range map[string]time.Duration{
		"sync.identify_timeout": c.IdentifyTimeout, "sync.heartbeat_interval": c.HeartbeatInterval,
		"sync.resume_window": c.ResumeWindow, "sync.reauth_lead_time": c.ReauthLeadTime,
		"sync.typing_min_interval": c.TypingMinInterval, "sync.typing_ttl": c.TypingTTL,
	} {
		if d <= 0 {
			problems = append(problems, fmt.Errorf("%s must be positive", name))
		}
	}
	if c.MaxFocusedThreads < 1 {
		problems = append(problems, errors.New("sync.max_focused_threads must be positive"))
	}
	if c.HeartbeatMissLimit < 0 {
		problems = append(problems, errors.New("sync.heartbeat_miss_limit must not be negative"))
	}
	if len(c.ProtocolVersions) == 0 {
		problems = append(problems, errors.New("no protocol version"))
	}
	return errors.Join(problems...)
}

// Deps are the collaborators of the gateway.
type Deps struct {
	// Authenticator verifies token and proof of identify, resume and reauth (identity, M1-34).
	Authenticator actor.Authenticator
	// Authorizer decides gateway.connect.
	Authorizer authorize.Authorizer
	// Nonces is the current DPoP nonce of identity.
	Nonces NonceSource
	// Limiter is rl.gateway_connect.
	Limiter ConnectLimiter
	// Snapshot builds the snapshot of ready (sync, M1-63).
	Snapshot sync.Snapshot
	// CatchUp checks the cursor of identify and resume (sync, M1-63).
	CatchUp sync.CatchUp
	// Epoch reads the sync epoch for hello.
	Epoch EpochSource
	// Delivery registers and attaches sessions (M1-65; a no-op until then).
	Delivery Delivery
	// Decoder checks and decodes the bodies of client ops (adapters/ws).
	Decoder FrameDecoder
	// Alarm counts handlers without an authorization check.
	Alarm ProofAlarm
	// Threads reads the space and channel of a thread (chat, M1-55) for focus and typing.
	Threads Threads
	// OpLimiter holds rl.read and rl.typing of focus and typing.
	OpLimiter OpLimiter
	// Publisher relays typing.started (the hub of this process, M1-66).
	Publisher eventlog.EphemeralPublisher
	// Consumers is the delivery engine of agent consumers (M2-07); nil leaves agent sessions on
	// the delivery of people (tests of the connection) and registers no handler of ack.
	Consumers *ConsumerEngine
	// AgentLimiter is rl.agent_runtime of the ack op and of the member agent of heartbeat; nil
	// applies no limit (tests).
	AgentLimiter AgentLimiter
	// Presence takes the member agent of heartbeat (agents, M2-20); nil leaves the member without
	// effect (tests of the connection).
	Presence agents.Presence
	// Autocomplete takes the answers of command.autocomplete_result (runs.CommandExec, M2-17); nil
	// registers no handler of the op.
	Autocomplete AutocompleteAnswerer
	// Clock is the time of deadlines and frames; nil means clock.System().
	Clock clock.Clock
	// Log receives the errors of ops; nil means slog.Default(). It never receives tokens,
	// proofs, nonces or snapshots.
	Log *slog.Logger
	// Metrics records the gateway metrics of connection setup and end (M1-68); nil records
	// nothing.
	Metrics Metrics
}

// Gateway holds the use cases of the WebSocket gateway (docs/spec/04-sync-protocol.md section
// 6): hello, identify, resume, heartbeat, reauth, the deadlines of a connection and the session
// store. It is safe for concurrent use.
type Gateway struct {
	cfg      Config
	d        Deps
	sessions *Sessions
	ops      *Dispatcher
	handlers map[string]OpHandler

	// mu guards the open connections and the drain state of the shutdown (shutdown.go).
	mu       gosync.Mutex
	open     map[*Connection]struct{}
	draining bool
	gone     chan struct{}
}

// New returns the gateway over d with the configuration cfg. Errors: an invalid value of cfg or
// a missing collaborator.
func New(cfg Config, d Deps) (*Gateway, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("gateway: %w", err)
	}
	if d.Authenticator == nil || d.Authorizer == nil || d.Nonces == nil || d.Limiter == nil || d.Snapshot == nil || d.CatchUp == nil ||
		d.Epoch == nil || d.Delivery == nil || d.Decoder == nil || d.Alarm == nil || d.Threads == nil || d.OpLimiter == nil || d.Publisher == nil {
		return nil, errors.New("gateway: New needs every collaborator")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	d.Metrics = orNop(d.Metrics)
	cfg.ProtocolVersions = slices.Clone(cfg.ProtocolVersions)
	g := &Gateway{cfg: cfg, d: d, sessions: newSessions(cfg.ResumeWindow, d.Clock), ops: newDispatcher(d.Decoder, d.Alarm, d.Log),
		open: map[*Connection]struct{}{}, gone: make(chan struct{})}
	g.handlers = map[string]OpHandler{opIdentify: g.identify, opResume: g.resume, "heartbeat": g.heartbeat, "reauth": g.reauth,
		opFocus: g.focus, opTypingStart: g.typingStart, opTypingStop: g.typingStop}
	if d.Consumers != nil {
		g.handlers["ack"] = g.ack
	}
	if d.Autocomplete != nil {
		g.handlers[opAutocompleteResult] = g.autocompleteResult
	}
	return g, nil
}

// Register registers the gateway's own handler of op with action in the dispatcher. Errors: an
// op the gateway has no handler for (its ticket has not built it yet), or a second registration.
// internal/wire calls it for every op of routes.GatewayOps with the action of its x-authz-action.
func (g *Gateway) Register(op string, action authorize.Action) error {
	h, ok := g.handlers[op]
	if !ok {
		return fmt.Errorf("gateway: no handler for the op %q", op)
	}
	return g.ops.Register(op, action, h)
}

// Dispatcher returns the op registry, for handlers of later tickets.
func (g *Gateway) Dispatcher() *Dispatcher { return g.ops }

// Sessions returns the session store; the lifecycle runs its sweeper.
func (g *Gateway) Sessions() *Sessions { return g.sessions }

// SessionOf returns the session of c, or nil before identify or resume.
func (g *Gateway) SessionOf(c *Connection) *Session {
	s, _ := c.current()
	return s
}

// Open starts a connection on conn right after the upgrade and sends hello (04 section 6.3 step
// 2): heartbeat interval and resume window, protocol versions, the current nonce, the server time
// and the sync epoch. Errors: the epoch cannot be read; the caller closes with 4000. Both refusals
// (that one and a draining gateway, 4013) are recorded as a connect with outcome server_error and
// as their close.
func (g *Gateway) Open(ctx context.Context, conn Conn) (*Connection, error) {
	epoch, err := g.d.Epoch.SyncEpoch(ctx)
	if err != nil {
		g.refused(ctx, domain.CloseUnknownError)
		return nil, err
	}
	now := g.d.Clock.Now()
	c := &Connection{id: ids.New(), conn: conn, opened: now, state: domain.StateAwaitingIdentify}
	if !g.track(c) {
		conn.Close(domain.CloseServerRestart, domain.CloseServerRestart.Name())
		c.mu.Lock()
		c.finished = true
		c.mu.Unlock()
		g.refused(ctx, domain.CloseServerRestart)
		return c, errDraining
	}
	err = conn.Send(Hello{
		HeartbeatInterval: g.cfg.HeartbeatInterval, ResumeWindow: g.cfg.ResumeWindow, ProtocolVersions: slices.Clone(g.cfg.ProtocolVersions),
		DPoPNonce: g.d.Nonces.CurrentNonce(), ServerTime: now, SyncEpoch: epoch,
	})
	return c, err
}

// Handle dispatches the client op op with the raw body d on c (Dispatcher.Dispatch).
// While the frame is handled, Tick closes for no identify or heartbeat deadline: the frame
// arrived before the deadline it may meet (docs/DECISIONS.md M1-64/15).
func (g *Gateway) Handle(ctx context.Context, c *Connection, op string, d json.RawMessage) {
	c.mu.Lock()
	c.handling++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.handling--
		c.mu.Unlock()
	}()
	g.ops.Dispatch(ctx, c, op, d)
}

// Tick applies the deadlines of c at now and returns the instant it wants the next tick, zero
// once the connection is closed: 4003 without identify or resume within sync.identify_timeout;
// 4009 at the heartbeat deadline (domain.HeartbeatDeadline); 4011 at the expiry of the
// credential; for people one reauth_required at sync.reauth_lead_time before it, with the
// current nonce. Agents and apps get no reauth_required.
func (g *Gateway) Tick(_ context.Context, c *Connection, now time.Time) time.Time {
	c.mu.Lock()
	var t tick
	switch c.state {
	case domain.StateAwaitingIdentify:
		t = g.tickAwaiting(c, now)
	case domain.StateReady:
		t = g.tickReady(c, now)
	case domain.StateDetached, domain.StateClosed:
		c.mu.Unlock()
		return time.Time{}
	}
	if c.handling > 0 && (t.close == domain.CloseNotIdentified || t.close == domain.CloseHeartbeatTimeout) {
		// The adapter ticks again after the frame (poke); this instant only bounds the wait.
		t = tick{next: now.Add(g.cfg.IdentifyTimeout)}
	}
	c.mu.Unlock()
	if t.close != 0 {
		c.Close(t.close, "")
		return time.Time{}
	}
	if t.reauth != nil {
		_ = c.conn.Send(*t.reauth)
	}
	return t.next
}

// tick is the result of one deadline check: a close, a reauth_required to send, the next instant.
type tick struct {
	close  domain.CloseCode
	reauth *ReauthRequired
	next   time.Time
}

// tickAwaiting is the deadline of a connection before identify; c.mu is held.
func (g *Gateway) tickAwaiting(c *Connection, now time.Time) tick {
	deadline := c.opened.Add(g.cfg.IdentifyTimeout)
	if !now.Before(deadline) {
		return tick{close: domain.CloseNotIdentified}
	}
	return tick{next: deadline}
}

// tickReady are the deadlines of a ready connection; c.mu is held.
func (g *Gateway) tickReady(c *Connection, now time.Time) tick {
	beat := domain.HeartbeatDeadline(c.lastBeat, g.cfg.HeartbeatInterval, g.cfg.HeartbeatMissLimit)
	// The expiry goes first: when both have passed, 4011 lets the client renew its token instead
	// of reconnecting with the expired one, and Tick keeps it while a frame is in flight.
	switch {
	case !c.expiresAt.IsZero() && !now.Before(c.expiresAt):
		return tick{close: domain.CloseTokenExpired}
	case !now.Before(beat):
		return tick{close: domain.CloseHeartbeatTimeout}
	case c.expiresAt.IsZero():
		return tick{next: beat}
	}
	t := tick{next: earliest(beat, c.expiresAt)}
	if c.actor.Kind != actor.KindUser || c.reauthSent {
		return t
	}
	at := domain.ReauthAt(c.expiresAt, g.cfg.ReauthLeadTime)
	if now.Before(at) {
		t.next = earliest(t.next, at)
		return t
	}
	c.reauthSent = true
	t.reauth = &ReauthRequired{ExpiresAt: c.expiresAt, DPoPNonce: g.d.Nonces.CurrentNonce()}
	return t
}

// earliest returns the earlier of a and b.
func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// resumableEnds are the close codes after which the session stays resumable: the client is told
// to reconnect and try resume, or the connection simply went away (0).
var resumableEnds = map[domain.CloseCode]bool{
	0: true, domain.CloseNormal: true, domain.CloseGoingAway: true, domain.CloseUnknownError: true, domain.CloseUnknownOp: true,
	domain.CloseDecodeError: true, domain.CloseHeartbeatTimeout: true, domain.CloseTokenExpired: true,
	domain.CloseServerRestart: true, domain.CloseSlowConsumer: true,
}

// Closed ends c after its connection is gone: the connection leaves delivery, and its session stays
// resumable for sync.resume_window after a resumable end; after 4004, 4012, 4016, 4017, 4018 and
// the other final ends it is removed. A session that a newer connection took over is left alone.
// The end is recorded once (finished): the connect before ready with its outcome, the -1 of a
// connection counted as active, and the close code of the server or else of the client; a
// connection that ended without a close frame in either direction counts no close.
func (g *Gateway) Closed(c *Connection) {
	now := g.d.Clock.Now()
	c.mu.Lock()
	s, code := c.session, c.code
	c.state = domain.StateClosed
	end := c.finished
	c.finished = true
	ready, active, peer, kind := !c.readyAt.IsZero(), c.activeKind, c.peerCode, string(c.actor.Kind)
	c.mu.Unlock()
	if !end {
		g.recordEnd(now.Sub(c.opened), code, peer, ready, active, kind)
	}
	g.untrack(c)
	if s == nil {
		return
	}
	g.d.Delivery.Detach(c)
	g.sessions.release(s, c, resumableEnds[code])
}

// replace closes every connection of old: with 4015 for the session of an agent consumer (another
// connection took the consumer over; agent connections have no device, brief M2-07 note 4), else
// with 4017 (one connection per device, 04 section 6.1).
func (g *Gateway) replace(s *Session, old []*Connection) {
	code := domain.CloseDeviceReplaced
	if g.consumerSession(s.PrincipalKind) {
		code = domain.CloseConsumerReplaced
	}
	for _, o := range old {
		o.Close(code, "")
	}
}

// consumerSession reports whether a session of kind is an agent consumer: its durable cursor
// decides where delivery starts, so the client cursor and epoch are ignored (brief M2-07 note 4).
func (g *Gateway) consumerSession(kind actor.Kind) bool {
	return kind == actor.KindAgent && g.d.Consumers != nil
}

// attach is note 12 of the brief: with a cursor, catch-up decides; a reason sends
// resync_required and delivery starts at head (the connection stays live from there, 04 section
// 7.3); otherwise delivery starts at the cursor; without a cursor at head.
func (g *Gateway) attach(ctx context.Context, c *Connection, cursor *int64, epoch *ids.ID, head int64) error {
	from := head
	if _, a := c.current(); g.consumerSession(a.Kind) {
		cursor = nil
	}
	if cursor != nil {
		res, err := g.d.CatchUp.Check(ctx, sync.CheckInput{After: *cursor, Epoch: epoch})
		if err != nil {
			return err
		}
		if res.Reason != "" {
			if c.conn.Send(ResyncRequired{Reason: res.Reason, HeadSeq: head, SyncEpoch: res.SyncEpoch}) == nil {
				g.resynced(ctx, c, string(res.Reason))
			}
		} else {
			from = *cursor
		}
	}
	return g.d.Delivery.Attach(ctx, c, from, head)
}

// recordEnd records the end of a connection that lived for d (see Closed).
func (g *Gateway) recordEnd(d time.Duration, code domain.CloseCode, peer int, ready bool, active, kind string) {
	ctx := context.Background()
	if !ready {
		if kind == "" {
			kind = KindAnonymous
		}
		g.d.Metrics.Connected(ctx, d, domain.Outcome(code, false), kind)
	}
	if active != "" {
		g.d.Metrics.ConnectionClosed(ctx, active)
	}
	switch {
	case code != 0:
		g.d.Metrics.Closed(ctx, int(code))
	case peer != 0:
		g.d.Metrics.Closed(ctx, peer)
	}
}

// refused records a connection Open refused with code: a connect with outcome server_error before
// identification and its close.
func (g *Gateway) refused(ctx context.Context, code domain.CloseCode) {
	g.d.Metrics.Connected(ctx, 0, domain.OutcomeServerError, KindAnonymous)
	g.d.Metrics.Closed(ctx, int(code))
}

// connected records that c reached ready or resumed as kind at the first call: the connect with
// outcome ok, the start of its catch-up and +1 active connection.
func (g *Gateway) connected(ctx context.Context, c *Connection, kind string) {
	now := g.d.Clock.Now()
	c.mu.Lock()
	if !c.readyAt.IsZero() {
		c.mu.Unlock()
		return
	}
	c.readyAt, c.activeKind = now, kind
	c.mu.Unlock()
	g.d.Metrics.Connected(ctx, now.Sub(c.opened), domain.OutcomeOK, kind)
	g.d.Metrics.ConnectionOpened(ctx, kind)
}

// resynced records a resync_required with reason sent on c: its reason, and the end of the
// catch-up with outcome resync_required.
func (g *Gateway) resynced(ctx context.Context, c *Connection, reason string) {
	g.d.Metrics.Resync(ctx, reason)
	g.d.Metrics.CatchupDone(ctx, g.d.Clock.Now().Sub(c.readyTime()), OutcomeResyncRequired)
}
