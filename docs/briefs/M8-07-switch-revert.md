# M8-07: Switchover and revert per channel

Status: ready | Depends on: M8-05, M8-06 | Milestone: M8

## Goal

After this ticket the owner can move one channel at a time from Discord to Bothy and back, in an order in which two places never lead at the same time. `bothy import discord switch --guild <id> --channel <id> --discord-locked` runs the last mirror round for that channel (22 13.3 step 2) and ends with exit `1` and the number of held-back messages when the settle rule still holds anything back; otherwise it prints the owner's step 3 for the web console (lead `bothy`, which lifts read-only in the same transaction, then the planned operator with `channel.operator.set`) and the probe of step 4. The flag `--discord-locked` is the owner's statement that step 1 (Discord channel read-only, agent removed there) is done; without it the command ends with exit `2` before any call, so the order cannot be skipped by accident. `bothy import discord revert --guild <id> --channel <id>` only reads: it prints, per Bothy thread of the channel, the number of messages written in Bothy while it led (they never go to Discord, R-IM-12) and the owner's steps of 22 13.4. `status` gains per channel the lead, the read-only state and whether the channel's operator equals the planned operator. An integration test runs the whole sequence against the real server and the fake Discord: mirror, switch, test trigger with session binding, a late Discord message warned about and not copied, revert, mirror resumes; after every step exactly one side leads. Why: the migration is channel by channel without a big bang (PLAN 14 goal, 22 13), and the switch is the step where duplicate agent answers or lost messages would happen.

## Read first

- `docs/spec/22-import-migration.md`: 8 (owner steps in the web console), 9 (binding at the probe), 10.1 (rows `switch`, `revert`, `status`), 12.1, 12.2 item 4, 13 (13.1 states, 13.2, 13.3, 13.4, 13.5), 16 (rows "Read-only in the mirror", "Binding once"), 17 (R-IM-07, R-IM-12).
- `docs/spec/05-actions.md` (core contract, read only): rule 1.8, `channel.operator.set`, `import.manage`.
- `docs/spec/08-identity.md` 6.4 (no step-up from the tool).
- `docs/DECISIONS.md`: D-IM-04, D-W-100, D-W-120, D-W-124; rows `D-M8-03-<n>`, `D-M8-04-<n>`, `D-M8-05-<n>`, `D-M8-06-<n>`; section 10 format.
- Briefs (merged names win, D-W-100): `M8-03-import-server.md` (note 4 `setImportLead`), `M8-04-import-tool.md` (flow, lease, `status`, real-server tests), `M8-05-read-mirror.md` (round, settle function), `M8-06-session-binding-import.md`, `M1-54-chat-operators.md` (`setChannelOperator`, I1), `M2-12-agent-sessions.md` (the agent test client of its tests).
- Code on the base branch: `services/server/cmd/bothy/importtool`, `services/server/test/importtool`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. Order of 22 13.3: Discord locked first (owner, confirmed by `--discord-locked`), last mirror round and settle check by the tool, then the Bothy side in the web console; the tool never calls `import.manage` or `channel.operator.set` (08 6.4, 22 8).
12. `switch` aborts before step 3 when busy or unsettled messages are still in the channel after the last round (exit `1` with the counts, 22 13.3 last paragraph).
13. `revert` copies nothing to Discord and only reads; it prints the messages written in Bothy per thread so the owner decides about them (22 13.4, R-IM-12).
14. In Bothy a channel never leads twice: `lead` and `channels.read_only` change in one transaction (M8-03), and the mirror copies only `lead = discord` channels (M8-05).
15. Output never contains message content or names of people (22 10.2 rule 4).

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create (package `cmd/bothy/importtool`):

- `switch.go`: the command `switch` (Design notes 1 and 2).
- `revert.go`: the command `revert` (note 3).
- Tests named under "Tests", including `services/server/test/importtool/switch_test.go`.

Change:

- `status.go` of M8-04 (note 4), `run.go` (the subcommands), `README.md` (the per-channel migration flow with the console steps).
- `cmd/bothy/importtool/bothyapi/bothytest`: the routes `getChannel`, `listChannelThreads` and `listThreadMessages` that `revert` and `status` read.
- `docs/spec/22-import-migration.md` 10.1 (the flag `--discord-locked`), 13.3 (that the flag stands for step 1).
- `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M8-07-1` to `D-M8-07-4`.

## Out of scope

- The console screens and actions of step 3 and of revert step 1 (M8-08); the test performs them through the API with a step-up session, as the console would.
- Any Discord write, including setting the Discord channel read-only (owner step in Discord).
- Completing the source (console, 22 13.5).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-07-switch-revert`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(import): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Never contact Discord or any real service: the fakes of M8-01, M8-02 and M8-10, the harness server of M1-71 with `pgtest`, and the agent test client of M2-12; no real model call. Without a Docker-compatible runtime the integration test fails; it never skips.
- Stop only processes you started, by PID. Temporary directories only via `t.TempDir()`, removed at the end.
- Never read or print secrets.
- Time only through `kernel/clock`; tests use a fake clock.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Preconditions of `switch`** (22 13.3; Ruling for docs/DECISIONS.md). Without `--discord-locked` exit `2` and the text of step 1, before any call. Then the M8-04 steps (configuration, token, session, preflight for this channel only); the channel must be mapped with a target other than `skip`, imported (`channel:<id>` posted), with stored `lead = discord`, and the source `running`; otherwise exit `2` naming the condition. The lease rules of M8-04 apply (exit `3`, `--takeover`).

2. **Last round and decision.** Run the mirror round of M8-05 for this one channel and its threads. Then re-read the channel's threads and messages newer than the cursor and apply the settle function once more with the current time: when anything is held back (busy, unsettled, lookback), exit `1` with the counts per reason and the hint to repeat after `import.follow.settle_after`; when Discord shows messages newer than the cursor that are not held back (written between the round and the check), run the round once more and decide again (at most twice, then exit `1`). On success print, on standard error, step 3 for the web console (Import, channel, "Bothy leads", then the channel's operator, naming the planned operator's handle from the stored mapping) and step 4 (probe: write in an imported thread, the agent answers in Bothy only), and exit `0`. The tool writes nothing beyond the mirror round and the lease.

3. **`revert`** (22 13.4). Reads only (no lease). For the mapped channel: its main thread and its threads in Bothy (`listChannelThreads`, `listThreadMessages` with pagination); count messages with `origin` other than `import` and `kind` other than `system` per thread; print one line per thread with a count above zero (Bothy thread ID and count, no title) and the total; then the owner steps: web console "Discord leads" for the channel (sets read-only and cancels active runs), remove the operator; in Discord lift read-only and add the agent again; the mirror resumes at the cursor. Exit `0`. A channel whose stored lead is already `discord` prints the same, noting the console step is done.

4. **`status` per channel.** For every mapped channel: Discord channel name, Bothy channel name, stored `lead`, `read_only` of the Bothy channel, and `operator ok`, `operator differs from planned` or `no operator` (`getChannel`); a channel with `lead = bothy` and `read_only = true` (or `lead = discord` and `read_only = false`, after someone used `setChannelReadOnly` by hand) is flagged "inconsistent" (22 12.1: the flag is set by the ownership list).

5. **Rulings for `docs/DECISIONS.md`** (`D-M8-07-<n>`): the `--discord-locked` confirmation (note 1); the second round rule and exit codes of `switch` (note 2); what `revert` counts (note 3); the consistency flags of `status` (note 4).

## Tests (write them first)

- `importtool/switch_test.go` with the fakes: `TestSwitchNeedsDiscordLocked` (exit `2`, no request at either fake); `TestSwitchHeldBackExit1` (busy and unsettled counts); `TestSwitchSecondRound`; `TestSwitchWrongLeadExit2`; `TestRevertOnlyReads` (Bothy fake logs only `GET`, Discord fake logs nothing); `TestStatusFlags`.
- `test/importtool/switch_test.go` (plan acceptance: "integration test: mirror, switchover, test trigger, revert; two places never lead at the same time") against the real server, with Robin's step-up session standing in for the web console and the agent test client for Lotse:
    1. Seed: Alex has a grant with `trigger` on the target channel's space. Import and mirror a fixture channel with an imported session reference; `lead = discord`, read-only, no operator.
    2. A busy Discord message: `switch --discord-locked` exits `1`; after it settles and the busy reaction is gone, `switch` exits `0` and the message is copied once.
    3. Console step through the API: `setImportLead` `bothy`, `setChannelOperator` Lotse.
    4. Probe: Alex writes in the imported thread; exactly one run for Lotse; the imported session binds (`session_mode = bind`); Lotse's answer is in Bothy; the fake Discord logged only `GET` for the whole test.
    5. A late Discord message in the switched channel: the next mirror round warns and copies nothing.
    6. `revert` prints the thread with two Bothy messages (Alex and Lotse).
    7. Alex triggers Lotse once more and the agent test client keeps the run `running`; console step: `setImportLead` `discord`; that run gets `run.cancel_requested` with `channel_read_only`; then the console removes the operator (`setChannelOperator` with the value 03 uses for no operator); Alex's `createMessage` is `403 channel_read_only`.
    8. The next mirror round copies the late Discord message of step 5, after the cursor, once.
    After every step the test asserts the invariant: stored `lead = discord` exactly when the Bothy channel is read-only and has no running run of Lotse started after the step; stored `lead = bothy` exactly when it is writable (22 12.1).

Commands that must pass, from the repository root:

- `cd services/server && go test ./cmd/bothy/... ./test/importtool/... -race -count=1`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] The integration test runs mirror, switch, test trigger, revert and resume, and after every step exactly one side leads.
- [ ] `switch` refuses without `--discord-locked` and aborts with held-back messages.
- [ ] `revert` only reads and counts the messages written in Bothy per thread.
- [ ] README, 22, CHANGELOG and rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go symbol; comments explain why.
- `cmd/bothy/importtool/README.md` updated in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` entry under `[Unreleased]`.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (the Discord steps are the owner's and are only confirmed by a flag).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
