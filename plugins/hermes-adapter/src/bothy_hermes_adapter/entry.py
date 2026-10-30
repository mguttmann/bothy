# SPDX-License-Identifier: Apache-2.0
"""The plugin entry: ``register(ctx)`` without importing Hermes or the SDK at load time.

Hermes loads plugins in a background thread while its main thread still imports the gateway
(``hermes gateway run``), and it waits only a bounded time for that discovery. A plugin that
imports gateway modules at load time races those imports (CPython then fails one of them with a
``KeyError`` in ``_load_unlocked``), and a slow load lets Hermes start without the platform.
So this module imports neither Hermes nor the SDK; `bothy_hermes_adapter.platform`, which
imports both, is imported when Hermes builds the adapter (docs/DECISIONS.md D-M2-33-13), and
`bothy_hermes_adapter.reports` when a process without the gateway sends a report.

Cron delivery (09 17.9): ``cron_deliver_env_var`` makes ``deliver=<platform key>`` a valid cron
target (the platform's home channel), ``parse_target_ref_fn`` reads the explicit targets
``<platform key>:<channel ID>`` and ``<platform key>:<channel ID>:<thread ID>``
(`parse_target_ref`), and ``standalone_sender_fn`` sends a report from a process that runs no
gateway (``hermes cron run``, ``hermes send``).

Examples:
    >>> callable(make_register("bothy"))
    True
"""

from __future__ import annotations

import sys
from typing import Any, Final
from uuid import UUID

from bothy_hermes_adapter.defaults import NO_SPLIT_LENGTH

__all__ = ["HOOKS", "make_register", "parse_target_ref", "standalone_send"]

HOOKS: Final = ("pre_llm_call", "on_session_reset", "post_api_request")
"""The Hermes plugin hooks the platform listens to."""

_PLATFORM_MODULE: Final = "bothy_hermes_adapter.platform"


def parse_target_ref(target_ref: str) -> tuple[str, str | None] | None:
    """Hermes' ``parse_target_ref_fn``: ``<channel ID>`` or ``<channel ID>:<thread ID>``.

    Args:
        target_ref: The part of a target after ``<platform key>:``.

    Returns:
        Channel and thread in canonical form, or ``None`` for anything else (Hermes then
        reports the target as not resolvable).

    Examples:
        >>> parse_target_ref("0192F4A0-7B1C-7000-8000-000000000001")
        ('0192f4a0-7b1c-7000-8000-000000000001', None)
        >>> parse_target_ref("#general") is None
        True
    """
    parts = str(target_ref or "").strip().split(":")
    if len(parts) > 2:
        return None
    try:
        ids = [str(UUID(part.strip())) for part in parts]
    except ValueError:
        return None
    return ids[0], ids[1] if len(ids) == 2 else None


async def standalone_send(pconfig: Any, chat_id: str, message: str, **kwargs: Any) -> Any:
    """Hermes' ``standalone_sender_fn``: `bothy_hermes_adapter.reports.standalone_send`.

    Args:
        pconfig: Hermes' platform configuration.
        chat_id: The channel.
        message: The report.
        **kwargs: ``thread_id``, ``media_files`` and what else Hermes passes.

    Returns:
        Hermes' result dict.
    """
    from bothy_hermes_adapter.reports import standalone_send as send  # noqa: PLC0415 - see doc

    return await send(pconfig, chat_id, message, **kwargs)


def make_register(platform_key: str) -> Any:
    """Build the plugin's ``register(ctx)`` for a platform key (the installer's entry calls it).

    Args:
        platform_key: `hermes_adapter.platform_key`.

    Returns:
        ``register(ctx)``: registers the platform and the hooks.
    """

    def factory(config: Any) -> Any:
        from bothy_hermes_adapter.platform import ServerPlatform  # noqa: PLC0415 - see module doc

        return ServerPlatform(config, platform_key)

    def hook(name: str) -> Any:
        def callback(**kwargs: Any) -> None:
            if str(kwargs.get("platform") or "") != platform_key:
                return
            # No platform exists before the factory imported its module; a hook never imports it.
            module = sys.modules.get(_PLATFORM_MODULE)
            if module is not None:
                module.dispatch_hook(platform_key, name, kwargs)

        return callback

    def register(ctx: Any) -> None:
        ctx.register_platform(
            name=platform_key,
            label="Chat server adapter",
            adapter_factory=factory,
            check_fn=lambda: True,
            allowed_users_env=f"{platform_key.upper()}_ALLOWED_USERS",
            allow_all_env=f"{platform_key.upper()}_ALLOW_ALL_USERS",
            # Hermes never splits an answer; the adapter keeps one message (M4-47).
            max_message_length=NO_SPLIT_LENGTH,
            cron_deliver_env_var=f"{platform_key.upper()}_HOME_CHANNEL",
            parse_target_ref_fn=parse_target_ref,
            standalone_sender_fn=standalone_send,
        )
        for name in HOOKS:
            ctx.register_hook(name, hook(name))

    return register
