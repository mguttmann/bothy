# SPDX-License-Identifier: Apache-2.0
"""ERD gate: the ERDs in docs/architecture match the data model in docs/spec/03-data-model.md.

Rules (docs/architecture/README.md, section "Conventions"):

- Every table of section 4 of 03 has exactly one full entity (a foreign key or an attribute
  that is no primary key) in the `erd-*.mmd` files. Other entities only show their key.
- Every entity of an ERD is a table of 03.
- Every foreign key of 03 is drawn as a relationship between child and parent table, as
  often as there are foreign keys between the two. Exempt are protocol columns: a column
  whose name ends in `_by` and points to `principals`.
"""

from __future__ import annotations

import re
import sys
from collections import Counter
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
DATA_MODEL = Path("docs") / "spec" / "03-data-model.md"
ERD_GLOB = "docs/architecture/erd-*.mmd"

_TABLE_HEADING = re.compile(r"^#### `([a-z_][a-z0-9_]*)`\s*$")
_FK_END = re.compile(
    r"(?:^|\s)(?:PK|Unique|Indexes|Checks|Retention|Permissions|Partitioning|Trigger|Note):",
)
_PARENT = re.compile(r"->\s*`([a-z_][a-z0-9_]*)\([^)`]*\)`")
_COLUMN = re.compile(r"`([a-z_][a-z0-9_]*)`")
_ENTITY_OPEN = re.compile(r"^\s*([A-Za-z_]\w*)\s*\{\s*$")
_RELATION = re.compile(r"^\s*([A-Za-z_]\w*)\s+[|}o{]{2}(?:--|\.\.)[|}o{]{2}\s+([A-Za-z_]\w*)\s*:")


def model_tables(text: str) -> dict[str, str]:
    """Return table name -> its section text, for every table heading of section 4 of 03."""
    start = text.index("\n## 4.")
    end = text.index("\n## 5.", start)
    tables: dict[str, str] = {}
    current: str | None = None
    lines: list[str] = []
    for line in text[start:end].splitlines():
        m = _TABLE_HEADING.match(line)
        if m or line.startswith("### "):
            if current is not None:
                tables[current] = "\n".join(lines)
            current, lines = (m.group(1) if m else None), []
            continue
        lines.append(line)
    if current is not None:
        tables[current] = "\n".join(lines)
    return tables


def _fk_text(section: str) -> str:
    """Return the text after `FK:` up to the next clause (PK, Unique, Indexes, ...)."""
    i = section.find("FK:")
    if i < 0:
        return ""
    rest = " ".join(section[i + 3 :].split())
    m = _FK_END.search(rest)
    return rest[: m.start()] if m else rest


def foreign_keys(table: str, section: str) -> list[tuple[str, str, str]]:
    """Return (child table, column, parent table) for each foreign key of one table section."""
    parts = _PARENT.split(_fk_text(section))
    keys = []
    for i in range(0, len(parts) - 1, 2):
        before = parts[i].rsplit(";", 1)[-1]
        keys += [(table, column, parts[i + 1]) for column in _COLUMN.findall(before)]
    return keys


def is_protocol_column(column: str, parent: str) -> bool:
    """Return True for audit-only columns (`created_by` and similar on principals)."""
    return parent == "principals" and column.endswith("_by")


def parse_erd(text: str) -> tuple[dict[str, list[str]], list[tuple[str, str]]]:
    """Return (entity -> attribute lines, relationships as (left, right)) of one erDiagram."""
    entities: dict[str, list[str]] = {}
    relations: list[tuple[str, str]] = []
    current: str | None = None
    for raw in text.splitlines():
        line = raw.split("%%", 1)[0].rstrip()
        if current is not None:
            if line.strip() == "}":
                current = None
            elif line.strip():
                entities[current].append(line.strip())
            continue
        m = _ENTITY_OPEN.match(line)
        if m:
            current = m.group(1)
            entities.setdefault(current, [])
            continue
        r = _RELATION.match(line)
        if r:
            relations.append((r.group(1), r.group(2)))
    return entities, relations


def _is_full(attributes: list[str]) -> bool:
    """An entity is the full one when it lists a foreign key or an attribute that is no primary key.

    A stub of a table from another module shows only its primary key, never a foreign key.
    """
    keys = [a.split('"', 1)[0] for a in attributes]
    return any(re.search(r"\bFK\b", k) or not re.search(r"\bPK\b", k) for k in keys)


def check(model_text: str, erds: dict[str, str]) -> list[str]:
    """Return every mismatch between the data model text and the ERD sources (name -> text)."""
    tables = model_tables(model_text)
    full: dict[str, list[str]] = {}
    known: set[str] = set()
    drawn: Counter[frozenset[str]] = Counter()
    problems: list[str] = []
    for name, text in sorted(erds.items()):
        entities, relations = parse_erd(text)
        for entity, attributes in entities.items():
            known.add(entity)
            if entity not in tables:
                problems.append(f"{name}: entity {entity} is no table of {DATA_MODEL}")
            if _is_full(attributes):
                full.setdefault(entity, []).append(name)
        for left, right in relations:
            drawn[frozenset((left, right))] += 1
    for table in sorted(tables):
        where = full.get(table, [])
        if not where:
            problems.append(f"{DATA_MODEL}: table {table} has no full entity in any ERD")
        elif len(where) > 1:
            problems.append(f"{DATA_MODEL}: table {table} has a full entity in {', '.join(where)}")
    needed: Counter[frozenset[str]] = Counter()
    labels: dict[frozenset[str], list[str]] = {}
    for table, section in tables.items():
        for child, column, parent in foreign_keys(table, section):
            if parent not in tables:
                problems.append(f"{DATA_MODEL}: {child}.{column} points to unknown table {parent}")
                continue
            if is_protocol_column(column, parent):
                continue
            pair = frozenset((child, parent))
            needed[pair] += 1
            labels.setdefault(pair, []).append(f"{child}.{column} -> {parent}")
    for pair, count in sorted(needed.items(), key=lambda item: sorted(item[0])):
        if drawn[pair] < count:
            missing = count - drawn[pair]
            problems.append(
                f"ERD: {missing} of {count} relationship(s) missing for {'; '.join(labels[pair])}",
            )
    return problems


def problems(root: Path) -> list[str]:
    """Run the check on the repository at `root`."""
    model_text = (root / DATA_MODEL).read_text(encoding="utf-8")
    erds = {
        p.relative_to(root).as_posix(): p.read_text(encoding="utf-8")
        for p in sorted(root.glob(ERD_GLOB))
    }
    if not erds:
        return [f"ERD: no file matches {ERD_GLOB}"]
    return check(model_text, erds)


def main() -> int:
    """Print the mismatches; returns 1 if there are any."""
    found = problems(REPO_ROOT)
    for line in found:
        print(line)
    if found:
        return 1
    print("erd: every table and foreign key of the data model is drawn")
    return 0


if __name__ == "__main__":
    sys.exit(main())
