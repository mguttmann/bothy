// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/session"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// brokenKeys is fakeKeys whose Signer and VerificationKey fail as set.
type brokenKeys struct {
	*fakeKeys
	signerErr, verifyErr error
	signErr              error
}

type brokenSigner struct {
	instance.Signer
	err error
}

func (s brokenSigner) Sign(context.Context, []byte) ([]byte, error) { return nil, s.err }

func (k *brokenKeys) Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error) {
	if k.signerErr != nil {
		return nil, k.signerErr
	}
	sg, err := k.fakeKeys.Signer(ctx, p)
	if err != nil || k.signErr == nil {
		return sg, err
	}
	return brokenSigner{Signer: sg, err: k.signErr}, nil
}

func (k *brokenKeys) VerificationKey(ctx context.Context, p instance.Purpose, kid string) (instance.PublicKey, error) {
	if k.verifyErr != nil {
		return instance.PublicKey{}, k.verifyErr
	}
	return k.fakeKeys.VerificationKey(ctx, p, kid)
}

// TestNewAccessTokens_Refuses: a public URL that is no origin, a zero instance id, a TTL not above
// zero, a negative leeway and no key port are errors; a nil clock is the system clock.
func TestNewAccessTokens_Refuses(t *testing.T) {
	t.Parallel()
	keys := newFakeKeys()
	good := app.AccessTokenConfig{InstanceID: ids.New(), PublicURL: testPublicURL, TTL: time.Minute}
	bad := map[string]app.AccessTokenConfig{
		"url":    {InstanceID: good.InstanceID, PublicURL: "chat.example.com/path", TTL: time.Minute},
		"id":     {PublicURL: testPublicURL, TTL: time.Minute},
		"ttl":    {InstanceID: good.InstanceID, PublicURL: testPublicURL},
		"leeway": {InstanceID: good.InstanceID, PublicURL: testPublicURL, TTL: time.Minute, Leeway: -time.Second},
	}
	for name, cfg := range bad {
		if _, err := app.NewAccessTokens(cfg, keys, nil); err == nil {
			t.Errorf("%s: NewAccessTokens passed", name)
		}
	}
	if _, err := app.NewAccessTokens(good, nil, nil); err == nil {
		t.Error("NewAccessTokens without keys passed")
	}
	if _, err := app.NewAccessTokens(good, keys, nil); err != nil {
		t.Errorf("NewAccessTokens with a nil clock: %v", err)
	}
}

// TestAccessTokens_KeyPortErrors: errors of the signing key port come back from Issue, a failed
// signature is internal unless it already is a kernel error, and a database error of
// VerificationKey comes back from Verify unchanged.
func TestAccessTokens_KeyPortErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newTokenEnv(t)
	raw, _, err := e.at.Issue(ctx, e.record(), "j")
	if err != nil {
		t.Fatal(err)
	}
	unavailable := errs.New(errs.CodeUnavailable, "no active key")
	cases := map[string]struct {
		keys *brokenKeys
		want errs.Code
	}{
		"signer":      {&brokenKeys{fakeKeys: e.keys, signerErr: unavailable}, errs.CodeUnavailable},
		"sign kernel": {&brokenKeys{fakeKeys: e.keys, signErr: unavailable}, errs.CodeUnavailable},
		"sign plain":  {&brokenKeys{fakeKeys: e.keys, signErr: errors.New("key store")}, errs.CodeInternalError},
	}
	for name, c := range cases {
		at, err := app.NewAccessTokens(app.AccessTokenConfig{InstanceID: e.iss, PublicURL: testPublicURL, TTL: time.Minute}, c.keys, e.clk)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = at.Issue(ctx, e.record(), "j")
		if ee, ok := errs.As(err); !ok || ee.Code != c.want {
			t.Errorf("%s: Issue = %v, want code %s", name, err, c.want)
		}
	}
	dbErr := errors.New("database")
	at, err := app.NewAccessTokens(app.AccessTokenConfig{InstanceID: e.iss, PublicURL: testPublicURL, TTL: time.Minute},
		&brokenKeys{fakeKeys: e.keys, verifyErr: dbErr}, e.clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := at.Verify(ctx, raw); !errors.Is(err, dbErr) {
		t.Errorf("Verify with a failing key port = %v, want the database error", err)
	}
}

// TestAccessTokens_RefusesCraftedTokens: a token without kid and a validly signed token whose
// subject is no id are 401 unauthenticated.
func TestAccessTokens_RefusesCraftedTokens(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newTokenEnv(t)
	_, c, err := e.at.Issue(ctx, e.record(), "j")
	if err != nil {
		t.Fatal(err)
	}
	priv := e.keys.priv[e.keys.active]
	sign := jose.SignerFunc{Algorithm: instance.AlgEdDSA, Fn: func(in []byte) ([]byte, error) { return ed25519.Sign(priv, in), nil }}
	craft := func(kid string, claims session.Claims) string {
		payload, err := json.Marshal(claims)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := jose.SignCompact(sign, jose.Header{Typ: "at+jwt", Kid: kid}, payload)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	badSub := c
	badSub.Subject = "not-an-id"
	for name, raw := range map[string]string{"no kid": craft("", c), "subject": craft(e.keys.active, badSub)} {
		if _, err := e.at.Verify(ctx, raw); codeOfErr(err) != "unauthenticated" {
			t.Errorf("%s: Verify = %v, want unauthenticated", name, err)
		}
	}
	if _, err := e.at.Verify(ctx, craft(e.keys.active, c)); err != nil {
		t.Errorf("the crafted token with the issued claims did not verify: %v", err)
	}
}
