// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	authztelemetry "github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	gatewaymetrics "github.com/OWNER/bothy/services/server/internal/gateway/adapters/otelmetrics"
	gatewayws "github.com/OWNER/bothy/services/server/internal/gateway/adapters/ws"
	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/sync"
	synchttp "github.com/OWNER/bothy/services/server/internal/sync/adapters/httpapi"
)

// The route of the gateway (04 section 6.1): one endpoint on the public listener. It is not an
// operation of openapi.yaml (the contract is asyncapi.yaml), so its binding is written here.
const (
	gatewayOperationID = "gatewayConnect"
	gatewayPath        = routes.BaseURL + "/gateway"
	gatewayPattern     = http.MethodGet + " " + gatewayPath
	gatewayClass       = "rl.gateway_connect"
)

// GatewayOperation returns the route of GET /api/v1/gateway for the HTTP chain: action
// gateway.connect, class rl.gateway_connect, marked AuthorizedInBand, because identify and resume
// authenticate and authorize on the connection (R-SY-01); it is the only route with the marker.
func GatewayOperation() httpserver.Operation {
	return httpserver.Operation{ID: gatewayOperationID, Method: http.MethodGet, Path: gatewayPath, Pattern: gatewayPattern,
		Action: string(authorize.ActionGatewayConnect), RateLimitClass: gatewayClass, AuthorizedInBand: true}
}

// GatewayRouteBinding is the binding of the gateway route for the catalogue check of the start
// (AZ-P1): its action and class are checked like those of every REST route.
func GatewayRouteBinding() authorize.RouteBinding {
	op := GatewayOperation()
	return authorize.RouteBinding{OperationID: op.ID, Method: op.Method, Pattern: op.Pattern, Action: op.Action, RateLimitClass: op.RateLimitClass}
}

// GatewayConfig binds the keys of the gateway (M1-64).
type GatewayConfig struct {
	PublicURL          string        `config:"server.public_url"`
	IdentifyTimeout    time.Duration `config:"sync.identify_timeout"`
	HeartbeatInterval  time.Duration `config:"sync.heartbeat_interval"`
	HeartbeatMissLimit int           `config:"sync.heartbeat_miss_limit"`
	ResumeWindow       time.Duration `config:"sync.resume_window"`
	ReauthLeadTime     time.Duration `config:"sync.reauth_lead_time"`
	MaxFrameBytes      int           `config:"sync.ws_max_frame_bytes"`
	OutboundQueueMax   int           `config:"sync.outbound_queue_max"`
	WriteTimeout       time.Duration `config:"gateway.write_timeout"`
	MaxJSONDepth       int           `config:"http.max_json_depth"`
	// The keys of delivery (M1-65).
	CatchUpPageSize     int           `config:"sync.catchup_page_size"`
	CursorAdvanceEvery  int           `config:"sync.cursor_advance_every"`
	LogPollInterval     time.Duration `config:"sync.log_poll_interval"`
	ReconnectBackoffCap time.Duration `config:"database.listen_reconnect_backoff_cap"`
	// The keys of focus, typing and the ephemeral publisher (M1-66).
	MaxFocusedThreads int           `config:"sync.max_focused_threads"`
	TypingMinInterval time.Duration `config:"sync.typing_min_interval"`
	TypingTTL         time.Duration `config:"sync.typing_ttl"`
	PayloadMax        int           `config:"sync.event_payload_max_bytes"`
}

// GatewayConfigFrom binds GatewayConfig from v. The error names the key whose binding failed,
// never a value.
func GatewayConfigFrom(v *config.Values) (GatewayConfig, error) {
	var c GatewayConfig
	if err := config.Bind(v, &c); err != nil {
		return GatewayConfig{}, err
	}
	return c, nil
}

// GatewayDeps are the collaborators of the gateway.
type GatewayDeps struct {
	// Authenticator is identity's Authenticator (M1-34).
	Authenticator actor.Authenticator
	// Authorizer is the Authorizer of the process.
	Authorizer authorize.Authorizer
	// Nonces is the DPoP verifier of identity (its current nonce).
	Nonces gatewayapp.NonceSource
	// Limiter holds the bucket rl.gateway_connect (the limiter of the HTTP chain).
	Limiter *ratelimit.Limiter
	// Reads are catch-up and snapshot of sync (M1-63).
	Reads *SyncReads
	// Instance reads the sync epoch for hello.
	Instance InstanceLoader
	// Threads is the chat port of the threads (chat.Threads) for focus and typing (M1-66).
	Threads gatewayapp.Threads
	// Delivery nil means the hub of M1-65 over the authenticator, the authorizer and catch-up;
	// tests of the connection pass their own.
	Delivery gatewayapp.Delivery
	// Clock nil means clock.System(); Log nil means slog.Default().
	Clock clock.Clock
	Log   *slog.Logger
	// Metrics records the gateway and delivery metrics; nil means gateway/adapters/otelmetrics
	// on the global meter provider of platform/otel (M1-68).
	Metrics gatewayapp.Metrics
}

// InstanceLoader loads the instance row (instance.State), the source of the sync epoch.
type InstanceLoader interface {
	Load(ctx context.Context) (instance.Record, error)
}

// Gateway is the gateway of an api process: the use cases with the ops of gateway_ops.enabled
// registered, the WebSocket handler of GET /api/v1/gateway, the hub that the log tail feeds (nil
// when GatewayDeps.Delivery was given) and the process's kernel/eventlog.EphemeralPublisher, the
// hub (M1-66), which runs, agents and notify use from M2 and M3 on.
type Gateway struct {
	App       *gatewayapp.Gateway
	Handler   *gatewayws.Handler
	Hub       *gatewayapp.Hub
	Publisher eventlog.EphemeralPublisher
}

// The rate-limit classes of the client ops the gateway counts itself (x-rate-limit-class of
// asyncapi.yaml; TestGatewayOpRateClasses): focus is keyed by the principal, typing by the
// connection. identify and resume count rl.gateway_connect in the handshake.
const (
	focusClass  = "rl.read"
	typingClass = "rl.typing"
)

// GatewayOpClasses returns the rate-limit class of each client op of M1-66, for the contract test.
func GatewayOpClasses() map[string]string {
	return map[string]string{"focus": focusClass, "typing.start": typingClass, "typing.stop": typingClass}
}

// NewGateway builds the gateway over d with cfg and registers exactly the ops of
// routes.GatewayOps with their actions (gateway_ops.enabled of api/operations.yaml). Errors: a
// missing collaborator, an invalid value, an enabled op without a handler or an unknown action.
func NewGateway(cfg GatewayConfig, d GatewayDeps) (*Gateway, error) {
	if d.Authenticator == nil || d.Authorizer == nil || d.Nonces == nil || d.Limiter == nil || d.Reads == nil || d.Instance == nil ||
		d.Threads == nil {
		return nil, errors.New("wire: the gateway needs the authenticator, the authorizer, the nonces, the limiter, catch-up and snapshot, " +
			"the instance and the threads of chat")
	}
	hub, err := newMeasuredHub(cfg, &d)
	if err != nil {
		return nil, err
	}
	var publisher eventlog.EphemeralPublisher = noPublisher{}
	if hub != nil {
		publisher = hub
	}
	htu, err := gatewayws.HTU(cfg.PublicURL)
	if err != nil {
		return nil, err
	}
	ops := routes.GatewayOps()
	schemas := map[string]string{}
	for _, op := range ops {
		schemas[op.Op] = op.Schema
	}
	check, err := frameValidator()
	if err != nil {
		return nil, err
	}
	alarm, err := authztelemetry.NewErrorCounter(nil)
	if err != nil {
		return nil, err
	}
	gw, err := gatewayapp.New(gatewayapp.Config{
		HTU: htu, IdentifyTimeout: cfg.IdentifyTimeout, HeartbeatInterval: cfg.HeartbeatInterval, HeartbeatMissLimit: cfg.HeartbeatMissLimit,
		ResumeWindow: cfg.ResumeWindow, ReauthLeadTime: cfg.ReauthLeadTime, ProtocolVersions: routes.GatewayProtocolVersions(),
		MaxFocusedThreads: cfg.MaxFocusedThreads, TypingMinInterval: cfg.TypingMinInterval, TypingTTL: cfg.TypingTTL,
	}, gatewayapp.Deps{
		Authenticator: d.Authenticator, Authorizer: d.Authorizer, Nonces: d.Nonces, Limiter: connectLimiter{l: d.Limiter},
		Snapshot: d.Reads.Snapshot, CatchUp: d.Reads.CatchUp, Epoch: epochSource{d.Instance}, Delivery: d.Delivery,
		Decoder: gatewayws.NewDecoder(schemas, check), Alarm: proofAlarm{alarm}, Clock: d.Clock, Log: orDiscard(d.Log),
		Threads: d.Threads, OpLimiter: opLimiter{l: d.Limiter}, Publisher: publisher, Metrics: d.Metrics,
	})
	if err != nil {
		return nil, err
	}
	for _, op := range ops {
		action, ok := authorize.ParseAction(op.Action)
		if !ok {
			return nil, errors.New("wire: the gateway op " + op.Op + " names an unknown action")
		}
		if err := gw.Register(op.Op, action); err != nil {
			return nil, err
		}
	}
	h, err := gatewayws.NewHandler(gatewayws.Config{
		PublicURL: cfg.PublicURL, MaxFrameBytes: cfg.MaxFrameBytes, MaxJSONDepth: cfg.MaxJSONDepth,
		OutboundQueueMax: cfg.OutboundQueueMax, WriteTimeout: cfg.WriteTimeout,
	}, gatewayws.Deps{Gateway: gw, Snapshot: synchttp.SnapshotREST, Clock: d.Clock, Log: orDiscard(d.Log), Metrics: d.Metrics})
	if err != nil {
		return nil, err
	}
	return &Gateway{App: gw, Handler: h, Hub: hub, Publisher: publisher}, nil
}

// newMeasuredHub sets the gateway metrics of d (adapters/otelmetrics on the global meter
// provider when d brings none) and builds the hub (newHub) with bothy.event.fanout.lag over it;
// the registration of the gauge lives as long as the process (M1-68).
func newMeasuredHub(cfg GatewayConfig, d *GatewayDeps) (*gatewayapp.Hub, error) {
	if d.Metrics == nil {
		m, err := gatewaymetrics.New(nil)
		if err != nil {
			return nil, err
		}
		d.Metrics = m
	}
	hub, err := newHub(cfg, d)
	if err != nil || hub == nil {
		return hub, err
	}
	if _, err := gatewaymetrics.RegisterFanoutLag(nil, hub.FanoutLag); err != nil {
		return nil, err
	}
	return hub, nil
}

// newHub builds the hub of M1-65 and makes it the delivery of d when d brings none; otherwise it
// returns nil (tests of the connection pass their own delivery).
func newHub(cfg GatewayConfig, d *GatewayDeps) (*gatewayapp.Hub, error) {
	if d.Delivery != nil {
		return nil, nil
	}
	h, err := gatewayapp.NewHub(gatewayapp.HubConfig{
		QueueMax: cfg.OutboundQueueMax, PageSize: cfg.CatchUpPageSize, CursorEvery: cfg.CursorAdvanceEvery,
		RetryBase: cfg.LogPollInterval, RetryCap: cfg.ReconnectBackoffCap, PayloadMax: cfg.PayloadMax,
	}, gatewayapp.HubDeps{Authenticator: d.Authenticator, Authorizer: d.Authorizer, CatchUp: d.Reads.CatchUp, Log: orDiscard(d.Log),
		Clock: d.Clock, Metrics: d.Metrics})
	if err != nil {
		return nil, err
	}
	d.Delivery = h
	return h, nil
}

// Mount mounts the gateway route on mux through chain (only the outer steps run for it, see
// httpserver.Operation.AuthorizedInBand).
func (g *Gateway) Mount(mux Mux, chain *httpserver.Chain) {
	mux.Handle(gatewayPattern, chain.Wrap(GatewayOperation(), g.Handler))
}

// StartSweeper starts the sweeper of the gateway sessions and returns its stop step, which ends
// it in the HTTP phase.
func (g *Gateway) StartSweeper() lifecycle.StopStep {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		g.App.Sessions().Run(ctx)
	}()
	return lifecycle.StopStep{Name: "gateway_sessions", Phase: lifecycle.PhaseHTTP, Run: func(stopCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
	}}
}

// frameValidator compiles the schemas of the enabled gateway ops once per process, like the
// request validator (enabledValidator).
var frameValidator = stdsync.OnceValues(func() (*httpserver.SchemaValidator, error) {
	var names []string
	for _, op := range routes.GatewayOps() {
		// typing.start and typing.stop share GatewayTyping; the validator takes each name once.
		if !slices.Contains(names, op.Schema) {
			names = append(names, op.Schema)
		}
	}
	return httpserver.NewSchemaValidator(rest.Spec, names)
})

// StopConnections is the stop step of PhaseConnections (02 "Shutdown" step 3): every connection
// closes with 4013 and new identify and resume are refused, within the rest of
// shutdown.total_timeout.
func (g *Gateway) StopConnections() lifecycle.StopStep {
	return lifecycle.StopStep{Name: "gateway_connections", Phase: lifecycle.PhaseConnections, Run: g.App.Shutdown}
}

// DeliverySink returns the sink of the log tail of an api process (02 "Log tail per api
// process"): every event goes to Authorizer.Observe and then to the hub, in seq order and in the
// goroutine of the tail. hub nil observes only.
//
// Before a batch it observes the committed rights version read after the batch (an event without
// type). An event carries the version its transaction read before commit, so a rights change
// without an event (the revocation of an API token) that commits between that read and the commit
// of the event is not in the event's version; the version read here is, because the batch was
// read after its events committed (M1-65/14). A failed read only skips that step.
//
// Before the batch reaches the hub, the hub learns that the tail knows its events (Hub.Known), the
// start of bothy.event.fanout.lag.
func DeliverySink(az authorize.Authorizer, hub *gatewayapp.Hub, versions instance.Versions) sync.Sink {
	return func(ctx context.Context, batch []sync.Event) error {
		if hub != nil && len(batch) > 0 {
			hub.Known(batch[len(batch)-1].Seq)
		}
		if len(batch) > 0 && versions != nil {
			if pv, err := versions.PermissionVersion(ctx); err == nil {
				az.Observe(ctx, authorize.LogEvent{Seq: batch[0].Seq, PermissionVersion: pv})
			}
		}
		for _, ev := range batch {
			az.Observe(ctx, authorize.LogEvent{Seq: ev.Seq, Type: ev.Type, PermissionVersion: ev.PermissionVersion, Audience: ev.AudiencePrincipalID})
			if hub != nil {
				hub.Dispatch(ctx, ev)
			}
		}
		return nil
	}
}

// connectLimiter adapts the limiter to the class rl.gateway_connect, keyed by device or token.
type connectLimiter struct{ l *ratelimit.Limiter }

// AllowConnect implements gatewayapp.ConnectLimiter.
func (c connectLimiter) AllowConnect(a actor.Actor) (bool, int, error) {
	key, err := ratelimit.DeviceOrTokenKey(a)
	if err != nil {
		return false, 0, err
	}
	d, err := c.l.Allow(gatewayClass, key)
	if err != nil {
		return false, 0, err
	}
	return d.Allowed, d.RetryAfterSeconds, nil
}

// opLimiter adapts the limiter to the classes of focus and typing (GatewayOpClasses).
type opLimiter struct{ l *ratelimit.Limiter }

// AllowRead implements gatewayapp.OpLimiter: rl.read keyed by the principal.
func (o opLimiter) AllowRead(principal ids.ID) (bool, error) {
	d, err := o.l.Allow(focusClass, ratelimit.KeyForPrincipal(principal))
	return d.Allowed, err
}

// AllowTyping implements gatewayapp.OpLimiter: rl.typing keyed by the connection.
func (o opLimiter) AllowTyping(conn ids.ID) (bool, error) {
	d, err := o.l.Allow(typingClass, ratelimit.KeyForConnection(conn.String()))
	return d.Allowed, err
}

// noPublisher is the publisher of a gateway whose tests bring their own delivery: it relays
// nothing.
type noPublisher struct{}

// PublishEphemeral implements eventlog.EphemeralPublisher.
func (noPublisher) PublishEphemeral(context.Context, eventlog.EphemeralEvent) error { return nil }

// epochSource reads sync_epoch of the instance row.
type epochSource struct{ i InstanceLoader }

// SyncEpoch implements gatewayapp.EpochSource.
func (e epochSource) SyncEpoch(ctx context.Context) (ids.ID, error) {
	rec, err := e.i.Load(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	return rec.SyncEpoch, nil
}

// proofAlarm counts an op without authorization check in bothy.authz.errors.
type proofAlarm struct{ c *authztelemetry.ErrorCounter }

// NotChecked implements gatewayapp.ProofAlarm.
func (p proofAlarm) NotChecked(ctx context.Context, _ string, _ authorize.Action) {
	p.c.CountError(ctx, errs.CauseAuthzNotChecked)
}

// Compile-time check of the adapters.
var (
	_ gatewayapp.ConnectLimiter = connectLimiter{}
	_ gatewayapp.OpLimiter      = opLimiter{}
)
