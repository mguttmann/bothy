# SPDX-License-Identifier: Apache-2.0
"""The runner configuration: one TOML file (design note 2, docs/spec/09-agents.md 21).

Keys are nested by their dots: ``runner.stop_grace`` is ``stop_grace`` in the table
``[runner]``; the SDK keys ``adapter.invocation_token_leeway``,
``adapter.jwks_refresh_min_interval``, ``agent.run_update_min_interval`` and ``sdk.*`` are read
from the same file and handed to the SDK. Durations are a number with ``ms``, ``s``, ``m``,
``h`` or ``d`` (docs/operations/configuration.md 2). Unknown keys, wrong types, missing required
keys and invalid values abort the start with a `ConfigError` that names key and file, never a
value (configuration.md 3).

Examples:
    >>> parse_duration("200ms"), parse_duration("14d")
    (0.2, 1209600.0)
"""

from __future__ import annotations

import re
import tomllib
import unicodedata
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit
from uuid import UUID

from bothy_sdk.settings import SdkSettings

from claude_code_runner.reply_attachments import (
    DEFAULT_DENIED_PATTERNS,
    ROOT_NAMES,
    check_allowed_root_names,
    check_denied_patterns,
)
from claude_code_runner.secrets import SecretError, SecretRef, parse_secret_ref
from claude_code_runner.statedir import StateDirError, ensure_private_dir
from claude_code_runner.toolclass import ORDER
from claude_code_runner.workspace import WorkspaceError, resolve_default

__all__ = ["ConfigError", "RunnerConfig", "check_argument", "load_config", "parse_duration"]

_DURATION = re.compile(r"(\d+(?:\.\d+)?)(ms|s|m|h|d)")
_UNIT = {"ms": 0.001, "s": 1.0, "m": 60.0, "h": 3600.0, "d": 86400.0}
_CONSUMER = re.compile(r"[a-z0-9][a-z0-9_-]*")
_BIDI = frozenset("\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069")


class ConfigError(Exception):
    """An invalid configuration; the message names key and file, never the value.

    Attributes:
        key: The dotted key (or the table) that is wrong.
        file: The configuration file.
        problem: What is wrong, as a short phrase.
    """

    def __init__(self, key: str, file: Path, problem: str) -> None:
        """Create the error; see the class docstring."""
        super().__init__(f"{file}: {key}: {problem}")
        self.key = key
        self.file = file
        self.problem = problem


def parse_duration(text: str) -> float:
    """Parse a duration of the configuration reference into seconds.

    Args:
        text: A number with ``ms``, ``s``, ``m``, ``h`` or ``d``, for example ``"30s"``.

    Returns:
        The duration in seconds.

    Raises:
        ValueError: Not such a duration.
    """
    match = _DURATION.fullmatch(text)
    if match is None:
        raise ValueError("not a duration")
    return float(match.group(1)) * _UNIT[match.group(2)]


def check_argument(value: str) -> bool:
    """Tell whether a value may become one process argument of a turn (M2-03 note 4).

    Args:
        value: A model or tool name.

    Returns:
        False for an empty value, one starting with ``-`` (it would be read as a flag) and one
        with a control or bidi character.
    """
    if not value or value.startswith("-"):
        return False
    return not any(unicodedata.category(c).startswith("C") or c in _BIDI for c in value)


@dataclass(frozen=True)
class RunnerConfig:
    """The loaded configuration; durations in seconds.

    Attributes:
        source: The configuration file.
        server_url: ``runner.server_url``.
        instance_id: ``runner.instance_id``.
        agent_token_ref: ``runner.agent_token_ref``.
        dpop_key_ref: ``runner.dpop_key_ref`` (``None`` when empty).
        consumer: ``runner.consumer``.
        tool_class: ``runner.tool_class``.
        scoped_allowed_tools: ``runner.scoped_allowed_tools``.
        default_model: ``runner.default_model``.
        anthropic_api_key_ref: ``runner.anthropic_api_key_ref`` (``None`` when empty).
        allow_personal: ``runner.allow_personal``.
        forward_ssh_agent: ``runner.forward_ssh_agent``.
        cli_probe_timeout: ``runner.cli_probe_timeout``.
        poll_interval: ``runner.poll_interval``.
        claude_bin: ``runner.claude_bin``.
        projects_root: ``runner.projects_root``.
        state_dir: ``runner.state_dir``.
        followup_grace: ``runner.followup_grace`` (used from M2-35 on).
        followup_file_wait: ``runner.followup_file_wait``, the longest wait for the check of a
            follow-up's attachments before it is written (FIX-scan-state-stale).
        stop_grace: ``runner.stop_grace``.
        kill_grace: ``runner.kill_grace``.
        session_lock_wait: ``runner.session_lock_wait``.
        max_parallel_runs: ``runner.max_parallel_runs``.
        stream_log_retention: ``runner.stream_log_retention``.
        progress_patch_interval: ``runner.progress_patch_interval``: ``tool_progress`` becomes a
            patch of ``/metrics/progress_events`` only when the last patch of the turn is older
            (09 18.4, M2-51); keep it below the server's ``run.stall_after``.
        attach_allowed_roots: ``runner.attach_allowed_roots``: root names of answer attachments
            (``project``, ``thread_inbox``, ``runner_temp``), by name only (18.10, M3-12).
        attach_denied_patterns: ``runner.attach_denied_patterns``: denied patterns of answer
            attachments; always contains the list of 18.10 (M3-12).
        sdk: The SDK settings of the ``adapter``, ``agent`` and ``sdk`` keys.
        git_bin: ``runner.git_bin``: ``git`` of the folder creation (09 18.12, M4-43).
        provision_timeout: ``runner.provision_timeout``: time limit of one folder creation.
        provision_retry_after: ``runner.provision_retry_after``: a failed creation is not tried
            again for the same binding content before this time.
        project_folders_max: ``runner.project_folders_max``: most folder names the heartbeat
            reports in ``project_folders`` (09 18.13, M4-44); at least 1.
        channels_resync_interval: ``runner.channels_resync_interval``: the served channels are
            loaded again at this interval, the fallback for an operator or binding change whose
            event did not reach the runner (09 18.1); above zero.
        transcribe_cmd: ``runner.transcribe_cmd``: the argument list of a local transcriber; the
            path of an audio file is appended and the text on stdout is the transcript (09
            18.14). Empty: no transcription.
        transcribe_timeout: ``runner.transcribe_timeout``: time limit of one transcription.
        trash_check_interval: ``runner.trash_check_interval``: the bound folders are checked for
            a move to the trash at most this often, and a heartbeat that can carry a folder move
            request is sent at this interval (09 18.15); above zero.
        default_workspace: ``runner.default_workspace``: the folder of a forum without a binding
            (RUNNER-NO-GIT), ``.`` for the projects root or a reference under it; ``None`` (the
            default, empty in the file) leaves such a forum without a folder.
    """

    source: Path
    server_url: str
    instance_id: UUID
    agent_token_ref: SecretRef
    dpop_key_ref: SecretRef | None
    consumer: str
    tool_class: str
    scoped_allowed_tools: tuple[str, ...]
    default_model: str
    anthropic_api_key_ref: SecretRef | None
    allow_personal: bool
    forward_ssh_agent: bool
    cli_probe_timeout: float
    poll_interval: float
    claude_bin: str
    projects_root: Path
    state_dir: Path
    followup_grace: float
    followup_file_wait: float
    stop_grace: float
    kill_grace: float
    session_lock_wait: float
    max_parallel_runs: int
    stream_log_retention: float
    progress_patch_interval: float
    attach_allowed_roots: tuple[str, ...]
    attach_denied_patterns: tuple[str, ...]
    sdk: SdkSettings
    git_bin: str = "git"
    provision_timeout: float = 600.0
    provision_retry_after: float = 300.0
    project_folders_max: int = 500
    channels_resync_interval: float = 300.0
    transcribe_cmd: tuple[str, ...] = ()
    transcribe_timeout: float = 300.0
    default_workspace: str | None = None
    trash_check_interval: float = 60.0


_Parse = Callable[[Any], Any]


class _Reader:
    """Typed access to one table; every failure is a `ConfigError` naming the key."""

    def __init__(self, file: Path, table: str, values: Mapping[str, Any]) -> None:
        self.file = file
        self.table = table
        self.values = dict(values)
        self.used: set[str] = set()

    def _fail(self, name: str, problem: str) -> ConfigError:
        return ConfigError(f"{self.table}.{name}", self.file, problem)

    def get(self, name: str, kind: type, default: Any = ..., *, required: bool = False) -> Any:
        self.used.add(name)
        if name not in self.values:
            if required or default is ...:
                raise self._fail(name, "required key missing")
            return default
        value = self.values[name]
        if kind is int and isinstance(value, bool):
            raise self._fail(name, "wrong type, expected an integer")
        if not isinstance(value, kind):
            raise self._fail(name, f"wrong type, expected {kind.__name__}")
        return value

    def duration(self, name: str, default: str) -> float:
        text = self.get(name, str, default)
        try:
            return parse_duration(text)
        except ValueError:
            raise self._fail(name, "invalid duration") from None

    def strings(self, name: str, default: tuple[str, ...] = ()) -> tuple[str, ...]:
        value = self.get(name, list, list(default))
        if not all(isinstance(v, str) for v in value):
            raise self._fail(name, "wrong type, expected a list of strings")
        return tuple(value)

    def secret_ref(self, name: str, *, required: bool = False) -> SecretRef | None:
        text = self.get(name, str, "", required=required)
        if not text:
            if required:
                raise self._fail(name, "required key missing")
            return None
        try:
            return parse_secret_ref(text)
        except SecretError:
            raise self._fail(name, "not a secret_ref of the schemes file or env") from None

    def fail(self, name: str, problem: str) -> ConfigError:
        return self._fail(name, problem)

    def unknown(self) -> None:
        for name in sorted(set(self.values) - self.used):
            raise self._fail(name, "unknown key")


def _url(reader: _Reader) -> str:
    url = reader.get("server_url", str, required=True)
    parts = urlsplit(url)
    secure = parts.scheme == "https" and bool(parts.hostname)
    loopback = parts.scheme == "http" and parts.hostname == "127.0.0.1"
    if not (secure or loopback):
        raise reader.fail("server_url", "only https, or http for the host 127.0.0.1")
    return str(url).rstrip("/")


def _root(reader: _Reader, name: str) -> Path:
    path = Path(reader.get(name, str, required=True))
    if not path.is_absolute() or not path.exists():
        raise reader.fail(name, "must be an absolute path that exists")
    return path


def _state_dir(reader: _Reader) -> Path:
    path = Path(reader.get("state_dir", str, required=True))
    if not path.is_absolute() or not path.parent.exists():
        raise reader.fail("state_dir", "must be an absolute path whose parent exists")
    try:
        return ensure_private_dir(path)
    except StateDirError:
        raise reader.fail("state_dir", "group or others have a permission on it") from None


def _checked(
    reader: _Reader,
    name: str,
    default: tuple[str, ...],
    check: Callable[[tuple[str, ...]], str | None],
) -> tuple[str, ...]:
    """A list of strings with its own form check (M3-12 notes 3 a and 4 a)."""
    values = reader.strings(name, default)
    problem = check(values)
    if problem is not None:
        raise reader.fail(name, problem)
    return values


def _sdk(file: Path, adapter: _Reader, agent: _Reader, sdk: _Reader) -> SdkSettings:
    keys = set(SdkSettings.keys().values())
    texts: dict[str, str] = {}
    for reader in (adapter, agent, sdk):
        for name, raw in reader.values.items():
            key = f"{reader.table}.{name}"
            reader.used.add(name)
            if key not in keys:
                raise reader.fail(name, "unknown key")
            if isinstance(raw, bool) or not isinstance(raw, str | int):
                raise reader.fail(name, "wrong type")
            text = str(raw)
            if isinstance(raw, str):
                try:
                    # The SDK parses durations without the unit d; hand it seconds.
                    text = f"{parse_duration(raw)}s"
                except ValueError:
                    raise reader.fail(name, "invalid duration") from None
            texts[key] = text
    try:
        return SdkSettings.from_mapping(texts)
    except ValueError:
        raise ConfigError("sdk", file, "invalid SDK value") from None


def _at_least_one(r: _Reader, name: str, default: int) -> int:
    """The integer key ``name`` of ``r``, refused below 1."""
    value = r.get(name, int, default)
    if value < 1:
        raise r.fail(name, "must be at least 1")
    return int(value)


def _positive(r: _Reader, name: str, default: str) -> float:
    """The duration key ``name`` of ``r``, refused at zero."""
    value = r.duration(name, default)
    if value <= 0:
        raise r.fail(name, "must be above zero")
    return value


def _transcribe_cmd(r: _Reader) -> tuple[str, ...]:
    """``runner.transcribe_cmd``: a program and its arguments, no shell (09 18.14)."""
    cmd = r.strings("transcribe_cmd")
    if cmd and cmd[0].startswith("-"):
        raise r.fail("transcribe_cmd", "the program starts with -")
    if not all(_plain(part) for part in cmd):
        raise r.fail("transcribe_cmd", "an entry is empty or has controls")
    return cmd


def _default_workspace(r: _Reader, projects_root: Path) -> str | None:
    """``runner.default_workspace``: empty is off, ``.`` the root, else a folder under it."""
    value = r.get("default_workspace", str, "")
    if not value:
        return None
    try:
        resolve_default(projects_root, value)
    except WorkspaceError:
        raise r.fail(
            "default_workspace", "not . or a folder name under the projects root"
        ) from None
    return str(value)


def _plain(value: str) -> bool:
    """Not empty and without a control or bidi character (an argument may start with -)."""
    return check_argument("x" + value) and bool(value)


def load_config(path: Path) -> RunnerConfig:
    """Load and check the configuration file.

    ``runner.state_dir`` is created with mode ``0700`` when it is missing.

    Args:
        path: The TOML file of ``--config``.

    Returns:
        The configuration.

    Raises:
        ConfigError: The file cannot be read or parsed, or a key is unknown, missing, of the
            wrong type or invalid.
    """
    try:
        # The operator's own --config file, never model output (D-M2-34-14).
        data = tomllib.loads(path.read_text(encoding="utf-8"))  # NOSONAR
    except (OSError, UnicodeDecodeError):
        raise ConfigError("file", path, "cannot be read") from None
    except tomllib.TOMLDecodeError:
        raise ConfigError("file", path, "not valid TOML") from None
    for table, value in data.items():
        if table not in ("runner", "adapter", "agent", "sdk") or not isinstance(value, dict):
            raise ConfigError(table, path, "unknown table")
    r = _Reader(path, "runner", data.get("runner", {}))
    adapter = _Reader(path, "adapter", data.get("adapter", {}))
    agent = _Reader(path, "agent", data.get("agent", {}))
    sdk_table = _Reader(path, "sdk", data.get("sdk", {}))
    server_url = _url(r)
    try:
        instance_id = UUID(r.get("instance_id", str, required=True))
    except ValueError:
        raise r.fail("instance_id", "not a UUID") from None
    agent_token_ref = r.secret_ref("agent_token_ref", required=True)
    consumer = r.get("consumer", str, "default")
    if not _CONSUMER.fullmatch(consumer):
        raise r.fail("consumer", "not a consumer name")
    tool_class = r.get("tool_class", str, "chat")
    if tool_class not in ORDER:
        raise r.fail("tool_class", "not chat, scoped or host")
    tools = r.strings("scoped_allowed_tools")
    if not all(check_argument(t) for t in tools):
        raise r.fail("scoped_allowed_tools", "an entry is empty, starts with - or has controls")
    default_model = r.get("default_model", str, required=True)
    if not check_argument(default_model):
        raise r.fail("default_model", "empty, starts with - or has controls")
    folders_max = _at_least_one(r, "project_folders_max", 500)
    max_parallel = _at_least_one(r, "max_parallel_runs", 4)
    claude_bin = r.get("claude_bin", str, "claude")
    if not claude_bin:
        raise r.fail("claude_bin", "must not be empty")
    git_bin = r.get("git_bin", str, "git")
    if not check_argument(git_bin):
        raise r.fail("git_bin", "empty, starts with - or has controls")
    projects_root = _root(r, "projects_root")
    config = RunnerConfig(
        source=path,
        server_url=server_url,
        instance_id=instance_id,
        agent_token_ref=agent_token_ref,  # type: ignore[arg-type]  # required above
        dpop_key_ref=r.secret_ref("dpop_key_ref"),
        consumer=consumer,
        tool_class=tool_class,
        scoped_allowed_tools=tools,
        default_model=default_model,
        anthropic_api_key_ref=r.secret_ref("anthropic_api_key_ref"),
        allow_personal=r.get("allow_personal", bool, False),
        forward_ssh_agent=r.get("forward_ssh_agent", bool, False),
        cli_probe_timeout=r.duration("cli_probe_timeout", "30s"),
        poll_interval=r.duration("poll_interval", "200ms"),
        claude_bin=claude_bin,
        projects_root=projects_root,
        state_dir=_state_dir(r),
        followup_grace=r.duration("followup_grace", "30s"),
        followup_file_wait=r.duration("followup_file_wait", "2m"),
        stop_grace=r.duration("stop_grace", "10s"),
        kill_grace=r.duration("kill_grace", "5s"),
        session_lock_wait=r.duration("session_lock_wait", "10m"),
        max_parallel_runs=max_parallel,
        stream_log_retention=r.duration("stream_log_retention", "14d"),
        progress_patch_interval=r.duration("progress_patch_interval", "2m"),
        attach_allowed_roots=_checked(
            r, "attach_allowed_roots", ROOT_NAMES, check_allowed_root_names
        ),
        attach_denied_patterns=_checked(
            r, "attach_denied_patterns", DEFAULT_DENIED_PATTERNS, check_denied_patterns
        ),
        sdk=_sdk(path, adapter, agent, sdk_table),
        git_bin=git_bin,
        provision_timeout=r.duration("provision_timeout", "10m"),
        provision_retry_after=r.duration("provision_retry_after", "5m"),
        project_folders_max=folders_max,
        channels_resync_interval=_positive(r, "channels_resync_interval", "5m"),
        transcribe_cmd=_transcribe_cmd(r),
        transcribe_timeout=_positive(r, "transcribe_timeout", "5m"),
        default_workspace=_default_workspace(r, projects_root),
        trash_check_interval=_positive(r, "trash_check_interval", "60s"),
    )
    r.unknown()
    return config
