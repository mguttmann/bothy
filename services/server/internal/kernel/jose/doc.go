// SPDX-License-Identifier: Apache-2.0

// Package jose reads, builds and verifies compact JWS (ES256, EdDSA, RS256, PS256), reads JWKs and
// computes their RFC 7638 thumbprints with the standard library only
// (docs/adr/0067-jose-profil-mit-der-standardbibliothek.md). It is part of the kernel itself;
// identity (DPoP, access tokens, step-up, ID tokens, client assertions), instance (JWKS,
// checkpoint verification), audit and later runs use it.
package jose
