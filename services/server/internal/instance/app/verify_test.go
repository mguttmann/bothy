// SPDX-License-Identifier: Apache-2.0

//go:build unix

package app_test

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// signWith returns a compact JWS of payload signed by the active key of p.
func signWith(t *testing.T, keys instance.Keys, p instance.Purpose, payload string) (string, string) {
	t.Helper()
	ctx := context.Background()
	s, err := keys.Signer(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := jose.SignCompact(jose.SignerFunc{Algorithm: s.Alg(), Fn: func(in []byte) ([]byte, error) { return s.Sign(ctx, in) }},
		jose.Header{Typ: "audit-checkpoint+json", Kid: s.KeyID()}, []byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	return tok, s.KeyID()
}

// TestVerify_AcceptsEveryStateOfThePurpose: Keys.Verify checks a compact JWS against the key its
// header names, in any state of the purpose including retired (old audit checkpoints, M1-50),
// and refuses another purpose, an unknown or missing kid, a changed payload or signature and the
// algorithm none.
func TestVerify_AcceptsEveryStateOfThePurpose(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	k := e.keys(t)
	ctx := context.Background()
	if err := k.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	const payload = `{"seq":1}`
	tok, kid := signWith(t, k, instance.PurposeAuditCheckpoint, payload)
	got, gotKID, err := k.Verify(ctx, instance.PurposeAuditCheckpoint, []byte(tok))
	if err != nil || string(got) != payload || gotKID != kid {
		t.Fatalf("Verify = %q, %q, %v; want the payload and %s", got, gotKID, err, kid)
	}

	owner := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	if _, err := k.Rotate(ctx, owner, instance.RotateCommand{Purpose: instance.PurposeAuditCheckpoint, Mode: instance.ModeEmergency, Reason: "Test"}); err != nil {
		t.Fatal(err)
	}
	if pk, err := k.PublicKey(ctx, kid); err != nil || pk.State != instance.KeyRetired {
		t.Fatalf("the old key is %+v, %v; want retired", pk, err)
	}
	if got, _, err := k.Verify(ctx, instance.PurposeAuditCheckpoint, []byte(tok)); err != nil || string(got) != payload {
		t.Errorf("a JWS of a retired key does not verify: %v", err)
	}

	other, _ := signWith(t, k, instance.PurposeWebhookOut, payload)
	parts := strings.Split(tok, ".")
	changed := parts[0] + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"seq":2}`)) + "." + parts[2]
	unknown := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","kid":"unknown"}`)) + "." + parts[1] + "." + parts[2]
	noKID := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA"}`)) + "." + parts[1] + "." + parts[2]
	none := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","kid":"`+kid+`"}`)) + "." + parts[1] + "."
	sig := []byte(parts[2])
	sig[0] ^= 1
	badSig := parts[0] + "." + parts[1] + "." + string(sig)
	for name, s := range map[string]string{
		"another purpose": other, "changed payload": changed, "unknown kid": unknown, "no kid": noKID,
		"alg none": none, "changed signature": badSig, "not a JWS": "abc",
	} {
		if _, _, err := k.Verify(ctx, instance.PurposeAuditCheckpoint, []byte(s)); !errors.Is(err, instance.ErrJWSInvalid) {
			t.Errorf("%s: Verify = %v, want instance.ErrJWSInvalid", name, err)
		}
	}
	if _, _, err := k.Verify(ctx, instance.Purpose("terminal"), []byte(tok)); !errors.Is(err, instance.ErrJWSInvalid) {
		t.Errorf("unmanaged purpose: %v", err)
	}
}
