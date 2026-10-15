// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestServe_AuthenticatesPeople: a started api process authenticates a person with an access
// token issued over the same installation (instance_id as iss, the shared signing keys, the
// token pepper of the secrets) through identity's Authenticator, and answers the token as Bearer
// with invalid_dpop_proof (M1-34; before, AnonymousOnly refused every credential).
func TestServe_AuthenticatesPeople(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newServeDB(t)
	env := serveEnv(t, db)
	p := start(t, wire.RoleAPI, env, wire.Deps{})
	waitReadyz(t, p.waitInternal(t), http.StatusOK)
	public := p.logs.port("public listener started")

	// The same installation, built beside the process: identity over the shared key store.
	values := loadEnv(t, env)
	cfg, err := wire.IdentityConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	keyCfg, err := wire.SigningKeysConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	keyCfg.Dir = envValue(env, "BOTHY_SECRETS__SIGNING_KEY_DIR")
	sp, err := secrets.NewProvider(secrets.Options{Dir: envValue(env, "BOTHY_SECRETS__DIR"), SigningKeyDir: keyCfg.Dir, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	pepper, err := wire.TokenPepper(ctx, sp, cfg.TokenPepperRef)
	if err != nil {
		t.Fatal(err)
	}
	pool := db.Pool(t, pg.RoleApp)
	log := slog.New(slog.DiscardHandler)
	a, err := wire.NewAudit(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute},
		auditapp.Config{DenyAggregateWindow: time.Minute, DenyAggregateMaxKeys: 10, IPKeyPeriod: 24 * time.Hour, IPKeyRetention: 48 * time.Hour,
			IPKEKRef: "file:<secrets.dir>/audit_ip_kek"}, testKEK{}, log)
	if err != nil {
		t.Fatal(err)
	}
	syncCfg, err := wire.SyncConfigFrom(values)
	if err != nil {
		t.Fatal(err)
	}
	sy, err := wire.NewSync(pool, a.UoW(), syncCfg, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := wire.NewSigningKeys(keyCfg, pool, sp, a.UoW(), allowAll{}, a.Recorder(), nil, log)
	if err != nil {
		t.Fatal(err)
	}
	id, err := wire.NewIdentity(cfg, pool, a.UoW(), allowAll{}, wire.IdentityWrites{Audit: a.Recorder(), Events: sy.Appender()}, pepper, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	var instanceID string
	if err := db.Pool(t, pg.RoleOwner).QueryRow(ctx, `SELECT instance_id::text FROM instance`).Scan(&instanceID); err != nil {
		t.Fatal(err)
	}
	if err := id.BindSessions(keys.Keys, ids.MustParse(instanceID)); err != nil {
		t.Fatal(err)
	}

	user := ids.New()
	owner := db.Pool(t, pg.RoleOwner)
	if _, err := owner.Exec(ctx, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, user.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO users (principal_id) VALUES ($1)`, user.String()); err != nil {
		t.Fatal(err)
	}
	key, pub := identitytest.NewDeviceKey(jose.AlgEdDSA)
	dev, err := id.Devices.Register(ctx, identity.RegisterDevice{DeviceID: ids.New(), UserID: user, Name: "Alex Laptop",
		Platform: identity.PlatformMacOS, DPoPKey: pub, State: identity.DeviceStateActive, ApprovedBy: &user})
	if err != nil {
		t.Fatal(err)
	}
	set, err := id.Sessions.StartSession(ctx, identity.StartSession{UserID: user, DeviceID: dev.ID, AuthMethod: identity.AuthPasskey})
	if err != nil {
		t.Fatal(err)
	}
	access := set.AccessToken.Reveal()
	const path = "/api/v1/me/sessions"
	call := func(auth string, proof bool) (int, string) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(public)+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", auth)
		if proof {
			req.Header.Set("DPoP", identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: http.MethodGet, HTU: cfg.PublicURL + path,
				IAT: time.Now(), Nonce: id.DPoP.CurrentNonce(), ATH: dpop.ATH(access)}))
		}
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, body := call("DPoP "+access, true); code != http.StatusOK || !strings.Contains(body, set.Session.ID.String()) {
		t.Fatalf("DPoP request = %d %s", code, body)
	}
	if code, body := call("Bearer "+access, false); code != http.StatusUnauthorized || !strings.Contains(body, `"code":"invalid_dpop_proof"`) {
		t.Fatalf("Bearer request = %d %s", code, body)
	}
}
