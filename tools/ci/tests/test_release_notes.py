# SPDX-License-Identifier: Apache-2.0
"""Tests for the release page text (tools/ci/release_notes.py, .github/release-template.md).

The page starts with the user summary of the changelog section, then what changed for users,
the upgrade notes, the technical entries and the verification.
"""

from __future__ import annotations

from pathlib import Path

import pytest
import release_notes
import release_prep

REPO = Path(__file__).resolve().parents[3]
TEMPLATE = REPO / ".github" / "release-template.md"
CHANGELOG = """# Changelog

## [Unreleased]

### Added

- Not released yet.

## [0.2.0] - 2026-10-15

Summary: Replies arrive faster, and the server
starts on hosts without IPv6.

### Fixed

- Start on hosts without IPv6.

### Migrations

- Expand only; no manual step.

### Internal

- Faster test fixtures.

## [0.1.0] - 2026-10-01

Summary: First release.

- The first thing.
"""


def render(changelog: str = CHANGELOG, version: str = "0.2.0") -> str:
    return release_notes.render(
        TEMPLATE.read_text(encoding="utf-8"),
        changelog,
        version=version,
        tag=f"server/v{version}",
        repository="example/bothy",
    )


def test_summary_comes_first() -> None:
    page = render()
    first = page.lstrip().splitlines()[0]
    assert first == "Replies arrive faster, and the server starts on hosts without IPv6."


def test_parts_in_the_order_of_the_template() -> None:
    page = render()
    order = [
        page.index("Replies arrive faster"),
        page.index("## What changed for users"),
        page.index("- Start on hosts without IPv6."),
        page.index("## Upgrade notes"),
        page.index("- Expand only; no manual step."),
        page.index("## Technical entries"),
        page.index("- Faster test fixtures."),
        page.index("## Verification"),
        page.index("gh release download server/v0.2.0"),
    ]
    assert order == sorted(order)
    assert "Not released yet" not in page
    assert "The first thing" not in page
    assert "<!--" not in page
    assert "{{" not in page
    assert "Summary:" not in page


def test_empty_parts_say_so() -> None:
    page = render(version="0.1.0")
    assert "- The first thing." in page
    assert page.count(release_notes.NONE_LISTED) == 2


def test_section_without_a_summary_is_an_error() -> None:
    text = CHANGELOG.replace("Summary: First release.\n\n", "")
    with pytest.raises(release_notes.ReleaseNotesError, match=r"\[0\.1\.0\]"):
        render(text, version="0.1.0")


def test_placeholder_is_an_error() -> None:
    text = CHANGELOG.replace("Summary: First release.", release_prep.SUMMARY_PLACEHOLDER)
    with pytest.raises(release_notes.ReleaseNotesError, match="placeholder"):
        render(text, version="0.1.0")


def test_missing_section_is_an_error() -> None:
    with pytest.raises(release_notes.ReleaseNotesError, match=r"no section \[9\.9\.9\]"):
        render(version="9.9.9")


def test_unknown_placeholder_is_an_error() -> None:
    with pytest.raises(release_notes.ReleaseNotesError, match="unknown placeholder"):
        release_notes.render("{{summary}} {{nope}}", CHANGELOG, "0.2.0", "v0.2.0", "example/bothy")


def test_cli_writes_the_page(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    changelog = tmp_path / "CHANGELOG.md"
    changelog.write_text(CHANGELOG, encoding="utf-8")
    out = tmp_path / "notes.md"
    args = ["--changelog", str(changelog), "--tag", "server/v0.2.0", "--version", "0.2.0"]
    args += ["--repository", "example/bothy", "--output", str(out)]
    assert release_notes.main(args) == 0
    assert out.read_text(encoding="utf-8") == render()
    assert release_notes.main([*args[:-1], str(out), "--version", "9.9.9"]) == 1
    assert "no section [9.9.9]" in capsys.readouterr().err
    with pytest.raises(SystemExit) as exc:
        release_notes.main(["--changelog", str(changelog)])
    assert exc.value.code == 2
