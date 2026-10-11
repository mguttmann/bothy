---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team B (API)
informed: all teams
---

# API versioning and deprecation policy

## Context and Problem Statement

Apps arrive through stores and auto-update; every owner runs their own server. New apps therefore
meet old servers and vice versa. Third-party plugins and adapters use the same API. How do we
version REST, gateway and events, and how long do old versions stay in place?

## Decision Drivers

* Independent updates of apps, servers, adapters and plugins.
* A simple rule for generated clients and for proxies and logs.
* Breaking changes are rare, announced and machine-detectable.
* No fixed periods in prose; periods are named values.

## Considered Options

* Major version in the path (`/api/v1`), within it only additive changes.
* Version per header or media type (`Accept: application/vnd...+json;version=2`).
* Date-based versions per client (the server translates between versions).

## Decision Outcome

Chosen option: "Major version in the path", because it is visible to all generators, proxies and
logs and, together with strict additivity, supports the independent updates without translation
layers in the server.

Rules (details in docs/spec/06-api.md, section 13):

* `info.version` of both contracts is SemVer; MINOR for additive changes; `0.x` until the first
  release.
* Additive: new endpoints, ops, event types, response fields, optional request fields, new
  enum values in responses, new problem codes. Clients ignore what they do not know.
* Breaking: removing, renaming, changing a type, tightening what is required, changing the status
  code, the pagination or the action of an op. Only in a new major version.
* Deprecation: `deprecated: true` with `x-deprecated-since` and `x-sunset` in the spec; the server
  sends `Deprecation` (RFC 9745), `Sunset` (RFC 8594) and `Link rel="deprecation"`. `Sunset` lies
  at least `api.deprecation_min_notice` after `Deprecation`.
* Server releases with `v2` keep serving `v1` for at least `api.major_version_overlap`.
* Gateway: protocol version in `hello` and `identify`, `4007` if it is not supported;
  event payloads only additive, `schema_version` counts along.
* Security vulnerabilities may change behavior immediately; this is stated in the security
  advisory.

### Consequences

* Good, because a client recognizes in one place which major version it speaks.
* Good, because new features arrive without coordination between app and server releases.
* Bad, because a breaking change in one place needs a whole new major version; therefore we
  prefer to add new endpoints rather than change existing ones.
* Bad, because "new enum values are additive" only holds if every client tolerates them; for
  Python this needs a handwritten layer (probe of 2026-10-10, openapi-python-client 0.29.1 throws
  on unknown values).

### Confirmation

Gate G5 (tolerance test per SDK with an unknown field, enum value and event type) and G6
(breaking change check against the most recently published spec). A lint rule will later check
that `x-sunset` lies at least `api.deprecation_min_notice` after the date of `x-deprecated-since`
(ticket B-1). The tool for G6 is oasdiff, as soon as ticket B-6 proves that it is fit for OpenAPI 3.1.

## Pros and Cons of the Options

### Major version in the path

* Good, because visible and simple to route.
* Good, because generated clients get the version through the server URL.
* Bad, because the granularity is coarse.

### Version per header or media type

* Good, because URLs stay the same.
* Bad, because invisible in logs and not representable for many generators.
* Bad, because caches and proxies must respect `Vary`.

### Date-based versions per client

* Good, because very fine-grained evolution without big jumps.
* Bad, because the server must carry every old version in translations; too expensive for a
  self-hosted project without a central operations team.

## More Information

* RFC 9745 (Deprecation): https://www.rfc-editor.org/rfc/rfc9745.html (retrieved 2026-10-10)
  [verified].
* RFC 8594 (Sunset): https://www.rfc-editor.org/rfc/rfc8594.html; the order Sunset not before
  Deprecation according to RFC 9745 [verified].
* oasdiff: https://github.com/oasdiff/oasdiff (retrieved 2026-10-10); OpenAPI 3.1 status
  [assumed].
