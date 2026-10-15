# Typografie

Quelle: `design/tokens/typography.tokens.json`. Werte in px; die Apps skalieren sie mit der
Systemeinstellung (iOS Dynamic Type, Android Schriftgröße, Windows Textgröße) über
`MediaQuery.textScaler`. Bilder: [Komponenten](previews/components-dark.png).

## Schriftwahl

| Rolle | Schrift | Lizenz | Achsen |
|---|---|---|---|
| Oberfläche, Nachrichten | Atkinson Hyperlegible Next | SIL Open Font License 1.1 | variabel, Gewicht 200 bis 800, mit Kursiv |
| Code, Werkzeugnamen, IDs, Kennzahlen | Atkinson Hyperlegible Mono | SIL Open Font License 1.1 | variabel, Gewicht 200 bis 800, mit Kursiv |

Belege: Paketdaten von `@fontsource-variable/atkinson-hyperlegible-next@5.3.0` und
`@fontsource-variable/atkinson-hyperlegible-mono@5.3.0` (Lizenz OFL-1.1, Achse `wght` 200 bis
800; https://fontsource.org/fonts/atkinson-hyperlegible-next, abgerufen 2026-10-10 [belegt]).
Quell-Repositorys: https://github.com/googlefonts/atkinson-hyperlegible-next und
https://github.com/googlefonts/atkinson-hyperlegible-next-mono (abgerufen 2026-10-10 [belegt]).
Die Schriftfamilie stammt vom Braille Institute und ist für Menschen mit Sehschwäche entworfen;
die Fassung "Next" erschien 2025 mit sieben Gewichten und einer Mono-Variante [belegt über die
Fontsource-Seite und Presseberichte, Primärseite des Braille Institute nicht gelesen].

Gründe:

1. **Lesbarkeit als Eigenschaft der Schrift.** Verwechselbare Zeichen (I, l, 1; 0, O) sind klar
   getrennt. In einem Chat mit IDs, Pfaden, Befehlen und Code ist das ein Arbeitsvorteil, kein
   Schmuck.
2. **Eigenes Gesicht.** Die Schrift ist erkennbar anders als die üblichen Grotesken (Inter,
   Systemschriften) des generischen Looks und anders als Discords Hausschrift.
3. **Eine Familie für Text und Code.** Next und Mono teilen Formen und Laufweite; Kennzahlen in
   der Live-Box (Mono) stehen ruhig neben Fließtext.
4. **Variabel.** Ein Font-File je Stil deckt alle Gewichte ab; das hält App-Bundles klein.
5. **Freie Lizenz.** OFL erlaubt Einbetten in Apps und Weitergabe; der Lizenztext liegt bei
   (`tools/design-tokens/vendor/fonts/*.OFL.txt`, im App-Paket ebenso).

Studie B empfahl Systemschriften (SF, Segoe UI). Die Anforderung "unverwechselbar" und die
Lesbarkeit wiegen schwerer; die Systemschriften bleiben als Rückfall in `font.family.*` stehen.

Verworfen: Inter (generischer Look), Systemschrift allein (kein eigenes Gesicht, je Plattform
anders), Bricolage Grotesque (stark, aber für dichte Listen zu eigenwillig), Material- oder
Discord-nahe Schriften.

## Rollen

Komponenten nutzen nur diese Rollen, nie `font.size.*` direkt.

| Rolle | Charakter | Verwendung |
|---|---|---|
| `type.display` | sehr groß, extra fett, enger gesetzt | leere Zustände, Einrichtung, Kennzahl-Kacheln |
| `type.title` | groß, fett | Titel einer Ansicht, Dialog |
| `type.heading` | mittel, halbfett | Kanal- und Thread-Kopf, Abschnitte |
| `type.body` | Grundgröße, normal | Nachrichtentext Desktop |
| `type.body-strong` | Grundgröße, halbfett | Autorname |
| `type.body-sm` | eine Stufe kleiner | Listenzeilen, Kanalnamen, Live-Box |
| `type.label` | klein, halbfett | Knöpfe, Chips, Abzeichen |
| `type.section` | klein, fett, weiter gesetzt | Bereichs-Titel in der Seitenleiste |
| `type.caption` | klein, normal | Zeitstempel, Hilfetexte |
| `type.code` | Mono, eine Stufe unter Grundgröße | Code-Blöcke, Inline-Code |
| `type.code-sm` | Mono, klein, mittel | Werkzeug-Chips, Kosten, Token-Zahlen |

Größen, Gewichte, Zeilenhöhen und Laufweiten stehen in der erzeugten
[Token-Referenz](tokens.md); diese Seite nennt nur Namen und Absicht.

Nachrichtentext auf Touch-Geräten: `density.message-font-size` (Kontext `touch`) statt der Größe
aus `type.body`, damit das iPhone nicht kleiner als die Systemvorgabe liest.

## Regeln

- **Keine Großschreibung per Transformation.** Lange deutsche Wörter ("Benachrichtigungen")
  werden in Versalien unlesbar. Bereichs-Titel unterscheiden sich durch Gewicht und Laufweite.
- **Zahlen in Mono, wo sie sich ändern.** Laufzeit, Kosten, Token, Kontext in der Live-Box nutzen
  `type.code-sm`, damit die Zeile beim Hochzählen nicht springt.
- **Dynamische Schriftgröße bis 200 Prozent** ohne Verlust von Inhalt (WCAG 2.2 SC 1.4.4,
  https://www.w3.org/TR/WCAG22/, abgerufen 2026-10-10 [belegt]). Zeilen wachsen in der Höhe;
  Spalten brechen auf schmalen Fenstern in den Navigationsstapel um (siehe 01-produkt.md).
- **Markdown.** Überschriften in Nachrichten nutzen höchstens `type.heading`; ein Agent kann den
  Verlauf nicht mit Display-Größen füllen.
- **Rückfall.** Fehlt ein Zeichen in Atkinson (z. B. CJK), greift die Rückfall-Liste aus
  `font.family.*`; die Apps bündeln keine weiteren Schriften.

## Einbindung

- Flutter: `packages/design_system` bundles the variable TTF files of both families (upright
  and italic, axis `wght`) from the source repositories at a recorded commit, with the OFL text of
  each family next to them, under `packages/design_system/fonts/`; `fonts/MANIFEST.json` records
  source URL, commit, SHA-256 and license of every file, and `test/fonts_test.dart` fails on a
  changed, missing or extra file or a file without license text (docs/DECISIONS.md D-M4-04-4).
  The files carry the Google Fonts download names (`<Family>-VariableFont_wght.ttf`), not the
  bracket names of the repositories. `pubspec.yaml` declares the families
  `Atkinson Hyperlegible Next` and `Atkinson Hyperlegible Mono`. `DsType` (generated, copied into
  `lib/src/generated/` by `tokens.py build`) supplies the `TextStyle` constants; `dsTextStyle`
  adds the package font name `packages/design_system/<family>` in front of the fallback list
  (apps that depend on the package register the fonts under that name) and drives the `wght`
  axis from the weight. The theme (`buildDsTheme`) takes its text theme from the roles through
  `dsTextStyle`; the fallback list of `font.family.*` stays behind it.
- Web-Konsole und Doku-Site: WOFF2 aus demselben Stand; `tokens.css` liefert die Variablen
  `--ds-type-*`.
- Vorschau: die WOFF2-Dateien liegen unter `tools/design-tokens/vendor/fonts/` und werden
  eingebettet.
