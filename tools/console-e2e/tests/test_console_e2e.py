# SPDX-License-Identifier: Apache-2.0
"""Tests of the runner of ``make console-e2e`` (briefs M6-03, M6-04 and M6-18): teardown after a
failure, the preconditions, the pins of the overlays, the CI job, phase 2 (the patrol suites of the
console administration: bundle build, overlay, order), phase 3 (the terminal: host image, host
binaries, host service container, pairing, result) and the pure helpers of headless/admin_suites.py
and headless/terminal_suites.py. The browser flows themselves run only against the stack
(``make console-e2e``)."""

from __future__ import annotations

import base64
import datetime
import hashlib
import json
import re
import subprocess
import sys
import tomllib
import uuid
from collections.abc import Callable
from pathlib import Path

import components
import console_e2e as e2e
import pytest
import yaml
from cryptography import x509
from cryptography.hazmat.primitives import serialization

ROOT = Path(__file__).resolve().parents[3]
OVERLAY = ROOT / "infra" / "compose" / "test" / "compose.console-e2e.yaml"
ADMIN_OVERLAY = ROOT / "infra" / "compose" / "test" / "compose.console-e2e-admin.yaml"
TERMINAL_OVERLAY = ROOT / "infra" / "compose" / "test" / "compose.console-e2e-terminal.yaml"
GRAFANA_OVERLAY = ROOT / "infra" / "compose" / "test" / "compose.console-e2e-grafana.yaml"
CI = ROOT / ".github" / "workflows" / "ci.yml"
DOCKERFILE = ROOT / "services" / "server" / "Dockerfile"
DOCKERIGNORE = ROOT / "services" / "server" / "Dockerfile.dockerignore"

# The headless modules run in the browser image with their directory on sys.path; the pure
# helpers of admin_suites are tested here the same way.
sys.path.insert(0, str(ROOT / "tools" / "console-e2e" / "headless"))
import admin_suites as admin  # noqa: E402
import grafana_suites as graf  # noqa: E402
import terminal_suites as term  # noqa: E402

REPORT_LINE = (
    '{"msg":"csp violation reported","bothy.csp.directive":"%s",'
    '"bothy.csp.blocked_origin":"inline"}'
)

EXPORT = """playwright==1.64.0 \\
    --hash=sha256:aaa \\
    --hash=sha256:bbb
    # via bothy-devtools
greenlet==3.5.6 \\
    --hash=sha256:ccc
    # via playwright
pyee==14.0.0 \\
    --hash=sha256:ddd
typing-extensions==4.16.0 \\
    --hash=sha256:eee
    # via
    #   pyee
pytest==9.1.1 \\
    --hash=sha256:fff
"""

# The CA file of the server image, as docker cp copies it out (a stand-in with one certificate).
ROOTS_PEM = "-----BEGIN CERTIFICATE-----\nTUlJ\n-----END CERTIFICATE-----\n"


def receiver_line(path: str, webhook_id: str | None, status: int = 200) -> str:
    """One access log line of the webhook receiver (caddy's JSON access log)."""
    headers: dict[str, list[str]] = {"Content-Type": ["application/json"]}
    if webhook_id is not None:
        headers["Webhook-Id"] = [webhook_id]
    return json.dumps(
        {
            "level": "info",
            "logger": "http.log.access",
            "msg": "handled request",
            "request": {"method": "POST", "uri": path, "headers": headers},
            "status": status,
        }
    )


RECEIVER_LOG = receiver_line("/hooks/second", "msg_0192f4a0") + "\n"


def proxy_line(
    host: str, path: str, headers: dict[str, list[str]] | None = None, status: int = 200
) -> str:
    """One access log line of the Grafana test proxy (caddy's JSON access log)."""
    return json.dumps(
        {
            "level": "info",
            "logger": "http.log.access",
            "msg": "handled request",
            "request": {
                "method": "GET",
                "host": host,
                "uri": path,
                "headers": headers or {"Sec-Fetch-Dest": ["iframe"]},
            },
            "status": status,
        }
    )


PROXY_LOG = proxy_line("grafana.localhost:9444", "/d/bothy-ops/operations") + "\n"


SUBCOMMANDS = {"config", "build", "pull", "up", "exec", "run", "logs", "down", "ps", "cp", "rm"}
# A fake pairing code, shorter than the real format so no secret scanner takes it for one.
PAIRING_CODE = "bothy_hpc_FAKEPAIRINGCODE_FAKECD"
FINGERPRINT = "7K2Q-M9XD-AB12-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000"


def terminal_result(ok: bool = True) -> dict[str, object]:
    """The result file the browser container writes in phase 3 (headless/terminal_suites.py)."""
    return {
        "browser": "chromium",
        "ok": ok,
        "checks": {name: {"ok": ok} for name in term.CHECKS},
        "measurement": {
            "frame_to_paint_p95_ms": 9.5,
            "host_to_paint_p95_ms": 21.0,
            "throughput_bytes_per_s": 3_000_000.0,
        },
    }


class FakeDocker:
    """Records every command and its environment; ``fail_on`` makes the compose subcommand of
    that name fail, ``fail_admin`` only in phase 2 (calls with the admin overlay),
    ``fail_terminal`` only in phase 3 (calls with the terminal overlay), ``fail_bundle`` the
    build of the admin bundle.

    Phase 3: ``host_image`` says whether the host image exists already; the detached browser run
    writes the pairing request into SMOKE_OUT (unless ``browser_exits_early``), ``docker wait``
    writes the result file (``result_ok``, none with ``no_result``) and answers ``browser_rc``;
    ``fail_pair`` makes bothy-host pair fail."""

    def __init__(
        self,
        fail_on: str | None = None,
        image: bool = True,
        leftovers: str = "",
        fail_admin: str | None = None,
        fail_bundle: bool = False,
        receiver_log: str = RECEIVER_LOG,
        fail_roots: bool = False,
        *,
        fail_terminal: str | None = None,
        host_image: bool = False,
        browser_exits_early: bool = False,
        browser_rc: int = 0,
        fail_pair: bool = False,
        result_ok: bool = True,
        no_result: bool = False,
        fail_build_of: str | None = None,
        boot_states: tuple[str, ...] = (),
        wait_timeout: bool = False,
        proxy_log: str = PROXY_LOG,
        same_origin_check_rc: int = 1,
        fail_grafana: str | None = None,
    ) -> None:
        self.calls: list[list[str]] = []
        self.receiver_log = receiver_log
        self.fail_roots = fail_roots
        self.envs: list[dict[str, str]] = []
        self.fail_on = fail_on
        self.fail_admin = fail_admin
        self.fail_terminal = fail_terminal
        self.fail_bundle = fail_bundle
        self.image = image
        self.host_image = host_image
        self.leftovers = leftovers
        self.browser_exits_early = browser_exits_early
        self.browser_rc = browser_rc
        self.fail_pair = fail_pair
        self.result_ok = result_ok
        self.no_result = no_result
        self.out: Path | None = None
        self.seen_pairing_at_pair: bool | None = None
        self.paired: dict[str, str] | None = None
        self.fail_build_of = fail_build_of
        self.boot_states = list(boot_states)
        self.wait_timeout = wait_timeout
        self.proxy_log = proxy_log
        self.same_origin_check_rc = same_origin_check_rc
        self.fail_grafana = fail_grafana
        self.stderr = ""

    def __call__(self, args, **kwargs):
        self.calls.append(list(args))
        env = dict(kwargs.get("env") or {})
        self.envs.append(env)
        if env.get("SMOKE_OUT"):
            self.out = Path(env["SMOKE_OUT"])
        out, rc = "", 0
        self.stderr = ""
        if args[:3] == ["docker", "buildx", "build"]:
            rc = 1 if self.fail_bundle else 0
            if self.fail_build_of is not None and any(self.fail_build_of in a for a in args):
                return subprocess.CompletedProcess(args, 1, stdout="", stderr="")
            if not self.fail_bundle and "--output" in args:  # the local exporter writes files
                dest = Path(args[args.index("--output") + 1].split("dest=", 1)[1])
                dest.mkdir(parents=True)
                for name in ("index.html", "bothy-host", "claude"):
                    (dest / name).write_text("x", encoding="utf-8")
        if args[:2] == ["docker", "version"]:
            out = "29.5.2"
        elif args[:3] == ["docker", "compose", "version"]:
            out = "5.6.0"
        elif args[:3] == ["docker", "image", "inspect"]:
            if args[3].startswith("termsuite-host:"):
                rc = 0 if self.host_image else 1
            else:
                rc = 0 if self.image else 1
        elif args[:1] == ["uv"]:
            out = EXPORT
        elif args[:2] == ["docker", "ps"]:
            out = self.leftovers
        elif args[:2] == ["docker", "create"]:
            out = "cid0roots\n"
        elif args[:2] == ["docker", "cp"]:
            rc = 1 if self.fail_roots else 0
            if not self.fail_roots:
                Path(args[-1]).write_text(ROOTS_PEM, encoding="utf-8")
        elif args[:2] == ["docker", "inspect"]:
            out = "false" if self.browser_exits_early else "true"
        elif args[:2] == ["docker", "wait"]:
            if self.wait_timeout:
                raise subprocess.TimeoutExpired(args, kwargs.get("timeout") or 0)
            out, rc = self._browser_done()
        elif args[:2] == ["docker", "compose"]:
            out, rc = self._compose(args)
        stdout = kwargs.get("stdout")
        if hasattr(stdout, "write"):
            stdout.write("")
        return subprocess.CompletedProcess(args, rc, stdout=out, stderr=self.stderr)

    def _compose(self, args: list[str]) -> tuple[str, int]:
        out, rc = "", 0
        sub = next((a for a in args[2:] if a in SUBCOMMANDS), "")
        if sub == self.fail_on:
            rc = 1
        if sub == self.fail_admin and str(ADMIN_OVERLAY) in args:
            rc = 1
        if sub == self.fail_terminal and str(TERMINAL_OVERLAY) in args:
            rc = 1
        if sub == self.fail_grafana and str(GRAFANA_OVERLAY) in args:
            rc = 1
        if sub == "logs":
            out = {
                "webhook-receiver": self.receiver_log,
                "grafana-proxy": self.proxy_log,
            }.get(args[-1], REPORT_LINE % "script-src-elem")
        if sub == "exec" and "check" in args and any(a.startswith(e2e.EMBED_KEY_ENV) for a in args):
            rc = self.same_origin_check_rc
            self.stderr = (
                "config: console.grafana_embed_url must not have the origin of server.public_url\n"
            )
        if sub == "run" and "--detach" in args and rc == 0 and not self.browser_exits_early:
            self._ask_for_pairing()
        if sub == "exec" and "is-system-running" in args:
            out = (self.boot_states.pop(0) if self.boot_states else "running") + "\n"
        if sub == "exec" and "pair" in args:
            self.seen_pairing_at_pair = (self.out / e2e.PAIRING_FILE).exists()
            rc = 1 if self.fail_pair else rc
            out = "" if self.fail_pair else f"paired\nfingerprint: {FINGERPRINT}\n"
        return out, rc

    def _ask_for_pairing(self) -> None:
        assert self.out is not None
        body = {"host_id": "0192f4a0-7b1c-7a11-8c3e-4f5a6b7c8d9e", "pairing_code": PAIRING_CODE}
        (self.out / e2e.PAIRING_FILE).write_text(json.dumps(body), encoding="utf-8")

    def _browser_done(self) -> tuple[str, int]:
        assert self.out is not None
        paired = self.out / e2e.PAIRED_FILE
        self.paired = json.loads(paired.read_text(encoding="utf-8")) if paired.exists() else None
        if not self.no_result:
            results = self.out / "results"
            results.mkdir(exist_ok=True)
            doc = terminal_result(self.result_ok)
            (results / e2e.TERMINAL_RESULT).write_text(json.dumps(doc), encoding="utf-8")
        return f"{self.browser_rc}\n", 0

    def compose_subcommands(self) -> list[str]:
        subs = []
        for c in self.calls:
            if c[:2] == ["docker", "compose"] and "version" not in c:
                subs.append(next(a for a in c[2:] if a in SUBCOMMANDS))
        return subs


@pytest.fixture
def environ(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> dict[str, str]:
    monkeypatch.setattr(e2e.shutil, "which", lambda _name: "/usr/bin/docker")
    return {
        "TMPDIR": str(tmp_path),
        "PATH": "/usr/bin",
        "BOTHY_PUBLIC_URL": "https://leak.example.com",
        "E2E_SERVER_IMAGE": "img:1",
    }


def test_pass_runs_every_step_and_tears_down(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    subs = fake.compose_subcommands()
    assert subs[0] == "config"
    assert subs[-1] == "down"
    assert "run" in subs
    assert "exec" in subs
    assert list(tmp_path.iterdir()) == []  # the temp directory is gone
    # The variables of an installation never reach Compose.
    for c in fake.calls:
        assert "leak.example.com" not in " ".join(c)


def test_teardown_after_a_failure(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker(fail_on="up")
    assert e2e.main([], environ, fake) == 1
    subs = fake.compose_subcommands()
    assert "down" in subs
    assert "run" not in subs  # nothing after the failed step
    assert any(c[:3] == ["docker", "image", "rm"] for c in fake.calls)
    assert list(tmp_path.iterdir()) == []


def test_failed_browsers_save_results_and_tear_down(
    environ: dict[str, str], tmp_path: Path
) -> None:
    artifacts = tmp_path / "artifacts"
    environ["E2E_ARTIFACTS"] = str(artifacts)
    fake = FakeDocker(fail_on="run")
    assert e2e.main([], environ, fake) == 1
    assert "down" in fake.compose_subcommands()
    assert {p.name for p in tmp_path.iterdir()} <= {"artifacts"}  # the temp directory is gone


def test_leftovers_fail_the_run(environ: dict[str, str]) -> None:
    assert e2e.main([], environ, FakeDocker(leftovers="bothy-console-e2e-x-api-1")) == 1


def test_missing_image_is_a_precondition(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker(image=False)
    assert e2e.main([], environ, fake) == 2
    assert fake.compose_subcommands() == []
    assert list(tmp_path.iterdir()) == []


def test_browser_requirements_takes_the_pinned_entries() -> None:
    req = e2e.browser_requirements(EXPORT)
    assert req.splitlines()[0] == "playwright==1.64.0 \\"
    assert "pytest" not in req
    assert req.count("--hash=") == 5
    with pytest.raises(e2e.PreconditionError):
        e2e.browser_requirements("playwright==1.64.0\n")


def test_report_seen() -> None:
    assert e2e.report_seen("x " + REPORT_LINE % "script-src-elem")
    assert not e2e.report_seen(REPORT_LINE % "style-src-elem")
    assert not e2e.report_seen('{"msg":"http request"}')


def test_certs_name_the_origins_and_pin_the_leaf(tmp_path: Path) -> None:
    now = datetime.datetime(2026, 10, 20, tzinfo=datetime.UTC)
    certs = e2e.make_certs(tmp_path / "c", now)
    leaf, ca = x509.load_pem_x509_certificates((certs.directory / "leaf.pem").read_bytes())
    sans = leaf.extensions.get_extension_for_class(x509.SubjectAlternativeName).value
    # webhook-receiver: the receiver of the outgoing webhooks of phase 2 (brief M6-08).
    # grafana.localhost: the Grafana test proxy of phase 1b (brief M6-17).
    assert set(sans.get_values_for_type(x509.DNSName)) == {
        "localhost",
        "api",
        "webhook-receiver",
        "grafana.localhost",
    }
    assert leaf.issuer == ca.subject
    spki = leaf.public_key().public_bytes(
        serialization.Encoding.DER, serialization.PublicFormat.SubjectPublicKeyInfo
    )
    assert certs.spki_sha256 == base64.b64encode(hashlib.sha256(spki).digest()).decode()
    assert (certs.directory / "leaf.key").stat().st_mode & 0o077 == 0
    assert leaf.not_valid_after_utc - now <= datetime.timedelta(days=1)


def test_overlay_images_are_pinned_and_match_the_lock() -> None:
    overlay = OVERLAY.read_text(encoding="utf-8")
    lock = tomllib.loads((ROOT / "tools" / "uv.lock").read_text(encoding="utf-8"))
    version = next(p["version"] for p in lock["package"] if p["name"] == "playwright")
    images = re.findall(r"(?:image:|FROM) +(\S+)", overlay)
    pinned = [i for i in images if "${" not in i]
    assert pinned, "no external image in the overlay"
    for image in pinned:
        assert re.search(r"@sha256:[0-9a-f]{64}$", image), f"{image} is not pinned by digest"
    assert any(i.startswith(f"mcr.microsoft.com/playwright/python:v{version}-") for i in pinned)
    data = yaml.safe_load(overlay)
    for name in ("tls", "console-seed", "console-browser"):
        svc = data["services"][name]
        assert svc["network_mode"] == "service:api", name
        assert "ports" not in svc, name
        assert svc["read_only"] is True, name
        assert svc["cap_drop"] == ["ALL"], name


def test_webhook_receiver_is_hardened_and_unpublished() -> None:
    data = yaml.safe_load(OVERLAY.read_text(encoding="utf-8"))
    tls = data["services"]["tls"]
    svc = data["services"]["webhook-receiver"]
    # The same pinned caddy image as the TLS sidecar, test only, on the Compose network.
    assert svc["image"] == tls["image"]
    assert svc["profiles"] == ["console-e2e"]
    assert "network_mode" not in svc
    assert "ports" not in svc
    assert "expose" not in svc
    for key in ("user", "read_only", "cap_drop", "cap_add", "security_opt", "tmpfs"):
        assert svc[key] == tls[key], key
    assert svc["volumes"] == [
        "./test/console-e2e/receiver.Caddyfile:/etc/caddy-e2e/Caddyfile:ro",
        "${E2E_CERTS:?set by console_e2e.py}:/certs:ro",
    ]
    assert svc["command"] == tls["command"]


def test_receiver_caddyfile_serves_https_and_logs_every_request() -> None:
    text = (OVERLAY.parent / "console-e2e" / "receiver.Caddyfile").read_text(encoding="utf-8")
    assert "https://:8443 {" in text
    assert "tls /certs/leaf.pem /certs/leaf.key" in text
    assert re.search(r"log \{\s*output stdout\s*format json\s*\}", text)
    assert re.search(r"respond @post 200", text)
    assert "reverse_proxy" not in text


def test_ci_job_is_pinned_read_only_and_feeds_ci_ok() -> None:
    wf = yaml.safe_load(CI.read_text(encoding="utf-8"))
    job = wf["jobs"]["console-e2e"]
    assert job["permissions"] == {"contents": "read"}
    # The plan decides (tools/ci/components.py CONSOLE_E2E_JOBS); no other condition skips it.
    assert job["if"] == "contains(format(' {0} ', needs.plan.outputs.jobs), ' console-e2e ')"
    for step in job["steps"]:
        if "uses" in step:
            assert re.search(r"@[0-9a-f]{40}$", step["uses"]), step["uses"]
    assert any(step.get("run") == "make console-e2e" for step in job["steps"])
    # ci-ok needs the job; tools/ci/ci_ok.py accepts a skip only when the plan left it out.
    assert "console-e2e" in wf["jobs"]["ci-ok"]["needs"]
    assert "console-e2e" in components.CONDITIONAL_JOBS


def test_make_target_outside_gates() -> None:
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    assert re.search(r"^console-e2e: ##", makefile, re.M)
    for line in re.findall(r"^gates:(?! ##).*(?:\\\n.*)*", makefile, re.M):
        assert "console-e2e" not in line


# ---- phase 2: the patrol suites of the console administration (brief M6-04) ---------------


def _compose_calls(fake: FakeDocker) -> list[tuple[list[str], dict[str, str]]]:
    return [
        (c, e)
        for c, e in zip(fake.calls, fake.envs, strict=True)
        if c[:2] == ["docker", "compose"] and "version" not in c
    ]


def test_bundle_build_command_line(tmp_path: Path) -> None:
    assert e2e.bundle_build_args(tmp_path, "1.2.3") == [
        "docker",
        "buildx",
        "build",
        "--quiet",
        "--file",
        str(ROOT / "services" / "server" / "Dockerfile"),
        "--target",
        "console-e2e-web",
        "--build-arg",
        "VERSION=1.2.3",
        "--output",
        f"type=local,dest={tmp_path / 'admin-bundle'}",
        str(ROOT),
    ]


def test_bundle_is_built_in_the_build_step(environ: dict[str, str]) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    builds = [
        i
        for i, c in enumerate(fake.calls)
        if c[:3] == ["docker", "buildx", "build"] and "console-e2e-web" in c
    ]
    assert len(builds) == 1
    first_up = next(
        i for i, c in enumerate(fake.calls) if c[:2] == ["docker", "compose"] and "up" in c
    )
    assert builds[0] < first_up
    version = (ROOT / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    assert f"VERSION={version}" in fake.calls[builds[0]]


def test_phase_two_adds_the_admin_overlay_after_phase_one(environ: dict[str, str]) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    calls = _compose_calls(fake)
    with_overlay = [i for i, (c, _) in enumerate(calls) if str(ADMIN_OVERLAY) in c]
    # The report check of phase 1 (logs api) comes before the first call of phase 2.
    report = next(i for i, (c, _) in enumerate(calls) if c[-2:] == ["--no-color", "api"])
    assert with_overlay
    assert min(with_overlay) > report
    # Phase 2 keeps every file of phase 1 and appends the overlay last.
    first = calls[with_overlay[0]][0]
    files = [first[i + 1] for i, a in enumerate(first) if a == "--file"]
    assert files == [str(f) for f in e2e.Stack(project="x", env={}).files] + [str(ADMIN_OVERLAY)]
    phase2 = [c for c, _ in calls if str(ADMIN_OVERLAY) in c]
    ups = [c for c in phase2 if "up" in c]
    # The webhook receiver first, so the first delivery finds it; then api and the worker (which
    # runs the deliveries) with --wait, then the sidecar in the network namespace of api.
    assert ups[0][-1] == "webhook-receiver"
    assert "--no-deps" in ups[0]
    assert ups[1][-2:] == ["api", "worker"]
    assert "--wait" in ups[1]
    assert "--force-recreate" in ups[1]
    assert "--no-deps" in ups[1]
    assert ups[2][-1] == "tls"
    assert "--force-recreate" in ups[2]
    assert phase2.index(ups[0]) < phase2.index(ups[1]) < phase2.index(ups[2])
    runs = [(c, e) for c, e in calls if str(ADMIN_OVERLAY) in c and "run" in c]
    assert len(runs) == 1
    assert runs[0][0][-1] == "console-browser"
    assert runs[0][1]["E2E_MODE"] == "admin"
    assert phase2.index(ups[2]) < phase2.index(runs[0][0])
    # The deliveries the receiver recorded are read after the suites.
    receiver_logs = [c for c in phase2 if "logs" in c and c[-1] == "webhook-receiver"]
    assert len(receiver_logs) == 1
    assert "--no-log-prefix" in receiver_logs[0]
    assert phase2.index(runs[0][0]) < phase2.index(receiver_logs[0])
    # Phase 1 never sees E2E_MODE=admin.
    for c, env in calls:
        if str(ADMIN_OVERLAY) not in c and "run" in c:
            assert env.get("E2E_MODE") != "admin"
    assert "down" in calls[-1][0]


def test_phase_one_failure_skips_phase_two(environ: dict[str, str]) -> None:
    fake = FakeDocker(fail_on="run")
    assert e2e.main([], environ, fake) == 1
    assert not any(str(ADMIN_OVERLAY) in c and "up" in c for c in fake.calls)


@pytest.mark.parametrize("sub", ["up", "run"])
def test_teardown_after_a_phase_two_failure(
    environ: dict[str, str], tmp_path: Path, sub: str
) -> None:
    fake = FakeDocker(fail_admin=sub)
    assert e2e.main([], environ, fake) == 1
    assert "down" in fake.compose_subcommands()
    assert any(c[:3] == ["docker", "image", "rm"] for c in fake.calls)
    assert list(tmp_path.iterdir()) == []  # the bundle lives in the temp directory, gone too


def test_bundle_build_failure_fails_and_tears_down(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker(fail_bundle=True)
    assert e2e.main([], environ, fake) == 1
    subs = fake.compose_subcommands()
    assert "up" not in subs
    assert "down" in subs
    assert list(tmp_path.iterdir()) == []


def test_stack_env_names_the_bundle_inside_the_temp_directory(tmp_path: Path) -> None:
    env = e2e.stack_env({}, tmp_path, "img:1", "abcd")
    assert env["E2E_ADMIN_BUNDLE"] == str(tmp_path / "admin-bundle")
    assert "E2E_MODE" not in env


def test_share_bundle_makes_the_files_readable(tmp_path: Path) -> None:
    bundle = tmp_path / "admin-bundle"
    (bundle / "assets").mkdir(parents=True)
    (bundle / "index.html").write_text("x", encoding="utf-8")
    (bundle / "index.html").chmod(0o600)
    (bundle / "assets").chmod(0o700)
    e2e.share_bundle(bundle)
    assert (bundle / "index.html").stat().st_mode & 0o777 == 0o644
    assert (bundle / "assets").stat().st_mode & 0o777 == 0o755
    assert bundle.stat().st_mode & 0o777 == 0o755


def test_share_bundle_requires_an_index(tmp_path: Path) -> None:
    (tmp_path / "admin-bundle").mkdir()
    with pytest.raises(e2e.StepError):
        e2e.share_bundle(tmp_path / "admin-bundle")


def test_admin_overlay_pins() -> None:
    text = ADMIN_OVERLAY.read_text(encoding="utf-8")
    assert "TEST ONLY" in text
    data = yaml.safe_load(text)
    assert set(data["services"]) == {"api", "worker", "console-browser"}
    for name, svc in data["services"].items():
        assert "ports" not in svc, name
    # The receiver of the outgoing webhooks (brief M6-08): api checks the addresses, the worker
    # delivers; both trust the test CA through the bundle the runner writes.
    hooks = {
        "BOTHY_WEBHOOK__ALLOWED_HOSTS": "webhook-receiver:8443",
        "BOTHY_EGRESS__ALLOWED_HOSTS": "webhook-receiver:8443;allow_private",
        "SSL_CERT_FILE": "/etc/bothy-e2e/ca-bundle.pem",
    }
    bundle = "${E2E_CERTS:?set by console_e2e.py}/ca-bundle.pem:/etc/bothy-e2e/ca-bundle.pem:ro"
    api = data["services"]["api"]
    assert api["environment"] == {
        "BOTHY_CONSOLE__ASSETS_DIR": "/usr/share/bothy/console-e2e",
        "BOTHY_AUTH__STEP_UP_MAX_AGE": "3s",
        **hooks,
    }
    assert api["volumes"] == [
        "${E2E_ADMIN_BUNDLE:?set by console_e2e.py}:/usr/share/bothy/console-e2e:ro",
        bundle,
    ]
    worker = data["services"]["worker"]
    assert worker["environment"] == hooks
    assert worker["volumes"] == [bundle]
    assert data["services"]["console-browser"]["environment"] == {"E2E_MODE": "${E2E_MODE:-}"}


def test_admin_overlay_sets_only_existing_configuration_keys() -> None:
    data = yaml.safe_load(ADMIN_OVERLAY.read_text(encoding="utf-8"))
    keys = ROOT / "services" / "server" / "internal" / "wire"
    names = "".join(p.read_text(encoding="utf-8") for p in keys.glob("configkeys_*.go"))
    for svc in ("api", "worker"):
        for var in data["services"][svc]["environment"]:
            if not var.startswith("BOTHY_"):
                assert var == "SSL_CERT_FILE", var  # read by Go's crypto/x509, not the server
                continue
            key = var.removeprefix("BOTHY_").lower().replace("__", ".")
            # A key is named in its Name field or in a constant of the same files.
            assert f'"{key}"' in names, key


# ---- phase 2: the webhook receiver and the trust of the test CA (brief M6-08) --------------


def test_system_roots_come_from_the_server_image_and_the_container_goes(tmp_path: Path) -> None:
    fake = FakeDocker()
    pem = e2e.system_roots("img:1", tmp_path, fake)
    assert pem == ROOTS_PEM.encode()
    assert fake.calls[0][:2] == ["docker", "create"]
    assert "img:1" in fake.calls[0]
    assert fake.calls[1][:3] == ["docker", "cp", "-L"]
    assert fake.calls[1][3] == "cid0roots:/etc/ssl/certs/ca-certificates.crt"
    assert fake.calls[-1] == ["docker", "rm", "cid0roots"]
    assert list(tmp_path.iterdir()) == []  # the copy is read and removed


def test_system_roots_failure_removes_the_container(tmp_path: Path) -> None:
    fake = FakeDocker(fail_roots=True)
    with pytest.raises(e2e.StepError, match="CA file"):
        e2e.system_roots("img:1", tmp_path, fake)
    assert fake.calls[-1] == ["docker", "rm", "cid0roots"]


def test_trust_bundle_holds_the_roots_and_the_test_ca(tmp_path: Path) -> None:
    certs = e2e.make_certs(tmp_path / "c")
    bundle = e2e.write_trust_bundle(certs.directory, ROOTS_PEM.encode())
    text = bundle.read_text(encoding="utf-8")
    assert bundle == certs.directory / "ca-bundle.pem"
    assert text.startswith(ROOTS_PEM)
    assert text.endswith((certs.directory / "ca.pem").read_text(encoding="utf-8"))
    assert bundle.stat().st_mode & 0o777 == 0o644  # api and worker run as 65532
    with pytest.raises(e2e.StepError, match="no certificate"):
        e2e.write_trust_bundle(certs.directory, b"")


def test_deliveries_read_method_path_and_webhook_id() -> None:
    logs = "\n".join(
        [
            "not json",
            '{"level":"warn","msg":"something else"}',
            receiver_line("/hooks/events", "msg_1"),
            receiver_line("/hooks/second", None),
            receiver_line("/hooks/second", "msg_2", status=500),
            "x | " + receiver_line("/hooks/second", "msg_3"),
        ]
    )
    got = e2e.deliveries(logs)
    assert got == [
        e2e.Delivery("POST", "/hooks/events", "msg_1", 200),
        e2e.Delivery("POST", "/hooks/second", "", 200),
        e2e.Delivery("POST", "/hooks/second", "msg_2", 500),
        e2e.Delivery("POST", "/hooks/second", "msg_3", 200),
    ]


def test_check_deliveries_needs_a_signed_delivery() -> None:
    e2e.check_deliveries([e2e.Delivery("POST", "/hooks/second", "msg_1", 200)])
    for bad in (
        [],
        [e2e.Delivery("POST", "/hooks/second", "", 200)],
        [e2e.Delivery("GET", "/hooks/second", "msg_1", 200)],
        [e2e.Delivery("POST", "/hooks/second", "msg_1", 500)],
    ):
        with pytest.raises(e2e.StepError, match="webhook-id"):
            e2e.check_deliveries(bad)


def test_phase_two_writes_the_trust_bundle_before_api_starts(environ: dict[str, str]) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    create = next(i for i, c in enumerate(fake.calls) if c[:2] == ["docker", "create"])
    removed = next(i for i, c in enumerate(fake.calls) if c == ["docker", "rm", "cid0roots"])
    first_admin = next(i for i, c in enumerate(fake.calls) if str(ADMIN_OVERLAY) in c)
    assert create < removed < first_admin
    assert "img:1" in fake.calls[create]


def test_no_delivery_at_the_receiver_fails_the_run(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker(receiver_log=receiver_line("/hooks/second", None) + "\n")
    assert e2e.main([], environ, fake) == 1
    assert "down" in fake.compose_subcommands()
    assert list(tmp_path.iterdir()) == []


def test_failure_logs_name_the_worker_and_the_receiver() -> None:
    assert {"api", "worker", "webhook-receiver"} <= set(e2e.DIAGNOSE_SERVICES)


def _stages(text: str) -> list[tuple[str, str]]:
    return re.findall(r"^FROM (?:--platform=\S+ )?(\S+)(?: AS (\S+))?$", text, re.M)


def test_dockerfile_console_e2e_stages() -> None:
    text = DOCKERFILE.read_text(encoding="utf-8")
    stages = _stages(text)
    names = [alias for _, alias in stages]
    assert ("console", "console-e2e-build") in stages
    assert ("scratch", "console-e2e-web") in stages
    assert (
        names.index("console") < names.index("console-e2e-build") < names.index("console-e2e-web")
    )
    assert stages[-1][1] == ""  # the production stage stays last
    console = text.split(" AS console\n", 1)[1].split("\nFROM ", 1)[0]
    assert "COPY packages/admin_ui/lib packages/admin_ui/lib" in console
    build = text.split(" AS console-e2e-build\n", 1)[1].split("\nFROM ", 1)[0]
    flags = "--base-href /console/ --no-web-resources-cdn --no-pub --no-wasm-dry-run"
    assert flags in console
    assert flags in build
    assert "--target integration_test/admin/admin_bundle.dart" in build
    assert "tool/version_json.sh" in build
    assert "COPY" not in build  # the context already holds the suites
    web = text.split(" AS console-e2e-web\n", 1)[1].split("\nFROM ", 1)[0]
    assert re.findall(r"^COPY .*$", web, re.M) == [
        "COPY --from=console-e2e-build /src/apps/console/build/admin-bundle /"
    ]
    production = text.rsplit("\nFROM ", 1)[1]
    assert "admin-bundle" not in production
    assert "console-e2e" not in production


def test_dockerignore_admits_admin_ui_and_the_suites_but_no_unit_tests() -> None:
    lines = DOCKERIGNORE.read_text(encoding="utf-8").splitlines()
    assert "!packages/admin_ui/lib/" in lines
    assert "!apps/console/" in lines
    assert "apps/console/test/" in lines
    assert not any("integration_test" in line for line in lines if not line.startswith("#"))


# ---- pure helpers of headless/admin_suites.py ---------------------------------------------


def test_flatten_tests_names_like_patrol_and_skips_the_explorer() -> None:
    tree = {
        "type": "group",
        "name": "",
        "skip": False,
        "entries": [
            {"type": "test", "name": "patrol_test_explorer", "entries": [], "skip": False},
            {
                "type": "group",
                "name": "a1_people_test",
                "skip": False,
                "entries": [
                    {
                        "type": "test",
                        "name": "A1 create a person and approve a waiting device",
                        "entries": [],
                        "skip": False,
                    }
                ],
            },
            {
                "type": "group",
                "name": "a2",
                "skip": True,
                "entries": [
                    {
                        "type": "group",
                        "name": "inner",
                        "skip": False,
                        "entries": [{"type": "test", "name": "t", "entries": [], "skip": False}],
                    }
                ],
            },
        ],
    }
    assert admin.flatten_tests(tree) == [
        admin.PatrolTest("a1_people_test A1 create a person and approve a waiting device", False),
        admin.PatrolTest("a2 inner t", True),
    ]


def test_proof_claims_follow_the_rules_of_the_server() -> None:
    claims = admin.proof_claims(
        "POST", "https://localhost:8443/api/v1/me", nonce="", access_token="", now=7, jti="j"
    )
    assert claims == {
        "jti": "j",
        "htm": "POST",
        "htu": "https://localhost:8443/api/v1/me",
        "iat": 7,
    }
    with_token = admin.proof_claims(
        "GET", "https://localhost:8443/api/v1/me", nonce="n1", access_token="tok", now=7, jti="j"
    )
    assert with_token["nonce"] == "n1"
    want = base64.urlsafe_b64encode(hashlib.sha256(b"tok").digest()).rstrip(b"=").decode()
    assert with_token["ath"] == want
    assert "=" not in with_token["ath"]


def test_request_htu_drops_the_query_and_keeps_the_public_origin() -> None:
    assert (
        admin.request_htu("https://localhost:8443/", "/api/v1/devices/x?a=1#f")
        == "https://localhost:8443/api/v1/devices/x"
    )
    with pytest.raises(ValueError, match="absolute"):
        admin.request_htu("https://localhost:8443", "https://other.example.com/api/v1/me")
    with pytest.raises(ValueError, match="absolute"):
        admin.request_htu("https://localhost:8443", "api/v1/me")


def test_proof_header_carries_only_the_public_key() -> None:
    jwk = {"kty": "EC", "crv": "P-256", "x": "X", "y": "Y", "d": "secret", "ext": True}
    assert admin.proof_header(jwk) == {
        "typ": "dpop+jwt",
        "alg": "ES256",
        "jwk": {"kty": "EC", "crv": "P-256", "x": "X", "y": "Y"},
    }


def test_uuid7_has_version_and_variant() -> None:
    u = uuid.UUID(admin.uuid7(now_ms=0x0123456789AB))
    assert u.version == 7
    assert u.variant == uuid.RFC_4122
    assert u.hex.startswith("0123456789ab")
    assert admin.uuid7() != admin.uuid7()


def test_totp_secret_reads_the_otpauth_uri() -> None:
    uri = "otpauth://totp/Team%20Chat:sam?secret=jbswy3dpehpk3pxp%3D%3D&issuer=Team%20Chat"
    assert admin.totp_secret(uri) == "JBSWY3DPEHPK3PXP"
    with pytest.raises(admin.CheckError):
        admin.totp_secret("otpauth://totp/Team%20Chat:sam?issuer=Team%20Chat")


def _seed() -> dict[str, object]:
    obj = {"id": "00000000-0000-7000-8000-000000000000", "name": "n"}
    return {
        "owner": dict(obj, name="Alex"),
        "sam": dict(obj, name="Sam"),
        "pendingDevice": dict(obj, name="Sam laptop", code="123456"),
        "spaces": {k: dict(obj) for k in ("support", "marketing", "engineering")},
        "channels": {
            k: dict(obj)
            for k in (
                "supportGeneral",
                "supportRequests",
                "marketingCampaigns",
                "marketingInternal",
                "engineeringBuilds",
            )
        },
        "agents": {"supportAgent": dict(obj), "texter": dict(obj)},
        "group": dict(obj, name="Support Team"),
    }


def test_check_seed_accepts_the_shape_the_dart_tests_read() -> None:
    seed = _seed()
    admin.check_seed(seed)
    assert json.loads(json.dumps(seed)) == seed


@pytest.mark.parametrize(
    ("path", "value"),
    [
        (("extra",), {"id": "x", "name": "y"}),
        (("spaces", "support"), {"id": "x"}),
        (("channels", "supportGeneral"), {"id": "", "name": "general"}),
        (("pendingDevice", "code"), "12a4"),
        (("agents", "other"), {"id": "x", "name": "y"}),
    ],
)
def test_check_seed_rejects_a_wrong_shape(path: tuple[str, ...], value: object) -> None:
    seed = _seed()
    target = seed
    for key in path[:-1]:
        target = target[key]  # type: ignore[assignment]
    target[path[-1]] = value  # type: ignore[index]
    with pytest.raises(admin.CheckError):
        admin.check_seed(seed)


def test_named_takes_the_display_name_where_asked() -> None:
    assert admin.named({"id": 1, "name": "Support"}) == {"id": "1", "name": "Support"}
    assert admin.named({"id": "a", "display_name": "Sam"}, "display_name") == {
        "id": "a",
        "name": "Sam",
    }


def test_csv_export_seen() -> None:
    rec = admin.DownloadRecord
    assert admin.csv_export_seen([rec("audit.csv", "occurred_at,actor,action")])
    assert admin.csv_export_seen([rec("x.txt", ""), rec("Audit.CSV", "id,occurred_at")])
    assert not admin.csv_export_seen([rec("audit.csv", "id,actor")])
    assert not admin.csv_export_seen([rec("audit.json", "occurred_at")])
    assert not admin.csv_export_seen([])


def test_problem_code() -> None:
    assert admin.problem_code({"json": {"code": "step_up_required"}}) == "step_up_required"
    assert admin.problem_code({"json": None}) == ""
    assert admin.problem_code({"json": ["x"]}) == ""


class _FakeContext:
    def __init__(self) -> None:
        self.granted: list[tuple[list[str], str]] = []

    def grant_permissions(self, permissions: list[str], origin: str) -> None:
        self.granted.append((permissions, origin))


class _FakePage:
    def __init__(self) -> None:
        self.context = _FakeContext()
        self.evaluated: list[tuple[str, object]] = []
        self.media: list[str] = []
        self.waited: list[tuple[str, str, float]] = []
        self.files: list[tuple[str, object]] = []

    def wait_for_selector(self, selector: str, *, state: str, timeout: float) -> None:
        self.waited.append((selector, state, timeout))

    def set_input_files(self, selector: str, files: object) -> None:
        self.files.append((selector, files))

    def evaluate(self, script: str, arg: object = None) -> object:
        self.evaluated.append((script, arg))
        return "clip"

    def emulate_media(self, color_scheme: str) -> None:
        self.media.append(color_scheme)


def test_platform_handler_answers_lifecycle_and_rejects_unknown_actions() -> None:
    page = _FakePage()
    lines: list[str] = []
    handler = admin.PlatformHandler(page, lines.append)  # type: ignore[arg-type]
    handler.downloads.append("old.csv")
    assert handler(None, {"action": "startTest", "params": {}}) is None
    assert handler.downloads == []
    handler.downloads.append("audit.csv")
    assert handler(None, {"action": "verifyFileDownloads", "params": {}}) == ["audit.csv"]
    assert handler(None, {"action": "getClipboard", "params": {}}) == "clip"
    assert handler(None, {"action": "setClipboard", "params": {"text": "t"}}) is None
    assert page.evaluated[-1][1] == "t"
    assert handler(None, {"action": "grantPermissions", "params": {"permissions": ["a"]}}) is None
    assert page.context.granted == [(["a"], admin.ORIGIN)]
    assert handler(None, {"action": "enableDarkMode", "params": {}}) is None
    assert page.media == ["dark"]
    with pytest.raises(admin.CheckError, match="tap"):
        handler(None, {"action": "tap", "params": {}})
    assert handler.unknown == ["tap"]
    assert lines[0] == "platform action startTest"


class _NoPage:
    """A page every call of which fails: the binding runs inside a pending page.evaluate, where a
    nested call of the sync API never returns (it hung phase 2 until the job timeout)."""

    def __getattr__(self, name: str) -> object:
        raise AssertionError(f"the platform handler called page.{name}")


def test_platform_handler_answers_the_manifest_file_without_calling_the_page() -> None:
    handler = admin.PlatformHandler(_NoPage(), lambda _line: None)  # type: ignore[arg-type]
    params = {"name": "repo-bridge.json"}
    answer = handler(None, {"action": "e2eSetManifestFile", "params": params})
    assert answer == admin.manifest_file("repo-bridge.json")
    assert handler.unknown == []
    with pytest.raises(admin.CheckError, match=r"other\.json"):
        handler(None, {"action": "e2eSetManifestFile", "params": {"name": "other.json"}})


class _ListenPage:
    def __init__(self) -> None:
        self.listeners: dict[str, Callable[[object], None]] = {}

    def on(self, event: str, listener: Callable[[object], None]) -> None:
        self.listeners[event] = listener


def test_file_choosers_are_intercepted_so_the_console_input_stays() -> None:
    # Headless Chromium fires "cancel" at once on a file input's click() unless the page
    # intercepts file choosers; the console then removes its input before the suite sets the file.
    page = _ListenPage()
    admin.intercept_file_choosers(page)  # type: ignore[arg-type]
    listener = page.listeners["filechooser"]
    # The listener runs inside the pending page.evaluate of a test: it must not call anything.
    listener(_NoPage())


def test_manifest_file_is_the_repo_bridge_manifest_as_json() -> None:
    payload = admin.manifest_file("repo-bridge.json")
    assert payload["name"] == "repo-bridge.json"
    assert payload["mimeType"] == "application/json"
    assert json.loads(payload["text"]) == admin.repo_bridge_manifest()
    with pytest.raises(admin.CheckError, match="no manifest"):
        admin.manifest_file("../etc/passwd")


def test_repo_bridge_manifest_points_at_the_receiver() -> None:
    m = admin.repo_bridge_manifest()
    assert m["schema_version"] == 1
    assert m["slug"] == "repo-bridge"
    assert m["name"] == "Repo Bridge"
    assert m["min_api_version"] == "0.2"
    assert m["scopes"] == {
        "required": ["messages:write"],
        "optional": ["messages:read", "events:subscribe"],
    }
    assert m["events"] == ["message.created"]
    assert m["delivery"] == {
        "mode": "webhook",
        "events_url": "https://webhook-receiver:8443/hooks/events",
    }
    assert m["requested_grants"] == [
        {
            "target_kind": "channel",
            "level": "write",
            "capabilities": [],
            "purpose": "Post build results",
        }
    ]
    assert m["data_egress"]
    # White label: example texts only, no other host than the receiver and example.com.
    hosts = set(re.findall(r"https://([^/:\"]+)", json.dumps(m)))
    assert hosts <= {"webhook-receiver", "bridge.example.com"}
    # Commands or components would need an interactions_url (11 3.2); the flow has none.
    assert "commands" not in m
    assert "components" not in m


def test_judge_fails_on_every_kind_of_failure() -> None:
    ok = [{"name": "a", "result": "success"}]
    csv = [admin.DownloadRecord("audit.csv", "occurred_at,action")]
    admin.judge(ok, [], csv)
    with pytest.raises(admin.CheckError, match="no patrol test"):
        admin.judge([], [], csv)
    with pytest.raises(admin.CheckError, match="tap"):
        admin.judge(ok, ["tap"], csv)
    with pytest.raises(admin.CheckError, match="b"):
        admin.judge([*ok, {"name": "b", "result": "skipped"}], [], csv)
    with pytest.raises(admin.CheckError, match="CSV"):
        admin.judge(ok, [], [])


def test_problem_text_names_code_and_reason_only() -> None:
    answer = {"json": {"code": "conflict", "reason": "billing_mode_not_personal", "detail": "x"}}
    assert admin.problem_text(answer) == "conflict billing_mode_not_personal"
    assert admin.problem_text({"json": {"code": "forbidden"}}) == "forbidden"
    assert admin.problem_text({"json": None}) == ""


def test_save_results_copies_the_json_files_and_the_screenshots(tmp_path: Path) -> None:
    results = tmp_path / "results"
    results.mkdir()
    for name in ("admin.json", "admin-01-failure.png", "other.txt"):
        (results / name).write_text("x", encoding="utf-8")
    e2e.save_results(results, str(tmp_path / "artifacts"))
    assert {p.name for p in (tmp_path / "artifacts").iterdir()} == {
        "admin.json",
        "admin-01-failure.png",
    }


def test_console_log_keeps_test_output_out_of_stderr(capsys: pytest.CaptureFixture[str]) -> None:
    class Msg:
        def __init__(self, text: str) -> None:
            self.text = text
            self.type = "log"

    lines: list[str] = []
    listen = admin.console_log(lines)
    listen(Msg('PATROL_LOG {"type":"test"}'))  # type: ignore[arg-type]
    for i in range(admin.CONSOLE_KEEP + 5):
        listen(Msg(f"line {i} " + "x" * 600))  # type: ignore[arg-type]
    err = capsys.readouterr().err
    assert err == 'PATROL_LOG {"type":"test"}\n'
    assert len(lines) == admin.CONSOLE_KEEP
    assert lines[0].startswith("log: line 5 ")
    assert all(len(line) <= admin.CONSOLE_LINE_MAX for line in lines)


# ---- phase 3: the terminal (brief M6-18 Design notes 5 and 6) -----------------------------


def _phase3(fake: FakeDocker) -> list[tuple[list[str], dict[str, str]]]:
    return [(c, e) for c, e in _compose_calls(fake) if str(TERMINAL_OVERLAY) in c]


def _index(calls: list[list[str]], pred) -> int:
    return next(i for i, c in enumerate(calls) if pred(c))


def test_phase_three_follows_phase_two_with_the_terminal_overlay(environ: dict[str, str]) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    calls = _compose_calls(fake)
    last_admin = max(i for i, (c, _) in enumerate(calls) if str(ADMIN_OVERLAY) in c)
    terminal = [i for i, (c, _) in enumerate(calls) if str(TERMINAL_OVERLAY) in c]
    assert terminal
    assert min(terminal) > last_admin
    # Phase 3 keeps the files of phase 1, drops the admin overlay and appends its own last.
    first = calls[terminal[0]][0]
    files = [first[i + 1] for i, a in enumerate(first) if a == "--file"]
    assert files == [str(f) for f in e2e.Stack(project="x", env={}).files] + [str(TERMINAL_OVERLAY)]
    phase3 = [c for c, _ in _phase3(fake)]
    ups = [c for c in phase3 if "up" in c]
    assert [u[-1] for u in ups] == ["api", "tls", "console-host"]
    assert "--wait" in ups[0]
    assert "--force-recreate" in ups[0]
    assert "--force-recreate" in ups[1]
    runs = [(c, e) for c, e in _phase3(fake) if "run" in c]
    assert len(runs) == 1
    run, env = runs[0]
    assert run[-1] == "console-browser"
    assert "--detach" in run
    assert env["E2E_MODE"] == "terminal"
    assert env["E2E_HOST_IMAGE"] == e2e.host_image_tag()
    assert env["E2E_TERMINAL_BUNDLE"].endswith("/" + e2e.TERMINAL_BUNDLE)
    assert "down" in calls[-1][0]


def test_phase_three_provisions_the_host_before_the_browser_and_pairs_on_request(
    environ: dict[str, str],
) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    calls = fake.calls
    up_host = _index(calls, lambda c: "up" in c and c[-1] == "console-host")
    boot = _index(calls, lambda c: "is-system-running" in c)
    copies = [i for i, c in enumerate(calls) if c[:2] == ["docker", "compose"] and "cp" in c]
    install = _index(calls, lambda c: "install" in c and "--session-user" in c)
    browser = _index(calls, lambda c: "run" in c and "--detach" in c)
    pair = _index(calls, lambda c: "exec" in c and "pair" in c)
    wait = _index(calls, lambda c: c[:2] == ["docker", "wait"])
    assert up_host < boot < min(copies) < install < browser < pair < wait
    targets = sorted(calls[i][-1] for i in copies)
    assert targets == [
        "console-host:/opt/termsuite/bin/bothy-host",
        "console-host:/opt/termsuite/bin/claude",
    ]
    assert calls[install][-4:] == [
        "/opt/termsuite/bin/bothy-host",
        "install",
        "--session-user",
        e2e.SESSION_USER,
    ]
    assert calls[pair][-6:] == [
        "/opt/bothy-host/bin/bothy-host",
        "pair",
        "--server",
        "https://api:8443",
        "--code",
        PAIRING_CODE,
    ]
    # The test CA is trusted inside the host container before it pairs (24 2.2, SEC-TLS-1).
    trust = _index(calls, lambda c: any("update-ca-certificates" in a for a in c))
    assert install > trust > min(copies)
    # The request is gone before the pairing runs; the browser gets the printed fingerprint.
    assert fake.seen_pairing_at_pair is False
    assert fake.paired == {"fingerprint": FINGERPRINT}
    name = calls[browser][calls[browser].index("--name") + 1]
    assert calls[wait][-1] == name
    assert ["docker", "logs", name] in calls
    assert ["docker", "rm", "--force", name] in calls


def test_pairing_code_is_never_printed(
    environ: dict[str, str], capsys: pytest.CaptureFixture[str]
) -> None:
    assert e2e.main([], environ, FakeDocker()) == 0
    captured = capsys.readouterr()
    assert PAIRING_CODE not in captured.out + captured.err


def test_host_image_is_built_only_when_missing_and_removed_after(environ: dict[str, str]) -> None:
    tag = e2e.host_image_tag()
    missing = FakeDocker(host_image=False)
    assert e2e.main([], dict(environ), missing) == 0
    assert e2e.host_image_build_args(tag) in missing.calls
    assert ["docker", "image", "rm", tag] in missing.calls
    present = FakeDocker(host_image=True)
    assert e2e.main([], dict(environ), present) == 0
    assert e2e.host_image_build_args(tag) not in present.calls
    assert ["docker", "image", "rm", tag] not in present.calls


def test_phase_two_failure_skips_phase_three(environ: dict[str, str]) -> None:
    fake = FakeDocker(fail_admin="run")
    assert e2e.main([], environ, fake) == 1
    assert not any(str(TERMINAL_OVERLAY) in c for c in fake.calls)
    assert not any("console-e2e-terminal-web" in c for c in fake.calls)


@pytest.mark.parametrize(
    "fake",
    [
        FakeDocker(fail_terminal="up"),
        FakeDocker(fail_terminal="exec"),
        FakeDocker(fail_terminal="cp"),
        FakeDocker(fail_terminal="run"),
        FakeDocker(browser_rc=1),
        FakeDocker(fail_pair=True),
        FakeDocker(browser_exits_early=True, browser_rc=1),
        FakeDocker(result_ok=False),
        FakeDocker(no_result=True),
    ],
    ids=["up", "exec", "cp", "run", "browser", "pair", "early-exit", "result", "no-result"],
)
def test_teardown_after_a_phase_three_failure(
    environ: dict[str, str], tmp_path: Path, fake: FakeDocker
) -> None:
    assert e2e.main([], environ, fake) == 1
    assert "down" in fake.compose_subcommands()
    assert ["docker", "image", "rm", e2e.host_image_tag()] in fake.calls
    assert list(tmp_path.iterdir()) == []
    started = [c for c in fake.calls if "run" in c and "--detach" in c and "console-browser" in c]
    if started and fake.fail_terminal != "run":
        name = started[0][started[0].index("--name") + 1]
        assert ["docker", "rm", "--force", name] in fake.calls


def test_a_browser_that_never_asks_is_not_paired(environ: dict[str, str]) -> None:
    fake = FakeDocker(browser_exits_early=True, browser_rc=1)
    assert e2e.main([], environ, fake) == 1
    assert not any("exec" in c and "pair" in c for c in fake.calls)


def test_diagnosis_names_the_host_service_only_in_phase_three() -> None:
    stack = e2e.Stack(project="x", env={})
    assert "console-host" not in e2e.diagnose_services(stack)
    stack.files.append(e2e.TERMINAL_OVERLAY)
    assert e2e.diagnose_services(stack)[-1] == "console-host"


def test_a_failed_phase_three_prints_the_journal_of_the_host_service(
    environ: dict[str, str],
) -> None:
    fake = FakeDocker(browser_rc=1)
    assert e2e.main([], environ, fake) == 1
    journal = [c for c in fake.calls if "exec" in c and "journalctl" in c]
    assert len(journal) == 1
    assert journal[0][journal[0].index("exec") :][:3] == ["exec", "-T", e2e.HOST_SERVICE]
    assert "bothy-host*" in journal[0]


def test_a_failure_before_phase_three_reads_no_journal(environ: dict[str, str]) -> None:
    fake = FakeDocker(fail_admin="run")
    assert e2e.main([], environ, fake) == 1
    assert not any("journalctl" in c for c in fake.calls)


def test_terminal_bundle_build_command_line(tmp_path: Path) -> None:
    assert e2e.terminal_bundle_build_args(tmp_path, "1.2.3") == [
        "docker",
        "buildx",
        "build",
        "--quiet",
        "--file",
        str(DOCKERFILE),
        "--target",
        "console-e2e-terminal-web",
        "--build-arg",
        "VERSION=1.2.3",
        "--output",
        f"type=local,dest={tmp_path / 'terminal-bundle'}",
        str(ROOT),
    ]


def test_host_image_like_the_terminal_suite() -> None:
    dockerfile = ROOT / "infra" / "compose" / "test" / "host" / "Dockerfile"
    digest = hashlib.sha256(dockerfile.read_bytes()).hexdigest()
    tag = e2e.host_image_tag()
    assert tag == "termsuite-host:" + digest[:12]
    assert e2e.host_image_build_args(tag) == [
        "docker",
        "buildx",
        "build",
        "--quiet",
        "--load",
        "--tag",
        tag,
        "--file",
        str(dockerfile),
        str(dockerfile.parent),
    ]


def test_host_binaries_command_line(tmp_path: Path) -> None:
    assert e2e.host_bin_build_args(tmp_path) == [
        "docker",
        "buildx",
        "build",
        "--quiet",
        "--file",
        str(ROOT / "tools" / "console-e2e" / "host-bin.Dockerfile"),
        "--output",
        f"type=local,dest={tmp_path / 'host-bin'}",
        str(ROOT / "services" / "host"),
    ]


def test_host_bin_dockerfile_uses_the_go_of_the_server_image() -> None:
    text = (ROOT / "tools" / "console-e2e" / "host-bin.Dockerfile").read_text(encoding="utf-8")
    assert text.startswith("# SPDX-License-Identifier: Apache-2.0\n")
    assert "TEST ONLY" in text
    server = DOCKERFILE.read_text(encoding="utf-8")
    golang = re.findall(r"^FROM --platform=\$BUILDPLATFORM (golang:\S+)", server, re.M)
    stages = _stages(text)
    assert stages[0][0] == golang[0]
    assert stages[-1] == ("scratch", "")
    # The mark of the platform suite (D-M2-48-5), as BuildHostBinaries of M2-48 builds it.
    assert "-X github.com/OWNER/bothy/services/host/internal/buildinfo.PlatformSuite=1" in text
    assert "./cmd/bothy-host" in text
    assert "./testdata/fakeclaude" in text
    assert "-X main.startLog=/var/tmp/termsuite-claude-starts.log" in text
    assert "GOTOOLCHAIN=local" in text
    assert "CGO_ENABLED=0" in text


def test_terminal_overlay_pins() -> None:
    text = TERMINAL_OVERLAY.read_text(encoding="utf-8")
    assert text.startswith("# SPDX-License-Identifier: Apache-2.0\n")
    assert "TEST ONLY" in text
    data = yaml.safe_load(text)
    assert set(data["services"]) == {"api", "console-host", "console-browser"}
    for name, svc in data["services"].items():
        assert "ports" not in svc, name
        assert svc.get("privileged") is not True or name == "console-host", name
    api = data["services"]["api"]
    assert api["environment"] == {"BOTHY_CONSOLE__ASSETS_DIR": "/usr/share/bothy/console-e2e"}
    assert api["volumes"] == [
        "${E2E_TERMINAL_BUNDLE:?set by console_e2e.py}:/usr/share/bothy/console-e2e:ro"
    ]
    assert api["networks"] == ["consolehosts"]
    host = data["services"]["console-host"]
    assert host["image"] == "${E2E_HOST_IMAGE:?set by console_e2e.py}"
    assert host["pull_policy"] == "never"
    assert host["privileged"] is True
    assert host["user"] == "0:0"
    assert host["networks"] == ["consolehosts"]
    assert host["volumes"] == [
        "${E2E_CERTS:?set by console_e2e.py}/ca.pem"
        ":/usr/local/share/ca-certificates/console-e2e-test-ca.crt:ro"
    ]
    browser = data["services"]["console-browser"]
    assert browser["environment"] == {"E2E_MODE": "${E2E_MODE:-}"}
    assert browser["volumes"] == ["${E2E_CERTS:?set by console_e2e.py}/ca.pem:/e2e-ca/ca.pem:ro"]
    # The host network carries only api (with the sidecar in its namespace) and the host.
    assert data["networks"] == {"consolehosts": {"internal": True}}


def test_terminal_overlay_sets_only_existing_configuration_keys() -> None:
    data = yaml.safe_load(TERMINAL_OVERLAY.read_text(encoding="utf-8"))
    keys = ROOT / "services" / "server" / "internal" / "wire"
    names = "".join(p.read_text(encoding="utf-8") for p in keys.glob("configkeys_*.go"))
    for var in data["services"]["api"]["environment"]:
        key = var.removeprefix("BOTHY_").lower().replace("__", ".")
        assert f'Name: "{key}"' in names, key


def test_stack_env_names_the_terminal_bundle_inside_the_temp_directory(tmp_path: Path) -> None:
    env = e2e.stack_env({"E2E_HOST_IMAGE": "leak:1"}, tmp_path, "img:1", "abcd")
    assert env["E2E_TERMINAL_BUNDLE"] == str(tmp_path / "terminal-bundle")
    assert "E2E_HOST_IMAGE" not in env


def test_runner_and_suite_share_their_names() -> None:
    assert e2e.PAIRING_FILE == term.PAIRING_FILE
    assert e2e.PAIRED_FILE == term.PAIRED_FILE
    assert e2e.TERMINAL_RESULT == term.RESULT_FILE
    assert e2e.TERMINAL_CHECKS == term.CHECKS
    assert e2e.SESSION_USER == term.SESSION_USER


def _dart_int(source: str, name: str) -> int:
    """The value of ``const <name> = <a> [* <b> ...];`` in a Dart source."""
    m = re.search(rf"^const {name} = ([0-9 *]+);$", source, re.MULTILINE)
    assert m, f"no integer constant {name}"
    value = 1
    for factor in m.group(1).split("*"):
        value *= int(factor)
    return value


def test_burst_chunk_fits_the_host_send_queue() -> None:
    """One acknowledged chunk of the burst never fills the host's send queue.

    The queue counts messages, one per PTY read, and a full queue closes the data connection
    (1000; the browser gets 4102, "the host is not connected"). The program of the burst writes
    a chunk through ``fold`` on the PTY, so stdio flushes every line on its own; a PTY read can
    split a write once at most (the line and its CR LF), so a chunk can become two messages per
    line plus its marker writes and the echo of the acknowledgement. That worst case must fit
    the queue, whatever the scheduling of the runner.
    """
    suite = ROOT / "apps" / "console" / "integration_test" / "terminal" / "terminal_test.dart"
    dart = suite.read_text(encoding="utf-8")
    conn = ROOT / "services" / "host" / "internal" / "session" / "dataconn.go"
    host = conn.read_text(encoding="utf-8")
    m = re.search(r"^const sendQueue = (\d+)$", host, re.MULTILINE)
    assert m, "no sendQueue in the host's data connection"
    queue = int(m.group(1))
    lines = -(-_dart_int(dart, "_burstChunkBytes") // _dart_int(dart, "_burstLineChars"))
    worst = 2 * (lines + 2) + 2
    assert worst <= queue, f"a chunk can become {worst} messages, the host queue holds {queue}"


def test_fingerprint_of_reads_the_line_of_pair() -> None:
    assert e2e.fingerprint_of(f"paired\nfingerprint: {FINGERPRINT}\n") == FINGERPRINT
    assert e2e.fingerprint_of("paired\n") == ""
    assert e2e.fingerprint_of("my fingerprint: X\n") == ""


def test_desktop_targets_come_from_the_configuration_reference() -> None:
    doc = (ROOT / "docs" / "operations" / "configuration.md").read_text(encoding="utf-8")
    targets = e2e.desktop_targets(doc)
    assert set(targets) == set(e2e.PERF_KEYS)
    p95 = targets["client.perf.terminal_frame_to_paint_p95"]
    rate = targets["client.perf.terminal_throughput_min"]
    assert p95["ms"] == e2e.parse_duration_ms(p95["value"])
    assert rate["bytes_per_s"] == e2e.parse_byte_rate(rate["value"])
    with pytest.raises(e2e.StepError, match="terminal_throughput_min"):
        e2e.desktop_targets(doc.replace("client.perf.terminal_throughput_min", "x"))


@pytest.mark.parametrize(
    ("text", "ms"), [("16ms", 16.0), ("1s", 1000.0), ("250us", 0.25), ("1.5ms", 1.5)]
)
def test_parse_duration_ms(text: str, ms: float) -> None:
    assert e2e.parse_duration_ms(text) == ms


@pytest.mark.parametrize(
    ("text", "rate"),
    [("1MiB/s", 1_048_576.0), ("512KiB/s", 524_288.0), ("2MB/s", 2_000_000.0), ("10B/s", 10.0)],
)
def test_parse_byte_rate(text: str, rate: float) -> None:
    assert e2e.parse_byte_rate(text) == rate


@pytest.mark.parametrize("text", ["fast", "16", "1MiB", "-1ms"])
def test_parse_rejects_other_forms(text: str) -> None:
    with pytest.raises(e2e.StepError):
        e2e.parse_duration_ms(text)
    with pytest.raises(e2e.StepError):
        e2e.parse_byte_rate(text)


def test_locked_versions_read_the_workspace_lock() -> None:
    lock = (ROOT / "pubspec.lock").read_text(encoding="utf-8")
    versions = e2e.locked_versions(lock, e2e.VERSIONED_PACKAGES)
    data = yaml.safe_load(lock)
    assert versions == {n: data["packages"][n]["version"] for n in ("xterm", "cryptography")}
    with pytest.raises(e2e.StepError, match="nopackage"):
        e2e.locked_versions(lock, ["nopackage"])


def _targets() -> dict[str, dict[str, object]]:
    return {
        "client.perf.terminal_frame_to_paint_p95": {"value": "16ms", "ms": 16.0},
        "client.perf.terminal_throughput_min": {"value": "1MiB/s", "bytes_per_s": 1_048_576.0},
    }


def test_finish_terminal_result_records_targets_and_versions(tmp_path: Path) -> None:
    path = tmp_path / "terminal.json"
    path.write_text(json.dumps(terminal_result()), encoding="utf-8")
    doc = e2e.finish_terminal_result(path, _targets(), {"xterm": "4.0.0", "cryptography": "2.9.0"})
    assert json.loads(path.read_text(encoding="utf-8")) == doc
    assert doc["desktop_targets"] == _targets()
    assert doc["versions"] == {"xterm": "4.0.0", "cryptography": "2.9.0"}
    # A measurement, not a gate: the comparison is recorded, the result stays ok either way.
    assert doc["within_desktop_targets"] == {
        "client.perf.terminal_frame_to_paint_p95": True,
        "client.perf.terminal_throughput_min": True,
    }
    slow = terminal_result()
    slow["measurement"] = {"frame_to_paint_p95_ms": 40.0, "throughput_bytes_per_s": 10.0}
    path.write_text(json.dumps(slow), encoding="utf-8")
    doc = e2e.finish_terminal_result(path, _targets(), {})
    assert doc["within_desktop_targets"] == {
        "client.perf.terminal_frame_to_paint_p95": False,
        "client.perf.terminal_throughput_min": False,
    }


@pytest.mark.parametrize("case", ["missing", "garbage", "not-ok", "check-missing", "check-failed"])
def test_finish_terminal_result_rejects_a_failed_run(tmp_path: Path, case: str) -> None:
    path = tmp_path / "terminal.json"
    doc = terminal_result()
    if case == "not-ok":
        doc["ok"] = False
    if case == "check-missing":
        del doc["checks"][term.CHECKS[0]]  # type: ignore[attr-defined]
    if case == "check-failed":
        doc["checks"][term.CHECKS[-1]] = {"ok": False}  # type: ignore[index]
    if case == "garbage":
        path.write_text("{", encoding="utf-8")
    elif case != "missing":
        path.write_text(json.dumps(doc), encoding="utf-8")
    with pytest.raises(e2e.StepError):
        e2e.finish_terminal_result(path, _targets(), {})


# ---- pure helpers of headless/terminal_suites.py ------------------------------------------


def test_checks_name_the_items_of_the_brief() -> None:
    assert [c.split("_", 1)[0] for c in term.CHECKS] == list("abcdefgh")


def test_p95_is_the_nearest_rank() -> None:
    assert term.p95([float(v) for v in range(1, 101)]) == 95.0
    assert term.p95([3.0]) == 3.0
    assert term.p95([5.0, 1.0, 9.0, 2.0]) == 9.0
    with pytest.raises(term.CheckError):
        term.p95([])


def _metrics(marks: int = 60) -> dict[str, object]:
    return {
        "marks_expected": 60,
        "write_to_paint_ms": [float(i % 10) for i in range(marks)],
        "host_to_paint_ms": [float(10 + i % 10) for i in range(marks)],
        "throughput": {"bytes": 2_000_000, "host_start_ms": 1000.0, "paint_end_ms": 2000.0},
    }


def test_measurement_reduces_the_samples() -> None:
    m = term.measurement(_metrics())
    assert m["frame_to_paint_p95_ms"] == 9.0
    assert m["host_to_paint_p95_ms"] == 19.0
    assert m["throughput_bytes_per_s"] == 2_000_000.0
    assert m["samples"] == 60
    assert m["throughput_bytes"] == 2_000_000


@pytest.mark.parametrize(
    "broken",
    [
        {"write_to_paint_ms": []},
        {"throughput": {"bytes": 1, "host_start_ms": 5.0, "paint_end_ms": 5.0}},
        {"throughput": None},
        {"marks_expected": 600},
    ],
)
def test_measurement_refuses_too_little_data(broken: dict[str, object]) -> None:
    metrics = _metrics()
    metrics.update(broken)
    with pytest.raises(term.CheckError):
        term.measurement(metrics)


def test_parse_metrics_line() -> None:
    line = term.METRICS_PREFIX + json.dumps({"opened": True})
    assert term.parse_metrics_line(line) == {"opened": True}
    assert term.parse_metrics_line("PATROL_LOG {}") is None
    assert term.parse_metrics_line(term.METRICS_PREFIX + "{") is None
    assert term.parse_metrics_line(term.METRICS_PREFIX + "[1]") is None


def _answer(status: int, code: str = "", reason: str = "") -> dict[str, object]:
    body = {"code": code, "reason": reason} if code else None
    return {"status": status, "json": body}


def test_passkey_required_accepts_only_the_console_rule() -> None:
    term.expect_passkey_required(_answer(401, "step_up_required", "passkey_required"), "open")
    for wrong in (
        _answer(201),
        _answer(401, "step_up_required", "terminal_step_up_not_fresh"),
        _answer(403, "step_up_required", "passkey_required"),
        _answer(401, "unauthenticated"),
    ):
        with pytest.raises(term.CheckError, match="open"):
            term.expect_passkey_required(wrong, "open")


def test_check_terminal_seed() -> None:
    seed = {
        "owner": {"id": "o", "name": "Alex"},
        "host": {"id": "h", "name": "Build host"},
    }
    term.check_terminal_seed(seed)
    for broken in (
        {"owner": seed["owner"]},
        {**seed, "host": {"id": "h", "name": "Other"}},
        {**seed, "host": {"id": "", "name": "Build host"}},
        {**seed, "extra": {}},
    ):
        with pytest.raises(term.CheckError):
            term.check_terminal_seed(broken)


def test_pin_databases_match_the_name_of_the_pin_store() -> None:
    names = [{"name": "bothy-host-pins", "version": 1}, {"name": "other"}, {"version": 2}]
    assert term.pin_databases(names) == ["bothy-host-pins"]
    assert term.pin_databases([{"name": "PINNED"}]) == ["PINNED"]
    assert term.pin_databases([]) == []


def test_upgrade_request_carries_the_foreign_origin_and_the_cookie() -> None:
    raw = term.upgrade_request(
        "/api/v1/terminal/sessions/s/stream",
        host="localhost:8443",
        origin="https://localhost:9443",
        cookie="a=1; b=2",
        key="sample-websocket-key",
    )
    lines = raw.decode().split("\r\n")
    assert lines[0] == "GET /api/v1/terminal/sessions/s/stream HTTP/1.1"
    assert "Host: localhost:8443" in lines
    assert "Origin: https://localhost:9443" in lines
    assert "Cookie: a=1; b=2" in lines
    assert "Upgrade: websocket" in lines
    assert "Connection: Upgrade" in lines
    assert "Sec-WebSocket-Version: 13" in lines
    assert raw.endswith(b"\r\n\r\n")
    with pytest.raises(ValueError, match="line break"):
        term.upgrade_request("/x", host="h", origin="o\r\nX: y", cookie="", key="k")


def test_status_code_of_a_response_head() -> None:
    assert term.status_code(b"HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n") == 403
    assert term.status_code(b"HTTP/1.1 101 Switching Protocols\r\n\r\n") == 101
    assert term.status_code(b"") == 0
    assert term.status_code(b"garbage") == 0


def test_cookie_header_takes_the_cookies_of_the_console_host() -> None:
    cookies = [
        {"name": "bothy_console", "value": "v1", "domain": "localhost"},
        {"name": "other", "value": "v2", "domain": "example.com"},
    ]
    assert term.cookie_header(cookies, "localhost") == "bothy_console=v1"
    assert term.cookie_header([], "localhost") == ""


def test_fingerprint_prefix() -> None:
    assert term.fingerprint_prefix(FINGERPRINT) == "7K2QM9XD"
    with pytest.raises(term.CheckError):
        term.fingerprint_prefix("7K2Q")


def _host(**over: object) -> dict[str, object]:
    host: dict[str, object] = {
        "state": "active",
        "connected": True,
        "reported_session_users": [{"os_user": term.SESSION_USER, "workdirs": []}],
        "profiles": [{"name": "shell", "display_name": "Shell", "kind": "shell"}],
    }
    host.update(over)
    return host


def test_host_ready() -> None:
    assert term.host_ready(_host())
    assert not term.host_ready(_host(state="confirming"))
    assert not term.host_ready(_host(connected=False))
    assert not term.host_ready(_host(reported_session_users=[]))
    assert not term.host_ready(_host(profiles=[{"name": "claude"}]))
    assert not term.host_ready({})


def test_terminal_op_classifies_the_calls_of_the_terminal() -> None:
    base = "https://localhost:8443/api/v1"
    assert term.terminal_op("POST", base + "/hosts/h1/terminal/sessions") == "open"
    assert term.terminal_op("POST", base + "/terminal/sessions/s1/attach") == "attach"
    assert term.terminal_op("POST", base + "/terminal/sessions/s1/close") == "close"
    assert term.terminal_op("GET", base + "/hosts/h1/terminal/sessions") is None
    assert term.terminal_op("POST", base + "/hosts") is None


def test_session_ids_of_a_page() -> None:
    page = {"items": [{"id": "s1", "host_id": "h"}, {"id": "s2", "host_id": "x"}]}
    assert term.session_ids(page, "h") == ["s1"]
    assert term.session_ids(None, "h") == []


def test_key_hash_matches_the_pattern_of_the_contract() -> None:
    pattern = re.compile(r"^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$")
    for _ in range(20):
        assert pattern.match(term.key_hash())


def test_foreign_except_drops_only_the_test_origin() -> None:
    assert term.foreign_except(["https://localhost:9443", "https://cdn.example.com"]) == [
        "https://cdn.example.com"
    ]


def test_failed_checks() -> None:
    checks = {name: {"ok": True} for name in term.CHECKS}
    assert term.failed_checks(checks) == []
    checks[term.CHECKS[1]] = {"ok": False, "error": "x"}
    del checks[term.CHECKS[2]]
    assert term.failed_checks(checks) == [term.CHECKS[1], term.CHECKS[2]]


@pytest.mark.parametrize("what", ["console-e2e-terminal-web", "--load", "host-bin.Dockerfile"])
def test_a_failed_build_of_phase_three_fails_and_tears_down(
    environ: dict[str, str], tmp_path: Path, what: str
) -> None:
    fake = FakeDocker(fail_build_of=what)
    assert e2e.main([], environ, fake) == 1
    assert not any(str(TERMINAL_OVERLAY) in c and "up" in c for c in fake.calls)
    assert "down" in fake.compose_subcommands()
    assert list(tmp_path.iterdir()) == []


def test_boot_is_polled_until_systemd_runs(
    environ: dict[str, str], monkeypatch: pytest.MonkeyPatch
) -> None:
    sleeps: list[float] = []
    monkeypatch.setattr(e2e.time, "sleep", sleeps.append)
    fake = FakeDocker(boot_states=("starting", "initializing", "degraded"))
    assert e2e.main([], environ, fake) == 0
    assert sum("is-system-running" in c for c in fake.calls) == 3
    assert sleeps == [e2e.POLL_S, e2e.POLL_S]


def test_a_host_that_never_boots_fails(
    environ: dict[str, str], monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(e2e, "BOOT_WAIT_S", -1)
    monkeypatch.setattr(e2e.time, "sleep", lambda _s: None)
    fake = FakeDocker(boot_states=("starting",))
    assert e2e.main([], environ, fake) == 1
    assert not any("install" in c and "--session-user" in c for c in fake.calls)


def test_a_seed_that_never_asks_fails_after_the_wait(
    environ: dict[str, str], monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(e2e, "PAIRING_WAIT_S", -1)
    sleeps: list[float] = []
    monkeypatch.setattr(e2e.time, "sleep", sleeps.append)
    fake = FakeDocker()
    fake._ask_for_pairing = lambda: None  # type: ignore[method-assign]
    assert e2e.main([], environ, fake) == 1
    assert sleeps == []
    assert not any("exec" in c and "pair" in c for c in fake.calls)
    started = next(c for c in fake.calls if "run" in c and "--detach" in c)
    assert ["docker", "rm", "--force", started[started.index("--name") + 1]] in fake.calls


def test_a_browser_that_never_ends_fails(environ: dict[str, str]) -> None:
    fake = FakeDocker(wait_timeout=True)
    assert e2e.main([], environ, fake) == 1
    assert any(c[:3] == ["docker", "rm", "--force"] for c in fake.calls)


def test_a_passing_run_prints_the_measurement(
    environ: dict[str, str], capsys: pytest.CaptureFixture[str]
) -> None:
    assert e2e.main([], environ, FakeDocker()) == 0
    err = capsys.readouterr().err
    line = next(x for x in err.splitlines() if "phase 3: measurement" in x)
    doc = json.loads(line.split("phase 3: measurement ", 1)[1])
    assert doc["measurement"]["frame_to_paint_p95_ms"] == 9.5
    assert doc["measurement"]["throughput_bytes_per_s"] == 3_000_000.0
    assert set(doc["desktop_targets"]) == set(e2e.PERF_KEYS)
    assert set(doc["within_desktop_targets"]) == set(e2e.PERF_KEYS)
    assert set(doc["versions"]) == {"xterm", "cryptography"}


def test_dockerfile_terminal_bundle_stages() -> None:
    """The terminal bundle of phase 3 has its own build and export stages (brief M6-18)."""
    text = DOCKERFILE.read_text(encoding="utf-8")
    stages = _stages(text)
    names = [alias for _, alias in stages]
    assert ("console", "console-e2e-terminal-build") in stages
    assert ("scratch", "console-e2e-terminal-web") in stages
    assert (
        names.index("console-e2e-web")
        < names.index("console-e2e-terminal-build")
        < names.index("console-e2e-terminal-web")
    )
    assert stages[-1][1] == ""
    build = text.split(" AS console-e2e-terminal-build\n", 1)[1].split("\nFROM ", 1)[0]
    assert "--target integration_test/terminal/terminal_bundle.dart" in build
    assert "--base-href /console/ --no-web-resources-cdn --no-pub --no-wasm-dry-run" in build
    web = text.split(" AS console-e2e-terminal-web\n", 1)[1].split("\nFROM ", 1)[0]
    assert re.findall(r"^COPY .*$", web, re.M) == [
        "COPY --from=console-e2e-terminal-build /src/apps/console/build/terminal-bundle /"
    ]
    assert "terminal-bundle" not in text.rsplit("\nFROM ", 1)[1]


# ---- phase 1b: the Grafana embedding behind a test proxy (brief M6-17) --------------------


def test_phase_one_b_runs_between_phase_one_and_two(environ: dict[str, str]) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    calls = _compose_calls(fake)
    report = next(i for i, (c, _) in enumerate(calls) if c[-2:] == ["--no-color", "api"])
    graf = [i for i, (c, _) in enumerate(calls) if str(GRAFANA_OVERLAY) in c]
    admin_calls = [i for i, (c, _) in enumerate(calls) if str(ADMIN_OVERLAY) in c]
    assert graf
    assert report < min(graf) < max(graf) < min(admin_calls)
    # Phase 1b keeps every file of phase 1 and appends its overlay last; phase 2 never has it.
    first = calls[graf[0]][0]
    files = [first[i + 1] for i, a in enumerate(first) if a == "--file"]
    assert files == [str(f) for f in e2e.Stack(project="x", env={}).files] + [str(GRAFANA_OVERLAY)]
    assert not any(str(GRAFANA_OVERLAY) in c for c, _ in calls[min(admin_calls) :])
    phase = [c for c, _ in calls if str(GRAFANA_OVERLAY) in c]
    ups = [c for c in phase if "up" in c]
    assert ups[0][-1] == "api"
    assert "--force-recreate" in ups[0]
    assert "--wait" in ups[0]
    assert ups[1][-2:] == ["tls", "grafana-proxy"]
    assert "--force-recreate" in ups[1]
    checks = [c for c in phase if "exec" in c and c[-3:] == ["bothy", "config", "check"]]
    assert len(checks) == 2
    assert checks[0][checks[0].index("-e") + 1] == (
        f"{e2e.EMBED_KEY_ENV}={e2e.CONSOLE_ORIGIN}/grafana/d/ops"
    )
    assert "-e" not in checks[1]
    runs = [(c, e) for c, e in calls if str(GRAFANA_OVERLAY) in c and "run" in c]
    assert len(runs) == 1
    assert runs[0][1]["E2E_MODE"] == "grafana"
    assert phase.index(ups[1]) < phase.index(checks[0]) < phase.index(runs[0][0])
    logs = [c for c in phase if "logs" in c and c[-1] == "grafana-proxy"]
    assert len(logs) == 1
    assert phase.index(runs[0][0]) < phase.index(logs[0])
    removed = [c for c in phase if "rm" in c]
    assert removed
    assert removed[0][-1] == "grafana-proxy"
    # Phase 2 runs the admin suites, never the Grafana suite.
    assert all(e.get("E2E_MODE") != "grafana" for c, e in calls if str(ADMIN_OVERLAY) in c)


@pytest.mark.parametrize(
    "fake",
    [
        FakeDocker(fail_grafana="run"),
        FakeDocker(fail_grafana="up"),
        FakeDocker(same_origin_check_rc=0),
        FakeDocker(proxy_log=""),
        FakeDocker(proxy_log=proxy_line("localhost:9445", "/d/blocked") + "\n" + PROXY_LOG),
        FakeDocker(
            proxy_log=proxy_line(
                "grafana.localhost:9444",
                "/d/x",
                {"Referer": ["https://localhost:8443/console/grafana"]},
            )
        ),
    ],
    ids=["suite", "up", "config check passes", "no dashboard", "blocked origin", "referer"],
)
def test_a_phase_one_b_failure_fails_skips_phase_two_and_tears_down(
    environ: dict[str, str], tmp_path: Path, fake: FakeDocker
) -> None:
    assert e2e.main([], environ, fake) == 1
    assert not any(str(ADMIN_OVERLAY) in c for c in fake.calls)
    assert "down" in fake.compose_subcommands()
    assert list(tmp_path.iterdir()) == []


def test_diagnosis_names_the_proxy_only_in_phase_one_b() -> None:
    stack = e2e.Stack(project="x", env={})
    assert e2e.GRAFANA_PROXY not in e2e.diagnose_services(stack)
    stack.files.append(e2e.GRAFANA_OVERLAY)
    assert e2e.GRAFANA_PROXY in e2e.diagnose_services(stack)


def test_proxy_requests_read_host_path_headers_and_status() -> None:
    logs = "grafana-proxy-1  | " + proxy_line(
        "GRAFANA.LOCALHOST:9444", "/d/bothy-ops/operations", {"Cookie": ["REDACTED"]}, 200
    )
    logs += "\nnot json\n{}\n"
    assert e2e.proxy_requests(logs) == [
        e2e.ProxyRequest(
            "GET", "grafana.localhost:9444", "/d/bothy-ops/operations", frozenset({"cookie"}), 200
        )
    ]


@pytest.mark.parametrize(
    ("log", "message"),
    [
        ("", "served no dashboard"),
        (proxy_line("grafana.localhost:9444", "/d/x", status=403), "served no dashboard"),
        (
            proxy_line("grafana.localhost:9444", "/d/x", {"Authorization": ["REDACTED"]}),
            "authorization",
        ),
        (proxy_line("grafana.localhost:9444", "/d/x", {"Cookie": ["REDACTED"]}), "cookie"),
        (PROXY_LOG + proxy_line("localhost:9445", "/d/blocked"), "does not name"),
    ],
)
def test_check_proxy_requests_fails(log: str, message: str) -> None:
    with pytest.raises(e2e.StepError, match=message):
        e2e.check_proxy_requests(e2e.proxy_requests(log))


def test_check_proxy_requests_passes_a_clean_load() -> None:
    e2e.check_proxy_requests(e2e.proxy_requests(PROXY_LOG))


def test_grafana_overlay_pins() -> None:
    text = GRAFANA_OVERLAY.read_text(encoding="utf-8")
    assert "TEST ONLY" in text
    data = yaml.safe_load(text)
    assert set(data["services"]) == {"api", "grafana-proxy", "console-browser"}
    embed = f"https://{e2e.EMBED_HOST}/d/bothy-ops/operations"
    assert data["services"]["api"] == {"environment": {e2e.EMBED_KEY_ENV: embed}}
    tls = yaml.safe_load(OVERLAY.read_text(encoding="utf-8"))["services"]["tls"]
    proxy = data["services"]["grafana-proxy"]
    assert proxy["profiles"] == ["console-e2e"]
    for key in (
        "image",
        "network_mode",
        "user",
        "read_only",
        "cap_drop",
        "cap_add",
        "security_opt",
        "command",
    ):
        assert proxy[key] == tls[key], key
    assert "ports" not in proxy
    assert proxy["volumes"] == [
        "./test/console-e2e/grafana.Caddyfile:/etc/caddy-e2e/Caddyfile:ro",
        "./test/console-e2e/grafana-dashboard:/srv/dashboard:ro",
        "${E2E_CERTS:?set by console_e2e.py}:/certs:ro",
    ]
    assert data["services"]["console-browser"]["environment"] == {
        "E2E_MODE": "${E2E_MODE:-}",
        "E2E_EMBED_URL": embed,
        "E2E_BLOCKED_ORIGIN": f"https://{e2e.BLOCKED_HOST}",
    }
    # The key exists in the schema of the server.
    keys = ROOT / "services" / "server" / "internal" / "wire"
    names = "".join(p.read_text(encoding="utf-8") for p in keys.glob("configkeys_*.go"))
    assert '"console.grafana_embed_url"' in names


def test_console_origin_is_the_public_url_of_the_overlay() -> None:
    env = (ROOT / "infra" / "compose" / "test" / "console-e2e.env").read_text(encoding="utf-8")
    assert f"BOTHY_PUBLIC_URL={e2e.CONSOLE_ORIGIN}" in env.splitlines()


def test_grafana_caddyfile_sends_the_headers_of_the_auth_proxy() -> None:
    text = (GRAFANA_OVERLAY.parent / "console-e2e" / "grafana.Caddyfile").read_text(
        encoding="utf-8"
    )
    port = e2e.EMBED_HOST.rsplit(":", 1)[1]
    blocked = e2e.BLOCKED_HOST.rsplit(":", 1)[1]
    assert f"https://:{port}, https://:{blocked} {{" in text
    assert "tls /certs/leaf.pem /certs/leaf.key" in text
    assert f'Content-Security-Policy "frame-ancestors {e2e.CONSOLE_ORIGIN}"' in text
    assert "Cross-Origin-Resource-Policy cross-origin" in text
    assert "Cross-Origin-Embedder-Policy require-corp" in text
    assert re.search(r"log \{\s*output stdout\s*format json\s*\}", text)
    dashboard = GRAFANA_OVERLAY.parent / "console-e2e" / "grafana-dashboard" / "index.html"
    assert f'id="{graf.DASHBOARD_MARKER.lstrip("#")}"' in dashboard.read_text(encoding="utf-8")
    assert "<script" not in dashboard.read_text(encoding="utf-8")


def test_frame_sources_and_check_policy() -> None:
    csp = "default-src 'none'; frame-ancestors 'none'; report-to csp; frame-src https://localhost:9444"
    assert graf.frame_sources(csp) == [["https://localhost:9444"]]
    graf.check_policy(
        {"Content-Security-Policy": csp, "X-Frame-Options": "DENY"}, "https://localhost:9444"
    )
    bad = [
        (
            {
                "Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
                "X-Frame-Options": "DENY",
            },
            "frame-src",
        ),
        (
            {
                "Content-Security-Policy": csp + "; frame-src https://localhost:9445",
                "X-Frame-Options": "DENY",
            },
            "frame-src",
        ),
        (
            {
                "Content-Security-Policy": csp.replace(
                    "frame-src https://localhost:9444",
                    "frame-src https://localhost:9444 https://localhost:9445",
                ),
                "X-Frame-Options": "DENY",
            },
            "frame-src",
        ),
        (
            {
                "Content-Security-Policy": csp.replace(
                    "frame-ancestors 'none'", "frame-ancestors *"
                ),
                "X-Frame-Options": "DENY",
            },
            "frame-ancestors",
        ),
        ({"Content-Security-Policy": csp}, "X-Frame-Options"),
    ]
    for headers, message in bad:
        with pytest.raises(graf.CheckError, match=message):
            graf.check_policy(headers, "https://localhost:9444")


def test_foreign_requests_allow_the_embed_origin_only_from_the_frame() -> None:
    console, embed = "https://localhost:8443", "https://grafana.localhost:9444"
    seen = [
        graf.Seen(console, False),
        graf.Seen(embed, True),
        graf.Seen(console, True),
    ]
    assert graf.foreign_requests(seen, console, embed) == []
    seen += [graf.Seen(embed, False), graf.Seen("https://cdn.example.com", True)]
    assert graf.foreign_requests(seen, console, embed) == [
        "https://cdn.example.com (frame)",
        "https://grafana.localhost:9444 (main frame)",
    ]


def test_origin_of() -> None:
    assert (
        graf.origin_of("https://Grafana.LocalHost:9444/d/x?y=1#z")
        == "https://grafana.localhost:9444"
    )


def test_phase_one_b_creates_no_person_a_later_phase_creates() -> None:
    """Every phase shares one database: a handle of phase 1b in a later phase is a 409 there.

    The later phases create people in the seed of admin_suites.py and in the patrol suites of
    apps/console/integration_test (A1 types the handle into the form).
    """
    handle = graf.NON_OWNER_HANDLE
    later = [ROOT / "tools" / "console-e2e" / "headless" / "admin_suites.py"]
    later += sorted((ROOT / "apps" / "console" / "integration_test").rglob("*.dart"))
    created: set[str] = set()
    for path in later:
        text = path.read_text(encoding="utf-8")
        created |= set(re.findall(r'"handle": "([^"]+)"', text))
        created |= set(re.findall(r"'createPerson\.handle', '([^']+)'", text))
    assert {"sam", "robin"} <= created  # the extraction still finds the people of phase 2
    assert handle.lower() not in {h.lower() for h in created}
