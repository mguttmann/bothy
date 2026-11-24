# SPDX-License-Identifier: Apache-2.0
"""Memory limits of the Compose stack (AUD-PKG-P10-repo-ops-host): the collector, whose
memory_limiter counts in percent of the memory it sees, has a container limit, and every limit
is a named variable whose default equals .env.example and docs/operations/configuration.md.
No network."""

from __future__ import annotations

import re
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
COMPOSE = REPO / "infra" / "compose" / "compose.yaml"
ENV_EXAMPLE = REPO / "infra" / "compose" / ".env.example"
CONFIG = REPO / "docs" / "operations" / "configuration.md"
_LIMIT = re.compile(r"^\$\{(?P<var>BOTHY_[A-Z0-9_]+_MEMORY_LIMIT):-(?P<default>[^}]+)\}$")


def memory_limits() -> dict[str, str]:
    """Service name to its memory limit as written in compose.yaml."""
    services = yaml.safe_load(COMPOSE.read_text(encoding="utf-8"))["services"]
    out = {}
    for name, svc in services.items():
        limit = svc.get("deploy", {}).get("resources", {}).get("limits", {}).get("memory")
        if limit is not None:
            out[name] = str(limit)
    return out


def test_the_collector_has_a_memory_limit() -> None:
    assert "otel-collector" in memory_limits()


def test_every_memory_limit_is_a_documented_variable() -> None:
    env = ENV_EXAMPLE.read_text(encoding="utf-8")
    config = CONFIG.read_text(encoding="utf-8")
    for service, limit in memory_limits().items():
        m = _LIMIT.match(limit)
        assert m, f"{service}: memory limit {limit!r} is not ${{BOTHY_..._MEMORY_LIMIT:-default}}"
        var, default = m.group("var"), m.group("default")
        assert re.search(rf"^{var}={re.escape(default)}$", env, re.MULTILINE), (
            f"{var} in .env.example"
        )
        assert f"| `{var}` | `{default}` |" in config, f"{var} in configuration.md"
