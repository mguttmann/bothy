# SPDX-License-Identifier: Apache-2.0
"""The heartbeat of the Hermes adapter (09 4.8; D-L8; 10 8; M2-20 note 3).

`HermesAdapterLoop` is the SDK's `bothy_sdk.adapter.Adapter` with the Hermes report in every
heartbeat, built with `bothy_sdk.heartbeat.build_heartbeat` inside the closed key set and forms of
``agent_heartbeat.v1`` (docs/DECISIONS.md D-M2-33-11):

* ``state`` ``online``, or ``degraded`` with ``details.degraded_reasons`` from
  ``busy_mode_invalid`` and ``billing_evidence_unknown``;
* ``runtime_version`` the Hermes version; ``reported_billing_mode`` the actual mode and
  ``billing_modes_supported`` exactly that mode, both absent when unknown (Hermes proves one mode
  per profile process); ``billing_evidence`` without any value; ``reported_tool_class``;
* ``details``: ``capabilities`` (``components``, ``title``, ``followups`` unless ``interrupt``),
  ``active_runs``, ``adapter`` ``{name, version}``, ``runtime`` ``{name: hermes-agent}`` and
  ``protocol_version`` from the SDK.

Never ``rate_limit`` and never ``project_readiness``. Each heartbeat also determines the mode
again, so a change during a running turn ends that run (BR-05).

Examples:
    >>> ADAPTER_NAME, RUNTIME_NAME
    ('hermes-adapter', 'hermes-agent')
"""

from __future__ import annotations

from typing import Any, Final, Protocol, runtime_checkable

from bothy_sdk.adapter import Adapter
from bothy_sdk.generated.models import Heartbeat
from bothy_sdk.heartbeat import build_heartbeat

from bothy_hermes_adapter.actual_mode import ActualMode

__all__ = [
    "ADAPTER_NAME",
    "BILLING_UNKNOWN",
    "BUSY_MODE_INVALID",
    "RUNTIME_NAME",
    "HeartbeatReport",
    "HermesAdapterLoop",
    "build_report",
]

ADAPTER_NAME: Final = "hermes-adapter"
"""``details.adapter.name``."""

RUNTIME_NAME: Final = "hermes-agent"
"""``details.runtime.name``."""

BUSY_MODE_INVALID: Final = "busy_mode_invalid"
"""Degraded reason: ``busy_input_mode`` is ``interrupt`` (09 17.5)."""

BILLING_UNKNOWN: Final = "billing_evidence_unknown"
"""Degraded reason: the actual mode cannot be determined (09 17.8)."""


@runtime_checkable
class HeartbeatReport(Protocol):
    """What the handler adds to every heartbeat."""

    def heartbeat_report(self) -> tuple[ActualMode, str, dict[str, Any]]:
        """Return the actual mode (determined now), the tool class and the capabilities."""
        ...


def build_report(
    base: Heartbeat,
    actual: ActualMode,
    tool_class: str,
    capabilities: dict[str, Any],
    runtime_version: str | None,
) -> Heartbeat:
    """Build the full heartbeat from the SDK's base heartbeat and the Hermes report.

    Args:
        base: The SDK's heartbeat (``active_runs``, ``adapter``, ``runtime``,
            ``protocol_version``).
        actual: The actual mode.
        tool_class: The actual tool class.
        capabilities: ``details.capabilities``.
        runtime_version: The Hermes version, if known.

    Returns:
        The heartbeat.
    """
    details = dict(base.to_dict().get("details") or {})
    details.pop("degraded_reasons", None)
    details["capabilities"] = dict(capabilities)
    reasons = []
    if "followups" not in capabilities:
        reasons.append(BUSY_MODE_INVALID)
    if actual.mode is None:
        reasons.append(BILLING_UNKNOWN)
    if reasons:
        details["degraded_reasons"] = reasons
    return build_heartbeat(
        "degraded" if reasons else "online",
        runtime_version=runtime_version,
        reported_billing_mode=actual.mode,
        billing_modes_supported=[actual.mode] if actual.mode is not None else None,
        billing_evidence=actual.evidence,
        reported_tool_class=tool_class,
        details=details,
    )


class HermesAdapterLoop(Adapter):
    """The SDK adapter loop with the Hermes heartbeat; everything else is the SDK's.

    A handler with ``attach(client)`` gets the loop's client for the agent's own writes without
    a run (command replies and reports, 09 10.4, 14).
    """

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        """Build the SDK loop, then attach the handler to its client."""
        super().__init__(*args, **kwargs)
        attach = getattr(self.handler, "attach", None)
        if callable(attach):
            attach(self.client)

    def heartbeat(self) -> Heartbeat:
        """Return the heartbeat with the Hermes report (09 4.8).

        Returns:
            The heartbeat.
        """
        base = super().heartbeat()
        if not isinstance(self.handler, HeartbeatReport):
            return base
        actual, tool_class, capabilities = self.handler.heartbeat_report()
        return build_report(base, actual, tool_class, capabilities, self.config.runtime_version)
