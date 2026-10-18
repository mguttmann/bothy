// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	authztelemetry "github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	gatewayws "github.com/OWNER/bothy/services/server/internal/gateway/adapters/ws"
	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
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
	// Delivery nil means the delivery that delivers nothing until M1-65.
	Delivery gatewayapp.Delivery
	// Clock nil means clock.System(); Log nil means slog.Default().
	Clock clock.Clock
	Log   *slog.Logger
}

// InstanceLoader loads the instance row (instance.State), the source of the sync epoch.
type InstanceLoader interface {
	Load(ctx context.Context) (instance.Record, error)
}

// Gateway is the gateway of an api process: the use cases with the four ops of
// gateway_ops.enabled registered, and the WebSocket handler of GET /api/v1/gateway.
type Gateway struct {
	App     *gatewayapp.Gateway
	Handler *gatewayws.Handler
}

// NewGateway builds the gateway over d with cfg and registers exactly the ops of
// routes.GatewayOps with their actions (gateway_ops.enabled of api/operations.yaml). Errors: a
// missing collaborator, an invalid value, an enabled op without a handler or an unknown action.
func NewGateway(cfg GatewayConfig, d GatewayDeps) (*Gateway, error) {
	if d.Authenticator == nil || d.Authorizer == nil || d.Nonces == nil || d.Limiter == nil || d.Reads == nil || d.Instance == nil {
		return nil, errors.New("wire: the gateway needs the authenticator, the authorizer, the nonces, the limiter, catch-up and snapshot and the instance")
	}
	if d.Delivery == nil {
		d.Delivery = noDelivery{}
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
	}, gatewayapp.Deps{
		Authenticator: d.Authenticator, Authorizer: d.Authorizer, Nonces: d.Nonces, Limiter: connectLimiter{l: d.Limiter},
		Snapshot: d.Reads.Snapshot, CatchUp: d.Reads.CatchUp, Epoch: epochSource{d.Instance}, Delivery: d.Delivery,
		Decoder: gatewayws.NewDecoder(schemas, check), Alarm: proofAlarm{alarm}, Clock: d.Clock, Log: orDiscard(d.Log),
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
	}, gatewayws.Deps{Gateway: gw, Snapshot: synchttp.SnapshotREST, Clock: d.Clock, Log: orDiscard(d.Log)})
	if err != nil {
		return nil, err
	}
	return &Gateway{App: gw, Handler: h}, nil
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
		names = append(names, op.Schema)
	}
	return httpserver.NewSchemaValidator(rest.Spec, names)
})

// noDelivery is the delivery of M1-64: it registers and attaches nothing and delivers no event.
// M1-65 replaces it with the hub; until then a connection receives no dispatch frames.
type noDelivery struct{}

// Register does nothing.
func (noDelivery) Register(*gatewayapp.Session) {}

// Attach does nothing.
func (noDelivery) Attach(context.Context, *gatewayapp.Session, int64) error { return nil }

// Detach does nothing.
func (noDelivery) Detach(*gatewayapp.Session) {}

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

// Compile-time checks of the adapters.
var (
	_ gatewayapp.Delivery       = noDelivery{}
	_ gatewayapp.ConnectLimiter = connectLimiter{}
)
