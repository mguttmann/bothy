// SPDX-License-Identifier: Apache-2.0

package testkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// JSONNull returns the JSON pointer (RFC 6901, "" for a value itself) of the first null in the
// JSON text b, object members in the order of their names. b holds one JSON value or a sequence of
// them (NDJSON); empty text holds none. It decodes b and walks the values, so the letters null
// inside a string (a random base64url token, a name) are no null: a substring check on the text
// failed correct responses at random (issue #238). found is false when the values hold no null;
// err is set when b holds text that is not JSON after the values checked so far.
func JSONNull(b []byte) (ptr string, found bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	for {
		var v any
		if err := dec.Decode(&v); errors.Is(err, io.EOF) {
			return "", false, nil
		} else if err != nil {
			return "", false, err
		}
		if ptr, found := nullAt(v, ""); found {
			return ptr, true, nil
		}
	}
}

// HasJSONNull reports whether the JSON text b holds a null at any depth (ADR 0053). Text that is
// not JSON holds no JSON null; a test that needs JSON decodes the text itself or uses NoJSONNull.
func HasJSONNull(b []byte) bool {
	_, found, _ := JSONNull(b)
	return found
}

// NoJSONNull fails the test (Errorf) when the JSON text b holds a null at any depth or holds text
// that is not JSON (ADR 0053: an absent value is left out, never sent as null). what names b.
func NoJSONNull(t testing.TB, what string, b []byte) {
	t.Helper()
	ptr, found, err := JSONNull(b)
	switch {
	case found:
		t.Errorf("%s: JSON null at %s in %s", what, displayPointer(ptr), b)
	case err != nil:
		t.Errorf("%s: not JSON (%v): %s", what, err, b)
	}
}

// displayPointer names the document itself for an empty pointer, so a message never ends in "at ".
func displayPointer(ptr string) string {
	if ptr == "" {
		return "(the document)"
	}
	return ptr
}

// nullAt returns the pointer of the first null in v below ptr.
func nullAt(v any, ptr string) (string, bool) {
	switch x := v.(type) {
	case nil:
		return ptr, true
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			if p, ok := nullAt(x[k], ptr+"/"+pointerToken.Replace(k)); ok {
				return p, true
			}
		}
	case []any:
		for i, e := range x {
			if p, ok := nullAt(e, ptr+"/"+strconv.Itoa(i)); ok {
				return p, true
			}
		}
	}
	return "", false
}

// pointerToken escapes a member name as a JSON pointer token (RFC 6901 section 3).
var pointerToken = strings.NewReplacer("~", "~0", "/", "~1")
