// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	terminalhttp "github.com/OWNER/bothy/services/server/internal/terminal/adapters/httpapi"
	terminalpg "github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// TerminalConfig binds the keys of the host use cases (host.name_max_chars,
// host.os_user_max_chars, audit.reason_max_chars, terminal.pairing_code_ttl,
// api.page_size_default, api.page_size_max). The error names the key whose binding failed, never
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

// Terminal is the terminal module of the process (M2-39): the use cases of terminal.Hosts and the
// REST handlers of the five host operations.
type Terminal struct {
	// Service implements terminal.Hosts.
	Service *terminalapp.Service
	// Handlers are the REST handlers.
	Handlers *terminalhttp.Hosts
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
		People: people{dir: identityapp.NewDirectory(identitypg.NewRepository(pool))}, Presence: terminalapp.NoConnections{},
		UoW: uow, Authorizer: az, Audit: w.Audit, Events: w.Events, Cursors: terminalhttp.Cursors{}, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &Terminal{Service: svc, Handlers: terminalhttp.New(svc)}, nil
}
