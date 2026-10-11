---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung
---

# Mehrbenutzer-Dev-Umgebung nur mit Abschottung

## Context and Problem Statement

Dev-Projekte sind Foren mit Projektbindung, die ein Runner-Agent bedient. Der Runner startet einen
Coding-Agenten ohne Werkzeug-Rückfragen als Benutzer des Hosts, ohne Sandbox. Heute ist das sicher
genug, weil nur der Owner auslöst. Sobald eine zweite Person auslösen darf, bedeutet "Auslösen"
Shell-Zugriff auf den Host mit allen Geheimnissen des Betreibers; dasselbe gilt für jede
Prompt-Injection in Repository-Inhalten. Baseline Abschnitt 6 plant die Mehrbenutzer-Dev-Umgebung
als spätere Etappe.

Unter welchen Bedingungen dürfen weitere Personen Dev-Läufe auslösen, und wie setzt der Server das
durch?

## Decision Drivers

- Kein Weg von einer Person mit `trigger` zur Shell des Hosts oder zu Geheimnissen des Betreibers.
- Trennung der Personen voneinander.
- Durchsetzung im Server, nicht durch Konfiguration, die man vergessen kann.
- Grenze auch gegen Injektion, nicht nur gegen böse Absicht.
- Machbar mit vorhandenen Werkzeugen und Kosten, die ein kleines Team trägt.
- Vereinbar mit der Backend-Richtlinie (I1) und den Anbieterbedingungen.

## Considered Options

1. Dev-Projekte dauerhaft nur für Owner.
2. Gemeinsamer Runner, aber mit eingeschalteten Werkzeug-Rückfragen.
3. Eigener Benutzer des Betriebssystems je Person auf dem Host.
4. Umgebung je Person mit Hypervisor-Grenze (lokale VM oder Cloud-VM), freigeschaltet über einen
   geprüften Datensatz `dev_environments` und eine Schutzregel im Server.

## Decision Outcome

Gewählte Option: 4, weil nur eine VM-Grenze Host, Geheimnisse und andere Personen auch dann
schützt, wenn der Agent vollständig dem Auslöser oder einer Injektion folgt, und weil die
Freischaltung als Datensatz mit Prüfungen vom Server bei jedem Aufruf geprüft werden kann.

Festlegungen ([../spec/21-dev-isolation.md](../spec/21-dev-isolation.md),
[../spec/07-rechte.md](../spec/07-rechte.md)):

- Schutzregel SR3: Ein Nicht-Owner löst in einem Dev-Projekt nur aus, wenn eine eigene, bereite
  Umgebung besteht; der Lauf läuft genau dort; Threads bleiben an ihre Umgebung gebunden.
- Schutzregel SR2: Agenten der Werkzeugklasse `host` löst nur ein Owner aus oder eine Person in
  ihrer eigenen bereiten Umgebung.
- Bereit heißt: Prüfungen D1 bis D10 bestanden und nicht älter als `dev.reverify_interval`, Red Team
  für Art und Abbild bestanden, Freigabe durch den Owner mit Step-up.
- Standard ist eine Linux-VM je Person; macOS-VM nur bei Bedarf an Apple-Werkzeugen; Cloud-VM
  gleichwertig. Wo die VMs laufen, entscheidet der Owner.
- LLM-Zugang fremder Umgebungen in dieser Etappe nur über Organisations-Schlüssel.

Bis eine Umgebung bereit ist, verhält sich das Produkt wie Option 1, ohne dass jemand daran denken
muss.

### Consequences

- Gut, weil die heutige Grenze ("nur Owner") ohne Konfiguration erhalten bleibt, bis die Abschottung
  nachgewiesen ist.
- Gut, weil Freigaben (`trigger` auf ein Forum) und Abschottung getrennt sind: Eine Person kann
  mitlesen und schreiben, bevor sie auslösen darf.
- Gut, weil eine gescheiterte Prüfung die Umgebung sofort sperrt.
- Schlecht, weil VMs Betrieb, Abbilder, Netzregeln und Kosten verursachen.
- Schlecht, weil macOS-VMs je Mac begrenzt sind.
- Schlecht, weil das persönliche Abo einer Person in ihrer eigenen Umgebung erst nach einer Änderung
  an I1 möglich wird (21, K-DV-2: auf die Etappe verschoben; bis dahin nur `organization`,
  R-DM-24).

### Confirmation

- Tests DEV-T01 bis DEV-T07 (21) und AZ-M15 bis AZ-M20 (07).
- Red Team P3 je Umgebungsart und Abbild ([../spec/19-sicherheit.md](../spec/19-sicherheit.md),
  Abschnitt 13), Ergebnis im Datensatz (`red_team_passed_at`, `red_team_ref`).

## Pros and Cons of the Options

### 1. Dauerhaft nur Owner

- Gut, weil sicher und ohne Aufwand.
- Schlecht, weil eine Anforderung des Produkts (Mitarbeit weiterer Personen an Dev-Projekten)
  dauerhaft fehlt.

### 2. Gemeinsamer Runner mit Werkzeug-Rückfragen

- Gut, weil ohne neue Infrastruktur.
- Schlecht, weil Rückfragen in langen Läufen abgeschaltet oder blind bestätigt werden.
- Schlecht, weil eine bestätigte Rückfrage immer noch mit den Rechten des Host-Benutzers läuft.

### 3. Eigener Benutzer je Person

- Gut, weil auf jedem Host ohne Virtualisierung machbar.
- Schlecht, weil gemeinsamer Kernel, lokale Dienste und Fehlkonfigurationen (lesbare Dateien,
  weitergereichter SSH-Agent) die Grenze aufweichen.
- Schlecht, weil Netzregeln je Benutzer schwer durchzusetzen sind.

### 4. VM je Person mit Bereitschaftsprüfung

- Gut, weil die Grenze auch gegen Injektion hält.
- Gut, weil Zurücksetzen auf ein sauberes Abbild möglich ist.
- Neutral, weil mehrere Plattformen (Linux, macOS, Cloud) unterstützt werden müssen.
- Schlecht, weil Aufwand und Kosten.

## More Information

- Studie C: Dev-Umgebung für andere nur als eigener Benutzer oder VM (VM stärker), eigene Anmeldung
  oder eigener Schlüssel, eigener Git-Zugang, Netz nur zu Anbieter, Git-Host und Paketquellen.
- Bedrohung T-DEV-01: [../security/threat-model.md](../security/threat-model.md).
- Anthropic erlaubt die eigene Anmeldung einer Person im unveränderten Claude Code, auch wenn eine
  Plattform es betreibt: https://code.claude.com/docs/en/legal-and-compliance, abgerufen 2026-10-10
  [belegt].
