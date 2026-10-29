# SPDX-License-Identifier: Apache-2.0
"""Command guard: every `bothy-backupd` command line in the current documentation is one the
program accepts.

The source is the usage text of services/server/cmd/bothy-backupd/main.go, checked against the
commands its `run` dispatches. Checked are the code spans and fenced blocks of every tracked
Markdown file except the historical records (briefs, decision logs, change logs), which keep the
words of their time. A runbook that names a missing command (such as `bothy-backupd run --now
full` or `bothy-backupd restore --target-time`) fails here instead of during an installation.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
MAIN = REPO / "services" / "server" / "cmd" / "bothy-backupd" / "main.go"
PROGRAM = "bothy-backupd"
# Historical records: their text stays as written (docs/briefs, decision logs, change logs).
HISTORY = re.compile(
    r"^docs/briefs/|^docs/decisions\.d/|^docs/DECISIONS[^/]*\.md$|(?:^|/)CHANGELOG\.md$|(?:^|/)changelog\.d/"
)
# The aliases of the help, which the usage text does not list as commands.
HELP = {"help", "-h", "--help"}

_USAGE = re.compile(r"^const usage = `(?P<text>[^`]*)`", re.MULTILINE)
_USAGE_LINE = re.compile(rf"^(?:Usage:)?\s+{PROGRAM}(?P<rest>(?: .*)?)$")
_DISPATCH = re.compile(r"^func run\(.*?^}", re.MULTILINE | re.DOTALL)
_CASE = re.compile(r'^\t\tcase (?P<names>"[^\n]*"):', re.MULTILINE)
_FENCE = re.compile(r"^ *(```|~~~)[^\n]*\n(?P<body>.*?)^ *\1 *$", re.MULTILINE | re.DOTALL)
_SPAN = re.compile(r"`(?P<code>[^`]+)`")
# A call of the program: its name as a word (not a path element such as cmd/bothy-backupd/ and
# not the log prefix "bothy-backupd:"), then white space and the first argument.
_CALL = re.compile(rf"(?<![\w/.-]){PROGRAM}[ \t]+(?P<args>[^\n]*)")


def usage() -> str:
    m = _USAGE.search(MAIN.read_text(encoding="utf-8"))
    assert m, f"no usage constant in {MAIN.relative_to(REPO)}"
    return m.group("text")


def usage_lines() -> list[list[str]]:
    """The argument lists of the synopsis lines of the usage text (continuation lines joined)."""
    lines: list[list[str]] = []
    for raw in usage().split("\n\n", 1)[0].splitlines():
        if m := _USAGE_LINE.match(raw):
            lines.append(m.group("rest").split())
        elif lines and raw.startswith(" "):
            lines[-1].extend(raw.split())
    return lines


def commands() -> set[str]:
    """The subcommands of the usage text."""
    return {args[0] for args in usage_lines() if args and not args[0].startswith("[")}


def default_flags() -> set[str]:
    """The options of the default command (the synopsis line without a subcommand)."""
    flags: set[str] = set()
    for args in usage_lines():
        if not args or args[0].startswith("["):
            flags |= {a.strip("[]") for a in args if a.startswith("[--")}
    return flags


def backup_kinds() -> set[str]:
    for args in usage_lines():
        if args[:1] == ["backup"]:
            return set(args[1].strip("<>").split("|"))
    raise AssertionError("the usage text has no synopsis of backup")


def dispatched() -> set[str]:
    """The commands that `run` in main.go dispatches, help aliases included."""
    m = _DISPATCH.search(MAIN.read_text(encoding="utf-8"))
    assert m, "no func run in main.go"
    return {
        name
        for c in _CASE.finditer(m.group(0))
        for name in re.findall(r'"([^"]+)"', c.group("names"))
    }


def code_of(text: str) -> list[str]:
    """The fenced blocks and the code spans of a Markdown text."""
    blocks = [m.group("body") for m in _FENCE.finditer(text)]
    rest = _FENCE.sub("", text)
    return blocks + [m.group("code") for m in _SPAN.finditer(rest)]


def problems(code: str) -> list[str]:
    """What is wrong with each call of bothy-backupd in a piece of code, one entry per call."""
    found: list[str] = []
    cmds, flags, kinds = commands(), default_flags(), backup_kinds()
    for m in _CALL.finditer(code):
        args = m.group("args").split()
        first = args[0] if args else ""
        call = f"{PROGRAM} {' '.join(args[:3])}".strip()
        if first in cmds or first in HELP or first in ("...", "") or first.startswith("["):
            pass
        elif first.startswith("-"):
            if first.split("=", 1)[0] not in flags:
                found.append(f"{call}: unknown option {first}")
            continue
        else:
            found.append(f"{call}: unknown command {first}")
            continue
        if first == "backup" and len(args) > 1:
            kind = args[1].rstrip(")`.,;")
            if not (kind.startswith("<") or kind in kinds):
                found.append(f"{call}: unknown backup kind {kind}")
    return found


def documents() -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", "*.md"],
        cwd=REPO,
        capture_output=True,
        check=True,
        text=True,
    ).stdout
    return [REPO / p for p in out.split("\0") if p and not HISTORY.search(p)]


def test_the_usage_text_lists_the_dispatched_commands() -> None:
    assert commands() == dispatched() - HELP
    assert commands() >= {"backup", "expire", "restore-test", "healthcheck", "version"}
    assert backup_kinds() == {"pg_full", "pg_diff", "logical", "objects"}
    assert "--config" in default_flags()


@pytest.mark.parametrize(
    "code",
    [
        "docker compose exec backup bothy-backupd run --now full",
        "docker compose exec backup bothy-backupd run --now restore-test",
        'docker compose run --rm backup bothy-backupd restore --target-time "<time>"',
        "bothy-backupd backup everything",
        "bothy-backupd --now",
    ],
)
def test_a_missing_command_is_reported(code: str) -> None:
    assert problems(code), code


@pytest.mark.parametrize(
    "code",
    [
        "docker compose exec backup bothy-backupd backup pg_full",
        "bothy-backupd backup <kind>",
        "bothy-backupd backup <pg_full|pg_diff|logical|objects> [--config FILE]",
        "docker compose exec backup bothy-backupd restore-test",
        "bothy-backupd expire --offsite --credentials DIR",
        "bothy-backupd healthcheck",
        "bothy-backupd version",
        "bothy-backupd --help",
        "bothy-backupd [--pgbackrest PATH] [--stanza NAME]",
        "bothy-backupd ...",
        "bothy-backupd",
        "bothy-backupd: backup pg_full succeeded",
        "go test ./cmd/bothy-backupd/",
        'ServiceName: "bothy-backupd", ...',
    ],
)
def test_a_real_command_or_a_mere_name_passes(code: str) -> None:
    assert problems(code) == [], code


def test_code_spans_and_fenced_blocks_are_read() -> None:
    text = (
        "Run `bothy-backupd run\n  --now full`.\n\n"
        "```sh\nbothy-backupd restore --target-time x\n```\n"
    )
    found = [p for code in code_of(text) for p in problems(code)]
    assert len(found) == 2, found
    assert code_of("prose names bothy-backupd run here") == []


def test_the_history_is_left_as_written() -> None:
    for path in (
        "docs/briefs/M7-03-restore-test.md",
        "docs/decisions.d/M7-02.md",
        "docs/DECISIONS.md",
        "CHANGELOG.md",
        "services/server/changelog.d/M7-02.md",
    ):
        assert HISTORY.search(path), path
    for path in ("docs/operations/install-compose.md", "docs/spec/16-operations.md"):
        assert not HISTORY.search(path), path


def test_the_documentation_names_only_real_commands() -> None:
    found = [
        f"{path.relative_to(REPO)}: {p}"
        for path in documents()
        for code in code_of(path.read_text(encoding="utf-8"))
        for p in problems(code)
    ]
    assert found == [], "\n".join(found)
