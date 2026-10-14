# M8-05: Read mirror with settle rule

Status: ready | Depends on: M8-04 | Milestone: M8

## Goal

After this ticket the import tool keeps Bothy in step with Discord during parallel operation: `bothy import discord --apply --follow` runs mirror rounds every `import.follow.poll_interval`, `--apply --once` runs exactly one round (for scheduled calls; exit `3` when another run holds the lease), and `--follow` without `--apply` reads one round per interval and reports what it would copy without writing. A round reads `GET /guilds/{id}/channels` and the active threads, looks only at channels whose stored `lead` is `discord` and whose `last_message_id` is newer than the cursor, creates new threads, applies renames (only while the title is automatic), archiving and reopening, and copies per thread exactly the beginning of the new messages up to before the first busy or unsettled message by the settle rule (busy reactions, settle time, lookback), in this direction only. Channels with `lead = bothy` in which Discord shows new messages are reported as a warning and nothing is copied. A failed round (network, `5xx`, rate limit above the limit) never ends the loop: the next round waits with backoff up to `import.follow.max_backoff`, and an error in one thread does not hold up the others. Why: half-finished agent outputs must not be copied (RK-74, 22 12.3), and the mirror must run for days without a person watching it (22 12, R-IM-07).

## Read first

- `docs/spec/22-import-migration.md`: 4.1 (rows "One channel or thread", "Messages before an ID"), 5.1.1 (done rule), 7.3, 7.4 (lease), 10.1 (rows `--apply --follow`, `--apply --once`), 10.2 rule 3 (contradictions), 10.3 (exit `3` with `--once`), 12 (12.1 to 12.4), 16 row "Settle rule", 19 (`import.follow.*`).
- `docs/REUSE.md`: 5.3 (settle rule: rule and adaptation), L19 (archived only with an explicit marker), L21 (half-finished agent output in parallel operation), 4.1.
- `docs/RISKS.md`: RK-74.
- `docs/DECISIONS.md`: D-IM-04, D-IM-08, D-W-100, D-W-120, D-W-124; rows `D-M8-04-<n>`, `D-M8-11-<n>`; section 10 format.
- Briefs (merged names win, D-W-100): `M8-04-import-tool.md` (run flow, cursor and lease, plan, conversion, report, fakes and real-server tests), `M8-11-import-content-items.md` (mirror changes of a posted thread), `M8-01-discord-read-client.md` (`GuildChannels`, `ActiveThreads`, `Channel`, `MessagesAfter`, `MessagesBefore`, the guard).
- Code on the base branch: `services/server/cmd/bothy/importtool/...`, `services/server/test/importtool`.

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

11. Only the direction Discord to Bothy; only channels with `lead = discord` in the stored mapping; the tool never writes to Discord (22 12.1, R-IM-12).
12. Settle rule of 22 12.3 items 1 to 4 with `import.follow.busy_reactions`, `import.follow.settle_after` and `import.follow.busy_lookback`; types that are not copied do not stop the copy.
13. Round of 22 12.2 items 1 to 5; backoff up to `import.follow.max_backoff`; errors of one thread do not hold up others.
14. `--follow` with `--once` and `--only-thread` with `--follow` end with exit `2` without any call (22 10.2 rule 3); `--once` exits `3` when another run's lease is fresh (22 10.3).
15. A dry mirror round (`--follow` without `--apply`) writes nothing (22 10.1).
16. Edits and deletions after copying are not kept in sync (22 12.4); a later full run counts "changed at the source".

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create (package `cmd/bothy/importtool` unless named otherwise):

- `settle.go`: the pure settle function of Design note 1.
- `follow.go`: the round and the loop (notes 2 to 4), the flags `--follow` and `--once` with their contradiction rules.
- Tests named under "Tests".

Change:

- `run.go`, `report.go` of M8-04: the mirror modes, the warning row for `lead = bothy` channels, the "held back" counts.
- `cmd/bothy/importtool/bothyapi/bothytest` and `discord/discordtest`: fault and timing hooks the tests need (a message edited "now", reactions added after a round).
- `cmd/bothy/importtool/README.md` (mirror section with the settle rule and a scheduled `--once` example), `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M8-05-1` to `D-M8-05-5`.

## Out of scope

- `switch` and `revert` (M8-07; they reuse the round of this ticket for one channel).
- Keeping edits and deletions in sync (never, 22 12.4).
- A container image or service definition for a long-running mirror (installation-specific, 22 "Execution flow"; the runbook of M8-09 describes it).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-05-read-mirror`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(import): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Never contact Discord or any real service: the fakes of M8-01, M8-02 and M8-10 and the harness server of M1-71 on `127.0.0.1` with port 0, PostgreSQL through `pgtest`. Without a Docker-compatible runtime the real-server tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `t.TempDir()`, removed at the end.
- Never read or print secrets.
- Time only through `kernel/clock`; every wait of the loop goes through an injectable sleeper; tests never sleep for real beyond a few milliseconds.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Settle function** (22 12.3; REUSE 5.3; Ruling for docs/DECISIONS.md). `Settle(now time.Time, newMsgs []discord.Message, lookback []discord.Message, starter func(id string) *discord.Message, cfg SettleConfig) (copy []discord.Message, held int, reason HoldReason)` with `newMsgs` ascending. A message is busy when it, or for type 21 its starter message, has a reaction whose emoji name is in `import.follow.busy_reactions` with `count > 0`; unsettled when `max(timestamp, edited_timestamp)` is less than `import.follow.settle_after` before `now`. `copy` is the longest prefix of `newMsgs` before the first busy or unsettled message among the types that are taken over (22 6.1); messages of other types inside the prefix are passed through as skipped and never stop it. If any of the `lookback` messages (the `import.follow.busy_lookback` messages before the first new one, read with `MessagesBefore`) is busy, `copy` is empty and `reason` is `lookback_busy`. Pure, no I/O.

2. **Round** (22 12.2). (a) `GuildChannels` and `ActiveThreads`. (b) For each mapped channel with stored `lead = discord` (read the mapping from `getImport` every round, because the owner changes leads in the web console) whose `last_message_id` or whose threads' `last_message_id` is newer than the cursor: read new messages with `MessagesAfter` from the cursor, pages of `import.discord.page_size`. (c) New threads (in the active list or a channel's `last_message_id` revealing them) are planned and posted as in M8-04; for known threads compute the mirror change: name, archived state per 22 5.1.1 (a thread missing from the active list is confirmed with `Channel(id)` before it is closed), and post the thread item so the server applies the rules of M8-11 note 3 b. (d) Per thread run the settle function and post the copyable prefix as in M8-04 (people, attachments, conversion, batches, cursor after each batch). (e) Channels with `lead = bothy` whose `last_message_id` is newer than the cursor: a warning row "written in Discord after the switch-over", nothing copied (22 12.2 item 4, 11 notes). (f) The lease is renewed by the ticker of M8-04 independently of the round.

3. **Loop and backoff** (22 12.2 item 5; Ruling for docs/DECISIONS.md). After a failed round wait `min(import.follow.max_backoff, import.follow.poll_interval * 2^failures)`, after a successful round `import.follow.poll_interval`; `failures` resets after a success. The lease ticker of M8-04 keeps renewing during every wait, so a long backoff never frees the lease of a living mirror (22 7.4); a renewal that fails is a failed round, not the end of the loop. A failure inside one thread (a `403` on its channel, a conversion error of one message, a `409` from Bothy for one batch) is recorded in the report and the round continues with the next thread; only errors that hit every call (network, `5xx` of Bothy, `401`, Discord rate wait above the limit) fail the round. A source that is no longer `running` (the owner paused it) makes the round a no-op with a note; `completed` or `failed` ends the loop with exit `0` or `1`. SIGINT and SIGTERM finish the current batch, clear the lease and exit `0`.

4. **`--once` and dry rounds.** `--apply --once` is one round with the lease rules of M8-04 (exit `3` when another run holds it). `--follow` without `--apply` runs the round of note 2 with reads only, no lease and no write, printing "would copy" counts per channel each interval; `--once` without `--apply` is one such dry round.

5. **Rulings for `docs/DECISIONS.md`** (`D-M8-05-<n>`): the settle function signature and the type pass-through (note 1); leads re-read each round (note 2 b); the backoff formula and the failure classes (note 3); behaviour on a paused, completed or failed source (note 3); dry rounds without a lease (note 4).

## Tests (write them first)

- `importtool/settle_test.go` (plan acceptance, 22 16 row "Settle rule"): table test with one row per case: all settled; a busy message in the middle stops the copy before it; an unsettled message stops; an edit within `import.follow.settle_after` makes an old message unsettled; a type-21 message whose starter is busy stops; a skipped type inside the prefix does not stop; a busy message in the lookback empties the copy; a reaction with `count = 0` is not busy; an emoji outside `import.follow.busy_reactions` is not busy.
- `importtool/follow_test.go` with the fakes: `TestFailedRoundWaitsAndContinues` (plan acceptance: a `5xx` round is followed by a wait recorded by the sleeper and a successful round); `TestBackoffCapped`; `TestLeaseFreshDuringLongBackoff` (fake clock: a backoff of `import.follow.max_backoff` keeps the lease younger than three intervals); `TestThreadErrorDoesNotBlockOthers`; `TestLeadBothyWarnsAndCopiesNothing`; `TestLeadsRereadEachRound`; `TestDryMirrorWritesNothing` (plan acceptance: Bothy fake logs only `GET`, working and temp folders empty); `TestOnceExit3WhenLeaseHeld`; `TestContradictoryOptionsExit2` (no call to either fake); `TestPausedSourceIsNoop`.
- `test/importtool/mirror_test.go` against the real server: after an import, new fixture messages (one settled, one busy, one unsettled) and a rename of an automatic and of a manual thread and an archived thread; one round copies only the settled one, renames only the automatic one, closes the archived one; after advancing the fake clock beyond `import.follow.settle_after` and removing the busy reaction, the next round copies the rest in order; no message is copied twice.

Commands that must pass, from the repository root:

- `cd services/server && go test ./cmd/bothy/... ./test/importtool/... -race -count=1`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] The settle rule table test covers busy, unsettled, starter, pass-through and lookback.
- [ ] A failed round waits and does not end the loop; the backoff is capped.
- [ ] A dry mirror run writes nothing.
- [ ] Only `lead = discord` channels are copied; `lead = bothy` channels with new Discord messages are warned about.
- [ ] README, CHANGELOG and rulings are updated.
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
4. What is not proven (the settle values come from the predecessor and were never measured against the real Discord, 22 19).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
