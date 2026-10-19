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

Die Namen, Verzeichnisse und Tag-Präfixe stehen an einer Stelle: `RELEASE_COMPONENTS` in
`tools/ci/components.py` ([ADR 0060](../adr/0060-release-automatisierung-im-monorepo.md)). Diese Tabelle gibt sie wieder.

| Komponente | Verzeichnis | Tag-Präfix | Artefakte | Kanäle |
|---|---|---|---|---|
| `bothy` | root | none (`vX.Y.Z`) | installation bundle from `make release-build`: `infra/compose` without `infra/compose/test` (test overlays), `LICENSE`, `NOTICE`, `CHANGELOG.md`, `VERSION`; its `infra/compose/.env.example` contains the pinned server version and the registry (section 2.1); in addition `install.sh` as a separate file with a built-in version, SHA-256 and download address of the bundle (section 2.1, [16-betrieb.md](16-betrieb.md), section 3.1) | stable, beta |
| `server` | `services/server` | `server/` | Images `bothy-server`, `bothy-postgres` (linux/amd64, linux/arm64), Binärdateien `bothy` | stable, beta |
| `host` | `services/host` | `host/` | Binärdateien `bothy-host` für macOS (arm64, amd64), Linux (amd64, arm64) und Windows (amd64), je mit Signatur und SBOM; Installationspakete der Dienste ([24-terminal.md](24-terminal.md), [16-betrieb.md](16-betrieb.md) Abschnitt 4.4) | stable, beta |
| `client` | `apps/client` | `client/` | iOS, Android, macOS, Windows (eine Flutter-Codebasis, B5) | stable, beta |
| `console` | `apps/console` | `console/` | Web-Konsole | mit `server` |
| `sdk-python` | `sdk/python` | `sdk-python/` | Paket auf PyPI | stable, Vorabversionen |
| `sdk-typescript` | `sdk/typescript` | `sdk-typescript/` | Paket auf npm | stable, Vorabversionen |
| `hermes-adapter` | `plugins/hermes-adapter` | `hermes-adapter/` | Plugin für Hermes Agent | stable, Vorabversionen |
| `claude-code-runner` | `plugins/claude-code-runner` | `claude-code-runner/` | Dev-Runner für Claude Code | stable, Vorabversionen |
| `grafana-bothy-live` | `plugins/grafana-bothy-live` | `grafana-bothy-live/` | Grafana-Panel-Plugin `bothy-live-panel` als ZIP mit SBOM; bis V1 unsigniert für lokales Provisioning ([25-visualisierung.md](25-visualisierung.md), Abschnitt 8.3) | stable, Vorabversionen |
| `api` | `docs/api` | keiner | OpenAPI- und AsyncAPI-Dokument; Version im Dokument (ADR 0013, 0014) | mit `server` |

Regeln:

1. Jede Komponente hat in ihrem Verzeichnis eine Datei `VERSION` (eine Zeile, SemVer ohne `v`)
   und eine `CHANGELOG.md`. Die Verzeichnisse legt die Repo-Struktur fest (Team H).
2. Tags are named `<prefix>v<SemVer>`, for example `server/v1.2.3`; the prefix is a rule of this
   repository. The Go modules reference states two rules without a condition: a module in a
   subdirectory of its repository has tags prefixed with that subdirectory ("each tag name must
   be prefixed with the module subdirectory, followed by a slash", here
   `services/server/v1.2.3`), and from major version 2 on "the module path must end with a
   major version suffix like `/v2`". It names their purpose in the same section: "If a module
   might be depended on by other modules, these rules must be followed so that the go command
   can find and download the module", and "A module that will never be fetched as a dependency
   of any other module may use any valid package path for its module path"
   (https://go.dev/ref/mod, sections "Module paths" and "Mapping versions to commits", retrieved
   2026-10-15) [verified]. The server module `github.com/OWNER/bothy/services/server` is a main
   module that no other module requires (PLAN P-05): the go command never maps one of its
   versions to a tag, and the version of the binary comes from `VERSION` (rule 5), not from a
   tag. So neither rule takes effect, and `server/vX.Y.Z` stays (docs/DECISIONS.md D-M7-10-4).
   Tags are signed.
3. Soll das Server-Modul je als Abhängigkeit nutzbar werden, ist das eine eigene Entscheidung
   (Tag-Form und `/v2`-Pfad nach der Go-Regel).
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

### 2.1 Server version and registry in the product bundle

The bundle of the component `bothy` determines which server images an installation pulls:
`install.sh` takes `BOTHY_VERSION` and `BOTHY_REGISTRY` unchanged from its
`infra/compose/.env.example` ([16-betrieb.md](16-betrieb.md), section 3.1, step 4). Compose
pulls the images by their tag with this version, not by digest; what follows from that is in
[16-betrieb.md](16-betrieb.md), section 6.1, rule 2. The checked-in template carries
placeholders (`ghcr.io/OWNER`, `0.0.0-placeholder`), and so does the source of `install.sh`.
Therefore, for every product release, beta as well as stable:

1. **Server version.** Its only source is the line `BOTHY_VERSION=` in
   `infra/compose/.env.example` of the tagged commit.
   `make release-prep COMPONENT=bothy SERVER_VERSION=<version>` writes it in the same commit as
   `VERSION` (ADR 0060). It writes nothing and names the value if the tag
   `server/v<version>` does not exist, if without `CHANNEL=beta` (stable product) `<version>`
   carries a pre-release identifier, or if `SERVER_VERSION` is missing while the line carries the
   placeholder. The gate `release-pins` (`tools/ci/components.py pins`) checks the committed state
   in every `make check` and in CI: exactly one line, SemVer without `v` or the placeholder, and
   no server pre-release in a stable product, which only a product beta (`X.Y.Z-beta.N`) may
   pin. Before the tag it runs in the pre-push of the
   branch with the release commit and in the required check `ci-ok` of its pull request; the
   maintainer sets the tag only after the merge, on the squash commit on `main`
   ([push-discipline.md](../quality/push-discipline.md)). The push of the tag alone starts no
   check: its commit is already on `origin` then, and pre-commit runs no pre-push hook for a push
   without new commits (function `_pre_push_ns` in
   https://github.com/pre-commit/pre-commit/blob/v4.6.2/pre_commit/commands/hook_impl.py,
   retrieved 2026-10-11) [verified]. After the push of the tag, the job `gates` of `release.yml`
   checks again, before the build. `make release-build` takes the value over unchanged.
2. **Registry.** `make release-build` writes `BOTHY_REGISTRY=ghcr.io/<owner>` into the
   `.env.example` of the bundle, with `<owner>` in lowercase from `GITHUB_REPOSITORY_OWNER`,
   like the image name in `release.yml`. GitHub sets this variable in every step to "The
   repository owner's name"
   (https://docs.github.com/en/actions/reference/workflows-and-actions/variables, retrieved
   2026-10-11) [verified]; a run outside GitHub Actions sets it itself. Every other
   file and line of the bundle is the same as in the commit.
3. **Download address.** `make release-build` writes `install.sh` as an artifact of its own from
   `infra/compose/install.sh` and sets three values: the version of the product, the SHA-256 of
   the bundle and the download address
   `https://github.com/<repository>/releases/download/v<version>/`, with `<repository>` from
   `GITHUB_REPOSITORY`. GitHub sets this variable in every step to "The owner and repository
   name" (source as in rule 2) [verified]; a run outside GitHub Actions sets it itself. From this
   address `install.sh` downloads the bundle and its signature
   ([16-betrieb.md](16-betrieb.md), section 3.1, step 2).
4. **Abort.** `make release-build` writes no bundle and no `install.sh` and names the value if
   `BOTHY_VERSION` equals `0.0.0-placeholder` or is not a SemVer version without `v`, if a
   stable product (`VERSION` without a pre-release identifier) pins a server pre-release (second
   safeguard after `release-prep` and `release-pins`), if `GITHUB_REPOSITORY_OWNER` or
   `GITHUB_REPOSITORY` is missing, if `BOTHY_REGISTRY` or `install.sh` still contains `OWNER` or a
   placeholder of the source afterwards or if
   one of the images that `infra/compose/compose.yaml` names with `${BOTHY_VERSION}` (today
   `bothy-server:<version>` and `bothy-postgres:pg18-<version>`) is missing under `BOTHY_REGISTRY`.
   It reads the list of images from `compose.yaml`, not from a list of its own.

Reason: without this step, every installation from the bundle would pull an image that does not
exist (`ghcr.io/OWNER/bothy-server:0.0.0-placeholder`), including the portability test
([16-betrieb.md](16-betrieb.md), section 11.1). The server version is in the commit instead of
being an argument of the build: this way the tag alone determines what the bundle installs, and the
release commit shows the version for review. A server pre-release in the stable product shows up
before the tag already (`release-prep`, at the latest `release-pins` in the pull request), because
a pushed tag cannot be taken back. Registry and download address come from the build because they
depend on the location of the repository (fork, replacing `OWNER` in V1-03), not on the commit:
even a tag whose source still names `OWNER` delivers an `install.sh` that finds its bundle. PLAN M7-29
takes on the build (with tests), M7-10 takes on `make release-prep` itself.

## 3. Semantische Versionierung je Komponente

| Komponente | Major | Minor | Patch |
|---|---|---|---|
| `server` | Entfernen einer unterstützten API-Hauptversion oder Protokollversion; Entfernen oder Umdeuten eines Konfigurationsschlüssels; Upgrade, das Handarbeit an Daten verlangt | neue Funktionen, neue Schlüssel, neue Ereignistypen (additiv) | Fehlerbehebung ohne Verhaltensänderung |
| `client`, `console` | Mindest-Serverversion steigt über eine Server-Hauptversion | neue Funktionen | Fehlerbehebung |
| SDKs, Adapter | inkompatible Änderung der öffentlichen Schnittstelle | additive Änderung | Fehlerbehebung |

Vor 1.0.0 gilt SemVer 0.x: Minor darf brechen; die Release Notes nennen jeden Bruch.

## 4. Changelog und Release Notes

1. `CHANGELOG.md` folgt "Keep a Changelog" mit den Abschnitten Added, Changed, Deprecated,
   Removed, Fixed, Security. Die Einträge sind Englisch (Community-Sprache).
2. Commits folgen Conventional Commits mit der Komponente als Scope. Ein Werkzeug erzeugt daraus
   einen Entwurf; ein Mensch oder Agent prüft ihn vor dem Tag. Werkzeug: git-cliff hinter
   `make release-prep` ([ADR 0060](../adr/0060-release-automatisierung-im-monorepo.md)).
3. Release Notes des Servers haben feste Abschnitte: Upgrade-Hinweise (Handarbeit ja oder nein),
   Migrationen (Expand oder Contract, Dauer auf großen Tabellen), Kompatibilität (unterstützte
   Protokollversionen, Mindest-App-Version), Sicherheit, bekannte Probleme.

## 5. Kanäle

| Artefakt | stable | beta |
|---|---|---|
| Server-Images | Tag `X.Y.Z` und Digest | Tag `X.Y.Z-beta.N` |
| Host-Dienst `bothy-host` | Version `X.Y.Z` | `X.Y.Z-beta.N` |
| iOS | App Store, sobald freigegeben; bis dahin TestFlight-Gruppe "stable" | TestFlight-Gruppe "beta" |
| macOS | Sparkle-Appcast ohne Kanal (Standard) | Sparkle-Kanal `beta` |
| Windows | `stable.appinstaller` | `beta.appinstaller` |
| Android | Testkanal "stable" (Owner-Entscheidung, Abschnitt 8) | interner Testkanal |
| SDKs, Adapter | Version ohne Vorabkennung | `X.Y.Z-beta.N` (npm-Tag `beta`, PyPI Vorabversion) |

Regeln:

1. Apps: Die Version enthält keinen Kanal. Derselbe Build (gleiche Build-Nummer, gleiche
   Signatur) wandert von beta nach stable: TestFlight-Gruppe wechseln, im Appcast das Element
   `sparkle:channel` entfernen, `stable.appinstaller` auf dasselbe MSIX zeigen lassen.
2. Server, Host-Dienst, SDKs, Adapter: Beta ist eine SemVer-Vorabversion (`X.Y.Z-beta.N`), weil die Version im
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
| SBOM | Syft (server: the pinned binary through `tools/ci/release_chain.py sbom`; other components: anchore/sbom-action until their release tickets) | SPDX JSON and CycloneDX JSON per image and platform and per binary, attached to the release |
| Signatur Images | öffentlich: cosign keyless (Fulcio, Rekor), Identität = Release-Workflow; privat: cosign-Schlüsselpaar ohne Transparenz-Log ([ADR 0062](../adr/0062-release-signaturen-solange-das-repo-privat-ist.md)) | Signatur je Digest |
| Signatur Dateien | cosign `sign-blob` mit Bundle, Verfahren wie bei den Images | `*.sigstore.json` neben jeder Datei |
| Herkunft | `actions/attest` (SLSA Build Provenance) | Attestation im Repository |
| Prüfsummen | SHA-256 | `SHA256SUMS`, selbst signiert |

Belege (abgerufen 2026-10-10): https://github.com/anchore/sbom-action,
https://docs.sigstore.dev/cosign/signing/overview/, https://docs.sigstore.dev/cosign/verifying/verify/,
https://github.com/actions/attest-build-provenance [belegt].

Grenze: Artifact Attestations sind in GitHub Free, Pro und Team nur für öffentliche Repositorys
verfügbar (https://github.com/actions/attest-build-provenance, abgerufen 2026-10-10) [belegt].
Bis das Produkt-Repository öffentlich ist, gibt es Signatur und SBOM, aber keine Attestation,
und signiert wird mit dem Schlüsselpaar der privaten Phase ([ADR 0062](../adr/0062-release-signaturen-solange-das-repo-privat-ist.md)); geprüft wird dann mit
`cosign verify --key` und dem öffentlichen Schlüssel aus dem Repository. Im öffentlichen Zustand
prüft der Betreiber so:

```
cosign verify ghcr.io/<owner>/bothy-server@sha256:<digest> \
  --certificate-identity-regexp '^https://github.com/<owner>/bothy/.github/workflows/release.yml@refs/tags/server/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
gh attestation verify oci://ghcr.io/<owner>/bothy-server@sha256:<digest> --repo <owner>/bothy
```

Der Issuer `https://token.actions.githubusercontent.com` ist nicht in den gelesenen Quellen
belegt [vermutet]; das Release-Ticket prüft ihn am ersten echten Release.

### 6.1 Server release chain (as built by M7-10)

`release.yml` runs these steps for a tag `server/vX.Y.Z`, in this order; the steps after the
image builds are commands of `tools/ci/release_chain.py`, which `make release-trial` runs the
same way against a local registry (section 6.2):

1. `resolve`: `tools/ci/components.py release` rejects the tag if `services/server/VERSION`
   differs from it or `services/server/CHANGELOG.md` has no section `[X.Y.Z]`; nothing is built
   then. The full gate (`ci.yml`) follows.
2. `build`: the binaries `bothy_<version>_linux_amd64` and `..._linux_arm64` from
   `make -C services/server release-build`, and `bothy-server` for both platforms in one build
   (it cross compiles), pushed by digest without a tag. `postgres-build`: `bothy-postgres` per
   platform on a native runner (`ubuntu-24.04`, `ubuntu-24.04-arm`), pushed by digest.
   `scancode`: ScanCode over the tracked tree with the license gate (`make scancode`,
   [licenses.md](../quality/licenses.md) section 4); its report is a release artifact.
3. `supply-chain` (environment `release`, after `scancode`): join the two `bothy-postgres`
   digests into one multi-arch index (by digest, no tag); vulnerability scan of both images
   (`make trivy-image`); SBOMs; signatures of both image digests, both binaries and
   `SHA256SUMS` (computed over every file of the release except the signature bundles), with a
   `.sigstore.json` bundle each; a missing SBOM stops this step before the first signature. In
   the private phase every bundle is checked to carry no transparency log entry. Then the
   verification of section 6 or 6.2 against the result, and only then the version tags
   `bothy-server:X.Y.Z` and `bothy-postgres:pg18-X.Y.Z`.
4. `publish`: the GitHub release with the binaries, SBOMs, signature bundles, `images.txt`
   (name and digest per image), `SHA256SUMS` and the ScanCode report; Build Provenance only in
   the public phase.

Signing paths ([ADR 0062](../adr/0062-release-signaturen-solange-das-repo-privat-ist.md)): in
the private phase cosign signs with the key pair whose private key and password are the
secrets `COSIGN_PRIVATE_KEY` and `COSIGN_PASSWORD` of the environment `release`, and with a
signing configuration from `cosign signing-config create --no-default-rekor
--no-default-fulcio --no-default-tsa --no-default-oidc`; the public key is
`quality/release/cosign-private-phase.pub`. In the public phase (`github.event.repository.visibility`)
cosign signs keyless with the workflow identity, and `actions/attest` adds Build Provenance per
image digest. `tools/ci/check_runners.py` (rule `signing-outside-release`) rejects a signing
step in a job without `environment: release`.

### 6.2 Trial release

`make release-trial COMPONENT=server` (docs/DECISIONS.md D-W-127, D-M7-10-1) runs the steps of
6.1 on the build host against throwaway local targets: a registry container pinned by digest
on `127.0.0.1` with a random port, a cosign key pair generated for the run, the signing
configuration of the private phase, SBOMs, `SHA256SUMS` and its signature in a temporary
directory, then `cosign verify --key`, `cosign verify-blob` and `sha256sum --check`, the
version tags, and a check that every image `infra/compose/compose.yaml` names with
`${BOTHY_VERSION}` exists under that version with the signed digest. Three probes must fail: a
changed file against `SHA256SUMS`, an unsigned digest against `cosign verify`, a release without
one SBOM against the signing step. It never pushes to another registry, never tags the
repository or creates a release, and refuses to run when `GITHUB_ACTIONS` is set. Vulnerability
scan and ScanCode are separate targets and not part of the trial. Not proven by the trial:
the keyless path and the attestations of the public phase, and a push to `ghcr.io`; both run
first with the owner's first tag.

## 7. Server-Images

1. Plattformen linux/amd64 und linux/arm64, als ein Multi-Arch-Manifest (B12).
2. Nie per Emulation gebaut: `bothy-server` kompiliert das Go-Binary im Builder-Schritt auf der
   Build-Plattform für die Zielarchitektur (ohne cgo, `--platform=$BUILDPLATFORM`); Images mit
   Schritten, die auf der Zielarchitektur laufen müssen (`bothy-postgres`), entstehen je
   Architektur auf eigenem Runner. In `release.yml` `bothy-postgres` is built per architecture
   on a native runner and pushed by digest; the release joins both digests into one multi-arch
   index (section 6.1). Images are pushed by digest only; the version tag comes last, after the
   vulnerability scan, ScanCode, SBOMs, signatures and verification.
3. `bothy-server` ist ein statisches Go-Binary (ohne cgo) auf einem minimalen Basis-Image ohne
   Shell. Es läuft als unprivilegierter Benutzer mit uid und gid `65532`; `bothy secrets init`
   setzt diesen Besitzer für die Geheimnisse von `server` und `migrate` (16, Abschnitt 5.3), und
   das Image legt `secrets.signing_key_dir` mit diesem Besitzer und Modus `0700` an.
   `HEALTHCHECK` und Compose rufen `bothy healthcheck` auf, weil keine Shell und kein curl im Image
   sind.
4. `bothy-postgres` baut auf dem offiziellen `postgres:18`-Image auf und enthält pgBackRest ab
   2.59.3 und `bothy-backupd` ([16-betrieb.md](16-betrieb.md) 8). Its tag is `pg18-<version>`
   with the server version, as `infra/compose/compose.yaml` names it; the tag of `bothy-server`
   is `<version>`.
5. Basis-Images sind per Digest gepinnt; Renovate hält sie aktuell; ein Check im Gate lehnt jedes
   `FROM` ohne Digest ab (SEC-SC-4).
6. Labels nach OCI (`org.opencontainers.image.source`, `.version`, `.revision`, `.licenses`);
   Version, Revision und Quelle kommen als Build-Argumente, ein Build ohne Version scheitert.
7. `bothy-server` is built from `services/server/Dockerfile` with the repository root as build
   context (the same for `make image` and `release.yml`). The file
   `services/server/Dockerfile.dockerignore` applies to this Dockerfile only: it ignores
   everything and allows only `go.mod`, `go.sum`, `VERSION`, `api/`, `cmd/`, `internal/` and
   `migrations/` of the server, without test files and `testdata/`. A build whose `VERSION`
   differs from `services/server/VERSION` fails. For the web console (M6-02) the context also
   admits the root `pubspec.yaml`, `pubspec.lock` and `analysis_options.yaml`, every member
   `pubspec.yaml` of the pub workspace (the root resolution needs them all), `apps/console`
   without `test/`, `build/` and `coverage/`, the `lib/` of `api_client`, `auth`,
   `design_system`, `domain` and `l10n`, the `assets/` and `fonts/` of `design_system` and
   `docs/api/openapi.yaml` (for `version.json`). The console stage runs on `$BUILDPLATFORM`
   (rule 2), because its output is platform independent; it builds with the Flutter version of
   the workspace (`pubspec.yaml`), cloned at its tag with the revision checked, on the pinned
   builder image of the server (rule 5), and the final stage copies `build/web` to
   `/usr/share/bothy/console`, which `BOTHY_CONSOLE__ASSETS_DIR` of the image names.

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

### 9.1 Host-Dienst und Server

Der Host-Dienst meldet seine Version in `identify` der Steuerverbindung. Der Server nimmt die
Versionen an, deren Protokollversion er unterstützt (Prolog `bothy-terminal/1` und
`x-protocol-versions` des Kanals in [asyncapi.yaml](../api/asyncapi.yaml)); eine zu alte schließt er
mit `4114`, und Konsole und Rechner-Karte zeigen "Host-Dienst aktualisieren". Der Host-Dienst
aktualisiert sich nicht selbst. Grund: Ein Programm, das sich selbst ersetzt, bräuchte
Schreibrechte auf sein eigenes Binary; der Dienstbenutzer hat sie bewusst nicht
([24-terminal.md](24-terminal.md), Abschnitt 2.2). Updates laufen über den Paketweg des Betreibers.

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
| R-RL-01 | Version und Changelog je Komponente, Tags `<komponente>/vX.Y.Z` (Produktbündel `vX.Y.Z`). | Unabhängige Releases; eine Quelle für Namen und Präfixe in `tools/ci/components.py`; die Go-Tag-Regel gilt für das nie geladene Hauptmodul nicht (P-05). |
| R-RL-02 | Kein Tag `latest`; Compose und OpenTofu pinnen Versionen. | Wiederholbare Installation; ein Upgrade ist eine bewusste Änderung. |
| R-RL-03 | Apps: derselbe Build wandert von beta nach stable. Server, SDKs, Adapter: stable aus demselben Commit wie die letzte Beta, nur die Versionsnummer ändert sich. | Was getestet wurde, wird ausgeliefert; wo die Version im Artefakt steht, ist ein Neubau unvermeidbar und auf die Nummer begrenzt. |
| R-RL-04 | Alle Artefakte signiert mit SBOM: im öffentlichen Zustand cosign keyless mit Attestation, im privaten mit einem Schlüsselpaar ohne Transparenz-Log ([ADR 0062](../adr/0062-release-signaturen-solange-das-repo-privat-ist.md)). | Prüfbare Herkunft ohne Schlüsselverwaltung; Attestation hängt an der Sichtbarkeit. |
| R-RL-05 | Apps sind White Label; jede Organisation baut und signiert selbst mit `distribution.yaml`. | Push-Schlüssel, Bundle-IDs und Update-Feeds gehören dem Herausgeber; keine zentrale Push-Weiterleitung nötig. |
| R-RL-06 | macOS nur als Developer-ID-Ausgabe mit Sparkle, keine zweite TestFlight-Ausgabe. | Eine Ausgabe, kein Sandbox-Zwang, eigener Update-Kanal. |
| R-RL-07 | Apps werden vorwärts korrigiert, nie zurückgerollt. | Keine Plattform installiert ältere Versionen zuverlässig. |
| R-RL-08 | Mindest- und empfohlene App-Version je Plattform als Server-Konfiguration. | Der Betreiber steuert Sicherheitsupdates ohne Release des Servers. |
| R-RL-09 | The product bundle pins the server version in the tagged commit (`BOTHY_VERSION` in `infra/compose/.env.example`, written by `make release-prep COMPONENT=bothy SERVER_VERSION=<version>`, which rejects a server pre-release without `CHANNEL=beta`; gate `release-pins` in the pull request before the tag); `make release-build` sets `BOTHY_REGISTRY` from the owner and the download address in `install.sh` from the name of the repository and aborts on placeholders, on a server pre-release in the stable product and on missing images (2.1). | `install.sh` takes over both values unchanged; without a producer, every installation would pull the placeholder image. Version in the commit so that the tag alone determines the bundle, checked before the tag because a pushed tag stays; registry and download address from the build because they depend on the location of the repository. |
