# SPDX-License-Identifier: Apache-2.0
"""Tests for the check that the built site loads no script, stylesheet or font from elsewhere."""

from __future__ import annotations

from pathlib import Path

import external_assets_check as check
import pytest


def _site(tmp_path: Path, html: str, name: str = "index.html") -> Path:
    page = tmp_path / name
    page.parent.mkdir(parents=True, exist_ok=True)
    page.write_text(f"<!doctype html><html><head></head><body>{html}</body></html>\n")
    return tmp_path


@pytest.mark.parametrize(
    "html",
    [
        '<script src="https://cdn.example.com/lib.js"></script>',
        '<script src="//cdn.example.com/lib.js"></script>',
        '<script type="module" src="HTTP://cdn.example.com/lib.js"></script>',
        '<link rel="stylesheet" href="https://cdn.example.com/site.css">',
        '<link rel="preload" as="font" href="https://fonts.example.com/a.woff2">',
        '<link rel="modulepreload" href="https://cdn.example.com/m.js">',
        "<style>@import url(https://fonts.example.com/css?family=A);</style>",
        "<style>@font-face{src:url('https://fonts.example.com/a.woff2')}</style>",
        '<div style="background:url(https://cdn.example.com/x.png)"></div>',
    ],
)
def test_external_asset_fails(tmp_path: Path, html: str) -> None:
    findings = check.scan(_site(tmp_path, html))
    assert len(findings) == 1
    assert findings[0].path.name == "index.html"


@pytest.mark.parametrize(
    "html",
    [
        '<script src="assets/javascripts/bundle.js"></script>',
        '<script src="/assets/app.js"></script>',
        "<script>var x = 1;</script>",
        '<link rel="stylesheet" href="../assets/site.css">',
        '<a href="https://example.com/guide">Guide</a>',
        '<link rel="canonical" href="https://docs.example.com/page/">',
        "<style>@font-face{src:url('../fonts/a.woff2')}</style>",
        '<img alt="" src="data:image/png;base64,AAAA">',
    ],
)
def test_local_asset_and_external_anchor_pass(tmp_path: Path, html: str) -> None:
    assert check.scan(_site(tmp_path, html)) == []


def test_external_font_in_a_stylesheet_fails(tmp_path: Path) -> None:
    css = tmp_path / "assets" / "site.css"
    css.parent.mkdir(parents=True)
    css.write_text("@font-face { src: url(https://fonts.example.com/a.woff2); }\n")
    findings = check.scan(tmp_path)
    assert [f.path for f in findings] == [css]
    assert "fonts.example.com" in findings[0].url


def test_nested_pages_are_scanned(tmp_path: Path) -> None:
    _site(tmp_path, '<script src="https://cdn.example.com/a.js"></script>', "a/b/index.html")
    findings = check.scan(tmp_path)
    assert [str(f.path.relative_to(tmp_path)) for f in findings] == ["a/b/index.html"]
    assert findings[0].line == 1


def test_main_reports_and_fails(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    _site(tmp_path, '<script src="https://cdn.example.com/a.js"></script>')
    assert check.main([str(tmp_path)]) == 1
    err = capsys.readouterr().err
    assert "index.html:1" in err
    assert "https://cdn.example.com/a.js" in err


def test_main_passes_a_clean_site(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    _site(tmp_path, '<a href="https://example.com/">x</a>')
    assert check.main([str(tmp_path)]) == 0
    assert capsys.readouterr().err == ""


def test_main_fails_without_pages(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert check.main([str(tmp_path / "missing")]) == 1
    assert "no HTML page" in capsys.readouterr().err
