# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from typing import Any

class GatewayInboundTranscriptionMixin:
    async def _enrich_message_with_transcription(
        self, user_text: str, audio_paths: list[str], clip_memo: dict[str, Any] | None = ...
    ) -> tuple[str, list[str]]: ...
