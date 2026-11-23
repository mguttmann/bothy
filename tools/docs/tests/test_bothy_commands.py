# SPDX-License-Identifier: Apache-2.0
"""Command register guard: the commands that the documentation lists for the server binary are the
commands the binary registers.

The source is the table `commands` of services/server/cmd/bothy/commands.go. The register is the
table of docs/spec/02-architecture.md section 3.3; the table of services/server/README.md repeats
it. A row whose command the binary does not register must say "not built" and name its plan row,
and every registered command has a row in the register (audit docs-onboarding-6: the register
named a rescan command for files and a seed command for dev, which the binary does not know).
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
COMMANDS = REPO / "services" / "server" / "cmd" / "bothy" / "commands.go"
REGISTER = REPO / "docs" / "spec" / "02-architecture.md"
README = REPO / "services" / "server" / "README.md"

_TABLE = re.compile(r"^var commands = \[\]command\{\n(?P<body>.*?)^\}", re.MULTILINE | re.DOTALL)
_NAME = re.compile(r'^\t\tname:\s+"(?P<name>[a-z-]+)",$', re.MULTILINE)
# The name of the binary is the folder of its sources (cmd/<binary>).
BINARY = COMMANDS.parent.name
# A row of a command table: its first cell starts with the code span of the binary and a command.
_ROW = re.compile(r"^\| `" + re.escape(BINARY) + r" (?P<name>[a-z-]+)[ `\\|]")
_NOT_BUILT = re.compile(r"not built \(plan row V\d+-\d+\)")


def registered() -> set[str]:
    m = _TABLE.search(COMMANDS.read_text(encoding="utf-8"))
    assert m, f"no table `commands` in {COMMANDS.relative_to(REPO)}"
    names = {n.group("name") for n in _NAME.finditer(m.group("body"))}
    assert names, "the table `commands` names no command"
    return names


def section(path: Path, heading: str | None) -> list[str]:
    """The lines of `path` below `heading` up to the next heading (the whole file without one)."""
    lines = path.read_text(encoding="utf-8").splitlines()
    if heading is None:
        return lines
    start = lines.index(heading)
    end = next((i for i in range(start + 1, len(lines)) if lines[i].startswith("#")), len(lines))
    return lines[start:end]


def rows(lines: list[str]) -> dict[str, list[str]]:
    """The rows per command of the command tables in `lines`."""
    found: dict[str, list[str]] = {}
    for line in lines:
        if m := _ROW.match(line):
            found.setdefault(m.group("name"), []).append(line)
    return found


DOCUMENTS = {
    "register": (REGISTER, "### 3.3 Commands of the binary"),
    "server README": (README, None),
}


@pytest.mark.parametrize("doc", sorted(DOCUMENTS))
def test_every_documented_command_is_registered_or_marked(doc: str) -> None:
    path, heading = DOCUMENTS[doc]
    documented = rows(section(path, heading))
    assert documented, f"{path.relative_to(REPO)} has no command table"
    names = registered()
    wrong = [
        f"{path.relative_to(REPO)}: `{BINARY} {name}` is not registered in commands.go and its row"
        " does not say 'not built (plan row ...)'"
        for name, lines in sorted(documented.items())
        if name not in names and not all(_NOT_BUILT.search(line) for line in lines)
    ]
    assert not wrong, "\n".join(wrong)


@pytest.mark.parametrize("doc", sorted(DOCUMENTS))
def test_no_registered_command_is_marked_not_built(doc: str) -> None:
    path, heading = DOCUMENTS[doc]
    stale = [
        f"{path.relative_to(REPO)}: `{BINARY} {name}` is registered but marked not built"
        for name, lines in sorted(rows(section(path, heading)).items())
        if name in registered() and any(_NOT_BUILT.search(line) for line in lines)
    ]
    assert not stale, "\n".join(stale)


def test_every_registered_command_is_in_the_register() -> None:
    documented = rows(section(*DOCUMENTS["register"]))
    missing = sorted(registered() - documented.keys())
    assert not missing, f"commands without a row in {REGISTER.relative_to(REPO)} 3.3: {missing}"
