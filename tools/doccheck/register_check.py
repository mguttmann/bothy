# SPDX-License-Identifier: Apache-2.0
"""Module register checked against the core contract (gate `register-check`, M1-06, ADR 0050).

The module register `services/server/architecture/modules.yaml` is the single source of module
boundaries. This check keeps it equal to the core contract and keeps the documents that repeat it
equal to the register. It compares and never generates: the tables of chapter 02 mix machine
columns with prose columns that people maintain ("Verantwortung", "Grund je Kante"), so only the
machine columns count (docs/DECISIONS.md, M1-06).

Sources, repository-relative; Markdown sections are found by their title, never by number, and
a title must be unique in its file:

- the register: `modules.<m>.tables`, `actions`, `events`, `ephemeral_events`, `ws_ops`,
  `depends_on`, and `kernel.core` plus `kernel.ports`;
- docs/spec/03-datenmodell.md, section "Module" (module set; column "Tabellen" is `ja` or
  `keine`) and section "Tabellen" (each heading `#### `<name>`` is a table whose owner is the
  first `Modul: `<m>`` before its first table row; headings under "Technische Tabellen" are no
  module tables; a heading naming a table at another level is a finding);
- docs/spec/actions.yaml through `load_catalog` of tools/catalog: actions, durable and ephemeral
  event types, gateway ops with their direction;
- docs/spec/02-architektur.md, sections "Modulliste", "Erlaubte Abhängigkeiten",
  "Querschnitts-Ports (`internal/kernel`)", "Aktionen je Modul" and "Ereignistypen und
  WebSocket-Ops je Modul": the one table of each section as Python-Markdown finds it (rows with
  or without a leading `|`, a pipe in a code span splits no cell), each row starting with a
  backticked module (or package); a cell lists names as code spans (`` `x` `` or
  `<code>x</code>`), and an empty cell or one starting with the word `nichts` or `keine` (and
  naming nothing) is the empty set;
- docs/architecture/c4-3-komponenten-server.mmd: every line is front matter, blank, a `%%`
  comment or one statement: a keyword line (`flowchart`, `subgraph`, `end`, `classDef`, `style`
  and the like), an edge `<a> --> <b>`, a layout line `<a> ~~~ <b>` or exactly one node
  declaration (`id`, `id[...]`, `id(...)`, `id{...}`, `id>...]`, `id@{...}`, each with an
  optional `:::class`); a `;` or `&` outside labels and any other line is a finding. Nodes are
  the declared ones plus every id an edge, a layout line or `style`, `class` or `click` names
  (Mermaid creates those too), subgraph ids left out;
- services/server/internal/<m>/README.md: the paragraph starting with `Darf die Ports dieser
  Module nutzen:`, the sections "Tabellen" and "Aktionen (05-aktionen.md)", and in the section
  "Ereignisse und WebSocket-Ops" exactly the bullets "Dauerhaft:", "Kurzlebig:" and
  "WebSocket-Ops (Client an Server):" with their continuation lines, nested items and indented
  paragraphs.

What the docs site hides is not read: fenced code and HTML comments (a `<!--` in a code span is
text). A name that a cell, line or section the check reads states outside a code span is a
finding when it is a known name (module, register item, catalogue action, event type or op), and
raw HTML there is a finding, because the site may hide it. The static reading approximates
Python-Markdown and Mermaid; the tests render every source (Markdown with the configuration of
mkdocs.yml, the diagram with the pinned mermaid-cli) and hold the two readings equal on the real
files (docs/DECISIONS.md M1-06/6 and M1-06/11), so a form the approximation reads differently
fails `make test`.

Rules, one finding per line in the form `register-check: <path>: <rule>: <message>`:

- C1: every table of 03 is in exactly one module's `tables`, the one its `Modul:` line names;
  a register table unknown to 03, a table heading without `Modul:` line, a level-4 heading
  under "Tabellen" in another form and a table heading at another level are findings.
- C2: the module set of 03 "Module" equals the register; a module marked `keine` lists no table,
  one marked `ja` at least one.
- C3: every catalogue action is in exactly one module's `actions`; no unknown action.
- C4: every durable event type is in exactly one module's `events`, every ephemeral one in
  exactly one module's `ephemeral_events`; no unknown or misplaced type.
- C5: every op a client, agent or app sends is in exactly one module's `ws_ops`; no
  server-to-client frame and no unknown op.
- C6 to C9: the 02 tables "Modulliste" (tables), "Erlaubte Abhängigkeiten" (`depends_on`),
  "Aktionen je Modul" (actions) and "Ereignistypen und WebSocket-Ops je Modul" (durable and
  ephemeral event types, ops) have one row per register module with the register's sets.
- C10: the packages of 02 "Querschnitts-Ports" are exactly `kernel.core` plus `kernel.ports`.
- C11: the diagram declares a node for every module, and no node besides the modules and
  `kernel` and `platform` (the register sections of the same name) appears; its edges are
  exactly the `depends_on` pairs, and an edge with an end that is no module (also `kernel` or
  `platform`, whose edges are not drawn) is a finding.
- C12: each module README states the register's dependencies, tables, actions, event types and
  ops; a README under services/server/internal whose directory is no register module (other
  than `kernel`, `platform`, `wire`) is a finding.

Where a difference is fixed: names and owners come from the core contract (03, 05,
actions.yaml), the register follows it, and 02, the diagram and the READMEs follow the register.
Findings of C1 to C5 therefore name the register, unless 03 itself is incomplete.

Usage: `register_check.py [--repo PATH]`. Exit codes: 0 the sources agree, 1 findings, 2 a file,
section or column is missing or a source cannot be read (the message names it).

Example:

    $ python tools/doccheck/register_check.py
    register-check: ok (<n> modules, <t> tables, <a> actions, <e> event types, <o> ops)
"""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Iterable, Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path

import yaml

# The Makefile runs this file as a script, so Python puts only tools/doccheck on sys.path; the
# catalogue loader lives in tools/catalog (pytest finds it through its pythonpath setting instead).
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "catalog"))

from action_catalog import Catalog, CatalogError, load_catalog

REPO = Path(__file__).resolve().parents[2]
REGISTER = "services/server/architecture/modules.yaml"
CATALOGUE = "docs/spec/actions.yaml"
CHAPTER_02 = "docs/spec/02-architektur.md"
CHAPTER_03 = "docs/spec/03-datenmodell.md"
DIAGRAM = "docs/architecture/c4-3-komponenten-server.mmd"
README = "services/server/internal/{module}/README.md"

FIELDS = ("tables", "actions", "events", "ephemeral_events", "ws_ops", "depends_on")
"""Name lists of a module entry that the check reads (ADR 0050)."""

SHARED_NODES = ("kernel", "platform")
"""Diagram nodes that are no module: the register sections `kernel` and `platform`."""

KERNEL_TITLE = "Querschnitts-Ports (`internal/kernel`)"
SERVER_TO_CLIENT = "server_to_client"
DEPENDENCY_LINE = "Darf die Ports dieser Module nutzen:"


class SourceError(Exception):
    """A source file, section or column is missing, or a source cannot be read (exit code 2).

    The message starts with the repository-relative path, for example
    `docs/spec/02-architektur.md: section 'Aktionen je Modul' missing`.
    """


def finding(path: str, rule: str, message: str) -> str:
    """Format one finding line.

    Args:
        path: Repository-relative path of the file that has to change.
        rule: Rule id, `C1` to `C12`.
        message: What differs, naming the item.

    Returns:
        `register-check: <path>: <rule>: <message>`.
    """
    return f"register-check: {path}: {rule}: {message}"


# ---- register and catalogue ------------------------------------------------------------------


@dataclass(frozen=True)
class Module:
    """One entry under `modules` of the register.

    Attributes:
        name: Module name, for example `chat`.
        lists: The name lists of `FIELDS` in file order; duplicates stay, so that the rules see
            an item listed twice.
    """

    name: str
    lists: Mapping[str, tuple[str, ...]]


@dataclass(frozen=True)
class Register:
    """The parts of the register the rules compare.

    Attributes:
        modules: Modules in register order.
        kernel: Packages of `kernel.core` followed by those of `kernel.ports`.
    """

    modules: tuple[Module, ...]
    kernel: tuple[str, ...]

    @property
    def names(self) -> list[str]:
        """Module names in register order."""
        return [m.name for m in self.modules]

    def field(self, name: str) -> dict[str, tuple[str, ...]]:
        """Module name to its list `name` (one of `FIELDS`)."""
        return {m.name: m.lists[name] for m in self.modules}


def read_text(repo: Path, rel: str) -> str:
    """Read a source file as UTF-8.

    Raises:
        SourceError: The file is missing or cannot be read.
    """
    path = repo / rel
    if not path.is_file():
        raise SourceError(f"{rel}: file missing")
    try:
        return path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as exc:
        raise SourceError(f"{rel}: cannot read: {exc}") from exc


def _children(node: yaml.Node, out: list[tuple[int, str]]) -> list[yaml.Node]:
    """The child nodes of `node`; a key given twice in one mapping goes to `out`."""
    if isinstance(node, yaml.SequenceNode):
        return list(node.value)
    if not isinstance(node, yaml.MappingNode):
        return []
    seen: set[object] = set()
    for key, _ in node.value:
        if isinstance(key, yaml.ScalarNode):
            if key.value in seen:
                line = key.start_mark.line + 1
                out.append((line, f"line {line}: duplicate key {key.value!r}"))
            seen.add(key.value)
    return [value for _, value in node.value]


def _node_problems(root: yaml.Node | None) -> list[str]:
    """Duplicate keys and recursive aliases of a composed YAML document, in line order.

    `safe_load` would keep the last of two equal keys silently, and an alias inside the node it
    names makes a structure that no rule can walk. Aliases share their node, so each node is
    visited once however deeply aliases nest, and an explicit stack keeps deep nesting within
    the recursion limit. The same walk as the private `_node_findings` of
    tools/catalog/action_catalog.py, which is not part of its public interface
    (tools/catalog/README.md); M1-06 leaves tools/catalog as is.

    Returns:
        `line <n>: duplicate key '<key>'` and `line <n>: recursive alias`.
    """
    out: list[tuple[int, str]] = []
    done: set[int] = set()
    open_nodes: set[int] = set()
    stack: list[tuple[yaml.Node, bool]] = [] if root is None else [(root, False)]
    while stack:
        node, leaving = stack.pop()
        if leaving:
            open_nodes.discard(id(node))
            done.add(id(node))
        elif id(node) in open_nodes:
            line = node.start_mark.line + 1
            out.append((line, f"line {line}: recursive alias"))
        elif id(node) not in done:
            open_nodes.add(id(node))
            stack.append((node, True))
            stack += [(child, False) for child in reversed(_children(node, out))]
    return [problem for _, problem in sorted(out, key=lambda item: item[0])]


def _mapping(data: Mapping[object, object], key: str) -> Mapping[object, object]:
    value = data.get(key)
    if not isinstance(value, dict):
        raise SourceError(f"{REGISTER}: key {key!r} must be a mapping")
    return value


def _name_list(value: object, key: str) -> tuple[str, ...]:
    if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
        raise SourceError(f"{REGISTER}: key {key!r} must be a list of names")
    return tuple(value)


def load_register(repo: Path) -> Register:
    """Load the module register.

    Args:
        repo: Repository root.

    Returns:
        Modules with the lists of `FIELDS`, and the kernel packages.

    Raises:
        SourceError: The file is missing; is no YAML or holds a value Python cannot build (an
            invalid date, an integer beyond the digit limit, nesting deeper than the recursion
            limit); has a duplicate key or a recursive alias; or a field the rules read is not
            a list of names (the message names the key, for example `modules.chat.tables`).
    """
    text = read_text(repo, REGISTER)
    try:
        node = yaml.compose(text, Loader=yaml.SafeLoader)
        problems = _node_problems(node)
        # What `safe_load` returns, built from the node already composed.
        data = None if node is None or problems else yaml.SafeLoader("").construct_document(node)
    except RecursionError as exc:
        raise SourceError(f"{REGISTER}: cannot parse: nested too deeply") from exc
    except (yaml.YAMLError, ValueError) as exc:
        raise SourceError(f"{REGISTER}: cannot parse: {exc}") from exc
    if problems:
        raise SourceError(f"{REGISTER}: {problems[0]}")
    if not isinstance(data, dict):
        raise SourceError(f"{REGISTER}: not a mapping")
    kernel = _mapping(data, "kernel")
    modules: list[Module] = []
    for name, entry in _mapping(data, "modules").items():
        if not isinstance(name, str) or not isinstance(entry, dict):
            raise SourceError(f"{REGISTER}: key 'modules.{name}' must be a mapping")
        lists = {f: _name_list(entry.get(f), f"modules.{name}.{f}") for f in FIELDS}
        modules.append(Module(name, lists))
    core = _name_list(kernel.get("core"), "kernel.core")
    ports = _name_list(kernel.get("ports"), "kernel.ports")
    return Register(tuple(modules), core + ports)


def load_catalogue(repo: Path) -> Catalog:
    """Load docs/spec/actions.yaml with the loader of tools/catalog (ADR 0049).

    Raises:
        SourceError: The file is missing or the catalogue is invalid; `make catalog-check`
            reports every finding of an invalid catalogue.
    """
    path = repo / CATALOGUE
    if not path.is_file():
        raise SourceError(f"{CATALOGUE}: file missing")
    try:
        return load_catalog(path)
    except CatalogError as exc:
        first = exc.findings[0] if exc.findings else str(exc)
        raise SourceError(
            f"{CATALOGUE}: invalid catalogue ({len(exc.findings)} findings, see make "
            f"catalog-check), first: {first}"
        ) from exc


# ---- Markdown: sections and tables ---------------------------------------------------------

# As Python-Markdown reads an ATX heading (the docs site): `#` in the first column, no space
# needed after it, closing `#` dropped. A stricter reading would let `####x` drop out unseen.
_ATX = re.compile(r"^(?P<hashes>#{1,6})(?P<title>.*?)#*$")
_NUMBER = re.compile(r"^[0-9]+(?:\.[0-9]+)*\.?[ \t]+")
# A backtick fence has no backtick in its info string; "```x``` | y |" is a code span in a row.
_FENCE_OPEN = re.compile(r"^ {0,3}(?P<fence>`{3,}(?=[^`]*$)|~{3,})")
_FENCE_CLOSE = re.compile(r"^ {0,3}(?P<fence>`{3,}|~{3,})[ \t]*$")
_TICKS = re.compile(r"(?<!\\)`+")
_ROW_PIPE = re.compile(r"\\.|\|")
_END_BORDER = re.compile(r"(?<!\\)(?:\\\\)*\|$")
_CODE_TAG = re.compile(r"<code>(.*?)</code>", re.IGNORECASE)
_HTML = re.compile(r"<[A-Za-z!?/]")
# `nichts` or `keine` at the start, also emphasised (`**keine**`), as the site shows the word.
_EMPTY = re.compile(r"[*_]*(?P<word>nichts|keine)\b", re.IGNORECASE)
# A name as the register and the catalogue write it: `chat`, `user_space_states`, `thread.move`.
_NAME = re.compile(r"[a-z][a-z0-9_]*(?:\.[a-z0-9_]+)*")
_LIST_ITEM = re.compile(r"^[ \t]*(?:[*+-]|[0-9]+[.)])[ \t]")


def code_spans(line: str) -> list[tuple[int, int, str]]:
    """Code spans of one line as Python-Markdown pairs backtick runs.

    A run of N backticks opens a span that the next run of exactly N backticks closes; a run
    without a partner is text.

    Args:
        line: One line of Markdown.

    Returns:
        `(start, end, content)` of each span, `end` exclusive, content without the backticks.
    """
    runs = [(m.start(), m.end()) for m in _TICKS.finditer(line)]
    spans: list[tuple[int, int, str]] = []
    index = 0
    while index < len(runs):
        start, after = runs[index]
        size = after - start
        partner = next(
            (k for k in range(index + 1, len(runs)) if runs[k][1] - runs[k][0] == size), None
        )
        if partner is None:
            index += 1
            continue
        spans.append((start, runs[partner][1], line[after : runs[partner][0]]))
        index = partner + 1
    return spans


def _in_span(spans: Sequence[tuple[int, int, str]], position: int) -> bool:
    return any(start <= position < end for start, end, _ in spans)


def as_markdown(text: str) -> str:
    """`<code>x</code>` written as the code span `` `x` ``: the site shows both alike."""
    return _CODE_TAG.sub(lambda m: f"`{m.group(1)}`", text)


def ticked_names(text: str) -> list[str]:
    """Contents of the code spans of a text, stripped, in order (empty spans left out)."""
    names: list[str] = []
    for line in as_markdown(text).split("\n"):
        names += [content.strip() for _, _, content in code_spans(line) if content.strip()]
    return names


def without_code(text: str) -> str:
    """The text with every code span (and `<code>` element) replaced by a space."""
    out: list[str] = []
    for line in as_markdown(text).split("\n"):
        kept, last = [], 0
        for start, end, _ in code_spans(line):
            kept += [line[last:start], " "]
            last = end
        out.append("".join([*kept, line[last:]]))
    return "\n".join(out)


@dataclass(frozen=True)
class Heading:
    """An ATX heading outside fenced code.

    Attributes:
        index: Zero-based line index.
        level: Number of `#`.
        title: Title without a leading section number (`4.6 Aktionen je Modul` gives
            `Aktionen je Modul`).
    """

    index: int
    level: int
    title: str


@dataclass(frozen=True)
class Doc:
    """A Markdown file as the rules read it.

    Attributes:
        path: Repository-relative path.
        lines: Lines of the file as the site shows them: lines of fenced code blocks, fences
            included, are empty and HTML comments are blanked, so that a `#` comment in a code
            block is no heading and a commented-out row or name is not read.
        headings: Headings in file order.
    """

    path: str
    lines: tuple[str, ...]
    headings: tuple[Heading, ...]


def _visible(text: str) -> list[str]:
    lines: list[str] = []
    fence: str | None = None
    for line in text.split("\n"):
        if fence is None:
            opening = _FENCE_OPEN.match(line)
            fence = opening.group("fence") if opening else None
            lines.append("" if fence else line)
            continue
        closing = _FENCE_CLOSE.match(line)
        if (
            closing
            and closing.group("fence")[0] == fence[0]
            and len(closing["fence"]) >= len(fence)
        ):
            fence = None
        lines.append("")
    return _without_comments(lines)


def _comment_start(line: str, position: int) -> int:
    """Index of the next `<!--` from `position` that is not inside a code span, or -1."""
    spans = code_spans(line)
    found = line.find("<!--", position)
    while found >= 0 and _in_span(spans, found):
        found = line.find("<!--", found + 1)
    return found


def _without_comments(lines: Sequence[str]) -> list[str]:
    """Blank HTML comments with spaces, keeping line numbers and the text around them.

    A comment runs from `<!--` outside a code span to the next `-->`, over lines if needed,
    and to the end of the file when it is not closed (a browser hides that much).
    """
    out: list[str] = []
    inside = False
    for line in lines:
        kept: list[str] = []
        position = 0
        while position < len(line) or not kept:
            if inside:
                end = line.find("-->", position)
                stop = len(line) if end < 0 else end + 3
                kept.append(" " * (stop - position))
                inside, position = end < 0, stop
            else:
                start = _comment_start(line, position)
                if start < 0:
                    kept.append(line[position:])
                    break
                kept.append(line[position:start])
                inside, position = True, start
            if position >= len(line):
                break
        out.append("".join(kept))
    return out


def parse_doc(path: str, text: str) -> Doc:
    """Split a Markdown text into lines and headings, ignoring fenced code.

    Args:
        path: Repository-relative path, used in messages.
        text: File content.

    Returns:
        The document.
    """
    lines = _visible(text)
    headings: list[Heading] = []
    for index, line in enumerate(lines):
        m = _ATX.match(line)
        if m is not None:
            title = _NUMBER.sub("", m.group("title").strip(), count=1)
            headings.append(Heading(index, len(m.group("hashes")), title))
    return Doc(path, tuple(lines), tuple(headings))


def read_doc(repo: Path, rel: str) -> Doc:
    """Read and parse a Markdown source (see `parse_doc`).

    Raises:
        SourceError: The file is missing or cannot be read.
    """
    return parse_doc(rel, read_text(repo, rel))


def section(doc: Doc, title: str) -> range:
    """Line indexes of the body of the heading titled `title`.

    The body ends before the next heading with as many `#` or fewer.

    Raises:
        SourceError: No heading or more than one heading has this title.
    """
    found = [h for h in doc.headings if h.title == title]
    if not found:
        raise SourceError(f"{doc.path}: section {title!r} missing")
    if len(found) > 1:
        raise SourceError(f"{doc.path}: section {title!r} appears {len(found)} times")
    head = found[0]
    end = next(
        (h.index for h in doc.headings if h.index > head.index and h.level <= head.level),
        len(doc.lines),
    )
    return range(head.index + 1, end)


@dataclass(frozen=True)
class Row:
    """A data row of a Markdown table.

    Attributes:
        line: One-based line number.
        cells: Cell texts, stripped; a `\\|` does not split a cell.
    """

    line: int
    cells: tuple[str, ...]

    def cell(self, index: int) -> str:
        """Text of cell `index`, empty when the row is shorter."""
        return self.cells[index] if index < len(self.cells) else ""


@dataclass(frozen=True)
class Table:
    """The one table of a section.

    Attributes:
        header: Header cells.
        rows: Data rows after the separator row.
    """

    header: tuple[str, ...]
    rows: tuple[Row, ...]


def _cells(line: str) -> tuple[str, ...]:
    """Cells of a table row as Python-Markdown splits it.

    The border pipes go, and a pipe splits only outside code spans and when not escaped.
    """
    row = line.strip()
    row = _END_BORDER.sub("", row.removeprefix("|"))
    spans = code_spans(row)
    cells: list[str] = []
    last = 0
    for m in _ROW_PIPE.finditer(row):
        if m.group() == "|" and not _in_span(spans, m.start()):
            cells.append(row[last : m.start()])
            last = m.end()
    cells.append(row[last:])
    return tuple(cell.strip() for cell in cells)


def paragraphs(doc: Doc, indexes: range) -> list[list[int]]:
    """Runs of non-blank lines within `indexes`, as Markdown splits blocks.

    Args:
        doc: The document.
        indexes: Line indexes, usually a section body.

    Returns:
        Each paragraph as its line indexes, in order.
    """
    out: list[list[int]] = []
    current: list[int] = []
    for index in indexes:
        if doc.lines[index].strip():
            current.append(index)
        elif current:
            out.append(current)
            current = []
    return [*out, current] if current else out


def _is_table(header: str, separator: str) -> bool:
    """Whether two lines start a table as Python-Markdown decides it.

    The header splits into more than one cell (or has a border pipe), and the separator splits
    into as many cells and holds only `|`, `:`, `-` and spaces, so `|:|:|` counts as well.
    """
    cells = _cells(header)
    if len(cells) < 2 and not header.strip().startswith("|"):
        return False
    parts = _cells(separator)
    return len(parts) == len(cells) and set("".join(parts)) <= set("|:- ")


def table(doc: Doc, title: str) -> Table:
    """The only table in the section titled `title`.

    A table is a paragraph whose second line is a separator row, as Python-Markdown reads it:
    every further line of the paragraph is a row, also without a leading `|`.

    Raises:
        SourceError: The section is missing or ambiguous, or holds no table or more than one.
    """
    blocks = [
        block
        for block in paragraphs(doc, section(doc, title))
        if len(block) >= 2 and _is_table(doc.lines[block[0]], doc.lines[block[1]])
    ]
    if len(blocks) != 1:
        held = "no table" if not blocks else f"{len(blocks)} tables, expected one"
        raise SourceError(f"{doc.path}: section {title!r} holds {held}")
    rows = blocks[0]
    return Table(
        _cells(doc.lines[rows[0]]), tuple(Row(i + 1, _cells(doc.lines[i])) for i in rows[2:])
    )


def column(doc: Doc, title: str, header: tuple[str, ...], name: str) -> int:
    """Index of the column `name` of the table in section `title`.

    Raises:
        SourceError: The table has no such column.
    """
    if name not in header:
        raise SourceError(f"{doc.path}: section {title!r}: table has no column {name!r}")
    return header.index(name)


def listed_names(text: str) -> tuple[str, ...] | None:
    """Backticked names of a cell, line or paragraph.

    Returns:
        The names in order; the empty tuple when there are none and the text is empty or starts
        with the word `nichts` or `keine` (any case); `None` when the text names nothing but
        says something else, which is a finding, never silently the empty set.
    """
    names = tuple(ticked_names(text))
    if names:
        return names
    words = text.strip()
    return () if not words or _EMPTY.match(words) else None


def unticked_names(text: str, known: frozenset[str]) -> list[str]:
    """Known names a text states outside backticks.

    Args:
        text: A cell, line or section the check reads.
        known: Every name a source of the check defines (see `known_names`).

    Returns:
        Sorted known names left after removing the code spans: written as plain text or as
        `<code>...</code>`, they would show on the site but drop out of the comparison.
    """
    return sorted({n for n in _NAME.findall(without_code(text)) if n in known})


@dataclass(frozen=True)
class ModuleTable:
    """The table of a section with one row per module.

    Attributes:
        title: Section title.
        table: The table.
        rows: First row of each module, by module name.
        findings: Rows whose first cell names no single module, and modules with several rows.
    """

    title: str
    table: Table
    rows: Mapping[str, Row]
    findings: tuple[str, ...]


def module_table(doc: Doc, title: str, rule: str) -> ModuleTable:
    """Read the table of section `title` whose first column names one module per row.

    Raises:
        SourceError: See `table`.
    """
    parsed = table(doc, title)
    found: dict[str, list[Row]] = {}
    findings: list[str] = []
    for row in parsed.rows:
        names = ticked_names(row.cell(0))
        if len(names) != 1:
            findings.append(
                finding(doc.path, rule, f"line {row.line}: first cell names no single module")
            )
            continue
        found.setdefault(names[0], []).append(row)
    findings += [
        finding(doc.path, rule, f"module {module} has {len(rows)} rows")
        for module, rows in found.items()
        if len(rows) > 1
    ]
    first = {module: rows[0] for module, rows in found.items()}
    return ModuleTable(title, parsed, first, tuple(findings))


# ---- comparisons ---------------------------------------------------------------------------


def compare_sets(
    path: str, rule: str, prefix: str, documented: Iterable[str], registered: Iterable[str]
) -> list[str]:
    """Findings for names the register has and the document lacks, and the reverse.

    Args:
        path: File the findings name.
        rule: Rule id.
        prefix: Start of each message naming module and kind, for example `module chat: action`.
        documented: Names the document states.
        registered: Names the register states.

    Returns:
        `<prefix> <name> missing` and `<prefix> <name> not in the register`, sorted by name.
    """
    doc_set, reg_set = set(documented), set(registered)
    missing = [finding(path, rule, f"{prefix} {n} missing") for n in sorted(reg_set - doc_set)]
    extra = [
        finding(path, rule, f"{prefix} {n} not in the register") for n in sorted(doc_set - reg_set)
    ]
    return missing + extra


def _modules_by_item(listed: Mapping[str, Sequence[str]]) -> dict[str, list[str]]:
    owners: dict[str, list[str]] = {}
    for module, items in listed.items():
        for item in items:
            owners.setdefault(item, []).append(module)
    return owners


def exactly_once(
    rule: str, noun: str, field: str, expected: Iterable[str], listed: Mapping[str, Sequence[str]]
) -> list[str]:
    """Findings for expected items that are in no module's list or in more than one.

    Args:
        rule: Rule id.
        noun: Kind of item in messages, for example `durable event`.
        field: Register field, for example `events`.
        expected: Items of the core contract.
        listed: Module name to its register list `field`.

    Returns:
        Findings on the register.
    """
    owners = _modules_by_item(listed)
    out: list[str] = []
    for item in expected:
        modules = sorted(owners.get(item, []))
        if not modules:
            out.append(finding(REGISTER, rule, f"{noun} {item} is in no module's {field}"))
        elif len(modules) > 1:
            where = ", ".join(modules)
            out.append(finding(REGISTER, rule, f"{noun} {item} is in {field} of modules {where}"))
    return out


def foreign_items(
    rule: str,
    field: str,
    listed: Mapping[str, Sequence[str]],
    wanted: set[str],
    explain: Mapping[str, str],
    unknown: str,
) -> list[str]:
    """Findings for register items that do not belong in the list `field`.

    Args:
        rule: Rule id.
        field: Register field.
        listed: Module name to its register list `field`.
        wanted: Items that belong in this field.
        explain: Why a known item is misplaced, by item (for example `an ephemeral event type`).
        unknown: Text for an item the source does not know at all.

    Returns:
        `module <m>: <field> names <item>, <reason>` on the register.
    """
    return [
        finding(REGISTER, rule, f"module {m}: {field} names {item}, {explain.get(item, unknown)}")
        for m, items in listed.items()
        for item in items
        if item not in wanted
    ]


# ---- C1, C2: chapter 03 --------------------------------------------------------------------

_TABLE_TITLE = re.compile(r"`(?P<name>[a-z][a-z0-9_]*)`")
_OWNER = re.compile(r"Modul:\s+`(?P<module>[a-z][a-z0-9_]*)`")


def _owner(lines: Sequence[str]) -> str | None:
    """Module of the first `Modul: `<m>`` before the first table row, `None` without one."""
    text: list[str] = []
    for line in lines:
        if line.lstrip().startswith("|"):
            break
        text.append(line)
    owner = _OWNER.search(as_markdown("\n".join(text)))
    return owner["module"] if owner else None


def tables_of_03(doc: Doc) -> tuple[dict[str, str | None], list[str]]:
    """Module tables of 03 with the owner their `Modul:` line names.

    Returns:
        Table name to owner (`None` without a `Modul:` line before the first table row), and
        findings for a missing owner, a table with several headings, a level-4 heading that is
        not one backticked table name, and a heading at level 3 that is one, or at level 5 or 6
        that names one.

    Raises:
        SourceError: Section "Tabellen" is missing or ambiguous.
    """
    body = section(doc, "Tabellen")
    heads = [h for h in doc.headings if h.index in body]
    owners: dict[str, list[str | None]] = {}
    findings: list[str] = []
    technical = False
    for pos, head in enumerate(heads):
        name = _TABLE_TITLE.fullmatch(head.title)
        if head.level <= 3:
            technical = head.title == "Technische Tabellen"
        hashes = "#" * head.level
        # A table one level off, or a deeper heading, would drop out of the comparison unseen.
        if (head.level == 3 and head.title.startswith("`")) or (
            head.level > 4 and ticked_names(head.title)
        ):
            message = f"heading '{hashes} {head.title}' names a table, but table headings are"
            findings.append(finding(doc.path, "C1", f"{message} '#### `<table>`'"))
        elif head.level > 4:
            message = f"heading '{hashes} {head.title}' is deeper than a table heading"
            findings.append(finding(doc.path, "C1", f"{message} '#### `<table>`'"))
        elif head.level == 3 and not technical:
            end = heads[pos + 1].index if pos + 1 < len(heads) else body.stop
            if _owner(doc.lines[head.index + 1 : end]) is not None:
                message = f"heading '### {head.title}' has a 'Modul:' line, but table headings are"
                findings.append(finding(doc.path, "C1", f"{message} '#### `<table>`'"))
        if head.level != 4 or technical:
            continue
        if name is None:
            # A table heading in another form would drop the table out of the comparison.
            message = f"heading '#### {head.title}' is not of the form '#### `<table>`'"
            findings.append(finding(doc.path, "C1", message))
            continue
        end = heads[pos + 1].index if pos + 1 < len(heads) else body.stop
        owners.setdefault(name["name"], []).append(_owner(doc.lines[head.index + 1 : end]))
    findings += [
        finding(doc.path, "C1", f"table {t} has {len(found)} headings")
        for t, found in owners.items()
        if len(found) > 1
    ]
    findings += [
        finding(doc.path, "C1", f"table {t} has no line 'Modul: `<module>`'")
        for t, found in owners.items()
        if found[0] is None
    ]
    return {t: found[0] for t, found in owners.items()}, findings


def rule_c1(doc: Doc, register: Register) -> list[str]:
    """C1: table owners of 03 against the register's `tables`."""
    owners, out = tables_of_03(doc)
    listed = register.field("tables")
    out += exactly_once("C1", "table", "tables", owners, listed)
    where = _modules_by_item(listed)
    for name, owner in owners.items():
        modules = where.get(name, [])
        if owner is not None and len(modules) == 1 and modules[0] != owner:
            message = f"table {name}: register module {modules[0]}, 03 module {owner}"
            out.append(finding(REGISTER, "C1", message))
    out += foreign_items("C1", "tables", listed, set(owners), {}, "unknown to 03")
    return out


def rule_c2(doc: Doc, register: Register) -> list[str]:
    """C2: module set and the column "Tabellen" of 03 "Module" against the register."""
    parsed = module_table(doc, "Module", "C2")
    index = column(doc, "Module", parsed.table.header, "Tabellen")
    out = list(parsed.findings)
    out += [
        finding(REGISTER, "C2", f'module {m} is not in 03 "Module"')
        for m in register.names
        if m not in parsed.rows
    ]
    out += [
        finding(REGISTER, "C2", f'module {m} of 03 "Module" is not in the register')
        for m in parsed.rows
        if m not in register.names
    ]
    for module in register.modules:
        row = parsed.rows.get(module.name)
        if row is None:
            continue
        value, tables = row.cell(index), module.lists["tables"]
        if value == "keine" and tables:
            message = f'03 "Module" says keine, the register lists {", ".join(tables)}'
            out.append(finding(REGISTER, "C2", f"module {module.name}: {message}"))
        elif value == "ja" and not tables:
            message = '03 "Module" says ja, the register lists no table'
            out.append(finding(REGISTER, "C2", f"module {module.name}: {message}"))
        elif value not in ("ja", "keine"):
            message = f"column 'Tabellen' is {value!r}, expected ja or keine"
            out.append(finding(doc.path, "C2", f"module {module.name}: {message}"))
    return out


# ---- C3 to C5: catalogue -------------------------------------------------------------------

_UNKNOWN = "unknown to the catalogue"


def rule_c3(catalog: Catalog, register: Register) -> list[str]:
    """C3: catalogue actions against the register's `actions`."""
    expected = [a.id for a in catalog.actions]
    listed = register.field("actions")
    out = exactly_once("C3", "action", "actions", expected, listed)
    return out + foreign_items("C3", "actions", listed, set(expected), {}, _UNKNOWN)


def rule_c4(catalog: Catalog, register: Register) -> list[str]:
    """C4: durable and ephemeral event types against `events` and `ephemeral_events`."""
    durable = [e.type for e in catalog.events if e.kind == "durable"]
    ephemeral = [e.type for e in catalog.events if e.kind == "ephemeral"]
    events, short = register.field("events"), register.field("ephemeral_events")
    out = exactly_once("C4", "durable event", "events", durable, events)
    out += exactly_once("C4", "ephemeral event", "ephemeral_events", ephemeral, short)
    as_ephemeral = dict.fromkeys(ephemeral, "an ephemeral event type")
    as_durable = dict.fromkeys(durable, "a durable event type")
    out += foreign_items("C4", "events", events, set(durable), as_ephemeral, _UNKNOWN)
    out += foreign_items("C4", "ephemeral_events", short, set(ephemeral), as_durable, _UNKNOWN)
    return out


def rule_c5(catalog: Catalog, register: Register) -> list[str]:
    """C5: ops a client, agent or app sends against `ws_ops`; no server-to-client frame."""
    inbound = [o.op for o in catalog.ws_ops if o.direction != SERVER_TO_CLIENT]
    frames = dict.fromkeys(
        (o.op for o in catalog.ws_ops if o.direction == SERVER_TO_CLIENT),
        "a server-to-client frame",
    )
    listed = register.field("ws_ops")
    out = exactly_once("C5", "op", "ws_ops", inbound, listed)
    return out + foreign_items("C5", "ws_ops", listed, set(inbound), frames, _UNKNOWN)


# ---- C6 to C10: chapter 02 -----------------------------------------------------------------


@dataclass(frozen=True)
class Column:
    """A machine column of a 02 table.

    Attributes:
        header: Column title in 02.
        kind: Item kind in messages.
        field: Register field it repeats.
    """

    header: str
    kind: str
    field: str


CHAPTER_02_TABLES: tuple[tuple[str, str, tuple[Column, ...]], ...] = (
    ("C6", "Modulliste", (Column("Besitzt Tabellen", "table", "tables"),)),
    ("C7", "Erlaubte Abhängigkeiten", (Column("Darf nutzen", "dependency", "depends_on"),)),
    ("C8", "Aktionen je Modul", (Column("Aktionen", "action", "actions"),)),
    (
        "C9",
        "Ereignistypen und WebSocket-Ops je Modul",
        (
            Column("Dauerhafte Ereignisse", "durable event", "events"),
            Column("Kurzlebige Ereignisse", "ephemeral event", "ephemeral_events"),
            Column("WebSocket-Ops (Client an Server)", "op", "ws_ops"),
        ),
    ),
)
"""Rule, section title and machine columns of the module tables of 02; other columns are prose."""


@dataclass(frozen=True)
class Place:
    """Where a listing text stands, for findings.

    Attributes:
        path: File the findings name.
        rule: Rule id.
        where: What the text is, for example `cell 'Aktionen'`.
        empty_ok: Whether an empty text is the empty set (a table cell) or a finding (a README
            line or section, which must say `keine` when it lists nothing).
    """

    path: str
    rule: str
    where: str
    empty_ok: bool


def stated_findings(
    place: Place, text: str, module: Module, col: Column, known: frozenset[str]
) -> list[str]:
    """Findings for one text (cell, README line or section) that lists the names of a field.

    Args:
        place: Where the text stands.
        text: The text as the site shows it.
        module: Module whose register list the text repeats.
        col: Kind and register field of the names.
        known: Every name a source defines (`known_names`).

    Returns:
        A finding for a text that names nothing and is not `nichts` or `keine` (nor empty where
        that is allowed) or that starts with `nichts` or `keine` and still names something, one
        per known name outside backticks, one for raw HTML, and the differences to the register
        list.
    """
    prefix = f"module {module.name}"
    out = [
        finding(place.path, place.rule, f"{prefix}: {place.where} names {name} without backticks")
        for name in unticked_names(text, known)
    ]
    if _HTML.search(without_code(text)):
        message = f"{place.where} holds raw HTML, which the site may hide"
        out.append(finding(place.path, place.rule, f"{prefix}: {message}"))
    names = listed_names(text) if text.strip() or place.empty_ok else None
    if names is None:
        allowed = "empty, nichts or keine" if place.empty_ok else "nichts or keine"
        message = f"{place.where} names nothing and is not {allowed}"
        return [*out, finding(place.path, place.rule, f"{prefix}: {message}")]
    empty = _EMPTY.match(text.strip())
    if names and empty:
        # "nichts (früher `x`)": the site says nothing, the names would say x.
        message = f"{place.where} starts with {empty['word']} but names {', '.join(names)}"
        return [*out, finding(place.path, place.rule, f"{prefix}: {message}")]
    kind = f"{prefix}: {col.kind}"
    return out + compare_sets(place.path, place.rule, kind, names, module.lists[col.field])


def rule_02_table(
    doc: Doc,
    register: Register,
    rule: str,
    title: str,
    columns: tuple[Column, ...],
    known: frozenset[str],
) -> list[str]:
    """C6 to C9: one row per register module, machine columns equal to the register.

    Args:
        doc: Chapter 02.
        register: The register.
        rule: Rule id of the table.
        title: Section title.
        columns: Machine columns of the table.
        known: Every name a source defines (`known_names`).

    Returns:
        Findings on chapter 02.

    Raises:
        SourceError: The section, its table or a column is missing.
    """
    parsed = module_table(doc, title, rule)
    indexes = [column(doc, title, parsed.table.header, c.header) for c in columns]
    out = list(parsed.findings)
    out += compare_sets(doc.path, rule, "module", parsed.rows, register.names)
    for module in register.modules:
        row = parsed.rows.get(module.name)
        if row is None:
            continue
        for col, index in zip(columns, indexes, strict=True):
            place = Place(doc.path, rule, f"cell {col.header!r}", empty_ok=True)
            out += stated_findings(place, row.cell(index), module, col, known)
    return out


def rule_c10(doc: Doc, register: Register) -> list[str]:
    """C10: packages of 02 "Querschnitts-Ports" against `kernel.core` plus `kernel.ports`."""
    parsed = table(doc, KERNEL_TITLE)
    index = column(doc, KERNEL_TITLE, parsed.header, "Paket")
    out: list[str] = []
    packages: list[str] = []
    for row in parsed.rows:
        names = ticked_names(row.cell(index))
        if len(names) != 1:
            out.append(
                finding(doc.path, "C10", f"line {row.line}: cell 'Paket' names no single package")
            )
        elif not names[0].startswith("kernel/") or names[0] == "kernel/":
            out.append(
                finding(doc.path, "C10", f"row names {names[0]!r}, expected 'kernel/<package>'")
            )
        else:
            packages.append(names[0].removeprefix("kernel/"))
    out += [
        finding(doc.path, "C10", f"kernel package {p} has {packages.count(p)} rows")
        for p in sorted(set(packages))
        if packages.count(p) > 1
    ]
    return out + compare_sets(doc.path, "C10", "kernel package", packages, register.kernel)


# ---- C11: diagram --------------------------------------------------------------------------

# One node declaration per line, labels already emptied: `id`, `id[""]`, `id("")`, `id{}`,
# `id>""]`, `id@{ ... }`, each optionally with `:::class`. Nothing else may follow.
_NODE = re.compile(r"^\s*([a-z][a-z0-9_]*)(?:\s*[\[({>@][^&;]*[\])}])?(?::::[A-Za-z0-9_-]+)?\s*$")
_KEYWORD = re.compile(
    r"^\s*(?P<word>flowchart|graph|subgraph|end|classDef|class|style|linkStyle|click|direction)"
    r"\b(?P<rest>.*)$"
)
_FIRST_ID = re.compile(r"^\s*([A-Za-z0-9_,-]+)")
_EDGE = re.compile(r"^\s*([a-z][a-z0-9_]*)\s*-->\s*([a-z][a-z0-9_]*)\s*$")
_LAYOUT = re.compile(r"^\s*([A-Za-z0-9_]+)\s*~~~\s*([A-Za-z0-9_]+)\s*$")
_LINK = re.compile(r"--|==|-\.|~~|<-")
_QUOTED = re.compile(r'"[^"]*"')
# Mermaid ends a statement at `;` and joins nodes with `&`: a second statement could hide.
_SECOND_STATEMENT = re.compile(r"[;&]")


@dataclass(frozen=True)
class Diagram:
    """Nodes and edges of the component diagram as the check reads it.

    Attributes:
        nodes: Declared node ids.
        mentioned: Every node id Mermaid creates from the file: declared nodes, ends of edges and
            layout lines, ids that `style`, `class` or `click` name; subgraph ids left out.
        edges: `(line, from, to)` of every `a --> b` line.
        layout: `(from, to)` of every layout line `a ~~~ b`.
        findings: Lines the check cannot read: more than one statement, a link in another form,
            or a line that is no single node declaration, edge, layout line, keyword line or
            comment.
    """

    nodes: frozenset[str]
    mentioned: frozenset[str]
    edges: tuple[tuple[int, str, str], ...]
    layout: frozenset[tuple[str, str]]
    findings: tuple[str, ...]


def _keyword_ids(word: str, rest: str) -> list[str]:
    """Ids a keyword line names: the subgraph id, or the nodes `style`, `class`, `click` touch."""
    first = _FIRST_ID.match(rest)
    if first is None or word not in ("subgraph", "style", "class", "click"):
        return []
    return [i for i in first.group(1).split(",") if i] if word == "class" else [first.group(1)]


def parse_diagram(text: str) -> Diagram:
    """Read nodes and edges of a Mermaid flowchart.

    Front matter between leading `---` lines, blank lines and `%%` comments are skipped. Every
    other line is one statement: a keyword line (`flowchart`, `subgraph`, `end`, `classDef`,
    `class`, `style`, `linkStyle`, `click`, `direction`), an edge `a --> b`, a layout line
    `a ~~~ b` or exactly one node declaration. A `;` or `&` outside quoted labels, another link
    form or any other line is a finding, so that neither an edge nor a node can hide in a syntax
    the check does not read. That this reading equals Mermaid's on the real diagram, a test
    holds with the pinned mermaid-cli (docs/DECISIONS.md M1-06/6).
    """
    lines = text.split("\n")
    start = 0
    if lines and lines[0].strip() == "---":
        start = next((i + 1 for i in range(1, len(lines)) if lines[i].strip() == "---"), len(lines))
    nodes: set[str] = set()
    named: set[str] = set()
    subgraphs: set[str] = set()
    edges: list[tuple[int, str, str]] = []
    layout: set[tuple[str, str]] = set()
    bad: list[str] = []
    for index in range(start, len(lines)):
        line, number = lines[index], index + 1
        bare = _QUOTED.sub('""', line)
        if not line.strip() or line.lstrip().startswith("%%"):
            continue
        if _SECOND_STATEMENT.search(bare):
            message = f"line {number}: more than one statement: {line.strip()}"
            bad.append(finding(DIAGRAM, "C11", message))
        elif (keyword := _KEYWORD.match(bare)) is not None:
            ids = _keyword_ids(keyword["word"], keyword["rest"])
            (subgraphs if keyword["word"] == "subgraph" else named).update(ids)
        elif (edge := _EDGE.match(line)) is not None:
            edges.append((number, edge.group(1), edge.group(2)))
        elif (link := _LAYOUT.match(line)) is not None:
            layout.add((link.group(1), link.group(2)))
        elif _LINK.search(bare):
            message = f"line {number}: link is not of the form 'a --> b': {line.strip()}"
            bad.append(finding(DIAGRAM, "C11", message))
        elif (node := _NODE.match(bare)) is not None:
            nodes.add(node.group(1))
        else:
            message = f"line {number}: not a node, edge or layout line: {line.strip()}"
            bad.append(finding(DIAGRAM, "C11", message))
    ends = {end for _, a, b in edges for end in (a, b)} | {end for pair in layout for end in pair}
    mentioned = (nodes | named | ends) - subgraphs
    return Diagram(
        frozenset(nodes), frozenset(mentioned), tuple(edges), frozenset(layout), tuple(bad)
    )


def rule_c11(diagram: Diagram, register: Register) -> list[str]:
    """C11: diagram nodes and module edges against the register modules and `depends_on`.

    Args:
        diagram: The diagram as `parse_diagram` reads it.
        register: The register.

    Returns:
        Findings on the diagram.
    """
    modules = set(register.names)
    allowed = modules | set(SHARED_NODES)
    out = list(diagram.findings)
    out += [
        finding(DIAGRAM, "C11", f"module {m} has no node") for m in sorted(modules - diagram.nodes)
    ]
    out += [
        finding(DIAGRAM, "C11", f"node {n} is not a register module")
        for n in sorted(diagram.mentioned - allowed)
    ]
    # Every edge is a module edge: kernel and platform edges are not drawn (diagram comment).
    drawn: set[tuple[str, str]] = set()
    for _, source, target in diagram.edges:
        unknown = [end for end in (source, target) if end not in modules]
        for end in unknown:
            message = f"edge {source} --> {target}: {end} is not a register module"
            out.append(finding(DIAGRAM, "C11", message))
        if not unknown:
            drawn.add((source, target))
    wanted = {(m.name, d) for m in register.modules for d in m.lists["depends_on"]}
    out += [finding(DIAGRAM, "C11", f"edge {a} --> {b} missing") for a, b in sorted(wanted - drawn)]
    out += [
        finding(DIAGRAM, "C11", f"edge {a} --> {b} not in the register")
        for a, b in sorted(drawn - wanted)
    ]
    return out


# ---- C12: module READMEs -------------------------------------------------------------------

README_BULLETS = (
    Column("- Dauerhaft:", "durable event", "events"),
    Column("- Kurzlebig:", "ephemeral event", "ephemeral_events"),
    Column("- WebSocket-Ops (Client an Server):", "op", "ws_ops"),
)
"""Bullets of the README section "Ereignisse und WebSocket-Ops" and the fields they repeat."""

README_SECTIONS = (
    Column("Tabellen", "table", "tables"),
    Column("Aktionen (05-aktionen.md)", "action", "actions"),
)
"""README sections whose names, in every paragraph, are the names of a field."""


def _indent(line: str) -> int:
    return len(line.expandtabs(4)) - len(line.expandtabs(4).lstrip())


def _continued(doc: Doc, start: int, stop: int) -> tuple[str, int]:
    """Line `start` with its continuation lines, as Markdown joins them.

    A paragraph runs to a blank line or heading; list-like lines right after it still belong to
    it, because Python-Markdown needs a blank line before a list. A list item also takes deeper
    indented lines, nested items and, after a blank line, indented paragraphs; it ends at an item
    of the same or a smaller indent.

    Returns:
        The joined text and the index after its last line.
    """
    item = _LIST_ITEM.match(doc.lines[start]) is not None
    indent = _indent(doc.lines[start])
    text = [doc.lines[start]]
    index = start + 1
    while index < stop:
        line = doc.lines[index]
        if not line.strip():
            following = next((i for i in range(index, stop) if doc.lines[i].strip()), None)
            if not item or following is None or _indent(doc.lines[following]) < indent + 2:
                break
            index = following
            continue
        if _ATX.match(line) or (item and _LIST_ITEM.match(line) and _indent(line) <= indent):
            break
        text.append(line.strip())
        index += 1
    return " ".join(text), index


def section_text(doc: Doc, title: str) -> str:
    """The section titled `title` as one line per paragraph.

    A code span may run over a line break inside a paragraph, as Python-Markdown pairs the
    backticks there, but never over a paragraph.

    Raises:
        SourceError: The section is missing or ambiguous.
    """
    blocks = paragraphs(doc, section(doc, title))
    return "\n".join(" ".join(doc.lines[i].strip() for i in block) for block in blocks)


def _readme_line(
    doc: Doc, module: Module, starts: Sequence[int], stop: int, col: Column, known: frozenset[str]
) -> tuple[list[str], range]:
    """The one line starting with `col.header`, with its continuation, lists `col.field`.

    Returns:
        The findings and the line indexes the line and its continuation take.
    """
    if len(starts) != 1:
        message = f"line {col.header!r} appears {len(starts)} times, expected once"
        return [finding(doc.path, "C12", f"module {module.name}: {message}")], range(0)
    text, end = _continued(doc, starts[0], stop)
    place = Place(doc.path, "C12", f"line {col.header!r}", empty_ok=False)
    found = stated_findings(place, text[len(col.header) :], module, col, known)
    return found, range(starts[0], end)


def rule_c12(doc: Doc, module: Module, known: frozenset[str]) -> list[str]:
    """C12: one module README against the module's register entry.

    Args:
        doc: The README of the module.
        module: The module's register entry.
        known: Every name a source defines (`known_names`).

    Returns:
        Findings on the README.

    Raises:
        SourceError: A README section the rule reads is missing or ambiguous.
    """
    dependency = Column(DEPENDENCY_LINE, "dependency", "depends_on")
    starts = [i for i, x in enumerate(doc.lines) if x.startswith(DEPENDENCY_LINE)]
    out, _ = _readme_line(doc, module, starts, len(doc.lines), dependency, known)
    for col in README_SECTIONS:
        text = section_text(doc, col.header)
        place = Place(doc.path, "C12", f"section {col.header!r}", empty_ok=False)
        out += stated_findings(place, text, module, col, known)
    title = "Ereignisse und WebSocket-Ops"
    body = section(doc, title)
    taken: set[int] = set()
    for col in README_BULLETS:
        starts = [i for i in body if doc.lines[i].startswith(col.header)]
        found, lines = _readme_line(doc, module, starts, body.stop, col, known)
        out += found
        taken.update(lines)
    # Only the three bullets: another line there could state names the check never compares.
    out += [
        finding(
            doc.path,
            "C12",
            f"module {module.name}: section {title!r}: line {i + 1} belongs to none of the three "
            "bullets",
        )
        for i in body
        if doc.lines[i].strip() and i not in taken
    ]
    return out


NON_MODULE_DIRS = ("kernel", "platform", "wire")
"""Directories under services/server/internal that are register sections, not modules."""


def stray_readmes(repo: Path, register: Register) -> list[str]:
    """Findings for module READMEs whose directory is no register module.

    Args:
        repo: Repository root.
        register: The register.

    Returns:
        One C12 finding per `services/server/internal/<dir>/README.md` whose directory is
        neither a register module nor one of `NON_MODULE_DIRS`.
    """
    base = repo / Path(README).parent.parent
    found = sorted(p.parent.name for p in base.glob("*/README.md")) if base.is_dir() else []
    return [
        finding(
            README.format(module=name),
            "C12",
            f"README of module {name}, which is not in the register",
        )
        for name in found
        if name not in register.names and name not in NON_MODULE_DIRS
    ]


# ---- run -----------------------------------------------------------------------------------


@dataclass(frozen=True)
class Sources:
    """Every source of the check, read once.

    Attributes:
        register: The module register.
        catalog: The action catalogue.
        chapter_02: docs/spec/02-architektur.md.
        chapter_03: docs/spec/03-datenmodell.md.
        diagram: The component diagram.
        readmes: README of each register module, by module name.
        stray: Findings for READMEs of directories that are no register module.
    """

    register: Register
    catalog: Catalog
    chapter_02: Doc
    chapter_03: Doc
    diagram: Diagram
    readmes: Mapping[str, Doc]
    stray: tuple[str, ...]


def load_sources(repo: Path) -> Sources:
    """Read every source of the check.

    Args:
        repo: Repository root.

    Raises:
        SourceError: A file is missing or cannot be read; the register or the catalogue is
            invalid.
    """
    register = load_register(repo)
    return Sources(
        register=register,
        catalog=load_catalogue(repo),
        chapter_02=read_doc(repo, CHAPTER_02),
        chapter_03=read_doc(repo, CHAPTER_03),
        diagram=parse_diagram(read_text(repo, DIAGRAM)),
        readmes={m: read_doc(repo, README.format(module=m)) for m in register.names},
        stray=tuple(stray_readmes(repo, register)),
    )


def known_names(register: Register, catalog: Catalog) -> frozenset[str]:
    """Every name a source defines: modules, register items, actions, event types and ops.

    Args:
        register: The register.
        catalog: The catalogue.

    Returns:
        The names a machine cell or README list may only state in backticks.
    """
    names = set(register.names)
    for module in register.modules:
        for field in FIELDS:
            names.update(module.lists[field])
    names.update(a.id for a in catalog.actions)
    names.update(e.type for e in catalog.events)
    names.update(o.op for o in catalog.ws_ops)
    return frozenset(names)


def check(sources: Sources) -> list[str]:
    """Run C1 to C12.

    Args:
        sources: Every source, read by `load_sources`.

    Returns:
        Findings, sorted and without repeats; empty when every source agrees.

    Raises:
        SourceError: A section or column a rule reads is missing or ambiguous.
    """
    reg, cat = sources.register, sources.catalog
    known = known_names(reg, cat)
    out = rule_c1(sources.chapter_03, reg) + rule_c2(sources.chapter_03, reg)
    out += rule_c3(cat, reg) + rule_c4(cat, reg) + rule_c5(cat, reg)
    for rule, title, columns in CHAPTER_02_TABLES:
        out += rule_02_table(sources.chapter_02, reg, rule, title, columns, known)
    out += rule_c10(sources.chapter_02, reg) + rule_c11(sources.diagram, reg)
    for module in reg.modules:
        out += rule_c12(sources.readmes[module.name], module, known)
    out += sources.stray
    return sorted(set(out))


def summary(register: Register) -> str:
    """Counts for the success line: modules, tables, actions, event types and ops."""
    total = {f: sum(len(m.lists[f]) for m in register.modules) for f in FIELDS}
    events = total["events"] + total["ephemeral_events"]
    return (
        f"{len(register.modules)} modules, {total['tables']} tables, {total['actions']} actions, "
        f"{events} event types, {total['ws_ops']} ops"
    )


def main(argv: Sequence[str] | None = None) -> int:
    """Command line: check the repository and print the result.

    Args:
        argv: Arguments without the program name; `None` reads `sys.argv`.

    Returns:
        0 when the sources agree (one `ok` line on stdout), 1 with one finding per line on
        stderr, 2 when a source is missing or unreadable (one line on stderr).
    """
    parser = argparse.ArgumentParser(
        prog="register_check.py",
        description="Check the module register against 03, actions.yaml, 02, the component "
        "diagram and the module READMEs (make register-check).",
    )
    parser.add_argument(
        "--repo", type=Path, default=REPO, help="repository root (default: this repository)"
    )
    args = parser.parse_args(argv)
    try:
        sources = load_sources(args.repo)
        findings = check(sources)
    except SourceError as exc:
        # One line, also for a parser message that spans several (YAML marks the column).
        print(f"register-check: {' '.join(str(exc).split())}", file=sys.stderr)
        return 2
    if findings:
        print("\n".join(findings), file=sys.stderr)
        return 1
    print(f"register-check: ok ({summary(sources.register)})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
