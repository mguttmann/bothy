// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"testing"
)

// FuzzReadMessage1 feeds arbitrary bytes to a responder as message 1: an error or a result,
// never a panic.
func FuzzReadMessage1(f *testing.F) {
	static := testKey(f, "test/static")
	init, err := NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: testKey(f, "test/eph-i")})
	if err != nil {
		f.Fatal(err)
	}
	m1, err := init.WriteMessage1([]byte("token"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(m1)
	f.Add([]byte{})
	f.Add(make([]byte, KeyLen+TagLen))
	f.Fuzz(func(t *testing.T, msg []byte) {
		resp, err := NewResponder(ResponderConfig{Static: static})
		if err != nil {
			t.Fatal(err)
		}
		_, _, _ = resp.ReadMessage1(msg)
	})
}

// FuzzDecrypt feeds arbitrary bytes to a transport: an error or a plaintext, never a panic.
func FuzzDecrypt(f *testing.F) {
	device, _ := handshakePair(f)
	msg, err := device.Encrypt([]byte("data"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(msg)
	f.Add([]byte{})
	f.Add(make([]byte, TagLen))
	f.Fuzz(func(t *testing.T, msg []byte) {
		_, host := handshakePair(t)
		_, _ = host.Decrypt(msg)
	})
}
