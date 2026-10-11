# Offene Fragen an den Owner

Stand: Ende M0 (Planung). Hier stehen nur Fragen, die allein der Owner beantworten kann und die für
das Produkt gelten: Recht und Anbieterbedingungen, Kosten, externe Konten, öffentliche Zusagen des
Projekts und echte Produktentscheidungen. Technische Fragen entscheidet die Planung selbst und hält
sie in den Abschnitten "Entscheidungen dieses Kapitels" der [Kapitel](spec/00-glossar.md) und in den
[ADRs](adr/README.md) fest.

Jede Frage hat eine Voreinstellung. Der [Plan](PLAN.md) baut mit ihr weiter, bis der Owner anders
entscheidet; keine Etappe wartet auf eine Antwort, solange die Frist nicht erreicht ist. Die Frist
nennt das Ticket oder die Etappe, vor deren Beginn die Antwort feststehen muss, weil danach eine
Änderung teurer wird oder nach außen wirkt.

Fragen, die nur eine einzelne Instanz betreffen (eigene Konten, eigene Rechner, eigener Betrieb),
stehen im privaten Betriebs-Repository dieser Instanz, nicht hier.

**Regeln.**

1. Eine Antwort des Owners wird in derselben Änderung umgesetzt, die die Frage hier schließt:
   ADR oder Eintrag im Ruling-Log, betroffene Kapitel, Plan-Tickets.
2. Eine geschlossene Frage wandert mit Datum und Antwort in den Abschnitt
   [Entschieden](#entschieden). IDs werden nie neu vergeben.
3. Eine neue Frage kommt nur hierher, wenn sie die Prüffrage besteht: Kann die Planung sie mit
   einem Grund selbst entscheiden? Dann ist sie keine Owner-Frage.

## Übersicht

Sortiert nach Frist.

| ID | Frage | Voreinstellung | Spätestens bis |
|---|---|---|---|
| OQ-01 | Zweiter Owner, solange persönliche Agenten laufen | strenge I1 ohne Owner-Ausnahme | vor M1-29 |
| OQ-02 | Sichtbarkeit des Abo-Inhabers auf der Agenten-Karte | sichtbar für alle mit `agent.read` | vor M1-51 |
| OQ-03 | Eigenes langlebiges Abo-Token des Inhabers im Konto des Sitzungsbenutzers für das Terminal-Profil `claude` | zulässig als eigene Nutzung; das Produkt fasst das Token nicht an; M2-45 prüft die Doku vor dem Bau | vor M2-45 |
| OQ-04 | Inhaltsaufzeichnung von Terminal-Sitzungen | nein, nur Metadaten | vor M2-46 |
| OQ-05 | Automatisches Folgen und leise Ruhezeiten | ja, wie in Kapitel 13 | vor M3-19 |
| OQ-06 | Keine Präsenz und keine Lesebestätigungen für Menschen | so lassen | vor M4 |
| OQ-07 | Etappe für Mehrknoten-Betrieb | M7 (M7-17); Terminal-Relais schon in M2 | vor M7-17 |
| OQ-08 | Herausgeber-Konten für Signatur und Verteilung der Apps | Test-Herausgeber bis zur Entscheidung | vor M7-12 und M7-13 |
| OQ-09 | Live-Tests in fremden Clouds | nur `tofu validate` und lokale VM | vor M7-16 |
| OQ-10 | Reihenfolge von Migration und Veröffentlichung | Migration zuerst | vor M8-09 |
| OQ-11 | Externer Penetrationstest | vor der Veröffentlichung beauftragen | vor V1-02 |
| OQ-12 | Ort des Repositorys und Go-Modulpfad | Platzhalter `OWNER`; Empfehlung Organisation | vor V1-03 |
| OQ-13 | Externe Prüfdienste vor der Veröffentlichung | aus bis zur Veröffentlichung | vor V1-05 |
| OQ-14 | Öffentliche Zusagen zu Deprecation und Versions-Überlappung | Standardwerte aus Kapitel 06 | vor V1-05 |
| OQ-15 | Zusagen in SECURITY.md und GOVERNANCE.md | Standardwerte aus Kapitel 23 | vor V1-05 |
| OQ-16 | Endgültiger Name und Markenprüfung | Bothy bleibt Arbeitsname | vor V1-06 |
| OQ-17 | Paketnamen und Konten für SDKs bei PyPI und npm | erst nach der Namenswahl reservieren | vor der ersten SDK-Veröffentlichung, spätestens V1-06 |
| OQ-18 | Persönliches Abo in Dev-Umgebungen | nein; V2-06 wird nicht gebaut | vor V2-06 |
| OQ-19 | Direktnachrichten für Owner unsichtbar | ja, Zugriff nur über den Rechtsweg | vor V3-01 |
| OQ-20 | Grafana-Cloud-Konto für die Signatur des Plugins | unsigniert und lokal bis zur Veröffentlichung | vor V3-17 |
| OQ-21 | Problem-Typen als absolute URIs | relative URIs in API v1 | vor API v2 |
| OQ-22 | API-Doku aus dem Code oder aus dem Vertrag | spec-first nach ADR 0013 | vor M1-17 |

## Fragen

### OQ-01: Zweiter Owner bei persönlichen Agenten

- **Frage:** Soll ein zweiter Owner möglich sein, ohne vorher alle Agenten mit
  `billing_mode = personal` auf `organization` umzustellen? Das ginge nur mit einer Owner-Ausnahme
  in I1: andere Owner zählten nicht zur Auslöser-Menge und würden von I2 abgewiesen.
- **Warum nur der Owner:** I1 ist eine Vorgabe des Owners und hängt an den Anbieterbedingungen
  (persönliches Abo nur für eigene Nutzung). Eine Ausnahme ändert "Owner darf alles".
- **Voreinstellung:** Nein. I1 bleibt streng: `personal` nur, solange der `credential_holder` der
  einzige Owner ist. `owner.add` wird sonst abgelehnt; der geführte Ablauf `resolve_i1` stellt
  vorher um.
- **Was der Plan damit tut:** M1-29 baut I1 ohne Ausnahme, M2-21 den geführten Ablauf.
  [Kapitel 10](spec/10-backend-richtlinie.md), [ADR 0024](adr/0024-backend-richtlinie-personal-organization-mit-invariante.md).
  Das Terminal hat eine eigene, engere Regel mit Owner-Ausnahme (I6,
  [Kapitel 24](spec/24-terminal.md) Abschnitt 7.3); sie schränkt Owner ein und ändert I1 nicht.
- **Bis wann:** vor M1-29. Eine spätere Ausnahme wäre eine neue ADR mit Migration der Prüfungen.

### OQ-02: Sichtbarkeit des Abo-Inhabers

- **Frage:** Darf die Agenten-Karte allen, die den Agenten sehen, zeigen, wessen persönliches Abo er
  nutzt (`credential_holder_id`)?
- **Warum nur der Owner:** Abwägung zwischen Transparenz und Datenschutz gegenüber den Menschen der
  Organisation; keine technische Frage.
- **Voreinstellung:** Ja, sichtbar für alle mit `agent.read`. Grund: Das Abzeichen "Persönlich" ist
  ohnehin sichtbar, und die Angabe erklärt, warum nur eine Person auslösen darf.
- **Was der Plan damit tut:** Das Schema `Agent` in [openapi.yaml](api/openapi.yaml) enthält das
  Feld; M1-51 liefert es mit `getAgent` und `listAgents` aus. Bei "nein" sieht es nur, wer `agent.update` darf.
- **Bis wann:** vor M1-51.

### OQ-03: Langlebiges Abo-Token im Terminal

- **Frage:** Ist es zulässig, dass der Inhaber einer Abo-Anmeldung (`llm_credentials = personal`)
  sich im Konto des Sitzungsbenutzers mit einem eigenen langlebigen Token aus `claude setup-token`
  (Variable `CLAUDE_CODE_OAUTH_TOKEN`) für seine interaktiven Sitzungen des Profils `claude`
  anmeldet? Das Produkt speichert, liest und übergibt das Token dabei nicht (D-W-17).
- **Warum nur der Owner:** Auslegung der Anbieterbedingungen; das Risiko trägt der Owner. Die Doku
  beschreibt das Token für Umgebungen ohne Browser, sagt aber nicht, ob eine interaktive Sitzung
  derselben Person gemeint ist ([Kapitel 24](spec/24-terminal.md) Abschnitt 7.4, [vermutet]).
- **Voreinstellung:** Ja, als eigene Nutzung des Inhabers im unveränderten Coding-Agenten.
- **Was der Plan damit tut:** M2-45 prüft die aktuelle Anbieter-Doku vor dem Bau (Ruling T10). Ist
  der Weg dort ausgeschlossen, bleibt bei `personal` nur `/login` in einer Sitzung; geht auch das
  für einen Benutzer ohne Anmeldesitzung nicht, bietet der Rechner das Profil `claude` nur mit
  `organization` an.
- **Bis wann:** vor M2-45.

### OQ-04: Inhaltsaufzeichnung von Terminal-Sitzungen

- **Frage:** Sollen Terminal-Sitzungen inhaltlich aufgezeichnet werden können, etwa für
  Nachweispflichten?
- **Warum nur der Owner:** Rechts- und Nachweisfrage, Datenschutz und Mitbestimmung.
- **Voreinstellung:** Nein. Es gibt nur Metadaten (`terminal_sessions`, Audit) mit der
  Aufbewahrungsklasse `security`.
- **Was der Plan damit tut:** M2-46 baut Audit und Kennzahlen ohne Inhalt; 01 führt die Aufzeichnung
  als "nie". [Kapitel 24](spec/24-terminal.md) Abschnitt 10.4, [Kapitel 20](spec/20-datenschutz.md)
  (R-DP-07), [Kapitel 01](spec/01-produkt.md). Bei "ja" braucht es eine eigene ADR: Der Server darf
  den Inhalt nach [ADR 0068](adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md)
  nicht lesen, eine Aufzeichnung müsste also im Host-Dienst entstehen.
- **Bis wann:** vor M2-46.

### OQ-05: Automatisches Folgen und Ruhezeiten

- **Frage:** Soll es wie bei Discord sein: Wer in einem Thread schreibt, folgt ihm automatisch, und
  Ruhezeiten machen Hinweise leise statt stumm?
- **Warum nur der Owner:** Produktentscheidung über das Benachrichtigungsverhalten.
- **Voreinstellung:** Ja, wie geplant.
- **Was der Plan damit tut:** M3-19 baut automatisches Folgen und Bündeln nach
  [Kapitel 13](spec/13-push-benachrichtigungen.md) Abschnitt 3.2; die Person kann "Folgen" ändern.
- **Bis wann:** vor M3-19.

### OQ-06: Präsenz und Lesebestätigungen

- **Frage:** Menschen haben nie eine Präsenz ("online") und nie Lesebestätigungen für andere; die
  Tipp-Anzeige gibt es. Ist das so gewollt?
- **Warum nur der Owner:** Datenschutz und Mitbestimmung (Leistungskontrolle) betreffen die
  Organisation.
- **Voreinstellung:** Ja, so lassen.
- **Was der Plan damit tut:** Die Clients ab M4 bauen ohne Präsenz von Menschen und ohne
  Lesebestätigungen. [Kapitel 01](spec/01-produkt.md) (R-PR-02, R-PR-03).
- **Bis wann:** vor M4.

### OQ-07: Etappe für Mehrknoten-Betrieb

- **Frage:** In welcher Etappe soll der Betrieb mit mehreren api-Knoten produktionsreif sein?
- **Warum nur der Owner:** Umfang und Zeit: Cloud-Betrieb mit Ausfallsicherheit gegen Betrieb auf
  einem Rechner.
- **Voreinstellung:** In M7. Der Code bleibt ab M1 knotenneutral.
- **Was der Plan damit tut:** M7-17 baut Stufe 2 aus [Kapitel 02](spec/02-architektur.md)
  Abschnitt 16; das Terminal-Relais über mehrere Knoten kommt schon mit M2-47 (P-25,
  [Kapitel 24](spec/24-terminal.md) Abschnitt 9).
- **Bis wann:** vor M7-17.

### OQ-08: Herausgeber-Konten für die Apps

- **Frage:** Mit welchen Konten signiert und verteilt das Projekt seine Apps (Apple Developer
  Program, Signatur für Windows, Weg für Android)?
- **Warum nur der Owner:** externe Konten, laufende Kosten, Name des Herausgebers. Die Konten einer
  einzelnen Instanz sind Sache ihres Betreibers.
- **Voreinstellung:** Herausgeber-Modell aus
  [ADR 0037](adr/0037-app-verteilung-und-auto-update-kanaele.md) (Status `proposed`): jede
  Organisation signiert selbst. Bis zur Entscheidung bauen und prüfen die Tickets nur mit einem
  Test-Herausgeber.
- **Was der Plan damit tut:** M7-12 und M7-13 liefern Release-Kette, `distribution.yaml`-Vorlage
  und Runbook; echte Signaturen entstehen erst nach der Antwort. [Kapitel 18](spec/18-release.md).
- **Bis wann:** vor M7-12 und M7-13.

### OQ-09: Live-Tests in fremden Clouds

- **Frage:** Sollen die OpenTofu-Module gegen echte Konten bei Cloud-Anbietern getestet werden?
- **Warum nur der Owner:** Kosten und externe Konten.
- **Voreinstellung:** Nein. Nur `tofu validate` und der Portabilitätstest auf einer lokalen VM.
- **Was der Plan damit tut:** M7-16 liefert die Module mit diesen Prüfungen; ein Live-Test wäre ein
  eigenes Ticket mit Kostenrahmen. [Kapitel 16](spec/16-betrieb.md),
  [Cloud-Migration](operations/cloud-migration.md).
- **Bis wann:** vor M7-16.

### OQ-10: Reihenfolge von Migration und Veröffentlichung

- **Frage:** Läuft die erste Migration einer bestehenden Installation (M8-09) vor der
  Veröffentlichung des Repositorys (V1) oder danach? Und wann genau wird es öffentlich?
- **Warum nur der Owner:** Die Veröffentlichung wirkt nach außen und ist nicht umkehrbar.
- **Voreinstellung:** Migration zuerst (P-15). Eine Migration mit echten Daten vor der
  Veröffentlichung liefert Belege.
  Bis zur Veröffentlichung gelten die Release-Signaturen aus
  [ADR 0062](adr/0062-release-signaturen-solange-das-repo-privat-ist.md) ohne Kosten; Artifact
  Attestations kommen mit der Veröffentlichung, weil GitHub sie in den Plänen Free, Pro und Team
  nur für öffentliche Repositorys anbietet ([Kapitel 18](spec/18-release.md) Abschnitt 6).
- **Was der Plan damit tut:** V1 hängt an den Bau-Tickets von M8, nicht an der Durchführung M8-09;
  der Owner kann V1 vorziehen. [Plan](PLAN.md) Abschnitte 15 und 19,
  [Checkliste Öffentlich-Schalten](quality/public-switch.md).
- **Bis wann:** vor M8-09.

### OQ-11: Externer Penetrationstest

- **Frage:** Wird vor der Veröffentlichung ein externer Penetrationstest beauftragt?
- **Warum nur der Owner:** Kosten und Auswahl eines Dienstleisters.
- **Voreinstellung:** Ja, Stufe P2 aus [Kapitel 19](spec/19-sicherheit.md) "Penetrationstest-Plan",
  auf einer Test-Instanz mit synthetischen Daten.
- **Was der Plan damit tut:** V1-02 ist Voraussetzung für V1-04; ohne Test gibt es keine
  Veröffentlichung.
- **Bis wann:** vor V1-02.

### OQ-12: Ort des Repositorys und Go-Modulpfad

- **Frage:** Unter welchem Konto oder welcher Organisation erscheint das öffentliche Repository, und
  welcher Go-Modulpfad folgt daraus?
- **Warum nur der Owner:** externes Konto und Außenwirkung; nur der Owner kann eine Organisation
  anlegen und benennen.
- **Voreinstellung:** Es gilt die frühere Vorgabe des Owners (persönliches Konto). Alle Dateien
  nutzen bis dahin den Platzhalter `OWNER` (P-05). Empfehlung: vor dem ersten öffentlichen Commit
  eine neutrale Organisation anlegen, weil ein späterer Umzug den Go-Modulpfad aller Importe bricht
  und ein persönlicher Name sonst im Modulpfad steht.
- **Was der Plan damit tut:** V1-03 ersetzt `OWNER` und den vorläufigen Modulpfad; ein Test prüft,
  dass kein Platzhalter bleibt. [ADR 0044](adr/0044-arbeitsname-und-branding-als-konfiguration.md).
- **Bis wann:** vor V1-03.

### OQ-13: Externe Prüfdienste vor der Veröffentlichung

- **Frage:** Dürfen Diffs des noch privaten Repositorys an externe Dienste gehen: die CodeRabbit CLI
  und die gehostete Renovate-App?
- **Warum nur der Owner:** Abfluss von privatem Code an Dritte.
- **Voreinstellung:** Nein. Die CodeRabbit CLI ist aus, bis `BOTHY_CODERABBIT=1` gesetzt ist;
  Renovate wird beim Öffentlich-Schalten aktiviert.
- **Was der Plan damit tut:** [Push-Disziplin](quality/push-discipline.md),
  [ADR 0041](adr/0041-abhaengigkeits-updates-mit-renovate.md),
  [Checkliste Öffentlich-Schalten](quality/public-switch.md) Schritt 16. Bis dahin prüfen die
  lokalen Gates allein.
- **Bis wann:** vor V1-05. Die Voreinstellung kostet nichts; eine frühere Freigabe ist jederzeit
  möglich.

### OQ-14: Zusagen zu Deprecation und Versions-Überlappung

- **Frage:** Welche Fristen sagt das Projekt Integratoren zu: Mindestvorlauf vor dem Abschalten
  einer veralteten Operation und Mindestdauer, die eine alte Hauptversion neben der neuen läuft?
- **Warum nur der Owner:** öffentliche Zusage an Dritte mit Pflegeaufwand.
- **Voreinstellung:** die Standardwerte von `api.deprecation_min_notice` und
  `api.major_version_overlap` aus [Kapitel 06](spec/06-api.md).
- **Was der Plan damit tut:** [ADR 0014](adr/0014-api-versionierung-und-deprecation-policy.md) und
  die Bruch-Prüfung M7-11 arbeiten mit diesen Werten.
- **Bis wann:** vor V1-05.

### OQ-15: Zusagen in SECURITY.md und GOVERNANCE.md

- **Frage:** Sind die Fristen für Meldungen von Schwachstellen (Eingang, Bewertung, Korrektur,
  Offenlegung) und für Issues (Erstantwort, Schließen bei fehlender Rückmeldung) zusagbar?
- **Warum nur der Owner:** öffentliche Zusage des Projekts.
- **Voreinstellung:** die Standardwerte von `security.*` und `triage.*` aus
  [Kapitel 23](spec/23-qualitaet.md); ein Test hält die Dateien gleich.
- **Was der Plan damit tut:** Die Community-Dateien nennen diese Werte; V1-05 macht sie öffentlich.
- **Bis wann:** vor V1-05.

### OQ-16: Endgültiger Name und Markenprüfung

- **Frage:** Bleibt es bei Bothy oder wird es einer der anderen Kandidaten? Wird eine
  Markenrecherche durch eine Fachperson in den Klassen 9 und 42 beauftragt?
- **Warum nur der Owner:** Marken- und Rechtsfrage, Kosten der Recherche, Produktentscheidung.
- **Voreinstellung:** Bothy bleibt Arbeitsname; Recherche vor der Veröffentlichung beauftragen.
- **Was der Plan damit tut:** Der angezeigte Name ist Konfiguration (`branding.product_name`); im
  Code steht er nur an den Stellen, die
  [ADR 0044](adr/0044-arbeitsname-und-branding-als-konfiguration.md) listet, und ein Test hält die
  Liste. V1-06 setzt den endgültigen Namen.
- **Bis wann:** vor V1-06. Davon hängen OQ-17 und OQ-20 ab.

### OQ-17: Paketnamen und Konten für die SDKs

- **Frage:** Unter welchen Namen werden die SDKs bei PyPI und npm registriert, und wer hält die
  Konten (npm-Scope oder Organisation)?
- **Warum nur der Owner:** externe Konten; hängt am Namen (OQ-16).
- **Voreinstellung:** erst nach der endgültigen Namenswahl reservieren.
- **Was der Plan damit tut:** Die SDKs entstehen in M2-29 und M2-30 ohne Veröffentlichung; die
  Release-Kette veröffentlicht erst mit festem Namen.
  [ADR 0026](adr/0026-sdks-generierter-kern-plus-handschicht.md), [Kapitel 18](spec/18-release.md).
- **Bis wann:** vor der ersten Veröffentlichung eines SDK, spätestens V1-06.

### OQ-18: Persönliches Abo in Dev-Umgebungen

- **Frage:** Darf eine Person in ihrer eigenen, abgeschotteten Dev-Umgebung ihr persönliches Abo
  nutzen? Das verlangt I1 je Umgebung statt je Agent.
- **Warum nur der Owner:** ändert die Vorgabe I1 und berührt Anbieterbedingungen.
- **Voreinstellung:** Nein. Dev-Umgebungen haben `organization` (eigener Schlüssel mit
  Ausgabengrenze je Person) oder keine LLM-Anmeldung.
- **Was der Plan damit tut:** V2-06 wird ohne Entscheidung nicht gebaut.
  [Kapitel 21](spec/21-dev-isolation.md) (K-DV-2),
  [ADR 0046](adr/0046-mehrbenutzer-dev-umgebung-nur-mit-abschottung.md).
- **Bis wann:** vor V2-06.

### OQ-19: Direktnachrichten für Owner unsichtbar

- **Frage:** Bleiben Direktnachrichten zwischen Menschen auch für Owner unsichtbar? Das weicht von
  "Owner darf alles" ab.
- **Warum nur der Owner:** Vertraulichkeit zwischen Menschen gegen Kontrolle durch die
  Organisation; Rechtsfrage.
- **Voreinstellung:** Ja, unsichtbar. Zugriff nur über einen Auskunfts- oder Rechtsweg mit Step-up
  und Audit (Ruling L21).
- **Was der Plan damit tut:** V3-01 baut Direktnachrichten mit Schutzregel SR6.
  [Kapitel 07](spec/07-rechte.md), [Kapitel 05](spec/05-aktionen.md) (R-AZ-02).
- **Bis wann:** vor V3-01.

### OQ-20: Grafana-Cloud-Konto für die Plugin-Signatur

- **Frage:** Wann und unter welchem Organisationsnamen wird das Konto bei Grafana Cloud angelegt,
  das die Signatur des Panel-Plugins braucht? Der erste Teil der Plugin-ID muss zum Konto passen.
- **Warum nur der Owner:** externes Konto und Name beim Anbieter; hängt an OQ-16.
- **Voreinstellung:** Bis zur Veröffentlichung unsigniert und lokal provisioniert; freigegeben ist
  nur genau diese Plugin-ID (Ruling T11). Konto und Antrag als Community-Plugin nach der
  endgültigen Namenswahl.
- **Was der Plan damit tut:** M7-25 und M7-26 liefern das Plugin unsigniert; V3-17 signiert es.
  [Kapitel 25](spec/25-visualisierung.md) Abschnitt 8,
  [ADR 0069](adr/0069-visualisierung-auf-zwei-ebenen-ohne-iframes-in-apps.md).
- **Bis wann:** vor V3-17; früher nur, wenn das Plugin in Grafana Cloud laufen soll.

### OQ-21: Problem-Typen als absolute URIs

- **Frage:** Sollen die Fehler-Typen der API später absolute URIs unter einer öffentlichen
  Projekt-Domain bekommen?
- **Warum nur der Owner:** setzt eine öffentliche Domain des Produkts voraus und hängt an OQ-16.
- **Voreinstellung:** Relativer URI `/api/v1/problems/{code}` bleibt für API v1.
- **Was der Plan damit tut:** nichts bis v2; eine Umstellung wäre ein Bruch.
  [Kapitel 06](spec/06-api.md) Abschnitt 5,
  [ADR 0015](adr/0015-fehlerformat-rfc-9457-idempotenz-paginierung.md).
- **Bis wann:** vor der Planung von API v2.

### OQ-22: API-Doku aus dem Code oder aus dem Vertrag

- **Frage:** Bau-Vorgabe 3 verlangt "API documentation is generated from code (OpenAPI from the
  routes)". ADR 0013 macht `openapi.yaml` normativ und erzeugt den Code daraus. Gilt für den
  Server die Lesart "eine Quelle, spec-first" (D-W-43)?
- **Warum nur der Owner:** Die Bau-Vorgabe ist eine Owner-Vorgabe; nur er kann sie auslegen oder
  ändern.
- **Voreinstellung:** Ja, spec-first. Server-Stubs, Clients und die API-Referenz entstehen aus
  dem Vertrag; ein Test (`make codegen-check`) hält Code und Vertrag gleich. Für SDKs und
  Dart-Pakete gilt die Vorgabe wörtlich.
- **Was der Plan damit tut:** M1-17 und alle API-Tickets arbeiten spec-first. Antwortet der
  Owner "aus den Routen", braucht es eine neue ADR, die ADR 0013 ersetzt, und M1-17 ändert die
  Codegenerierung um.
- **Bis wann:** vor M1-17.

## Entschieden

Fragen aus den Team-Berichten und Fragen dieser Seite, die inzwischen entschieden sind. Sie stehen
hier, damit niemand sie erneut stellt. Eine geschlossene Frage dieser Seite behält ihre ID in der
Spalte "Frage"; die ID wird nicht neu vergeben.

| Frage | Entscheidung | Wer, wann | Wirkung |
|---|---|---|---|
| Markenfarbe und Gestaltung (Petrol dunkel, gelber Akzent) | abgenommen | Owner, 2026-10-10, auf den Vorschauen in [design](design/README.md) | Palette und Tokens bleiben; Akzent-Varianten je Instanz kommen mit M4-03. |
| Gilt die Abnahme für die sieben danach neu erzeugten Vorschaubilder (D-W-46)? | ja, ohne Nachabnahme: Die Abnahme gilt für die Bilder in `docs/design/previews`, und die sieben sind inhaltliche Korrekturen innerhalb des abgenommenen Designs (Lauf-Zustände als Wörter, `amber.100` im Warn-Banner, keine abgeschnittene Nachricht). | Owner-Entscheidung "Design auf `docs/design/previews` abgenommen", der Planung in dieser Fassung am 2026-10-11 übermittelt, nachdem die sieben Bilder am selben Tag neu erzeugt waren (D-W-46, D-W-77) | Die Tickets bauen mit diesen Ständen. |
| OQ-23: Gelten die Vorschaubilder Terminal und Statistik (`terminal-*`, `viz-*`) als abgenommen? | ja, ohne offene Owner-Frage: Sie liegen in `docs/design/previews` und wenden das abgenommene Design-System an (gleiche Tokens, gleiche Regeln). Sie entstanden nach der Abnahme vom 2026-10-10: beide Paare am selben Tag (D-W-13), `viz-*` am 2026-10-11 neu erzeugt, zuletzt mit D-W-55. | Owner-Entscheidung wie in der Zeile davor; Ruling T14 der Leitung, 2026-10-11 (D-W-77) | Die Tickets bauen mit diesen Ständen; die frühere Frist vor M4-03 entfällt. |
| Commit-Identität im öffentlichen Repository | Die Commits tragen die vom Owner gewählte Identität; das Produkt-Repository nennt sie nicht. | Owner, 2026-10-10 | Kein Umschreiben der Autoren nötig. Veröffentlicht wird ein Commit ohne Eltern; die Planungshistorie bleibt privat (D-W-93, Schritt 2 der [Checkliste Veröffentlichen](quality/public-switch.md)). |
| Android, Sprachnachrichten, Teilen aus anderen Apps, Anmeldung über OIDC | Teil des Kerns | Owner, Planungsgrundlage | [Kapitel 01](spec/01-produkt.md), [Kapitel 08](spec/08-identitaet.md); Tickets in M3 bis M5. |
| Revisionen bearbeiteter Nachrichten und Standardwerte des Kern-Vertrags (Aufbewahrung des Ereignis-Logs, Sitzungsdauer, Größen) | keine Owner-Frage | Planung | Revisionen folgen der Klasse `content` in [Kapitel 20](spec/20-datenschutz.md); die Werte sind benannte Konfigurationswerte, die jede Instanz ändern kann ([Konfigurationsreferenz](operations/configuration.md)). |
| Ansicht mit Daten über einzelne Abo-Sitze oder Zugangsdaten | nicht im Produkt | Planung, Ruling T8 | Das Produkt zeigt nur die Backend-Ansicht ([Kapitel 25](spec/25-visualisierung.md) Abschnitt 6.2) und kennt keine Zugangsdaten (I5). |
