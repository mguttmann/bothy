# SPDX-License-Identifier: Apache-2.0
"""The runtime part of the heartbeat (09 4.8, design note 15)."""

from __future__ import annotations

import sys
from pathlib import Path
from uuid import UUID

import pytest
from bothy_sdk.generated.models import ProjectReadiness
from bothy_sdk.heartbeat import build_heartbeat

from claude_code_runner.heartbeat import (
    CAPABILITIES,
    MIN_CLAUDE_CODE_VERSION,
    PINNED_CLAUDE_CODE_VERSION,
    Probe,
    billing_modes_supported,
    parse_version,
    probe_version,
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
        Probe("2.1.259", ()), [ProjectReadiness(channel_id=CHANNEL, ready=True)], modes
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
    assert "followups" not in beat["details"]["capabilities"]
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
    probe = await probe_version(prog, {}, tmp_path, 5.0)
    assert probe.reasons == reasons
    assert probe.blocking is ("runtime_version_unsupported" in reasons)


@pytest.mark.anyio
async def test_probe_unreachable(tmp_path: Path) -> None:
    assert (await probe_version(None, {}, tmp_path, 1.0)).reasons == ("runtime_unreachable",)
    slow = script(tmp_path, "import time\ntime.sleep(30)")
    assert (await probe_version(slow, {}, tmp_path, 0.3)).blocking
    failing = script(tmp_path, "import sys\nsys.exit(1)")
    assert (await probe_version(failing, {}, tmp_path, 5.0)).reasons == ("runtime_unreachable",)
    assert MIN_CLAUDE_CODE_VERSION <= PINNED_CLAUDE_CODE_VERSION
