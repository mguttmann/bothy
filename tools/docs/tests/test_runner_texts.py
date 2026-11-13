# SPDX-License-Identifier: Apache-2.0
"""The texts of the Claude Code runner say what the CLI does, never what one account showed.

AGENTS.md, "This repository is public": no billing or plan details and nothing about the
machines and tools somebody uses privately. The runner works on an account of Claude Code, so a
fix that starts from a failure of such an account is where a sentence about it slips in: its
plan, or the installation the failure was read from. A comment, a fixture, a test, a changelog
entry, a ruling and the specification give the behaviour of the pinned CLI instead, read from
its file or run against the local stand-in for the API (`tests/real_cli` of the runner).

The list below holds wordings. It does not recognize every such sentence, and no count, date or
duration of somebody's use: a review still reads the texts of a change against the rule. The
test lives with the tools tests because they run on every change, also on one that touches only
a ruling (`test_host_service_texts.py` holds the host service the same way).

A ruling is found through the changelog fragment of its ticket, so it is read from the change
that writes both until the release that takes the fragment up.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

from git_fixtures import commit, init_repo

REPO = Path(__file__).resolve().parents[3]
RUNNER = "plugins/claude-code-runner"
CHAPTER = "docs/spec/09-agents.md"

# What such a text never says.
ONE_ACCOUNT = {
    "the plan of an account": r"\bseats?\b",
    "what one installation showed": (
        r"\b(?:installation|instance|host|machine|runner|account)s? in use\b"
    ),
}


def _texts_of_the_runner(repo: Path) -> list[Path]:
    """Every tracked file of the runner, the rulings of its fragments and its chapter."""
    tracked = subprocess.run(
        ["git", "ls-files", "-z", "--", RUNNER, CHAPTER],
        cwd=repo,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.split("\0")
    # A tracked file can be gone: release-prep deletes the fragments before the release commit.
    files = [repo / name for name in tracked if name and (repo / name).is_file()]
    assert repo / CHAPTER in files, f"{CHAPTER} is not tracked"
    assert len(files) > 1, f"no tracked file under {RUNNER}"
    # A ticket records its changelog entry and its rulings under the same file name. A release
    # leaves no fragment, so there may be no ruling to read.
    fragments = sorted((repo / RUNNER / "changelog.d").glob("*.md"))
    rulings = [repo / "docs" / "decisions.d" / fragment.name for fragment in fragments]
    return files + [ruling for ruling in rulings if ruling.is_file()]


def _sentences_about_one_account(repo: Path) -> list[str]:
    """Every wording of ONE_ACCOUNT in the texts of the runner, with its file."""
    found = []
    for path in _texts_of_the_runner(repo):
        try:
            raw = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        # One line of spaces: a sentence wraps, and a comment sign starts its next line.
        text = " ".join(re.sub(r"(?m)^\s*#", "", raw).split())
        for what, pattern in ONE_ACCOUNT.items():
            found += [
                f"{path.relative_to(repo).as_posix()}: {what}: {match.group(0)}"
                for match in re.finditer(pattern, text, re.IGNORECASE)
            ]
    return found


def test_texts_of_the_runner_describe_no_account_in_use() -> None:
    assert _sentences_about_one_account(REPO) == []


def test_a_sentence_is_found_in_every_kind_of_text(tmp_path: Path) -> None:
    repo = init_repo(tmp_path / "repo")
    texts = {
        f"{RUNNER}/README.md": "The usage limit of a subscription seat is reached.\n",
        # The sentence wraps inside a comment.
        f"{RUNNER}/src/account.py": "# An example comment: read from a runner in\n# use.\n",
        f"{RUNNER}/tests/fixtures/limit.jsonl": '{"source":"read on an installation in use"}\n',
        f"{RUNNER}/changelog.d/T-1.md": "### Fixed\n\n- An example entry.\n",
        "docs/decisions.d/T-1.md": "# T-1 An example ruling\n\nAn account with several seats.\n",
        # No ticket of the runner, and no file of it: neither is read.
        "docs/decisions.d/T-2.md": "# T-2 Another ruling\n\nIts own seat.\n",
        "services/host/run.go": "// A seat of the operating system.\npackage main\n",
        CHAPTER: "An example rule, as seen on a host in use.\n",
    }
    for name, text in texts.items():
        (repo / name).parent.mkdir(parents=True, exist_ok=True)
        (repo / name).write_text(text, encoding="utf-8")
    commit(repo, "fix(claude-code-runner): an example ticket")
    # What an interpreter leaves next to the sources is not tracked and not read.
    (repo / RUNNER / "src" / "cache.txt").write_text("A seat.\n", encoding="utf-8")
    assert _sentences_about_one_account(repo) == [
        f"{CHAPTER}: what one installation showed: host in use",
        f"{RUNNER}/README.md: the plan of an account: seat",
        f"{RUNNER}/src/account.py: what one installation showed: runner in use",
        f"{RUNNER}/tests/fixtures/limit.jsonl: what one installation showed: installation in use",
        "docs/decisions.d/T-1.md: the plan of an account: seats",
    ]
