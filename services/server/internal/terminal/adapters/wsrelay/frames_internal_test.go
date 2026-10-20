// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"bytes"
	"errors"
	"testing"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// FuzzReadFrame: any input gives a forwarded frame, errProtocol or the end of the stream, never a
// panic and never a frame above the Noise limit; a binary message within the limit comes back
// byte for byte.
func FuzzReadFrame(f *testing.F) {
	f.Add(true, []byte{})
	f.Add(true, []byte{1, 2, 3})
	f.Add(false, []byte(`{"op":"identify"}`))
	f.Add(true, bytes.Repeat([]byte{7}, domain.NoiseMaxMessage+1))
	f.Fuzz(func(t *testing.T, binary bool, in []byte) {
		typ := websocket.MessageText
		if binary {
			typ = websocket.MessageBinary
		}
		got, err := classifyFrame(typ, bytes.NewReader(in))
		switch {
		case err != nil && !errors.Is(err, errProtocol):
			t.Fatalf("unexpected error %v", err)
		case len(got) > domain.NoiseMaxMessage:
			t.Fatalf("frame of %d bytes", len(got))
		case err == nil && (!binary || !bytes.Equal(got, in)):
			t.Fatalf("frame not forwarded unchanged")
		case err != nil && binary && len(in) <= domain.NoiseMaxMessage:
			t.Fatalf("a binary message within the limit was refused")
		}
	})
}

// TestClassifyFrame: the edges of the Noise limit and a text frame.
func TestClassifyFrame(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 1000, domain.NoiseMaxMessage - 1, domain.NoiseMaxMessage} {
		if b, err := classifyFrame(websocket.MessageBinary, bytes.NewReader(make([]byte, n))); err != nil || len(b) != n {
			t.Errorf("%d bytes: %d %v", n, len(b), err)
		}
	}
	if _, err := classifyFrame(websocket.MessageBinary, bytes.NewReader(make([]byte, domain.NoiseMaxMessage+1))); !errors.Is(err, errProtocol) {
		t.Errorf("above the limit: %v", err)
	}
	if _, err := classifyFrame(websocket.MessageText, bytes.NewReader([]byte("x"))); !errors.Is(err, errProtocol) {
		t.Errorf("text: %v", err)
	}
}
