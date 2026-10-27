# SPDX-License-Identifier: Apache-2.0
"""The demo overlay infra/compose/demo.yaml and the inputs of infra/compose/demo.sh (brief M7-27).

The overlay tests read the YAML with a loader that keeps the Compose tags; the input tests run
demo.sh, which checks its inputs before it looks for Docker, so they need no Docker. The live
check of a restart of `api` is `make demo-restart-check` (docs/operations/demo.md).
"""

from __future__ import annotations

import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
COMPOSE_DIR = REPO / "infra" / "compose"
DEMO_YAML = COMPOSE_DIR / "demo.yaml"
DEMO_SH = COMPOSE_DIR / "demo.sh"
HOLDER = "demo-net"
# The services of the overlay that run for the whole demo (one-off services excluded).
LONG_RUNNING = ("demo-tls", "demo-agent", "demo-host")


@dataclass(frozen=True)
class Tagged:
    """A value with a Compose merge tag (`!reset` or `!override`)."""

    tag: str
    value: Any


class ComposeLoader(yaml.SafeLoader):
    """SafeLoader that keeps `!reset` and `!override` as `Tagged` values."""


def _tagged(loader: yaml.SafeLoader, node: yaml.Node) -> Tagged:
    if isinstance(node, yaml.MappingNode):
        value: Any = loader.construct_mapping(node, deep=True)
    elif isinstance(node, yaml.SequenceNode):
        value = loader.construct_sequence(node, deep=True)
    else:
        value = loader.construct_scalar(node)  # type: ignore[arg-type]
    return Tagged(node.tag, value)


ComposeLoader.add_constructor("!reset", _tagged)
ComposeLoader.add_constructor("!override", _tagged)


def untag(value: Any) -> Any:
    return value.value if isinstance(value, Tagged) else value


def services() -> dict[str, Any]:
    text = DEMO_YAML.read_text(encoding="utf-8")
    data = yaml.load(text, Loader=ComposeLoader)["services"]  # noqa: S506
    assert isinstance(data, dict)
    return data


def test_no_service_joins_the_namespace_of_api() -> None:
    # A restart of api gives it a new network namespace; a container that joined the old one
    # keeps it and the demo answers nothing on its port (demo test R2).
    joined = [n for n, s in services().items() if s.get("network_mode") == "service:api"]
    assert joined == []


def test_api_and_the_long_running_services_share_the_holder() -> None:
    svc = services()
    for name in ("api", *LONG_RUNNING):
        assert svc[name]["network_mode"] == f"service:{HOLDER}", name
    # The namespace of api moves to the holder: api keeps no port and no network of its own.
    assert svc["api"]["ports"] == Tagged("!reset", [])
    assert svc["api"]["networks"] == Tagged("!reset", [])


def test_the_holder_owns_the_port_and_the_networks_of_api() -> None:
    holder = services()[HOLDER]
    port = "${DEMO_PORT:?set by demo.sh}"
    assert holder["ports"] == [f"127.0.0.1:{port}:{port}"]
    networks = holder["networks"]
    assert set(networks) == {"internal", "default"}
    for name, value in networks.items():
        assert value["aliases"] == ["api"], name
    # Only a process that does nothing holds the namespace, so nothing makes it restart.
    assert holder["entrypoint"] == ["sleep", "infinity"]
    assert holder["init"] is True
    assert holder["read_only"] is True
    assert holder["cap_drop"] == ["ALL"]


@pytest.mark.parametrize("name", [HOLDER, *LONG_RUNNING])
def test_long_running_services_come_back_after_a_restart_of_docker(name: str) -> None:
    # A restart of the Docker VM starts only containers with a restart policy; the engine starts
    # the holder before the containers in its namespace.
    assert untag(services()[name].get("restart")) == "unless-stopped"


# ---- inputs of demo.sh --------------------------------------------------------------------


def run_demo(tmp_path: Path, **env: str) -> subprocess.CompletedProcess[str]:
    """Run demo.sh with only the tools it needs before its Docker check."""
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    for tool in ("bash", "dirname", "sed", "tr", "cat"):
        found = shutil.which(tool)
        assert found is not None, tool
        (bin_dir / tool).symlink_to(found)
    return subprocess.run(
        [str(DEMO_SH)],
        env={"PATH": str(bin_dir), "HOME": str(tmp_path), "TMPDIR": str(tmp_path), **env},
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )


@pytest.mark.parametrize("port", ["abc", "0", "65536", "-1", "80x"])
def test_a_bad_fixed_port_is_a_precondition(tmp_path: Path, port: str) -> None:
    result = run_demo(tmp_path, DEMO_PORT=port)
    assert result.returncode == 2, result.stderr
    assert f"DEMO_PORT must be a port number from 1 to 65535, got '{port}'" in result.stderr


@pytest.mark.parametrize("value", ["yes", "2"])
def test_a_bad_restart_check_input_is_a_precondition(tmp_path: Path, value: str) -> None:
    result = run_demo(tmp_path, DEMO_CHECK_RESTART=value)
    assert result.returncode == 2, result.stderr
    assert f"DEMO_CHECK_RESTART must be 0 or 1, got '{value}'" in result.stderr


@pytest.mark.parametrize("port", ["1", "61851", "65535"])
def test_a_valid_fixed_port_passes_the_inputs(tmp_path: Path, port: str) -> None:
    # The run then stops at the Docker check: there is no docker on PATH.
    result = run_demo(tmp_path, DEMO_PORT=port, DEMO_CHECK_RESTART="1")
    assert result.returncode == 2, result.stderr
    assert "DEMO_PORT" not in result.stderr
    assert "docker is not on PATH" in result.stderr
