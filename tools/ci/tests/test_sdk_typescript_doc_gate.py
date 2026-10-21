# SPDX-License-Identifier: Apache-2.0
"""The doc gate of the TypeScript SDK fails on an undocumented export (D-M2-30-12).

`npm run lint` of `sdk/typescript` runs ESLint and then typedoc with `validation.notDocumented`
as an error (docs/spec/23-qualitaet.md "Documentation in code", PLAN row M2-30: "test with a
deliberately undocumented export"). Each test runs the script on a copy of the package without
`node_modules`, `dist`, `build` and `coverage`, linked to the real `node_modules`, so the working
tree stays untouched; the generated core `src/generated/` is outside the doc rule.
"""

from __future__ import annotations

import os
import shutil
import subprocess  # nosec B404 - runs npm and git with fixed arguments
from collections.abc import Iterator
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
SDK = REPO / "sdk" / "typescript"
UNDOCUMENTED = "export const undocumentedExample = 1;\n"
GENERATED_ONLY = "UndocumentedGeneratedExample"
# eslint over the whole package and typedoc over three entry points, on a cold cache.
TIMEOUT_S = 600


def _git_status() -> str:
    return subprocess.run(
        [
            "git",
            "-C",
            str(REPO),
            "status",
            "--porcelain",
            "--ignored",
            "--untracked-files=all",
            "--",
            "sdk/typescript",
        ],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


@pytest.fixture(scope="module", autouse=True)
def _tree_untouched() -> Iterator[None]:
    """The real working tree has the same status before and after the tests of this module."""
    before = _git_status()
    yield
    assert _git_status() == before


@pytest.fixture
def package(tmp_path: Path) -> Path:
    """A copy of the package that uses the real node_modules."""
    modules = SDK / "node_modules"
    assert modules.is_dir(), "sdk/typescript/node_modules is missing; run `make bootstrap-ci`"
    copy = tmp_path / "typescript"
    shutil.copytree(
        SDK, copy, ignore=shutil.ignore_patterns("node_modules", "dist", "build", "coverage")
    )
    (copy / "node_modules").symlink_to(modules, target_is_directory=True)
    return copy


def _lint(package: Path) -> subprocess.CompletedProcess[str]:
    npm = shutil.which("npm")
    assert npm, "npm is not on PATH"
    env = {**os.environ, "NO_COLOR": "1", "FORCE_COLOR": "0"}
    return subprocess.run(
        [npm, "run", "lint"],
        cwd=package,
        capture_output=True,
        text=True,
        timeout=TIMEOUT_S,
        env=env,
        check=False,
    )


def _append(path: Path, text: str) -> None:
    with path.open("a", encoding="utf-8") as handle:
        handle.write(text)


def test_unchanged_copy_passes(package: Path) -> None:
    result = _lint(package)
    assert result.returncode == 0, result.stdout + result.stderr


def test_undocumented_export_fails_and_is_named(package: Path) -> None:
    _append(package / "src" / "index.ts", UNDOCUMENTED)
    result = _lint(package)
    output = result.stdout + result.stderr
    assert result.returncode != 0
    assert "undocumentedExample" in output


def test_symbol_only_in_the_generated_core_is_not_reported(package: Path) -> None:
    _append(
        package / "src" / "generated" / "openapi.ts",
        f"export interface {GENERATED_ONLY} {{\n    value: string;\n}}\n",
    )
    _append(
        package / "src" / "index.ts",
        f'export type {{ {GENERATED_ONLY} }} from "./generated/openapi.js";\n',
    )
    result = _lint(package)
    output = result.stdout + result.stderr
    assert result.returncode == 0, output
    assert GENERATED_ONLY not in output
