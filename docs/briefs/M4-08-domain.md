# M4-08: Domain package

Status: ready | Depends on: M4-01 | Milestone: M4

## Goal

After this ticket `packages/domain` holds the client's entities, value objects and pure rules in plain Dart without Flutter: typed identifiers and a UUIDv7 generator for the outbox IDs, the entities of the local schema mirror of 15 6.1 with only the columns a client sees, closed enums with an `unknown` member for values a newer server adds, the sync rules of chapter 04 that every other package must apply the same way (version rule, cursor rule, monotonic read state, gaps in `thread_seq`, unread and mention counting from message events), the display rules of 01 (message grouping, forum sort with the Done section, cost labels), and the app configuration table: every value of place `App` in `docs/operations/configuration.md` with its default, held equal to that reference by a test. Why: `local_store`, `sync_engine`, `auth`, `design_system` mappings and the apps all decide with these rules (15 1 principle 5, 15 3.1); written once and tested without a device, they cannot drift between packages.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-08, M4-12 to M4-16 (they build on this package).
- `docs/spec/15-client-architecture.md`: 1 (principles 2, 5), 3, 3.1 (rule 1), 5 "Data flow" (version rule and the `reaction.*` and `pin.*` exception), 6.1 "Schema mirror" (local tables and their notes), 7.3, 7.5, 19, 22 (the client values).
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope", 4 "Thread sequence", 5.1 (`client_msg_id` is a UUIDv7), 7.1 "Cursor", 9 "Delivery guarantee" (items 2 and 3 with the list of types without `resource.version`), 13 "Offline behavior of the client", 14 "Cross-device synchronization" (read state monotonic, client counts unread and mentions from `message.*` with `mention_ids`, drafts with `overwrote_newer`).
- `docs/spec/03-data-model.md`: the tables of 15 6.1 (`spaces`, `user_space_states`, `channels`, `forum_tags`, `threads`, `thread_tags`, `messages`, `message_attachments`, `files`, `reactions`, `mentions`, `message_components`, `runs`, `principals`, `agents`, `commands`, `read_states`, `drafts`, `user_settings`, `notification_preferences`, `quiet_hours`, `instance_settings`) and their enum columns; 6.5 (redaction).
- `docs/spec/01-product.md`: 3.3 (long replies), 6.1 (Done section), 6.7 ("Costs"), 11 (setting keys and defaults), 14 (configuration values); `docs/spec/13-push-notifications.md` 3.3 (`notify.*` keys and defaults, only to list them).
- `docs/spec/05-actions.md` section 6 (event types and which carry `resource.version`), `docs/spec/actions.yaml` (`events:` entries with `kind`, `resource`, `receive_action` and `resource_version`).
- `docs/operations/configuration.md` section 4 (every row with place `App`, the rows of the keys D-W-115 names; the column layout); `docs/api/openapi.yaml` schema `InstanceLimits` (limits the client reads from the server, never from `AppConfig`).
- `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` (enum schemas such as `TriggerState`, `RunState`, `ChannelKind`, `BillingMode`).
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115, D-W-116; section 10. Briefs: `docs/briefs/M4-01-workspace-arch.md`.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`. This ticket creates the client side of that rule: `AppConfig` (Design note 6) holds every key of place `App` and the server-place keys of D-W-115 and D-W-116 with the default of the reference, and every package reads values from it by key; code comments name the key, never the number.
10. Specification first: if an entity needs a field that chapter 03 or the API lacks, stop (Boundaries); this ticket changes only 15 (the sentence that names `AppConfig`) and adds no spec content.

Product requirements implemented here:

11. Pure Dart: no `package:flutter`, no `dart:ui`, no I/O, no clock and no randomness except through injected functions (15 1 principle 5, 15 3.1 rule 1).
12. Version rule: a change applies only if `resource.version` is greater than the local version; an event without `resource.version` applies in `seq` order; events with `seq` at or below the cursor are discarded (04 9 items 2 and 3).
13. Read state is monotonic; resetting is a separate operation (04 14, R-SY-13). Unread and mention counters are counted by the client from `message.created`, `message.updated` and `message.deleted` with `mention_ids`; the server delivers the computed values in the snapshot and `read_state.updated` (04 14).
14. `thread_seq` is gapless per thread; a gap means missing data (04 4); the loaded ranges detect it (15 6.1 `loaded_ranges`).
15. Outbox IDs are UUIDv7 created by the client (04 5.1).
16. Fields that need `manage` (channel prompt, skills) do not exist in the client entities (15 6.1).
17. Unknown enum values of a newer server decode to `unknown` and never throw (06 13 item 3).

## Scope

Create in `packages/domain/lib/src/`:

- `ids.dart` (typed IDs), `uuid7.dart` (`Uuid7Generator`), `seq.dart` (`GlobalSeq`, `ThreadSeq`, `ResourceVersion`, `PermissionVersion`).
- `entities/` one file per entity group: `space.dart`, `channel.dart`, `thread.dart`, `message.dart` (with `TriggerState`, `MessageKind`, `MessageOrigin`), `file.dart`, `reaction.dart`, `mention.dart`, `component.dart`, `run.dart` (summary and `RunState`, `StepKind`, `StepStatus`), `principal.dart` (`Principal`, `AgentCard`, `BillingMode`, `AgentPresence`), `command.dart`, `personal.dart` (`ReadState`, `Draft`, `UserSetting`, `NotificationPreference`, `QuietHours`), `instance.dart` (`InstanceInfo` with branding and evaluated feature flags).
- `rules/` : `version_rule.dart`, `cursor_rule.dart`, `read_state_rule.dart`, `loaded_ranges.dart`, `unread_counter.dart`, `message_grouping.dart`, `forum_order.dart`, `cost_label.dart`, `event_catalog.dart` (Design notes 3 to 5).
- `config/app_config.dart` and the generated `config/app_config_keys.g.dart` (Design note 6).
- `lib/domain.dart` exports.
- Tests under `test/` (Tests) and `tools/configdoc/tests/test_app_config.py` or a Dart test reading the reference (Design note 6).

Change:

- `packages/domain/pubspec.yaml` (no runtime dependency except `meta` and `collection` if needed), `README.md`.
- `docs/spec/15-client-architecture.md` 22 (one sentence: the app reads its values through `AppConfig` in `packages/domain`, held equal to the configuration reference).
- `docs/DECISIONS.md` section 10: rows `D-M4-08-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Persistence (M4-12), mapping from `api_client` models (M4-12 and M4-14 map into these entities), the sync state machine (M4-13), run documents and their patches (M4-16), notification level resolution (M4-33 uses 13's rules), permission logic of any kind (15 1 principle 1).
- Overriding `AppConfig` values from the server or a build configuration: the apps decide later; this ticket provides the table and the override hook only.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-08-domain`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Identifiers and UUIDv7.** One extension type or small final class per ID kind (`SpaceId`, `ChannelId`, `ThreadId`, `MessageId`, `RunId`, `PrincipalId`, `FileId`, `DeviceId`, `HostId`, `TerminalSessionId`, `ComponentId`, `CommandId`) wrapping the canonical lowercase string; `parse` accepts the 36-character form in either case and rejects anything else with a `FormatException` that does not echo the input beyond its length. `Uuid7Generator({required int Function() nowMillis, required int Function(int bits) random})` produces RFC 9562 version 7 UUIDs with a monotonic counter within the same millisecond (method 1 of RFC 9562 6.2: a 12-bit `rand_a` counter seeded randomly, incremented within a millisecond, and on overflow the timestamp advanced by one), so IDs sort in creation order on one device.

2. **Entities.** Immutable classes with `==`, `hashCode`, `copyWith` and a `version` where the server table has one; fields named as in 03 in `lowerCamelCase`; optional fields as nullable Dart fields (the wire has no `null`, 04 2.2; absence maps to `null` in Dart). `Message` has `threadSeq`, `trigger_state` as `TriggerState`, `deletedAt`, `editedAt`, `contentTruncated`, `mentionIds`; it has no `clientPayloadSha256` (15 6.1). `Channel` has no `prompt`, no skills and no project binding details beyond what `channel.read` returns. Enums: one per enum column or schema with exactly the values of 03 and the API plus `unknown`; `fromCode(String)` never throws. A test reads the enum values from `docs/api/openapi.yaml` (walk up to the repository root) and fails when a value of the spec has no member.

3. **Sync rules.**
    - `ApplyDecision decideApply({required ResourceVersion? local, required ResourceVersion? incoming, required bool typeCarriesVersion})`: `apply` when the type carries a version and `incoming > local` or `local` is absent; `ignoreStale` when `incoming <= local`; `applyInSeqOrder` when the type carries none. `typeCarriesVersion` comes from `EventCatalog`, generated from `docs/spec/actions.yaml` (`events:` with `kind` and `resource_version`) into `event_catalog.g.dart` with a gate that compares (the generator in `tools/catalog/` if M1-02's tool offers an output hook, otherwise a small Python script in `tools/catalog/` with tests; record which). An event type unknown to the catalogue is `unknown` and skipped by the caller.
    - `bool isDuplicate(GlobalSeq seq, GlobalSeq? cursor)`: true when `cursor != null && seq <= cursor`.
    - `ReadState mergeReadState(ReadState local, ReadState incoming)`: `lastReadSeq` is the maximum unless `incoming` is an explicit reset with a newer version (the separate operation of 04 14); counters from the incoming computed values.
    - `LoadedRanges` (immutable): `add(ThreadSeq from, ThreadSeq to)` merges overlapping and adjacent ranges; `contains(seq)`; `gapsUpTo(ThreadSeq lastMessageSeq)` lists missing ranges; `fits(ThreadSeq seq)` tells whether a new message continues a loaded range or opens a gap (15 7.3 item 4).
    - `UnreadCounter`: `apply(MessageEvent e, {required PrincipalId me, required ThreadSeq lastReadSeq})` updates `unread` and `mentions` per thread for `message.created` (not own, above the read state; mention when `mentionIds` contains `me`), `message.updated` (mention added or removed above the read state), `message.deleted` (decrement if it was counted); `resetFrom(ReadState)` takes the server's computed values. Imported content never counts (`unread_floor_seq`, 04 14).

4. **Display rules.** `bool isContinuation(Message previous, Message next, Duration window)` (same author, same kind class, `next.createdAt - previous.createdAt < window`, neither is a system message); window from `ui.message_group_window`. `ForumOrder.sort(List<Thread>, ForumSort sort)` returns open threads by `lastActivityAt` or creation descending and a separate Done list (closed threads) in the same order (01 6.1, components.md 6). `CostLabel costLabel({required BillingMode mode, required bool isEstimate})`: `estimate` for organization while `cost_is_estimate`, `final` after reconciliation, `informationOnly` for personal (01 6.7).

5. **Event catalogue.** `EventCatalog.lookup(String type) -> EventInfo?` with `durable`, `carriesResourceVersion`, `resourceKind`, `personal` (audience self). Generated, never edited by hand (Design note 3).

6. **App configuration.** Ruling for docs/DECISIONS.md: `AppConfig` is the client's single source of the values of place `App` and of the server-place keys that steer the client's own behaviour without an API that delivers them (D-W-115: `sync.reconnect_backoff_base`, `sync.reconnect_backoff_cap`, `sync.typing_min_interval`, `sync.typing_ttl`, `sync.max_focused_threads`, `client.cache_max_age`, `client.outbox_retry_base` and `client.outbox_retry_cap`; D-W-116: `auth.device_pending_ttl`; the generator keeps this list as a constant with the reason, and a test fails when a listed key is missing from the reference or changes its place). Limits that `InstanceLimits` of `getInstance` delivers (`file_max_upload_bytes`, `voice_max_duration_ms`, the message limits) are never in `AppConfig`; the apps read them from the server. `app_config_keys.g.dart` is generated from the rows of `docs/operations/configuration.md` section 4 with place `App` and the listed rows (key, kind derived from the default: duration, int, bytes with units `KiB`, `MiB`, `GiB`, rate `kbps` and `MiB/s`, float, px, list); `AppConfig.defaults()` returns all of them; `AppConfig.withOverrides(Map<String, String> raw)` parses with the same parsers and rejects unknown keys and malformed values (a `FormatException` naming the key, not the value). Typed getters per key (`Duration outboxStuckAfter`, `int quickSwitchMaxResults`, ...) named after the key. The generator is a small Python tool next to `tools/configdoc/` (standard library, ruff `D`, pytest) with a `--check` mode wired into `make codegen-check`; a value of place `App` that is added to a chapter table without regeneration fails the gate. Values of place `App` that the native extensions also read (for example `client.push.fetch_timeout`, which the Notification Service Extension reads too) are included in `AppConfig` like every other value of place `App`; the README notes that native code must read the same reference.

7. **Rulings for docs/DECISIONS.md** (`D-M4-08-<n>`): typed IDs and the monotonic UUIDv7 counter (note 1); enums with `unknown` and the spec test (note 2); the generated event catalogue (note 3); the forum and grouping rules (note 4); `AppConfig` generated from the reference (note 6).

## Tests (write them first)

`packages/domain/test/` (`package:test`):

- `uuid7_test.dart`: version and variant bits; 1000 IDs in the same millisecond sort strictly increasing; the timestamp advances on counter overflow; `parse` rejects 35 and 37 characters and braces.
- `enums_test.dart`: every enum value of the spec schemas has a member; `fromCode('rocket')` is `unknown`.
- `version_rule_test.dart`: newer applies, equal and older are ignored, absent local applies, a type without version gives `applyInSeqOrder` (`reaction.added`, `pin.added`, `read_state.updated`), an unknown type is unknown.
- `cursor_rule_test.dart`, `read_state_rule_test.dart` (never decreases on a normal update; an explicit reset with a newer version lowers it).
- `loaded_ranges_test.dart`: merge adjacent and overlapping ranges; gaps; `fits` at the edge and with a gap.
- `unread_counter_test.dart`: own message not counted; message at or below the read state not counted; mention added by an edit; deleted message decrements; imported message below `unread_floor_seq` not counted; `resetFrom` takes the server values.
- `message_grouping_test.dart`, `forum_order_test.dart`, `cost_label_test.dart`.
- `event_catalog_test.dart`: the generated catalogue equals `docs/spec/actions.yaml` (every durable type present with its `resource.version` flag).
- `app_config_test.dart`: `defaults()` contains every key of place `App` of the reference and every key of D-W-115 and D-W-116 with the reference default (the test reads `docs/operations/configuration.md`); `withOverrides` rejects an unknown key and a malformed duration; `outboxStuckAfter` parses `2m` as two minutes.
- Generator tests (pytest) for the `AppConfig` and event catalogue generators: deterministic output; `--check` fails on a stale file.

Commands that must pass, from the repository root:

- `cd packages/domain && flutter test --coverage` (pure Dart tests through the root `Makefile` rules)
- `make codegen-check` (the gate that holds the generated files), `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] `dart test` (through `make test`) is green; `domain` imports no `package:flutter` (`make arch`).
- [ ] The version, cursor, read state, loaded range and unread rules match 04 9 and 04 14 with the tests listed.
- [ ] Every enum of the spec has a member and an `unknown` fallback.
- [ ] `AppConfig` contains every key of place `App` and the keys of D-W-115 and D-W-116 with the reference default; a stale generation fails the gate.
- [ ] The event catalogue is generated from `actions.yaml` and checked.
- [ ] Rulings recorded; 15 22 names `AppConfig`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and name the chapter and section of each rule.
- `packages/domain/README.md` (English): entities, rules with their sources, `AppConfig`, the generators.
- Rulings of Design note 7 in `docs/DECISIONS.md` section 10; 15 22 updated.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The number of entities, enums, rules and `AppConfig` keys; where the two generators live.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
