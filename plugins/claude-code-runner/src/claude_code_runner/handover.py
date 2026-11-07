# SPDX-License-Identifier: Apache-2.0
"""The handover at SIGTERM and ``claude-code-runner stop --turns`` (09 18.19.3).

With ``runner.turn_handover`` SIGTERM is a handover, not a stop (`hand_over`): the runner stops
taking new runs (the inbox stays as it is, 4.9) and sends no further heartbeat; it leaves every
live turn running with its record, finishes an input line in flight within ``runner.kill_grace``
or records it with its byte count, closes a launch pipe whose go byte is not recorded, abandons a
run in ``preparing`` or ``waiting_lock`` in memory with its record left, logs ``turn_handed_over``
per turn and exits with code 75 when at least one turn was handed over (else 0), so a service
manager that restarts on failure starts the next runner, which adopts them.

Only ``claude-code-runner stop --turns`` (`stop_turns`) or a stop of a run ends a turn on
purpose: it asks a running runner (named in ``<runner.state_dir>/runner.pid``) for the stop of
every turn as in 18.6 by the marker ``<runner.state_dir>/stop-turns`` and SIGTERM, waits for it to
exit, then stops what is left of the turn records itself (the turns of a runner that is not
running), finishing each run ``cancelled`` with the reason ``adapter_stopped``, and exits 0.

Examples:
    >>> HANDOVER_EXIT
    75
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import os
import signal
import time
from pathlib import Path
from typing import TYPE_CHECKING, Any

from claude_code_runner.adoption import read_identity

if TYPE_CHECKING:
    from bothy_sdk.adapter import Adapter

    from claude_code_runner.app import RunnerHandler
    from claude_code_runner.config import RunnerConfig

__all__ = [
    "HANDOVER_EXIT",
    "hand_over",
    "remove_pid_file",
    "stop_turns",
    "stop_turns_requested",
    "write_pid_file",
]

HANDOVER_EXIT = 75
"""``EX_TEMPFAIL``: the exit code after a SIGTERM that handed at least one turn over."""

_PID_FILE = "runner.pid"
_STOP_MARKER = "stop-turns"
_PROGRAM_MARKERS = ("claude_code_runner", "claude-code-runner")
_LOG = logging.getLogger("claude_code_runner")


def _log(event: str, **ids: object) -> None:
    _LOG.info(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def write_pid_file(state_dir: Path) -> None:
    """Name this runner in ``<runner.state_dir>/runner.pid`` (mode ``0600``) for ``stop --turns``.

    Args:
        state_dir: ``runner.state_dir``.
    """
    from claude_code_runner.statedir import ensure_private_dir, write_private  # noqa: PLC0415

    ensure_private_dir(state_dir)
    write_private(state_dir / _PID_FILE, json.dumps({"pid": os.getpid()}), atomic=True)


def remove_pid_file(state_dir: Path) -> None:
    """Remove the pid file when it names this process.

    Args:
        state_dir: ``runner.state_dir``.
    """
    path = state_dir / _PID_FILE
    with contextlib.suppress(OSError, ValueError, KeyError, TypeError):
        if int(json.loads(path.read_text())["pid"]) == os.getpid():
            path.unlink()


def stop_turns_requested(state_dir: Path) -> bool:
    """Whether ``stop --turns`` asked for the stop of every turn (the marker exists).

    Args:
        state_dir: ``runner.state_dir``.

    Returns:
        True while the marker exists.
    """
    return (state_dir / _STOP_MARKER).exists()


async def hand_over(handler: RunnerHandler, adapter: Adapter | None) -> int:
    """Hand every live turn over to the next runner (18.19.3).

    Args:
        handler: The runner's handler (its own and adopted turns).
        adapter: The SDK adapter; stopped with ``hand_over=True`` (no further heartbeat, its run
            tasks cancelled with `Adapter.handing_over` set).

    Returns:
        The number of turns handed over.
    """
    handler.handing_over = True
    count = 0
    turns = [s.turn for s in list(handler._runs.values()) if s.turn is not None]
    turns += list(handler.adopted.values())
    for turn in turns:
        if await turn.hand_over():
            count += 1
            _log("turn_handed_over", **turn.ids)
    if adapter is not None:
        await adapter.stop(hand_over=True)
    return count


def _runner_pid(state_dir: Path, timeout: float) -> int | None:
    """The running runner the pid file names, after its identity check, else ``None``."""
    try:
        pid = int(json.loads((state_dir / _PID_FILE).read_text())["pid"])
    except (OSError, ValueError, KeyError, TypeError):
        return None
    if pid <= 1 or pid == os.getpid():
        return None
    found = read_identity(pid, timeout)
    if found is None or not any(m in found.command for m in _PROGRAM_MARKERS):
        return None
    return pid


def _gone(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    except PermissionError:
        return False
    return False


def stop_turns(config: RunnerConfig) -> int:
    """``claude-code-runner stop --turns``: end every turn and its run on purpose (18.19.3).

    A running runner gets the stop marker and SIGTERM and stops every own and adopted turn as
    in 18.6 before it exits; the turn records left afterwards (turns of a runner that is not
    running) are stopped by this process the same way.

    Args:
        config: The runner configuration.

    Returns:
        0 once done.
    """
    state_dir = config.state_dir
    marker = state_dir / _STOP_MARKER
    pid = _runner_pid(state_dir, config.cli_probe_timeout)
    if pid is not None:
        marker.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        marker.touch(mode=0o600)
        with contextlib.suppress(ProcessLookupError):
            os.kill(pid, signal.SIGTERM)
        # The runner waits stop_grace plus kill_grace plus cli_probe_timeout per stop (18.19.2).
        bound = 2 * (config.stop_grace + config.kill_grace + config.cli_probe_timeout)
        deadline = time.monotonic() + bound
        # A zombie (exited, not yet reaped by its service manager) no longer matches.
        while _runner_pid(state_dir, config.cli_probe_timeout) == pid and not _gone(pid):
            if time.monotonic() >= deadline:
                break
            time.sleep(config.poll_interval)
    try:
        asyncio.run(_stop_left(config))
    finally:
        with contextlib.suppress(FileNotFoundError):
            marker.unlink()
    return 0


async def _stop_left(config: RunnerConfig) -> None:
    """Stop the turns of the records left, as in 18.6, and finish their runs ``cancelled``."""
    from claude_code_runner.app import Runner  # noqa: PLC0415 - the runner imports this module

    turns = config.state_dir / "turns"
    if not turns.is_dir() or not any(turns.glob("*.json")):
        return
    runner = Runner(config)
    adapter = await runner.build()
    handler = runner.handler
    handler.stopping_all = True
    try:
        await handler.adopt_turns(adapter)
        bound = 2 * (config.stop_grace + config.kill_grace + config.cli_probe_timeout)
        loop = asyncio.get_running_loop()
        deadline = loop.time() + bound
        while handler.adopted and loop.time() < deadline:
            await asyncio.sleep(config.poll_interval)
        await _drain(handler)
    finally:
        await handler.close()
        await adapter.outbox.flush()
        await adapter.inbox.close()
        await adapter.outbox.store.close()
        await adapter.client.aclose()


async def _drain(handler: Any) -> None:
    """Wait for the adopted turns' tasks to end (each finished its run)."""
    tasks = list(getattr(handler, "_adopting", ()))
    if tasks:
        await asyncio.wait(tasks, timeout=handler.config.kill_grace)
