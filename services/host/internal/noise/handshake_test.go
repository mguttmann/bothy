// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/flynn/noise"
)

// testKey derives a deterministic private key from a label (no secret: test data).
func testKey(t testing.TB, label string) KeyPair {
	t.Helper()
	kp, err := KeyPairFromPrivate(sha256.Sum256([]byte(label)))
	if err != nil {
		t.Fatal(err)
	}
	return kp
}

// handshakePair runs a full handshake with the test keys and returns both transports.
func handshakePair(t testing.TB) (device, host *Transport) {
	t.Helper()
	sid, hid := mustID(t, goSessionID), mustID(t, goHostID)
	static, eph := testKey(t, "test/static"), testKey(t, "test/eph-i")
	init, err := NewInitiator(InitiatorConfig{HostStatic: static.Public, SessionID: sid, HostID: hid, Ephemeral: eph})
	if err != nil {
		t.Fatal(err)
	}
	respEph := testKey(t, "test/eph-r")
	resp, err := NewResponder(ResponderConfig{Static: static, SessionID: sid, HostID: hid, Ephemeral: &respEph})
	if err != nil {
		t.Fatal(err)
	}
	m1, err := init.WriteMessage1([]byte("token"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := resp.ReadMessage1(m1); err != nil {
		t.Fatal(err)
	}
	m2, host, err := resp.WriteMessage2([]byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_, device, err = init.ReadMessage2(m2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(init.HandshakeHash(), resp.HandshakeHash()) || len(init.HandshakeHash()) != 32 {
		t.Fatal("handshake hashes differ")
	}
	return device, host
}

func TestHandshakeRoundTrip(t *testing.T) {
	device, host := handshakePair(t)
	for i := 0; i < 3; i++ {
		msg, err := device.Encrypt([]byte("ping"))
		if err != nil {
			t.Fatal(err)
		}
		if pt, err := host.Decrypt(msg); err != nil || string(pt) != "ping" {
			t.Fatalf("host decrypt: %v", err)
		}
		msg, err = host.Encrypt([]byte("pong"))
		if err != nil {
			t.Fatal(err)
		}
		if pt, err := device.Decrypt(msg); err != nil || string(pt) != "pong" {
			t.Fatalf("device decrypt: %v", err)
		}
	}
}

func TestResponderGeneratesEphemeral(t *testing.T) {
	static := testKey(t, "test/static")
	resp, err := NewResponder(ResponderConfig{Static: static, Random: bytes.NewReader(bytes.Repeat([]byte{9}, 32))})
	if err != nil {
		t.Fatal(err)
	}
	init, err := NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: testKey(t, "test/eph-i")})
	if err != nil {
		t.Fatal(err)
	}
	m1, _ := init.WriteMessage1(nil)
	payload, e, err := resp.ReadMessage1(m1)
	if err != nil || len(payload) != 0 || e != testKey(t, "test/eph-i").Public {
		t.Fatalf("ReadMessage1: %v", err)
	}
	m2, _, err := resp.WriteMessage2(nil)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := KeyPairFromPrivate([KeyLen]byte(bytes.Repeat([]byte{9}, KeyLen)))
	if !bytes.Equal(m2[:32], want.Public[:]) {
		t.Error("the responder did not take its ephemeral key from Random")
	}
}

func TestInvalidConfigs(t *testing.T) {
	static := testKey(t, "test/static")
	bad := static
	bad.Public[0] ^= 1
	if _, err := NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: bad}); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("initiator with a mismatched ephemeral pair: %v", err)
	}
	if _, err := NewResponder(ResponderConfig{Static: bad}); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("responder with a mismatched static pair: %v", err)
	}
	if _, err := NewResponder(ResponderConfig{Static: static, Ephemeral: &bad}); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("responder with a mismatched ephemeral pair: %v", err)
	}
	if _, err := NewResponder(ResponderConfig{Static: static, Random: bytes.NewReader(nil)}); err == nil {
		t.Error("responder with an empty random source")
	}
}

func TestMessageLimits(t *testing.T) {
	device, host := handshakePair(t)
	max := bytes.Repeat([]byte{0xa5}, MaxPlaintextLen)
	msg, err := device.Encrypt(max)
	if err != nil || len(msg) != MaxMessageLen {
		t.Fatalf("Encrypt of %d bytes: %d bytes, %v", len(max), len(msg), err)
	}
	if pt, err := host.Decrypt(msg); err != nil || len(pt) != len(max) {
		t.Fatalf("Decrypt of %d bytes: %v", len(msg), err)
	}
	if _, err := device.Encrypt(append(max, 0)); !errors.Is(err, ErrMessageTooLong) {
		t.Fatalf("Encrypt of %d bytes: err = %v, want ErrMessageTooLong", len(max)+1, err)
	}
	// The refused call used no nonce: the next message decrypts with the expected one.
	msg, err = device.Encrypt([]byte("next"))
	if err != nil {
		t.Fatal(err)
	}
	if pt, err := host.Decrypt(msg); err != nil || string(pt) != "next" {
		t.Fatalf("message after the refused one: %v", err)
	}

	_, host2 := handshakePair(t)
	if _, err := host2.Decrypt(make([]byte, MaxMessageLen+1)); !errors.Is(err, ErrMessageTooLong) {
		t.Errorf("Decrypt of %d bytes: err = %v", MaxMessageLen+1, err)
	}
	_, host3 := handshakePair(t)
	if _, err := host3.Decrypt(make([]byte, TagLen-1)); !errors.Is(err, ErrDecrypt) {
		t.Errorf("Decrypt of %d bytes: err = %v", TagLen-1, err)
	}

	static := testKey(t, "test/static")
	init, _ := NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: testKey(t, "test/eph-i")})
	if _, err := init.WriteMessage1(make([]byte, MaxMessageLen-KeyLen-TagLen+1)); !errors.Is(err, ErrMessageTooLong) {
		t.Errorf("message-1 payload over the limit: err = %v", err)
	}
	init, _ = NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: testKey(t, "test/eph-i")})
	if m1, err := init.WriteMessage1(make([]byte, MaxMessageLen-KeyLen-TagLen)); err != nil || len(m1) != MaxMessageLen {
		t.Errorf("largest message 1: %d bytes, %v", len(m1), err)
	}
	resp, _ := NewResponder(ResponderConfig{Static: static})
	if _, _, err := resp.ReadMessage1(make([]byte, MaxMessageLen+1)); !errors.Is(err, ErrMessageTooLong) {
		t.Errorf("message 1 of %d bytes: err = %v", MaxMessageLen+1, err)
	}
	resp, _ = NewResponder(ResponderConfig{Static: static})
	init, _ = NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: testKey(t, "test/eph-i")})
	m1, _ := init.WriteMessage1(nil)
	if _, _, err := resp.ReadMessage1(m1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resp.WriteMessage2(make([]byte, MaxMessageLen-KeyLen-TagLen+1)); !errors.Is(err, ErrMessageTooLong) {
		t.Errorf("message-2 payload over the limit: err = %v", err)
	}
	if _, _, err := init.ReadMessage2(make([]byte, MaxMessageLen+1)); !errors.Is(err, ErrMessageTooLong) {
		t.Errorf("message 2 of %d bytes: err = %v", MaxMessageLen+1, err)
	}
}

func TestLowOrderKeyRejected(t *testing.T) {
	init, err := NewInitiator(InitiatorConfig{Ephemeral: testKey(t, "test/eph-i")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := init.WriteMessage1(nil); !errors.Is(err, ErrHandshake) {
		t.Errorf("all-zero host key: err = %v, want ErrHandshake", err)
	}
	resp, err := NewResponder(ResponderConfig{Static: testKey(t, "test/static")})
	if err != nil {
		t.Fatal(err)
	}
	m1 := make([]byte, KeyLen+TagLen)
	if _, _, err := resp.ReadMessage1(m1); !errors.Is(err, ErrHandshake) {
		t.Errorf("all-zero ephemeral key in message 1: err = %v, want ErrHandshake", err)
	}
	// The responder side of message 2: an initiator that reads a low-order ephemeral key.
	static := testKey(t, "test/static")
	init, _ = NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: testKey(t, "test/eph-i")})
	if _, err := init.WriteMessage1(nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := init.ReadMessage2(make([]byte, KeyLen+TagLen)); !errors.Is(err, ErrHandshake) {
		t.Errorf("all-zero ephemeral key in message 2: err = %v, want ErrHandshake", err)
	}
}

func TestBrokenAfterFailure(t *testing.T) {
	device, host := handshakePair(t)
	msg, _ := device.Encrypt([]byte("data"))
	tampered := bytes.Clone(msg)
	tampered[0] ^= 0x01
	if _, err := host.Decrypt(tampered); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("flipped bit: err = %v, want ErrDecrypt", err)
	}
	if _, err := host.Decrypt(msg); !errors.Is(err, ErrBroken) {
		t.Errorf("valid message after a failure: err = %v, want ErrBroken", err)
	}
	if _, err := host.Encrypt([]byte("x")); !errors.Is(err, ErrBroken) {
		t.Errorf("Encrypt after a failure: err = %v, want ErrBroken", err)
	}

	// A failed handshake step breaks the handshake object as well.
	static := testKey(t, "test/static")
	resp, _ := NewResponder(ResponderConfig{Static: static})
	init, _ := NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: testKey(t, "test/eph-i")})
	m1, _ := init.WriteMessage1([]byte("token"))
	bad := bytes.Clone(m1)
	bad[len(bad)-1] ^= 1
	if _, _, err := resp.ReadMessage1(bad); !errors.Is(err, ErrHandshake) {
		t.Fatalf("tampered message 1: %v", err)
	}
	if _, _, err := resp.ReadMessage1(m1); !errors.Is(err, ErrBroken) {
		t.Errorf("valid message 1 after a failure: err = %v, want ErrBroken", err)
	}
	if _, _, err := resp.WriteMessage2(nil); !errors.Is(err, ErrBroken) {
		t.Errorf("WriteMessage2 after a failure: err = %v, want ErrBroken", err)
	}
	if _, _, err := init.ReadMessage2(make([]byte, 10)); !errors.Is(err, ErrHandshake) {
		t.Fatalf("short message 2: %v", err)
	}
	if _, _, err := init.ReadMessage2(make([]byte, 10)); !errors.Is(err, ErrBroken) {
		t.Errorf("after a failed message 2: err = %v, want ErrBroken", err)
	}
	if _, err := init.WriteMessage1(nil); !errors.Is(err, ErrBroken) {
		t.Errorf("WriteMessage1 after a failure: err = %v, want ErrBroken", err)
	}
}

func TestNonceExhausted(t *testing.T) {
	device, host := handshakePair(t)
	device.setNonces(math.MaxUint64-1, 0)
	host.setNonces(0, math.MaxUint64-1)
	msg, err := device.Encrypt([]byte("last"))
	if err != nil {
		t.Fatalf("nonce 2^64-2: %v", err)
	}
	if pt, err := host.Decrypt(msg); err != nil || string(pt) != "last" {
		t.Fatalf("decrypt at nonce 2^64-2: %v", err)
	}
	if _, err := device.Encrypt([]byte("one more")); !errors.Is(err, ErrNonceExhausted) {
		t.Errorf("Encrypt at nonce 2^64-1: err = %v, want ErrNonceExhausted", err)
	}
	if _, err := host.Decrypt(msg); !errors.Is(err, ErrNonceExhausted) {
		t.Errorf("Decrypt at nonce 2^64-1: err = %v, want ErrNonceExhausted", err)
	}
}

func TestCallOrder(t *testing.T) {
	static := testKey(t, "test/static")
	init, _ := NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: testKey(t, "test/eph-i")})
	if _, _, err := init.ReadMessage2(make([]byte, 48)); !errors.Is(err, ErrState) {
		t.Errorf("ReadMessage2 before WriteMessage1: %v", err)
	}
	if init.HandshakeHash() != nil {
		t.Error("handshake hash before completion")
	}
	m1, err := init.WriteMessage1(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := init.WriteMessage1(nil); !errors.Is(err, ErrState) {
		t.Errorf("second WriteMessage1: %v", err)
	}
	resp, _ := NewResponder(ResponderConfig{Static: static})
	if _, _, err := resp.WriteMessage2(nil); !errors.Is(err, ErrState) {
		t.Errorf("WriteMessage2 before ReadMessage1: %v", err)
	}
	if resp.HandshakeHash() != nil {
		t.Error("handshake hash before completion")
	}
	if _, _, err := resp.ReadMessage1(m1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resp.ReadMessage1(m1); !errors.Is(err, ErrState) {
		t.Errorf("second ReadMessage1: %v", err)
	}
	m2, _, err := resp.WriteMessage2(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := resp.WriteMessage2(nil); !errors.Is(err, ErrState) {
		t.Errorf("second WriteMessage2: %v", err)
	}
	if _, _, err := init.ReadMessage2(m2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := init.ReadMessage2(m2); !errors.Is(err, ErrState) {
		t.Errorf("second ReadMessage2: %v", err)
	}
}

func TestConcurrentDirections(t *testing.T) {
	device, host := handshakePair(t)
	const n = 200
	toHost := make(chan []byte, n)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // the device's writer
		defer wg.Done()
		for i := 0; i < n; i++ {
			msg, err := device.Encrypt([]byte{byte(i)})
			if err != nil {
				t.Error(err)
				return
			}
			toHost <- msg
		}
		close(toHost)
	}()
	go func() { // the device's reader, at the same time as its writer
		defer wg.Done()
		for i := 0; i < n; i++ {
			msg, err := host.Encrypt([]byte{byte(i)})
			if err != nil {
				t.Error(err)
				return
			}
			if pt, err := device.Decrypt(msg); err != nil || pt[0] != byte(i) {
				t.Errorf("device decrypt %d: %v", i, err)
				return
			}
		}
	}()
	i := 0
	for msg := range toHost {
		if pt, err := host.Decrypt(msg); err != nil || pt[0] != byte(i) {
			t.Fatalf("host decrypt %d: %v", i, err)
		}
		i++
	}
	wg.Wait()
}

// TestErrorsCarryNoSecrets runs every failing call with a canary payload and the test keys and
// checks that no error text contains the canary or a key in hex or base64url (SEC-TERM-1,
// R-TM-10, docs/spec/24-terminal.md 10.1).
func TestErrorsCarryNoSecrets(t *testing.T) {
	const canary = "CANARY-7f3a91"
	static, eph := testKey(t, "test/static"), testKey(t, "test/eph-i")
	var secrets []string
	for _, k := range [][KeyLen]byte{static.Private, static.Public, eph.Private, eph.Public} {
		secrets = append(secrets, hex.EncodeToString(k[:]), strings.ToUpper(hex.EncodeToString(k[:])),
			base64.RawURLEncoding.EncodeToString(k[:]), base64.StdEncoding.EncodeToString(k[:]))
	}
	var errs []error
	collect := func(err error) {
		if err == nil {
			t.Error("an expected failure did not fail")
			return
		}
		errs = append(errs, err)
	}
	// Handshake failures: tampered message 1, wrong prologue, oversize, low order.
	init, _ := NewInitiator(InitiatorConfig{HostStatic: static.Public, Ephemeral: eph})
	m1, _ := init.WriteMessage1([]byte(canary))
	tampered := bytes.Clone(m1)
	tampered[40] ^= 1
	resp, _ := NewResponder(ResponderConfig{Static: static})
	_, _, err := resp.ReadMessage1(tampered)
	collect(err)
	_, _, err = resp.ReadMessage1(m1)
	collect(err)
	resp, _ = NewResponder(ResponderConfig{Static: static, SessionID: [IDLen]byte{1}})
	_, _, err = resp.ReadMessage1(m1)
	collect(err)
	resp, _ = NewResponder(ResponderConfig{Static: static})
	_, _, err = resp.ReadMessage1(append(bytes.Repeat([]byte(canary), 6000), m1...))
	collect(err)
	_, err = (&Initiator{}).WriteMessage1([]byte(canary))
	collect(err)
	init, _ = NewInitiator(InitiatorConfig{Ephemeral: eph})
	_, err = init.WriteMessage1([]byte(canary))
	collect(err)
	_, err = EphemeralKeyHash([]byte(canary))
	collect(err)
	_, err = ParseID(canary)
	collect(err)
	// Transport failures.
	device, host := handshakePair(t)
	msg, _ := device.Encrypt([]byte(canary))
	msg[0] ^= 1
	_, err = host.Decrypt(msg)
	collect(err)
	_, err = host.Decrypt(msg)
	collect(err)
	_, err = device.Encrypt(bytes.Repeat([]byte(canary), 6000))
	collect(err)
	for _, e := range errs {
		text := e.Error()
		if strings.Contains(text, canary) || strings.Contains(text, "7f3a91") {
			t.Errorf("an error text carries the payload (%d characters)", len(text))
		}
		for _, s := range secrets {
			if strings.Contains(text, s) || strings.Contains(text, s[:16]) {
				t.Errorf("an error text carries key material (%d characters)", len(text))
			}
		}
	}
}

// zeroDH stands for a curve implementation that does not reject low-order points itself.
type zeroDH struct{ noise.DHFunc }

func (zeroDH) DH(_, _ []byte) ([]byte, error) { return make([]byte, KeyLen), nil }

// failingDH stands for a curve implementation that reports its own error.
type failingDH struct{ noise.DHFunc }

func (failingDH) DH(_, _ []byte) ([]byte, error) { return nil, errors.New("curve failure") }

// TestSafeDHRejectsZeroResult holds the own check of D-M2-38-9, which crypto/ecdh and
// x/crypto/curve25519 make unreachable today: a zero result fails even if the library returns it.
func TestSafeDHRejectsZeroResult(t *testing.T) {
	if _, err := (safeDH{zeroDH{noise.DH25519}}).DH(make([]byte, KeyLen), make([]byte, KeyLen)); err == nil {
		t.Error("a zero Diffie-Hellman result passed")
	}
	if _, err := (safeDH{failingDH{noise.DH25519}}).DH(make([]byte, KeyLen), make([]byte, KeyLen)); err == nil {
		t.Error("a failing curve function passed")
	}
	kp := testKey(t, "test/static")
	peer := testKey(t, "test/eph-i")
	out, err := (safeDH{noise.DH25519}).DH(kp.Private[:], peer.Public[:])
	if err != nil || len(out) != KeyLen {
		t.Errorf("a valid exchange failed: %v", err)
	}
}
