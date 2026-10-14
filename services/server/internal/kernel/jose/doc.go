// SPDX-License-Identifier: Apache-2.0

// Package jose reads, builds and verifies compact JWS, reads JWKs and computes their RFC 7638
// thumbprints with the standard library only
// (docs/adr/0067-jose-profil-mit-der-standardbibliothek.md). It is part of the kernel itself;
// identity (DPoP, access tokens, step-up, ID tokens, client assertions), instance (JWKS,
// checkpoint verification), audit and later runs use it.
//
// The profile (M1-33): the compact serialization only; ES256 (R || S, 64 bytes) and EdDSA
// (Ed25519); header and payload are JSON objects without duplicate members; every parse names its
// allowed algorithms and the expected typ; none, HS*, crit, jku, x5u, x5c and zip are refused.
// Keys are EC P-256 and OKP Ed25519 without private members. M1-41 added RS256 and PS256 (ADR 0067)
// for the ID tokens of identity providers (ParseVerificationJWK also accepts RSA keys of 2048 to
// 8192 bits) and the client assertions of private_key_jwt (RS256Signer, PS256Signer); ParseJWK,
// the parser of device keys and DPoP proofs, still refuses RSA.
//
// Example:
//
//	j, err := jose.ParseCompactJWS(proof, []string{jose.AlgES256, jose.AlgEdDSA}, "dpop+jwt")
//	if err != nil {
//		return err // wraps jose.ErrInvalidJWS
//	}
//	k, err := jose.ParseJWK(j.Header.JWK) // wraps jose.ErrInvalidJWK
//	if err == nil {
//		err = j.Verify(k) // wraps jose.ErrSignature
//	}
package jose
