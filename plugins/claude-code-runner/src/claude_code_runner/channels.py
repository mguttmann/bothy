# SPDX-License-Identifier: Apache-2.0
"""Cache of the channels the runner serves and their project bindings (09 4.4, 18.1).

Loaded at start with ``listServedChannels``; a channel is reloaded with
``getServedChannelConfig`` after ``agent.config_changed`` and when an invocation names a newer
``settings_version``. A ``404`` removes the channel. Content (prompts, bindings) is never logged.

Examples:
    >>> ChannelCache.__name__
    'ChannelCache'
"""

from __future__ import annotations

import json
import logging
from pathlib import Path
from uuid import UUID

from bothy_sdk.client import Client
from bothy_sdk.errors import NotFoundError
from bothy_sdk.generated.api.agent_runtime import get_served_channel_config, list_served_channels
from bothy_sdk.generated.models import (
    AgentChannelConfig,
    AgentChannelConfigList,
    ProjectBinding,
    ProjectReadiness,
)

from claude_code_runner.provision import Provisioner, want_of
from claude_code_runner.workspace import readiness

__all__ = ["ChannelCache"]

_LOG = logging.getLogger("claude_code_runner")


class ChannelCache:
    """The served channels with their configuration.

    Args:
        client: The SDK client (calls made outside any run binding).
        projects_root: ``runner.projects_root``.
        provisioner: Creates the folders of bindings with ``provision`` ``init`` or ``clone``
            and answers their readiness with a reason (09 18.12); without it only the plain
            readiness of 18.1 is computed.
    """

    def __init__(
        self, client: Client, projects_root: Path, provisioner: Provisioner | None = None
    ) -> None:
        """Create an empty cache; `load` fills it."""
        self._client = client
        self._root = projects_root
        self._provisioner = provisioner
        self._configs: dict[UUID, AgentChannelConfig] = {}

    async def load(self) -> None:
        """Replace the cache with ``listServedChannels``."""
        with self._client.unbound():
            listed = await self._client.call_as(
                AgentChannelConfigList, list_served_channels.asyncio_detailed
            )
        self._configs = {c.channel_id: c for c in listed.items}
        _LOG.info(json.dumps({"event": "channels_loaded", "count": len(self._configs)}))

    async def reload(self, channel_id: UUID) -> bool:
        """Reload one channel; a ``404`` removes it.

        Args:
            channel_id: The channel.

        Returns:
            True when its readiness or its reason changed.
        """
        before = self._ready(channel_id)
        try:
            with self._client.unbound():
                config = await self._client.call_as(
                    AgentChannelConfig,
                    get_served_channel_config.asyncio_detailed,
                    channel_id=channel_id,
                )
            self._configs[channel_id] = config
        except NotFoundError:
            self._configs.pop(channel_id, None)
        _LOG.info(json.dumps({"event": "channel_reloaded", "channel_id": str(channel_id)}))
        return before != self._ready(channel_id)

    def get(self, channel_id: UUID) -> AgentChannelConfig | None:
        """Return the cached configuration of a channel.

        Args:
            channel_id: The channel.

        Returns:
            The configuration, or ``None``.
        """
        return self._configs.get(channel_id)

    def settings_version(self, channel_id: UUID) -> int | None:
        """Return the cached ``settings_version`` of a channel.

        Args:
            channel_id: The channel.

        Returns:
            The version, or ``None`` for an unknown channel.
        """
        config = self._configs.get(channel_id)
        return None if config is None else config.behavior.settings_version

    def binding(self, channel_id: UUID) -> ProjectBinding | None:
        """Return the project binding of a served forum.

        Args:
            channel_id: The channel.

        Returns:
            The binding, or ``None`` when the channel has none or is not served.
        """
        config = self._configs.get(channel_id)
        binding = config.behavior.project_binding if config is not None else None
        return binding if isinstance(binding, ProjectBinding) else None

    def _ready(self, channel_id: UUID) -> tuple[bool, str | None] | None:
        binding = self.binding(channel_id)
        if binding is None:
            return None
        if self._provisioner is None:
            return readiness(self._root, binding.workspace_ref)
        return self._provisioner.readiness(channel_id, want_of(binding))

    def readiness(self) -> list[ProjectReadiness]:
        """Return the readiness of every bound forum (computed now) with its reason.

        A forum that is not ready carries the reason of 09 4.8 and 18.12; a due folder creation
        starts here.

        Returns:
            One entry per channel with a binding.
        """
        out = []
        for channel_id in self._configs:
            state = self._ready(channel_id)
            if state is None:
                continue
            ready, reason = state
            entry = ProjectReadiness(channel_id=channel_id, ready=ready)
            if not ready and reason:
                entry.reason = reason
            out.append(entry)
        return out
