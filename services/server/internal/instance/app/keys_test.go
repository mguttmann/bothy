// SPDX-License-Identifier: Apache-2.0

//go:build unix

package app_test

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/keystore"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// keyCfg are short rotation times, so a test walks through several rotations in few steps.
var keyCfg = app.KeysConfig{
	CacheTTL: 30 * time.Second, InvocationRotation: 2 * time.Hour, AccessTokenRotation: 30 * 24 * time.Hour,
	JWKSMaxAge: 20 * time.Minute, AccessTokenTTL: 10 * time.Minute, InvocationTokenTTL: time.Minute,
	WebhookDeliveryTimeout: 15 * time.Second, ClockLeeway: 30 * time.Second,
}

// checkInterval is signing_key.rotation_check_interval of these tests.
const checkInterval = 5 * time.Minute

// testKEK is the KEK source of the audit module: one key under every reference.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(strings.Repeat("6b", 32)), nil
}

// allow is an Authorizer that allows every call; the HTTP tests use the real one.
type allow struct{}

func (allow) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (allow) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// keysEnv is a database, a key store directory, a fake clock and the audit module.
type keysEnv struct {
	db    *pgtest.DB
	pool  *pg.Pool
	super *pg.Pool
	audit *wire.Audit
	clk   *clock.Fake
	dir   string
	logs  *logRecords
}

func newKeysEnv(t *testing.T) *keysEnv {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 8
	pool := db.OpenPool(t, cfg)
	unit := pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}
	a, err := wire.NewAudit(pool, unit, auditapp.Config{
		DenyAggregateWindow: time.Minute, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, testKEK{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return &keysEnv{
		db: db, pool: pool, super: db.Pool(t, pg.RoleOwner), audit: a,
		clk: clock.NewFake(time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)), dir: dir, logs: &logRecords{},
	}
}

// keys returns a new Keys instance over the environment, as a second process would build it:
// its own provider (own secret cache) and its own verification cache.
func (e *keysEnv) keys(t *testing.T) *app.Keys {
	t.Helper()
	return e.keysWith(t, nil)
}

// keysWith is keys with the repository wrapped by wrap (nil keeps it).
func (e *keysEnv) keysWith(t *testing.T, wrap func(app.KeyRepository) app.KeyRepository) *app.Keys {
	t.Helper()
	p, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(e.dir), SigningKeyDir: e.dir, CacheTTL: time.Minute, Clock: e.clk})
	if err != nil {
		t.Fatal(err)
	}
	ks, err := keystore.New(p, e.dir)
	if err != nil {
		t.Fatal(err)
	}
	var repo app.KeyRepository = postgres.NewRepository(e.pool)
	if wrap != nil {
		repo = wrap(repo)
	}
	k, err := app.NewKeys(keyCfg, app.KeysDeps{
		Repo: repo, Store: ks, UoW: e.audit.UoW(), Authorizer: allow{},
		Audit: e.audit.Recorder(), Clock: e.clk, Log: slog.New(e.logs),
	})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// row is a key read back as the owner.
type row struct {
	kid, purpose, state, ref string
	notAfter                 *time.Time
}

func (e *keysEnv) rows(t *testing.T) []row {
	t.Helper()
	rs, err := e.super.Query(context.Background(), `SELECT kid, purpose, state, private_key_ref, not_after FROM server_signing_keys ORDER BY created_at, kid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var out []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.kid, &r.purpose, &r.state, &r.ref, &r.notAfter); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// transitionAudits returns the details of every signing_key.transition entry.
func (e *keysEnv) transitionAudits(t *testing.T) []map[string]string {
	t.Helper()
	rs, err := e.super.Query(context.Background(), `SELECT details, actor_principal_id IS NULL, resource_kind FROM audit_events WHERE action = 'signing_key.transition' ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var out []map[string]string
	for rs.Next() {
		var raw []byte
		var noActor bool
		var kind string
		if err := rs.Scan(&raw, &noActor, &kind); err != nil {
			t.Fatal(err)
		}
		d := map[string]string{}
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		if !noActor || kind != "instance" {
			t.Errorf("transition entry with an actor or resource kind %q", kind)
		}
		out = append(out, d)
	}
	return out
}

// files returns the names in the key store.
func (e *keysEnv) files(t *testing.T) []string {
	t.Helper()
	des, err := os.ReadDir(e.dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s has mode %04o, want 0600", de.Name(), fi.Mode().Perm())
		}
		out = append(out, de.Name())
	}
	return out
}

// logRecords captures every log line with its attributes as text.
type logRecords struct {
	mu    sync.Mutex
	lines []string
}

func (l *logRecords) Enabled(context.Context, slog.Level) bool { return true }

func (l *logRecords) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" " + a.Key + "=" + a.Value.String())
		return true
	})
	l.mu.Lock()
	l.lines = append(l.lines, b.String())
	l.mu.Unlock()
	return nil
}

func (l *logRecords) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *logRecords) WithGroup(string) slog.Handler      { return l }

func activeByPurpose(rs []row) map[string][]string {
	out := map[string][]string{}
	for _, r := range rs {
		if r.state == "active" {
			out[r.purpose] = append(out[r.purpose], r.kid)
		}
	}
	return out
}

func TestEnsureActive_EmptyDatabase(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	k := e.keys(t)
	if err := k.EnsureActive(context.Background()); err != nil {
		t.Fatalf("EnsureActive: %v", err)
	}
	rs := e.rows(t)
	active := activeByPurpose(rs)
	for _, p := range instance.Purposes() {
		if len(active[string(p)]) != 1 {
			t.Errorf("purpose %s has %d active keys, want 1", p, len(active[string(p)]))
		}
	}
	if len(rs) != 4 || len(e.files(t)) != 4 {
		t.Fatalf("%d rows and %d files, want 4 each", len(rs), len(e.files(t)))
	}
	for _, r := range rs {
		if r.ref != "file:"+filepath.Join(e.dir, r.purpose+"-"+r.kid+".pem") {
			t.Errorf("private_key_ref %q of %s", r.ref, r.kid)
		}
	}
	if got := e.transitionAudits(t); len(got) != 4 || got[0]["from_state"] != "none" || got[0]["to_state"] != "active" {
		t.Errorf("transition audit entries %v, want four from none to active", got)
	}
	if err := k.EnsureActive(context.Background()); err != nil {
		t.Fatal(err)
	}
	if again := e.rows(t); !slices.EqualFunc(again, rs, func(a, b row) bool { return a.kid == b.kid && a.state == b.state }) {
		t.Errorf("a second EnsureActive changed the keys: %v -> %v", rs, again)
	}
}

func TestEnsureActive_ConcurrentInstances(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	a, b := e.keys(t), e.keys(t)
	var wg sync.WaitGroup
	errc := make(chan error, 2)
	for _, k := range []*app.Keys{a, b} {
		wg.Go(func() { errc <- k.EnsureActive(context.Background()) })
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		if err != nil {
			t.Fatalf("EnsureActive: %v", err)
		}
	}
	rs := e.rows(t)
	active := activeByPurpose(rs)
	for _, p := range instance.Purposes() {
		if len(active[string(p)]) != 1 {
			t.Errorf("purpose %s has %d active keys, want 1", p, len(active[string(p)]))
		}
	}
	files := e.files(t)
	if len(rs) != 4 || len(files) != 4 {
		t.Errorf("%d rows and files %v, want exactly 4 each (the losing files removed)", len(rs), files)
	}
}

// sample is one signature of TestScheduledRotation_EverySignatureStaysVerifiable.
type sample struct {
	purpose  instance.Purpose
	kid      string
	signedAt time.Time
	validity time.Duration
	msg, sig []byte
}

func TestScheduledRotation_EverySignatureStaysVerifiable(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	k := e.keys(t)
	ctx := context.Background()
	if err := k.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	transitions := 4 // the creations of EnsureActive
	validity := map[instance.Purpose]time.Duration{
		instance.PurposeInvocation:  keyCfg.InvocationTokenTTL + keyCfg.ClockLeeway,
		instance.PurposeAccessToken: keyCfg.AccessTokenTTL + keyCfg.ClockLeeway,
	}
	firstPublished := map[string]time.Time{}
	initial := map[string]bool{}
	for _, r := range e.rows(t) {
		initial[r.kid] = true
	}
	var samples []sample
	invocationKIDs := map[string]bool{}
	accessKIDs := map[string]bool{}
	steps := int((2*keyCfg.InvocationRotation + 3*keyCfg.JWKSMaxAge) / checkInterval)
	for step := range steps {
		if step == 3 {
			rot, err := k.Rotate(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}, instance.RotateCommand{
				Purpose: instance.PurposeAccessToken, Mode: instance.ModeScheduled, Reason: "Scheduled rotation"})
			if err != nil {
				t.Fatalf("Rotate: %v", err)
			}
			if !rot.PreviousNotAfter.Equal(e.clk.Now().Add(keyCfg.JWKSMaxAge)) {
				t.Errorf("previous_not_after %v, want now + agent.jwks_max_age", rot.PreviousNotAfter)
			}
		}
		n, err := k.RunRotation(ctx)
		if err != nil {
			t.Fatalf("step %d: RunRotation: %v", step, err)
		}
		transitions += n
		now := e.clk.Now()
		set, err := k.JWKS(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range set.Keys {
			if _, ok := firstPublished[j.Kid]; !ok {
				firstPublished[j.Kid] = now
			}
		}
		for _, p := range []instance.Purpose{instance.PurposeInvocation, instance.PurposeAccessToken} {
			s, err := k.Signer(ctx, p)
			if err != nil {
				t.Fatalf("step %d: Signer(%s): %v", step, p, err)
			}
			msg := []byte(now.String() + string(p))
			sig, err := s.Sign(ctx, msg)
			if err != nil {
				t.Fatal(err)
			}
			if s.Alg() != "EdDSA" {
				t.Errorf("Alg %q", s.Alg())
			}
			samples = append(samples, sample{purpose: p, kid: s.KeyID(), signedAt: now, validity: validity[p], msg: msg, sig: sig})
			if p == instance.PurposeInvocation {
				invocationKIDs[s.KeyID()] = true
			} else {
				accessKIDs[s.KeyID()] = true
			}
			// TestScheduledRotation_NextPublishedBeforeUse: a key that was not active from the
			// start is in the JWKS at least agent.jwks_max_age before its first signature.
			if pub, ok := firstPublished[s.KeyID()]; !initial[s.KeyID()] && (!ok || now.Sub(pub) < keyCfg.JWKSMaxAge) {
				t.Errorf("step %d: key %s signs %v after it was first published", step, s.KeyID(), now.Sub(pub))
			}
		}
		for _, sm := range samples {
			if now.Sub(sm.signedAt) >= sm.validity {
				continue
			}
			pk, err := k.VerificationKey(ctx, sm.purpose, sm.kid)
			if err != nil {
				t.Fatalf("step %d: a signature of %s from %v, still valid, does not verify: %v", step, sm.kid, sm.signedAt, err)
			}
			if !ed25519.Verify(pk.Key, sm.msg, sm.sig) {
				t.Fatalf("step %d: signature of %s does not verify with its key", step, sm.kid)
			}
		}
		for _, r := range e.rows(t) {
			if r.state != "retired" {
				continue
			}
			if _, err := k.VerificationKey(ctx, instance.Purpose(r.purpose), r.kid); errs.KindOf(err) != errs.KindNotFound {
				t.Fatalf("step %d: retired key %s verifies: %v", step, r.kid, err)
			}
			if _, err := os.Stat(strings.TrimPrefix(r.ref, "file:")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("step %d: the private file of retired key %s still exists", step, r.kid)
			}
		}
		e.clk.Advance(checkInterval)
	}
	if len(invocationKIDs) < 3 {
		t.Errorf("invocation signed with %d keys, want at least three (two complete rotations)", len(invocationKIDs))
	}
	if len(accessKIDs) != 2 {
		t.Errorf("access_token signed with %d keys, want two (one owner-requested rotation)", len(accessKIDs))
	}
	// TestKeyRotationJob_AuditsTransitions: one signing_key.transition entry per transition of
	// the job (and of EnsureActive), none for the owner's rotation, which is signing_key.rotate.
	audits := e.transitionAudits(t)
	if len(audits) != transitions {
		t.Errorf("%d signing_key.transition entries for %d transitions", len(audits), transitions)
	}
	seen := map[string]bool{}
	for _, d := range audits {
		if d["purpose"] == "" || d["from_state"] == "" || d["to_state"] == "" || len(d["kid"]) != 64 {
			t.Errorf("transition entry %v lacks purpose, states or kid", d)
		}
		seen[d["from_state"]+">"+d["to_state"]] = true
	}
	for _, want := range []string{"none>active", "none>next", "next>active", "active>retiring", "retiring>retired"} {
		if !seen[want] {
			t.Errorf("no transition %s audited; seen %v", want, seen)
		}
	}
}

func TestEmergencyRotation_RetiresEveryKeyOfThePurpose(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	a, b := e.keys(t), e.keys(t)
	ctx := context.Background()
	owner := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	if err := a.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	rotate := func(mode instance.RotateMode) instance.Rotation {
		t.Helper()
		r, err := a.Rotate(ctx, owner, instance.RotateCommand{Purpose: instance.PurposeWebhookOut, Mode: mode, Reason: "Test"})
		if err != nil {
			t.Fatalf("Rotate %s: %v", mode, err)
		}
		return r
	}
	// active -> retiring through a scheduled rotation and the job, then a fresh next.
	first := rotate(instance.ModeScheduled)
	if again := rotate(instance.ModeScheduled); again.NewKID != first.NewKID {
		t.Errorf("a second scheduled rotation announced %s, want the pending next %s", again.NewKID, first.NewKID)
	}
	e.clk.Advance(keyCfg.JWKSMaxAge)
	if _, err := a.RunRotation(ctx); err != nil {
		t.Fatal(err)
	}
	second := rotate(instance.ModeScheduled)
	states := map[string]string{}
	for _, r := range e.rows(t) {
		states[r.kid] = r.state
	}
	retiring, active, next := first.PreviousKID, first.NewKID, second.NewKID
	if states[retiring] != "retiring" || states[active] != "active" || states[next] != "next" {
		t.Fatalf("setup states %v", states)
	}
	// b warms its cache with every key.
	for _, kid := range []string{retiring, active, next} {
		if _, err := b.VerificationKey(ctx, instance.PurposeWebhookOut, kid); err != nil {
			t.Fatalf("b: VerificationKey(%s): %v", kid, err)
		}
	}

	em := rotate(instance.ModeEmergency)
	if em.PreviousKID != active || !em.PreviousNotAfter.Equal(e.clk.Now()) {
		t.Errorf("emergency rotation %+v, want previous %s at now", em, active)
	}
	for _, r := range e.rows(t) {
		if r.purpose != "webhook_out" {
			continue
		}
		want := "retired"
		if r.kid == em.NewKID {
			want = "active"
		}
		if r.state != want {
			t.Errorf("key %s is %s after the emergency rotation, want %s", r.kid, r.state, want)
		}
		if want == "retired" {
			if r.notAfter == nil {
				t.Errorf("retired key %s has no not_after", r.kid)
			}
			if _, err := os.Stat(strings.TrimPrefix(r.ref, "file:")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the private file of %s still exists", r.kid)
			}
		}
	}
	for _, kid := range []string{retiring, active, next} {
		if _, err := a.VerificationKey(ctx, instance.PurposeWebhookOut, kid); errs.KindOf(err) != errs.KindNotFound {
			t.Errorf("a: old key %s still verifies in the same process: %v", kid, err)
		}
		if _, err := b.VerificationKey(ctx, instance.PurposeWebhookOut, kid); err != nil {
			t.Errorf("b: within signing_key.cache_ttl the warm cache still answers for %s: %v", kid, err)
		}
	}
	e.clk.Advance(keyCfg.CacheTTL)
	for _, kid := range []string{retiring, active, next} {
		if _, err := b.VerificationKey(ctx, instance.PurposeWebhookOut, kid); errs.KindOf(err) != errs.KindNotFound {
			t.Errorf("b: old key %s still verifies after signing_key.cache_ttl: %v", kid, err)
		}
	}
	s, err := a.Signer(ctx, instance.PurposeWebhookOut)
	if err != nil || s.KeyID() != em.NewKID {
		t.Fatalf("Signer after the emergency rotation: %v, %v; want the new key %s", s, err, em.NewKID)
	}
	sig, err := s.Sign(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []*app.Keys{a, b} {
		pk, err := k.VerificationKey(ctx, instance.PurposeWebhookOut, em.NewKID)
		if err != nil || !ed25519.Verify(pk.Key, []byte("payload"), sig) {
			t.Errorf("the new key does not verify: %v", err)
		}
	}
	if pk, err := a.PublicKey(ctx, active); err != nil || pk.State != instance.KeyRetired {
		t.Errorf("PublicKey of a retired key = %+v, %v; want it in state retired", pk, err)
	}
}

func TestRotate_ValidatesBeforeAnything(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	k := e.keys(t)
	owner := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	for name, cmd := range map[string]instance.RotateCommand{
		"terminal":     {Purpose: "terminal", Mode: instance.ModeScheduled, Reason: "x"},
		"unknown mode": {Purpose: instance.PurposeInvocation, Mode: "later", Reason: "x"},
		"no reason":    {Purpose: instance.PurposeInvocation, Mode: instance.ModeScheduled, Reason: " "},
		"bidi reason":  {Purpose: instance.PurposeInvocation, Mode: instance.ModeScheduled, Reason: "a\u202eb"},
	} {
		if _, err := k.Rotate(context.Background(), owner, cmd); errs.KindOf(err) != errs.KindBadRequest {
			t.Errorf("%s: Rotate = %v, want validation_failed", name, err)
		}
	}
	if n := len(e.files(t)); n != 0 {
		t.Errorf("a refused rotation wrote %d key files", n)
	}
}

func TestVerificationKey_UnknownKidReloadsOncePerTTL(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	a, b := e.keys(t), e.keys(t)
	ctx := context.Background()
	if err := a.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	s, err := a.Signer(ctx, instance.PurposeInvocation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.VerificationKey(ctx, instance.PurposeInvocation, s.KeyID()); err != nil {
		t.Fatal(err)
	}
	if _, err := b.VerificationKey(ctx, instance.PurposeAccessToken, s.KeyID()); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("a key of another purpose verifies: %v", err)
	}
	// An unknown kid reloads once; a new key of another process is found at once.
	if _, err := b.VerificationKey(ctx, instance.PurposeInvocation, "unknown"); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("unknown kid: %v", err)
	}
	em, err := a.Rotate(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}, instance.RotateCommand{
		Purpose: instance.PurposeInvocation, Mode: instance.ModeEmergency, Reason: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.VerificationKey(ctx, instance.PurposeInvocation, em.NewKID); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("within signing_key.cache_ttl of the last miss a new kid reloaded again: %v", err)
	}
	e.clk.Advance(keyCfg.CacheTTL)
	if _, err := b.VerificationKey(ctx, instance.PurposeInvocation, em.NewKID); err != nil {
		t.Errorf("after signing_key.cache_ttl the new kid is unknown: %v", err)
	}
}

// TestEnsureActive_ReplacesKeysWithoutFile: after a restore without the key store an active key
// has no private file; the next start retires it (audited) and creates a fresh active key, so the
// server signs again instead of failing every token (16 section 5.4, "Signierschlüssel" lost).
func TestEnsureActive_ReplacesKeysWithoutFile(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	ctx := context.Background()
	if err := e.keys(t).EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	var lost row
	for _, r := range e.rows(t) {
		if r.purpose == "invocation" {
			lost = r
		}
	}
	if err := os.Remove(strings.TrimPrefix(lost.ref, "file:")); err != nil {
		t.Fatal(err)
	}
	k := e.keys(t) // a new process: no cached secret
	if _, err := k.Signer(ctx, instance.PurposeInvocation); errs.KindOf(err) != errs.KindUnavailable {
		t.Fatalf("Signer without the key file = %v, want unavailable", err)
	}
	if err := k.EnsureActive(ctx); err != nil {
		t.Fatalf("EnsureActive: %v", err)
	}
	states := map[string]string{}
	for _, r := range e.rows(t) {
		states[r.kid] = r.state
	}
	if states[lost.kid] != "retired" || len(states) != 5 {
		t.Fatalf("states after the repair %v, want the lost key retired and one new key", states)
	}
	s, err := k.Signer(ctx, instance.PurposeInvocation)
	if err != nil || s.KeyID() == lost.kid {
		t.Fatalf("Signer after the repair: %v, %v", s, err)
	}
	audits := e.transitionAudits(t)
	var retired, created int
	for _, d := range audits[4:] {
		switch {
		case d["from_state"] == "active" && d["to_state"] == "retired" && d["purpose"] == "invocation":
			retired++
		case d["from_state"] == "none" && d["to_state"] == "active" && d["purpose"] == "invocation":
			created++
		}
	}
	if retired != 1 || created != 1 || len(audits) != 6 {
		t.Errorf("audit entries %v, want one retirement and one creation of invocation after the first four", audits)
	}
	if err := k.EnsureActive(ctx); err != nil || len(e.rows(t)) != 5 {
		t.Errorf("a further EnsureActive changed the keys: %v", err)
	}
}

// TestScheduledRotation_NextPublishedBeforeUse: through one complete scheduled rotation of
// invocation keys, every key that was not active from the start is in the JWKS at least
// agent.jwks_max_age before its first signature.
func TestScheduledRotation_NextPublishedBeforeUse(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	k := e.keys(t)
	ctx := context.Background()
	if err := k.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	first, err := k.Signer(ctx, instance.PurposeInvocation)
	if err != nil {
		t.Fatal(err)
	}
	published := map[string]time.Time{}
	used := map[string]time.Time{}
	for range int((keyCfg.InvocationRotation + 2*keyCfg.JWKSMaxAge) / checkInterval) {
		if _, err := k.RunRotation(ctx); err != nil {
			t.Fatal(err)
		}
		now := e.clk.Now()
		set, err := k.JWKS(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range set.Keys {
			if _, ok := published[j.Kid]; !ok {
				published[j.Kid] = now
			}
		}
		s, err := k.Signer(ctx, instance.PurposeInvocation)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := used[s.KeyID()]; !ok {
			used[s.KeyID()] = now
		}
		e.clk.Advance(checkInterval)
	}
	if len(used) != 2 {
		t.Fatalf("invocation signed with %d keys, want the first and its successor", len(used))
	}
	for kid, at := range used {
		if kid == first.KeyID() {
			continue
		}
		if lead := at.Sub(published[kid]); lead < keyCfg.JWKSMaxAge {
			t.Errorf("key %s was published %v before its first signature, want at least %v", kid, lead, keyCfg.JWKSMaxAge)
		}
	}
}

// TestKeyRotationJob_AuditsTransitions: every transition the job applies leaves exactly one audit
// row signing_key.transition with purpose, both states and the kid; only invocation rotates on
// schedule within this time.
func TestKeyRotationJob_AuditsTransitions(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	k := e.keys(t)
	ctx := context.Background()
	if err := k.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	total := 4
	for range int((keyCfg.InvocationRotation + keyCfg.JWKSMaxAge + time.Hour) / checkInterval) {
		n, err := k.RunRotation(ctx)
		if err != nil {
			t.Fatal(err)
		}
		total += n
		e.clk.Advance(checkInterval)
	}
	audits := e.transitionAudits(t)
	if len(audits) != total || total < 4+4 {
		t.Fatalf("%d audit rows for %d transitions (EnsureActive and the job)", len(audits), total)
	}
	byKID := map[string][]string{}
	for _, d := range audits {
		byKID[d["kid"]] = append(byKID[d["kid"]], d["from_state"]+">"+d["to_state"])
		if d["purpose"] != "invocation" && d["from_state"] != "none" {
			t.Errorf("transition of %s without a schedule: %v", d["purpose"], d)
		}
	}
	var full bool
	for _, seq := range byKID {
		full = full || slices.Equal(seq, []string{"none>active", "active>retiring", "retiring>retired"})
	}
	if !full {
		t.Errorf("no key went through active, retiring and retired: %v", byKID)
	}
}

// lockHook runs before once just before the first LockPurpose of purpose, as a second process
// would act between the read outside the lock and the lock.
type lockHook struct {
	app.KeyRepository
	purpose instance.Purpose
	once    sync.Once
	before  func()
}

func (h *lockHook) LockPurpose(ctx context.Context, p instance.Purpose) error {
	if p == h.purpose {
		h.once.Do(h.before)
	}
	return h.KeyRepository.LockPurpose(ctx, p)
}

// TestEnsureActive_LostNextPromotedConcurrently: the next key of a purpose has lost its file and a
// second process promotes it between the read of EnsureActive and its lock. Under the lock the
// promoted key is lost and no active key is left, although none was generated before; EnsureActive
// must still leave exactly one active key that signs (red team round 1, major 1).
func TestEnsureActive_LostNextPromotedConcurrently(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	ctx := context.Background()
	job := e.keys(t)
	if err := job.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	next, err := job.Rotate(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}, instance.RotateCommand{
		Purpose: instance.PurposeInvocation, Mode: instance.ModeScheduled, Reason: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range e.rows(t) {
		if r.kid == next.NewKID {
			if err := os.Remove(strings.TrimPrefix(r.ref, "file:")); err != nil {
				t.Fatal(err)
			}
		}
	}
	e.clk.Advance(keyCfg.JWKSMaxAge)
	var promoted int
	k := e.keysWith(t, func(r app.KeyRepository) app.KeyRepository {
		return &lockHook{KeyRepository: r, purpose: instance.PurposeInvocation, before: func() {
			n, err := job.RunRotation(ctx)
			if err != nil {
				t.Errorf("concurrent job: %v", err)
			}
			promoted = n
		}}
	})
	if err := k.EnsureActive(ctx); err != nil {
		t.Fatalf("EnsureActive: %v", err)
	}
	if promoted == 0 {
		t.Fatal("the concurrent job promoted nothing; the interleaving did not happen")
	}
	missing, err := k.Unready(ctx)
	if err != nil || len(missing) != 0 {
		t.Fatalf("purposes without an active key after EnsureActive: %v, %v", missing, err)
	}
	s, err := k.Signer(ctx, instance.PurposeInvocation)
	if err != nil {
		t.Fatalf("Signer after the repair: %v", err)
	}
	if s.KeyID() == next.NewKID || s.KeyID() == next.PreviousKID {
		t.Errorf("Signer uses %s, want a fresh key", s.KeyID())
	}
}
