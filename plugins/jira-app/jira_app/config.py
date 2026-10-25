# SPDX-License-Identifier: Apache-2.0
"""Configuration of the app and its secret files (brief M2-50 Design note 3).

A JSON file named by ``--config`` holds one object ``jira_app`` whose members are the keys of
docs/spec/11-extensions.md 16 with Place "Jira app", without their prefix. Unknown members,
duplicate member names, wrong types, bad durations and missing required keys raise
`ConfigError` with a message that names key and file and never a value.

Secret files (``token_file``, ``dpop_key_file``, ``webhook_secret_file``,
``jira_credentials_file``) must have mode ``0400`` or ``0600`` and belong to the process user
(SEC-SEC-1 applied to the app). The webhook secret and the Jira credentials are read at each use
through `read_secret_file`, so a rotation needs no restart. The content of a secret file is used
without one trailing line break.

Reused keys are never new numbers: ``webhook.receiver_tolerance``, ``sdk.http_timeout`` and the
``sdk.retry_*`` keys come from the SDK's `bothy_sdk.SdkSettings` defaults.

Examples:
    >>> parse_listen("127.0.0.1:0", "config.json")
    ('127.0.0.1', 0)
"""

from __future__ import annotations

import ipaddress
import json
import os
import re
import stat
import unicodedata
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit
from uuid import UUID

__all__ = [
    "DEFAULTS",
    "KEYS",
    "AppConfig",
    "ConfigError",
    "JiraCredentials",
    "ProjectMap",
    "check_start_files",
    "check_state_dir",
    "is_loopback_host",
    "load_config",
    "parse_config",
    "parse_config_duration",
    "parse_listen",
    "read_jira_credentials",
    "read_secret_file",
]

PREFIX = "jira_app"
"""The member of the configuration file and the prefix of every key."""

DEFAULTS: Mapping[str, Any] = {
    "dpop_key_file": "",
    "max_body_bytes": 1048576,
    "dedupe_window": "72h",
    "max_queued_jobs": 10000,
}
"""Defaults of the optional keys; equal to the "Default" column of configuration.md."""

KEYS: tuple[str, ...] = (
    "listen",
    "server_url",
    "token_file",
    "dpop_key_file",
    "webhook_secret_file",
    "jira_base_url",
    "jira_credentials_file",
    "projects",
    "state_dir",
    "max_body_bytes",
    "dedupe_window",
    "max_queued_jobs",
)
"""Every key without its prefix, in the order of docs/spec/11-extensions.md 16."""

_PROJECT_KEY = re.compile(r"^[A-Z][A-Z0-9_]*$")
_DURATION = re.compile(r"^(0|[1-9][0-9]*)(ms|s|m|h|d)$")
_UNIT_NS = {"ms": 10**6, "s": 10**9, "m": 60 * 10**9, "h": 3600 * 10**9, "d": 86400 * 10**9}
# The largest duration of Go's time.Duration (int64 nanoseconds), configuration.md section 2.
_MAX_NS = 2**63 - 1
_SECRET_MODES = frozenset({0o400, 0o600})
_BIDI = frozenset("\u200e\u200f\u061c\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069")


class ConfigError(Exception):
    """The configuration or a secret file failed a check; the message names key and file."""


@dataclass(frozen=True)
class ProjectMap:
    """One entry of ``jira_app.projects``.

    Attributes:
        project: The Jira project key, ``^[A-Z][A-Z0-9_]*$``.
        channel_id: The forum of its issues (lower-case UUID text).
        issue_type: The name of the Jira issue type that ``/issue`` creates in the project.
    """

    project: str
    channel_id: str
    issue_type: str


@dataclass(frozen=True)
class JiraCredentials:
    """The Jira account e-mail and API token of ``jira_app.jira_credentials_file``.

    ``repr`` never shows the values.

    Attributes:
        line: The line ``<account e-mail>:<API token>`` as read.
    """

    line: str

    def __repr__(self) -> str:
        """Return a text without the credentials."""
        return "JiraCredentials([redacted])"


@dataclass(frozen=True)
class AppConfig:
    """The loaded configuration (Design note 3).

    Attributes:
        source: The configuration file, named in error messages.
        listen_host: Host of ``jira_app.listen``.
        listen_port: Port of ``jira_app.listen``.
        server_url: ``jira_app.server_url`` without a trailing slash.
        token_file: ``jira_app.token_file``.
        dpop_key_file: ``jira_app.dpop_key_file`` (empty: no DPoP).
        webhook_secret_file: ``jira_app.webhook_secret_file``.
        jira_base_url: ``jira_app.jira_base_url`` without a trailing slash.
        jira_credentials_file: ``jira_app.jira_credentials_file``.
        projects: ``jira_app.projects``.
        state_dir: ``jira_app.state_dir``.
        max_body_bytes: ``jira_app.max_body_bytes``.
        dedupe_window: ``jira_app.dedupe_window`` in seconds.
        max_queued_jobs: ``jira_app.max_queued_jobs``.
    """

    source: str
    listen_host: str
    listen_port: int
    server_url: str
    token_file: str
    dpop_key_file: str
    webhook_secret_file: str
    jira_base_url: str
    jira_credentials_file: str
    projects: tuple[ProjectMap, ...]
    state_dir: str
    max_body_bytes: int
    dedupe_window: float
    max_queued_jobs: int

    def project(self, key: str) -> ProjectMap | None:
        """Return the entry of a project key, compared exactly.

        Args:
            key: A Jira project key.

        Returns:
            The entry, or ``None`` when the project is not configured.
        """
        for entry in self.projects:
            if entry.project == key:
                return entry
        return None

    def projects_of_forum(self, channel_id: str) -> tuple[ProjectMap, ...]:
        """Return the projects whose issues go to a forum, in configuration order.

        Args:
            channel_id: The forum (any case).

        Returns:
            The entries; empty when the forum is not configured.
        """
        wanted = channel_id.lower()
        return tuple(p for p in self.projects if p.channel_id == wanted)


def _no_duplicates(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for key, value in pairs:
        if key in out:
            raise ConfigError(f"duplicate member {key}")
        out[key] = value
    return out


def is_loopback_host(host: str) -> bool:
    """Return whether ``host`` is a loopback host (``localhost``, ``127.0.0.0/8``, ``::1``).

    Args:
        host: A host name or address, IPv6 with or without brackets.

    Returns:
        True for a loopback host.
    """
    if host.lower() == "localhost":
        return True
    try:
        return ipaddress.ip_address(host.strip("[]")).is_loopback
    except ValueError:
        return False


def parse_listen(value: str, source: str) -> tuple[str, int]:
    """Parse ``host:port`` of ``jira_app.listen``.

    Args:
        value: The text, ``host:port`` or ``[v6]:port``.
        source: The configuration file, for the message.

    Returns:
        Host and port.

    Raises:
        ConfigError: The text is no ``host:port`` with a port from 0 to 65535.
    """
    host, sep, port = value.rpartition(":")
    if not sep or not host or not port.isdigit() or not 0 <= int(port) <= 65535:
        raise ConfigError(f"{PREFIX}.listen in {source}: expected host:port")
    if host.startswith("[") and host.endswith("]"):
        host = host[1:-1]
    return host, int(port)


def _url(value: str, key: str, source: str, *, prefix_allowed: bool) -> str:
    parts = urlsplit(value)
    scheme = parts.scheme.lower()
    try:
        host = parts.hostname or ""
        _ = parts.port
    except ValueError as exc:
        raise ConfigError(f"{PREFIX}.{key} in {source}: not a URL") from exc
    if not host or scheme not in ("https", "http"):
        raise ConfigError(f"{PREFIX}.{key} in {source}: expected an https URL")
    if scheme == "http" and not is_loopback_host(host):
        raise ConfigError(f"{PREFIX}.{key} in {source}: http only for a loopback host")
    if parts.query or parts.fragment or "?" in value or "#" in value:
        raise ConfigError(f"{PREFIX}.{key} in {source}: no query and no fragment")
    if parts.username is not None or parts.password is not None:
        raise ConfigError(f"{PREFIX}.{key} in {source}: no user information")
    path = parts.path.rstrip("/")
    if path and not prefix_allowed:
        raise ConfigError(f"{PREFIX}.{key} in {source}: no path")
    return f"{scheme}://{parts.netloc}{path}"


def parse_config_duration(text: str) -> float | None:
    """Parse a duration of docs/operations/configuration.md section 2.

    Args:
        text: Exactly ``(0|[1-9][0-9]*)(ms|s|m|h|d)``, for example ``72h`` or ``250ms``.

    Returns:
        Seconds, or ``None`` for any other text and for a value above Go's largest duration.

    Examples:
        >>> parse_config_duration("2d"), parse_config_duration("1.5h")
        (172800.0, None)
    """
    match = _DURATION.fullmatch(text)
    if match is None:
        return None
    nanoseconds = int(match.group(1)) * _UNIT_NS[match.group(2)]
    if nanoseconds > _MAX_NS:
        return None
    return nanoseconds / 10**9


def _text(raw: Mapping[str, Any], key: str, source: str, *, required: bool) -> str:
    if key not in raw:
        if required:
            raise ConfigError(f"{PREFIX}.{key} in {source}: missing")
        return str(DEFAULTS[key])
    value = raw[key]
    if not isinstance(value, str):
        raise ConfigError(f"{PREFIX}.{key} in {source}: expected a string")
    if required and value == "":
        raise ConfigError(f"{PREFIX}.{key} in {source}: must not be empty")
    return value


def _count(raw: Mapping[str, Any], key: str, source: str) -> int:
    value = raw.get(key, DEFAULTS[key])
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
        raise ConfigError(f"{PREFIX}.{key} in {source}: expected a positive integer")
    return value


def _clean_display(text: str) -> bool:
    return bool(text.strip()) and not any(
        unicodedata.category(c) == "Cc" or c in _BIDI for c in text
    )


def _projects(raw: Mapping[str, Any], source: str) -> tuple[ProjectMap, ...]:
    key = "projects"
    if key not in raw:
        raise ConfigError(f"{PREFIX}.{key} in {source}: missing")
    value = raw[key]
    if not isinstance(value, list) or not value:
        raise ConfigError(f"{PREFIX}.{key} in {source}: expected a non-empty list")
    seen: set[str] = set()
    out: list[ProjectMap] = []
    for index, item in enumerate(value):
        where = f"{PREFIX}.{key}[{index}] in {source}"
        if not isinstance(item, dict):
            raise ConfigError(f"{where}: expected an object")
        unknown = set(item) - {"project", "channel_id", "issue_type"}
        if unknown:
            raise ConfigError(f"{where}: unknown member {sorted(unknown)[0]}")
        project, channel, issue_type = (
            item.get("project"),
            item.get("channel_id"),
            item.get("issue_type"),
        )
        if not isinstance(project, str) or _PROJECT_KEY.fullmatch(project) is None:
            raise ConfigError(f"{where}: project must match ^[A-Z][A-Z0-9_]*$")
        if project in seen:
            raise ConfigError(f"{where}: project listed twice")
        seen.add(project)
        try:
            channel_id = str(UUID(channel)) if isinstance(channel, str) else ""
        except ValueError:
            channel_id = ""
        if not channel_id:
            raise ConfigError(f"{where}: channel_id must be a UUID")
        if not isinstance(issue_type, str) or not _clean_display(issue_type):
            raise ConfigError(f"{where}: issue_type must be a display text")
        out.append(ProjectMap(project, channel_id, issue_type))
    return tuple(out)


def parse_config(data: object, source: str) -> AppConfig:
    """Validate a decoded configuration document.

    Args:
        data: The decoded JSON document.
        source: The configuration file, named in messages.

    Returns:
        The configuration.

    Raises:
        ConfigError: A rule of Design note 3 is broken; the message names key and file.
    """
    if not isinstance(data, dict) or set(data) != {PREFIX}:
        raise ConfigError(f"{source}: expected one object {PREFIX}")
    raw = data[PREFIX]
    if not isinstance(raw, dict):
        raise ConfigError(f"{PREFIX} in {source}: expected an object")
    unknown = sorted(set(raw) - set(KEYS))
    if unknown:
        raise ConfigError(f"{PREFIX}.{unknown[0]} in {source}: unknown key")
    host, port = parse_listen(_text(raw, "listen", source, required=True), source)
    window = parse_config_duration(_text(raw, "dedupe_window", source, required=False))
    if window is None:
        raise ConfigError(f"{PREFIX}.dedupe_window in {source}: expected a duration")
    if window <= 0:
        raise ConfigError(f"{PREFIX}.dedupe_window in {source}: must be positive")
    return AppConfig(
        source=source,
        listen_host=host,
        listen_port=port,
        server_url=_url(
            _text(raw, "server_url", source, required=True),
            "server_url",
            source,
            prefix_allowed=False,
        ),
        token_file=_text(raw, "token_file", source, required=True),
        dpop_key_file=_text(raw, "dpop_key_file", source, required=False),
        webhook_secret_file=_text(raw, "webhook_secret_file", source, required=True),
        jira_base_url=_url(
            _text(raw, "jira_base_url", source, required=True),
            "jira_base_url",
            source,
            prefix_allowed=True,
        ),
        jira_credentials_file=_text(raw, "jira_credentials_file", source, required=True),
        projects=_projects(raw, source),
        state_dir=_text(raw, "state_dir", source, required=True),
        max_body_bytes=_count(raw, "max_body_bytes", source),
        dedupe_window=window,
        max_queued_jobs=_count(raw, "max_queued_jobs", source),
    )


def load_config(path: str | os.PathLike[str]) -> AppConfig:
    """Read and validate the configuration file.

    Args:
        path: The file named by ``--config``.

    Returns:
        The configuration.

    Raises:
        ConfigError: The file is unreadable, no JSON, or breaks a rule of Design note 3.

    Examples:
        >>> from pathlib import Path
        >>> example = Path(__file__).resolve().parents[1] / "config.example.json"
        >>> load_config(example).listen_port
        8091
    """
    source = str(path)
    try:
        # The operator's own --config file, never model output (D-M2-50-2).
        text = Path(path).read_text(encoding="utf-8")  # NOSONAR
    except (OSError, UnicodeDecodeError) as exc:
        raise ConfigError(f"{source}: cannot be read") from exc
    try:
        data = json.loads(text, object_pairs_hook=_no_duplicates)
    except ConfigError as exc:
        raise ConfigError(f"{source}: {exc}") from None
    except ValueError as exc:
        raise ConfigError(f"{source}: not a JSON document") from exc
    return parse_config(data, source)


def _check_secret_file(path: str, key: str) -> Path:
    file = Path(path)
    try:
        info = file.stat()
    except OSError as exc:
        raise ConfigError(f"{PREFIX}.{key} ({path}): cannot be read") from exc
    if not stat.S_ISREG(info.st_mode):
        raise ConfigError(f"{PREFIX}.{key} ({path}): not a regular file")
    if stat.S_IMODE(info.st_mode) not in _SECRET_MODES:
        raise ConfigError(f"{PREFIX}.{key} ({path}): mode must be 0400 or 0600")
    if info.st_uid != os.getuid():
        raise ConfigError(f"{PREFIX}.{key} ({path}): must belong to the process user")
    return file


def read_secret_file(path: str, key: str) -> str:
    """Read a secret file after its mode and owner check.

    Args:
        path: The file.
        key: The configuration key without prefix, for the message.

    Returns:
        The content without one trailing line break.

    Raises:
        ConfigError: The file is missing, not private, not the process user's, or empty; the
            message names key and file, never the content.
    """
    file = _check_secret_file(path, key)
    try:
        text = file.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as exc:
        raise ConfigError(f"{PREFIX}.{key} ({path}): cannot be read") from exc
    if text.endswith("\r\n"):
        text = text[:-2]
    elif text.endswith("\n"):
        text = text[:-1]
    if text == "":
        raise ConfigError(f"{PREFIX}.{key} ({path}): empty")
    return text


def read_jira_credentials(path: str) -> JiraCredentials:
    """Read ``jira_app.jira_credentials_file``: one line ``<account e-mail>:<API token>``.

    Args:
        path: The file.

    Returns:
        The credentials.

    Raises:
        ConfigError: The file fails `read_secret_file` or its line has no ``:``, an empty part
            or a line break.
    """
    line = read_secret_file(path, "jira_credentials_file")
    user, sep, token = line.partition(":")
    if not sep or not user or not token or "\n" in line or "\r" in line:
        raise ConfigError(
            f"{PREFIX}.jira_credentials_file ({path}): expected one line <e-mail>:<API token>"
        )
    return JiraCredentials(line)


def check_state_dir(path: str) -> Path:
    """Create the state directory with mode ``0700`` or check an existing one.

    Args:
        path: ``jira_app.state_dir``.

    Returns:
        The directory.

    Raises:
        ConfigError: The directory cannot be created, is no directory, or grants group or other
            access.
    """
    directory = Path(path)
    try:
        directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        info = directory.stat()
    except OSError as exc:
        raise ConfigError(f"{PREFIX}.state_dir ({path}): cannot be created") from exc
    if not stat.S_ISDIR(info.st_mode):
        raise ConfigError(f"{PREFIX}.state_dir ({path}): not a directory")
    if stat.S_IMODE(info.st_mode) & (stat.S_IRWXG | stat.S_IRWXO):
        raise ConfigError(f"{PREFIX}.state_dir ({path}): must not grant group or other access")
    if info.st_uid != os.getuid():
        raise ConfigError(f"{PREFIX}.state_dir ({path}): must belong to the process user")
    return directory


def check_start_files(config: AppConfig) -> None:
    """Run every file check of the start: secret files and the state directory.

    Args:
        config: The configuration.

    Raises:
        ConfigError: A check failed.
    """
    read_secret_file(config.token_file, "token_file")
    if config.dpop_key_file:
        _check_secret_file(config.dpop_key_file, "dpop_key_file")
    read_secret_file(config.webhook_secret_file, "webhook_secret_file")
    read_jira_credentials(config.jira_credentials_file)
    check_state_dir(config.state_dir)
