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


def test_config_error_exits_2(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    path = config(tmp_path, 'surprise = "secret-value"\n')
    assert main(["check-config", "--config", str(path)]) == 2
    err = capsys.readouterr().err
    assert "runner.surprise" in err
    assert "secret-value" not in err
