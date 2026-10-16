// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// apiPrefix is the path of the API below the server origin (docs/api/openapi.yaml servers).
const apiPrefix = "/api/v1"

// egressPurpose labels the client in the logs of platform/egress.
const egressPurpose = "bothy_import"

// Limits are the values of the tool that the client uses (22 19). M8-04 fills them from the
// tool's configuration loader; until then DefaultLimits gives the defaults of 22 19.
type Limits struct {
	// LoginPollInterval is import.login_poll_interval: the wait between two completePairing
	// calls of Login. Positive.
	LoginPollInterval time.Duration
	// Max429Retries is import.discord.max_429_retries: retries after a 429 of the server per
	// call. Not negative.
	Max429Retries int
	// MaxRateWait is import.discord.max_rate_wait: a longer Retry-After ends the call with
	// ErrRateWaitTooLong. Positive.
	MaxRateWait time.Duration
	// HTTPTimeout is import.http_timeout: the time limit of one request. Positive.
	HTTPTimeout time.Duration
	// MaxResponseBytes is egress.max_response_bytes: the largest response body. Positive.
	MaxResponseBytes int64
}

// DefaultLimits returns the defaults of 22 19 (import.login_poll_interval,
// import.discord.max_429_retries, import.discord.max_rate_wait, import.http_timeout) and of
// egress.max_response_bytes (19).
func DefaultLimits() Limits {
	return Limits{LoginPollInterval: 5 * time.Second, Max429Retries: 5, MaxRateWait: time.Minute,
		HTTPTimeout: time.Minute, MaxResponseBytes: 1 << 20}
}

// validate checks every field.
func (l Limits) validate() error {
	switch {
	case l.LoginPollInterval <= 0:
		return errors.New("bothyapi: import.login_poll_interval must be positive")
	case l.Max429Retries < 0:
		return errors.New("bothyapi: import.discord.max_429_retries must not be negative")
	case l.MaxRateWait <= 0:
		return errors.New("bothyapi: import.discord.max_rate_wait must be positive")
	case l.HTTPTimeout <= 0:
		return errors.New("bothyapi: import.http_timeout must be positive")
	case l.MaxResponseBytes <= 0:
		return errors.New("bothyapi: egress.max_response_bytes must be positive")
	}
	return nil
}

// Sleeper waits d or until ctx ends, whichever comes first, and returns the context error in the
// second case. Tests pass a recording sleeper that never waits.
type Sleeper func(ctx context.Context, d time.Duration) error

// sleepContext is the Sleeper of production.
func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Deps are the dependencies of the client; every field may be nil.
type Deps struct {
	// Clock is the time of proofs, expiry and the pairing deadline; nil means clock.System().
	Clock clock.Clock
	// Sleep waits for Retry-After and between pairing polls; nil means a timer that ends with
	// the context.
	Sleep Sleeper
	// Logger receives one debug record per request (method, path, status, attempt) and one info
	// record per wait; never a header, body or token. nil discards them.
	Logger *slog.Logger
	// Resolver resolves the server host; nil means the system resolver.
	Resolver egress.Resolver
	// RootCAs are the trusted TLS roots; nil means the system roots.
	RootCAs *x509.CertPool
}

// Options are the settings of a client of an existing session.
type Options struct {
	// SessionFile is the path of the session file (DefaultSessionPath or --session-file).
	SessionFile string
	// Limits are the tool values; see DefaultLimits.
	Limits Limits
	// Deps are the dependencies.
	Deps Deps
}

// Client is the generated API client (api/restclient) over the DPoP transport of a session. Call
// the operations of the contract through the embedded ClientWithResponses; Me and Limits are
// helpers for the preflight of the tool. It is safe for concurrent use.
type Client struct {
	*restclient.ClientWithResponses
	core *core
}

// String names only the type, so a client in a log line shows nothing of the session.
func (c *Client) String() string { return "bothyapi.Client" }

// GoString returns the same as String.
func (c *Client) GoString() string { return c.String() }

// Format prints String for every verb.
func (c *Client) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(c.String())) }

// New returns a client of the session in opts.SessionFile. Errors: ErrNotLoggedIn without a
// session file, ErrInsecureSessionFile, ErrInvalidSessionFile, ErrInvalidServerURL for a
// session whose server URL is no longer allowed, or an invalid Limits value.
//
// Example:
//
//	c, err := bothyapi.New(ctx, bothyapi.Options{SessionFile: path, Limits: bothyapi.DefaultLimits()})
//	resp, err := c.GetMeWithResponse(ctx)
func New(_ context.Context, opts Options) (*Client, error) {
	if err := opts.Limits.validate(); err != nil {
		return nil, err
	}
	file := SessionFile{Path: opts.SessionFile}
	s, err := file.Load()
	if err != nil {
		return nil, err
	}
	origin, err := checkServerURL(s.ServerURL)
	if err != nil {
		return nil, err
	}
	key, err := parseProofKey(s.DPoPPrivateJWK)
	if err != nil {
		return nil, err
	}
	co, err := newCore(origin, file, key, s, opts.Limits, opts.Deps)
	if err != nil {
		return nil, err
	}
	return &Client{ClientWithResponses: co.api, core: co}, nil
}

// Me returns the person of the session (getMe). Errors: those of the transport, or a
// *StatusError for another status than 200.
func (c *Client) Me(ctx context.Context) (*restclient.Me, error) {
	resp, err := c.GetMeWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, statusError("getMe", resp.HTTPResponse, resp.Body)
	}
	return resp.JSON200, nil
}

// Limits returns the effective limits of the server (getInstance, InstanceLimits). Errors:
// those of the transport, or a *StatusError for another status than 200.
func (c *Client) Limits(ctx context.Context) (restclient.InstanceLimits, error) {
	resp, err := c.GetInstanceWithResponse(ctx)
	if err != nil {
		return restclient.InstanceLimits{}, err
	}
	if resp.JSON200 == nil {
		return restclient.InstanceLimits{}, statusError("getInstance", resp.HTTPResponse, resp.Body)
	}
	return resp.JSON200.Limits, nil
}

// core is the state behind a client: the session, the key, the egress client and the transport.
type core struct {
	origin string
	file   SessionFile
	key    *proofKey
	limits Limits
	clk    clock.Clock
	sleep  Sleeper
	log    *slog.Logger
	http   *egress.Client
	api    *restclient.ClientWithResponses

	mu    sync.Mutex
	sess  *Session // nil during a login
	nonce string

	// refreshMu serializes the refreshes of this process; the file lock serializes processes.
	refreshMu sync.Mutex
	// lockHeld is set while the caller of the core holds the file lock (Logout), so a refresh
	// inside does not take it a second time.
	lockHeld atomic.Bool
}

// newCore builds the egress client for the server origin, the transport and the generated
// client.
func newCore(origin string, file SessionFile, key *proofKey, s *Session, limits Limits, deps Deps) (*core, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return nil, ErrInvalidServerURL
	}
	var port uint16
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return nil, ErrInvalidServerURL
		}
		port = uint16(n)
	}
	// The server host is the only allowed host; allow_private lets it resolve to a private
	// address because the owner names it explicitly (requirement 15). http is limited to a
	// loopback host by checkServerURL before any request.
	host := u.Hostname()
	if a, err := netip.ParseAddr(host); err == nil {
		host = a.Unmap().String()
	}
	rule := egress.Rule{Host: strings.ToLower(host), Port: port, AllowPrivate: true}
	hc, err := egress.New(egress.Config{AllowedHosts: []egress.Rule{rule}, Timeout: limits.HTTPTimeout, MaxResponseBytes: limits.MaxResponseBytes},
		egress.Options{Purpose: egressPurpose, Redirects: egress.RedirectNever},
		egress.Deps{Resolver: deps.Resolver, RootCAs: deps.RootCAs, Logger: deps.Logger})
	if err != nil {
		return nil, fmt.Errorf("bothyapi: egress: %w", err)
	}
	c := &core{origin: origin, file: file, key: key, limits: limits, clk: deps.Clock, sleep: deps.Sleep, log: deps.Logger, http: hc, sess: s}
	if c.clk == nil {
		c.clk = clock.System()
	}
	if c.sleep == nil {
		c.sleep = sleepContext
	}
	if c.log == nil {
		c.log = slog.New(slog.DiscardHandler)
	}
	// The transport goes in as a function value of an api type: a value of a cmd type handed to
	// a function of api counts as a dependency of api on cmd for the deep scan of go-arch-lint.
	api, err := restclient.NewClientWithResponses(origin+apiPrefix, restclient.WithHTTPClient(restclient.DoerFunc(c.RoundTrip)))
	if err != nil {
		return nil, fmt.Errorf("bothyapi: client: %w", err)
	}
	c.api = api
	return c, nil
}

// session returns the current session (nil during a login).
func (c *core) session() *Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sess
}

// setSession replaces the current session.
func (c *core) setSession(s *Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sess = s
}
