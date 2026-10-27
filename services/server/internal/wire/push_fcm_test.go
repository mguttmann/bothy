// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	notifydomain "github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestConfigLoad_PushTTLBounds: push.ttl holds between one second and 28 days in every role,
// and the problem names the key, never the value (brief M3-21 Design note 2 a).
func TestConfigLoad_PushTTLBounds(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		value string
		ok    bool
	}{{"673h", false}, {"500ms", false}, {"1s", true}, {"24h", true}, {"672h", true}} {
		for _, role := range []string{"api", "worker", "all"} {
			problems := load(t, "BOTHY_PUSH__TTL="+c.value, "BOTHY_SERVER__ROLE="+role)
			if c.ok != (len(problems) == 0) {
				t.Fatalf("push.ttl %s in %s: problems %v", c.value, role, problems)
			}
			if !c.ok {
				text := (&config.LoadError{Problems: problems}).Error()
				if !strings.Contains(text, "push.ttl") {
					t.Fatalf("problem does not name push.ttl: %s", text)
				}
				noValues(t, problems, c.value)
			}
		}
	}
}

// TestConfigCheckPushFCMProjectID: the cross-check push_fcm_project_id refuses a project ID that
// would not be a clean path segment of the send URL; the problem names the key, not the value.
func TestConfigCheckPushFCMProjectID(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"Example-Chat", "1chat", "chat/x", "chat project"} {
		problems := load(t, "BOTHY_PUSH__FCM__PROJECT_ID="+bad)
		text := (&config.LoadError{Problems: problems}).Error()
		if len(problems) == 0 || !strings.Contains(text, "push.fcm.project_id") {
			t.Fatalf("%q: problems %v", bad, problems)
		}
		noValues(t, problems, bad)
	}
	if problems := load(t, "BOTHY_PUSH__FCM__PROJECT_ID=example-chat"); len(problems) != 0 {
		t.Fatalf("example-chat: %v", problems)
	}
}

// fcmSecrets counts resolves and answers from a fixed value or an error.
type fcmSecrets struct {
	n     atomic.Int64
	value []byte
	err   error
}

func (c *fcmSecrets) Resolve(context.Context, string) (secrets.Value, secrets.Version, error) {
	c.n.Add(1)
	if c.err != nil {
		return secrets.Value{}, "", c.err
	}
	return secrets.NewValue(c.value), "v1", nil
}

// googleServiceAccount returns a service account JSON with Google's token_uri and key.
func googleServiceAccount(t *testing.T, key any) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	account := map[string]string{"type": "service_account", // #nosec G101 -- a test key generated per run
		"client_email": "push@example-chat.example.com", "private_key_id": "EXAMPLEKEY",
		"private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), "token_uri": "https://oauth2.googleapis.com/token"}
	b, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestFCMWiring: disabled builds nothing and reads no secret; enabled builds the adapter in
// worker and all, none in api, without a request at start; a missing, broken or non-RSA
// service account stops the start naming push.fcm.service_account_ref (brief M3-21 Design
// notes 2 d and 9).
func TestFCMWiring(t *testing.T) {
	t.Parallel()
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ek, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// DenyAll refuses every request: a request at start would show up as an error of New.
	deps := func(s fcm.SecretResolver) wire.FCMDeps {
		return wire.FCMDeps{Egress: egress.Config{Timeout: time.Second, MaxResponseBytes: 1 << 16}, EgressDeps: egress.Deps{DenyAll: true}, Secrets: s}
	}
	values, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatal(err)
	}
	off, err := wire.FCMConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	if off.Enabled() || off.TTL != 24*time.Hour || off.PauseFor != 5*time.Minute ||
		!strings.HasSuffix(off.ServiceAccountRef, "/fcm_service_account.json") {
		t.Fatalf("default config %+v", off)
	}
	none := &fcmSecrets{err: errors.New("no file")}
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
		ps, err := wire.FCMProviders(ctx, role, off, deps(none))
		if err != nil || len(ps) != 0 {
			t.Fatalf("disabled in %s: %v %v", role, ps, err)
		}
	}
	if none.n.Load() != 0 {
		t.Fatal("disabled FCM read the secret")
	}
	on := off
	on.ProjectID = "example-chat"
	good := &fcmSecrets{value: googleServiceAccount(t, rk)}
	for role, want := range map[wire.Role]int{wire.RoleAPI: 0, wire.RoleWorker: 1, wire.RoleAll: 1} {
		ps, err := wire.FCMProviders(ctx, role, on, deps(good))
		if err != nil || len(ps) != want {
			t.Fatalf("enabled in %s: %d providers, %v", role, len(ps), err)
		}
		if want == 1 && ps[0].Name() != "fcm" {
			t.Fatalf("provider %q", ps[0].Name())
		}
	}
	for name, s := range map[string]*fcmSecrets{
		"missing": {err: secrets.ErrNotFound},
		"broken":  {value: []byte(`{"type":`)},
		"ec key":  {value: googleServiceAccount(t, ek)},
	} {
		_, err := wire.FCMProviders(ctx, wire.RoleWorker, on, deps(s))
		if err == nil || !strings.Contains(err.Error(), "push.fcm.service_account_ref") {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// TestFCMProvidersUseTestEndpoints (brief M5-11 Design note 1): FCMDeps.Endpoints reach the
// adapter, so the E2E server build gets a token from Fake-FCM and sends to it; without them the
// adapter keeps Google's endpoints and refuses the Fake's service account, whose token_uri is not
// Google's (the adapter never takes a host from a secret file).
func TestFCMProvidersUseTestEndpoints(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fake, err := fcmtest.Start(fcmtest.Options{ProjectID: "example-chat", ClientEmail: "push@example-chat.example.com", KeyID: "EXAMPLEKEY", PublicKey: &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fake.Close)
	eg, err := fake.EgressConfig(testkit.Patience)
	if err != nil {
		t.Fatal(err)
	}
	cfg := fcm.Config{ProjectID: "example-chat", ServiceAccountRef: "file:/run/example/fcm_service_account.json", TTL: 24 * time.Hour, PauseFor: 5 * time.Minute}
	deps := wire.FCMDeps{Egress: eg, EgressDeps: egress.Deps{RootCAs: fake.RootCAs()}, Secrets: &fcmSecrets{value: fake.ServiceAccountJSON(key)}}
	if _, err := wire.FCMProviders(t.Context(), wire.RoleWorker, cfg, deps); err == nil || !strings.Contains(err.Error(), "push.fcm.service_account_ref") {
		t.Fatalf("without endpoints: %v, want the refused service account", err)
	}
	deps.Endpoints = wire.FCMEndpoints{Send: fake.URL(), Token: fake.TokenURL()}
	ps, err := wire.FCMProviders(t.Context(), wire.RoleWorker, cfg, deps)
	if err != nil || len(ps) != 1 {
		t.Fatalf("with endpoints: %d providers, %v", len(ps), err)
	}
	reg := notifyapp.Registration{ID: ids.New(), UserID: ids.New(), DeviceID: ids.New(), Provider: "fcm", Environment: "production",
		AppID: "com.example.chat", Token: "alex-phone-token:APA91bEXAMPLE"}
	n := notifyapp.Notification{ID: ids.New(), InstanceID: ids.New(), ThreadID: ids.New(), MessageID: ids.New(),
		Kind: string(notifydomain.KindMessage), Count: 1, Badge: 1, HasBadge: true}
	res, err := ps[0].Send(t.Context(), reg, n)
	if err != nil || res.Class != notifydomain.ResultAccepted {
		t.Fatalf("send: %+v, %v", res, err)
	}
	if reqs := fake.Requests(); len(reqs) != 1 || fake.TokenRequests() != 1 {
		t.Fatalf("Fake-FCM saw %d sends and %d token requests, want 1 and 1", len(reqs), fake.TokenRequests())
	}
}
