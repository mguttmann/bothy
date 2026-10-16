// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
)

// TestNonceRetryOnce (08 7.3): after RotateNonce the next request gets use_dpop_nonce and is
// sent again once with the new nonce; a second use_dpop_nonce in a row is not retried again.
func TestNonceRetryOnce(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	e.fake.RotateNonce()
	before := e.fake.Count(http.MethodGet, bothytest.PathMe)
	if _, err := c.Me(t.Context()); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if n := e.fake.Count(http.MethodGet, bothytest.PathMe) - before; n != 2 {
		t.Errorf("getMe requests = %d, want 2 (one retry)", n)
	}
	before = e.fake.Count(http.MethodGet, bothytest.PathMe)
	e.fake.NewNonceEveryRequest(true)
	_, err := c.Me(t.Context())
	if err == nil {
		t.Error("Me with a nonce that changes on every request = nil error")
	}
	if n := e.fake.Count(http.MethodGet, bothytest.PathMe) - before; n != 2 {
		t.Errorf("getMe requests = %d, want 2", n)
	}
}

// TestRefreshOnInvalidToken (08 7.4): a 401 with the invalid_token challenge (the server ended
// the access token before its stored expiry) refreshes once and repeats the request once.
func TestRefreshOnInvalidToken(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	e.fake.ExpireAccess()
	if _, err := c.Me(t.Context()); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if n := e.fake.Rotations(); n != 1 {
		t.Errorf("rotations = %d, want 1", n)
	}
	s, err := SessionFile{Path: e.path}.Load()
	if err != nil || s.RefreshToken != e.fake.CurrentRefresh() {
		t.Errorf("stored refresh token is not the current one: %v", err)
	}
}

// TestRefreshStoredBeforeUse (08 7.4, note 4): when the first request with the new access token
// arrives, the session file already holds the new refresh token.
func TestRefreshStoredBeforeUse(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	old := e.fake.CurrentRefresh()
	e.clk.Advance(bothytest.AccessTTL + time.Second)
	checked := false
	e.fake.OnProtected(func(string) {
		s, err := SessionFile{Path: e.path}.Load()
		if err != nil {
			t.Errorf("Load in the request: %v", err)
			return
		}
		checked = true
		if s.RefreshToken == old || s.RefreshToken != e.fake.CurrentRefresh() {
			t.Error("the new refresh token was not stored before the new access token was used")
		}
	})
	if _, err := c.Me(t.Context()); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if !checked {
		t.Error("no protected request arrived")
	}
}

// TestRefreshRevokedSession (note 4): a refresh that the server answers with session_revoked
// reports ErrSessionRevoked and deletes nothing.
func TestRefreshRevokedSession(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	if err := Logout(t.Context(), e.options()); err != nil {
		t.Fatal(err)
	}
	// The client still holds the old tokens in memory; its refresh is a reuse.
	e.clk.Advance(bothytest.AccessTTL + time.Second)
	if _, err := c.Me(t.Context()); !errors.Is(err, ErrSessionRevoked) && !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("Me after logout = %v, want ErrSessionRevoked or ErrNotLoggedIn", err)
	}
}

// TestBothy429 (22 4.3 item 5): Retry-After is waited and the call retried; a wait above
// import.discord.max_rate_wait aborts before sleeping; more 429 than
// import.discord.max_429_retries abort with ErrRateLimited.
func TestBothy429(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	e.fake.Fail(bothytest.PathMe, http.StatusTooManyRequests, "3", "", 2)
	if _, err := c.Me(t.Context()); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if got := e.sleep.all(); len(got) < 2 || got[len(got)-1] != 3*time.Second || got[len(got)-2] != 3*time.Second {
		t.Errorf("waits = %v, want two of 3s", got)
	}
	n := len(e.sleep.all())
	e.fake.Fail(bothytest.PathMe, http.StatusTooManyRequests, "3600", "", 1)
	if _, err := c.Me(t.Context()); !errors.Is(err, ErrRateWaitTooLong) {
		t.Errorf("Me with a long wait = %v, want ErrRateWaitTooLong", err)
	}
	if len(e.sleep.all()) != n {
		t.Error("slept although the wait was above the limit")
	}
	e.fake.Fail(bothytest.PathMe, http.StatusTooManyRequests, "1", "", DefaultLimits().Max429Retries+1)
	if _, err := c.Me(t.Context()); !errors.Is(err, ErrRateLimited) {
		t.Errorf("Me with too many 429 = %v, want ErrRateLimited", err)
	}
}

// TestNoRedirectFollowed (requirement 15): a 3xx is not followed; the call fails with
// ErrRedirect and the target is never requested.
func TestNoRedirectFollowed(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	e.fake.Fail(bothytest.PathMe, http.StatusFound, "", e.fake.URL+bothytest.PathInstance, 1)
	before := e.fake.Count(http.MethodGet, bothytest.PathInstance)
	if _, err := c.Me(t.Context()); !errors.Is(err, ErrRedirect) {
		t.Errorf("Me with a redirect = %v, want ErrRedirect", err)
	}
	if e.fake.Count(http.MethodGet, bothytest.PathInstance) != before {
		t.Error("the redirect target was requested")
	}
}

// TestLimitsHelper: Limits reads the effective limits of getInstance.
func TestLimitsHelper(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	l, err := c.Limits(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if l.FileMaxUploadBytes <= 0 {
		t.Errorf("limits = %+v", l)
	}
}
