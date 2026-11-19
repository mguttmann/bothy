# SPDX-License-Identifier: Apache-2.0
"""The command line: ``check-config`` prints codes, never values (design note 2)."""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

from claude_code_runner.__main__ import main

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import helper


def config(tmp_path: Path, extra: str = "") -> Path:
    (tmp_path / "projects").mkdir()
    token = tmp_path / "token"
    token.write_text("agent-token-canary\n")
    token.chmod(0o600)
    path = tmp_path / "runner.toml"
    path.write_text(
        "[runner]\n"
        'server_url = "https://chat.example.com"\n'
        'instance_id = "0192d3a0-0000-7000-8000-0000000000a1"\n'
        f'agent_token_ref = "file:{token}"\n'
        f'anthropic_api_key_ref = "file:{tmp_path / "missing_key"}"\n'
        'default_model = "model-large"\n'
        f'projects_root = "{tmp_path / "projects"}"\n'
        f'state_dir = "{tmp_path / "state"}"\n'
        f'claude_bin = "{tmp_path / "bin" / "claude"}"\n' + extra
    )
    return path


def test_check_config_prints_codes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    helper.install(tmp_path / "bin", {}, tmp_path / "capture.jsonl")
    assert main(["check-config", "--config", str(config(tmp_path))]) == 1
    out = capsys.readouterr().out.splitlines()
    assert out == [
        "config ok",
        "runner.agent_token_ref ok",
        "runner.dpop_key_ref unset",
        "runner.anthropic_api_key_ref missing",
        "runtime ok",
    ]


@pytest.mark.parametrize(
    ("version", "runtime", "code"),
    [
        ("2.1.326", "runtime ok", 0),
        ("2.1.325", "runtime runtime_version_untested", 1),  # the pin before 2026-11-17
        ("2.1.321", "runtime runtime_version_untested", 1),  # the pin before 2026-11-16
        ("2.1.288", "runtime runtime_version_untested", 1),  # the pin before 2026-11-10
        ("2.1.327", "runtime runtime_version_untested", 1),  # a later version is untested too
    ],
)
def test_check_config_takes_cli_2_1_326_as_tested(
    tmp_path: Path, capsys: pytest.CaptureFixture[str], version: str, runtime: str, code: int
) -> None:
    """``check-config`` prints ``runtime ok`` for Claude Code 2.1.326 and for no other version."""
    key = tmp_path / "missing_key"
    key.write_text("api-key-canary\n")
    key.chmod(0o600)
    helper.install(tmp_path / "bin", {"version": version}, tmp_path / "capture.jsonl")
    assert main(["check-config", "--config", str(config(tmp_path))]) == code
    out = capsys.readouterr().out.splitlines()
    assert out == [
        "config ok",
        "runner.agent_token_ref ok",
        "runner.dpop_key_ref unset",
        "runner.anthropic_api_key_ref ok",
        runtime,
    ]


@pytest.mark.parametrize(
    ("version", "pin", "lines", "code"),
    [
        ("2.1.327", "2.1.326", ["runner.tested_claude_code_version ok", "runtime ok"], 0),
        (
            "2.1.328",
            "2.1.326",
            ["runner.tested_claude_code_version ok", "runtime runtime_version_untested"],
            1,
        ),
        (
            "2.1.327",
            "2.1.325",
            ["runner.tested_claude_code_version pin_changed", "runtime runtime_version_untested"],
            1,
        ),
        ("2.1.326", "2.1.325", ["runner.tested_claude_code_version pin_changed", "runtime ok"], 1),
    ],
)
def test_check_config_takes_the_tested_version_of_this_pin(
    tmp_path: Path,
    capsys: pytest.CaptureFixture[str],
    version: str,
    pin: str,
    lines: list[str],
    code: int,
) -> None:
    """The version the update chain proved is ``runtime ok`` while its entry names this pin.

    An entry proved against another pin is reported (``pin_changed``) and fails the check, so a
    deploy of a new pin removes it (FIX-RUNNER-TESTED-CLI).
    """
    key = tmp_path / "missing_key"
    key.write_text("api-key-canary\n")
    key.chmod(0o600)
    helper.install(tmp_path / "bin", {"version": version}, tmp_path / "capture.jsonl")
    entry = f'tested_claude_code_version = "2.1.327"\ntested_claude_code_pin = "{pin}"\n'
    assert main(["check-config", "--config", str(config(tmp_path, entry))]) == code
    out = capsys.readouterr().out.splitlines()
    assert out == [
        "config ok",
        "runner.agent_token_ref ok",
        "runner.dpop_key_ref unset",
        "runner.anthropic_api_key_ref ok",
        *lines,
    ]


def test_config_error_exits_2(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    path = config(tmp_path, 'surprise = "secret-value"\n')
    assert main(["check-config", "--config", str(path)]) == 2
    err = capsys.readouterr().err
    assert "runner.surprise" in err
    assert "secret-value" not in err
