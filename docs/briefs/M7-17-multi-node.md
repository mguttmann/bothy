# M7-17: Multi-node operation (level 2)

Status: ready | Depends on: M1-72, M2-10, M2-47, M3-23 | Milestone: M7

## Goal

After this ticket an instance runs with several `api` nodes behind a proxy and a separate `worker` (02 16 level 2, OQ-07): with `cluster.mode = multi` the channel `NOTIFY ephemeral` carries the ephemeral events of 02 11.6 between nodes (typing, `run.updated` by ID and version, agent presence, the replacement of a device connection, `notification.created` and `command.autocomplete_requested` by ID), the agent consumers are fenced through `agent_consumer_cursors.holder_session_id` so that a second `identify` of the same consumer on another node closes the older connection with `4015` and no run is executed twice, a second `identify` of the same device on another node closes the older connection with `4017`, and all nodes share one signing key store (a shared volume or a writable driver per target, ADR 0066). The worker gets what M3-23 left for this ticket: the shared presence state in the table `gateway_presence` (R-DM-51), which every `api` process in role `api` writes and the consumer `push` reads through `presence.View`, and the path of `notification.created` from the worker to the api nodes, so desktop hints work with separate roles and several api nodes (13 12 point 4, RK-37, D-W-108). Typing and the live box reach clients on another node within `perf.fanout_cross_node_p99`. Why: exit criterion 5 of M7; a rolling update without downtime and a node that reaches `perf.ws_connections_per_node` need more than one api node, and every per-process state that M1 to M6 kept in memory must work across nodes or fail safe.

## Read first

- `docs/spec/02-architecture.md`: 3.2 (roles), 11.5, 11.6 (ephemeral events table, `NOTIFY` only as a wake-up without content), 15.3, 16 (levels, "Multiple nodes in detail"), 17 (`perf.fanout_cross_node_p99`, `perf.ws_connections_per_node`).
- `docs/spec/03-data-model.md` (core contract, read only): `agent_consumer_cursors` (`holder_session_id`, the guarded update), `gateway_presence` (R-DM-51), 6.6 (`NOTIFY`).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 6.1 (`4017`), 6.4 (close codes `4015`, `4017`), 15 (agent connections, 15.1 fencing, 15.3).
- `docs/spec/13-push-notifications.md` 4.4, 4.6, 12 point 4; `docs/spec/16-operations.md` 5.2 (`secrets.signing_key_dir`), 13, 14, R-OP-19; `docs/adr/0066-signing-keys-in-the-writable-key-store.md`; `docs/spec/24-terminal.md` 9 (relay across nodes, already built by M2-47).
- `docs/RISKS.md` RK-37, `docs/OPEN-QUESTIONS.md` OQ-07, `docs/operations/configuration.md` (`cluster.mode`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), R-DM-51 (D-W-128), D-W-108; section 10 (format of `D-M7-17-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-65-gateway-delivery.md` (log tail and `NOTIFY`), `M1-66-gateway-focus-typing.md`, `M2-07-agent-consumers.md` (consumer sessions), `M2-10-run-document-live-box.md` (`run.updated` coalescing), `M2-47-terminal-multi-node.md` (hash routing, the two-node test harness), `M3-23-desktop-notices.md` (`presence.View`, `LiveRoutes`, the role wiring and "A shared presence state ... belong to M7-17"), `M1-32-signing-keys.md`, `M1-72-performance-gate.md` (`make perf`).
- Code on the base branch: `services/server/internal/gateway`, `internal/runs`, `internal/agents`, `internal/notify`, `internal/kernel/presence`, `internal/instance` (signing keys), `internal/wire`, `perf/`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names.
9. Intervals and limits only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour or configuration updates `docs/spec/02-architecture.md`, `13-push-notifications.md` or `16-operations.md` in the same change. The core contract is not edited by this ticket; `gateway_presence` is built exactly as 03 has it. Everything this ticket adds is English.
11. No cost (D-W-125).

Product requirements implemented here:

12. `NOTIFY` carries only IDs and versions, never content (02 11.6); the channel `ephemeral` is active only with `cluster.mode = multi`.
13. Fencing: a node that loses the cursor row (the guarded `UPDATE ... WHERE holder_session_id = $own_session` affects no row) closes its consumer connection with `4015` before it delivers another event; run starts stay compare-and-swap (04 15.3); a run is never executed twice.
14. A device holds one connection across all nodes: a newer `identify` or `resume` closes the older one with `4017` on whichever node holds it.
15. `gateway_presence` holds only counted connections of humans in phase `live` (M3-23 Design note 2), is written by processes in role `api` (any `cluster.mode`) and by none in role `all` with `cluster.mode = single`, and rows of a node that stopped refreshing are ignored after `cluster.presence_ttl`. Role `all` with `cluster.mode = multi` writes the table like role `api`.
16. The worker's `presence.View` reads `gateway_presence` through a reader of `gateway`, the owner of the table (03 principle 12, ADR 0050); with it the decision yields `gateway` and `push_delayed` exactly as in role `all`; `notification.created` reaches the node that holds the connection through `NOTIFY ephemeral` with IDs, and that node sends the hint after loading it.
17. All nodes use one signing key store; a key rotated on one node is accepted by every node within `signing_key.cache_ttl`.

## Scope

Server module (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- Migrations: `agent_consumer_cursors.holder_session_id` if M2-07 did not create it, and the table `gateway_presence` exactly as 03 (R-DM-51), with the grants of 16 7.1 rule 3.
- `internal/gateway`: publisher and listener of `ephemeral` for every row of 02 11.6; `4017` across nodes; the writer of `gateway_presence` (Design note 2); the receiver of `notification.created` by ID.
- `internal/agents` and `internal/runs`: fencing with `holder_session_id` and the `4015` close.
- `internal/gateway`: the reader of `gateway_presence` behind a port; `gateway_presence` has the retention class `transient`: where the retention coverage test of M7-05 exists on the base branch, register a participant that deletes rows older than `cluster.presence_ttl`.
- `internal/notify`: the table-backed `presence.View` for the worker uses the port of `gateway`; `LiveRoutes` in role `worker` sends through `NOTIFY ephemeral` instead of the fail-closed default of M3-23.
- `internal/wire`: role and mode wiring (Design note 3).
- Signing keys: `secrets.signing_key_dir` per target documented (shared volume in Compose); a test with two processes on one key store.
- Configuration: `cluster.mode` becomes active; new keys `cluster.presence_flush_interval` and `cluster.presence_ttl` (02 and the reference).
- Tests: a two-node harness (two server processes with role `api`, one `worker`, one database) and the perf test `perf/gateway_fanout_multi`.
- Docs: `docs/spec/02-architecture.md` 11.6 and 16 as built, `docs/spec/13-push-notifications.md` 12 point 4, `docs/spec/16-operations.md` (several api nodes: proxy, sticky sessions not needed, the key store, rate limits per node), `docs/RISKS.md` RK-37 (mitigation as built), `docs/operations/` guide for several nodes, READMEs, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-17-<n>`.

## Out of scope

- Level 3 and 4 of 02 16 (several workers already work through River; read replicas); shared rate limit counters (02 16: per node by design).
- The terminal relay across nodes (M2-47).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-17-multi-node`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(gateway): ...`, `feat(notify): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Time only through `kernel/clock` in unit tests; the two-node tests use real time only for the latency measurement against `perf.fanout_cross_node_p99`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Ephemeral channel** (02 11.6). One `LISTEN` connection per node; payloads are `<kind>:<id>[:<version>]`; the receiver loads whatever it needs from the database (run document, notification, device session) and hands it to its hub; its own publications are ignored by a node ID in the payload. Payload size and rate stay within the `NOTIFY` limits; a test sends the maximum burst of typing events of `sync.typing_min_interval`.

2. **Presence writer** (R-DM-51, 13 4.4; Ruling for docs/DECISIONS.md). On a connection entering `live`, on focus or idle changes and on close the node upserts or deletes its row in batches at most every `cluster.presence_flush_interval`, and at the same interval upserts the rows of all its live connections with a fresh `updated_at` (an upsert, not an update, so rows lost with the `UNLOGGED` table after a database crash come back within one interval; a test clears the table and expects the rows back); on a clean shutdown it deletes its rows; the reader ignores rows older than `cluster.presence_ttl`. A failed write is logged and counted and never blocks a connection.

3. **Wiring** (M3-23 Design note 1). Role `all` with `single`: in-memory `presence.View`, no table writes, no `NOTIFY`; role `api`: in-memory view for its own decisions plus the table writer; role `worker`: table-backed view and `NOTIFY`-based live routes; `multi` turns on the ephemeral channel for every role.

4. **Rulings for `docs/DECISIONS.md`** (`D-M7-17-<n>`): payload format (note 1), presence writes and TTL (note 2), wiring (note 3), the shared key store per target.

## Tests (write them first)

Unit:

- `gateway`: payload encode and decode; own publications ignored.
- `notify`: the table-backed `presence.View` returns the same entries as the in-memory one for the same connections (table test).

Integration (two `api` processes and one `worker` on one `pgtest` database):

- `TestTypingAndLiveBoxCrossNode`: typing and a run patch from a client on node A reach a client on node B; latency recorded.
- `TestConsumerFencing4015`: a second `identify` of the same consumer on node B closes node A's connection with `4015`; no event is delivered twice and no run starts twice.
- `TestDeviceReplacement4017`: a second `identify` of the same device on node B closes the older connection on node A with `4017`.
- `TestDesktopHintWithSeparateWorker`: a person with a live desktop connection on node B gets the hint through the gateway, and the mobile push is delayed exactly as in role `all`; with the desktop idle the push goes at once.
- `TestPresenceTTL`: rows of a killed node are ignored after `cluster.presence_ttl`.
- `TestSharedKeyStore`: a key rotated by node A is accepted by node B within `signing_key.cache_ttl`.

Performance:

- `perf/gateway_fanout_multi` in `make perf` against `perf.fanout_cross_node_p99`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/... ./cmd/...`
- `make perf` (the multi-node target; record the profile used)
- `make migrations-lint`, `make migrations-lock`, `make codegen-check`, `make arch`, `make register-check`, `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] Two api nodes: typing and the live box reach clients on the other node within `perf.fanout_cross_node_p99`.
- [ ] A second `identify` of the same consumer closes with `4015`; a second `identify` of the same device on another node closes the older connection with `4017`; no duplicate run execution.
- [ ] Desktop hints work with a separate worker and several api nodes through `gateway_presence` and `NOTIFY ephemeral`.
- [ ] One shared key store for all nodes is documented per target and tested.
- [ ] 02, 13, 16, RK-37, the guide, READMEs, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `gateway`, `notify`, `agents` updated in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make perf` with the measured cross-node p99.
3. Review and red team verdicts and the findings fixed.
4. The write rate of `gateway_presence` under the perf load.
5. What is not proven (behaviour behind a real load balancer, more than two api nodes).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
