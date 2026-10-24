# SPDX-License-Identifier: Apache-2.0
"""Every first and every second crash point of the runner (brief design note 12 f, D-M2-35-28).

The real `RunTurn` runs a scenario (a turn; follow-ups queued and folded; a full context with
compaction and retry; a bound import session with a cost baseline) against an in-memory server,
a scripted CLI and a fake clock. A runner may die after any durable effect: a write of the turn
record or the usage file, a server call that changes something, a write to the CLI's stdin. The
test counts those effects, kills the runner after each one in turn (its stdin closes, so the CLI
finishes what it has, like a real crash), restarts a runner on the same state directory (the CLI
still running, or gone), kills that one after each of its effects too, and lets a third runner
finish. After every combination:

* the run ended exactly once (one accepted finish), or was left to the server because the
  process of the record produced no ``result`` (the server ends it ``interrupted``, 09 5.5);
* the costs were booked once: the finish carries the session's cumulative cost of the last
  ``result`` minus the import's baseline, and a run left to the server stored nothing, so the
  session's next run books it;
* each ``result`` was answered at most once, and every one of a completed run;
* each follow-up was answered at most once, an answered one is ``absorbed`` (a ``delivered``
  one would be deferred into a run of its own and answered again), and in a completed run every
  follow-up is answered or still open for the server to defer;
* each tool call is one step of the run, ended.

Every wait is a step of the scripted CLI driven by the stream reader's polls; nothing depends on
the machine's speed.
"""

from __future__ import annotations

import asyncio
import contextvars
import dataclasses
import json
from collections.abc import Awaitable, Callable, Generator
from datetime import UTC, datetime, timedelta
from decimal import Decimal
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from uuid import UUID, uuid4

import pytest
from bothy_sdk.ids import uuid7
from bothy_sdk.invocation import InvocationClaims
from bothy_sdk.outbox import OutboxState

from claude_code_runner import runturn, streamreader
from claude_code_runner.adoption import AdoptionAction, adoption_decision
from claude_code_runner.app import end_in_record_or_log
from claude_code_runner.commandline import SessionChoice
from claude_code_runner.config import RunnerConfig, load_config
from claude_code_runner.runturn import RunState, RunTurn
from claude_code_runner.turn import TurnPaths
from claude_code_runner.usage import UsageStore, to_micros

MODEL = "model-large"
TICK = 0.25  # runner.poll_interval of the test, in fake seconds
SAM = UUID("01900000-0000-7000-8000-00000000a5a5")
_REAL_SLEEP = asyncio.sleep


class Crash(BaseException):
    """The runner process died at this point (a BaseException: no handler of the runner sees it)."""


@dataclasses.dataclass
class Fate:
    """The life of one runner process: it dies right after its ``at``-th durable effect."""

    at: int | None
    count: int = 0
    dead: bool = False
    names: list[str] = dataclasses.field(default_factory=list)
    on_death: Callable[[], None] | None = None

    def check(self) -> None:
        if self.dead:
            raise Crash

    def done(self, name: str) -> None:
        self.count += 1
        self.names.append(name)
        if self.at is not None and self.count >= self.at:
            self.dead = True
            if self.on_death is not None:
                self.on_death()
            raise Crash


FATE: contextvars.ContextVar[Fate] = contextvars.ContextVar("fate")


def durable(name: str, act: Callable[[], Any]) -> Any:
    """Perform a durable effect of the current runner; it may die right after it."""
    fate = FATE.get()
    fate.check()
    out = act()
    fate.done(name)
    return out


# ---- the server ----------------------------------------------------------------------------


@dataclasses.dataclass
class Server:
    """The part of the server a turn changes."""

    state: str = "running"
    finishes: list[dict[str, Any]] = dataclasses.field(default_factory=list)
    repeated_finishes: int = 0
    messages: dict[UUID, str] = dataclasses.field(default_factory=dict)
    steps: dict[str, dict[str, Any]] = dataclasses.field(default_factory=dict)
    step_errors: list[str] = dataclasses.field(default_factory=list)
    followups: dict[UUID, str] = dataclasses.field(default_factory=dict)
    notes: list[str] = dataclasses.field(default_factory=list)

    def apply(self, ops: list[tuple[str, Any, Any]]) -> None:
        order = {"queued": 0, "delivered": 1, "absorbed": 2}
        for op, key, value in ops:
            if op == "add":
                if str(key) in self.steps:
                    self.step_errors.append(f"duplicate add {key}")
                self.steps[str(key)] = dict(value)
            elif op == "update":
                if str(key) not in self.steps:
                    self.step_errors.append(f"update of unknown {key}")
                    continue
                self.steps[str(key)].update(value)
            elif op == "followup" and order[value] > order[self.followups.get(key, "queued")]:
                self.followups[key] = value

    def post(self, client_msg_id: UUID, text: str) -> None:
        old = self.messages.get(client_msg_id)
        assert old in (None, text), "the same client_msg_id with another text"
        self.messages[client_msg_id] = text

    def finish(self, state: str, costs: list[Any]) -> None:
        if self.state != "running":
            self.repeated_finishes += 1  # the server answers with the repeated final state
            return
        self.state = state
        self.finishes.append({"state": state, "micros": sum(c.amount_micros for c in costs)})


class Patcher:
    """`RunPatcher` as the live box and the turn use it; ops go out at `flush`."""

    def __init__(self, server: Server) -> None:
        self.server = server
        self.ops: list[tuple[str, Any, Any]] = []
        self.local: dict[UUID, str] = {}
        self.settings = SimpleNamespace(run_update_min_interval=4 * TICK)
        self.ended = False

    def __getattr__(self, name: str) -> Callable[..., None]:
        if name.startswith(("set_", "clear_")):
            return lambda *_, **__: None  # display members: not part of the invariants
        raise AttributeError(name)

    def add_step(self, kind: str, name: str, **members: Any) -> UUID:
        step = uuid7()
        self.ops.append(("add", step, {"kind": kind, "name": name, "status": "running"}))
        del members
        return step

    def update_step(self, step: UUID, *, status: str | None = None, **_: Any) -> None:
        self.ops.append(("update", step, {"status": status}))

    def _followup(self, message_id: UUID, state: str) -> None:
        order = {"delivered": 1, "absorbed": 2}
        if order.get(self.local.get(message_id, ""), 0) >= order[state]:
            raise ValueError("forward only")
        self.local[message_id] = state
        self.ops.append(("followup", message_id, state))

    def followup_delivered(self, message_id: UUID) -> None:
        self._followup(message_id, "delivered")

    def followup_absorbed(self, message_id: UUID) -> None:
        self._followup(message_id, "absorbed")

    async def flush(self) -> None:
        if self.ops:
            ops, self.ops = self.ops, []
            durable("patch", lambda: self.server.apply(ops))

    async def send_ops(self, ops: list[Any]) -> int:
        durable("box_note", lambda: self.server.notes.append(str(ops[0].value["name"])))
        return 1


class Clock:
    """The fake clock of the client (and, through the world, of the turn)."""

    def __init__(self, world: World) -> None:
        self.world = world

    def now(self) -> datetime:
        return datetime(2026, 10, 25, tzinfo=UTC) + timedelta(seconds=self.world.now)

    async def sleep(self, seconds: float) -> None:
        self.world.now += seconds


class Handle:
    """`RunHandle` as the turn uses it, on the in-memory server."""

    def __init__(self, world: World, run_id: UUID) -> None:
        self.world = world
        self.run_id = run_id
        self.patch = Patcher(world.server)
        self.client = SimpleNamespace(clock=Clock(world), call_as=self._call_as)
        self.outbox = SimpleNamespace()
        self.finished = False

    async def _call_as(self, _model: Any, operation: Any, **kwargs: Any) -> Any:
        assert "get_message" in getattr(operation, "__module__", ""), operation
        followup = self.world.followups[kwargs["message_id"]]
        author = SimpleNamespace(
            id=SAM, to_dict=lambda: {"display_name": "Sam", "handle": "sam", "id": str(SAM)}
        )
        return SimpleNamespace(
            author=author,
            thread_id=self.world.thread_id,
            deleted_at=None,
            content_md=followup.text,
        )

    async def send(self, reply: str, *, client_msg_id: UUID) -> Any:
        durable("send", lambda: self.world.server.post(client_msg_id, reply))
        return SimpleNamespace(state=OutboxState.SENT, entry_id=1)

    async def finish(self, state: str, **kwargs: Any) -> None:
        durable("finish", lambda: self.world.server.finish(state, kwargs.get("costs") or []))


# ---- the CLI -------------------------------------------------------------------------------


@dataclasses.dataclass
class Followup:
    name: str
    message_id: UUID
    text: str
    answered: int = 0
    offered: bool = False
    offer_done: bool = False


Step = Generator[tuple[str, Any], None, None]


class Cli:
    """One CLI process: a script of stream lines, one line per poll of the stream reader.

    Like the pinned CLI it echoes a line when a turn takes it (at the start of its own turn,
    or folded into a running one), names the inputs of a turn in ``user_message_uuids`` (a
    notification turn names none), answers a full context with an API error message and an
    error result, and exits once its stdin is closed and its work is done.
    """

    def __init__(self, world: World, paths: TurnPaths, index: int) -> None:
        self.world = world
        self.paths = paths
        self.index = index
        self.pid = self.pgid = 70000 + index
        self.queue: list[dict[str, Any]] = []
        self.stdin_open = True
        self.exited = False
        self.results = 0
        self._program = self.program()

    # Turn and StoppableGroup -----------------------------------------------------------------

    async def write_bytes(self, data: bytes) -> bool:
        if not self.stdin_open:
            return False
        durable("stdin", lambda: self.queue.append(json.loads(data)))
        return True

    async def write_line(self, line: str) -> None:
        await self.write_bytes(line.encode() + b"\n")

    def close_stdin(self) -> None:
        if self.stdin_open:
            durable("stdin_close", self.eof)

    def eof(self) -> None:
        self.stdin_open = False

    def alive(self) -> bool:
        return not self.exited

    def may_signal(self) -> bool:
        return not self.exited

    def exit_label(self) -> str:
        return "0"

    # The script ------------------------------------------------------------------------------

    def step(self) -> bool:
        """One action; False when the process waits for input or has exited."""
        if self.exited:
            return False
        action, value = next(self._program, ("exit", None))
        if action == "emit":
            with self.paths.stream.open("a") as fh:
                fh.write(json.dumps(value) + "\n")
            return True
        if action == "arrive":
            self.world.arrive(value)
            return True
        if action == "exit":
            self.exited = True
        return False

    def emit(self, message: dict[str, Any]) -> Step:
        yield "emit", {"session_id": self.world.ref, **message}

    def take(self) -> Generator[tuple[str, Any], None, dict[str, Any] | None]:
        while not self.queue:
            if not self.stdin_open:
                return None
            yield "wait", None
        return self.queue.pop(0)

    def result(self, *lines: dict[str, Any], error: str = "") -> Step:
        self.results += 1
        self.world.spent += Decimal("0.001")
        message: dict[str, Any] = {
            "type": "result",
            "subtype": "success",
            "is_error": bool(error),
            "result": error or f"Answer {self.index}.{self.results}",
            "modelUsage": {
                MODEL: {
                    "inputTokens": 100 * self.world.results,
                    "outputTokens": 10,
                    "costUSD": float(self.world.spent),
                    "contextWindow": 200000,
                }
            },
        }
        self.world.results += 1
        if lines:
            message["user_message_uuids"] = [x["uuid"] for x in lines]
            message["user_message_uuid"] = lines[-1]["uuid"]
        yield from self.emit(message)

    def took(self, line: dict[str, Any]) -> Step:
        yield from self.emit({"type": "user", "isReplay": True, "uuid": line["uuid"]})

    def answered(self, *lines: dict[str, Any]) -> None:
        for line in lines:
            followup = self.world.followup_of(line)
            if followup is not None:
                followup.answered += 1

    def program(self) -> Step:
        first = yield from self.take()
        if first is None:
            return
        init = {"type": "system", "subtype": "init", "model": MODEL}
        init["apiKeySource"] = "ANTHROPIC_API_KEY"
        if _text(first) == "/compact":
            yield from self.emit(init)
            yield from self.emit({"type": "system", "subtype": "compact_boundary"})
            yield from self.result()
            while self.stdin_open:
                yield "wait", None
            return
        yield from self.emit(init)
        if self.world.processes == 1:
            yield from self.first_turn(first)
        else:
            self.queue.insert(0, first)
        yield from self.own_turns()

    def first_turn(self, prompt: dict[str, Any]) -> Step:
        world = self.world
        yield from self.took(prompt)
        tool = f"toolu_{self.index}"
        world.tools.add(tool)
        use = {"type": "tool_use", "id": tool, "name": "Read", "input": {"file_path": "a.txt"}}
        yield from self.emit({"type": "assistant", "message": {"content": [use]}})
        for followup in world.arrivals:
            yield "arrive", followup
            while not followup.offer_done:
                yield "wait", None
        done = {"type": "tool_result", "tool_use_id": tool, "content": "ok"}
        yield from self.emit({"type": "user", "message": {"content": [done]}})
        yield from self.result(prompt)
        self.answered(prompt)
        if world.notification and self.queue:
            # A background task's notification turn folds the first queued line; its result
            # names nothing (2.1.259).
            folded = self.queue.pop(0)
            yield from self.took(folded)
            yield from self.result()
            self.answered(folded)

    def own_turns(self) -> Step:
        while True:
            line = yield from self.take()
            if line is None:
                return
            yield from self.took(line)
            followup = self.world.followup_of(line)
            full = followup is not None and followup.name == self.world.context_full_on
            if full and not self.world.context_filled:
                self.world.context_filled = True
                api_error = {
                    "type": "assistant",
                    "is_api_error_message": True,
                    "parent_tool_use_id": None,
                    "message": {"content": [{"type": "text", "text": "Prompt is too long"}]},
                }
                yield from self.emit(api_error)
                yield from self.result(line, error="Prompt is too long")
                while self.stdin_open:  # a failed turn takes nothing more
                    yield "wait", None
                return
            yield from self.result(line)
            self.answered(line)


def _text(line: dict[str, Any]) -> str:
    try:
        return str(line["message"]["content"][0]["text"])
    except (KeyError, IndexError, TypeError):
        return ""


# ---- the world -----------------------------------------------------------------------------


@dataclasses.dataclass
class Scenario:
    name: str
    followups: tuple[str, ...] = ()
    notification: bool = False
    context_full_on: str = ""
    baseline_micros: int = 0


SCENARIOS = [
    Scenario("turn"),
    Scenario("followups", followups=("F1", "F2"), notification=True),
    Scenario("compaction", followups=("F1", "F2"), notification=True, context_full_on="F2"),
    Scenario("import_baseline", baseline_micros=15000),
]


class World:
    """One run of a scenario: server, CLI processes, clock and the runners that come and go."""

    def __init__(self, tmp: Path, scenario: Scenario) -> None:
        self.tmp = tmp
        self.scenario = scenario
        self.now = 0.0
        self.server = Server()
        self.run_id = uuid4()
        self.thread_id = uuid4()
        self.channel_id = uuid4()
        self.ref = str(uuid4())
        self.processes = 0
        self.cli: Cli | None = None
        self.clis: list[Cli] = []
        self.tools: set[str] = set()
        self.results = 1
        self.spent = Decimal(scenario.baseline_micros) / Decimal(1_000_000)
        self.notification = scenario.notification
        self.context_full_on = scenario.context_full_on
        self.context_filled = False
        self.followups = {
            (mid := uuid4()): Followup(name, mid, f"{name} please") for name in scenario.followups
        }
        self.arrivals = list(self.followups.values())
        self.turn: RunTurn | None = None
        self.fate: Fate | None = None
        self.tasks: list[asyncio.Task[Any]] = []
        self.config = _config(tmp)

    def followup_of(self, line: dict[str, Any]) -> Followup | None:
        text = _text(line)
        return next((f for f in self.followups.values() if text.endswith(f.text)), None)

    # The CLI processes -----------------------------------------------------------------------

    def start(self, argv: list[str], **kwargs: Any) -> Cli:
        del argv
        self.processes += 1
        cli = Cli(self, kwargs["paths"], self.processes)
        durable("spawn", lambda: None)
        self.cli = cli
        self.clis.append(cli)
        return cli

    async def tick(self, seconds: float) -> None:
        self.now += seconds
        if self.cli is not None:
            self.cli.step()
        for _ in range(3):
            await _REAL_SLEEP(0)

    def drain(self) -> None:
        """A process that runs on while no runner reads it (until it exits)."""
        cli = self.cli
        while cli is not None and not cli.exited:
            if not cli.step() and cli.stdin_open:
                cli.eof()  # nobody writes any more: the dead runner's pipe is closed

    def arrive(self, followup: Followup) -> None:
        """A follow-up reaches the runner (the SDK hands it to the turn)."""
        followup.offered = True
        turn, fate = self.turn, self.fate
        if turn is None or fate is None or fate.dead or turn.adopted_record is not None:
            followup.offer_done = True  # no runner takes it: it stays queued for the server
            return
        claims = InvocationClaims(
            iss="instance",
            aud="agent",
            sub=SAM,
            jti=str(uuid4()),
            iat=int(Clock(self).now().timestamp()) - 5,
            exp=int(Clock(self).now().timestamp()) + 3600,
            trigger_kind="followup",
            thread_id=self.thread_id,
            channel_id=self.channel_id,
            space_id=uuid4(),
            level="trigger",
            billing_mode="organization",
            tool_class="scoped",
            permission_version=1,
            run_id=self.run_id,
            message_id=followup.message_id,
        )
        payload = SimpleNamespace(message_id=followup.message_id)
        inv = SimpleNamespace(payload=payload, claims=claims)

        async def offer() -> None:
            try:
                await turn.offer_followup(inv)  # type: ignore[arg-type]
            finally:
                followup.offer_done = True

        self.tasks.append(asyncio.ensure_future(offer()))

    # The runners -----------------------------------------------------------------------------

    def state(self) -> RunState:
        state = RunState(self.run_id, self.thread_id, self.channel_id)
        state.session = SessionChoice(self.ref, resume=True, new=False)
        state.session_mode = "resume"
        state.model = MODEL
        state.mode = "organization"
        state.cwd = self.tmp
        state.context = SimpleNamespace()  # type: ignore[assignment]
        if self.scenario.baseline_micros:
            state.agent_session = SimpleNamespace(  # type: ignore[assignment]
                started=True,
                version=1,
                cost_baseline_micros=self.scenario.baseline_micros,
                external_session_ref=self.ref,
                currency="USD",
            )
        else:
            state.agent_session = SimpleNamespace(started=True, version=1)  # type: ignore[assignment]
        return state

    def handler(self) -> Any:
        async def precheck(*_: Any) -> None:
            return None

        return SimpleNamespace(config=self.config, claude_bin="claude", precheck=precheck)

    async def life(self, at: int | None, body: Callable[[], Awaitable[None]]) -> Fate:
        """One runner process: it runs ``body`` and dies after its ``at``-th durable effect."""
        fate = Fate(at)
        self.fate = fate
        self.tasks = []

        def on_death() -> None:
            if self.cli is not None:
                self.cli.eof()  # the pipe of the dead runner closes: the CLI sees the end
            for task in [main, *self.tasks]:
                if task is not asyncio.current_task():
                    task.cancel()

        fate.on_death = on_death
        token = FATE.set(fate)
        try:
            main = asyncio.ensure_future(body())
        finally:
            FATE.reset(token)
        try:
            await main
        except (Crash, asyncio.CancelledError):
            assert fate.dead
        for task in self.tasks:
            if not task.done():
                task.cancel()
            try:
                await task
            except (Crash, asyncio.CancelledError):
                assert fate.dead
        self.turn = None
        return fate

    async def first_runner(self) -> None:
        turn = RunTurn(self.handler(), self.state(), Handle(self, self.run_id))  # type: ignore[arg-type]
        self.turn = turn
        await turn.run()

    async def next_runner(self) -> None:
        """A runner that starts on the state directory: `app` 's adoption decision, then it."""
        path = self.config.state_dir / "turns" / f"{self.run_id}.json"
        if not path.exists():
            return
        record = json.loads(path.read_text())
        cli = next((c for c in self.clis if c.pid == record.get("pid")), None)
        alive = cli is not None and cli.alive()
        in_log = end_in_record_or_log(record)
        action = adoption_decision(alive=alive, run_state=self.server.state, result_in_log=in_log)
        if action in (AdoptionAction.LEAVE, AdoptionAction.STOP):
            if cli is not None and alive:
                cli.eof()
                self.drain()
            runturn.remove_turn_record(path)
            return
        state = RunState(self.run_id, self.thread_id, self.channel_id)
        state.session = SessionChoice(str(record["session_ref"]), resume=True, new=False)
        state.session_mode = str(record.get("session_mode") or "resume")
        turn = RunTurn(self.handler(), state, Handle(self, self.run_id))  # type: ignore[arg-type]
        self.turn = turn
        known = {key: str(step["status"]) for key, step in self.server.steps.items()}
        group = cli if cli is not None else Cli(self, TurnPaths(path, path, path, path, path), 0)
        group.exited = group.exited or not alive
        await turn.adopted(record, group, known)  # type: ignore[arg-type]


def _config(tmp: Path) -> RunnerConfig:
    tmp.mkdir(parents=True, exist_ok=True)
    (tmp / "key").write_text("sk-ant-test\n")
    (tmp / "token").write_text("agent-token\n")
    values = {
        "server_url": "http://127.0.0.1:9",
        "instance_id": str(uuid4()),
        "agent_token_ref": f"file:{tmp / 'token'}",
        "anthropic_api_key_ref": f"file:{tmp / 'key'}",
        "default_model": MODEL,
        "projects_root": str(tmp),
        "state_dir": str(tmp / "state"),
        "tool_class": "scoped",
        "scoped_allowed_tools": ["Read"],
        "poll_interval": f"{int(TICK * 1000)}ms",
        "followup_grace": "30s",
    }
    lines = ["[runner]", *(f"{k} = {json.dumps(v)}" for k, v in values.items())]
    path = tmp / "runner.toml"
    path.write_text("\n".join(lines) + "\n")
    return load_config(path)


@pytest.fixture
def harness(monkeypatch: pytest.MonkeyPatch) -> None:
    """Route the turn's I/O to the world of the running scenario (`World.current`)."""
    worlds: list[World] = []
    real_write, real_remove = runturn.write_turn_record, runturn.remove_turn_record
    real_record, real_start = UsageStore.record, UsageStore.start_without_baseline

    def world() -> World:
        return worlds[-1]

    async def terminate(process: Any, *_: Any, **__: Any) -> None:
        FATE.get().check()
        process.close_stdin()

    monkeypatch.setattr(
        runturn, "write_turn_record", lambda p, r: durable("record", lambda: real_write(p, r))
    )
    monkeypatch.setattr(
        runturn, "remove_turn_record", lambda p: durable("unrecord", lambda: real_remove(p))
    )
    monkeypatch.setattr(
        UsageStore,
        "record",
        lambda self, *a, **k: durable("usage", lambda: real_record(self, *a, **k)),
    )
    monkeypatch.setattr(
        UsageStore,
        "start_without_baseline",
        lambda self: durable("usage", lambda: real_start(self)),
    )
    start = SimpleNamespace(start=lambda argv, **k: world().start(argv, **k))
    monkeypatch.setattr(runturn, "Turn", start)
    monkeypatch.setattr(runturn, "terminate", terminate)
    monkeypatch.setattr(runturn, "read_identity", lambda pid, timeout: None)
    monkeypatch.setattr(runturn, "_group_runs", lambda group: False)
    monkeypatch.setattr(runturn, "render_context", lambda context, file_texts: None)
    monkeypatch.setattr(runturn, "context_items", lambda rendered: ["Fix the form"])
    monkeypatch.setattr(RunTurn, "monotonic", lambda self: world().now)
    monkeypatch.setattr(streamreader, "asyncio", SimpleNamespace(sleep=lambda s: world().tick(s)))
    World.current = worlds  # type: ignore[attr-defined]


async def play(
    tmp: Path, scenario: Scenario, first: int | None, mode: str, second: int | None
) -> World:
    """One combination: the first runner dies after effect ``first``, the second after
    ``second``; ``mode`` says whether the CLI still runs (``live``) or has exited (``gone``)
    when the next runner starts."""
    world = World(tmp, scenario)
    World.current.append(world)  # type: ignore[attr-defined]
    fates = [await world.life(first, world.first_runner)]
    for at in (second, None):
        if not fates[-1].dead:
            break
        if mode == "gone":
            world.drain()
        fates.append(await world.life(at, world.next_runner))
    world.fates = fates  # type: ignore[attr-defined]
    return world


def check(world: World) -> None:
    """The invariants after one combination (see the module docstring)."""
    server, scenario = world.server, world.scenario
    assert server.step_errors == []
    # One end, or left to the server only when the record's process left no result.
    last = world.clis[-1] if world.clis else None
    if server.state == "running":
        assert server.finishes == []
        assert last is None or last.results == 0, "left running although a result exists"
        usage = UsageStore(world.config.state_dir / "sessions" / world.ref)
        assert usage.pending_for(str(world.run_id)) is None
    else:
        assert len(server.finishes) == 1
        expected = to_micros(world.spent) - scenario.baseline_micros
        assert server.finishes[0]["micros"] == expected, "costs not booked exactly once"
    # Answers: each at most once; every one of a completed run.
    texts = list(server.messages.values())
    assert len(texts) == len(set(texts)), "an answer posted twice"
    # Follow-ups: answered at most once; answered means absorbed; open ones are deferred.
    for followup in world.followups.values():
        state = server.followups.get(followup.message_id, "queued")
        assert followup.answered <= 1, f"{followup.name} answered twice"
        if followup.answered:
            assert state == "absorbed", f"{followup.name} answered but {state}"
        elif server.state == "completed":
            assert state in ("queued", "delivered"), f"{followup.name} lost ({state})"
    # Steps: one per tool call, ended, once the run finished.
    if server.state != "running":
        tool_steps = [s for s in server.steps.values() if s["kind"] == "tool_call"]
        assert len(tool_steps) == len(world.tools)
        assert all(s["status"] != "running" for s in tool_steps)


def effects(tmp: Path, scenario: Scenario, *crashes: int | None, mode: str = "live") -> int:
    """How many durable effects the last runner of a combination has without dying."""
    first, *rest = crashes
    world = asyncio.run(play(tmp, scenario, first, mode, rest[0] if rest else None))
    return world.fates[-1].count  # type: ignore[attr-defined]


@pytest.mark.parametrize("scenario", SCENARIOS, ids=lambda s: s.name)
def test_every_first_and_second_crash_point(
    tmp_path: Path, harness: None, scenario: Scenario
) -> None:
    combos = 0
    clean = asyncio.run(play(tmp_path / "clean", scenario, None, "live", None))
    check(clean)
    assert clean.server.state == "completed"
    assert all(f.answered == 1 for f in clean.followups.values())
    first_points = clean.fates[0].count  # type: ignore[attr-defined]
    assert first_points > 5
    for first in range(1, first_points + 1):
        for mode in ("live", "gone"):
            base = tmp_path / f"{first}-{mode}"
            probe = asyncio.run(play(base / "x", scenario, first, mode, None))
            try:
                check(probe)
            except AssertionError as err:
                names = [f.names for f in probe.fates]  # type: ignore[attr-defined]
                raise AssertionError(f"{scenario.name} {first}/{mode} {names}") from err
            fates = probe.fates  # type: ignore[attr-defined]
            second_points = fates[1].count if len(fates) > 1 else 0
            for second in range(1, second_points + 1):
                world = asyncio.run(play(base / str(second), scenario, first, mode, second))
                try:
                    check(world)
                except AssertionError as err:
                    names = [f.names for f in world.fates]  # type: ignore[attr-defined]
                    where = f"{scenario.name} {first}/{mode}/{second} {names}"
                    raise AssertionError(where) from err
                combos += 1
    assert combos > first_points
