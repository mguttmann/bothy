// SPDX-License-Identifier: Apache-2.0

// Package virtualauthn is a software WebAuthn authenticator for tests (brief M1-40 "Virtual
// authenticator"; also used by M1-71 and the console tests). An Authenticator holds one key pair
// in memory and answers creation and request options with the JSON a browser sends: a
// PublicKeyCredential whose clientDataJSON carries type, challenge and origin, and whose
// authenticator data carries the RP ID hash, the flags and the counter. Attestation is always
// format none. Every field of an Authenticator is a knob a test may turn between two calls: drop
// the user verified flag, change the origin or the RP ID, set or freeze the counter, sign with
// another key, set the backup flags, or answer with the user handle of a discoverable credential.
// It never touches the network or a real authenticator.
//
// Example:
//
//	a := virtualauthn.New("https://chat.example.com", virtualauthn.ES256)
//	resp, err := a.Create(creationOptionsJSON) // completes a registration
//	resp, err = a.Get(requestOptionsJSON)      // completes a login or step-up
package virtualauthn

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

// Alg is a COSE algorithm the authenticator can create keys for.
type Alg int64

// The algorithms of the authenticator: the three the server accepts, and ES384, which it refuses.
const (
	ES256 Alg = -7
	EdDSA Alg = -8
	ES384 Alg = -35
	RS256 Alg = -257
)

// The flags of the authenticator data (WebAuthn Level 3 section 6.1).
const (
	flagUP = 0x01
	flagUV = 0x04
	flagBE = 0x08
	flagBS = 0x10
	flagAT = 0x40
)

// Authenticator is one software credential. Create makes its key and credential ID; Get signs
// assertions with it. The exported fields are the knobs of the tests; the zero values of the
// knobs behave like a well-behaved platform authenticator with user verification. It is not safe
// for concurrent use.
type Authenticator struct {
	// Origin is the origin the client data names.
	Origin string
	// RPID overrides the RP ID whose hash the authenticator data carries; "" takes the RP ID of the
	// options.
	RPID string
	// NoUserVerification drops the user verified flag.
	NoUserVerification bool
	// Counter is the signature counter; Get adds one before signing unless FreezeCounter is set.
	Counter       uint32
	FreezeCounter bool
	// BackupEligible and BackupState set the backup flags.
	BackupEligible, BackupState bool
	// Discoverable makes Get return the user handle, like a discoverable credential.
	Discoverable bool
	// SignWith signs assertions with another authenticator's key (a wrong signature).
	SignWith *Authenticator
	// Reuse makes Create answer with the key and credential ID of the earlier Create instead of a
	// new one (a duplicate registration).
	Reuse bool

	alg          Alg
	signer       crypto.Signer
	credentialID []byte
	userHandle   []byte
}

// New returns an authenticator for origin that creates a key of alg on Create.
func New(origin string, alg Alg) *Authenticator {
	return &Authenticator{Origin: origin, alg: alg}
}

// CredentialID returns the credential ID after Create; nil before.
func (a *Authenticator) CredentialID() []byte { return a.credentialID }

// creationOptions is the part of PublicKeyCredentialCreationOptions the authenticator reads.
type creationOptions struct {
	Challenge string `json:"challenge"`
	RP        struct {
		ID string `json:"id"`
	} `json:"rp"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
}

// requestOptions is the part of PublicKeyCredentialRequestOptions the authenticator reads.
type requestOptions struct {
	Challenge string `json:"challenge"`
	RPID      string `json:"rpId"`
}

// Create answers creation options (JSON form) with a new key and credential: a PublicKeyCredential
// with clientDataJSON of type webauthn.create and an attestation object of format none. Errors:
// unreadable options or a failing key generation.
func (a *Authenticator) Create(optionsJSON []byte) ([]byte, error) {
	var o creationOptions
	if err := json.Unmarshal(optionsJSON, &o); err != nil || o.Challenge == "" || o.RP.ID == "" {
		return nil, errors.New("virtualauthn: unreadable creation options")
	}
	handle, err := base64.RawURLEncoding.DecodeString(o.User.ID)
	if err != nil {
		return nil, errors.New("virtualauthn: user.id is not base64url")
	}
	if !a.Reuse || a.signer == nil {
		if a.signer, err = newKey(a.alg); err != nil {
			return nil, err
		}
		a.credentialID = make([]byte, 32)
		if _, err := rand.Read(a.credentialID); err != nil {
			return nil, err
		}
	}
	a.userHandle = handle
	cose, err := coseKey(a.alg, a.signer.Public())
	if err != nil {
		return nil, err
	}
	auth := a.authData(o.RP.ID, flagAT)
	attested := make([]byte, 0, 18+len(a.credentialID)+len(cose))
	attested = append(attested, make([]byte, 16)...)                                // AAGUID zero: attestation none
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(a.credentialID))) //nolint:gosec // G115: the credential ID has 32 bytes
	attested = append(attested, a.credentialID...)
	attested = append(attested, cose...)
	auth = append(auth, attested...)
	obj, err := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": auth})
	if err != nil {
		return nil, err
	}
	cd := a.clientData("webauthn.create", o.Challenge)
	return json.Marshal(map[string]any{
		"id": b64(a.credentialID), "rawId": b64(a.credentialID), memberType: credentialType, "authenticatorAttachment": attachment,
		"clientExtensionResults": map[string]any{},
		"response":               map[string]any{"clientDataJSON": b64(cd), "attestationObject": b64(obj), "transports": []string{"internal"}},
	})
}

// Get answers request options (JSON form) with an assertion of the credential: clientDataJSON of
// type webauthn.get and a signature over the authenticator data and the hash of the client data.
// Errors: unreadable options or Get before Create.
func (a *Authenticator) Get(optionsJSON []byte) ([]byte, error) {
	if a.signer == nil {
		return nil, errors.New("virtualauthn: Get before Create")
	}
	var o requestOptions
	if err := json.Unmarshal(optionsJSON, &o); err != nil || o.Challenge == "" {
		return nil, errors.New("virtualauthn: unreadable request options")
	}
	if !a.FreezeCounter {
		a.Counter++
	}
	auth := a.authData(o.RPID, 0)
	cd := a.clientData("webauthn.get", o.Challenge)
	h := sha256.Sum256(cd)
	signer := a
	if a.SignWith != nil {
		signer = a.SignWith
	}
	sig, err := signer.sign(append(append([]byte(nil), auth...), h[:]...))
	if err != nil {
		return nil, err
	}
	resp := map[string]any{"clientDataJSON": b64(cd), "authenticatorData": b64(auth), "signature": b64(sig)}
	if a.Discoverable {
		resp["userHandle"] = b64(a.userHandle)
	}
	return json.Marshal(map[string]any{
		"id": b64(a.credentialID), "rawId": b64(a.credentialID), memberType: credentialType, "authenticatorAttachment": attachment,
		"clientExtensionResults": map[string]any{}, "response": resp,
	})
}

// authData builds the authenticator data for rpID (or the RPID knob) with the flags of the knobs
// and extra.
func (a *Authenticator) authData(rpID string, extra byte) []byte {
	if a.RPID != "" {
		rpID = a.RPID
	}
	h := sha256.Sum256([]byte(rpID))
	flags := byte(flagUP) | extra
	if !a.NoUserVerification {
		flags |= flagUV
	}
	if a.BackupEligible {
		flags |= flagBE
	}
	if a.BackupState {
		flags |= flagBS
	}
	out := append(h[:], flags)
	return binary.BigEndian.AppendUint32(out, a.Counter)
}

// clientData is the collected client data a browser serializes.
func (a *Authenticator) clientData(typ, challenge string) []byte {
	b, _ := json.Marshal(map[string]any{memberType: typ, "challenge": challenge, "origin": a.Origin, "crossOrigin": false})
	return b
}

// sign signs data with the key of a as its algorithm requires (ECDSA in ASN.1 DER).
func (a *Authenticator) sign(data []byte) ([]byte, error) {
	switch a.alg {
	case ES256, RS256:
		h := sha256.Sum256(data)
		return a.signer.Sign(rand.Reader, h[:], crypto.SHA256)
	case ES384:
		h := sha512.Sum384(data)
		return a.signer.Sign(rand.Reader, h[:], crypto.SHA384)
	case EdDSA:
		return a.signer.Sign(rand.Reader, data, crypto.Hash(0))
	}
	return nil, fmt.Errorf("virtualauthn: algorithm %d", a.alg)
}

// newKey generates a key for alg.
func newKey(alg Alg) (crypto.Signer, error) {
	switch alg {
	case ES256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case ES384:
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case RS256:
		return rsa.GenerateKey(rand.Reader, 2048)
	case EdDSA:
		_, k, err := ed25519.GenerateKey(rand.Reader)
		return k, err
	}
	return nil, fmt.Errorf("virtualauthn: algorithm %d", alg)
}

// coseKey encodes pub as a COSE key of alg (RFC 9053).
func coseKey(alg Alg, pub crypto.PublicKey) ([]byte, error) {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		crv, size := int64(webauthncose.P256), 32
		if alg == ES384 {
			crv, size = int64(webauthncose.P384), 48
		}
		raw, err := k.Bytes() // uncompressed point 0x04 || X || Y
		if err != nil {
			return nil, err
		}
		return webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
			PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.EllipticKey), Algorithm: int64(alg)},
			Curve:         crv, XCoord: raw[1 : 1+size], YCoord: raw[1+size:],
		})
	case *rsa.PublicKey:
		return webauthncbor.Marshal(webauthncose.RSAPublicKeyData{
			PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.RSAKey), Algorithm: int64(alg)},
			Modulus:       k.N.Bytes(), Exponent: binary.BigEndian.AppendUint32(nil, uint32(k.E))[1:], //nolint:gosec // G115: the generated exponent is 65537
		})
	case ed25519.PublicKey:
		return webauthncbor.Marshal(webauthncose.OKPPublicKeyData{
			PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.OctetKey), Algorithm: int64(alg)},
			Curve:         int64(webauthncose.Ed25519), XCoord: k,
		})
	}
	return nil, errors.New("virtualauthn: unknown key type")
}

// The members every PublicKeyCredential of this authenticator carries.
const (
	credentialType = "public-key"
	memberType     = "type"
	attachment     = "platform"
)

// b64 is base64url without padding, the encoding of the JSON form.
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
