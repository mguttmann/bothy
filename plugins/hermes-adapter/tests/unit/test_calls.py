# SPDX-License-Identifier: Apache-2.0
"""Normalization of recorded calls and the pattern comparison."""

from __future__ import annotations

import json
import re

from hermes_harness.calls import (
    CURSOR,
    RECORD_HINT,
    Redactions,
    compare_patterns,
    derive_pattern,
    digest,
    normalize,
)

ANSWER = "The trip to the lake is planned for Saturday morning."
ASK = "Please plan the trip to the lake."


def _red(**kwargs: object) -> Redactions:
    base: dict[str, object] = {
        "tmp_root": "/var/tmp/hr-x1",
        "home": "/home/robin",
        "host": "box7",
        "user": "hermes",
        "scenario_texts": (ASK, ANSWER),
    }
    base.update(kwargs)
    return Redactions(**base)  # type: ignore[arg-type]


def _call(method: str, **args: object) -> dict[str, object]:
    return {"seq": 1, "method": method, "args": args}


def test_ids_become_placeholders_in_order_of_first_use() -> None:
    calls = [
        _call(
            "send", chat_id="c-general", content=ASK, metadata={"thread_id": "t-1"}, message_id="p1"
        ),
        _call("edit_message", chat_id="c-general", message_id="p1", content=ANSWER, finalize=True),
        _call("hook:pre_llm_call", session_id="20261012_065350_7db892b5"),
        _call(
            "send", chat_id="c-general", content="/resume 20261012_065350_7db892b5", message_id="p2"
        ),
        _call("send_draft", chat_id="c-general", draft_id=41636727526180, content=ANSWER),
    ]
    out = normalize(calls, _red(scenario_texts=(ASK, ANSWER, "/resume 20261012_065350_7db892b5")))
    assert out[0]["args"]["chat_id"] == "<id:1>"
    assert out[0]["args"]["metadata"]["thread_id"] == "<id:2>"
    assert out[0]["args"]["message_id"] == "<id:3>"
    assert out[1]["args"]["message_id"] == "<id:3>"
    assert out[2]["args"]["session_id"] == "<id:4>"
    assert out[3]["args"]["content"] == "/resume <id:4>"
    assert out[4]["args"]["draft_id"] == "<id:6>"
    assert "seq" not in out[0]


def test_temp_root_home_host_and_user_are_redacted() -> None:
    calls = [
        _call(
            "send_document",
            file_path="/var/tmp/hr-x1/00/work/report.txt",
            caption="see /home/robin/notes.txt",
            host="box7",
            user="hermes",
            elsewhere="/srv/hermes/box7/file.txt",
        )
    ]
    args = normalize(calls, _red())[0]["args"]
    assert args["file_path"] == "<tmp>/00/work/report.txt"
    assert args["host"] == "<host>"
    assert args["user"] == "<user>"
    # A path outside <tmp> and <home> is no placeholder path: hashed, after the redaction.
    assert args["elsewhere"] == digest("/srv/<user>/<host>/file.txt")
    # A longer third-party text is hashed after the redaction, so it never shows the path.
    assert args["caption"].startswith("sha256:")


def test_user_and_host_inside_tokens_stay_verbatim() -> None:
    calls = [
        _call(
            "x", module="hermes_cli.config", path="/var/tmp/hr-x1/hermes/probe", tool="hermes-agent"
        )
    ]
    args = normalize(calls, _red())[0]["args"]
    assert args["module"] == "hermes_cli.config"
    assert args["path"] == "<tmp>/hermes/probe"
    assert args["tool"] == "hermes-agent"


def test_normalization_does_not_depend_on_user_or_host() -> None:
    def calls(root: str, home: str, user: str, host: str) -> list[dict[str, object]]:
        return [
            _call("send", content=f"{root}/x", where=f"/srv/{user}/{host}", who=user, at=host),
            _call("send", content=f"{home}/y", message_id="p1"),
        ]

    one = normalize(
        calls("/tmp/a1", "/home/alex", "alex", "h1"),
        _red(tmp_root="/tmp/a1", home="/home/alex", user="alex", host="h1"),
    )
    two = normalize(
        calls("/var/b2", "/export/home/sam", "sam", "h2"),
        _red(tmp_root="/var/b2", home="/export/home/sam", user="sam", host="h2"),
    )
    assert json.dumps(one, sort_keys=True) == json.dumps(two, sort_keys=True)


def test_third_party_text_is_hashed_and_scenario_text_kept() -> None:
    calls = [
        _call("send", content="Third party status line with many words."),
        _call("send", content=ASK),
        _call("send", content="\n\n" + ANSWER),
        _call("send", content="The trip to the lake  " + CURSOR),
        _call("send", content=ANSWER + " " + CURSOR),
        _call("send", content="todo_list"),
        _call(
            "send",
            content=None,
            flag=True,
            count=3,
            outcome={"enum": "ProcessingOutcome", "value": "success"},
        ),
    ]
    out = [c["args"] for c in normalize(calls, _red())]
    assert re.fullmatch(r"sha256:[0-9a-f]{64}:\d+", out[0]["content"])
    assert out[1]["content"] == ASK
    assert out[2]["content"] == "<ws>" + ANSWER
    assert out[3]["content"] == "<partial><cursor>"
    assert out[4]["content"] == ANSWER + "<cursor>"
    assert out[5]["content"] == "todo_list"
    assert out[6] == {
        "content": None,
        "flag": True,
        "count": 3,
        "outcome": {"enum": "ProcessingOutcome", "value": "success"},
    }


def test_timestamps_and_durations_are_dropped() -> None:
    calls = [_call("x", created=17, duration_ms=4, started_at="2026-10-12", kept="ok")]
    assert normalize(calls, _red())[0]["args"] == {"kept": "ok"}


def test_pattern_collapses_only_draft_and_status_runs() -> None:
    calls = normalize(
        [
            _call("set_status_text", chat_id="c", text="Working on it now please wait."),
            _call("set_status_text", chat_id="c", text=None),
            _call("send_typing", chat_id="c"),
            _call("send_draft", chat_id="c", draft_id=5, content="The trip"),
            _call("send_draft", chat_id="c", draft_id=5, content=ANSWER),
            _call("send", chat_id="c", content=ANSWER, message_id="p1"),
            _call("send", chat_id="c", content=ANSWER, message_id="p2"),
            _call("read:enforces_own_access_policy", value=True),
            _call("hook:pre_llm_call", session_id="20261012_065350_7db892b5"),
            _call(
                "on_processing_complete", outcome={"enum": "ProcessingOutcome", "value": "success"}
            ),
        ],
        _red(),
    )
    pattern = derive_pattern(calls)
    assert [p["method"] for p in pattern] == [
        "send_draft",
        "send",
        "send",
        "on_processing_complete",
        "set_status_text",
    ]
    assert pattern[4]["args"]["text"] is None
    assert pattern[0]["args"]["content"] == ANSWER
    # IDs are renumbered within the pattern, so calls left out (hooks, timers) cannot shift them.
    assert pattern[0]["args"]["draft_id"] == "<id:2>"
    assert pattern[4]["args"]["chat_id"] == "<id:1>"
    assert pattern[3]["args"]["outcome"] == {"enum": "ProcessingOutcome", "value": "success"}


def test_status_text_position_does_not_count() -> None:
    # Hermes calls set_status_text from the agent thread and sends progress from its event
    # loop, so their interleaving depends on thread scheduling.
    phrase = _call("set_status_text", chat_id="c", text="Working on it now please wait.")
    cleared = _call("set_status_text", chat_id="c", text=None)
    bubble = _call(
        "send", chat_id="c", content="Progress line of the gateway here.", message_id="p1"
    )
    answer = _call("send", chat_id="c", content=ANSWER, message_id="p2")
    first = derive_pattern(normalize([phrase, cleared, bubble, answer], _red()))
    second = derive_pattern(normalize([phrase, bubble, cleared, answer], _red()))
    assert first == second
    assert [p["method"] for p in first] == ["send", "send", "set_status_text"]
    assert first[-1]["args"]["text"] is None


def test_pattern_folds_streaming_edits_and_drops_no_op_edits() -> None:
    calls = normalize(
        [
            _call("send", chat_id="c", content="The trip to the lake " + CURSOR, message_id="p1"),
            _call(
                "edit_message",
                chat_id="c",
                message_id="p1",
                content="The trip to the lake is " + CURSOR,
                finalize=False,
            ),
            _call(
                "edit_message",
                chat_id="c",
                message_id="p1",
                content=ANSWER + " " + CURSOR,
                finalize=False,
            ),
            _call("edit_message", chat_id="c", message_id="p1", content=ANSWER, finalize=True),
            _call(
                "send", chat_id="c", content="Progress line of the gateway here.", message_id="p2"
            ),
            _call(
                "edit_message",
                chat_id="c",
                message_id="p2",
                content="Progress line of the gateway here.",
                finalize=False,
            ),
        ],
        _red(),
    )
    pattern = derive_pattern(calls)
    assert [(p["method"], p["args"].get("finalize")) for p in pattern] == [
        ("send", None),
        ("edit_message", False),
        ("edit_message", True),
        ("send", None),
    ]
    assert pattern[1]["args"]["content"] == ANSWER + "<cursor>"


def test_compare_names_the_first_difference_and_the_record_hint() -> None:
    expected = [
        {"method": "send", "args": {"content": "a"}},
        {"method": "edit_message", "args": {}},
    ]
    assert compare_patterns(expected, list(expected)) == []
    changed = [{"method": "send", "args": {"content": "b"}}, {"method": "edit_message", "args": {}}]
    problems = compare_patterns(expected, changed)
    assert "entry 1" in problems[0]
    assert "content" in problems[0]
    assert problems[-1] == RECORD_HINT
    shorter = compare_patterns(expected, expected[:1])
    assert "missing" in shorter[0]
    longer = compare_patterns(expected[:1], expected)
    assert "unexpected" in longer[0]
