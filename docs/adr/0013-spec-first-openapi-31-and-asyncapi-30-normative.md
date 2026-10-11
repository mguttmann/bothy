---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team B (API)
informed: all teams
---

# Spec first: OpenAPI 3.1 and AsyncAPI 3.0 are normative, code generation

## Context and Problem Statement

The server (Go), the apps (Flutter/Dart), the SDKs (Python, TypeScript) and the Hermes adapter
speak the same API. In addition, there are gateway events and webhooks with the same objects. Every
route must carry an action from the catalogue (B7, 05-actions.md rule 3). How do we keep a
contract from which all sides are generated with types and which can be checked?

## Decision Drivers

* One contract for four languages, without manual work on types (Baseline B2, R-Clients).
* Authorization action and rate-limit class on every operation, machine-checkable.
* REST, gateway and webhooks share schemas (event envelope, message, run).
* Tools must handle OpenAPI 3.1; multi-type unions are a risk for oapi-codegen
  (Baseline R-Stack).
* Changes should be visible in the review as a contract change.

## Considered Options

* Spec-first: hand-maintained openapi.yaml and asyncapi.yaml are normative, code is generated.
* Code-first: the spec is generated from annotated Go code.
* Intermediate language (TypeSpec or similar), from which OpenAPI and AsyncAPI are generated.

## Decision Outcome

Chosen option: "Spec-first", because it is the only option that fixes the contract before the code,
serves all four generators directly and enforces the required fields `x-authz-action` and
`x-rate-limit-class` by lint, without introducing a further language.

Rulings:

* openapi.yaml (OpenAPI 3.1) contains REST, webhooks and all schemas, including those of the
  gateway. asyncapi.yaml (AsyncAPI 3.0) references them via `$ref`. Event schemas carry no
  OpenAPI discriminator objects, because AsyncAPI parsers expect a string (own probe with
  @asyncapi/cli 6.2.0 on 2026-10-10).
* Conservative schemas: no `null`, no type arrays, no `anyOf`, `oneOf` only with a
  discriminator, repeated enums as named schemas (06-api.md, sections 2 and 14).
* Generators: oapi-codegen (Go server, `std-http-server` plus `strict-server`), openapi-typescript,
  openapi-python-client, openapi-generator `dart-dio`. Generated code is checked in.
* Descriptions in the YAML files are English, because they move into generated code.

### Consequences

* Good, because all clients come from one source; a field change breaks the build
  immediately at every affected place.
* Good, because lint rules enforce the binding to the action catalogue before code exists.
* Good, because reviewers see contract changes as the diff of one file.
* Bad, because the spec becomes large and must be maintained by hand; errors in examples are caught
  only by the lint.
* Bad, because every generator has its own gaps (DPoP schema, `application/octet-stream` in
  Dart, strict enums in Python); every SDK needs a handwritten layer.
* Neutral, because OpenAPI 3.1 support counts as "initial" in oapi-codegen v2.8.0 and as beta in
  openapi-generator; the probe of 2026-10-10 still ran through in all four generators.

### Confirmation

Gate G1 (Redocly lint with docs/api/redocly.yaml, zero errors and warnings), G2 (comparison with
05-actions.md and 04-sync-protocol.md), G4 (all four generators generate and compile,
no difference to the checked-in code) and the route test from 05-actions.md section 9.
See docs/spec/06-api.md, section 15.

## Pros and Cons of the Options

### Spec-first

* Good, because the contract is fixed before the implementation and apps can build in parallel.
* Good, because tools (lint, Schemathesis, generators) work directly on the file.
* Bad, because server and spec can drift apart; generated stubs and G8 help against this.

### Code-first

* Good, because the spec never deviates from the server.
* Bad, because the contract only comes into being after the code, and apps wait.
* Bad, because extensions such as `x-authz-action` and webhook sections depend on the annotations
  of the framework; for Go there is no tool that generates OpenAPI 3.1, webhooks and AsyncAPI
  together [assumed].

### Intermediate language

* Good, because more compact than YAML and with reuse.
* Bad, because a further language, a further compiler and a second truth next to the
  generated YAML; error messages of the generators point to generated files.
* Bad, because AsyncAPI 3 output is not verified there [assumed].

## More Information

* docs/spec/06-api.md (conventions, code generation, gate), docs/api/README.md (commands).
* The owner's fixed build rule "API documentation is generated from code (OpenAPI from the routes)"
  counts here as "one source": the source is `docs/api/openapi.yaml`, routes, handler interfaces
  and clients are generated from it, and `make codegen-check` keeps both the same. No build session
  generates OpenAPI from handler code; the goal of the rule (no deviation between docs and routes)
  is thus met.
* oapi-codegen v2.8.0 release notes: https://github.com/oapi-codegen/oapi-codegen/releases
  (retrieved 2026-10-10) [verified].
* dart-dio generator: https://openapi-generator.tech/docs/generators/dart-dio/ (retrieved
  2026-10-10) [verified].
