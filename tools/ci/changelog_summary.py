# SPDX-License-Identifier: Apache-2.0
"""Changelog summary check: every release section starts with a summary for users.

Brief M7-28 item 15, docs/spec/18-release.md section 4. A release section is a heading
`## [<version>] - <date>` of a `CHANGELOG.md`; its first non-empty line starts with `Summary: `
followed by text for users, before the first `###` heading or list item. The summary is that
paragraph and may wrap onto further lines. `## [Unreleased]` is exempt. The line
SUMMARY_PLACEHOLDER of tools/ci/release_prep.py, which `make release-prep` writes into a new
section, does not count, so a release pull request fails until a person has written the summary.
The release page shows the summary first (tools/ci/release_notes.py,
.github/release-template.md).

The check covers every tracked `CHANGELOG.md` of the repository, which includes the root file
and the one of every release component of tools/ci/components.py.

Example of a section that passes:

    ## [0.2.0] - 2026-10-15

    Summary: Replies arrive faster, and the server starts on hosts without IPv6.

    ### Fixed

    - ...

Usage:
  changelog_summary.py [--repo DIR]
Exit codes: 0 every section has a summary, 1 findings (file, line and version on stderr),
2 usage error.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

import components
from release_prep import SUMMARY_PLACEHOLDER

SUMMARY_PREFIX = "Summary:"
_SECTION = re.compile(r"^## \[([^\]]+)\]")
# Where the technical entries begin; the summary must come before them.
_ENTRIES = re.compile(r"^(?:### |[-*+] )")


class UsageError(Exception):
    """The check cannot run, for example because the repository is no git work tree."""


@dataclass(frozen=True)
class Section:
    """A release section of a changelog.

    Attributes:
        version: the text in the brackets of the heading, for example `0.2.0`.
        line: the 1-based line number of the heading.
        body: the lines after the heading up to the next `## ` heading.
    """

    version: str
    line: int
    body: tuple[str, ...]


@dataclass(frozen=True)
class Finding:
    """A release section without a valid summary.

    Attributes:
        path: the changelog, repository-relative.
        line: the 1-based line number of the section heading.
        version: the version of the section.
        problem: what is wrong, in words.
    """

    path: str
    line: int
    version: str
    problem: str

    def __str__(self) -> str:
        """Return `path:line: [version] problem`, the form of the error output."""
        return f"{self.path}:{self.line}: [{self.version}] {self.problem}"


def release_sections(text: str) -> list[Section]:
    """Split a changelog into its release sections.

    Args:
        text: the content of a CHANGELOG.md.

    Returns:
        Every section with a heading `## [<version>]` except `## [Unreleased]`, in file order.
    """
    lines = text.splitlines()
    starts = [i for i, line in enumerate(lines) if line.startswith("## ")]
    sections = []
    for n, start in enumerate(starts):
        head = _SECTION.match(lines[start])
        if head is None or head.group(1) == "Unreleased":
            continue
        end = starts[n + 1] if n + 1 < len(starts) else len(lines)
        sections.append(Section(head.group(1), start + 1, tuple(lines[start + 1 : end])))
    return sections


def summary_paragraph(section: Section) -> list[str]:
    """Return the summary paragraph of a section as it stands in the file.

    Args:
        section: a release section.

    Returns:
        The lines from the first non-empty line, if it starts with `Summary:`, up to the next
        empty line, `###` heading or list item; an empty list if the section has no such line.
    """
    body = list(section.body)
    while body and not body[0].strip():
        body.pop(0)
    if not body or not body[0].startswith(SUMMARY_PREFIX):
        return []
    paragraph = [body[0]]
    for line in body[1:]:
        if not line.strip() or _ENTRIES.match(line):
            break
        paragraph.append(line)
    return paragraph


def summary_of(section: Section) -> str | None:
    """Return the summary text of a section, without the prefix, on one line.

    Args:
        section: a release section.

    Returns:
        The text, or None when the section has no summary, an empty one or the placeholder.
    """
    paragraph = summary_paragraph(section)
    if not paragraph or paragraph[0].strip() == SUMMARY_PLACEHOLDER:
        return None
    text = " ".join(line.strip() for line in paragraph)[len(SUMMARY_PREFIX) :].strip()
    return text or None


def problem_of(section: Section) -> str | None:
    """Return what is wrong with the summary of a section.

    Args:
        section: a release section.

    Returns:
        A description of the problem, or None when the summary is valid.
    """
    paragraph = summary_paragraph(section)
    if not paragraph:
        return f"the section does not start with a line '{SUMMARY_PREFIX} <text for users>'"
    if paragraph[0].strip() == SUMMARY_PLACEHOLDER:
        return "the summary is still the placeholder of release-prep; write it for users"
    if summary_of(section) is None:
        return "the summary is empty"
    return None


def check_text(path: str, text: str) -> list[Finding]:
    """Check every release section of one changelog.

    Args:
        path: the name of the changelog in the findings.
        text: its content.

    Returns:
        One finding per section without a valid summary, in file order.
    """
    found = []
    for section in release_sections(text):
        problem = problem_of(section)
        if problem is not None:
            found.append(Finding(path, section.line, section.version, problem))
    return found


def changelog_files(repo: Path) -> list[str]:
    """Return the changelogs to check, repository-relative and sorted.

    Args:
        repo: the root of the git work tree.

    Returns:
        Every tracked file named `CHANGELOG.md`, plus the `CHANGELOG.md` of every release
        component of tools/ci/components.py that exists, should it not be tracked yet.

    Raises:
        UsageError: `repo` is no git work tree.
    """
    try:
        listed = subprocess.run(
            ["git", "-C", str(repo), "ls-files", "-z", "--", "CHANGELOG.md", "**/CHANGELOG.md"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout
    except (subprocess.CalledProcessError, FileNotFoundError, NotADirectoryError) as exc:
        raise UsageError(f"{repo} is no git work tree") from exc
    files = {name for name in listed.split("\0") if name}
    for directory, _ in components.RELEASE_COMPONENTS.values():
        own = Path(directory, "CHANGELOG.md").as_posix()
        if (repo / own).is_file():
            files.add(own)
    return sorted(files)


def check_repo(repo: Path) -> list[Finding]:
    """Check every changelog of a repository.

    Args:
        repo: the root of the git work tree.

    Returns:
        The findings of all changelogs, by file and line.

    Raises:
        UsageError: `repo` is no git work tree.
    """
    found = []
    for name in changelog_files(repo):
        found += check_text(name, (repo / name).read_text(encoding="utf-8"))
    return found


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Check the user summary of every release section.")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    args = parser.parse_args(argv)
    try:
        found = check_repo(args.repo)
    except UsageError as exc:
        print(f"changelog-summary: {exc}", file=sys.stderr)
        return 2
    for finding in found:
        print(f"changelog-summary: {finding}", file=sys.stderr)
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
