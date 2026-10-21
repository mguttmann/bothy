# SPDX-License-Identifier: Apache-2.0
"""Resolution of ``workspace_ref`` under ``runner.projects_root`` and readiness (09 18.1).

The reference is an opaque name; it resolves to a strict descendant of the real path of the
root, also after symbolic links, and the resolved real path is the working directory of a
turn (design note 6).

Examples:
    >>> import tempfile, pathlib
    >>> root = pathlib.Path(tempfile.mkdtemp())
    >>> readiness(root, "website")
    (False, 'workspace_missing')
"""

from __future__ import annotations

import os
import re
from pathlib import Path

__all__ = ["WorkspaceError", "readiness", "resolve"]

_DRIVE = re.compile(r"^[A-Za-z]:")


class WorkspaceError(Exception):
    """A workspace that is not ready or not allowed.

    Attributes:
        code: ``outside_root``, ``workspace_missing``, ``not_a_git_worktree`` or
            ``not_writable``.
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


def readiness(projects_root: Path, workspace_ref: str) -> tuple[bool, str | None]:
    """Tell whether a project is ready: resolved, a directory, a Git work tree, rights fit.

    ``.git`` may be a directory or a file, so linked work trees count.

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
    if not path.is_dir():
        return False, "workspace_missing"
    if not (path / ".git").exists():
        return False, "not_a_git_worktree"
    if not os.access(path, os.R_OK | os.W_OK | os.X_OK):
        return False, "not_writable"
    return True, None
