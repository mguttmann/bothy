# SPDX-License-Identifier: Apache-2.0
"""One ``claude -p`` call without a session (09-agents.md 18.17, 26-builtin-agents.md 6.2).

The search assistant and the builder of the interview each run one call of the CLI per request:
the data on stdin, the reply on stdout, a time limit, the process group killed when the limit
passes or the caller is cancelled. This module holds that call once for both.

Examples:
    >>> import asyncio
    >>> out = asyncio.run(run_once(["/bin/echo", "ok"], {}, ".", b"", 5.0, failed=print))
    >>> out.strip()
    b'ok'
"""

from __future__ import annotations

import asyncio
import contextlib
import os
import signal
from collections.abc import Callable, Mapping, Sequence
from pathlib import Path

__all__ = ["kill_group", "run_once"]


async def run_once(  # noqa: PLR0913 - the parts of one process call
    argv: Sequence[str],
    env: Mapping[str, str],
    cwd: Path | str,
    data: bytes,
    timeout: float,
    *,
    failed: Callable[[str], None],
) -> bytes | None:
    """Run ``argv`` once with ``data`` on stdin and return its stdout.

    Args:
        argv: The command line.
        env: The whole environment of the process.
        cwd: The working folder.
        data: The bytes written to stdin.
        timeout: The time limit in seconds; then the process group is killed.
        failed: Called with a code (``runtime_unreachable``, ``timeout``, ``exit_<n>``) when the
            call gives no output; it never receives text of the call.

    Returns:
        The stdout of a call that exited with 0, else ``None``.

    Raises:
        asyncio.CancelledError: The caller was cancelled; the process group is killed first.
    """
    try:
        proc = await asyncio.create_subprocess_exec(
            *argv,
            env=dict(env),
            cwd=str(cwd),
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.DEVNULL,
            start_new_session=True,
        )
    except OSError:
        failed("runtime_unreachable")
        return None
    try:
        out, _ = await asyncio.wait_for(proc.communicate(data), timeout)
    except TimeoutError:
        kill_group(proc.pid)
        await proc.wait()
        failed("timeout")
        return None
    except asyncio.CancelledError:
        kill_group(proc.pid)
        await proc.wait()
        raise
    if proc.returncode != 0:
        failed("exit_" + str(proc.returncode))
        return None
    return out


def kill_group(pid: int) -> None:
    """Kill the process group of ``pid``; a group that is gone already is no error."""
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pid, signal.SIGKILL)
