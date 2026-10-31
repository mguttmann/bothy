# SPDX-License-Identifier: Apache-2.0
r"""Follow-ups of people into a running turn (09 7, 18.5; design notes 2 to 5).

A follow-up (``agent.followup``) names its message only by ID (04 2.2 rule 4). The runner reads
the message with ``getMessage``, checks the follow-up's token again at the moment of writing
(`followup_verdict`), writes it as one further ``user`` line with its own ``uuid`` on the
turn's stdin (`build_followup_line`), patches ``delivered``, and patches ``absorbed`` once
Claude Code echoes the line with the same ``uuid`` (``--replay-user-messages``).
`PendingFollowups` keeps what was written and echoed; `stdin_close_due` decides when stdin
closes after a ``result``.

The line never starts with the message text: the lead-in ``Follow-up from <name> (@<handle>):``
comes first, so a text that starts with ``/`` never becomes a command of the CLI.

Examples:
    >>> import json
    >>> line = build_followup_line({"display_name": "Sam", "handle": "sam"}, "/compact", "u-1")
    >>> json.loads(line)["message"]["content"][0]["text"]
    'Follow-up from Sam (@sam):\n\n/compact'
"""

from __future__ import annotations

import json
import uuid
from collections.abc import Collection, Iterator, Mapping, Sequence
from dataclasses import dataclass, field
from enum import Enum
from typing import Any
from uuid import UUID

from bothy_sdk.invocation import Actual, InvocationClaims, TokenVerdict, compare_actual

__all__ = [
    "FollowupCheck",
    "FollowupVerdict",
    "PendingFollowups",
    "WrittenFollowup",
    "build_followup_line",
    "followup_verdict",
    "stdin_close_due",
]

LEAD_IN = "Follow-up from {name} (@{handle}):"
"""The lead-in of a follow-up line, in the language of the runner prompt."""


class FollowupVerdict(Enum):
    """The outcome of the check at the moment of writing (design note 2 c)."""

    WRITE = "write"
    """Every check passes: write the line now."""
    HOLD = "hold"
    """The turn has not started: keep it and write it after the prompt line."""
    SKIP = "skip"
    """A check failed: never written, never acknowledged; the run goes on."""
    DUPLICATE = "duplicate"
    """The message is written already (a redelivery): nothing happens."""


@dataclass(frozen=True)
class FollowupCheck:
    """The inputs of `followup_verdict`.

    Attributes:
        claims: The verified claims of the follow-up's token (signature, ``iss``, ``aud``, the
            payload's run, thread and message passed at acceptance, 09 4.5 points 1 to 4 and 6),
            or ``None`` when the token did not pass there.
        now: The current time in seconds since the epoch.
        leeway: ``adapter.invocation_token_leeway``.
        turn_started: The turn's process is running with its prompt line written.
        run_id: The run of the turn.
        thread_id: The thread of the run.
        billing_mode: The mode the running turn uses.
        tool_class: ``runner.tool_class`` (the class the turn loaded).
        message_id: The follow-up message.
        author_id: The author of the message read with ``getMessage``, ``None`` before the read.
        message_thread_id: The thread of the message read, ``None`` before the read.
        written: The message IDs already written to this run's turn.
    """

    claims: InvocationClaims | None
    now: float
    leeway: float
    turn_started: bool
    run_id: UUID
    thread_id: UUID
    billing_mode: str
    tool_class: str
    message_id: UUID
    author_id: UUID | None = None
    message_thread_id: UUID | None = None
    written: Collection[UUID] = ()


def followup_verdict(check: FollowupCheck) -> tuple[FollowupVerdict, str]:  # noqa: PLR0911 - one answer per check
    """Decide what happens to a follow-up now (design note 2 b and c).

    The token is checked again with the points of 09 4.5 that M2-34 applies before a start:
    expiry with ``adapter.invocation_token_leeway`` (point 5), the actual mode and tool class
    (points 8 and 10); plus the run of the turn, ``trigger_kind`` ``followup``, and the message's
    thread and author ``sub``.

    Args:
        check: The inputs.

    Returns:
        The verdict and a log code (empty for ``write``, ``hold`` and ``duplicate``).
    """
    if check.message_id in check.written:
        return FollowupVerdict.DUPLICATE, ""
    if not check.turn_started:
        return FollowupVerdict.HOLD, ""
    claims = check.claims
    if claims is None:
        return FollowupVerdict.SKIP, "followup_token_invalid"
    if check.now >= claims.exp + check.leeway or claims.iat > check.now + check.leeway:
        return FollowupVerdict.SKIP, "followup_token_expired"
    if claims.run_id != check.run_id or claims.trigger_kind != "followup":
        return FollowupVerdict.SKIP, "followup_check_failed"
    if claims.thread_id != check.thread_id or claims.message_id != check.message_id:
        return FollowupVerdict.SKIP, "followup_check_failed"
    if check.author_id != claims.sub or check.message_thread_id != check.thread_id:
        return FollowupVerdict.SKIP, "followup_check_failed"
    actual = Actual(check.billing_mode, check.tool_class)
    if compare_actual(claims, actual) is not TokenVerdict.VALID:
        return FollowupVerdict.SKIP, "followup_check_failed"
    return FollowupVerdict.WRITE, ""


def build_followup_line(
    author: Mapping[str, Any],
    text: str,
    line_uuid: str | None = None,
    files: Sequence[str] = (),
) -> bytes:
    r"""Build the stdin line of one follow-up (design note 3).

    Args:
        author: The message's author (``display_name``, ``handle``).
        text: ``content_md`` of the message; may be blank when the message has attachments.
        line_uuid: The ``uuid`` of the line; a fresh UUIDv4 when omitted.
        files: One text per attachment (`attachments.followup_file_texts`), listed after the
            text under ``Attachments:`` (FIX-scan-state-stale).

    Returns:
        One JSON line of type ``user`` with its line break.

    Examples:
        >>> import json
        >>> line = build_followup_line({"handle": "sam"}, "", "u-2", ["a.png (image/png): x"])
        >>> json.loads(line)["message"]["content"][0]["text"]
        'Follow-up from sam (@sam):\n\nAttachments:\n- a.png (image/png): x'
    """
    name = str(author.get("display_name") or author.get("handle") or "")
    handle = str(author.get("handle") or "")
    parts = [LEAD_IN.format(name=name, handle=handle)]
    if text.strip() or not files:
        parts.append(text)
    if files:
        parts.append("Attachments:\n" + "\n".join(f"- {f}" for f in files))
    doc = {
        "type": "user",
        "uuid": line_uuid or str(uuid.uuid4()),
        "message": {"role": "user", "content": [{"type": "text", "text": "\n\n".join(parts)}]},
    }
    return json.dumps(doc, ensure_ascii=False, separators=(",", ":")).encode("utf-8") + b"\n"


@dataclass
class WrittenFollowup:
    """One follow-up written to the run's turn.

    Attributes:
        message_id: The follow-up message.
        uuid: The ``uuid`` of its stdin line (kept over restarts, design note 4 d).
        line: The stdin line (kept in memory only, to write it again after a restart).
        delivered: ``delivered`` was patched.
        echoed: Claude Code echoed the line.
        absorbed: ``absorbed`` was patched.
    """

    message_id: UUID
    uuid: str
    line: bytes = b""
    delivered: bool = False
    echoed: bool = False
    absorbed: bool = False


@dataclass
class PendingFollowups:
    """The follow-ups of one run: written, echoed, pending, by ``uuid`` and ``message_id``."""

    by_message: dict[UUID, WrittenFollowup] = field(default_factory=dict)

    def add(self, message_id: UUID, line_uuid: str, line: bytes) -> WrittenFollowup:
        """Record a line about to be written.

        Args:
            message_id: The follow-up message.
            line_uuid: The line's ``uuid``.
            line: The line.

        Returns:
            The record.
        """
        item = WrittenFollowup(message_id, line_uuid, line)
        self.by_message[message_id] = item
        return item

    def __iter__(self) -> Iterator[WrittenFollowup]:
        """Iterate in write order."""
        return iter(list(self.by_message.values()))

    def __contains__(self, message_id: object) -> bool:
        """Whether a message was written already."""
        return message_id in self.by_message

    def pending(self) -> list[WrittenFollowup]:
        """The written follow-ups without echo (in write order)."""
        return [f for f in self.by_message.values() if not f.echoed]

    def delivered(self, message_id: UUID) -> bool:
        """Note that ``delivered`` was patched.

        Args:
            message_id: The follow-up.

        Returns:
            True when its echo came already, so ``absorbed`` is due now.
        """
        item = self.by_message[message_id]
        item.delivered = True
        return item.echoed and not item.absorbed

    def echo(self, line_uuid: object) -> WrittenFollowup | None:
        """Handle an echo with ``isReplay``.

        Args:
            line_uuid: The ``uuid`` of the echoed line.

        Returns:
            The follow-up whose ``absorbed`` is due now: echoed for the first time and
            ``delivered`` patched already; ``None`` for the prompt's echo, an unknown ``uuid``, a
            second echo, or an echo before the ``delivered`` patch (then `delivered` returns it).
        """
        for item in self.by_message.values():
            if item.uuid == line_uuid:
                if item.echoed:
                    return None
                item.echoed = True
                return item if item.delivered else None
        return None

    @classmethod
    def from_record(cls, raw: object) -> PendingFollowups:
        """The follow-up map of a turn record (an adoption; the lines are not stored).

        Args:
            raw: The record's ``followups`` member.

        Returns:
            The follow-ups written by the previous runner, with their echo flags.
        """
        out = cls()
        for item in raw if isinstance(raw, list) else []:
            try:
                message_id = UUID(str(item["message_id"]))
                line_uuid = str(item["uuid"])
            except (KeyError, TypeError, ValueError):
                continue
            written = out.add(message_id, line_uuid, b"")
            written.echoed = bool(item.get("echoed"))
        return out

    def to_record(self) -> list[dict[str, Any]]:
        """The follow-up map of the turn record (no text, design note 12 b)."""
        return [
            {
                "message_id": str(f.message_id),
                "uuid": f.uuid,
                "written": True,
                "echoed": f.echoed,
            }
            for f in self.by_message.values()
        ]


def stdin_close_due(now: float, last_result_at: float | None, pending: int, grace: float) -> bool:
    """Whether stdin of the turn closes now (design note 5).

    Args:
        now: Monotonic seconds.
        last_result_at: When the last ``result`` came, ``None`` before the first.
        pending: Written follow-ups without echo.
        grace: ``runner.followup_grace``.

    Returns:
        True after a ``result`` with nothing pending, or when ``runner.followup_grace`` after
        the last ``result`` has passed.
    """
    if last_result_at is None:
        return False
    return pending == 0 or now - last_result_at >= grace
