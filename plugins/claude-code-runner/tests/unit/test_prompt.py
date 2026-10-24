# SPDX-License-Identifier: Apache-2.0
"""The two inputs of a turn (09 4.6, ADR 0054, design note 10)."""

from __future__ import annotations

import json
import uuid

from bothy_sdk.context import RenderedContext

from claude_code_runner.prompt import RUNNER_PROMPT, context_items, system_prompt, user_line


def test_system_file_sections() -> None:
    new = system_prompt(title_request=True, channel_prompt="Answer in short sentences.")
    assert new.startswith(RUNNER_PROMPT.splitlines()[0])
    assert "TITLE:" in new
    assert "## Channel prompt" in new
    assert new.rstrip().endswith("Answer in short sentences.")
    resumed = system_prompt(title_request=False, channel_prompt="")
    assert "TITLE:" not in resumed
    assert "## Channel prompt" not in resumed
    assert "data block" in resumed
    assert "<!--" not in resumed


def test_stdin_line_shape() -> None:
    rendered = RenderedContext(system="SYSTEM PART", trigger="TRIGGER PART")
    line, ident = user_line(context_items(rendered))
    assert "\n" not in line
    doc = json.loads(line)
    assert uuid.UUID(doc["uuid"]).version == 4
    assert doc["uuid"] == ident
    assert doc == {
        "type": "user",
        "uuid": ident,
        "message": {
            "role": "user",
            "content": [
                {"type": "text", "text": "SYSTEM PART"},
                {"type": "text", "text": "TRIGGER PART"},
            ],
        },
    }
