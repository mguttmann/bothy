// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// ErrContentDigest reports a Content-Digest header without exactly one valid SHA-256 digest.
// Errors wrapping it name the defect, never the header value.
var ErrContentDigest = errors.New("files: invalid Content-Digest")

// digestMember is the member of the RFC 9530 dictionary that carries the SHA-256 digest.
const digestMember = "sha-256"

// ParseContentDigest returns the SHA-256 digest of a Content-Digest header (RFC 9530): a
// Structured Fields dictionary (RFC 8941 section 3.2) whose member sha-256 is a byte sequence of
// exactly 32 bytes. Other members and parameters are parsed and ignored. A missing member, a
// member that is not a byte sequence, a value of another length, bad base64 and a header that is
// not a valid dictionary give an error wrapping ErrContentDigest; the caller answers
// 400 validation_failed with the pointer #header/Content-Digest (docs/DECISIONS.md D-M3-03-10).
// When the member occurs twice, the last one counts, as RFC 8941 section 4.2.2 says.
//
// Example:
//
//	sum, err := domain.ParseContentDigest("sha-256=:X48E9qOokqqrvdts8nOJRJN3OWDUoyWxBf7kbu9DBPE=:")
func ParseContentDigest(h string) ([32]byte, error) {
	p := sfParser{s: h}
	members, err := p.dictionary()
	if err != nil {
		return [32]byte{}, err
	}
	v, ok := members[digestMember]
	if !ok {
		return [32]byte{}, fmt.Errorf("%w: no member %s", ErrContentDigest, digestMember)
	}
	if !v.isBytes {
		return [32]byte{}, fmt.Errorf("%w: member %s is not a byte sequence", ErrContentDigest, digestMember)
	}
	if len(v.bytes) != 32 {
		return [32]byte{}, fmt.Errorf("%w: member %s does not hold 32 bytes", ErrContentDigest, digestMember)
	}
	var out [32]byte
	copy(out[:], v.bytes)
	return out, nil
}

// sfValue is a parsed dictionary member: a byte sequence or any other value.
type sfValue struct {
	isBytes bool
	bytes   []byte
}

// sfParser is a parser of the RFC 8941 dictionary syntax, enough to find one byte sequence
// member: keys, bare items (integer, decimal, string, token, byte sequence, boolean, and the
// date and display string of RFC 9651), inner lists and parameters.
type sfParser struct {
	s string
	i int
}

// fail returns an error at the current position.
func (p *sfParser) fail(what string) error {
	return fmt.Errorf("%w: %s at byte %d", ErrContentDigest, what, p.i)
}

func (p *sfParser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

func (p *sfParser) skipSP() {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
}

func (p *sfParser) skipOWS() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}

// dictionary parses the whole input (RFC 8941 section 4.2.2 with the leading and trailing spaces
// of section 4.2).
func (p *sfParser) dictionary() (map[string]sfValue, error) {
	out := map[string]sfValue{}
	p.skipSP()
	if p.i == len(p.s) {
		return nil, p.fail("empty header")
	}
	for {
		key, err := p.key()
		if err != nil {
			return nil, err
		}
		var v sfValue
		if p.peek() == '=' {
			p.i++
			if v, err = p.itemOrInnerList(); err != nil {
				return nil, err
			}
		} else if err := p.parameters(); err != nil {
			return nil, err
		}
		out[key] = v
		p.skipOWS()
		if p.i == len(p.s) {
			return out, nil
		}
		if p.peek() != ',' {
			return nil, p.fail("expected a comma")
		}
		p.i++
		p.skipOWS()
		if p.i == len(p.s) {
			return nil, p.fail("trailing comma")
		}
	}
}

// key parses a key: lcalpha or "*", then lcalpha, digits, "_", "-", "." and "*".
func (p *sfParser) key() (string, error) {
	start := p.i
	c := p.peek()
	if (c < 'a' || c > 'z') && c != '*' {
		return "", p.fail("expected a key")
	}
	p.i++
	for p.i < len(p.s) {
		c = p.s[p.i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' || c == '*' {
			p.i++
			continue
		}
		break
	}
	return p.s[start:p.i], nil
}

// itemOrInnerList parses a member value with its parameters.
func (p *sfParser) itemOrInnerList() (sfValue, error) {
	if p.peek() == '(' {
		p.i++
		for {
			p.skipSP()
			if p.peek() == ')' {
				p.i++
				return sfValue{}, p.parameters()
			}
			if _, err := p.bareItem(); err != nil {
				return sfValue{}, err
			}
			if err := p.parameters(); err != nil {
				return sfValue{}, err
			}
			if c := p.peek(); c != ' ' && c != ')' {
				return sfValue{}, p.fail("malformed inner list")
			}
		}
	}
	v, err := p.bareItem()
	if err != nil {
		return sfValue{}, err
	}
	return v, p.parameters()
}

// parameters parses ";key[=bare item]" repetitions.
func (p *sfParser) parameters() error {
	for p.peek() == ';' {
		p.i++
		p.skipSP()
		if _, err := p.key(); err != nil {
			return err
		}
		if p.peek() == '=' {
			p.i++
			if _, err := p.bareItem(); err != nil {
				return err
			}
		}
	}
	return nil
}

// bareItem parses one bare item; only a byte sequence keeps its value.
func (p *sfParser) bareItem() (sfValue, error) {
	switch c := p.peek(); {
	case c == '-' || (c >= '0' && c <= '9'):
		return sfValue{}, p.number()
	case c == '"':
		return sfValue{}, p.str()
	case c == '%':
		p.i++
		if p.peek() != '"' {
			return sfValue{}, p.fail("malformed display string")
		}
		return sfValue{}, p.str()
	case c == ':':
		return p.byteSequence()
	case c == '?':
		p.i++
		if b := p.peek(); b != '0' && b != '1' {
			return sfValue{}, p.fail("malformed boolean")
		}
		p.i++
		return sfValue{}, nil
	case c == '@':
		p.i++
		return sfValue{}, p.number()
	case c == '*' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
		p.token()
		return sfValue{}, nil
	}
	return sfValue{}, p.fail("expected an item")
}

// number parses an integer or decimal.
func (p *sfParser) number() error {
	if p.peek() == '-' {
		p.i++
	}
	digits, dot := 0, false
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '.' && !dot && digits > 0:
			dot = true
		default:
			if digits == 0 || p.s[p.i-1] == '.' {
				return p.fail("malformed number")
			}
			return nil
		}
		p.i++
	}
	if digits == 0 || p.s[p.i-1] == '.' {
		return p.fail("malformed number")
	}
	return nil
}

// str parses a string of printable ASCII with \" and \\ escapes.
func (p *sfParser) str() error {
	p.i++ // opening quote
	for p.i < len(p.s) {
		c := p.s[p.i]
		p.i++
		switch {
		case c == '\\':
			if n := p.peek(); n != '"' && n != '\\' {
				return p.fail("malformed escape")
			}
			p.i++
		case c == '"':
			return nil
		case c < 0x20 || c > 0x7e:
			return p.fail("character outside a string")
		}
	}
	return p.fail("unterminated string")
}

// token parses a token: the first character is checked by the caller.
func (p *sfParser) token() {
	p.i++
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c > 0x20 && c < 0x7f && !strings.ContainsRune(`"(),;<=>?@[\]{} `, rune(c)) || c == ':' || c == '/' {
			p.i++
			continue
		}
		return
	}
}

// byteSequence parses ":<base64>:"; padding is optional (RFC 8941 section 4.2.7).
func (p *sfParser) byteSequence() (sfValue, error) {
	p.i++
	end := strings.IndexByte(p.s[p.i:], ':')
	if end < 0 {
		return sfValue{}, p.fail("unterminated byte sequence")
	}
	enc := p.s[p.i : p.i+end]
	p.i += end + 1
	b, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(enc)
	}
	if err != nil {
		return sfValue{}, p.fail("bad base64")
	}
	return sfValue{isBytes: true, bytes: b}, nil
}
