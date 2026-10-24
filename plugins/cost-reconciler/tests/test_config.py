# SPDX-License-Identifier: Apache-2.0
"""Configuration file: keys, defaults and errors that name the key, never the value."""

from __future__ import annotations

from pathlib import Path
from typing import Any

import pytest

from cost_reconciler.config import Config, ConfigError, load_config, parse_config, parse_duration

A1 = "0192f4a0-7b1c-7250-807f-c268a7b21791"
A2 = "0192f4a0-7b1c-7250-807f-c268a7b21792"
MARK = "VALUEMARK7"


def base(**reconciler: Any) -> dict[str, Any]:
    values: dict[str, Any] = {
        "server_url": "https://chat.example.com",
        "cost_api_url": "https://costs.example.com",
        "token_ref": "file:/run/secrets/token",
        "admin_key_ref": "env:ADMIN_KEY",
        "workspace_map": {"wrkspc_1": [A1], "wrkspc_2": [A2]},
    }
    values.update(reconciler)
    return {"reconciler": {k: v for k, v in values.items() if v is not None}}


def error_of(data: dict[str, Any]) -> ConfigError:
    with pytest.raises(ConfigError) as info:
        parse_config(data)
    return info.value


def test_defaults_of_chapter_17() -> None:
    config = parse_config(base())
    assert config.min_request_interval_ms == 60_000
    assert config.lookback_days == 3
    assert config.http_timeout_ms == 30_000
    assert config.server_url == "https://chat.example.com"
    assert config.workspace_of(A1) == "wrkspc_1"
    assert config.workspace_of("unknown") is None
    assert config.agent_ids == (A1, A2)


def test_explicit_values() -> None:
    config = parse_config(
        base(
            min_request_interval="500ms",
            lookback="72h",
            http_timeout="1m",
            server_url="https://chat.example.com/base/",
        )
    )
    assert config.min_request_interval_ms == 500
    assert config.lookback_days == 3
    assert config.http_timeout_ms == 60_000
    assert config.server_url == "https://chat.example.com/base"


def test_load_from_file(tmp_path: Path) -> None:
    path = tmp_path / "config.toml"
    path.write_text(
        "[reconciler]\n"
        'server_url = "https://chat.example.com"\n'
        'cost_api_url = "https://costs.example.com"\n'
        'token_ref = "file:/run/secrets/token"\n'
        'admin_key_ref = "file:/run/secrets/admin_key"\n'
        'lookback = "2d"\n'
        "[reconciler.workspace_map]\n"
        f'wrkspc_1 = ["{A1}", "{A2}"]\n',
        encoding="utf-8",
    )
    config = load_config(path)
    assert isinstance(config, Config)
    assert config.lookback_days == 2
    assert config.workspace_map == {"wrkspc_1": (A1, A2)}


def test_unreadable_and_invalid_files(tmp_path: Path) -> None:
    with pytest.raises(ConfigError) as info:
        load_config(tmp_path / "missing.toml")
    assert info.value.key == "--config"
    bad = tmp_path / "bad.toml"
    bad.write_text(f'[reconciler]\nserver_url = "{MARK}\n', encoding="utf-8")
    with pytest.raises(ConfigError) as info:
        load_config(bad)
    assert info.value.key == "--config"
    assert MARK not in str(info.value)


@pytest.mark.parametrize(
    ("data", "key"),
    [
        (base(server_url=None), "reconciler.server_url"),
        (base(cost_api_url=None), "reconciler.cost_api_url"),
        (base(token_ref=None), "reconciler.token_ref"),
        (base(admin_key_ref=None), "reconciler.admin_key_ref"),
        (base(workspace_map=None), "reconciler.workspace_map"),
        ({"reconciler": base()["reconciler"], "other": {"x": MARK}}, "other"),
        ({}, "reconciler"),
        ({"reconciler": MARK}, "reconciler"),
        (base(extra=MARK), "reconciler.extra"),
        (base(**{"Bad Key": MARK}), "reconciler"),
        (base(server_url=7), "reconciler.server_url"),
        (base(min_request_interval=f"{MARK}s"), "reconciler.min_request_interval"),
        (base(min_request_interval="1h30m"), "reconciler.min_request_interval"),
        (base(min_request_interval="01s"), "reconciler.min_request_interval"),
        (base(min_request_interval="-1s"), "reconciler.min_request_interval"),
        (base(min_request_interval="1.5s"), "reconciler.min_request_interval"),
        (base(min_request_interval="106752d"), "reconciler.min_request_interval"),
        (base(min_request_interval=60), "reconciler.min_request_interval"),
        (base(min_request_interval="0s"), "reconciler.min_request_interval"),
        (base(lookback="36h"), "reconciler.lookback"),
        (base(lookback="0d"), "reconciler.lookback"),
        (base(lookback="90m"), "reconciler.lookback"),
        (base(http_timeout="0s"), "reconciler.http_timeout"),
        (base(http_timeout="abc"), "reconciler.http_timeout"),
        (base(workspace_map={}), "reconciler.workspace_map"),
        (base(workspace_map=MARK), "reconciler.workspace_map"),
        (base(workspace_map={"w": [A1], "v": [A1]}), "reconciler.workspace_map"),
        (base(workspace_map={"w": [A1, A1]}), "reconciler.workspace_map"),
        (base(workspace_map={"w": []}), "reconciler.workspace_map"),
        (base(workspace_map={"w": [MARK]}), "reconciler.workspace_map"),
        (base(workspace_map={"w": A1}), "reconciler.workspace_map"),
        (base(workspace_map={"w": [A1.upper()]}), "reconciler.workspace_map"),
        (base(workspace_map={"": [A1]}), "reconciler.workspace_map"),
        (base(token_ref=f"vault:{MARK}"), "reconciler.token_ref"),
        (base(token_ref=MARK), "reconciler.token_ref"),
        (base(admin_key_ref=f"file:relative/{MARK}"), "reconciler.admin_key_ref"),
        (base(admin_key_ref=f"file:/run/../{MARK}"), "reconciler.admin_key_ref"),
        (base(admin_key_ref=f"env:lower{MARK}"), "reconciler.admin_key_ref"),
        (base(admin_key_ref=f"azkv://vault/{MARK}"), "reconciler.admin_key_ref"),
        (base(server_url=f"http://{MARK}.example.com"), "reconciler.server_url"),
        (base(server_url=f"ftp://{MARK}.example.com"), "reconciler.server_url"),
        (base(server_url=f"https://user:{MARK}@chat.example.com"), "reconciler.server_url"),
        (base(server_url=f"https://chat.example.com/#{MARK}"), "reconciler.server_url"),
        (base(server_url=f"https://chat.example.com/?k={MARK}"), "reconciler.server_url"),
        (base(server_url="https://chat.example.com:0"), "reconciler.server_url"),
        (base(server_url="https://chat.example.com:08443"), "reconciler.server_url"),
        (base(server_url="https://chat.example.com:99999"), "reconciler.server_url"),
        (base(server_url="https://"), "reconciler.server_url"),
        (base(cost_api_url=f"http://costs.example.net/{MARK}"), "reconciler.cost_api_url"),
        (base(cost_api_url="http://localhost.example.com"), "reconciler.cost_api_url"),
    ],
)
def test_errors_name_the_key_and_never_the_value(data: dict[str, Any], key: str) -> None:
    err = error_of(data)
    assert err.key == key
    assert MARK not in str(err)
    assert MARK not in repr(err)
    assert str(err).startswith(key)


@pytest.mark.parametrize(
    "url",
    [
        "http://127.0.0.1:8443",
        "http://127.5.6.7",
        "http://[::1]:9000/prefix",
        "http://localhost:1234",
        "https://costs.example.com",
        "https://costs.example.com:8443/v",
    ],
)
def test_http_only_for_loopback(url: str) -> None:
    config = parse_config(base(cost_api_url=url))
    assert config.cost_api_url == url.rstrip("/")


@pytest.mark.parametrize(
    ("text", "millis"),
    [
        ("0s", 0),
        ("250ms", 250),
        ("60s", 60_000),
        ("1m", 60_000),
        ("2h", 7_200_000),
        ("3d", 259_200_000),
        ("106751d", 106751 * 86_400_000),
        ("2562047h", 2562047 * 3_600_000),
    ],
)
def test_parse_duration(text: str, millis: int) -> None:
    assert parse_duration(text, "k.k") == millis


def test_repr_shows_references_not_values() -> None:
    config = parse_config(base())
    assert "/run/secrets/token" in repr(config)
