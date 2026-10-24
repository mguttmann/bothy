# SPDX-License-Identifier: Apache-2.0
"""The admin key reaches the cost API and nothing else; the token reaches only the server."""

from __future__ import annotations

import io
import json

import pytest
from conftest import World
from fakes import Reply

from cost_reconciler.config import load_config
from cost_reconciler.logs import EventLog
from cost_reconciler.secretref import REDACTED


def assert_nowhere(world: World, value: str) -> None:
    for request in world.server.requests:
        assert value not in request.text()
    assert value not in world.stderr.getvalue()


def test_admin_key_goes_only_to_the_cost_api(world: World) -> None:
    assert world.run("--once") == 0
    # Positive control: the scan below cannot pass because the key was never sent.
    assert world.cost_api.requests
    for request in world.cost_api.requests:
        assert request.headers["x-api-key"] == world.admin_key
        assert world.token not in request.text()
    assert world.server.requests
    assert_nowhere(world, world.admin_key)
    for request in world.server.requests:
        assert request.headers["authorization"] == f"Bearer {world.token}"
    assert world.token not in world.stderr.getvalue()


@pytest.mark.parametrize("status", [401, 403, 500])
def test_cost_api_errors_leak_nothing(world: World, status: int) -> None:
    echo = json.dumps({"error": {"message": f"key {world.admin_key} is invalid"}}).encode()
    world.cost_api.override = Reply(status, echo, {"Content-Type": "application/json"})
    assert world.run("--once") == 1
    assert world.cost_api.requests[0].headers["x-api-key"] == world.admin_key
    assert_nowhere(world, world.admin_key)
    assert "is invalid" not in world.stderr.getvalue()


def test_server_errors_leak_nothing(world: World) -> None:
    world.server.submit_reply = Reply.of_json(
        400,
        {
            "code": "validation_failed",
            "detail": world.admin_key,
            "reason": "x" + world.token,
            "errors": [{"pointer": "/" + world.token, "code": world.admin_key}],
        },
        "application/problem+json",
    )
    assert world.run("--once") == 1
    stderr = world.stderr.getvalue()
    assert world.admin_key not in stderr
    assert world.token not in stderr
    assert REDACTED in stderr


def test_configuration_repr_holds_no_secret(world: World) -> None:
    config = load_config(world.write())
    assert world.admin_key not in repr(config)
    assert world.token not in repr(config)


def test_filter_redacts_current_secret_values() -> None:
    stream = io.StringIO()
    log = EventLog(stream)
    secret = "canarysecretvalue1234"
    log.set_secrets([secret, ""])
    log.emit(
        "pass_failed",
        level="error",
        reason=f"a{secret}b",
        run_ids=[secret],
        errors=[{"pointer": secret, "code": "x"}],
    )
    line = json.loads(stream.getvalue())
    assert secret not in stream.getvalue()
    assert line["reason"] == f"a{REDACTED}b"
    assert line["run_ids"] == [REDACTED]
    assert line["errors"] == [{"pointer": REDACTED, "code": "x"}]
