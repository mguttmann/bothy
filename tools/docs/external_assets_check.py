# SPDX-License-Identifier: Apache-2.0
"""Check that the built documentation site loads no script, stylesheet or font from elsewhere.

A public docs site that pulls scripts, styles or fonts from another server tells that server
who reads which page and breaks the content security rules the project applies elsewhere
(brief M7-19, requirement 13). Everything a page needs is bundled into the site at build time,
so `make docs` runs this check over the built HTML and CSS after `mkdocs build`.

What fails: `<script src>`, `<link href>` with rel `stylesheet`, `preload`, `modulepreload` or
`prefetch`, and CSS `url(...)` or `@import` (in `<style>`, in `style` attributes and in `.css`
files) whose URL names a host (`https://...`, `//...`). Plain anchors to external pages and
links such as `rel="canonical"` stay allowed: they load nothing.

Not covered: requests a bundled script makes at run time (a static check cannot see them).

Usage: `python tools/docs/external_assets_check.py site` (exit 1 on a finding or an empty site).
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urlsplit

# rel values whose href the browser loads as a script, stylesheet or font.
_LOADING_RELS = frozenset({"stylesheet", "preload", "modulepreload", "prefetch"})
# `url(x)`, `url('x')`, `@import 'x'` and `@import url(x)` in CSS.
_CSS_URL = re.compile(
    r"""url\(\s*['"]?(?P<url>[^'")\s]+)|@import\s+['"](?P<imp>[^'"]+)""", re.IGNORECASE
)


@dataclass(frozen=True)
class Finding:
    """One external asset reference.

    Attributes:
        path: File of the built site that holds the reference.
        line: 1-based line of the reference in that file.
        url: The external URL as written.
    """

    path: Path
    line: int
    url: str


def is_external(url: str) -> bool:
    """Return True when `url` names a host, i.e. points to another origin.

    Relative paths, root paths and `data:` URLs name no host and stay local.

    Example: `is_external("//cdn.example.com/a.js")` is True, `is_external("a.js")` False.
    """
    return bool(urlsplit(url.strip()).netloc)


def _css_urls(text: str) -> list[tuple[int, str]]:
    """Return (line offset, URL) of every external URL in a CSS text, offsets from 0."""
    found = []
    for match in _CSS_URL.finditer(text):
        url = match.group("url") or match.group("imp")
        if is_external(url):
            found.append((text.count("\n", 0, match.start()), url))
    return found


class _PageScanner(HTMLParser):
    """Collects external asset references of one HTML page."""

    def __init__(self, path: Path) -> None:
        super().__init__(convert_charrefs=True)
        self.path = path
        self.findings: list[Finding] = []
        self._in_style = False

    def _add(self, url: str, line: int | None = None) -> None:
        self.findings.append(Finding(self.path, line or self.getpos()[0], url))

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        values = {name: value or "" for name, value in attrs}
        if tag == "script" and is_external(values.get("src", "")):
            self._add(values["src"])
        if tag == "link":
            rels = set(values.get("rel", "").lower().split())
            if rels & _LOADING_RELS and is_external(values.get("href", "")):
                self._add(values["href"])
        if style := values.get("style"):
            for _, url in _css_urls(style):
                self._add(url)
        self._in_style = tag == "style"

    def handle_endtag(self, tag: str) -> None:
        if tag == "style":
            self._in_style = False

    def handle_data(self, data: str) -> None:
        if self._in_style:
            start = self.getpos()[0]
            for offset, url in _css_urls(data):
                self._add(url, start + offset)


def scan(site: Path) -> list[Finding]:
    """Return every external script, stylesheet or font reference under `site`.

    Args:
        site: Directory of the built site (`mkdocs build --site-dir`).

    Returns:
        The findings in path order; empty when the site is clean.
    """
    findings: list[Finding] = []
    for path in sorted(site.rglob("*")):
        if path.suffix == ".html":
            scanner = _PageScanner(path)
            scanner.feed(path.read_text(encoding="utf-8", errors="replace"))
            scanner.close()
            findings.extend(scanner.findings)
        elif path.suffix == ".css":
            text = path.read_text(encoding="utf-8", errors="replace")
            findings.extend(Finding(path, off + 1, url) for off, url in _css_urls(text))
    return findings


def main(argv: list[str] | None = None) -> int:
    """Scan the built site; print each finding and return 1 if there is any.

    An empty or missing site directory also returns 1: a check over nothing proves nothing.
    """
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("site", type=Path, help="directory of the built site")
    args = parser.parse_args(argv)
    if not any(args.site.rglob("*.html")):
        print(f"external_assets_check: no HTML page under {args.site}", file=sys.stderr)
        return 1
    findings = scan(args.site)
    for f in findings:
        print(f"{f.path}:{f.line}: external asset {f.url}", file=sys.stderr)
    if findings:
        print(
            "external_assets_check: the site must bundle every script, stylesheet and font",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
