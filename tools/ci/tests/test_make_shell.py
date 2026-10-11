# SPDX-License-Identifier: Apache-2.0
"""The root Makefile runs every recipe line with `bash -euo pipefail`, under every GNU make.

GNU make 3.81 (macOS) ignores .SHELLFLAGS; for it the Makefile puts the flags into SHELL
itself. The probes run the `make` that comes first on PATH: a second makefile, read after
the root Makefile, adds targets that run with the root Makefile's shell settings. Each probe
line prints a start marker first, so a probe that never ran cannot pass.
"""

from __future__ import annotations

import os
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
_UNSET = "BOTHY_MAKE_UNSET_PROBE"
# The test may run under `make test`; its make variables must not reach the probe runs.
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", _UNSET}

PROBES = {
    "errexit": "x=$$(false); echo errexit-off",
    "pipefail": "false | true; echo pipefail-off",
    "nounset": f'echo "$${_UNSET}"; echo nounset-off',
}


def _probe(tmp_path: Path, name: str, line: str) -> subprocess.CompletedProcess[str]:
    probe = tmp_path / "probe.mk"
    probe.write_text(f"probe-{name}:\n\t@echo probe-started; {line}\n", encoding="utf-8")
    env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
    return subprocess.run(
        ["make", "-s", "-C", str(REPO), "-f", "Makefile", "-f", str(probe), f"probe-{name}"],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )


@pytest.mark.parametrize(("name", "line"), PROBES.items(), ids=list(PROBES))
def test_recipe_lines_stop_on_a_failure(tmp_path: Path, name: str, line: str) -> None:
    result = _probe(tmp_path, name, line)
    assert "probe-started" in result.stdout, result.stderr
    assert result.returncode != 0
    assert f"{name}-off" not in result.stdout


def test_the_probe_passes_without_a_failure(tmp_path: Path) -> None:
    result = _probe(tmp_path, "control", "echo control-ran")
    assert result.returncode == 0, result.stderr
    assert result.stdout == "probe-started\ncontrol-ran\n"
