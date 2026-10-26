// SPDX-License-Identifier: Apache-2.0

package load

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/test/smoke"
)

// apiPrefix is the path prefix of the REST API and the gateway (docs/api/openapi.yaml servers).
const apiPrefix = "/api/v1"

// Bounds of one exchange: the answer the driver reads and the time one request may take. A send
// that takes longer counts as failed (status 0), far beyond any target.
const (
	maxBody        = 4 << 20
	requestTimeout = 30 * time.Second
)

// Driver holds the shared HTTP transport of every simulated person against one server. It is
// safe for concurrent use.
type Driver struct {
	base, public string
	hc           *http.Client
	// nonce is the newest DPoP-Nonce of the server, shared by every person.
	nonce atomic.Pointer[string]
	log   io.Writer
	mu    sync.Mutex
}

// NewDriver returns a driver that dials baseURL (for example http://127.0.0.1:8080) and signs its
// DPoP proofs for publicURL (server.public_url). log receives progress lines (never a token, a
// code or a text); nil discards them. The error reports a URL that is not usable.
func NewDriver(baseURL, publicURL string, log io.Writer) (*Driver, error) {
	if u, err := url.Parse(baseURL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("load: base URL %q is no http URL", baseURL)
	}
	if _, err := smoke.RESTHTU(publicURL, apiPrefix); err != nil {
		return nil, err
	}
	if log == nil {
		log = io.Discard
	}
	tr := &http.Transport{Proxy: nil, ForceAttemptHTTP2: false, MaxIdleConns: 4096, MaxIdleConnsPerHost: 4096, IdleConnTimeout: 90 * time.Second}
	hc := &http.Client{Transport: tr, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Driver{base: baseURL, public: publicURL, hc: hc, log: log}, nil
}

// logf writes one progress line.
func (d *Driver) logf(format string, args ...any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, _ = fmt.Fprintf(d.log, "load: "+format+"\n", args...)
}

// Person is one simulated person: its device key, tokens and the client address it sends from.
type Person struct {
	// Index is the position in the generated world (Generator.Person).
	Index int
	Spec  PersonSpec
	// ID is the principal, DeviceID the device of the enrollment.
	ID, DeviceID rest.Uuid
	// Address is sent as X-Forwarded-For: an own /64 of the documentation range 2001:db8::/32 per
	// person, as distinct clients behind the reverse proxy of a deployment, so the per-address
	// rate limit rl.public_auth does not count the whole load as one client.
	Address string

	key     smoke.Key
	stepUp  smoke.Key
	mu      sync.Mutex
	access  string
	refresh string
	session string
	expires time.Time
}

// ClientAddress returns the documentation address of person i: its own /64 of 2001:db8::/32
// (RFC 3849), because the server counts IPv6 clients by /64.
func ClientAddress(i int) string { return fmt.Sprintf("2001:db8:%x:%x::1", (i+1)>>16, (i+1)&0xffff) }

// tokens returns the access token and its expiry.
func (p *Person) tokens() (string, time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.access, p.expires
}

// adopt stores the tokens of an answer of redeemEnrollmentCode or refreshToken.
func (p *Person) adopt(tr rest.TokenResponse, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.access, p.refresh, p.session = tr.AccessToken, tr.RefreshToken, tr.Session.Id.String()
	p.expires = now.Add(time.Duration(tr.ExpiresIn) * time.Second)
}

// StatusError is an answer with another status than the expected one.
type StatusError struct {
	Op     string
	Status int
	Code   string
}

// Error implements error with the operation, the status and the problem code, never a body.
func (e *StatusError) Error() string {
	return fmt.Sprintf("load: %s answered %d %s", e.Op, e.Status, e.Code)
}

// request is one exchange of a person: operation name, method, path below /api/v1, body, the
// expected status and where to decode the answer.
type request struct {
	op, method, path string
	body             any
	idempotent       bool
	want             int
	out              any
	// anonymous sends no access token (redeemEnrollmentCode, refreshToken).
	anonymous bool
}

// do sends r as p (nil p: anonymous without an address) and decodes the answer; a 401
// use_dpop_nonce is repeated once with the new nonce. It returns the status (0 on a transport
// error).
func (d *Driver) do(ctx context.Context, p *Person, key smoke.Key, r request) (int, error) {
	var raw []byte
	if r.body != nil {
		b, err := json.Marshal(r.body)
		if err != nil {
			return 0, err
		}
		raw = b
	}
	idem := ""
	if r.idempotent {
		idem = NewUUIDv7().String()
	}
	for attempt := 0; ; attempt++ {
		status, body, err := d.send(ctx, p, key, r, raw, idem)
		if err != nil {
			return 0, err
		}
		if status == r.want {
			if r.out != nil {
				if err := json.Unmarshal(body, r.out); err != nil {
					return status, fmt.Errorf("load: %s: decode: %w", r.op, err)
				}
			}
			return status, nil
		}
		var pr rest.Problem
		_ = json.Unmarshal(body, &pr)
		if status == http.StatusUnauthorized && pr.Code == "use_dpop_nonce" && attempt == 0 {
			continue
		}
		return status, &StatusError{Op: r.op, Status: status, Code: pr.Code}
	}
}

// send performs one exchange and keeps the newest DPoP-Nonce.
func (d *Driver) send(ctx context.Context, p *Person, key smoke.Key, r request, raw []byte, idem string) (int, []byte, error) {
	path := apiPrefix + r.path
	htu, err := smoke.RESTHTU(d.public, path)
	if err != nil {
		return 0, nil, err
	}
	var body io.Reader
	if raw != nil {
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, d.base+path, body)
	if err != nil {
		return 0, nil, err
	}
	token := ""
	if p != nil && !r.anonymous {
		token, _ = p.tokens()
	}
	nonce := ""
	if n := d.nonce.Load(); n != nil {
		nonce = *n
	}
	proof, err := smoke.NewProof(key, r.method, htu, nonce, token, time.Now())
	if err != nil {
		return 0, nil, err
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
	if p != nil {
		req.Header.Set("X-Forwarded-For", p.Address)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := d.hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("load: %s: %w", r.op, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return 0, nil, fmt.Errorf("load: %s: read the answer: %w", r.op, err)
	}
	if n := resp.Header.Get("DPoP-Nonce"); n != "" {
		d.nonce.Store(&n)
	}
	return resp.StatusCode, b, nil
}

// Refresh renews the tokens of p (refreshToken, rotation) when they expire before until.
func (d *Driver) Refresh(ctx context.Context, p *Person, until time.Time) error {
	_, exp := p.tokens()
	if exp.After(until) {
		return nil
	}
	p.mu.Lock()
	refresh := p.refresh
	p.mu.Unlock()
	var tr rest.TokenResponse
	if _, err := d.do(ctx, p, p.key, request{op: "refreshToken", method: http.MethodPost, path: "/auth/token/refresh",
		body: rest.TokenRefreshRequest{RefreshToken: refresh}, want: http.StatusOK, out: &tr, anonymous: true}); err != nil {
		return err
	}
	p.adopt(tr, time.Now())
	return nil
}

// errNoRandom reports a failure of the random source.
var errNoRandom = errors.New("load: no randomness")

// NewUUIDv7 returns a fresh UUIDv7 (RFC 9562 5.7): device_id, client_msg_id and Idempotency-Key
// are UUIDv7 in the contract. It panics when the system has no randomness, like crypto/rand.
func NewUUIDv7() rest.Uuid {
	var u [16]byte
	if _, err := rand.Read(u[6:]); err != nil {
		panic(errors.Join(errNoRandom, err))
	}
	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(time.Now().UnixMilli())) // #nosec G115 -- the clock is after 1970
	copy(u[:6], ms[2:])
	u[6] = u[6]&0x0f | 0x70
	u[8] = u[8]&0x3f | 0x80
	return rest.Uuid(u)
}
