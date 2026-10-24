# SPDX-License-Identifier: Apache-2.0
"""How a turn process ended: the recognised CLI texts and the classification (09 18.9, RK-14).

`ERROR_TEXTS` is the one table of texts the runner recognises; each entry names the stable part
of a text of the pinned Claude Code CLI, the channel the CLI writes it to and the evidence.
`classify_turn_end` is a pure function from what a process left behind (`TurnEvidence`) to its
end (`TurnEnd`), in this order (docs/DECISIONS.md D-M2-35-7): a requested stop; a ``result``
with ``subtype`` ``success`` and no ``is_error``; the first table entry whose fragment occurs in
its channel, where ``session_missing`` and ``session_exists`` count only while no
``system/init`` came; otherwise ``runtime_error`` with the error texts of the last ``result``.

stderr is read only to match fragments: its text never reaches a `TurnEnd`, a step, a log line
or a finish body, because it can echo content (design note 7 c).

Examples:
    >>> end = classify_turn_end(TurnEvidence(result={"subtype": "success", "is_error": False}))
    >>> end.outcome
    'completed'
"""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from enum import Enum
from typing import Any

__all__ = [
    "ERROR_TEXTS",
    "ErrorKind",
    "ErrorText",
    "TurnEnd",
    "TurnEvidence",
    "classify_turn_end",
    "result_errors",
]

_PINNED_FILE = "claude (package @anthropic-ai/claude-code-darwin-arm64 2.1.259)"


class ErrorKind(Enum):
    """A recognised failure of a turn process (09 18.9 rows 1 to 4)."""

    CONTEXT_FULL = "context_full"
    """The conversation no longer fits the model's context window."""
    SESSION_MISSING = "session_missing"
    """``--resume <ref>``: Claude Code does not know the session."""
    SESSION_EXISTS = "session_exists"
    """``--session-id <ref>``: the session exists already."""
    NOT_AUTHENTICATED = "not_authenticated"
    """The runtime has no valid login or key."""


@dataclass(frozen=True)
class ErrorText:
    """One recognised text of the pinned CLI.

    Attributes:
        kind: What the text means.
        fragment: The stable part of the text (no IDs, no numbers that vary).
        channel: Where the pinned CLI writes it: ``result`` (the ``result`` text or one of its
            ``errors``), ``assistant`` (the text of the last ``assistant`` message) or
            ``stderr``.
        source: The evidence: a file of the installed pinned CLI or a documentation URL with
            its retrieval date.
    """

    kind: ErrorKind
    fragment: str
    channel: str
    source: str


ERROR_TEXTS: tuple[ErrorText, ...] = (
    ErrorText(
        ErrorKind.CONTEXT_FULL,
        "Prompt is too long",
        "assistant",
        f"{_PINNED_FILE}: the API error text of a request above the context window, written as "
        "the text of an assistant message with isApiErrorMessage (retrieved 2026-10-24)",
    ),
    ErrorText(
        ErrorKind.SESSION_MISSING,
        "No conversation found with session ID",
        "stderr",
        f"{_PINNED_FILE}: print mode with --resume of an unknown session writes it to stderr "
        "(and, with stream-json, as an error result) and exits 1 before system/init "
        "(retrieved 2026-10-24)",
    ),
    ErrorText(
        ErrorKind.SESSION_EXISTS,
        "is already in use",
        "stderr",
        f"{_PINNED_FILE}: 'Error: Session ID <id> is already in use.' for --session-id of a "
        "session that exists, before system/init (retrieved 2026-10-24)",
    ),
    ErrorText(
        ErrorKind.NOT_AUTHENTICATED,
        "Not logged in",
        "assistant",
        f"{_PINNED_FILE}: 'Not logged in · Please run /login' as the API error text of an "
        "assistant message (retrieved 2026-10-24)",
    ),
    ErrorText(
        ErrorKind.NOT_AUTHENTICATED,
        "Invalid API key",
        "assistant",
        f"{_PINNED_FILE}: 'Invalid API key · Fix external API key' as the API error text of "
        "an assistant message (retrieved 2026-10-24)",
    ),
)
"""The recognised texts in match order; the first matching entry decides."""

_BEFORE_INIT_ONLY = frozenset({ErrorKind.SESSION_MISSING, ErrorKind.SESSION_EXISTS})


@dataclass(frozen=True)
class TurnEvidence:
    """What one ended turn process left behind.

    Attributes:
        stop_requested: A stop of the run was requested before or during the process.
        init_seen: The process emitted ``system/init``.
        result: Its last ``result`` message, if any.
        assistant_text: The text items of its last ``assistant`` message, joined.
        stderr: Its stderr file content (only matched, never returned).
        exit_label: The exit code or the name of the ending signal (``Turn.exit_label``).
    """

    stop_requested: bool = False
    init_seen: bool = False
    result: Mapping[str, Any] | None = None
    assistant_text: str = ""
    stderr: str = ""
    exit_label: str = "0"


@dataclass(frozen=True)
class TurnEnd:
    """How a process ended.

    Attributes:
        outcome: ``stopped``, ``completed``, ``runtime_error`` or an `ErrorKind` value.
        errors: For ``runtime_error``: the error texts of the last ``result`` (never stderr).
        exit_label: The exit code or signal name.
    """

    outcome: str
    errors: tuple[str, ...] = field(default=())
    exit_label: str = "0"


def result_errors(result: Mapping[str, Any] | None) -> tuple[str, ...]:
    """Return the error texts of a ``result``: its ``errors`` strings (Agent SDK reference).

    Args:
        result: A ``result`` message or ``None``.

    Returns:
        The texts in order; empty without a ``result`` or without ``errors``.
    """
    if result is None:
        return ()
    errors = result.get("errors")
    if not isinstance(errors, Sequence) or isinstance(errors, str):
        return ()
    return tuple(e for e in errors if isinstance(e, str) and e)


def _channel_text(evidence: TurnEvidence, channel: str) -> str:
    if channel == "stderr":
        return evidence.stderr
    if channel == "assistant":
        return evidence.assistant_text
    result = evidence.result or {}
    text = result.get("result")
    parts = [text] if isinstance(text, str) else []
    return "\n".join([*parts, *result_errors(evidence.result)])


def classify_turn_end(evidence: TurnEvidence) -> TurnEnd:
    """Classify the end of one turn process (design note 7 b).

    Args:
        evidence: What the process left behind.

    Returns:
        The end; ``errors`` only for ``runtime_error``.
    """
    label = evidence.exit_label
    if evidence.stop_requested:
        return TurnEnd("stopped", exit_label=label)
    result = evidence.result
    if result is not None and result.get("subtype") == "success" and result.get("is_error") is not True:
        return TurnEnd("completed", exit_label=label)
    for entry in ERROR_TEXTS:
        if entry.kind in _BEFORE_INIT_ONLY and evidence.init_seen:
            continue
        if entry.fragment in _channel_text(evidence, entry.channel):
            return TurnEnd(entry.kind.value, exit_label=label)
    return TurnEnd("runtime_error", errors=result_errors(result), exit_label=label)
