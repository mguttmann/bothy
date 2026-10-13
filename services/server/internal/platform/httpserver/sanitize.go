// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

// maxSafeInteger is 2^53 - 1, the largest integer JavaScript represents exactly (06
// "Basic rules", row Integers).
const maxSafeInteger = 1<<53 - 1

// errorTypeNullMember is error.type of the log record of a repaired response.
const errorTypeNullMember = "null_member"

// The errors of SanitizeJSON; each refuses the whole response.
var (
	// ErrNullElement is a null array element or a null top-level value, which has no member to drop.
	ErrNullElement = errors.New("httpserver: JSON null outside an object member")
	// ErrUnsafeInteger is an integer whose absolute value exceeds 2^53 - 1.
	ErrUnsafeInteger = errors.New("httpserver: integer beyond 2^53 - 1")
	// ErrInvalidJSON is a body that is not exactly one JSON value.
	ErrInvalidJSON = errors.New("httpserver: response body is not one JSON value")
)

// sanitizeErrorType maps an error of SanitizeJSON to error.type of its log record.
func sanitizeErrorType(err error) string {
	switch {
	case errors.Is(err, ErrNullElement):
		return "null_element"
	case errors.Is(err, ErrUnsafeInteger):
		return "unsafe_integer"
	}
	return "invalid_json"
}

// SanitizeJSON enforces ADR 0053 and the integer rule of 06 on a response body. An object member
// whose value is null is removed, and removed counts them; the body is then re-encoded compactly
// (numbers keep their text). Without a removal the result is body itself, byte for byte. A null
// array element or top-level null (ErrNullElement), an integer beyond 2^53 - 1 (ErrUnsafeInteger)
// and anything that is not exactly one JSON value (ErrInvalidJSON) are errors: the response
// writer then sends 500 internal_error instead. The repair is a safety net; tests fail any
// handler that needs it.
//
// Example:
//
//	out, removed, err := httpserver.SanitizeJSON([]byte(`{"a":1,"b":null}`))
//	// out = {"a":1}, removed = 1, err = nil
func SanitizeJSON(body []byte) (out []byte, removed int, err error) {
	s := &sanitizer{dec: json.NewDecoder(bytes.NewReader(body))}
	s.dec.UseNumber()
	tok, err := s.dec.Token()
	if err != nil {
		return nil, 0, ErrInvalidJSON
	}
	if err := s.value(tok); err != nil {
		return nil, 0, err
	}
	if _, err := s.dec.Token(); !errors.Is(err, io.EOF) {
		return nil, 0, ErrInvalidJSON
	}
	if s.removed == 0 {
		return body, 0, nil
	}
	return s.out.Bytes(), s.removed, nil
}

// sanitizer re-encodes the token stream of one body.
type sanitizer struct {
	dec     *json.Decoder
	out     bytes.Buffer
	removed int
}

// value writes the value that starts with tok.
func (s *sanitizer) value(tok json.Token) error {
	switch t := tok.(type) {
	case json.Delim:
		if t == '{' {
			return s.object()
		}
		return s.array()
	case nil:
		return ErrNullElement
	case json.Number:
		if !safeNumber(t.String()) {
			return ErrUnsafeInteger
		}
		s.out.WriteString(t.String())
	case string:
		s.writeString(t)
	case bool:
		s.out.WriteString(strconv.FormatBool(t))
	}
	return nil
}

// object writes an object without its null members; the opening brace is consumed.
func (s *sanitizer) object() error {
	s.out.WriteByte('{')
	first := true
	for s.dec.More() {
		key, err := s.dec.Token()
		if err != nil {
			return ErrInvalidJSON
		}
		val, err := s.dec.Token()
		if err != nil {
			return ErrInvalidJSON
		}
		if val == nil {
			s.removed++
			continue
		}
		if !first {
			s.out.WriteByte(',')
		}
		first = false
		name, _ := key.(string)
		s.writeString(name)
		s.out.WriteByte(':')
		if err := s.value(val); err != nil {
			return err
		}
	}
	if _, err := s.dec.Token(); err != nil { // the closing brace
		return ErrInvalidJSON
	}
	s.out.WriteByte('}')
	return nil
}

// array writes an array; a null element is an error. The opening bracket is consumed.
func (s *sanitizer) array() error {
	s.out.WriteByte('[')
	first := true
	for s.dec.More() {
		val, err := s.dec.Token()
		if err != nil {
			return ErrInvalidJSON
		}
		if !first {
			s.out.WriteByte(',')
		}
		first = false
		if err := s.value(val); err != nil {
			return err
		}
	}
	if _, err := s.dec.Token(); err != nil { // the closing bracket
		return ErrInvalidJSON
	}
	s.out.WriteByte(']')
	return nil
}

// writeString writes str as a JSON string without HTML escaping, as the generated handlers do not
// rely on it and the body is never HTML (nosniff).
func (s *sanitizer) writeString(str string) {
	enc := json.NewEncoder(&s.out)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(str)
	s.out.Truncate(s.out.Len() - 1) // Encode appends a newline
}

// safeNumber reports whether a JSON number is a non-integer or an integer within ±(2^53 - 1).
// Numbers with a fraction or an exponent are not integers by syntax and pass.
func safeNumber(text string) bool {
	if strings.ContainsAny(text, ".eE") {
		return true
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return false // beyond int64, so beyond 2^53 too
	}
	return n >= -maxSafeInteger && n <= maxSafeInteger
}
