# M8-04: Import tool: dry run, apply, report

Status: ready | Depends on: M8-02, M8-10, M8-11 | Milestone: M8

## Goal

After this ticket `bothy import discord` takes over the history of a Discord server into Bothy. Without `--apply` it is a dry run that only reads (Discord through the client of M8-01, Bothy through the client of M8-10), writes nothing anywhere, not even a local file, and prints the check report of 22 11. With `--apply` it imports into the source of this guild that the owner created and started in the web console: preflight check (22 4.4), mapping of categories, channels, forums, threads, people and tags (22 5), conversion of every message to the Markdown dialect of the server (22 6.3), attachments through the CDN fetch of M8-02 and `createImportFile` with a jump-link line when they cannot be taken over (22 6.5), batches per thread in Discord order through `applyImportBatch`, the cursor advanced only after a successful batch, a run lease that keeps two writing runs apart unless `--takeover`, and the report stored in the source's counters. `--channel` and `--only-thread` narrow the run and an unknown or non-matching ID ends it before any write; `init-map` writes a skeleton of the mapping file; `status` prints source, state, ownership list and counters. The tool's own values come from `--config` (nested YAML with `import.*`) and the environment `BOTHY_IMPORT__...`, which wins. Exit codes follow 22 10.3. An abort between a batch and its cursor never produces a duplicate: a new run takes every item over exactly once, in Discord order per thread. Why: this is the import itself; its guarantees (GET only in the dry run, no duplicates, no secrets in output, no trigger) are the exit criteria of M8 (PLAN 14, 22 16, ADR 0043).

## Read first

- `docs/spec/22-import-migration.md`: all of sections 1 to 11, 14, 16 and 19 (the mirror of 12 and the switch-over of 13 are M8-05 and M8-07, the session binding of 9 is M8-06, but read them to leave room).
- `docs/adr/0043-discord-import-read-only-dry-run-origin-key.md`; `docs/REUSE.md` 4.1, 4.3, 4.4, 4.7, L17, L18, L19, L20; `docs/RISKS.md` RK-73, RK-76.
- `docs/operations/configuration.md`: section 2 (naming rule, formats), section 4 "22 Import and migration from Discord"; `docs/DECISIONS.md` D-W-51 (tool values never in the server file).
- `docs/spec/02-architecture.md` 3.3 (exit codes and output of commands); `docs/spec/15-client-architecture.md` 11 (in-app paths used for channel links); `docs/spec/06-api.md` 7.
- `docs/api/openapi.yaml`: the tag `Import`, `getMe`, `getInstance` (`InstanceLimits`), `listUsers`, `listAgents`, `listSpaces`, `listSpaceChannels`, `uploadImportFilePart`, `completeImportFile`, and the data schemas `Import*Data` added by M8-03 and M8-11.
- `docs/DECISIONS.md`: D-IM-01 to D-IM-08, D-W-100, D-W-120, D-W-124; the rows `D-M8-01-<n>`, `D-M8-02-<n>`, `D-M8-03-<n>`, `D-M8-10-<n>`, `D-M8-11-<n>`; section 10 format.
- Briefs (merged names win, D-W-100): `M8-01-discord-read-client.md`, `M8-02-cdn-fetch.md`, `M8-10-import-login-client.md` (session, transport, `bothytest`, `bothy import` command group), `M8-03-import-server.md` (states, mapping schema, structure items, `source_changed`, cursor and stats namespaces), `M8-11-import-content-items.md` (thread batch shape, message and file items), `M1-07-config-schema-loader.md` (`platform/config` `NewSchema`, `Load`, `Sources`), `M1-71-contract-tests.md` (`harness.Start`).
- Code on the base branch: `services/server/cmd/bothy`, `services/server/cmd/bothy/importtool/...`, `services/server/internal/platform/config`, `services/server/api/restclient`, `services/server/test/`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Discord fixtures use invented IDs and names.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. Dry run is the default: only `GET` against Discord and Bothy (plus `refreshToken` when the access token expired), no attachment download, no file written except the rotated session file of M8-10, no source created (22 10.1, 10.2 rules 1 and 5, R-IM-02, ADR 0043 ruling 2).
12. `--apply` imports only into the `running` source of this guild that the owner created in the web console; the stored mapping is authoritative and the local `--map` must equal it apart from `lead` values; no `import.manage` call from the tool (22 8, D-W-124).
13. Preflight of 22 4.4 before every run, including the dry run: bot token valid, server and mapped channels readable, intent probe, active owner session, every target person and agent exists.
14. Mapping and conversion exactly as 22 5 and 6 (types, fields, content table, attachments, pins and tags, reactions only counted).
15. Idempotency and resuming as 22 7: origin keys, cursor only after a successful batch, run lease with `--takeover`, every item at most once and in Discord order per thread.
16. Options and exit codes as 22 10.1 to 10.3: an unknown or non-matching ID in `--channel` or `--only-thread` ends the run before any write, also for the valid IDs of the same call; contradictory options end with exit `2` without any call.
17. Output (standard output, standard error, report, log) never contains message content, file names of attachments, names of people, tokens or signed URLs (22 10.2 rule 4, 11).
18. Configuration: `--config` reads one YAML file with nested `import.*` keys; flat dot keys and keys outside `import.*` are errors; the environment `BOTHY_IMPORT__...` takes precedence; the server file is never read (22 19, D-W-51).
19. Telemetry of the tool is counters and durations in the report and in `import_sources.stats` (namespace `tool.*`), no content, no Discord IDs (22 14).

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create (package `cmd/bothy/importtool` unless named otherwise):

- `config.go`: the schema of the tool keys of 22 19 over `platform/config`, loading from `--config` and the filtered environment (Design note 2).
- `mapping.go`: strict YAML parse of `import_source.discord.v1`, validation without calls, comparison with the stored mapping (note 3).
- `preflight.go` (note 4), `plan.go` (structure and thread plan, note 5), `convert.go` (content conversion, note 6), `attachments.go` (note 7), `apply.go` (batches, cursor, lease, note 8), `report.go` (note 9), `initmap.go` (note 10), `status.go`, `run.go` (the command flow and exit codes, note 1).
- `cmd/bothy/importtool/bothyapi/bothytest`: extend the fake of M8-10 with an in-memory import interface (note 11).
- `cmd/bothy/importtool/discord/testdata/`: extra fixtures where the conversion table needs them.
- `services/server/test/importtool/`: real-server tests with the harness of M1-71 and the fake Discord (note 12).
- The subcommand `bothy import discord` (flags of note 1) in the `bothy import` group of M8-10; `importtool/README.md` (purpose, commands, options, exit codes, configuration, the dry-run guarantee, an example mapping with white-label data).

Change:

- `docs/spec/22-import-migration.md`: 6.3 (the channel link form of note 6), 10.1 (the `--session-file` option and that `status` prints no names), 11 (the JSON form of the report, note 9); no other section unless a gap is found (then report it).
- `services/server/README.md`, `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M8-04-1` to `D-M8-04-10`.

This ticket adds no API operation, no table, no action, no event and no server configuration key.

## Out of scope

- `--follow` and `--once` (read mirror, M8-05), `--sessions` (M8-06), `switch` and `revert` (M8-07): the flags do not exist yet; their tickets add them and their contradiction rules.
- Creating, starting, pausing, completing a source and lead changes (web console, M8-08).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-04-import-tool`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(import): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Never contact Discord, its CDN or any real service: Discord and the CDN are the fakes of M8-01 and M8-02; Bothy is the fake of M8-10 or the harness server of M1-71 on `127.0.0.1` with port 0, PostgreSQL through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the real-server tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `t.TempDir()`, removed at the end.
- Never read or print secrets; tokens in tests are invented strings.
- Time only through `kernel/clock`; tests use a fake clock.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Command and flow** (22 10). `bothy import discord --guild <id> --map <file> [--apply] [--channel <id>]... [--only-thread <id>]... [--report text|json] [--takeover] [--discord-token-file <path>] [--config <file>] [--session-file <path>]`, plus `init-map --guild <id> --out <file>` and `status --guild <id>`. Order of a run: (a) parse flags and check contradictions (`--takeover` without `--apply`, `--report` with another value, a non-digit ID) with exit `2` and no call; (b) load configuration (note 2) and the token (environment `BOTHY_IMPORT_DISCORD_TOKEN` or `--discord-token-file`; both or neither is exit `2`; a token file readable by group or others is refused like the session file of M8-10); (c) parse and validate the mapping without calls (exit `2`); (d) preflight (note 4; exit `2` on a finding); (e) with `--apply`: find the source, compare mappings, take the lease (note 8; exit `3` when held); (f) plan, then read and write (or only read); (g) report (note 9). Exit `1` for network, server error, rate limit above the limit or Bothy `409 conflict` with `reason` `import_not_running` during the run (the owner paused it); the state stays and a new run resumes. Report and log lines go to standard error; the report goes to standard output (text or JSON).

2. **Configuration** (22 19, D-W-51; Ruling for docs/DECISIONS.md). Reuse `platform/config` with a schema of exactly the keys of 22 19 (and `import.login_poll_interval` of M8-10). Environment: pass only variables whose name starts with `BOTHY_IMPORT__` to `Load` (a server variable on the same machine must not abort the tool, and `BOTHY_IMPORT_DISCORD_TOKEN` is the token, not a key); file: `--config` (never `BOTHY_CONFIG_FILE`); the loader already rejects dotted keys, unknown keys and duplicates. `import.attachment_max_bytes` without a value means `file.max_upload_bytes` of the server (`getInstance` limits); a configured value above the server limit is lowered to it and the report says so. Test the precedence (environment over file over default) and that a key outside `import.*` in the file is an error.

3. **Mapping file** (22 5.2). Strict YAML (`yaml.v3` with `KnownFields(true)` through the vendor group the config package uses): unknown field, wrong type, duplicate Discord ID in a list, non-digit ID, a target in none of the allowed forms, `guild_id` different from `--guild` are exit `2` with the YAML line, before any call. Unknown handles are found in the preflight (they need a call) and are also exit `2`. With `--apply`, the source of this guild (`listImports`, `kind = discord`, `config.guild_id` equal; more than one source in state `draft`, `running` or `paused` for one guild is exit `2`) must be `running` (else exit `2` with "start the source in the web console"), and its stored mapping must equal the parsed `--map` after normalization except `lead` (else exit `2` naming the first differing JSON pointer); the leads come from the stored mapping, never from the file (22 12.1).

4. **Preflight** (22 4.4, RK-73). (1) `Me`: `bot = true`, else exit `2`. (2) `Guild`, `GuildChannels`: every mapped category and channel exists; a mapped channel the bot cannot read (`403` on its first message page) is exit `2`. (3) Intent probe: in the first readable mapped channel in Discord order with messages by other authors, among the first `import.discord.preflight_sample` messages by authors other than the bot at least one has content, an attachment or an embed; otherwise exit `2` with the note that `MESSAGE_CONTENT` is probably missing (22 16 row "Intent probe"). (4) `getMe`: the session's person is an active owner, else exit `2`. (5) Every `user:<handle>` and `agent:<handle>` target and every `planned_operator` exists and is active (`listUsers`, `listAgents`), else exit `2`. (6) Name collisions: a `create` name equal to the name of an existing space or channel (`listSpaces`, `listSpaceChannels`) that is not the target of a posted item of this source (`listImportItems`, when the source exists) is exit `2` before the first write (22 5.1 rule 1).

5. **Plan** (22 5.1, 5.1.1, 5.3, 6.1). Structure in Discord `position` order (spaces, then channels per space, then forum tags per forum), skipped targets and voice or stage channels counted; channels without category per `defaults.uncategorized_space`. Threads per channel: active list plus archived public and, where allowed, private lists (a `403` there is a note, not an abort); the done rule of 22 5.1.1 with `import.discord.closed_tags` (only `archived` exactly `true` and not in the active list, or a closed tag); title from the name; starter message key `message:<thread_id>` for threads of text channels. Order of writing per channel: main thread messages first, then threads by ascending ID, each thread's messages ascending from its cursor. People: before a thread batch the tool posts the user items of all authors of that batch that are not yet posted (a structure batch without `thread_origin_key`), honoring `defaults.unmapped_human`, `defaults.unmapped_bot` and webhook authors (22 5.3); messages of a skipped author are counted and skipped. `--channel` narrows to these channels (and their structure); `--only-thread` selects exactly these threads (and their channel and structure); every given ID must be a mapped channel or a thread of a mapped channel found in the reads, else exit `2` before the first write (22 10.2 rule 2, 16 row "Invalid selection").

6. **Conversion** (22 6.1 to 6.3, 5.3; Ruling for docs/DECISIONS.md). Pure functions with table tests: types 0, 19, 20, 21, 23 taken over; others counted as skipped by type; type 21 dropped when its starter is in the main thread; empty messages without attachment, embed, sticker and poll skipped. `<@id>` and `<@!id>` become `@DisplayName` from the message's `mentions` (fallback `@unknown-user`), never a mention row; `<@&id>` `@RoleName` from the guild roles (`GET /guilds/{id}` carries `roles`; fallback `@unknown-role`); `<#id>` a Markdown link `[#name](/c/<channel_id>)` with the in-app path of 15 11 when the channel is posted in this source, else `#name`; `<:NAME:ID>` and `<a:NAME:ID>` `:NAME:`; `<t:SECONDS:STYLE>` the ISO 8601 UTC text; embeds a quote block with title, link and description (no image is loaded); stickers `[Sticker: name]`; polls question and answers as a list with the line "Poll"; forwarded `message_snapshots` a quote block; a leading small-text marker (`-#` and one space) per line removed; replies (type 19) get `reply_to_origin_key` when the referenced message is posted in the same thread or earlier in the same batch, else a first line `> @Name: <text>` cut to `import.reply_quote_max_chars`. `content_sha256` is the hex SHA-256 of the converted Markdown plus the sorted attachment IDs. Every message stays one message (22 6.3, last paragraph); a converted text above `message.max_content_bytes` of the server is a reported failure of that message, never a split.

7. **Attachments** (22 6.5). Per attachment: size above the effective `import.attachment_max_bytes` is "too large"; else fetch through M8-02 into memory (bounded by the limit plus one byte, never a local file) and compute the SHA-256; then `createImportFile` (origin key `file:<attachment_id>`, the target channel, the mapped author, the file name, size, content type, `sha256`); when the slot has an `upload`, send the parts with `uploadImportFilePart` (part size and digests as the session prescribes) and finish with `completeImportFile` (M8-11 note 6 b2); a `posted` slot is reused without fetching again when the item is already `posted` (check `listImportItems` or the slot before fetching on a resumed run). On failure or "too large" the message gets one line per attachment: file name, size, reason and the jump link `https://discord.com/channels/{guild_id}/{channel_id}/{message_id}` (22 6.5 item 6); the report counts it. The file name is message content and never appears in output.

8. **Apply, cursor and lease** (22 7.3, 7.4; Ruling for docs/DECISIONS.md). Cursor object `{version: 1, run: {id, renewed_at}, channels: {<id>: <last message id>}, threads: {<id>: <last message id>}}`. Start: read the source and its `cursor` (`getImport`, member `cursor` added by M8-03); when `run.renewed_at` is younger than three times `import.follow.poll_interval` and `run.id` is not ours, exit `3` unless `--takeover`; write the lease with `setImportCursor`. Renewal does not depend on batches: a ticker renews the lease every `import.follow.poll_interval` while the run lives, serialized with the batch's own cursor writes (one mutex), so long rate-limit waits, large attachments or a backoff never let it age past the window. Each batch: at most `import.batch_max_items` items (and never more than the `maxItems` of `ImportBatch`; a configured value above it is lowered and reported), one thread; after a `2xx`, update the cursor of that channel or thread to the last message ID of the batch and renew the lease in the same `setImportCursor` call, together with the tool counters (`tool.*`). A `5xx`, a network error or a lost response: the run ends with exit `1` without advancing (the next run repeats the batch; the server answers posted keys with their targets, M8-03 note 5). At the end the lease is cleared (`run` removed). A test hook `afterBatch func(batchNo int) error` (unexported, set only by tests) lets the crash test end the process between batch and cursor.

9. **Report** (22 11; Ruling for docs/DECISIONS.md). One row per channel (Discord channel name and Bothy target name, which are structure names, not people) and a total: threads open and done; messages taken over, already present, skipped by reason; attachments taken over, too large, failed; people mapped and placeholders (counts only); sessions (left empty until M8-06); pins and tags; reactions not taken over (sum of counts); notes (`403`, missing `MANAGE_THREADS`, `permission_overwrites` present, changed at the source from `source_changed`, attachment limit lowered). `--report json` prints the same as one JSON object with a `version` member. A dry run says "would take over". The totals are also written into `stats.tool` with the last `setImportCursor` of an `--apply` run (no write in a dry run).

10. **`init-map`** (22 5.2). Reads structure and all messages (like a dry run, with the same preflight steps 1 to 3) and writes exactly the file `--out` (refusing to overwrite an existing file, exit `2`): every category and channel with `target: {create: {name: <Discord name>}}`, `planned_operator: none`, `lead: discord`, voice and stage channels as comments, every author seen with `target: placeholder` and a YAML comment of the form `# bot` or `# webhook` (no names in comments), `defaults` from 22 5.2. It writes no other file.

11. **Fake Bothy import interface** (extend `bothytest`). In memory: sources, items with origin keys (`posted` returns the same target, `source_changed` on a different hash), structure and thread batches with the shape rules of M8-11, files with slots and upload sessions, cursor and stats, `getMe`, `getInstance`, `listUsers`, `listAgents`; method counting per route; faults "stored, then 502", `409 conflict` with `reason` `import_not_running`, `429`. It is only for unit tests; the guarantees that need the real server run in note 12.

12. **Real-server tests** (`test/importtool`, component `testsupport`). `harness.Start` with Robin as owner; the test creates and starts the source with Robin's step-up session (the web console's job in production), logs the tool in through pairing (M8-10), and runs the tool in-process against the fake Discord fixture `guild_basic.json`.

## Tests (write them first)

- `importtool/config_test.go`: precedence; flat dot key rejected; key outside `import.*` rejected; a server variable such as `BOTHY_DATABASE__URL` in the environment is ignored; `BOTHY_IMPORT_DISCORD_TOKEN` is not a key.
- `importtool/mapping_test.go`: one row per error of note 3, each exit `2` before any call (both fakes log no request).
- `importtool/convert_test.go`: one row per line of the 22 6.3 table, 6.1 type rules, reply quote cut, `content_sha256` stability.
- `importtool/dryrun_test.go` (plan acceptance, 22 16 row "Dry run only GET"; REUSE 4.7): with a fresh access token both fakes log only `GET`; with an expired one Bothy additionally logs exactly one `refreshToken`; the working directory (`t.Chdir` into an empty `t.TempDir()`) and `TMPDIR` (another empty `t.TempDir()`) are empty afterwards; the session file changed only in the refresh case; the CDN fake logs no request.
- `importtool/selection_test.go` (plan acceptance, 22 16 row "Invalid selection"): an unknown `--only-thread` or `--channel` among valid ones ends with exit `2` and the Bothy fake logs no write.
- `importtool/preflight_test.go`: intent probe exit `2` (22 16 row "Intent probe"); non-bot token; not an owner; unknown handle; name collision.
- `importtool/secret_test.go` (22 16 row "No secret in output"): with echoing fakes and a fixture with distinctive message texts, file names and display names, none of token, signed query, message text, file name or display name appears in standard output, standard error, report (text and JSON) or debug log.
- `test/importtool/apply_test.go` against the real server: `TestApplyImportsFixture` (counts equal the fixture: spaces, channels, forum tags, threads open and done per the archive rule, messages with `origin = import` and `silent`, pins, placeholders; attachments over the limit get the jump-link line); `TestCrashBetweenBatchAndCursor` (plan acceptance, 22 16 row "No duplicate after a crash": the hook ends run 1 after batch N before the cursor; run 2 completes; every origin key exactly once in `import_items`, each thread's messages in `thread_seq` order equal to Discord ID order, no duplicate message rows); `TestRunLease` (a second run while the lease is fresh exits `3`; with `--takeover` it runs; with a fake clock, a run that waits longer than three intervals in a rate-limit wait still holds a fresh lease); the crash test lets run 2 use `--takeover` or advances the fake clock beyond the window, because run 1 left a lease; `TestPausedSourceExit1`; `TestMappingDiffersFromStoredExit2`; `TestImportedMessagesNeverTrigger` (a channel imported with `lead = bothy` and an operator gets no run).

Commands that must pass, from the repository root:

- `cd services/server && go test ./cmd/bothy/... ./test/importtool/... -race -count=1`
- `make arch`, `make lint`, `make fmt-check`, `make semgrep`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Dry run: the fakes see only `GET` (plus one refresh when the token expired); no file in the working or temp folder.
- [ ] An unknown `--only-thread` aborts before any write.
- [ ] Crash between batch and cursor: every item exactly once, order per thread equal to the source.
- [ ] `--config` reads nested YAML with `import.*`, rejects flat dot keys and keys outside `import.*`; the environment `BOTHY_IMPORT__...` takes precedence.
- [ ] Preflight, conversion, attachments, report and exit codes follow 22.
- [ ] No content, file name, person name, token or signed URL in any output.
- [ ] README, 22, CHANGELOG and rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go symbol; comments explain why.
- `cmd/bothy/importtool/README.md` in English.
- Rulings of Design notes 2, 3, 6, 8 and 9 recorded in `docs/DECISIONS.md` section 10.
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
4. What is not proven (the tool never ran against a real Discord server; the conversion follows the cited documentation).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
