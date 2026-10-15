# domain

Entities, value objects and pure rules of the apps, without Flutter (docs/spec/15-client-architektur.md,
1 principle 5 and 3.1 rule 1). No I/O, no clock and no randomness except through injected
functions, so every rule is tested without a device and runs in a background isolate.

Kind: pure Dart package, a member of the Dart workspace at the repository root
(`resolution: workspace`, `publish_to: none`). No runtime dependency; the tests also use
`package:yaml` to read the specification files.

## Contents

| Area | Files (`lib/src/`) | Source |
|---|---|---|
| Typed IDs | `ids.dart`: one extension type per kind (`SpaceId`, `ChannelId`, `ThreadId`, `MessageId`, `RunId`, `PrincipalId`, `FileId`, `DeviceId`, `HostId`, `TerminalSessionId`, `ComponentId`, `CommandId`, `ForumTagId`, `DraftId`, `QuietHoursId`); `parse` accepts the 36-character form in either case and its error names only the length | 03 |
| UUIDv7 | `uuid7.dart`: `Uuid7Generator` with injected clock and randomness, monotonic within a millisecond (RFC 9562 6.2 method 1) | 04 5.1 |
| Counters | `seq.dart`: `GlobalSeq`, `ThreadSeq`, `ResourceVersion`, `PermissionVersion` | 04 3, 4, 9 |
| Entities | `entities/`: immutable classes with `==`, `hashCode` and `copyWith`, only the columns a client sees; fields that need `manage` (channel prompt, skills) do not exist | 15 6.1, 03 |
| Enums | in the entity files; exactly the values of 03 and `docs/api/openapi.yaml` plus `unknown`, `fromCode` never throws; `test/enums_test.dart` fails when a value of the spec has no member | 03, 06 13 |
| Version rule | `rules/version_rule.dart`: `decideApply`, `decideApplyForEvent` | 04 9 item 3, 15 5 |
| Cursor rule | `rules/cursor_rule.dart`: `isDuplicate` | 04 9 item 2 |
| Read state | `rules/read_state_rule.dart`: `mergeReadState` (an older version changes nothing, a newer one never lowers the position; `reset: true` for the separate reset operation) | 04 9, 04 14 |
| Loaded ranges | `rules/loaded_ranges.dart`: `LoadedRanges` with `add`, `contains`, `gapsUpTo`, `fits` | 04 4, 15 7.3 item 4 |
| Unread counting | `rules/unread_counter.dart`: `UnreadCounter` from `message.*` events (an edit with the previous local copy), `resetFrom` the server values or the rule of 03 `read_states` | 04 14, 03 |
| Grouping | `rules/message_grouping.dart`: `isContinuation` with `ui.message_group_window` | 01 14 |
| Forum order | `rules/forum_order.dart`: `ForumOrder.sort` with the Done section | 01 6.1 |
| Cost labels | `rules/cost_label.dart`: `costLabel` | 01 6.7 |
| Event catalogue | `rules/event_catalog.dart` and the generated `event_catalog.g.dart`: `EventCatalog.lookup` | `docs/spec/actions.yaml` |
| App configuration | `config/app_config.dart` and the generated `app_config_keys.g.dart`: `AppConfig` | `docs/operations/configuration.md` section 4 |

## AppConfig

`AppConfig` holds every value of place `App` of the configuration reference and the keys of
place `Server` that steer the apps without an API that delivers them (docs/DECISIONS.md D-W-115
and D-W-116), each with the default of the reference. `AppConfig.defaults()` returns all of
them, one typed getter per key is named after the key without its first segment
(`outboxStuckAfter` for `client.outbox_stuck_after`), and `withOverrides` is the hook for
values the apps set later; it parses with the same parsers and rejects unknown keys and
malformed values with an error that names the key, never the value. Limits that the server
delivers in `InstanceLimits` (`getInstance`) are never in `AppConfig`.

Native code (the Notification Service Extension, the Android push service) reads values of
place `App` such as `client.push.fetch_timeout` too; it must read the same reference.

## Generators

| File | Generator | Regenerate | Gate |
|---|---|---|---|
| `lib/src/config/app_config_keys.g.dart` | `tools/configdoc/app_config.py` | `make app-config` | `make codegen-check` |
| `lib/src/rules/event_catalog.g.dart` | `tools/catalog/event_catalog_dart.py` | `make event-catalog` | `make codegen-check` |

Never edit a generated file by hand; the gate fails on a stale or edited file, and a value of
place `App` added to a chapter table without regeneration fails it too.

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): none: no arrow of the diagram leaves this package.
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).

## Layout

| Path | Content |
|---|---|
| `lib/domain.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/src/` | the sources listed above |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`); the enum, event catalogue and `AppConfig` tests read the specification files from the repository |

Run from the repository root: `make lint test coverage arch codegen-check COMPONENTS=apps`. The
API pages come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
