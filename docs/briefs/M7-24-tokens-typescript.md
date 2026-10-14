# M7-24: Token output for TypeScript

Status: ready | Depends on: M6-10 | Milestone: M7

## Goal

After this ticket `tools/design-tokens` emits a further output for TypeScript (25 10 item 3, ADR 0030): `design/build/ts/tokens.gen.ts` with the colors per theme (light and dark, including the chart roles `color.viz.*` and the state roles the traffic light maps to), the font families, font sizes and radii as typed constants, and it copies the vendored font files (Atkinson Hyperlegible Next and Mono) with their SIL Open Font License to `design/build/ts/fonts/`. `make tokens` generates and compares the new output like every other output: a changed token file without regeneration fails the gate, and the license file lies next to the fonts. The Grafana panel plugin (M7-25) imports only this file. Why: tokens are generated, never copied (25 10 item 3); a second hand-written palette in the plugin would drift from the apps and break the color vision check of M6-10.

## Read first

- `docs/spec/25-visualization.md` 10 (all items), `docs/adr/0030-*.md` (design tokens), `docs/design/` (token roles, `contrast.md`), `design/design-rules.json`.
- `tools/design-tokens/README.md` (outputs, fonts and their license, vendor manifest), `tools/design-tokens/Makefile` (`check`, `test`, `gate`), `tools/design-tokens/tokenkit/` (`emit_css.py`, `emit_dart.py` as the pattern), `design/build/README.md`.
- `docs/quality/licenses.md` (fonts under OFL, NOTICE), `NOTICE`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-24-<n>` rows).
- Briefs (merged names win, D-W-100): `M4-03-accent-variants.md` (contrast gate), `M6-10-viz-cvd-check.md` (chart roles and the color vision check).

This ticket touches no table, no action and no event.

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
8. White label: no product-specific values outside the tokens.
9. No new runtime dependency: the emitter uses the standard library and the existing tokenkit; the output is plain TypeScript without imports.
10. Specification first: if the output shape differs from 25 10 item 3, update 25 in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125).

Product requirements implemented here:

12. The output carries every color role of both themes that the Dart output carries, with the same names in a TypeScript-safe form (record the name mapping), plus font families, sizes and radii; values come from `tokens.resolved.json`, never typed by hand.
13. The generated file states that it is generated and must not be edited; it compiles with `tsc --strict` (a pinned TypeScript of the repository's lock file or the plugin's later lock file; record which).
14. The font files are copied byte-identical from `tools/design-tokens/vendor/fonts` with the OFL text next to them; the vendor manifest checksums hold.
15. `make tokens` fails when a token changes without regeneration, when the TypeScript file is edited by hand, or when a font or the license file is missing.

## Scope

- `tools/design-tokens/tokenkit/emit_ts.py` and its wiring in `tokens.py` (build and check).
- `design/build/ts/tokens.gen.ts`, `design/build/ts/fonts/` (generated outputs, committed like the other outputs).
- Tests in `tools/design-tokens/tests/` (output shape, comparison, fonts and license).
- Docs: `tools/design-tokens/README.md` (the new output), `design/build/README.md`, `docs/spec/25-visualization.md` 10 if sharpened, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-24-<n>`.

## Out of scope

- The Grafana plugin (M7-25); CSS or Dart outputs (unchanged).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-24-tokens-typescript`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(design-tokens): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Shape** (25 10 item 3; Ruling for docs/DECISIONS.md). `export const light = {...} as const; export const dark = {...} as const;` with nested groups by role, `export const fonts`, `sizes`, `radii`; a type `Theme` derived from `typeof light`; colors as hex strings exactly as the resolved JSON has them.

2. **Rulings for `docs/DECISIONS.md`** (`D-M7-24-<n>`): the shape and name mapping (note 1), the TypeScript used for the compile check.

## Tests (write them first)

- `test_emit_ts.py`: every color role of both themes is present with the resolved value; fonts, sizes and radii present; generated header; deterministic output.
- Gate tests: a token changed without regeneration fails; a hand edit of `tokens.gen.ts` fails; a missing font or license fails.
- Compile check: `tsc --strict --noEmit` over the generated file.

Commands that must pass, from the repository root:

- `make tokens`
- `make licenses`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `make tokens` generates and compares `design/build/ts/tokens.gen.ts`; a changed token file without regeneration fails.
- [ ] The OFL license lies next to the copied fonts; the vendor checksums hold.
- [ ] The file compiles with `tsc --strict`.
- [ ] READMEs, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public function (purpose, parameters, return value, errors); comments explain why.
- Rulings of Design note 2 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The name mapping from token roles to TypeScript keys.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
