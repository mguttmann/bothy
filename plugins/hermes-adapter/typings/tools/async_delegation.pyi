# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from typing import Any

def list_async_delegations() -> list[dict[str, Any]]: ...
def get_durable_delegation(delegation_id: str) -> dict[str, Any] | None: ...
def interrupt_for_session(
    session_key: str = ...,
    origin_ui_session_id: str = ...,
    parent_session_id: str = ...,
    reason: str = ...,
) -> int: ...
