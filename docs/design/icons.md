# Icons

Quelle: `design/tokens/icon.tokens.json`. Größen sind Tokens (`icon.size.sm`, `.md`, `.lg`); die
Zuordnung semantischer Namen zu Glyphen steht als Erweiterung `ds.icons` am Gruppenknoten `icon`,
weil DTCG 2025.10 keinen Icon-Typ kennt und eigene Typen verbietet
(https://www.designtokens.org/tr/2025.10/format/, abgerufen 2026-10-10 [belegt]).

## Wahl: Phosphor

| Kriterium | Phosphor | Lucide | Material Symbols |
|---|---|---|---|
| Lizenz | MIT [belegt] | ISC [belegt, npm `lucide-static`] | Apache-2.0 [belegt, npm `material-symbols`] |
| Strichstärken | thin, light, regular, bold, fill, duotone [belegt] | eine Strichform, Breite einstellbar | variable Achsen (Füllung, Gewicht) |
| gefüllte Variante für "aktiv" | ja (`fill`) | nein | ja |
| Wirkung | freundlich, geometrisch, eigenständig | neutral, häufig im generischen Look | sofort als Google/Android erkennbar |
| Flutter | Paket `phosphor_flutter` 2.1.0, zuletzt vor zwei Jahren veröffentlicht [belegt] | Community-Pakete | Paket `material_symbols_icons` |

Belege: https://github.com/phosphor-icons/core (MIT), Paket `@phosphor-icons/core@2.1.1` (Lizenz
MIT, Ordner `assets/<weight>`), https://pub.dev/packages/phosphor_flutter (Version 2.1.0,
Herausgeber phosphoricons.com, Stile Thin, Light, Regular, Bold, Fill, Duotone); alle abgerufen
2026-10-10.

Gründe: Die gefüllte Variante zeigt "aktiv" und "Zustand erreicht" ohne zweites Icon-Set; die
Stärke `bold` bleibt bei kleiner Größe lesbar; die Formen passen zu den offenen Formen von
Atkinson Hyperlegible. Material Symbols würde die App wie Android aussehen lassen; Lucide hat keine
gefüllte Variante.

**Einbindung ohne veraltetes Paket.** Weil `phosphor_flutter` seit zwei Jahren ruht, bindet
`packages/design_system` Phosphor selbst ein: Der Paket-Build nimmt die SVGs der benutzten Glyphen
aus `@phosphor-icons/core` in gepinnter Version und erzeugt daraus eine Icon-Schrift mit nur diesen
Glyphen (oder Vektor-Assets mit `vector_graphics_compiler`, flutter.dev); die Wahl trifft das
Ticket "Icons im Design-System-Paket" nach einem Größen- und Schärfevergleich. Die erzeugte Klasse
`DsIcons` liefert die semantischen Namen (`DsIcons.triggerWorking`). Die Vorschau nutzt dieselben
SVGs aus `tools/design-tokens/vendor/phosphor/` (mit MIT-Lizenztext).

### Integration (M4-05): compiled vector assets

The ticket built both candidates once from the same 128 SVGs (every mapped glyph in its mapped
weight and in `fill`, every space icon in `regular`, `@phosphor-icons/core` 2.1.1) and compared
them with the SVG rendered by `flutter_svg` in a Flutter test (Flutter 3.47.7, `dart:ui`
pictures without a widget tree, alpha channel per pixel, 2026-10-15):

- (a) a TrueType icon font with only these glyphs, code points from `U+E000` in sorted order of
  `<weight>/<glyph>`, cubic curves turned into quadratic ones within 1/2048 em, no hinting;
- (b) one `vector_graphics_compiler` 1.3.1 file per glyph and weight.

Pixel difference: mean absolute difference of the alpha value in percent of full coverage, over
all 128 glyphs; "worst glyph" is the glyph with the largest mean, "max pixel" the largest single
pixel difference. The reference covers about 27 % of the pixels at every size.

| Candidate | Bytes | Size, ratio | Mean | Worst glyph | Max pixel |
|---|---|---|---|---|---|
| font | 34,564 (one file) | 16 px, x1 | 2.97 % | 6.04 % | 47.1 % |
| font | | 20 px, x1 | 3.43 % | 7.23 % | 60.4 % |
| font | | 24 px, x1 | 3.12 % | 5.97 % | 62.4 % |
| font | | 16 px, x2 | 3.09 % | 5.41 % | 69.4 % |
| font | | 20 px, x2 | 3.22 % | 5.80 % | 86.7 % |
| font | | 24 px, x2 | 3.20 % | 5.60 % | 90.6 % |
| vectors | 74,608 (128 files) | every size and ratio | 0 % | 0 % | 0 % |

**Choice: vectors** (docs/DECISIONS.md D-M4-05-1). At 16 px they are clearly sharper: they match
the SVG reference pixel for pixel (`flutter_svg` compiles SVGs with the same compiler at run
time), while the font differs by about 3 % on average and up to 47 % in single edge pixels,
because the text engine places and anti-aliases the outline on its line box. The font would be
smaller, but the size rule only decides a tie. Only the vectors are committed.

`tools/design-tokens/tokens.py icons` (also part of `build`) compiles
`packages/design_system/assets/icons/<weight>/<glyph>.vec` with `vector_graphics_compiler` from
the workspace lock (offline, optimizers off, so no native library of the host is involved) and
writes `lib/src/generated/ds_icon_assets.g.dart` (`DsIconAssets.lookup`, `dsSemanticIcons`),
`assets/icons/LICENSE` (the Phosphor MIT text) and `assets/icons/MANIFEST.json` (package
version, compiler version, SHA-256 of every source SVG and asset, and `total_bytes`, the asset
size of the current mapping). `make tokens` rebuilds them in memory and fails on a stale, extra
or missing asset and on a mapped glyph without an SVG, naming the semantic name, the glyph and
the weight. `DsIcon` draws a glyph at `icon.size.*`; it needs a label or `decorative: true`, and
it spins the three spinning glyphs once per `motion.duration.pulse` only in full motion.

## Regeln

- **Icons nur über semantische Namen.** Eine Komponente fragt `trigger.working`, nicht
  `circle-notch`. Ein Tausch der Glyphe ist eine Änderung an einer Stelle.
- **Kein Emoji als Icon.** Emoji gibt es nur als freie Reaktion von Menschen. Status ist nie ein
  Emoji und nie eine Reaktion.
- **Icon nie allein.** Jedes Icon mit Bedeutung hat eine Beschriftung oder ein Screenreader-Label
  (siehe components.md). Farbe ist nie das einzige Merkmal; Zustände unterscheiden sich in der
  Form.
- **Gewichte:** `regular` als Standard, `fill` für aktiv und für erreichte Endzustände, `bold`
  für kleine Steuerzeichen (Pfeile, Plus, Schließen).
- **Drehen** nur für `trigger.working`, `run.running` und `state.reconnecting`, und nur im
  Bewegungs-Kontext `full`.

## Zuordnung (Auszug)

Vollständige Liste: `design/tokens/icon.tokens.json`, aufgelöst in
`design/build/tokens.resolved.json`.

| Semantischer Name | Glyphe | Stärke | Bedeutung |
|---|---|---|---|
| `trigger.received` | `check` | regular | Server hat die auslösende Nachricht angenommen |
| `trigger.queued` | `hourglass-medium` | regular | Lauf wartet in der Warteschlange |
| `trigger.working` | `circle-notch` | bold, dreht | Agent arbeitet |
| `trigger.done` | `check-circle` | fill | Lauf erfolgreich beendet |
| `trigger.failed` | `x-circle` | fill | Lauf oder Auslösung fehlgeschlagen |
| `trigger.followup` | `arrow-bend-down-right` | regular | Nachtrag an den laufenden Lauf zugestellt |
| `run.waiting` | `bell-ringing` | fill | Lauf wartet auf eine Antwort (waiting_input) |
| `run.failed` | `warning-octagon` | fill | Lauf fehlgeschlagen |
| `run.cancelled` | `prohibit` | regular | Lauf abgebrochen |
| `step.tool_call`, `step.subagent`, `step.workflow`, `step.background_job` | `wrench`, `robot`, `flow-arrow`, `clock-countdown` | regular | Schritt-Arten (`run_steps.kind`) |
| `channel.text`, `channel.forum` | `hash`, `chats-circle` | bold, regular | Kanal-Arten |
| `billing.personal`, `billing.organization` | `user-circle`, `buildings` | fill | Abzeichen Backend-Richtlinie |
| `security.step-up` | `shield-check` | fill | frische Bestätigung nötig, Freigabe-Karten |
| `action.stop` | `stop-circle` | fill | Lauf stoppen |

Die Auslöser-Zustände ersetzen die früheren Status-Zeichen aus Discord eins zu eins: empfangen,
Warteschlange, in Arbeit, fertig, Fehler, Nachtrag. Ihre Form ist verschieden, damit sie auch ohne
Farbe und bei kleiner Größe unterscheidbar bleiben.

## Bereichs-Icons

`spaces.icon` speichert einen Glyphen-Namen aus der Liste `space-icons` in `icon.tokens.json`
(zum Beispiel `headset`, `code`, `megaphone`, `flask`). Der Server prüft gegen dieselbe Liste
(aus `design/build/tokens.resolved.json`, Ticket "Erlaubte Farb- und Icon-Schlüssel im Server");
ein Emoji oder ein freier Name wird abgelehnt.
