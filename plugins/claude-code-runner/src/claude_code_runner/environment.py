# SPDX-License-Identifier: Apache-2.0
"""The environment of a turn, built as an allow list (docs/spec/09-agents.md 18.2, 18.3).

The environment is built from scratch and never by removing names from ``os.environ``: a deny
list lets every new secret through. Kept from the runner's own environment: ``HOME``,
``USER``, ``LOGNAME``, ``SHELL``, ``LANG``, ``TERM``, ``TMPDIR``, every ``LC_*``, a cleaned
``PATH`` and, only with ``runner.forward_ssh_agent``, ``SSH_AUTH_SOCK``. ``TMPDIR`` of a run
is its own temporary directory ``<runner.state_dir>/tmp/<run_id>`` (18.2, M3-12 note 3 c), so
files a turn writes as scratch output can be attached and never mix with another run's. Mode
``organization`` adds ``ANTHROPIC_API_KEY`` and the runner's own ``CLAUDE_CONFIG_DIR``;
``personal`` adds nothing (10 10.2, design note 7).

Examples:
    >>> sorted(build_environment({"HOME": "/home/alex", "SECRET": "x"}, mode="personal",
    ...     api_key=None, config_dir=None, forward_ssh_agent=False))
    ['HOME', 'PATH']
"""

from __future__ import annotations

import os
import stat
from collections.abc import Mapping, Sequence
from pathlib import Path

from bothy_sdk.auth import Secret

__all__ = [
    "RunEnvironmentError",
    "build_environment",
    "clean_path",
    "find_program",
    "kept_environment",
]

_KEPT = ("HOME", "USER", "LOGNAME", "SHELL", "LANG", "TERM", "TMPDIR")
_WIDE = stat.S_IWGRP | stat.S_IWOTH


class RunEnvironmentError(Exception):
    """The run environment cannot be built.

    Attributes:
        code: ``home_missing`` or ``organization_needs_key``.
    """

    def __init__(self, code: str) -> None:
        """Create the error with its code."""
        super().__init__(code)
        self.code = code


def clean_path(value: str) -> list[str]:
    """Return the safe entries of a ``PATH`` value in their order.

    Kept are absolute, existing directories that neither group nor others may write, each once;
    empty entries and ``.`` go. A writable directory on the path would let another account plant
    a program the turn runs.

    Args:
        value: The ``PATH`` of the runner.

    Returns:
        The kept entries.
    """
    kept: list[str] = []
    for entry in value.split(os.pathsep):
        if not entry or not Path(entry).is_absolute() or entry in kept:
            continue
        try:
            info = Path(entry).stat()
        except OSError:
            continue
        if stat.S_ISDIR(info.st_mode) and not info.st_mode & _WIDE:
            kept.append(entry)
    return kept


def find_program(name: str, entries: Sequence[str]) -> str | None:
    """Find a program by name in ``entries`` (or check an absolute path).

    Args:
        name: ``runner.claude_bin``.
        entries: The cleaned ``PATH``.

    Returns:
        The absolute path of an executable regular file, or ``None``.
    """
    candidates = [Path(name)] if Path(name).is_absolute() else [Path(e) / name for e in entries]
    for candidate in candidates:
        if candidate.is_file() and os.access(candidate, os.X_OK):
            return str(candidate)
    return None


def kept_environment(parent: Mapping[str, str]) -> dict[str, str]:
    """Return the allow list of 18.2 from the runner's own environment, without any mode part.

    Kept: ``HOME``, ``USER``, ``LOGNAME``, ``SHELL``, ``LANG``, ``TERM``, ``TMPDIR``, every
    ``LC_*`` and the cleaned ``PATH``; the folder creation of 18.12 runs ``git`` with it.

    Args:
        parent: The runner's own environment.

    Returns:
        The environment.
    """
    env = {name: parent[name] for name in _KEPT if name in parent}
    env.update({k: v for k, v in parent.items() if k.startswith("LC_")})
    env["PATH"] = os.pathsep.join(clean_path(parent.get("PATH", "")))
    return env


def build_environment(  # noqa: PLR0913 - the parts of a run environment are explicit
    parent: Mapping[str, str],
    *,
    mode: str,
    api_key: Secret | None,
    config_dir: Path | None,
    forward_ssh_agent: bool,
    tmpdir: Path | None = None,
) -> dict[str, str]:
    """Build the environment of ``auth status`` and of the turn of one run.

    Args:
        parent: The runner's own environment.
        mode: ``organization`` or ``personal`` (the desired mode of the token).
        api_key: The API key of ``organization`` (read for this run).
        config_dir: ``<runner.state_dir>/claude-config/organization`` for ``organization``.
        forward_ssh_agent: ``runner.forward_ssh_agent``.
        tmpdir: The run's temporary directory; it replaces the runner's own ``TMPDIR``.

    Returns:
        The environment.

    Raises:
        RunEnvironmentError: ``home_missing``, or ``organization_needs_key`` without key or
            configuration directory.
    """
    if not parent.get("HOME"):
        raise RunEnvironmentError("home_missing")
    env = kept_environment(parent)
    if tmpdir is not None:
        env["TMPDIR"] = str(tmpdir)
    if forward_ssh_agent and parent.get("SSH_AUTH_SOCK"):
        env["SSH_AUTH_SOCK"] = parent["SSH_AUTH_SOCK"]
    if mode == "organization":
        if api_key is None or config_dir is None:
            raise RunEnvironmentError("organization_needs_key")
        env["ANTHROPIC_API_KEY"] = api_key.reveal()
        env["CLAUDE_CONFIG_DIR"] = str(config_dir)
    return env
