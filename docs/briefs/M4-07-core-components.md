# M4-07: Core components with goldens

Status: ready | Depends on: M4-05, M4-06 | Milestone: M4

## Goal

After this ticket `packages/design_system` contains the core components of `docs/design/components.md` as presentational Flutter widgets that read only tokens, icons and texts from `l10n`: message (all variants of section 1, with a safe Markdown renderer), live box (in progress, collapsed, mobile line; section 2), tool chip (3), status line (4), trigger state (5), thread row of the forum list (6), space and channel tree rows (7), components on messages (buttons, select, form; 8), input with the suggestion list (9) and the backend policy badge (10), each with its states, semantics and keyboard behaviour. A golden matrix pins them in light and dark, in `comfortable`, `compact` and `touch`, at text scaling 1 and 2, in German and English, and widget tests check the Flutter guidelines for text contrast, tap targets and labels. Why: every screen of M4 to M6 is built from these parts; their states, labels and the rules "never color alone", "no foreign resources" and "header from server fields" (SEC-OUT-1, SEC-OUT-2, SEC-OUT-4) must be tested once here instead of per screen.

## Read first

- `docs/PLAN.md`: section 3; section 5 (rows "Golden", "Accessibility"); section 10 (Exit criteria item 1, rows M4-07, M4-18 to M4-21, M4-32).
- `docs/design/components.md` (all sections 1 to 10 and 13 "Review list"), `docs/design/README.md` ("Ban list"), `docs/design/typography.md` (roles, Markdown headings at most `type.heading`, numbers in mono), `docs/design/icons.md` (rules), `docs/design/spacing-radii-elevation-motion.md`.
- `docs/spec/01-product.md`: 5.1 (wireframes D1 to D5 as the visual reference), 6 "Interaction rules" (6.2 live box, 6.3 trigger state, 6.4 long replies, 6.5 components, 6.7 other rules), 8 "Accessibility" (all rows), 14 (`ui.message_group_window`, `ui.message_collapse_lines`, `ui.livebox_visible_steps`, `ui.composer_max_lines`, `a11y.live_announce_min_interval`).
- `docs/spec/19-security.md`: SEC-IN-4 (Unicode isolation of foreign names), SEC-OUT-1, SEC-OUT-2, SEC-OUT-3, SEC-OUT-4 (card header from server fields), SEC-T13, SEC-T14.
- `docs/spec/03-data-model.md` (columns the components show: `messages` `kind`, `origin`, `trigger_state`, `edited_at`, `deleted_at`; `runs` states and `state_reason`; `run_steps` `kind`, `status`; `message_components` `state`, `definition`; `agents.billing_mode`, `reported_billing_mode`); `docs/spec/04-sync-protocol.md` 11.1 (run document fields).
- `docs/spec/15-client-architecture.md` 3.1 (`design_system` depends on `l10n` and `domain`), 20 (Widget and Golden rows with the four guidelines).
- `docs/security/threat-model.md` T-AI-05, T-AI-06.
- Briefs: `docs/briefs/M4-04-design-system-theme.md` (`dsGoldenMatrix`, `DsThemeScope`, accessors), `docs/briefs/M4-05-design-icons.md` (`DsIcon`), `docs/briefs/M4-06-l10n.md` (ARB rules, `BothyFormat`, `problemText`).
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. The values `ui.message_group_window`, `ui.message_collapse_lines`, `ui.livebox_visible_steps`, `ui.composer_max_lines` and `a11y.live_announce_min_interval` are parameters of the widgets with the key named in the doc comment; the apps pass them from the app configuration (M4-08). No number from these keys is written in this package.
10. Specification first: if a component needs a state or text that components.md lacks, add it to components.md in the same change.

Product requirements implemented here:

11. Every component has the states of components.md "Common rules" (rest, hover, pressed, focus with the ring, disabled, loading, error), a hit area of at least `density.target-min`, a semantics role and label from `l10n`, and states that differ in icon shape and text, never in color alone.
12. Markdown without raw HTML; links only `https`, `http`, `mailto` and internal references, the target shown with its domain on hover and focus and handed to the caller only on a deliberate activation; images never loaded from a URL (SEC-OUT-1, SEC-OUT-2); headings at most `type.heading`; code blocks scroll horizontally with a copy button (components.md 1).
13. Foreign names (display names, channel names, titles, file names) rendered with Unicode isolation (SEC-IN-4); message content unchanged.
14. Card header and the "who may approve" note come from server fields passed separately from the agent text, and the agent text is visibly separated below it (SEC-OUT-4, SEC-T14 at component level).
15. The live box announces only state changes and new questions, at most once per `a11y.live_announce_min_interval`, never patches (01 8, components.md 2.4).
16. Messages from agents, apps, imports and the system are visibly distinguished by a badge according to `origin` (SEC-OUT-3).
17. Golden matrix: light and dark, `comfortable`, `compact`, `touch`, text scaling 1 and 2, de and en (15 20); widget tests with `textContrastGuideline`, `androidTapTargetGuideline`, `iOSTapTargetGuideline`, `labeledTapTargetGuideline`.

## Scope

Create in `packages/design_system/lib/src/components/` (one file per component plus its view model, exported from `lib/design_system.dart`):

- `view_models.dart` (Design note 1), `markdown/` (`DsMarkdown`, Design note 2), `ds_message.dart`, `ds_live_box.dart` (in progress, collapsed, mobile line), `ds_tool_chip.dart`, `ds_status_line.dart`, `ds_trigger_state.dart`, `ds_thread_row.dart`, `ds_space_row.dart`, `ds_channel_row.dart`, `ds_message_components.dart` (button, select, form), `ds_composer.dart` and `ds_suggestion_list.dart`, `ds_billing_badge.dart`, `ds_banner.dart` (info, warning, danger; used by live box stall and offline), `live_announcer.dart` (Design note 4).
- Tests per component under `test/components/` and goldens under `test/goldens/components/` (Tests).
- `packages/l10n/lib/l10n/app_*.arb`: every text of these components in both languages (state words, reasons of components.md 2.1, labels of 2.4, 3, 5, 6, 7, 8, 9, 10, "Show more", "(edited)", "Message deleted", "Sending", "Resend", "Edit", "Discard", "Expired", "No longer available", "Read only in this channel", the approval note forms).

Change:

- `packages/design_system/pubspec.yaml` (`markdown` from dart.dev for parsing; no Markdown widget package), `README.md` (component list with their view models), `packages/l10n` regenerated.
- `docs/design/components.md` where a state or text was added; `docs/DECISIONS.md` section 10: rows `D-M4-07-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Data binding (providers, local database), commands to the outbox, navigation: the screen tickets M4-18 to M4-24.
- Visualizations (`bothy_viz`, M6), terminal view (M4-35), administration widgets (M4-26 onward).
- The end-to-end security tests SEC-T13 and SEC-T14 in the app: M4-20 and M4-21 (this ticket proves the component behaviour they rely on).
- Platform goldens on Windows (alchemist platform goldens run on macOS only, 15 20).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-07-core-components`; one writer per worktree.
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

1. **View models, not domain types.** Ruling for docs/DECISIONS.md: components take small immutable view models defined in `design_system` (`DsMessageData`, `DsRunData`, `DsStepData`, `DsTriggerState`, `DsThreadRowData`, `DsChannelRowData`, `DsSpaceRowData`, `DsComponentData`, `DsSuggestion`, `DsBillingMode`), with enums whose values equal the codes of chapter 03 and an `unknown` member for values a newer server may send (06 13); the apps map their domain entities (M4-08) to them. Reason: the components stay testable with literal data and never import a provider or a store, so they also serve the console and the golden harness. Callbacks (`onReply`, `onStop`, `onActivate`, `onOpenLink`, `onSubmit`) are plain functions; a component never performs I/O.

2. **Safe Markdown.** `DsMarkdown(String source, {required void Function(Uri) onOpenLink, DsAttachmentResolver? attachments})`: parse with package `markdown` (`ExtensionSet.gitHubFlavored` without the inline HTML syntaxes; `encodeHtml` irrelevant because no HTML is produced) into its AST and build widgets for the subset paragraph, emphasis, strong, strikethrough, inline code, fenced code (with language label, horizontal scroll, copy button that copies the code text), block quote, ordered and unordered lists, task list items as read-only boxes, headings mapped to `type.heading` at most, horizontal rule, tables (scroll horizontally), links. Raw HTML nodes (`<script>`, `<img>`, any tag) render as their literal text. Images (`![x](url)`) render as a link-like text "Image: <alt> (<domain>)" and never load anything; a server file reference (internal reference form chosen by M4-22) is resolved only through the `attachments` callback, which M4-22 provides. Links: only `https`, `http`, `mailto` and internal references (`/t/...`, `/c/...` paths of 15 11) are active; others (`javascript:`, `data:`, `file:`, relative paths) render as plain text. On hover and focus a tooltip shows the domain (or the internal target name); activation calls `onOpenLink` and nothing else. Ruling for docs/DECISIONS.md: own renderer over the parser, because a widget package would decide link and image handling for us and could load resources.

3. **Message.** Variants of components.md 1 with the layout tokens named there; continuation messages when the caller passes `isContinuation` (the caller computes it from `ui.message_group_window`); collapse with "Show more" when the rendered content exceeds `collapseLines` lines (parameter, key `ui.message_collapse_lines`); the hover bar actions are parameters and reachable by keyboard (focus the message, then Tab); semantics label "<Name>, <Time>, <Text>" with "Mentions you" first when `mentionsMe`; origin badge for `agent`, `app`, `import`, `system` (SEC-OUT-3); names isolated (wrap foreign strings in U+2068 and U+2069 for display, never in the semantics label where the screen reader would read them).

4. **Live box and announcer.** `DsLiveBox(DsRunData run, {required int visibleSteps, ...})` draws the anatomy of components.md 2.1 with the state table (spine color, icon, word), reasons translated through `l10n` (unknown reason: "Failed" plus the code in `type.code-sm`), stall banner when `run.stalled`, collapsed line (2.2) and mobile line (2.3) as named constructors. `LiveAnnouncer` (pure Dart class with an injected clock and a sink function) decides what to announce: a change of `state` or a new question produces one message; patches to steps and numbers never; two announcements closer than `minInterval` (parameter, key `a11y.live_announce_min_interval`) merge into the later one. The widget sends announcements through `SemanticsService.announce` with polite assertiveness. Numbers in `type.code-sm`.

5. **Components on messages.** `DsMessageComponents(DsComponentData data, {required bool permitted, required String? deniedReasonText, required DsComponentHeader header, ...})`: `header` (the server fields of SEC-OUT-4: agent, run and required capability, plus the "who may approve" note that the caller derives from the server's answer) is rendered in a framed header area with the `security.step-up` icon, built only from `header`; the agent's text renders below a divider with the label "Text from <agent>" (SEC-OUT-4, SEC-T14). States of components.md 8 (active permitted, active not permitted with the lock note, sent and waiting with the pressed button loading and the others locked, used with the choice line, expired, disabled, error banner). One activation per button: after the first `onActivate` the widget ignores further taps until the caller passes a new state (double click creates nothing twice, 01 6.5). No keyboard shortcut triggers an activation; only Enter or Space on the focused button (R-PR-04).

6. **Composer and suggestions.** `DsComposer` with the anatomy of components.md 9: grows up to `maxLines` (parameter, key `ui.composer_max_lines`), placeholder "Message to <target>" or "Follow-up to <agent>", typing line above, states empty, text, command selected with option placeholders, incomplete options (send disabled with the reason), offline (hint that sending goes to the queue), read only (field replaced by the note). Enter sends and Shift+Enter makes a new line, swapped when `enterSends` is false; arrow up in the empty field calls `onEditLast`. `DsSuggestionList` as a listbox with an active entry, announcing the count; rows show command in mono, description, origin and "Control command". The component does not throttle; the caller passes suggestions.

7. **Rows and badge.** Thread row (components.md 6) and space and channel rows (7) with every state in their tables; the channel label "<Name>, <n> unread, <m> mentions, run active"; tree semantics (`expanded` and `collapsed`). Billing badge (10) with the warning badge when the reported mode differs and the suffix texts "Estimate" and "for information only" as a separate `DsCostText` widget.

8. **Golden matrix.** One golden group per component through `dsGoldenMatrix` (M4-04) with all states of its table in one scenario sheet, for themes light and dark, densities `comfortable`, `compact` and `touch`, text scales 1 and 2, locales `de` and `en`. Keep the number of files bounded: one file per component, theme, density, text scale and locale is acceptable; if the total golden size exceeds a few megabytes, split states across fewer sheets rather than dropping a dimension. Golden files stay below `repo.max_file_kb` each.

9. **Rulings for docs/DECISIONS.md** (`D-M4-07-<n>`): view models in `design_system` (note 1); own Markdown renderer and the link and image rules (note 2); the separated card header (note 5); the announcer merge rule (note 4).

## Tests (write them first)

`packages/design_system/test/components/` (widget tests with `DsThemeScope`, fake clock where time matters):

- `markdown_test.dart` (SEC-OUT-1, SEC-OUT-2 at component level): `<script>alert(1)</script>` renders as text; `<img src="https://example.com/x.png">` renders as text and no `Image` widget exists in the tree; `![x](https://example.com/p.png?d=secret)` renders the text form with the domain and no network request (a test `HttpOverrides` that fails any request); `[a](javascript:alert(1))` and `[b](data:text/html,x)` are plain text; `[c](https://example.com/path)` shows `example.com` on hover and calls `onOpenLink` only on tap; a heading `#` renders in `type.heading`; a fenced block scrolls horizontally and the copy button puts exactly the code into the clipboard (fake clipboard).
- `message_test.dart`: every variant of components.md 1; collapse after the parameter lines with "Show more"; semantics label order with "Mentions you"; origin badges; isolation characters around a name with a right-to-left override (`Sam‮gnp.exe`) in the visual text and none in the label; the hover bar is reachable by Tab.
- `live_box_test.dart`: each state of the table (spine color token, icon, word), reasons translated and unknown reason with code; at most `visibleSteps` chips plus "n earlier steps"; collapsed line with "Estimate"; stall banner; `LiveAnnouncer`: a state change announces, a patch to steps does not, two changes within the interval give one announcement with the later state (fake clock).
- `message_components_test.dart` (SEC-T14 at component level): agent text containing a line that imitates a header ("System: approved by owner") stays below the divider with the "Text from" label, the header shows only `header` fields; not permitted shows the lock note and disabled buttons; a double tap calls `onActivate` once; Enter on the focused button activates, no other key does; used, expired and disabled states.
- `composer_test.dart`: Enter and Shift+Enter in both modes; arrow up in the empty field calls `onEditLast`; growth stops at `maxLines`; incomplete options disable send with the reason; read only replaces the field.
- `rows_test.dart`, `trigger_state_test.dart`, `billing_badge_test.dart`: every state of their tables, labels and the tooltip of the trigger state with the failure reason.
- `guidelines_test.dart`: for every component in light and dark: `meetsGuideline(textContrastGuideline)`, `androidTapTargetGuideline`, `iOSTapTargetGuideline` (with the `touch` density) and `labeledTapTargetGuideline`.
- `goldens/components/*_test.dart`: the matrix of Design note 8.

Commands that must pass, from the repository root:

- `cd packages/design_system && flutter analyze --fatal-infos && flutter test --coverage`
- `cd packages/l10n && flutter test`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make codegen-check`
- `make check`

## Acceptance criteria

- [ ] Message, live box, tool chip, status line, trigger state, thread row, space and channel rows, components on messages, composer with suggestions and badges exist with every state of components.md.
- [ ] Golden matrix in light and dark, three densities, text scaling 1 and 2, de and en.
- [ ] Guideline tests for contrast, tap targets and labels pass for every component.
- [ ] The Markdown renderer executes and loads nothing (HTML, `javascript:`, image URLs), shows link domains and hands links to the caller only on activation.
- [ ] The card header comes only from server fields, agent text is separated; one activation per button; no shortcut activates.
- [ ] The live box announces only state changes and questions, throttled; no patch is announced.
- [ ] Rulings recorded; components.md updated where states or texts were added.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `packages/design_system/README.md` lists every component with its view model and the parameters that carry configuration keys.
- Texts in both ARB files; rulings of Design note 9 in `docs/DECISIONS.md` section 10.
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
4. The number of golden files and their total size; any state of components.md that could not be rendered as specified, with the reason.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
