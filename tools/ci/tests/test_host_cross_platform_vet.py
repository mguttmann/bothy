# SPDX-License-Identifier: Apache-2.0
"""Cross-platform vet of the host service (brief M2-45, "Repository tools"): the files of every
platform and their tests, the platform suite (build tag ``platform``) included, compile and pass
``go vet`` for macOS and Windows on any runner, so the code that only the throwaway platform
machines run cannot rot on the Linux runner.
"""

from __future__ import annotations

import os
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
HOST = REPO / "services" / "host"

_TARGETS = [("darwin", "arm64"), ("windows", "amd64")]


def _vet(goos: str, goarch: str, *tags: str) -> subprocess.CompletedProcess[str]:
    env = dict(os.environ)
    env.update({"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0", "GOTOOLCHAIN": "local"})
    args = ["go", "vet"]
    if tags:
        args += ["-tags", ",".join(tags)]
    return subprocess.run(
        [*args, "./..."],
        cwd=HOST,
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=900,
    )


@pytest.mark.parametrize(("goos", "goarch"), _TARGETS)
def test_host_vets_for_other_platforms(goos: str, goarch: str) -> None:
    result = _vet(goos, goarch)
    assert result.returncode == 0, result.stdout + result.stderr


@pytest.mark.parametrize(("goos", "goarch"), _TARGETS)
def test_platform_suite_vets_for_other_platforms(goos: str, goarch: str) -> None:
    result = _vet(goos, goarch, "platform")
    assert result.returncode == 0, result.stdout + result.stderr
