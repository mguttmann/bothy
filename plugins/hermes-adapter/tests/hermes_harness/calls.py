# SPDX-License-Identifier: Apache-2.0
"""Normalization of recorded calls, the call pattern and its comparison.

:func:`normalize` turns raw calls into data that does not depend on the machine, the user or
the run: IDs become ``<id:n>`` in order of first use, the temporary root and the home become
``<tmp>`` and ``<home>``, the host and user names become ``<host>`` and ``<user>`` where they
are a whole string or a whole path segment outside those two, timestamps and durations are
dropped, and every string that is neither a scenario text, an identifier nor a placeholder is
replaced by ``sha256:<hex>:<length>``. Fixtures therefore hold no third-party prose.

:func:`derive_pattern` keeps the calls of the adapter methods (no hooks, attribute reads or
timer calls), shapes their arguments and collapses runs whose repeat count depends on chunk
timing: consecutive ``send_draft`` calls and consecutive non-final edits of one streamed
message. ``set_status_text`` comes from Hermes' agent thread, so its position among the other
calls depends on thread scheduling: the pattern keeps only its last call, as the last entry.
:func:`compare_patterns` names the first difference.

Example:
    >>> pattern = derive_pattern(normalize(result.calls, redactions))
    >>> assert compare_patterns(fixture["pattern"], pattern) == []
"""

from __future__ import annotations

import hashlib
import re
from collections.abc import Iterable, Mapping, Sequence
from dataclasses import dataclass, field
from typing import Any, Final

#: The streaming cursor the pinned version appends to a growing message.
CURSOR: Final = "▉"
RECORD_HINT: Final = "record again: make -C plugins/hermes-adapter record"
#: Argument names whose values are IDs.
ID_FIELDS: Final = frozenset(
    {
        "chat_id",
        "thread_id",
        "message_id",
        "reply_to",
        "reply_to_message_id",
        "session_id",
        "draft_id",
        "clarify_id",
        "confirm_id",
        "tool_call_id",
        "tool_call_ids",
    }
)
#: IDs that can also stand inside a longer text (session IDs, tool call IDs of the fake model).
ID_PATTERNS: Final = (
    re.compile(r"\b[0-9]{8}_[0-9]{6}_[0-9a-f]{8}\b"),
    re.compile(r"\b(?:call|toolu)_[0-9]+_[0-9]+\b"),
)
#: Argument names that hold times or durations; they are dropped.
TIME_FIELDS: Final = frozenset(
    {"created", "timestamp", "started_at", "ended_at", "duration", "duration_ms", "api_duration"}
)
#: Calls that a timer repeats while a turn runs.
TIMER_METHODS: Final = frozenset({"send_typing", "stop_typing"})
#: Methods whose consecutive calls collapse into one entry with their last content.
COLLAPSING_METHODS: Final = frozenset({"send_draft"})
#: Called by Hermes from the agent thread, so its position among the calls of the event loop
#: depends on thread scheduling; the pattern keeps only its last call, as the last entry.
STATUS_METHOD: Final = "set_status_text"
_IDENTIFIER: Final = re.compile(r"[A-Za-z0-9_.:-]{1,64}")
_DIGEST: Final = re.compile(r"sha256:[0-9a-f]{64}:[0-9]+")
_PLACEHOLDER: Final = re.compile(r"<(?:id:[0-9]+|tmp|home|host|user|ws|partial|cursor)>")
_TEXT_ARGS: Final = frozenset({"content", "text", "message", "question", "caption", "title"})


@dataclass(frozen=True)
class Redactions:
    """What normalization replaces.

    Attributes:
        tmp_root: The temporary root of the run (becomes ``<tmp>``).
        home: The real home of the user who runs the tests (becomes ``<home>``).
        host: The host name (becomes ``<host>`` as a whole string or path segment).
        user: The OS user name (becomes ``<user>`` likewise).
        scenario_texts: Texts the scenario injected or expects; kept verbatim.
    """

    tmp_root: str
    home: str
    host: str
    user: str
    scenario_texts: tuple[str, ...] = field(default=())


class _Ids:
    def __init__(self) -> None:
        self.ids: dict[str, str] = {}

    def add(self, value: str) -> None:
        if value and value not in self.ids:
            self.ids[value] = f"<id:{len(self.ids) + 1}>"

    def scan(self, value: Any, key: str | None = None) -> None:
        if isinstance(value, Mapping):
            for k, v in value.items():
                self.scan(v, str(k))
        elif isinstance(value, list):
            for item in value:
                self.scan(item, key)
        elif key in ID_FIELDS and isinstance(value, (str, int)) and not isinstance(value, bool):
            self.add(str(value))
        elif isinstance(value, str):
            for pattern in ID_PATTERNS:
                for match in pattern.finditer(value):
                    self.add(match.group(0))

    def replace(self, text: str) -> str:
        if text in self.ids:
            return self.ids[text]
        for pattern in ID_PATTERNS:
            text = pattern.sub(lambda m: self.ids.get(m.group(0), m.group(0)), text)
        return text


def _redact_paths(text: str, red: Redactions) -> str:
    for prefix, token in ((red.tmp_root, "<tmp>"), (red.home, "<home>")):
        if prefix:
            text = re.sub(re.escape(prefix) + r"(?=/|$|[^A-Za-z0-9_.-])", token, text)
    if text in {red.user, red.host} and text:
        return "<user>" if text == red.user else "<host>"
    if text.startswith("/"):
        segments = text.split("/")
        text = "/".join(
            "<user>" if s == red.user and s else "<host>" if s == red.host and s else s
            for s in segments
        )
    return text


def digest(text: str) -> str:
    """``sha256:<hex>:<length>`` of a text."""
    return f"sha256:{hashlib.sha256(text.encode('utf-8')).hexdigest()}:{len(text)}"


def _classify(text: str, scenario: Sequence[str]) -> str:
    if text in scenario or _PLACEHOLDER.fullmatch(text) or _IDENTIFIER.fullmatch(text):
        return text
    if _DIGEST.fullmatch(text):
        return text
    if text.startswith(("<tmp>", "<home>")) and "\n" not in text and " " not in text:
        return text
    core = text
    lead = "<ws>" if core[:1].isspace() and core.strip() else ""
    cursor = ""
    if core.rstrip().endswith(CURSOR):
        core = core.rstrip()[: -len(CURSOR)]
        cursor = "<cursor>"
    core = core.strip()
    if core in scenario:
        return f"{lead}{core}{cursor}"
    if core and any(t.startswith(core) for t in scenario):
        return f"{lead}<partial>{cursor}"
    return digest(text)


def _normalize_value(value: Any, ids: _Ids, red: Redactions, scenario: Sequence[str]) -> Any:
    if isinstance(value, Mapping):
        return {
            str(k): _normalize_value(v, ids, red, scenario)
            for k, v in value.items()
            if str(k) not in TIME_FIELDS
        }
    if isinstance(value, list):
        return [_normalize_value(v, ids, red, scenario) for v in value]
    if isinstance(value, bool) or value is None or isinstance(value, float):
        return value
    if isinstance(value, int):
        return ids.ids.get(str(value), value)
    if isinstance(value, str):
        return _classify(_redact_paths(ids.replace(value), red), scenario)
    return digest(repr(value))


def normalize(calls: Iterable[Mapping[str, Any]], redactions: Redactions) -> list[dict[str, Any]]:
    """Normalize calls (see the module docstring).

    Args:
        calls: Raw calls ``{"seq", "method", "args"}`` in arrival order.
        redactions: The values to replace.

    Returns:
        ``[{"method", "args"}]`` in the same order, without ``seq``.
    """
    items = list(calls)
    ids = _Ids()
    for call in items:
        ids.scan(call.get("args", {}))
    scenario = tuple(
        _redact_paths(ids.replace(t), redactions) for t in redactions.scenario_texts if t
    )
    return [
        {
            "method": str(call["method"]),
            "args": _normalize_value(call.get("args", {}), ids, redactions, scenario),
        }
        for call in items
    ]


def _shape(value: Any, key: str | None = None) -> Any:
    if isinstance(value, Mapping):
        if set(value) == {"enum", "value"}:
            return dict(value)
        return {str(k): _shape(v, str(k)) for k, v in sorted(value.items())}
    if isinstance(value, list):
        return [_shape(v) for v in value]
    if value is None or isinstance(value, bool):
        return value
    if isinstance(value, str):
        return value
    return type(value).__name__


def _renumber(entries: list[dict[str, Any]]) -> list[dict[str, Any]]:
    mapping: dict[str, str] = {}

    def walk(value: Any) -> Any:
        if isinstance(value, dict):
            return {k: walk(v) for k, v in value.items()}
        if isinstance(value, list):
            return [walk(v) for v in value]
        if isinstance(value, str):
            return re.sub(
                r"<id:[0-9]+>",
                lambda m: mapping.setdefault(m.group(0), f"<id:{len(mapping) + 1}>"),
                value,
            )
        return value

    return [walk(entry) for entry in entries]


def derive_pattern(calls: Sequence[Mapping[str, Any]]) -> list[dict[str, Any]]:
    """The comparable call pattern of normalized calls.

    Args:
        calls: Output of :func:`normalize`.

    Returns:
        Entries ``{"method", "args"}`` with shaped arguments; IDs renumbered within the
        pattern, runs collapsed as the module docstring describes, no-op edits dropped.
    """
    entries: list[dict[str, Any]] = []
    status: dict[str, Any] | None = None
    contents: dict[str, Any] = {}
    for call in calls:
        method = str(call["method"])
        if method in TIMER_METHODS or method.startswith(("hook:", "read:")):
            continue
        args = dict(call.get("args", {}))
        if method == STATUS_METHOD:
            status = {"method": method, "args": _shape(args)}
            continue
        if method == "send" and "message_id" in args:
            contents[str(args["message_id"])] = args.get("content")
        if method == "edit_message":
            message = str(args.get("message_id"))
            if not args.get("finalize") and contents.get(message) == args.get("content"):
                continue  # a no-op edit: the visible message does not change
            contents[message] = args.get("content")
        entry = {"method": method, "args": _shape(args)}
        previous = entries[-1] if entries else None
        if previous is not None and previous["method"] == method:
            same_message = previous["args"].get("message_id") == entry["args"].get("message_id")
            if method in COLLAPSING_METHODS or (
                method == "edit_message"
                and same_message
                and not previous["args"].get("finalize")
                and not entry["args"].get("finalize")
            ):
                entries[-1] = entry
                continue
        entries.append(entry)
    if status is not None:
        entries.append(status)
    return _renumber(entries)


def compare_patterns(
    expected: Sequence[Mapping[str, Any]], actual: Sequence[Mapping[str, Any]]
) -> list[str]:
    """Name the first difference of two patterns.

    Args:
        expected: The pattern of the fixture.
        actual: The pattern of this run.

    Returns:
        An empty list when equal; else the first difference followed by :data:`RECORD_HINT`.
    """
    for index, (want, got) in enumerate(zip(expected, actual, strict=False), start=1):
        if want != got:
            keys = sorted(
                {
                    k
                    for k in {*want.get("args", {}), *got.get("args", {})}
                    if want.get("args", {}).get(k) != got.get("args", {}).get(k)
                }
            )
            detail = (
                f"method {want.get('method')} != {got.get('method')}"
                if (want.get("method") != got.get("method"))
                else f"{want.get('method')} differs in {', '.join(keys)}"
            )
            return [f"entry {index}: {detail}", RECORD_HINT]
    if len(expected) > len(actual):
        return [
            f"entry {len(actual) + 1}: missing {expected[len(actual)].get('method')}",
            RECORD_HINT,
        ]
    if len(actual) > len(expected):
        return [
            f"entry {len(expected) + 1}: unexpected {actual[len(expected)].get('method')}",
            RECORD_HINT,
        ]
    return []
