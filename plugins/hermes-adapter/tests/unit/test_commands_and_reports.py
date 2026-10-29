# SPDX-License-Identifier: Apache-2.0
"""Hermes commands and reports without a run, on the FakeServer (09 10, 14, 17.3, 17.9).

Hermes is the fake runtime of ``adapter_kit``; the world is the FakeServer's (Allgemein, a text
channel; forums from ``add_forum``; Alex invokes the commands).
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from uuid import UUID

import pytest
from adapter_kit import FakeRuntime, Rig, rig, settings_for
from bothy_sdk.ids import uuid7
from bothy_sdk.testing.fakeserver import FakeServer
from bothy_sdk.testing.faults import Fault

from bothy_hermes_adapter import hermes_commands as hc
from bothy_hermes_adapter.hermes_commands import declared_commands
from bothy_hermes_adapter.installer import InstallError, InstallRequest, plan_install
from bothy_hermes_adapter.reports import ReportError, report_title, standalone_send

pytestmark = pytest.mark.anyio

INSTANCE = UUID("0192f4a0-7b1c-7000-8000-000000000001")


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _bodies(r: Rig, operation_id: str) -> list[dict[str, Any]]:
    return [q.json for q in r.requests(operation_id)]


# ---- the command table ----------------------------------------------------------------------


def test_table_rows_are_valid_and_classified() -> None:
    for name, (kind, reason) in hc.COMMAND_CLASSES.items():
        assert hc._NAME.fullmatch(name), name
        assert kind in ("builtin", "control", "refused"), name
        assert reason.startswith(f"{kind}:"), name
    built_in = {n for n, (k, _) in hc.COMMAND_CLASSES.items() if k == "builtin"}
    assert built_in == hc.BUILT_IN - {"close"}  # Hermes has no /close
    controls = [s.name for s in hc.control_specs()]
    assert controls == sorted(hc.CONTROL_DESCRIPTIONS)
    assert not set(controls) & hc.BUILT_IN
    assert all(s.control and not s.options for s in hc.control_specs())
    assert all(len(d) <= hc.DESCRIPTION_MAX for d in hc.CONTROL_DESCRIPTIONS.values())


def test_skill_specs_keep_only_usable_names_and_clean_descriptions() -> None:
    specs = hc.skill_specs(
        {
            "summarize": "Summarize\nthe thread\u202e",
            "Bad Name": "x",
            "stop": "a built-in name",
            "plan": "a Hermes command",
            "version": "a control command",
            "empty": "  ",
            "long": "y" * 300,
        },
        taken=("version",),
    )
    by_name = {s.name: s for s in specs}
    assert sorted(by_name) == ["empty", "long", "summarize"]
    assert by_name["summarize"].description == "Summarize the thread"
    assert by_name["empty"].description == "Run the empty skill"
    assert len(by_name["long"].description) == hc.DESCRIPTION_MAX
    assert all(not s.control for s in specs)
    option = by_name["summarize"].options[0]
    assert (option.name, str(option.type_), option.required) == ("input", "string", False)


def test_declared_commands_stay_within_the_limit() -> None:
    skills = {f"skill-{i:03d}": "A skill" for i in range(150)}
    declared = declared_commands(skills)
    assert len(declared) == hc.MAX_COMMANDS
    assert [s.name for s in declared[:2]] == ["context", "version"]


def test_option_values_and_command_text() -> None:
    values = hc.option_values([SimpleNamespace(name="input", value="x"), SimpleNamespace()])
    assert values == {"input": "x"}
    assert hc.option_values(None) == {}
    # A value never reaches the Hermes command line.
    assert hc.command_text("context", {"input": "; /yolo"}) == "/context"


async def test_commands_are_reconciled_at_connect(tmp_path: Path) -> None:
    rt = FakeRuntime(skills={"summarize": "Summarize the thread"})
    async with rig(tmp_path, runtime=rt, commands=declared_commands(rt.skills)) as r:
        await r.until(lambda: len(r.fake.commands) == 3, "three registered commands")
        registered = {c["name"]: c for c in r.fake.commands.values()}
    assert registered["context"]["control"] is True
    assert registered["version"]["control"] is True
    assert registered["summarize"]["control"] is False
    assert registered["summarize"]["options"][0]["name"] == "input"


# ---- control commands ------------------------------------------------------------------------


async def test_control_command_posts_hermes_reply_without_a_run(tmp_path: Path) -> None:
    rt = FakeRuntime(replies=["Hermes Agent v0.22.5", "  "])
    async with rig(tmp_path, runtime=rt, commands=declared_commands({})) as r:
        await r.until(lambda: len(r.fake.commands) == 2, "the control commands")
        thread = r.fake.main_thread()
        await r.fake.invoke_command("version", thread_id=thread, options={"input": "ignored"})
        await r.until(lambda: rt.commands, "the command in Hermes")
        await r.until(lambda: _bodies(r, "createMessage"), "the reply")
    command = rt.commands[0]
    assert (command.thread_id, command.text) == (thread, "/version")
    assert command.channel_id == r.fake.world.channel_id
    assert command.user_id == r.fake.world.alex.id
    replies = list(r.requests("createMessage"))
    assert len(replies) == 1  # the blank second text is not posted
    assert replies[0].path.endswith(f"/threads/{thread}/messages")
    assert replies[0].json["content_md"] == "Hermes Agent v0.22.5"
    assert "run_id" not in replies[0].json
    assert "invocation-token" not in replies[0].headers
    assert r.requests("startRun") == []


async def test_control_command_during_a_run_stays_outside_the_run(tmp_path: Path) -> None:
    import asyncio

    release = asyncio.Event()

    async def held(turn: Any, rt: FakeRuntime) -> Any:
        await release.wait()
        await rt.events.on_answer(turn.thread_id, "Done.", str(turn.message_id))
        from bothy_hermes_adapter.runtime import TurnOutcome

        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=held)
    async with rig(tmp_path, runtime=rt, commands=declared_commands({})) as r:
        await r.until(lambda: len(r.fake.commands) == 2, "the control commands")
        triggered = await r.fake.trigger("Plan the trip.")
        await r.until(lambda: rt.turns, "the turn")
        await r.fake.invoke_command("context", run_id=triggered.run_id)
        await r.until(lambda: rt.commands, "the command in Hermes")
        await r.until(lambda: _bodies(r, "createMessage"), "the reply")
        reply = _bodies(r, "createMessage")[0]
        assert "run_id" not in reply
        release.set()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert rt.commands[0].text == "/context"
    assert r.fake.run_state(triggered.run_id).state == "completed"


@pytest.mark.parametrize(
    ("name", "control"),
    [("plan", True), ("version", False), ("unknown", True)],
)
async def test_other_commands_never_reach_hermes(tmp_path: Path, name: str, control: bool) -> None:
    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt) as r:
        command_id = await r.fake.register_command(name)
        r.fake.world.commands[command_id]["control"] = control
        await r.fake.invoke_command(name, thread_id=r.fake.main_thread())
        await r.until(lambda: r.source.envelopes("command.invoked"), "the delivery")
        await r.fake.pass_time(0.2)
    assert rt.commands == []
    assert r.requests("createMessage") == []


async def test_a_failing_command_posts_nothing(tmp_path: Path) -> None:
    rt = FakeRuntime(command_error=RuntimeError("Hermes failed"))
    async with rig(tmp_path, runtime=rt, commands=declared_commands({})) as r:
        await r.until(lambda: len(r.fake.commands) == 2, "the control commands")
        await r.fake.invoke_command("version", thread_id=r.fake.main_thread())
        await r.until(lambda: rt.commands, "the command in Hermes")
        await r.fake.pass_time(0.2)
    assert r.requests("createMessage") == []


# ---- skill commands as model commands --------------------------------------------------------


async def test_skill_command_runs_the_skill_in_a_turn(tmp_path: Path) -> None:
    rt = FakeRuntime(skills={"summarize": "Summarize the thread"})
    async with rig(tmp_path, runtime=rt, commands=declared_commands(rt.skills)) as r:
        await r.until(lambda: len(r.fake.commands) == 3, "the commands")
        triggered = await r.fake.trigger(
            "unused", command="summarize", options={"input": "the notes"}
        )
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    turn = rt.turns[0]
    assert turn.user_text == "[skill summarize] the notes"
    assert turn.message_id == triggered.run_id
    answer = [b for b in _bodies(r, "createMessage") if b.get("run_id") == str(triggered.run_id)]
    assert answer
    assert "reply_to_message_id" not in answer[0]


@pytest.mark.parametrize("name", ["gone", "version", "stop"])
async def test_a_command_run_without_a_skill_fails_before_the_start(
    tmp_path: Path, name: str
) -> None:
    rt = FakeRuntime(skills={"summarize": "Summarize"})
    async with rig(tmp_path, runtime=rt) as r:
        await r.fake.register_command(name)
        triggered = await r.fake.trigger("unused", command=name)
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", "adapter_error")
    assert view.starts == 0
    assert rt.turns == []


# ---- reports without a run -------------------------------------------------------------------


async def test_report_into_a_forum_opens_a_new_post(tmp_path: Path) -> None:
    async with rig(tmp_path) as r:
        forum = r.fake.add_forum("support", None)
        first = await r.handler.on_report(str(forum), None, "## Ticket 42\nPrinter is down.")
        second = await r.handler.on_report(str(forum), None, "Ticket 43\nNo coffee.")
    assert r.fake.created_threads == [first, second]
    creates = _bodies(r, "createThread")
    assert [c["title"] for c in creates] == ["Ticket 42", "Ticket 43"]
    assert creates[0]["first_message"]["content_md"] == "## Ticket 42\nPrinter is down."
    assert "run_id" not in creates[0]["first_message"]
    assert all(q.headers.get("idempotency-key") for q in r.requests("createThread"))
    assert r.fake.thread(first)["channel_id"] == str(forum)


async def test_report_into_a_text_channel_goes_to_its_main_thread(tmp_path: Path) -> None:
    async with rig(tmp_path) as r:
        posted = await r.handler.on_report(str(r.fake.world.channel_id), None, "Nightly: green")
        main = r.fake.main_thread()
    assert r.fake.created_threads == []
    created = r.requests("createMessage")
    assert len(created) == 1
    assert created[0].path.endswith(f"/threads/{main}/messages")
    assert created[0].json["content_md"] == "Nightly: green"
    assert "run_id" not in created[0].json
    assert posted == UUID(list(r.fake.world.messages.values())[-1]["id"])


async def test_report_into_a_fixed_thread(tmp_path: Path) -> None:
    async with rig(tmp_path) as r:
        thread = r.fake.main_thread()
        await r.handler.on_report(str(r.fake.world.channel_id), str(thread), "x" * 50)
    created = r.requests("createMessage")
    assert created[0].path.endswith(f"/threads/{thread}/messages")
    assert r.requests("getChannel") == []


async def test_report_is_cut_to_the_message_limit(tmp_path: Path) -> None:
    async with rig(tmp_path, max_message_bytes=16) as r:
        await r.handler.on_report(str(r.fake.world.channel_id), None, "abcdefghij" * 5)
    assert len(_bodies(r, "createMessage")[0]["content_md"].encode()) <= 16


@pytest.mark.parametrize(
    ("chat_id", "text", "reason"),
    [
        ("unassigned", "Report", "no report channel assigned"),
        ("not-a-uuid", "Report", "not a channel or thread ID"),
        (None, "   ", "the report is empty"),
    ],
)
async def test_report_refusals_send_nothing(
    tmp_path: Path, chat_id: str | None, text: str, reason: str
) -> None:
    async with rig(tmp_path) as r:
        before = len(r.fake.requests)
        target = chat_id if chat_id is not None else str(r.fake.world.channel_id)
        with pytest.raises(ReportError, match=reason):
            await r.handler.on_report(target, None, text)
        sent = [q.operation_id for q in r.fake.requests[before:]]
    assert not {"createMessage", "createThread"} & set(sent)


async def test_report_refused_by_the_server_names_the_code(tmp_path: Path) -> None:
    async with rig(tmp_path) as r:
        forum = r.fake.add_forum("support", None)
        r.fake.inject("createThread", Fault.problem("forbidden"))
        with pytest.raises(ReportError, match="refused by the server: forbidden"):
            await r.handler.on_report(str(forum), None, "Ticket 1")
        r.fake.inject("createMessage", Fault.problem("forbidden"))
        with pytest.raises(ReportError, match="refused by the server: forbidden"):
            await r.handler.on_report(str(r.fake.world.channel_id), None, "Report")
    assert r.fake.created_threads == []


def test_report_title() -> None:
    assert report_title("\n\n#  Daily  \nbody") == "Daily"
    assert report_title("   \n") == "Report"
    assert len(report_title("t" * 500)) == 200


# ---- the standalone sender (cron without the gateway) ---------------------------------------


async def test_standalone_send_posts_with_the_platform_configuration(tmp_path: Path) -> None:
    async with FakeServer(dpop=False) as fake:
        settings = settings_for(fake, tmp_path)
        extra = {
            "hermes_adapter.server_url": fake.base_url,
            "hermes_adapter.instance_id": str(fake.instance_id),
            "hermes_adapter.token_file": str(settings.token_file),
        }
        pconfig = SimpleNamespace(extra=extra)
        forum = fake.add_forum("support", None)
        result = await standalone_send(pconfig, str(forum), "Ticket 7\nDisk full")
        assert result.get("success") is True, result
        assert fake.created_threads == [UUID(result["message_id"])]
        agent = [q.headers["user-agent"] for q in fake.requests if q.operation_id == "createThread"]
        assert agent[0].startswith("hermes-adapter-standalone/")
        main = fake.main_thread()
        result = await standalone_send(pconfig, str(fake.world.channel_id), "Hi", thread_id=None)
        assert result["success"] is True
        posted = list(fake.world.messages.values())[-1]
        assert (posted["thread_id"], posted["content_md"]) == (str(main), "Hi")
        assert "run_id" not in posted
        before = len(fake.requests)
        assert (
            "not supported"
            in (
                await standalone_send(pconfig, str(forum), "x", media_files=[("/tmp/a.png", False)])
            )["error"]
        )
        assert (await standalone_send(pconfig, "unassigned", "x"))["error"] == (
            "no report channel assigned"
        )
        assert (await standalone_send(SimpleNamespace(extra={}), str(forum), "x"))["error"] == (
            "the platform configuration is not usable"
        )
        assert len(fake.requests) == before
        fake.inject("createThread", Fault.problem("forbidden"))
        refused = await standalone_send(pconfig, str(forum), "Ticket 8")
        assert refused == {"error": "refused by the server: forbidden"}


# ---- installer and plugin entry -------------------------------------------------------------


def _request(**over: Any) -> InstallRequest:
    values: dict[str, Any] = {
        "server_url": "https://example.com/api/v1",
        "instance_id": INSTANCE,
        "token_file": Path("/srv/hermes/token"),
        "busy_input_mode": "steer",
    }
    values.update(over)
    return InstallRequest(**values)


def test_installer_sets_the_report_channel_as_home_channel() -> None:
    channel, thread = uuid7(), uuid7()
    planned = plan_install({}, _request(report_channel=channel))
    home = planned["gateway"]["platforms"]["bothy"]["home_channel"]
    assert home == {"platform": "bothy", "chat_id": str(channel), "name": "Reports"}
    fixed = plan_install(planned, _request(report_channel=channel, report_thread=thread))
    assert fixed["gateway"]["platforms"]["bothy"]["home_channel"]["thread_id"] == str(thread)
    # A later install without the option keeps the assigned channel.
    kept = plan_install(fixed, _request())
    assert kept["gateway"]["platforms"]["bothy"]["home_channel"]["chat_id"] == str(channel)
    with pytest.raises(InstallError, match="--report-thread needs --report-channel"):
        plan_install({}, _request(report_thread=thread))


def test_plugin_entry_registers_cron_delivery() -> None:
    src = Path(__file__).resolve().parents[2] / "src"
    code = (
        "import sys, inspect\n"
        "from bothy_hermes_adapter.entry import make_register\n"
        "class Ctx:\n"
        "    def register_platform(self, **kw): self.kw = kw\n"
        "    def register_hook(self, name, cb): pass\n"
        "ctx = Ctx()\n"
        "make_register('bothy')(ctx)\n"
        "fn = ctx.kw['standalone_sender_fn']\n"
        "parse = ctx.kw['parse_target_ref_fn']\n"
        "print(ctx.kw['cron_deliver_env_var'], inspect.iscoroutinefunction(fn),\n"
        "      'bothy_sdk' in sys.modules, parse('x:y:z'))\n"
    )
    done = subprocess.run(
        [sys.executable, "-c", code],
        capture_output=True,
        text=True,
        check=True,
        env={"PYTHONPATH": str(src), "PATH": "/usr/bin:/bin"},
    )
    assert done.stdout.strip() == "BOTHY_HOME_CHANNEL True False None"


def test_parse_target_ref_reads_channel_and_thread() -> None:
    from bothy_hermes_adapter.entry import parse_target_ref

    channel, thread = uuid7(), uuid7()
    assert parse_target_ref(f"{channel}:{thread}") == (str(channel), str(thread))
    assert parse_target_ref(f" {str(channel).upper()} ") == (str(channel), None)
    for bad in ("unassigned", f"{channel}:x", f"{channel}:{thread}:{thread}", ""):
        assert parse_target_ref(bad) is None
