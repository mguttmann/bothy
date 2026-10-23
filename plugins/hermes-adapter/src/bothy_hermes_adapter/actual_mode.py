# SPDX-License-Identifier: Apache-2.0
"""The actual billing mode from the kind of credential Hermes resolves (09 17.8; 10 7, 10.1).

`determine` maps the answer of `bothy_hermes_adapter.runtime.Runtime.resolved_credential` (a
provider and a key type, never a value) with the table of 17.8 (docs/DECISIONS.md D-M2-33-4):

| Resolved | Mode | ``key_type`` |
|---|---|---|
| Anthropic with a console API key | ``organization`` | ``api_key`` |
| Anthropic with OAuth (OAuth wins over an API key) | ``personal`` | ``oauth`` |
| another provider with an API key | ``organization`` | ``api_key`` |
| everything else (no credential, cloud credentials, OAuth of another provider, an exception,
  an unknown answer) | none | ``unknown`` |

An undeterminable mode counts as unequal to every target (10 7, I3), so a run with it fails
with ``billing_mode_mismatch`` and the heartbeat reports ``degraded`` with
``billing_evidence_unknown``.

Examples:
    >>> from bothy_hermes_adapter.runtime import ResolvedCredential
    >>> determine(lambda: ResolvedCredential("anthropic", "oauth")).mode
    'personal'
    >>> determine(lambda: ResolvedCredential(None, "unknown")).evidence
    {'key_type': 'unknown', 'method': 'resolved_credentials'}
"""

from __future__ import annotations

import logging
from collections.abc import Callable
from dataclasses import dataclass
from typing import Final

from bothy_hermes_adapter.runtime import ResolvedCredential

__all__ = ["EVIDENCE_METHOD", "UNKNOWN", "ActualMode", "determine"]

_LOG = logging.getLogger("bothy_hermes_adapter")

EVIDENCE_METHOD: Final = "resolved_credentials"
"""``billing_evidence.method`` of every determination (09 4.8)."""

_ANTHROPIC: Final = "anthropic"
_PROVIDER_FORM: Final = frozenset("abcdefghijklmnopqrstuvwxyz0123456789_.:-")


@dataclass(frozen=True)
class ActualMode:
    """The determined mode with its evidence.

    Attributes:
        mode: ``organization``, ``personal`` or ``None`` when undeterminable.
        provider: The provider for the evidence, ``None`` when unknown.
        key_type: ``api_key``, ``oauth`` or ``unknown``.
    """

    mode: str | None
    provider: str | None
    key_type: str

    @property
    def sdk_mode(self) -> str:
        """The mode as the SDK compares it: ``unknown`` never equals a target."""
        return self.mode or "unknown"

    @property
    def evidence(self) -> dict[str, str]:
        """``billing_evidence`` of ``startRun`` and the heartbeat (no value, no path)."""
        out: dict[str, str] = {}
        if self.provider is not None:
            out["provider"] = self.provider
        out["key_type"] = self.key_type
        out["method"] = EVIDENCE_METHOD
        return out


UNKNOWN: Final = ActualMode(None, None, "unknown")
"""The undeterminable mode (fail closed)."""


def _provider(name: str | None) -> str | None:
    """A provider name in the code form of the heartbeat, else ``None`` (never a free text)."""
    if not name:
        return None
    lowered = name.strip().lower()
    if not lowered or not set(lowered) <= _PROVIDER_FORM or not lowered[0].isalnum():
        return None
    return lowered


def determine(resolve: Callable[[], object]) -> ActualMode:
    """Determine the actual mode now.

    Args:
        resolve: Asks Hermes which credential the next turn uses (kind only).

    Returns:
        The mode; `UNKNOWN` for every answer outside the determinable rows and for a failure
        of ``resolve``, which is logged by its type only.
    """
    try:
        answer = resolve()
    except Exception as exc:  # any failure of the resolution is "not determinable"
        _LOG.warning("credential resolution failed: %s", type(exc).__name__)
        return UNKNOWN
    if not isinstance(answer, ResolvedCredential):
        return UNKNOWN
    provider = _provider(answer.provider)
    if provider is None:
        return UNKNOWN
    if answer.key_type == "oauth" and provider == _ANTHROPIC:
        return ActualMode("personal", provider, "oauth")
    if answer.key_type == "api_key":
        return ActualMode("organization", provider, "api_key")
    return ActualMode(None, provider, "unknown")
