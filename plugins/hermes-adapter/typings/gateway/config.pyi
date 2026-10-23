# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from enum import Enum
from typing import Any

class Platform(Enum):
    LOCAL = "local"

class PlatformConfig:
    enabled: bool
    extra: dict[str, Any]
    def __init__(
        self,
        enabled: bool = ...,
        token: str | None = ...,
        api_key: str | None = ...,
        home_channel: Any = ...,
        reply_to_mode: str = ...,
        gateway_restart_notification: bool = ...,
        typing_indicator: bool = ...,
        typing_status_text: str | None = ...,
        channel_overrides: dict[str, Any] = ...,
        extra: dict[str, Any] = ...,
    ) -> None: ...
