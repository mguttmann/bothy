# M2-36: Performance targets of the agent path

Status: ready | Depends on: M1-72, M2-31 | Milestone: M2

## Goal

After this ticket `make perf` measures the two performance targets of the agent path on a recorded host profile, writes value and threshold of each and fails when one is exceeded: `perf.invocation_delivery_p99`, the time from the commit of a message that triggers an agent to the arrival of `agent.invocation` with its fresh invocation token at a fake adapter connected as a gateway consumer (test `perf/invocation`), and `perf.run_patch_fanout_p99`, the time from the adapter's `run.update` to the first `run.updated` carrying that version at every client that focuses the run's thread (test `perf/run_patch`). Both tests run against a server process on a durable database under the sustained message load `perf.sustained_messages_per_s` with the default rate limits, and both prove correctness while they measure: every trigger yields exactly one delivery whose token the reference verifier accepts, every run ends `completed`, and every focused client follows every version without a gap. This closes exit criterion 6 of M2 (02 "Performance targets", ADR 0063) and the mitigation "performance targets of the agent path" of RK-19; both targets stop being "unchecked" (02 17).

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket" (item 6: `make perf` in every ticket that touches a performance target, no lowered threshold); section 4 "Closing a milestone" item 3; section 5 "Test strategy" row "Load, endurance, chaos"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Exit criteria 1 and 6, Test strategy "load on the agent path", Risks) with the rows M2-07 to M2-10, M2-13 and M2-20 (what the measured path is made of), M2-31 (the dependency) and M2-36 (this ticket); section 7 rows M1-16, M1-71 and M1-72; section 19 P-07, P-09, P-10, P-11. No ticket of the plan depends on M2-36; its result is M2 exit criterion 6.
- Dependencies: the header names M2-31. Implied through it and therefore not in the header: M2-29, M2-27, M2-24, M2-23, M2-20, M2-19, M2-18, M2-17, M2-16, M2-15, M2-14, M2-13, M2-12, M2-11, M2-10, M2-09, M2-08, M2-07, M2-06, M2-05, M2-04, M2-02 and the M1 tickets below them, among them M1-01 (the `perf` frame target), M1-07 (configuration defaults), M1-12 (`pgtest`), M1-16 (`perf/event_commit`, `perf/internal/report`, `pgtest.Durable()`), M1-64 to M1-66 (gateway, hub, focus, the test client `wstest`). The header also names M1-72 (perf frame, `perf/internal/report`, `harness.StartProcess`, `Options.DurableDB`; M1-71 follows through it). The seeding of Design note 5 and the owner bootstrap of the harness also build on M1-37 (`redeemEnrollmentCode`, `createUser` with `local_account`, `setup bootstrap-owner`).
- `docs/spec/02-architecture.md`: 6 "Code layout" (`perf/`, component `testsupport`), 10 "Gateway", 11.2 "Log tail per api process", 11.4 "Hub and delivery", 11.6 "Ephemeral events" (row `run.updated`), 11.7 "Cost of serialization", 16 "Scaling path", 17 "Performance targets", 22 "Configuration values of this chapter" (all `perf.*` rows and the sentence that they are test thresholds read by `make perf`).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 2.2 "Envelope" (`occurred_at`, the delivery fields `invocation_token` and `redelivered`), 2.3, 6.4 (`4014`), 11.2 "From the adapter to the server", 11.3 "From the server to the clients" (points 2 to 5, the client rule), 15.1 "Connection and cursor", 15.2 "Ack and redelivery", 15.3 "Invocation and run start" (steps 1, 2 and 4).
- `docs/spec/09-agents.md`: 3.3 "Channel switches", 3.4 "Decision per message", 3.7 "Auto-thread from a channel message", 4.2 "Transports", 4.3 "Connection and cursor", 4.5 "Invocation token" ("Check in the adapter"), 4.7 "Adapter operations", 4.8 "Heartbeat and reported state" (`active_runs`), 4.9 "Order in the adapter: commitment first, then runtime", 5.3 "Run document (`run_document.v1`)", 5.5, 16.1 "Test doubles", 21 "Configuration values of this chapter".
- `docs/spec/05-actions.md` (core contract, read only): 3 "Rate-limit classes" (`rl.message` and `rl.trigger` per principal, `rl.agent_runtime` per agent), 4.10 rows `agent.events.consume`, `agent.heartbeat`, `run.context.read`, `run.start`, `run.update`, `run.finish`.
- `docs/spec/03-data-model.md` (core contract, read only): `runs` (`state`, `version`, `triggering_message_id`), `messages` (`trigger_state`, the content format of mentions), `event_log`; for the read-only checks after each test.
- `docs/spec/23-quality.md`: 2 (`make perf` at the end of a milestone and in the release) and 3 (coverage leaves out `perf/**`).
- ADRs 0063 "Performance gate for the event log" (points 2 to 5, Confirmation), 0025 (run as its own object for the live box), 0023 (invocation token and the check in the adapter), 0009.
- `docs/api/openapi.yaml`: operations `createMessage`, `createThread`, `createSpace`, `createChannel`, `setChannelOperator`, `createGrant`, `createUser`, `redeemEnrollmentCode`, `createAgent`, `createToken`, `createStepUpChallenge`, `completeStepUpWithDeviceKey`, `getJwks`, `sendHeartbeat`, `getRunContext`, `startRun`, `finishRun`, `getRun`; schemas `MessageCreate`, `Message`, `EventEnvelope`, `AgentInvocationEvent`, `AgentInvocationPayload`, `RunStart`, `RunFinish`, `Run`, `Heartbeat`, `HeartbeatAck`, `GatewayIdentify`, `GatewayAck`, `GatewayFocus`, `GatewayRunUpdate`, `GatewayError`, `RunUpdatedEvent`, `RunUpdatedPayload`, `Problem` (`x-problem-codes`).
- `docs/api/asyncapi.yaml`: messages `op.identify`, `op.ack`, `op.focus`, `op.run.update`, `op.error`, `event.agent.invocation`, `event.run.updated`.
- `docs/operations/configuration.md`: the keys of Design note 12.
- `docs/observability/signals.md`: 4.1 `bothy.gateway.outbound.dropped`, 4.2 `bothy.delivery.duration`, 4.3 `bothy.run.start.latency` (production signals of the same path; this ticket adds none and scrapes none).
- `docs/RISKS.md`: RK-19 (agents receive every visible channel event; its mitigation names this ticket), RK-31, RK-32. `docs/security/threat-model.md`: T-DS-01 (its test column names the load tests). `docs/DECISIONS.md`: D-AR-20, D-W-100; section 10 for the rows of this ticket. No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- Briefs: `docs/briefs/M1-72-performance-gate.md` (the perf frame this ticket extends: `perf/internal/report`, `.cache/perf/results.json`, profile modes, the seeding rule, `harness.StartProcess`, `Options.DurableDB`, the clock skew correction used by `gateway_fanout`), `M1-71-contract-tests.md` (shared harness `test/harness`), `M1-16-event-log.md` (origin of the report, `pgtest.Durable()`), `M1-01-server-gates.md` (`perf` target, `PERF_TIMEOUT`), `M1-64-gateway-connection.md` (`wstest`), `M2-06-invocation-token.md` (`runstest.ReferenceVerifier`), `M2-07-agent-consumers.md` (consumer, `ack`, in-flight window), `M2-08-invocation-delivery.md` (acceptance, `runs.Delivery`, a fresh token per delivery), `M2-09-run-lifecycle.md` (`startRun`, `finishRun`, `runs.stall_check`), `M2-10-run-document-live-box.md` (path table, `ParseOps`, coalescer, snapshot on focus, op `run.update`), `M2-13-run-context.md` (`getRunContext`), `M2-20-heartbeat-presence.md` (`sendHeartbeat`, `details.active_runs`, `HeartbeatAck.interval_ms`). `M2-31-agent-conformance-suite.md` (M2-31): read it with its plan row and `services/server/test/agents` on the base branch.
- Code on the base branch (merged names win, D-W-100): the root `Makefile` target `perf`; `services/server/perf/` (`event_commit`, `load_messages`, `gateway_fanout`, `internal/report`, `README.md`); `services/server/test/harness`; `internal/platform/pg/pgtest`; `internal/platform/config` (defaults of the keys); `internal/gateway/adapters/wstest`; `internal/runs/adapters/runstest`; `internal/runs/domain` (patch checks of M2-10); `internal/kernel/textcheck`.

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

11. `make perf` measures `perf.invocation_delivery_p99` ("commit of the triggering message to `agent.invocation` at the fake adapter", test `perf/invocation`) and `perf.run_patch_fanout_p99` ("`run.update` of the adapter to `run.updated` at the focused client", test `perf/run_patch`) and fails when one exceeds its threshold (PLAN row M2-36; 02 17; M2 exit criterion 6).
12. Thresholds only by name from the table of 02 22 (meaning `target`: the measured value must not exceed the threshold); `perf.*` values are test thresholds, not runtime configuration, and never appear as literals in code or docs (02 22, ADR 0063, the loader of M1-16 and M1-72).
13. Measured only on databases with `fsync` and `synchronous_commit` on; every result names the profile mode of `perf.reference_profile`; only `match` and `emulated` can end green, `below` never does (ADR 0063 points 2 and 3).
14. `make perf` runs at the closing of a milestone, in the release and in every ticket that touches a target, never in `make check` or the pre-push gate (P-07; PLAN 3 item 6; ADR 0063 point 4).
15. The fake adapter follows the adapter contract: one connection per consumer, `ack` after acceptance, at most `agent.max_inflight_events` unacknowledged events, `run.start` only with the delivered token after the check of 09 4.5, patches only through `run.update` with `base_version` and throttled to `agent.run_update_min_interval`, heartbeats with `active_runs` (04 15.1 to 15.3 and 11.2; 09 4.3 and 4.7 to 4.9).
16. Content changes reach clients only as `run.updated`, coalesced per `sync.run_patch_coalesce_interval` and only on connections that focus the thread; focusing sends the whole document once; a client applies a frame whose `base_version` equals its version, ignores older frames and treats anything else as a gap (04 11.3 points 2 to 5).
17. Every principal of the load stays within its rate-limit class under the default limits of the measured server (`rl.message`, `rl.trigger`, `rl.agent_runtime`; 05 3); the measured server runs with the default configuration (rule of M1-72).
18. No threshold is lowered and no default of a key named in this brief is changed to pass a target (PLAN 3 item 6; ADR 0063 point 5).

## Scope

All paths under `services/server/` unless they start with `docs/` or name the root `Makefile`. Names of M1-72's perf frame below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `perf/invocation/` (build tag `perf`, the one tag of M1-16 and M1-72): `invocation_test.go` with `TestPerf_InvocationDeliveryP99` (Design notes 6 and 8) and a package comment naming the target.
- `perf/run_patch/` (same tag): `run_patch_test.go` with `TestPerf_RunPatchFanoutP99` (Design notes 7 and 8). The directory names are those of 02 17 and the plan row; the package clause follows the convention M1-72 chose for `load_messages` and `gateway_fanout`.
- `perf/internal/agentpath/` (untagged, unit tested in `make test`): `constants.go` (the named test parameters of Design note 4), `plan.go` (`Plan` derived from configuration defaults and the constants), `latency.go` (invocation samples and the patch version matcher of Design notes 6 and 7), `doc.go`.
- `perf/internal/fakeadapter/` (untagged, unit tested): the fake adapter of Design note 3 with `adapter.go` (connection, read loop, modes `invocation` and `patch`), `ack.go` (ack policy), `patches.go` (schedule and op generator), `heartbeat.go`, `transport.go` (small interfaces for the gateway connection and the REST calls, bound to `wstest` and the harness client; the unit tests use fakes), `doc.go`.
- `perf/internal/agentworld/` (untagged, compiled and vetted in `make check`): `Seed` builds the world of Design note 5 through the public API of the seeding server and returns handles (agents with tokens, channels with their main thread ids, background threads, trigger persons, background senders, viewers).
- `perf/internal/load/` only if M1-72 kept its open-loop scheduler and sender set inside `perf/load_messages`: move them here without a change of behaviour so that `load_messages` and both new tests use one scheduler; M1-72's tests keep their names and stay green. Never write a second scheduler.

Change:

- `perf/internal/report` (M1-16, M1-72): no new mechanism. If the threshold loader or its tests hold a list of expected names, add `perf.invocation_delivery_p99` and `perf.run_patch_fanout_p99`; extend `TestThresholdsFromChapterTable` (or its merged equivalent) with both names; add the exit-logic test of "Tests".
- `perf/README.md`: one section per new test (what is measured, the measuring points, the load shape, the parameters with their values, the failure conditions, how to read `extra`).
- Root `Makefile`, target `perf`: add `-p 1` to its `go test` call if it does not serialize the packages yet (Design note 10). No new target.
- `docs/spec/02-architecture.md` (English, like the chapter): 17 rows `perf.invocation_delivery_p99` and `perf.run_patch_fanout_p99` with the measuring points and the load of Design note 9; 22 row `perf.sustained_messages_per_s` with the meaning of Design note 9.
- `docs/spec/09-agents.md` 16.1 "Test doubles": the row "Fake adapter (load)" of Design note 9.
- `docs/operations/configuration.md`: regenerate with `make config-reference` (the row of `perf.sustained_messages_per_s` changes).
- `docs/adr/0063-performance-gate-for-the-event-log.md`: "Confirmation" gains a bullet for M2-36 (both targets, the fake adapter, durable database, profile modes, failure on exceeding); "More Information" names the agent path targets; the decision stays unchanged; then `make adr-index`.
- `docs/DECISIONS.md` section 10: rows `D-M2-36-<n>` (Design note 13).
- `services/server/CHANGELOG.md` under "Unreleased".

If the base branch already carries the sentences of 02 17, 02 22 or 09 16.1 named above (a planning change made after this brief), keep them and change nothing there.

This ticket creates no migration, no operation, no event, no metric, no configuration key and no new top-level make target.

## Out of scope

- The thresholds and `perf.reference_profile` themselves; the M1 targets (M1-16, M1-72), `perf.fanout_cross_node_p99` (M7) and `perf.push_handoff_p95` (M3).
- The conformance suite AG-01 to AG-30, the Python fake agent and the SDK FakeServer (M2-29, M2-31); the chaos scenarios of the agent path named in PLAN 8 "Test strategy" (AG-09, AG-10), which run in M2-31's suite in `make check`.
- Measuring the REST poll or the webhook transport to agents (Design note 3 a); `cluster.mode = multi` and the `NOTIFY ephemeral` path of `run.updated` (M7-17).
- Terminal load and chaos (TM-T16 in `make perf` comes with M2-48).
- New production signals or alerts; the perf tests read no metrics endpoint.
- Optimisations beyond what a missed target requires (Design note 11).
- The milestone-closing run on the reference host itself (owner operation after the merge, PLAN 4 item 3).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-36-agent-path-perf`; one writer per worktree.
- Conventional Commits with scope (for example `test(perf): ...`, `docs(spec): ...`, `perf(<module>): ...` for a fix of Design note 11) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief and the fixes of Design note 11, each with a test and listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1`; in-process listeners use port 0, and the server subprocess of `harness.StartProcess` listens on loopback ports picked free just before its start (M1-72). PostgreSQL runs only as a throwaway container through `pgtest` with `pgtest.Durable()` (P-09, P-10). Without a Docker-compatible runtime the tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end.
- Never read or print secrets. Invocation tokens and API tokens never appear in logs, test output, `results.json` or failure messages (print a hash prefix of a `jti` if an id is needed). No network calls except package and image downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test. A missed target on a host in mode `below` is reported as not proven, never "fixed" by changing numbers.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M2-05 to M2-31: follow their names and list each difference in the report.
- No agent runtime, no model call, no SDK and no third-party service in any test: the adapter side is the Go fake adapter of Design note 3.
- Raised rate limits only on the seeding server; the measured server runs with the default configuration.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red` and the failing gate. Confirm that the base has M2-31 (`services/server/test/agents` runs in `make check`). Run `make perf` once only at the end (Definition of done); if an M1 target fails in that run, run the same package on the base commit and report both values, so that a host limit is not mistaken for a regression of this ticket.

2. **Packages.** `perf/invocation` and `perf/run_patch` hold only the two tagged tests and their orchestration. Everything that can be tested without a server lives untagged in `perf/internal/agentpath` (plan, sample computation, matcher) and `perf/internal/fakeadapter` (adapter logic behind small transport interfaces), so `make test` runs their unit tests and `make check` compiles, vets and lints them. `perf/**` is the architecture component `testsupport` (02 6; M1-05) and may import `internal/runs/adapters/runstest`, `internal/runs/domain`, `internal/gateway/adapters/wstest` and `test/harness`; nothing outside `testsupport` imports these packages. Coverage leaves `perf/**` out (23 3).

3. **Fake adapter** (Ruling for docs/DECISIONS.md: 02 17 names a "fake adapter" as the measuring point, while 09 16.1 knows only the fake agent on the Python SDK, whose interpreter and process boundary would sit inside the measured interval; the perf fake adapter is a Go client without SDK in `perf/internal/fakeadapter`, and 09 16.1 gets its row). One `Adapter` per agent:

    a. Connection: the gateway test client `wstest` of M1-64 with the agent's API token and the consumer name of Design note 4, `identify` to `ready`, automatic gateway `heartbeat`; where the base connects agent consumers through the harness `GatewayConn` instead (M2-07), use that. Only the gateway transport is measured: it is the standard for every adapter that runs permanently (09 4.2); the REST poll depends on the adapter's own poll cadence and the webhook transport has no owner yet. Ruling for docs/DECISIONS.md.

    b. Read loop: one goroutine per connection. The receive time of a frame is `time.Now()` taken as soon as the frame has been read from the socket, before any decoding; then the frame is decoded strictly (unknown members rejected) as `EventEnvelope` or `GatewayError`.

    c. Ack policy (04 15.2; an `ack` costs `rl.agent_runtime`, 05 4.10 `agent.events.consume`): a cumulative `ack` with the highest received `seq` right after an invocation-bearing event has reached its ack step (d); otherwise when the unacknowledged count reaches half of `agent.max_inflight_events`; otherwise when `agent.ack_persist_interval` has passed since the last ack and an event is unacknowledged. The window never fills, so the server never waits on the adapter during a measurement.

    d. Invocation handling, in the order of 09 4.9, all after the receive time and therefore outside the measured interval: an envelope with `redelivered: true` is recorded as an error; the token is checked with `runstest.ReferenceVerifier.Check` (M2-06) against `getJwks` with the expectations agent, run, thread, actual billing mode `organization` and tool class `chat`; a rejection is recorded as an error, the run is reported with `finishRun` `failed` and reason `invalid_invocation_token` and the event is acknowledged without a start (09 4.9); `ack` (c); `getRunContext`; `startRun` with `billing_mode_used: organization` and `tool_class_used: chat`; then in mode `invocation` `finishRun` with `state: completed`, in mode `patch` the run joins the patch schedule (e). A second `agent.invocation` for the same run is recorded as an error.

    e. Patches (mode `patch`; 04 11.2; 09 4.7): per run one `run.update {run_id, base_version, ops}` on the adapter's own connection every `agent.run_update_min_interval`, the runs of one adapter phase-shifted evenly over that interval. `base_version` starts at the `version` of the `startRun` answer and rises by one per patch, because one stored patch is one version step (03 `runs.version`) and a successful op sends no frame (M2-10 note 11). The send time of a patch is its scheduled time; when the writer lags, the scheduled time still counts, so a slow server cannot hide behind a slow generator. Every `error` frame is recorded with op, code and `ref`.

    f. Ops of patch k, all on paths of 09 5.3 with values M2-10 accepts: `replace /status_line` with a short ASCII display text naming k; `replace /last_text_excerpt` with a text that grows by a fixed chunk per patch (the server keeps the tail of `run.excerpt_max_chars`); `replace /metrics/tokens_out` with an integer that rises with k; every `StepEvery`-th patch also `add /steps/<new UUIDv7>` with `kind: tool_call`, `name: search`, a `title` and `status: running`, and the next patch also `replace /steps/<that id>/status` with `completed`.

    g. Heartbeat: `sendHeartbeat` once before the consumer connects and then every `HeartbeatAck.interval_ms` with `state: online`, `billing_modes_supported: [organization]`, `reported_tool_class: chat` and `details.active_runs` listing the adapter's runs in `running` (09 4.8; M2-20), so `runs.stall_check` (M2-09) never ends a measured run.

    h. No model, no runtime, no reply messages: the adapter accepts, starts, patches and finishes, nothing else.

4. **Load shape** (Ruling for docs/DECISIONS.md; 02 17 names no load for these two targets): both tests run under the background load of `perf.sustained_messages_per_s` human messages, the sustained design load of 02 22, against the measured server in default configuration, because the agent path shares the process, the pool and `event_seq` with chat. The test parameters are named constants in `perf/internal/agentpath/constants.go`, documented with their values in `perf/README.md` and copied into `results.json` (M1-72 convention). Note on rule 9: these constants are parameters of a test, not limits of the product; the product limits they depend on are read by key name.

    - `TriggerEvery = 10`: in `perf/invocation` every tenth human message triggers an agent and the rest are background messages; in `perf/run_patch` all human messages are background messages. Reason: agents are a core use of every channel, and a tenth keeps the trigger path a real share of the load without replacing chat.
    - `BudgetShare = 0.5`: every principal plans at most half of its rate-limit class; the other half absorbs cumulative acks, bursts and jitter.
    - `ChannelsPerAgent = 2`, `ThreadsPerChannel = 20` (background threads per channel), `ViewersPerRun = 4`, `StepEvery = 4`, `CallsPerRun = 4` (ack, `getRunContext`, `startRun`, `finishRun`).
    - `Warmup = 30s`, `Window = 3m`, `Drain = 30s`, `MinSamples = 1000`, `ConsumerName = "perf"`.

    The `Plan` is a pure function, unit tested, that derives every count from the defaults of the configuration keys (read through `internal/platform/config`, so a changed default changes the plan), from `perf.sustained_messages_per_s` (read through the report loader) and from the constants:

    - trigger rate = `perf.sustained_messages_per_s / TriggerEvery`. In `perf/invocation` triggers run at this rate during the measurement and the background rate is the rest of `perf.sustained_messages_per_s`; in `perf/run_patch` the background rate is all of `perf.sustained_messages_per_s`, and the trigger persons post only the setup triggers of the patch runs, evenly spaced at the trigger rate, before the background load starts. One plan and one world serve both tests.
    - agents `A` = the smallest number with `(trigger rate / A) * CallsPerRun + 1 / heartbeat interval <= BudgetShare * ratelimit.rl.agent_runtime.per_minute / 60`, where the heartbeat interval is `sync.heartbeat_interval` (the `interval_ms` that M2-20 answers); at least one.
    - patch runs per agent `R` = the largest number with `R / agent.run_update_min_interval + 1 / heartbeat interval <= BudgetShare * ratelimit.rl.agent_runtime.per_minute / 60`; `R < 1` is an error that names these keys.
    - trigger persons = `ceil(trigger rate / (BudgetShare * min(ratelimit.rl.message.per_minute, ratelimit.rl.trigger.per_minute) / 60))`, so they stay inside `rl.trigger` too, should a triggering message ever consume it; background senders = `ceil(perf.sustained_messages_per_s / (BudgetShare * ratelimit.rl.message.per_minute / 60))`, sized for the full rate of `perf/run_patch`; in `perf/invocation` the same senders send the smaller background rate, each correspondingly less often.
    - viewers = `A * R * ViewersPerRun`, one person with one device each; every viewer focuses one thread, below `sync.max_focused_threads`.
    - Senders send evenly spaced, so no planned burst approaches the `.burst` keys.

5. **World and seeding** (the rule of M1-72: seeding through the public API with a seeding server whose `ratelimit.*` keys are raised, then the measured server on the same database with the default configuration; both through `harness.StartProcess` with the durable database). `agentworld.Seed` creates, with the owner and step-up as the harness does:

    - one space and per agent `ChannelsPerAgent` text channels with the default switches of 09 3.3 (`require_mention` and `auto_thread` set; `thread_require_mention`, `free_response` and `allow_mentioned_agents` not set), the agent set as operator through `setChannelOperator`;
    - `A` agents through `createAgent` (`handle`, `display_name`, `adapter_kind`; `AgentCreate` has no billing mode or tool class, and the 03 `agents` defaults `organization` and `chat` are what the measured path needs), each with an API token through `createToken`, as the harness `NewAgent` does; assert `billing_mode = organization` and `tool_class = chat` on the `createAgent` answer;
    - trigger persons with a grant of level `trigger` on the space, background senders with `write`, viewers with `read` (`createGrant`); every person through `createUser` and `redeemEnrollmentCode` with one device;
    - `ThreadsPerChannel` threads per channel through `createThread` for the background;
    - generated handles and display names (`agent-01`, `trigger-001`, `sender-001`, `viewer-001`) with addresses at `example.com`.

    Background messages come from write-only senders into the background threads, open loop and evenly spaced, round robin over the threads (the scheduler of `perf/internal/load` or the merged equivalent). The operator is a candidate there, but the author lacks `trigger`, so nothing triggers (09 3.4 point 4), while every message still passes acceptance and reaches the agent's consumer as a channel event. That is the load RK-19 describes. The viewers are connected in both tests; in `perf/invocation` they only receive the channel events of the space.

6. **`TestPerf_InvocationDeliveryP99`** (`perf.invocation_delivery_p99`):

    a. Sequence: seed; start the measured server (profile limits as M1-72's mode decides); connect the `A` fake adapters in mode `invocation`, heartbeat first; connect the viewers; start the background load and the trigger generator; measure for `Window` after `Warmup`; stop both generators; wait up to `Drain` for outstanding invocations; then the checks of Design note 8.

    b. Trigger generator: open loop at the trigger rate, evenly spaced, round robin over the pairs of trigger person and channel: `createMessage` into the channel's main thread with `content_md` mentioning the channel's operator agent by its handle (the server resolves mentions, 03 `messages`) and a fresh UUIDv7 `client_msg_id`. The answer must be `201` with `trigger_state = received` (04 15.3 step 1, M2-08); message id and scheduled time are recorded. With `auto_thread` each trigger creates a new thread (09 3.4 point 5 and 3.7), so no trigger meets an active run and the generator needs no thread pool.

    c. Measuring points (Ruling for docs/DECISIONS.md): start is the `occurred_at` of the received `agent.invocation` envelope, the time of the transaction of the triggering message, because the event is written in that transaction (04 2.2, 15.3 step 1); it lies at or before the commit, so the sample is an upper bound of "commit to receipt". End is the receive time of Design note 3 b. The sample is end minus start, corrected by the clock offset between database and host with the helper that M1-72's `gateway_fanout` uses. The interval contains the log tail, the hub, the consumer runner of M2-07, `runs.Delivery` with the fresh token (M2-08) and the write of the frame.

    d. One sample per trigger whose scheduled time lies inside the window, matched by `AgentInvocationPayload.message_id`; the value is the nearest-rank p99 of the report (M1-72).

    e. Extras in `results.json`: achieved trigger and background rates, invocations received, p50 and maximum, the p99 of the `startRun` and `finishRun` round trips, planned and highest observed calls per agent and second, `A` and all person counts.

7. **`TestPerf_RunPatchFanoutP99`** (`perf.run_patch_fanout_p99`):

    a. Sequence: seed; start the measured server; connect the `A` fake adapters in mode `patch`; for each of the `A * R` runs one trigger person posts one trigger as in Design note 6 b, evenly spaced at the trigger rate of the plan, and the adapter starts the run (Design note 3 d); setup triggers give no sample; wait until every run is `running` (the `startRun` answer of its adapter), because the version step of `startRun` after a focus would reach the viewer as a snapshot (M2-10 note 9); connect the viewers, each sends `focus` with its run's thread (`GatewayFocus` with one thread id) and waits for the one `run.updated` with `snapshot` that focusing sends (04 11.3 point 5), which sets its local version; start the background load (no triggers) and the patch schedule; measure for `Window` after `Warmup`; stop; wait up to `Drain`; read `getRun` for every run and keep its `version` as the final version; then `finishRun` with `completed` for every run (its transition raises `version` without a `run.updated`, M2-05); then the checks of Design note 8.

    b. Viewer reading, the client rule of 04 11.3 point 4: every frame gets its receive time (Design note 3 b) and is decoded strictly as `RunUpdatedEvent`; a frame with `snapshot` sets the local version (every snapshot after the first is a late snapshot); a frame whose `base_version` equals the local version sets it to `version`; a frame whose `version` is not above the local version is ignored; anything else is a gap.

    c. Measuring points (Ruling for docs/DECISIONS.md): start is the scheduled send time of the patch that produced version v (Design note 3 e); end is the receive time of the first frame that lifts the viewer's local version to v or above. One sample per version and viewer, for versions whose scheduled time lies inside the window, so many viewers of one live box count as many samples (RK-32). Both times come from the clock of the test process; no skew correction applies. The interval contains the gateway op path, the stored patch, the coalescing window `sync.run_patch_coalesce_interval` and the write of the frame.

    d. Extras: achieved patches per second, viewer connections, frames received per version (the coalescing ratio), p50 and maximum, `A` and `R`.

8. **Results and failure conditions** (Ruling for docs/DECISIONS.md: a measurement counts only when the path behaved correctly while it was taken):

    - Both tests write through `perf/internal/report` (M1-72): `name` the target, `value` the p99 in the unit of the 02 row, `threshold` and `comparison` (`<=`, meaning `target`) from the 02 table, `pass`, `samples`, `test`; parameters, plan counts and extras under `extra`; profile and durability fields come from M1-72's report unchanged.
    - A test fails, and `make perf` exits non-zero, when its p99 exceeds the threshold or on any of: a trigger in the window without exactly one `agent.invocation`; an invocation for an unknown message; a `redelivered` envelope; a token rejected by the reference verifier; a run of the test that does not end `completed` (read-only query on `runs` through the admin pool of the pgtest container, as M1-72's invariants query); any HTTP answer other than the expected `2xx`, among them `429 rate_limited`; any `error` frame or close frame on any connection; a gap; a late snapshot (it means a viewer's queue overflowed and patches were dropped, which this target does not describe, 04 11.3 point 3); a viewer whose final local version differs from the final version read through `getRun` after `Drain` and before `finishRun`; a version in the window that did not reach every viewer; fewer than `MinSamples` samples.
    - Failure messages name counts, run ids and message ids, never tokens.

9. **Spec texts** (English, like the chapters; keep the "Test" column):

    - 02 17, row `perf.invocation_delivery_p99`: "commit of the triggering message (from the `occurred_at` of its transaction, upper bound) to the receipt of `agent.invocation` with invocation token at the fake adapter (09 section 16.1), under `perf.sustained_messages_per_s` with a share of triggering messages (`perf/README.md`)".
    - 02 17, row `perf.run_patch_fanout_p99`: "`run.update` of the fake adapter (09 section 16.1) to the first `run.updated` with this version at every focused client, under `perf.sustained_messages_per_s`".
    - 02 22, row `perf.sustained_messages_per_s`, meaning: "sustained load for `perf.message_create_p99`, `perf.invocation_delivery_p99` and `perf.run_patch_fanout_p99`". The text keeps its start "sustained load", so M1-72's loader still classifies the row as a load parameter and not as a target; `TestThresholdsFromChapterTable` against the real 02 proves it.
    - 09 16.1, new row "Fake adapter (load)": purpose "Measuring point of the performance targets of the agent path (02 section 17)"; setup "Go client without SDK in `services/server/perf/internal/fakeadapter`: consumer at the gateway with `ack`, checks every token with the reference verifier, starts and finishes runs, sends `run.update` throttled to `agent.run_update_min_interval` and heartbeats with `active_runs`; no runtime, no model".

10. **Serial packages and duration.** Two load tests on one host at the same time measure each other: if the `perf` recipe does not pass `-p 1` to `go test` yet, add it (Ruling for docs/DECISIONS.md). Each new test adds its seeding plus `Warmup + Window + Drain`; report the total duration of `make perf`. If the time limit of the `perf` target (`PERF_TIMEOUT`, M1-01) no longer suffices, raise it together with its named value in chapter 23 in the same change (the guard test of M1-01 keeps both equal) and record a row; never shorten `Window` below what `MinSamples` needs.

11. **A missed target** in mode `match` or `emulated` with a durable database: find the cause (for example a CPU, block or mutex profile of the suspected step through a Go benchmark of that code, or the profiling the base offers for the server process) and fix it in the module that owns the slow step, with a test that holds the behaviour, each fix listed in the report. Never lower a threshold, never change a default of a key named in this brief, never weaken a check of the path (the delivery check, the token, `authorize`, the coalescer rules) to gain time. If the target cannot be met without a change to 02 or a new ADR decision, stop with `Verdict: BLOCKED target missed` and the table and findings (ADR 0063 point 5). In mode `below` report the values as not proven.

12. **Configuration keys used**, read by name, no new key and no changed default: `perf.reference_profile`, `perf.sustained_messages_per_s`, `perf.invocation_delivery_p99`, `perf.run_patch_fanout_p99`, `ratelimit.rl.message.per_minute`, `ratelimit.rl.message.burst`, `ratelimit.rl.trigger.per_minute`, `ratelimit.rl.trigger.burst`, `ratelimit.rl.agent_runtime.per_minute`, `ratelimit.rl.agent_runtime.burst`, `agent.run_update_min_interval`, `agent.max_inflight_events`, `agent.ack_persist_interval`, `agent.ack_timeout`, `agent.presence_timeout`, `agent.invocation_token_ttl`, `adapter.invocation_token_leeway`, `adapter.jwks_refresh_min_interval`, `sync.heartbeat_interval`, `sync.run_patch_coalesce_interval`, `sync.max_focused_threads`, `sync.outbound_queue_max`, `run.excerpt_max_chars`, `run.max_steps_persisted`, `run.orphan_timeout`. The seeding server raises the `ratelimit.*` keys through the harness `Config` map, never through code.

13. **Rulings for `docs/DECISIONS.md`** (section 10, one row `D-M2-36-<n>` each, with date, decision, reason and place): the Go fake adapter as the measuring point and the gateway as the only measured transport (Design note 3 and 3 a); the load shape with `TriggerEvery` and `BudgetShare` under `perf.sustained_messages_per_s` and the default limits (Design note 4); the measuring points of `perf.invocation_delivery_p99` (Design note 6 c); the measuring points and per-viewer samples of `perf.run_patch_fanout_p99` (Design note 7 c); the failure conditions besides the threshold (Design note 8); `-p 1` and a raised time limit, each only if made (Design note 10).

## Tests (write them first)

Unit tests (untagged, in `make test`, no container needed):

- `perf/internal/report`: `TestThresholdsFromChapterTable` (or its merged equivalent) finds `perf.invocation_delivery_p99` and `perf.run_patch_fanout_p99` in the fixture copy and in the real 02, each with unit `ms` and comparison `<=`, and still classifies `perf.sustained_messages_per_s` as a parameter after its new meaning; `TestReport_AgentPathTargetsFailAboveThreshold`: a fixture result for `perf.invocation_delivery_p99` with the threshold plus one millisecond makes the exit logic fail and marks that row failed, a result exactly at the threshold passes; the same for `perf.run_patch_fanout_p99`; a passing value with `synchronous_commit` off in the profile still ends not green.
- `perf/internal/agentpath`:
    - `TestPlan_DefaultsWithinRateLimits`: with the default configuration every planned principal stays at or below `BudgetShare` of its class (`rl.agent_runtime` per agent in both modes, `rl.message` and `rl.trigger` per trigger person, `rl.message` per sender); `A` is minimal (`A - 1` agents would exceed the share), `R` is maximal; viewers equal `A * R * ViewersPerRun`.
    - `TestPlan_FollowsKeys`: halving `ratelimit.rl.agent_runtime.per_minute` gives at least as many agents and at most as many runs per agent; doubling `perf.sustained_messages_per_s` doubles the trigger rate; an `agent.run_update_min_interval` short enough that `R < 1` gives an error naming the keys.
    - `TestInvocationSample`: sample = receive time minus `occurred_at`, corrected by the skew; triggers scheduled in `Warmup` or after the window give no sample; a missing and a duplicated invocation are reported with their message id.
    - `TestPatchMatcher`: a frame covering v to v+2 gives three samples, each from its own send time; the first snapshot sets the version without a sample; a later snapshot gives samples and counts as late; an older frame is ignored; a gap is an error; a version sent in the window that never arrives is reported as missing; versions sent before the window give no sample.
    - `TestMinSamples`: fewer than `MinSamples` samples make the result fail.
- `perf/internal/fakeadapter` (fake transport, fake REST client, fake clock where timing matters):
    - `TestAckPolicy`: an ack right after an invocation-bearing event; at half of `agent.max_inflight_events`; after `agent.ack_persist_interval` with something unacknowledged; over a random stream never more than `agent.max_inflight_events` unacknowledged; every ack names the highest received `seq`.
    - `TestInvocationHandling_Order`: the receive time is taken before decoding; then token check, ack, `getRunContext`, `startRun` with `organization` and `chat`, then `finishRun` with `completed` in mode `invocation` or the patch schedule in mode `patch`; a rejected token, a `redelivered` envelope and a second invocation for one run are recorded as errors and start nothing; the rejected token leads to `finishRun` `failed` with `invalid_invocation_token`, then the ack.
    - `TestPatchSchedule`: one patch per run per `agent.run_update_min_interval`; the runs of one adapter phase-shifted evenly; base versions v, v+1, v+2; a lagging writer keeps the scheduled time as send time.
    - `TestPatchOps_Allowed`: every patch of a long generated sequence passes `ParseOps` of `internal/runs/domain` (M2-10, or the merged equivalent) without a field error, and its display texts pass `textcheck.DisplayText`.
    - `TestHeartbeat`: the first heartbeat goes out before the consumer connects; the interval comes from `HeartbeatAck.interval_ms`; `details.active_runs` lists exactly the running runs of the adapter; the body validates against `Heartbeat`.
    - `TestErrorFrameFails`: an `error` frame for `run.update` (codes `rate_limited` and `base_version_conflict`) is recorded with op, code and `ref`, and the adapter's report fails.

Unit tests may use Alex, Sam and Robin as fixture names; the seeded world uses the generated handles of Design note 5.

Tagged tests (build tag `perf`, run by `make perf`): `TestPerf_InvocationDeliveryP99` and `TestPerf_RunPatchFanoutP99` as in Design notes 6 to 8. They have no red run before their packages exist; the red runs to keep for the report are those of the unit tests above.

Commands that must pass, from the repository root:

```sh
cd services/server && go test -race -count=1 ./perf/internal/...
cd services/server && go vet -tags perf ./perf/...
make config-reference
make config-doc-check
make adr-index
make docs
make perf
make check
```

## Acceptance criteria

- [ ] `make perf` runs `TestPerf_InvocationDeliveryP99` and `TestPerf_RunPatchFanoutP99` and writes `perf.invocation_delivery_p99` and `perf.run_patch_fanout_p99` with value, threshold, comparison, pass and samples, together with the profile mode, into `.cache/perf/results.json` and the table.
- [ ] `make perf` exits non-zero when one of the two exceeds its threshold (`TestReport_AgentPathTargetsFailAboveThreshold`).
- [ ] Thresholds come only from the 02 table; no `perf.*` value is a literal in code or docs.
- [ ] The invocation latency runs from `occurred_at` of the triggering transaction to the receipt at the fake adapter; every trigger in the window has exactly one invocation with a token the reference verifier accepts, no redelivery, and every run ends `completed`.
- [ ] The patch latency runs from the scheduled `run.update` to the first `run.updated` with that version at every focused viewer; no gap, no late snapshot, no error or close frame; every viewer ends at the version `getRun` reports after `Drain`, before the run is finished.
- [ ] The load runs at `perf.sustained_messages_per_s` with the default rate limits and no principal is rate limited (`TestPlan_DefaultsWithinRateLimits`; no `429 rate_limited` during the run).
- [ ] 02 17 and 22, 09 16.1, the configuration reference, ADR 0063 and `docs/DECISIONS.md` carry the measuring points, the load and the rulings.
- [ ] The tagged code compiles and passes vet and lint inside `make check`; the untagged unit tests run in `make test`.
- [ ] On the build host both targets pass in mode `match` or `emulated`; in mode `below` the report lists them as not proven.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `services/server/perf/README.md` updated (the two tests, measuring points, load shape, parameters with values, failure conditions, extras).
- `docs/spec/02-architecture.md` 17 and 22, `docs/spec/09-agents.md` 16.1, the regenerated `docs/operations/configuration.md` and the "Confirmation" of ADR 0063 carry this ticket; Rulings of Design note 13 recorded in `docs/DECISIONS.md` section 10 (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green and `make perf` run on the build host with its full table in the report (PLAN 3 item 6).
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: a lost, duplicated or redelivered invocation or a dropped patch cannot end green; the measuring points cannot under-report (start before commit, scheduled send time); no token in any output; the load stays within the default limits; no threshold or key default was changed.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`, the full `make perf` table with profile mode, host data (logical CPUs, memory, `GOMAXPROCS`) and durability fields, the extras of both tests, the plan counts (`A`, `R`, persons, viewers) and the total duration of `make perf`.
3. Review and red team verdicts and the findings fixed; every fix of Design note 11 with its test.
4. What is not proven (at least: the values on `perf.reference_profile` when the mode was neither `match` nor `emulated`), and each difference between this brief and the merged names of M1-72, M2-06, M2-07, M2-10 or M2-20.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
