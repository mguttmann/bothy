# SPDX-License-Identifier: Apache-2.0
"""Generate section 4 of docs/operations/configuration.md from the chapter tables.

Ruling L4: the source of every named configuration value is the table "Konfigurationswerte
dieses Kapitels" of its chapter in docs/spec/. This tool collects those tables, fails if a
name appears twice, and writes the block between the markers in configuration.md. `--check`
fails if the committed block is stale. From M1 the configuration schema in the server code
becomes the source; then this tool reads the schema instead.

Table rows: the first cell holds one name, a list (`a.x`, `a.y`) or a shorthand
(`a.b.per_minute` / `.burst`, defaults `30` / `10`). A table with an "Ort" column names the
program that reads the value; without it the chapter default below applies.
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

START = "<!-- config-reference:start -->"
END = "<!-- config-reference:end -->"
REPO = Path(__file__).resolve().parents[2]
DEFAULT_SPEC = REPO / "docs" / "spec"
DEFAULT_REFERENCE = REPO / "docs" / "operations" / "configuration.md"
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
_SECTION = re.compile(
    r"^## (?P<num>[0-9]+(?:\.[0-9]+)*)\.? Konfigurationswerte dieses Kapitels\s*$", re.MULTILINE
)
_TITLE = re.compile(r"^# (?P<title>.+)$", re.MULTILINE)
_NAME = re.compile(r"^[a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+$")
_TICKED = re.compile(r"`([^`]+)`")
# Links in chapter tables are relative to docs/spec/; the reference lives in docs/operations/.
_LOCAL_LINK = re.compile(r"\]\((?!\.\./|[a-z]+:|#|/)(?P<target>[^)]+)\)")


class ConfigError(Exception):
    """A chapter table breaks the rules of ruling L4."""


@dataclass(frozen=True)
class Value:
    name: str
    place: str
    default: str
    meaning: str

    @property
    def env(self) -> str:
        if self.place != SERVER:
            return ""
        return "BOTHY_" + self.name.upper().replace(".", "__")


@dataclass(frozen=True)
class Chapter:
    file: str
    title: str
    section: str
    values: tuple[Value, ...]


def _cells(line: str) -> list[str]:
    return [c.strip() for c in line.strip().strip("|").split("|")]


def _names(cell: str, where: str) -> list[str]:
    names: list[str] = []
    for token in _TICKED.findall(cell):
        name = token
        if token.startswith(".") and names:
            name = names[-1].rsplit(".", 1)[0] + token
        if not _NAME.match(name):
            raise ConfigError(f"{where}: no value name in `{name}`")
        names.append(name)
    if not names:
        raise ConfigError(f"{where}: no value name in {cell!r}")
    return names


def _defaults(cell: str, count: int, where: str) -> list[str]:
    parts = [p.strip() for p in cell.split(" / ")]
    if count == 1 or len(parts) == 1:
        return [cell] * count
    if len(parts) != count:
        raise ConfigError(f"{where}: {count} names but {len(parts)} defaults")
    return parts


def parse(path: Path) -> Chapter | None:
    text = path.read_text(encoding="utf-8")
    m = _SECTION.search(text)
    if m is None:
        return None
    body = text[m.end() :]
    nxt = re.search(r"^## ", body, re.MULTILINE)
    if nxt:
        body = body[: nxt.start()]
    title_m = _TITLE.search(text)
    title = title_m.group("title").strip() if title_m else path.stem
    rows = [ln for ln in body.splitlines() if ln.startswith("|")]
    values: list[Value] = []
    if rows:
        header = [h.lower() for h in _cells(rows[0])]
        has_place = len(header) > 3 and header[1] == "ort"
        default_place = DEFAULT_PLACE.get(path.name, SERVER)
        for n, line in enumerate(rows[2:], start=1):
            cells = _cells(line)
            where = f"{path.name}, row {n}"
            place = cells[1] if has_place else default_place
            rest = cells[2:] if has_place else cells[1:]
            names = _names(cells[0], where)
            defaults = _defaults(rest[0], len(names), where)
            meaning = _LOCAL_LINK.sub(r"](../spec/\g<target>)", " | ".join(rest[1:]))
            values += [Value(nm, place, d, meaning) for nm, d in zip(names, defaults, strict=True)]
    return Chapter(path.name, title, m.group("num"), tuple(values))


def collect(spec_dir: Path) -> list[Chapter]:
    chapters: list[Chapter] = []
    seen: dict[str, str] = {}
    for path in sorted(spec_dir.glob("[0-9][0-9]-*.md")):
        if path.name in EXCLUDED:
            continue
        chapter = parse(path)
        if chapter is None:
            continue
        for value in chapter.values:
            if value.name in seen:
                raise ConfigError(
                    f"{value.name}: named twice ({seen[value.name]} and {path.name}); ruling L4"
                )
            seen[value.name] = path.name
        chapters.append(chapter)
    return chapters


def render(reference: str, chapters: list[Chapter]) -> str:
    if START not in reference or END not in reference:
        raise ConfigError(f"configuration.md needs the markers {START} and {END}")
    out: list[str] = []
    for ch in chapters:
        out += [
            f"### {ch.title}",
            "",
            f"Quelle: [{ch.file}](../spec/{ch.file}), Abschnitt {ch.section}.",
            "",
            "| Schlüssel | Umgebung | Ort | Standard | Bedeutung |",
            "|---|---|---|---|---|",
        ]
        for v in ch.values:
            env = f"`{v.env}`" if v.env else ""
            out.append(f"| `{v.name}` | {env} | {v.place} | {v.default} | {v.meaning} |")
        out.append("")
    head, rest = reference.split(START, 1)
    _, tail = rest.split(END, 1)
    return f"{head}{START}\n" + "\n".join(out) + f"{END}{tail}"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Generate or check the configuration reference.")
    parser.add_argument("--spec", type=Path, default=DEFAULT_SPEC)
    parser.add_argument("--reference", type=Path, default=DEFAULT_REFERENCE)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args(argv)
    try:
        current = args.reference.read_text(encoding="utf-8")
        wanted = render(current, collect(args.spec))
    except (ConfigError, FileNotFoundError) as exc:
        print(f"config_reference: {exc}", file=sys.stderr)
        return 1
    if args.check:
        if current != wanted:
            print(
                "config_reference: docs/operations/configuration.md is stale; "
                "run `make config-reference`",
                file=sys.stderr,
            )
            return 1
        return 0
    args.reference.write_text(wanted, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
