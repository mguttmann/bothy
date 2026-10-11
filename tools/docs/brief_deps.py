# SPDX-License-Identifier: Apache-2.0
"""Keep the build-brief index, its waves, its graph and the plan consistent.

docs/briefs/README.md lists every brief with its dependencies ("Index"), the waves derived from
them ("Waves") and the dependency graph ("Dependency graph"). docs/PLAN.md repeats the
dependencies in the column "Depends on" of each ticket row. A launcher starts a brief as soon
as its own dependencies are merged, so a brief must not call a `make` target that only a ticket
outside its dependency closure creates (the target would be missing when it runs).

This check reads all of them and fails on:

- a dependency that names an unknown ticket, a cycle, or a dependency implied by another one
  (the index lists the transitive reduction);
- a wave that is not 1 + the largest wave of the dependencies (1 without dependencies), a
  "Waves" table that does not group the index by wave, a graph whose edges are not the index
  dependencies;
- a plan row whose "Depends on" or a brief whose header line "Depends on" differs from the
  index;
- a `make <target>` inside code of a brief whose target is neither in the Makefile nor in the
  table "Make targets created by tickets", or whose creating ticket is not the brief itself or
  in its dependency closure.

`--write` regenerates the wave column, the "Waves" table and the graph edges from the index
dependencies (the index stays the source). Fix dependencies by hand, then run `--write`.

Usage: python tools/docs/brief_deps.py [--write] [--root DIR]  (exit 1 on a finding)
"""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Iterable
from pathlib import Path

README = Path("docs/briefs/README.md")
PLAN = Path("docs/PLAN.md")
MAKEFILE = Path("Makefile")

_ID = re.compile(r"M\d-\d\d")
_FENCE = re.compile(r"^\s*(`{3,}|~{3,})")
_INLINE_CODE = re.compile(r"(`+)((?:(?!\1)(?:[^\n]|\n(?![ \t]*\n)))+?)\1")
_MAKE = re.compile(r"(?<![\w./-])make ([a-z][a-z0-9-]*)")
_MAKE_TARGET = re.compile(r"^([a-zA-Z0-9][a-zA-Z0-9_-]*)\s*:(?!=)", re.MULTILINE)
_EDGE = re.compile(r"^\s*(M\d_\d\d) --> (M\d_\d\d)\s*$")


class Index:
    """Parsed index: ticket id to (wave, dependencies, brief file), in document order."""

    def __init__(self) -> None:
        """Create an empty index."""
        self.wave: dict[str, int] = {}
        self.deps: dict[str, list[str]] = {}
        self.file: dict[str, str] = {}


def _section(text: str, heading: str) -> tuple[int, int]:
    """Return start and end offsets of the body of the `## heading` section."""
    m = re.search(rf"^## {re.escape(heading)}\s*$", text, re.MULTILINE)
    if m is None:
        raise ValueError(f"section '## {heading}' missing")
    nxt = re.search(r"^## ", text[m.end() :], re.MULTILINE)
    end = m.end() + nxt.start() if nxt else len(text)
    return m.end(), end


def _rows(body: str) -> list[list[str]]:
    rows = []
    for line in body.splitlines():
        if line.startswith("|") and not line.startswith("|---"):
            rows.append([c.strip() for c in line.strip().strip("|").split("|")])
    return rows[1:] if rows else rows


def parse_index(text: str) -> Index:
    """Read the table of section "Index" (Wave, ID, Title, Brief file, Depends on)."""
    start, end = _section(text, "Index")
    index = Index()
    for row in _rows(text[start:end]):
        tid = row[1]
        index.wave[tid] = int(row[0])
        index.deps[tid] = _ID.findall(row[4])
        index.file[tid] = row[3].strip("`")
    return index


def parse_targets(text: str) -> dict[str, str]:
    """Read the table "Make targets created by tickets": target to creating ticket."""
    start, end = _section(text, "Make targets created by tickets")
    return {row[0].strip("`"): row[1] for row in _rows(text[start:end])}


def makefile_targets(text: str) -> set[str]:
    """Targets defined in the Makefile today (rules, not variable assignments)."""
    return set(_MAKE_TARGET.findall(text))


def code_text(text: str) -> str:
    """Return only fenced blocks and code spans, so prose like "make it green" is ignored."""
    parts: list[str] = []
    prose: list[str] = []
    fence: str | None = None
    for line in text.splitlines():
        m = _FENCE.match(line)
        if fence is not None:
            if m and m.group(1)[0] == fence[0] and len(m.group(1)) >= len(fence):
                fence = None
            else:
                parts.append(line)
            continue
        if m:
            fence = m.group(1)
            continue
        prose.append(line)
    parts += [m.group(2) for m in _INLINE_CODE.finditer("\n".join(prose))]
    return "\n".join(parts)


def make_calls(text: str) -> set[str]:
    """Targets named as `make <target>` inside code of a brief."""
    return set(_MAKE.findall(code_text(text)))


def closure(deps: dict[str, list[str]], tid: str) -> set[str]:
    """All tickets `tid` depends on, directly or transitively."""
    seen: set[str] = set()
    stack = list(deps.get(tid, []))
    while stack:
        d = stack.pop()
        if d not in seen:
            seen.add(d)
            stack.extend(deps.get(d, []))
    return seen


def waves(deps: dict[str, list[str]]) -> dict[str, int]:
    """Wave of each ticket: 1 + largest wave of its dependencies; raises on a cycle."""
    result: dict[str, int] = {}
    active: set[str] = set()

    def visit(tid: str) -> int:
        if tid in result:
            return result[tid]
        if tid in active:
            raise ValueError(f"dependency cycle through {tid}")
        active.add(tid)
        w = 1 + max((visit(d) for d in deps[tid]), default=0)
        active.discard(tid)
        result[tid] = w
        return w

    for tid in deps:
        visit(tid)
    return result


def _edges(deps: dict[str, list[str]]) -> list[tuple[str, str]]:
    return [(d, t) for t in sorted(deps) for d in sorted(deps[t])]


def _plan_deps(text: str, ids: Iterable[str]) -> dict[str, list[str]]:
    wanted = set(ids)
    out: dict[str, list[str]] = {}
    for line in text.splitlines():
        m = re.match(r"^\| (M\d-\d\d) \|", line)
        if m and m.group(1) in wanted:
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            out[m.group(1)] = _ID.findall(cells[-1])
    return out


def check(root: Path) -> list[str]:
    """Return one message per inconsistency between briefs, index, waves, graph and plan."""
    readme = (root / README).read_text(encoding="utf-8")
    index = parse_index(readme)
    problems: list[str] = []
    for tid, ds in index.deps.items():
        problems += [f"{README}: {tid}: unknown dependency {d}" for d in ds if d not in index.deps]
    if problems:
        return problems
    try:
        computed = waves(index.deps)
    except ValueError as exc:
        return [f"{README}: {exc}"]
    for tid, ds in index.deps.items():
        for d in ds:
            if any(d in closure(index.deps, o) for o in ds if o != d):
                problems.append(f"{README}: {tid}: dependency {d} is implied by another one")
        if computed[tid] != index.wave[tid]:
            problems.append(f"{README}: {tid}: wave {index.wave[tid]}, expected {computed[tid]}")
    if render_waves(computed) not in readme:
        problems.append(f"{README}: table 'Waves' differs from the index (run --write)")
    start, end = _section(readme, "Dependency graph")
    shown = sorted(
        (a.replace("_", "-"), b.replace("_", "-"))
        for line in readme[start:end].splitlines()
        if (m := _EDGE.match(line))
        for a, b in [m.groups()]
    )
    if shown != sorted(_edges(index.deps)):
        problems.append(f"{README}: graph edges differ from the index (run --write)")
    plan = _plan_deps((root / PLAN).read_text(encoding="utf-8"), index.deps)
    for tid, ds in index.deps.items():
        if tid not in plan:
            problems.append(f"{PLAN}: no ticket row for {tid}")
        elif sorted(plan[tid]) != sorted(ds):
            problems.append(f"{PLAN}: {tid}: depends on {plan[tid]}, index says {ds}")
    targets = parse_targets(readme)
    existing = makefile_targets((root / MAKEFILE).read_text(encoding="utf-8"))
    for target, creator in targets.items():
        if creator not in index.deps:
            problems.append(f"{README}: target {target}: unknown ticket {creator}")
    for tid, name in index.file.items():
        path = root / README.parent / name
        if not path.is_file():
            problems.append(f"{README}: {tid}: brief file {name} missing")
            continue
        brief = path.read_text(encoding="utf-8")
        header = re.search(r"^Status: .*?\| Depends on: ([^|]*)\|", brief, re.MULTILINE)
        if header is None or sorted(_ID.findall(header.group(1))) != sorted(index.deps[tid]):
            problems.append(f"{name}: header 'Depends on' differs from the index")
        reach = closure(index.deps, tid) | {tid}
        for target in sorted(make_calls(brief)):
            maker = targets.get(target)
            if maker is None:
                if target not in existing:
                    problems.append(f"{name}: make {target}: no Makefile target and no creator")
            elif maker not in reach:
                problems.append(
                    f"{name}: make {target}: created by {maker}, not a dependency of {tid}"
                )
    return problems


def render_waves(computed: dict[str, int]) -> str:
    """The "Waves" table for the computed waves."""
    lines = ["| Wave | Tickets that may run in parallel |", "|---|---|"]
    for w in sorted(set(computed.values())):
        ids = sorted(t for t, v in computed.items() if v == w)
        lines.append(f"| {w} | {', '.join(ids)} |")
    return "\n".join(lines) + "\n"


def write(root: Path) -> None:
    """Regenerate wave column, "Waves" table and graph edges from the index dependencies."""
    path = root / README
    text = path.read_text(encoding="utf-8")
    index = parse_index(text)
    computed = waves(index.deps)
    start, end = _section(text, "Waves")
    body = text[start:end]
    table = re.search(r"^\| Wave \| Tickets.*?(?=\n[^|]|\Z)", body, re.MULTILINE | re.DOTALL)
    if table is None:
        raise ValueError("table 'Waves' missing")
    body = body[: table.start()] + render_waves(computed).rstrip("\n") + body[table.end() :]
    text = text[:start] + body + text[end:]
    start, end = _section(text, "Index")
    body = re.sub(
        r"^\| \d+ \| (M\d-\d\d) \|",
        lambda m: f"| {computed[m.group(1)]} | {m.group(1)} |",
        text[start:end],
        flags=re.MULTILINE,
    )
    text = text[:start] + body + text[end:]
    start, end = _section(text, "Dependency graph")
    body = text[start:end]
    lines = [ln for ln in body.splitlines() if not _EDGE.match(ln)]
    close = max(i for i, ln in enumerate(lines) if ln.strip() == "```")
    edges = [f"  {a.replace('-', '_')} --> {b.replace('-', '_')}" for a, b in _edges(index.deps)]
    lines[close:close] = edges
    text = text[:start] + "\n".join(lines) + ("\n" if body.endswith("\n") else "") + text[end:]
    path.write_text(text, encoding="utf-8")


def main(argv: list[str] | None = None) -> int:
    """Check the brief index and plan, or regenerate waves and graph with `--write`."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--write", action="store_true", help="regenerate waves and graph")
    parser.add_argument("--root", type=Path, default=Path())
    args = parser.parse_args(argv)
    if args.write:
        write(args.root)
    problems = check(args.root)
    for problem in problems:
        print(f"brief-deps: {problem}", file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
