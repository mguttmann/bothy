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
   `.review/<sha>.md` (git-ignored; `<sha>` is the commit that is pushed). The report
   stays local; the pull request names the reviewed commit and the verdict (step 6). The last
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
   CI checks it (`PR_TITLE` in `make commitlint`). The body becomes the message of that
   commit, so title and body are written fresh, in your own words: what and why, how it
   was verified, which commit the independent review covered with which verdict and what
   was fixed. Never paste a raw review report, bot or agent output, local paths, user or
   machine names, temporary folder names or notes about tools. Section
   "Pull request title and body" below describes the gate.
7. **CI.** `ci.yml` runs the same make targets. The only required check: `ci-ok`.
8. **Merge.** "Squash and merge". GitHub signs the resulting commit; its message is the
   pull request body (repository setting below). `main` stays linear.

## Protection of `main` (ruleset)

| Rule | Setting | Reason |
|---|---|---|
| Require a pull request before merging | on; allowed merge method squash only | no direct pushes, one commit per feature |
| Require status checks to pass | `ci-ok` | one check combines all jobs; skipped component jobs do not block |
| Require linear history | on | readable history, simple bisects |
| Require signed commits | on | provenance of every commit on `main` |
| Block force pushes | on | history stays immutable |
| Restrict deletions | on | `main` cannot be deleted |
| Squash commit message (repository setting, not part of the ruleset) | title: pull request title, message: pull request body | the checked title becomes the commit header on `main` |

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

## Pull request title and body

On "Squash and merge", the title becomes the header and the body the message of the commit
on `main`, and a commit on `main` is public and is never rewritten without the owner. So the
text of the pull request goes through the same deny list as every commit:

- **CI.** Both deny-list jobs of `ci.yml` (`denylist` with the private list, `denylist-fork`
  with the generic rules) get title and body from the event payload as the environment
  variables `PR_TITLE` and `PR_BODY`, never interpolated into a script. A finding fails the
  job and with it `ci-ok`. `ci.yml` also runs on `pull_request` type `edited`, so every
  change of title or body is checked again before the merge; an edit restarts the whole
  run, because `ci-ok` must cover the current text and the current code together.
- **Rules.** The body is checked like a commit message: every line gets the private and
  the generic rules, a `Signed-off-by` or `Co-authored-by` line only the private list. An
  allowed identity (`allow-authors.txt`) passes in `Signed-off-by` only; in
  `Co-authored-by` or in the text it is a finding, as in a commit.
- **Locally.** `make denylist` (and so `make check` and the pre-push hook) reads the same
  variables. Before the push, check the planned squash message:
  `PR_TITLE="feat(x): ..." PR_BODY_FILE=<file> make denylist`, or `PR_BODY` with
  the text itself. A `PR_BODY_FILE` that cannot be read fails the scan.
- **Bots.** CodeRabbit writes its summary into the walkthrough comment, not into the body
  (`high_level_summary_in_walkthrough` in `.coderabbit.yaml`), so bot text does not reach
  `main` and does not restart CI.

`denylist-main.yml` stays the check after the merge: it scans what actually reached `main`.

## Deny-list hit on `main`

`denylist-main.yml` checks every push to `main` with the private list (D-W-34). If it
fails, a term of the list has reached `main`, for example from a
fork contribution or from a commit message changed during the squash (an edit of the
body after the last CI run is checked by the `edited` run; a text typed into the merge
dialog is not). Flow:

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
