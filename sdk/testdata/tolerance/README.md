# sdk/testdata/tolerance

Shared fixtures of gate G5 (docs/spec/06-api.md 15): both SDKs read the same files, so both prove
the same cases. Each file is a complete `EventPage` body of `listEvents`, valid against
`docs/api/openapi.yaml` except for the one deliberate unknown it is named after. A change here
selects both SDK components in CI (`tools/ci/components.py`).

| File | Content | Read by |
|---|---|---|
| `unknown_field.json` | one `thread.updated` event with an extra member on the page, the envelope, the payload and the `thread` (an object value) | `test_g5_unknown_field_is_ignored`, `G5: unknown field is ignored` |
| `unknown_enum_value.json` | one `thread.updated` event with values outside the spec for `thread.state` (`ThreadState`), `thread.kind` (inline enum) and `actor.kind` (`ActorKind`) | `test_g5_unknown_enum_value_keeps_raw_value`, `G5: unknown enum value keeps its raw value` |
| `unknown_event_type.json` | `thread.updated`, then type `future.example` whose payload fits `ThreadMovedOutPayload`, then `thread.updated`; `next_cursor` and `head_seq` equal the last `seq` | `test_g5_unknown_event_type_is_skipped`, `G5: unknown event type is skipped` |
| `null_member.json` | one `thread.updated` event with `null` for `correlation_id` and `thread.closed_at` | `test_g5_null_member_is_treated_as_absent`, `G5: null member is treated as absent` |

The polling tests (`pollAgentEvents`) read all four. The tests check that the unknown names and
values really are unknown to the spec (Python by reading the spec, TypeScript with
`@ts-expect-error`), so they fail instead of passing silently once the spec gains one of them.
Python tests live in `sdk/python/tests/`, TypeScript tests in `sdk/typescript/test/`.
