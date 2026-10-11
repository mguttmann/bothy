# Push discipline

Goal: failures surface before the push. `main` contains only finished, checked features, one
commit per feature, signed and linear.

## Flow

1. **Branch.** Create a branch `<type>/<topic>` from the current `main`. Never work on
   `main`.
2. **Work and commit.** Small commits are allowed, also `fixup!` commits; the
   pre-commit hook checks every change quickly, the commit-msg hook checks format and DCO
   (`git commit -s`).
3. **Squash before the push.** Rebase onto `main`, squash intermediate states into meaningful
   Conventional Commits (`git rebase -i main` in the terminal). No commit
   is called "fix CI", "typo" or "wip".
4. **Local review before every push.** An independent, read-only reviewer checks the
   diff of the squashed branch against `main` according to
   [red-team-gate.md](red-team-gate.md) and writes the report to
   `.review/<sha>.md` (git-ignored; `<sha>` is the commit that is pushed); its text goes
   into the section "Review report" of the pull request. The last
   line is `Urteil: JA` or `Urteil: NEIN <reason>`. If a finding changes the state, the
   reviewer checks the delta, and the report applies to the new commit. `make review` creates the
   folder, shows the expected file name and optionally starts the CodeRabbit CLI (only
   with `BOTHY_CODERABBIT=1`, see below). At the end of a milestone, the overall review
   over the whole milestone is added.
5. **Push.** The pre-push hook first checks the report (`tools/hooks/review_gate.py`:
   file present, last line `Urteil: JA`), then it runs `make check`. Red means:
   fix, do not bypass. `git push --no-verify` is a process failure. Contributors from
   forks set `REVIEW_SKIP=1`; then a maintainer checks before the merge.
6. **Pull request.** One pull request per finished feature. The title is a
   Conventional Commit header line, because on merge it becomes the header line of the commit on `main`;
   CI checks it (`PR_TITLE` in `make commitlint`).
7. **CI.** `ci.yml` runs the same make targets. The only required check: `ci-ok`.
8. **Merge.** "Squash and merge". GitHub signs the resulting commit; the
   `Signed-off-by` lines of the individual commits are in the commit message. `main` stays linear.

## Protection of `main` (ruleset)

| Rule | Setting | Reason |
|---|---|---|
| Require a pull request before merging | on; allowed merge method squash only | no direct pushes, one commit per feature |
| Require status checks to pass | `ci-ok` | one check combines all jobs; skipped component jobs do not block |
| Require linear history | on | readable history, simple bisects |
| Require signed commits | on | provenance of every commit on `main` |
| Block force pushes | on | history stays immutable |
| Restrict deletions | on | `main` cannot be deleted |

Rationale for squash instead of rebase merge: with a rebase merge, GitHub creates new commits
that it cannot sign; signed commits and rebase merge through the interface
are therefore mutually exclusive
(https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/about-merge-methods-on-github,
retrieved 2026-10-10) [verified]. GitHub signs the squash commit
(https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets,
retrieved 2026-10-10) [verified]. Rulesets are available for private repositories from GitHub Pro
on [verified, research source
https://raw.githubusercontent.com/github/docs/main/data/reusables/gated-features/repo-rules.md,
retrieved 2026-10-10]. The rule "Required reviewers" for teams does not exist in personal
repositories; in single-person operation, the independent review before every
push (step 4) replaces the second review.

## Failures after the push

A red CI run after a green `make check` means: local and CI are not the same. That
is a failure of the toolchain, not bad luck. Flow:

1. Determine the cause and note it in the pull request.
2. Close the gap locally (gate, test, pin), in the same or in a separate pull request.
3. A commit "fix CI" or "typo" on `main` is a process failure: short analysis as an
   entry in `docs/DECISIONS.md` (date, cause, new rule).

Findings after the push are acceptable only from checkers that cannot run locally:
SonarQube Cloud (for example Dart rules) and CodeRabbit. Every such finding becomes
a local rule: Semgrep rule, test, linter setting or profile rule in
`quality/sonar/`.

## Deny-list hit on `main`

`denylist-main.yml` checks every push to `main` with the private list (D-W-34). If it
fails, a term of the list has reached `main`, for example from a
fork contribution or from a commit message changed during the squash. Flow:

1. Look at the hit locally with `make denylist DENYLIST_RANGE=<before>..<after> DENYLIST_FLAGS=--reveal`
   (locally only, never in CI).
2. The repository is public (ADR 0039), so the term is published: remove it, inform the owner,
   analysis as an entry in `docs/DECISIONS.md` as for a process failure. Only the owner decides
   whether the public history is rewritten.

## CodeRabbit CLI and data outflow

The CodeRabbit CLI checks local changes through the CodeRabbit service; the
documentation names a check of the reachability of the backend (`cr doctor`) and no
purely local execution (https://docs.coderabbit.ai/cli/overview, retrieved 2026-10-10)
[derived]. So the diff leaves the host. That is why it is off until
`BOTHY_CODERABBIT=1` is set. In the private state, the owner decides whether private
code may go to the service (open question). In the public state, the code is
public anyway; CodeRabbit then also rates every pull request
(`.coderabbit.yaml`). According to the vendor, the free tier of the CLI is limited
("Basic static analysis with limited daily usage", same source) [verified].
