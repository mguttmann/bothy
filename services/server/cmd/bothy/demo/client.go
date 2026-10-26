// SPDX-License-Identifier: Apache-2.0

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
	hc := &http.Client{
		Transport:     &http.Transport{Proxy: nil, ForceAttemptHTTP2: false},
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

// do sends cl, repeats it once on 401 use_dpop_nonce and after a 429 with Retry-After (the same
// Idempotency-Key both times), and decodes the answer.
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
	nonceRetried, limited := false, 0
	for {
		status, body, h, err := s.send(ctx, cl, raw, idem)
		if err != nil {
			return err
		}
		if status == cl.want {
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
		var p rest.Problem
		_ = json.Unmarshal(body, &p)
		if status == http.StatusUnauthorized && p.Code == "use_dpop_nonce" && !nonceRetried {
			nonceRetried = true
			continue
		}
		if wait, ok := retryAfter(h.Get("Retry-After")); status == http.StatusTooManyRequests && ok && limited < maxRateLimitRetries {
			limited++
			if err := s.tr.sleep(ctx, wait); err != nil {
				return fmt.Errorf("demo: %s: rate limited: %w", cl.op, err)
			}
			continue
		}
		se := &StatusError{Operation: cl.op, Status: status, Code: p.Code}
		if p.Errors != nil && len(*p.Errors) > 0 {
			se.Field = (*p.Errors)[0].Code + " " + (*p.Errors)[0].Pointer
		}
		return se
	}
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
	s.token, s.sessionID = out.AccessToken, out.Session.Id.String()
	s.mu.Unlock()
	return out, nil
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

// post sends body to path and decodes the answer with status want.
func (s *session) post(ctx context.Context, op, path string, body any, want int, out any) error {
	return s.do(ctx, call{op: op, method: http.MethodPost, path: path, body: body, idempotent: true, want: want, out: out})
}

// ownerPost is post with withStepUp, for the owner actions with step-up (05 2.4).
func (s *session) ownerPost(ctx context.Context, op, path string, body any, want int, out any) error {
	return s.withStepUp(ctx, func() error { return s.post(ctx, op, path, body, want, out) })
}
