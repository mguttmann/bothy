// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

// vectorFile is the test vector file of docs/spec/27-computer-use.md 9.4.
const vectorFile = "../../testdata/computer/channel-vectors.json"

type vector struct {
	Name     string          `json:"name"`
	Type     byte            `json:"type"`
	Object   json.RawMessage `json:"object"`
	BytesHex string          `json:"bytes_hex"`
	FrameHex string          `json:"frame_hex"`
}

func loadVectors(t *testing.T) []vector {
	t.Helper()
	b, err := os.ReadFile(vectorFile)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Vectors []vector `json:"vectors"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Vectors) == 0 {
		t.Fatal("no vectors")
	}
	return f.Vectors
}

// asObject decodes b into a generic value for comparison.
func asObject(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestChannelVectors: the decoder reads every vector to its object or bytes, every op decodes into
// its type, and the encoder's output decodes to the same object.
func TestChannelVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		raw, err := hex.DecodeString(v.FrameHex)
		if err != nil {
			t.Fatal(err)
		}
		kind, body, err := ReadFrame(bytes.NewReader(raw), testFrameMax)
		if err != nil || kind != v.Type {
			t.Fatalf("%s: ReadFrame kind %d, %v", v.Name, kind, err)
		}
		if kind == KindImage {
			want, _ := hex.DecodeString(v.BytesHex)
			if !bytes.Equal(body, want) {
				t.Errorf("%s: image bytes differ", v.Name)
			}
			out, err := EncodeFrame(KindImage, want, testFrameMax)
			if err != nil || !bytes.Equal(out, raw) {
				t.Errorf("%s: EncodeFrame differs", v.Name)
			}
			continue
		}
		if !reflect.DeepEqual(asObject(t, body), asObject(t, v.Object)) {
			t.Errorf("%s: object differs", v.Name)
		}
		_, msg, err := DecodeMsg(body)
		if err != nil {
			t.Fatalf("%s: DecodeMsg: %v", v.Name, err)
		}
		out, err := EncodeMsg(msg, testFrameMax)
		if err != nil {
			t.Fatal(err)
		}
		_, again, err := ReadFrame(bytes.NewReader(out), testFrameMax)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(asObject(t, again), asObject(t, v.Object)) {
			t.Errorf("%s: re-encoded object differs: %s", v.Name, again)
		}
	}
}

// TestChannelLimits: a frame above host_service.computer_frame_max_bytes, an empty frame and an
// unknown kind close the channel (27 9.4).
func TestChannelLimits(t *testing.T) {
	big, err := EncodeFrame(KindImage, make([]byte, 100), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFrame(bytes.NewReader(big), 100); !errors.Is(err, ErrFrameTooLong) {
		t.Errorf("over the limit: %v", err)
	}
	if _, _, err := ReadFrame(bytes.NewReader(big), 101); err != nil {
		t.Errorf("at the limit: %v", err)
	}
	if _, err := EncodeFrame(KindImage, make([]byte, 100), 100); !errors.Is(err, ErrFrameTooLong) {
		t.Errorf("encode over the limit: %v", err)
	}
	if _, _, err := ReadFrame(bytes.NewReader([]byte{0, 0, 0, 0}), 100); !errors.Is(err, ErrFrameTooLong) {
		t.Errorf("empty frame: %v", err)
	}
	if _, _, err := ReadFrame(bytes.NewReader([]byte{0, 0, 0, 1, 7}), 100); !errors.Is(err, ErrChannelMalformed) {
		t.Errorf("unknown kind: %v", err)
	}
}

// TestDecodeMsgStrict: unknown ops and members, nulls and trailing data are refused.
func TestDecodeMsgStrict(t *testing.T) {
	for _, b := range []string{
		`{"op":"nope"}`,
		`{"op":"stop","session_id":"x","detail":"local_escape","extra":1}`,
		`{"op":"stop","session_id":null,"detail":"local_escape"}`,
		`{"op":"release"} {}`,
		`[]`,
	} {
		if _, _, err := DecodeMsg([]byte(b)); !errors.Is(err, ErrChannelMalformed) {
			t.Errorf("%s: %v", b, err)
		}
	}
}

// testFrameMax is the default of host_service.computer_frame_max_bytes for the tests.
const testFrameMax = 64 << 20
