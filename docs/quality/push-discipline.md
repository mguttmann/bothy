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
7. **CI.** `ci.yml` runs the same make targets. The only required check: `ci-ok`. It
   starts with every run and waits for the other jobs (`tools/ci/ci_ok.py`), so it is
   pending from the first second of a run.
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
- **No stale green.** `ci-ok` has no `needs`: a job with `needs` gets its check run only
  when the other jobs are done, so after an edit the older green `ci-ok` of the same head
  commit would stay the latest result for the whole new run. Instead the `ci-ok` job starts
  with the run, is pending at once, and `tools/ci/ci_ok.py` reads the jobs of its run through
  the API (`actions: read`) and combines their results.
- **Rules.** The body is checked like a commit message: every line gets the private and
  the generic rules. A `Signed-off-by` or `Co-authored-by` line of the exact form
  `Key: Name <local@domain>` (a plain address) gets the private list as a whole and the
  generic rules on the line without the local part of the address, so the name and the
  domain are still checked; a trailer line with any other text is ordinary text and gets
  all rules. In the body,
  `Signed-off-by` passes only with a full identity from `allow-authors.txt`
  (`Name <email>`); in `Co-authored-by` or in the text an allowed identity is a finding, as
  in a commit.
- **Locally.** `make denylist` (and so `make check` and the pre-push hook) reads the same
  variables. Before the push, check the planned squash message:
  `PR_TITLE="feat(x): ..." PR_BODY_FILE=<file> make denylist`, or `PR_BODY` with
  the text itself. A `PR_BODY_FILE` that cannot be read fails the scan.
- **Bots.** CodeRabbit writes its summary into the walkthrough comment, not into the body
  (`high_level_summary_in_walkthrough` in `.coderabbit.yaml`, schema at
  https://coderabbit.ai/integrations/schema.v2.json, retrieved 2026-10-12), so bot text
  does not reach `main` and does not restart CI.

What the gate does not cover, so `denylist-main.yml` stays the check after the merge (it
scans what actually reached `main`):

- the seconds between an edit and the start of the new run (GitHub delivers the event);
- a manual `workflow_dispatch` of `ci.yml` on the pull request branch or "Re-run all jobs"
  of a run from before the edit: both run without the current text (maintainers only);
- a re-run of a single job: `ci-ok` has no `needs`, so it is not re-run with it
  (maintainers only; re-run all jobs instead);
- a run that waits for a maintainer's approval (first contribution from a fork) has no
  `ci-ok` yet, so the older result stays the latest until the maintainer approves;
- a message typed into the merge dialog instead of the pull request body.

## Dependency bot pull requests

Dependabot (`dependabot[bot]`) and Renovate (`renovate[bot]`) open pull requests from branches
of this repository, but a run that Dependabot starts gets no repository secrets: GitHub treats
it like a run from a fork
(https://docs.github.com/en/code-security/dependabot/troubleshooting-dependabot/troubleshooting-dependabot-on-github-actions,
retrieved 2026-10-13) [verified]. Bot commits also carry no DCO sign-off of their author.
So that such a pull request passes for the right reasons, and only such a pull request
(D-DEPS-3 in [DECISIONS](../DECISIONS.md#10-rulings-of-the-build-milestones)):

- **Secret checks.** `tools/ci/pr_mode.py` (job `plan` of `ci.yml`) reduces the run when the
  pull request author and the actor of the run are both one of the two bot logins, exactly as
  GitHub reports them in the event. The run is then handled like a fork: the job
  `denylist (generic rules, fork)` scans with the generic rules, `sonar` is skipped, and the
  run shows the notice "Reduced checks" with the reason. A run that a person starts on the
  same pull request (an edit of title or body, a push to the bot branch) gets the private list
  and SonarQube with the secrets, and a missing secret fails it.
- **Sign-off.** CI passes the pull request author to `make commitlint` (`PR_AUTHOR`). In a pull
  request by one of the bots, a commit whose author e-mail is exactly that bot's noreply
  address needs no `Signed-off-by` (`BOT_AUTHORS` in `tools/hooks/commit_msg.py`); the check
  prints one line per exempt commit. The Conventional Commits rules still apply to these
  commits and to the title. Every other commit, also a maintainer's commit on the bot
  branch, still needs its sign-off; a bot address in a person's pull request or in a fork
  exempts nothing, because only GitHub sets the pull request author. Locally `PR_AUTHOR` is
  unset, so `make check` exempts nothing.
- **Before the merge.** The maintainer replaces the bot's body with a short text of their own
  (what changed and how it was verified) that ends with their `Signed-off-by` line, and
  checks that the title is a Conventional Commit header. The squash commit on `main` takes
  title and body, so it carries the maintainer's sign-off, and the edit starts a run with every
  check, which `ci-ok` must pass before the merge. After the merge, `denylist-main.yml` scans
  the pushed commit with the private list as for every push.

What this does not cover: nothing enforces the maintainer's rewrite of the body; as with a
fork pull request, a merge straight after a reduced run has not seen the private list or
SonarQube before the merge, and only `denylist-main.yml` checks afterwards. Anyone with write
access can push a commit with a forged bot author address to a bot branch, and that commit
then needs no sign-off; the push is a run of a person, so it gets every other check, and the
squash commit on `main` carries the maintainer's sign-off from the body.

## Deny-list hit on `main`

`denylist-main.yml` checks every push to `main` with the private list (D-W-34). If it
fails, a term of the list has reached `main`, for example from a
fork contribution or from a commit message changed during the squash (see the list of
what the pull request text gate does not cover). Flow:

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
