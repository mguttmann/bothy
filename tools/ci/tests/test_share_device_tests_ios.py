# SPDX-License-Identifier: Apache-2.0
"""The iOS share device tests run patrol with `--verbose`.

Without it patrol shows only `** TEST BUILD FAILED **` of a failed `xcodebuild build-for-testing`
(main run 40567821515, `apps (iOS)`): the compiler and linker errors go to its detail log. The test
runs the real script with a fake `xcrun` and a fake `patrol` on PATH and records the arguments.
"""

from __future__ import annotations

import os
import stat
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
SCRIPT = REPO / "apps/client/tool/run_share_device_tests.sh"

_XCRUN = """#!/bin/sh
case "$*" in
"simctl list devices"*)
  echo "    iPhone 16 Pro (0A1B2C3D-0000-4000-8000-00000000000A) (Booted)" ;;
esac
exit 0
"""

_PATROL = """#!/bin/sh
printf '%s\\n' "$*" >>"$PATROL_ARGS"
"""


def _tool(folder: Path, name: str, body: str) -> None:
    path = folder / name
    path.write_text(body, encoding="utf-8")
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def test_every_ios_patrol_run_is_verbose(tmp_path: Path) -> None:
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    _tool(bin_dir, "xcrun", _XCRUN)
    _tool(bin_dir, "patrol", _PATROL)
    args = tmp_path / "patrol-args.txt"
    env = {**os.environ, "PATH": f"{bin_dir}{os.pathsep}{os.environ['PATH']}"}
    env["PATROL_ARGS"] = str(args)
    subprocess.run(["sh", str(SCRIPT), "ios"], env=env, check=True, capture_output=True)
    runs = args.read_text(encoding="utf-8").splitlines()
    assert len(runs) == 2, runs
    for run in runs:
        assert run.startswith("--verbose test "), run
