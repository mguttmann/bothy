// SPDX-License-Identifier: Apache-2.0

// Package runstest is test support of the runs module; only tests import it, and the binary
// exclusion test keeps it out of cmd/bothy.
//
// ReferenceVerifier is the reference of the adapter check of docs/spec/09-agents.md 4.5 "Check in
// the adapter" (M2-06, ruling D-M2-06-9). It stands in for an external adapter: it fetches the
// public JWKS (getJwks) through a client the caller passes and checks an invocation token with
// the Go standard library only (encoding/base64 in strict mode, encoding/json, crypto/ed25519),
// independently of internal/kernel/jose and of the server's verifier, so that a defect shared by
// the issuer and the server verifier cannot hide. Later tickets use it as the oracle: M2-08 and
// M2-09 check delivered tokens with it, and the SDK verifiers of M2-29 and M2-30 must agree with it
// on the cases of the M2-06 tests.
//
// Example:
//
//	v, err := runstest.NewReferenceVerifier(runstest.ReferenceOptions{
//		JWKSURL: srv.URL + "/api/v1/instance/jwks", Client: srv.Client(), Clock: clk,
//		Issuer: instanceID, AgentID: agentID, Leeway: leeway, RefreshMinInterval: refresh,
//	})
//	claims, err := v.Check(ctx, token, runstest.ReferenceExpectation{RunID: runID, ActualBillingMode: "organization"})
package runstest

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The results of Check besides success, in the precedence of Design note 6 of M2-06.
var (
	// ErrRefInvalid: points 1, 2, 4 or 6 fail (alg, typ, crit, signature, iss, aud, run, thread,
	// triggering object, payload shape), or iat lies after now plus the leeway. The adapter
	// finishes the run with invalid_invocation_token.
	ErrRefInvalid = errors.New("runstest: invocation token invalid")
	// ErrRefUnknownKey: the kid is missing from the JWKS after at most one forced reload (point
	// 3). The adapter neither accepts nor acknowledges the event.
	ErrRefUnknownKey = errors.New("runstest: invocation token kid unknown")
	// ErrRefBillingModeMismatch: billing_mode differs from the actual mode, or the actual mode
	// is unknown (point 8, 10 section 7).
	ErrRefBillingModeMismatch = errors.New("runstest: billing_mode_mismatch")
	// ErrRefToolClassMismatch: the actual tool class is above tool_class (point 10).
	ErrRefToolClassMismatch = errors.New("runstest: tool_class_mismatch")
	// ErrRefExpired: everything else holds and now is at or after exp plus the leeway (point 5);
	// the adapter fetches a fresh token (4.5.1).
	ErrRefExpired = errors.New("runstest: invocation token expired")
	// ErrRefReplay: the jti was seen before and has not expired (point 7).
	ErrRefReplay = errors.New("runstest: invocation token replayed")
)

// maxJWKSBytes bounds the JWKS body the verifier reads; a test JWKS is far smaller.
const maxJWKSBytes = 1 << 20

// ReferenceOptions configure a ReferenceVerifier. Every option is required: the verifier has no
// default for a configuration key.
type ReferenceOptions struct {
	// JWKSURL is the absolute URL of getJwks.
	JWKSURL string
	// Client fetches the JWKS; the verifier builds no client, transport or dialer.
	Client *http.Client
	// Clock is the time of every check and of the JWKS cache.
	Clock clock.Clock
	// Issuer is the configured instance (iss); AgentID the own agent ID (aud).
	Issuer, AgentID ids.ID
	// Leeway is adapter.invocation_token_leeway, applied to iat and exp (point 5).
	Leeway time.Duration
	// RefreshMinInterval is adapter.jwks_refresh_min_interval: at most one forced reload per
	// interval (point 3).
	RefreshMinInterval time.Duration
}

// ReferenceExpectation is the payload of the event the token came with (point 6) and the actual
// state of the runtime (points 8 and 10). Zero values are not compared.
type ReferenceExpectation struct {
	RunID, ThreadID, MessageID, InteractionID, CommandID ids.ID
	// ActualBillingMode is the mode the runtime reports for this run; "unknown" never matches.
	ActualBillingMode string
	// ActualToolClass is the tool class the runtime actually loaded.
	ActualToolClass string
}

// ReferenceClaims are the claims of a checked token as the verifier read them.
type ReferenceClaims struct {
	Iss, Aud, Sub, Jti                         string
	Iat, Exp                                   int64
	TriggerKind                                string
	RunID, MessageID, InteractionID, CommandID string
	ThreadID, ChannelID, SpaceID               string
	Level, BillingMode, ToolClass              string
	PermissionVersion                          int64
}

// ReferenceVerifier implements the adapter check of 09 4.5. Safe for concurrent use.
type ReferenceVerifier struct {
	o ReferenceOptions

	mu         sync.Mutex
	keys       map[string]ed25519.PublicKey
	loaded     bool
	expires    time.Time
	lastForced time.Time
	forcedOnce bool
	fetches    int
	forced     int
	seen       map[string]time.Time
}

// NewReferenceVerifier returns a verifier over o. Errors: a missing option, a negative leeway or a
// refresh interval that is not positive.
func NewReferenceVerifier(o ReferenceOptions) (*ReferenceVerifier, error) {
	if o.JWKSURL == "" || o.Client == nil || o.Clock == nil || o.Issuer.IsZero() || o.AgentID.IsZero() {
		return nil, errors.New("runstest: JWKSURL, Client, Clock, Issuer and AgentID are required")
	}
	if o.Leeway < 0 || o.RefreshMinInterval <= 0 {
		return nil, errors.New("runstest: Leeway must not be negative and RefreshMinInterval must be positive")
	}
	return &ReferenceVerifier{o: o, seen: map[string]time.Time{}}, nil
}

// Fetches returns how many times the JWKS was fetched, forced reloads included.
func (v *ReferenceVerifier) Fetches() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.fetches
}

// ForcedReloads returns how many fetches a kid missing from the cache caused.
func (v *ReferenceVerifier) ForcedReloads() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.forced
}

// Refresh fetches the JWKS now, as an adapter does at start; it counts as a fetch, not as a forced
// reload. Errors: the fetch failed.
func (v *ReferenceVerifier) Refresh(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.fetch(ctx)
}

// Check runs the adapter check of 09 4.5 on token against e, in the precedence of Design note 6
// of M2-06: points 1 and 2 (alg exactly EdDSA, typ exactly agent-invocation+jwt, no crit), 3 (kid
// known after at most one forced reload, else ErrRefUnknownKey), 4 (signature, iss, aud) and 6
// (run_id, thread_id, triggering object) as ErrRefInvalid; then 8 (ErrRefBillingModeMismatch) and
// 10 (ErrRefToolClassMismatch); then 5 (ErrRefExpired, or ErrRefInvalid for an iat in the future);
// then 7 (ErrRefReplay). Point 9 (allow list of triggerers) is not part of it. A token that
// passes is remembered until exp plus the leeway. Errors: those above, or the error of a JWKS
// fetch.
func (v *ReferenceVerifier) Check(ctx context.Context, token string, e ReferenceExpectation) (ReferenceClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ReferenceClaims{}, fmt.Errorf("%w: not three segments", ErrRefInvalid)
	}
	head, err1 := decodeStrict(parts[0])
	body, err2 := decodeStrict(parts[1])
	sig, err3 := decodeStrict(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return ReferenceClaims{}, fmt.Errorf("%w: segments are not base64url without padding", ErrRefInvalid)
	}
	kid, err := checkHeader(head)
	if err != nil {
		return ReferenceClaims{}, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	key, err := v.key(ctx, kid)
	if err != nil {
		return ReferenceClaims{}, err
	}
	if len(sig) != ed25519.SignatureSize || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return ReferenceClaims{}, fmt.Errorf("%w: signature", ErrRefInvalid)
	}
	c, err := decodeClaims(body)
	if err != nil {
		return ReferenceClaims{}, err
	}
	if err := v.checkClaims(c, e); err != nil {
		return ReferenceClaims{}, err
	}
	if err := v.checkTimeAndReplay(c); err != nil {
		return ReferenceClaims{}, err
	}
	return c, nil
}

// checkTimeAndReplay applies point 5 (iat, then the expiry) and point 7 (the jti store), and
// remembers the jti of a token that passes until exp plus the leeway.
func (v *ReferenceVerifier) checkTimeAndReplay(c ReferenceClaims) error {
	now := v.o.Clock.Now()
	if time.Unix(c.Iat, 0).After(now.Add(v.o.Leeway)) || c.Exp <= c.Iat {
		return fmt.Errorf("%w: iat in the future or exp not after iat", ErrRefInvalid)
	}
	end := time.Unix(c.Exp, 0).Add(v.o.Leeway)
	if !now.Before(end) {
		return ErrRefExpired
	}
	for jti, until := range v.seen {
		if !now.Before(until) {
			delete(v.seen, jti)
		}
	}
	if _, ok := v.seen[c.Jti]; ok {
		return ErrRefReplay
	}
	v.seen[c.Jti] = end
	return nil
}

// checkClaims applies points 4 (iss, aud), 6 (run, thread, triggering object), 8 (billing mode)
// and 10 (tool class) in this order.
func (v *ReferenceVerifier) checkClaims(c ReferenceClaims, e ReferenceExpectation) error {
	if c.Iss != v.o.Issuer.String() || c.Aud != v.o.AgentID.String() {
		return fmt.Errorf("%w: iss or aud", ErrRefInvalid)
	}
	for _, m := range []struct {
		want ids.ID
		got  string
	}{{e.RunID, c.RunID}, {e.ThreadID, c.ThreadID}, {e.MessageID, c.MessageID}, {e.InteractionID, c.InteractionID}, {e.CommandID, c.CommandID}} {
		if !m.want.IsZero() && m.got != m.want.String() {
			return fmt.Errorf("%w: run, thread or triggering object differs from the event", ErrRefInvalid)
		}
	}
	if e.ActualBillingMode != "" && (e.ActualBillingMode == "unknown" || e.ActualBillingMode != c.BillingMode) {
		return ErrRefBillingModeMismatch
	}
	if e.ActualToolClass != "" {
		actual, ok := toolClassRank(e.ActualToolClass)
		claim, _ := toolClassRank(c.ToolClass)
		if !ok || actual > claim {
			return ErrRefToolClassMismatch
		}
	}
	return nil
}

// decodeStrict decodes base64url without padding and refuses non-canonical trailing bits.
func decodeStrict(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty segment")
	}
	return base64.RawURLEncoding.Strict().DecodeString(s)
}

// checkHeader applies points 1 and 2 and returns the kid. It decodes the header member by member
// so that a duplicate member is refused (a parser that kept the first and one that kept the last
// would read different headers).
func checkHeader(head []byte) (string, error) {
	members, err := objectMembers(head)
	if err != nil {
		return "", fmt.Errorf("%w: header", ErrRefInvalid)
	}
	var alg, typ, kid string
	if !stringMember(members, "alg", &alg) || alg != "EdDSA" {
		return "", fmt.Errorf("%w: alg", ErrRefInvalid)
	}
	if !stringMember(members, "typ", &typ) || typ != "agent-invocation+jwt" {
		return "", fmt.Errorf("%w: typ", ErrRefInvalid)
	}
	if _, ok := members["crit"]; ok {
		return "", fmt.Errorf("%w: crit", ErrRefInvalid)
	}
	if !stringMember(members, "kid", &kid) || kid == "" {
		return "", fmt.Errorf("%w: kid", ErrRefInvalid)
	}
	return kid, nil
}

// objectMembers decodes a JSON object into its members and refuses duplicates and trailing data.
func objectMembers(b []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, errors.New("not an object")
	}
	out := map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := tok.(string)
		if !ok {
			return nil, errors.New("member name")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, dup := out[name]; dup {
			return nil, errors.New("duplicate member")
		}
		out[name] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing data")
	}
	return out, nil
}

// stringMember reads the JSON string member name of members into dst.
func stringMember(members map[string]json.RawMessage, name string, dst *string) bool {
	raw, ok := members[name]
	if !ok || len(raw) == 0 || raw[0] != '"' {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

// intMember reads the JSON integer member name of members into dst.
func intMember(members map[string]json.RawMessage, name string, dst *int64) bool {
	raw, ok := members[name]
	if !ok {
		return false
	}
	n, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return false
	}
	*dst = n
	return true
}

// decodeClaims reads the payload. The members every token carries are required; the location
// and triggering members are read when present and compared by point 6.
func decodeClaims(body []byte) (ReferenceClaims, error) {
	members, err := objectMembers(body)
	if err != nil {
		return ReferenceClaims{}, fmt.Errorf("%w: payload", ErrRefInvalid)
	}
	var c ReferenceClaims
	for _, f := range []struct {
		name     string
		dst      *string
		required bool
	}{
		{"iss", &c.Iss, true}, {"aud", &c.Aud, true}, {"sub", &c.Sub, true}, {"jti", &c.Jti, true},
		{"trigger_kind", &c.TriggerKind, true}, {"thread_id", &c.ThreadID, true}, {"channel_id", &c.ChannelID, true},
		{"space_id", &c.SpaceID, true}, {"level", &c.Level, true}, {"billing_mode", &c.BillingMode, true},
		{"tool_class", &c.ToolClass, true}, {"run_id", &c.RunID, false}, {"message_id", &c.MessageID, false},
		{"interaction_id", &c.InteractionID, false}, {"command_id", &c.CommandID, false},
	} {
		if _, present := members[f.name]; !present && !f.required {
			continue
		}
		if !stringMember(members, f.name, f.dst) || *f.dst == "" {
			return ReferenceClaims{}, fmt.Errorf("%w: claim %s", ErrRefInvalid, f.name)
		}
	}
	if !intMember(members, "iat", &c.Iat) || !intMember(members, "exp", &c.Exp) || !intMember(members, "permission_version", &c.PermissionVersion) {
		return ReferenceClaims{}, fmt.Errorf("%w: iat, exp or permission_version", ErrRefInvalid)
	}
	if _, ok := toolClassRank(c.ToolClass); !ok {
		return ReferenceClaims{}, fmt.Errorf("%w: tool_class", ErrRefInvalid)
	}
	return c, nil
}

// toolClassRank orders the tool classes chat < scoped < host (07 section 2.11).
func toolClassRank(s string) (int, bool) {
	switch s {
	case "chat":
		return 0, true
	case "scoped":
		return 1, true
	case "host":
		return 2, true
	}
	return 0, false
}

// key returns the key kid (point 3): from the cache while it is fresh, after a fetch when it
// expired, and after one forced reload per RefreshMinInterval when the kid is missing.
func (v *ReferenceVerifier) key(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	now := v.o.Clock.Now()
	if !v.loaded || !now.Before(v.expires) {
		if err := v.fetch(ctx); err != nil {
			return nil, err
		}
	}
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	if v.forcedOnce && now.Sub(v.lastForced) < v.o.RefreshMinInterval {
		return nil, ErrRefUnknownKey
	}
	v.forcedOnce, v.lastForced = true, now
	v.forced++
	if err := v.fetch(ctx); err != nil {
		return nil, err
	}
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, ErrRefUnknownKey
}

// jwk is one member of the JWKS as the verifier reads it.
type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

// fetch loads the JWKS and caches it for the max-age of Cache-Control (absent: until the next
// use). Unusable members are skipped. Errors: the request, a status other than 200, a body that
// is not a key set.
func (v *ReferenceVerifier) fetch(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.o.JWKSURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/jwk-set+json")
	v.fetches++
	resp, err := v.o.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("runstest: JWKS status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes))
	if err != nil {
		return err
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(raw, &set); err != nil {
		return fmt.Errorf("runstest: JWKS body: %w", err)
	}
	keys := map[string]ed25519.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Kid == "" || (k.Alg != "" && k.Alg != "EdDSA") || (k.Use != "" && k.Use != "sig") {
			continue
		}
		x, err := base64.RawURLEncoding.Strict().DecodeString(k.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			continue
		}
		keys[k.Kid] = ed25519.PublicKey(x)
	}
	v.keys, v.loaded = keys, true
	v.expires = v.o.Clock.Now().Add(maxAge(resp.Header.Get("Cache-Control")))
	return nil
}

// maxAge reads the max-age directive of a Cache-Control value; 0 when absent or broken.
func maxAge(cc string) time.Duration {
	for _, d := range strings.Split(cc, ",") {
		name, val, ok := strings.Cut(strings.TrimSpace(d), "=")
		if !ok || !strings.EqualFold(name, "max-age") {
			continue
		}
		n, err := strconv.ParseInt(strings.Trim(val, `"`), 10, 64)
		if err != nil || n < 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	return 0
}
