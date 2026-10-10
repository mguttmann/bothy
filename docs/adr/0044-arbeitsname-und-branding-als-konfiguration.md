---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team Apps und Design
consulted: Team Betrieb, Team Qualität und Repo
informed: alle Teams
---

# Arbeitsname und Branding als Konfiguration

## Context and Problem Statement

Das Produkt heißt vorläufig "Bothy" (Namensrecherche: [naming.md](../design/naming.md)); die
endgültige Wahl und die Markenprüfung stehen aus. Zugleich soll jede Instanz ihre eigenen Apps mit
eigenem Namen, Logo, Akzent, Bundle-IDs, Push-Themen und Update-Kanälen ausliefern können, und das
Produkt-Repo muss frei von Firmennamen bleiben (White Label). Wo darf der Name stehen, und wie
kommt das Branding einer Instanz in Server, Apps und Builds?

## Decision Drivers

- Namenswechsel nach Markenprüfung billig halten
- White Label: keine Firmendaten im Produkt-Repo
- Instanzen liefern eigene, signierte Apps (eigene IDs, Push, Updates)
- Kontrast AA darf durch Branding nicht brechen
- wenige, gelistete Stellen statt verstreuter Konstanten

## Considered Options

1. Zwei Ebenen: Laufzeit-Branding in den Instanz-Einstellungen, Build-Branding in einer Datei der
   Instanz; der Name im Code nur an gelisteten Stellen
2. Nur Laufzeit-Branding; eine App für alle Instanzen
3. Name und Branding fest im Code; Fork je Instanz

## Decision Outcome

Chosen option: "Zwei Ebenen", weil Name, Logo und Akzent zur Laufzeit wechseln können, ohne neue
Apps zu bauen, während Bundle-IDs, Push-Themen, Signier-Identitäten und Update-Feeds technisch zur
Bauzeit feststehen müssen und je Instanz eigen sind.

| Ebene | Inhalt | Ort |
|---|---|---|
| Laufzeit | `branding.product_name`, `branding.logo_file_id`, `branding.accent_color` | `instance_settings`, Aktion `instance.branding.update` |
| Build | App-Name, Bundle- und Paket-IDs, App-Icons aus einer SVG-Quelle, URL-Schema, Push (APNs-Topic, Firebase-Projekt), Sparkle-Feed und EdDSA-Schlüssel, MSIX-Identität und `.appinstaller`-URL, Absturzdienst, Standard-Server | `branding.yaml` der Instanz im Betreiber-Repo; das Produkt-Repo enthält nur ein Beispiel mit `example.com` |
| Code | Go-Modulpfad, Paketnamen, Binary `bothy`, Präfix der API-Tokens `bothy_` (03 `api_tokens`) | gelistet in `docs/branding-code-locations.md` (Ticket), geprüft durch einen Test |

Der Akzent ist ein Schlüssel aus der geprüften Kategorie-Palette (`spruce`, `loch`, `heather`,
`rowan`, `amber`, `gorse`, `moss`), kein freier Farbwert, damit jede Kombination im Kontrast-Gate
liegt (ADR 0030).

### Consequences

- Good, because ein Namenswechsel `branding.*`, die gelisteten Code-Stellen (darunter das
  Token-Präfix `bothy_`) und die Beispiel-Konfiguration betrifft, sonst nichts.
- Good, because Instanzen eigene, signierte Apps bauen, ohne das Produkt zu verzweigen.
- Good, because Kontrast AA auch mit Instanz-Akzent gilt.
- Bad, because jede Instanz mit eigenen Apps eigene Apple-, Google- und Microsoft-Konten,
  Schlüssel und Signaturen braucht (Owner-Konten, Baseline R-Push und R-Desktop-Verteilung).
- Bad, because freie Markenfarben nicht möglich sind.

### Confirmation

- Ein Test sucht den Arbeitsnamen im Repo und erlaubt ihn nur an den gelisteten Stellen, in Doku
  und im Beispiel des Builds.
- Der Server prüft `branding.accent_color` gegen die Schlüssel aus
  `design/build/tokens.resolved.json` (Ticket "Erlaubte Farb- und Icon-Schlüssel im Server").
- Der App-Build bricht ab, wenn ein Pflichtfeld in `branding.yaml` fehlt.

## Pros and Cons of the Options

### Zwei Ebenen

- Good, because trennt, was zur Laufzeit wechseln kann, von dem, was signiert sein muss.
- Bad, because zwei Orte für Branding.

### Nur Laufzeit

- Good, because eine App für alle.
- Bad, because Push-Themen, Bundle-IDs und Update-Feeds wären fremde Konten; Instanzen könnten
  keine eigenen Apps verteilen.

### Fest im Code

- Good, because am einfachsten am ersten Tag.
- Bad, because Namenswechsel teuer; Forks je Instanz verletzen White Label und vervielfachen Pflege.

## More Information

- Bedienung und Branding-Regeln: [01-produkt.md](../spec/01-produkt.md), Abschnitt 12.
- Build-Konfiguration in den Apps: [15-client-architektur.md](../spec/15-client-architektur.md),
  Abschnitte 11 und 15.
- Endgültiger Name: offene Owner-Frage; Empfehlung Bothy, Alternativen Corvo und Gilde.
