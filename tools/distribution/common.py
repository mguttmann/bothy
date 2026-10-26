# SPDX-License-Identifier: Apache-2.0
"""Shared parts of the desktop release steps (tools/distribution/apple.py and windows.py).

`Runner` runs the external tools (flutter, codesign, signtool, ...) and is the one seam the
tests replace; `serve` is the local update host of the trials (127.0.0.1 with a port the system
picks, brief M7-12 Boundaries); `wait_for` waits for a condition with one generous upper bound
instead of a fixed sleep; `branding_for` writes the build branding of a release from the
branding file and the distribution, so the bundle identifier comes from `distribution.yaml`.
"""

from __future__ import annotations

import contextlib
import functools
import http.server
import os
import shutil
import subprocess  # nosec B404 - runs the build and signing tools with fixed arguments
import sys
import threading
import time
from collections.abc import Callable, Iterator, Mapping, Sequence
from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[2]
CLIENT = REPO / "apps" / "client"
DEFAULT_BRANDING = CLIENT / "branding.example.yaml"
# The one upper bound of a wait for an external effect (an installer that replaces a bundle, a
# package that appears): far above what it takes on a loaded runner, so only a hang reaches it.
PATIENCE_SECONDS = 600.0


class StepError(Exception):
    """A release or trial step failed; the message names the step."""


class Runner:
    """Runs external commands; every release and trial step goes through one instance.

    Commands print to the log of the job unless `capture` is set. A failed command raises
    StepError with its command line and, when captured, its output.
    """

    def __call__(
        self,
        cmd: Sequence[str | Path],
        *,
        cwd: Path | None = None,
        env: Mapping[str, str] | None = None,
        stdin: str | None = None,
        capture: bool = False,
        check: bool = True,
        redact: Sequence[str] = (),
    ) -> subprocess.CompletedProcess[str]:
        """Run `cmd` and return its result.

        Args:
            cmd: program and arguments; the program is looked up on PATH (also `.bat` and
                `.cmd` files on Windows, which a call without a shell does not resolve).
            cwd: working directory.
            env: extra environment variables on top of this process's environment.
            stdin: text for the standard input (a key that must not appear on a command line).
            capture: return stdout and stderr instead of printing them.
            check: raise StepError when the command fails.
            redact: values (passwords) the printed command line shows as `***`.

        Raises:
            StepError: the program is missing or, with `check`, exits with a non-zero code.
        """
        args = [str(c) for c in cmd]
        # Only batch files need the full path (CreateProcess finds `.exe` itself); a resolved
        # `dart.EXE` makes the Dart hook runner call `dart.EXE.exe`.
        found = shutil.which(args[0])
        program = found if found and found.lower().endswith((".bat", ".cmd")) else args[0]
        shown = " ".join("***" if a in redact else a for a in args)
        print(f"+ {shown}", flush=True)
        try:
            done = subprocess.run(  # nosec B603
                [program, *args[1:]],
                cwd=cwd,
                env={**os.environ, **env} if env else None,
                input=stdin,
                text=True,
                capture_output=capture,
                check=False,
            )
        except FileNotFoundError as err:
            raise StepError(f"{args[0]} is not installed") from err
        if check and done.returncode != 0:
            output = f": {done.stdout}{done.stderr}".rstrip() if capture else ""
            raise StepError(f"{shown} exited with {done.returncode}{output}")
        return done


def wait_for(
    what: str,
    condition: Callable[[], bool],
    *,
    patience: float = PATIENCE_SECONDS,
    interval: float = 1.0,
    clock: Callable[[], float] = time.monotonic,
    sleep: Callable[[float], None] = time.sleep,
) -> None:
    """Return as soon as `condition()` is true.

    Raises:
        StepError: `condition` stayed false for `patience` seconds; the message names `what`.
    """
    deadline = clock() + patience
    while not condition():
        if clock() >= deadline:
            raise StepError(f"gave up waiting for {what}")
        sleep(interval)


class _QuietHandler(http.server.SimpleHTTPRequestHandler):
    """Static files of the trial's update host, with the MIME types App Installer expects."""

    extensions_map: dict[str, str] = {  # noqa: RUF012 - the base class reads it as a class field
        **http.server.SimpleHTTPRequestHandler.extensions_map,
        ".appinstaller": "application/appinstaller",
        ".msix": "application/msix",
        ".xml": "application/xml",
        ".zip": "application/zip",
    }

    def log_message(self, format: str, *args: Any) -> None:
        """Keep the job log free of one line per request."""
        return


@contextlib.contextmanager
def serve(root: Path) -> Iterator[str]:
    """Serve `root` on 127.0.0.1 with a port the system picks; yields the base URL.

    The URL ends with a slash, the form of the update addresses in distribution.yaml.
    """
    handler = functools.partial(_QuietHandler, directory=str(root))
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_address[1]}/"
    finally:
        server.shutdown()
        server.server_close()


def branding_for(branding: Path, cfg: dict[str, Any], out: Path) -> Path:
    """Write the build branding of a release to `out` and return it.

    The branding file keeps name, URL scheme and default server; the bundle identifier and the
    Android application ID come from the distribution (R-RL-05: the publisher owns the identity).

    Raises:
        StepError: the branding file is missing or no mapping.
    """
    try:
        doc = yaml.safe_load(branding.read_text(encoding="utf-8"))
    except OSError as err:
        raise StepError(f"branding file {branding}: {err.strerror}") from err
    if not isinstance(doc, dict):
        raise StepError(f"branding file {branding} is no mapping")
    doc["bundle_id"] = cfg["bundle_id"]
    doc["application_id"] = cfg["android"]["package_name"]
    out.write_text(yaml.safe_dump(doc, sort_keys=False), encoding="utf-8")
    return out


@contextlib.contextmanager
def branded_tree(run: Runner, branding: Path, release_branding: Path) -> Iterator[None]:
    """Write the native branding files of `release_branding`; restore those of `branding`.

    `tool/branding_defines.dart` writes the checked-in native files (Branding.xcconfig,
    branding.h, ...) and build/branding/dart_defines.json; the release build needs them for the
    publisher's identity, and the tree goes back to the given branding afterwards, also after a
    failure.
    """
    run(["dart", "run", "tool/branding_defines.dart", "--input", release_branding], cwd=CLIENT)
    try:
        yield
    finally:
        run(["dart", "run", "tool/branding_defines.dart", "--input", branding], cwd=CLIENT)


def require_env(names: Sequence[str], environ: Mapping[str, str], job: str) -> dict[str, str]:
    """Return the named secrets from `environ`, or stop before anything is signed.

    Raises:
        StepError: naming every missing or empty variable and the environment that holds it.
    """
    missing = [n for n in names if not environ.get(n)]
    if missing:
        raise StepError(
            f"{job} needs the secrets {', '.join(missing)} of the environment {job}; nothing "
            "was signed (docs/operations/release.md 4)"
        )
    return {n: environ[n] for n in names}


def fail(message: str) -> int:
    """Print `message` as the reason of a failed step and return exit code 1."""
    print(f"error: {message}", file=sys.stderr, flush=True)
    return 1
