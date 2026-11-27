// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/wsrelay"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The two upgrade routes of the terminal sessions (M2-44, asyncapi channels terminalStream and
// hostData) on the public listener of the roles api and all. Like the control connection they are
// no operations of openapi.yaml; each authorizes on the connection (terminal.stream after
// identify, host.connect at the login of the host service), so both are marked
// AuthorizedInBand. No listener and no port is added (R-TM-01).
const (
	terminalStreamOperationID = "terminalStream"
	terminalStreamPath        = "/api/v1/terminal/sessions/{session_id}/stream"
	hostDataOperationID       = "hostData"
	hostDataPath              = "/api/v1/hosts/sessions/{session_id}/stream"
	terminalStreamClass       = "rl.terminal"
	// The two upgrade routes of computer sessions (CU-04, asyncapi computerAttach and
	// hostComputerData): computer.attach authorizes on the connection after identify, the host
	// data connection logs in with host.connect.
	computerAttachOperationID = "computerAttach"
	computerAttachPath        = "/api/v1/computer/sessions/{session_id}/attach"
	hostComputerDataID        = "hostComputerData"
	hostComputerDataPath      = "/api/v1/hosts/computer-sessions/{session_id}/stream"
	computerAttachClass       = "rl.agent_runtime"
)

// ComputerAttachOperation returns the route of the attach connection of bothy-computer-mcp for the
// HTTP chain: action computer.attach, class rl.agent_runtime, marked AuthorizedInBand (CU-04).
func ComputerAttachOperation() httpserver.Operation {
	return httpserver.Operation{ID: computerAttachOperationID, Method: http.MethodGet, Path: computerAttachPath,
		Pattern: http.MethodGet + " " + computerAttachPath, Action: string(authorize.ActionComputerAttach),
		RateLimitClass: computerAttachClass, AuthorizedInBand: true}
}

// HostComputerDataOperation returns the route of the computer data connection of the host service
// for the HTTP chain: action host.connect, class rl.host, marked AuthorizedInBand (CU-04).
func HostComputerDataOperation() httpserver.Operation {
	return httpserver.Operation{ID: hostComputerDataID, Method: http.MethodGet, Path: hostComputerDataPath,
		Pattern: http.MethodGet + " " + hostComputerDataPath, Action: string(authorize.ActionHostConnect),
		RateLimitClass: hostConnectClass, AuthorizedInBand: true}
}

// TerminalStreamOperation returns the route of the device stream for the HTTP chain: action
// terminal.stream, class rl.terminal, marked AuthorizedInBand.
func TerminalStreamOperation() httpserver.Operation {
	return httpserver.Operation{ID: terminalStreamOperationID, Method: http.MethodGet, Path: terminalStreamPath,
		Pattern: http.MethodGet + " " + terminalStreamPath, Action: string(authorize.ActionTerminalStream),
		RateLimitClass: terminalStreamClass, AuthorizedInBand: true}
}

// HostDataOperation returns the route of the data connection for the HTTP chain: action
// host.connect, class rl.host, marked AuthorizedInBand.
func HostDataOperation() httpserver.Operation {
	return httpserver.Operation{ID: hostDataOperationID, Method: http.MethodGet, Path: hostDataPath,
		Pattern: http.MethodGet + " " + hostDataPath, Action: string(authorize.ActionHostConnect),
		RateLimitClass: hostConnectClass, AuthorizedInBand: true}
}

// TerminalStreamRouteBindings are the bindings of both routes for the catalogue check of the start
// (AZ-P1).
func TerminalStreamRouteBindings() []authorize.RouteBinding {
	var out []authorize.RouteBinding
	for _, op := range []httpserver.Operation{TerminalStreamOperation(), HostDataOperation(), ComputerAttachOperation(), HostComputerDataOperation()} {
		out = append(out, authorize.RouteBinding{OperationID: op.ID, Method: op.Method, Pattern: op.Pattern, Action: op.Action,
			RateLimitClass: op.RateLimitClass})
	}
	return out
}

// TerminalStreamsConfig binds the keys of the relay and the device stream (M2-44).
type TerminalStreamsConfig struct {
	PublicURL   string        `config:"server.public_url"`
	OpenTimeout time.Duration `config:"terminal.open_timeout"`
	// The limits the relay enforces without the host service (M2-46 Design note 7).
	IdleTimeout        time.Duration `config:"terminal.idle_timeout"`
	IdleGrace          time.Duration `config:"terminal.idle_grace"`
	MaxSessionDuration time.Duration `config:"terminal.max_session_duration"`
	DetachGrace        time.Duration `config:"terminal.detach_grace"`
	// The windows of a session in the owner account mode (D-TSL-1).
	OwnerAccountDetachGrace        time.Duration `config:"terminal.owner_account_detach_grace"`
	OwnerAccountMaxSessionDuration time.Duration `config:"terminal.owner_account_max_session_duration"`
	BufferMaxBytes                 int           `config:"terminal.relay_buffer_max_bytes"`
	StallTimeout                   time.Duration `config:"terminal.relay_stall_timeout"`
	MaxFrameBytes                  int           `config:"sync.ws_max_frame_bytes"`
	MaxJSONDepth                   int           `config:"http.max_json_depth"`
	// ComputerOpenTimeout is computer.open_timeout and WriteTimeout gateway.write_timeout, the
	// deadlines of the computer relay (CU-04).
	ComputerOpenTimeout time.Duration `config:"computer.open_timeout"`
	WriteTimeout        time.Duration `config:"gateway.write_timeout"`
}

// TerminalStreamsConfigFrom binds TerminalStreamsConfig from v; the error names the key, never a
// value.
func TerminalStreamsConfigFrom(v *config.Values) (TerminalStreamsConfig, error) {
	var c TerminalStreamsConfig
	if err := config.Bind(v, &c); err != nil {
		return TerminalStreamsConfig{}, err
	}
	return c, nil
}

// TerminalStreamsDeps are the collaborators of the streams.
type TerminalStreamsDeps struct {
	// Authenticator verifies the access token and proof of identify (identity, M1-34).
	Authenticator actor.Authenticator
	// Nonces gives the DPoP nonce of hello (identity.DPoP).
	Nonces wsrelay.Nonces
	// Signer signs the session tokens with the key of purpose terminal (instance.Keys).
	Signer terminalapp.TokenSigner
	// Timers drives the deadlines of the relay; nil means the real timers.
	Timers terminalapp.Timers
	// Metrics records bothy.terminal.relay.unpaired; nil creates the OpenTelemetry adapter on the
	// global provider.
	Metrics terminalapp.Metrics
	// Clock gives the times of the relay and of hello; nil means clock.System().
	Clock clock.Clock
	// Log receives the relay records and failures without content; nil means slog.Default().
	Log *slog.Logger
	// Multi is cluster.mode multi (M2-47, wsrelay.RelayConfig.Multi).
	Multi bool
	// Limiter applies rl.terminal and rl.public_auth on the device stream (M2-46); nil allows
	// everything (wsrelay.NoLimiter).
	Limiter wsrelay.Limiter
}

// TerminalStreams are the relay of this process with its two handlers.
type TerminalStreams struct {
	Relay  *wsrelay.Relay
	Stream *wsrelay.StreamHandler
	Data   *wsrelay.DataHandler
	// Computer is the relay of computer sessions with its two handlers (CU-04).
	Computer     *wsrelay.ComputerRelay
	Attach       *wsrelay.AttachHandler
	ComputerData *wsrelay.ComputerDataHandler
}

// attachSchemas compiles the schema of identify of the attach connection once per process.
var attachSchemas = stdsync.OnceValues(func() (*httpserver.SchemaValidator, error) {
	return httpserver.NewSchemaValidator(rest.Spec, wsrelay.AttachSchemaNames())
})

// streamSchemas compiles the schema of identify of the device stream once per process.
var streamSchemas = stdsync.OnceValues(func() (*httpserver.SchemaValidator, error) {
	return httpserver.NewSchemaValidator(rest.Spec, wsrelay.StreamSchemaNames())
})

// NewTerminalStreams builds the relay over the terminal module t and the control connection hc
// and binds it: the module asks it for slots and counters (BindRelay), signs with d.Signer
// (BindSigner) and records its metric (BindMetrics); the control connection tells it of
// session.started and of its end (Links.BindRelay). Errors: an invalid configuration value or a
// missing collaborator.
func NewTerminalStreams(cfg TerminalStreamsConfig, t *Terminal, hc *HostControl, d TerminalStreamsDeps) (*TerminalStreams, error) {
	if t == nil || t.Service == nil || hc == nil || d.Authenticator == nil || d.Nonces == nil || d.Signer == nil {
		return nil, errors.New("wire: the terminal streams need the module, the control connection, the authenticator, the nonces and the signer")
	}
	m := d.Metrics
	if m == nil && t.Metrics != nil {
		m = t.Metrics
	}
	if m == nil {
		om, err := otelmetrics.New(nil)
		if err != nil {
			return nil, err
		}
		m = om
	}
	windows := domain.Windows{Idle: cfg.IdleTimeout, IdleGrace: cfg.IdleGrace, MaxDuration: cfg.MaxSessionDuration, DetachGrace: cfg.DetachGrace,
		OpenTimeout: cfg.OpenTimeout, OwnerDetachGrace: cfg.OwnerAccountDetachGrace, OwnerMaxDuration: cfg.OwnerAccountMaxSessionDuration}
	relay, err := wsrelay.NewRelay(wsrelay.RelayConfig{BufferMaxBytes: cfg.BufferMaxBytes, StallTimeout: cfg.StallTimeout, OpenTimeout: cfg.OpenTimeout, Multi: d.Multi,
		Windows: windows, Metrics: t.Service.Instruments()}, t.Service, hc.Links, d.Timers, d.Clock, d.Log)
	if err != nil {
		return nil, err
	}
	schemas, err := streamSchemas()
	if err != nil {
		return nil, err
	}
	stream, err := wsrelay.NewStreamHandler(wsrelay.StreamConfig{PublicURL: cfg.PublicURL, OpenTimeout: cfg.OpenTimeout,
		MaxFrameBytes: cfg.MaxFrameBytes, MaxJSONDepth: cfg.MaxJSONDepth},
		wsrelay.StreamDeps{UseCases: t.Service, Authenticator: d.Authenticator, Nonces: d.Nonces, Relay: relay, Schemas: schemas,
			Timers: d.Timers, Clock: d.Clock, Log: d.Log, Limiter: d.Limiter})
	if err != nil {
		return nil, err
	}
	data, err := wsrelay.NewDataHandler(hc.Handler, relay, t.Service)
	if err != nil {
		return nil, err
	}
	t.Service.BindSigner(d.Signer)
	t.Service.BindMetrics(m)
	t.Service.BindRelay(relay)
	hc.Links.BindRelay(relay)
	ts := &TerminalStreams{Relay: relay, Stream: stream, Data: data}
	if err := ts.buildComputer(cfg, t, hc, d); err != nil {
		return nil, err
	}
	return ts, nil
}

// buildComputer builds the computer relay and its two handlers and binds the relay to the module
// (CU-04): the opening tells it the expectation, the log tail ends its slots.
func (s *TerminalStreams) buildComputer(cfg TerminalStreamsConfig, t *Terminal, hc *HostControl, d TerminalStreamsDeps) error {
	var cm wsrelay.ComputerMetrics = wsrelay.NoComputerMetrics{}
	if t.Metrics != nil {
		cm = t.Metrics
	}
	relay, err := wsrelay.NewComputerRelay(wsrelay.ComputerRelayConfig{OpenTimeout: cfg.ComputerOpenTimeout, WriteTimeout: cfg.WriteTimeout},
		t.Service, hc.Links, d.Clock, d.Log, cm)
	if err != nil {
		return err
	}
	schemas, err := attachSchemas()
	if err != nil {
		return err
	}
	attach, err := wsrelay.NewAttachHandler(wsrelay.AttachConfig{OpenTimeout: cfg.ComputerOpenTimeout, MaxFrameBytes: cfg.MaxFrameBytes,
		MaxJSONDepth: cfg.MaxJSONDepth}, t.Service, relay, schemas, d.Clock, d.Log)
	if err != nil {
		return err
	}
	data, err := wsrelay.NewComputerDataHandler(hc.Handler, relay, t.Service)
	if err != nil {
		return err
	}
	t.Service.BindComputerRelay(relay)
	s.Computer, s.Attach, s.ComputerData = relay, attach, data
	return nil
}

// Mount mounts both upgrade routes on mux through chain (only the outer steps run for them).
func (s *TerminalStreams) Mount(mux Mux, chain *httpserver.Chain) {
	mux.Handle(TerminalStreamOperation().Pattern, chain.Wrap(TerminalStreamOperation(), s.Stream))
	mux.Handle(HostDataOperation().Pattern, chain.Wrap(HostDataOperation(), s.Data))
	mux.Handle(ComputerAttachOperation().Pattern, chain.Wrap(ComputerAttachOperation(), s.Attach))
	mux.Handle(HostComputerDataOperation().Pattern, chain.Wrap(HostComputerDataOperation(), s.ComputerData))
}

// StopConnections is the stop step of PhaseConnections (02 section 15.3 step 3, Design note 14 of
// M2-44): every relay connection closes with 4013 within the rest of shutdown.total_timeout.
func (s *TerminalStreams) StopConnections() lifecycle.StopStep {
	return lifecycle.StopStep{Name: "terminal_relay_connections", Phase: lifecycle.PhaseConnections, Run: func(ctx context.Context) error {
		return errors.Join(s.Relay.Shutdown(ctx), s.Computer.Shutdown(ctx))
	}}
}

// Observer returns the observer of the log tail (Design note 11 of M2-44): every event after
// Observe and Dispatch goes to the relay, which closes the connections of a closed session and
// tells the host service.
func (s *TerminalStreams) Observer() LogObserver {
	return func(ctx context.Context, ev sync.Event) {
		s.Relay.ObserveLogEvent(ctx, ev.Type, ev.ResourceID, ev.Payload)
		s.Computer.ObserveLogEvent(ctx, ev.Type, ev.ResourceID, ev.Payload)
	}
}

// LogObserver is a consumer of the log tail of an api process after Authorizer.Observe and the
// hub; it must not block.
type LogObserver func(ctx context.Context, ev sync.Event)

// terminalLimiter adapts the limiter of the process to the terminal channels (M2-46 Design note
// 10): rl.terminal keyed by the human principal, rl.host keyed by the host after its proof, and the
// reservation of rl.public_auth of the client address before presented credentials are checked.
type terminalLimiter struct{ l *ratelimit.Limiter }

// The classes of the terminal channels (05 section 3).
const (
	terminalClass   = ratelimit.Class(terminalStreamClass)
	hostClass       = ratelimit.Class(hostConnectClass)
	publicAuthClass = ratelimit.ClassPublicAuth
)

// AllowTerminal implements wsrelay.Limiter; an error of the limiter (an unknown class, which the
// start check rules out) refuses.
func (t terminalLimiter) AllowTerminal(id ids.ID) (bool, int) {
	return t.allow(terminalClass, ratelimit.KeyForPrincipal(id))
}

// AllowHost implements wsrelay.Limiter.
func (t terminalLimiter) AllowHost(id ids.ID) (bool, int) {
	return t.allow(hostClass, ratelimit.KeyForHost(id))
}

// ReserveCredentials implements wsrelay.Limiter: like the failed-credentials rule of the REST
// chain, one token is reserved and given back unless the check failed.
func (t terminalLimiter) ReserveCredentials(ip netip.Addr) (bool, int, func(bool)) {
	key := ratelimit.KeyForIP(ip)
	ok, retry := t.allow(publicAuthClass, key)
	if !ok {
		return false, retry, func(bool) {}
	}
	return true, 0, func(failed bool) {
		if !failed {
			_ = t.l.Refund(publicAuthClass, key)
		}
	}
}

// allow takes one token of class and key; no limiter allows.
func (t terminalLimiter) allow(class ratelimit.Class, key string) (bool, int) {
	if t.l == nil {
		return true, 0
	}
	d, err := t.l.Allow(class, key)
	if err != nil {
		return false, 1
	}
	return d.Allowed, d.RetryAfterSeconds
}
