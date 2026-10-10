---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Abhängigkeits-Updates mit Renovate

## Context and Problem Statement

Das Repository pinnt alles: Actions per Commit-SHA, Images per Digest, Python- und
Node-Werkzeuge per Lock-Datei, Release-Binärdateien per sha256, einzelne Versionen im
Makefile. Ohne automatische Updates veralten die Pins, mit zu eiligen Updates kommt eine
kompromittierte Version schnell herein (Trivy-Vorfall 2026-03-19,
https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23, abgerufen
2026-10-10) [belegt]. Welches Werkzeug hält die Pins aktuell?

## Decision Drivers

* Alle Pin-Orte abdecken, auch eigene Formate (`tools/bin/tools.lock.json`, Makefile).
* Wartezeit vor neuen Versionen (Cooldown), Sicherheits-Updates ohne Wartezeit.
* Gruppierte Pull Requests, damit `make check` je Gruppe einmal läuft.
* Actions und Images bleiben gepinnt.

## Considered Options

* Renovate
* Dependabot
* Manuelle Updates nach Kalender

## Decision Outcome

Gewählt: "Renovate", weil es eigene Pin-Formate über Regex-Manager abdeckt, uv-Lock-Dateien
pflegt (Manager pep621, https://docs.renovatebot.com/modules/manager/pep621/, abgerufen
2026-10-10) [belegt] und mit `config:best-practices` Actions und Images auf Digests pinnt
(https://docs.renovatebot.com/presets-config/, abgerufen 2026-10-10) [belegt].

Konfiguration (`renovate.json`): wöchentlich, gruppiert (Actions, Python-Werkzeuge,
Node-Werkzeuge, Release-Binärdateien, lokaler SonarQube), Wartezeit
`deps.min_release_age`, Major-Updates erst nach Freigabe im Dependency Dashboard.
Ausnahmen:

* Release-Binärdateien: Renovate erhöht die Version, ein Maintainer erneuert die
  sha256-Pins mit `make tools-lock`. Bis dahin schlägt CI fehl; jede neue Binärdatei geht
  so durch eine menschliche Prüfung.
* Trivy ist für Renovate abgeschaltet und bleibt auf der im Advisory als sicher genannten
  Version, bis die Prüfung aus docs/quality/toolchain.md eine neuere freigibt.

### Consequences

* Gut, weil alle Pins einen Update-Weg haben und nichts unbemerkt veraltet.
* Gut, weil die Wartezeit kurzlebige bösartige Versionen meist abfängt, bevor sie
  vorgeschlagen werden.
* Schlecht, weil Release-Binärdateien zwei Schritte brauchen. Das ist gewollt.

### Confirmation

* `make lint` prüft `renovate.json` gegen das Renovate-Schema (check-jsonschema).
* `tools/docs/tests/test_config_values.py` hält `minimumReleaseAge` gleich
  `deps.min_release_age` in Kapitel 23.
* `tools/bin/tests/test_fetch_tools.py` prüft, dass eine Binärdatei ohne passende sha256
  nicht installiert wird.

## Pros and Cons of the Options

### Renovate

* Gut, weil Regex-Manager, Gruppen, Wartezeit und Digest-Pinning eingebaut sind.
* Neutral, weil die gehostete App ein Drittanbieter-Dienst ist.

### Dependabot

* Gut, weil in GitHub eingebaut.
* Schlecht, weil eigene Pin-Formate (Lock-Datei der Binärdateien, Makefile) nicht
  abgedeckt sind.

### Manuelle Updates

* Gut, weil volle Kontrolle.
* Schlecht, weil sie erfahrungsgemäß ausbleiben, gerade bei Sicherheits-Updates.

## More Information

Ob die gehostete Renovate-App für das private Repository genutzt wird oder erst ab dem
Öffentlich-Schalten, entscheidet der Owner (offene Frage im Rückgabebericht).
