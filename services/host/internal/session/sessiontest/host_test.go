// SPDX-License-Identifier: Apache-2.0

package sessiontest_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/pairing"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
	"github.com/OWNER/bothy/services/host/internal/state"
)

const pairCode = "EXAMPLE-CODE"

// pairWith pairs a state directory against h like bothy-host pair and returns it.
func pairWith(t *testing.T, h *sessiontest.HostServer, code string) (*state.Dir, pairing.Result, error) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	st := state.Open(dir, state.Plain{})
	res, err := pairing.Pair(context.Background(), pairing.Options{ServerURL: h.URL, Code: code, CAPEM: h.CAPEM(), State: st,
		Platform: "linux", HostVersion: "1.2.3"})
	return st, res, err
}

func TestHostServerPair(t *testing.T) {
	h := sessiontest.NewHostServer(t, sessiontest.HostOptions{Code: pairCode, Declaration: "organization"})
	if _, _, err := pairWith(t, h, "EXAMPLE-WRONG"); err == nil || !strings.Contains(err.Error(), "pairing_code_invalid") {
		t.Fatalf("a wrong code paired: %v", err)
	}
	if _, ok := h.NoisePublic(); ok {
		t.Fatal("a refused pairing kept a key")
	}
	st, res, err := pairWith(t, h, pairCode)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if res.HostID != h.HostID || res.InstanceID != h.InstanceID {
		t.Fatalf("answer %+v, want host %s", res, h.HostID)
	}
	keys, err := st.ReadKeys()
	if err != nil {
		t.Fatal(err)
	}
	kp, err := noise.KeyPairFromPrivate(keys.NoisePrivate)
	if err != nil {
		t.Fatal(err)
	}
	pub, ok := h.NoisePublic()
	if !ok || pub != kp.Public {
		t.Fatal("the fake server kept another Noise key than the host stored")
	}
	if len(h.Received()) == 0 {
		t.Fatal("the pairing body was not recorded")
	}
}

// hostSide is the host end of a control connection in these tests.
type hostSide struct {
	t  *testing.T
	ws *websocket.Conn
}

func (s hostSide) next(ctx context.Context) (string, json.RawMessage) {
	s.t.Helper()
	typ, b, err := s.ws.Read(ctx)
	if err != nil || typ != websocket.MessageText {
		s.t.Fatalf("read: %v", err)
	}
	var env struct {
		Op string          `json:"op"`
		D  json.RawMessage `json:"d"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		s.t.Fatal(err)
	}
	return env.Op, env.D
}

func (s hostSide) send(ctx context.Context, op string, d any) {
	s.t.Helper()
	b, _ := json.Marshal(map[string]any{"op": op, "d": d})
	if err := s.ws.Write(ctx, websocket.MessageText, b); err != nil {
		s.t.Fatal(err)
	}
}

func dial(t *testing.T, ctx context.Context, h *sessiontest.HostServer, path string) *websocket.Conn {
	t.Helper()
	hc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: h.Pool, MinVersion: tls.VersionTLS12}}}
	ws, resp, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(h.URL, "https")+path, &websocket.DialOptions{HTTPClient: hc})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	return ws
}

// login runs hello, identify and ready as the host and returns the side and the hello.
func login(t *testing.T, ctx context.Context, h *sessiontest.HostServer, st *state.Dir, live []string) (hostSide, control.Hello) {
	t.Helper()
	keys, err := st.ReadKeys()
	if err != nil {
		t.Fatal(err)
	}
	s := hostSide{t: t, ws: dial(t, ctx, h, sessiontest.ControlPath)}
	op, d := s.next(ctx)
	var hello control.Hello
	if op != "hello" || json.Unmarshal(d, &hello) != nil {
		t.Fatalf("first frame %s", op)
	}
	s.send(ctx, "identify", control.Identify{ProtocolVersion: 1, HostID: h.HostID, HostVersion: "1.2.3", LiveSessions: live,
		Proof: control.Proof(keys.AuthPrivate, h.HostID, hello.Challenge, time.Now())})
	return s, hello
}

func TestHostServerControl(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	h := sessiontest.NewHostServer(t, sessiontest.HostOptions{Code: pairCode, Declaration: "organization"})
	st, _, err := pairWith(t, h, pairCode)
	if err != nil {
		t.Fatal(err)
	}
	s, hello := login(t, ctx, h, st, []string{})
	if len(hello.JWKS.Keys) != 1 || hello.JWKS.Keys[0]["kid"] != h.Signer.Kid || hello.IdleTimeoutMs != sessiontest.DefaultValues.IdleTimeoutMs {
		t.Fatalf("hello %+v", hello)
	}
	op, d := s.next(ctx)
	var ready control.ReadyFrame
	if op != "ready" || json.Unmarshal(d, &ready) != nil || ready.HostID != h.HostID || ready.LLMCredentials != "organization" || ready.State != "active" {
		t.Fatalf("ready %s %s", op, d)
	}
	if id := h.Identifies(); len(id) != 1 || !id[0].ProofOK {
		t.Fatalf("identify not recorded with a valid proof: %+v", id)
	}
	mark := h.ReportCount()
	if err := h.Open(ctx, "0192f4a0-0000-7000-8000-0000000000e0", "shell", time.Minute); err == nil {
		t.Fatal("session.open before the report of the host")
	}
	s.send(ctx, "report", map[string]any{"profiles": []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}},
		"session_users":  []any{map[string]any{"os_user": "bothy-s-alex", "workdirs": []string{"A"}}},
		"doctor":         []any{map[string]any{"check": "HD6", "ok": false, "code": "subscription_login_detected", "session_user": "bothy-s-sam"}},
		"is_server_host": true, "require_local_confirm": false})
	r, err := h.WaitReport(ctx, mark, func(r sessiontest.Report) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasProfile("shell") || r.HasProfile("claude") || !r.Failed("HD6", "subscription_login_detected", "bothy-s-sam") ||
		len(r.Workdirs("bothy-s-alex")) != 1 || !r.IsServerHost {
		t.Fatalf("report %+v", r)
	}
	s.send(ctx, "heartbeat", map[string]any{"active_sessions": 0})
	if op, _ := s.next(ctx); op != "heartbeat_ack" {
		t.Fatalf("heartbeat answered with %s", op)
	}
	id := "0192f4a0-0000-7000-8000-0000000000e1"
	if err := h.Open(ctx, id, "shell", time.Minute); err != nil {
		t.Fatal(err)
	}
	op, d = s.next(ctx)
	var open control.SessionOpen
	if op != "session.open" || json.Unmarshal(d, &open) != nil || open.SessionID != id || open.DataPath != sessiontest.DataPrefix+id+"/stream" || open.Profile != "shell" {
		t.Fatalf("session.open %s %s", op, d)
	}
	if _, err := time.Parse(time.RFC3339Nano, open.ExpiresAt); err != nil {
		t.Fatalf("expires_at %q", open.ExpiresAt)
	}
	data := dial(t, ctx, h, open.DataPath)
	if typ, _, err := data.Read(ctx); err != nil || typ != websocket.MessageText {
		t.Fatalf("data hello: %v", err)
	}
	b, _ := json.Marshal(map[string]any{"op": "identify", "d": map[string]any{"session_id": id}})
	if err := data.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
	if c := h.Data(ctx, id, 1); c == nil || c.Identify["session_id"] != id {
		t.Fatal("data connection not handed to the test")
	}
	s.send(ctx, "session.started", map[string]any{"session_id": id})
	s.send(ctx, "session.ended", map[string]any{"session_id": id, "close_reason": "open_failed", "close_detail": "shared_location"})
	e, err := h.WaitEnded(ctx, id)
	if err != nil || e.CloseReason != "open_failed" || e.CloseDetail != "shared_location" || h.StartedCount(id) != 1 {
		t.Fatalf("ended %+v %v, started %d", e, err, h.StartedCount(id))
	}
	if err := h.CloseSession(ctx, id, "access_revoked"); err != nil {
		t.Fatal(err)
	}
	if op, d := s.next(ctx); op != "session.close" || !strings.Contains(string(d), "access_revoked") {
		t.Fatalf("session.close %s %s", op, d)
	}
	if err := h.SetDeclaration(ctx, "none"); err != nil {
		t.Fatal(err)
	}
	if op, d := s.next(ctx); op != "ready" || !strings.Contains(string(d), `"llm_credentials":"none"`) {
		t.Fatalf("further ready %s %s", op, d)
	}
	if err := h.Open(ctx, id, "shell", time.Minute); !errors.Is(err, sessiontest.ErrNotReported) {
		t.Fatalf("session.open after a ready without a report: %v", err)
	}
}

func TestHostServerReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	h := sessiontest.NewHostServer(t, sessiontest.HostOptions{Code: pairCode, Declaration: "personal"})
	st, _, err := pairWith(t, h, pairCode)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := login(t, ctx, h, st, []string{})
	first.next(ctx) // ready
	_ = first.ws.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(10 * time.Second)
	for h.Connected() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if h.Connected() {
		t.Fatal("a closed control connection still counts as connected")
	}
	second, _ := login(t, ctx, h, st, []string{"0192f4a0-0000-7000-8000-0000000000e2"})
	if op, d := second.next(ctx); op != "ready" || !strings.Contains(string(d), "personal") {
		t.Fatalf("ready on the second connection %s %s", op, d)
	}
	ids := h.Identifies()
	if len(ids) != 2 || len(ids[1].LiveSessions) != 1 || h.Logins() != 2 {
		t.Fatalf("identifies %+v logins %d", ids, h.Logins())
	}
}
