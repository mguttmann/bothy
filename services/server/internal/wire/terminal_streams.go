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
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/wsrelay"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
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
)

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
	for _, op := range []httpserver.Operation{TerminalStreamOperation(), HostDataOperation()} {
		out = append(out, authorize.RouteBinding{OperationID: op.ID, Method: op.Method, Pattern: op.Pattern, Action: op.Action,
			RateLimitClass: op.RateLimitClass})
	}
	return out
}

// TerminalStreamsConfig binds the keys of the relay and the device stream (M2-44).
type TerminalStreamsConfig struct {
	PublicURL      string        `config:"server.public_url"`
	OpenTimeout    time.Duration `config:"terminal.open_timeout"`
	BufferMaxBytes int           `config:"terminal.relay_buffer_max_bytes"`
	MaxFrameBytes  int           `config:"sync.ws_max_frame_bytes"`
	MaxJSONDepth   int           `config:"http.max_json_depth"`
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
}

// TerminalStreams are the relay of this process with its two handlers.
type TerminalStreams struct {
	Relay  *wsrelay.Relay
	Stream *wsrelay.StreamHandler
	Data   *wsrelay.DataHandler
}

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
	if m == nil {
		om, err := otelmetrics.New(nil)
		if err != nil {
			return nil, err
		}
		m = om
	}
	relay, err := wsrelay.NewRelay(wsrelay.RelayConfig{BufferMaxBytes: cfg.BufferMaxBytes, OpenTimeout: cfg.OpenTimeout}, t.Service, hc.Links,
		d.Timers, d.Clock, d.Log)
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
			Clock: d.Clock, Log: d.Log})
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
	return &TerminalStreams{Relay: relay, Stream: stream, Data: data}, nil
}

// Mount mounts both upgrade routes on mux through chain (only the outer steps run for them).
func (s *TerminalStreams) Mount(mux Mux, chain *httpserver.Chain) {
	mux.Handle(TerminalStreamOperation().Pattern, chain.Wrap(TerminalStreamOperation(), s.Stream))
	mux.Handle(HostDataOperation().Pattern, chain.Wrap(HostDataOperation(), s.Data))
}

// StopConnections is the stop step of PhaseConnections (02 section 15.3 step 3, Design note 14 of
// M2-44): every relay connection closes with 4013 within the rest of shutdown.total_timeout.
func (s *TerminalStreams) StopConnections() lifecycle.StopStep {
	return lifecycle.StopStep{Name: "terminal_relay_connections", Phase: lifecycle.PhaseConnections, Run: s.Relay.Shutdown}
}

// Observer returns the observer of the log tail (Design note 11 of M2-44): every event after
// Observe and Dispatch goes to the relay, which closes the connections of a closed session and
// tells the host service.
func (s *TerminalStreams) Observer() LogObserver {
	return func(ctx context.Context, ev sync.Event) {
		s.Relay.ObserveLogEvent(ctx, ev.Type, ev.ResourceID, ev.Payload)
	}
}

// LogObserver is a consumer of the log tail of an api process after Authorizer.Observe and the
// hub; it must not block.
type LogObserver func(ctx context.Context, ev sync.Event)
