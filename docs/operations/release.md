# Ein Release machen

Status: Entwurf M0. Ablauf zu [18-release.md](../spec/18-release.md). Gates und Runner legen
ADR 0038 und 0039 fest (Team H); dieser Ablauf nennt nur die Schritte, die zum Release gehören.

![Release-Kette](diagrams/release-pipeline.svg)

## 1. Voraussetzungen

1. Hauptzweig grün in allen lokalen Gates und in CI.
2. Portabilitätstest der letzten Beta grün ([16-betrieb.md](../spec/16-betrieb.md) 11.1).
3. `CHANGELOG.md` der Komponente hat einen geprüften Abschnitt für die neue Version; Release Notes
   mit den Pflichtabschnitten ([18-release.md](../spec/18-release.md) 4).
4. Für `server`: Migrationen erfüllen M1 bis M12 ([upgrade.md](upgrade.md) 4); der
   N-1-Kompatibilitätstest ist grün.
5. Mindest-App-Versionen und Deprecation-Daten in den Release Notes stimmen mit ADR 0014.

## 2. Server

1. Once before the first tag of the private phase (owner step): create the key pair of the
   private phase (section 2.2), commit its public key as
   `quality/release/cosign-private-phase.pub`, and store the private key and its password as the
   secrets `COSIGN_PRIVATE_KEY` and `COSIGN_PASSWORD` of the environment `release`.
2. `make release-prep COMPONENT=server [CHANNEL=beta]` writes `services/server/VERSION`, renames
   the section "Unreleased" of `services/server/CHANGELOG.md` to `[X.Y.Z] - <date>` and prints
   the commits without a changelog entry as a checklist
   ([ADR 0060](../adr/0060-release-automatisierung-im-monorepo.md)); review the section, add the
   mandatory parts of the release notes ([18-release.md](../spec/18-release.md) 4), commit
   `chore(release): server vX.Y.Z` and land it through a pull request.
3. `make release-trial COMPONENT=server` on the build host (section 2.1) with the release
   commit; it must pass.
4. Signed tag on the squash commit on `main`: `git tag -s server/vX.Y.Z <commit>` (beta:
   `server/vX.Y.Z-beta.N`), then push the tag.
5. `release.yml` resolves the tag (VERSION and changelog section), runs the gates, builds the
   binaries and both images by digest, scans the tree with ScanCode, joins, scans, writes the
   SBOMs, signs, verifies and only then sets the version tags; `publish` creates the release
   with binaries, SBOMs, signature bundles, `images.txt`, `SHA256SUMS` and the ScanCode report
   ([18-release.md](../spec/18-release.md) 6.1).
6. Check like an instance operator (section 5).
7. Beta: to `staging`; stable: mark the release as "latest release", inform the operators.

### 2.1 Trial release

```
make release-trial COMPONENT=server [TRIAL_PLATFORMS=linux/amd64,linux/arm64]
```

Runs on the build host with Docker and buildx, never under GitHub Actions (the target refuses
when `GITHUB_ACTIONS` is set). It starts a registry container pinned by digest on `127.0.0.1`
with a random port, builds the binaries and both images for both platforms (no emulation: both
Dockerfiles build the target platform on the build platform), pushes them by digest, joins
`bothy-postgres`, generates a cosign key pair for this run only, writes SBOMs, signs with the
signing configuration of the private phase, writes and signs `SHA256SUMS`, verifies with the
commands of section 5 (private phase), sets the version tags, checks the images that
`infra/compose/compose.yaml` pulls, and runs three probes that must fail (changed file, unsigned
digest, missing SBOM). It prints the digests and `SHA256SUMS`. The registry, the buildx builder,
the key pair and the temporary directory are removed at the end, also after a failure. It needs
no account and costs nothing ([18-release.md](../spec/18-release.md) 6.2).

### 2.2 Key pair of the private phase (owner)

```
COSIGN_PASSWORD='<new password>' cosign generate-key-pair
mv cosign.pub quality/release/cosign-private-phase.pub
```

with the pinned cosign (`uv run --project tools --locked python tools/bin/fetch_tools.py path
cosign`). Store the content of `cosign.key` as the secret `COSIGN_PRIVATE_KEY` and the password
as `COSIGN_PASSWORD` of the environment `release`, then delete `cosign.key` locally. A release
without the public key file stops at the verification, before any tag.

## 3. SDKs und Adapter

1. `make release-prep COMPONENT=<komponente>`, Commit, Tag `<komponente>/vX.Y.Z` (Namen aus
   `tools/ci/components.py`).
2. Build, SBOM, Signatur der Dateien (`cosign sign-blob`), Veröffentlichung auf PyPI bzw. npm über
   Trusted Publishing des Registers, wo vorhanden [vermutet: Verfügbarkeit je Register im Ticket
   prüfen].
3. Beta: Vorabversion, npm-Tag `beta`.

## 4. Apps

Apps baut der Herausgeber mit seiner `distribution.yaml` ([18-release.md](../spec/18-release.md) 8).

| Plattform | Schritte |
|---|---|
| iOS | Build mit Build-Nummer, Upload nach App Store Connect, TestFlight-Gruppe beta; nach Bewährung Gruppe stable bzw. Einreichung |
| macOS | Build, Signatur mit Developer ID, `xcrun notarytool submit --wait`, `xcrun stapler staple`, Archiv mit `sign_update` (EdDSA) signieren, Eintrag im Appcast mit `sparkle:channel` beta; nach Bewährung Element entfernen |
| Windows | MSIX bauen und signieren (Artifact Signing oder OV mit HSM), `beta.appinstaller` auf die neue Version; nach Bewährung `stable.appinstaller` |
| Android | AAB bauen und signieren, interner Testkanal; nach Bewährung nächster Kanal |
| Web-Konsole | mit dem Server-Release (Vorschlag, ADR 0028) |

Vor dem Wechsel nach stable: Absturzrate des Beta-Builds prüfen (falls ein Ziel für
Absturzberichte konfiguriert ist) und keine offenen Fehler der Stufe kritisch.

## 5. Prüfen wie ein Betreiber

Download the release assets of the tag into one directory (`gh release download server/vX.Y.Z`),
take the digests from `images.txt`, and run the commands of the current phase. Every command
must succeed; `sha256sum --check` lists every file with `OK`.

Private phase ([ADR 0062](../adr/0062-release-signaturen-solange-das-repo-privat-ist.md),
cosign 3.1.3, key pair without a transparency log, so `--insecure-ignore-tlog`):

```
cosign verify --key quality/release/cosign-private-phase.pub --insecure-ignore-tlog \
  ghcr.io/<owner>/bothy-server@sha256:<digest>
cosign verify --key quality/release/cosign-private-phase.pub --insecure-ignore-tlog \
  ghcr.io/<owner>/bothy-postgres@sha256:<digest>
cosign verify-blob --key quality/release/cosign-private-phase.pub --insecure-ignore-tlog \
  --bundle SHA256SUMS.sigstore.json SHA256SUMS
cosign verify-blob --key quality/release/cosign-private-phase.pub --insecure-ignore-tlog \
  --bundle bothy_X.Y.Z_linux_amd64.sigstore.json bothy_X.Y.Z_linux_amd64
sha256sum --check --strict SHA256SUMS
```

Public phase (keyless with the identity of the release workflow, plus attestations):

```
cosign verify ghcr.io/<owner>/bothy-server@sha256:<digest> \
  --certificate-identity-regexp '^https://github\.com/<owner>/bothy/\.github/workflows/release\.yml@refs/tags/server/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
gh attestation verify oci://ghcr.io/<owner>/bothy-server@sha256:<digest> --repo <owner>/bothy
cosign verify-blob SHA256SUMS --bundle SHA256SUMS.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/<owner>/bothy/\.github/workflows/release\.yml@refs/tags/server/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --check --strict SHA256SUMS
```

The same for `bothy-postgres` and each binary. The public path and the attestations run first
with the owner's first public release; the trial release proves the private path only. The
release workflow is `.github/workflows/release.yml`; the commands of both phases are those of
`tools/ci/release_chain.py verify`.

## 6. Rückruf

1. Server: Release als zurückgezogen markieren, Hinweis an Betreiber mit Rückweg
   ([upgrade.md](upgrade.md) 3), korrigierte Patch-Version.
2. Apps: korrigierten Build mit höherer Nummer; bei Sicherheitsfehlern `client.min_version.<plattform>`
   auf die korrigierte Version setzen lassen.
3. SDKs: Version zurückziehen (PyPI yank, npm deprecate), Korrektur veröffentlichen.
