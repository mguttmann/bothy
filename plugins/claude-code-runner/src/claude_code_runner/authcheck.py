# SPDX-License-Identifier: Apache-2.0
"""The pre-check of the billing mode with ``claude auth status`` (09 18.3, 10 7, note 8).

The check runs in the very environment the turn will get, right before ``startRun``, and its
outcome binds the start: ``organization`` needs ``api_key``, ``personal`` needs ``claude.ai`` or
``oauth_token``; ``none`` is ``runtime_not_authenticated``; anything else, including an output
that cannot be read, is ``billing_mode_mismatch`` (an unknown mode differs from every desired
one). The exit code is ignored, because ``none`` may come with a non-zero exit.

Examples:
    >>> decide("organization", "api_key"), decide("personal", "none")
    ('start', 'runtime_not_authenticated')
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import subprocess  # nosec B404 - argument list, never a shell, program from the cleaned PATH
from collections.abc import Mapping
from pathlib import Path

from bothy_sdk.generated.models import BillingEvidence, BillingEvidenceKeyType

__all__ = [
    "KNOWN_METHODS",
    "auth_status",
    "decide",
    "evidence",
    "key_type_for",
    "parse_auth_method",
]

_LOG = logging.getLogger("claude_code_runner")

KNOWN_METHODS = frozenset(
    {"none", "claude.ai", "oauth_token", "api_key", "api_key_helper", "third_party"}
)
"""The ``authMethod`` values of the CLI reference (18.3 [belegt])."""

_START = {
    "organization": frozenset({"api_key"}),
    "personal": frozenset({"claude.ai", "oauth_token"}),
}
_KEY_TYPE = {
    "api_key": "api_key",
    "api_key_helper": "api_key",
    "claude.ai": "oauth",
    "oauth_token": "oauth",
    "third_party": "cloud",
    "none": "none",
}


def parse_auth_method(stdout: bytes | str) -> str:
    """Read ``authMethod`` from the output of ``auth status``.

    Args:
        stdout: The standard output.

    Returns:
        A value of `KNOWN_METHODS`, or ``unknown`` for anything else.
    """
    try:
        data = json.loads(stdout)
    except (ValueError, UnicodeDecodeError):
        return "unknown"
    method = data.get("authMethod") if isinstance(data, dict) else None
    return method if isinstance(method, str) and method in KNOWN_METHODS else "unknown"


def key_type_for(method: str) -> str:
    """Return ``billing_evidence.key_type`` for an ``authMethod``.

    Args:
        method: The method.

    Returns:
        ``api_key``, ``oauth``, ``cloud``, ``none`` or ``unknown``.
    """
    return _KEY_TYPE.get(method, "unknown")


def decide(desired: str, method: str) -> str:
    """Apply the table of design note 8 (first match wins).

    Args:
        desired: The claim ``billing_mode``.
        method: The ``authMethod`` (or ``unknown``).

    Returns:
        ``start``, ``runtime_not_authenticated`` or ``billing_mode_mismatch``.
    """
    if method in _START.get(desired, frozenset()):
        return "start"
    if method == "none":
        return "runtime_not_authenticated"
    return "billing_mode_mismatch"


def evidence(method: str) -> BillingEvidence:
    """Return the ``billing_evidence`` of ``startRun``; never a value or prefix of a key.

    Args:
        method: The ``authMethod`` of the pre-check.

    Returns:
        ``{provider: anthropic, key_type, method: claude_auth_status}``.
    """
    return BillingEvidence(
        key_type=BillingEvidenceKeyType(key_type_for(method)),
        provider="anthropic",
        method="claude_auth_status",
    )


async def auth_status(claude_bin: str, env: Mapping[str, str], cwd: Path, timeout: float) -> str:
    """Run ``<claude_bin> auth status`` with the run environment and read its method.

    Args:
        claude_bin: The CLI found in the cleaned ``PATH``.
        env: The run environment (design note 7).
        cwd: The project directory.
        timeout: ``runner.cli_probe_timeout``.

    Returns:
        The method; ``unknown`` after a start failure or the time limit.
    """
    try:
        proc = await asyncio.create_subprocess_exec(  # nosec B603 - argument list, no shell
            claude_bin,
            "auth",
            "status",
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            cwd=cwd,
            env=dict(env),
            start_new_session=True,
        )
    except OSError:
        _LOG.warning(json.dumps({"event": "auth_status_start_failed"}))
        return "unknown"
    try:
        stdout, _ = await asyncio.wait_for(proc.communicate(), timeout)
    except TimeoutError:
        with contextlib.suppress(ProcessLookupError):
            proc.kill()
        await proc.wait()
        _LOG.warning(json.dumps({"event": "auth_status_timeout"}))
        return "unknown"
    return parse_auth_method(stdout)
