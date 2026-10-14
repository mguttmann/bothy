// SPDX-License-Identifier: Apache-2.0

// Package discordtest is a fake Discord REST API for tests of the import tool
// (docs/REUSE.md 4.1 "Fake server with fault injection" and 4.3 "Secret leak test"). It serves
// the read calls of docs/spec/22-import-migration.md 4.1 from a fixture over TLS on 127.0.0.1
// with a port the system assigns, under the host name discord.test, logs every request, answers
// every method other than GET with 405 and injects faults per path: status codes, 429 with
// retry_after in the body, in the header or in both, X-RateLimit-Remaining 0, redirects and an
// echo of the bot token and a signed query into the error body and a header.
//
// Only _test.go files import this package; a test in package discord holds that it never
// reaches the binary bothy.
//
// Example:
//
//	fake := discordtest.New(t, discordtest.BasicFixture(t))
//	client, err := discord.New(discord.Config{APIBase: fake.BaseURL, Egress: fake.EgressConfig(), ...},
//		token, discord.Deps{Resolver: fake.Resolver, RootCAs: fake.RootCAs})
//	fake.Inject("/api/v10/users/@me", discordtest.Status(http.StatusUnauthorized))
package discordtest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// Host is the host name of the fake in base URLs; the resolver of the fake maps it to
// 127.0.0.1 and the certificate of the fake is issued for it.
const Host = "discord.test"

// OtherHost is a second name the resolver maps to the fake; no certificate and no rule cover
// it, so a client that tried to follow a redirect there would fail. Tests use it as the target
// of redirects.
const OtherHost = "other.test"

// APIPath is the path prefix of the fake, like the version path of the real API base.
const APIPath = "/api/v10"

// SignedQuery is the invented query of a signed CDN link that the echo fault places into
// error bodies and headers; the secret leak test searches every output for it.
const SignedQuery = "ex=6a6b6c6d&is=6a6a6a6a&hm=echoechoechoecho0123456789abcdef0123456789abcdef0123456789abcd"

// Page is one page of an archived thread list as the fixture holds it.
type Page struct {
	// HasMore is the has_more flag of the page.
	HasMore bool `json:"has_more"`
	// Threads are the raw thread objects of the page, newest first.
	Threads []json.RawMessage `json:"threads"`
}

// Fixture is the content of a fake Discord server: raw Discord objects as the API returns
// them, so that a test can also serve invalid objects (an ID that is not a digit string).
type Fixture struct {
	// Me is the user object of GET /users/@me.
	Me json.RawMessage `json:"me"`
	// Guild is the guild object of GET /guilds/{guild.id}; its "id" selects the guild.
	Guild json.RawMessage `json:"guild"`
	// Channels are the channels of GET /guilds/{guild.id}/channels.
	Channels []json.RawMessage `json:"channels"`
	// ActiveThreads are the threads of GET /guilds/{guild.id}/threads/active.
	ActiveThreads []json.RawMessage `json:"active_threads"`
	// ArchivedPublic are the pages of GET /channels/{channel.id}/threads/archived/public per
	// channel ID. The first request (without before) gets page 0; a request whose before equals
	// the archive_timestamp of the last thread of page i gets page i+1, and one beyond the last
	// page gets the last page again, like a server that repeats itself.
	ArchivedPublic map[string][]Page `json:"archived_public"`
	// ArchivedPrivate are the pages of the private list in the same form; a channel without an
	// entry answers 403 (the bot lacks MANAGE_THREADS).
	ArchivedPrivate map[string][]Page `json:"archived_private"`
	// Messages are the messages per channel or thread ID in ascending ID order.
	Messages map[string][]json.RawMessage `json:"messages"`
}

// LoadFixture reads a fixture file and fails the test when it does not parse.
func LoadFixture(t testing.TB, path string) *Fixture {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- a fixture path chosen by the test
	if err != nil {
		t.Fatalf("discordtest: read fixture: %v", err)
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("discordtest: parse fixture %s: %v", path, err)
	}
	return &f
}

// BasicFixture loads testdata/guild_basic.json of the discord package: one guild with two
// categories, a text, an announcement, a forum, a voice channel and a channel without category,
// active threads, archived threads over three pages, a private archived list that answers 403
// and messages of every type of 22 6.1 (replies, forwards, stickers, polls, embeds, attachments,
// pins, reactions, a webhook and a bot author). Later tickets reuse it from any package.
func BasicFixture(t testing.TB) *Fixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("discordtest: no caller")
	}
	return LoadFixture(t, filepath.Join(filepath.Dir(file), "..", "testdata", "guild_basic.json"))
}

// Request is one entry of the request log of the fake.
type Request struct {
	// Method is the HTTP method.
	Method string
	// Host is the host name the request named (without port).
	Host string
	// Path is the path, APIPath included.
	Path string
	// Query is the raw query string.
	Query string
	// Authorization reports whether the request carried an Authorization header.
	Authorization bool
	// UserAgent is the User-Agent header.
	UserAgent string
}

// Response is an injected answer. With Status 0 the fake serves the fixture answer and adds
// Header (and the echo header); otherwise it answers Status with Header and Body.
type Response struct {
	// Status is the status code; 0 means the fixture answer.
	Status int
	// Header are extra response headers.
	Header http.Header
	// Body is the response body of a non-fixture answer.
	Body string
	// Echo places the received Authorization value and SignedQuery into a header and, for a
	// non-fixture answer, into the body.
	Echo bool
}

// Status answers code with a Discord-style JSON error body.
func Status(code int) Response {
	return Response{Status: code, Body: fmt.Sprintf(`{"message": %q, "code": 0}`, http.StatusText(code))}
}

// RateLimitBody answers 429 with retry_after (seconds as JSON number text) in the body only.
func RateLimitBody(seconds string) Response {
	return Response{Status: http.StatusTooManyRequests,
		Body: `{"message": "You are being rate limited.", "retry_after": ` + seconds + `, "global": false}`}
}

// RateLimitHeader answers 429 with the header Retry-After only; the body has no retry_after.
func RateLimitHeader(seconds string) Response {
	return Response{Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {seconds}},
		Body: `{"message": "You are being rate limited.", "global": false}`}
}

// RateLimitBoth answers 429 with retry_after in the body and Retry-After in the header.
func RateLimitBoth(body, header string) Response {
	r := RateLimitBody(body)
	r.Header = http.Header{"Retry-After": {header}}
	return r
}

// RateLimitGlobal answers a global 429 (X-RateLimit-Global: true) with retry_after in the body.
func RateLimitGlobal(seconds string) Response {
	return Response{Status: http.StatusTooManyRequests, Header: http.Header{"X-Ratelimit-Global": {"true"}},
		Body: `{"message": "You are being rate limited.", "retry_after": ` + seconds + `, "global": true}`}
}

// RemainingZero serves the fixture answer with X-RateLimit-Remaining 0, the given
// X-RateLimit-Reset-After (omitted when empty) and the bucket (omitted when empty).
func RemainingZero(resetAfter, bucket string) Response {
	h := http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Limit": {"5"}}
	if resetAfter != "" {
		h.Set("X-RateLimit-Reset-After", resetAfter)
	}
	if bucket != "" {
		h.Set("X-RateLimit-Bucket", bucket)
	}
	return Response{Header: h}
}

// Redirect answers 302 with the given Location.
func Redirect(location string) Response {
	return Response{Status: http.StatusFound, Header: http.Header{"Location": {location}}}
}

// Echo answers code (0: the fixture answer) and echoes the token and SignedQuery.
func Echo(code int) Response {
	r := Response{Echo: true}
	if code != 0 {
		r = Status(code)
		r.Echo = true
	}
	return r
}

// AnyPath as the path of Inject or Always matches every request.
const AnyPath = "*"

// Fake is a running fake Discord server. Its fields are set by New and must not be changed.
type Fake struct {
	// BaseURL is the API base of the fake: https://discord.test:<port>/api/v10.
	BaseURL string
	// Rule is the test-only egress rule "discord.test:<port>;allow_private".
	Rule egress.Rule
	// Resolver maps Host and OtherHost to 127.0.0.1.
	Resolver egress.Resolver
	// RootCAs trusts the certificate of the fake.
	RootCAs *x509.CertPool
	// Server is the underlying test server.
	Server *httptest.Server

	fixture *Fixture
	mu      sync.Mutex
	log     []Request
	queued  map[string][]Response
	always  map[string]Response
}

// New starts a fake for fixture on 127.0.0.1 with a free port; the test cleanup stops it.
func New(t testing.TB, fixture *Fixture) *Fake {
	t.Helper()
	cert, pool := newCertificate(t)
	f := &Fake{fixture: fixture, RootCAs: pool, queued: map[string][]Response{}, always: map[string]Response{}}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(f.serve))
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("discordtest: listen: %v", err)
	}
	_ = srv.Listener.Close()
	srv.Listener = ln
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	f.Server = srv
	port := ln.Addr().(*net.TCPAddr).Port
	f.BaseURL = "https://" + Host + ":" + strconv.Itoa(port) + APIPath
	rule, err := egress.ParseRule(Host + ":" + strconv.Itoa(port) + ";allow_private")
	if err != nil {
		t.Fatalf("discordtest: rule: %v", err)
	}
	f.Rule = rule
	f.Resolver = resolver{}
	return f
}

// EgressConfig returns an egress configuration that allows exactly the fake (Rule) with test
// limits.
func (f *Fake) EgressConfig() egress.Config {
	return egress.Config{AllowedHosts: []egress.Rule{f.Rule}, Timeout: 10 * time.Second, MaxResponseBytes: 1 << 20}
}

// Inject queues answers for path (APIPath included, or AnyPath); each request on that path
// takes the next one, and an empty queue falls back to Always and then to the fixture.
func (f *Fake) Inject(path string, rs ...Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queued[path] = append(f.queued[path], rs...)
}

// Always answers every request on path (or AnyPath) with r once its queue is empty.
func (f *Fake) Always(path string, r Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.always[path] = r
}

// Requests returns a copy of the request log.
func (f *Fake) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.log)
}

// Methods returns the sorted set of methods in the request log.
func (f *Fake) Methods() []string {
	var out []string
	for _, r := range f.Requests() {
		if !slices.Contains(out, r.Method) {
			out = append(out, r.Method)
		}
	}
	slices.Sort(out)
	return out
}

// next returns the injected answer for path, if any.
func (f *Fake) next(path string) (Response, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, key := range []string{path, AnyPath} {
		if q := f.queued[key]; len(q) > 0 {
			f.queued[key] = q[1:]
			return q[0], true
		}
	}
	for _, key := range []string{path, AnyPath} {
		if r, ok := f.always[key]; ok {
			return r, true
		}
	}
	return Response{}, false
}

// serve is the handler of the fake.
func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	f.mu.Lock()
	f.log = append(f.log, Request{Method: r.Method, Host: host, Path: r.URL.Path, Query: r.URL.RawQuery,
		Authorization: r.Header.Get("Authorization") != "", UserAgent: r.Header.Get("User-Agent")})
	f.mu.Unlock()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, []byte(`{"message": "405: Method Not Allowed", "code": 0}`))
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bot ") {
		writeJSON(w, http.StatusUnauthorized, []byte(`{"message": "401: Unauthorized", "code": 0}`))
		return
	}
	inj, injected := f.next(r.URL.Path)
	if injected && inj.Echo {
		w.Header().Set("X-Echo", r.Header.Get("Authorization")+" https://cdn.example.com/a/b.png?"+SignedQuery)
	}
	if injected {
		for k, vs := range inj.Header {
			w.Header()[k] = vs
		}
	}
	if injected && inj.Status != 0 {
		body := inj.Body
		if inj.Echo {
			body = fmt.Sprintf(`{"message": %q, "code": 0}`, "echo "+r.Header.Get("Authorization")+
				" https://cdn.example.com/a/b.png?"+SignedQuery)
		}
		writeJSON(w, inj.Status, []byte(body))
		return
	}
	status, body := f.route(r)
	writeJSON(w, status, body)
}

// writeJSON writes a JSON answer.
func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body) //nolint:gosec // G705: JSON fixture bodies for the client under test, never HTML

}

// Path segments of the routes.
const (
	segChannels = "channels"
	segThreads  = "threads"
)

// notFound is the body of an unknown resource.
var notFound = []byte(`{"message": "Unknown Channel", "code": 10003}`)

// route answers a GET from the fixture.
func (f *Fake) route(r *http.Request) (int, []byte) {
	rest, ok := strings.CutPrefix(r.URL.Path, APIPath+"/")
	if !ok {
		return http.StatusNotFound, notFound
	}
	parts := strings.Split(rest, "/")
	switch {
	case rest == "users/@me":
		return http.StatusOK, f.fixture.Me
	case len(parts) >= 2 && parts[0] == "guilds":
		return f.guildRoute(parts[1], parts[2:])
	case len(parts) == 2 && parts[0] == segChannels:
		return f.channel(parts[1])
	case len(parts) == 3 && parts[0] == segChannels && parts[2] == "messages":
		return f.messages(parts[1], r)
	case len(parts) == 5 && parts[0] == segChannels && parts[2] == segThreads && parts[3] == "archived":
		return f.archived(parts[1], parts[4], r.URL.Query().Get("before"))
	}
	return http.StatusNotFound, notFound
}

// guildRoute answers the guild calls.
func (f *Fake) guildRoute(id string, rest []string) (int, []byte) {
	var g struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(f.fixture.Guild, &g)
	if id != g.ID {
		return http.StatusNotFound, []byte(`{"message": "Unknown Guild", "code": 10004}`)
	}
	switch strings.Join(rest, "/") {
	case "":
		return http.StatusOK, f.fixture.Guild
	case segChannels:
		return http.StatusOK, mustJSON(f.fixture.Channels)
	case "threads/active":
		return http.StatusOK, mustJSON(map[string]any{segThreads: f.fixture.ActiveThreads, "members": []any{}})
	}
	return http.StatusNotFound, notFound
}

// channel answers GET /channels/{id} from channels, active and archived threads.
func (f *Fake) channel(id string) (int, []byte) {
	all := slices.Concat(f.fixture.Channels, f.fixture.ActiveThreads)
	for _, pages := range f.fixture.ArchivedPublic {
		for _, p := range pages {
			all = append(all, p.Threads...)
		}
	}
	for _, raw := range all {
		if objectID(raw) == id {
			return http.StatusOK, raw
		}
	}
	return http.StatusNotFound, notFound
}

// archived answers the archived thread lists by the page rule of Fixture.ArchivedPublic.
func (f *Fake) archived(channelID, kind, before string) (int, []byte) {
	lists := f.fixture.ArchivedPublic
	if kind == "private" {
		lists = f.fixture.ArchivedPrivate
	} else if kind != "public" {
		return http.StatusNotFound, notFound
	}
	pages, ok := lists[channelID]
	if !ok {
		if kind == "private" {
			return http.StatusForbidden, []byte(`{"message": "Missing Access", "code": 50001}`)
		}
		return http.StatusOK, []byte(`{"threads": [], "members": [], "has_more": false}`)
	}
	index := 0
	if before != "" {
		index = len(pages) - 1
		for i, p := range pages {
			if len(p.Threads) > 0 && archiveTimestamp(p.Threads[len(p.Threads)-1]) == before && i+1 < len(pages) {
				index = i + 1
				break
			}
		}
	}
	p := pages[index]
	return http.StatusOK, mustJSON(map[string]any{segThreads: p.Threads, "members": []any{}, "has_more": p.HasMore})
}

// messages answers the message list with after, before or around, newest first like Discord
// (the client sorts).
func (f *Fake) messages(channelID string, r *http.Request) (int, []byte) {
	list, ok := f.fixture.Messages[channelID]
	if !ok {
		return http.StatusNotFound, notFound
	}
	q := r.URL.Query()
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	page := selectMessages(list, q, limit)
	slices.SortFunc(page, func(a, b json.RawMessage) int { return compareIDs(objectID(b), objectID(a)) })
	return http.StatusOK, mustJSON(page)
}

// selectMessages picks the messages of an after, before or around query, or the newest.
func selectMessages(list []json.RawMessage, q url.Values, limit int) []json.RawMessage {
	var page []json.RawMessage
	switch {
	case q.Has("after"):
		for _, m := range list {
			if idLess(q.Get("after"), objectID(m)) && len(page) < limit {
				page = append(page, m)
			}
		}
	case q.Has("before"):
		for i := len(list) - 1; i >= 0; i-- {
			if idLess(objectID(list[i]), q.Get("before")) && len(page) < limit {
				page = append(page, list[i])
			}
		}
	case q.Has("around"):
		page = around(list, q.Get("around"), limit)
	default:
		page = slices.Clone(list[max(0, len(list)-limit):])
	}
	return page
}

// around returns up to limit messages centered on id.
func around(list []json.RawMessage, id string, limit int) []json.RawMessage {
	center := len(list)
	for i, m := range list {
		if !idLess(objectID(m), id) {
			center = i
			break
		}
	}
	start := max(0, center-limit/2)
	end := min(len(list), start+limit)
	return slices.Clone(list[start:end])
}

// objectID returns the "id" of a raw object.
func objectID(raw json.RawMessage) string {
	var o struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &o)
	return o.ID
}

// archiveTimestamp returns thread_metadata.archive_timestamp of a raw thread.
func archiveTimestamp(raw json.RawMessage) string {
	var o struct {
		Meta struct {
			ArchiveTimestamp string `json:"archive_timestamp"`
		} `json:"thread_metadata"`
	}
	_ = json.Unmarshal(raw, &o)
	return o.Meta.ArchiveTimestamp
}

// compareIDs orders digit strings numerically (shorter first, then lexically).
func compareIDs(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

// idLess reports a < b for digit strings.
func idLess(a, b string) bool { return compareIDs(a, b) < 0 }

// mustJSON marshals v, which is always marshalable here.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// resolver maps Host and OtherHost to 127.0.0.1.
type resolver struct{}

// LookupNetIP implements egress.Resolver.
func (resolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if host == Host || host == OtherHost {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// newCertificate creates a self-signed certificate for Host and a pool that trusts it.
func newCertificate(t testing.TB) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("discordtest: key: %v", err)
	}
	// The validity window uses the wall clock, because TLS verification in the client checks it
	// against the wall clock too.
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: Host},
		DNSNames:              []string{Host},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("discordtest: certificate: %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("discordtest: parse certificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}
