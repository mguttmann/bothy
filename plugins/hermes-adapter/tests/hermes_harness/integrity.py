# SPDX-License-Identifier: Apache-2.0
"""Integrity of the pinned Hermes checkout and of the installed files.

The recording proves that it changed nothing: snapshots are taken after provisioning and
after the last scenario, and :func:`assert_unchanged` names every path that differs. The
checkout is read-only during the run (:func:`make_read_only`), so a write attempt fails at
once instead of being found afterwards.

Example:
    >>> before = snapshot_checkout(checkout)
    >>> ...  # run the scenarios
    >>> assert_unchanged(before, snapshot_checkout(checkout))
"""

from __future__ import annotations

import hashlib
import json
import os
import stat
import subprocess
from collections.abc import Iterable, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Literal, overload
from urllib.parse import unquote, urlparse

_WRITE_BITS = stat.S_IWUSR | stat.S_IWGRP | stat.S_IWOTH


class IntegrityError(AssertionError):
    """A checkout or an installation differs from its snapshot."""


@dataclass(frozen=True)
class CheckoutState:
    """Snapshot of a git checkout.

    Attributes:
        head: The commit SHA of ``HEAD``.
        status: Output of ``git status --porcelain --ignored`` (empty when clean).
        digest: :func:`tree_digest` of the working tree without ``.git``.
        entries: Per-path entry digests, used to name changed paths.
    """

    head: str
    status: str
    digest: str
    entries: Mapping[str, str] = field(default_factory=dict, compare=False)


def _entry(path: Path, rel: str) -> str:
    info = path.lstat()
    if stat.S_ISLNK(info.st_mode):
        return f"link {path.readlink()}"
    mode = stat.S_IMODE(info.st_mode)
    if stat.S_ISDIR(info.st_mode):
        return f"dir {mode:o}"
    content = hashlib.sha256(path.read_bytes()).hexdigest() if stat.S_ISREG(info.st_mode) else "-"
    return f"file {mode:o} {content}"


def _walk(root: Path, exclude: Iterable[str]) -> dict[str, str]:
    skip = set(exclude)
    entries: dict[str, str] = {}
    for current, dirs, files in os.walk(root, followlinks=False):
        here = Path(current)
        rel_dir = here.relative_to(root).as_posix()
        dirs[:] = sorted(d for d in dirs if (f"{rel_dir}/{d}" if rel_dir != "." else d) not in skip)
        for name in sorted([*files, *(d for d in dirs if (here / d).is_symlink())]):
            rel = f"{rel_dir}/{name}" if rel_dir != "." else name
            if rel not in skip:
                entries[rel] = _entry(here / name, rel)
        for name in dirs:
            if not (here / name).is_symlink():
                rel = f"{rel_dir}/{name}" if rel_dir != "." else name
                entries[f"{rel}/"] = _entry(here / name, rel)
    return entries


def _combine(entries: Mapping[str, str]) -> str:
    digest = hashlib.sha256()
    for rel in sorted(entries):
        digest.update(rel.encode("utf-8") + b"\0" + entries[rel].encode("utf-8") + b"\n")
    return digest.hexdigest()


@overload
def tree_digest(
    path: Path, *, exclude: Iterable[str] = (), entries: Literal[False] = False
) -> str: ...
@overload
def tree_digest(
    path: Path, *, exclude: Iterable[str] = (), entries: Literal[True]
) -> dict[str, str]: ...
def tree_digest(
    path: Path, *, exclude: Iterable[str] = (), entries: bool = False
) -> str | dict[str, str]:
    """Digest a directory tree.

    The digest is the SHA-256 over the sorted entries; each entry is the relative path, the
    file mode, the SHA-256 of the content of regular files and the target of symlinks.
    Directories are entries too (with a trailing slash), so an added empty directory counts.

    Args:
        path: The root of the tree.
        exclude: Relative paths (files or directories) to leave out, for example ``.git``.
        entries: When true, return the per-path entries instead of the digest.

    Returns:
        The hex digest, or the mapping of relative path to entry.
    """
    found = _walk(path, exclude)
    return found if entries else _combine(found)


def _git(path: Path, *args: str) -> str:
    # No caller environment: no config file of the user, no locks written into the checkout.
    env = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "HOME": str(path),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_OPTIONAL_LOCKS": "0",
        "LANG": "C",
    }
    result = subprocess.run(
        ["git", *args], cwd=path, env=env, check=True, capture_output=True, text=True
    )
    return result.stdout


def snapshot_checkout(path: Path) -> CheckoutState:
    """Record ``HEAD``, the porcelain status and the tree digest of a checkout.

    Args:
        path: The working tree of a git checkout.

    Returns:
        The snapshot.

    Raises:
        subprocess.CalledProcessError: ``path`` is no git checkout.
    """
    found = _walk(path, (".git",))
    return CheckoutState(
        head=_git(path, "rev-parse", "HEAD").strip(),
        status=_git(path, "status", "--porcelain", "--ignored"),
        digest=_combine(found),
        entries=found,
    )


def _site_packages(venv: Path) -> list[Path]:
    found = sorted(venv.glob("lib/python*/site-packages")) + sorted(venv.glob("Lib/site-packages"))
    if not found:
        raise IntegrityError(f"{venv}: no site-packages directory")
    return found


def _editable_source(info: Path) -> Path | None:
    direct = info / "direct_url.json"
    if not direct.is_file():
        return None
    data = json.loads(direct.read_text(encoding="utf-8"))
    if not (data.get("dir_info") or {}).get("editable"):
        return None
    url = urlparse(str(data.get("url", "")))
    return Path(unquote(url.path)) if url.scheme == "file" else None


def installed_entries(venv: Path) -> dict[str, str]:
    """Per-path entries of every installed distribution of a virtual environment.

    Each distribution contributes the files its ``RECORD`` lists; an editable install also
    contributes its source tree, because that is the code the interpreter runs.

    Args:
        venv: The virtual environment.

    Returns:
        Mapping of a label (``<dist-info>:<path>`` or ``<dist-info>:src:<path>``) to entry.

    Raises:
        IntegrityError: ``venv`` has no site-packages directory.
    """
    entries: dict[str, str] = {}
    for site in _site_packages(venv):
        for info in sorted(site.glob("*.dist-info")):
            record = info / "RECORD"
            rows = record.read_text(encoding="utf-8").splitlines() if record.is_file() else []
            for row in sorted(rows):
                rel = row.split(",", 1)[0]
                if not rel:
                    continue
                target = site / rel
                label = f"{info.name}:{rel}"
                entries[label] = _entry(target, rel) if target.exists() else "missing"
            source = _editable_source(info)
            if source is not None:
                for rel, entry in _walk(source, (".git",)).items():
                    entries[f"{info.name}:src:{rel}"] = entry
    return entries


def installed_digest(venv: Path) -> str:
    """Digest the installed distributions of a virtual environment (see installed_entries).

    Args:
        venv: The virtual environment.

    Returns:
        The hex digest; an added distribution (for example a lazy install) changes it.

    Raises:
        IntegrityError: ``venv`` has no site-packages directory.
    """
    return _combine(installed_entries(venv))


def assert_unchanged(before: CheckoutState | str, after: CheckoutState | str) -> None:
    """Fail when a snapshot differs.

    Args:
        before: Snapshot after provisioning (a :class:`CheckoutState` or an installed digest).
        after: Snapshot after the last scenario, of the same kind.

    Raises:
        IntegrityError: Something differs; the message names ``HEAD``, the status and each
            changed, added and removed path.
    """
    if isinstance(before, str) or isinstance(after, str):
        if before != after:
            raise IntegrityError(f"installed files changed: {before} -> {after}")
        return
    problems: list[str] = []
    if before.head != after.head:
        problems.append(f"HEAD moved: {before.head} -> {after.head}")
    if before.status != after.status:
        problems.append(f"status changed: {after.status.strip()!r}")
    if before.digest != after.digest:
        old, new = before.entries, after.entries
        problems += [f"changed: {p}" for p in sorted(set(old) & set(new)) if old[p] != new[p]]
        problems += [f"added: {p}" for p in sorted(set(new) - set(old))]
        problems += [f"removed: {p}" for p in sorted(set(old) - set(new))]
        if not (set(old) | set(new)):
            problems.append("tree digest changed")
    if problems:
        raise IntegrityError("checkout changed: " + "; ".join(problems))


def _chmod_tree(root: Path, *, writable: bool) -> None:
    paths = [root]
    for current, dirs, files in os.walk(root, followlinks=False):
        paths += [Path(current) / name for name in (*dirs, *files)]
    # Directories last when removing bits (their children must stay reachable first); first
    # when restoring them.
    ordered = paths if writable else list(reversed(paths))
    for path in ordered:
        info = path.lstat()
        if stat.S_ISLNK(info.st_mode):
            continue
        mode = stat.S_IMODE(info.st_mode)
        path.chmod(mode | stat.S_IWUSR if writable else mode & ~_WRITE_BITS)


def make_read_only(root: Path) -> None:
    """Remove every write bit below ``root`` (symlinks untouched), so writes fail loudly.

    Args:
        root: The checkout.
    """
    _chmod_tree(root, writable=False)


def restore_writable(root: Path) -> None:
    """Give the owner its write bit back below ``root``, so the tree can be removed.

    Args:
        root: The checkout made read-only before.
    """
    _chmod_tree(root, writable=True)
