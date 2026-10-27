# SPDX-License-Identifier: Apache-2.0
"""Capture of the showcase from the demo instance (brief M7-28; `make showcase PLATFORM=<p>`).

One run starts the demo instance of M7-27 (`infra/compose/demo.sh`, seed `--seed`), waits until
it prints that it is ready, captures every platform of `--platform` from it in turn, strips the
metadata of every file (media.py), files it under `docs/assets/showcase/<platform>/` with its
manifest entry (manifest.py: script, commit and demo seed), and stops the demo, which removes its
stack. Nothing is edited by hand and nothing of the build host appears: the apps are captured by
Flutter's image API inside the app (apps/client/e2e_test/showcase), the console and Grafana by the
page screenshot of a headless browser, the video by the screen recorder of the Android emulator.

Platforms (`--platform`, comma separated; the build host runs the demo next to each):

- `android`: the app on an Android emulator through `flutter drive` with
  `apps/client/e2e_test/showcase/showcase_test.dart`, both themes, and the video of one flow
  (`adb shell screenrecord`). Inputs: `SHOWCASE_ANDROID_SERIAL` (a running emulator) or
  `SHOWCASE_AVD` (an AVD to start; default the first of `emulator -list-avds`).
- `ios`, `macos`, `windows`: the same test on a simulator or desktop that runs next to the demo
  (`SHOWCASE_DEVICE` names the Flutter device). A host without one of them leaves the platform
  out (brief Boundaries); the manifest then has no entry for it.
- `console`: the web console in headless Chromium (Playwright of the tools environment), logged
  in as Alex with the password and TOTP that `bothy-smoke seed` sets, both colour schemes.
- `grafana`: the dashboard of the panel plugin of M7-25 in a local Grafana with Prometheus,
  fed with the demo's metrics (grafana.py).

The first platform uses the enrollment code the demo printed; every further platform gets a
fresh code of Alex from `bothy setup issue-enrollment-code` in the demo's `api` container,
because a code is valid once.

The run refuses a working tree with changes outside `docs/assets/showcase` (the manifest names
the commit, so the images must come from it); `--allow-dirty` is for developing a driver only.

Exit codes: 0 every platform captured and filed; 1 a capture or a step failed (nothing of that
platform is filed); 2 a precondition is missing (named with its fix).
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import re
import shlex
import shutil
import signal
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request
from collections.abc import Callable, Iterator, Sequence
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import manifest
import media

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = "tools/showcase/capture.py"
DEMO_SH = Path("infra/compose/demo.sh")
CLIENT = Path("apps/client")
SHOWCASE_TEST = "e2e_test/showcase/showcase_test.dart"
SHOWCASE_DRIVER = "test_driver/showcase_driver.dart"
THEMES = ("light", "dark")
APP_PLATFORMS = ("android", "ios", "macos", "windows")
PLATFORMS = (*APP_PLATFORMS, "console", "grafana")
# The address under which the Android emulator reaches the loopback of its host.
EMULATOR_HOST_ALIAS = "10.0.2.2"
# Width in pixels of the images per platform: phones in portrait, desktop and browser wide.
CAPTURE_WIDTH = {"android": 720, "ios": 720, "macos": 1600, "windows": 1600}
# The bound of one wait of the run (demo start, a capture); a hang becomes a failure. The demo
# warm-up alone posts its prompts minutes apart (infra/demo/demo.yaml values.pace).
DEMO_READY_TIMEOUT_S = 45 * 60
STEP_TIMEOUT_S = 30 * 60
# The screen recording of the video: small enough for repo.max_file_kb (1024 KiB), readable.
VIDEO_SIZE = "360x800"
VIDEO_BIT_RATE = 150_000
MAX_FILE_BYTES = 1024 * 1024
# Markers of the showcase test in the output of flutter drive (showcase_test.dart).
VIDEO_READY = "showcase: video ready "
VIDEO_DONE = "showcase: video done"

# The lines infra/compose/demo.sh prints once the demo is ready (its `say` calls; the test
# test_capture.py holds them equal to the script).
_READY_ADDRESS = re.compile(r"^demo:   address\s+(?P<url>https://\S+)$")
_READY_CA = re.compile(r"^demo:   CA certificate (?P<path>\S+) \(")
_READY_CODE = re.compile(r"^demo:   sign in as Alex with the enrollment code: (?P<code>\S+)$")
# The cache folder of the Android app in the path the video marker names.
_ANDROID_PACKAGE = re.compile(r"^/data/(?:user/\d+|data)/(?P<pkg>[A-Za-z0-9_.]+)/")


class PreconditionError(Exception):
    """A missing precondition; the message names the fix (exit code 2)."""


class StepError(Exception):
    """A failed step of the run (exit code 1)."""


def say(message: str) -> None:
    """Print one progress line on stderr."""
    print(f"showcase: {message}", file=sys.stderr, flush=True)


@dataclass(frozen=True)
class Demo:
    """The running demo as demo.sh printed it.

    Attributes:
        url: the address, `https://localhost:<port>`.
        ca: the PEM file of the demo CA.
        code: the enrollment code of Alex that demo.sh printed (valid once).
        seed: the seed of the warm-up.
    """

    url: str
    ca: Path
    code: str
    seed: int

    @property
    def port(self) -> int:
        """The published port of the demo's TLS front."""
        return int(self.url.rsplit(":", 1)[1])


def parse_ready(lines: Sequence[str], seed: int) -> Demo | None:
    """Return the demo once demo.sh printed its three ready lines, else None.

    Args:
        lines: the lines demo.sh wrote to stderr so far.
        seed: the seed the run started the demo with.

    Returns:
        The demo, or None while a line is still missing.
    """
    url = ca = code = None
    for line in lines:
        if m := _READY_ADDRESS.match(line):
            url = m["url"]
        elif m := _READY_CA.match(line):
            ca = Path(m["path"])
        elif m := _READY_CODE.match(line):
            code = m["code"]
    if url is None or ca is None or code is None:
        return None
    return Demo(url=url, ca=ca, code=code, seed=seed)


def android_package(path: str) -> str:
    """Return the application ID in the path of the app's cache folder.

    Raises:
        StepError: the path is not inside an app's data folder.

    Example: `android_package("/data/user/0/com.example.chat/cache/showcase-go")` is
    `"com.example.chat"`.
    """
    m = _ANDROID_PACKAGE.match(path)
    if m is None:
        raise StepError(f"the video marker names no app data folder: {path!r}")
    return m["pkg"]


def asset_name(platform: str, view: str, theme: str, ext: str = "png") -> str:
    """Return the manifest path of one file, `<platform>/<view>-<theme>.<ext>`."""
    return f"{platform}/{view}-{theme}.{ext}"


def split_image_name(stem: str) -> tuple[str, str]:
    """Split `<view>-<theme>` into view and theme.

    Raises:
        StepError: the stem does not end in a theme.
    """
    view, _, theme = stem.rpartition("-")
    if not view or theme not in THEMES:
        raise StepError(f"image {stem!r} is not named <view>-<light|dark>")
    return view, theme


def file_assets(
    root: Path,
    platform: str,
    files: Sequence[tuple[Path, str, str, str]],
    commit: str,
    seed: int,
    script: str = SCRIPT,
) -> list[str]:
    """Strip, file and record the captured files of one platform, replacing its old ones.

    Args:
        root: the repository root.
        platform: the platform; its folder and entries are replaced as a whole.
        files: (captured file, kind, view, theme) per file; kind `image` (PNG) or `video` (MP4).
        commit: the commit the capture ran on.
        seed: the demo seed.
        script: the capture script the manifest names.

    Returns:
        The filed paths relative to `docs/assets/showcase`.

    Raises:
        StepError: a file cannot be stripped, is larger than repo.max_file_kb after the
            strip, or still carries metadata; nothing of the platform is filed then.
    """
    stripped: list[tuple[bytes, str, str, str]] = []
    for path, kind, view, theme in files:
        try:
            data = media.strip(path.read_bytes())
        except media.MediaError as exc:
            raise StepError(f"{path.name}: {exc}") from exc
        left = media.metadata(data)
        if left:
            raise StepError(f"{path.name}: metadata left after the strip: {', '.join(left)}")
        if len(data) > MAX_FILE_BYTES:
            raise StepError(
                f"{path.name}: {len(data)} bytes after the strip, more than repo.max_file_kb "
                f"({MAX_FILE_BYTES // 1024} KiB); never raise the limit, make the capture smaller"
            )
        stripped.append((data, kind, view, theme))
    manifest.drop_platform(root, platform)
    filed = []
    for data, kind, view, theme in stripped:
        name = asset_name(platform, view, theme, manifest.KINDS[kind])
        target = root / manifest.ASSET_DIR / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        manifest.record(
            root,
            manifest.Asset(
                file=name,
                kind=kind,
                platform=platform,
                view=view,
                theme=theme,
                script=script,
                commit=commit,
                demo_seed=seed,
            ),
        )
        filed.append(name)
    return filed


def dirty_paths(root: Path) -> list[str]:
    """Return the changed or untracked paths outside the showcase folder."""
    out = subprocess.run(
        ["git", "-C", str(root), "status", "--porcelain", "--untracked-files=all"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    asset_dir = manifest.ASSET_DIR.as_posix() + "/"
    paths = [line[3:] for line in out.splitlines() if line.strip()]
    return [p for p in paths if not p.startswith(asset_dir)]


def head_commit(root: Path) -> str:
    """Return the commit of HEAD."""
    return subprocess.run(
        ["git", "-C", str(root), "rev-parse", "HEAD"], check=True, capture_output=True, text=True
    ).stdout.strip()


def free_port() -> int:
    """Return a free TCP port of 127.0.0.1 that the kernel hands out for port 0."""
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


def tls_context(ca: Path) -> ssl.SSLContext:
    """Return a TLS context that trusts only the demo CA."""
    context = ssl.create_default_context(cafile=str(ca))
    return context


def instance_id(demo: Demo) -> str:
    """Read the instance ID from the discovery document of the demo."""
    url = demo.url + "/api/v1/instance/discovery"
    with urllib.request.urlopen(url, context=tls_context(demo.ca), timeout=60) as r:  # noqa: S310 # nosec B310 - https to the local demo only
        doc = json.load(r)
    value = doc.get("instance_id")
    if not isinstance(value, str) or not value:
        raise StepError("the discovery document of the demo names no instance_id")
    return value


def until(what: str, condition: Callable[[], bool], timeout_s: float, poll_s: float = 1.0) -> None:
    """Poll `condition` until it holds; raise StepError naming `what` after `timeout_s`."""
    deadline = time.monotonic() + timeout_s
    while not condition():
        if time.monotonic() >= deadline:
            raise StepError(f"timed out after {timeout_s:.0f} s waiting for {what}")
        time.sleep(poll_s)


class DemoProcess:
    """`infra/compose/demo.sh` in the background: started, waited for, stopped.

    demo.sh removes its stack, volumes, images and temp folder itself when it gets SIGTERM
    (its trap); the run only starts it and stops it.
    """

    def __init__(self, root: Path, seed: int, log: Path) -> None:
        """Prepare the process; [start] runs it.

        Args:
            root: the repository root.
            seed: the demo seed.
            log: the file that receives demo.sh's output.
        """
        self.root = root
        self.seed = seed
        self.log = log
        self.proc: subprocess.Popen[str] | None = None
        self.lines: list[str] = []
        self._reader: threading.Thread | None = None

    def start(self) -> None:
        """Start demo.sh with the seed; its output goes to the log and to [lines]."""
        env = dict(os.environ, DEMO_SEED=str(self.seed))
        env.pop("DEMO_KEEP", None)
        self.proc = subprocess.Popen(
            [str(self.root / DEMO_SH)],
            cwd=self.root,
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            text=True,
            start_new_session=True,
        )
        self._reader = threading.Thread(target=self._read, daemon=True)
        self._reader.start()

    def _read(self) -> None:
        if self.proc is None or self.proc.stderr is None:
            return
        with self.log.open("a", encoding="utf-8") as log:
            for line in self.proc.stderr:
                # The code of Alex is in one line; the log keeps everything else of it.
                masked = re.sub(r"(enrollment code: )\S+", r"\1<code>", line)
                log.write(masked)
                log.flush()
                self.lines.append(line.rstrip("\n"))

    def wait_ready(self, timeout_s: float = DEMO_READY_TIMEOUT_S) -> Demo:
        """Wait until demo.sh printed that the demo is ready; return the demo."""
        demo: Demo | None = None

        def ready() -> bool:
            nonlocal demo
            demo = parse_ready(list(self.lines), self.seed)
            if demo is None and self.proc is not None and self.proc.poll() is not None:
                raise StepError(f"demo.sh ended with {self.proc.returncode}; see {self.log}")
            return demo is not None

        until("the demo to be ready", ready, timeout_s, poll_s=2.0)
        if demo is None:
            raise StepError("the demo printed no ready lines")
        return demo

    def stop(self) -> None:
        """Stop demo.sh with SIGTERM and wait until it removed the demo."""
        if self.proc is None or self.proc.poll() is not None:
            return
        # SIGTERM, not SIGINT: a shell started without job control ignores SIGINT, and an
        # ignored signal cannot be trapped; demo.sh traps both the same way.
        os.kill(self.proc.pid, signal.SIGTERM)
        self.proc.wait(timeout=STEP_TIMEOUT_S)
        if self._reader is not None:
            self._reader.join(timeout=60)
        if self.proc.returncode != 0:
            raise StepError(f"demo.sh did not remove the demo cleanly ({self.proc.returncode})")


def docker(
    *args: str, check: bool = True, capture: bool = True
) -> subprocess.CompletedProcess[str]:
    """Run a docker command."""
    return subprocess.run(["docker", *args], check=check, capture_output=capture, text=True)


def demo_container(demo: Demo, service: str) -> str:
    """Return the ID of the container of `service` in the demo's Compose project.

    The project is the one whose TLS front publishes the demo's port.
    """
    front = docker("ps", "--filter", f"publish={demo.port}", "--format", "{{.ID}}").stdout.split()
    if len(front) != 1:
        raise StepError(f"expected one container publishing port {demo.port}, found {len(front)}")
    project = docker(
        "inspect", "--format", '{{index .Config.Labels "com.docker.compose.project"}}', front[0]
    ).stdout.strip()
    ids = docker(
        "ps",
        "--filter",
        f"label=com.docker.compose.project={project}",
        "--filter",
        f"label=com.docker.compose.service={service}",
        "--format",
        "{{.ID}}",
    ).stdout.split()
    if len(ids) != 1:
        raise StepError(f"expected one {service} container in {project}, found {len(ids)}")
    return ids[0]


def fresh_code(demo: Demo) -> str:
    """Issue a fresh enrollment code of Alex in the demo's api container (valid once)."""
    out = docker(
        "exec",
        demo_container(demo, "api"),
        "bothy",
        "setup",
        "issue-enrollment-code",
        "--user",
        "alex",
    ).stdout
    out = str(out).strip()
    if not out or "\n" in out:
        raise StepError("bothy setup issue-enrollment-code printed no single code")
    return out


class Codes:
    """The enrollment codes of the run: the demo's first, then a fresh one per platform."""

    def __init__(self, demo: Demo) -> None:
        """Start with the code demo.sh printed."""
        self.demo = demo
        self._first: str | None = demo.code

    def next(self) -> str:
        """Return a code nobody used yet."""
        if self._first is not None:
            code, self._first = self._first, None
            return code
        return fresh_code(self.demo)


# ---- the apps --------------------------------------------------------------------------------


def showcase_define(demo: Demo, instance: str, code: str, platform: str, video: bool) -> str:
    """Return the base64 JSON of the define SHOWCASE of showcase_test.dart."""
    data = {
        "server": demo.url,
        "instance_id": instance,
        "ca_pem": demo.ca.read_text(encoding="utf-8"),
        "code": code,
        "platform": platform,
        "upstream_host": EMULATOR_HOST_ALIAS if platform == "android" else None,
        "themes": list(THEMES),
        "capture_width": CAPTURE_WIDTH[platform],
        "video": video,
    }
    return base64.b64encode(json.dumps(data).encode("utf-8")).decode("ascii")


@dataclass
class Emulator:
    """The Android emulator of the run: the serial and whether the run started it."""

    serial: str
    started: subprocess.Popen[bytes] | None = None

    def adb(self, *args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
        """Run adb against this emulator."""
        return subprocess.run(
            ["adb", "-s", self.serial, *args], check=check, capture_output=True, text=True
        )

    def stop(self) -> None:
        """Stop the emulator if the run started it."""
        if self.started is None:
            return
        self.adb("emu", "kill", check=False)
        try:
            self.started.wait(timeout=120)
        except subprocess.TimeoutExpired:
            self.started.terminate()
            self.started.wait(timeout=60)


def start_emulator(work: Path) -> Emulator:
    """Return the emulator of SHOWCASE_ANDROID_SERIAL, or start the AVD of SHOWCASE_AVD."""
    serial = os.environ.get("SHOWCASE_ANDROID_SERIAL")
    if serial:
        return Emulator(serial)
    if shutil.which("emulator") is None or shutil.which("adb") is None:
        raise PreconditionError(
            "android: emulator and adb are not on PATH; add the Android SDK's emulator and "
            "platform-tools, or set SHOWCASE_ANDROID_SERIAL"
        )
    avd = os.environ.get("SHOWCASE_AVD")
    if not avd:
        avds = subprocess.run(
            ["emulator", "-list-avds"], check=True, capture_output=True, text=True
        ).stdout.split()
        if not avds:
            raise PreconditionError(
                "android: no AVD; create one with avdmanager or set SHOWCASE_AVD"
            )
        avd = avds[0]
    # The console port of the emulator: an even port whose next port is free too.
    port = 0
    for _ in range(50):
        candidate = free_port()
        if candidate % 2 == 0 and candidate < 65535:
            port = candidate
            break
    if not port:
        raise StepError("android: no free even port for the emulator")
    log = (work / "emulator.log").open("wb")
    proc = subprocess.Popen(
        [
            "emulator",
            "-avd",
            avd,
            "-port",
            str(port),
            "-no-window",
            "-no-audio",
            "-no-snapshot",
            "-no-boot-anim",
        ],
        stdout=log,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    emu = Emulator(f"emulator-{port}", proc)

    def booted() -> bool:
        if proc.poll() is not None:
            raise StepError(f"android: the emulator ended with {proc.returncode}")
        r = emu.adb("shell", "getprop", "sys.boot_completed", check=False)
        return str(r.stdout).strip() == "1"

    until("the emulator to boot", booted, STEP_TIMEOUT_S, poll_s=2.0)
    return emu


def demo_mode(emu: Emulator, on: bool) -> None:
    """Switch the system UI demo mode: a fixed clock, full battery and signal, no icons.

    The status bar then shows nothing of the moment or of the build host in the video.
    """
    if not on:
        emu.adb(
            "shell",
            "am",
            "broadcast",
            "-a",
            "com.android.systemui.demo",
            "-e",
            "command",
            "exit",
            check=False,
        )
        return
    emu.adb("shell", "settings", "put", "global", "sysui_demo_allowed", "1")
    for extra in (
        ["-e", "command", "enter"],
        ["-e", "command", "clock", "-e", "hhmm", "1200"],
        ["-e", "command", "battery", "-e", "level", "100", "-e", "plugged", "false"],
        ["-e", "command", "network", "-e", "wifi", "show", "-e", "level", "4"],
        ["-e", "command", "notifications", "-e", "visible", "false"],
    ):
        emu.adb("shell", "am", "broadcast", "-a", "com.android.systemui.demo", *extra)


class Recorder:
    """`adb shell screenrecord` of the video flow, started and stopped by the test's markers."""

    REMOTE = "/sdcard/showcase-flow.mp4"

    def __init__(self, emu: Emulator, out: Path) -> None:
        """Prepare the recording into `out`."""
        self.emu = emu
        self.out = out
        self.proc: subprocess.Popen[bytes] | None = None

    def on_line(self, line: str) -> None:
        """React to a line of flutter drive: start on the ready marker, stop on the done one."""
        if VIDEO_READY in line:
            go = line.split(VIDEO_READY, 1)[1].strip()
            self.proc = subprocess.Popen(
                [
                    "adb",
                    "-s",
                    self.emu.serial,
                    "shell",
                    "screenrecord",
                    "--size",
                    VIDEO_SIZE,
                    "--bit-rate",
                    str(VIDEO_BIT_RATE),
                    self.REMOTE,
                ],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            # The recorder writes once it runs; then the flow may start.
            until(
                "the screen recorder",
                lambda: self.emu.adb("shell", "ls", self.REMOTE, check=False).returncode == 0,
                STEP_TIMEOUT_S,
                poll_s=0.2,
            )
            self.emu.adb("shell", "run-as", android_package(go), "touch", go)
        elif VIDEO_DONE in line and self.proc is not None:
            self.emu.adb("shell", "pkill", "-INT", "screenrecord", check=False)
            self.proc.wait(timeout=120)
            self.emu.adb("pull", self.REMOTE, str(self.out))
            self.emu.adb("shell", "rm", "-f", self.REMOTE, check=False)
            self.proc = None


@dataclass(frozen=True)
class Remote:
    """A host of the run that builds an Apple platform next to the demo (brief Design note 2).

    The demo runs on the build host with Docker; a Mac with Xcode reaches it through an SSH
    reverse tunnel of the demo's port, so the app there talks to `https://localhost:<port>` as
    on the build host. The run ships the tracked files of the working tree to `directory`,
    runs `flutter drive` there and copies the images back.

    Attributes:
        host: the SSH destination (`SHOWCASE_REMOTE_SSH`).
        directory: the build folder on the host (`SHOWCASE_REMOTE_DIR`), replaced by every run.
        flutter: the Flutter command on the host (`SHOWCASE_REMOTE_FLUTTER`, default `flutter`).
    """

    host: str
    directory: str
    flutter: str = "flutter"

    @classmethod
    def from_environment(cls) -> Remote | None:
        """Return the remote of the environment, or None to build on the build host."""
        host = os.environ.get("SHOWCASE_REMOTE_SSH")
        if not host:
            return None
        directory = os.environ.get("SHOWCASE_REMOTE_DIR", "")
        if not directory.startswith("/") or directory.rstrip("/").count("/") < 2:
            raise PreconditionError(
                "SHOWCASE_REMOTE_DIR must be an absolute build folder below a home or work "
                "folder; the run replaces it"
            )
        return cls(
            host, directory.rstrip("/"), os.environ.get("SHOWCASE_REMOTE_FLUTTER", "flutter")
        )

    def ssh(self, command: str, **kwargs: Any) -> subprocess.CompletedProcess[bytes]:
        """Run `command` on the host."""
        return subprocess.run(["ssh", self.host, command], check=True, **kwargs)

    def ship(self, root: Path) -> None:
        """Replace the build folder with the tracked and new files of the working tree."""
        files = subprocess.run(
            [
                "git",
                "-C",
                str(root),
                "ls-files",
                "-z",
                "--cached",
                "--others",
                "--exclude-standard",
            ],
            check=True,
            capture_output=True,
        ).stdout
        tar = subprocess.Popen(
            ["tar", "-C", str(root), "--null", "-T", "-", "-cf", "-"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
        )
        assert_pipes(tar)
        d = shlex.quote(self.directory)
        ssh = subprocess.Popen(
            ["ssh", self.host, f"rm -rf {d} && mkdir -p {d} && tar -C {d} -xf -"],
            stdin=tar.stdout,
        )
        tar.stdin.write(files)  # type: ignore[union-attr]
        tar.stdin.close()  # type: ignore[union-attr]
        if ssh.wait() != 0 or tar.wait() != 0:
            raise StepError(f"shipping the source to {self.host} failed")

    @contextmanager
    def tunnel(self, port: int) -> Iterator[None]:
        """Forward `localhost:<port>` of the host to the demo on the build host."""
        proc = subprocess.Popen(
            [
                "ssh",
                "-N",
                "-o",
                "ExitOnForwardFailure=yes",
                "-R",
                f"127.0.0.1:{port}:127.0.0.1:{port}",
                self.host,
            ]
        )
        try:
            until(
                "the reverse tunnel",
                lambda: (
                    self.ssh(f"nc -z 127.0.0.1 {port}", check=False, capture_output=True).returncode
                    == 0
                    or proc.poll() is not None
                ),
                STEP_TIMEOUT_S,
                poll_s=1.0,
            )
            if proc.poll() is not None:
                raise StepError(f"the reverse tunnel to {self.host} ended ({proc.returncode})")
            yield
        finally:
            proc.terminate()
            proc.wait(timeout=60)

    def fetch(self, remote_out: str, out: Path) -> None:
        """Copy the images of `remote_out` on the host into `out`."""
        data = self.ssh(f"tar -C {shlex.quote(remote_out)} -cf - .", capture_output=True).stdout
        subprocess.run(["tar", "-C", str(out), "-xf", "-"], input=data, check=True)


def assert_pipes(proc: subprocess.Popen[bytes]) -> None:
    """Raise StepError when a pipe of `proc` is missing."""
    if proc.stdin is None or proc.stdout is None:
        raise StepError("a pipe of a child process is missing")


def flutter_drive(
    root: Path,
    device: str,
    define: str,
    out: Path,
    on_line: Callable[[str], None] = lambda _line: None,
    remote: Remote | None = None,
) -> None:
    """Run the showcase test on `device`; the driver writes the images into `out`.

    With `remote` the test runs on that host (the source shipped there first) and the images
    are copied back.
    """
    args = [
        "drive",
        "-d",
        device,
        f"--driver={SHOWCASE_DRIVER}",
        f"--target={SHOWCASE_TEST}",
        f"--dart-define=SHOWCASE={define}",
    ]
    if remote is None:
        command = ["flutter", *args]
        cwd: Path | None = root / CLIENT
        env = dict(os.environ, SHOWCASE_OUT=str(out))
    else:
        remote_out = f"{remote.directory}/showcase-out"
        client = shlex.quote(f"{remote.directory}/{CLIENT.as_posix()}")
        command = [
            "ssh",
            remote.host,
            f"mkdir -p {shlex.quote(remote_out)} && cd {client} && "
            f"SHOWCASE_OUT={shlex.quote(remote_out)} {shlex.join([remote.flutter, *args])}",
        ]
        cwd, env = None, dict(os.environ)
    proc = subprocess.Popen(
        command, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True
    )
    if proc.stdout is None:
        raise StepError("flutter drive has no output pipe")
    for raw in proc.stdout:
        line = raw.rstrip("\n")
        print(f"  {line}", file=sys.stderr, flush=True)
        on_line(line)
    if proc.wait() != 0:
        raise StepError(f"flutter drive on {device} failed ({proc.returncode})")
    if remote is not None:
        remote.fetch(f"{remote.directory}/showcase-out", out)


def captured_images(out: Path) -> list[tuple[Path, str, str, str]]:
    """Return (file, kind, view, theme) of every PNG the driver wrote, sorted."""
    files = []
    for path in sorted(out.glob("*.png")):
        view, theme = split_image_name(path.stem)
        files.append((path, "image", view, theme))
    if not files:
        raise StepError(f"the showcase test wrote no image into {out}")
    return files


def capture_app(
    root: Path, platform: str, demo: Demo, code: str, work: Path
) -> list[tuple[Path, str, str, str]]:
    """Capture the app on `platform`; returns the captured files."""
    out = work / platform
    out.mkdir()
    instance = instance_id(demo)
    if platform == "android":
        emu = start_emulator(work)
        try:
            demo_mode(emu, True)
            video = work / "flow.mp4"
            recorder = Recorder(emu, video)
            flutter_drive(
                root,
                emu.serial,
                showcase_define(demo, instance, code, platform, video=True),
                out,
                recorder.on_line,
            )
            if not video.is_file():
                raise StepError("android: the video flow left no recording")
            # The flow ends in the last theme of the images.
            return [*captured_images(out), (video, "video", "flow", THEMES[-1])]
        finally:
            demo_mode(emu, False)
            emu.stop()
    device = os.environ.get("SHOWCASE_DEVICE", platform)
    define = showcase_define(demo, instance, code, platform, False)
    remote = Remote.from_environment()
    if remote is None:
        flutter_drive(root, device, define, out)
    else:
        remote.ship(root)
        with remote.tunnel(demo.port):
            flutter_drive(root, device, define, out, remote=remote)
    return captured_images(out)


# ---- the run ---------------------------------------------------------------------------------


def preflight(root: Path, platforms: Sequence[str], allow_dirty: bool) -> None:
    """Check the preconditions of the run; raise PreconditionError naming the fix."""
    unknown = [p for p in platforms if p not in PLATFORMS]
    if unknown or not platforms:
        raise PreconditionError(f"PLATFORM must name one or more of {', '.join(PLATFORMS)}")
    if not allow_dirty:
        dirty = dirty_paths(root)
        if dirty:
            raise PreconditionError(
                "the working tree has changes outside docs/assets/showcase; commit them first, "
                f"because the manifest names the commit ({', '.join(dirty[:5])})"
            )
    if shutil.which("docker") is None:
        raise PreconditionError("docker is not on PATH; the demo needs Docker (make demo)")
    if any(p in APP_PLATFORMS for p in platforms) and shutil.which("flutter") is None:
        raise PreconditionError("flutter is not on PATH; the app captures need the pinned Flutter")


@contextmanager
def workspace() -> Iterator[Path]:
    """A temp folder of the run (mktemp -d), removed at the end unless SHOWCASE_KEEP_WORK=1."""
    path = Path(tempfile.mkdtemp(prefix="bothy-showcase."))
    try:
        yield path
    finally:
        if os.environ.get("SHOWCASE_KEEP_WORK") == "1":
            say(f"SHOWCASE_KEEP_WORK=1: the temp folder {path} is kept; remove it yourself")
        else:
            shutil.rmtree(path, ignore_errors=True)


def capture(root: Path, platforms: Sequence[str], seed: int) -> list[str]:
    """Start the demo, capture every platform and file the results; returns the filed paths."""
    import console_capture  # noqa: PLC0415 - Playwright only when a browser platform runs
    import grafana_capture  # noqa: PLC0415

    commit = head_commit(root)
    filed: list[str] = []
    with workspace() as work:
        demo_proc = DemoProcess(root, seed, work / "demo.log")
        say(f"starting the demo (seed {seed}); the warm-up takes a few minutes")
        demo_proc.start()
        try:
            demo = demo_proc.wait_ready()
            say(f"demo ready at {demo.url}")
            codes = Codes(demo)
            for platform in platforms:
                say(f"capturing {platform}")
                if platform in APP_PLATFORMS:
                    files = capture_app(root, platform, demo, codes.next(), work)
                elif platform == "console":
                    files = console_capture.capture(root, demo, codes.next(), work)
                else:
                    files = grafana_capture.capture(root, demo, work)
                filed += file_assets(root, platform, files, commit, seed)
                say(f"{platform}: filed {len(files)} file(s)")
        finally:
            say("stopping the demo")
            demo_proc.stop()
    return filed


def parse_args(argv: Sequence[str]) -> argparse.Namespace:
    """Parse the command line."""
    parser = argparse.ArgumentParser(description="Capture the showcase from the demo instance.")
    parser.add_argument(
        "--platform", required=True, help="comma separated: " + ", ".join(PLATFORMS)
    )
    parser.add_argument("--seed", type=int, default=1, help="seed of the demo (default 1)")
    parser.add_argument("--root", type=Path, default=REPO_ROOT)
    parser.add_argument(
        "--allow-dirty",
        action="store_true",
        help="capture from a working tree with changes (developing a driver only)",
    )
    return parser.parse_args(list(argv))


def main(argv: Sequence[str] | None = None) -> int:
    """Run the capture; returns the exit code."""
    args = parse_args(sys.argv[1:] if argv is None else argv)
    platforms = [p.strip() for p in args.platform.split(",") if p.strip()]
    try:
        preflight(args.root, platforms, args.allow_dirty)
        filed = capture(args.root, platforms, args.seed)
    except PreconditionError as exc:
        say(str(exc))
        return 2
    except (StepError, subprocess.CalledProcessError, manifest.ManifestError) as exc:
        say(f"FAILED: {exc}")
        return 1
    for name in filed:
        print(name)
    return 0


if __name__ == "__main__":
    sys.exit(main())
