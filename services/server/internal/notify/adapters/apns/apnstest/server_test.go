// SPDX-License-Identifier: Apache-2.0

package apnstest_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

const (
	team = "EXAMPLETEAM"
	kid  = "EXAMPLEKEY"
)

// token is a fixture device token: 64 hex digits.
var token = strings.Repeat("ab", 32)

// fake starts a Fake with a fresh key and returns it with the key.
func fake(t *testing.T) (*apnstest.Server, *ecdsa.PrivateKey) {
	t.Helper()
	key := newKey(t)
	srv, err := apnstest.Start(apnstest.Options{TeamID: team, KeyID: kid, PublicKey: &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv, key
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// jwt signs a provider token with key, kid and iss.
func jwt(t *testing.T, key *ecdsa.PrivateKey, keyID, iss string) string {
	t.Helper()
	payload := []byte(`{"iss":"` + iss + `","iat":` + strconv.FormatInt(time.Now().Unix(), 10) + `}`)
	tok, err := jose.SignCompact(jose.ES256Signer{Key: key}, jose.Header{Kid: keyID}, payload)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// client returns an egress client with HTTP/2 that trusts the Fake.
func client(t *testing.T, srv *apnstest.Server) *egress.Client {
	t.Helper()
	r, err := egress.ParseRule(srv.EgressRule())
	if err != nil {
		t.Fatal(err)
	}
	c, err := egress.New(egress.Config{AllowedHosts: []egress.Rule{r}, Timeout: 10 * time.Second, MaxResponseBytes: 1 << 16},
		egress.Options{Purpose: "test", HTTP2: true}, egress.Deps{RootCAs: srv.RootCAs()})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type answer struct {
	status int
	reason string
	ts     int64
	apnsID string
	proto  string
}

// send sends one request and returns the answer.
func send(t *testing.T, srv *apnstest.Server, method, path, bearer string) answer {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL()+path, bytes.NewReader([]byte(`{"aps":{"badge":1}}`)))
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "bearer "+bearer)
	}
	req.Header.Set("Apns-Id", "0192f1d3-0a55-7b21-8c4d-5e6f7a8b9c0d")
	resp, err := client(t, srv).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	a := answer{status: resp.StatusCode, apnsID: resp.Header.Get("Apns-Id"), proto: resp.Proto}
	if len(b) > 0 {
		var body struct {
			Reason    string `json:"reason"`
			Timestamp int64  `json:"timestamp"`
		}
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatalf("body %q: %v", b, err)
		}
		a.reason, a.ts = body.Reason, body.Timestamp
	}
	return a
}

func TestFakeVerifiesJWT(t *testing.T) {
	t.Parallel()
	srv, key := fake(t)
	path := "/3/device/" + token
	if a := send(t, srv, http.MethodPost, path, ""); a.status != http.StatusForbidden || a.reason != "MissingProviderToken" {
		t.Errorf("no JWT: %+v", a)
	}
	for name, bearer := range map[string]string{
		"other key": jwt(t, newKey(t), kid, team),
		"wrong kid": jwt(t, key, "OTHERKEY", team),
		"wrong iss": jwt(t, key, kid, "OTHERTEAM"),
		"garbage":   "not.a.jwt",
	} {
		if a := send(t, srv, http.MethodPost, path, bearer); a.status != http.StatusForbidden || a.reason != "InvalidProviderToken" {
			t.Errorf("%s: %+v", name, a)
		}
	}
	srv.Script(token, apnstest.Response{Status: http.StatusGone, Reason: "Unregistered", Timestamp: time.UnixMilli(1760000000123)})
	a := send(t, srv, http.MethodPost, path, jwt(t, key, kid, team))
	if a.status != http.StatusGone || a.reason != "Unregistered" || a.ts != 1760000000123 {
		t.Errorf("valid JWT: %+v, want the scripted 410", a)
	}
	if a.apnsID != "0192f1d3-0a55-7b21-8c4d-5e6f7a8b9c0d" || a.proto != "HTTP/2.0" {
		t.Errorf("apns-id %q proto %q", a.apnsID, a.proto)
	}
	reqs := srv.Requests()
	last := reqs[len(reqs)-1]
	if last.Token != token || last.KeyID != kid || last.Issuer != team || last.IssuedAt.IsZero() || last.Proto != "HTTP/2.0" {
		t.Errorf("recorded %+v", last)
	}
	rotated := newKey(t)
	srv.SetPublicKey(&rotated.PublicKey)
	if a := send(t, srv, http.MethodPost, path, jwt(t, key, kid, team)); a.reason != "InvalidProviderToken" {
		t.Errorf("old key after SetPublicKey: %+v", a)
	}
	if a := send(t, srv, http.MethodPost, path, jwt(t, rotated, kid, team)); a.status != http.StatusOK {
		t.Errorf("new key after SetPublicKey: %+v", a)
	}
}

func TestFakeRequiresPostAndPath(t *testing.T) {
	t.Parallel()
	srv, key := fake(t)
	bearer := jwt(t, key, kid, team)
	if a := send(t, srv, http.MethodGet, "/3/device/"+token, bearer); a.status != http.StatusMethodNotAllowed || a.reason != "MethodNotAllowed" {
		t.Errorf("GET: %+v", a)
	}
	for _, p := range []string{"/3/devices/" + token, "/3/device/", "/3/device/" + token + "/x", "/"} {
		if a := send(t, srv, http.MethodPost, p, bearer); a.status != http.StatusNotFound || a.reason != "BadPath" {
			t.Errorf("%s: %+v", p, a)
		}
	}
	if srv.Connections() < 1 {
		t.Error("no connection counted")
	}
	if !strings.HasSuffix(srv.EgressRule(), ";allow_private") || !strings.HasPrefix(srv.EgressRule(), "127.0.0.1:") {
		t.Errorf("EgressRule %q", srv.EgressRule())
	}
}

func TestFakeScriptOrder(t *testing.T) {
	t.Parallel()
	srv, key := fake(t)
	bearer := jwt(t, key, kid, team)
	srv.Script(token, apnstest.Response{Status: 503, Reason: "ServiceUnavailable"}, apnstest.Response{Status: 429, Reason: "TooManyRequests"})
	var got []int
	for range 3 {
		got = append(got, send(t, srv, http.MethodPost, "/3/device/"+token, bearer).status)
	}
	if got[0] != 503 || got[1] != 429 || got[2] != 200 {
		t.Errorf("statuses %v, want [503 429 200]", got)
	}
	if n := len(srv.Requests()); n != 3 {
		t.Errorf("%d requests recorded, want 3", n)
	}
}

// TestFakeNotInBinary: the server binary never depends on Fake-APNs (pattern of identitytest).
func TestFakeNotInBinary(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "../../../../../cmd/bothy").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "/internal/notify/app") {
		t.Fatalf("go list output does not look like the dependencies of cmd/bothy:\n%.400s", out)
	}
	if strings.Contains(string(out), "/internal/notify/adapters/apns/apnstest") {
		t.Fatal("cmd/bothy depends on apnstest")
	}
}

// TestFakeStampsAcceptedAt (brief M3-24, t1 of 13 section 9): a request answered with 200 carries
// AcceptedAt between the moment the client sent it and the moment the client read the 200; a
// request rejected with 403 carries none.
func TestFakeStampsAcceptedAt(t *testing.T) {
	t.Parallel()
	srv, key := fake(t)
	path := "/3/device/" + token
	before := time.Now()
	if a := send(t, srv, http.MethodPost, path, jwt(t, key, kid, team)); a.status != http.StatusOK {
		t.Fatalf("valid JWT: %+v", a)
	}
	after := time.Now()
	if a := send(t, srv, http.MethodPost, path, jwt(t, newKey(t), kid, team)); a.status != http.StatusForbidden {
		t.Fatalf("other key: %+v", a)
	}
	reqs := srv.Requests()
	if len(reqs) != 2 {
		t.Fatalf("%d requests recorded, want 2", len(reqs))
	}
	if at := reqs[0].AcceptedAt; at.Before(before) || at.After(after) {
		t.Errorf("AcceptedAt %s outside [%s, %s]", at, before, after)
	}
	if !reqs[1].AcceptedAt.IsZero() {
		t.Errorf("a 403 carries AcceptedAt %s, want zero", reqs[1].AcceptedAt)
	}
}

// TestRequestsShowsOnlyAnsweredRequests (review of M3-24): a reader polling Requests while
// requests arrive never sees an accepted request without its AcceptedAt, and every request shows
// up once it is answered.
func TestRequestsShowsOnlyAnsweredRequests(t *testing.T) {
	t.Parallel()
	srv, key := fake(t)
	bearer := jwt(t, key, kid, team)
	const n = 20
	done := make(chan struct{})
	bad := make(chan string, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			for _, r := range srv.Requests() {
				if r.JWT == bearer && r.AcceptedAt.IsZero() {
					select {
					case bad <- "an accepted request without AcceptedAt":
					default:
					}
				}
			}
		}
	}()
	for range n {
		if a := send(t, srv, http.MethodPost, "/3/device/"+token, bearer); a.status != http.StatusOK {
			t.Fatalf("send: %+v", a)
		}
	}
	close(done)
	select {
	case msg := <-bad:
		t.Fatal(msg)
	default:
	}
	if got := len(srv.Requests()); got != n {
		t.Fatalf("%d requests, want %d", got, n)
	}
}

// leafFor returns a test CA pool and a TLS leaf for host signed by that CA, as fakepush (M5-11)
// serves under its per-run CA.
func leafFor(t *testing.T, host string) (*x509.CertPool, *tls.Certificate) {
	t.Helper()
	caKey, leafKey := newKey(t), newKey(t)
	now := time.Now()
	caTmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "apnstest CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return pool, &tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}
}

// TestFakeListensOnAddrUnderHostAndCertificate (brief M5-11 Design note 1): with Addr, Host and
// Certificate the Fake listens on the given address, names Host in URL and EgressRule, and
// serves HTTP/2 under the given certificate, so a client that trusts only the issuing CA reaches
// it by the host name; OnAccept sees the accepted request.
func TestFakeListensOnAddrUnderHostAndCertificate(t *testing.T) {
	t.Parallel()
	key := newKey(t)
	pool, leaf := leafFor(t, "localhost")
	accepted := make(chan apnstest.Request, 1)
	srv, err := apnstest.Start(apnstest.Options{TeamID: team, KeyID: kid, PublicKey: &key.PublicKey,
		Addr: "127.0.0.1:0", Host: "localhost", Certificate: leaf, OnAccept: func(r apnstest.Request) { accepted <- r }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL())
	if err != nil || u.Scheme != "https" || u.Hostname() != "localhost" || u.Port() == "" || u.Path != "" {
		t.Fatalf("URL %q: %v", srv.URL(), err)
	}
	if srv.EgressRule() != "localhost:"+u.Port()+";allow_private" {
		t.Fatalf("EgressRule %q", srv.EgressRule())
	}
	if !bytes.Equal(srv.Certificate().Raw, leaf.Certificate[0]) {
		t.Fatal("Certificate is not the given leaf")
	}
	r, err := egress.ParseRule(srv.EgressRule())
	if err != nil {
		t.Fatal(err)
	}
	c, err := egress.New(egress.Config{AllowedHosts: []egress.Rule{r}, Timeout: testkit.Patience, MaxResponseBytes: 1 << 16},
		egress.Options{Purpose: "test", HTTP2: true}, egress.Deps{RootCAs: pool})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL()+"/3/device/"+token, strings.NewReader(`{"aps":{"badge":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "bearer "+jwt(t, key, kid, team))
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Proto != "HTTP/2.0" {
		t.Fatalf("status %d proto %s", resp.StatusCode, resp.Proto)
	}
	got := testkit.Receive(t, "the accepted request", accepted)
	if got.Token != token || string(got.Body) != `{"aps":{"badge":1}}` || got.AcceptedAt.IsZero() {
		t.Fatalf("OnAccept got %+v", got)
	}
}

// TestFakeOnAcceptSeesOnlyAcceptedRequests: OnAccept runs once per request answered with 200, in
// the order of acceptance, and never for a refused or scripted non-200 answer.
func TestFakeOnAcceptSeesOnlyAcceptedRequests(t *testing.T) {
	t.Parallel()
	key := newKey(t)
	var mu sync.Mutex
	var seen []string
	srv, err := apnstest.Start(apnstest.Options{TeamID: team, KeyID: kid, PublicKey: &key.PublicKey, OnAccept: func(r apnstest.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.AcceptedAt.IsZero() {
			t.Error("OnAccept got a request without AcceptedAt")
		}
		seen = append(seen, r.Token)
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	other := strings.Repeat("cd", 32)
	srv.Script(other, apnstest.Response{Status: http.StatusGone, Reason: "Unregistered"})
	bearer := jwt(t, key, kid, team)
	if a := send(t, srv, http.MethodPost, "/3/device/"+token, ""); a.status != http.StatusForbidden {
		t.Fatalf("no JWT: %+v", a)
	}
	for _, tok := range []string{other, token, other} {
		send(t, srv, http.MethodPost, "/3/device/"+tok, bearer)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 || seen[0] != token || seen[1] != other {
		t.Fatalf("OnAccept saw %v, want [%s %s]", seen, token, other)
	}
}

// TestFakeListenErrorIsReturned: an address that cannot be bound is an error of Start.
func TestFakeListenErrorIsReturned(t *testing.T) {
	t.Parallel()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	key := newKey(t)
	srv, err := apnstest.Start(apnstest.Options{TeamID: team, KeyID: kid, PublicKey: &key.PublicKey, Addr: ln.Addr().String()})
	if err == nil {
		srv.Close()
		t.Fatal("Start on a taken address succeeded")
	}
}
