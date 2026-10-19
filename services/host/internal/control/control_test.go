// SPDX-License-Identifier: Apache-2.0

package control_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/control/controltest"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// hostID is the host of the tests.
const hostID = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f"

// key is the fixed login key of the tests and the recording (SHA-256 of control-recording/host-auth).
func key() ed25519.PrivateKey {
	s := sha256.Sum256([]byte("control-recording/host-auth"))
	return ed25519.NewKeyFromSeed(s[:])
}

// The backoff of the tests: short, so reconnects are quick.
const (
	base     = 20 * time.Millisecond
	capDelay = 400 * time.Millisecond
	waitLong = 10 * time.Second
)

// newClient returns a client of srv with sessions (nil: RefuseSessions).
func newClient(t *testing.T, srv *controltest.Server, sessions control.Sessions) *control.Client {
	t.Helper()
	c, err := control.New(control.Config{ServerURL: srv.URL, HostID: hostID, AuthKey: key(), HostVersion: "1.0.0", BackoffBase: base,
		BackoffCap: capDelay, RootCAs: srv.Pool}, sessions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// ctx is bounded by waitLong.
func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), waitLong)
	t.Cleanup(cancel)
	return c
}

// handshake answers hello and identify of a new connection with ready (decl) and returns the
// connection and the identify of the client.
func handshake(t *testing.T, srv *controltest.Server, intervalMs int64, decl string) (*controltest.Conn, control.Identify) {
	t.Helper()
	c := srv.Accept(ctx(t))
	if err := c.Send(ctx(t), "hello", controltest.Hello(intervalMs, 3)); err != nil {
		t.Fatal(err)
	}
	var id control.Identify
	if err := controltest.Strict(c.Expect(t, ctx(t), "identify"), &id); err != nil {
		t.Fatal(err)
	}
	if err := c.Send(ctx(t), "ready", controltest.Ready(hostID, decl)); err != nil {
		t.Fatal(err)
	}
	return c, id
}

// sampleReport is a report of one session user with every check passed.
func sampleReport(osVersion string) startup.Report {
	var doc []doctor.ReportEntry
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login", "hd7_profiles",
		"hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doc = append(doc, doctor.ReportEntry{Check: c, OK: true})
	}
	return startup.Report{Profiles: []doctor.Profile{{Name: "shell", DisplayName: "Shell", Kind: "shell"}},
		SessionUsers: []doctor.ReportedSessionUser{{OSUser: "bothy-s-alex", Workdirs: []string{"Project A"}}}, Doctor: doc, OSVersion: osVersion}
}

// TestProofFormat (M2-43 Design note 12 b): header and claims exactly as 24 section 3.5 says, kid
// the RFC 7638 thumbprint, verifies with crypto/ed25519.
func TestProofFormat(t *testing.T) {
	t.Parallel()
	k := key()
	iat := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	p := control.Proof(k, hostID, "abc", iat)
	parts := strings.Split(p, ".")
	if len(parts) != 3 {
		t.Fatalf("%d parts", len(parts))
	}
	dec := base64.RawURLEncoding
	head, _ := dec.DecodeString(parts[0])
	claims, _ := dec.DecodeString(parts[1])
	sig, _ := dec.DecodeString(parts[2])
	pub := k.Public().(ed25519.PublicKey)
	sum := sha256.Sum256([]byte(`{"crv":"Ed25519","kty":"OKP","x":"` + dec.EncodeToString(pub) + `"}`))
	if string(head) != `{"alg":"EdDSA","typ":"host-auth+jwt","kid":"`+dec.EncodeToString(sum[:])+`"}` {
		t.Errorf("header %s", head)
	}
	if string(claims) != `{"challenge":"abc","host_id":"`+hostID+`","iat":`+strconv.FormatInt(iat.Unix(), 10)+`}` {
		t.Errorf("claims %s", claims)
	}
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		t.Error("signature does not verify")
	}
}

// TestChooseProtocolVersion: the highest common version; none is false.
func TestChooseProtocolVersion(t *testing.T) {
	t.Parallel()
	if v, ok := control.ChooseVersion([]int{1, 2}, []int{3, 2, 1}); !ok || v != 2 {
		t.Errorf("%d %v", v, ok)
	}
	if _, ok := control.ChooseVersion([]int{1}, []int{2}); ok {
		t.Error("no common version chosen")
	}
}

// hostControlCodes reads the codes of x-close-codes of the channel hostControl.
func hostControlCodes(t *testing.T) []int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "api", "asyncapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "\n  hostControl:\n")
	j := strings.Index(s[i+1:], "\n  hostData:\n")
	var out []int
	for _, m := range regexp.MustCompile(`- code: (\d+)`).FindAllStringSubmatch(s[i:i+1+j], -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

// TestClosePolicy (M2-43 Design note 12 f): one row per code of hostControl x-close-codes plus
// an unknown code.
func TestClosePolicy(t *testing.T) {
	t.Parallel()
	want := map[int]control.Kind{
		1000: control.Backoff, 1001: control.Backoff, 4000: control.Backoff, 4002: control.Backoff, 4008: control.Wait,
		4009: control.Backoff, 4013: control.Soon, 4014: control.Backoff, 4100: control.Backoff, 4110: control.Stop,
		4111: control.WaitEndPTYs, 4112: control.Stop, 4113: control.Backoff, 4114: control.Stop, 4999: control.Backoff,
	}
	codes := append(hostControlCodes(t), 4999)
	for _, code := range codes {
		k, ok := want[code]
		if !ok {
			t.Errorf("code %d of x-close-codes has no row", code)
			continue
		}
		if got := control.Policy(code, "", capDelay); got.Kind != k {
			t.Errorf("%d: %v, want %v", code, got.Kind, k)
		}
	}
	if r := control.Policy(4008, "retry_after=7", capDelay); r.Delay != 7*time.Second {
		t.Errorf("4008 with retry_after: %v", r)
	}
	if r := control.Policy(4008, "", capDelay); r.Delay != capDelay {
		t.Errorf("4008 without retry_after: %v", r)
	}
	if r := control.Policy(4111, "", capDelay); r.Delay != capDelay {
		t.Errorf("4111: %v", r)
	}
}

// TestBackoff (M2-43 Design note 12 f): full jitter below base * 2^n, never above the cap; Reset
// starts again.
func TestBackoff(t *testing.T) {
	t.Parallel()
	b := control.NewBackoff(time.Second, 10*time.Second, func() float64 { return 0.999999 })
	var got []time.Duration
	for range 6 {
		got = append(got, b.Next().Round(time.Second))
	}
	if !slices.Equal(got, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 10 * time.Second, 10 * time.Second}) {
		t.Errorf("ceilings %v", got)
	}
	b.Reset()
	if d := b.Next().Round(time.Second); d != time.Second {
		t.Errorf("after reset %v", d)
	}
	zero := control.NewBackoff(time.Second, 10*time.Second, func() float64 { return 0 })
	if zero.Next() != 0 || zero.Soon() != 0 {
		t.Error("full jitter starts at zero")
	}
	if s := b.Soon(); s >= time.Second {
		t.Errorf("soon %v not below the base", s)
	}
}

// TestControlImplementsStartupControl (M2-43 Design note 12): Login returns the declaration of
// ready, a second ready reaches the ready handler, Report sends report, Close closes with 1000.
func TestControlImplementsStartupControl(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	var sc startup.Control = c
	got := make(chan startup.Ready, 1)
	c.SetReadyHandler(func(_ context.Context, r startup.Ready) { got <- r })
	done := make(chan startup.Ready, 1)
	go func() {
		r, err := sc.Login(ctx(t))
		if err != nil {
			t.Error(err)
		}
		done <- r
	}()
	conn, id := handshake(t, srv, 60000, "organization")
	if r := <-done; r.LLMCredentials != "organization" {
		t.Fatalf("login %+v", r)
	}
	if id.HostID != hostID || id.ProtocolVersion != 1 || id.LiveSessions == nil || len(id.LiveSessions) != 0 {
		t.Errorf("identify %+v", id)
	}
	if err := sc.Report(ctx(t), sampleReport("Debian 13")); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, ctx(t), "report")
	if err := conn.Send(ctx(t), "ready", controltest.Ready(hostID, "personal")); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, ctx(t), "report") // the current report first after every ready
	select {
	case r := <-got:
		if r.LLMCredentials != "personal" {
			t.Errorf("second ready %+v", r)
		}
	case <-ctx(t).Done():
		t.Fatal("the second ready did not reach the handler")
	}
	if err := sc.Close(); err != nil {
		t.Fatal(err)
	}
	f, _ := conn.Next(ctx(t))
	if f.Err == nil || f.Code != 1000 {
		t.Errorf("close %+v", f)
	}
}

// TestReportIsFirstFrameAfterReady and TestReportResentOnChange (M2-43 Design note 12 c): after a
// reconnect the stored report is the first frame after ready; a changed report is sent at once.
func TestReportIsFirstFrameAfterReady(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 60000, "none")
	if err := c.Report(ctx(t), sampleReport("Debian 13")); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, ctx(t), "report")
	conn.Close(1000, "")
	conn2, id := handshake(t, srv, 60000, "none")
	if len(id.LiveSessions) != 0 {
		t.Errorf("live_sessions %v", id.LiveSessions)
	}
	var r startup.Report
	if err := controltest.Strict(conn2.Expect(t, ctx(t), "report"), &r); err != nil || r.OSVersion != "Debian 13" {
		t.Fatalf("first frame after ready: %+v %v", r, err)
	}
	if err := c.Report(ctx(t), sampleReport("Debian 14")); err != nil {
		t.Fatal(err)
	}
	if err := controltest.Strict(conn2.Expect(t, ctx(t), "report"), &r); err != nil || r.OSVersion != "Debian 14" {
		t.Fatalf("changed report: %+v %v", r, err)
	}
}

func TestReportResentOnChange(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 60000, "none")
	for _, v := range []string{"Debian 13", "Debian 14"} {
		if err := c.Report(ctx(t), sampleReport(v)); err != nil {
			t.Fatal(err)
		}
		var r startup.Report
		if err := controltest.Strict(conn.Expect(t, ctx(t), "report"), &r); err != nil || r.OSVersion != v {
			t.Fatalf("report %+v %v, want %s", r, err, v)
		}
	}
}

// TestPeerIsLocal (24 section 2.3): loopback and an own address are local; another address is not.
func TestPeerIsLocal(t *testing.T) {
	t.Parallel()
	own := []net.IP{net.ParseIP("192.0.2.10")}
	for ip, want := range map[string]bool{"127.0.0.1": true, "::1": true, "192.0.2.10": true, "192.0.2.11": false} {
		if got := control.PeerIsLocal(net.ParseIP(ip), own); got != want {
			t.Errorf("%s: %v", ip, got)
		}
	}
	if control.PeerIsLocal(nil, own) {
		t.Error("no address is not local")
	}
}

// TestServerHostFlagFromPeer (24 section 2.3): a server reached on loopback sets is_server_host.
func TestServerHostFlagFromPeer(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 60000, "none")
	if err := c.Report(ctx(t), sampleReport("Debian 13")); err != nil {
		t.Fatal(err)
	}
	var r startup.Report
	if err := controltest.Strict(conn.Expect(t, ctx(t), "report"), &r); err != nil || !r.IsServerHost {
		t.Fatalf("is_server_host %+v %v", r, err)
	}
}

// TestJWKSFromHelloAndHeartbeatAck (24 section 5.3): the key set of hello, replaced by the one of a
// heartbeat_ack with jwks; a heartbeat_ack without jwks keeps it.
func TestJWKSFromHelloAndHeartbeatAck(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 60000, "none")
	waitFor(t, "hello keys", func() bool { return kids(c) == "terminal-key-1" })
	if err := conn.Send(ctx(t), "heartbeat_ack", map[string]any{"server_time": "2026-10-14T09:00:00.000Z"}); err != nil {
		t.Fatal(err)
	}
	if err := conn.Send(ctx(t), "heartbeat_ack", map[string]any{"jwks": map[string]any{"keys": []any{controltest.Key("terminal-key-1"),
		controltest.Key("terminal-key-2")}}}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "new keys", func() bool { return kids(c) == "terminal-key-1,terminal-key-2" })
}

// kids returns the kids of the key store, joined.
func kids(c *control.Client) string {
	var out []string
	for _, k := range c.Keys().Keys() {
		s, _ := k["kid"].(string)
		out = append(out, s)
	}
	return strings.Join(out, ",")
}

// waitFor waits until cond holds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitLong)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestHeartbeatAndServerSilence (M2-43 Design note 12 d): after the report the client sends
// heartbeat every heartbeat_interval_ms; a silent server is closed with 4009 after the interval
// times (miss limit + 1), and the client reconnects.
func TestHeartbeatAndServerSilence(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 50, "none")
	if err := c.Report(ctx(t), sampleReport("Debian 13")); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, ctx(t), "report")
	conn.Expect(t, ctx(t), "heartbeat")
	var code int
	for {
		f, err := conn.Next(ctx(t))
		if err != nil {
			t.Fatal(err)
		}
		if f.Err != nil {
			code = f.Code
			break
		}
	}
	if code != 4009 {
		t.Fatalf("close %d, want 4009", code)
	}
	handshake(t, srv, 60000, "none")
}

// TestSessionOpenBeforeReportCloses4002 (24 section 2.2 point 5): session.open before the own
// report closes with 4002 and the client reconnects.
func TestSessionOpenBeforeReportCloses4002(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 60000, "none")
	if err := conn.Send(ctx(t), "session.open", map[string]any{"session_id": hostID, "data_path": "/x", "profile": "shell",
		"expires_at": "2026-10-14T09:00:20.000Z"}); err != nil {
		t.Fatal(err)
	}
	f, _ := conn.Next(ctx(t))
	if f.Err == nil || f.Code != 4002 {
		t.Fatalf("got %+v, want close 4002", f)
	}
	handshake(t, srv, 60000, "none")
}

// TestRefuseSessionsAnswersOpenFailed (M2-43 Design note 12 e): until M2-45 every session.open is
// answered with session.ended {open_failed, pty_failed}.
func TestRefuseSessionsAnswersOpenFailed(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 60000, "none")
	if err := c.Report(ctx(t), sampleReport("Debian 13")); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, ctx(t), "report")
	session := "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e10"
	if err := conn.Send(ctx(t), "session.open", map[string]any{"session_id": session, "data_path": "/x", "profile": "shell",
		"expires_at": "2026-10-14T09:00:20.000Z"}); err != nil {
		t.Fatal(err)
	}
	var e control.SessionEnded
	if err := controltest.Strict(conn.Expect(t, ctx(t), "session.ended"), &e); err != nil {
		t.Fatal(err)
	}
	if e.SessionID != session || e.CloseReason != "open_failed" || e.CloseDetail != "pty_failed" {
		t.Errorf("session.ended %+v", e)
	}
}

// recordingSessions records EndAll.
type recordingSessions struct {
	control.RefuseSessions
	mu    gosync.Mutex
	ended int
}

func (r *recordingSessions) EndAll(context.Context) {
	r.mu.Lock()
	r.ended++
	r.mu.Unlock()
}

// TestStopCodesReturnErrStop (M2-43 Design note 12 f): 4110, 4112 and 4114 end all PTYs and stop
// with StopError; no reconnect follows.
func TestStopCodesReturnErrStop(t *testing.T) {
	t.Parallel()
	for _, code := range []int{4110, 4112, 4114} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			srv := controltest.New(t)
			s := &recordingSessions{}
			c := newClient(t, srv, s)
			go func() { _, _ = c.Login(ctx(t)) }()
			conn, _ := handshake(t, srv, 60000, "none")
			conn.Close(code, "")
			select {
			case <-c.Stopped():
			case <-ctx(t).Done():
				t.Fatal("not stopped")
			}
			var stop *control.StopError
			if !errors.As(c.Err(), &stop) || stop.Code != code {
				t.Errorf("err %v", c.Err())
			}
			s.mu.Lock()
			ended := s.ended
			s.mu.Unlock()
			if ended != 1 {
				t.Errorf("EndAll %d times", ended)
			}
			time.Sleep(3 * capDelay)
			if n := srv.Count(); n != 1 {
				t.Errorf("%d connections, want no reconnect", n)
			}
		})
	}
}

// TestRefusedStart_ReportsOnceAndStops (M2-43 Design note 12 g, D-W-91, TM-T21 (g)): the
// sequence of a refused start over the client: one login, one report, close 1000 over the same
// connection; with the server down Login gives up after the cap with ErrUnreachable.
func TestRefusedStart_ReportsOnceAndStops(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	done := make(chan error, 1)
	go func() {
		if _, err := c.Login(ctx(t)); err != nil {
			done <- err
			return
		}
		if err := c.Report(ctx(t), sampleReport("Debian 13")); err != nil {
			done <- err
			return
		}
		done <- c.Close()
	}()
	conn, id := handshake(t, srv, 60000, "organization")
	if len(id.LiveSessions) != 0 {
		t.Errorf("live_sessions %v", id.LiveSessions)
	}
	conn.Expect(t, ctx(t), "report")
	f, _ := conn.Next(ctx(t))
	if f.Err == nil || f.Code != 1000 {
		t.Fatalf("got %+v, want close 1000", f)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if srv.Count() != 1 {
		t.Errorf("%d logins", srv.Count())
	}

	down := controltest.New(t)
	down.Close()
	c2 := newClient(t, down, nil)
	start := time.Now()
	if _, err := c2.Login(ctx(t)); !errors.Is(err, startup.ErrUnreachable) {
		t.Fatalf("server down: %v", err)
	}
	if d := time.Since(start); d < capDelay || d > waitLong/2 {
		t.Errorf("gave up after %v, want the cap %v", d, capDelay)
	}
}

// TestControlRefusesPlainAndUnverifiedTLS (R-TM-01): a ws:// or http:// URL is refused; an
// untrusted certificate never logs in.
func TestControlRefusesPlainAndUnverifiedTLS(t *testing.T) {
	t.Parallel()
	for _, u := range []string{"ws://127.0.0.1:1", "http://127.0.0.1:1", "wss://127.0.0.1:1", "https://"} {
		if _, err := control.New(control.Config{ServerURL: u, HostID: hostID, AuthKey: key(), BackoffBase: base, BackoffCap: capDelay}, nil); !errors.Is(err, control.ErrNotHTTPS) {
			t.Errorf("%s: %v", u, err)
		}
	}
	srv := controltest.New(t)
	c, err := control.New(control.Config{ServerURL: srv.URL, HostID: hostID, AuthKey: key(), HostVersion: "1.0.0", BackoffBase: base,
		BackoffCap: capDelay}, nil) // system roots only: the test certificate is untrusted
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Login(ctx(t)); !errors.Is(err, startup.ErrUnreachable) {
		t.Fatalf("untrusted certificate: %v", err)
	}
	if srv.Count() != 0 {
		t.Error("connected through an untrusted certificate")
	}
}

// TestLateReadyReachesHandler (M2-43): when Login gave up before the first ready (the server
// answered too late), that ready reaches the ready handler, also when the handler is set after it
// arrived.
func TestLateReadyReachesHandler(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	conn := make(chan *controltest.Conn, 1)
	go func() { conn <- srv.Accept(ctx(t)) }()
	if _, err := c.Login(ctx(t)); !errors.Is(err, startup.ErrUnreachable) {
		t.Fatalf("login: %v", err)
	}
	k := <-conn
	if err := k.Send(ctx(t), "hello", controltest.Hello(60000, 3)); err != nil {
		t.Fatal(err)
	}
	k.Expect(t, ctx(t), "identify")
	if err := k.Send(ctx(t), "ready", controltest.Ready(hostID, "personal")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	got := make(chan startup.Ready, 1)
	c.SetReadyHandler(func(_ context.Context, r startup.Ready) { got <- r })
	select {
	case r := <-got:
		if r.LLMCredentials != "personal" {
			t.Errorf("ready %+v", r)
		}
	case <-ctx(t).Done():
		t.Fatal("the late ready never reached the handler")
	}
}
