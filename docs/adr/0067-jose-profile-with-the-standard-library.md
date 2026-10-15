---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
---

# JOSE profile with the standard library instead of a JOSE library

## Context and Problem Statement

The server builds and verifies compact JWS in several places: access tokens (EdDSA, purpose
`access_token`), invocation tokens for agents (purpose `invocation`), checkpoints of the audit log
(purpose `audit_checkpoint`), DPoP proofs and step-up signatures of the devices (`ES256`, `EdDSA`),
ID tokens of the OIDC providers (`RS256`, `PS256`, `ES256`, `EdDSA`) and client assertions for
`private_key_jwt` ([08-identity.md](../spec/08-identity.md), sections 2.2, 6.2, 7.2 and 7.3;
[19-security.md](../spec/19-security.md), section 12). These places are in different
modules (`identity`, `instance`, `audit`, `runs`), and domain packages may import only packages from
`kernel-core`, no third-party libraries ([02-architecture.md](../spec/02-architecture.md),
section 5). Without a ruling, two build tickets of the same wave (PLAN M1-32 and M1-33) would
each have chosen their own solution. What does the server use to build and verify JWS?

## Decision Drivers

* RFC 8725 requires that the caller specifies the allowed algorithms and that no others
  are used (section 3.1 "Perform Algorithm Verification"), and recommends an explicit
  type (section 3.11 "Use Explicit Typing")
  (https://www.rfc-editor.org/rfc/rfc8725.html, retrieved 2026-10-10) [verified].
* One implementation for all modules; signing and verifying must not drift apart.
* Usable from domain packages, that is, without a third-party library.
* Small attack surface: only the compact form, a few algorithms and JWK
  fingerprints per RFC 7638 are needed; JWE, JSON serialization and keys from headers are not.
* Few dependencies in a security path (supply chain, SEC-SC-1).

## Considered Options

* Own JOSE profile with the Go standard library in the package `internal/kernel/jose`
* A JOSE library (for example `github.com/go-jose/go-jose`) in the adapters
* A comprehensive JOSE library (for example `github.com/lestrrat-go/jwx`) in the adapters
* Each module builds its JWS itself

## Decision Outcome

Chosen: "Own JOSE profile with the standard library in `internal/kernel/jose`", because it is the
only option usable from domain packages, covers exactly the profile that is needed and enforces the
algorithm list on every call.

Profile (normative together with 08):

1. Only the compact serialization of JWS (RFC 7515). No JWE, no JSON serialization.
2. Every call names the allowed algorithms and the expected `typ`. `none`, `HS*` and every
   algorithm that is not named are rejected, as are headers with `crit`, `jku`, `x5u`, `x5c`
   or `zip`. Algorithms of the profile: `EdDSA` (Ed25519), `ES256` (signature as `R || S` with
   64 bytes per RFC 7518, section 3.4, never DER), `RS256` and `PS256` (only for verifying ID tokens
   and signing client assertions).
3. The caller always supplies the verification key (`kid` from its key list). The only
   exception is the public `jwk` in the header of a DPoP proof (RFC 9449); a `jwk` with
   private parts is rejected.
4. JWK fingerprints per RFC 7638 for `EC` (P-256), `OKP` (Ed25519) and `RSA`.
5. Signing takes a signer over the signing input; private keys stay with the
   caller (for server keys the port `instance.Keys`, ADR 0066).

Who builds what: PLAN M1-33 creates the package with parsing, verifying (`EdDSA`, `ES256`),
fingerprints and assembling compact JWS; M1-34 uses it to assemble the access tokens; M1-41 adds
`RS256` and `PS256` (RSA keys with 2048 to 8192 bits only through `ParseVerificationJWK`, never for
device keys) and, when signing the client assertion, the header `x5t#S256` that Entra requires for
certificates; verifying does not read it. Every extension of the profile (an algorithm, a header) needs a
change to this ADR.

### Consequences

* Good, because every JWS path goes through the same algorithm check and no module introduces a
  second implementation.
* Good, because domain packages can verify JWS without breaking the layer rule.
* Good, because no third-party library is in the login path.
* Bad, because own code in the area of cryptography is a risk. Mitigation: only the structure and
  the validation of the format are own code, all cryptographic algorithms come from the
  standard library; test vectors from the RFCs, negative tests and a fuzz test (Confirmation).
* Bad, because a later need for JWE or further algorithms needs a new decision.
  That is intended.

### Confirmation

* Unit tests in `internal/kernel/jose` with the examples from RFC 7515 appendix A.3 (`ES256`),
  RFC 8037 appendix A.4 (`Ed25519`) and RFC 7638 section 3.1 (fingerprint).
* Negative tests: `alg` equal to `none`, `HS256` or not in the allowed list, wrong `typ`,
  `ES256` signature in DER form, `crit` in the header, `jwk` with a private part, extra dots or
  characters outside base64url; all rejected.
* Go fuzz test of the parser with a seed corpus in the normal test run (PLAN M1-33).
* The architecture gate (`make arch`) allows `internal/kernel/jose` only imports from the
  standard library ([02-architecture.md](../spec/02-architecture.md), section 7.2).

## Pros and Cons of the Options

### Own profile with the standard library

* Good, because it is small, verifiable and usable from every layer.
* Good, because the list of allowed algorithms is a required parameter of every verification
  function.
* Bad, because the format code is written and maintained in-house.

### `go-jose` in the adapters

* Good, because it is widespread and specialized in JOSE.
* Bad, because domain packages must not import it; verification would then be in adapters,
  separated from the rules it needs.
* Bad, because it supports more formats and algorithms than the product needs.

### `jwx` in the adapters

* Good, because it brings JWT, JWK sets and caches.
* Bad, because it brings even more scope than needed and hits the same layer boundary.

### Each module builds its own

* Good, because it is possible without coordination.
* Bad, because several implementations drift apart; a missing algorithm check in
  one place is enough for an attack.

## More Information

* Related: ADR 0020 (DPoP), ADR 0066 (signing keys), ADR 0021 (audit checkpoints).
* Normative places: [08-identity.md](../spec/08-identity.md), sections 7.2 and 7.3;
  [02-architecture.md](../spec/02-architecture.md), section 4.5.
