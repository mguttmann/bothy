# SPDX-License-Identifier: Apache-2.0
"""Named values the hand layer works with (docs/spec/11-extensions.md 16).

Every limit, window and wait of the SDK is a field of `SdkSettings`; each field names its
configuration key in ``metadata["key"]`` and its default is the "Default" column of
``docs/operations/configuration.md`` (a test holds both equal). The server-side limits
(``component.*``, ``command.*``, ``agent.run_update_min_interval``) are only the defaults of the
SDK's own early checks: the server stays the authority and its ``400 validation_failed`` reaches
the caller as usual (docs/DECISIONS.md D-M2-29-2).

Examples:
    >>> s = SdkSettings.from_mapping({"sdk.http_timeout": "5s", "sdk.retry_max_attempts": "3"})
    >>> (s.http_timeout, s.retry_max_attempts)
    (5.0, 3)
"""

from __future__ import annotations

import dataclasses
import math
import re
from collections.abc import Mapping
from dataclasses import dataclass, field
from typing import Any

__all__ = ["SdkSettings", "parse_duration"]

_DURATION = re.compile(r"^(\d+(?:\.\d+)?)(ms|s|m|h)$")
_UNIT_SECONDS = {"ms": 0.001, "s": 1.0, "m": 60.0, "h": 3600.0}


def parse_duration(text: str) -> float:
    """Parse a duration of the configuration reference into seconds.

    Args:
        text: A number with one of the units ``ms``, ``s``, ``m`` or ``h``, for example
            ``"500ms"`` or ``"30s"``.

    Returns:
        The duration in seconds.

    Raises:
        ValueError: The text is not such a duration.

    Examples:
        >>> parse_duration("500ms")
        0.5
    """
    match = _DURATION.match(text.strip())
    if match is None:
        raise ValueError(f"not a duration: {text!r}")
    return float(match.group(1)) * _UNIT_SECONDS[match.group(2)]


@dataclass(frozen=True)
class SdkSettings:
    """The named values of the SDK; durations in seconds, counts and sizes as integers.

    Every field carries its configuration key in ``metadata["key"]``; the defaults below are
    the "Default" column of ``docs/operations/configuration.md``.

    Attributes:
        http_timeout: ``sdk.http_timeout``, bound of one REST request.
        retry_max_attempts: ``sdk.retry_max_attempts``, attempts per idempotent operation.
        retry_backoff_base: ``sdk.retry_backoff_base``, base of the retry backoff.
        retry_backoff_cap: ``sdk.retry_backoff_cap``, largest retry wait.
        poll_interval: ``sdk.poll_interval``, wait between polls after an empty page.
        invocation_token_leeway: ``adapter.invocation_token_leeway``, clock tolerance of the
            invocation token check.
        jwks_refresh_min_interval: ``adapter.jwks_refresh_min_interval``, at most one forced
            JWKS reload per interval.
        run_update_min_interval: ``agent.run_update_min_interval``, throttle of run patches.
        reconnect_backoff_base: ``sync.reconnect_backoff_base``, base of the gateway reconnect.
        reconnect_backoff_cap: ``sync.reconnect_backoff_cap``, largest reconnect wait.
        ws_max_frame_bytes: ``sync.ws_max_frame_bytes``, largest gateway frame received.
        component_max_per_message: ``component.max_per_message``.
        component_custom_id_max_chars: ``component.custom_id_max_chars``.
        component_form_max_fields: ``component.form_max_fields``.
        autocomplete_max_choices: ``command.autocomplete_max_choices``.
        command_description_max_chars: ``command.description_max_chars``.
    """

    http_timeout: float = field(default=30.0, metadata={"key": "sdk.http_timeout"})
    retry_max_attempts: int = field(default=5, metadata={"key": "sdk.retry_max_attempts"})
    retry_backoff_base: float = field(default=0.5, metadata={"key": "sdk.retry_backoff_base"})
    retry_backoff_cap: float = field(default=30.0, metadata={"key": "sdk.retry_backoff_cap"})
    poll_interval: float = field(default=2.0, metadata={"key": "sdk.poll_interval"})
    invocation_token_leeway: float = field(
        default=10.0, metadata={"key": "adapter.invocation_token_leeway"}
    )
    jwks_refresh_min_interval: float = field(
        default=60.0, metadata={"key": "adapter.jwks_refresh_min_interval"}
    )
    run_update_min_interval: float = field(
        default=0.5, metadata={"key": "agent.run_update_min_interval"}
    )
    reconnect_backoff_base: float = field(
        default=1.0, metadata={"key": "sync.reconnect_backoff_base"}
    )
    reconnect_backoff_cap: float = field(
        default=60.0, metadata={"key": "sync.reconnect_backoff_cap"}
    )
    ws_max_frame_bytes: int = field(default=1048576, metadata={"key": "sync.ws_max_frame_bytes"})
    component_max_per_message: int = field(
        default=25, metadata={"key": "component.max_per_message"}
    )
    component_custom_id_max_chars: int = field(
        default=100, metadata={"key": "component.custom_id_max_chars"}
    )
    component_form_max_fields: int = field(
        default=10, metadata={"key": "component.form_max_fields"}
    )
    autocomplete_max_choices: int = field(
        default=25, metadata={"key": "command.autocomplete_max_choices"}
    )
    command_description_max_chars: int = field(
        default=100, metadata={"key": "command.description_max_chars"}
    )

    @classmethod
    def keys(cls) -> dict[str, str]:
        """Return the configuration key of every field.

        Returns:
            A mapping from field name to dotted configuration key.
        """
        return {f.name: f.metadata["key"] for f in dataclasses.fields(cls)}

    @classmethod
    def from_mapping(cls, values: Mapping[str, str]) -> SdkSettings:
        """Build settings from dotted configuration keys; absent keys keep their default.

        Args:
            values: Dotted keys (``"sdk.http_timeout"``) to their text form; durations as
                ``"500ms"``, ``"30s"``, ``"2m"``; counts and sizes as decimal integers.

        Returns:
            The settings.

        Raises:
            ValueError: A key is unknown, a value does not parse or is not positive; the message
                names the key.
        """
        by_key = {f.metadata["key"]: f for f in dataclasses.fields(cls)}
        kwargs: dict[str, Any] = {}
        for key, text in values.items():
            f = by_key.get(key)
            if f is None:
                raise ValueError(f"unknown SDK setting: {key}")
            try:
                value = int(text) if f.type in ("int", int) else parse_duration(text)
            except ValueError as exc:
                raise ValueError(f"invalid value for {key}") from exc
            # Every named value is a positive count, size or wait; zero would poll or retry
            # without pause or never try at all.
            if not (math.isfinite(value) and value > 0):
                raise ValueError(f"invalid value for {key}: must be positive")
            kwargs[f.name] = value
        return cls(**kwargs)
