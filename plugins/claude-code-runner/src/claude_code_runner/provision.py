# SPDX-License-Identifier: Apache-2.0
"""Creation of a project folder for a binding with ``provision`` ``init`` or ``clone`` (09 18.12).

The server asks for a folder through the binding; the runner, which owns
``runner.projects_root``, creates it in the background and reports the outcome as a readiness
reason (``provisioning``, ``workspace_exists``, ``init_failed``, ``clone_failed``,
``provision_timeout``, ``provision_refused``). ``git`` runs from an argument list, never a
shell, with ``--`` before every operand, no prompt and only the ``https`` protocol. The runner
never writes into a folder it did not create: a target that exists without an entry in the
creation record (``<runner.state_dir>/provisioned.json``) is ``workspace_exists``. A failure
removes only what the failed attempt created. Output of ``git`` is discarded; it can hold URLs.

Examples:
    >>> Want("website", "init", None, "main").key()
    ('website', 'init', None, 'main')
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import os
import re
import shutil
import signal
import time
import unicodedata
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit
from uuid import UUID, uuid4

from claude_code_runner.environment import kept_environment
from claude_code_runner.statedir import write_private
from claude_code_runner.workspace import WorkspaceError, readiness, resolve

__all__ = ["Provisioner", "Want", "want_of"]

_LOG = logging.getLogger("claude_code_runner")

_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]*")
_RECORD = "provisioned.json"
_TMP_PREFIX = ".provision-"
_CREATES = ("init", "clone")


@dataclass(frozen=True)
class Want:
    """What a binding asks for: the members of 09 18.12 that decide a creation.

    Attributes:
        workspace_ref: The binding's ``workspace_ref``.
        provision: ``none``, ``init`` or ``clone``.
        repository_url: The URL to clone, or ``None``.
        default_branch: The initial branch of ``init``, or ``None``.
    """

    workspace_ref: str
    provision: str
    repository_url: str | None
    default_branch: str | None

    def key(self) -> tuple[str, str, str | None, str | None]:
        """The binding content a failure is remembered for (D-M4-43-6)."""
        return (self.workspace_ref, self.provision, self.repository_url, self.default_branch)


def want_of(binding: Any) -> Want:
    """Read the members of a generated ``ProjectBinding`` (unset members are ``None``).

    Args:
        binding: The binding of the channel cache.

    Returns:
        The request of the binding; an absent ``provision`` is ``none``.
    """

    def text(name: str) -> str | None:
        value = getattr(binding, name, None)
        value = getattr(value, "value", value)
        return value if isinstance(value, str) else None

    return Want(
        workspace_ref=binding.workspace_ref,
        provision=text("provision") or "none",
        repository_url=text("repository_url"),
        default_branch=text("default_branch"),
    )


@dataclass
class _Failure:
    key: tuple[str, str, str | None, str | None]
    code: str
    at: float


class Provisioner:
    """Creates the folders of bound forums and answers their readiness with a reason.

    Args:
        projects_root: ``runner.projects_root``.
        state_dir: ``runner.state_dir`` (holds the creation record).
        git_bin: The ``git`` found from ``runner.git_bin`` in the cleaned ``PATH``, or ``None``.
        environ: The runner's own environment (the allow list is taken from it).
        timeout: ``runner.provision_timeout`` in seconds.
        retry_after: ``runner.provision_retry_after`` in seconds.
        clock: A monotonic clock for the retry window.
        on_done: Called after every creation ends (the runner sends a heartbeat at once).
    """

    def __init__(  # noqa: PLR0913 - the parts of a creation are explicit
        self,
        *,
        projects_root: Path,
        state_dir: Path,
        git_bin: str | None,
        environ: Mapping[str, str],
        timeout: float,
        retry_after: float,
        clock: Callable[[], float] = time.monotonic,
        on_done: Callable[[], Awaitable[None]] | None = None,
    ) -> None:
        """Create the provisioner; the record is read once here."""
        self._root = projects_root
        self._record_path = state_dir / _RECORD
        self._git = git_bin
        self._env = {
            **kept_environment(environ),
            "GIT_TERMINAL_PROMPT": "0",
            "GIT_ALLOW_PROTOCOL": "https",
        }
        self._timeout = timeout
        self._retry_after = retry_after
        self._clock = clock
        self._on_done = on_done
        self._record = self._load_record()
        self._failures: dict[UUID, _Failure] = {}
        self._jobs: dict[UUID, asyncio.Task[None]] = {}

    # ---- readiness -----------------------------------------------------------------------

    def readiness(self, channel_id: UUID, want: Want) -> tuple[bool, str | None]:
        """Answer the readiness of a bound forum and start its creation when it is due.

        Args:
            channel_id: The forum.
            want: What its binding asks for.

        Returns:
            ``(True, None)`` or ``(False, reason)`` with a code of 09 18.12.
        """
        if want.provision == "none" or self._record.get(str(channel_id)) == want.workspace_ref:
            return readiness(self._root, want.workspace_ref)
        if channel_id in self._jobs:
            return False, "provisioning"
        failure = self._failures.get(channel_id)
        if (
            failure is not None
            and failure.key == want.key()
            and self._clock() - failure.at < self._retry_after
        ):
            return False, failure.code
        target = self._checked_target(want)
        if target is None:
            self._fail(channel_id, want, "provision_refused")
            return False, "provision_refused"
        if os.path.lexists(self._root / want.workspace_ref) or os.path.lexists(target):
            # Also a link under the name, dangling or not: the runner writes through no link.
            return False, "workspace_exists"
        self._start(channel_id, want, target)
        return False, "provisioning"

    def _checked_target(self, want: Want) -> Path | None:
        """The target of a creation, or ``None`` when a check of 18.12 item 2 refuses it."""
        if want.provision not in _CREATES or not _NAME.fullmatch(want.workspace_ref):
            return None
        if want.default_branch is not None and not _argument(want.default_branch):
            return None
        if want.provision == "clone" and not _clone_url(want.repository_url):
            return None
        try:
            target = resolve(self._root, want.workspace_ref)
        except WorkspaceError:
            return None
        real_root = Path(os.path.realpath(self._root))
        if target.parent != real_root:
            return None
        return real_root / want.workspace_ref

    # ---- creation ------------------------------------------------------------------------

    def _start(self, channel_id: UUID, want: Want, target: Path) -> None:
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            # Readiness computed outside the event loop starts nothing; the next heartbeat,
            # which runs inside it, starts the creation.
            return
        self._failures.pop(channel_id, None)
        self._jobs[channel_id] = loop.create_task(self._create(channel_id, want, target))

    async def _create(self, channel_id: UUID, want: Want, target: Path) -> None:
        failed = "init_failed" if want.provision == "init" else "clone_failed"
        try:
            code = await (
                self._init(target, want) if want.provision == "init" else self._clone(target, want)
            )
        except asyncio.CancelledError:
            self._jobs.pop(channel_id, None)
            raise
        except Exception:  # any other error is a failed creation, never a success
            code = failed
        self._jobs.pop(channel_id, None)
        if code is None:
            self._record[str(channel_id)] = want.workspace_ref
            self._save_record()
            _LOG.info(json.dumps({"event": "workspace_created", "channel_id": str(channel_id)}))
        else:
            self._fail(channel_id, want, code)
        if self._on_done is not None:
            await self._on_done()

    async def _init(self, target: Path, want: Want) -> str | None:
        try:
            target.mkdir()
        except OSError:
            return "init_failed"
        argv = ["init", "--quiet"]
        if want.default_branch:
            argv += ["--initial-branch", want.default_branch]
        try:
            code = await self._git_run(argv, cwd=target, failed="init_failed")
        except BaseException:
            _remove_tree(target)
            raise
        if code is not None:
            _remove_tree(target)
        return code

    async def _clone(self, target: Path, want: Want) -> str | None:
        try:
            target.mkdir()
        except OSError:
            return "clone_failed"
        tmp = target.parent / f"{_TMP_PREFIX}{uuid4().hex}"
        try:
            code = await self._git_run(
                [
                    "clone",
                    "--quiet",
                    "--no-recurse-submodules",
                    "--",
                    str(want.repository_url),
                    str(tmp),
                ],
                cwd=target.parent,
                failed="clone_failed",
            )
            if code is None:
                # The rename replaces the empty folder that holds the name since the start; it
                # fails when anything appeared in it meanwhile.
                try:
                    tmp.rename(target)
                except OSError:
                    code = "clone_failed"
        except BaseException:
            _remove_tree(tmp)
            _remove_empty(target)
            raise
        if code is not None:
            _remove_tree(tmp)
            _remove_empty(target)
        return code

    async def _git_run(self, argv: list[str], *, cwd: Path, failed: str) -> str | None:
        """Run ``git`` with ``argv``; ``None`` on success, else the failure code."""
        if self._git is None:
            _LOG.warning(json.dumps({"event": "provision_failed", "code": "git_missing"}))
            return failed
        try:
            proc = await asyncio.create_subprocess_exec(
                self._git,
                *argv,
                cwd=cwd,
                env=self._env,
                stdin=asyncio.subprocess.DEVNULL,
                stdout=asyncio.subprocess.DEVNULL,
                stderr=asyncio.subprocess.DEVNULL,
                start_new_session=True,
            )
        except OSError:
            return failed
        try:
            status = await asyncio.wait_for(proc.wait(), self._timeout)
        except TimeoutError:
            _kill_group(proc.pid)
            await proc.wait()
            return "provision_timeout"
        except asyncio.CancelledError:
            _kill_group(proc.pid)
            await proc.wait()
            raise
        if status != 0:
            _LOG.warning(json.dumps({"event": "provision_failed", "code": failed, "exit": status}))
            return failed
        return None

    def _fail(self, channel_id: UUID, want: Want, code: str) -> None:
        self._failures[channel_id] = _Failure(want.key(), code, self._clock())
        _LOG.warning(
            json.dumps(
                {
                    "event": "provision_refused"
                    if code == "provision_refused"
                    else "provision_ended",
                    "channel_id": str(channel_id),
                    "code": code,
                }
            )
        )

    # ---- record and shutdown -------------------------------------------------------------

    def _load_record(self) -> dict[str, str]:
        try:
            data = json.loads(self._record_path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            return {}
        if not isinstance(data, dict):
            return {}
        return {str(k): v for k, v in data.items() if isinstance(v, str)}

    def _save_record(self) -> None:
        write_private(self._record_path, json.dumps(self._record, sort_keys=True), atomic=True)

    async def wait_idle(self) -> None:
        """Wait until every running creation has ended (tests and orderly shutdown)."""
        while self._jobs:
            await asyncio.gather(*list(self._jobs.values()), return_exceptions=True)

    async def close(self) -> None:
        """Stop every running creation; what it created is removed."""
        jobs = list(self._jobs.values())
        for job in jobs:
            job.cancel()
        await asyncio.gather(*jobs, return_exceptions=True)


def _argument(value: str) -> bool:
    """A process argument: not empty, not an option, no white space or control character."""
    return (
        bool(value)
        and not value.startswith("-")
        and not any(c.isspace() or unicodedata.category(c).startswith("C") for c in value)
    )


def _clone_url(value: str | None) -> bool:
    """An ``https`` URL with a host and without user information, query or fragment (18.12)."""
    if value is None or not _argument(value) or "#" in value or "?" in value:
        return False
    try:
        parts = urlsplit(value)
        host = parts.hostname
    except ValueError:
        return False
    return (
        parts.scheme == "https" and bool(host) and parts.username is None and parts.password is None
    )


def _kill_group(pid: int) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pid, signal.SIGKILL)


def _remove_tree(path: Path) -> None:
    """Remove a folder this attempt created (never a link: ``rmtree`` refuses one)."""
    with contextlib.suppress(FileNotFoundError):
        shutil.rmtree(path)


def _remove_empty(path: Path) -> None:
    """Remove the folder that held the name, only while it is still empty."""
    with contextlib.suppress(OSError):
        path.rmdir()
