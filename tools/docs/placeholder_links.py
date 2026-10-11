# SPDX-License-Identifier: Apache-2.0
"""Fail on links to the placeholder repository `OWNER` in Markdown files.

Until the owner chooses the repository location, files name it with the placeholder `OWNER`
(PLAN P-05); V1-03 replaces every occurrence. Inside code (commands, module paths) the
placeholder is expected. A link or a bare URL in prose, however, renders as a link that leads
to a 404 page. lychee cannot see this: `make linkcheck` runs offline. This check finds every
URL outside fenced code blocks and inline code whose host or path has a label or segment that
is exactly `OWNER`. Links inside the documentation use relative paths instead; the MkDocs
hook (mkdocs_repo_links.py) builds the repository URL from `repo_url`, the one place V1-03
changes.

Usage: python tools/docs/placeholder_links.py FILE...  (exit 1 on a finding)
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

PLACEHOLDER = "OWNER"
_URL = re.compile(r"https?://[^\s<>()\"'`\]]+")
_LABEL = re.compile(r"(?:^|[/.@])" + PLACEHOLDER + r"(?:[/.:]|$)")
_FENCE = re.compile(r"^\s*(`{3,}|~{3,})")
# A code span may continue on the next line of its paragraph, never across a blank line.
_INLINE_CODE = re.compile(r"(`+)(?:(?!\1)(?:[^\n]|\n(?![ \t]*\n)))+?\1")


def _is_placeholder(url: str) -> bool:
    return _LABEL.search(url.split("://", 1)[1]) is not None


def _blank_code(text: str) -> str:
    """Replace fenced blocks and code spans by spaces; newlines stay, so line numbers hold."""
    lines: list[str] = []
    fence: str | None = None
    for line in text.splitlines():
        m = _FENCE.match(line)
        if fence is not None:
            if m and m.group(1)[0] == fence[0] and len(m.group(1)) >= len(fence):
                fence = None
            lines.append("")
            continue
        if m:
            fence = m.group(1)
            lines.append("")
            continue
        lines.append(line)
    prose = "\n".join(lines)
    return _INLINE_CODE.sub(lambda c: re.sub(r"[^\n]", " ", c.group(0)), prose)


def check_file(path: Path) -> list[str]:
    """Return one message per placeholder URL outside code, with file name and line."""
    prose = _blank_code(path.read_text(encoding="utf-8"))
    problems: list[str] = []
    for m in _URL.finditer(prose):
        if _is_placeholder(m.group(0)):
            line = prose.count("\n", 0, m.start()) + 1
            problems.append(f"{path.name}:{line}: link to placeholder repository: {m.group(0)}")
    return problems


def main(argv: list[str] | None = None) -> int:
    """Check the given Markdown files; returns 1 if a link points to the placeholder repository."""
    args = sys.argv[1:] if argv is None else argv
    problems: list[str] = []
    for name in args:
        path = Path(name)
        problems += [
            f"{path.parent}/{p}" if str(path.parent) != "." else p for p in check_file(path)
        ]
    for problem in problems:
        print(problem, file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
