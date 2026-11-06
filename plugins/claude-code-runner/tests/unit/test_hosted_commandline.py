# SPDX-License-Identifier: Apache-2.0
"""The command line of a hosted turn against 26 5.3 (BA-T13).

The whole argument list per class: the class flags of 26 5.3 in place of those of 09 18.2, the
hosted additions directly after ``--permission-prompts none`` and before ``--add-dir``, and
``--max-budget-usd`` directly before ``--append-system-prompt-file``; with and without skills,
with and without MCP servers, with and without a cap. The definition's tool list only narrows
``runner.scoped_allowed_tools`` and keeps its order. The refusals of a definition before
``run.start`` are in ``tests/scenarios/test_hosted_turns.py``.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from claude_code_runner.commandline import (
    ArgumentError,
    HostedArgs,
    SessionChoice,
    budget_argument,
    build_argv,
    class_flags,
    hosted_class_flags,
)
from claude_code_runner.definition import intersect

PROMPT = Path("/state/hosted/a/sessions/s/r.system-prompt.md")
CONTROL = Path("/agents/a.control")
SETTINGS = CONTROL / "settings.json"
PLUGIN = CONTROL / "plugin"
TURN_MCP = CONTROL / "turns" / "r.mcp.json"
INBOX = Path("/state/hosted/a/files/t")
SESSION = SessionChoice(ref="0192d3a0-0000-4000-8000-000000000001", resume=False, new=True)
BASE = [
    "/usr/bin/claude",
    "-p",
    "--input-format",
    "stream-json",
    "--output-format",
    "stream-json",
    "--verbose",
    "--replay-user-messages",
    "--model",
    "model-small",
    "--session-id",
    SESSION.ref,
]
PROMPTS = ["--permission-prompts", "none"]
END = ["--append-system-prompt-file", str(PROMPT)]
RUNNER_TOOLS = ("Read", "Bash(git log:*)", "Edit", "WebFetch")


def hosted(
    tool_class: str,
    *,
    skills: bool = False,
    mcp: bool = False,
    cap: str | None = None,
    tools: list[str] | None = None,
    add_dir: Path | None = None,
    plugin_dirs: tuple[Path, ...] = (),
) -> list[str]:
    return build_argv(
        claude_bin="/usr/bin/claude",
        model="model-small",
        session=SESSION,
        tool_class=tool_class,
        tools=tools or [],
        add_dir=add_dir,
        system_prompt_file=PROMPT,
        hosted=HostedArgs(
            settings=SETTINGS,
            plugin=PLUGIN if skills else None,
            mcp_config=TURN_MCP if mcp else None,
            has_skills=skills,
        ),
        plugin_dirs=plugin_dirs,
        max_budget_usd=cap,
    )


def test_chat_without_skills() -> None:
    flags = ["--permission-mode", "dontAsk", "--tools", "", "--disallowedTools", "mcp__*"]
    added = ["--restricted", "--settings", str(SETTINGS), "--strict-mcp-config"]
    assert hosted("chat") == BASE + flags + PROMPTS + added + END


def test_chat_with_skills_and_a_cap() -> None:
    flags = ["--permission-mode", "dontAsk", "--tools", "Skill", "--allowedTools", "Skill"]
    flags += ["--disallowedTools", "mcp__*"]
    added = ["--restricted", "--settings", str(SETTINGS), "--plugin-dir", str(PLUGIN)]
    added += ["--strict-mcp-config"]
    cap = ["--max-budget-usd", "2.5"]
    assert hosted("chat", skills=True, cap="2.5") == BASE + flags + PROMPTS + added + cap + END


def test_chat_never_loads_mcp_servers() -> None:
    with pytest.raises(ArgumentError):
        hosted("chat", mcp=True)


def test_scoped_with_skills_mcp_files_and_a_cap() -> None:
    tools = intersect(RUNNER_TOOLS, ("Edit", "Read", "Write"))
    assert tools == ["Read", "Edit"]
    flags = ["--permission-mode", "dontAsk"]
    flags += ["--tools", "Read", "Grep", "Glob", "Bash", "Skill", "Edit"]
    flags += ["--allowedTools", "Read", "Edit", "Skill"]
    added = ["--restricted", "--settings", str(SETTINGS), "--plugin-dir", str(PLUGIN)]
    added += ["--mcp-config", str(TURN_MCP), "--strict-mcp-config"]
    tail = ["--add-dir", str(INBOX), "--max-budget-usd", "1"]
    line = hosted("scoped", skills=True, mcp=True, cap="1", tools=tools, add_dir=INBOX)
    assert line == BASE + flags + PROMPTS + added + tail + END


def test_scoped_without_skills_or_mcp() -> None:
    tools = intersect(RUNNER_TOOLS, None)
    flags = ["--permission-mode", "dontAsk"]
    flags += ["--tools", "Read", "Grep", "Glob", "Bash", "Bash(git log:*)", "Edit", "WebFetch"]
    flags += ["--allowedTools", *RUNNER_TOOLS]
    added = ["--restricted", "--settings", str(SETTINGS), "--strict-mcp-config"]
    assert hosted("scoped", tools=tools) == BASE + flags + PROMPTS + added + END


def test_scoped_with_an_empty_tool_list_names_no_allowed_tool() -> None:
    tools = intersect(RUNNER_TOOLS, ())
    flags = ["--permission-mode", "dontAsk", "--tools", "Read", "Grep", "Glob", "Bash"]
    added = ["--restricted", "--settings", str(SETTINGS), "--strict-mcp-config"]
    assert hosted("scoped", tools=tools) == BASE + flags + PROMPTS + added + END


def test_host_with_everything() -> None:
    flags = ["--permission-mode", "bypassPermissions"]
    added = ["--setting-sources", "project,local", "--settings", str(SETTINGS)]
    added += ["--plugin-dir", str(PLUGIN), "--plugin-dir", "/skills/review"]
    added += ["--mcp-config", str(TURN_MCP), "--strict-mcp-config"]
    tail = ["--add-dir", str(INBOX), "--max-budget-usd", "0.75"]
    line = hosted(
        "host",
        skills=True,
        mcp=True,
        cap="0.75",
        add_dir=INBOX,
        plugin_dirs=(Path("/skills/review"),),
    )
    assert line == BASE + flags + PROMPTS + added + tail + END


def test_host_without_skills_or_mcp() -> None:
    flags = ["--permission-mode", "bypassPermissions"]
    added = ["--setting-sources", "project,local", "--settings", str(SETTINGS)]
    added += ["--strict-mcp-config"]
    assert hosted("host") == BASE + flags + PROMPTS + added + END


@pytest.mark.parametrize("tool_class", ["chat", "scoped"])
def test_channel_skill_folders_never_below_host(tool_class: str) -> None:
    with pytest.raises(ArgumentError):
        hosted(tool_class, plugin_dirs=(Path("/skills/review"),))


def test_host_class_flags_are_those_of_09_18_2() -> None:
    assert hosted_class_flags("host", [], has_skills=True) == class_flags("host", [])


def test_the_runner_s_own_host_turn_takes_channel_skill_folders() -> None:
    # 26 5.9: the runner's own agent under host; nothing else of its line changes.
    line = build_argv(
        claude_bin="/usr/bin/claude",
        model="model-small",
        session=SESSION,
        tool_class="host",
        tools=[],
        add_dir=None,
        system_prompt_file=PROMPT,
        plugin_dirs=(Path("/skills/review"), Path("/skills/notes")),
    )
    flags = ["--permission-mode", "bypassPermissions"]
    added = ["--plugin-dir", "/skills/review", "--plugin-dir", "/skills/notes"]
    assert line == BASE + flags + PROMPTS + added + END


@pytest.mark.parametrize(
    ("value", "text"),
    [(2.5, "2.5"), (2.0, "2.0"), (50, "50"), (0.0000001, "0.0000001"), (12.34, "12.34")],
)
def test_budget_argument_is_a_plain_decimal(value: float, text: str) -> None:
    assert budget_argument(value) == text
