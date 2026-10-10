---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team G (Plattformdienste), Team C (Sicherheit)
informed: Team E (Apps)
---

# Bild-Metadaten verlustfrei entfernen

## Context and Problem Statement

Fotos tragen Metadaten: EXIF mit Ort, Gerät und Zeit, XMP, IPTC, Text-Chunks in PNG. Diese Daten
verlassen sonst mit jedem geteilten Bild den Absender, ohne dass er es bemerkt
([19-sicherheit.md](../spec/19-sicherheit.md), SEC-UP-10, R-SE-06). Die Planungsgrundlage sah
ursprünglich das Entfernen durch Neukodieren vor (Ruling R-Objektspeicher). Neukodieren kostet aber
Qualität bei jedem Original, und reines Go verwirft dabei Farbprofil und Orientierung. Wie entfernt
der Server Metadaten so, dass das Original in voller Qualität erhalten bleibt (Ruling L16)?

## Decision Drivers

- Ortsdaten verlassen nie den Absender; kein Schalter zum Abschalten (R-SE-06).
- Das Original bleibt als Download in voller Qualität (Ruling L16).
- Farbe und Lage stimmen in Original und Vorschau.
- Im Zweifel entfernen: Was der Server nicht kennt, fällt weg (Erlaubnisliste statt Sperrliste,
  R-SE-03).
- Reines Go im Standard-Build, kein cgo (ADR 0005, Baseline R-Objektspeicher).
- Kein Dekodieren übergroßer Bilder (Dekompressions-Bomben, SEC-T08).

## Considered Options

- Verlustfreier Segment- und Chunk-Filter mit Erlaubnisliste für JPEG und PNG, Vorschaubilder neu
  kodiert, übrige Formate nur als Download
- Neukodieren jedes Inline-Bildes einschließlich des Originals
- Sperrliste: nur bekannte Metadaten-Felder (EXIF-GPS, XMP) entfernen, den Rest behalten
- Entfernen im Client vor dem Upload
- Externes Werkzeug (libvips oder exiftool) als eigener Prozess für alle Formate

## Decision Outcome

Gewählte Option: "Verlustfreier Segment- und Chunk-Filter mit Erlaubnisliste", weil sie als
einzige Ortsdaten sicher entfernt und das Original Byte für Byte in den Bilddaten erhält. Die
Regel steht in [12-dateien.md](../spec/12-dateien.md), Abschnitt 7 (R-FI-07).

1. **JPEG.** Der Server schreibt beim Abschluss des Uploads ein neues Original (`orig-2`) aus der
   Erlaubnisliste: alle Markierungen, die das Dekodieren nach ITU-T T.81 braucht (Kopf, Tabellen,
   Rahmen, Scans mit ihren Daten, Ende), `APP0` (JFIF), `APP14` (Adobe), `APP2` nur mit der Kennung
   `ICC_PROFILE` (alle Teile eines mehrteiligen Profils) und ein neu geschriebenes `APP1` mit genau
   dem EXIF-Tag "Orientation". Alles andere fällt weg: übrige `APPn` (EXIF samt eingebettetem
   Vorschaubild, XMP, IPTC, Multi-Picture-Format), `COM` und jedes Byte nach dem ersten
   Bildende. Grund für den letzten Punkt: Formate wie Ultra HDR hängen weitere Bilder hinter das
   erste [Q3, belegt]; diese tragen eigene Metadaten und sind für das erste Bild nicht nötig.
2. **PNG.** Es bleiben die kritischen Chunks und die Chunks für Farbe, Transparenz, Maße und
   Animation aus der Liste in 12, Abschnitt 7.1. Text-Chunks, `eXIf`, `tIME` und jeder unbekannte
   Chunk fallen weg, auch wenn er als kopierbar markiert ist.
3. **Scheitert der Filter** an einer Datei, die als JPEG oder PNG erkannt wurde (beschädigte
   Struktur), wird sie nicht gespeichert (`upload_state = 'failed'`). Grund: Die Zusage gilt für
   jedes gespeicherte JPEG und PNG; eine unverändert abgelegte Datei könnte Ortsdaten tragen.
4. **Vorschaubilder** entstehen durch Dekodieren und Neukodieren in reinem Go, mit angewendeter
   Orientierung. Sie tragen keine Metadaten außer dem unveränderten Farbprofil des Originals. Grund:
   Die Standardbibliothek wandelt keine Farbräume; ohne Profil sähen Fotos mit großem Farbraum in
   Verlauf und Vollbild blass aus. Das Profil enthält keine Ortsdaten, und das Original trägt es
   ohnehin.
5. **Andere Formate** bleiben wie in 12, Abschnitt 7.1: GIF wird neu kodiert, unbewegtes WebP in
   PNG oder JPEG gewandelt, HEIC, AVIF, TIFF und bewegtes WebP sind nur Download-Karten, außer
   libvips ist eingeschaltet. SVG ist nie inline.

### Consequences

- Gut, weil Originale ohne Generationsverlust bleiben und `sha256` das gespeicherte Objekt
  beschreibt; die angemeldete Prüfsumme bleibt in der Upload-Session.
- Gut, weil die Erlaubnisliste neue Metadaten-Arten ohne Codeänderung entfernt.
- Gut, weil der Filter nur Struktur liest und keine Pixel dekodiert; Bomben treffen nur die
  Vorschau, und die hat `file.image.max_pixels` als Grenze.
- Schlecht, weil HDR-Zusatzbilder (Gain-Maps) wegfallen; das Original zeigt dann nur das
  Standardbild. Das ist der Preis der Erlaubnisliste.
- Schlecht, weil unbewegtes WebP ohne Alpha als JPEG gespeichert und damit verlustbehaftet
  gewandelt wird; der Filter gilt nach L16 nur für JPEG und PNG.
- Schlecht, weil Dateien in anderen Formaten (HEIC, PDF, Office, Archive) ihre Metadaten
  behalten. Die Grenze der Zusage steht in 12, Abschnitt 7.1; dort wandeln die Apps Fotos aus der
  Mediathek vor dem Upload in JPEG, damit HEIC nur als bewusst gesendete Datei ankommt.
- Neutral, weil ein beschädigtes JPEG oder PNG jetzt abgelehnt wird statt als Download zu landen.

### Confirmation

- Test "JPEG mit GPS-EXIF, XMP, IPTC, `COM`, `APP2` mit Multi-Picture-Format, angehängtem Bild und
  Orientierung 6": Das gespeicherte Original hat genau ein `APP1` mit nur dem Tag "Orientation" = 6,
  `APP2` nur mit `ICC_PROFILE` (Profil Byte für Byte gleich), kein anderes `APPn`, kein `COM`, kein
  Byte nach dem Bildende; die Scan-Daten sind Byte für Byte gleich der Eingabe.
- Test "PNG mit `tEXt`, `iTXt`, `zTXt`, `eXIf`, `tIME` und einem unbekannten Hilfs-Chunk": Das
  Ergebnis enthält nur Chunks der Erlaubnisliste; `IDAT` ist Byte für Byte gleich; die dekodierten
  Pixel sind gleich.
- Fuzz-Test (`go test -fuzz`) des Filters: Für beliebige eingefügte Segmente und Chunks enthält die
  Ausgabe nur Einträge der Erlaubnisliste, oder der Upload scheitert.
- Test "abgeschnittenes JPEG": `upload_state = 'failed'`, kein Objekt im Speicher.
- Test der Vorschau: keine EXIF-, XMP- oder Text-Daten in einer Variante, Orientierung in den Pixeln
  angewendet, Farbprofil gleich dem Original.
- SEC-T08 aus [19-sicherheit.md](../spec/19-sicherheit.md) und Architektur-Regel "kein cgo im
  Standard-Build"; libvips nur in einem eigenen Build mit `file.image.libvips_enabled`.
- Bau-Ticket M3-06 in [PLAN.md](../PLAN.md).

## Pros and Cons of the Options

### Verlustfreier Filter mit Erlaubnisliste

- Gut, weil `APPn`-Segmente und `COM` für die Anwendung reserviert sind und das Dekodieren nicht
  brauchen [Q1, belegt]; ihr Entfernen ändert die Bilddaten nicht [abgeleitet].
- Gut, weil PNG Text und Exif in eigenen Chunks führt [Q2, belegt] und sich damit sauber trennen
  lässt.
- Schlecht, weil zwei Parser (JPEG-Segmente, PNG-Chunks) gebaut und gegen Fuzzing gehärtet werden
  müssen.

### Neukodieren jedes Inline-Bildes

- Gut, weil ein Weg für alle Formate und nichts außer Pixeln übrig bleibt.
- Schlecht, weil jedes JPEG-Original Qualität verliert und Farbprofil und Orientierung von Hand
  nachgebaut werden müssen; reines Go dekodiert ohne beides [Q5, abgeleitet].
- Schlecht, weil L16 das Original in voller Qualität verlangt.

### Sperrliste bekannter Felder

- Gut, weil das Original fast unverändert bleibt.
- Schlecht, weil jede neue Metadaten-Art (Herstellerfelder, eingebettete Vorschaubilder mit
  eigenem EXIF, angehängte Bilder) unbemerkt durchgeht. Das ist die falsche Fehlerklasse (R-SE-03).

### Entfernen im Client

- Gut, weil die Daten das Gerät gar nicht erst verlassen.
- Schlecht, weil Agenten, Importe und fremde Clients ebenfalls hochladen; der Server müsste
  trotzdem prüfen. Eine zweite Umsetzung je Plattform brächte nur doppelten Code.

### Externes Werkzeug für alle Formate

- Gut, weil auch HEIC, AVIF und TIFF abgedeckt wären.
- Schlecht, weil libvips cgo und LGPL-2.1 bringt [Q4, belegt] und exiftool eine weitere Laufzeit;
  beides bleibt optional (`file.image.libvips_enabled`), nicht Standard.

## More Information

- Quellen (abgerufen 2026-10-10): Q1 ITU-T T.81, Anhang B,
  https://www.w3.org/Graphics/JPEG/itu-t81.pdf [belegt]; Q2 PNG Third Edition,
  https://www.w3.org/TR/png-3/ [belegt]; Q3 Ultra HDR Image Format v1.1,
  https://developer.android.com/media/platform/hdr-image-format [belegt]; Q4 govips,
  https://github.com/davidbyttow/govips [belegt]; Q5 Go `image/jpeg`,
  https://pkg.go.dev/image/jpeg [belegt]; Kennung `ICC_PROFILE` in `APP2`:
  https://www.color.org/profile_embedding.html und libjpeg-turbo `jcicc.c`,
  https://skia.googlesource.com/external/github.com/libjpeg-turbo/libjpeg-turbo/+/HEAD/jcicc.c
  [belegt].
- Erweiterung auf WebP: Dasselbe Muster ginge mit einem Filter der RIFF-Chunks `EXIF` und `XMP `.
  Das ist eine eigene Entscheidung, weil L16 den Filter auf JPEG und PNG begrenzt; bis dahin gilt
  die Umwandlung aus 12.
- Verwandt: ADR 0010 (Objektspeicher), ADR 0033 (Malware-Scan), ADR 0048 (Abruf).
