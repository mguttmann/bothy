# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from typing import Any

class ProcessSession:
    id: str
    command: str
    session_key: str
    started_at: float

class ProcessRegistry:
    def list_sessions(
        self,
        task_id: str | None = ...,
        session_key: str | None = ...,
        *,
        include_retained: bool = ...,
    ) -> list[dict[str, Any]]: ...
    def get(self, session_id: str) -> ProcessSession | None: ...
    def kill_process(
        self, session_id: str, *, source: str = ..., consume_output: bool = ...
    ) -> dict[str, Any]: ...

process_registry: ProcessRegistry
