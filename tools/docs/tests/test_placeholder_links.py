# SPDX-License-Identifier: Apache-2.0
"""Tests for the placeholder link check (tools/docs/placeholder_links.py)."""

from __future__ import annotations

from pathlib import Path

import placeholder_links
import pytest


def _write(tmp_path: Path, text: str) -> Path:
    path = tmp_path / "page.md"
    path.write_text(text, encoding="utf-8")
    return path


@pytest.mark.parametrize(
    "text",
    [
        "See [CONTRIBUTING](https://github.com/OWNER/bothy/blob/main/CONTRIBUTING.md).\n",
        "Site: <https://OWNER.github.io/bothy/>\n",
        "[ref]: https://github.com/OWNER/bothy\n",
        '<a href="https://github.com/OWNER/bothy/issues">Issues</a>\n',
        '<img src="https://raw.githubusercontent.com/OWNER/bothy/main/x.png" alt="x">\n',
        "Image ![x](https://github.com/OWNER/bothy/raw/main/x.png)\n",
    ],
)
def test_link_with_placeholder_fails(tmp_path: Path, text: str) -> None:
    problems = placeholder_links.check_file(_write(tmp_path, text))
    assert len(problems) == 1
    assert "page.md:1" in problems[0]


@pytest.mark.parametrize(
    "text",
    [
        "Clone with `git clone https://github.com/OWNER/bothy.git`.\n",
        "```sh\ncurl -fsSL https://github.com/OWNER/bothy/releases/x/install.sh | sh\n```\n",
        "~~~\n[x](https://github.com/OWNER/bothy)\n~~~\n",
        "The placeholder OWNER is replaced in V1-03.\n",
        "See [CONTRIBUTING](../CONTRIBUTING.md) and [owners](https://github.com/example/OWNERS).\n",
        "[Owner](https://github.com/owner-of-the-repo/bothy)\n",
    ],
)
def test_code_prose_and_other_links_pass(tmp_path: Path, text: str) -> None:
    assert placeholder_links.check_file(_write(tmp_path, text)) == []


def test_code_span_across_lines_passes(tmp_path: Path) -> None:
    text = "Labels: `.source` from `ARG\nSOURCE_URL` (default `https://github.com/OWNER/b`), end.\n"
    assert placeholder_links.check_file(_write(tmp_path, text)) == []


def test_code_span_does_not_cross_paragraphs(tmp_path: Path) -> None:
    text = "A stray ` here.\n\nSee [x](https://github.com/OWNER/bothy) and `code`.\n"
    problems = placeholder_links.check_file(_write(tmp_path, text))
    assert problems == ["page.md:3: link to placeholder repository: https://github.com/OWNER/bothy"]


def test_line_numbers_after_code_block(tmp_path: Path) -> None:
    text = (
        "intro\n```\n[x](https://github.com/OWNER/a)\n```\n\nbad [y](https://github.com/OWNER/b)\n"
    )
    problems = placeholder_links.check_file(_write(tmp_path, text))
    assert problems == ["page.md:6: link to placeholder repository: https://github.com/OWNER/b"]


def test_main_reports_and_fails(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    good = tmp_path / "good.md"
    good.write_text("[a](b.md)\n", encoding="utf-8")
    bad = _write(tmp_path, "[x](https://github.com/OWNER/bothy)\n")
    assert placeholder_links.main([str(good)]) == 0
    assert placeholder_links.main([str(good), str(bad)]) == 1
    assert "placeholder repository" in capsys.readouterr().err
