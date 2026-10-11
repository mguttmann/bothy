# SPDX-License-Identifier: Apache-2.0
"""Generate the ADR index table in docs/adr/README.md from the ADR files.

ADR files are named NNNN-title-in-kebab-case.md. Status and date come from the MADR
front matter (`status:`, `date:`), else from a `Status:` / `Datum:` / `Date:` line near
the top; a missing value shows as MISSING in the table. The table between the markers
in README.md is generated; the text around it is hand-written. `--check` fails if the
committed table is stale.
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

START = "<!-- adr-index:start -->"
END = "<!-- adr-index:end -->"
DEFAULT_DIR = Path(__file__).resolve().parents[2] / "docs" / "adr"
_NAME = re.compile(r"^(\d{4})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$")
_FRONT = re.compile(r"\A---\n(?P<meta>.*?)\n---\n", re.DOTALL)
_TITLE = re.compile(r"^# (?P<title>.+)$", re.MULTILINE)
_BODY_FIELD = re.compile(
    r"^\W*(?P<key>status|datum|date)\W*:\s*(?P<value>[^\n]+?)\s*$", re.IGNORECASE | re.MULTILINE
)
MISSING = "MISSING"


class AdrError(Exception):
    """An ADR file does not follow the naming or front matter rules."""


@dataclass(frozen=True)
class Entry:
    """One ADR row: number, file name, title, status and date."""

    number: str
    file: str
    title: str
    status: str
    date: str


def _meta(text: str) -> dict[str, str]:
    m = _FRONT.match(text)
    if not m:
        return {}
    meta = {}
    for line in m.group("meta").splitlines():
        key, sep, value = line.partition(":")
        if sep:
            meta[key.strip()] = value.strip().strip("\"'")
    return meta


def _fields(text: str) -> tuple[str, str]:
    meta = _meta(text)
    status, date = meta.get("status", ""), meta.get("date", "")
    head = "\n".join(text.splitlines()[:30])
    for m in _BODY_FIELD.finditer(head):
        key, value = m.group("key").lower(), m.group("value").strip("*_` ")
        if key == "status" and not status:
            status = value
        elif key in ("date", "datum") and not date:
            date = value
    return status or MISSING, date or MISSING


def collect(directory: Path) -> list[Entry]:
    """Read all ADRs in `directory`; raises AdrError for a bad name, number or title."""
    entries: list[Entry] = []
    seen: dict[str, str] = {}
    for path in sorted(directory.glob("*.md")):
        if path.name == "README.md":
            continue
        m = _NAME.match(path.name)
        if not m:
            raise AdrError(f"{path.name}: name must be NNNN-title-in-kebab-case.md")
        number = m.group(1)
        if number in seen:
            raise AdrError(f"{number}: used by {seen[number]} and {path.name}")
        seen[number] = path.name
        text = path.read_text(encoding="utf-8")
        title = _TITLE.search(text)
        if title is None:
            raise AdrError(f"{path.name}: needs a '# ' title")
        status, date = _fields(text)
        entries.append(Entry(number, path.name, title.group("title").strip(), status, date))
    return entries


def render(readme: str, entries: list[Entry]) -> str:
    """Return the README with the generated index between the markers; raises AdrError."""
    if START not in readme or END not in readme:
        raise AdrError(f"README.md needs the markers {START} and {END}")
    rows = ["| Nr | Titel | Status | Datum |", "|---|---|---|---|"]
    rows += [f"| [{e.number}]({e.file}) | {e.title} | {e.status} | {e.date} |" for e in entries]
    head, rest = readme.split(START, 1)
    _, tail = rest.split(END, 1)
    return f"{head}{START}\n" + "\n".join(rows) + f"\n{END}{tail}"


def main(argv: list[str] | None = None) -> int:
    """Write or check (`--check`) the ADR index; returns the process exit code."""
    parser = argparse.ArgumentParser(description="Generate or check the ADR index.")
    parser.add_argument("--dir", type=Path, default=DEFAULT_DIR)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args(argv)
    readme = args.dir / "README.md"
    try:
        current = readme.read_text(encoding="utf-8")
        wanted = render(current, collect(args.dir))
    except (AdrError, FileNotFoundError) as exc:
        print(f"adr_index: {exc}", file=sys.stderr)
        return 1
    if args.check:
        if current != wanted:
            print("adr_index: docs/adr/README.md is stale; run `make adr-index`", file=sys.stderr)
            return 1
        return 0
    readme.write_text(wanted, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
