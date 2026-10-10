# Abstände, Radien, Erhebung, Bewegung, Dichte

Quellen: `design/tokens/space.tokens.json`, `elevation.tokens.json`, `motion.*.tokens.json`,
`density.*.tokens.json`. Erzeugte Werte: `design/build/tokens.css`, `design/build/dart/`.
Maßgeblich sind die JSON-Dateien; Tabellen hier zeigen nur die Namen und die Absicht.

## Abstände

Raster 4 px. Dazu 2 px und 6 px für dichte Listen und Chips. Namen sind die Pixelwerte
(`space.4`, `space.12`), weil das Raster primitiv ist; Bedeutung bekommen Abstände in den
Komponenten-Spezifikationen.

| Token | Typische Verwendung |
|---|---|
| `space.2`, `space.4` | Abstand Icon zu Text in Chips, Zeilen-Innenabstand compact |
| `space.6`, `space.8` | Chips untereinander, Listenzeilen, Knopf-Gruppen |
| `space.12`, `space.16` | Spalteninnenrand, Live-Box-Innenabstand, Abstand Avatar zu Text |
| `space.20`, `space.24` | Abschnitte in Einstellungen und Konsole |
| `space.32` bis `space.64` | leere Zustände, Einrichtung |

## Radien

Knappe Radien: das Produkt ist ein Werkzeug. `radius.pill` nur für Zähler und Abzeichen.

| Token | Verwendung |
|---|---|
| `radius.xs` | Inline-Code, Tastenkappen |
| `radius.sm` | Knöpfe, Chips, Kanalzeilen, Eingabefelder |
| `radius.md` | Live-Box, Karten, Agenten-Avatare, Code-Blöcke |
| `radius.lg` | Eingabe, Menüs, Schnellwechsel |
| `radius.xl` | Telefon-Rahmen in der Vorschau, große Dialoge |
| `radius.pill` | Zähler, Abzeichen |

Formsprache: Menschen haben runde Avatare, Agenten abgerundete Quadrate (`radius.md`). So ist auf
einen Blick klar, wer geschrieben hat, auch ohne Farbe.

## Linien

| Token | Wert und Zweck |
|---|---|
| `border-width.hairline` | Spaltengrenzen, Karten, Zeilentrenner |
| `border-width.focus` | Fokusring, mindestens 2 px (Ziel WCAG 2.2 SC 2.4.13) |
| `border-width.spine` | Rückgrat links an Live-Box und Erwähnungen, Farbe aus dem Zustand |

## Erhebung

Flächen liegen flach und trennen sich durch Farbe (`color.surface.*`) und Linien. Schatten gibt es
nur für Ebenen über dem Inhalt:

| Token | Verwendung |
|---|---|
| `elevation.menu` | Kontextmenü, Vorschlagsliste, Tooltip |
| `elevation.dialog` | Dialog, Schnellwechsel, Freigabe-Assistent |

Die Schattenfarbe `color.shadow` ist je Theme eigen (im dunklen Theme dunkler und dichter, damit
der Schatten auf dunklen Flächen überhaupt trägt). Kein Glühen, keine farbigen Schatten, keine
Unschärfe hinter Ebenen.

## Bewegung

Bewegung zeigt einen Zustandswechsel und schmückt nie. Zwei Kontexte im Resolver:

| Token | `full` | `reduced` | Verwendung |
|---|---|---|---|
| `motion.duration.move-fast` | kurz | 0 | Einklappen von Bereichen, Hover |
| `motion.duration.move` | mittel | 0 | Live-Box klappt zusammen, Spalten |
| `motion.duration.move-slow` | lang | 0 | eigene Übergänge im Navigationsstapel (Plattformübergang hat Vorrang) |
| `motion.duration.fade` | kurz | kürzer | Überblenden ohne Bewegung |
| `motion.duration.pulse` | Periode | 0 | Drehen des Lauf-Icons, Puls des Lauf-Punkts; 0 heißt statisch |
| `motion.easing.standard`, `.enter`, `.exit` | Kurven | linear | |

Regeln:

- `reduced` gilt, wenn die Systemeinstellung "Bewegung reduzieren" aktiv ist oder die Person
  `appearance.reduce_motion = on` gewählt hat (`off` erzwingt `full`). WCAG 2.2 SC 2.3.3 ist das
  Ziel (https://www.w3.org/TR/WCAG22/, abgerufen 2026-10-10 [belegt]).
- Ein laufender Agent zeigt im Kontext `reduced` ein statisches Icon; der Zustand bleibt über
  Icon-Form und Beschriftung erkennbar.
- Live-Box-Patches animieren nichts. Neue Schritte erscheinen ohne Einfliegen; nur der Puls des
  Zustands-Icons zeigt "läuft".
- Kein Parallax, kein Wackeln bei Fehlern, keine Konfetti-Effekte.

## Dichte

Drei Kontexte im Resolver; die App wählt ihn, nicht die Person allein:

| Kontext | Gilt für | Wahl |
|---|---|---|
| `comfortable` | Desktop, Web-Konsole | Standard |
| `compact` | Desktop | Einstellung `appearance.density = compact` |
| `touch` | iPhone, Android | fest |

| Token | Bedeutung |
|---|---|
| `density.row-channel` | Höhe einer Kanalzeile |
| `density.row-post` | Höhe einer Beitragszeile im Forum (zweizeilig) |
| `density.row-suggestion` | Zeile in Vorschlagsliste und Schnellwechsel |
| `density.control-height` | Knöpfe, Auswahlfelder |
| `density.target-min` | kleinste Trefferfläche: WCAG 2.2 SC 2.5.8 auf Desktop, 48 dp auf Touch |
| `density.message-gap`, `density.message-pad-y` | Abstände im Verlauf |
| `density.message-font-size` | Nachrichtentext |
| `density.livebox-pad` | Innenabstand der Live-Box |
| `density.composer-min-height` | Mindesthöhe der Eingabe |

Belege Trefferflächen: WCAG 2.2 SC 2.5.8 verlangt 24 mal 24 CSS-Pixel
(https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html, abgerufen 2026-10-10
[belegt]); Android empfiehlt 48 dp (https://support.google.com/accessibility/android/answer/7101858,
abgerufen 2026-10-10 [belegt]); Apple nennt 44 pt [abgeleitet aus Sekundärquellen, Primärseite der
Human Interface Guidelines nicht gelesen]. `touch` nutzt den größeren Material-Wert, damit beide
Plattformen erfüllt sind. Alle Zahlen: [Token-Referenz](tokens.md).

Die sichtbare Fläche eines Elements darf kleiner sein als `density.target-min`; die Trefferfläche
nie. Flutter-Tests prüfen das mit `androidTapTargetGuideline` und `iOSTapTargetGuideline`
(https://api.flutter.dev/flutter/flutter_test/meetsGuideline.html, abgerufen 2026-10-10 [belegt]).
