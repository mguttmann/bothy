# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.
# The adapter reads it only on a Hermes before the storage interface (be3d6affc8); the pinned
# version keeps it as a deprecated stand-in with these names.

from pathlib import Path
from typing import Any

class SessionDB:
    def __init__(self, db_path: Path | None = ..., read_only: bool = ...) -> None: ...
    def __enter__(self) -> SessionDB: ...
    def __exit__(self, *exc: object) -> None: ...
    def get_session(self, session_id: str) -> dict[str, Any] | None: ...
