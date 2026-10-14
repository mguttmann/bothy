// SPDX-License-Identifier: Apache-2.0

package jose

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

// MaxJSONDepth is the deepest nesting a JOSE header, payload or JWK may have (an object or array
// is one level). It equals the default of http.max_json_depth, the decoder limit of the request
// bodies (platform/httpserver, M1-18); the objects of the profile need three levels at most, so
// a token never reaches it. A constant instead of the configured value because the kernel reads
// no configuration and the signature of ParseCompactJWS is fixed (docs/DECISIONS.md M1-33).
const MaxJSONDepth = 32

// errJSON is the cause of every JSON rule violation; callers wrap it into their own error.
var errJSON = errors.New("jose: not one JSON object without duplicate members")

// checkObject reports whether b is valid UTF-8 and exactly one JSON object without a duplicate
// member name at any level and no deeper than MaxJSONDepth (SEC-IN-2).
func checkObject(b []byte) error {
	trimmed := bytes.TrimSpace(b)
	if !utf8.Valid(b) || len(trimmed) == 0 || trimmed[0] != '{' {
		return errJSON
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var w walker
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) && len(w.stack) == 0 && w.values == 1 {
			return nil
		}
		if err != nil || !w.step(tok) {
			return errJSON
		}
	}
}

// frame is one open object or array of the walker.
type frame struct {
	object    bool
	expectKey bool
	keys      map[string]struct{}
}

// walker follows the tokens of encoding/json and checks the rules of checkObject.
type walker struct {
	stack  []*frame
	values int
}

// step applies one token; false means a rule is broken.
func (w *walker) step(tok json.Token) bool {
	if len(w.stack) == 0 {
		if w.values == 1 {
			return false // a second top-level value
		}
		w.values++
	}
	d, isDelim := tok.(json.Delim)
	if isDelim && (d == '}' || d == ']') {
		w.stack = w.stack[:len(w.stack)-1]
		return true
	}
	if n := len(w.stack); n > 0 && w.stack[n-1].object {
		if top := w.stack[n-1]; top.expectKey {
			return top.key(tok)
		}
		w.stack[n-1].expectKey = true // a member value; the next token is a key again
	}
	if !isDelim {
		return true
	}
	if len(w.stack) >= MaxJSONDepth {
		return false
	}
	w.stack = append(w.stack, &frame{object: d == '{', expectKey: d == '{', keys: map[string]struct{}{}})
	return true
}

// key records a member name; false for a duplicate.
func (f *frame) key(tok json.Token) bool {
	k, _ := tok.(string)
	if _, dup := f.keys[k]; dup {
		return false
	}
	f.keys[k] = struct{}{}
	f.expectKey = false
	return true
}

// stringOf unmarshals a JSON string into dst; null and every other type are false, so an
// explicit null never reads as an absent or empty member.
func stringOf(raw json.RawMessage, dst *string) bool {
	return len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, dst) == nil
}

// decodeSegment decodes base64url without padding. Unlike encoding/base64 it refuses every byte
// outside the URL-safe alphabet, including the newlines the standard decoder skips, so one token
// has exactly one encoding.
func decodeSegment(s string) ([]byte, bool) {
	for i := 0; i < len(s); i++ {
		if !isURLSafe(s[i]) {
			return nil, false
		}
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, false
	}
	return b, true
}

// isURLSafe reports a byte of the base64url alphabet.
func isURLSafe(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// encodeSegment is base64url without padding.
func encodeSegment(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
