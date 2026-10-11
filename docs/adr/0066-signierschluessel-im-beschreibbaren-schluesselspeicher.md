---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team F (Betrieb), Team C (Sicherheit), Team D (Agenten-Plattform)
informed: alle Planungsteams
---

# Signierschlüssel im beschreibbaren Schlüsselspeicher

## Context and Problem Statement

Der Server signiert Zugriffstokens, Aufruf-Marken, ausgehende Webhooks und Audit-Prüfpunkte mit
eigenen Schlüsseln je Zweck. Der private Teil liegt nur im Secret-Provider, die Datenbank hält den
öffentlichen Teil und den Verweis (`server_signing_keys.private_key_ref`,
[03-datenmodell.md](../spec/03-datenmodell.md)). Schlüssel durchlaufen die Zustände `next`,
`active`, `retiring` und `retired`; die Aufruf-Marke wechselt planmäßig, und jeder Zweck braucht
eine Notfall-Rotation ([09-agenten.md](../spec/09-agenten.md), Abschnitt 4.5;
[19-sicherheit.md](../spec/19-sicherheit.md), Abschnitt 10).

Die Treiber `file` und `env` des Secret-Providers lesen nur, und Compose bindet die Geheimnisse
des Servers nur lesend ein ([16-betrieb.md](../spec/16-betrieb.md), Abschnitt 5). Damit kann der
Server keinen neuen Schlüssel anlegen. Wie entstehen neue private Schlüssel ohne Handgriff auf dem
Host, ohne dass der Server andere Geheimnisse schreiben kann?

## Decision Drivers

- Planmäßige Rotation läuft ohne Eingriff; eine Notfall-Rotation wirkt sofort.
- Der private Teil steht nie in der Datenbank, in Logs oder in Antworten (03, Grundsatz 9).
- Der Server bekommt so wenig Schreibrecht wie möglich; alle anderen Geheimnisse bleiben nur
  lesbar.
- Sicherung und Wiederherstellung erfassen die Schlüssel (Hinterlegung, 16 Abschnitt 5.4).
- Später beschreibbare Treiber (`azkv`, `awssm`) passen ohne neue Entscheidung.

## Considered Options

- Beschreibbarer Schlüsselspeicher: ein eigenes Verzeichnis `secrets.signing_key_dir`, das nur der
  Server über `Put` des Datei-Treibers schreibt
- Vorerzeugte Reserveschlüssel: `bothy secrets init` legt Schlüssel auf Vorrat an, der Server
  schaltet nur zwischen ihnen um
- Private Schlüssel mit einem Schlüssel-Verschlüsselungsschlüssel verschlüsselt in der Datenbank

## Decision Outcome

Chosen option: "Beschreibbarer Schlüsselspeicher", because nur er automatische und
Notfall-Rotation ohne Handgriff erlaubt und das Schreibrecht des Servers auf genau ein Verzeichnis
begrenzt.

1. `secrets.signing_key_dir` ist ein Verzeichnis im Besitz des Dienstbenutzers mit Modus `0700`.
   Der Server prüft beim Start Besitz, Modus und Schreibbarkeit; sonst bricht der Start ab.
2. Nur der Datei-Treiber kann schreiben (`Put`, `Delete`), und nur unter diesem Verzeichnis; jeder
   andere Verweis ist nur lesbar, `env:` schreibt nie. `Put` legt eine Datei mit Modus `0600` an,
   schreibt atomar und überschreibt nie einen vorhandenen Namen.
3. Ein Schlüssel ist Ed25519; `kid` ist der Fingerabdruck nach RFC 7638; die Datei heißt
   `<zweck>-<kid>.pem`. Der Server legt die Datei vor der Datenbankzeile an und löscht die private
   Datei, sobald der Schlüssel `retired` ist (die Zeile bleibt als Beleg).
4. `secrets-init` erzeugt keine Signierschlüssel mehr. Fehlt einem Zweck ein `active`-Schlüssel,
   legt der Server ihn beim Start an ([02-architektur.md](../spec/02-architektur.md), Abschnitt
   15.1); `/readyz` scheitert mit `signing_keys`, solange einer fehlt.
5. `bothy secrets export` nimmt den Schlüsselspeicher in das verschlüsselte Bündel auf.

### Consequences

- Good, because Rotation und Notfall-Rotation ohne Zugriff auf den Host laufen.
- Good, because ein Leck des Schlüsselspeichers nur Signierschlüssel betrifft, keine
  Datenbank-Passwörter oder fremden Zugänge.
- Bad, because der Server ein beschreibbares Volume braucht. Ein übernommener Server-Prozess
  gewinnt dadurch wenig: Er liest den aktiven privaten Schlüssel ohnehin; neu ist nur, dass er
  Dateien in diesem einen Verzeichnis anlegen kann.
- Bad, because Mehrknoten-Betrieb (Stufe 2, [02-architektur.md](../spec/02-architektur.md)
  Abschnitt 16) ein gemeinsames Volume oder einen beschreibbaren Treiber (`azkv`, `awssm`) braucht;
  das entscheidet das Ticket des Mehrknoten-Betriebs.
- Neutral, because nach einem Absturz zwischen Datei und Datenbankzeile eine Datei ohne Zeile
  bleiben kann; sie wird nie benutzt.

### Confirmation

- Tests des Bau-Tickets M1-32 ([PLAN.md](../PLAN.md)): `TestSecretsPut_*` (nur unter dem
  Verzeichnis, Modus `0600`, kein Überschreiben), `TestStartup_SigningKeys` (Start scheitert ohne
  beschreibbares Verzeichnis, `/readyz` meldet `signing_keys`), `TestEnsureActive_*`,
  `TestEmergencyRotation_*` und der Kanarien-Test (kein privater Schlüssel in Datenbank, Log oder
  Antwort).
- Compose bindet das Volume `signing-keys` nur in `api`, `worker` und lesend in `secrets-init` ein
  ([infra/compose/compose.yaml](../../infra/compose/compose.yaml)).

## Pros and Cons of the Options

### Beschreibbarer Schlüsselspeicher

- Good, because der Zustandsautomat aus 19 ohne Handgriff durchläuft.
- Good, because das Schreibrecht auf ein Verzeichnis und eine Dateiart begrenzt ist.
- Bad, because ein zusätzliches Volume gesichert werden muss.

### Vorerzeugte Reserveschlüssel

- Good, because der Server nichts schreibt.
- Bad, because die Rotation ohne Handgriff endet, sobald der Vorrat verbraucht ist.
- Bad, because wer das Geheimnis-Verzeichnis liest, auch jeden künftigen Schlüssel kennt; eine
  Notfall-Rotation nach einem solchen Leck hilft dann nicht.

### Verschlüsselt in der Datenbank

- Good, because kein weiteres Volume nötig ist und Mehrknoten-Betrieb ohne gemeinsames Volume geht.
- Bad, because es 03 widerspricht: Der private Teil liegt nur im Secret-Provider.
- Bad, because ein Datenbank-Abzug zusammen mit dem Verschlüsselungsschlüssel alle Schlüssel
  verrät, auch die früheren.

## More Information

- [16-betrieb.md](../spec/16-betrieb.md), Abschnitte 5.2 bis 5.4 und 14 (`secrets.signing_key_dir`);
  [19-sicherheit.md](../spec/19-sicherheit.md), Abschnitte 10 und 16 (`signing_key.*`);
  [09-agenten.md](../spec/09-agenten.md), Abschnitt 4.5.
- Verwandt: ADR 0011 (Secret-Provider), ADR 0020 (Gerätebindung, Zugriffstokens), ADR 0023
  (Aufruf-Marke), ADR 0021 (Audit-Prüfpunkte).
