# SPDX-License-Identifier: Apache-2.0
"""Tests for the MkDocs hook that points links outside docs/ to the repository host."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

import mkdocs_repo_links as hook
import pytest
from mkdocs.config.defaults import MkDocsConfig
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File, Files
from mkdocs.structure.pages import Page

HOOK = Path(hook.__file__).resolve()
URL = "https://github.com/OWNER/bothy"


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    (tmp_path / "docs" / "guide").mkdir(parents=True)
    (tmp_path / "docs" / "guide" / "page.md").write_text("# Page\n")
    (tmp_path / "infra" / "rules").mkdir(parents=True)
    (tmp_path / "infra" / "compose.yaml").write_text("services: {}\n")
    (tmp_path / "infra" / "my file.yaml").write_text("x: 1\n")
    return tmp_path


def _target(repo: Path) -> hook.Target:
    return hook.Target(
        repo_root=repo,
        docs_dir=repo / "docs",
        repo_url=URL,
        branch="main",
        page_dir=repo / "docs" / "guide",
    )


@pytest.mark.parametrize(
    ("href", "expected"),
    [
        ("../../infra/compose.yaml", f"{URL}/blob/main/infra/compose.yaml"),
        ("../../infra/compose.yaml#L3", f"{URL}/blob/main/infra/compose.yaml#L3"),
        ("../../infra/rules/", f"{URL}/tree/main/infra/rules"),
        ("../../infra/my%20file.yaml", f"{URL}/blob/main/infra/my file.yaml"),
    ],
)
def test_existing_target_outside_docs_points_to_the_repository(
    repo: Path, href: str, expected: str
) -> None:
    assert _target(repo).rewrite(href) == expected


@pytest.mark.parametrize(
    "href",
    [
        "../../infra/missing.yaml",  # missing: the strict build keeps failing on it
        "../../../outside-the-repository.txt",
        "page.md",
        "../index.md",
        "https://example.com/x",
        "mailto:alex@example.com",
        "#section",
        "/absolute/path",
        "",
    ],
)
def test_other_links_stay_unchanged(repo: Path, href: str) -> None:
    assert _target(repo).rewrite(href) is None


def test_no_page_no_rewrite(repo: Path) -> None:
    target = _target(repo)
    target.page_dir = None
    assert target.rewrite("../../infra/compose.yaml") is None


def _config(repo: Path, **values: str) -> MkDocsConfig:
    cfg = MkDocsConfig(config_file_path=str(repo / "mkdocs.yml"))
    cfg.load_dict({"site_name": "t", "docs_dir": str(repo / "docs"), **values})
    errors, _warnings = cfg.validate()
    assert errors == []
    return cfg


def test_branch_comes_from_edit_uri(repo: Path) -> None:
    target = hook.target_from_config(_config(repo, repo_url=URL + "/", edit_uri="edit/dev/docs/"))
    assert (target.repo_url, target.branch, target.repo_root) == (URL, "dev", repo.resolve())


def test_missing_edit_uri_branch_is_an_error(repo: Path) -> None:
    with pytest.raises(PluginError):
        hook.target_from_config(_config(repo, repo_url=URL, edit_uri="docs/"))


def _build(repo: Path, page: str) -> subprocess.CompletedProcess[str]:
    (repo / "docs" / "index.md").write_text(page)
    (repo / "mkdocs.yml").write_text(
        f"site_name: t\nrepo_url: {URL}\nedit_uri: edit/main/docs/\n"
        f"hooks:\n  - {HOOK}\nvalidation:\n  links:\n    not_found: warn\n"
        "    unrecognized_links: warn\n"
    )
    return subprocess.run(
        [sys.executable, "-m", "mkdocs", "build", "--strict", "-f", str(repo / "mkdocs.yml")],
        capture_output=True,
        text=True,
        check=False,
    )


def test_strict_build_passes_with_an_existing_repository_link(repo: Path) -> None:
    proc = _build(repo, "# Start\n\n[Compose](../infra/compose.yaml)\n\n`[code](../x)`\n")
    assert proc.returncode == 0, proc.stderr
    html = (repo / "site" / "index.html").read_text()
    assert f'href="{URL}/blob/main/infra/compose.yaml"' in html


def test_strict_build_still_fails_on_a_missing_target(repo: Path) -> None:
    proc = _build(repo, "# Start\n\n[Gone](../infra/gone.yaml)\n")
    assert proc.returncode == 1
    assert "../infra/gone.yaml" in proc.stderr


def test_hooks_register_the_extension_and_track_the_page(
    repo: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(hook, "_TARGET", None)
    cfg = hook.on_config(_config(repo, repo_url=URL, edit_uri="edit/main/docs/"))
    assert any(isinstance(ext, hook.RepoLinkExtension) for ext in cfg.markdown_extensions)
    page = Page(None, File("guide/page.md", str(repo / "docs"), str(repo / "site"), True), cfg)
    assert hook.on_page_markdown("# Page\n", page=page, config=cfg, files=Files([])) == "# Page\n"
    assert hook._TARGET is not None
    assert hook._TARGET.page_dir == repo.resolve() / "docs" / "guide"
