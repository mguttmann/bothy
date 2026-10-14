# M6-10: Design tokens for charts: check for color vision deficiency

Status: ready | Depends on: M4-03 | Milestone: M6

## Goal

After this ticket the token gate (`make tokens`) checks the chart series `color.viz.series.1` to `color.viz.series.8` not only for contrast `ui` against the surfaces (which it does since M0) but also for distinguishability under simulated color vision deficiency: for each theme, the series are transformed with a published simulation model for protanopia, deuteranopia and tritanopia, and every pair of adjacent series (in the fixed order the views use, 25 10 item 2, components.md 11) must keep at least the minimum distance that `design/design-rules.json` names as `cvd_min_delta_e_ok`; otherwise the gate fails and names the theme, the deficiency, the pair and the distance. The series are adjusted where they fail, the contrast pairings of `design-rules.json` stay green in both themes and with every accent of M4-03, `docs/design/contrast.md` lists the new check, and the previews `docs/design/previews/viz-*.png` are regenerated. The traffic light uses only the existing state roles (`color.success.*`, `color.warning.*`, `color.danger.*`), which the check holds too. Why: charts must not depend on red-green or blue-yellow distinctions that a part of the audience cannot see (25 10 item 2, 01 8 "Color"); "contrast AA is a test, not a hope" (R-PR-08) applies to the visualizations in the same way.

## Read first

- `docs/spec/25-visualization.md`: 1 rule 5 (values generated from `design/tokens/`), 7.4 (states with icon and text), 10 "Shared design language" (items 1 and 2: the roles `color.viz.*`, "distinguishable despite red-green confusion", the traffic light on the state roles, every pairing in `design/design-rules.json`), 13 (row "after the early M4 tickets").
- `docs/spec/01-product.md`: 8 "Accessibility" (rows "Contrast" and "Color").
- `docs/design/README.md`, `docs/design/components.md` 11 "Visualizations" (series order, axis, area fill, grid), `docs/design/contrast.md` (generated; how rows are listed), `docs/design/palette.md`, `docs/design/tokens.md`.
- `design/design-rules.json` (the pairing "Charts: series and axes against the surface", the palette guards), `design/tokens/color.light.tokens.json` and `color.dark.tokens.json` (group `viz`), `design/tokens/color.palette.tokens.json`.
- `tools/design-tokens/README.md`, `tools/design-tokens/tokens.py` (`build`, `check`, `previews --view viz`), `tools/design-tokens/tokenkit/rules.py` (contrast rows, `RuleError`), `tokenkit/color.py` (OKLab and conversions), `tokenkit/emit_docs.py` (contrast.md), and the tests under `tools/design-tokens/tests/`.
- ADR 0030 (design tokens), ADR 0069 (visualization on two levels).
- `docs/DECISIONS.md`: D-W-54, D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-10-<n>` rows).
- Brief (merged names win, D-W-100): `docs/briefs/M4-03-accent-variants.md` (accent derivation, how `tokens.py check` iterates themes and accents, `DsAccent`).
- Machado, Oliveira and Fernandes, "A Physiologically-based Model for Simulation of Color Vision Deficiency", IEEE Transactions on Visualization and Computer Graphics 15(6), 2009 (the simulation matrices for severity 1.0), or another published model with matrices you can cite; Viénot, Brettel and Mollon 1999 is acceptable for protanopia and deuteranopia but has no tritanopia matrix of the same kind.

This ticket touches no API operation, no table, no action, no event and no server configuration key.

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
8. White label: examples use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. The minimum distance is a named value in `design/design-rules.json` with its source and reason next to it; code and docs name the value, never the number.
10. Specification first: if the check changes a rule of 25 10 or components.md 11, update them in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Every chart series reaches contrast `ui` against `color.surface.canvas`, `color.surface.list` and `color.surface.raised` in light and dark (the existing pairing stays and stays green).
12. Adjacent series (1 and 2, 2 and 3, up to 7 and 8, and 8 and 1 because the order wraps when a view needs more than eight groups; components.md 11) stay distinguishable under simulated protanopia, deuteranopia and tritanopia at full severity, measured as the OKLab distance (`delta_e_ok`, the measure the palette guards already use) of the simulated colors, at least `cvd_min_delta_e_ok`.
13. The traffic light uses only `color.success.*`, `color.warning.*` and `color.danger.*`; the check holds that green, yellow and red stay pairwise distinguishable under the three simulations by the same minimum, and states always carry an icon and text as well (25 7.4).
14. The check is part of `make tokens` and therefore of `make check`; a token change that breaks it fails the gate.

## Scope

Create:

- `tools/design-tokens/tokenkit/cvd.py`: the simulation (linear RGB matrices of the chosen model at severity 1.0, applied after sRGB linearization and clipped before re-encoding), and the distance check over a list of role pairs (Design notes 1 and 2).
- `tools/design-tokens/tests/test_cvd.py` as `unittest.TestCase` classes, because `make tokens` runs `python -m unittest discover` over that folder (`tools/design-tokens/Makefile`) (Tests).

Change:

- `tools/design-tokens/tokenkit/rules.py`: a new rule kind `cvd_pairs` read from `design-rules.json`; results as rows like the contrast rows (theme, deficiency, pair, distance, required, passed); `RuleError` for an unknown role or deficiency.
- `tools/design-tokens/tokens.py` (`check` and `build`): run the CVD rows with the contrast rows and fail on any failed row; print one line per failure with theme, deficiency, both roles and the distance.
- `tools/design-tokens/tokenkit/emit_docs.py`: a section "Color vision deficiency" in `docs/design/contrast.md` with the model, its source, the minimum and one row per checked pair and theme.
- `design/design-rules.json`: `cvd_min_delta_e_ok` with `source` and `reason`, and `cvd_pairs`: the adjacent series pairs, the wrap pair, and the traffic light pairs (Design note 3).
- `design/tokens/color.light.tokens.json` and `color.dark.tokens.json` (and `color.palette.tokens.json` if a new palette step is needed): adjust the series that fail (Design note 4).
- Regenerated outputs of `make tokens` under `design/build/` and `docs/design/contrast.md`; `docs/design/previews/viz-light.png` and `viz-dark.png` regenerated with `tokens.py previews --view viz`.
- `docs/design/components.md` 11 and `docs/spec/25-visualization.md` 10 item 2: name the check and the value (by name).
- `tools/design-tokens/README.md`; root `CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M6-10-1` to `D-M6-10-4`.

## Out of scope

- The package `bothy_viz` and its views (M6-11, M6-12); the TypeScript token output for the Grafana plugin (M7).
- Contrast pairs outside the charts, unless a series change moves a shared palette step that another pairing uses (then that pairing must stay green and the report names it).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-10-viz-cvd-check`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(design-tokens): ...`, `docs(design): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests make no network call; the previews use Playwright through `npx` as `tokens.py previews` does; temporary directories only via `mktemp -d` or `tempfile.TemporaryDirectory`.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- Never lower a threshold, disable a gate, delete a test or loosen a palette guard; never lower a contrast threshold to make a series pass.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Model** (Ruling for docs/DECISIONS.md). Use the matrices of Machado, Oliveira and Fernandes 2009 at severity 1.0 for protanopia, deuteranopia and tritanopia, applied in linear sRGB (decode sRGB, multiply, clip to `[0, 1]`, encode). Put the nine numbers of each matrix in `cvd.py` with a comment that cites the paper and the table they come from, and test them against reference values (Tests). Reason: one model for all three deficiencies, in linear RGB, widely used by browser developer tools; the gate needs a reproducible transformation, not a perceptual study.

2. **Distance** (Ruling for docs/DECISIONS.md). OKLab Euclidean distance of the two simulated colors, the measure `tokenkit/color.py` and the palette guards already use (`delta_e_ok`). Choose `cvd_min_delta_e_ok` with a source or a stated derivation (for example from the just noticeable difference in OKLab that the palette guards use, scaled for small chart marks) and write the reason into `design-rules.json`; the report states the chosen value and the measured minimum per theme before and after the adjustment.

3. **Pairs.** `cvd_pairs` lists role pairs and the deficiencies: series `n` with `n+1` for `n` from 1 to 7, series 8 with series 1, and `color.success.fg` with `color.warning.fg`, `color.warning.fg` with `color.danger.fg`, `color.success.fg` with `color.danger.fg`. Pairs are checked per theme (`light`, `dark`); the accents of M4-03 do not touch `color.viz.*` and the state roles, so the check runs once per theme; if the accent derivation of M4-03 maps an accent onto a state role, check that combination as well and say so.

4. **Adjusting the series.** Change the fewest series possible, keep the hue order of components.md 11 recognisable (for example by moving lightness or chroma before moving hue), keep every palette guard green (no Discord blurple, no forbidden warm hues where a guard forbids them) and keep contrast `ui` against the three surfaces. Record every changed value with before and after in the report and in `D-M6-10-3`. If the traffic light pair fails under tritanopia, do not change the state roles silently: change them only if the contrast pairings of the banners and badges stay green, otherwise stop with `Verdict: BLOCKED` naming the pair (the state roles carry many components).

5. **Outputs.** `contrast.md` and `tokens.resolved.json` gain the CVD rows; the Dart output of `design_system` changes only through the changed values; `make tokens` regenerates everything in a temporary folder and compares as for every output.

6. **Rulings for `docs/DECISIONS.md`** (section 10): `D-M6-10-1` the simulation model (note 1); `D-M6-10-2` the distance and `cvd_min_delta_e_ok` (note 2); `D-M6-10-3` the changed series values (note 4); `D-M6-10-4` the pairs including the wrap pair and the traffic light (note 3).

## Tests (write them first)

`tools/design-tokens/tests/test_cvd.py` (`unittest.TestCase`, run by `make tokens`):

- `test_matrices_match_reference`: each matrix applied to the primaries and to white reproduces the reference values of the paper (white stays white within rounding); a cited sample color gives the simulated value of the source within `1e-3`.
- `test_simulation_clips_and_roundtrips`: inputs at the gamut edge stay within `[0, 1]`; gray stays gray.
- `test_adjacent_pairs_checked_with_wrap`: a fake rules file with eight series yields eight pairs per deficiency and theme.
- `test_failing_pair_fails_gate`: two nearly identical series under deuteranopia produce a failed row and a non-zero exit of `tokens.py check` with the expected message.
- `test_unknown_role_or_deficiency_is_rule_error`.
- `test_real_tokens_pass`: the real `design/` passes the CVD check and the contrast check in both themes.
- `test_contrast_md_lists_cvd_rows`: the generated `contrast.md` contains the section and one row per pair, theme and deficiency.

Extend `test_gate.py` (M0-01): a changed series value without regeneration fails `make tokens` as before; a series that breaks the CVD rule fails it with the CVD message.

Commands that must pass, from the repository root:

- `make tokens` (runs the unit tests of `tools/design-tokens/tests` with `unittest`)
- `make tokens`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `make tokens` checks protanopia, deuteranopia and tritanopia for every adjacent series pair, the wrap pair and the traffic light pairs in both themes, and fails below `cvd_min_delta_e_ok`.
- [ ] Every series reaches contrast `ui` against the three surfaces in both themes; every palette guard stays green.
- [ ] The simulation is unit tested against reference values of the cited model.
- [ ] `contrast.md`, the previews, components.md 11 and 25 10 are current; the rulings are recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; docstrings on every public Python symbol (ruff `D`); comments explain why.
- `tools/design-tokens/README.md` updated in English (the CVD check, the model, the value and how to read a failure).
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
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
4. The model and its source, the chosen `cvd_min_delta_e_ok` with reason, the minimum measured distance per theme and deficiency before and after, every changed token value.
5. What is not proven (a simulation is not a test with people).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
