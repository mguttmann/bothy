# SPDX-License-Identifier: Apache-2.0
"""Runner of the mobile device E2E test, ``make e2e-mobile`` (brief M5-11 Design note 3).

It checks which platforms can run on this host and prints a SKIP line for each that cannot (a
missing tool), fails when none can, starts the devices (an iOS simulator it creates and an
Android emulator from an AVD, or the ones named by the inputs), picks a free loopback port for
the capture endpoint of the fake push providers, and runs ``infra/compose/e2e.sh`` with the
mobile overlay ``infra/compose/test/compose.e2e-mobile.yaml``. e2e.sh starts the Compose stack,
waits until it is ready and runs this script again with ``--inner`` as its command, which runs
per platform the push relay (``push_relay.py``) and the patrol suites
(``apps/client/tool/mobile_e2e_suite.sh``). e2e.sh removes the stack and its temporary data
after the run, also after a failure. Then the terminal suite of brief M5-13 runs against the
terminal stack of the Go helper ``services/server/test/terminal/clientstack`` in the mode
``mobile`` (the stack of M2-48 with a real host service in a container, paired through the API
and assigned to Alex): the runner starts the helper, waits for its connection file, runs
``mobile_e2e_suite.sh <platform> terminal`` per platform and then tells the helper to stop and
waits until it removed the stack, the host container among it, also after a failing suite. This
script stops the relays, the emulator it started (by serial and PID) and shuts down and deletes
the simulator it created (by UDID), also after a failure. The logs of each suite and relay go
to ``MOBILE_E2E_LOG_DIR`` (default ``build/e2e-mobile``); the suites and the relay print no
message content and no token.

Inputs (environment):
    MOBILE_E2E_PLATFORMS       platforms to run, default ``ios android``
    MOBILE_E2E_UDID            an existing simulator to use (kept as it is), else one is created
    MOBILE_E2E_SERIAL          an emulator or device already attached to adb (kept running)
    MOBILE_E2E_AVD             the AVD to start, default the first of ``emulator -list-avds``
    MOBILE_E2E_BOOT_TIMEOUT_S  bound of the emulator boot, default 900
    MOBILE_E2E_LOG_DIR         directory of the logs, default build/e2e-mobile
    MOBILE_E2E_SUITES          suites to run (login push share offline terminal), default all
    The runner exports MOBILE_E2E_FAKEPUSH_PORT (the capture endpoint on 127.0.0.1) and
    MOBILE_E2E_PATROL_PORTS (the test and app server ports of patrol) for the inner run, and
    TERMINAL_STACK (the helper's connection file) and MOBILE_E2E_TERMINAL_PORTS (the TLS front
    and the control endpoint on 127.0.0.1) for the terminal suite.
    E2E_SERVER_IMAGE and the other inputs of infra/compose/e2e.sh pass through.

Exit codes: 0 every platform that could run passed, 1 a failure or no platform could run, 2 a
missing precondition of the stack (from e2e.sh, or Go for the terminal stack) or of the inner
run.
"""

from __future__ import annotations

import json
import os
import platform as pyplatform
import secrets
import shutil
import socket
import subprocess
import sys
import tempfile
import time
from collections.abc import Callable, Iterable, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import IO, Protocol

REPO = Path(__file__).resolve().parents[2]
E2E_SH = REPO / "infra" / "compose" / "e2e.sh"
OVERLAY = REPO / "infra" / "compose" / "test" / "compose.e2e-mobile.yaml"
SUITE = REPO / "apps" / "client" / "tool" / "mobile_e2e_suite.sh"
RELAY = Path(__file__).resolve().parent / "push_relay.py"
PLATFORMS = ("ios", "android")
# The suites of M5-11 against the stack of e2e.sh, and the terminal suite of M5-13 against the
# terminal stack of the Go helper.
WORLD_SUITES = ("login", "push", "share", "offline")
TERMINAL_SUITE = "terminal"
SERVER = REPO / "services" / "server"
# The helper of the terminal stack and how often the runner looks for its connection file.
HELPER = [
    "go",
    "-C",
    str(SERVER),
    "test",
    "-run",
    "^TestClientStack$",
    "-count=1",
    "-timeout",
    "180m",
    "-v",
    "./test/terminal/clientstack/",
]
HELPER_POLL_S = 2.0
# The services of the mobile overlay that e2e.sh builds (E2E_BUILD): the E2E server image and the
# fake push providers.
BUILD = "api fakepush"
# Ports the host rules reserve for other services; the kernel's ephemeral range does not
# contain them on the supported hosts, the check keeps it so on any host.
RESERVED_PORTS = frozenset(
    {3000, 3978, 5000, 5432, 5433, 7000, 8080, 8081, 8798, 8799, 9119, 9177, 11434, 20241}
    | set(range(8642, 8653))
)
# The console ports of the Android emulator are even numbers in this range (adb scans it).
CONSOLE_PORTS = range(5554, 5684, 2)
DEFAULT_BOOT_TIMEOUT_S = 900


class Proc(Protocol):
    """A background process the runner started."""

    pid: int

    def poll(self) -> int | None:
        """The exit code, or None while it runs."""

    def terminate(self) -> None:
        """Asks the process to stop (SIGTERM)."""

    def kill(self) -> None:
        """Stops the process (SIGKILL)."""

    def wait(self, timeout: float | None = None) -> int:
        """Waits for the end; raises subprocess.TimeoutExpired after timeout."""


class Host(Protocol):
    """What the runner needs of the host; tests give it a fake."""

    system: str
    env: dict[str, str]

    def which(self, name: str) -> str | None:
        """The path of a tool on PATH, or None."""

    def run(
        self, cmd: Sequence[str], env: dict[str, str] | None = None, log: Path | None = None
    ) -> tuple[int, str]:
        """Runs cmd to its end and returns its exit code and output.

        With log the output goes to the console and to that file and is not returned.
        """

    def spawn(
        self, cmd: Sequence[str], env: dict[str, str] | None = None, log: Path | None = None
    ) -> Proc:
        """Starts cmd in the background, its output to log (or discarded)."""

    def now(self) -> float:
        """Monotonic seconds."""

    def sleep(self, seconds: float) -> None:
        """Waits."""

    def port_free(self, port: int) -> bool:
        """Whether 127.0.0.1:port can be bound now."""


@dataclass
class RealHost:
    """The host this process runs on."""

    system: str = field(default_factory=pyplatform.system)
    env: dict[str, str] = field(default_factory=lambda: dict(os.environ))

    def which(self, name: str) -> str | None:
        """See Host.which; the Android SDK tools are also found under ANDROID_HOME."""
        found = shutil.which(name, path=self.env.get("PATH"))
        if found:
            return found
        sdk = self.env.get("ANDROID_HOME") or self.env.get("ANDROID_SDK_ROOT")
        for sub in ("platform-tools", "emulator"):
            if sdk and (Path(sdk) / sub / name).is_file():
                return str(Path(sdk) / sub / name)
        return None

    def _argv(self, cmd: Sequence[str]) -> list[str]:
        resolved = self.which(cmd[0]) if "/" not in cmd[0] else cmd[0]
        return [resolved or cmd[0], *cmd[1:]]

    def run(
        self, cmd: Sequence[str], env: dict[str, str] | None = None, log: Path | None = None
    ) -> tuple[int, str]:
        """See Host.run."""
        argv = self._argv(cmd)
        if log is None:
            done = subprocess.run(
                argv, env=env or self.env, capture_output=True, text=True, check=False
            )
            return done.returncode, done.stdout
        with log.open("a", encoding="utf-8", buffering=1) as f:
            proc = subprocess.Popen(
                argv,
                env=env or self.env,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
            )
            out: IO[str] | None = proc.stdout
            for line in out or ():
                sys.stdout.write(line)
                sys.stdout.flush()
                f.write(line)
            return proc.wait(), ""

    def spawn(
        self, cmd: Sequence[str], env: dict[str, str] | None = None, log: Path | None = None
    ) -> Proc:
        """See Host.spawn."""
        sink = log.open("a", encoding="utf-8", buffering=1) if log else subprocess.DEVNULL
        try:
            return subprocess.Popen(
                self._argv(cmd),
                env=env or self.env,
                stdout=sink,
                stderr=subprocess.STDOUT,
                start_new_session=True,
            )
        finally:
            if log and not isinstance(sink, int):
                sink.close()

    def now(self) -> float:
        """See Host.now."""
        return time.monotonic()

    def sleep(self, seconds: float) -> None:
        """See Host.sleep."""
        time.sleep(seconds)

    def port_free(self, port: int) -> bool:
        """See Host.port_free."""
        with socket.socket() as s:
            try:
                s.bind(("127.0.0.1", port))
            except OSError:
                return False
        return True


def _kernel_port() -> int:
    """A port of 127.0.0.1 the kernel hands out for port 0 (its ephemeral range)."""
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        port: int = s.getsockname()[1]
        return port


def free_port(
    taken: Iterable[int] = (), bind: Callable[[], int] = _kernel_port, tries: int = 32
) -> int:
    """Returns a free loopback port that is neither in taken nor reserved by the host rules.

    Raises:
        RuntimeError: no such port after tries attempts.
    """
    avoid = set(taken) | RESERVED_PORTS
    for _ in range(tries):
        port = bind()
        if port not in avoid:
            return port
    raise RuntimeError("could not find a free port on 127.0.0.1")


def free_console_port(is_free: Callable[[int], bool]) -> int:
    """Returns the first even console port of the emulator whose port and port + 1 are free.

    Raises:
        RuntimeError: every console port is taken.
    """
    for port in CONSOLE_PORTS:
        if is_free(port) and is_free(port + 1):
            return port
    raise RuntimeError("no free emulator console port in 5554 to 5682")


def platforms_from(value: str) -> list[str]:
    """Parses MOBILE_E2E_PLATFORMS (space or comma separated).

    Raises:
        ValueError: an unknown platform.
    """
    names = [n for n in value.replace(",", " ").split() if n]
    for n in names:
        if n not in PLATFORMS:
            raise ValueError(f"unknown platform {n!r} in MOBILE_E2E_PLATFORMS (ios, android)")
    return names or list(PLATFORMS)


def _skip(platform: str, why: str) -> str:
    return f"SKIP e2e-mobile {platform}: {why}"


def suites_from(value: str) -> tuple[list[str], bool]:
    """Parses MOBILE_E2E_SUITES: the suites of M5-11 to run and whether the terminal suite runs.

    Empty means every suite.

    Raises:
        ValueError: an unknown suite.
    """
    names = [n for n in value.replace(",", " ").split() if n]
    for n in names:
        if n not in (*WORLD_SUITES, TERMINAL_SUITE):
            raise ValueError(
                f"unknown suite {n!r} in MOBILE_E2E_SUITES ({' '.join(WORLD_SUITES)} terminal)"
            )
    if not names:
        return list(WORLD_SUITES), True
    return [n for n in names if n != TERMINAL_SUITE], TERMINAL_SUITE in names


def preflight(platforms: Sequence[str], host: Host) -> tuple[list[str], list[str]]:
    """Returns the platforms that can run and a SKIP line for each that cannot.

    For Android without MOBILE_E2E_SERIAL it also sets MOBILE_E2E_AVD in host.env to the first
    AVD when none is named.
    """
    ready: list[str] = []
    skips: list[str] = []
    for p in platforms:
        if host.which("flutter") is None:
            skips.append(_skip(p, "flutter missing (install the Flutter SDK of pubspec.yaml)"))
            continue
        if p == "ios":
            if host.system != "Darwin":
                skips.append(_skip(p, "needs macOS with Xcode"))
            elif host.which("xcrun") is None or host.run(["xcrun", "--find", "simctl"])[0] != 0:
                skips.append(
                    _skip(p, "xcrun simctl missing (install Xcode with the iOS simulator)")
                )
            else:
                ready.append(p)
            continue
        if host.which("adb") is None:
            skips.append(
                _skip(p, "adb missing (install the Android SDK platform-tools, set ANDROID_HOME)")
            )
            continue
        if host.env.get("MOBILE_E2E_SERIAL"):
            ready.append(p)
            continue
        if host.which("emulator") is None:
            skips.append(
                _skip(
                    p, "emulator missing (install the Android emulator, or set MOBILE_E2E_SERIAL)"
                )
            )
            continue
        if not host.env.get("MOBILE_E2E_AVD"):
            avds = [a for a in host.run(["emulator", "-list-avds"])[1].splitlines() if a.strip()]
            if not avds:
                skips.append(
                    _skip(
                        p,
                        "no AVD (create one with avdmanager, or set MOBILE_E2E_AVD or"
                        " MOBILE_E2E_SERIAL)",
                    )
                )
                continue
            host.env["MOBILE_E2E_AVD"] = avds[0].strip()
        ready.append(p)
    return ready, skips


class DeviceError(Exception):
    """A device did not come up."""


def _boot_timeout(host: Host) -> int:
    raw = host.env.get("MOBILE_E2E_BOOT_TIMEOUT_S", str(DEFAULT_BOOT_TIMEOUT_S))
    if not raw.isdigit() or int(raw) <= 0:
        raise DeviceError("MOBILE_E2E_BOOT_TIMEOUT_S must be a whole number of seconds above 0")
    return int(raw)


def start_android(host: Host, cleanups: list[Callable[[], None]]) -> str:
    """Returns the adb serial of the emulator of the run, starting it from the AVD if needed.

    A started emulator gets a cleanup that kills it by serial and then by PID.

    Raises:
        DeviceError: the emulator did not finish booting within MOBILE_E2E_BOOT_TIMEOUT_S.
    """
    given = host.env.get("MOBILE_E2E_SERIAL", "")
    if given:
        return given
    port = free_console_port(host.port_free)
    serial = f"emulator-{port}"
    avd = host.env["MOBILE_E2E_AVD"]
    print(f"e2e-mobile: starting the emulator {avd} as {serial}", flush=True)
    proc = host.spawn(
        [
            "emulator",
            "-avd",
            avd,
            "-port",
            str(port),
            "-no-window",
            "-no-audio",
            "-no-boot-anim",
            "-no-snapshot-save",
        ]
    )

    def stop() -> None:
        host.run(["adb", "-s", serial, "emu", "kill"])
        try:
            proc.wait(timeout=120)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=30)

    cleanups.append(stop)
    deadline = host.now() + _boot_timeout(host)
    while True:
        rc, out = host.run(["adb", "-s", serial, "shell", "getprop", "sys.boot_completed"])
        if rc == 0 and out.strip() == "1":
            return serial
        if proc.poll() is not None:
            raise DeviceError(f"the emulator {avd} ended before it booted")
        if host.now() >= deadline:
            raise DeviceError(f"the emulator {avd} did not boot within MOBILE_E2E_BOOT_TIMEOUT_S")
        host.sleep(2)


def _iphone_runtime(listing: str) -> tuple[str, str]:
    """The newest available iOS runtime and its first iPhone device type from simctl's JSON.

    Raises:
        DeviceError: no iOS runtime with an iPhone.
    """
    best: tuple[tuple[int, ...], str, str] | None = None
    for rt in json.loads(listing).get("runtimes", []):
        if rt.get("platform") != "iOS" or not rt.get("isAvailable"):
            continue
        phones = [
            d["identifier"]
            for d in rt.get("supportedDeviceTypes", [])
            if d.get("productFamily") == "iPhone"
        ]
        if not phones:
            continue
        version = tuple(int(x) for x in str(rt.get("version", "0")).split(".") if x.isdigit())
        if best is None or version > best[0]:
            best = (version, rt["identifier"], phones[0])
    if best is None:
        raise DeviceError("no available iOS runtime with an iPhone (xcrun simctl list runtimes)")
    return best[1], best[2]


def start_ios(host: Host, cleanups: list[Callable[[], None]]) -> str:
    """Returns the UDID of the simulator of the run, creating and booting one if none is given.

    A created simulator gets a cleanup that shuts it down and deletes it by UDID; a given one is
    booted when needed and shut down again only then.

    Raises:
        DeviceError: simctl failed.
    """
    given = host.env.get("MOBILE_E2E_UDID", "")
    if given:
        rc, out = host.run(["xcrun", "simctl", "list", "devices"])
        if rc == 0 and any(given in line and "(Booted)" in line for line in out.splitlines()):
            return given
        if host.run(["xcrun", "simctl", "boot", given])[0] != 0:
            raise DeviceError(f"cannot boot the simulator {given}")
        cleanups.append(lambda: _quiet(host, ["xcrun", "simctl", "shutdown", given]))
    else:
        rc, out = host.run(["xcrun", "simctl", "list", "runtimes", "-j"])
        if rc != 0:
            raise DeviceError("xcrun simctl list runtimes failed")
        runtime, devicetype = _iphone_runtime(out)
        name = f"bothy-e2e-{secrets.token_hex(4)}"
        rc, out = host.run(["xcrun", "simctl", "create", name, devicetype, runtime])
        udid = out.strip()
        if rc != 0 or not udid:
            raise DeviceError("xcrun simctl create failed")
        given = udid
        print(f"e2e-mobile: created the simulator {name} ({udid})", flush=True)

        def remove() -> None:
            _quiet(host, ["xcrun", "simctl", "shutdown", udid])
            _quiet(host, ["xcrun", "simctl", "delete", udid])

        cleanups.append(remove)
        if host.run(["xcrun", "simctl", "boot", udid])[0] != 0:
            raise DeviceError(f"cannot boot the simulator {udid}")
    if host.run(["xcrun", "simctl", "bootstatus", given, "-b"])[0] != 0:
        raise DeviceError(f"the simulator {given} did not finish booting")
    return given


def _quiet(host: Host, cmd: Sequence[str]) -> None:
    """Runs a cleanup command; a failure is reported, never raised."""
    rc, _ = host.run(cmd)
    if rc != 0:
        print(f"e2e-mobile: cleanup step failed ({' '.join(cmd[:3])}, exit {rc})", flush=True)


def _log_dir(host: Host) -> Path:
    d = Path(host.env.get("MOBILE_E2E_LOG_DIR") or REPO / "build" / "e2e-mobile")
    d.mkdir(parents=True, exist_ok=True)
    return d


def run_outer(platforms: Sequence[str], host: Host, bind: Callable[[], int] = _kernel_port) -> int:
    """The whole run: preflight, devices, the stack with the inner run, teardown.

    Returns the exit code of the run.
    """
    ready, skips = preflight(platforms, host)
    for line in skips:
        print(line, flush=True)
    if not ready:
        print("e2e-mobile: no platform could run", flush=True)
        return 1
    try:
        world, terminal = suites_from(host.env.get("MOBILE_E2E_SUITES", ""))
    except ValueError as err:
        print(f"e2e-mobile: {err}", flush=True)
        return 2
    if terminal and host.which("go") is None:
        print(
            "e2e-mobile: go is not on PATH; the terminal suite needs the Go toolchain of"
            " go.work for the helper of its stack (make bootstrap)",
            flush=True,
        )
        return 2
    cleanups: list[Callable[[], None]] = []
    try:
        env = dict(host.env)
        # patrol, the suite and the relay call adb by name; the SDK's copy may be off PATH.
        adb = host.which("adb") if "android" in ready else None
        if adb and "/" in adb:
            env["PATH"] = os.pathsep.join([str(Path(adb).parent), env.get("PATH", "")])
        if "android" in ready:
            env["MOBILE_E2E_SERIAL"] = start_android(host, cleanups)
        if "ios" in ready:
            env["MOBILE_E2E_UDID"] = start_ios(host, cleanups)
        # The capture endpoint of fakepush and the two ports patrol binds on the host (its
        # defaults are reserved by the host rules).
        capture = free_port(bind=bind)
        test_server = free_port(taken={capture}, bind=bind)
        app_server = free_port(taken={capture, test_server}, bind=bind)
        env.update(
            E2E_OVERLAY=str(OVERLAY),
            E2E_BUILD=BUILD,
            MOBILE_E2E_READY=" ".join(ready),
            MOBILE_E2E_FAKEPUSH_PORT=str(capture),
            MOBILE_E2E_PATROL_PORTS=f"{test_server} {app_server}",
            MOBILE_E2E_LOG_DIR=str(_log_dir(host)),
            MOBILE_E2E_SUITES=" ".join(world),
        )
        rc = 0
        if world:
            # The stack's own output (the steps of e2e.sh and the inner run) goes to the console
            # and to stack.log.
            rc, _ = host.run(
                [str(E2E_SH), sys.executable, str(Path(__file__).resolve()), "--inner"],
                env=env,
                log=Path(env["MOBILE_E2E_LOG_DIR"]) / "stack.log",
            )
        if terminal:
            # One stack at a time: the terminal stack starts after e2e.sh removed its own.
            rc_terminal = run_terminal(ready, env, host)
            rc = rc or rc_terminal
        return rc
    except (DeviceError, RuntimeError) as err:
        print(f"e2e-mobile: {err}", flush=True)
        return 1
    finally:
        for stop in reversed(cleanups):
            try:
                stop()
            except Exception as err:
                print(f"e2e-mobile: cleanup failed: {type(err).__name__}", flush=True)


def _connection_ports(connection: Path) -> tuple[int, int]:
    """The ports of the TLS front and of the control endpoint in the helper's connection file.

    Raises:
        ValueError: the file names no such ports.
    """
    data = json.loads(connection.read_text(encoding="utf-8"))
    front = data.get("published_port")
    control = str(data.get("control_url", "")).rsplit(":", 1)[-1]
    if not isinstance(front, int) or not control.isdigit():
        raise ValueError("the connection file names no front port or control endpoint")
    return front, int(control)


def run_terminal(ready: Sequence[str], env: dict[str, str], host: Host) -> int:
    """The terminal suite of brief M5-13 on every ready platform against one terminal stack.

    Starts the helper of the terminal stack in the mode mobile, waits for its connection file,
    runs ``mobile_e2e_suite.sh <platform> terminal`` per platform, then always writes the stop
    file and waits until the helper removed the stack (containers, volumes and networks, the
    host container among them; the helper reports a leftover as its failure), and removes the
    temporary folder of the connection file. Returns 0 when every suite passed and the helper
    ended cleanly, else 1.
    """
    logs = Path(env["MOBILE_E2E_LOG_DIR"])
    work = Path(tempfile.mkdtemp(prefix="mobile-e2e-terminal-"))
    out = work / "connection.json"
    helper = host.spawn(
        HELPER,
        env={**env, "BOTHY_CLIENTSTACK_OUT": str(out), "BOTHY_CLIENTSTACK_MODE": "mobile"},
        log=logs / "terminal-stack.log",
    )
    results: dict[str, int] = {}
    rc = 0
    ready_stack = False
    try:
        while not out.exists():
            if helper.poll() is not None:
                print(
                    "e2e-mobile: the terminal stack ended before it was ready"
                    f" (see {logs / 'terminal-stack.log'})",
                    flush=True,
                )
                return 1
            host.sleep(HELPER_POLL_S)
        ready_stack = True
        front, control = _connection_ports(out)
        for p in ready:
            results[p], _ = host.run(
                [str(SUITE), p, TERMINAL_SUITE],
                env={
                    **env,
                    "TERMINAL_STACK": str(out),
                    "MOBILE_E2E_TERMINAL_PORTS": f"{front} {control}",
                },
                log=logs / f"{p}-terminal.log",
            )
    except ValueError as err:
        print(f"e2e-mobile: {err}", flush=True)
        rc = 1
    finally:
        # The helper removes the stack on its stop file, also after a failing suite.
        (work / "connection.json.stop").touch()
        if helper.wait() != 0:
            if ready_stack:
                print("e2e-mobile: the terminal stack was not removed cleanly", flush=True)
            rc = 1
        shutil.rmtree(work, ignore_errors=True)
    for p, code in results.items():
        print(f"e2e-mobile {p} terminal: {'PASS' if code == 0 else 'FAIL'}", flush=True)
    if any(code != 0 for code in results.values()):
        rc = 1
    return rc


def run_inner(host: Host) -> int:
    """The part inside e2e.sh: per platform the relay and the suites; returns the exit code."""
    ready = platforms_from(host.env.get("MOBILE_E2E_READY", ""))
    port = host.env.get("MOBILE_E2E_FAKEPUSH_PORT", "")
    device = {
        "ios": host.env.get("MOBILE_E2E_UDID", ""),
        "android": host.env.get("MOBILE_E2E_SERIAL", ""),
    }
    if not port.isdigit() or any(not device[p] for p in ready):
        print(
            "e2e-mobile: run through make e2e-mobile (tools/e2e/mobile_e2e.py),"
            " never --inner alone",
            flush=True,
        )
        return 2
    logs = _log_dir(host)
    # MOBILE_E2E_SUITES picks suites of the script (login push share offline); empty runs all.
    suites = host.env.get("MOBILE_E2E_SUITES", "").split()
    results: dict[str, int] = {}
    for p in ready:
        flag = "--udid" if p == "ios" else "--serial"
        relay = host.spawn(
            [
                sys.executable,
                str(RELAY),
                "--capture-url",
                f"http://127.0.0.1:{port}",
                "--platform",
                p,
                flag,
                device[p],
            ],
            log=logs / f"{p}-relay.log",
        )
        try:
            results[p], _ = host.run(
                [str(SUITE), p, *suites], env=dict(host.env), log=logs / f"{p}-suite.log"
            )
        finally:
            relay.terminate()
            try:
                relay.wait(timeout=30)
            except subprocess.TimeoutExpired:
                relay.kill()
    for p, rc in results.items():
        print(f"e2e-mobile {p}: {'PASS' if rc == 0 else 'FAIL'}", flush=True)
    return 0 if all(rc == 0 for rc in results.values()) else 1


def main(argv: Sequence[str] | None = None) -> int:
    """Entry point: ``mobile_e2e.py`` for the whole run, ``--inner`` inside e2e.sh."""
    args = list(sys.argv[1:] if argv is None else argv)
    host = RealHost()
    if args == ["--inner"]:
        return run_inner(host)
    if args:
        print(
            "usage: mobile_e2e.py (inputs in the environment, see the module docstring)",
            file=sys.stderr,
        )
        return 2
    try:
        platforms = platforms_from(host.env.get("MOBILE_E2E_PLATFORMS", ""))
    except ValueError as err:
        print(f"e2e-mobile: {err}", file=sys.stderr)
        return 2
    return run_outer(platforms, host)


if __name__ == "__main__":
    sys.exit(main())
