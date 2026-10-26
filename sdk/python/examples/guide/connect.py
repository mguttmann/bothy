# SPDX-License-Identifier: Apache-2.0
"""Step 4: connect to the gateway as a consumer.

The SDK holds the connection: it identifies with the agent token and a DPoP proof of the
adapter key, reconnects with backoff, stores every event with its token in the inbox before it
acknowledges it (``ack``) and keeps the consumer's cursor on the server. The instance ID and the
agent ID come from the owner once; the SDK checks every invocation token against them.
"""

from __future__ import annotations

import stat
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import Any
from uuid import UUID

from bothy_sdk.adapter import Adapter, AdapterConfig
from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.commands import CommandSpec
from bothy_sdk.generated.api.me import get_me
from bothy_sdk.generated.models import Me
from bothy_sdk.inbox import EntryState

ADAPTER_NAME = "guide-example"
ADAPTER_VERSION = "1.0.0"
TOKEN_FILE = "agent.token"  # noqa: S105  # nosec B105 - a file name, not a secret
KEY_FILE = "adapter.key"


def report(step: int, text: str) -> None:
    """Print one confirmed action as ``Step <n>: <text>``.

    Never pass a message text, a prompt, a token or a person's name: stdout is a log.

    Args:
        step: The step of the guide that explains the action.
        text: What happened.
    """
    print(f"Step {step}: {text}", flush=True)


def credentials(adapter_dir: Path) -> Credentials:
    """Load the agent token and the adapter key, refusing files others can read.

    Args:
        adapter_dir: The adapter's folder.

    Returns:
        The token bound to the key (DPoP).

    Raises:
        SystemExit: A file is readable or writable by group or others.
    """
    for name in (TOKEN_FILE, KEY_FILE):
        path = adapter_dir / name
        if path.stat().st_mode & (stat.S_IRWXG | stat.S_IRWXO):
            raise SystemExit(f"{path} must be accessible by the adapter only (chmod 600 {path})")
    return Credentials(
        Secret.from_file(adapter_dir / TOKEN_FILE), FileDpopKey.load(adapter_dir / KEY_FILE)
    )


def adapter_config(  # noqa: PLR0913 - the settings of the command line
    *,
    server_url: str,
    instance_id: UUID,
    agent_id: UUID,
    adapter_dir: Path,
    consumer: str,
    capabilities: Mapping[str, Any],
    commands: Sequence[CommandSpec],
) -> AdapterConfig:
    """Configure the SDK's adapter for the gateway.

    Args:
        server_url: The address of the instance.
        instance_id: The instance ID from the owner.
        agent_id: The agent ID from the owner.
        adapter_dir: The adapter's folder; ``state/`` keeps the inbox and the outbox.
        consumer: The consumer name; each consumer has its own cursor.
        capabilities: The heartbeat capabilities (step 5).
        commands: The declared commands (step 6).

    Returns:
        The configuration.
    """
    state = adapter_dir / "state"
    state.mkdir(mode=0o700, exist_ok=True)
    return AdapterConfig(
        base_url=f"{server_url.rstrip('/')}/api/v1",
        instance_id=instance_id,
        agent_id=agent_id,
        credentials=credentials(adapter_dir),
        consumer=consumer,
        transport="gateway",
        state_dir=state,
        adapter_name=ADAPTER_NAME,
        adapter_version=ADAPTER_VERSION,
        runtime_name="guide-rules",
        capabilities=capabilities,
        commands=commands,
    )


async def accepted_runs(adapter: Adapter) -> set[UUID]:
    """Return the runs the inbox holds accepted but not started (a restart resumes them).

    Args:
        adapter: The adapter before `Adapter.run`.

    Returns:
        Their run IDs.
    """
    entries = await adapter.inbox.in_states(EntryState.ACCEPTED)
    return {e.run_id for e in entries if e.run_id is not None}


async def announce(adapter: Adapter) -> None:
    """Print that the adapter is connected, with the agent's handle (``getMe``).

    Args:
        adapter: The connected adapter.
    """
    with adapter.client.unbound():
        me = await adapter.client.call_as(Me, get_me.asyncio_detailed)
    report(4, f"connected as {me.principal.handle} with consumer {adapter.config.consumer}")
