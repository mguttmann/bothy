# SPDX-License-Identifier: Apache-2.0
"""MkDocs hook: Markdown that the site renders differently from its source fails the build.

GitHub renders Markdown after CommonMark; the site renders it with Python-Markdown. The two
disagree on lists: Python-Markdown nests a block inside a list item only when it is indented by
four spaces per level, and it starts a list only after a blank line. A sublist indented by two
spaces looks right on GitHub, but on the site its items become siblings of the outer item or run
into the paragraph before them as plain text ("4. ... 5. ... - ..."), and a paragraph indented
by four spaces after such a broken list turns into a code block. `mkdocs build --strict` stays
green in all these cases, because nothing is missing. This hook finds three signs while a page
is built and logs one warning per finding; the strict build (`make docs`) fails on warnings:

1. a line outside fenced code that starts with a list marker (`-`, `*`, `+` or a number with
   `.` or `)`, each followed by a space) after a number of spaces that is not a multiple of
   four: Python-Markdown reads its level from multiples of four;
2. a paragraph or list item whose text has a line that starts with a list marker: a list that
   Python-Markdown did not see, mostly because the blank line before it is missing;
3. a code block made by indentation instead of a fence: the site's code blocks are fenced
   (pymdownx.superfences), so an indented block is a paragraph that lost its list.

Registered in mkdocs.yml under `hooks:`.
"""

from __future__ import annotations

import logging
import re
from dataclasses import dataclass, field
from typing import TYPE_CHECKING

from markdown import Markdown
from markdown.blockparser import BlockParser
from markdown.blockprocessors import BlockProcessor
from markdown.extensions import Extension
from markdown.treeprocessors import Treeprocessor
from mkdocs.config.defaults import MkDocsConfig
from mkdocs.structure.files import Files
from mkdocs.structure.pages import Page

if TYPE_CHECKING:
    # Type only: the hook walks the tree Markdown built and parses no XML.
    from xml.etree.ElementTree import Element  # nosec B405

_MARKER = r"(?:[-*+]|\d{1,9}[.)])[ \t]+\S"
# A source line that starts with a list marker, with its indentation.
_MARKER_LINE = re.compile(rf"^(?P<indent> *)(?P<item>{_MARKER}.*)$")
# A line inside a paragraph text that starts like a list item.
_LIST_LINE = re.compile(rf"\n[ \t]*(?P<item>{_MARKER}[^\n]*)")
# Opening or closing line of a fenced code block.
_FENCE = re.compile(r"^ *(?P<fence>`{3,}|~{3,})")
_INDENT = 4
# Before Markdown's inline processor (priority 20): the paragraph text is still the source.
_TREE_PRIORITY = 30
# Python-Markdown registers its indented code block processor as "code" with priority 80;
# pymdownx.superfences replaces it under the same name and priority.
_CODE_NAME = "code"
_CODE_PRIORITY = 80
_SNIPPET = 60

MISALIGNED = "list marker not indented by a multiple of 4 spaces"
SWALLOWED = "list marker inside a paragraph (blank line or 4-space indent missing)"
INDENTED_CODE = "code block without fence (indented paragraph?)"

log = logging.getLogger("mkdocs.hooks.markdown_structure")


def _snippet(text: str) -> str:
    return " ".join(text.split())[:_SNIPPET]


@dataclass
class Findings:
    """Findings of the page being rendered."""

    page: str = ""
    items: list[str] = field(default_factory=list)

    def add(self, kind: str, text: str) -> None:
        """Record one finding with the start of the text it concerns."""
        self.items.append(f"{kind}: '{_snippet(text)}'")


def misaligned_markers(markdown: str) -> list[tuple[int, str]]:
    """Return line number and text of list markers outside fences not indented by 4 per level."""
    found: list[tuple[int, str]] = []
    fence = ""
    for number, raw in enumerate(markdown.splitlines(), start=1):
        line = raw.expandtabs(_INDENT)
        opener = _FENCE.match(line)
        if fence:
            if (
                opener
                and opener.group("fence")[0] == fence[0]
                and len(opener.group("fence")) >= len(fence)
            ):
                fence = ""
            continue
        if opener:
            fence = opener.group("fence")
            continue
        marker = _MARKER_LINE.match(line)
        if marker and len(marker.group("indent")) % _INDENT:
            found.append((number, marker.group("item")))
    return found


def list_lines(text: str) -> list[str]:
    """Return the lines of a paragraph text that start with a list marker."""
    return [m.group("item").strip() for m in _LIST_LINE.finditer(text)]


class _ListLineProcessor(Treeprocessor):
    def __init__(self, md: Markdown, findings: Findings) -> None:
        super().__init__(md)
        self.findings = findings

    def run(self, root: Element) -> None:
        for element in root.iter():
            if element.tag in ("p", "li") and element.text:
                for line in list_lines(element.text):
                    self.findings.add(SWALLOWED, line)


class _IndentedCodeRecorder(BlockProcessor):
    """Delegate to the registered code block processor and record each block it takes."""

    def __init__(self, parser: BlockParser, inner: BlockProcessor, findings: Findings) -> None:
        super().__init__(parser)
        self.inner = inner
        self.findings = findings

    def test(self, parent: Element, block: str) -> bool:
        return bool(self.inner.test(parent, block))

    def run(self, parent: Element, blocks: list[str]) -> bool | None:
        first = blocks[0] if blocks else ""
        handled = self.inner.run(parent, blocks)
        # superfences returns False for a block it leaves to the next processor; Markdown's own
        # processor returns None after it made a code block.
        if handled is not False:
            self.findings.add(INDENTED_CODE, first)
        return handled


class MarkdownStructureExtension(Extension):
    """Markdown extension that records lists and code blocks the source did not mean."""

    def __init__(self, findings: Findings) -> None:
        """Keep the findings of the current page."""
        super().__init__()
        self.findings = findings

    def extendMarkdown(self, md: Markdown) -> None:  # noqa: N802 (Markdown API name)
        """Register the list check and wrap the indented code block processor."""
        md.treeprocessors.register(
            _ListLineProcessor(md, self.findings), "markdown_structure_lists", _TREE_PRIORITY
        )
        processors = md.parser.blockprocessors
        if _CODE_NAME in processors:
            inner = processors[_CODE_NAME]
            processors.register(
                _IndentedCodeRecorder(md.parser, inner, self.findings), _CODE_NAME, _CODE_PRIORITY
            )


_FINDINGS = Findings()


def on_config(config: MkDocsConfig) -> MkDocsConfig:
    """MkDocs hook: add the extension after the configured ones (superfences included)."""
    # Markdown takes extension instances; MkDocs types the validated list as names only.
    config.markdown_extensions.append(MarkdownStructureExtension(_FINDINGS))  # type: ignore[arg-type]
    return config


def on_page_markdown(markdown: str, *, page: Page, config: MkDocsConfig, files: Files) -> str:
    """MkDocs hook: start the findings of the page and check the list indentation."""
    del config, files
    _FINDINGS.page = page.file.src_uri
    _FINDINGS.items.clear()
    for number, item in misaligned_markers(markdown):
        _FINDINGS.add(f"{MISALIGNED} (line {number})", item)
    return markdown


def on_page_content(html: str, *, page: Page, config: MkDocsConfig, files: Files) -> str:
    """MkDocs hook: log every finding of the rendered page as a warning."""
    del page, config, files
    for item in _FINDINGS.items:
        log.warning("%s: %s", _FINDINGS.page, item)
    _FINDINGS.items.clear()
    return html
