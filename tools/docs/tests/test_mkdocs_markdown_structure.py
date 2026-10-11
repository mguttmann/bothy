# SPDX-License-Identifier: Apache-2.0
"""Tests for the MkDocs hook that fails the build on lists the site renders wrongly."""

from __future__ import annotations

import logging
import subprocess
import sys
from pathlib import Path

import mkdocs_markdown_structure as hook
import pytest
from markdown import Markdown
from mkdocs.config.defaults import MkDocsConfig
from mkdocs.structure.files import File, Files
from mkdocs.structure.pages import Page

HOOK = Path(hook.__file__).resolve()
REPO = HOOK.parents[2]

GOOD = """# Start

Text before a list.

- Outer item with a sublist:
    1. First, wrapped
       over two lines.
    2. Second, with bullets:
        - a bullet
        - another bullet

        A paragraph of the second item.

    A paragraph of the outer item.

- Next outer item.

```yaml
list:
  - indented two spaces inside a fence
   3. and three
```
"""


@pytest.mark.parametrize(
    ("markdown", "expected"),
    [
        ("- a\n  - b\n", [(2, "- b")]),
        ("1. a\n   - b\n", [(2, "- b")]),
        ("- a\n    - b\n        1. c\n", []),
        ("- a\n\t- b\n", []),
        ("text\n   3. a sibling item on the site\n", [(2, "3. a sibling item on the site")]),
        # Python-Markdown makes this a second item of the bullet list; only the source shows it.
        ("- item\n  2. sibling on the site\n", [(2, "2. sibling on the site")]),
        ("```\n  - in a fence\n```\n  - after it\n", [(4, "- after it")]),
        ("~~~~\n  - x\n```\n  - still inside\n~~~~\n", []),
        ("  -not a marker\n  2.2 not a marker\n", []),
        (" * star\n  + plus\n", [(1, "* star"), (2, "+ plus")]),
    ],
)
def test_misaligned_markers(markdown: str, expected: list[tuple[int, str]]) -> None:
    assert hook.misaligned_markers(markdown) == expected


def test_list_lines() -> None:
    assert hook.list_lines("Text:\n- one\n  2. two\nend") == ["- one", "2. two"]
    assert hook.list_lines("one line only") == []


def _build(tmp_path: Path, page: str) -> subprocess.CompletedProcess[str]:
    (tmp_path / "docs").mkdir(exist_ok=True)
    (tmp_path / "docs" / "index.md").write_text(page)
    (tmp_path / "mkdocs.yml").write_text(
        f"site_name: t\nhooks:\n  - {HOOK}\nmarkdown_extensions:\n"
        "  - pymdownx.highlight\n  - pymdownx.superfences\n"
    )
    return subprocess.run(
        [sys.executable, "-m", "mkdocs", "build", "--strict", "-f", str(tmp_path / "mkdocs.yml")],
        capture_output=True,
        text=True,
        check=False,
    )


def test_strict_build_passes_with_lists_indented_by_four(tmp_path: Path) -> None:
    proc = _build(tmp_path, GOOD)
    assert proc.returncode == 0, proc.stderr
    html = (tmp_path / "site" / "index.html").read_text()
    assert "<li>a bullet</li>" in html
    assert "<p>A paragraph of the second item.</p>" in html


@pytest.mark.parametrize(
    ("page", "kind"),
    [
        ("# S\n\n- outer\n  1. nested by two spaces\n", hook.MISALIGNED),
        ("# S\n\nText:\n- swallowed by the paragraph\n", hook.SWALLOWED),
        (
            "# S\n\n- outer\n\n    Paragraph of the item.\n    - a sublist without blank line\n",
            hook.SWALLOWED,
        ),
        ("# S\n\nParagraph.\n\n    indented, not fenced\n", hook.INDENTED_CODE),
    ],
)
def test_strict_build_fails_on_each_sign(tmp_path: Path, page: str, kind: str) -> None:
    proc = _build(tmp_path, page)
    assert proc.returncode == 1
    assert "index.md: " + kind in proc.stderr


def test_findings_belong_to_their_page(tmp_path: Path) -> None:
    (tmp_path / "docs").mkdir()
    (tmp_path / "docs" / "bad.md").write_text("# B\n\n- a\n  - b\n")
    proc = _build(tmp_path, "# Start\n")
    assert proc.returncode == 1
    assert "bad.md: " + hook.MISALIGNED in proc.stderr
    assert "index.md: " not in proc.stderr


def test_site_config_registers_the_hook() -> None:
    lines = (REPO / "mkdocs.yml").read_text().splitlines()
    start = lines.index("hooks:") + 1
    hooks = []
    for line in lines[start:]:
        if not line.startswith("  - "):
            break
        hooks.append(line.removeprefix("  - "))
    assert "tools/docs/mkdocs_markdown_structure.py" in hooks


@pytest.mark.parametrize("extensions", [[], ["pymdownx.superfences"]])
@pytest.mark.parametrize(
    ("text", "kind"),
    [
        ("Text:\n- swallowed\n", hook.SWALLOWED),
        (
            "- item\n\n    Paragraph of the item.\n    - a sublist without blank line\n",
            hook.SWALLOWED,
        ),
        ("Paragraph.\n\n    indented, not fenced\n", hook.INDENTED_CODE),
    ],
)
def test_extension_records_each_sign(extensions: list[str], text: str, kind: str) -> None:
    findings = hook.Findings()
    md = Markdown(extensions=[*extensions, hook.MarkdownStructureExtension(findings)])
    md.convert(text)
    assert [item.split(":")[0] for item in findings.items] == [kind]


@pytest.mark.parametrize("extensions", [[], ["pymdownx.superfences"]])
def test_extension_accepts_lists_indented_by_four(extensions: list[str]) -> None:
    findings = hook.Findings()
    md = Markdown(extensions=[*extensions, hook.MarkdownStructureExtension(findings)])
    md.convert(GOOD.split("```yaml", maxsplit=1)[0])
    assert findings.items == []


def test_hooks_log_the_findings_of_each_page(
    tmp_path: Path, caplog: pytest.LogCaptureFixture
) -> None:
    (tmp_path / "docs").mkdir()
    cfg = MkDocsConfig(config_file_path=str(tmp_path / "mkdocs.yml"))
    cfg.load_dict({"site_name": "t", "docs_dir": str(tmp_path / "docs")})
    errors, _warnings = cfg.validate()
    assert errors == []
    cfg = hook.on_config(cfg)
    assert any(isinstance(ext, hook.MarkdownStructureExtension) for ext in cfg.markdown_extensions)
    page = Page(None, File("a.md", str(tmp_path / "docs"), str(tmp_path / "site"), True), cfg)
    markdown = "# A\n\n- a\n  - b\n"
    assert hook.on_page_markdown(markdown, page=page, config=cfg, files=Files([])) == markdown
    with caplog.at_level(logging.WARNING, logger="mkdocs.hooks.markdown_structure"):
        assert hook.on_page_content("<p/>", page=page, config=cfg, files=Files([])) == "<p/>"
        hook.on_page_content("<p/>", page=page, config=cfg, files=Files([]))
    assert [r.getMessage() for r in caplog.records] == [f"a.md: {hook.MISALIGNED} (line 4): '- b'"]
