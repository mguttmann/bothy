# SPDX-License-Identifier: Apache-2.0
"""The recognised texts and the stream messages exist in the installed, pinned CLI (RK-14).

Collected only behind the switch of this folder (``CLAUDE_CODE_RUNNER_REAL_CLI=1``, see
``tests/conftest.py``); with the switch on and no CLI installed it fails, it never skips. It
starts no session, uses no credential and calls no model: it prints the version and reads the
bytes of the CLI's files (design note 14). ``CLAUDE_CODE_RUNNER_CLAUDE_BIN`` names the program
(default ``claude``), looked up in the cleaned ``PATH`` like the runner does.

A change of the pin runs this file against the new CLI: the version probe reports ``online``,
the recorded stream field table equals a fresh read of the CLI's file (``record.py`` writes it),
and every text of the CLI's own "too old" pattern is one the runner recognises.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
from pathlib import Path

import pytest
from real_cli.record import RECORDING, cli_bytes, read_fields

from claude_code_runner.__main__ import main
from claude_code_runner.environment import clean_path, find_program
from claude_code_runner.failures import ERROR_TEXTS, ErrorKind
from claude_code_runner.heartbeat import (
    PINNED_CLAUDE_CODE_VERSION,
    parse_version,
    probe_version,
    runtime_status,
)
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
    # FIX-runner-hold-background: an input line the CLI has queued and not started yet.
    "command_uuid",
)
# The names behind the findings of 09 18.19.12, 18.9 and 26 5 that were read from the CLI's
# file; a name that left the CLI means its finding has to be read again.
FACT_TOKENS = (
    "CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS",
    "CLAUDE_CODE_STOP_HOOK_BLOCK_CAP",
    "BASH_DEFAULT_TIMEOUT_MS",
    "BASH_MAX_TIMEOUT_MS",
    "CLAUDE_CODE_MAX_TURNS",
    "API_TIMEOUT_MS",
    "rate_limit_event",
    "anthropic-ratelimit-unified-status",
    "scheduled_tasks.json",
    "mock-limits",
    "auto-approves every tool call (except explicit deny rules)",
    "supportsNonInteractive",
    # The fallback of a model the API refuses (README, runner.default_model).
    "model_fallback",
)
_TOO_OLD = re.compile(rb"\\bClaude Code \\S\+ \(\?:([^)]+)\)")
"""The CLI's own pattern of the API error kind ``claude_code_version_too_old``."""


def _program() -> Path:
    name = os.environ.get("CLAUDE_CODE_RUNNER_CLAUDE_BIN", "claude")
    found = find_program(name, clean_path(os.environ.get("PATH", "")))
    assert found is not None, "the Claude Code CLI is not installed (the switch test fails)"
    return Path(found).resolve()


@pytest.fixture(scope="module")
def data() -> bytes:
    return cli_bytes(_program())


def test_pinned_cli_knows_error_texts_and_events(data: bytes) -> None:
    program = _program()
    out = subprocess.run(
        [str(program), "--version"], capture_output=True, text=True, timeout=120, check=False
    )
    version = parse_version(out.stdout)
    assert version is not None
    assert ".".join(map(str, version)) == PINNED_CLAUDE_CODE_VERSION
    missing = [e.fragment for e in ERROR_TEXTS if e.fragment.encode() not in data]
    assert missing == []
    absent = [t for t in (*READ_TOKENS, *FACT_TOKENS) if t.encode() not in data]
    assert absent == []


@pytest.mark.anyio
async def test_pinned_cli_is_reported_online(tmp_path: Path) -> None:
    """The runner's own probe of the installed CLI: no degraded reason, state ``online``."""
    path = os.pathsep.join(clean_path(os.environ.get("PATH", "")))
    env = {"HOME": str(tmp_path), "PATH": path}
    probe = await probe_version(str(_program()), env, tmp_path, 120.0)
    assert (probe.version, probe.reasons) == (PINNED_CLAUDE_CODE_VERSION, ())
    assert runtime_status(probe, [], None).state == "online"


def test_check_config_reports_the_pinned_cli_ok(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    """``check-config`` of a runner whose ``claude_bin`` is the installed CLI: ``runtime ok``."""
    (tmp_path / "projects").mkdir()
    token = tmp_path / "token"
    token.write_text("agent-token-canary\n")
    token.chmod(0o600)
    config = tmp_path / "runner.toml"
    config.write_text(
        "[runner]\n"
        'server_url = "https://chat.example.com"\n'
        'instance_id = "0192d3a0-0000-7000-8000-0000000000a1"\n'
        f'agent_token_ref = "file:{token}"\n'
        'default_model = "claude-fable-5-2[1m]"\n'
        f'projects_root = "{tmp_path / "projects"}"\n'
        f'state_dir = "{tmp_path / "state"}"\n'
        f'claude_bin = "{_program()}"\n'
    )
    assert main(["check-config", "--config", str(config)]) == 0
    assert capsys.readouterr().out.splitlines()[-1] == "runtime ok"


def test_recorded_stream_fields_equal_the_installed_cli(data: bytes) -> None:
    """The field table of the recording is the one the installed CLI's file holds."""
    path = RECORDING / f"stream_fields_{PINNED_CLAUDE_CODE_VERSION}.json"
    recorded = json.loads(path.read_text("utf-8"))
    fresh = read_fields(data)
    for name, members in fresh.pop("messages").items():
        # The recording appends what the CLI wrote in the recorded turn beyond the schema.
        assert recorded["messages"][name][: len(members)] == members, name
    assert {key: recorded[key] for key in fresh} == fresh


def test_every_text_of_the_too_old_pattern_is_recognised(data: bytes) -> None:
    """The runner's fragments cover each branch of the CLI's "too old" pattern (09 18.9)."""
    patterns = {found.decode() for found in _TOO_OLD.findall(data)}
    assert len(patterns) == 1, patterns
    fragments = [e.fragment for e in ERROR_TEXTS if e.kind is ErrorKind.VERSION_UNSUPPORTED]
    for branch in patterns.pop().split("|"):
        assert any(fragment in branch for fragment in fragments), branch
