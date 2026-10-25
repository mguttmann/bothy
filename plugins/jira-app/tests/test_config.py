# SPDX-License-Identifier: Apache-2.0
"""Configuration and secret files of the app (brief M2-50 Design note 3).

Proves: each missing required key, an unknown member, a duplicate member name, a bad duration,
a wrong type, a project key in lower case, a project listed twice and a credentials line
without ``:`` abort with a message that names key and file and never a value; a secret file
with mode ``0644`` aborts the start; a state directory with group access is refused;
``config.example.json`` loads; every ``jira_app.*`` default equals the "Default" column of
docs/operations/configuration.md; the reused keys (``sdk.http_timeout``, ``sdk.retry_*``,
``webhook.receiver_tolerance``) come from the SDK's `SdkSettings` and have no own default.
"""

from __future__ import annotations

import dataclasses
import inspect
import json
import logging
import uuid
from collections.abc import Callable
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.settings import SdkSettings, parse_duration

import jira_app.app as app_module
import jira_app.chat.receive as receive_module
from jira_app.__main__ import main
from jira_app.app import AppDeps, start_app
from jira_app.config import (
    DEFAULTS,
    KEYS,
    AppConfig,
    ConfigError,
    check_start_files,
    check_state_dir,
    load_config,
    read_jira_credentials,
)
from jira_app.jira.client import JiraClient
from jira_app.log import AppLog
from jira_app.worker.worker import Worker
from tests.fakes.chat import FakeChat
from tests.helpers import CREDENTIALS, SECRETS, SUP_FORUM, WEB_FORUM, ListHandler, write_secret

REPO = Path(__file__).resolve().parents[3]
APP_DIR = Path(__file__).resolve().parents[1]
REFERENCE = REPO / "docs" / "operations" / "configuration.md"

REQUIRED = (
    "listen",
    "server_url",
    "token_file",
    "webhook_secret_file",
    "jira_base_url",
    "jira_credentials_file",
    "projects",
    "state_dir",
)
REUSED = (
    "sdk.http_timeout",
    "sdk.retry_max_attempts",
    "sdk.retry_backoff_base",
    "sdk.retry_backoff_cap",
    "webhook.receiver_tolerance",
)


def canary() -> str:
    """A value that must never appear in a message."""
    return f"canary{uuid.uuid4().hex}"


def write_secrets(directory: Path) -> None:
    """Write the three secret files of a valid configuration with mode 0600."""
    write_secret(directory / "app-token", SECRETS["app_token"])
    write_secret(directory / "webhook-secret", SECRETS["webhook_secret"])
    write_secret(directory / "jira-credentials", CREDENTIALS)


def document(directory: Path) -> dict[str, Any]:
    """The members of a valid configuration whose files live in ``directory``."""
    return {
        "listen": "127.0.0.1:0",
        "server_url": "http://127.0.0.1:9",
        "token_file": str(directory / "app-token"),
        "webhook_secret_file": str(directory / "webhook-secret"),
        "jira_base_url": "http://127.0.0.1:9",
        "jira_credentials_file": str(directory / "jira-credentials"),
        "projects": [
            {"project": "SUP", "channel_id": SUP_FORUM, "issue_type": "Task"},
            {"project": "WEB", "channel_id": WEB_FORUM, "issue_type": "Bug"},
        ],
        "state_dir": str(directory / "state"),
    }


def write_config(directory: Path, members: dict[str, Any]) -> Path:
    """Write ``{"jira_app": members}`` and return the file."""
    file = directory / "config.json"
    file.write_text(json.dumps({"jira_app": members}), encoding="utf-8")
    return file


def config_error(file: Path) -> str:
    """Load ``file``, expect a `ConfigError` and return its message."""
    with pytest.raises(ConfigError) as caught:
        load_config(file)
    return str(caught.value)


def assert_names(message: str, key: str, file: Path, *hidden: str) -> None:
    """The message names the key and the file and none of ``hidden``."""
    assert key in message, message
    assert str(file) in message, message
    for value in hidden:
        assert value not in message, "a value leaked into the message"


def reference_rows() -> dict[str, list[str]]:
    """The rows of configuration.md whose key starts with ``jira_app.``, cells without ticks."""
    rows: dict[str, list[str]] = {}
    for line in REFERENCE.read_text(encoding="utf-8").splitlines():
        if not line.startswith("| `jira_app."):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        rows[cells[0].strip("`")] = cells
    return rows


# Required keys, unknown and duplicate members


def test_valid_document_loads_with_the_defaults(tmp_path: Path) -> None:
    config = load_config(write_config(tmp_path, document(tmp_path)))
    assert (config.listen_host, config.listen_port) == ("127.0.0.1", 0)
    assert [p.project for p in config.projects] == ["SUP", "WEB"]
    assert config.dpop_key_file == DEFAULTS["dpop_key_file"]
    assert config.max_body_bytes == DEFAULTS["max_body_bytes"]
    assert config.dedupe_window == parse_duration(str(DEFAULTS["dedupe_window"]))
    assert config.max_queued_jobs == DEFAULTS["max_queued_jobs"]


def test_required_keys_are_exactly_the_keys_without_default() -> None:
    assert tuple(k for k in KEYS if k not in DEFAULTS) == REQUIRED
    assert set(DEFAULTS) <= set(KEYS)


@pytest.mark.parametrize("key", REQUIRED)
def test_each_missing_required_key_aborts(tmp_path: Path, key: str) -> None:
    secret_host = canary()
    members = document(tmp_path)
    if key != "server_url":
        members["server_url"] = f"https://{secret_host}.example.com"
    del members[key]
    file = write_config(tmp_path, members)
    message = config_error(file)
    assert_names(message, f"jira_app.{key}", file, secret_host)
    assert "missing" in message


@pytest.mark.parametrize("key", [k for k in REQUIRED if k != "projects"])
def test_an_empty_required_text_aborts(tmp_path: Path, key: str) -> None:
    members = document(tmp_path)
    members[key] = ""
    file = write_config(tmp_path, members)
    assert_names(config_error(file), f"jira_app.{key}", file)


def test_unknown_member_aborts(tmp_path: Path) -> None:
    value = canary()
    members = document(tmp_path) | {"http_timeout": value}
    file = write_config(tmp_path, members)
    message = config_error(file)
    assert_names(message, "jira_app.http_timeout", file, value)
    assert "unknown" in message


def test_unknown_member_of_a_project_aborts(tmp_path: Path) -> None:
    value = canary()
    members = document(tmp_path)
    members["projects"][0]["colour"] = value
    file = write_config(tmp_path, members)
    message = config_error(file)
    assert_names(message, "jira_app.projects[0]", file, value)
    assert "colour" in message


def test_a_second_top_level_member_aborts(tmp_path: Path) -> None:
    file = tmp_path / "config.json"
    file.write_text(json.dumps({"jira_app": document(tmp_path), "other": {}}), encoding="utf-8")
    assert_names(config_error(file), "jira_app", file)


def test_duplicate_member_name_aborts(tmp_path: Path) -> None:
    first, second = canary(), canary()
    body = json.dumps(document(tmp_path))[1:-1]
    text = (
        '{"jira_app": {'
        + body
        + f', "dedupe_window": "{first}", "dedupe_window": "{second}"'
        + "}}"
    )
    file = tmp_path / "config.json"
    file.write_text(text, encoding="utf-8")
    message = config_error(file)
    assert_names(message, "dedupe_window", file, first, second)
    assert "duplicate" in message


def test_not_a_json_document_aborts_without_its_text(tmp_path: Path) -> None:
    value = canary()
    file = tmp_path / "config.json"
    file.write_text('{"jira_app": {"listen": "' + value, encoding="utf-8")
    message = config_error(file)
    assert str(file) in message
    assert value not in message


# Durations and types


@pytest.mark.parametrize(
    "text",
    [
        "1h30m",
        "72 hours",
        "-1h",
        "0s",
        "h",
        "1.5h",
        "01h",
        "106752d",
    ],
)
def test_bad_duration_aborts(tmp_path: Path, text: str) -> None:
    file = write_config(tmp_path, document(tmp_path) | {"dedupe_window": text})
    assert_names(config_error(file), "jira_app.dedupe_window", file)


@pytest.mark.parametrize(
    ("text", "seconds"),
    [
        ("250ms", 0.25),
        ("90s", 90.0),
        ("30m", 1800.0),
        ("72h", 259200.0),
        ("2d", 172800.0),
    ],
)
def test_durations_of_the_reference_load(tmp_path: Path, text: str, seconds: float) -> None:
    file = write_config(tmp_path, document(tmp_path) | {"dedupe_window": text})
    assert load_config(file).dedupe_window == seconds


def test_bad_duration_canary_is_not_in_the_message(tmp_path: Path) -> None:
    value = canary()
    file = write_config(tmp_path, document(tmp_path) | {"dedupe_window": value})
    assert_names(config_error(file), "jira_app.dedupe_window", file, value)


@pytest.mark.parametrize(
    ("key", "value"),
    [
        ("listen", 8091),
        ("server_url", ["https://chat.example.com"]),
        ("token_file", 1),
        ("dpop_key_file", False),
        ("max_body_bytes", "1048576"),
        ("max_body_bytes", 1.5),
        ("max_body_bytes", True),
        ("max_body_bytes", 0),
        ("max_queued_jobs", -1),
        ("dedupe_window", 72),
        ("projects", {"project": "SUP"}),
        ("projects", []),
    ],
)
def test_wrong_type_aborts(tmp_path: Path, key: str, value: object) -> None:
    file = write_config(tmp_path, document(tmp_path) | {key: value})
    assert_names(config_error(file), f"jira_app.{key}", file)


def test_wrong_type_canary_is_not_in_the_message(tmp_path: Path) -> None:
    value = canary()
    file = write_config(tmp_path, document(tmp_path) | {"max_queued_jobs": value})
    assert_names(config_error(file), "jira_app.max_queued_jobs", file, value)


def test_bad_listen_and_urls_abort_without_their_value(tmp_path: Path) -> None:
    host = canary()
    cases: list[tuple[str, str]] = [
        ("listen", f"{host}"),
        ("server_url", f"http://{host}.example.com"),
        ("jira_base_url", f"ftp://{host}.example.com"),
        ("server_url", f"https://chat.example.com/{host}"),
        ("jira_base_url", f"https://jira.example.com/?q={host}"),
    ]
    for key, value in cases:
        file = write_config(tmp_path, document(tmp_path) | {key: value})
        assert_names(config_error(file), f"jira_app.{key}", file, host)


# Projects


def test_project_key_in_lower_case_aborts(tmp_path: Path) -> None:
    members = document(tmp_path)
    members["projects"][1]["project"] = "web"
    file = write_config(tmp_path, members)
    message = config_error(file)
    assert_names(message, "jira_app.projects[1]", file)
    assert "project must match" in message
    assert '"web"' not in message


def test_project_key_canary_is_not_in_the_message(tmp_path: Path) -> None:
    value = canary()
    members = document(tmp_path)
    members["projects"][0]["project"] = value
    file = write_config(tmp_path, members)
    assert_names(config_error(file), "jira_app.projects[0]", file, value)


def test_project_listed_twice_aborts(tmp_path: Path) -> None:
    members = document(tmp_path)
    members["projects"][1]["project"] = "SUP"
    file = write_config(tmp_path, members)
    message = config_error(file)
    assert_names(message, "jira_app.projects[1]", file)
    assert "listed twice" in message


def test_bad_channel_and_issue_type_abort_without_their_value(tmp_path: Path) -> None:
    value = canary()
    for member, bad in (("channel_id", value), ("issue_type", f"Task{chr(0x202E)}{value}")):
        members = document(tmp_path)
        members["projects"][0][member] = bad
        file = write_config(tmp_path, members)
        message = config_error(file)
        assert_names(message, "jira_app.projects[0]", file, value)
        assert member in message


def test_channel_id_is_kept_in_lower_case(tmp_path: Path) -> None:
    members = document(tmp_path)
    members["projects"][0]["channel_id"] = SUP_FORUM.upper()
    config = load_config(write_config(tmp_path, members))
    assert config.projects[0].channel_id == SUP_FORUM


# Secret files and the state directory


def test_credentials_line_without_colon_aborts(tmp_path: Path) -> None:
    value = canary()
    file = write_secret(tmp_path / "jira-credentials", value)
    with pytest.raises(ConfigError) as caught:
        read_jira_credentials(str(file))
    message = str(caught.value)
    assert "jira_app.jira_credentials_file" in message
    assert str(file) in message
    assert value not in message


@pytest.mark.parametrize("line", ["sam@example.com:", ":token", "sam@example.com:a\nb"])
def test_credentials_line_with_an_empty_part_or_two_lines_aborts(tmp_path: Path, line: str) -> None:
    file = write_secret(tmp_path / "jira-credentials", line)
    with pytest.raises(ConfigError, match=r"jira_app\.jira_credentials_file"):
        read_jira_credentials(str(file))


def test_credentials_are_read_without_one_trailing_line_break(tmp_path: Path) -> None:
    file = write_secret(tmp_path / "jira-credentials", CREDENTIALS + "\n")
    credentials = read_jira_credentials(str(file))
    assert credentials.line == CREDENTIALS
    assert SECRETS["jira_token"] not in repr(credentials)


def test_credentials_line_without_colon_aborts_the_start(tmp_path: Path) -> None:
    write_secrets(tmp_path)
    value = canary()
    write_secret(tmp_path / "jira-credentials", value)
    config = load_config(write_config(tmp_path, document(tmp_path)))
    with pytest.raises(ConfigError) as caught:
        check_start_files(config)
    assert "jira_app.jira_credentials_file" in str(caught.value)
    assert value not in str(caught.value)


@pytest.mark.parametrize(
    "key", ["token_file", "webhook_secret_file", "jira_credentials_file", "dpop_key_file"]
)
def test_secret_file_with_mode_0644_aborts_the_start(
    tmp_path: Path, key: str, capsys: pytest.CaptureFixture[str]
) -> None:
    write_secrets(tmp_path)
    members = document(tmp_path)
    if key == "dpop_key_file":
        write_secret(tmp_path / "dpop-key", SECRETS["app_token"])
        members["dpop_key_file"] = str(tmp_path / "dpop-key")
    secret = Path(members[key])
    secret.chmod(0o644)
    file = write_config(tmp_path, members)
    assert main(["--config", str(file)]) != 0
    err = capsys.readouterr().err
    assert f"jira_app.{key}" in err
    assert str(secret) in err
    assert "0400 or 0600" in err
    for value in SECRETS.values():
        assert value not in err, "a secret leaked into the message"
    assert not (tmp_path / "state").exists()


@pytest.mark.parametrize("mode", [0o400, 0o600])
def test_secret_file_with_a_private_mode_passes(tmp_path: Path, mode: int) -> None:
    write_secrets(tmp_path)
    (tmp_path / "app-token").chmod(mode)
    config = load_config(write_config(tmp_path, document(tmp_path)))
    check_start_files(config)
    assert (tmp_path / "state").is_dir()


@pytest.mark.parametrize("mode", [0o750, 0o710, 0o705])
def test_state_directory_with_group_or_other_access_is_refused(tmp_path: Path, mode: int) -> None:
    state = tmp_path / "state"
    state.mkdir()
    state.chmod(mode)
    with pytest.raises(ConfigError) as caught:
        check_state_dir(str(state))
    message = str(caught.value)
    assert "jira_app.state_dir" in message
    assert str(state) in message
    assert "group or other access" in message


def test_state_directory_with_group_access_aborts_the_start(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    write_secrets(tmp_path)
    state = tmp_path / "state"
    state.mkdir()
    state.chmod(0o770)
    file = write_config(tmp_path, document(tmp_path))
    assert main(["--config", str(file)]) != 0
    err = capsys.readouterr().err
    assert "jira_app.state_dir" in err
    assert str(state) in err
    assert not any(state.iterdir())


def test_new_state_directory_is_created_with_mode_0700(tmp_path: Path) -> None:
    state = tmp_path / "a" / "state"
    assert check_state_dir(str(state)) == state
    assert state.stat().st_mode & 0o777 == 0o700


# config.example.json


def test_config_example_loads_with_the_loader() -> None:
    example = APP_DIR / "config.example.json"
    config = load_config(example)
    assert isinstance(config, AppConfig)
    assert config.source == str(example)
    assert [p.project for p in config.projects] == ["SUP", "WEB"]
    raw = json.loads(example.read_text(encoding="utf-8"))["jira_app"]
    assert tuple(raw) == KEYS, "the example names every key in the order of the reference"


# Defaults against configuration.md


def test_every_default_equals_the_configuration_reference() -> None:
    rows = reference_rows()
    assert rows, (
        "docs/operations/configuration.md has no jira_app.* row yet: add the keys of "
        "brief M2-50 Design note 3 with Place 'Jira app' to docs/spec/11-extensions.md 16 "
        "and run make config-reference"
    )
    assert sorted(rows) == sorted(f"jira_app.{k}" for k in KEYS)
    for key in KEYS:
        cells = rows[f"jira_app.{key}"]
        place, default = cells[2], cells[4].strip("`")
        assert place == "Jira app", key
        if key not in DEFAULTS:
            assert default == "no default, required", key
            continue
        expected = DEFAULTS[key]
        if key == "dedupe_window":
            assert parse_duration(default) == parse_duration(str(expected)), key
        elif isinstance(expected, int):
            assert int(default) == expected, key
        else:
            assert (default if default != "empty" else "") == expected, key


def test_reused_keys_have_no_default_of_the_app() -> None:
    words = ("timeout", "retry", "backoff", "tolerance")
    own = list(KEYS) + [f.name for f in dataclasses.fields(AppConfig)]
    assert not [k for k in own if any(w in k for w in words)]
    sdk_keys = {f.metadata["key"] for f in dataclasses.fields(SdkSettings)}
    assert set(REUSED) <= sdk_keys
    assert AppDeps().settings is None


def test_verify_webhook_uses_the_sdk_tolerance() -> None:
    source = inspect.getsource(receive_module)
    assert "verify_webhook(" in source
    assert "tolerance" not in source, "the app must not pass its own receiver tolerance"


@pytest.mark.anyio
async def test_start_without_settings_uses_the_sdk_defaults(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    seen: dict[str, Any] = {}

    def spy(name: str, real: Callable[..., Any]) -> Callable[..., Any]:
        def build(*args: Any, **kwargs: Any) -> Any:
            seen[name] = args
            return real(*args, **kwargs)

        return build

    monkeypatch.setattr(app_module, "JiraClient", spy("jira", JiraClient))
    monkeypatch.setattr(app_module, "Worker", spy("worker", Worker))
    write_secrets(tmp_path)
    config = load_config(write_config(tmp_path, document(tmp_path)))
    running = await start_app(
        config, AppDeps(chat=FakeChat(), log=AppLog(ListHandler(), logging.DEBUG))
    )
    try:
        defaults = SdkSettings()
        assert seen["jira"][2] == defaults.http_timeout
        assert seen["worker"][3] == defaults.retry_backoff_cap
    finally:
        await running.stop()
