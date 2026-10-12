# Checklist: publishing

The product repository is public from the first push
([ADR 0039](../adr/0039-only-github-hosted-runners-public-from-the-first-push.md)); every
workflow runs on GitHub-hosted runners. This list is worked through completely and in this
order, every item with evidence (command output or screenshot in the
ops repository): first check locally, then push and make public right away, then connect SonarQube
Cloud and CodeRabbit, then protect `main`. After that, each ticket comes as its own pull
request.

## A. Before the push

1. **Red-team gate** over the whole state with the verdict `JA` (red-team-gate.md).
2. **One root commit with the chosen identity.** What is published is a single commit
   without parents with the checked state; the planning history stays private (D-W-93). Rewriting the
   history with `git filter-repo` (PLAN M0-08, D-W-48) is only needed if a history
   is to be published after all. The owner chose the commit identity; it is entered in
   `allow-authors.txt` of the private list and allowed there only in the author and
   committer fields (and in a `Signed-off-by` line that repeats exactly this identity, D-W-37).
   Every identity in `allow-authors.txt` that is not the chosen one is removed before
   step 3 runs; only then does a scan without a hit prove that no other identity is in the
   commit.
3. **Scans** in a fresh clone of only this commit, with the private list: `make denylist`,
   `make denylist-history` and `make gitleaks GITLEAKS_LOG_OPTS=--all` are green. Every hit
   is removed from the state, then the commit is rebuilt and both scans run again.
4. **Rotate secrets** that were ever in the repository or in CI logs.
5. **License gate** green, `NOTICE` complete, ScanCode run over the whole tree without an
   unexpected license (licenses.md).
6. **Create the environment `denylist`** in the still empty repository: secrets `BOTHY_DENYLIST` and
   `BOTHY_DENYLIST_ALLOW_AUTHORS`, reachable for pull requests from the repository and for
   `main`. `denylist-main.yml` already runs on the push (step 7); if the allow-list of
   authors is missing, its log can show that the author identity matches a rule of the private list,
   and this log is public afterwards.

## B. Push and public right away

7. **Push** the commit as `main` into the empty repository. There must be no `main` yet:
   an existing `main` accepts a commit without parents only by force push.
8. **Public right away:** Repository, Settings, General, Danger Zone: visibility "Public". In
   the same move, set the repository variable `BOTHY_VISIBILITY` to `public` (scheduled runs of
   CodeQL, Scorecard and Pages, ADR 0039 rule 1).
9. **Enable private vulnerability reporting** (Settings, Advanced Security), in the same
   move as step 8: `SECURITY.md` and `CODE_OF_CONDUCT.md` name only this reporting channel, it
   must work from the switch on. Cross-check: start a new issue; the chooser
   links `SECURITY.md` and `SUPPORT.md` from the repository [assumed, not checked
   offline]. If one of the two links is missing, `.github/ISSUE_TEMPLATE/config.yml` gets, by
   pull request, an entry under `contact_links` with `name`, `about` and the absolute URL of the
   file; the entry must pass `make denylist`, and the header comment of the file is
   adjusted.
10. **Enable secret scanning and push protection** (Settings, Advanced Security);
    free for public repositories
    (https://docs.github.com/en/code-security/secret-scanning/introduction/about-push-protection,
    retrieved 2026-10-10) [verified for secret scanning; push protection at
    repository level free according to the 2023 changelog, not checked again].
11. **Deny scan of the push:** `denylist-main.yml` ran green for the push from step 7.
    If it is not green, find the cause and start it again via `workflow_dispatch`.

## C. SonarQube Cloud and CodeRabbit

12. **SonarQube Cloud**: organization of the account that owns the repository (name in the
    ops repository), with the OSS plan, create the project, automatic analysis off (R-QA-10),
    `make sonar-config` and `make sonar-config-check` against the cloud (sonarqube-local.md,
    section 6). Repository variables `SONAR_HOST_URL` (`https://sonarcloud.io`),
    `SONAR_ORGANIZATION`, `SONAR_PROJECT_KEY`, secret `SONAR_TOKEN`. Without these values, the
    job `sonar` of every pull request from the repository fails.
13. **CodeRabbit**: install it as a GitHub app for the repository; `.coderabbit.yaml` applies.

## D. Branch protection

14. **Rulesets** for `main` according to push-discipline.md (pull request, squash only, required check
    `ci-ok`, linear history, signed commits, no force push, no deletion); in addition
    "Require actions to be pinned to a full-length commit SHA" in the Actions settings and,
    in the general settings, the squash commit message "Pull request title and description"
    (push-discipline.md;
    [configuring commit squashing](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/configuring-commit-squashing-for-pull-requests),
    retrieved 2026-10-11).
15. **Environments** `release`, `release-apple` and `release-windows` with required reviewers and
    tag rule `*/v*` and `v*`; `github-pages` for the docs.

## E. Afterwards: each ticket as its own pull request

Each ticket comes in order as its own pull request according to push-discipline.md onto `main`.

16. **Enable Renovate** (ADR 0041) and check the Dependency Dashboard.
17. **Pages**: set the source to "GitHub Actions"; start `docs.yml` manually.
18. **Trial run:** first start `codeql.yml` on `main` by hand, so that code scanning has an
    analysis of the base branch to compare the pull request results with [assumed, not checked
    offline]. Then the first pull request from the repository (full list):
    all jobs ran on GitHub-hosted runners, `ci-ok` is green, SonarQube Cloud, CodeQL and
    CodeRabbit report, Scorecard starts; the jobs `analyze (actions)`, `analyze (go)` and
    `analyze (python)` are green, and code scanning shows results of the category
    `/language:go`. In addition, a pull request from a fork (generic deny rules, no secret).
19. **Check DCO**: a pull request without sign-off must fail at `ci-ok`.
20. **Release signatures** before the first release tag (build ticket M7-10): the release chain
    signs keyless and with attestations (ADR 0036). The key pair from
    [ADR 0062](../adr/0062-release-signatures-while-the-repo-is-private.md) applies only to
    releases from a private repository and is not created.
21. **Portability test on the public path** with the first product release: first make the
    packages `bothy-server` and `bothy-postgres` in the container registry public
    (package settings, Danger Zone, "Change visibility"). A new package is private, and a
    package linked to the repository inherits "the access permissions (but not the visibility)"
    of the repository
    (https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility,
    retrieved 2026-10-11) [verified]. The step cannot be undone ("Once you make a
    package public, you cannot make it private again", same source) [verified]. Then start
    `portability.yml` via `workflow_dispatch` with the latest product tag. It checks out this
    tag (test code matching the release, [16-operations.md](../spec/16-operations.md) 11.1), fetches
    `install.sh` without login with the documented `curl` command (source `url`,
    [16-operations.md](../spec/16-operations.md) 3.1), downloads the bundle from the address that
    the release set into `install.sh`, and pulls the images that the bundle pins
    ([18-release.md](../spec/18-release.md) 2.1, rules 1 to 3), also without login; a still private
    package makes it fail. It must be green.
22. **Replace the placeholder `OWNER`** (PLAN V1-03): `@OWNER` in `.github/CODEOWNERS`, `OWNER` in
    `mkdocs.yml`, `docs/index.md`, `README.md` and `CONTRIBUTING.md` (`grep -rn OWNER` as a
    cross-check). `make denylist` must be green afterwards; if the private list matches the new path,
    it gets a narrow exception for exactly this path in the ops repository.
