# SPDX-License-Identifier: Apache-2.0
"""Command line: `python -m cost_reconciler --config <file> [--once]`.

With `--once` the app runs one pass and exits with `0` when every submission succeeded, `1` when
anything failed and `2` for a configuration error (also a secret file with an open mode).
Without it the app runs passes until SIGTERM or SIGINT, which end it after the current request,
and waits `reconciler.min_request_interval` between passes; it then exits with `0`, or with `2`
when the configuration file is wrong at the start. In that mode a secret that cannot be read
fails only the pass, so a rotation in progress does not stop the service.

Every request to the cost API, also across passes, starts at least
`reconciler.min_request_interval` after the previous one. A schedule that starts the app with
`--once` must not start it more often than that interval.
"""

from __future__ import annotations

import argparse
import os
import signal
import sys
import threading
import time
import urllib.request
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import Protocol, TextIO

from cost_reconciler.config import ConfigError, load_config
from cost_reconciler.costapi import CostApiClient
from cost_reconciler.logs import EventLog
from cost_reconciler.reconcile import PassContext, PassResult, run_pass
from cost_reconciler.server import ServerClient
from cost_reconciler.transport import Pacer, build_opener
from cost_reconciler.uuid7 import new_uuid7

EXIT_OK = 0
EXIT_FAILED = 1
EXIT_CONFIG = 2


class StopSignal(Protocol):
    """A stop request, set by the signal handlers (a `threading.Event` in production)."""

    def is_set(self) -> bool:
        """Report whether a stop was requested."""
        ...

    def wait(self, timeout: float) -> bool:
        """Wait up to `timeout` seconds or until the stop; return whether it is set."""
        ...


def _utc_now() -> datetime:
    return datetime.now(UTC)


@dataclass(frozen=True)
class Runtime:
    """The process's clocks, streams and environment; tests replace them.

    Attributes:
        wall_now: The wall clock with a time zone (window, log time stamps).
        monotonic: The monotonic clock of the request interval.
        sleep: Waits between requests; returns early on a stop in production.
        stop: The stop request.
        stderr: The stream of the log lines.
        environ: The environment for `env:` references.
        opener: The HTTP opener without redirects.
    """

    wall_now: Callable[[], datetime]
    monotonic: Callable[[], float]
    sleep: Callable[[float], object]
    stop: StopSignal
    stderr: TextIO
    environ: Mapping[str, str]
    opener: urllib.request.OpenerDirector = field(default_factory=build_opener)


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="python -m cost_reconciler",
        description="Reconcile estimated run costs with the provider's daily cost report.",
    )
    parser.add_argument("--config", required=True, type=Path, help="the TOML configuration file")
    parser.add_argument("--once", action="store_true", help="run one pass and exit")
    return parser


def run(argv: Sequence[str], runtime: Runtime) -> int:
    """Run the app with arguments and a runtime.

    Args:
        argv: The arguments without the program name.
        runtime: Clocks, streams and environment.

    Returns:
        The exit code.
    """
    args = _parser().parse_args(argv)
    log = EventLog(runtime.stderr, runtime.wall_now)
    try:
        config = load_config(args.config)
    except ConfigError as err:
        log.emit("config_error", level="error", key=err.key, rule=err.rule)
        return EXIT_CONFIG
    timeout_s = config.http_timeout_ms / 1000
    ctx = PassContext(
        config=config,
        cost_api=CostApiClient(
            base_url=config.cost_api_url,
            timeout_s=timeout_s,
            pacer=Pacer(
                config.min_request_interval_ms / 1000,
                runtime.monotonic,
                runtime.sleep,
                runtime.stop.is_set,
            ),
            opener=runtime.opener,
        ),
        server=ServerClient(
            base_url=config.server_url,
            timeout_s=timeout_s,
            opener=runtime.opener,
            new_key=new_uuid7,
        ),
        log=log,
        now=runtime.wall_now,
        stopped=runtime.stop.is_set,
        environ=runtime.environ,
    )
    if args.once:
        result = run_pass(ctx)
        return {PassResult.OK: EXIT_OK, PassResult.CONFIG_ERROR: EXIT_CONFIG}.get(
            result, EXIT_FAILED
        )
    interval_s = config.min_request_interval_ms / 1000
    while not runtime.stop.is_set():
        run_pass(ctx)
        if runtime.stop.wait(interval_s):
            break
    return EXIT_OK


def main(argv: Sequence[str] | None = None) -> int:
    """Run the app as a process: real clocks, stderr and SIGTERM and SIGINT as stop.

    Args:
        argv: The arguments without the program name; `sys.argv[1:]` when absent.

    Returns:
        The exit code.
    """
    stop = threading.Event()

    def request_stop(signum: int, frame: object) -> None:
        del signum, frame
        stop.set()

    signal.signal(signal.SIGTERM, request_stop)
    signal.signal(signal.SIGINT, request_stop)
    runtime = Runtime(
        wall_now=_utc_now,
        monotonic=time.monotonic,
        sleep=stop.wait,
        stop=stop,
        stderr=sys.stderr,
        environ=os.environ,
    )
    return run(sys.argv[1:] if argv is None else argv, runtime)


if __name__ == "__main__":
    sys.exit(main())
