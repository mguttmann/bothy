# SPDX-License-Identifier: Apache-2.0
"""The turn environment is an allow list (09 18.2, 18.3, BR-12, I5)."""

from __future__ import annotations

import os
import stat
from pathlib import Path

import pytest
from bothy_sdk.auth import Secret

from claude_code_runner.environment import (
    RunEnvironmentError,
    build_environment,
    clean_path,
    find_program,
)

FORBIDDEN = (
    "ANTHROPIC_AUTH_TOKEN",
    "CLAUDE_CODE_OAUTH_TOKEN",
    "ANTHROPIC_PROFILE",
    "CLAUDE_CODE_USE_BEDROCK",
    "CLAUDE_CODE_USE_VERTEX",
    "CLAUDE_CODE_USE_FOUNDRY",
)


def parent(tmp_path: Path) -> dict[str, str]:
    good = tmp_path / "bin"
    good.mkdir(exist_ok=True)
    env = {
        "HOME": str(tmp_path / "home"),
        "USER": "alex",
        "LOGNAME": "alex",
        "SHELL": "/bin/sh",
        "PATH": str(good),
        "LANG": "en_US.UTF-8",
        "LC_ALL": "en_US.UTF-8",
        "LC_CTYPE": "UTF-8",
        "TERM": "xterm",
        "TMPDIR": str(tmp_path),
        "SSH_AUTH_SOCK": "/tmp/agent.sock",
        "AWS_SECRET_ACCESS_KEY": "aws-canary",
        "GITHUB_TOKEN": "gh-canary",
        "BOTHY_SECRET_X": "bothy-canary",
        "ANTHROPIC_API_KEY": "parent-key",
        "CLAUDE_CONFIG_DIR": "/elsewhere",
    }
    env.update(dict.fromkeys(FORBIDDEN, "1"))
    return env


BASE_KEYS = {
    "HOME",
    "USER",
    "LOGNAME",
    "SHELL",
    "PATH",
    "LANG",
    "LC_ALL",
    "LC_CTYPE",
    "TERM",
    "TMPDIR",
    "CLAUDE_PRODUCT_RUN",
}


def test_exact_key_set_per_mode(tmp_path: Path) -> None:
    config_dir = tmp_path / "claude-config" / "organization"
    org = build_environment(
        parent(tmp_path),
        mode="organization",
        api_key=Secret("sk-canary"),
        config_dir=config_dir,
        forward_ssh_agent=False,
    )
    assert set(org) == BASE_KEYS | {"ANTHROPIC_API_KEY", "CLAUDE_CONFIG_DIR"}
    assert org["ANTHROPIC_API_KEY"] == "sk-canary"
    assert org["CLAUDE_CONFIG_DIR"] == str(config_dir)
    personal = build_environment(
        parent(tmp_path), mode="personal", api_key=None, config_dir=None, forward_ssh_agent=False
    )
    assert set(personal) == BASE_KEYS
    assert personal["HOME"] == str(tmp_path / "home")
    for env in (org, personal):
        for name in (*FORBIDDEN, "AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "BOTHY_SECRET_X"):
            assert name not in env


def test_turn_is_marked_as_product_run_even_from_a_bare_parent(tmp_path: Path) -> None:
    # User-level stop hooks hold a headless turn open while background tasks run unless the
    # turn carries this marker; the allow list drops the runner's own copy, so the runner sets it.
    bare = {"HOME": str(tmp_path / "home"), "PATH": ""}
    for mode, key, cfg in (
        ("personal", None, None),
        ("organization", Secret("sk-canary"), tmp_path / "cfg"),
    ):
        env = build_environment(
            bare, mode=mode, api_key=key, config_dir=cfg, forward_ssh_agent=False
        )
        assert env["CLAUDE_PRODUCT_RUN"] == "1"


def test_ssh_agent_only_with_switch(tmp_path: Path) -> None:
    env = build_environment(
        parent(tmp_path), mode="personal", api_key=None, config_dir=None, forward_ssh_agent=True
    )
    assert env["SSH_AUTH_SOCK"] == "/tmp/agent.sock"


def test_home_missing_aborts(tmp_path: Path) -> None:
    env = parent(tmp_path)
    del env["HOME"]
    with pytest.raises(RunEnvironmentError) as err:
        build_environment(
            env, mode="personal", api_key=None, config_dir=None, forward_ssh_agent=False
        )
    assert err.value.code == "home_missing"


def test_organization_needs_key_and_dir(tmp_path: Path) -> None:
    with pytest.raises(RunEnvironmentError):
        build_environment(
            parent(tmp_path),
            mode="organization",
            api_key=None,
            config_dir=tmp_path,
            forward_ssh_agent=False,
        )


def test_clean_path(tmp_path: Path) -> None:
    a = tmp_path / "a"
    b = tmp_path / "b"
    wide = tmp_path / "wide"
    group = tmp_path / "group"
    for d in (a, b, wide, group):
        d.mkdir()
    wide.chmod(0o777)
    group.chmod(0o775)
    a.chmod(0o755)
    b.chmod(0o755)
    value = os.pathsep.join(
        [
            "",
            ".",
            "relative/bin",
            str(tmp_path / "missing"),
            str(b),
            str(wide),
            str(group),
            str(a),
            str(b),
        ]
    )
    assert clean_path(value) == [str(b), str(a)]


def test_find_program(tmp_path: Path) -> None:
    d = tmp_path / "bin"
    d.mkdir()
    prog = d / "claude"
    prog.write_text("#!/bin/sh\n")
    prog.chmod(prog.stat().st_mode | stat.S_IXUSR)
    assert find_program("claude", [str(d)]) == str(prog)
    assert find_program("missing", [str(d)]) is None
    assert find_program(str(prog), []) == str(prog)
