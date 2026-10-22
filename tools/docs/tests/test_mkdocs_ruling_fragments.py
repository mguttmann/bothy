# SPDX-License-Identifier: Apache-2.0
"""Tests for the MkDocs hook that writes the index page of docs/decisions.d/ (PL/24)."""

from __future__ import annotations

from pathlib import Path

import mkdocs_ruling_fragments as hook
import pytest
import yaml
from mkdocs.commands.build import build
from mkdocs.config import load_config
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File, Files

REPO = Path(__file__).resolve().parents[3]
HOOK = Path(hook.__file__).resolve()


def _config(tmp_path: Path):
    docs = tmp_path / "docs"
    (docs / "decisions.d").mkdir(parents=True)
    (docs / "index.md").write_text("# Start\n", encoding="utf-8")
    (docs / "DECISIONS.md").write_text("# Ruling log\n", encoding="utf-8")
    (docs / "DECISIONS-FOUNDATION.md").write_text("# Foundation\n", encoding="utf-8")
    for name, title in (
        ("M1-10", "M1-10 Ten"),
        ("M1-9", "M1-9 Nine"),
        ("int-m1-wave2", "Integration"),
    ):
        (docs / "decisions.d" / f"{name}.md").write_text(
            f"# {title}\n\nSee [the log](../DECISIONS.md).\n", encoding="utf-8"
        )
    cfg = tmp_path / "mkdocs.yml"
    cfg.write_text(
        yaml.safe_dump(
            {
                "site_name": "t",
                "docs_dir": str(docs),
                "site_dir": str(tmp_path / "site"),
                "hooks": [str(HOOK)],
                "validation": {"links": {"not_found": "warn"}, "nav": {"omitted_files": "warn"}},
                "not_in_nav": "/decisions.d/*.md\n/DECISIONS-FOUNDATION.md\n",
                "nav": [{"Start": "index.md"}, {"Log": "DECISIONS.md"}, {hook.TITLE: hook.INDEX}],
            }
        )
    )
    return load_config(str(cfg), strict=True)


def test_the_index_lists_every_fragment_in_natural_order(tmp_path: Path) -> None:
    config = _config(tmp_path)
    build(config)
    page = (tmp_path / "site" / "decisions.d" / "index.html").read_text(encoding="utf-8")
    assert '<a href="M1-9/">M1-9 Nine</a>' in page
    assert (tmp_path / "site" / "decisions.d" / "M1-10" / "index.html").is_file()
    files = Files(
        [
            File(f"decisions.d/{n}.md", config.docs_dir, config.site_dir, False)
            for n in ("M1-10", "int-m1-wave2", "M1-9")
        ]
    )
    text = hook.render(list(files))
    assert text.index("(M1-9.md)") < text.index("(M1-10.md)")
    assert text.index("(int-m1-wave2.md)") < text.index("(M1-9.md)")
    assert "- [M1-10 Ten](M1-10.md)" in text


def test_a_committed_index_fails(tmp_path: Path) -> None:
    config = _config(tmp_path)
    files = Files([File(hook.INDEX, config.docs_dir, config.site_dir, False)])
    with pytest.raises(PluginError, match="generated"):
        hook.on_files(files, config=config)


def test_site_config_registers_the_hook_and_the_page() -> None:
    text = (REPO / "mkdocs.yml").read_text(encoding="utf-8")
    assert "tools/docs/mkdocs_ruling_fragments.py" in text
    assert hook.INDEX in text
    assert "/decisions.d/*.md" in text
