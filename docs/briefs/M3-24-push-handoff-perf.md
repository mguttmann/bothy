# M3-24: Push handoff performance target

Status: ready | Depends on: M1-72, M3-20 | Milestone: M3

## Goal

After this ticket `make perf` measures the performance target `perf.push_handoff_p95` on a recorded host profile, writes value and threshold and fails when the value exceeds the threshold: the time from the commit of a message that mentions a person to the moment Fake-APNs accepts the push for that person's device with `200` (13 section 9, t0 to t1; 02 17 "commit to handoff to the push service (fake)"; test `perf/push_handoff`). The measured interval holds the whole server path of a hint: the worker consumer `push` reading the log (ADR 0051), the candidates and facts, the pure decision, the job `notify.push_send` on queue `delivery`, the rights check at send time, the badge, the APNs adapter with its JWT and payload and the HTTP/2 request through the egress client. The test runs under the sustained design load `perf.sustained_messages_per_s` with the default rate limit of the measured class on a durable database, and it counts only when the path behaved correctly while it was measured: every mention in the window yields exactly one push per registration, with the right IDs, a badge and no text. This closes the push part of M3 exit criterion 3, the confirmation "Load test `perf/push_handoff`" of ADR 0051 and the delay that ADR 0051 accepts as a consequence; the target stops being "unchecked" (02 17).

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket" (item 6: `make perf` in every ticket that touches a performance target, no lowered threshold); section 4 "Closing a milestone" item 3; section 5 "Test strategy" rows "Test doubles" (Fake-APNs) and "Load, endurance, chaos"; section 9 "M3 Platform services" (Goal, Exit criteria 3 and 4, Test strategy "fake APNs (HTTP/2 with TLS)", Red team "content in push payloads") with the rows M3-17 to M3-19 (what the measured path is made of), M3-20 (a dependency), M3-22 and M3-23 (beside it, not in its closure) and M3-24 (this ticket, the contract); section 7 rows M1-01, M1-16, M1-71 and M1-72 (a dependency); section 8 row M2-36 (the sibling load test); section 19 P-07, P-09, P-10, P-11. No ticket of the plan depends on M3-24; its result is the push part of M3 exit criterion 3.
- Dependencies: the header names M1-72 (the perf frame: profile modes, `.cache/perf/results.json`, the clock skew helper, `Options.DurableDB`; D-W-108 point 4) and M3-20. Implied through them and therefore not in the header: M3-19 (consumer `push`, `notify.push_send`, `push_registrations`, the provider port, the badge of 13 8), M3-18 (`putNotificationPreference`), M3-17 (the decision), M2-01 (consumer frame), M2-09 and the M2 run chain below it, M1-38, M1-71 (`test/harness` with `Start`, `Options`, `NewUser`, `AdminDB`), M1-69, M1-68, M1-65, M1-56 (`createMessage`, mentions with M1-60), M1-53 (`createChannel`, `main_thread_id`), M1-48 (`createGrant`), M1-37 (`createUser` with `local_account`, `redeemEnrollmentCode`), M1-33 (`kernel/jose`), M1-22 (`platform/egress`, `Options.HTTP2` of M3-20), M1-16 (`perf/event_commit`, the first `perf/internal/report`, `pgtest.Durable()`), M1-14, M1-12 (`pgtest`), M1-09 (secret provider, `file` driver), M1-07 and M1-08 (configuration), M1-01 (the `perf` frame target and `PERF_TIMEOUT`). M3-22 (`registerPush`) is not in the closure either; Design note 5 writes the registrations as fixture rows.
- `docs/spec/02-architecture.md`: 6 "Code layout" (`perf/`, component `testsupport`), 11.1 "Writing", 11.5 "Worker consumers (push, webhooks)", 12.2 "Queues" (`delivery`), 12.3 row `notify.push_send`, 12.4 "Rules for jobs", 14.2 "Deadlines" (row "outbound calls"), 17 "Performance targets" (row `perf.push_handoff_p95`), 22 "Configuration values of this chapter" (all `perf.*` rows, `consumer.batch_size`, `consumer.poll_interval`, `jobs.queue.delivery.max_workers`, and the sentence that `perf.*` are test thresholds read by `make perf`).
- `docs/spec/13-push-notifications.md`: 1 "Goals" (points 1 and 5); 2 "Kinds of notifications and recipients" (kind `mention`); 3.1 "Effective level"; 4 "Decision per recipient" (4.1 to 4.6, in particular 4.5 point 3: `mention` goes at once, and 4.6: a device without a live connection gets a push to each active registration); 5.1 "iOS and macOS: APNs with Notification Service Extension"; 6.1 "APNs" (the keys of the payload, `badge`, `b.m`, `b.t`, `b.k`, `b.nid`); 7.3 "Common"; 8 "Badge and sync"; 9 "Delivery measurement" (t0, t1); 12 "Execution in the server" (all five points); 14 "Tests"; 17 "Configuration values of this chapter".
- `docs/spec/03-data-model.md` (core contract, read only): `event_log` (`occurred_at` "time of the transaction", `type`, `resource_kind`, `resource_id`, the index `(resource_kind, resource_id)`), `event_consumer_cursors` (`name`, `last_seq`), `push_registrations` (all columns and checks), `auth_sessions` (`device_id`), `devices` (`state`).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 3.2 "Pattern with a counter row" (commit order is log order), 7.6 "Consumers in the worker (push, webhooks)".
- `docs/spec/05-actions.md` (core contract, read only): 3 "Rate-limit classes" (`rl.message` per principal), 4.7 row `message.create`, 4.9 rows `push.deliver` and `notification_pref.update`, 5 paragraph "Check without a request", R-AZ-29, R-AZ-30.
- `docs/spec/23-quality.md`: 2 (`make perf` at the end of a milestone and in the release) and 3 (coverage leaves out `perf/**`).
- ADRs 0063 "Performance gate for the event log" (points 2 to 5, Confirmation), 0051 "Worker consumers of the event log with a cursor instead of job inserts in the write path" (Consequences: the delay until the next round; Confirmation: "Load test `perf/push_handoff`"), 0031 (push without content), 0009.
- `docs/api/openapi.yaml`: operations `createSpace`, `createChannel`, `createGrant`, `createUser`, `redeemEnrollmentCode`, `createStepUpChallenge`, `completeStepUpWithDeviceKey`, `putNotificationPreference`, `createMessage`; schemas `MessageCreate` (`content_md`, `mention_ids`, `client_msg_id`), `Message`, `Channel` (`main_thread_id`), `NotificationKind`, `NotificationLevel`, `Problem` (`x-problem-codes`).
- `docs/operations/configuration.md`: the keys of Design note 12.
- `docs/observability/signals.md` 4.4 (`bothy.push.sent`, `bothy.push.send.duration`, `bothy.push.skipped`) and 5 span `push.send`: production signals of the same path; this ticket adds none and scrapes none. `docs/observability/slos.md` `bothy-push/provider-acceptance` (the acceptance, not the delay).
- `docs/security/threat-model.md` 5.12 "Push" and T-PU-01 (no content at the provider; the load test checks it on every request). `docs/RISKS.md` RK-37 (presence per api node; this test runs one node in role `all`). `docs/DECISIONS.md`: D-W-100, D-W-106 and D-W-108 (the perf frame edges of M2-36 and M3-16; the push block); section 10 for the rows of this ticket. No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- Briefs (merged names win, D-W-100): `docs/briefs/M1-72-performance-gate.md` (the perf frame: `perf/internal/report`, `.cache/perf/results.json`, profile modes, durability check, the clock skew correction of `gateway_fanout`, `Options.DurableDB`, the open-loop scheduler of `load_messages`), `M1-71-contract-tests.md` (`test/harness`: `Start`, `Options`, `Server`, `NewUser`, `AdminDB`, `Client.Do`), `M1-16-event-log.md` (origin of the report, `pgtest.Durable()`), `M1-01-server-gates.md` (`perf` target, `PERF_TIMEOUT`), `M2-36-agent-path-perf.md` (the sibling pattern: named constants, `-p 1`, the move of the scheduler to `perf/internal/load`, failure conditions besides the threshold). Briefs of M3-19 and M3-20 as merged (D-W-100; their names below are the expected ones): M3-19 Design notes 11 to 13 (`notify.push_send` with `DueAt` = t0 of 13 9, `PushProvider`, `Registration`, `Notification` with `Badge`, `HasBadge` and `KindSync`, the result classes, `push.max_concurrent_requests`) and Design note 16 (the badge of 13 8 through `BadgeCounter.For`, D-W-108 (6)); M3-20 Design notes 1, 2, 5 and 8 (`internal/notify/adapters/apns`, `Deps.Endpoints` for tests only, `egress.Options.HTTP2`, the payload table, Fake-APNs `apnstest` with `Start`, `Options`, `Request`, `Requests`, `EgressRule`, `RootCAs`, `URL`, `Close`).
- Code on the base branch (all under `services/server/` unless it names the root `Makefile`): the root `Makefile` (target `perf`, variable `PERF_TIMEOUT`); `perf/` (`event_commit`, `load_messages`, `internal/report`, `internal/load` if moved, `README.md`); `test/harness`; `internal/platform/pg/pgtest`; `internal/platform/config` (defaults); `internal/wire` (the provider set, `push_apns.go`, the override struct through which the harness passes `Clock`, `Telemetry` and `EgressDenyAll`); `internal/notify` (`adapters/apns`, `adapters/apns/apnstest`, `app/send.go`, `adapters/jobs`); `internal/platform/egress`.

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

Note on rule 9: the parameters of the load (counts, shares, durations of Design note 4) are named constants of a test, not limits of the product; their values stand in `perf/internal/pushhandoff/constants.go` and `perf/README.md`, and every product limit they depend on is read by its key name.

Product requirements implemented here:

11. `make perf` measures `perf.push_handoff_p95` ("commit to handoff to the push service (fake)", test `perf/push_handoff`) against Fake-APNs and fails when it exceeds its threshold (PLAN row M3-24; 02 17; M3 exit criterion 3; ADR 0051 Confirmation).
12. The measuring points are those of 13 9: t0 is the commit of the triggering event (for a hint that goes at once), t1 is the moment the provider accepted with `200`.
13. The threshold comes only by name from the table of 02 22 (meaning `target`: the measured value must not exceed it); `perf.*` values are test thresholds, not runtime configuration, and never appear as literals in code or docs (02 22, ADR 0063, the loader of M1-16 and M1-72).
14. Measured only on databases with `fsync` and `synchronous_commit` on; every result names the profile mode of `perf.reference_profile`; only `match` and `emulated` can end green, `below` never does (ADR 0063 points 2 and 3).
15. `make perf` runs at the milestone close, in the release and in every ticket that touches a target, never in `make check` or the pre-push gate (P-07; PLAN 3 item 6; ADR 0063 point 4).
16. No content reaches the provider: every push carries only IDs, the kind, flags and numbers (13 1 point 1, 6.1, ADR 0031, T-PU-01); the load test checks every request it measures.
17. Every sender of the load stays within `rl.message` under its default limit (05 3); no request of the measurement is answered `429 rate_limited`.
18. No threshold is lowered and no default of a key named in this brief is changed to pass the target (PLAN 3 item 6; ADR 0063 point 5).

## Scope

All paths under `services/server/` unless they start with `docs/` or name the root `Makefile`. Names of M1-72's perf frame, of the harness and of M3-19 and M3-20 below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `perf/push_handoff/` (build tag `perf`, the one tag of M1-16 and M1-72): `push_handoff_test.go` with `TestPerf_PushHandoffP95` (Design notes 6 to 9) and a package comment naming the target. The directory name is that of 02 17 and the plan row; the package clause follows the convention M1-72 chose for `load_messages`.
- `perf/internal/pushhandoff/` (untagged, unit tested in `make test` without a container): `constants.go` (the named test parameters of Design note 4), `plan.go` (`Plan` derived from configuration defaults, the 02 table and the constants), `requests.go` (strict decoding of a Fake-APNs request into a hint record, Design note 8 a), `samples.go` (samples, window filter, skew correction through the frame helper), `checks.go` (the matcher and the failure conditions of Design note 9), `doc.go`.
- `perf/internal/pushworld/` (untagged): `Seed` builds the world of Design note 5 through the public API and writes the registrations as fixture rows; `seed_test.go` holds the small-world integration test of "Tests" (container runtime like every `pgtest` test, runs in `make test`).
- `perf/internal/load/` only if M1-72 kept its open-loop scheduler inside `perf/load_messages` and no other ticket moved it yet: move it here without a change of behaviour so that `load_messages` and `perf/push_handoff` use one scheduler; M1-72's tests keep their names and stay green. Never write a second scheduler.

Change:

- `internal/notify/adapters/apns/apnstest` (M3-20, additive): the field `Request.AcceptedAt` (Design note 7 b) with its test.
- `test/harness` (M1-71, M1-72, additive): `Options.APNsFake` (Design note 3) with its test and README line.
- `internal/wire` (additive), only if M3-20 left no Go-level path by which a test points the wired APNs adapter at Fake-APNs: the two test overrides of Design note 3 with `TestProductionWiringHasNoTestOverrides`.
- `perf/internal/report` (M1-16, M1-72): no new mechanism. If the threshold loader or its tests hold a list of expected names, add `perf.push_handoff_p95`; extend `TestThresholdsFromChapterTable` (or its merged equivalent); add the exit-logic test of "Tests".
- `perf/README.md`: a section `push_handoff` (what is measured, the measuring points, world and load shape, parameters with their values, failure conditions, how to read `extra`, that the server runs in the test process).
- Root `Makefile`, target `perf`: add `-p 1` to its `go test` call if it does not serialize the packages yet (Design note 10). No new target.
- `docs/spec/02-architecture.md`: 17 row `perf.push_handoff_p95` and 22 row `perf.sustained_messages_per_s` (Design note 11).
- `docs/spec/13-push-notifications.md`: 9 a paragraph on the load test, 14 a row (Design note 11).
- `docs/operations/configuration.md`: regenerate with `make config-reference` (the meaning of `perf.sustained_messages_per_s` changes).
- `docs/adr/0063-performance-gate-for-the-event-log.md`: "Confirmation" gains a bullet for M3-24, "More Information" names the push target; the decision stays unchanged; then `make adr-index`.
- `docs/DECISIONS.md` section 10: rows `D-M3-24-1` to `D-M3-24-6` (Design note 13).
- READMEs: `internal/notify/README.md` (one English line in the Fake-APNs section on `AcceptedAt`; keep the line `May use the ports of these modules: ...` and the section `## Tables` exactly as they are, because M1-06 parses them), `test/harness/README.md`, `internal/wire/README.md` if the overrides were added; `services/server/CHANGELOG.md` under "Unreleased".

If the base branch already carries a sentence of Design note 11 (a planning change made after this brief), keep it and change nothing there.

This ticket creates no migration, no operation, no event, no metric, no job kind, no configuration key and no new top-level make target.

## Out of scope

- The push path itself: the consumer `push`, the decision, bundling, the job `notify.push_send`, the badge (M3-17 to M3-19) and the APNs adapter (M3-20). Only a fix of Design note 14 may touch them.
- FCM and Fake-FCM (M3-21, not in the closure): the plan row measures against Fake-APNs, and after the decision both providers share the job; a second provider would measure the same server path twice.
- Hints over the gateway (`notification.created`, t1 = "the gateway has written the frame" of 13 9) and the delayed push while a desktop is active (M3-23); bundled hints, whose t0 is the window end (13 4.5, 9).
- `registerPush`, the preview endpoint and t2 of 13 9 (M3-22; device side M5-04).
- A production signal for t1 minus t0 or an SLO on it (13 9 "Push sending"; signals and SLOs are Team F's files, M6-13 measures own statistics); this test reads no metrics endpoint.
- The thresholds and `perf.reference_profile` themselves; the other targets of `make perf` (M1-16, M1-72, M2-36, M3-16).
- `cluster.mode = multi` and separate `api` and `worker` processes (M7-17, RK-37).
- The milestone-close run on the reference host itself (owner operation after the merge, PLAN 4 item 3).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-24-push-handoff-perf`; one writer per worktree.
- Conventional Commits with scope (for example `test(perf): ...`, `test(harness): ...`, `docs(spec): ...`, `perf(<module>): ...` for a fix of Design note 14) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief and the fixes of Design note 14, each with a test and listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0: the measured server runs in the test process through `harness.Start` with loopback listeners, and Fake-APNs is an in-process TLS server on loopback. PostgreSQL runs only as a throwaway container through `pgtest` with `pgtest.Durable()` (P-09, P-10). Without a Docker-compatible runtime the tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end.
- Never read or print secrets. Device tokens, the provider JWT, the test key, API tokens and DPoP proofs never appear in logs, test output, `results.json` or failure messages (name registrations by their ID). No network calls except package and image downloads for pinned tools and Go modules.
- No real Apple service, no Apple account and no real key: the key is generated per run with `crypto/ecdsa` and written with mode `0600` into a `t.TempDir()` for the `file` driver; team and key IDs contain `EXAMPLE`; the topic is `com.example.chat`. Nothing in this ticket costs money.
- Never lower a threshold, disable a gate or delete a test. A missed target on a host in mode `below` is reported as not proven, never "fixed" by changing numbers.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). This ticket touches no core table, event or action.
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-17 to M3-20 and of the perf frame: follow their names for `PushProvider`, `Notification`, `notify.push_send`, `apnstest`, the report, the harness and the scheduler, and list each difference in the report.
- This ticket uses the perf frame (profile modes, results file, durability check, clock skew helper, `Options.DurableDB` of the harness) of M1-72 and never builds a second one.
- Test overrides in production packages (Design note 3) are Go fields that no configuration key, flag or environment variable reaches; `cmd/bothy` never sets them.
- Direct SQL writes only in `perf/internal/pushworld` (the fixture registrations of Design note 5), only through the admin pool of the throwaway container and before the measurement starts; every other SQL of this ticket is read-only.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red` and the failing gate. Confirm that the base has M3-20 (`internal/notify/adapters/apns` with `apnstest`, `egress.Options.HTTP2`) and through it M3-19 (`notify.push_send`, the consumer `push`, `push_registrations`), else `Verdict: BLOCKED dependency M3-20 missing`, and M1-72 (the perf frame of Boundaries), else `Verdict: BLOCKED dependency M1-72 missing`. Run `make perf` once only at the end (Definition of done); if a target of another ticket fails in that run, run that package on the base commit and report both values, so that a host limit is not mistaken for a regression of this ticket.

2. **Packages.** `perf/push_handoff` holds only the tagged test and its orchestration. Everything that can be tested without a server lives untagged in `perf/internal/pushhandoff` (plan, request decoding, samples, matcher), so `make test` runs its unit tests and `make check` compiles, vets and lints it; `perf/internal/pushworld` holds the seeding and its small-world test. `perf/**` is the architecture component `testsupport` (02 6; M1-05) and may import `test/harness`, `internal/notify/adapters/apns/apnstest` and the report; nothing outside `testsupport` imports these packages. Coverage leaves `perf/**` out (23 3).

3. **The measured server runs in the test process** (Ruling for docs/DECISIONS.md, `D-M3-24-1`). The wired APNs adapter reaches Fake-APNs only through `Deps.Endpoints` and the egress root CAs, which are Go values that `internal/wire` never sets and that no configuration key reaches (M3-20 Design note 1 e); a server subprocess (`harness.StartProcess`) could therefore only talk to the real Apple hosts. So `TestPerf_PushHandoffP95` starts the server with `harness.Start` (the real `internal/wire` graph in role `all`, M1-71) and `Options.DurableDB`:

    a. `harness.Options.APNsFake *apnstest.Server` (additive): when set, `Start` points the wired APNs adapter at the Fake for both environments, adds the Fake's root CAs to the egress dependencies and appends `APNsFake.EgressRule()` to `egress.allowed_hosts`. The test still sets the `push.apns.*` keys of Design note 12 through `Options.Config`, so the adapter is enabled exactly as in production.

    b. If M3-20 left a Go-level path for this (for example the one its `TestPushSendThroughAPNs` uses), `Start` uses it. Otherwise add two fields to the merged override struct through which the harness passes `Clock`, `Telemetry` and `EgressDenyAll` into `internal/wire`: `APNsEndpoints map[string]string` (environment to base URL, passed to `apns.Deps.Endpoints`) and `EgressRootCAs *x509.CertPool` (passed to `egress.Deps.RootCAs`). `TestProductionWiringHasNoTestOverrides` builds the graph the way `cmd/bothy` does, with APNs enabled, and asserts that the adapter targets the two Apple base URLs and that the egress root CAs are the system pool.

    c. Profile: in mode `emulated` the test sets `runtime.GOMAXPROCS` to the reference vCPU count before `Start` and the PostgreSQL container gets the profile limits through the pgtest options of M1-72, as M1-72's mode decides; in mode `match` nothing is capped. The load generator and Fake-APNs share the CPUs with the server, so the value is an upper bound of the server's own time; `results.json` says so in `extra`.

4. **Load shape and parameters** (Ruling for docs/DECISIONS.md, `D-M3-24-3`; 02 17 names no load for this target). The test runs under `perf.sustained_messages_per_s` human messages, the sustained design load of 02 22, because the consumer `push` reads every event of the log and the job shares the process, the pool and `event_seq` with chat. Hints are of kind `mention` only: 13 4.5 point 3 sends a mention at once, so its t0 is the commit (13 9); a `message` hint can be collected in a window whose t0 is the window end, and after the decision every kind takes the same job and adapter. Named constants in `perf/internal/pushhandoff/constants.go`, documented with their values in `perf/README.md` and copied into `results.json`:

    - `BudgetShare = 0.5`: every sender plans at most half of `rl.message`; the other half absorbs jitter.
    - `SendersPerChannel = 10`, `RecipientsPerChannel = 4`: small team channels; every channel's readers are its senders, its recipients and the owner.
    - `MentionEvery = 10`: every tenth scheduled message mentions recipients; `MentionsPerMessage = 2`: it mentions two recipients of its channel, round robin, so every recipient is mentioned in turn.
    - `Warmup = 30s`, `Window = 3m`, `Drain = 30s`, `MinSamples = 1000`, `LagSampleEvery = 1s`.

    `Plan` is a pure function, unit tested, that derives every count from the default of `ratelimit.rl.message.per_minute` (read through `internal/platform/config`, so a changed default changes the plan), from `perf.sustained_messages_per_s` (read through the report loader) and from the constants: senders = `ceil(perf.sustained_messages_per_s / (BudgetShare * ratelimit.rl.message.per_minute / 60))`; channels = `ceil(senders / SendersPerChannel)`; recipients = channels times `RecipientsPerChannel`; mention rate = `perf.sustained_messages_per_s / MentionEvery`; expected pushes per second = mention rate times `MentionsPerMessage`; `Window` must hold at least `MinSamples` expected pushes, else the plan is an error that names the constants. Senders send evenly spaced, so no planned burst approaches `ratelimit.rl.message.burst`.

5. **World and seeding** (Ruling for docs/DECISIONS.md, `D-M3-24-4`). One server for seeding and measurement, because the in-process server of Design note 3 cannot be restarted on the same database: its `Options.Config` raises every `ratelimit.*` key except `ratelimit.rl.message.per_minute` and `ratelimit.rl.message.burst`. The measured phase sends only `createMessage` (class `rl.message`, 05 4.7 row `message.create`), and the path after the commit uses no rate-limit class, so the raised keys never touch what is measured; seeding sends no message. `pushworld.Seed` creates, with the owner and step-up as the harness does:

    - one space and `channels` text channels (`createSpace`, `createChannel`), keeping each `main_thread_id`;
    - senders and recipients through `harness.NewUser` (`createUser` with `local_account`, `redeemEnrollmentCode` with one device each), with generated handles (`sender-001`, `recipient-001`) and addresses at `example.com`;
    - per channel a grant of level `write` to its `SendersPerChannel` senders and of level `read` to its `RecipientsPerChannel` recipients (`createGrant` on the channel);
    - per recipient `putNotificationPreference` with level `mentions` on its channel, so the kind `message` never reaches a recipient whatever `notify.user_default_level` says (13 3.1, 4.3);
    - per recipient one `push_registrations` row, written through `AdminDB()` because `registerPush` (M3-22) is not in the closure: `provider = 'apns'`, `environment = 'production'`, `app_id = 'com.example.chat'`, a token of 64 random hex digits, `device_id` the recipient's device, `auth_session_id` the newest `auth_sessions.id` of that device; columns as 03, no other write.

    No recipient ever opens a gateway connection, so every hint goes to its registration (13 4.6 point 2); no recipient reads, so no badge sync arises (13 8). Senders hold no registration.

6. **`TestPerf_PushHandoffP95`, sequence.** Generate the P-256 key and write it; start Fake-APNs with `apnstest.Start(apnstest.Options{TeamID, KeyID, PublicKey})`; start the server (Design note 3) with `push.apns.team_id = EXAMPLETEAM`, `push.apns.key_id = EXAMPLEKEY`, `push.apns.private_key_ref` pointing at the key file, `push.apns.topics = com.example.chat` and the raised rate limits; read `SHOW fsync` and `SHOW synchronous_commit` through the frame; seed; start the open-loop scheduler at the planned rate, round robin over the senders, each sender into the main thread of its channel; every message carries a fresh UUIDv7 `client_msg_id` and a `content_md` with a per-run canary string, a mention message names its two recipients by handle in `content_md` and lists their IDs in `mention_ids` (the server checks both sets, `MessageCreate`); every answer must be `201`; record message ID, scheduled time and mentioned recipients. Sample the consumer lag every `LagSampleEvery` (Design note 8 d). Measure for `Window` after `Warmup`, stop the scheduler, wait `Drain`, then run the checks of Design note 9.

7. **Measuring points** (Ruling for docs/DECISIONS.md, `D-M3-24-2`).

    a. Start t0: the `occurred_at` of the `message.created` event of the mention message, the time of its transaction (03 `event_log`), read after `Drain` in one read-only query through `AdminDB()` on the index `(resource_kind, resource_id)` (`type = 'message.created'`, `resource_kind = 'message'`, `resource_id = ANY(<the window's mention messages>)`). It lies at or before the commit, so each sample is an upper bound of "commit to handoff"; it is also the `DueAt` that M3-19 carries as t0 of 13 9.

    b. End t1: `apnstest.Request.AcceptedAt` (additive in M3-20's Fake), the test process time taken after the Fake has read the whole request and verified the JWT, immediately before it writes the status `200`: "the provider has accepted (`200`)" (13 9).

    c. Sample = t1 minus t0, corrected by the clock offset between database and host with the helper that M1-72's `gateway_fanout` uses. One sample per mention message whose scheduled time lies in the window and per mentioned recipient (one registration each); the value is the nearest-rank p95 of the report.

8. **Requests at the Fake.**

    a. `requests.go` decodes each recorded request strictly: path token, `apns-id`, `apns-collapse-id`, and the body as `{"aps": {...}, "b": {"v", "nid", "k", "i", "t", "m"}}` with unknown members rejected; a body without `b` is a badge sync form (13 8, M3-20 Design note 5 d).

    b. A hint record is (registration by token, `b.nid`, `b.k`, `b.t`, `b.m`, `aps.badge`, `AcceptedAt`). Tokens map to registration IDs inside the test only and are never printed.

    c. Matching: the expected set is every (mention message, mentioned recipient) of the run; a record matches by `b.m` and the recipient of its token.

    d. Extras in `results.json`: achieved message and mention rates, expected and received pushes, p50, p99 and maximum of the samples, the largest and the mean consumer lag (`max(event_log.seq)` minus `event_consumer_cursors.last_seq` for `name = 'push'`, read-only every `LagSampleEvery`), the counts of senders, recipients and channels, `GOMAXPROCS`, the note of Design note 3 c.

9. **Results and failure conditions** (Ruling for docs/DECISIONS.md, `D-M3-24-5`: a measurement counts only when the path behaved correctly while it was taken).

    - The test writes through `perf/internal/report`: `name` `perf.push_handoff_p95`, `value` the p95 in the unit of the 02 row, `threshold` and `comparison` (`<=`, meaning `target`) from the 02 table, `pass`, `samples`, `test`; parameters, plan counts and extras under `extra`; profile and durability fields from the frame unchanged.
    - The test fails, and `make perf` exits non-zero, when the p95 exceeds the threshold or on any of: an answer of `createMessage` other than `201`, among them `429 rate_limited`; an expected pair in the window without a matching request after `Drain`; two requests for one pair or two requests with one `nid`; a request whose `b.m` is not a mention message of the run, whose token belongs to no seeded registration, whose recipient was not mentioned in that message, whose `b.k` is not `mention`, whose `b.t` or `apns-collapse-id` is not the main thread of the message's channel, or whose `aps.badge` is missing or below one; a badge sync request (no recipient reads, 13 8); the canary string in any request body or header (13 1 point 1, T-PU-01); a Fake answer other than `200` (a rejected JWT); a registration with `disabled_at` set or `failure_count` above zero after `Drain` (read-only query); fewer than `MinSamples` samples.
    - Failure messages name counts, message IDs and registration IDs, never tokens.

10. **Serial packages and duration.** Two load tests on one host at the same time measure each other: if the `perf` recipe does not pass `-p 1` to `go test` yet, add it. The test adds its seeding plus `Warmup + Window + Drain`; report the total duration of `make perf`. If the time limit of the `perf` target (`PERF_TIMEOUT`, M1-01) no longer suffices, raise it together with its named value `quality.perf.timeout` in chapter 23 in the same change (the guard test of M1-01 keeps both equal); never shorten `Window` below what `MinSamples` needs. Ruling for docs/DECISIONS.md, `D-M3-24-6`, only for what was changed.

11. **Spec texts** (rule 10; nothing in 00, 03, 04 or 05 changes; all new text in English, existing text stays as it is; keep every existing column):

    - 02 17, row `perf.push_handoff_p95`, column "What is measured": keep the existing text and append "; measured from `occurred_at` of the transaction of a message that mentions the recipient (an upper bound of the commit, 13 section 9 t0) to the `200` of Fake-APNs (t1), for `mention` hints under `perf.sustained_messages_per_s`, with the server in the test process (`perf/README.md`)".
    - 02 22, row `perf.sustained_messages_per_s`, column "Meaning": keep the existing text, whose first words "sustained load" let the loader classify the row as a load parameter, and append "; also the load of `perf.push_handoff_p95`". `TestThresholdsFromChapterTable` against the real 02 proves that the row is still a parameter.
    - 13 9, a paragraph after the SLIs: the load test `perf/push_handoff` measures t1 minus t0 for `mention` hints against Fake-APNs under the sustained load, with t0 read as `occurred_at` of the triggering transaction and t1 as the `200` of the Fake; the value is held by `perf.push_handoff_p95` (02 section 17), the operational SLI stays as written.
    - 13 14, a new row: "Load test `perf/push_handoff` (`make perf`)" | "`perf.push_handoff_p95`; every mention gives exactly one push per registration, with IDs, badge and no text".
    - ADR 0063: "Confirmation" bullet "Build ticket M3-24: `make perf` measures `perf.push_handoff_p95` against Fake-APNs on a durable database in the profile modes and fails when it is exceeded"; "More Information" names the push target.

12. **Configuration keys used**, read by name, no new key and no changed default: `perf.reference_profile`, `perf.sustained_messages_per_s`, `perf.push_handoff_p95` (through the report from 02); `ratelimit.rl.message.per_minute`, `ratelimit.rl.message.burst` (defaults, never raised); every other `ratelimit.*` key (raised for seeding through `Options.Config`, Design note 5); `push.apns.team_id`, `push.apns.key_id`, `push.apns.private_key_ref`, `push.apns.topics` (set to the test values of Design note 6); `egress.allowed_hosts` (the Fake's rule, through `Options.APNsFake`); and at their defaults, named in `perf/README.md` as the configuration the value holds for: `consumer.batch_size`, `consumer.poll_interval`, `jobs.queue.delivery.max_workers`, `push.max_concurrent_requests`, `push.max_age`, `push.ttl`, `push.apns.jwt_refresh_interval`, `notify.coalesce_window`, `egress.timeout`.

13. **Rulings for `docs/DECISIONS.md`** (section 10, one row each with date, decision, reason and place): `D-M3-24-1` the server in the test process with `Options.APNsFake` and, if needed, the two wire overrides (Design note 3); `D-M3-24-2` the measuring points t0 = `occurred_at`, t1 = `AcceptedAt` before the `200` (Design note 7); `D-M3-24-3` the load shape: `mention` hints only, under `perf.sustained_messages_per_s`, with the constants (Design note 4); `D-M3-24-4` one server for seeding and measurement with every `ratelimit.*` key raised except `rl.message`, and the fixture registrations (Design note 5); `D-M3-24-5` the failure conditions besides the threshold (Design note 9); `D-M3-24-6` `-p 1` and a raised time limit, each only if made (Design note 10).

14. **A missed target** in mode `match` or `emulated` with a durable database: find the cause (the consumer lag of Design note 8 d shows whether the round or the job is slow; then a CPU, block or mutex profile of the suspected step through a Go benchmark of that code) and fix it in the module that owns the slow step, with a test that holds the behaviour, each fix listed in the report. Never lower a threshold, never change a default of a key named in this brief, never weaken a check of the path (`push.deliver` at decision and send time, the payload rules, the JWT) to gain time. If the target cannot be met without a change to 02, 13 or a new ADR decision (for example a wake signal for the job queue, ADR 0051 "Bad, because consequences have a delay until the next round"), stop with `Verdict: BLOCKED target missed` and the table and findings (ADR 0063 point 5). In mode `below` report the value as not proven.

## Tests (write them first)

Unit tests (untagged, in `make test`, no container needed):

- `perf/internal/report`: `TestThresholdsFromChapterTable` (or its merged equivalent) finds `perf.push_handoff_p95` in the fixture copy and in the real 02 with unit `s` and comparison `<=`, and still classifies `perf.sustained_messages_per_s` as a parameter after its new meaning; `TestReport_PushHandoffFailsAboveThreshold`: a fixture result for `perf.push_handoff_p95` at the threshold plus one millisecond makes the exit logic fail and marks the row failed, a result exactly at the threshold passes, and a passing value with `synchronous_commit` off in the profile still ends not green.
- `perf/internal/pushhandoff`:
    - `TestPlan_DefaultsWithinRateLimits`: with the default configuration every sender plans at most `BudgetShare` of `rl.message` and never a burst above `ratelimit.rl.message.burst`; the sender count is minimal (one sender less would exceed the share); channels, recipients and the expected pushes per second follow the formulas of Design note 4; `Window` holds at least `MinSamples` expected pushes.
    - `TestPlan_FollowsKeys`: halving `ratelimit.rl.message.per_minute` doubles the senders; doubling `perf.sustained_messages_per_s` doubles the mention rate; a `Window` too short for `MinSamples` is an error naming the constants.
    - `TestDecodeRequest`: a body built like the example of 13 6.1 decodes into a hint record; an unknown member in `aps` or `b` is an error; a body with only `aps.badge` is a sync form.
    - `TestHintSample`: sample = `AcceptedAt` minus `occurred_at`, corrected by the skew; messages scheduled in `Warmup` or after the window give no sample; the p95 is the nearest rank of the report.
    - `TestMatcher`: one subtest per failure condition of Design note 9 (missing pair, duplicate pair, duplicate `nid`, unknown message, unknown token, recipient not mentioned, kind `message`, wrong thread, wrong collapse ID, missing badge, badge zero, sync form, canary in the body, canary in a header, a Fake answer `403`), each makes the result fail with a message that contains no token; a clean set passes.
    - `TestMinSamples`: fewer than `MinSamples` samples make the result fail.

Unit, `internal/notify/adapters/apns/apnstest`:

- `TestFakeStampsAcceptedAt`: every recorded request has `AcceptedAt` after its send time in the client and before the client received the `200`; a request rejected with `403` has `AcceptedAt` zero.

Integration (container runtime, in `make test`):

- `harness`: `TestStartWithAPNsFake`: `Start` with `Options.APNsFake` and the `push.apns.*` keys; Alex mentions Sam in a channel both can read, Sam holds an `apns` fixture registration and level `mentions`; exactly one request reaches the Fake over HTTP/2 (`Request.Proto`), with `b.k = mention`, `b.m` the message and `aps.badge` at least one.
- `internal/wire` (only if the overrides were added): `TestProductionWiringHasNoTestOverrides` as in Design note 3 b.
- `perf/internal/pushworld`: `TestSeedSmallWorld`: a plan with two channels, two senders and two recipients each; `Seed` creates the grants, the preferences and one registration per recipient with the columns of Design note 5; one scheduled mention per channel gives exactly one matched request per mentioned recipient, the matcher passes and the plain messages give none.

Unit tests may use Alex, Sam and Robin as fixture names; the seeded world uses the generated handles of Design note 5.

Tagged test (build tag `perf`, run by `make perf`): `TestPerf_PushHandoffP95` as in Design notes 6 to 9. It has no red run before its package exists; the red runs to keep for the report are those of the tests above.

Commands that must pass, from the repository root:

```sh
cd services/server && go test -race -count=1 ./perf/internal/... ./test/harness/... ./internal/notify/adapters/apns/... ./internal/wire/...
cd services/server && go vet -tags perf ./perf/...
make config-reference
make adr-index
make docs
make perf
make check
```

## Acceptance criteria

- [ ] `make perf` runs `TestPerf_PushHandoffP95` against Fake-APNs and writes `perf.push_handoff_p95` with value, threshold, comparison, pass and samples, together with the profile mode and the durability fields, into `.cache/perf/results.json` and the table.
- [ ] `make perf` exits non-zero when the value exceeds the threshold (`TestReport_PushHandoffFailsAboveThreshold`).
- [ ] The threshold comes only from the 02 table; no `perf.*` value is a literal in code or docs.
- [ ] Each sample runs from `occurred_at` of the mention's transaction to the `200` of Fake-APNs; every mention in the window gives exactly one push per mentioned registration with the right message, thread, kind and a badge, and no request carries text (`TestMatcher`, `TestSeedSmallWorld`).
- [ ] The load runs at `perf.sustained_messages_per_s` with the default `rl.message` limits and no request is answered `429 rate_limited` (`TestPlan_DefaultsWithinRateLimits`).
- [ ] The production wiring cannot reach the test overrides (`TestProductionWiringHasNoTestOverrides` where the overrides were added).
- [ ] 02 17 and 22, 13 9 and 14, the configuration reference, ADR 0063, the READMEs and `docs/DECISIONS.md` carry the measuring points, the load and the rulings.
- [ ] The tagged code compiles and passes vet and lint inside `make check`; the untagged tests run in `make test`.
- [ ] On the build host the target passes in mode `match` or `emulated`; in mode `below` the report lists it as not proven.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `services/server/perf/README.md` updated (the test, measuring points, world, load shape, parameters with values, failure conditions, extras, the in-process server); READMEs of the harness, of `notify` (Fake-APNs) and of `internal/wire` (if changed) name their additions.
- `docs/spec/02-architecture.md` 17 and 22, `docs/spec/13-push-notifications.md` 9 and 14, the regenerated `docs/operations/configuration.md` and the "Confirmation" of ADR 0063 carry this ticket; rulings of Design note 13 recorded in `docs/DECISIONS.md` section 10 (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green and `make perf` run on the build host with its full table in the report (PLAN 3 item 6).
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: a lost, duplicated or misdirected push cannot end green; the measuring points cannot under-report (t0 at or before the commit, t1 at the `200`); no token, JWT or key in any output; the test overrides are unreachable from `cmd/bothy`; the load stays within the default `rl.message`; no threshold or key default was changed.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`, the full `make perf` table with profile mode, host data (logical CPUs, memory, `GOMAXPROCS`) and durability fields, the extras of the test (rates, pushes, p50, p99, maximum, consumer lag), the plan counts and the total duration of `make perf`.
3. Review and red team verdicts and the findings fixed; every fix of Design note 14 with its test.
4. What is not proven (at least: the value on `perf.reference_profile` when the mode was neither `match` nor `emulated`; the delay against the real APNs service and over a real network; FCM and gateway hints), and each difference between this brief and the merged names of the perf frame, the harness, M3-19 and M3-20.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
