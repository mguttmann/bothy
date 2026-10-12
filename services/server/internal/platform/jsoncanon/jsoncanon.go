// SPDX-License-Identifier: Apache-2.0

package jsoncanon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/gowebpki/jcs"
)

// ErrNoCanonicalForm reports an input that has no canonical form: it is not exactly one JSON
// value in valid UTF-8, an object repeats a member name, or an integer literal is not exactly a
// IEEE 754 double.
var ErrNoCanonicalForm = errors.New("jsoncanon: the input has no canonical JSON form")

// Canonicalize returns the RFC 8785 serialization of the JSON value data: members sorted by
// their UTF-16 code units, no insignificant whitespace, strings with the minimal escapes and
// numbers in the ECMAScript form of their IEEE 754 double value.
//
// RFC 8785 rounds every number to a double. For an integer literal (no fraction, no exponent)
// that would map two different values to one form (2^64 - 1 and 2^64 - 2), so such a literal is
// refused instead; the API carries no integer beyond 2^53 - 1 anyway (ADR 0053). Literals with a fraction or an exponent are rounded as RFC 8785
// says.
//
// The error wraps ErrNoCanonicalForm and never repeats the input.
func Canonicalize(data []byte) ([]byte, error) {
	if err := exactIntegers(data); err != nil {
		return nil, err
	}
	out, err := jcs.Transform(data)
	if err != nil {
		// The text of jcs may quote parts of the input; it is not kept.
		return nil, fmt.Errorf("%w: not one valid JSON value", ErrNoCanonicalForm)
	}
	return out, nil
}

// exactIntegers walks the tokens of data and refuses an integer literal that a double cannot hold
// exactly. Syntax errors are left to jcs, which reports them as well.
func exactIntegers(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: not one valid JSON value", ErrNoCanonicalForm)
		}
		n, ok := tok.(json.Number)
		if !ok || strings.ContainsAny(string(n), ".eE") {
			continue
		}
		if !exactDouble(string(n)) {
			return fmt.Errorf("%w: an integer beyond the exact range of a double", ErrNoCanonicalForm)
		}
	}
}

// maxExactDigits is the length of the longest integer literal a double can hold exactly: the
// largest finite double has 309 digits, plus a sign. A longer literal is refused before it is
// parsed, so a body of digits costs no big-number arithmetic.
const maxExactDigits = 310

// exactDouble reports whether the integer literal lit equals its double value.
func exactDouble(lit string) bool {
	if len(lit) > maxExactDigits {
		return false
	}
	i, ok := new(big.Int).SetString(lit, 10)
	if !ok {
		return false
	}
	f, acc := new(big.Float).SetInt(i).Float64()
	if acc != big.Exact {
		return false
	}
	back, _ := big.NewFloat(f).Int(nil)
	return back.Cmp(i) == 0
}
