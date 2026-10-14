# M8-03: Import interface in the server

Status: ready | Depends on: M2-09 | Milestone: M8

## Goal

After this ticket the server has the source-neutral import interface of 22 8 for everything except the content of threads: the module `importer` owns `import_sources` and `import_items`; owners list, create (with the mapping of schema `import_source.discord.v1`, validated field by field), read, start, pause, cancel and complete a source with step-up and audit, and change the ownership list per channel (`setImportLead`), which sets `channels.read_only` of the target channel in the same transaction and cancels active runs there when it becomes read-only; `channel.read_only.set` (`setChannelReadOnly`) exists as the general owner switch of 05 rule 1.8. `applyImportBatch` writes structure batches (spaces, channels, forum tags, people) of a `running` source as one transaction: per item it answers an existing `posted` origin key with its existing target and no event, otherwise it writes `pending`, creates the target through the port of the owning module and sets `posted` in the same transaction, so a repeated batch and a batch that was stored before a `502` produce no duplicate (22 7.2). Unmapped and webhook authors become disabled placeholder people that can never log in or trigger (22 5.3, R-IM-06). New channels get no grants and are read-only while Discord leads (22 5.1 rules 2 and 3). `setImportCursor` stores the tool's cursor and counters. Every write emits `import.progress`. Thread, message, pin and file items stay `not_enabled` until M8-11. Why: the import tool (M8-04) writes only through this interface and the owner steps of `import.manage` run in the web console (M8-08), because a `cli` session can never provide step-up (08 6.4, 22 8, D-W-124).

## Read first

- `docs/spec/22-import-migration.md`: 1, 2, 5 (5.1 with rules 1 to 4, 5.2 mapping file, 5.3 people and placeholders, 5.4 agents), 6.6 (forum tags), 7 (7.1 origin keys, 7.2 `pending` and `posted`, 7.3 cursor and the run lease, 7.4), 8 (operations, `import.apply` conditions, the `cli` sentence), 11 (stats), 12.1 (ownership list, read-only), 13.3 to 13.5 (lead changes, completion), 14, 16 (rows "Stored, then error", "Read-only in the mirror"), 17, 18 (K-IM-1, K-IM-2, K-IM-8).
- `docs/spec/03-data-model.md` (core contract, read only): `import_sources`, `import_items` and the table after them ("What the import sets differently"), `channels.read_only`, `principals` and `users` (`kind`, `status`, handle rules), `forum_tags`, 4.11, R-DM-25.
- `docs/spec/05-actions.md` (core contract, read only): rule 1.8 (read-only channels), 2.4 (step-up), 4.12 rows `import.read`, `import.manage`, `import.apply`, the row `channel.read_only.set`, 6 (row `import.progress`), 10 (`rl.import`), R-AZ-12.
- `docs/spec/04-sync-protocol.md` 15.8 (writes of an import; here only `import.progress`), "Delivery guarantee".
- `docs/spec/08-identity.md` 6.4 (`cli` sessions have no step-up), 12 (personal tokens have no owner rights).
- `docs/spec/06-api.md`: 7 (idempotency; `applyImportBatch` is exempt by origin key), the table of operations (rows of the tag Import), 5 (problem codes).
- `docs/spec/20-privacy.md`: retention classes `import` and `account` (placeholders are personal data).
- `docs/api/openapi.yaml`: tag `Import` (`listImports`, `createImport`, `getImport`, `listImportItems`, `startImport`, `pauseImport`, `cancelImport`, `completeImport`, `setImportLead`, `applyImportBatch`, `setImportCursor`), `setChannelReadOnly`, schemas `ImportSource`, `ImportSourceCreate`, `ImportLeadSet`, `ImportBatch`, `ImportObject`, `ImportBatchResult`, `ImportItem`, `ImportCursorSet`, `ChannelReadOnlySet`; `docs/api/asyncapi.yaml` `import.progress`.
- `docs/adr/0043-discord-import-read-only-dry-run-origin-key.md`.
- `docs/DECISIONS.md`: D-IM-01 to D-IM-08, D-LS-17, D-LS-19, D-K-I1, D-K-I8, D-K-X8, D-W-100, D-W-120, D-W-124; section 10 (format of `D-M8-03-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-53-chat-channels.md` (channels, `read_only` column, `forum_tags`, "Setting `read_only`: M8-03"), `M1-31-identity-people.md` (people, handles), `M1-26-identity-principals.md`, `M1-28-authz-core.md` (rule 1.8 in `authorize`, owner checks, step-up), `M1-24-audit-chain.md` (`auditing.Recorder`), `M1-13-unit-of-work.md` (`txn`, in-tx events), `M1-16-event-log.md`, `M1-19-rate-limits.md` (`rl.import`), `M1-20-idempotency-keys.md`, `M2-09-run-lifecycle.md` (`requestCancel`, the planned `chat.ChannelReadOnlySet` subscription with reason `channel_read_only`), `M1-56-messaging-send.md` (the FK `messages.import_item_id` left to M8).
- Code on the base branch (all under `services/server/`): `internal/importer` (skeleton), `internal/chat`, `internal/identity`, `internal/runs`, `internal/authz`, `internal/wire`, `architecture/modules.yaml`, `api/operations.yaml`, `migrations/`.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Discord IDs are invented digit strings.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. All import routes are for owners only, through `authorize`; `import.manage` needs step-up and writes audit; `import.apply` needs an active owner, a `running` source and a target in a channel of this source, no step-up (05 4.12, 22 8). A personal token of an owner gets `403` (08 12, SR5).
12. Idempotency by origin key: `(source_id, origin_key)` is unique; a `posted` key returns its target without a duplicate and without an event; otherwise `pending`, target and `posted` in one transaction (22 7.2, R-IM-04).
13. One batch is one transaction with at most the `maxItems` of `ImportBatch`; structure items come in batches without `thread_origin_key` (22 8).
14. The tool only creates: no item deletes, archives or renames anything; an existing space or channel is used only when the stored mapping names it (22 5.1 rule 1).
15. New channels get no grants and `read_only = (lead = discord)` from the mapping; no operator is set by the import (22 5.1 rules 2 and 3, 5.4).
16. Placeholder people: `kind = user`, `status = disabled`, no identity, no grants, handle `imported-<name>` with a counter on collision, `display_name` from `global_name` or `username`; they never log in and never trigger (22 5.3, R-IM-06).
17. The ownership list is the only switch of the read-only state of imported channels: `setImportLead` changes `lead` in the stored mapping and `channels.read_only` in one transaction; read-only on cancels active runs in the channel (22 12.1, 13.3, 13.4, 05 `channel.read_only.set`).
18. Read-only applies to every principal including owners; only `import.apply` writes there (05 rule 1.8, R-AZ-12).
19. Completion only when every mapped channel that was imported (its channel item is `posted`) has `lead = bothy`; mapped channels never imported do not block it (22 13.5).
20. `import.progress` after every committed change of a source; no content and no Discord IDs in logs or telemetry (22 14).

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `migrations/<next>_import.sql`: tables `import_sources` and `import_items` exactly as 03 (columns, checks, defaults, FKs, unique `(source_id, origin_key)`, indexes `(state)`, `(source_id, state)`, `(target_kind, target_id)`); and the FK `messages.import_item_id -> import_items(id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED` if the merged migrations deferred it (read them first; never edit a merged migration). `make migrations-lock`.
- `internal/importer` root package: the port `Imports` (Design note 1), DTOs, the in-tx event `SourceChanged`.
- `internal/importer/domain/`: source state machine (note 3), mapping model and validation of `import_source.discord.v1` (note 2), origin key rules (note 6), the placeholder handle rule (note 8), stats merge.
- `internal/importer/app/`: use cases `ListSources`, `CreateSource`, `GetSource`, `ListItems`, `Start`, `Pause`, `Cancel`, `Complete`, `SetLead`, `ApplyBatch`, `SetCursor`.
- `internal/importer/adapters/postgres/` (sqlc queries and repository), `adapters/httpapi/` (handlers of the eleven operations).
- Ports in the owning modules (root packages, implemented in their `app`, Design note 7): `chat.ImportStructure` (create or adopt a space, create or adopt a channel with `read_only`, create or adopt a forum tag by name, `SetReadOnly`), `identity.ImportPeople` (resolve a mapped person or agent by handle, create a placeholder).
- `internal/chat` use case and handler `setChannelReadOnly` (`channel.read_only.set`), and the in-tx event `chat.ChannelReadOnlySet{ChannelID; ReadOnly bool}`; `internal/runs` subscriber `OnChannelReadOnlySet` that calls `requestCancel` with reason `channel_read_only` for every active run of the channel when `ReadOnly` is true (M2-09 note 7g).
- Tests named under "Tests".

Change:

- `api/operations.yaml`: enable `listImports`, `createImport`, `getImport`, `listImportItems`, `startImport`, `pauseImport`, `cancelImport`, `completeImport`, `setImportLead`, `applyImportBatch`, `setImportCursor`, `setChannelReadOnly` (`createImportFile` stays in `later` with ticket M8-11); regenerate.
- `docs/api/openapi.yaml` (Design notes 2, 4, 5 and 9): component schemas `ImportSourceDiscordConfig`, `ImportSpaceData`, `ImportChannelData`, `ImportForumTagData`, `ImportUserData`; `ImportObject.data` description naming them; `ImportItem` gains the optional member `source_changed` (boolean); `ImportSource` gains the member `cursor` (`JsonObject`, the object last stored by `setImportCursor`, returned by `getImport`, `listImports` and `setImportCursor` to callers with `import.read`), because the tool must read its cursor and run lease back in a new process (22 7.3, 7.4); descriptions of the state operations with their `409` reasons; `make api-lint`, `make api-checks`, `make codegen-check`.
- `architecture/modules.yaml`: `importer.depends_on` stays as registered (`identity`, `chat`, `agents`, `messaging`, `files`); add the in-tx event wiring if the register lists in-tx events; `make register-check`, `make arch`.
- `internal/wire`: wiring in roles `api` and `all`, the `runs` subscription.
- Docs: `docs/spec/22-import-migration.md` 7.2 (the `source_changed` flag of note 5) and 8 (the `409` reasons of note 3, the schema names of note 2); `internal/importer/README.md`, the READMEs of `chat`, `identity`, `runs`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M8-03-1` to `D-M8-03-9`.

This ticket adds no action, no event type, no configuration key and no top-level make target.

## Out of scope

- Thread, message and pin items, `thread_origin_key` batches, `unread_floor_seq`, `thread.updated` per batch and `createImportFile` (M8-11): until then `applyImportBatch` answers `400 validation_failed` with `errors[].code` `not_enabled` at `/items/<i>/item_kind` for `thread`, `message` and `pin`, and at `/thread_origin_key` when it is present.
- Earlier session rows (M8-06), the tool (M8-04, M8-10), the console screens (M8-08).
- Deleting `import_items` by retention (M7, class `import`); the privacy export of placeholders (M7-06 through `personaldata`).
- The source kind `archive` (`400 validation_failed`, `not_enabled`).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-03-import-server`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(importer): ...`, `feat(chat): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Never contact Discord or any other real service: the server never calls Discord (R-IM-01), and every test uses only fixtures and fakes (D-W-124).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (`t.TempDir()`), removed at the end.
- Never read or print secrets. Logs carry no Discord ID, no name and no content.
- Time only through `kernel/clock`; tests use a fake clock.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Port and boundaries** (02 4.2 row `importer`; Ruling for docs/DECISIONS.md). `importer` owns both tables; only it reads or writes them. It creates objects only through ports of the owning modules (note 7), inside the batch's unit of work (`txn`), so one batch is one transaction across modules. The port `Imports` exposes `SourceOfTarget(ctx, kind, id)` for "where does this object come from" (index `(target_kind, target_id)`); the console and M8-11 use it.

2. **Mapping schema `import_source.discord.v1`** (22 5.2; Ruling for docs/DECISIONS.md). `ImportSourceCreate.config` for `kind = discord` must validate against the new component `ImportSourceDiscordConfig` (JSON of the YAML of 22 5.2; the console converts YAML to JSON, M8-08): `version` = 1; `guild_id` digits; `defaults` with `unmapped_human` and `unmapped_bot` in `placeholder|skip` and `uncategorized_space` either `skip` or the `discord_category` of an entry in `spaces`; `spaces[]` with `discord_category` and `target` exactly one of `{create: {name}}`, `{existing: <uuid>}`, `"skip"`; `channels[]` with `discord_channel`, `target` (same three forms; `create` has `name`), `planned_operator` (`agent:<handle>` or `none`), `lead` (`discord|bothy`, default `discord`); `users[]` with `discord_user` and `target` (`user:<handle>`, `agent:<handle>`, `placeholder`). Unknown members, a duplicate Discord ID within a list, a non-digit ID, an unknown handle, an agent that is not active, an `existing` UUID that is not a space or channel, and a name longer than `channel.name_max_chars` (for spaces `space.name_max_chars`) are each one `400 validation_failed` error with a JSON pointer; all errors are reported at once. The server stores the normalized JSON in `import_sources.config`; it never holds a secret (no token member exists in the schema).

3. **State machine** (03 `import_sources.state`; 22 7.4, 13.5; Ruling for docs/DECISIONS.md). `draft -> running` (`startImport`, sets `started_at` on the first start), `paused -> running` (`startImport`), `running -> paused` (`pauseImport`), `draft|running|paused -> failed` (`cancelImport`, sets `finished_at`; the enum has no `cancelled`, so a cancelled source is `failed` with the stats member `cancelled: true`), `running|paused -> completed` (`completeImport`, only when every channel of the mapping whose target is not `skip` and whose channel item is `posted` has `lead = bothy`, else `409 conflict` with `reason` `lead_not_bothy`; sets `finished_at`). A transition from any other state is `409 conflict` with `reason` `import_state`. Repeating the transition that already happened (start on `running`, pause on `paused`) answers `200`/`202` with the source unchanged, no audit entry beyond the request's own, no event (06 2 "No effect"). `running` says that the owner allows imports, not that a tool run is active (22 7.4).

4. **`setImportLead`** (22 12.1, 13.3, 13.4; Ruling for docs/DECISIONS.md). Body `ImportLeadSet{source_channel, lead}`. The channel must be in the mapping with a target other than `skip` (else `404`), and the target channel must exist: its item `channel:<id>` is `posted` (else `409 conflict` with `reason` `channel_not_imported`). In one transaction: update `lead` of that entry in `import_sources.config`; call `chat.ImportStructure.SetReadOnly(channel, lead == discord, reason "import lead")`, which writes `channels.read_only`, raises the channel version, appends `channel.updated` and the in-tx event `chat.ChannelReadOnlySet`; `runs` cancels active runs in the channel when it becomes read-only; audit `import.manage` with details `import_id`, `channel_id`, `lead`; append `import.progress`. A lead equal to the current one changes nothing. The source may be in any state except `completed` and `failed` (`409 conflict` with `reason` `import_state`). The operator is not touched (the owner sets it with `channel.operator.set`, 22 13.3 step 3).

5. **`applyImportBatch`** (22 7.2, 8; 06 7 exemption by origin key). Authorize `import.apply` on the source (owner session, source `running`, else `409 conflict` with `reason` `import_not_running`; a `paused` source answers the same). Per item in order, inside one unit of work:
    a. Validate the origin key form against `item_kind` (note 6) and `data` against the item kind's schema (note 9); errors are `400 validation_failed` at `/items/<i>/...`; the whole batch is rejected and nothing is written.
    b. `INSERT ... ON CONFLICT (source_id, origin_key) DO NOTHING RETURNING` a `pending` row; when the row exists, lock it (`FOR UPDATE`; a concurrent batch with the same key waits for the other's commit, 22 7.4) and read it: `posted` returns its target; when `content_sha256` is given and differs from the stored value, the item is returned with `source_changed = true` and the stats counter `source_changed` is raised, and nothing else changes (22 7.2 item 4); `skipped` returns the skipped item.
    c. New row: create or adopt the target (note 7), set `target_kind`, `target_id`, `content_sha256`, `state = posted`, `updated_at`; a user item whose mapping resolves to `skip` is stored as `skipped` without a target.
    d. A target that is not in a channel of this source (an `existing` channel not named in the mapping, a forum tag of an unmapped channel) is `409 conflict` with `reason` `target_outside_source` (05 4.12 `import.apply`).
    The response is `ImportBatchResult.items` in request order. After the items: update `import_sources.stats` counters per item kind and state, append one `import.progress`. Structure creations emit the normal events of their module (`space.created`, `channel.created`, `channel.updated`), because structure is small and clients must see new channels; `thread.updated` per batch is M8-11. The handler records a denial audit entry for refused `import.apply` (05 Audit `denials`).

6. **Origin keys** (22 7.1). `space:<digits>`, `channel:<digits>`, `forum_tag:<channel digits>:<tag digits>` (a tag ID is only unique within its channel in the data the tool sees; Ruling for docs/DECISIONS.md), `user:<digits>`; the item kind must match the prefix; at most the length the column allows by convention of 06 (reject above 128 bytes). `thread:`, `message:`, `pin:`, `file:` keys are M8-11.

7. **Ports of the owning modules** (Ruling for docs/DECISIONS.md: the import creates through ports, never through SQL of another module, 03 principle 12).
    - `chat.ImportStructure.Space(ctx, ImportedSpace{Name; SortKey; ExistingID *ids.ID}) (ids.ID, error)`: create a space like `space.create` without the permission check (the import is authorized by `import.apply`) or verify that `ExistingID` exists and is not archived.
    - `Channel(ctx, ImportedChannel{SpaceID; Name; Kind text|forum; Topic; SortKey; ReadOnly bool; ExistingID *ids.ID}) (ids.ID, error)`: create with the main thread for `text` (R-DM-11), `read_only` as given, no grant, no operator; a new name equal to the name of an existing channel in that space that the import did not create is `409 conflict` with `reason` `name_collision` (22 5.1 rule 1; the tool checks this in its dry run, the server enforces it); an adopted channel gets `read_only` set as given through `SetReadOnly`.
    - `ForumTag(ctx, channelID, ImportedTag{Name; Emoji}) (ids.ID, error)`: an existing tag with the same name (case-insensitive) is adopted, never duplicated (22 6.6).
    - `SetReadOnly(ctx, channelID, readOnly bool, reason string) error`: shared by `setChannelReadOnly` and `setImportLead`.
    - `identity.ImportPeople.Resolve(ctx, Ref{Kind user|agent; Handle}) (ids.ID, error)` and `Placeholder(ctx, PlaceholderSpec{DisplayName; NameHint}) (ids.ID, error)` (note 8).

8. **Placeholders** (22 5.3; Ruling for docs/DECISIONS.md). `kind = user`, `status = disabled`, no `user_identities`, no `local_credentials`, no grants, no group membership; `display_name` = `global_name` or else `username`, cut to `principal.display_name_max_chars`; handle `imported-` plus the name lowercased, every character outside the handle alphabet of 03 replaced by `-`, runs of `-` collapsed, trimmed, cut so that the handle with a counter suffix `-2`, `-3`, ... fits `principal.handle_max_chars`; the first free handle wins (unique index, retry on conflict). The user item maps the Discord user to the principal, so later batches find it by origin key. The authorizer treats a disabled principal as inactive (no login, no trigger, 05 rule 1); the test proves it. Placeholders are personal data of class `account` (22 14); note in the identity README that M7-06 must include them in exports and deletions.

9. **Data schemas of the structure items** (`openapi.yaml`, spec first). `ImportSpaceData{name?, sort_key, existing_space_id?}` (exactly one of `name` and `existing_space_id`; `existing_space_id` must equal the mapping's `existing` for this category); `ImportChannelData{space_origin_key?, existing_space_id?, name?, kind text|forum, topic?, sort_key, existing_channel_id?}` (the space comes from the posted space item or, for a channel without category, from `defaults.uncategorized_space`; `existing_channel_id` must equal the mapping; `read_only` is not a member: the server derives it from the stored `lead`); `ImportForumTagData{channel_origin_key, name, emoji?}`; `ImportUserData{discord_kind user|bot|webhook, username, global_name?}` (the server reads the target from the stored mapping and `defaults`; a webhook author is always a placeholder per webhook ID, 22 5.3). All with `additionalProperties: false`.

10. **Cursor** (22 7.3, 7.4; Ruling for docs/DECISIONS.md). `setImportCursor` replaces `cursor` with the given object and, when `stats` is given, merges its top-level members into `import_sources.stats` (the tool's counters; server-owned counters of note 5 keep their names under `server.*`, the tool's under `tool.*`, so neither overwrites the other). Only for a `running` source (`409 conflict` with `reason` `import_not_running`). The server never interprets the cursor; the run lease (member `run`) belongs to the tool. Body size is bounded by the HTTP layer's request limit. Appends `import.progress`.

11. **Rulings for `docs/DECISIONS.md`** (`D-M8-03-<n>`): note 1 (port, single transaction across modules); note 2 (mapping schema and validation); note 3 (state machine, cancel as `failed`, no-effect repeats); note 4 (lead change semantics); note 5 b (`source_changed`); note 6 (forum tag key); note 7 (ports, collision rule); note 8 (placeholder rules); note 10 (cursor replace, stats namespaces).

## Tests (write them first)

- `internal/importer/domain`: `TestStateMachine` (every pair of state and operation); `TestMappingValidation` (one row per rule of note 2, all errors reported together with pointers); `TestOriginKeyForms`; `TestPlaceholderHandle` (collision counter, alphabet, length at `principal.handle_max_chars`, emoji-only names).
- `internal/importer/adapters/httpapi` integration tests with `pgtest` and the HTTP chain (seed: Robin owner with a session and fresh step-up, Alex a person, the agent Lotse):
    - `TestCreateImport_StepUpAndAudit`: without step-up `401 step_up_required`; a `cli` device session gets `401` (08 6.4, the CLI device from the identity test helper); a personal token of Robin `403`; with step-up `201`, state `draft`, audit entry.
    - `TestStateTransitions` (note 3) including `lead_not_bothy` and `import_state`.
    - `TestApplyBatch_RepeatSameTargets` (plan acceptance): a structure batch twice gives the same targets, the second time no event in the event log and no new row.
    - `TestApplyBatch_StoredThen502` (plan acceptance, 22 16): the test makes the response writer fail after the commit (a middleware that drops the response); the retry returns the same targets and the stats count no duplicate.
    - `TestApplyBatch_ConcurrentSameKey`: two batches with the same keys in parallel give one target each key.
    - `TestApplyBatch_SourceChanged`: a different `content_sha256` returns `source_changed` and changes nothing.
    - `TestApplyBatch_NotRunning`, `TestApplyBatch_TargetOutsideSource`, `TestApplyBatch_NameCollision`, `TestApplyBatch_NotEnabledKinds`.
    - `TestPlaceholderNeverLogsInOrTriggers`: the placeholder principal is `disabled`, has no `user_identities` row and no credential of any kind through the identity ports that exist, the identity port reports it inactive, and `authorize` denies it `message.create` and `agent.invoke` (the login paths themselves refuse inactive principals by the rules of M1-34 and later identity tickets).
    - `TestNewChannelNoGrants`: Alex does not see the imported channel; Robin does; it is read-only with `lead = discord`.
    - `TestReadOnlyRejectsOwner_ImportApplySucceeds` (plan acceptance, 22 16 row "Read-only in the mirror"): Robin's `createMessage` in the read-only channel gives `403 channel_read_only`; a forum tag batch into the same channel succeeds.
    - `TestSetImportLead_ReadOnlyInSameTransaction` (plan acceptance): `lead = bothy` lifts read-only and `lead = discord` sets it, each with `channel.updated` and `import.progress` in the same commit (read both from the event log at one `seq` range); an active run in the channel gets `run.cancel_requested` with reason `channel_read_only` when it becomes read-only.
    - `TestSetChannelReadOnly`: step-up, `If-Match`, audit with reason, runs cancelled.
    - `TestSetCursor_ReplaceAndStatsNamespaces`; `TestGetImportReturnsCursor` (the stored cursor comes back unchanged from `getImport`).
    - `TestImportRoutesOwnerOnly`: Alex gets the `404` or `403` of the catalogue for every route.

Commands that must pass, from the repository root:

- `cd services/server && go test ./internal/importer/... ./internal/chat/... ./internal/identity/... ./internal/runs/... -race -count=1`
- `make migrations-lint`, `make migrations-lock`, `make register-check`, `make arch`, `make api-lint`, `make api-checks`, `make codegen-check`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Repeating a batch returns the same targets without events; "stored, then 502" gives no duplicate.
- [ ] Read-only rejects `message.create` of an owner; `import.apply` succeeds in the same channel.
- [ ] A lead change sets or lifts read-only in the same transaction and cancels active runs when it sets it.
- [ ] Every `import.manage` route needs step-up and writes audit; a `cli` session cannot use them.
- [ ] Placeholders are disabled, without identity and grants, and can neither log in nor trigger.
- [ ] The OpenAPI additions, 22, the READMEs, the CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go symbol; comments explain why.
- `internal/importer/README.md` and the touched module READMEs updated in English.
- Rulings of Design note 11 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` entry under `[Unreleased]`.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
