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
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	terminalhttp "github.com/OWNER/bothy/services/server/internal/terminal/adapters/httpapi"
	terminalpg "github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/wsrelay"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// TerminalConfig binds the keys of the host use cases (host.name_max_chars,
// host.os_user_max_chars, audit.reason_max_chars, terminal.pairing_code_ttl,
// terminal.fingerprint_confirm_chars, terminal.fingerprint_max_attempts,
// terminal.report_label_max_chars, terminal.report_max_entries, api.page_size_default,
// api.page_size_max). The error names the key whose binding failed, never
// a value.
func TerminalConfig(v *config.Values) (terminalapp.Config, error) {
	var c terminalapp.Config
	if err := config.Bind(v, &c); err != nil {
		return terminalapp.Config{}, err
	}
	return c, nil
}

// NewHostSource returns the authz.HostSource of the process over pool: the host records of the
// terminal module (M2-39). The Authorizer, the rights hook and the grant use cases read hosts
// through it.
func NewHostSource(pool *pg.Pool) authz.HostSource {
	return terminalapp.NewHostSource(terminalpg.NewRepository(pool))
}

// pairingCodes binds the outgoing port terminal/app.PairingCodes to identity.Tokens.
type pairingCodes struct{ tokens identity.Tokens }

// Issue implements terminalapp.PairingCodes.
func (p pairingCodes) Issue(ctx context.Context) (identity.PairingCode, error) {
	return p.tokens.NewHostPairingCode(ctx)
}

// Hash implements terminalapp.PairingCodes (M2-41).
func (p pairingCodes) Hash(ctx context.Context, raw string) ([]byte, error) {
	return p.tokens.HashHostPairingCode(ctx, raw)
}

// instanceIdentity binds the outgoing port terminal/app.Instance to instance.State (M2-41).
type instanceIdentity struct{ state instance.State }

// InstanceID implements terminalapp.Instance.
func (i instanceIdentity) InstanceID(ctx context.Context) (ids.ID, error) {
	r, err := i.state.Load(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	return r.InstanceID, nil
}

// people binds the outgoing port terminal/app.People to identity.Directory.
type people struct{ dir identity.Directory }

// User implements terminalapp.People: an unknown id is found false.
func (p people) User(ctx context.Context, id ids.ID) (terminalapp.UserFacts, bool, error) {
	v, err := p.dir.Principal(ctx, id)
	if errs.KindOf(err) == errs.KindNotFound {
		return terminalapp.UserFacts{}, false, nil
	}
	if err != nil {
		return terminalapp.UserFacts{}, false, err
	}
	return terminalapp.UserFacts{Kind: v.Kind, Status: v.Status}, true, nil
}

// IsActiveOwner implements terminalapp.People (M2-40): an active owners row and status active,
// read through identity.Directory.User without authorize; an unknown id is false.
func (p people) IsActiveOwner(ctx context.Context, id ids.ID) (bool, error) {
	v, err := p.dir.User(ctx, id)
	if errs.KindOf(err) == errs.KindNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v.IsOwner && v.Status == identity.StatusActive, nil
}

// Terminal is the terminal module of the process (M2-39): the use cases of terminal.Hosts and the
// REST handlers of the five host operations.
type Terminal struct {
	// Service implements terminal.Hosts.
	Service *terminalapp.Service
	// Handlers are the REST handlers.
	Handlers *terminalhttp.Hosts
	// Sessions are the REST handlers of the terminal sessions (M2-44).
	Sessions *terminalhttp.TerminalSessions
}

// NewTerminal builds the terminal module over pool with the unit of work uow, the Authorizer az,
// the recorder and appender of w, the identity module id (the pairing codes of identity.Tokens and
// the directory of people) and the clock (nil means clock.System()). Presence is NoConnections
// until M2-43 builds the control connections. It opens no connection. Errors: a missing
// collaborator or a configuration value the use cases refuse.
func NewTerminal(cfg terminalapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, w IdentityWrites, id *Identity,
	clk clock.Clock, _ *slog.Logger) (*Terminal, error) {
	if id == nil || id.Tokens == nil {
		return nil, errors.New("wire: the terminal module needs the identity module")
	}
	svc, err := terminalapp.New(cfg, terminalapp.Deps{
		Store: terminalpg.NewRepository(pool), PairingCodes: pairingCodes{tokens: id.Tokens},
		Instance: instanceIdentity{state: instanceapp.NewState(instancepg.NewRepository(pool), uow)},
		People:   people{dir: identityapp.NewDirectory(identitypg.NewRepository(pool))}, Presence: terminalapp.NoConnections{},
		UoW: uow, Authorizer: az, Audit: w.Audit, Events: w.Events, Cursors: terminalhttp.Cursors{}, Clock: clk,
		StepUps: lateStepUps{id: id},
	})
	if err != nil {
		return nil, err
	}
	return &Terminal{Service: svc, Handlers: terminalhttp.New(svc), Sessions: terminalhttp.NewSessions(svc.Sessions())}, nil
}

// lateStepUps binds terminal/app.StepUps to identity's StepUps, which the sessions step of
// identity sets after the terminal module is built (BindSessions); before it the consumption fails
// closed.
type lateStepUps struct{ id *Identity }

// ConsumeFresh implements terminalapp.StepUps.
func (l lateStepUps) ConsumeFresh(ctx context.Context, a actor.Actor) error {
	if l.id == nil || l.id.StepUps == nil {
		return errs.New(errs.CodeInternalError, "wire: the step-ups of identity are not bound")
	}
	return l.id.StepUps.ConsumeFresh(ctx, a)
}

// The route of the control connection of the host services (M2-43, asyncapi channel hostControl):
// one endpoint on the public listener of the roles api and all. Like the gateway it is no
// operation of openapi.yaml, so its binding is written here; it authorizes host.connect on the
// connection (identify and every later op), so it is marked AuthorizedInBand.
const (
	hostConnectOperationID = "hostConnect"
	hostConnectPath        = "/api/v1/hosts/connect"
	hostConnectPattern     = http.MethodGet + " " + hostConnectPath
	hostConnectClass       = "rl.host"
)

// HostConnectOperation returns the route of GET /api/v1/hosts/connect for the HTTP chain: action
// host.connect, class rl.host, marked AuthorizedInBand (the proof and authorize run on the
// connection, 24 section 3.5).
func HostConnectOperation() httpserver.Operation {
	return httpserver.Operation{ID: hostConnectOperationID, Method: http.MethodGet, Path: hostConnectPath, Pattern: hostConnectPattern,
		Action: string(authorize.ActionHostConnect), RateLimitClass: hostConnectClass, AuthorizedInBand: true}
}

// HostConnectRouteBinding is the binding of the route for the catalogue check of the start (AZ-P1).
func HostConnectRouteBinding() authorize.RouteBinding {
	op := HostConnectOperation()
	return authorize.RouteBinding{OperationID: op.ID, Method: op.Method, Pattern: op.Pattern, Action: op.Action, RateLimitClass: op.RateLimitClass}
}

// HostControlConfig binds the keys of the control connection (M2-43).
type HostControlConfig struct {
	HeartbeatInterval     time.Duration `config:"terminal.host_heartbeat_interval"`
	HeartbeatMissLimit    int           `config:"terminal.host_heartbeat_miss_limit"`
	AuthTimeout           time.Duration `config:"terminal.host_auth_timeout"`
	DuplicateProbeTimeout time.Duration `config:"terminal.host_duplicate_probe_timeout"`
	MaxFrameBytes         int           `config:"sync.ws_max_frame_bytes"`
	MaxJSONDepth          int           `config:"http.max_json_depth"`
	OutboundQueueMax      int           `config:"sync.outbound_queue_max"`
	WriteTimeout          time.Duration `config:"gateway.write_timeout"`
}

// HostControlConfigFrom binds HostControlConfig from v; the error names the key, never a value.
func HostControlConfigFrom(v *config.Values) (HostControlConfig, error) {
	var c HostControlConfig
	if err := config.Bind(v, &c); err != nil {
		return HostControlConfig{}, err
	}
	return c, nil
}

// HostControl is the control connection of the host services in an api process: the registry of
// links and the WebSocket handler.
type HostControl struct {
	Links   *wsrelay.Links
	Handler *wsrelay.Handler
}

// hostSchemas compiles the schemas of the host ops once per process.
var hostSchemas = stdsync.OnceValues(func() (*httpserver.SchemaValidator, error) {
	return httpserver.NewSchemaValidator(rest.Spec, wsrelay.SchemaNames())
})

// NewHostControl builds the control connection over the terminal module t: the registry publishes
// the presence through publisher (the gateway hub, M1-66), the use cases read the keys of purpose
// terminal from keys, and the module closes and updates the links after its commits
// (Service.BindControl). Errors: an invalid configuration value.
func NewHostControl(cfg HostControlConfig, t *Terminal, publisher eventlog.EphemeralPublisher, keys terminalapp.TerminalKeys,
	clk clock.Clock, log *slog.Logger) (*HostControl, error) {
	if t == nil || t.Service == nil {
		return nil, errors.New("wire: the control connection needs the terminal module")
	}
	schemas, err := hostSchemas()
	if err != nil {
		return nil, err
	}
	links := wsrelay.NewLinks(publisher, log)
	h, err := wsrelay.NewHandler(wsrelay.Config{
		HeartbeatInterval: cfg.HeartbeatInterval, HeartbeatMissLimit: cfg.HeartbeatMissLimit, AuthTimeout: cfg.AuthTimeout,
		DuplicateProbeTimeout: cfg.DuplicateProbeTimeout, MaxFrameBytes: cfg.MaxFrameBytes, MaxJSONDepth: cfg.MaxJSONDepth,
		OutboundQueueMax: cfg.OutboundQueueMax, WriteTimeout: cfg.WriteTimeout,
	}, wsrelay.Deps{UseCases: t.Service, Links: links, Schemas: schemas, Clock: clk, Log: log})
	if err != nil {
		return nil, err
	}
	t.Service.BindKeys(keys)
	links.Bind(t.Service, t.Service)
	t.Service.BindControl(links)
	return &HostControl{Links: links, Handler: h}, nil
}

// Mount mounts GET /api/v1/hosts/connect on mux through chain (only the outer steps run for it).
func (c *HostControl) Mount(mux Mux, chain *httpserver.Chain) {
	mux.Handle(hostConnectPattern, chain.Wrap(HostConnectOperation(), c.Handler))
}

// StopConnections is the stop step of PhaseConnections (02 section 15.3 step 3): every control
// connection detaches its sessions and closes with 4013 within the rest of shutdown.total_timeout.
func (c *HostControl) StopConnections() lifecycle.StopStep {
	return lifecycle.StopStep{Name: "host_control_connections", Phase: lifecycle.PhaseConnections, Run: c.Links.Shutdown}
}
