// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ticketLen is the length of an encoded ticket: 58 payload bytes and a 32-byte tag in unpadded
// base64url.
const ticketLen = (58 + 32) * 4 / 3

// newKey returns a fresh random ticket key for one test.
func newKey(t testing.TB) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

// testNow is the fake clock of the ticket tests.
var testNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

// testExpiryUnix is the Unix second of the expiry of newTicket, testNow plus five minutes.
const testExpiryUnix = 1792497900

func newTicket(v domain.Variant) domain.Ticket {
	return domain.Ticket{
		FileID:      ids.New(),
		PrincipalID: ids.New(),
		SessionID:   ids.New(),
		Variant:     v,
		ExpiresAt:   testNow.Add(5 * time.Minute),
	}
}

// assertInvalid fails unless err is ErrTicketInvalid; it logs only lengths, never the ticket.
func assertInvalid(t *testing.T, what string, s string, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrTicketInvalid) {
		t.Errorf("%s (ticket of %d bytes): err = %v, want ErrTicketInvalid", what, len(s), err)
	}
	if err != nil && s != "" && strings.Contains(err.Error(), s) {
		t.Errorf("%s: the error text holds the ticket", what)
	}
}

// sign builds a ticket from raw payload bytes, so tests can reach the checks behind the MAC.
func sign(key, payload []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write(payload)
	return base64.RawURLEncoding.EncodeToString(append(append([]byte(nil), payload...), m.Sum(nil)...))
}

// withURLChars mints tickets until one holds "-" or "_", the two characters in which base64url
// differs from standard base64, and returns it.
func withURLChars(t *testing.T, key []byte) string {
	t.Helper()
	for {
		s := domain.EncodeTicket(key, newTicket(domain.VariantOriginal))
		if strings.ContainsAny(s, "-_") {
			return s
		}
	}
}

func TestTicket_RoundTrip(t *testing.T) {
	key := newKey(t)
	for _, v := range []domain.Variant{domain.VariantOriginal, domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster} {
		in := newTicket(v)
		in.ExpiresAt = in.ExpiresAt.Add(700 * time.Millisecond).In(time.FixedZone("x", 3600))
		s := domain.EncodeTicket(key, in)
		if len(s) != ticketLen {
			t.Fatalf("%s: length %d, want %d", v, len(s), ticketLen)
		}
		if strings.Trim(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") != "" {
			t.Fatalf("%s: ticket holds characters outside the URL-safe alphabet or padding", v)
		}
		out, err := domain.DecodeTicket([][]byte{key}, s, testNow)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		want := in
		want.ExpiresAt = time.Unix(in.ExpiresAt.Unix(), 0).UTC()
		if out != want {
			t.Errorf("%s: round trip changed the ticket", v)
		}
		if out.ExpiresAt.Location() != time.UTC {
			t.Errorf("%s: ExpiresAt not in UTC", v)
		}
	}
}

func TestTicket_TamperedByteRejected(t *testing.T) {
	key := newKey(t)
	s := domain.EncodeTicket(key, newTicket(domain.VariantSmall))
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	for i := range raw {
		for _, bit := range []byte{0x01, 0x80, 0xff} {
			b := append([]byte(nil), raw...)
			b[i] ^= bit
			bad := base64.RawURLEncoding.EncodeToString(b)
			_, err := domain.DecodeTicket([][]byte{key}, bad, testNow)
			assertInvalid(t, "flipped byte", bad, err)
		}
	}
}

func TestTicket_WrongLengthAlphabetVersionVariantRejected(t *testing.T) {
	key := newKey(t)
	in := newTicket(domain.VariantOriginal)
	s := domain.EncodeTicket(key, in)
	raw, _ := base64.RawURLEncoding.DecodeString(s)
	payload := raw[:58]

	cases := map[string]string{
		"empty":                 "",
		"one short":             s[:len(s)-1],
		"one long":              s + "A",
		"padding inside length": s[:len(s)-2] + "==",
		"padding appended":      s + "==",
		"standard alphabet":     strings.NewReplacer("-", "+", "_", "/").Replace(withURLChars(t, key)),
		"newline inside":        s[:60] + "\n" + s[61:],
		"newline appended":      s[:len(s)-1] + "\n",
		"space":                 s[:60] + " " + s[61:],
		"short payload":         sign(key, payload[:57]),
		"long payload":          sign(key, append(append([]byte(nil), payload...), 0)),
		"truncated tag":         base64.RawURLEncoding.EncodeToString(raw[:len(raw)-1]),
	}
	for _, version := range []byte{0, 2, 0xff} {
		p := append([]byte(nil), payload...)
		p[0] = version
		cases[fmt.Sprint("version ", version)] = sign(key, p)
	}
	for _, code := range []byte{5, 6, 0x7f, 0xff} {
		p := append([]byte(nil), payload...)
		p[17] = code
		cases[fmt.Sprint("variant code ", code)] = sign(key, p)
	}
	for name, bad := range cases {
		_, err := domain.DecodeTicket([][]byte{key}, bad, testNow)
		assertInvalid(t, name, bad, err)
	}

	// The layout itself: version 1, the file ID, the variant code, principal, session, expiry.
	if payload[0] != domain.TicketVersion || [16]byte(payload[1:17]) != in.FileID ||
		payload[17] != 0 || [16]byte(payload[18:34]) != in.PrincipalID ||
		[16]byte(payload[34:50]) != in.SessionID ||
		binary.BigEndian.Uint64(payload[50:58]) != testExpiryUnix {
		t.Error("payload does not follow the fixed layout")
	}
	codes := map[domain.Variant]byte{domain.VariantOriginal: 0, domain.VariantSmall: 1, domain.VariantMedium: 2, domain.VariantLarge: 3, domain.VariantPoster: 4}
	for v, code := range codes {
		b, _ := base64.RawURLEncoding.DecodeString(domain.EncodeTicket(key, newTicket(v)))
		if b[17] != code {
			t.Errorf("variant %s has code %d, want %d", v, b[17], code)
		}
	}

	// A variant outside the five never yields a ticket that verifies.
	bogus := newTicket(domain.Variant("bogus"))
	bad := domain.EncodeTicket(key, bogus)
	_, err := domain.DecodeTicket([][]byte{key}, bad, testNow)
	assertInvalid(t, "unknown variant minted", bad, err)
}

func TestTicket_ExpiryBoundary(t *testing.T) {
	key := newKey(t)
	in := newTicket(domain.VariantMedium)
	s := domain.EncodeTicket(key, in)
	if _, err := domain.DecodeTicket([][]byte{key}, s, in.ExpiresAt.Add(-time.Second)); err != nil {
		t.Errorf("one second before expiry: %v", err)
	}
	for _, now := range []time.Time{in.ExpiresAt, in.ExpiresAt.Add(time.Nanosecond), in.ExpiresAt.Add(time.Hour)} {
		_, err := domain.DecodeTicket([][]byte{key}, s, now)
		assertInvalid(t, "at or after expiry", s, err)
	}

	// A fraction of a second is cut off when minting, so the ticket never outlives ExpiresAt.
	frac := newTicket(domain.VariantMedium)
	frac.ExpiresAt = frac.ExpiresAt.Add(900 * time.Millisecond)
	s = domain.EncodeTicket(key, frac)
	_, err := domain.DecodeTicket([][]byte{key}, s, frac.ExpiresAt.Truncate(time.Second))
	assertInvalid(t, "truncated expiry", s, err)

	// An expiry before 1970 never turns into one in the far future.
	old := newTicket(domain.VariantSmall)
	old.ExpiresAt = time.Unix(-10, 0)
	s = domain.EncodeTicket(key, old)
	_, err = domain.DecodeTicket([][]byte{key}, s, testNow)
	assertInvalid(t, "expiry before 1970", s, err)

	// An expiry beyond the int64 range of Unix seconds is refused.
	raw, _ := base64.RawURLEncoding.DecodeString(domain.EncodeTicket(key, in))
	p := append([]byte(nil), raw[:58]...)
	binary.BigEndian.PutUint64(p[50:], 1<<63)
	s = sign(key, p)
	_, err = domain.DecodeTicket([][]byte{key}, s, testNow)
	assertInvalid(t, "expiry beyond int64", s, err)
}

func TestTicket_CurrentAndPreviousKey(t *testing.T) {
	older, previous, current := newKey(t), newKey(t), newKey(t)
	in := newTicket(domain.VariantOriginal)
	ring := [][]byte{current, previous}

	byPrevious := domain.EncodeTicket(previous, in)
	if _, err := domain.DecodeTicket(ring, byPrevious, testNow); err != nil {
		t.Errorf("minted with the previous key: %v", err)
	}
	byOlder := domain.EncodeTicket(older, in)
	_, err := domain.DecodeTicket(ring, byOlder, testNow)
	assertInvalid(t, "minted with an older key", byOlder, err)

	byCurrent := domain.EncodeTicket(current, in)
	if _, err := domain.DecodeTicket([][]byte{current}, byCurrent, testNow); err != nil {
		t.Errorf("minted with the current key: %v", err)
	}
	if byCurrent == byPrevious {
		t.Error("the key does not change the ticket")
	}
	// After the next rotation the old current key is the previous one; the old previous is gone.
	next := newKey(t)
	if _, err := domain.DecodeTicket([][]byte{next, current}, byCurrent, testNow); err != nil {
		t.Errorf("current key after rotation: %v", err)
	}
	_, err = domain.DecodeTicket([][]byte{next, current}, byPrevious, testNow)
	assertInvalid(t, "two rotations back", byPrevious, err)

	// Empty keys in the ring are skipped: a ticket under the empty key never verifies.
	byEmpty := domain.EncodeTicket(nil, in)
	for _, r := range [][][]byte{nil, {}, {nil}, {{}, current}} {
		_, err := domain.DecodeTicket(r, byEmpty, testNow)
		assertInvalid(t, "empty key", byEmpty, err)
	}
}

func FuzzDecodeTicket(f *testing.F) {
	key := newKey(f)
	valid := domain.EncodeTicket(key, newTicket(domain.VariantSmall))
	for _, s := range []string{"", "A", valid, valid[:len(valid)-1], valid + "=", strings.Repeat("_", ticketLen), strings.Repeat("A", ticketLen)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		tk, err := domain.DecodeTicket([][]byte{key}, s, testNow)
		if s == valid {
			if err != nil {
				t.Fatalf("the seed ticket does not verify: %v", err)
			}
			return
		}
		if err == nil {
			t.Fatalf("random input of %d bytes verified as variant %s", len(s), tk.Variant)
		}
		if !errors.Is(err, domain.ErrTicketInvalid) {
			t.Fatalf("error does not wrap ErrTicketInvalid: %v", err)
		}
	})
}
