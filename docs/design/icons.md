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
