---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team B (API), Team E (apps), Team D (agent platform)
informed: all planning teams
---

# No JSON null in API and events

## Context and Problem Statement

Many fields are optional: a column with NULL, an event without an actor, a run without a session.
JSON has two forms for this: the field is missing, or it is present with `null`. All clients are
generated from one specification (ADR 0013): Go server, TypeScript, Python and Dart. The
Dart generator `dart-dio` does not support the null type [Q1, verified]. How do the API, the
gateway and webhooks express "no value", and how does a client remove a value?

## Decision Drivers

- All four generators produce working code from the same specification (gate G4 in
  [06-api.md](../spec/06-api.md)).
- One form for "no value" in responses, requests and events.
- No three-valued logic in changes (missing, `null`, value) that every client must interpret
  correctly.
- Conservative schemas: no type array, no `anyOf` (06, section 14).
- Unknown and unexpected input is tolerated and does not lead to a crash (06, section 2).

## Considered Options

- No JSON `null` on the wire; a missing field means "no value"; removal through separate
  fields or modes
- `null` as an explicit value with the OpenAPI 3.1 type `[T, "null"]` and changes following JSON
  Merge Patch (`null` means delete)
- Placeholder values for "no value" (empty string, 0, epoch time)
- Wrapper objects for optional fields (`{"value": ...}`)

## Decision Outcome

Chosen option: "No JSON `null`; missing means no value", because it works with all generators
and makes changes unambiguous. Normative in
[04-sync-protocol.md](../spec/04-sync-protocol.md), section 2.2 (R-SY-16),
[03-data-model.md](../spec/03-data-model.md), principle 13, and
[06-api.md](../spec/06-api.md), section 2 (R-API-03).

1. The server never sends JSON `null`, neither in REST responses nor in gateway frames, envelopes,
   payloads or webhooks. A column with NULL appears as a missing field.
2. Required fields are always present, also with zero values such as `0`, `false` or `""`. Only
   optional fields may be missing.
3. Values are removed through separate fields or modes, never through `null`: for example
   `use_global_order: true` removes the own order in the sidebar, and `OperatorSet.mode =
   none` sets "no operator". In changes a missing field means "unchanged".
4. Requests with `null` fail the schema validation before the handler (SEC-IN-1 in
   [19-security.md](../spec/19-security.md)), because no schema allows the null type.
   Adapters and apps omit a field instead of sending `null`.
5. Clients treat a received `null` like a missing field (tolerance, 06 section 2).

### Consequences

- Good, because the types in Dart, TypeScript, Python and Go are generated without special cases.
- Good, because a change knows exactly two cases (field present: set; field missing: leave) and
  removal is visible and auditable.
- Bad, because every removable field needs its own mode or its own field; the API
  becomes somewhat wider at these places.
- Bad, because Go code must take care: with `omitempty`, zero values of required fields
  disappear too. Therefore only optional fields carry `omitempty` (as pointers or with
  `omitzero`), required fields never; the response validation (G8) finds missing required fields.
- Neutral, because the database keeps using NULL; the boundary lies in the encoding to the
  outside.

### Confirmation

- Lint rule `rule/no-type-arrays-or-null` in `docs/api/redocly.yaml` (gate G1): no schema with
  a null type or a type array.
- Property test in build ticket M1-18: `TestNoNull_PropertyRandomValues` and `FuzzSanitizeJSON`
  in `services/server/internal/platform/httpserver` (the response writer removes `null` members
  and rejects `null` elements and integers above 2^53 - 1 with `500`; `TestNoNull_NullInArrayGives500`,
  `TestInt53_AboveLimitGives500`, `TestNoNull_CleanBodyByteIdentical`), plus the harness
  `httpservertest`, which fails every handler test with `null` in the response; a test in build
  ticket M1-16: no envelope contains JSON `null` (built: `FuzzEnvelopeHasNoJSONNull` and
  `TestEnvelope_FieldsMatchOpenAPI` in `services/server/internal/sync`; the appender rejects a
  payload with JSON `null` at any depth, `TestAppend_Validation`).
- Gate G7 (Schemathesis) with its own check "no `null`" and gate G8 (response validation in
  integration tests) in [06-api.md](../spec/06-api.md), section 15.
- Tolerance test G5: the hand-written layer of every SDK skips a received `null` without an error.
- Test: a request with `null` in an optional field returns `400` with `validation_failed`.

## Pros and Cons of the Options

### No `null`, missing means no value

- Good, because all generators can represent it and Go handles it directly with `omitempty` or
  `omitzero`.
- Bad, because "remove a value" is not generic but is designed per field.

### `null` with JSON Merge Patch

- Good, because Merge Patch has a well-known rule: `null` removes a value [Q2, verified].
- Bad, because `dart-dio` does not support the null type in the schema [Q1, verified]; the option
  `useOptional` does distinguish missing, `null` and value [Q1, verified], but helps only in the
  Dart client. Type arrays are supported to different degrees in the generators (oapi-codegen
  only since v2.8.0, 06 source Q1).
- Bad, because every client must correctly distinguish three states (missing, `null`, value); a
  client that treats `null` and "missing" the same deletes or keeps values unintentionally.

### Placeholder values

- Good, because every field is always present.
- Bad, because `0`, `false` and `""` are often real values (counters, switches); placeholders
  confuse "no value" with a valid value.

### Wrapper objects

- Good, because unambiguous.
- Bad, because every optional field becomes an object; schemas, examples and generated code
  become considerably larger without any generator requiring it.

## More Information

- Sources (retrieved 2026-10-10): Q1 openapi-generator `dart-dio`,
  https://openapi-generator.tech/docs/generators/dart-dio/ [verified]; Q2 RFC 7396 JSON Merge
  Patch, section 1, https://www.rfc-editor.org/rfc/rfc7396 [verified].
- `omitzero` exists in `encoding/json` since Go 1.24 (https://go.dev/doc/go1.24, retrieved
  2026-10-10) [verified].
- Related: ADR 0013 (spec-first), ADR 0015 (error format), ADR 0026 (SDKs), ADR 0052 (envelope).
