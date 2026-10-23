# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

def resolve_gateway_approval(
    session_key: str,
    choice: str,
    resolve_all: bool = ...,
    reason: str | None = ...,
    request_id: str | None = ...,
) -> int: ...
