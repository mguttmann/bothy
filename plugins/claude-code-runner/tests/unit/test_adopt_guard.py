# SPDX-License-Identifier: Apache-2.0
"""One turn record whose adoption fails never stops the start of the runner (09 18.9)."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from unit.test_main import config as config_file

from claude_code_runner.app import RunnerHandler
from claude_code_runner.config import load_config

pytestmark = pytest.mark.anyio

FIRST = "0192f4a0-7b1c-7d07-84b0-000000000001"
SECOND = "0192f4a0-7b1c-7d07-84b0-000000000002"
THREAD = "0192f4a0-7b1c-7d07-84b0-0000000000aa"


async def test_a_failing_record_is_logged_and_the_next_one_adopted(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, caplog: pytest.LogCaptureFixture
) -> None:
    config = load_config(config_file(tmp_path))
    turns = config.state_dir / "turns"
    turns.mkdir(parents=True, mode=0o700)
    for run in (FIRST, SECOND):
        record = {"run_id": run, "thread_id": THREAD, "session_ref": THREAD}
        (turns / f"{run}.json").write_text(json.dumps(record))
    adopted: list[UUID] = []

    async def adopt(
        self: Any, adapter: Any, path: Path, record: Any, run_id: UUID, *_: Any
    ) -> None:
        if str(run_id) == FIRST:
            raise ValueError("/a/path/canary in the message")
        adopted.append(run_id)

    monkeypatch.setattr(RunnerHandler, "_adopt", adopt)
    handler = RunnerHandler(config, {})
    with caplog.at_level("INFO", logger="claude_code_runner"):
        await handler.adopt_turns(None)  # type: ignore[arg-type]
    assert adopted == [UUID(SECOND)]
    errors = [r.getMessage() for r in caplog.records if r.levelname == "ERROR"]
    assert errors == [
        json.dumps({"event": "turn_adoption_failed", "record": FIRST, "error": "ValueError"})
    ]
    assert "canary" not in caplog.text
    assert (turns / f"{FIRST}.json").exists()  # kept: the next start tries it again
