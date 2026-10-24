# SPDX-License-Identifier: Apache-2.0
"""The docstring gate of docs/spec/23-quality.md "Documentation in code" bites."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

PYPROJECT = Path(__file__).resolve().parents[1] / "pyproject.toml"


def _ruff(path: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [
            sys.executable,
            "-m",
            "ruff",
            "check",
            "--no-cache",
            "--config",
            str(PYPROJECT),
            str(path),
        ],
        capture_output=True,
        text=True,
        check=False,
    )


def test_undocumented_public_function_fails(tmp_path: Path) -> None:
    module = tmp_path / "sample.py"
    module.write_text('"""A module."""\n\n\ndef public(value: int) -> int:\n    return value\n')
    result = _ruff(module)
    assert result.returncode != 0
    assert "D103" in result.stdout


def test_documented_public_function_passes(tmp_path: Path) -> None:
    module = tmp_path / "sample.py"
    module.write_text(
        '"""A module."""\n\n\ndef public(value: int) -> int:\n'
        '    """Return the value.\n\n    Args:\n        value: Any number.\n\n'
        '    Returns:\n        The same number.\n    """\n    return value\n'
    )
    result = _ruff(module)
    assert result.returncode == 0, result.stdout
