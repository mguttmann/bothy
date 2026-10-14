# M8-09: Carry out the migration of an existing installation (owner go)

Status: ready | Depends on: M8-07, M8-08 | Milestone: M8

## Goal

After this ticket the migration of 22 13 is ready to be carried out on a real installation, and everything that can be proven without a real Discord server is proven. The product repository gets the generic, white-label migration guide `docs/operations/discord-migration.md` (prerequisites, the separate read-only bot, login of the tool, dry run until no abort, pilot, parallel operation with spot checks, switch-over channel by channel, revert, completion, retention of `import_items`, what to watch on the status page) and a rehearsal: one test that walks the whole flow of 22 13 over the full fake Discord fixture against the real server and the console steps through the API, and checks the exit criteria of PLAN 14 item 2 on it: a dry run over the whole source without aborts, pilot channels switched over, parallel operation with spot checks of the check report against the source, a test trigger per channel, at the end every channel with `lead = bothy` and the source `completed`. The live migration itself is an owner step: it uses a real Discord bot token and a real installation, so this ticket never runs it; it writes the owner's decision sheet into its report, and the installation-specific runbook (mapping file, session references, dates) stays outside the product repository (PLAN 14 "Execution flow"). Why: exit criterion 2 of M8 needs a real execution, and the owner must be able to start it with one decision, knowing what it costs, what it proves and what can go wrong (owner rule on irreversible or external steps).

## Read first

- `docs/PLAN.md` section 14 (Goal, Exit criteria 2, Execution flow, Red team, Risks) and P-15.
- `docs/spec/22-import-migration.md`: all of it, in particular 4.2 (separate bot, permissions, intent), 10 (commands), 11 (report), 12, 13 (13.1 to 13.5), 14, 16.
- `docs/spec/20-privacy.md` (retention class `import`, placeholders as personal data).
- `docs/operations/install-compose.md`, `backup-restore.md`, `upgrade.md` (the guide links to the backup before the first `--apply`).
- `docs/RISKS.md`: RK-73 to RK-76.
- `docs/DECISIONS.md`: D-IM-01 to D-IM-08, D-W-100, D-W-120, D-W-124, D-W-126; rows `D-M8-0<n>-<m>` of M8-01 to M8-08, M8-10 and M8-11; section 10 format.
- Briefs (merged names win, D-W-100): `M8-04-import-tool.md`, `M8-05-read-mirror.md`, `M8-06-session-binding-import.md`, `M8-07-switch-revert.md` (its integration test is the model for one channel), `M8-08-import-console.md`. The plan rows M7-14 (portability test of the released stack), M7-21 (status page in apps and console) and M5-11 (device E2E on mobile) are prerequisites of the live run on a real installation, not of this brief (D-W-126); the guide names them as such.
- Code on the base branch: `services/server/cmd/bothy/importtool`, `services/server/test/importtool`, `docs/operations/`.

This ticket touches no API operation, no table, no action, no event and no configuration key.

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
8. White label: the guide and the test data use `example.com`, the people Alex, Sam and Robin and invented Discord IDs; no company, host, guild or personal names, no paths under a home directory. Nothing instance-specific enters the repository (PLAN 14 "Execution flow").
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; the guide names the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. The guide follows 22 13 step by step, with the Discord steps of the owner, the commands of the tool and the console steps, and says for every step how to check it and how to go back (22 13.4).
12. The rehearsal proves on fakes what PLAN 14 exit criterion 2 asks of the real execution: dry run over the whole source without aborts, pilot, parallel operation with spot checks, a test trigger per channel, all channels `lead = bothy`, source `completed`.
13. No live Discord call, no real bot token and no real installation in this ticket; the live run needs the owner's explicit go (owner rule on external steps; 22 4.2).
14. No cost: the guide requires no paid plan or paid service (a Discord bot application and the bot's read permissions cost nothing; the installation is the owner's existing one).

## Scope

Create:

- `docs/operations/discord-migration.md` (Design note 1), linked from the `mkdocs.yml` navigation and from `docs/operations/install-compose.md`.
- `services/server/test/importtool/rehearsal_test.go` (note 2) and, if the fixture of M8-01 is too small for a whole-server rehearsal, `services/server/cmd/bothy/importtool/discord/testdata/guild_rehearsal.json` (several categories, channels of every mapped kind of 22 5.1 (text, announcement, forum, media, voice to be skipped, one without category), archived and active threads, bots, webhooks, busy agent outputs, attachments over and under the limit, a session references file for `--sessions`).
- Tests named under "Tests".

Change:

- `docs/spec/22-import-migration.md` 13.2 (link to the guide); `services/server/cmd/bothy/importtool/README.md` (link); root `CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M8-09-1` to `D-M8-09-3`.

## Out of scope

- The live migration of any real installation, the real Discord bot, the real mapping file and session references (owner go; installation-specific runbook outside the product repository).
- New features of the tool or the console; a gap found by the rehearsal is fixed only when it is a Blocker or a reproduced Major of a merged M8 ticket's behaviour against 22 (then in this branch with a test), otherwise reported.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-09-migration-execution`; one writer per worktree.
- Conventional Commits (for example `docs(operations): ...`, `test(import): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Never contact Discord, its CDN or any real installation; never ask for, read or store a real bot token. The rehearsal runs only against the fakes of M8-01, M8-02, M8-10 and the harness server of M1-71 with `pgtest`. Without a Docker-compatible runtime it fails; it never skips.
- Stop only processes you started, by PID. Temporary directories only via `t.TempDir()` or `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **The guide** (22 13; Ruling for docs/DECISIONS.md). Sections: (a) prerequisites: a released installation that passed the portability test of the release (PLAN row M7-14), the status page (PLAN row M7-21) as the place to announce the migration window, the mobile apps proven on devices (PLAN row M5-11), a backup and a restore test before the first `--apply`, the owner's app or console with a passkey; (b) the separate Discord bot (22 4.2): permissions `VIEW_CHANNEL`, `READ_MESSAGE_HISTORY`, `MANAGE_THREADS` only for private archived threads, intent `MESSAGE_CONTENT`, never the token of a productive agent bot, token only in `BOTHY_IMPORT_DISCORD_TOKEN` or a file readable only by the user; (c) `bothy import login` and approving the pairing code; (d) `init-map`, editing the mapping, dry runs until the report has no abort, the meaning of every report column and note; (e) creating and starting the source in the console; (f) pilot with `--channel`, `--sessions`; (g) the read mirror as a long-running process (a service or container of the installation with a restart policy, or scheduled `--once` calls; exit `3` is normal for overlapping schedules) and spot checks: counts of the report against the Discord channel, attachments, session references, `status`; (h) switch-over per channel exactly in the order of 22 13.3 with `switch --discord-locked` and the console; the probe; (i) revert per channel (22 13.4) and what happens to messages written in Bothy; (j) completion and the warning that deleting the retention class `import` ends idempotency (22 13.5); (k) troubleshooting by exit code and report note (`403`, missing intent, rate limit, `import_not_running`, lease held). The guide contains no instance values; where the owner fills something in, it shows a placeholder like `<guild_id>`.

2. **Rehearsal** (PLAN 14 exit criterion 2; Ruling for docs/DECISIONS.md: the rehearsal is the repository's proof; the live run is the owner's). One test, steps as subtests, against `harness.Start` with Robin as owner and the agent test client for the operator agents:
    a. `init-map` on the rehearsal fixture, the test completes the mapping (targets, planned operators, two people mapped, the rest placeholders); dry run over the whole source: exit `0`, no abort note, the fakes saw only `GET`.
    b. Console steps through the API (create with the mapping, start, step-up session).
    c. Pilot: `--apply --channel` for two channels with `--sessions`; report counts equal the fixture for those channels.
    d. Full import, then parallel operation: several mirror rounds while the fake Discord gets new messages (some busy, some unsettled); spot checks compare, per channel, the report counts and a sample of messages (author mapping, order, attachments or jump links) with the fixture.
    e. Switch every channel in turn: `switch --discord-locked`, console steps, and in every switched channel a test trigger in one of its imported threads that the operator answers in Bothy only, with a binding where the fixture has a reference; once revert one channel and switch it again.
    f. Complete the source; assert every channel `lead = bothy`, the source `completed`, the fake Discord saw only `GET` across the whole test, and no message was copied twice.

3. **Decision sheet in the report** (owner rule on external steps). For the live run: what it costs (no money: a free Discord bot application; owner time; a maintenance window per channel), what it proves (exit criterion 2 on real data), what can go wrong and how it is contained (missing intent: preflight abort; rate limits and IP blocks: the guard and the separate bot, RK-76; half-finished outputs: settle rule, RK-74; double booked costs: baseline, RK-75; a wrong mapping: dry run and revert), the order of steps, and the recommendation (start with a dry run on the real server and one pilot channel). The owner decides; this ticket ends with that sheet.

## Tests (write them first)

- `services/server/test/importtool/rehearsal_test.go`: the steps of Design note 2, red first (the guide's commands are executed literally from a list in the test, so the guide and the rehearsal cannot drift: the test reads the fenced command blocks of `docs/operations/discord-migration.md` marked `rehearsal` and runs them with the placeholders replaced by test values).
- `tools/docs` checks stay green on the new guide (`make docs`, `make linkcheck`).

Commands that must pass, from the repository root:

- `cd services/server && go test ./test/importtool/... -race -count=1 -run Rehearsal`
- `make docs`, `make linkcheck`, `make lint`
- `make check`

## Acceptance criteria

- [ ] The rehearsal passes: dry run without aborts, pilot, parallel operation with spot checks, a test trigger per channel, all channels `lead = bothy`, source `completed`, only `GET` to Discord.
- [ ] The guide covers 22 13 step by step, white label, without instance values, and its rehearsal commands are executed by the test.
- [ ] The report ends with the owner's decision sheet for the live run.
- [ ] `make check` is green.

## Definition of done

- Guide, test and fixture as specified.
- Rulings of Design notes 1 and 2 recorded in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under `[Unreleased]`.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (the live run on a real Discord server and installation: PLAN 14 exit criterion 2 is met only after the owner's go and the live run).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), spec gaps found, and the decision sheet of Design note 3.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
