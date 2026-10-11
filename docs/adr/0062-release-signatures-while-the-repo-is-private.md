---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team H (quality and repo), Team F (release)
informed: all planning teams
---

# Release signatures while the repo is private

## Context and Problem Statement

ADR 0036 specifies: every artifact is signed with cosign keyless, has an SBOM and, as soon as the
repository is public, an attestation. Until the owner's go, the product repository stays
private. Two things then do not work as planned: GitHub Free, Pro and Team offer artifact
attestations only for public repositories ([18-release.md](../spec/18-release.md), section 6), and
a keyless signature writes an entry to the public transparency log Rekor, whose
certificate names the repository and the workflow [Q1, Q2, verified]. This would make a private
repository publicly visible. How are releases signed while the repository is private?

## Decision Drivers

- Nothing about the private repository (name, account, tags, points in time) reaches public logs
  (white label; only the owner approves the visibility).
- Releases are signed, also in the private state (owner requirement for signed releases);
  the owner's instance can verify them before deployment.
- Signing secrets are held only in protected environments of the tag workflows.
- The verification path of the release chain runs for real once before going public
  (build ticket M7-10).
- No costs without a decision by the owner.

## Considered Options

- Key pair for cosign in the private state, signature without a transparency log; from going
  public on, keyless and attestations per ADR 0036
- Keyless with the public Sigstore also in the private state
- No signature until going public, only checksums, SBOM and provenance in the
  registry entry
- GitHub Enterprise Cloud, whose own Sigstore instance issues private attestations without a public
  log

## Decision Outcome

Chosen option: "Key pair without a transparency log in the private state", because it delivers
signed and verifiable releases without publishing anything, and it costs nothing.

1. A cosign key pair (ECDSA P-256) is created once for the private phase. The private
   key and its password are held only as secrets of the environment `release`, which is allowed
   only for tags of the form `v*` and `*/v*`. The public key is in the repository.
2. The release chain signs image digests and `SHA256SUMS` with this key and a
   signing config without Rekor, Fulcio and timestamp service (`cosign signing-config create` with
   `--no-default-rekor`, `--no-default-fulcio`, `--no-default-tsa`, `--no-default-oidc`)
   [Q3, verified]. The outdated flag `--tlog-upload=false` is not used; cosign 3.0.3 deprecated
   it [Q4, verified]. The signature bundles are stored next to the artifacts.
3. Verification uses the public key (`cosign verify --key`); because there is no
   log entry, with `--insecure-ignore-tlog` [Q5, verified]. Build ticket M7-10 sets the exact
   command for the pinned cosign version.
4. SBOM and `SHA256SUMS` are created as in ADR 0036; attestations are not (`release.yml` runs them
   only when `visibility == 'public'`).
5. When going public ([public-switch.md](../quality/public-switch.md)), the chain switches to
   keyless and attestations (ADR 0036). The private key is deleted; the public key
   stays in the repository so that old releases remain verifiable.

### Consequences

- Good, because no public log entry gives away the private repository.
- Good, because the owner's instance can verify images against the public key before the start
  and the signing path is in operation before going public.
- Bad, because a long-lived key must be protected. On GitHub Pro, private repositories get no
  required reviewers on environments, only environment secrets and rules for branches
  and tags [Q6, verified]; whoever can set tags can have releases signed. In the private state
  that is only the owner.
- Bad, because without a transparency log a stolen key could sign unnoticed.
  Mitigation: short lifetime (only the private phase), deletion when going public, replacing
  the key after any suspicion.
- Bad, because instance operators verify differently from going public on (identity instead of
  key); in the private state there is only one instance operator, the owner.
- Neutral, because the release chain knows two signing paths, controlled by the visibility like the
  attestations.

### Confirmation

- Build ticket M7-10 in [PLAN.md](../PLAN.md): `cosign verify` with the public key and
  `sha256sum --check` against a sample release are green; a release without a signature aborts.
- Test of the release chain in the private state: the signing config contains no Rekor, Fulcio or
  timestamp service, and no generated bundle has an entry of a transparency log.
- `make runners-check`: `release.yml` runs only on tag pushes. An addition in build ticket M7-10
  checks that signing steps are only in jobs with `environment: release`.
- Checklist [public-switch.md](../quality/public-switch.md): switch to keyless, deletion of the
  private key, sample release with `gh attestation verify`.

## Pros and Cons of the Options

### Key pair without a transparency log

- Good, because signed, verifiable and without publicity.
- Bad, because of key management and no public log.

### Keyless with the public Sigstore

- Good, because there is no key management.
- Bad, because every signing operation is public in the log, with a certificate including repository
  and workflow [Q1, Q2, verified]; that publishes the private repository before the owner's go.

### No signature until going public

- Good, because there is nothing to manage; this is how `release.yml` works today.
- Bad, because a checksum shows only transmission errors, not origin (ADR 0036); whoever
  can change release assets also changes `SHA256SUMS`.
- Bad, because the signing and verification path runs for the first time only after going public.

### GitHub Enterprise Cloud

- Good, because private repositories get attestations through GitHub's own Sigstore instance without
  a transparency log [Q2, verified].
- Bad, because a different plan with running costs is needed; only the owner decides that.

## More Information

- Sources (retrieved 2026-10-10): Q1 Sigstore, Cosign Signing Overview,
  https://docs.sigstore.dev/cosign/signing/overview/, and Fulcio OID extensions (Source
  Repository URI, Build Signer URI),
  https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md [verified]; Q2 GitHub, Artifact
  attestations,
  https://docs.github.com/en/actions/concepts/security/artifact-attestations [verified]; Q3 cosign
  `signing-config create`,
  https://github.com/sigstore/cosign/blob/main/doc/cosign_signing-config_create.md [verified]; Q4
  cosign changelog v3.0.3 "Deprecate tlog-upload flag",
  https://github.com/sigstore/cosign/blob/main/CHANGELOG.md [verified]; Q5 cosign `verify`,
  https://github.com/sigstore/cosign/blob/main/doc/cosign_verify.md [verified]; Q6 GitHub Docs,
  availability of environments,
  https://github.com/github/docs/blob/main/data/reusables/gated-features/environments.md [verified].
- The owner decides costs and plan (option GitHub Enterprise Cloud); the default is this
  ADR.
- Related: ADR 0036 (releases), ADR 0039 (runners and publication), ADR 0060
  (release preparation).
