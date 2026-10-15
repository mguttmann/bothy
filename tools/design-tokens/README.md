# tools/design-tokens

Generator des Design-Systems. Liest `design/tokens/` (W3C Design Tokens Format Module 2025.10 und
Resolver Module 2025.10), prüft Kontrast und Palettenwächter und schreibt alle Ausgaben. Entscheidung
und Gründe: ADR 0030. Gestaltung: [docs/design/README.md](../../docs/design/README.md).

Nur Python-Standardbibliothek; `uv` stellt die Python-Version (PEP 723 im Kopf von `tokens.py`).
Keine Paket-Abhängigkeit, damit das Gate ohne Netz und ohne Lieferkette läuft.

## Befehle

Aus dem Wurzelverzeichnis des Repositorys:

| Befehl | Wirkung |
|---|---|
| `uv run tools/design-tokens/tokens.py build` | Gates prüfen, dann alle Ausgaben schreiben. Exit 1 bei Verstoß, nichts wird geschrieben. |
| `uv run tools/design-tokens/tokens.py check` | Gates prüfen und vergleichen; Exit 1, wenn eine erzeugte Datei nicht zur Quelle passt. Für das lokale Gate und CI. |
| `uv run tools/design-tokens/tokens.py previews [--view NAME]` | PNG-Vorschauen nach `docs/design/previews/` mit dem Playwright-CLI (`npx`, gepinnte Version in `tokens.py`). Braucht Node und einen installierten Chromium. Nicht Teil von `check`, weil Pixel je Maschine abweichen. `--view` (repeatable: `palette`, `layout`, `components`, `mobile`, `viz`) renders only these views and leaves the other images unchanged. |
| `uv run tools/design-tokens/tokens.py vendor --phosphor DIR --sans DIR --mono DIR` | Schriften und benutzte Phosphor-SVGs nach `vendor/` kopieren (Textdateien mit LF), `vendor/MANIFEST.json` mit SHA-256 der gespeicherten Bytes schreiben. |
| `make tokens` | Das Gate (Teil von `make gates`): ruft `make -C tools/design-tokens check` auf, also die Tests unter `tests/` und danach `tokens.py check`, beides in der gepinnten Umgebung `tools/`. |
| `make lint`, `make fmt-check` | ruff mit der Konfiguration `ruff.toml` dieses Ordners, dazu bandit. |

Zusätzliche Schema-Prüfung gegen die offiziellen JSON-Schemas der DTCG (braucht Netz für den ersten
Abruf):

```bash
uv run --project tools --locked check-jsonschema --schemafile https://www.designtokens.org/schemas/2025.10/format.json design/tokens/*.tokens.json
uv run --project tools --locked check-jsonschema --schemafile https://www.designtokens.org/schemas/2025.10/resolver.json design/tokens/theme.resolver.json
```

## Ausgaben

| Datei | Inhalt |
|---|---|
| `design/build/tokens.css` | CSS-Variablen `--ds-*`; Theme über `data-theme` oder `prefers-color-scheme`, Dichte über `data-density`, Bewegung über `data-motion` oder `prefers-reduced-motion`; instance accent over `data-accent="<key>"` on the same element as `data-theme` (only the swapped variables, per non-default accent and theme) |
| `packages/design_system/lib/src/generated/design_tokens.g.dart` | the same bytes as `design/build/dart/design_tokens.g.dart`: the package copy, because a package may not import a file outside its folder; `check` reports it as stale when it differs (docs/DECISIONS.md D-M4-04-1) |
| `design/build/dart/design_tokens.g.dart` | `DsPalette`, `DsColors`, `DsAccent`, `DsElevation`, `DsDensity`, `DsMotion` (je `ThemeExtension` mit Instanzen je Kontext; `DsAccent` with one constant per theme and accent and `DsAccent.of(key, brightness)`, an unknown key gives the default), `DsSpace`, `DsRadius`, `DsBorderWidth`, `DsLayout`, `DsSize`, `DsIconSize`, `DsFonts`, `DsType`, `DsIcons` |
| `design/build/tokens.resolved.json` | alle Tokens aufgelöst, je Kontext, plus Icon-Zuordnung und erlaubte Bereichs-Icons; member `accents`: default and, per accent and theme, every accent role with `hex`, `alias`, `moved_from` and `swapped` |
| `design/build/ts/tokens.gen.ts` | plain TypeScript without imports for the Grafana panel plugin (M7-25): `light` and `dark` with every colour role of the themes (the same roles as `DsColors`, nested by role, keys camelCase, numeric segments as numeric keys: `color.accent.bg-hover` is `light.accent.bgHover`, `color.viz.series.1` is `light.viz.series[1]`), hex strings as in `tokens.resolved.json` (translucent roles as `{ hex, alpha }`), the type `Theme` that `dark` must satisfy, `fonts` (families with fallbacks), `sizes` (font sizes in px) and `radii` (px); every object `as const` |
| `design/build/ts/fonts/` | byte-identical copies of every file under `vendor/fonts/`: the WOFF2 files of Atkinson Hyperlegible Next and Mono and their SIL Open Font License texts (`<family>.OFL.txt`) |
| `design/build/preview.html` | eigenständige Vorschau mit eingebetteten Schriften und Icons |
| `design/build/README.md` | Hinweis "generiert" |
| `docs/design/palette.md`, `contrast.md`, `tokens.md` | erzeugte Doku |

The Dart file is not `dart format`-formatted; as generated code the package copy is outside the
format check, the doc and style lint and coverage (docs/DECISIONS.md D-M4-01-4). It compiles
against the real Flutter SDK (3.47.7): `packages/design_system` exports it, `flutter analyze`
analyzes its users and `flutter test` compiles it (M4-04).

## Gates

1. **DTCG-Struktur:** Namen, Typen (alle Typen der Spezifikation außer `gradient`),
   Aliase, Zyklen, Werte je Typ; Farben außerhalb des sRGB-Gamuts werden abgelehnt.
2. **Kontrast:** jede Paarung aus `design/design-rules.json` in jedem Theme; Schwellen
   `text` 4.5, `large-text` 3, `ui` 3 (WCAG 2.2 SC 1.4.3 und 1.4.11). Luminanz nach WCAG mit der
   Schwelle 0.04045 der sRGB-Norm (https://www.w3.org/WAI/GL/wiki/Relative_luminance, abgerufen
   2026-10-10).
   Accent variants (`tokenkit/accents.py`, member `accent_variants` of
   `design/design-rules.json`): for every allowed accent and theme, each role that aliases a step
   of the default scale takes the same step of the chosen scale (nearest step by number if it is
   missing); a step that fails a pairing moves along the scale towards more contrast. Every
   pairing that contains a swapped role is checked per accent and theme with the same
   thresholds; a variant below them, or a scale without a passing step, fails `build` and `check`.
   An invalid member (default not an accent, accent not a scale, role missing) is an error (exit 2).
3. **Color vision deficiency** (`tokenkit/cvd.py`, members `cvd_min_delta_e_ok` and `cvd_pairs`
   of `design/design-rules.json`): every listed pair is simulated for protanopia, deuteranopia and
   tritanopia in each theme with the matrices of Machado, Oliveira and Fernandes 2009 at severity
   1.0 (decode sRGB, multiply in linear sRGB, clip to [0, 1], encode), and the OKLab distance
   (`delta_e_ok`) of the two simulated colours must reach `cvd_min_delta_e_ok`; the value, its
   source and its reason stand next to it in the rules file. The pairs: adjacent chart series in
   the fixed order of components.md section 11 (`sequence`, with `wrap` for series 8 with series
   1) and the traffic light pairs of `color.success.fg`, `color.warning.fg` and
   `color.danger.fg` (`pairs`). A failure reads
   `gate: [light, deuteranopia] color.viz.series.3 and color.viz.series.4: deltaE_ok <distance> < <minimum> (<hex a> and <hex b>, simulated <hex> and <hex>)`:
   theme, deficiency, both roles, the measured distance, the minimum, the source colours and what
   they look like under the simulation. Fix it by moving one of the two roles along its scale
   (lightness first, so the hue order stays recognisable) and keep the contrast pairings green.
   An unknown role or deficiency is an error of the rules file (exit 2). `contrast.md` lists every
   row in the section "Color vision deficiency", `tokens.resolved.json` in the member `cvd`.
   The accents of M4-03 swap none of these roles, so the check runs once per theme (a test holds
   this).
4. **Palettenwächter:** verbotene Referenzfarben (OKLab-Abstand), verbotene Farbtonbereiche,
   Mindest-Chroma für Neutrale. Regeln und Quellen stehen in `design/design-rules.json`.
5. **Aktualität:** `check` vergleicht jede erzeugte Datei Byte für Byte.
   The font copies under `design/build/ts/fonts/` are compared with `vendor/fonts/` byte for
   byte; a missing copy (font or licence), a changed copy or a file that is no copy fails as
   `stale:`, and `build` removes such a file. A vendored font without its `<family>.OFL.txt` is an
   error (exit 2). The target `ts` of the Makefile (part of `check`) then compiles
   `tokens.gen.ts` with `tsc --strict --noEmit` (TypeScript 5.9.3 of
   `sdk/typescript/package-lock.json`, installed by `make bootstrap-ci`; without it the target
   fails with a hint).
6. **Herkunft:** `check` vergleicht jede Datei unter `vendor/fonts/` und `vendor/phosphor/` mit ihrem
   SHA-256 in `vendor/MANIFEST.json`; eine fehlende, zusätzliche oder veränderte Datei schlägt fehl.

## Bewusste Grenzen

- Unterstützt wird die Teilmenge der Spezifikation, die das Design-System nutzt. `$extends` und
  JSON-Pointer-`$ref` in Token-Dateien lehnt das Werkzeug mit Fehler ab, statt sie still zu
  übergehen.
- Farbräume: `oklch` und `srgb` werden gerechnet; andere Farbräume brauchen `hex` als Rückfall.
- Ein Token darf nur von einem Modifikator abhängen; sonst bricht `build` ab (die Ausgaben wären
  nicht eindeutig).
- The instance accent is no resolver modifier: an accent role always depends on the theme as
  well, which the rule above forbids. The variants are derived from the theme contexts and written
  as the separate extension `DsAccent` and separate CSS blocks.

## vendor/

Eingebettete Dateien für Vorschau und Doku, mit Herkunft und SHA-256 in `vendor/MANIFEST.json`:
Atkinson Hyperlegible Next und Mono (WOFF2, Teilmengen latin und latin-ext, SIL Open Font License
1.1, Lizenztexte daneben) und die benutzten Phosphor-SVGs (MIT, `vendor/phosphor/LICENSE`).
Erneuern: die Pakete `@fontsource-variable/atkinson-hyperlegible-next`,
`@fontsource-variable/atkinson-hyperlegible-mono` und `@phosphor-icons/core` in der Version aus
`MANIFEST.json` mit `npm pack` holen, entpacken und `tokens.py vendor` aufrufen.
