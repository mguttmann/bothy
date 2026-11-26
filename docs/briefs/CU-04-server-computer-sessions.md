# CU-04: Server: computer sessions

Status: ready | Depends on: none (specification merged) | Milestone: CU

## Goal

After this ticket the server keeps computer sessions in the module `terminal`: the catalogue rows of `docs/spec/07-permissions.md` 2.15, the operations, WebSocket channels, host ops and events of `docs/spec/27-computer-use.md` 12 in the API documents and the enable list, the migrations for `computer_sessions` and the `hosts` columns, guard rule SR13, the attach token, the relay pairing of `computerAttach` and `hostComputerData`, the enablement mirror with the remote disable, the binding to the run and every revocation in the transaction that causes it, the audit of 27 13.1, and the sentinel test that finds no screenshot byte on the server.

## Read first

- `docs/spec/27-computer-use.md` sections 4, 5, 8, 12, 13, 14, 15 (CU-T01 to CU-T06, CU-T16 to CU-T18), 18; `docs/spec/07-permissions.md` 2.15, SR2, SR7, SR9, SR13, R-RE-14; `docs/spec/05-actions.md` R-AZ-38 and section 5 "Check without a request"; `docs/spec/03-data-model.md` 4.14 (`hosts`, `computer_sessions`), R-DM-65.
- `docs/spec/24-terminal.md` 5.3, 6.1, 8.3, 9 (several nodes), 10; `docs/spec/09-agents.md` 4.5 (invocation token), 5.1 (end states).
- Code on the base branch: `services/server/internal/terminal` (`app/sessions_open.go`, `app/token.go`, `app/revocation.go`, `adapters/wsrelay/relay.go`, `control.go`, `hostdata.go`, `stream.go`, `domain/token.go`), `internal/authz/domain/terminal.go`, `internal/runs` (where a run reaches an end state), `internal/wire/retention_coverage_test.go`, `services/server/migrations`, `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `services/server/api/operations.yaml`, `docs/spec/actions.yaml`, `services/server/architecture/modules.yaml`.
- Design note `docs/design/computer-use.md` (context and evidence; where it differs from the chapters, the chapters win); ADR 0078; `docs/decisions.d/CU-01.md`; `docs/security/threat-model.md` 5.18 and 6.10.

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
8. White label: examples, templates and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no internal script names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it.
11. Additive only: every new key defaults to off or to the behaviour before, and an installation that sets none of the new keys (`runner.computer_use`, `host_service.computer_use`) and enables no helper behaves exactly as before.
12. Full operation (owner ruling OR-1, D-CU-01-1): build no app tier, no key or app block list, no confirmation per session or action and no lock on the Bothy windows; the only protections are those of `docs/spec/27-computer-use.md` section 1 item 3.

Note on rule 1: "stages" are the milestones and the stages A1, AR and CU of the plan (PLAN P-27, P-29, P-30).

Product requirements implemented here:

13. Catalogue: the seven actions of 07 2.15 and the event `computer_session.changed` in `docs/spec/actions.yaml` with exactly those columns, then `make catalog`, `make event-catalog` and every generated copy; the actions and event in `modules.yaml` under `terminal`.
14. API documents: the five operations of 27 12.2 in `openapi.yaml` (with `x-authz-action`, the rate-limit class and the `Invocation-Token` header where named), the channels `computerAttach` and `hostComputerData` and the host ops of 27 12.3 in `asyncapi.yaml`, the operations under `enabled` in `operations.yaml`, `make contract-documents`; refusals only with registered problem codes and the reasons of 27 12.4.
15. Migrations (expand only): `computer_sessions` and the `hosts` columns `reported_computer` and `computer_disabled_generation` exactly as 03 says, with their checks and unique indexes; migration tests column by column; existing rows unchanged.
16. SR13 in `authz` at `computer.desktops.list`, `computer.open` and `computer.attach` with the order of 27 5.1 step 5 and the reasons `computer_not_owner_run`, `computer_tool_class`, `computer_not_enabled`, all five items including `run_feeders` (item 5) and the `command` form of item 2 (`triggering_command_id` invoked by `triggered_by`); a follow-up of any other person is not delivered into a run with a non-closed session (it stays `queued`, then `deferred`, CU-T25); SR9's run binding with `run_inactive`; `404` before `403` (R-AZ-01); every denial audited (R-AZ-04).
17. Open in one transaction (row, audit, event), then `computer.open` to the host after the commit; the attach token of 27 5.2 signed with a key of purpose `terminal`; `computer.open_timeout` as in 24 6.1 step 6; `computer.attach` once per session with the checks of 27 5.4; pairing in the relay and across nodes as for terminal streams (24 9).
18. Enablement: `report.computer` validated and stored in `hosts.reported_computer` (with `locked`; the doctor list of `report` stays HD1 to HD11), a change audited under `host.connect`; the listed state of 27 4 item 3; `computer.device.disable` sets `computer_disabled_generation`, ends the session (`device_disabled`) and sends `computer.disable`; `host.updated` carries both.
19. Ends: `computer.state` updates `action_count` and `front_app` and the event; in the transaction that ends a run with a computer session the thread's current agent session ends with `end_reason = 'computer_use'` (03, a migration widens the check) and `agent_session.changed` is written; one end function for computer sessions like the terminal's (M2-46): `computer.stop`, `computer.ended` from the host, `run_ended` from the module `runs` in the transaction that ends the run, `access_revoked` from `authorize.AccessChanged` and `identity.SessionEnded` with `user_disabled`, host state changes, lost connections; the first committed end wins; `computer.terminate` audited with the details of 27 13.1; `close_detail` only from the list of 27 8.3 and only with `open_failed`, `stopped_by_user` and `error`.
20. The table `computer_sessions` gets its retention participant (class `security`) and the entry `ticket:CU-04` leaves `handledElsewhere` in `internal/wire/retention_coverage_test.go`.
21. The relay counts only lengths and times; no frame content enters a log, metric, audit entry or event (sentinel test CU-T18 with a known byte pattern through the relay, then a scan of every table, the captured logs and the telemetry export of the test).
22. Server configuration keys `computer.*` of 27 section 18 in the server schema; `tools/configdoc/planned.yaml` loses the rule `computer.` once they are in it; `make config-reference`.
## Scope

All paths under `services/server/` unless they start with `docs/` or `tools/`.

Create:

- Migrations `<next number>_computer_sessions.sql` and `<next number>_hosts_computer.sql` with tests.
- In `internal/terminal`: the computer session use cases (open, attach, stop, disable, end, read), the attach token, the enablement mirror, their adapters (HTTP, relay, control ops, postgres) and tests; integration tests under `test/integration/terminal/`.
- `docs/decisions.d/CU-04.md`.

Change:

- `internal/authz` (SR13 and the new actions), `internal/runs` (the end hook into `terminal` through a port, the follow-up rule of SR13 item 5), `internal/agents` (the session end with `computer_use`), `internal/terminal/README.md`, `internal/wire` (schema keys, retention), `architecture/modules.yaml`, `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `api/operations.yaml`, `docs/spec/actions.yaml` with `make catalog`, `make event-catalog`, `make contract-documents` and every generated copy of `make codegen-check`, `tools/configdoc/planned.yaml`, `docs/operations/configuration.md`, `docs/observability/signals.md` (relay counters of computer sessions without content), `docs/spec/27-computer-use.md` where the code needs a precise name.

## Out of scope

- The host service (CU-03), the helpers (CU-02), the MCP server (CU-05), the runner (CU-06), the clients (CU-07).
- A grant or capability for computer use (D-CU-01-23); any change to terminal sessions.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/CU-04-server-computer-sessions`, created from the branch of the stage that holds CU-01; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers.
- Do not push and do not open a pull request: the stage lands as one pull request with CU-01 to CU-07 and one CI round (PLAN P-30, `AGENTS.md`); the coordinator merges your branch into the stage branch. Never push to `main`, never tag, never create remote repositories.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules); no GUI or input test on a maintainer's machine and no run with a real model; native GUI tests run only in CI jobs.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers. Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, attach tokens, DPoP keys, signing material and screenshots never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money; a check of the pinned CLI runs only without a login against a local stand-in for the API on `127.0.0.1`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/27-computer-use.md`, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm `docs/spec/27-computer-use.md` section 12 and `computer_sessions` in 03, else `Verdict: BLOCKED specification missing`.
2. **Module boundary.** Computer sessions live in `terminal` (D-CU-01-15). The end of a run reaches them through a port that `runs` calls in its own transaction (the pattern of the revocation subscriptions); `terminal` never imports `runs` internals.
3. **SR13 data.** The run's `trigger_kind`, `triggered_by`, the trigger message's `origin` and author, and the run's tool class come from `runs` through a read port in the same transaction as the check; the owner status from the current owner list (R-AZ-29).
4. **Host ops in parallel with CU-03.** The JSON member names of 27 12.3 are the contract; a test decodes the examples of the asyncapi document with the Go types.
5. **Rulings for `docs/decisions.d/CU-04.md`:** migration numbers, the port names, the job interval of any sweep you need for `opening` rows, every name that differs from chapter 27.

## Tests (write them first)

- Go unit and integration tests (container runtime): CU-T01 to CU-T06, CU-T25 with one test per refusal of SR13 and SR9; CU-T16 server part, one test per end path with the reason, the relay closed, `computer.close` sent and the audit; CU-T17; the attach token claims; the enablement mirror and the disabled generation; the migrations; the sentinel CU-T18.
- The catalogue, enable-list and contract tests stay green with the new rows (`TestEnableListComplete`, `TestAZP1_RoutesNeedKnownActionAndClass`, the retention coverage).

```sh
(cd services/server && go test -race -count=1 ./internal/terminal/... ./internal/authz/... ./internal/runs/... ./internal/wire/... ./migrations/...)
make catalog && make catalog-check
make config-reference && make config-doc-check
make codegen-check
make check
```

## Acceptance criteria

- [ ] Every refusal and end path of 27 is tested; CU-T01 to CU-T06, CU-T16 to CU-T18 (server parts) and CU-T25 are green.
- [ ] Catalogue, API documents, enable list, configuration reference and every generated copy are current.
- [ ] Existing hosts and terminal sessions are unchanged by the migrations (a test with rows from before).
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- An entry `changelog.d/CU-04.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/CU-04.md`, one row each with date, ruling and reason; every name that differs from chapter 27 is a ruling and a change of chapter 27 in the same commit.
- An ADR only if you take an architecture decision beyond ADR 0078 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green for the components you changed.
- An independent review of your diff by a fresh reviewer (one follow-up round at most); record its verdict. The red team of the stage runs once over the whole stage diff before the pull request; it checks at least, for this ticket: no run other than the owner's own `host` run lists, opens or attaches; the attach token is single-use and bound; a remote disable holds; every revocation and the end of the run end the session in the same transaction; no screenshot byte reaches a table, log, audit entry, event or telemetry; terminal sessions behave as before.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. The review verdict and the findings fixed.
4. What is not proven (at least: the path with the real host service, MCP server and runner (CU-06)), each probe of `docs/spec/27-computer-use.md` section 16 you ran with its result, and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
