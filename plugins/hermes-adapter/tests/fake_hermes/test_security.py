# SPDX-License-Identifier: Apache-2.0
"""Tokens and the actual mode against the pinned Hermes (AG-08, AZ-P6, AG-11, BR-05, BR-13).

One gateway serves the token and mode scenarios of this module: the profile's ``.env`` changes
between runs, and Hermes resolves the credential again for every turn (09 17.10
``credential_type``). Every scenario that must not start a turn asserts that the fake model
received no request for it.
"""

from __future__ import annotations

import json
from typing import Any
from uuid import UUID

import pytest
import yaml
from bothy_sdk.auth import Secret
from bothy_sdk.testing._fake_tokens import TYP
from bothy_sdk.testing.faults import Fault
from fake_hermes_kit import HermesRig, HermesWorld, Profile, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

pytestmark = pytest.mark.anyio

ANSWER = ("Noted ", "for the ", "lake trip.")
FORGED = ("missing", "bad_signature", "alg_hs256", "typ_access", "aud_other_agent")


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _answers(n: int) -> list[ScriptTurn]:
    return [ScriptTurn(text=ANSWER, paced=False) for _ in range(n)]


def _scripted(rig: HermesRig) -> int:
    return len(rig.model_requests(scripted=True))


def _invocation_seq(rig: HermesRig, run_id: Any) -> int:
    entry = next(e for e in rig.fake.hub.log if e.envelope["payload"].get("run_id") == str(run_id))
    return int(entry.seq)


def _agent_beats(rig: HermesRig) -> list[dict[str, Any]]:
    beats = [
        f["d"]["agent"]
        for f in rig.fake.frames
        if f.get("op") == "heartbeat" and "agent" in f.get("d", {})
    ]
    return [*rig.fake.heartbeats, *beats]


async def _fails(rig: HermesRig, run_id: Any, reason: str) -> None:
    try:
        await rig.until(lambda: rig.fake.run_state(run_id).state == "failed", "the refusal")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None
    assert rig.fake.run_state(run_id).state_reason == reason
    assert rig.fake.run_state(run_id).starts == 0


async def _completes(rig: HermesRig, run_id: Any) -> None:
    try:
        await rig.until(lambda: rig.finished(run_id), "the end of the run")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None
    assert rig.fake.run_state(run_id).state == "completed", gateway_log(rig)


async def test_ag08_invalid_tokens_start_nothing(world: HermesWorld) -> None:
    async with hermes_rig(world, _answers(1)) as rig:
        for variant in FORGED:
            before = _scripted(rig)
            triggered = await rig.fake.trigger(f"AG-08 {variant}", forged=variant)
            await _fails(rig, triggered.run_id, "invalid_invocation_token")
            seq = _invocation_seq(rig, triggered.run_id)
            await rig.until(lambda s=seq: rig.fake.consumer_state("default")[0] >= s, "the ack")
            runs = [q for q in rig.fake.requests if str(triggered.run_id) in q.path]
            ops = {q.operation_id for q in runs}
            assert "startRun" not in ops
            assert "renewInvocationToken" not in ops
            assert _scripted(rig) == before
        assert _scripted(rig) == 0


async def test_azp6_reference_adapter_never_starts_runtime(world: HermesWorld) -> None:
    profile = Profile(toolsets=("todo", "clarify", "web"))
    async with hermes_rig(world, _answers(2), profile=profile, tool_class="host") as rig:
        fake = rig.fake
        # An expired token whose renewal the server refuses.
        fake.inject("renewInvocationToken", Fault.problem("agent_invocation_denied"))
        expired = await fake.trigger("expired", forged="expired")
        await rig.until(
            lambda: any(
                q.operation_id == "renewInvocationToken" and str(expired.run_id) in q.path
                for q in fake.requests
            ),
            "the renewal",
        )
        assert fake.run_state(expired.run_id).starts == 0
        # A billing mode unequal to the actual mode.
        fake.world.billing_mode = "personal"
        mode = await fake.trigger("mode")
        await _fails(rig, mode.run_id, "billing_mode_mismatch")
        fake.world.billing_mode = "organization"
        # A tool class below the loaded class (web is host).
        fake.world.tool_class = "scoped"
        tool = await fake.trigger("tool")
        await _fails(rig, tool.run_id, "tool_class_mismatch")
        fake.world.tool_class = "host"
        assert _scripted(rig) == 0
        # A repeated jti: the token of a started run under a new event ID.
        good = await fake.trigger("good")
        await _completes(rig, good.run_id)
        after_good = _scripted(rig)
        run = fake.world.runs[good.run_id]
        claims = run.issued[str(run.started_jti)]
        issuer = fake.world.issuer
        replay = issuer._sign({"alg": "EdDSA", "typ": TYP, "kid": issuer.kid}, claims)
        event = next(
            e for e in fake.hub.log if e.envelope["payload"].get("run_id") == str(good.run_id)
        )
        replayed = await fake.deliver_with_token(UUID(event.envelope["id"]), Secret(replay))
        await rig.until(
            lambda: any(f.get("op") == "heartbeat" for f in fake.frames[-5:]), "a later frame"
        )
        assert fake.run_state(good.run_id).starts == 1
        assert _scripted(rig) == after_good
        assert replayed is not None


async def test_ag11_actual_mode_unequal_target(world: HermesWorld) -> None:
    async with hermes_rig(world, _answers(1)) as rig:
        # Token personal while the profile resolves an Anthropic API key.
        rig.fake.world.billing_mode = "personal"
        first = await rig.fake.trigger("personal token")
        await _fails(rig, first.run_id, "billing_mode_mismatch")
        # Token organization while an OAuth canary is set next to the API key (OAuth wins).
        rig.write_dotenv({"anthropic_api_key": "api_key", "anthropic_oauth": "oauth"})
        rig.fake.world.billing_mode = "organization"
        second = await rig.fake.trigger("organization token")
        await _fails(rig, second.run_id, "billing_mode_mismatch")
        assert rig.model_requests(scripted=True) == []


async def test_actual_mode_matches_used_credential(world: HermesWorld) -> None:
    cases = [
        ({"anthropic_api_key": "api_key"}, "organization", "api_key"),
        ({"anthropic_oauth": "oauth"}, "personal", "oauth"),
        ({"anthropic_api_key": "api_key", "anthropic_oauth": "oauth"}, "personal", "oauth"),
    ]
    async with hermes_rig(world, _answers(len(cases))) as rig:
        for dotenv, mode, kind in cases:
            rig.write_dotenv(dotenv)
            rig.fake.world.billing_mode = mode
            before = len(rig.model_requests())
            triggered = await rig.fake.trigger(f"mode {mode}")
            await _completes(rig, triggered.run_id)
            start = next(q for q in rig.requests("startRun") if str(triggered.run_id) in q.path)
            assert start.json["billing_mode_used"] == mode
            assert start.json["billing_evidence"]["key_type"] == kind
            # Only the turn's own requests (they offer tools): a title or memory request of the
            # previous run may still arrive with that run's credential.
            used = [
                r
                for r in rig.model_requests()[before:]
                if r["route"] == "anthropic.messages" and r.get("tools")
            ]
            assert used
            assert {str(u["credential"]) for u in used} == {kind}


async def test_unknown_actual_mode_gives_mismatch(world: HermesWorld) -> None:
    async with hermes_rig(world, _answers(1), profile=Profile(dotenv={})) as rig:
        triggered = await rig.fake.trigger("no credential")
        await _fails(rig, triggered.run_id, "billing_mode_mismatch")
        await rig.until(lambda: _agent_beats(rig), "a heartbeat")
        beat = _agent_beats(rig)[-1]
        assert beat["state"] == "degraded"
        assert "billing_evidence_unknown" in beat["details"]["degraded_reasons"]
        assert beat["billing_evidence"]["key_type"] == "unknown"
        assert "reported_billing_mode" not in beat
        # Cloud credentials: a provider that authenticates through a cloud SDK.
        config = yaml.safe_load((rig.home / "config.yaml").read_text(encoding="utf-8"))
        config["model"]["provider"] = "bedrock"
        (rig.home / "config.yaml").write_text(yaml.safe_dump(config), encoding="utf-8")
        rig.write_dotenv({"anthropic_api_key": "api_key"})
        cloud = await rig.fake.trigger("cloud")
        await _fails(rig, cloud.run_id, "billing_mode_mismatch")
        assert rig.model_requests(scripted=True) == []


async def test_br13_no_key_value_anywhere(world: HermesWorld) -> None:
    async with hermes_rig(world, _answers(1)) as rig:
        api_key, oauth = rig.creds.api_key, rig.creds.oauth_token
        done = await rig.fake.trigger("one run")
        await _completes(rig, done.run_id)
        rig.fake.world.billing_mode = "personal"
        refused = await rig.fake.trigger("mismatch")
        await _fails(rig, refused.run_id, "billing_mode_mismatch")
        rig.write_dotenv({"anthropic_api_key": "api_key", "anthropic_oauth": "oauth"})
        outputs = [rig.installed_output]
        for args in (
            ("check", "--profile-home", str(rig.home)),
            (
                "switch-mode",
                "personal",
                "--profile-home",
                str(rig.home),
                "--credential-holder",
                str(rig.fake.robin_id),
            ),
            ("check", "--profile-home", str(rig.home)),
        ):
            result = rig.installer(*args)
            outputs += [result.stdout, result.stderr]
        await rig.until(lambda: _agent_beats(rig), "a heartbeat")
        recorded = json.dumps(
            [[q.path, dict(q.headers), q.json] for q in rig.fake.requests]
            + rig.fake.frames
            + _agent_beats(rig),
            default=str,
        )
        state = b"".join(p.read_bytes() for p in rig.state_files())
        log = gateway_log(rig) if rig.gateway is not None else ""
        full_log = rig.gateway.log.read_text(errors="replace") if rig.gateway else ""
        for canary in (api_key, oauth):
            assert canary not in recorded
            assert canary not in "\n".join(outputs)
            assert canary.encode() not in state
            assert canary not in full_log
            assert canary not in log
        assert rig.state_files()
