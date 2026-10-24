# SPDX-License-Identifier: Apache-2.0
"""Docs gate of the terminal test catalogue (brief M2-48 Design note 12).

`services/server/test/terminal/catalogue.yaml` names, for every test ID of the terminal, where
it runs and which ticket implemented it. This gate keeps it together with the documents it
mirrors, so that a test that a document promises cannot silently disappear:

1. The IDs of the table in docs/spec/24-terminal.md section 14 equal the TM-T entries. The cells
   `title` and `expected` are compared only when the chapter is English (header
   `| ID | Test | Expected |`); a German chapter is compared by ID only, because the catalogue
   holds the English cells.
2. Every TM-T, AZ-M and ID-T ID of the PLAN row M2-48 and of exit criterion 8 in PLAN section 8
   has a run that `make check` executes (kind `e2e` or `unit`, or `platform` on `linux`) and is
   not deferred; TM-T16 also runs in `make perf`; the IDs of the `doctor` matrix run on every
   platform; the console parts of TM-T07, TM-T12 and ID-T28 are deferred to M6-18.
3. Every row of docs/security/threat-model.md section 6.7 names a test; every test ID it names
   has an entry that lists the threat; every Go test it names exists under services/; every
   `PLAN Mx-yy` it cites is a ticket row of docs/PLAN.md.
4. Every `ref` of a run exists, and every ticket and test the catalogue names exists.

The structure of the catalogue is validated strictly first: unknown fields, duplicate keys or
IDs, unknown kinds or operating systems and entries out of order are rejected.

Usage:
  terminal_catalogue.py --check [--repo <dir>]   print one line per problem, exit 1 on any
"""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Iterable
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, TypeGuard

import yaml

DEFAULT_REPO = Path(__file__).resolve().parents[2]
CATALOGUE = Path("services/server/test/terminal/catalogue.yaml")
CHAPTER = Path("docs/spec/24-terminal.md")
PLAN = Path("docs/PLAN.md")
THREAT_MODEL = Path("docs/security/threat-model.md")
TERMINAL_DIR = Path("services/server/test/terminal")
PLATFORM_DIR = Path("services/host/test/platform")
DART_DIR = Path("packages/bothy_noise/test")

KINDS = ("e2e", "platform", "perf", "unit")
OS_NAMES = ("linux", "macos", "windows10", "windows11")
E2E_PACKAGES = ("conformance", "hostpolicy", "limits", "twonode", "restart", "hostplatform")
# The order of the ID families in the catalogue: chapter 24 first, then the IDs of the other
# chapters that threat model 6.7 names.
ID_FAMILIES = ("TM-T", "AZ-M", "ID-T", "SEC-T")
# The IDs whose `doctor` checks exit criterion 8 demands on every platform: TM-T13 (HD1 to
# HD3), TM-T24 (HD9) and TM-T29 (HD11 at a location for all and part (i)).
DOCTOR_MATRIX = ("TM-T13", "TM-T24", "TM-T29")
DOCTOR_PLATFORMS = ("macos", "windows10", "windows11")
# The IDs whose console part the web console ticket proves (24 section 14, 08 ID-T28).
CONSOLE_PARTS = ("TM-T07", "TM-T12", "ID-T28")
CONSOLE_TICKET = "M6-18"
PERF_IDS = ("TM-T16",)

TOP_FIELDS = {"format", "entries"}
ENTRY_FIELDS = {
    "id",
    "title",
    "expected",
    "runs",
    "implemented_by",
    "deferred",
    "deferred_parts",
    "threats",
}
ENTRY_REQUIRED = ("id", "title", "expected", "runs", "implemented_by", "threats")
RUN_FIELDS = {"kind", "ref", "os"}
IMPLEMENTED_FIELDS = ("ticket", "tests")
DEFERRED_FIELDS = ("ticket", "reason")
DEFERRED_PART_FIELDS = ("part", "ticket")

_ID = re.compile(r"^(TM-T|AZ-M|ID-T|SEC-T)(\d{2})$")
_THREAT = re.compile(r"^T-TM-\d{2}$")
_TICKET = re.compile(r"^[A-Z]\d-\d{2}$")
_TEST_NAME = r"Test[A-Za-z0-9_]*"
_E2E_REF = re.compile(
    rf"^e2e/(?P<pkg>{'|'.join(E2E_PACKAGES)}):"
    rf"(?:scenarios/(?P<file>[a-z0-9-]+\.yaml)|(?P<func>{_TEST_NAME}))$"
)
_PLATFORM_REF = re.compile(rf"^{_TEST_NAME}$")
_PERF_REF = re.compile(rf"^services/server/test/chaos:(?P<func>{_TEST_NAME})$")
_GO_UNIT_REF = re.compile(rf"^(?P<dir>services/[a-z0-9_./-]+):(?P<func>{_TEST_NAME})$")
_DART_UNIT_REF = re.compile(r"^(?P<file>packages/bothy_noise/test/[a-z0-9_]+\.dart):(?P<name>.+)$")

_FAMILY = "TM-T|AZ-M|ID-T|SEC-T"
_RANGE = re.compile(rf"\b({_FAMILY})(\d{{2}})\s+(?:to|bis)\s+(?:(?:{_FAMILY}))?(\d{{2}})\b")
_SINGLE = re.compile(rf"\b({_FAMILY})(\d{{2}})\b")
_GO_TEST_IN_TEXT = re.compile(r"\bTest[A-Z0-9_][A-Za-z0-9_]*")
_PLAN_REF = re.compile(r"\bPLAN ([A-Z]\d-\d{2})\b")
_PLAN_TICKET_ROW = re.compile(r"^\| ([A-Z]\d-\d{2}) \|", re.MULTILINE)
_GO_FUNC = re.compile(r"^func (Test[A-Za-z0-9_]*)\(", re.MULTILINE)
_DART_TEST = re.compile(r"\b(?:test|group)\(\s*(['\"])(?P<name>.*?)(?<!\\)\1", re.DOTALL)
_DART_INTERPOLATION = re.compile(r"\$\{[^}]*\}|\$[A-Za-z_][A-Za-z0-9_]*")
_CELL_SPLIT = re.compile(r"(?<!\\)\|")


class CatalogueError(Exception):
    """The catalogue file cannot be read or decoded as YAML."""


class _UniqueKeyLoader(yaml.SafeLoader):
    """SafeLoader that rejects a mapping with a duplicate key instead of keeping the last."""


def _construct_mapping(loader: yaml.SafeLoader, node: yaml.MappingNode) -> dict[Any, Any]:
    seen: set[Any] = set()
    for key_node, _ in node.value:
        key = loader.construct_object(key_node)
        if key in seen:
            raise yaml.constructor.ConstructorError(
                None, None, f"duplicate key {key}", key_node.start_mark
            )
        seen.add(key)
    return loader.construct_mapping(node)


_UniqueKeyLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _construct_mapping)


@dataclass
class Sources:
    """The repository files the gate reads, indexed once per run.

    Attributes:
        repo: root of the repository.
        go_tests: Go test function name -> repo-relative directories that define it.
        plan_tickets: ticket IDs that have a row in docs/PLAN.md.
    """

    repo: Path
    go_tests: dict[str, set[str]] = field(default_factory=dict)
    plan_tickets: set[str] = field(default_factory=set)
    _dart: dict[str, list[re.Pattern[str]]] = field(default_factory=dict)

    @classmethod
    def load(cls, repo: Path) -> Sources:
        """Index the Go tests under services/ and the ticket rows of the plan.

        Args:
            repo: root of the repository.

        Returns:
            The indexed sources; a missing plan gives an empty ticket set.
        """
        sources = cls(repo)
        for path in sorted((repo / "services").rglob("*.go")):
            directory = path.parent.relative_to(repo).as_posix()
            for name in _GO_FUNC.findall(path.read_text(encoding="utf-8")):
                sources.go_tests.setdefault(name, set()).add(directory)
        plan = _read(repo / PLAN)
        sources.plan_tickets = set(_PLAN_TICKET_ROW.findall(plan or ""))
        return sources

    def go_test_in(self, name: str, directory: str) -> bool:
        """Report whether `func <name>(` exists in a Go file directly in `directory`."""
        return directory in self.go_tests.get(name, set())

    def dart_test_in(self, name: str, file: str) -> bool:
        """Report whether a test() or group() of the Dart file has the given name.

        A name with string interpolation (`'TM-T01 $name'`) matches any text in place of each
        interpolation, because the catalogue names the test as the runner reports it.
        """
        if file not in self._dart:
            text = _read(self.repo / file) or ""
            patterns = []
            for m in _DART_TEST.finditer(text):
                parts = _DART_INTERPOLATION.split(m.group("name"))
                patterns.append(re.compile(".+".join(re.escape(p) for p in parts)))
            self._dart[file] = patterns
        return any(p.fullmatch(name) for p in self._dart[file])

    def dart_test_anywhere(self, name: str) -> bool:
        """Report whether any Dart test file of packages/bothy_noise/test has the given name."""
        files = sorted((self.repo / DART_DIR).glob("*.dart"))
        return any(self.dart_test_in(name, f.relative_to(self.repo).as_posix()) for f in files)


def _read(path: Path) -> str | None:
    try:
        return path.read_text(encoding="utf-8")
    except OSError:
        return None


def id_key(id_: str) -> tuple[int, int]:
    """Sort key of a catalogue ID: the family in ID_FAMILIES order, then the number.

    Args:
        id_: an ID such as `TM-T07`.

    Returns:
        A tuple that orders TM-T before AZ-M, ID-T and SEC-T; an unknown form sorts last.
    """
    m = _ID.match(id_)
    if not m:
        return (len(ID_FAMILIES), 0)
    return (ID_FAMILIES.index(m.group(1)), int(m.group(2)))


def ids_in(text: str, families: Iterable[str] = ID_FAMILIES) -> set[str]:
    """Collect the test IDs a text names, with ranges "X to Y" and "X bis Y" expanded.

    Args:
        text: free text such as a plan row or a cell of the threat model.
        families: the ID families to keep.

    Returns:
        The IDs of the wanted families.
    """
    wanted = tuple(families)
    found: set[str] = set()
    for family, first, last in _RANGE.findall(text):
        found.update(f"{family}{n:02d}" for n in range(int(first), int(last) + 1))
    for family, number in _SINGLE.findall(text):
        found.add(f"{family}{number}")
    return {i for i in found if i.startswith(wanted)}


def _cells(line: str) -> list[str]:
    return [c.strip() for c in _CELL_SPLIT.split(line.strip().strip("|"))]


def _section(text: str, heading: re.Pattern[str]) -> list[str]:
    """Return the lines after the matching heading up to the next heading of its level."""
    lines = text.splitlines()
    for i, line in enumerate(lines):
        m = heading.match(line)
        if m:
            level = line.split(" ", 1)[0]
            out = []
            for rest in lines[i + 1 :]:
                if rest.startswith("#") and len(rest.split(" ", 1)[0]) <= len(level):
                    break
                out.append(rest)
            return out
    return []


def chapter_rows(text: str) -> tuple[bool, dict[str, tuple[str, str]]] | None:
    """Parse the test table of chapter 24 section 14.

    Args:
        text: the chapter.

    Returns:
        (english, rows) with rows mapping each TM-T ID to its (test, expected) cells, where
        english is true for the header `| ID | Test | Expected |`; None without a table.
    """
    lines = _section(text, re.compile(r"^## 14\. "))
    header = next((line for line in lines if line.startswith("| ID |")), None)
    if header is None:
        return None
    english = _cells(header) == ["ID", "Test", "Expected"]
    rows: dict[str, tuple[str, str]] = {}
    for line in lines:
        if line.startswith("| TM-T"):
            cells = _cells(line)
            rows[cells[0]] = (cells[1], cells[2]) if len(cells) >= 3 else ("", "")
    return english, rows


def plan_ids(text: str) -> tuple[set[str] | None, set[str] | None]:
    """Parse the TM-T, AZ-M and ID-T IDs of the PLAN row M2-48 and of exit criterion 8.

    Args:
        text: docs/PLAN.md, English or German.

    Returns:
        (row IDs, exit criterion IDs); an element is None when the row or the criterion is
        missing. The criterion is the item `8. Terminal (M2-48):` of section 8 with its
        indented continuation lines.
    """
    families = ("TM-T", "AZ-M", "ID-T")
    row = next((line for line in text.splitlines() if line.startswith("| M2-48 |")), None)
    lines = _section(text, re.compile(r"^## 8\. "))
    criterion: list[str] = []
    for i, line in enumerate(lines):
        if line.startswith("8. Terminal (M2-48):"):
            criterion.append(line)
            for rest in lines[i + 1 :]:
                if not rest.startswith(" ") or not rest.strip():
                    break
                criterion.append(rest)
            break
    return (
        ids_in(row, families) if row is not None else None,
        ids_in(" ".join(criterion), families) if criterion else None,
    )


def threat_rows(text: str) -> dict[str, str] | None:
    """Parse the Tests column of the table in threat model section 6.7.

    Args:
        text: docs/security/threat-model.md.

    Returns:
        T-TM ID -> the text of its Tests cell; None without a table with a Tests column.
    """
    lines = _section(text, re.compile(r"^### 6\.7 "))
    header = next((line for line in lines if line.startswith("| ID |")), None)
    if header is None or "Tests" not in _cells(header):
        return None
    column = _cells(header).index("Tests")
    rows: dict[str, str] = {}
    for line in lines:
        if line.startswith("| T-TM-"):
            cells = _cells(line)
            rows[cells[0]] = cells[column] if len(cells) > column else ""
    return rows


def _missing(data: dict[str, Any], required: Iterable[str], allowed: set[str]) -> list[str]:
    problems = [f"missing field {k}" for k in required if k not in data]
    problems += [f"unknown field {k}" for k in sorted(data) if k not in allowed]
    return problems


def _text(value: Any) -> TypeGuard[str]:
    return isinstance(value, str) and value != ""


def _ticket(value: Any) -> bool:
    return isinstance(value, str) and _TICKET.match(value) is not None


def _check_ref(kind: str, ref: str) -> bool:
    if kind == "e2e":
        return _E2E_REF.match(ref) is not None
    if kind == "platform":
        return _PLATFORM_REF.match(ref) is not None
    if kind == "perf":
        return _PERF_REF.match(ref) is not None
    return _GO_UNIT_REF.match(ref) is not None or _DART_UNIT_REF.match(ref) is not None


def _structure_run(id_: str, n: int, run: Any) -> list[str]:
    if not isinstance(run, dict):
        return [f"{id_}: run {n} must be a mapping"]
    problems = [f"{id_}: run {n} has {p}" for p in _missing(run, (), RUN_FIELDS)]
    kind, ref, os_name = run.get("kind"), run.get("ref"), run.get("os")
    if kind not in KINDS:
        return [*problems, f"{id_}: run {n} has unknown kind {kind}"]
    if not _text(ref):
        problems.append(f"{id_}: run {n} needs a ref")
    elif not _check_ref(kind, ref):
        problems.append(f"{id_}: run {n} ref {ref} has an unknown form")
    if kind == "platform" and "os" not in run:
        problems.append(f"{id_}: run {n} of kind platform needs os")
    elif kind == "platform" and os_name not in OS_NAMES:
        problems.append(f"{id_}: run {n} has unknown os {os_name}")
    elif kind != "platform" and "os" in run:
        problems.append(f"{id_}: run {n} of kind {kind} must not have os")
    return problems


def _structure_list(id_: str, name: str, value: Any, required: tuple[str, ...]) -> list[str]:
    if not isinstance(value, list):
        return [f"{id_}: {name} must be a list"]
    problems: list[str] = []
    for n, item in enumerate(value, 1):
        if not isinstance(item, dict):
            problems.append(f"{id_}: {name} entry {n} must be a mapping")
            continue
        for p in _missing(item, required, set(required)):
            verb = "" if p.startswith("missing") else "has "
            problems.append(f"{id_}: {name} entry {n} {verb}{p}")
        if "ticket" in item and not _ticket(item["ticket"]):
            problems.append(f"{id_}: {name} entry {n} ticket {item['ticket']} has an unknown form")
    return problems


def _structure_entry(entry: Any, index: int) -> list[str]:
    if not isinstance(entry, dict):
        return [f"catalogue: entry {index} must be a mapping"]
    id_ = str(entry.get("id", f"entry {index}"))
    problems = [f"{id_}: {p}" for p in _missing(entry, ENTRY_REQUIRED, ENTRY_FIELDS)]
    if not _ID.match(id_):
        problems.append(f"{id_}: ID has an unknown form")
    for name in ("title", "expected"):
        if name in entry and not _text(entry[name]):
            problems.append(f"{id_}: {name} must be a text")
    runs = entry.get("runs", [])
    if not isinstance(runs, list):
        problems.append(f"{id_}: runs must be a list")
    else:
        for n, run in enumerate(runs, 1):
            problems += _structure_run(id_, n, run)
    problems += _structure_list(
        id_, "implemented_by", entry.get("implemented_by", []), IMPLEMENTED_FIELDS
    )
    implemented = entry.get("implemented_by")
    for n, item in enumerate(implemented if isinstance(implemented, list) else [], 1):
        if isinstance(item, dict) and "tests" in item:
            tests = item["tests"]
            if not isinstance(tests, list) or not tests or not all(map(_text, tests)):
                problems.append(f"{id_}: implemented_by entry {n} tests must be a list of names")
    if "deferred" in entry:
        deferred = entry["deferred"]
        if not isinstance(deferred, dict):
            problems.append(f"{id_}: deferred must be a mapping")
        else:
            problems += [
                f"{id_}: deferred {p}"
                for p in _missing(deferred, DEFERRED_FIELDS, set(DEFERRED_FIELDS))
            ]
            if "ticket" in deferred and not _ticket(deferred["ticket"]):
                problems.append(f"{id_}: deferred ticket {deferred['ticket']} has an unknown form")
    if "deferred_parts" in entry:
        problems += _structure_list(
            id_, "deferred_parts", entry["deferred_parts"], DEFERRED_PART_FIELDS
        )
    threats = entry.get("threats", [])
    if not isinstance(threats, list):
        problems.append(f"{id_}: threats must be a list")
    else:
        problems += [
            f"{id_}: threat {t} has an unknown form"
            for t in threats
            if not (isinstance(t, str) and _THREAT.match(t))
        ]
    return problems


def check_structure(doc: Any) -> list[str]:
    """Validate the shape of a decoded catalogue strictly.

    Args:
        doc: the decoded YAML document.

    Returns:
        One line per problem: unknown or missing fields, a format other than 1, duplicate or
        unordered IDs, unknown kinds, operating systems or ref forms, broken sub-entries.
    """
    if not isinstance(doc, dict):
        return ["catalogue: must be a mapping with format and entries"]
    problems = [f"catalogue: {p}" for p in _missing(doc, ("format", "entries"), TOP_FIELDS)]
    if "format" in doc and doc["format"] != 1:
        problems.append("catalogue: format must be 1")
    entries = doc.get("entries", [])
    if not isinstance(entries, list):
        return [*problems, "catalogue: entries must be a list"]
    seen: set[str] = set()
    previous: str | None = None
    for index, entry in enumerate(entries, 1):
        problems += _structure_entry(entry, index)
        if not isinstance(entry, dict):
            continue
        id_ = str(entry.get("id", ""))
        if id_ in seen:
            problems.append(f"{id_}: duplicate ID")
        seen.add(id_)
        if previous is not None and id_key(id_) < id_key(previous):
            problems.append(f"{id_}: entries are not ordered by ID (after {previous})")
        previous = id_
    return problems


def check_chapter(entries: dict[str, dict[str, Any]], text: str | None) -> list[str]:
    """Check 1: the TM-T entries equal the rows of chapter 24 section 14.

    Args:
        entries: catalogue entries by ID.
        text: docs/spec/24-terminal.md, or None if unreadable.

    Returns:
        One line per problem.
    """
    parsed = chapter_rows(text or "")
    if parsed is None:
        return [f"{CHAPTER}: section 14 has no test table"]
    english, rows = parsed
    where = f"{CHAPTER} section 14"
    problems = [
        f"{i}: catalogue entry has no row in {where}"
        for i in sorted(entries, key=id_key)
        if i.startswith("TM-T") and i not in rows
    ]
    problems += [
        f"{i}: row of {where} has no catalogue entry"
        for i in sorted(rows, key=id_key)
        if i not in entries
    ]
    if english:
        for id_, (title, expected) in sorted(rows.items(), key=lambda r: id_key(r[0])):
            entry = entries.get(id_)
            if entry is None:
                continue
            for name, cell in (("title", title), ("expected", expected)):
                if entry.get(name) != cell:
                    problems.append(f"{id_}: {name} differs from the cell of {where}")
    return problems


def _runs(entry: dict[str, Any]) -> list[dict[str, Any]]:
    return list(entry.get("runs") or [])


def check_plan(
    entries: dict[str, dict[str, Any]], text: str | None, tickets: set[str]
) -> list[str]:
    """Check 2: the PLAN row M2-48 and exit criterion 8 against the runs of the catalogue.

    Args:
        entries: catalogue entries by ID.
        text: docs/PLAN.md, or None if unreadable.
        tickets: the ticket IDs with a row in the plan.

    Returns:
        One line per problem.
    """
    row, criterion = plan_ids(text or "")
    problems: list[str] = []
    if row is None:
        problems.append(f"{PLAN}: no row `| M2-48 |`")
    if criterion is None:
        problems.append(f"{PLAN}: no exit criterion `8. Terminal (M2-48):` in section 8")
    named = "named by the PLAN row M2-48 or exit criterion 8"
    for id_ in sorted((row or set()) | (criterion or set()), key=id_key):
        entry = entries.get(id_)
        if entry is None:
            problems.append(f"{id_}: {named} but has no catalogue entry")
            continue
        if not any(
            r.get("kind") in ("e2e", "unit")
            or (r.get("kind") == "platform" and r.get("os") == "linux")
            for r in _runs(entry)
        ):
            problems.append(
                f"{id_}: {named} but has no run of kind e2e or unit or platform run on linux"
            )
        if "deferred" in entry:
            problems.append(f"{id_}: {named} and must not be deferred")
    for id_ in PERF_IDS:
        if id_ in entries and not any(r.get("kind") == "perf" for r in _runs(entries[id_])):
            problems.append(f"{id_}: needs a run of kind perf")
    for id_ in DOCTOR_MATRIX:
        if id_ not in entries:
            continue
        have = {r.get("os") for r in _runs(entries[id_]) if r.get("kind") == "platform"}
        lacking = [o for o in DOCTOR_PLATFORMS if o not in have]
        if lacking:
            problems.append(f"{id_}: doctor matrix ID needs platform runs on {', '.join(lacking)}")
    for id_ in CONSOLE_PARTS:
        parts = entries.get(id_, {}).get("deferred_parts") or []
        if id_ in entries and not any(p.get("ticket") == CONSOLE_TICKET for p in parts):
            problems.append(
                f"{id_}: needs a deferred_parts entry for its console part with ticket "
                f"{CONSOLE_TICKET}"
            )
    for id_, entry in sorted(entries.items(), key=lambda e: id_key(e[0])):
        for part in entry.get("deferred_parts") or []:
            if part.get("ticket") not in tickets:
                problems.append(
                    f"{id_}: deferred_parts ticket {part.get('ticket')} is no row of {PLAN}"
                )
    return problems


def check_threats(
    entries: dict[str, dict[str, Any]], text: str | None, sources: Sources
) -> list[str]:
    """Check 3: threat model section 6.7 names existing tests and the catalogue mirrors it.

    Args:
        entries: catalogue entries by ID.
        text: docs/security/threat-model.md, or None if unreadable.
        sources: the indexed Go tests and plan tickets.

    Returns:
        One line per problem.
    """
    rows = threat_rows(text or "")
    if rows is None:
        return [f"{THREAT_MODEL}: section 6.7 has no threat table"]
    problems: list[str] = []
    for threat, cell in rows.items():
        ids = ids_in(cell)
        go_tests = _GO_TEST_IN_TEXT.findall(cell)
        if not ids and not go_tests:
            problems.append(f"{threat}: row of {THREAT_MODEL} section 6.7 names no test")
        for id_ in sorted(ids, key=id_key):
            entry = entries.get(id_)
            if entry is None:
                problems.append(f"{threat}: names {id_}, which has no catalogue entry")
            elif threat not in (entry.get("threats") or []):
                problems.append(
                    f"{threat}: names {id_}, whose catalogue entry does not list the threat"
                )
        for name in go_tests:
            if name not in sources.go_tests:
                problems.append(f"{threat}: Go test {name} is no `func {name}(` under services/")
        for ticket in _PLAN_REF.findall(cell):
            if ticket not in sources.plan_tickets:
                problems.append(f"{threat}: PLAN {ticket} is no row of {PLAN}")
    return problems


def _ref_problem(kind: str, ref: str, sources: Sources) -> str | None:
    """Return why a well-formed ref names nothing, or None when it exists."""
    e2e, dart = _E2E_REF.match(ref), _DART_UNIT_REF.match(ref)
    if kind == "e2e" and e2e is not None and e2e.group("file"):
        path = TERMINAL_DIR / "scenarios" / e2e.group("file")
        return None if (sources.repo / path).is_file() else f"names no file {path}"
    if kind == "e2e" and e2e is not None:
        directory, func = (TERMINAL_DIR / "e2e" / e2e.group("pkg")).as_posix(), e2e.group("func")
    elif kind == "platform":
        directory, func = PLATFORM_DIR.as_posix(), ref
    elif kind == "unit" and dart is not None and not ref.startswith("services/"):
        found = sources.dart_test_in(dart.group("name"), dart.group("file"))
        return None if found else "names no Dart test or group"
    else:
        # unit and perf refs of Go tests share the form `<package dir>:<TestFunc>`.
        directory, _, func = ref.partition(":")
    if sources.go_test_in(func, directory):
        return None
    return f"names no `func {func}(` in {directory}"


def check_refs(entries: dict[str, dict[str, Any]], sources: Sources) -> list[str]:
    """Check 4: every ref, ticket and implementing test of the catalogue exists.

    Args:
        entries: catalogue entries by ID, structurally valid.
        sources: the indexed repository.

    Returns:
        One line per problem.
    """
    problems: list[str] = []
    for id_, entry in sorted(entries.items(), key=lambda e: id_key(e[0])):
        for run in _runs(entry):
            why = _ref_problem(run["kind"], run["ref"], sources)
            if why is not None:
                problems.append(f"{id_}: ref {run['ref']} {why}")
        for item in entry.get("implemented_by") or []:
            if item["ticket"] not in sources.plan_tickets:
                problems.append(
                    f"{id_}: implemented_by ticket {item['ticket']} is no row of {PLAN}"
                )
            for test in item["tests"]:
                if test not in sources.go_tests and not sources.dart_test_anywhere(test):
                    problems.append(
                        f"{id_}: implemented_by test {test} is neither a Go test under "
                        f"services/ nor a Dart test of {DART_DIR}"
                    )
        deferred = entry.get("deferred")
        if deferred is not None and deferred["ticket"] not in sources.plan_tickets:
            problems.append(f"{id_}: deferred ticket {deferred['ticket']} is no row of {PLAN}")
    return problems


def load(path: Path) -> Any:
    """Decode the catalogue with duplicate mapping keys rejected.

    Args:
        path: the catalogue file.

    Returns:
        The decoded document.

    Raises:
        CatalogueError: the file is unreadable or not valid YAML.
    """
    text = _read(path)
    if text is None:
        raise CatalogueError(f"cannot read {CATALOGUE}")
    loader = _UniqueKeyLoader(text)
    try:
        return loader.get_single_data()
    except yaml.YAMLError as exc:
        raise CatalogueError(" ".join(str(exc).split())) from exc
    finally:
        loader.dispose()


def check(repo: Path) -> list[str]:
    """Run every check of the gate against a repository.

    Args:
        repo: root of the repository.

    Returns:
        One line per problem, empty when the catalogue and the documents agree. Structural
        problems are returned alone, because the other checks rely on a valid shape.
    """
    try:
        doc = load(repo / CATALOGUE)
    except CatalogueError as exc:
        return [f"catalogue: {exc}"]
    problems = check_structure(doc)
    if problems:
        return problems
    entries = {e["id"]: e for e in doc["entries"]}
    sources = Sources.load(repo)
    problems += check_chapter(entries, _read(repo / CHAPTER))
    problems += check_plan(entries, _read(repo / PLAN), sources.plan_tickets)
    problems += check_threats(entries, _read(repo / THREAT_MODEL), sources)
    problems += check_refs(entries, sources)
    return problems


def main(argv: list[str] | None = None) -> int:
    """Command line entry: check the catalogue and print one line per problem to stderr.

    Args:
        argv: arguments without the program name; None reads sys.argv.

    Returns:
        0 when the gate passes, 1 on any problem.
    """
    parser = argparse.ArgumentParser(description="Check the terminal test catalogue.")
    parser.add_argument("--check", action="store_true", help="check only (the only mode)")
    parser.add_argument("--repo", type=Path, default=DEFAULT_REPO)
    args = parser.parse_args(argv)
    problems = check(args.repo)
    for problem in problems:
        print(f"terminal_catalogue: {problem}", file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
