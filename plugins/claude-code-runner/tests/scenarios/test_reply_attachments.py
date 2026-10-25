# SPDX-License-Identifier: Apache-2.0
"""Answer attachments of the runner (09 18.10, brief M3-12 Tests "Scenarios", plan row M3-12).

The real runner with real Fake-Claude processes and the FakeServer of M2-29. The FakeServer
answers ``getInstance`` with ``message_max_attachments`` 3 and a small
``file_max_upload_bytes``, and its ``part_size_bytes`` is small enough that one file needs
three parts. Every scenario checks that no request body, header or log line holds the content
of a file, except the ``uploadPart`` bodies of an accepted file.
"""

from __future__ import annotations

import hashlib
import json
import os
import stat
import time
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.faults import Fault, HoldGate

from .conftest import LogLines, World

pytestmark = pytest.mark.anyio

PART = 8
MAX_BYTES = 64
REPORT = b"report-canary-content-01"  # 24 bytes: three parts of 8
SUMMARY = b"summary-canary-text-0002"
NOTES = b"notes-canary-content-003"
SECRET = b"SECRET-CANARY-do-not-send"


@pytest.fixture
def world_timings() -> dict[str, float]:
    # The server never ends a run on its own timers inside a scenario.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def _limits(world: World) -> None:
    uploads = world.fake.uploads
    uploads.limits["message_max_attachments"] = 3
    uploads.limits["file_max_upload_bytes"] = MAX_BYTES
    uploads.part_size = PART


def _project(world: World) -> Path:
    return world.projects / "website"


def _write(base: Path, files: dict[str, bytes]) -> None:
    for rel, data in files.items():
        path = base / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)


async def _answer(
    world: World,
    text: str,
    files: dict[str, bytes] | None = None,
    **scenario: Any,
) -> UUID:
    """Start the runner, let Fake-Claude answer ``text`` and trigger one run in Projekt-A."""
    _limits(world)
    _write(_project(world), files or {})
    world.scenario(fixture="plain", result_override={"result": text}, **scenario)
    await world.start()
    run = await world.fake.trigger("Send me the report", channel_id=world.forum_a)
    return run.run_id


def _bodies(world: World, op: str) -> list[dict[str, Any]]:
    return [r.json for r in world.requests(op)]


def _no_leak(world: World, logs: LogLines | None, *needles: bytes, sent: tuple[bytes, ...] = ()):
    """No request (except the parts of ``sent``) and no log line holds a needle."""
    for request in world.fake.requests:
        blob = json.dumps(request.json) + json.dumps(dict(request.headers))
        for needle in needles:
            assert needle.decode() not in blob, request.operation_id
        if request.content is not None:
            if any(request.content in data for data in sent):
                continue
            for needle in needles:
                assert needle not in request.content, request.operation_id
    if logs is not None:
        for needle in needles:
            assert needle.decode() not in logs.text


def _notice_lines(text: str) -> list[str]:
    _, _, notice = text.partition("Attachments not sent:\n")
    return notice.splitlines()


def _token(world: World) -> str:
    header: str = world.requests("getRunContext")[0].headers["invocation-token"]
    return header


async def test_reply_with_attachments_uploaded_and_attached(world: World, logs: LogLines) -> None:
    gate = world.gate("turn_gate")
    _limits(world)
    _write(_project(world), {"dist/report.md": REPORT})
    world.scenario(fixture="attach_two", turn_gate=str(gate))
    await world.start()
    run = await world.fake.trigger("Send me the report", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    tmp = world.state / "tmp" / str(run.run_id)
    (turn,) = world.turns()
    assert turn["env"]["TMPDIR"] == str(tmp)
    assert stat.S_IMODE(tmp.stat().st_mode) == 0o700
    (tmp / "summary.txt").write_bytes(SUMMARY)  # scratch output of the turn
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"

    assert len(world.requests("getInstance")) == 1
    creates = _bodies(world, "createUpload")
    assert creates == [
        {
            "name": "report.md",
            "size_bytes": len(REPORT),
            "sha256": hashlib.sha256(REPORT).hexdigest(),
        },
        {
            "name": "summary.txt",
            "size_bytes": len(SUMMARY),
            "sha256": hashlib.sha256(SUMMARY).hexdigest(),
        },
    ]
    keys = [r.headers["idempotency-key"] for r in world.requests("createUpload")]
    keys += [r.headers["idempotency-key"] for r in world.requests("completeUpload")]
    assert len(set(keys)) == 4
    parts = world.requests("uploadPart")
    assert [r.path.rsplit("/", 1)[-1] for r in parts] == ["1", "2", "3"] * 2
    assert b"".join(r.content or b"" for r in parts) == REPORT + SUMMARY
    sessions = list(world.fake.uploads.sessions.values())
    assert [s.content() for s in sessions] == [REPORT, SUMMARY]
    completes = _bodies(world, "completeUpload")
    assert [len(c["parts"]) for c in completes] == [3, 3]

    (message,) = world.fake.messages
    assert message["content_md"] == "Done."
    assert message["run_id"] == str(run.run_id)
    assert message["attachment_file_ids"] == [str(s.file_id) for s in sessions]
    assert all(s.attached is not None for s in sessions)
    token = _token(world)
    for op in (
        "getInstance",
        "createUpload",
        "uploadPart",
        "completeUpload",
        "createMessage",
    ):
        assert all(r.headers.get("invocation-token") == token for r in world.requests(op)), op
    lines = [
        op
        for body in _bodies(world, "patchRun")
        for op in body["ops"]
        if op.get("path") == "/status_line"
    ]
    assert lines == [
        {"op": "replace", "path": "/status_line", "value": "uploading_attachments"},
        {"op": "remove", "path": "/status_line"},
    ]
    # Removed when the run ends, right after its finish.
    await world.until(lambda: not tmp.exists())
    await world.until(lambda: not (world.state / "outgoing" / str(run.run_id)).exists())
    (finish,) = _bodies(world, "finishRun")
    assert finish["state"] == "completed"
    _no_leak(world, logs, REPORT, SUMMARY, sent=(REPORT, SUMMARY))


async def test_symlink_escape_rejected_nothing_uploaded(world: World, logs: LogLines) -> None:
    outside = world.root / "home-of-alex" / "secret.txt"
    outside.parent.mkdir()
    outside.write_bytes(SECRET)
    (_project(world) / "link.txt").symlink_to(outside)
    run_id = await _answer(world, "Done.\nATTACH: notes.md\nATTACH: link.txt", {"notes.md": NOTES})
    assert await world.ended(run_id) == "completed"
    assert world.requests("createUpload") == []
    (message,) = world.fake.messages
    assert "attachment_file_ids" not in _bodies(world, "createMessage")[0]
    assert message["content_md"].startswith("Done.\n\nAttachments not sent:\n")
    assert _notice_lines(message["content_md"]) == [
        "- `notes.md`: set_not_sent",
        "- `link.txt`: outside_allowed_roots",
    ]
    _no_leak(world, logs, SECRET, NOTES, str(outside.parent).encode())


async def test_secret_patterns_rejected_nothing_uploaded(world: World, logs: LogLines) -> None:
    files = {"README.md": b"read me", ".env": SECRET, ".ssh/id_ed25519": SECRET}
    run_id = await _answer(world, "Here.\nATTACH: README.md\nATTACH: .env", files)
    assert await world.ended(run_id) == "completed"
    world.scenario(
        fixture="plain", result_override={"result": "Key.\nATTACH: config/../.ssh/id_ed25519"}
    )
    (_project(world) / "config").mkdir()
    second = await world.fake.trigger("And the key", channel_id=world.forum_a)
    assert await world.ended(second.run_id) == "completed"
    assert world.requests("createUpload") == []
    first, again = world.fake.messages
    assert _notice_lines(first["content_md"]) == [
        "- `README.md`: set_not_sent",
        "- `.env`: denied_pattern",
    ]
    assert _notice_lines(again["content_md"]) == ["- `id_ed25519`: denied_pattern"]
    _no_leak(world, logs, SECRET)


async def test_all_or_none_upload_failure_attaches_none(world: World, logs: LogLines) -> None:
    files = {"a.md": REPORT, "b.md": SUMMARY, "c.md": NOTES}
    world.fake.inject("createUpload", Fault.passthrough())
    world.fake.inject("createUpload", Fault.problem("conflict", reason="quota_exceeded"))
    run_id = await _answer(world, "Three.\nATTACH: a.md\nATTACH: b.md\nATTACH: c.md", files)
    assert await world.ended(run_id) == "completed"
    assert [b["name"] for b in _bodies(world, "createUpload")] == ["a.md", "b.md"]
    assert all("attachment_file_ids" not in b for b in _bodies(world, "createMessage"))
    (message,) = world.fake.messages
    assert _notice_lines(message["content_md"]) == [
        "- `a.md`: set_not_sent",
        "- `b.md`: upload_refused (conflict, quota_exceeded)",
        "- `c.md`: set_not_sent",
    ]
    # The completed first file is not aborted (it stays unattached for M3-08's cleanup).
    assert world.requests("abortUpload") == []
    _no_leak(world, logs, SUMMARY, NOTES, sent=(REPORT,))


async def test_all_or_none_transport_failure_aborts_open_session(
    world: World, logs: LogLines
) -> None:
    files = {"a.md": REPORT, "b.md": SUMMARY, "c.md": NOTES}
    # Parts 1 to 3 of the first file and 1 and 2 of the second go through; the third part of
    # the second file loses its connection on every attempt.
    for _ in range(5):
        world.fake.inject("uploadPart", Fault.passthrough())
    world.fake.inject(
        "uploadPart", Fault.disconnect(), times=world.fake.sdk_settings.retry_max_attempts + 1
    )
    run_id = await _answer(world, "Three.\nATTACH: a.md\nATTACH: b.md\nATTACH: c.md", files)
    assert await world.ended(run_id) == "completed"
    assert [b["name"] for b in _bodies(world, "createUpload")] == ["a.md", "b.md"]
    sessions = list(world.fake.uploads.sessions.values())
    aborted = [r.path.rsplit("/", 1)[-1] for r in world.requests("abortUpload")]
    assert aborted == [str(sessions[1].file_id)]
    assert sessions[0].state == "complete"
    assert sessions[1].state == "aborted"
    (message,) = world.fake.messages
    assert _notice_lines(message["content_md"]) == [
        "- `a.md`: set_not_sent",
        "- `b.md`: upload_failed",
        "- `c.md`: set_not_sent",
    ]
    _no_leak(world, logs, NOTES, sent=(REPORT, SUMMARY))


async def test_all_or_none_limits_from_instance(world: World) -> None:
    files = {f"{n}.md": REPORT for n in "abcd"}
    files["big.bin"] = b"x" * (MAX_BYTES + 1)
    four = "\n".join(f"ATTACH: {n}.md" for n in "abcd")
    run_id = await _answer(world, f"Four.\n{four}", files)
    assert await world.ended(run_id) == "completed"
    assert _notice_lines(world.fake.messages[0]["content_md"]) == [
        "- `a.md`: too_many",
        "- `b.md`: too_many",
        "- `c.md`: too_many",
        "- and 1 more",
    ]
    world.scenario(fixture="plain", result_override={"result": "Big.\nATTACH: big.bin"})
    second = await world.fake.trigger("Big", channel_id=world.forum_a)
    assert await world.ended(second.run_id) == "completed"
    assert _notice_lines(world.fake.messages[1]["content_md"]) == ["- `big.bin`: too_large"]
    world.fake.inject("getInstance", Fault.problem("unavailable"), times=10)
    world.scenario(fixture="plain", result_override={"result": "Again.\nATTACH: a.md"})
    third = await world.fake.trigger("Again", channel_id=world.forum_a)
    assert await world.ended(third.run_id) == "completed"
    assert _notice_lines(world.fake.messages[2]["content_md"]) == ["- `a.md`: limits_unavailable"]
    assert world.requests("createUpload") == []


async def test_acting_for_insufficient_refuses_upload(world: World) -> None:
    world.fake.inject("createUpload", Fault.problem("forbidden", reason="acting_for_insufficient"))
    run_id = await _answer(world, "Done.\nATTACH: dist/report.md", {"dist/report.md": REPORT})
    assert await world.ended(run_id) == "completed"
    assert world.requests("uploadPart") == []
    (message,) = world.fake.messages
    assert _notice_lines(message["content_md"]) == [
        "- `report.md`: upload_refused (forbidden, acting_for_insufficient)"
    ]


@pytest.mark.parametrize(
    ("code", "reason"), [("feature_disabled", None), ("conflict", "file_not_attachable")]
)
async def test_message_refused_falls_back_without_attachments(
    world: World, code: str, reason: str | None
) -> None:
    world.fake.inject("createMessage", Fault.problem(code, reason=reason))
    run_id = await _answer(world, "Done.\nATTACH: dist/report.md", {"dist/report.md": REPORT})
    assert await world.ended(run_id) == "completed"
    first, second = _bodies(world, "createMessage")
    assert first["attachment_file_ids"]
    assert "attachment_file_ids" not in second
    assert first["client_msg_id"] != second["client_msg_id"]
    (message,) = world.fake.messages
    detail = f"{code}, {reason}" if reason else code
    assert _notice_lines(message["content_md"]) == [f"- `report.md`: not_attached ({detail})"]
    assert "attachment_file_ids" not in message
    assert all(s.attached is None for s in world.fake.uploads.sessions.values())


async def test_message_stored_then_502_attaches_once(world: World) -> None:
    world.fake.inject("createMessage", Fault.stored_then_502())
    run_id = await _answer(world, "Done.\nATTACH: dist/report.md", {"dist/report.md": REPORT})
    assert await world.ended(run_id) == "completed"
    bodies = _bodies(world, "createMessage")
    assert len(bodies) >= 2
    assert len({b["client_msg_id"] for b in bodies}) == 1
    (message,) = world.fake.messages
    assert len(message["attachment_file_ids"]) == 1


async def test_attach_only_answer_sends_empty_text(world: World) -> None:
    _limits(world)
    _write(_project(world), {"dist/report.md": REPORT})
    world.scenario(fixture="attach_only")
    await world.start()
    run = await world.fake.trigger("Only the file", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    (message,) = world.fake.messages
    assert message["content_md"] == ""
    assert len(message["attachment_file_ids"]) == 1
    # Without any ATTACH line an empty answer still sends nothing (M2-34).
    world.scenario(fixture="plain", result_override={"result": "  "})
    second = await world.fake.trigger("Nothing", channel_id=world.forum_a)
    assert await world.ended(second.run_id) == "completed"
    assert len(world.fake.messages) == 1


async def test_attach_lines_in_code_block_stay_text(world: World) -> None:
    _limits(world)
    world.scenario(fixture="attach_code_block")
    await world.start()
    run = await world.fake.trigger("How do I attach?", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert world.requests("getInstance") == []
    assert world.requests("createUpload") == []
    (message,) = world.fake.messages
    assert message["content_md"] == "Use this syntax:\n\n```\nATTACH: example.txt\n```"


async def test_thread_inbox_file_reattached(world: World) -> None:
    _limits(world)
    file_id = world.fake.add_file(REPORT, name="input.md", mime="text/markdown")
    other = world.state / "files" / "0192d3a0-0000-7000-8000-00000000ffff"
    world.config()  # creates the state directory
    other.mkdir(parents=True)
    (other / "x.md").write_bytes(SECRET)
    world.scenario(
        fixture="plain",
        result_override={"result": f"Back.\nATTACH: {{{{add_dir}}}}/{file_id}"},
    )
    await world.start()
    run = await world.fake.trigger("Send it back", channel_id=world.forum_a, attachments=[file_id])
    assert await world.ended(run.run_id) == "completed"
    (message,) = world.fake.messages
    (attached,) = message["attachment_file_ids"]
    assert attached != str(file_id)  # a new file
    assert world.fake.uploads.sessions[UUID(attached)].content() == REPORT
    world.scenario(fixture="plain", result_override={"result": f"Other.\nATTACH: {other}/x.md"})
    second = await world.fake.trigger("Other", channel_id=world.forum_a)
    assert await world.ended(second.run_id) == "completed"
    assert _notice_lines(world.fake.messages[1]["content_md"]) == [
        "- `x.md`: outside_allowed_roots"
    ]
    _no_leak(world, None, SECRET, sent=(REPORT,))


async def test_followup_results_each_answer_own_attachments(world: World) -> None:
    run_id = await _answer(
        world,
        "First.\nATTACH: a.md",
        {"a.md": REPORT, "b.md": SUMMARY},
        meta_result=True,
        meta_result_text="Second.\nATTACH: b.md",
    )
    assert await world.ended(run_id) == "completed"
    first, second = world.fake.messages
    assert (first["content_md"], second["content_md"]) == ("First.", "Second.")
    contents = [
        world.fake.uploads.sessions[UUID(m["attachment_file_ids"][0])].content()
        for m in (first, second)
    ]
    assert contents == [REPORT, SUMMARY]
    assert len(world.requests("getInstance")) == 2


async def test_stop_during_upload_sends_nothing(world: World) -> None:
    gate = HoldGate()
    world.fake.inject("uploadPart", Fault.passthrough())
    world.fake.inject("uploadPart", Fault.hold(gate))
    run_id = await _answer(world, "Done.\nATTACH: dist/report.md", {"dist/report.md": REPORT})
    await gate.reached.wait()
    await world.fake.cancel(run_id, "user_cancelled")
    assert await world.ended(run_id) == "cancelled"
    gate.open()
    assert len(world.requests("uploadPart")) == 2
    assert world.requests("completeUpload") == []
    (session,) = world.fake.uploads.sessions.values()
    assert session.state == "aborted"
    assert len(world.requests("abortUpload")) == 1
    assert world.fake.messages == []
    (finish,) = _bodies(world, "finishRun")
    assert (finish["state"], finish["reason"]) == ("cancelled", "user_cancelled")


async def test_run_ended_on_server_during_upload(world: World) -> None:
    world.fake.inject(
        "uploadPart", Fault.problem("invocation_token_invalid", reason="run_inactive")
    )
    run_id = await _answer(world, "Done.\nATTACH: dist/report.md", {"dist/report.md": REPORT})
    await world.until(lambda: any(c["kind"] == "turn_end" for c in world.calls("turn_end")))
    (end,) = world.calls("turn_end")
    assert end.get("signal") == "SIGINT"
    await world.until(lambda: not (world.state / "turns" / f"{run_id}.json").exists())
    assert world.fake.messages == []
    assert world.requests("finishRun") == []


async def test_heartbeat_reports_files_out(world: World) -> None:
    await world.start()
    beats = [h for h in world.fake.heartbeats if h.get("details")]
    assert beats
    assert all(h["details"]["capabilities"]["files_out"] is True for h in beats)


async def test_no_names_paths_or_content_in_logs(world: World, logs: LogLines) -> None:
    name, folder, content = "canary-name-77.txt", "canary-dir-88", b"canary-body-99-xyz"
    run_id = await _answer(world, f"Done.\nATTACH: {folder}/{name}", {f"{folder}/{name}": content})
    assert await world.ended(run_id) == "completed"
    for needle in (name, folder, content.decode()):
        assert needle not in logs.text
        for body in _bodies(world, "patchRun") + _bodies(world, "finishRun"):
            assert needle not in json.dumps(body)
    assert _bodies(world, "createUpload")[0]["name"] == name
    assert folder not in json.dumps(_bodies(world, "createUpload"))
    (message,) = world.fake.messages
    assert name not in message["content_md"]
    # The notice of a failed set names the base name only, never the folder.
    world.fake.inject("createUpload", Fault.problem("payload_too_large"))
    world.scenario(fixture="plain", result_override={"result": f"Again.\nATTACH: {folder}/{name}"})
    second = await world.fake.trigger("Again", channel_id=world.forum_a)
    assert await world.ended(second.run_id) == "completed"
    notice = world.fake.messages[1]["content_md"]
    assert name in notice
    assert folder not in notice
    assert str(world.root) not in notice
    assert folder not in logs.text


async def test_state_files_private_and_retention(world: World) -> None:
    gate = HoldGate()
    world.fake.inject("createMessage", Fault.hold(gate))
    _limits(world)
    _write(_project(world), {"dist/report.md": REPORT})
    world.scenario(fixture="plain", result_override={"result": "Done.\nATTACH: dist/report.md"})
    await world.start(world.config(stream_log_retention="1h"))
    run = await world.fake.trigger("Send", channel_id=world.forum_a)
    await gate.reached.wait()
    staged = world.state / "outgoing" / str(run.run_id)
    records = list((world.state / "sessions").rglob("*.attach-1.json"))
    assert len(records) == 1
    for path in [*staged.rglob("*"), staged, staged.parent, *records]:
        mode = stat.S_IMODE(path.lstat().st_mode)
        assert mode == (0o700 if path.is_dir() else 0o600), path
    assert [p.read_bytes() for p in staged.rglob("*") if p.is_file()] == [REPORT]
    gate.open()
    assert await world.ended(run.run_id) == "completed"
    assert not staged.exists()
    assert not records[0].exists()
    # Leftovers of a crash fall to the next sweep.
    past = time.time() - 7200
    leftovers = [
        world.state / "outgoing" / "0192d3a0-0000-7000-8000-00000000eeee" / "1" / "1",
        world.state / "tmp" / "0192d3a0-0000-7000-8000-00000000eeee" / "scratch.txt",
    ]
    for path in leftovers:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(b"x")
    for path in leftovers:
        for item in (path, path.parent, path.parent.parent):
            os.utime(item, (past, past))
    world.scenario(fixture="plain")
    second = await world.fake.trigger("Two", channel_id=world.forum_a)
    assert await world.ended(second.run_id) == "completed"
    await world.until(lambda: not any(p.exists() for p in leftovers))
    assert not (world.state / "tmp" / "0192d3a0-0000-7000-8000-00000000eeee").exists()
