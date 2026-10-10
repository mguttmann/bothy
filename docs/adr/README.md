# Architekturentscheidungen (ADR)

Tragende Entscheidungen stehen hier als Architecture Decision Records im Format MADR 4
([ADR 0001](0001-adrs-im-madr-format-decisions-als-ruling-log.md)). Neue ADR: Vorlage
[0000](0000-vorlage-madr-4.md) kopieren, nächste freie Nummer, dann `make adr-index`.
Kleinere Festlegungen stehen im Ruling-Log [DECISIONS.md](../DECISIONS.md).

Die Tabelle unten erzeugt `tools/docs/adr_index.py` aus den Dateien; `make docs` schlägt
fehl, wenn sie veraltet ist. Nicht von Hand bearbeiten.

<!-- adr-index:start -->
| Nr | Titel | Status | Datum |
|---|---|---|---|
| [0000](0000-vorlage-madr-4.md) | Vorlage (MADR 4) | template | 2026-10-10 |
| [0001](0001-adrs-im-madr-format-decisions-als-ruling-log.md) | ADRs im MADR-Format, DECISIONS.md als Ruling-Log | accepted | 2026-10-10 |
| [0002](0002-zwei-repos-oeffentliches-produkt-privates-ops-repo.md) | Zwei Repos: öffentliches Produkt, privates Ops-Repo | accepted | 2026-10-10 |
| [0003](0003-lizenz-apache-2-0-mit-dco-statt-cla.md) | Lizenz Apache-2.0 mit DCO statt CLA | accepted | 2026-10-10 |
| [0038](0038-lokale-gates-zuerst-ein-befehl-identisch-zu-ci.md) | Lokale Gates zuerst: ein Befehl identisch zu CI | accepted | 2026-10-10 |
| [0039](0039-runner-strategie-privat-oeffentlich-und-release-ausnahmen.md) | Runner-Strategie privat/öffentlich und Release-Ausnahmen | accepted | 2026-10-10 |
| [0040](0040-sonarqube-lokal-und-cloud-mit-gleichem-profil-und-gate.md) | SonarQube lokal und Cloud mit gleichem Profil und Gate | accepted | 2026-10-10 |
| [0041](0041-abhaengigkeits-updates-mit-renovate.md) | Abhängigkeits-Updates mit Renovate | accepted | 2026-10-10 |
| [0042](0042-deny-list-scanner-mit-liste-aus-dem-ops-repo.md) | Deny-List-Scanner mit Liste aus dem Ops-Repo | accepted | 2026-10-10 |
| [0045](0045-dokumentations-site-generator.md) | Dokumentations-Site-Generator | accepted | 2026-10-10 |
<!-- adr-index:end -->
