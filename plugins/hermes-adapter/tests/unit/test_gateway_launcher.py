# SPDX-License-Identifier: Apache-2.0
"""The Fake-Hermes gateway runs Hermes' own CLI entry with its plugin discovery on the main thread.

Hermes ``v0.22`` starts its plugin discovery in a background thread while the main thread still
imports the gateway; on a loaded machine the overlapping first imports of its own modules end in
a ``KeyError`` of the import system (``gateway.platforms.base_exec_approval``) and the gateway
exits before any platform starts (docs/decisions.d/FIX-main-fake-hermes-v022.md). A stand-in
``hermes_cli`` package shows what the launcher runs.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import fake_hermes_kit

MAIN = """\
import json, sys, threading

def main():
    from hermes_cli.plugins import start_background_plugin_discovery
    start_background_plugin_discovery()
    print(json.dumps({"argv": sys.argv, "threads": sorted(t.name for t in threading.enumerate())}))
    return 7
"""

PLUGINS = """\
import threading

def start_background_plugin_discovery():
    threading.Thread(target=lambda: None, name="plugin-discovery").start()
    raise AssertionError("the background discovery ran")
"""


def _hermes(tmp_path: Path, plugins: str) -> tuple[Path, Path]:
    package = tmp_path / "src" / "hermes_cli"
    package.mkdir(parents=True)
    (package / "__init__.py").write_text("", encoding="utf-8")
    (package / "main.py").write_text(MAIN, encoding="utf-8")
    (package / "plugins.py").write_text(plugins, encoding="utf-8")
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    return tmp_path / "src", bin_dir


def _run(src: Path, launcher: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(launcher), "gateway", "run", "--force"],
        capture_output=True,
        text=True,
        check=False,
        env={"PATH": "/usr/bin:/bin", "PYTHONPATH": str(src)},
    )


def test_launcher_runs_hermes_main_without_the_discovery_thread(tmp_path: Path) -> None:
    src, bin_dir = _hermes(tmp_path, PLUGINS)
    launcher = fake_hermes_kit.write_gateway_launcher(bin_dir)
    done = _run(src, launcher)
    assert done.returncode == 7, done.stderr
    seen = json.loads(done.stdout)
    # Hermes sees its own executable and the arguments unchanged.
    assert seen["argv"] == [str(bin_dir / "hermes"), "gateway", "run", "--force"]
    assert "plugin-discovery" not in seen["threads"]


def test_launcher_fails_when_hermes_renames_the_discovery_start(tmp_path: Path) -> None:
    src, bin_dir = _hermes(tmp_path, "def discover_plugins():\n    pass\n")
    done = _run(src, fake_hermes_kit.write_gateway_launcher(bin_dir))
    assert done.returncode != 0
    assert "start_background_plugin_discovery" in done.stderr
    assert done.stdout == ""
