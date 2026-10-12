// SPDX-License-Identifier: Apache-2.0

package frame

import (
	"bytes"
	"errors"
	"testing"
)

// FuzzDecode is the fuzzing of the frame parser that docs/PLAN.md section 5 ("Sicherheit")
// asks for: arbitrary input from either sender gives a frame or one of the sentinel errors,
// never a panic, and a decoded frame encodes back to exactly the input.
func FuzzDecode(f *testing.F) {
	for _, c := range allowed() {
		b, err := Encode(c.f)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b, true)
		f.Add(b, false)
	}
	for _, c := range malformed {
		f.Add(c.b, c.from == FromDevice)
	}
	f.Add([]byte{}, true)
	f.Add([]byte{0x08}, false)
	f.Fuzz(func(t *testing.T, b []byte, fromDevice bool) {
		from := FromHost
		if fromDevice {
			from = FromDevice
		}
		fr, err := Decode(b, from)
		if err != nil {
			for _, sentinel := range []error{ErrEmpty, ErrUnknownType, ErrWrongDirection, ErrMalformed, ErrTooLong} {
				if errors.Is(err, sentinel) {
					return
				}
			}
			t.Fatalf("Decode gave an error that wraps no sentinel (%d bytes)", len(b))
		}
		out, err := Encode(fr)
		if err != nil {
			t.Fatalf("a decoded frame of type 0x%02x does not encode: %v", byte(fr.Type), err)
		}
		if !bytes.Equal(out, b) {
			t.Fatalf("type 0x%02x: re-encoding gives %d bytes for %d input bytes", byte(fr.Type), len(out), len(b))
		}
	})
}
