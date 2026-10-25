# SPDX-License-Identifier: Apache-2.0
"""Command line entry: ``python -m jira_app --config <file>`` (brief M2-50 Design notes 2, 9).

The process sets its umask to ``0o077``, loads the configuration, starts the app and runs until
SIGTERM or SIGINT; then it stops accepting (requests get ``503``), lets the running step finish,
closes the store, releases the lock and exits with code 0. Any start failure prints one line
naming key and file (never a value) to stderr and exits non-zero.

Examples:
    >>> parser().parse_args(["--config", "app.json"]).config
    'app.json'
"""

from __future__ import annotations

import argparse
import asyncio
import os
import signal
import sys
from collections.abc import Sequence

import httpx
from bothy_sdk.errors import ProblemError

from jira_app.app import start_app
from jira_app.config import ConfigError, load_config
from jira_app.store.base import StoreError

__all__ = ["main", "parser", "run"]


def parser() -> argparse.ArgumentParser:
    """Return the argument parser.

    Returns:
        The parser with the required option ``--config``.
    """
    p = argparse.ArgumentParser(prog="jira_app", description="Reference app for Jira.")
    p.add_argument("--config", required=True, help="JSON configuration file")
    return p


async def run(config_path: str, stop: asyncio.Event | None = None) -> None:
    """Run the app until ``stop`` is set or the process gets SIGTERM or SIGINT.

    Args:
        config_path: The configuration file.
        stop: An event that ends the run (tests); default only the signals.

    Raises:
        ConfigError: The configuration or a file failed its check.
        StoreError: The state directory is locked or the store failed.
        ProblemError: The server refused ``getMe``, ``getInstance`` or ``getWebhookKeys``.
    """
    config = load_config(config_path)
    done = stop or asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, done.set)
    try:
        app = await start_app(config)
        try:
            await done.wait()
        finally:
            await app.stop()
    finally:
        for sig in (signal.SIGTERM, signal.SIGINT):
            loop.remove_signal_handler(sig)


def main(argv: Sequence[str] | None = None) -> int:
    """Run the command line.

    Args:
        argv: The arguments without the program name (default ``sys.argv[1:]``).

    Returns:
        0 after a clean stop, 1 after a start failure.
    """
    args = parser().parse_args(argv)
    os.umask(0o077)
    try:
        asyncio.run(run(args.config))
    except (ConfigError, StoreError) as exc:
        print(f"jira_app: {exc}", file=sys.stderr)
        return 1
    except ProblemError as exc:
        print(f"jira_app: the server refused the start ({exc.status} {exc.code})", file=sys.stderr)
        return 1
    except (OSError, httpx.HTTPError) as exc:
        print(f"jira_app: cannot start ({type(exc).__name__})", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
