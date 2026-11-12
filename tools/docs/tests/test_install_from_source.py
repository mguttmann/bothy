# SPDX-License-Identifier: Apache-2.0
"""The guide "From source" names images the Compose stack reads.

docs/operations/install-compose.md, section 1.2, tells an operator to build both images with
`make image` and `make postgres-image` under a local name and to put two values into `.env`.
The page once sent readers to a stack it called not startable (docs/decisions.d/AUD-root-01.md).
The test takes the commands and the values from the page, asks the real Makefile for the image
references the two targets would load (`make -n`) and compares them with the references of
infra/compose/compose.yaml for these values.
"""

from __future__ import annotations

import os
import re
import shlex
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
GUIDE = REPO / "docs" / "operations" / "install-compose.md"
COMPOSE = REPO / "infra" / "compose" / "compose.yaml"
COMMIT = "0123abc"
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS"}
_SECTION = re.compile(
    r"^### 1\.2 From source[^\n]*\n(?P<body>.*?)(?=^## )", re.MULTILINE | re.DOTALL
)
_COMPOSE_IMAGE = re.compile(
    r"^\s*image: \$\{BOTHY_REGISTRY[^}]*\}/(?P<name>[a-z0-9-]+):(?P<tag>\S+)$", re.MULTILINE
)
_VARIABLE = re.compile(r"\$\{(?P<name>[A-Z_]+)[^}]*\}")


def _section() -> str:
    match = _SECTION.search(GUIDE.read_text(encoding="utf-8"))
    assert match, "install-compose.md has no section 1.2 From source"
    return match.group("body")


def _make_commands() -> dict[str, list[str]]:
    """The `make` lines of the section by target, with `$tag` as the page defines it."""
    section = _section()
    assert 'tag="src-$(git rev-parse --short HEAD)"' in section
    commands = {}
    for line in section.splitlines():
        if line.startswith("make "):
            words = shlex.split(line.replace("$tag", f"src-{COMMIT}"))
            commands[words[1]] = words[2:]
    return commands


def _loaded_image(target: str, variables: list[str]) -> str:
    """The reference `make <target>` loads into the local image store with these variables."""
    env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
    printed = subprocess.run(
        ["make", "-n", "-C", str(REPO), target, *variables],
        env=env,
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    [tag] = re.findall(r"--load --tag '([^']+)'", printed)
    return str(tag)


def _env_values() -> dict[str, str]:
    values = dict(re.findall(r"^(BOTHY_[A-Z_]+)=(\S+)$", _section(), re.MULTILINE))
    assert set(values) == {"BOTHY_REGISTRY", "BOTHY_VERSION"}
    assert values["BOTHY_VERSION"] == "src-<commit>"
    values["BOTHY_VERSION"] = f"src-{COMMIT}"
    return values


def test_the_guide_builds_the_two_images_the_stack_reads() -> None:
    commands = _make_commands()
    assert set(commands) == {"image", "postgres-image"}
    built = {_loaded_image(target, variables) for target, variables in commands.items()}

    values = _env_values()
    compose = COMPOSE.read_text(encoding="utf-8")
    wanted = {
        f"{values['BOTHY_REGISTRY']}/{name}:"
        + _VARIABLE.sub(lambda m: values[m.group("name")], tag)
        for name, tag in _COMPOSE_IMAGE.findall(compose)
    }
    assert wanted == {
        f"localhost/bothy-server:src-{COMMIT}",
        f"localhost/bothy-postgres:pg18-src-{COMMIT}",
    }
    assert built == wanted


def test_the_stale_notes_about_a_stack_that_cannot_start_are_gone() -> None:
    for path in (GUIDE, COMPOSE):
        text = path.read_text(encoding="utf-8")
        assert "not started" not in text, path
        assert "created with the first release" not in text, path
