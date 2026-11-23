# SPDX-License-Identifier: Apache-2.0
"""Command line of the runner: ``run``, ``check-config`` and ``stop --turns``, each ``--config``.

``run`` starts the runner and runs until the SDK stops (for example ``4015`` when another
process took the consumer over) or SIGINT, SIGTERM or SIGHUP arrives (SIGHUP like SIGTERM; a failed
stop is logged as ``stop_failed`` and exits 1). With ``runner.turn_handover`` SIGTERM
hands the live turns over and the runner exits 75 when it handed at least one over (09 18.19.3).
``check-config`` loads the file, resolves every set ``secret_ref`` without printing it, probes
``claude --version`` and prints one code per check; it contacts no server. A set
``runner.tested_claude_code_version`` adds the line ``ok``, or ``pin_changed`` (a failure) when
it was proved against another pin than the code's. ``stop --turns`` stops
every turn as in 18.6, finishes each run ``cancelled`` with ``adapter_stopped`` and exits 0
(`claude_code_runner.handover.stop_turns`). Logs are JSON lines on stderr with ``ts``, ``level``
and ``logger`` (`claude_code_runner.logformat`).

Examples:
    >>> main(["--help"])  # doctest: +SKIP
"""

from __future__ import annotations

import argparse
import asyncio
import contextlib
import json
import logging
import os
import signal
import sys
from collections.abc import Awaitable, Callable, Sequence
from pathlib import Path

from claude_code_runner.app import Runner, RunnerHandler, StartError
from claude_code_runner.config import ConfigError, RunnerConfig, load_config
from claude_code_runner.handover import HANDOVER_EXIT, stop_turns
from claude_code_runner.heartbeat import extra_tested_version
from claude_code_runner.logformat import configure as configure_logging
from claude_code_runner.secrets import SecretError, resolve

__all__ = ["check_config", "main"]


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="claude-code-runner")
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("run", "check-config"):
        cmd = sub.add_parser(name)
        cmd.add_argument("--config", required=True, type=Path)
    stop = sub.add_parser("stop")
    stop.add_argument("--turns", required=True, action="store_true")
    stop.add_argument("--config", required=True, type=Path)
    return parser


def check_config(config: RunnerConfig) -> list[str]:
    """Run the offline checks of ``check-config``.

    Args:
        config: The loaded configuration (loading already checked keys, types and paths).

    Returns:
        One line ``<check> <code>`` per check; a code other than ``ok`` is a failure.
    """
    lines = ["config ok"]
    refs = {
        "runner.agent_token_ref": config.agent_token_ref,
        "runner.dpop_key_ref": config.dpop_key_ref,
        "runner.anthropic_api_key_ref": config.anthropic_api_key_ref,
    }
    for key, ref in refs.items():
        if ref is None:
            lines.append(f"{key} unset")
            continue
        try:
            resolve(ref)
            lines.append(f"{key} ok")
        except SecretError as err:
            lines.append(f"{key} {err.code}")
    if config.tested_claude_code_version is not None:
        # The entry is proof only for the pin it was proved against (FIX-RUNNER-TESTED-CLI).
        valid = extra_tested_version(
            config.tested_claude_code_version, config.tested_claude_code_pin
        )
        lines.append(f"runner.tested_claude_code_version {'ok' if valid else 'pin_changed'}")
    handler = RunnerHandler(config, os.environ)
    probe = asyncio.run(handler.refresh_probe())
    lines.append(f"runtime {','.join(probe.reasons) or 'ok'}")
    return lines


STOP_SIGNALS = (signal.SIGINT, signal.SIGTERM, signal.SIGHUP)
"""The signals that stop ``run`` (SIGHUP like SIGTERM)."""


class StopOnSignal:
    """Start ``stop`` once for the first stop signal, hold its task and log a failure.

    Args:
        stop: The stop of the runner (`Runner.stop`).
        on_failure: Called once the stop raised (ends ``run``, so the process exits 1 instead of
            waiting for the SIGKILL of its service manager).
    """

    def __init__(
        self, stop: Callable[[], Awaitable[None]], on_failure: Callable[[], object]
    ) -> None:
        self._stop = stop
        self._on_failure = on_failure
        self.task: asyncio.Task[None] | None = None
        self.failed = False

    def __call__(self) -> None:
        """The signal handler: a further signal while the stop runs changes nothing."""
        if self.task is not None:
            return
        self.task = asyncio.ensure_future(self._stop())
        self.task.add_done_callback(self._done)

    def _done(self, task: asyncio.Task[None]) -> None:
        if task.cancelled() or task.exception() is None:
            return
        self.failed = True
        err = task.exception()
        log = logging.getLogger("claude_code_runner")
        # The message of an error may hold a path or a value: the error line names only its
        # kind, the traceback goes to level debug, which the command line does not write.
        log.error(json.dumps({"event": "stop_failed", "error": type(err).__name__}))
        log.debug("stop failed", exc_info=err)
        self._on_failure()


async def _serve(config: RunnerConfig) -> int:
    runner = Runner(config)
    loop = asyncio.get_running_loop()
    run = asyncio.ensure_future(runner.run())
    on_signal = StopOnSignal(runner.stop, run.cancel)
    for sig in STOP_SIGNALS:
        with contextlib.suppress(NotImplementedError):
            loop.add_signal_handler(sig, on_signal)
    try:
        reason = await run
    except asyncio.CancelledError:
        if on_signal.failed:
            return 1
        raise
    except StartError as err:
        logging.getLogger("claude_code_runner").error(
            '{"event": "start_failed", "code": "%s"}', err
        )
        return 1
    if on_signal.task is not None:
        with contextlib.suppress(Exception):
            await on_signal.task
        if on_signal.failed:
            return 1
    if runner.handed_over:
        return HANDOVER_EXIT  # a service manager restarts the runner, which adopts the turns
    return 0 if reason.code in (1000, 1001) else 1


def main(argv: Sequence[str] | None = None) -> int:
    """Entry point of the console script ``claude-code-runner``.

    Args:
        argv: The arguments (default ``sys.argv[1:]``).

    Returns:
        The exit code: 0 on success, 1 on a failed check, start or stop, 2 on a configuration error,
        75 after a SIGTERM that handed at least one turn over (09 18.19.3).
    """
    args = _parser().parse_args(argv)
    # JSON lines with time and level; no request lines of httpx, no frame dumps of websockets
    # (they can hold the API token).
    configure_logging(sys.stderr)
    try:
        config = load_config(args.config)
    except ConfigError as err:
        print(str(err), file=sys.stderr)
        return 2
    if args.command == "check-config":
        lines = check_config(config)
        print("\n".join(lines))
        return 0 if all(line.endswith((" ok", " unset")) for line in lines) else 1
    if args.command == "stop":
        try:
            return stop_turns(config)
        except StartError as err:
            logging.getLogger("claude_code_runner").error(
                '{"event": "stop_turns_failed", "code": "%s"}', err
            )
            return 1
    return asyncio.run(_serve(config))


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
