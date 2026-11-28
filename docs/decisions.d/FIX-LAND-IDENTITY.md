# FIX-LAND-IDENTITY A sign-off on main repeats the author GitHub writes

Process failure (push-discipline.md, "Deny-list hit on `main`"), the second of its class after
FIX-LAND-SIGNOFF: `denylist-main` failed on 0117ec48c, the squash commit of #1784, a pull
request of `hermes-pin.yml`. Cause, read on the commit and on GitHub: its message is exactly
the checked pull request body, which ends with the maintainer's sign-off, but GitHub made the
pull request author `github-actions[bot]` the author of the squash commit. The deny list
accepts a sign-off only when it repeats the author or committer of its commit (D-W-37), so the
maintainer's identity in the trailer was checked against the private list. The sign-off had
been added because FIX-LAND-SIGNOFF/2 and push-discipline.md ("Before the merge") asked a
maintainer to sign the body of a bot's pull request; `make commitlint` and `land_pr.py` passed
it. The earlier bot squash commit 77122cffe without a sign-off passed `denylist-main`.

| No | Date | Ruling | Reason |
|---|---|---|---|
| FIX-LAND-IDENTITY/1 | 2026-11-28 | A sign-off in a pull request body signs as the pull request author, the author GitHub gives the squash commit: `check_pr_body` (`make commitlint` in CI, which every merge path passes through `ci-ok`) refuses any `Signed-off-by` or `Co-authored-by` line in the body of a bot of `PR_BODY_BOTS` and a person's sign-off whose no-reply address names another login than `PR_AUTHOR`. `land_pr.py` lands a bot's pull request without a sign-off and without `--author-email`, the maintainer's own pull request with exactly the maintainer's sign-off and address, and refuses one of any other account. After the merge it compares every sign-off of the commit on `main` with its author (exit 2 on a difference). | The rule of `denylist-main` checked before the merge, on every path, instead of after it. The tests read the real commit 0117ec48c and must refuse its body. |
| FIX-LAND-IDENTITY/2 | 2026-11-28 | 0117ec48c stays on `main` as it is; its red `denylist-main` run stays red, and its issue is handled like #1755. | Only the owner decides a rewrite of the public history (FIX-LAND-SIGNOFF/3, #1755). |
