# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from enum import Enum
from typing import Any

class MessageType(Enum):
    TEXT = "text"

class ProcessingOutcome(Enum):
    SUCCESS = "success"
    FAILURE = "failure"
    CANCELLED = "cancelled"

class MessageEvent:
    text: str
    message_id: str | None
    user_id: str | None
    user_name: str | None
    channel_prompt: str | None
    source: Any
    def __init__(
        self,
        text: str,
        message_type: MessageType = ...,
        user_id: str | None = ...,
        user_name: str | None = ...,
        source: Any = ...,
        raw_message: Any = ...,
        message_id: str | None = ...,
        ledger_message_id: str | None = ...,
        reply_anchor_override: str | None = ...,
        platform_update_id: int | None = ...,
        media_urls: list[str] = ...,
        media_types: list[str] = ...,
        media_text_inlined: list[bool | None] = ...,
        reply_to_message_id: str | None = ...,
        reply_to_text: str | None = ...,
        reply_to_author_id: str | None = ...,
        reply_to_author_name: str | None = ...,
        reply_to_is_own_message: bool = ...,
        prompt_response: dict[str, Any] | None = ...,
        auto_skill: str | list[str] | None = ...,
        channel_prompt: str | None = ...,
        channel_context: str | None = ...,
        internal: bool = ...,
        metadata: dict[str, Any] = ...,
        timestamp: Any = ...,
        allow_gateway_control: bool = ...,
        reply_expected: bool | None = ...,
        inherit_channel_pin: bool = ...,
        edit_revision: str | None = ...,
    ) -> None: ...
