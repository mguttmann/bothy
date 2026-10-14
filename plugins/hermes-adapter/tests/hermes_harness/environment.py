# SPDX-License-Identifier: Apache-2.0
"""Throwaway homes and the allowlisted environment of the Hermes process tree.

Every process the harness starts gets an environment built from an allowlist, never the
caller's environment: no API key, token, agent socket, session bus or cloud credential of
the machine reaches it. Homes, caches and temporary files live in one temporary root, and
shims named like the credential store tools come first on ``PATH``, refuse and record the
call.

Example:
    >>> homes = ThrowawayHomes.create(root)
    >>> env = build_environment(homes, "probe", proxy.url, {}, venv_bin=install.venv / "bin")
"""

from __future__ import annotations

import os
import re
import sys
from collections.abc import Iterable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Final

#: The keyring backend that stores and finds nothing.
NULL_KEYRING: Final = "keyring.backends.null.Keyring"
#: Credential store tools that must stay unreachable (macOS keychain, Secret Service).
STORE_SHIMS: Final = ("security", "secret-tool")
#: Service managers; Hermes must never install, start or query a system service.
SERVICE_SHIMS: Final = ("launchctl", "systemctl")
_PROXY_NAMES: Final = ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY")
#: Every name the Hermes process tree may receive besides a scenario's own names.
ALLOWED_NAMES: Final = frozenset(
    {
        "PATH",
        "HOME",
        "HERMES_HOME",
        "XDG_CONFIG_HOME",
        "XDG_DATA_HOME",
        "XDG_CACHE_HOME",
        "XDG_STATE_HOME",
        "TMPDIR",
        "LANG",
        "TZ",
        "PYTHONDONTWRITEBYTECODE",
        "PYTHON_KEYRING_BACKEND",
        "NO_COLOR",
        "TERM",
        "NO_PROXY",
        "no_proxy",
        *_PROXY_NAMES,
        *(name.lower() for name in _PROXY_NAMES),
    }
)
_EXTRA_NAME: Final = re.compile(r"[A-Z][A-Z0-9_]*")
_PROFILE: Final = re.compile(r"[a-z0-9_-]{1,40}")
_SHIM: Final = """#!/bin/sh
# Shim of the recording harness: refuse and record the tool and its subcommand only.
printf '%s %s\\n' "$(basename "$0")" "$1" >> '{log}'
exit 1
"""


class HarnessEnvironmentError(RuntimeError):
    """A path leaves the temporary root, or an environment name is not allowed."""


def assert_inside(root: Path, path: Path) -> Path:
    """Resolve ``path`` (following symlinks) and refuse it outside ``root``.

    Args:
        root: The temporary root of the run.
        path: A path that must lie below ``root``.

    Returns:
        The resolved path.

    Raises:
        HarnessEnvironmentError: The resolved path is outside the resolved root.
    """
    resolved = path.resolve()
    if not resolved.is_relative_to(root.resolve()):
        raise HarnessEnvironmentError(f"{path} resolves to {resolved}, outside {root}")
    return resolved


@dataclass(frozen=True)
class ThrowawayHomes:
    """The directories of one run below a temporary root.

    Attributes:
        root: The temporary root (outside the repository).
        home: ``HOME`` of every started process.
        work: Working directory of the gateway.
        tmp: ``TMPDIR``.
        shims: Directory of the refusing shims, first on ``PATH``.
        xdg: Base of the XDG directories.
    """

    root: Path
    home: Path
    work: Path
    tmp: Path
    shims: Path
    xdg: Path

    @classmethod
    def create(cls, root: Path) -> ThrowawayHomes:
        """Create the directories and the shims below ``root``.

        Args:
            root: An empty or new directory, normally from ``mktemp -d``.

        Returns:
            The homes.
        """
        root.mkdir(parents=True, exist_ok=True)
        root = root.resolve()
        homes = cls(
            root=root,
            home=root / "home",
            work=root / "work",
            tmp=root / "tmp",
            shims=root / "shims",
            xdg=root / "xdg",
        )
        for path in (homes.home, homes.work, homes.tmp, homes.shims, homes.xdg, root / "hermes"):
            path.mkdir(parents=True, exist_ok=True)
        for sub in ("config", "data", "cache", "state"):
            (homes.xdg / sub).mkdir(exist_ok=True)
        for name in (*STORE_SHIMS, *SERVICE_SHIMS):
            shim = homes.shims / name
            shim.write_text(_SHIM.format(log=homes.store_log), encoding="utf-8")
            shim.chmod(0o755)
        return homes

    @property
    def store_log(self) -> Path:
        """The file the shims append their calls to."""
        return self.root / "store-calls.log"

    def hermes(self, profile: str) -> Path:
        """Return (and create) the Hermes home of one configuration.

        Args:
            profile: Lower-case name of the configuration, for example the scenario name.

        Returns:
            ``<root>/hermes/<profile>``.

        Raises:
            HarnessEnvironmentError: The name is not a plain lower-case identifier.
        """
        if not _PROFILE.fullmatch(profile):
            raise HarnessEnvironmentError(f"profile name {profile!r} is not allowed")
        path = self.root / "hermes" / profile
        path.mkdir(parents=True, exist_ok=True)
        return assert_inside(self.root, path)

    def store_calls(self) -> list[str]:
        """Return the recorded shim calls (``<tool> <subcommand>``), oldest first."""
        if not self.store_log.is_file():
            return []
        return [line.strip() for line in self.store_log.read_text().splitlines() if line.strip()]


def build_environment(
    homes: ThrowawayHomes,
    profile: str,
    proxy_url: str,
    extra: Mapping[str, str],
    *,
    venv_bin: Path,
) -> dict[str, str]:
    """Build the environment of a Hermes process from the allowlist.

    Args:
        homes: The throwaway homes of the run.
        profile: The configuration whose Hermes home becomes ``HERMES_HOME``.
        proxy_url: URL of the egress recorder; every proxy variable points to it.
        extra: The scenario's own names (fake credentials, base URLs).
        venv_bin: ``bin`` directory of the throwaway venv.

    Returns:
        The complete environment; nothing of ``os.environ`` is in it.

    Raises:
        HarnessEnvironmentError: An extra name is malformed or would replace an allowlisted name.
    """
    env = {
        "PATH": os.pathsep.join([str(homes.shims), str(venv_bin), "/usr/bin", "/bin"]),
        "HOME": str(homes.home),
        "HERMES_HOME": str(homes.hermes(profile)),
        "XDG_CONFIG_HOME": str(homes.xdg / "config"),
        "XDG_DATA_HOME": str(homes.xdg / "data"),
        "XDG_CACHE_HOME": str(homes.xdg / "cache"),
        "XDG_STATE_HOME": str(homes.xdg / "state"),
        "TMPDIR": str(homes.tmp),
        "LANG": "C.UTF-8",
        "TZ": "UTC",
        "PYTHONDONTWRITEBYTECODE": "1",
        "PYTHON_KEYRING_BACKEND": NULL_KEYRING,
        "NO_COLOR": "1",
        "TERM": "dumb",
        "NO_PROXY": "127.0.0.1,localhost",
        "no_proxy": "127.0.0.1,localhost",
    }
    for name in _PROXY_NAMES:
        env[name] = proxy_url
        env[name.lower()] = proxy_url
    for name, value in extra.items():
        if not _EXTRA_NAME.fullmatch(name):
            raise HarnessEnvironmentError(f"extra environment name {name!r} is malformed")
        if name in ALLOWED_NAMES or name.upper() in {n.upper() for n in ALLOWED_NAMES}:
            raise HarnessEnvironmentError(f"extra environment name {name!r} would replace {name}")
        env[name] = value
    return env


def build_provisioning_environment(
    homes: ThrowawayHomes,
    *,
    uv_cache_dir: Path,
    uv_python_dir: Path,
    tool_dirs: Iterable[Path],
) -> dict[str, str]:
    """Build the environment of the checkout and install steps.

    These steps need the network for the pinned downloads, so there is no proxy; the throwaway
    ``HOME`` keeps every git or uv credential helper and configuration of the caller away.

    Args:
        homes: The throwaway homes of the run.
        uv_cache_dir: The caller's uv cache, so repeated runs do not download again.
        uv_python_dir: The caller's managed Python directory.
        tool_dirs: Directories of ``git`` and ``uv``, first on ``PATH``.

    Returns:
        The environment.
    """
    path = [str(d) for d in tool_dirs] + ["/usr/bin", "/bin"]
    return {
        "PATH": os.pathsep.join(dict.fromkeys(path)),
        "HOME": str(homes.home),
        "TMPDIR": str(homes.tmp),
        "LANG": "C.UTF-8",
        "TZ": "UTC",
        "NO_COLOR": "1",
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_TERMINAL_PROMPT": "0",
        "UV_CACHE_DIR": str(uv_cache_dir),
        "UV_PYTHON_INSTALL_DIR": str(uv_python_dir),
        "PYTHONDONTWRITEBYTECODE": "1",
    }


def interpreter_dirs() -> list[Path]:
    """Directories the harness interpreter itself is read from (base and venv prefix)."""
    return sorted({Path(sys.base_prefix).resolve(), Path(sys.prefix).resolve()})
