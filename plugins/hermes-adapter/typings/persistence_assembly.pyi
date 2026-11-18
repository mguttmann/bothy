# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from pathlib import Path
from typing import Any

class Persistence:
    sessions: Any
    def __enter__(self) -> Persistence: ...
    def __exit__(self, *exc: object) -> None: ...

def profile_scope(profile: str | None = ..., *, home: Path | None = ...) -> Any: ...
def open_persistence(scope: Any, *, eager: bool = ...) -> Persistence: ...
