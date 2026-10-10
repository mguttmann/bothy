# 18 Release

Status: Entwurf M0. Normativ für Versionierung, Tags, Changelogs, Release-Kanäle, Signatur,
SBOM, Provenance, Server-Images, App-Verteilung, Kompatibilität zwischen App und Server und
Rollback. Ablauf eines Releases: [docs/operations/release.md](../operations/release.md).
Gates und Runner: ADR 0038 und 0039 (Team H). API-Versionierung: ADR 0014 (Team B).

## 1. Ziele

1. Jede Komponente wird unabhängig versioniert und veröffentlicht, wenn sie sich ändert.
2. Jedes veröffentlichte Artefakt ist signiert, hat eine SBOM und eine prüfbare Herkunft.
3. Ein Betreiber erkennt am Release, ob ein Upgrade Handarbeit braucht.
4. Alte Apps funktionieren gegen neue Server, solange ihre Protokollversion unterstützt ist,
   und bekommen sonst einen klaren Hinweis statt eines Fehlers.

## 2. Komponenten

| Komponente | Tag-Präfix | Artefakte | Kanäle |
|---|---|---|---|
| `server` | `server/` | Images `bothy-server`, `bothy-postgres` (linux/amd64, linux/arm64), Binärdateien `bothy`, Compose-Bündel `infra/compose` | stable, beta |
| `apps` | `apps/` | iOS, Android, macOS, Windows, Web-Konsole (eine Flutter-Codebasis, B5) | stable, beta |
| `sdk-python` | `sdk-python/` | Paket auf PyPI | stable, Vorabversionen |
| `sdk-typescript` | `sdk-typescript/` | Paket auf npm | stable, Vorabversionen |
| `adapter-hermes` | `adapter-hermes/` | Plugin für Hermes Agent | stable, Vorabversionen |
| `adapter-code-runner` | `adapter-code-runner/` | Dev-Runner für Claude Code | stable, Vorabversionen |
| `api` | keiner | OpenAPI- und AsyncAPI-Dokument; Version im Dokument (ADR 0013, 0014) | mit `server` |

Regeln:

1. Jede Komponente hat in ihrem Verzeichnis eine Datei `VERSION` (eine Zeile, SemVer ohne `v`)
   und eine `CHANGELOG.md`. Die Verzeichnisse legt die Repo-Struktur fest (Team H).
2. Tags heißen `<präfix>v<SemVer>`, z. B. `server/v1.2.3`. Für Go-Module in Unterverzeichnissen
   ist genau diese Form vorgeschrieben (https://go.dev/ref/mod, Abschnitt "Module paths",
   abgerufen 2026-10-10) [belegt]. Tags sind signiert.
3. Ab Hauptversion 2 muss ein Go-Modulpfad auf `/v2` enden (gleiche Quelle) [belegt]. Ein
   Major-Sprung des Servers ist darum auch eine Änderung des Modulpfads.
4. Die Datei `VERSION` und der Tag stimmen überein; ein Gate-Check prüft es.
5. Der Server kennt seine Version aus `internal/platform/buildinfo` (`Version()`): eine Konstante
   gleich `services/server/VERSION`, gehalten von einem Unit-Test; keine Linker-Flags. Grund: Ein
   `go build` ohne Zusatz ergibt dieselbe Version wie der Release-Build. Sie dient
   `service.version` (17), dem Vergleich mit `instance.min_server_version` (16, Abschnitt 10) und
   den Image-Labels (Abschnitt 7).
6. Neben den eingebetteten Migrationen steht die Konstante `MinServerVersion`: die älteste
   Server-Version, die mit dem Schema dieser Migrationen laufen darf. Nach Regel M1 aus
   [upgrade.md](../operations/upgrade.md) ist das die vorige Version; ein bewusster Bruch setzt die
   eigene; vor dem ersten Release ist es die aktuelle. `bothy migrate up` schreibt sie nach
   `instance.min_server_version`.

## 3. Semantische Versionierung je Komponente

| Komponente | Major | Minor | Patch |
|---|---|---|---|
| `server` | Entfernen einer unterstützten API-Hauptversion oder Protokollversion; Entfernen oder Umdeuten eines Konfigurationsschlüssels; Upgrade, das Handarbeit an Daten verlangt | neue Funktionen, neue Schlüssel, neue Ereignistypen (additiv) | Fehlerbehebung ohne Verhaltensänderung |
| `apps` | Mindest-Serverversion steigt über eine Server-Hauptversion | neue Funktionen | Fehlerbehebung |
| SDKs, Adapter | inkompatible Änderung der öffentlichen Schnittstelle | additive Änderung | Fehlerbehebung |

Vor 1.0.0 gilt SemVer 0.x: Minor darf brechen; die Release Notes nennen jeden Bruch.

## 4. Changelog und Release Notes

1. `CHANGELOG.md` folgt "Keep a Changelog" mit den Abschnitten Added, Changed, Deprecated,
   Removed, Fixed, Security. Die Einträge sind Englisch (Community-Sprache).
2. Commits folgen Conventional Commits mit der Komponente als Scope. Ein Werkzeug erzeugt daraus
   einen Entwurf; ein Mensch oder Agent prüft ihn vor dem Tag. Werkzeugwahl: ADR-Vorschlag im
   Bericht.
3. Release Notes des Servers haben feste Abschnitte: Upgrade-Hinweise (Handarbeit ja oder nein),
   Migrationen (Expand oder Contract, Dauer auf großen Tabellen), Kompatibilität (unterstützte
   Protokollversionen, Mindest-App-Version), Sicherheit, bekannte Probleme.

## 5. Kanäle

| Artefakt | stable | beta |
|---|---|---|
| Server-Images | Tag `X.Y.Z` und Digest | Tag `X.Y.Z-beta.N` |
| iOS | App Store, sobald freigegeben; bis dahin TestFlight-Gruppe "stable" | TestFlight-Gruppe "beta" |
| macOS | Sparkle-Appcast ohne Kanal (Standard) | Sparkle-Kanal `beta` |
| Windows | `stable.appinstaller` | `beta.appinstaller` |
| Android | Testkanal "stable" (Owner-Entscheidung, Abschnitt 8) | interner Testkanal |
| SDKs, Adapter | Version ohne Vorabkennung | `X.Y.Z-beta.N` (npm-Tag `beta`, PyPI Vorabversion) |

Regeln:

1. Apps: Die Version enthält keinen Kanal. Derselbe Build (gleiche Build-Nummer, gleiche
   Signatur) wandert von beta nach stable: TestFlight-Gruppe wechseln, im Appcast das Element
   `sparkle:channel` entfernen, `stable.appinstaller` auf dasselbe MSIX zeigen lassen.
2. Server, SDKs, Adapter: Beta ist eine SemVer-Vorabversion (`X.Y.Z-beta.N`), weil die Version im
   Binary und im Paket steht. Stable entsteht aus demselben Commit wie die letzte Beta; zwischen
   beiden ändert sich nur die Versionsnummer, und alle Gates laufen erneut.
3. Images haben keinen beweglichen Tag `latest`. Compose und OpenTofu pinnen `BOTHY_VERSION`;
   Renovate (ADR 0041) schlägt Updates vor.
4. Sparkle-Kanäle sind für vorübergehende Zweige gedacht, nicht für dauerhaft parallele Ausgaben
   (https://sparkle-project.org/documentation/publishing/, abgerufen 2026-10-10) [belegt]. Das
   passt zu beta; eine zweite dauerhafte Ausgabe bräuchte einen eigenen Appcast.

## 6. Lieferkette

| Schritt | Werkzeug | Ergebnis |
|---|---|---|
| SBOM | Syft (über anchore/sbom-action) | SPDX-JSON und CycloneDX-JSON je Image und Binärdatei, am Release angehängt |
| Signatur Images | cosign keyless (Fulcio, Rekor), Identität = Release-Workflow | Signatur je Digest |
| Signatur Dateien | cosign `sign-blob` mit Bundle | `*.sigstore.json` neben jeder Datei |
| Herkunft | `actions/attest` (SLSA Build Provenance) | Attestation im Repository |
| Prüfsummen | SHA-256 | `SHA256SUMS`, selbst signiert |

Belege (abgerufen 2026-10-10): https://github.com/anchore/sbom-action,
https://docs.sigstore.dev/cosign/signing/overview/, https://docs.sigstore.dev/cosign/verifying/verify/,
https://github.com/actions/attest-build-provenance [belegt].

Grenze: Artifact Attestations sind in GitHub Free, Pro und Team nur für öffentliche Repositorys
verfügbar (https://github.com/actions/attest-build-provenance, abgerufen 2026-10-10) [belegt].
Bis das Produkt-Repository öffentlich ist, gibt es Signatur und SBOM, aber keine Attestation.
Der Betreiber prüft so:

```
cosign verify ghcr.io/<owner>/bothy-server@sha256:<digest> \
  --certificate-identity-regexp '^https://github.com/<owner>/bothy/.github/workflows/release-server.yaml@refs/tags/server/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
gh attestation verify oci://ghcr.io/<owner>/bothy-server@sha256:<digest> --repo <owner>/bothy
```

Der Issuer `https://token.actions.githubusercontent.com` ist nicht in den gelesenen Quellen
belegt [vermutet]; das Release-Ticket prüft ihn am ersten echten Release.

## 7. Server-Images

1. Plattformen linux/amd64 und linux/arm64, als ein Multi-Arch-Manifest (B12).
2. Nie per Emulation gebaut: `bothy-server` kompiliert das Go-Binary im Builder-Schritt auf der
   Build-Plattform für die Zielarchitektur (ohne cgo, `--platform=$BUILDPLATFORM`); Images mit
   Schritten, die auf der Zielarchitektur laufen müssen (`bothy-postgres`), entstehen je
   Architektur auf eigenem Runner.
3. `bothy-server` ist ein statisches Go-Binary (ohne cgo) auf einem minimalen Basis-Image ohne
   Shell. Es läuft als unprivilegierter Benutzer mit uid und gid `65532`; `bothy secrets init`
   setzt diesen Besitzer für die Geheimnisse von `server` und `migrate` (16, Abschnitt 5.3), und
   das Image legt `secrets.signing_key_dir` mit diesem Besitzer und Modus `0700` an.
   `HEALTHCHECK` und Compose rufen `bothy healthcheck` auf, weil keine Shell und kein curl im Image
   sind.
4. `bothy-postgres` baut auf dem offiziellen `postgres:18`-Image auf und enthält pgBackRest ab
   2.59.3 und `bothy-backupd` ([16-betrieb.md](16-betrieb.md) 8).
5. Basis-Images sind per Digest gepinnt; Renovate hält sie aktuell; ein Check im Gate lehnt jedes
   `FROM` ohne Digest ab (SEC-SC-4).
6. Labels nach OCI (`org.opencontainers.image.source`, `.version`, `.revision`, `.licenses`);
   Version, Revision und Quelle kommen als Build-Argumente, ein Build ohne Version scheitert.

## 8. App-Verteilung

Die Apps sind White Label. Wer eine App signiert, besitzt ihre Identität (Bundle-ID,
Paketname, Push-Schlüssel, Update-Feed). Darum gilt:

1. Das Produkt liefert Quelltext, Build-Skripte und eine Vorlage `distribution.yaml` mit allen
   Werten eines Herausgebers: Bundle- und Paket-IDs, Team-ID, Sparkle-Feed-URL und öffentlicher
   EdDSA-Schlüssel, MSIX-Publisher, Firebase-Projekt, APNs-Topic, optionales Sentry-Ziel.
2. Jede Organisation baut und signiert ihre Apps selbst mit ihren Konten. Für die Firmeninstanz
   liegt `distribution.yaml` im Ops-Repo.
3. Push-Schlüssel und Firebase-Projekt der Instanz gehören zum Herausgeber der App (R-Push).
   Eine Instanz kann nur Push an Apps senden, deren Schlüssel sie hat.

| Plattform | Weg | Belege (abgerufen 2026-10-10) |
|---|---|---|
| iOS | TestFlight: bis 100 interne und 10.000 externe Tester, Build 90 Tage nutzbar, externe Tester brauchen für den ersten Build ein Beta App Review | https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview/ [belegt] |
| macOS | Developer ID, Notarisierung mit `xcrun notarytool`, Stapeln mit `xcrun stapler`, Auto-Update mit Sparkle 2 (EdDSA-signierte Archive, Kanäle, gestufte Verteilung mit `sparkle:phasedRolloutInterval`) | https://developer.apple.com/developer-id/, https://sparkle-project.org/documentation/publishing/ [belegt] |
| Windows | MSIX mit `.appinstaller` je Kanal; Signatur über Azure Artifact Signing (Organisation in der EU möglich) oder OV-Zertifikat mit HSM. SmartScreen warnt anfangs auch bei EV | https://learn.microsoft.com/en-us/windows/msix/app-installer/app-installer-file-overview, https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart, https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation [belegt] |
| Android | Play Internal Testing (bis 100 Tester) oder Firebase App Distribution; neue persönliche Play-Konten brauchen vor Produktion einen geschlossenen Test mit 12 Testern über 14 Tage | https://support.google.com/googleplay/android-developer/answer/9845334, https://support.google.com/googleplay/android-developer/answer/14151465, https://firebase.google.com/docs/app-distribution [belegt] |
| Web-Konsole | Vorschlag: Teil des Server-Images, ausgeliefert vom Server; damit ist sie immer gleich alt wie die API | entscheidet ADR 0028 (Team E) |

Folgen:

1. TestFlight für macOS verlangt eine App Sandbox [vermutet, nur über Websuche gelesen]. Die
   Developer-ID-Ausgabe mit Sparkle läuft ohne Sandbox. Beide wären zwei Builds; das Produkt
   liefert für macOS nur die Developer-ID-Ausgabe.
2. Das Protokoll `ms-appinstaller:` ist seit Dezember 2023 standardmäßig aus; Nutzer laden die
   `.appinstaller`-Datei und öffnen sie (Beleg wie oben) [belegt].
3. Kosten und Konten (Apple Developer Program, Artifact Signing oder OV-Zertifikat, Play-Konto)
   entscheidet der Owner (Bericht).

## 9. Kompatibilität zwischen App und Server

1. Die App meldet in `identify` ihre Protokollversion und Plattform-Version
   ([04-sync-protokoll.md](04-sync-protokoll.md) 6.3). Der Server unterstützt die Protokollversionen
   aus `hello.protocol_versions`. Ältere schließt er mit `4007 protocol_version_unsupported`; die
   App zeigt dann den Hinweis "Update nötig" statt eines Fehlers.
2. `instance.discovery.read` liefert je Plattform `client.min_version.<plattform>` und
   `client.recommended_version.<plattform>`. Unter der Mindestversion verbindet die App nicht und
   zeigt den Update-Weg ihres Kanals. Unter der empfohlenen Version zeigt sie einen Hinweis.
3. Wie lange eine Protokoll- oder API-Version nach ihrer Ablösung unterstützt bleibt, legt die
   Deprecation-Policy fest (ADR 0014, Team B). Release Notes nennen das Datum.
4. Der Server ist die führende Seite: Ein Server-Upgrade setzt die Mindestversion nie höher als
   die älteste App, die laut Deprecation-Policy noch unterstützt wird.
5. Adapter melden ihre Version im Heartbeat (`reported_runtime_version`); die Admin-Konsole zeigt
   Adapter, deren SDK-Version nicht mehr unterstützt ist.

## 10. Rollback

| Artefakt | Weg | Grenze |
|---|---|---|
| Server | vorheriges Image per Digest starten | genau eine Version zurück (Expand/Contract, [16-betrieb.md](16-betrieb.md) 10); weiter zurück nur per Wiederherstellung |
| iOS | Build in TestFlight ablaufen lassen, korrigierten Build mit höherer Nummer verteilen | kein Downgrade auf Geräten |
| macOS | neue Version mit höherer Nummer im Appcast; bei Gefahr als kritisches Update markieren | Sparkle installiert keine ältere Version |
| Windows | neues MSIX mit höherer Version in der `.appinstaller`-Datei | kein automatisches Downgrade |
| Android | neuer Build mit höherem `versionCode` | kein Downgrade |
| SDKs | Version zurückziehen (PyPI yank, npm deprecate), Korrektur veröffentlichen | installierte Kopien bleiben |

Apps werden also nie zurückgerollt, sondern vorwärts korrigiert. Darum gehen App-Builds erst in
beta und dann mit derselben Build-Nummer in stable.

## 11. Sicherheits-Releases

1. Meldungen kommen über den Weg aus `SECURITY.md` (Team H).
2. Korrektur auf einem privaten Zweig, Release mit Abschnitt Security, Hinweis an Betreiber vor
   der öffentlichen Beschreibung. Die Frist bis zur Veröffentlichung steht in `SECURITY.md`.
3. Betroffene Mindestversionen von Apps hebt der Betreiber mit `client.min_version.<plattform>`.

## 12. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `client.min_version.ios` | leer | kleinste zugelassene App-Version auf iOS; leer heißt keine Grenze |
| `client.min_version.android` | leer | dasselbe für Android |
| `client.min_version.macos` | leer | dasselbe für macOS |
| `client.min_version.windows` | leer | dasselbe für Windows |
| `client.recommended_version.ios` | leer | darunter zeigt die App einen Update-Hinweis |
| `client.recommended_version.android` | leer | dasselbe für Android |
| `client.recommended_version.macos` | leer | dasselbe für macOS |
| `client.recommended_version.windows` | leer | dasselbe für Windows |

## 13. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-RL-01 | Version und Changelog je Komponente, Tags `<komponente>/vX.Y.Z`. | Unabhängige Releases; Go verlangt diese Tag-Form für Module in Unterverzeichnissen. |
| R-RL-02 | Kein Tag `latest`; Compose und OpenTofu pinnen Versionen. | Wiederholbare Installation; ein Upgrade ist eine bewusste Änderung. |
| R-RL-03 | Apps: derselbe Build wandert von beta nach stable. Server, SDKs, Adapter: stable aus demselben Commit wie die letzte Beta, nur die Versionsnummer ändert sich. | Was getestet wurde, wird ausgeliefert; wo die Version im Artefakt steht, ist ein Neubau unvermeidbar und auf die Nummer begrenzt. |
| R-RL-04 | Alle Artefakte signiert (cosign keyless) mit SBOM; Attestation sobald das Repository öffentlich ist. | Prüfbare Herkunft ohne Schlüsselverwaltung; Attestation hängt an der Sichtbarkeit. |
| R-RL-05 | Apps sind White Label; jede Organisation baut und signiert selbst mit `distribution.yaml`. | Push-Schlüssel, Bundle-IDs und Update-Feeds gehören dem Herausgeber; keine zentrale Push-Weiterleitung nötig. |
| R-RL-06 | macOS nur als Developer-ID-Ausgabe mit Sparkle, keine zweite TestFlight-Ausgabe. | Eine Ausgabe, kein Sandbox-Zwang, eigener Update-Kanal. |
| R-RL-07 | Apps werden vorwärts korrigiert, nie zurückgerollt. | Keine Plattform installiert ältere Versionen zuverlässig. |
| R-RL-08 | Mindest- und empfohlene App-Version je Plattform als Server-Konfiguration. | Der Betreiber steuert Sicherheitsupdates ohne Release des Servers. |
