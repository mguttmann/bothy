# SPDX-License-Identifier: Apache-2.0
"""The runtime part of the runner's heartbeat (docs/spec/09-agents.md 4.8, note 15).

`probe_version` runs ``claude --version`` within ``runner.cli_probe_timeout``; its outcome gives
the degraded reasons ``runtime_unreachable`` (no answer), ``runtime_version_unsupported``
(below `MIN_CLAUDE_CODE_VERSION`, below a minimum the API named for the model, or unreadable)
and ``runtime_version_untested`` (another version than `PINNED_CLAUDE_CODE_VERSION`).
`runtime_status` turns the last probe, the project readiness and the provable billing modes
into the SDK's `RuntimeStatus`; the SDK adds
``details`` (capabilities, active runs, adapter and runtime names, protocol version).

Examples:
    >>> parse_version("2.1.259 (Claude Code)")
    (2, 1, 259)
"""

from __future__ import annotations

import asyncio
import contextlib
import re
import subprocess  # nosec B404 - argument list, never a shell, program from the cleaned PATH
from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from pathlib import Path

from bothy_sdk.adapter import RuntimeStatus
from bothy_sdk.generated.models import ProjectReadiness

__all__ = [
    "CAPABILITIES",
    "MIN_CLAUDE_CODE_VERSION",
    "PINNED_CLAUDE_CODE_VERSION",
    "Probe",
    "billing_modes_supported",
    "parse_version",
    "probe_version",
    "raised_minimum",
    "runtime_status",
]

MIN_CLAUDE_CODE_VERSION = "2.1.259"
"""The version 09 18.2 names for ``--permission-prompts none``."""

PINNED_CLAUDE_CODE_VERSION = "2.1.288"
"""The version the fixtures and the stream field table were written for (RK-14).

2.1.259 until 2026-10-29, when the API refused current models to it ("version 2.1.280 or newer
is required"); docs/DECISIONS.md D-RUNNER-CLI-PIN-1."""

CAPABILITIES: Mapping[str, bool | str] = {
    "followups": "steer",
    "title": True,
    "components": False,
    "voice_input": False,
    "files_out": True,
}
"""``details.capabilities`` (09 2.2, 4.8): follow-ups reach the running turn on stdin (18.5);
answers carry files (18.10, M3-12). The owner's switch ``supports_files_out`` is enforced by the
server at the send (``403 feature_disabled``), so the runner never reads ``operator_config``.
``voice_input`` is the default; the runner reports true when ``runner.transcribe_cmd`` is set
(18.14)."""

_VERSION = re.compile(r"^\s*v?(\d+)\.(\d+)\.(\d+)(?:\s|$)")
BLOCKING = frozenset({"runtime_unreachable", "runtime_version_unsupported"})
"""Degraded reasons under which no run starts (prepare ends it with ``runtime_error``)."""


def parse_version(text: str) -> tuple[int, int, int] | None:
    """Read ``major.minor.patch`` at the start of the ``--version`` output.

    Args:
        text: The output.

    Returns:
        The version, or ``None`` when it is unreadable.
    """
    match = _VERSION.match(text)
    if match is None:
        return None
    return int(match.group(1)), int(match.group(2)), int(match.group(3))


def _as_tuple(version: str) -> tuple[int, int, int]:
    parsed = parse_version(version)
    if parsed is None:  # pragma: no cover - the constants are well formed
        raise ValueError("bad version constant")
    return parsed


@dataclass(frozen=True)
class Probe:
    """The outcome of one ``claude --version``.

    Attributes:
        version: ``major.minor.patch``, or ``None``.
        reasons: The degraded reasons it gives.
    """

    version: str | None = None
    reasons: tuple[str, ...] = field(default=("runtime_unreachable",))

    @property
    def blocking(self) -> bool:
        """True when no run may start."""
        return bool(BLOCKING & set(self.reasons))


def raised_minimum(minimum: str, *, current: str | None, required: str | None) -> str:
    """Return the minimum after the API refused the model to the CLI as too old (18.9).

    Args:
        minimum: The minimum so far.
        current: The probed version when the refusal came, if known.
        required: The version the API named (``version <v> or newer is required``); ``None``
            when it named none (an organization's policy floor).

    Returns:
        The higher of ``minimum`` and ``required``; without ``required``, the version after
        ``current`` (patch plus one), so the probed version counts as too old; without both,
        ``minimum`` unchanged.

    Examples:
        >>> raised_minimum("2.1.259", current="2.1.259", required="2.1.280")
        '2.1.280'
    """
    if required is not None and parse_version(required) is not None:
        wanted = _as_tuple(required)
    elif current is not None and (parsed := parse_version(current)) is not None:
        wanted = (parsed[0], parsed[1], parsed[2] + 1)
    else:
        return minimum
    if wanted <= _as_tuple(minimum):
        return minimum
    return ".".join(str(p) for p in wanted)


async def probe_version(  # noqa: PLR0911 - one return per outcome
    claude_bin: str | None,
    env: Mapping[str, str],
    cwd: Path,
    timeout: float,  # NOSONAR - a timeout is a classified outcome, D-RUNNER-CLI-PIN-4
    *,
    minimum: str = MIN_CLAUDE_CODE_VERSION,
) -> Probe:
    """Run ``<claude_bin> --version`` and classify the answer.

    Args:
        claude_bin: The CLI in the cleaned ``PATH``; ``None`` when it was not found.
        env: An environment without credentials.
        cwd: The working directory (``runner.state_dir``).
        timeout: ``runner.cli_probe_timeout``.
        minimum: The lowest supported version: `MIN_CLAUDE_CODE_VERSION`, or the higher one
            the API named for the model (`raised_minimum`).

    Returns:
        The probe.
    """
    if claude_bin is None:
        return Probe()
    try:
        proc = await asyncio.create_subprocess_exec(  # nosec B603 - argument list, no shell
            claude_bin,
            "--version",
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            cwd=cwd,
            env=dict(env),
            start_new_session=True,
        )
    except OSError:
        return Probe()
    try:
        out, _ = await asyncio.wait_for(proc.communicate(), timeout)
    except TimeoutError:
        await _reap(proc)
        return Probe()
    except asyncio.CancelledError:
        await _reap(proc)  # a stopping runner leaves no probe process behind
        raise
    if proc.returncode != 0:
        return Probe()
    parsed = parse_version(out.decode("utf-8", errors="replace"))
    if parsed is None:
        return Probe(None, ("runtime_version_unsupported",))
    version = ".".join(str(p) for p in parsed)
    if parsed < max(_as_tuple(MIN_CLAUDE_CODE_VERSION), _as_tuple(minimum)):
        return Probe(version, ("runtime_version_unsupported",))
    if parsed != _as_tuple(PINNED_CLAUDE_CODE_VERSION):
        return Probe(version, ("runtime_version_untested",))
    return Probe(version, ())


async def _reap(proc: asyncio.subprocess.Process) -> None:
    """Kill a probe process and wait for it (also when the waiting task is cancelled)."""
    with contextlib.suppress(ProcessLookupError):
        proc.kill()
    with contextlib.suppress(asyncio.CancelledError):
        await asyncio.shield(proc.wait())


def billing_modes_supported(*, has_key_ref: bool, allow_personal: bool) -> tuple[str, ...] | None:
    """Return ``billing_modes_supported``: the modes the runner can prove.

    Args:
        has_key_ref: ``runner.anthropic_api_key_ref`` is set.
        allow_personal: ``runner.allow_personal``.

    Returns:
        The modes, or ``None`` (member left out) when both are off.
    """
    modes = (("organization",) if has_key_ref else ()) + (("personal",) if allow_personal else ())
    return modes or None


def runtime_status(
    probe: Probe,
    readiness: Sequence[ProjectReadiness],
    modes: tuple[str, ...] | None,
    folders: Sequence[str] | None = None,
) -> RuntimeStatus:
    """Build the heartbeat members that change at run time.

    No ``reported_billing_mode`` and no ``billing_evidence``: the runner follows the desired
    mode per run (D-L8) and proves it per run.

    Args:
        probe: The last version probe.
        readiness: One entry per bound forum in the channel cache.
        modes: `billing_modes_supported`.
        folders: The project folder names of 09 18.13; ``None`` leaves the member out.

    Returns:
        The status.
    """
    return RuntimeStatus(
        state="degraded" if probe.reasons else "online",
        degraded_reasons=probe.reasons,
        runtime_version=probe.version,
        billing_modes_supported=modes,
        project_readiness=tuple(readiness),
        project_folders=None if folders is None else tuple(folders),
    )
