# M4-05: Icons in the design system

Status: ready | Depends on: M4-04 | Milestone: M4

## Goal

After this ticket every semantic icon of `design/tokens/icon.tokens.json` renders in Flutter from `packages/design_system`: the Phosphor glyphs of the pinned `@phosphor-icons/core` version, integrated without the dormant `phosphor_flutter` package, either as an icon font with only the used glyphs or as compiled vector assets (`vector_graphics_compiler`), chosen after a measured comparison of size and sharpness (docs/design/icons.md "Integration without an outdated package"). A widget `DsIcon` draws a `DsGlyph` from `DsIcons` at the token sizes, applies the spin rule (`trigger.working`, `run.running`, `state.reconnecting` only in the `full` motion context) and always carries a semantics label or is excluded from semantics when decorative. A golden shows all semantic icons in both weights they are used in, the asset size is measured and documented, and the gate fails when a mapped glyph is missing from the assets or an asset is not current. Why: components (M4-07), the terminal view and the visualizations must ask for `trigger.working`, never for `circle-notch` (icons.md "Rules"), and the ban on emoji and outdated packages must hold in code.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-05 and M4-07.
- `docs/design/icons.md` (all: choice, integration, rules, mapping, space icons), `docs/design/README.md` ("Ban list": emoji as icons; "Process for a change": new icon), `docs/design/components.md` (common rules: never color alone, semantics labels), `docs/design/spacing-radii-elevation-motion.md` (motion rules).
- `design/tokens/icon.tokens.json` (sizes, `ds.icons` mapping, `space-icons`), `design/build/tokens.resolved.json` (icon mapping), `design/build/dart/design_tokens.g.dart` (`DsGlyph`, `DsIconWeight`, `DsIcons`, `DsIconSize`).
- `tools/design-tokens/README.md` ("vendor/", `tokens.py vendor`), `tools/design-tokens/vendor/MANIFEST.json`, `tools/design-tokens/vendor/phosphor/` (the used SVGs and the MIT license).
- `docs/spec/15-client-architecture.md` 20 (Golden), `docs/spec/01-product.md` 8 (rows "Color", "Motion", "Screen reader").
- `docs/quality/licenses.md`, `NOTICE`.
- Briefs: `docs/briefs/M4-04-design-system-theme.md` (`dsGoldenMatrix`, `DsThemeScope`, fonts manifest pattern).
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
9. Sizes only from `icon.size.*` (`DsIconSize`); spin duration from `motion.duration.*`; no new configuration key.
10. Specification first: `docs/design/icons.md` records the chosen integration and the measured sizes in the same change.

Product requirements implemented here:

11. Icons only through semantic names (`DsIcons`); the glyph and its weight come from the token mapping; swapping a glyph is a change in `icon.tokens.json` only (icons.md "Rules").
12. Weights `regular`, `bold`, `fill` as mapped; `fill` for active and reached final states (icons.md).
13. Spinning only for `trigger.working`, `run.running` and `state.reconnecting`, only in the `full` motion context (icons.md, `DsGlyph.spins`).
14. Never an icon alone: `DsIcon` requires either a label or an explicit `decorative: true` (components.md, 01 8).
15. No emoji as an icon, no `phosphor_flutter` dependency, Phosphor under MIT with its license text bundled (icons.md).

## Scope

Create or change in `packages/design_system/`:

- The icon assets: either `fonts/DsIcons.ttf` plus `lib/src/generated/ds_icon_codepoints.g.dart` (glyph name and weight to code point), or `assets/icons/<weight>/<glyph>.vec` plus `lib/src/generated/ds_icon_assets.g.dart` (Design note 1); `assets/icons/LICENSE` (Phosphor MIT text); `assets/icons/MANIFEST.json` (package version, source SHA-256 per SVG, asset SHA-256).
- `lib/src/icons/ds_icon.dart` (`DsIcon`), exported from `lib/design_system.dart`.
- Test files: `test/icons_complete_test.dart`, `test/ds_icon_test.dart`, `test/goldens/icons_test.dart` and the golden files.

Change in `tools/design-tokens/`:

- `tokens.py` gains a subcommand `icons` that builds the assets from `vendor/phosphor/` into the package and is part of `build`; `check` compares them (Design note 2); `tokens.py vendor` copies the `fill` variant too where a glyph is used in it (Design note 3); `tests/test_icons.py` (new).
- `README.md` of the tool.

Change elsewhere:

- `packages/design_system/pubspec.yaml` (assets or font family; `vector_graphics` as a dependency if vectors are chosen), `README.md`; `docs/design/icons.md` ("Integration" with the choice, the comparison table and the asset size); `NOTICE` (Phosphor attribution).
- `docs/DECISIONS.md` section 10: rows `D-M4-05-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Using icons in components (M4-07) and in the apps.
- The server's check of `spaces.icon` against `space-icons` (a server ticket of M1).
- New glyphs beyond the mapping; the `duotone` and `thin` weights.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-05-design-icons`; one writer per worktree.
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
- Network only to fetch the pinned `@phosphor-icons/core` package once (`npm pack` of the version in `vendor/MANIFEST.json`) when `vendor/phosphor/` lacks a needed SVG.
- The asset build must run without a network and with tools already pinned: the Python standard library for the font (a TrueType writer built on `struct`, no `fontTools` unless it is already in the locked `tools/` environment), or `vector_graphics_compiler` run through `dart run` from the workspace lock for vectors.

## Design notes

1. **Font or vectors: measured choice.** Build both candidates once in a temporary folder from the same SVGs: (a) a TrueType icon font with only the used glyphs and weights (one font file with code points in the Private Use Area from `U+E000`, assigned in sorted order of `<weight>/<glyph>` so the mapping is stable), (b) `vector_graphics_compiler` output per glyph and weight. Measure: total bytes, and sharpness as the pixel difference between a 16, 20 and 24 logical pixel render at device pixel ratios 1 and 2 and the SVG rendered by `flutter_svg` in a test (only for the measurement; `flutter_svg` is not a dependency of the result). Choose the candidate with clearly better sharpness at 16 px; on a tie the smaller one. Record the table (bytes, mean and maximum pixel difference per size) in icons.md and the choice as a ruling. Only the chosen candidate is committed.

2. **Generated, checked, current.** `tokens.py icons` reads the mapping from the resolved model, builds the chosen assets and the Dart lookup file (`DsIconData lookup(DsGlyph glyph)` returning either an `IconData` with the font family and code point or the asset path), and writes `MANIFEST.json`. `tokens.py check` rebuilds into memory and compares every asset and the lookup file byte for byte; the build must be deterministic (fixed timestamps in the font tables, sorted order). A mapped glyph without an SVG in `vendor/phosphor/` fails `build` and `check` naming the semantic name, glyph and weight. Ruling for docs/DECISIONS.md: the icon assets are generated by the token tool, because icons.md names `icon.tokens.json` as their single source.

3. **Weights.** The golden must show every semantic icon in its mapped weight and in `fill` (the active form, icons.md "Weights"), so the assets contain both for every mapped glyph; `bold` glyphs (small control glyphs) are shown in `bold` and `fill`. `vendor` copies the SVGs of `assets/<weight>/<glyph>-<weight>.svg` of the pinned package for every needed pair and updates `MANIFEST.json` with their SHA-256 (the tool's provenance gate keeps holding them).

4. **`DsIcon`.** `DsIcon(DsGlyph glyph, {DsIconSizeRole size = DsIconSizeRole.md, Color? color, String? label, bool decorative = false, bool filled = false})`: asserts in debug and throws `ArgumentError` in release when neither `label` nor `decorative` is given; color defaults to `context.dsColors.textPrimary` (generated name for `color.text.primary`); `filled` selects the `fill` asset; a glyph with `spins` rotates continuously with `motion.duration.pulse` only when `context.dsMotion` is the `full` context, otherwise it stands still (and an `AnimationController` is never created in `reduced`). Semantics: `label` as the semantics label with `image: true`, or `ExcludeSemantics` when decorative. Ruling: the spin period uses `motion.duration.pulse` because no dedicated token exists; adding one would change the token files of M0-01's gate.

5. **Rulings for docs/DECISIONS.md** (`D-M4-05-<n>`): font or vectors with the measurements (note 1); assets generated by the token tool (note 2); both weights in the assets (note 3); the spin period (note 4).

## Tests (write them first)

`packages/design_system/test/`:

- `icons_complete_test.dart`: every member of `DsIcons` (enumerated through the generated list of the lookup file) resolves to an asset or code point in its mapped weight and in `fill`; every entry of `space-icons` resolves; the lookup file names no glyph that the mapping lacks.
- `ds_icon_test.dart`: label required (missing label and not decorative throws); a decorative icon is excluded from semantics; a labeled one has the label; `trigger.working` spins in `full` (the rotation changes between two pumps) and stands still in `reduced`; a glyph without `spins` never rotates; sizes `sm`, `md`, `lg` equal `icon.size.*`.
- `goldens/icons_test.dart`: one grid of all semantic icons with their semantic names as captions (captions are test strings), each in its mapped weight and in `fill`, in light and dark, at `icon.size.md`, as CI golden and platform golden on macOS.

`tools/design-tokens/tests/test_icons.py`: `test_build_is_deterministic`; `test_missing_svg_fails_naming_glyph`; `test_check_fails_on_changed_asset`; `test_codepoints_stable_order` (font) or `test_asset_paths_stable` (vectors); `test_manifest_hashes_match`.

Commands that must pass, from the repository root:

- `uv run tools/design-tokens/tokens.py build` (no diff on a second run), `make tokens`
- `cd packages/design_system && flutter analyze --fatal-infos && flutter test --coverage`
- `make lint`, `make fmt-check`, `make coverage`, `make licenses`
- `make check`

## Acceptance criteria

- [ ] Golden of all semantic icons in both weights, light and dark.
- [ ] Asset size measured and documented in icons.md with the font-versus-vector comparison and the choice.
- [ ] `make tokens` fails on a missing glyph or a stale asset; the build is deterministic.
- [ ] `DsIcon` requires a label or `decorative`, spins only the three glyphs and only in `full` motion.
- [ ] No `phosphor_flutter`, no emoji; Phosphor MIT text bundled and named in `NOTICE`; `make licenses` green.
- [ ] Rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments and docstrings on every public symbol; comments explain why.
- `packages/design_system/README.md` (icons section), `tools/design-tokens/README.md`, `docs/design/icons.md`, `NOTICE` updated; rulings of Design note 5 in `docs/DECISIONS.md` section 10.
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
4. The comparison table (bytes and pixel differences) and the choice; the `@phosphor-icons/core` version; the total asset size.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
