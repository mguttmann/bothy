// SPDX-License-Identifier: Apache-2.0

package frame

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// computerFrames lists one valid frame of every computer type with the sender that may send it
// (docs/spec/27-computer-use.md 6.3).
func computerFrames(t *testing.T) []struct {
	f    Frame
	from Sender
} {
	t.Helper()
	must := func(f Frame, err error) Frame {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	return []struct {
		f    Frame
		from Sender
	}{
		{Close(), FromDevice},
		{must(NewAction(Action{Seq: 1, Action: "left_click", Args: []byte(`{"coordinate":[10,20],"scale_id":1}`)})), FromDevice},
		{must(NewAction(Action{Seq: 2, Action: "key", Args: []byte(`{"keys":"Return"}`), Quick: true})), FromDevice},
		{must(NewResult(Result{Seq: 1, Code: "ok", Image: true, Width: 1280, Height: 800, ScaleID: 1, App: App{ID: "com.example.editor", Name: "Editor"}})), FromHost},
		{must(NewResult(Result{Seq: 2, Code: "ok", App: App{}, Cursor: &Point{X: 3, Y: 4}})), FromHost},
		{must(NewImageBegin(ImageBegin{Seq: 1, Format: "jpeg", Width: 1280, Height: 800, ScaleID: 1, Bytes: 4})), FromHost},
		{ImageData([]byte{0xff, 0xd8, 0xff, 0xd9}), FromHost},
		{ImageEnd(), FromHost},
		{must(NewState(State{State: "paused", Reason: "locked"})), FromHost},
	}
}

// TestComputerRoundTrip: every computer type encodes and decodes on a computer session (CU-T09).
func TestComputerRoundTrip(t *testing.T) {
	for _, c := range computerFrames(t) {
		b, err := Encode(c.f)
		if err != nil {
			t.Fatalf("Encode type 0x%02x: %v", byte(c.f.Type), err)
		}
		got, err := Decode(b, KindComputer, c.from)
		if err != nil {
			t.Fatalf("Decode type 0x%02x: %v", byte(c.f.Type), err)
		}
		if got.Type != c.f.Type || !bytes.Equal(got.Data, c.f.Data) {
			t.Errorf("type 0x%02x does not round-trip", byte(c.f.Type))
		}
	}
}

// TestComputerTypesRefusedOnTerminal: the computer types are unknown on a terminal session, so the
// terminal behaviour is unchanged (CU-T09).
func TestComputerTypesRefusedOnTerminal(t *testing.T) {
	for _, c := range computerFrames(t) {
		if c.f.Type == TypeClose {
			continue
		}
		b, err := Encode(c.f)
		if err != nil {
			t.Fatal(err)
		}
		for _, from := range []Sender{FromDevice, FromHost} {
			if _, err := Decode(b, KindTerminal, from); !errors.Is(err, ErrUnknownType) {
				t.Errorf("type 0x%02x from %v on a terminal session: %v, want ErrUnknownType", byte(c.f.Type), from, err)
			}
		}
	}
}

// TestTerminalTypesRefusedOnComputer: start, data, resize, exit, replay_begin and replay_end are
// refused on a computer session; close passes (CU-T09).
func TestTerminalTypesRefusedOnComputer(t *testing.T) {
	for _, c := range allowed() {
		b, err := Encode(c.f)
		if err != nil {
			t.Fatal(err)
		}
		for _, from := range c.from {
			_, err := Decode(b, KindComputer, from)
			if c.f.Type == TypeClose {
				if err != nil {
					t.Errorf("close on a computer session: %v", err)
				}
				continue
			}
			if !errors.Is(err, ErrUnknownType) {
				t.Errorf("type 0x%02x from %v on a computer session: %v, want ErrUnknownType", byte(c.f.Type), from, err)
			}
		}
	}
}

// TestComputerDirections: every computer type from the wrong side is ErrWrongDirection (CU-T09).
func TestComputerDirections(t *testing.T) {
	for _, c := range computerFrames(t) {
		b, err := Encode(c.f)
		if err != nil {
			t.Fatal(err)
		}
		wrong := FromHost
		if c.from == FromHost {
			wrong = FromDevice
		}
		if _, err := Decode(b, KindComputer, wrong); !errors.Is(err, ErrWrongDirection) {
			t.Errorf("type 0x%02x from %v: %v, want ErrWrongDirection", byte(c.f.Type), wrong, err)
		}
	}
}

// TestComputerMalformed: wrong contents end the session (CU-T09, 27 6.2: one JSON object without
// unknown members, numbers are integers).
func TestComputerMalformed(t *testing.T) {
	cases := []struct {
		name string
		b    []byte
		from Sender
	}{
		{"action unknown member", append([]byte{0x20}, `{"seq":1,"action":"screenshot","args":{},"x":1}`...), FromDevice},
		{"action missing args", append([]byte{0x20}, `{"seq":1,"action":"screenshot"}`...), FromDevice},
		{"action args no object", append([]byte{0x20}, `{"seq":1,"action":"screenshot","args":[]}`...), FromDevice},
		{"action float seq", append([]byte{0x20}, `{"seq":1.5,"action":"screenshot","args":{}}`...), FromDevice},
		{"action null", append([]byte{0x20}, `{"seq":1,"action":null,"args":{}}`...), FromDevice},
		{"action trailing", append([]byte{0x20}, `{"seq":1,"action":"screenshot","args":{}} {}`...), FromDevice},
		{"action no json", append([]byte{0x20}, `seq`...), FromDevice},
		{"action empty", []byte{0x20}, FromDevice},
		{"action seq zero", append([]byte{0x20}, `{"seq":0,"action":"screenshot","args":{}}`...), FromDevice},
		{"action empty name", append([]byte{0x20}, `{"seq":1,"action":"","args":{}}`...), FromDevice},
		{"action quick no bool", append([]byte{0x20}, `{"seq":1,"action":"key","args":{"keys":"a"},"quick":1}`...), FromDevice},
		{"action quick null", append([]byte{0x20}, `{"seq":1,"action":"key","args":{"keys":"a"},"quick":null}`...), FromDevice},
		{"result missing app", append([]byte{0x21}, `{"seq":1,"code":"ok","image":false,"width":0,"height":0,"scale_id":0}`...), FromHost},
		{"begin bad format", append([]byte{0x22}, `{"seq":1,"format":"gif","width":1,"height":1,"scale_id":1,"bytes":1}`...), FromHost},
		{"begin zero bytes", append([]byte{0x22}, `{"seq":1,"format":"png","width":1,"height":1,"scale_id":1,"bytes":0}`...), FromHost},
		{"data empty", []byte{0x23}, FromHost},
		{"end with body", []byte{0x24, 0x00}, FromHost},
		{"state bad", append([]byte{0x25}, `{"state":"sleeping","reason":""}`...), FromHost},
		{"close with body", []byte{0x04, 0x00}, FromDevice},
	}
	for _, c := range cases {
		if _, err := Decode(c.b, KindComputer, c.from); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: %v, want ErrMalformed", c.name, err)
		}
	}
}

// TestComputerUnknownKind: a decode without a known kind refuses every frame.
func TestComputerUnknownKind(t *testing.T) {
	if _, err := Decode([]byte{0x04}, Kind(0), FromDevice); !errors.Is(err, ErrUnknownType) {
		t.Errorf("kind 0: %v", err)
	}
}

// TestImageDataLimit: an image data frame carries 1 to MaxImageChunk bytes (CU-T10).
func TestImageDataLimit(t *testing.T) {
	if MaxImageChunk != 65518 {
		t.Fatalf("MaxImageChunk = %d", MaxImageChunk)
	}
	b, err := Encode(ImageData(bytes.Repeat([]byte{1}, MaxImageChunk)))
	if err != nil || len(b) != MaxLen {
		t.Fatalf("full chunk: %d bytes, %v", len(b), err)
	}
	if _, err := Encode(ImageData(bytes.Repeat([]byte{1}, MaxImageChunk+1))); !errors.Is(err, ErrTooLong) {
		t.Errorf("chunk over the limit: %v", err)
	}
}

// TestComputerParse returns the typed bodies.
func TestComputerParse(t *testing.T) {
	f, err := Decode(append([]byte{0x20}, `{"seq":7,"action":"key","args":{"keys":"cmd+q"}}`...), KindComputer, FromDevice)
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.ActionBody()
	if err != nil || a.Seq != 7 || a.Action != "key" || !strings.Contains(string(a.Args), "cmd+q") || a.Quick {
		t.Errorf("ActionBody = %+v, %v", a, err)
	}
	// quick is optional: a batch item before the last one (27 6.3, 6.4).
	f, err = Decode(append([]byte{0x20}, `{"seq":8,"action":"key","args":{"keys":"a"},"quick":true}`...), KindComputer, FromDevice)
	if err != nil {
		t.Fatal(err)
	}
	if a, err = f.ActionBody(); err != nil || !a.Quick {
		t.Errorf("ActionBody quick = %+v, %v", a, err)
	}
	f, err = Decode(append([]byte{0x25}, `{"state":"stopped","reason":"done"}`...), KindComputer, FromHost)
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.StateBody()
	if err != nil || s.State != "stopped" || s.Reason != "done" {
		t.Errorf("StateBody = %+v, %v", s, err)
	}
}
