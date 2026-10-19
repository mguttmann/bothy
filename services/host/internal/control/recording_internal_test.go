// SPDX-License-Identifier: Apache-2.0

package control

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// The recordings of the control connection (M2-43 Design note 13): the host service writes the
// frames it sends (host_frames.json), the server those it sends (server_frames.json, written by
// services/server). UPDATE_RECORDINGS=1 writes, otherwise the comparison is byte for byte.
var (
	hostFramesFile   = filepath.Join("..", "..", "testdata", "control", "host_frames.json")
	serverFramesFile = filepath.Join("..", "..", "testdata", "control", "server_frames.json")
)

// The exchange of the recording: the challenge and server time of the recorded hello.
const (
	recordingHost      = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f"
	recordingChallenge = "SGVsbG8gZnJvbSB0aGUgcmVjb3JkaW5nIG9mIE0yLTQzIQ"
	recordingSession   = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e10"
)

var recordingTime = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// recordingKey is the fixed key of the recording: the seed is SHA-256 of
// control-recording/host-auth.
func recordingKey() ed25519.PrivateKey {
	s := sha256.Sum256([]byte("control-recording/host-auth"))
	return ed25519.NewKeyFromSeed(s[:])
}

// mustFrame encodes one frame.
func mustFrame(t *testing.T, op string, d any) json.RawMessage {
	t.Helper()
	b, err := encodeFrame(op, d)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestHostFramesRecordingCurrent: one frame per host-to-server op of hostControl, made by the
// code of this package with the fixed key and clock, equals the recording.
func TestHostFramesRecordingCurrent(t *testing.T) {
	t.Parallel()
	k := recordingKey()
	code := 0
	rep := withPeer(startup.Report{Profiles: []doctor.Profile{{Name: "shell", DisplayName: "Shell", Kind: "shell"}},
		SessionUsers: []doctor.ReportedSessionUser{{OSUser: "bothy-s-alex", Workdirs: []string{"Project A"}}},
		Doctor:       []doctor.ReportEntry{{Check: "hd1_user", OK: true}, {Check: "hd6_llm_login", OK: false, Code: "subscription_login_detected", SessionUser: "bothy-s-alex"}},
		OSVersion:    "Debian 13"}, false)
	rec := struct {
		PublicJWK  map[string]string `json:"public_jwk"`
		HostID     string            `json:"host_id"`
		Challenge  string            `json:"challenge"`
		ServerTime time.Time         `json:"server_time"`
		Frames     []json.RawMessage `json:"frames"`
	}{
		PublicJWK: PublicJWK(k), HostID: recordingHost, Challenge: recordingChallenge, ServerTime: recordingTime,
		Frames: []json.RawMessage{
			mustFrame(t, opIdentify, Identify{ProtocolVersion: 1, HostID: recordingHost, Proof: Proof(k, recordingHost, recordingChallenge, recordingTime),
				HostVersion: "1.0.0", LiveSessions: []string{recordingSession}}),
			mustFrame(t, opHeartbeat, Heartbeat{ActiveSessions: 1}),
			mustFrame(t, opReport, rep),
			mustFrame(t, opSessionStarted, SessionStarted{SessionID: recordingSession}),
			mustFrame(t, opSessionEnded, SessionEnded{SessionID: recordingSession, CloseReason: "process_exited", ExitCode: &code}),
		},
	}
	want, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if os.Getenv("UPDATE_RECORDINGS") == "1" {
		if err := os.WriteFile(hostFramesFile, want, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(hostFramesFile) //nolint:gosec // G304: a fixed path of the repository's test data
	if err != nil {
		t.Fatalf("%v (run with UPDATE_RECORDINGS=1)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run with UPDATE_RECORDINGS=1", hostFramesFile)
	}
}

// TestServerFramesDecodeStrictly: every frame the server recorded decodes strictly into the
// hand-written types.
func TestServerFramesDecodeStrictly(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(serverFramesFile) //nolint:gosec // G304: a fixed path of the repository's test data
	if err != nil {
		t.Fatal(err)
	}
	var frames []json.RawMessage
	if err := json.Unmarshal(b, &frames); err != nil {
		t.Fatal(err)
	}
	ops := map[string]bool{}
	for _, f := range frames {
		op, _, err := decodeServerFrame(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
		}
		ops[op] = true
	}
	for op := range required {
		if !ops[op] {
			t.Errorf("the recording has no %s", op)
		}
	}
}

// FuzzDecodeServerFrame (M2-43, PLAN section 5 "Security"): arbitrary frames give a frame or
// ErrFrame, never a panic.
func FuzzDecodeServerFrame(f *testing.F) {
	if b, err := os.ReadFile(serverFramesFile); err == nil { //nolint:gosec // G304: a fixed path of the repository's test data
		var frames []json.RawMessage
		if json.Unmarshal(b, &frames) == nil {
			for _, fr := range frames {
				f.Add([]byte(fr))
			}
		}
	}
	f.Add([]byte(`{"op":"hello","d":null}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		op, body, err := decodeServerFrame(b)
		if err != nil {
			return
		}
		if _, ok := required[op]; !ok || body == nil {
			t.Fatalf("decoded %q without a body", op)
		}
	})
}
