# SPDX-License-Identifier: Apache-2.0
"""The pin follows the newest upstream main commit of Hermes Agent (D-HERMES-PIN-1).

The scheduled workflow ``.github/workflows/hermes-pin.yml`` runs ``bump`` once a day
(D-HERMES-PIN-2): it reads the head of ``refs/heads/main`` of the pinned repository with
``git ls-remote`` and, when it is newer than the pin and no maintainer works on the open
proposal, rewrites the pin file (``commit``, ``recorded_on``, no ``tag``) and the sentence
"Current pin: ..." of spec 09 section 17.10. The workflow then records again and runs the
component's tests. Standard library only, so the workflow runs it without the project
environment:

    uv run --no-project --python 3.14 plugins/hermes-adapter/tests/follow_main.py \
        bump --root . [--proposal REF] [--github-output PATH]

Exit codes: 0 done (``propose=true|false`` in the output), 1 an error, 2 a usage error.
"""

from __future__ import annotations

import argparse
import datetime
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Final

#: The standing ruling of the pin; the pin file and 17.10 name it.
FOLLOW_MAIN: Final = "D-HERMES-PIN-1"
#: The branch of the one open proposal; the workflow force-pushes it while only the bot wrote it.
BRANCH: Final = "bot/hermes-main-pin"
#: Commit identity of the workflow (the GitHub Actions bot).
BOT_NAME: Final = "github-actions[bot]"
BOT_EMAIL: Final = "41898282+github-actions[bot]@users.noreply.github.com"
PIN_PATH: Final = "plugins/hermes-adapter/tests/hermes_pin.toml"
CHAPTER_PATH: Final = "docs/spec/09-agents.md"

_SHA: Final = re.compile(r"[0-9a-f]{40}")
_COMMIT_LINE: Final = re.compile(r'^commit = "[0-9a-f]{40}"$', re.MULTILINE)
_DAY_LINE: Final = re.compile(r"^recorded_on = \d{4}-\d{2}-\d{2}$", re.MULTILINE)
_TAG_LINE: Final = re.compile(r'^tag = "[^"\n]*"\n', re.MULTILINE)
_SENTENCE: Final = re.compile(
    r"^Current pin: upstream main commit `[0-9a-f]{12}`, recorded on \d{4}-\d{2}-\d{2} "
    + re.escape(f"({FOLLOW_MAIN}).")
    + "$",
    re.MULTILINE,
)


class FollowMainError(RuntimeError):
    """Upstream gave no single main commit, or a file is not in the form this module rewrites."""


@dataclass(frozen=True)
class Decision:
    """Whether the workflow proposes ``head``, and why (for the job log)."""

    propose: bool
    reason: str


def current_pin_sentence(commit: str, day: datetime.date) -> str:
    """The sentence of 09 17.10 that names the current pin (one line of its own)."""
    return (
        f"Current pin: upstream main commit `{commit[:12]}`, recorded on {day.isoformat()} "
        f"({FOLLOW_MAIN})."
    )


def _replace_once(pattern: re.Pattern[str], text: str, new: str, what: str) -> str:
    found = pattern.findall(text)
    if len(found) != 1:
        raise FollowMainError(f"expected exactly one {what}, found {len(found)}")
    return pattern.sub(lambda _m: new, text)


def bump_pin_text(text: str, commit: str, day: datetime.date) -> str:
    """The pin file with ``commit`` and ``recorded_on = day`` and without a ``tag`` line."""
    if not _SHA.fullmatch(commit):
        raise FollowMainError(f"not a full commit SHA: {commit!r}")
    text = _replace_once(_COMMIT_LINE, text, f'commit = "{commit}"', "commit line")
    text = _replace_once(_DAY_LINE, text, f"recorded_on = {day.isoformat()}", "recorded_on line")
    return _TAG_LINE.sub("", text)


def bump_chapter_text(text: str, commit: str, day: datetime.date) -> str:
    """Chapter 09 with the "Current pin" sentence of 17.10 naming ``commit`` and ``day``."""
    head, sep, rest = text.partition("### 17.10 ")
    if not sep:
        raise FollowMainError("chapter 09 has no section 17.10")
    section, sep_end, tail = rest.partition("\n## ")
    section = _replace_once(
        _SENTENCE, section, current_pin_sentence(commit, day), "'Current pin' sentence in 17.10"
    )
    return head + sep + section + sep_end + tail


def _git(*args: str, cwd: Path | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=cwd,
        capture_output=True,
        text=True,
        check=False,
        env={"GIT_TERMINAL_PROMPT": "0", **_path_env()},
    )


def _path_env() -> dict[str, str]:
    keep = ("PATH", "HOME", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM", "SSL_CERT_FILE")
    return {k: v for k, v in os.environ.items() if k in keep}


def main_head(repository: str) -> str:
    """The commit of ``refs/heads/main`` of ``repository``; fails closed on anything else."""
    done = _git("ls-remote", "--", repository, "refs/heads/main")
    if done.returncode != 0:
        raise FollowMainError(f"git ls-remote failed: {done.stderr.strip()}")
    rows = [line.split("\t") for line in done.stdout.splitlines() if line.strip()]
    heads = [row[0] for row in rows if len(row) == 2 and row[1] == "refs/heads/main"]
    if len(heads) != 1 or not _SHA.fullmatch(heads[0]):
        raise FollowMainError(f"no single main commit in {repository}: {done.stdout!r}")
    return heads[0]


def decide(
    *, head: str, pinned: str, proposal: str | None, proposal_authors: list[str]
) -> Decision:
    """Propose ``head`` only when it is new and the open proposal is the bot's own."""
    if head == pinned:
        return Decision(False, f"main already pins the newest upstream main commit {head[:12]}")
    if any(author != BOT_EMAIL for author in proposal_authors):
        return Decision(False, f"a maintainer works on the open proposal ({BRANCH}); left as is")
    if proposal == head:
        return Decision(False, f"the open proposal already names {head[:12]}")
    return Decision(True, f"propose upstream main commit {head[:12]}")


def _pinned_commit(text: str) -> str:
    match = re.search(r'^commit = "([0-9a-f]{40})"$', text, re.MULTILINE)
    if match is None:
        raise FollowMainError("the pin file has no commit line")
    return match.group(1)


def _repository(text: str) -> str:
    match = re.search(r'^repository = "([^"]+)"$', text, re.MULTILINE)
    if match is None:
        raise FollowMainError("the pin file has no repository line")
    return match.group(1)


def _proposal(root: Path, ref: str | None) -> tuple[str | None, list[str]]:
    """Pinned commit and commit authors (beyond HEAD) of the open proposal ``ref``, if any."""
    if not ref or _git("rev-parse", "--verify", "-q", f"{ref}^{{commit}}", cwd=root).returncode:
        return None, []
    shown = _git("show", f"{ref}:{PIN_PATH}", cwd=root)
    pinned = _pinned_commit(shown.stdout) if shown.returncode == 0 else None
    log = _git("log", "--format=%ae%n%ce", f"HEAD..{ref}", cwd=root)
    if log.returncode != 0:
        raise FollowMainError(f"git log of {ref} failed: {log.stderr.strip()}")
    return pinned, sorted({a for a in log.stdout.splitlines() if a})


def main(argv: list[str] | None = None) -> int:
    """Command line entry point (see the module docstring)."""
    parser = argparse.ArgumentParser(description="Follow the upstream main commit of Hermes.")
    sub = parser.add_subparsers(dest="command", required=True)
    bump = sub.add_parser("bump")
    bump.add_argument("--root", type=Path, default=Path())
    bump.add_argument("--proposal", help="git ref of the open proposal branch, if fetched")
    bump.add_argument("--commit", help="use this commit instead of git ls-remote (tests)")
    bump.add_argument("--today", type=datetime.date.fromisoformat)
    bump.add_argument("--github-output", type=Path)
    args = parser.parse_args(argv)
    root: Path = args.root
    pin_file, chapter_file = root / PIN_PATH, root / CHAPTER_PATH
    try:
        pin_text = pin_file.read_text(encoding="utf-8")
        head = args.commit or main_head(_repository(pin_text))
        proposal, authors = _proposal(root, args.proposal)
        decision = decide(
            head=head, pinned=_pinned_commit(pin_text), proposal=proposal, proposal_authors=authors
        )
        if decision.propose:
            day = args.today or datetime.datetime.now(datetime.UTC).date()
            new_pin = bump_pin_text(pin_text, head, day)
            new_chapter = bump_chapter_text(chapter_file.read_text(encoding="utf-8"), head, day)
            pin_file.write_text(new_pin, encoding="utf-8")
            chapter_file.write_text(new_chapter, encoding="utf-8")
    except (FollowMainError, OSError) as exc:
        print(f"follow_main: {exc}", file=sys.stderr)
        return 1
    print(decision.reason)
    lines = [f"propose={str(decision.propose).lower()}", f"reason={decision.reason}"]
    if decision.propose:
        lines += [f"commit={head}", f"short={head[:12]}"]
    if args.github_output:
        with args.github_output.open("a", encoding="utf-8") as out:
            out.write("".join(line + "\n" for line in lines))
    return 0


if __name__ == "__main__":
    sys.exit(main())
