# Komponenten

Status: Entwurf M0. Spezifikation der Kern-Komponenten für `packages/design_system`. Maße und
Farben nur als Token-Namen; Zahlen in [tokens.md](tokens.md) und [palette.md](palette.md). Jede
Text- oder Icon-Farbe auf einer Fläche steht als Paarung in `design/design-rules.json` und ist in
[contrast.md](contrast.md) für hell und dunkel geprüft. Bilder:
[Komponenten dunkel](previews/components-dark.png), [hell](previews/components-light.png); charts
[dark](previews/viz-dark.png), [light](previews/viz-light.png).
Datenfelder nach [03-datenmodell.md](../spec/03-datenmodell.md), Ereignisse nach
[04-sync-protokoll.md](../spec/04-sync-protokoll.md).

Gemeinsame Regeln für alle Komponenten:

- **Zustände:** Ruhe, Hover (nur Zeiger), gedrückt, Fokus (Ring `color.focus.ring`,
  `border-width.focus`, Abstand außen), deaktiviert, lädt, Fehler. Fokus ist immer sichtbar.
- **Trefferfläche** mindestens `density.target-min`, auch wenn die sichtbare Fläche kleiner ist.
- **Semantik:** Jede Komponente setzt Rolle und Label für Screenreader (Flutter `Semantics`,
  im Web als ARIA). Labels kommen aus `packages/l10n`, nie aus Daten des Agenten allein.
- **Farbe nie allein:** Zustände unterscheiden sich in Icon-Form und Text.
- **Bewegung** nur über `motion.*`; im Kontext `reduced` keine Bewegung.
- **Accent roles from `DsAccent`:** components read `color.accent.*`, `color.text.link`,
  `color.focus.ring` and `color.run.running` only from the theme extension `DsAccent` (Dart) or the
  `--ds-*` variables under `data-accent` (web), never from `DsColors`, so the instance accent
  (`branding.accent_color`) applies everywhere. Every variant is in [contrast.md](contrast.md).

## 1. Nachricht

Zweck: ein Beitrag in einem Thread (`messages`), von Mensch, Agent, App, Import oder System.

Anatomie: Avatar (Mensch rund, Agent abgerundetes Quadrat `radius.md`, App mit kleinem Abzeichen),
Kopfzeile (Name, bei Agenten Abzeichen "AGENT", Zeitstempel, bei auslösenden Nachrichten der
Auslöser-Status), Inhalt (Markdown), Anhänge, Komponenten (bei `kind = card`), Reaktionen,
Antwort-Bezug (Zitatzeile über dem Inhalt), Hover-Leiste (Antworten, Reaktion, Bearbeiten,
Anheften, Mehr).

| Variante | Daten | Darstellung |
|---|---|---|
| Text | `kind = text` | Markdown in `type.body` bzw. `density.message-font-size` |
| Folge-Nachricht | gleicher Autor innerhalb von `ui.message_group_window` | ohne Avatar und Kopf; Zeitstempel beim Hover |
| Sprachnachricht | `kind = voice` | Abspielknopf, Dauer, Fortschritt; Transkript darunter, sobald `transcript_status = done` |
| Live-Box-Anker | `kind = run` | rendert die Live-Box (Abschnitt 2) an dieser Stelle |
| Karte | `kind = card` | Inhalt plus Komponenten (Abschnitt 8) in `color.surface.raised` |
| System | `kind = system` | eine Zeile in `type.caption`, `color.text.secondary`, Icon links |
| Erwähnt mich | ich in `mentions` | Hinterlegung `color.attention.subtle`, Rückgrat `color.attention.border` mit `border-width.spine` |
| wird gesendet | Eintrag im Ausgangsfach | Deckkraft reduziert, Zeile "wird gesendet" mit `trigger.queued`-Icon |
| fehlgeschlagen | Ausgangsfach, dauerhafter Fehler | Zeile in `color.danger.fg` mit Grund, Knöpfe "Erneut senden", "Bearbeiten", "Verwerfen" |
| bearbeitet | `edited_at` gesetzt | Zusatz "(bearbeitet)" in `color.text.muted`; Fassungen per `message.revisions.read` |
| gelöscht | `deleted_at` gesetzt | Zeile "Nachricht gelöscht" in `color.text.muted`; keine Hover-Leiste |

Lange Antworten: Inhalt über `ui.message_collapse_lines` Zeilen wird eingeklappt; ein Textknopf
"Mehr anzeigen" (`color.text.link`) klappt auf. Code-Blöcke in `type.code` auf `color.code.bg`,
horizontal scrollbar, mit Kopierknopf.

Maße: Avatar `size.avatar-md`; Abstand Avatar zu Inhalt `space.12`; Gruppenabstand
`density.message-gap`; Zeilen-Innenabstand `density.message-pad-y`; Lesebreite höchstens
`layout.message-max-width`.

Barrierefreiheit: Eine Nachricht ist ein Semantik-Knoten mit Label "<Name>, <Zeit>, <Text>"; der
Auslöser-Status ist ein eigener Knoten mit Label (Abschnitt 5). Die Hover-Leiste ist per Tastatur
über die Nachricht erreichbar (Fokus auf Nachricht, dann Tab). Erwähnung wird im Label mit
"Erwähnt dich" eingeleitet.

## 2. Live-Box

Zweck: Darstellung eines Laufs (`runs`) mit Zustand, Kennzahlen, Statuszeile, Schritten und
letztem Text. Sie ist angeheftet oben im Thread, solange der Lauf aktiv ist, und klappt am Ende zu
einer Zeile zusammen.

Daten: Lauf-Dokument `run_document.v1` (04, Abschnitt 11): `state`, `version`, `status_line`,
`last_text_excerpt`, `metrics` (`context_pct`, Tokens, Unteragenten, Hintergrundjobs,
API-Wiederholungen), `cost`, `billing_mode_used`, `steps`, `followups`. Zustandswechsel kommen als
dauerhaftes `run.state_changed`, Inhalte als kurzlebiges `run.updated` (nur bei Fokus).

### 2.1 Laufend (`queued`, `running`, `waiting_input`)

Anatomie, von oben:

1. **Kopfzeile:** Zustands-Icon und Zustandswort (`run.*`), Laufzeit, Kontext in Prozent,
   Tokens, Kosten (alle in `type.code-sm`), Abzeichen Backend-Richtlinie (Abschnitt 10),
   Aufklapp-Pfeil.
2. **Statuszeile** (Abschnitt 4).
3. **Schritte:** Werkzeug-Chips (Abschnitt 3) in Reihenfolge `ordinal`, Unterschritte eingerückt
   (`parent_step_id`). Sichtbar sind die letzten `ui.livebox_visible_steps`; ältere hinter
   "n frühere Schritte".
4. **Letzter Text:** `last_text_excerpt` als Zitat mit Linie links in `color.border.default`.
5. **Fuß:** Knopf "Stopp" (Variante danger-outline, Icon `action.stop`), Hinweis zu Nachträgen.
   Bei `waiting_input` statt Hinweis das Formular oder die Knöpfe der Rückfrage (Abschnitt 8).
6. **Stillstand:** Nach `run.stall_after` ohne Fortschritt erscheint ein Banner in
   `color.warning.*` ("Seit n Minuten kein Fortschritt").

Fläche `color.surface.raised`, Rand `color.border.default`, Radius `radius.md`, Innenabstand
`density.livebox-pad`. Rückgrat links in `border-width.spine` mit der Farbe des Zustands:

| Zustand | Rückgrat und Icon | Icon | Wort |
|---|---|---|---|
| `queued` | `color.run.queued` | `run.queued` | "In Warteschlange" |
| `running` | `color.run.running` | `run.running` (dreht) | "Läuft" |
| `waiting_input` | `color.run.waiting` | `run.waiting` | "Wartet auf dich" bzw. "Wartet auf Alex" |
| `completed` | `color.run.completed` | `run.completed` | "Erledigt" |
| `failed` | `color.run.failed` | `run.failed` | "Fehlgeschlagen" plus Grund |
| `cancelled` | `color.run.cancelled` | `run.cancelled` | "Abgebrochen" |

Gründe (`state_reason`) werden übersetzt, nie roh gezeigt: `billing_mode_mismatch` ->
"Backend passt nicht", `access_revoked` -> "Zugriff entzogen", `stale` -> "Abgebrochen nach
Stillstand", `user_cancelled` -> "Von <Name> abgebrochen", `adapter_error` -> "Fehler im Adapter",
`cancel_timeout` -> "Adapter hat den Abbruch nicht bestätigt", `project_not_ready` -> "Projekt
nicht bereit". Unbekannte Gründe: "Fehlgeschlagen" plus Code in `type.code-sm`.

### 2.2 Eingeklappt (Endzustände)

Eine Zeile auf `color.surface.sunken`, Radius `radius.md`: Icon, "Lief <Dauer>", Zahl der
Werkzeuge, Unteragenten, Kosten mit "Schätzung", solange `cost_is_estimate`. Die Zusammenfassung
kommt aus `run.state_changed`; Aufklappen lädt das Dokument per REST (`run.read`). Studie B
schlug das Format "Lief 28 min · 3 Agenten · 1,20 USD" vor; es ist übernommen.

### 2.3 Mobil

Kompakte Zeile unter der Navigationsleiste (Höhe `density.control-height` plus Innenabstand):
Zustands-Icon, Zustand und Laufzeit, Statuszeile einzeilig gekürzt, Stopp-Knopf rechts mit
Trefferfläche `density.target-min`. Antippen öffnet die volle Live-Box als Blatt.

### 2.4 Barrierefreiheit

- Die Live-Box ist ein Semantik-Container mit Label "Lauf von <Agent>: <Zustand>, seit <Dauer>,
  Kontext <n> Prozent".
- Live-Region: Nur Zustandswechsel und neue Rückfragen werden angesagt (höflich), höchstens einmal
  je `a11y.live_announce_min_interval`. Patches an Schritten und Zahlen werden nie angesagt; sie
  würden Screenreader überfluten.
- Der Stopp-Knopf hat das Label "Lauf von <Agent> stoppen" und ist per Kürzel erreichbar
  (01-produkt.md, Tastenkürzel).

## 3. Werkzeug-Chip

Zweck: ein Schritt der Live-Box (`run_steps`).

Anatomie: Icon nach `kind` (`step.*`), Name (`name`, z. B. Werkzeugname) in `type.code-sm`,
Kurzbeschreibung (`title`) in `color.text.secondary`, Dauer bzw. "läuft n s". Fläche
`color.surface.sunken`, Radius `radius.sm`, Höhe `density.row-channel` minus Innenabstand.

| `status` | Icon-Farbe | Zusatz |
|---|---|---|
| `pending` | `color.run.queued` | "wartet" |
| `running` | `color.run.running`, Icon dreht | Laufzeit |
| `completed` | `color.run.completed` | Dauer |
| `failed` | `color.run.failed` | "fehlgeschlagen", Detail beim Aufklappen |
| `cancelled` | `color.run.cancelled` | "abgebrochen" |

Klick oder Enter öffnet `detail` (`run_step_detail.v1`) in einem Ausklapper; Inhalte sind gekürzt
auf `run.step_detail_max_bytes`. Label: "<Art> <Name>, <Status>, <Dauer>".

## 4. Statuszeile

Ein Satz, was der Agent gerade tut (`runs.status_line`), in `type.body-sm`, `color.text.primary`,
eine Zeile, am Ende gekürzt mit Tooltip. Inline-Code erlaubt. Leer: Zeile entfällt. Nicht Teil der
Live-Region (siehe 2.4).

## 5. Auslöser-Status

Zweck: Zustand an der auslösenden Nachricht eines Menschen (`messages.trigger_state`); ersetzt
die Status-Emoji aus Discord.

Platz: in der Kopfzeile der Nachricht nach dem Zeitstempel, Größe `icon.size.sm`. Antippen oder
Hover zeigt die Beschriftung und springt zur Live-Box.

| `trigger_state` | Icon | Farbe | Label (DE) | Label (EN) |
|---|---|---|---|---|
| `received` | `trigger.received` | `color.trigger.received` | Empfangen | Received |
| `queued` | `trigger.queued` | `color.trigger.queued` | In Warteschlange | Queued |
| `working` | `trigger.working` (dreht) | `color.trigger.working` | Wird bearbeitet | Working |
| `done` | `trigger.done` | `color.trigger.done` | Erledigt | Done |
| `failed` | `trigger.failed` | `color.trigger.failed` | Fehlgeschlagen | Failed |
| `followup_delivered` | `trigger.followup` | `color.trigger.followup` | Nachtrag zugestellt | Follow-up delivered |

Bei `failed` steht der Grund im Tooltip und im Semantik-Label ("Fehlgeschlagen: Projekt nicht
bereit"). Nachrichten ohne Auslösung (`trigger_state` leer) zeigen nichts.

## 6. Thread-Liste mit Status (Forum)

Zweck: dichte Liste der Beiträge eines Forums, sortiert nach letzter Aktivität
(`threads.last_activity_at`) oder Anlage (`forum_default_sort`).

Zeile (Höhe `density.row-post`, zwei Zeilen):

- links Zustands-Icon: aktiver Lauf (`run.*` aus "Läuft gerade"), sonst Erwähnung
  (`color.attention.fg`), sonst nichts;
- Titel in `type.body-sm`, fett bei Ungelesenem; Zeit rechts in `type.caption`;
- zweite Zeile: Abzeichen "Wartet auf dich" (`color.attention.bg`), Bediener-Agent,
  Erwähnungszähler, Tags (`category.<name>.chip-*`), bei Fehler der Grund in `color.danger.fg`.

Zustände der Zeile: Ruhe, Hover (`color.surface.hover`), gewählt (`color.surface.selected`),
Fokus. Kopf: Forum-Name, Knopf "Neuer Beitrag" (primär), Filter-Chips (Offen, Wartet auf mich,
Tags, Sortierung). Unten der Erledigt-Bereich: Zeile "Erledigt (n)", standardmäßig eingeklappt;
aufgeklappt folgen erledigte Beiträge mit `color.text.muted`.

Barrierefreiheit: Liste mit Rolle "Liste", Zeilen mit Label "<Titel>, <Zustand>, <Zeit>,
<n> ungelesen". Pfeiltasten bewegen, Enter öffnet.

## 7. Bereichs- und Kanalbaum

Zweck: Seitenleiste mit Bereichen (`spaces`) und Kanälen (`channels`), persönlich einklappbar und
sortierbar (`user_space_states`).

Bereichs-Zeile: Pfeil (`action.expand` bzw. `action.collapse`), Icon des Bereichs aus
`space-icons` in der Kategorie-Farbe (`category.<name>.on-sidebar`, auf einer leicht getönten
Kachel derselben Farbe), Name in `type.section`, `color.text.on-sidebar`. Eingeklappt mit Ungelesenem: Punkt
rechts.

Kanal-Zeile (Höhe `density.row-channel`): Icon nach Art (`channel.text`, `channel.forum`), Name in
`type.body-sm`.

| Zustand | Darstellung |
|---|---|
| gelesen | `color.text.on-sidebar-secondary` |
| ungelesen | `color.text.on-sidebar`, fett, Balken links |
| Erwähnungen | Zähler (`color.attention.bg`, `radius.pill`) |
| aktiver Lauf | Lauf-Punkt `category.spruce.on-sidebar` mit Puls `motion.duration.pulse` |
| stumm | `color.text.on-sidebar-muted`, Icon `action.mute` |
| gewählt | `color.surface.sidebar-selected` |
| Hover | `color.surface.sidebar-hover` |
| Fokus | Ring `color.focus.ring-on-sidebar` |

Oben: Zeichen und Produktname (`branding.product_name`), Schnellwechsel-Feld ("Springen zu" mit
Kürzel), Eintrag "Läuft gerade" mit Zähler. Unten: eigener Avatar, Einstellungen.

Verhalten: Einklappen per Klick, Enter oder Pfeil links/rechts; Zustand wird per `sidebar.update`
synchronisiert. Ziehen ändert die persönliche Reihenfolge (`user_space_states.sort_key`); die
globale Reihenfolge ändert nur `space.reorder` in der Verwaltung.

Barrierefreiheit: Baum mit Rolle "Baum"; Bereiche sind ausklappbare Knoten mit Zustand
"eingeklappt" bzw. "ausgeklappt"; Kanal-Label "<Name>, <n> ungelesen, <m> Erwähnungen, Lauf
aktiv".

## 8. Komponenten an Nachrichten (Knöpfe, Auswahl, Formular)

Zweck: interaktive Elemente eines Agenten oder einer App (`message_components`), z. B.
Befehlsfreigabe ("Einmal erlauben", "Für diese Sitzung", "Immer", "Ablehnen"), Modellwahl,
Rückfragen.

| Typ | Darstellung |
|---|---|
| `button` | Knopf; Stil aus `definition.style`: primär (`color.accent.bg`), Erfolg (`color.success.bg`), Gefahr (`color.danger.bg`), sekundär (Rand `color.border.control`), ruhig |
| `select` | Auswahlfeld mit Menü (`elevation.menu`), Einfach- oder Mehrfachauswahl |
| `form` | Felder mit Beschriftung über dem Feld, Absenden-Knopf; Prüfung nach Schema der Komponente |

Zustände (`message_components.state`):

| Zustand | Darstellung |
|---|---|
| `active`, berechtigt | bedienbar |
| `active`, nicht berechtigt | sichtbar, deaktiviert, Hinweis mit `security.lock`: "Nur <Auslöser> oder ein Owner kann das freigeben" bzw. "Dafür fehlt dir das Recht 'Aktionen freigeben'" |
| gesendet, Antwort offen | gedrückter Knopf zeigt Lade-Icon; andere Knöpfe gesperrt |
| `used` | alle Knöpfe weg; Zeile "<Name> hat '<Wahl>' gewählt, <Zeit>" mit `trigger.done` |
| `expired` | Zeile "Abgelaufen" in `color.text.muted` |
| `disabled` | Zeile "Nicht mehr verfügbar" |
| Fehler | Banner in `color.danger.*` mit Grund; Knöpfe wieder bedienbar, wenn die Komponente noch `active` ist |

Regeln: Eine Bedienung ist eine Interaktion mit `client_interaction_id` aus dem Ausgangsfach; ein
Doppelklick erzeugt keine zweite. Zerstörende Wahl ("Immer") verlangt keine zweite Bestätigung,
weil sie den Rechten folgt; die Karte zeigt aber, was "Immer" bedeutet. Knöpfe in einer Reihe,
auf Mobilgeräten untereinander in voller Breite (Bild "Mobil").

Barrierefreiheit: Gruppe mit Label aus dem Kartentitel; Knöpfe in Lesereihenfolge; kein Kürzel
löst eine Freigabe global aus (Fehlbedienung); Enter oder Leertaste nur auf dem fokussierten
Knopf.

## 9. Eingabe mit Slash-Vorschlägen

Zweck: Nachricht verfassen, Befehle (`commands`) mit Optionen und Autovervollständigung,
Erwähnungen, Anhänge, Sprachaufnahme.

Anatomie: Feld (Fläche `color.surface.input`, Rand `color.border.control`, Radius `radius.lg`,
Mindesthöhe `density.composer-min-height`, wächst bis `ui.composer_max_lines`), links Anhang,
rechts Mikrofon und Senden. Darüber die Zeile "Tippen" ("Sam schreibt gerade"), bei aktivem Lauf
Platzhalter "Nachtrag an <Agent>".

Vorschlagsliste (Overlay `color.surface.overlay`, `elevation.menu`): öffnet bei "/" am
Zeilenanfang und bei "@". Zeilen (`density.row-suggestion`): Befehl in Mono, Beschreibung,
Herkunft (Agent) und Hinweis "Steuerbefehl" bei `control = true`. Reihenfolge: Befehle des
Bedieners dieses Kanals zuerst; gleiche Namen anderer Agenten werden mit dem Handle qualifiziert
(03-datenmodell.md, `commands`). Nach Auswahl eines Befehls zeigt die Eingabe die Optionen als
Platzhalter; Optionen mit `autocomplete` holen Vorschläge über `command.autocomplete`, gedrosselt
mit `ui.autocomplete_debounce`. Erwähnungs-Vorschläge zeigen nur Personen und Agenten, die die
Person sehen darf.

Zustände: leer, Text, Befehl gewählt, Optionen unvollständig (Senden deaktiviert mit Grund),
offline (Senden landet im Ausgangsfach), keine Schreibrechte (Feld ersetzt durch Hinweis "Nur
lesen in diesem Kanal").

Tastatur: Pfeile wählen, Tab oder Enter übernimmt, Escape schließt; Enter sendet, Shift+Enter
macht eine neue Zeile (umkehrbar über `composer.enter_sends`); Pfeil hoch im leeren Feld bearbeitet
die eigene letzte Nachricht.

Barrierefreiheit: Feld mit Label "Nachricht an <Kanal oder Thread>"; Vorschlagsliste als Listbox
mit aktivem Eintrag; Anzahl der Vorschläge wird angesagt.

## 10. Abzeichen Backend-Richtlinie

Zweck: zeigt in Thread-Kopf, Live-Box und Agenten-Karte, worauf der Agent läuft
(`agents.billing_mode`, Baseline Abschnitt 5).

| Modus | Icon | Farbe | Text |
|---|---|---|---|
| `personal` | `billing.personal` | `color.badge.personal-bg`, `-fg` | "Persönlich" |
| `organization` | `billing.organization` | `color.badge.organization-bg`, `-fg` | "Organisation" |

Höhe `size.badge-height`, Radius `radius.pill`, Text `type.label`. Weicht der gemeldete Ist-Modus
(`reported_billing_mode`) vom Soll ab, zeigt die Agenten-Karte zusätzlich ein Warn-Abzeichen
(`color.warning.*`, "Ist: Organisation"); Läufe scheitern dann mit `billing_mode_mismatch`. Kosten
bei `organization` mit Zusatz "Schätzung", bis abgeglichen; bei `personal` mit Zusatz "nur
Information". Label: "Backend: Persönlich, läuft auf dem Abo von <Name>".

## 11. Visualisierungen (`bothy_viz`)

Zweck: Grundformen für die Ansichten aus [25-visualisierung.md](../spec/25-visualisierung.md) in
Apps und Konsole; dieselben Rollen nutzt das Grafana-Panel über die TypeScript-Ausgabe der Tokens.

| Grundform | Aussehen | Tokens |
|---|---|---|
| Bahn (Bereich) | Zeile mit Kopf links (Kategorie-Farbe und Icon des Bereichs), Linie `color.viz.grid` zwischen Bahnen | `color.border.*`, `space.*` |
| Lauf-Karte | wie eine eingeklappte Live-Box: Rückgrat links in der Zustandsfarbe des Laufs, Agent, Werkzeug-Chip (Abschnitt 3), Abzeichen Backend-Richtlinie (Abschnitt 10), Kosten in Mono | Rollen der Live-Box, `type.code` |
| Kennzahl-Kachel | Zahl groß in Mono, Etikett darüber, Vergleich zum Vorzeitraum als Text mit Icon (nie nur Farbe) | `type.display`, `type.label` |
| Fläche und Balken | Serien in `color.viz.series.1` bis `.8` in fester Reihenfolge; Achsen `color.viz.axis`; Füllung `color.viz.area-fill`; grid lines `color.viz.grid` | `color.viz.*` |
| SLO-Ampel | Punkt mit Icon und Text "im Ziel", "Budget knapp", "Ziel verfehlt", "zu wenig Daten" und Etikett "eigene Messung" | `color.success.*`, `color.warning.*`, `color.danger.*`, neutrale Rolle für "zu wenig Daten" |
| Zeitleiste | Abspielkopf als senkrechte Linie, Steuerung mit Icons aus `icon.tokens.json` | `motion.*` |

Regeln:

- Kein Verlauf, kein Leuchten, keine 3D-Optik (Verbotsliste in [README.md](README.md)).
- Jede Grundform hat Semantik und eine Tabellenansicht mit denselben Werten.
- The roles `color.viz.*` are in the tokens; series and axis are checked against `surface.canvas`,
  `surface.list` and `surface.raised` with contrast `ui` in both themes
  ([contrast.md](contrast.md)). `grid` and `area-fill` never carry information alone and therefore
  have no contrast requirement. The series order starts with blue and orange, which stay
  distinguishable with red-green color blindness; a second cue (label or pattern) is required
  anyway. `make tokens` also checks every adjacent pair of series (1 with 2 up to 7 with 8, and
  8 with 1 because the order wraps when a view needs more than eight groups) under simulated
  protanopia, deuteranopia and tritanopia: the OKLab distance of the simulated colours must reach
  `cvd_min_delta_e_ok` of `design/design-rules.json` (section "Color vision deficiency" of
  [contrast.md](contrast.md)).
- The SLO traffic light uses only `color.success.*`, `color.warning.*` and `color.danger.*`; the
  same check holds `success.fg`, `warning.fg` and `danger.fg` pairwise apart by
  `cvd_min_delta_e_ok`, and every state carries an icon and text as well.
- TV-Modus: dunkles Theme, Dichte `comfortable`, Schriftgrößen eine Stufe größer; keine Bewegung im
  Kontext `reduced`.

## 12. Review-Liste (nicht automatisch prüfbar)

- Keine Glas-, Unschärfe- oder Leuchteffekte; keine Verläufe.
- Keine schwebenden Karten mit großem Schatten; Schatten nur aus `elevation.*`.
- Kein Willkommens-Banner, keine Terminal-Optik, keine Deko-Illustrationen.
- Kein Emoji in UI-Texten oder als Icon; Status nie als Reaktion.
- Jede neue Farbe ist eine Rolle mit Paarung in `design/design-rules.json`.
- Golden-Tests in hell und dunkel, in `comfortable` und `touch`, mit Textskalierung 1 und 2
  (15-client-architektur.md, Teststrategie).

## 13. Konfigurationswerte

Die Werte `ui.*` und `a11y.*`, die diese Seite nennt, stehen mit Standard in
[01-produkt.md](../spec/01-produkt.md), Abschnitt "Konfigurationswerte dieses Kapitels".
