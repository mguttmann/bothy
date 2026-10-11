---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team D (Agenten-Plattform), Team C (Sicherheit)
informed: Team B (API)
---

# Herkunfts- und Vertrauensmarkierung fremder Inhalte im Agenten-Kontext

## Context and Problem Statement

Ein Agent bekommt bei jedem Lauf Kontext: die auslösende Nachricht, frühere Nachrichten des
Threads, Anhänge und Transkripte. Darin stehen Texte von Menschen ohne Auslöserecht, von anderen
Agenten, Apps und Importen. Jeder dieser Texte kann Anweisungen enthalten, die die Laufzeit für
einen Auftrag hält (Prompt-Injection, [threat-model.md](../security/threat-model.md), T-AI-01 und
T-AI-03). Die Laufzeit kann Absender nicht selbst unterscheiden, und nur der Server weiß, wer zur
Zeit des Laufs welches Recht hat. Wer baut den Kontext, und wie erfährt die Laufzeit, welchen
Inhalten sie folgen darf?

## Decision Drivers

- Nur `authz` kennt die Rechte der Autoren; der Adapter soll sie nicht nachbauen (B7).
- Anweisungen kommen nur vom Auslöser; alles andere ist Material (SEC-AI-1 in
  [19-sicherheit.md](../spec/19-sicherheit.md)).
- Rechte gelten zur Zeit des Laufs, nicht zur Zeit des Schreibens.
- Die Marke muss jede Laufzeit erreichen, auch fremde Adapter (Hermes Agent, Claude-Code-Runner,
  SDK-Adapter).
- Prompt-Injection ist nicht vollständig verhinderbar [Q1, belegt]; Markierung ist eine von
  mehreren Grenzen, nicht die einzige.

## Considered Options

- Der Server baut den Kontext mit Rolle und Vertrauensmarke je Element (`run.context.read`,
  Schema `run_context.v1`); Adapter geben markierte Inhalte als Daten in einem festen Systemteil
  weiter
- Der Adapter baut den Kontext selbst aus Ereignissen und `thread.read`
- Der Server filtert: Inhalte nicht vertrauenswürdiger Autoren erreichen den Agenten gar nicht
- Erkennung von Injektionen mit einem Klassifikator vor der Weitergabe
- Keine Markierung; nur Werkzeugklasse, Freigabe-Karten und Auftragsbindung begrenzen den Schaden

## Decision Outcome

Gewählte Option: "Der Server baut den Kontext mit Rolle und Vertrauensmarke", weil nur er die Marke
richtig setzen kann und jeder Adapter sie dann gleich weitergibt. Normativ in
[09-agenten.md](../spec/09-agenten.md), Abschnitt 4.6 (R-AG-06).

1. Der Adapter holt den Kontext mit `run.context.read` (`GET /api/v1/runs/{id}/context`, nur für
   eigene aktive Läufe). Jedes Element trägt `author`, `origin`, `role` (`user`, `assistant`,
   `other`) und `trust`.
2. `trust` setzt der Server zur Abfragezeit: `trigger` (auslösender Mensch), `authorized` (anderer
   Mensch mit wirksam `trigger` auf dem Kanal), `self` (dieser Agent), `unverified` (Mensch ohne
   `trigger`, andere Agenten, Apps, Importe), `system`.
3. Pflicht für jeden Adapter: Inhalte mit `unverified` gehen an die Laufzeit nur als markierte
   Daten mit einem festen Hinweis davor, nie als Anweisung. Der Kontext ist ein fester Systemteil
   mit Autor und Marke, getrennt vom Text des Auslösers.
4. Inhalte von Dateien (über `file.read` geholt) sind immer Daten, auch wenn der Auslöser sie
   hochgeladen hat: Ein Dokument aus dem Netz kann Anweisungen enthalten, die nicht vom Menschen
   stammen (T-AI-03). Das Transkript einer Sprachnachricht trägt dagegen die Marke ihrer Nachricht,
   weil es die Worte ihres Autors sind.
5. Die auslösende Nachricht wird nie gekürzt; der übrige Kontext hat die Grenzen
   `agent.context_message_max_bytes` und `agent.context_max_bytes`.
6. Die übrigen Grenzen bleiben: Agenten lösen nie aus (SR4), Werkzeugklasse (SR2), Freigabe-Karten
   nur für den Auslöser (SEC-AI-4), Auftragsbindung (SR9 in [07-rechte.md](../spec/07-rechte.md)).

### Consequences

- Gut, weil die Marke an einer Stelle entsteht und Rechte-Änderungen sofort wirken: Verliert ein
  Autor `trigger`, sind seine Nachrichten im nächsten Kontext `unverified`.
- Gut, weil Adapter keine Rechte-Logik brauchen und die Nutzlast nicht in Ereignisse passen muss.
- Gut, weil fremde Inhalte weiter verfügbar sind: Ein Agent kann eine Kundenmail zusammenfassen,
  ohne ihr zu gehorchen. Ein Filter würde den Nutzen zerstören.
- Schlecht, weil die Wirkung an der Laufzeit hängt: Markierung der Herkunft senkte in einer Studie
  die Erfolgsrate von Angriffen deutlich [Q2, belegt], verhindert sie aber nicht [Q1, belegt]. Das
  Restrisiko steht im Bedrohungsmodell.
- Schlecht, weil jeder Adapter die Formatierung selbst umsetzt; Fehler fallen nur im
  Konformitätstest auf. Gegenmittel: der Pflicht-Test AG-20 für jeden Referenz-Adapter und eine
  Formatierungshilfe in der Handschicht der SDKs (ADR 0026).
- Neutral, weil ein Kontextabruf je Lauf zusätzliche Last bringt; er ersetzt das Mitlesen aller
  Kanal-Ereignisse im Adapter.

### Confirmation

- Pflicht-Szenario AG-20 ([09-agenten.md](../spec/09-agenten.md), Abschnitt 16.2): Autor ohne
  `trigger`, anderer Agent und Import ergeben `trust = unverified`; der Adapter-Test zeigt den Text
  nur im markierten Block.
- Pflicht-Szenario AG-21: Kontext über `agent.context_max_bytes` wird von alt nach neu gekürzt, der
  Auslöser bleibt vollständig.
- Test: Ein Autor verliert `trigger` zwischen zwei Läufen; im zweiten Kontext ist seine Nachricht
  `unverified`.
- Test im Referenz-Adapter: Der Inhalt eines Anhangs erscheint nur als markierte Daten, auch bei
  Upload durch den Auslöser.
- Bau-Ticket M2-13 in [PLAN.md](../PLAN.md) (SEC-AI-1, Größengrenzen).

## Pros and Cons of the Options

### Server baut den Kontext mit Marken

- Gut, weil Rechte und Marke aus einer Quelle kommen.
- Schlecht, weil eine weitere Operation und ein versioniertes Schema gepflegt werden.

### Adapter baut den Kontext

- Gut, weil der Server weniger tut.
- Schlecht, weil der Adapter die Rechte jedes Autors kennen müsste; er sieht nur seine eigenen
  Freigaben, nicht die anderer Menschen.

### Server filtert fremde Inhalte

- Gut, weil nicht vertrauenswürdiger Text die Laufzeit nie erreicht.
- Schlecht, weil Agenten genau solche Inhalte bearbeiten sollen (Zusammenfassen, Beantworten); der
  Kontext wäre lückenhaft und Antworten falsch.

### Klassifikator für Injektionen

- Gut, weil auffällige Texte vorab erkannt werden könnten.
- Schlecht, weil der Server dafür Modelle aufrufen müsste (gegen B10 und R-AG-14) und Erkennung
  umgangen werden kann; OWASP nennt keine sichere Erkennung [Q1, belegt].

### Keine Markierung

- Gut, weil nichts zu bauen ist.
- Schlecht, weil die Laufzeit Anweisungen in fremden Texten nicht von Aufträgen unterscheiden kann;
  OWASP empfiehlt, fremde Inhalte zu trennen und zu kennzeichnen [Q1, belegt].

## More Information

- Quellen (abgerufen 2026-10-10): Q1 OWASP Gen AI Security Project, "LLM01:2025 Prompt Injection",
  https://genai.owasp.org/llmrisk/llm01-prompt-injection/ [belegt]; Q2 Hines u. a., "Defending
  Against Indirect Prompt Injection Attacks With Spotlighting" (2024),
  https://arxiv.org/abs/2403.14720 [belegt; Messung mit GPT-Modellen, Übertragbarkeit auf andere
  Modelle abgeleitet].
- Format im Referenz-Adapter für Hermes Agent: [09-agenten.md](../spec/09-agenten.md),
  Abschnitt 17.6.
- Verwandt: ADR 0022 (Agenten außerhalb des Servers), ADR 0023 (Aufruf-Marke), ADR 0025 (Lauf),
  ADR 0026 (SDKs).
