# SPDX-License-Identifier: Apache-2.0
"""`make test` runs the tests of infra/compose/install.sh with the repository-wide area only.

The setup wizard is no component: its tests belong to the area `tools`, like the other
repository-wide checks. Before, every `make test` ended with them, so a full CI run started the
container of `make install-test` in every test job (the server shards, `test (apps)`,
`test (SDKs and plugins)` and `test (tools)`). Now `test (tools)` runs them once, and the
pre-push gate runs them whenever the push changes a file that is not source code of a
component (ci-gates-17; docs/quality/gates.md, "List").
"""

from __future__ import annotations

import shutil
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
# The command of the recipe of install-test, which `make -n` prints when `make test` reaches it.
INSTALL_RUN = "sh /src/infra/compose/test/install/run.sh"


def _dry_run(*args: str) -> str:
    make = shutil.which("make")
    assert make
    return subprocess.run(
        [make, "-n", "--no-print-directory", "test", *args],
        cwd=REPO,
        check=True,
        capture_output=True,
        text=True,
        env={"PATH": "/usr/bin:/bin", "INSTALL_TEST_NATIVE": "0"},
    ).stdout


@pytest.mark.parametrize(
    "args",
    [
        ("COMPONENTS=server", "AREAS=none"),
        ("COMPONENTS=apps", "AREAS=none"),
        ("COMPONENTS=sdk-python sdk-typescript plugins", "AREAS=none"),
    ],
)
def test_a_component_test_job_leaves_the_install_tests_out(args: tuple[str, ...]) -> None:
    assert INSTALL_RUN not in _dry_run(*args)


@pytest.mark.parametrize("args", [("COMPONENTS=none",), ("COMPONENTS=none", "AREAS=tools"), ()])
def test_the_tools_area_runs_the_install_tests(args: tuple[str, ...]) -> None:
    assert INSTALL_RUN in _dry_run(*args)
