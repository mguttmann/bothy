# SPDX-License-Identifier: Apache-2.0
"""A public function of the Python SDK without a docstring fails `make lint` (R-QA-14, rule 3).

The proof has two steps and runs neither the network-bound nor the repository-wide parts of
`make lint`. First the dry run `make -n lint COMPONENTS=sdk-python` shows that the recipe runs
`ruff check` over every project of `PY_PROJECTS`, `sdk/python` included. Then the ruff of the
SDK's locked environment checks a copy of `pyproject.toml` and `src/` of `sdk/python` with the
copy's configuration, so nothing is written into the real tree: with an undocumented public
function it fails with `D103`, unmodified it passes. Every module of the hand layer declares
`__all__`, and pydocstyle counts only the names listed there as public, so the added function
is listed too.
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess  # nosec B404 - runs make and uv with fixed argument lists
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
SDK = REPO / "sdk" / "python"
# A public module of the hand layer; any public module would do.
MODULE = Path("src") / "bothy_sdk" / "ids.py"
# The test may run under `make test`; its make variables must not leak into the dry run.
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "COMPONENTS"}
# uv runs the SDK's own environment, not one that runs this test.
_DROPPED |= {"VIRTUAL_ENV"}
_PY_LINT = re.compile(
    r'if \[ -z "(?P<on>[^"]*)" \]; then [^\n]*py-lint[^\n]*\n'
    r"\s*else for p in (?P<projects>[^;]+); do uv run --project \$p --locked ruff check \$p "
)


def _env() -> dict[str, str]:
    return {k: v for k, v in os.environ.items() if k not in _DROPPED}


def py_projects() -> list[str]:
    """The Python projects the Makefile collects into `PY_PROJECTS`, from the same pattern."""
    listed = subprocess.run(  # nosec B603 B607 - fixed argument list
        ["git", "ls-files", "--", "sdk/python/pyproject.toml", "plugins/*pyproject.toml"],
        cwd=REPO,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.split()
    return sorted(str(Path(p).parent) for p in listed)


def sdk_copy(tmp_path: Path) -> Path:
    """`pyproject.toml` and `src/` of the SDK in a temporary directory."""
    copy = tmp_path / "python"
    copy.mkdir()
    shutil.copy(SDK / "pyproject.toml", copy / "pyproject.toml")
    shutil.copytree(SDK / "src", copy / "src", ignore=shutil.ignore_patterns("__pycache__"))
    return copy


def ruff_check(copy: Path) -> subprocess.CompletedProcess[str]:
    """Ruff of the SDK's locked environment over the copy, with the copy's configuration."""
    return subprocess.run(  # nosec B603 B607 - fixed argument list
        ["uv", "run", "--project", str(SDK), "--locked", "ruff", "check", "."],
        cwd=copy,
        env=_env(),
        capture_output=True,
        text=True,
        check=False,
    )


def test_public_function_without_docstring_fails_make_lint(tmp_path: Path) -> None:
    dry = subprocess.run(  # nosec B603 B607 - fixed argument list
        ["make", "-n", "lint", "COMPONENTS=sdk-python"],
        cwd=REPO,
        env=_env(),
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    match = _PY_LINT.search(dry)
    assert match, "the py-lint step of make lint runs no ruff check over PY_PROJECTS"
    assert match.group("on"), "COMPONENTS=sdk-python does not select the Python projects"
    assert "sdk/python" in py_projects()
    assert match.group("projects").split() == py_projects()

    copy = sdk_copy(tmp_path)
    with (copy / MODULE).open("a", encoding="utf-8") as fh:
        fh.write(
            '\n\n__all__ += ["undocumented_helper"]\n\n\ndef undocumented_helper() -> None: ...\n'
        )
    result = ruff_check(copy)
    assert result.returncode != 0
    # The concise form names the file before the code, the full form (ruff's default) after it.
    path = re.escape(MODULE.as_posix())
    found = rf"{path}:\d+:\d+: D103\b|\bD103\b[^\n]*\n\s*--> {path}:\d+:\d+"
    assert re.search(found, result.stdout), result.stdout + result.stderr


def test_documented_copy_passes_make_lint(tmp_path: Path) -> None:
    result = ruff_check(sdk_copy(tmp_path))
    assert result.returncode == 0, result.stdout + result.stderr
