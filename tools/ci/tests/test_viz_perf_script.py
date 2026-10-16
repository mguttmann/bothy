# SPDX-License-Identifier: Apache-2.0
"""The performance script of the visualization harness (brief M6-12, design note 6).

`packages/bothy_viz/harness/tool/perf_macos.sh` makes the macOS folder with `flutter create`,
runs `flutter drive` and then removes everything it added, because a native folder under
`packages/` fails rule 4 of `make arch`. A fake `flutter` stands in for the SDK: `create` writes
the files the real command writes, `drive` exits with `FAKE_DRIVE_EXIT`.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
SCRIPT = REPO / "packages" / "bothy_viz" / "harness" / "tool" / "perf_macos.sh"

FAKE = """#!{python}
import os, sys
args = sys.argv[1:]
if args[:1] == ["create"]:
    os.makedirs("macos/Runner", exist_ok=True)
    open("macos/Runner/AppDelegate.swift", "w").write("// native")
    os.makedirs("test", exist_ok=True)
    open("test/widget_test.dart", "w").write("void main() {{}}")
    for name in ("pubspec.lock", ".metadata", ".gitignore", "bothy_viz_harness.iml"):
        open(name, "w").write("x")
    os.makedirs(".idea", exist_ok=True)
    with open("analysis_options.yaml", "a") as fh:
        fh.write("analyzer:\\n  exclude:\\n    - macos/**\\n")
    sys.exit(0)
if args[:1] == ["drive"]:
    open(".flutter-plugins-dependencies", "w").write("{{}}")
    open("drive.args", "w").write(" ".join(args))
    sys.exit(int(os.environ.get("FAKE_DRIVE_EXIT", "0")))
sys.exit(2)
"""


@pytest.fixture
def harness(tmp_path: Path) -> Path:
    root = tmp_path / "harness"
    (root / "tool").mkdir(parents=True)
    (root / "test").mkdir()
    (root / "test" / "harness_test.dart").write_text("void main() {}\n", encoding="utf-8")
    (root / "pubspec.yaml").write_text("name: bothy_viz_harness\n", encoding="utf-8")
    (root / "analysis_options.yaml").write_text("include: shared.yaml\n", encoding="utf-8")
    shutil.copy(SCRIPT, root / "tool" / "perf_macos.sh")
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    fake = bin_dir / "flutter"
    fake.write_text(FAKE.format(python=sys.executable), encoding="utf-8")
    fake.chmod(0o755)
    return root


def _run(root: Path, **env: str) -> subprocess.CompletedProcess[str]:
    environ = dict(os.environ)
    environ["PATH"] = f"{root.parent / 'bin'}{os.pathsep}{environ['PATH']}"
    environ.update(env)
    return subprocess.run(
        ["sh", str(root / "tool" / "perf_macos.sh")],
        env=environ,
        capture_output=True,
        text=True,
        check=False,
        timeout=60,
    )


def _tree(root: Path) -> set[str]:
    return {p.relative_to(root).as_posix() for p in root.rglob("*")}


def test_runs_the_profile_drive_and_removes_what_it_added(harness: Path) -> None:
    before = _tree(harness)
    result = _run(harness)
    assert result.returncode == 0, result.stdout + result.stderr
    args = (harness / "drive.args").read_text(encoding="utf-8")
    assert "--profile -d macos" in args
    assert "--target=integration_test/topology_perf_test.dart" in args
    (harness / "drive.args").unlink()
    assert _tree(harness) == before


def test_restores_the_analysis_options_that_create_edits(harness: Path) -> None:
    result = _run(harness, FAKE_DRIVE_EXIT="1")
    assert result.returncode == 1
    assert (harness / "analysis_options.yaml").read_text(encoding="utf-8") == (
        "include: shared.yaml\n"
    )


def test_keeps_the_exit_code_of_the_drive_and_still_cleans_up(harness: Path) -> None:
    before = _tree(harness)
    result = _run(harness, FAKE_DRIVE_EXIT="3")
    assert result.returncode == 3
    (harness / "drive.args").unlink()
    assert _tree(harness) == before


def test_never_removes_a_file_that_was_there_before(harness: Path) -> None:
    (harness / ".gitignore").write_text("mine\n", encoding="utf-8")
    result = _run(harness)
    assert result.returncode == 0, result.stdout + result.stderr
    assert (harness / ".gitignore").read_text(encoding="utf-8") == "x"
