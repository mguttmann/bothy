# SPDX-License-Identifier: Apache-2.0
"""The doc gate of the GitHub app fails on an undocumented export (brief M2-49 Design note 20).

`npm run lint` of `plugins/github-app` runs ESLint and then typedoc with
`validation.notDocumented` as an error over every module of `src/` (docs/spec/23-quality.md
"Documentation in code"). Each test runs the script on a copy of the app without `node_modules`,
`dist` and `coverage`, linked to the real `node_modules`, inside a copy of the repository layout
whose `sdk/typescript` links to the real SDK (the script builds it first), so the working tree
stays untouched. The steps are those of `test_sdk_typescript_doc_gate.py` (M2-30).
"""

from __future__ import annotations

import os
import shutil
import subprocess  # nosec B404 - runs npm and git with fixed arguments
from collections.abc import Iterator
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
APP = REPO / "plugins" / "github-app"
SDK = REPO / "sdk" / "typescript"
UNDOCUMENTED = "export const undocumentedExample = 1;\n"
# eslint over the whole app, the SDK build and typedoc over every module, on a cold cache.
TIMEOUT_S = 600


def _git_status() -> str:
    return subprocess.run(
        [
            "git",
            "-C",
            str(REPO),
            "status",
            "--porcelain",
            "--untracked-files=all",
            "--",
            "plugins/github-app",
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
def app(tmp_path: Path) -> Path:
    """A copy of the app that uses the real node_modules and the real SDK."""
    modules = APP / "node_modules"
    assert modules.is_dir(), "plugins/github-app/node_modules is missing; run `make bootstrap-ci`"
    copy = tmp_path / "plugins" / "github-app"
    shutil.copytree(APP, copy, ignore=shutil.ignore_patterns("node_modules", "dist", "coverage"))
    (copy / "node_modules").symlink_to(modules, target_is_directory=True)
    (tmp_path / "sdk").mkdir()
    (tmp_path / "sdk" / "typescript").symlink_to(SDK, target_is_directory=True)
    return copy


def _lint(app: Path) -> subprocess.CompletedProcess[str]:
    npm = shutil.which("npm")
    assert npm, "npm is not on PATH"
    env = {**os.environ, "NO_COLOR": "1", "FORCE_COLOR": "0"}
    return subprocess.run(
        [npm, "run", "lint"],
        cwd=app,
        capture_output=True,
        text=True,
        timeout=TIMEOUT_S,
        env=env,
        check=False,
    )


def test_unchanged_copy_passes(app: Path) -> None:
    result = _lint(app)
    assert result.returncode == 0, result.stdout + result.stderr


def test_undocumented_export_fails_and_is_named(app: Path) -> None:
    with (app / "src" / "app.ts").open("a", encoding="utf-8") as handle:
        handle.write(UNDOCUMENTED)
    result = _lint(app)
    output = result.stdout + result.stderr
    assert result.returncode != 0
    assert "undocumentedExample" in output
