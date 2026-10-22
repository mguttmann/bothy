// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/specschema"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The recordings of the control connection in both directions (M2-43 Design note 13): the server
// writes the frames it sends, the host service the frames it sends; each side holds the other's
// hand-written or generated code to the contract. `go test -update` writes the server recording
// (the host module writes its own with UPDATE_RECORDINGS=1), otherwise the recording is compared
// byte for byte.
var update = flag.Bool("update", false, "rewrite services/host/testdata/control/server_frames.json")

var (
	recordingDir  = filepath.Join("..", "..", "..", "..", "..", "host", "testdata", "control")
	serverFrames  = filepath.Join(recordingDir, "server_frames.json")
	hostFrames    = filepath.Join(recordingDir, "host_frames.json")
	recordingTime = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	recordingHost = ids.MustParse("0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f")
)

// recordedHello are the values of the recorded hello frame; no connection runs with them.
var recordedHello = Config{
	HeartbeatInterval:  20 * time.Second,
	HeartbeatMissLimit: 3, // nosemgrep: bothy-go-test-gateway-window -- values of a recorded frame, no window is in effect
	ClockLeeway:        30 * time.Second,
	IdleTimeout:        30 * time.Minute,
	DetachGrace:        15 * time.Minute,
}

// recordedServerFrames are one frame per server-to-host op of hostControl, heartbeat_ack with and
// without jwks, and error.
func recordedServerFrames() [][]byte {
	keys := []app.JWK{{Kty: "OKP", Crv: "Ed25519", X: "11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo", Kid: "terminal-key-1", Alg: "EdDSA", Use: "sig"}}
	session := ids.MustParse("0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e10")
	return [][]byte{
		helloFrame("SGVsbG8gZnJvbSB0aGUgcmVjb3JkaW5nIG9mIE0yLTQzIQ", recordingTime, recordedHello, keys),
		readyFrame(app.Ready{HostID: recordingHost, State: "active", LLMCredentials: "organization"}),
		heartbeatAckFrame(recordingTime, nil),
		heartbeatAckFrame(recordingTime, keys),
		sessionOpenFrame(app.SessionOpen{SessionID: session, DataPath: "/api/v1/hosts/sessions/" + session.String() + "/stream", Profile: "shell",
			ExpiresAt: recordingTime.Add(20 * time.Second)}),
		sessionCloseFrame(domain.SessionClose{SessionID: session, Reason: domain.CloseAccessRevoked}),
		errorFrame(opReport, codeInvalid),
	}
}

// joinFrames writes one frame per line inside a JSON array.
func joinFrames(frames [][]byte) []byte {
	var b bytes.Buffer
	b.WriteString("[\n")
	for i, f := range frames {
		b.WriteString("  ")
		b.Write(f)
		if i < len(frames)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n")
	return b.Bytes()
}

// TestServerFramesRecordingCurrent: the recording of the server frames equals what the codec
// encodes, and every frame passes the contract (G9).
func TestServerFramesRecordingCurrent(t *testing.T) {
	t.Parallel()
	c, err := specschema.Default()
	if err != nil {
		t.Fatal(err)
	}
	frames := recordedServerFrames()
	for _, f := range frames {
		if err := c.ValidateChannelFrame("hostControl", true, f); err != nil {
			t.Errorf("server frame %s: %v", f, err)
		}
	}
	want := joinFrames(frames)
	if *update {
		if err := os.WriteFile(serverFrames, want, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(serverFrames) //nolint:gosec // G304: a fixed path of the repository's test data
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run with -update", serverFrames)
	}
}

// hostRecording is the file the host service writes (services/host, TestHostFramesRecordingCurrent).
type hostRecording struct {
	PublicJWK  json.RawMessage   `json:"public_jwk"`
	HostID     string            `json:"host_id"`
	Challenge  string            `json:"challenge"`
	ServerTime time.Time         `json:"server_time"`
	Frames     []json.RawMessage `json:"frames"`
}

// TestHostClientFramesValid: every frame of the host recording passes the contract (G9) and is
// decoded strictly by the server codec; the recorded proof verifies with the server's
// CheckHostProof.
func TestHostClientFramesValid(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(hostFrames) //nolint:gosec // G304: a fixed path of the repository's test data
	if err != nil {
		t.Fatal(err)
	}
	var rec hostRecording
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	c, err := specschema.Default()
	if err != nil {
		t.Fatal(err)
	}
	check, err := newSchemas()
	if err != nil {
		t.Fatal(err)
	}
	ops := map[string]bool{}
	var identify app.Identify
	for _, f := range rec.Frames {
		if err := c.ValidateChannelFrame("hostControl", false, f); err != nil {
			t.Errorf("host frame %s: %v", f, err)
		}
		fr, err := decodeFrame(f, 32, check)
		if err != nil {
			t.Errorf("host frame %s: %v", f, err)
			continue
		}
		ops[fr.op] = true
		if fr.op == opIdentify {
			identify = identifyOf(fr.d.(*restIdentify))
		}
	}
	for op := range hostOps {
		if !ops[op] {
			t.Errorf("the recording has no %s", op)
		}
	}
	k, err := jose.ParseJWK(rec.PublicJWK)
	if err != nil {
		t.Fatal(err)
	}
	v := domain.CheckHostProof(domain.ProofCheck{Proof: identify.Proof, HostID: ids.MustParse(rec.HostID), Challenge: rec.Challenge,
		HelloAt: rec.ServerTime, Now: rec.ServerTime, Leeway: time.Minute}, domain.HostLoginKey{Found: true, State: domain.StateActive,
		JKT: k.Thumbprint(), JWK: k.Canonical()})
	if v != domain.ProofValid {
		t.Fatalf("recorded proof: verdict %v", v)
	}
}
