---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team Migration (I), Team API (B), Team Sicherheit und Rechte (C)
informed: alle Teams
---

# Discord-Import: nur lesen, Trockenlauf, Herkunftsschlüssel

## Context and Problem Statement

Organisationen, die Bothy einführen, haben ihren Verlauf oft in Discord: Kategorien, Kanäle,
Foren, Threads mit Agenten-Sitzungen, Anhänge. Bothy soll diesen Verlauf übernehmen, Kanal für
Kanal umschalten und während einer Übergangszeit neue Discord-Nachrichten nachziehen
([22-import-migration.md](../spec/22-import-migration.md)).

Dabei gelten harte Grenzen der Baseline: Der Server lädt nie fremde URLs (Abschnitt 4, SSRF),
Erweiterungen laufen nie im Server-Prozess (B6), jede Route geht durch `authorize` (B7), und
importierte Nachrichten dürfen nie einen Agenten auslösen. Die Vorgänger haben zwei Fehlerklassen
mehrfach gezeigt: Doppel nach unsicherer Zustellung und ein Werkzeug, das bei einem vergessenen
Schalter schreibt (REUSE.md, Lehren L02, L17, L18).

Frage: Wo läuft der Import, wie greift er auf Discord zu, und wie verhindert er Doppel bei
Abbruch und Wiederholung?

## Decision Drivers

- Kein server-seitiges Laden fremder URLs; das Discord-Token gehört nicht in den Server.
- Ein Lauf ist an jeder Stelle abbrechbar und fortsetzbar, ohne Doppel.
- Der Trockenlauf ist der Standard und schreibt nachweisbar nichts.
- Discord wird nur gelesen; das Werkzeug kann Discord nicht verändern.
- Rechte, Audit und Ereignisse gelten wie bei jedem anderen Schreibweg.
- Der Weg taugt auch für einen späteren zweiten Quell-Typ (Zulip).

## Considered Options

1. Eigener Prozess `bothy import discord` über eine Import-Schnittstelle der öffentlichen API;
   Discord nur `GET`; Idempotenz über `import_items (source_id, origin_key)`.
2. Import als Hintergrund-Job im Server (Rolle `worker`), der Discord selbst abruft.
3. Direkter Datenbank-Import (Konverter schreibt SQL an der Anwendung vorbei).

## Decision Outcome

Gewählt: Option 1, eigener Prozess über die Import-Schnittstelle.

Festlegungen:

1. Der Discord-Client im Werkzeug kennt nur `GET`; die Liste der Endpunkte steht in Kapitel 22,
   Abschnitt 4.1. Das Token liegt nur beim Werkzeug.
2. Ohne `--apply` ist jeder Aufruf ein Trockenlauf: nur `GET` gegen Discord und Bothy, keine
   Datei, kein Server-Zustand.
3. Jedes übernommene Objekt hat einen Herkunftsschlüssel. Der Server schreibt `pending`, legt das
   Ziel an und setzt `posted` in derselben Transaktion; eine Wiederholung liefert das vorhandene
   Ziel.
4. Fortschritt liegt im Server (`import_sources.cursor`, `stats`), nicht in einer lokalen Datei.
5. Importierte Nachrichten haben `origin = import`, `silent = true` und keine `mentions`.

### Consequences

- Gut, weil der Server keine Verbindung zu Discord aufbaut und kein Discord-Token kennt; die
  SSRF-Grenze der Baseline bleibt ohne Ausnahme.
- Gut, weil Rechte, Audit und Ereignisse über die normalen Schichten laufen; nur Owner dürfen
  importieren.
- Gut, weil ein abgebrochener Lauf auf jedem Rechner fortsetzt; der Zustand liegt im Server.
- Gut, weil derselbe Weg einen späteren Zulip-Import trägt.
- Schlecht, weil die API eine Owner-Schnittstelle mit Rückdatierung und fremdem Autor bekommt
  (`import.apply`, 05 Abschnitt 4.12). Ausgleich: nur Owner, nur für eine laufende
  Quelle, nur mit `origin = import`, Start mit Step-up, Audit.
- Schlecht, weil ein langer Lese-Spiegel einen laufenden Client-Prozess braucht. Ausgleich: Der
  Prozess ist zustandslos und kann als eigener Container mit Neustart-Regel laufen.

### Confirmation

Diese Tests halten die Entscheidung im Repo (Kapitel 22, Abschnitt 16):

- Trockenlauf-Test: Fake-Discord und Fake-Bothy zählen Methoden; erlaubt ist nur `GET`; Temp- und
  Arbeitsordner bleiben leer.
- Statische Prüfung: Der Discord-Client exportiert keine Methode mit einem anderen Verb als `GET`.
- Absturz-Test: Abbruch zwischen Stapel und Cursor; ein neuer Lauf erzeugt jedes Objekt genau
  einmal und hält die Reihenfolge je Thread.
- Fehler-Injektion: Fake-Bothy speichert, antwortet dann `502`; die Wiederholung liefert dasselbe
  Ziel.
- Auslöse-Test: Eine importierte Nachricht in einem Kanal mit Bediener erzeugt keinen Lauf.
- Abhängigkeitsregel nach ADR 0008: Kein Server-Paket importiert den Discord-Client.

## Pros and Cons of the Options

### Option 1: Eigener Prozess über die Import-Schnittstelle

- Gut, weil der Server nie fremde URLs lädt und kein Fremd-Token hält.
- Gut, weil der Trockenlauf ohne Server-Schreibrecht und ohne lokale Datei auskommt.
- Gut, weil die Idempotenz im Ziel liegt und nicht im Werkzeug.
- Neutral, weil der Import-Durchsatz durch die API und ihre Rate-Limit-Klasse begrenzt ist; die
  Discord-Grenzen sind ohnehin enger.
- Schlecht, weil eine zusätzliche, mächtige Owner-Route entsteht.

### Option 2: Hintergrund-Job im Server

- Gut, weil Wiederaufnahme und Fortschritt die vorhandene Job-Infrastruktur nutzen.
- Gut, weil kein Client-Prozess über Tage laufen muss.
- Schlecht, weil der Server ausgehende Verbindungen zu Discord und zum CDN aufbaut; das bricht
  die Baseline-Grenze gegen server-seitiges Laden und braucht eine Ausnahme mit Host-Liste.
- Schlecht, weil das Discord-Token in den Secret-Provider des Servers muss.
- Schlecht, weil Discord-spezifischer Code in den Kern wandert.

### Option 3: Direkter Datenbank-Import

- Gut, weil er am schnellsten schreibt.
- Schlecht, weil er `authorize`, Audit, Ereignis-Log und Rechte-Version umgeht (B7, B8).
- Schlecht, weil jede Migration des Schemas den Konverter bricht.
- Schlecht, weil ein Fortsetzen nach Abbruch eigene Logik neben `import_items` bräuchte.

## More Information

- Discord-Endpunkte, Intents und Rate-Limits mit Quellen: Kapitel 22, Abschnitt 4 und Quellen.
- Bauform der Kommandozeile und Anmeldung als Owner (Gerät `platform = cli`, Step-up):
  [08-identitaet.md](../spec/08-identitaet.md).
- Lehren der Vorgänger (Trockenlauf-Beweis, Herkunftsschlüssel, Ruhe-Regel, CDN-Abruf):
  [REUSE.md](../REUSE.md).
- Abgleich mit dem Kern-Vertrag: Kapitel 22, Abschnitt 18.
- Neu bewerten, wenn ein Quell-Typ nur über einen Server-Push (Webhook) erreichbar ist; dann
  braucht der Server einen Eingang, aber weiterhin keinen ausgehenden Abruf.
