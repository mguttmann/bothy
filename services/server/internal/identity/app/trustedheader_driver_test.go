// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// headerProviders is a ProviderStore that knows one provider or fails with err.
type headerProviders struct {
	app.ProviderStore
	p   identity.Provider
	err error
}

func (s headerProviders) Providers(context.Context, bool) ([]identity.Provider, error) {
	return []identity.Provider{s.p}, s.err
}

func (s headerProviders) EnabledProvider(_ context.Context, id ids.ID) (identity.Provider, bool, error) {
	return s.p, s.err == nil && id == s.p.ID, s.err
}

// headerIdentities is an IdentityStore that knows one identity record.
type headerIdentities struct {
	app.IdentityStore
	rec app.IdentityRecord
	err error
}

func (s headerIdentities) IdentityOfUser(_ context.Context, _, id ids.ID) (app.IdentityRecord, bool, error) {
	return s.rec, s.err == nil && id == s.rec.ID, s.err
}

// TestTrustedHeaderDriver covers the constructor, the readiness check and the request binding of
// the driver: a missing store is an error, an enabled provider without a loopback listener is not
// ready, and a bound session needs the header of its own identity from a trusted peer.
func TestTrustedHeaderDriver(t *testing.T) {
	ctx := context.Background()
	if _, err := app.NewTrustedHeaderDriver(app.Config{}, nil, nil, nil); err == nil {
		t.Fatal("NewTrustedHeaderDriver without stores passed")
	}
	prov := identity.Provider{ID: ids.New(), Kind: identity.ProviderTrustedHeader, Config: json.RawMessage(`{"login_header":"Tailscale-User-Login"}`)}
	rec := app.IdentityRecord{Identity: identity.Identity{ID: ids.New(), ProviderID: prov.ID, Subject: "alex@example.com"}}
	cfg := app.Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}, TrustedHeaderBindRequests: true}
	d, err := app.NewTrustedHeaderDriver(cfg, headerProviders{p: prov}, headerIdentities{rec: rec}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Ready(ctx) == nil {
		t.Error("ready without a loopback listener")
	}
	broken, err := app.NewTrustedHeaderDriver(cfg, headerProviders{err: errors.New("database")}, headerIdentities{err: errors.New("database")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if broken.Ready(ctx) == nil {
		t.Error("ready although the provider store failed")
	}
	d.SetPublicListenerLoopback(true)
	if err := d.Ready(ctx); err != nil {
		t.Errorf("ready with a loopback listener: %v", err)
	}
	sess := app.SessionRecord{ID: ids.New(), UserID: ids.New(), IdentityID: rec.ID, AuthMethod: identity.AuthTrustedHeader, ExpiresAt: time.Now().Add(time.Hour)}
	header := func(v string) context.Context {
		return reqmeta.WithTransport(ctx, reqmeta.Transport{Peer: netip.MustParseAddr("127.0.0.1"), Header: map[string][]string{"Tailscale-User-Login": {v}}})
	}
	if ok, err := d.Bound(header("alex@example.com"), sess); !ok || err != nil {
		t.Errorf("own login = %v, %v", ok, err)
	}
	if ok, _ := d.Bound(header("sam@example.com"), sess); ok {
		t.Error("a foreign login kept the session")
	}
	if ok, _ := d.Bound(ctx, sess); ok {
		t.Error("a request without transport kept the session")
	}
	noIdentity := sess
	noIdentity.IdentityID = ids.ID{}
	if ok, _ := d.Bound(header("alex@example.com"), noIdentity); ok {
		t.Error("a session without identity stayed bound")
	}
	if ok, err := broken.Bound(header("alex@example.com"), sess); ok || err == nil {
		t.Errorf("failing identity store = %v, %v", ok, err)
	}
	other := sess
	other.AuthMethod = identity.AuthPasskey
	if ok, _ := d.Bound(ctx, other); !ok {
		t.Error("a session of another login method was checked")
	}
}
