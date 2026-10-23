# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

async def resolve(
    session_key: str, confirm_id: str, choice: str, timeout: float = ...
) -> str | None: ...
