# M3-11: Channel export

Status: ready | Depends on: M1-57, M3-05 | Milestone: M3

## Goal

A person with the level `read` and the capability `export` on a channel, acting in a session with a fresh step-up, downloads the history of that channel for a time range: `GET /api/v1/channels/{channel_id}/export?from=...&to=...` (`exportChannel`) streams one `Message` per line as `application/x-ndjson`, thread by thread in `thread_seq` order, built by the same representation as the history of M1-57. The export contains only what the person may read at the moment each line is sent: every query carries the visible-channel predicate, every line passes `authorize(message.read)`, deleted messages appear only as tombstones, and a revocation during the stream ends it visibly. The start of every export is in the audit log before the first byte leaves the server, denials are audited by `authz`, and the rate-limit class `rl.export` throttles it. This is the function "Export of a channel" of 01 and the action `export.channel` of 05; the desktop channel menu (M4-33) builds on it.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 9 "M3 Platform services" (Goal, Scope, Out of scope, Exit criteria, Test strategy, Red team, Risks, the row M3-11 and the neighbouring rows M3-05, M3-07, M3-10 and M3-12); section 10 row M4-33; section 13 rows M7-06 and M7-08; section 7 rows M1-19, M1-50, M1-57, M1-69, M1-70 and M1-71.
- `docs/briefs/README.md` ("How a brief runs") and the briefs this ticket builds on: `M1-57-messaging-history.md` (read path, `MessageView`, tombstone representation, `TestQueriesCarryVisibleChannelPredicate`), `M1-56-messaging-send.md` (the `Message` representation), `M1-28-authz-core.md` (`Authorizer.Authorize`, `Authorizer.Scope`, `authorize.Filter`, `authorize.WithRequestScope`, the requirement for "read and +export", the order of checks, `writes(a)`), `M1-35-identity-step-up.md`, `M1-34-identity-sessions.md` (`RevokeSession`), `M1-24-audit-chain.md` (`auditing.Recorder`, `Entry`, `Attrs`), `M1-18-http-chain.md` (chain, deadlines, streamed bodies, proof of the check, recover stage), `M1-17-api-server-stubs.md` (`api/operations.yaml`, strict server, `api/routes`), `M1-19-rate-limits.md` (class `rl.export`), `M1-50-audit-verify.md` (the NDJSON export `exportAuditEvents`, same pattern), `M1-53-chat-channels.md` (`archiveChannel`), `M1-13-unit-of-work.md` (`txn.UnitOfWork`), and the M3-05 brief `M3-05-file-download.md` (streamed downloads and their deadline).
- Dependencies implied through M1-57 and M3-05 and therefore not in the header: M1-56, M1-55, M1-53, M1-35, M1-34, M1-28, M1-24, M1-18, M1-17, M1-13, M1-12 (`pgtest`), M3-04, M3-03, M3-01. M1-19, which this ticket needs (Design note 1), is in the dependency closure too.
- `docs/spec/01-product.md`: "3.4 Files and voice" (row "Export of a channel").
- `docs/spec/02-architecture.md`: "4.3 Ports, events and sources per module" (`messaging` port `Export`), "4.4 Allowed dependencies", "4.5 Cross-cutting ports (`internal/kernel`)", "4.6 Actions per module" (`export.channel` belongs to `messaging`; the bullet "`files` stores the package", Design note 12), "8.5 What never happens in a transaction", "9.1 Chain in the api node", "9.2 404 instead of 403", "9.3 Proof of the check", "14.1 Error kinds", "14.2 Deadlines", "15.3 Shutdown".
- `docs/spec/03-data-model.md`: "1. Principles" items 1, 2, 7, 12 and 13; table `messages` (indexes `(channel_id, created_at DESC)` and unique `(thread_id, thread_seq)`); table `audit_events`; R-DM-20.
- `docs/spec/04-sync-protocol.md`: "4. Thread sequence", "8.1 Basic rule".
- `docs/spec/05-actions.md`: "1. Binding rules" rules 1, 2, 3 and 9; "2.3 Effective level"; 2.4 (the columns Owner, Step-up, Audit and Permissions); "2.5 Decision"; "2.7 Effect (`effect`)"; "3. Rate-limit classes"; row `export.channel` in "4.12 Import, costs, privacy, status"; R-AZ-01 to R-AZ-04 and R-AZ-06 in "8. Decisions of this chapter"; "10. Configuration values of this chapter".
- `docs/spec/06-api.md`: "2. Basic rules" (Media types, No `null`, Caching, Time), "3.3 Step-up", "4. Authorization in the API", "5. Errors", "9. Rate limits", "15. Contract tests and gate" (G7, G8), "16. Endpoints" > "Privacy".
- `docs/spec/07-permissions.md`: "2.1 Subjects" (agents and apps never get `export`), "2.4 Capabilities", "4.1 Decisions and reasons", "4.2 Flow" (visibility before requirement before step-up; the use case writes the allow entry), "4.3 Visibility", "4.5 Archived spaces and channels" ("The data stays for retention and export"), "7.4 Query pattern" rules 1 and 3, "8.2 Immediate effect on revocation", "10. Examples", "11.2 Required tests" (AZ-P1, AZ-P4, AZ-P9, AZ-P12, AZ-P13).
- `docs/spec/08-identity.md`: "6.2 Step-up flow", "15. Tests" (ID-T18).
- `docs/spec/19-security.md`: "3. HTTP security headers" (SEC-HDR-1, SEC-HDR-2), "9. Logs without secrets and without content" (SEC-LOG-3, SEC-LOG-4), "12. Supply chain, backups and audit integrity" (SEC-AUD-4, SEC-AUD-5), "14. Tests" (SEC-T03, SEC-T10).
- `docs/spec/actions.yaml`: entry `export.channel` (`right: read and +export`, `step_up: true`, `audit: always`, `rl_class: rl.export`, `effect: write`).
- `docs/api/openapi.yaml`: operation `exportChannel` (parameter `ChannelIdPath`, query parameters `from` and `to` with schema `Timestamp`, `200` as `application/x-ndjson` with schema `Message`, responses `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `TooManyRequests`, `Error`); schemas `Message`, `Problem`, `FieldError`; for comparison `exportAuditEvents`.
- ADRs in `docs/adr/`: 0008, 0013, 0015, 0017, 0018, 0020, 0021, 0049, 0053, 0065.
- `docs/security/threat-model.md`: T-AZ-01, T-AZ-02, T-AZ-03, T-AZ-04. `docs/RISKS.md`: RK-02.
- `docs/OPEN-QUESTIONS.md`: OQ-19 (direct messages stay invisible to owners; default yes).
- `docs/observability/signals.md`: `bothy.authz.errors` with `bothy.reason = filter_mismatch`; `docs/observability/alerting.md`: `BothyAuthzError`.
- `docs/operations/configuration.md`: `ratelimit.rl.export.per_minute`, `ratelimit.rl.export.burst`, `auth.step_up_max_age`, `api.page_size_max`, `message.list_content_max_bytes`, `http.request_timeout`, `http.transfer_timeout`, `database.statement_timeout`, `database.idle_in_tx_timeout`, `audit.deny_aggregate_window`, `authz.not_found_timing_tolerance`.
- Code: `services/server/internal/messaging` (M1-56, M1-57), `internal/kernel/authorize`, `internal/kernel/auditing`, `internal/kernel/txn`, `internal/kernel/actor`, `internal/kernel/ids`, `internal/authz` with its fakes in `internal/authz/adapters/fake`, `internal/platform/httpserver`, `internal/platform/ratelimit`, `internal/platform/pg/pgtest`, `internal/wire`, `services/server/api/operations.yaml`, `services/server/api/routes`, and the download handlers of M3-05 in `internal/files/adapters/httpapi` (read only, for the streaming pattern).

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
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. `export.channel` needs the level `read` and the capability `export` on the channel (M1-28 maps "read and +export" to `All(Level(read), Cap(export))`), a step-up not older than `auth.step_up_max_age`, writes allow and deny to the audit log, and uses the rate-limit class `rl.export` (05 row `export.channel` and 2.4; 07 2.4; `docs/spec/actions.yaml`).
12. No read right gives `404 not_found`, identical for a foreign and an unknown channel id; read right without `export` gives `403 forbidden` with `reason` `capability_missing`; read and `export` without a fresh step-up give `401 step_up_required`; a principal without a session of a human (agent token, app token, personal API token) is refused with `403 forbidden` (05 rule 2, 2.4 and 2.5; 07 4.2; R-AZ-01, R-AZ-03; 06 3.3; ID-T18).
13. Owners have `export` on every channel except DM channels, and only in a session; agents and apps never have `export`; DM participants have `write` and `upload`, not `export` (05 2.3 items 1, 3 and 5; 07 2.1; R-DM-20; R-AZ-02; R-AZ-14; OQ-19).
14. `exportChannel` takes the required query parameters `from` (inclusive) and `to` (exclusive) and answers `200` as `application/x-ndjson` with one `Message` per line in `thread_seq` order per thread; the body streams (06 "Basic rules", Media types; openapi `exportChannel`).
15. Only visible content: every SQL statement of the export carries `channel_id = ANY($visible)` with the list from `Authorizer.Scope`, and every line passes `authorize(message.read)` in filter mode; a difference drops the line and counts `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 7.4 rules 1 and 3; AZ-P12; signals.md).
16. A deleted message appears only as a tombstone without content and keeps its `thread_seq` (03 "Principles" item 7 and `messages`; 04 "Thread sequence").
17. Rights at the time of delivery: once a revocation has committed, the export sends no further line (07 8.2 item 2; 04 8.1).
18. No JSON `null` in any line or response (06 "Basic rules"; 03 "Principles" item 13; ADR 0053).
19. The use case writes the allow entry with its details; the `authz` layer writes denials, aggregated per `audit.deny_aggregate_window`; `details` hold only codes, IDs and times (07 4.2; 05 2.4; 19 SEC-AUD-5; R-AZ-04; AZ-P13).
20. `rl.export` counts per principal with `ratelimit.rl.export.per_minute` and `ratelimit.rl.export.burst`; an exhausted bucket gives `429 rate_limited` with `Retry-After`, checked before `authorize` (05 3; 06 9).
21. No transaction stays open while lines go to the client, and every statement runs under `database.statement_timeout` (02 8.5 and 14.2, `database.idle_in_tx_timeout`); the response runs under `http.transfer_timeout` (02 14.2, "upload and download").
22. The response carries `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `Cross-Origin-Resource-Policy: same-origin`, `Correlation-Id`, `RateLimit` and `RateLimit-Policy` (19 SEC-HDR-1 and SEC-HDR-2; 06 "Basic rules", Caching; openapi `exportChannel`).
23. No message content, display names or query strings in logs, spans or metrics (19 SEC-LOG-3, SEC-LOG-4).

## Scope

All paths under `services/server/` unless noted. Follow the file and type naming that M1-56 and M1-57 established inside `internal/messaging`; the names below are the required public names. `ids.ID` is the ID type of `internal/kernel/ids`. This ticket creates no migration and no Makefile target.

Create:

- `internal/messaging/export.go` (port in the root package, 02 4.3 names it `Export`), each symbol with a doc comment:

    ```go
    // Export is the port of the channel export (02 "4.3 Ports, events and sources per module").
    type Export interface {
        // Channel validates the range, checks export.channel, records the audit entry and
        // returns the stream. On any error nothing has been sent to the client.
        Channel(ctx context.Context, q ChannelExportQuery) (ExportStream, error)
    }

    // ChannelExportQuery names the channel and the half-open range [From, To) on messages.created_at.
    type ChannelExportQuery struct {
        ChannelID ids.ID
        From, To  time.Time
    }

    // ExportStream yields the export in batches in export order. Next returns io.EOF after the
    // last batch and an error wrapping ErrExportAborted when a check during the stream fails.
    type ExportStream interface {
        Next(ctx context.Context) ([]MessageView, error)
    }

    // ErrExportAborted ends a stream whose re-check failed (Design note 7).
    var ErrExportAborted = errors.New("messaging: export aborted")
    ```

    `MessageView` is the DTO of M1-56 and M1-57; do not add a second one.

- `internal/messaging/domain/export.go`: `ValidateExportRange(from, to time.Time) error` (Design note 3c) and the keyset cursor `ExportCursor{ThreadID ids.ID; ThreadSeq int64}` with the pure step logic over thread pages and message pages (Design note 5).
- `internal/messaging/app/export.go`: the use case `ExportChannel` implementing `Export`, and the stream.
- `internal/messaging/adapters/postgres/queries/export.sql` (sqlc, regenerated `sqlcgen/`): `ListExportThreadIDs` and `ListExportThreadMessages` (Design note 5).
- `internal/messaging/adapters/httpapi/export.go`: the strict-server method `ExportChannel` and the streaming response type (Design note 8).
- The tests named in "Tests (write them first)".

Change:

- `api/operations.yaml`: move `exportChannel` from `later` to `enabled` (format of M1-17); regenerate with the generator that `make codegen-check` verifies.
- `internal/wire`: build the `Export` use case with the `Authorizer`, the audit recorder, the unit of work and the repository, hand it to the messaging HTTP handler; update the golden files that count routes (AZ-P1).
- Deadline class (Design note 9): the route metadata generator of `api/routes` (the binary-response flag of M3-05 on `routes.Operation`) and, if needed, `StageRoute` in `internal/platform/httpserver`, so that `exportChannel` runs under `http.transfer_timeout`; regenerate.
- `internal/platform/httpserver`, only if missing: the response writer of M1-18 exposes `Unwrap() http.ResponseWriter`, so that `http.ResponseController` can flush a streamed body (a test proves the flush reaches the client before the handler returns).
- `internal/kernel/authorize`, only if Design note 7a needs it: a documented function that starts a fresh rights-version memo for one batch while keeping the check log of M1-18.
- `internal/authz/app` tests: the decision table for `export.channel` (Tests). No change to the requirement table; the entry exists since M1-28.
- `internal/messaging/adapters/postgres/predicate_test.go` (M1-57): the two new statements carry the predicate and need no allowlist entry. If M1-70 has replaced this test with the repository-wide lint, that lint covers them.
- If the G8 response validator of M1-71 cannot validate an `application/x-ndjson` body line by line against its schema, extend it so it does (this also covers `exportAuditEvents`) and list the change in the report.
- The canary scenario of M1-69: add an export of the canary message (SEC-T10; the canary text appears in the export body for the authorized reader and nowhere in logs, spans, metrics or audit details).
- `docs/api/openapi.yaml`: extend the `description` of `exportChannel` with the rulings of Design notes 3c, 4, 5, 7 and 8 (English, one short paragraph); no schema, parameter or response change.
- `docs/spec/02-architecture.md` "4.6 Actions per module": correct the bullet on `export.channel` (Design note 12).
- `internal/messaging/README.md` (keep it in English and keep its section structure: status, port `Export`, operation, queries, stream and abort rules), `services/server/CHANGELOG.md` (entry under `Unreleased`), `docs/DECISIONS.md` section 10 rows `D-M3-11-1` to `D-M3-11-8` (Design note 13).

## Out of scope

- The export in the clients: the desktop channel menu is M4-33; the server side here is complete for every client.
- Privacy requests and their export packages (`createGdprRequest`, `downloadGdprExport`, `kernel/personaldata`, files with `purpose = 'gdpr_export'`): M7-06.
- The audit log export `exportAuditEvents`: M1-50.
- The answer `503 maintenance` for `export.channel` (effect `write`, 05 2.7): M7-08.
- Content that the shared representation does not emit yet: attachments (`Message.attachments`, M3-07), reactions and pins (M1-59), components (M2). File bytes never go into the export; files are downloaded through `downloadFile` (M3-05).
- Direct messages: the table `dm_participants` and DM channels come with V3-01; this ticket proves the DM rule of `export.channel` only at the `authz` level.
- Delegating the capability `export` through grants (`grant.create`, AZ-M38): M1-48.
- The repository-wide SQL predicate lint and the timing part of AZ-P4: M1-70; the Schemathesis run G7: M1-71; the canary scenario itself: M1-69 (extended here).
- A performance target: neither 02 "17. Performance targets" nor the M3 exit criteria name one for the export.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-11-channel-export`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(messaging): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- No new Go module dependency: lines are encoded with `encoding/json` and written with `net/http` of the standard library.
- No migration and no new index: the queries use the indexes 03 defines on `messages` (Design note 5).
- No real third-party service in tests; the export calls none.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. Start check. `make check` is green on the fresh worktree, else `Verdict: BLOCKED base branch red`. M1-57 and M3-05 are merged. The rate-limit stage of M1-19 (`internal/platform/ratelimit` with the class `rl.export` and the keys `ratelimit.rl.export.per_minute` and `ratelimit.rl.export.burst`) is on the base branch: M1-19 is in this ticket's dependency closure and the plan row requires `rl.export`; without it stop with `Verdict: BLOCKED dependency M1-19 missing`.

2. Operation.

    | operationId | Route | Action | Resource given to `authorize` | Class |
    |---|---|---|---|---|
    | `exportChannel` | `GET /api/v1/channels/{channel_id}/export` | `export.channel` | channel | `rl.export` |

    The export writes no event, changes no rights version and no business table (05: Permissions `no`). GET has no `Idempotency-Key`; every successful call is its own audited export, and after `401 step_up_required` the client repeats the same request (06 3.3).

3. Order of checks. Each step runs only if the previous one passed; nothing reaches the client before step g.
    a. Rate limit (chain, M1-19): `429 rate_limited` with `Retry-After`.
    b. Parameters (generated validation, M1-18): a missing `from` or `to` gives `400 validation_failed` with pointer `#query/from` or `#query/to` and code `required`; a value that is not an RFC 3339 time gives code `invalid_format`.
    c. Range rule, without any lookup, so it cannot leak: `from` must be strictly before `to`, else `400 validation_failed` with one `errors[]` entry, pointer `#query/to`, code `invalid`. Ruling for docs/DECISIONS.md: openapi states no rule; an empty or reversed range is a client error. There is no upper bound on the length of the range, because the spec names no key for one; `http.transfer_timeout` bounds one response, and clients split long ranges.
    d. `authorize(actor, export.channel, Resource{Kind: channel, ID: channel_id, ChannelID: channel_id})` in request mode. `authz` decides in the order of 07 4.2: an inactive principal, session, device or token gives `401`; no read right or an unknown channel gives `not_found`, and both take the same code path and return the same error value (02 9.2); read without `export` gives `forbidden` with reason `capability_missing` (agents and apps always, because their capabilities are capped at `upload`); no session of a human gives `forbidden` with reason `step_up_not_possible`; no step-up or one older than `auth.step_up_max_age` gives `step_up_required`. Denials are recorded by the `authz` layer and aggregated (R-AZ-04); the use case writes no denial.
    e. `Authorizer.Scope(actor, message.read)` gives the visible channel list for the SQL predicate (07 7.4 rule 1).
    f. The allow entry (Design note 6), committed.
    g. Response `200` with `Content-Type: application/x-ndjson`, then the lines (Design note 8).

4. Lines. One line per message: the REST representation `Message` produced by the single representation function of M1-56 that M1-57 uses for history, encoded with `encoding/json` and ended by exactly one `\n`. Ruling for docs/DECISIONS.md: the full `content_md` (never cut at `message.list_content_max_bytes`, `content_truncated` absent), because an export is the complete record; `my_reactions` absent, because it is the caller's state, not channel content, and optional in `Message`; tombstones included with the representation of M1-57 Design note 6, because they keep `thread_seq` and show no content (04 "Thread sequence"). `attachments`, `reactions`, `pinned` and `components` are whatever the shared representation emits on the base branch; when M3-07, M1-59 and M2 fill them, the export follows without a change here. `json.Marshal` escapes control characters, so no line contains a raw newline; a test proves it.

5. Selection and order. The export contains the messages with `channel_id` equal to the channel and `created_at >= from AND created_at < to`. Order: threads ascending by `thread_id`, the messages of a thread ascending by `thread_seq`. Ruling for docs/DECISIONS.md: openapi fixes only "`thread_seq` order per thread"; thread ids are UUIDv7 and therefore ordered by creation (03 "Principles" items 1 and 2: order never comes from timestamps), and the pair `(thread_id, thread_seq)` is a total, resumable keyset. Two statements in `queries/export.sql`, both with `m.channel_id = @channel_id AND m.channel_id = ANY(@visible_channel_ids::uuid[])` and the range:
    - `ListExportThreadIDs`: distinct `thread_id` greater than `@after_thread_id`, ascending, `LIMIT @limit`; served by the index `(channel_id, created_at DESC)`.
    - `ListExportThreadMessages`: the rows of one thread with `thread_seq > @after_seq`, ascending, `LIMIT @limit`; served by the unique index `(thread_id, thread_seq)`.

    `@limit` is `api.page_size_max` for both (as `exportAuditEvents` in M1-50). A batch is one page of `ListExportThreadMessages`. `ExportCursor` walks thread pages and message pages so that every message in the selection is emitted exactly once and in order, also when messages are added while the export runs.

6. Audit. The use case records exactly one allow entry through `auditing.Recorder.Record` inside its own `txn.UnitOfWork.Do` and commits it before the response starts: action `export.channel`, decision `allow`, resource kind `channel`, resource id the channel, `Attrs` `from` and `to` as `auditing.Time` (19 SEC-AUD-4 form), nothing else: no counts, names or content. Actor, session, device, IP pseudonym and `correlation_id` come from the context (M1-24). If the unit of work fails, the request ends with the problem of the error kind (`500 internal_error`, or `503 unavailable` with `Retry-After` when the database is unavailable) and no line is sent. Ruling for docs/DECISIONS.md: no export without its audit entry; the entry names the range and not the result size, because the size is known only at the end and the chain only appends.

7. Batches and rights during the stream. Before each batch:
    a. Start a fresh rights-version memo for the batch (derive the batch context with `authorize.WithRequestScope`, or with the function that the Scope list allows in `internal/kernel/authorize` if M1-28 keeps the memo in the check log of M1-18), so the batch uses the rights version current at that moment (07 8.2 item 2; 04 8.1: the rights at the time of delivery).
    b. `authorize(actor, channel.read, channel)` in filter mode (`authorize.Filter(ctx)`: no denial audit, 07 4.2). Any outcome but allow ends the stream with `ErrExportAborted`: revoked grant or membership, removed owner, disabled principal, revoked session or device.
    c. Read the page in a short read-only unit of work and build the representations; that unit of work ends before the first line of the batch is written (02 8.5; `database.idle_in_tx_timeout`). There is no snapshot across batches: each line shows the state when its batch was read.
    d. `authorize(actor, message.read, message)` per row in filter mode, with the descriptor M1-57 uses. A denied row is dropped, logged at error level with only the action and the resource kind, and counted in `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 7.4 rule 3, alert `BothyAuthzError`). Because b and d run on the same memo, a revocation never shows up as a filter mismatch.

    Ruling for docs/DECISIONS.md: the capability `export` and the step-up are checked once, when the export starts (05 2.4: the action is the start of the export); the stream then re-checks read access only, so a step-up that ages past `auth.step_up_max_age` does not cut a running export, but a lost read right does.

8. Streaming response. The strict-server method returns a response type of the adapter that implements the visitor method of the generated `ExportChannelResponseObject` interface (with oapi-codegen v2.8.0: `VisitExportChannelResponse(w http.ResponseWriter) error`). It sets only `Content-Type: application/x-ndjson`, writes `200`, and then per batch writes the lines and flushes through `http.ResponseController`. `Correlation-Id`, `RateLimit`, `RateLimit-Policy`, `Cache-Control: no-store`, `Cross-Origin-Resource-Policy: same-origin` and `X-Content-Type-Options: nosniff` come from the chain (M1-18, M1-19) and are not overwritten. The proof check of M1-18 passes because step 3d recorded `export.channel` before the first write. After the first byte no problem document can follow: on `ErrExportAborted`, a read error, the deadline or a shutdown (02 15.3), the method logs only the cause code and the route pattern and calls `panic(http.ErrAbortHandler)`; the recover stage of M1-18 passes it on (`TestRecover_ErrAbortHandlerRepanics`), so `net/http` ends the response without its final chunk (HTTP/1.1) or resets the stream (HTTP/2). A client therefore never takes a cut export for a complete one. A write error (client gone) ends the stream quietly. A range without messages gives `200` with an empty body. Ruling for docs/DECISIONS.md. If M3-05 put a streaming helper into `internal/platform/httpserver`, reuse it; `messaging` never imports `internal/files` adapters (02 4.4).

9. Deadline. 02 14.2 gives uploads and downloads `http.transfer_timeout`. M1-18 applies it to request bodies of kind multipart and octet-stream; M3-05 adds a flag on `routes.Operation` for operations whose `2xx` body is `format: binary`, and `StageRoute` applies `http.transfer_timeout` to them (`TestOperations_BinaryResponseFlag`, `TestStageRoute_BinaryResponseUsesTransferTimeout`). The `200` of `exportChannel` is `application/x-ndjson` with schema `Message`, which that rule does not catch. Extend the generator so the same flag (keep its name; rename nothing) is also set for operations whose `2xx` response media type is `application/x-ndjson`, and update its doc comment to "streamed response". This also gives `exportAuditEvents` the transfer deadline, which is right for the same reason; list it in the report. Without this a long export would end after `http.request_timeout`. Ruling for docs/DECISIONS.md.

10. Rate limit. `rl.export` counts per principal (05 3) and is shared with `createGdprRequest` and `downloadGdprExport`; denied requests count too, because the limiter runs before `authorize` (06 9).

11. What already holds and is only proven here, not built: archived channels stay exportable for readers with `export` (07 4.5 caps the level at `read`, capabilities stay); a read-only channel too (05 rule 1.8 blocks only `write` and `trigger`); "view as" gives `forbidden` with reason `view_as_read_only` because `export.channel` writes for M1-28 (class `rl.export`); the maintenance layer of M7-08 will answer `503 maintenance` because the effect is `write`.

12. Spec correction. 02 "4.6 Actions per module" says "`export.channel` belongs to `messaging`; `files` stores the package." openapi `exportChannel` streams the export (06 "Basic rules": "NDJSON lets large exports stream"), and 03 `files.purpose` has no value for a channel export, so nothing is stored. Change the bullet (English) to say that `export.channel` belongs to `messaging` and that the export streams NDJSON without storing a package; export packages in `files` belong only to the privacy export of `privacy` (M7-06). Chapter 02 is not part of the core contract. Ruling for docs/DECISIONS.md.

13. Rulings for `docs/DECISIONS.md` section 10, one row each with reason and place: `D-M3-11-1` range rule (note 3c); `D-M3-11-2` line content (note 4); `D-M3-11-3` order (note 5); `D-M3-11-4` audit before the first byte with `from` and `to` (note 6); `D-M3-11-5` batches without snapshot and the read re-check per batch, export capability and step-up only at the start (note 7); `D-M3-11-6` visible abort after the first byte and the empty body (note 8); `D-M3-11-7` transfer deadline (note 9); `D-M3-11-8` stream without stored package and the correction of 02 (note 12).

14. Threats. The red team of this ticket and of milestone M3 (PLAN 9 "Red team") read this diff for T-AZ-02 (a line of an invisible channel, existence through status, body or timing), T-AZ-03 (a line after a revocation) and T-AZ-04 (an export without audit entry). Each has a test below.

## Tests (write them first)

Unit:

- `internal/messaging/domain`: `TestValidateExportRange`: `from` before `to`; equal; reversed; a difference of one millisecond.
- `internal/messaging/domain`: `TestExportCursor`, table over fake pages: several threads, a page border inside a thread and at the end of a thread, a thread page that ends exactly at the limit, an empty selection, resuming after a cursor; every `(thread_id, thread_seq)` appears exactly once and ascending.
- `internal/messaging/adapters/httpapi`: `TestExportLines_OneMessagePerLine`: messages whose content holds a newline, U+2028, U+2029, quotes and backslashes, plus a tombstone: the line count equals the message count, every line decodes into `Message` and validates against the schema, no line contains a raw newline, no `null` token appears (decoder with `UseNumber`), `content_truncated` and `my_reactions` are absent.
- `internal/messaging/adapters/httpapi`: `FuzzExportLineEncoding` (the seed corpus runs in `go test`): any `content_md` gives exactly one line that decodes back to the same text.
- `internal/messaging/app` with fakes: `TestExportStream_FilterMismatchDropsLine`: an `Authorizer` fake that denies `message.read` for one row: the row is missing from the batch, the log record holds only action and resource kind, `bothy.authz.errors` with `bothy.reason = filter_mismatch` rises by one.
- `internal/authz/app` with the source fakes of `internal/authz/adapters/fake` (M1-28): `TestExportChannelDecisions`, one row each: Alex with `read` and `export` on Support and a fresh step-up: `allow`; Alex with `trigger` and `upload` but without `export`: `forbidden` `capability_missing`; Alex with `read` and `export` without step-up: `step_up_required`; step-up exactly `auth.step_up_max_age` old: `allow`, one nanosecond older: `step_up_required` (AZ-M36 pattern); Sam on a Support channel without a grant: `not_found`; an agent with a grant `read` on the channel: `forbidden` `capability_missing`; an installed app with a grant on the channel: `forbidden` `capability_missing`; a personal API token of a person with `read` and `export`: `forbidden` `step_up_not_possible`; the owner in a session with a fresh step-up: `allow`; the owner through a personal API token without own grants on the channel: `not_found` (R-AZ-14); a DM channel from the fake structure source: participant `forbidden` `capability_missing`, owner who is no participant `not_found` (R-AZ-02); archived channel with `read` and `export`: `allow`; read-only channel: `allow`; owner with "view as" Alex: `forbidden` `view_as_read_only`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain on `127.0.0.1:0` with real authentication and step-up as in M1-35, M1-56 and M1-57, grants through the `authz` test helpers, a steerable clock; the world of 07 "Examples": space Support with channels Allgemein and Eskalation, space Marketing with channel Kampagnen; in Support/Allgemein the main thread and two further threads with messages before, inside and after the range, one message longer than `message.list_content_max_bytes` (set small in the test configuration) and one tombstone made with the test helper of M1-57; test data only with `example.com` addresses):

- `TestExportChannel_StreamsVisibleHistory`: Alex with `read` and `export` on Support and a fresh step-up: `200`, `Content-Type: application/x-ndjson`; exactly the messages of Support/Allgemein with `created_at` in `[from, to)`, threads ascending by id, `thread_seq` ascending within each thread; the long message in full without `content_truncated`; the tombstone with empty `content_md` at its `thread_seq`; every line validates against `Message`; no JSON `null`.
- `TestExportChannel_OnlyThisChannelAndRange`: messages of Support/Eskalation and Marketing/Kampagnen in the same range and messages of Support/Allgemein outside the range never appear; `from` inclusive and `to` exclusive at millisecond precision.
- `TestExportChannel_EmptyRange`: a range without messages gives `200` with an empty body and one audit allow entry.
- `TestExportChannel_WithoutExportIs403` (plan row "without `export` `403`"): Alex with `trigger` and `upload` but without `export` gets `403 forbidden` with `reason` `capability_missing` and no body line; no allow entry.
- `TestExportChannel_InvisibleEqualsUnknown` (AZ-P4, with `ratelimit.rl.export.burst` raised in the test configuration): Alex on Marketing/Kampagnen and on a random UUIDv7 gets `404 not_found` twice with equal status, body without `correlation_id` and `instance`, and headers without `Correlation-Id` and `RateLimit`. Add `exportChannel` to the AZ-P4 table of M1-70.
- `TestExportChannel_StepUp` (ID-T18 for this action): without step-up `401 step_up_required`; with a step-up older than `auth.step_up_max_age` (steerable clock) `401 step_up_required`; after a fresh step-up the same request gives `200`. The case "step-up with a wrong key" belongs to M1-35.
- `TestExportChannel_AgentAndTokens`: the token of an agent with a grant `read` on Support (agent record of M1-51, token through `token.create` of M1-44, grant through the `authz` test helpers) and a personal API token of Alex both get `403 forbidden` (reasons `capability_missing` and `step_up_not_possible`); no line, no allow entry.
- `TestExportChannel_OwnerExportsAnyChannel`: the owner in a session with a fresh step-up exports Marketing/Kampagnen without a grant (05 2.3 item 1).
- `TestExportChannel_ArchivedChannel`: after `archiveChannel` on Support/Eskalation, Alex with `read` and `export` still exports it (07 4.5).
- `TestExportChannel_Audit`: one successful export writes exactly one entry `export.channel` with decision `allow`, `resource_kind = channel`, the channel id, actor, session and device of Alex, `details` exactly `from` and `to` in the form of 19 SEC-AUD-4, and `correlation_id` equal to the response header `Correlation-Id`; the entry is readable after only the response headers have arrived (committed before the status line).
- `TestExportChannel_DenialsAggregated` (AZ-P13): two `403` attempts of Alex inside `audit.deny_aggregate_window` give, after the recorder flush of M1-24, exactly one deny entry `export.channel` with `deny_reason = capability_missing` and an aggregate count of two.
- `TestExportChannel_AuditFailureSendsNothing`: with a recorder that fails, the request ends with `500 internal_error` as `application/problem+json`, and no NDJSON line is sent.
- `TestExportChannel_RateLimit` (plan row "`rl.export`"): with `ratelimit.rl.export.per_minute` and `ratelimit.rl.export.burst` from the test configuration, one request more than the burst in quick succession gives `429 rate_limited` with `Retry-After` and the `RateLimit` headers of class `rl.export`; an export by Sam right after still passes (the bucket is per principal).
- `TestExportChannel_RangeValidation`: a missing `from` or `to` gives `400 validation_failed` with pointer `#query/from` or `#query/to` and code `required`; a malformed time gives code `invalid_format`; `from = to` and `from > to` give `400 validation_failed` with pointer `#query/to` and code `invalid`; the same request on an invisible channel gives the same `400` (no lookup before the range rule).
- `TestExportChannel_BatchBorders`: with `api.page_size_max` small in the test configuration and more messages than three batches spread over several threads: every message exactly once, the order of note 5 across batch and thread borders.
- `TestExportStream_RevocationBetweenBatches` (AZ-P9 for a stream, through the `Export` port against PostgreSQL): after the first `Next`, revoke Alex's grant through the `authz` test helper (a committed revocation that raises the rights version); the next `Next` returns an error wrapping `ErrExportAborted` and no message.
- `TestExportStream_SessionRevokedBetweenBatches`: as above with `RevokeSession` of M1-34 instead of the grant.
- `TestExportChannel_AbortIsVisible`: a repository wrapper that fails on the second batch: the HTTP/1.1 client reading the body gets `io.ErrUnexpectedEOF`, and the received bytes are complete lines of the first batch with no problem document after them.
- `TestExportStream_NoTransactionBetweenBatches`: between two `Next` calls `pg_stat_activity` shows no connection of the test pool in state `idle in transaction`, and a pause longer than `database.idle_in_tx_timeout` (set small in the test configuration) does not break the next `Next`.
- `TestExportChannel_TransferDeadline`: the handler context of `exportChannel` has a deadline derived from `http.transfer_timeout`, not from `http.request_timeout` (both set to different small values in the test configuration); `TestOperations_BinaryResponseFlag` of M3-05 now expects the flag also on `exportChannel` and `exportAuditEvents`.
- `TestExportChannel_Headers` (SEC-T03 for this response kind): `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `Cross-Origin-Resource-Policy: same-origin`, `Content-Type: application/x-ndjson`, `Correlation-Id`, `RateLimit`, `RateLimit-Policy`; a flushed first batch arrives before the handler returns.
- `TestExportChannel_NoContentInLogs` (SEC-LOG-3, SEC-LOG-4): with a captured logger, no `content_md`, display name or query string appears in any log record of the integration tests above, and the audit `details` hold only `from` and `to`.
- `TestQueriesCarryVisibleChannelPredicate` (M1-57, AZ-P12): passes with `ListExportThreadIDs` and `ListExportThreadMessages` and without a new allowlist entry.
- `TestAZP1_RoutesMatchEnableList` (M1-17, AZ-P1): counts `exportChannel` with action `export.channel` and class `rl.export`.

Contract: every response of the integration tests above passes the G8 response validator of M1-71 (06 G8); the Schemathesis run G7 of M1-71 covers `exportChannel` inside `make check`.

Run `FuzzExportLineEncoding` once locally with `go test -run=^$ -fuzz=FuzzExportLineEncoding -fuzztime=<duration of your choice>` and report duration and result; commit any crashing input as a seed after the fix.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/messaging/... ./internal/authz/... ./internal/kernel/... ./internal/platform/httpserver/... ./internal/wire/... ./api/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make register-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `exportChannel` is enabled and registered with `export.channel` and `rl.export`; AZ-P1 passes.
- [ ] Without the capability `export` the answer is `403 forbidden` with `reason` `capability_missing`; without read right it is `404 not_found`, equal to an unknown channel (AZ-P4); without a fresh step-up `401 step_up_required`; agent, app and personal tokens are refused (ID-T18).
- [ ] The export contains only visible content: only messages of this channel in `[from, to)`, every query with the visible-channel predicate (AZ-P12), every line checked with `message.read`, tombstones without content, no line after a committed revocation (AZ-P9 for a stream).
- [ ] Lines are complete `Message` objects, one per line, threads ascending by id and `thread_seq` ascending, full content, no JSON `null`.
- [ ] Every successful export writes exactly one audit allow entry with `from` and `to` before the first byte; an audit failure sends nothing; denials are audited and aggregated (AZ-P13).
- [ ] `rl.export` throttles per principal with `429 rate_limited` and `Retry-After`.
- [ ] An error after the first byte aborts the response visibly; no transaction is open while lines are written; the route runs under `http.transfer_timeout`.
- [ ] The `exportChannel` description in `docs/api/openapi.yaml`, the bullet in 02 "4.6 Actions per module", the `messaging` README, the CHANGELOG and the rows `D-M3-11-1` to `D-M3-11-8` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `messaging` updated (port `Export`, operation, queries, the stream, re-check and abort rules of notes 7 and 8); READMEs of `platform/httpserver` and `kernel/authorize` only where this ticket changed them (deadline class, flush, fresh memo).
- Rulings of note 13 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, spec gaps found, and every difference between this brief and the merged code (Boundaries).

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
