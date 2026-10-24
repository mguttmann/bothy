# SPDX-License-Identifier: Apache-2.0
"""The plugin entry: ``register(ctx)`` without importing Hermes or the SDK at load time.

Hermes loads plugins in a background thread while its main thread still imports the gateway
(``hermes gateway run``), and it waits only a bounded time for that discovery. A plugin that
imports gateway modules at load time races those imports (CPython then fails one of them with a
``KeyError`` in ``_load_unlocked``), and a slow load lets Hermes start without the platform.
So this module imports neither Hermes nor the SDK; `bothy_hermes_adapter.platform`, which
imports both, is imported when Hermes builds the adapter (docs/DECISIONS.md D-M2-33-13).

Examples:
    >>> callable(make_register("bothy"))
    True
"""

from __future__ import annotations

import sys
from typing import Any, Final

from bothy_hermes_adapter.defaults import DEFAULT_MAX_MESSAGE_BYTES

__all__ = ["HOOKS", "make_register"]

HOOKS: Final = ("pre_llm_call", "on_session_reset", "post_api_request")
"""The Hermes plugin hooks the platform listens to."""

_PLATFORM_MODULE: Final = "bothy_hermes_adapter.platform"


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
            max_message_length=DEFAULT_MAX_MESSAGE_BYTES,
        )
        for name in HOOKS:
            ctx.register_hook(name, hook(name))

    return register
