# M1-62: Moving threads

Status: ready | Depends on: M1-54, M1-61 | Milestone: M1

## Goal

A person with `manage` on the source and on the target channel moves a thread into another channel whose effective operator equals the thread's fixed operator. The thread keeps its id; its messages, read states and drafts follow in the same transaction; readers of the target get `thread.moved`, readers of the source get `thread.moved_out`; and no authorization decision after the commit evaluates the thread at its old place. This is the function 'Move thread' of 01 and the baseline rule that a thread moves only within the same operator agent.

## Read first

- `docs/spec/03-data-model.md`: `threads` (the paragraph 'Moving changes channel_id' after the indexes, `operator_agent_id`, `moved_at`, the unique index on `kind = main`), `thread_tags` (the rule that a tag belongs to the thread's channel and the last sentence), `messages.channel_id`, `read_states.channel_id`, `drafts.channel_id`, `channels` (formula of the effective operator), `spaces.default_operator_agent_id`, 'I1 check', 'Lock order', R-DM-11, R-DM-12, R-DM-21.
- `docs/spec/05-actions.md`: `thread.move` in 'Threads and agent sessions'; 'Target of a resource' (files follow their message); 'Effective level' (operator rule); rule 8; 'Decision'; rows `thread.moved` and `thread.moved_out` in 'Durable event types and receive action'.
- `docs/spec/04-sync-protocol.md`: 'Visibility on permission changes' with 'Basic rule' (structure changes such as `thread.moved` are applied in log order), 'Editing and deleting' (row 'Thread moved'), 'Delivery guarantee'.
- `docs/spec/02-architecture.md`: 'Ports, events and sources per module' (`chat` emits `ThreadMoved`; `messaging` and `sync` subscribe), 'Permission cache', 'Transactions and Unit of Work', 'In-Tx events', 'Alignment with the core contract' (row on moving threads).
- `docs/spec/07-permissions.md`: 'Capabilities' (`manage` covers moving threads), 'The `authorize` function' (visibility before requirement), 'Validity of the cache'.
- `docs/spec/01-product.md`: 'Structure and conversations' (row 'Move thread'), 'Other rules' (Moving).
- `docs/spec/06-api.md`: 'Errors' (`operator_mismatch`, `conflict`), 'Idempotency'.
- `docs/api/openapi.yaml`: `moveThread`, schemas `ThreadMove`, `Thread`, `ThreadMovedEvent`, `ThreadMovedPayload`, `ThreadMovedOutEvent`, `ThreadMovedOutPayload`, response `Conflict`.
- ADRs 0017, 0018, 0021.
- `docs/PLAN.md`: 'Way of working per ticket', M1 rows M1-53 to M1-55, M1-61, M1-65 and M1-70; 'Prerequisites from the core contract' row on 02 point 6.
- Code: `internal/chat` (threads M1-55, operators M1-54, channels M1-53), `internal/messaging`, `internal/sync` (M1-61), `internal/authz` (M1-28), `internal/kernel/authorize`, `internal/kernel/auditing`, `internal/wire`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, permission or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. A thread moves only to a channel whose effective operator equals `threads.operator_agent_id`; otherwise `409 operator_mismatch` (03 `threads`, 01, 06 'Errors').
12. `thread.move` needs `manage` on source and target, is audited (Audit `yes`), uses rate-limit class `rl.admin` and accepts `Idempotency-Key` (05, openapi).
13. The thread id never changes (03 `threads.id`). The same transaction changes `messages.channel_id`, `read_states.channel_id` and `drafts.channel_id`; `cost_entries` and `event_log` keep the old channel; attached files follow their message and `files.scope_channel_id` stays (03 `threads`, 05 'Target of a resource').
14. Events `thread.moved` (place: target channel, receive action `thread.read`) and `thread.moved_out` (place: source channel, resource `channel`, receive action `channel.read`; it never names the target) (05, openapi).
15. After the commit no REST decision uses the old location: `authorize` resolves the thread's path at decision time (07 4.2). The gateway's permission cache applies `thread.moved` in log order; that side is M1-65 (04 'Basic rule', 02 'Permission cache').
16. No read permission gives `404`; `403` only with read permission but missing `manage` (05 rules 1 and 2, R-AZ-01).

## Scope

All paths under `services/server/` unless noted.

Create:

- `internal/chat/domain/move.go`: pure rule `CanMove(thread ThreadState, source, target ChannelState, targetEffectiveOperator *uuid) error` returning domain errors for every case in Design note 1b.
- `internal/chat/app/move.go`: the use case.
- `internal/chat/adapters/postgres/queries/threads_move.sql`: lock queries, the thread update, deletion of `thread_tags` of the thread.
- `internal/messaging/app/subscribers.go` (or the existing subscriber file): `OnThreadMoved`, with the query `MoveThreadMessages` in `internal/messaging/adapters/postgres/queries/`.
- In `internal/sync/app/subscribers.go`: `OnThreadMoved`, with the queries `MoveThreadReadStates` and `MoveThreadDrafts`.

Change:

- `internal/chat` port: `Threads.Move(ctx, MoveThread{ThreadID, TargetChannelID uuid; Reason string}) (Thread, error)`; in-transaction event type `ThreadMoved{ThreadID, FromChannelID, FromSpaceID, ToChannelID, ToSpaceID uuid; ThreadVersion int64}`.
- `internal/chat/adapters/httpapi`: the strict-server method for `moveThread`.
- `internal/authz` and `internal/kernel/authorize`: the requirement for `thread.move` checks `manage` on the thread's path and on the target channel's path, with the second target in the resource descriptor (07 4.2 "Actions with two targets"; 02 4.5). M1-28 already provides the descriptor field `TargetChannelID` (`thread.move`: destination channel) and the requirement `All(Cap(manage), TargetCap(manage))` for "manage on source and target"; set `TargetChannelID` in the descriptor and add no second field.
- `internal/wire`: register `chat.ThreadMoved -> messaging.OnThreadMoved` and `chat.ThreadMoved -> sync.OnThreadMoved` (in this order) and the handler; update the golden file of subscriptions.
- `api/operations.yaml`: enable `moveThread`; rerun the route generator.
- READMEs of `chat`, `messaging`, `sync`, `authz`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md`.

## Out of scope

- Delivery of `thread.moved` and `thread.moved_out` over the gateway, the hub indices and the gateway's permission cache (M1-65); this ticket writes the events and makes REST decisions follow the move.
- The lint for the visible-channel predicate (AZ-P12, M1-70) and the full leak tests (M1-70).
- Moving a channel between spaces (`moveChannel`, M1-53) and space reordering (M1-52).
- Runs, agent sessions and costs of a moved thread (M2); none reference the channel (03 `runs`, `agent_sessions`; `cost_entries` keeps the booking channel by design).
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-62-chat-thread-move`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(chat): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).

## Design notes

1. Use case `Threads.Move`, one unit of work:
   a. Load the thread (unknown id gives `404`). Build the descriptor `{kind: thread, id, TargetChannelID: target_channel_id}` and call `authorize(actor, thread.move, descriptor)`. `authz` decides in this order: no read permission on the thread gives `not_found`; no read permission on the target or an unknown target gives `not_found` with exactly the body and headers of an unknown thread id; missing `manage` on the source or the target gives `forbidden` (`capability_missing`). Owners have `manage` everywhere except DM channels (05 'Effective level').
   b. Domain checks after `authorize`, in this order: thread `kind = main` gives `409 conflict` (each text channel keeps exactly one main thread, R-DM-11 and the unique index); target equal to the source gives `400 validation_failed` with pointer `/target_channel_id`; archived target gives `409 conflict`; a DM channel as source or target gives `409 conflict` (not reachable after `authorize`, kept as defence in depth); effective operator of the target differs from `threads.operator_agent_id` gives `409 operator_mismatch`. Operators compare with `IS NOT DISTINCT FROM` semantics: a thread without operator moves only into a channel without effective operator.
   c. `reason` (required by `ThreadMove`): non-empty after trimming, no control or bidi characters (reuse the shared validator `kernel/textcheck` of M1-26, SEC-IN-4), otherwise `400 validation_failed` with pointer `/reason`.
   d. Locks, in this order: `threads` row `FOR UPDATE`; target `channels` row `FOR SHARE`; when the target inherits its operator, the target's `spaces` row `FOR SHARE`. Compute the effective operator only after these locks. Reason: `channel.operator.set` and `space.operator.set` update these rows, so the share lock serializes the operator check against a concurrent operator change and the move cannot break I1 (03 'I1 check', target kind 2). Deadlocks are retried by the unit of work (`database.tx_retry_max`).
   e. Update the thread: `channel_id = target`, `moved_at = now()`, `version = version + 1`. Delete all `thread_tags` of the thread: a tag must belong to the thread's channel (03 `thread_tags`), so tags never survive a move.
   f. Publish `chat.ThreadMoved`. `messaging.OnThreadMoved` runs `UPDATE messages SET channel_id = $to WHERE thread_id = $thread`; `sync.OnThreadMoved` runs the same for `read_states` and `drafts`. These are copies: no version change, no events from the subscribers; clients learn the new channel from `thread.moved`.
   g. Append, in this order, `thread.moved_out` then `thread.moved`, so a client that receives both ends with the thread in the target:
    - `thread.moved_out`: `scope {space_id: source space, channel_id: source}`, `resource {kind: channel, id: source}` without `version` (the channel object did not change, so a version would make clients drop the event, 04 'Delivery guarantee' point 3), payload `ThreadMovedOutPayload {thread_id, channel_id: source}`, receive action `channel.read`.
    - `thread.moved`: `scope {space_id: target space, channel_id: target, thread_id}`, `resource {kind: thread, id, version: new version}`, payload `ThreadMovedPayload {thread: the full Thread after the move, from_channel_id}`, receive action `thread.read`.
    h. Audit in the same transaction through `auditing.Recorder`: action `thread.move`, resource the thread, details `from_channel_id`, `to_channel_id` and `reason`, nothing else. Denials are audited by the authz layer like every action with Audit `yes` (R-AZ-04).
    i. Answer `200` with the `Thread` after the move. `Idempotency-Key` is handled by the middleware of M1-20.
2. The permission version is not raised and no `access.changed` is written (05: Permissions `no`). I1 needs no extra check: the target's effective operator is the thread's operator, so the target already is one of that agent's targets, and the source can only drop out (03 'I1 check'); note 1d keeps this true under concurrency.
3. A thread created from a main-thread message keeps `parent_message_id`, which now points into the source channel; readers of the target without read permission on the source get `404` for that message (03 `threads.parent_message_id`).
4. Permissions: REST decisions resolve the thread's channel at decision time (07 4.2) and cache no thread-to-channel resolution across requests; state that in the authz README. The gateway's cache side of `thread.moved` is M1-65.
5. `MoveThreadMessages` has no visible-channel predicate by design: it writes the rows of a thread that `authorize` checked in the same transaction (07 7.4 rule 1: writes are outside the lint). Name it in the messaging README with this reason.
6. Rulings for `docs/DECISIONS.md`: event order `thread.moved_out` before `thread.moved`; threads without operator move only to channels without effective operator; status codes for same channel (`400`) and archived target (`409 conflict`). Main threads never move, tags fall away on every move, `thread.moved_out` has no `resource.version` and `parent_message_id` stays: these are in 03 and 04.

## Tests (write them first)

Unit:

- `internal/chat/domain`: `TestCanMove` with a table for main thread, same channel, archived target, DM source or target, equal operator, different operator, both without operator, thread with operator and a target that inherits a different space default.
- `internal/authz`: `TestThreadMoveRequirement`: `manage` on source only gives `forbidden`; target invisible gives `not_found`; `manage` on both gives `allow`; Owner gives `allow`; DM participant gives `forbidden`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain, people from the identity test helpers; the world uses space Support with channels Allgemein and Eskalation, both with the same effective operator, and space Marketing with another operator):

- `TestMoveThreadHappyPath`: a thread with three messages, read states of Alex and Sam, a draft of Alex and two forum tags moves from Support/Allgemein to Support/Eskalation: same id; every `messages.channel_id`, `read_states.channel_id` and `drafts.channel_id` equals the target; `moved_at` set; `version` raised by one; no tags left.
- `TestMoveThreadOperatorMismatch`: target in Marketing gives `409 operator_mismatch`; nothing changed; no events, no audit allow entry.
- `TestMoveThreadNeedsManageOnBoth`: `manage` on the source only gives `403`; an invisible target gives `404` with the same status, body and headers as an unknown target id.
- `TestMoveMainThreadRejected`: `409 conflict`.
- `TestMoveThreadSameChannel`: `400 validation_failed`.
- `TestMoveThreadEvents`: exactly `thread.moved_out` followed by `thread.moved` with consecutive `seq`; the `thread.moved_out` row contains the target channel id nowhere; both payloads validate against their schemas; no JSON `null`.
- `TestMoveThreadAudit`: one allow entry with from, to and reason; a denied attempt by Sam produces a deny entry.
- `TestMoveThreadAuthorizeFollowsMove`: two `Authorizer` instances with warm snapshot caches (REST mode, no log events fed): after the move a person who reads only the target gets `allow` for `thread.read` and a person who reads only the source gets `not_found`, in both instances.
- `TestMoveThreadConcurrentReadState`: parallel `putReadState` of Alex and the move: afterwards `read_states.channel_id` equals `threads.channel_id`.
- `TestMoveThreadConcurrentOperatorChange`: parallel `setChannelOperator` on the target to another agent and the move: both requests end with a defined status (no deadlock reaches the client) and `VerifyInvariants` of `authz` passes afterwards.
- `TestMoveThreadIdempotencyKey`: a replay returns the stored response with `Idempotent-Replay: true` and writes no second set of events.
- `TestSendAfterMove`: `createMessage` to the moved thread id succeeds and the new message has the target `channel_id` (04 'Editing and deleting').

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/chat/... ./internal/messaging/... ./internal/sync/... ./internal/authz/... ./internal/kernel/... ./internal/wire/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make register-check`
- `make check`

## Acceptance criteria

- [ ] `moveThread` is enabled and registered; AZ-P1 passes.
- [ ] A different effective operator gives `409 operator_mismatch`; `manage` is required on source and target; an invisible target gives the same `404` as an unknown one.
- [ ] The thread id stays; `messages.channel_id`, `read_states.channel_id` and `drafts.channel_id` follow in the same transaction; tags are removed.
- [ ] `thread.moved_out` (source, without target) and `thread.moved` (target) are written in this order and validate against their schemas.
- [ ] The move is audited with from, to and reason; denials are audited.
- [ ] After the commit no REST `authorize` call in any process instance evaluates the thread at the source.
- [ ] Concurrent read-state writes and operator changes leave consistent copies and I1 intact.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `chat`, `messaging`, `sync` and `authz` updated (ports, events, subscriptions, the query exception of note 5, cache behaviour of note 4).
- Rulings of note 6 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Merge locally into the base branch only if review, red team and `make check` are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
