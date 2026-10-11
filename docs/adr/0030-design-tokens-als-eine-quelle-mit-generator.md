---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team Apps und Design
consulted: Team Qualität und Repo
informed: alle Teams
---

# Design-Tokens als eine Quelle mit Generator

## Context and Problem Statement

Farben, Schrift, Abstände, Bewegung und Dichte erscheinen in den Flutter-Apps, in der Web-Konsole,
auf der Doku-Site, in der Statusseite und in Vorschauen. Die Anforderung verlangt ein farbiges,
unverwechselbares Design mit hellem und dunklem Theme, Kontrast AA und ohne generischen KI-Look.
Wie bleibt das an allen Orten gleich, und wie wird "AA" und "kein generischer Look" überprüfbar
statt eine Absicht?

## Decision Drivers

- eine Quelle für alle Ausgaben (Dart, CSS, Doku, Vorschau)
- offener Standard statt eigenes Format
- Kontrast als Test, nicht als Review
- Verbotsliste messbar machen, wo möglich
- keine Lieferkette im Gate, läuft offline
- erzeugte Dateien prüfbar aktuell

## Considered Options

1. W3C-DTCG-Tokens (Format 2025.10, Resolver 2025.10) plus eigener Generator in Python ohne
   Abhängigkeiten
2. W3C-DTCG-Tokens plus Style Dictionary (Node) mit eigenen Formaten
3. Figma als Quelle mit einem Token-Plugin, Export ins Repo
4. Handgeschriebenes Dart-Theme, CSS getrennt gepflegt

## Decision Outcome

Chosen option: "DTCG plus eigener Generator", weil das Format ein stabiler Bericht der W3C Design
Tokens Community Group ist (Format, Color und Resolver Module 2025.10, "This specification is
considered stable", https://www.designtokens.org/tr/2025.10/format/, abgerufen 2026-10-10
[belegt]), der Resolver Theme, Dichte und Bewegung ohne Kopien abbildet
(https://www.designtokens.org/tr/2025.10/resolver/, abgerufen 2026-10-10 [belegt]) und die
eigentliche Arbeit ohnehin eigen ist: `ThemeExtension`-Klassen, Kontrast-Gate mit erklärten
Paarungen, Palettenwächter, Vorschau mit eingebetteten Schriften. Ein Generator in der
Python-Standardbibliothek läuft mit `uv` ohne Paket-Download und hat Unit-Tests.

### Consequences

- Good, because jede Ausgabe aus einer Quelle entsteht und `tokens.py check` Abweichungen findet.
- Good, because Kontrast AA für jede erklärte Paarung in hell und dunkel ein harter Test ist.
- Good, because Teile der Verbotsliste (Blurple, Grau ohne Stich, Beige, Lila- und
  Terrakotta-Akzent, Verläufe) Tests sind.
- Good, because Farben in OKLCH notiert sind (gleichmäßige Helligkeitsstufen) und außerhalb von sRGB
  abgelehnt werden.
- Bad, because der Generator nur die genutzte Teilmenge der Spezifikation kennt (`$extends` und
  JSON-Pointer-Referenzen in Token-Dateien lehnt er ab).
- Bad, because Designer ohne JSON-Kenntnis einen Weg zu den Tokens brauchen (DTCG-fähige Werkzeuge
  können die Dateien lesen; ein Import ist nicht Teil dieser Entscheidung).

### Confirmation

- `uv run tools/design-tokens/tokens.py check` im lokalen Gate: Struktur, Kontrast, Wächter,
  Aktualität aller erzeugten Dateien.
- Unit-Tests des Generators (`tools/design-tokens/tests`).
- Schema-Prüfung gegen die offiziellen JSON-Schemas der DTCG (Befehl in
  `tools/design-tokens/README.md`).
- Golden-Tests in `packages/design_system` nutzen nur die erzeugten Klassen.

## Pros and Cons of the Options

### DTCG plus eigener Generator

- Good, because Standardformat, keine Abhängigkeiten, Gates genau passend.
- Bad, because eigener Code zu pflegen (klein, getestet).

### DTCG plus Style Dictionary

- Good, because verbreitetes Werkzeug mit vielen Formaten.
- Bad, because Node-Abhängigkeitsbaum im Gate; die nötigen Formate (`ThemeExtension`, Kontrast,
  Wächter, Vorschau) wären trotzdem eigener Code.

### Figma als Quelle

- Good, because Gestaltung im gewohnten Werkzeug.
- Bad, because Konto bei einem Anbieter als Quelle der Wahrheit; nicht reproduzierbar im Repo;
  Prüfungen erst nach Export.

### Handgeschriebenes Dart-Theme

- Good, because kein Werkzeug.
- Bad, because CSS, Doku und Vorschau laufen auseinander; Kontrast nur per Review.

## More Information

- Quelle: `design/tokens/`, Regeln: `design/design-rules.json`, Werkzeug: `tools/design-tokens/`.
- Gestaltung und Verbotsliste: [docs/design/README.md](../design/README.md).
- Kontrastformel nach WCAG 2.2 mit der sRGB-Schwelle 0.04045
  (https://www.w3.org/WAI/GL/wiki/Relative_luminance, abgerufen 2026-10-10 [belegt]).
