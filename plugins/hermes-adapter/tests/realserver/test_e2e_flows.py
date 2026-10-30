# SPDX-License-Identifier: Apache-2.0
"""Every flow of the Hermes adapter against the real server of this tree (HERMES-E2E-REAL-SERVER).

The adapter's own code (`HermesHandler`, `HermesAdapterLoop`, the SDK loop) runs with the fake
Hermes runtime of ``adapter_kit``; the server is sdkserver with the world of 07 "Examples": Alex
triggers Lotse in Support/Allgemein (a text channel with the defaults ``auto_thread`` and
``require_mention`` in the main thread), forums are created in the space Support, where Lotse
operates. A request the server does not serve or refuses shows up here as a failed run or a
missing message, which the FakeServer of the unit tests cannot show (the classes behind
SERVER-AGENT-TRIGGERERS and this ticket).
"""

from __future__ import annotations

import asyncio
import itertools
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from adapter_kit import FakeRuntime, card_script, rig
from bothy_sdk.ids import uuid7
from bothy_sdk.testing.sdkserver import RealServer, m4a
from real_kit import agent_texts, consumer, ended, heartbeat, messages, ok, post, until

from bothy_hermes_adapter.hermes_commands import declared_commands
from bothy_hermes_adapter.runtime import (
    CardKind,
    CardRequest,
    MediaKind,
    TurnOutcome,
    TurnRequest,
)

pytestmark = [pytest.mark.anyio, pytest.mark.e2e]

_FORUMS = itertools.count(1)


async def _forum(real: RealServer) -> UUID:
    return await real.create_forum(f"Hilfe {next(_FORUMS)}")


async def _post_in_forum(real: RealServer, forum: UUID, title: str, text: str) -> UUID:
    body = {"title": title, "first_message": {"client_msg_id": str(uuid7()), "content_md": text}}
    created = ok(await real.call("createThread", path={"channel_id": forum}, body=body), 201)
    return UUID(created["first_message"]["id"])


def _no_run(item: dict[str, Any]) -> bool:
    return item.get("run_id") is None


# ---- triggers: auto thread, existing thread, forum post ---------------------------------------


async def test_mention_in_the_main_thread_answers_in_the_auto_thread(
    real: RealServer, tmp_path: Path
) -> None:
    async with rig(tmp_path, fake=real, consumer=consumer()):
        message = await post(real, real.world.main_thread_id, "@lotse the export fails")
        run_id, thread_id = await real.wait_run(message)
        assert thread_id != real.world.main_thread_id  # the server opened the auto thread
        view = await ended(real, run_id)
    assert (view.state, view.state_reason) == ("completed", None)
    answers = agent_texts(await messages(real, thread_id), real.agent_id)
    assert [a["content_md"] for a in answers] == ["Done so far."]
    # The trigger lies in the main thread, so the answer in the auto thread quotes nothing.
    assert answers[0].get("reply_to_message_id") is None


async def test_reply_in_an_existing_thread_quotes_the_trigger(
    real: RealServer, tmp_path: Path
) -> None:
    async with rig(tmp_path, fake=real, consumer=consumer()):
        first = await real.trigger("Hello @lotse")
        assert (await ended(real, first.run_id)).state == "completed"
        message = await post(real, first.thread_id, "And the import?")
        run_id, thread_id = await real.wait_run(message)
        assert thread_id == first.thread_id
        view = await ended(real, run_id)
    assert view.state == "completed"
    answers = agent_texts(await messages(real, thread_id), real.agent_id)
    assert len(answers) == 2
    assert answers[0]["reply_to_message_id"] == str(first.message_id)
    assert answers[-1]["reply_to_message_id"] == str(message)


async def test_forum_post_is_answered_in_the_post(real: RealServer, tmp_path: Path) -> None:
    forum = await _forum(real)
    async with rig(tmp_path, fake=real, consumer=consumer()):
        message = await _post_in_forum(real, forum, "Export broken", "The export fails")
        run_id, thread_id = await real.wait_run(message)
        view = await ended(real, run_id)
    assert view.state == "completed"
    answers = agent_texts(await messages(real, thread_id), real.agent_id)
    assert [a["content_md"] for a in answers] == ["Done so far."]
    assert answers[0]["reply_to_message_id"] == str(message)


# ---- reports without a run (cron jobs, hermes send) -------------------------------------------


async def test_report_into_a_text_channel_lands_in_its_main_thread(
    real: RealServer, tmp_path: Path
) -> None:
    async with rig(tmp_path, fake=real, consumer=consumer()) as r:
        posted = await r.handler.on_report(str(real.world.channel_id), None, "Nightly report")
    assert posted is not None
    items = await messages(real, real.world.main_thread_id)
    mine = [m for m in items if m["id"] == str(posted)]
    assert [m["content_md"] for m in mine] == ["Nightly report"]
    assert _no_run(mine[0])


async def test_report_into_a_fixed_thread(real: RealServer, tmp_path: Path) -> None:
    async with rig(tmp_path, fake=real, consumer=consumer()) as r:
        first = await real.trigger("Hello @lotse")
        await ended(real, first.run_id)
        posted = await r.handler.on_report(
            str(real.world.channel_id), str(first.thread_id), "Follow-up report"
        )
    items = await messages(real, first.thread_id)
    assert [m["content_md"] for m in items if m["id"] == str(posted)] == ["Follow-up report"]


async def test_report_into_a_forum_opens_a_new_post(real: RealServer, tmp_path: Path) -> None:
    forum = await _forum(real)
    async with rig(tmp_path, fake=real, consumer=consumer()) as r:
        posted = await r.handler.on_report(str(forum), None, "# Ticket 42 failed\n\nDetails")
    assert posted is not None
    threads = ok(await real.call("listChannelThreads", path={"channel_id": forum}))["items"]
    titles = [t["title"] for t in threads]
    assert titles == ["Ticket 42 failed"]
    items = await messages(real, UUID(threads[0]["id"]))
    assert [m["content_md"] for m in items] == ["# Ticket 42 failed\n\nDetails"]
    assert _no_run(items[0])


# ---- files out, voice out, voice in -----------------------------------------------------------


async def test_files_and_voice_go_out_with_the_answer(real: RealServer, tmp_path: Path) -> None:
    report = tmp_path / "report.txt"
    report.write_text("line one\n", encoding="utf-8")
    voice = tmp_path / "answer.mp3"
    voice.write_bytes(b"ID3\x04\x00\x00\x00\x00\x00\x00" + bytes(512))
    sent: list[Any] = []

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        await rt.events.on_answer(turn.thread_id, "Here are the files.", str(turn.message_id))
        sent.append(
            await rt.events.on_file(
                turn.thread_id, str(report), MediaKind.DOCUMENT, "The report", None
            )
        )
        sent.append(await rt.events.on_file(turn.thread_id, str(voice), MediaKind.VOICE, "", None))
        return TurnOutcome.COMPLETED

    async with rig(tmp_path, fake=real, consumer=consumer(), runtime=FakeRuntime(script=script)):
        first = await real.trigger("Send me the report @lotse")
        view = await ended(real, first.run_id)
    assert view.state == "completed"
    assert [s.error for s in sent] == [None, None]
    answers = agent_texts(await messages(real, first.thread_id), real.agent_id)
    files = [
        (a["content_md"], [f["file"]["original_name"] for f in a["attachments"]]) for a in answers
    ]
    assert files == [
        ("Here are the files.", []),
        ("The report", ["report.txt"]),
        ("", ["answer.mp3"]),
    ]
    assert all(a["kind"] == "text" for a in answers)


async def test_voice_message_is_transcribed_and_the_transcript_submitted(
    real: RealServer, tmp_path: Path
) -> None:
    audio = m4a()
    runtime = FakeRuntime(transcripts={audio: "Please check the export"})
    async with rig(tmp_path, fake=real, consumer=consumer(), runtime=runtime) as r:
        first = await real.trigger("Hello @lotse")
        await ended(real, first.run_id)
        uploaded: list[UUID] = []

        async def voice_run() -> Any:
            # A voice message needs a final file (409 file_not_ready before the scan ended) and
            # the effective feature voice_input, which the adapter's heartbeat reports; a voice
            # message posted before that triggers nothing and keeps its file, so the next try
            # uploads a new one.
            if not uploaded:
                uploaded.append(
                    await real.upload(real.world.channel_id, "note.m4a", audio, "audio/mp4")
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
            got = ok(await real.call("getMessage", path={"message_id": answer.body["id"]}))
            if got.get("trigger_state") in (None, "failed"):
                uploaded.clear()
                return None
            return answer.body["id"]

        message = UUID(await until("an accepted voice message", voice_run))
        run_id, _ = await real.wait_run(message)
        view = await ended(real, run_id)
    assert view.state == "completed"
    assert [h[1] for h in runtime.heard] == ["note.m4a"]
    assert runtime.heard[0][2].startswith("audio/")
    assert "Please check the export" in r.runtime.turns[-1].user_text
    file = ok(await real.call("getFile", path={"file_id": uploaded[-1]}))
    assert (file["transcript_status"], file["transcript_source"]) == ("done", "agent")


# ---- commands ---------------------------------------------------------------------------------


async def _command(real: RealServer, name: str) -> UUID:
    async def find() -> Any:
        listed = ok(
            await real.call("listChannelCommands", path={"channel_id": real.world.channel_id})
        )
        return next((c["id"] for c in listed["items"] if c["name"] == name), None)

    return UUID(await until(f"the command /{name}", find))


async def test_control_command_replies_without_a_run(real: RealServer, tmp_path: Path) -> None:
    runtime = FakeRuntime(replies=["Hermes Agent v0.22.5"])
    async with rig(
        tmp_path, fake=real, consumer=consumer(), runtime=runtime, commands=declared_commands({})
    ):
        first = await real.trigger("Hello @lotse")
        await ended(real, first.run_id)
        command = await _command(real, "version")
        ok(
            await real.call(
                "invokeCommand",
                path={"channel_id": real.world.channel_id, "command_id": command},
                body={"thread_id": str(first.thread_id), "options": []},
            ),
            202,
        )

        async def reply() -> Any:
            items = agent_texts(await messages(real, first.thread_id), real.agent_id)
            return [m for m in items if m["content_md"] == "Hermes Agent v0.22.5"]

        replies = await until("the reply of /version", reply)
    assert [c.text for c in runtime.commands] == ["/version"]
    assert _no_run(replies[0])


async def test_skill_command_runs_a_turn_without_a_quote(real: RealServer, tmp_path: Path) -> None:
    runtime = FakeRuntime(skills={"summarize": "Summarize the thread"})
    async with rig(
        tmp_path,
        fake=real,
        consumer=consumer(),
        runtime=runtime,
        commands=declared_commands(runtime.skills),
    ):
        first = await real.trigger("Hello @lotse")
        await ended(real, first.run_id)
        command = await _command(real, "summarize")
        invoked = ok(
            await real.call(
                "invokeCommand",
                path={"channel_id": real.world.channel_id, "command_id": command},
                body={
                    "thread_id": str(first.thread_id),
                    "options": [{"name": "input", "value": "briefly"}],
                },
            ),
            202,
        )
        view = await ended(real, UUID(invoked["run_id"]))
    assert view.state == "completed"
    assert runtime.turns[-1].user_text == "[skill summarize] briefly"
    answers = agent_texts(await messages(real, first.thread_id), real.agent_id)
    assert answers[-1]["content_md"] == "Done so far."
    assert answers[-1].get("reply_to_message_id") is None


# ---- cards, follow-ups, edits and the title ---------------------------------------------------


async def test_card_answered_by_a_click(real: RealServer, tmp_path: Path) -> None:
    question = CardRequest(CardKind.CLARIFY, "c1", "Which file?", ("a.txt", "b.txt"))
    runtime = FakeRuntime(script=card_script(question))
    async with rig(tmp_path, fake=real, consumer=consumer(), runtime=runtime):
        first = await real.trigger("Which file do you mean @lotse")

        async def card() -> Any:
            for m in agent_texts(await messages(real, first.thread_id), real.agent_id):
                if m["kind"] == "card":
                    return m
            return None

        shown = await until("the card", card)
        assert (await real.run_state(first.run_id)).state == "waiting_input"
        button = next(c for c in shown["components"] if c["custom_id"].endswith(":1"))
        ok(
            await real.call(
                "createInteraction",
                path={"component_id": button["id"]},
                body={"client_interaction_id": str(uuid7()), "values": {}},
            ),
            201,
            202,
        )
        view = await ended(real, first.run_id)
    assert view.state == "completed"
    assert runtime.resolved == [("cl:c1:1", "b.txt")]
    answers = agent_texts(await messages(real, first.thread_id), real.agent_id)
    assert answers[-1]["content_md"] == "You chose b.txt."


async def test_followup_steers_the_running_turn(real: RealServer, tmp_path: Path) -> None:
    async def waits_for_steer(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.events.on_status(turn.thread_id, "Reading the logs")
        rt.events.on_model_reply(turn.thread_id, 1)  # a tool call: the turn takes a steer
        while not rt.steered:
            await asyncio.sleep(0.05)
        rt.events.on_status(turn.thread_id, None)
        await rt.events.on_answer(turn.thread_id, "Both checked.", str(rt.steered[0][2]))
        return TurnOutcome.COMPLETED

    runtime = FakeRuntime(script=waits_for_steer)
    async with rig(tmp_path, fake=real, consumer=consumer(), runtime=runtime):
        # The invocation carries the effective features of its time: the adapter's capability
        # followups = steer reaches the server with its first heartbeat.
        await heartbeat(real)
        first = await real.trigger("Check the export @lotse")

        async def running() -> bool:
            return (await real.run_state(first.run_id)).state == "running"

        await until("the running run", running)
        followup = await post(real, first.thread_id, "Also the import, please")
        view = await ended(real, first.run_id)
    assert view.state == "completed"
    assert [(t, text, m) for t, text, m in runtime.steered] == [
        (first.thread_id, "Also the import, please", followup)
    ]
    answers = agent_texts(await messages(real, first.thread_id), real.agent_id)
    assert [a["content_md"] for a in answers] == ["Both checked."]
    # The steered follow-up lies in the run's thread, so the answer may quote it.
    assert answers[0]["reply_to_message_id"] == str(followup)


async def test_edit_of_the_answer_and_the_title(real: RealServer, tmp_path: Path) -> None:
    async def edits(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        key = await rt.events.on_answer(turn.thread_id, "Draft answer", str(turn.message_id))
        assert key is not None
        assert await rt.events.on_edit(turn.thread_id, key, "Final answer", True)
        rt.titles[turn.thread_id] = "Export failures"
        return TurnOutcome.COMPLETED

    async with rig(tmp_path, fake=real, consumer=consumer(), runtime=FakeRuntime(script=edits)):
        message = await post(real, real.world.main_thread_id, "@lotse why does the export fail")
        run_id, thread_id = await real.wait_run(message)
        assert (await ended(real, run_id)).state == "completed"

        async def titled() -> Any:
            thread = ok(await real.call("getThread", path={"thread_id": thread_id}))
            return thread if thread.get("title") == "Export failures" else None

        await until("the title of the auto thread", titled)
    answers = agent_texts(await messages(real, thread_id), real.agent_id)
    assert [a["content_md"] for a in answers] == ["Final answer"]
    assert answers[0]["edited_at"] is not None


# ---- cancel and retry -------------------------------------------------------------------------


async def test_cancel_stops_the_turn(real: RealServer, tmp_path: Path) -> None:
    async def held(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.events.on_status(turn.thread_id, "Thinking")
        await rt.stop_event(turn.thread_id).wait()
        return TurnOutcome.CANCELLED

    runtime = FakeRuntime(script=held)
    async with rig(tmp_path, fake=real, consumer=consumer(), runtime=runtime):
        first = await real.trigger("Take your time @lotse")

        async def running() -> bool:
            return (await real.run_state(first.run_id)).state == "running"

        await until("the running run", running)
        # "running" is set by startRun, before the SDK enters the handler: a cancel in between
        # stops the run without any Hermes turn to interrupt. Cancel once the turn runs in Hermes.
        await until("the turn in Hermes", _async(runtime.started.is_set))
        ok(
            await real.call("cancelRun", path={"run_id": first.run_id}, body={"reason": "test"}),
            202,
        )
        view = await ended(real, first.run_id)
        await until("the interrupt in Hermes", _async(lambda: runtime.interrupted))
    assert view.state == "cancelled"
    assert runtime.interrupted == [first.thread_id]
    assert agent_texts(await messages(real, first.thread_id), real.agent_id) == []


async def test_retry_of_a_failed_run_in_the_auto_thread(real: RealServer, tmp_path: Path) -> None:
    attempts: list[UUID] = []

    async def flaky(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        attempts.append(turn.run_id)
        if len(attempts) == 1:
            return TurnOutcome.FAILED
        await rt.events.on_answer(turn.thread_id, "Second time lucky.", str(turn.message_id))
        return TurnOutcome.COMPLETED

    async with rig(tmp_path, fake=real, consumer=consumer(), runtime=FakeRuntime(script=flaky)):
        message = await post(real, real.world.main_thread_id, "@lotse try the import")
        run_id, thread_id = await real.wait_run(message)
        assert (await ended(real, run_id)).state == "failed"
        retried = ok(await real.call("retryRun", path={"run_id": run_id}), 202)
        second = UUID(retried["id"])
        assert second != run_id
        view = await ended(real, second)
    assert view.state == "completed"
    assert attempts == [run_id, second]
    answers = agent_texts(await messages(real, thread_id), real.agent_id)
    assert [a["content_md"] for a in answers] == ["Second time lucky."]
    assert answers[0].get("reply_to_message_id") is None


# ---- the recovery path of Hermes' delivery ledger ---------------------------------------------


async def test_a_recovered_copy_of_an_answer_is_not_posted_again(
    real: RealServer, tmp_path: Path
) -> None:
    notice = (
        chr(0x267B)
        + chr(0xFE0F)
        + " Recovered reply "
        + chr(0x2014)
        + " the gateway restarted during delivery, so this may be a duplicate:\n\n"
    )
    async with rig(tmp_path, fake=real, consumer=consumer()) as r:
        message = await post(real, real.world.main_thread_id, "@lotse once only please")
        run_id, thread_id = await real.wait_run(message)
        await ended(real, run_id)
        assert await r.handler.recovered_duplicate(thread_id, notice + "Done so far.")
    answers = agent_texts(await messages(real, thread_id), real.agent_id)
    assert [a["content_md"] for a in answers] == ["Done so far."]


def _async(check: Any) -> Any:
    async def run() -> Any:
        await asyncio.sleep(0)
        return check()

    return run
