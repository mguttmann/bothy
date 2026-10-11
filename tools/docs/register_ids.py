# SPDX-License-Identifier: Apache-2.0
r"""Every ID that a table of the documentation defines is defined once in the repository.

Rulings, tests, guard rules, risks, decisions and owner questions carry stable IDs that
other documents cite without naming the file (docs/SPEC.md section 13 "Change rule", item 4;
docs/DECISIONS.md rule 1). An ID defined twice makes every citation ambiguous. A definition is a
table row in any docs/**/*.md whose first cell is exactly an ID: a prefix of capital letters and
digits, one or more parts joined by `-`, and a number at the end that may carry capital letters
in front and one small letter behind (R-AG-22, T-SC-05, SEC-TERM-01, ID-T01, AZ-M01, M1-23,
D-W-01, D-B1, RK-82, OQ-07). Decisions may also end in a word instead of a number (D-R-STACK,
D-G-DEP: `D`, then parts of capital letters only). Every such ID is unique across all files. A
table that only refers to IDs names them in a later column (the ranking table of RISKS.md does);
short local numbers without `-` (M1, D1, W1) are not IDs in this sense and stay unchecked.

docs/OPEN-QUESTIONS.md also has a heading `### OQ-<n>` per question: each heading is unique,
and headings and the rows of the overview table list the same IDs.

Every row of a table in docs/**/*.md has as many cells as the header of its table. The site
renderer pads a short row and drops the extra cells of a long one without a warning, so a row put
into the wrong table loses its content and can define an ID in a column where no ID belongs (two
rows of the 7-column threat register in a 5-column table, finding R5-01). Cells are counted like
Python-Markdown splits them: an escaped `\|` and a pipe inside a code span do not split.

Code blocks are ignored. Usage: python tools/docs/register_ids.py [--root DIR]  (exit 1 on a
finding)
"""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Iterator
from pathlib import Path

DOCS = Path("docs")
QUESTIONS = Path("docs/OPEN-QUESTIONS.md")

_FENCE = re.compile(r"^\s*(`{3,}|~{3,})")
_FIRST_CELL = re.compile(r"^\|\s*([^|]*?)\s*\|")
_ID = re.compile(r"[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[A-Z]*\d+[a-z]?|D(?:-[A-Z]+)+")
_QUESTION_ID = re.compile(r"OQ-\d+")
_QUESTION_HEADING = re.compile(r"^###\s+(OQ-\d+)\b")
_DELIMITER_ROW = re.compile(r"^\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)*\|?$")
_CODE_SPAN = re.compile(r"(`+)(?!`).*?(?<!`)\1(?!`)")


def _lines(path: Path) -> Iterator[tuple[int, str]]:
    """Yield (line number, line) of `path` outside fenced code blocks."""
    fence: str | None = None
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        match = _FENCE.match(line)
        if match:
            marker = match.group(1)[0]
            if fence is None:
                fence = marker
            elif fence == marker:
                fence = None
            continue
        if fence is None:
            yield number, line


def _first_cells(path: Path, pattern: re.Pattern[str]) -> list[tuple[int, str]]:
    """Return (line, id) of every table row whose first cell is exactly an ID of `pattern`."""
    found = []
    for number, line in _lines(path):
        match = _FIRST_CELL.match(line)
        if match and pattern.fullmatch(match.group(1)):
            found.append((number, match.group(1)))
    return found


def _duplicates(rel: Path, entries: list[tuple[int, str]], kind: str) -> list[str]:
    """Report every ID of `entries` that occurs again after its first line."""
    first: dict[str, int] = {}
    problems = []
    for number, ident in entries:
        if ident in first:
            problems.append(
                f"{rel}:{number}: {kind} {ident} is already defined in line {first[ident]}"
            )
        else:
            first[ident] = number
    return problems


def _table_ids(root: Path) -> list[str]:
    """Report every table ID of docs/**/*.md that an earlier row or file already defined."""
    first: dict[str, tuple[Path, int]] = {}
    problems = []
    for path in sorted((root / DOCS).rglob("*.md")):
        rel = path.relative_to(root)
        for number, ident in _first_cells(path, _ID):
            if ident in first:
                where, line = first[ident]
                place = f"line {line}" if where == rel else f"{where} line {line}"
                problems.append(f"{rel}:{number}: ID {ident} is already defined in {place}")
            else:
                first[ident] = (rel, number)
    return problems


def _cells(row: str) -> int:
    r"""Count the cells of a table row; `\|` and pipes inside a code span do not split.

    An escaped backslash (`\\`) escapes nothing, so a pipe after it splits.
    """
    body = _CODE_SPAN.sub("code", row.strip().replace("\\\\", "e").replace("\\|", "e"))
    return body.removeprefix("|").removesuffix("|").count("|") + 1


def _table_widths(root: Path) -> list[str]:
    """Report every table row of docs/**/*.md whose cell count differs from its header."""
    problems = []
    for path in sorted((root / DOCS).rglob("*.md")):
        rel = path.relative_to(root)
        rows = list(_lines(path))
        header: tuple[int, int] | None = None
        for index, (number, line) in enumerate(rows):
            text = line.strip()
            if not text.startswith("|"):
                header = None
                continue
            if header is None:
                following = rows[index + 1] if index + 1 < len(rows) else (0, "")
                if following[0] == number + 1 and _DELIMITER_ROW.match(following[1].strip()):
                    header = (number, _cells(text))
                continue
            if number == header[0] + 1:
                continue
            width = _cells(text)
            if width != header[1]:
                problems.append(
                    f"{rel}:{number}: table row has {width} cells, "
                    f"its header in line {header[0]} has {header[1]}"
                )
    return problems


def check(root: Path) -> list[str]:
    """Return one message per duplicate table ID, misshapen table row or inconsistent question."""
    problems = _table_ids(root) + _table_widths(root)
    rows = _first_cells(root / QUESTIONS, _QUESTION_ID)
    headings = [
        (number, m.group(1))
        for number, line in _lines(root / QUESTIONS)
        if (m := _QUESTION_HEADING.match(line))
    ]
    problems += _duplicates(QUESTIONS, headings, "heading")
    in_rows = {ident for _, ident in rows}
    in_headings = {ident for _, ident in headings}
    for number, ident in headings:
        if ident not in in_rows:
            problems.append(
                f"{QUESTIONS}:{number}: question {ident} has a heading, no overview row"
            )
    for number, ident in rows:
        if ident not in in_headings:
            problems.append(
                f"{QUESTIONS}:{number}: question {ident} has an overview row, no heading"
            )
    return problems


def main(argv: list[str] | None = None) -> int:
    """Run the check and print every finding to stderr."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", type=Path, default=Path(), help="repository root")
    args = parser.parse_args(argv)
    problems = check(args.root)
    for problem in problems:
        print(problem, file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
