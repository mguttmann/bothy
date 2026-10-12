// SPDX-License-Identifier: Apache-2.0

package frame

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/noise"
)

// allowed lists one frame of every type with the senders that may send it.
func allowed() []struct {
	f    Frame
	from []Sender
} {
	return []struct {
		f    Frame
		from []Sender
	}{
		{Start(120, 40), []Sender{FromDevice}},
		{Data([]byte("ls\n")), []Sender{FromDevice, FromHost}},
		{Resize(1, 65535), []Sender{FromDevice}},
		{Close(), []Sender{FromDevice}},
		{Exit(-1), []Sender{FromHost}},
		{ReplayBegin(), []Sender{FromHost}},
		{ReplayEnd(), []Sender{FromHost}},
	}
}

func TestRoundTrip(t *testing.T) {
	for _, c := range allowed() {
		b, err := Encode(c.f)
		if err != nil {
			t.Fatalf("Encode type 0x%02x: %v", byte(c.f.Type), err)
		}
		if b[0] != byte(c.f.Type) {
			t.Errorf("type byte 0x%02x, want 0x%02x", b[0], byte(c.f.Type))
		}
		for _, from := range c.from {
			got, err := Decode(b, from)
			if err != nil {
				t.Fatalf("Decode type 0x%02x from %v: %v", byte(c.f.Type), from, err)
			}
			if got.Type != c.f.Type || got.Cols != c.f.Cols || got.Rows != c.f.Rows ||
				got.ExitCode != c.f.ExitCode || !bytes.Equal(got.Data, c.f.Data) {
				t.Errorf("type 0x%02x from %v does not round-trip", byte(c.f.Type), from)
			}
		}
	}
}

func TestBigEndian(t *testing.T) {
	b, err := Encode(Start(0x0102, 0x0304))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, []byte{0x01, 0x01, 0x02, 0x03, 0x04}) {
		t.Errorf("Start(0x0102, 0x0304) = % x", b)
	}
	b, err = Encode(Resize(0x0102, 0x0304))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, []byte{0x03, 0x01, 0x02, 0x03, 0x04}) {
		t.Errorf("Resize(0x0102, 0x0304) = % x", b)
	}
}

func TestExitCodeSigned(t *testing.T) {
	cases := map[int32][]byte{
		-1:            {0x05, 0xff, 0xff, 0xff, 0xff},
		0:             {0x05, 0, 0, 0, 0},
		math.MinInt32: {0x05, 0x80, 0, 0, 0},
		math.MaxInt32: {0x05, 0x7f, 0xff, 0xff, 0xff},
	}
	for code, want := range cases {
		b, err := Encode(Exit(code))
		if err != nil || !bytes.Equal(b, want) {
			t.Errorf("Exit(%d) = % x, %v; want % x", code, b, err, want)
		}
		f, err := Decode(want, FromHost)
		if err != nil || f.ExitCode != code {
			t.Errorf("Decode(% x) = %d, %v", want, f.ExitCode, err)
		}
	}
}

func TestDecodeUnknownType(t *testing.T) {
	for _, from := range []Sender{FromDevice, FromHost} {
		for b := 0; b <= 0xff; b++ {
			if b >= 0x01 && b <= 0x07 {
				continue
			}
			if _, err := Decode([]byte{byte(b), 0, 0, 0, 0}, from); !errors.Is(err, ErrUnknownType) {
				t.Errorf("type 0x%02x from %v: err = %v, want ErrUnknownType", b, from, err)
			}
		}
	}
}

func TestDecodeWrongDirection(t *testing.T) {
	cases := []struct {
		b    []byte
		from Sender
	}{
		{[]byte{0x01, 0, 80, 0, 24}, FromHost},
		{[]byte{0x03, 0, 80, 0, 24}, FromHost},
		{[]byte{0x04}, FromHost},
		{[]byte{0x05, 0, 0, 0, 0}, FromDevice},
		{[]byte{0x06}, FromDevice},
		{[]byte{0x07}, FromDevice},
	}
	for _, c := range cases {
		if _, err := Decode(c.b, c.from); !errors.Is(err, ErrWrongDirection) {
			t.Errorf("type 0x%02x from %v: err = %v, want ErrWrongDirection", c.b[0], c.from, err)
		}
	}
}

// malformed lists every case of TestDecodeMalformed with its sender; FuzzDecode seeds from it.
var malformed = []struct {
	name string
	b    []byte
	from Sender
}{
	{"start 3 bytes", []byte{0x01, 0, 80, 0}, FromDevice},
	{"start 5 bytes", []byte{0x01, 0, 80, 0, 24, 0}, FromDevice},
	{"start 0 columns", []byte{0x01, 0, 0, 0, 24}, FromDevice},
	{"start 0 rows", []byte{0x01, 0, 80, 0, 0}, FromDevice},
	{"resize 3 bytes", []byte{0x03, 0, 80, 0}, FromDevice},
	{"resize 5 bytes", []byte{0x03, 0, 80, 0, 24, 0}, FromDevice},
	{"resize 0 columns", []byte{0x03, 0, 0, 0, 24}, FromDevice},
	{"resize 0 rows", []byte{0x03, 0, 80, 0, 0}, FromDevice},
	{"close with body", []byte{0x04, 0}, FromDevice},
	{"replay_begin with body", []byte{0x06, 0}, FromHost},
	{"replay_end with body", []byte{0x07, 0}, FromHost},
	{"exit 3 bytes", []byte{0x05, 0, 0, 0}, FromHost},
	{"exit 5 bytes", []byte{0x05, 0, 0, 0, 0, 0}, FromHost},
	{"data from device without bytes", []byte{0x02}, FromDevice},
	{"data from host without bytes", []byte{0x02}, FromHost},
}

func TestDecodeMalformed(t *testing.T) {
	for _, from := range []Sender{FromDevice, FromHost} {
		if _, err := Decode(nil, from); !errors.Is(err, ErrEmpty) {
			t.Errorf("empty input from %v: err = %v, want ErrEmpty", from, err)
		}
	}
	for _, c := range malformed {
		if _, err := Decode(c.b, c.from); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: err = %v, want ErrMalformed", c.name, err)
		}
	}
}

func TestEncodeRejectsInvalidFrames(t *testing.T) {
	cases := map[string]Frame{
		"start 0 columns":  Start(0, 24),
		"resize 0 rows":    Resize(80, 0),
		"data empty":       Data(nil),
		"close with data":  {Type: TypeClose, Data: []byte{1}},
		"exit with data":   {Type: TypeExit, Data: []byte{1}},
		"start with data":  {Type: TypeStart, Cols: 1, Rows: 1, Data: []byte{1}},
		"data with size":   {Type: TypeData, Cols: 1, Data: []byte{1}},
		"close with size":  {Type: TypeClose, Rows: 1},
		"replay with exit": {Type: TypeReplayBegin, ExitCode: 1},
	}
	for name, f := range cases {
		if _, err := Encode(f); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: err = %v, want ErrMalformed", name, err)
		}
	}
	for _, typ := range []Type{0x00, 0x08, 0xff} {
		if _, err := Encode(Frame{Type: typ}); !errors.Is(err, ErrUnknownType) {
			t.Errorf("type 0x%02x: err = %v, want ErrUnknownType", byte(typ), err)
		}
	}
}

func TestFrameSizeLimit(t *testing.T) {
	big := bytes.Repeat([]byte{0x5a}, MaxLen-1)
	b, err := Encode(Data(big))
	if err != nil || len(b) != MaxLen {
		t.Fatalf("data of %d bytes: %d bytes, %v", len(big), len(b), err)
	}
	if f, err := Decode(b, FromHost); err != nil || len(f.Data) != len(big) {
		t.Fatalf("decode data of %d bytes: %v", len(big), err)
	}
	tooBig := bytes.Repeat([]byte{0x5a}, MaxLen)
	if _, err := Encode(Data(tooBig)); !errors.Is(err, ErrTooLong) {
		t.Errorf("Encode data of %d bytes: err = %v, want ErrTooLong", len(tooBig), err)
	}
	if _, err := Decode(append([]byte{0x02}, tooBig...), FromDevice); !errors.Is(err, ErrTooLong) {
		t.Errorf("Decode frame of %d bytes: err = %v, want ErrTooLong", len(tooBig)+1, err)
	}
}

func TestMaxLenMatchesNoise(t *testing.T) {
	if MaxLen != noise.MaxPlaintextLen {
		t.Errorf("MaxLen = %d, noise.MaxPlaintextLen = %d", MaxLen, noise.MaxPlaintextLen)
	}
}

func TestFrameIsInput(t *testing.T) {
	for _, c := range allowed() {
		want := c.f.Type == TypeData || c.f.Type == TypeResize
		if c.f.IsInput() != want {
			t.Errorf("type 0x%02x: IsInput = %v, want %v", byte(c.f.Type), c.f.IsInput(), want)
		}
	}
}

func TestDecodeAliasesInput(t *testing.T) {
	b := []byte{0x02, 'a', 'b'}
	f, err := Decode(b, FromDevice)
	if err != nil {
		t.Fatal(err)
	}
	b[1] = 'z'
	if f.Data[0] != 'z' {
		t.Error("Data does not alias the input as documented")
	}
}

func TestErrorsCarryNoContent(t *testing.T) {
	const canary = "CANARY-7f3a91"
	inputs := []struct {
		b    []byte
		from Sender
	}{
		{append([]byte{0x04}, canary...), FromDevice},
		{append([]byte{0x05}, canary...), FromHost},
		{append([]byte{0x02}, canary...), FromHost},
		{append([]byte{0x09}, canary...), FromDevice},
		{append([]byte{0x01}, canary...), FromDevice},
		{append([]byte{0x02}, bytes.Repeat([]byte(canary), 6000)...), FromDevice},
	}
	for _, in := range inputs {
		_, err := Decode(in.b, in.from)
		if err == nil {
			continue
		}
		if strings.Contains(err.Error(), canary) || strings.Contains(err.Error(), "7f3a91") {
			t.Errorf("error of type 0x%02x carries the payload (%d characters)", in.b[0], len(err.Error()))
		}
	}
	_, err := Encode(Frame{Type: TypeClose, Data: []byte(canary)})
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Errorf("Encode error is nil or carries the payload")
	}
}

func TestSenderString(t *testing.T) {
	if FromDevice.String() != "device" || FromHost.String() != "host" || Sender(9).String() != "sender(9)" {
		t.Errorf("String: %q %q %q", FromDevice, FromHost, Sender(9))
	}
}

func TestDecodeUnknownSender(t *testing.T) {
	if _, err := Decode([]byte{0x02, 'a'}, Sender(9)); !errors.Is(err, ErrWrongDirection) {
		t.Errorf("unknown sender: err = %v, want ErrWrongDirection", err)
	}
}
