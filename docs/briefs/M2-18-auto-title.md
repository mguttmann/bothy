# M2-18: Auto title and placeholder detection

Status: ready | Depends on: M2-09 | Milestone: M2

## Goal

After this ticket the title source of every thread is decided in one place, the package `internal/chat/domain/title`, exactly as 09 "Auto-title" says. The placeholder words live as data per language, one file for German and one for English, embedded in the `chat` module; the built-in default of `thread.placeholder_titles` is rendered from them, and the effective value of that key is the list the server compares against. A person who creates a forum post or a thread with an empty or placeholder title (also with an appended number, "Neuer Chat 3") keeps `title_source = auto`; a real title, including short real names such as "AI" or "C#", becomes `manual`; a person's later title change always gives `manual`. The operator agent of the thread, typically after the first finished run in the auto thread of 09 3.7 that M2-05 creates without a title, replaces only an `auto` title and keeps `auto`; on a `manual` title it gets `409 title_manual` (AG-19). Guest agents set no title, and every other principal sets `manual` as 05 `thread.title.update` demands. Why: a title a person chose is never overwritten automatically (00 "Title source", R-AG-15, REUSE L23), placeholder words are data per language and not code (REUSE 5.1, 01 9), and the conformance suite (M2-31) needs AG-19 proven against a real run.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope, Out of scope, Exit criteria point 1, Test strategy, Red team, Risks, block "agents and extensions") with the rows M2-05, M2-08 and M2-09 (what exists), M2-14, M2-17, M2-19, M2-31, M2-33 and M2-34 (neighbours and who builds what next); section 7 rows M1-55 and M1-56; section 14 row M8-03; section 18.3 row "Fixed thread ID, auto title with source, edit title" (M1-55, M2-18, M4-20).
- Dependencies: the header names M2-09. Through it this ticket also builds on M2-08, M2-07, M2-06, M2-05, M2-04 and M2-02 and on the M1 tickets M1-07, M1-08, M1-12, M1-13, M1-16, M1-17, M1-26, M1-28, M1-44, M1-51, M1-54, M1-55 and M1-56 (all implied, so not in the header).
- `docs/spec/09-agents.md`: 2.2 "Features" (row "Title suggestion", `supports_title`), 3.3 "Channel switches" (`auto_thread`, `allow_mentioned_agents`), 3.6 "Guest agents", 3.7 "Auto-thread from a channel message", 4.7 "Adapter operations" (row "Suggest title"), 12 "Auto-title" (points 1 to 5), 14 "Reports without a trigger" point 2, 16.2 "Required scenarios" (AG-19; AG-04, AG-05 and AG-06 for the test world), 19 "Decisions of this chapter" (R-AG-02, R-AG-04, R-AG-15), 21 "Configuration values of this chapter" (row `thread.placeholder_titles`).
- `docs/spec/05-actions.md`: 2.1 'Column "Permission"', 2.3 "Effective level", 4.6 "Threads and agent sessions" (rows `thread.create` and `thread.title.update`), 6 "Durable event types and receive action" (rows `thread.created`, `thread.updated`), 8 "Decisions of this chapter" (R-AZ-01, R-AZ-13). On the base branch also `docs/spec/actions.yaml`, entry `thread.title.update` (right `write and (creator or manage or operator agent)`, audit `never`, `rl.write`).
- `docs/spec/03-data-model.md`: `threads` (`title`, `title_source` with "once `manual`, it stays `manual`", `operator_agent_id`, `created_by`, `version`), 6.3 "Lock order", 8 R-DM-08, R-DM-12, R-DM-20, 9 "Configuration values of this chapter" (`thread.title_max_chars`).
- `docs/spec/00-glossary.md`: "Thread", "Title source". `docs/spec/01-product.md`: 3.1 rows 'Button "New post" or "New chat"' and "Fixed thread ID, auto title with source, edit title"; 9 "Languages" (German and English).
- `docs/spec/07-permissions.md`: 2.9 "Agents as principals", 4.1 "Decisions and reasons" (`not_self`, `capability_missing`), 4.2 "Flow", SR9 "Acting-for binding (confused deputy)" (owned by M2-14), 10 "Examples" and 11.1 "Test matrix" (test world).
- `docs/spec/06-api.md`: 2 "Basic rules" (no `null`, no effect without change), 5 "Errors" (points 3 and 7; code `title_manual`), 8 "Concurrency" points 1 and 2 (`If-Match` on thread titles), 16 "Endpoints" (group Threads).
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (row `chat`), 5 "Layers within a module" (what a domain package may import), 7.2 "go-arch-lint", 8.1 "Flow", 13 "Configuration loading" points 2, 4, 5 and 6.
- `docs/spec/19-security.md`: SEC-IN-4, SEC-T07. `docs/spec/22-import-migration.md`: 5.1.1 "Threads" (title rule of the import, see the spec gap in Design note 11) and 12.2 "Round" point 3. `docs/SPEC.md` 8 "Chapters" (the core contract wins over other chapters).
- `docs/REUSE.md`: L23 "Auto title overwrites real titles" and 5.1 "Auto title placeholder rule" (the examples of protected short names; the parts that 09 does not adopt are named in Design note 4).
- `docs/api/openapi.yaml`: operations `createThread`, `setThreadTitle`, `getThread`; schemas `Thread` (`title`, `title_source`), `ThreadCreate`, `ThreadCreated`, `ThreadTitlePut`, `ThreadPayload`, `ThreadCreatedEvent`, `ThreadUpdatedEvent`, `Problem` (`x-problem-codes` with `title_manual`); responses `Conflict`, `Forbidden`, `PreconditionFailed`, `PreconditionRequired`. For the run-driven tests: `createMessage`, `putChannelBehavior`, `pollAgentEvents`, `ackAgentEvents`, `startRun`, `finishRun`, `listThreadRuns`. `docs/api/asyncapi.yaml`: messages `event.thread.created`, `event.thread.updated`.
- ADRs 0001, 0008, 0011, 0013, 0017, 0053.
- `docs/DECISIONS.md`: D-L4 (one source per value, the schema in code is the source), D-AG-14, D-W-100; section 10 for the format of new rows. No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- `docs/security/threat-model.md`: T-AI-04 (an agent writes outside what it operates). `docs/RISKS.md`: RK-01 (prompt injection; an injected runtime may try to rename threads). No metric of `docs/observability/signals.md` concerns this ticket.
- `docs/operations/configuration.md`: section 2 "Naming rule and formats" (lists), section 4 rows `thread.placeholder_titles` and `thread.title_max_chars`.
- Briefs: M1-55 (`chat.Threads` with `Create` and `SetTitle`, D1 placeholder rule and source at creation, D4 title change and `409 title_manual`, tests `TestPlaceholderTitle` and `TestAG_19_OperatorAgentTitleOnManual409`), M1-56 (`createThread`), M1-28 (descriptor `Resource` with `OwnerID`, requirement `All(Level(write), Any(Self, Cap(manage), ThreadOperator))`, `ThreadFacts`), M1-26 (`kernel/textcheck.DisplayText`, principal kinds `user`, `agent`, `app` as `actor.Kind`), M1-07 (`config.Key`, `config.KindStringList`, "an empty element is an error", `Default` equals the chapter cell), M1-08 (`make config-doc-check` rules D5 and D6, `tools/configdoc/planned.yaml`, `services/server/architecture/config-schema.json` from `services/server/tools/configschema`), M1-13 (unit of work), M1-16 (event log), M1-12 (`pgtest`), M1-44 and M1-51 (agent principals and tokens), M1-54 (operators).
- The briefs of the M2 tickets this one builds on: `docs/briefs/M2-02-channel-behavior-config.md`, `M2-04-trigger-decision.md`, `M2-05-runs-state-machine.md` (auto thread `T'` without title, its Out of scope names this ticket for the auto title), `M2-07-agent-consumers.md`, `M2-08-invocation-delivery.md`, `M2-09-run-lifecycle.md`.
- Code on the base branch: `internal/chat` (port, `domain`, `app`, `adapters/postgres`, `adapters/httpapi`; the placeholder rule and the `Config` field of M1-55), `internal/messaging` (M1-56 `createThread`), `internal/authz`, `internal/kernel/authorize`, `internal/kernel/actor`, `internal/kernel/errs` (`CodeTitleManual`), `internal/kernel/textcheck`, `internal/platform/config` (`kind.go`, `schema.go`), `internal/wire` (`configkeys_*.go`, `configchapters.go`), `services/server/tools/configschema`, `tools/configdoc`.

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

11. A thread has `title_source = auto` until a principal other than its operator agent sets a title; then it is `manual` and stays `manual` (09 12 point 1, 03 `threads.title_source`, 00 "Title source", 01 3.1).
12. Placeholder detection at creation: a person's title that is empty or equals an entry of `thread.placeholder_titles` (case-insensitive, white space normalised), also with an appended number ("Neuer Chat 3"), keeps `auto`; any other title of a person gives `manual` (09 12 point 2, R-AG-15).
13. The placeholder words are data per language, German and English, not code (PLAN row M2-18, REUSE 5.1 "Adaptation", 01 9); the list the server compares against and the title length come only from the keys `thread.placeholder_titles` and `thread.title_max_chars` (09 21, 03 9).
14. Short real names are no placeholders (REUSE L23 and 5.1, PLAN row M2-18).
15. The operator agent sets a title only while the source is `auto`, and the source stays `auto`; on `manual` the answer is `409 title_manual` and nothing changes (09 12 point 3, 09 4.7 row "Suggest title", 05 `thread.title.update`, 06 "Errors", AG-19).
16. Guest agents set no titles (09 12 point 4, 09 3.6); the decision is the one `authorize` call with the requirement of 05; without read right `404`, with read right `403` (R-AZ-01, ADR 0017).
17. Every title change of a principal other than the thread's operator agent sets `manual` (05 `thread.title.update`; the core contract wins over 09 12 point 1, `docs/SPEC.md` 8).
18. Title changes keep the rules of M1-55: `If-Match` required (`428`, `412 version_conflict`), at most `thread.title_max_chars`, no control or bidi characters (06 8 point 2, SEC-IN-4, SEC-T07), `thread.updated` only when something changed, no JSON `null` (ADR 0053).

## Scope

All paths under `services/server/` unless noted. Names of M1-55 and M2-05 to M2-09 below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `internal/chat/domain/title/doc.go`: package comment (what the package decides and why it is the only place).
- `internal/chat/domain/title/source.go`: `type Source string` with `Auto` and `Manual`; `type By uint8` with `ByPerson`, `ByOperatorAgent`, `ByOther`; `func (p Placeholders) SourceAtCreation(by By, title string) Source`; `func SourceOnUpdate(by By, current Source) (Source, error)`; the `title_manual` error (Design notes 5 and 6).
- `internal/chat/domain/title/placeholders.go`: `type Placeholders struct` (unexported fields), `func NewPlaceholders(entries []string) Placeholders`, `func (p Placeholders) IsPlaceholder(title string) bool`, `func Normalize(s string) string` (Design note 4).
- `internal/chat/domain/title/wordlists.go`: `//go:embed wordlists/*.txt`; `func Languages() []string`, `func WordList(lang string) ([]string, bool)`, `func DefaultEntries() []string`, `func DefaultConfigText() string` (Design note 2).
- `internal/chat/domain/title/wordlists/de.txt` and `internal/chat/domain/title/wordlists/en.txt` (the data files of the plan row).
- Tests next to them: `source_test.go`, `placeholders_test.go`, `wordlists_test.go`, `example_test.go` (`ExamplePlaceholders_IsPlaceholder`), `fuzz_test.go` (`FuzzIsPlaceholder`).

Change:

- `internal/chat/domain`: remove the placeholder rule and the source decisions M1-55 placed there; move every case of its unit test (`TestPlaceholderTitle` or the merged name) into `internal/chat/domain/title` without dropping one.
- `internal/chat/app`: the use cases behind `Threads.Create` and `Threads.SetTitle` classify the actor (Design note 6a) and call `title`; the app builds one `title.Placeholders` from its `Config` at construction (02 13 points 5 and 6). No other change to the order of checks, the locks, the events or the answers of M1-55.
- `internal/wire`: in the file that registers `thread.placeholder_titles` (expected `configkeys_chat.go`; create it if the key is missing): `Kind: config.KindStringList`, `Default: title.DefaultConfigText()`, `Chapter: chapter09`, the `Doc` sentence; `chapter09 = "09-agents.md"` in `configchapters.go` if missing; bind the effective list into the `chat` `Config`.
- `architecture/config-schema.json`: regenerate with the generator of M1-08 (`tools/configschema`).
- `tools/configdoc/planned.yaml` (repository root): remove an exact rule for `thread.placeholder_titles` if one exists (rule D6 reports a stale rule).
- `docs/spec/09-agents.md` (English, like the chapter): 12 points 1 and 2 and the 21 default cell as Design note 10 says.
- `docs/operations/configuration.md`: regenerate section 4 with `make config-reference`.
- `docs/api/openapi.yaml`: the `description` of `setThreadTitle` (Design note 10); then the server code generation.
- `docs/REUSE.md`: in L23 the line "Test:" names the test that now holds the lesson (`internal/chat/domain/title`, `TestIsPlaceholder_ProtectedShortNames`); nothing else in that file.
- `internal/chat/README.md` (English, existing structure), `services/server/CHANGELOG.md`, `docs/DECISIONS.md` section 10 rows `D-M2-18-<n>` for every Ruling below.

This ticket creates no migration, no operation, no event type, no job, no metric and no new top-level make target. `api/operations.yaml` stays unchanged (`createThread` and `setThreadTitle` are enabled by M1-56 and M1-55).

## Out of scope

- Thread storage, `createThread`, `setThreadTitle`, `getThread`, `If-Match` handling, title length and character checks, `thread.created` and `thread.updated` (M1-55, M1-56); this ticket changes only how the source is decided and where the placeholder words live.
- Creating the auto thread `T'` without a title (09 3.7) and the acceptance of a triggering message (M2-05); delivery and tokens (M2-08); starting and finishing runs (M2-09).
- SR9 for title changes made with a run's invocation token (M2-14).
- Reported capabilities in the heartbeat and `capabilities_effective` (M2-20, M2-08); a server-side refusal of a title from an operator agent whose `title` capability is not effective is not specified (spec gap, Design note 11).
- Setting the title once after the first finished run of a new session (09 12 point 5): Hermes Agent adapter (M2-33) and Claude Code runner (M2-34); SDK helpers (M2-29, M2-30); the AG-19 scenario of the fake agent suite (M2-31).
- Report threads of agents with the title from the first line (09 14) and agent messages (M2-19); incoming webhooks that create threads (M2-27); apps and their tokens (M2-24, M2-25).
- Titles of imported threads and renames in the read mirror (22 5.1.1 and 12.2): M8-03 to M8-05; they will call this rule through a `chat` port method their ticket adds.
- Showing the first line of the starting message until a title exists (09 3.7) and title editing in the apps (M4, M4-20). Title search vectors (M3-13).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-18-auto-title`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(chat): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test. Moving the cases of M1-55's placeholder test into the new package is not a deletion; list the move in the report.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for M2-02 and M2-04 to M2-09: their merged names win over the expected names here; list each difference.
- No real agent runtime and no model call: the adapter side in the tests is plain Go HTTP code with an agent API token; no third-party service. Time only through `kernel/clock`.
- The word lists hold exactly the non-empty entries of the 09 default, split by language; adding or removing a word is a change of 09 21 that this ticket does not make.
- No new third-party library: the rule uses only the Go standard library and `kernel-core` packages (02 5).
- Invocation tokens and API tokens never appear in logs, test output or problem bodies.

## Design notes

1. **One place for the rule.** `internal/chat/domain/title` is the only code that knows placeholders and title sources. M1-55's placeholder function, its source decision in `Threads.Create` and its source decision in `SetTitle` move there; after the move no other function in `services/server` compares a title with `thread.placeholder_titles` (check with `grep` before the review and say so in the report). The package imports only the standard library and `kernel-core` (`kernel/errs`, `kernel/textcheck`), as 02 5 allows for a domain package; the app layer classifies the actor, because `kernel/actor` is a kernel port that a domain package may not import.

2. **Word lists as data.** Ruling for docs/DECISIONS.md.
   a. Files `wordlists/<lang>.txt`, UTF-8, one entry per line; the first line is `# SPDX-License-Identifier: Apache-2.0`; lines starting with `#` are comments; blank lines are ignored. Every entry is already normalised (Design note 4a), passes `textcheck.DisplayText` and occurs once per file (compared with `strings.EqualFold`).
   b. Content, in this order: `de.txt`: `Neuer Chat`, `Chat`, `Ohne Titel`, `Test`; `en.txt`: `New chat`, `Chat`, `Untitled`, `Test`. Together they are exactly the non-empty entries of the 09 21 default; German first because 01 9 names German as the default language.
   c. The files are embedded with `embed.FS` and parsed once at package initialisation; reading the embedded file system touches only memory, so the domain stays free of input and output (02 5). Malformed embedded data panics at initialisation like `regexp.MustCompile`; the unit tests make that unreachable in a build that passes `make check`.
   d. `Languages()` returns the file stems in ascending order (`de`, `en`). `DefaultEntries()` returns the entries of all files in that language order and file order, skipping an entry already present under `strings.EqualFold`: `Neuer Chat`, `Chat`, `Ohne Titel`, `Test`, `New chat`, `Untitled`.
   e. `DefaultConfigText()` renders `DefaultEntries()` as a JSON array with `", "` between elements and no HTML escaping, exactly the form of the chapter cell: `["Neuer Chat", "Chat", "Ohne Titel", "Test", "New chat", "Untitled"]`.
   f. A new language is a new file plus the new default in 09 21 in the same change; the Go code does not change.

3. **Configuration key** `thread.placeholder_titles` (09 21, 02 13).
   a. The schema key uses `Default: title.DefaultConfigText()`, so the words exist once (D-L4); `make config-doc-check` (rule D5) holds the schema default equal to the 09 21 cell, which this ticket sets to the rendering of 2e.
   b. The empty string leaves the default. Ruling for docs/DECISIONS.md: the loader of M1-07 rejects an empty list element ("an empty element is an error"), so the cell of 09 21 cannot be registered as written, and the empty title is a placeholder through 09 12 point 2 itself ("that is empty").
   c. The effective value (default, file or `BOTHY_THREAD__PLACEHOLDER_TITLES`) is the list the server compares against; it replaces the default completely (09 12 point 2: "the entries of `thread.placeholder_titles`"). An entry that is blank after normalisation is ignored, because the empty title is a placeholder anyway. The list is read at start only (02 13 point 6).

4. **Matching** (09 12 point 2: "case-insensitive, whitespace normalized" and "with an appended number").
   a. `Normalize`: remove leading and trailing runes for which `unicode.IsSpace` is true and replace every inner run of such runes by one U+0020. Titles reach the rule only after the length and character checks of M1-55.
   b. A normalised title that is empty is a placeholder.
   c. A normalised title equal to an entry under `strings.EqualFold` (Unicode simple case folding) is a placeholder. Compare by a linear scan over the normalised entries; do not build lower-case map keys, because `strings.ToLower` is not case folding.
   d. Numbered form: a normalised title of the form `P + " " + D`, where `D` is one or more ASCII digits `0` to `9` and `P` is non-empty and equal to an entry under `strings.EqualFold`, is a placeholder ("Neuer Chat 3", "untitled 12", "Test 007"). The empty entry has no numbered form, so "2026" stays a real title. Ruling for docs/DECISIONS.md.
   e. Everything else is not a placeholder, in particular: an entry with digits but no space ("Test2"), with other characters ("Neuer Chat #3", "Neuer Chat (3)", "Neuer Chat 3."), an entry inside a longer title ("Chatbot", "Testrun", "New chat about invoices"), short real names ("AI", "C#", "PC 2", "UX", "Q3" and every other example of REUSE 5.1), and titles of only punctuation, symbols or emoji ("?", "...", a single emoji).
   f. Ruling for docs/DECISIONS.md: no Unicode NFKC and no "only punctuation up to a fixed length" rule. REUSE 5.1 names both, but 09 12 point 2 ends with "Otherwise the source becomes `manual`", NFKC would need a library outside the standard library in an adapter, and a fixed length would be a threshold without a configuration key (rule 9). The difference is reported (Design note 11).

5. **Source at creation** (09 12 points 1 and 2, M1-55 D1).
   a. `SourceAtCreation(ByPerson, t)` is `Manual` when `t` is not a placeholder, else `Auto`; an absent title counts as empty. `ByOperatorAgent` and `ByOther` always give `Auto`: agents and apps never create a `manual` title (09 12 point 1, "as long as no person").
   b. Ruling for docs/DECISIONS.md: the rule applies to every creation by a person that carries a title, forum post and thread from a message alike. 09 12 point 2 names forum posts because only they usually carry a title; the reason ("New chat" is not a title anyone wants to keep) holds for both.
   c. The title text is stored as given (M1-55); the rule only chooses the source. The auto thread of 09 3.7 is created by M2-05 without a title, so it starts `auto`.

6. **Source on a title change** (`setThreadTitle`, action `thread.title.update`).
   a. Classification in `internal/chat/app` from the request principal of `kernel/actor`: kind `user` is `ByPerson`; kind `agent` whose ID equals `threads.operator_agent_id` of the thread is `ByOperatorAgent`; every other agent or app is `ByOther`. Main threads have no title (M1-55 answers `400`), so the operator of a main thread never reaches this point.
   b. `ByPerson` gives `Manual`, also when the text is a placeholder and also on a thread that is already `manual`. Ruling for docs/DECISIONS.md: placeholder detection runs only at creation (09 12 point 2 "at creation", R-AG-15); a person who sets a title chose it (09 12 point 1).
   c. `ByOperatorAgent`: `Auto` stays `Auto`; `Manual` returns the `title_manual` error (`errs.New(errs.CodeTitleManual, ...)` or the domain error the merged M1-55 code maps to it), and the use case writes nothing and appends no event (09 12 point 3, AG-19).
   d. `ByOther` (an agent or app that passed `authorize` as the creator of the thread) gives `Manual`. Ruling for docs/DECISIONS.md: 05 `thread.title.update` says "sets `title_source = manual`" and exempts only the operator agent; 09 12 point 1 speaks only of people; `docs/SPEC.md` 8 makes the core contract win and asks the change that finds a contradiction to fix it, so this ticket aligns 09 12 point 1 and the description of `setThreadTitle` (note 10).
   e. Order of answers, unchanged from M1-55: `404` (unknown or invisible thread), `403` (`authorize`), `428` and `412 version_conflict` (`If-Match`), `400` (main thread, validation), then this decision. The decision uses the thread row read `FOR UPDATE` in the same unit of work (03 6.3), and every source change raises `version`, so a person's concurrent change can never be overwritten by the operator agent: with the old `If-Match` it gets `412`, with the new one `409 title_manual`.
   f. Who may call is decided only by `authorize` with the descriptor `{Kind: thread, ID, OwnerID: threads.created_by}` and the M1-28 requirement `All(Level(write), Any(Self, Cap(manage), ThreadOperator))`; the `chat` code adds no rights logic.
   g. Unchanged title and source answer `200` without event and without a new version (M1-55, 06 2). Otherwise `version + 1` and one `thread.updated` with `ThreadPayload` (scope `{space_id, channel_id, thread_id}`, resource `{thread, id, version}`, receive action `thread.read`, 05 6). Audit `no`, no rights version, rate-limit class `rl.write`.

7. **Guest agents** (09 12 point 4, 09 3.6). Note 6f enforces the rule: a guest agent is not the thread's operator and holds at most `write` (07 2.9, R-DM-08, R-DM-20), so in a thread it did not create it fails `Any(Self, Cap(manage), ThreadOperator)` with `forbidden` (the reason of the first alternative, `not_self`); without read right it gets `404` (R-AZ-01). No extra check in `chat`. An agent that created the thread itself (09 14) passes as creator under 05; that case is reported (Design note 11).

8. **Operator agent after a run.** The server does not count title changes: 09 12 point 5 ("once after the first completed run of a new session") binds the reference adapters, not the server. The operator agent may set an `auto` title with or without an active run; whether a run's invocation token narrows its rights is SR9 (M2-14).

9. **Errors and events stay as they are.** `409 title_manual` has no `reason` (06 5). No new problem code, event type, operation or audit entry. No JSON `null` in responses or events.

10. **Specification changes** in the same change (English in 09 and in `openapi.yaml`; no en or em dash):
    a. 09 12 point 1, replace by: "A thread has `title_source = 'auto'` until a principal other than its operator agent sets a title (a person, or an app or an agent as creator, 05 `thread.title.update`). Then the source becomes `manual` and stays so (03). Agents and apps always create threads with `auto`."
    b. 09 12 point 2: "when creating a forum post" becomes "when creating a forum post or a thread from a message"; add: "An appended number is a space and one or more digits 0 to 9; the empty entry has no form with a number. The default of `thread.placeholder_titles` is the union of the word lists per language (German and English) that the module `chat` carries as data files; a test keeps the table and the files equal."
    c. 09 21, default cell of `thread.placeholder_titles`: `["Neuer Chat", "Chat", "Ohne Titel", "Test", "New chat", "Untitled"]`; then `make config-reference`.
    d. `openapi.yaml`, `setThreadTitle.description`: "Every principal except the thread's operator agent sets `title_source = manual`. The operator agent may only replace an `auto` title, which stays `auto`; on a `manual` title it gets `409 title_manual`."
    e. Do not change 22 5.1.1, REUSE 5.1 or the core contract; the differences go into the report (note 11).

11. **Spec gaps to report** (with the evidence of the file and line): 22 5.1.1 names "the auto-title rule (REUSE.md, section 5.1)" for imports, and REUSE 5.1 differs from 09 12 (punctuation-only titles, NFKC, a number only for some entries, a fixed length as a constant); 09 2.2 names "Effect: Section 12" for the capability `title`, but neither 09 12 nor 05 nor `openapi.yaml` say what the server does when it is not effective; 05 lets an agent that created a thread set its title although 09 12 point 4 says guest agents set none. This ticket follows 09 12 and 05 as written.

12. Rulings for docs/DECISIONS.md, one row each with reason: notes 2 (word lists as embedded data and the rendered default as the only copy), 3b (empty string out of the default), 4d (numbered form), 4f (no NFKC, no punctuation rule), 5b (rule at every creation by a person), 6b (a person's change is always `manual`), 6d (`ByOther` gives `manual`). Facts already in the specification (operator rule, `409 title_manual`, `If-Match`) are not repeated there.

## Tests (write them first)

Tests that pass at their first run because M1-55 already built the behaviour (for example the operator agent's `409 title_manual` without a run) are listed in the report with that reason and the name of the M1-55 test that was red first; every test of the new package and every test of a Ruling must have a red run.

Unit (`internal/chat/domain/title`):

- `TestNormalize`: leading, trailing and repeated inner spaces, also Unicode spaces such as U+00A0 and U+3000, become single U+0020 or disappear at the ends; already normalised text is unchanged.
- `TestIsPlaceholder_DefaultList`: with `NewPlaceholders(DefaultEntries())`: every entry; case variants ("NEUER CHAT", "new CHAT"); white space variants; numbered forms ("Neuer Chat 3", "New chat 2", "untitled 12", "Ohne Titel 1", "Test 007", "Neuer Chat   3"); the empty string and a title of only spaces are placeholders. Not placeholders: "Test2", "Neuer Chat #3", "Neuer Chat (3)", "Neuer Chat 3.", "Neuer Chat 3a", "2", "2026", "Chatbot", "Testrun", "New chat about invoices", "Untitled draft", "?", "...", a single emoji.
- `TestIsPlaceholder_ProtectedShortNames` (plan row "protected short names not treated as placeholders", REUSE L23): every example of REUSE 5.1 plus "AI", "C#", "PC 2", "UX", "Q3", "2FA" is no placeholder under the default list and under each language list alone, and no word-list entry equals one of them under `strings.EqualFold`.
- `TestIsPlaceholder_ConfiguredListReplacesDefault`: with `["Draft"]` the titles "Draft" and "Draft 2" are placeholders and "Neuer Chat" is not; blank configured entries are ignored and the empty title stays a placeholder.
- `TestSourceAtCreation`: person with a real title gives `Manual`; person with absent, empty, placeholder or numbered placeholder title gives `Auto`; operator agent and other principals give `Auto` for every title.
- `TestSourceOnUpdate`: person on `Auto` and on `Manual` gives `Manual`, also with a placeholder text; operator agent on `Auto` gives `Auto`, on `Manual` the `title_manual` error with code `title_manual`; other principal on `Auto` and `Manual` gives `Manual`.
- `TestWordListFiles`: `Languages()` is exactly `de`, `en`; each file starts with the SPDX line, has only normalised entries that pass `textcheck.DisplayText`, no duplicates under `strings.EqualFold`, and the content of Design note 2b.
- `TestDefaultEntriesAndConfigText`: order of Design note 2d; `DefaultConfigText()` equals `["Neuer Chat", "Chat", "Ohne Titel", "Test", "New chat", "Untitled"]` and parses as a JSON array of strings with the same elements.
- `ExamplePlaceholders_IsPlaceholder` (doc example, checked output).
- `FuzzIsPlaceholder`: any input never panics; `IsPlaceholder(s) == IsPlaceholder(Normalize(s))`; `Normalize` is idempotent.

Unit (`internal/chat/app`, fakes): `TestTitleByClassification`: kind `user` is `ByPerson` (also an owner); the agent equal to `operator_agent_id` is `ByOperatorAgent`; another agent and an app are `ByOther`.

Unit (`internal/wire`): `TestConfigKey_PlaceholderTitles`: the key is registered as `list(string)` with chapter `09-agents.md` and `Default` equal to `title.DefaultConfigText()`; loading with `BOTHY_THREAD__PLACEHOLDER_TITLES=Draft, Sketch` gives the `chat` `Config` exactly `Draft`, `Sketch`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain, people and agent tokens from the identity test helpers, fake clock). World: 07 11.1 as M2-05 extends it (spaces Support with channels Allgemein and Eskalation, Marketing; Lotse operates Support, Texter operates Marketing, both `organization`; the Owner; Alex with `trigger` on Support (G-1); Robin with `write` on Support/Eskalation; Texter with `write` on Support/Eskalation; `allow_mentioned_agents = true` on Support/Eskalation, `auto_thread` at its default `true`), plus a forum channel Support/Fragen that inherits Lotse, with `write` for Sam and for Texter. Responses validate against `Thread` and `Problem`, events against `ThreadCreatedEvent` and `ThreadUpdatedEvent`; no JSON `null`.

- `TestCreateForumPost_TitleSource` (plan row "placeholders with a number detected", "a title set by a person becomes `manual`"): Sam creates posts in Support/Fragen through `createThread`: "Invoice export fails" gives `manual`; no title, "New chat 2", "Neuer Chat 3" and "  ohne   titel  " give `auto`; the stored title is the text as sent; each `thread.created` payload carries the same `title_source`.
- `TestCreateForumPost_ProtectedShortNamesManual` (plan row "protected short names not treated as placeholders"): Sam's posts "AI", "C#" and "PC 2" give `manual`.
- `TestCreateThreadFromMessage_TitleSource` (Ruling 5b): Robin starts threads from main-thread messages of Support/Eskalation with the title "Chat 3" (`auto`) and "Release plan" (`manual`).
- `TestSetThreadTitle_PersonAlwaysManual` (Ruling 6b): on an `auto` post that Sam created, Sam sets "Neuer Chat 2" and gets `200` with `manual`; one `thread.updated`.
- `TestAG19_OperatorAgentTitleAfterRun` (AG-19: an agent sets a title on a `manual` thread and gets `409 title_manual`): Alex mentions Lotse in the main thread of Support/Eskalation, which creates the auto thread `T'` with `title = ''` and `auto` (M2-05); Lotse's adapter code polls with `pollAgentEvents`, acks, calls `startRun` with the delivered token and `finishRun` with `completed`; then Lotse calls `setThreadTitle` with the `ETag` of `getThread`: `200`, the new title, `title_source` still `auto`, one `thread.updated`; Lotse again with a new title: `200`, still `auto`; Alex (the creator of `T'`) sets a title: `manual`; Lotse with the current `If-Match`: `409 title_manual`, title, source and `version` unchanged, no event; Lotse with the old `If-Match`: `412 version_conflict`.
- `TestGuestAgentSetsNoTitle` (09 12 point 4): after Lotse's run ended, Alex mentions Texter in `T'`, which creates a guest run (`listThreadRuns` shows it); Texter's `setThreadTitle` gives `403 forbidden` (deny reason `not_self` in the decision; the problem body carries a `reason` only where the M1-28 mapping exposes one); on a thread in Support/Allgemein, which Texter cannot read, it gives `404` with the same body and headers as an unknown thread id; nothing changes and no event is written.
- `TestSetThreadTitle_CreatorAgentSetsManual` (Ruling 6d): Texter creates a post in Support/Fragen through `chat.Threads.Create` inside a unit of work with Texter as actor (as `messaging` would): `auto`; Texter's `setThreadTitle` gives `manual`; Lotse (the thread's operator) then gets `409 title_manual`.
- `TestSetThreadTitle_ConcurrentPersonAndOperatorAgent`: on an `auto` post Sam and Lotse send `setThreadTitle` in parallel with the same `If-Match`, many rounds: exactly one `200` and one `412` per round; the final source is `manual` exactly when Sam's request succeeded; never a `manual` thread that turns `auto`.
- `TestPlaceholderTitles_ConfiguredList`: a server built through `internal/wire` with `thread.placeholder_titles` set to `Draft` in the configuration source of the test harness (never the process environment of parallel tests): Sam's post "Draft 2" gives `auto`, "Neuer Chat" gives `manual`.

Existing tests of M1-55 for titles (`TestAG_19_OperatorAgentTitleOnManual409`, `TestSEC_T07_ThreadTitleRejectsControlAndBidi`, `TestSetThreadTitle_IfMatchAndSource` or their merged names) stay green unchanged.

Red team focus of the milestone (PLAN 8 "Red team": confused deputy over the server API, prompt injection): an injected or foreign agent renaming threads is covered by `TestAG19_OperatorAgentTitleAfterRun`, `TestGuestAgentSetsNoTitle` and `TestSetThreadTitle_ConcurrentPersonAndOperatorAgent` (T-AI-04, RK-01); name them in the red team request.

Run `FuzzIsPlaceholder` once locally with `go test -run=^$ -fuzz=FuzzIsPlaceholder -fuzztime=<duration of your choice> ./internal/chat/domain/title/` from `services/server` and report duration and result; commit any crashing input as a seed after the fix.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/chat/... ./internal/messaging/... ./internal/runs/... ./internal/authz/... ./internal/wire/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`
- `make config-reference` then `make config-doc-check`
- `make docs`
- `make check`

## Acceptance criteria

- [ ] AG-19: the operator agent sets a title on an `auto` thread after a real run and the source stays `auto`; after a person's title it gets `409 title_manual` and nothing changes.
- [ ] Placeholders with a number are recognised ("Neuer Chat 3", "New chat 2" keep `auto`).
- [ ] Protected short names are no placeholders (every example of REUSE 5.1 and "AI", "C#", "PC 2" give `manual`).
- [ ] A person's title becomes `manual`: at creation when it is no placeholder, and on every title change.
- [ ] The placeholder words live only in `internal/chat/domain/title/wordlists/de.txt` and `en.txt`; the default of `thread.placeholder_titles` is rendered from them and equals the 09 21 cell; a configured list replaces it.
- [ ] Guest agents set no title (`403` with read right, `404` without); every other non-operator principal sets `manual`.
- [ ] 09 12, 09 21, `configuration.md` and the description of `setThreadTitle` match the behaviour; the rulings are in `docs/DECISIONS.md`.
- [ ] `make config-doc-check` and `make codegen-check` are green.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `chat` updated (the package `domain/title`, the word lists and how to add a language, the configuration key, the source rules of notes 5 and 6).
- Rulings of note 12 recorded in `docs/DECISIONS.md` (one row each with reason); the changes to 09, `configuration.md`, `openapi.yaml` and REUSE L23 of the Scope made in the same change.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
