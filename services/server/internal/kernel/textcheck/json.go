// SPDX-License-Identifier: Apache-2.0

package textcheck

// StorableJSON reports whether the JSON text raw is free of the escapes PostgreSQL refuses in
// jsonb: \u0000 and a \u escape of a surrogate that is not part of a pair. encoding/json turns
// them into NUL and U+FFFD and lets them through, so without this rule a document that passes
// every other check fails in the database with a 500 (SEC-IN-4, SEC-T06; D-M2-24-21, D-M2-15-27).
// raw is valid JSON when it is called.
func StorableJSON(raw []byte) bool {
	sc := escapeScan{}
	for i := 0; i < len(raw); i++ {
		next, ok := sc.step(raw, i)
		if !ok {
			return false
		}
		i = next
	}
	return !sc.pendingHigh
}

// escapeScan is the state of StorableJSON: inside a string or not, and whether the last escape
// was a high surrogate that waits for its low half.
type escapeScan struct {
	inString, pendingHigh bool
}

// step reads the byte at i of raw and returns the index of the last byte it consumed, and false
// when the text is not storable.
func (sc *escapeScan) step(raw []byte, i int) (int, bool) {
	b := raw[i]
	switch {
	case !sc.inString:
		sc.inString = b == '"'
		return i, true
	case b != '\\':
		sc.inString = b != '"'
		return i, !sc.pendingHigh
	case i+1 >= len(raw) || raw[i+1] != 'u':
		return i + 1, !sc.pendingHigh
	}
	cu, ok := hex4(raw[i+2:])
	return i + 5, ok && sc.unit(cu)
}

// unit takes the code unit cu of one \u escape and reports whether the text is still storable.
func (sc *escapeScan) unit(cu uint16) bool {
	switch {
	case cu == 0:
		return false
	case cu >= 0xDC00 && cu <= 0xDFFF:
		ok := sc.pendingHigh
		sc.pendingHigh = false
		return ok
	case sc.pendingHigh:
		return false
	}
	sc.pendingHigh = cu >= 0xD800 && cu <= 0xDBFF
	return true
}

// hex4 parses the four hex digits at the start of b.
func hex4(b []byte) (uint16, bool) {
	if len(b) < 4 {
		return 0, false
	}
	var v uint16
	for _, d := range b[:4] {
		switch {
		case d >= '0' && d <= '9':
			d -= '0'
		case d >= 'a' && d <= 'f':
			d -= 'a' - 10
		case d >= 'A' && d <= 'F':
			d -= 'A' - 10
		default:
			return 0, false
		}
		v = v<<4 | uint16(d)
	}
	return v, true
}
