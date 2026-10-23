# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from pathlib import Path
from typing import Any

def load_config() -> dict[str, Any]: ...
def read_user_config_raw(config_path: Path | None = ...) -> dict[str, Any]: ...
def save_config(
    config: dict[str, Any],
    *,
    strip_defaults: bool = ...,
    preserve_keys: set[tuple[str, ...]] | None = ...,
    merge_existing: bool = ...,
) -> None: ...
def save_env_value(key: str, value: str) -> None: ...
def remove_env_value(key: str) -> bool: ...
