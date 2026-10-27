// SPDX-License-Identifier: Apache-2.0

package fakepush_test

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/fakepush"
)

// topic is the bundle ID of the example app (push.apns.topics of the overlay).
const topic = "com.example.chat"

// start runs a fakepush on loopback with free ports under the host name localhost, writing into
// a fresh directory, and returns it with that directory.
func start(t *testing.T) (*fakepush.Server, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := fakepush.Start(fakepush.Options{Dir: dir, Host: "localhost", APNsAddr: "127.0.0.1:0", FCMAddr: "127.0.0.1:0",
		CaptureAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

// TestStartWritesTheRunMaterial (brief M5-11 Design note 1): the directory holds exactly the
// APNs key, the FCM service account (both mode 0600, so the secret file driver of the server
// accepts them) and the CA (readable); the key is a PKCS #8 P-256 key the APNs adapter parses,
// the service account one the FCM adapter accepts for the token endpoint of the fake, and the
// CA verifies the certificates of both fakes for the host name, while its key is never written.
func TestStartWritesTheRunMaterial(t *testing.T) {
	t.Parallel()
	s, dir := start(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{fakepush.FileCA, fakepush.FileAPNsKey, fakepush.FileFCMAccount}
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Fatalf("files %v, want %v", names, want)
	}
	for name, mode := range map[string]os.FileMode{fakepush.FileAPNsKey: 0o600, fakepush.FileFCMAccount: 0o600, fakepush.FileCA: 0o644} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil || fi.Mode().Perm() != mode || !fi.Mode().IsRegular() {
			t.Fatalf("%s: mode %v, %v; want %v", name, fi.Mode(), err, mode)
		}
	}
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := sp.Resolve(t.Context(), "file:"+filepath.Join(dir, fakepush.FileAPNsKey))
	if err != nil {
		t.Fatalf("secret file driver refuses the APNs key: %v", err)
	}
	if _, err := apns.ParsePrivateKey(key); err != nil {
		t.Fatalf("APNs key: %v", err)
	}
	account, _, err := sp.Resolve(t.Context(), "file:"+filepath.Join(dir, fakepush.FileFCMAccount))
	if err != nil {
		t.Fatalf("secret file driver refuses the service account: %v", err)
	}
	if _, err := fcm.ParseServiceAccount(account, s.FCMURL()+"/token"); err != nil {
		t.Fatalf("service account: %v", err)
	}
	pool := caPool(t, dir)
	for _, base := range []string{s.APNsURL(), s.FCMURL()} {
		u, err := url.Parse(base)
		if err != nil || u.Scheme != "https" || u.Hostname() != "localhost" {
			t.Fatalf("URL %q: %v", base, err)
		}
		d := tls.Dialer{Config: &tls.Config{RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS12}}
		conn, err := d.DialContext(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", u.Port())) // nosemgrep: bothy-go-egress-only -- a TLS check of the in-process fake
		if err != nil {
			t.Fatalf("TLS to %s under the run CA: %v", base, err)
		}
		_ = conn.Close()
	}
	raw, err := os.ReadFile(filepath.Join(dir, fakepush.FileCA)) // #nosec G304 -- a file of the test directory
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("PRIVATE KEY")) {
		t.Fatal("ca.pem carries a private key")
	}
}

// TestStartNeedsAnExistingDirectory: a missing directory is an error of Start, and nothing
// listens afterwards.
func TestStartNeedsAnExistingDirectory(t *testing.T) {
	t.Parallel()
	s, err := fakepush.Start(fakepush.Options{Dir: filepath.Join(t.TempDir(), "missing"), Host: "localhost",
		APNsAddr: "127.0.0.1:0", FCMAddr: "127.0.0.1:0", CaptureAddr: "127.0.0.1:0"})
	if err == nil {
		_ = s.Close()
		t.Fatal("Start without the directory succeeded")
	}
}

// caPool returns a pool with the CA the run wrote.
func caPool(t *testing.T, dir string) *x509.CertPool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, fakepush.FileCA)) // #nosec G304 -- a file of the test directory
	if err != nil {
		t.Fatal(err)
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		t.Fatalf("ca.pem is not one certificate")
	}
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !ca.IsCA {
		t.Fatalf("ca.pem: CA %v, %v", ca != nil && ca.IsCA, err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return pool
}

// adapters builds the real APNs and FCM adapters of the server against s: keys from the files of
// the run through the secret file driver, the run CA as the only root, the fakes in
// egress.allowed_hosts with allow_private, as the E2E server build in the Compose overlay.
func adapters(t *testing.T, s *fakepush.Server, dir string) (*apns.Adapter, *fcm.Adapter) {
	t.Helper()
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	cfg := egress.Config{Timeout: testkit.Patience, MaxResponseBytes: 1 << 16}
	for _, base := range []string{s.APNsURL(), s.FCMURL()} {
		r, err := egress.ParseRule(strings.TrimPrefix(base, "https://") + ";allow_private")
		if err != nil {
			t.Fatal(err)
		}
		cfg.AllowedHosts = append(cfg.AllowedHosts, r)
	}
	ed := egress.Deps{RootCAs: caPool(t, dir)}
	a, err := apns.New(t.Context(), apns.Config{TeamID: fakepush.APNsTeamID, KeyID: fakepush.APNsKeyID,
		PrivateKeyRef: "file:" + filepath.Join(dir, fakepush.FileAPNsKey), Topics: []string{topic},
		JWTRefreshInterval: 40 * time.Minute, Retry5xxDelay: 15 * time.Minute, TTL: 24 * time.Hour},
		apns.Deps{Egress: cfg, EgressDeps: ed, Secrets: sp, Endpoints: map[string]string{"production": s.APNsURL(), "sandbox": s.APNsURL()}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := fcm.New(t.Context(), fcm.Config{ProjectID: fakepush.FCMProjectID, ServiceAccountRef: "file:" + filepath.Join(dir, fakepush.FileFCMAccount),
		TTL: 24 * time.Hour, PauseFor: 5 * time.Minute},
		fcm.Deps{Egress: cfg, EgressDeps: ed, Secrets: sp, Endpoints: fcm.Endpoints{Send: s.FCMURL(), Token: s.FCMURL() + "/token"}})
	if err != nil {
		t.Fatal(err)
	}
	return a, f
}

// hint returns a message hint for one new message in a thread.
func hint() notifyapp.Notification {
	return notifyapp.Notification{ID: ids.New(), InstanceID: ids.New(), ThreadID: ids.New(), MessageID: ids.New(),
		Kind: string(domain.KindMessage), Count: 1, Badge: 1, HasBadge: true}
}

// registration returns Alex's registration of provider with token.
func registration(provider, env, token string) notifyapp.Registration {
	return notifyapp.Registration{ID: ids.New(), UserID: ids.New(), DeviceID: ids.New(), Provider: provider, Environment: env,
		AppID: topic, Token: token}
}

// item is one captured send as the capture API returns it.
type item struct {
	Seq      int64             `json:"seq"`
	Provider string            `json:"provider"`
	Token    string            `json:"token"`
	Topic    string            `json:"topic"`
	PushType string            `json:"push_type"`
	Priority string            `json:"priority"`
	Payload  json.RawMessage   `json:"payload"`
	Data     map[string]string `json:"data"`
	Android  json.RawMessage   `json:"android"`
}

// captured reads GET /captured with query and returns status, content type and items.
func captured(t *testing.T, s *fakepush.Server, query string) (int, string, []item) {
	t.Helper()
	status, ct, body := get(t, s.CaptureURL()+"/captured"+query)
	if status != http.StatusOK {
		return status, ct, nil
	}
	var page struct {
		Items []item `json:"items"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&page); err != nil || page.Items == nil {
		t.Fatalf("captured%s: %q: %v", query, body, err)
	}
	return status, ct, page.Items
}

// get sends a GET to target and returns status, content type and body.
func get(t *testing.T, target string) (int, string, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req) // nosemgrep: bothy-go-egress-only -- a test client of the in-process capture API on loopback
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), b
}

// TestRealAdaptersReachTheCaptureAPI (brief M5-11 Design note 1, contract "Capture API"): the
// APNs and FCM adapters of the server, configured as in the overlay, send to the fakes, which
// accept with 200; the capture API returns both sends in order with seq from 1: APNs with token,
// topic, push type, priority and the body exactly as sent, FCM with token, data and android.
func TestRealAdaptersReachTheCaptureAPI(t *testing.T) {
	t.Parallel()
	s, dir := start(t)
	a, f := adapters(t, s, dir)
	apnsToken := strings.Repeat("ab", 32)
	if res, err := a.Send(t.Context(), registration("apns", "sandbox", apnsToken), hint()); err != nil || res.Class != domain.ResultAccepted {
		t.Fatalf("APNs send: %+v, %v", res, err)
	}
	fcmToken := "alex-phone-token:APA91bEXAMPLE"
	n := hint()
	if res, err := f.Send(t.Context(), registration("fcm", "production", fcmToken), n); err != nil || res.Class != domain.ResultAccepted {
		t.Fatalf("FCM send: %+v, %v", res, err)
	}
	status, ct, items := captured(t, s, "")
	if status != http.StatusOK || !strings.HasPrefix(ct, "application/json") || len(items) != 2 {
		t.Fatalf("captured: %d %q %+v", status, ct, items)
	}
	ap, fc := items[0], items[1]
	if ap.Seq != 1 || ap.Provider != "apns" || ap.Token != apnsToken || ap.Topic != topic || ap.PushType == "" || ap.Priority == "" {
		t.Fatalf("APNs item %+v", ap)
	}
	var aps map[string]json.RawMessage
	if err := json.Unmarshal(ap.Payload, &aps); err != nil || aps["aps"] == nil || ap.Data != nil {
		t.Fatalf("APNs payload %s: %v", ap.Payload, err)
	}
	if fc.Seq != 2 || fc.Provider != "fcm" || fc.Token != fcmToken || fc.Data["nid"] != n.ID.String() || len(fc.Android) == 0 || fc.Payload != nil {
		t.Fatalf("FCM item %+v", fc)
	}
}

// TestCaptureAPIOrderAndAfter (contract "Capture API"): items come in increasing seq from 1;
// after=<seq> returns only later items, beyond the last none; a missing after means 0; an after
// that is not a whole number is 400; a refused send is never captured; /healthz answers 200;
// other paths are 404 and other methods 405.
func TestCaptureAPIOrderAndAfter(t *testing.T) {
	t.Parallel()
	s, dir := start(t)
	a, f := adapters(t, s, dir)
	for i, provider := range []string{"apns", "fcm", "apns", "fcm"} {
		var err error
		var res domain.SendResult
		if provider == "apns" {
			res, err = a.Send(t.Context(), registration("apns", "production", strings.Repeat(strconv.Itoa(i), 64)), hint())
		} else {
			res, err = f.Send(t.Context(), registration("fcm", "production", "token-"+strconv.Itoa(i)), hint())
		}
		if err != nil || res.Class != domain.ResultAccepted {
			t.Fatalf("send %d: %+v, %v", i, res, err)
		}
	}
	// A request without a provider token: Fake-APNs answers 403 and the capture stays the same.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, s.APNsURL()+"/3/device/"+strings.Repeat("cd", 32), strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Transport: &http.Transport{ // nosemgrep: bothy-go-egress-only -- a test client of the in-process fake on loopback
		TLSClientConfig: &tls.Config{RootCAs: caPool(t, dir), MinVersion: tls.VersionTLS12}, ForceAttemptHTTP2: true}}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("send without JWT: %d", resp.StatusCode)
	}
	for query, want := range map[string][]int64{"": {1, 2, 3, 4}, "?after=0": {1, 2, 3, 4}, "?after=2": {3, 4}, "?after=4": {}, "?after=99": {}} {
		_, _, items := captured(t, s, query)
		var seqs []int64
		for _, it := range items {
			seqs = append(seqs, it.Seq)
		}
		if !slices.Equal(seqs, want) {
			t.Errorf("captured%s: seq %v, want %v", query, seqs, want)
		}
	}
	_, _, all := captured(t, s, "")
	if all[0].Provider != "apns" || all[1].Provider != "fcm" || all[2].Provider != "apns" || all[3].Provider != "fcm" || all[1].Token != "token-1" {
		t.Errorf("order %+v", all)
	}
	for _, query := range []string{"?after=-1", "?after=x", "?after=1.5", "?after=", "?after=1&after=2"} {
		if status, _, _ := get(t, s.CaptureURL()+"/captured"+query); status != http.StatusBadRequest {
			t.Errorf("captured%s: %d, want 400", query, status)
		}
	}
	if status, _, _ := get(t, s.CaptureURL()+"/healthz"); status != http.StatusOK {
		t.Errorf("healthz: %d", status)
	}
	if status, _, _ := get(t, s.CaptureURL()+"/other"); status != http.StatusNotFound {
		t.Errorf("other path: %d", status)
	}
	post, err := http.NewRequestWithContext(t.Context(), http.MethodPost, s.CaptureURL()+"/captured", nil)
	if err != nil {
		t.Fatal(err)
	}
	pr, err := http.DefaultClient.Do(post) // nosemgrep: bothy-go-egress-only -- a test client of the in-process capture API on loopback
	if err != nil {
		t.Fatal(err)
	}
	_ = pr.Body.Close()
	if pr.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST captured: %d", pr.StatusCode)
	}
}

// TestCloseClosesEveryServer: Close ends the capture API and both fakes, so a connection opened
// before it (an idle keep-alive connection to the capture API, a TLS connection to each fake) is
// closed by the server. The check uses connections that exist before Close, never a new dial to
// a freed port, which a parallel test could bind again.
func TestCloseClosesEveryServer(t *testing.T) {
	t.Parallel()
	s, dir := start(t)
	u, err := url.Parse(s.CaptureURL())
	if err != nil {
		t.Fatal(err)
	}
	ctx := testkit.Context(t)
	var d net.Dialer
	capture, err := d.DialContext(ctx, "tcp", u.Host) // nosemgrep: bothy-go-egress-only -- a raw connection to the in-process capture API
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = capture.Close() }()
	if err := capture.SetDeadline(testkit.Deadline()); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(capture, "GET /healthz HTTP/1.1\r\nHost: fakepush\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(capture)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Close {
		t.Fatalf("healthz on a keep-alive connection: %d, close %v", resp.StatusCode, resp.Close)
	}
	type open struct {
		name string
		conn net.Conn
		r    io.Reader
	}
	conns := []open{{"capture API", capture, br}}
	for _, f := range []struct{ name, base string }{{"Fake-APNs", s.APNsURL()}, {"Fake-FCM", s.FCMURL()}} {
		fu, err := url.Parse(f.base)
		if err != nil {
			t.Fatal(err)
		}
		td := tls.Dialer{Config: &tls.Config{RootCAs: caPool(t, dir), ServerName: "localhost", MinVersion: tls.VersionTLS12}}
		c, err := td.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", fu.Port())) // nosemgrep: bothy-go-egress-only -- a TLS connection to the in-process fake
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.Close() }()
		conns = append(conns, open{f.name, c, c})
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for _, o := range conns {
		// A deadline per read, so a server left open never uses up the wait of the next one.
		if err := o.conn.SetReadDeadline(testkit.Deadline()); err != nil {
			t.Fatal(err)
		}
		n, err := o.r.Read(make([]byte, 1))
		var ne net.Error
		if err == nil || n != 0 || (errors.As(err, &ne) && ne.Timeout()) {
			t.Errorf("%s: read after Close gave %d bytes, %v; want the connection closed by the server", o.name, n, err)
		}
	}
}
