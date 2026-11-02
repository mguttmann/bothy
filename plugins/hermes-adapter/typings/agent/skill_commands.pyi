# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

from pathlib import Path
from typing import Any

def get_skill_commands() -> dict[str, dict[str, Any]]: ...
def build_skill_invocation_message(
    cmd_key: str, user_instruction: str = ..., task_id: str | None = ..., runtime_note: str = ...
) -> str | None: ...
def _load_skill_payload(
    skill_identifier: str, task_id: str | None = ...
) -> tuple[dict[str, Any], Path | None, str] | None: ...
def _build_skill_message(
    loaded_skill: dict[str, Any],
    skill_dir: Path | None,
    activation_note: str,
    user_instruction: str = ...,
    runtime_note: str = ...,
    session_id: str | None = ...,
) -> str: ...
