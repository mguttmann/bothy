// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"bytes"
	"context"
	"encoding/json"
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

// maxBody bounds the answer the probe reads; every answer of the scenario is far smaller.
const maxBody = 8 << 20

// requestTimeout bounds one HTTP exchange; the server answers the scenario's calls in
// milliseconds, a hang is a failure.
const requestTimeout = 30 * time.Second

// REST is the part of the public API (docs/api/openapi.yaml, prefix /api/v1) that the scenario
// calls, one method per operation, with the request and response types of the generated package
// api/rest. A non-2xx answer is a *StatusError.
type REST interface {
	// SetCredentials sets the DPoP key of every later proof and the access token ("" for none).
	SetCredentials(k Key, accessToken string)
	// GetDiscovery is getDiscovery (GET /instance/discovery).
	GetDiscovery(ctx context.Context) (rest.Discovery, error)
	// RedeemEnrollmentCode is redeemEnrollmentCode (POST /auth/enroll/redeem), 201.
	RedeemEnrollmentCode(ctx context.Context, body rest.EnrollRedeemRequest) (rest.TokenResponse, error)
	// GetMe is getMe (GET /me).
	GetMe(ctx context.Context) (rest.Me, error)
	// CreateSpace is createSpace (POST /spaces), 201.
	CreateSpace(ctx context.Context, body rest.SpaceCreate) (rest.Space, error)
	// CreateChannel is createChannel (POST /spaces/{space_id}/channels), 201.
	CreateChannel(ctx context.Context, spaceID rest.Uuid, body rest.ChannelCreate) (rest.Channel, error)
	// CreateThread is createThread (POST /channels/{channel_id}/threads), 201.
	CreateThread(ctx context.Context, channelID rest.Uuid, body rest.ThreadCreate) (rest.ThreadCreated, error)
	// CreateMessage is createMessage (POST /threads/{thread_id}/messages), 201.
	CreateMessage(ctx context.Context, threadID rest.Uuid, body rest.MessageCreate) (rest.Message, error)
	// ListEvents is listEvents (GET /events).
	ListEvents(ctx context.Context, params rest.ListEventsParams) (rest.EventPage, error)
	// CreateStepUpChallenge is createStepUpChallenge (POST /auth/step-up/challenge).
	CreateStepUpChallenge(ctx context.Context) (rest.StepUpChallenge, error)
	// CompleteStepUpWithDeviceKey is completeStepUpWithDeviceKey (POST /auth/step-up/device-key).
	CompleteStepUpWithDeviceKey(ctx context.Context, body rest.StepUpDeviceKeyRequest) (rest.StepUpResult, error)
	// RevokeDevice is revokeDevice (POST /devices/{device_id}/revoke).
	RevokeDevice(ctx context.Context, deviceID rest.Uuid, body rest.ReasonBody) (rest.Device, error)
}

// StatusError is a non-2xx answer: the operation, the HTTP status and the code and type of the
// problem body (06 "Errors"). It never holds the body, which may echo a value.
type StatusError struct {
	Operation string
	Status    int
	Code      string
	Type      string
}

// Error returns operation, status and code.
func (e *StatusError) Error() string {
	return fmt.Sprintf("smoke: %s answered %d %s", e.Operation, e.Status, e.Code)
}

// HTTPClient implements REST with net/http against one base URL. Every request carries a DPoP
// proof of the current key for the public URL (RESTHTU) with the newest DPoP-Nonce of the server;
// a 401 use_dpop_nonce is repeated exactly once with the new nonce (06 3.1). Writes that are not
// exempt carry a fresh UUIDv7 Idempotency-Key, the same on the repetition. The client never uses
// a proxy and never follows a redirect, so it reaches only the base URL. It is safe for
// concurrent use.
type HTTPClient struct {
	base, public string
	hc           *http.Client

	mu    sync.Mutex
	key   Key
	token string
	nonce string
	// retryAfter is the Retry-After header of the newest answer (429).
	retryAfter string
}

// Compile-time check: HTTPClient is the REST of the command.
var _ REST = (*HTTPClient)(nil)

// NewHTTPClient returns a client that dials baseURL (for example http://127.0.0.1:8080) and signs
// for publicURL (server.public_url). It starts with a fresh key and no token, so the anonymous
// calls of the precheck carry a proof too. The error reports a URL that is not usable or a
// failure of the random source.
func NewHTTPClient(baseURL, publicURL string) (*HTTPClient, error) {
	if u, err := url.Parse(baseURL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("smoke: base URL %q is no http URL", baseURL)
	}
	if _, err := publicOrigin(publicURL); err != nil {
		return nil, err
	}
	k, err := NewKey()
	if err != nil {
		return nil, err
	}
	hc := &http.Client{
		Transport:     &http.Transport{Proxy: nil, ForceAttemptHTTP2: false},
		Timeout:       requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &HTTPClient{base: baseURL, public: publicURL, hc: hc, key: k}, nil
}

// SetCredentials implements REST.
func (c *HTTPClient) SetCredentials(k Key, accessToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.key, c.token = k, accessToken
}

// call is one operation: method, path below /api/v1, query, body, whether it takes an
// Idempotency-Key, the expected status and where to decode the answer.
type call struct {
	op, method, path string
	query            url.Values
	body             any
	idempotent       bool
	want             int
	out              any
	// ifMatch is the If-Match header of a conditional write (06 "Concurrency"); etag receives
	// the ETag of the answer.
	ifMatch string
	etag    *string
	// header holds further request headers (Content-Digest of an upload part).
	header http.Header
	// raw is a binary body sent as application/octet-stream instead of body.
	raw []byte
}

// do sends cl, repeats it once on 401 use_dpop_nonce and decodes the answer.
func (c *HTTPClient) do(ctx context.Context, cl call) error {
	raw := cl.raw
	if cl.body != nil {
		b, err := json.Marshal(cl.body)
		if err != nil {
			return fmt.Errorf("smoke: %s: encode: %w", cl.op, err)
		}
		raw = b
	}
	idem := ""
	if cl.idempotent {
		idem = newUUIDv7().String()
	}
	nonceRetried, limited := false, 0
	for {
		status, body, etag, err := c.send(ctx, cl, raw, idem)
		if err != nil {
			return err
		}
		if status == cl.want {
			if cl.etag != nil {
				*cl.etag = etag
			}
			if cl.out == nil {
				return nil
			}
			if err := json.Unmarshal(body, cl.out); err != nil {
				return fmt.Errorf("smoke: %s: decode the %d answer: %w", cl.op, status, err)
			}
			return nil
		}
		var p rest.Problem
		_ = json.Unmarshal(body, &p)
		if status == http.StatusUnauthorized && p.Code == "use_dpop_nonce" && !nonceRetried {
			nonceRetried = true
			continue
		}
		// 429 means the server did nothing (06 "Rate limits"): wait Retry-After and send the
		// same request again, with the same Idempotency-Key.
		if wait, ok := retryAfter(c.lastRetryAfter()); status == http.StatusTooManyRequests && ok && limited < maxRateLimitRetries {
			limited++
			select {
			case <-ctx.Done():
				return fmt.Errorf("smoke: %s: rate limited: %w", cl.op, ctx.Err())
			case <-time.After(wait):
			}
			continue
		}
		return &StatusError{Operation: cl.op, Status: status, Code: p.Code, Type: p.Type}
	}
}

// Bounds of the retries after 429: a few rounds, each wait at most maxRetryAfter.
const (
	maxRateLimitRetries = 5
	maxRetryAfter       = 2 * time.Minute
)

// retryAfter parses Retry-After in whole seconds (06 "Rate limits"); a value above maxRetryAfter
// or another form is not waited for.
func retryAfter(v string) (time.Duration, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 || time.Duration(n)*time.Second > maxRetryAfter {
		return 0, false
	}
	return time.Duration(n) * time.Second, true
}

// lastRetryAfter is the Retry-After header of the newest answer.
func (c *HTTPClient) lastRetryAfter() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.retryAfter
}

// send performs one exchange, keeps the newest DPoP-Nonce and returns the status, the body and
// the ETag of the answer.
func (c *HTTPClient) send(ctx context.Context, cl call, raw []byte, idem string) (int, []byte, string, error) {
	path := apiPrefix + cl.path
	htu, err := RESTHTU(c.public, path)
	if err != nil {
		return 0, nil, "", err
	}
	target := c.base + path
	if len(cl.query) > 0 {
		target += "?" + cl.query.Encode()
	}
	var body io.Reader
	if raw != nil {
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, cl.method, target, body)
	if err != nil {
		return 0, nil, "", err
	}
	c.mu.Lock()
	key, token, nonce := c.key, c.token, c.nonce
	c.mu.Unlock()
	proof, err := NewProof(key, cl.method, htu, nonce, token, time.Now())
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("DPoP", proof)
	if token != "" {
		req.Header.Set("Authorization", "DPoP "+token)
	}
	switch {
	case cl.raw != nil:
		req.Header.Set("Content-Type", "application/octet-stream")
	case raw != nil:
		req.Header.Set("Content-Type", "application/json")
	}
	for name, values := range cl.header {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	if cl.ifMatch != "" {
		req.Header.Set("If-Match", cl.ifMatch)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, "", fmt.Errorf("smoke: %s: %w", cl.op, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return 0, nil, "", fmt.Errorf("smoke: %s: read the answer: %w", cl.op, err)
	}
	c.mu.Lock()
	if n := resp.Header.Get("DPoP-Nonce"); n != "" {
		c.nonce = n
	}
	c.retryAfter = resp.Header.Get("Retry-After")
	c.mu.Unlock()
	return resp.StatusCode, b, resp.Header.Get("ETag"), nil
}

// GetDiscovery implements REST.
func (c *HTTPClient) GetDiscovery(ctx context.Context) (rest.Discovery, error) {
	var out rest.Discovery
	return out, c.do(ctx, call{op: "getDiscovery", method: http.MethodGet, path: "/instance/discovery", want: http.StatusOK, out: &out})
}

// RedeemEnrollmentCode implements REST.
func (c *HTTPClient) RedeemEnrollmentCode(ctx context.Context, body rest.EnrollRedeemRequest) (rest.TokenResponse, error) {
	var out rest.TokenResponse
	return out, c.do(ctx, call{op: "redeemEnrollmentCode", method: http.MethodPost, path: "/auth/enroll/redeem", body: body, want: http.StatusCreated, out: &out})
}

// GetMe implements REST.
func (c *HTTPClient) GetMe(ctx context.Context) (rest.Me, error) {
	var out rest.Me
	return out, c.do(ctx, call{op: "getMe", method: http.MethodGet, path: "/me", want: http.StatusOK, out: &out})
}

// CreateSpace implements REST.
func (c *HTTPClient) CreateSpace(ctx context.Context, body rest.SpaceCreate) (rest.Space, error) {
	var out rest.Space
	return out, c.do(ctx, call{op: "createSpace", method: http.MethodPost, path: "/spaces", body: body, idempotent: true, want: http.StatusCreated, out: &out})
}

// CreateChannel implements REST.
func (c *HTTPClient) CreateChannel(ctx context.Context, spaceID rest.Uuid, body rest.ChannelCreate) (rest.Channel, error) {
	var out rest.Channel
	return out, c.do(ctx, call{op: "createChannel", method: http.MethodPost, path: "/spaces/" + spaceID.String() + "/channels", body: body,
		idempotent: true, want: http.StatusCreated, out: &out})
}

// CreateThread implements REST.
func (c *HTTPClient) CreateThread(ctx context.Context, channelID rest.Uuid, body rest.ThreadCreate) (rest.ThreadCreated, error) {
	var out rest.ThreadCreated
	return out, c.do(ctx, call{op: "createThread", method: http.MethodPost, path: "/channels/" + channelID.String() + "/threads", body: body,
		idempotent: true, want: http.StatusCreated, out: &out})
}

// CreateMessage implements REST; the operation is exempt from Idempotency-Key, client_msg_id
// makes it idempotent (04 5.1).
func (c *HTTPClient) CreateMessage(ctx context.Context, threadID rest.Uuid, body rest.MessageCreate) (rest.Message, error) {
	var out rest.Message
	return out, c.do(ctx, call{op: "createMessage", method: http.MethodPost, path: "/threads/" + threadID.String() + "/messages", body: body,
		want: http.StatusCreated, out: &out})
}

// ListEvents implements REST.
func (c *HTTPClient) ListEvents(ctx context.Context, params rest.ListEventsParams) (rest.EventPage, error) {
	q := url.Values{"after": {strconv.FormatInt(params.After, 10)}}
	if params.Epoch != nil {
		q.Set("epoch", params.Epoch.String())
	}
	if params.Limit != nil {
		q.Set("limit", strconv.Itoa(*params.Limit))
	}
	var out rest.EventPage
	return out, c.do(ctx, call{op: "listEvents", method: http.MethodGet, path: "/events", query: q, want: http.StatusOK, out: &out})
}

// CreateStepUpChallenge implements REST.
func (c *HTTPClient) CreateStepUpChallenge(ctx context.Context) (rest.StepUpChallenge, error) {
	var out rest.StepUpChallenge
	return out, c.do(ctx, call{op: "createStepUpChallenge", method: http.MethodPost, path: "/auth/step-up/challenge", want: http.StatusOK, out: &out})
}

// CompleteStepUpWithDeviceKey implements REST.
func (c *HTTPClient) CompleteStepUpWithDeviceKey(ctx context.Context, body rest.StepUpDeviceKeyRequest) (rest.StepUpResult, error) {
	var out rest.StepUpResult
	return out, c.do(ctx, call{op: "completeStepUpWithDeviceKey", method: http.MethodPost, path: "/auth/step-up/device-key", body: body,
		want: http.StatusOK, out: &out})
}

// RevokeDevice implements REST.
func (c *HTTPClient) RevokeDevice(ctx context.Context, deviceID rest.Uuid, body rest.ReasonBody) (rest.Device, error) {
	var out rest.Device
	return out, c.do(ctx, call{op: "revokeDevice", method: http.MethodPost, path: "/devices/" + deviceID.String() + "/revoke", body: body,
		idempotent: true, want: http.StatusOK, out: &out})
}
