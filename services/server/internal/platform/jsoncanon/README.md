# Package `platform/jsoncanon`

Status: built (M1-20). Normative are [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) and
[02-architektur.md](../../../../../docs/spec/02-architektur.md) section "Idempotenz im Unit of
Work"; the chapter wins on any difference.

## Purpose

The canonical form of a JSON value (JSON Canonicalization Scheme), so that two requests that differ
only in member order, whitespace or string escapes have the same fingerprint in the
Idempotency-Key stage of `platform/httpserver`.

## Interface

- `Canonicalize(data) ([]byte, error)`: the RFC 8785 serialization. Members sorted by UTF-16 code
  units, no whitespace, minimal string escapes, numbers in the ECMAScript form of their double
  value.
- `ErrNoCanonicalForm`: the input is not exactly one JSON value in valid UTF-8, repeats a member
  name, or holds an integer literal (no fraction, no exponent) that a double cannot hold exactly.
  RFC 8785 would round such an integer, and two different requests would get one fingerprint;
  the API carries no integer beyond 2^53 - 1 anyway (ADR 0053). The error never repeats the
  input.

## Tests

`TestJCS_RFC8785Vectors` holds the examples of RFC 8785 sections 3.2.2 and 3.2.3 and numbers of
Appendix B; `TestCanonicalize_Refuses` the inputs without a canonical form.

## Dependencies

Only the standard library and the vendor group `jcs` (`github.com/gowebpki/jcs` v1.0.2,
Apache-2.0; module register).
