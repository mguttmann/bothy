# SPDX-License-Identifier: Apache-2.0
"""The console script of the README is installed (pyproject ``[project.scripts]``)."""

from __future__ import annotations

from importlib.metadata import entry_points


def test_console_script_installed() -> None:
    scripts = {e.name: e.value for e in entry_points(group="console_scripts")}
    assert scripts.get("claude-code-runner") == "claude_code_runner.__main__:main"
