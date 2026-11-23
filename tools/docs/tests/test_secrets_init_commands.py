# SPDX-License-Identifier: Apache-2.0
"""A `secrets init` command of the operations guides names the owners of the Compose stack.

`secrets-init` in infra/compose/compose.yaml gives every service its subdirectory of the volume
`secrets` with an owner (`--owner <service>=<uid>:<gid>`). A run of `secrets init` by hand that
leaves an owner out sets that directory back to the default owner; for `postgres` and `backup`
pgBackRest then refuses its TLS keys. So every `secrets init` command of docs/operations that
runs in the container `secrets-init` names the same owners as compose.yaml.

The test lives with the tools tests because they run on every change, also on one that touches
only a document.
"""

from __future__ import annotations

import re
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
COMPOSE = REPO / "infra" / "compose" / "compose.yaml"
GUIDES = REPO / "docs" / "operations"
OWNER = re.compile(r"--owner\s+([a-z-]+=\d+:\d+)")
COMMAND = re.compile(r"`(docker compose run [^`]*secrets-init secrets init [^`]*)`")


def compose_owners() -> set[str]:
    """Return the `--owner` values of the service `secrets-init` in compose.yaml."""
    command = yaml.safe_load(COMPOSE.read_text(encoding="utf-8"))["services"]["secrets-init"][
        "command"
    ]
    return {command[i + 1] for i, part in enumerate(command) if part == "--owner"}


def guide_commands() -> list[tuple[str, str]]:
    """Return every `secrets init` command run in `secrets-init` of a guide, with its file."""
    return [
        (path.name, match.group(1))
        for path in sorted(GUIDES.glob("*.md"))
        for match in COMMAND.finditer(path.read_text(encoding="utf-8"))
    ]


def test_compose_names_the_four_owners() -> None:
    owners = compose_owners()
    assert {owner.split("=")[0] for owner in owners} == {"server", "migrate", "postgres", "backup"}


def test_guides_have_a_command() -> None:
    # disaster-recovery.md case B runs it with --adopt-source; the test must see that one.
    assert any(name == "disaster-recovery.md" for name, _ in guide_commands())


def test_guide_commands_name_the_compose_owners() -> None:
    want = compose_owners()
    for name, command in guide_commands():
        assert set(OWNER.findall(command)) == want, f"{name}: {command}"
