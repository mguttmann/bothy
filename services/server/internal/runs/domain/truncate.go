// SPDX-License-Identifier: Apache-2.0

package domain

// The cuts of agent output before it is stored (09 5.3): the text excerpt to
// run.excerpt_max_chars runes and a step detail to run.step_detail_max_bytes bytes of canonical
// JSON.

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// truncatedOnly is the stored detail when no cut of the string leaves makes a detail fit.
const truncatedOnly = `{"truncated":true}`

// CutExcerpt returns the last maxRunes runes of s (all of s when it is shorter). It keeps the
// tail because a draft grows at the end, so the newest text stays visible. maxRunes is
// run.excerpt_max_chars; 0 or less gives "".
//
// Example:
//
//	domain.CutExcerpt("Hello Alex", 4) // "Alex"
func CutExcerpt(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	skip := utf8.RuneCountInString(s) - maxRunes
	i := 0
	for ; skip > 0; skip-- {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return s[i:]
}

// leaf is one string value inside a detail, addressable for the cut.
type leaf struct {
	get func() string
	set func(string)
}

// CutDetail returns the canonical JSON of the object detail (members sorted, no HTML escaping)
// when it has at most maxBytes bytes. Otherwise it sets the top-level member "truncated" to true
// and shortens the string values, the longest first (ties: the first in the canonical order of
// their pointers), each at a rune boundary, until the encoding fits; if it still does not fit
// (for example because of many members or numbers), the result is {"truncated":true}. A detail
// that is not a JSON object also gives {"truncated":true}. maxBytes is run.step_detail_max_bytes.
//
// The result is a valid JSON object of at most maxBytes bytes (given maxBytes is at least the
// length of {"truncated":true}) and depends only on its input.
//
// Example:
//
//	domain.CutDetail([]byte(`{"log":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), 32)
//	// {"log":"aaaaa","truncated":true}
func CutDetail(detail json.RawMessage, maxBytes int) json.RawMessage {
	v, err := decodeJSON(detail)
	obj, isObject := v.(map[string]any)
	if err != nil || !isObject {
		return json.RawMessage(truncatedOnly)
	}
	// Numbers are measured and stored in the plain decimal form a jsonb numeric prints, so the
	// limit bounds every read; numbers cannot be cut, so if their plain forms alone exceed the
	// limit, only the fallback fits.
	if budget := maxBytes; !plainNumbers(obj, &budget) {
		return json.RawMessage(truncatedOnly)
	}
	enc := encodeCanonical(obj)
	if len(enc) <= maxBytes {
		return enc
	}
	obj["truncated"] = true
	var leaves []leaf
	collectLeaves(obj, &leaves)
	// Longest first; the stable sort keeps the canonical order among equal lengths.
	slices.SortStableFunc(leaves, func(x, y leaf) int { return len(y.get()) - len(x.get()) })
	total := len(encodeCanonical(obj))
	for _, l := range leaves {
		excess := total - maxBytes
		if excess <= 0 {
			break
		}
		s := l.get()
		// Each removed byte removes at least one encoded byte (escapes only grow a byte), so
		// cutting excess bytes fits unless the string is shorter than excess.
		keep := max(len(s)-excess, 0)
		for keep > 0 && !utf8.RuneStart(s[keep]) {
			keep--
		}
		before := len(encodeCanonical(s))
		l.set(s[:keep])
		total -= before - len(encodeCanonical(s[:keep]))
	}
	if total > maxBytes {
		return json.RawMessage(truncatedOnly)
	}
	enc = encodeCanonical(obj)
	if len(enc) > maxBytes {
		return json.RawMessage(truncatedOnly)
	}
	return enc
}

// collectLeaves appends the string values below v in canonical order (members sorted, array
// elements in order).
func collectLeaves(v any, out *[]leaf) {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(x) {
			if _, ok := x[k].(string); ok {
				*out = append(*out, leaf{get: func() string { return x[k].(string) }, set: func(n string) { x[k] = n }})
				continue
			}
			collectLeaves(x[k], out)
		}
	case []any:
		for i := range x {
			if _, ok := x[i].(string); ok {
				*out = append(*out, leaf{get: func() string { return x[i].(string) }, set: func(n string) { x[i] = n }})
				continue
			}
			collectLeaves(x[i], out)
		}
	}
}

// plainNumbers replaces every json.Number below v by its plainDecimal form and subtracts each
// form's length from budget. It returns false, leaving v partly replaced, as soon as a number has
// no bounded plain form or the budget is exhausted, so a literal like 1e999999999 is never
// expanded.
func plainNumbers(v any, budget *int) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if !plainChild(e, func(n json.Number) { x[k] = n }, budget) {
				return false
			}
		}
	case []any:
		for i, e := range x {
			if !plainChild(e, func(n json.Number) { x[i] = n }, budget) {
				return false
			}
		}
	}
	return true
}

// plainChild handles one member or element e of plainNumbers: a number is replaced through set,
// anything else is walked.
func plainChild(e any, set func(json.Number), budget *int) bool {
	n, isNumber := e.(json.Number)
	if !isNumber {
		return plainNumbers(e, budget)
	}
	p, ok := plainDecimal(string(n), *budget)
	if !ok {
		return false
	}
	*budget -= len(p)
	set(json.Number(p))
	return true
}

// plainDecimal returns the JSON number literal lit (as the decoder accepted it) in the form
// PostgreSQL prints the numeric it parses: no exponent, the scale of the literal (fraction
// digits minus exponent, at least 0), no sign on zero. It returns false when that form would be
// longer than maxLen bytes, before building it.
//
// Example:
//
//	domain.plainDecimal("1.5e-3", 100) // "0.0015", true
func plainDecimal(lit string, maxLen int) (string, bool) {
	neg := strings.HasPrefix(lit, "-")
	lit = strings.TrimPrefix(lit, "-")
	mant, expPart, hasExp := strings.Cut(strings.ToLower(lit), "e")
	exp := 0
	if hasExp {
		e, err := strconv.Atoi(expPart)
		if err != nil || e > 1<<30 || e < -(1<<30) {
			return "", false
		}
		exp = e
	}
	intDigits, frac, _ := strings.Cut(mant, ".")
	digits := intDigits + frac
	point := len(intDigits) + exp // position of the decimal point within digits
	scale := max(len(digits)-point, 0)
	intLen := max(point, 1)
	if 1+intLen+1+scale > maxLen {
		return "", false
	}
	var ip string
	switch {
	case point <= 0:
		ip = "0"
	case point >= len(digits):
		ip = digits + strings.Repeat("0", point-len(digits))
	default:
		ip = digits[:point]
	}
	if ip = strings.TrimLeft(ip, "0"); ip == "" {
		ip = "0"
	}
	var fp string
	if scale > 0 {
		if point < 0 {
			fp = strings.Repeat("0", -point) + digits
		} else {
			fp = digits[point:]
		}
	}
	out := ip
	if fp != "" {
		out += "." + fp
	}
	if neg && strings.Trim(digits, "0") != "" {
		out = "-" + out
	}
	return out, true
}
