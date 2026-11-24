# SPDX-License-Identifier: Apache-2.0
"""tools/README.md names every tool (its closing rule: a make target and a row per tool).

A folder of `tools/` counts as one tool when the table names it (`<folder>/`); otherwise every
script directly in it needs its own row. Tests, `__init__.py` and `conftest.py` are no tools.
"""

from __future__ import annotations

import subprocess
from pathlib import Path, PurePosixPath

REPO = Path(__file__).resolve().parents[3]
README = REPO / "tools" / "README.md"


def _tracked_tools() -> list[PurePosixPath]:
    out = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "--", "tools"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [PurePosixPath(line).relative_to("tools") for line in out.splitlines() if line]


def test_every_tool_has_a_row() -> None:
    text = README.read_text(encoding="utf-8")
    missing: set[str] = set()
    for path in _tracked_tools():
        if len(path.parts) < 2 or "tests" in path.parts:
            continue
        folder = path.parts[0]
        if f"`{folder}/" not in text:
            missing.add(f"{folder}/")
            continue
        if f"`{folder}/`" in text or path.suffix != ".py" or len(path.parts) != 2:
            continue
        if path.name not in {"__init__.py", "conftest.py"} and f"`{path}`" not in text:
            missing.add(str(path))
    assert sorted(missing) == []
