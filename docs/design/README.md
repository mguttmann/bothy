# Design-System

Status: Entwurf M0. Quelle aller Werte: `design/tokens/` (W3C-DTCG-Format 2025.10). Erzeugt mit
`uv run tools/design-tokens/tokens.py build`; geprüft mit `... check`. Entscheidung: ADR 0030.
Komponenten: [components.md](components.md). Produktsicht: [01-produkt.md](../spec/01-produkt.md).

![Desktop, dunkel](previews/layout-dark.png)

![Desktop, hell](previews/layout-light.png)

Weitere Bilder: [Komponenten hell](previews/components-light.png),
[Komponenten dunkel](previews/components-dark.png), [Mobil hell](previews/mobile-light.png),
[Mobil dunkel](previews/mobile-dark.png), [Palette hell](previews/palette-light.png),
[Palette dunkel](previews/palette-dark.png). Alle Bilder entstehen aus `design/build/preview.html`
(`uv run tools/design-tokens/tokens.py previews`); die Seite selbst ist eigenständig und hat
Schalter für Theme, Dichte und Bewegung.

## Prinzipien

1. **Aufbau wie Discord, Gesicht eigen.** Spalten, Dichte, einklappbare Bereiche und Foren mit
   Beitragsliste folgen Discord, weil die Nutzer das kennen. Farbe, Schrift, Icons, Logo und
   Formen sind eigen. Grund: Marke und Lizenz (Studie B), und die Anforderung "unverwechselbar".
2. **Farbe trägt Bedeutung.** Jede Farbe ist eine Rolle: Marke, Zustand eines Laufs,
   Auslöser-Status, Kategorie eines Bereichs, Abzeichen der Backend-Richtlinie. Keine Farbe ist Schmuck.
   So ist die Oberfläche farbig, ohne laut zu sein.
3. **Die Seitenleiste ist Markenfläche.** Die Spalte mit Bereichen und Kanälen ist in beiden
   Themes tiefes Fichtengrün. Das macht das Produkt auf einen Blick erkennbar und trennt
   Navigation klar von Inhalt.
4. **Agentenarbeit sichtbar, aber ruhig.** Die Live-Box zeigt Zustand, Kennzahlen und Schritte.
   Ihr Rückgrat links trägt die Zustandsfarbe; Zahlen stehen in Mono; Bewegung gibt es nur für
   "läuft" und lässt sich abschalten.
5. **Lesbarkeit zuerst.** Schrift Atkinson Hyperlegible (entworfen für Menschen mit
   Sehschwäche), Kontrast AA als harter Test, dynamische Schriftgröße bis 200 Prozent.
6. **Dicht wie ein Werkzeug.** Knappe Radien, Linien statt Schatten, drei Dichten
   (comfortable, compact, touch). Agenten liefern lange Texte; der Platz gehört dem Inhalt.
7. **Eine Quelle, viele Ausgaben.** Werte stehen nur in `design/tokens/`. CSS, Dart, Doku und
   Vorschau entstehen daraus. Wer eine Farbe ändert, ändert JSON und lässt die Tests laufen.

## Verbotsliste

Die Anforderung verbietet den generischen KI-Look. Was sich messen lässt, prüft das Werkzeug bei
jedem `build` und `check`; der Rest ist Teil der Review-Liste in [components.md](components.md).

| Verbot | Grund | Durchsetzung |
|---|---|---|
| Discord-Blurple `#5865F2` (https://discord.com/branding, abgerufen 2026-10-10 [belegt]) und Farben in seiner Nähe | fremde Marke | Wächter in `design/design-rules.json`: OKLab-Abstand jeder Palettenfarbe zu Blurple; Marke und Akzent mit größerem Abstand |
| Discord-Schrift, Discord-Icons, Discord-Logo | fremde Marke, Lizenz | Schrift-Token `font.family.*`, Icon-Set Phosphor, eigenes Zeichen |
| Standard-Grautöne (Grau ohne Farbstich) | generischer Look | Wächter: Mindest-Chroma für `palette.granite.*`, `color.surface.*`, `color.border.*` |
| Beige, warmer Sand, Creme | generischer Look, Nähe zu bekannten KI-Paletten | Wächter: verbotener warmer Farbtonbereich für Neutrale und Flächen |
| Terrakotta oder Orange auf Beige als Marke (Claude- oder Anthropic-ähnlich) | Verwechslung | Wächter: verbotener Farbtonbereich Orange bis Sand für Marke und Akzent |
| Lila als Akzent, Verläufe von Lila nach Blau | generischer Look (Studie B, Anforderung 18) | Wächter: verbotener Farbtonbereich Lila für Marke und Akzent; das Werkzeug kennt keinen Typ `gradient` und lehnt ihn ab |
| Glassmorphism, Glühen, Neon | generischer Look | kein Token für Unschärfe oder Leuchten; Schatten nur in `elevation.*`; Review |
| Emoji als Icons | Anforderung 18 | Icons nur aus `icon.tokens.json`; ein Test über die ARB-Dateien lehnt Emoji in UI-Texten ab (Ticket in 15-client-architektur.md) |
| Gedankenstriche in UI-Texten | Vorgabe | derselbe ARB-Test lehnt U+2013 und U+2014 ab |
| Status als Reaktion, Status als Emoji | Studie B: Status-Emoji waren der alte Behelf | Auslöser-Status ist ein Feld (`trigger_state`) mit eigenen Icons |
| Schwebende Karten, Willkommens-Banner, Terminal-Optik, Brutalismus | Studie B | Review |

Guard scope of the instance accent (M4-03): the guard "Marke und Akzent" (scope `palette.spruce.`,
`color.accent.`) protects the product's own brand and checks the base roles only. The accent
variants of an instance are not checked against it, because chapter 01 section 12 deliberately
allows `heather`, `rowan` and `amber` as the instance owner's choice from the checked palette,
while the product brand stays spruce; contrast applies to the variants in full. The guard "Keine
Discord-Nähe in der ganzen Palette" covers every scale and so every variant.

## Namensherkunft

Der Arbeitsname **Bothy** ist das schottische Wort für eine einfache, offene Schutzhütte, die
jeder unentgeltlich nutzen darf ("A bothy is a basic shelter, usually left unlocked and available
for anyone to use free of charge", https://en.wikipedia.org/wiki/Bothy, abgerufen 2026-10-10
[belegt]). Das Bild passt: ein Ort, an dem Menschen und Agenten unterkommen und arbeiten; "bot"
steckt im Wort. Recherche und Alternativen: [naming.md](naming.md). Der Name ist Konfiguration
(ADR 0044); das Design hängt nicht an ihm.

Die Palette nimmt das Bild auf: Farben der Hochland-Landschaft rund um eine Hütte.

| Name | Deutsch | Rolle |
|---|---|---|
| `spruce` | Fichte | Marke, Akzent, Seitenleiste, laufender Agent |
| `gorse` | Ginster | Signal: Erwähnung, "wartet auf dich", Freigabe nötig |
| `moss` | Moos | Erfolg, erledigt |
| `rowan` | Vogelbeere | Fehler, zerstörende Aktion |
| `amber` | Bernstein | Warnung, Stillstand |
| `loch` | See | Information, Nachtrag zugestellt |
| `heather` | Heide | nur Kategorie (Bereiche, Tags) und Abzeichen "Persönlich" |
| `granite` | Granit | Neutrale mit kühlem Stich |

Das Zeichen ist ein Giebel (Dach und Wände einer Hütte) in Ginster auf Fichte. Es ist bewusst
einfach, damit es als App-Icon, in der Seitenleiste und als Favicon der Konsole trägt.

## Warum diese Farben

- **Fichte als Marke.** Ein grünstichiges Petrol ist im Feld der Team-Chats selten besetzt
  (Discord Blurple, andere Produkte Violett, Aubergine oder Blau) und vom verbotenen Lila und
  Terrakotta weit entfernt. Es trägt hell und dunkel: als tiefe Fläche (Seitenleiste) und als
  helle Akzentfarbe im dunklen Theme.
- **Ginster als Gegenpol.** Gelb ist die einzige warme Signalfarbe. Sie steht für alles, was auf
  einen Menschen wartet. Die dunklen Stufen driften Richtung Gold, damit Text nicht oliv wirkt.
- **Granit statt Grau.** Die Neutralen haben einen kühlen Stich (Farbton 210). Im dunklen Theme
  ist er sichtbar und gibt dem Produkt seinen Grundton; im hellen Theme hält er Weiß frisch.
- **Kategorien.** Bereiche und Tags bekommen eine von acht Farben (`category.*`). Der Server
  speichert nur den Schlüssel (`heather`, `loch`, ...), nie einen freien Farbwert; so bleibt jede
  Kombination im Kontrast-Test.
- **OKLCH als Quelle.** Die Tonleitern sind in OKLCH notiert, weil gleiche Helligkeitsstufen dort
  gleich hell aussehen. Der Generator rechnet sRGB und lehnt Farben außerhalb des sRGB-Gamuts ab.

## Dateien

| Pfad | Inhalt |
|---|---|
| `design/tokens/theme.resolver.json` | Einstieg (DTCG Resolver Module 2025.10): Basis plus Modifikatoren `theme`, `density`, `motion` |
| `design/tokens/color.palette.tokens.json` | primitive Tonleitern (OKLCH) |
| `design/tokens/color.light.tokens.json`, `color.dark.tokens.json` | semantische Farbrollen je Theme, gleiche Schlüssel |
| `design/tokens/typography.tokens.json` | Schriften und Typo-Rollen |
| `design/tokens/space.tokens.json` | Abstände, Radien, Linien, Spaltenbreiten, feste Maße |
| `design/tokens/elevation.tokens.json` | Schatten für Menü und Dialog |
| `design/tokens/motion.full.tokens.json`, `motion.reduced.tokens.json` | Dauer und Kurven; reduzierte Bewegung |
| `design/tokens/density.*.tokens.json` | Dichte comfortable, compact, touch |
| `design/tokens/icon.tokens.json` | Icon-Größen und Zuordnung semantischer Namen zu Phosphor-Glyphen |
| `design/design-rules.json` | Kontrast-Paarungen und Palettenwächter |
| `design/build/` | erzeugt, nicht bearbeiten: CSS, Dart, aufgelöstes JSON, Vorschau |
| `tools/design-tokens/` | Generator, Tests, eingebettete Schriften und Icons |
| [palette.md](palette.md), [contrast.md](contrast.md), [tokens.md](tokens.md) | erzeugt |
| [typography.md](typography.md), [spacing-radii-elevation-motion.md](spacing-radii-elevation-motion.md), [icons.md](icons.md), [components.md](components.md), [naming.md](naming.md) | von Hand |

## Ablauf bei einer Änderung

1. Wert in `design/tokens/` ändern (nie in `design/build/`).
2. `uv run tools/design-tokens/tokens.py build`: prüft Kontrast und Wächter, schreibt alle
   Ausgaben. Bricht ab, wenn eine Paarung unter AA fällt.
3. `uv run tools/design-tokens/tokens.py previews` und die Bilder ansehen.
4. Alles zusammen committen. Das Gate `uv run tools/design-tokens/tokens.py check` schlägt fehl,
   wenn eine erzeugte Datei nicht zur Quelle passt.

Neue Paarung in einer Komponente: zuerst in `design/design-rules.json` eintragen, dann bauen.
Neues Icon: in `icon.tokens.json` eintragen und `tokens.py vendor` mit dem gepinnten Paket
ausführen (siehe [tools/design-tokens/README.md](../../tools/design-tokens/README.md)).

## Laufzeit-Branding

Eine Instanz kann Produktname, Logo und Akzent einstellen (`branding.*`, ADR 0044). Der Akzent ist
kein freier Farbwert, sondern ein Schlüssel aus `category.*` (Standard `spruce`), weil nur diese
Kombinationen den Kontrast-Test bestehen. Wählt eine Instanz einen anderen Akzent, tauscht die App
die Rollen `color.accent.*`, `color.text.link`, `color.focus.ring` und `color.run.running` gegen
die gleich hellen Stufen der gewählten Leiter.

The generator builds these variants (M4-03). The member `accent_variants` of
`design/design-rules.json` names the allowed accents (`spruce`, `loch`, `heather`, `rowan`,
`amber`, `gorse`, `moss`; default `spruce`) and the swapped roles. Per theme a role is swapped only
where it aliases a step of `palette.spruce`; it then takes the same step of the chosen scale, or
the nearest step by number when the scale lacks it (a tie goes darker in light, lighter in dark).
Every other role keeps its value: in dark, `color.focus.ring` aliases `gorse` and stays, and so do
the sidebar, the text on the accent and the category colours. If a variant fails a contrast
pairing of the rules file, it moves one step at a time along its scale towards more contrast until
every pairing passes; if no step passes, `build` and `check` fail. Every pairing that contains a
swapped role is checked for every accent in light and dark and listed in
[contrast.md](contrast.md), moved steps marked. The outputs: the Dart extension `DsAccent` (one
constant per theme and accent, `DsAccent.of(key, brightness)`, an unknown key gives `spruce`),
blocks `[data-accent="<key>"]` in `tokens.css` (on the same element as `data-theme`) and the member
`accents` of `tokens.resolved.json`. The default accent equals the base roles exactly. The preview
shows the default accent only.
