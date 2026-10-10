---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team Agenten-Plattform
---

# Signierte Aufruf-Marke und Prüfung im Adapter

## Context and Problem Statement

Der Server entscheidet, ob ein Mensch einen Agenten auslösen darf (`agent.invoke`, I2). Der
Adapter außerhalb des Servers startet daraufhin eine Laufzeit mit echten Kosten und
Nebenwirkungen. Wie belegt der Server dem Adapter diese Entscheidung so, dass der Adapter nichts
ohne gültige Entscheidung startet, Doppelstarts unmöglich sind und der Adapter prüfen kann, ob
sein Backend zum Soll passt (I3)?

## Decision Drivers

- Der Adapter darf ohne gültige Entscheidung nichts tun (Pflicht-Test aus 07).
- Die Entscheidung muss zum Zeitpunkt der Zustellung gelten, nicht zum Zeitpunkt der Nachricht
  (Rechte-Entzug wirkt sofort).
- Der Adapter muss Soll und Ist der Backend-Richtlinie vergleichen können (I3).
- Ein Adapter darf keine Entscheidung fälschen können, auch nicht mit seinem eigenen Token.
- Schlüsselwechsel ohne Ausfall.

## Considered Options

1. Asymmetrisch signiertes JWT (JWS, EdDSA) je Zustellung, Schlüssel über JWKS, Prüfung im
   Adapter und erneut beim Lauf-Start.
2. HMAC-Prüfvermerk mit einem gemeinsamen Geheimnis je Agent.
3. Keine Marke; der Adapter vertraut der authentifizierten Gateway-Verbindung.
4. Rückfrage des Adapters beim Server vor jedem Start ("darf ich?"), ohne Marke.

## Decision Outcome

Gewählt: Option 1, weil nur sie Fälschungssicherheit gegenüber dem Adapter, einen prüfbaren Beleg
mit allen Feldern für I3 und einen Schlüsselwechsel ohne gemeinsames Geheimnis verbindet. Die Marke
wird bei jeder Zustellung neu ausgestellt (R-SY-07); der Server prüft sie beim Lauf-Start erneut
(`jti`, Ablauf, Soll gleich Ist).

### Consequences

- Gut, weil ein Adapter-Token allein keinen Lauf starten kann.
- Gut, weil der Claim `billing_mode` den Vergleich Soll gegen Ist am Ort der Laufzeit erzwingt.
- Gut, weil ein Notfall-Schlüsselwechsel höchstens eine Zustellung verzögert: Die nächste
  Zustellung trägt eine Marke des neuen Schlüssels.
- Schlecht, weil jeder Adapter JOSE korrekt prüfen muss (Algorithmen-Erlaubnisliste, `typ`,
  Zeitfenster). Das SDK liefert die Prüfung fertig (ADR 0026).
- Schlecht, weil Uhren von Server und Adapter ungefähr stimmen müssen (Toleranz als
  Konfigurationswert).

### Confirmation

- Szenario AG-08 in docs/spec/09-agenten.md: fehlende Marke, falsche Signatur, anderer `alg`,
  fremdes `aud`, abgelaufen, falscher `typ` führen zu keinem Start.
- Szenario AG-22: planmäßiger Wechsel ohne fehlgeschlagene Läufe, Notfall mit höchstens einer
  verzögerten Zustellung.
- Szenario AG-09: doppelte Zustellung führt zu genau einem erfolgreichen `run.start`.
- Ein Schema-Test hält die Claims gleich mit docs/spec/05-aktionen.md Abschnitt 5.

## Pros and Cons of the Options

### Option 1: Asymmetrisches JWT je Zustellung

- Gut, weil der Adapter prüfen, aber nicht fälschen kann (R-SY-08).
- Gut, weil Standard-Bibliotheken existieren und RFC 8725 die Regeln festlegt (explizites
  `typ`, Algorithmen-Erlaubnisliste;
  https://www.rfc-editor.org/rfc/rfc8725.html, abgerufen 2026-10-10, [belegt]).
- Neutral, weil die JWKS veröffentlicht und zwischengespeichert werden muss.
- Schlecht, weil Signieren je Zustellung Rechenzeit kostet (EdDSA ist dafür schnell).

### Option 2: HMAC mit gemeinsamem Geheimnis

- Gut, weil einfach und schnell.
- Schlecht, weil wer prüfen kann, auch fälschen kann: Ein kompromittierter Adapter könnte sich
  selbst Marken ausstellen.
- Schlecht, weil das Geheimnis je Agent verteilt und rotiert werden muss.

### Option 3: Vertrauen in die Verbindung

- Gut, weil kein zusätzlicher Mechanismus nötig ist.
- Schlecht, weil der Adapter keinen Beleg für Soll, Auslöser und Ablauf hat; I3 wäre nicht
  prüfbar.
- Schlecht, weil eine Zustellung über einen anderen Weg (REST-Abruf, Webhook) nichts belegen
  würde.

### Option 4: Rückfrage vor jedem Start

- Gut, weil die Entscheidung immer ganz frisch ist.
- Schlecht, weil sie einen zusätzlichen synchronen Aufruf je Lauf braucht und bei Netzproblemen
  den Start blockiert.
- Schlecht, weil der Adapter dem Ergebnis nur über seine Verbindung glauben kann, also wie
  Option 3 keinen eigenständigen Beleg hat.

## More Information

- Baseline I2, I3; docs/spec/04-sync-protokoll.md Abschnitt 15.3; docs/spec/05-aktionen.md
  Abschnitt 5; docs/spec/09-agenten.md Abschnitt 4.5.
- Die Claims einschließlich `space_id`, `trigger_kind`, `command_id`, `tool_class` und Kopf `typ`
  stehen in docs/spec/05-aktionen.md Abschnitt 5.
