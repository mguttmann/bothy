# M4-23: Search, quick switcher, mentions and Running now

Status: ready | Depends on: M4-20, M3-14 | Milestone: M4

## Goal

After this ticket the cross-cutting views of 01 3.5 and 5.1 work on the desktop: the search view D7 with the full-text search of the server (`search`, types messages, threads and files, filters of 14 4.4, sort, cursor paging, highlighted segments built only from the server's `segments`), a local offline search over the FTS5 index of `local_store` with the note "messages stored only on this device" and local hits only while the server answer is pending; the quick switcher D6 (spaces, channels, people, agents and commands from the local database, thread titles by prefix from `searchThreadTitles`, recently visited first, at most `ui.quick_switch_max_results`, only what is visible); the mentions view (`listMyMentions`, `/mentions`); and "Running now" D10 (every active run the person may see, filters, stop only where the server allows `run.cancel`, the organization sum as an estimate and personal runs as information only), with the sidebar entry and its counter. The local search index is filled from messages, thread titles and transcripts in the cache and loses every row of a removed target (14 12, client row). Why: these views are how a person finds work across channels; search must never show a hit from a channel the person lost, neither online nor offline (B7, 14 6).

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-20, M4-23, M4-24, M4-34.
- `docs/spec/01-product.md`: 3.5 "Notification, search, navigation", 5.1 D6 "Quick switcher", D7 "Search", D10 "Running now", 6.7 (costs), 7 (Cmd+K, Cmd+Shift+F, Cmd+F, Cmd+Shift+R, Cmd+Shift+M: bindings in M4-24, views here), 8, 14 (`ui.quick_switch_max_results`).
- `docs/spec/14-search.md`: 2, 4 "Query" (4.1 input and notation, 4.4 filters, 4.5 `candidates_capped`, 4.6 segments, 4.7 paging and `cursor_invalid`, 4.9 prefix search), 6 "Offline search in the client", 11 "Endpoints", 12 "Tests" (row "Client").
- `docs/spec/15-client-architecture.md`: 6.1 (`search_index`, `my_mentions`, `runs`), 6.3, 11 (`/running`, `/mentions`, `/search?q=`).
- `docs/spec/04-sync-protocol.md`: 8.3 (deleting the search index of removed targets), 11.3 item 1 (`run.state_changed` feeds "Running now").
- `docs/spec/09-agents.md` 8 "Stop" (who may stop).
- `docs/api/openapi.yaml`: `search`, `searchThreadTitles`, `listMyMentions`, `listActiveRuns`, `cancelRun`, `getTranscript`; problem codes `validation_failed` (`q` with `empty`, `too_long`), `cursor_invalid`.
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-20-thread-view.md` (history, jump to `?m=`), `docs/briefs/M4-12-local-store.md` (`SearchDao`, FTS5 tokenizer), `docs/briefs/M4-14-sync-catch-up.md` (indexing on apply, removal), `docs/briefs/M4-19-sidebar-post-list.md` (sidebar header entries), `docs/briefs/M4-17-fake-server.md`; for the server side `docs/briefs/M3-14-search-coordinator.md`.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits only through named keys: `ui.quick_switch_max_results` from `AppConfig`; page sizes and query length from the server's answers (`400 validation_failed` with `too_long`), never a literal limit in code.
10. No permission logic in the client: the views show what the server returns and what the local database holds; a hidden target is never inferred (15 1).

Product requirements implemented here:

11. Server search with `q`, `type`, the filters of 14 4.4 as parameters (the app may translate a notation like `from:Alex` into `author_id`), `sort`, cursor paging; the result count is only what is visible; `candidates_capped` asks for a narrower search (14 4).
12. Highlighting only from the server's `segments`; no HTML is interpreted (14 4.6).
13. Offline: local FTS5 hits with the note "messages stored only on this device"; online the server results apply and local hits only supplement them while the server answer is pending (14 6).
14. On `access.changed` with `removed` and on revocation the local index loses every row of the targets (14 6, 14 12).
15. Quick switcher: channels, threads, people, agents and commands, only from what is visible, recently visited first, at most `ui.quick_switch_max_results`, with the filter row Channels, Threads, People, Agents, Commands (01 D6).
16. Mentions: the person's own mentions with jump to the message (01 3.5).
17. Running now: all active runs the person may see with state, duration, context, costs; stop only where `run.cancel` is allowed; the sum of organization runs as an estimate and personal runs as information only (01 D10, 6.7).

## Scope

Create in `apps/client/lib/`:

- `search/` (`search_view.dart` with the filter column of D7, `search_controller.dart` merging server and local results, `query_notation.dart` translating `from:`, `in:`, `has:file`, `before:`, `after:` into parameters, `segments_text.dart` rendering `segments`, `local_search.dart` over `SearchDao`, `providers.dart`).
- `switcher/` (`quick_switcher.dart` overlay, `switcher_sources.dart` local sources and the prefix call, `recent_targets.dart` kept per device, Design note 3).
- `mentions/mentions_view.dart`, `running/running_now_view.dart`, `running/running_sum.dart`.
- `sync/search_index_feed.dart` only if M4-14 does not already index thread titles and transcripts on apply (transcripts from `file.updated` with `transcript_status = done`).
- Fake server handlers through the registry of M4-17 for `search`, `searchThreadTitles`, `listMyMentions`, `listActiveRuns`, `cancelRun` if missing.
- Tests under `test/search/`, `test/switcher/`, `test/running/`, goldens under `test/goldens/search/`, integration tests under `integration_test/` (Tests).
- ARB keys for every new text in `packages/l10n`.

Change:

- `apps/client/lib/sidebar/sidebar_header.dart` (M4-19): the "Jump to" field opens the switcher and "Running now" shows the live count.
- `apps/client/README.md` (cross-cutting views).
- `docs/DECISIONS.md` section 10: rows `D-M4-23-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Search in the thread (Cmd+F) as a binding and the shortcut set (M4-24; the in-thread search uses the server search with `channel_id` and the thread, built here as an API of the search controller).
- Live topology and statistics (M6), notification settings (M4-33).
- File text search beyond what the server returns for `type = files` (M3-15 on the server).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-23-search-switcher`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Search controller.** Input `q` plus filter state; on submit: start the server query (`search`), and, while it is pending or when offline, query `SearchDao.search` locally and show those hits marked "stored only on this device"; when the server answers, replace the list with the server hits (the local ones disappear unless offline). Paging with the opaque `next_cursor`; `400 cursor_invalid` restarts from page one; `400 validation_failed` on `q` shows the server's reason (`empty`, `too_long`). Results show path (space, channel, thread), author, time and the segments; selecting a hit navigates to `/t/<thread>?m=<message>` or the file.

2. **Notation.** `query_notation.dart` is a pure function that splits `q` into the free text for the server and parameters: `from:<handle>` to `author_id` (resolved against local principals; an unknown handle stays text), `in:<channel>` to `channel_id`, `has:file`, `before:` and `after:` with dates; anything else stays in `q`. The server never sees the notation (14 4.1).

3. **Quick switcher.** Sources: spaces and channels (local), people and agents (local principals), commands of the current channel (local), thread titles (local prefix match on loaded threads, plus `searchThreadTitles` when online, merged by ID). Ranking: recently visited targets first (a per-device list of the last targets kept in a local-only table or the settings store of the device, never synchronized, because 01 11 has no key), then prefix matches, then the rest; at most `ui.quick_switch_max_results`. Tab cycles the filter row. Enter navigates; Esc closes.

4. **Running now.** `listActiveRuns` on opening and the local `runs` table (kept by `run.state_changed`) for live changes; filters Space, Agent and "mine only"; rows with state icon, agent, path, duration, context, costs with the suffix rules; Stop through `cancelRun` (outbox) only where the run's capabilities allow it; the footer sums organization runs as "estimate" and states that personal runs are information only. The sidebar counter is the count of active runs in the local table.

5. **Index hygiene.** The index rows come from the dispatcher (M4-14) and are removed with targets; a test asserts the client row of 14 12 against the real index after an `access.changed` with `removed`.

6. **Rulings for docs/DECISIONS.md** (`D-M4-23-<n>`): merge rule of server and local hits (note 1); the notation (note 2); recent targets per device (note 3).

## Tests (write them first)

Unit and widget tests:

- `test/search/notation_test.dart`: each notation form, unknown handles, mixed text.
- `test/search/controller_test.dart`: local hits shown while the server is pending and replaced afterwards; offline only local hits with the note; `cursor_invalid` restarts; `validation_failed` reasons shown; segments rendered as text with match marks, never as HTML.
- `test/search/offline_revocation_test.dart`: after `access.changed` with `removed` for a channel, the local index has no row of it and the offline search finds nothing there (14 12, client row); after a wipe nothing is found.
- `test/switcher/switcher_test.dart`: recent targets first; limit from `AppConfig`; only local visible items plus server titles; filter row with Tab.
- `test/running/running_now_test.dart`: rows, filters, Stop visible only with the capability, the sum line.
- `test/goldens/search/*`: search view with filters and results, quick switcher, mentions, Running now; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration tests (`integration_test/`, against `FakeServer`):

- `search_test.dart`: search finds a message of a visible channel; going offline shows local hits with the note; after the fake removes the channel the offline search finds nothing there.
- `switcher_mentions_running_test.dart`: Cmd+K or Ctrl+K opens the switcher (through the action, the binding is M4-24) and jumps to a thread; the mentions view lists a mention of Alex by Sam; Running now lists the scripted agent's run and stops it.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Integration tests of search, quick switcher, mentions and Running now are green against the fake server.
- [ ] The offline search deletes the hits of removed targets and of a revoked device (14 12, client row).
- [ ] Server search with filters, paging and segments as 14 4 says; local hits only as 14 6 allows.
- [ ] Quick switcher with recent targets first and the limit from `AppConfig`; Running now with stop only where allowed and the cost suffixes.
- [ ] Goldens recorded; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01 or 14.
- `apps/client/README.md` (English) as in Scope.
- Rulings of Design note 6 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the integration tests ran, and the result of the offline revocation test with the number of index rows before and after.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
