// SPDX-License-Identifier: Apache-2.0

package apns_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// members decodes one JWT segment into its members.
func members(t *testing.T, seg string) map[string]any {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestJWT_HeaderAndClaims(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	if res := w.send(w.phone, w.hint()); res.Class != domain.ResultAccepted {
		t.Fatalf("result %+v", res)
	}
	reqs := w.prod.Requests()
	if len(reqs) != 1 {
		t.Fatalf("%d requests", len(reqs))
	}
	tok := reqs[0].JWT
	j, err := jose.ParseCompactJWS(tok, []string{jose.AlgES256}, "")
	if err != nil {
		t.Fatal(err)
	}
	k, err := jose.JWKFromPublicKey(&w.key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Verify(k); err != nil {
		t.Fatalf("verify: %v", err)
	}
	parts := strings.Split(tok, ".")
	head, claims := members(t, parts[0]), members(t, parts[1])
	if len(head) != 2 || head["alg"] != "ES256" || head["kid"] != keyID {
		t.Errorf("header %v, want exactly alg ES256 and kid", head)
	}
	if len(claims) != 2 || claims["iss"] != teamID || claims["iat"] != float64(start.Unix()) {
		t.Errorf("claims %v, want exactly iss and iat %d", claims, start.Unix())
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		t.Errorf("signature of %d bytes (%v), want 64", len(sig), err)
	}
}

// parallel sends n hints to reg at once and fails on any result but accepted.
func parallel(t *testing.T, w *world, reg app.Registration, n int) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Go(func() {
			res, err := w.a.Send(t.Context(), reg, w.hint())
			if err == nil && res.Class != domain.ResultAccepted {
				err = errors.New("not accepted: " + string(res.Class))
			}
			if err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// distinctJWTs counts the distinct bearer tokens at s.
func distinctJWTs(s *apnstest.Server) int {
	seen := map[string]bool{}
	for _, r := range s.Requests() {
		seen[r.JWT] = true
	}
	return len(seen)
}

func TestJWTCache_AtMostOnePerInterval(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	parallel(t, w, w.phone, 200)
	if n := apns.JWTCreations(w.a, "production"); n != 1 {
		t.Fatalf("%d JWT creations after 200 parallel sends, want 1", n)
	}
	if n := distinctJWTs(w.prod); n != 1 {
		t.Fatalf("%d distinct JWTs at the Fake, want 1", n)
	}
	w.clk.Advance(interval - time.Second)
	w.send(w.phone, w.hint())
	if n := apns.JWTCreations(w.a, "production"); n != 1 {
		t.Fatalf("one second before the interval end: %d creations, want 1", n)
	}
	w.clk.Advance(time.Second)
	w.send(w.phone, w.hint())
	if n := apns.JWTCreations(w.a, "production"); n != 2 {
		t.Fatalf("at the interval end: %d creations, want 2", n)
	}
	parallel(t, w, w.phone, 200)
	if n := apns.JWTCreations(w.a, "production"); n != 2 {
		t.Fatalf("after 200 more parallel sends: %d creations, want 2", n)
	}
	if n := apns.JWTCreations(w.a, "sandbox"); n != 0 {
		t.Fatalf("sandbox created %d JWTs without a send", n)
	}
	w.send(w.mac, w.hint())
	if n := apns.JWTCreations(w.a, "sandbox"); n != 1 {
		t.Fatalf("sandbox: %d creations, want 1", n)
	}
	if n := apns.JWTCreations(w.a, "production"); n != 2 {
		t.Fatalf("production renewed by sandbox: %d creations", n)
	}
}

func TestJWTCache_KeyRotation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.send(w.phone, w.hint())
	rotated := newKey(t)
	writeKey(t, w.keyFile, rotated)
	w.prod.SetPublicKey(&rotated.PublicKey)
	w.clk.Advance(max(interval, cacheTTL))
	if res := w.send(w.phone, w.hint()); res.Class != domain.ResultAccepted {
		t.Fatalf("after the rotation: %+v", res)
	}
	reqs := w.prod.Requests()
	j, err := jose.ParseCompactJWS(reqs[len(reqs)-1].JWT, []string{jose.AlgES256}, "")
	if err != nil {
		t.Fatal(err)
	}
	k, err := jose.JWKFromPublicKey(&rotated.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Verify(k); err != nil {
		t.Fatalf("the JWT after the rotation is not signed with the new key: %v", err)
	}
}

func TestExpiredProviderToken_RenewOnce(t *testing.T) {
	t.Parallel()
	expired := apnstest.Response{Status: 403, Reason: "ExpiredProviderToken"}
	t.Run("renew then accepted", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.prod.Script(w.phone.Token, expired)
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultAccepted {
			t.Fatalf("result %+v", res)
		}
		reqs := w.prod.Requests()
		if len(reqs) != 2 || reqs[0].JWT == reqs[1].JWT {
			t.Fatalf("%d requests, the second must carry a new JWT", len(reqs))
		}
		// A second ExpiredProviderToken within the interval after the replacement: no new JWT.
		w.prod.Script(w.phone.Token, expired)
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError {
			t.Fatalf("second expiry within the interval: %+v", res)
		}
		if n := apns.JWTCreations(w.a, "production"); n != 2 {
			t.Fatalf("%d creations, want 2 (no new JWT)", n)
		}
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError || len(w.prod.Requests()) != 3 {
			t.Fatalf("the environment must pause: %+v, %d requests", res, len(w.prod.Requests()))
		}
	})
	t.Run("expired twice", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.prod.Script(w.phone.Token, expired, expired)
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError {
			t.Fatalf("result %+v", res)
		}
		if n := len(w.prod.Requests()); n != 2 {
			t.Fatalf("%d requests, want 2", n)
		}
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError || len(w.prod.Requests()) != 2 {
			t.Fatalf("paused: %+v, %d requests", res, len(w.prod.Requests()))
		}
	})
	t.Run("ten parallel", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.send(w.phone, w.hint())
		regs := make([]app.Registration, 10)
		for i := range regs {
			regs[i] = w.phone
			regs[i].Token = newToken(t)
			w.prod.Script(regs[i].Token, expired)
		}
		// All ten requests are in flight with the first JWT before any answer comes back.
		w.prod.Hold(len(regs))
		var wg sync.WaitGroup
		results := make(chan domain.SendResult, len(regs))
		for _, r := range regs {
			wg.Go(func() {
				res, err := w.a.Send(t.Context(), r, w.hint())
				if err != nil {
					t.Error(err)
				}
				results <- res
			})
		}
		wg.Wait()
		close(results)
		for res := range results {
			if res.Class != domain.ResultAccepted {
				t.Errorf("result %+v", res)
			}
		}
		if n := apns.JWTCreations(w.a, "production"); n != 2 {
			t.Fatalf("%d creations, want 2 (one replacement)", n)
		}
		if n := len(w.prod.Requests()); n != 21 {
			t.Fatalf("%d requests, want 1 + 10 + 10 retries", n)
		}
	})
}

// errorsOf returns the captured Error records.
func errorsOf(c *capture) []record {
	var out []record
	for _, r := range c.records() {
		if r.Level == slog.LevelError {
			out = append(out, r)
		}
	}
	return out
}

func TestConfigErrorPausesEnvironment(t *testing.T) {
	t.Parallel()
	t.Run("answer", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.prod.Script(w.phone.Token, apnstest.Response{Status: 403, Reason: "InvalidProviderToken"})
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError {
			t.Fatalf("result %+v", res)
		}
		if n := len(errorsOf(w.log)); n != 1 {
			t.Fatalf("%d Error records, want 1", n)
		}
		for range 50 {
			if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError {
				t.Fatalf("paused send: %+v", res)
			}
		}
		if n := len(w.prod.Requests()); n != 1 {
			t.Fatalf("%d requests while paused, want 1", n)
		}
		if n := len(w.log.records()); n != 1 {
			t.Fatalf("%d records while paused, want 1", n)
		}
		if res := w.send(w.mac, w.hint()); res.Class != domain.ResultAccepted {
			t.Fatalf("sandbox: %+v", res)
		}
		w.clk.Advance(interval - time.Second)
		w.send(w.phone, w.hint())
		if n := len(w.prod.Requests()); n != 1 {
			t.Fatalf("a request before pausedUntil")
		}
		w.clk.Advance(time.Second)
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultAccepted {
			t.Fatalf("probe: %+v", res)
		}
		reqs := w.prod.Requests()
		if len(reqs) != 2 || reqs[1].JWT == reqs[0].JWT {
			t.Fatalf("exactly one probe with a new JWT: %d requests", len(reqs))
		}
		var infos int
		for _, r := range w.log.records() {
			if r.Level == slog.LevelInfo {
				infos++
			}
		}
		if infos != 1 {
			t.Fatalf("%d Info records, want 1", infos)
		}
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultAccepted || len(w.prod.Requests()) != 3 {
			t.Fatalf("after the pause: %+v", res)
		}
		// A probe answered BadEnvironmentKeyIdInToken starts the next pause.
		w.prod.Script(w.phone.Token, apnstest.Response{Status: 403, Reason: "Forbidden"})
		w.send(w.phone, w.hint())
		w.clk.Advance(interval)
		w.prod.Script(w.phone.Token, apnstest.Response{Status: 403, Reason: "BadEnvironmentKeyIdInToken"})
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError {
			t.Fatalf("probe: %+v", res)
		}
		before := len(w.prod.Requests())
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError || len(w.prod.Requests()) != before {
			t.Fatalf("the next pause must hold: %+v", res)
		}
	})
	t.Run("probe answered ExpiredProviderToken", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.prod.Script(w.phone.Token, apnstest.Response{Status: 403, Reason: "InvalidProviderToken"})
		w.send(w.phone, w.hint())
		w.clk.Advance(interval)
		w.prod.Script(w.phone.Token, apnstest.Response{Status: 403, Reason: "ExpiredProviderToken"})
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError {
			t.Fatalf("probe: %+v", res)
		}
		if n := len(errorsOf(w.log)); n != 2 {
			t.Fatalf("%d Error records, want 2 (the probe starts the next pause)", n)
		}
		before := len(w.prod.Requests())
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError || len(w.prod.Requests()) != before {
			t.Fatalf("the next pause must hold: %+v", res)
		}
		w.clk.Advance(interval)
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultAccepted || len(w.prod.Requests()) != before+1 {
			t.Fatalf("the probe after the next pause: %+v, %d requests", res, len(w.prod.Requests())-before)
		}
	})
	t.Run("resolve error", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.resolver.setFail(&secrets.RefError{Ref: "file:" + w.keyFile, Err: secrets.ErrNotFound})
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultConfigError {
			t.Fatalf("result %+v", res)
		}
		errs := errorsOf(w.log)
		if len(errs) != 1 || !strings.Contains(errs[0].Message, "push.apns.private_key_ref") {
			t.Fatalf("Error records %+v, want one naming push.apns.private_key_ref", errs)
		}
		if n := len(w.prod.Requests()); n != 0 {
			t.Fatalf("%d requests", n)
		}
		w.send(w.phone, w.hint())
		if n := len(w.log.records()); n != 1 {
			t.Fatalf("%d records while paused, want 1", n)
		}
		w.resolver.setFail(nil)
		w.clk.Advance(interval)
		if res := w.send(w.phone, w.hint()); res.Class != domain.ResultAccepted {
			t.Fatalf("probe after the key is back: %+v", res)
		}
	})
}
