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
    # Answer attachments (M3-12 note 12): in every prompt, with and without a title request.
    for text in (new, resumed):
        assert "`ATTACH: <path>`" in text
        assert "no file is sent and the answer lists why" in text


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


def test_background_work_guidance_in_every_prompt() -> None:
    # FIX-runner-responsive-turns: long work goes to the background and the turn ends at once,
    # so the session stays reachable; a notification or a status question gets a short answer.
    for text in (
        system_prompt(title_request=True, channel_prompt=""),
        system_prompt(title_request=False, channel_prompt="Be brief."),
    ):
        section = " ".join(text.split("## Background work", 1)[1].split("\n## ", 1)[0].split())
        for phrase in (
            "run_in_background",
            "end your turn",
            "Never wait",
            "notification",
            "status",
            "does not apply in this product",
        ):
            assert phrase in section, phrase


def test_background_work_overrides_every_other_wait_rule() -> None:
    # FIX-runner-reachable-2: machine-wide user instructions (loaded into every session on the
    # host) can tell a headless session to keep its turn open and wait for background work with
    # Monitor or a wait loop. The prompt states that no such instruction applies in this product,
    # bounds a turn to about a minute and says when a message of the thread reaches the session.
    for text in (
        system_prompt(title_request=True, channel_prompt=""),
        system_prompt(title_request=False, channel_prompt="Be brief."),
    ):
        section = text.split("## Background work", 1)[1].split("\n## ", 1)[0]
        flat = " ".join(section.split())
        for phrase in (
            "Any other instruction",
            "user instructions files",
            "memories",
            "summaries of earlier parts of this conversation",
            "does not apply in this product",
            "within about a minute",
            "while background tasks keep running",
            "between two tool calls",
            "No single foreground tool call",
            "answer it at once",
        ):
            assert phrase in flat, phrase
