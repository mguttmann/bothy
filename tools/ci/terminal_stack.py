# SPDX-License-Identifier: Apache-2.0
"""The stack test of the app's terminal (brief M4-36 Design note 6; `make terminal-stack`).

Starts the Go helper `services/server/test/terminal/clientstack` (the terminal stack of M2-48
with one host container, a world with Alex and enrollment codes) in the background, waits until
it wrote its connection data, runs `apps/client/terminal_stack_test/terminal_stack_test.dart`
against it, then tells the helper to stop and waits until it removed the stack (containers,
volumes and networks; the helper's `Down` checks that nothing is left). The temporary folder
of the connection data is removed at the end.

It needs Go and a Docker engine (the helper's preflight fails with its message on a host without
one) and the pinned Flutter SDK. Not part of `make gates`: the stack takes many minutes.

Usage: terminal_stack.py [--root <repository>]
Exit 0 when the Dart test passed and the stack was removed, 1 otherwise.
"""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

# How often the script looks for the connection file while the helper starts the stack.
POLL_SECONDS = 2.0


def main(argv: list[str] | None = None) -> int:
    """Runs the helper and the Dart test; returns the exit code."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", default=".", help="repository root")
    args = parser.parse_args(argv)
    root = Path(args.root).resolve()
    work = Path(tempfile.mkdtemp(prefix="terminal-stack-"))
    out = work / "connection.json"
    env = {**os.environ, "BOTHY_CLIENTSTACK_OUT": str(out)}
    helper = subprocess.Popen(
        [
            "go",
            "test",
            "-run",
            "^TestClientStack$",
            "-count=1",
            "-timeout",
            "180m",
            "-v",
            "./test/terminal/clientstack/",
        ],
        cwd=root / "services" / "server",
        env=env,
    )
    rc = 1
    try:
        while not out.exists():
            if helper.poll() is not None:
                print(
                    "terminal-stack: the helper ended before the stack was ready",
                    file=sys.stderr,
                )
                return 1
            time.sleep(POLL_SECONDS)
        rc = subprocess.call(
            ["flutter", "test", "terminal_stack_test/terminal_stack_test.dart"],
            cwd=root / "apps" / "client",
            env=env,
        )
    finally:
        # The helper removes the stack on its stop file and reports a leftover as a failure.
        Path(f"{out}.stop").touch()
        if helper.wait() != 0:
            rc = 1
        shutil.rmtree(work, ignore_errors=True)
    return 0 if rc == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
