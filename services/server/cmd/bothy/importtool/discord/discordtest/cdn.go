// SPDX-License-Identifier: Apache-2.0

package discordtest

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// The host names of the fake CDN. CDNHost and MediaHost stand for the two hosts of
// import.discord.cdn_hosts; ForeignHost is a host that no allow list of a test names, used for
// the negative cases (a foreign attachment URL, the target of a redirect). The certificate of
// the fake covers all three, so only the fetcher's own checks keep a request away from
// ForeignHost.
const (
	CDNHost     = "cdn.discord.test"
	MediaHost   = "media.discord.test"
	ForeignHost = "evil.test"
)

// CDNRequest is one entry of the request log of the fake CDN.
type CDNRequest struct {
	// Method is the HTTP method.
	Method string
	// Host is the host name the request named (without port).
	Host string
	// Path is the path.
	Path string
	// Query is the raw query string.
	Query string
	// Authorization reports whether the request carried an Authorization header.
	Authorization bool
	// Cookie reports whether the request carried a Cookie header.
	Cookie bool
	// Headers are the canonical names of the request headers, sorted.
	Headers []string
}

// CDNFile is a file the fake CDN serves on a path of every one of its hosts.
type CDNFile struct {
	// Size is the number of bytes served.
	Size int64
	// NoLength serves the file chunked, without Content-Length.
	NoLength bool
	// Signature, when set, is the only hm value of the query the CDN accepts; any other answers
	// 403, like an expired signed link ([R5]).
	Signature string
	// ContentType is the Content-Type of the answer; empty means application/octet-stream.
	ContentType string
}

// CDNResponse is an injected answer of the fake CDN.
type CDNResponse struct {
	// Status is the status code.
	Status int
	// Header are extra response headers.
	Header http.Header
	// Body is the response body.
	Body string
	// EchoURL replaces the body with the full request URL, query included, like an error page
	// that quotes the signed link.
	EchoURL bool
	// LieLength, when positive, sends a chunked body of LieLength bytes behind a smaller
	// Content-Length header; only CDNLie sets it.
	LieLength int64
	// lieDeclared is the Content-Length the lie announces.
	lieDeclared int64
}

// CDNStatus answers code with a short text body.
func CDNStatus(code int) CDNResponse {
	return CDNResponse{Status: code, Body: http.StatusText(code)}
}

// CDNRedirect answers 302 with the given Location.
func CDNRedirect(location string) CDNResponse {
	return CDNResponse{Status: http.StatusFound, Header: http.Header{"Location": {location}}}
}

// CDNRateLimit answers 429 with retry_after (seconds as JSON number text) in the body.
func CDNRateLimit(seconds string) CDNResponse {
	return CDNResponse{Status: http.StatusTooManyRequests, Header: http.Header{"Content-Type": {"application/json"}},
		Body: `{"message": "You are being rate limited.", "retry_after": ` + seconds + `, "global": false}`}
}

// CDNEcho answers code with a body that echoes the full request URL with its query.
func CDNEcho(code int) CDNResponse {
	return CDNResponse{Status: code, EchoURL: true}
}

// CDNLie answers 200 with a Content-Length header of declared bytes and then sends size bytes
// as a chunked body: an oversize body behind a small Content-Length.
func CDNLie(declared, size int64) CDNResponse {
	return CDNResponse{Status: http.StatusOK, LieLength: size, lieDeclared: declared}
}

// CDN is a running fake Discord CDN on 127.0.0.1 under CDNHost, MediaHost and ForeignHost with
// one port. Its fields are set by NewCDN and must not be changed.
type CDN struct {
	// Port is the port of the fake; every host name shares it.
	Port int
	// Resolver maps the three host names to 127.0.0.1.
	Resolver egress.Resolver
	// RootCAs trusts the certificate of the fake.
	RootCAs *x509.CertPool
	// Server is the underlying test server.
	Server *httptest.Server

	mu     sync.Mutex
	files  map[string]CDNFile
	queued map[string][]CDNResponse
	always map[string]CDNResponse
	log    []CDNRequest
}

// NewCDN starts a fake CDN on 127.0.0.1 with a free port; the test cleanup stops it.
func NewCDN(t testing.TB) *CDN {
	t.Helper()
	cert, pool := newCertificate(t, CDNHost, MediaHost, ForeignHost)
	c := &CDN{RootCAs: pool, Resolver: cdnResolver{}, files: map[string]CDNFile{},
		queued: map[string][]CDNResponse{}, always: map[string]CDNResponse{}}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(c.serve))
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("discordtest: listen: %v", err)
	}
	_ = srv.Listener.Close()
	srv.Listener = ln
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	c.Server = srv
	c.Port = ln.Addr().(*net.TCPAddr).Port
	return c
}

// Hosts returns the allow list of a test: CDNHost and MediaHost with the port of the fake, in
// the form "host:port" of import.discord.cdn_hosts entries that name a port.
func (c *CDN) Hosts() []string {
	p := strconv.Itoa(c.Port)
	return []string{CDNHost + ":" + p, MediaHost + ":" + p}
}

// EgressConfig returns an egress configuration with the test-only rules
// "<host>:<port>;allow_private" for all three host names, ForeignHost included, so that the
// fetcher's own allow list is what keeps requests away from ForeignHost.
func (c *CDN) EgressConfig(t testing.TB) egress.Config {
	t.Helper()
	var rules []egress.Rule
	for _, h := range []string{CDNHost, MediaHost, ForeignHost} {
		r, err := egress.ParseRule(h + ":" + strconv.Itoa(c.Port) + ";allow_private")
		if err != nil {
			t.Fatalf("discordtest: rule: %v", err)
		}
		rules = append(rules, r)
	}
	return egress.Config{AllowedHosts: rules, Timeout: 10 * time.Second, MaxResponseBytes: 1 << 20}
}

// URL returns https://<host>:<port><path> with the raw query appended when it is not empty.
func (c *CDN) URL(host, path, query string) string {
	u := "https://" + host + ":" + strconv.Itoa(c.Port) + path
	if query != "" {
		u += "?" + query
	}
	return u
}

// AddFile serves f on path (on every host of the fake).
func (c *CDN) AddFile(path string, f CDNFile) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[path] = f
}

// Inject queues answers for path; each request on that path takes the next one, and an empty
// queue falls back to Always and then to the files.
func (c *CDN) Inject(path string, rs ...CDNResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queued[path] = append(c.queued[path], rs...)
}

// Always answers every request on path with r once its queue is empty.
func (c *CDN) Always(path string, r CDNResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.always[path] = r
}

// Requests returns a copy of the request log.
func (c *CDN) Requests() []CDNRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.log)
}

// next returns the injected answer for path, if any.
func (c *CDN) next(path string) (CDNResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if q := c.queued[path]; len(q) > 0 {
		c.queued[path] = q[1:]
		return q[0], true
	}
	r, ok := c.always[path]
	return r, ok
}

// serve is the handler of the fake CDN.
func (c *CDN) serve(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	var names []string
	for k := range r.Header {
		names = append(names, k)
	}
	slices.Sort(names)
	c.mu.Lock()
	c.log = append(c.log, CDNRequest{Method: r.Method, Host: host, Path: r.URL.Path, Query: r.URL.RawQuery,
		Authorization: r.Header.Get("Authorization") != "", Cookie: r.Header.Get("Cookie") != "", Headers: names})
	file, known := c.files[r.URL.Path]
	c.mu.Unlock()
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if inj, ok := c.next(r.URL.Path); ok {
		c.answer(w, r, inj)
		return
	}
	switch {
	case !known:
		http.Error(w, "not found", http.StatusNotFound)
	case file.Signature != "" && r.URL.Query().Get("hm") != file.Signature:
		http.Error(w, "This content is no longer available.", http.StatusForbidden)
	default:
		serveFile(w, file)
	}
}

// answer writes an injected answer.
func (c *CDN) answer(w http.ResponseWriter, r *http.Request, inj CDNResponse) {
	if inj.LieLength > 0 {
		lie(w, inj.lieDeclared, inj.LieLength)
		return
	}
	for k, vs := range inj.Header {
		w.Header()[k] = vs
	}
	body := inj.Body
	if inj.EchoURL {
		body = "error for https://" + r.Host + r.URL.RequestURI()
	}
	w.WriteHeader(inj.Status)
	_, _ = io.WriteString(w, body) //nolint:gosec // G705: plain test bodies for the fetcher under test, never HTML in a browser
}

// serveFile writes the bytes of f.
func serveFile(w http.ResponseWriter, f CDNFile) {
	ct := f.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	if !f.NoLength {
		w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.CopyN(w, zeros{}, f.Size)
	if f.NoLength {
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}
}

// lie hijacks the connection and writes a 200 with Content-Length declared and a chunked body
// of size bytes; net/http would refuse to write such an answer itself.
func lie(w http.ResponseWriter, declared, size int64) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "no hijacker", http.StatusInternalServerError)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	writeLie(buf.Writer, declared, size)
}

// writeLie writes the lying answer to bw.
func writeLie(bw *bufio.Writer, declared, size int64) {
	_, _ = fmt.Fprintf(bw, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n"+
		"Transfer-Encoding: chunked\r\nConnection: close\r\n\r\n", declared)
	const chunk = 32 << 10
	left := size
	for left > 0 {
		n := min(left, chunk)
		_, _ = fmt.Fprintf(bw, "%x\r\n", n)
		_, _ = io.CopyN(bw, zeros{}, n)
		_, _ = io.WriteString(bw, "\r\n")
		left -= n
	}
	_, _ = io.WriteString(bw, "0\r\n\r\n")
	_ = bw.Flush()
}

// zeros is an endless reader of the byte 'x'.
type zeros struct{}

// Read fills p.
func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// cdnResolver maps the CDN host names to 127.0.0.1.
type cdnResolver struct{}

// LookupNetIP implements egress.Resolver.
func (cdnResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if host == CDNHost || host == MediaHost || host == ForeignHost {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// AttachmentMessage returns a raw message object with ID id in channel channelID by the user
// Alex that holds one attachment with ID attachmentID, file name, size and URL, for a fixture
// that answers the re-read of an expired attachment link.
func AttachmentMessage(id, channelID, attachmentID, filename string, size int64, rawURL string) json.RawMessage {
	return mustJSON(map[string]any{
		"id": id, "channel_id": channelID, "type": 0,
		"author":  map[string]any{"id": "13000011", "username": "alex", "global_name": "Alex", "bot": false},
		"content": "", "timestamp": "2026-03-01T10:00:00.000000+00:00", "pinned": false, "mentions": []any{},
		"attachments": []any{map[string]any{"id": attachmentID, "filename": filename, "size": size,
			"content_type": "application/octet-stream", "url": rawURL}},
	})
}
