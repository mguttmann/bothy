# SPDX-License-Identifier: Apache-2.0
"""Land a pull request: squash it with exactly the checked pull request text.

Usage:
  land_pr.py NUMBER [--repo OWNER/NAME] [--delete-branch] [--dry-run]

The squash commit on `main` gets the pull request title plus ` (#NUMBER)` as its header and the
pull request body as its message, without the template's HTML comments: the text that CI
checked (`make commitlint`, `make denylist`). A merge without an explicit body takes GitHub's
default squash text instead, which wraps the body at 72 columns, so the sign-off breaks over
two lines, and appends the trailers of the branch commits after a `---------` line, with
whatever identity those commits carry (docs/quality/push-discipline.md, step 8). So this step
always passes the body as a file and never lets GitHub compose it.

The maintainer who lands sets BOTHY_SIGNOFF_IDENTITY ("Name <address>", a GitHub no-reply
address, OQ-25). GitHub makes the pull request author the author of the squash commit, and
denylist-main accepts a sign-off only when it repeats that author (FIX-LAND-IDENTITY). So a
pull request the maintainer opened must end with exactly `Signed-off-by: ` plus that
identity, the only sign-off line (commit_msg.check_squash_body), and the address is passed as
author; a pull request of a bot of PR_BODY_BOTS lands without any Signed-off-by or
Co-authored-by line (commit_msg.check_pr_body); one of any other account is refused. The
merge is bound to the checked head commit. After the merge the message of the commit on `main`
is read back and compared with the checked text, and its sign-offs with its author; a
difference exits with 2, because a commit on `main` is never rewritten without the owner.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from collections.abc import Callable
from pathlib import Path

from commit_msg import NOREPLY, PR_BODY_BOTS, check_pr_body, check_squash_body

Gh = Callable[[list[str]], str]
_IDENTITY = re.compile(r"^(?P<name>[^<>]*\S) <(?P<address>[^<>\s]+)>$")
IDENTITY_VARIABLE = "BOTHY_SIGNOFF_IDENTITY"
# A comment that fills its lines alone; removing it never joins text that was around it.
_STANDALONE_COMMENT = re.compile(r"^[ \t]*<!--(?:(?!-->)[\s\S])*-->[ \t]*(?:\n|\Z)", re.MULTILINE)
# A Signed-off-by trailer as the deny list reads it on main.
_SIGNOFF_LINE = re.compile(r"^signed-off-by:[ \t]*(?P<identity>.*?)[ \t]*$", re.IGNORECASE)


class LandError(Exception):
    """The pull request cannot be landed as it is."""


def run_gh(args: list[str]) -> str:
    """Run gh and return its standard output; a failure raises CalledProcessError."""
    return subprocess.run(["gh", *args], check=True, capture_output=True, text=True).stdout


def parse_identity(identity: str) -> tuple[str, str]:
    """Return name and address of "Name <address>"; only a GitHub no-reply address passes."""
    match = _IDENTITY.match(identity.strip())
    if match is None or not NOREPLY.match(match.group("address")):
        msg = f"{IDENTITY_VARIABLE} must be 'Name <id+login@users.noreply.github.com>' (OQ-25)"
        raise LandError(msg)
    return match.group("name"), match.group("address")


def pr_author(author: dict[str, object] | None) -> str:
    """The login of the pull request author as CI sees it (`app/x` of gh is `x[bot]`)."""
    login = str((author or {}).get("login") or "")
    if (author or {}).get("is_bot") and login.startswith("app/"):
        return login.removeprefix("app/") + "[bot]"
    return login


def foreign_signoffs(message: str, author: str) -> int:
    """Count the Signed-off-by lines of `message` that do not repeat `author` ("Name <address>").

    denylist-main accepts a sign-off only when it repeats the author or committer of its
    commit; GitHub commits as itself, so the author is the one identity a sign-off can repeat.
    """
    wanted = author.strip().lower()
    return sum(
        1
        for line in message.splitlines()
        if (m := _SIGNOFF_LINE.match(line.strip())) and m.group("identity").lower() != wanted
    )


def normalize(body: str) -> str:
    """The body as git stores it: LF line ends, no trailing blank lines, one final newline."""
    return body.replace("\r\n", "\n").rstrip() + "\n"


def squash_body(body: str) -> str:
    """The pull request body without the template's HTML comments, which the page hides.

    A comment after the sign-off would end the message, and git would no longer read the
    sign-off as a trailer. Only comments that fill their lines alone are removed, so no text
    is joined that CI scanned apart; any other comment refuses the landing.
    """
    text = _STANDALONE_COMMENT.sub("", body.replace("\r\n", "\n"))
    if "<!--" in text:
        msg = "an HTML comment inside a line of the body: move it to lines of its own or remove it"
        raise LandError(msg)
    return normalize(re.sub(r"\n{3,}", "\n\n", text).strip("\n"))


def check_author(
    author: dict[str, object] | None, body: str, identity: str, address: str
) -> tuple[str | None, list[str]]:
    """Return the --author-email to pass (None: none) and the errors of the body for `author`.

    GitHub makes the pull request author the author of the squash commit, and denylist-main
    accepts a sign-off only when it repeats that author (FIX-LAND-IDENTITY): a bot's pull
    request lands without any sign-off, a human one only when the maintainer who lands opened
    it, with exactly the maintainer's sign-off.
    """
    login = pr_author(author)
    if login in PR_BODY_BOTS:
        return None, check_pr_body(body, author=login)
    if login.lower() == address.split("@", 1)[0].split("+", 1)[1].lower():
        return address, check_squash_body(body, identity=identity)
    return None, [
        f"the pull request was opened by {login or 'an unknown account'}, not by the "
        f"maintainer of {IDENTITY_VARIABLE}: GitHub makes its author the author of the "
        "squash commit, which the maintainer's sign-off would not repeat"
    ]


def land(
    number: str,
    *,
    identity: str,
    gh: Gh = run_gh,
    repo: str | None = None,
    delete_branch: bool = False,
    dry_run: bool = False,
) -> int:
    """Squash-merge pull request `number`; 0 landed, 1 refused before the merge, 2 after."""
    where = ["--repo", repo] if repo else []
    try:
        _, address = parse_identity(identity)
        fields = "title,body,headRefOid,state,author"
        pr = json.loads(gh(["pr", "view", number, *where, "--json", fields]))
        if pr["state"] != "OPEN":
            raise LandError(f"the pull request is {pr['state']}, not OPEN")
        subject = f"{pr['title']} (#{number})"
        body = squash_body(pr["body"] or "")
        author_email, errors = check_author(pr.get("author"), body, identity.strip(), address)
        if errors:
            raise LandError("; ".join(errors))
    except LandError as err:
        print(f"land_pr: not merged: {err}")
        return 1
    with tempfile.TemporaryDirectory() as tmp:
        body_file = Path(tmp) / "squash-body.md"
        body_file.write_text(body, encoding="utf-8")
        cmd = ["pr", "merge", number, *where, "--squash", "--subject", subject]
        cmd += ["--body-file", str(body_file)]
        if author_email is not None:
            cmd += ["--author-email", author_email]
        cmd += ["--match-head-commit", pr["headRefOid"]]
        if delete_branch:
            cmd.append("--delete-branch")
        if dry_run:
            print("land_pr: would run: gh " + " ".join(cmd))
            print(f"{subject}\n\n{body}", end="")
            return 0
        try:
            gh(cmd)
        except subprocess.CalledProcessError:
            # gh can merge and then fail on a later step (deleting a branch that a worktree
            # holds), so the state decides, not the exit code.
            after = json.loads(gh(["pr", "view", number, *where, "--json", "state"]))
            if after["state"] != "MERGED":
                print("land_pr: not merged: gh pr merge failed (head moved, checks or a thread?)")
                return 1
    try:
        merged = json.loads(gh(["pr", "view", number, *where, "--json", "mergeCommit"]))
        oid = merged["mergeCommit"]["oid"]
        api_repo = repo or "{owner}/{repo}"
        commit = json.loads(gh(["api", f"repos/{api_repo}/commits/{oid}", "--jq", ".commit"]))
        on_main = commit["message"]
        landed_author = f"{commit['author']['name']} <{commit['author']['email']}>"
    except (subprocess.CalledProcessError, KeyError, TypeError, ValueError):
        print(
            "land_pr: merged, but the commit on main could not be read back: compare its "
            "message with the pull request text by hand (gh pr view --json mergeCommit)"
        )
        return 2
    if normalize(on_main) != normalize(f"{subject}\n\n{body}"):
        print(
            f"land_pr: merged as {oid}, but its message differs from the checked text; "
            "a commit on main is not rewritten without the owner: report it"
        )
        return 2
    if foreign_signoffs(on_main, landed_author):
        print(
            f"land_pr: merged as {oid}, but a sign-off of its message does not repeat its "
            "author, which denylist-main reports; a commit on main is not rewritten without "
            "the owner: report it"
        )
        return 2
    print(f"land_pr: merged as {oid} with the checked text")
    return 0


def main(argv: list[str] | None = None) -> int:
    """Land the pull request of the command line; the exit code of `land`."""
    parser = argparse.ArgumentParser(description="Squash-merge with the checked text.")
    parser.add_argument("number")
    parser.add_argument("--repo")
    parser.add_argument("--delete-branch", action="store_true")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    return land(
        args.number,
        identity=os.environ.get(IDENTITY_VARIABLE, ""),
        repo=args.repo,
        delete_branch=args.delete_branch,
        dry_run=args.dry_run,
    )


if __name__ == "__main__":
    sys.exit(main())
