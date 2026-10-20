# SPDX-License-Identifier: Apache-2.0
"""Runner of the end-to-end test of the alert path (brief M7-20 Design note 2, ``make alert-e2e``).

Starts the Compose smoke stack of M1-73 with the profile ``observability`` and the test overlay
``infra/compose/test/compose.alert-e2e.yaml`` from the real server image and proves the path an
alert takes (docs/observability/alerting.md):

1. Alertmanager refuses to start while its webhook URL files are missing (requirement 16).
2. The owner (``bothy setup bootstrap-owner``) sets up the channel "Operations", an app with
   ``write`` and an incoming webhook in a session with step-up (``bothy-smoke alerts``).
3. ``infra/compose/alert-setup.sh`` writes the three URLs into the secret source; a second run
   with the same URL reports ``kept``; ``secrets-init`` copies them, Alertmanager starts.
4. Prometheus loads the test rules of ``infra/compose/test/alert-e2e/rules.yaml`` next to the real
   ones; the probe sees a firing message per test alert in the channel (origin ``app``), removes the
   flag so the rules stop, and sees the resolved messages.
5. The external stub got the ``page`` alert and never the ``ticket`` alert; the heartbeat stub
   got the Watchdog.

The stack, its images and the temp directory are always removed. Exit codes: 0 pass, 1 a check
failed, 2 a precondition is missing. Steps are announced as ``alert-e2e: <step>`` on stderr; no
webhook URL is ever printed.

Example::

    uv run --project tools python tools/observability/alert_e2e.py
"""

from __future__ import annotations

import json
import os
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
from collections.abc import Callable, Iterator, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml

ROOT = Path(__file__).resolve().parents[2]
COMPOSE_DIR = ROOT / "infra" / "compose"
TEST_DIR = COMPOSE_DIR / "test"
RULES = TEST_DIR / "alert-e2e" / "rules.yaml"
PROMETHEUS_CONFIG = ROOT / "infra" / "observability" / "prometheus" / "prometheus.yaml"
SETUP_SCRIPT = COMPOSE_DIR / "alert-setup.sh"
PROFILES = "direct,smoke,observability,alert-e2e"
# The URLs of the stub in the Compose network (alert-stub of the overlay).
STUB_ONCALL = "http://alert-stub:9000/oncall"
STUB_HEARTBEAT = "http://alert-stub:9000/heartbeat"
# Intervals of the derived Prometheus configuration: the test waits seconds, not minutes, for a
# scrape and an evaluation. Alertmanager keeps its own timers (group_wait, group_interval).
E2E_INTERVAL = "5s"
STUB_TARGET = "alert-stub:9000"
GUARD_MESSAGE = "refusing to start"
SECRET_NAMES = (
    "alertmanager_bothy_webhook_url",
    "alertmanager_external_webhook_url",
    "alertmanager_heartbeat_url",
)
# Bounds: the URL file of the probe, the refusal of the guard, the whole probe (two waits of at
# most ten minutes each in bothy-smoke alerts).
URL_WAIT_S = 180
GUARD_WAIT_S = 90
PROBE_WAIT_S = 1500
DIAGNOSE_SERVICES = ("api", "alertmanager", "prometheus", "alert-stub")
MIN_ENGINE = (26, 0, 0)
MIN_COMPOSE = (2, 26, 0)


class PreconditionError(Exception):
    """A tool or input the run needs is missing (exit code 2)."""


class StepError(Exception):
    """A step of the test failed (exit code 1)."""


def say(message: str) -> None:
    """Writes one progress line to stderr."""
    print(f"alert-e2e: {message}", file=sys.stderr, flush=True)


Runner = Callable[..., subprocess.CompletedProcess[str]]


def _run(args: Sequence[str], **kwargs: Any) -> subprocess.CompletedProcess[str]:
    """Runs a command with text output; the caller decides about the exit code."""
    res: subprocess.CompletedProcess[str] = subprocess.run(
        list(args), text=True, check=False, **kwargs
    )
    return res


def parse_version(text: str) -> tuple[int, int, int]:
    """Returns the first three numbers of a version such as ``v29.5.2`` or ``2.40.3-desktop.1``."""
    nums: list[int] = []
    for part in text.lstrip("v").replace("-", ".").split("."):
        if part.isdigit():
            nums.append(int(part))
        if len(nums) == 3:
            break
    while len(nums) < 3:
        nums.append(0)
    return nums[0], nums[1], nums[2]


def default_image(root: Path = ROOT) -> str:
    """The tag ``make image`` loads: ``bothy-server:<services/server/VERSION>-local``."""
    version = (root / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    return f"bothy-server:{version}-local"


def preflight(image: str, run: Runner = _run) -> None:
    """Docker, Compose, their versions and the server image; PreconditionError names the fix."""
    if shutil.which("docker") is None:
        raise PreconditionError("docker is not on PATH; install Docker Engine 26 with Compose 2.26")
    engine = run(["docker", "version", "--format", "{{.Server.Version}}"], capture_output=True)
    if engine.returncode != 0:
        raise PreconditionError("no Docker runtime is reachable; start Docker or set DOCKER_HOST")
    if parse_version(engine.stdout.strip()) < MIN_ENGINE:
        raise PreconditionError(f"Docker Engine {engine.stdout.strip()} is too old; 26.0.0 needed")
    comp = run(["docker", "compose", "version", "--short"], capture_output=True)
    if comp.returncode != 0 or parse_version(comp.stdout.strip()) < MIN_COMPOSE:
        raise PreconditionError("docker compose 2.26.0 or newer is needed (!reset, !override)")
    if run(["docker", "image", "inspect", image], capture_output=True).returncode != 0:
        raise PreconditionError(
            f"server image {image} not found; build it with make image or set E2E_SERVER_IMAGE"
        )


@dataclass
class ExpectedAlert:
    """One test rule: alert name, severity label and summary annotation."""

    name: str
    severity: str
    summary: str

    def flag(self) -> str:
        """The value of ``bothy-smoke alerts --expect``."""
        return f"{self.name}|{self.severity}|{self.summary}"


def expected_alerts(rules: Path = RULES) -> list[ExpectedAlert]:
    """The alerts of the test rule file; StepError when one lacks a severity or a summary."""
    spec = yaml.safe_load(rules.read_text(encoding="utf-8"))
    out: list[ExpectedAlert] = []
    for group in spec["groups"]:
        for rule in group["rules"]:
            severity = rule.get("labels", {}).get("severity", "")
            summary = rule.get("annotations", {}).get("summary", "")
            if not severity or not summary:
                raise StepError(f"test rule {rule['alert']} needs a severity and a summary")
            out.append(ExpectedAlert(rule["alert"], severity, summary))
    return out


def prometheus_config(base: Path = PROMETHEUS_CONFIG) -> dict[str, Any]:
    """The configuration of the test: the real one with the intervals E2E_INTERVAL, the test rules
    as one more rule file and the scrape job of alert-stub. Alerting (the Alertmanager target)
    and the real rule files stay as they are."""
    cfg: dict[str, Any] = yaml.safe_load(base.read_text(encoding="utf-8"))
    cfg["global"]["scrape_interval"] = E2E_INTERVAL
    cfg["global"]["evaluation_interval"] = E2E_INTERVAL
    cfg["rule_files"] = [*cfg["rule_files"], "/etc/prometheus-e2e/rules.yaml"]
    cfg["scrape_configs"] = [
        *cfg["scrape_configs"],
        {"job_name": "alert-stub", "static_configs": [{"targets": [STUB_TARGET]}]},
    ]
    return cfg


def records(path: Path) -> Iterator[dict[str, Any]]:
    """The notifications the stub recorded in ``path`` (one JSON object per line)."""
    if not path.exists():
        return
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            yield json.loads(line)


def alert_names(path: Path, status: str) -> set[str]:
    """Names of the alerts with ``status`` in the notifications of ``path``."""
    return {
        a.get("labels", {}).get("alertname", "")
        for n in records(path)
        for a in n.get("alerts", [])
        if a.get("status") == status
    }


def check_stubs(stub: Path, alerts: Sequence[ExpectedAlert]) -> None:
    """The external route got every page alert and nothing else of the test; the heartbeat route
    got the Watchdog. Raises StepError naming the gap."""
    oncall = alert_names(stub / "oncall.jsonl", "firing")
    test_names = {a.name for a in alerts}
    for a in alerts:
        if a.severity == "page" and a.name not in oncall:
            raise StepError(f"the external stub got no firing {a.name}")
        if a.severity != "page" and a.name in oncall:
            raise StepError(f"the external stub got {a.name} of level {a.severity}")
    if "Watchdog" not in alert_names(stub / "heartbeat.jsonl", "firing"):
        raise StepError("the heartbeat stub got no Watchdog")
    if alert_names(stub / "heartbeat.jsonl", "firing") & test_names:
        raise StepError("the heartbeat stub got a test alert")


def check_setup_line(line: str, name: str, status: str) -> None:
    """One output line of alert-setup.sh: ``<status> <name> sha256:<12 hex>`` and nothing else."""
    parts = line.split()
    ok = (
        len(parts) == 3
        and parts[:2] == [status, name]
        and parts[2].startswith("sha256:")
        and len(parts[2]) == len("sha256:") + 12
    )
    if not ok:
        raise StepError(f"alert-setup.sh {name}: unexpected output, want '{status} {name} sha256:'")


@dataclass
class Stack:
    """One Compose project of the test: its name, the files and the variables of the overlay."""

    project: str
    env: dict[str, str]
    run: Runner = _run
    files: list[Path] = field(
        default_factory=lambda: [
            COMPOSE_DIR / "compose.yaml",
            TEST_DIR / "compose.smoke.yaml",
            TEST_DIR / "compose.alert-e2e.yaml",
        ]
    )

    def args(self, *rest: str) -> list[str]:
        """The docker compose command line of the project with ``rest``."""
        out = ["docker", "compose", "--project-name", self.project]
        out += ["--project-directory", str(COMPOSE_DIR)]
        for f in self.files:
            out += ["--file", str(f)]
        return [*out, "--env-file", str(TEST_DIR / "smoke.env"), *rest]

    def compose(
        self, *rest: str, check: bool = True, **kwargs: Any
    ) -> subprocess.CompletedProcess[str]:
        """Runs docker compose; a non-zero exit is a StepError when ``check`` is set."""
        res = self.run(self.args(*rest), env=self.env, **kwargs)
        if check and res.returncode != 0:
            raise StepError(f"docker compose {rest[0]} failed with exit code {res.returncode}")
        return res

    def logs(self, service: str) -> str:
        """The log of one service without colours."""
        return self.compose("logs", "--no-color", service, check=False, capture_output=True).stdout

    def down(self) -> None:
        """Removes every container, volume, network and local image of the project."""
        self.compose(
            "down", "--volumes", "--remove-orphans", "--rmi", "local", "--timeout", "60",
            check=False, capture_output=True,
        )  # fmt: skip

    def leftovers(self) -> list[str]:
        """Names of containers, volumes and networks that still carry the project label."""
        label = f"label=com.docker.compose.project={self.project}"
        left: list[str] = []
        for kind in (["ps", "-a"], ["volume", "ls"], ["network", "ls"]):
            fmt = "{{.Names}}" if kind[0] == "ps" else "{{.Name}}"
            res = self.run(
                ["docker", *kind, "--filter", label, "--format", fmt],
                env=self.env,
                capture_output=True,
            )
            left += [line for line in res.stdout.splitlines() if line.strip()]
        return left


def clean_env(base: dict[str, str]) -> dict[str, str]:
    """``base`` without BOTHY_*, COMPOSE_* and OTEL_* (the .env of an installation never counts)."""
    return {k: v for k, v in base.items() if not k.startswith(("BOTHY_", "COMPOSE_", "OTEL_"))}


def workspace(environ: dict[str, str]) -> Path:
    """The temp directory of the run: secrets-input (0700, the owner's source), out, out/stub
    and ctl (written by the containers, which run as the calling user), logs and prometheus
    (the derived configuration and the test rules, read by Prometheus)."""
    tmp = Path(tempfile.mkdtemp(prefix="bothy-alert-e2e.", dir=environ.get("TMPDIR")))
    (tmp / "secrets-input").mkdir(mode=0o700)
    for sub, mode in (("out", 0o777), ("out/stub", 0o777), ("ctl", 0o777), ("logs", 0o700)):
        (tmp / sub).mkdir()
        (tmp / sub).chmod(mode)
    prom = tmp / "prometheus"
    prom.mkdir()
    (prom / "prometheus.yaml").write_text(
        yaml.safe_dump(prometheus_config(), sort_keys=False), encoding="utf-8"
    )
    shutil.copy2(RULES, prom / "rules.yaml")
    for p in (prom, prom / "prometheus.yaml", prom / "rules.yaml"):
        p.chmod(0o755 if p.is_dir() else 0o644)
    return tmp


def stack_env(environ: dict[str, str], tmp: Path, image: str, run_id: str) -> dict[str, str]:
    """The environment of docker compose: the caller's without installation variables, plus the
    inputs of the overlays."""
    env = clean_env(environ)
    env.update(
        COMPOSE_PROFILES=PROFILES,
        SMOKE_SERVER_IMAGE=image,
        SMOKE_RUN_ID=run_id,
        SMOKE_SECRETS_INPUT=str(tmp / "secrets-input"),
        SMOKE_OUT=str(tmp / "out"),
        SMOKE_TMP=str(tmp),
        SMOKE_CANARY="ale" + secrets.token_hex(12),
        E2E_UID_GID=f"{os.getuid()}:{os.getgid()}",
        E2E_CTL=str(tmp / "ctl"),
        E2E_PROMETHEUS=str(tmp / "prometheus"),
    )
    return env


def wait_for(check: Callable[[], bool], seconds: float, interval: float = 2.0) -> bool:
    """Repeats ``check`` until it is true or ``seconds`` are over; reports whether it held."""
    deadline = time.monotonic() + seconds
    while True:
        if check():
            return True
        if time.monotonic() >= deadline:
            return False
        time.sleep(interval)


def guard_refuses(stack: Stack) -> None:
    """Starts Alertmanager without its URL files and expects the guard's refusal in the log,
    naming every missing file (requirement 16)."""
    stack.compose("up", "--detach", "--no-deps", "alertmanager")

    def refused() -> bool:
        return GUARD_MESSAGE in stack.logs("alertmanager")

    seen = wait_for(refused, GUARD_WAIT_S)
    log = stack.logs("alertmanager")
    stack.compose("stop", "alertmanager", check=False, capture_output=True)
    if not seen:
        raise StepError("Alertmanager started without its webhook URL files")
    missing = [n for n in SECRET_NAMES if n not in log]
    if missing:
        raise StepError("the refusal does not name " + ", ".join(missing))
    if "Listening on" in log:
        raise StepError("Alertmanager listened although its webhook URL files were missing")


def setup_secret(tmp: Path, kind: str, source: Path | str, status: str, run: Runner) -> None:
    """Runs alert-setup.sh for one route with the URL from the file or string ``source``."""
    name = {
        "channel": SECRET_NAMES[0],
        "external": SECRET_NAMES[1],
        "heartbeat": SECRET_NAMES[2],
    }[kind]
    stdin = source.read_text(encoding="utf-8") if isinstance(source, Path) else source
    res = run(
        ["sh", str(SETUP_SCRIPT), "--secrets-dir", str(tmp / "secrets-input"), kind],
        input=stdin,
        capture_output=True,
    )
    if res.returncode != 0:
        raise StepError(f"alert-setup.sh {kind} failed with exit code {res.returncode}")
    check_setup_line(res.stdout.strip(), name, status)
    say(res.stdout.strip())


def bootstrap_owner(stack: Stack, tmp: Path) -> None:
    """``bothy setup bootstrap-owner`` in api; the code goes to out/setup.out (mode 0600), which
    is never printed."""
    say("bootstrap owner")
    setup_out = tmp / "out" / "setup.out"
    with setup_out.open("w", encoding="utf-8") as out, (tmp / "logs" / "setup.err").open("w") as e:
        setup_out.chmod(0o600)
        stack.compose(
            "exec", "-T", "api", "bothy", "setup", "bootstrap-owner",
            "--handle", "alex", "--display-name", "Alex", stdout=out, stderr=e,
        )  # fmt: skip


def configure_alerting(stack: Stack, tmp: Path, url_file: Path) -> None:
    """The steps of the guide (docs/operations/alerting-setup.md): alert-setup.sh for the three
    routes (the second run of the same URL reports kept), secrets-init, then Alertmanager and
    Prometheus."""
    say("alert-setup.sh writes the three URLs")
    setup_secret(tmp, "channel", url_file, "created", stack.run)
    setup_secret(tmp, "channel", url_file, "kept", stack.run)
    setup_secret(tmp, "external", STUB_ONCALL, "created", stack.run)
    setup_secret(tmp, "heartbeat", STUB_HEARTBEAT, "created", stack.run)
    say("secrets-init, then Alertmanager and Prometheus")
    stack.compose("run", "--rm", "secrets-init", capture_output=True)
    stack.compose("up", "--detach", "--force-recreate", "--no-deps", "alertmanager")
    stack.compose("up", "--detach", "--no-deps", "prometheus")


def run_test(stack: Stack, tmp: Path, version: str, popen: Callable[..., Any]) -> None:
    """The steps of the test on a stack whose variables are set; raises StepError."""
    alerts = expected_alerts()
    say("validate")
    stack.compose("config", "--quiet")
    say("build and pull")
    stack.compose("build", "--quiet", "--build-arg", f"VERSION={version}", "smoke-probe")
    stack.compose("pull", "--quiet", "postgres", "prometheus", "alertmanager")
    say("start (secrets init, migrate up --bootstrap, api)")
    stack.compose("up", "--detach", "--wait", "--wait-timeout", "240", "api")
    say("Alertmanager refuses to start without its webhook URL files")
    guard_refuses(stack)
    bootstrap_owner(stack, tmp)
    say("start the stub (test rules firing)")
    (tmp / "ctl" / "firing").write_text("", encoding="utf-8")
    stack.compose("up", "--detach", "--no-deps", "alert-stub")
    say("probe: channel Operations, app with write and incoming webhook with step-up")
    probe_cmd = stack.args(
        "run", "--rm", "--no-deps", "-T", "alert-probe", "alerts",
        *[arg for a in alerts for arg in ("--expect", a.flag())],
    )  # fmt: skip
    log = (tmp / "logs" / "probe.log").open("w", encoding="utf-8")
    probe = popen(probe_cmd, env=stack.env, stdout=log, stderr=subprocess.STDOUT, text=True)
    try:
        url_file = tmp / "out" / "webhook-url"
        if not wait_for(lambda: url_file.exists() or probe.poll() is not None, URL_WAIT_S):
            raise StepError("the probe wrote no webhook URL in time")
        if not url_file.exists():
            raise StepError("the probe ended before it wrote the webhook URL; see logs/probe.log")
        configure_alerting(stack, tmp, url_file)
        say("probe: firing messages, then resolved messages (Alertmanager group_interval)")
        try:
            rc = probe.wait(timeout=PROBE_WAIT_S)
        except subprocess.TimeoutExpired as e:
            raise StepError("the probe did not finish in time") from e
    finally:
        if probe.poll() is None:
            probe.kill()
            probe.wait()
        log.close()
    sys.stderr.write((tmp / "logs" / "probe.log").read_text(encoding="utf-8"))
    if rc != 0:
        raise StepError("the probe failed; see the lines above")
    say("external and heartbeat stubs")
    check_stubs(tmp / "out" / "stub", alerts)
    for m in json.loads((tmp / "out" / "alert-messages.json").read_text(encoding="utf-8")):
        say("message (origin " + m["origin"] + "):\n" + m["content_md"])


def cleanup(stack: Stack, tmp: Path, run_id: str) -> bool:
    """Removes the stack, the probe image and the temp directory; reports whether nothing is
    left."""
    say("cleanup")
    stack.down()
    stack.run(["docker", "image", "rm", f"bothy-smoke:{run_id}"], capture_output=True)
    shutil.rmtree(tmp, ignore_errors=True)
    left = stack.leftovers()
    if left:
        say("cleanup left objects behind: " + ", ".join(left))
    return not left


def save_artifacts(tmp: Path, target: str | None) -> None:
    """Copies the recorded messages and the stub records (no URL, no token) to ``target``."""
    if not target:
        return
    dest = Path(target)
    dest.mkdir(parents=True, exist_ok=True)
    for f in [tmp / "out" / "alert-messages.json", *sorted((tmp / "out" / "stub").glob("*"))]:
        if f.is_file():
            shutil.copy2(f, dest / f.name)


def main(
    argv: Sequence[str] | None = None,
    environ: dict[str, str] | None = None,
    run: Runner = _run,
    popen: Callable[..., Any] = subprocess.Popen,
) -> int:
    """Runs the test and returns the exit code; the stack and the temp directory always go,
    unless E2E_KEEP=1 keeps them for debugging."""
    del argv
    environ = dict(os.environ if environ is None else environ)
    image = environ.get("E2E_SERVER_IMAGE") or default_image()
    try:
        preflight(image, run)
    except PreconditionError as e:
        say(str(e))
        return 2
    tmp = workspace(environ)
    run_id = secrets.token_hex(4)
    stack = Stack(project=f"bothy-alert-e2e-{run_id}", env=stack_env(environ, tmp, image, run_id))
    stack.run = run
    version = (ROOT / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    rc = 0
    try:
        run_test(stack, tmp, version, popen)
        say("PASS")
    except StepError as e:
        rc = 1
        say(f"FAIL: {e}")
        stack.compose("ps", "--all", check=False)
        for svc in DIAGNOSE_SERVICES:
            stack.compose("logs", "--no-color", "--tail", "100", svc, check=False)
    finally:
        save_artifacts(tmp, environ.get("E2E_ARTIFACTS"))
        if environ.get("E2E_KEEP") == "1":
            say(f"E2E_KEEP=1: the stack {stack.project} and {tmp} are kept")
        elif not cleanup(stack, tmp, run_id):
            rc = 1
    return rc


if __name__ == "__main__":
    sys.exit(main())
