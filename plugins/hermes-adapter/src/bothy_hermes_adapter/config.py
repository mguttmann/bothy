# SPDX-License-Identifier: Apache-2.0
"""The adapter's configuration from ``PlatformConfig.extra`` (docs/spec/09-agents.md 17.2, 21).

Hermes hands every platform its section of the profile configuration; the installer writes the
adapter's keys there flat, with their full names. `load_settings` validates them into the frozen
`AdapterSettings` and fails with a message that names the key and never the value. A key with a
dot that is not one of `KNOWN_KEYS` is refused (a typo must not fall back to a default); keys
without a dot are Hermes's own platform settings (for example ``dm_policy``) and are left to
Hermes. The adapter never reads the process environment (17.2): the API token comes from the
file `hermes_adapter.token_file` names.

Examples:
    >>> s = load_settings({
    ...     "hermes_adapter.server_url": "https://example.com/api/v1",
    ...     "hermes_adapter.instance_id": "0192f4a0-7b1c-7000-8000-000000000001",
    ...     "hermes_adapter.token_file": "/srv/hermes/bothy/token",
    ... })
    >>> (s.platform_key, s.consumer, s.max_message_bytes)
    ('bothy', 'default', 262144)
"""

from __future__ import annotations

import os
import re
import stat
from collections.abc import Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Final
from uuid import UUID

from bothy_sdk.auth import Secret
from bothy_sdk.settings import SdkSettings

__all__ = [
    "DEFAULT_ATTACHMENT_TEXT_MAX_BYTES",
    "DEFAULT_MAX_MESSAGE_BYTES",
    "KNOWN_KEYS",
    "AdapterSettings",
    "ConfigError",
    "load_settings",
    "load_token",
]

DEFAULT_MAX_MESSAGE_BYTES: Final = 262144
"""Default of `hermes_adapter.max_message_bytes`: the default of ``message.max_content_bytes``."""

DEFAULT_ATTACHMENT_TEXT_MAX_BYTES: Final = 32768
"""Default of `hermes_adapter.attachment_text_max_bytes`: the default of
``agent.context_message_max_bytes``."""

_SDK_KEYS: Final = (
    "adapter.invocation_token_leeway",
    "adapter.jwks_refresh_min_interval",
    "sdk.http_timeout",
    "sdk.retry_max_attempts",
    "sdk.retry_backoff_base",
    "sdk.retry_backoff_cap",
)
_OWN_KEYS: Final = (
    "hermes_adapter.platform_key",
    "hermes_adapter.consumer",
    "hermes_adapter.server_url",
    "hermes_adapter.instance_id",
    "hermes_adapter.token_file",
    "hermes_adapter.dpop_key_file",
    "hermes_adapter.max_message_bytes",
    "hermes_adapter.attachment_text_max_bytes",
)
KNOWN_KEYS: Final = frozenset((*_OWN_KEYS, *_SDK_KEYS))
"""Every dotted key the adapter reads from ``PlatformConfig.extra``."""

_PLATFORM_KEY: Final = re.compile(r"[a-z][a-z0-9_]{0,39}")
_CONSUMER: Final = re.compile(r"[a-z0-9][a-z0-9_-]{0,63}")
_REQUIRED: Final = (
    "hermes_adapter.server_url",
    "hermes_adapter.instance_id",
    "hermes_adapter.token_file",
)


class ConfigError(ValueError):
    """A configuration key is missing, unknown or invalid; the text names the key only."""


@dataclass(frozen=True)
class AdapterSettings:
    """The validated configuration of one Hermes profile's adapter.

    Attributes:
        platform_key: ``hermes_adapter.platform_key``, the platform name in Hermes.
        consumer: ``hermes_adapter.consumer``, the consumer at the gateway.
        server_url: ``hermes_adapter.server_url``, the API base of the instance.
        instance_id: ``hermes_adapter.instance_id``, the expected ``iss`` of every token.
        token_file: ``hermes_adapter.token_file``, the file with the agent's API token.
        dpop_key_file: ``hermes_adapter.dpop_key_file``, the DPoP key, ``None`` without DPoP.
        max_message_bytes: ``hermes_adapter.max_message_bytes``, the longest answer.
        attachment_text_max_bytes: ``hermes_adapter.attachment_text_max_bytes``, the longest
            attachment text inlined into the context.
        sdk: The SDK settings of ``adapter.*`` and ``sdk.*``.
    """

    platform_key: str
    consumer: str
    server_url: str
    instance_id: UUID
    token_file: Path
    dpop_key_file: Path | None = None
    max_message_bytes: int = DEFAULT_MAX_MESSAGE_BYTES
    attachment_text_max_bytes: int = DEFAULT_ATTACHMENT_TEXT_MAX_BYTES
    sdk: SdkSettings = field(default_factory=SdkSettings)


def _text(extra: Mapping[str, Any], key: str, default: str | None = None) -> str:
    value = extra.get(key, default)
    if value is None:
        raise ConfigError(f"{key}: required")
    if not isinstance(value, str) or not value.strip():
        raise ConfigError(f"{key}: must be a non-empty text")
    return value.strip()


def _positive(extra: Mapping[str, Any], key: str, default: int) -> int:
    value = extra.get(key, default)
    if isinstance(value, bool) or not isinstance(value, int | str):
        raise ConfigError(f"{key}: must be a positive integer")
    try:
        number = int(value)
    except ValueError:
        raise ConfigError(f"{key}: must be a positive integer") from None
    if number < 1:
        raise ConfigError(f"{key}: must be a positive integer")
    return number


def load_settings(extra: Mapping[str, Any]) -> AdapterSettings:
    """Validate the adapter's keys of ``PlatformConfig.extra``.

    Args:
        extra: The platform's ``extra`` mapping as Hermes hands it to the plugin.

    Returns:
        The settings.

    Raises:
        ConfigError: A required key is missing, a dotted key is unknown, or a value does not
            parse; the message names the key and never the value.
    """
    unknown = sorted(k for k in extra if "." in str(k) and k not in KNOWN_KEYS)
    if unknown:
        raise ConfigError(f"{unknown[0]}: unknown configuration key")
    for key in _REQUIRED:
        _text(extra, key)
    platform_key = _text(extra, "hermes_adapter.platform_key", "bothy")
    if not _PLATFORM_KEY.fullmatch(platform_key):
        raise ConfigError("hermes_adapter.platform_key: must match [a-z][a-z0-9_]*")
    consumer = _text(extra, "hermes_adapter.consumer", "default")
    if not _CONSUMER.fullmatch(consumer):
        raise ConfigError("hermes_adapter.consumer: must match [a-z0-9][a-z0-9_-]*")
    try:
        instance_id = UUID(_text(extra, "hermes_adapter.instance_id"))
    except ValueError:
        raise ConfigError("hermes_adapter.instance_id: must be a UUID") from None
    dpop = extra.get("hermes_adapter.dpop_key_file")
    sdk_values = {k: str(extra[k]) for k in _SDK_KEYS if k in extra}
    try:
        sdk = SdkSettings.from_mapping(sdk_values)
    except ValueError as exc:
        # The SDK's message names the key; keep only the key part, never a value.
        name = next((k for k in sdk_values if k in str(exc)), "sdk")
        raise ConfigError(f"{name}: invalid value") from None
    return AdapterSettings(
        platform_key=platform_key,
        consumer=consumer,
        server_url=_text(extra, "hermes_adapter.server_url"),
        instance_id=instance_id,
        token_file=Path(_text(extra, "hermes_adapter.token_file")),
        dpop_key_file=Path(_text(extra, "hermes_adapter.dpop_key_file")) if dpop else None,
        max_message_bytes=_positive(
            extra, "hermes_adapter.max_message_bytes", DEFAULT_MAX_MESSAGE_BYTES
        ),
        attachment_text_max_bytes=_positive(
            extra, "hermes_adapter.attachment_text_max_bytes", DEFAULT_ATTACHMENT_TEXT_MAX_BYTES
        ),
        sdk=sdk,
    )


def load_token(path: Path) -> Secret:
    """Read the agent's API token from its file.

    The file must be a regular file that neither the group nor others may read or write, like
    the SDK's state files (POSIX). The value is wrapped in `bothy_sdk.auth.Secret` at once and
    never appears in an error text.

    Args:
        path: `AdapterSettings.token_file`.

    Returns:
        The token.

    Raises:
        ConfigError: The file is missing, not a regular file, open to group or others, or empty.
    """
    key = "hermes_adapter.token_file"
    try:
        info = path.stat()
    except OSError:
        raise ConfigError(f"{key}: file not readable") from None
    if not stat.S_ISREG(info.st_mode):
        raise ConfigError(f"{key}: not a regular file")
    if os.name == "posix" and info.st_mode & (stat.S_IRWXG | stat.S_IRWXO):
        raise ConfigError(f"{key}: readable or writable by group or others")
    value = path.read_text(encoding="utf-8").strip()
    if not value:
        raise ConfigError(f"{key}: empty")
    return Secret(value)
