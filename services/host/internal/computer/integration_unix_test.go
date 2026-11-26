// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package computer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/confirm"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/session"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// TestIntegration runs the host service side of a computer session end to end against the fake
// server of the session tests: the control client with the manager, computer.open, the real data
// connection with the Noise handshake, a fake helper on the real Unix socket endpoint in a
// temporary folder (admitted as the test peer of this user), a screenshot with its image chunks,
// done, and computer.ended on the control connection.
func TestIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	signer := sessiontest.NewSigner("terminal-key-1")
	srv := sessiontest.NewServer(t, func() map[string]any {
		return sessiontest.Hello([]map[string]any{signer.JWK()}, sessiontest.DefaultValues)
	})
	static, _ := noise.GenerateKeyPair(rand.Reader)
	_, auth, _ := ed25519.GenerateKey(rand.Reader)

	dir, err := os.MkdirTemp("", "cu")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	endpoint := filepath.Join(dir, "computer.sock")
	l, err := confirm.Listen(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	me := strconv.Itoa(os.Getuid())
	var cl *control.Client
	var mgr *Manager
	h := NewHelper(HelperOptions{Listener: l, PeerOf: confirm.PeerOf, MaxBytes: testFrameMax,
		Rule:    PeerRule{GOOS: hostos.Linux, TestPeerUID: me, Console: confirm.Rule{ServiceUID: "0"}},
		OnEvent: func(e Event) { mgr.HelperEvent(e) }})
	mgr = NewManager(Config{HostID: tHost, NoiseKey: static, HostVersion: "1.0.0", Platform: "linux", Helper: h,
		Dial: func(ctx context.Context, path, sid string) (Stream, error) {
			url, err := cl.DataURL(path)
			if err != nil {
				return nil, err
			}
			return session.DialStream(ctx, cl.HTTPClient(), url, session.StreamIdentify{HostID: tHost, HostVersion: "1.0.0", SessionID: sid},
				func(ch string) string { return control.Proof(auth, tHost, ch, time.Now()) })
		},
		OnReport: func() { _ = cl.ComputerChanged(context.Background()) }})
	cl, err = control.New(control.Config{ServerURL: srv.URL, HostID: tHost, AuthKey: auth, HostVersion: "1.0.0", BackoffBase: time.Second,
		BackoffCap: time.Minute, RootCAs: srv.Pool, Computers: mgr}, nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = h.Serve(ctx) }()
	t.Cleanup(func() { mgr.Shutdown(); _ = cl.Close() })

	// The helper connects to the real endpoint.
	hc, err := (&net.Dialer{}).DialContext(ctx, "unix", endpoint)
	if err != nil {
		t.Fatal(err)
	}
	fh := startFakeHelper(t, hc, hello())
	deadline := time.Now().Add(wait)
	for !h.State().Connected && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !h.State().Connected {
		t.Fatal("the helper on the real endpoint was not admitted")
	}

	// Login: hello, identify, ready, report with the computer object.
	go func() { _, _ = cl.Login(ctx) }()
	cc := srv.AcceptControl(ctx)
	if err := cc.Send(ctx, "hello", sessiontest.Hello([]map[string]any{signer.JWK()}, sessiontest.DefaultValues)); err != nil {
		t.Fatal(err)
	}
	expectOp(ctx, t, cc, "identify")
	if err := cc.Send(ctx, "ready", map[string]any{"host_id": tHost, "state": "active", "llm_credentials": "none"}); err != nil {
		t.Fatal(err)
	}
	if err := cl.Report(ctx, sampleStartupReport()); err != nil {
		t.Fatal(err)
	}
	rep := expectOp(ctx, t, cc, "report")
	var r struct {
		Computer control.ComputerReport `json:"computer"`
	}
	if err := json.Unmarshal(rep, &r); err != nil || r.Computer.Helper != "connected" || !r.Computer.Enabled || r.Computer.Generation != 3 {
		t.Fatalf("report computer %+v %v", r.Computer, err)
	}

	sid := sidA
	if err := cc.Send(ctx, "computer.open", openOp(sid)); err != nil {
		t.Fatal(err)
	}
	dc := srv.Data(ctx, sid, 1)
	eph, _ := noise.GenerateKeyPair(rand.Reader)
	sidb, _ := noise.ParseID(sid)
	hostb, _ := noise.ParseID(tHost)
	init, _ := noise.NewInitiator(noise.InitiatorConfig{HostStatic: static.Public, SessionID: sidb, HostID: hostb, Ephemeral: eph, Label: noise.ComputerPrologueLabel})
	cek, _ := noise.EphemeralKeyHash(eph.Public[:])
	tok := signer.SignRaw(map[string]any{"alg": "EdDSA", "kid": signer.Kid, "typ": TokenType}, claimsFor(sid, cek))
	m1, _ := init.WriteMessage1([]byte(tok))
	if err := dc.SendBinary(ctx, m1); err != nil {
		t.Fatal(err)
	}
	m2, err := dc.NextBinary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, tr, err := init.ReadMessage2(m2)
	if err != nil {
		t.Fatal(err)
	}
	next := func() frame.Frame {
		t.Helper()
		b, err := dc.NextBinary(ctx)
		if err != nil {
			t.Fatal(err)
		}
		pt, err := tr.Decrypt(b)
		if err != nil {
			t.Fatal(err)
		}
		f, err := frame.Decode(pt, frame.KindComputer, frame.FromHost)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	send := func(f frame.Frame) {
		t.Helper()
		b, _ := frame.Encode(f)
		m, _ := tr.Encrypt(b)
		if err := dc.SendBinary(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if st, _ := next().StateBody(); st.State != "active" {
		t.Fatalf("first cu_state %+v", st)
	}
	expectOp(ctx, t, cc, "computer.started")
	a, _ := frame.NewAction(frame.Action{Seq: 1, Action: actScreenshot, Args: []byte(`{}`)})
	send(a)
	res, _ := next().ResultBody()
	if res.Code != CodeOK || !res.Image {
		t.Fatalf("screenshot %+v", res)
	}
	begin, _ := next().ImageBeginBody()
	var img []byte
	for f := next(); f.Type != frame.TypeCUImageEnd; f = next() {
		img = append(img, f.Data...)
	}
	if !bytes.Equal(img, fh.img) || begin.Bytes != int64(len(img)) {
		t.Errorf("image of %d bytes differs", len(img))
	}
	send(frame.Close())
	if st, _ := next().StateBody(); st.State != "stopped" || st.Reason != "done" {
		t.Errorf("last cu_state %+v", st)
	}
	var e control.ComputerEnded
	for {
		op, d := nextOp(ctx, t, cc)
		if op == "computer.ended" {
			if err := json.Unmarshal(d, &e); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if e.SessionID != sid || e.CloseReason != "done" || e.ActionCounts[actScreenshot] != 1 {
		t.Errorf("computer.ended %+v", e)
	}
	fh.waitOp(OpHide)
}

// expectOp returns the body of the next text frame, which must be op (heartbeats and reports
// between are skipped only for ops other than report).
func expectOp(ctx context.Context, t *testing.T, c *sessiontest.Conn, op string) json.RawMessage {
	t.Helper()
	for {
		got, d := nextOp(ctx, t, c)
		if got == op {
			return d
		}
		if got != "heartbeat" && got != "report" {
			t.Fatalf("frame %s, want %s", got, op)
		}
	}
}

func nextOp(ctx context.Context, t *testing.T, c *sessiontest.Conn) (string, json.RawMessage) {
	t.Helper()
	f, err := c.Next(ctx)
	if err != nil || f.Err != nil {
		t.Fatalf("control connection: %v %v", err, f.Err)
	}
	return f.Op, f.D
}

// sampleStartupReport is a report of one session user with every check passed.
func sampleStartupReport() startup.Report {
	var doc []doctor.ReportEntry
	for _, c := range doctor.Checks {
		doc = append(doc, doctor.ReportEntry{Check: c, OK: true})
	}
	return startup.Report{Profiles: []doctor.Profile{{Name: "shell", DisplayName: "Shell", Kind: "shell"}},
		SessionUsers: []doctor.ReportedSessionUser{{OSUser: "bothy-s-alex", Workdirs: []string{}}}, Doctor: doc, OSVersion: "Debian 13"}
}
