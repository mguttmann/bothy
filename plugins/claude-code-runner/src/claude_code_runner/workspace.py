# SPDX-License-Identifier: Apache-2.0
"""Resolution of ``workspace_ref``, readiness and the project folder report (09 18.1, 18.13).

The reference is an opaque name; it resolves to a strict descendant of the real path of the
root, also after symbolic links, and the resolved real path is the working directory of a
turn (design note 6). Git is optional (RUNNER-NO-GIT): any folder the runner user may read,
write and search is ready. ``runner.default_workspace`` resolves the same way, and ``.`` names
the root itself.

Examples:
    >>> import tempfile, pathlib
    >>> root = pathlib.Path(tempfile.mkdtemp())
    >>> readiness(root, "website")
    (False, 'workspace_missing')
"""

from __future__ import annotations

import json
import logging
import os
import re
from pathlib import Path

__all__ = [
    "ROOT",
    "WorkspaceError",
    "default_readiness",
    "project_folders",
    "readiness",
    "resolve",
    "resolve_default",
]

ROOT = "."
"""The value of ``runner.default_workspace`` that names the projects root itself."""

_DRIVE = re.compile(r"^[A-Za-z]:")
_FOLDER_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]*")
_FOLDER_NAME_MAX = 255
_LOG = logging.getLogger("claude_code_runner")


class WorkspaceError(Exception):
    """A workspace that is not ready or not allowed.

    Attributes:
        code: ``outside_root``, ``workspace_missing`` or ``not_writable``.
    """

    def __init__(self, code: str) -> None:
        """Create the error with its code; the path is never part of the message."""
        super().__init__(code)
        self.code = code


def resolve(projects_root: Path, workspace_ref: str) -> Path:
    r"""Resolve a reference to its real directory path under the root.

    Args:
        projects_root: ``runner.projects_root``.
        workspace_ref: The binding's ``workspace_ref``.

    Returns:
        The real path (it may not exist).

    Raises:
        WorkspaceError: ``outside_root`` for an empty, absolute, ``~`` or ``-`` led, ``\\``,
            drive or ``..`` reference and for a path that is not a strict descendant of the
            root after resolving symbolic links.
    """
    ref = workspace_ref
    if (
        not ref
        or ref.startswith(("/", "~", "-"))
        or "\\" in ref
        or "\x00" in ref
        or _DRIVE.match(ref)
        or ".." in ref.split("/")
    ):
        raise WorkspaceError("outside_root")
    root = os.path.realpath(projects_root)
    target = os.path.realpath(Path(root) / ref)
    if target == root or os.path.commonpath([root, target]) != root:
        raise WorkspaceError("outside_root")
    return Path(target)


def resolve_default(projects_root: Path, value: str) -> Path:
    """Resolve ``runner.default_workspace``: ``.`` is the root, any other value as `resolve`.

    Args:
        projects_root: ``runner.projects_root``.
        value: ``runner.default_workspace`` (not empty).

    Returns:
        The real path (it may not exist).

    Raises:
        WorkspaceError: ``outside_root`` as for `resolve`.
    """
    if value == ROOT:
        return Path(os.path.realpath(projects_root))
    return resolve(projects_root, value)


def readiness(projects_root: Path, workspace_ref: str) -> tuple[bool, str | None]:
    """Tell whether a project is ready: resolved, a directory, and the rights fit.

    Git is optional (RUNNER-NO-GIT): a folder without ``.git`` is as ready as a work tree.

    Args:
        projects_root: ``runner.projects_root``.
        workspace_ref: The binding's ``workspace_ref``.

    Returns:
        ``(True, None)`` or ``(False, code)`` with a code of `WorkspaceError`.
    """
    try:
        path = resolve(projects_root, workspace_ref)
    except WorkspaceError as err:
        return False, err.code
    return _usable(path)


def default_readiness(projects_root: Path, value: str) -> tuple[bool, str | None]:
    """The readiness of ``runner.default_workspace``, with the codes of `readiness`.

    Args:
        projects_root: ``runner.projects_root``.
        value: ``runner.default_workspace`` (not empty).

    Returns:
        ``(True, None)`` or ``(False, code)``.
    """
    try:
        path = resolve_default(projects_root, value)
    except WorkspaceError as err:
        return False, err.code
    return _usable(path)


def _usable(path: Path) -> tuple[bool, str | None]:
    """A folder the runner user may read, write and search."""
    if not path.is_dir():
        return False, "workspace_missing"
    if not os.access(path, os.R_OK | os.W_OK | os.X_OK):
        return False, "not_writable"
    return True, None


def project_folders(projects_root: Path, limit: int) -> list[str]:
    """Return the names of the Git work trees directly under the root (09 18.13).

    An entry counts when its name is one segment of ``^[A-Za-z0-9][A-Za-z0-9._-]*$`` of at most
    255 characters (so no hidden folder and no ``.provision-<uuid>`` folder of 18.12), it
    resolves by `resolve` to a folder (a symbolic link only when its target stays under the
    root) and it holds ``.git`` (a folder or a file). Only names leave the runner, never a path.

    Args:
        projects_root: ``runner.projects_root``.
        limit: ``runner.project_folders_max``.

    Returns:
        The names in code point order, at most ``limit``; when there are more, the first ones,
        and the log code ``project_folders_truncated`` names the count. A root that cannot be
        read gives ``[]`` and the log code ``project_folders_unreadable``.
    """
    try:
        entries = [e.name for e in Path(os.path.realpath(projects_root)).iterdir()]
    except OSError:
        _LOG.warning(json.dumps({"event": "project_folders_unreadable"}))
        return []
    names = sorted(n for n in entries if _is_project_folder(projects_root, n))
    if len(names) > limit:
        _LOG.info(json.dumps({"event": "project_folders_truncated", "count": len(names)}))
        names = names[:limit]
    return names


def _is_project_folder(projects_root: Path, name: str) -> bool:
    """Whether the entry ``name`` of the root is a reported project folder."""
    if len(name) > _FOLDER_NAME_MAX or not _FOLDER_NAME.fullmatch(name):
        return False
    try:
        path = resolve(projects_root, name)
    except WorkspaceError:
        return False
    return path.is_dir() and (path / ".git").exists()
