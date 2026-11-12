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
| Content and name of every tracked file (binary files too; for PNG everything except the pixel data (`IDAT`), because compressed pixels produce random matches; `zTXt`, `iTXt` and `iCCP` are also decompressed; a damaged PNG is checked raw) | yes | yes, plus every earlier object with the path it is listed under (limit below) |
| Commit messages | commits in `--range` (default: not yet on `origin/main`, without a remote all of them) | all commits of all refs |
| Content and name of every file that a commit adds or changes, read from the commit (a merge commit: what differs from all of its parents) | like messages | no, the first row reads the objects of all refs instead (limit below) |
| Author, committer and the identity lines `Signed-off-by` and `Co-authored-by` | like messages; author and committer only against the private list, identity lines also against the generic rules without the local part of the address | yes |
| Branch, remote and tag names, tag messages | yes | yes |
| Additional names (`--name`, `BOTHY_DENYLIST_NAMES`) | yes | yes |
| Pull request title (`PR_TITLE`) and body (`PR_BODY`, or a file in `PR_BODY_FILE`), the planned squash commit on `main`; the body like a commit message, `Signed-off-by` passes only with a full allowed identity `Name <email>` (with `--allow-missing` the generic rules check every line, identity lines without the local part of the address) | when set | when set |

A commit stays readable on the remote after a later commit removed what it added: a term that
one commit of a branch adds and the next one deletes is public as soon as both are pushed. So the
default scan reads the files of every commit of the range from the commits themselves, with the
private and the generic rules and the `@exclude-path` globs, and reports them as `commit-content`
and `commit-path` with the first commit of the range that holds the file. What the scan of the
tracked files already read under the same name is not reported twice; an edit of the working
tree that is not committed does not hide what the commit holds. A file that a merge commit takes
unchanged from one of its parents belongs to that parent: a commit of the range, which is
checked itself, or a commit outside it, which is published already. A scan without a range (no
remote) reads the files of every commit of `HEAD` and takes accordingly longer.

Limit of `--history`: it reads every object once, with the name `git rev-list --objects --all`
lists it under. A name that only an earlier commit used for a file whose content did not change
(a rename) is therefore not read by `--history`; the default scan reads it for the commits of
its range.

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
commit 0123456789ab docs/x.md:2:7: commit-content matches private rule line 7
commit 0123456789ab path ************-notes.md: commit-path matches private rule line 5
```

Only the location and the line of the rule, never the match; matches in paths are masked.
`--reveal` shows the match (only locally, never in CI). Exit codes: 0 clean, 1 findings,
2 configuration error (missing list, broken expression).

`--summary FILE` (or `BOTHY_DENYLIST_SUMMARY`) also writes a JSON summary: exit code, number
of findings per kind and the masked locations without column, rule or match. The workflow
`denylist-main.yml` puts it into the issue of a red run (`main_alert.py`); `main_range.py`
picks the commits that workflow scans (docs/quality/push-discipline.md, section "Deny-list hit
on `main`").

## Fixing

* Match in the current state: replace the term (example domains example.com, people
  Alex, Sam, Robin).
* Match in commit messages or commit identities of commits not yet pushed:
  `git commit --amend` or a rebase with a new message or identity.
* Match in a file of a commit not yet pushed (`commit-content`, `commit-path`): a later commit
  that removes the term does not help, the named commit still holds it. Squash or rebase the
  branch so that no commit holds it (docs/quality/push-discipline.md, "Squash before the push").
  If the commit is pushed already, the term is public: inform the owner.
* Match in old history: rewrite it as described in docs/quality/public-switch.md.
