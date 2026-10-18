// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Config holds the configuration keys egress.* (docs/spec/19-security.md, "Configuration values
// of this chapter"); internal/wire builds it from the loaded configuration.
type Config struct {
	// AllowedHosts are the entries of egress.allowed_hosts, each in the form ParseRule returns,
	// without two entries for the same host and port.
	AllowedHosts []Rule
	// Timeout is egress.timeout: the longest time of one request, including name resolution,
	// connect, TLS, redirects and reading the body. It must be positive.
	Timeout time.Duration
	// MaxResponseBytes is egress.max_response_bytes: the largest body a caller can read from
	// one response, counted after the transport decoded the content encoding it negotiated, and
	// also the largest response header (a consumer override changes only the body limit). It
	// must be positive.
	MaxResponseBytes int64
}

// HostPolicy is the allow list of one consumer, for example the hosts of the configured identity
// providers and the endpoints of their checked discovery documents (19 SEC-NET-2). The client
// asks it before any DNS lookup with the scheme ("https" or "http"), the canonical host (lower
// case without trailing dot, or an IP literal as net/netip prints it) and the port (the
// default port of the scheme when the URL has none). It must be safe for concurrent use.
//
// A host the policy allows is still subject to the address check: a policy can never open
// private addresses or plain http, because a consumer list can come from data such as a
// discovery document, while only the operator writes egress.allowed_hosts.
type HostPolicy func(ctx context.Context, scheme, host string, port uint16) bool

// RedirectMode says what the client does with a 3xx response that carries a Location.
type RedirectMode int

const (
	// RedirectNever returns the 3xx response to the caller unchanged (webhooks, 11 "Outgoing
	// webhooks").
	RedirectNever RedirectMode = iota
	// RedirectChecked follows only targets that pass every check again, at most 10 hops like
	// net/http (OIDC, 19 SEC-NET-4). A refused hop ends with redirect_not_allowed or, at
	// connection time, address_blocked.
	RedirectChecked
)

// The schemes the client speaks.
const (
	schemeHTTPS = "https"
	schemeHTTP  = "http"
)

// maxRedirects is the redirect limit of net/http, which a custom CheckRedirect replaces and so
// has to keep itself.
const maxRedirects = 10

// Options are the settings of one consumer.
type Options struct {
	// Purpose is a short label of the consumer for logs, for example "oidc" or "webhook":
	// lower-case letters, digits and "_", starting with a letter.
	Purpose string
	// Hosts is the allow list of the consumer; nil means only Config.AllowedHosts.
	Hosts HostPolicy
	// Redirects selects the redirect mode; the zero value is RedirectNever.
	Redirects RedirectMode
	// Timeout overrides Config.Timeout when a chapter names its own time limit; 0 keeps
	// Config.Timeout.
	Timeout time.Duration
	// MaxResponseBytes overrides Config.MaxResponseBytes when a chapter names its own size
	// limit; 0 keeps Config.MaxResponseBytes.
	MaxResponseBytes int64
}

// Resolver resolves a host name to its addresses; *net.Resolver implements it. The client
// calls it at connection time with network "ip" and checks every address it returns.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Metrics records the counter of refused attempts, bothy.egress.blocked with the attribute
// bothy.reason (docs/observability/signals.md). platform/otel implements it; it must be safe
// for concurrent use and must not block.
type Metrics interface { // NOSONAR: names the port, as clock.Clock does (M1-05/26)
	// Blocked counts one refused attempt with its reason.
	Blocked(ctx context.Context, reason Reason)
}

// Deps are the dependencies of a client.
type Deps struct {
	// Resolver resolves host names; nil means net.DefaultResolver. Tests pass a fake.
	Resolver Resolver
	// RootCAs are the trusted roots of TLS; nil means the roots of the system. There is no way
	// to switch verification off.
	RootCAs *x509.CertPool
	// Logger receives one Warn record per refused attempt; nil discards them.
	Logger *slog.Logger
	// Metrics counts refused attempts; nil counts nothing.
	Metrics Metrics
	// DenyAll refuses every request before any resolution or connection (host_not_allowed). Only
	// test servers set it, so that generated hosts and issuers cause no outbound traffic (M1-71).
	DenyAll bool
}

// Log attributes of a refused attempt; the keys are in docs/observability/signals.md 5.3.
const (
	logMessage  = "egress request blocked"
	attrPurpose = "bothy.egress.purpose"
	attrReason  = "bothy.reason"
	attrHost    = "server.address"
)

// idleConnTimeout closes idle connections like http.DefaultTransport does.
const idleConnTimeout = 90 * time.Second

var purposePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Client sends HTTP requests to allowed targets only. Build it with New; it is safe for
// concurrent use. Every request passes three stages:
//
//  1. before any connection: no user info, scheme https (or http with a matching rule with
//     allow_private), host and port allowed by a rule or the consumer policy; a refused host
//     is never resolved;
//  2. at connection time: the host is resolved, every address is checked, and only checked
//     addresses are dialed (never the name again); the dialer checks the socket address once
//     more before connect;
//  3. on the response: TLS was verified against RootCAs and the host name, the body is limited,
//     and the whole exchange is bounded in time.
type Client struct {
	rules     []Rule
	policy    HostPolicy
	purpose   string
	redirects RedirectMode
	timeout   time.Duration
	maxBytes  int64
	resolver  Resolver
	logger    *slog.Logger
	metrics   Metrics
	denyAll   bool
	// dial opens a connection to a checked address ("ip:port"); tests wrap it to count dials.
	dial func(ctx context.Context, network, address string) (net.Conn, error)
	http *http.Client
}

// New builds a client for one consumer from the configuration egress.*, the options of the
// consumer and the dependencies.
//
// It returns an error when Config.Timeout or Config.MaxResponseBytes is not positive, an option
// override is negative, Purpose is not a label of lower-case letters, digits and "_", the
// redirect mode is unknown, or a rule is not in the form ParseRule returns or repeats the host
// and port of another rule. Errors name the field or the index of a rule, never its value.
//
// Example (a consumer with its own host list, following checked redirects):
//
//	c, err := egress.New(cfg, egress.Options{
//		Purpose:   "oidc",
//		Hosts:     func(_ context.Context, scheme, host string, port uint16) bool { return host == "idp.example.com" },
//		Redirects: egress.RedirectChecked,
//	}, egress.Deps{Logger: logger, Metrics: metrics})
func New(cfg Config, opts Options, deps Deps) (*Client, error) {
	if err := validate(cfg, opts); err != nil {
		return nil, err
	}
	c := &Client{
		rules:     append([]Rule(nil), cfg.AllowedHosts...),
		policy:    opts.Hosts,
		purpose:   opts.Purpose,
		redirects: opts.Redirects,
		timeout:   cfg.Timeout,
		maxBytes:  cfg.MaxResponseBytes,
		resolver:  deps.Resolver,
		logger:    deps.Logger,
		metrics:   deps.Metrics,
		denyAll:   deps.DenyAll,
	}
	if opts.Timeout != 0 {
		c.timeout = opts.Timeout
	}
	if opts.MaxResponseBytes != 0 {
		c.maxBytes = opts.MaxResponseBytes
	}
	if c.resolver == nil {
		c.resolver = net.DefaultResolver
	}
	if c.logger == nil {
		c.logger = slog.New(slog.DiscardHandler)
	}
	if c.metrics == nil {
		c.metrics = noMetrics{}
	}
	c.dial = connect
	// Proxy stays nil: HTTP_PROXY and HTTPS_PROXY would make the address check see only the
	// proxy (19 SEC-NET-2). HTTP/2 stays off because DialContext and TLSClientConfig are set
	// and ForceAttemptHTTP2 is not.
	transport := &http.Transport{ // nosemgrep: bothy-go-egress-only -- the egress package is the one place that builds transports (19 SEC-NET-1)
		DialContext: c.dialContext,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    deps.RootCAs,
		},
		TLSHandshakeTimeout:    c.timeout,
		MaxResponseHeaderBytes: cfg.MaxResponseBytes,
		IdleConnTimeout:        idleConnTimeout,
	}
	c.http = &http.Client{ // nosemgrep: bothy-go-egress-only -- the egress package is the one place that builds transports (19 SEC-NET-1)
		Transport:     &checkingTransport{c: c, next: transport},
		CheckRedirect: c.checkRedirect,
		Timeout:       c.timeout,
	}
	return c, nil
}

// validate checks the input of New.
func validate(cfg Config, opts Options) error {
	switch {
	case cfg.Timeout <= 0:
		return errors.New("egress: Config.Timeout must be positive")
	case cfg.MaxResponseBytes <= 0:
		return errors.New("egress: Config.MaxResponseBytes must be positive")
	case opts.Timeout < 0:
		return errors.New("egress: Options.Timeout must not be negative")
	case opts.MaxResponseBytes < 0:
		return errors.New("egress: Options.MaxResponseBytes must not be negative")
	case !purposePattern.MatchString(opts.Purpose):
		return errors.New("egress: Options.Purpose must be lower-case letters, digits and \"_\", starting with a letter")
	case opts.Redirects != RedirectNever && opts.Redirects != RedirectChecked:
		return errors.New("egress: Options.Redirects is not a redirect mode")
	}
	seen := map[[2]string]int{}
	for i, r := range cfg.AllowedHosts {
		if err := validateRule(r); err != nil {
			return fmt.Errorf("egress: Config.AllowedHosts[%d]: %w", i, err)
		}
		key := [2]string{r.Host, strconv.Itoa(int(r.Port))}
		if j, dup := seen[key]; dup {
			return fmt.Errorf("egress: Config.AllowedHosts[%d] repeats the host and port of entry %d", i, j)
		}
		seen[key] = i
	}
	return nil
}

// Do sends req after the checks of the client and returns the response, whose body the caller
// must close. The body returns ErrResponseTooLarge beyond the limit.
//
// A refused request returns an error with errors.Is(err, ErrBlocked) and a *BlockedError inside;
// other errors are those of net/http (timeouts report Timeout() and wrap
// context.DeadlineExceeded). The error text names method, scheme and host, never path, query or
// user info, so that a caller may log it (19 SEC-LOG-3). With RedirectNever a 3xx response is a
// response, not an error, unless its Location does not parse; a 101 answer is always an error.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req) // #nosec G704 -- this is the SSRF guard: checkingTransport checks every request and hop (19 SEC-NET-1 to 4)
	if err != nil {
		// On a refused redirect net/http also returns the previous response with its body
		// already closed; the caller gets only the error.
		return nil, redactURL(err)
	}
	return resp, nil
}

// HTTPClient returns an *http.Client whose transport enforces every check, for libraries that
// take a client (OIDC, S3). Each call returns a new copy, so a caller that changes its copy
// changes nothing for others; CloseIdleConnections reaches the shared transport. Callers must
// not replace its Transport or CheckRedirect: without the transport nothing is checked, and
// without CheckRedirect the redirect mode is lost. Errors of such a client are those of net/http
// and carry the full URL of the request.
func (c *Client) HTTPClient() *http.Client {
	hc := *c.http
	return &hc
}

// target is the part of a URL the checks look at.
type target struct {
	scheme string
	host   string // canonical host, see HostPolicy
	port   uint16
}

// decision is the result of the checks before the connection; the transport carries it in the
// request context to the dialer, so that the address check knows whether a rule with
// allow_private matched. The connection pool of net/http keys connections by scheme, host and
// port, from which the decision follows, so a pooled connection never serves a request with a
// different decision.
type decision struct {
	host         string
	allowPrivate bool
}

type decisionKey struct{}

// parseTarget extracts scheme, canonical host and port from u, or the reason why u is refused.
func parseTarget(u *url.URL) (target, *BlockedError) {
	if u == nil {
		return target{}, &BlockedError{Reason: ReasonURLNotAllowed}
	}
	host, ok := requestHost(u.Hostname())
	if !ok {
		return target{}, &BlockedError{Reason: ReasonURLNotAllowed}
	}
	if u.User != nil || u.Opaque != "" {
		return target{}, &BlockedError{Reason: ReasonURLNotAllowed, Host: host}
	}
	t := target{scheme: strings.ToLower(u.Scheme), host: host, port: defaultPort(strings.ToLower(u.Scheme))}
	if text := u.Port(); text != "" {
		n, err := strconv.ParseUint(text, 10, 16)
		if err != nil || n == 0 {
			return target{}, &BlockedError{Reason: ReasonURLNotAllowed, Host: host}
		}
		t.port = uint16(n)
	}
	return t, nil
}

// requestHost returns the canonical form of the host of a request URL: an IP literal unmapped
// as net/netip prints it, or a DNS name in lower case without one trailing dot. It refuses an
// empty host, a zone and names outside the grammar of ParseRule.
func requestHost(hostname string) (string, bool) {
	if hostname == "" || strings.Contains(hostname, "%") {
		return "", false
	}
	if addr, err := netip.ParseAddr(hostname); err == nil {
		return addr.Unmap().String(), true
	}
	name := strings.TrimSuffix(strings.ToLower(hostname), ".")
	if !validName(name) {
		return "", false
	}
	return name, true
}

// defaultPort returns the default port of an http or https scheme, else 0.
func defaultPort(scheme string) uint16 {
	switch scheme {
	case schemeHTTPS:
		return 443
	case schemeHTTP:
		return 80
	}
	return 0
}

// check runs the checks before the connection and returns the decision, or the refusal
// without logging it.
func (c *Client) check(req *http.Request) (decision, *BlockedError) {
	t, refused := parseTarget(req.URL)
	if refused != nil {
		return decision{}, refused
	}
	if c.denyAll {
		return decision{}, &BlockedError{Reason: ReasonHostNotAllowed, Host: t.host}
	}
	if t.scheme != schemeHTTPS && t.scheme != schemeHTTP {
		return decision{}, &BlockedError{Reason: ReasonSchemeNotAllowed, Host: t.host}
	}
	matched, allowPrivate := c.match(t)
	if t.scheme == schemeHTTP && !allowPrivate {
		return decision{}, &BlockedError{Reason: ReasonSchemeNotAllowed, Host: t.host}
	}
	if !matched && (c.policy == nil || !c.policy(req.Context(), t.scheme, t.host, t.port)) {
		return decision{}, &BlockedError{Reason: ReasonHostNotAllowed, Host: t.host}
	}
	return decision{host: t.host, allowPrivate: allowPrivate}, nil
}

// match reports whether a rule allows the target and whether a matching rule has allow_private.
// A rule without port matches only the default port of the scheme.
func (c *Client) match(t target) (matched, allowPrivate bool) {
	for _, r := range c.rules {
		if r.Host != t.host {
			continue
		}
		if r.Port == t.port || (r.Port == 0 && t.port == defaultPort(t.scheme)) {
			matched = true
			allowPrivate = allowPrivate || r.AllowPrivate
		}
	}
	return matched, allowPrivate
}

// block logs and counts a refused attempt once and returns its error. The record carries
// purpose, reason and host, never path, query, headers or bodies (19 SEC-NET-5, SEC-LOG-3).
func (c *Client) block(ctx context.Context, reason Reason, host string) error {
	c.logger.LogAttrs(ctx, slog.LevelWarn, logMessage,
		slog.String(attrPurpose, c.purpose),
		slog.String(attrReason, string(reason)),
		slog.String(attrHost, host))
	c.metrics.Blocked(ctx, reason)
	return &BlockedError{Reason: reason, Host: host}
}

// checkRedirect is the CheckRedirect of the client: RedirectNever hands the 3xx response back,
// RedirectChecked runs the checks before the connection on the next hop; the transport runs
// them again together with the address check.
func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if c.redirects != RedirectChecked {
		return http.ErrUseLastResponse
	}
	if len(via) >= maxRedirects {
		// The text of the default policy of net/http, which this function replaces.
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if _, refused := c.check(req); refused != nil {
		return c.block(req.Context(), ReasonRedirectNotAllowed, refused.Host)
	}
	return nil
}

// checkingTransport runs the checks before the connection for every request, also for clients
// from HTTPClient, then hands the request to the transport with the decision, the time limit
// and the body limit.
type checkingTransport struct {
	c    *Client
	next http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *checkingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c := t.c
	d, refused := c.check(req)
	if refused != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, c.block(req.Context(), refused.Reason, refused.Host)
	}
	// The time limit of each request; http.Client.Timeout of Do and HTTPClient bounds the whole
	// chain of hops, this deadline each hop, also for a library that wraps this transport in its
	// own client without a timeout.
	ctx, cancel := context.WithTimeout(context.WithValue(req.Context(), decisionKey{}, d), c.timeout)
	resp, err := t.next.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	if err := checkResponse(req, resp); err != nil {
		_ = resp.Body.Close()
		cancel()
		return nil, err
	}
	resp.Body = &limitedBody{rc: resp.Body, left: c.maxBytes, cancel: cancel}
	return resp, nil
}

// CloseIdleConnections closes the idle connections of the transport, so that
// http.Client.CloseIdleConnections of HTTPClient reaches it.
func (t *checkingTransport) CloseIdleConnections() { closeIdle(t.next) }

// closeIdle closes the idle connections of rt when it keeps any.
func closeIdle(rt http.RoundTripper) {
	if ci, ok := rt.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}

var (
	// errProtocolSwitch ends a 101 answer: net/http then hands the raw connection over as the
	// body and no deadline applies to it any more; no consumer upgrades a connection.
	errProtocolSwitch = errors.New("egress: the target switched protocols (101), which the client does not allow")
	// errInvalidLocation ends a redirect whose Location does not parse. net/http would end it
	// with an error that quotes the header, path and query included (19 SEC-LOG-3).
	errInvalidLocation = errors.New("egress: redirect response with an invalid Location header")
)

// checkResponse refuses the answers the client never hands to a caller: a protocol switch and a
// redirect with a Location that net/http could not parse.
func checkResponse(req *http.Request, resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusSwitchingProtocols:
		return errProtocolSwitch
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		if loc := resp.Header.Get("Location"); loc != "" {
			if _, err := req.URL.Parse(loc); err != nil {
				return errInvalidLocation
			}
		}
	}
	return nil
}

// dialContext is the DialContext of the transport: it resolves the host, keeps the addresses
// that pass the check and dials them in the order of the answer until one connects, each with a
// share of the time left. net/http detaches the dial from the cancellation of the request, so the
// dial bounds itself with the time limit of the client.
func (c *Client) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d, port, err := checkedTarget(ctx, address)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	allowed, err := c.checkedAddrs(ctx, d)
	if err != nil {
		return nil, err
	}
	var first error
	for i, a := range allowed {
		conn, err := c.dialOne(ctx, network, netip.AddrPortFrom(a, port).String(), len(allowed)-i)
		if err == nil {
			return conn, nil
		}
		if errors.Is(err, ErrBlocked) {
			return nil, c.block(ctx, ReasonAddressBlocked, d.host)
		}
		if first == nil {
			first = err
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil, first
}

// checkedTarget returns the decision the transport put into the dial context and the port of
// the dial address, after checking that the address names the host of that decision.
func checkedTarget(ctx context.Context, address string) (decision, uint16, error) {
	d, ok := ctx.Value(decisionKey{}).(decision)
	if !ok {
		return decision{}, 0, errors.New("egress: dial without a checked request")
	}
	hostname, portText, err := net.SplitHostPort(address)
	if err != nil {
		return decision{}, 0, fmt.Errorf("egress: dial address: %w", err)
	}
	host, ok := requestHost(hostname)
	port, perr := strconv.ParseUint(portText, 10, 16)
	if !ok || host != d.host || perr != nil {
		return decision{}, 0, errors.New("egress: dial address differs from the checked request")
	}
	return d, uint16(port), nil
}

// checkedAddrs resolves the host of d (an IP literal is its own answer) and returns the addresses
// that pass the address check, unmapped and without zone, in the order of the answer. When none
// passes, the attempt is refused with address_blocked.
func (c *Client) checkedAddrs(ctx context.Context, d decision) ([]netip.Addr, error) {
	var answer []netip.Addr
	if addr, err := netip.ParseAddr(d.host); err == nil {
		answer = []netip.Addr{addr}
	} else if answer, err = c.resolver.LookupNetIP(ctx, "ip", d.host); err != nil {
		return nil, err
	}
	var allowed []netip.Addr
	for _, a := range answer {
		if addrAllowed(a, d.allowPrivate) {
			allowed = append(allowed, canonicalAddr(a))
		}
	}
	if len(allowed) == 0 {
		return nil, c.block(ctx, ReasonAddressBlocked, d.host)
	}
	return allowed, nil
}

// dialOne dials one checked address with an equal share of the time left among the remaining
// addresses, so that an address that never answers leaves time for the next one, as net.Dialer
// does when it dials the addresses of one name.
func (c *Client) dialOne(ctx context.Context, network, address string, remaining int) (net.Conn, error) {
	if deadline, ok := ctx.Deadline(); ok && remaining > 1 {
		share := time.Until(deadline) / time.Duration(remaining)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, share)
		defer cancel()
	}
	return c.dial(ctx, network, address)
}

// connect dials one checked address. The control function checks the socket address once
// more, as a second line behind dialContext.
func connect(ctx context.Context, network, address string) (net.Conn, error) {
	d, _ := ctx.Value(decisionKey{}).(decision)
	dialer := net.Dialer{Control: controlFor(d.allowPrivate)} // nosemgrep: bothy-go-egress-only -- the egress package is the one place that builds transports (19 SEC-NET-1)
	return dialer.DialContext(ctx, network, address)
}

// controlFor returns a net.Dialer.Control that refuses a socket address failing the check.
func controlFor(allowPrivate bool) func(network, address string, _ syscall.RawConn) error {
	return func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil || !addrAllowed(ap.Addr(), allowPrivate) {
			return &BlockedError{Reason: ReasonAddressBlocked}
		}
		return nil
	}
}

// noMetrics counts nothing.
type noMetrics struct{}

func (noMetrics) Blocked(context.Context, Reason) {
	// Without a metrics port the client counts nothing; it still logs the attempt.
}

// limitedBody limits a response body to the limit of the client and ends the time limit of the
// request on Close.
type limitedBody struct {
	rc     io.ReadCloser
	left   int64
	cancel context.CancelFunc
	err    error
}

// Read returns at most the bytes left under the limit. When the body yields a byte beyond the
// limit, Read returns the bytes up to the limit with ErrResponseTooLarge and keeps returning the
// error; a body of exactly the limit ends with the error of the underlying body (io.EOF).
func (b *limitedBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	n, err := b.rc.Read(p)
	if int64(n) > b.left {
		n = int(b.left)
		b.left = 0
		b.err = ErrResponseTooLarge
		return n, b.err
	}
	b.left -= int64(n)
	return n, err
}

// Close closes the body and releases the time limit of the request.
func (b *limitedBody) Close() error {
	err := b.rc.Close()
	b.cancel()
	return err
}

// requestError is an error of Do without the path, query and user info that *url.Error would
// carry.
type requestError struct {
	op     string // method, as in url.Error.Op
	target string // scheme://host[:port]
	err    error
}

func (e *requestError) Error() string {
	return "egress: " + e.op + " " + e.target + ": " + e.err.Error()
}

func (e *requestError) Unwrap() error { return e.err }

// Timeout reports whether the cause is a timeout, as url.Error does.
func (e *requestError) Timeout() bool {
	var ne net.Error
	return errors.As(e.err, &ne) && ne.Timeout()
}

// redactURL replaces a *url.Error by a requestError that names only scheme and host.
func redactURL(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	// url.URL.Host holds neither user info nor path; the scheme and host are what a log needs.
	shown := "(invalid URL)"
	if u, perr := url.Parse(ue.URL); perr == nil && u.Host != "" {
		shown = u.Scheme + "://" + u.Host
	}
	return &requestError{op: ue.Op, target: shown, err: ue.Err}
}
