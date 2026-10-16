// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The paths the tool calls without an access token: the proof alone binds them to the key.
var unauthenticated = map[string]bool{
	apiPrefix + "/auth/pairing/begin":    true,
	apiPrefix + "/auth/pairing/complete": true,
	apiPrefix + "/auth/token/refresh":    true,
}

// The DPoP errors of the WWW-Authenticate challenge (08 7.3, RFC 9449 section 7.1).
const (
	challengeUseNonce     = "use_dpop_nonce"
	challengeInvalidToken = "invalid_token"
)

// challengeError matches the error parameter of a DPoP challenge.
var challengeError = regexp.MustCompile(`(?i)^\s*DPoP\b.*\berror="([a-z_]+)"`)

// problemCode matches the problem codes the client repeats in errors: a server could put
// anything into the member, and only a code of this form is safe to print.
var problemCode = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// Log attributes of the transport; records never carry a header, a body or a token.
const (
	attrMethod  = "http.request.method"
	attrPath    = "url.path"
	attrStatus  = "http.response.status_code"
	attrAttempt = "bothy.import.attempt"
	attrWait    = "bothy.import.wait"
)

// RoundTrip sends req through platform/egress with the DPoP proof of the session key (08 7.3):
// the access token (refreshed first when its stored expiry has passed) as Authorization: DPoP
// except on pairing and refresh, a fresh proof with htu of the server origin plus the path, ath
// and the last DPoP-Nonce. It repeats the request once on use_dpop_nonce, refreshes once and
// repeats once on a 401 with the invalid_token challenge, waits Retry-After on 429 within
// Limits, and returns ErrRedirect for a 3xx instead of following it. POST, PUT, PATCH and DELETE
// outside /auth carry one Idempotency-Key per call, kept across the repeats. It never modifies
// req.
func (c *core) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	path := req.URL.Path
	auth := !unauthenticated[path]
	header := req.Header.Clone()
	if header == nil {
		header = http.Header{}
	}
	if isWrite(req.Method) && !strings.HasPrefix(path, apiPrefix+"/auth/") && header.Get("Idempotency-Key") == "" {
		header.Set("Idempotency-Key", ids.New().String())
	}
	var token string
	if auth {
		t, err := c.accessToken(ctx, "")
		if err != nil {
			return nil, err
		}
		token = t
	}
	st := &attemptState{token: token}
	for attempt := 1; ; attempt++ {
		resp, err := c.send(req, header, st.token, attempt)
		if err != nil {
			return nil, err
		}
		retry, err := c.decide(ctx, path, resp, auth, st)
		if err != nil {
			return nil, err
		}
		if !retry {
			return resp, nil
		}
	}
}

// attemptState is what the repeats of one call have used up.
type attemptState struct {
	token        string
	nonceRetried bool
	refreshed    bool
	waits        int
}

// decide reports whether the call is sent again after resp: once on use_dpop_nonce, once after a
// refresh on invalid_token, after a Retry-After wait on 429 within Limits. A 3xx is ErrRedirect.
// The body of a response that is not returned is drained and closed.
func (c *core) decide(ctx context.Context, path string, resp *http.Response, auth bool, st *attemptState) (bool, error) {
	status := resp.StatusCode
	switch {
	case status == http.StatusUnauthorized && challenge(resp) == challengeUseNonce && !st.nonceRetried:
		drain(resp)
		st.nonceRetried = true
		return true, nil
	case status == http.StatusUnauthorized && auth && challenge(resp) == challengeInvalidToken && !st.refreshed:
		drain(resp)
		st.refreshed = true
		token, err := c.accessToken(ctx, st.token)
		st.token = token
		return err == nil, err
	case status == http.StatusTooManyRequests:
		wait := retryAfter(resp, c.clk.Now(), c.limits.MaxRateWait)
		drain(resp)
		if st.waits >= c.limits.Max429Retries {
			return false, ErrRateLimited
		}
		if wait > c.limits.MaxRateWait {
			return false, ErrRateWaitTooLong
		}
		st.waits++
		c.log.LogAttrs(ctx, slog.LevelInfo, "bothy rate limit wait", slog.String(attrPath, path), slog.Duration(attrWait, wait))
		if err := c.sleep(ctx, wait); err != nil {
			return false, err
		}
		return true, nil
	case status >= 300 && status < 400:
		drain(resp)
		return false, ErrRedirect
	}
	return false, nil
}

// send sends one attempt of req with header, a fresh proof and token, and keeps the newest
// DPoP-Nonce of the answer.
func (c *core) send(req *http.Request, header http.Header, token string, attempt int) (*http.Response, error) {
	ctx := req.Context()
	r := req.Clone(ctx)
	r.Header = header.Clone()
	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody == nil {
			if attempt > 1 {
				return nil, errors.New("bothyapi: the request body cannot be sent again")
			}
		} else {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r.Body = body
		}
	}
	c.mu.Lock()
	nonce := c.nonce
	c.mu.Unlock()
	proof, err := c.key.proof(req.Method, proofHTU(c.origin, req.URL.Path), token, nonce, c.clk.Now())
	if err != nil {
		return nil, err
	}
	r.Header.Set("DPoP", proof)
	if token != "" {
		r.Header.Set("Authorization", "DPoP "+token)
	}
	resp, err := c.http.Do(r)
	if err != nil {
		return nil, err
	}
	if n := resp.Header.Get("DPoP-Nonce"); n != "" {
		c.mu.Lock()
		c.nonce = n
		c.mu.Unlock()
	}
	c.log.LogAttrs(ctx, slog.LevelDebug, "bothy request", slog.String(attrMethod, req.Method), slog.String(attrPath, req.URL.Path),
		slog.Int(attrStatus, resp.StatusCode), slog.Int(attrAttempt, attempt))
	return resp, nil
}

// accessToken returns the access token to use. rejected is the token the server just refused
// with invalid_token ("" for none): then, or when the stored expiry has passed, the session is
// refreshed.
func (c *core) accessToken(ctx context.Context, rejected string) (string, error) {
	s := c.session()
	if s == nil {
		return "", ErrNotLoggedIn
	}
	if rejected == "" && c.clk.Now().Before(s.AccessExpiresAt) {
		return s.AccessToken, nil
	}
	stale := rejected
	if stale == "" {
		stale = s.AccessToken
	}
	return c.refresh(ctx, stale)
}

// refresh rotates the token pair (08 7.4, notes 2 and 4) under the file lock: after taking it,
// it re-reads the file, and when another process already rotated the tokens it uses those. A
// successful refresh stores the new pair before it returns the new access token. A 401 means the
// session is revoked (ErrSessionRevoked); nothing is deleted. The tool never retries a refresh
// with an older token.
func (c *core) refresh(ctx context.Context, stale string) (string, error) {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	now := c.clk.Now()
	if cur := c.session(); cur.AccessToken != stale && now.Before(cur.AccessExpiresAt) {
		return cur.AccessToken, nil
	}
	if !c.lockHeld.Load() {
		unlock, err := c.file.Lock()
		if err != nil {
			return "", err
		}
		defer func() { _ = unlock() }()
	}
	disk, err := c.file.Load()
	if err != nil {
		return "", err
	}
	if token, done, err := c.adopt(disk, stale, now); done || err != nil {
		return token, err
	}
	resp, err := c.api.RefreshTokenWithResponse(ctx, &restclient.RefreshTokenParams{},
		restclient.RefreshTokenJSONRequestBody{RefreshToken: disk.RefreshToken})
	if err != nil {
		return "", err
	}
	if resp.StatusCode() == http.StatusUnauthorized {
		return "", ErrSessionRevoked
	}
	if resp.JSON200 == nil {
		return "", statusError("refreshToken", resp.HTTPResponse, resp.Body)
	}
	next := sessionFrom(disk, resp.JSON200, c.clk.Now())
	if err := c.file.Store(next); err != nil {
		return "", err
	}
	c.setSession(next)
	return next.AccessToken, nil
}

// adopt compares the session just read under the lock with the one in memory: another login
// (another device) is ErrSessionRevoked; tokens another process rotated are taken over, and when
// their access token is valid and not the stale one, done is true and it is returned.
func (c *core) adopt(disk *Session, stale string, now time.Time) (token string, done bool, err error) {
	cur := c.session()
	if disk.DeviceID != cur.DeviceID || disk.ServerURL != cur.ServerURL {
		// Another login replaced the session; this client's key is no longer the session's.
		return "", false, ErrSessionRevoked
	}
	if disk.RefreshToken == cur.RefreshToken {
		return "", false, nil
	}
	c.setSession(disk)
	if disk.AccessToken != stale && now.Before(disk.AccessExpiresAt) {
		return disk.AccessToken, true, nil
	}
	return "", false, nil
}

// sessionFrom returns base with the token pair of t, the access expiry counted from now.
func sessionFrom(base *Session, t *restclient.TokenResponse, now time.Time) *Session {
	next := *base
	next.AccessToken = t.AccessToken
	next.AccessExpiresAt = now.Add(time.Duration(t.ExpiresIn) * time.Second)
	next.RefreshToken = t.RefreshToken
	next.RefreshExpiresAt = t.RefreshExpiresAt
	return &next
}

// isWrite reports whether method changes state.
func isWrite(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// challenge returns the error parameter of the DPoP challenge of resp, "" without one.
func challenge(resp *http.Response) string {
	for _, v := range resp.Header.Values("WWW-Authenticate") {
		if m := challengeError.FindStringSubmatch(v); m != nil {
			return strings.ToLower(m[1])
		}
	}
	return ""
}

// retryAfter returns the wait of the Retry-After header of resp: delay seconds or an HTTP date.
// A missing or unparsable header counts as fallback (import.discord.max_rate_wait), so the
// tool waits rather than hammering (22 4.3, D-M8-01-2).
func retryAfter(resp *http.Response, now time.Time, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
		if n > int64(fallback/time.Second) {
			// Above the limit; the caller aborts. Converting n could overflow.
			return fallback + time.Second
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
		return 0
	}
	return fallback
}

// drain reads and closes the body of a response the caller does not get.
func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// statusError returns the *StatusError of an answer of op: its status and its problem code when
// the code has the form of a code.
func statusError(op string, resp *http.Response, body []byte) error {
	e := &StatusError{Op: op}
	if resp != nil {
		e.Status = resp.StatusCode
	}
	var p struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(body, &p) == nil && problemCode.MatchString(p.Code) {
		e.Code = p.Code
	}
	return e
}
