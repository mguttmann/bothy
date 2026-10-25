# SPDX-License-Identifier: Apache-2.0
"""Jira text in the chat (brief M2-50 Design note 11, "Tests" bullet ``test_render.py``).

Proves that mentions and links of Jira text end up only inside code, that a fence or code span
cannot be closed by the text, that titles are cleaned and cut to code points, that messages are
cut to ``limits.message_max_content_bytes`` at a code point boundary with the fence closed and
the cut line, that a link is built only from the configuration and a valid key, and that the
server's ``400 validation_failed`` answers lead to exactly the retries of Design note 11.
"""

from __future__ import annotations

import re
from collections.abc import AsyncIterator
from pathlib import Path

import pytest

from jira_app.domain.render import (
    CUT_LINE,
    RenderLimits,
    browse_link,
    code_span,
    cut_bytes,
    fence,
    render_event,
    thread_title,
    with_block,
)
from jira_app.jira.events import Change, JiraEvent, parse_event
from tests.fakes.chat import problem
from tests.fakes.jira_api import render_fixture
from tests.helpers import World, start_world

BASE = "https://jira.example.com"
LINK = "https://jira.example.com/browse/SUP-12"
WIDE = RenderLimits(message_max_content_bytes=262144, thread_title_max_chars=200)
HOSTILE = "@alex see [x](https://example.com) and <https://example.com/raw>"
_URL = re.compile(r"https?://[^\s)\]>`]+")


def _outside_code(markdown: str) -> str:
    """Return the parts of ``markdown`` that are not inside a fenced block or code span."""
    out: list[str] = []
    lines = markdown.split("\n")
    i = 0
    prose: list[str] = []
    while i < len(lines):
        line = lines[i]
        if re.fullmatch(r"`{3,}", line):
            # A fenced block: skip to the closing fence of at least the same length.
            closing = i + 1
            while closing < len(lines) and not (
                re.fullmatch(r"`+", lines[closing]) and len(lines[closing]) >= len(line)
            ):
                closing += 1
            assert closing < len(lines), "fence not closed"
            i = closing + 1
            prose.append("")
            continue
        prose.append(line)
        i += 1
    for text in prose:
        pos = 0
        while pos < len(text):
            run = re.compile(r"`+").search(text, pos)
            if run is None:
                out.append(text[pos:])
                break
            out.append(text[pos : run.start()])
            ticks = run.group()
            end = re.compile(rf"(?<!`){ticks}(?!`)").search(text, run.end())
            assert end is not None, f"code span not closed in {text!r}"
            pos = end.end()
        out.append("\n")
    return "".join(out)


def _created(text: str, *, actor: str = "Sam", summary: str = "Login fails") -> JiraEvent:
    return JiraEvent("jira:issue_created", "10012", "SUP-12", "SUP", summary, actor, text=text)


# -- foreign text only inside code -------------------------------------------------------


def test_mention_and_link_of_a_description_stay_in_the_fence() -> None:
    message = render_event(_created(HOSTILE, actor="@alex"), BASE, WIDE)
    prose = _outside_code(message)
    assert "@alex" in message
    assert "@alex" not in prose
    assert "https://example.com" not in prose
    assert "](https://example.com)" not in prose
    assert "`@alex`" in message
    assert f"```\n{HOSTILE}\n```" in message


def test_mention_and_link_of_a_comment_stay_in_the_fence() -> None:
    event = JiraEvent(
        "comment_created", "10012", "SUP-12", "SUP", "Login fails", "@alex", HOSTILE, (), "10050"
    )
    message = render_event(event, BASE, WIDE)
    assert message.startswith(f"`@alex` commented on [SUP-12]({LINK}):")
    prose = _outside_code(message)
    assert "@alex" not in prose
    assert "example.com)" not in prose.replace(f"({LINK})", "")


def test_field_values_of_an_update_stay_in_code_spans() -> None:
    event = JiraEvent(
        "jira:issue_updated",
        "10012",
        "SUP-12",
        "SUP",
        "Login fails",
        "Sam",
        changes=(Change("@alex", "[x](https://example.com)", "@alex"),),
    )
    message = render_event(event, BASE, WIDE)
    prose = _outside_code(message)
    assert "@alex" not in prose
    assert "https://example.com" not in prose
    assert message.endswith("`@alex`: `[x](https://example.com)` to `@alex`")


# -- delimiters the text cannot close --------------------------------------------------------


def test_description_with_backtick_runs_keeps_its_fence_closed() -> None:
    text = "start\n```\nnot a fence end\n````\n@alex ``` [x](https://example.com)\nend"
    message = render_event(_created(text), BASE, WIDE)
    lines = message.split("\n")
    opening = lines.index("`````")
    assert lines[-1] == "`````"
    assert "\n".join(lines[opening + 1 : -1]) == text
    prose = _outside_code(message)
    assert "@alex" not in prose
    assert "not a fence end" not in prose


def test_fence_and_code_span_are_one_longer_than_the_longest_run() -> None:
    assert fence("plain") == "```\nplain\n```"
    assert fence("a ```` b") == "`````\na ```` b\n`````"
    assert code_span("@alex") == "`@alex`"
    assert code_span("a``b") == "```a``b```"
    assert code_span("`x`") == "`` `x` ``"
    assert code_span("line one\nline two") == "`line one line two`"


# -- titles ---------------------------------------------------------------------------------


def test_summary_with_bidi_control_and_line_break_is_cleaned() -> None:
    summary = "Login\u202e fails\x07\nagain\u2066 \u200f"
    assert thread_title("SUP-12", summary, 200) == "SUP-12 Login fails again"


def test_title_is_cut_to_code_points() -> None:
    summary = "\u00fc" * 300
    title = thread_title("SUP-12", summary, 50)
    assert len(title) == 50
    assert title == "SUP-12 " + "\u00fc" * 43
    # Bytes are not the measure: 43 two-byte characters exceed 50 bytes.
    assert len(title.encode("utf-8")) > 50


def test_empty_summary_leaves_the_key_alone() -> None:
    assert thread_title("SUP-12", "", 200) == "SUP-12"
    assert thread_title("SUP-12", " \u202e\x07\n ", 200) == "SUP-12"


# -- cutting --------------------------------------------------------------------------------


@pytest.mark.parametrize("limit", range(1, 40))
def test_cut_bytes_stops_at_a_code_point_boundary(limit: int) -> None:
    text = "a\u00e4\u20ac\U0001f600" * 4
    cut = cut_bytes(text, limit)
    assert text.startswith(cut)
    assert len(cut.encode("utf-8")) <= limit
    if cut != text:
        # The next code point would not fit: the cut is the longest prefix within the bound.
        assert len(text[: len(cut) + 1].encode("utf-8")) > limit


@pytest.mark.parametrize("limit", range(115, 170))
def test_cut_message_keeps_the_fence_closed_and_adds_the_cut_line(limit: int) -> None:
    text = "\u20ac\U0001f600\u00e4 @alex " * 20
    head = f"**Issue [SUP-12]({LINK}) created** by `Sam`"
    message = with_block(head, text, limit)
    assert len(message.encode("utf-8")) <= limit
    lines = message.split("\n")
    assert lines[0] == head
    assert lines[-1] == CUT_LINE
    assert lines[2] == "```"
    assert lines[-2] == "```"
    inner = "\n".join(lines[3:-2])
    assert text.startswith(inner)
    assert inner
    assert "@alex" not in _outside_code(message)


def test_render_event_cuts_to_message_max_content_bytes() -> None:
    limits = RenderLimits(message_max_content_bytes=120, thread_title_max_chars=200)
    message = render_event(_created("\u00e4" * 500), BASE, limits)
    assert len(message.encode("utf-8")) <= 120
    assert message.endswith(f"\n```\n{CUT_LINE}")
    assert message.startswith(f"**Issue [SUP-12]({LINK}) created** by `Sam`\n\n```\n")


def test_list_of_changes_is_cut_with_the_cut_line() -> None:
    changes = tuple(Change(f"field{n}", "\u20ac" * 10, "\U0001f600" * 10) for n in range(50))
    event = JiraEvent("jira:issue_updated", "10012", "SUP-12", "SUP", "x", "Sam", changes=changes)
    limits = RenderLimits(message_max_content_bytes=400, thread_title_max_chars=200)
    message = render_event(event, BASE, limits)
    assert len(message.encode("utf-8")) <= 400
    lines = message.split("\n")
    assert lines[-1] == CUT_LINE
    assert 1 < len(lines) - 1 < 51
    full = render_event(event, BASE, WIDE).split("\n")
    assert lines[:-1] == full[: len(lines) - 1]


# -- links ----------------------------------------------------------------------------------


def test_link_only_to_browse_of_a_valid_key() -> None:
    assert browse_link(BASE, "SUP-12") == f"[SUP-12]({LINK})"
    assert browse_link(BASE, "SUP-12)") == "`SUP-12)`"
    assert browse_link(BASE, "SUP-12](https://example.com/x)") == (
        "`SUP-12](https://example.com/x)`"
    )
    assert "](" not in _outside_code(browse_link(BASE, "SUP-12)"))


def test_payload_urls_never_become_links() -> None:
    payload = render_fixture("jira:issue_created", text=HOSTILE)
    payload["issue"]["self"] = "https://other.example.com/rest/api/2/issue/10012"
    payload["user"]["self"] = "https://other.example.com/rest/api/2/user"
    event = parse_event(payload, ["SUP"])
    assert isinstance(event, JiraEvent)
    message = render_event(event, BASE, WIDE)
    assert _URL.findall(_outside_code(message)) == [LINK]
    assert "other.example.com" not in message
    assert "/rest/api" not in message


def test_key_with_trailing_parenthesis_is_never_parsed_or_linked() -> None:
    payload = render_fixture("jira:issue_created", key="SUP-12)")
    assert not isinstance(parse_event(payload, ["SUP"]), JiraEvent)


# -- server retries -------------------------------------------------------------------------


@pytest.fixture
async def world(tmp_path: Path) -> AsyncIterator[World]:
    w = await start_world(tmp_path)
    try:
        yield w
    finally:
        await w.close()


@pytest.mark.anyio
async def test_validation_failed_on_title_retries_once_with_the_key_alone(world: World) -> None:
    world.chat.fail(
        "createThread",
        problem(400, "validation_failed", errors=[{"pointer": "/title", "code": "invalid"}]),
    )
    assert await world.deliver(render_fixture("jira:issue_created")) == 202
    await world.settle()
    calls = world.chat.calls_of("createThread")
    assert [c.args["title"] for c in calls] == ["SUP-12 Login fails", "SUP-12"]
    assert len(world.chat.threads) == 1
    assert next(iter(world.chat.threads.values())).title == "SUP-12"


@pytest.mark.anyio
async def test_validation_failed_on_title_twice_ends_after_one_retry(world: World) -> None:
    refused = problem(400, "validation_failed", errors=[{"pointer": "/title", "code": "invalid"}])
    world.chat.fail("createThread", refused, refused)
    assert await world.deliver(render_fixture("jira:issue_created")) == 202
    await world.settle()
    assert len(world.chat.calls_of("createThread")) == 2
    assert world.chat.threads == {}


@pytest.mark.anyio
async def test_too_long_reloads_the_limits_and_retries_once(world: World) -> None:
    before = world.chat.limit_calls
    world.chat.fail(
        "createThread",
        problem(
            400,
            "validation_failed",
            errors=[{"pointer": "/first_message/content_md", "code": "too_long"}],
        ),
    )
    # The server's limit is lower than the one the app loaded at start.
    world.chat.limit_values["message_max_content_bytes"] = 150
    payload = render_fixture("jira:issue_created", text="\u00e4" * 400)
    assert await world.deliver(payload) == 202
    await world.settle()
    assert world.chat.limit_calls == before + 1
    calls = world.chat.calls_of("createThread")
    assert len(calls) == 2
    first = calls[0].args["first_message"]["content_md"]
    second = calls[1].args["first_message"]["content_md"]
    assert len(first.encode("utf-8")) > 150
    assert len(second.encode("utf-8")) <= 150
    assert second.endswith(f"\n```\n{CUT_LINE}")
    assert len(world.chat.threads) == 1


@pytest.mark.anyio
async def test_too_long_twice_reloads_the_limits_only_once(world: World) -> None:
    before = world.chat.limit_calls
    refused = problem(
        400,
        "validation_failed",
        errors=[{"pointer": "/first_message/content_md", "code": "too_long"}],
    )
    world.chat.fail("createThread", refused, refused)
    assert await world.deliver(render_fixture("jira:issue_created")) == 202
    await world.settle()
    assert world.chat.limit_calls == before + 1
    assert len(world.chat.calls_of("createThread")) == 2
    assert world.chat.threads == {}
