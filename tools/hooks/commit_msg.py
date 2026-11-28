# SPDX-License-Identifier: Apache-2.0
"""Check commit messages: Conventional Commits 1.0 header and DCO sign-off.

Usage:
  commit_msg.py [--require-signoff] MSGFILE      commit-msg hook (one message)
  commit_msg.py [--require-signoff] --range A..B [--pr-author LOGIN]
                                                 check every non-merge commit in a range
  commit_msg.py --pr-body FILE --pr-author LOGIN  the pull request body (squash text)

Rules (docs/quality/push-discipline.md):
  * header `type(scope)!: description`, type from TYPES, at most HEADER_MAX_LENGTH chars
  * a blank line between header and body
  * with --require-signoff: a `Signed-off-by:` trailer whose e-mail matches the author
  * the message of a merge in progress (MERGE_HEAD in the git directory, as `git merge`
    leaves it for the hook) is not checked; a commit whose header merely starts with
    `Merge `, a commit of a range (merges are left out there) and the pull request title are
    checked like any other message
  * `fixup!`/`squash!`/`amend!` commits are fine locally and rejected in ranges,
    because they must be squashed before a push
  * with --pr-author naming a dependency bot of BOT_AUTHORS (CI passes the pull request
    author GitHub reports): that bot's own commits need no sign-off; the header rules still
    apply, and every other commit of the range still needs its sign-off (D-DEPS-3)
  * --pr-body: the body of the pull request becomes the message of the squash commit on
    `main`, so its last line outside HTML comments is `Signed-off-by: Name <e-mail>` (git
    reads a sign-off only in the trailer block at the end of a message), unless
    the author is a bot of PR_BODY_BOTS (#872). That line is the only `Signed-off-by` line of
    the body, in one line, with a GitHub no-reply address (owner decision of 2026-11-07,
    OQ-25): a sign-off wrapped over two lines or a second identity fails. The landing step
    (land_pr.py) also requires the exact identity of the maintainer who lands.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

TYPES = (
    "build",
    "chore",
    "ci",
    "docs",
    "feat",
    "fix",
    "perf",
    "refactor",
    "revert",
    "style",
    "test",
)
# Named value `commit.header_max_length` (docs/spec/23-quality.md).
HEADER_MAX_LENGTH = 100
_HEADER = re.compile(
    r"^(?P<type>[a-z]+)(?:\((?P<scope>[a-z0-9][a-z0-9._/-]*)\))?(?P<bang>!)?: (?P<desc>\S.*)$"
)
_HEADER_NO_DESC = re.compile(r"^[a-z]+(?:\([^)]*\))?!?:\s*$")
_SIGNOFF = re.compile(r"^Signed-off-by: .+ <(?P<email>[^>]+)>\s*$", re.MULTILINE)
_EMAIL = re.compile(r"<([^>]+)>")
_SCISSORS = "# ------------------------ >8 ------------------------"
_AUTOSQUASH = ("fixup! ", "squash! ", "amend! ")
# Dependency bots whose own commits need no DCO sign-off in their own pull requests: the
# pull request author login GitHub reports -> the commit author e-mail the bot writes.
# tools/ci/pr_mode.py routes the same logins (a test keeps both lists equal).
BOT_AUTHORS = {
    "dependabot[bot]": "49699333+dependabot[bot]@users.noreply.github.com",
    "renovate[bot]": "29139614+renovate[bot]@users.noreply.github.com",
}
# Pull request authors whose body needs no sign-off: the dependency bots and the bot of the
# repository's own workflows (hermes-pin.yml opens its pull requests as github-actions[bot]).
PR_BODY_BOTS = frozenset({*BOT_AUTHORS, "github-actions[bot]"})
_HTML_COMMENT = re.compile(r"<!--.*?-->", re.DOTALL)
# GitHub's no-reply address of an account: `<id>+<login>@users.noreply.github.com`.
NOREPLY = re.compile(r"^\d+\+[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?@users\.noreply\.github\.com$")


def check_squash_body(body: str, *, identity: str | None = None) -> list[str]:
    """Return the errors of the text that becomes the squash commit message below its header.

    It ends with exactly one `Signed-off-by: Name <address>` line, in one line, with a GitHub
    no-reply address, and with `identity` ("Name <address>") when it is given. HTML comments of
    the template never reach the commit and do not count. The errors never repeat a rejected
    line: CI logs are public.
    """
    text = _HTML_COMMENT.sub("", body.replace("\r\n", "\n"))
    lines = [ln.strip() for ln in text.splitlines() if ln.strip()]
    last = lines[-1] if lines else ""
    match = _SIGNOFF.fullmatch(last)
    errors: list[str] = []
    if match is None:
        errors.append(
            "the body must end with a 'Signed-off-by: Name <e-mail>' line, in one line: it "
            "becomes the squash commit on main (CONTRIBUTING.md, Pull requests)"
        )
    elif identity is not None and last != f"Signed-off-by: {identity}":
        errors.append(f"the last line must be exactly 'Signed-off-by: {identity}'")
    elif not NOREPLY.match(match.group("email")):
        errors.append("the sign-off must use a GitHub no-reply address (OQ-25)")
    if any(ln.lower().startswith("signed-off-by") for ln in lines[:-1]):
        errors.append(
            "the last line must be the only 'Signed-off-by' line: a second identity or a "
            "sign-off wrapped over two lines reaches main"
        )
    return errors


def check_pr_body(body: str, *, author: str) -> list[str]:
    """Return the errors of a pull request body; empty means it passes (#872).

    The body becomes the message of the squash commit on `main`, so it needs a DCO sign-off
    of its own (check_squash_body); a body of a bot of PR_BODY_BOTS needs none.
    """
    if author in PR_BODY_BOTS:
        return []
    return check_squash_body(body)


def _clean(message: str) -> str:
    if _SCISSORS in message:
        message = message.split(_SCISSORS, 1)[0]
    lines = [ln for ln in message.splitlines() if not ln.startswith("#")]
    return "\n".join(lines).strip("\n")


def check_message(
    message: str,
    *,
    require_signoff: bool,
    author: str,
    allow_fixup: bool = True,
    merge: bool = False,
) -> list[str]:
    """Return a list of human readable errors; empty means the message passes.

    `merge` is true only for the message of a real merge commit (`_merging`); the header text
    alone never exempts a message.
    """
    text = _clean(message)
    lines = text.splitlines()
    header = lines[0] if lines else ""
    errors: list[str] = []
    if merge:
        return errors
    if header.startswith(_AUTOSQUASH):
        if not allow_fixup:
            errors.append(f"'{header}': fixup/squash commits must be squashed before pushing")
        return errors
    match = _HEADER.match(header)
    if not match or match.group("type") not in TYPES:
        if _HEADER_NO_DESC.match(header):
            errors.append(f"'{header}': description is missing after 'type: '")
        else:
            errors.append(
                f"'{header}': header must be 'type(scope): description' with type one of "
                + ", ".join(TYPES)
            )
    if len(header) > HEADER_MAX_LENGTH:
        errors.append(f"header is longer than {HEADER_MAX_LENGTH} characters")
    if len(lines) > 1 and lines[1].strip():
        errors.append("the body must be separated from the header by a blank line")
    if require_signoff:
        want = _email(author)
        signed = {m.group("email").lower() for m in _SIGNOFF.finditer(text)}
        if want not in signed:
            errors.append(
                f"missing 'Signed-off-by:' for the author <{want}> "
                "(commit with -s, see CONTRIBUTING.md)"
            )
    return errors


def _email(identity: str) -> str:
    m = _EMAIL.search(identity)
    return (m.group(1) if m else identity).strip().lower()


def _git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout


def _merging(repo: Path) -> bool:
    """True while `git merge` waits for its commit: the hook then sees MERGE_HEAD."""
    try:
        path = Path(_git(repo, "rev-parse", "--git-path", "MERGE_HEAD").strip())
    except subprocess.CalledProcessError:
        return False
    return (path if path.is_absolute() else repo / path).is_file()


def _author_ident(repo: Path) -> str:
    # `git var` prints "Name <email> timestamp tz".
    return _git(repo, "var", "GIT_AUTHOR_IDENT").rsplit(" ", 2)[0]


def _range_commits(repo: Path, rev_range: str) -> list[tuple[str, str, str]]:
    out = _git(repo, "log", "--no-merges", "--format=%H%x1f%an <%ae>%x1f%B%x1e", rev_range)
    commits = []
    for raw_record in out.split("\x1e"):
        record = raw_record.strip("\n")
        if record:
            sha, author, body = record.split("\x1f", 2)
            commits.append((sha, author, body))
    return commits


def main(argv: list[str] | None = None) -> int:
    """Check a message file or a commit range; returns 1 on a format or sign-off problem."""
    parser = argparse.ArgumentParser(description="Check Conventional Commits and DCO sign-off.")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--require-signoff", action="store_true")
    parser.add_argument("--range", dest="rev_range")
    parser.add_argument(
        "--pr-author",
        default="",
        help="login of the pull request author; exempts a dependency bot's own commits "
        "from the sign-off",
    )
    parser.add_argument("--pr-body", type=Path, help="file with the pull request body")
    parser.add_argument("msgfile", nargs="?", type=Path)
    args = parser.parse_args(argv)

    if args.pr_body is not None:
        body_errors = check_pr_body(args.pr_body.read_text(encoding="utf-8"), author=args.pr_author)
        for err in body_errors:
            print(f"pull request body: {err}")
        return 1 if body_errors else 0

    if args.rev_range:
        failed = 0
        bot_email = BOT_AUTHORS.get(args.pr_author)
        for sha, author, body in _range_commits(args.repo, args.rev_range):
            require_signoff = args.require_signoff
            if require_signoff and bot_email is not None and _email(author) == bot_email:
                print(
                    f"{sha[:12]}: sign-off not required: commit by {args.pr_author} "
                    "in its own pull request"
                )
                require_signoff = False
            errors = check_message(
                body, require_signoff=require_signoff, author=author, allow_fixup=False
            )
            for err in errors:
                print(f"{sha[:12]}: {err}")
            failed += bool(errors)
        return 1 if failed else 0
    if args.msgfile is None:
        parser.error("either MSGFILE or --range is required")
    # Only the sign-off check needs the author; the pull request title is checked on a CI
    # runner, which has no git identity.
    errors = check_message(
        args.msgfile.read_text(encoding="utf-8"),
        require_signoff=args.require_signoff,
        author=_author_ident(args.repo) if args.require_signoff else "",
        merge=_merging(args.repo),
    )
    for err in errors:
        print(f"commit message: {err}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
