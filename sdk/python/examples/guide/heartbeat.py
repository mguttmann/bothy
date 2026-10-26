# SPDX-License-Identifier: Apache-2.0
"""Step 5: report what the adapter can do and how it bills.

The SDK sends this report with every gateway heartbeat. Right after connecting, the example
sends it once with ``sendHeartbeat`` as well, because that answer confirms the report and shows
the presence the server derived from it.
"""

from __future__ import annotations

from connect import report

from bothy_sdk.adapter import Adapter, RuntimeStatus
from bothy_sdk.generated.api.agent_runtime import send_heartbeat
from bothy_sdk.generated.models import HeartbeatAck

CAPABILITIES = {"followups": "steer", "components": True, "files_out": True}
"""What the adapter handles (09 2.2): follow-ups steer the running run, it sends cards and files."""

STATUS = RuntimeStatus(
    state="online",
    reported_billing_mode="organization",
    billing_modes_supported=("organization",),
    # Where the runtime's key comes from, never the key itself: the example uses none.
    billing_evidence={"key_type": "none"},
)
"""The billing part of every heartbeat."""


async def report_once(adapter: Adapter) -> None:
    """Send the heartbeat now and print the presence the server answered.

    Args:
        adapter: The connected adapter.
    """
    with adapter.client.unbound():
        ack = await adapter.client.call_as(
            HeartbeatAck, send_heartbeat.asyncio_detailed, body=adapter.heartbeat()
        )
    presence = ack.presence
    report(
        5,
        f"heartbeat accepted: {presence.state}, mode {presence.reported_billing_mode}, "
        f"tool class {presence.reported_tool_class}",
    )
