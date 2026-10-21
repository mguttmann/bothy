# SPDX-License-Identifier: Apache-2.0
"""Install Fake-Claude as an executable named ``claude`` for one test."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

FAKE = Path(__file__).resolve().parent / "fake_claude.py"


def install(directory: Path, scenario: dict[str, Any], capture: Path, name: str = "claude") -> Path:
    """Write the scenario and a wrapper with the test interpreter and the paths baked in.

    Returns:
        The wrapper (mode 0755, in ``directory``, which should be on the runner's PATH).
    """
    directory.mkdir(parents=True, exist_ok=True)
    directory.chmod(0o755)
    scenario_path = directory / f"{name}.scenario.json"
    scenario_path.write_text(json.dumps(scenario))
    wrapper = directory / name
    wrapper.write_text(
        f"#!{sys.executable}\n"
        "import sys\n"
        f"sys.path.insert(0, {str(FAKE.parent)!r})\n"
        "import fake_claude\n"
        f"sys.exit(fake_claude.main({str(scenario_path)!r}, {str(capture)!r}))\n"
    )
    wrapper.chmod(0o755)
    return wrapper


def update(wrapper: Path, scenario: dict[str, Any]) -> None:
    """Replace the scenario of an installed wrapper (the next call uses it)."""
    (wrapper.parent / f"{wrapper.name}.scenario.json").write_text(json.dumps(scenario))


def read_capture(capture: Path) -> list[dict[str, Any]]:
    """Return the recorded calls in order (empty when nothing ran)."""
    if not capture.exists():
        return []
    return [json.loads(line) for line in capture.read_text().splitlines() if line.strip()]


def turns(capture: Path) -> list[dict[str, Any]]:
    """Return the ``turn_start`` records, each with the stdin lines of its process."""
    records = read_capture(capture)
    out = []
    for start in (r for r in records if r["kind"] == "turn_start"):
        lines = [r["line"] for r in records if r["kind"] == "stdin" and r["pid"] == start["pid"]]
        out.append({**start, "stdin": lines})
    return out
