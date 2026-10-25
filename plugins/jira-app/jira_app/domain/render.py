# SPDX-License-Identifier: Apache-2.0
"""Jira text in the chat (brief M2-50 Design notes 10 and 11; 03 content format, SEC-OUT-1).

Jira text never resolves a mention or becomes a link in the chat: display names, field names
and field values go into a code span, descriptions and comments into a fenced code block, each
delimiter one backtick longer than the longest backtick run of its text (CommonMark 0.31.2), so
the text cannot close it (03 content format rule 2: no mentions in code). A link is written
only to ``<jira_app.jira_base_url>/browse/<KEY>`` for a key that matches the pattern of
`jira_app.jira.events.ISSUE_KEY`, never to a URL of the payload.

Messages are cut to ``limits.message_max_content_bytes`` of ``getInstance`` (UTF-8 bytes at a
code point boundary) by cutting the fenced text or the list of changes and adding the line
`CUT_LINE`, with the fence always closed; titles to ``limits.thread_title_max_chars`` code
points, without control (``Cc``) and bidi characters (SEC-IN-4).

Examples:
    >>> code_span("@alex")
    '`@alex`'
    >>> clean_title(" SUP-12 Login fails ", 50)
    'SUP-12 Login fails'
"""

from __future__ import annotations

import re
import unicodedata
from dataclasses import dataclass

from jira_app.jira.events import ISSUE_KEY, Change, JiraEvent

__all__ = [
    "CUT_LINE",
    "RenderLimits",
    "browse_link",
    "clean_title",
    "code_span",
    "cut_bytes",
    "fence",
    "render_event",
    "thread_title",
    "with_block",
]

CUT_LINE = "(cut, full text in Jira)"
"""The line after a cut text."""

_BIDI = frozenset("\u200e\u200f\u061c\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069")
_LINE_BREAK = re.compile("\r\n|[\n\r\u2028\u2029\u0085]")


@dataclass(frozen=True)
class RenderLimits:
    """The limits of ``getInstance`` the rendering needs.

    Attributes:
        message_max_content_bytes: ``limits.message_max_content_bytes``.
        thread_title_max_chars: ``limits.thread_title_max_chars``.
    """

    message_max_content_bytes: int
    thread_title_max_chars: int


def _longest_run(text: str) -> int:
    longest = run = 0
    for c in text:
        run = run + 1 if c == "`" else 0
        longest = max(longest, run)
    return longest


def code_span(text: str) -> str:
    """Return ``text`` as a CommonMark code span that it cannot close.

    Args:
        text: Foreign text on one line (line breaks become spaces).

    Returns:
        The code span.
    """
    flat = _LINE_BREAK.sub(" ", text)
    ticks = "`" * (_longest_run(flat) + 1)
    # A space on both sides keeps a leading or trailing backtick apart from the delimiter;
    # CommonMark removes exactly one on each side.
    pad = " " if flat[:1] in ("`", " ") or flat[-1:] in ("`", " ") else ""
    return f"{ticks}{pad}{flat}{pad}{ticks}"


def _fence_for(text: str) -> str:
    return "`" * max(3, _longest_run(text) + 1)


def fence(text: str, delimiter: str | None = None) -> str:
    """Return ``text`` as a fenced code block that it cannot close.

    Args:
        text: Foreign text.
        delimiter: The fence; default one backtick longer than the longest run, at least three.

    Returns:
        The block.
    """
    mark = delimiter or _fence_for(text)
    body = text if text.endswith("\n") else text + "\n"
    return f"{mark}\n{body}{mark}"


def utf8_length(text: str) -> int:
    """Return the UTF-8 length of ``text`` in bytes.

    Args:
        text: Any text.

    Returns:
        The byte count.
    """
    return len(text.encode("utf-8"))


def cut_bytes(text: str, max_bytes: int) -> str:
    """Cut ``text`` to at most ``max_bytes`` UTF-8 bytes at a code point boundary.

    Args:
        text: Any text.
        max_bytes: The bound.

    Returns:
        The longest prefix within the bound.
    """
    data = text.encode("utf-8")
    if len(data) <= max_bytes:
        return text
    return data[: max(0, max_bytes)].decode("utf-8", errors="ignore")


def with_block(head: str, text: str, max_bytes: int) -> str:
    """Return ``head``, an empty line and ``text`` fenced, within ``max_bytes``.

    Args:
        head: The first line (already safe Markdown).
        text: Foreign text; empty gives the head alone.
        max_bytes: ``limits.message_max_content_bytes``.

    Returns:
        The message; a cut text keeps its fence closed and is followed by `CUT_LINE`.
    """
    text = text.rstrip("\n")
    if text == "":
        return cut_bytes(head, max_bytes)
    full = f"{head}\n\n{fence(text)}"
    if utf8_length(full) <= max_bytes:
        return full
    mark = _fence_for(text)
    frame = f"{head}\n\n{mark}\n\n{mark}\n{CUT_LINE}"
    room = max_bytes - utf8_length(frame)
    if room <= 0:
        return cut_bytes(head, max_bytes)
    return f"{head}\n\n{fence(cut_bytes(text, room), mark)}\n{CUT_LINE}"


def clean_title(title: str, max_chars: int) -> str:
    """Clean a title: line breaks become spaces, control and bidi characters go, then trim.

    Args:
        title: Foreign text.
        max_chars: ``limits.thread_title_max_chars`` in code points.

    Returns:
        The cleaned title within the bound.
    """
    flat = _LINE_BREAK.sub(" ", title)
    kept = "".join(c for c in flat if c not in _BIDI and unicodedata.category(c) != "Cc")
    return kept.strip()[: max(0, max_chars)].strip()


def thread_title(key: str, summary: str, max_chars: int) -> str:
    """Return the title ``<KEY> <summary>`` of an issue's thread.

    Args:
        key: The issue key.
        summary: The summary (foreign text).
        max_chars: ``limits.thread_title_max_chars``.

    Returns:
        The cleaned and cut title; an empty summary leaves the key alone.
    """
    cleaned = clean_title(summary, max_chars)
    if not cleaned:
        return clean_title(key, max_chars)
    return clean_title(f"{key} {cleaned}", max_chars)


def browse_link(base_url: str, key: str) -> str:
    """Return the key linked to ``<base_url>/browse/<KEY>``, or the key alone.

    Args:
        base_url: ``jira_app.jira_base_url`` (from the configuration, never the payload).
        key: An issue key.

    Returns:
        ``[KEY](<base>/browse/KEY)`` for a key of the pattern, else the key in a code span.
    """
    if ISSUE_KEY.fullmatch(key) is None:
        return code_span(key)
    return f"[{key}]({base_url}/browse/{key})"


def _change_line(change: Change) -> str:
    if change.field == "description":
        return f"{code_span(change.field)} changed"
    old = change.from_text or "(none)"
    new = change.to_text or "(none)"
    return f"{code_span(change.field)}: {code_span(old)} to {code_span(new)}"


def _with_lines(head: str, lines: list[str], max_bytes: int) -> str:
    full = "\n".join([head, *lines])
    if utf8_length(full) <= max_bytes:
        return full
    kept: list[str] = []
    for line in lines:
        if utf8_length("\n".join([head, *kept, line, CUT_LINE])) > max_bytes:
            break
        kept.append(line)
    return cut_bytes("\n".join([head, *kept, CUT_LINE]), max_bytes)


def render_event(event: JiraEvent, base_url: str, limits: RenderLimits) -> str:
    """Return the chat message of a Jira event (English templates of Design note 10).

    Args:
        event: The parsed event.
        base_url: ``jira_app.jira_base_url``.
        limits: The limits of ``getInstance``.

    Returns:
        Markdown within ``limits.message_max_content_bytes``.
    """
    max_bytes = limits.message_max_content_bytes
    ref = browse_link(base_url, event.issue_key)
    who = code_span(event.actor)
    if event.event == "jira:issue_created":
        return with_block(f"**Issue {ref} created** by {who}", event.text, max_bytes)
    if event.event == "jira:issue_updated":
        lines = [_change_line(c) for c in event.changes]
        return _with_lines(f"**{ref} updated** by {who}:", lines, max_bytes)
    return with_block(f"{who} commented on {ref}:", event.text, max_bytes)
