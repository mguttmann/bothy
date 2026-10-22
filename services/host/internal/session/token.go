// SPDX-License-Identifier: Apache-2.0

package session

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// TokenType is the typ of a terminal session token (24 5.3).
const TokenType = "terminal-session+jwt"

// Values of the claim att.
const (
	AttOpen   = "open"
	AttAttach = "attach"
)

// ErrToken is a token that does not parse, whose signature does not verify or whose key is not in
// the key set: step 1 of the check order, close_detail token_invalid.
var ErrToken = errors.New("session: token unreadable or not signed by a key of the set")

// Header is the protected header of a session token.
type Header struct {
	Alg, Kid, Typ string
}

// Claims are the claims of a terminal session token (24 5.3). Wdi is nil when absent.
type Claims struct {
	Iss, Aud, Sub, Sid, Jti string
	Iat, Exp                int64
	Dev, Prf, Cek, Osu, Att string
	Own                     bool
	Wdi                     *int
}

// forbiddenHeader are the header members ADR 0067 rule 2 refuses.
var forbiddenHeader = []string{"crit", "jku", "x5u", "x5c", "zip", "jwk"}

// b64url is base64url without padding (RFC 7515).
var b64url = base64.RawURLEncoding

// ParseToken parses the compact JWS tok with the profile of ADR 0067 (Design note 4 of M2-45) and
// verifies its EdDSA signature with the Ed25519 key of keys whose kid equals the header kid. keys are
// the JWK objects of the jwks of the hello of this data connection. It returns the header and the
// claims; any failure wraps ErrToken. typ is returned, not checked: step 2 checks it.
//
// Rules: exactly two dots, base64url without padding; header alg exactly EdDSA, a kid, a typ, none
// of crit, jku, x5u, x5c, zip, jwk; the key kty OKP, crv Ed25519; the claims iss, aud, sub, sid,
// jti, iat, exp, dev, prf, cek, own, osu, att required with their JSON types, wdi an integer of at
// least 0 when present, att open or attach; a JSON null anywhere, a missing or mistyped claim or
// trailing data fails; unknown claims are ignored.
func ParseToken(tok string, keys []map[string]any) (Header, Claims, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return Header{}, Claims{}, ErrToken
	}
	hb, err1 := b64url.DecodeString(parts[0])
	pb, err2 := b64url.DecodeString(parts[1])
	sig, err3 := b64url.DecodeString(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return Header{}, Claims{}, ErrToken
	}
	h, err := parseHeader(hb)
	if err != nil {
		return Header{}, Claims{}, err
	}
	pub, err := keyOf(keys, h.Kid)
	if err != nil {
		return Header{}, Claims{}, err
	}
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		return Header{}, Claims{}, ErrToken
	}
	c, err := parseClaims(pb)
	if err != nil {
		return Header{}, Claims{}, err
	}
	return h, c, nil
}

// objectOf decodes b as one JSON object without null values and without trailing data.
func objectOf(b []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	var m map[string]json.RawMessage
	if err := dec.Decode(&m); err != nil || m == nil {
		return nil, ErrToken
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrToken // trailing data, also data that is no JSON
	}
	for _, v := range m {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return nil, ErrToken
		}
	}
	return m, nil
}

func parseHeader(b []byte) (Header, error) {
	m, err := objectOf(b)
	if err != nil {
		return Header{}, err
	}
	for _, k := range forbiddenHeader {
		if _, ok := m[k]; ok {
			return Header{}, ErrToken
		}
	}
	var h Header
	if !str(m, "alg", &h.Alg) || !str(m, "kid", &h.Kid) || !str(m, "typ", &h.Typ) || h.Alg != "EdDSA" || h.Kid == "" {
		return Header{}, ErrToken
	}
	return h, nil
}

// keyOf returns the Ed25519 key with kid from keys.
func keyOf(keys []map[string]any, kid string) (ed25519.PublicKey, error) {
	for _, k := range keys {
		if k["kid"] != kid {
			continue
		}
		x, ok := k["x"].(string)
		if k["kty"] != "OKP" || k["crv"] != "Ed25519" || !ok {
			return nil, ErrToken
		}
		raw, err := b64url.DecodeString(x)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, ErrToken
		}
		return ed25519.PublicKey(raw), nil
	}
	return nil, ErrToken
}

func parseClaims(b []byte) (Claims, error) {
	m, err := objectOf(b)
	if err != nil {
		return Claims{}, err
	}
	var c Claims
	ok := str(m, "iss", &c.Iss) && str(m, "aud", &c.Aud) && str(m, "sub", &c.Sub) && str(m, "sid", &c.Sid) &&
		str(m, "jti", &c.Jti) && num(m, "iat", &c.Iat) && num(m, "exp", &c.Exp) && str(m, "dev", &c.Dev) &&
		str(m, "prf", &c.Prf) && str(m, "cek", &c.Cek) && str(m, "osu", &c.Osu) && str(m, "att", &c.Att)
	if !ok || (c.Att != AttOpen && c.Att != AttAttach) {
		return Claims{}, ErrToken
	}
	raw, present := m["own"]
	if !present || json.Unmarshal(raw, &c.Own) != nil {
		return Claims{}, ErrToken
	}
	if raw, present := m["wdi"]; present {
		var w int64
		if !integer(raw, &w) || w < 0 {
			return Claims{}, ErrToken
		}
		i := int(w)
		c.Wdi = &i
	}
	return c, nil
}

// str reads the string member k of m into dst.
func str(m map[string]json.RawMessage, k string, dst *string) bool {
	raw, ok := m[k]
	if !ok || len(raw) == 0 || raw[0] != '"' {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

// num reads the integer member k of m (a NumericDate) into dst.
func num(m map[string]json.RawMessage, k string, dst *int64) bool {
	raw, ok := m[k]
	return ok && integer(raw, dst)
}

// integer decodes raw as a JSON integer without fraction or exponent.
func integer(raw json.RawMessage, dst *int64) bool {
	s := strings.TrimSpace(string(raw))
	if s == "" || strings.ContainsAny(s, ".eE") {
		return false
	}
	return json.Unmarshal([]byte(s), dst) == nil
}
