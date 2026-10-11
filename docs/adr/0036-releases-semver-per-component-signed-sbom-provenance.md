---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team F (operations)
consulted: Team H (quality and repo), Team D (agent platform), Team E (apps)
informed: all teams
---

# Releases: SemVer per component, signed artifacts, SBOM, provenance

## Context and Problem Statement

The monorepo contains server, apps, SDKs (Python, TypeScript) and adapters. They change at
different paces and have different users: instance operators, app publishers, developers of
integrations. Instance operators must be able to check the origin and content of every artifact.
How do we version, tag, sign and document?

## Decision Drivers

* independent releases without artificial version jumps
* tags must be unique per component and resolvable for tools
* verifiable origin without long-lived signing keys
* SBOM for license and vulnerability checks by the instance operator
* release notes that say whether an upgrade needs manual work

## Considered Options

* SemVer per component with `VERSION` and `CHANGELOG.md` per component, tags `<component>/vX.Y.Z`;
  cosign keyless, SBOM (Syft), build provenance (`actions/attest`)
* One shared version for the whole monorepo (lockstep)
* Signature with a long-lived key (GPG or cosign key pair)
* No signature, only checksums

## Decision Outcome

Chosen option: "SemVer per component; cosign keyless, SBOM, provenance", because components are
published independently and the origin can be verified without key management.

1. Tags `<component>/vX.Y.Z` (`server/vX.Y.Z`, `client/vX.Y.Z`, `sdk-python/vX.Y.Z` and so on),
   the installation bundle `vX.Y.Z`; names and prefixes from `tools/ci/components.py`
   ([18-release.md](../spec/18-release.md) 2). Go's tag rule for modules in
   subdirectories (https://go.dev/ref/mod, retrieved 2026-10-10) does not apply to the server
   module, because as a main module it is never loaded (PLAN P-05).
2. Channels stable and beta; apps promote the same build, server and packages a pre-release
   from the same commit ([18-release.md](../spec/18-release.md) 5).
3. Every artifact: SBOM (SPDX and CycloneDX), cosign signature (keyless; as long as the repository
   is private, a key pair without a transparency log according to ADR 0062), checksums;
   attestation via `actions/attest` as soon as the repository is public (in Free, Pro and Team
   available only for public repositories; https://github.com/actions/attest-build-provenance,
   retrieved 2026-10-10).
4. No `latest` tag; installations pin versions.

### Consequences

* Good, because an SDK bug does not force a server release and vice versa.
* Good, because instance operators verify the origin with `cosign verify` and
  `gh attestation verify`.
* Bad, because several changelogs and tags have to be maintained (a tool generates drafts).
* Bad, because no attestation is created until the repository is published.
* Neutral, because the Go module path of the server stays the same on a major jump, as long as
  the module is never loaded as a dependency (PLAN P-05).

### Confirmation

* Gate check: `VERSION` equals the tag; the changelog section is present.
* The release chain aborts if signature, SBOM or checksums are missing.
* The portability test pulls images by digest and checks the signature first.

## Pros and Cons of the Options

### SemVer per component

* Good, because versions say something about the respective component.
* Bad, because the compatibility between components needs its own rules (protocol version,
  minimum app version).

### Shared version

* Good, because there is one number for everything.
* Bad, because every change to one component publishes all others again.
* Bad, because SemVer statements (breaking yes or no) would be wrong for most components.

### Long-lived signing key

* Good, because it is independent of CI and public infrastructure.
* Bad, because the key must be protected, rotated and revoked on loss.

### Only checksums

* Good, because it is simple.
* Bad, because a checksum shows only transmission errors, not the origin.

## More Information

* Procedure: [docs/operations/release.md](../operations/release.md).
* Tool for changelog drafts: git-cliff behind `make release-prep`
  ([ADR 0060](0060-release-automation-in-the-monorepo.md)).
* Signatures in the private phase: [ADR 0062](0062-release-signatures-while-the-repo-is-private.md).
