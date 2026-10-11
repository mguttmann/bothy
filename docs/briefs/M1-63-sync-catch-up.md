# M1-63: Catch-up over REST and snapshot

Status: ready | Depends on: M1-61 | Etappe: M1

## Goal

`GET /api/v1/events` delivers every durable event after a cursor that the caller may see at delivery time, with `authorize` per event and a cursor that means 'last checked', or `410 resync_required` with a reason. `GET /api/v1/sync/snapshot` delivers structure, counters and versions without thread contents. Both are built as the `sync` ports `CatchUp` and `Snapshot`, which the gateway reuses for `ready`, `resume` and streaming catch-up (M1-64, M1-65), so REST and WebSocket follow exactly the same rules (04 'REST-Variante').

## Read first

- `docs/spec/04-sync-protokoll.md`: 'Umschlag (Envelope)', 'Globale, commit-geordnete Sequenz', 'Nachholen (Catch-up)' with 'Cursor', 'Ablauf', 'resync_required', 'REST-Variante', 'Sync-Epoche'; 'Lebenszyklus' step 4 (content of the snapshot); 'Sichtbarkeit bei Rechte-Änderungen'; 'Zustellgarantie'; sequence diagram 'Wiederverbinden mit Nachholen'; decisions R-SY-03, R-SY-04, R-SY-16, R-SY-18.
- `docs/spec/03-datenmodell.md`: `event_log`, `sequence_counters`, `instance.sync_epoch`, 'Sync-Epoche und Wiederherstellung', `read_states`, `threads` (`last_message_seq`, `unread_floor_seq`), `drafts`, `user_settings`, 'Grundsätze' point 12, R-DM-26.
- `docs/spec/05-aktionen.md`: `sync.snapshot.read`, `events.catch_up` ('Persönliche Daten, Benachrichtigung, Sync'); 'Scopes für Apps' (`events:subscribe`); 'Dauerhafte Ereignistypen und Empfangs-Aktion'.
- `docs/spec/07-rechte.md`: 'Rechte-Stand (Snapshot)', 'Gültigkeit des Caches', 'Abfrage-Muster', 'Zustellfilter', AZ-P5 in 'Pflicht-Tests'.
- `docs/spec/02-architektur.md`: 'Regeln' of the module list (point 1: only the owning module reads its tables), 'Ports, Ereignisse und Quellen je Modul' (`sync`: `CatchUp`, `Snapshot`), 'Erlaubte Abhängigkeiten', 'Querschnitts-Ports' (`Authorizer.Scope`), 'Log-Tail je api-Prozess', 'Rechte-Cache', 'Leistungsziele' (`perf.catchup_page_p99`, measured by M1-72).
- `docs/spec/06-api.md`: 'Fehler' (`resync_required`), 'Paginierung' points 3 and 5, 'Weitere Punkte' K19.
- `docs/api/openapi.yaml`: `listEvents`, `getSnapshot`; schemas `EventPage`, `Event`, `EventEnvelope`, `Snapshot`, `SpaceView`, `ChannelView`, `SidebarState`, `UnreadThread`, `RunSummary`, `AgentPresence`, `AgentRuntimeState`, `UserSetting`, `DraftRef`, `PrincipalRef`, `EvaluatedFlag`, `InstanceInfo`, `Problem`, response `Gone`.
- ADRs 0009, 0013, 0017, 0052, 0053, 0059.
- `docs/PLAN.md`: 'Arbeitsweise je Ticket', M1 rows M1-16, M1-28, M1-47, M1-51 to M1-53, M1-61, M1-64, M1-65, M1-70, M1-72; P-12.
- Code: `internal/sync` (appender, tail, envelope encoder of M1-16, personal data of M1-61), `internal/instance` (`Versions`, sync epoch, `Flags`), `internal/authz` (`Scope`), `internal/chat` (space and channel views of `listSpaces` and `listSpaceChannels`), `internal/agents` (listing behind `listAgents`), `internal/identity` (`Directory`).

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
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossar.md`, `03-datenmodell.md`, `04-sync-protokoll.md`, `05-aktionen.md`) is not edited by this ticket.

Product requirements implemented here:

11. The cursor is the highest `seq` checked for the principal, not the highest delivered; invisible events advance it (04 'Cursor', R-SY-03).
12. Filter: the SQL predicate of 04 'Ablauf' point 1 for performance and `authorize(principal, receive_action, resource)` per event as the truth; rights at delivery time apply (04 'Ablauf' point 2, 'Grundregel').
13. `410 resync_required` with the reasons `epoch_changed`, `retention`, `too_far_behind`, `cursor_ahead` (04 'resync_required', 'Sync-Epoche').
14. Events with a personal audience go only to that principal (05 'Dauerhafte Ereignistypen und Empfangs-Aktion', 07 'Zustellfilter').
15. The snapshot holds visible spaces and channels with versions, the sidebar state, unread and mention counters, running runs, agent presence, versions of settings and drafts and the evaluated feature flags, and no thread contents (04 'Lebenszyklus' step 4).
16. Counters and snapshot never consider a channel without read right (AZ-P5, snapshot part).
17. No JSON `null` (ADR 0053).

## Scope

All paths under `services/server/` unless noted.

Create:

- `internal/sync/domain/catchup.go`: resync decision `func ResyncReason(after int64, epoch *uuid, head, minRetained int64, current uuid, maxEvents int64) Reason` and the next-cursor rule.
- `internal/sync/app/catchup.go` and `internal/sync/app/snapshot.go`.
- Port additions in `internal/sync`:
    - `CatchUp.Check(ctx, CheckInput{After int64; Epoch *uuid}) (CheckResult{HeadSeq, MinRetainedSeq int64; SyncEpoch uuid; Reason ResyncReason}, error)` where `ResyncReason` is empty or one of `epoch_changed`, `cursor_ahead`, `retention`, `too_far_behind`.
    - `CatchUp.Page(ctx, PageInput{After, Head int64; Limit int}) (PageResult{Events []EnvelopeJSON; NextCursor int64}, error)` for the actor in the context; `EnvelopeJSON` is the wire form of the M1-16 encoder.
    - `Snapshot.Build(ctx) (Snapshot, error)` for the actor in the context; the DTO mirrors the `Snapshot` schema.
- `internal/sync/adapters/postgres/queries/catchup.sql`: the page query, `min(seq)` of `event_log`.
- `internal/sync/adapters/httpapi/sync.go`: strict-server methods for `listEvents` and `getSnapshot`.
- Outgoing port `RunSummarySource` in `internal/sync/app` and its M1 binding `internal/sync/adapters/noruns` (see note 9).

Change:

- The specification is in place: `docs/api/openapi.yaml` has the optional query parameter `epoch` on `listEvents` and the required property `features` on `Snapshot` (with example), and 02 'Erlaubte Abhängigkeiten' lets `sync` use `identity`. Change them only where the implementation proves them wrong. Regenerate code (`make codegen-check`).
- `architecture/modules.yaml`: add `identity` to `depends_on` of `sync` as in 02; regenerate `.go-arch-lint.yml`; `make register-check` green.
- Port additions in `internal/chat`, additive and only if absent: `Threads.Counters(ctx, channelIDs []uuid) ([]ThreadCounter{ThreadID, ChannelID uuid; LastMessageSeq, UnreadFloorSeq int64}, error)` returning only threads with `last_message_seq > unread_floor_seq`, query with `channel_id = ANY($1)`.
- If `kernel/authorize` has no constructor for a resource descriptor from an event row, add `authorize.ResourceFromEvent(kind string, id, spaceID, channelID, threadID uuid) Resource` with tests in `internal/authz`; the gateway hub (M1-65) uses the same constructor.
- `api/operations.yaml`: enable `listEvents` and `getSnapshot`.
- `internal/wire`: handlers and the `noruns` binding.
- ADR 0059: extend its "Confirmation" (note 12).
- READMEs of `sync`, `chat`, `authz` (if changed); `services/server/CHANGELOG.md`; `docs/DECISIONS.md`.

## Out of scope

- Streaming catch-up over the WebSocket with `cursor` frames, buffering of live events and `catchup.complete` (M1-65); this ticket provides `Check` and `Page` for it.
- Agent consumer cursors, `acked_seq` and redelivery (M2-07); delivery-only fields `invocation_token` and `redelivered` (M2).
- Running runs from the `runs` module (M2-05) and agent presence from heartbeats (M2-20).
- `bothy restore finalize`, which writes a new epoch (M7).
- Metrics `bothy.gateway.resync` and others (M1-68); the load test for `perf.catchup_page_p99` (M1-72); the full AZ-P4 and AZ-P5 suites over all routes (M1-70).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-63-sync-catch-up`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(sync): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the CHANGELOG, README, ADR and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).

## Design notes

Catch-up

1. `HeadSeq` is the current value of `sequence_counters('event_seq')`, read without a lock through `instance.Versions` (02 "Ports, Ereignisse und Quellen je Modul"; add `EventSeq(ctx) (int64, error)` next to `PermissionVersion` of M1-28). Never through `instance.Counters`: its phase guard (M1-15) rejects calls outside the phase hooks. Reason: after retention the log can be empty while the counter is not, and `max(seq)` would then report `cursor_ahead` wrongly. `MinRetainedSeq` is `min(seq)` of `event_log`, or `HeadSeq + 1` when the log is empty. `SyncEpoch` is `instance.sync_epoch` from the instance port. Because a commit is visible before its counter lock is released, every `seq <= HeadSeq` is visible when `HeadSeq` is read (04 'Muster mit Zähler-Zeile').
2. Check order (first match wins): `epoch` given and different from `SyncEpoch` gives `epoch_changed`; `After > HeadSeq` gives `cursor_ahead`; `After < MinRetainedSeq - 1` gives `retention`; `HeadSeq - After > sync.catchup_max_events` gives `too_far_behind`. Without `epoch` there is no epoch check; `cursor_ahead` stays the second net (R-SY-04).
3. `Page`: get the visible sets from `Authorizer.Scope(ctx, actor)`: visible channel ids, visible space ids (at least one visible channel), fully visible space ids (grant on the space) and the instance-wide flag (Owner or grant on the instance). Run exactly the predicate of 04 'Ablauf' point 1 with `seq > $after AND seq <= $head ORDER BY seq LIMIT $limit`. Then per row: if `audience_principal_id` is set, deliver only when it equals the actor; otherwise call `authorize(actor, receive_action, authorize.ResourceFromEvent(...))` in filter mode `authorize.Filter(ctx)` (M1-28; no denial audit, 07 4.2) and deliver only on `allow`. Never query content tables per event.
4. `NextCursor`: if the query returned `limit` rows, the `seq` of the last row; otherwise `Head`. Rows dropped by `authorize` or the audience rule still count as checked.
5. Envelope: use the M1-16 encoder (no `null`, `redacted: true` when `redacted_at` is set, `correlation_id`). `trace_context` only when the actor is an agent (04 'Umschlag'). No delivery-only fields in M1.
6. `listEvents`: `authorize(actor, events.catch_up, instance)` (apps need `events:subscribe`, 05 'Scopes für Apps'). Parameters: `after` required and at least 0; `epoch` optional; `limit` defaults to `sync.catchup_page_size` and larger values are capped to it (06 'Paginierung' point 3). A resync reason gives `410` with problem code `resync_required` and `reason` set to the reason. Otherwise `200 EventPage {events, next_cursor, head_seq, sync_epoch}`; `next_cursor` is present even when `events` is empty.

Snapshot

7. `getSnapshot`: `authorize(actor, sync.snapshot.read, instance)`; a pending device gets `403 device_pending` from `authorize`.
8. Consistency: read `head_seq` and `sync_epoch` first, then all other parts (READ COMMITTED). Every change with `seq <= head_seq` is then contained; later changes either are contained too or arrive by catch-up after `head_seq`, and duplicates are harmless (04 'Zustellgarantie').
9. Parts and their sources:
    - `head_seq`, `sync_epoch`: note 1.
    - `permission_version`, `is_owner`: from `Authorizer.Scope`.
    - `principal`: `PrincipalRef` of the actor from `identity.Directory` (new allowed edge `sync -> identity`).
    - `spaces`, `channels`: `SpaceView` and `ChannelView` of the visible, not archived spaces and channels with level and capabilities, through the chat port listing that serves `listSpaces` and `listSpaceChannels`; DM channels of participation are included.
    - `sidebar`: `Settings.ListSidebar` of M1-61.
    - `unread_threads`: `Threads.Counters` for the visible channel ids, combined in Go with the actor's `read_states` of those channels: unread = `last_message_seq - GREATEST(last_read_seq, unread_floor_seq)` (never below 0), and the unread mentions per thread from `messaging.Mentions.CountUnread` (M1-60) with the same read positions (computed, never stored, 03 R-DM-32); a thread is listed when unread is above 0 or its mention count is above 0; each listed thread is checked with `authorize(actor, thread.read, thread)` in filter mode `authorize.Filter(ctx)` and dropped on deny (07 'Abfrage-Muster' rule 3). No SQL join across module tables (02 'Regeln' point 1); the join in 07 'Abfrage-Muster' illustrates the result, not the place of the query.
    - `active_runs`: from `RunSummarySource`. In M1 `internal/wire` binds `internal/sync/adapters/noruns`, which returns an empty list, because the table `runs` exists only from M2-05; its doc comment says so and names M2-05 as the ticket that binds the `runs` port.
    - `agents`: agents the actor may read (`agent.read`) from the agents port listing behind `listAgents`, each with `state: unknown` and without `reported_billing_mode`, because no heartbeat exists before M2-20 (`AgentRuntimeState` documents `unknown`).
    - `settings`: all stored rows of the person (M1-61).
    - `drafts`: `DraftRef` of the person's drafts in channels the actor can still read (rule of M1-61).
    - `features`: the flags evaluated for the actor from `instance.Flags` (M1-47).
    - For agents and apps `sidebar`, `unread_threads`, `settings` and `drafts` are empty arrays.
10. No message text, no draft text, no thread title beyond what `ChannelView` and `SpaceView` already carry; the snapshot carries no thread contents (04).
11. Rulings for `docs/DECISIONS.md`: head from the counter (note 1); check order (note 2); `active_runs` empty and agents `unknown` until M2-05 and M2-20 (note 9); unread computed through ports (note 9); `trace_context` only for agents (note 5).
12. ADR 0059 (already in the repository): add `TestListEventsResync` (case `epoch_changed`) to its "Confirmation" without changing the decision, then `make adr-index`.
13. Configuration keys used: `sync.catchup_page_size`, `sync.catchup_max_events`, `sync.event_retention` (through M1-16's partition maintenance in tests).

## Tests (write them first)

Unit, `internal/sync/domain` and `internal/sync/app` with fakes:

- `TestResyncReasonOrder`: table covering each reason, combinations (epoch wrong and cursor ahead gives `epoch_changed`), an empty log, `After` exactly `MinRetainedSeq - 1`.
- `TestNextCursor`: page full, page not full, all rows dropped by `authorize`.
- `TestPersonalEventsOnlyToAudience`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain; Alex reads Support, Sam reads Support and Marketing, Robin reads nothing):

- `TestListEventsSkipsInvisible`: events of Support and Marketing interleaved; Alex gets only Support events; paging with a small `limit` reaches `head_seq`; the cursor passes over Marketing ranges.
- `TestListEventsCursorIsLastChecked`: a full page ends at the `seq` of its last row.
- `TestListEventsPersonalAudience`: Sam's `read_state.updated` in Support is not delivered to Alex; Sam receives it.
- `TestListEventsResync`: cases `retention` (partition removed through M1-16's maintenance with a controlled clock), `too_far_behind` (test configuration with a small `sync.catchup_max_events`), `cursor_ahead`, `epoch_changed`; each gives `410` with the right `reason`.
- `TestListEventsNewChannelInVisibleSpace`: a channel created in a space where Alex has a space grant after Alex's rights snapshot was cached: its events are delivered.
- `TestListEventsAfterRevoke`: after Alex's grant on Support is revoked, no Support event is delivered to Alex, also none before the revoke that Alex had not fetched yet (rights at delivery time).
- `TestListEventsRedacted`: a deleted message's event comes with `redacted: true` and the redacted payload.
- `TestListEventsLimitCapped`: `limit` above `sync.catchup_page_size` returns at most that many events.
- `TestListEventsAppNeedsScope`: an app without `events:subscribe` gets the same response as without read right on the instance resource per `authorize`.
- `TestSnapshotContents`: a world with spaces, channels, threads, read states, mentions, drafts and settings: every part as in note 9; responses validate against `Snapshot`, `features` present.
- `TestSnapshotHeadReadFirst`: with a concurrent writer, every change with `seq <= head_seq` is contained in the snapshot.
- `TestAZP5SnapshotNoForeignChannel`: property test over random worlds (reuse the world generator of AZ-P8 from `internal/authz` tests if it is exported; otherwise build worlds through the public ports): no entry of `spaces`, `channels`, `unread_threads`, `drafts` or `sidebar` and no counter refers to a channel without read right. Run a fixed number of worlds with a logged seed in `make test`.
- `TestSnapshotAgentHasEmptyPersonalParts`.
- `TestSnapshotHasNoContent`: no `content_md` member anywhere in the response.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/sync/... ./internal/chat/... ./internal/authz/... ./internal/kernel/... ./internal/wire/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make register-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `listEvents` and `getSnapshot` are enabled and registered; AZ-P1 passes.
- [ ] The cursor is 'last checked' and passes over invisible events; `next_cursor` is present on every page.
- [ ] `410 resync_required` with `retention`, `too_far_behind`, `cursor_ahead` and `epoch_changed`, in the order of note 2.
- [ ] Personal events reach only their audience; all other events pass `authorize` with their receive action at delivery time.
- [ ] The snapshot contains every part of note 9 and validates against the schema, including `features`.
- [ ] AZ-P5 (snapshot part) passes; no response contains thread contents.
- [ ] Generated code matches openapi.yaml (`epoch` on `listEvents`, `features` on `Snapshot`).
- [ ] `sync -> identity` is in `modules.yaml` and 02; `make register-check` and `make arch` are green.
- [ ] ADR 0059 "Confirmation" names `TestListEventsResync` (case `epoch_changed`); its decision text is unchanged (`git diff` of the ADR touches only "Confirmation").
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `sync` (ports `CatchUp` and `Snapshot`, dependencies, the `noruns` binding), `chat` and, if changed, `authz`.
- Rulings of note 11 recorded in `docs/DECISIONS.md` (one row each with reason); ADR 0059 "Confirmation" extended.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Merge locally into the base branch only if review, red team and `make check` are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example the page latency, measured only by M1-72).
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
