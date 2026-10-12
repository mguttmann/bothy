# SPDX-License-Identifier: Apache-2.0
"""Facts and verdicts of the six points, computed from synthetic recordings."""

from __future__ import annotations

import copy
from pathlib import Path
from typing import Any

import pytest
from hermes_harness.observations import (
    POINT_SCENARIOS,
    SLUGS,
    discovery_facts,
    egress_facts,
    observe,
    scenario_names,
    store_call_facts,
    surface_facts,
)
from hermes_harness.scenarios import ScenarioResult, load_scenarios

SCENARIOS = {s.name: s for s in load_scenarios()}
ANSWER = SCENARIOS["tool_and_text"].texts["answer"]
FOLLOWUP = SCENARIOS["busy_steer"].texts["followup"]
CURSOR = "▉"


def _c(method: str, **args: Any) -> dict[str, Any]:
    return {"method": method, "args": args}


def _req(
    turn: int | None,
    *,
    credential: str = "none",
    texts: list[str] | None = None,
    completed: bool = True,
) -> dict[str, Any]:
    return {
        "route": "openai.chat",
        "turn": turn,
        "scripted": turn is not None,
        "credential": credential,
        "scenario_texts": texts or [],
        "completed": completed,
    }


def _done(message: str = "m-1", outcome: str = "success") -> dict[str, Any]:
    return _c(
        "on_processing_complete",
        event={"message_id": message},
        outcome={"enum": "ProcessingOutcome", "value": outcome},
    )


def _start(message: str) -> dict[str, Any]:
    return _c("on_processing_start", event={"message_id": message})


def _r(
    name: str,
    calls: list[dict[str, Any]] | None = None,
    model: list[dict[str, Any]] | None = None,
    commands: list[dict[str, Any]] | None = None,
    sessions: dict[str, str] | None = None,
) -> ScenarioResult:
    return ScenarioResult(
        name=name,
        root=Path("/nowhere"),
        calls=[_c("connect", extra_keys=["thread_sessions_per_user"])] + (calls or []),
        model_requests=model or [],
        commands=commands or [],
        sessions=sessions or {},
    )


def _cred(kind: str, *, public: bool = False) -> dict[str, Any]:
    return {
        "command": "credential_type",
        "result": {"kind": kind, "surface": "resolver", "public": public, "source": "env"},
    }


def holding() -> dict[str, ScenarioResult]:
    """A recording in which every claim holds."""
    tool_event = _c(
        "format_tool_event", event={"type": "ToolCallChunk", "fields": {"tool_name": "todo_list"}}
    )
    results = {
        "tool_and_text": _r(
            "tool_and_text",
            [
                tool_event,
                _c("set_status_text", text="phrase"),
                _c("set_status_text", text=None),
                tool_event,
                _c("send_draft", content="\n\nThe trip"),
                _c("send_draft", content="\n\n" + ANSWER),
                _c("send", content=ANSWER, reply_to="m-1"),
                _done(),
            ],
        ),
        "tool_without_progress": _r(
            "tool_without_progress", [_c("send", content=ANSWER, reply_to="m-1")]
        ),
        "text_without_draft": _r(
            "text_without_draft",
            [
                _c(
                    "send",
                    content="The trip " + CURSOR,
                    reply_to="m-1",
                    metadata={"expect_edits": True},
                ),
                _c("edit_message", content=ANSWER, finalize=True),
            ],
        ),
        "interrupt": _r(
            "interrupt",
            [_done(outcome="cancelled"), _c("send", content="Stopped.", reply_to="m-2")],
            [_req(1)],
        ),
        "interrupt_command": _r("interrupt_command", [_done(outcome="cancelled")], [_req(1)]),
        "session_new": _r(
            "session_new",
            [_c("send_slash_confirm"), _c("hook:on_session_reset")],
            [_req(1, texts=["first"]), _req(3, texts=["third"])],
            [{"command": "confirm", "result": {"way": "resolver"}}],
            {"before": "s1", "after": "s2"},
        ),
        "session_resume": _r(
            "session_resume", sessions={"thread_one": "s1", "thread_two": "s2", "after": "s1"}
        ),
        "session_resume_admin": _r(
            "session_resume_admin", sessions={"thread_one": "s1", "after": "s1"}
        ),
        "session_resume_same_thread": _r(
            "session_resume_same_thread", sessions={"before": "s1", "after": "s1"}
        ),
        "session_resume_missing": _r(
            "session_resume_missing",
            [_c("send", reply_to="m-2")],
            sessions={"before": "s1", "after": "s1"},
        ),
        "access_policy_own": _r(
            "access_policy_own",
            [_c("read:enforces_own_access_policy", value=True), _c("send")],
            [_req(1)],
        ),
        "access_policy_default": _r(
            "access_policy_default", [_c("read:enforces_own_access_policy", value=False)]
        ),
    }
    busy = {
        "busy_steer": "steered",
        "busy_queue": "new_turn",
        "busy_interrupt": "interrupted",
        "busy_steer_global": "steered",
        "busy_queue_global": "new_turn",
        "busy_steer_command": "steered",
    }
    for name, kind in busy.items():
        if kind == "new_turn":
            results[name] = _r(
                name, [_start("m-1"), _start("m-2")], [_req(1), _req(3, texts=[FOLLOWUP])]
            )
        else:
            results[name] = _r(name, [_start("m-1")], [_req(1), _req(2, texts=[FOLLOWUP])])
            results[name].model_requests[1]["roles"] = (
                ["system", "user", "assistant"] + (["tool"] if kind == "steered" else []) + ["user"]
            )
    truth = {
        "credential_api_key_only": ["api_key"],
        "credential_oauth_only": ["oauth"],
        "credential_both": ["oauth"],
        "credential_none": [],
        "credential_login_file_only": ["oauth"],
        "credential_login_file_optout": [],
        "credential_other_provider": ["api_key"],
        "credential_per_turn": ["api_key", "oauth"],
    }
    for name, kinds in truth.items():
        model = [_req(i + 1, credential=k) for i, k in enumerate(kinds)]
        answers = [_cred(k, public=True) for k in (kinds or ["none"])]
        results[name] = _r(name, model=model, commands=answers)
    return results


def _variant(change: str) -> dict[str, ScenarioResult]:
    results = copy.deepcopy(holding())
    if change == "no_format_tool_event":
        results["tool_and_text"].calls = [
            c for c in results["tool_and_text"].calls if c["method"] != "format_tool_event"
        ]
    elif change == "progress_message":
        results["tool_and_text"].calls.append(_c("send", content="Progress line.", reply_to=None))
    elif change == "draft_not_answer":
        results["tool_and_text"].calls = [
            c for c in results["tool_and_text"].calls if c["args"].get("content") != "\n\n" + ANSWER
        ]
    elif change == "two_final_edits":
        results["text_without_draft"].calls.append(
            _c("edit_message", content=ANSWER, finalize=True)
        )
    elif change == "per_platform_ignored":
        results["busy_steer"].model_requests[1]["roles"] = ["system", "user", "assistant", "user"]
    elif change == "steer_lost":
        results["busy_steer"].model_requests = [_req(1)]
    elif change == "internal_surface":
        for name in POINT_SCENARIOS["credential_type"]:
            for command in results[name].commands:
                command["result"]["public"] = False
    elif change == "mismatch":
        results["credential_both"].commands = [_cred("api_key", public=True)]
    elif change == "api_key_wins":
        results["credential_both"].model_requests = [_req(1, credential="api_key")]
        results["credential_both"].commands = [_cred("api_key", public=True)]
    elif change == "no_way_stops":
        for name in ("interrupt", "interrupt_command"):
            results[name].model_requests.append(_req(2))
    elif change == "not_cancelled":
        results["interrupt"].calls[1] = _done(outcome="success")
        results["interrupt"].model_requests.append(_req(2))
        results["interrupt_command"].calls[1] = _done(outcome="success")
    elif change == "new_keeps_session":
        results["session_new"].sessions["after"] = "s1"
    elif change == "bind_blocked":
        for name in ("session_resume", "session_resume_admin"):
            results[name].sessions["after"] = "s2"
    elif change == "same_thread_blocked":
        results["session_resume_same_thread"].sessions["after"] = "s3"
    elif change == "declaration_ignored":
        results["access_policy_own"].model_requests = []
    elif change == "default_open":
        results["access_policy_default"].model_requests = [_req(1)]
    return results


FAILING = {
    "progress_hooks": [
        "no_format_tool_event",
        "progress_message",
        "draft_not_answer",
        "two_final_edits",
    ],
    "steer": ["per_platform_ignored", "steer_lost"],
    "credential_type": ["internal_surface", "mismatch", "api_key_wins"],
    "interrupt_outcome": ["no_way_stops", "not_cancelled"],
    "session_switch": ["new_keeps_session", "bind_blocked", "same_thread_blocked"],
    "own_access_policy": ["declaration_ignored", "default_open"],
}


def test_claim_of_each_point_holds_on_a_holding_recording() -> None:
    points = observe(holding(), SCENARIOS)
    assert list(points) == list(SLUGS)
    assert {slug: p["holds"] for slug, p in points.items()} == dict.fromkeys(SLUGS, True)


@pytest.mark.parametrize(("slug", "change"), [(s, c) for s, cs in FAILING.items() for c in cs])
def test_claim_of_each_point(slug: str, change: str) -> None:
    points = observe(_variant(change), SCENARIOS)
    assert points[slug]["holds"] is False
    others = {s: p["holds"] for s, p in points.items() if s != slug}
    assert all(others.values()), others


def test_facts_name_the_details() -> None:
    points = observe(holding(), SCENARIOS)
    progress = points["progress_hooks"]["facts"]
    assert progress["format_tool_event"] == {"calls": 2, "event_fields": ["tool_name"]}
    assert progress["send_draft"]["cumulative"] is True
    assert progress["send_draft"]["leading_whitespace_after_tool"] is True
    assert progress["answer_without_draft"]["preview_send_with_cursor"] is True
    steer = points["steer"]["facts"]
    assert steer["per_platform_behaviour"] == {
        "steer": "steered",
        "queue": "new_turn",
        "interrupt": "interrupted",
    }
    cred = points["credential_type"]["facts"]
    assert cred["profiles"]["credential_none"]["ground_truth"] == ["none"]
    assert cred["profiles"]["credential_api_key_only"]["sources"] == [
        "process environment: anthropic_api_key"
    ]
    assert cred["rechosen_per_turn"] is True
    interrupt = points["interrupt_outcome"]["facts"]
    assert interrupt["way_used"] == "BasePlatformAdapter.cancel_session_processing(session_key)"
    assert (
        interrupt["ways"]["BasePlatformAdapter.cancel_session_processing(session_key)"][
            "acknowledgement_sent"
        ]
        is True
    )
    assert points["session_switch"]["facts"]["new"]["confirmation_resolved_with"] == "resolver"
    assert points["own_access_policy"]["facts"]["own"]["declaration_value"] is True


def test_run_wide_facts() -> None:
    results = holding()
    results["tool_and_text"].commands.append(
        {"command": "describe", "result": {"base.send": {"present": True}}}
    )
    results["credential_oauth_only"].store_calls = ["security find-generic-password"] * 3
    results["credential_both"].egress = [("models.dev:443", "start"), ("models.dev:443", "start")]
    assert surface_facts(results) == {
        "base.send": {"present": True},
        "config.extra:thread_sessions_per_user": {"present": True, "kind": "seeded extra key"},
    }
    assert surface_facts({}) == {}
    assert egress_facts(results) == [{"destination": "models.dev:443", "phase": "start"}]
    assert store_call_facts(results) == {
        "credential_oauth_only": ["security find-generic-password"]
    }
    assert discovery_facts(results)["loaded_in_every_scenario"] is True
    assert set(scenario_names()) == set(results)
