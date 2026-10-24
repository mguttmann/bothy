# M4-39: Run graph and control room on the desktop

Status: ready | Depends on: M2-51, M4-21, M4-23, M6-11 | Milestone: M4

## Goal

After this ticket a person on the desktop sees how a run works while it works. The live box (D3) has a button "Graph" that opens the run graph D14: an indented trace view with the agent as the root, subagents, workflows and background jobs as children with their tools, each row with state as color, icon and word, duration, current tool and cost where known, and a bar per row on a shared time axis; the graph grows live and keeps its tree when old steps are summarized. "Running now" (D10) becomes the control room: live cards with agent, path, state, duration, current tool, open subagents, workflows and jobs, stall notice and cost, Stop where allowed, each opening its run graph; "List" keeps the rows of M4-23. Both views have a list form for screen readers. The derivation of the tree is one pure function in `packages/bothy_viz` that the mobile layouts of M5-14 reuse. Why: the owner wants to see visually what runs and the agents they start are doing right now (2026-10-24; D-RV-1, D-RV-2, D-RV-7, D-RV-8, D-RV-11, D-RV-12).

## Read first

- `docs/design/run-visualization.md` (all), `docs/decisions.d/run-visualization.md`.
- `docs/PLAN.md`: section 3; section 10 rows M4-16, M4-21, M4-23, M4-24, M4-39; section 11 row M5-14 (who builds on it); section 12 rows M6-11 and M6-12.
- Dependencies: the header names M2-51, M4-21, M4-23 and M6-11; everything below them is implied.
- `docs/spec/01-product.md`: 3.2 (rows "Overview Running now" and "Run graph"), 5.1 D3, D10, D14, 6.2, 6.7 (costs), 8 (rows "Live content", "Run graph and control room", "Motion", "Color"), 13 (R-PR-15, R-PR-16), 14 (`ui.livebox_visible_steps`, `a11y.live_announce_min_interval`).
- `docs/spec/09-agents.md`: 5.3 (summary step per parent), 5.4, 5.5 (stall), 5.6 (all), 8 (who may stop), 18.4 (what the runner reports).
- `docs/spec/04-sync-protocol.md`: 6.2 (`focus` with `idle` and `background_thread_ids`), 11.1, 11.3 (items 1 to 7), R-SY-27; `docs/spec/13-push-notifications.md` 4.4.
- `docs/spec/15-client-architecture.md`: the `RunStore` and `FocusManager` rows, principle 1 (no permission logic in the client).
- `docs/spec/25-visualization.md`: 1 (boundary paragraph of 2026-10-24), 7.1 (row `lib/src/run_graph/`), 7.2, 7.3, 7.4, 10, R-VZ-14.
- `docs/design/components.md` section 2 (live box state colors and icons), `docs/design/tokens.md`.
- `docs/api/openapi.yaml`: `Run`, `RunStep`, `RunSummary`, `RunMetrics`, `listActiveRuns`, `getRun`, `listRunSteps`, `cancelRun`.
- Briefs: `docs/briefs/M4-16-sync-run-documents.md`, `M4-21-agent-interface.md`, `M4-23-search-switcher.md` (Running now), `M6-11-viz-package.md`, `M6-12-viz-views.md` (forms and goldens), `M2-51-run-graph-data.md`; merged code wins on names.
- Code on the base branch: `packages/bothy_viz` (marks, model, fixtures, harness), `packages/sync_engine` (run documents, focus), `apps/client/lib/running/`, the live box of `apps/client/lib` (M4-21).

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
9. Limits only through named keys: `ui.livebox_visible_steps`, `a11y.live_announce_min_interval` and `sync.max_focused_threads` from `AppConfig`, never a literal limit in code.
10. No permission logic in the client: the views show what the server returns; Stop only where the run's capabilities allow `run.cancel` (15 1).

Product requirements implemented here:

11. The derivation of 09 5.6 (root, nodes, `status` markers, summary rows, parent, merge rule, order and time, node content, limits, live) as one pure function over the generated `Run`; the client never estimates a cost share (D-RV-5).
12. D14: trace view with a shared time axis, collapsible rows (mouse and arrow keys), "n earlier steps" above `ui.livebox_visible_steps` finished children, never collapsing a running node or its ancestors; the button "Graph" in the live box and on every card.
13. D10: switch "Cards" and "List" (cards default); cards visible on the screen are focused through the `focus` member `background_thread_ids` (04 6.2, built by M2-51) within `sync.max_focused_threads` together with the threads already shown, and `idle` keeps following the window state of 15 section 10 (D-M4-30-3), so the cards change no notification rule of 13 4.4; a run graph opened from a card focuses its thread the same way; other cards show the `RunSummary` with "live details paused"; the stall notice from `stall_warned_at`.
14. "As list" for the graph (outline with tree semantics: level, position, state as text, duration, cost) and "List" for the control room; announcements only for the end of a node and state changes, at most once per `a11y.live_announce_min_interval`; reduced motion without animation (01 8).

## Scope

`packages/bothy_viz`:

- `lib/src/run_graph/`: `run_graph_model.dart` (the pure derivation and its node type), `run_graph_view.dart` (trace view with time axis and the list form), `run_card.dart` (control room card), texts through `l10n`; marks reused from `lib/src/marks/` (bar, run card), new marks only where none fits.
- `fixtures/`: `Run.<state>.json` for a flat run, a run with subagents and a background job, a run with a summary row, a stalled run and a large run (`load`), listed in `fixtures/README.md`; the harness shows the run graph and the cards from them.

`apps/client/lib`:

- `running/`: the card grid and the switch in `running_now_view.dart`, the focus of visible cards through the existing focus manager.
- A route for the run graph from the live box button and the cards; the button "Graph" in the live box of M4-21.
- ARB keys in `packages/l10n` for every new text.
- Fake server handlers (registry of M4-17) for the document shapes of M2-51 if missing.

Fragments: `docs/decisions.d/M4-39.md` (`D-M4-39-<n>`), changelog fragments of the touched components (`<component>/changelog.d/M4-39.md`, root `changelog.d/M4-39.md` otherwise); README of `packages/bothy_viz` and `apps/client`.

## Out of scope

- Server changes (M2-51); phone layouts (M5-14); the web console (no run views, D-RV-11); TV mode.
- Keyboard shortcuts beyond arrow keys inside the graph (M4-24 owns the shortcut set).
- A graph across runs, a replay slider, costs per node from an estimate.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-39-run-graph-desktop`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(bothy-viz): ...`, `feat(client): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the README and fragment files named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; the fake server replaces the server. Temporary directories only via `Directory.systemTemp.createTemp`, removed at the end. Time comes from an injected clock, never from a fixed wait.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH`; without it the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test or extend `quality/licenses/allowed.txt`; no charting library (R-VZ-10).
- `bothy_viz` keeps its dependencies (no `sync_engine`, no `local_store`); the app hands the document to the package.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M4 closed, in the manner of D-W-100 and D-W-114. Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Model first.** `RunGraphModel.fromRun(Run run, DateTime now)` returns the root node with children; every rule of 09 5.6 is a unit test. The model is recomputed from the document on every applied patch; keep it linear in the number of steps (map by ID, one pass for children, one for the merge rule).
2. **Time axis.** Start at `started_at` (else `queued_at`), end at `finished_at` or the injected now; the axis labels follow the length (seconds, minutes, hours). A running bar ends in an arrow; color, icon and word of the state come from the live box tokens.
3. **Virtualized rows.** The view builds only visible rows, so a run with `run.max_steps_persisted` steps stays smooth; the collapse rule of 09 5.6 item 7 applies before layout.
4. **Focus budget.** The control room computes the visible cards after layout and asks the focus manager for their threads as background threads; the focus manager sends them in `background_thread_ids`, keeps the threads the app already shows in `thread_ids` first and gives the rest of `sync.max_focused_threads` (both lists together) to cards in on-screen order; it never changes `idle` for them. A card without a slot shows "live details paused".
5. **Rulings for `docs/decisions.d/M4-39.md`** (`D-M4-39-<n>`): the axis labels, the focus order of cards, any mark added to `bothy_viz`.

## Tests (write them first)

- `packages/bothy_viz/test/run_graph/run_graph_model_test.dart`: root fields; `status` steps as markers; summary rows under their parent; unknown parent under the root until it arrives; merge rule with one and with two task children; order by `ordinal`; current tool after the merge; cost only from `detail.usage`; collapse above `ui.livebox_visible_steps` and never on the running path.
- `packages/bothy_viz/test/run_graph/goldens_test.dart`: trace view and cards from every fixture, light and dark, `comfortable`, `compact`, text scaling 1 and 2, `de` and `en`, and the context `reduced`.
- `packages/bothy_viz/test/run_graph/semantics_test.dart`: the outline list has tree semantics and the same values; announcements only on node end and state change, rate-limited.
- `apps/client/test/running/control_room_test.dart`: cards and list switch; Stop only with the capability; cards beyond the budget show "live details paused"; the focus request carries the card threads in `background_thread_ids`, not in `thread_ids`, and `idle` stays the window state (foreground window: `idle = false`).
- `apps/client/integration_test/run_graph_test.dart` against the fake server: a scripted run with a subagent and a background job grows live in the graph opened from the live box; the stall notice appears after the scripted `add /stall_warned_at`; a card opens the graph.

Run the added tests by name with ten repetitions (`--plain-name`) through the machine's test slot.

Commands that must pass, from the repository root:

- `cd packages/bothy_viz && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make api-checks`, `make docs`, `make fragments-check`
- `make check`

## Acceptance criteria

- [ ] The derivation follows 09 5.6 rule by rule, with a unit test per rule.
- [ ] D14 opens from the live box and from a card, grows live and keeps the tree after summaries; the list form has the same content.
- [ ] D10 shows cards by default, live where focused within the budget, "live details paused" elsewhere, the stall notice and Stop only where allowed.
- [ ] Goldens recorded for every fixture and context; semantics tests green.
- [ ] Rulings and fragments recorded; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01 or 09.
- `packages/bothy_viz/README.md` and `apps/client/README.md` updated.
- Rulings in `docs/decisions.d/M4-39.md`; changelog fragments as in Scope.
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
