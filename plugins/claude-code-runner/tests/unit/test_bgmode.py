# SPDX-License-Identifier: Apache-2.0
"""What the background turns of one runner process share (09 18.2 "Background mode", 18.9)."""

from __future__ import annotations

import dataclasses
import re
from pathlib import Path
from uuid import UUID

from claude_code_runner.bgmode import (
    FALLBACK_CODES,
    UNAVAILABLE,
    BackgroundMode,
    launch_environment,
)
from claude_code_runner.causes import CAUSES
from claude_code_runner.config import load_config
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION

from .test_config import write

ROOT = Path(__file__).resolve().parents[2]
SPEC = ROOT.parents[1] / "docs" / "spec" / "09-agents.md"
README = ROOT / "README.md"
LISTED = UUID("0192d3a0-0000-7000-8000-0000000000c1")
OTHER = UUID("0192d3a0-0000-7000-8000-0000000000c2")
ENVIRON = {"HOME": "/home/runner", "PATH": "/usr/bin:/bin"}


def mode(
    tmp_path: Path, version: str | None = PINNED_CLAUDE_CODE_VERSION, **keys: object
) -> tuple[BackgroundMode, list[str]]:
    """A mode over the default configuration with ``keys`` replaced, and its change calls."""
    config = dataclasses.replace(load_config(write(tmp_path)), **keys)  # type: ignore[arg-type]
    changes: list[str] = []
    built = BackgroundMode(
        config,
        "/usr/bin/claude",
        ENVIRON,
        on_change=lambda: changes.append("beat"),
        version=lambda: version,
    )
    return built, changes


def test_the_mode_is_wanted_only_with_the_key_and_in_a_listed_channel(tmp_path: Path) -> None:
    off, _ = mode(tmp_path)
    assert off.wants(LISTED) is False  # the default is print mode
    everywhere, _ = mode(tmp_path, launch_mode="background")
    assert everywhere.wants(LISTED) is True
    assert everywhere.wants(OTHER) is True
    listed, _ = mode(tmp_path, launch_mode="background", background_channels=(LISTED,))
    assert listed.wants(LISTED) is True
    assert listed.wants(OTHER) is False


def test_the_mode_runs_only_on_the_pinned_version(tmp_path: Path) -> None:
    assert mode(tmp_path)[0].pinned() is True
    assert mode(tmp_path, version="2.1.259")[0].pinned() is False
    assert mode(tmp_path, version=None)[0].pinned() is False


def test_a_cause_of_the_host_shows_until_the_next_launch_starts(tmp_path: Path) -> None:
    built, changes = mode(tmp_path, launch_mode="background")
    assert built.unavailable is None
    built.fell_back("background_workspace_untrusted")  # a cause of the folder, not of the host
    assert (built.unavailable, changes) == (None, [])
    built.fell_back("background_supervisor_foreign")
    assert (built.unavailable, changes) == ("background_supervisor_foreign", ["beat"])
    built.fell_back("background_launch_failed")  # still unavailable: no second heartbeat
    assert (built.unavailable, changes) == ("background_launch_failed", ["beat"])
    built.launched()
    assert (built.unavailable, changes) == (None, ["beat", "beat"])
    built.launched()
    assert changes == ["beat", "beat"]


def test_every_command_of_the_mode_keeps_the_cli_on_its_version(tmp_path: Path) -> None:
    assert launch_environment({"PATH": "/usr/bin"}) == {
        "PATH": "/usr/bin",
        "DISABLE_AUTOUPDATER": "1",
    }
    built, _ = mode(tmp_path, launch_mode="background")
    assert built.env["DISABLE_AUTOUPDATER"] == "1"
    assert "ANTHROPIC_API_KEY" not in built.env
    assert built.config_dir == Path("/home/runner/.claude")


def _rows(text: str, heading: str) -> dict[str, list[str]]:
    """The rows of the first table after ``heading`` whose first cell is one code."""
    rows: dict[str, list[str]] = {}
    for line in text[text.index(heading) :].splitlines():
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        found = re.fullmatch(r"`(background_[a-z_]+)`", cells[0])
        if found is not None:
            rows[found.group(1)] = cells[1:]
        elif rows and not line.startswith("|"):
            break
    return rows


def test_the_spec_and_the_readme_list_exactly_the_fallback_codes() -> None:
    spec = _rows(SPEC.read_text(encoding="utf-8"), "Background mode is unavailable")
    assert set(spec) == set(FALLBACK_CODES)
    assert {code: cells[-1] == "yes" for code, cells in spec.items()} == FALLBACK_CODES
    readme = _rows(README.read_text(encoding="utf-8"), "When the mode is not available")
    assert set(readme) == set(FALLBACK_CODES)
    assert f"`{UNAVAILABLE}`" in SPEC.read_text(encoding="utf-8")


def test_a_code_that_can_end_a_run_has_a_sentence_for_the_thread() -> None:
    # A launch in doubt ends the run with its code as the cause (18.9).
    assert {"background_launch_failed", "background_supervisor_foreign"} <= set(CAUSES)
