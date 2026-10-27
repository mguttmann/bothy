# M5-15: Threads in the sidebar and new chat on iPhone and Android

Status: ready | Depends on: M4-41, M5-03 | Milestone: M5

## Goal

After this ticket the spaces screen M1 on iPhone and Android shows the open posts and threads nested under their channel with the same rules as the desktop sidebar (01 6.8 items 1 to 7, 10, 11): the same selection, bound, order without jumping rows, "All posts", markers and counters, the kind icons with their explanation, and the "+" action at every forum row, always visible in density `touch`. A tap on a thread row opens its history M3 directly; a tap on the forum name opens the post list M2 as before; "+" opens the new chat view of M4-41 in phone form (a full screen page with the input focused), and sending creates the post through the outbox. There is no pinned column on the phone. Why: the same model on every device (2026-10-27; [design note](../design/sidebar-sessions.md), D-SB-13).

## Read first

- `docs/design/sidebar-sessions.md` (all), `docs/decisions.d/sidebar-sessions.md`.
- `docs/PLAN.md`: section 3; section 11 rows M5-02, M5-03, M5-10, M5-15; section 10 rows M4-40 and M4-41.
- Dependencies: the header names M4-41 and M5-03; everything below them (M4-40, M5-02, M2-52, M4-24 and the M4 foundation) is implied.
- `docs/spec/01-product.md`: 5.3 (navigation stack, M1, M2, M3), 5.4 (Android), 6.8 (all), 8 (tree semantics, targets, text scaling up to 200 percent), 13 (R-PR-17, R-PR-18), 14 (`ui.sidebar_thread_window`, `ui.sidebar_threads_per_channel`).
- `docs/design/components.md` 7 (thread row, "+" action, density `touch`); `docs/design/spacing-radii-elevation-motion.md` (density `touch`, `density.target-min`).
- `docs/decisions.d/M4-40.md`, `docs/decisions.d/M4-41.md`, `docs/decisions.d/M5-02.md`, `docs/decisions.d/M5-03.md`.
- Briefs: `docs/briefs/M4-40-sidebar-threads.md`, `M4-41-sidebar-new-chat.md`, `M5-02-iphone-layout.md`, `M5-03-android-layout.md`, `M5-10-mobile-a11y.md`; merged code wins on names.
- Code on the base branch: the selection function and thread row of M4-40 (`apps/client/lib/sidebar/`), the new chat view and controller of M4-41 (`apps/client/lib/forum/`), the phone layouts (`apps/client/lib/phone/phone_pages.dart`, `phone_navigation.dart`, `phone_shell.dart`, `android/android_controls.dart`).

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
9. Limits only through named keys (`ui.sidebar_thread_window`, `ui.sidebar_threads_per_channel` from `AppConfig`); no new configuration key and no new personal setting key.
10. No permission logic in the client (15 1); the selection function and the new chat controller of M4-40 and M4-41 are reused, never copied.

Product requirements implemented here:

11. M1 shows the thread groups of 01 6.8 with the same selection, bound, order, deferred removal (while a finger is on the list or the screen reader focus is in it), "All posts", markers and counters as the desktop (D-SB-13).
12. Every forum row has "+" at the target size `density.target-min`, labelled as on the desktop; it opens the new chat view as a page with the input focused; system back (Android) and the edge swipe (iOS) leave it, and an empty draft is deleted.
13. A tap on a thread row opens M3 of that thread; a tap on "All posts" or on the forum name opens M2; the draft row opens the new chat view.
14. Group collapse per channel is kept per device (01 6.8 item 7); there is no pinned column and no Cmd+Shift+O on the phone.
15. VoiceOver and TalkBack read the tree with levels (space, channel, thread) and the labels of components.md 7; text scaling up to 200 percent without loss.

## Scope

`apps/client/lib`:

- `phone/`: the thread groups and the "+" action in the M1 page, the routes from a thread row to M3 and from "+" or the draft row to the new chat page, Android back handling.
- Reuse of `sidebar/` and `forum/` widgets of M4-40 and M4-41 in density `touch`; changes there only where the phone needs a parameter (for example the pointer rule of Design note 1).
- ARB keys in `packages/l10n` only for texts that differ on the phone.

Fragments: `docs/decisions.d/M5-15.md` (`D-M5-15-<n>`), root `changelog.d/M5-15.md` or the client's changelog fragment; README of `apps/client` (phone navigation).

## Out of scope

- Desktop behaviour (M4-40, M4-41); server or runner changes.
- iPad layout (V3).
- A synchronized collapse state.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-15-sidebar-threads-mobile`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package.
- Do not modify files outside the scope above, except the README and fragment files named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); device and simulator runs on the Mac-bound host only through the launcher's rules.
- Tests bind only `127.0.0.1` with port 0 and make no network call; the fake server replaces the server. Time comes from an injected clock, never from a fixed wait.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH`; without it the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M5 closed, in the manner of D-W-100 and D-W-120. Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **No pointer on touch.** The deferred removal of 01 6.8 item 4 uses "a finger is on the list or the list is scrolling" instead of the pointer, and the screen reader focus instead of the keyboard focus; the rule stays in the shared group widget of M4-40 as a parameter.
2. **One page for a new chat.** The new chat view of M4-41 is placed as a page of the navigation stack; its controller and draft handling are shared, so a draft started on the phone shows as the draft row on the desktop and back.
3. **Rulings for `docs/decisions.d/M5-15.md`** (`D-M5-15-<n>`): the touch rule of note 1; how back leaves the new chat page.

## Tests (write them first)

Widget and golden tests (`flutter test`, in-memory `local_store`, fake clock):

- `test/phone/spaces_threads_test.dart`: M1 with thread groups, "All posts", the "+" action at `density.target-min`; a tap on a thread row opens M3; a tap on the forum name opens M2; the deferred removal while a finger is on the list.
- `test/phone/new_chat_page_test.dart`: "+" opens the page with the focus in the input; back leaves it and deletes an empty draft; sending creates the post.
- `test/goldens/phone/m1_threads_*`: density `touch`, light and dark, text scaling 1 and 2, `de` and `en`, iPhone and Android chrome.

Integration tests (`integration_test/`, against `FakeServer`, on the iPhone and Android layouts):

- `phone_threads_test.dart`: after connect M1 shows the active posts of every forum without a list request per channel; a post that Sam creates appears; "+", type, send creates the post and its history opens.
- Semantics snapshot of the M1 tree (levels and labels) for VoiceOver and TalkBack.

Run the added tests by name with repetitions as the team rules require (Dart `--plain-name`, 10 repetitions), through the machine's test slot.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- `make apps-platform`; otherwise its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make docs`, `make fragments-check`
- `make check`

## Acceptance criteria

- [ ] M1 shows the thread groups with the rules of 01 6.8 on iPhone and Android.
- [ ] A thread row opens M3, the forum name opens M2, "+" opens the new chat page and sending creates the post.
- [ ] Goldens in density `touch`, light and dark, text scaling 1 and 2, `de` and `en`; semantics snapshot of the tree.
- [ ] Rulings recorded; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01 6.8 or M1.
- `apps/client/README.md` (English) as in Scope.
- Rulings in `docs/decisions.d/M5-15.md`; changelog fragment as in Scope.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the integration tests ran (simulator, emulator or CI).
5. What is not proven (for example a check with VoiceOver or TalkBack done by hand only).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
