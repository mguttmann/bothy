# Lizenzen

Das Projekt steht unter Apache-2.0 mit DCO (ADR 0003). Ausgelieferte Teile (Server,
Images, Apps, SDKs, Adapter) dürfen nur Abhängigkeiten mit vereinbarer Lizenz enthalten.
Entwicklungswerkzeuge unter `tools/` werden nicht ausgeliefert; für sie gilt die Liste
nicht (ihre Lizenzen stehen in toolchain.md).

## Erlaubte Lizenzen

Quelle der Wahrheit ist `quality/licenses/allowed.txt`; ein Test hält diese Tabelle
gleich.

| SPDX | Art |
|---|---|
| `0BSD` | erlaubend |
| `Apache-2.0` | erlaubend, mit Patentlizenz |
| `BSD-2-Clause` | erlaubend |
| `BSD-3-Clause` | erlaubend |
| `BSL-1.0` | erlaubend |
| `CC0-1.0` | gemeinfrei-ähnlich |
| `ISC` | erlaubend |
| `MIT` | erlaubend |
| `MIT-0` | erlaubend |
| `MPL-2.0` | schwaches Copyleft je Datei; nur unverändert nutzen |
| `PSF-2.0` | erlaubend |
| `Python-2.0` | erlaubend |
| `Unicode-3.0` | erlaubend (Unicode-Daten) |
| `Unicode-DFS-2016` | erlaubend (Unicode-Daten) |
| `Zlib` | erlaubend |

Nicht erlaubt ohne ADR: GPL, AGPL, LGPL in statisch gelinkten Teilen (Go-Server, Apps),
SSPL, BUSL, Lizenzen mit Nutzungsbeschränkung (zum Beispiel Commons Clause), proprietäre
und unbekannte Lizenzen. Ein Dienst unter einer solchen Lizenz darf als getrennter Prozess
angebunden werden, wenn das Kapitel des Dienstes das begründet (zum Beispiel der
Virenscanner in Kapitel 12).

MPL-2.0 ist erlaubt, weil Pflichten nur geänderte Dateien der Bibliothek betreffen; die
Planungsgrundlage nennt River (MPL-2.0) für Hintergrundjobs.

## Lizenz-Gate (`make licenses`)

1. `LICENSE` ist der unveränderte Apache-2.0-Text (sha256 gegen
   https://www.apache.org/licenses/LICENSE-2.0.txt, abgerufen 2026-10-10) [belegt].
2. Jede handgeschriebene Quelldatei trägt `SPDX-License-Identifier: Apache-2.0` in den
   ersten `license.spdx_header_lines` Zeilen; generierter Code ist ausgenommen
   (`tools/ci/license_check.py`).
3. Abhängigkeiten ausgelieferter Komponenten: OSV-Scanner mit `--licenses` und der Liste
   oben über alle Lock-Dateien der Komponente (Go, Dart, Python, npm); OSV-Scanner bezieht
   Lizenzdaten über deps.dev (https://github.com/google/osv-scanner, abgerufen 2026-10-10)
   [belegt]. Für Go zusätzlich `go-licenses check` über die tatsächlich gelinkten Pakete.
   Meldet OSV-Scanner eine Lizenz als `UNKNOWN` oder `non-standard` (deps.dev kennt den
   Lizenztext nicht als SPDX-Ausdruck), wird die Lizenz in der LICENSE-Datei des Pakets
   geprüft und in einer `osv-scanner.toml` der Komponente als `PackageOverrides` mit Grund
   festgehalten; nie pauschal für alle Pakete.
4. Vor jedem Release und vor dem Öffentlich-Schalten: ScanCode über den ganzen Baum
   (Lizenztexte und Copyright-Angaben in Dateien), Ergebnis im Release-Protokoll. ScanCode
   ist kein Pull-Request-Gate, weil ein Lauf über den ganzen Baum lange dauert; der
   Ablauf ist ein Ticket der Release-Etappe.

## SBOM und Prüfung von Releases

* Jedes Release enthält `sbom.spdx.json` (SPDX, erzeugt mit Syft über
  `anchore/sbom-action`) und `SHA256SUMS`.
* Server-Images tragen Provenance und SBOM als Attestation im Registry-Eintrag
  (Buildx, `provenance: mode=max`, `sbom: true`).
* Im öffentlichen Zustand zusätzlich signierte Build-Provenance und SBOM-Attestation über
  GitHub Artifact Attestations; Prüfung mit `gh attestation verify <datei> --repo
  <owner>/<repo>` (https://github.com/actions/attest-build-provenance, abgerufen
  2026-10-10) [belegt]. Im privaten Zustand gibt es sie nicht: auf GitHub Pro sind
  Attestations nur für öffentliche Repositories verfügbar (gleiche Quelle) [belegt].
