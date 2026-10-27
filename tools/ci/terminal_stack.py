# SPDX-License-Identifier: Apache-2.0
"""The client stack of the app's terminal and the tests against it.

`make terminal-stack` (brief M4-36 Design note 6) and the terminal parts of `make e2e-desktop`
and `make perf-desktop` (brief M4-38).

Starts the Go helper `services/server/test/terminal/clientstack` (the terminal stack of M2-48
with one host container) in the mode of `--mode` in the background, waits until it wrote its
connection data, runs the test of the mode against it, then tells the helper to stop and waits
until it removed the stack (containers, volumes and networks; the helper's `Down` checks that
nothing is left). The temporary folder of the connection data is removed at the end.

Modes:
  stack  `apps/client/terminal_stack_test/terminal_stack_test.dart` headless (M4-36)
  e2e    the terminal E2E of the desktop app: `apps/client/tool/e2e_suite.sh terminal` with
         TERMINAL_STACK naming the connection file (the host container starts unpaired)
  perf   the terminal targets of the performance level: `apps/client/tool/e2e_suite.sh
         terminal-perf` (a paired host with the generator profiles)
  phone  the phone run of M5-12 on the iOS simulator or Android emulator that
         TERMINAL_STACK_DEVICE names: `apps/client/tool/e2e_suite.sh terminal-phone` on the
         helper's perf world (a paired host assigned to Alex, enrollment codes from the control
         endpoint); without TERMINAL_STACK_DEVICE it prints a SKIP line and starts no stack

It needs Go and a Docker engine (the helper's preflight fails with its message on a host without
one) and the pinned Flutter SDK. Not part of `make gates`: the stack takes many minutes.

Usage: terminal_stack.py [--root <repository>] [--mode stack|e2e|perf|phone]
Exit 0 when the test passed and the stack was removed, 1 otherwise.
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

# Per mode: the mode of the Go helper (BOTHY_CLIENTSTACK_MODE) and the test command, run in
# apps/client.
MODES: dict[str, tuple[str, list[str]]] = {
    "stack": ("", ["flutter", "test", "terminal_stack_test/terminal_stack_test.dart"]),
    "e2e": ("e2e", ["tool/e2e_suite.sh", "terminal"]),
    "perf": ("perf", ["tool/e2e_suite.sh", "terminal-perf"]),
    "phone": ("perf", ["tool/e2e_suite.sh", "terminal-phone"]),
}


def main(argv: list[str] | None = None) -> int:
    """Runs the helper and the Dart test; returns the exit code."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", default=".", help="repository root")
    parser.add_argument("--mode", choices=sorted(MODES), default="stack", help="what to run")
    args = parser.parse_args(argv)
    root = Path(args.root).resolve()
    if args.mode == "phone" and not os.environ.get("TERMINAL_STACK_DEVICE"):
        print(
            "SKIP terminal-stack phone: TERMINAL_STACK_DEVICE names no iOS simulator or "
            "Android emulator"
        )
        return 0
    work = Path(tempfile.mkdtemp(prefix="terminal-stack-"))
    out = work / "connection.json"
    helper_mode, command = MODES[args.mode]
    env = {
        **os.environ,
        "BOTHY_CLIENTSTACK_OUT": str(out),
        "BOTHY_CLIENTSTACK_MODE": helper_mode,
        "TERMINAL_STACK": str(out),
    }
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
        rc = subprocess.call(command, cwd=root / "apps" / "client", env=env)
    finally:
        # The helper removes the stack on its stop file and reports a leftover as a failure.
        Path(f"{out}.stop").touch()
        if helper.wait() != 0:
            rc = 1
        shutil.rmtree(work, ignore_errors=True)
    return 0 if rc == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
