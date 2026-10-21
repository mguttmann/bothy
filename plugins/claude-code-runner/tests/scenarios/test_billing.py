# SPDX-License-Identifier: Apache-2.0
"""The billing mode is proven per run (BR-05, BR-12, AG-11; 09 18.3, 10 7)."""

from __future__ import annotations

import pytest

from .conftest import API_KEY, World

pytestmark = pytest.mark.anyio

FORBIDDEN = {
    "ANTHROPIC_AUTH_TOKEN": "x",
    "CLAUDE_CODE_OAUTH_TOKEN": "x",
    "ANTHROPIC_PROFILE": "x",
    "CLAUDE_CODE_USE_BEDROCK": "1",
    "CLAUDE_CODE_USE_VERTEX": "1",
    "CLAUDE_CODE_USE_FOUNDRY": "1",
}


async def _refused(world: World, reason: str) -> None:
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == reason
    assert [r for r in world.requests("startRun") if r.path.endswith(f"{run.run_id}/start")] == []


@pytest.mark.parametrize("variant", ["empty", "missing", "empty_file"])
async def test_BR_12_organization_without_key_no_fallback(world: World, variant: str) -> None:
    ref = {
        "empty": "",
        "missing": f"file:{world.root / 'no_key'}",
        "empty_file": f"file:{world.root / 'empty_key'}",
    }[variant]
    empty = world.root / "empty_key"
    empty.write_text("\n")
    empty.chmod(0o600)
    await world.start(world.config(anthropic_api_key_ref=ref))
    await _refused(world, "runtime_not_authenticated")
    assert world.calls("auth_status") == []
    assert world.turns() == []


async def test_BR_12_auth_status_none_and_valid_key(world: World) -> None:
    world.scenario(fixture="plain", auth_method="none")
    await world.start(**FORBIDDEN)
    await _refused(world, "runtime_not_authenticated")
    assert world.turns() == []
    world.scenario(fixture="plain")
    run = await world.fake.trigger("Again", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    (turn,) = world.turns()
    env = turn["env"]
    assert env["CLAUDE_CONFIG_DIR"] == str(world.state / "claude-config" / "organization")
    assert env["ANTHROPIC_API_KEY"] == API_KEY
    for name in FORBIDDEN:
        assert name not in env
    for call in world.calls("auth_status"):
        assert not set(FORBIDDEN) & set(call["env"])
    assert not any((world.state / "claude-config" / "organization").iterdir())


@pytest.mark.parametrize("variant", ["claude_ai", "non_json", "personal_not_allowed"])
async def test_AG_11_BR_05_billing_mode_mismatch(world: World, variant: str) -> None:
    if variant == "claude_ai":
        world.scenario(auth_method="claude.ai")
    elif variant == "non_json":
        world.scenario(auth_raw="Logged in as alex@example.com")
    else:
        world.fake.world.billing_mode = "personal"
    await world.start()
    await _refused(world, "billing_mode_mismatch")
    assert world.turns() == []
    if variant == "personal_not_allowed":
        assert world.calls("auth_status") == []


async def test_personal_allowed_with_subscription_login(world: World) -> None:
    world.fake.world.billing_mode = "personal"
    world.scenario(fixture="plain")
    await world.start(world.config(allow_personal=True))
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    start = world.requests("startRun")[0].json
    assert start["billing_mode_used"] == "personal"
    assert start["billing_evidence"]["key_type"] == "oauth"
    env = world.turns()[0]["env"]
    assert "ANTHROPIC_API_KEY" not in env
    assert "CLAUDE_CONFIG_DIR" not in env
    assert env["HOME"] == str(world.home)


@pytest.mark.parametrize("stubborn", [False, True])
async def test_BR_05_counter_check_aborts_turn(world: World, stubborn: bool) -> None:
    world.scenario(fixture="plain", api_key_source="none", hold=True, ignore_sigint=stubborn, ignore_sigterm=stubborn)
    await world.start()
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "billing_mode_mismatch"
    assert world.fake.messages == []
    (turn,) = world.turns()
    ends = [c for c in world.calls("turn_end") if c["pid"] == turn["pid"]]
    if stubborn:
        assert ends == []  # killed by the escalation, never a clean end
    else:
        assert ends[0]["signal"] == "SIGINT"
    _assert_gone(turn["pid"])


def _assert_gone(pid: int) -> None:
    import os

    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return
    raise AssertionError("the turn process still runs")
