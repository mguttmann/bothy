# M4-14: Sync engine: catch-up, reconciliation and revocation

Status: ready | Depends on: M4-13 | Milestone: M4

## Goal

After this ticket the sync engine keeps the local database equal to what the server allows the person to see: it applies `ready.snapshot`, catch-up pages and live events through a `Dispatcher` that discards duplicates by the cursor, applies a change only when `resource.version` is newer (types without one in `seq` order), stores the cursor and the sync epoch in the same transaction as each page, follows `catchup.complete` into the live phase, handles `resync_required` without reconnecting (empty the mirror, load the snapshot, cursor to `head_seq`, keep outbox, drafts and share inbox), detects gaps in `thread_seq` and fills them through a `Fetcher` before the messages are shown, applies the personal data rules of 04 14 (own echo ignored, monotonic read state, `overwrote_newer`), keeps unread and mention counters between snapshots, deletes removed targets in one transaction on `access.changed` (04 8.3) and the whole account on `4012` or `session.revoked` (R-SY-14), and keeps typing and agent presence in an `EphemeralStore`. Tests cover duplicates, gaps, `cursor_ahead` and a revocation in the middle of a catch-up. Why: the interface reads only from the local database (R-CL-03); if the database drifts from the server, the person sees content they lost access to or misses messages, and both are the failures the threat model and 04 exist to prevent.

## Read first

- `docs/PLAN.md`: section 3; section 10 (Red team: T-DV-01); rows M4-13, M4-14, M4-17, M4-23.
- `docs/spec/15-client-architecture.md`: 5 "Data flow" (version rule, `reaction.*` and `pin.*` exception), 6.3 (access removal and `4012`), 7.1 (rows `CatchUp`, `Dispatcher`, `Fetcher`, `EphemeralStore`; background isolate above `client.sync.parse_offload_bytes`), 7.3 "Catch-up and reconciliation" (items 1 to 5), 21 (R-CL-03).
- `docs/spec/04-sync-protocol.md`: 2.1, 2.2 (envelope, `truncated`, `redacted`), 4, 6.3 item 4 (`ready.snapshot`), 7 "Catch-up" (7.1 to 7.5), 8.3 to 8.5, 9, 10, 11.3 item 1 (`run.state_changed` feeds the sidebar), 12 items 1 and 2 (typing TTL, presence), 13.4, 14 (table and the echo rule), 16.3 and 16.4 (sequence diagrams).
- `docs/spec/05-actions.md` section 6 (durable and ephemeral types with resource and receive action); `docs/spec/actions.yaml` `events:` (which types carry `resource.version`).
- `docs/spec/14-search.md` 6 (index content, deletion on removal).
- `docs/api/openapi.yaml`: `getSnapshot`, `listEvents`, `listThreadMessages`, `getMessage`, `getThread`, `getChannel`, `listMyReadStates`, `listMyDrafts`, `listMySettings`; schemas `Snapshot`, `GatewayReady`, `GatewayCursor`, `GatewayCatchupComplete`, `GatewayResyncRequired`, `AccessChangedPayload`, `RunUpdatedPayload`.
- `docs/security/threat-model.md` T-DV-01.
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115 (`sync.typing_ttl` and `client.cache_max_age` through `AppConfig`); section 10.
- Briefs: `docs/briefs/M4-13-sync-connection.md` (`SessionListener`, `SessionControl`, phases), `docs/briefs/M4-12-local-store.md` (DAOs, `commitPage`, `upsertIfNewer`, `removeTargets`, `wipe`, `SearchDao`), `docs/briefs/M4-08-domain.md` (`decideApply`, `isDuplicate`, `mergeReadState`, `LoadedRanges`, `UnreadCounter`, `EventCatalog`), `docs/briefs/M4-02-dart-api-client.md` (`EventsPageDecoder`, `decodeEnvelope`, `UnknownEvent`); for the server side `docs/briefs/M1-63-sync-catch-up.md`, `docs/briefs/M1-65-gateway-delivery.md`.

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
9. Limits only through named keys: `client.sync.parse_offload_bytes` and `sync.typing_ttl` from `AppConfig` (M4-08, D-W-115); no literal in code.
10. Specification first: an event type that the catalogue of M4-08 knows but that has no local table (for example `host.updated` before M4-36) advances the cursor and is skipped; an event the catalogue does not know is skipped the same way (04 6.1: additive changes).

Product requirements implemented here:

11. Every event with `seq <= cursor` is discarded (04 9 item 2); a change is applied only when `resource.version` is greater than the local one; types without `resource.version` in `seq` order (04 9 item 3, 15 5).
12. The cursor is stored after every processed page in the same transaction as the page; `cursor` frames advance it without events (04 7.1, 15 7.3 item 1).
13. `resync_required`: discard the server state in the cache (never outbox, drafts, share inbox), load `GET /api/v1/sync/snapshot`, set cursor and epoch to its values, reload open threads; the connection stays open and events up to the new `head_seq` are discarded (04 7.3).
14. A gap in `thread_seq` is loaded through REST before the message is shown (04 4, 15 7.3 item 4).
15. `access.changed` with `removed`: delete threads, messages, files, thumbnails, search index and drafts of the targets and close their views with "No access anymore" (04 8.3); `added`: load the structure of the new targets through REST, no catch-up of older events (04 8.4).
16. `4012` and `session.revoked`: delete all local data of the account and sign out (04 8.5, R-SY-14).
17. Personal data: own echo by `updated_by_device_id` ignored; read state only increases; a draft with `overwrote_newer` raises a notice; a draft being typed on this device is not overwritten (04 14).
18. `truncated: true` reloads the object through REST; a redacted payload stores the tombstone (04 2.2).

## Scope

Create in `packages/sync_engine/lib/src/`:

- `catch_up.dart` (`CatchUpListener implements SessionListener`, Design note 1), `dispatcher.dart` (`Dispatcher`, Design note 2), `appliers/` one file per resource group (`message_appliers.dart`, `thread_appliers.dart`, `structure_appliers.dart`, `run_summary_appliers.dart`, `principal_appliers.dart`, `personal_appliers.dart`, `instance_appliers.dart`, `file_appliers.dart`, `command_appliers.dart`, `access_appliers.dart`), `snapshot_applier.dart`, `fetcher.dart` (`Fetcher`, Design note 4), `gap_filler.dart`, `ephemeral_store.dart` (`EphemeralStore`, Design note 6), `sync_events.dart` (`SyncNotice` stream: `targetsRemoved`, `draftOverwritten`, `resynced`, `signedOut`), `isolate_decode.dart` (Design note 7).
- Tests under `test/catch_up/` (Tests).

Change:

- `packages/sync_engine/lib/sync_engine.dart` exports, `README.md` (applier table: event type, local tables, version rule).
- `docs/spec/15-client-architecture.md` 7.3 (one sentence each: a gateway page ends at a `cursor` frame or `catchup.complete`; events without a local table advance the cursor).
- `docs/DECISIONS.md` section 10: rows `D-M4-14-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The outbox and its echo confirmation (M4-15; this ticket exposes the echo as a callback, Design note 2) and run documents and `run.updated` (M4-16; this ticket forwards ephemeral `run.updated` to a sink, Design note 2).
- The composition of connection, dispatcher, outbox and run store into one engine, and the fake server (M4-17).
- Host and terminal events (`host.updated`, `host.presence_changed`, `terminal_session.changed`): M4-36 adds their tables and appliers.
- Deleting file bytes and key material on disk (the app does it from the returned paths and the `signedOut` notice, M4-18 and M4-22).
- Push notifications and `notification.created` (M4-30 and M5).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-14-sync-catch-up`; one writer per worktree.
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

1. **Catch-up listener.** `onReady`: if `ready.snapshot` is present and the client had no cursor (first connection), apply it through `SnapshotApplier` in one transaction together with `cursor = head_seq` and the epoch; with a cursor, apply only the snapshot parts that are not events (counters, presence, feature flags, active run summaries) and wait for the catch-up. Set the phase `catchingUp`. On `catchup.complete` set `live` and call `onLive` (the outbox start hook of M4-17). A gateway page is the run of `dispatch` frames up to the next `cursor` frame or `catchup.complete`: buffer them, then apply them with `SyncStateDao.commitPage` and the cursor of that frame; a page larger than `client.sync.parse_offload_bytes` decodes in a background isolate (note 7). In the live phase every durable `dispatch` is its own transaction with `cursor = seq`. `resync_required`: phase `resyncing`; in one transaction empty the mirror tables (not `outbox`, `drafts`, `share_inbox`) and `loaded_ranges`; `getSnapshot`; apply it with cursor and epoch from the response; reload the threads that are in focus (`listThreadMessages` newest page); phase `live`; emit `resynced`. Frames that arrive during the resync are kept and applied after it with the duplicate rule.

2. **Dispatcher.** `apply(LocalTxn txn, DecodedEvent e)`: (a) `isDuplicate(e.seq, cursor)` discards; (b) `EventCatalog.lookup(e.type)`: unknown or without a local table, skip; (c) `decideApply` with the stored version and `carriesResourceVersion`; (d) the applier of the type writes through the DAOs (`upsertIfNewer` for versioned rows; absolute values for `reaction.*` and `pin.*`; `thread.moved` updates the channel of the thread; `thread.moved_out` deletes the thread when the target channel is not visible locally; `channel.updated` with a new `read_only` or operator updates the row; `space.archived` and `channel.archived` mark the rows archived; `run.state_changed` writes the run summary and the `trigger_state` of the triggering message comes with its own `message.updated`); (e) `truncated: true` schedules `getMessage` or the matching read through the `Fetcher` and stores what it returns; (f) `message.*` feed `UnreadCounter` of M4-08 and the `SearchDao`; (g) `message.created` with a `client_msg_id` from this person calls `onEcho(clientMsgId, message)` (hook for the outbox of M4-15); (h) ephemeral `run.updated` goes to `onRunUpdated(raw)` (hook for M4-16) and `typing.started` and `agent.presence_changed` to the `EphemeralStore`. The hooks are constructor parameters with no-op defaults; M4-17 wires them.

3. **Access and revocation.** `access.changed`: store `permission_version`; for `removed` call `removeTargets` of M4-12 in the same transaction, then (after the commit) delete the returned file paths through a `FileCacheEraser` port and emit `targetsRemoved` with the IDs so the app closes views; for `added` call `Fetcher.loadStructure(targets)`; for `changed` reload the target through `getChannel` or `getSpace`, whose `ChannelView` and `SpaceView` carry the new `level` and `capabilities` the interface shows controls by. Outbox entries of removed targets stay; they fail with `404` when sent (M4-15). `session.revoked` (personal) and the close code `4012` (from M4-13's sign-out) call `wipe()` and emit `signedOut(deleteLocalData: true)`; nothing is applied after that. An `access.changed` or `4012` in the middle of a catch-up stops the page at that event: what came before it is committed with the cursor of the last applied event, nothing after it is applied.

4. **Fetcher and gaps.** `Fetcher` over the generated API: `snapshot()`, `threadPage(threadId, {ThreadSeq? before, int limit})` (newest first, backward), `range(threadId, from, to)`, `message(id)`, `loadStructure(targets)` (`getSpace`, `getChannel`, `listSpaceChannels`, `listChannelThreads` first page), personal lists. `GapFiller`: when a `message.created` does not fit `LoadedRanges` (M4-08 `fits`), store the message, record the gap and fetch the missing range; `MessagesDao.watchThread` shows only the contiguous part ending at the newest loaded message until the range is filled, so no message appears out of order. A fetch error leaves the gap recorded and retries with the read retry of M4-02 on the next focus.

5. **Personal data.** `read_state.updated`: `mergeReadState`; counters from the payload. `draft.updated`: ignore when `updated_by_device_id` is this device; if the person is typing in this draft locally (a `DraftEditing` port the app sets), keep the local text and let the next local write win; otherwise store. `overwrote_newer` in a `putThreadDraft` response (the app's write path) emits `draftOverwritten`. `settings.updated` per key with `deleted: true` removing the key. `sidebar.updated`, `notification_pref.updated`, `quiet_hours.updated` per row.

6. **Ephemeral store.** In memory only: typing per thread (principal, `expires_at` capped at `now + sync.typing_ttl`; removed at expiry or at the next `message.created` of that principal in the thread), agent presence per agent (`state`, `reported_billing_mode`, seeded from `ready.snapshot`); streams per thread and per agent. Lost on reconnect by design (04 2.1).

7. **Decoding off the main isolate.** Pages whose raw size exceeds `client.sync.parse_offload_bytes` are decoded with `Isolate.run` into plain records, then applied on the caller's isolate (the database already runs in its own isolate, 15 7.1).

8. **Rulings for docs/DECISIONS.md** (`D-M4-14-<n>`): page boundary at `cursor` frames (note 1); snapshot use with and without a cursor (note 1); hooks instead of dependencies on the outbox and run store (note 2); stopping a page at a revocation (note 3); showing only the contiguous range (note 4); the draft typing rule (note 5).

## Tests (write them first)

`packages/sync_engine/test/catch_up/` (fake transport and clock from M4-13, a fake REST adapter for `api_client` with recorded responses, in-memory `local_store`):

- `duplicates_test.dart`: the same page delivered twice and live events below the cursor change nothing; a live event repeated after reconnect is applied once.
- `version_rule_test.dart`: an older `message.updated` after a REST page with a newer version is ignored; reactions and pins without version are applied in `seq` order; a type without a local table advances the cursor.
- `cursor_transaction_test.dart`: an exception inside a page leaves the cursor at the previous value; a `cursor` frame without events advances it.
- `gaps_test.dart`: `thread_seq` 10 then 13 fetches 11 to 12 before 13 is visible in `watchThread`; a fetch error keeps the gap recorded.
- `resync_test.dart`: `resync_required` with each reason (`retention`, `too_far_behind`, `epoch_changed`, `cursor_ahead`) empties the mirror, keeps outbox, drafts and share inbox rows, loads the snapshot, sets cursor and epoch, discards events up to the new `head_seq` and stays connected (no new `identify`).
- `revocation_test.dart`: `access.changed` with `removed` in the middle of a catch-up page deletes the channel's rows, search entries and drafts in one transaction, applies nothing of the page after it, emits `targetsRemoved`, and outbox entries remain; `4012` during a catch-up and `session.revoked` call `wipe` and emit `signedOut`.
- `personal_test.dart`: own echo ignored; read state never decreases; a draft being typed locally is not overwritten; `settings.updated` with `deleted`.
- `ephemeral_test.dart`: typing expires after the TTL and at the next message; presence from snapshot and events.
- `truncated_redacted_test.dart`: `truncated: true` fetches the message; a redacted `message.updated` stores the tombstone and removes the search entry.

Commands that must pass, from the repository root:

- `cd packages/sync_engine && dart test --coverage=coverage` (or the command M4-01 uses for pure Dart members)
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Duplicates, gaps, `cursor_ahead` and a revocation in the middle of a catch-up are tested and behave as 04 says.
- [ ] Cursor and epoch are stored in the transaction of each page; the version rule and the `seq` rule are applied per the catalogue.
- [ ] `resync_required` keeps outbox, drafts and share inbox and does not reconnect.
- [ ] Removed targets disappear in one transaction including search index and drafts; `4012` and `session.revoked` wipe everything.
- [ ] Personal data follow 04 14; typing and presence live only in memory.
- [ ] `sync_engine` imports no Flutter (`make arch`); rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 04.
- `packages/sync_engine/README.md` (English) with the applier table.
- 15 7.3 updated; rulings of Design note 8 in `docs/DECISIONS.md` section 10.
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
4. The applier table as built (event type, local tables, version or `seq` rule), the event types skipped for lack of a local table, and every difference between the snapshot schema on the base branch and 04 6.3 item 4.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
