# SPDX-License-Identifier: Apache-2.0
"""Tests for the changelog and ruling fragments (tools/ci/fragments.py, docs/DECISIONS.md PL/24).

The range checks build a fixture repository in a temporary directory with a base commit and a
pull request branch, so the merge base is the real one.
"""

from __future__ import annotations

import subprocess
from pathlib import Path

import fragments
import pytest

CHANGELOG = """# Changelog

Intro.

## [Unreleased]

Planning stage.

### Added

- Old entry.
  Second line.

### Fixed

- Old fix.

## [0.1.0] - 2026-01-01

### Added

- The first thing.
"""


def frag(name: str, text: str) -> fragments.Fragment:
    return fragments.Fragment(Path(name), fragments.parse_changelog_fragment(text, name))


def test_entries_go_to_the_end_of_their_subsection() -> None:
    out = fragments.assemble(CHANGELOG, [frag("M2-05.md", "### Added\n\n- New entry.\n")])
    assert "### Added\n\n- Old entry.\n  Second line.\n- New entry.\n\n### Fixed\n" in out
    assert out.startswith("# Changelog\n\nIntro.\n\n## [Unreleased]\n\nPlanning stage.\n\n")
    assert out.endswith("## [0.1.0] - 2026-01-01\n\n### Added\n\n- The first thing.\n")


def test_a_missing_subsection_is_added_in_keep_a_changelog_order() -> None:
    out = fragments.assemble(
        CHANGELOG,
        [
            frag("M2-05.md", "### Security\n\n- Sec.\n\n### Changed\n\n- Chg.\n  more\n"),
        ],
    )
    unreleased = out.split("## [Unreleased]", 1)[1].split("## [0.1.0]", 1)[0]
    order = [line for line in unreleased.splitlines() if line.startswith("### ")]
    assert order == ["### Added", "### Changed", "### Fixed", "### Security"]
    assert "### Changed\n\n- Chg.\n  more\n\n### Fixed" in out


def test_fragments_are_assembled_in_natural_name_order(tmp_path: Path) -> None:
    folder = tmp_path / "changelog.d"
    folder.mkdir()
    for name in ("M2-10.md", "M2-9.md", "M10-1.md"):
        (folder / name).write_text(f"### Fixed\n\n- {name}\n", encoding="utf-8")
    names = [f.path.name for f in fragments.read_fragments(tmp_path)]
    assert names == ["M2-9.md", "M2-10.md", "M10-1.md"]


def test_an_empty_unreleased_section_gets_the_entries() -> None:
    text = "# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-01-01\n\n- x\n"
    out = fragments.assemble(text, [frag("a.md", "### Fixed\n\n- A fix.\n")])
    assert out == (
        "# Changelog\n\n## [Unreleased]\n\n### Fixed\n\n- A fix.\n\n"
        "## [0.1.0] - 2026-01-01\n\n- x\n"
    )


def test_without_fragments_the_changelog_stays_byte_for_byte() -> None:
    assert fragments.assemble(CHANGELOG, []) == CHANGELOG


@pytest.mark.parametrize(
    ("text", "message"),
    [
        ("- No heading.\n", "before the first"),
        ("### Improved\n\n- x\n", "not one of"),
        ("### Fixed\n\n- x\n\n### Fixed\n\n- y\n", "appears twice"),
        ("### Fixed\n\nA paragraph.\n", "expected a list item"),
        ("### Fixed\n\n", "has no list item"),
        ("\n", "no entry"),
    ],
)
def test_a_malformed_fragment_is_refused(text: str, message: str) -> None:
    with pytest.raises(fragments.FragmentError, match=message):
        fragments.parse_changelog_fragment(text, "x.md")


def git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout


def write(root: Path, rel: str, text: str) -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    git(tmp_path, "init", "-q", "-b", "main")
    git(tmp_path, "config", "user.name", "Alex")
    git(tmp_path, "config", "user.email", "alex@example.com")
    git(tmp_path, "config", "commit.gpgsign", "false")
    write(tmp_path, "CHANGELOG.md", CHANGELOG)
    write(tmp_path, "services/server/CHANGELOG.md", CHANGELOG)
    write(tmp_path, "docs/DECISIONS.md", "# Ruling log\n\n## PL Rulings\n\n| a |\n")
    commit(tmp_path, "base")
    git(tmp_path, "switch", "-q", "-c", "feat")
    return tmp_path


def commit(root: Path, message: str) -> None:
    git(root, "add", "-A")
    git(root, "commit", "-q", "--allow-empty", "-m", message)


def test_valid_fragments_pass(repo: Path) -> None:
    write(repo, "changelog.d/M2-05.md", "### Added\n\n- x\n")
    write(repo, "services/server/changelog.d/M2-05.md", "### Fixed\n\n- y\n")
    write(repo, "docs/decisions.d/M2-05.md", "# M2-05 Runs\n\n| No |\n")
    commit(repo, "fragments")
    assert fragments.check_tree(repo) == []
    assert fragments.check_range(repo, "main..HEAD") == []


@pytest.mark.parametrize(
    ("rel", "text", "message"),
    [
        ("apps/x/changelog.d/M2-05.md", "### Added\n\n- x\n", "release component"),
        ("services/host/changelog.d/M2-05.md", "### Added\n\n- x\n", "CHANGELOG.md is missing"),
        ("changelog.d/M2 05.md", "### Added\n\n- x\n", "name a fragment"),
        ("changelog.d/M2-05.md", "### Nope\n\n- x\n", "not one of"),
        ("docs/decisions.d/M2-05.md", "Rulings\n", "first line is the heading"),
        ("docs/decisions.d/index.md", "# Index\n", "generated"),
        ("docs/decisions.d/sub/M2-05.md", "# M2-05\n", "no folders"),
    ],
)
def test_misplaced_or_malformed_fragments_fail(
    repo: Path, rel: str, text: str, message: str
) -> None:
    write(repo, rel, text)
    commit(repo, "bad")
    problems = fragments.check_tree(repo)
    assert len(problems) == 1
    assert message in problems[0]


def test_a_new_line_under_unreleased_fails(repo: Path) -> None:
    write(
        repo,
        "services/server/CHANGELOG.md",
        CHANGELOG.replace("- Old fix.\n", "- Old fix.\n- New.\n"),
    )
    commit(repo, "direct entry")
    [problem] = fragments.check_range(repo, "main..HEAD")
    assert "services/server/CHANGELOG.md: adds '- New.'" in problem
    assert "services/server/changelog.d/<ticket>.md" in problem


def test_a_new_line_counts_against_the_merge_base_not_the_moved_base(repo: Path) -> None:
    """Entries main gained after the branch point are not the pull request's."""
    git(repo, "switch", "-q", "main")
    write(repo, "CHANGELOG.md", CHANGELOG.replace("- Old fix.\n", "- Old fix.\n- On main.\n"))
    commit(repo, "main moved")
    git(repo, "switch", "-q", "feat")
    write(repo, "changelog.d/M2-05.md", "### Fixed\n\n- x\n")
    commit(repo, "fragment")
    assert fragments.check_range(repo, "main..HEAD") == []


def test_editing_or_removing_an_existing_entry_passes(repo: Path) -> None:
    text = CHANGELOG.replace("- Old fix.\n", "- Old fix, reworded.\n").replace(
        "  Second line.\n", ""
    )
    write(repo, "CHANGELOG.md", text)
    commit(repo, "edit")
    assert fragments.check_range(repo, "main..HEAD") == []


def test_a_release_section_passes(repo: Path) -> None:
    """release-prep empties Unreleased into a version section; nothing is added under it."""
    text = CHANGELOG.replace("## [Unreleased]\n", "## [Unreleased]\n\n## [0.2.0] - 2026-10-22\n")
    write(repo, "services/server/CHANGELOG.md", text)
    commit(repo, "chore(release): server v0.2.0")
    assert fragments.check_range(repo, "main..HEAD") == []


@pytest.mark.parametrize(
    "heading", ["## M2-05 Runs", "## Integration int/m1-wave9", "## M10-1 Ten"]
)
def test_a_ticket_section_in_the_log_fails(repo: Path, heading: str) -> None:
    """Also a section that a rebase kept from main, which no range check sees as added."""
    log = f"# Ruling log\n\n## PL Rulings\n\n| a |\n\n{heading}\n\n| x |\n"
    git(repo, "switch", "-q", "main")
    write(repo, "docs/DECISIONS.md", log)
    commit(repo, "main has it")
    git(repo, "switch", "-q", "feat")
    git(repo, "rebase", "-q", "main")
    assert fragments.check_range(repo, "main..HEAD") == []
    [problem] = fragments.check_tree(repo)
    assert f"has the ticket section '{heading}'" in problem
    assert "docs/decisions.d/<ticket>.md" in problem


def test_general_sections_of_the_log_pass(repo: Path) -> None:
    log = "# Ruling log\n\n## Rules for this log\n\n## 10. Rulings\n\n## PL Rulings\n\n## New\n"
    write(repo, "docs/DECISIONS.md", log)
    commit(repo, "general")
    assert fragments.check_tree(repo) == []


def test_files_other_than_markdown_in_changelog_d_are_not_assembled(tmp_path: Path) -> None:
    folder = tmp_path / "changelog.d"
    folder.mkdir()
    (folder / ".DS_Store").write_bytes(b"\x00\x01")
    (folder / "M2-05.md").write_text("### Fixed\n\n- x\n", encoding="utf-8")
    assert [f.path.name for f in fragments.read_fragments(tmp_path)] == ["M2-05.md"]


def test_the_command_line_reports_and_assembles(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    write(repo, "services/server/changelog.d/M2-05.md", "### Fixed\n\n- Via fragment.\n")
    commit(repo, "fragment")
    assert fragments.main(["check", "--range", "main..HEAD", "--repo", str(repo)]) == 0
    assert (
        fragments.main(["assemble", "--component", "server", "--repo", str(repo), "--print"]) == 0
    )
    assert "- Old fix.\n- Via fragment.\n" in capsys.readouterr().out
    assert (repo / "services/server/changelog.d/M2-05.md").is_file()
    assert fragments.main(["assemble", "--component", "server", "--repo", str(repo)]) == 0
    assert not (repo / "services/server/changelog.d/M2-05.md").exists()
    assert "- Old fix.\n- Via fragment.\n" in (repo / "services/server/CHANGELOG.md").read_text()
    write(repo, "CHANGELOG.md", CHANGELOG.replace("- Old fix.\n", "- Old fix.\n- Direct.\n"))
    commit(repo, "direct")
    assert fragments.main(["check", "--range", "main..HEAD", "--repo", str(repo)]) == 1
    assert "adds '- Direct.'" in capsys.readouterr().err


def test_an_untracked_fragment_is_checked(repo: Path) -> None:
    write(repo, "changelog.d/M2-05.md", "### Nope\n\n- x\n")
    [problem] = fragments.check_tree(repo)
    assert "not one of" in problem


def test_a_fragment_deleted_but_not_committed_is_skipped(repo: Path) -> None:
    """release-prep deletes the fragments it assembled before the release commit."""
    write(repo, "services/server/changelog.d/M2-05.md", "### Fixed\n\n- x\n")
    write(repo, "docs/decisions.d/M2-05.md", "# M2-05 Runs\n")
    commit(repo, "fragments")
    (repo / "services/server/changelog.d/M2-05.md").unlink()
    (repo / "docs/decisions.d/M2-05.md").unlink()
    assert fragments.check_tree(repo) == []
