# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from typing import Any

def resolve_runtime_provider(
    *,
    requested: str | None = ...,
    explicit_api_key: str | None = ...,
    explicit_base_url: str | None = ...,
    target_model: str | None = ...,
) -> dict[str, Any]: ...
