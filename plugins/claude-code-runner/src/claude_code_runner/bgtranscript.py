# SPDX-License-Identifier: Apache-2.0
"""The transcript of a background session as the stream of a turn (09 18.4 "Background mode").

A background session has no stream for the runner: its live source is the session transcript
``<config dir>/projects/<folder>/<session id>.jsonl``, which the CLI appends while a turn runs,
and the session list. `TranscriptMapper` turns each transcript record into the stream messages
of 18.4 that mean the same, so one code path fills the live box, keeps the ledger and decides
the stop and release of a turn in both modes (`claude_code_runner.processstate.ProcessState`
decides from the same events). `TranscriptReader` tails the file like
`claude_code_runner.streamreader.StreamReader` and adds what only the session list knows.

The mapping, record by record (only records of the main conversation, ``isSidechain`` false):

* The first ``user`` record with a text content and ``origin.kind`` ``human`` after the launch
  offset is the run's prompt: ``system/init`` with the session, then the echo of the runner's
  prompt line. A record before it is skipped when it carries no origin or one of the session
  itself: a resume first writes what closes the interrupted turn of the process before.
* Every ``api-request`` record gives ``system/bg_request`` with ``login``: whether
  ``params.betas`` lists `LOGIN_BETA`, the beta the pinned CLI sends exactly when a request
  uses the stored login (the counter-check of 18.3). The name is compared as a whole.
* ``assistant``: the message with its ``text`` and ``tool_use`` items and its usage; with
  ``isApiErrorMessage`` it carries ``is_api_error_message``, ``error`` and
  ``api_error_params``. The first one that is not synthetic first gives ``system/init`` with
  its model.
* ``user`` with ``origin.kind`` ``peer``: the echo of the inbox line with that ``uuid``
  (18.5), when the runner wrote that line. ``user`` without an origin and with ``tool_result``
  items: those items, reduced to ``tool_use_id`` and ``is_error``. The origin of a ``user``
  record is judged before its content, so only a record without one is ever a tool result.
* ``queue-operation``: ``enqueue`` is ``command_lifecycle`` ``queued``; ``dequeue`` is
  ``started`` and the start of a turn (``system/init``); ``remove`` with ``absorbed_mid_turn``
  is the echo of the runner's line with ``commandUuid``. The records of one queue are matched
  in their order, because ``enqueue`` and ``dequeue`` carry no id.
* ``attachment`` of the type ``queued_command``: what a running turn took in. Only its
  origin and ``source_uuid`` are read; every other attachment is skipped.

**Whose input.** The inbox socket of a session takes a line of every program of the host
user, and the CLI marks it ``peer`` whatever the line says about itself. So the mapper knows
the runner's own lines only by their ``uuid`` (the predicate ``own``, which the runner answers
from the set of the lines it wrote), and each ``uuid`` counts once. An input of anyone else
gives ``system/bg_foreign`` and nothing else: a ``user`` record or a ``queued_command``
attachment with ``origin.kind`` ``peer`` whose ``uuid`` the runner did not write, or wrote and
saw taken before, one with ``origin.kind`` ``human`` that is not the run's prompt (a person
typed into the session), and one with any origin that is not known as the session's own. The
runner then stops the session and posts no answer (18.5). Whose input a record is, is read
from its origin alone and before anything else (`_sender`): no content makes a record of a
sender a step of the turn. An input the session gives itself (`_own_origin`: the notification
of work the session itself launched, a continuation the CLI queues by itself) is none of
these: the CLI removes it from its queue like a line, under a ``uuid`` it made, and names
its origin in the attachment. The kind ``task-notification`` alone does not make an input the
session's own: the CLI records the message of another session of the same user, a relay and
a scheduled prompt under that kind too, so only the producer it stamps for the session's own
work counts. A ``queued_command`` attachment with ``commandMode`` ``task-notification`` and
no origin is the session's own too: the CLI queues the notification of a background agent, a
monitor, a workflow or a timer without one, and every other sender's input carries an origin.
So an input a running turn took under a ``uuid`` the runner did not write stays open until
that attachment attributes it to the session itself. The request that carries the input seals
it, and one still open at the first output after that request, at the following request or at
the end of the turn is foreign too: the CLI writes the attachment right after the request
record (real transcript of the pinned CLI, FIX-RUNNER-BG-NOTICE-ORDER) and may write records
of the answer before the request, which that input cannot have led to. A mapper without
``own`` knows no line of the runner.
* ``system`` ``turn_duration``: the end of the turn, a ``result`` whose text is the text of
  the turn's last assistant message. An API error record ends its turn itself (``result``
  with ``is_error``): the CLI writes no ``turn_duration`` after it since 2.1.326, and one
  that follows it right away ends nothing more.
* ``cost-state``: ``bg_totals`` with ``modelUsage``, the session's totals (18.7).

Every other record type is skipped and counted per type in a debug line. No transcript line is
copied anywhere: a mapped message keeps only the members named above, and the records that hold
account identifiers (``session-owner``, the attachments ``session_context`` and
``credential_org``) are of types this module never reads.

Examples:
    >>> mapper = TranscriptMapper("p-1")
    >>> [m["type"] for m in mapper.map({"type": "user", "isSidechain": False, "sessionId": "s",
    ...     "origin": {"kind": "human"}, "message": {"role": "user", "content": "Fix it"}})]
    ['system', 'user']
"""

from __future__ import annotations

import asyncio
import json
import logging
import time
from collections import Counter, deque
from collections.abc import AsyncIterator, Callable, Mapping
from decimal import Decimal
from pathlib import Path
from typing import Any

from claude_code_runner.bgsession import SessionEntry, SessionList
from claude_code_runner.livecost import PriceTable, base_model, priced_micros
from claude_code_runner.usage import ModelUsage

__all__ = [
    "BACKGROUND_TASK",
    "CAUSE_FOREIGN",
    "CAUSE_REPLACED",
    "CAUSE_WAITING",
    "LOGIN_BETA",
    "READ_FIELDS",
    "PricedUsage",
    "TranscriptMapper",
    "TranscriptReader",
    "foreign_origin",
    "prompt_taken",
    "replayed_records",
]

_LOG = logging.getLogger("claude_code_runner")

READ_FIELDS: dict[str, tuple[str, ...]] = {
    "user": ("isSidechain", "message", "origin", "sessionId", "uuid"),
    "assistant": ("apiErrorParams", "error", "isApiErrorMessage", "isSidechain", "message"),
    "api-request": ("params",),
    "queue-operation": ("commandUuid", "operation", "reason"),
    "attachment": ("attachment", "isSidechain"),
    "attachment/queued_command": ("origin", "source_uuid", "type"),
    "origin/task-notification": ("kind", "producer", "subkind"),
    "system/turn_duration": ("isSidechain", "subtype"),
    "cost-state": ("modelUsage",),
}
"""The members the runner reads per transcript record type, the counterpart of the stream table
of 18.4. ``tests/fixtures/recording/background_<version>/transcript_fields.json`` lists what the
pinned CLI writes; a test holds every name here against it, and the level-one test of the
real-CLI suite compares that file with a fresh run. ``apiErrorParams`` is read in the CLI's
file, not recorded: the stand-in for the API sends no usage window. ``attachment/queued_command``
names the members of the object ``attachment`` of that type, the only attachment read.
``origin/task-notification`` names the members of an origin of that kind, of a ``user`` record
and of that attachment alike (of every other origin only ``kind`` is read); its ``subkind`` is
read in the CLI's file, not recorded: no recorded session got a delivery of another sender."""

BACKGROUND_TASK = "background"
"""The task id the runner reports while the session list says ``busy`` between two turns: the
session works without a turn, and the list names no single task."""

CAUSE_WAITING = "background_session_waiting"
"""The cause of a run whose session waited for a person and was stopped."""

CAUSE_REPLACED = "background_process_replaced"
"""The cause of a run whose session runs in another process than the one the runner launched."""

CAUSE_FOREIGN = "background_foreign_input"
"""The cause of a run whose session took an input the runner did not write (18.5)."""

LOGIN_BETA = "oauth-2025-04-20"
"""The beta of a request that uses the stored login, as the pinned CLI names it. A request
record counts as one on the login only when ``params.betas`` lists exactly this name:
``tests/fixtures/recording/background_<version>/transcript_login.jsonl`` is a request record of
the pinned CLI that carries it, and the level-one test of the real-CLI suite runs it again. A
version that renames the beta fails that test, and until the name here follows, every
``personal`` background turn ends ``billing_mode_mismatch`` instead of running unchecked."""

_SYNTHETIC = "<synthetic>"
_QUEUED = "queued_command"
_WAITING = {"input needed": "a question", "permission prompt": "a permission prompt"}

_PEER = "peer"
_HUMAN = "human"
_SELF = "self"
_OTHER = "other"
_NOTIFICATION = "task-notification"
_OWN_PRODUCER = "session-task"
_CONTINUATION = "auto-continuation"


def _content(record: Mapping[str, Any]) -> object:
    message = record.get("message")
    return message.get("content") if isinstance(message, Mapping) else None


def _own_origin(origin: Mapping[str, Any]) -> bool:
    """Whether an origin is one under which a session gives itself an input.

    The list is closed, and it holds two origins:

    * ``task-notification`` with ``producer`` ``session-task`` and without a ``subkind``: the
      notification of work the session itself launched (a background command, run with the
      pinned CLI; the CLI's file names a background agent, a workflow and a monitor too).
      The kind alone proves nothing. The pinned CLI records under it what other senders
      deliver as well, each either with a ``subkind`` (``peer-send-message``: a message of
      another session of the same user; ``projects-relay``, ``session-inbox``,
      ``scheduled-trigger``) or with no marker at all (a webhook), and it stamps the producer
      only on a notification about the session's own work (read in the file of the pinned
      CLI; the recording holds the producer).
    * ``auto-continuation``: the continuation the CLI queues by itself (read in the file of
      the pinned CLI). That origin has no further member.

    An origin of any other kind or shape, also one a later version adds, is another sender's
    until it stands here.
    """
    kind = origin.get("kind")
    if kind == _CONTINUATION:
        return True
    return (
        kind == _NOTIFICATION
        and origin.get("producer") == _OWN_PRODUCER
        and "subkind" not in origin
    )


def _sender(record: Mapping[str, Any]) -> str | None:
    """Who gave a ``user`` record or a queued input to the session, by its origin alone.

    The one place that reads an origin, and the first thing every reader of such a record
    asks: nothing of the record's content is looked at here, so no content can change whose
    input it is.

    Returns:
        ``None`` for a record without an origin, which is the CLI's own record (a tool
        result, a note): every input of a sender carries one (run with the pinned CLI).
        ``peer`` for a line of the inbox socket, ``human`` for what a person gave, ``self``
        for an input the session gave itself (`_own_origin`), and ``other`` for every other
        origin, also one whose kind cannot be read and a notification that is not about the
        session's own work.
    """
    origin = record.get("origin")
    if origin is None:
        return None
    kind = origin.get("kind") if isinstance(origin, Mapping) else None
    if not isinstance(kind, str):
        return _OTHER
    if kind in (_PEER, _HUMAN):
        return kind
    return _SELF if _own_origin(origin) else _OTHER


def _is_text(content: object) -> bool:
    """Whether a content is what a prompt has: a text, or items none of which is a tool result."""
    if isinstance(content, str):
        return True
    return isinstance(content, list) and not any(
        isinstance(item, Mapping) and item.get("type") == "tool_result" for item in content
    )


def _is_input(record: Mapping[str, Any]) -> bool:
    """A ``user`` record of the main conversation that carries an origin: an input, no step."""
    return (
        record.get("type") == "user"
        and record.get("isSidechain") is not True
        and _sender(record) is not None
    )


class TranscriptMapper:
    """Maps the transcript records of one session process to stream messages of 18.4.

    The mapper holds only what the records before produced, so a runner that adopts a turn
    rebuilds it by mapping the records from the launch offset again (09 18.9).

    Args:
        prompt_uuid: The ``uuid`` of the runner's input line the session was launched with.
        own: Tells whether the runner wrote an input line with this ``uuid`` to the session's
            inbox socket. Without it no line counts as the runner's, so every input of the
            socket is another sender's.

    Attributes:
        turn_open: A turn of the process has started and its ``turn_duration`` is not read.
        skipped: The skipped records, counted per record type.
        started: The run's prompt was read: the session took it.
    """

    def __init__(self, prompt_uuid: str, own: Callable[[str], bool] | None = None) -> None:
        """Start before the prompt of the launch."""
        self._prompt_uuid = prompt_uuid
        self._own = own
        self.started = False
        self.turn_open = False
        self.skipped: Counter[str] = Counter()
        self._session_id = ""
        self._taken: set[str] = set()
        self._attached: set[str] = set()
        self._open: set[str] = set()
        self._sealed: set[str] = set()
        self._model_sent = False
        self._queued: deque[str] = deque()
        self._commands = 0
        self._texts: dict[str, list[str]] = {}
        self._last_message: str | None = None
        self._last_error = False
        self._error_ended = False
        """The last turn ended at its API error record; a ``turn_duration`` that an older CLI
        writes right after it ends nothing more."""
        self._requests: set[str] = set()

    # ---- helpers ---------------------------------------------------------------------------

    def _init(self, **more: Any) -> dict[str, Any]:
        self.turn_open = True
        return {"type": "system", "subtype": "init", **more}

    def _turn_start(self) -> list[dict[str, Any]]:
        return [] if self.turn_open else [self._init()]

    def _command(self) -> str:
        self._commands += 1
        return f"bg-command-{self._commands}"

    @staticmethod
    def _echo(line_uuid: object) -> dict[str, Any]:
        return {"type": "user", "isReplay": True, "uuid": line_uuid}

    @staticmethod
    def _foreign(origin: str) -> dict[str, Any]:
        """An input the runner did not write; nothing of its record but whose it is.

        ``origin`` is one of four fixed words, never a text of the record: ``peer``,
        ``human``, ``other`` (`_sender`) or ``unattributed`` (`_unattributed`).
        """
        return {"type": "system", "subtype": "bg_foreign", "origin": origin}

    def _line(self, line_uuid: object, seen: set[str]) -> bool | None:
        """Whose line a ``uuid`` names, and whether it may still count.

        Returns:
            True for a line of the runner that ``seen`` does not hold yet (it holds it from
            now on); False for a line of the runner seen before: its ``uuid`` counts once, so
            a second input under it is another sender's; ``None`` for any other ``uuid``.
        """
        if not isinstance(line_uuid, str) or self._own is None or not self._own(line_uuid):
            return None
        if line_uuid in seen:
            return False
        seen.add(line_uuid)
        return True

    # ---- the mapping -----------------------------------------------------------------------

    def map(self, record: Mapping[str, Any]) -> list[dict[str, Any]]:  # noqa: PLR0911 - one answer per record type
        """Map one transcript record.

        Args:
            record: A decoded transcript line.

        Returns:
            The stream messages it means, in order; empty for a record the runner skips.
        """
        kind = record.get("type")
        if not isinstance(kind, str):
            self.skipped["untyped"] += 1
            return []
        if record.get("isSidechain") is True:
            self.skipped["sidechain"] += 1
            return []
        if kind == "api-request":
            # The request carries every input the turn took since the last one. The CLI
            # writes the attachments that attribute them right after this record (read in a
            # real transcript of the pinned CLI), so an input open here is sealed and judged
            # at the first output of this request: no output can answer it before
            # (FIX-RUNNER-BG-NOTICE-ORDER).
            out = self._unattributed(self._sealed)
            self._sealed |= self._open
            self._open.clear()
            return [*out, *self._request(record)]
        if kind == "cost-state":
            usage = record.get("modelUsage")
            if not isinstance(usage, Mapping):
                return []
            return [{"type": "bg_totals", "modelUsage": _numbers(usage)}]
        if kind == "attachment":
            return self._attachment(record)
        if not self.started:
            return self._before_prompt(kind, record)
        if kind == "user":
            return self._user(record)
        if kind == "assistant":
            return self._assistant(record)
        if kind == "queue-operation":
            return self._queue(record)
        if kind == "system" and record.get("subtype") == "turn_duration":
            if self._error_ended:
                self._error_ended = False
                return self._unattributed(self._sealed | self._open)
            return self._unattributed(self._sealed | self._open) or [self._turn_end()]
        self.skipped[kind] += 1
        return []

    def _before_prompt(self, kind: str, record: Mapping[str, Any]) -> list[dict[str, Any]]:
        """A record before the run's prompt: skipped, unless it is the prompt or an input.

        The origin decides first. Without one the record is the CLI's own (a resume first
        writes what closes the interrupted turn of the process before), and what the session
        gave itself belongs to that earlier process too: both are skipped. The first text of
        a person is the prompt of the launch. Every other record with an origin is an input
        of another sender: the runner writes no line before the session took its prompt, and
        the prompt of a launch is a text, whatever else a person's record holds.
        """
        sender = _sender(record) if kind == "user" else None
        if sender is None or sender == _SELF:
            self.skipped[kind] += 1
            return []
        if sender != _HUMAN or not _is_text(_content(record)):
            return [self._foreign(sender)]
        self.started = True
        # The prompt came by the launch, not the socket: no line may carry its uuid.
        self._taken.add(self._prompt_uuid)
        self._attached.add(self._prompt_uuid)
        session = record.get("sessionId")
        self._session_id = session if isinstance(session, str) else ""
        return [self._init(session_id=self._session_id), self._echo(self._prompt_uuid)]

    def _request(self, record: Mapping[str, Any]) -> list[dict[str, Any]]:
        """Every request record: whether it lists the login beta (the whole name, 18.3)."""
        params = record.get("params")
        betas = params.get("betas") if isinstance(params, Mapping) else None
        names = betas if isinstance(betas, list) else []
        return [{"type": "system", "subtype": "bg_request", "login": LOGIN_BETA in names}]

    def _attachment(self, record: Mapping[str, Any]) -> list[dict[str, Any]]:
        """What a running turn took in: only whose input it is (18.5), never its text."""
        inner = record.get("attachment")
        if not isinstance(inner, Mapping) or inner.get("type") != _QUEUED:
            self.skipped["attachment"] += 1
            return []
        sender = _sender(inner)
        if sender is None and inner.get("commandMode") == _NOTIFICATION:
            # The CLI queues the notification of the session's own work without an origin
            # too (a background agent, a monitor, a workflow, a timer: read in the file of
            # the pinned CLI, the agent's in a real transcript); every other sender's input
            # carries one (FIX-RUNNER-BG-OWN-NOTICE).
            sender = _SELF
        if sender is None:
            return []  # it attributes nothing: an open input stays open
        self._open.discard(str(inner.get("source_uuid")))
        self._sealed.discard(str(inner.get("source_uuid")))
        if not self.started:
            # Before the prompt the runner has written no line, so a line is never its own,
            # and an origin that is neither a person nor the session itself is another
            # sender's, as for a ``user`` record there. What a person or the session queued
            # in an earlier process is skipped with its records.
            settled = sender in (_HUMAN, _SELF)
        elif sender == _PEER:
            # A line of the runner shows once here, beside its ``remove`` record.
            settled = self._line(inner.get("source_uuid"), self._attached) is True
        else:
            # The CLI attributes the input to the session itself (the notification of work
            # the session launched): no line of the socket and no person. Every other
            # origin is another sender's, also a notification without that producer.
            settled = sender == _SELF
        return [] if settled else [self._foreign(sender)]

    def _unattributed(self, inputs: set[str]) -> list[dict[str, Any]]:
        """An input a turn took that nothing attributed before an output could answer it."""
        if not inputs:
            return []
        self._open.clear()
        self._sealed.clear()
        return [self._foreign("unattributed")]

    def _user(self, record: Mapping[str, Any]) -> list[dict[str, Any]]:
        """A ``user`` record after the prompt: whose input it is first, then what it holds.

        The origin is judged before the content, so a record of a sender never passes as a
        step of the turn because its content is shaped like one (18.5). Only a record without
        an origin is read as tool results.
        """
        sender = _sender(record)
        if sender == _PEER:
            # A line of the inbox socket that starts a turn: its echo, when it is the
            # runner's and its first (18.5).
            if self._line(record.get("uuid"), self._taken):
                return [*self._turn_start(), self._echo(record.get("uuid"))]
            return [self._foreign(_PEER)]
        if sender == _SELF:
            return self._turn_start()
        if sender is not None:
            # The run's prompt came before: a person typed this into the session, or it
            # came from an origin that is not known as the session's own.
            return [self._foreign(sender)]
        content = _content(record)
        results = [
            {
                "type": "tool_result",
                "tool_use_id": item.get("tool_use_id"),
                "is_error": item.get("is_error") is True,
            }
            for item in (content if isinstance(content, list) else [])
            if isinstance(item, Mapping) and item.get("type") == "tool_result"
        ]
        if results:
            return [{"type": "user", "message": {"role": "user", "content": results}}]
        self.skipped["user"] += 1
        return []

    def _assistant(self, record: Mapping[str, Any]) -> list[dict[str, Any]]:
        message = record.get("message")
        if not isinstance(message, Mapping):
            self.skipped["assistant"] += 1
            return []
        out = [*self._unattributed(self._sealed), *self._turn_start()]
        model = message.get("model")
        error = record.get("isApiErrorMessage") is True
        items: list[dict[str, Any]] = []
        texts: list[str] = []
        content = message.get("content")
        for item in content if isinstance(content, list) else []:
            if not isinstance(item, Mapping):
                continue
            if item.get("type") == "text" and isinstance(item.get("text"), str):
                items.append({"type": "text", "text": item["text"]})
                texts.append(item["text"])
            elif item.get("type") == "tool_use":
                items.append(
                    {
                        "type": "tool_use",
                        "id": item.get("id"),
                        "name": item.get("name"),
                        "input": item.get("input"),
                    }
                )
        mapped: dict[str, Any] = {
            "type": "assistant",
            "session_id": self._session_id,
            "parent_tool_use_id": None,
            "message": {
                "id": message.get("id"),
                "model": model,
                "role": "assistant",
                "content": items,
                "usage": message.get("usage"),
            },
        }
        if error:
            mapped["is_api_error_message"] = True
            mapped["error"] = record.get("error")
            if isinstance(record.get("apiErrorParams"), Mapping):
                mapped["api_error_params"] = dict(record["apiErrorParams"])
        elif isinstance(model, str) and model != _SYNTHETIC and not self._model_sent:
            self._model_sent = True
            out.append(self._init(model=model))
        ident = message.get("id")
        key = ident if isinstance(ident, str) and ident else f"record-{len(self._texts)}"
        self._texts.setdefault(key, []).extend(texts)
        self._last_message = key
        self._last_error = error
        if not error and model != _SYNTHETIC:
            self._requests.add(key)
        out.append(mapped)
        self._error_ended = error
        if error:
            # 2.1.326 writes no ``turn_duration`` after an API error: the error record is the
            # last record of its turn (FIX-RUNNER-CLI-PIN-326).
            out.extend(self._unattributed(self._sealed | self._open) or [self._turn_end()])
        return out

    def _queue(self, record: Mapping[str, Any]) -> list[dict[str, Any]]:
        operation = record.get("operation")
        if operation == "enqueue":
            command = self._command()
            self._queued.append(command)
            return [{"type": "command_lifecycle", "command_uuid": command, "state": "queued"}]
        command = self._queued.popleft() if self._queued else self._command()
        if operation == "dequeue":
            # The queued input starts a turn of its own (a line of the inbox, a notification).
            started = {"type": "command_lifecycle", "command_uuid": command, "state": "started"}
            return [started, self._init()]
        out: list[dict[str, Any]] = [
            {"type": "command_lifecycle", "command_uuid": command, "state": "completed"}
        ]
        line_uuid = record.get("commandUuid")
        if record.get("reason") == "absorbed_mid_turn" and isinstance(line_uuid, str):
            mine = self._line(line_uuid, self._taken)
            if mine:
                out.append(self._echo(line_uuid))
            elif mine is False:
                out.append(self._foreign("peer"))
            else:
                # An input of the session itself (the notification of a task) or of another
                # sender: the ``queued_command`` attachment says which, and until it does
                # the input is nobody's.
                self._open.add(line_uuid)
        return out

    def _turn_end(self) -> dict[str, Any]:
        texts = self._texts.get(self._last_message or "", [])
        result = {
            "type": "result",
            "subtype": "success",
            "is_error": self._last_error,
            "session_id": self._session_id,
            "result": "" if self._last_error else "\n".join(texts),
            "num_turns": len(self._requests),
        }
        self.turn_open = False
        self._texts = {}
        self._last_message = None
        self._last_error = False
        self._requests = set()
        return result


def _numbers(usage: Mapping[str, Any]) -> dict[str, Any]:
    """``modelUsage`` reduced to model names and numbers (never a text)."""
    out: dict[str, Any] = {}
    for model, values in usage.items():
        if isinstance(model, str) and isinstance(values, Mapping):
            out[model] = {
                k: v
                for k, v in values.items()
                if isinstance(k, str) and isinstance(v, int | float) and not isinstance(v, bool)
            }
    return out


def _decode(raw: bytes) -> dict[str, Any] | None:
    if not raw.strip():
        return None
    try:
        record = json.loads(raw)
    except ValueError:
        return None
    return record if isinstance(record, dict) else None


def prompt_taken(path: Path | None, offset: int) -> bool:
    """Whether a transcript shows a prompt after ``offset``: a session took the launch's prompt.

    Only the origin is judged, as in the mapper: a session that took any input of a person
    after the offset has started, whatever the content of that record is, so its prompt is
    never given to a print turn as well. Whether that input is the prompt is the mapper's
    question (`TranscriptMapper`).

    Args:
        path: The session transcript; ``None`` when the session has none.
        offset: The launch offset.

    Returns:
        True when a ``user`` record of the main conversation with ``origin.kind`` ``human``
        follows the offset.
    """
    if path is None:
        return False
    try:
        with path.open("rb") as fh:
            fh.seek(offset)
            data = fh.read()
    except OSError:
        return False
    for raw in data.split(b"\n"):
        record = _decode(raw)
        if record is not None and _is_input(record) and _sender(record) == _HUMAN:
            return True
    return False


def replayed_records(
    path: Path | None, start: int, end: int, mapper: TranscriptMapper
) -> list[tuple[dict[str, Any], int, int]]:
    """Map the transcript records whose lines start in ``[start, end)`` (an adoption, 18.9).

    Args:
        path: The session transcript.
        start: The launch offset (``process_start`` of the ledger).
        end: The record's stored offset.
        mapper: A new mapper; afterwards it stands where the previous runner's stood.

    Returns:
        Each mapped message with the offsets where its transcript line starts and ends.
    """
    if path is None or end <= start:
        return []
    try:
        with path.open("rb") as fh:
            fh.seek(start)
            data = fh.read(end - start)
    except OSError:
        return []
    out: list[tuple[dict[str, Any], int, int]] = []
    at = start
    for raw in data.split(b"\n")[:-1]:
        line_start, at = at, at + len(raw) + 1
        record = _decode(raw)
        if record is None:
            continue
        out.extend((message, line_start, at) for message in mapper.map(record))
    return out


def foreign_origin(
    path: Path | None, start: int, prompt_uuid: str, own: Callable[[str], bool] | None
) -> str | None:
    """Whose input a session process took that was not the runner's, if it took one (18.5).

    The whole transcript of the process, from its launch offset to the end of the file,
    mapped as `TranscriptMapper` maps it for the runner that wrote the lines ``own``. Asked
    when a session is released and no reader read its transcript to the end (an error or a
    cancellation ends the run, an adopter does not follow the turn): what the session took
    in the part nobody read stays in its conversation all the same.

    Args:
        path: The session transcript; ``None`` when the session has none.
        start: The launch offset of the process.
        prompt_uuid: The ``uuid`` of the input the process was launched with.
        own: Tells whether the runner wrote an input line with this ``uuid``.

    Returns:
        The word of the first foreign input (`TranscriptMapper`), or ``None`` without one.
    """
    if path is None:
        return None
    try:
        end = path.stat().st_size
    except OSError:
        return None
    mapper = TranscriptMapper(prompt_uuid, own)
    for message, _, _ in replayed_records(path, start, end, mapper):
        if message.get("type") == "system" and message.get("subtype") == "bg_foreign":
            return str(message.get("origin"))
    return None


class PricedUsage:
    """The usage of a run's background turns, priced from its ``assistant`` messages (18.7).

    The session's totals come from the ``cost-state`` record the CLI writes when a session
    process ends. A process that was killed hard writes none; the run then books what its
    messages reported, priced with the start prices like the cost display.
    """

    def __init__(self) -> None:
        """Start without a message."""
        self._messages: dict[str, tuple[str, Mapping[str, Any]]] = {}

    def add(self, message: Mapping[str, Any]) -> None:
        """Take one mapped ``assistant`` message (a message counts once, with its newest usage).

        Args:
            message: The stream message.
        """
        inner = message.get("message")
        if not isinstance(inner, Mapping) or message.get("is_api_error_message") is True:
            return
        ident, model, usage = inner.get("id"), inner.get("model"), inner.get("usage")
        if isinstance(ident, str) and isinstance(model, str) and isinstance(usage, Mapping):
            self._messages[ident] = (model, usage)

    def model_usage(self, prices: PriceTable) -> dict[str, ModelUsage]:
        """Return the summed usage per model with its priced amount.

        Args:
            prices: The price table.

        Returns:
            The values per model name of the price table (empty without a message).
        """
        sums: dict[str, list[int]] = {}
        for model, usage in self._messages.values():
            name = base_model(model)
            row = sums.setdefault(name, [0, 0, 0, 0, 0])
            for index, member in enumerate(
                (
                    "input_tokens",
                    "output_tokens",
                    "cache_read_input_tokens",
                    "cache_creation_input_tokens",
                )
            ):
                value = usage.get(member)
                if isinstance(value, int) and not isinstance(value, bool) and value > 0:
                    row[index] += value
            row[4] += priced_micros(prices, name, usage)
        return {
            name: ModelUsage(row[0], row[1], row[2], row[3], Decimal(row[4]) / Decimal(1_000_000))
            for name, row in sums.items()
        }


class TranscriptReader:
    """Reads the mapped messages of one session process: its transcript and the session list.

    The transcript is read every ``runner.poll_interval`` from an offset, like the stream file
    of a print turn. The session list adds what no record says:

    * After a turn end the reader waits for a reading of the list taken after it. ``busy``
      then means that the session works without a turn (background work), unless the
      transcript already shows the start of the next turn: the reader reports one running
      task (``system/background_tasks_changed``) before the ``result``, and an empty list
      once a later reading says ``idle``.
    * While a turn runs and the list says ``busy``, each reading gives one ``tool_progress``,
      so a long tool call is not reported stalled (18.4).
    * A session that waits for a person (``waitingFor``), and a session whose process is not
      the one the runner launched, give an error ``result`` with the cause; the runner then
      stops the session and the run ends ``runtime_error`` (18.9).

    Args:
        locate: Returns the transcript once it exists (the CLI creates it after the launch).
        poll_interval: ``runner.poll_interval``.
        alive: Tells whether the session process still lives.
        mapper: The mapper of the process.
        sessions: The reader of the session list.
        short_id: The short id of the session.
        pid: The process the runner drives.
        offset: Where to start in the transcript (the launch offset, or a stored offset).
        clock: Monotonic seconds (tests).
    """

    def __init__(  # noqa: PLR0913 - every input of the reader is explicit
        self,
        locate: Callable[[], Path | None],
        poll_interval: float,
        alive: Callable[[], bool],
        mapper: TranscriptMapper,
        *,
        sessions: SessionList,
        short_id: str,
        pid: int,
        offset: int = 0,
        clock: Callable[[], float] = time.monotonic,
    ) -> None:
        """Create the reader; nothing is read before `items`."""
        self._locate = locate
        self.poll_interval = poll_interval
        self.alive = alive
        self.mapper = mapper
        self.sessions = sessions
        self.short_id = short_id
        self.pid = pid
        self.offset = offset
        """The offset after the last complete transcript line mapped so far."""
        self.line_offset = offset
        """The offset where the line of the message yielded last starts."""
        self._clock = clock
        self._path: Path | None = None
        self._read_to = offset
        self._read_at = 0.0
        self._partial = b""
        self._pending: deque[bytes] = deque()
        self._working = False
        self._failed = False
        self._status_at = 0.0
        self.status: str | None = None
        """``status`` of the session in the newest reading: ``busy``, ``idle`` or ``waiting``."""

    def _pull(self) -> int:
        """Read what the file gained and queue its complete lines; the count of bytes read."""
        self._read_at = self._clock()
        if self._path is None:
            self._path = self._locate()
        if self._path is None:
            return 0
        try:
            with self._path.open("rb") as fh:
                fh.seek(self._read_to)
                chunk = fh.read()
        except FileNotFoundError:
            return 0
        self._read_to += len(chunk)
        *lines, self._partial = (self._partial + chunk).split(b"\n")
        self._pending.extend(lines)
        return len(chunk)

    async def items(self) -> AsyncIterator[dict[str, Any] | None]:
        """Yield every mapped message in order, and ``None`` after a poll that found nothing.

        The reader ends when the session process is gone and the transcript is read to its
        end, so the ``cost-state`` record a process writes at its end is read too.

        Yields:
            One stream message of 18.4, or ``None`` for an idle poll.
        """
        try:
            while True:
                running = self.alive()
                read = self._pull()
                while self._pending:
                    raw = self._pending.popleft()
                    self.line_offset = self.offset
                    self.offset += len(raw) + 1
                    record = _decode(raw)
                    if record is None:
                        continue
                    for message in self.mapper.map(record):
                        if message["type"] == "result":
                            for extra in await self._after_turn():
                                yield extra
                        yield message
                if running:
                    for message in await self._between_polls():
                        yield message
                if not read and not running:
                    return
                if not read:
                    yield None
                    await asyncio.sleep(self.poll_interval)
        finally:
            if self.mapper.skipped:
                counts = dict(self.mapper.skipped)
                _LOG.debug(json.dumps({"event": "transcript_skipped", "counts": counts}))

    # ---- the session list ------------------------------------------------------------------

    def _entry(self, entry: SessionEntry | None) -> list[dict[str, Any]]:
        """What an entry of the list says beyond its status: a failure of the run, once."""
        if entry is None or self._failed:
            return []
        if entry.waiting_for is not None:
            what = _WAITING.get(entry.waiting_for, "a person")
            text = f"The background session waited for {what} and was stopped."
            return [self._failure(CAUSE_WAITING, text)]
        if entry.pid is not None and entry.pid != self.pid:
            text = "The background session runs in another process than the runner started."
            return [self._failure(CAUSE_REPLACED, text)]
        return []

    def _failure(self, cause: str, text: str) -> dict[str, Any]:
        self._failed = True
        self.mapper.turn_open = False
        return {
            "type": "result",
            "subtype": "error_during_execution",
            "is_error": True,
            "errors": [text],
            "bg_cause": cause,
        }

    def _tasks(self, working: bool) -> list[dict[str, Any]]:
        if working == self._working:
            return []
        self._working = working
        tasks = [{"task_id": BACKGROUND_TASK}] if working else []
        return [{"type": "system", "subtype": "background_tasks_changed", "tasks": tasks}]

    def _turn_follows(self) -> bool:
        """Whether the lines read and not yet mapped already start the next turn.

        Asked of the records the mapper reads: an input of the main conversation, by its
        origin alone, whoever gave it (the mapper then says whose it is).
        """
        for raw in self._pending:
            record = _decode(raw)
            if record is None:
                continue
            if record.get("type") == "queue-operation" and record.get("operation") == "dequeue":
                return True
            if _is_input(record):
                return True
        return False

    async def _after_turn(self) -> list[dict[str, Any]]:
        """The reading taken after a turn end, before its ``result`` is given on."""
        reading = await self.sessions.read(after=self._clock())
        self._status_at = self._clock()
        entry = reading.entry(self.short_id) if reading is not None else None
        if entry is None:
            return []
        self.status = entry.status
        failure = self._entry(entry)
        if failure:
            return failure
        if entry.status != "busy":
            return self._tasks(False)
        # Busy after a turn end is background work, unless the next turn has started already:
        # a queued line or a notification makes the session busy again at once.
        self._pull()
        return [] if self._turn_follows() else self._tasks(True)

    async def _between_polls(self) -> list[dict[str, Any]]:
        """One shared reading per ``runner.background_status_interval`` while the turn lives."""
        if self._clock() - self._status_at < self.sessions.interval:
            return []
        reading = await self.sessions.read()
        entry = reading.entry(self.short_id) if reading is not None else None
        if reading is None or entry is None:
            self._status_at = self._clock()
            return []
        busy = entry.status == "busy"
        if busy and not self.mapper.turn_open and reading.at > self._read_at:
            # The session may have started a turn that the transcript, read before this
            # reading, does not show yet: decide with the next read of the file.
            return self._entry(entry)
        self._status_at = self._clock()
        self.status = entry.status
        return self._entry(entry) or self._status_messages(busy)

    def _status_messages(self, busy: bool) -> list[dict[str, Any]]:
        if self.mapper.turn_open:
            return [{"type": "tool_progress"}] if busy else []
        return self._tasks(busy) if self.mapper.started else []
