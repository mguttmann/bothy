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

from claude_code_runner.secrets import SecretError, SecretRef, parse_secret_ref
from claude_code_runner.statedir import StateDirError, ensure_private_dir
from claude_code_runner.toolclass import ORDER

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
        stop_grace: ``runner.stop_grace``.
        kill_grace: ``runner.kill_grace``.
        session_lock_wait: ``runner.session_lock_wait``.
        max_parallel_runs: ``runner.max_parallel_runs``.
        stream_log_retention: ``runner.stream_log_retention``.
        progress_patch_interval: ``runner.progress_patch_interval``: ``tool_progress`` becomes a
            patch of ``/metrics/progress_events`` only when the last patch of the turn is older
            (09 18.4, M2-51); keep it below the server's ``run.stall_after``.
        attach_allowed_roots: ``runner.attach_allowed_roots`` (used from M3-12 on).
        attach_denied_patterns: ``runner.attach_denied_patterns`` (used from M3-12 on).
        sdk: The SDK settings of the ``adapter``, ``agent`` and ``sdk`` keys.
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
    stop_grace: float
    kill_grace: float
    session_lock_wait: float
    max_parallel_runs: int
    stream_log_retention: float
    progress_patch_interval: float
    attach_allowed_roots: tuple[str, ...]
    attach_denied_patterns: tuple[str, ...]
    sdk: SdkSettings


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

    def strings(self, name: str) -> tuple[str, ...]:
        value = self.get(name, list, [])
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
    max_parallel = r.get("max_parallel_runs", int, 4)
    if max_parallel < 1:
        raise r.fail("max_parallel_runs", "must be at least 1")
    claude_bin = r.get("claude_bin", str, "claude")
    if not claude_bin:
        raise r.fail("claude_bin", "must not be empty")
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
        projects_root=_root(r, "projects_root"),
        state_dir=_state_dir(r),
        followup_grace=r.duration("followup_grace", "30s"),
        stop_grace=r.duration("stop_grace", "10s"),
        kill_grace=r.duration("kill_grace", "5s"),
        session_lock_wait=r.duration("session_lock_wait", "10m"),
        max_parallel_runs=max_parallel,
        stream_log_retention=r.duration("stream_log_retention", "14d"),
        progress_patch_interval=r.duration("progress_patch_interval", "2m"),
        attach_allowed_roots=r.strings("attach_allowed_roots"),
        attach_denied_patterns=r.strings("attach_denied_patterns"),
        sdk=_sdk(path, adapter, agent, sdk_table),
    )
    r.unknown()
    return config
