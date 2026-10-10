# SPDX-License-Identifier: Apache-2.0
"""MkDocs hook: links to repository files outside docs/ point to the repository host.

Pages link to configuration files and READMEs outside the documentation directory with
relative paths (`../../infra/compose/compose.yaml`). Those links work on GitHub and pass
lychee's offline check (`make linkcheck`), but the site holds only docs/, so the strict
build (`make docs`) would report them as missing. This hook rewrites such a link to
`<repo_url>/blob/<branch>/<path>` (`/tree/` for a directory) when the target exists inside
the repository. A target that does not exist or lies outside the repository stays as it
is, and the strict build fails on it as before. Links inside docs/ are left to MkDocs;
images are not rewritten (a repository page is not an image).

The branch comes from `edit_uri` (`edit/<branch>/docs/` or `blob/<branch>/docs/`), so
mkdocs.yml names it once. Registered in mkdocs.yml under `hooks:`.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass
from pathlib import Path, PurePosixPath
from typing import TYPE_CHECKING
from urllib.parse import unquote, urlsplit

from markdown import Markdown
from markdown.extensions import Extension
from markdown.treeprocessors import Treeprocessor
from mkdocs.config.defaults import MkDocsConfig
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import Files
from mkdocs.structure.pages import Page

if TYPE_CHECKING:
    # Type only: the hook walks the tree Markdown built and parses no XML.
    from xml.etree.ElementTree import Element  # nosec B405

_BRANCH = re.compile(r"^(?:edit|blob)/(?P<branch>[^/]+)/")
# After Markdown's inline processor (priority 20) and before MkDocs' link validation (0).
_PRIORITY = 8


@dataclass
class Target:
    """Where a repository link of the current page points to."""

    repo_root: Path
    docs_dir: Path
    repo_url: str
    branch: str
    page_dir: Path | None = None

    def rewrite(self, href: str) -> str | None:
        """Return the repository URL for `href`, or None to leave the link unchanged."""
        if self.page_dir is None:
            return None
        parts = urlsplit(href)
        if parts.scheme or parts.netloc or not parts.path or href.startswith(("/", "#")):
            return None
        target = Path(os.path.normpath(self.page_dir / unquote(parts.path)))
        if target.is_relative_to(self.docs_dir) or not target.is_relative_to(self.repo_root):
            return None
        if not target.exists():
            return None
        kind = "tree" if target.is_dir() else "blob"
        path = target.relative_to(self.repo_root).as_posix()
        fragment = f"#{parts.fragment}" if parts.fragment else ""
        return f"{self.repo_url}/{kind}/{self.branch}/{path}{fragment}"


class _RepoLinkProcessor(Treeprocessor):
    def __init__(self, md: Markdown, target: Target) -> None:
        super().__init__(md)
        self.target = target

    def run(self, root: Element) -> None:
        for anchor in root.iter("a"):
            href = anchor.get("href")
            new = self.target.rewrite(href) if href else None
            if new is not None:
                anchor.set("href", new)


class RepoLinkExtension(Extension):
    def __init__(self, target: Target) -> None:
        super().__init__()
        self.target = target

    def extendMarkdown(self, md: Markdown) -> None:  # noqa: N802 (Markdown API name)
        md.treeprocessors.register(_RepoLinkProcessor(md, self.target), "repo_links", _PRIORITY)


def target_from_config(config: MkDocsConfig) -> Target:
    """Build the link target from mkdocs.yml (`repo_url`, `edit_uri`, `docs_dir`)."""
    repo_url = (config.repo_url or "").rstrip("/")
    match = _BRANCH.match(config.edit_uri or "")
    if not repo_url or match is None:
        raise PluginError(
            "mkdocs_repo_links: mkdocs.yml needs repo_url and edit_uri of the form "
            "'edit/<branch>/docs/'"
        )
    return Target(
        repo_root=Path(config.config_file_path).resolve().parent,
        docs_dir=Path(config.docs_dir).resolve(),
        repo_url=repo_url,
        branch=match.group("branch"),
    )


_TARGET: Target | None = None


def on_config(config: MkDocsConfig) -> MkDocsConfig:
    global _TARGET  # noqa: PLW0603 (MkDocs hooks keep build state at module level)
    _TARGET = target_from_config(config)
    # Markdown takes extension instances; MkDocs types the validated list as names only.
    config.markdown_extensions.append(RepoLinkExtension(_TARGET))  # type: ignore[arg-type]
    return config


def on_page_markdown(markdown: str, *, page: Page, config: MkDocsConfig, files: Files) -> str:
    del config, files
    if _TARGET is not None:
        _TARGET.page_dir = _TARGET.docs_dir / PurePosixPath(page.file.src_uri).parent
    return markdown
