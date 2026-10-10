---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Dokumentations-Site-Generator

## Context and Problem Statement

Die Spezifikation, die ADRs und die Anleitungen sind Markdown im Repository. Sie sollen
als Site lesbar sein, mit Suche, Navigation und Mermaid-Diagrammen, und der Build soll
als Gate fehlerhafte Verweise finden. Material for MkDocs ist seit 2025-11-05 im
Wartungsmodus; die Herstellerin arbeitet an Zensical
(https://squidfunk.github.io/mkdocs-material/blog/2025/11/05/zensical/, abgerufen
2026-10-10) [belegt]. Material 9.7.5 begrenzt MkDocs auf Versionen vor 2.0
(https://squidfunk.github.io/mkdocs-material/changelog/, abgerufen 2026-10-10) [belegt].

## Decision Drivers

* Strikter Build als Gate: fehlende Seiten und kaputte Links schlagen fehl.
* Mermaid, Suche, Hell- und Dunkel-Modus ohne eigene Entwicklung.
* Python-Werkzeugkette wie die übrigen Repository-Werkzeuge (ein uv-Lock).
* Geplanter, risikoarmer Wechsel, wenn der Nachfolger reif ist.

## Considered Options

* MkDocs 1.6 mit Material 9.7, gepinnt, Wechsel zu Zensical als geplanter Schritt
* Zensical sofort
* Docusaurus
* Hugo
* Nur die Markdown-Darstellung von GitHub

## Decision Outcome

Gewählt: "MkDocs mit Material, gepinnt", weil es heute stabil ist, `--strict` Verweise
prüft und Zensical `mkdocs.yml` nativ liest (https://zensical.org/docs/get-started/,
abgerufen 2026-10-10) [belegt], der Wechsel also ohne Umbau der Inhalte möglich bleibt.
Versionen stehen in `tools/uv.lock`; Renovate schlägt Updates vor. Der Wechsel zu Zensical
wird ein eigener Pull Request, sobald Zensical 1.0 erschienen ist und die genutzten
Funktionen (Suche, Mermaid, Navigation, Hell/Dunkel) im Build dieses Repositorys
nachgewiesen sind.

Weitere Festlegungen: Systemschriften statt Webfonts (keine Anfrage an Fremd-Server beim
Lesen), Anker im Stil von GitHub (`pymdownx.slugs.slugify`), damit Links auf GitHub und in
der Site gleich funktionieren; die Site wird nur im öffentlichen Zustand veröffentlicht
(`docs.yml`). Die API-Verträge (`docs/api/`) liegen als Dateien in der Site; eine
eingebettete Redoc- oder Swagger-Oberfläche gibt es in M0 nicht: beide sind
JavaScript-Anwendungen, die eingebettet entweder ein CDN brauchen oder als zusätzliche
Assets gepflegt werden müssten, und die API ändert sich in der Planung noch oft. Eine
statisch erzeugte Referenz (`redocly build-docs`, in der gepinnten Redocly CLI vorhanden)
ist ein eigenes Ticket, sobald die API stabil ist.

### Consequences

* Gut, weil `make docs` heute ein striktes Gate ist.
* Schlecht, weil Material nach dem Ende der Sicherheitszusage keine Korrekturen mehr
  bekommt; der Wechsel ist eingeplant.
* Schlecht, weil Mermaid in der Site per JavaScript aus einem CDN geladen wird; das betrifft
  nur das Lesen der Site, nicht den Build.

### Confirmation

`make docs` (Teil von `make check`) baut mit `mkdocs build --strict`; die Navigation in
`mkdocs.yml` verweist auf jede Seite, die es geben muss, und eine fehlende Seite lässt den
Build scheitern. `make linkcheck` prüft zusätzlich Links und Anker aller Markdown-Dateien
außerhalb von `docs/`.

## Pros and Cons of the Options

### MkDocs mit Material, gepinnt

* Gut, weil ausgereift und mit striktem Modus.
* Schlecht, weil im Wartungsmodus.

### Zensical sofort

* Gut, weil der designierte Nachfolger.
* Schlecht, weil Version 0.x; die Website und PyPI nannten am Abruftag verschiedene
  Versionen (PyPI 0.0.69) [belegt, https://pypi.org/project/zensical/, abgerufen 2026-10-10].

### Docusaurus

* Gut, weil verbreitet und aktiv.
* Schlecht, weil eine zweite Werkzeugkette (React, Node) nur für Doku.

### Hugo

* Gut, weil ein Binärprogramm und schnell.
* Schlecht, weil Mermaid, Suche und strikte Linkprüfung Zusatzarbeit wären.

### Nur GitHub-Darstellung

* Gut, weil nichts zu bauen.
* Schlecht, weil keine Suche, keine Navigation und kein Gate für kaputte Verweise.

## More Information

Prüfung des Wechsels: Ticket im Plan; Grundlage Planungsgrundlage R-Doku-Site.
