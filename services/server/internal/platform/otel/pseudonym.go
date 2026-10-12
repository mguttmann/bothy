// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// MinKeyBytes is the shortest master key of the pseudonyms (telemetry.pseudonym_key_ref): 32
// bytes, the output size of HMAC-SHA-256.
const MinKeyBytes = 32

// periodKeyPrefix is the message of the period key, followed by the decimal period index
// (docs/observability/privacy.md "Pseudonyms" item 1).
const periodKeyPrefix = "bothy/telemetry-pseudonym/v1/"

// pseudonymHexLen is the length of a pseudonym in hex characters (16 bytes).
const pseudonymHexLen = 32

var (
	// ErrKeyTooShort reports a master key shorter than MinKeyBytes.
	ErrKeyTooShort = errors.New("otel: pseudonym key shorter than 32 bytes")
	// ErrRotation reports a privacy.telemetry_pseudonym_rotation below one second.
	ErrRotation = errors.New("otel: pseudonym rotation below one second")
	// ErrPrincipalID reports a principal id that is not a UUID in canonical text form.
	ErrPrincipalID = errors.New("otel: principal id is not a UUID")
)

// KeySource resolves a secret reference; *secrets.Provider implements it. The provider cache
// handles a rotated master key: every pseudonym resolves the reference again.
type KeySource interface {
	Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
}

// Pseudonymizer derives bothy.principal.pseudo_id, the pseudonym of a human principal in spans
// and logs (docs/observability/privacy.md "Pseudonyms", 20-privacy.md "Telemetry"):
//
//	p      = floor(unix_seconds(at) / seconds(privacy.telemetry_pseudonym_rotation)), UTC
//	K_p    = HMAC-SHA-256(master, "bothy/telemetry-pseudonym/v1/" + decimal(p))
//	pseudo = hex(HMAC-SHA-256(K_p, lowercase canonical UUID of the principal id))[:32]
//
// Only one secret has to be escrowed; without it, pseudonyms of two periods cannot be linked,
// and with it the owner maps an id for any period (bothy telemetry pseudonym --at). Agents and
// apps are no people and appear by handle; metrics never carry a pseudonym. A Pseudonymizer is
// safe for concurrent use.
type Pseudonymizer struct {
	keys     KeySource
	ref      string
	rotation int64 // seconds
	clock    clock.Clock
}

// NewPseudonymizer checks the rotation and resolves the master key once, so a missing or short
// key fails the start of the process instead of the first request. clk is the source of now for
// Attr; nil means clock.System().
//
// Errors: ErrRotation for a rotation below one second; the error of keys.Resolve; ErrKeyTooShort
// for a key under MinKeyBytes. No error contains the key.
func NewPseudonymizer(ctx context.Context, keys KeySource, ref string, rotation time.Duration, clk clock.Clock) (*Pseudonymizer, error) {
	if rotation < time.Second {
		return nil, ErrRotation
	}
	if clk == nil {
		clk = clock.System()
	}
	p := &Pseudonymizer{keys: keys, ref: ref, rotation: int64(rotation / time.Second), clock: clk}
	if _, err := p.key(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

// key resolves the master key and checks its length.
func (p *Pseudonymizer) key(ctx context.Context) ([]byte, error) {
	v, _, err := p.keys.Resolve(ctx, p.ref)
	if err != nil {
		return nil, fmt.Errorf("otel: pseudonym key: %w", err)
	}
	if v.Len() < MinKeyBytes {
		return nil, ErrKeyTooShort
	}
	return v.Bytes(), nil
}

// Pseudonym returns the pseudonym of principalID in the period that contains at: 32 lowercase
// hex characters. principalID is a UUID in canonical text form, any letter case.
//
// Errors: ErrPrincipalID for another form; the errors of resolving the key (see
// NewPseudonymizer).
func (p *Pseudonymizer) Pseudonym(ctx context.Context, principalID string, at time.Time) (string, error) {
	pseudo, _, err := p.PseudonymWithPeriod(ctx, principalID, at)
	return pseudo, err
}

// PseudonymWithPeriod returns the pseudonym like Pseudonym and the start of its period in UTC.
// bothy telemetry pseudonym prints both.
func (p *Pseudonymizer) PseudonymWithPeriod(ctx context.Context, principalID string, at time.Time) (string, time.Time, error) {
	id, err := canonicalUUID(principalID)
	if err != nil {
		return "", time.Time{}, err
	}
	master, err := p.key(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	defer clear(master)
	period := floorDiv(at.Unix(), p.rotation)
	periodKey := hmacSHA256(master, []byte(periodKeyPrefix+strconv.FormatInt(period, 10)))
	defer clear(periodKey)
	sum := hmacSHA256(periodKey, []byte(id))
	return hex.EncodeToString(sum)[:pseudonymHexLen], time.Unix(period*p.rotation, 0).UTC(), nil
}

// Attr returns the attribute bothy.principal.pseudo_id of principalID for now. Spans and logs
// only; metrics never carry it.
//
// Example:
//
//	kv, err := tel.Pseudonymizer().Attr(ctx, principalID)
//	if err == nil {
//		span.SetAttributes(kv)
//	}
func (p *Pseudonymizer) Attr(ctx context.Context, principalID string) (attribute.KeyValue, error) {
	pseudo, err := p.Pseudonym(ctx, principalID, p.clock.Now())
	if err != nil {
		return attribute.KeyValue{}, err
	}
	return attribute.String(attr.PrincipalPseudoID, pseudo), nil
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}

// floorDiv divides and rounds toward negative infinity, so instants before 1970 get their own
// periods instead of sharing period 0.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// canonicalUUID checks the 8-4-4-4-12 hex form and returns it in lowercase.
func canonicalUUID(s string) (string, error) {
	if len(s) != 36 {
		return "", ErrPrincipalID
	}
	for i := range len(s) {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return "", ErrPrincipalID
			}
		default:
			if !isHex(c) {
				return "", ErrPrincipalID
			}
		}
	}
	return strings.ToLower(s), nil
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
