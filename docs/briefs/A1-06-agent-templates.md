# A1-06: Templates and their gate

Status: ready | Depends on: A1-02 | Milestone: A1

## Goal

After this ticket the repository ships generic agent templates under `templates/agents/`, the server lists them (and private ones from `agents.templates_dir`) to owners with `listAgentTemplates`, and a template gate in `make check` and the pull request CI proves every template generic and closed without the private deny list: a failing fixture template exists for every surface of 26 5.5 and every rule of 26 7.3. The templates are written for the repository from scratch; nothing in them names an agent, profile, company, host, person, ticket or internal script of a real installation.

## Read first

- `docs/spec/26-builtin-agents.md`: 1, 4.2, 4.3, 5.5, 7 (all), 9, 12 (BA-T17, BA-T18), 13 (R-BA-10), 14 (`agents.templates_dir`).
- `docs/spec/07-permissions.md` 2.13 (`agent.templates.read`); `docs/spec/05-actions.md` R-AZ-36.
- `AGENTS.md` ("This repository is public"), `CONTRIBUTING.md` (example values), [ADR 0042](../adr/0042-deny-list-scanner-with-the-list-from-the-ops-repo.md), `docs/quality/gates.md`, `tools/denylist/` (the scanner and `--allow-missing`).
- ADR 0076; design note D-A1-14 to D-A1-16; `docs/decisions.d/A1-01.md`.
- The corpus `tests/agent-definitions/` and the Go validator of A1-02 (`internal/agents/domain/definition`).
- Code on the base branch: `Makefile` (targets `check`, `gates`, `denylist`), `.github/workflows/ci.yml` (the job plan), `services/server/internal/agents`, `services/server/internal/wire`, `docs/api/openapi.yaml`.

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

12. `templates/agents/<slug>/` holds exactly `template.yaml`, `CLAUDE.md` and `skills/<name>/SKILL.md` per skill; `template.yaml` follows `agent_template.v1` (26 7.1).
13. `make templates-check` fails on every rule of 26 7.3, runs in `make check` and in the pull request CI, and needs no private deny list; its tests hold one failing fixture template per surface of 26 5.5 and per rule (BA-T17).
14. The shipped templates are generic (topics of 26 7.5), pass the gate, the corpus rules of 26 4.3 and the private deny list.
15. The server embeds a copy made by `go generate` and held by `make codegen-check`; `listAgentTemplates` (owner) returns shipped and private templates; a private template with a taken slug or an invalid schema is skipped with `template_slug_taken` or `template_invalid` (26 7.2, BA-T18).
16. Before the merge the owner has extended the private deny list and its CI secret (D-A1-16); the pull request stays open until then.

## Scope

Create:

- `templates/agents/<slug>/` for each topic of 26 7.5 (a writing helper, a code reviewer working in a folder, a research summarizer, a meeting notes helper), each with `template.yaml`, `CLAUDE.md` and the skills it needs; MCP servers in templates only as `http` with an `https://example.com/...` URL and `secret_ref` names, or none; `README.md` in `templates/agents/` on the layout, the rules and how a private folder works.
- `tools/templates/template_gate.py` with tests under `tools/templates/tests/` and failing fixtures under `tools/templates/tests/fixtures/<rule>/`; it reuses the rules of `tests/agent-definitions/` through the same corpus format (one shared check of 26 4.3 in Python may live in `tools/templates/definition_rules.py` and be tested against the whole corpus).
- `Makefile` target `templates-check` (in `gates`, so `make check` runs it) and a job or step in `.github/workflows/ci.yml` that runs it for every pull request that touches `templates/`, `tools/templates/` or the corpus, following the plan of affected parts that `ci.yml` uses.
- `services/server/internal/agents/templates/` (the generated copy and its `go:generate` directive), the loader of shipped and private templates, `listAgentTemplates` with `agent.templates.read`.
- `docs/decisions.d/A1-06.md`.

Change:

- `docs/api/openapi.yaml` (`listAgentTemplates`, schema `AgentTemplate`), `services/server/api/operations.yaml`, `docs/spec/actions.yaml` (`agent.templates.read`, columns of 07 2.13), `make catalog` and every generated copy of `make codegen-check`.
- `services/server/internal/wire`: the key `agents.templates_dir`; `make config-reference`.
- `docs/briefs/README.md` table "Make targets created by tickets" already names `templates-check` for this ticket; `docs/quality/gates.md` gets the gate.
- `README.md` of `services/server/internal/agents`.

## Out of scope

- The interview's use of the catalogue (A1-07) and the clients (A1-08).
- Exporting or converting an existing agent into a template, ever (26 7.5).
- Any template that copies, paraphrases or describes a real installation's agent.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-06-agent-templates`; one writer per worktree.
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
- Never write a term of the private deny list into the repository, a test, a log or the report; never print the list. Run the private scan with `make denylist` only with the list the launcher provides.
- Templates use only the example persons Alex, Sam and Robin, the domain `example.com` and generic topics.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that the base has A1-02 (`tests/agent-definitions/` and the Go validator), else `Verdict: BLOCKED dependency A1-02 missing`.

2. **Gate rules.** In order per template, each finding as `templates: <path>: <rule>: <detail>` without printing a matched private term: files outside the layout; schema of `template.yaml`; frontmatter keys of each `SKILL.md`; the template converted to a definition draft and checked with the rules of 26 4.3; hosts, URLs and e-mail addresses outside `example.com`, `example.org`, `example.net` and their subdomains; absolute paths and `~` paths; IPv4 and IPv6 addresses; names of people outside Alex, Sam and Robin (a short list of common first names is not a proof; check capitalized words in a person position such as "for <Name>" and the frontmatter author fields, and rely on review and the private list for the rest); secret-looking values (the token patterns of `.gitleaks.toml`); then `tools/denylist/denylist_scan.py --allow-missing` over `templates/`.

3. **Fixtures.** One fixture template per surface of 26 5.5 (a `.claude/` folder, `.mcp.json`, `.lsp.json`, a plugin manifest, `hooks/hooks.json`, `commands/`, `agents/`, `output-styles/`, a `SKILL.md` with `hooks`, one with `allowed-tools`, inline shell in both forms, an `@` import with an absolute path) and per rule of design note 2; the test asserts the rule name of each.

4. **Copy and loader.** `go:generate` copies `templates/agents/` into the module (Go embeds only below its module); the loader validates each template at start with the Go validator and the schema and keeps them in memory. The private folder is read at start only; a change needs a restart (say so in the README).

5. **Owner step.** The private list extension of D-A1-16 is an owner action outside this repository. The launcher passes whether it is done; if not, finish the work, open the pull request as draft and report the owner step as the only open item.

6. **Rulings for `docs/decisions.d/A1-06.md`**: the person-name heuristic and its limits; the CI trigger paths; the template topics and their tool classes.

## Tests (write them first)

- `tools/templates/tests/test_template_gate.py`: every fixture fails with its rule; every shipped template passes; the gate runs without `BOTHY_DENYLIST_FILE` set (BA-T17).
- `tools/templates/tests/test_definition_rules.py`: the whole corpus `tests/agent-definitions/` (the Python rules equal the Go validator).
- Go unit, `internal/agents`: the loader with a private folder holding a taken slug and an invalid template (BA-T18); `listAgentTemplates` as owner and refused for others (BA-T04 templates part); the generated copy equals `templates/agents/` (`make codegen-check`).

```sh
uv run --project tools --locked pytest -q tools/templates
make templates-check
make catalog && make catalog-check
make config-reference && make config-doc-check
make denylist
make check
```

## Acceptance criteria

- [ ] BA-T17 and BA-T18 are green; the shipped templates pass the gate and the private scan.
- [ ] `make templates-check` runs in `make check` and in the pull request CI.
- [ ] `listAgentTemplates` returns shipped and private templates to owners only.
- [ ] The owner step D-A1-16 is done before the merge, or the pull request is a draft with that step reported.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `templates/agents/README.md`, `docs/quality/gates.md` and the agents module README describe the templates and the gate.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: no template names anything of a real installation; every surface of 26 5.5 fails the gate; the gate works without the private list; private templates never enter the repository.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: names of people in a template that the heuristic cannot see (review and the private list hold them)), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
