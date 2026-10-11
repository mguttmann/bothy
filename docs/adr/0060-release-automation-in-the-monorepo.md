---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team F (operations and release), Team H (quality and repo)
informed: all planning teams
---

# Release automation in the monorepo

## Context and Problem Statement

Every component of the monorepo has its own version in `VERSION`, its own `CHANGELOG.md`
following "Keep a Changelog" and tags of the form `<component>/v<semver>` (ADR 0036,
[18-release.md](../spec/18-release.md), sections 2 to 4). Commits on `main` are squash commits
with a Conventional Commits header; the scope names the component
([push-discipline.md](../quality/push-discipline.md)). Every ticket records its change under
"Unreleased" in the changelog of the component ([PLAN.md](../PLAN.md), section 3, rule 7). Chapter
18 leaves open which tool determines the next version and prepares the release. Which tool, and how
far does it automate?

## Decision Drivers

- Local first: one command, the same locally and in CI (ADR 0038); no requirement for GitHub
  services.
- Push, tag and pull request only after the owner's approval; tags are signed (18, section 2).
- Workflows may only run on pull requests and release tags, never on pushes to branches
  (`tools/ci/check_runners.py`, rule `push-on-branch`; ADR 0039).
- A person or agent reviews every changelog section before the tag (18, section 4).
- Monorepo with Go, Dart, Python and TypeScript; the version is in `VERSION`, not in a
  package manifest.
- Before 1.0.0, a minor version may break (18, section 3).

## Considered Options

- git-cliff as a pinned local command line tool behind a make target; version from Conventional
  Commits per path, changelog from the tickets' entries, tag signed by hand
- release-please in manifest mode as a GitHub Action with release pull requests
- semantic-release with monorepo extensions, fully automatic after the merge
- GoReleaser with the monorepo feature
- Own script without a third-party tool

## Decision Outcome

Chosen option: "git-cliff locally behind a make target", because it runs offline, evaluates commits
per path and tag patterns and leaves the tag to the person.

1. `make release-prep COMPONENT=<name> [CHANNEL=beta]` calls a tested tool script.
   It reads the directory and tag prefix of the component from `tools/ci/components.py` (one source
   for CI path filters, tag resolution and release preparation).
2. git-cliff determines the next version from the commits since the last tag of the component
   (`--tag-pattern`, `--include-path` on the directory of the component, for `server` also
   `docs/api/`) with `--bumped-version` [Q1, verified]. Rules: `fix` gives a patch, `feat` a minor,
   a break a major; before 1.0.0 a break gives a minor (`breaking_always_bump_major = false`) [Q2,
   verified]. `CHANNEL=beta` appends `-beta.N`, N from the existing tags.
3. The text of the changelog comes from the entries under "Unreleased" that every ticket writes.
   The script renames the section to `[X.Y.Z] - <date>`, writes `VERSION` and shows the
   commits of the component that did not change its `CHANGELOG.md` as a checklist. The person or
   agent adds the required sections of the server release notes (upgrade notes, migrations,
   compatibility, security) in the review.
4. Commit `chore(release): <component> vX.Y.Z`, signed tag `git tag -s <component>/vX.Y.Z`
   by the maintainer, push after approval. Then `release.yml` runs (ADR 0036, ADR 0062).
5. git-cliff comes as a pinned binary with a checksum through `tools/bin/tools.lock.json`, like the
   other release binaries of the toolchain ([toolchain.md](../quality/toolchain.md)).
6. For the component `bothy`, `SERVER_VERSION=<version>` additionally writes the server version
   that the product bundle installs into the line `BOTHY_VERSION=` of
   `infra/compose/.env.example`, only if the tag `server/v<version>` exists
   ([18-release.md](../spec/18-release.md), section 2.1; D-W-76).

### Consequences

- Good, because the preparation runs locally and without network and uses the same state as CI.
- Good, because no tool needs write permissions on the repository; tags stay signed.
- Good, because the version rule follows from the commits and not from memory.
- Bad, because the version is only as good as the scope and type of the squash commits; the path
  filter catches wrong scopes, but not wrong types. Mitigation: a check of the PR title in
  `make commitlint`.
- Bad, because the changelog text remains manual work of the tickets; in return it holds language
  for users instead of commit subject lines.
- Neutral, because another binary is pinned and updated by Renovate (ADR 0041).

### Confirmation

- Tests of the tool script against a fixture repository with commits in several components:
  the right version per component (patch, minor, break before and after 1.0.0, beta counter),
  commits of other components do not count, a commit without a change to `CHANGELOG.md` appears in
  the checklist.
- Existing checks: `tools/ci/components.py release` rejects a tag whose version is not in
  `VERSION`; `release.yml` aborts if `CHANGELOG.md` has no section for the version.
- `make runners-check`: no workflow runs on pushes to branches; a release bot workflow would
  fail on it.
- Pin check `test_toolchain_doc.py`: the version of git-cliff in `toolchain.md` matches the pin.

## Pros and Cons of the Options

### git-cliff locally

- Good, because path filter, tag pattern and version rule are built in [Q1, Q2, verified] and it is
  under MIT or Apache-2.0 [Q3, verified].
- Bad, because a wrapper script for components, beta counter and changelog renaming is
  needed.

### release-please in manifest mode

- Good, because release pull requests, monorepo configuration and tags with the component are
  ready-made; squash merges are recommended [Q4, verified].
- Bad, because it works through the GitHub API: creating pull requests and setting tags needs
  write permissions, nothing runs locally, and the tags set by the tool are not signed by the
  maintainer [derived].
- Bad, because the usual trigger is a push to `main`; `make runners-check` rejects that.

### semantic-release

- Good, because it is fully automatic.
- Bad, because no person reviews the release before publishing ("no human is directly
  involved") [Q5, verified]; that contradicts 18, section 4, and the approval by the owner.

### GoReleaser with the monorepo feature

- Good, because building and publishing Go binaries is built in.
- Bad, because the monorepo feature is only in the paid Pro edition [Q6, verified]
  and does not cover apps, SDKs and adapters.

### Own script without a third-party tool

- Good, because there is no dependency.
- Bad, because commit parsing, tag selection and the version rule, which git-cliff already has,
  would have to be rebuilt and tested.

## More Information

- Sources (retrieved 2026-10-10): Q1 git-cliff, command line arguments,
  https://github.com/orhun/git-cliff/blob/main/website/docs/usage/args.md, and Monorepos,
  https://github.com/orhun/git-cliff/blob/main/website/docs/usage/monorepos.md [verified]; Q2
  git-cliff, section `bump`,
  https://github.com/orhun/git-cliff/blob/main/website/docs/configuration/bump.md [verified]; Q3
  https://github.com/orhun/git-cliff (license) [verified]; Q4 release-please,
  https://github.com/googleapis/release-please and
  https://github.com/googleapis/release-please/blob/main/docs/manifest-releaser.md [verified]; Q5
  semantic-release, https://github.com/semantic-release/semantic-release [verified]; Q6 GoReleaser
  Monorepo, https://goreleaser.com/customization/monorepo/ [verified].
- Tag form: for a Go module in a subdirectory, the subdirectory is the prefix of the tags
  (https://go.dev/ref/mod, section "Module paths") [verified]. The server module lives in
  `services/server` and is never loaded as a dependency ([PLAN.md](../PLAN.md), P-05); for the build
  the tag form is therefore irrelevant, and `server/vX.Y.Z` stays valid.
- Related: ADR 0036 (releases), ADR 0038 (local gates), ADR 0039 (runner strategy), ADR 0062
  (signatures while the repo is private).
