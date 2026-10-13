# M4-15: Sync engine: outbox

Status: ready | Depends on: M4-02, M4-12 | Milestone: M4

## Goal

After this ticket every write of the apps goes through one outbox (15 1 principle 3): an entry with its own ID (`client_msg_id` for messages, `client_interaction_id` for interactions, `Idempotency-Key` for every other write) is stored in `local_store` before anything is sent; a sender per thread sends strictly FIFO and threads in parallel; network errors, timeouts, `5xx`, `429` and `409 request_in_progress` are retried with full-jitter backoff and the same ID, while `400`, `404`, `410`, `413` and `422` are final and leave the entry visible as failed with its reason; `412 version_conflict` keeps both revisions for the person to decide; the echo `message.created` with a matching `client_msg_id` confirms an entry; file uploads are their own entries before the message that attaches them, and the message waits until its files are complete; entries survive a restart; an entry waiting longer than `client.outbox_stuck_after` shows "Sending delayed"; and the outbox sends only while the sync engine is live after the catch-up. The executors per operation are registered, so the later app tickets add their writes without changing the outbox. Why: 04 13 and 16.2 promise exactly one message per send even when a response is lost, and an outbox that drops or duplicates a write breaks that promise for every feature built on it.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-15, M4-17, M4-20, M4-22.
- `docs/spec/15-client-architecture.md`: 1 (principle 3), 5 "Data flow" (controller, outbox, sender per thread FIFO), 7.1 (row `Outbox`), 7.5 "Outbox" (diagram and the text on `client.outbox_stuck_after` and uploads before messages), 14 (upload as an outbox entry, attach only when `upload_state = complete`), 22 (`client.outbox_stuck_after`).
- `docs/spec/04-sync-protocol.md`: 5 "Idempotent writes" (5.1 `client_msg_id` and `Idempotent-Replay`, `422 idempotency_key_reuse`; 5.2 `Idempotency-Key`, `409 request_in_progress`), 8.3 item 4 (removed targets fail with `404` and keep the text), 13.2 "Outbox", 13.3 "Editing and deleting" (the whole table), 13.4, 16.1 and 16.2 (sequence diagrams), 18 (`client.outbox_retry_base`, `client.outbox_retry_cap`).
- `docs/spec/06-api.md`: 5 (problem codes `version_conflict`, `message_deleted`, `request_in_progress`, `idempotency_key_reuse`, `rate_limited` with `Retry-After`), 7 (idempotency).
- `docs/spec/12-files.md`: the upload session states (`upload_state`) only for the dependency rule.
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115 (`client.outbox_retry_base` and `client.outbox_retry_cap` through `AppConfig`), D-W-116 (this ticket depends on M4-02 because its executors call the generated API and classify `ApiProblem`); section 10.
- Briefs: `docs/briefs/M4-12-local-store.md` (`outbox` table, `OutboxDao`), `docs/briefs/M4-08-domain.md` (`Uuid7Generator`, `AppConfig`), `docs/briefs/M4-02-dart-api-client.md` (the `idempotency_key` request option, `ApiProblem`; the outbox calls the API only through executors, Design note 3).

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
9. Limits only through named keys: `client.outbox_retry_base`, `client.outbox_retry_cap` (D-W-115) and `client.outbox_stuck_after` from `AppConfig`; no literal in code.
10. Specification first: 15 7.5 does not name `409 request_in_progress` and `412 version_conflict`; this ticket follows Design note 2 and adds both to 15 7.5 in the same change.

Product requirements implemented here:

11. Every write first lands in the outbox with its own UUIDv7 ID and, where needed, the expected version; there is no second write path (04 13.2 item 1, 15 1 principle 3).
12. Strictly FIFO per thread: the next entry of a thread is sent only after the response to the previous one; different threads in parallel (04 13.2 item 2).
13. Retry on a network error, a timeout, `5xx` and `429` with full jitter between `client.outbox_retry_base` and `client.outbox_retry_cap`, honouring `Retry-After`, with the same ID on every retry (04 13.2 item 4).
14. `400`, `404`, `410`, `413`, `422` end the retries; the entry stays visible as failed with the reason; the person can edit it, send it again with a new ID or discard it; nothing is discarded silently (04 13.2 item 5).
15. The echo with a matching `client_msg_id` confirms the entry and replaces the waiting display with the server message (04 13.2 item 6).
16. Editing and deleting follow 04 13.3: an unsent edit changes the entry; an unsent delete removes it, a sent but unconfirmed one gets a delete entry after it; `412 version_conflict` shows both revisions; `410 message_deleted` discards the edit and keeps the text for copying.
17. Uploads are entries before their message; the message is sent only when every file it attaches is complete (15 7.5, 15 14).
18. After a restart every non-final entry is sent again with its stored ID; after a reconnect the outbox sends only after `catchup.complete` (04 13.4).

## Scope

Create in `packages/sync_engine/lib/src/outbox/`:

- `outbox.dart` (`Outbox`: `enqueue`, `edit`, `remove`, `retry`, `discard`, `confirmByEcho`, `start`, `pause`, `watchThread`, `depth`), `entry.dart` (`OutboxEntry` view with `stateLabel` including `delayed`), `sender.dart` (per-thread FIFO scheduler), `classify.dart` (`OutcomeClass` from an `ApiProblem` or a transport error, Design note 2), `executors.dart` (`OutboxExecutor` registry, Design note 3), `backoff.dart` (or reuse the full-jitter function of M4-13 if it is merged and exported; the two must not differ), `operations/message_ops.dart` (executors for `createMessage`, `updateMessage`, `deleteMessage`) and `operations/personal_ops.dart` (`putReadState`, `putThreadDraft`, `deleteThreadDraft`, `putMySetting`, `deleteMySetting`, `addReaction`, `removeReaction`).
- Tests under `test/outbox/` (Tests).

Change:

- `packages/sync_engine/lib/sync_engine.dart` exports, `README.md` (entry life cycle, error classes, how a feature registers an executor).
- `docs/spec/15-client-architecture.md` 7.5 (`409 request_in_progress` retried, `412 version_conflict` kept for the person).
- `docs/DECISIONS.md` section 10: rows `D-M4-15-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The upload protocol itself (parts, resumption): M4-22 registers an `upload` executor; this ticket provides the dependency rule and tests it with a fake executor.
- Interactions and commands (`createInteraction`, `invokeCommand`): M4-21 registers their executors.
- Wiring the echo from the dispatcher and the start after `catchup.complete` (M4-17 composes them; this ticket offers `confirmByEcho` and `start` and `pause`).
- The interface of failed entries (M4-20).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-15-sync-outbox`; one writer per worktree.
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

1. **Entry and IDs.** `enqueue(OutboxRequest r)` creates the ID with `Uuid7Generator` (the caller can pass one, as the share inbox does with the ID the extension created), writes the row with `state = pending` and returns the entry. For messages the ID is the `client_msg_id` in the body; for interactions `client_interaction_id`; for everything else the executor passes it as `Idempotency-Key` through the request option of M4-02. The ID never changes on retry; "send again" after a final failure creates a new entry with a new ID and discards the old one (04 13.2 item 5).

2. **Outcome classes.** `classify(Object error)`: transport errors, timeouts, `5xx`, `429` and `409 request_in_progress` are `retry` (`429` and `503` wait `Retry-After` when present, otherwise backoff); `400`, `404`, `410`, `413`, `422` are `final` with the problem `code` and `reason` as the failure reason; `412 version_conflict` is `conflict` (state `failed` with the `current` revision of the problem kept in the entry, so the person decides; no automatic merge, 04 13.3); `401` is handed to `auth` by its interceptors before it reaches the outbox (a remaining `401` after refresh is `final`); `422 idempotency_key_reuse` is `final` and names a programming error in the report. `2xx`, `200` with `Idempotent-Replay: true` and `204` on a delete are `confirmed`. Ruling for docs/DECISIONS.md.

3. **Executors.** `typedef OutboxExecutor = Future<OutboxResponse> Function(OutboxEntry entry, ApiCallContext ctx)`; `OutboxExecutors.register(String kind, OutboxExecutor executor)`. The registry is the only coupling to `api_client`; tests register fakes. A kind without an executor fails the entry with `no_executor` (a programming error the tests catch). Each executor declares whether its entry belongs to a thread (FIFO key) or to a global lane (`putMySetting` and similar run in a lane keyed by the resource).

4. **Sender.** One worker per FIFO key: take the oldest non-final entry; if it `dependsOn` an entry that is not `confirmed`, wait (an upload entry is confirmed by its executor only when the upload may be attached; M4-22 defines that point from 12 3.4); set `sending`; call the executor; apply the outcome (`confirmed` deletes the row after storing what the response returned through the executor; `retry` sets `waiting` with `next_attempt_at`; `final` and `conflict` set `failed`). Entries of other keys run in parallel up to a bound of eight concurrent workers (a constant with the reason in a comment: browsers and servers limit connections per host; record as a ruling). `pause()` lets running requests finish and starts none; `start()` resumes. On start after a restart, `sending` entries become `pending` (the request may have reached the server; the same ID makes the repeat safe).

5. **Echo and display.** `confirmByEcho(String clientMsgId)` marks the matching entry `confirmed` even while its request is still waiting for a response (16.2: the echo can come first). `watchThread(threadId)` streams the non-final and failed entries of a thread in FIFO order for the display after the last confirmed `thread_seq` (04 13.2 item 3); an entry in `waiting` longer than `client.outbox_stuck_after` reports `delayed` (from the injected clock). `depth` streams the count of non-final entries (for `client.outbox.depth`, M4-31).

6. **Editing and deleting (04 13.3).** `edit(id, payload)`: on `pending`, `waiting` or `failed` the payload changes in place (a failed entry goes back to `pending` with a new ID because the content changed); on `sending` a follow-up `updateMessage` entry with `If-Match` is queued once the create is confirmed. `remove(id)`: on `pending` or `waiting` the row is deleted; on `sending` a `deleteMessage` entry is queued after it. An edit of a sent message is its own entry with the expected version; `410 message_deleted` discards it and keeps the text in the entry's failure record for copying.

7. **Rulings for docs/DECISIONS.md** (`D-M4-15-<n>`): the outcome classes (note 2); the executor registry (note 3); the worker bound and lanes (note 4); confirmation by echo before the response (note 5); new ID after an edit of a failed entry (note 6).

## Tests (write them first)

`packages/sync_engine/test/outbox/` (in-memory `local_store`, fake executors with scripted outcomes, fake clock, seeded random source):

- `fifo_test.dart`: three entries in thread A and two in thread B: A's are sent strictly in order, each after the previous response; B runs in parallel with A.
- `retry_test.dart`: `5xx`, `429` (with and without `Retry-After`), a timeout and `409 request_in_progress` are retried with the same ID within the jitter bounds; `400`, `404`, `410`, `413`, `422` end in `failed` with the reason and no further call.
- `conflict_test.dart`: `412 version_conflict` keeps the current revision; `410 message_deleted` keeps the text.
- `restart_test.dart`: entries in `pending`, `waiting` and `sending` survive closing and reopening the database and are sent again with their stored IDs; confirmed entries are not.
- `echo_test.dart`: an echo before the response confirms the entry and the late response does not create a second display entry; an echo with an unknown ID changes nothing.
- `dependency_test.dart`: a message that depends on two uploads waits until both are confirmed; a failed upload leaves the message waiting and visible with the reason.
- `edit_delete_test.dart`: the cases of 04 13.3 for unsent, sending and sent entries.
- `pause_start_test.dart`: nothing is sent while paused; `start` resumes in FIFO order.
- `stuck_test.dart`: an entry waiting longer than `client.outbox_stuck_after` reports `delayed`.

Commands that must pass, from the repository root:

- `cd packages/sync_engine && dart test --coverage=coverage` (or the command M4-01 uses for pure Dart members)
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] `5xx` and `429` are retried with the same ID, `4xx` (`400`, `404`, `410`, `413`, `422`) are final; the outbox survives a restart.
- [ ] FIFO per thread with threads in parallel; echo matching confirms entries; uploads are sent before the message that needs them.
- [ ] Edit and delete follow 04 13.3; "Sending delayed" after `client.outbox_stuck_after`.
- [ ] The outbox sends only while started; executors are registered per operation.
- [ ] 15 7.5 updated; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 04 13.
- `packages/sync_engine/README.md` (English) as in Scope.
- 15 7.5 updated; rulings of Design note 7 in `docs/DECISIONS.md` section 10.
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
4. The outcome class of every status code and problem code the tests cover, and the list of registered executors with their FIFO key.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
