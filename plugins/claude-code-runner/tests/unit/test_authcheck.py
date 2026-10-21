# SPDX-License-Identifier: Apache-2.0
"""The pre-check table of design note 8 (09 18.3, 10 7, AG-11, BR-05, BR-12)."""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

from claude_code_runner.authcheck import (
    auth_status,
    decide,
    evidence,
    key_type_for,
    parse_auth_method,
)


@pytest.mark.parametrize(
    ("desired", "method", "outcome"),
    [
        ("organization", "api_key", "start"),
        ("personal", "claude.ai", "start"),
        ("personal", "oauth_token", "start"),
        ("organization", "none", "runtime_not_authenticated"),
        ("personal", "none", "runtime_not_authenticated"),
        ("organization", "claude.ai", "billing_mode_mismatch"),
        ("organization", "oauth_token", "billing_mode_mismatch"),
        ("organization", "api_key_helper", "billing_mode_mismatch"),
        ("organization", "third_party", "billing_mode_mismatch"),
        ("personal", "api_key", "billing_mode_mismatch"),
        ("organization", "unknown", "billing_mode_mismatch"),
        ("personal", "unknown", "billing_mode_mismatch"),
    ],
)
def test_table(desired: str, method: str, outcome: str) -> None:
    assert decide(desired, method) == outcome


@pytest.mark.parametrize(
    ("method", "key_type"),
    [
        ("api_key", "api_key"),
        ("api_key_helper", "api_key"),
        ("claude.ai", "oauth"),
        ("oauth_token", "oauth"),
        ("third_party", "cloud"),
        ("none", "none"),
        ("unknown", "unknown"),
        ("other", "unknown"),
    ],
)
def test_key_type(method: str, key_type: str) -> None:
    assert key_type_for(method) == key_type
    ev = evidence(method).to_dict()
    assert ev == {"provider": "anthropic", "key_type": key_type, "method": "claude_auth_status"}


@pytest.mark.parametrize(
    "raw",
    [
        b"",
        b"not json",
        b"[]",
        b'{"loggedIn": true}',
        b'{"authMethod": "magic"}',
        b'{"authMethod": 1}',
    ],
)
def test_parse_unknown(raw: bytes) -> None:
    assert parse_auth_method(raw) == "unknown"


def test_parse_known() -> None:
    assert parse_auth_method(json.dumps({"loggedIn": True, "authMethod": "api_key"})) == "api_key"


def script(tmp_path: Path, body: str) -> str:
    path = tmp_path / "claude"
    path.write_text(f"#!{sys.executable}\n{body}\n")
    path.chmod(0o755)
    return str(path)


@pytest.mark.anyio
async def test_auth_status_none_with_exit_1(tmp_path: Path) -> None:
    prog = script(
        tmp_path,
        "import json, sys\n"
        "print(json.dumps({'loggedIn': False, 'authMethod': 'none'}))\n"
        "sys.exit(1)",
    )
    method = await auth_status(prog, {"PATH": "/usr/bin"}, tmp_path, 5.0)
    assert method == "none"
    assert decide("organization", method) == "runtime_not_authenticated"


@pytest.mark.anyio
async def test_auth_status_time_limit_and_start_failure(tmp_path: Path) -> None:
    prog = script(tmp_path, "import time\ntime.sleep(30)")
    assert await auth_status(prog, {}, tmp_path, 0.3) == "unknown"
    assert await auth_status(str(tmp_path / "missing"), {}, tmp_path, 1.0) == "unknown"


@pytest.mark.anyio
async def test_auth_status_passes_env_and_args(tmp_path: Path) -> None:
    out = tmp_path / "seen.json"
    prog = script(
        tmp_path,
        "import json, os, sys\n"
        "seen = {'argv': sys.argv[1:], 'env': dict(os.environ), 'cwd': os.getcwd()}\n"
        f"open({str(out)!r}, 'w').write(json.dumps(seen))\n"
        "print(json.dumps({'authMethod': 'api_key'}))",
    )
    assert await auth_status(prog, {"ANTHROPIC_API_KEY": "k"}, tmp_path, 5.0) == "api_key"
    seen = json.loads(out.read_text())
    assert seen["argv"] == ["auth", "status"]
    assert seen["env"]["ANTHROPIC_API_KEY"] == "k"
    assert Path(seen["cwd"]).resolve() == tmp_path.resolve()
