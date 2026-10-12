# Deny-list scanner

Finds instance-specific terms before they reach a public repository
(ADR 0042). Run it through `make denylist` (part of `make check`) or
`make denylist-history` (whole history, before switching the repository to public).

## Set up the list (maintainer)

The private list lives outside this repository. One of the two options:

```sh
export BOTHY_DENYLIST_FILE=/path/to/denylist.txt
# or permanently for this clone (the file is git-ignored):
echo /path/to/denylist.txt > .denylist-path
```

A relative path in `.denylist-path` is relative to the repository. Without a list, the
scan fails. Allowed commit identities are in `allow-authors.txt` next to the list
(or `BOTHY_DENYLIST_ALLOW_AUTHORS`).

## Format

```text
# comment
a-literal-term
re:\bregular[- ]expression\b
@exclude-path: path/with/*glob
```

One rule per line, case-insensitive. `re:` starts a regular expression
in Python syntax. Short terms are better written as an expression with word boundaries `\b`,
otherwise they match parts of other words. `allow-authors.txt`: one identity per line as
`Name <email>` or just `<email>`.

## What is checked

| Location | Default | `--history` |
|---|---|---|
| Content and name of every tracked file (binary files too; for PNG everything except the pixel data (`IDAT`), because compressed pixels produce random matches; `zTXt`, `iTXt` and `iCCP` are also decompressed; a damaged PNG is checked raw) | yes | yes, plus every earlier object and every earlier path |
| Commit messages | commits in `--range` (default: not yet on `origin/main`, without a remote all of them) | all commits of all refs |
| Author, committer and the identity lines `Signed-off-by` and `Co-authored-by` | like messages, only against the private list | yes |
| Branch, remote and tag names, tag messages | yes | yes |
| Additional names (`--name`, `BOTHY_DENYLIST_NAMES`) | yes | yes |
| Pull request title (`PR_TITLE`) and body (`PR_BODY`, or a file in `PR_BODY_FILE`), the planned squash commit on `main`; the body like a commit message, `Signed-off-by` passes only with a full allowed identity `Name <email>` (with `--allow-missing` the generic rules check every line, trailers on their name part) | when set | when set |

The public rules in `generic.rules` always run, but do not check
commit identities: contributors sign off with their own address (DCO). An identity line
counts only in the exact form `Signed-off-by: Name <local@domain>` (or `Co-authored-by`) with
a plain address; the generic rules still check the line without the local part, so name and
domain are checked, and a line with any other text is ordinary message text. With
`--allow-missing` (only fork pull requests in CI), only they run. Other lines such as
`Reviewed-by` are normal message text and get all rules.

The allowed identities from `allow-authors.txt` apply only as author and committer and in
a `Signed-off-by` line that repeats exactly the author or committer of the same commit
(`git commit -s`). In `Co-authored-by`, in a sign-off by someone else and in the text, the
private list reports them like any other match (requirement 22, D-W-06, D-W-37).

## Output

```text
docs/x.md:2:7: content matches private rule line 7
path ************-notes.md: path matches private rule line 5
```

Only the location and the line of the rule, never the match; matches in paths are masked.
`--reveal` shows the match (only locally, never in CI). Exit codes: 0 clean, 1 findings,
2 configuration error (missing list, broken expression).

## Fixing

* Match in the current state: replace the term (example domains example.com, people
  Alex, Sam, Robin).
* Match in commit messages or commit identities of commits not yet pushed:
  `git commit --amend` or a rebase with a new message or identity.
* Match in old history: rewrite it as described in docs/quality/public-switch.md.
