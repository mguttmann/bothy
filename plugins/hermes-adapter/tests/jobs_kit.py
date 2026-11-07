# SPDX-License-Identifier: Apache-2.0
"""Real detached processes for the tests of registered jobs (09 17.12 point 8).

`spawn_detached` starts a job the way a supervisor script of a profile does: a launcher process
starts the supervisor in a new session (``start_new_session``), the supervisor starts a worker in
its process group, and the launcher exits at once, so the supervisor's parent is the init process.
"""

from __future__ import annotations

import contextlib
import json
import os
import signal
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path

from bothy_hermes_adapter import jobs

_SUPERVISOR = """
import subprocess, sys, time
worker = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(600)"])
open(sys.argv[1] + ".tmp", "w").write(str(worker.pid))
__import__("os").replace(sys.argv[1] + ".tmp", sys.argv[1])
worker.wait()
"""

_LAUNCHER = """
import json, os, subprocess, sys
sup = subprocess.Popen(
    [sys.executable, "-c", sys.argv[1], sys.argv[2]],
    stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    start_new_session=True,
)
print(json.dumps({"supervisor": sup.pid}))
"""


@dataclass(frozen=True)
class Detached:
    """A detached supervisor (group leader) and its worker."""

    supervisor: int
    worker: int


def spawn_detached(tmp: Path) -> Detached:
    """Start a detached supervisor with a worker; return both PIDs once both run."""
    marker = tmp / f"worker-{time.monotonic_ns()}"
    out = subprocess.run(
        [sys.executable, "-c", _LAUNCHER, _SUPERVISOR, str(marker)],
        capture_output=True,
        text=True,
        check=True,
        timeout=30,
    )
    supervisor = int(json.loads(out.stdout)["supervisor"])
    deadline = time.monotonic() + 20
    while not marker.exists():
        if time.monotonic() > deadline:
            raise AssertionError("the detached worker did not start")
        time.sleep(0.02)
    worker = int(marker.read_text())
    return Detached(supervisor, worker)


def wait_gone(pid: int, start: str | None, seconds: float = 20.0) -> bool:
    """Wait until the process with that identity is gone; returns whether it is."""
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if start is None or jobs.process_start(pid) != start:
            return True
        time.sleep(0.02)
    return False


def kill_quietly(job: Detached, starts: tuple[str | None, str | None]) -> None:
    """Clean up after a test: kill both processes if they still are the ones started."""
    for pid, start in ((job.supervisor, starts[0]), (job.worker, starts[1])):
        if start is not None and jobs.process_start(pid) == start:
            with contextlib.suppress(OSError):
                os.kill(pid, signal.SIGKILL)
