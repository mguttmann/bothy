# Making a release

Status: draft M0. Procedure for [18-release.md](../spec/18-release.md). ADR 0038 and 0039 define
gates and runners (team H); this procedure names only the steps that belong to the release.

![Release pipeline](diagrams/release-pipeline.svg)

## 1. Prerequisites

1. Main branch green in all local gates and in CI.
2. Portability test ([16-operations.md](../spec/16-operations.md) 11.1) green, depending on the tag:
    - stable server `server/vX.Y.Z`: the test of the product beta whose bundle pins its last
      beta `X.Y.Z-beta.N`. Stable is built from the same commit as this beta
      ([18-release.md](../spec/18-release.md) 5, rule 2).
    - server beta `server/vX.Y.Z-beta.N`: no precondition from the test. A bundle can pin it
      only once its tag exists ([18-release.md](../spec/18-release.md) 2.1, rule 1), and for
      `beta.1` there is no earlier beta. It is checked by the test of the next product beta
      that pins it; that test runs after the release of this product beta (2.2, step 5;
      [environments-and-flags.md](environments-and-flags.md) 1.3, item 1). Only when it is green
      may the stable server version follow (item above).
    - product beta `vA.B.C-beta.M`: no precondition; `release.yml` starts its own test after
      `publish` (2.2, step 5).
    - stable product `vA.B.C` and every other component: the test of the last product beta,
      as soon as there is one.
3. `CHANGELOG.md` of the component has a reviewed section for the new version; release notes
   with the required sections ([18-release.md](../spec/18-release.md) 4).
4. For `server`: migrations fulfill M1 to M12 ([upgrade.md](upgrade.md) 4); the
   N-1 compatibility test is green.
5. Minimum app versions and deprecation dates in the release notes match ADR 0014.

## 2. Server and product

New installations with `install.sh` get the server version that the bundle of a
product release pins. Therefore a server release that new installations are to get is followed
by a product release; existing installations raise `BOTHY_VERSION` themselves
([upgrade.md](upgrade.md)).

### 2.1 Server

1. `make release-prep COMPONENT=server [CHANNEL=beta]` sets `VERSION` and closes the
   changelog section ([ADR 0060](../adr/0060-release-automation-in-the-monorepo.md)); commit
   on a branch, pull request to `main` ([push-discipline.md](../quality/push-discipline.md)).
2. After the merge, set the signed tag on the squash commit on `main`: `server/vX.Y.Z`
   (beta: `server/vX.Y.Z-beta.N`).
3. The release pipeline builds per architecture, creates the multi-arch manifest, SBOMs, signatures,
   checksums and (in the public repository) the attestation, and publishes to `ghcr.io`.
4. Check like an instance operator (section 5).
5. Beta: to `staging`; stable: mark the release as "latest release", inform the instance operators.

### 2.2 Product (component `bothy`)

1. The server version that the bundle is to install is published (section 2.1).
   A product beta may pin a server beta, a stable product only a stable
   server version ([18-release.md](../spec/18-release.md) 2.1).
2. `make release-prep COMPONENT=bothy SERVER_VERSION=<version> [CHANNEL=beta]` sets `VERSION`,
   closes the changelog section and writes the server version into the line
   `BOTHY_VERSION=` of `infra/compose/.env.example`. It aborts without a change if the tag
   `server/v<version>` does not exist or if without `CHANNEL=beta` the version carries a
   pre-release identifier. Commit on a branch, pull request to `main`; the diff shows the pinned
   server version. The gate `release-pins` checks the same rule in the pre-push of this branch
   and in the required check `ci-ok` of the pull request. The push of the tag alone starts no
   check ([18-release.md](../spec/18-release.md) 2.1, rule 1).
3. After the merge, set the signed tag on the squash commit on `main`: `vA.B.C` (beta:
   `vA.B.C-beta.M`). The numbers do not depend on the
   server version ([environments-and-flags.md](environments-and-flags.md) 1.3).
4. The release pipeline builds the bundle with `make release-build`: `BOTHY_VERSION` from the commit,
   `BOTHY_REGISTRY` from the owner of the repository. In addition `install.sh` with version,
   SHA-256 and download address of the bundle, the address from the name of the repository. It
   aborts without a bundle on a placeholder, a server pre-release in the stable product (second
   safeguard) or an image that is missing in the registry
   ([18-release.md](../spec/18-release.md) 2.1). Signatures, SBOM and checksums as for the server.
5. After `publish`, `release.yml` starts the portability test with the tag
   ([16-operations.md](../spec/16-operations.md) 11.1); it must be green. Then check like an
   instance operator (section 5).

## 3. SDKs and adapters

1. `make release-prep COMPONENT=<component>`, commit, pull request to `main`, after the merge
   tag `<component>/vX.Y.Z` on the squash commit (names from
   `tools/ci/components.py`).
2. Build, SBOM, signature of the files (`cosign sign-blob`), publication on PyPI or npm via
   trusted publishing of the registry, where available [assumed: check the availability per
   registry in the ticket].
3. Beta: pre-release, npm tag `beta`.

## 4. Apps

The publisher builds the apps with their `distribution.yaml` ([18-release.md](../spec/18-release.md) 8).

| Platform | Steps |
|---|---|
| iOS | build with a build number, upload to App Store Connect, TestFlight group beta; once proven, group stable or submission |
| macOS | build, signature with Developer ID, `xcrun notarytool submit --wait`, `xcrun stapler staple`, sign the archive with `sign_update` (EdDSA), entry in the appcast with `sparkle:channel` beta; once proven, remove the element |
| Windows | build and sign the MSIX (Artifact Signing or OV with HSM), `beta.appinstaller` to the new version; once proven, `stable.appinstaller` |
| Android | build and sign the AAB, internal test track; once proven, the next track |
| Web console | with the server release (proposal, ADR 0028) |

Before the switch to stable: check the crash rate of the beta build (if a target for
crash reports is configured) and no open bugs of severity critical.

## 5. Check like an instance operator

```
cosign verify ghcr.io/<owner>/bothy-server@sha256:<digest> \
  --certificate-identity-regexp '^https://github.com/<owner>/bothy/.github/workflows/release.yml@refs/tags/server/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
gh attestation verify oci://ghcr.io/<owner>/bothy-server@sha256:<digest> --repo <owner>/bothy
cosign verify-blob SHA256SUMS --bundle SHA256SUMS.sigstore.json \
  --certificate-identity-regexp '^https://github.com/<owner>/bothy/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --check SHA256SUMS
```

These commands apply in the public state (keyless). In the private state, the instance operator
checks with the public key from the repository (`cosign verify --key`, without a transparency log;
the exact command for the pinned cosign version will be here after M7-10, [ADR 0062](../adr/0062-release-signatures-while-the-repo-is-private.md)). The
release workflow is `.github/workflows/release.yml`.

## 6. Recall

1. Server: mark the release as withdrawn, notice to the instance operators with the rollback
   ([upgrade.md](upgrade.md) 3), corrected patch version.
2. Apps: corrected build with a higher number; for security bugs, have `client.min_version.<platform>`
   set to the corrected version.
3. SDKs: withdraw the version (PyPI yank, npm deprecate), publish the fix.
