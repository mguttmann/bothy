# SPDX-License-Identifier: Apache-2.0
"""Workspace trust of a bound project folder for background mode (FIX-RUNNER-NEW-PROJECT-TRUST).

The pinned CLI refuses a ``--bg`` launch in a folder without workspace trust (design
2.2 item 5). A folder the owner binds to a forum is the place the owner told the agent to
work, so before a background launch in a bound folder the runner records the trust the CLI
documents for it, ``projects["<real path>"].hasTrustDialogAccepted: true`` in the CLI's global
configuration file, unless it is there already.

Limits (ruling D-FIX-RUNNER-NEW-PROJECT-TRUST-1):

* Only the real path of the folder itself, and only when it is a strict descendant of the
  real path of ``runner.projects_root``; never the root, a parent or a path outside it.
* Only the one entry: every other member of the file stays as read.
* The write takes the CLI's own lock of the file (the folder ``<file>.lock`` that the CLI
  creates and removes around each of its writes) and replaces the file atomically, so a
  reader sees the old or the new content and a write of the CLI is never lost. A held lock
  is waited for at most ``wait`` seconds and never taken from its holder.
* A legacy configuration file ``<config dir>/.config.json``, which the CLI would read instead,
  or a file that is missing or no JSON object, is not written.

The outcome is a code; the caller logs it. A folder that stays untrusted makes the launch
refuse, and the run ends with the cause ``background_workspace_untrusted`` (09 18.9).

Examples:
    >>> import tempfile, pathlib
    >>> root = pathlib.Path(tempfile.mkdtemp())
    >>> grant_trust(root / ".claude.json", root, root, wait=0, config_dir=None)
    'outside_root'
"""

from __future__ import annotations

import contextlib
import json
import os
import time
from collections.abc import Mapping
from pathlib import Path

from claude_code_runner.statedir import write_private

__all__ = ["TRUST_KEY", "global_config_file", "grant_trust"]

TRUST_KEY = "hasTrustDialogAccepted"
"""The member of a project entry that records workspace trust (design 2.2 item 5)."""

_LEGACY = ".config.json"
_POLL = 0.05
"""The pause between two tries of a held lock, in seconds."""


def global_config_file(env: Mapping[str, str]) -> Path | None:
    """Return the CLI's global configuration file for a launch environment.

    Args:
        env: The environment of the launch (``CLAUDE_CONFIG_DIR``, else ``HOME``).

    Returns:
        ``$CLAUDE_CONFIG_DIR/.claude.json``, else ``$HOME/.claude.json``; ``None`` without
        either.
    """
    if env.get("CLAUDE_CONFIG_DIR"):
        return Path(env["CLAUDE_CONFIG_DIR"]) / ".claude.json"
    if env.get("HOME"):
        return Path(env["HOME"]) / ".claude.json"
    return None


def grant_trust(
    config_file: Path,
    folder: Path,
    projects_root: Path,
    *,
    wait: float,
    config_dir: Path | None,
) -> str:
    """Record workspace trust of ``folder`` in ``config_file`` (see the module docstring).

    Args:
        config_file: The CLI's global configuration file (`global_config_file`).
        folder: The project folder of the launch.
        projects_root: ``runner.projects_root``.
        wait: How long a lock the CLI holds is waited for, in seconds.
        config_dir: The CLI's configuration folder (``CLAUDE_CONFIG_DIR``, else
            ``$HOME/.claude``), where a legacy ``.config.json`` would lie.

    Returns:
        ``trusted`` (the entry was there), ``granted`` (the runner wrote it), or why it did
        not: ``outside_root``, ``legacy_config``, ``config_missing``, ``config_unreadable``,
        ``locked`` or ``write_failed``.
    """
    real = Path(os.path.realpath(folder))
    root = Path(os.path.realpath(projects_root))
    if real == root or root not in real.parents:
        return "outside_root"
    if config_dir is not None and _non_empty(config_dir / _LEGACY):
        return "legacy_config"
    key = str(real)
    first = _read(config_file)
    if isinstance(first, str):
        return first
    if _trusted(first, key):
        return "trusted"
    lock = Path(f"{config_file}.lock")
    refused = _take_lock(lock, wait)
    if refused is not None:
        return refused
    try:
        return _write_entry(config_file, key)  # read again under the lock: the CLI may have written
    finally:
        with contextlib.suppress(OSError):
            lock.rmdir()


def _take_lock(lock: Path, wait: float) -> str | None:
    """Create the CLI's lock folder; ``None`` once held, else ``locked`` or ``write_failed``."""
    deadline = time.monotonic() + wait
    while True:
        try:
            lock.mkdir()
        except FileExistsError:
            if time.monotonic() >= deadline:
                return "locked"
            time.sleep(min(_POLL, max(wait, 0)))
        except OSError:
            return "write_failed"
        else:
            return None


def _write_entry(config_file: Path, key: str) -> str:
    """Under the lock: read the file, set the one member, replace the file atomically."""
    document = _read(config_file)
    if isinstance(document, str):
        return document
    if _trusted(document, key):
        return "trusted"
    projects = document.get("projects")
    if not isinstance(projects, dict):
        projects = {}
        document["projects"] = projects
    entry = projects.get(key)
    projects[key] = {**entry, TRUST_KEY: True} if isinstance(entry, dict) else {TRUST_KEY: True}
    target = Path(os.path.realpath(config_file))
    try:
        write_private(target, json.dumps(document, indent=2, ensure_ascii=False), atomic=True)
    except OSError:
        return "write_failed"
    return "granted"


def _non_empty(path: Path) -> bool:
    try:
        return path.is_file() and path.stat().st_size > 0
    except OSError:
        return False


def _read(config_file: Path) -> dict[str, object] | str:
    try:
        raw = config_file.read_text("utf-8")
    except FileNotFoundError:
        return "config_missing"
    except (OSError, UnicodeDecodeError):
        return "config_unreadable"
    try:
        document = json.loads(raw)
    except ValueError:
        return "config_unreadable"
    return document if isinstance(document, dict) else "config_unreadable"


def _trusted(document: Mapping[str, object], key: str) -> bool:
    projects = document.get("projects")
    if not isinstance(projects, dict):
        return False
    entry = projects.get(key)
    return isinstance(entry, dict) and entry.get(TRUST_KEY) is True
