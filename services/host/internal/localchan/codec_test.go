// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

func raw(kind byte, body []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(len(body)+1)) //nolint:gosec // small test values
	return append(append(b, kind), body...)
}

func TestCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	in := Message{Op: OpCheckRun, ID: "1", D: json.RawMessage(`{"mode":"full"}`)}
	if err := WriteMessage(&buf, in); err != nil {
		t.Fatal(err)
	}
	out, err := DecodeMessage(buf.Bytes())
	if err != nil || out.Op != in.Op || out.ID != in.ID || string(out.D) != string(in.D) {
		t.Fatalf("%+v %v", out, err)
	}
	b, err := Encode(Message{Op: OpHello})
	if err != nil || !strings.Contains(string(b), `"d":{}`) {
		t.Errorf("empty body %q %v", b, err)
	}
}

func TestCodecLimits(t *testing.T) {
	big := Message{Op: "x", D: json.RawMessage(`"` + strings.Repeat("a", frame.MaxLen) + `"`)}
	if _, err := Encode(big); !errors.Is(err, ErrTooLarge) {
		t.Errorf("oversized encode: %v", err)
	}
	over := binary.BigEndian.AppendUint32(nil, MaxLen+1)
	if _, err := DecodeMessage(append(over, 0)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("oversized decode: %v", err)
	}
	if _, err := DecodeMessage([]byte{0, 0, 0, 0}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("zero length: %v", err)
	}
	if _, err := DecodeMessage(raw(0x02, []byte("{}"))); !errors.Is(err, ErrUnknownKind) {
		t.Errorf("unknown kind: %v", err)
	}
	if _, err := DecodeMessage(raw(KindFrame, []byte{1, 2})); !errors.Is(err, ErrFrameKind) {
		t.Errorf("kind 0x01: %v", err)
	}
	for _, body := range []string{`{}`, `{"op":"x","extra":1}`, `not json`, `{"op":"x"} {"op":"y"}`} {
		if _, err := DecodeMessage(raw(KindControl, []byte(body))); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: %v", body, err)
		}
	}
	good := raw(KindControl, []byte(`{"op":"x","id":"1","d":{}}`))
	if _, err := DecodeMessage(append(good, 0)); !errors.Is(err, ErrMalformed) {
		t.Errorf("trailing byte: %v", err)
	}
	if _, err := DecodeMessage(good[:6]); !errors.Is(err, ErrMalformed) {
		t.Errorf("short: %v", err)
	}
	if _, err := Encode(Message{Op: "x", D: json.RawMessage(`{`)}); err == nil {
		t.Error("invalid raw JSON encoded")
	}
}

func FuzzDecodeMessage(f *testing.F) {
	f.Add(raw(KindControl, []byte(`{"op":"hello","id":"1","d":{"version":1}}`)))
	f.Add(raw(KindFrame, []byte{0}))
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := DecodeMessage(b)
		if err == nil && m.Op == "" {
			t.Fatal("message without op")
		}
	})
}

func TestVerifyPeer(t *testing.T) {
	if VerifyPeer(Peer{UID: "1", PID: 5}, Expect{UID: "1", PID: 5}) != nil {
		t.Error("equal peer refused")
	}
	for _, p := range []Peer{{UID: "2", PID: 5}, {UID: "1", PID: 6}, {UID: "1", PID: 0}} {
		if VerifyPeer(p, Expect{UID: "1", PID: 5}) == nil {
			t.Errorf("%+v accepted", p)
		}
	}
	if HelperEndpoint("linux", "a") != "/run/bothy-host/helper-a.sock" || HelperEndpoint("windows", "a") != `\\.\pipe\bothy-host-helper-a` ||
		HelperEndpoint("darwin", "a") != "/Library/Application Support/bothy-host/run/helper-a.sock" {
		t.Error("endpoints")
	}
	if EndpointDir("windows") != "" || EndpointDir("linux") != LinuxRunDir || EndpointDir("darwin") != DarwinRunDir {
		t.Error("endpoint dirs")
	}
}
