# SPDX-License-Identifier: Apache-2.0
"""The example adapter against the SDK's FakeServer: in process, on 127.0.0.1, no model.

Scenarios of docs/spec/09-agents.md 16.2 on the adapter side (AG-08, AG-09, AG-11, AG-12,
AG-28), the runtime's rules, the key file, the refusal of readable secret files, an output
without message text or secrets, and the order of the start lines.
"""

from __future__ import annotations

import asyncio
import contextlib
import inspect
import random
import re
import stat
import time
from collections.abc import AsyncIterator, Callable
from pathlib import Path
from typing import Any
from uuid import UUID

import commands
import connect
import heartbeat
import keygen
import pytest
from agent import GuideAgent, parse
from runtime import Rule, decide

from bothy_sdk.adapter import Adapter, AdapterConfig, Invocation, StartOptions
from bothy_sdk.client import Client
from bothy_sdk.gateway import Gateway
from bothy_sdk.generated.api.agent_runtime import patch_run
from bothy_sdk.generated.models import JsonPatchOperation, JsonPatchOperationOp, RunPatch
from bothy_sdk.inbox import InboxEntry
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import GatewaySource
from bothy_sdk.testing import FakeServer, fingerprint

# quality.test.patience: the only upper bound of a wait; it ends a test that would hang.
PATIENCE = 180.0
CANARY = "canary-5e1f-in-every-text"


async def until(fake: FakeServer, predicate: Callable[[], Any], what: str) -> None:
    """Wait for a condition while the FakeServer's clock moves on."""
    deadline = time.monotonic() + PATIENCE
    while True:
        result = predicate()
        if inspect.isawaitable(result):
            result = await result
        if result:
            return
        assert time.monotonic() < deadline, f"waited for {what}"
        await fake.pass_time(0.05)
        await asyncio.sleep(0.005)


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


def config(fake: FakeServer, consumer: str, state_dir: Path | None = None) -> AdapterConfig:
    """The configuration of connect.adapter_config, with the FakeServer's credentials."""
    return AdapterConfig(
        state_dir=state_dir,
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer=consumer,
        settings=fake.sdk_settings,
        capabilities=heartbeat.CAPABILITIES,
        commands=commands.COMMANDS,
    )


@contextlib.asynccontextmanager
async def running(
    fake: FakeServer,
    handler: GuideAgent,
    consumer: str,
    rng: random.Random | None = None,
    state_dir: Path | None = None,
) -> AsyncIterator[Adapter]:
    """Run the example's handler until the start lines are out; stop it at the end."""
    adapter = Adapter(config(fake, consumer, state_dir), handler, clock=fake.clock)
    if rng is not None:
        gateway = Gateway(
            fake.gateway_url,
            fake.credentials,
            consumer=consumer,
            agent_id=fake.agent_id,
            settings=fake.sdk_settings,
            clock=fake.clock,
            rng=rng,
            heartbeat=adapter.heartbeat,
            nonce_sink=adapter.client.auth.remember_nonce,
        )
        adapter.source = GatewaySource(gateway)
    handler.attach(adapter, await connect.accepted_runs(adapter))
    task = asyncio.ensure_future(adapter.run())
    try:
        await until(fake, handler.ready.is_set, "the start lines")
        yield adapter
    finally:
        await adapter.stop()
        with contextlib.suppress(Exception):
            await asyncio.wait_for(task, PATIENCE)


class Gated(GuideAgent):
    """Holds every run in ``prepare`` until the test opens the gate."""

    def __init__(self) -> None:
        super().__init__()
        self.gate = asyncio.Event()
        self.prepared = 0
        self.reconciled = 0

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        self.prepared += 1
        await self.gate.wait()
        return await super().prepare(inv, run)

    async def on_commands_reconciled(self, result: Any) -> None:
        self.reconciled += 1
        await super().on_commands_reconciled(result)


def starts(fake: FakeServer, run_id: UUID) -> list[Any]:
    return [r for r in fake.requests if r.operation_id == "startRun" and str(run_id) in r.path]


async def entry_of(adapter: Adapter, run_id: UUID) -> InboxEntry:
    entries = await adapter.inbox.for_run(run_id)
    return entries[0]


@pytest.mark.anyio
@pytest.mark.parametrize(
    "kind", ["missing", "bad_signature", "alg_hs256", "aud_other_agent", "typ_access"]
)
async def test_ag08_invalid_token_starts_nothing(
    fake: FakeServer, kind: str, capsys: pytest.CaptureFixture[str]
) -> None:
    async with running(fake, GuideAgent(), f"ag08-{kind}"):
        triggered = await fake.trigger("Lotse, please help", forged=kind)
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "failed", "the end")
    view = fake.run_state(triggered.run_id)
    assert view.state_reason == "invalid_invocation_token"
    assert view.starts == 0
    assert "Step 7: accepted" not in capsys.readouterr().out


@pytest.mark.anyio
async def test_ag09_double_delivery_starts_once(fake: FakeServer) -> None:
    handler = Gated()
    async with running(fake, handler, "ag09") as adapter:
        triggered = await fake.trigger("Lotse, please help")
        await until(fake, lambda: handler.prepared == 1, "prepare")
        first = await entry_of(adapter, triggered.run_id)
        assert first.seq is not None
        # The server delivers the acknowledged event again with a new token (09 4.3).
        await fake.restart("ag09", first.seq - 1)

        async def replaced() -> bool:
            entry = await entry_of(adapter, triggered.run_id)
            return entry.deliveries >= 2 and entry.token != first.token

        await until(fake, replaced, "the newer token in the inbox")
        newest = await entry_of(adapter, triggered.run_id)
        handler.gate.set()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed", "end")
    assert handler.prepared == 1
    started = starts(fake, triggered.run_id)
    assert len(started) == 1
    assert newest.token is not None
    assert started[0].json["invocation_token"] == fingerprint(newest.token)


@pytest.mark.anyio
async def test_ag11_billing_mode_mismatch_starts_no_runtime(
    capsys: pytest.CaptureFixture[str],
) -> None:
    async with FakeServer(billing_mode="personal") as personal:
        async with running(personal, GuideAgent(), "ag11"):
            triggered = await personal.trigger("Lotse, please help")
            await until(
                personal, lambda: personal.run_state(triggered.run_id).state == "failed", "end"
            )
        view = personal.run_state(triggered.run_id)
    assert view.state_reason == "billing_mode_mismatch"
    assert view.starts == 0
    assert "Step 8: started" not in capsys.readouterr().out


@pytest.mark.anyio
async def test_ag12_patch_conflict_is_rebased(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    async with running(fake, GuideAgent(), "ag12"):
        triggered = await fake.trigger("Lotse, wait for my follow-up")
        run_id = triggered.run_id
        await until(fake, lambda: "waits for a follow-up" in capsys.readouterr().out, "waiting")
        # Someone else changes the run first: the example's next patch gets 409.
        other = Client(
            fake.base_url, fake.credentials, settings=fake.sdk_settings, clock=fake.clock
        )
        try:
            await other.call(
                patch_run.asyncio_detailed,
                run_id=run_id,
                body=RunPatch(
                    base_version=fake.run_state(run_id).version,
                    ops=[
                        JsonPatchOperation(
                            op=JsonPatchOperationOp.REPLACE,
                            path="/status_line",
                            value="external change",
                        )
                    ],
                ),
            )
        finally:
            await other.aclose()
        message_id = await fake.deliver_followup(run_id, "It happens on mobile only")
        await until(fake, lambda: fake.run_state(run_id).state == "completed", "the end")
    assert fake.followup_state(run_id, message_id) == "absorbed"
    assert fake.run(run_id)["status_line"] == "external change"


@pytest.mark.anyio
async def test_ag28_expired_token_is_renewed(fake: FakeServer) -> None:
    handler = Gated()
    async with running(fake, handler, "ag28"):
        triggered = await fake.trigger("Lotse, please help")
        await until(fake, lambda: handler.prepared == 1, "prepare")
        await fake.pass_time(
            fake.timings["agent.invocation_token_ttl"] + fake.timings["auth.clock_leeway"] + 1.0
        )
        handler.gate.set()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed", "end")
    renewals = [r for r in fake.requests if r.operation_id == "renewInvocationToken"]
    assert len(renewals) == 1
    assert fake.run_state(triggered.run_id).starts == 1
    assert len(starts(fake, triggered.run_id)) == 1


def test_runtime_rules() -> None:
    assert decide("Please wait for my FOLLOW-UP") is Rule.FOLLOWUP
    assert decide("ask me a question") is Rule.QUESTION
    assert decide("after my approval") is Rule.APPROVAL
    assert decide("a long import") is Rule.LONG
    assert decide("send the log as a FILE") is Rule.FILE
    assert decide("", command="summarize") is Rule.SUMMARIZE
    assert decide("Hello") is Rule.ANSWER
    # First match wins, in the order of the rules.
    assert decide("a long question after a follow-up") is Rule.FOLLOWUP
    assert decide("long approval question") is Rule.QUESTION
    assert decide("long approval") is Rule.APPROVAL
    assert decide("long summary", command="summarize") is Rule.LONG


def test_keygen_writes_private_key_file(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    tmp_path.chmod(0o700)
    keygen.main(["--adapter-dir", str(tmp_path)])
    printed = capsys.readouterr().out
    match = re.fullmatch(r"dpop_jkt: ([A-Za-z0-9_-]{43})\n", printed)
    assert match is not None
    key = tmp_path / connect.KEY_FILE
    assert stat.S_IMODE(key.stat().st_mode) == 0o600
    before = key.read_bytes()
    with pytest.raises(SystemExit) as exit_info:
        keygen.main(["--adapter-dir", str(tmp_path)])
    assert exit_info.value.code == 1
    assert key.read_bytes() == before


def _secret_files(tmp_path: Path) -> None:
    tmp_path.chmod(0o700)
    keygen.main(["--adapter-dir", str(tmp_path)])
    token = tmp_path / connect.TOKEN_FILE
    token.write_text(CANARY, encoding="utf-8")
    token.chmod(0o600)


@pytest.mark.parametrize("mode", [0o750, None])
def test_keygen_refuses_a_shared_or_missing_folder(tmp_path: Path, mode: int | None) -> None:
    folder = tmp_path / "adapter"
    if mode is not None:
        folder.mkdir()
        folder.chmod(mode)
    with pytest.raises(SystemExit) as exit_info:
        keygen.main(["--adapter-dir", str(folder)])
    assert exit_info.value.code == 1
    assert not (folder / connect.KEY_FILE).exists()


@pytest.mark.parametrize("name", [connect.TOKEN_FILE, connect.KEY_FILE])
def test_refuses_readable_secret_files(tmp_path: Path, name: str) -> None:
    _secret_files(tmp_path)
    (tmp_path / name).chmod(0o640)
    with pytest.raises(SystemExit) as exit_info:
        connect.credentials(tmp_path)
    message = str(exit_info.value.code)
    assert name in message
    assert CANARY not in message


def test_adapter_config_reads_the_adapter_dir(tmp_path: Path) -> None:
    _secret_files(tmp_path)
    instance, agent = UUID(int=1), UUID(int=2)
    cfg = connect.adapter_config(
        server_url="https://chat.example.com/",
        instance_id=instance,
        agent_id=agent,
        adapter_dir=tmp_path,
        consumer="default",
        capabilities={},
        commands=(),
    )
    assert cfg.base_url == "https://chat.example.com/api/v1"
    assert cfg.transport == "gateway"
    assert cfg.state_dir == tmp_path / "state"
    assert stat.S_IMODE((tmp_path / "state").stat().st_mode) == 0o700
    assert cfg.credentials.dpop is not None


@pytest.mark.anyio
async def test_output_has_no_content_or_secret(
    fake: FakeServer, capsys: pytest.CaptureFixture[str], caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level("DEBUG")
    async with running(fake, GuideAgent(), "canary"):
        triggered = await fake.trigger(f"Lotse, {CANARY}")
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed", "end")
        message_id = await fake.deliver_followup(triggered.run_id, CANARY)
        assert message_id is not None
    captured = capsys.readouterr()
    for text in (captured.out, captured.err, caplog.text):
        assert CANARY not in text
        assert fake.credentials.api_token.reveal() not in text
        assert re.search(r"eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+", text) is None
    assert f"Step 8: finished run {triggered.run_id}: completed, usage reported" in captured.out


class EarlyBeat(random.Random):
    """The first gateway heartbeat right after ``ready``, before the reconciliation ends."""

    def random(self) -> float:
        return 0.001


START_LINES = [
    "Step 4: connected as lotse with consumer {consumer}",
    "Step 5: heartbeat accepted: online, mode organization, tool class chat",
    "Step 6: commands registered: summarize, version",
]


@pytest.mark.anyio
@pytest.mark.parametrize("seed", [None, 1, 2, 3])
async def test_start_lines_order(
    fake: FakeServer, seed: int | None, capsys: pytest.CaptureFixture[str]
) -> None:
    consumer = f"order-{seed or 'early'}"
    rng = EarlyBeat() if seed is None else random.Random(seed)  # noqa: S311 - jitter, no secret
    handler = Gated()
    async with running(fake, handler, consumer, rng=rng):
        # Further gateway heartbeats, then a reconnect with a new ready and a new reconciliation.
        await fake.pass_time(fake.timings.get("sync.heartbeat_interval", 30.0) * 3)
        await fake.close_connections(4013)
        await until(fake, lambda: handler.reconciled >= 2, "the reconciliation after reconnect")
    lines = capsys.readouterr().out.splitlines()
    assert lines == [line.format(consumer=consumer) for line in START_LINES]


def lines_of(capsys: pytest.CaptureFixture[str], keep: list[str]) -> Callable[[], list[str]]:
    """A reader of the stdout lines so far; ``keep`` collects what each call took."""

    def read() -> list[str]:
        keep.extend(capsys.readouterr().out.splitlines())
        return keep

    return read


@pytest.mark.anyio
async def test_question_resumes_with_the_answer(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    out: list[str] = []
    read = lines_of(capsys, out)
    async with running(fake, GuideAgent(), "question"):
        triggered = await fake.trigger("Lotse, ask me a question first")
        run_id = triggered.run_id
        await until(fake, lambda: f"Step 9: run {run_id} waits for an answer" in read(), "wait")
        message_id = await fake.deliver_followup(run_id, "Yes, my own account")
        await until(fake, lambda: fake.run_state(run_id).state == "completed", "the end")
    read()
    assert f"Step 9: question card sent in run {run_id}" in out
    assert f"Step 9: answer received, run {run_id} resumed" in out
    assert fake.followup_state(run_id, message_id) == "absorbed"


@pytest.mark.anyio
async def test_approval_resumes_with_the_click(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    out: list[str] = []
    read = lines_of(capsys, out)
    async with running(fake, GuideAgent(), "approval"):
        triggered = await fake.trigger("Lotse, reset it after my approval")
        run_id = triggered.run_id
        await until(fake, lambda: f"Step 9: run {run_id} waits for an approval" in read(), "wait")
        assert await fake.click(run_id, "approval.once") is not None
        await until(fake, lambda: fake.run_state(run_id).state == "completed", "the end")
    read()
    card = next(line for line in out if line.startswith("Step 9: approval card sent"))
    component_id = UUID(card.rsplit(" ", 1)[1])
    assert fake.component(component_id)["definition"]["label"] == "Decision: once"
    assert f"Step 9: decision once in run {run_id}, run resumed" in out


@pytest.mark.anyio
async def test_long_run_answers_control_commands_and_stops(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    out: list[str] = []
    read = lines_of(capsys, out)
    async with running(fake, GuideAgent(), "long"):
        triggered = await fake.trigger("Lotse, watch the long import")
        run_id = triggered.run_id
        await until(
            fake,
            lambda: f"Step 9: run {run_id} keeps working until it is stopped" in read(),
            "work",
        )
        await fake.invoke_command("version", run_id=run_id)
        await until(
            fake,
            lambda: f"Step 9: answered control command version in run {run_id}" in read(),
            "the answer",
        )
        await fake.cancel(run_id)
        await until(fake, lambda: fake.run_state(run_id).state == "cancelled", "the stop")
    read()
    assert f"Step 9: stopped run {run_id}: user_cancelled" in out
    assert fake.run_state(run_id).state_reason == "user_cancelled"


@pytest.mark.anyio
async def test_restart_resumes_the_accepted_run_once(
    fake: FakeServer, tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    out: list[str] = []
    read = lines_of(capsys, out)
    state = tmp_path / "state"
    state.mkdir(mode=0o700)
    async with running(fake, GuideAgent(stop_after_accept=True), "restart", state_dir=state):
        triggered = await fake.trigger("Lotse, the reset mail does not arrive")
        run_id = triggered.run_id
        stop_line = f"Step 10: stopped after accepting run {run_id}"
        await until(fake, lambda: stop_line in read(), "the stop after the acceptance")
    assert fake.run_state(run_id).starts == 0
    async with running(fake, GuideAgent(), "restart", state_dir=state):
        await until(fake, lambda: fake.run_state(run_id).state == "completed", "the end")
    read()
    assert f"Step 10: resuming run {run_id} from the inbox" in out
    assert out.count(f"Step 8: started run {run_id}") == 1
    assert fake.run_state(run_id).starts == 1


@pytest.mark.anyio
async def test_stop_ends_a_run_that_waits_for_a_follow_up(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    out: list[str] = []
    read = lines_of(capsys, out)
    async with running(fake, GuideAgent(), "stop"):
        triggered = await fake.trigger("Lotse, wait for my follow-up")
        run_id = triggered.run_id
        await until(fake, lambda: f"Step 9: run {run_id} waits for a follow-up" in read(), "wait")
        await fake.cancel(run_id)
        await until(fake, lambda: fake.run_state(run_id).state == "cancelled", "the stop")
    read()
    assert f"Step 9: stopped run {run_id}: user_cancelled" in out
    assert fake.run_state(run_id).state_reason == "user_cancelled"


def test_command_line() -> None:
    args = parse(
        [
            "--server-url",
            "https://chat.example.com",
            "--instance-id",
            str(UUID(int=1)),
            "--agent-id",
            str(UUID(int=2)),
            "--adapter-dir",
            "adapter",
            "--stop-after",
            "accept",
        ]
    )
    assert args.consumer == "default"
    assert args.stop_after == "accept"
    assert args.adapter_dir == Path("adapter")
    with pytest.raises(SystemExit):
        parse(["--server-url", "https://chat.example.com"])
