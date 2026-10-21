# SPDX-License-Identifier: Apache-2.0
"""The two inputs of a turn (docs/spec/09-agenten.md 4.6 and 18.2 "Input", ADR 0054, note 10).

* The system prompt file (``--append-system-prompt-file``) holds only the fixed runner
  instructions of ``prompts/runner_prompt.md`` (the TITLE request only for a new or unstarted
  session) and, in an own section, the channel prompt ``behavior.prompt`` (SEC-AI-1).
* The first stdin line is one ``user`` message with two text items: the system part of
  `bothy_sdk.context.render_context` (context with trust marks, ``unverified`` content and
  every file entry in marked data blocks) and its trigger part (the words and transcripts of
  the triggering person). The context thus becomes part of the session history, so a
  ``--resume`` turn still knows what ``context_floor_seq`` no longer sends.

Examples:
    >>> "TITLE:" in system_prompt(title_request=False, channel_prompt="")
    False
"""

from __future__ import annotations

import json
import re
import uuid
from collections.abc import Sequence
from importlib import resources

from bothy_sdk.context import RenderedContext

__all__ = ["RUNNER_PROMPT", "context_items", "system_prompt", "user_line"]

RUNNER_PROMPT = (
    resources.files("claude_code_runner").joinpath("prompts/runner_prompt.md").read_text("utf-8")
)
"""The fixed runner instructions, with the TITLE request between its markers."""

_TITLE = re.compile(r"\n?<!-- title:start -->\n(.*?)<!-- title:end -->\n?", re.DOTALL)


def system_prompt(*, title_request: bool, channel_prompt: str) -> str:
    """Return the content of the system prompt file of one turn.

    Args:
        title_request: Ask for a ``TITLE:`` line (new or unstarted session).
        channel_prompt: ``behavior.prompt`` of the forum; an own section when not empty.

    Returns:
        The text; it never contains a context element.
    """
    text = _TITLE.sub(
        (lambda m: "\n" + m.group(1)) if title_request else (lambda _m: ""), RUNNER_PROMPT
    )
    text = text.rstrip() + "\n"
    if channel_prompt.strip():
        text += "\n## Channel prompt\n\n" + channel_prompt.strip() + "\n"
    return text


def context_items(rendered: RenderedContext) -> list[str]:
    """Return the two content items of the first stdin line.

    Args:
        rendered: The rendered run context.

    Returns:
        ``[system part, trigger part]``.
    """
    return [rendered.system, rendered.trigger]


def user_line(items: Sequence[str]) -> tuple[str, str]:
    """Build one stdin line of type ``user`` (M2-35 builds follow-up lines with it).

    Args:
        items: The text items in order.

    Returns:
        The JSON line without a line break and its ``uuid`` (to ignore the echo).
    """
    ident = str(uuid.uuid4())
    doc = {
        "type": "user",
        "uuid": ident,
        "message": {"role": "user", "content": [{"type": "text", "text": t} for t in items]},
    }
    return json.dumps(doc, ensure_ascii=False, separators=(",", ":")), ident
