# Werkzeugkette

Jedes Werkzeug der Gates mit Version, Zweck, Lauf-Ort, Lizenz und Grund. Die Versionen
in dieser Seite hält ein Test gleich den Pins (`tools/docs/tests/test_toolchain_doc.py`):
`tools/uv.lock`, `tools/node/package-lock.json`, `tools/bin/tools.lock.json`, `Makefile`,
`tools/sonarqube/compose.yaml`, `.pre-commit-config.yaml`, die Workflows und die
`tool`-Direktiven der Go-Module unter `services/`.

Lauf-Ort: "lokal" heißt über `make` auf dem Rechner der Entwicklerin, "CI" heißt im
selben make-Ziel in GitHub Actions. Alle Gate-Werkzeuge laufen an beiden Orten.

## Gate-Werkzeuge

| Werkzeug | Version | Zweck | Lizenz | Warum dieses |
|---|---|---|---|---|
| `uv` | Systemwerkzeug | Python-Umgebung und Lock für alle Python-Werkzeuge | MIT OR Apache-2.0 | ein Lock mit Hashes für alle Python-Werkzeuge; lädt die Python-Version selbst |
| `pyyaml` | `6.0.3` | YAML lesen in den Prüfskripten | MIT | Standard für YAML in Python; `on:` wird bewusst nicht als Wahrheitswert gelesen |
| `ruff` | `0.17.0` | Format und Lint für Python | MIT | ersetzt mehrere Linter, schnell |
| `mypy` | `2.4.0` | Typprüfung (strict) | MIT | Referenz-Typprüfer für Python |
| `bandit` | `1.9.4` | Sicherheitsmuster in Python | Apache-2.0 | von der Anforderung gefordert; ergänzt ruff um Sicherheitsregeln |
| `pytest` | `9.1.1` | Tests der Werkzeuge | MIT | Standard |
| `pytest-cov` | `7.1.0` | Abdeckung in pytest | MIT | Standard |
| `coverage` | `7.16.2` | Abdeckungsschwelle | Apache-2.0 | Grundlage von pytest-cov |
| `pre-commit` | `4.6.2` | Hooks für commit, commit-msg, pre-push | MIT | verbreitet, Hooks als Datei im Repository |
| `pre-commit-hooks` | `v6.0.0` | Datei-Hygiene-Hooks | MIT | gepflegte Standard-Hooks, per Commit-SHA gepinnt |
| `check-jsonschema` | `0.38.2` | Schema-Prüfung von Issue-Formularen, Renovate- und CodeRabbit-Konfiguration | Apache-2.0 | bringt die Schemas der Formate mit |
| `zizmor` | `1.30.1` | Sicherheitsanalyse der Workflows | MIT | Pflicht laut Planungsgrundlage; prüft Injektion, Rechte, Pinning |
| `mkdocs` | `1.6.1` | Doku-Site | BSD | ADR 0045 |
| `mkdocs-material` | `9.7.7` | Theme der Doku-Site | MIT | ADR 0045 |
| `semgrep` | `1.180.0` | Projektregeln (`.semgrep/`) | LGPL-2.1 | Muster-Regeln für Projekt-Invarianten ohne eigenen Parser (R-QA-05) |
| `@mermaid-js/mermaid-cli` | `11.17.0` | Diagramme rendern | MIT | Referenz-Renderer von Mermaid; Major 11 wie bei der Planung |
| `puppeteer` | `25.13.0` | Browser-Steuerung für mermaid-cli | Apache-2.0 | Abhängigkeit von mermaid-cli; bestimmt die Browser-Version |
| `@redocly/cli` | `2.62.1` | Lint von OpenAPI und AsyncAPI | MIT | lintet beide Formate mit eigenen Regeln |
| `@sonar/scan` | `5.0.1` | SonarScanner ohne lokale Java-Installation | LGPL-3.0 | gleicher Scanner lokal und in CI, kein Docker für den Scan |
| `actionlint` | `1.7.12` | Syntax und Ausdrücke der Workflows | MIT | Pflicht laut Planungsgrundlage |
| `gitleaks` | `8.30.1` | Secrets in allen Commits | MIT | als CLI, nicht als Action (Lizenzpflicht der Action in Organisationen) |
| `golangci-lint` | `2.14.0` | Go-Linter-Bündel inklusive gosec und depguard | GPL-3.0 | Standard für Go; Abhängigkeitsregeln der Module (Kapitel 02) |
| `go-arch-lint` | `1.19.0` | Modulgrenzen der Go-Module aus dem Modulregister | MIT | in 02 "go-arch-lint" an einem Gerüst aller Module geprüft; prüft Pakete und Fremdbibliotheken |
| `sqlc` | `1.31.1` | typisierter Go-Code aus SQL, Drift-Prüfung | MIT | ADR 0007 |
| `oapi-codegen` | `v2.8.0` | Go-Server-Schnittstelle und Modelle aus `docs/api/openapi.yaml` (`tool` in `services/server/go.mod`, läuft in `make codegen-check`) | Apache-2.0 | ADR 0013; 06 "Codegenerierung" |
| `govulncheck` | `v1.8.0` | erreichbare Go-Schwachstellen | BSD-3-Clause | prüft Aufrufgraphen statt nur Versionen |
| `go-licenses` | `v2.0.1` | Lizenzen gelinkter Go-Pakete | Apache-2.0 | vom Recherche-Ruling gefordert |
| `lychee` | `0.24.2` | Links und Anker in Markdown | Apache-2.0 OR MIT | prüft Anker in Markdown-Dateien offline |
| `osv-scanner` | `2.6.0` | Schwachstellen und Lizenzen aller Lock-Dateien | Apache-2.0 | ein Werkzeug für Go, Dart, Python und npm |
| `trivy` | `0.69.3` | Fehlkonfiguration und Image-Schwachstellen | Apache-2.0 | Image-Scan beim Release; Version siehe unten |
| `otelcol-contrib` | `0.162.0` | `otelcol-contrib validate` für die Collector-Basis, jedes Overlay und Paare aus zwei Overlays (`ops-check`) | Apache-2.0 | dieselbe Distribution und Version wie das Collector-Image (`BOTHY_OTELCOL_VERSION`); nur sie kennt die contrib-Komponenten der Overlays |
| `promtool` | `3.15.0` | `promtool check config`, `check rules` und `test rules` (`ops-check`) | Apache-2.0 | Prüfwerkzeug aus dem Prometheus-Release der Image-Version (`BOTHY_PROMETHEUS_VERSION`) |
| `amtool` | `0.34.1` | `amtool check-config` und Routing-Tests (`ops-check`) | Apache-2.0 | Prüfwerkzeug aus dem Alertmanager-Release der Image-Version (`BOTHY_ALERTMANAGER_VERSION`) |
| `loki` | `3.7.8` | `loki -verify-config` (`ops-check`) | AGPL-3.0-only | nur Loki selbst kennt seine Konfigurationsschlüssel; Version des Images (`BOTHY_LOKI_VERSION`). Wird nur zur Prüfung ausgeführt und nicht verteilt; AGPL-3.0 greift bei Verteilung und bei Netznutzung veränderter Fassungen, beides geschieht hier nicht |
| `tempo` | `3.1.0` | `tempo -config.verify=true` (`ops-check`) | AGPL-3.0-only | nur Tempo selbst kennt seine Konfigurationsschlüssel; Version des Images (`BOTHY_TEMPO_VERSION`). Wird nur zur Prüfung ausgeführt und nicht verteilt; AGPL-3.0 greift bei Verteilung und bei Netznutzung veränderter Fassungen, beides geschieht hier nicht |
| `sloth` | `0.16.0` | SLO-Regeln neu erzeugen und mit den eingecheckten vergleichen (`ops-check`) | Apache-2.0 | Generator der Burn-Rate-Regeln (ADR 0035); dieselbe Version steht im Kopf der erzeugten Dateien |
| `tofu` | `1.13.1` | `tofu init` und `tofu validate` je Modul unter `infra/tofu/modules/` (`ops-check`) | MPL-2.0 | Werkzeug der Module (ADR 0012); prüft HCL, Typen und Validierungen ohne Zugang zu einem Provider |
| `sonarqube` | `26.9.0.129388-community` | lokaler SonarQube Community Build | LGPL-3.0 | ADR 0040 |
| `postgres` | `18.6-trixie` | Datenbank des lokalen SonarQube | PostgreSQL | vom Community Build für den Dauerbetrieb verlangt (H2 nur für Tests) |

Lizenzen: PyPI, npm und die LICENSE-Dateien der Projekte, abgerufen 2026-10-10 [belegt].
Entwicklungswerkzeuge werden nicht ausgeliefert; die Lizenzregeln für ausgelieferte
Abhängigkeiten stehen in [licenses.md](licenses.md).

## GitHub Actions

Alle per voller Commit-SHA gepinnt; der Kommentar nennt das Release-Tag. Die SHAs wurden
am 2026-10-10 mit `git ls-remote` im Original-Repository nachgeschlagen (bei annotierten
Tags die Commit-SHA hinter dem Tag).

| Action | Version | Wofür |
|---|---|---|
| `actions/checkout` | `v7.0.1` | Checkout ohne gespeicherte Zugangsdaten |
| `actions/setup-node` | `v7.1.0` | Node für die Node-Werkzeuge, mit npm-Cache |
| `actions/setup-go` | `v7.0.0` | Go aus `go.mod`, sobald es existiert |
| `astral-sh/setup-uv` | `v10.3.0` | uv, mit Cache über `tools/uv.lock` |
| `actions/cache` | `v6.1.0` | Binär-Cache und pre-commit-Umgebungen |
| `actions/upload-artifact` | `v7.0.2` | Release-Artefakte, Scorecard-Ergebnis |
| `actions/download-artifact` | `v8.0.2` | Release-Artefakte einsammeln |
| `actions/attest` | `v4.2.2` | Build-Provenance und SBOM-Attestation (öffentlich) |
| `anchore/sbom-action` | `v0.24.3` | SBOM (SPDX) der Release-Artefakte |
| `docker/setup-qemu-action` | `v4.4.0` | Multi-Arch-Build |
| `docker/setup-buildx-action` | `v4.4.1` | Buildx mit Provenance und SBOM; Tag erst nach dem Scan (`imagetools create`) |
| `docker/login-action` | `v4.6.0` | Anmeldung an ghcr.io |
| `docker/build-push-action` | `v7.4.0` | Server-Image bauen und pushen |
| `subosito/flutter-action` | `v2.23.0` | Flutter aus `pubspec.yaml`, sobald die App existiert |
| `github/codeql-action` | `v4.38.3` | CodeQL und SARIF-Upload (öffentlich) |
| `ossf/scorecard-action` | `v2.4.4` | OpenSSF Scorecard (öffentlich) |
| `actions/upload-pages-artifact` | `v5.0.0` | Doku-Site hochladen (öffentlich) |
| `actions/deploy-pages` | `v5.0.1` | Doku-Site veröffentlichen (öffentlich) |

## Updates

* Renovate schlägt Updates gruppiert und mit Wartezeit vor (ADR 0041). Ein Update ist erst
  fertig, wenn diese Seite die neue Version nennt; der Test erzwingt das.
* Release-Binärdateien: nach dem Renovate-Vorschlag `make tools-lock TOOL=<name>
  VERSION=<version>`; der Befehl übernimmt die sha256-Werte, die GitHub für die
  Release-Dateien meldet. Den Diff prüfen, dann `make bootstrap` und `make check`.
* Die Validatoren der Hilfsdienste (`otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo`)
  folgen den Image-Versionen in `infra/compose/`: Renovate erhöht die Images (Gruppe
  "Compose-Images", ADR 0041) und ist für diese fünf Pins abgeschaltet. Im Pull Request des
  Image-Updates zieht der Maintainer sie mit `make tools-lock` nach; bis dahin scheitert der
  Check `tool-versions` von `make ops-check`, weil ein Validator anderer Version andere
  Schlüssel annimmt oder ablehnt als die laufende Komponente.
* Neue Werkzeuge kommen mit Zeile in dieser Seite, Pin, Lizenzprüfung und Grund.

## Trivy: Version halten

Trivy bleibt auf `0.69.3`. Hintergrund: Am 2026-03-19 wurde mit gestohlenen Zugangsdaten
eine bösartige Trivy-Version 0.69.4 veröffentlicht, Tags von `trivy-action` und
`setup-trivy` wurden umgebogen; das Advisory nennt 0.69.3 als nicht betroffen, geschützt
durch unveränderliche Releases
(https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23,
abgerufen 2026-10-10) [belegt]. Renovate ist für Trivy abgeschaltet. Ein Wechsel auf eine
neuere Version braucht:

1. Das Release ist als unveränderlich markiert (GitHub-Release-API, Feld `immutable`).
2. Die Signatur der Prüfsummen-Datei ist mit cosign gegen die Workflow-Identität des
   Trivy-Repositorys geprüft.
3. Das Advisory oder ein späteres nennt keinen Vorfall für diese Version.
4. Ein Pull Request mit `make tools-lock TOOL=trivy VERSION=<neu>` und dem Prüfprotokoll
   in der Beschreibung.

Trivy wird nie über `trivy-action` oder `setup-trivy` eingebunden, sondern als gepinnte
Binärdatei über `tools/bin/fetch_tools.py`.

## Bekannte Grenzen

* actionlint 1.7.12 kennt die neue Schreibweise `uses: $/...` für Workflows im selben
  Repository noch nicht; zizmor empfiehlt sie (Audit `self-repository`). Bis actionlint
  sie versteht, bleibt `./` mit einer begründeten zizmor-Ausnahme in `release.yml`.
* zizmor prüft ohne `GH_TOKEN` nur offline; online-Audits (zum Beispiel unechte Commits
  hinter einer SHA) laufen in CI mit dem Workflow-Token.
* Für Windows-Rechner gibt es keine Binär-Pins; Beitragende nutzen WSL.
