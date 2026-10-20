// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ErrTicketInvalid reports a file ticket that does not verify: wrong length, alphabet, version
// or variant code, a tag that no key of the ring produced, or an expired ticket. The caller
// answers every case with the unknown-id 404 (12 "9.2" point 3). The error text never contains
// the ticket, because the ticket is a bearer secret (SEC-LOG-3).
var ErrTicketInvalid = errors.New("files: invalid file ticket")

// TicketVersion is the format version, the first payload byte of every ticket.
const TicketVersion = 1

// Ticket is the content of a file ticket (12 "9.2", ADR 0048 point 2): it lets the holder fetch
// one variant of one file as PrincipalID within the auth session SessionID until ExpiresAt. The
// rights are checked again on every fetch; the ticket only names who asks.
type Ticket struct {
	FileID      ids.ID
	PrincipalID ids.ID
	SessionID   ids.ID
	Variant     Variant
	ExpiresAt   time.Time
}

// The fixed payload layout: version, file ID, variant code, principal ID, auth session ID,
// expiry as unsigned big-endian Unix seconds.
const (
	offFile      = 1
	offVariant   = offFile + 16
	offPrincipal = offVariant + 1
	offSession   = offPrincipal + 16
	offExpiry    = offSession + 16
	payloadLen   = offExpiry + 8
	ticketRawLen = payloadLen + sha256.Size
)

// ticketVariants maps the variant codes to the variants; the index is the code.
var ticketVariants = [...]Variant{VariantOriginal, VariantSmall, VariantMedium, VariantLarge, VariantPoster}

// unknownVariantCode is written for a variant outside ticketVariants; DecodeTicket refuses it.
const unknownVariantCode = 0xff

// ticketTag returns HMAC-SHA-256 of payload under key.
func ticketTag(key, payload []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(payload)
	return m.Sum(nil)
}

// EncodeTicket mints a ticket for t under key, the current key of the ring: base64url without
// padding (RFC 4648 section 5) of the 58-byte payload followed by its full HMAC-SHA-256 tag.
// ExpiresAt is cut to whole Unix seconds, so the ticket never outlives it; an expiry before 1970
// is written as 0, already expired. A variant outside the five constants gets a code that
// DecodeTicket refuses, so such a ticket never verifies. The result is a bearer secret: callers
// never log it.
//
// Example:
//
//	s := domain.EncodeTicket(key, domain.Ticket{FileID: f, PrincipalID: p, SessionID: s,
//		Variant: domain.VariantOriginal, ExpiresAt: now.Add(ttl)}) // 120 characters
func EncodeTicket(key []byte, t Ticket) string {
	p := make([]byte, payloadLen, ticketRawLen)
	p[0] = TicketVersion
	copy(p[offFile:], t.FileID[:])
	p[offVariant] = unknownVariantCode
	for code, v := range ticketVariants {
		if v == t.Variant {
			p[offVariant] = byte(code)
		}
	}
	copy(p[offPrincipal:], t.PrincipalID[:])
	copy(p[offSession:], t.SessionID[:])
	binary.BigEndian.PutUint64(p[offExpiry:], uint64(max(t.ExpiresAt.Unix(), 0)))
	return base64.RawURLEncoding.EncodeToString(append(p, ticketTag(key, p)...))
}

// DecodeTicket verifies s and returns its content. keys is the ring, current and previous key;
// empty keys are skipped. s must be exactly one encoded ticket in the base64url alphabet without
// padding, with version TicketVersion, a known variant code and a tag that one key produced
// (compared with hmac.Equal), and now must be before ExpiresAt. ExpiresAt comes back in UTC.
//
// Every failure returns ErrTicketInvalid itself, without detail, so no failure can be told from
// another and nothing of the ticket reaches a log.
//
// Example:
//
//	t, err := domain.DecodeTicket([][]byte{current, previous}, s, clock.Now())
func DecodeTicket(keys [][]byte, s string, now time.Time) (Ticket, error) {
	// The length check comes first and the alphabet check is explicit, because the decoder
	// skips CR and LF.
	if len(s) != base64.RawURLEncoding.EncodedLen(ticketRawLen) || !urlSafe(s) {
		return Ticket{}, ErrTicketInvalid
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil || len(raw) != ticketRawLen {
		return Ticket{}, ErrTicketInvalid
	}
	payload := raw[:payloadLen]
	if !verifyTag(keys, payload, raw[payloadLen:]) || payload[0] != TicketVersion || int(payload[offVariant]) >= len(ticketVariants) {
		return Ticket{}, ErrTicketInvalid
	}
	expiry := binary.BigEndian.Uint64(payload[offExpiry:])
	if expiry > math.MaxInt64 {
		return Ticket{}, ErrTicketInvalid
	}
	t := Ticket{
		FileID:      ids.ID(payload[offFile:offVariant]),
		PrincipalID: ids.ID(payload[offPrincipal:offSession]),
		SessionID:   ids.ID(payload[offSession:offExpiry]),
		Variant:     ticketVariants[payload[offVariant]],
		ExpiresAt:   time.Unix(int64(expiry), 0).UTC(),
	}
	if !now.Before(t.ExpiresAt) {
		return Ticket{}, ErrTicketInvalid
	}
	return t, nil
}

// urlSafe reports whether s holds only characters of the base64url alphabet, padding excluded.
func urlSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// verifyTag reports whether one non-empty key of the ring produced tag over payload.
func verifyTag(keys [][]byte, payload, tag []byte) bool {
	for _, k := range keys {
		if len(k) > 0 && hmac.Equal(tag, ticketTag(k, payload)) {
			return true
		}
	}
	return false
}
