# SPDX-License-Identifier: Apache-2.0
"""The transcript of a background session as the stream of a turn (09 18.4 "Background mode").

The mapping record by record against the recorded transcripts of the pinned CLI
(``tests/fixtures/recording/background_<version>``), the members the runner reads against the
recorded member table, and the class test of the mode: every recorded transcript, read through
`TranscriptReader` with the statuses the session list showed, gives the same decisions of the
stdin and stop rule (`ProcessState.decide`) as the print stream of the same scenario. Then what
only the session list says (background work, a session that waits, a replaced process), with
synthetic records.
"""

from __future__ import annotations

import json
import time
from decimal import Decimal
from pathlib import Path
from typing import Any

import pytest
from real_cli.record import reduced_record
from real_cli.record_background import KEPT

from claude_code_runner.account import AccountRetry, account_error
from claude_code_runner.bgsession import NAME_PREFIX, Reading, SessionEntry
from claude_code_runner.bgtranscript import (
    BACKGROUND_TASK,
    CAUSE_REPLACED,
    CAUSE_WAITING,
    READ_FIELDS,
    PricedUsage,
    TranscriptMapper,
    TranscriptReader,
    prompt_taken,
    replayed_records,
)
from claude_code_runner.failures import api_error_text
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION
from claude_code_runner.livecost import PriceTable
from claude_code_runner.processstate import ProcessState

pytestmark = pytest.mark.anyio

RECORDED = (
    Path(__file__).resolve().parents[1]
    / "fixtures"
    / "recording"
    / f"background_{PINNED_CLAUDE_CODE_VERSION}"
)
SCENARIOS = ("new_post", "resumed", "followup", "background", "stopped")
TWINS = ("new_post", "resumed", "followup", "background")
ACCOUNT_RECORDS = ("session-owner", "attachment")
"""The record types that hold account identifiers (``session-owner`` and, as attachments,
``session_context`` and ``credential_org``): no recording and no mapped message has them."""
PID = 4711


def lines(name: str) -> list[dict[str, Any]]:
    return [json.loads(x) for x in (RECORDED / name).read_text("utf-8").splitlines()]


def transcript(scenario: str) -> list[dict[str, Any]]:
    return lines(f"transcript_{scenario}.jsonl")


def mapped(scenario: str) -> list[dict[str, Any]]:
    mapper = TranscriptMapper("prompt-uuid")
    return [m for record in transcript(scenario) for m in mapper.map(record)]


def shape(message: dict[str, Any]) -> str:
    kind = str(message["type"])
    if kind == "system":
        return f"system/{message['subtype']}"
    if kind == "command_lifecycle":
        return f"command/{message['state']}"
    if kind == "user":
        return "echo" if message.get("isReplay") else "tool_result"
    if kind == "result":
        return "result/error" if message["is_error"] else "result"
    return kind


# ---- the mapping, record by record --------------------------------------------------------------


def test_a_new_post_maps_to_the_stream_of_a_turn() -> None:
    messages = mapped("new_post")
    assert [shape(m) for m in messages] == [
        "system/init",  # the prompt record: the session took the prompt
        "echo",
        "system/bg_request",
        "system/init",  # the model of the first assistant message
        "assistant",
        "tool_result",
        "assistant",
        "result",
        "bg_totals",
        "bg_totals",
    ]
    init, echo, request, model = messages[:4]
    assert init["session_id"] == transcript("new_post")[0]["sessionId"]
    assert echo == {"type": "user", "isReplay": True, "uuid": "prompt-uuid"}
    assert request["login"] is False  # the stand-in is no stored login
    assert model["model"] == messages[4]["message"]["model"] != "<synthetic>"
    tool = messages[4]["message"]["content"][0]
    assert (tool["type"], tool["name"]) == ("tool_use", "Bash")
    assert messages[5]["message"]["content"] == [
        {"type": "tool_result", "tool_use_id": tool["id"], "is_error": False}
    ]
    result = messages[7]
    assert (result["subtype"], result["result"], result["num_turns"]) == ("success", "Answer 1", 2)
    totals = messages[-1]["modelUsage"]
    [(model_name, usage)] = totals.items()
    assert model_name == model["model"]
    assert usage["inputTokens"] == 2400  # two requests of the stand-in
    assert usage["costUSD"] > 0


def test_a_resumed_process_starts_at_its_own_prompt() -> None:
    assert [shape(m) for m in mapped("resumed")][:5] == [
        "system/init",
        "echo",
        "system/bg_request",
        "system/init",
        "assistant",
    ]
    assert [m["result"] for m in mapped("resumed") if m["type"] == "result"] == ["Answer 2"]


def test_inbox_lines_map_to_their_echo_in_the_turn_or_as_a_turn_of_their_own() -> None:
    records, messages = transcript("followup"), mapped("followup")
    absorbed = next(r for r in records if r.get("reason") == "absorbed_mid_turn")
    own = next(r for r in records if (r.get("origin") or {}).get("kind") == "peer")
    shapes = [shape(m) for m in messages]
    # During the tool call: queued, then taken into the running turn (its echo, 18.5).
    at = shapes.index("command/queued")
    assert shapes[at : at + 3] == ["command/queued", "command/completed", "echo"]
    assert messages[at + 2]["uuid"] == absorbed["commandUuid"]
    # Between two turns: queued, started as a turn of its own, then its echo.
    again = shapes.index("command/queued", at + 1)
    assert shapes[again : again + 4] == ["command/queued", "command/started", "system/init", "echo"]
    assert messages[again + 3]["uuid"] == own["uuid"]
    assert [m["result"] for m in messages if m["type"] == "result"] == ["Answer 1", "Answer 2"]
    assert shapes.index("echo", at) < shapes.index("result")  # absorbed before the answer


def test_an_api_error_maps_to_the_message_the_account_rule_reads() -> None:
    messages = mapped("background")
    errors = [m for m in messages if m.get("is_api_error_message")]
    assert len(errors) == 2  # the refused follow-up and the refused notification
    for error in errors:
        assert error["error"] == "rate_limit"
        assert error["message"]["model"] == "<synthetic>"
        assert api_error_text(error) is not None
        found = account_error(error)
        assert found is not None
        assert found.kind == "usage_limit"
    results = [m for m in messages if m["type"] == "result"]
    assert [r["is_error"] for r in results] == [False, True, True, False]
    assert [r["result"] for r in results] == ["Answer 1", "", "", "Answer 2"]
    # The notification of the background command starts a turn by itself.
    assert sum(1 for m in messages if shape(m) == "command/started") == 3


def test_a_stopped_turn_has_no_end_and_still_its_totals() -> None:
    shapes = [shape(m) for m in mapped("stopped")]
    assert "result" not in shapes
    assert shapes[-2:] == ["bg_totals", "bg_totals"]


def test_a_recorded_usage_window_would_carry_its_reset_time() -> None:
    # ``apiErrorParams`` is read in the CLI's file: the stand-in sends no usage window.
    record = next(r for r in transcript("background") if r.get("isApiErrorMessage"))
    record = {**record, "apiErrorParams": {"rate_limit_info": {"resetsAt": 1794535800}}}
    mapper = TranscriptMapper("p")
    mapper.started = True
    [error] = [m for m in mapper.map(record) if m["type"] == "assistant"]
    found = account_error(error)
    assert found is not None
    assert found.resets_at == 1794535800


def test_no_content_but_what_the_live_box_shows_reaches_a_mapped_message() -> None:
    for scenario in SCENARIOS:
        for message in mapped(scenario):
            text = json.dumps(message)
            assert "Another Claude session" not in text  # the text of an inbox line
            assert "task-notification" not in text
            assert "toolUseResult" not in text
            if message["type"] == "user" and not message.get("isReplay"):
                for item in message["message"]["content"]:  # never the output of a tool call
                    assert set(item) == {"type", "tool_use_id", "is_error"}


def test_records_before_the_prompt_sidechains_and_other_types_are_skipped() -> None:
    mapper = TranscriptMapper("p")
    before = [
        {"type": "user", "message": {"content": [{"type": "tool_result", "tool_use_id": "t"}]}},
        {"type": "assistant", "message": {"id": "m", "model": "x", "content": []}},
        {"type": "system", "subtype": "turn_duration"},
        {"type": "queue-operation", "operation": "enqueue"},
    ]
    assert [mapper.map(r) for r in before] == [[], [], [], []]
    assert not mapper.started
    prompt = {
        "type": "user",
        "isSidechain": False,
        "sessionId": "s",
        "origin": {"kind": "human"},
        "message": {"role": "user", "content": "Fix it"},
    }
    assert [shape(m) for m in mapper.map(prompt)] == ["system/init", "echo"]
    assert mapper.map({**prompt, "isSidechain": True}) == []
    assert mapper.map({"type": "session-owner", "accountUuid": "a"}) == []
    assert mapper.map({"type": "attachment", "attachment": {"type": "session_context"}}) == []
    assert mapper.map({"no": "type"}) == []
    counted = dict(mapper.skipped)
    assert counted["session-owner"] == 1
    assert counted["attachment"] == 1
    assert counted["sidechain"] == 1


def test_a_request_with_the_stored_login_is_known_by_its_beta() -> None:
    def login(betas: object) -> object:
        mapper = TranscriptMapper("p")
        [message] = mapper.map({"type": "api-request", "params": {"betas": betas}})
        assert mapper.map({"type": "api-request", "params": {"betas": betas}}) == []  # once
        return message["login"]

    assert login(["claude-code-20250219", "oauth-2025-04-20"]) is True
    assert login(["claude-code-20250219"]) is False
    assert login(None) is False


# ---- the recorded member table ------------------------------------------------------------------


def test_every_member_the_runner_reads_is_one_the_pinned_cli_wrote() -> None:
    table = json.loads((RECORDED / "transcript_fields.json").read_text("utf-8"))
    for record, members in READ_FIELDS.items():
        written = set(table["records"][record])
        # Read in the CLI's file, not recorded: the stand-in sends no usage window.
        assert set(members) - {"apiErrorParams"} <= written, record
    read = {"id", "sessionId", "pid", "status", "waitingFor", "kind", "name"}
    assert read <= set(table["session_entry"])
    meta = json.loads((RECORDED / "meta.json").read_text("utf-8"))
    assert meta["cli_version"] == PINNED_CLAUDE_CODE_VERSION


def test_a_recording_keeps_only_the_record_types_the_runner_reads() -> None:
    table = json.loads((RECORDED / "transcript_fields.json").read_text("utf-8"))
    # The CLI wrote attachments (``session-owner`` comes only with a login); none is kept.
    assert "attachment" in table["records"]
    for scenario in SCENARIOS:
        kinds = {r["type"] for r in transcript(scenario)}
        assert kinds <= set(KEPT), scenario
        assert not kinds & set(ACCOUNT_RECORDS)
        versions = {r.get("version") for r in transcript(scenario)} - {None}
        assert versions == {PINNED_CLAUDE_CODE_VERSION}


def test_a_recorded_request_keeps_only_its_model_and_betas() -> None:
    raw = {
        "type": "api-request",
        "sessionId": "s",
        "timestamp": "t",
        "version": "v",
        "querySource": "repl_main_thread",
        "params": {"model": "m", "betas": ["b"], "max_tokens": 1, "thinking": {"type": "x"}},
        "cacheControl": [{"path": ["system", 0]}],
        "requestDigest": "d",
    }
    assert reduced_record(raw) == {
        "type": "api-request",
        "sessionId": "s",
        "timestamp": "t",
        "version": "v",
        "querySource": "repl_main_thread",
        "params": {"model": "m", "betas": ["b"]},
    }
    assert reduced_record({"type": "user", "uuid": "u"}) == {"type": "user", "uuid": "u"}
    for scenario in SCENARIOS:
        for record in transcript(scenario):
            if record["type"] == "api-request":
                assert set(record["params"]) <= {"model", "betas"}


# ---- the reader: the transcript and the session list --------------------------------------------


class Sessions:
    """The session list of a test: the status of one session, changed by the test."""

    interval = 0.0

    def __init__(self, status: str = "busy", after_turn: list[str] | None = None) -> None:
        self.status = status
        self.after_turn = list(after_turn or [])
        self.waiting: str | None = None
        self.pid: int | None = PID
        self.last = self._reading()

    def now(self) -> float:
        return time.monotonic()

    def _reading(self) -> Reading:
        entry = SessionEntry(
            "1a2b3c4d",
            "session",
            f"{NAME_PREFIX}run",
            own=True,
            has_process=self.pid is not None,
            pid=self.pid,
            status=self.status,
            waiting_for=self.waiting,
        )
        return Reading(time.monotonic(), (entry,))

    async def read(self, *, after: float | None = None) -> Reading:
        if after is not None:
            if self.after_turn:
                self.status = self.after_turn.pop(0)
            self.last = self._reading()
        return self.last


def chunks(records: list[dict[str, Any]]) -> list[bytes]:
    """The transcript as the CLI wrote it over time: one piece per turn, then the rest."""
    out, piece = [], b""
    for record in records:
        piece += json.dumps(record).encode() + b"\n"
        if record.get("subtype") == "turn_duration":
            out.append(piece)
            piece = b""
    return [*out, piece] if piece else out


async def read_through(
    tmp_path: Path, pieces: list[bytes], sessions: Sessions
) -> tuple[list[dict[str, Any]], TranscriptReader]:
    """Read a transcript that grows piece by piece; the process ends after the last piece."""
    path = tmp_path / "session.jsonl"
    path.write_bytes(b"")
    alive = [True]
    reader = TranscriptReader(
        lambda: path,
        0.0,
        lambda: alive[0],
        TranscriptMapper("prompt-uuid"),
        sessions=sessions,  # type: ignore[arg-type]
        short_id="1a2b3c4d",
        pid=PID,
    )
    left = list(pieces)
    out: list[dict[str, Any]] = []
    async for message in reader.items():
        if message is not None:
            out.append(message)
        elif left:
            with path.open("ab") as fh:
                fh.write(left.pop(0))
        else:
            alive[0] = False
    return out, reader


def decisions(messages: list[dict[str, Any]]) -> list[str]:
    """The decision of the stdin and stop rule after every turn end, and once all is quiet."""
    schedule = AccountRetry(first=60.0, second=120.0, interval=180.0, max_wait=1200.0)
    state = ProcessState(has_stdin=True, account_retry=schedule)
    now = 0.0
    out = []
    for message in messages:
        now += 1.0
        state.apply(message, now)
        if message.get("type") == "result":
            out.append(_decide(state, now))
    return [*out, _decide(state, now + 31.0)]


def _decide(state: ProcessState, now: float) -> str:
    return state.decide(now, stdin_open=True, pending=0, followup_grace=30.0, stop_grace=10.0).value


@pytest.mark.parametrize("scenario", TWINS)
async def test_a_recorded_transcript_gives_the_decisions_of_its_print_stream(
    scenario: str, tmp_path: Path
) -> None:
    meta = json.loads((RECORDED / "meta.json").read_text("utf-8"))
    sessions = Sessions(after_turn=meta["after_turn"][scenario])
    background, _ = await read_through(tmp_path, chunks(transcript(scenario)), sessions)
    printed = lines(f"stream_{scenario}.jsonl")
    assert decisions(background) == decisions(printed)
    results = [m for m in printed if m.get("type") == "result"]
    assert [m["result"] for m in background if m["type"] == "result" and not m["is_error"]] == [
        r["result"] for r in results if not r.get("is_error")
    ]


async def test_the_decisions_of_the_recorded_scenarios(tmp_path: Path) -> None:
    # What the class test compares, spelled out once: a turn without background work is
    # released at its end; a session with background work is held, also through two refused
    # turns, and released one quiet window after the answered retry.
    meta = json.loads((RECORDED / "meta.json").read_text("utf-8"))
    found = {}
    for scenario in TWINS:
        folder = tmp_path / scenario
        folder.mkdir()
        sessions = Sessions(after_turn=meta["after_turn"][scenario])
        messages, _ = await read_through(folder, chunks(transcript(scenario)), sessions)
        found[scenario] = decisions(messages)
    assert found == {
        "new_post": ["close", "close"],
        "resumed": ["close", "close"],
        "followup": ["close", "close", "close"],
        "background": ["hold", "hold", "hold", "hold", "close"],
    }


def record(kind: str, **members: Any) -> dict[str, Any]:
    return {"type": kind, "isSidechain": False, "sessionId": "session", **members}


PROMPT = record("user", origin={"kind": "human"}, message={"role": "user", "content": "Go"})
ANSWER = record(
    "assistant",
    message={"id": "msg_1", "model": "model-large", "content": [{"type": "text", "text": "Done."}]},
)
TURN_END = record("system", subtype="turn_duration")
TOOL = record(
    "assistant",
    message={
        "id": "msg_0",
        "model": "model-large",
        "content": [{"type": "tool_use", "id": "t1", "name": "Bash", "input": {"command": "x"}}],
    },
)


def piece(*records: dict[str, Any]) -> bytes:
    return b"".join(json.dumps(r).encode() + b"\n" for r in records)


async def test_busy_after_a_turn_end_is_background_work(tmp_path: Path) -> None:
    sessions = Sessions(after_turn=["busy", "idle"])
    notified = record("user", origin={"kind": "task-notification"}, message={"content": "n"})
    pieces = [
        piece(PROMPT, ANSWER, TURN_END),
        piece(
            record("queue-operation", operation="enqueue"),
            record("queue-operation", operation="dequeue"),
            notified,
            ANSWER,
            TURN_END,
        ),
    ]
    messages, _ = await read_through(tmp_path, pieces, sessions)
    tasks = [
        (i, [t["task_id"] for t in m["tasks"]])
        for i, m in enumerate(messages)
        if m.get("subtype") == "background_tasks_changed"
    ]
    results = [i for i, m in enumerate(messages) if m["type"] == "result"]
    # One running task before the first result, none before the second.
    assert [t for _, t in tasks] == [[BACKGROUND_TASK], []]
    assert tasks[0][0] < results[0] < tasks[1][0] < results[1]


async def test_busy_because_the_next_turn_started_is_no_background_work(tmp_path: Path) -> None:
    # A line queued during the turn's last request starts its turn at once: the list says
    # busy after the turn end, and the transcript already shows the start of that turn.
    sessions = Sessions(after_turn=["busy", "idle"])
    queued = record("queue-operation", operation="enqueue")
    started = record("queue-operation", operation="dequeue")
    peer = record("user", uuid="line-1", origin={"kind": "peer"}, message={"content": "x"})
    pieces = [piece(PROMPT, queued, ANSWER, TURN_END, started, peer, ANSWER, TURN_END)]
    messages, _ = await read_through(tmp_path, pieces, sessions)
    assert not [m for m in messages if m.get("subtype") == "background_tasks_changed"]
    assert sum(1 for m in messages if m["type"] == "result") == 2


async def test_a_long_tool_call_counts_as_progress(tmp_path: Path) -> None:
    sessions = Sessions(status="busy", after_turn=["idle"])
    pieces = [piece(PROMPT, TOOL), piece(ANSWER, TURN_END)]
    messages, reader = await read_through(tmp_path, pieces, sessions)
    kinds = [m["type"] for m in messages]
    assert "tool_progress" in kinds[: kinds.index("result")]
    assert reader.status == "idle"


async def test_a_session_that_waits_for_a_person_fails_the_turn_once(tmp_path: Path) -> None:
    sessions = Sessions(status="waiting")
    sessions.waiting = "permission prompt"
    sessions.last = sessions._reading()
    messages, _ = await read_through(tmp_path, [piece(PROMPT, TOOL)], sessions)
    failures = [m for m in messages if m["type"] == "result"]
    assert failures == [
        {
            "type": "result",
            "subtype": "error_during_execution",
            "is_error": True,
            "errors": ["The background session waited for a permission prompt and was stopped."],
            "bg_cause": CAUSE_WAITING,
        }
    ]


async def test_a_text_of_the_cli_never_reaches_the_failure(tmp_path: Path) -> None:
    sessions = Sessions(status="waiting")
    sessions.waiting = "SECRET-TEXT of another version"
    sessions.last = sessions._reading()
    messages, _ = await read_through(tmp_path, [piece(PROMPT, TOOL)], sessions)
    [failure] = [m for m in messages if m["type"] == "result"]
    assert failure["errors"] == ["The background session waited for a person and was stopped."]


async def test_a_process_the_runner_did_not_launch_fails_the_turn(tmp_path: Path) -> None:
    sessions = Sessions(status="busy")
    sessions.pid = PID + 1
    sessions.last = sessions._reading()
    messages, _ = await read_through(tmp_path, [piece(PROMPT, TOOL)], sessions)
    [failure] = [m for m in messages if m["type"] == "result"]
    assert failure["bg_cause"] == CAUSE_REPLACED


# ---- the launch offset, the replay and the priced usage -----------------------------------------


def test_a_transcript_shows_whether_a_session_took_the_prompt(tmp_path: Path) -> None:
    path = tmp_path / "session.jsonl"
    old = piece(PROMPT, ANSWER, TURN_END)
    path.write_bytes(old + piece(record("custom-title"), TOOL))
    assert prompt_taken(path, 0) is True
    assert prompt_taken(path, len(old)) is False  # only what follows the launch offset counts
    with path.open("ab") as fh:
        fh.write(piece(PROMPT))
    assert prompt_taken(path, len(old)) is True
    assert prompt_taken(None, 0) is False
    assert prompt_taken(tmp_path / "missing.jsonl", 0) is False


def test_an_adopter_maps_the_records_before_the_offset_again(tmp_path: Path) -> None:
    path = tmp_path / "session.jsonl"
    first, second = piece(PROMPT, TOOL), piece(ANSWER, TURN_END)
    path.write_bytes(first + second)
    mapper = TranscriptMapper("prompt-uuid")
    replayed = replayed_records(path, 0, len(first), mapper)
    assert [shape(m) for m, _, _ in replayed] == ["system/init", "echo", "system/init", "assistant"]
    assert replayed[-1][1:] == (len(piece(PROMPT)), len(first))  # the line of the last message
    assert mapper.started
    assert mapper.turn_open
    # The mapper stands where the previous runner's stood: the turn end gives the answer.
    rest = [m for raw in second.splitlines() for m in mapper.map(json.loads(raw))]
    assert [m.get("result") for m in rest if m["type"] == "result"] == ["Done."]
    assert replayed_records(path, 5, 5, TranscriptMapper("p")) == []
    assert replayed_records(None, 0, 10, TranscriptMapper("p")) == []


def test_messages_are_priced_once_with_their_newest_usage() -> None:
    prices = PriceTable(
        {("model-large", "input_mtok"): 3_000_000, ("model-large", "output_mtok"): 15_000_000}
    )
    usage = PricedUsage()
    assert usage.model_usage(prices) == {}

    def message(ident: str, tokens: int, **more: Any) -> dict[str, Any]:
        inner = {
            "id": ident,
            "model": "model-large[1m]",
            "usage": {"input_tokens": tokens, "output_tokens": 10},
        }
        return {"type": "assistant", "message": inner, **more}

    usage.add(message("msg_1", 100))
    usage.add(message("msg_1", 200))  # a further content block of the same message
    usage.add(message("msg_2", 1000))
    usage.add(message("msg_3", 9, is_api_error_message=True))  # an API error is no request
    [(model, values)] = usage.model_usage(prices).items()
    assert model == "model-large"
    assert (values.input_tokens, values.output_tokens) == (1200, 20)
    assert values.cost_usd == Decimal(1200 * 3 + 20 * 15) / Decimal(1_000_000)
