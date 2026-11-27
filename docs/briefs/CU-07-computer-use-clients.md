# CU-07: Clients: status, stop and desktops

Status: ready | Depends on: CU-04 | Milestone: CU

## Goal

After this ticket the apps show computer use: a status chip in the thread of a run whose computer session is not closed, with the desktop, the frontmost app, the action count, the paused state and a stop button; the list of desktops in the host settings with the enablement state and "Switch enablement off"; a tray entry of the desktop app while a session of the owner is live, with stop; desktop and phone as `docs/spec/01-product.md` 3.9 names them, and the console where it shows hosts. No view ever requests, caches or shows a screenshot.

## Read first

- `docs/spec/27-computer-use.md` sections 4, 8.1 to 8.3, 11, 12.2, 12.4; `docs/spec/01-product.md` 3.9; `docs/spec/15-client-architecture.md` R-CL-17 and R-CL-18; `docs/spec/07-permissions.md` 2.15.
- Code on the base branch after CU-04: `apps/client/lib/terminal/` (host list and settings as the model), `apps/client/lib/desktop/tray.dart`, the thread view and its run chips, `packages/admin_ui` (host settings of the console), the generated Dart client and `packages/sync_engine` (fake server and event catalogue).
- Design note `docs/design/computer-use.md` (context and evidence; where it differs from the chapters, the chapters win); ADR 0079; `docs/decisions.d/CU-01.md`; `docs/security/threat-model.md` 5.18 and 6.10.

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

13. Status chip from `computer_session.changed` and `listComputerSessions`: "Controls <desktop> · <frontmost app> · <n> actions" or "<desktop> locked, waiting", with stop (`computer.stop`); it disappears when the session closes and names the close reason once ("Stopped", "Enablement switched off", "Run ended", ...); English and German strings through the ARB files.
14. Desktops in the host settings (desktop app and console): per host the reported enablement, the operating system account, the permission state, and "Switch enablement off" (`computer.device.disable`) with a short explanation that switching on happens only at the machine; no switch to turn it on.
15. Tray of the desktop app (macOS and Windows): an entry while a session of the owner is live, with stop; gone when no session is live.
16. R-CL-18: no screenshot, no cache of status data beyond the local database rows the sync engine keeps for events, app identities not in crash reports or metrics; accessibility labels and keyboard operation for chip, stop and the switch-off.
17. Owner only: the views show nothing to a person who is not an owner; the server sends the computer state of a host only to owners (07 2.15 item 5) and refuses the actions.
## Scope

Create:

- `apps/client/lib/computer/` (chip, session repository, desktop list section) with widget tests; the console part in `packages/admin_ui` with tests.
- `docs/decisions.d/CU-07.md`.

Change:

- The thread view (chip slot), the host settings, `apps/client/lib/desktop/tray.dart`, the ARB files, the Dart fake server of `packages/sync_engine` (the computer operations and event, generated where the contract copy is generated), the README of each changed package, `docs/spec/01-product.md` 3.9 only if a name changes.

## Out of scope

- A live view or screenshots (CU-10, F3); computer use for persons other than the owner (CU-12).
- Server, host, MCP server and runner (CU-02 to CU-06).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/CU-07-computer-use-clients`, created from the branch of the stage that holds CU-01; one writer per worktree.
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

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that CU-04 is merged into the stage branch (the operations are in the generated Dart client), else `Verdict: BLOCKED dependency CU-04 not merged`.
2. **Follow the terminal views** for layout, wording style and the console's host settings; the design tokens and components of `docs/design/` apply.
3. **Rulings for `docs/decisions.d/CU-07.md`:** strings, the chip's placement per platform, every name that differs from chapter 27.

## Tests (write them first)

- Widget tests: chip states (opening, active, paused, closed with each reason), stop, desktop list with and without enablement, switch-off with its refusal, tray entry on and off; golden images only where the package already uses them.
- Integration tests against the Dart fake server on desktop and phone; console tests in `packages/admin_ui`.

```sh
(cd apps/client && flutter test --concurrency=2 test/computer)
(cd packages/admin_ui && flutter test --concurrency=2)
make check
```

## Acceptance criteria

- [ ] Every chip state, stop, switch-off and the tray entry are tested on desktop and phone; the console shows the enablement and the switch-off.
- [ ] No view requests or shows an image (a test asserts that the views never call an image or file operation for a computer session).
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- An entry `changelog.d/CU-07.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/CU-07.md`, one row each with date, ruling and reason; every name that differs from chapter 27 is a ruling and a change of chapter 27 in the same commit.
- An ADR only if you take an architecture decision beyond ADR 0079 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green for the components you changed.
- An independent review of your diff by a fresh reviewer (one follow-up round at most); record its verdict. The red team of the stage runs once over the whole stage diff before the pull request; it checks at least, for this ticket: no view shows or caches a screenshot; no switch turns an enablement on; stop and switch-off reach the API from every platform; non-owners see nothing.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. The review verdict and the findings fixed.
4. What is not proven (at least: the views against a real server with a real desktop (the stage's end-to-end test covers the server side)), each probe of `docs/spec/27-computer-use.md` section 16 you ran with its result, and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
