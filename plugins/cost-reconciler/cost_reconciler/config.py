# SPDX-License-Identifier: Apache-2.0
"""The configuration file of the app: a TOML table `[reconciler]`.

Keys (docs/spec/10-backend-policy.md section 17, place "Reconciliation app"):

- `server_url` (required): base address of the server; the API lies below `/api/v1`.
- `cost_api_url` (required): base address of the provider's API, from the documentation cited in
  10 12.4; no host name is built into the code.
- `token_ref` (required): `secret_ref` of the app token.
- `admin_key_ref` (required): `secret_ref` of the provider's admin key.
- `min_request_interval` (default `60s`): least time between two requests to the cost API.
- `lookback` (default `3d`): this many complete UTC days before today are reconciled.
- `http_timeout` (default the standard value of `sdk.http_timeout`): time limit of one request.
- `workspace_map` (required, at least one entry): sub-table of workspace ID to a list of agent IDs.

Durations use the format of docs/operations/configuration.md section 2. Every error names the key
and the violated rule, never the value.

Example:
    >>> config = parse_config({"reconciler": {
    ...     "server_url": "https://chat.example.com",
    ...     "cost_api_url": "https://costs.example.com",
    ...     "token_ref": "file:/run/secrets/token",
    ...     "admin_key_ref": "file:/run/secrets/admin_key",
    ...     "workspace_map": {"wrkspc_1": ["0192f4a0-7b1c-7250-807f-c268a7b21797"]},
    ... }})
    >>> config.lookback_days
    3
"""

from __future__ import annotations

import ipaddress
import re
import tomllib
import uuid
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import urlsplit

from cost_reconciler.secretref import SecretRef, SecretRefError, parse_secret_ref

SECTION = "reconciler"

# Defaults of 10 17 and, for http_timeout, of `sdk.http_timeout` (configuration.md section 4).
DEFAULT_MIN_REQUEST_INTERVAL = "60s"
DEFAULT_LOOKBACK = "3d"
DEFAULT_HTTP_TIMEOUT = "30s"

_REQUIRED = ("server_url", "cost_api_url", "token_ref", "admin_key_ref", "workspace_map")
_KNOWN = frozenset((*_REQUIRED, "min_request_interval", "lookback", "http_timeout"))
_SEGMENT = re.compile(r"[a-z][a-z0-9]*(_[a-z0-9]+)*")
_DURATION = re.compile(r"(0|[1-9][0-9]*)(ms|s|m|h|d)")
_UNIT_MS = {"ms": 1, "s": 1_000, "m": 60_000, "h": 3_600_000, "d": 86_400_000}
_DAY_MS = _UNIT_MS["d"]
# The largest duration of Go (int64 nanoseconds), so a value valid for the server's parser is
# valid here and the other way round (configuration.md section 2).
_MAX_NANOSECONDS = 2**63 - 1


class ConfigError(Exception):
    """A configuration error. The message is `<key>: <rule>`, never a value.

    Attributes:
        key: The configuration key, `--config` for the file itself.
        rule: The violated rule.
    """

    def __init__(self, key: str, rule: str) -> None:
        """Create the error.

        Args:
            key: The key that is wrong.
            rule: A fixed text naming the rule.
        """
        super().__init__(f"{key}: {rule}")
        self.key = key
        self.rule = rule


@dataclass(frozen=True)
class Config:
    """The checked configuration. It holds references to secrets, never their values.

    Attributes:
        server_url: Base address of the server without a trailing slash.
        cost_api_url: Base address of the cost API without a trailing slash.
        token_ref: Reference of the app token.
        admin_key_ref: Reference of the admin key.
        min_request_interval_ms: `reconciler.min_request_interval` in milliseconds.
        lookback_days: `reconciler.lookback` in whole days.
        http_timeout_ms: `reconciler.http_timeout` in milliseconds.
        workspace_map: Workspace ID to the IDs of its agents.
    """

    server_url: str
    cost_api_url: str
    token_ref: SecretRef
    admin_key_ref: SecretRef
    min_request_interval_ms: int
    lookback_days: int
    http_timeout_ms: int
    workspace_map: Mapping[str, tuple[str, ...]]

    @property
    def agent_ids(self) -> tuple[str, ...]:
        """All mapped agent IDs, sorted."""
        return tuple(sorted(a for agents in self.workspace_map.values() for a in agents))

    def workspace_of(self, agent_id: str) -> str | None:
        """Return the workspace an agent is mapped to, or None.

        Args:
            agent_id: The agent ID.

        Returns:
            The workspace ID or None for an agent without an entry.
        """
        for workspace, agents in self.workspace_map.items():
            if agent_id in agents:
                return workspace
        return None


def parse_duration(text: object, key: str) -> int:
    """Parse a duration of configuration.md section 2 into milliseconds.

    Args:
        text: The value, for example `60s`, `3d` or `500ms`.
        key: The key, for the error.

    Returns:
        The duration in milliseconds.

    Raises:
        ConfigError: The value is no duration in that format or exceeds the largest duration.
    """
    match = _DURATION.fullmatch(text) if isinstance(text, str) else None
    if match is None:
        raise ConfigError(key, "not a duration of the form <number><ms|s|m|h|d>")
    millis = int(match.group(1)) * _UNIT_MS[match.group(2)]
    if millis * 1_000_000 > _MAX_NANOSECONDS:
        raise ConfigError(key, "the duration exceeds the largest supported duration")
    return millis


def _string(section: Mapping[str, object], name: str) -> str:
    value = section[name]
    if not isinstance(value, str):
        raise ConfigError(f"{SECTION}.{name}", "must be a string")
    return value


def _is_loopback(host: str) -> bool:
    if host == "localhost":
        return True
    try:
        return ipaddress.ip_address(host).is_loopback
    except ValueError:
        return False


def _url(section: Mapping[str, object], name: str) -> str:
    key = f"{SECTION}.{name}"
    text = _string(section, name)
    try:
        parts = urlsplit(text)
        port = parts.port
    except ValueError:
        raise ConfigError(key, "not a valid address") from None
    if parts.scheme not in {"https", "http"} or not parts.hostname:
        raise ConfigError(key, "needs the scheme https and a host")
    if parts.username is not None or parts.password is not None:
        raise ConfigError(key, "must not hold a user part")
    if parts.query or parts.fragment or "?" in text or "#" in text:
        raise ConfigError(key, "must not hold a query or a fragment")
    if port is not None:
        raw_port = parts.netloc.rsplit(":", 1)[1]
        if port == 0 or raw_port.startswith("0"):
            raise ConfigError(key, "the port must be 1 to 65535 without a leading zero")
    if parts.scheme == "http" and not _is_loopback(parts.hostname):
        # Plain http only to a loopback address (tests, a local proxy): a token or key must not
        # cross a network unencrypted.
        raise ConfigError(key, "http is allowed only for a loopback address; use https")
    return text.rstrip("/")


def _secret_ref(section: Mapping[str, object], name: str) -> SecretRef:
    key = f"{SECTION}.{name}"
    try:
        return parse_secret_ref(_string(section, name))
    except SecretRefError as err:
        raise ConfigError(key, err.code) from None


def _workspace_map(value: object) -> dict[str, tuple[str, ...]]:
    key = f"{SECTION}.workspace_map"
    if not isinstance(value, dict) or not value:
        raise ConfigError(key, "needs at least one workspace with its agents")
    seen: set[str] = set()
    result: dict[str, tuple[str, ...]] = {}
    for workspace, agents in value.items():
        if not workspace:
            raise ConfigError(key, "a workspace ID is empty")
        if not isinstance(agents, list) or not agents:
            raise ConfigError(key, "each workspace needs a non-empty list of agent IDs")
        for agent in agents:
            if not isinstance(agent, str) or not _is_uuid(agent):
                raise ConfigError(key, "an agent ID is not a UUID in lower case")
            if agent in seen:
                raise ConfigError(key, "an agent ID is listed more than once")
            seen.add(agent)
        result[workspace] = tuple(agents)
    return result


def _is_uuid(text: str) -> bool:
    try:
        return str(uuid.UUID(text)) == text
    except ValueError:
        return False


def parse_config(data: Mapping[str, object]) -> Config:
    """Check the parsed TOML document and build the configuration.

    Args:
        data: The document as `tomllib` returns it.

    Returns:
        The checked configuration.

    Raises:
        ConfigError: An unknown or missing key, a value of the wrong type or format, a lookback
            that is not a whole number of days, an empty workspace map or an agent listed twice.
    """
    for top in data:
        if top != SECTION:
            raise ConfigError(top if _SEGMENT.fullmatch(top) else "<key>", "unknown key")
    section = data.get(SECTION)
    if not isinstance(section, dict):
        raise ConfigError(SECTION, "the table [reconciler] is missing")
    for name in section:
        if name not in _KNOWN:
            shown = f"{SECTION}.{name}" if _SEGMENT.fullmatch(name) else SECTION
            raise ConfigError(shown, "unknown key")
    for name in _REQUIRED:
        if name not in section:
            raise ConfigError(f"{SECTION}.{name}", "required key is missing")
    interval = parse_duration(
        section.get("min_request_interval", DEFAULT_MIN_REQUEST_INTERVAL),
        f"{SECTION}.min_request_interval",
    )
    if interval == 0:
        # Zero would turn the loop into a busy loop against the cost API and the server.
        raise ConfigError(f"{SECTION}.min_request_interval", "must be greater than zero")
    lookback = parse_duration(section.get("lookback", DEFAULT_LOOKBACK), f"{SECTION}.lookback")
    if lookback == 0 or lookback % _DAY_MS:
        raise ConfigError(f"{SECTION}.lookback", "must be a whole number of days, at least one")
    timeout = parse_duration(
        section.get("http_timeout", DEFAULT_HTTP_TIMEOUT), f"{SECTION}.http_timeout"
    )
    if timeout == 0:
        raise ConfigError(f"{SECTION}.http_timeout", "must be greater than zero")
    return Config(
        server_url=_url(section, "server_url"),
        cost_api_url=_url(section, "cost_api_url"),
        token_ref=_secret_ref(section, "token_ref"),
        admin_key_ref=_secret_ref(section, "admin_key_ref"),
        min_request_interval_ms=interval,
        lookback_days=lookback // _DAY_MS,
        http_timeout_ms=timeout,
        workspace_map=_workspace_map(section["workspace_map"]),
    )


def load_config(path: Path) -> Config:
    """Read and check the configuration file.

    Args:
        path: The TOML file.

    Returns:
        The checked configuration.

    Raises:
        ConfigError: The file cannot be read (key `--config`), is no TOML document (key
            `--config`, without the parser's message, which may quote the text) or fails a
            check of `parse_config`.
    """
    try:
        raw = path.read_bytes()
    except OSError:
        raise ConfigError("--config", "the file cannot be read") from None
    try:
        data = tomllib.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, tomllib.TOMLDecodeError):
        raise ConfigError("--config", "the file is no valid UTF-8 TOML document") from None
    return parse_config(data)
