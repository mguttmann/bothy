# A1-07: Interview

Status: ready | Depends on: A1-02 | Milestone: A1

## Goal

After this ticket an owner can let a builder agent interview them for a new agent. `interviewAgent` sends the interview so far to the agent named in `agents.builder_agent_id` as the ephemeral event `agent.interview_requested`, waits up to `agents.interview.timeout`, and returns either the builder's next questions or a draft definition that the server has validated with the rules of 26 4.3, plus the cost estimate; nothing of the interview is stored, and every request writes one audit entry and one log line with counts only. The Claude Code runner answers as the builder behind `runner.agent_builder` with one model call without tools. Without a builder, or when it is offline, slow or wrong, the answer names the fallback and the apps offer templates and the manual form.

## Read first

- `docs/spec/26-builtin-agents.md`: 1, 4.2, 4.3, 6 (all), 7.4, 9, 12 (BA-T15), 13 (R-BA-09), 14 (`agents.builder_agent_id`, `agents.interview.*`, `runner.agent_builder*`).
- `docs/spec/09-agents.md` 10.6 and 18.17 (the search assistant: the pattern this ticket copies), 4.10 (`CostReport`), 18.2 (`chat` flags, environment), 18.3.
- `docs/spec/14-search.md` 4.10 (the assistant's request and answer rules), `docs/decisions.d/F-INTELLIGENTE-SUCHE.md` (D-IS-3, D-IS-8, D-IS-9, D-IS-10, D-IS-15).
- `docs/spec/07-permissions.md` 2.13 (`agent.interview`, `agent.interview.respond`, "view as"); `docs/spec/05-actions.md` R-AZ-36; `docs/spec/02-architecture.md` 14.2 (the deadline row); `docs/spec/06-api.md` R-API-29.
- ADR 0075, ADR 0076, ADR 0054 (data, never instructions); design note D-A1-10 to D-A1-13.
- The Go validator and the corpus of A1-02; the catalogue part of 26 7.4 (templates come with A1-06; until then the catalogue is empty).
- Code on the base branch: `services/server/internal/search` (`assist.go`, `app/assist.go`, `domain/assist.go`), `services/server/internal/gateway/app/assist.go`, `services/server/internal/wire` (search wiring and `configchecks.go`), `plugins/claude-code-runner/src/claude_code_runner/searchassist.py` and `prompts/search_assist.md`, `sdk/python/src/bothy_sdk/search_assist.py` and `adapter.py`.

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
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it: the catalogue rows of `docs/spec/actions.yaml` that R-AZ-36 assigns to this ticket and their generated copies (`make catalog`).
11. Additive only (26 section 11): every new column is nullable or defaults to "as before", every key defaults to off, and an installation that sets none of the new keys behaves exactly as before.

Note on rule 1: "stages" are the milestones and the stage A1 of the plan (PLAN P-27).

Product requirements implemented here:

12. `interviewAgent` is owner-only, class `rl.trigger`, effect `write` (it writes an audit entry, so the maintenance mode refuses it, 05 2.7), counts as writing for "view as", carries at most `agents.interview.max_turns` turns of at most `agents.interview.answer_max_chars` characters, and uses `agents.interview.timeout` as its own deadline (26 6.1, 02 14.2).
13. The event `agent.interview_requested` goes to one live gateway connection of the builder agent without a token; `answerAgentInterview` is accepted only from that agent and only once per request, before `expires_at`.
14. A draft is validated with the Go validator of A1-02; a draft that fails is answered with its field errors; the builder's answer schema has no member for any surface of 26 5.5, and a builder answer that carries one is refused (BA-T15).
15. Fallbacks `builder_not_configured`, `builder_offline`, `builder_timeout`, `builder_invalid` are named in the answer; nothing of the interview is stored; one audit entry `agent.interview` and one log line per request with outcome, fallback reason, turn count and cost estimate, never content (R-BA-09).
16. The runner answers only with `runner.agent_builder` on and only as the agent of `agents.builder_agent_id`: one `claude -p` with the `chat` flags, `--output-format json`, `runner.agent_builder_model`, `runner.agent_builder_timeout`, the billing rule of 09 18.17 item 1, a private folder removed afterwards, no session; it checks its own answer against the corpus rules before sending (26 6.2).

## Scope

Server (paths under `services/server/`):

- `internal/agents/app/interview.go` (or a module the register allows, following the pattern of `internal/search` for the relay): the request, the relay through `gateway` like `search.AssistRelay`, the wait with its own deadline, the answer endpoint, validation, fallbacks, audit and log line; `internal/agents/domain/interview/` for the pure answer and draft rules.
- `internal/wire`: the keys `agents.builder_agent_id`, `agents.interview.timeout`, `agents.interview.max_turns`, `agents.interview.answer_max_chars`; `make config-reference`. No cross-check against `http.request_timeout`, because the route has its own deadline; the runner README says that `runner.agent_builder_timeout` must stay below `agents.interview.timeout`, which the runner cannot read (the rule `runner.kill_grace` already follows in 09 21).
- `docs/api/openapi.yaml` (`interviewAgent`, `answerAgentInterview`, schemas `AgentInterviewRequest`, `AgentInterviewAnswer`, `AgentDraft`), `docs/api/asyncapi.yaml` (`agent.interview_requested`), `services/server/api/operations.yaml`, `docs/spec/actions.yaml` (`agent.interview`, `agent.interview.respond`, the event), `make catalog`, `make event-catalog`, `make contract-documents` and every generated copy of `make codegen-check`.
- `services/server/architecture/modules.yaml`, module READMEs, `docs/spec/02-architecture.md` 14.2 only if the deadline mechanism differs from the row.

Runner and SDK:

- `plugins/claude-code-runner/src/claude_code_runner/builder.py`, `prompts/agent_builder.md`, `config.py` (`runner.agent_builder`, `runner.agent_builder_model`, `runner.agent_builder_timeout`), `config.example.toml`, README; tests in `tests/unit/test_builder.py` and `tests/scenarios/test_agent_builder.py`; `tests/fake_claude` learns the builder call.
- `sdk/python/src/bothy_sdk/agent_interview.py` and the adapter hook, like `search_assist.py`, with tests and a changelog fragment.
- `docs/decisions.d/A1-07.md`.

## Out of scope

- The dialog in the apps (A1-08); storing interviews (never, 26 6.1 item 3); a run, thread or session for an interview.
- Tools for the builder (never, D-A1-13).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-07-agent-interview`; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers; squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, DPoP keys, secret values and secret references resolved to values never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money, except a test behind the explicit switch of the runner's real CLI tests when this brief names one and the launcher allows it.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/26-builtin-agents.md`, stop with `Verdict: BLOCKED <reason>`.
- The builder prompt and its tests use only generic example answers (Alex wants a writing helper for `example.com` texts); no real installation appears.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has A1-02 (the Go validator and `tests/agent-definitions/`), else `Verdict: BLOCKED dependency A1-02 missing`.

2. **Shape.** Request: `turns[]` of `{role: "builder" | "owner", text}` in order, optional `want_draft` (the person asked for a draft early). Event payload: `request_id`, `turns`, `catalogue` (26 7.4), `limits` (the definition limits the draft must keep), `expires_at`. Answer: `kind` `questions` with one or two `questions[]`, or `kind` `draft` with `draft` (the definition shape plus `name`, `handle`, `description`, `channel_suggestion` with a space name and channel name), and `costs` (`CostReport`).

3. **Validation.** The server decodes the answer strictly: an unknown member anywhere is `builder_invalid`; a draft runs through the Go validator with the agent's prospective class from the draft's `tool_class_wish` (the creation flow sets that class before it stores the definition, 26 8.2, so a draft valid here is stored); the response carries the draft and its field errors so the person can ask the builder to fix them, and never stores anything. An MCP server of the draft that is not in the catalogue sent with the request (26 7.4) is the field error `mcp_not_in_catalogue` at its JSON Pointer (an `http` server matches in transport and URL, a `stdio` server in transport, command, package, every argument and the set of environment names, since an argument can carry an endpoint; a test changes only the endpoint argument of a catalogue `stdio` server and gets the error), so the prompt is not the only guard.

4. **Deadline.** The route uses `agents.interview.timeout` through the response controller like uploads use `http.transfer_timeout`; the console's backend for frontend passes the request with the same deadline. A builder answer after the deadline is refused the way a late search assistant answer is (14 4.10) and changes nothing.

5. **Builder prompt.** A fixed instruction: ask only what the draft needs (26 6.3), one or two questions per turn, never ask for a secret value, suggest MCP servers only from the catalogue, say that a `stdio` server makes the agent `host`, output exactly one JSON object of the answer schema. The turns and the catalogue follow in a data block marked as untrusted content (ADR 0054).

6. **Runner check.** The runner parses the last JSON object of the reply, checks it against the answer schema and the Python port of 26 4.3 in `src/claude_code_runner/definition.py`, and sends nothing for an answer that fails. That module is the one place of the rules in the runner: if the base already has it, use and extend it; otherwise create it here, tested against the whole corpus `tests/agent-definitions/`. Never write a second copy.

7. **Audit.** Action `agent.interview`, outcome (`questions`, `draft`, `fallback`), fallback reason, turn count, draft field error count, cost estimate in micros; the log line has the same fields. A canary string in an owner answer appears in no log, audit entry, event other than `agent.interview_requested`, or telemetry.

8. **Rulings for `docs/decisions.d/A1-07.md`**: the request and answer shapes, the deadline mechanism, the builder model default.

## Tests (write them first)

- Go unit and integration (container runtime) for the server (BA-T15): a question turn, a draft turn, a draft with field errors, a draft with an MCP server outside the catalogue (field error), a builder answer carrying each surface of 26 5.5 (refused), every fallback, a late answer, an answer from another agent (`404`), a second answer (`409`), a non-owner (`404` or `403` as 07 2.13 says), "view as" refused, `503 maintenance` in maintenance mode (effect `write`); one audit entry and one log line per request without content (canary).
- Runner unit and scenario tests with fake Claude: the command line of the builder call, the billing choice, the private folder removed, an invalid reply sends nothing, a reply with a surface is not sent, the time limit.
- SDK tests of the hook.

```sh
(cd services/server && go test -race -count=1 ./internal/agents/... ./internal/gateway/...)
(cd plugins/claude-code-runner && uv run --locked pytest -q)
make catalog && make catalog-check
make config-reference && make config-doc-check
make check
```

## Acceptance criteria

- [ ] BA-T15 is green on the server and in the runner.
- [ ] Nothing of an interview is stored; audit and log carry counts only.
- [ ] Every fallback is named in the answer.
- [ ] The catalogue, the API documents, the configuration reference and every generated copy are current; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of the agents and gateway modules, the runner and the SDK describe the interview.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: no builder answer can carry a surface of 26 5.5 into a stored definition; only owners interview; nothing is stored; no content in audit or logs; the builder holds no tool.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the quality of real builder answers (no real model call in the gate)), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
