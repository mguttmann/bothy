# SPDX-License-Identifier: Apache-2.0
"""The mirror of Hermes sessions outside the platform (09 17.13, design note hermes-session-mirror).

A foreign session is simulated by calling the plugin hooks the way Hermes calls them for a
session of ``acp``, ``cli`` or ``cron``; the server is the SDK's FakeServer. The mirror's worker
runs on its own thread and event loop, so every check waits for the worker with `Mirror.drain`
while the FakeServer keeps serving on the test's loop.
"""

from __future__ import annotations

import asyncio
import socket
import time
from collections.abc import Iterator
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from adapter_kit import FakeRuntime, Rig, captured_logs, rig, settings_for
from bothy_sdk.ids import uuid7
from bothy_sdk.testing.fakeserver import FakeServer
from bothy_sdk.testing.faults import Fault

from bothy_hermes_adapter import entry, mirror
from bothy_hermes_adapter.config import ConfigError, load_settings
from bothy_hermes_adapter.hermes_commands import declared_commands
from bothy_hermes_adapter.mirror import (
    MIRROR_AGENT,
    MIRROR_READ_ONLY,
    Mirror,
    MirrorConfig,
    MirrorMap,
    mirror_config,
)

pytestmark = pytest.mark.anyio

INSTANCE = "0192f4a0-7b1c-7000-8000-000000000001"


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture(autouse=True)
def _no_process_mirror() -> Iterator[None]:
    yield
    mirror.reset_process_mirrors()


def _base(**extra: Any) -> dict[str, Any]:
    return {
        "hermes_adapter.server_url": "https://example.com/api/v1",
        "hermes_adapter.instance_id": INSTANCE,
        "hermes_adapter.token_file": "/srv/hermes/token",
        **extra,
    }


async def _drain(fake: FakeServer, m: Mirror) -> None:
    """Let the worker post everything queued while the FakeServer serves on this loop."""
    deadline = time.monotonic() + 10
    while not m.idle():
        if time.monotonic() > deadline:
            raise AssertionError("the mirror did not drain")
        await asyncio.sleep(0.02)
    del fake


def _turn(
    m: Mirror,
    session: str,
    prompt: Any,
    answer: str | None,
    *,
    platform: str = "acp",
    tools: tuple[str, ...] = (),
    parent: str = "",
    failed: bool = False,
) -> None:
    """One turn of a foreign session, hook by hook, as Hermes fires them."""
    m.hook(
        "pre_llm_call",
        {
            "session_id": session,
            "parent_session_id": parent,
            "user_message": prompt,
            "platform": platform,
            "is_first_turn": True,
        },
    )
    for name in tools:
        m.hook("post_tool_call", {"session_id": session, "tool_name": name, "status": "ok"})
    if answer is not None:
        m.hook(
            "post_llm_call",
            {"session_id": session, "assistant_response": answer, "platform": platform},
        )
    m.hook(
        "on_session_end",
        {
            "session_id": session,
            "completed": not failed,
            "failed": failed,
            "interrupted": False,
            "platform": platform,
        },
    )


def _mirror(
    fake: FakeServer,
    root: Path,
    channel: UUID,
    *,
    platforms: tuple[str, ...] = ("acp", "cli"),
    **kw: Any,
) -> Mirror:
    root.mkdir(parents=True, exist_ok=True)
    settings = settings_for(fake, root, **kw.pop("settings", {}))
    config = MirrorConfig(channel, frozenset(platforms))
    return Mirror(settings, config, MirrorMap(root / "state" / "mirror.sqlite3"), **kw)


def _thread_messages(fake: FakeServer, thread: UUID) -> list[str]:
    stored = fake.world.messages.values()
    return [m["content_md"] for m in stored if m["thread_id"] == str(thread)]


# ---- configuration --------------------------------------------------------------------------


def test_mirror_is_off_by_default_and_reads_both_keys() -> None:
    assert mirror_config(load_settings(_base())) is None
    channel = str(uuid7())
    settings = load_settings(
        _base(
            **{
                "hermes_adapter.mirror_channel": channel,
                "hermes_adapter.mirror_platforms": ["acp", " CLI "],
            }
        )
    )
    assert mirror_config(settings) == MirrorConfig(UUID(channel), frozenset({"acp", "cli"}))
    as_text = load_settings(
        _base(
            **{
                "hermes_adapter.mirror_channel": channel,
                "hermes_adapter.mirror_platforms": "acp, cron",
            }
        )
    )
    assert mirror_config(as_text) == MirrorConfig(UUID(channel), frozenset({"acp", "cron"}))


@pytest.mark.parametrize(
    ("extra", "message"),
    [
        (
            {"hermes_adapter.mirror_channel": "nope", "hermes_adapter.mirror_platforms": ["acp"]},
            "hermes_adapter.mirror_channel: must be a UUID",
        ),
        (
            {"hermes_adapter.mirror_channel": str(uuid7())},
            "hermes_adapter.mirror_platforms: required with hermes_adapter.mirror_channel",
        ),
        (
            {"hermes_adapter.mirror_platforms": ["acp"]},
            "hermes_adapter.mirror_channel: required with hermes_adapter.mirror_platforms",
        ),
        (
            {
                "hermes_adapter.mirror_channel": str(uuid7()),
                "hermes_adapter.mirror_platforms": ["bothy"],
            },
            "hermes_adapter.mirror_platforms: the adapter's own platform is never mirrored",
        ),
        (
            {
                "hermes_adapter.mirror_channel": str(uuid7()),
                "hermes_adapter.mirror_platforms": ["A b"],
            },
            "hermes_adapter.mirror_platforms: must be platform keys",
        ),
    ],
)
def test_a_half_or_wrong_mirror_configuration_is_refused(
    extra: dict[str, Any], message: str
) -> None:
    with pytest.raises(ConfigError) as caught:
        load_settings(_base(**extra))
    assert str(caught.value) == message


# ---- a foreign session with several rounds ---------------------------------------------------


async def test_a_foreign_session_becomes_one_forum_post(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        m = _mirror(fake, tmp_path, forum)
        try:
            _turn(
                m,
                "s1",
                "Fix the build\nIt fails on main.",
                "Fixed.",
                tools=("terminal",) * 2 + ("read_file",),
            )
            _turn(m, "s1", "Now run the tests", "All green.")
            _turn(m, "s1", "Thanks", "")
            await _drain(fake, m)
        finally:
            m.close(5)
        assert len(fake.created_threads) == 1
        post = fake.created_threads[0]
        meta = fake.thread(post)
        assert meta["channel_id"] == str(forum)
        assert meta["title"] == "acp: Fix the build"
        assert _thread_messages(fake, post) == [
            "**Request** (acp session)\n\n> Fix the build\n> It fails on main.",
            "Fixed.\n\nTools: terminal x2, read_file x1",
            "**Request** (acp session)\n\n> Now run the tests",
            "All green.",
            "**Request** (acp session)\n\n> Thanks",
            "The turn ended without a text answer.",
        ]
        assert all("run_id" not in msg for msg in fake.messages)
        agents = {
            q.headers["user-agent"].split("/")[0]
            for q in fake.requests
            if q.operation_id in ("createThread", "createMessage")
        }
        assert agents == {MIRROR_AGENT}


async def test_two_sessions_get_two_posts_and_other_platforms_none(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        m = _mirror(fake, tmp_path, forum)
        try:
            _turn(m, "a", "First task", "One.")
            _turn(m, "b", "Second task", "Two.", platform="cli")
            _turn(m, "c", "A cron job", "Three.", platform="cron")  # not listed
            _turn(m, "d", "Own platform", "Four.", platform="bothy")
            m.hook("post_tool_call", {"session_id": "x", "tool_name": "terminal"})  # unknown
            await _drain(fake, m)
        finally:
            m.close(5)
        titles = sorted(fake.thread(t)["title"] for t in fake.created_threads)
        assert titles == ["acp: First task", "cli: Second task"]
        assert not any(
            "Three" in msg["content_md"] or "Four" in msg["content_md"] for msg in fake.messages
        )


async def test_failed_turn_and_cut_texts_follow_the_send_path(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        m = _mirror(fake, tmp_path, forum, settings={"max_message_bytes": 120})
        try:
            prompt = [
                {"type": "text", "text": "Describe this picture"},
                {"type": "image_url", "image_url": {"url": "data:image/png;base64,AAAA"}},
            ]
            _turn(m, "s", prompt, None, failed=True)
            _turn(m, "s", "Long one", "y" * 500)
            await _drain(fake, m)
        finally:
            m.close(5)
        post = fake.created_threads[0]
        texts = _thread_messages(fake, post)
        assert texts[0] == (
            "**Request** (acp session)\n\n> Describe this picture\n\n(1 part not mirrored)"
        )
        assert texts[1] == "The turn ended without an answer (failed)."
        assert "data:image" not in "".join(texts)
        assert all(len(t.encode()) <= 120 for t in texts)
        assert texts[3].endswith("\n\n(cut)")


# ---- persistence: restart, resume and compression -------------------------------------------


async def test_the_post_survives_a_restart_a_resume_and_a_compression(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        first = _mirror(fake, tmp_path, forum)
        try:
            _turn(first, "s1", "Start", "Started.")
            await _drain(fake, first)
        finally:
            first.close(5)
        # A new process of the profile (a restart, an ACP session/load of the same session ID).
        second = _mirror(fake, tmp_path, forum)
        try:
            _turn(second, "s1", "Go on", "Going on.")
            # Hermes compressed the session into a new ID whose parent is the mapped one.
            _turn(second, "s1-c", "After compression", "Still here.", parent="s1")
            _turn(second, "s1-c", "Again", "Yes.")
            await _drain(fake, second)
        finally:
            second.close(5)
        assert len(fake.created_threads) == 1
        post = fake.created_threads[0]
        assert len(_thread_messages(fake, post)) == 8
        mapping = MirrorMap(tmp_path / "state" / "mirror.sqlite3")
        assert mapping.thread_for("s1") == post
        assert mapping.thread_for("s1-c") == post
        assert mapping.is_mirror(post)
        assert not mapping.is_mirror(uuid7())


async def test_a_deleted_post_is_forgotten_and_the_next_turn_opens_a_new_one(
    tmp_path: Path,
) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        m = _mirror(fake, tmp_path, forum)
        try:
            _turn(m, "s", "One", "1")
            await _drain(fake, m)
            fake.inject("createMessage", Fault.problem("not_found"))
            _turn(m, "s", "Two", None)  # the request is refused: the post is gone
            await _drain(fake, m)
            _turn(m, "s", "Three", "3")
            await _drain(fake, m)
        finally:
            m.close(5)
        assert len(fake.created_threads) == 2
        assert _thread_messages(fake, fake.created_threads[1])[0].endswith("> Three")


# ---- fail open -------------------------------------------------------------------------------


def _closed_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


async def test_an_unreachable_server_never_holds_the_session(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        url = f"http://127.0.0.1:{_closed_port()}/api/v1"
        with captured_logs() as logs:
            m = _mirror(fake, tmp_path, forum, settings={"server_url": url})
            try:
                started = time.monotonic()
                for i in range(3):
                    _turn(m, "s", f"Prompt {i} secret-content", f"Answer {i}")
                assert time.monotonic() - started < 0.5  # the hooks only queue
                await _drain(fake, m)
            finally:
                m.close(5)
        assert fake.created_threads == []
        assert m.dropped >= 6
        text = logs.text()
        assert "mirror: event dropped" in text
        assert "secret-content" not in text
        assert "Answer" not in text
        assert MirrorMap(tmp_path / "state" / "mirror.sqlite3").thread_for("s") is None


async def test_a_wrong_channel_turns_the_mirror_off_once(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        text_channel = fake.world.channel_id
        with captured_logs() as logs:
            m = _mirror(fake, tmp_path, text_channel)
            try:
                _turn(m, "s", "One", "1")
                _turn(m, "t", "Two", "2")
                await _drain(fake, m)
            finally:
                m.close(5)
        assert m.disabled
        assert fake.created_threads == []
        assert logs.text().count("mirror: turned off") == 1
        forum = fake.add_forum("mirror", None)
        fake.inject("createThread", Fault.problem("forbidden"))
        refused = _mirror(fake, tmp_path / "b", forum)
        try:
            _turn(refused, "s", "One", "1")
            _turn(refused, "s", "Two", "2")
            await _drain(fake, refused)
        finally:
            refused.close(5)
        assert refused.disabled
        assert len([q for q in fake.requests if q.operation_id == "createThread"]) == 1


async def test_a_full_queue_drops_and_counts(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        m = _mirror(fake, tmp_path, forum, queue_max=2, autostart=False)
        with captured_logs() as logs:
            _turn(m, "s", "One", "1")
            _turn(m, "s", "Two", "2")
        assert m.dropped == 2  # two turns are four events (request and reply each): two fit
        assert "mirror: queue full" in logs.text()
        m.start()
        try:
            await _drain(fake, m)
        finally:
            m.close(5)
        assert _thread_messages(fake, fake.created_threads[0]) == [
            "**Request** (acp session)\n\n> One",
            "1",
        ]


async def test_a_hook_never_raises(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        m = _mirror(fake, tmp_path, forum, autostart=False)
        m.hook("pre_llm_call", {"platform": "acp"})  # no session
        m.hook("pre_llm_call", {"platform": "acp", "session_id": object(), "user_message": 3})
        m.hook("post_llm_call", None)  # type: ignore[arg-type]
        m.close(1)


async def test_the_exit_flush_is_bounded(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        forum = fake.add_forum("mirror", None)
        m = _mirror(fake, tmp_path, forum)
        _turn(m, "s", "One", "1")
        started = time.monotonic()
        await asyncio.to_thread(m.close, 5)
        assert time.monotonic() - started < 5
        assert len(_thread_messages(fake, fake.created_threads[0])) == 2


# ---- the plugin entry -------------------------------------------------------------------------


def test_the_entry_passes_foreign_hooks_to_the_mirror(monkeypatch: pytest.MonkeyPatch) -> None:
    seen: list[tuple[str, str, Any]] = []

    def fake_hook(platform_key: str, name: str, kwargs: Any) -> None:
        seen.append((platform_key, name, kwargs.get("platform")))

    monkeypatch.setattr(mirror, "mirror_hook", fake_hook)
    register = entry.make_register("bothy")
    hooks: dict[str, Any] = {}

    class Ctx:
        def register_platform(self, **kw: Any) -> None:
            pass

        def register_hook(self, name: str, cb: Any) -> None:
            hooks[name] = cb

    register(Ctx())
    assert {"pre_llm_call", "post_llm_call", "post_tool_call", "on_session_end"} <= set(hooks)
    assert hooks["pre_llm_call"](platform="acp", session_id="s") is None
    assert hooks["post_llm_call"](platform="cli", session_id="s") is None
    assert hooks["post_tool_call"](session_id="s", tool_name="terminal") is None
    hooks["pre_llm_call"](platform="bothy", session_id="own")
    assert [(n, p) for _, n, p in seen] == [
        ("pre_llm_call", "acp"),
        ("post_llm_call", "cli"),
        ("post_tool_call", None),
    ]


def test_the_process_mirror_is_off_when_the_configuration_does_not_load(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[str] = []

    def broken(platform_key: str) -> Any:
        calls.append(platform_key)
        raise ConfigError("hermes_adapter.server_url: required")

    monkeypatch.setattr(mirror, "_load_process_mirror", broken)
    with captured_logs() as logs:
        for _ in range(3):
            assert mirror.mirror_hook("bothy", "pre_llm_call", {"platform": "acp"}) is None
    assert calls == ["bothy"]  # read once per process
    assert logs.text().count("mirror: off") == 1


def test_the_process_mirror_stays_off_without_mirror_keys(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(mirror, "_load_process_mirror", lambda key: None)
    assert mirror.mirror_hook("bothy", "pre_llm_call", {"platform": "acp"}) is None
    assert mirror.process_mirror("bothy") is None


# ---- read-only: people in a mirror post ------------------------------------------------------


async def _mirror_post(r: Rig, mapping: MirrorMap) -> UUID:
    forum = r.fake.add_forum("mirror", None)
    post = await r.handler.on_report(str(forum), None, "**Request** (acp session)\n\n> Task")
    assert post is not None
    mapping.bind("foreign-session", post, "acp")
    return post


async def test_a_message_in_a_mirror_post_never_reaches_hermes(tmp_path: Path) -> None:
    rt = FakeRuntime()
    mapping = MirrorMap(tmp_path / "state" / "mirror.sqlite3")
    async with rig(tmp_path, runtime=rt, mirror=mapping) as r:
        post = await _mirror_post(r, mapping)
        first = await r.fake.trigger("Can you also do X?", thread_id=post)
        await r.until(lambda: r.finished(first.run_id), "the first run")
        second = await r.fake.trigger("Hello?", thread_id=post)
        await r.until(lambda: r.finished(second.run_id), "the second run")
    assert rt.turns == []
    assert r.fake.run_state(first.run_id).state == "completed"
    assert r.fake.run_state(second.run_id).state == "completed"
    answers = [m for m in r.fake.messages if m.get("run_id")]
    assert [m["content_md"] for m in answers] == [MIRROR_READ_ONLY]
    assert answers[0]["run_id"] == str(first.run_id)
    assert r.requests("reportAgentSession") == []


async def test_a_command_in_a_mirror_post_runs_nothing(tmp_path: Path) -> None:
    rt = FakeRuntime(skills={"summarize": "Summarize"})
    mapping = MirrorMap(tmp_path / "state" / "mirror.sqlite3")
    async with rig(
        tmp_path, runtime=rt, mirror=mapping, commands=declared_commands({"summarize": "S"})
    ) as r:
        post = await _mirror_post(r, mapping)
        await r.until(lambda: len(r.fake.commands) >= 2, "the commands")
        await r.fake.invoke_command("version", thread_id=post)
        await r.until(
            lambda: any(m["content_md"] == MIRROR_READ_ONLY for m in r.fake.messages),
            "the notice",
        )
        skill = await r.fake.trigger("", thread_id=post, command="summarize")
        await r.until(lambda: r.finished(skill.run_id), "the skill run")
    assert rt.commands == []
    assert rt.turns == []
    assert sum(m["content_md"] == MIRROR_READ_ONLY for m in r.fake.messages) == 1
