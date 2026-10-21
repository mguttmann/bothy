# SPDX-License-Identifier: Apache-2.0
"""The shared context golden files of both SDKs (docs/DECISIONS.md D-M2-30-1, ADR 0054).

Every case directory of `sdk/testdata/context/` holds `input.json` (`boundary`, the `RunContext`
body `context` and `file_texts` keyed by `file_id`) and the expected `system.txt` and
`trigger.txt`: the texts `render_context` returns with that boundary, each followed by one line
feed. The TypeScript SDK renders the same cases in `context.test.ts`, so both produce the same
bytes. With `BOTHY_SDK_RECORD_CONTEXT=1` the test writes the expected files instead of comparing.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest

from bothy_sdk.context import render_context
from bothy_sdk.generated.models import RunContext

CASES = Path(__file__).resolve().parents[2] / "testdata" / "context"
RECORD = os.environ.get("BOTHY_SDK_RECORD_CONTEXT") == "1"
# The cases the brief requires (M2-30 "context.test.ts"), each by the case that holds it.
REQUIRED = {
    "author without trigger": "thread_marks",
    "another agent": "thread_marks",
    "import": "thread_marks",
    "file content of the trigger's upload": "trigger_upload_and_voice",
    "transcript of the trigger's voice message": "trigger_upload_and_voice",
}


def case_names() -> list[str]:
    """Every case directory, sorted."""
    return sorted(p.parent.name for p in CASES.glob("*/input.json"))


def load(name: str) -> dict[str, Any]:
    """The input of case `name`."""
    data: dict[str, Any] = json.loads((CASES / name / "input.json").read_text(encoding="utf-8"))
    return data


def render(name: str) -> tuple[str, str]:
    """System and trigger part of case `name` with its fixed boundary."""
    data = load(name)
    rendered = render_context(
        RunContext.from_dict(data["context"]),
        file_texts={UUID(k): v for k, v in data.get("file_texts", {}).items()},
        boundary=data["boundary"],
    )
    return rendered.system, rendered.trigger


def test_required_cases_exist() -> None:
    assert set(REQUIRED.values()) <= set(case_names())


@pytest.mark.parametrize("name", case_names())
def test_golden_pair(name: str) -> None:
    system, trigger = render(name)
    files = {"system.txt": system + "\n", "trigger.txt": trigger + "\n"}
    for file, text in files.items():
        path = CASES / name / file
        if RECORD:
            path.write_text(text, encoding="utf-8", newline="\n")
        assert path.read_text(encoding="utf-8") == text, f"{name}/{file}"


def _messages(context: dict[str, Any]) -> list[dict[str, Any]]:
    return [*context.get("channel_context", []), *context["thread_context"]]


def test_thread_marks_case_holds_its_kinds() -> None:
    """The kinds the brief names are really in the input, with the marks of 09 4.6."""
    context = load("thread_marks")["context"]
    trusts = {(m["author"]["kind"], m["origin"], m["trust"]) for m in _messages(context)}
    assert ("user", "human", "unverified") in trusts
    assert ("agent", "agent", "unverified") in trusts
    assert any(origin == "import" and trust == "unverified" for _, origin, trust in trusts)


def test_upload_case_holds_a_file_text_and_a_voice_transcript() -> None:
    data = load("trigger_upload_and_voice")
    trigger = data["context"]["trigger_message"]
    assert trigger["trust"] == "trigger"
    ids = {a["file_id"] for a in trigger["attachments"]}
    assert ids & set(data["file_texts"])
    assert any("transcript" in a for a in trigger["attachments"])
    system, rendered_trigger = render("trigger_upload_and_voice")
    boundary = data["boundary"]
    assert f"<<<DATA {boundary} source=file" in system
    voice = next(a for a in trigger["attachments"] if "transcript" in a)
    assert voice["transcript"] in rendered_trigger
    assert voice["transcript"] not in system
