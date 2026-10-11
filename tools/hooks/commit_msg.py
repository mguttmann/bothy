# SPDX-License-Identifier: Apache-2.0
"""Check commit messages: Conventional Commits 1.0 header and DCO sign-off.

Usage:
  commit_msg.py [--require-signoff] MSGFILE      commit-msg hook (one message)
  commit_msg.py [--require-signoff] --range A..B check every non-merge commit in a range

Rules (docs/quality/push-discipline.md):
  * header `type(scope)!: description`, type from TYPES, at most HEADER_MAX_LENGTH chars
  * a blank line between header and body
  * with --require-signoff: a `Signed-off-by:` trailer whose e-mail matches the author
  * `fixup!`/`squash!`/`amend!` commits are fine locally and rejected in ranges,
    because they must be squashed before a push
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
# Named value `commit.header_max_length` (docs/spec/23-qualitaet.md).
HEADER_MAX_LENGTH = 100
_HEADER = re.compile(
    r"^(?P<type>[a-z]+)(?:\((?P<scope>[a-z0-9][a-z0-9._/-]*)\))?(?P<bang>!)?: (?P<desc>\S.*)$"
)
_HEADER_NO_DESC = re.compile(r"^[a-z]+(?:\([^)]*\))?!?:\s*$")
_SIGNOFF = re.compile(r"^Signed-off-by: .+ <(?P<email>[^>]+)>\s*$", re.MULTILINE)
_EMAIL = re.compile(r"<([^>]+)>")
_SCISSORS = "# ------------------------ >8 ------------------------"
_AUTOSQUASH = ("fixup! ", "squash! ", "amend! ")


def _clean(message: str) -> str:
    if _SCISSORS in message:
        message = message.split(_SCISSORS, 1)[0]
    lines = [ln for ln in message.splitlines() if not ln.startswith("#")]
    return "\n".join(lines).strip("\n")


def check_message(
    message: str, *, require_signoff: bool, author: str, allow_fixup: bool = True
) -> list[str]:
    """Return a list of human readable errors; empty means the message passes."""
    text = _clean(message)
    lines = text.splitlines()
    header = lines[0] if lines else ""
    errors: list[str] = []
    if header.startswith("Merge "):
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
    parser.add_argument("msgfile", nargs="?", type=Path)
    args = parser.parse_args(argv)

    if args.rev_range:
        failed = 0
        for sha, author, body in _range_commits(args.repo, args.rev_range):
            errors = check_message(
                body, require_signoff=args.require_signoff, author=author, allow_fixup=False
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
    )
    for err in errors:
        print(f"commit message: {err}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
