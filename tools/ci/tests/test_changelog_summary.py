# SPDX-License-Identifier: Apache-2.0
"""Tests for the changelog summary check (tools/ci/changelog_summary.py, brief M7-28 item 15).

Every release section of a CHANGELOG.md starts with a summary for users; the placeholder that
`make release-prep` writes does not count, and "Unreleased" is exempt.
"""

from __future__ import annotations

import subprocess
from pathlib import Path

import changelog_summary
import pytest
import release_prep

REPO = Path(__file__).resolve().parents[3]
HEAD = "# Changelog\n\nIntro text.\n\n## [Unreleased]\n\n### Added\n\n- Not released yet.\n\n"


def findings(text: str) -> list[changelog_summary.Finding]:
    return changelog_summary.check_text("x/CHANGELOG.md", text)


def test_release_section_without_a_summary_fails() -> None:
    text = HEAD + "## [0.2.0] - 2026-10-15\n\n### Fixed\n\n- A fix.\n"
    [finding] = findings(text)
    assert finding.path == "x/CHANGELOG.md"
    assert finding.line == 11
    assert finding.version == "0.2.0"
    assert str(finding).startswith("x/CHANGELOG.md:11: [0.2.0] ")


def test_list_item_before_the_summary_fails() -> None:
    text = HEAD + "## [0.2.0] - 2026-10-15\n\n- A fix.\n\nSummary: Too late.\n"
    assert [f.version for f in findings(text)] == ["0.2.0"]


def test_release_section_with_a_summary_passes() -> None:
    text = HEAD + (
        "## [0.2.0] - 2026-10-15\n\nSummary: Replies arrive faster and a crash on start is\n"
        "fixed.\n\n### Fixed\n\n- A fix.\n"
    )
    assert findings(text) == []
    [section] = changelog_summary.release_sections(text)
    assert (
        changelog_summary.summary_of(section)
        == "Replies arrive faster and a crash on start is fixed."
    )


def test_placeholder_fails() -> None:
    text = HEAD + f"## [0.2.0] - 2026-10-15\n\n{release_prep.SUMMARY_PLACEHOLDER}\n\n- A fix.\n"
    [finding] = findings(text)
    assert "placeholder" in finding.problem


def test_section_of_release_prep_fails_until_the_placeholder_is_replaced() -> None:
    text = release_prep.release_changelog(HEAD, "0.2.0", "2026-10-15", None)
    [finding] = findings(text)
    assert (finding.version, "placeholder" in finding.problem) == ("0.2.0", True)
    written = text.replace(release_prep.SUMMARY_PLACEHOLDER, "Summary: Something for users.")
    assert findings(written) == []


def test_empty_summary_fails() -> None:
    text = HEAD + "## [0.2.0] - 2026-10-15\n\nSummary:\n\n- A fix.\n"
    assert [f.version for f in findings(text)] == ["0.2.0"]


def test_unreleased_is_exempt() -> None:
    assert findings(HEAD) == []
    assert changelog_summary.release_sections(HEAD) == []


def test_section_with_only_a_summary_passes() -> None:
    text = HEAD + "## [0.2.0] - 2026-10-15\n\nSummary: Only a rebuild with new signatures.\n"
    assert findings(text) == []


def test_every_section_is_checked_on_its_own() -> None:
    text = HEAD + (
        "## [0.3.0] - 2026-10-20\n\nSummary: Good.\n\n- x\n\n"
        "## [0.2.0] - 2026-10-15\n\n- y\n\n"
        "## [0.1.0] - 2026-10-01\n\nSummary: Good too.\n"
    )
    assert [(f.version, f.line) for f in findings(text)] == [("0.2.0", 17)]


def git_repo(root: Path, files: dict[str, str]) -> Path:
    subprocess.run(["git", "init", "-q", str(root)], check=True)
    for name, text in files.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
    subprocess.run(["git", "-C", str(root), "add", "."], check=True)
    return root


def test_check_covers_every_tracked_changelog(tmp_path: Path) -> None:
    repo = git_repo(
        tmp_path,
        {
            "CHANGELOG.md": HEAD,
            "plugins/a/CHANGELOG.md": HEAD + "## [0.1.0] - 2026-10-01\n\n- x\n",
            "docs/notes.md": HEAD + "## [0.1.0] - 2026-10-01\n\n- x\n",
        },
    )
    assert [str(f).split(":")[0] for f in changelog_summary.check_repo(repo)] == [
        "plugins/a/CHANGELOG.md"
    ]


def test_cli_exit_codes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    good = git_repo(tmp_path / "good", {"CHANGELOG.md": HEAD})
    assert changelog_summary.main(["--repo", str(good)]) == 0
    bad = git_repo(tmp_path / "bad", {"a/CHANGELOG.md": HEAD + "## [1.0.0] - 2026-10-01\n\n- x\n"})
    assert changelog_summary.main(["--repo", str(bad)]) == 1
    assert "a/CHANGELOG.md:11: [1.0.0]" in capsys.readouterr().err
    assert changelog_summary.main(["--repo", str(tmp_path / "missing")]) == 2
    with pytest.raises(SystemExit) as exc:
        changelog_summary.main(["--unknown"])
    assert exc.value.code == 2


def test_component_changelogs_are_covered() -> None:
    files = changelog_summary.changelog_files(REPO)
    assert "CHANGELOG.md" in files
    assert "plugins/cost-reconciler/CHANGELOG.md" in files
    for component, (directory, _) in release_prep.components.RELEASE_COMPONENTS.items():
        path = Path(directory, "CHANGELOG.md").as_posix()
        if (REPO / path).is_file():
            assert path in files, component


def test_real_repository_passes() -> None:
    assert changelog_summary.check_repo(REPO) == []
