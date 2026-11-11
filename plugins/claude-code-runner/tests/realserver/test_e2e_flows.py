# SPDX-License-Identifier: Apache-2.0
"""Every flow of the Claude Code runner against the real server of this tree.

HERMES-E2E-REAL-SERVER: the runner's own code with Fake-Claude, the server is sdkserver (07
"Examples"). The runner is Lotse: it answers a mention in the main thread of Support/Allgemein in
the auto thread, a post in a forum of Support, a reply in an existing thread, sends files with its
answer, transcribes voice messages with a local program, stops on a cancel, and serves a retry.
A request the server does not serve or refuses shows up as a failed run or a missing message.
"""

from __future__ import annotations

import sys
from datetime import UTC, datetime
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.ids import uuid7
from bothy_sdk.testing.sdkserver import ApiAnswer, RealServer, m4a

from .conftest import RealWorld

pytestmark = [pytest.mark.anyio, pytest.mark.e2e]

_FORUMS = iter(range(1, 1000))
ENDED = ("completed", "failed", "cancelled")


def _ok(answer: ApiAnswer, *statuses: int) -> Any:
    assert answer.status in (statuses or (200, 201)), f"status {answer.status}: {answer.body}"
    return answer.body


async def _until(what: str, check: Any) -> Any:
    import asyncio
    import time

    from bothy_sdk.testing._patience import PATIENCE

    deadline = time.monotonic() + PATIENCE
    while True:
        value = await check()
        if value:
            return value
        if time.monotonic() > deadline:
            raise AssertionError(f"not within {PATIENCE} s: {what}")
        await asyncio.sleep(0.1)


async def _ended(real: RealServer, run_id: UUID) -> Any:
    async def check() -> Any:
        view = await real.run_state(run_id)
        return view if view.state in ENDED else None

    return await _until(f"the end of run {run_id}", check)


async def _post(real: RealServer, thread_id: UUID, text: str, **extra: Any) -> UUID:
    body = {"client_msg_id": str(uuid7()), "content_md": text, **extra}
    created = _ok(await real.call("createMessage", path={"thread_id": thread_id}, body=body), 201)
    return UUID(created["id"])


async def _agent_messages(real: RealServer, thread_id: UUID) -> list[dict[str, Any]]:
    page = _ok(await real.call("listThreadMessages", path={"thread_id": thread_id}))
    items = sorted(page["items"], key=lambda m: int(m.get("thread_seq") or 0))
    return [
        m
        for m in items
        if (m.get("author") or {}).get("id") == str(real.agent_id) and m.get("kind") != "run"
    ]


async def _heartbeat(real: RealServer) -> None:
    """Wait for a heartbeat of the agent after now: an invocation carries the features of its
    time, and the runner reports its capabilities (followups, voice_input) with the heartbeat."""
    since = datetime.now(UTC)

    async def check() -> bool:
        agent = _ok(await real.call("getAgent", as_="owner", path={"agent_id": real.agent_id}))
        at = (agent.get("presence") or {}).get("last_heartbeat_at")
        return at is not None and datetime.fromisoformat(at.replace("Z", "+00:00")) > since

    await _until("a heartbeat of the agent", check)


async def _forum_post(real: RealServer, text: str) -> tuple[UUID, UUID]:
    forum = await real.create_forum(f"Projekt {next(_FORUMS)}")
    body = {"title": "Login", "first_message": {"client_msg_id": str(uuid7()), "content_md": text}}
    created = _ok(await real.call("createThread", path={"channel_id": forum}, body=body), 201)
    return forum, UUID(created["first_message"]["id"])


async def test_forum_post_is_answered_and_titled(world: RealWorld) -> None:
    real = world.real
    await world.start()
    _, message = await _forum_post(real, "Please fix the login form")
    run_id, thread_id = await real.wait_run(message)
    view = await _ended(real, run_id)
    assert (view.state, view.state_reason) == ("completed", None)
    answers = await _agent_messages(real, thread_id)
    assert len(answers) == 1
    assert answers[0]["content_md"].strip()
    assert len(world.turns()) == 1


async def test_mention_in_the_main_thread_answers_in_the_auto_thread(world: RealWorld) -> None:
    real = world.real
    await world.start()
    message = await _post(real, real.world.main_thread_id, "@lotse the export fails")
    run_id, thread_id = await real.wait_run(message)
    assert thread_id != real.world.main_thread_id
    view = await _ended(real, run_id)
    assert view.state == "completed"
    answers = await _agent_messages(real, thread_id)
    assert len(answers) == 1
    assert answers[0].get("reply_to_message_id") is None


async def test_reply_in_an_existing_thread_resumes_the_session(world: RealWorld) -> None:
    real = world.real
    await world.start()
    first = await real.trigger("Hello @lotse")
    assert (await _ended(real, first.run_id)).state == "completed"
    message = await _post(real, first.thread_id, "And the import?")
    run_id, thread_id = await real.wait_run(message)
    assert thread_id == first.thread_id
    assert (await _ended(real, run_id)).state == "completed"
    assert len(await _agent_messages(real, thread_id)) == 2
    turns = world.turns()
    assert len(turns) == 2
    assert "--resume" in turns[1]["argv"]


async def test_answer_with_an_attachment(world: RealWorld) -> None:
    real = world.real
    report = world.projects / "website" / "dist" / "report.md"
    report.parent.mkdir(parents=True)
    report.write_text("# Report\n", encoding="utf-8")
    world.scenario(fixture="attach_only")
    await world.start()
    first = await real.trigger("Send me the report @lotse")
    view = await _ended(real, first.run_id)
    assert view.state == "completed"
    answers = await _agent_messages(real, first.thread_id)
    names = [f["file"]["original_name"] for a in answers for f in a.get("attachments") or []]
    assert names == ["report.md"]


async def test_voice_message_is_transcribed(world: RealWorld) -> None:
    real = world.real
    transcriber = world.root / "transcribe"
    transcriber.write_text(f"#!{sys.executable}\nprint('VOICE please run the tests')\n")
    transcriber.chmod(0o755)
    await world.start(world.config(transcribe_cmd=[str(transcriber)]))
    first = await real.trigger("Hello @lotse")
    await _ended(real, first.run_id)
    uploaded: list[UUID] = []

    async def voice_run() -> Any:
        if not uploaded:
            uploaded.append(
                await real.upload(real.world.channel_id, "memo.m4a", m4a(), "audio/mp4")
            )
        answer = await real.call(
            "createMessage",
            path={"thread_id": first.thread_id},
            body={
                "client_msg_id": str(uuid7()),
                "content_md": "",
                "kind": "voice",
                "attachment_file_ids": [str(uploaded[-1])],
            },
        )
        if answer.status != 201:
            assert answer.member("code") == "file_not_ready", answer.body
            return None
        got = _ok(await real.call("getMessage", path={"message_id": answer.body["id"]}))
        if got.get("trigger_state") in (None, "failed"):
            uploaded.clear()  # posted before the heartbeat reported voice_input
            return None
        return answer.body["id"]

    message = UUID(await _until("an accepted voice message", voice_run))
    run_id, _ = await real.wait_run(message)
    assert (await _ended(real, run_id)).state == "completed"
    stdin = world.turns()[-1]["stdin"][0]
    assert "VOICE please run the tests" in stdin
    file = _ok(await real.call("getFile", path={"file_id": uploaded[-1]}))
    assert (file["transcript_status"], file["transcript_source"]) == ("done", "agent")


async def test_followup_reaches_the_running_turn(world: RealWorld) -> None:
    real = world.real
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start()
    await _heartbeat(real)
    first = await real.trigger("Fix the form @lotse")

    async def waiting() -> bool:
        return len(world.calls("gate_wait")) == 1

    await _until("the turn at its gate", waiting)
    followup = await _post(real, first.thread_id, "Also check the footer")

    async def written() -> bool:
        return len(world.turns()[0]["stdin"]) == 2

    await _until("the follow-up on the turn's stdin", written)
    assert "Also check the footer" in world.turns()[0]["stdin"][1]
    world.gate("turn_gate").touch()
    assert (await _ended(real, first.run_id)).state == "completed"
    got = _ok(await real.call("getMessage", path={"message_id": followup}))
    assert got["trigger_state"] == "followup_delivered"


async def test_cancel_stops_the_turn(world: RealWorld) -> None:
    real = world.real
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start()
    first = await real.trigger("Take your time @lotse")

    async def waiting() -> bool:
        return len(world.calls("gate_wait")) == 1

    await _until("the turn at its gate", waiting)
    _ok(await real.call("cancelRun", path={"run_id": first.run_id}, body={"reason": "e2e"}), 202)
    view = await _ended(real, first.run_id)
    assert view.state == "cancelled"
    assert await _agent_messages(real, first.thread_id) == []


async def test_cancel_reaches_the_retry_after_a_stop_after_an_error(world: RealWorld) -> None:
    # A turn fills the context while its background job runs: the runner stops that process
    # after the error result, compacts and tries the turn again (09 18.5, 18.9). A cancel
    # applies to the process that runs (18.6); the stop of the first process once stayed set
    # for the whole run, so the retry was never signalled and worked on to its own end.
    real = world.real
    job = world.gate("background_gate")  # never opened: the job outlives the failed turn
    first = {
        "api_error": "context_full",
        "api_error_after": 1,
        "background_gate": str(job),
        "background_survives_eof": True,
    }
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", per_process={"0": first, "2": {"turn_gate": str(gate)}})
    await world.start()
    run = await real.trigger("Fix the form @lotse")

    async def waiting() -> bool:
        return len(world.calls("gate_wait")) == 1

    await _until("the retry at its gate", waiting)
    stopped_after_error, _compaction, retry = world.turns()
    _ok(await real.call("cancelRun", path={"run_id": run.run_id}, body={"reason": "e2e"}), 202)

    def signal_of(pid: int) -> str | None:
        return next((c.get("signal") for c in world.calls("turn_end") if c["pid"] == pid), None)

    async def signalled() -> bool:
        return signal_of(retry["pid"]) == "SIGINT"

    await _until("SIGINT at the retry", signalled)
    assert signal_of(stopped_after_error["pid"]) == "SIGINT"
    view = await _ended(real, run.run_id)
    assert (view.state, view.state_reason) == ("cancelled", "user_cancelled")
    assert len(world.turns()) == 3  # nothing started after the stop
    assert await _agent_messages(real, run.thread_id) == []


async def test_retry_of_a_failed_run(world: RealWorld) -> None:
    real = world.real
    world.scenario(fixture="error_result")
    await world.start()
    message = await _post(real, real.world.main_thread_id, "@lotse try the import")
    run_id, thread_id = await real.wait_run(message)
    assert (await _ended(real, run_id)).state == "failed"
    world.scenario(fixture="happy")
    retried = _ok(await real.call("retryRun", path={"run_id": run_id}), 202)
    second = UUID(retried["id"])
    assert second != run_id
    assert (await _ended(real, second)).state == "completed"
    assert len(await _agent_messages(real, thread_id)) >= 1


async def _put_operator(real: RealServer, channel: UUID, body: dict[str, Any]) -> ApiAnswer:
    current = _ok(await real.call("getChannel", as_="owner", path={"channel_id": channel}))
    return await real.call(
        "setChannelOperator",
        as_="owner",
        path={"channel_id": channel},
        body={**body, "reason": "e2e"},
        header={"If-Match": f'"{current["channel"]["version"]}"'},
    )


async def _put_space_operator(real: RealServer, space: UUID, agent: UUID) -> None:
    current = _ok(await real.call("getSpace", as_="owner", path={"space_id": space}))
    _ok(
        await real.call(
            "setSpaceOperator",
            as_="owner",
            path={"space_id": space},
            body={"mode": "agent", "agent_id": str(agent), "reason": "e2e"},
            header={"If-Match": f'"{current["space"]["version"]}"'},
        )
    )


async def _binding(real: RealServer, forum: UUID) -> dict[str, Any]:
    behavior = _ok(await real.call("getChannelBehavior", as_="owner", path={"channel_id": forum}))
    return behavior.get("project_binding") or {}


def _field(answer: ApiAnswer) -> tuple[str, str]:
    error = (answer.body.get("errors") or [{}])[0]
    return error.get("pointer", ""), error.get("code", "")


async def test_project_forum_with_a_chat_agent_says_why_and_is_triggered_again(
    world: RealWorld,
) -> None:
    """F-PROJEKT-BEREIT, the reported case (09 3.8, 18.16): a chat agent cannot become
    the operator of a project forum; a bound forum that a chat agent operates anyway (through its
    space) explains itself, its post fails with that reason, and after the owner made the runner
    the operator the post is triggered again and the runner completes it."""
    real = world.real
    (world.projects / "shop").mkdir()
    await world.start()
    space = UUID(
        _ok(await real.call("createSpace", as_="owner", body={"name": "Projects"}), 201)["id"]
    )
    await _put_space_operator(real, space, real.agent_id)
    forum = UUID(
        _ok(
            await real.call(
                "createChannel",
                as_="owner",
                path={"space_id": space},
                body={"kind": "forum", "name": "Shop"},
            ),
            201,
        )["id"]
    )
    bind = {"workspace_ref": "shop", "provision": "none", "reason": "e2e"}
    _ok(await real.call("putProjectBinding", as_="owner", path={"channel_id": forum}, body=bind))

    # The wrong operator is refused with its reason.
    refused = await _put_operator(
        real, forum, {"mode": "agent", "agent_id": str(real.other_agent_id)}
    )
    assert refused.status == 400
    assert _field(refused) == ("/agent_id", "operator_cannot_serve_project")
    other = UUID(
        _ok(
            await real.call(
                "createChannel",
                as_="owner",
                path={"space_id": real.world.space_id},
                body={"kind": "forum", "name": "Chat forum"},
            ),
            201,
        )["id"]
    )
    _ok(await _put_operator(real, other, {"mode": "agent", "agent_id": str(real.other_agent_id)}))
    refused = await real.call(
        "putProjectBinding",
        as_="owner",
        path={"channel_id": other},
        body={**bind, "workspace_ref": "chat"},
    )
    assert refused.status == 400
    assert _field(refused) == ("#path/channel_id", "operator_cannot_serve_project")

    # The space's default becomes the chat agent: the bound forum says why it is not ready.
    await _put_space_operator(real, space, real.other_agent_id)
    assert (await _binding(real, forum)).get("ready_reason") == "operator_cannot_serve_project"
    body = {
        "title": "Build",
        "first_message": {"client_msg_id": str(uuid7()), "content_md": "Set it up"},
    }
    created = _ok(
        await real.call("createThread", as_="owner", path={"channel_id": forum}, body=body), 201
    )
    message = UUID(created["first_message"]["id"])
    got = _ok(await real.call("getMessage", as_="owner", path={"message_id": message}))
    assert (got["trigger_state"], got.get("trigger_state_reason")) == (
        "failed",
        "operator_cannot_serve_project",
    )

    # The owner makes the runner the forum's operator; the runner reports the project ready.
    _ok(await _put_operator(real, forum, {"mode": "agent", "agent_id": str(real.agent_id)}))

    async def ready() -> bool:
        return bool((await _binding(real, forum)).get("ready"))

    await _until("the runner reports the project ready", ready)
    again = _ok(await real.call("retriggerMessage", as_="owner", path={"message_id": message}))
    assert again["trigger_state"] == "received"
    run_id, _ = await real.wait_run(message)
    view = await _ended(real, run_id)
    assert (view.state, view.state_reason) == ("completed", None)
    assert len(world.turns()) == 1
