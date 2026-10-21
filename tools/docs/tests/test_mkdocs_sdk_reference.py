# SPDX-License-Identifier: Apache-2.0
"""Tests for the MkDocs hook that mounts the typedoc reference of the TypeScript SDK.

The hook adds every file of `sdk/typescript/build/reference/` to the site under
`api/typescript-reference/` (docs/DECISIONS.md D-M2-30-13). `make linkcheck` checks the tracked
Markdown files on disk, where that directory does not exist, so its lychee call excludes
exactly the reference's URL pattern; the last tests hold that pattern to the reference path.
"""

from __future__ import annotations

import os
import re
import shlex
import subprocess  # nosec B404 - runs git on temporary repositories and the tracked file list
from pathlib import Path
from typing import Any

import mkdocs_sdk_reference as hook
import pytest
import yaml
from mkdocs.commands.build import build
from mkdocs.config import load_config
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File, Files

REPO = Path(__file__).resolve().parents[3]
HOOK = Path(hook.__file__).resolve()
_LINK = re.compile(r"\]\((?P<target>[^)\s]+)(?:\s+\"[^\"]*\")?\)")


def _git(root: Path, *args: str) -> None:
    env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"}
    subprocess.run(["git", "-C", str(root), *args], check=True, capture_output=True, env=env)


def _repo(tmp_path: Path, *, tracked: bool) -> Path:
    """A repository with docs/ and, when `tracked`, a committed sdk/typescript/package.json."""
    root = tmp_path / "repo"
    docs = root / "docs" / "api"
    docs.mkdir(parents=True)
    (root / "docs" / "index.md").write_text("# Start\n")
    (docs / "sdk-typescript.md").write_text(
        "# TypeScript SDK\n\n[API reference](typescript-reference/index.html)\n"
    )
    package = root / "sdk" / "typescript" / "package.json"
    package.parent.mkdir(parents=True)
    package.write_text('{"name": "bothy-sdk", "private": true}\n')
    _git(root, "init", "--quiet")
    if tracked:
        _git(root, "add", "sdk/typescript/package.json")
    return root


def _reference(root: Path) -> dict[str, str]:
    """Write a small typedoc output and return its files by relative path."""
    files = {
        "index.html": "<html><body>bothy-sdk</body></html>\n",
        "modules/core.html": "<html><body>core</body></html>\n",
        "assets/style.css": "body { color: black; }\n",
    }
    for rel, text in files.items():
        path = root / hook.REFERENCE_DIR / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
    return files


def _config(root: Path) -> Any:
    cfg = root / "mkdocs.yml"
    cfg.write_text(
        yaml.safe_dump(
            {
                "site_name": "t",
                "docs_dir": str(root / "docs"),
                "site_dir": str(root / "site"),
                "theme": {"name": "material", "font": False},
                "hooks": [str(HOOK)],
                "validation": {"links": {"not_found": "warn", "unrecognized_links": "warn"}},
                "nav": [{"Start": "index.md"}, {"TypeScript SDK": "api/sdk-typescript.md"}],
            }
        )
    )
    return load_config(str(cfg), strict=True)


def _run(config: Any, files: Files | None = None) -> Files:
    """Run the `on_files` event of the configured hooks, as a build does."""
    result: Files = config.plugins.on_files(files or Files([]), config=config)
    return result


def test_mounts_every_file_of_the_reference(tmp_path: Path) -> None:
    root = _repo(tmp_path, tracked=True)
    written = _reference(root)
    files = _run(_config(root))
    mounted = {f.src_uri: f for f in files}
    assert set(mounted) == {f"{hook.MOUNT}{rel}" for rel in written}
    for rel, text in written.items():
        file = mounted[f"{hook.MOUNT}{rel}"]
        assert file.generated_by is not None
        assert Path(file.abs_src_path or "").read_text() == text
        assert not file.is_documentation_page()


def test_strict_build_resolves_the_link_of_the_page(tmp_path: Path) -> None:
    root = _repo(tmp_path, tracked=True)
    _reference(root)
    build(_config(root))
    assert (root / "site" / "api" / "typescript-reference" / "index.html").is_file()
    assert (root / "site" / "api" / "typescript-reference" / "assets" / "style.css").is_file()
    page = (root / "site" / "api" / "sdk-typescript" / "index.html").read_text()
    # With directory URLs MkDocs writes the link to index.html as its directory.
    assert 'href="../typescript-reference/"' in page


@pytest.mark.parametrize("state", ["missing", "empty"])
def test_tracked_package_without_reference_fails_naming_the_command(
    tmp_path: Path, state: str
) -> None:
    root = _repo(tmp_path, tracked=True)
    if state == "empty":
        (root / hook.REFERENCE_DIR).mkdir(parents=True)
    with pytest.raises(PluginError, match=re.escape(hook.NPM_COMMAND)):
        _run(_config(root))


def test_untracked_package_mounts_nothing(tmp_path: Path) -> None:
    root = _repo(tmp_path, tracked=False)
    _reference(root)
    files = _run(_config(root))
    assert list(files) == []


def test_committed_file_under_the_mount_fails(tmp_path: Path) -> None:
    root = _repo(tmp_path, tracked=True)
    _reference(root)
    config = _config(root)
    stale = root / "docs" / "api" / "typescript-reference" / "index.html"
    stale.parent.mkdir(parents=True)
    stale.write_text("old\n")
    files = Files([File(f"{hook.MOUNT}index.html", config.docs_dir, config.site_dir, False)])
    with pytest.raises(PluginError, match="generated"):
        config.plugins.on_files(files, config=config)


def test_site_config_registers_the_hook_and_the_page() -> None:
    text = (REPO / "mkdocs.yml").read_text(encoding="utf-8")
    assert "tools/docs/mkdocs_sdk_reference.py" in text
    assert "api/sdk-typescript.md" in text
    page = (REPO / "docs" / "api" / "sdk-typescript.md").read_text(encoding="utf-8")
    assert "[API reference](typescript-reference/index.html)" in page


# ---- linkcheck ----------------------------------------------------------------------------------


def linkcheck_excludes() -> list[str]:
    """The `--exclude` patterns of the lychee call in the `linkcheck` recipe of the Makefile."""
    makefile = (REPO / "Makefile").read_text(encoding="utf-8")
    recipe = makefile.split("\nlinkcheck:", 1)[1].split("\n\n", 1)[0]
    lychee = next(line for line in recipe.splitlines() if "lychee" in line and "--offline" in line)
    words = shlex.split(lychee.replace("$(call tool,lychee)", "lychee").strip())
    return [words[i + 1] for i, word in enumerate(words) if word == "--exclude"]


def _file_url(path: Path) -> str:
    """The URL form lychee gives a local link target (`file://` and the absolute path)."""
    return path.resolve().as_uri()


def _tracked_markdown() -> list[Path]:
    out = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "-z", "--", "*.md"],
        check=True,
        capture_output=True,
    ).stdout
    return [REPO / name for name in out.decode().split("\0") if name]


def test_linkcheck_excludes_exactly_the_reference_pattern() -> None:
    patterns = linkcheck_excludes()
    assert patterns == [hook.LINKCHECK_EXCLUDE]
    exclude = re.compile(hook.LINKCHECK_EXCLUDE)
    reference = REPO / "docs" / "api" / "typescript-reference"
    assert exclude.search(_file_url(reference / "index.html"))
    assert exclude.search(_file_url(reference / "modules" / "core.html"))
    assert not exclude.search(_file_url(REPO / "docs" / "api" / "sdk-typescript.md"))
    assert not exclude.search(_file_url(REPO / "sdk" / "typescript" / "build" / "reference"))
    assert not exclude.search("https://example.com/docs/api/typescript-reference/index.html")


def test_linkcheck_exclude_matches_no_other_tracked_link() -> None:
    """Every tracked file and every link of a tracked Markdown file stays checked."""
    exclude = re.compile(hook.LINKCHECK_EXCLUDE)
    reference = REPO / "docs" / "api" / "typescript-reference"
    tracked = _tracked_markdown()
    assert not [p for p in tracked if exclude.search(_file_url(p))]
    hidden = []
    for page in tracked:
        for match in _LINK.finditer(page.read_text(encoding="utf-8", errors="replace")):
            target = match.group("target").split("#", 1)[0]
            if not target or re.match(r"^[a-z][a-z0-9+.-]*:", target):
                continue
            url = _file_url(page.parent / target)
            if exclude.search(url) and not (page.parent / target).resolve().is_relative_to(
                reference.resolve()
            ):
                hidden.append(f"{page.relative_to(REPO)}: {target}")
    assert hidden == []
