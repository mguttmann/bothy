// SPDX-License-Identifier: Apache-2.0

package jose

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
)

// ErrInvalidJWS is wrapped by every error of ParseCompactJWS and SignCompact: the token breaks
// the profile (form, header, allowlist, typ). The wrapping error names the rule only.
var ErrInvalidJWS = errors.New("jose: invalid JWS")

// ErrSignature is wrapped by every error of JWS.Verify: the signature does not verify with the
// key, has the wrong length or form, or the key does not fit the algorithm.
var ErrSignature = errors.New("jose: signature does not verify")

// es256SigSize is the length of R || S (RFC 7518 section 3.4).
const es256SigSize = 64

// forbiddenHeaders are header members the profile refuses (ADR 0067 item 2): crit could make a
// verifier skip a rule, jku, x5u and x5c would let the token pick its own key, zip compresses.
var forbiddenHeaders = []string{"crit", "jku", "x5u", "x5c", "zip"}

// supported are the algorithms this package verifies and signs.
var supported = []string{AlgES256, AlgEdDSA}

// Header is the protected header of a compact JWS. Parsing fills the members the profile reads;
// signing writes the members that are set, in the order alg, typ, kid, jwk.
type Header struct {
	// Alg is the algorithm; SignCompact sets it from the signer when empty.
	Alg string `json:"alg"`
	// Typ is the explicit type (RFC 8725 section 3.11), for example "dpop+jwt"; "" for none.
	Typ string `json:"typ,omitempty"`
	// Kid names the key of the issuer; "" for none.
	Kid string `json:"kid,omitempty"`
	// JWK is the public key in the header (DPoP only, RFC 9449); nil for none. ParseCompactJWS
	// returns it unchecked: the caller validates it with ParseJWK.
	JWK json.RawMessage `json:"jwk,omitempty"`
}

// JWS is a parsed compact JWS whose form passed the profile; Verify checks the signature.
type JWS struct {
	// Header is the protected header.
	Header Header
	// Payload is the decoded payload, a JSON object.
	Payload []byte
	// signingInput is "<header>.<payload>" as received.
	signingInput []byte
	signature    []byte
}

// Signer signs the signing input of a JWS with a private key it keeps to itself (ADR 0067 item 5;
// server keys come from instance.Keys, ADR 0066). Alg is the JWS algorithm of the key.
type Signer interface {
	// Alg returns the algorithm, AlgES256 or AlgEdDSA.
	Alg() string
	// Sign returns the JWS signature over signingInput: R || S with 64 bytes for ES256, the
	// Ed25519 signature for EdDSA.
	Sign(signingInput []byte) ([]byte, error)
}

// ParseCompactJWS parses s in the compact serialization and checks the profile (ADR 0067, brief
// M1-33 "JWS profile"): exactly three segments of base64url without padding; header and payload
// are JSON objects without duplicate members, valid UTF-8 and no deeper than MaxJSONDepth; alg is
// in allowed and in the profile (none and HS* never are); no crit, jku, x5u, x5c or zip member;
// typ equals typ without regard to ASCII case when typ is not empty. The signature is checked by
// Verify, with a key the caller chooses.
//
// Errors wrap ErrInvalidJWS.
//
// Example:
//
//	j, err := jose.ParseCompactJWS(proof, []string{jose.AlgES256, jose.AlgEdDSA}, "dpop+jwt")
//	k, err := jose.ParseJWK(j.Header.JWK)
//	err = j.Verify(k)
func ParseCompactJWS(s string, allowed []string, typ string) (JWS, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return JWS{}, fmt.Errorf("%w: want three segments", ErrInvalidJWS)
	}
	head, ok1 := decodeSegment(parts[0])
	payload, ok2 := decodeSegment(parts[1])
	sig, ok3 := decodeSegment(parts[2])
	if !ok1 || !ok2 || !ok3 || len(sig) == 0 {
		return JWS{}, fmt.Errorf("%w: segments must be non-empty base64url without padding", ErrInvalidJWS)
	}
	if checkObject(head) != nil || checkObject(payload) != nil {
		return JWS{}, fmt.Errorf("%w: header and payload must be JSON objects", ErrInvalidJWS)
	}
	h, err := parseHeader(head)
	if err != nil {
		return JWS{}, err
	}
	if !slices.Contains(supported, h.Alg) || !slices.Contains(allowed, h.Alg) {
		return JWS{}, fmt.Errorf("%w: algorithm not allowed", ErrInvalidJWS)
	}
	if typ != "" && !strings.EqualFold(h.Typ, typ) {
		return JWS{}, fmt.Errorf("%w: unexpected typ", ErrInvalidJWS)
	}
	return JWS{Header: h, Payload: payload, signingInput: []byte(parts[0] + "." + parts[1]), signature: sig}, nil
}

// parseHeader reads the members of the profile and refuses the forbidden ones.
func parseHeader(head []byte) (Header, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(head, &all); err != nil {
		return Header{}, fmt.Errorf("%w: header", ErrInvalidJWS)
	}
	for _, m := range forbiddenHeaders {
		if _, ok := all[m]; ok {
			return Header{}, fmt.Errorf("%w: header member %q is not allowed", ErrInvalidJWS, m)
		}
	}
	var h Header
	for _, f := range []struct {
		name     string
		dst      *string
		required bool
	}{{"alg", &h.Alg, true}, {"typ", &h.Typ, false}, {"kid", &h.Kid, false}} {
		v, ok := all[f.name]
		if !ok {
			if f.required {
				return Header{}, fmt.Errorf("%w: header member %q is missing", ErrInvalidJWS, f.name)
			}
			continue
		}
		if !stringOf(v, f.dst) {
			return Header{}, fmt.Errorf("%w: header member %q is not a string", ErrInvalidJWS, f.name)
		}
	}
	if v, ok := all["jwk"]; ok {
		h.JWK = append(json.RawMessage(nil), v...)
	}
	return h, nil
}

// Verify checks the signature with k (ADR 0067 item 4): ES256 needs an EC P-256 key and a
// signature of exactly 64 bytes R || S over SHA-256 of the signing input (DER and other lengths
// fail); EdDSA needs an Ed25519 key and a 64-byte signature. A key whose alg member names another
// algorithm fails too. Errors wrap ErrSignature.
func (j JWS) Verify(k JWK) error {
	if k.Alg != "" && k.Alg != j.Header.Alg {
		return fmt.Errorf("%w: key is for another algorithm", ErrSignature)
	}
	return verifySignature(j.Header.Alg, k, j.signingInput, j.signature)
}

// verifySignature checks sig over input with k for alg.
func verifySignature(alg string, k JWK, input, sig []byte) error {
	switch alg {
	case AlgES256:
		pub, ok := k.pub.(*ecdsa.PublicKey)
		if !ok || len(sig) != es256SigSize {
			return fmt.Errorf("%w: ES256 needs a P-256 key and a 64-byte signature", ErrSignature)
		}
		sum := sha256.Sum256(input)
		r := new(big.Int).SetBytes(sig[:32])
		s := new(big.Int).SetBytes(sig[32:])
		if !ecdsa.Verify(pub, sum[:], r, s) {
			return ErrSignature
		}
		return nil
	case AlgEdDSA:
		pub, ok := k.pub.(ed25519.PublicKey)
		if !ok || len(sig) != ed25519.SignatureSize {
			return fmt.Errorf("%w: EdDSA needs an Ed25519 key and a 64-byte signature", ErrSignature)
		}
		if !ed25519.Verify(pub, input, sig) {
			return ErrSignature
		}
		return nil
	}
	return fmt.Errorf("%w: algorithm outside the profile", ErrSignature)
}

// SignCompact builds a compact JWS of payload (a JSON object) with header and signer: alg is the
// signer's algorithm (a different header.Alg is an error), the header is written with the members
// that are set. Token issuers use it: access tokens (M1-34), audit checkpoints (M1-50),
// invocation tokens (M2). Errors wrap ErrInvalidJWS, or are the signer's error.
//
// Example:
//
//	tok, err := jose.SignCompact(signer, jose.Header{Typ: "at+jwt", Kid: kid}, claims)
func SignCompact(signer Signer, header Header, payload []byte) (string, error) {
	alg := signer.Alg()
	if !slices.Contains(supported, alg) || header.Alg != "" && header.Alg != alg {
		return "", fmt.Errorf("%w: header alg must be the signer's algorithm", ErrInvalidJWS)
	}
	if checkObject(payload) != nil {
		return "", fmt.Errorf("%w: payload must be a JSON object", ErrInvalidJWS)
	}
	if header.JWK != nil && checkObject(header.JWK) != nil {
		return "", fmt.Errorf("%w: header jwk must be a JSON object", ErrInvalidJWS)
	}
	header.Alg = alg
	head, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("%w: header", ErrInvalidJWS)
	}
	input := encodeSegment(head) + "." + encodeSegment(payload)
	sig, err := signer.Sign([]byte(input))
	if err != nil {
		return "", err
	}
	return input + "." + encodeSegment(sig), nil
}

// ES256Signer signs with an ECDSA P-256 private key; the signature is R || S with 64 bytes.
type ES256Signer struct {
	// Key is the private key; it must be on P-256.
	Key *ecdsa.PrivateKey
}

// Alg returns AlgES256.
func (ES256Signer) Alg() string { return AlgES256 }

// Sign signs SHA-256 of signingInput with randomness from crypto/rand and returns R || S.
func (s ES256Signer) Sign(signingInput []byte) ([]byte, error) {
	sum := sha256.Sum256(signingInput)
	r, sv, err := ecdsa.Sign(rand.Reader, s.Key, sum[:])
	if err != nil {
		return nil, err
	}
	out := make([]byte, es256SigSize)
	r.FillBytes(out[:32])
	sv.FillBytes(out[32:])
	return out, nil
}

// EdDSASigner signs with an Ed25519 private key.
type EdDSASigner struct {
	// Key is the private key.
	Key ed25519.PrivateKey
}

// Alg returns AlgEdDSA.
func (EdDSASigner) Alg() string { return AlgEdDSA }

// Sign returns the Ed25519 signature of signingInput.
func (s EdDSASigner) Sign(signingInput []byte) ([]byte, error) {
	return ed25519.Sign(s.Key, signingInput), nil
}
