# SPDX-License-Identifier: Apache-2.0
"""Install the adapter into a Hermes profile and switch its billing mode (09 17.2, 17.5, 17.7).

Three commands (``python -m bothy_hermes_adapter``, run with the profile's own Python, so no
module of the package starts a process; docs/DECISIONS.md D-M2-33-12):

* ``install``: places the plugin entry where the pinned version discovers it
  (``<home>/plugins/<platform key>``), writes the platform section with the adapter's keys, the
  own access policy (``dm_policy: allowlist``), the adapter's control sender as slash-command
  admin, ``display.busy_input_mode`` (``steer`` or ``queue``; ``interrupt`` is refused) and
  ``busy_ack_enabled: false``, ``tool_progress: off`` and ``busy_steer_ack_enabled: false`` for
  the platform and a placeholder home channel (Hermes would otherwise
  ask for one in the first turn's thread; cron delivery is not built yet, 09 17.9); it
  refuses to change an existing platform key
  (D-AG-19), because every thread would lose its Hermes session.
* ``switch-mode``: ``organization`` requires the API-key variable of the profile environment to
  be set (judged by its name), empties the OAuth variables and Hermes's allow list of the
  platform, refuses a login file of another tool in the profile's home and sets
  ``auth.adopt_external_logins: false``; ``personal`` empties the API-key variable and writes
  Hermes's allow list (``<KEY>_ALLOWED_USERS``) with exactly the credential holder (17.7 layer 3).
* ``check``: the same reads and the prediction without a change.

Every command reads the configuration with Hermes's own reader before and after, prints a summary
without any value (no URL, no path, no token, no key) and the mode Hermes would now resolve; it
never restarts a process and names the restart the operator must do (17.8, 10 10.1).

Examples:
    >>> plugin_entry("bothy")["plugin.yaml"].splitlines()[1]
    'name: bothy'
"""

from __future__ import annotations

import copy
from collections.abc import Callable, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Final, Protocol
from uuid import UUID

from bothy_hermes_adapter.actual_mode import ActualMode, determine
from bothy_hermes_adapter.config import load_settings
from bothy_hermes_adapter.runtime import ResolvedCredential

__all__ = [
    "CONTROL_SENDER",
    "InstallError",
    "InstallRequest",
    "ProfileBridge",
    "check",
    "install",
    "plugin_entry",
    "switch_mode",
]

CONTROL_SENDER: Final = "bothy-adapter-control"
"""The adapter's control sender (``platform.CONTROL_SENDER``); a test holds both equal."""

_LOGIN_FILE: Final = Path(".claude") / ".credentials.json"

HOME_CHANNEL_UNASSIGNED: Final = "unassigned"
"""``chat_id`` of the platform's home channel until cron delivery is built (09 17.9)."""


class InstallError(ValueError):
    """A command refused the change; the text names the reason, never a value."""


class ProfileBridge(Protocol):
    """Hermes's own reader and writer for one profile home (``hermes_config``)."""

    home: Path

    def read_config(self) -> dict[str, Any]:
        """``config.yaml`` as written."""
        ...

    def write_config(self, config: Mapping[str, Any]) -> None:
        """Write ``config.yaml``."""
        ...

    def env_defines(self, name: str) -> bool:
        """Whether ``.env`` sets the variable (judged by name)."""
        ...

    def set_env(self, name: str, value: str) -> None:
        """Set a variable in ``.env``."""
        ...

    def remove_env(self, name: str) -> None:
        """Remove a variable from ``.env``."""
        ...

    def predict(self) -> ResolvedCredential:
        """The credential kind a fresh gateway of the profile resolves."""
        ...

    def account_home(self) -> Path:
        """The ``HOME`` the profile's gateway runs with."""
        ...

    def api_key_variable(self) -> str:
        """The API-key variable name of the record."""
        ...

    def oauth_variables(self) -> tuple[str, ...]:
        """The OAuth variable names of the record."""
        ...


@dataclass(frozen=True)
class InstallRequest:
    """The arguments of ``install``.

    Attributes:
        server_url: `hermes_adapter.server_url`.
        instance_id: `hermes_adapter.instance_id`.
        token_file: `hermes_adapter.token_file`.
        busy_input_mode: ``steer`` or ``queue``.
        platform_key: `hermes_adapter.platform_key`.
        consumer: `hermes_adapter.consumer`.
        dpop_key_file: `hermes_adapter.dpop_key_file`, if any.
    """

    server_url: str = field(repr=False)
    instance_id: UUID
    token_file: Path = field(repr=False)
    busy_input_mode: str
    platform_key: str = "bothy"
    consumer: str = "default"
    dpop_key_file: Path | None = field(default=None, repr=False)


def plugin_entry(platform_key: str) -> dict[str, str]:
    """The files of the plugin directory the pinned version discovers.

    Args:
        platform_key: `hermes_adapter.platform_key`.

    Returns:
        File name to text: ``plugin.yaml`` (``kind: platform``) and ``__init__.py``.
    """
    manifest = (
        "# SPDX-License-Identifier: Apache-2.0\n"
        f"name: {platform_key}\n"
        "label: Chat server adapter\n"
        "kind: platform\n"
        "version: 0.0.0\n"
        "description: Connects this Hermes profile to the chat server as one agent.\n"
    )
    module = (
        "# SPDX-License-Identifier: Apache-2.0\n"
        '"""Plugin entry written by the installer of bothy_hermes_adapter."""\n\n'
        "from bothy_hermes_adapter.entry import make_register\n\n"
        f'register = make_register("{platform_key}")\n'
    )
    return {"plugin.yaml": manifest, "__init__.py": module}


def _platform_sections(config: Mapping[str, Any]) -> dict[str, Any]:
    gateway = config.get("gateway") or {}
    platforms = gateway.get("platforms") if isinstance(gateway, Mapping) else None
    return dict(platforms) if isinstance(platforms, Mapping) else {}


def _installed_key(config: Mapping[str, Any]) -> str | None:
    for section in _platform_sections(config).values():
        extra = section.get("extra") if isinstance(section, Mapping) else None
        if isinstance(extra, Mapping) and "hermes_adapter.platform_key" in extra:
            return str(extra["hermes_adapter.platform_key"])
    return None


def _nested(config: dict[str, Any], *path: str) -> dict[str, Any]:
    node = config
    for key in path:
        child = node.get(key)
        if not isinstance(child, dict):
            child = {}
            node[key] = child
        node = child
    return node


def plan_install(config: Mapping[str, Any], request: InstallRequest) -> dict[str, Any]:
    """The configuration after ``install`` (pure; the bridge writes it).

    Args:
        config: ``config.yaml`` before.
        request: The arguments.

    Returns:
        The new configuration.

    Raises:
        InstallError: ``interrupt`` or an unknown busy mode, or a different platform key is
            installed already.
    """
    if request.busy_input_mode not in ("steer", "queue"):
        raise InstallError("busy input mode must be steer or queue (interrupt is refused)")
    existing = _installed_key(config)
    if existing is not None and existing != request.platform_key:
        raise InstallError("the platform key is fixed after commissioning (D-AG-19)")
    extra: dict[str, Any] = {
        "hermes_adapter.platform_key": request.platform_key,
        "hermes_adapter.consumer": request.consumer,
        "hermes_adapter.server_url": request.server_url,
        "hermes_adapter.instance_id": str(request.instance_id),
        "hermes_adapter.token_file": str(request.token_file),
        "dm_policy": "allowlist",
        "allow_admin_from": [CONTROL_SENDER],
        "group_allow_admin_from": [CONTROL_SENDER],
    }
    if request.dpop_key_file is not None:
        extra["hermes_adapter.dpop_key_file"] = str(request.dpop_key_file)
    load_settings(extra)  # the same validation the plugin applies at connect
    new = copy.deepcopy(dict(config))
    section = _nested(new, "gateway", "platforms", request.platform_key)
    section["enabled"] = True
    if not isinstance(section.get("home_channel"), dict):
        # Hermes asks every platform without a home channel to set one, in the first turn's
        # thread; cron delivery is not built yet (09 17.9), so the channel only stops the notice.
        section["home_channel"] = {
            "platform": request.platform_key,
            "chat_id": HOME_CHANNEL_UNASSIGNED,
            "name": "Not assigned",
        }
    old_extra = section.get("extra")
    section["extra"] = {**(old_extra if isinstance(old_extra, dict) else {}), **extra}
    plugins = _nested(new, "plugins")
    listed = plugins.get("enabled")
    enabled: list[Any] = list(listed) if isinstance(listed, list) else []
    if request.platform_key not in enabled:
        enabled.append(request.platform_key)
    plugins["enabled"] = enabled
    _nested(new, "display")["busy_input_mode"] = request.busy_input_mode
    # Hermes' own busy notices ("queued for the next turn") would become messages of the run;
    # the profile serves only this platform (09 17.5), so the global switch is right here.
    _nested(new, "display")["busy_ack_enabled"] = False
    display = _nested(new, "display", "platforms", request.platform_key)
    display["tool_progress"] = "off"
    # Hermes would answer a steered follow-up with a notice of its own in the thread; the run's
    # live box already shows the follow-up's state (09 7), so the notice stays off.
    display["busy_steer_ack_enabled"] = False
    return new


def _summary(bridge: ProfileBridge, config: Mapping[str, Any], key: str | None) -> list[str]:
    lines = []
    platform = _platform_sections(config).get(key or "", {}) if key else {}
    extra = platform.get("extra") if isinstance(platform, Mapping) else None
    lines.append(f"platform: {'installed' if extra else 'not installed'}")
    if isinstance(extra, Mapping):
        for name in sorted(k for k in extra if str(k).startswith("hermes_adapter.")):
            lines.append(f"{name}: set")
    display = config.get("display") if isinstance(config.get("display"), Mapping) else {}
    busy = str((display or {}).get("busy_input_mode") or "interrupt")
    lines.append(f"busy_input_mode: {busy}")
    api_key = bridge.api_key_variable()
    lines.append(f"{api_key}: {'set' if bridge.env_defines(api_key) else 'empty'}")
    for name in bridge.oauth_variables():
        lines.append(f"{name}: {'set' if bridge.env_defines(name) else 'empty'}")
    if key:
        allowed = f"{key.upper()}_ALLOWED_USERS"
        lines.append(f"{allowed}: {'set' if bridge.env_defines(allowed) else 'empty'}")
    return lines


def _prediction(bridge: ProfileBridge) -> tuple[ActualMode, list[str]]:
    actual = determine(bridge.predict)
    mode = actual.mode or "not determinable (runs fail with billing_mode_mismatch)"
    return actual, [
        f"predicted mode: {mode} (key type {actual.key_type})",
        "restart the Hermes gateway of this profile for the change to take effect; a gateway "
        "that serves several profiles restarts all of them",
    ]


def install(
    bridge: ProfileBridge, request: InstallRequest, place: Callable[[Path, str], None]
) -> list[str]:
    """``install``: write the platform section and place the plugin entry.

    Args:
        bridge: Hermes's reader and writer for the profile.
        request: The arguments.
        place: Writes one file of the plugin entry (path, text).

    Returns:
        The printed summary (no values).

    Raises:
        InstallError: A refusal of `plan_install`, or the written configuration does not read
            back as written.
    """
    before = bridge.read_config()
    planned = plan_install(before, request)
    bridge.write_config(planned)
    after = bridge.read_config()
    section = _platform_sections(after).get(request.platform_key, {})
    display = after.get("display") if isinstance(after.get("display"), Mapping) else {}
    if (section or {}).get("extra", {}).get("hermes_adapter.platform_key") != request.platform_key:
        raise InstallError("the platform section did not read back")
    if (display or {}).get("busy_input_mode") != request.busy_input_mode:
        raise InstallError("busy_input_mode did not read back")
    directory = bridge.home / "plugins" / request.platform_key
    for name, text in plugin_entry(request.platform_key).items():
        place(directory / name, text)
    _, lines = _prediction(bridge)
    return ["installed", *_summary(bridge, after, request.platform_key), *lines]


def switch_mode(bridge: ProfileBridge, mode: str, credential_holder: UUID | None) -> list[str]:
    """``switch-mode``: change the profile environment for ``organization`` or ``personal``.

    Args:
        bridge: Hermes's reader and writer for the profile.
        mode: ``organization`` or ``personal``.
        credential_holder: The principal ID of the person whose login ``personal`` uses.

    Returns:
        The printed summary and prediction (no values).

    Raises:
        InstallError: An unknown mode, ``personal`` without a holder, ``organization`` without
            an API key or with a login file of another tool in the profile's home, or no adapter
            installed.
    """
    config = bridge.read_config()
    key = _installed_key(config)
    if key is None:
        raise InstallError("the adapter is not installed in this profile")
    allowed = f"{key.upper()}_ALLOWED_USERS"
    if mode == "organization":
        if not bridge.env_defines(bridge.api_key_variable()):
            raise InstallError(f"{bridge.api_key_variable()} must be set for organization")
        if (bridge.account_home() / _LOGIN_FILE).exists():
            raise InstallError("a login file of another tool lies in the profile's home")
        for name in bridge.oauth_variables():
            bridge.remove_env(name)
        bridge.remove_env(allowed)
        _nested(config, "auth")["adopt_external_logins"] = False
        bridge.write_config(config)
    elif mode == "personal":
        if credential_holder is None:
            raise InstallError("personal needs --credential-holder")
        bridge.remove_env(bridge.api_key_variable())
        bridge.set_env(allowed, str(credential_holder))
    else:
        raise InstallError("mode must be organization or personal")
    after = bridge.read_config()
    _, lines = _prediction(bridge)
    return [f"switched to {mode}", *_summary(bridge, after, key), *lines]


def check(bridge: ProfileBridge) -> list[str]:
    """``check``: the reads and the prediction without a change.

    Args:
        bridge: Hermes's reader and writer for the profile.

    Returns:
        The printed summary (no values).
    """
    config = bridge.read_config()
    _, lines = _prediction(bridge)
    return ["check", *_summary(bridge, config, _installed_key(config)), *lines]
