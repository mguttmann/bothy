# SPDX-License-Identifier: Apache-2.0
"""Install the adapter into a Hermes profile and switch its billing mode (09 17.2, 17.5, 17.7).

Three commands (``python -m bothy_hermes_adapter``, run with the profile's own Python, so no
module of the package starts a process; docs/DECISIONS.md D-M2-33-12):

* ``install``: places the plugin entry where the pinned version discovers it
  (``<home>/plugins/<platform key>``) as a self-contained directory (``packages.json`` names the
  directory of each package the plugin needs, a package that lay inside the installing Python
  environment is copied into ``vendor/``, because Hermes replaces that environment; 09 17.2),
  writes the platform section with the adapter's keys, the
  own access policy (``dm_policy: allowlist``), the adapter's control sender as slash-command
  admin, ``display.busy_input_mode`` (``steer`` or ``queue``; ``interrupt`` is refused) and
  ``busy_ack_enabled: false``, ``tool_progress: off`` and ``busy_steer_ack_enabled: false`` for
  the platform, and the home channel: the report channel of ``--report-channel`` (with an
  optional fixed ``--report-thread``), the target of ``deliver=<key>`` cron jobs (09 17.9), or
  a placeholder that refuses every report (Hermes would otherwise ask for one in the first
  turn's thread); it refuses to change an existing platform key
  (D-AG-19), because every thread would lose its Hermes session.
* ``switch-mode``: ``organization`` requires the API-key variable of the profile environment to
  be set (judged by its name), empties the OAuth variables and Hermes's allow list of the
  platform, refuses a login file of another tool in the profile's home and sets
  ``auth.adopt_external_logins: false``; ``personal`` empties the API-key variable and writes
  Hermes's allow list (``<KEY>_ALLOWED_USERS``) with exactly the credential holder (17.7 layer 3).
* ``check``: the same reads and the prediction without a change, and whether the gateway can
  load the adapter (``adapter: loadable`` or ``adapter: not loadable: <reasons>``; the command
  then exits with ``1``).

Every command reads the configuration with Hermes's own reader before and after, prints a summary
without any value (no URL, no path, no token, no key) and the mode Hermes would now resolve; it
never restarts a process and names the restart the operator must do (17.8, 10 10.1).

Examples:
    >>> plugin_entry("bothy")["plugin.yaml"].splitlines()[1]
    'name: bothy'
"""

from __future__ import annotations

import copy
import importlib.util
import json
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Final, Protocol
from uuid import UUID

from bothy_hermes_adapter.actual_mode import ActualMode, determine
from bothy_hermes_adapter.config import load_settings
from bothy_hermes_adapter.runtime import ResolvedCredential

__all__ = [
    "ADAPTER_LOADABLE",
    "ADAPTER_NOT_LOADABLE",
    "ADDED_PACKAGES",
    "CONTROL_SENDER",
    "HERMES_PACKAGES",
    "OWN_PACKAGES",
    "PACKAGES_FILE",
    "InstallError",
    "InstallRequest",
    "PackageSource",
    "ProfileBridge",
    "adapter_problems",
    "check",
    "install",
    "loadable",
    "locate_packages",
    "packages_manifest",
    "plugin_entry",
    "switch_mode",
]

CONTROL_SENDER: Final = "bothy-adapter-control"
"""The adapter's control sender (``platform.CONTROL_SENDER``); a test holds both equal."""

_LOGIN_FILE: Final = Path(".claude") / ".credentials.json"

HOME_CHANNEL_UNASSIGNED: Final = "unassigned"
"""``chat_id`` of the platform's home channel without a report channel (09 17.9): Hermes stops
asking for a home channel in the first turn's thread, and a report to it is refused."""


OWN_PACKAGES: Final = ("bothy_hermes_adapter", "bothy_sdk")
"""The adapter's own import packages: found before anything of Hermes' environment."""

ADDED_PACKAGES: Final = ("attr", "attrs")
"""Import packages the SDK needs beyond Hermes' environment (``attrs``): used only when the
environment lacks them."""

HERMES_PACKAGES: Final = ("cryptography", "httpx", "jwt", "websockets")
"""Import packages of the SDK's other dependencies, which Hermes' environment ships (11 11.3)."""

PACKAGES_FILE: Final = "packages.json"
"""File of the plugin directory naming the directory of each package (``first``, ``last``)."""

VENDOR_DIR: Final = "vendor"
"""Directory of the plugin directory with the copied packages."""

ADAPTER_LOADABLE: Final = "adapter: loadable"
ADAPTER_NOT_LOADABLE: Final = "adapter: not loadable"


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
        report_channel: The report channel (Hermes' home channel, 09 14, 17.9), if any.
        report_thread: A fixed thread of the report channel for every report, if any.
    """

    server_url: str = field(repr=False)
    instance_id: UUID
    token_file: Path = field(repr=False)
    busy_input_mode: str
    platform_key: str = "bothy"
    consumer: str = "default"
    dpop_key_file: Path | None = field(default=None, repr=False)
    report_channel: UUID | None = None
    report_thread: UUID | None = None


@dataclass(frozen=True)
class PackageSource:
    """Where the installing Python found one package the plugin needs.

    Attributes:
        name: The import name (``bothy_sdk``).
        directory: The package's directory.
        copy: The installer copies it into the plugin directory: an added package always, an
            own package when it lies inside the installing Python environment, which Hermes
            replaces; an own package outside it (a source tree) is named by its directory.
        first: An own package, found before Hermes' environment; otherwise found only when the
            environment lacks it.
    """

    name: str
    directory: Path
    copy: bool
    first: bool


def locate_packages(locate: Callable[[str], Path | None], environment: Path) -> list[PackageSource]:
    """Find every package the plugin needs (`OWN_PACKAGES`, `ADDED_PACKAGES`) and its placement.

    Args:
        locate: The directory of an import package in the installing Python, or ``None``.
        environment: The installing Python's environment (``sys.prefix``).

    Returns:
        One source per package.

    Raises:
        InstallError: A package is not importable in the installing Python.
    """
    root = environment.resolve()
    found = []
    for name in (*OWN_PACKAGES, *ADDED_PACKAGES):
        directory = locate(name)
        if directory is None:
            raise InstallError(f"{name} is not importable in this Python (see the README)")
        directory = directory.resolve()
        own = name in OWN_PACKAGES
        # An added package is always copied: wherever the operator put it for the install
        # (Hermes' environment, a directory of its own) need not outlive the install.
        copied = directory.is_relative_to(root) or not own
        found.append(PackageSource(name, directory, copy=copied, first=own))
    return found


def packages_manifest(packages: Sequence[PackageSource]) -> str:
    """``packages.json``: a copied package relative to the plugin directory, others absolute.

    Args:
        packages: The packages of `locate_packages`.

    Returns:
        The JSON text.

    Examples:
        >>> text = packages_manifest([PackageSource("attrs", Path("/e/attrs"), True, False)])
        >>> json.loads(text)
        {'first': {}, 'last': {'attrs': 'vendor/attrs'}}
    """
    data: dict[str, dict[str, str]] = {"first": {}, "last": {}}
    for package in packages:
        where = f"{VENDOR_DIR}/{package.name}" if package.copy else str(package.directory)
        data["first" if package.first else "last"][package.name] = where
    return json.dumps(data, indent=2, sort_keys=True) + "\n"


_PLUGIN_MODULE = """\
# SPDX-License-Identifier: Apache-2.0
\"\"\"Plugin entry written by the installer of bothy_hermes_adapter (09 17.2).

Hermes replaces the Python environment of a profile on many of its commands, so nothing of the
adapter lives there: packages.json names the directory of each package, and a finder serves
exactly those names, the adapter's own before the environment and the added dependencies only
when the environment lacks them. A missing package is named in the gateway log.
\"\"\"

import importlib.abc
import importlib.util
import json
import logging
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_FIX = "run `python -m bothy_hermes_adapter install` for this profile again"


class _Packages(importlib.abc.MetaPathFinder):
    bothy_plugin_packages = True

    def __init__(self, packages):
        self.packages = packages

    def find_spec(self, name, path=None, target=None):
        directory = self.packages.get(name)
        if directory is None:
            return None
        return importlib.util.spec_from_file_location(
            name, directory / "__init__.py", submodule_search_locations=[str(directory)]
        )


def _install_finders():
    try:
        listed = json.loads((_HERE / "packages.json").read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        message = f"bothy_hermes_adapter: packages.json of the plugin is unreadable ({exc}); {_FIX}"
        logging.getLogger("bothy_hermes_adapter").error(message)
        raise ImportError(message) from exc
    groups = {}
    missing = []
    for group in ("first", "last"):
        groups[group] = {}
        for name, where in sorted(listed.get(group, {}).items()):
            directory = (_HERE / where).resolve()
            if (directory / "__init__.py").is_file():
                groups[group][name] = directory
            else:
                missing.append(name)
    if missing:
        message = f"bothy_hermes_adapter: missing package(s) {', '.join(missing)}; {_FIX}"
        logging.getLogger("bothy_hermes_adapter").error(message)
        raise ImportError(message)
    sys.meta_path[:] = [
        f for f in sys.meta_path if not getattr(f, "bothy_plugin_packages", False)
    ]
    sys.meta_path.insert(0, _Packages(groups["first"]))
    sys.meta_path.append(_Packages(groups["last"]))


_install_finders()

from bothy_hermes_adapter.entry import make_register  # noqa: E402

register = make_register("{platform_key}")
"""


def plugin_entry(platform_key: str) -> dict[str, str]:
    """The files of the plugin directory the pinned version discovers (without the packages).

    Args:
        platform_key: `hermes_adapter.platform_key`.

    Returns:
        File name to text: ``plugin.yaml`` (``kind: platform``) and ``__init__.py``, which
        loads the packages ``packages.json`` names.
    """
    manifest = (
        "# SPDX-License-Identifier: Apache-2.0\n"
        f"name: {platform_key}\n"
        "label: Chat server adapter\n"
        "kind: platform\n"
        "version: 0.0.0\n"
        "description: Connects this Hermes profile to the chat server as one agent.\n"
    )
    module = _PLUGIN_MODULE.replace("{platform_key}", platform_key)
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
    if request.report_thread is not None and request.report_channel is None:
        raise InstallError("--report-thread needs --report-channel")
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
    if request.report_channel is not None:
        # Hermes' home channel is the target of `deliver=<key>` cron jobs and `hermes send
        # --to <key>` (09 14, 17.9); a thread makes every report go into that one thread.
        home: dict[str, Any] = {
            "platform": request.platform_key,
            "chat_id": str(request.report_channel),
            "name": "Reports",
        }
        if request.report_thread is not None:
            home["thread_id"] = str(request.report_thread)
        section["home_channel"] = home
    elif not isinstance(section.get("home_channel"), dict):
        # Hermes asks every platform without a home channel to set one, in the first turn's
        # thread; without a report channel the placeholder only stops the notice.
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
    bridge: ProfileBridge,
    request: InstallRequest,
    place: Callable[[Path, str], None],
    packages: Sequence[PackageSource],
    copy_package: Callable[[Path, Path], None],
) -> list[str]:
    """``install``: write the platform section and place the self-contained plugin directory.

    Args:
        bridge: Hermes's reader and writer for the profile.
        request: The arguments.
        place: Writes one file of the plugin entry (path, text).
        packages: The packages of `locate_packages`.
        copy_package: Replaces a package directory (source, target) with a copy.

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
    # Packages first, their list next and the entry last: a first install never leaves an
    # entry without the packages it names.
    for package in packages:
        if package.copy:
            copy_package(package.directory, directory / VENDOR_DIR / package.name)
    place(directory / PACKAGES_FILE, packages_manifest(packages))
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


def _importable(name: str) -> bool:
    return importlib.util.find_spec(name) is not None


def adapter_problems(directory: Path, enabled: bool, available: Callable[[str], bool]) -> list[str]:
    """Why a gateway of the profile could not load the adapter (empty: it can).

    Args:
        directory: The plugin directory (``<home>/plugins/<platform key>``).
        enabled: Whether ``plugins.enabled`` lists the platform key.
        available: Whether an import package is importable in Hermes' environment.

    Returns:
        The reasons, without a path or another value.
    """
    problems = []
    if not enabled:
        problems.append("the plugin is not enabled in plugins.enabled")
    entry = [name for name in ("plugin.yaml", "__init__.py") if not (directory / name).is_file()]
    if entry:
        problems.append(f"the plugin entry lacks {', '.join(entry)}")
    try:
        listed = json.loads((directory / PACKAGES_FILE).read_text(encoding="utf-8"))
    except (OSError, ValueError):
        problems.append(f"the plugin entry lacks a readable {PACKAGES_FILE}")
        listed = None
    if isinstance(listed, Mapping):
        named: dict[str, str] = {}
        for group in ("first", "last"):
            part = listed.get(group)
            named.update(part if isinstance(part, Mapping) else {})
        for name in (*OWN_PACKAGES, *ADDED_PACKAGES):
            where = named.get(name)
            if where is None or not (directory / where / "__init__.py").is_file():
                problems.append(f"package {name} is missing")
    problems.extend(
        f"package {name} is not in Hermes' environment"
        for name in HERMES_PACKAGES
        if not available(name)
    )
    if problems:
        problems.append("run install again")
    return problems


def loadable(lines: Sequence[str]) -> bool:
    """Whether the lines of `check` report no adapter that cannot be loaded (exit status)."""
    return not any(line.startswith(ADAPTER_NOT_LOADABLE) for line in lines)


def check(bridge: ProfileBridge, available: Callable[[str], bool] = _importable) -> list[str]:
    """``check``: the reads and the prediction without a change, and the adapter's state.

    Args:
        bridge: Hermes's reader and writer for the profile.
        available: Whether an import package is importable in Hermes' environment (default:
            in this Python, which is the profile's).

    Returns:
        The printed summary (no values); `loadable` decides the exit status.
    """
    config = bridge.read_config()
    key = _installed_key(config)
    state = []
    if key is not None:
        plugins = config.get("plugins") if isinstance(config.get("plugins"), Mapping) else {}
        listed = (plugins or {}).get("enabled")
        enabled = isinstance(listed, list) and key in listed
        problems = adapter_problems(bridge.home / "plugins" / key, enabled, available)
        state = [f"{ADAPTER_NOT_LOADABLE}: {'; '.join(problems)}" if problems else ADAPTER_LOADABLE]
    _, lines = _prediction(bridge)
    return ["check", *_summary(bridge, config, key), *state, *lines]
