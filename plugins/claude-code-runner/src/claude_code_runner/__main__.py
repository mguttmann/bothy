# SPDX-License-Identifier: Apache-2.0
"""Command line of the runner: ``run``, ``check-config`` and ``stop --turns``, each ``--config``.

``run`` starts the runner and runs until the SDK stops (for example ``4015`` when another
process took the consumer over) or SIGINT/SIGTERM arrives. With ``runner.turn_handover`` SIGTERM
hands the live turns over and the runner exits 75 when it handed at least one over (09 18.19.3).
``check-config`` loads the file, resolves every set ``secret_ref`` without printing it, probes
``claude --version`` and prints one code per check; it contacts no server. ``stop --turns`` stops
every turn as in 18.6, finishes each run ``cancelled`` with ``adapter_stopped`` and exits 0
(`claude_code_runner.handover.stop_turns`). Logs are JSON lines on stderr.

Examples:
    >>> main(["--help"])  # doctest: +SKIP
"""

from __future__ import annotations

import argparse
import asyncio
import contextlib
import logging
import os
import signal
import sys
from collections.abc import Sequence
from pathlib import Path

from claude_code_runner.app import Runner, RunnerHandler, StartError
from claude_code_runner.config import ConfigError, RunnerConfig, load_config
from claude_code_runner.handover import HANDOVER_EXIT, stop_turns
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
    handler = RunnerHandler(config, os.environ)
    probe = asyncio.run(handler.refresh_probe())
    lines.append(f"runtime {','.join(probe.reasons) or 'ok'}")
    return lines


async def _serve(config: RunnerConfig) -> int:
    runner = Runner(config)
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        with contextlib.suppress(NotImplementedError):
            loop.add_signal_handler(sig, lambda: asyncio.ensure_future(runner.stop()))
    try:
        reason = await runner.run()
    except StartError as err:
        logging.getLogger("claude_code_runner").error(
            '{"event": "start_failed", "code": "%s"}', err
        )
        return 1
    if runner.handed_over:
        return HANDOVER_EXIT  # a service manager restarts the runner, which adopts the turns
    return 0 if reason.code in (1000, 1001) else 1


def main(argv: Sequence[str] | None = None) -> int:
    """Entry point of the console script ``claude-code-runner``.

    Args:
        argv: The arguments (default ``sys.argv[1:]``).

    Returns:
        The exit code: 0 on success, 1 on a failed check or start, 2 on a configuration error,
        75 after a SIGTERM that handed at least one turn over (09 18.19.3).
    """
    args = _parser().parse_args(argv)
    logging.basicConfig(stream=sys.stderr, level=logging.INFO, format="%(message)s")
    # Frame dumps of the WebSocket library can hold the API token (identify); never log them.
    logging.getLogger("websockets").setLevel(logging.WARNING)
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
