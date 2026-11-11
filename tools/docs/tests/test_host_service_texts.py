# SPDX-License-Identifier: Apache-2.0
"""The texts of the host service say what the program and the operating system do.

AGENTS.md, "This repository is public": nothing instance-specific goes into the repository. A
fix of the host service starts from a failure that somebody met on a machine, so its comments,
tests, changelog entries and rulings are where a sentence about that machine slips in. The
reason of a ruling is the behaviour of the operating system and the test that shows it.

The list below holds wordings, it does not recognize every such sentence: a review still reads
the texts of a change against the rule. The test lives with the tools tests because they run on
every change, also on one that touches only a ruling.

A ruling is found through the changelog fragment of its ticket, so it is read from the change
that writes both until the release that takes the fragment up (`make release-prep` moves the
entries into `CHANGELOG.md` of the host service and deletes the fragments). After that release
the entries are read in `CHANGELOG.md`; the rulings of the released tickets are not read again.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

import fetch_tools
import pytest
import release_prep
from git_fixtures import commit, init_repo

REPO = Path(__file__).resolve().parents[3]
HOST = "services/host"

# What such a text never says. "In operation" alone is no finding: the host service uses it for
# what holds while the service runs (a check in operation).
ONE_INSTALLATION = {
    "what one installation showed": r"\b(?:installation|instance|host|runner)s? in operation\b",
    "a machine in use": r"\blive (?:installation|instance|host|runner)s?\b",
}


def _texts_of_the_host_service(repo: Path) -> list[Path]:
    """Every tracked file of the host service that exists, and the rulings of its fragments."""
    tracked = subprocess.run(
        ["git", "ls-files", "-z", "--", HOST], cwd=repo, capture_output=True, text=True, check=True
    ).stdout.split("\0")
    # A tracked file can be gone: release-prep deletes the fragments before the release commit.
    files = [repo / name for name in tracked if name and (repo / name).is_file()]
    assert files, f"no tracked file under {HOST}"
    # A ticket records its changelog entry and its rulings under the same file name. A release
    # leaves no fragment, so there may be no ruling to read.
    fragments = sorted((repo / HOST / "changelog.d").glob("*.md"))
    rulings = [repo / "docs" / "decisions.d" / fragment.name for fragment in fragments]
    return files + [ruling for ruling in rulings if ruling.is_file()]


def _sentences_about_one_installation(repo: Path) -> list[str]:
    """Every wording of ONE_INSTALLATION in the texts of the host service, with its file."""
    found = []
    for path in _texts_of_the_host_service(repo):
        try:
            raw = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        # One line of spaces: a sentence wraps, and a comment sign starts its next line.
        text = " ".join(re.sub(r"(?m)^\s*(?://|#)", "", raw).split())
        for what, pattern in ONE_INSTALLATION.items():
            found += [
                f"{path.relative_to(repo).as_posix()}: {what}: {match.group(0)}"
                for match in re.finditer(pattern, text, re.IGNORECASE)
            ]
    return found


def test_texts_of_the_host_service_describe_no_installation() -> None:
    assert _sentences_about_one_installation(REPO) == []


@pytest.fixture(scope="session")
def git_cliff() -> Path:
    """The pinned git-cliff binary (fetched once into the tool cache)."""
    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    return fetch_tools.ensure(
        lock, "git-cliff", fetch_tools.platform_key(), fetch_tools.DEFAULT_CACHE
    )


@pytest.fixture
def one_ticket(tmp_path: Path) -> Path:
    """A repository whose host service has one ticket, with such a sentence in each of its texts.

    The ticket wrote a source file (the sentence wraps inside a comment), a changelog fragment
    and a ruling under the same file name.
    """
    repo = init_repo(tmp_path / "repo")
    texts = {
        f"{HOST}/VERSION": "0.0.0\n",
        f"{HOST}/CHANGELOG.md": "# Changelog\n\n## [Unreleased]\n",
        f"{HOST}/run.go": "// An example comment: seen on a live\n// host.\npackage main\n",
        f"{HOST}/changelog.d/T-1.md": (
            "### Fixed\n\n- An example entry: seen on an instance in operation.\n"
        ),
        "docs/decisions.d/T-1.md": "# T-1 An example ruling\n\nSeen on a live runner.\n",
    }
    for name, text in texts.items():
        (repo / name).parent.mkdir(parents=True, exist_ok=True)
        (repo / name).write_text(text, encoding="utf-8")
    commit(repo, "fix(host): an example ticket")
    return repo


def test_a_sentence_is_found_in_a_file_a_changelog_fragment_and_a_ruling(one_ticket: Path) -> None:
    assert _sentences_about_one_installation(one_ticket) == [
        f"{HOST}/changelog.d/T-1.md: what one installation showed: instance in operation",
        f"{HOST}/run.go: a machine in use: live host",
        "docs/decisions.d/T-1.md: a machine in use: live runner",
    ]


def test_a_release_of_the_host_service_leaves_its_texts_readable(
    one_ticket: Path, git_cliff: Path
) -> None:
    """`make release-prep COMPONENT=host` deletes the fragments; the release commit follows."""
    result = release_prep.prepare(
        "host",
        repo=one_ticket,
        git_cliff=git_cliff,
        config=REPO / "cliff.toml",
        date="2026-10-15",
    )
    assert result.used_fragments == (f"{HOST}/changelog.d/T-1.md",)
    released = [
        f"{HOST}/CHANGELOG.md: what one installation showed: instance in operation",
        f"{HOST}/run.go: a machine in use: live host",
    ]
    # Before the release commit the deleted fragment is still a tracked file.
    assert _sentences_about_one_installation(one_ticket) == released
    commit(one_ticket, f"chore(release): host v{result.version}")
    assert _sentences_about_one_installation(one_ticket) == released
