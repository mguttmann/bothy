# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from typing import Any

from gateway.config import Platform, PlatformConfig
from gateway.platforms.event import MessageEvent, ProcessingOutcome

class SendResult:
    success: bool
    message_id: str | None
    error: str | None
    def __init__(
        self,
        success: bool,
        message_id: str | None = ...,
        error: str | None = ...,
        raw_response: Any = ...,
        retryable: bool = ...,
        retry_after: float | None = ...,
        continuation_message_ids: tuple[str, ...] = ...,
        error_kind: str | None = ...,
    ) -> None: ...

class ExecApprovalPrompt:
    chat_id: str
    session_key: str
    text: str
    actions: list[tuple[str, str, str]]
    command: str
    description: str
    smart_denied: bool
    metadata: dict[str, Any] | None
    def __init__(
        self,
        chat_id: str,
        session_key: str,
        text: str,
        actions: list[tuple[str, str, str]],
        command: str,
        description: str,
        smart_denied: bool,
        metadata: dict[str, Any] | None = ...,
    ) -> None: ...
    @property
    def choices(self) -> list[str]: ...

class BasePlatformAdapter:
    supports_status_text: bool
    splits_long_messages: bool
    gateway_runner: Any
    _session_tasks: dict[str, Any]
    def __init__(self, config: PlatformConfig, platform: Platform) -> None: ...
    @property
    def enforces_own_access_policy(self) -> bool: ...
    @property
    def authorization_is_upstream(self) -> bool: ...
    async def handle_message(self, event: MessageEvent) -> None: ...
    def _event_session_key(self, event: MessageEvent) -> str: ...
    def build_source(
        self,
        chat_id: str,
        chat_name: str | None = ...,
        chat_type: str = ...,
        user_id: str | None = ...,
        user_name: str | None = ...,
        thread_id: str | None = ...,
        chat_topic: str | None = ...,
        user_id_alt: str | None = ...,
        chat_id_alt: str | None = ...,
        is_bot: bool = ...,
        scope_id: str | None = ...,
        guild_id: str | None = ...,
        parent_chat_id: str | None = ...,
        message_id: str | None = ...,
        role_authorized: bool = ...,
        auto_thread_created: bool = ...,
        auto_thread_initial_name: str | None = ...,
    ) -> Any: ...
    def _mark_connected(self, *, listener_base: str | None = ...) -> None: ...
    def _mark_disconnected(self) -> None: ...
    async def connect(self, *, is_reconnect: bool = ...) -> bool: ...
    async def disconnect(self) -> None: ...
    async def send(
        self,
        chat_id: str,
        content: str,
        reply_to: str | None = ...,
        metadata: dict[str, Any] | None = ...,
    ) -> SendResult: ...
    async def edit_message(
        self, chat_id: str, message_id: str, content: str, *, finalize: bool = ...
    ) -> SendResult: ...
    def supports_draft_streaming(
        self,
        chat_type: str | None = ...,
        metadata: dict[str, Any] | None = ...,
        chat_id: str | None = ...,
    ) -> bool: ...
    async def send_draft(
        self, chat_id: str, draft_id: int, content: str, metadata: dict[str, Any] | None = ...
    ) -> SendResult: ...
    def set_status_text(self, chat_id: str, text: str | None) -> None: ...
    def format_tool_event(
        self, event: Any, *, mode: str = ..., preview_max_len: int = ...
    ) -> str | None: ...
    async def send_typing(self, chat_id: str, metadata: Any = ...) -> None: ...
    async def stop_typing(self, chat_id: str) -> None: ...
    async def on_processing_start(self, event: MessageEvent) -> None: ...
    async def on_processing_complete(
        self, event: MessageEvent, outcome: ProcessingOutcome
    ) -> None: ...
    async def get_chat_info(self, chat_id: str) -> dict[str, Any]: ...
    async def send_image(
        self,
        chat_id: str,
        image_url: str,
        caption: str | None = ...,
        reply_to: str | None = ...,
        metadata: dict[str, Any] | None = ...,
    ) -> SendResult: ...
    async def send_document(
        self,
        chat_id: str,
        file_path: str,
        caption: str | None = ...,
        file_name: str | None = ...,
        reply_to: str | None = ...,
        metadata: dict[str, Any] | None = ...,
        **kwargs: Any,
    ) -> SendResult: ...
    async def send_voice(
        self,
        chat_id: str,
        audio_path: str,
        caption: str | None = ...,
        reply_to: str | None = ...,
        metadata: dict[str, Any] | None = ...,
        **kwargs: Any,
    ) -> SendResult: ...
    async def send_video(
        self,
        chat_id: str,
        video_path: str,
        caption: str | None = ...,
        reply_to: str | None = ...,
        metadata: dict[str, Any] | None = ...,
        **kwargs: Any,
    ) -> SendResult: ...
    async def send_clarify(
        self,
        chat_id: str,
        question: str,
        choices: list[Any] | None,
        clarify_id: str,
        session_key: str,
        metadata: dict[str, Any] | None = ...,
    ) -> SendResult: ...
    async def _send_exec_approval_prompt(self, prompt: ExecApprovalPrompt) -> SendResult: ...
    async def send_slash_confirm(
        self,
        chat_id: str,
        title: str,
        message: str,
        session_key: str,
        confirm_id: str,
        metadata: dict[str, Any] | None = ...,
    ) -> SendResult: ...
