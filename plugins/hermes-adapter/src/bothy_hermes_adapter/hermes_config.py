# SPDX-License-Identifier: Apache-2.0
"""The configuration bridge to Hermes: its own YAML reader, credential resolution and toolsets.

One of the two modules that import Hermes (docs/DECISIONS.md D-M2-33-1); every function here
uses Hermes's own code so the adapter and the installer see what Hermes sees (09 17.5, 17.8;
10 10.1):

* `resolved_credential` asks Hermes which credential the next turn uses, with the function Hermes
  itself chooses the login with (``hermes_cli.runtime_provider.resolve_runtime_provider`` and
  ``agent.anthropic_credentials.anthropic_route_is_oauth``; no public surface exists in the pinned
  version, 17.8, RK-12). It returns only the provider and the key type; the value stays in this
  function and is never logged, formatted, stored or returned.
* `busy_input_mode` and `enabled_toolsets` read the profile configuration.
* The installer functions read and write ``config.yaml`` and ``.env`` of a profile home with
  Hermes's own reader and writer (``hermes_cli.config``), under a context-local home override.

The stubs in ``typings/`` describe exactly the Hermes symbols this module and ``platform.py`` use;
``test_hermes_stubs_match_pinned_version`` holds them equal to the pinned version.
"""

from __future__ import annotations

import contextlib
from collections.abc import Iterator, Mapping
from pathlib import Path
from typing import Any, Final

from agent.anthropic_credentials import anthropic_route_is_oauth
from hermes_cli.auth import PROVIDER_REGISTRY
from hermes_cli.config import (
    load_config,
    read_user_config_raw,
    remove_env_value,
    save_config,
    save_env_value,
)
from hermes_cli.env_loader import load_hermes_dotenv
from hermes_cli.runtime_provider import resolve_runtime_provider
from hermes_cli.tools_config import _get_platform_tools
from hermes_constants import reset_hermes_home_override, set_hermes_home_override

from bothy_hermes_adapter.runtime import ResolvedCredential

__all__ = [
    "API_KEY_VARIABLE",
    "OAUTH_VARIABLES",
    "busy_input_mode",
    "enabled_toolsets",
    "env_defines",
    "profile_home",
    "read_config",
    "remove_env",
    "resolved_credential",
    "set_env",
    "write_config",
]

API_KEY_VARIABLE: Final = "ANTHROPIC_API_KEY"
"""The console API key variable of the profile environment (M2-32 record, 10 10.1)."""

OAUTH_VARIABLES: Final = ("ANTHROPIC_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN")
"""The OAuth variables of the profile environment (M2-32 record, 10 10.1)."""

_KEYLESS: Final = frozenset({"", "no-key-required"})


def resolved_credential() -> ResolvedCredential:
    """The kind of credential Hermes resolves for the next turn (09 17.8).

    Returns:
        ``oauth`` for an Anthropic OAuth route, ``api_key`` for a non-empty key of an API-key
        provider, ``unknown`` for everything else (no credential, cloud or OAuth logins of other
        providers, an external process, a resolution error).
    """
    runtime: Mapping[str, Any] = resolve_runtime_provider()
    provider = runtime.get("provider")
    name = str(provider) if isinstance(provider, str) and provider else None
    credential = runtime.get("api_key")
    try:
        if runtime.get("auth_error"):
            return ResolvedCredential(name, "unknown")
        if anthropic_route_is_oauth(runtime.get("base_url"), credential, provider=name):
            return ResolvedCredential(name, "oauth")
        entry = PROVIDER_REGISTRY.get(name or "")
        auth_type = getattr(entry, "auth_type", "api_key") if entry is not None else "api_key"
        usable = isinstance(credential, str) and credential.strip() not in _KEYLESS
        if usable and auth_type == "api_key":
            return ResolvedCredential(name, "api_key")
        return ResolvedCredential(name, "unknown")
    finally:
        # The value never leaves this frame.
        credential = None
        runtime = {}


def busy_input_mode() -> str:
    """The global ``display.busy_input_mode`` Hermes applies (09 17.5).

    Returns:
        ``steer``, ``queue``, or ``interrupt`` (also for an absent or unknown value).
    """
    display = load_config().get("display") or {}
    mode = str(display.get("busy_input_mode") or "interrupt").strip().lower()
    return mode if mode in ("steer", "queue") else "interrupt"


def enabled_toolsets(platform_key: str) -> frozenset[str]:
    """The toolsets Hermes enables for the platform (09 17.11).

    Args:
        platform_key: `hermes_adapter.platform_key`.

    Returns:
        The toolset names (MCP server names included).
    """
    return frozenset(str(t) for t in _get_platform_tools(load_config(), platform_key))


@contextlib.contextmanager
def profile_home(home: Path) -> Iterator[None]:
    """Point Hermes's readers and writers at a profile home for the block (installer).

    Args:
        home: The profile's Hermes home.

    Yields:
        Nothing.
    """
    token = set_hermes_home_override(home)
    try:
        yield
    finally:
        reset_hermes_home_override(token)


def read_config() -> dict[str, Any]:
    """``config.yaml`` of the current home, exactly as written (Hermes's own reader).

    Returns:
        The mapping (empty when the file is missing).
    """
    return dict(read_user_config_raw())


def write_config(config: Mapping[str, Any]) -> None:
    """Write ``config.yaml`` of the current home with Hermes's own writer.

    Args:
        config: The whole configuration.
    """
    save_config(dict(config), strip_defaults=False)


def env_defines(home: Path, name: str) -> bool:
    """Whether the profile environment sets a variable, judged by its name only.

    The value is never read into this process: a line counts when it assigns the name a
    non-empty value.

    Args:
        home: The profile home.
        name: The variable name.

    Returns:
        True when ``.env`` of the home sets the variable.
    """
    path = home / ".env"
    if not path.is_file():
        return False
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip().removeprefix("export ").strip()
        key, sep, rest = line.partition("=")
        if sep and key.strip() == name and rest.strip() not in ("", '""', "''"):
            return True
    return False


def set_env(name: str, value: str) -> None:
    """Set a variable of the profile environment (``.env``) with Hermes's writer.

    Args:
        name: The variable name.
        value: The value (only principal IDs, never a credential).
    """
    save_env_value(name, value)


def remove_env(name: str) -> None:
    """Remove a variable from the profile environment with Hermes's writer.

    Args:
        name: The variable name.
    """
    remove_env_value(name)


def predict(home: Path) -> ResolvedCredential:
    """The credential kind a fresh gateway of this profile would resolve (installer).

    Args:
        home: The profile home.

    Returns:
        Provider and key type.
    """
    with profile_home(home):
        load_hermes_dotenv(hermes_home=home)
        return resolved_credential()
