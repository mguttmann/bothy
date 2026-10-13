# M4-04: Design system with tokens, theme and fonts (ADR 0030)

Status: ready | Depends on: M4-01, M4-03 | Milestone: M4

## Goal

After this ticket `packages/design_system` is the one place where the apps and the console get their look: the generated token classes of `design/build/dart/design_tokens.g.dart` (including the accent variants of M4-03) as a checked copy inside the package, a theme builder that turns the four inputs theme, density, motion and accent into a Flutter `ThemeData` with the token extensions, a pure resolver from the personal settings (`appearance.theme`, `appearance.density`, `appearance.reduce_motion`, 01 11) and the system values to those inputs, the pinned fonts Atkinson Hyperlegible Next and Mono with their license, and the golden test setup (alchemist, CI goldens with the Ahem font; platform goldens only on macOS) that every later widget ticket uses. A widget test switches light and dark, the three densities and both motion contexts, and a golden of a sample page pins the result. Why: components (M4-07), the terminal view (M4-35), the visualizations (M6) and both apps must read colors, type, spacing and motion only from the tokens (ADR 0030, design README principle 7), and the golden matrix of the exit criteria needs one shared setup.

## Read first

- `docs/PLAN.md`: section 3; section 5 (row "Golden"); section 10 (Exit criteria item 1, Test strategy, rows M4-04, M4-05, M4-07, M4-18, M4-35).
- `docs/design/README.md` (all), `docs/design/tokens.md`, `docs/design/typography.md` ("Integration": `DsType`, bundled variable TTF files), `docs/design/spacing-radii-elevation-motion.md`, `docs/design/components.md` ("Common rules for all components"), `docs/design/palette.md`.
- `docs/spec/15-client-architecture.md`: 3, 3.1, 4 (row "settings, theme, density, motion"), 20 (rows "Widget" and "Golden": alchemist, CI goldens with Ahem, platform goldens only on macOS).
- `docs/spec/01-product.md`: 8 "Accessibility" (rows "Font size", "Motion", "Contrast"), 11 "Personal settings" (`appearance.*`), 12 (accent).
- ADRs: `0030-design-tokens-as-one-source-with-a-generator.md` (Confirmation: golden tests use only the generated classes), `0027`, `0044`.
- `tools/design-tokens/README.md` (the Dart output is not formatted and was checked only against stand-ins; this ticket compiles it against the real SDK), `tools/design-tokens/tokens.py` (`generate`), `tools/design-tokens/vendor/MANIFEST.json`.
- `docs/quality/licenses.md` (fonts under OFL-1.1; NOTICE), `quality/licenses/allowed.txt`, `NOTICE`.
- Briefs: `docs/briefs/M4-01-workspace-arch.md` (member rules, generated patterns), `docs/briefs/M4-03-accent-variants.md` (`DsAccent`).
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
9. Limits and thresholds only through named values; the token values are the only source of sizes, colors, durations and curves (no literal in widget code except `0`). This ticket adds no configuration key.
10. Specification first: `docs/design/typography.md` "Integration" and `tools/design-tokens/README.md` "Outputs" describe the package copy and the fonts in the same change.

Product requirements implemented here:

11. Tokens only through the generated classes (`DsColors`, `DsAccent`, `DsElevation`, `DsDensity`, `DsMotion` as `ThemeExtension`s; `DsSpace`, `DsRadius`, `DsBorderWidth`, `DsLayout`, `DsSize`, `DsIconSize`, `DsFonts`, `DsType` as constants) (ADR 0030, tool README "Outputs").
12. Theme `light` and `dark`; density `comfortable`, `compact`, `touch` (mobile fixed to `touch`, `compact` only on desktop); motion `full` and `reduced`, where the system setting "Reduce motion" and `appearance.reduce_motion` select `reduced` (01 8, 01 11).
13. Fonts: Atkinson Hyperlegible Next and Mono, variable TTF from the source repositories in a pinned version with the OFL text bundled; the fallback list of `font.family.*` stays (typography.md).
14. Text scaling up to 200 percent through `MediaQuery.textScaler`; type roles only, never `font.size.*` directly (typography.md "Roles").
15. Golden tests with alchemist: CI goldens with Ahem on every machine, platform goldens only on macOS (15 20).

## Scope

Create or fill in `packages/design_system/`:

- `lib/src/generated/design_tokens.g.dart`: the copy of the generator output (Design note 1).
- `lib/src/theme/ds_context.dart` (`DsContext`, `DsThemeMode`, `DsDensityMode`, `DsMotionMode`), `lib/src/theme/ds_settings_resolver.dart`, `lib/src/theme/ds_theme.dart`, `lib/src/theme/ds_theme_scope.dart` (Design notes 2 to 4).
- `fonts/AtkinsonHyperlegibleNext[wght].ttf`, `fonts/AtkinsonHyperlegibleNext-Italic[wght].ttf`, `fonts/AtkinsonHyperlegibleMono[wght].ttf`, `fonts/AtkinsonHyperlegibleMono-Italic[wght].ttf` (only the styles the source repositories ship as variable files), `fonts/<family>.OFL.txt` for each family (Design note 5), `fonts/MANIFEST.json` (source URL, commit, SHA-256 per file) (Design note 5).
- `lib/design_system.dart` exports; `lib/testing.dart` with the golden helpers (Design note 6).
- `test/flutter_test_config.dart`, `test/theme_switch_test.dart`, `test/settings_resolver_test.dart`, `test/tokens_copy_test.dart`, `test/fonts_test.dart`, `test/goldens/sample_page_test.dart` with its golden files.

Change:

- `tools/design-tokens/tokens.py` (`generate`): also writes the package copy; `check` compares it (Design note 1); `tools/design-tokens/tests/test_gate.py` gains the case.
- `packages/design_system/pubspec.yaml` (fonts, `alchemist` as a dev dependency), `README.md`.
- `NOTICE` (font attribution), `docs/design/typography.md` "Integration", `tools/design-tokens/README.md` "Outputs", `docs/quality/licenses.md` (a section "Bundled fonts", Design note 5; no new row in the table of allowed licenses).
- `docs/DECISIONS.md` section 10: rows `D-M4-04-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Icons and `DsIcons` rendering: M4-05 (the generated `DsIcons` constants compile here, glyph rendering comes there).
- Components: M4-07. Localized texts: M4-06 (the sample page uses no text or only test strings in the test file).
- Reading `user_settings` from the local database and `branding.accent_color` from the instance: M4-18 and M4-25 feed the resolver.
- Changes to the token values.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-04-design-system-theme`; one writer per worktree.
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
- Network only for fetching the four font files once at a recorded commit (Design note 5).

## Design notes

1. **Token copy.** Ruling for docs/DECISIONS.md: `tokens.py build` writes the Dart output twice, to `design/build/dart/design_tokens.g.dart` (unchanged) and to `packages/design_system/lib/src/generated/design_tokens.g.dart` (same bytes), and `tokens.py check` compares both; reason: a package may not import a file outside its folder (15 3.1 rule 2), and one generator writing both copies keeps them equal without a second tool. The copy is generated code (M4-01 patterns): no doc lint, no format check, no coverage; it must compile without errors under `flutter analyze` against the real SDK. If it does not compile (the tool README says it was checked only against stand-ins), fix the generator (`tokenkit/emit_dart.py`) and its tests, never the copy. `test/tokens_copy_test.dart` asserts that the copy exists and that `DsColors`, `DsAccent`, `DsDensity`, `DsMotion` and `DsElevation` each have instances for all their contexts.

2. **Context and resolver.** `DsContext({required DsThemeMode theme, required DsDensityMode density, required DsMotionMode motion, required String accent})` (value type with `==` and `hashCode`; `accent` a key of `DsAccent`). `DsContext resolveDsContext({required String? appearanceTheme, required String? appearanceDensity, required String? appearanceReduceMotion, required Brightness platformBrightness, required bool platformReduceMotion, required bool isTouchPlatform, required String? accentKey})`: `appearance.theme` `system` or absent follows `platformBrightness`; `appearance.density` applies only on desktop, a touch platform is always `touch`, absent means `comfortable`; `appearance.reduce_motion` `on` gives `reduced`, `off` gives `full`, `system` or absent follows `platformReduceMotion`; an unknown value of any key falls back to the default of 01 11 (the server rejects unknown values, but a newer server may add one; 06 13); an unknown or absent accent gives `spruce`. Pure function, no Flutter widget import beyond `Brightness` from `dart:ui` (the package is a Flutter package).

3. **Theme.** `ThemeData buildDsTheme(DsContext context)`: `useMaterial3: true`, `brightness`, a `ColorScheme` mapped from `DsColors` and `DsAccent` (primary from `color.accent.bg`, onPrimary from `color.text.on-accent`, surface from `color.surface.canvas`, error from `color.danger.bg`, outline from `color.border.default`; record the full mapping in the package README), `extensions: [DsColors.<theme>, DsAccent.of(accent, brightness), DsElevation.<theme>, DsDensity.<density>, DsMotion.<motion>]`, `textTheme` from `DsType` roles with the font families of `DsFonts`, `visualDensity` from the density, `pageTransitionsTheme` without motion in `reduced`, and the focus ring from `color.focus.ring` with `border-width.focus` (components.md: focus always visible). Accessors `context.dsColors`, `context.dsAccent`, `context.dsDensity`, `context.dsMotion`, `context.dsElevation` as an extension on `BuildContext` that throw a `StateError` naming the missing extension when the theme was not built with `buildDsTheme`.

4. **Scope widget.** `DsThemeScope({required DsContext dsContext, required Widget child})` builds the theme with `AnimatedTheme` whose duration is `motion.duration.fade` (zero in `reduced`), so a switch of theme or density is visible but never animated under reduced motion. No state of its own: the caller passes a new `DsContext`.

5. **Fonts.** Fetch the variable TTF files and the OFL texts from `https://github.com/googlefonts/atkinson-hyperlegible-next` and `https://github.com/googlefonts/atkinson-hyperlegible-next-mono` at the commit of the default branch on the build day, at the font version that `tools/design-tokens/vendor/MANIFEST.json` names for the WOFF2 files (5.3.0 per typography.md) or the nearest release tag of that version; record URL, commit and SHA-256 in `fonts/MANIFEST.json`; `test/fonts_test.dart` recomputes the SHA-256 of every file and fails on a mismatch, a missing or an extra file. Every file must stay below `repo.max_file_kb`; if a variable file exceeds it, stop with `Verdict: BLOCKED font file over repo.max_file_kb` instead of subsetting by hand. `pubspec.yaml` declares the families `Atkinson Hyperlegible Next` and `Atkinson Hyperlegible Mono` with the files (variable weights through `FontWeight` axis use). `NOTICE` names both fonts with copyright and OFL-1.1. The font files are bundled assets, not package dependencies: `make licenses` (OSV-Scanner over lock files) does not see them, and `quality/licenses/allowed.txt` stays unchanged. Their license is settled by D-L11 and typography.md 5 (OFL-1.1, license text shipped with the files): bundle the OFL texts next to the files (`fonts/<family>.OFL.txt`), record `"license": "OFL-1.1"` per file in `fonts/MANIFEST.json`, and let `test/fonts_test.dart` also fail when a font file has no license text or a manifest entry names another license. Add a section "Bundled fonts" to `docs/quality/licenses.md` that states this rule with D-L11 (outside the table that its test compares with `allowed.txt`). Ruling for docs/DECISIONS.md.

6. **Golden setup.** `test/flutter_test_config.dart` loads the bundled fonts for platform goldens and sets `AlchemistConfig` with `ciGoldensConfig` enabled everywhere and `platformGoldensConfig` enabled only when `Platform.isMacOS` (15 20); golden files under `test/goldens/ci/` and `test/goldens/macos/`. `package:design_system/testing.dart` exports `dsGoldenMatrix({required String name, required Widget Function() builder, Iterable<DsThemeMode> themes, Iterable<DsDensityMode> densities, Iterable<double> textScales, Iterable<Locale> locales})` that builds one alchemist `GoldenTestGroup` with a scenario per combination, wrapped in `DsThemeScope` and a `MediaQuery` with the text scaler; later tickets call it. Ruling for docs/DECISIONS.md: test helpers in a separate library `testing.dart`, because several members need the same matrix and test folders cannot be shared between packages; `alchemist` stays a dev dependency of every member that imports `testing.dart` in its tests, and `testing.dart` itself imports only `flutter_test` types through the caller (it takes `alchemist` as a dependency only if `testing.dart` cannot be written without it; then `alchemist` becomes a regular dependency of `design_system` with the reason in the ruling). The README states how to regenerate goldens (`flutter test --update-goldens`) and that platform goldens are updated only on macOS.

7. **Rulings for docs/DECISIONS.md** (`D-M4-04-<n>`): the token copy written by the generator (note 1); the resolver's fallbacks for unknown values (note 2); the color scheme mapping (note 3); the font source and version (note 5); `testing.dart` and the alchemist dependency (note 6).

## Tests (write them first)

`packages/design_system/test/` (run by `flutter test --coverage`):

- `settings_resolver_test.dart`: one case per row: `system` with dark platform gives dark; `light` overrides a dark platform; touch platform with `compact` gives `touch`; desktop absent density gives `comfortable`; `on`, `off`, `system` with both platform values; unknown values (`"neon"`, `"tiny"`, `"sometimes"`) give the defaults; unknown accent `"purple"` gives `spruce`.
- `theme_switch_test.dart`: a widget reads `context.dsColors.textPrimary` (or the generated field name) and `context.dsDensity.controlHeight`; pumping with light then dark changes the color to the token value of each theme; `comfortable`, `compact`, `touch` give the three token heights; `reduced` gives a zero `AnimatedTheme` duration and `full` the token `motion.duration.fade`; the `loch` accent changes `context.dsAccent` to the `loch` variant; a widget under a plain `ThemeData` gets the `StateError`.
- `tokens_copy_test.dart` (Design note 1).
- `fonts_test.dart` (Design note 5) and `font family resolves`: a `Text` with `DsType.body` uses the family `Atkinson Hyperlegible Next`.
- `goldens/sample_page_test.dart`: a sample page (a header in `type.title`, a body paragraph, a code line in `type.code`, a primary and a secondary button built from plain Material buttons with the theme, a raised surface with `radius.md`) through `dsGoldenMatrix` in light and dark, `comfortable` and `touch`, text scale 1 and 2.
- `tools/design-tokens/tests/test_gate.py`: `test_check_fails_when_package_copy_differs`.

Commands that must pass, from the repository root:

- `uv run tools/design-tokens/tokens.py check`, `make tokens`
- `cd packages/design_system && flutter analyze --fatal-infos && flutter test --coverage`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`
- `make check`

## Acceptance criteria

- [ ] `flutter analyze` is clean for `design_system`, and the generated token file compiles against the real Flutter SDK.
- [ ] A widget test switches light and dark, the three densities, both motion contexts and an accent, and reads the token values.
- [ ] A golden of a sample page exists in light and dark, `comfortable` and `touch`, text scale 1 and 2, as CI golden (and as platform golden on macOS).
- [ ] The fonts are bundled from a pinned source with the OFL text and SHA-256; `NOTICE` names them; `make licenses` is green.
- [ ] `tokens.py check` fails when the package copy differs from the generator output.
- [ ] Rulings recorded; typography.md and the tool README updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `packages/design_system/README.md` (English): what the package provides, the color scheme mapping, the resolver table, how to update goldens and tokens.
- Rulings of Design note 7 in `docs/DECISIONS.md` section 10; `docs/design/typography.md`, `tools/design-tokens/README.md`, `NOTICE` updated.
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
4. Font source commits and SHA-256, file sizes; the alchemist and Flutter versions; the number of golden files; whether platform goldens were produced (macOS only).
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
