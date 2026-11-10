# SPDX-License-Identifier: Apache-2.0
"""The runtime part of the heartbeat (09 4.8, design note 15)."""

from __future__ import annotations

import sys
from pathlib import Path
from uuid import UUID

import pytest
from bothy_sdk.generated.models import (
    ProjectReadiness,
    ProjectTrashResult,
    ProjectTrashResultResult,
)
from bothy_sdk.heartbeat import build_heartbeat
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.heartbeat import (
    CAPABILITIES,
    MIN_CLAUDE_CODE_VERSION,
    PINNED_CLAUDE_CODE_VERSION,
    WAIT_SLOTS_FULL,
    WAIT_THREAD_BUSY,
    Probe,
    billing_modes_supported,
    capabilities,
    parse_version,
    probe_version,
    raised_minimum,
    runtime_status,
)

CHANNEL = UUID("0192d3a0-0000-7000-8000-0000000000c1")


def test_parse_version() -> None:
    assert parse_version("2.1.259 (Claude Code)") == (2, 1, 259)
    assert parse_version("v2.1.300\n") == (2, 1, 300)
    for bad in ("", "Claude Code", "2.1", "x2.1.3"):
        assert parse_version(bad) is None


@pytest.mark.parametrize(
    ("key", "personal", "modes"),
    [
        (True, False, ("organization",)),
        (False, True, ("personal",)),
        (True, True, ("organization", "personal")),
        (False, False, None),
    ],
)
def test_body_per_configuration(key: bool, personal: bool, modes: tuple[str, ...] | None) -> None:
    assert billing_modes_supported(has_key_ref=key, allow_personal=personal) == modes
    status = runtime_status(
        Probe(PINNED_CLAUDE_CODE_VERSION, ()),
        [ProjectReadiness(channel_id=CHANNEL, ready=True)],
        modes,
    )
    beat = build_heartbeat(
        status.state,
        runtime_version=status.runtime_version,
        billing_modes_supported=status.billing_modes_supported,
        reported_tool_class="scoped",
        details={"capabilities": dict(CAPABILITIES)},
        project_readiness=status.project_readiness,
    ).to_dict()
    assert beat["state"] == "online"
    assert "reported_billing_mode" not in beat
    assert "billing_evidence" not in beat
    assert beat["project_readiness"] == [{"channel_id": str(CHANNEL), "ready": True}]
    assert beat.get("billing_modes_supported") == (list(modes) if modes else None)
    assert beat["details"]["capabilities"]["followups"] == "steer"
    text = repr(beat)
    assert "/" not in text.replace("://", "")


def script(tmp_path: Path, body: str) -> str:
    path = tmp_path / "claude"
    path.write_text(f"#!{sys.executable}\n{body}\n")
    path.chmod(0o755)
    return str(path)


@pytest.mark.anyio
@pytest.mark.parametrize(
    ("output", "reasons"),
    [
        (f"{PINNED_CLAUDE_CODE_VERSION} (Claude Code)", ()),
        ("9.9.9 (Claude Code)", ("runtime_version_untested",)),
        ("1.0.0 (Claude Code)", ("runtime_version_unsupported",)),
        ("garbage", ("runtime_version_unsupported",)),
    ],
)
async def test_probe(tmp_path: Path, output: str, reasons: tuple[str, ...]) -> None:
    prog = script(tmp_path, f"print({output!r})")
    probe = await probe_version(prog, {}, tmp_path, PATIENCE)
    assert probe.reasons == reasons
    assert probe.blocking is ("runtime_version_unsupported" in reasons)


@pytest.mark.anyio
async def test_probe_unreachable(tmp_path: Path) -> None:
    assert (await probe_version(None, {}, tmp_path, 1.0)).reasons == ("runtime_unreachable",)
    slow = script(tmp_path, "import time\ntime.sleep(30)")
    assert (await probe_version(slow, {}, tmp_path, 0.3)).blocking
    failing = script(tmp_path, "import sys\nsys.exit(1)")
    assert (await probe_version(failing, {}, tmp_path, PATIENCE)).reasons == (
        "runtime_unreachable",
    )
    assert MIN_CLAUDE_CODE_VERSION <= PINNED_CLAUDE_CODE_VERSION


def test_pin_is_a_version_that_serves_current_models() -> None:
    # 2.1.259 was refused by the API for current models ("version 2.1.280 or newer is
    # required", 2026-10-29); the pin moved to 2.1.288 (RUNNER-CLI-PIN) and then to 2.1.321
    # (FIX-RUNNER-CLI-PIN-321).
    assert PINNED_CLAUDE_CODE_VERSION == "2.1.321"
    assert parse_version(PINNED_CLAUDE_CODE_VERSION) >= (2, 1, 280)  # type: ignore[operator]


@pytest.mark.anyio
async def test_cli_2_1_321_is_the_tested_version(tmp_path: Path) -> None:
    """A host with Claude Code 2.1.321 is online; the former pin 2.1.288 only warns."""
    current = script(tmp_path, "print('2.1.321 (Claude Code)')")
    probe = await probe_version(current, {}, tmp_path, PATIENCE)
    assert (probe.version, probe.reasons, probe.blocking) == ("2.1.321", (), False)
    status = runtime_status(probe, [], None)
    assert (status.state, status.degraded_reasons) == ("online", ())
    assert status.runtime_version == "2.1.321"
    former = script(tmp_path, "print('2.1.288 (Claude Code)')")
    probe = await probe_version(former, {}, tmp_path, PATIENCE)
    assert (probe.version, probe.reasons) == ("2.1.288", ("runtime_version_untested",))
    assert not probe.blocking
    assert runtime_status(probe, [], None).state == "degraded"


@pytest.mark.anyio
async def test_probe_below_a_raised_minimum_is_unsupported(tmp_path: Path) -> None:
    """A minimum the API named for the model makes an older CLI unsupported (blocking)."""
    prog = script(tmp_path, f"print('{PINNED_CLAUDE_CODE_VERSION} (Claude Code)')")
    above = raised_minimum(
        MIN_CLAUDE_CODE_VERSION, current=PINNED_CLAUDE_CODE_VERSION, required=None
    )
    probe = await probe_version(prog, {}, tmp_path, PATIENCE, minimum=above)
    assert probe.version == PINNED_CLAUDE_CODE_VERSION
    assert probe.reasons == ("runtime_version_unsupported",)
    assert probe.blocking
    same = await probe_version(prog, {}, tmp_path, PATIENCE, minimum=PINNED_CLAUDE_CODE_VERSION)
    assert same.reasons == ()


@pytest.mark.parametrize(
    ("current", "minimum", "required", "expected"),
    [
        # The API named the version: it becomes the minimum.
        ("2.1.259", "2.1.259", "2.1.280", "2.1.280"),
        # Never lowered by a smaller or unreadable requirement.
        ("2.1.321", "2.1.330", "2.1.280", "2.1.330"),
        # No version named (an organization's policy): anything after the probed one.
        ("2.1.321", "2.1.259", None, "2.1.322"),
        # Nothing probed and nothing named: unchanged.
        (None, "2.1.259", None, "2.1.259"),
    ],
)
def test_raised_minimum(
    current: str | None, minimum: str, required: str | None, expected: str
) -> None:
    assert raised_minimum(minimum, current=current, required=required) == expected


def test_runtime_status_carries_the_project_folders() -> None:
    """The folder names of 09 18.13 enter the heartbeat; ``None`` leaves the member out."""
    status = runtime_status(Probe(PINNED_CLAUDE_CODE_VERSION, ()), [], None, ["api", "website"])
    assert status.project_folders == ("api", "website")
    assert runtime_status(Probe(PINNED_CLAUDE_CODE_VERSION, ()), [], None).project_folders is None


def test_runtime_status_carries_the_folder_move_results() -> None:
    """``project_trash_results`` of 09 18.15; ``None`` or empty leaves the member out."""
    done = ProjectTrashResult(channel_id=CHANNEL, result=ProjectTrashResultResult.TRASHED)
    status = runtime_status(Probe(PINNED_CLAUDE_CODE_VERSION, ()), [], None, None, [done])
    assert status.project_trash_results == (done,)
    assert runtime_status(Probe(), [], None, None, []).project_trash_results is None


def test_runtime_status_carries_slots_and_waiting_runs() -> None:
    """``run_slots`` and ``waiting_runs`` in run id order (09 4.8, 5.2.1)."""
    a, b = (
        UUID("0192d3a0-0000-7000-8000-00000000000b"),
        UUID("0192d3a0-0000-7000-8000-00000000000a"),
    )
    status = runtime_status(
        Probe(PINNED_CLAUDE_CODE_VERSION, ()),
        [],
        None,
        slots=(2, 2),
        waiting={a: WAIT_SLOTS_FULL, b: WAIT_THREAD_BUSY},
    )
    assert status.run_slots == {"used": 2, "total": 2}
    assert status.waiting_runs == ((b, "thread_busy"), (a, "slots_full"))
    bare = runtime_status(Probe(PINNED_CLAUDE_CODE_VERSION, ()), [], None)
    assert bare.run_slots is None
    assert bare.waiting_runs is None


def test_capabilities_without_hosting_are_the_ones_before_stage_a1() -> None:
    # 26 11 item 3: without runner.hosted_agents the capabilities are byte-equal to before.
    assert capabilities(voice_input=False) == {**CAPABILITIES, "voice_input": False}
    assert list(capabilities(voice_input=True)) == list({**CAPABILITIES, "voice_input": True})
    assert "hosts_agents" not in capabilities(voice_input=True)


def test_capability_turn_handover_only_while_the_key_is_on() -> None:
    # 09 18.19.1: turn_handover: true only while runner.turn_handover is on; off, the member
    # is missing and the capabilities equal those before stage AR (AR-T16).
    assert "turn_handover" not in capabilities(voice_input=False)
    assert "turn_handover" not in capabilities(voice_input=False, turn_handover=False)
    caps = capabilities(voice_input=False, turn_handover=True)
    assert caps["turn_handover"] is True
    assert {k: v for k, v in caps.items() if k != "turn_handover"} == capabilities(
        voice_input=False
    )


@pytest.mark.parametrize("ceiling", ["chat", "scoped", "host"])
def test_capabilities_with_hosting_report_the_ceiling(ceiling: str) -> None:
    # 26 3.1 item 3, 09 2.2: hosts_agents carries runner.hosted_max_tool_class.
    caps = capabilities(voice_input=False, hosts_agents=ceiling)
    assert caps["hosts_agents"] == ceiling
    assert {k: v for k, v in caps.items() if k != "hosts_agents"} == capabilities(voice_input=False)
