// SPDX-License-Identifier: Apache-2.0

package fingerprint

// alphabet is the Crockford base32 alphabet (https://www.crockford.com/base32.html): digits and
// upper-case letters without I, L, O and U, so that a fingerprint read aloud or typed has no
// ambiguous characters.
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Encode returns b in Crockford base32: 5-bit groups from the most significant bit, the last
// group padded with zero bits on the right, upper case, no padding character and no check
// symbol (docs/spec/24-terminal.md 4.3). An empty input gives an empty string.
//
// Example: Encode([]byte("foobar")) is "CSQPYRK1E8".
func Encode(b []byte) string {
	out := make([]byte, 0, (len(b)*8+4)/5)
	var acc uint
	bits := 0
	for _, c := range b {
		acc = acc<<8 | uint(c)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out = append(out, alphabet[(acc>>uint(bits))&0x1f])
		}
	}
	if bits > 0 {
		out = append(out, alphabet[(acc<<uint(5-bits))&0x1f])
	}
	return string(out)
}
