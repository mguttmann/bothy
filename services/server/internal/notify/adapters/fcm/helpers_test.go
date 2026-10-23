// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// The test world of brief M3-21 "Tests".
const (
	projectID = "example-chat"
	pauseFor  = 5 * time.Minute
	pushTTL   = 24 * time.Hour
	cacheTTL  = time.Minute
	lifetime  = time.Hour
	canary    = "CANARY-error-message-echo"
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

// count returns the number of records of level.
func (c *capture) count(level slog.Level) int {
	n := 0
	for _, r := range c.records() {
		if r.Level == level {
			n++
		}
	}
	return n
}

// world is one Fake-FCM, a service account in a file secret, a fake clock and the adapter.
type world struct {
	t           *testing.T
	clk         *clock.Fake
	key         *rsa.PrivateKey
	accountFile string
	sp          *secrets.Provider
	resolver    *switchable
	fake        *fcmtest.Server
	cfg         Config
	egressCfg   egress.Config
	log         *capture
	a           *Adapter
	instance    ids.ID
	sam, robin  app.Registration
}

// switchable is the secret resolver of the world: the real provider, or an error while failing.
// The provider keeps serving a cached value when a file disappears, so a resolve error after the
// start needs this switch.
type switchable struct {
	mu   sync.Mutex
	next SecretResolver
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

// withoutRule leaves the Fake out of egress.allowed_hosts.
func withoutRule() option { return func(w *world) { w.egressCfg.AllowedHosts = nil } }

// newWorld builds the world of brief M3-21 "Tests" and the adapter.
func newWorld(t *testing.T, opts ...option) *world {
	t.Helper()
	w := &world{t: t, clk: clock.NewFake(start), key: newRSAKey(t), log: &capture{}, instance: ids.New()}
	fake, err := fcmtest.Start(fcmtest.Options{ProjectID: projectID, ClientEmail: testEmail, KeyID: testKeyID,
		PublicKey: &w.key.PublicKey, Lifetime: lifetime, ErrorMessage: canary})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fake.Close)
	w.fake = fake
	dir := t.TempDir()
	w.accountFile = filepath.Join(dir, "fcm_service_account.json")
	w.writeAccount(w.key)
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: cacheTTL, Clock: w.clk, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	w.sp, w.resolver = sp, &switchable{next: sp}
	w.egressCfg, err = fake.EgressConfig(testkit.Patience)
	if err != nil {
		t.Fatal(err)
	}
	w.cfg = Config{ProjectID: projectID, ServiceAccountRef: "file:" + w.accountFile, TTL: pushTTL, PauseFor: pauseFor}
	w.sam = app.Registration{ID: ids.New(), UserID: ids.New(), DeviceID: ids.New(), Provider: "fcm", Environment: "production",
		AppID: "com.example.chat", Token: "sam-phone-token:APA91bSAMEXAMPLE", CreatedAt: start.Add(-time.Hour)}
	w.robin = app.Registration{ID: ids.New(), UserID: ids.New(), DeviceID: ids.New(), Provider: "fcm", Environment: "production",
		AppID: "com.example.chat", Token: "robin-phone-token:APA91bROBINEXAMPLE", CreatedAt: start.Add(-time.Hour)}
	for _, o := range opts {
		o(w)
	}
	a, err := New(context.Background(), w.cfg, w.deps())
	if err != nil {
		t.Fatal(err)
	}
	w.a = a
	return w
}

// deps returns the dependencies of an adapter of the world.
func (w *world) deps() Deps {
	return Deps{Egress: w.egressCfg, EgressDeps: egress.Deps{RootCAs: w.fake.RootCAs()}, Secrets: w.resolver, Clock: w.clk,
		Logger: slog.New(w.log), Endpoints: Endpoints{Send: w.fake.URL(), Token: w.fake.TokenURL()}}
}

// writeAccount writes the service account of key as a file secret with mode 0600, replacing an
// earlier one.
func (w *world) writeAccount(key *rsa.PrivateKey) {
	w.t.Helper()
	tmp := w.accountFile + ".new"
	if err := os.WriteFile(tmp, w.fake.ServiceAccountJSON(key), 0o600); err != nil {
		w.t.Fatal(err)
	}
	if err := os.Rename(tmp, w.accountFile); err != nil {
		w.t.Fatal(err)
	}
}

// hint returns a Notification of kind message with thread, message, nid, instance and badge 3.
func (w *world) hint() app.Notification {
	return app.Notification{ID: ids.New(), InstanceID: w.instance, ThreadID: ids.New(), MessageID: ids.New(),
		Kind: string(domain.KindMessage), Count: 1, Badge: 3, HasBadge: true}
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

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// TestWorldDeadlineIsPatience: the request deadline of the test world is testkit.Patience, never
// a fixed number of seconds, so the parallel sends of the token cache tests cannot run into it on
// a loaded machine (docs/DECISIONS.md D-M3-21-16).
func TestWorldDeadlineIsPatience(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	if w.egressCfg.Timeout != testkit.Patience {
		t.Fatalf("egress timeout of the world = %v, want testkit.Patience (%v)", w.egressCfg.Timeout, testkit.Patience)
	}
	if len(w.egressCfg.AllowedHosts) != 1 {
		t.Fatalf("allowed hosts = %d, want the one rule of the Fake", len(w.egressCfg.AllowedHosts))
	}
}
