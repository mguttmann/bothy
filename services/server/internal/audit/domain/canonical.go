// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The decisions of audit_events.decision (03, CHECK on the column).
const (
	// DecisionAllow records an allowed action.
	DecisionAllow = "allow"
	// DecisionDeny records a denial.
	DecisionDeny = "deny"
	// DecisionError records a decision that failed (for example unknown_action).
	DecisionError = "error"
)

// MaxSafeInteger is 2^53 - 1, the largest integer every JSON reader (I-JSON, JavaScript) holds
// exactly; the canonical form refuses larger magnitudes (SEC-AUD-4, RFC 8785 section 3.2.2.3).
const MaxSafeInteger = 1<<53 - 1

// HashSize is the size of prev_hash and hash in bytes (SHA-256).
const HashSize = sha256.Size

// ZeroHash is the prev_hash of row 1: 32 zero bytes (03 audit_events).
var ZeroHash [HashSize]byte

// Row is one row of audit_events with every column (03 "4.12"). A zero ids.ID and an empty
// string stand for NULL in the optional columns; Details is the decoded jsonb object (never nil,
// {} when empty), with integers as int64, int or json.Number.
type Row struct {
	// Seq is the number from sequence_counters('audit_seq'), from 1 without gaps.
	Seq int64
	// ID is the UUID of the row.
	ID ids.ID
	// OccurredAt is the time of the decision, UTC with millisecond precision.
	OccurredAt time.Time
	// ActorPrincipalID, ActorDeviceID and ActorSessionID identify the actor; zero for system
	// actions and server commands.
	ActorPrincipalID, ActorDeviceID, ActorSessionID ids.ID
	// Action is the action id of 05 (system actions included).
	Action string
	// Decision is DecisionAllow, DecisionDeny or DecisionError.
	Decision string
	// DenyReason is the reason code of 07 "4.1"; empty for none.
	DenyReason string
	// ResourceKind and ResourceID name the resource; empty and zero for none.
	ResourceKind string
	ResourceID   ids.ID
	// Details is the object of schema audit_details.v1 (SEC-AUD-5).
	Details map[string]any
	// CorrelationID is the trace ID of the request as UUID (not a UUIDv7); zero for none.
	CorrelationID ids.ID
	// PrevHash is the hash of row Seq - 1, ZeroHash for row 1.
	PrevHash [HashSize]byte
	// Hash is ChainHash(PrevHash, CanonicalRow(row)).
	Hash [HashSize]byte
}

// ErrOutOfDomain is wrapped by every error of CanonicalRow: the row holds a value the canonical
// form does not represent (a float, a null, a non-ASCII key, an integer beyond 2^53 - 1, a NUL
// character, invalid UTF-8, a time below millisecond precision).
var ErrOutOfDomain = errors.New("audit: value outside the canonical form")

// timeLayout is the canonical time form of SEC-AUD-4 and the API schema Timestamp.
const timeLayout = "2006-01-02T15:04:05.000Z"

// FormatTime returns t in the canonical form: RFC 3339 in UTC with exactly three fractional
// digits and "Z", for example "2026-10-12T08:00:00.120Z". Digits below the millisecond are cut.
func FormatTime(t time.Time) string {
	return TruncateTime(t).Format(timeLayout)
}

// TruncateTime returns t in UTC, cut to the millisecond. The recorder stores and hashes only
// such times, so the round trip through timestamptz is exact.
func TruncateTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Millisecond)
}

// ParseTime parses the canonical time form; it is the inverse of FormatTime.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil || t.Format(timeLayout) != s {
		return time.Time{}, fmt.Errorf("%w: time is not in the form 2006-01-02T15:04:05.000Z", ErrOutOfDomain)
	}
	return t, nil
}

// CanonicalRow returns the canonical form of r, the input of its hash (SEC-AUD-4): a JSON object
// with the column names except hash; seq as integer; id, the actor ids, resource_id and
// correlation_id as lower-case hyphenated UUIDs; occurred_at as FormatTime; prev_hash as
// lower-case hex; details as stored; NULL columns omitted. Serialisation follows RFC 8785 in the
// restricted domain of the audit log: keys sorted (all keys are ASCII, so byte order equals the
// UTF-16 order the RFC asks for), strings escape only '"', '\' and characters below U+0020 (\b \t
// \n \f \r, otherwise \u00xx in lower-case hex), integers in decimal, no floats and no null. The
// API representation AuditEvent without hash is exactly this object, so an export verifies
// offline (M1-50).
//
// Errors wrap ErrOutOfDomain: Seq outside 1 .. 2^53 - 1, an empty action or decision, a time
// with digits below the millisecond or outside the years 0000 to 9999, nil details, and every
// value of details or a column the form does not represent.
//
// Example:
//
//	c, err := domain.CanonicalRow(row)
//	hash := domain.ChainHash(row.PrevHash, c)
func CanonicalRow(r Row) ([]byte, error) {
	if r.Seq < 1 || r.Seq > MaxSafeInteger {
		return nil, fmt.Errorf("%w: seq %d outside 1 .. 2^53 - 1", ErrOutOfDomain, r.Seq)
	}
	if r.Action == "" || r.Decision == "" {
		return nil, fmt.Errorf("%w: action and decision are required", ErrOutOfDomain)
	}
	if r.Details == nil {
		return nil, fmt.Errorf("%w: details is required", ErrOutOfDomain)
	}
	at := r.OccurredAt.UTC()
	if at.Year() < 0 || at.Year() > 9999 || !at.Equal(TruncateTime(at)) {
		return nil, fmt.Errorf("%w: occurred_at needs millisecond precision within the years 0000 to 9999", ErrOutOfDomain)
	}
	obj := map[string]any{
		"seq":         r.Seq,
		"id":          r.ID.String(),
		"occurred_at": FormatTime(at),
		"action":      r.Action,
		"decision":    r.Decision,
		"details":     r.Details,
		"prev_hash":   hex.EncodeToString(r.PrevHash[:]),
	}
	optionalID(obj, "actor_principal_id", r.ActorPrincipalID)
	optionalID(obj, "actor_device_id", r.ActorDeviceID)
	optionalID(obj, "actor_session_id", r.ActorSessionID)
	optionalID(obj, "resource_id", r.ResourceID)
	optionalID(obj, "correlation_id", r.CorrelationID)
	optionalString(obj, "deny_reason", r.DenyReason)
	optionalString(obj, "resource_kind", r.ResourceKind)
	return AppendCanonical(nil, obj)
}

// optionalID sets obj[key] to the UUID text of id unless id is zero (NULL).
func optionalID(obj map[string]any, key string, id ids.ID) {
	if !id.IsZero() {
		obj[key] = id.String()
	}
}

// optionalString sets obj[key] to s unless s is empty (NULL).
func optionalString(obj map[string]any, key, s string) {
	if s != "" {
		obj[key] = s
	}
}

// ChainHash returns SHA-256(prev || canonical), the hash of a row whose predecessor has the
// hash prev (ADR 0021).
func ChainHash(prev [HashSize]byte, canonical []byte) [HashSize]byte {
	h := sha256.New()
	h.Write(prev[:])
	h.Write(canonical)
	var out [HashSize]byte
	h.Sum(out[:0])
	return out
}

// AppendCanonical appends the RFC 8785 serialisation of v to dst in the restricted domain of
// CanonicalRow: map[string]any with ASCII keys, []any, string, bool and integers (int, int64,
// json.Number) within 2^53 - 1. Every other value is an error wrapping ErrOutOfDomain.
func AppendCanonical(dst []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case map[string]any:
		return appendObject(dst, x)
	case []any:
		dst = append(dst, '[')
		for i, e := range x {
			if i > 0 {
				dst = append(dst, ',')
			}
			var err error
			if dst, err = AppendCanonical(dst, e); err != nil {
				return nil, err
			}
		}
		return append(dst, ']'), nil
	case string:
		return appendString(dst, x)
	case bool:
		return strconv.AppendBool(dst, x), nil
	case int:
		return appendInt(dst, int64(x))
	case int64:
		return appendInt(dst, x)
	case json.Number:
		n, err := parseInteger(string(x))
		if err != nil {
			return nil, err
		}
		return appendInt(dst, n)
	case nil:
		return nil, fmt.Errorf("%w: null", ErrOutOfDomain)
	default:
		return nil, fmt.Errorf("%w: value of type %T", ErrOutOfDomain, v)
	}
}

// appendObject appends m with its keys in ascending byte order.
func appendObject(dst []byte, m map[string]any) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		for i := range len(k) {
			if k[i] >= utf8.RuneSelf {
				return nil, fmt.Errorf("%w: non-ASCII object key", ErrOutOfDomain)
			}
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)
	dst = append(dst, '{')
	for i, k := range keys {
		if i > 0 {
			dst = append(dst, ',')
		}
		var err error
		if dst, err = appendString(dst, k); err != nil {
			return nil, err
		}
		dst = append(dst, ':')
		if dst, err = AppendCanonical(dst, m[k]); err != nil {
			return nil, err
		}
	}
	return append(dst, '}'), nil
}

// shortEscapes are the two-character escapes of RFC 8785 (ECMAScript JSON.stringify).
var shortEscapes = map[byte]string{'\b': `\b`, '\t': `\t`, '\n': `\n`, '\f': `\f`, '\r': `\r`, '"': `\"`, '\\': `\\`}

// appendString appends s as a JSON string: '"', '\' and characters below U+0020 escaped,
// everything else literal UTF-8. NUL is refused, because neither text nor jsonb of PostgreSQL
// can hold it.
func appendString(dst []byte, s string) ([]byte, error) {
	if !utf8.ValidString(s) {
		return nil, fmt.Errorf("%w: invalid UTF-8", ErrOutOfDomain)
	}
	dst = append(dst, '"')
	for i := range len(s) {
		c := s[i]
		switch {
		case c == 0:
			return nil, fmt.Errorf("%w: NUL character", ErrOutOfDomain)
		case shortEscapes[c] != "":
			dst = append(dst, shortEscapes[c]...)
		case c < 0x20:
			dst = append(dst, `\u00`...)
			dst = append(dst, "0123456789abcdef"[c>>4], "0123456789abcdef"[c&0xf])
		default:
			dst = append(dst, c)
		}
	}
	return append(dst, '"'), nil
}

// appendInt appends n in decimal after checking the safe range.
func appendInt(dst []byte, n int64) ([]byte, error) {
	if n > MaxSafeInteger || n < -MaxSafeInteger {
		return nil, fmt.Errorf("%w: integer beyond 2^53 - 1", ErrOutOfDomain)
	}
	return strconv.AppendInt(dst, n, 10), nil
}

// parseInteger reads a JSON number that must be an integer without fraction, exponent or "-0".
func parseInteger(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || strconv.FormatInt(n, 10) != s {
		return 0, fmt.Errorf("%w: number is not an integer in canonical decimal form", ErrOutOfDomain)
	}
	return n, nil
}

// ParseUUID parses a UUID of any version in the canonical lower-case form 8-4-4-4-12. The
// correlation ID is such a UUID but no UUIDv7 (02 "Cross-cutting ports (`internal/kernel`)"), so kernel/ids.Parse,
// which accepts only version 7, does not fit it.
func ParseUUID(s string) (ids.ID, error) {
	var id ids.ID
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return id, fmt.Errorf("%w: not a UUID in the form 8-4-4-4-12", ErrOutOfDomain)
	}
	compact := s[:8] + s[9:13] + s[14:18] + s[19:23] + s[24:]
	b, err := hex.DecodeString(compact)
	if err != nil || hex.EncodeToString(b) != compact {
		return id, fmt.Errorf("%w: not a lower-case hex UUID", ErrOutOfDomain)
	}
	copy(id[:], b)
	return id, nil
}
