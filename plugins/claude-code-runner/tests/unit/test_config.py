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


def test_trash_check_interval(tmp_path: Path) -> None:
    """``runner.trash_check_interval`` (09 18.15): default 60s, above zero."""
    assert load_config(write(tmp_path)).trash_check_interval == 60.0
    cfg = load_config(write(tmp_path, 'trash_check_interval = "5m"\n'))
    assert cfg.trash_check_interval == 300.0
    with pytest.raises(ConfigError, match="trash_check_interval"):
        load_config(write(tmp_path, 'trash_check_interval = "0s"\n'))


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


def test_search_assist_keys(tmp_path: Path) -> None:
    """``runner.search_assist_model`` and ``runner.search_assist_timeout`` (09 18.17)."""
    cfg = load_config(write(tmp_path))
    assert cfg.search_assist_model == "haiku"
    assert cfg.search_assist_timeout == 9.0
    cfg = load_config(
        write(tmp_path, 'search_assist_model = "sonnet"\nsearch_assist_timeout = "5s"\n')
    )
    assert (cfg.search_assist_model, cfg.search_assist_timeout) == ("sonnet", 5.0)
    with pytest.raises(ConfigError, match="search_assist_model"):
        load_config(write(tmp_path, 'search_assist_model = "-x"\n'))
    with pytest.raises(ConfigError, match="search_assist_timeout"):
        load_config(write(tmp_path, 'search_assist_timeout = "0s"\n'))


def test_agent_builder_keys(tmp_path: Path) -> None:
    """``runner.agent_builder``, ``runner.agent_builder_model`` and
    ``runner.agent_builder_timeout`` (26 6.2): off by default."""
    cfg = load_config(write(tmp_path, ""))
    assert cfg.agent_builder is False
    assert cfg.agent_builder_model == "sonnet"
    assert cfg.agent_builder_timeout == 80.0
    cfg = load_config(
        write(
            tmp_path,
            'agent_builder = true\nagent_builder_model = "opus"\nagent_builder_timeout = "60s"\n',
        )
    )
    assert (cfg.agent_builder, cfg.agent_builder_model, cfg.agent_builder_timeout) == (
        True,
        "opus",
        60.0,
    )
    with pytest.raises(ConfigError, match="agent_builder_model"):
        load_config(write(tmp_path, 'agent_builder_model = "-x"\n'))
    with pytest.raises(ConfigError, match="agent_builder_timeout"):
        load_config(write(tmp_path, 'agent_builder_timeout = "0s"\n'))
    with pytest.raises(ConfigError, match="agent_builder"):
        load_config(write(tmp_path, 'agent_builder = "yes"\n'))


def test_default_workspace_is_off_by_default(tmp_path: Path) -> None:
    """RUNNER-NO-GIT: a forum without a binding gets no folder unless the operator names one."""
    assert load_config(write(tmp_path)).default_workspace is None


@pytest.mark.parametrize("value", [".", "scratch", "team/notes"])
def test_default_workspace_accepted(tmp_path: Path, value: str) -> None:
    cfg = load_config(write(tmp_path, runner=f'default_workspace = "{value}"'))
    assert cfg.default_workspace == value


def test_default_workspace_empty_is_off(tmp_path: Path) -> None:
    cfg = load_config(write(tmp_path, runner='default_workspace = ""'))
    assert cfg.default_workspace is None


@pytest.mark.parametrize("value", ["/srv/projects", "../x", "-x", "~x", "a/../../b"])
def test_default_workspace_outside_the_root_refused(tmp_path: Path, value: str) -> None:
    with pytest.raises(ConfigError) as err:
        load_config(write(tmp_path, runner=f'default_workspace = "{value}"'))
    assert err.value.key == "runner.default_workspace"


def test_hosting_keys_default_off(tmp_path: Path) -> None:
    # 26 14: every key of stage A1 is off by default (26 11 item 2).
    cfg = load_config(write(tmp_path))
    assert cfg.hosted_agents is False
    assert cfg.hosted_agents_root is None
    assert cfg.hosted_max_tool_class == "scoped"


def test_hosting_keys_read(tmp_path: Path) -> None:
    root = tmp_path / "agents"
    runner = (
        f'hosted_agents = true\nhosted_agents_root = "{root}"\nhosted_max_tool_class = "host"\n'
        'dpop_key_ref = "file:/srv/secrets/dpop.pem"'
    )
    cfg = load_config(write(tmp_path, runner))
    assert cfg.hosted_agents is True
    assert cfg.hosted_agents_root == root
    assert cfg.hosted_max_tool_class == "host"


def test_hosting_root_empty_is_unset(tmp_path: Path) -> None:
    cfg = load_config(write(tmp_path, 'hosted_agents_root = ""'))
    assert cfg.hosted_agents_root is None


@pytest.mark.parametrize("value", ["full", "", "Host", "scoped "])
def test_hosted_max_tool_class_refuses_an_unknown_class(tmp_path: Path, value: str) -> None:
    with pytest.raises(ConfigError) as err:
        load_config(write(tmp_path, f'hosted_max_tool_class = "{value}"'))
    assert err.value.key == "runner.hosted_max_tool_class"


@pytest.mark.parametrize(
    "runner",
    ['hosted_agents = "yes"', "hosted_agents_root = 3", "hosted_max_tool_class = 1"],
)
def test_hosting_keys_wrong_type(tmp_path: Path, runner: str) -> None:
    with pytest.raises(ConfigError) as err:
        load_config(write(tmp_path, runner))
    assert err.value.key.startswith("runner.hosted_")


def test_hosting_needs_the_dpop_key(tmp_path: Path) -> None:
    # 26 3.3 item 1: a hosted token is bound to the runner's DPoP key; without one the server
    # refuses every token (dpop_required), so the start names the missing key instead.
    with pytest.raises(ConfigError) as err:
        load_config(write(tmp_path, "hosted_agents = true"))
    assert err.value.key == "runner.dpop_key_ref"
    assert "hosted_agents" in err.value.problem


# ---- A1-05: the hosted agents root and the keys of the definition turns (26 5.1, 5.9, 14) ----

DPOP = 'dpop_key_ref = "file:/srv/secrets/dpop.pem"'


def _hosted(tmp_path: Path, root: Path, extra: str = "") -> Path:
    runner = f'hosted_agents = true\nhosted_agents_root = "{root}"\n{DPOP}\n{extra}'
    return write(tmp_path, runner)


def test_new_keys_default_off(tmp_path: Path) -> None:
    cfg = load_config(write(tmp_path))
    assert cfg.mcp_stdio_runners == ("npx", "uvx")
    assert cfg.channel_skills_dir is None


def test_hosted_agents_need_the_root(tmp_path: Path) -> None:
    with pytest.raises(ConfigError) as err:
        load_config(write(tmp_path, f"hosted_agents = true\n{DPOP}"))
    assert err.value.key == "runner.hosted_agents_root"


def test_a_missing_root_is_created_private(tmp_path: Path) -> None:
    root = tmp_path / "agents"
    cfg = load_config(_hosted(tmp_path, root))
    assert cfg.hosted_agents_root == root
    assert root.stat().st_mode & 0o777 == 0o700


@pytest.mark.parametrize(
    "place",
    ["state", "state/agents", "projects", "projects/agents", ".", "relative"],
)
def test_root_refused_at_state_or_projects(tmp_path: Path, place: str) -> None:
    # BA-T10: equal to, inside or above runner.state_dir or runner.projects_root.
    (tmp_path / "state").mkdir(mode=0o700)
    root = Path("agents") if place == "relative" else tmp_path / place
    with pytest.raises(ConfigError) as err:
        load_config(_hosted(tmp_path, root))
    assert err.value.key == "runner.hosted_agents_root"


@pytest.mark.parametrize("where", ["root", "parent", "grandparent"])
def test_root_refused_below_a_claude_md(tmp_path: Path, where: str) -> None:
    # BA-T10: a CLAUDE.md in the root or above it would reach every agent's prompt.
    base = tmp_path / "hosting"
    root = base / "a" / "agents"
    root.mkdir(parents=True, mode=0o700)
    holder = {"root": root, "parent": root.parent, "grandparent": base}[where]
    (holder / "CLAUDE.md").write_text("Be brief.\n")
    with pytest.raises(ConfigError) as err:
        load_config(_hosted(tmp_path, root))
    assert err.value.key == "runner.hosted_agents_root"
    assert "CLAUDE.md" in err.value.problem


@pytest.mark.parametrize("mode", [0o720, 0o702, 0o777])
def test_root_refused_when_others_may_write(tmp_path: Path, mode: int) -> None:
    # BA-T10: writable by group or others.
    root = tmp_path / "agents"
    root.mkdir()
    root.chmod(mode)
    with pytest.raises(ConfigError) as err:
        load_config(_hosted(tmp_path, root))
    assert err.value.key == "runner.hosted_agents_root"


def test_root_readable_by_group_is_taken(tmp_path: Path) -> None:
    root = tmp_path / "agents"
    root.mkdir()
    root.chmod(0o750)
    assert load_config(_hosted(tmp_path, root)).hosted_agents_root == root


def test_root_unchecked_without_hosted_agents(tmp_path: Path) -> None:
    # Without runner.hosted_agents the key is not used, so it never stops a start (26 11).
    cfg = load_config(write(tmp_path, f'hosted_agents_root = "{tmp_path / "state"}"'))
    assert cfg.hosted_agents is False


def _secrets(tmp_path: Path, secrets: Path, extra: str = "") -> Path:
    return _hosted(tmp_path, tmp_path / "agents", f'hosted_secrets_dir = "{secrets}"\n{extra}')


def test_hosted_secrets_dir_default_unset(tmp_path: Path) -> None:
    assert load_config(_hosted(tmp_path, tmp_path / "agents")).hosted_secrets_dir is None


def test_hosted_secrets_dir_read(tmp_path: Path) -> None:
    secrets = tmp_path / "hosted-secrets"
    secrets.mkdir(mode=0o700)
    assert load_config(_secrets(tmp_path, secrets)).hosted_secrets_dir == secrets


@pytest.mark.parametrize(
    "place", ["missing", "relative", "group_writable", "link", "agents", "state", "projects"]
)
def test_hosted_secrets_dir_refused(tmp_path: Path, place: str) -> None:
    # 26 5.1 item 3: an absolute private folder apart from the root, the state and the projects.
    (tmp_path / "state").mkdir(mode=0o700)
    (tmp_path / "agents").mkdir(mode=0o700)
    real = tmp_path / "hosted-secrets"
    real.mkdir(mode=0o700)
    secrets = {
        "missing": tmp_path / "nowhere",
        "relative": Path("hosted-secrets"),
        "group_writable": real,
        "link": tmp_path / "link",
        "agents": tmp_path / "agents" / "secrets",
        "state": tmp_path / "state",
        "projects": tmp_path / "projects",
    }[place]
    if place == "group_writable":
        real.chmod(0o770)
    if place == "link":
        secrets.symlink_to(real)
    if place == "agents":
        secrets.mkdir(mode=0o700)
    with pytest.raises(ConfigError) as err:
        load_config(_secrets(tmp_path, secrets))
    assert err.value.key == "runner.hosted_secrets_dir"


@pytest.mark.parametrize("key", ["agent_token_ref", "dpop_key_ref", "anthropic_api_key_ref"])
def test_hosted_secrets_dir_refused_around_a_runner_credential(tmp_path: Path, key: str) -> None:
    # The runner's own credentials never lie where a definition may name a file.
    secrets = tmp_path / "hosted-secrets"
    (secrets / "a").mkdir(parents=True, mode=0o700)
    own = secrets / "a" / "credential"
    path = _secrets(tmp_path, secrets, "" if key == "agent_token_ref" else f'{key} = "file:{own}"')
    text = path.read_text()
    if key == "agent_token_ref":
        text = text.replace("file:/srv/secrets/agent_token", f"file:{own}")
    else:
        text = text.replace(f"{DPOP}\n", "") if key == "dpop_key_ref" else text
    path.write_text(text)
    with pytest.raises(ConfigError) as err:
        load_config(path)
    assert err.value.key == "runner.hosted_secrets_dir"


def test_mcp_stdio_runners_read(tmp_path: Path) -> None:
    cfg = load_config(write(tmp_path, 'mcp_stdio_runners = ["npx"]'))
    assert cfg.mcp_stdio_runners == ("npx",)


@pytest.mark.parametrize("value", ['["-x"]', '[""]', '["a b"]', '["npx/x"]', "3"])
def test_mcp_stdio_runners_refused(tmp_path: Path, value: str) -> None:
    with pytest.raises(ConfigError) as err:
        load_config(write(tmp_path, f"mcp_stdio_runners = {value}"))
    assert err.value.key == "runner.mcp_stdio_runners"


def test_channel_skills_dir_read(tmp_path: Path) -> None:
    skills = tmp_path / "skills"
    skills.mkdir()
    cfg = load_config(write(tmp_path, f'channel_skills_dir = "{skills}"'))
    assert cfg.channel_skills_dir == skills


@pytest.mark.parametrize("value", ["relative/skills", "/does/not/exist"])
def test_channel_skills_dir_refused(tmp_path: Path, value: str) -> None:
    with pytest.raises(ConfigError) as err:
        load_config(write(tmp_path, f'channel_skills_dir = "{value}"'))
    assert err.value.key == "runner.channel_skills_dir"
