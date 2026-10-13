# M4-03: Accent variants in the token generator

Status: ready | Depends on: none (M0 closed) | Milestone: M4

## Goal

After this ticket the token generator derives, for every accent an instance may choose (`branding.accent_color`: `spruce`, `loch`, `heather`, `rowan`, `amber`, `gorse`, `moss`; 01 12, configuration reference section 6), the accent roles `color.accent.*`, `color.text.link`, `color.focus.ring` and `color.run.running` in light and dark, checks every pairing that involves them in the contrast gate, and writes them to every output: the Dart file gets one `DsAccent` theme extension per theme and accent, `tokens.css` gets a `data-accent` switch, `tokens.resolved.json` gets the variants, and `docs/design/contrast.md` lists them. `make tokens` fails when a variant falls below AA and when a generated file is not current. Why: the accent replaces the brand color at runtime (ADR 0044); "contrast AA is a test, not a hope" (R-PR-08) holds only if the generator has checked every combination an instance can set, before `design_system` (M4-04) applies it.

## Read first

- `docs/PLAN.md`: section 3; section 6 row M0-01 (the gate this extends); section 10 rows M4-03, M4-04, M4-07 (they use the variants).
- `docs/design/README.md`: "Principles", "Ban list", "Files", "Process for a change", "Runtime branding" (the role list and "equally light steps of the chosen scale"); `docs/design/palette.md` (the scales and their steps), `docs/design/contrast.md` (format), `docs/design/tokens.md`, `docs/design/components.md` (which components use the accent roles).
- `docs/spec/01-product.md` 12 "Branding per instance" (rules: only the category keys, contrast test); R-PR-08.
- `docs/operations/configuration.md` section 6 (`branding.accent_color`, default `spruce`); `docs/spec/03-data-model.md` (`instance_settings`, key `branding.accent_color`).
- ADRs: `0030-design-tokens-as-one-source-with-a-generator.md` (gates, Confirmation), `0044` (branding in two layers).
- `tools/design-tokens/README.md` (commands, outputs, gates, deliberate limitations: a token may depend on only one modifier), `tools/design-tokens/tokens.py`, `tokenkit/rules.py` (`expand_pairs`, `check_contrast`, `check_guards`), `tokenkit/resolver.py`, `tokenkit/emit_dart.py`, `tokenkit/emit_css.py`, `tokenkit/emit_docs.py`, `tests/test_rules.py`, `tests/test_gate.py`; `design/design-rules.json`; `design/tokens/color.light.tokens.json`, `color.dark.tokens.json`, `color.palette.tokens.json`.
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
9. Limits and thresholds only through named values: the contrast thresholds stay those of `design/design-rules.json` (`text` 4.5, `large-text` 3, `ui` 3, WCAG 2.2); this ticket adds no configuration key.
10. Specification first: `docs/design/README.md` ("Runtime branding") and 01 12 describe the variants in the same change. The core contract is not edited.

Product requirements implemented here:

11. Allowed accents are exactly the seven keys of 01 12; the default is `spruce`, whose variant equals today's roles byte for byte.
12. Swapped roles: `color.accent.bg`, `color.accent.bg-hover`, `color.accent.bg-pressed`, `color.accent.fg`, `color.accent.subtle`, `color.text.link`, `color.focus.ring`, `color.run.running`, each only where its value in a theme is an alias of a step of `palette.spruce` (in dark, `color.focus.ring` aliases `gorse` and stays); every other role stays as it is (sidebar, on-accent text, category colors).
13. Derivation: the variant of a role takes the same step of the chosen scale ("equally light steps", design README); if the scale lacks that step, the nearest step by number.
14. Every contrast pairing of `design/design-rules.json` that contains a swapped role is checked for every accent in light and dark; a variant below its threshold fails `build` and `check`.
15. The outputs stay deterministic and `check` compares them byte for byte (ADR 0030 gate 4).

## Scope

Change in `tools/design-tokens/`:

- `tokenkit/accents.py` (new): derivation and step search (Design notes 1 and 2).
- `tokenkit/rules.py`: contrast check per accent (Design note 3).
- `tokenkit/emit_dart.py`, `tokenkit/emit_css.py`, `tokenkit/emit_docs.py`, `tokens.py` (`_resolved_json`, `generate`): the new outputs (Design note 4).
- `tests/test_accents.py` (new), `tests/test_rules.py`, `tests/test_emit.py`, `tests/test_gate.py`: the cases of the Tests section.
- `README.md` of the tool: outputs and gates.

Change elsewhere:

- `design/design-rules.json`: a new member `accent_variants` (Design note 1).
- Regenerated outputs: `design/build/dart/design_tokens.g.dart`, `design/build/tokens.css`, `design/build/tokens.resolved.json`, `docs/design/contrast.md` (and `tokens.md`, `palette.md` if they change).
- `docs/design/README.md` "Runtime branding" and "Ban list" (the guard scope, Design note 5), `docs/design/components.md` (common rules: accent roles come from `DsAccent`), `docs/spec/01-product.md` 12 (one sentence: every variant is in the contrast table).
- `docs/DECISIONS.md` section 10: rows `D-M4-03-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Applying the accent in the apps (`DsAccent` lookup from `instance.read` and `instance.updated`): M4-04 builds the theme, M4-18 reads the instance setting.
- The server's check of the accent key (`instance.branding.update`): the instance settings ticket of M1 (M1-46).
- New colors or scales, a free color value, a gradient, previews of every accent (the preview may show the default only).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-03-accent-variants`; one writer per worktree.
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
- The generator keeps using only the Python standard library (tool README); no new dependency.
- Never edit `design/build/` or the generated docs by hand; run `uv run tools/design-tokens/tokens.py build`.

## Design notes

1. **Rules file.** `design/design-rules.json` gains:

    ```json
    "accent_variants": {
      "$comment": "Accents an instance may choose (branding.accent_color, 01 section 12). Roles that alias palette.spruce are swapped for the same step of the chosen scale; every pairing with a swapped role is checked per accent and theme.",
      "default": "spruce",
      "accents": ["spruce", "loch", "heather", "rowan", "amber", "gorse", "moss"],
      "roles": ["color.accent.bg", "color.accent.bg-hover", "color.accent.bg-pressed", "color.accent.fg", "color.accent.subtle", "color.text.link", "color.focus.ring", "color.run.running"]
    }
    ```

    `tokenkit/accents.py` validates it: `default` is in `accents`, every accent is a scale of `palette`, every role exists in both themes; otherwise a `RuleError` with the member name. A role that does not alias `palette.spruce.<step>` in a theme is reported as "not swapped" in the docs and kept.

2. **Derivation and step search.** `derive(themes, rules) -> dict[(theme, accent), dict[role, Variant]]` with `Variant(role, scale, step, hex, moved_from)`. Start: the same step number on the chosen scale; a missing step (for example `975` on a scale that ends at `950`) takes the nearest existing step by number, ties toward the darker step in light and the lighter in dark. Then, for each swapped role, run every expanded contrast pairing that contains it (as foreground or background) with the variant colors of all swapped roles of the same accent and theme. If one fails, move this role one step along its scale in the direction that raises the failing contrast (compare the relative luminance of the role and its partner: if the role is the darker of the two, go darker, else lighter) and check all its pairings again; repeat until all pass or the scale ends. A role that never passes fails the build with `accent <a>, theme <t>: no step of <scale> meets <pairing>`. The search is deterministic (same input, same steps), and `moved_from` records the start step. Ruling for docs/DECISIONS.md: the step search, because a fixed step on a lighter scale (for example `gorse`) cannot meet AA on white, while 01 12 promises that every listed accent works; the moved steps are visible in `contrast.md`.

3. **Contrast check per accent.** `check_contrast` keeps its rows for the base contexts and gains rows for each non-default accent with the column `accent`; `contrast.md` gets one section per theme and accent with only the pairings that contain a swapped role, marking moved steps (`loch.700, moved from 600`). The default accent produces no extra rows; a test proves that its derivation equals the base roles exactly.

4. **Outputs.**
    - Dart: `final class DsAccent extends ThemeExtension<DsAccent>` with one `Color` field per swapped role (identifier rules of `emit_dart.identifier`), `copyWith`, `lerp`, and the static members `DsAccent.of(String key, Brightness brightness)` (unknown key gives the default, documented) and one constant per theme and accent (`DsAccent.lightLoch`, `DsAccent.darkLoch`, ...); a doc comment on the class names the roles and the rule that components read accent roles only from `DsAccent`. `DsColors` keeps its fields unchanged so existing code compiles.
    - CSS: for each non-default accent and theme a block `[data-accent="<key>"][data-theme="<t>"]` (and the `prefers-color-scheme` form the file already uses for themes) that sets only the swapped `--ds-*` variables.
    - `tokens.resolved.json`: a member `accents` with, per accent and theme, the role, the hex value, the alias and `moved_from`.
    - The preview keeps the default accent.

5. **Guard scope.** The palette guard "Brand and accent" (`scope`: `palette.spruce.`, `color.accent.`) protects the product's own brand: it keeps checking the base contexts as today. Variants of an instance accent are not checked against it, because 01 12 allows `heather`, `rowan` and `amber` deliberately (the instance owner's choice from the checked palette) while the product brand stays spruce; contrast applies to them in full. Record this in the "Ban list" of the design README and as a ruling. The guard "No closeness to Discord anywhere in the palette" applies to the scales anyway.

6. **Rulings for docs/DECISIONS.md** (`D-M4-03-<n>`): the rules member and the role list (note 1); the step search (note 2); `DsAccent` as a separate extension instead of a fourth resolver modifier, because the tool allows a token to depend on one modifier only and the accent always combines with the theme (note 4); the guard scope (note 5).

## Tests (write them first)

`tools/design-tokens/tests/` (unittest, run by `make -C tools/design-tokens check`):

- `test_accents.py`:
    - `test_default_accent_equals_base_roles` (every swapped role of `spruce` equals the base value in both themes, no `moved_from`);
    - `test_same_step_on_other_scale` (a synthetic model where `color.accent.bg` is `spruce.700`: the `loch` variant is `loch.700`);
    - `test_missing_step_takes_nearest` (`spruce.975` on a scale ending at `950`: `950`);
    - `test_role_not_aliasing_spruce_is_kept` (dark `color.focus.ring` on `gorse.300` stays for every accent);
    - `test_failing_variant_moves_to_passing_step` (a synthetic scale whose start step fails 4.5 on white and whose next darker step passes: the variant is the darker step, `moved_from` set);
    - `test_no_passing_step_fails_build` (a synthetic scale without any passing step: `RuleError` naming accent, theme, scale and pairing);
    - `test_deterministic` (two derivations give identical results);
    - `test_rules_validation` (unknown default, unknown scale, unknown role: each a `RuleError` naming the member).
- `test_rules.py`: `test_every_accent_checked_light_and_dark` (on the real files: rows exist for six non-default accents times two themes, each pairing with a swapped role present, all passing).
- `test_emit.py`: `test_dart_has_accent_extension` (the class, one constant per non-default accent and theme, `of` with an unknown key returning the default), `test_css_accent_blocks` (only swapped variables in each block), `test_resolved_json_accents`.
- `test_gate.py`: `test_check_fails_when_accent_output_stale` (a changed accent hex in the Dart file fails `check`), `test_build_fails_below_aa_for_accent` (a temporary rules file that forces a failing scale makes `build` exit 1 and write nothing).

Commands that must pass, from the repository root:

- `uv run tools/design-tokens/tokens.py build` (no change after a second run), `uv run tools/design-tokens/tokens.py check`
- `make tokens`, `make lint`, `make fmt-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `make tokens` checks all seven accents in light and dark; a variant below AA fails it; a stale accent output fails it.
- [ ] `docs/design/contrast.md` lists every accent and theme with the pairings that involve swapped roles and marks moved steps.
- [ ] Unit tests of the derivation (same step, nearest step, kept roles, step search, failure, determinism) are green.
- [ ] Dart (`DsAccent`), CSS (`data-accent`) and `tokens.resolved.json` carry the variants; the default accent equals today's roles.
- [ ] Design README, components.md and 01 12 describe the variants and the guard scope; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; docstrings in the generator (ruff `D`, convention `google`); comments explain why.
- `tools/design-tokens/README.md` updated (outputs, gates); `docs/design/README.md`, `docs/design/components.md`, 01 12 updated; rulings of Design note 6 in `docs/DECISIONS.md` section 10.
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
4. Per accent and theme the steps chosen for each swapped role and every moved step with its reason (the failing pairing).
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
