// SPDX-License-Identifier: Apache-2.0

// Package fracindex computes fractional index keys for orders that change one row per move
// (docs/spec/03-datenmodell.md Grundsatz 5, docs/DECISIONS.md M1-52): the widely used base-62
// scheme with an integer part. Digits are 0-9A-Za-z in ASCII order, so the byte order of two keys
// is their order (columns with COLLATE "C"). A key is an integer part, a head letter followed by
// its digits (a to z: 1 to 26 digits, A to Z: 26 to 1 digits for the negative range), plus an
// optional fraction that never ends with 0. The smallest integer, A followed by 26 zeros, is not
// a key by itself (with a fraction it is), so there is always room before every key.
//
// Example:
//
//	first, _ := fracindex.KeyBetween("", "")     // "a0"
//	next, _ := fracindex.KeyBetween(first, "")   // "a1"
//	mid, _ := fracindex.KeyBetween(first, next)  // "a0V"
package fracindex

import (
	"errors"
	"strings"
)

// digits are the base-62 digits in ASCII order.
const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// smallestInteger is the integer part below every key; it is itself invalid.
var smallestInteger = "A" + strings.Repeat("0", 26)

// ErrInvalidKey reports a key outside the grammar of the package doc.
var ErrInvalidKey = errors.New("fracindex: invalid key")

// ErrOrder reports a pair whose lower bound is not below its upper bound.
var ErrOrder = errors.New("fracindex: the lower key is not below the upper key")

// ErrExhausted reports that no key exists beyond the end of the integer range; it needs 26
// integer digits on one side and does not happen in practice.
var ErrExhausted = errors.New("fracindex: the key range is exhausted")

// Valid reports whether key is a key of the grammar: a valid integer part, only base-62 digits,
// no fraction ending in 0, not the smallest integer.
func Valid(key string) bool {
	_, err := split(key)
	return err == nil
}

// KeyBetween returns a key strictly between a and b; "" means an open end. Both empty give "a0";
// with b empty the integer part of a grows (or, when it cannot, a fraction is appended); with a
// empty the integer part of b shrinks likewise; otherwise the shared prefix plus the digit
// midpoint. Errors: ErrInvalidKey for an invalid bound, ErrOrder for a >= b, ErrExhausted.
func KeyBetween(a, b string) (string, error) {
	ia, err := bound(a)
	if err != nil {
		return "", err
	}
	ib, err := bound(b)
	if err != nil {
		return "", err
	}
	switch {
	case a != "" && b != "" && a >= b:
		return "", ErrOrder
	case a == "" && b == "":
		return "a0", nil
	case a == "":
		return before(b, ib)
	case b == "":
		if i, ok := increment(ia); ok {
			return i, nil
		}
		return ia + midpoint(a[len(ia):], ""), nil
	case ia == ib:
		return ia + midpoint(a[len(ia):], b[len(ib):]), nil
	}
	i, ok := increment(ia)
	if !ok {
		return "", ErrExhausted
	}
	if i < b {
		return i, nil
	}
	return ia + midpoint(a[len(ia):], ""), nil
}

// bound checks a bound of KeyBetween and returns its integer part; "" is an open end.
func bound(key string) (string, error) {
	if key == "" {
		return "", nil
	}
	return split(key)
}

// before returns a key below b with the integer part ib.
func before(b, ib string) (string, error) {
	switch {
	case ib == smallestInteger:
		return ib + midpoint("", b[len(ib):]), nil
	case ib < b:
		return ib, nil
	}
	d, ok := decrement(ib)
	if !ok {
		return "", ErrExhausted
	}
	return d, nil
}

// integerLength returns the length of the integer part with head h, or 0 for no head letter.
func integerLength(h byte) int {
	switch {
	case h >= 'a' && h <= 'z':
		return int(h-'a') + 2
	case h >= 'A' && h <= 'Z':
		return int('Z'-h) + 2
	}
	return 0
}

// split checks key and returns its integer part.
func split(key string) (string, error) {
	if key == "" {
		return "", ErrInvalidKey
	}
	n := integerLength(key[0])
	if n == 0 || n > len(key) {
		return "", ErrInvalidKey
	}
	for i := 1; i < len(key); i++ {
		if strings.IndexByte(digits, key[i]) < 0 {
			return "", ErrInvalidKey
		}
	}
	if key == smallestInteger || (len(key) > n && key[len(key)-1] == '0') {
		return "", ErrInvalidKey
	}
	return key[:n], nil
}

// midpoint returns a fraction strictly between the fractions a and b; b "" means no upper bound.
// Neither ends with 0, and a < b when b is given (callers guarantee both).
func midpoint(a, b string) string {
	if b != "" {
		n := 0
		for n < len(b) && digitAt(a, n) == b[n] {
			n++
		}
		if n > 0 {
			return b[:n] + midpoint(tail(a, n), b[n:])
		}
	}
	da := 0
	if a != "" {
		da = strings.IndexByte(digits, a[0])
	}
	db := len(digits)
	if b != "" {
		db = strings.IndexByte(digits, b[0])
	}
	if db-da > 1 {
		return string(digits[(da+db+1)/2])
	}
	if len(b) > 1 {
		return b[:1]
	}
	return string(digits[da]) + midpoint(tail(a, 1), "")
}

// digitAt returns the digit at i of s, 0 beyond its end.
func digitAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return '0'
}

// tail returns s from n on, "" beyond its end.
func tail(s string, n int) string {
	if n >= len(s) {
		return ""
	}
	return s[n:]
}

// increment returns the next integer after x; false at the end of the range.
func increment(x string) (string, bool) {
	head, ds := x[0], []byte(x[1:])
	carry := true
	for i := len(ds) - 1; carry && i >= 0; i-- {
		d := strings.IndexByte(digits, ds[i]) + 1
		if d == len(digits) {
			ds[i] = '0'
		} else {
			ds[i], carry = digits[d], false
		}
	}
	if !carry {
		return string(head) + string(ds), true
	}
	switch head {
	case 'Z':
		return "a0", true
	case 'z':
		return "", false
	}
	h := head + 1
	if h > 'a' {
		ds = append(ds, '0')
	} else {
		ds = ds[:len(ds)-1]
	}
	return string(h) + string(ds), true
}

// decrement returns the integer before x; false at the start of the range.
func decrement(x string) (string, bool) {
	head, ds := x[0], []byte(x[1:])
	borrow := true
	for i := len(ds) - 1; borrow && i >= 0; i-- {
		d := strings.IndexByte(digits, ds[i]) - 1
		if d == -1 {
			ds[i] = digits[len(digits)-1]
		} else {
			ds[i], borrow = digits[d], false
		}
	}
	if !borrow {
		return string(head) + string(ds), true
	}
	switch head {
	case 'a':
		return "Z" + digits[len(digits)-1:], true
	case 'A':
		return "", false
	}
	h := head - 1
	if h < 'Z' {
		ds = append(ds, digits[len(digits)-1])
	} else {
		ds = ds[:len(ds)-1]
	}
	return string(h) + string(ds), true
}
