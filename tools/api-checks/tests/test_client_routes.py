# SPDX-License-Identifier: Apache-2.0
"""Tests of the check `client-routes` (HERMES-E2E-REAL-SERVER).

Every negative case builds a tree with copies of the real contract, enable list, route table and
generated SDK modules plus one client source under `plugins/`, changes one thing and expects the
named finding; the repository itself must stay clean.
"""

from __future__ import annotations

import shutil
from pathlib import Path

import client_routes
import pytest

REPO = Path(__file__).resolve().parents[3]
CLIENT = "plugins/demo/src/demo/client.py"


@pytest.fixture
def tree(tmp_path: Path) -> Path:
    """A repository root with the contract files and a client that calls getAgentTriggerers."""
    root = tmp_path / "repo"
    for rel in (client_routes.OPENAPI_PATH, client_routes.ENABLE_LIST, client_routes.ROUTE_TABLE):
        (root / rel).parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(REPO / rel, root / rel)
    shutil.copytree(REPO / client_routes.GENERATED, root / client_routes.GENERATED)
    (root / "plugins/demo/pyproject.toml").parent.mkdir(parents=True)
    (root / "plugins/demo/pyproject.toml").write_text("[project]\nname = 'demo'\n")
    client = root / CLIENT
    client.parent.mkdir(parents=True)
    client.write_text(
        "from bothy_sdk.generated.api.agent_runtime import get_agent_triggerers, start_run\n"
    )
    # A test of the plugin may import anything; tests are no client code.
    tests = root / "plugins/demo/tests/test_x.py"
    tests.parent.mkdir(parents=True)
    tests.write_text("from bothy_sdk.generated.api.agent_runtime import no_such_operation\n")
    return root


def _edit(path: Path, old: str, new: str) -> None:
    text = path.read_text(encoding="utf-8")
    assert old in text
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


def test_repository_clean() -> None:
    """The checkout itself has no finding."""
    assert client_routes.check(REPO) == []


def test_the_sources_include_sdk_and_plugins_but_no_tests() -> None:
    """The SDK's hand-written code and the plugins' code are scanned, never tests."""
    rels = {p.relative_to(REPO).as_posix() for p in client_routes.client_sources(REPO)}
    assert "sdk/python/src/bothy_sdk/adapter.py" in rels
    assert "plugins/hermes-adapter/src/bothy_hermes_adapter/handler.py" in rels
    assert "plugins/claude-code-runner/src/claude_code_runner/runturn.py" in rels
    assert not any("/generated/" in r or "/tests/" in r for r in rels)
    calls = {(c.tag, c.name) for c in client_routes.client_calls(REPO)}
    assert ("agent_runtime", "get_agent_triggerers") in calls


def test_tree_clean(tree: Path) -> None:
    """The copy with the demo client has no finding."""
    assert client_routes.check(tree) == []


def test_an_operation_under_later_is_found(tree: Path) -> None:
    """SERVER-AGENT-TRIGGERERS: documented, but not enabled, so the server answered 404."""
    _edit(tree / client_routes.ENABLE_LIST, "  - getAgentTriggerers\n", "")
    found = client_routes.check(tree)
    assert found == [
        f"client-routes: enabled: {CLIENT}:1: getAgentTriggerers is not enabled in "
        f"{client_routes.ENABLE_LIST}"
    ]


def test_an_unregistered_route_is_found(tree: Path) -> None:
    """Enabled, but missing from the generated route table (not regenerated)."""
    _edit(tree / client_routes.ROUTE_TABLE, 'ID:             "startRun"', 'ID: "startRunX"')
    found = client_routes.check(tree)
    assert len(found) == 1
    assert found[0].startswith(f"client-routes: router: {CLIENT}:1: startRun POST")


def test_an_operation_missing_from_openapi_is_found(tree: Path) -> None:
    """The generated module names a path the contract no longer has."""
    _edit(tree / client_routes.OPENAPI_PATH, "  /agents/me/triggerers:", "  /agents/me/gone:")
    found = client_routes.check(tree)
    assert len(found) == 1
    assert found[0].startswith(f"client-routes: openapi: {CLIENT}:1: GET /agents/me/triggerers")


def test_an_import_without_a_generated_module_is_found(tree: Path) -> None:
    """A client imports an operation the generated core does not have."""
    (tree / CLIENT).write_text(
        "def f():\n    from bothy_sdk.generated.api.runs.no_such_op import sync\n"
    )
    found = client_routes.check(tree)
    assert found == [
        f"client-routes: module: {CLIENT}:2: runs.no_such_op is no generated operation"
    ]


def test_a_plain_import_of_an_operation_is_found(tree: Path) -> None:
    """`import bothy_sdk.generated.api.<tag>.<op>` is checked like a from-import."""
    (tree / CLIENT).write_text(
        "import bothy_sdk.generated.api.agent_runtime.get_agent_triggerers\n"
    )
    assert client_routes.check(tree) == []
    _edit(tree / client_routes.ENABLE_LIST, "  - getAgentTriggerers\n", "")
    assert client_routes.check(tree) == [
        f"client-routes: enabled: {CLIENT}:1: getAgentTriggerers is not enabled in "
        f"{client_routes.ENABLE_LIST}"
    ]
