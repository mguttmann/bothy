# SPDX-License-Identifier: Apache-2.0
"""Migration rules of a Go module (gate `migrations-lint`, ADR 0007, chapter 14).

The base branch carries only sequential, gap-free migration numbers, and a merged
migration never changes (ADR 0007 rules 4 and 5). This tool holds both before the first
migration exists, together with two SQL rules that PostgreSQL would not reject on its
own: every `GENERATED ALWAYS AS (...)` column states `STORED` (PostgreSQL 18 creates
virtual columns by default, chapter 14 "Ausdrücke"), and a statement with `CONCURRENTLY`
stands in a file marked `-- +goose NO TRANSACTION` (ADR 0007 rule 3).

Immutability is held by a sum file next to `migrations/` (`<module>/migrations.sha256`,
format of `shasum -a 256`, sorted by name, final newline). Only `lock` adds lines, and it
never changes or removes one, so a changed or deleted merged migration stays a finding
until the change is undone, unless its line is edited or removed by hand as well (a limit
named in docs/quality/gates.md, "Was die Gates nicht abdecken"). By hand, a branch removes
only its own line, not yet merged, when it renumbers (docs/quality/gates.md "Migrationen").

Rule ids (Finding.rule): layout, name, timestamp-version, version-format,
duplicate-version, version-gap, goose-up, generated-not-stored, no-transaction, and the
sum-file rules sum-format, unlocked-migration, changed-migration, missing-migration.

Usage:
  migrations_lint.py check --dir <migrations dir> --sum <sum file>
  migrations_lint.py lock  --dir <migrations dir> --sum <sum file>

Exit codes: 0 clean (or locked), 1 findings or refusal, 2 usage error.
"""

from __future__ import annotations

import argparse
import hashlib
import re
import sys
from dataclasses import dataclass
from pathlib import Path

NAME = re.compile(r"(?P<version>[0-9]+)_(?P<slug>[a-z0-9]+(?:_[a-z0-9]+)*)\.sql")
SEQUENTIAL_DIGITS = 5
TIMESTAMP_DIGITS = 14
GOOSE_UP = "-- +goose Up"
GOOSE_NO_TRANSACTION = "-- +goose NO TRANSACTION"

_SUM_LINE = re.compile(r"(?P<digest>[0-9a-f]{64})  (?P<name>[^/\s]+)")
# Words follow PostgreSQL's lexer: letters, digits, `_`, `$` and every non-ASCII character
# continue an identifier (idx$concurrently, idx\u20acconcurrently), and only ASCII white space
# separates words (re.ASCII keeps `\s` from matching NBSP). `\b` would get both wrong.
_IDENT = r"[A-Za-z0-9_$\u0080-\U0010ffff]"
_IDENT_CHAR = re.compile(_IDENT)
_WORDS = re.IGNORECASE | re.ASCII
_GENERATED = re.compile(rf"(?<!{_IDENT})GENERATED\s+ALWAYS\s+AS\s*\(", _WORDS)
_STORED = re.compile(rf"\s*STORED(?!{_IDENT})", _WORDS)
_CONCURRENTLY = re.compile(rf"(?<!{_IDENT})CONCURRENTLY(?!{_IDENT})", _WORDS)
_REFRESH_PREFIX = re.compile(rf"(?<!{_IDENT})REFRESH\s+MATERIALIZED\s+VIEW\s+$", _WORDS)
# Dollar-quote tags: an identifier without `$` (non-ASCII letters included), or empty.
_DOLLAR_TAG = re.compile(r"\$(?:[A-Za-z_\u0080-\U0010ffff][A-Za-z0-9_\u0080-\U0010ffff]*)?\$")


@dataclass(frozen=True)
class Finding:
    """One rule violation.

    Attributes:
        file: path of the migration, of the sum file or of the offending directory.
        line: 1-based line number, or None when the finding concerns the whole file.
        rule: rule id, for example `version-gap` (list in the module docstring).
        message: what is wrong and how to fix it.
    """

    file: str
    line: int | None
    rule: str
    message: str

    def __str__(self) -> str:
        """Return `<file>:<line>: <rule>: <message>`, or `<file>: <rule>: <message>`."""
        where = self.file if self.line is None else f"{self.file}:{self.line}"
        return f"{where}: {self.rule}: {self.message}"


@dataclass(frozen=True)
class _SumEntry:
    line: int
    digest: str


def _is_ident(char: str) -> bool:
    """True for characters that may continue a PostgreSQL identifier (any non-ASCII too)."""
    return _IDENT_CHAR.fullmatch(char) is not None


def _quoted_end(text: str, start: int, quote: str, *, backslash: bool) -> int:
    """Index after the literal that opens at `start`; a doubled quote stays inside it."""
    i = start + 1
    while i < len(text):
        char = text[i]
        if backslash and char == "\\":
            i += 2
            continue
        if char == quote:
            if text.startswith(quote, i + 1):
                i += 2
                continue
            return i + 1
        i += 1
    return len(text)


def _block_comment_end(text: str, start: int) -> int:
    """Index after the (possibly nested) block comment that opens at `start`."""
    depth, i = 1, start + 2
    while i < len(text) and depth:
        if text.startswith("/*", i):
            depth, i = depth + 1, i + 2
        elif text.startswith("*/", i):
            depth, i = depth - 1, i + 2
        else:
            i += 1
    return i


def blank_sql(text: str) -> str:
    """Replace comments, string literals and quoted identifiers of SQL text by spaces.

    Line comments, nested block comments, single-quoted strings (also `E'...'` with
    backslash escapes), dollar-quoted strings (`$$...$$`, `$tag$...$tag$`) and
    double-quoted identifiers are blanked; newlines stay, so offsets and line numbers of
    the remaining code do not move. A keyword that only appears inside such text can then
    no longer match a rule. An unterminated literal or comment blanks to the end.

    Args:
        text: SQL source.

    Returns:
        Text of the same length with the blanked ranges replaced by spaces.

    Example:
        >>> blank_sql("SELECT 'x'; -- y")
        'SELECT    ;     '
    """
    out = list(text)
    i = 0
    while i < len(text):
        char = text[i]
        prev = text[i - 1] if i else ""
        if text.startswith("--", i):
            end = text.find("\n", i)
            end = len(text) if end < 0 else end
        elif text.startswith("/*", i):
            end = _block_comment_end(text, i)
        elif char == "'":
            escape = prev in ("e", "E") and (i < 2 or not _is_ident(text[i - 2]))
            end = _quoted_end(text, i, "'", backslash=escape)
        elif char == '"':
            end = _quoted_end(text, i, '"', backslash=False)
        elif char == "$" and not (prev and _is_ident(prev)) and (tag := _DOLLAR_TAG.match(text, i)):
            close = text.find(tag.group(0), tag.end())
            end = len(text) if close < 0 else close + len(tag.group(0))
        else:
            i += 1
            continue
        for k in range(i, end):
            if out[k] != "\n":
                out[k] = " "
        i = end
    return "".join(out)


def _line_of(text: str, pos: int) -> int:
    return text.count("\n", 0, pos) + 1


def _closing_paren(text: str, start: int) -> int | None:
    """Index of the parenthesis that closes the one at `start`, or None."""
    depth = 0
    for i in range(start, len(text)):
        if text[i] == "(":
            depth += 1
        elif text[i] == ")":
            depth -= 1
            if depth == 0:
                return i
    return None


def _sql_rules(path: Path, text: str) -> list[Finding]:
    """goose-up, generated-not-stored and no-transaction for one migration file."""
    findings: list[Finding] = []
    raw_lines = [line.rstrip() for line in text.split("\n")]
    ups = [no for no, line in enumerate(raw_lines, 1) if line == GOOSE_UP]
    if not ups:
        findings.append(Finding(str(path), None, "goose-up", f"missing the line `{GOOSE_UP}`"))
    elif len(ups) > 1:
        findings.append(
            Finding(
                str(path), ups[1], "goose-up", f"{len(ups)} lines `{GOOSE_UP}`; exactly one allowed"
            )
        )
    code = blank_sql(text)
    for match in _GENERATED.finditer(code):
        close = _closing_paren(code, match.end() - 1)
        if close is None or not _STORED.match(code, close + 1):
            findings.append(
                Finding(
                    str(path),
                    _line_of(code, match.start()),
                    "generated-not-stored",
                    "GENERATED ALWAYS AS (...) must be followed by STORED; PostgreSQL 18 "
                    'creates a virtual column otherwise (docs/spec/14-suche.md "Ausdrücke")',
                )
            )
    if GOOSE_NO_TRANSACTION not in raw_lines:
        for match in _CONCURRENTLY.finditer(code):
            statement_start = code.rfind(";", 0, match.start()) + 1
            if _REFRESH_PREFIX.search(code, statement_start, match.start()):
                continue
            findings.append(
                Finding(
                    str(path),
                    _line_of(code, match.start()),
                    "no-transaction",
                    f"CONCURRENTLY cannot run in a transaction; put the statement in its own "
                    f"file with the line `{GOOSE_NO_TRANSACTION}` (ADR 0007 rule 3)",
                )
            )
    return findings


def _scan_dir(migrations_dir: Path) -> tuple[list[Path], list[Finding]]:
    """The `.sql` files directly in the directory, and a `layout` finding per subdirectory."""
    files: list[Path] = []
    findings: list[Finding] = []
    for path in sorted(migrations_dir.iterdir()):
        if path.is_dir():
            findings.append(
                Finding(str(path), None, "layout", "migrations live directly in the directory")
            )
        elif path.name.endswith(".sql") and path.is_file():
            files.append(path)
    return files, findings


def _version_rules(files: list[Path]) -> list[Finding]:
    """name, timestamp-version, version-format, duplicate-version and version-gap."""
    findings: list[Finding] = []
    versions: dict[int, list[Path]] = {}
    for path in files:
        match = NAME.fullmatch(path.name)
        if match is None:
            findings.append(
                Finding(
                    str(path),
                    None,
                    "name",
                    "file name must be NNNNN_<slug>.sql: a five-digit version and a slug of "
                    "lower-case letters, digits and single underscores",
                )
            )
            continue
        digits = len(match.group("version"))
        if digits == TIMESTAMP_DIGITS:
            findings.append(
                Finding(
                    str(path),
                    None,
                    "timestamp-version",
                    "timestamp versions are for drafts only; number migrations sequentially "
                    "before the final `make check` (ADR 0007 rule 4)",
                )
            )
        elif digits != SEQUENTIAL_DIGITS:
            findings.append(
                Finding(
                    str(path),
                    None,
                    "version-format",
                    "the version must have exactly five digits (goose sequential format 00001)",
                )
            )
        else:
            versions.setdefault(int(match.group("version")), []).append(path)
    expected = 1
    for version in sorted(versions):
        first, *others = versions[version]
        findings.extend(
            Finding(
                str(other),
                None,
                "duplicate-version",
                f"version {version:05d} is also used by {first.name}; the branch merged second "
                "renumbers its migration (ADR 0007 rule 4)",
            )
            for other in others
        )
        if version != expected:
            findings.append(
                Finding(
                    str(first),
                    None,
                    "version-gap",
                    f"expected version {expected:05d} here, found {version:05d}; versions run "
                    "00001..N without gaps (ADR 0007 rule 4)",
                )
            )
        expected = version + 1
    return findings


def _migration_rules(migrations_dir: Path) -> tuple[list[Path], list[Finding]]:
    """Every rule that does not need the sum file."""
    files, findings = _scan_dir(migrations_dir)
    findings.extend(_version_rules(files))
    for path in files:
        findings.extend(_sql_rules(path, path.read_bytes().decode("utf-8", errors="replace")))
    return files, findings


def _read_sums(sum_file: Path) -> tuple[dict[str, _SumEntry], list[Finding], list[Finding]]:
    """Parse the sum file.

    Returns the entries by file name, the findings that make `lock` refuse (unreadable
    line, repeated name) and the findings `lock` repairs by rewriting (order, final newline).
    """
    entries: dict[str, _SumEntry] = {}
    broken: list[Finding] = []
    layout: list[Finding] = []
    if not sum_file.exists():
        return entries, broken, layout
    where = str(sum_file)
    try:
        text = sum_file.read_bytes().decode("utf-8")
    except UnicodeDecodeError:
        broken.append(Finding(where, None, "sum-format", "the file is not valid UTF-8"))
        return entries, broken, layout
    lines = text.split("\n")
    if text.endswith("\n"):
        lines.pop()
    elif text:
        layout.append(Finding(where, None, "sum-format", "the file must end with a newline"))
    else:
        lines = []
    for no, line in enumerate(lines, 1):
        match = _SUM_LINE.fullmatch(line)
        if match is None:
            broken.append(
                Finding(
                    where,
                    no,
                    "sum-format",
                    "expected '<sha256 in lower-case hex>  <file name>' (shasum -a 256)",
                )
            )
        elif match.group("name") in entries:
            broken.append(Finding(where, no, "sum-format", f"{match.group('name')} listed twice"))
        else:
            entries[match.group("name")] = _SumEntry(no, match.group("digest"))
    if list(entries) != sorted(entries):
        layout.append(Finding(where, None, "sum-format", "lines must be sorted by file name"))
    return entries, broken, layout


def _sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def lint(migrations_dir: Path, sum_file: Path) -> list[Finding]:
    """Check a migrations directory and its sum file against every rule.

    Args:
        migrations_dir: directory with the goose files (`<module>/migrations`).
        sum_file: the sum file (`<module>/migrations.sha256`); absent means empty.

    Returns:
        The findings, empty when the directory is clean.

    Raises:
        OSError: the directory cannot be read (callers check `is_dir()` first).
    """
    files, findings = _migration_rules(migrations_dir)
    entries, broken, layout = _read_sums(sum_file)
    findings.extend(broken)
    findings.extend(layout)
    present = {path.name for path in files}
    for path in files:
        entry = entries.get(path.name)
        if entry is None:
            findings.append(
                Finding(
                    str(path),
                    None,
                    "unlocked-migration",
                    f"not listed in {sum_file.name}; run `make migrations-lock`",
                )
            )
        elif _sha256(path) != entry.digest:
            findings.append(
                Finding(
                    str(path),
                    None,
                    "changed-migration",
                    f"sha256 differs from line {entry.line} of {sum_file.name}; a merged "
                    "migration never changes, write a new one (ADR 0007 rule 5)",
                )
            )
    findings.extend(
        Finding(
            str(sum_file),
            entry.line,
            "missing-migration",
            f"{name} is listed but is no migration file in {migrations_dir.name}/; a merged "
            "migration is never removed (ADR 0007 rule 5)",
        )
        for name, entry in entries.items()
        if name not in present
    )
    return findings


def lock(migrations_dir: Path, sum_file: Path) -> tuple[list[str], list[Finding]]:
    """Add a sum line for every migration that has none yet.

    Runs every rule except the sum-file rules first; if any finding remains, or a line of
    the sum file cannot be read or names a file twice, it refuses and writes nothing.
    Existing lines are never changed or removed, also when their file changed or is gone
    (`check` keeps failing then). The file is rewritten sorted by name with a final
    newline; it is not created while there is no migration.

    Args:
        migrations_dir: directory with the goose files.
        sum_file: the sum file to extend.

    Returns:
        The file names that were added, and the findings that made it refuse.
    """
    files, refused = _migration_rules(migrations_dir)
    entries, broken, _ = _read_sums(sum_file)
    refused.extend(broken)
    if refused:
        return [], refused
    digests = {name: entry.digest for name, entry in entries.items()}
    added = sorted(path.name for path in files if path.name not in digests)
    digests.update({path.name: _sha256(path) for path in files if path.name in added})
    content = "".join(f"{digests[name]}  {name}\n" for name in sorted(digests)).encode("utf-8")
    old = sum_file.read_bytes() if sum_file.exists() else b""
    if content != old:
        sum_file.write_bytes(content)
    return added, []


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring for usage and exit codes."""
    parser = argparse.ArgumentParser(
        prog="migrations_lint.py", description="Migration rules of a Go module (ADR 0007)."
    )
    sub = parser.add_subparsers(dest="cmd", required=True)
    for cmd, text in (("check", "check every rule"), ("lock", "add sum lines for new files")):
        cmd_parser = sub.add_parser(cmd, help=text)
        cmd_parser.add_argument("--dir", type=Path, required=True, help="migrations directory")
        cmd_parser.add_argument("--sum", type=Path, required=True, help="sum file")
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:  # argparse ends on usage errors (2) and on --help (0)
        return exc.code if isinstance(exc.code, int) else 2
    if not args.dir.is_dir():
        print(f"migrations-lint: {args.dir} is not a directory", file=sys.stderr)
        return 2
    if args.cmd == "lock":
        added, refused = lock(args.dir, args.sum)
        for finding in refused:
            print(finding)
        if refused:
            print(f"migrations-lock: refused, {args.sum} unchanged; fix the findings above")
            return 1
        for name in added:
            print(f"locked: {name}")
        if not added:
            print(f"migrations-lock: no new migration in {args.dir}")
        return 0
    findings = lint(args.dir, args.sum)
    for finding in findings:
        print(finding)
    if findings:
        print(f"migrations-lint: {len(findings)} finding(s) in {args.dir}")
        return 1
    print(f"migrations-lint: {args.dir} clean")
    return 0


if __name__ == "__main__":
    sys.exit(main())
