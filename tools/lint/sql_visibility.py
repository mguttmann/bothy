# SPDX-License-Identifier: Apache-2.0
"""Visibility predicate of reading sqlc queries on content tables (gate `sql-visibility`, AZ-P12).

Chapter 07 "Query pattern" rule 1: every query over content, threads, channels, spaces, runs,
files or mentions carries `channel_id = ANY($visible)` or `space_id = ANY($visible_spaces)`,
unless it is on the reviewed exception list. This tool reads every sqlc query file
`services/server/internal/<module>/adapters/postgres/queries/*.sql`, takes each block that
starts with a header `-- name: <Name> :<cmd>` as one query `<module>.<Name>`, and checks the
reads among them:

- a read starts with `SELECT` or `WITH` (after comments) and has no `INSERT`, `UPDATE` or
  `DELETE` at the top level (`FOR UPDATE` and `FOR NO KEY UPDATE` are row locks of a read);
- a read that names a content table after `FROM` or `JOIN` (with or without alias) passes when
  it has a predicate `(alias.)?(channel_id|space_id) = ANY(<param>)` whose argument is a sqlc
  parameter (`$n`, `@name`, `sqlc.arg(name)`, `sqlc.narg(name)`) with an optional cast; on the
  tables `spaces` and `channels` their key `id` is that column (ruling M1-70/2). A subquery
  inside `ANY(...)` does not count (03 "Principles" item 12);
- otherwise the query must stand under `exceptions` of the configuration with a category of
  CATEGORIES and a non-empty reason.

The configuration (CONFIG) also fails on an exception whose query no longer exists, an unknown
category, a duplicate entry and a content table that is no table heading of the data model
chapter it names (`data_model`). Comments and string literals never count.

Usage: python tools/lint/sql_visibility.py [--repo DIR]
Output: one line per finding `file:line: <module>.<Name>: <rule>`.
Exit codes: 0 clean, 1 findings, 2 usage error (unreadable or malformed configuration).
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

import yaml

CONFIG = "tools/lint/sql_visibility.yaml"
QUERY_GLOB = "services/server/internal/*/adapters/postgres/queries/*.sql"
CATEGORIES = frozenset({"authorize_resolution", "after_authorize", "system_job", "log_tail"})
# Tables whose key `id` is the channel or space the predicate names.
KEYED_TABLES = frozenset({"spaces", "channels"})

_HEADER = re.compile(r"^--\s*name:\s*(?P<name>\w+)\s+:(?P<cmd>\w+)")
_HEADING = re.compile(r"^#{2,6}\s+`(?P<table>[a-z0-9_]+)`\s*$")
_PARAM = r"(?:\$\d+|@\w+|sqlc\s*\.\s*n?arg\s*\(\s*\w+\s*\))"
_CAST = r"(?:\s*::\s*\w+(?:\s*\[\s*\])?)?"
_PREDICATE = re.compile(
    rf"(?:(?<![\w.])(?P<alias>\w+)\s*\.\s*|(?<![\w.]))(?P<col>channel_id|space_id|id)\s*=\s*"
    rf"ANY\s*\(\s*{_PARAM}{_CAST}\s*\)",
    re.IGNORECASE,
)
_TOKEN = re.compile(r"\w+|[(),.;]|\S")
# Words that end a table item instead of naming its alias.
_CLAUSE_WORDS = frozenset(
    {
        "where", "join", "inner", "left", "right", "full", "cross", "natural", "on", "using",
        "group", "order", "limit", "offset", "having", "window", "union", "intersect", "except",
        "for", "returning", "lateral", "tablesample", "fetch", "set", "values", "select",
    }
)  # fmt: skip
_WRITES = frozenset({"insert", "update", "delete"})


class UsageError(Exception):
    """A missing or malformed configuration or repository (exit code 2)."""


@dataclass(frozen=True)
class Finding:
    """One violation of the check.

    It names the file relative to the repository, the 1-based line, the query (or
    configuration entry) it concerns and the rule id.
    """

    file: str
    line: int
    query: str
    rule: str

    def __str__(self) -> str:
        """Return the finding as one report line."""
        return f"{self.file}:{self.line}: {self.query}: {self.rule}"


@dataclass(frozen=True)
class Query:
    """One sqlc query: its identifier `<module>.<Name>`, file, header line and SQL text."""

    ident: str
    file: str
    line: int
    sql: str


@dataclass(frozen=True)
class ExceptionEntry:
    """One entry of the exception list with the line it starts on in CONFIG."""

    query: str
    category: str
    reason: str
    line: int


@dataclass(frozen=True)
class Config:
    """The parsed configuration: data model chapter, content tables and exceptions."""

    data_model: str
    content_tables: tuple[str, ...]
    exceptions: tuple[ExceptionEntry, ...]


def strip_sql(sql: str) -> str:
    """Return sql with comments removed and string literals emptied.

    Neither then counts as SQL text. Line breaks of block comments stay, so the line count holds.
    """
    out: list[str] = []
    i, n = 0, len(sql)
    while i < n:
        c = sql[i]
        if sql.startswith("--", i):
            j = sql.find("\n", i)
            i = n if j < 0 else j
        elif sql.startswith("/*", i):
            j = sql.find("*/", i + 2)
            end = n if j < 0 else j + 2
            out.append("\n" * sql.count("\n", i, end) or " ")
            i = end
        elif c == "'":
            j = i + 1
            while j < n:
                if sql[j] == "'" and sql.startswith("''", j):
                    j += 2
                elif sql[j] == "'":
                    break
                else:
                    j += 1
            out.append("''")
            i = j + 1
        elif c == "$" and (m := re.match(r"\$([A-Za-z_]\w*)?\$", sql[i:])):
            tag = m.group(0)
            j = sql.find(tag, i + len(tag))
            out.append("''")
            i = n if j < 0 else j + len(tag)
        else:
            out.append(c)
            i += 1
    return "".join(out)


def _tokens(sql: str) -> list[str]:
    return _TOKEN.findall(strip_sql(sql))


def is_read(sql: str) -> bool:
    """Report whether sql is a read.

    A read has the first keyword SELECT or WITH and no INSERT, UPDATE or DELETE at the top
    level; FOR UPDATE and FOR NO KEY UPDATE are row locks, no writes.
    """
    toks = _tokens(sql)
    if not toks or toks[0].lower() not in {"select", "with"}:
        return False
    depth = 0
    for i, t in enumerate(toks):
        if t == "(":
            depth += 1
        elif t == ")":
            depth -= 1
        elif depth == 0 and t.lower() in _WRITES:
            prev = toks[i - 1].lower() if i else ""
            if t.lower() == "update" and prev in {"for", "key"}:
                continue
            return False
    return True


def _table_refs(sql: str) -> list[tuple[str, str]]:
    """Return (table, alias) for every table after FROM or JOIN, also in a comma list.

    The alias is the table name when none is given. Function calls and subqueries are skipped.
    """
    toks = _tokens(sql)
    refs: list[tuple[str, str]] = []
    i = 0
    while i < len(toks):
        word = toks[i].lower()
        i += 1
        if word not in {"from", "join"}:
            continue
        while i < len(toks):
            if toks[i].lower() == "only":
                i += 1
            if i >= len(toks) or not re.fullmatch(r"\w+", toks[i]):
                break
            name = toks[i].lower()
            i += 1
            if i + 1 < len(toks) and toks[i] == "." and re.fullmatch(r"\w+", toks[i + 1]):
                name = toks[i + 1].lower()
                i += 2
            if i < len(toks) and toks[i] == "(":
                break  # a function such as unnest(...)
            alias = name
            if i < len(toks) and toks[i].lower() == "as":
                i += 1
            if (
                i < len(toks)
                and re.fullmatch(r"\w+", toks[i])
                and toks[i].lower() not in _CLAUSE_WORDS
            ):
                alias = toks[i].lower()
                i += 1
            refs.append((name, alias))
            if word == "from" and i < len(toks) and toks[i] == ",":
                i += 1
                continue
            break
    return refs


def content_tables_of(sql: str, tables: set[str]) -> set[str]:
    """Return the content tables of tables that sql names after FROM or JOIN."""
    return {name for name, _ in _table_refs(sql) if name in tables}


def has_predicate(sql: str) -> bool:
    """Report whether sql carries the visibility predicate with a sqlc parameter."""
    refs = _table_refs(sql)
    aliases = {alias: name for name, alias in refs}
    keyed = any(name in KEYED_TABLES for name, _ in refs)
    for m in _PREDICATE.finditer(strip_sql(sql)):
        if m.group("col").lower() != "id":
            return True
        alias = (m.group("alias") or "").lower()
        if (alias and aliases.get(alias) in KEYED_TABLES) or (not alias and keyed):
            return True
    return False


def load_queries(repo: Path) -> list[Query]:
    """Return every sqlc query of the server modules in file order."""
    queries: list[Query] = []
    for path in sorted(repo.glob(QUERY_GLOB)):
        module = path.relative_to(repo / "services/server/internal").parts[0]
        rel = path.relative_to(repo).as_posix()
        current: tuple[str, int, list[str]] | None = None
        for no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
            m = _HEADER.match(line)
            if m:
                if current:
                    queries.append(Query(current[0], rel, current[1], "\n".join(current[2])))
                current = (f"{module}.{m.group('name')}", no, [])
            elif current:
                current[2].append(line)
        if current:
            queries.append(Query(current[0], rel, current[1], "\n".join(current[2])))
    return queries


def load_config(repo: Path) -> Config:
    """Read CONFIG of repo. Raises UsageError when it is missing or malformed."""
    path = repo / CONFIG
    try:
        text = path.read_text(encoding="utf-8")
        node = yaml.compose(text)
        data = yaml.safe_load(text)
    except (OSError, yaml.YAMLError) as err:
        raise UsageError(f"{CONFIG}: {err}") from err
    if not isinstance(data, dict):
        raise UsageError(f"{CONFIG}: not a mapping")
    model, tables, raw = data.get("data_model"), data.get("content_tables"), data.get("exceptions")
    if not isinstance(model, str) or not model:
        raise UsageError(f"{CONFIG}: data_model must name the data model chapter")
    if not isinstance(tables, list) or not all(isinstance(t, str) for t in tables):
        raise UsageError(f"{CONFIG}: content_tables must be a list of table names")
    if raw is None:
        raw = []
    if not isinstance(raw, list):
        raise UsageError(f"{CONFIG}: exceptions must be a list")
    lines = _exception_lines(node, len(raw))
    exceptions: list[ExceptionEntry] = []
    for entry, line in zip(raw, lines, strict=True):
        if not isinstance(entry, dict) or not all(
            isinstance(entry.get(k), str) for k in ("query", "category", "reason")
        ):
            raise UsageError(f"{CONFIG}:{line}: an exception needs query, category and reason")
        exceptions.append(ExceptionEntry(entry["query"], entry["category"], entry["reason"], line))
    return Config(model, tuple(tables), tuple(exceptions))


def _exception_lines(node: yaml.Node | None, count: int) -> list[int]:
    """Return the 1-based start line of each exception entry (1 when unknown)."""
    if isinstance(node, yaml.MappingNode):
        for key, value in node.value:
            if key.value == "exceptions" and isinstance(value, yaml.SequenceNode):
                return [item.start_mark.line + 1 for item in value.value]
    return [1] * count


def data_model_tables(repo: Path, chapter: str) -> set[str]:
    """Return the table names of the headings of the data model chapter.

    Raises UsageError when the chapter cannot be read.
    """
    try:
        text = (repo / chapter).read_text(encoding="utf-8")
    except OSError as err:
        raise UsageError(f"{CONFIG}: data_model: {err}") from err
    return {m.group("table") for line in text.splitlines() if (m := _HEADING.match(line))}


def check(repo: Path) -> list[Finding]:
    """Return every finding of repo: queries first in file order, then the configuration."""
    config = load_config(repo)
    tables = set(config.content_tables)
    queries = load_queries(repo)
    excepted = {e.query for e in config.exceptions}
    findings: list[Finding] = []
    for q in queries:
        if q.ident in excepted or not is_read(q.sql) or not content_tables_of(q.sql, tables):
            continue
        if not has_predicate(q.sql):
            findings.append(Finding(q.file, q.line, q.ident, "missing-predicate"))
    known = {q.ident for q in queries}
    seen: set[str] = set()
    for e in config.exceptions:
        rule = ""
        if e.query in seen:
            rule = "duplicate-exception"
        elif e.query not in known:
            rule = "stale-exception"
        elif e.category not in CATEGORIES:
            rule = "unknown-category"
        elif not e.reason.strip():
            rule = "exception-reason"
        seen.add(e.query)
        if rule:
            findings.append(Finding(CONFIG, e.line, e.query, rule))
    model = data_model_tables(repo, config.data_model)
    findings.extend(
        Finding(CONFIG, 1, table, "unknown-content-table")
        for table in config.content_tables
        if table not in model
    )
    return findings


def main(argv: list[str] | None = None) -> int:
    """Run the check on --repo (default: the repository of this file) and print the findings.

    Returns 0 when clean, 1 on findings and 2 on a usage error.
    """
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[2])
    try:
        args = parser.parse_args(argv)
    except SystemExit:
        return 2
    try:
        if not args.repo.is_dir():
            raise UsageError(f"--repo {args.repo}: not a directory")
        findings = check(args.repo)
    except UsageError as err:
        print(f"sql_visibility: {err}", file=sys.stderr)
        return 2
    for f in findings:
        print(f)
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main())
