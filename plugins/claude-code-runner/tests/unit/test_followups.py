# SPDX-License-Identifier: Apache-2.0
"""Follow-ups: the check at writing time, the line, the echo and the stdin rule (09 18.5)."""

from __future__ import annotations

import dataclasses
import json
from uuid import UUID, uuid4

import pytest
from bothy_sdk.invocation import InvocationClaims

from claude_code_runner.followups import (
    FollowupCheck,
    FollowupVerdict,
    PendingFollowups,
    answered_inputs,
    build_followup_line,
    followup_verdict,
    stdin_close_due,
)

RUN, THREAD, MESSAGE, SAM, ALEX = (uuid4() for _ in range(5))
NOW = 1_000_000.0
LEEWAY = 10.0


def claims(**changes: object) -> InvocationClaims:
    base = InvocationClaims(
        iss="instance",
        aud="agent",
        sub=SAM,
        jti="j",
        iat=int(NOW) - 30,
        exp=int(NOW) + 300,
        trigger_kind="followup",
        thread_id=THREAD,
        channel_id=uuid4(),
        space_id=uuid4(),
        level="trigger",
        billing_mode="organization",
        tool_class="scoped",
        permission_version=1,
        run_id=RUN,
        message_id=MESSAGE,
    )
    return dataclasses.replace(base, **changes)  # type: ignore[arg-type]


def check(**changes: object) -> FollowupCheck:
    base = FollowupCheck(
        claims=claims(),
        now=NOW,
        leeway=LEEWAY,
        turn_started=True,
        run_id=RUN,
        thread_id=THREAD,
        billing_mode="organization",
        tool_class="scoped",
        message_id=MESSAGE,
        author_id=SAM,
        message_thread_id=THREAD,
    )
    return dataclasses.replace(base, **changes)  # type: ignore[arg-type]


W, H, S, D = (
    FollowupVerdict.WRITE,
    FollowupVerdict.HOLD,
    FollowupVerdict.SKIP,
    FollowupVerdict.DUPLICATE,
)
CASES = [
    ("valid", check(), W),
    ("turn not started", check(turn_started=False), H),
    ("expired beyond the leeway", check(claims=claims(exp=int(NOW - LEEWAY) - 1)), S),
    ("expired inside the leeway", check(claims=claims(exp=int(NOW) - 1)), W),
    ("refused at acceptance (signature, aud)", check(claims=None), S),
    ("another run", check(claims=claims(run_id=uuid4())), S),
    ("trigger kind message", check(claims=claims(trigger_kind="message")), S),
    ("author other than sub", check(author_id=ALEX), S),
    ("message of another thread", check(message_thread_id=uuid4()), S),
    ("other billing mode", check(claims=claims(billing_mode="personal")), S),
    ("tool class below the loaded one", check(claims=claims(tool_class="chat")), S),
    ("tool class above the loaded one", check(claims=claims(tool_class="host")), W),
    ("already written", check(written={MESSAGE}), D),
]


@pytest.mark.parametrize(("name", "item", "verdict"), CASES, ids=[c[0] for c in CASES])
def test_followup_verdict(name: str, item: FollowupCheck, verdict: FollowupVerdict) -> None:
    assert followup_verdict(item)[0] is verdict, name


def test_build_followup_line() -> None:
    author = {"display_name": "Sam", "handle": "sam"}
    first = build_followup_line(author, "/compact")
    second = build_followup_line(author, "/compact")
    doc = json.loads(first)
    assert first.endswith(b"\n")
    assert first.count(b"\n") == 1
    assert doc["type"] == "user"
    assert doc["uuid"] != json.loads(second)["uuid"]
    UUID(doc["uuid"])
    [item] = doc["message"]["content"]
    assert item["text"] == "Follow-up from Sam (@sam):\n\n/compact"
    assert not item["text"].startswith("/")
    assert json.loads(build_followup_line(author, "x", "fixed"))["uuid"] == "fixed"


def test_echo_matching() -> None:
    pending = PendingFollowups()
    a, b = uuid4(), uuid4()
    pending.add(a, "ua", b"a\n")
    pending.add(b, "ub", b"b\n")
    assert pending.echo("own-prompt") is None
    assert pending.echo("unknown") is None
    # Echo before the delivered patch: absorbed only once delivered went out.
    assert pending.echo("ua") is None
    assert pending.delivered(a) is True
    # Delivered first, then the echo.
    assert pending.delivered(b) is False
    echoed = pending.echo("ub")
    assert echoed is not None
    assert echoed.message_id == b
    assert pending.echo("ub") is None  # a second echo changes nothing
    assert pending.pending() == []
    assert [r["message_id"] for r in pending.to_record()] == [str(a), str(b)]


def test_stdin_close_due() -> None:
    grace = 30.0
    assert stdin_close_due(5.0, None, 0, grace) is False  # no result yet
    assert stdin_close_due(5.0, 5.0, 0, grace) is True  # result, nothing pending
    assert stdin_close_due(6.0, 5.0, 1, grace) is False  # pending keeps it open
    assert stdin_close_due(7.0, 7.0, 0, grace) is True  # echo, then the next result
    assert stdin_close_due(35.0, 5.0, 1, grace) is True  # the grace has passed
    pending = PendingFollowups()
    pending.add(uuid4(), "u1", b"")
    pending.add(uuid4(), "u2", b"")  # written inside the window: joins the set
    assert stdin_close_due(10.0, 5.0, len(pending.pending()), grace) is False


def test_answered_inputs_by_the_results_uuids() -> None:
    order = ["p", "f1", "f2"]
    # A follow-up folded into the running turn: one result answers the prompt and f1.
    assert answered_inputs(order, set(), {"user_message_uuids": ["p", "f1"]}) == {"p", "f1"}
    # Only the last member named: every input before it was consumed too.
    assert answered_inputs(order, set(), {"user_message_uuid": "f1"}) == {"p", "f1"}
    # An unknown uuid or none at all: the first unanswered input (an older producer).
    assert answered_inputs(order, {"p"}, {"user_message_uuids": ["x"]}) == {"p", "f1"}
    assert answered_inputs(order, {"p", "f1"}, {}) == {"p", "f1", "f2"}
    assert answered_inputs(order, set(order), {}) == set(order)
