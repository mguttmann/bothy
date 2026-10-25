# SPDX-License-Identifier: Apache-2.0
"""The documentation gate in code bites (brief M2-50 Design note 21, "Documentation in code").

A copy of ``pyproject.toml`` and ``jira_app/`` in a temporary folder is checked with the ruff of
this environment and the copy's configuration: a public function without a docstring fails with
``D103`` naming the file, and the unchanged copy passes.
"""

from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path

import pytest

PROJECT = Path(__file__).resolve().parent.parent
# Every module of the app names its public API in ``__all__``; pydocstyle then treats only those
# names as public, so the new function is exported as a real addition would be.
UNDOCUMENTED = (
    '\n\ndef undocumented_helper() -> int:\n    return 1\n\n\n__all__ += ["undocumented_helper"]\n'
)


def _ruff() -> str:
    """The ruff next to this interpreter, so the pinned version runs; else the one on PATH."""
    candidate = Path(sys.executable).parent / "ruff"
    if candidate.is_file():
        return str(candidate)
    found = shutil.which("ruff")
    assert found is not None, "ruff of this environment not found"
    return found


@pytest.fixture
def copy(tmp_path: Path) -> Path:
    shutil.copy2(PROJECT / "pyproject.toml", tmp_path / "pyproject.toml")
    shutil.copytree(
        PROJECT / "jira_app",
        tmp_path / "jira_app",
        ignore=shutil.ignore_patterns("__pycache__"),
    )
    return tmp_path


def _check(root: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(  # noqa: S603 - fixed arguments, the environment's own ruff
        [
            _ruff(),
            "check",
            "--no-cache",
            "--output-format",
            "concise",
            "--config",
            str(root / "pyproject.toml"),
            str(root / "jira_app"),
        ],
        cwd=root,
        check=False,
        capture_output=True,
        text=True,
    )


def test_unchanged_copy_passes(copy: Path) -> None:
    result = _check(copy)
    assert result.returncode == 0, result.stdout + result.stderr


def test_public_function_without_docstring_fails_with_d103(copy: Path) -> None:
    module = copy / "jira_app" / "log.py"
    module.write_text(module.read_text(encoding="utf-8") + UNDOCUMENTED, encoding="utf-8")
    lines = module.read_text(encoding="utf-8").splitlines()
    def_line = lines.index("def undocumented_helper() -> int:") + 1

    result = _check(copy)

    output = result.stdout + result.stderr
    assert result.returncode != 0, output
    d103 = [line for line in output.splitlines() if "D103" in line]
    # Exactly the new function, in that file, and nothing else of the copy.
    assert len(d103) == 1, output
    assert f"jira_app/log.py:{def_line}:" in d103[0], output
