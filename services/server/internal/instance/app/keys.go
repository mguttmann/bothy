// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Log messages and attribute keys of the signing keys. The keys are those of
// docs/observability/signals.md 5.3 (platform/otel/attr), repeated here because the app layer
// does not import platform; the log handlers drop every key outside that catalog.
const (
	msgKeyTransition    = "signing key transition"
	msgKeyFileNotRemove = "signing key file not removed"
	msgKeyLoadFailed    = "signing key not loadable"
	msgKeyFileMissing   = "signing key file missing, key retired"
	keyLogPurpose       = "bothy.signing_key.purpose"
	keyLogKID           = "bothy.signing_key.kid"
	keyLogFrom          = "bothy.signing_key.from_state"
	keyLogTo            = "bothy.signing_key.to_state"
	keyLogErrorType     = "error.type"
	stateNone           = "none"
)

// The audit attribute names of signing_key.rotate and signing_key.transition (brief M1-32). Key
// ids are written in hex (kidCode), because SEC-AUD-5 codes are lower case.
const (
	attrPurpose     = "purpose"
	attrMode        = "mode"
	attrNewKID      = "new_kid"
	attrPreviousKID = "previous_kid"
	attrReason      = "reason"
	attrFromState   = "from_state"
	attrToState     = "to_state"
	attrKID         = "kid"
)

// KeysConfig are the configuration values of the signing keys (19 section 16, 09 section 4.5);
// internal/wire binds them.
type KeysConfig struct {
	// CacheTTL is signing_key.cache_ttl: the lifetime of the verification cache.
	CacheTTL time.Duration `config:"signing_key.cache_ttl"`
	// InvocationRotation is signing_key.invocation_rotation_interval.
	InvocationRotation time.Duration `config:"signing_key.invocation_rotation_interval"`
	// AccessTokenRotation is signing_key.access_token_rotation_interval.
	AccessTokenRotation time.Duration `config:"signing_key.access_token_rotation_interval"`
	// JWKSMaxAge is agent.jwks_max_age: the lead of a next key and the max-age of the JWKS.
	JWKSMaxAge time.Duration `config:"agent.jwks_max_age"`
	// AccessTokenTTL is auth.access_token_ttl.
	AccessTokenTTL time.Duration `config:"auth.access_token_ttl"`
	// InvocationTokenTTL is agent.invocation_token_ttl.
	InvocationTokenTTL time.Duration `config:"agent.invocation_token_ttl"`
	// WebhookDeliveryTimeout is webhook.delivery_timeout; every delivery attempt signs anew.
	WebhookDeliveryTimeout time.Duration `config:"webhook.delivery_timeout"`
	// ClockLeeway is auth.clock_leeway, the margin of every window.
	ClockLeeway time.Duration `config:"auth.clock_leeway"`
	// TerminalSessionTokenTTL is terminal.session_token_ttl, the validity of a session token of
	// the terminal (24 section 5.3).
	TerminalSessionTokenTTL time.Duration `config:"terminal.session_token_ttl"`
}

// Policy returns the rotation policy of p: the scheduled interval (invocation and access_token
// only), the lead agent.jwks_max_age and the window of the purpose (brief M1-32 section 4).
func (c KeysConfig) Policy(p instance.Purpose) domain.RotationPolicy {
	pol := domain.RotationPolicy{Lead: c.JWKSMaxAge}
	switch p {
	case instance.PurposeAccessToken:
		pol.Interval, pol.Window = c.AccessTokenRotation, domain.Window(c.AccessTokenTTL, c.ClockLeeway)
	case instance.PurposeInvocation:
		pol.Interval, pol.Window = c.InvocationRotation, domain.Window(c.InvocationTokenTTL, c.ClockLeeway)
	case instance.PurposeWebhookOut:
		pol.Window = domain.Window(c.WebhookDeliveryTimeout, c.ClockLeeway)
	case instance.PurposeTerminal:
		// No interval: like webhook_out it rotates on owner request (19 section 10 names none).
		pol.Window = domain.Window(c.TerminalSessionTokenTTL, c.ClockLeeway)
	case instance.PurposeAuditCheckpoint:
		// Zero: old checkpoints are verified through PublicKey in any state.
	}
	return pol
}

// check refuses values the configuration schema cannot produce; tests build configs by hand.
func (c KeysConfig) check() error {
	if c.CacheTTL <= 0 || c.JWKSMaxAge <= 0 || c.InvocationRotation <= 0 || c.AccessTokenRotation <= 0 {
		return errors.New("instance: signing_key.cache_ttl, agent.jwks_max_age and both rotation intervals must be positive")
	}
	if c.AccessTokenTTL < 0 || c.InvocationTokenTTL < 0 || c.WebhookDeliveryTimeout < 0 || c.ClockLeeway < 0 || c.TerminalSessionTokenTTL < 0 {
		return errors.New("instance: token lifetimes and auth.clock_leeway must not be negative")
	}
	return nil
}

// KeysDeps are the collaborators of Keys.
type KeysDeps struct {
	// Repo reads and writes server_signing_keys.
	Repo KeyRepository
	// Store generates, loads and removes private keys in the key store.
	Store KeyStore
	// UoW runs every change; its registry holds the audit hook.
	UoW txn.UnitOfWork
	// Authorizer decides signing_key.rotate.
	Authorizer authorize.Authorizer
	// Audit writes signing_key.rotate and signing_key.transition inside the transaction.
	Audit auditing.Recorder
	// Clock is the time of every transition and of the cache; nil means clock.System().
	Clock clock.Clock
	// Log receives one line per transition; nil discards.
	Log *slog.Logger
}

// Keys implements instance.Keys and runs the job instance.key_rotation (RunRotation). It is safe
// for concurrent use; several instances in one or more processes coordinate through the advisory
// lock of each purpose.
type Keys struct {
	cfg   KeysConfig
	d     KeysDeps
	cache verifyCache
}

// Compile-time check: Keys is the port.
var _ instance.Keys = (*Keys)(nil)

// NewKeys returns the signing keys over deps. Errors: a missing collaborator or a configuration
// value outside its range.
func NewKeys(cfg KeysConfig, deps KeysDeps) (*Keys, error) {
	if deps.Repo == nil || deps.Store == nil || deps.UoW == nil || deps.Authorizer == nil || deps.Audit == nil {
		return nil, errors.New("instance: NewKeys needs repository, key store, unit of work, authorizer and recorder")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if deps.Clock == nil {
		deps.Clock = clock.System()
	}
	if deps.Log == nil {
		deps.Log = slog.New(slog.DiscardHandler)
	}
	return &Keys{cfg: cfg, d: deps}, nil
}

// generated is a key written to the key store before its transaction.
type generated struct {
	row  KeyRow
	used bool
}

// generate writes a new key of p to the key store; the row is filled for state at created.
func (k *Keys) generate(ctx context.Context, p instance.Purpose, state instance.KeyState, created time.Time) (*generated, error) {
	kid, pub, ref, err := k.d.Store.Generate(ctx, p)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, err, "the signing key store cannot be written")
	}
	return &generated{row: KeyRow{
		KID: kid, Purpose: p, Alg: instance.AlgEdDSA, Public: pub, Ref: ref, State: state, CreatedAt: created,
	}}, nil
}

// discard removes the file of a generated key whose row was never committed (best effort).
func (k *Keys) discard(ctx context.Context, g *generated, committed bool) {
	if g == nil || (g.used && committed) {
		return
	}
	k.removeFile(ctx, g.row)
}

// removeFile deletes the private file of row and logs a failure with purpose and kid only.
func (k *Keys) removeFile(ctx context.Context, row KeyRow) {
	if err := k.d.Store.Remove(context.WithoutCancel(ctx), row.Ref); err != nil {
		k.d.Log.WarnContext(ctx, msgKeyFileNotRemove, slog.String(keyLogPurpose, string(row.Purpose)),
			slog.String(keyLogKID, row.KID), slog.String(keyLogErrorType, "keystore"))
	}
}

// change is one transition that was committed, for the log and the file removal after the commit.
type change struct {
	row      KeyRow
	from, to string
}

// apply runs the transitions in the transaction of ctx and returns them as changes. A key that
// left its expected state is a conflict: under the advisory lock that cannot happen.
func (k *Keys) apply(ctx context.Context, keys []KeyRow, trs []domain.Transition) ([]change, error) {
	byKID := map[string]KeyRow{}
	for _, r := range keys {
		byKID[r.KID] = r
	}
	out := make([]change, 0, len(trs))
	for _, tr := range trs {
		ok, err := k.d.Repo.TransitionKey(ctx, tr.KID, instance.KeyState(tr.From), instance.KeyState(tr.To), tr.NotAfter)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errs.New(errs.CodeConflict, "A concurrent change of the signing keys won; send the request again with a new Idempotency-Key.")
		}
		row := byKID[tr.KID]
		row.State = instance.KeyState(tr.To)
		if !tr.NotAfter.IsZero() {
			row.NotAfter = tr.NotAfter
		}
		out = append(out, change{row: row, from: string(tr.From), to: string(tr.To)})
	}
	return out, nil
}

// afterCommit logs the committed changes, removes the private files of retired keys and drops
// the verification cache of this process.
func (k *Keys) afterCommit(ctx context.Context, changes []change) {
	if len(changes) == 0 {
		return
	}
	for _, c := range changes {
		k.d.Log.InfoContext(ctx, msgKeyTransition, slog.String(keyLogPurpose, string(c.row.Purpose)),
			slog.String(keyLogKID, c.row.KID), slog.String(keyLogFrom, c.from), slog.String(keyLogTo, c.to))
		if c.row.State == instance.KeyRetired {
			k.removeFile(ctx, c.row)
		}
	}
	k.cache.invalidate()
}

// recordTransition writes the audit entry of one job-driven transition (system action
// signing_key.transition, no actor, details of codes only, SEC-AUD-5).
func (k *Keys) recordTransition(ctx context.Context, c change) error {
	return k.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionSigningKeyTransition), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance),
		Attrs: auditing.Attrs{
			attrPurpose: auditing.Code(string(c.row.Purpose)), attrFromState: auditing.Code(c.from),
			attrToState: auditing.Code(c.to), attrKID: auditing.Code(KIDCode(c.row.KID)),
		},
	})
}

// KIDCode returns the form of a kid in audit details: the thumbprint bytes in lower-case hex (64
// characters). A kid is base64url with upper-case letters, which audit_details.v1 does not allow
// in a code (SEC-AUD-5); hex is the same SHA-256 value and converts back without loss. A kid that
// is not base64url is returned lower-cased, which only tests produce.
func KIDCode(kid string) string {
	b, err := base64.RawURLEncoding.DecodeString(kid)
	if err != nil {
		return strings.ToLower(kid)
	}
	return hex.EncodeToString(b)
}

// insert inserts a generated key and returns its change from none.
func (k *Keys) insert(ctx context.Context, g *generated) (change, error) {
	if err := k.d.Repo.InsertKey(ctx, g.row); err != nil {
		return change{}, err
	}
	g.used = true
	return change{row: g.row, from: stateNone, to: string(g.row.State)}, nil
}

// EnsureActive creates an active key for every managed purpose without one (instance.Keys). It
// also repairs a purpose whose keys lost their private file (a restore without the key store): it
// retires every next, active or retiring key whose file is missing (domain.LostTransitions) and,
// when no active key is left, creates a fresh one. Each purpose runs in its own unit of work under
// the advisory lock: the key is generated and stored before, inserted only when the purpose still
// needs it, and its file removed otherwise. Every change writes signing_key.transition.
func (k *Keys) EnsureActive(ctx context.Context) error {
	for _, p := range instance.Purposes() {
		if err := k.ensureActive(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// Unready returns the managed purposes that cannot sign, for the readiness check signing_keys:
// without an active key, or with an active key whose private file cannot be read or does not
// belong to it (Signer answers unavailable). Errors: the database error.
func (k *Keys) Unready(ctx context.Context) ([]instance.Purpose, error) {
	var out []instance.Purpose
	for _, p := range instance.Purposes() {
		_, err := k.Signer(ctx, p)
		switch {
		case errs.KindOf(err) == errs.KindUnavailable:
			out = append(out, p)
		case err != nil:
			return nil, err
		}
	}
	return out, nil
}

// lostKeys returns the kids of rows whose private file is missing.
func (k *Keys) lostKeys(ctx context.Context, rows []KeyRow) (map[string]bool, error) {
	lost := map[string]bool{}
	for _, r := range rows {
		if _, err := k.d.Store.Load(ctx, r.Ref); errors.Is(err, ErrKeyMissing) {
			lost[r.KID] = true
		} else if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return lost, nil
}

func (k *Keys) ensureActive(ctx context.Context, p instance.Purpose) error {
	rows, err := k.d.Repo.ListKeys(ctx, p)
	if err != nil {
		return err
	}
	lost, err := k.lostKeys(ctx, rows)
	if err != nil {
		return err
	}
	now := k.d.Clock.Now()
	plan := func(rows []KeyRow) ([]domain.Transition, bool) {
		return domain.LostTransitions(domainKeys(rows), lost, now)
	}
	trs, needActive := plan(rows)
	if len(trs) == 0 && !needActive {
		return nil
	}
	changes, err := k.commitPlan(ctx, p, needActive, instance.KeyActive, now, plan)
	for _, c := range changes {
		if lost[c.row.KID] {
			// A lost key is an operator matter (a restore without the key store, or a role without
			// the key store volume); the line names purpose and kid only.
			k.d.Log.ErrorContext(ctx, msgKeyFileMissing, slog.String(keyLogPurpose, string(p)), slog.String(keyLogKID, c.row.KID))
		}
	}
	return err
}

// planFunc computes, under the lock, the transitions of a purpose from its current rows and
// whether a generated key is to be inserted.
type planFunc func(rows []KeyRow) ([]domain.Transition, bool)

// errKeyNeeded ends a transaction of commitPlan whose plan, recomputed under the lock, asks for a
// new key that was not generated before (another process changed the keys in between).
var errKeyNeeded = errors.New("instance: the plan needs a key that was not generated")

// planAttempts bounds how often commitPlan regenerates after errKeyNeeded; each attempt needs a
// further concurrent change between the read and the lock, so a second one practically never runs.
const planAttempts = 3

// commitPlan runs one change of the keys of p: when generate is set it writes a new key in state
// before the transaction; then, in a unit of work of its own under the advisory lock of p, it
// recomputes the plan from the current rows, applies its transitions, inserts the new key when the
// plan still asks for one, and records one signing_key.transition per change. When the plan under
// the lock asks for a key that was not generated (a concurrent change), it rolls back, generates
// one and runs again. Afterwards it removes an unused key file and runs afterCommit. It returns the
// committed changes.
func (k *Keys) commitPlan(ctx context.Context, p instance.Purpose, generate bool, state instance.KeyState, now time.Time, plan planFunc) ([]change, error) {
	var g *generated
	var changes []change
	var err error
	for range planAttempts {
		if generate && g == nil {
			if g, err = k.generate(ctx, p, state, now); err != nil {
				return nil, err
			}
		}
		err = k.d.UoW.Do(txn.Detach(ctx), func(ctx context.Context) error {
			var err error
			changes, err = k.applyPlan(ctx, p, g, plan)
			return err
		})
		if !errors.Is(err, errKeyNeeded) {
			break
		}
		generate = true
	}
	if errors.Is(err, errKeyNeeded) {
		err = errs.Wrap(errs.CodeConflict, err, "The signing keys changed concurrently; send the request again with a new Idempotency-Key.")
	}
	k.discard(ctx, g, err == nil)
	if err != nil {
		return nil, err
	}
	k.afterCommit(ctx, changes)
	return changes, nil
}

// applyPlan is the transaction of commitPlan.
func (k *Keys) applyPlan(ctx context.Context, p instance.Purpose, g *generated, plan planFunc) ([]change, error) {
	if g != nil {
		g.used = false
	}
	if err := k.d.Repo.LockPurpose(ctx, p); err != nil {
		return nil, err
	}
	rows, err := k.d.Repo.ListKeys(ctx, p)
	if err != nil {
		return nil, err
	}
	trs, insert := plan(rows)
	applied, err := k.apply(ctx, rows, trs)
	if err != nil {
		return nil, err
	}
	if insert {
		if g == nil {
			return nil, errKeyNeeded
		}
		c, err := k.insert(ctx, g)
		if err != nil {
			return nil, err
		}
		applied = append(applied, c)
	}
	for _, c := range applied {
		if err := k.recordTransition(ctx, c); err != nil {
			return nil, err
		}
	}
	return applied, nil
}

// RunRotation is one run of the job instance.key_rotation: for every managed purpose it applies
// the due transitions of domain.DueTransitions at the current time in a unit of work of its own,
// with one signing_key.transition audit entry per transition, and returns how many transitions
// it applied. A next key is generated before the transaction and discarded when the plan inside
// the lock no longer asks for it. The first error stops the run; the job retries.
func (k *Keys) RunRotation(ctx context.Context) (int, error) {
	total := 0
	for _, p := range instance.Purposes() {
		n, err := k.rotatePurpose(ctx, p)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (k *Keys) rotatePurpose(ctx context.Context, p instance.Purpose) (int, error) {
	now := k.d.Clock.Now()
	pol := k.cfg.Policy(p)
	rows, err := k.d.Repo.ListKeys(ctx, p)
	if err != nil {
		return 0, err
	}
	due := domain.DueTransitions(domainKeys(rows), pol, now)
	if due.Empty() {
		return 0, nil
	}
	changes, err := k.commitPlan(ctx, p, due.CreateNext, instance.KeyNext, now, func(rows []KeyRow) ([]domain.Transition, bool) {
		plan := domain.DueTransitions(domainKeys(rows), pol, now)
		return plan.Transitions, plan.CreateNext
	})
	return len(changes), err
}

// domainKeys converts rows for the domain functions.
func domainKeys(rows []KeyRow) []domain.SigningKey {
	out := make([]domain.SigningKey, len(rows))
	for i, r := range rows {
		out[i] = domain.SigningKey{KID: r.KID, State: domain.KeyState(r.State), CreatedAt: r.CreatedAt, NotAfter: r.NotAfter}
	}
	return out
}

// Signer returns a Signer for the active key of p, read fresh (instance.Keys). The loaded private
// key must belong to the public key of the row; a swapped or unreadable file is kind unavailable,
// logged with purpose and kid only.
func (k *Keys) Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error) {
	if !p.Valid() {
		return nil, errs.New(errs.CodeInternalError, fmt.Sprintf("instance: unknown signing key purpose %q", p))
	}
	row, found, err := k.d.Repo.ActiveKey(ctx, p)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errs.New(errs.CodeUnavailable, "No active signing key; the server creates one at start.")
	}
	priv, err := k.d.Store.Load(ctx, row.Ref)
	if err == nil && !priv.Public().Equal(row.Public) {
		err = errors.New("instance: the key file does not belong to the public key of the row")
	}
	if err != nil {
		k.d.Log.ErrorContext(ctx, msgKeyLoadFailed, slog.String(keyLogPurpose, string(p)), slog.String(keyLogKID, row.KID),
			slog.String(keyLogErrorType, "keystore"))
		return nil, errs.Wrap(errs.CodeUnavailable, err, "The signing key cannot be read.")
	}
	return signer{kid: row.KID, key: priv}, nil
}

// signer signs with one loaded key.
type signer struct {
	kid string
	key PrivateKey
}

// KeyID returns the kid.
func (s signer) KeyID() string { return s.kid }

// Alg returns EdDSA.
func (s signer) Alg() string { return instance.AlgEdDSA }

// Sign returns the Ed25519 signature of signingInput.
func (s signer) Sign(ctx context.Context, signingInput []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.key.Sign(signingInput), nil
}

// VerificationKey returns a published key of p from the per-process cache (instance.Keys).
func (k *Keys) VerificationKey(ctx context.Context, p instance.Purpose, kid string) (instance.PublicKey, error) {
	row, found, err := k.cache.get(ctx, k, kid)
	if err != nil {
		return instance.PublicKey{}, err
	}
	if !found || row.Purpose != p || !domain.KeyState(row.State).Published() {
		return instance.PublicKey{}, errs.New(errs.CodeNotFound, "")
	}
	return publicKey(row), nil
}

// PublicKey returns the key kid in any state (instance.Keys).
func (k *Keys) PublicKey(ctx context.Context, kid string) (instance.PublicKey, error) {
	row, found, err := k.d.Repo.KeyByKID(ctx, kid)
	if err != nil {
		return instance.PublicKey{}, err
	}
	if !found {
		return instance.PublicKey{}, errs.New(errs.CodeNotFound, "")
	}
	return publicKey(row), nil
}

// Verify checks a compact JWS against the key of p its header names, in any state (instance.Keys).
func (k *Keys) Verify(ctx context.Context, p instance.Purpose, compactJWS []byte) ([]byte, string, error) {
	j, err := jose.ParseCompactJWS(string(compactJWS), instance.SignatureAlgs(), "")
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", instance.ErrJWSInvalid, err)
	}
	if j.Header.Kid == "" || !p.Valid() {
		return nil, "", fmt.Errorf("%w: no kid or an unmanaged purpose", instance.ErrJWSInvalid)
	}
	row, found, err := k.d.Repo.KeyByKID(ctx, j.Header.Kid)
	if err != nil {
		return nil, "", err
	}
	if !found || row.Purpose != p || row.Alg != j.Header.Alg {
		return nil, "", fmt.Errorf("%w: unknown key for the purpose and algorithm", instance.ErrJWSInvalid)
	}
	jwk, err := jose.JWKFromPublicKey(slices.Clone(row.Public))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", instance.ErrJWSInvalid, err)
	}
	if err := j.Verify(jwk); err != nil {
		return nil, "", fmt.Errorf("%w: %w", instance.ErrJWSInvalid, err)
	}
	return j.Payload, row.KID, nil
}

func publicKey(row KeyRow) instance.PublicKey {
	return instance.PublicKey{
		KID: row.KID, Purpose: row.Purpose, Alg: row.Alg, State: row.State,
		Key: slices.Clone(row.Public), NotAfter: row.NotAfter,
	}
}

// jwksPurposes are the purposes in the JWKS (09 section 4.5, 08 section 7.2); webhook_out keys go
// out through getWebhookKeys (M2), audit_checkpoint keys are never published.
var jwksPurposes = []instance.Purpose{instance.PurposeAccessToken, instance.PurposeInvocation}

// JWKS returns the published key set, read fresh (instance.Keys): the next, active and retiring
// keys of access_token and invocation, ordered by purpose, state (active, next, retiring) and kid.
// It authorizes instance.discovery.read for the principal of ctx (anonymous when there is none),
// a public action, so the proof of the check holds for this route too (02 "Proof of the check").
func (k *Keys) JWKS(ctx context.Context) (instance.JWKSet, error) {
	a, ok := actor.FromContext(ctx)
	if !ok {
		a = actor.Anonymous()
	}
	d, err := k.d.Authorizer.Authorize(ctx, a, authorize.ActionInstanceDiscoveryRead, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return instance.JWKSet{}, err
	}
	if err := d.Err(); err != nil {
		return instance.JWKSet{}, err
	}
	rows, err := k.d.Repo.PublishedKeys(ctx)
	if err != nil {
		return instance.JWKSet{}, err
	}
	stateOrder := map[instance.KeyState]int{instance.KeyActive: 0, instance.KeyNext: 1, instance.KeyRetiring: 2}
	rows = slices.DeleteFunc(rows, func(r KeyRow) bool {
		_, published := stateOrder[r.State]
		return !published || !slices.Contains(jwksPurposes, r.Purpose)
	})
	slices.SortFunc(rows, func(a, b KeyRow) int {
		if c := strings.Compare(string(a.Purpose), string(b.Purpose)); c != 0 {
			return c
		}
		if c := stateOrder[a.State] - stateOrder[b.State]; c != 0 {
			return c
		}
		return strings.Compare(a.KID, b.KID)
	})
	set := instance.JWKSet{Keys: make([]instance.JWK, 0, len(rows))}
	for _, r := range rows {
		set.Keys = append(set.Keys, instance.JWK{
			Kty: "OKP", Crv: "Ed25519", X: base64.RawURLEncoding.EncodeToString(r.Public), Kid: r.KID, Alg: r.Alg, Use: "sig",
		})
	}
	return set, nil
}

// Rotate runs signing_key.rotate (instance.Keys): check the command, authorize (owner with fresh
// step-up), generate the new key before the transaction when one is needed, then in a unit of
// work of its own under the advisory lock apply the rotation and record the allow entry with
// purpose, mode, kids and reason. The unit is detached from an ambient one (the Idempotency-Key
// stage), because the private files of retired keys are deleted after the commit and a file must
// never go for a change that could still roll back (docs/DECISIONS.md M1-32).
func (k *Keys) Rotate(ctx context.Context, a actor.Actor, cmd instance.RotateCommand) (instance.Rotation, error) {
	if err := checkRotate(cmd); err != nil {
		return instance.Rotation{}, err
	}
	ctx = actor.WithActor(ctx, a)
	d, err := k.d.Authorizer.Authorize(ctx, a, authorize.ActionSigningKeyRotate, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return instance.Rotation{}, err
	}
	if err := d.Err(); err != nil {
		return instance.Rotation{}, err
	}
	now := k.d.Clock.Now()
	g, err := k.rotationKey(ctx, cmd, now)
	if err != nil {
		return instance.Rotation{}, err
	}
	var out instance.Rotation
	var changes []change
	err = k.d.UoW.Do(txn.Detach(ctx), func(ctx context.Context) error {
		changes = nil
		if g != nil {
			g.used = false
		}
		var err error
		out, changes, err = k.rotate(ctx, cmd, g, now)
		if err != nil {
			return err
		}
		return k.recordRotation(ctx, cmd, out)
	})
	k.discard(ctx, g, err == nil)
	if err != nil {
		return instance.Rotation{}, err
	}
	k.afterCommit(ctx, changes)
	return out, nil
}

// rotationKey generates the key a rotation needs before its transaction: a fresh active key for
// emergency, a next key for scheduled unless one is pending already; nil when none is needed.
func (k *Keys) rotationKey(ctx context.Context, cmd instance.RotateCommand, now time.Time) (*generated, error) {
	if cmd.Mode == instance.ModeEmergency {
		return k.generate(ctx, cmd.Purpose, instance.KeyActive, now)
	}
	rows, err := k.d.Repo.ListKeys(ctx, cmd.Purpose)
	if err != nil {
		return nil, err
	}
	if _, pending := domain.PendingNext(domainKeys(rows)); pending {
		return nil, nil
	}
	return k.generate(ctx, cmd.Purpose, instance.KeyNext, now)
}

// recordRotation writes the allow entry of signing_key.rotate with purpose, mode, kids and reason.
func (k *Keys) recordRotation(ctx context.Context, cmd instance.RotateCommand, out instance.Rotation) error {
	attrs := auditing.Attrs{
		attrPurpose: auditing.Code(string(cmd.Purpose)), attrMode: auditing.Code(string(cmd.Mode)),
		attrNewKID: auditing.Code(KIDCode(out.NewKID)), attrReason: auditing.ReasonText(cmd.Reason),
	}
	if out.PreviousKID != "" {
		attrs[attrPreviousKID] = auditing.Code(KIDCode(out.PreviousKID))
	}
	return k.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionSigningKeyRotate), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	})
}

// rotate is the transaction of Rotate under the advisory lock of the purpose.
func (k *Keys) rotate(ctx context.Context, cmd instance.RotateCommand, g *generated, now time.Time) (instance.Rotation, []change, error) {
	if err := k.d.Repo.LockPurpose(ctx, cmd.Purpose); err != nil {
		return instance.Rotation{}, nil, err
	}
	rows, err := k.d.Repo.ListKeys(ctx, cmd.Purpose)
	if err != nil {
		return instance.Rotation{}, nil, err
	}
	keys := domainKeys(rows)
	out := instance.Rotation{Purpose: cmd.Purpose}
	if active, ok := domain.ActiveKey(keys); ok {
		out.PreviousKID = active.KID
	}
	if cmd.Mode == instance.ModeScheduled {
		if next, ok := domain.PendingNext(keys); ok {
			out.NewKID, out.PreviousNotAfter = next.KID, next.CreatedAt.Add(k.cfg.JWKSMaxAge)
			return out, nil, nil
		}
		if g == nil {
			// The next key seen before the lock was promoted in the meantime.
			return instance.Rotation{}, nil, errs.New(errs.CodeConflict, "A concurrent rotation of this purpose won; send the request again with a new Idempotency-Key.")
		}
		c, err := k.insert(ctx, g)
		if err != nil {
			return instance.Rotation{}, nil, err
		}
		out.NewKID, out.PreviousNotAfter = g.row.KID, now.Add(k.cfg.JWKSMaxAge)
		return out, []change{c}, nil
	}
	changes, err := k.apply(ctx, rows, domain.EmergencyTransitions(keys, now))
	if err != nil {
		return instance.Rotation{}, nil, err
	}
	c, err := k.insert(ctx, g)
	if err != nil {
		return instance.Rotation{}, nil, err
	}
	out.NewKID, out.PreviousNotAfter = g.row.KID, now
	return out, append(changes, c), nil
}

// checkRotate validates a command before anything else (400 validation_failed).
func checkRotate(cmd instance.RotateCommand) error {
	var fields []errs.FieldError
	if !cmd.Purpose.Valid() {
		fields = append(fields, errs.FieldError{Pointer: "/purpose", Code: "not_supported", Detail: "This purpose has no managed signing key in this server version."})
	}
	if cmd.Mode != instance.ModeScheduled && cmd.Mode != instance.ModeEmergency {
		fields = append(fields, errs.FieldError{Pointer: "/mode", Code: "invalid_value", Detail: "The mode is scheduled or emergency."})
	}
	fields = append(fields, reasonFields(cmd.Reason)...)
	if len(fields) > 0 {
		return errs.New(errs.CodeValidationFailed, "").WithFields(fields...)
	}
	return nil
}

// verifyCache is the per-process cache of the published keys behind VerificationKey: refreshed
// after signing_key.cache_ttl, and an unknown kid reloads at most once per signing_key.cache_ttl.
// The mutex makes concurrent reloads one (singleflight): a caller that waits sees the result of
// the reload before it.
type verifyCache struct {
	mu       sync.Mutex
	keys     map[string]KeyRow
	loadedAt time.Time
	missAt   time.Time
	loaded   bool
}

// get returns the cached published key kid, reloading when the cache expired, or once per TTL for
// an unknown kid.
func (c *verifyCache) get(ctx context.Context, k *Keys, kid string) (KeyRow, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := k.d.Clock.Now()
	ttl := k.cfg.CacheTTL
	expired := !c.loaded || now.Sub(c.loadedAt) >= ttl || now.Before(c.loadedAt)
	if !expired {
		if row, ok := c.keys[kid]; ok {
			return row, true, nil
		}
		if !c.missAt.IsZero() && now.Sub(c.missAt) < ttl && !now.Before(c.missAt) {
			return KeyRow{}, false, nil
		}
		c.missAt = now
	}
	rows, err := k.d.Repo.PublishedKeys(ctx)
	if err != nil {
		return KeyRow{}, false, err
	}
	c.keys = make(map[string]KeyRow, len(rows))
	for _, r := range rows {
		c.keys[r.KID] = r
	}
	c.loaded, c.loadedAt = true, now
	row, ok := c.keys[kid]
	return row, ok, nil
}

// invalidate makes the next get reload, after a transition of this process.
func (c *verifyCache) invalidate() {
	c.mu.Lock()
	c.loaded, c.missAt = false, time.Time{}
	c.mu.Unlock()
}
