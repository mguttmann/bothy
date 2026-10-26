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
| `bothy` | root | none (`vX.Y.Z`) | installation bundle `bothy-compose-<version>.tar.gz` from `make release-build`: `infra/compose` without `infra/compose/test` (test overlays) and without the source of `install.sh`, the folders `infra/otel` and `infra/observability` that `compose.yaml` mounts relative to itself, `LICENSE`, `NOTICE`, `CHANGELOG.md`, `VERSION`; its `infra/compose/.env.example` contains the pinned server version and the registry (section 2.1); in addition `install.sh` as a separate file with a built-in version, SHA-256 and download address of the bundle (section 2.1, [16-operations.md](16-operations.md), section 3.1) | stable, beta |
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
5. **Grafana panel plugin** (M7-26). The bundle carries the build of the panel plugin
   `bothy-live-panel` ([25-visualization.md](25-visualization.md), section 8.3): `make
   release-build` builds the release ZIP of the component `grafana-bothy-live` for the version in
   `plugins/grafana-bothy-live/VERSION` of the commit and puts its files into
   `infra/observability/grafana/plugins/bothy-live-panel/` of the bundle, with the time of the
   commit, mode `0644` and owner 0. It writes nothing and names the value if the ZIP is missing,
   names another plugin or another version than that file, or holds a member outside
   `bothy-live-panel/`, a link or `..`.

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

As built (M7-29, [docs/DECISIONS.md](../DECISIONS.md) D-M7-29-2, D-M7-29-3, D-M7-29-5): `make
release-build VERSION=<v> OUT=<dir>` runs `tools/ci/product_release.py build`. It reads
`VERSION`, `infra/compose/.env.example`, `compose.yaml` and `install.sh` from the commit `HEAD`,
not from the working tree; it checks each image with `docker manifest inspect` (the release job
logs in to `ghcr.io` first); it writes the bundle reproducibly (file order, modes and times of
the commit, a gzip header without a time) and `install.sh` with mode `0755`, and only after every
check, so an abort leaves no file. In `install.sh` the three values are the lines
`BOTHY_INSTALL_VERSION=`, `BOTHY_INSTALL_BUNDLE_SHA256=` and `BOTHY_INSTALL_BASE_URL=`; the
words `OWNER` and `placeholder` must not occur in the finished script in any case. Trial only
(D-W-127): `RELEASE_TRIAL_REGISTRY=127.0.0.1:<port>` replaces `ghcr.io/<owner>` and
`RELEASE_TRIAL_BASE_URL=http://127.0.0.1:<port>/` the download address; both are refused when
`GITHUB_ACTIONS` is set and for any other host, and the image check then uses
`docker manifest inspect --insecure`. `make release-trial COMPONENT=bothy` sets them (section 6.2).
In `release.yml` the job `product-signing` (environment `release`) writes SBOMs, signs bundle,
`install.sh` and `SHA256SUMS` with `tools/ci/release_chain.py` and verifies them like the server
files (section 6.1), and `publish` takes that set by artifact id.

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
| SBOM | Syft (server: the pinned binary through `tools/ci/release_chain.py sbom`; other components: anchore/sbom-action until their release tickets) | SPDX JSON and CycloneDX JSON per image and platform and per binary, attached to the release |
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

### 6.1 Server release chain (as built by M7-10)

`release.yml` runs these steps for a tag `server/vX.Y.Z`, in this order; the steps after the
image builds are commands of `tools/ci/release_chain.py`, which `make release-trial` runs the
same way against a local registry (section 6.2):

1. `resolve`: `tools/ci/components.py release` rejects the tag if `services/server/VERSION`
   differs from it or `services/server/CHANGELOG.md` has no section `[X.Y.Z]`; nothing is built
   then. The full gate (`ci.yml`) follows.
2. `build`: the binaries `bothy_<version>_linux_amd64` and `..._linux_arm64` from
   `make -C services/server release-build`, and `bothy-server` for both platforms in one build
   (it cross compiles), pushed by digest without a tag. `postgres-build-amd64` and
   `postgres-build-arm64`: `bothy-postgres` per platform on a native runner (`ubuntu-24.04`,
   `ubuntu-24.04-arm`), pushed by digest, each passing the id of its digest artifact.
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

Signing paths ([ADR 0062](../adr/0062-release-signatures-while-the-repo-is-private.md)): in
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

`make release-trial COMPONENT=bothy` (M7-29) runs the trial of `server` first and then, against
the same local registry, the product build of section 2.1 from a throwaway repository with the
bundle paths of the commit and `BOTHY_VERSION` set to the trial server version, with
`RELEASE_TRIAL_REGISTRY` and `RELEASE_TRIAL_BASE_URL` pointing at the local registry and a static
HTTP server on `127.0.0.1`. It then checks what an installation needs: `install.sh` and the bundle
downloaded from that address, the bundle's SHA-256 equal to the value in `install.sh`, registry
and server version of the bundle equal to the trial's, every image of the bundle's
`compose.yaml` present there with the signed digest, and a `docker pull` of each by the Docker
daemon. A full installation from that result is the local end-to-end run of M7-14
([16-operations.md](16-operations.md) 11.1). Since M7-26 the product trial also builds the plugin
ZIP for the bundle (rule 5 of section 2.1) and checks the plugin version in the downloaded bundle.

`make release-trial COMPONENT=grafana-bothy-live` (M7-26) needs no registry: it runs `make -C
plugins/grafana-bothy-live release-build`, checks the ZIP like the product build does, writes the
SPDX and CycloneDX SBOMs of the ZIP, signs the ZIP and `SHA256SUMS` with a throwaway key and the
signing configuration of the private phase, and verifies them with `cosign verify-blob` and
`sha256sum --check`, the steps of the job `plugin-signing` of `release.yml`. Two probes must
fail: a changed ZIP against `SHA256SUMS`, and the signing step of a release without its SBOM.

## 7. Server images

1. Platforms linux/amd64 and linux/arm64, as one multi-arch manifest (B12).
2. Never built via emulation: `bothy-server` compiles the Go binary in the builder step on the
   build platform for the target architecture (without cgo, `--platform=$BUILDPLATFORM`); images
   with steps that must run on the target architecture (`bothy-postgres`) are built per
   architecture on their own runner. In `release.yml` `bothy-postgres` is built per architecture
   on a native runner and pushed by digest; the release joins both digests into one multi-arch
   index (section 6.1). Images are pushed by digest only; the version tag comes last, after the
   vulnerability scan, ScanCode, SBOMs, signatures and verification.
3. `bothy-server` is a static Go binary (without cgo) on a minimal base image without a
   shell. It runs as an unprivileged user with uid and gid `65532`; `bothy secrets init`
   sets this owner for the secrets of `server` and `migrate` (16, section 5.3), and
   the image creates `secrets.signing_key_dir` with this owner and mode `0700`.
   `HEALTHCHECK` and Compose call `bothy healthcheck`, because there is no shell and no curl in
   the image.
4. `bothy-postgres` builds on the official `postgres:18` image and contains pgBackRest from
   2.59.3 on and `bothy-backupd` ([16-operations.md](16-operations.md) 8). Its tag is
   `pg18-<version>` with the server version, as `infra/compose/compose.yaml` names it
   (section 2.1); the tag of `bothy-server` is `<version>`.
5. Base images are pinned by digest; Renovate keeps them current; a check in the gate rejects every
   `FROM` without a digest (SEC-SC-4).
6. Labels according to OCI (`org.opencontainers.image.source`, `.version`, `.revision`, `.licenses`);
   version, revision and source come as build arguments, a build without a version fails.
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
   Developer ID edition with Sparkle does not need one; both would be two builds, and the product
   delivers only the Developer ID edition for macOS. It keeps the sandbox of its release
   entitlements and installs updates through Sparkle's installer service (8.1).
2. The protocol `ms-appinstaller:` has been off by default since December 2023; users download the
   `.appinstaller` file and open it (source as above) [verified].
3. The owner decides on costs and accounts (Apple Developer Program, Artifact Signing or OV
   certificate, Play account) (report).

### 8.1 Desktop release chain (as built by M7-12)

`apps/distribution/distribution.yaml` is the template of the publisher values, validated against
`distribution.schema.json`; a missing or malformed value fails the build and names its key, and a
release refuses every placeholder of the template. The build takes the bundle identifier from it
into the build branding. The steps are `tools/distribution/apple.py` and `windows.py`, started by
`make -C apps/client release-apple` and `release-windows` in the release jobs `build-apple`
(`macos-15`, environment `release-apple`) and `build-windows` (`windows-2025`, environment
`release-windows`); the publisher values come from the environment variable `DISTRIBUTION_YAML` and the build
branding from `BRANDING_YAML`,
the signing material from the environment's secrets, and without them a job stops before
anything is signed ([release.md](../operations/release.md) 4).

| | macOS | Windows |
|---|---|---|
| Package | the app with Sparkle.framework of the pinned release (version and SHA-256 in `apple.py`), zipped with `ditto` | MSIX with the package identity, the toast activator CLSID of the app (D-M4-30-2), the URL scheme and the Visual C++ runtime next to the app |
| Signature | Developer ID, hardened runtime, every nested code item first; the provisioning profiles of app and extension; `aps-environment` production | the publisher's certificate (`windows.publisher` is its subject), with the timestamp server of `windows.timestamp_url` |
| Checks | `codesign --verify --deep --strict` and the team of `apple.team_id`; `xcrun notarytool submit --wait`, `xcrun stapler staple`; `spctl --assess` must accept | `signtool verify /pa` |
| Update files | the archive's EdDSA signature from Sparkle's `sign_update`; `appcast.xml` with the entry in the channel `beta` and `sparkle:phasedRolloutInterval`, signed as a whole with the same key (the app requires a signed feed) | `beta.appinstaller` pointing at the MSIX, with an update check at every start |
| Promotion (5 rule 1) | `appdist.py promote-appcast` removes `sparkle:channel` of the build and signs the appcast again | `appdist.py promote-appinstaller` writes `stable.appinstaller` for the same MSIX |
| Updater in the app | `AppUpdater` loads Sparkle at run time when the bundle carries it; default channel, plus the channels of the user default `UpdateChannels` | App Installer, from the `.appinstaller` file the package was installed from |

Versions are `X.Y.Z` without a pre-release; the build number is `X * 1000000 + Y * 1000 + Z`,
the MSIX version `X.Y.Z.0`.

Trial with a test publisher (D-W-127, docs/decisions.d/M7-12.md): `make -C apps/client
trial-apple` and `trial-windows` run the same steps with a self-signed identity in a temporary
keychain or certificate store, a throwaway EdDSA key and an update host on `127.0.0.1`, build
`0.1.0` and `0.1.1`, and prove the update path: Sparkle's own updater (`SPUUpdater` in
`tools/distribution/macos/update_harness.swift`) offers nothing on the default channel, refuses a
changed archive, replaces the installed `0.1.0` by `0.1.1` on the channel beta and, after
promotion, on the default channel; App Installer (`Add-AppxPackage -AppInstallerFile`) installs
`0.1.0` from `beta.appinstaller`, refuses an MSIX with a changed signature, updates to `0.1.1` and installs it again
from `stable.appinstaller`. CI runs both in the jobs `desktop trial (macOS)` and `desktop trial
(Windows)` for changes of the release steps. Not proven by the trial: Developer ID signing,
notarization and the acceptance of `spctl` (it rejects a test publisher, which the trial
records), a signature by a certificate authority on Windows and the reputation of SmartScreen;
they run first with the owner's accounts (OQ-08).

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
5. Adapters report their version in the heartbeat in `details.adapter.version` (stored in
   `agents.reported_details`); `reported_runtime_version` (heartbeat member `runtime_version`) is
   the version of the runtime ([09-agents.md](09-agents.md) section 4.8). The admin console
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
