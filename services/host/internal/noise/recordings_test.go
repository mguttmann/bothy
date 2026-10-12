// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// The recordings of TM-T02 (docs/spec/24-terminal.md 14): Go writes recordings/go.json, Dart
// writes recordings/dart.json, and each language replays the other's file in both roles.
// Format and content: brief M2-38, Design note 11.

type recordedMessage struct {
	From    string   `json:"from"`
	Frame   hexField `json:"frame"`
	Message hexField `json:"message"`
}

type recordedScenario struct {
	Name                      string            `json:"name"`
	SessionID                 string            `json:"session_id"`
	HostID                    string            `json:"host_id"`
	ResponderStaticPrivate    hexField          `json:"responder_static_private"`
	ResponderStaticPublic     hexField          `json:"responder_static_public"`
	InitiatorEphemeralPrivate hexField          `json:"initiator_ephemeral_private"`
	InitiatorEphemeralPublic  hexField          `json:"initiator_ephemeral_public"`
	ResponderEphemeralPrivate hexField          `json:"responder_ephemeral_private"`
	ResponderEphemeralPublic  hexField          `json:"responder_ephemeral_public"`
	Message1Payload           hexField          `json:"message1_payload"`
	Message1                  hexField          `json:"message1"`
	Message2Payload           hexField          `json:"message2_payload"`
	Message2                  hexField          `json:"message2"`
	HandshakeHash             hexField          `json:"handshake_hash"`
	Transport                 []recordedMessage `json:"transport"`
}

type recording struct {
	Format    int                `json:"format"`
	Protocol  string             `json:"protocol"`
	Generator string             `json:"generator"`
	Scenarios []recordedScenario `json:"scenarios"`
}

// hexField is a byte field written as lowercase hex.
type hexField []byte

func (h hexField) MarshalJSON() ([]byte, error) { return json.Marshal(hex.EncodeToString(h)) }

func (h *hexField) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := hex.DecodeString(s)
	*h = v
	return err
}

type scriptedFrame struct {
	from frame.Sender
	f    frame.Frame
}

// script returns the frames of a scenario in sending order.
func script(name string) []scriptedFrame {
	dev, host := frame.FromDevice, frame.FromHost
	switch name {
	case "open_shell":
		return []scriptedFrame{
			{dev, frame.Start(120, 40)}, {dev, frame.Data([]byte("ls\n"))},
			{host, frame.Data([]byte("README.md\r\n"))}, {dev, frame.Resize(100, 30)},
			{dev, frame.Data([]byte("exit\n"))}, {host, frame.Exit(0)},
		}
	case "attach_replay":
		return []scriptedFrame{
			{dev, frame.Start(80, 24)}, {host, frame.ReplayBegin()},
			{host, frame.Data([]byte("earlier output\r\n"))}, {host, frame.ReplayEnd()}, {dev, frame.Close()},
		}
	default: // limits
		big := make([]byte, frame.MaxLen-1)
		for i := range big {
			big[i] = byte(i % 251)
		}
		return []scriptedFrame{{dev, frame.Start(65535, 65535)}, {dev, frame.Data(big)}, {host, frame.Exit(-1)}}
	}
}

var scenarioNames = []string{"open_shell", "attach_replay", "limits"}

const message2Payload = `{"v":1,"host_version":"0.0.0","profile":"shell"}`

func recordingKey(t *testing.T, generator, scenario, role string) KeyPair {
	t.Helper()
	return testKey(t, "noise-recording/"+generator+"/"+scenario+"/"+role)
}

// generateRecording runs every scenario of the Go recording and returns the file content.
func generateRecording(t *testing.T) []byte {
	t.Helper()
	rec := recording{Format: 1, Protocol: ProtocolName, Generator: "go"}
	for _, name := range scenarioNames {
		rs := recordingKey(t, "go", name, "responder-static")
		ie := recordingKey(t, "go", name, "initiator-ephemeral")
		re := recordingKey(t, "go", name, "responder-ephemeral")
		sc := recordedScenario{
			Name: name, SessionID: goSessionID, HostID: goHostID,
			ResponderStaticPrivate: rs.Private[:], ResponderStaticPublic: rs.Public[:],
			InitiatorEphemeralPrivate: ie.Private[:], InitiatorEphemeralPublic: ie.Public[:],
			ResponderEphemeralPrivate: re.Private[:], ResponderEphemeralPublic: re.Public[:],
			Message1Payload: []byte("EXAMPLE session token " + name), Message2Payload: []byte(message2Payload),
		}
		sid, hid := mustID(t, goSessionID), mustID(t, goHostID)
		init, err := NewInitiator(InitiatorConfig{HostStatic: rs.Public, SessionID: sid, HostID: hid, Ephemeral: ie})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := NewResponder(ResponderConfig{Static: rs, SessionID: sid, HostID: hid, Ephemeral: &re})
		if err != nil {
			t.Fatal(err)
		}
		if sc.Message1, err = init.WriteMessage1(sc.Message1Payload); err != nil {
			t.Fatal(err)
		}
		if _, _, err = resp.ReadMessage1(sc.Message1); err != nil {
			t.Fatal(err)
		}
		var hostT, devT *Transport
		if sc.Message2, hostT, err = resp.WriteMessage2(sc.Message2Payload); err != nil {
			t.Fatal(err)
		}
		if _, devT, err = init.ReadMessage2(sc.Message2); err != nil {
			t.Fatal(err)
		}
		sc.HandshakeHash = init.HandshakeHash()
		for _, s := range script(name) {
			fb, err := frame.Encode(s.f)
			if err != nil {
				t.Fatal(err)
			}
			sender := devT
			if s.from == frame.FromHost {
				sender = hostT
			}
			msg, err := sender.Encrypt(fb)
			if err != nil {
				t.Fatal(err)
			}
			sc.Transport = append(sc.Transport, recordedMessage{From: s.from.String(), Frame: fb, Message: msg})
		}
		rec.Scenarios = append(rec.Scenarios, sc)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rec); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func recordingPath(generator string) string {
	return filepath.Join(testdataDir, "recordings", generator+".json")
}

func TestRecordingsCurrent(t *testing.T) {
	want := generateRecording(t)
	path := recordingPath("go")
	if os.Getenv("UPDATE_RECORDINGS") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // G301: test data directory of the repository
			t.Fatal(err)
		}
		if err := os.WriteFile(path, want, 0o644); err != nil { //nolint:gosec // G306: test data in the repository, read by everyone
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path) //nolint:gosec // G304: a fixed path of the repository's test data
	if err != nil {
		t.Fatalf("%v; run make -C services/host recordings", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("recordings/go.json is not current (%d bytes, fresh %d bytes, sha256 %s); run make -C services/host recordings",
			len(got), len(want), shortHash(got))
	}
}

func shortHash(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:4])
}

func loadRecording(t *testing.T, generator string) recording {
	t.Helper()
	b, err := os.ReadFile(recordingPath(generator))
	if err != nil {
		t.Fatalf("%v; the %s recording is written by its own test", err, generator)
	}
	var rec recording
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Format != 1 || rec.Protocol != ProtocolName || rec.Generator != generator || len(rec.Scenarios) != len(scenarioNames) {
		t.Fatalf("%s recording: format %d, generator %q, %d scenarios", generator, rec.Format, rec.Generator, len(rec.Scenarios))
	}
	return rec
}

func keyPairOf(t *testing.T, private, public []byte) KeyPair {
	t.Helper()
	if len(private) != KeyLen {
		t.Fatalf("recorded private key of %d bytes", len(private))
	}
	kp, err := KeyPairFromPrivate([KeyLen]byte(private))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(kp.Public[:], public) {
		t.Fatal("recorded public key does not belong to the recorded private key")
	}
	return kp
}

func sameBytes(t *testing.T, what string, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs: %d bytes (sha256 %s), recorded %d bytes (sha256 %s)", what, len(got), shortHash(got), len(want), shortHash(want))
	}
}

// replayAsInitiator plays the device of a recording: message 1, the device frames and the
// handshake hash must equal the recording; message 2 and the host frames must decrypt to it.
func replayAsInitiator(t *testing.T, sc recordedScenario) {
	t.Helper()
	ie := keyPairOf(t, sc.InitiatorEphemeralPrivate, sc.InitiatorEphemeralPublic)
	init, err := NewInitiator(InitiatorConfig{HostStatic: [KeyLen]byte(sc.ResponderStaticPublic),
		SessionID: mustID(t, sc.SessionID), HostID: mustID(t, sc.HostID), Ephemeral: ie})
	if err != nil {
		t.Fatal(err)
	}
	m1, err := init.WriteMessage1(sc.Message1Payload)
	if err != nil {
		t.Fatal(err)
	}
	sameBytes(t, "message 1", m1, sc.Message1)
	p2, devT, err := init.ReadMessage2(sc.Message2)
	if err != nil {
		t.Fatalf("read message 2: %v", err)
	}
	sameBytes(t, "message 2 payload", p2, sc.Message2Payload)
	sameBytes(t, "handshake hash", init.HandshakeHash(), sc.HandshakeHash)
	for i, m := range sc.Transport {
		if m.From == "device" {
			msg, err := devT.Encrypt(m.Frame)
			if err != nil {
				t.Fatal(err)
			}
			sameBytes(t, fmt.Sprintf("transport %d", i), msg, m.Message)
			continue
		}
		pt, err := devT.Decrypt(m.Message)
		if err != nil {
			t.Fatalf("transport %d: %v", i, err)
		}
		sameBytes(t, fmt.Sprintf("frame %d", i), pt, m.Frame)
		if _, err := frame.Decode(pt, frame.FromHost); err != nil {
			t.Errorf("frame %d: %v", i, err)
		}
	}
}

// replayAsResponder plays the host of a recording: message 1 must decrypt to the recorded
// payload and initiator key; message 2 and the host frames must equal the recording.
func replayAsResponder(t *testing.T, sc recordedScenario) {
	t.Helper()
	rs := keyPairOf(t, sc.ResponderStaticPrivate, sc.ResponderStaticPublic)
	re := keyPairOf(t, sc.ResponderEphemeralPrivate, sc.ResponderEphemeralPublic)
	resp, err := NewResponder(ResponderConfig{Static: rs, SessionID: mustID(t, sc.SessionID), HostID: mustID(t, sc.HostID), Ephemeral: &re})
	if err != nil {
		t.Fatal(err)
	}
	p1, e, err := resp.ReadMessage1(sc.Message1)
	if err != nil {
		t.Fatalf("read message 1: %v", err)
	}
	sameBytes(t, "message 1 payload", p1, sc.Message1Payload)
	sameBytes(t, "initiator ephemeral key", e[:], sc.InitiatorEphemeralPublic)
	m2, hostT, err := resp.WriteMessage2(sc.Message2Payload)
	if err != nil {
		t.Fatal(err)
	}
	sameBytes(t, "message 2", m2, sc.Message2)
	sameBytes(t, "handshake hash", resp.HandshakeHash(), sc.HandshakeHash)
	for i, m := range sc.Transport {
		if m.From == "host" {
			msg, err := hostT.Encrypt(m.Frame)
			if err != nil {
				t.Fatal(err)
			}
			sameBytes(t, fmt.Sprintf("transport %d", i), msg, m.Message)
			continue
		}
		pt, err := hostT.Decrypt(m.Message)
		if err != nil {
			t.Fatalf("transport %d: %v", i, err)
		}
		sameBytes(t, fmt.Sprintf("frame %d", i), pt, m.Frame)
		if _, err := frame.Decode(pt, frame.FromDevice); err != nil {
			t.Errorf("frame %d: %v", i, err)
		}
	}
}

func TestRecordingsTMT02Dart(t *testing.T) {
	rec := loadRecording(t, "dart")
	for _, sc := range rec.Scenarios {
		t.Run(sc.Name+"/initiator", func(t *testing.T) { replayAsInitiator(t, sc) })
		t.Run(sc.Name+"/responder", func(t *testing.T) { replayAsResponder(t, sc) })
	}
}

// TestRecordingsTMT02Negative proves the prologue binding on both recordings: a responder with
// a flipped session ID, another static key or swapped session and host IDs fails message 1,
// and an initiator with a flipped session ID or another expected host key fails message 2;
// the correct configuration succeeds in the same test.
func TestRecordingsTMT02Negative(t *testing.T) {
	wrongStatic := testKey(t, "noise-recording/wrong-static")
	for _, generator := range []string{"go", "dart"} {
		rec := loadRecording(t, generator)
		for _, sc := range rec.Scenarios {
			t.Run(generator+"/"+sc.Name, func(t *testing.T) {
				rs := keyPairOf(t, sc.ResponderStaticPrivate, sc.ResponderStaticPublic)
				ie := keyPairOf(t, sc.InitiatorEphemeralPrivate, sc.InitiatorEphemeralPublic)
				sid, hid := mustID(t, sc.SessionID), mustID(t, sc.HostID)
				flipped := sid
				flipped[IDLen-1] ^= 0x01

				responders := map[string]ResponderConfig{
					"wrong prologue":   {Static: rs, SessionID: flipped, HostID: hid},
					"wrong static key": {Static: wrongStatic, SessionID: sid, HostID: hid},
					"swapped host ID":  {Static: rs, SessionID: hid, HostID: sid},
				}
				for name, cfg := range responders {
					resp, err := NewResponder(cfg)
					if err != nil {
						t.Fatal(err)
					}
					if _, _, err := resp.ReadMessage1(sc.Message1); !errors.Is(err, ErrHandshake) {
						t.Errorf("responder with %s: err = %v, want ErrHandshake", name, err)
					}
				}
				initiators := map[string]InitiatorConfig{
					"wrong prologue":   {HostStatic: rs.Public, SessionID: flipped, HostID: hid, Ephemeral: ie},
					"wrong static key": {HostStatic: wrongStatic.Public, SessionID: sid, HostID: hid, Ephemeral: ie},
				}
				for name, cfg := range initiators {
					init, err := NewInitiator(cfg)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := init.WriteMessage1(sc.Message1Payload); err != nil {
						t.Fatal(err)
					}
					if _, _, err := init.ReadMessage2(sc.Message2); !errors.Is(err, ErrHandshake) {
						t.Errorf("initiator with %s: err = %v, want ErrHandshake", name, err)
					}
				}
				// Only the correct configuration succeeds.
				resp, err := NewResponder(ResponderConfig{Static: rs, SessionID: sid, HostID: hid})
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err := resp.ReadMessage1(sc.Message1); err != nil {
					t.Errorf("correct responder: %v", err)
				}
				init, err := NewInitiator(InitiatorConfig{HostStatic: rs.Public, SessionID: sid, HostID: hid, Ephemeral: ie})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := init.WriteMessage1(sc.Message1Payload); err != nil {
					t.Fatal(err)
				}
				if _, _, err := init.ReadMessage2(sc.Message2); err != nil {
					t.Errorf("correct initiator: %v", err)
				}
			})
		}
	}
}

func TestGoRecordingReplays(t *testing.T) {
	rec := loadRecording(t, "go")
	for _, sc := range rec.Scenarios {
		t.Run(sc.Name+"/initiator", func(t *testing.T) { replayAsInitiator(t, sc) })
		t.Run(sc.Name+"/responder", func(t *testing.T) { replayAsResponder(t, sc) })
	}
}
