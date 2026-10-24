# SPDX-License-Identifier: Apache-2.0
"""The recovery bookkeeping of one turn process (brief design note 12 f, D-M2-35-28).

A runner may die after any write, and the runner that adopts the turn may die too. The four
recovery invariants hold here and nowhere else; `TurnLedger` is pure (no I/O), its state is part
of the turn record, and `claude_code_runner.runturn.RunTurn` calls it for an own turn and for an
adopted turn alike:

1. **Offset.** The stored offset moves only through `TurnLedger.checkpoint`, after the patches of
   the lines before it went out, and never past the first line of the run's end evidence (the
   last ``result`` read, and an API error message before it) while the run is not finished. The
   next runner reads that ``result`` again.
2. **Booked once.** A ``result`` line gets one answer: its ``client_msg_id`` is planned and stored
   (`TurnLedger.plan_answer`) before the reply goes out, and a line read again reuses it, or
   sends nothing once it is stored as sent (`TurnLedger.answer_sent`). Costs are booked only by
   the finish from the last ``result``.
3. **Inputs by their own id.** An input line counts as answered when a success ``result`` names
   its ``uuid`` or when its echo came before a success ``result`` (a line folded into a running
   turn whose result names nothing); there is no positional rule.
4. **Kind.** The kind of the process is fixed when the ledger is made and kept by an adoption.

Examples:
    >>> ledger = TurnLedger.begin("turn", 0)
    >>> ledger.written("p")
    >>> ledger.line({"type": "user", "isReplay": True, "uuid": "p"}, 0, 10)
    >>> ledger.line({"type": "result", "subtype": "success"}, 10, 30)
    >>> ledger.unanswered(), ledger.checkpoint(30)
    ([], 10)
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, field
from typing import Any
from uuid import UUID

from bothy_sdk.ids import uuid7

from claude_code_runner.failures import api_error_text

__all__ = ["KINDS", "PlannedAnswer", "TurnLedger", "is_success"]

KINDS = ("turn", "compaction")
"""The kinds of a turn process."""


def is_success(result: Mapping[str, Any]) -> bool:
    """Whether a ``result`` ended without an error.

    Args:
        result: A ``result`` message.

    Returns:
        True for subtype ``success`` without ``is_error``.
    """
    return result.get("subtype") == "success" and result.get("is_error") is not True


def _named(result: Mapping[str, Any]) -> set[str]:
    named = result.get("user_message_uuids")
    out = {u for u in named if isinstance(u, str)} if isinstance(named, list) else set()
    last = result.get("user_message_uuid")
    if isinstance(last, str):
        out.add(last)
    return out


@dataclass(frozen=True)
class PlannedAnswer:
    """The answer of one ``result`` line (rule 2).

    Attributes:
        client_msg_id: The reply's ``client_msg_id``, stored before the reply goes out.
        first: The first answer of the run (the one that may carry a title).
        sent: The reply was stored as sent (or refused for good).
    """

    client_msg_id: UUID
    first: bool
    sent: bool = False


@dataclass
class _Input:
    uuid: str
    echoed: bool = False
    answered: bool = False


@dataclass
class TurnLedger:
    """The bookkeeping of one turn process; see the module docstring.

    Attributes:
        kind: ``turn`` or ``compaction`` (rule 4).
        offset: The stored offset of the stream file (rule 1).
        answers: The planned answers by the offset of their ``result`` line (rule 2).
    """

    kind: str
    offset: int
    answers: dict[int, PlannedAnswer] = field(default_factory=dict)
    _inputs: list[_Input] = field(default_factory=list)
    _end_from: int | None = None
    _error_from: int | None = None
    last_result: dict[str, Any] | None = None
    """The last ``result`` read (not stored: it is read again, rule 1)."""
    api_error: str = ""
    """The text of the last API error message read (classification only, never stored)."""
    init_seen: bool = False
    compacted: bool = False

    @classmethod
    def begin(
        cls, kind: str, offset: int, answers: Mapping[int, PlannedAnswer] | None = None
    ) -> TurnLedger:
        """The ledger of a process that starts now.

        Args:
            kind: ``turn`` or ``compaction``.
            offset: Where the process's lines start in the stream file.
            answers: The answers of earlier processes of the run (their lines lie before).

        Returns:
            The ledger.

        Raises:
            ValueError: An unknown kind.
        """
        if kind not in KINDS:
            raise ValueError(kind)
        return cls(kind, offset, dict(answers or {}))

    @classmethod
    def from_record(cls, record: Mapping[str, Any]) -> TurnLedger:
        """The ledger a turn record holds (an adoption).

        Args:
            record: The turn record.

        Returns:
            The ledger, with the record's kind (rule 4); an unknown kind reads as a compaction,
            which is never posted and never completes a run.
        """
        kind = str(record.get("kind", "turn"))
        offset = record.get("offset")
        ledger = cls(
            kind if kind in KINDS else "compaction",
            offset if isinstance(offset, int) and offset >= 0 else 0,
        )
        for raw in record.get("answers") or []:
            try:
                ledger.answers[int(raw["line"])] = PlannedAnswer(
                    UUID(str(raw["id"])), bool(raw.get("first")), bool(raw.get("sent"))
                )
            except (KeyError, TypeError, ValueError):
                continue
        for raw in record.get("inputs") or []:
            if isinstance(raw, Mapping) and isinstance(raw.get("uuid"), str):
                ledger._inputs.append(
                    _Input(raw["uuid"], bool(raw.get("echoed")), bool(raw.get("answered")))
                )
        return ledger

    def to_record(self) -> dict[str, Any]:
        """The ledger's part of the turn record (no text).

        Returns:
            ``kind``, ``offset``, ``answers`` and ``inputs``.
        """
        return {
            "kind": self.kind,
            "offset": self.offset,
            "answers": [
                {"line": line, "id": str(a.client_msg_id), "first": a.first, "sent": a.sent}
                for line, a in sorted(self.answers.items())
            ],
            "inputs": [
                {"uuid": i.uuid, "echoed": i.echoed, "answered": i.answered} for i in self._inputs
            ],
        }

    # ---- rule 3: inputs --------------------------------------------------------------------

    def written(self, line_uuid: str) -> None:
        """An input line was written to the process.

        Args:
            line_uuid: Its ``uuid``.
        """
        if all(i.uuid != line_uuid for i in self._inputs):
            self._inputs.append(_Input(line_uuid))

    def unanswered(self) -> list[str]:
        """The inputs no success ``result`` answered, in write order (a recovery's inputs)."""
        return [i.uuid for i in self._inputs if not i.answered]

    def _result(self, result: Mapping[str, Any]) -> None:
        if not is_success(result):
            return
        named = _named(result)
        for item in self._inputs:
            if item.uuid in named or item.echoed:
                item.answered = True

    # ---- reading ---------------------------------------------------------------------------

    def line(self, message: Mapping[str, Any], start: int, end: int) -> None:
        """Take one message of the stream (its line spans ``start`` to ``end``).

        Args:
            message: The decoded message.
            start: The offset where its line starts.
            end: The offset after its line.
        """
        del end  # the reader's offset is the checkpoint candidate
        kind, subtype = message.get("type"), message.get("subtype")
        if kind == "system" and subtype == "init":
            self.init_seen = True
        elif kind == "system" and subtype == "compact_boundary":
            self.compacted = True
        elif kind == "user" and message.get("isReplay") is True:
            for item in self._inputs:
                if item.uuid == message.get("uuid"):
                    item.echoed = True
        elif kind == "assistant":
            text = api_error_text(message)
            if text is not None:
                self.api_error = text
                if self._error_from is None:
                    self._error_from = start
        elif kind == "result":
            self.last_result = dict(message)
            self._end_from = self._error_from if self._error_from is not None else start
            self._error_from = None
            self._result(message)

    def echoed(self, line_uuid: object) -> bool:
        """Whether an input's echo was read.

        Args:
            line_uuid: The ``uuid``.

        Returns:
            True after its echo.
        """
        return any(i.uuid == line_uuid and i.echoed for i in self._inputs)

    # ---- rule 1: the offset ----------------------------------------------------------------

    def hold(self) -> int | None:
        """The first line the stored offset must not pass yet (the run's end evidence)."""
        if self._end_from is not None:
            return self._end_from
        return self._error_from

    def checkpoint(self, read_to: int) -> int:
        """Move the stored offset after the patches of the lines before ``read_to`` went out.

        Args:
            read_to: The reader's offset (after its last complete line).

        Returns:
            The stored offset: ``read_to`` capped at the end evidence, never moving back.
        """
        hold = self.hold()
        target = read_to if hold is None else min(read_to, hold)
        self.offset = max(self.offset, target)
        return self.offset

    # ---- rule 2: answers -------------------------------------------------------------------

    def planned(self, line: int) -> PlannedAnswer | None:
        """The planned answer of a ``result`` line, if any."""
        return self.answers.get(line)

    def plan_answer(self, line: int, *, may_title: bool) -> PlannedAnswer:
        """Plan the answer of a ``result`` line: a new ``client_msg_id``, stored first.

        Args:
            line: The offset of the ``result`` line.
            may_title: The run asked for a title (only its first answer carries one).

        Returns:
            The planned answer (the existing one for a line planned before).
        """
        existing = self.answers.get(line)
        if existing is not None:
            return existing
        # Lines are read in order and a line read again is planned already: an answer planned
        # now is the run's first one when no other is planned.
        planned = PlannedAnswer(uuid7(), may_title and not self.answers)
        self.answers[line] = planned
        return planned

    def answer_sent(self, line: int) -> None:
        """The reply of a ``result`` line is stored as sent (or refused for good).

        Args:
            line: The offset of the ``result`` line.
        """
        planned = self.answers.get(line)
        if planned is not None and not planned.sent:
            self.answers[line] = PlannedAnswer(planned.client_msg_id, planned.first, sent=True)
