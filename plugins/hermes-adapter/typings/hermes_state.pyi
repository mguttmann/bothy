# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from pathlib import Path

class SessionDB:
    def __init__(self, db_path: Path | None = ..., read_only: bool = ...) -> None: ...
    def get_session_title(self, session_id: str) -> str | None: ...
