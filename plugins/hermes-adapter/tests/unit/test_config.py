# SPDX-License-Identifier: Apache-2.0
"""Configuration from PlatformConfig.extra (09 17.2, 21; Design note 3)."""

from __future__ import annotations

import logging
import re
from pathlib import Path
from typing import Any

import pytest

from bothy_hermes_adapter.config import (
    DEFAULT_ATTACHMENT_TEXT_MAX_BYTES,
    DEFAULT_MAX_MESSAGE_BYTES,
    ConfigError,
    load_settings,
    load_token,
)

REPO = Path(__file__).resolve().parents[4]
CONFIG_DOC = REPO / "docs" / "operations" / "configuration.md"
INSTANCE = "0192f4a0-7b1c-7000-8000-000000000001"


def _extra(**over: Any) -> dict[str, Any]:
    extra: dict[str, Any] = {
        "hermes_adapter.server_url": "https://example.com/api/v1",
        "hermes_adapter.instance_id": INSTANCE,
        "hermes_adapter.token_file": "/srv/hermes/bothy/token",
    }
    extra.update(over)
    return {k: v for k, v in extra.items() if v is not None}


@pytest.mark.parametrize(
    "missing",
    ["hermes_adapter.server_url", "hermes_adapter.instance_id", "hermes_adapter.token_file"],
)
def test_config_requires_server_url_instance_id_and_token_file(missing: str) -> None:
    with pytest.raises(ConfigError, match=re.escape(missing)):
        load_settings(_extra(**{missing: None}))


def test_config_defaults_and_values() -> None:
    settings = load_settings(
        _extra(
            **{
                "sdk.http_timeout": "5s",
                "adapter.invocation_token_leeway": "2s",
                "hermes_adapter.dpop_key_file": "/srv/hermes/bothy/dpop.pem",
                "dm_policy": "allowlist",
            }
        )
    )
    assert (settings.platform_key, settings.consumer) == ("bothy", "default")
    assert settings.max_message_bytes == DEFAULT_MAX_MESSAGE_BYTES
    assert settings.attachment_text_max_bytes == DEFAULT_ATTACHMENT_TEXT_MAX_BYTES
    assert settings.sdk.http_timeout == 5.0
    assert settings.sdk.invocation_token_leeway == 2.0
    assert settings.dpop_key_file == Path("/srv/hermes/bothy/dpop.pem")


@pytest.mark.parametrize(
    ("key", "value"),
    [
        ("hermes_adapter.instance_id", "not-a-uuid"),
        ("hermes_adapter.platform_key", "Bad Key"),
        ("hermes_adapter.consumer", "-x"),
        ("hermes_adapter.max_message_bytes", 0),
        ("hermes_adapter.attachment_text_max_bytes", "many"),
        ("hermes_adapter.max_message_bytes", True),
        ("sdk.retry_max_attempts", "zero"),
        ("hermes_adapter.server_url", ""),
    ],
)
def test_config_rejects_invalid_values(key: str, value: Any) -> None:
    with pytest.raises(ConfigError) as caught:
        load_settings(_extra(**{key: value}))
    assert str(caught.value).startswith(key.split(".", maxsplit=1)[0])
    if isinstance(value, str) and len(value) > 3:
        assert value not in str(caught.value)


def test_config_rejects_unknown_key() -> None:
    with pytest.raises(ConfigError, match=r"hermes_adapter\.server_ulr: unknown"):
        load_settings(_extra(**{"hermes_adapter.server_ulr": "https://example.com"}))


def test_token_file_value_never_in_repr_or_logs(
    tmp_path: Path, caplog: pytest.LogCaptureFixture
) -> None:
    canary = "bothy_canary_" + "q" * 24
    path = tmp_path / "token"
    path.write_text(canary + "\n", encoding="utf-8")
    path.chmod(0o600)
    with caplog.at_level(logging.DEBUG):
        token = load_token(path)
        settings = load_settings(_extra(**{"hermes_adapter.token_file": str(path)}))
    assert token.reveal() == canary
    assert canary not in repr(token) + str(token) + repr(settings)
    assert canary not in caplog.text
    path.chmod(0o644)
    with pytest.raises(ConfigError) as open_file:
        load_token(path)
    assert canary not in str(open_file.value)
    empty = tmp_path / "empty"
    empty.write_text("\n", encoding="utf-8")
    empty.chmod(0o600)
    with pytest.raises(ConfigError, match="empty"):
        load_token(empty)
    with pytest.raises(ConfigError, match="not readable"):
        load_token(tmp_path / "missing")
    with pytest.raises(ConfigError, match="not a regular file"):
        load_token(tmp_path)


def _default_of(key: str) -> str:
    for line in CONFIG_DOC.read_text(encoding="utf-8").splitlines():
        cells = [c.strip() for c in line.split("|")]
        if len(cells) > 5 and cells[1] == f"`{key}`":
            return cells[5].strip("`")
    raise AssertionError(f"{key} is not in configuration.md")


def test_max_message_bytes_default_matches_server() -> None:
    assert int(_default_of("message.max_content_bytes")) == DEFAULT_MAX_MESSAGE_BYTES
    assert int(_default_of("hermes_adapter.max_message_bytes")) == DEFAULT_MAX_MESSAGE_BYTES
    assert int(_default_of("agent.context_message_max_bytes")) == (
        DEFAULT_ATTACHMENT_TEXT_MAX_BYTES
    )
    assert int(_default_of("hermes_adapter.attachment_text_max_bytes")) == (
        DEFAULT_ATTACHMENT_TEXT_MAX_BYTES
    )
