# 18 Release

Status: draft M0. Normative for versioning, tags, changelogs, release channels, signing,
SBOM, provenance, server images, app distribution, compatibility between app and server and
rollback. Flow of a release: [docs/operations/release.md](../operations/release.md).
Gates and runners: ADR 0038 and 0039 (team H). API versioning: ADR 0014 (team B).

## 1. Goals

1. Every component is versioned and published independently when it changes.
2. Every published artifact is signed, has an SBOM and a verifiable provenance.
3. An instance operator can tell from the release whether an upgrade needs manual work.
4. Old apps work against new servers as long as their protocol version is supported,
   and otherwise get a clear notice instead of an error.

## 2. Components

The names, directories and tag prefixes are in one place: `RELEASE_COMPONENTS` in
`tools/ci/components.py` ([ADR 0060](../adr/0060-release-automation-in-the-monorepo.md)). This table reproduces them.

| Component | Directory | Tag prefix | Artifacts | Channels |
|---|---|---|---|---|
| `bothy` | root | none (`vX.Y.Z`) | installation bundle from `make release-build`: `infra/compose` without `infra/compose/test` (test overlays), `LICENSE`, `NOTICE`, `CHANGELOG.md`, `VERSION`; its `infra/compose/.env.example` contains the pinned server version and the registry (section 2.1); in addition `install.sh` as a separate file with a built-in version, SHA-256 and download address of the bundle (section 2.1, [16-operations.md](16-operations.md), section 3.1) | stable, beta |
| `server` | `services/server` | `server/` | images `bothy-server`, `bothy-postgres` (linux/amd64, linux/arm64), binaries `bothy` | stable, beta |
| `host` | `services/host` | `host/` | binaries `bothy-host` for macOS (arm64, amd64), Linux (amd64, arm64) and Windows (amd64), each with a signature and an SBOM; installation packages of the services ([24-terminal.md](24-terminal.md), [16-operations.md](16-operations.md) section 4.4) | stable, beta |
| `client` | `apps/client` | `client/` | iOS, Android, macOS, Windows (one Flutter code base, B5) | stable, beta |
| `console` | `apps/console` | `console/` | web console | with `server` |
| `sdk-python` | `sdk/python` | `sdk-python/` | package on PyPI | stable, pre-releases |
| `sdk-typescript` | `sdk/typescript` | `sdk-typescript/` | package on npm | stable, pre-releases |
| `hermes-adapter` | `plugins/hermes-adapter` | `hermes-adapter/` | plugin for Hermes Agent | stable, pre-releases |
| `claude-code-runner` | `plugins/claude-code-runner` | `claude-code-runner/` | dev runner for Claude Code | stable, pre-releases |
| `grafana-bothy-live` | `plugins/grafana-bothy-live` | `grafana-bothy-live/` | Grafana panel plugin `bothy-live-panel` as a ZIP with an SBOM; unsigned until V1 for local provisioning ([25-visualization.md](25-visualization.md), section 8.3) | stable, pre-releases |
| `api` | `docs/api` | none | OpenAPI and AsyncAPI document; version in the document (ADR 0013, 0014) | with `server` |

Rules:

1. Every component has a file `VERSION` (one line, SemVer without `v`) and a `CHANGELOG.md` in
   its directory. The repo structure defines the directories (team H).
2. Tags are named `<prefix>v<SemVer>`, for example `server/v1.2.3`; the prefix is a rule of this
   repository. For a module in a subdirectory, Go requires the directory as the prefix
   (`services/server/v1.2.3`) and, from major version 2 on, a module path ending in `/v2`, only if
   the module is loaded as a dependency (https://go.dev/ref/mod, section "Module paths", retrieved
   2026-10-10) [verified]. The server module is a main module that is never loaded (PLAN P-05);
   therefore neither applies to it. Tags are signed.
3. Should the server module ever become usable as a dependency, that is a separate decision
   (tag form and `/v2` path according to the Go rule).
4. The file `VERSION` and the tag match; a gate check verifies this.
5. The server knows its version from `internal/platform/buildinfo` (`Version()`): a constant
   equal to `services/server/VERSION`, held by a unit test; no linker flags. Reason: a
   `go build` without additions yields the same version as the release build. It serves
   `service.version` (17), the comparison with `instance.min_server_version` (16, section 10) and
   the image labels (section 7).
6. Next to the embedded migrations is the constant `MinServerVersion`: the oldest
   server version that may run with the schema of these migrations. According to rule M1 from
   [upgrade.md](../operations/upgrade.md) this is the previous version; a deliberate break sets its
   own version; before the first release it is the current one. `bothy migrate up` writes it to
   `instance.min_server_version`.

### 2.1 Server version and registry in the product bundle

The bundle of the component `bothy` determines which server images an installation pulls:
`install.sh` takes `BOTHY_VERSION` and `BOTHY_REGISTRY` unchanged from its
`infra/compose/.env.example` ([16-operations.md](16-operations.md), section 3.1, step 4). Compose
pulls the images by their tag with this version, not by digest; what follows from that is in
[16-operations.md](16-operations.md), section 6.1, rule 2. The checked-in template carries
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
   ([16-operations.md](16-operations.md), section 3.1, step 2).
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
([16-operations.md](16-operations.md), section 11.1). The server version is in the commit instead of
being an argument of the build: this way the tag alone determines what the bundle installs, and the
release commit shows the version for review. A server pre-release in the stable product shows up
before the tag already (`release-prep`, at the latest `release-pins` in the pull request), because
a pushed tag cannot be taken back. Registry and download address come from the build because they
depend on the location of the repository (fork, replacing `OWNER` in V1-03), not on the commit:
even a tag whose source still names `OWNER` delivers an `install.sh` that finds its bundle. PLAN M7-29
takes on the build (with tests), M7-10 takes on `make release-prep` itself.

## 3. Semantic versioning per component

| Component | Major | Minor | Patch |
|---|---|---|---|
| `server` | removal of a supported API major version or protocol version; removal or reinterpretation of a configuration key; an upgrade that requires manual work on data | new features, new keys, new event types (additive) | bug fix without a change in behavior |
| `client`, `console` | the minimum server version rises across a server major version | new features | bug fix |
| SDKs, adapters | incompatible change of the public interface | additive change | bug fix |

Before 1.0.0, SemVer 0.x applies: a minor version may break; the release notes name every break.

## 4. Changelog and release notes

1. `CHANGELOG.md` follows "Keep a Changelog" with the sections Added, Changed, Deprecated,
   Removed, Fixed, Security. The entries are in English (the community language).
2. Commits follow Conventional Commits with the component as the scope. A tool generates a draft
   from them; a person or an agent reviews it before the tag. Tool: git-cliff behind
   `make release-prep` ([ADR 0060](../adr/0060-release-automation-in-the-monorepo.md)).
3. Release notes of the server have fixed sections: upgrade notes (manual work yes or no),
   migrations (expand or contract, duration on large tables), compatibility (supported
   protocol versions, minimum app version), security, known issues.

## 5. Channels

| Artifact | stable | beta |
|---|---|---|
| Server images | tag `X.Y.Z` and digest | tag `X.Y.Z-beta.N` |
| Host service `bothy-host` | version `X.Y.Z` | `X.Y.Z-beta.N` |
| iOS | App Store once approved; until then TestFlight group "stable" | TestFlight group "beta" |
| macOS | Sparkle appcast without a channel (default) | Sparkle channel `beta` |
| Windows | `stable.appinstaller` | `beta.appinstaller` |
| Android | test track "stable" (owner decision, section 8) | internal test track |
| SDKs, adapters | version without a pre-release identifier | `X.Y.Z-beta.N` (npm tag `beta`, PyPI pre-release) |

Rules:

1. Apps: the version contains no channel. The same build (same build number, same
   signature) moves from beta to stable: switch the TestFlight group, remove the element
   `sparkle:channel` in the appcast, let `stable.appinstaller` point to the same MSIX.
2. Server, host service, SDKs, adapters: beta is a SemVer pre-release (`X.Y.Z-beta.N`), because the version is in the
   binary and in the package. Stable is built from the same commit as the last beta; between
   the two only the version number changes, and all gates run again.
3. Images have no moving tag `latest`. Compose and OpenTofu pin `BOTHY_VERSION`;
   Renovate (ADR 0041) proposes updates.
4. Sparkle channels are meant for temporary branches, not for permanently parallel editions
   (https://sparkle-project.org/documentation/publishing/, retrieved 2026-10-10) [verified]. That
   fits beta; a second permanent edition would need its own appcast.

## 6. Supply chain

| Step | Tool | Result |
|---|---|---|
| SBOM | Syft (via anchore/sbom-action) | SPDX JSON and CycloneDX JSON per image and binary, attached to the release |
| Signing images | public: cosign keyless (Fulcio, Rekor), identity = release workflow; private: cosign key pair without a transparency log ([ADR 0062](../adr/0062-release-signatures-while-the-repo-is-private.md)) | signature per digest |
| Signing files | cosign `sign-blob` with a bundle, same procedure as for the images | `*.sigstore.json` next to every file |
| Provenance | `actions/attest` (SLSA Build Provenance) | attestation in the repository |
| Checksums | SHA-256 | `SHA256SUMS`, itself signed |

Sources (retrieved 2026-10-10): https://github.com/anchore/sbom-action,
https://docs.sigstore.dev/cosign/signing/overview/, https://docs.sigstore.dev/cosign/verifying/verify/,
https://github.com/actions/attest-build-provenance [verified].

Limit: artifact attestations are available in GitHub Free, Pro and Team only for public
repositories (https://github.com/actions/attest-build-provenance, retrieved 2026-10-10) [verified].
Until the product repository is public, there are a signature and an SBOM but no attestation,
and signing uses the key pair of the private phase ([ADR 0062](../adr/0062-release-signatures-while-the-repo-is-private.md)); verification then uses
`cosign verify --key` and the public key from the repository. In the public state the
instance operator verifies like this:

```
cosign verify ghcr.io/<owner>/bothy-server@sha256:<digest> \
  --certificate-identity-regexp '^https://github.com/<owner>/bothy/.github/workflows/release.yml@refs/tags/server/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
gh attestation verify oci://ghcr.io/<owner>/bothy-server@sha256:<digest> --repo <owner>/bothy
```

The issuer `https://token.actions.githubusercontent.com` is not confirmed in the sources read
[assumed]; the release ticket checks it at the first real release.

## 7. Server images

1. Platforms linux/amd64 and linux/arm64, as one multi-arch manifest (B12).
2. Never built via emulation: `bothy-server` compiles the Go binary in the builder step on the
   build platform for the target architecture (without cgo, `--platform=$BUILDPLATFORM`); images
   with steps that must run on the target architecture (`bothy-postgres`) are built per
   architecture on their own runner.
3. `bothy-server` is a static Go binary (without cgo) on a minimal base image without a
   shell. It runs as an unprivileged user with uid and gid `65532`; `bothy secrets init`
   sets this owner for the secrets of `server` and `migrate` (16, section 5.3), and
   the image creates `secrets.signing_key_dir` with this owner and mode `0700`.
   `HEALTHCHECK` and Compose call `bothy healthcheck`, because there is no shell and no curl in
   the image.
4. `bothy-postgres` builds on the official `postgres:18` image and contains pgBackRest from
   2.59.3 on and `bothy-backupd` ([16-operations.md](16-operations.md) 8). Its tag is
   `pg18-<version>` with the server version, as `infra/compose/compose.yaml` names it
   (section 2.1).
5. Base images are pinned by digest; Renovate keeps them current; a check in the gate rejects every
   `FROM` without a digest (SEC-SC-4).
6. Labels according to OCI (`org.opencontainers.image.source`, `.version`, `.revision`, `.licenses`);
   version, revision and source come as build arguments, a build without a version fails.
7. `bothy-server` is built from `services/server/Dockerfile` with the repository root as build
   context (the same for `make image` and `release.yml`). The file
   `services/server/Dockerfile.dockerignore` applies to this Dockerfile only: it ignores
   everything and allows only `go.mod`, `go.sum`, `VERSION`, `api/`, `cmd/`, `internal/` and
   `migrations/` of the server, without test files and `testdata/`. A build whose `VERSION`
   differs from `services/server/VERSION` fails.

## 8. App distribution

The apps are white label. Whoever signs an app owns its identity (bundle ID,
package name, push keys, update feed). Therefore:

1. The product delivers source code, build scripts and a template `distribution.yaml` with all
   values of a publisher: bundle and package IDs, team ID, Sparkle feed URL and public
   EdDSA key, MSIX publisher, Firebase project, APNs topic, optional Sentry target.
2. Every organization builds and signs its apps itself with its own accounts. For an instance,
   `distribution.yaml` lives in the ops repo.
3. Push keys and the Firebase project of the instance belong to the publisher of the app (R-Push).
   An instance can only send push notifications to apps whose keys it has.

| Platform | Route | Sources (retrieved 2026-10-10) |
|---|---|---|
| iOS | TestFlight: up to 100 internal and 10,000 external testers, a build is usable for 90 days, external testers need a Beta App Review for the first build | https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview/ [verified] |
| macOS | Developer ID, notarization with `xcrun notarytool`, stapling with `xcrun stapler`, auto-update with Sparkle 2 (EdDSA-signed archives, channels, phased rollout with `sparkle:phasedRolloutInterval`) | https://developer.apple.com/developer-id/, https://sparkle-project.org/documentation/publishing/ [verified] |
| Windows | MSIX with `.appinstaller` per channel; signing via Azure Artifact Signing (organization in the EU possible) or an OV certificate with an HSM. SmartScreen warns at first even with EV | https://learn.microsoft.com/en-us/windows/msix/app-installer/app-installer-file-overview, https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart, https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation [verified] |
| Android | Play Internal Testing (up to 100 testers) or Firebase App Distribution; new personal Play accounts need a closed test with 12 testers over 14 days before production | https://support.google.com/googleplay/android-developer/answer/9845334, https://support.google.com/googleplay/android-developer/answer/14151465, https://firebase.google.com/docs/app-distribution [verified] |
| Web console | Proposal: part of the server image, served by the server; this way it is always the same age as the API | decided by ADR 0028 (team E) |

Consequences:

1. TestFlight for macOS requires an App Sandbox [assumed, only read via web search]. The
   Developer ID edition with Sparkle runs without a sandbox. Both would be two builds; the product
   delivers only the Developer ID edition for macOS.
2. The protocol `ms-appinstaller:` has been off by default since December 2023; users download the
   `.appinstaller` file and open it (source as above) [verified].
3. The owner decides on costs and accounts (Apple Developer Program, Artifact Signing or OV
   certificate, Play account) (report).

## 9. Compatibility between app and server

1. The app reports its protocol version and platform version in `identify`
   ([04-sync-protocol.md](04-sync-protocol.md) 6.3). The server supports the protocol versions
   from `hello.protocol_versions`. It closes older ones with `4007 protocol_version_unsupported`; the
   app then shows the notice "Update required" instead of an error.
2. `instance.discovery.read` returns `client.min_version.<platform>` and
   `client.recommended_version.<platform>` per platform. Below the minimum version the app does not
   connect and shows the update route of its channel. Below the recommended version it shows a notice.
3. How long a protocol or API version stays supported after it is superseded is defined by the
   deprecation policy (ADR 0014, team B). Release notes name the date.
4. The server is the leading side: a server upgrade never sets the minimum version higher than
   the oldest app that is still supported according to the deprecation policy.
5. Adapters report their version in the heartbeat (`reported_runtime_version`); the admin console
   shows adapters whose SDK version is no longer supported.

### 9.1 Host service and server

The host service reports its version in `identify` of the control connection. The server accepts
the versions whose protocol version it supports (prologue `bothy-terminal/1` and
`x-protocol-versions` of the channel in [asyncapi.yaml](../api/asyncapi.yaml)); it closes a version
that is too old with `4114`, and the console and the host card show "Update host service". The host
service does not update itself. Reason: a program that replaces itself would need
write permissions on its own binary; the service user deliberately does not have them
([24-terminal.md](24-terminal.md), section 2.2). Updates run through the package route of the
instance operator.

## 10. Rollback

| Artifact | Route | Limit |
|---|---|---|
| Server | start the previous version (`BOTHY_VERSION`, [upgrade.md](../operations/upgrade.md) 3) | exactly one version back (expand/contract, [16-operations.md](16-operations.md) 10); further back only via a restore |
| iOS | let the build expire in TestFlight, distribute a corrected build with a higher number | no downgrade on devices |
| macOS | new version with a higher number in the appcast; mark it as a critical update in case of danger | Sparkle does not install an older version |
| Windows | new MSIX with a higher version in the `.appinstaller` file | no automatic downgrade |
| Android | new build with a higher `versionCode` | no downgrade |
| SDKs | withdraw the version (PyPI yank, npm deprecate), publish a fix | installed copies remain |

Apps are therefore never rolled back but corrected forward. That is why app builds go into
beta first and then, with the same build number, into stable.

## 11. Security releases

1. Reports come in through the route described in `SECURITY.md` (team H).
2. Fix on a private branch, release with a Security section, notice to instance operators before
   the public description. The deadline until publication is in `SECURITY.md`.
3. The instance operator raises affected minimum versions of apps with
   `client.min_version.<platform>`.

## 12. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `client.min_version.ios` | empty | smallest allowed app version on iOS; empty means no limit |
| `client.min_version.android` | empty | the same for Android |
| `client.min_version.macos` | empty | the same for macOS |
| `client.min_version.windows` | empty | the same for Windows |
| `client.recommended_version.ios` | empty | below it the app shows an update notice |
| `client.recommended_version.android` | empty | the same for Android |
| `client.recommended_version.macos` | empty | the same for macOS |
| `client.recommended_version.windows` | empty | the same for Windows |

## 13. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-RL-01 | Version and changelog per component, tags `<component>/vX.Y.Z` (product bundle `vX.Y.Z`). | Independent releases; one source for names and prefixes in `tools/ci/components.py`; the Go tag rule does not apply to the main module that is never loaded (P-05). |
| R-RL-02 | No tag `latest`; Compose and OpenTofu pin versions. | Repeatable installation; an upgrade is a deliberate change. |
| R-RL-03 | Apps: the same build moves from beta to stable. Server, SDKs, adapters: stable from the same commit as the last beta, only the version number changes. | What was tested is what ships; where the version is in the artifact, a rebuild is unavoidable and limited to the number. |
| R-RL-04 | All artifacts signed with an SBOM: in the public state cosign keyless with attestation, in the private state with a key pair without a transparency log ([ADR 0062](../adr/0062-release-signatures-while-the-repo-is-private.md)). | Verifiable provenance without key management; attestation depends on the visibility. |
| R-RL-05 | Apps are white label; every organization builds and signs them itself with `distribution.yaml`. | Push keys, bundle IDs and update feeds belong to the publisher; no central push forwarding needed. |
| R-RL-06 | macOS only as a Developer ID edition with Sparkle, no second TestFlight edition. | One edition, no forced sandbox, own update channel. |
| R-RL-07 | Apps are corrected forward, never rolled back. | No platform reliably installs older versions. |
| R-RL-08 | Minimum and recommended app version per platform as server configuration. | The instance operator controls security updates without a release of the server. |
| R-RL-09 | The product bundle pins the server version in the tagged commit (`BOTHY_VERSION` in `infra/compose/.env.example`, written by `make release-prep COMPONENT=bothy SERVER_VERSION=<version>`, which rejects a server pre-release without `CHANNEL=beta`; gate `release-pins` in the pull request before the tag); `make release-build` sets `BOTHY_REGISTRY` from the owner and the download address in `install.sh` from the name of the repository and aborts on placeholders, on a server pre-release in the stable product and on missing images (2.1). | `install.sh` takes over both values unchanged; without a producer, every installation would pull the placeholder image. Version in the commit so that the tag alone determines the bundle, checked before the tag because a pushed tag stays; registry and download address from the build because they depend on the location of the repository. |
