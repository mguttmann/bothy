---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team F (Betrieb), Team E (Apps)
informed: Team C (Sicherheit)
---

# Feature-Flag-Registrierung mit Aufräum-Gate

## Context and Problem Statement

Neue Funktionen sollen schrittweise freigeschaltet werden (für einzelne Personen oder Gruppen), und
der Owner braucht dauerhafte Produktschalter wie `dm.enabled` oder `voice.enabled`. Feature-Flags
leisten beides, hinterlassen aber erfahrungsgemäß tote Schalter und verlocken dazu, Grenzwerte oder
Sicherheitsregeln abschaltbar zu machen. Wo stehen Flags, wer wertet sie aus, und was erzwingt ihr
Entfernen?

## Decision Drivers

- Keine toten Flags: Ein Freischalt-Flag hat ein Ende, und das Gate erzwingt es.
- Flags schalten Funktionen, nie Sicherheitsgrenzen, Grenzwerte oder Geheimnisse
  ([16-betrieb.md](../spec/16-betrieb.md), Abschnitt 6.2).
- Auswertung nur im Server; Clients bekommen ausgewertete Werte (keine Rechte-Logik im Client,
  [15-client-architektur.md](../spec/15-client-architektur.md), Abschnitt 18).
- Ändern ist eine Verwaltungsaktion mit Step-up und Audit.
- Kein weiterer Dienst im Betrieb.

## Considered Options

- Registrierung jedes Flags im Code (Schlüssel, Art `release` oder `option`, Beschreibung, Standard,
  `remove_by` bei `release`), Werte in der Tabelle `feature_flags`, Gate-Check gegen `VERSION`
- Externer Flag-Dienst über eine Standard-Schnittstelle (OpenFeature mit Anbieter wie flagd oder
  Unleash)
- Flags nur in der Datenbank, ohne Registrierung im Code
- Flags als Konfigurationswerte (Umgebung oder Datei), ohne Rollout je Person
- Keine Flags; Funktionen erst mit dem fertigen Release

## Decision Outcome

Gewählte Option: "Registrierung im Code, Werte in `feature_flags`, Gate-Check", weil die
Registrierung jedes Flag mit Art und Ende beschreibt und das Gate das Ende durchsetzt. Normativ in
[16-betrieb.md](../spec/16-betrieb.md), Abschnitt 6.2 (R-OP-14); Anleitung in
[environments-and-flags.md](../operations/environments-and-flags.md), Abschnitt 2.

1. **Registrierung.** Jedes Flag steht an einer Stelle im Server mit Schlüssel, Art, Beschreibung
   (Englisch, für die Konsole), Standard, zuständigem Modul und bei `release` der Version
   `remove_by`. `option`-Flags haben kein `remove_by`.
2. **Werte.** Tabelle `feature_flags` ([03-datenmodell.md](../spec/03-datenmodell.md), Abschnitt
   4.1): globaler Wert und `rollout` mit Personen und Gruppen, für die der Wert umgekehrt gilt.
   Fehlt eine Zeile, gilt der registrierte Standard. Schlüssel ohne Registrierung sind aus und
   erscheinen in der Konsole als verwaist.
3. **Auswertung** nur im Server (`flags.Enabled(principal, key)`), mit gültigen
   Gruppenmitgliedschaften. Clients bekommen die für sie ausgewerteten Werte in `instance.read` und
   der Momentaufnahme; `instance.updated` nennt nur geänderte Schlüssel.
4. **Ändern** mit `feature_flag.update`: Owner, Step-up, Grund, Audit
   ([05-aktionen.md](../spec/05-aktionen.md), Abschnitt 4.2).
5. **Aufräumen.** Ein Check im Gate scheitert, sobald die Version in `VERSION` des Servers
   `remove_by` eines noch registrierten `release`-Flags erreicht. Wer das Flag entfernt, entfernt
   Code und Registrierung und löscht die Zeile per Migration in derselben Änderung. Ein
   `release`-Flag wird nie zu `option`; ein dauerhafter Schalter bekommt einen neuen Schlüssel.

`remove_by` ist eine Version, kein Datum: Ein Datum ließe Builds alter Stände an einem beliebigen
Tag rot werden; die Version ändert sich nur mit einem Release und ist im Repository prüfbar.

### Consequences

- Gut, weil jedes Freischalt-Flag ein Ende in Form einer Version hat, das das Gate durchsetzt.
- Gut, weil die Konsole jedes Flag mit Art und Beschreibung zeigen kann und verwaiste Zeilen
  auffallen.
- Gut, weil kein externer Dienst Nutzerdaten für das Ausrollen bekommt.
- Schlecht, weil das Ausrollen nur Personen und Gruppen kennt, keine Prozentsätze; für ein
  Team-Chat mit benannten Menschen reicht das.
- Schlecht, weil ein vergessenes Flag ein Release blockiert; das ist gewollt, kostet aber im
  falschen Moment Zeit. Gegenmittel: `remove_by` großzügig setzen und beim Release-Vorbereiten
  anzeigen (ADR 0060).
- Neutral, weil Agenten und Apps Flags wie Menschen erleben; `rollout` nennt nur Menschen und
  Gruppen.

### Confirmation

- Bau-Ticket M1-47 in [PLAN.md](../PLAN.md): Auswertung global, je Person und je Gruppe mit
  abgelaufener Mitgliedschaft; unbekannte Schlüssel sind aus; Änderungen nur durch Owner mit Step-up
  und Audit.
- `make flag-check` (Werkzeug `services/server/tools/flagcheck`, Teil von `gates`): scheitert,
  wenn `VERSION` `remove_by` eines registrierten `release`-Flags erreicht (eine Vorabversion wie
  `1.4.0-beta.1` hat `1.4.0` nicht erreicht); scheitert auch bei `release` ohne `remove_by`, bei
  `option` mit `remove_by`, bei jedem anderen Verstoß gegen die Regeln der Registrierung und wenn
  die Tabelle in [configuration.md](../operations/configuration.md) 7 von der Registrierung in
  Schlüssel, Art oder Standard abweicht. Belegt durch die Tests `TestFlagCheck*`
  (`tools/flagcheck`), `TestCheckRegistry`, `TestReachedRemoveBy`, `TestEvaluate`, `TestInRollout`
  (`internal/instance/domain`), `TestRegisteredFlagsValid` (`internal/instance`) und die
  Integrationstests `TestFlags_*`, `TestPutFeatureFlag_*`, `TestPersonalToken_*`
  (`internal/identity/adapters/httpapi`).
- Durch die Form gehalten: Ein Flag ist nur ein Schalter (`feature_flags.enabled boolean`); Zahlen,
  Listen oder Geheimnisse kann ein Flag nicht tragen. Ob ein Schalter eine Sicherheitsgrenze
  berührt, prüft das Review jedes Tickets und das Red-Team-Gate
  ([red-team-gate.md](../quality/red-team-gate.md)); ein Mechanismus dafür existiert nicht.

## Pros and Cons of the Options

### Registrierung im Code mit Gate

- Gut, weil Code, Registrierung und Aufräumregel an einer Stelle zusammenkommen.
- Schlecht, weil nur einfache Zielgruppen möglich sind.

### Externer Flag-Dienst

- Gut, weil OpenFeature eine herstellerneutrale Schnittstelle mit Go-SDK und Anbietern ist
  (CNCF-Projekt, Stufe Incubating) [Q1, belegt] und Prozent-Rollouts mitbringt.
- Schlecht, weil ein weiterer Dienst betrieben und gesichert werden muss oder Nutzerkennungen an
  einen Anbieter gehen; die Aufräumregel fehlt trotzdem.

### Nur Datenbank

- Gut, weil Flags ohne Release entstehen.
- Schlecht, weil niemand weiß, welcher Code ein Flag liest und wann es weg kann; genau so entstehen
  tote Flags.

### Flags als Konfiguration

- Gut, weil kein neuer Mechanismus nötig ist (ADR 0011).
- Schlecht, weil Konfiguration einen Neustart braucht und keine Freigabe je Person kennt.

### Keine Flags

- Gut, weil nichts zu verwalten ist.
- Schlecht, weil schrittweises Freischalten und dauerhafte Produktschalter des Owners fehlen.

## More Information

- Quelle (abgerufen 2026-10-10): Q1 OpenFeature, https://openfeature.dev/docs/reference/intro
  [belegt].
- Verwandt: ADR 0011 (Konfiguration), ADR 0036 (Releases), ADR 0060 (Release-Vorbereitung).
