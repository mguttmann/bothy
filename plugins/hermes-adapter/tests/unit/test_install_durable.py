# SPDX-License-Identifier: Apache-2.0
"""A self-contained plugin directory: the install survives a new Hermes environment (09 17.2).

Hermes replaces the Python environment of a profile on many CLI commands; whatever the installer
put into one environment (a ``.pth`` file, ``attrs``) is gone in the next one. These tests hold
the plugin directory self-contained: it names where each package lives, copies what lay inside
the installing environment, loads the packages without the environment's help and names a
missing package in the gateway log; ``check`` reports it.
"""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from adapter_kit import MemoryBridge

from bothy_hermes_adapter.__main__ import copy_package, locate, place_file
from bothy_hermes_adapter.installer import (
    ADAPTER_LOADABLE,
    ADAPTER_NOT_LOADABLE,
    PACKAGES_FILE,
    InstallError,
    InstallRequest,
    PackageSource,
    check,
    install,
    loadable,
    locate_packages,
)

INSTANCE = UUID("0192f4a0-7b1c-7000-8000-000000000001")


def _request() -> InstallRequest:
    return InstallRequest(
        server_url="https://example.com/api/v1",
        instance_id=INSTANCE,
        token_file=Path("/srv/hermes/token"),
        busy_input_mode="steer",
    )


def _fake_tree(root: Path, name: str) -> Path:
    directory = root / name
    directory.mkdir(parents=True)
    (directory / "__init__.py").write_text(f"NAME = {name!r}\n", encoding="utf-8")
    return directory


def test_locate_packages_copies_the_environment_and_the_added_packages(tmp_path: Path) -> None:
    environment = tmp_path / "env"
    sources = tmp_path / "checkout"
    found = {
        "bothy_hermes_adapter": _fake_tree(sources, "bothy_hermes_adapter"),
        "bothy_sdk": _fake_tree(environment / "site", "bothy_sdk"),
        "attr": _fake_tree(environment / "site", "attr"),
        "attrs": _fake_tree(tmp_path / "deps", "attrs"),
    }
    placed = {p.name: p for p in locate_packages(found.get, environment)}
    assert set(placed) == set(found)
    assert (placed["bothy_hermes_adapter"].copy, placed["bothy_hermes_adapter"].first) == (
        False,
        True,
    )
    assert (placed["bothy_sdk"].copy, placed["bothy_sdk"].first) == (True, True)
    assert (placed["attr"].copy, placed["attr"].first) == (True, False)
    # An added package is copied wherever it lay: its directory need not outlive the install.
    assert (placed["attrs"].copy, placed["attrs"].first) == (True, False)


def test_locate_packages_refuses_a_missing_package(tmp_path: Path) -> None:
    found = {"bothy_hermes_adapter": _fake_tree(tmp_path, "bothy_hermes_adapter")}
    with pytest.raises(InstallError, match="bothy_sdk"):
        locate_packages(found.get, tmp_path / "env")


def _installed(tmp_path: Path) -> tuple[MemoryBridge, Path]:
    """Install with the packages of this test process; the test environment is copied."""
    home = tmp_path / "home"
    bridge = MemoryBridge(home)
    packages = locate_packages(locate, Path(sys.prefix))
    install(bridge, _request(), place_file, packages, copy_package)
    return bridge, home / "plugins" / "bothy"


def test_install_writes_a_self_contained_plugin_directory(tmp_path: Path) -> None:
    bridge, plugin = _installed(tmp_path)
    assert {p.name for p in plugin.iterdir()} >= {"plugin.yaml", "__init__.py", PACKAGES_FILE}
    manifest = json.loads((plugin / PACKAGES_FILE).read_text(encoding="utf-8"))
    assert set(manifest["first"]) == {"bothy_hermes_adapter", "bothy_sdk"}
    assert set(manifest["last"]) == {"attr", "attrs"}
    # attrs lies in this test's environment: copied, named relative to the plugin directory.
    assert manifest["last"]["attrs"] == "vendor/attrs"
    assert (plugin / "vendor" / "attrs" / "__init__.py").is_file()
    assert not list((plugin / "vendor").rglob("__pycache__"))
    # The source trees lie outside the environment: named by their absolute directory.
    assert Path(manifest["first"]["bothy_sdk"]).is_absolute()
    # A second install replaces the copy instead of failing on it.
    install(
        bridge,
        _request(),
        place_file,
        locate_packages(locate, Path(sys.prefix)),
        copy_package,
    )


_LOAD = """
import importlib.util, sys
from pathlib import Path
plugin = Path(sys.argv[1])
spec = importlib.util.spec_from_file_location(
    "hermes_plugins.under_test", plugin / "__init__.py", submodule_search_locations=[str(plugin)])
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)
calls = []
class Ctx:
    def register_platform(self, **kw): calls.append(kw["name"])
    def register_hook(self, name, cb): calls.append(name)
module.register(Ctx())
import attrs
print(calls[0])
print(Path(attrs.__file__).resolve().is_relative_to(plugin.resolve()))
print(importlib.util.find_spec("bothy_sdk").origin)
"""


def _load(plugin: Path) -> subprocess.CompletedProcess[str]:
    # -S: no site-packages and no .pth file of any environment; only the plugin directory.
    return subprocess.run(
        [sys.executable, "-S", "-c", _LOAD, str(plugin)],
        capture_output=True,
        text=True,
        check=False,
        env={"PATH": "/usr/bin:/bin"},
    )


def test_plugin_loads_its_packages_without_the_environment(tmp_path: Path) -> None:
    _, plugin = _installed(tmp_path)
    done = _load(plugin)
    assert done.returncode == 0, done.stderr
    name, attrs_inside, sdk_origin = done.stdout.split()
    assert name == "bothy"
    assert attrs_inside == "True"
    sdk = json.loads((plugin / PACKAGES_FILE).read_text(encoding="utf-8"))["first"]["bothy_sdk"]
    assert Path(sdk_origin) == Path(sdk) / "__init__.py"


def test_plugin_names_a_missing_package_in_the_log(tmp_path: Path) -> None:
    _, plugin = _installed(tmp_path)
    shutil.rmtree(plugin / "vendor" / "attrs")
    done = _load(plugin)
    assert done.returncode != 0
    assert "ImportError" in done.stderr
    assert "attrs" in done.stderr
    assert "install" in done.stderr


def test_plugin_installed_without_a_package_list_names_the_fix(tmp_path: Path) -> None:
    _, plugin = _installed(tmp_path)
    (plugin / PACKAGES_FILE).unlink()
    done = _load(plugin)
    assert done.returncode != 0
    assert "install" in done.stderr


def _available(missing: set[str]) -> Any:
    return lambda name: name not in missing


def test_check_reports_a_loadable_and_a_missing_adapter(tmp_path: Path) -> None:
    bridge, plugin = _installed(tmp_path)
    lines = check(bridge, _available(set()))
    assert ADAPTER_LOADABLE in lines
    assert loadable(lines)
    shutil.rmtree(plugin / "vendor" / "attrs")
    lines = check(bridge, _available({"httpx"}))
    problem = next(line for line in lines if line.startswith(ADAPTER_NOT_LOADABLE))
    assert "attrs" in problem
    assert "httpx" in problem
    assert not loadable(lines)
    # No value in the output: no path of the profile or of a package.
    assert str(tmp_path) not in "\n".join(lines)


def test_check_reports_a_plugin_entry_that_is_gone(tmp_path: Path) -> None:
    bridge, plugin = _installed(tmp_path)
    shutil.rmtree(plugin)
    lines = check(bridge, _available(set()))
    assert not loadable(lines)
    assert any("plugin entry" in line for line in lines)


def test_check_reports_a_plugin_that_is_not_enabled(tmp_path: Path) -> None:
    bridge, _ = _installed(tmp_path)
    bridge.config["plugins"] = {"enabled": []}
    lines = check(bridge, _available(set()))
    assert not loadable(lines)
    assert any("not enabled" in line for line in lines)


def test_check_without_an_install_needs_no_plugin_directory(tmp_path: Path) -> None:
    lines = check(MemoryBridge(tmp_path), _available(set()))
    assert "platform: not installed" in lines
    assert loadable(lines)


def test_package_source_is_a_value() -> None:
    source = PackageSource("attrs", Path("/x/attrs"), copy=True, first=False)
    assert source == PackageSource("attrs", Path("/x/attrs"), copy=True, first=False)
