# design/build

GENERIERT von tools/design-tokens/tokens.py aus design/tokens. Nicht bearbeiten; Quelle ändern und neu erzeugen.

| Datei | Inhalt | Verbraucher |
|---|---|---|
| `tokens.css` | CSS-Variablen (`--ds-*`), Theme, Dichte und Bewegung per Attribut oder Systemeinstellung | Doku-Site, Vorschau, Statusseite |
| `dart/design_tokens.g.dart` | Konstanten und `ThemeExtension`-Klassen | `packages/design_system` |
| `tokens.resolved.json` | alle Tokens aufgelöst, je Kontext | Server-Prüfung erlaubter Farb- und Icon-Schlüssel, Design-Werkzeuge |
| `preview.html` | eigenständige Vorschau (Schriften und Icons eingebettet) | Menschen, Playwright für `docs/design/previews/` |
| `mermaid.config.json` | Mermaid-Thema `base` mit Farben und Schrift des Themes light | `make diagrams` (`tools/docs/render_diagrams.py`) für alle Diagramme |
| `docs/design/site-theme.css` (außerhalb dieses Ordners) | Farben der Doku-Site und ihrer eingebetteten Mermaid-Blöcke, je Farbschema | `mkdocs.yml` (`extra_css`) |

Erzeugen: `uv run tools/design-tokens/tokens.py build`. Prüfen: `make tokens`.
