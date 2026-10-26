// SPDX-License-Identifier: Apache-2.0

// Package demo is bothy setup demo (docs/spec/16-operations.md 16, brief M7-27): it loads the
// data set of the demo instance and turns an empty instance into the demo through the public API
// of the running server, in the sessions of the people of the data set. It imports no server
// module; the only server package it uses is the generated contract api/rest.
package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// apiPrefix is the path prefix of the REST API (docs/api/openapi.yaml servers).
const apiPrefix = "/api/v1"

// Bounds of one exchange with the server: the largest answer the command reads, the time limit
// of one request, and the retries after 429 (06 "Rate limits"), each wait at most maxRetryAfter.
const (
	maxBody             = 8 << 20
	requestTimeout      = time.Minute
	maxRateLimitRetries = 20
	maxRetryAfter       = 2 * time.Minute
)

// StatusError is a non-2xx answer: the operation, the HTTP status, the problem code and, for a
// validation error, the code and pointer of the first field error (06 "Errors"). It never holds
// the body, which may echo a value.
type StatusError struct {
	Operation string
	Status    int
	Code      string
	Field     string
}

// Error returns operation, status and codes.
func (e *StatusError) Error() string {
	s := fmt.Sprintf("demo: %s answered %d %s", e.Operation, e.Status, e.Code)
	if e.Field != "" {
		s += " (" + e.Field + ")"
	}
	return s
}

// transport is what the sessions of one run share: the address the command dials, the public URL
// the proofs name and the HTTP client, which uses no proxy and follows no redirect, so the
// command reaches only the server.
type transport struct {
	base   string
	public *url.URL
	hc     *http.Client
	sleep  func(ctx context.Context, d time.Duration) error
}

// newTransport returns the transport for apiURL (for example http://127.0.0.1:8080) and the
// public URL (server.public_url). The error names an unusable URL.
func newTransport(apiURL, publicURL string, sleep func(ctx context.Context, d time.Duration) error) (*transport, error) {
	if u, err := url.Parse(apiURL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" {
		return nil, errors.New("demo: the API address must be an http or https URL")
	}
	public, err := publicOrigin(publicURL)
	if err != nil {
		return nil, err
	}
	// The command is a client of the operator's own instance (--api, loopback by default), not an
	// outgoing connection of the server, so the egress checks of SEC-NET do not apply.
	hc := &http.Client{ // nosemgrep: bothy-go-egress-only -- client of the own instance at --api, no server egress
		Transport:     &http.Transport{Proxy: nil, ForceAttemptHTTP2: false}, // nosemgrep: bothy-go-egress-only -- client of the own instance at --api, no server egress
		Timeout:       requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &transport{base: strings.TrimSuffix(apiURL, "/"), public: public, hc: hc, sleep: sleep}, nil
}

// session is one device session of a person: its DPoP key, its step-up key, the access token and
// the newest DPoP-Nonce. Keys and token live in memory only (16 16.2). It is safe for concurrent
// use.
type session struct {
	tr     *transport
	dpop   key
	stepUp key

	mu        sync.Mutex
	token     string
	refresh   string
	sessionID string
	nonce     string
}

// newSession returns a session without a token whose proofs use fresh keys.
func newSession(tr *transport) (*session, error) {
	d, err := newKey()
	if err != nil {
		return nil, err
	}
	s, err := newKey()
	if err != nil {
		return nil, err
	}
	return &session{tr: tr, dpop: d, stepUp: s}, nil
}

// call is one operation: method, path below /api/v1, query, body, whether it takes an
// Idempotency-Key, the expected status, where to decode the answer, and the If-Match header of a
// conditional write with the ETag of the answer.
type call struct {
	op, method, path string
	query            url.Values
	body             any
	idempotent       bool
	want             int
	out              any
	ifMatch          string
	etag             *string
}

// do sends cl, repeats it once on 401 use_dpop_nonce, once after a refresh of the token pair on
// 401 token_expired (a demo with a long warm-up outlives an access token) and after a 429 with
// Retry-After (the same Idempotency-Key every time), and decodes the answer.
func (s *session) do(ctx context.Context, cl call) error {
	var raw []byte
	if cl.body != nil {
		b, err := json.Marshal(cl.body)
		if err != nil {
			return fmt.Errorf("demo: %s: encode: %w", cl.op, err)
		}
		raw = b
	}
	idem := ""
	if cl.idempotent {
		idem = newUUIDv7().String()
	}
	r := retries{}
	for {
		status, body, h, err := s.send(ctx, cl, raw, idem)
		if err != nil {
			return err
		}
		if status == cl.want {
			return decode(cl, status, body, h)
		}
		var p rest.Problem
		_ = json.Unmarshal(body, &p)
		again, err := s.retry(ctx, cl.op, &r, status, p.Code, h)
		if err != nil {
			return err
		}
		if !again {
			return problemError(cl.op, status, p)
		}
	}
}

// retries counts the repeats of one call.
type retries struct {
	nonce, refreshed bool
	limited          int
}

// retry decides whether a failed answer is repeated: once after use_dpop_nonce (the nonce is
// stored already), once after a refresh on token_expired, and after a 429 with a Retry-After the
// command waits for. The error is a failed refresh or an ended wait.
func (s *session) retry(ctx context.Context, op string, r *retries, status int, code string, h http.Header) (bool, error) {
	switch {
	case status == http.StatusUnauthorized && code == "use_dpop_nonce" && !r.nonce:
		r.nonce = true
		return true, nil
	case status == http.StatusUnauthorized && code == "token_expired" && !r.refreshed && s.hasRefresh():
		r.refreshed = true
		return true, s.renew(ctx)
	case status == http.StatusTooManyRequests && r.limited < maxRateLimitRetries:
		wait, ok := retryAfter(h.Get("Retry-After"))
		if !ok {
			return false, nil
		}
		r.limited++
		if err := s.tr.sleep(ctx, wait); err != nil {
			return false, fmt.Errorf("demo: %s: rate limited: %w", op, err)
		}
		return true, nil
	}
	return false, nil
}

// decode stores the ETag of the answer when asked and decodes its body into cl.out.
func decode(cl call, status int, body []byte, h http.Header) error {
	if cl.etag != nil {
		*cl.etag = h.Get("ETag")
	}
	if cl.out == nil {
		return nil
	}
	if err := json.Unmarshal(body, cl.out); err != nil {
		return fmt.Errorf("demo: %s: decode the %d answer: %w", cl.op, status, err)
	}
	return nil
}

// problemError returns the StatusError of a failed answer with its first field error.
func problemError(op string, status int, p rest.Problem) error {
	se := &StatusError{Operation: op, Status: status, Code: p.Code}
	if p.Errors != nil && len(*p.Errors) > 0 {
		se.Field = (*p.Errors)[0].Code + " " + (*p.Errors)[0].Pointer
	}
	return se
}

// retryAfter parses Retry-After in whole seconds; another form or a value above maxRetryAfter is
// not waited for. A value of 0 waits one second, so a retry never spins.
func retryAfter(v string) (time.Duration, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 || time.Duration(n)*time.Second > maxRetryAfter {
		return 0, false
	}
	return max(time.Duration(n)*time.Second, time.Second), true
}

// send performs one exchange and keeps the newest DPoP-Nonce.
func (s *session) send(ctx context.Context, cl call, raw []byte, idem string) (int, []byte, http.Header, error) {
	path := apiPrefix + cl.path
	target := s.tr.base + path
	if len(cl.query) > 0 {
		target += "?" + cl.query.Encode()
	}
	var body io.Reader
	if raw != nil {
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, cl.method, target, body)
	if err != nil {
		return 0, nil, nil, err
	}
	s.mu.Lock()
	token, nonce := s.token, s.nonce
	s.mu.Unlock()
	proof, err := s.dpop.proof(cl.method, restHTU(s.tr.public, path), nonce, token, time.Now())
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("DPoP", proof)
	if token != "" {
		req.Header.Set("Authorization", "DPoP "+token)
	}
	if raw != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	if cl.ifMatch != "" {
		req.Header.Set("If-Match", cl.ifMatch)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.tr.hc.Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("demo: %s: %w", cl.op, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("demo: %s: read the answer: %w", cl.op, err)
	}
	if n := resp.Header.Get("DPoP-Nonce"); n != "" {
		s.mu.Lock()
		s.nonce = n
		s.mu.Unlock()
	}
	return resp.StatusCode, b, resp.Header, nil
}

// enroll redeems code as a new device of the person (redeemEnrollmentCode, 08 5.1): platform
// macos with the DPoP key and the step-up key of the session, both in memory only. Afterwards
// every request of the session carries the access token.
func (s *session) enroll(ctx context.Context, code, deviceName string) (rest.TokenResponse, error) {
	var out rest.TokenResponse
	err := s.do(ctx, call{op: "redeemEnrollmentCode", method: http.MethodPost, path: "/auth/enroll/redeem", want: http.StatusCreated, out: &out,
		body: rest.EnrollRedeemRequest{
			EnrollmentCode: code,
			Device: rest.DeviceRegistration{
				DeviceId: newUUIDv7(), Name: deviceName, Platform: rest.DevicePlatformMacos,
				Alg: rest.DeviceRegistrationAlgES256, PublicJwk: s.dpop.publicJWK(),
				StepUpKey: &rest.DeviceKeyRegistration{Alg: rest.DeviceKeyRegistrationAlgES256, PublicJwk: s.stepUp.publicJWK()},
			},
		}})
	if err != nil {
		return out, err
	}
	s.mu.Lock()
	s.token, s.refresh, s.sessionID = out.AccessToken, out.RefreshToken, out.Session.Id.String()
	s.mu.Unlock()
	return out, nil
}

// hasRefresh reports whether the session holds a refresh token (an agent session does not).
func (s *session) hasRefresh() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refresh != ""
}

// renew rotates the token pair with refreshToken (08 7.4): the request carries the DPoP proof of
// the device key without the access token.
func (s *session) renew(ctx context.Context) error {
	s.mu.Lock()
	refresh := s.refresh
	s.token = ""
	s.mu.Unlock()
	var out rest.TokenResponse
	err := s.do(ctx, call{op: "refreshToken", method: http.MethodPost, path: "/auth/token/refresh", want: http.StatusOK, out: &out,
		body: map[string]string{"refresh_token": refresh}})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.token, s.refresh = out.AccessToken, out.RefreshToken
	s.mu.Unlock()
	return nil
}

// confirm completes a step-up with the device key (08 6.2): a challenge, then the signature of
// the step-up key over challenge, session and time.
func (s *session) confirm(ctx context.Context) error {
	var ch rest.StepUpChallenge
	if err := s.do(ctx, call{op: "createStepUpChallenge", method: http.MethodPost, path: "/auth/step-up/challenge", want: http.StatusOK, out: &ch}); err != nil {
		return err
	}
	s.mu.Lock()
	sid := s.sessionID
	s.mu.Unlock()
	sig, err := s.stepUp.stepUpSignature(ch.Challenge, sid, time.Now())
	if err != nil {
		return err
	}
	return s.do(ctx, call{op: "completeStepUpWithDeviceKey", method: http.MethodPost, path: "/auth/step-up/device-key", want: http.StatusOK,
		body: rest.StepUpDeviceKeyRequest{ChallengeId: ch.ChallengeId, Signature: sig}})
}

// withStepUp runs fn; when it fails with 401 step_up_required (the step-up of the session is
// missing or older than auth.step_up_max_age), it steps up once and runs fn again.
func (s *session) withStepUp(ctx context.Context, fn func() error) error {
	err := fn()
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusUnauthorized || se.Code != "step_up_required" {
		return err
	}
	if err := s.confirm(ctx); err != nil {
		return err
	}
	return fn()
}

// get decodes the answer of a GET of path below /api/v1.
func (s *session) get(ctx context.Context, op, path string, query url.Values, out any) error {
	return s.do(ctx, call{op: op, method: http.MethodGet, path: path, query: query, want: http.StatusOK, out: out})
}

// post sends body to path and decodes the answer of a creation (201).
func (s *session) post(ctx context.Context, op, path string, body, out any) error {
	return s.do(ctx, call{op: op, method: http.MethodPost, path: path, body: body, idempotent: true, want: http.StatusCreated, out: out})
}

// ownerPost is a creating post (201) with withStepUp, for the owner actions with step-up (05 2.4).
func (s *session) ownerPost(ctx context.Context, op, path string, body, out any) error {
	return s.withStepUp(ctx, func() error { return s.post(ctx, op, path, body, out) })
}
