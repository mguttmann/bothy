# tools/design-tokens

Generator of the design system. Reads `design/tokens/` (W3C Design Tokens Format Module 2025.10 and
Resolver Module 2025.10), checks contrast and palette guards and writes all outputs. Decision
and reasons: ADR 0030. Design: [docs/design/README.md](../../docs/design/README.md).

Python standard library only; `uv` provides the Python version (PEP 723 in the header of `tokens.py`).
No package dependency, so that the gate runs without a network and without a supply chain.

## Commands

From the root directory of the repository:

| Command | Effect |
|---|---|
| `uv run tools/design-tokens/tokens.py build` | Check the gates, then write all outputs. Exit 1 on a violation, nothing is written. |
| `uv run tools/design-tokens/tokens.py check` | Check the gates and compare; exit 1 if a generated file does not match the source. For the local gate and CI. |
| `uv run tools/design-tokens/tokens.py previews [--view NAME]` | PNG previews to `docs/design/previews/` with the Playwright CLI (`npx`, pinned version in `tokens.py`). Needs Node and an installed Chromium. Not part of `check`, because pixels differ per machine. `--view` (repeatable: `palette`, `layout`, `components`, `mobile`, `terminal`, `viz`) renders only these views and leaves the other images unchanged. |
| `uv run tools/design-tokens/tokens.py vendor --phosphor DIR --sans DIR --mono DIR` | Copy fonts and the used Phosphor SVGs to `vendor/` (text files with LF), write `vendor/MANIFEST.json` with the SHA-256 of the stored bytes. |
| `make tokens` | The gate (part of `make gates`): calls `make -C tools/design-tokens check`, that is the tests under `tests/` and then `tokens.py check`, both in the pinned environment `tools/`. |
| `make lint`, `make fmt-check` | ruff with the configuration `ruff.toml` of this folder, plus bandit. |

Additional schema check against the official JSON schemas of the DTCG (needs a network for the first
download):

```bash
uv run --project tools --locked check-jsonschema --schemafile https://www.designtokens.org/schemas/2025.10/format.json design/tokens/*.tokens.json
uv run --project tools --locked check-jsonschema --schemafile https://www.designtokens.org/schemas/2025.10/resolver.json design/tokens/theme.resolver.json
```

## Outputs

| File | Content |
|---|---|
| `design/build/tokens.css` | CSS variables `--ds-*`; theme via `data-theme` or `prefers-color-scheme`, density via `data-density`, motion via `data-motion` or `prefers-reduced-motion`; instance accent over `data-accent="<key>"` on the same element as `data-theme` (only the swapped variables, per non-default accent and theme) |
| `design/build/dart/design_tokens.g.dart` | `DsPalette`, `DsColors`, `DsAccent`, `DsElevation`, `DsDensity`, `DsMotion` (each a `ThemeExtension` with instances per context; `DsAccent` with one constant per theme and accent and `DsAccent.of(key, brightness)`, an unknown key gives the default), `DsSpace`, `DsRadius`, `DsBorderWidth`, `DsLayout`, `DsSize`, `DsIconSize`, `DsFonts`, `DsType`, `DsIcons` |
| `design/build/tokens.resolved.json` | all tokens resolved, per context, plus the icon mapping and the allowed space icons; member `accents`: default and, per accent and theme, every accent role with `hex`, `alias`, `moved_from` and `swapped` |
| `design/build/preview.html` | standalone preview with embedded fonts and icons |
| `design/build/README.md` | note "generated" |
| `docs/design/palette.md`, `contrast.md`, `tokens.md` | generated docs |

The Dart file is not formatted with `dart format`; `packages/design_system` takes it over in its
build and formats it there. It was checked with `dart analyze` (Dart 3.13.5) against stand-ins
for the Flutter types it uses; it has not yet been compiled against the real Flutter SDK (ticket
"design_system package").

## Gates

1. **DTCG structure:** names, types (all types of the specification except `gradient`),
   aliases, cycles, values per type; colors outside the sRGB gamut are rejected.
2. **Contrast:** every pairing from `design/design-rules.json` in every theme; thresholds
   `text` 4.5, `large-text` 3, `ui` 3 (WCAG 2.2 SC 1.4.3 and 1.4.11). Luminance per WCAG with the
   threshold 0.04045 of the sRGB standard (https://www.w3.org/WAI/GL/wiki/Relative_luminance, retrieved
   2026-10-10).
   Accent variants (`tokenkit/accents.py`, member `accent_variants` of
   `design/design-rules.json`): for every allowed accent and theme, each role that aliases a step
   of the default scale takes the same step of the chosen scale (nearest step by number if it is
   missing); a step that fails a pairing moves along the scale towards more contrast. Every
   pairing that contains a swapped role is checked per accent and theme with the same
   thresholds; a variant below them, or a scale without a passing step, fails `build` and `check`.
   An invalid member (default not an accent, accent not a scale, role missing) is an error (exit 2).
3. **Palette guards:** forbidden reference colors (OKLab distance), forbidden hue ranges,
   minimum chroma for neutrals. Rules and sources are in `design/design-rules.json`.
4. **Freshness:** `check` compares every generated file byte for byte.
5. **Provenance:** `check` compares every file under `vendor/fonts/` and `vendor/phosphor/` with its
   SHA-256 in `vendor/MANIFEST.json`; a missing, extra or changed file fails.

## Deliberate limitations

- The tool supports the subset of the specification that the design system uses. It rejects
  `$extends` and JSON Pointer `$ref` in token files with an error instead of silently
  skipping them.
- Color spaces: `oklch` and `srgb` are computed; other color spaces need `hex` as a fallback.
- A token may depend on only one modifier; otherwise `build` aborts (the outputs would not be
  unambiguous).
- The instance accent is no resolver modifier: an accent role always depends on the theme as
  well, which the rule above forbids. The variants are derived from the theme contexts and written
  as the separate extension `DsAccent` and separate CSS blocks.

## vendor/

Embedded files for the preview and the docs, with provenance and SHA-256 in `vendor/MANIFEST.json`:
Atkinson Hyperlegible Next and Mono (WOFF2, subsets latin and latin-ext, SIL Open Font License
1.1, license texts next to them) and the used Phosphor SVGs (MIT, `vendor/phosphor/LICENSE`).
Renew: fetch the packages `@fontsource-variable/atkinson-hyperlegible-next`,
`@fontsource-variable/atkinson-hyperlegible-mono` and `@phosphor-icons/core` in the version from
`MANIFEST.json` with `npm pack`, unpack them and call `tokens.py vendor`.
