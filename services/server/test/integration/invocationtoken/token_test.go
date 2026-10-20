// SPDX-License-Identifier: Apache-2.0

package invocationtoken_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
)

// wantReason fails the test unless err is 403 invocation_token_invalid with reason.
func wantReason(t *testing.T, name string, err error, reason string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeInvocationTokenInvalid || e.Reason != reason || errs.KindOf(err) != errs.KindForbidden {
		t.Errorf("%s: %v, want invocation_token_invalid with reason %s", name, err, reason)
	}
}

func TestInvocationToken_RoundTrip(t *testing.T) {
	t.Parallel()
	x := newWorld(t)
	ctx := context.Background()
	got := x.issue(x.a)
	if got.KeyID != x.activeKID() {
		t.Fatalf("kid %s, want the active invocation key %s", got.KeyID, x.activeKID())
	}
	tok := got.Token.Reveal()
	head, err := base64.RawURLEncoding.DecodeString(strings.Split(tok, ".")[0])
	if err != nil {
		t.Fatal(err)
	}
	var h map[string]string
	if err := json.Unmarshal(head, &h); err != nil || h["kid"] != got.KeyID || h["alg"] != "EdDSA" || h["typ"] != "agent-invocation+jwt" || len(h) != 3 {
		t.Fatalf("header %v, %v", h, err)
	}
	c, err := x.a.tokens.Verify(ctx, tok, x.verify())
	if err != nil || c.Jti != got.JTI.String() || c.Iss != x.instanceID.String() || c.Sub != x.alex.String() || c.SpaceID != x.space.String() {
		t.Fatalf("Verify in A: %+v, %v", c, err)
	}
	if _, err := x.b.tokens.Verify(ctx, tok, x.verify()); err != nil {
		t.Fatalf("Verify in B: %v", err)
	}
	rc, err := x.verifier().Check(ctx, tok, x.expect())
	if err != nil || rc.Jti != got.JTI.String() {
		t.Fatalf("reference verifier: %+v, %v", rc, err)
	}
	// Nothing is stored: the token appears in no table the test can read.
	if n := x.w.Count(`SELECT count(*)::int FROM runs WHERE invocation_jti = $1`, got.JTI.String()); n != 0 {
		t.Fatalf("the jti was stored by Issue")
	}
}

func TestInvocationToken_ForeignPurposeKey(t *testing.T) {
	t.Parallel()
	x := newWorld(t)
	ctx := context.Background()
	got := x.issue(x.a)
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(got.Token.Reveal(), ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	sg, err := x.a.keys.Signer(ctx, instance.PurposeAccessToken)
	if err != nil {
		t.Fatal(err)
	}
	sign := jose.SignerFunc{Algorithm: sg.Alg(), Fn: func(in []byte) ([]byte, error) { return sg.Sign(ctx, in) }}
	// The invocation header with valid claims, signed with the access_token key that the same
	// JWKS publishes: the server never verifies it, because the key has another purpose.
	forged := identitytest.SignJWS(sign, jose.Header{Kid: sg.KeyID(), Typ: "agent-invocation+jwt"}, payload)
	for name, p := range map[string]*proc{"A": x.a, "B": x.b} {
		_, err := p.tokens.Verify(ctx, forged, x.verify())
		wantReason(t, "access_token key in "+name, err, "invalid")
	}
	// The access-token header with the access_token key: rejected by both verifiers.
	at := identitytest.SignJWS(sign, jose.Header{Kid: sg.KeyID(), Typ: "at+jwt"}, payload)
	_, err = x.a.tokens.Verify(ctx, at, x.verify())
	wantReason(t, "at+jwt", err, "invalid")
	if _, err := x.verifier().Check(ctx, at, x.expect()); !errors.Is(err, runstest.ErrRefInvalid) {
		t.Fatalf("reference verifier on at+jwt: %v", err)
	}
	// A terminal key is never published and never verifies an invocation token either.
	ts, err := x.a.keys.Signer(ctx, instance.PurposeTerminal)
	if err != nil {
		t.Fatal(err)
	}
	tsign := jose.SignerFunc{Algorithm: ts.Alg(), Fn: func(in []byte) ([]byte, error) { return ts.Sign(ctx, in) }}
	term := identitytest.SignJWS(tsign, jose.Header{Kid: ts.KeyID(), Typ: "agent-invocation+jwt"}, payload)
	_, err = x.a.tokens.Verify(ctx, term, x.verify())
	wantReason(t, "terminal key", err, "invalid")
	if _, err := x.verifier().Check(ctx, term, x.expect()); !errors.Is(err, runstest.ErrRefUnknownKey) {
		t.Fatalf("reference verifier on a terminal key: %v", err)
	}
}

// pending is a token waiting for its check at the last moment of its validity.
type pending struct {
	tok   string
	kid   string
	last  time.Time
	byZ   bool
	label string
}

// TestAG22_ScheduledRotationFailsNoToken is the scheduled half of AG-22 at token level (09 16.2:
// "no failed runs when scheduled"): through two complete rotations of purpose invocation, no token
// fails at three adapters or at a second server process inside its validity, and no adapter needs a
// forced JWKS reload.
func TestAG22_ScheduledRotationFailsNoToken(t *testing.T) {
	t.Parallel()
	x := newWorld(t)
	ctx := context.Background()
	clk := x.w.Clock
	k := x.cfg.Keys
	tick := x.cfg.CheckInterval
	if !(k.CacheTTL < k.JWKSMaxAge && x.tok.TTL+x.tok.ClockLeeway < k.JWKSMaxAge && k.JWKSMaxAge < k.InvocationRotation && tick < x.tok.TTL) {
		t.Fatalf("the test values break the order of the brief: %+v %+v %v", k, x.tok, tick)
	}
	X, Y, Z := x.verifier(), x.verifier(), x.verifier()
	if err := X.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	var queue []pending
	var promotions, lastMomentByFresh int
	zStarted := false
	kids := map[string]bool{}
	expect := x.expect()
	// late checks every token at one second before exp plus the leeway: X, Y and Z have seen it
	// before, so ErrRefReplay (the last point of the check) proves every other point held; B
	// verifies it; a verifier that fetches the JWKS at that moment accepts a token whose key is no
	// longer active.
	late := func(p pending) {
		t.Helper()
		clk.Set(p.last)
		for name, v := range map[string]*runstest.ReferenceVerifier{"X": X, "Y": Y, "Z": Z} {
			if name == "Z" && !p.byZ {
				continue
			}
			if _, err := v.Check(ctx, p.tok, expect); !errors.Is(err, runstest.ErrRefReplay) {
				t.Fatalf("%s at its last valid moment, adapter %s: %v", p.label, name, err)
			}
		}
		if _, err := x.b.tokens.Verify(ctx, p.tok, x.verify()); err != nil {
			t.Fatalf("%s at its last valid moment, process B: %v", p.label, err)
		}
		if p.kid != x.activeKID() {
			w := x.verifier()
			if _, err := w.Check(ctx, p.tok, expect); err != nil || w.ForcedReloads() != 0 {
				t.Fatalf("%s of the old key %s at its last valid moment, fresh adapter: %v (forced %d)", p.label, p.kid, err, w.ForcedReloads())
			}
			lastMomentByFresh++
		}
	}
	for step := 0; step < 400; step++ {
		now := clk.Now()
		sort.Slice(queue, func(i, j int) bool { return queue[i].last.Before(queue[j].last) })
		for len(queue) > 0 && queue[0].last.Before(now.Add(tick)) {
			late(queue[0])
			queue = queue[1:]
		}
		clk.Set(now.Add(tick))
		now = clk.Now()
		before := x.invocationKeys()
		activeBefore := ""
		for _, r := range before {
			if r.state == "active" {
				activeBefore = r.kid
			}
		}
		// X fetches just before the next key is created: the job of this step creates one when
		// the active key is at least the rotation interval old and no next exists.
		if dueNext(before, now, k.InvocationRotation) {
			if err := X.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := x.a.keys.RunRotation(ctx); err != nil {
			t.Fatalf("step %d: RunRotation: %v", step, err)
		}
		after := x.invocationKeys()
		if countState(after, "next") > countState(before, "next") {
			// Y fetches just after the next key was published.
			if err := Y.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
		}
		active := x.activeKID()
		if active != activeBefore {
			promotions++
		}
		got := x.issue(x.a)
		if got.KeyID != active {
			t.Fatalf("step %d: token signed with %s, active is %s", step, got.KeyID, active)
		}
		kids[got.KeyID] = true
		tok := got.Token.Reveal()
		label := fmt.Sprintf("step %d token of %s", step, got.KeyID[:8])
		if promotions > 0 {
			zStarted = true
		}
		for name, v := range map[string]*runstest.ReferenceVerifier{"X": X, "Y": Y, "Z": Z} {
			if name == "Z" && !zStarted {
				continue
			}
			if _, err := v.Check(ctx, tok, expect); err != nil {
				t.Fatalf("%s at issue time, adapter %s: %v", label, name, err)
			}
		}
		if _, err := x.b.tokens.Verify(ctx, tok, x.verify()); err != nil {
			t.Fatalf("%s at issue time, process B: %v", label, err)
		}
		queue = append(queue, pending{tok: tok, kid: got.KeyID, last: got.ExpiresAt.Add(x.tok.ClockLeeway - time.Second), byZ: zStarted, label: label})
		if promotions >= 2 && len(queue) > 0 && countState(after, "retiring") == 0 && countState(after, "next") == 0 {
			break
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i].last.Before(queue[j].last) })
	for _, p := range queue {
		late(p)
	}
	if promotions < 2 || len(kids) < 3 {
		t.Fatalf("%d promotions, %d keys signed; want two complete rotations", promotions, len(kids))
	}
	if lastMomentByFresh < 2 {
		t.Fatalf("%d tokens of an old key were checked at their last valid moment, want at least one per promotion", lastMomentByFresh)
	}
	retired := countState(x.invocationKeys(), "retired")
	if retired < 2 {
		t.Fatalf("%d invocation keys retired, want the two old keys", retired)
	}
	t.Logf("%d promotions, %d keys, %d late checks of an old key by a fresh adapter, fetches X %d Y %d Z %d",
		promotions, len(kids), lastMomentByFresh, X.Fetches(), Y.Fetches(), Z.Fetches())
	for name, v := range map[string]*runstest.ReferenceVerifier{"X": X, "Y": Y, "Z": Z} {
		if v.ForcedReloads() != 0 {
			t.Errorf("adapter %s needed %d forced JWKS reloads", name, v.ForcedReloads())
		}
	}
}

// dueNext reports whether the job at now creates a next key: no next exists and the active key is
// at least interval old.
func dueNext(rows []keyRow, now time.Time, interval time.Duration) bool {
	for _, r := range rows {
		if r.state == "next" {
			return false
		}
	}
	for _, r := range rows {
		if r.state == "active" && now.Sub(r.created) >= interval {
			return true
		}
	}
	return false
}

// countState counts the keys in state.
func countState(rows []keyRow, state string) int {
	n := 0
	for _, r := range rows {
		if r.state == state {
			n++
		}
	}
	return n
}

// TestAG22_EmergencyRotationTokenLevel is the emergency half of AG-22 at token level: after
// signing_key.rotate with mode emergency the old key verifies nothing in A at once and in B after
// signing_key.cache_ttl at the latest; an adapter that fetched after the rotation reports the old
// kid unknown after one forced reload and fetches nothing more within
// adapter.jwks_refresh_min_interval; a new token passes A, B and the adapters, an adapter with an
// old cache after at most one forced reload.
func TestAG22_EmergencyRotationTokenLevel(t *testing.T) {
	t.Parallel()
	x := newWorld(t)
	ctx := context.Background()
	old := x.issue(x.a)
	oldTok := old.Token.Reveal()
	stale := x.verifier()
	if _, err := stale.Check(ctx, oldTok, x.expect()); err != nil {
		t.Fatal(err)
	}
	if _, err := x.b.tokens.Verify(ctx, oldTok, x.verify()); err != nil {
		t.Fatal(err)
	}
	owner := x.w.Person("owner-rotating", true)
	rot, err := x.a.keys.Rotate(ctx, x.w.Actor("owner-rotating"), instance.RotateCommand{Purpose: instance.PurposeInvocation,
		Mode: instance.ModeEmergency, Reason: "Key compromised"})
	if err != nil {
		t.Fatalf("emergency rotation by owner %s: %v", owner, err)
	}
	if rot.PreviousKID != old.KeyID || rot.NewKID == old.KeyID {
		t.Fatalf("rotation %+v", rot)
	}
	_, err = x.a.tokens.Verify(ctx, oldTok, x.verify())
	wantReason(t, "old key in A at once", err, "invalid")
	_, err = x.a.tokens.Verify(ctx, oldTok, runsapp.VerifyInvocation{Audience: x.lotse, RunID: x.run, IgnoreExpiry: true})
	wantReason(t, "old key in A with IgnoreExpiry", err, "invalid")
	x.w.Clock.Advance(x.cfg.Keys.CacheTTL)
	_, err = x.b.tokens.Verify(ctx, oldTok, x.verify())
	wantReason(t, "old key in B after signing_key.cache_ttl", err, "invalid")
	fresh := x.verifier()
	if err := fresh.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.Check(ctx, oldTok, x.expect()); !errors.Is(err, runstest.ErrRefUnknownKey) || fresh.ForcedReloads() != 1 || fresh.Fetches() != 2 {
		t.Fatalf("fresh adapter on the old token: %v, forced %d fetches %d", err, fresh.ForcedReloads(), fresh.Fetches())
	}
	x.w.Clock.Advance(refreshMin - time.Second - x.cfg.Keys.CacheTTL)
	if _, err := fresh.Check(ctx, oldTok, x.expect()); !errors.Is(err, runstest.ErrRefUnknownKey) || fresh.Fetches() != 2 {
		t.Fatalf("again within adapter.jwks_refresh_min_interval: %v, fetches %d", err, fresh.Fetches())
	}
	got := x.issue(x.a)
	if got.KeyID != rot.NewKID {
		t.Fatalf("new token signed with %s, want %s", got.KeyID, rot.NewKID)
	}
	tok := got.Token.Reveal()
	for name, p := range map[string]*proc{"A": x.a, "B": x.b} {
		if _, err := p.tokens.Verify(ctx, tok, x.verify()); err != nil {
			t.Fatalf("new token in %s: %v", name, err)
		}
	}
	if _, err := fresh.Check(ctx, tok, x.expect()); err != nil || fresh.ForcedReloads() != 1 {
		t.Fatalf("fresh adapter on the new token: %v, forced %d", err, fresh.ForcedReloads())
	}
	if _, err := stale.Check(ctx, tok, x.expect()); err != nil || stale.ForcedReloads() > 1 {
		t.Fatalf("adapter with the cache from before the rotation: %v, forced %d", err, stale.ForcedReloads())
	}
}

func TestInvocationToken_LogCanary(t *testing.T) {
	t.Parallel()
	x := newWorld(t)
	ctx := context.Background()
	var tokens []string
	var errTexts []string
	for range 3 {
		got := x.issue(x.a)
		tok := got.Token.Reveal()
		tokens = append(tokens, tok)
		if _, err := x.a.tokens.Verify(ctx, tok, x.verify()); err != nil {
			t.Fatal(err)
		}
		if _, err := x.verifier().Check(ctx, tok, x.expect()); err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(tok, ".")
		for _, bad := range []string{tok + "x", parts[0] + "." + parts[1] + ".AAAA", parts[0] + ".e30." + parts[2]} {
			for _, v := range []runsapp.VerifyInvocation{x.verify(), {Audience: ids.New()}} {
				_, err := x.b.tokens.Verify(ctx, bad, v)
				if err == nil {
					t.Fatal("a broken token verified")
				}
				errTexts = append(errTexts, fmt.Sprintf("%v %+v %#v", err, err, err))
			}
		}
		_, err := x.a.tokens.Verify(ctx, tok, runsapp.VerifyInvocation{Audience: ids.New()})
		errTexts = append(errTexts, fmt.Sprintf("%v %+v", err, err))
		x.w.Clock.Advance(2 * time.Minute)
		_, err = x.a.tokens.Verify(ctx, tok, x.verify())
		errTexts = append(errTexts, fmt.Sprintf("%v %+v", err, err))
		r := x.jwks.Do(x.jwks.NewRequest("GET", "/api/v1/instance/jwks", ""))
		errTexts = append(errTexts, string(r.Body))
	}
	var records []string
	for _, logs := range []*httpservertest.LogCapture{x.logs, x.jwks.Logs, x.w.Logs} {
		for _, r := range logs.Records() {
			records = append(records, fmt.Sprintf("%s %v", r.Message, r.Attrs))
		}
	}
	for _, tok := range tokens {
		for _, s := range append(records, errTexts...) {
			if leaks(s, tok) {
				t.Fatalf("a log record or error contains the token or a segment of it: %.300s", s)
			}
		}
	}
}
