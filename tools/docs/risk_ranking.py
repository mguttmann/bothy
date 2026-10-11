# SPDX-License-Identifier: Apache-2.0
r"""The ranking of the largest risks in docs/RISKS.md follows the ratings of its register.

docs/RISKS.md ranks its largest risks by three criteria in order; the first is the rating of
the register, likelihood times impact (`low` 1, `medium` 2, `high` 3). This check holds the
part a program can decide: every ranked risk stands in the register with the same likelihood
and impact, ranks count from 1 without gaps or repeats, the rating never rises down the ranking,
no risk of the register outside the ranking is rated higher than the last ranked one, and the
list behind the last rank names exactly the risks outside the ranking rated like it (D-W-79).
A risk raised in the register without a new ranking fails here (RK-77 before D-W-78), and so
does one raised to the rating of the last rank without joining that list. Which criterion
decides between risks of the same rating is a judgement; the milestone gate reviews it
(docs/PLAN.md section 4 item 6).

The ranking table is the one table whose header starts with `Rank | ID`; register tables start
with `ID` and have the columns `Likelihood` and `Impact`. Cells split like
Python-Markdown splits them (an escaped `\|` and a pipe in a code span do not split); code
blocks are ignored. The list behind the last rank starts with a line `**Behind rank <n>**` and
ends at the next heading; it names a risk by its ID in the bold span that starts a list item, so
a comparison in the running text names nothing.
Usage: python tools/docs/risk_ranking.py [--root DIR]  (exit 1 on a finding)
"""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Iterator
from dataclasses import dataclass
from pathlib import Path

RISKS = Path("docs/RISKS.md")
LEVELS = {"low": 1, "medium": 2, "high": 3}
LIKELIHOOD = "Likelihood"
IMPACT = "Impact"

_FENCE = re.compile(r"^\s*(`{3,}|~{3,})")
_DELIMITER_ROW = re.compile(r"^\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)*\|?$")
_CODE_SPAN = re.compile(r"(`+)(?!`).*?(?<!`)\1(?!`)")
_BEHIND = re.compile(r"^\*\*Behind rank (\d+)\*\*")
_HEADING = re.compile(r"^#{1,6}\s")
_LIST_ITEM = re.compile(r"^\s*[-*+]\s+\*\*(.+?)\*\*")
_RISK_ID = re.compile(r"\bRK-\d+\b")


@dataclass(frozen=True)
class Rated:
    """A table row with a risk ID, its likelihood and impact."""

    line: int
    ident: str
    likelihood: str
    impact: str

    def score(self) -> int | None:
        """Return likelihood times impact, or None if a word is not a level."""
        if self.likelihood in LEVELS and self.impact in LEVELS:
            return LEVELS[self.likelihood] * LEVELS[self.impact]
        return None

    def rating(self) -> str:
        """Return the rating as written, e.g. `medium x high`."""
        return f"{self.likelihood} x {self.impact}"


@dataclass(frozen=True)
class Behind:
    """The list behind the last rank: its first line, rank number and named (line, ID) pairs."""

    line: int
    rank: str
    named: tuple[tuple[int, str], ...]


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


def _cells(row: str) -> list[str]:
    r"""Split a table row into stripped cells; `\|` and pipes in code spans do not split."""
    text = row.strip()
    masked = text.replace("\\\\", "ee").replace("\\|", "ee")
    masked = _CODE_SPAN.sub(lambda m: "x" * len(m.group(0)), masked)
    cuts = [i for i, char in enumerate(masked) if char == "|"]
    if cuts and cuts[0] == 0:
        cuts = cuts[1:]
        start = 1
    else:
        start = 0
    end = len(text)
    if cuts and cuts[-1] == len(text) - 1:
        end = cuts[-1]
        cuts = cuts[:-1]
    cells = []
    for cut in [*cuts, end]:
        cells.append(text[start:cut].strip())
        start = cut + 1
    return cells


def _tables(path: Path) -> Iterator[tuple[int, list[str], list[tuple[int, list[str]]]]]:
    """Yield (header line, header cells, [(line, cells)]) of every table in `path`."""
    rows = list(_lines(path))
    index = 0
    while index < len(rows):
        number, line = rows[index]
        following = rows[index + 1] if index + 1 < len(rows) else (0, "")
        if (
            line.strip().startswith("|")
            and following[0] == number + 1
            and _DELIMITER_ROW.match(following[1].strip())
        ):
            body: list[tuple[int, list[str]]] = []
            index += 2
            while (
                index < len(rows)
                and rows[index][0] == rows[index - 1][0] + 1
                and rows[index][1].strip().startswith("|")
            ):
                body.append((rows[index][0], _cells(rows[index][1])))
                index += 1
            yield number, _cells(line), body
            continue
        index += 1


def _rated(header: list[str], body: list[tuple[int, list[str]]]) -> list[tuple[str, Rated]]:
    """Return (first cell, row) for each row of a table with ID, likelihood and impact."""
    ident, likelihood, impact = (header.index(n) for n in ("ID", LIKELIHOOD, IMPACT))
    width = max(ident, likelihood, impact)
    return [
        (cells[0], Rated(number, cells[ident], cells[likelihood], cells[impact]))
        for number, cells in body
        if len(cells) > width
    ]


def _behind(path: Path) -> list[Behind]:
    """Return every list that starts with `**Behind rank <n>**`, up to the next heading."""
    lists: list[Behind] = []
    current: tuple[int, str, list[tuple[int, str]]] | None = None
    for number, line in _lines(path):
        start = _BEHIND.match(line)
        if start or _HEADING.match(line):
            if current is not None:
                lists.append(Behind(current[0], current[1], tuple(current[2])))
            current = (number, start.group(1), []) if start else None
            continue
        item = _LIST_ITEM.match(line)
        if current is not None and item:
            current[2].extend((number, ident) for ident in _RISK_ID.findall(item.group(1)))
    if current is not None:
        lists.append(Behind(current[0], current[1], tuple(current[2])))
    return lists


def _describe(row: Rated) -> str:
    """Return `<score> (<rating>)` for a message."""
    return f"{row.score()} ({row.rating()})"


def check(root: Path) -> list[str]:
    """Return one message per finding in the ranking of docs/RISKS.md under `root`."""
    problems: list[str] = []
    ranking: list[tuple[str, Rated]] = []
    ranking_line = 0
    register: dict[str, Rated] = {}
    for line, header, body in _tables(root / RISKS):
        if header[:2] == ["Rank", "ID"] and LIKELIHOOD in header and IMPACT in header:
            if ranking_line:
                problems.append(
                    f"{RISKS}:{line}: a second ranking table, the first starts in line "
                    f"{ranking_line}"
                )
                continue
            ranking_line = line
            ranking = _rated(header, body)
        elif header[:1] == ["ID"] and LIKELIHOOD in header and IMPACT in header:
            for _, row in _rated(header, body):
                register[row.ident] = row
                for kind, word in (("likelihood", row.likelihood), ("impact", row.impact)):
                    if word not in LEVELS:
                        problems.append(
                            f"{RISKS}:{row.line}: {row.ident} has the {kind} {word!r}, "
                            f"allowed are {', '.join(LEVELS)}"
                        )
    if not ranking_line:
        return [f"{RISKS}: no ranking table (header 'Rank | ID | ...')", *problems]

    seen: dict[str, str] = {}
    previous: tuple[str, Rated] | None = None
    for rank, row in ranking:
        where = f"{RISKS}:{row.line}: rank {rank}"
        expected = str(int(previous[0]) + 1 if previous and previous[0].isdigit() else 1)
        if rank != expected:
            after = f"follows rank {previous[0]}" if previous else "comes first"
            problems.append(f"{where} {after}, expected rank {expected}")
        if row.ident in seen:
            problems.append(f"{where} {row.ident} is already rank {seen[row.ident]}")
            continue
        seen[row.ident] = rank
        entry = register.get(row.ident)
        if entry is None:
            problems.append(f"{where} {row.ident} is not in the register")
        elif (row.likelihood, row.impact) != (entry.likelihood, entry.impact):
            problems.append(
                f"{where} {row.ident} is rated {row.rating()}, the register in line "
                f"{entry.line} says {entry.rating()}"
            )
        if previous is not None:
            before, now = previous[1].score(), row.score()
            if before is not None and now is not None and now > before:
                problems.append(
                    f"{where} {row.ident} is rated {_describe(row)}, higher than rank "
                    f"{previous[0]} {previous[1].ident} with {_describe(previous[1])}"
                )
        previous = (rank, row)

    if previous is not None and previous[1].score() is not None:
        last_rank, last = previous
        lowest = last.score() or 0
        for row in register.values():
            score = row.score()
            if row.ident not in seen and score is not None and score > lowest:
                problems.append(
                    f"{RISKS}:{row.line}: {row.ident} is rated {_describe(row)} but not "
                    f"ranked, rank {last_rank} {last.ident} is rated {_describe(last)}"
                )
        problems.extend(_check_behind(root, register, seen, last_rank, last))
    return problems


def _check_behind(
    root: Path, register: dict[str, Rated], seen: dict[str, str], last_rank: str, last: Rated
) -> list[str]:
    """Check that the list behind the last rank names exactly the unranked risks rated like it."""
    problems: list[str] = []
    lists = _behind(root / RISKS)
    for extra in lists[1:]:
        problems.append(
            f"{RISKS}:{extra.line}: a second list '**Behind rank {extra.rank}**', the first "
            f"starts in line {lists[0].line}"
        )
    named: dict[str, int] = {}
    if lists:
        behind = lists[0]
        if behind.rank != last_rank:
            problems.append(
                f"{RISKS}:{behind.line}: the list '**Behind rank {behind.rank}**' names rank "
                f"{behind.rank}, the last rank is {last_rank}"
            )
        for line, ident in behind.named:
            where = f"{RISKS}:{line}: {ident} is named behind rank {last_rank}"
            entry = register.get(ident)
            if ident in named:
                problems.append(
                    f"{RISKS}:{line}: {ident} is named twice behind rank {last_rank}, first in "
                    f"line {named[ident]}"
                )
                continue
            named[ident] = line
            if entry is None:
                problems.append(f"{where} but not in the register")
            elif ident in seen:
                problems.append(f"{where} but is rank {seen[ident]}")
            elif entry.score() != last.score():
                problems.append(
                    f"{where} but rated {_describe(entry)}, rank {last_rank} {last.ident} is "
                    f"rated {_describe(last)}"
                )
    for row in register.values():
        if row.ident not in seen and row.ident not in named and row.score() == last.score():
            problems.append(
                f"{RISKS}:{row.line}: {row.ident} is rated {_describe(row)} like rank "
                f"{last_rank} {last.ident} but not named in the list behind rank {last_rank}"
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
