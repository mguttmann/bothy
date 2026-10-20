# SPDX-License-Identifier: Apache-2.0
"""Generate and check section 4 of docs/operations/configuration.md (ruling L4).

The configuration schema in the server code is the source of every server key: the tool reads
its export `services/server/architecture/config-schema.json` (written by
`services/server/tools/configschema` through `go generate`, held current by
`make codegen-check`), so this check needs no Go toolchain. It joins the schema with the tables
"Konfigurationswerte dieses Kapitels" of the chapters in docs/spec/, the etappe of planned keys
(`tools/configdoc/planned.yaml`) and renders the block between the markers of section 4. Each
row says whether this server version accepts the key: `aktiv` for schema keys, `geplant
(<etappe>)` for server keys of a later ticket, nothing for values of other programs.

Chapter tables: the header is `| Name | Standard | Bedeutung |` or `| Name | Ort | Standard |
Bedeutung |`, every row has as many cells as the header (`\\|` is a pipe inside a cell). The
first cell holds one name, a list (`a.x`, `a.y`) or a shorthand (`a.b.per_minute` / `.burst`,
defaults `30` / `10`). The column "Ort" names the program that reads the value (one of
`PLACES`); without it the chapter default below applies. Chapter 23 holds quality thresholds,
not runtime values (tools/docs/tests/test_config_values.py holds them).

Checks (`--check`), each finding printed as `config-doc: <path>[:<line>]: <id>: <message>`:

    D1   a name appears in more than one chapter table row
    D2   a schema key has no chapter row
    D3   a schema key is in the table of another chapter than its `chapter` field
    D4   a schema key's row names a place other than Server
    D5   schema default and chapter cell differ (`leer` is "", `keiner, Pflicht` is required)
    D6   a planned server key has no rule in planned.yaml, or a rule decides no planned key
    D7   a retired name of retired.yaml occurs in code (fenced block or code span), or its
         BOTHY_ form anywhere, in a tracked text file outside the allowlist
    D8   a Compose service that runs the server (`runs_serve`) has no stop_grace_period
         longer than shutdown.total_timeout (its own BOTHY_SHUTDOWN__TOTAL_TIMEOUT if set), or
         no service runs the server
    D9   two names map to the same environment variable, or the schema export names another
         variable than the name rule
    D10  the committed section 4 differs from the rendered one
    table  a heading, header or row of a chapter table breaks the rules above

Usage: configdoc.py [--check] [--root DIR]

Without `--check` the tool writes section 4 and reports the findings of D1 to D9; with
findings it exits 1 although it wrote, so one run brings the reference up to date and lists
what is left. Exit codes: 0 ok (prints `config-doc: ok (<n> keys, <s> in schema, <p>
planned)`), 1 findings, 2 an input file cannot be read or is malformed, or the markers of
section 4 are missing, repeated or out of order.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from collections.abc import Iterable, Iterator, Mapping, Sequence
from dataclasses import dataclass, field
from datetime import timedelta
from pathlib import Path, PurePosixPath
from typing import Any

import yaml

START = "<!-- config-reference:start -->"
END = "<!-- config-reference:end -->"
REPO = Path(__file__).resolve().parents[2]
SERVER = "Server"
# Chapter 23 names quality thresholds of the repository tool chain, not runtime values;
# tools/docs/tests/test_config_values.py holds them against their sources.
EXCLUDED = frozenset({"23-qualitaet.md"})
# Chapters whose table has no "Ort" column but whose values belong to another program.
DEFAULT_PLACE = {
    "01-produkt.md": "App",
    "15-client-architektur.md": "App",
    "22-import-migration.md": "Werkzeug `bothy import`",
}
# Files that name retired names on purpose: the plan and the ruling log (with its companion
# DECISIONS-EARLY.md, INT-W40/3) record W1, the brief of M1-08 describes it, retired.yaml lists
# the names and the tests use them as fixtures. A trailing "/" allows a whole directory.
RETIRED_ALLOWLIST = (
    "docs/PLAN.md",
    "docs/DECISIONS.md",
    "docs/DECISIONS-EARLY.md",
    "docs/briefs/M1-08-config-reference.md",
    "tools/configdoc/retired.yaml",
    "tools/configdoc/tests/",
)
# Programs a chapter table may name in the column "Ort" (docs/operations/configuration.md
# section 4); an unknown place is a typo until it is added here.
PLACES = frozenset(
    {
        "Server",
        "App",
        "Lasttest",
        "Adapter",
        "Hermes-Adapter",
        "Runner",
        "Host-Dienst",
        "SDK",
        "Abgleich-App",
        "Werkzeug `bothy import`",
        # Options of the setup wizard infra/compose/install.sh (16 section 3.1, brief M7-29).
        "Installationsskript",
    }
)
# The two forms of the header of a table "Konfigurationswerte dieses Kapitels".
HEADERS = (("name", "standard", "bedeutung"), ("name", "ort", "standard", "bedeutung"))
SHUTDOWN_KEY = "shutdown.total_timeout"
SHUTDOWN_CHAPTER = "02-architektur.md"
SHUTDOWN_ENV = "BOTHY_SHUTDOWN__TOTAL_TIMEOUT"
SCHEMA_VERSION = 1
SCHEMA_FIELDS = (
    "name",
    "env",
    "kind",
    "default",
    "required",
    "enum",
    "min",
    "max",
    "chapter",
    "doc",
)
# Etappen that build code (docs/PLAN.md "Schnitt der Etappen"); M0 is the planning stage.
ETAPPE = re.compile(r"^(M[1-8]|V[1-3])$")

_SECTION = re.compile(
    r"^## (?P<num>[0-9]+(?:\.[0-9]+)*)\.? Konfigurationswerte dieses Kapitels\s*$"
)
_TITLE = re.compile(r"^# (?P<title>.+)$")
_NAME = re.compile(r"^[a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+$")
_PREFIX = re.compile(r"^[a-z][a-z0-9_]*(?:\.[a-z0-9_]+)*\.$")
_TICKED = re.compile(r"`([^`]+)`")
_SECTION_TITLE = "Konfigurationswerte dieses Kapitels"
_HEADING = re.compile(r"^ {0,3}#{1,6}(?:\s|$)")
_SECTION_END = re.compile(r"^ {0,3}#{1,2}(?:\s|$)")
_ROW = re.compile(r"^ {0,3}\|")
_PIPE = re.compile(r"(?<!\\)\|")
_SEPARATOR = re.compile(r"^:?-+:?$")
# Fences are recognised at any indentation: inside list items they follow the item's content
# column, and elsewhere the indented lines would be an indented code block, so code as well.
_FENCE_OPEN = re.compile(r"^[ \t]*(`{3,}|~{3,})(.*)$")
_FENCE_CLOSE = re.compile(r"^[ \t]*(`{3,}|~{3,})[ \t]*$")
_BACKTICKS = re.compile(r"`+")
# A list item marker with its indentation and the text after it.
_ITEM = re.compile(r"^(?P<indent>[ \t]*)(?P<marker>[-*+]|[0-9]{1,9}[.)])(?P<rest>(?:[ \t].*)?)$")
# The marker of a block quote line, with one optional space after it.
_QUOTE = re.compile(r"^ {0,3}> ?")
# Links in chapter tables are relative to docs/spec/; the reference lives in docs/operations/.
_LOCAL_LINK = re.compile(r"\]\((?!\.\./|[a-z]+:|#|/)(?P<target>[^)]+)\)")
# Duration grammar of the server configuration (docs/operations/configuration.md section 2).
_CONFIG_DURATION = re.compile(r"^(0|[1-9][0-9]*)(ms|s|m|h|d)$")
# Duration grammar of Compose (Go durations: 1m30s, 75s, 500ms).
_COMPOSE_DURATION = re.compile(r"^(?:[0-9]+(?:\.[0-9]+)?(?:us|ms|s|m|h))+$")
_COMPOSE_PART = re.compile(r"([0-9]+(?:\.[0-9]+)?)(us|ms|s|m|h)")
_UNIT = {
    "us": timedelta(microseconds=1),
    "ms": timedelta(milliseconds=1),
    "s": timedelta(seconds=1),
    "m": timedelta(minutes=1),
    "h": timedelta(hours=1),
    "d": timedelta(days=1),
}
# git exports these to hooks (the pre-push hook runs `make check`); with them set, `git -C` would
# list another repository.
_GIT_LOCATION_VARIABLES = frozenset(
    {"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX"}
)


class InputError(Exception):
    """An input file or a marker is missing or malformed; the check cannot run (exit code 2)."""


@dataclass(frozen=True, order=True)
class Finding:
    """One violated rule, printed as `config-doc: <path>[:<line>]: <id>: <message>`.

    Attributes:
        path: file relative to the repository root.
        line: 1-based line, or 0 when the finding concerns the whole file.
        rule: the rule id (`D1` to `D10` or `table`).
        message: what is wrong and, where it helps, what to do.
    """

    path: str
    line: int
    rule: str
    message: str

    def __str__(self) -> str:
        where = f"{self.path}:{self.line}" if self.line else self.path
        return f"config-doc: {where}: {self.rule}: {self.message}"


def env_name(name: str) -> str:
    """Environment variable of a server key: `BOTHY_` plus the name in upper case, `.` as `__`.

    Example: `env_name("sync.event_retention")` is `"BOTHY_SYNC__EVENT_RETENTION"`.
    """
    return "BOTHY_" + name.upper().replace(".", "__")


@dataclass(frozen=True)
class Value:
    """One named value of a chapter table.

    Attributes:
        name: the key, for example `sync.event_retention`.
        place: the program that reads it (`Server`, `App`, `Lasttest`, ...).
        default: the cell "Standard" as written (with backticks).
        meaning: the remaining cells joined by ` | `, links rewritten for docs/operations/.
        file: file name of the chapter, for example `04-sync-protokoll.md`.
        line: 1-based line of the row in the chapter.
    """

    name: str
    place: str
    default: str
    meaning: str
    file: str
    line: int

    @property
    def env(self) -> str:
        """Environment variable for a server value, "" for values of other programs."""
        return env_name(self.name) if self.place == SERVER else ""


@dataclass(frozen=True)
class Chapter:
    """The table "Konfigurationswerte dieses Kapitels" of one chapter.

    Attributes:
        file: file name, for example `02-architektur.md`.
        title: the first level-1 heading.
        section: number of the section, for example `22`.
        values: the rows in table order.
    """

    file: str
    title: str
    section: str
    values: tuple[Value, ...]


@dataclass(frozen=True)
class SchemaKey:
    """One key of config-schema.json (fields as services/server/tools/configschema writes them).

    Attributes:
        line: 1-based line of the key's `"name"` field in the file, 0 if not found.
    """

    name: str
    env: str
    kind: str
    default: str
    required: bool
    enum: tuple[str, ...]
    min: str
    max: str
    chapter: str
    doc: str
    line: int = 0

    @property
    def type_label(self) -> str:
        """The column "Typ": the kind, for an enum with its values (`enum(api, worker, all)`)."""
        if self.kind == "enum":
            return f"enum({', '.join(self.enum)})"
        return self.kind


@dataclass(frozen=True)
class Rule:
    """A rule of planned.yaml: the etappe of planned keys.

    Attributes:
        match: an exact key name, or a prefix ending in "." (for example `file.`).
        etappe: the etappe that builds the consumer (`M1` to `M8`, `V1` to `V3`).
        line: 1-based line of the rule in planned.yaml.
    """

    match: str
    etappe: str
    line: int

    @property
    def is_prefix(self) -> bool:
        """True for a prefix rule (match ends in ".")."""
        return self.match.endswith(".")


@dataclass(frozen=True)
class Retired:
    """A name retired by W1 (retired.yaml) with the name that replaces it.

    Attributes:
        line: 1-based line of the entry in retired.yaml.
    """

    name: str
    replaced_by: str
    line: int

    @property
    def env(self) -> str:
        """The `BOTHY_` form of the retired name."""
        return env_name(self.name)


@dataclass(frozen=True)
class Paths:
    """Where the inputs live; every path is relative to `root` (the repository)."""

    root: Path = REPO
    spec: str = "docs/spec"
    reference: str = "docs/operations/configuration.md"
    schema: str = "services/server/architecture/config-schema.json"
    planned: str = "tools/configdoc/planned.yaml"
    retired: str = "tools/configdoc/retired.yaml"
    compose: str = "infra/compose/compose.yaml"

    def file(self, rel: str) -> Path:
        """Absolute path of a repository-relative path."""
        return self.root / rel


@dataclass(frozen=True)
class Result:
    """Everything `evaluate` found.

    Attributes:
        findings: findings of D1 to D9 and table rows, sorted.
        current: the reference as committed.
        rendered: the reference with section 4 rendered from the inputs.
        counts: (names in all chapter tables, schema keys, planned server keys).
    """

    findings: list[Finding]
    current: str
    rendered: str
    counts: tuple[int, int, int]
    start_line: int = field(default=0)


# ---- chapter tables ---------------------------------------------------------------------


def _cells(line: str) -> list[str]:
    """Cells of a table row; an escaped pipe `\\|` stays in its cell (GitHub Markdown)."""
    text = line.strip()
    if text.startswith("|"):
        text = text[1:]
    if text.endswith("|") and not text.endswith("\\|"):
        text = text[:-1]
    return [c.strip() for c in _PIPE.split(text)]


def _names(cell: str) -> list[str]:
    """Names of the first cell; raises ValueError with the reason if the cell breaks the rules."""
    names: list[str] = []
    for token in _TICKED.findall(cell):
        name = token
        if token.startswith(".") and names:
            name = names[-1].rsplit(".", 1)[0] + token
        if not _NAME.match(name):
            raise ValueError(f"no value name in `{name}`")
        names.append(name)
    if not names:
        raise ValueError(f"no value name in {cell!r}")
    return names


def _defaults(cell: str, count: int) -> list[str]:
    parts = [p.strip() for p in cell.split(" / ")]
    if count == 1 or len(parts) == 1:
        return [cell] * count
    if len(parts) != count:
        raise ValueError(f"{count} names but {len(parts)} defaults")
    return parts


def _fences(lines: Sequence[str]) -> list[int]:
    """Per line: 0 outside a fenced code block, 1 for an opening or closing fence, 2 inside.

    Fences follow CommonMark: three or more backticks or tildes (at any indentation, see
    `_FENCE_OPEN`); a backtick fence's info string has no backtick; the closing fence uses the
    same character, at least as many times, and nothing else. An unclosed fence runs to the end.
    """
    states: list[int] = []
    fence: tuple[str, int] | None = None
    for line in lines:
        if fence is None:
            m = _FENCE_OPEN.match(line)
            if m and not (m.group(1)[0] == "`" and "`" in m.group(2)):
                fence = (m.group(1)[0], len(m.group(1)))
                states.append(1)
            else:
                states.append(0)
            continue
        m = _FENCE_CLOSE.match(line)
        if m and m.group(1)[0] == fence[0] and len(m.group(1)) >= fence[1]:
            fence = None
            states.append(1)
        else:
            states.append(2)
    return states


def parse(path: Path, rel_dir: str = "docs/spec") -> tuple[Chapter | None, list[Finding]]:
    """Read the table "Konfigurationswerte dieses Kapitels" of one chapter.

    Headings, rows and fences follow Markdown: lines in fenced code blocks are not read, a row
    may be indented by up to three spaces, `\\|` is a pipe inside a cell. The section ends at
    the next heading of level 1 or 2.

    Args:
        path: the chapter file.
        rel_dir: its directory relative to the repository root, for the findings.

    Returns:
        The chapter (None if it has no such section) and a `table` finding for every heading,
        header or row that breaks the rules; such a row is left out, a broken header leaves the
        table empty.

    Raises:
        InputError: the file cannot be read or is not UTF-8.
    """
    rel = f"{rel_dir}/{path.name}"
    lines = _read(path, rel).splitlines()
    fenced = _fences(lines)
    plain = [(n, line) for n, line in enumerate(lines, start=1) if not fenced[n - 1]]
    title = next(
        (m.group("title").strip() for _, line in plain if (m := _TITLE.match(line))), path.stem
    )
    problems: list[Finding] = []
    start: tuple[int, re.Match[str]] | None = None
    for n, line in plain:
        if not _HEADING.match(line) or _SECTION_TITLE not in line:
            continue
        m = _SECTION.match(line)
        if m is None:
            problems.append(
                Finding(rel, n, "table", f"heading must read '## <number>. {_SECTION_TITLE}'")
            )
        elif start is not None:
            problems.append(
                Finding(rel, n, "table", f"second section {_SECTION_TITLE}; one table per chapter")
            )
        else:
            start = (n, m)
    if start is None:
        return None, problems
    rows: list[tuple[int, str]] = []
    for n, line in plain:
        if n <= start[0]:
            continue
        if _SECTION_END.match(line):
            break
        if _ROW.match(line):
            rows.append((n, line))
    values = _rows(path.name, rel, rows, problems)
    return Chapter(path.name, title, start[1].group("num"), tuple(values)), problems


def _rows(
    name: str, rel: str, rows: Sequence[tuple[int, str]], problems: list[Finding]
) -> list[Value]:
    """Values of the table rows of one chapter; broken rows become `table` findings."""
    if not rows:
        return []
    header = tuple(h.lower() for h in _cells(rows[0][1]))
    if header not in HEADERS:
        problems.append(
            Finding(
                rel,
                rows[0][0],
                "table",
                "header must be | Name | Standard | Bedeutung | or | Name | Ort | Standard |"
                " Bedeutung |",
            )
        )
        return []
    separator = _cells(rows[1][1]) if len(rows) > 1 else []
    if len(separator) != len(header) or not all(_SEPARATOR.match(c) for c in separator):
        at = rows[1][0] if len(rows) > 1 else rows[0][0] + 1
        problems.append(Finding(rel, at, "table", "the header needs a separator row (|---|)"))
        return []
    has_place = "ort" in header
    values: list[Value] = []
    for number, line in rows[2:]:
        cells = _cells(line)
        if len(cells) != len(header):
            problems.append(
                Finding(rel, number, "table", f"{len(cells)} cells, the header has {len(header)}")
            )
            continue
        place = cells[1] if has_place else DEFAULT_PLACE.get(name, SERVER)
        standard, meaning = cells[-2], cells[-1]
        if place not in PLACES:
            known = ", ".join(sorted(PLACES))
            problems.append(
                Finding(rel, number, "table", f"unknown place {place!r}; known: {known}")
            )
            continue
        if not standard:
            problems.append(
                Finding(
                    rel, number, "table", "empty Standard cell; write `leer` for an empty default"
                )
            )
            continue
        try:
            names = _names(cells[0])
            defaults = _defaults(standard, len(names))
        except ValueError as exc:
            problems.append(Finding(rel, number, "table", str(exc)))
            continue
        meaning = _LOCAL_LINK.sub(r"](../spec/\g<target>)", meaning)
        values += [
            Value(nm, place, d, meaning, name, number)
            for nm, d in zip(names, defaults, strict=True)
        ]
    return values


def collect(spec_dir: Path, rel_dir: str = "docs/spec") -> tuple[list[Chapter], list[Finding]]:
    """Read the tables of every chapter `NN-*.md` in `spec_dir` except chapter 23.

    Returns:
        The chapters with a table, in file order, and the `table` findings of their rows.

    Raises:
        InputError: `spec_dir` is not a directory.
    """
    if not spec_dir.is_dir():
        raise InputError(f"{rel_dir}: no such directory")
    chapters: list[Chapter] = []
    problems: list[Finding] = []
    for path in sorted(spec_dir.glob("[0-9][0-9]-*.md")):
        if path.name in EXCLUDED:
            continue
        chapter, found = parse(path, rel_dir)
        problems += found
        if chapter is not None:
            chapters.append(chapter)
    return chapters, problems


# ---- inputs -----------------------------------------------------------------------------


def _read(path: Path, rel: str) -> str:
    """Text of an input file; every failure becomes an InputError that names `rel`."""
    try:
        return path.read_text(encoding="utf-8")
    except FileNotFoundError:
        raise InputError(f"{rel}: no such file") from None
    except UnicodeDecodeError:
        raise InputError(f"{rel}: not UTF-8") from None
    except OSError as exc:
        raise InputError(f"{rel}: cannot read ({exc.strerror or exc})") from None


def load_schema(path: Path, rel: str) -> dict[str, SchemaKey]:
    """Read config-schema.json.

    Returns:
        The keys by name.

    Raises:
        InputError: the file is missing, not JSON or not in the format of version 1.
    """
    text = _read(path, rel)
    try:
        doc = json.loads(text)
    except json.JSONDecodeError as exc:
        raise InputError(f"{rel}: not JSON ({exc.msg}, line {exc.lineno})") from None
    if not isinstance(doc, dict) or set(doc) != {"version", "keys"}:
        raise InputError(f"{rel}: want an object with version and keys")
    if doc["version"] != SCHEMA_VERSION:
        raise InputError(f"{rel}: version {doc['version']!r}, this tool reads {SCHEMA_VERSION}")
    if not isinstance(doc["keys"], list):
        raise InputError(f"{rel}: keys must be a list")
    lines = text.splitlines()
    keys: dict[str, SchemaKey] = {}
    for entry in doc["keys"]:
        key = _schema_key(entry, rel, lines)
        if key.name in keys:
            raise InputError(f"{rel}: key {key.name!r} twice")
        keys[key.name] = key
    return keys


def _schema_key(entry: object, rel: str, lines: Sequence[str]) -> SchemaKey:
    if not isinstance(entry, dict) or tuple(sorted(entry)) != tuple(sorted(SCHEMA_FIELDS)):
        raise InputError(f"{rel}: every key needs exactly the fields {', '.join(SCHEMA_FIELDS)}")
    name = entry["name"]
    if not isinstance(name, str):
        raise InputError(f"{rel}: name must be a string")
    where = f"{rel}: key {name!r}"
    for field_name in ("env", "kind", "default", "min", "max", "chapter", "doc"):
        if not isinstance(entry[field_name], str):
            raise InputError(f"{where}: {field_name} must be a string")
    if not isinstance(entry["required"], bool):
        raise InputError(f"{where}: required must be a bool")
    enum = entry["enum"]
    if not isinstance(enum, list) or not all(isinstance(v, str) for v in enum):
        raise InputError(f"{where}: enum must be a list of strings")
    marker = f'"name": {json.dumps(name)},'
    line = next((i for i, text in enumerate(lines, start=1) if text.strip() == marker), 0)
    return SchemaKey(
        name=name,
        env=entry["env"],
        kind=entry["kind"],
        default=entry["default"],
        required=entry["required"],
        enum=tuple(enum),
        min=entry["min"],
        max=entry["max"],
        chapter=entry["chapter"],
        doc=entry["doc"],
        line=line,
    )


def _compose_yaml(path: Path, rel: str) -> yaml.Node | None:
    text = _read(path, rel)
    try:
        node: yaml.Node | None = yaml.compose(text, Loader=yaml.SafeLoader)
    except yaml.YAMLError as exc:
        raise InputError(f"{rel}: not YAML ({exc})") from None
    return node


def _list_of_mappings(
    path: Path, rel: str, top: str, fields: tuple[str, str]
) -> list[tuple[dict[str, yaml.Node], int]]:
    """The list under the single key `top` of a YAML file, each item a mapping of `fields`."""
    root = _compose_yaml(path, rel)
    pairs = root.value if isinstance(root, yaml.MappingNode) else None
    if pairs is None or [k.value for k, _ in pairs] != [top]:
        raise InputError(f"{rel}: want a mapping with the key {top}")
    items = pairs[0][1]
    if not isinstance(items, yaml.SequenceNode):
        raise InputError(f"{rel}: {top} must be a list")
    out: list[tuple[dict[str, yaml.Node], int]] = []
    for item in items.value:
        line = item.start_mark.line + 1
        if (
            not isinstance(item, yaml.MappingNode)
            or not all(isinstance(k, yaml.ScalarNode) for k, _ in item.value)
            or sorted(str(k.value) for k, _ in item.value) != sorted(fields)
        ):
            raise InputError(f"{rel}:{line}: each entry needs exactly {fields[0]} and {fields[1]}")
        values = {k.value: v for k, v in item.value}
        for name, node in values.items():
            if not isinstance(node, yaml.ScalarNode):
                raise InputError(f"{rel}:{line}: {name} must be a string")
        out.append((values, line))
    return out


def load_planned(path: Path, rel: str) -> list[Rule]:
    """Read planned.yaml (`rules:` with `match` and `etappe` per entry).

    Raises:
        InputError: the file is missing or malformed, a match is neither a key name nor a prefix
            ending in ".", an etappe is not M1 to M8 or V1 to V3, or a match appears twice.
    """
    rules: list[Rule] = []
    for values, line in _list_of_mappings(path, rel, "rules", ("match", "etappe")):
        match, etappe = str(values["match"].value), str(values["etappe"].value)
        if not (_NAME.match(match) or _PREFIX.match(match)):
            raise InputError(
                f"{rel}:{line}: match {match!r} is neither a key name nor a prefix ending in '.'"
            )
        if not ETAPPE.match(etappe):
            raise InputError(f"{rel}:{line}: etappe {etappe!r} is not M1 to M8 or V1 to V3")
        if any(r.match == match for r in rules):
            raise InputError(f"{rel}:{line}: match {match!r} twice")
        rules.append(Rule(match, etappe, line))
    return rules


def load_retired(path: Path, rel: str) -> list[Retired]:
    """Read retired.yaml (`retired:` with `name` and `replaced_by` per entry).

    Raises:
        InputError: the file is missing or malformed, a name is not an exact key name (retired
            names are never prefixes), or a name appears twice.
    """
    retired: list[Retired] = []
    for values, line in _list_of_mappings(path, rel, "retired", ("name", "replaced_by")):
        name, replaced_by = str(values["name"].value), str(values["replaced_by"].value)
        if not _NAME.match(name):
            raise InputError(f"{rel}:{line}: name {name!r} is not an exact key name")
        if any(r.name == name for r in retired):
            raise InputError(f"{rel}:{line}: name {name!r} twice")
        retired.append(Retired(name, replaced_by, line))
    return retired


def etappe_of(name: str, rules: Iterable[Rule]) -> Rule | None:
    """The rule that gives the etappe of `name`: the exact rule, else the longest prefix rule.

    Example: with rules `a.` and `a.b.c`, `a.b.c` gets the exact rule and `a.x` the prefix rule.
    """
    best: Rule | None = None
    for rule in rules:
        if rule.match == name:
            return rule
        longer = best is None or len(rule.match) > len(best.match)
        if rule.is_prefix and name.startswith(rule.match) and longer:
            best = rule
    return best


# ---- checks -----------------------------------------------------------------------------


def _values(chapters: Iterable[Chapter]) -> list[Value]:
    return [v for ch in chapters for v in ch.values]


def check_names(chapters: Sequence[Chapter], rel_dir: str = "docs/spec") -> list[Finding]:
    """D1: every name stands in exactly one chapter table row."""
    first: dict[str, Value] = {}
    found: list[Finding] = []
    for v in _values(chapters):
        if v.name in first:
            seen = first[v.name]
            found.append(
                Finding(
                    f"{rel_dir}/{v.file}",
                    v.line,
                    "D1",
                    f"{v.name} is named twice (also {rel_dir}/{seen.file}:{seen.line}); one name"
                    " per value (ruling L4)",
                )
            )
        else:
            first[v.name] = v
    return found


def default_matches(cell: str, key: SchemaKey) -> bool:
    """D5: does the chapter cell "Standard" say what the schema says?

    The cell without backticks equals the default; `leer` stands for ""; `keiner, Pflicht`
    stands for a required key without default.
    """
    text = cell.strip()
    if text == "keiner, Pflicht":
        return key.required and key.default == ""
    if key.required:
        return False
    if text == "leer":
        return key.default == ""
    return text.replace("`", "") == key.default


def check_schema(
    chapters: Sequence[Chapter], schema: Mapping[str, SchemaKey], paths: Paths
) -> list[Finding]:
    """D2 to D5: every schema key has its row in its chapter, with place Server and its default."""
    rows: dict[str, list[Value]] = {}
    for v in _values(chapters):
        rows.setdefault(v.name, []).append(v)
    found: list[Finding] = []
    for key in sorted(schema.values(), key=lambda k: k.name):
        if key.name not in rows:
            found.append(
                Finding(
                    paths.schema,
                    key.line,
                    "D2",
                    f"{key.name} has no row in a chapter table; add it to the table of"
                    f" {key.chapter}",
                )
            )
            continue
        for v in rows[key.name]:
            where = f"{paths.spec}/{v.file}"
            if v.file != key.chapter:
                found.append(
                    Finding(
                        where,
                        v.line,
                        "D3",
                        f"{key.name} is in the table of {v.file}, the schema names {key.chapter}",
                    )
                )
            if v.place != SERVER:
                found.append(
                    Finding(
                        where,
                        v.line,
                        "D4",
                        f"{key.name} is in the schema, but its row names the place {v.place}, not"
                        f" {SERVER}",
                    )
                )
            if not default_matches(v.default, key):
                required = " (required)" if key.required else ""
                found.append(
                    Finding(
                        where,
                        v.line,
                        "D5",
                        f"{key.name}: schema default {key.default!r}{required}, chapter cell"
                        f" {v.default!r}",
                    )
                )
    return found


def planned(chapters: Iterable[Chapter], schema: Mapping[str, SchemaKey]) -> list[Value]:
    """Chapter values with place Server that the schema does not know yet."""
    return [v for v in _values(chapters) if v.place == SERVER and v.name not in schema]


def check_planned(
    chapters: Sequence[Chapter],
    schema: Mapping[str, SchemaKey],
    rules: Sequence[Rule],
    paths: Paths,
) -> list[Finding]:
    """D6: every planned server key has an etappe, and every rule gives the etappe of one."""
    used: set[str] = set()
    found: list[Finding] = []
    for v in planned(chapters, schema):
        rule = etappe_of(v.name, rules)
        if rule is None:
            found.append(
                Finding(
                    f"{paths.spec}/{v.file}",
                    v.line,
                    "D6",
                    f"{v.name} is planned but no rule of {paths.planned} gives its etappe",
                )
            )
        else:
            used.add(rule.match)
    for rule in rules:
        if rule.match not in used:
            found.append(
                Finding(
                    paths.planned,
                    rule.line,
                    "D6",
                    f"rule {rule.match!r} gives the etappe of no planned key (stale); remove it",
                )
            )
    return found


def tracked_files(root: Path) -> list[str]:
    """Paths git tracks under `root` (`git ls-files`), relative to it.

    Raises:
        InputError: `root` is not inside a git working tree.
    """
    env = {k: v for k, v in os.environ.items() if k not in _GIT_LOCATION_VARIABLES}
    try:
        listing = subprocess.run(
            ["git", "-C", str(root), "ls-files", "-z"],
            env=env,
            check=True,
            capture_output=True,
        ).stdout
    except (OSError, subprocess.CalledProcessError) as exc:
        raise InputError(f"{root}: git ls-files failed ({exc})") from None
    return sorted({p for p in os.fsdecode(listing).split("\0") if p})


def _allowed(rel: str, allow: Iterable[str]) -> bool:
    return any(rel.startswith(a) if a.endswith("/") else rel == a for a in allow)


def code_parts(text: str, markdown: bool = True) -> Iterator[tuple[int, str]]:
    """The code of a text as (1-based line where it starts, code), in text order.

    Code is every line inside a fenced code block and every code span. The blocks follow
    CommonMark with GitHub tables, as far as the repository's Markdown needs it:

    - A paragraph ends at a blank line, a fence, a heading, a table (a row followed by a
      delimiter row; each row is a block of its own and each cell an inline context), a block
      quote that starts after a non-quote line, and a list item. Outside a list only a bullet
      item indented by at most three spaces or an ordered item numbered 1 interrupts a
      paragraph; inside a list every item marker does (sibling or nested item).
    - Quote markers (`>`) are removed from the lines of a quote paragraph.
    - Spans follow CommonMark: a run of N backticks that is not escaped by a backslash opens a
      span that the next run of exactly N backticks closes; a span covers line breaks of its
      paragraph, and a run without its closing run is plain text.

    Indented code blocks are not code here. On every tracked Markdown file, tools/configdoc/tests
    holds that this function finds every name in code that a CommonMark parser finds; it finds
    more on purpose: table cells beyond the header, which GitHub tables drop, and spans in HTML
    blocks, HTML comments and indented code blocks. With `markdown=False` (other text files,
    where `#` starts a comment, not a heading) only fences and blank lines end a paragraph.
    """
    lines = text.splitlines()
    fenced = _fences(lines)
    table = _table_rows(lines, fenced)
    paragraph: list[str] = []
    first = 0
    in_list = False
    quote = False
    for number, (line, state) in enumerate(zip(lines, fenced, strict=True), start=1):
        if state or not line.strip():
            yield from _spans("\n".join(paragraph), first)
            paragraph = []
            if state == 2:
                yield number, line
            if not line.strip() and not state:
                quote = False
            continue
        if not markdown:
            if not paragraph:
                first = number
            paragraph.append(line)
            continue
        if table[number - 1]:
            yield from _spans("\n".join(paragraph), first)
            paragraph = []
            for cell in _cells(line):
                yield from _spans(cell, number)
            continue
        is_quote = bool(_QUOTE.match(line))
        starts = (
            bool(_HEADING.match(line))
            or (is_quote and not quote)
            or _interrupts_as_item(line, in_list, bool(paragraph))
        )
        if paragraph and starts:
            yield from _spans("\n".join(paragraph), first)
            paragraph = []
        if not paragraph:
            first = number
            in_list = in_list or bool(_ITEM.match(line))
            if not _ITEM.match(line) and not line.startswith((" ", "\t")) and not is_quote:
                in_list = False
        quote = is_quote or (quote and bool(paragraph))
        paragraph.append(_QUOTE.sub("", line) if is_quote else line)
        if _HEADING.match(line):
            yield from _spans("\n".join(paragraph), first)
            paragraph = []
    yield from _spans("\n".join(paragraph), first)


def _interrupts_as_item(line: str, in_list: bool, continuing: bool) -> bool:
    """Does a list item marker on `line` start a new block (CommonMark "can interrupt")?"""
    m = _ITEM.match(line)
    if not m or not m.group("rest").strip():
        return False
    if in_list or not continuing:
        return True
    if len(m.group("indent")) > 3:
        return False
    marker = m.group("marker")
    return marker in "-*+" or marker[:-1] == "1"


def _table_rows(lines: Sequence[str], fenced: Sequence[int]) -> list[bool]:
    """Per line: is it a row of a GitHub table (header, delimiter or body row)?

    A table starts with a row followed by a delimiter row with as many cells, and goes on until
    a blank line or a line that is not a row.
    """
    rows = [False] * len(lines)
    index = 0
    while index < len(lines) - 1:
        header, delimiter = lines[index], lines[index + 1]
        cells = _cells(delimiter) if "|" in delimiter else []
        if (
            not fenced[index]
            and not fenced[index + 1]
            and "|" in header
            and cells
            and all(_SEPARATOR.match(c.strip()) for c in cells)
            and len(cells) == len(_cells(header))
        ):
            end = index + 2
            while end < len(lines) and not fenced[end] and lines[end].strip() and "|" in lines[end]:
                end += 1
            for k in range(index, end):
                rows[k] = True
            index = end
        else:
            index += 1
    return rows


def _spans(paragraph: str, first: int) -> Iterator[tuple[int, str]]:
    """Code spans of one paragraph that starts on line `first`."""
    runs = [(m.start(), m.end()) for m in _BACKTICKS.finditer(paragraph)]
    index = 0
    while index < len(runs):
        start, end = runs[index]
        if _escaped(paragraph, start):
            start += 1
        size = end - start
        closing = next(
            (k for k in range(index + 1, len(runs)) if runs[k][1] - runs[k][0] == size), None
        )
        if size == 0 or closing is None:
            index += 1
            continue
        line = first + paragraph.count("\n", 0, end)
        yield line, paragraph[end : runs[closing][0]]
        index = closing + 1


def _escaped(text: str, position: int) -> bool:
    """Is the character at `position` escaped by an odd number of backslashes before it?"""
    count = 0
    while position - count - 1 >= 0 and text[position - count - 1] == "\\":
        count += 1
    return count % 2 == 1


def check_retired(
    root: Path,
    retired: Sequence[Retired],
    allow: Iterable[str] = RETIRED_ALLOWLIST,
    texts: Mapping[str, str] | None = None,
) -> list[Finding]:
    """D7: no retired name in code and no `BOTHY_` form of one in a tracked text file.

    Code is a fenced code block or a code span (`code_parts`); the `BOTHY_` form counts
    anywhere. A name counts only as a whole name: `db.system.name` or `http.listen_addr_v2`
    are other names. Files with a NUL byte or invalid UTF-8 are not text and are skipped; so are
    the allowlisted files and files missing from the working tree.

    Args:
        root: the repository; the files are those of `git ls-files`.
        retired: the entries of retired.yaml.
        allow: files (or directories ending in "/") that may name retired names.
        texts: text to check instead of the file content, by path; the reference passes its
            rendered form, because make config-reference replaces its generated block.
    """
    if not retired:
        return []
    by_name = {r.name: r for r in retired}
    by_env = {r.env: r for r in retired}
    names = re.compile(
        r"(?<![A-Za-z0-9_.])("
        + "|".join(re.escape(n) for n in sorted(by_name, key=len, reverse=True))
        + r")(?![A-Za-z0-9_]|\.[A-Za-z0-9_])"
    )
    envs = re.compile(
        r"(?<![A-Za-z0-9_])("
        + "|".join(re.escape(e) for e in sorted(by_env, key=len, reverse=True))
        + r")(?![A-Za-z0-9_])"
    )
    allow = tuple(allow)
    texts = texts or {}
    found: list[Finding] = []
    for rel in tracked_files(root):
        text = texts.get(rel)
        if _allowed(rel, allow):
            continue
        if text is None:
            text = _text_file(root / rel)
        if text is None or (not any(n in text for n in by_name) and "BOTHY_" not in text):
            continue
        for start, code in code_parts(text, markdown=rel.endswith(".md")):
            for m in names.finditer(code):
                r = by_name[m.group(1)]
                number = start + code.count("\n", 0, m.start())
                found.append(
                    Finding(rel, number, "D7", f"{r.name} is retired (W1); use {r.replaced_by}")
                )
        for number, line in enumerate(text.splitlines(), start=1):
            for m in envs.finditer(line):
                r = by_env[m.group(1)]
                found.append(
                    Finding(
                        rel,
                        number,
                        "D7",
                        f"{r.env} is the environment form of the retired {r.name} (W1); use"
                        f" {r.replaced_by}",
                    )
                )
    return found


def _text_file(path: Path) -> str | None:
    """Content of a tracked file if it is a regular UTF-8 text file without NUL bytes."""
    if path.is_symlink() or not path.is_file():
        return None
    data = path.read_bytes()
    if b"\0" in data:
        return None
    try:
        return data.decode("utf-8")
    except UnicodeDecodeError:
        return None


def config_duration(text: str) -> timedelta | None:
    """A duration in the grammar of the server configuration (`60s`, `30d`), else None."""
    m = _CONFIG_DURATION.match(text)
    if not m:
        return None
    try:
        return int(m.group(1)) * _UNIT[m.group(2)]
    except OverflowError:
        return None


def compose_duration(text: str) -> timedelta | None:
    """A Compose duration (`75s`, `1m30s`, `500ms`), else None."""
    if not _COMPOSE_DURATION.match(text):
        return None
    try:
        return sum((float(n) * _UNIT[u] for n, u in _COMPOSE_PART.findall(text)), start=timedelta())
    except OverflowError:
        return None


def _total_timeout(
    chapters: Sequence[Chapter], schema: Mapping[str, SchemaKey], where: str
) -> tuple[tuple[timedelta, str] | None, list[Finding]]:
    """shutdown.total_timeout: the schema default, else the cell of chapter 02."""
    if SHUTDOWN_KEY in schema:
        text = schema[SHUTDOWN_KEY].default
    else:
        cells = [
            v.default.replace("`", "")
            for v in _values(chapters)
            if v.name == SHUTDOWN_KEY and v.file == SHUTDOWN_CHAPTER
        ]
        if not cells:
            return None, [
                Finding(
                    where,
                    0,
                    "D8",
                    f"{SHUTDOWN_KEY} is neither in the schema nor in the table of"
                    f" {SHUTDOWN_CHAPTER}",
                )
            ]
        text = cells[0]
    limit = config_duration(text)
    if limit is None:
        return None, [
            Finding(
                where,
                0,
                "D8",
                f"cannot read {SHUTDOWN_KEY} {text!r} (want a duration such as 60s)",
            )
        ]
    return (limit, text), []


def _service_lines(node: yaml.Node | None) -> dict[str, int]:
    """Line of every service name in the composed Compose file."""
    if not isinstance(node, yaml.MappingNode):
        return {}
    for k, v in node.value:
        if k.value == "services" and isinstance(v, yaml.MappingNode):
            return {str(sk.value): sk.start_mark.line + 1 for sk, _ in v.value}
    return {}


def check_compose(
    chapters: Sequence[Chapter], schema: Mapping[str, SchemaKey], paths: Paths
) -> list[Finding]:
    """D8: every Compose service that runs `serve` waits longer than shutdown.total_timeout.

    The services are read after YAML merge keys (`<<: *api`); a service runs the server when
    `runs_serve` says so. The limit is the value the service sets for
    `BOTHY_SHUTDOWN__TOTAL_TIMEOUT` in `environment`, else the schema default, else the cell of
    chapter 02 (`env_file` and the configuration file are not read). The orchestrator must wait
    longer than the server's own shutdown (02 "Shutdown"). A Compose file in which no service
    runs the server is a finding too: the rule would hold nothing.

    Raises:
        InputError: the Compose file is missing or not YAML.
    """
    path = paths.file(paths.compose)
    node = _compose_yaml(path, paths.compose)
    try:
        doc: Any = yaml.safe_load(_read(path, paths.compose))
    except yaml.YAMLError as exc:
        raise InputError(f"{paths.compose}: not YAML ({exc})") from None
    limit, found = _total_timeout(chapters, schema, paths.compose)
    services = doc.get("services") if isinstance(doc, dict) else None
    if not isinstance(services, dict) or not services:
        return [*found, Finding(paths.compose, 0, "D8", "no services")]
    lines = _service_lines(node)
    serving = serve_services(services)
    if not serving:
        found.append(
            Finding(
                paths.compose,
                0,
                "D8",
                "no service runs `serve`; D8 would hold nothing (a service must run serve or"
                " bothy serve)",
            )
        )
    by_name = {str(key): value for key, value in services.items()}
    for name in serving:
        service = by_name[name]
        line = lines.get(name, 0)
        own = _service_timeout(service)
        if own is not None:
            value = config_duration(own)
            if value is None:
                found.append(
                    Finding(
                        paths.compose,
                        line,
                        "D8",
                        f"service {name}: cannot read {SHUTDOWN_ENV} {own!r} (want a duration such"
                        " as 60s)",
                    )
                )
                continue
            wanted = (value, f"{own}, {SHUTDOWN_ENV} of the service")
        elif limit is None:
            continue
        else:
            wanted = limit
        found += _grace_findings(name, service.get("stop_grace_period"), wanted, line, paths)
    return found


def _grace_findings(
    name: str, grace: object, limit: tuple[timedelta, str], line: int, paths: Paths
) -> list[Finding]:
    """D8 for one service: stop_grace_period exists, is readable and longer than `limit`."""
    if grace is None:
        return [
            Finding(
                paths.compose,
                line,
                "D8",
                f"service {name} runs `serve` without stop_grace_period; set it longer than"
                f" {SHUTDOWN_KEY} ({limit[1]})",
            )
        ]
    wait = compose_duration(str(grace))
    if wait is None:
        return [
            Finding(
                paths.compose,
                line,
                "D8",
                f"service {name}: cannot read stop_grace_period {str(grace)!r} (want a Compose"
                " duration such as 75s or 1m30s)",
            )
        ]
    if wait <= limit[0]:
        return [
            Finding(
                paths.compose,
                line,
                "D8",
                f"service {name}: stop_grace_period {grace} is not longer than {SHUTDOWN_KEY}"
                f" ({limit[1]})",
            )
        ]
    return []


def _words(value: object) -> list[str]:
    """Words of a Compose `entrypoint` or `command` given as a list or as a string."""
    if isinstance(value, list):
        return [str(v) for v in value]
    return str(value or "").split()


def runs_serve(service: Mapping[str, Any]) -> bool:
    """Does a Compose service run the server: `entrypoint` plus `command` start with `serve`
    (the image's entrypoint is the binary) or with the binary `bothy` (any path) followed by
    `serve`?
    """
    words = _words(service.get("entrypoint")) + _words(service.get("command"))
    if not words:
        return False
    if words[0] == "serve":
        return True
    return PurePosixPath(words[0]).name == "bothy" and words[1:2] == ["serve"]


def serve_services(services: Mapping[Any, Any]) -> list[str]:
    """Names of the Compose services that run the server (`runs_serve`), in file order."""
    return [str(n) for n, s in services.items() if isinstance(s, dict) and runs_serve(s)]


def _service_timeout(service: Mapping[str, Any]) -> str | None:
    """The value a service sets for shutdown.total_timeout in its `environment`, if any; a
    variable without value (Compose takes it from the host) is "", which no duration matches."""
    env = service.get("environment")
    if isinstance(env, dict):
        if SHUTDOWN_ENV not in env:
            return None
        value = env[SHUTDOWN_ENV]
        return "" if value is None else str(value)
    if isinstance(env, list):
        for item in env:
            key, _, value = str(item).partition("=")
            if key == SHUTDOWN_ENV:
                return value
    return None


def check_env(
    chapters: Sequence[Chapter], schema: Mapping[str, SchemaKey], paths: Paths
) -> list[Finding]:
    """D9: no two names share an environment variable (`a.b__c` and `a.b.c` would), and the
    variable the schema export names for a key is the one of the name rule (`env_name`)."""
    owner: dict[str, str] = {}
    found: list[Finding] = []

    def claim(env: str, name: str, where: str, line: int) -> None:
        other = owner.setdefault(env, name)
        if other != name:
            found.append(
                Finding(
                    where, line, "D9", f"{env} is the environment variable of {other} and {name}"
                )
            )

    for v in _values(chapters):
        if v.place == SERVER:
            env = schema[v.name].env if v.name in schema else v.env
            claim(env, v.name, f"{paths.spec}/{v.file}", v.line)
    for key in sorted(schema.values(), key=lambda k: k.name):
        claim(key.env, key.name, paths.schema, key.line)
        if key.env != env_name(key.name):
            found.append(
                Finding(
                    paths.schema,
                    key.line,
                    "D9",
                    f"{key.name}: the schema names {key.env}, the name rule gives"
                    f" {env_name(key.name)}",
                )
            )
    return found


# ---- rendering --------------------------------------------------------------------------


def _row(v: Value, schema: Mapping[str, SchemaKey], rules: Sequence[Rule]) -> str:
    key = schema.get(v.name)
    if key is not None:
        env, kind, state = key.env, key.type_label, "aktiv"
    elif v.place == SERVER:
        rule = etappe_of(v.name, rules)
        env, kind, state = v.env, "", f"geplant ({rule.etappe})" if rule else "geplant"
    else:
        env, kind, state = "", "", ""
    env_cell = f"`{env}`" if env else ""
    return f"| `{v.name}` | {env_cell} | {v.place} | {kind} | {v.default} | {v.meaning} | {state} |"


def render(
    reference: str,
    chapters: Sequence[Chapter],
    schema: Mapping[str, SchemaKey],
    rules: Sequence[Rule],
) -> str:
    """The reference with the block between the markers rendered from the inputs.

    One subsection per chapter with the columns Schlüssel, Umgebung (server keys only), Ort, Typ
    (schema keys only), Standard and Bedeutung (from the chapter) and Stand (`aktiv`,
    `geplant (<etappe>)` or empty for other programs). The output depends only on the inputs.

    Raises:
        InputError: the reference does not have exactly one marker line of each kind, start
            before end (`marker_lines`).
    """
    start, end = marker_lines(reference)
    out: list[str] = []
    for ch in chapters:
        out += [
            f"### {ch.title}",
            "",
            f"Quelle: [{ch.file}](../spec/{ch.file}), Abschnitt {ch.section}.",
            "",
            "| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |",
            "|---|---|---|---|---|---|---|",
        ]
        out += [_row(v, schema, rules) for v in ch.values]
        out.append("")
    lines = reference.splitlines(keepends=True)
    return "".join(lines[: start + 1]) + "\n".join(out) + "".join(lines[end:])


def marker_lines(reference: str) -> tuple[int, int]:
    """0-based lines of the start and end marker of section 4.

    A marker is a line that holds only the marker text; a marker named inside prose does not
    count, so the introduction may mention it.

    Raises:
        InputError: not exactly one start and one end marker line, or the end comes first.
    """
    lines = reference.splitlines()
    starts = [i for i, line in enumerate(lines) if line.strip() == START]
    ends = [i for i, line in enumerate(lines) if line.strip() == END]
    if len(starts) != 1 or len(ends) != 1 or starts[0] > ends[0]:
        raise InputError(
            f"configuration.md needs the markers: exactly one line {START} and after it one"
            f" line {END}"
        )
    return starts[0], ends[0]


# ---- driver -----------------------------------------------------------------------------


def evaluate(paths: Paths) -> Result:
    """Load every input, run D1 to D9 and render section 4.

    Raises:
        InputError: an input is missing or malformed, or the reference lacks a marker.
    """
    chapters, found = collect(paths.file(paths.spec), paths.spec)
    schema = load_schema(paths.file(paths.schema), paths.schema)
    rules = load_planned(paths.file(paths.planned), paths.planned)
    retired = load_retired(paths.file(paths.retired), paths.retired)
    current = _read(paths.file(paths.reference), paths.reference)
    rendered = render(current, chapters, schema, rules)
    found += check_names(chapters, paths.spec)
    found += check_schema(chapters, schema, paths)
    found += check_planned(chapters, schema, rules, paths)
    # The reference is checked as it will be written: its generated block follows the chapters,
    # which D7 checks themselves, and D10 reports a stale block.
    found += check_retired(paths.root, retired, texts={paths.reference: rendered})
    found += check_compose(chapters, schema, paths)
    found += check_env(chapters, schema, paths)
    names = {v.name for v in _values(chapters)}
    counts = (len(names), len(schema), len(planned(chapters, schema)))
    return Result(sorted(found), current, rendered, counts, marker_lines(current)[0] + 1)


def _with_stale(result: Result, paths: Paths) -> list[Finding]:
    """The findings of `result` plus D10 when the committed section 4 is not the rendered one."""
    found = list(result.findings)
    if result.rendered != result.current:
        found.append(
            Finding(
                paths.reference,
                result.start_line,
                "D10",
                "section 4 is stale; run `make config-reference`",
            )
        )
    return found


def check(paths: Paths) -> list[Finding]:
    """Every finding of D1 to D10 (`make config-doc-check`).

    Raises:
        InputError: as `evaluate`.
    """
    return _with_stale(evaluate(paths), paths)


def main(argv: list[str] | None = None) -> int:
    """Command line: write section 4, or check everything with `--check`; returns the exit code."""
    parser = argparse.ArgumentParser(
        description="Generate or check section 4 of docs/operations/configuration.md."
    )
    parser.add_argument("--check", action="store_true", help="check only (D1 to D10)")
    parser.add_argument("--root", type=Path, default=REPO, help="repository root")
    args = parser.parse_args(argv)
    paths = Paths(root=args.root)
    try:
        result = evaluate(paths)
    except InputError as exc:
        print(f"config-doc: {exc}", file=sys.stderr)
        return 2
    keys, in_schema, planned_keys = result.counts
    summary = f"{keys} keys, {in_schema} in schema, {planned_keys} planned"
    if args.check:
        found = _with_stale(result, paths)
    else:
        found = list(result.findings)
        if result.rendered != result.current:
            paths.file(paths.reference).write_text(result.rendered, encoding="utf-8")
            print(f"config-doc: wrote {paths.reference} ({summary})")
    for finding in found:
        print(finding, file=sys.stderr)
    if found:
        return 1
    print(f"config-doc: ok ({summary})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
