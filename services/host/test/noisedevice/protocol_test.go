// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// The IDs of the session and the host in the prologue; any canonical UUIDs do.
const (
	testSessionID = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f"
	testHostID    = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e10"
)

var b64 = base64.RawURLEncoding

// sidecar runs serve in a goroutine over pipes, the way the device drives the process over its
// stdin and stdout.
type sidecar struct {
	t      *testing.T
	in     *io.PipeWriter
	out    *bufio.Reader
	stderr bytes.Buffer
	exit   chan int
}

func startSidecar(t *testing.T) *sidecar {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	s := &sidecar{t: t, in: inW, out: bufio.NewReader(outR), exit: make(chan int, 1)}
	go func() {
		code := serve(inR, outW, &s.stderr)
		// A serve that returns early must fail the test, not block its next write.
		_ = inR.Close()
		_ = outW.Close()
		s.exit <- code
	}()
	t.Cleanup(func() { _ = inW.Close() })
	return s
}

// line sends one request line and returns the decoded answer line.
func (s *sidecar) line(req string) map[string]any {
	s.t.Helper()
	if _, err := io.WriteString(s.in, req+"\n"); err != nil {
		s.t.Fatalf("write request: %v", err)
	}
	ans, err := s.out.ReadBytes('\n')
	if err != nil {
		s.t.Fatalf("read answer: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(ans, &m); err != nil {
		s.t.Fatalf("answer is not one JSON object: %v", err)
	}
	return m
}

// call sends a request built from fields and returns the answer.
func (s *sidecar) call(fields map[string]any) map[string]any {
	s.t.Helper()
	b, err := json.Marshal(fields)
	if err != nil {
		s.t.Fatal(err)
	}
	return s.line(string(b))
}

// ok asserts a successful answer and returns its base64url field as bytes, or nil for "".
func (s *sidecar) ok(ans map[string]any, field string) []byte {
	s.t.Helper()
	if ans["ok"] != true {
		s.t.Fatalf("answer not ok: %v", ans["error"])
	}
	if field == "" {
		return nil
	}
	v, isString := ans[field].(string)
	if !isString {
		s.t.Fatalf("answer without string field %q", field)
	}
	b, err := b64.DecodeString(v)
	if err != nil {
		s.t.Fatalf("field %q is not base64url without padding: %v", field, err)
	}
	return b
}

// errKind returns the error kind of a failed answer.
func (s *sidecar) errKind(ans map[string]any) string {
	s.t.Helper()
	if ans["ok"] != false {
		s.t.Fatalf("answer is ok, want an error")
	}
	e, isObject := ans["error"].(map[string]any)
	if !isObject {
		s.t.Fatalf("error answer without error object")
	}
	kind, _ := e["kind"].(string)
	return kind
}

// finish closes stdin and asserts exit 0 and an empty stderr.
func (s *sidecar) finish() {
	s.t.Helper()
	_ = s.in.Close()
	if _, err := io.ReadAll(s.out); err != nil {
		s.t.Fatal(err)
	}
	if code := <-s.exit; code != 0 {
		s.t.Errorf("exit code %d at EOF, want 0", code)
	}
	if s.stderr.Len() != 0 {
		s.t.Errorf("stderr received %d bytes, want none", s.stderr.Len())
	}
}

func enc(b []byte) string { return b64.EncodeToString(b) }

func mustID(t *testing.T, s string) [noise.IDLen]byte {
	t.Helper()
	id, err := noise.ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// recordedFrame is one transport frame of M2-38's Go recording (hex fields).
type recordedFrame struct {
	From  string `json:"from"`
	Frame string `json:"frame"`
}

type recordedScenario struct {
	Name                   string          `json:"name"`
	SessionID              string          `json:"session_id"`
	HostID                 string          `json:"host_id"`
	ResponderStaticPrivate string          `json:"responder_static_private"`
	Message1Payload        string          `json:"message1_payload"`
	Message2Payload        string          `json:"message2_payload"`
	Transport              []recordedFrame `json:"transport"`
}

// loadGoRecording reads testdata/noise/recordings/go.json of M2-38 (TM-T02).
func loadGoRecording(t *testing.T) []recordedScenario {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "noise", "recordings", "go.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		Generator string             `json:"generator"`
		Scenarios []recordedScenario `json:"scenarios"`
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Generator != "go" || len(rec.Scenarios) == 0 {
		t.Fatalf("go recording: generator %q, %d scenarios", rec.Generator, len(rec.Scenarios))
	}
	return rec.Scenarios
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// handshake runs start, message 1 and message 2 of the sidecar against a responder with the
// given static key and returns the responder's transport.
func handshake(t *testing.T, s *sidecar, static noise.KeyPair, sessionID, hostID string, p1, p2 []byte) *noise.Transport {
	t.Helper()
	start := s.call(map[string]any{"op": "start", "host_static": enc(static.Public[:]), "session_id": sessionID, "host_id": hostID})
	ephPublic := s.ok(start, "ephemeral_public")
	resp, err := noise.NewResponder(noise.ResponderConfig{Static: static, SessionID: mustID(t, sessionID), HostID: mustID(t, hostID)})
	if err != nil {
		t.Fatal(err)
	}
	m1 := s.ok(s.call(map[string]any{"op": "message1", "payload": enc(p1)}), "message")
	got1, initEph, err := resp.ReadMessage1(m1)
	if err != nil {
		t.Fatalf("responder reads message 1: %v", err)
	}
	if !bytes.Equal(got1, p1) {
		t.Errorf("message 1 payload: %d bytes, want %d", len(got1), len(p1))
	}
	if !bytes.Equal(initEph[:], ephPublic) {
		t.Error("ephemeral key of message 1 differs from ephemeral_public of start")
	}
	m2, hostT, err := resp.WriteMessage2(p2)
	if err != nil {
		t.Fatal(err)
	}
	got2 := s.ok(s.call(map[string]any{"op": "message2", "message": enc(m2)}), "payload")
	if !bytes.Equal(got2, p2) {
		t.Errorf("message 2 payload: %d bytes, want %d", len(got2), len(p2))
	}
	return hostT
}

func TestProtocolHandshakeAgainstResponder(t *testing.T) {
	for _, sc := range loadGoRecording(t) {
		t.Run(sc.Name, func(t *testing.T) {
			static, err := noise.KeyPairFromPrivate([noise.KeyLen]byte(unhex(t, sc.ResponderStaticPrivate)))
			if err != nil {
				t.Fatal(err)
			}
			s := startSidecar(t)
			hostT := handshake(t, s, static, sc.SessionID, sc.HostID, unhex(t, sc.Message1Payload), unhex(t, sc.Message2Payload))
			if len(sc.Transport) == 0 {
				t.Fatal("scenario without transport frames")
			}
			for i, m := range sc.Transport {
				fb := unhex(t, m.Frame)
				switch m.From {
				case "device":
					msg := s.ok(s.call(map[string]any{"op": "seal", "frame": enc(fb)}), "message")
					plain, err := hostT.Decrypt(msg)
					if err != nil {
						t.Fatalf("frame %d: responder decrypts the sealed device frame: %v", i, err)
					}
					if !bytes.Equal(plain, fb) {
						t.Errorf("frame %d: device frame differs after seal", i)
					}
				case "host":
					msg, err := hostT.Encrypt(fb)
					if err != nil {
						t.Fatal(err)
					}
					got := s.ok(s.call(map[string]any{"op": "open", "message": enc(msg)}), "frame")
					if !bytes.Equal(got, fb) {
						t.Errorf("frame %d: host frame differs after open", i)
					}
				default:
					t.Fatalf("frame %d from %q", i, m.From)
				}
			}
			s.finish()
		})
	}
}

func TestCekEqualsEphemeralKeyHash(t *testing.T) {
	static, err := noise.GenerateKeyPair(nil)
	if err != nil {
		t.Fatal(err)
	}
	s := startSidecar(t)
	ans := s.call(map[string]any{"op": "start", "host_static": enc(static.Public[:]), "session_id": testSessionID, "host_id": testHostID})
	eph := s.ok(ans, "ephemeral_public")
	want, err := noise.EphemeralKeyHash(eph)
	if err != nil {
		t.Fatal(err)
	}
	if cek, _ := ans["cek"].(string); cek != want {
		t.Errorf("cek %q, want EphemeralKeyHash of ephemeral_public %q", cek, want)
	}
	s.finish()
	// A second process draws a fresh ephemeral key.
	s2 := startSidecar(t)
	eph2 := s2.ok(s2.call(map[string]any{"op": "start", "host_static": enc(static.Public[:]), "session_id": testSessionID, "host_id": testHostID}), "ephemeral_public")
	if bytes.Equal(eph, eph2) {
		t.Error("two sessions got the same ephemeral key")
	}
	s2.finish()
}

func TestProtocolErrors(t *testing.T) {
	static, err := noise.GenerateKeyPair(nil)
	if err != nil {
		t.Fatal(err)
	}
	start := map[string]any{"op": "start", "host_static": enc(static.Public[:]), "session_id": testSessionID, "host_id": testHostID}
	dataFrame, err := frame.Encode(frame.Data([]byte("ls\n")))
	if err != nil {
		t.Fatal(err)
	}
	exitFrame, err := frame.Encode(frame.Exit(0))
	if err != nil {
		t.Fatal(err)
	}

	s := startSidecar(t)
	steps := []struct {
		name string
		req  string
		kind string
	}{
		{"garbled json", `{"op":"start",`, "bad_json"},
		{"not an object", `[1,2]`, "bad_json"},
		{"empty line", ``, "bad_json"},
		{"unknown op", `{"op":"rekey"}`, "unknown_op"},
		{"missing op", `{}`, "unknown_op"},
		{"message1 before start", `{"op":"message1","payload":""}`, "out_of_order"},
		{"message2 before start", `{"op":"message2","message":"AAAA"}`, "out_of_order"},
		{"seal before start", `{"op":"seal","frame":"` + enc(dataFrame) + `"}`, "out_of_order"},
		{"open before start", `{"op":"open","message":"AAAA"}`, "out_of_order"},
		{"start without host_static", `{"op":"start","session_id":"` + testSessionID + `","host_id":"` + testHostID + `"}`, "bad_field"},
		{"start with padded base64", `{"op":"start","host_static":"` + base64.URLEncoding.EncodeToString(static.Public[:]) + `","session_id":"` + testSessionID + `","host_id":"` + testHostID + `"}`, "bad_field"},
		{"start with a short key", `{"op":"start","host_static":"` + enc(static.Public[:31]) + `","session_id":"` + testSessionID + `","host_id":"` + testHostID + `"}`, "ErrInvalidKey"},
		{"start with a bad session id", `{"op":"start","host_static":"` + enc(static.Public[:]) + `","session_id":"{` + testSessionID[1:] + `","host_id":"` + testHostID + `"}`, "ErrInvalidID"},
		{"start without host_id", `{"op":"start","host_static":"` + enc(static.Public[:]) + `","session_id":"` + testSessionID + `"}`, "bad_field"},
	}
	for _, st := range steps {
		if got := s.errKind(s.line(st.req)); got != st.kind {
			t.Errorf("%s: kind %q, want %q", st.name, got, st.kind)
		}
	}

	// The process kept serving: a valid start now succeeds; later order errors follow.
	s.ok(s.call(start), "ephemeral_public")
	later := []struct {
		name string
		req  map[string]any
		kind string
	}{
		{"second start", start, "out_of_order"},
		{"message2 before message1", map[string]any{"op": "message2", "message": "AAAA"}, "ErrState"},
		{"seal before message2", map[string]any{"op": "seal", "frame": enc(dataFrame)}, "out_of_order"},
		{"open before message2", map[string]any{"op": "open", "message": "AAAA"}, "out_of_order"},
		{"message1 with bad base64", map[string]any{"op": "message1", "payload": "not base64!"}, "bad_field"},
		{"message1 without payload", map[string]any{"op": "message1"}, "bad_field"},
	}
	for _, st := range later {
		if got := s.errKind(s.call(st.req)); got != st.kind {
			t.Errorf("%s: kind %q, want %q", st.name, got, st.kind)
		}
	}
	m1 := s.ok(s.call(map[string]any{"op": "message1", "payload": enc([]byte("EXAMPLE token"))}), "message")
	if got := s.errKind(s.call(map[string]any{"op": "message1", "payload": ""})); got != "ErrState" {
		t.Errorf("second message1: kind %q, want ErrState", got)
	}
	resp, err := noise.NewResponder(noise.ResponderConfig{Static: static, SessionID: mustID(t, testSessionID), HostID: mustID(t, testHostID)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := resp.ReadMessage1(m1); err != nil {
		t.Fatal(err)
	}
	m2, _, err := resp.WriteMessage2(nil)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Clone(m2)
	tampered[len(tampered)-1] ^= 1
	if got := s.errKind(s.call(map[string]any{"op": "message2", "message": enc(tampered)})); got != "ErrHandshake" {
		t.Errorf("tampered message 2: kind %q, want ErrHandshake", got)
	}
	if got := s.errKind(s.call(map[string]any{"op": "seal", "frame": enc(dataFrame)})); got != "out_of_order" {
		t.Errorf("seal after a failed message 2: kind %q, want out_of_order", got)
	}
	s.finish()

	// A completed handshake: frame errors and decryption errors.
	s = startSidecar(t)
	hostT := handshake(t, s, static, testSessionID, testHostID, []byte("EXAMPLE token"), []byte(`{"v":1}`))
	frameErrors := []struct {
		name string
		req  map[string]any
		kind string
	}{
		{"seal of a host frame", map[string]any{"op": "seal", "frame": enc(exitFrame)}, "ErrWrongDirection"},
		{"seal of an unknown type", map[string]any{"op": "seal", "frame": enc([]byte{0x7f})}, "ErrUnknownType"},
		{"seal of a malformed start", map[string]any{"op": "seal", "frame": enc([]byte{0x01, 0, 1})}, "ErrMalformed"},
		{"seal of an empty frame", map[string]any{"op": "seal", "frame": ""}, "ErrEmpty"},
		{"seal of a frame over the limit", map[string]any{"op": "seal", "frame": enc(append([]byte{0x02}, make([]byte, frame.MaxLen)...))}, "ErrTooLong"},
		{"seal without frame", map[string]any{"op": "seal"}, "bad_field"},
		{"open with bad base64", map[string]any{"op": "open", "message": "@@"}, "bad_field"},
	}
	for _, st := range frameErrors {
		if got := s.errKind(s.call(st.req)); got != st.kind {
			t.Errorf("%s: kind %q, want %q", st.name, got, st.kind)
		}
	}
	// Host messages that decrypt but carry a frame the host may not send: a data frame without
	// bytes and a frame type of the device.
	msg, err := hostT.Encrypt(dataFrame[:1:1])
	if err != nil {
		t.Fatal(err)
	}
	if got := s.errKind(s.call(map[string]any{"op": "open", "message": enc(msg)})); got != "ErrMalformed" {
		t.Errorf("open of an empty data frame: kind %q, want ErrMalformed", got)
	}
	startFrame, err := frame.Encode(frame.Start(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	if msg, err = hostT.Encrypt(startFrame); err != nil {
		t.Fatal(err)
	}
	if got := s.errKind(s.call(map[string]any{"op": "open", "message": enc(msg)})); got != "ErrWrongDirection" {
		t.Errorf("open of a device frame: kind %q, want ErrWrongDirection", got)
	}
	if got := s.errKind(s.call(map[string]any{"op": "open", "message": enc([]byte("short"))})); got != "ErrDecrypt" {
		t.Errorf("open of a short message: kind %q, want ErrDecrypt", got)
	}
	if got := s.errKind(s.call(map[string]any{"op": "open", "message": enc(make([]byte, 32))})); got != "ErrBroken" {
		t.Errorf("open after a failed decryption: kind %q, want ErrBroken", got)
	}
	s.finish()
}

// TestKeypairBeforeStart proves the op keypair: the device needs client_ephemeral_key_hash for
// openTerminalSession before it knows the session ID of the prologue, so keypair draws the
// ephemeral key first and start keeps it; a second keypair, or keypair after start, is out of
// order.
func TestKeypairBeforeStart(t *testing.T) {
	static, err := noise.GenerateKeyPair(nil)
	if err != nil {
		t.Fatal(err)
	}
	s := startSidecar(t)
	kp := s.call(map[string]any{"op": "keypair"})
	eph := s.ok(kp, "ephemeral_public")
	want, err := noise.EphemeralKeyHash(eph)
	if err != nil {
		t.Fatal(err)
	}
	if cek, _ := kp["cek"].(string); cek != want {
		t.Errorf("keypair cek %q, want %q", cek, want)
	}
	if k := s.errKind(s.call(map[string]any{"op": "keypair"})); k != "out_of_order" {
		t.Errorf("a second keypair gives %q, want out_of_order", k)
	}
	st := s.call(map[string]any{"op": "start", "host_static": enc(static.Public[:]), "session_id": testSessionID, "host_id": testHostID})
	if got := s.ok(st, "ephemeral_public"); !bytes.Equal(got, eph) {
		t.Error("start after keypair drew another ephemeral key")
	}
	if cek, _ := st["cek"].(string); cek != want {
		t.Errorf("start cek %q, want the cek of keypair %q", cek, want)
	}
	resp, err := noise.NewResponder(noise.ResponderConfig{Static: static, SessionID: mustID(t, testSessionID), HostID: mustID(t, testHostID)})
	if err != nil {
		t.Fatal(err)
	}
	m1 := s.ok(s.call(map[string]any{"op": "message1", "payload": enc([]byte("token"))}), "message")
	if _, initEph, err := resp.ReadMessage1(m1); err != nil || !bytes.Equal(initEph[:], eph) {
		t.Errorf("the responder reads message 1 with %v and another ephemeral key: %v", err, !bytes.Equal(initEph[:], eph))
	}
	s.finish()
	s2 := startSidecar(t)
	s2.call(map[string]any{"op": "start", "host_static": enc(static.Public[:]), "session_id": testSessionID, "host_id": testHostID})
	if k := s2.errKind(s2.call(map[string]any{"op": "keypair"})); k != "out_of_order" {
		t.Errorf("keypair after start gives %q, want out_of_order", k)
	}
	s2.finish()
}
