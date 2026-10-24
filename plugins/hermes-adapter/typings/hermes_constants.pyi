# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from contextvars import Token
from pathlib import Path

def get_hermes_home() -> Path: ...
def set_hermes_home_override(path: str | Path | None) -> Token[object]: ...
def reset_hermes_home_override(token: Token[object]) -> None: ...
