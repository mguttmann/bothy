---
status: accepted
date: 2026-10-11
decision-makers: Maintainer
---

# Only GitHub-hosted runners, public from the first push

## Context and Problem Statement

Contributions are to flow one after another as pull requests into a public repository, checked
by CI, SonarQube Cloud and CodeRabbit. For public repositories: "Use of the standard
GitHub-hosted runners is free and unlimited on public repositories"
(https://docs.github.com/en/actions/reference/runners/github-hosted-runners, retrieved
2026-10-11) [verified]. A runner that the maintainers operate themselves would, in a
public repository, run code from other people's pull requests on their machines; GitHub advises
against this for public repositories
(https://docs.github.com/en/actions/reference/security/secure-use, retrieved 2026-10-10)
[verified]. On which runners do the workflows run, and from when on is the repository public?

## Decision Drivers

* No pull request runs code on a machine of the maintainers.
* One set of workflow files without a switch, valid from the first push on.
* CI on GitHub's standard runners, which are free for public repositories.
* Checkable by a tool, not only by review.

## Considered Options

* Only GitHub-hosted runners, repository public from the first push
* Self-hosted runners as long as the repository is private, then GitHub-hosted runners
  (switch in `runs-on`)
* Always self-hosted runners

## Decision Outcome

Chosen option: "Only GitHub-hosted runners, repository public from the first push", because no
runner of the maintainers exists that other people's code can reach, and the workflows need no
switch. There are no self-hosted runners. Every job names a fixed label of a
standard runner: `ubuntu-24.04`, for the portability test additionally `ubuntu-24.04-arm`, for
the app builds `macos-15` and `windows-2025`. The repository is switched to public directly after
the first push ([public-switch.md](../quality/public-switch.md)).

Supplementary rules:

1. **Public-only jobs.** CodeQL, Scorecard and Pages run only while the repository is
   public (job condition `github.event.repository.visibility == 'public'`): code
   scanning and Scorecard require a public repository, and Pages would turn even a
   private repository into a public website. Scheduled runs carry no repository object in
   the payload [derived from the event reference, which names no payload for `schedule`,
   https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows,
   retrieved 2026-10-10]; for them `vars.BOTHY_VISIBILITY == 'public'` applies in addition, set
   when switching to public. If the variable is missing, scheduled jobs do not run.
2. **Release workflows.** `release.yml` starts only on a tag push, `portability.yml` only as a
   call from `release.yml` after `publish` (`workflow_call`) or by hand with a product tag
   (`workflow_dispatch`, D-W-59). Only `release.yml` calls a release workflow, from one job
   and without a matrix (D-W-73), and no workflow starts another one by `workflow_dispatch`,
   because none has `actions: write` (D-W-111; the dispatch endpoint requires the permission
   "Actions" with `write`,
   https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps,
   retrieved 2026-10-11 [verified]): the portability test checks the release that
   the same run published, and does so once.
3. **Fixed labels.** No label `*-latest` (it changes the image without a change in the
   repository), no larger runners (they are not standard runners), no list, no group and
   no expression except `${{ matrix.<key> }}` over a static matrix of fixed labels.
4. **Release jobs.** Signing secrets exist only in the environments `release`, `release-apple`,
   `release-windows` and `release-android` (M7-13). A job with one of these environments uses outputs of other jobs only in the
   `if:` of the job or of a step (D-W-72), resolves the tag again itself and compares, and
   fetches artifacts only by ID (D-W-52). GitHub compares environment names without case
   ("Environment names are not case sensitive",
   https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments,
   retrieved 2026-10-11 [verified]); the check does so too and requires a literal environment name without an expression (D-W-110).
   Every standard runner except the single-core runners is
   a new VM ("each GitHub-hosted runner is a new virtual machine (VM) hosted by GitHub",
   same source as above, retrieved 2026-10-11) [verified]; a job therefore takes over no state of
   another job except through caches, artifacts and outputs.
5. **Scan after the merge.** The only exceptions to "no `push` on branches" are
   `denylist-main.yml`: a `push` to exactly `main`, which checks the pushed commits against the
   private deny list (D-W-34), and `build-verify.yml`: a `push` to exactly `build/**`, which runs
   every gate of `make gates` without secrets for a build team and is no required check
   ([push-discipline.md](../quality/push-discipline.md#build-branches)).

### Consequences

* Good, because no pull request runs code on a machine of the maintainers.
* Good, because the workflows are simple: one fixed label per job, no switch, no setup
  and no removal of runners.
* Good, because CI, CodeQL, Scorecard, SonarQube Cloud and CodeRabbit run from the first pull
  request on.
* Bad, because everything in the repository is public from the first push; the state must be
  fully checked beforehand ([public-switch.md](../quality/public-switch.md), part A).
* Bad, because no CI runs before the first push; until then `make check` checks locally
  (ADR 0038).
* Bad, because a new image version from GitHub needs a change to the workflows; in return no
  run changes without a change in the repository.

### Confirmation

`tools/ci/check_runners.py` (gate `runners-check`, tests in `tools/ci/tests/`) rejects every job
whose runner is not a fixed GitHub-hosted label (rule `runner-not-github-hosted`),
as well as scheduled workflows without a public condition (`schedule-without-public-guard`),
release workflows with triggers other than their own (`release-trigger`), calls of a
release workflow outside a matrix-free job of `release.yml` (`reusable-release-caller`,
D-W-73), `actions: write` and `write-all`, also on a job (rules `actions-write`,
`broad-permissions`, D-W-111), `push` on branches except `main` in `denylist-main.yml`
(`push-on-branch`), environment names that are not literal (`environment-not-literal`, D-W-110),
in jobs with environment `release*` in any spelling outputs of other jobs outside the `if:`
(allow list D-W-61, D-W-72, rule `release-untrusted-output`) and artifacts not fetched by ID
(`release-artifact-by-id`).

## Pros and Cons of the Options

### Only GitHub-hosted runners, public from the first push

* Good, because it is simple and secure.
* Good, because the standard runners are free in public repositories (see above).
* Bad, because no CI runs before the publication.

### Self-hosted runners while private, then GitHub-hosted runners

* Good, because CI would already run before the publication.
* Bad, because a switch stands in every job, runners have to be set up, separated and removed
  before switching to public, and a forgotten step brings other people's code onto a
  machine of the maintainers.

### Always self-hosted runners

* Bad, because it is insecure in the public repository (see above).

## More Information

Procedure of the publication: [public-switch.md](../quality/public-switch.md). Local gates:
[ADR 0038](0038-local-gates-first-one-command-identical-to-ci.md).
