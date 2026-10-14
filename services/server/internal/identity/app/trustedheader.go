// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/trustedheader"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// The answers of the trusted header driver (08 section 2.4, PLAN W4).
const (
	// ReasonTrustedHeaderRequiresLoopback is the readiness check name and the reason of 409
	// conflict when the driver is enabled without a loopback public listener.
	ReasonTrustedHeaderRequiresLoopback = "trusted_header_requires_loopback"
	// hintTrustedHeaderLoopback names the key the operator changes.
	hintTrustedHeaderLoopback = "The trusted_header driver needs the public listener on a loopback address; set server.listen_addr to 127.0.0.1 or ::1 behind the proxy."
)

// msgSeveralValues is the warning of a login header with several field lines; it never carries a
// value (SEC-LOG-2).
const msgSeveralValues = "trusted header login with several values"

// errRequiresLoopback is the readiness error of an enabled driver without a loopback listener;
// only the check name leaves the process.
var errRequiresLoopback = errors.New("identity: " + ReasonTrustedHeaderRequiresLoopback)

// SessionBinding decides whether a request still belongs to its session beyond token and proof
// (M1-43: the header login of a trusted_header session). The Authenticator and the refresh call it
// after the token and DPoP checks succeeded.
type SessionBinding interface {
	// Bound reports whether the request in ctx may go on with sess; false means the session must
	// be revoked with trusted_header_mismatch. Errors: the store errors.
	Bound(ctx context.Context, sess SessionRecord) (bool, error)
}

// TrustedHeaderDriver holds the state and the shared rules of the driver trusted_header (brief
// M1-43): whether the bound public listener is loopback, whether the driver is active, the
// evaluation of the login header from a trusted TCP peer, the per-request binding and the
// readiness check. It is safe for concurrent use; the loopback flag is set once the public
// listener is bound.
type TrustedHeaderDriver struct {
	proxies   []netip.Prefix
	bind      bool
	loopback  atomic.Bool
	providers ProviderStore
	ids       IdentityStore
	log       *slog.Logger
}

// Compile-time check: the driver binds sessions.
var _ SessionBinding = (*TrustedHeaderDriver)(nil)

// NewTrustedHeaderDriver returns the driver with server.trusted_proxies and
// auth.trusted_header_bind_requests of cfg over the provider and identity stores; the listener
// counts as not loopback until SetPublicListenerLoopback (fail closed). log nil means
// slog.Default(). Errors: a missing store.
func NewTrustedHeaderDriver(cfg Config, providers ProviderStore, identities IdentityStore, log *slog.Logger) (*TrustedHeaderDriver, error) {
	if providers == nil || identities == nil {
		return nil, errors.New("identity: NewTrustedHeaderDriver needs the provider and identity stores")
	}
	if log == nil {
		log = slog.Default()
	}
	return &TrustedHeaderDriver{proxies: append([]netip.Prefix(nil), cfg.TrustedProxies...), bind: cfg.TrustedHeaderBindRequests,
		providers: providers, ids: identities, log: log}, nil
}

// SetPublicListenerLoopback records whether the bound public listener is a loopback address
// (trustedheader.IsLoopbackListener of its address); internal/wire calls it after the listen.
func (d *TrustedHeaderDriver) SetPublicListenerLoopback(loopback bool) { d.loopback.Store(loopback) }

// Loopback reports the flag of SetPublicListenerLoopback; a nil driver is never loopback.
func (d *TrustedHeaderDriver) Loopback() bool { return d != nil && d.loopback.Load() }

// requiresLoopback is the 409 conflict of enabling the driver without a loopback listener.
func requiresLoopback() error {
	return errs.New(errs.CodeConflict, "").WithReason(ReasonTrustedHeaderRequiresLoopback).WithHint(hintTrustedHeaderLoopback)
}

// enabledProvider returns the enabled trusted_header provider with its configuration; found is
// false when there is none. A stored configuration that no longer validates counts as none (fail
// closed).
func (d *TrustedHeaderDriver) enabledProvider(ctx context.Context) (identity.Provider, trustedheader.Config, bool, error) {
	list, err := d.providers.Providers(ctx, true)
	if err != nil {
		return identity.Provider{}, trustedheader.Config{}, false, err
	}
	for _, p := range list {
		if p.Kind != identity.ProviderTrustedHeader {
			continue
		}
		cfg, err := trustedheader.ParseConfig(p.Config)
		if err != nil {
			return identity.Provider{}, trustedheader.Config{}, false, nil
		}
		return p, cfg, true, nil
	}
	return identity.Provider{}, trustedheader.Config{}, false, nil
}

// Ready is the readiness check trusted_header_requires_loopback (W4): it fails while an enabled
// trusted_header provider exists and the public listener is not loopback. Errors: that failure
// and the store errors.
func (d *TrustedHeaderDriver) Ready(ctx context.Context) error {
	if d.Loopback() {
		return nil
	}
	list, err := d.providers.Providers(ctx, true)
	if err != nil {
		return err
	}
	for _, p := range list {
		if p.Kind == identity.ProviderTrustedHeader {
			return errRequiresLoopback
		}
	}
	return nil
}

// evaluate returns the login of the header name from the transport of ctx: only with a loopback
// listener and a trusted TCP peer, exactly one valid value (trustedheader.Evaluate). Several
// values are logged as a warning without the values.
func (d *TrustedHeaderDriver) evaluate(ctx context.Context, peer netip.Addr, header map[string][]string, name string) (string, bool) {
	if !d.Loopback() || name == "" {
		return "", false
	}
	v, outcome := trustedheader.Evaluate(peer, d.proxies, reqmeta.Transport{Header: header}.Values(name))
	if outcome == trustedheader.Multiple {
		d.log.LogAttrs(ctx, slog.LevelWarn, msgSeveralValues, slog.String(logKeyCorrID, reqmeta.CorrelationID(ctx)))
	}
	return v, outcome == trustedheader.Present
}

// Bound implements SessionBinding (08 section 2.4 rule 5): with auth.trusted_header_bind_requests
// a session started with trusted_header stays bound only while the request carries, from a trusted
// peer behind a loopback listener, the login of the session's identity, compared byte for byte
// after decoding. A missing, unlinked or foreign identity, a disabled provider and a missing
// transport are a mismatch. With the switch off, or for another login method, no header is read.
func (d *TrustedHeaderDriver) Bound(ctx context.Context, sess SessionRecord) (bool, error) {
	if !d.bind || sess.AuthMethod != identity.AuthTrustedHeader {
		return true, nil
	}
	if sess.IdentityID.IsZero() {
		return false, nil
	}
	rec, found, err := d.ids.IdentityOfUser(ctx, sess.UserID, sess.IdentityID)
	if err != nil || !found || !rec.UnlinkedAt.IsZero() {
		return false, err
	}
	p, found, err := d.providers.EnabledProvider(ctx, rec.ProviderID)
	if err != nil || !found || p.Kind != identity.ProviderTrustedHeader {
		return false, err
	}
	cfg, err := trustedheader.ParseConfig(p.Config)
	if err != nil {
		return false, nil
	}
	t, _ := reqmeta.TransportOf(ctx)
	login, ok := d.evaluate(ctx, t.Peer, t.Header, cfg.LoginHeader)
	return ok && login == rec.Subject, nil
}
