// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"math"
	"strings"
)

// RangeKind is the outcome of ParseRange.
type RangeKind uint8

// The values of RangeKind; the zero value is no outcome.
const (
	// RangeFull serves the whole variant with 200.
	RangeFull RangeKind = iota + 1
	// RangePartial serves RangeResult.First to RangeResult.Last with 206.
	RangePartial
	// RangeUnsatisfiable answers 416 range_not_satisfiable.
	RangeUnsatisfiable
)

// RangeResult is the decision of ParseRange. First and Last are the inclusive byte positions of
// a RangePartial result and zero for the other kinds.
type RangeResult struct {
	Kind        RangeKind
	First, Last int64
}

// ParseRange decides how a download of a variant of size bytes answers its Range and If-Range
// headers (RFC 9110 section 14; 12 "9.3"). etag is the current ETag of the variant.
//
//   - No Range header, a unit other than bytes (compared case-insensitively), more than one range
//     and every syntax error give RangeFull: the header is ignored.
//   - One range first-last, first- or -suffix gives RangePartial; Last is capped at size-1, and a
//     suffix longer than the variant means the whole variant. White space around the range is
//     allowed, inside it is not.
//   - A range whose first byte is at or after size, and a zero suffix, give RangeUnsatisfiable.
//   - A non-empty ifRange lets the range apply only when it equals etag exactly and etag is a
//     strong ETag; any other value, an HTTP date included, gives RangeFull, because the server
//     sends no Last-Modified.
//
// A RangePartial result always satisfies 0 <= First <= Last < size.
//
// Example:
//
//	r := domain.ParseRange("bytes=-10", "", etag, 1000) // {RangePartial, 990, 999}
func ParseRange(header, ifRange, etag string, size int64) RangeResult {
	full := RangeResult{Kind: RangeFull}
	if header == "" || (ifRange != "" && (ifRange != etag || !strongETag(etag))) {
		return full
	}
	unit, set, ok := strings.Cut(header, "=")
	if !ok || !strings.EqualFold(unit, "bytes") || strings.Contains(set, ",") {
		return full
	}
	return parseSpec(strings.Trim(set, " \t"), size)
}

// parseSpec decides one range spec of ParseRange, already without the white space around it.
func parseSpec(spec string, size int64) RangeResult {
	full := RangeResult{Kind: RangeFull}
	firstText, lastText, ok := strings.Cut(spec, "-")
	if !ok {
		return full
	}
	if firstText == "" {
		suffix, ok := parseDigits(lastText)
		if !ok {
			return full
		}
		if suffix == 0 || size <= 0 {
			return RangeResult{Kind: RangeUnsatisfiable}
		}
		return RangeResult{Kind: RangePartial, First: size - min(suffix, size), Last: size - 1}
	}
	first, ok := parseDigits(firstText)
	if !ok {
		return full
	}
	last := int64(math.MaxInt64)
	if lastText != "" {
		if last, ok = parseDigits(lastText); !ok || last < first {
			return full
		}
	}
	if first >= size {
		return RangeResult{Kind: RangeUnsatisfiable}
	}
	return RangeResult{Kind: RangePartial, First: first, Last: min(last, size-1)}
}

// strongETag reports whether etag is a strong entity tag: a quoted string, not W/"...".
func strongETag(etag string) bool {
	return len(etag) >= 2 && etag[0] == '"' && etag[len(etag)-1] == '"'
}

// parseDigits parses a non-empty string of ASCII digits; ok is false for any other character,
// signs included, and for a value beyond int64.
func parseDigits(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	var n int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		d := int64(c - '0')
		if n > (math.MaxInt64-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	return n, true
}
