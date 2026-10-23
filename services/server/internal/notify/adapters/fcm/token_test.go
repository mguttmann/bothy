// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

func TestAssertion_HeaderAndClaims(t *testing.T) {
	w := newWorld(t)
	sa, err := ParseServiceAccount(secrets.NewValue(w.fake.ServiceAccountJSON(w.key)), w.fake.TokenURL())
	if err != nil {
		t.Fatal(err)
	}
	tok, err := signAssertion(sa, w.fake.TokenURL(), w.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("assertion has %d parts", len(parts))
	}
	segment := func(i int) map[string]any {
		b, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	wantHead := map[string]any{"alg": "RS256", "typ": "JWT", "kid": testKeyID}
	if h := segment(0); !reflect.DeepEqual(h, wantHead) {
		t.Fatalf("header %v", h)
	}
	iat := float64(w.clk.Now().Unix())
	wantClaims := map[string]any{"iss": testEmail, "scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud": w.fake.TokenURL(), "iat": iat, "exp": iat + 3600}
	if c := segment(1); !reflect.DeepEqual(c, wantClaims) {
		t.Fatalf("claims %v", c)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&w.key.PublicKey, crypto.SHA256, sum[:], sig); err != nil {
		t.Fatalf("signature: %v", err)
	}
	// Fake-FCM accepts only exactly this header, these claims and a form of exactly grant_type
	// and assertion: a send that gets a token proves the request it received.
	if res := w.send(w.sam, w.hint()); res.Class != domain.ResultAccepted {
		t.Fatalf("send = %+v", res)
	}
	if w.fake.TokenRequests() != 1 {
		t.Fatalf("token requests %d", w.fake.TokenRequests())
	}
}

// parallel sends n hints to Sam's phone at once and fails on any result other than accepted.
func (w *world) parallel(n int) {
	w.t.Helper()
	var wg sync.WaitGroup
	errs := make(chan string, n)
	for range n {
		wg.Go(func() {
			res, err := w.a.Send(context.Background(), w.sam, w.hint())
			if err != nil || res.Class != domain.ResultAccepted {
				errs <- string(res.Class) + " " + errString(err)
			}
		})
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		w.t.Fatalf("parallel send: %s", e)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestTokenCache_OnePerHalfLifetime(t *testing.T) {
	w := newWorld(t)
	w.parallel(200)
	if n := w.fake.TokenRequests(); n != 1 {
		t.Fatalf("token requests after 200 parallel sends: %d", n)
	}
	first := w.fake.Requests()[0].Bearer
	for _, r := range w.fake.Requests() {
		if r.Bearer != first {
			t.Fatal("a send used another token")
		}
	}
	w.clk.Advance(lifetime/2 - time.Second)
	w.send(w.sam, w.hint())
	if n := w.fake.TokenRequests(); n != 1 {
		t.Fatalf("token requests one second before half the lifetime: %d", n)
	}
	w.clk.Advance(time.Second)
	w.send(w.sam, w.hint())
	if n := w.fake.TokenRequests(); n != 2 {
		t.Fatalf("token requests at half the lifetime: %d", n)
	}
	w.parallel(200)
	if n := w.fake.TokenRequests(); n != 2 {
		t.Fatalf("token requests after 200 further sends: %d", n)
	}
}

func TestTokenCache_KeyRotation(t *testing.T) {
	w := newWorld(t)
	w.send(w.sam, w.hint())
	next := newRSAKey(t)
	w.writeAccount(next)
	w.fake.SetPublicKey(&next.PublicKey)
	w.clk.Advance(max(cacheTTL, lifetime/2))
	if res := w.send(w.sam, w.hint()); res.Class != domain.ResultAccepted {
		t.Fatalf("send after rotation = %+v", res)
	}
	if n := w.fake.TokenRequests(); n != 2 {
		t.Fatalf("token requests %d", n)
	}
}

func TestTokenRenewOn401(t *testing.T) {
	w := newWorld(t)
	w.send(w.sam, w.hint())
	w.fake.RevokeTokens()
	sends := len(w.fake.Requests())
	if res := w.send(w.sam, w.hint()); res.Class != domain.ResultAccepted {
		t.Fatalf("send after revocation = %+v", res)
	}
	reqs := w.fake.Requests()
	// The rejected request is not recorded by the Fake (401 comes before recording), so the
	// retry is the one new recorded request; the token count shows the renewal.
	if len(reqs) != sends+1 || w.fake.TokenRequests() != 2 {
		t.Fatalf("send requests %d, token requests %d", len(reqs)-sends, w.fake.TokenRequests())
	}
	if reqs[len(reqs)-1].Bearer == reqs[0].Bearer {
		t.Fatal("retry used the revoked token")
	}

	// Revoked again right after the renewal inside the same send: configuration error, pause.
	w2 := newWorld(t)
	w2.send(w2.sam, w2.hint())
	w2.fake.RevokeTokens()
	revoking := &revokeAfterToken{fake: w2.fake}
	w2.a.tokens.onToken = revoking.after
	if res := w2.send(w2.sam, w2.hint()); res.Class != domain.ResultConfigError {
		t.Fatalf("second 401 = %+v", res)
	}
	tokens := w2.fake.TokenRequests()
	if res := w2.send(w2.sam, w2.hint()); res.Class != domain.ResultConfigError || w2.fake.TokenRequests() != tokens {
		t.Fatalf("send in the pause = %+v, token requests %d", res, w2.fake.TokenRequests()-tokens)
	}

	// Ten parallel sends that all get 401 for the same token create exactly one replacement.
	w3 := newWorld(t)
	w3.send(w3.sam, w3.hint())
	w3.fake.RevokeTokens()
	w3.parallel(10)
	if n := w3.fake.TokenRequests(); n != 2 {
		t.Fatalf("token requests after ten 401: %d", n)
	}
}

// revokeAfterToken revokes every issued token right after the adapter obtained one.
type revokeAfterToken struct {
	fake *fcmtest.Server
}

func (r *revokeAfterToken) after() { r.fake.RevokeTokens() }

func TestConfigErrorPauses(t *testing.T) {
	t.Run("403 PERMISSION_DENIED", func(t *testing.T) {
		w := newWorld(t)
		w.fake.Script(w.sam.Token, fcmtest.Response{Status: 403, RPCStatus: "PERMISSION_DENIED"})
		w.checkPause(func() { w.send(w.sam, w.hint()) }, "PERMISSION_DENIED")
	})
	t.Run("token refused with invalid_grant", func(t *testing.T) {
		w := newWorld(t)
		w.fake.ScriptToken(fcmtest.TokenResponse{Status: 400, Error: "invalid_grant"})
		w.checkPause(func() {
			if res := w.send(w.sam, w.hint()); res.Class != domain.ResultConfigError {
				t.Fatalf("refused token = %+v", res)
			}
		}, "invalid_grant")
	})
	t.Run("service account missing", func(t *testing.T) {
		w := newWorld(t)
		w.resolver.setFail(fmt.Errorf("file: %w", secrets.ErrNotFound))
		w.checkPause(func() {
			if res := w.send(w.sam, w.hint()); res.Class != domain.ResultConfigError {
				t.Fatalf("missing account = %+v", res)
			}
			w.resolver.setFail(nil)
		}, "not_found")
		recs := w.log.records()
		if !strings.Contains(recs[0].Message, "push.fcm.service_account_ref") {
			t.Fatalf("record %q does not name the key", recs[0].Message)
		}
	})
	for _, c := range []fcmtest.Response{
		{Status: 403, RPCStatus: "PERMISSION_DENIED", ErrorCode: "SENDER_ID_MISMATCH"},
		{Status: 401, RPCStatus: "UNAUTHENTICATED", ErrorCode: "THIRD_PARTY_AUTH_ERROR"},
	} {
		t.Run(c.ErrorCode+" does not pause", func(t *testing.T) {
			w := newWorld(t)
			w.fake.Script(w.sam.Token, c)
			w.send(w.sam, w.hint())
			sends := len(w.fake.Requests())
			if res := w.send(w.robin, w.hint()); res.Class != domain.ResultAccepted || len(w.fake.Requests()) != sends+1 {
				t.Fatalf("Robin's send = %+v", res)
			}
		})
	}
}

// checkPause runs trigger, which must start a pause with one Error record of errType, then
// checks the pause and the probe of Design note 4 c.
func (w *world) checkPause(trigger func(), errType string) {
	t := w.t
	t.Helper()
	trigger()
	if n := w.log.count(slog.LevelError); n != 1 {
		t.Fatalf("Error records after the trigger: %d", n)
	}
	if got := w.log.records()[0].Attrs["error.type"]; got != errType {
		t.Fatalf("error.type %q, want %q", got, errType)
	}
	sends, tokens := len(w.fake.Requests()), w.fake.TokenRequests()
	for range 50 {
		if res := w.send(w.sam, w.hint()); res.Class != domain.ResultConfigError {
			t.Fatalf("send in the pause = %+v", res)
		}
	}
	w.clk.Advance(pauseFor - time.Second)
	if res := w.send(w.robin, w.hint()); res.Class != domain.ResultConfigError {
		t.Fatalf("send one second before the end = %+v", res)
	}
	if len(w.fake.Requests()) != sends || w.fake.TokenRequests() != tokens || len(w.log.records()) != 1 {
		t.Fatalf("the pause sent %d, asked %d tokens, logged %d", len(w.fake.Requests())-sends, w.fake.TokenRequests()-tokens, len(w.log.records()))
	}
	w.clk.Advance(time.Second)
	if res := w.send(w.sam, w.hint()); res.Class != domain.ResultAccepted {
		t.Fatalf("probe = %+v", res)
	}
	if w.fake.TokenRequests() != tokens+1 || len(w.fake.Requests()) != sends+1 {
		t.Fatalf("probe: %d token requests, %d sends", w.fake.TokenRequests()-tokens, len(w.fake.Requests())-sends)
	}
	recs := w.log.records()
	if len(recs) != 2 || recs[1].Level != slog.LevelInfo {
		t.Fatalf("records after the probe: %+v", recs)
	}
	if res := w.send(w.robin, w.hint()); res.Class != domain.ResultAccepted {
		t.Fatalf("send after the pause = %+v", res)
	}
}
