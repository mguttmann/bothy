// SPDX-License-Identifier: Apache-2.0

package apns_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The test world of brief M3-20 "Tests".
const (
	teamID     = "EXAMPLETEAM"
	keyID      = "EXAMPLEKEY"
	topicPhone = "com.example.chat"
	topicMac   = "com.example.chat.mac"
	interval   = 40 * time.Minute
	retry5xx   = 15 * time.Minute
	ttl        = 24 * time.Hour
	cacheTTL   = time.Minute
)

// start is the fixed start of the fake clock.
var start = time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)

// record is one captured log record.
type record struct {
	Level   slog.Level
	Message string
	Attrs   map[string]string
}

// capture is a slog handler that keeps every record.
type capture struct {
	mu   sync.Mutex
	recs []record
}

func (c *capture) Enabled(context.Context, slog.Level) bool { return true }

func (c *capture) Handle(_ context.Context, r slog.Record) error {
	rec := record{Level: r.Level, Message: r.Message, Attrs: map[string]string{}}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.String()
		return true
	})
	c.mu.Lock()
	c.recs = append(c.recs, rec)
	c.mu.Unlock()
	return nil
}

func (c *capture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *capture) WithGroup(string) slog.Handler      { return c }

// records returns a copy of the captured records.
func (c *capture) records() []record {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]record(nil), c.recs...)
}

// world is Fake-APNs per environment, a key in a file secret, a fake clock and the adapter.
type world struct {
	t          *testing.T
	clk        *clock.Fake
	key        *ecdsa.PrivateKey
	keyFile    string
	sp         *secrets.Provider
	resolver   *switchable
	prod, sand *apnstest.Server
	cfg        apns.Config
	egressCfg  egress.Config
	roots      *x509.CertPool
	log        *capture
	a          *apns.Adapter
	instance   ids.ID
	phone, mac app.Registration
}

// switchable is the secret resolver of the world: the real provider, or an error while failing.
type switchable struct {
	mu   sync.Mutex
	next apns.SecretResolver
	fail error
}

func (s *switchable) Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error) {
	s.mu.Lock()
	fail := s.fail
	s.mu.Unlock()
	if fail != nil {
		return secrets.Value{}, "", fail
	}
	return s.next.Resolve(ctx, raw)
}

func (s *switchable) setFail(err error) {
	s.mu.Lock()
	s.fail = err
	s.mu.Unlock()
}

// option changes the world before the adapter is built.
type option func(*world)

// withoutRules leaves the Fakes out of egress.allowed_hosts.
func withoutRules() option { return func(w *world) { w.egressCfg.AllowedHosts = nil } }

// newWorld builds the world of brief M3-20 "Tests" and the adapter.
func newWorld(t *testing.T, opts ...option) *world {
	t.Helper()
	w := &world{t: t, clk: clock.NewFake(start), key: newKey(t), log: &capture{}, instance: ids.New()}
	dir := t.TempDir()
	w.keyFile = filepath.Join(dir, "apns_key.p8")
	writeKey(t, w.keyFile, w.key)
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: cacheTTL, Clock: w.clk, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	w.sp, w.resolver = sp, &switchable{next: sp}
	w.prod, w.sand = startFake(t, &w.key.PublicKey), startFake(t, &w.key.PublicKey)
	w.roots = x509.NewCertPool()
	w.roots.AddCert(w.prod.Certificate())
	w.roots.AddCert(w.sand.Certificate())
	w.egressCfg = egress.Config{Timeout: 10 * time.Second, MaxResponseBytes: 1 << 16}
	for _, s := range []*apnstest.Server{w.prod, w.sand} {
		r, err := egress.ParseRule(s.EgressRule())
		if err != nil {
			t.Fatal(err)
		}
		w.egressCfg.AllowedHosts = append(w.egressCfg.AllowedHosts, r)
	}
	w.cfg = apns.Config{TeamID: teamID, KeyID: keyID, PrivateKeyRef: "file:" + w.keyFile, Topics: []string{topicPhone, topicMac},
		JWTRefreshInterval: interval, Retry5xxDelay: retry5xx, TTL: ttl}
	w.phone = app.Registration{ID: ids.New(), UserID: ids.New(), DeviceID: ids.New(), Provider: "apns", Environment: "production",
		AppID: topicPhone, Token: newToken(t), CreatedAt: start.Add(-time.Hour)}
	w.mac = app.Registration{ID: ids.New(), UserID: w.phone.UserID, DeviceID: ids.New(), Provider: "apns", Environment: "sandbox",
		AppID: topicMac, Token: newToken(t), CreatedAt: start.Add(-time.Hour)}
	for _, o := range opts {
		o(w)
	}
	w.a = w.build(t)
	return w
}

// build builds an adapter over the world.
func (w *world) build(t *testing.T) *apns.Adapter {
	t.Helper()
	a, err := apns.New(context.Background(), w.cfg, w.deps())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// deps returns the dependencies of an adapter of the world.
func (w *world) deps() apns.Deps {
	return apns.Deps{Egress: w.egressCfg, EgressDeps: egress.Deps{RootCAs: w.roots}, Secrets: w.resolver, Clock: w.clk,
		Logger: slog.New(w.log), Endpoints: map[string]string{"production": w.prod.URL(), "sandbox": w.sand.URL()}}
}

// hint returns a Notification of kind message with thread, message, nid and instance.
func (w *world) hint() app.Notification {
	return app.Notification{ID: ids.New(), InstanceID: w.instance, ThreadID: ids.New(), MessageID: ids.New(), Kind: string(domain.KindMessage),
		Count: 1, Badge: 3, HasBadge: true}
}

// send sends n to reg and fails on a Go error.
func (w *world) send(reg app.Registration, n app.Notification) domain.SendResult {
	w.t.Helper()
	res, err := w.a.Send(context.Background(), reg, n)
	if err != nil {
		w.t.Fatalf("Send: %v", err)
	}
	return res
}

func startFake(t *testing.T, pub *ecdsa.PublicKey) *apnstest.Server {
	t.Helper()
	s, err := apnstest.Start(apnstest.Options{TeamID: teamID, KeyID: keyID, PublicKey: pub})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// pemPKCS8 returns key as a PEM block PRIVATE KEY.
func pemPKCS8(t *testing.T, key any) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// writeKey writes key as a .p8 file with mode 0600, replacing an earlier one.
func writeKey(t *testing.T, path string, key *ecdsa.PrivateKey) {
	t.Helper()
	tmp := path + ".new"
	if err := os.WriteFile(tmp, pemPKCS8(t, key), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

// newToken returns a random device token of 32 bytes in hex.
func newToken(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
