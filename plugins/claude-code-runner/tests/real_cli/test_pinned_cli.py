# SPDX-License-Identifier: Apache-2.0
"""The recognised texts and the stream messages exist in the installed, pinned CLI (RK-14).

Collected only behind the switch of this folder (``CLAUDE_CODE_RUNNER_REAL_CLI=1``, see
``tests/conftest.py``); with the switch on and no CLI installed it fails, it never skips. It
starts no session, uses no credential and calls no model: it prints the version and reads the
bytes of the CLI's files (design note 14). ``CLAUDE_CODE_RUNNER_CLAUDE_BIN`` names the program
(default ``claude``), looked up in the cleaned ``PATH`` like the runner does.
"""

from __future__ import annotations

import os
import subprocess
from pathlib import Path

from claude_code_runner.environment import clean_path, find_program
from claude_code_runner.failures import ERROR_TEXTS
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION, parse_version
from claude_code_runner.streamreader import KNOWN_TYPES

# Every member and value the runner reads from the stream (09 18.4, 18.5, 18.9; M2-34 mapping).
READ_TOKENS = (
    *sorted(KNOWN_TYPES),
    "init",
    "task_started",
    "task_progress",
    "task_updated",
    "task_notification",
    "background_tasks_changed",
    "api_retry",
    "compacting",
    "apiKeySource",
    "isReplay",
    "is_api_error_message",
    "is_error",
    "error_during_execution",
    "modelUsage",
    "contextWindow",
    "costUSD",
    "parent_tool_use_id",
    "tool_use_id",
    "compact_boundary",
    "user_message_uuids",
    "user_message_uuid",
    # M2-51: the run graph fields (tests/fixtures/recording).
    "task_type",
    "local_workflow",
    "total_tokens",
    "tool_uses",
    "duration_ms",
    # RUNNER-RESUME-COST: the transcript line a resume restores the session totals from.
    "cost-state",
    "totalCostUSD",
    # FIX-runner-responsive-turns: the members of the running background set.
    "tasks",
    "task_id",
    "ambient",
    "skip_transcript",
)
_WRAPPER_MAX_BYTES = 1 << 20  # a launcher script, not the program; read its package instead


def _program() -> Path:
    name = os.environ.get("CLAUDE_CODE_RUNNER_CLAUDE_BIN", "claude")
    found = find_program(name, clean_path(os.environ.get("PATH", "")))
    assert found is not None, "the Claude Code CLI is not installed (the switch test fails)"
    return Path(found).resolve()


def _cli_bytes(program: Path) -> bytes:
    if program.stat().st_size > _WRAPPER_MAX_BYTES:
        return program.read_bytes()
    package = program.parent.parent if program.parent.name == "bin" else program.parent
    return b"\n".join(p.read_bytes() for p in sorted(package.rglob("*")) if p.is_file())


def test_pinned_cli_knows_error_texts_and_events() -> None:
    program = _program()
    out = subprocess.run(
        [str(program), "--version"], capture_output=True, text=True, timeout=120, check=False
    )
    version = parse_version(out.stdout)
    assert version is not None
    assert ".".join(map(str, version)) == PINNED_CLAUDE_CODE_VERSION
    data = _cli_bytes(program)
    missing = [e.fragment for e in ERROR_TEXTS if e.fragment.encode() not in data]
    assert missing == []
    absent = [t for t in READ_TOKENS if t.encode() not in data]
    assert absent == []
