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
| `uv run tools/design-tokens/tokens.py previews` | PNG-Vorschauen nach `docs/design/previews/` mit dem Playwright-CLI (`npx`, gepinnte Version in `tokens.py`). Braucht Node und einen installierten Chromium. Nicht Teil von `check`, weil Pixel je Maschine abweichen. |
| `uv run tools/design-tokens/tokens.py vendor --phosphor DIR --sans DIR --mono DIR` | Schriften und benutzte Phosphor-SVGs nach `vendor/` kopieren, `vendor/MANIFEST.json` mit SHA-256 schreiben. |
| `uv run --no-project python -m unittest discover -s tools/design-tokens/tests -t tools/design-tokens` | Unit-Tests |
| `uvx ruff check tools/design-tokens` | Lint (Konfiguration `ruff.toml`) |

`make tokens` ruft später `build` auf; `make check` ruft `check` und die Tests auf (Verdrahtung durch
das Qualitäts-Team).

Zusätzliche Schema-Prüfung gegen die offiziellen JSON-Schemas der DTCG (braucht Netz für den ersten
Abruf):

```bash
uvx check-jsonschema --schemafile https://www.designtokens.org/schemas/2025.10/format.json design/tokens/*.tokens.json
uvx check-jsonschema --schemafile https://www.designtokens.org/schemas/2025.10/resolver.json design/tokens/theme.resolver.json
```

## Ausgaben

| Datei | Inhalt |
|---|---|
| `design/build/tokens.css` | CSS-Variablen `--ds-*`; Theme über `data-theme` oder `prefers-color-scheme`, Dichte über `data-density`, Bewegung über `data-motion` oder `prefers-reduced-motion` |
| `design/build/dart/design_tokens.g.dart` | `DsPalette`, `DsColors`, `DsElevation`, `DsDensity`, `DsMotion` (je `ThemeExtension` mit Instanzen je Kontext), `DsSpace`, `DsRadius`, `DsBorderWidth`, `DsLayout`, `DsSize`, `DsIconSize`, `DsFonts`, `DsType`, `DsIcons` |
| `design/build/tokens.resolved.json` | alle Tokens aufgelöst, je Kontext, plus Icon-Zuordnung und erlaubte Bereichs-Icons |
| `design/build/preview.html` | eigenständige Vorschau mit eingebetteten Schriften und Icons |
| `design/build/README.md` | Hinweis "generiert" |
| `docs/design/palette.md`, `contrast.md`, `tokens.md` | erzeugte Doku |

Die Dart-Datei ist nicht `dart format`-formatiert; `packages/design_system` übernimmt sie in seinem
Build und formatiert dort. Geprüft wurde sie mit `dart analyze` (Dart 3.13.5) gegen Stellvertreter
der benutzten Flutter-Typen; gegen das echte Flutter-SDK ist sie noch nicht übersetzt (Ticket
"design_system-Paket").

## Gates

1. **DTCG-Struktur:** Namen, Typen (alle Typen der Spezifikation außer `gradient`),
   Aliase, Zyklen, Werte je Typ; Farben außerhalb des sRGB-Gamuts werden abgelehnt.
2. **Kontrast:** jede Paarung aus `design/design-rules.json` in jedem Theme; Schwellen
   `text` 4.5, `large-text` 3, `ui` 3 (WCAG 2.2 SC 1.4.3 und 1.4.11). Luminanz nach WCAG mit der
   Schwelle 0.04045 der sRGB-Norm (https://www.w3.org/WAI/GL/wiki/Relative_luminance, abgerufen
   2026-10-10).
3. **Palettenwächter:** verbotene Referenzfarben (OKLab-Abstand), verbotene Farbtonbereiche,
   Mindest-Chroma für Neutrale. Regeln und Quellen stehen in `design/design-rules.json`.
4. **Aktualität:** `check` vergleicht jede erzeugte Datei Byte für Byte.

## Bewusste Grenzen

- Unterstützt wird die Teilmenge der Spezifikation, die das Design-System nutzt. `$extends` und
  JSON-Pointer-`$ref` in Token-Dateien lehnt das Werkzeug mit Fehler ab, statt sie still zu
  übergehen.
- Farbräume: `oklch` und `srgb` werden gerechnet; andere Farbräume brauchen `hex` als Rückfall.
- Ein Token darf nur von einem Modifikator abhängen; sonst bricht `build` ab (die Ausgaben wären
  nicht eindeutig).

## vendor/

Eingebettete Dateien für Vorschau und Doku, mit Herkunft und SHA-256 in `vendor/MANIFEST.json`:
Atkinson Hyperlegible Next und Mono (WOFF2, Teilmengen latin und latin-ext, SIL Open Font License
1.1, Lizenztexte daneben) und die benutzten Phosphor-SVGs (MIT, `vendor/phosphor/LICENSE`).
Erneuern: die Pakete `@fontsource-variable/atkinson-hyperlegible-next`,
`@fontsource-variable/atkinson-hyperlegible-mono` und `@phosphor-icons/core` in der Version aus
`MANIFEST.json` mit `npm pack` holen, entpacken und `tokens.py vendor` aufrufen.
