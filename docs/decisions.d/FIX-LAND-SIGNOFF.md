# FIX-LAND-SIGNOFF Squash commits on main with the checked text and one sign-off

Process failure (push-discipline.md, "Deny-list hit on `main`"): two squash commits of
2026-11-25 and 2026-11-26 carry a sign-off wrapped over two lines, which the deny list reads as
message text, and three of 2026-11-18 and 2026-11-22 a second sign-off with the earlier commit
identity. Cause, read on the commits and on GitHub: they did not get the pull request body that
CI checked, but GitHub's default squash text (`viewerMergeBodyText`, the text of the merge
dialog, which `gh pr merge --squash` without a body also takes). That text wraps the body at 72
columns and appends the trailers of the branch commits after a `---------` line; the branch
commits of the three older pull requests were signed with the identity of the machine's git
configuration.

| No | Date | Ruling | Reason |
|---|---|---|---|
| FIX-LAND-SIGNOFF/1 | 2026-11-28 | A pull request lands only through `tools/hooks/land_pr.py`: the title plus ` (#<number>)` as subject, the pull request body without the template's HTML comments as `--body-file` (only comments on lines of their own are removed, so no text is joined that CI scanned apart; any other comment refuses the landing), the address of `BOTHY_SIGNOFF_IDENTITY` as author, `--match-head-commit`; after the merge the message of the commit on `main` is compared with the checked text (exit 2 on a difference). | A merge with an explicit body keeps the text as it is (the squash commit of #1796 has lines of 98 characters and nothing appended); only GitHub's own default composes. |
| FIX-LAND-SIGNOFF/2 | 2026-11-28 | `check_squash_body` (`make commitlint`, `PR_BODY`): the last line is the only `Signed-off-by` line, in one line, with a GitHub no-reply address; `land_pr.py` also requires the exact identity of `BOTHY_SIGNOFF_IDENTITY`, for bot pull requests too (for bot pull requests replaced by FIX-LAND-IDENTITY/1). The error never repeats the rejected line. | The wrapped default text and the second identity passed the old check, because their last line was a correct sign-off. CI logs are public. |
| FIX-LAND-SIGNOFF/3 | 2026-11-28 | The identity is not named in the repository; the maintainer's environment sets it. The commits already on `main` stay as they are. | The repository names no personal identity (Decided, "Commit identity in the public repository"); only the owner decides a rewrite of the public history. |

## Open

- A merge with the button on the pull request page still takes GitHub's default text; nothing in
  the repository can stop it, `denylist-main.yml` reports it afterwards.
- The squash of a merge queue (inactive) composes its text on GitHub's side; it is not checked
  by `land_pr.py`.
