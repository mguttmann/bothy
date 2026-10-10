---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team D (Agenten-Plattform), Team C (Sicherheit)
informed: Team F (Betrieb)
---

# Kostenabgleich als eigene App, Admin-Key nie im Server

## Context and Problem Statement

Im Modus `organization` zeigt das Produkt Kosten je Lauf, Thread, Bereich und Agent. Diese Beträge
sind Schätzungen aus Token-Zahlen und einer Preistabelle; Zuschläge, Rabatte und Preisstufen fehlen
([10-backend-richtlinie.md](../spec/10-backend-richtlinie.md), Abschnitte 12.1 und 12.3). Die
tatsächlichen Kosten liefert die Kosten-API des Anbieters, aber nur mit einem Admin-Zugang der
Organisation. Dieser Zugang kann laut Dokumentation Mitglieder, Arbeitsbereiche, Einladungen und
API-Keys verwalten; einen nur lesenden Admin-Key beschreibt die Dokumentation nicht [Q1, belegt].
Wo läuft der Abgleich, ohne dass ein Einbruch in den Chat-Server diesen Zugang erreicht?

## Decision Drivers

- Der Server speichert und vermittelt keine Zugangsdaten zu Modell-Anbietern (B10, I5).
- Ein Einbruch in den Server darf keine Verwaltung der Anbieter-Organisation ermöglichen.
- Abgleich ist optional; ohne ihn bleiben Beträge als "Schätzung" gekennzeichnet.
- Ein Abgleich läuft unbeaufsichtigt und kann keinen Step-up leisten (R-AZ-03).
- Abgeglichene Beträge sind nachvollziehbar und ersetzen Schätzungen, ohne sie zu löschen.

## Considered Options

- Eigene App außerhalb des Servers mit eigenem Secret-Provider und Scope `costs:reconcile`;
  standardmäßig aus
- Abgleich-Job im Server mit dem Admin-Key im Secret-Provider des Servers
- Nur Abgleich von Hand durch den Owner (Beträge aus der Rechnung eintragen)
- Kein Abgleich, nur Schätzungen

## Decision Outcome

Gewählte Option: "Eigene App außerhalb des Servers", weil der Admin-Zugang dann nur in einem
Prozess liegt, der nichts anderes tut, und der Server nie mit dem Anbieter spricht. Normativ in
[10-backend-richtlinie.md](../spec/10-backend-richtlinie.md), Abschnitt 12.4.

1. Die App liest den Admin-Key aus ihrem eigenen Secret-Provider und ruft die Kosten-API mit
   Tagesbuckets ab, gruppiert nach Arbeitsbereich [Q2, belegt]. Empfohlen ist ein Arbeitsbereich
   mit eigenem Key je Agent, weil die API nach Arbeitsbereich und nicht nach Key gruppiert.
2. Je Tag und Arbeitsbereich verteilt sie den Ist-Betrag auf die Schätzungen der zugeordneten
   Agenten im Verhältnis der Schätzungen; der Rundungsrest geht an den größten Eintrag. Beträge ohne
   passenden Lauf meldet sie als "nicht zugeordnet".
3. Sie übermittelt neue Einträge mit `source = 'reconciled'`, `is_estimate = false` und
   `supersedes_id` über die Aktion `cost.reconcile.submit` (Scope `costs:reconcile`, ohne Step-up,
   [05-aktionen.md](../spec/05-aktionen.md), Abschnitt 4.12). Die Aktion legt nur Einträge zu
   vorhandenen Läufen an und löscht nichts.
4. Grenzen der App: Abstand der Anfragen `reconciler.min_request_interval`, Rückblick
   `reconciler.lookback`, Zuordnung `reconciler.workspace_map` (Konfiguration der App, nicht des
   Servers).
5. Der Owner kann Schätzungen weiterhin von Hand abgleichen (`cost.reconcile`, Step-up, Audit).

### Consequences

- Gut, weil ein Einbruch in den Server den Admin-Zugang nicht erreicht; die App braucht keinen
  eingehenden Port.
- Gut, weil der Server keinen Anbieter-Code und keine Anbieter-Schlüssel enthält; der Abgleich für
  andere Anbieter ist eine weitere App.
- Gut, weil abgeglichene Einträge die Schätzung ersetzen, aber nicht löschen; die Spur bleibt
  prüfbar.
- Schlecht, weil eine weitere Komponente betrieben, aktualisiert und überwacht werden muss.
- Schlecht, weil die Kosten-API nur Tagesbuckets kennt: Ein Betrag je Lauf ist eine anteilige
  Verteilung, keine Messung. Teilen sich mehrere Agenten einen Arbeitsbereich, wird auch zwischen
  ihnen verteilt.
- Schlecht, weil Kosten der Priority-Stufe im Kosten-Endpunkt fehlen [Q2, belegt]; solche Beträge
  bleiben "nicht zugeordnet" oder fehlen.

### Confirmation

- Test BR-10 ([10-backend-richtlinie.md](../spec/10-backend-richtlinie.md), Abschnitt 15): Die
  Verteilung eines Tagesbetrags ergibt in Summe den Ist-Betrag, der Rest geht an den größten
  Eintrag, Schätzungen sind ersetzt.
- Test BR-13: Ein Geheimtext-Kanarienwert erscheint in keiner Antwort und keinem Protokoll.
- Test: `cost.reconcile.submit` ohne Scope `costs:reconcile` wird abgelehnt; mit Scope legt die
  Aktion nur `reconciled`-Einträge zu vorhandenen Läufen an und ändert oder löscht keine anderen.
- Semgrep-Regel `bothy-go-no-model-credentials` (`.semgrep/`): Der Server liest keine
  Anbieter-Zugangsdaten aus der Umgebung.
- Bau-Ticket M2-28 in [PLAN.md](../PLAN.md): Der Server speichert keinen Anbieter-Schlüssel.

## Pros and Cons of the Options

### Eigene App

- Gut, weil Rechte und Geheimnis eng begrenzt sind.
- Schlecht, weil ein weiterer Prozess mit eigener Konfiguration entsteht.

### Abgleich-Job im Server

- Gut, weil nichts zusätzlich zu betreiben ist.
- Schlecht, weil der Server dann einen Zugang hält, mit dem man Keys und Mitglieder der
  Anbieter-Organisation verwalten kann; das widerspricht B10 und I5.
- Schlecht, weil der Server erstmals selbst mit einem Modell-Anbieter spräche.

### Nur von Hand

- Gut, weil kein Admin-Zugang irgendwo gespeichert ist.
- Schlecht, weil die Arbeit täglich anfällt und Fehler bei der Zuordnung zu Läufen wahrscheinlich
  sind. Bleibt als Korrekturweg.

### Kein Abgleich

- Gut, weil nichts zu bauen ist.
- Schlecht, weil Beträge dauerhaft Schätzungen bleiben; Cache-Schreiben ohne gemeldete Dauer,
  Zuschläge und Rabatte verfälschen sie ohne Korrektur (10, Abschnitt 12.3).

## More Information

- Quellen (abgerufen 2026-10-10): Q1 Admin API,
  https://platform.claude.com/docs/en/manage-claude/admin-api [belegt]; Q2 Usage and Cost API,
  https://platform.claude.com/docs/en/manage-claude/usage-cost-api [belegt]. Beide wie in
  [10-backend-richtlinie.md](../spec/10-backend-richtlinie.md), Abschnitt 12.4, zitiert.
- Verwandt: ADR 0022 (Agenten und Plugins außerhalb des Servers), ADR 0024 (Backend-Richtlinie),
  ADR 0011 (Secret-Provider).
