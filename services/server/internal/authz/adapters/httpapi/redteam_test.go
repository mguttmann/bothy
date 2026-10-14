// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// anonymous sends a request without credentials; a POST gets an Idempotency-Key when keyed.
func (w *world) anonymous(method, path, body string, keyed bool) *httpservertest.Response {
	w.t.Helper()
	var h []string
	if keyed {
		h = append(h, "Idempotency-Key", ids.New().String())
	}
	return w.h.Do(w.h.NewRequest(method, path, body, h...))
}

// pendingClient logs user in on a device that waits for approval.
func (w *world) pendingClient(user ids.ID) *client {
	w.t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	dev, err := w.id.Devices.Register(context.Background(), identity.RegisterDevice{DeviceID: ids.New(), UserID: user, Name: "New phone",
		Platform: identity.PlatformIOS, DPoPKey: pub, State: identity.DeviceStatePending})
	must(w.t, err)
	set, err := w.id.Sessions.StartSession(context.Background(), identity.StartSession{UserID: user, DeviceID: dev.ID, AuthMethod: identity.AuthPasskey})
	must(w.t, err)
	return &client{w: w, key: key, user: user, session: set.Session.ID, access: set.AccessToken.Reveal()}
}

// TestAnonymousWritesAre401 (red team M1-48, Major 1): an anonymous create of a group or a grant
// is 401 unauthenticated, with and without Idempotency-Key, never 500.
func TestAnonymousWritesAre401(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for _, keyed := range []bool{false, true} {
		w.want(w.anonymous(http.MethodPost, "/api/v1/groups", `{"name":"Team"}`, keyed), http.StatusUnauthorized, "unauthenticated")
		w.want(w.anonymous(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "space", w.support, "read"), keyed), http.StatusUnauthorized, "unauthenticated")
	}
}

// TestNoExistenceOracleBeforeVisibility (red team M1-48, Minor 1; the class of M1-31/16): callers
// that authorize refuses before visibility (anonymous, a pending device) get the same answer for
// an existing and for a random id, on every operation with an id.
func TestNoExistenceOracleBeforeVisibility(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	random := ids.New()
	cases := []struct{ method, existing, missing, body string }{
		{http.MethodGet, groupPath(w.leser), groupPath(random), ""},
		{http.MethodGet, membersPath(w.leser), membersPath(random), ""},
		{http.MethodPatch, groupPath(w.leser), groupPath(random), `{"name":"X"}`},
		{http.MethodDelete, groupPath(w.leser), groupPath(random), ""},
		{http.MethodPost, membersPath(w.leser), membersPath(random), `{"user_id":"` + w.robin.String() + `"}`},
		{http.MethodDelete, memberPath(w.leser, w.sam), memberPath(random, w.sam), ""},
		{http.MethodGet, grantPath(w.g1), grantPath(random), ""},
		{http.MethodPost, revokePath(w.g1), revokePath(random), `{"reason":"x"}`},
		{http.MethodPost, "/api/v1/grants", "/api/v1/grants", grantBody("user", w.sam, "space", w.support, "read") + "|" + grantBody("user", w.sam, "space", random, "read")},
		{http.MethodGet, "/api/v1/grants?target_kind=space&target_id=" + w.support.String(), "/api/v1/grants?target_kind=space&target_id=" + random.String(), ""},
	}
	pending := w.pendingClient(w.robin)
	for _, c := range cases {
		bodyA, bodyB := c.body, c.body
		if i := indexByte(c.body, '|'); i >= 0 {
			bodyA, bodyB = c.body[:i], c.body[i+1:]
		}
		a, b := w.anonymous(c.method, c.existing, bodyA, false), w.anonymous(c.method, c.missing, bodyB, false)
		if a.StatusCode != b.StatusCode || strip(a.Problem(t)) != strip(b.Problem(t)) {
			t.Errorf("anonymous %s %s: %d %s / %d %s", c.method, c.existing, a.StatusCode, a.Body, b.StatusCode, b.Body)
		}
		a, b = pending.call(c.method, c.existing, bodyA), pending.call(c.method, c.missing, bodyB)
		if a.StatusCode != b.StatusCode || strip(a.Problem(t)) != strip(b.Problem(t)) {
			t.Errorf("pending device %s %s: %d %s / %d %s", c.method, c.existing, a.StatusCode, a.Body, b.StatusCode, b.Body)
		}
	}
}

// indexByte returns the index of c in s, or -1.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// TestConcurrentKeyedRevocations (red team M1-48, Major 2): concurrent revocations of one grant,
// each with its own Idempotency-Key, all answer 200 with the stored grant; one revocation event
// is written.
func TestConcurrentKeyedRevocations(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for round := range 4 {
		g := w.grantAs(w.boss.actor(), "user", w.robin, "channel", w.eskalation, "read")
		seq := w.eventSeq()
		var wg sync.WaitGroup
		codes := make([]int, 4)
		for i := range codes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i] = w.boss.call(http.MethodPost, revokePath(g), `{"reason":"cleanup"}`).StatusCode
			}()
		}
		wg.Wait()
		for i, c := range codes {
			if c != http.StatusOK {
				t.Fatalf("round %d call %d: %d", round, i, c)
			}
		}
		n := 0
		for _, e := range w.events(seq) {
			if e.Type == "grant.changed" {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("round %d: %d grant.changed, want 1", round, n)
		}
	}
}
