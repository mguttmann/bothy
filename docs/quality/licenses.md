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
   (`tools/ci/license_check.py`). Eine Go-Datei, die nach ihrem Namen als erzeugt gilt
   (`*.gen.go`, `*_gen.go`, `*.pb.go`), trägt vor der `package`-Zeile die Markierung
   `// Code generated ... DO NOT EDIT.`; sonst ist sie ein Befund, damit eine handgeschriebene
   Datei nicht allein über ihren Namen aus SPDX-Prüfung, Abdeckung und Sonar fällt.
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
   ist kein Pull-Request-Gate, weil ein Lauf über den ganzen Baum lange dauert. As built
   (M7-10): `make scancode` scans the tracked tree (`git archive HEAD`) with ScanCode Toolkit
   32.5.0 in a container built from `tools/scancode/Dockerfile` (base image pinned by digest,
   release archive by sha256; linux/amd64 only, emulated on arm64 hosts), writes
   `scancode.json` and runs `tools/ci/scancode_policy.py`: every file whose detected license
   expression the list above does not satisfy (`OR` needs one allowed side, `AND` both,
   `WITH` its license; `LicenseRef-*` never) and every file ScanCode could not scan is a
   finding, except paths listed with a reason in `quality/licenses/scancode-exceptions.txt`
   (text that names or quotes a license without licensing the file). In `release.yml` the job
   `scancode` runs it for every server tag; a finding stops the release before any image tag,
   and the report is attached to the release.

## Bundled fonts

The apps bundle the fonts Atkinson Hyperlegible Next and Atkinson Hyperlegible Mono as asset
files under `packages/design_system/fonts/` (docs/design/typography.md). Their license is
OFL-1.1, settled by D-L11 and docs/design/typography.md (reason 5): the OFL allows embedding in
apps and redistribution when the license text ships with the files. The font files are bundled
assets, not package dependencies: `make licenses` (OSV-Scanner over the lock files) does not see
them, and `quality/licenses/allowed.txt` stays unchanged. Instead the OFL text of each family
lies next to the files (`fonts/<family>.OFL.txt`), `fonts/MANIFEST.json` records
`"license": "OFL-1.1"` per file, and `packages/design_system/test/fonts_test.dart` fails when a
font file has no license text, a manifest entry names another license, or a file differs from
its recorded SHA-256. `NOTICE` names both fonts with their copyright (docs/DECISIONS.md
D-M4-04-4).

## Bundled native code

The apps load SQLite3MultipleCiphers as their encrypted SQLite (docs/spec/15-client-architektur.md
6, `packages/local_store`). The build hook of the Dart package `sqlite3` (3.7.1, MIT, a lock file
package that `make licenses` checks) downloads the prebuilt library of its release and verifies it
against the sha256 hashes in its sources; the root `pubspec.yaml` selects it with
`hooks: user_defines: sqlite3: source: sqlite3mc`. SQLite3MultipleCiphers 2.6.1 is under the MIT
license (`LICENSE` of its repository, Ulrich Telle); the SQLite 3.54.0 it contains is in the
public domain (https://sqlite.org/copyright.html). Both are bundled native code, not packages of
the lock file: OSV-Scanner does not see them and `quality/licenses/allowed.txt` stays unchanged;
MIT is on that list (docs/DECISIONS.md D-M4-12-9).

## SBOM und Prüfung von Releases

* Jedes Release enthält `sbom.spdx.json` (SPDX, erzeugt mit Syft über
  `anchore/sbom-action`) und `SHA256SUMS`. Server releases (M7-10) instead carry SPDX and
  CycloneDX SBOMs per image platform and per binary from the pinned Syft, signature bundles and
  a signed `SHA256SUMS` ([18-release.md](../spec/18-release.md) 6.1).
* Server-Images tragen Provenance und SBOM als Attestation im Registry-Eintrag
  (Buildx, `provenance: mode=max`, `sbom: true`).
* Im öffentlichen Zustand zusätzlich signierte Build-Provenance und SBOM-Attestation über
  GitHub Artifact Attestations; Prüfung mit `gh attestation verify <datei> --repo
  <owner>/<repo>` (https://github.com/actions/attest-build-provenance, abgerufen
  2026-10-10) [belegt]. Im privaten Zustand gibt es sie nicht: auf GitHub Pro sind
  Attestations nur für öffentliche Repositories verfügbar (gleiche Quelle) [belegt].
