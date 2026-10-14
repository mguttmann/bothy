// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"

	identityhttp "github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/replay"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// IdentityConfig binds the keys of the identity module (principal.*, the DPoP windows,
// auth.token_pepper_ref, server.public_url and the page sizes) into its configuration.
//
// The error names the key whose binding failed (config.Bind), never a value.
func IdentityConfig(v *config.Values) (app.Config, error) {
	var c app.Config
	if err := config.Bind(v, &c); err != nil {
		return app.Config{}, err
	}
	return c, nil
}

// pepperSize is the length of the token pepper: secrets init writes 32 random bytes as hex
// (secrets/stack GeneratorKey).
const pepperSize = 32

// errTokenPepper reports a token pepper that is not 32 bytes as hex; it never names the value.
var errTokenPepper = errors.New("wire: auth.token_pepper_ref does not hold 32 bytes as hex")

// TokenPepper resolves auth.token_pepper_ref through the secret provider and returns the raw 32
// bytes. Errors: those of the provider (a missing or unreadable secret) and errTokenPepper; no
// error contains the secret.
func TokenPepper(ctx context.Context, sp *secrets.Provider, ref string) ([]byte, error) {
	v, _, err := sp.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	raw, err := hex.DecodeString(strings.TrimSpace(string(v.Bytes())))
	if err != nil || len(raw) != pepperSize {
		return nil, errTokenPepper
	}
	return raw, nil
}

// Identity is the identity module of the process (M1-33): the device use cases, the DPoP verifier
// with its jti store and the REST handlers of the device operations.
type Identity struct {
	// Devices is the port identity.Devices.
	Devices *app.Devices
	// DPoP is the port identity.DPoP; M1-34 hands it to the Authenticator.
	DPoP *app.DPoPVerifier
	// Replay is the jti store of DPoP; Start sweeps it.
	Replay *replay.Store
	// Handlers are the REST handlers of listMyDevices, listUserDevices and getDevice.
	Handlers *identityhttp.Devices
	cfg      app.Config
}

// NewIdentity builds the identity module over pool with the unit of work uow, the Authorizer az,
// the token pepper (raw bytes), the clock (nil means clock.System()) and log. It opens no
// connection and starts nothing. Errors: a window or page size outside its range, an empty pepper.
func NewIdentity(cfg app.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, pepper []byte,
	clk clock.Clock, log *slog.Logger) (*Identity, error) {
	if clk == nil {
		clk = clock.System()
	}
	repo := identitypg.NewRepository(pool)
	store := replay.New(clk)
	verifier, err := app.NewDPoPVerifier(cfg, pepper, store, clk, orDiscard(log))
	if err != nil {
		return nil, err
	}
	devices, err := app.NewDevices(cfg, app.DevicesDeps{
		Store: repo, People: repo, UoW: uow, Authorizer: az, Cursors: identityhttp.Cursors{},
	})
	if err != nil {
		return nil, err
	}
	return &Identity{Devices: devices, DPoP: verifier, Replay: store, Handlers: identityhttp.NewDevices(devices), cfg: cfg}, nil
}

// Start runs the sweeper of the jti store every auth.dpop_proof_max_age (the length of the replay
// window, docs/DECISIONS.md M1-33) and registers its stop step in the HTTP phase.
func (i *Identity) Start(r *lifecycle.Runner) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		i.Replay.Run(ctx, i.cfg.DPoPProofMaxAge)
	}()
	r.OnStop(lifecycle.StopStep{Name: "dpop_replay_sweeper", Phase: lifecycle.PhaseHTTP, Run: func(stopCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
	}})
}
