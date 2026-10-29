# SPDX-License-Identifier: Apache-2.0
"""Loading the runner configuration (design note 2, configuration.md 2 and 3)."""

from __future__ import annotations

from pathlib import Path

import pytest

from claude_code_runner.config import ConfigError, check_argument, load_config, parse_duration

INSTANCE = "0192d3a0-0000-7000-8000-0000000000a1"


def write(tmp_path: Path, runner: str = "", extra: str = "") -> Path:
    projects = tmp_path / "projects"
    state = tmp_path / "state"
    projects.mkdir(exist_ok=True)
    text = f"""
[runner]
server_url = "https://chat.example.com"
instance_id = "{INSTANCE}"
agent_token_ref = "file:/srv/secrets/agent_token"
default_model = "model-large"
projects_root = "{projects}"
state_dir = "{state}"
{runner}
{extra}
"""
    path = tmp_path / "runner.toml"
    path.write_text(text)
    return path


def test_defaults_and_required(tmp_path: Path) -> None:
    cfg = load_config(write(tmp_path))
    assert cfg.server_url == "https://chat.example.com"
    assert str(cfg.instance_id) == INSTANCE
    assert cfg.consumer == "default"
    assert cfg.tool_class == "chat"
    assert cfg.scoped_allowed_tools == ()
    assert cfg.anthropic_api_key_ref is None
    assert cfg.dpop_key_ref is None
    assert cfg.allow_personal is False
    assert cfg.forward_ssh_agent is False
    assert cfg.cli_probe_timeout == 30.0
    assert cfg.poll_interval == 0.2
    assert cfg.claude_bin == "claude"
    assert cfg.stop_grace == 10.0
    assert cfg.kill_grace == 5.0
    assert cfg.session_lock_wait == 600.0
    assert cfg.max_parallel_runs == 4
    assert cfg.stream_log_retention == 14 * 86400.0
    assert cfg.state_dir.is_dir()
    assert cfg.state_dir.stat().st_mode & 0o777 == 0o700


def test_sdk_and_adapter_keys(tmp_path: Path) -> None:
    extra = """
[adapter]
invocation_token_leeway = "5s"
jwks_refresh_min_interval = "2m"

[agent]
run_update_min_interval = "250ms"

[sdk]
http_timeout = "7s"
retry_max_attempts = 3
"""
    cfg = load_config(write(tmp_path, extra=extra))
    assert cfg.sdk.invocation_token_leeway == 5.0
    assert cfg.sdk.jwks_refresh_min_interval == 120.0
    assert cfg.sdk.run_update_min_interval == 0.25
    assert cfg.sdk.http_timeout == 7.0
    assert cfg.sdk.retry_max_attempts == 3


@pytest.mark.parametrize(
    ("runner", "key"),
    [
        ('surprise = "1"', "runner.surprise"),
        ('stop_grace = "ten seconds"', "runner.stop_grace"),
        ('stop_grace = "10w"', "runner.stop_grace"),
        ('max_parallel_runs = "4"', "runner.max_parallel_runs"),
        ('tool_class = "full"', "runner.tool_class"),
        ('scoped_allowed_tools = ["-rf"]', "runner.scoped_allowed_tools"),
        ('scoped_allowed_tools = ["Read\\u202e"]', "runner.scoped_allowed_tools"),
        ('consumer = "Bad Name"', "runner.consumer"),
        ('anthropic_api_key_ref = "vault:x"', "runner.anthropic_api_key_ref"),
        ('allow_personal = "yes"', "runner.allow_personal"),
    ],
)
def test_bad_values_name_key_and_file_not_value(tmp_path: Path, runner: str, key: str) -> None:
    path = write(tmp_path, runner=runner)
    with pytest.raises(ConfigError) as err:
        load_config(path)
    message = str(err.value)
    assert key in message
    assert str(path) in message
    for leaked in ("ten seconds", "10w", "-rf", "Bad Name", "vault:x", "yes"):
        assert leaked not in message


def test_missing_required(tmp_path: Path) -> None:
    path = tmp_path / "runner.toml"
    path.write_text('[runner]\nserver_url = "https://chat.example.com"\n')
    with pytest.raises(ConfigError, match=r"runner\.instance_id"):
        load_config(path)


def test_unknown_table(tmp_path: Path) -> None:
    with pytest.raises(ConfigError, match=r"other"):
        load_config(write(tmp_path, extra="[other]\nx = 1\n"))


@pytest.mark.parametrize(
    "url", ["http://chat.example.com", "ftp://chat.example.com", "chat.example.com"]
)
def test_server_url(tmp_path: Path, url: str) -> None:
    path = write(tmp_path)
    path.write_text(path.read_text().replace("https://chat.example.com", url))
    with pytest.raises(ConfigError, match=r"runner\.server_url"):
        load_config(path)


def test_loopback_http_allowed(tmp_path: Path) -> None:
    path = write(tmp_path)
    path.write_text(path.read_text().replace("https://chat.example.com", "http://127.0.0.1:8443"))
    assert load_config(path).server_url == "http://127.0.0.1:8443"


def test_state_dir_permissions(tmp_path: Path) -> None:
    state = tmp_path / "state"
    state.mkdir(mode=0o700)
    state.chmod(0o750)
    with pytest.raises(ConfigError, match=r"runner\.state_dir"):
        load_config(write(tmp_path))


def test_relative_and_missing_roots(tmp_path: Path) -> None:
    path = write(tmp_path)
    path.write_text(path.read_text().replace(str(tmp_path / "projects"), "projects"))
    with pytest.raises(ConfigError, match=r"runner\.projects_root"):
        load_config(path)
    path.write_text(path.read_text().replace('"projects"', f'"{tmp_path / "nope"}"'))
    with pytest.raises(ConfigError, match=r"runner\.projects_root"):
        load_config(path)


def test_bad_toml(tmp_path: Path) -> None:
    path = tmp_path / "runner.toml"
    path.write_text("[runner\n")
    with pytest.raises(ConfigError):
        load_config(path)


def test_parse_duration() -> None:
    assert parse_duration("200ms") == 0.2
    assert parse_duration("30s") == 30.0
    assert parse_duration("10m") == 600.0
    assert parse_duration("2h") == 7200.0
    assert parse_duration("14d") == 14 * 86400.0
    for bad in ("", "1", "s", "-1s", "1.5.2s", "1 s"):
        with pytest.raises(ValueError, match="duration"):
            parse_duration(bad)


def test_check_argument() -> None:
    assert check_argument("Bash(git status)")
    for bad in ("", "-x", "a\x00b", "a\nb", "a\u200fb", "a\u2066b"):
        assert not check_argument(bad)


def test_example_lists_every_runner_key() -> None:
    import dataclasses
    import tomllib

    from claude_code_runner.config import RunnerConfig

    example = tomllib.loads(
        (Path(__file__).resolve().parents[2] / "config.example.toml").read_text()
    )
    fields = {f.name for f in dataclasses.fields(RunnerConfig)} - {"source", "sdk"}
    assert set(example["runner"]) == fields


def test_provision_keys(tmp_path: Path) -> None:
    cfg = load_config(write(tmp_path))
    assert cfg.git_bin == "git"
    assert cfg.provision_timeout == 600.0
    assert cfg.provision_retry_after == 300.0
    cfg = load_config(
        write(
            tmp_path,
            'git_bin = "/usr/bin/git"\nprovision_timeout = "30s"\nprovision_retry_after = "1m"\n',
        )
    )
    assert (cfg.git_bin, cfg.provision_timeout, cfg.provision_retry_after) == (
        "/usr/bin/git",
        30.0,
        60.0,
    )
    with pytest.raises(ConfigError, match="git_bin"):
        load_config(write(tmp_path, 'git_bin = "-x"\n'))


def test_project_folders_max(tmp_path: Path) -> None:
    """``runner.project_folders_max`` (09 18.13, M4-44): default 500, at least 1."""
    assert load_config(write(tmp_path)).project_folders_max == 500
    assert load_config(write(tmp_path, "project_folders_max = 3\n")).project_folders_max == 3
    with pytest.raises(ConfigError, match="project_folders_max"):
        load_config(write(tmp_path, "project_folders_max = 0\n"))


def test_channels_resync_interval(tmp_path: Path) -> None:
    """``runner.channels_resync_interval``: the fallback reload of the served channels."""
    assert load_config(write(tmp_path)).channels_resync_interval == 300.0
    cfg = load_config(write(tmp_path, 'channels_resync_interval = "30s"\n'))
    assert cfg.channels_resync_interval == 30.0
    with pytest.raises(ConfigError, match="channels_resync_interval"):
        load_config(write(tmp_path, 'channels_resync_interval = "0s"\n'))


def test_transcribe_keys(tmp_path: Path) -> None:
    """``runner.transcribe_cmd`` (off by default) and ``runner.transcribe_timeout``."""
    cfg = load_config(write(tmp_path))
    assert cfg.transcribe_cmd == ()
    assert cfg.transcribe_timeout == 300.0
    cfg = load_config(
        write(
            tmp_path,
            'transcribe_cmd = ["/opt/whisper/transcribe", "-l", "auto"]\n'
            'transcribe_timeout = "90s"\n',
        )
    )
    assert cfg.transcribe_cmd == ("/opt/whisper/transcribe", "-l", "auto")
    assert cfg.transcribe_timeout == 90.0
    for bad in ('["-x"]', '[""]', '["whisper", ""]', '["whisper", "a\\u0007"]', '"whisper"'):
        with pytest.raises(ConfigError, match="transcribe_cmd"):
            load_config(write(tmp_path, f"transcribe_cmd = {bad}\n"))
    with pytest.raises(ConfigError, match="transcribe_timeout"):
        load_config(write(tmp_path, 'transcribe_timeout = "0s"\n'))
