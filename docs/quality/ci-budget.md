# CI-Budget

Ziel im privaten Zustand: GitHub-gehostete Minuten nahe null. Im öffentlichen Zustand
sind Standard-Runner kostenlos; dann gilt nur noch die Sicherheitsregel aus ADR 0039.

## Ausgangslage

Das Repository liegt im privaten Zustand unter einem persönlichen Konto mit GitHub Pro.
Dessen Inklusivminuten für Actions in privaten Repositories sind bereits überschritten;
jede weitere gehostete Minute wird berechnet.

## Fakten (Stand der Quellen)

| Fakt | Quelle, abgerufen 2026-10-10 | Marker |
|---|---|---|
| Standard-Runner sind in öffentlichen Repositories kostenlos, self-hosted Runner ebenfalls ("free for standard GitHub-hosted runners in public repositories, and for self-hosted runners") | https://docs.github.com/en/actions/concepts/billing-and-usage | [belegt] |
| Inklusivminuten je Monat für private Repositories: GitHub Free 2.000, GitHub Pro 3.000 | https://docs.github.com/en/billing/concepts/product-billing/github-actions | [belegt] |
| Preise je Minute: Linux 2 Kerne x64 0,006 USD, Linux 2 Kerne arm64 0,005 USD, Windows 2 Kerne 0,010 USD, macOS 0,062 USD; je Job auf volle Minuten aufgerundet | https://docs.github.com/en/billing/reference/actions-runner-pricing | [belegt] |
| Die aktuellen Seiten nennen keine Minuten-Multiplikatoren mehr; die frühere Seite zu Multiplikatoren zeigt nur noch die Preistabelle | gleiche Quelle | [belegt] |
| Preislich kostet eine Windows-Minute etwa das 1,7-fache, eine macOS-Minute etwa das 10-fache einer Linux-Minute | aus der Preistabelle gerechnet | [abgeleitet] |
| Ob Inklusivminuten für Windows und macOS preisgewichtet verbraucht werden | nicht belegt | offen |
| Larger Runner kosten immer, auch in öffentlichen Repositories | https://docs.github.com/en/billing/concepts/product-billing/github-actions | [belegt] |
| Die für 2026-03-01 angekündigte Gebühr für self-hosted Runner ist verschoben, ohne neues Datum | https://github.com/resources/insights/2026-pricing-changes-for-github-actions | [belegt] |

Folgerungen: Jeder kurze gehostete Job kostet mindestens eine volle Minute; viele kleine
Jobs sind teurer als ein großer. macOS ist der teuerste Runner. Self-hosted Runner sind
heute kostenlos, aber die verschobene Gebühr kann kommen; dann ändert sich diese Seite,
nicht die Architektur.

## Regeln

1. Pull-Request-Gates laufen im privaten Zustand auf dem self-hosted Runner (Weiche,
   ADR 0039). Kein `push`-Trigger auf Branches.
2. Gehostete Runner nur in der Release-Klasse (`release.yml`, `portability.yml`): Windows
   (keine eigene Hardware) und der Portabilitätstest auf frischer Linux-Maschine, nur bei
   Release-Tags. macOS-Builds laufen im privaten Zustand auf dem self-hosted Runner
   (eigene Hardware vorhanden), im öffentlichen auf gehosteten Runnern.
3. Öffentlich-Jobs (CodeQL, Scorecard, Pages) laufen im privaten Zustand gar nicht.
   Geplante Läufe brauchen zusätzlich `vars.BOTHY_VISIBILITY == 'public'`.
4. `timeout-minutes` an jedem Job; `concurrency` mit Abbruch überholter Läufe bei Pull
   Requests; Caches für uv, npm, Binärdateien und pre-commit.
5. Pfadfilter: Komponenten-Gates nur für betroffene Komponenten (`tools/ci/components.py`);
   repository-weite Gates immer. Kein Pfadfilter auf Workflow-Ebene, weil ein dadurch
   nicht gestarteter Workflow den Pflicht-Check `ci-ok` offen ließe.

`tools/ci/check_runners.py` (Gate `runners-check`) setzt die Regeln 1 bis 4 für jeden Job
durch.

## Erwartete gehostete Minuten im privaten Zustand

Nur bei Releases: der Windows-Build einer App-Version und der Portabilitätstest einer
Produkt- oder Server-Version. Alles andere ist ein Fehler der Workflows und wird wie ein
roter Test behandelt.

## Messung

* Monatlich die Nutzung prüfen: GitHub, Settings, Billing and licensing, Usage, Filter
  auf Actions und dieses Repository. Erwartung: nur Einträge der Release-Klasse.
* Taucht ein anderer Workflow mit gehosteten Minuten auf, ist der Job falsch
  konfiguriert: Ursache finden, `check_runners.py` um die fehlende Regel ergänzen, Test
  dazu.
* Ob sich die Nutzung über die REST-API abfragen lässt, ist für persönliche Konten nicht
  geprüft [offen]; bis dahin gilt die Ansicht in den Einstellungen.
