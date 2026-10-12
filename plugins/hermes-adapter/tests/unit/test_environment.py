# SPDX-License-Identifier: Apache-2.0
"""Throwaway homes and the allowlisted environment of the Hermes process tree."""

from __future__ import annotations

import os
import stat
import subprocess
from pathlib import Path

import pytest
from hermes_harness.environment import (
    ALLOWED_NAMES,
    NULL_KEYRING,
    STORE_SHIMS,
    HarnessEnvironmentError,
    ThrowawayHomes,
    assert_inside,
    build_environment,
    build_provisioning_environment,
)

CALLER = {
    "ANTHROPIC_API_KEY": "fake-caller-anthropic",
    "OPENAI_API_KEY": "fake-caller-openai",
    "GH_TOKEN": "fake-caller-gh",
    "SSH_AUTH_SOCK": "/tmp/fake-agent.sock",
    "DBUS_SESSION_BUS_ADDRESS": "unix:path=/tmp/fake-bus",
    "AWS_ACCESS_KEY_ID": "fake-caller-aws",
    "HERMES_HOME": "/tmp/fake-hermes-home",
}


@pytest.fixture
def homes(tmp_path: Path) -> ThrowawayHomes:
    return ThrowawayHomes.create(tmp_path / "root")


def test_environment_is_an_allowlist(
    homes: ThrowawayHomes, monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    for name, value in CALLER.items():
        monkeypatch.setenv(name, value)
    env = build_environment(
        homes, "probe", "http://127.0.0.1:9", {"FAKE_PROVIDER_KEY": "x"}, venv_bin=tmp_path / "bin"
    )
    assert set(env) == set(ALLOWED_NAMES) | {"FAKE_PROVIDER_KEY"}
    for name, value in CALLER.items():
        assert value not in env.values(), name
    assert env["HERMES_HOME"] == str(homes.hermes("probe"))
    assert env["HOME"] == str(homes.home)
    assert env["TZ"] == "UTC"
    assert env["TERM"] == "dumb"


def test_extra_names_cannot_override_or_smuggle(homes: ThrowawayHomes, tmp_path: Path) -> None:
    with pytest.raises(HarnessEnvironmentError, match="HOME"):
        build_environment(homes, "probe", "http://127.0.0.1:9", {"HOME": "/"}, venv_bin=tmp_path)
    with pytest.raises(HarnessEnvironmentError, match="name"):
        build_environment(
            homes, "probe", "http://127.0.0.1:9", {"bad name": "x"}, venv_bin=tmp_path
        )


def test_homes_live_inside_the_temp_root(homes: ThrowawayHomes) -> None:
    for path in (homes.home, homes.work, homes.tmp, homes.shims, homes.hermes("probe")):
        assert path.is_dir()
        assert path.resolve().is_relative_to(homes.root.resolve())
    assert homes.hermes("probe").parent.name == "hermes"
    with pytest.raises(HarnessEnvironmentError):
        homes.hermes("../escape")


def test_assert_inside_refuses_paths_outside_the_root(tmp_path: Path) -> None:
    root = tmp_path / "root"
    (root / "a").mkdir(parents=True)
    outside = tmp_path / "outside"
    outside.mkdir()
    assert assert_inside(root, root / "a") == (root / "a").resolve()
    with pytest.raises(HarnessEnvironmentError):
        assert_inside(root, outside)
    with pytest.raises(HarnessEnvironmentError):
        assert_inside(root, root / ".." / "outside")
    (root / "link").symlink_to(outside)
    with pytest.raises(HarnessEnvironmentError):
        assert_inside(root, root / "link")


def test_proxy_variables_point_to_the_egress_recorder(
    homes: ThrowawayHomes, tmp_path: Path
) -> None:
    env = build_environment(homes, "probe", "http://127.0.0.1:4242", {}, venv_bin=tmp_path)
    for name in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"):
        assert env[name] == "http://127.0.0.1:4242"
        assert env[name.lower()] == "http://127.0.0.1:4242"
    assert env["NO_PROXY"] == env["no_proxy"] == "127.0.0.1,localhost"


def test_keyring_is_null_and_store_shims_come_first(homes: ThrowawayHomes, tmp_path: Path) -> None:
    env = build_environment(homes, "probe", "http://127.0.0.1:9", {}, venv_bin=tmp_path / "bin")
    assert env["PYTHON_KEYRING_BACKEND"] == NULL_KEYRING
    parts = env["PATH"].split(os.pathsep)
    assert parts == [str(homes.shims), str(tmp_path / "bin"), "/usr/bin", "/bin"]
    for name in STORE_SHIMS:
        shim = homes.shims / name
        assert shim.stat().st_mode & stat.S_IXUSR
        result = subprocess.run([str(shim), "find-generic-password", "-w"], check=False, env=env)
        assert result.returncode != 0
    # Only the tool and its subcommand are kept, never an argument that could carry a value.
    assert homes.store_calls() == [f"{name} find-generic-password" for name in STORE_SHIMS]


def test_provisioning_environment_has_no_proxy_and_no_credentials(
    homes: ThrowawayHomes, monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    for name, value in CALLER.items():
        monkeypatch.setenv(name, value)
    monkeypatch.setenv("HTTPS_PROXY", "http://127.0.0.1:1")
    env = build_provisioning_environment(
        homes, uv_cache_dir=tmp_path / "cache", uv_python_dir=tmp_path / "py", tool_dirs=[tmp_path]
    )
    assert not {n for n in env if n.lower().endswith("_proxy")}
    for value in CALLER.values():
        assert value not in env.values()
    assert env["HOME"] == str(homes.home)
    assert env["UV_CACHE_DIR"] == str(tmp_path / "cache")
    assert env["UV_PYTHON_INSTALL_DIR"] == str(tmp_path / "py")
    assert env["GIT_TERMINAL_PROMPT"] == "0"
    assert env["PATH"].split(os.pathsep)[0] == str(tmp_path)
