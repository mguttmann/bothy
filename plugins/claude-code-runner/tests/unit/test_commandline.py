# SPDX-License-Identifier: Apache-2.0
"""The command line of a turn against 09 18.2 (R-AG-21, R-AG-23, D-W-16)."""

from __future__ import annotations

import re
import shlex
import uuid
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner.commandline import (
    ArgumentError,
    SessionChoice,
    build_argv,
    choose_model,
    choose_session,
    class_flags,
    reported_model,
    scoped_tools,
)

SPEC = Path(__file__).resolve().parents[4] / "docs" / "spec" / "09-agents.md"
PROMPT = Path("/var/lib/claude-code-runner/sessions/s/r.system-prompt.md")
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
    "model-large",
    "--session-id",
    SESSION.ref,
]
TAIL = ["--permission-prompts", "none"]
END = ["--append-system-prompt-file", str(PROMPT)]


def argv(tool_class: str, tools: list[str], add_dir: Path | None = None) -> list[str]:
    return build_argv(
        claude_bin="/usr/bin/claude",
        model="model-large",
        session=SESSION,
        tool_class=tool_class,
        tools=tools,
        add_dir=add_dir,
        system_prompt_file=PROMPT,
    )


def test_argv_chat_matches_spec_table() -> None:
    flags = ["--permission-mode", "dontAsk", "--tools", "", "--disallowedTools", "mcp__*"]
    assert argv("chat", []) == BASE + flags + TAIL + END


def test_argv_scoped_matches_spec_table() -> None:
    flags = ["--permission-mode", "dontAsk", "--allowedTools", "Read", "Bash(git status)"]
    assert argv("scoped", ["Read", "Bash(git status)"]) == BASE + flags + TAIL + END
    assert scoped_tools(["Read", "Grep", "Edit"], None) == ["Read", "Grep", "Edit"]
    assert scoped_tools(["Read", "Grep", "Edit"], ["Edit", "Read", "Write"]) == ["Read", "Edit"]
    assert scoped_tools(["Read"], ["Write"]) == []
    assert argv("scoped", []) == [*BASE, "--permission-mode", "dontAsk", *TAIL, *END]


def test_argv_host_matches_spec_table() -> None:
    flags = ["--permission-mode", "bypassPermissions"]
    assert argv("host", []) == BASE + flags + TAIL + END


def _spec_table() -> dict[str, str]:
    text = SPEC.read_text(encoding="utf-8")
    start = text.index("| Class | Flags | Effect |")
    rows: dict[str, str] = {}
    for line in text[start:].splitlines()[2:]:
        if not line.startswith("|"):
            break
        cells = [c.strip() for c in line.strip("|").split("|")]
        rows[cells[0].strip("`")] = cells[1].strip("`")
    return rows


def test_class_flags_equal_09_18_2_table() -> None:
    table = _spec_table()
    assert set(table) == {"chat", "scoped", "host"}
    tools = ["Read", "Grep"]
    for tool_class, cell in table.items():
        placeholders = re.findall(r"<[^>]+>", cell)
        expected = shlex.split(re.sub(r"<[^>]+>", "", cell))
        if tool_class == "scoped":
            # The placeholder names the tools of the configuration: one argument per tool.
            assert placeholders, "the scoped cell names the configured tools"
            expected += tools
        else:
            assert not placeholders
        assert class_flags(tool_class, tools) == expected


def test_session_flag() -> None:
    new = choose_session(None)
    assert new.new
    assert not new.resume
    assert uuid.UUID(new.ref).version == 4
    assert "--session-id" in build_argv(
        claude_bin="c",
        model="m",
        session=new,
        tool_class="chat",
        tools=[],
        add_dir=None,
        system_prompt_file=PROMPT,
    )
    ref = "0192d3a0-0000-7000-8000-0000000000aa"
    unstarted: Any = type("S", (), {"external_session_ref": ref, "started": False})()
    choice = choose_session(unstarted)
    assert (choice.ref, choice.resume, choice.new) == (ref, False, False)
    started: Any = type("S", (), {"external_session_ref": ref, "started": True})()
    resumed = choose_session(started)
    assert (resumed.ref, resumed.resume) == (ref, True)
    line = build_argv(
        claude_bin="c",
        model="m",
        session=resumed,
        tool_class="chat",
        tools=[],
        add_dir=None,
        system_prompt_file=PROMPT,
    )
    assert line[line.index("--resume") + 1] == ref
    assert "--session-id" not in line
    bad: Any = type("S", (), {"external_session_ref": "sess-7f3a", "started": True})()
    with pytest.raises(ArgumentError) as err:
        choose_session(bad)
    assert err.value.code == "session_ref_not_uuid"


def test_add_dir_only_scoped_and_host_with_files() -> None:
    inbox = Path("/var/lib/claude-code-runner/files/t")
    for tool_class in ("scoped", "host"):
        line = argv(tool_class, ["Read"], inbox)
        assert line[line.index("--add-dir") + 1] == str(inbox)
        assert line.index("--add-dir") == line.index("--permission-prompts") + 2
    with pytest.raises(ArgumentError):
        argv("chat", [], inbox)


def test_model_precedence() -> None:
    assert choose_model("model-small", "model-large") == "model-small"
    assert choose_model(None, "model-large") == "model-large"


def _model_pattern() -> re.Pattern[str]:
    """The pattern of `default_model` that 09 2.2 names (the server's model name check)."""
    lines = SPEC.read_text("utf-8").splitlines()
    row = next(line for line in lines if line.startswith("| `default_model`"))
    found = re.search(r"pattern `([^`]+)`", row)
    assert found is not None
    return re.compile(found.group(1))


@pytest.mark.parametrize(
    ("cli_model", "reported"),
    [
        ("model-large", "model-large"),
        # The CLI notation with a context window: the server gets the model name (09 2.2).
        ("claude-opus-5-5[1m]", "claude-opus-5-5"),
        ("claude-sonnet-5[200k]", "claude-sonnet-5"),
        ("opus", "opus"),
        # Nothing left that the server could take: the member is left out, the run goes on.
        ("[1m]", None),
        ("model large[1m]", None),
        ("a]b", None),
    ],
)
def test_reported_model_is_a_server_model_name(cli_model: str, reported: str | None) -> None:
    assert reported_model(cli_model) == reported
    if reported is not None:
        assert _model_pattern().fullmatch(reported)


def test_cli_notation_reaches_the_cli_unchanged() -> None:
    model = choose_model("claude-opus-5-5[1m]", "model-large")
    assert model == "claude-opus-5-5[1m]"
    line = build_argv(
        claude_bin="/usr/bin/claude",
        model=model,
        session=SESSION,
        tool_class="chat",
        tools=[],
        add_dir=None,
        system_prompt_file=PROMPT,
    )
    assert line[line.index("--model") + 1] == "claude-opus-5-5[1m]"


@pytest.mark.parametrize("value", ["-p", "--x", "", "a\nb", "a\u202eb"])
def test_values_starting_with_dash_rejected(value: str) -> None:
    with pytest.raises(ArgumentError):
        choose_model(value, "model-large")
    with pytest.raises(ArgumentError):
        argv("scoped", [value])
