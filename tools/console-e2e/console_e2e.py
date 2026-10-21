# SPDX-License-Identifier: Apache-2.0
"""Runner of the console E2E test SEC-T05 (brief M6-03, ``make console-e2e``).

Starts the Compose smoke stack of M1-73 with the console overlay
``infra/compose/test/compose.console-e2e.yaml`` from the real server image, bootstraps the owner,
gives it a password with TOTP (``bothy-smoke seed``), starts the TLS sidecar with a certificate of
a test CA generated for this run, and runs the headless browsers of
``tools/console-e2e/headless/console_headless.py`` in the pinned Playwright image inside the network
namespace of ``api``. Afterwards it checks that the violation report of the negative test reached
the server log, and always tears the stack down, also after a failure.

Phase 2 (brief M6-04 Design note 4) follows a passed phase 1 on the same stack: ``api`` is
recreated with the overlay ``compose.console-e2e-admin.yaml``, which serves the web bundle of the
patrol suites of the console administration (built from the target ``console-e2e-web`` of
``services/server/Dockerfile`` into the temp directory) as the console and holds a step-up for 3 s;
then the TLS sidecar and the browser container with ``E2E_MODE=admin`` (headless/admin_suites.py).
Phase 2 also starts the worker, which delivers the outgoing webhooks, and the receiver
``webhook-receiver`` of brief M6-08 on the Compose network (HTTPS with a certificate of the test
CA, every POST answered 200 and logged); api and the worker trust the test CA through a bundle of
the system roots of the server image and the test CA (``SSL_CERT_FILE``). After the suites the
receiver must have logged a delivery with a ``webhook-id`` header.

Phase 3 (brief M6-18 Design notes 5 and 6) follows a passed phase 2 on the same stack: ``api`` is
recreated with the overlay ``compose.console-e2e-terminal.yaml`` in place of the admin overlay; it
serves the bundle of the terminal patrol test (target ``console-e2e-terminal-web``) and joins the
internal network of the host service container ``console-host``, the host image of the terminal
suite of M2-48 (``termsuite-host:<hash of infra/compose/test/host/Dockerfile>``, built when the tag
is missing) with ``bothy-host`` and the Fake-claude built from ``services/host``
(``host-bin.Dockerfile``). The runner provisions and installs the host, starts the browser
container with ``E2E_MODE=terminal`` (headless/terminal_suites.py) detached, pairs the host with
``bothy-host pair --server https://api:8443`` when the seed asks for it, and checks
``out/results/terminal.json``, to which it adds the desktop targets of the measurement and the
versions of ``xterm`` and ``cryptography``.

Exit codes: 0 pass, 1 a check failed, 2 a precondition is missing (Docker, Compose, the image).
Each step is announced as ``console-e2e: <step>`` on stderr; no secret is ever printed.

Example::

    uv run --project tools python tools/console-e2e/console_e2e.py
"""

from __future__ import annotations

import base64
import datetime
import hashlib
import ipaddress
import json
import os
import re
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
from collections.abc import Callable, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import NameOID

ROOT = Path(__file__).resolve().parents[2]
COMPOSE_DIR = ROOT / "infra" / "compose"
MIN_ENGINE = (26, 0, 0)
MIN_COMPOSE = (2, 26, 0)
# The log message of the server for an accepted report (instance/app.LogCSPViolation).
REPORT_LOG = "csp violation reported"
# The services whose last log lines a failure prints.
DIAGNOSE_SERVICES = ("api", "worker", "migrate", "tls", "webhook-receiver")
# The overlay of phase 2 (brief M6-04 Design note 4) and the directory of the bundle in the temp
# directory of the run.
ADMIN_OVERLAY = COMPOSE_DIR / "test" / "compose.console-e2e-admin.yaml"
ADMIN_BUNDLE = "admin-bundle"
# The receiver of the outgoing webhooks of phase 2 (brief M6-08): its Compose service name, which
# the leaf certificate also names, and the trust bundle of api and the worker in the certs
# directory (the overlay mounts it as SSL_CERT_FILE).
RECEIVER = "webhook-receiver"
TRUST_BUNDLE = "ca-bundle.pem"
# The CA file of the distroless server image (Debian's ca-certificates).
IMAGE_ROOTS = "/etc/ssl/certs/ca-certificates.crt"
PEM_CERT = b"-----BEGIN CERTIFICATE-----"
# Phase 3 (brief M6-18 Design note 5): the overlay, the directories of the terminal bundle and of
# the host binaries in the temp directory, the host image of the terminal suite of M2-48 and the
# Dockerfile of the host binaries.
TERMINAL_OVERLAY = COMPOSE_DIR / "test" / "compose.console-e2e-terminal.yaml"
TERMINAL_BUNDLE = "terminal-bundle"
HOST_BIN = "host-bin"
HOST_DOCKERFILE = COMPOSE_DIR / "test" / "host" / "Dockerfile"
HOST_BIN_DOCKERFILE = ROOT / "tools" / "console-e2e" / "host-bin.Dockerfile"
# The service of the host container, its session user (one per person, ruling T12) and the URL it
# pairs with: the TLS sidecar under the name api, which the certificate of the test CA names.
HOST_SERVICE = "console-host"
SESSION_USER = "bothy-s-alex"
HOST_SERVER_URL = "https://api:8443"
# Where the binaries go before bothy-host install, and where install puts the host service
# (M2-48's stack.HostBinDir and the installed path), and the start log of the Fake-claude
# (stack.ClaudeStartLog); paths inside the host container, not on this machine.
HOST_BIN_DIR = "/opt/termsuite/bin"
HOST_SERVICE_BIN = "/opt/bothy-host/bin/bothy-host"
CLAUDE_START_LOG = "/var/tmp/termsuite-claude-starts.log"  # noqa: S108  # nosec B108
# The files of the hand-over between the seed in the browser container and the runner (in out/,
# which both mount): the request with the one-time pairing code (mode 0600, removed as soon as it
# is read) and the answer with the fingerprint bothy-host pair printed.
PAIRING_FILE = "terminal-pairing.json"
PAIRED_FILE = "terminal-paired.json"
TERMINAL_RESULT = "terminal.json"
# The checks of headless/terminal_suites.py (a to h of the brief); each must pass.
TERMINAL_CHECKS = (
    "a_open_with_uv_passkey",
    "b_password_totp_refused",
    "c_passkey_without_uv_refused",
    "d_no_quick_reattach",
    "e_foreign_origin_upgrade",
    "f_logout_clears_pins",
    "g_csp_with_open_terminal",
    "h_measurement",
)
# The desktop targets the measurement is recorded next to (docs/spec/15 22.5, M4-38), read from
# the configuration reference, and the packages whose versions the result names (24 12).
PERF_KEYS = ("client.perf.terminal_frame_to_paint_p95", "client.perf.terminal_throughput_min")
CONFIG_REFERENCE = ROOT / "docs" / "operations" / "configuration.md"
PUBSPEC_LOCK = ROOT / "pubspec.lock"
VERSIONED_PACKAGES = ("xterm", "cryptography")
# Bounds of the waits of phase 3: systemd of the host container, the request of the seed, the
# browser container (the patrol test, the measurement and the TOTP steps of the logins).
BOOT_WAIT_S = 120
PAIRING_WAIT_S = 900
BROWSER_WAIT_S = 2400
POLL_S = 1.0


class PreconditionError(Exception):
    """A tool or input the run needs is missing (exit code 2)."""


class StepError(Exception):
    """A step of the test failed (exit code 1)."""


def say(message: str) -> None:
    """Writes one progress line to stderr."""
    print(f"console-e2e: {message}", file=sys.stderr, flush=True)


def parse_version(text: str) -> tuple[int, int, int]:
    """Returns the first three numbers of a version such as ``v29.5.2`` or ``5.6.0-desktop``."""
    nums = [int(n) for n in re.findall(r"\d+", text.lstrip("v"))[:3]]
    while len(nums) < 3:
        nums.append(0)
    return nums[0], nums[1], nums[2]


def default_image(root: Path = ROOT) -> str:
    """The tag ``make image`` loads: ``IMAGE_NAME:IMAGE_TAG`` of the Makefile with the version."""
    makefile = (root / "Makefile").read_text(encoding="utf-8")
    name = re.search(r"^IMAGE_NAME \?= (.+)$", makefile, re.M)
    tag = re.search(r"^IMAGE_TAG \?= (.+)$", makefile, re.M)
    if not name or not tag:
        raise PreconditionError("the Makefile names no IMAGE_NAME or IMAGE_TAG")
    version = (root / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    return f"{name.group(1).strip()}:{tag.group(1).strip().replace('$(SERVER_VERSION)', version)}"


@dataclass
class Certs:
    """The test CA and the leaf certificate of the sidecar, and the SPKI pin of the leaf."""

    directory: Path
    spki_sha256: str


def make_certs(directory: Path, now: datetime.datetime | None = None) -> Certs:
    """Writes a test CA and a leaf for ``localhost``, ``api``, the receiver and 127.0.0.1.

    The receiver of phase 2 (``webhook-receiver``) serves the same leaf, so the server's webhook
    client verifies it against the test CA in its trust bundle. The keys are P-256 and exist only
    for this run; the leaf is valid for one day. Chromium trusts the leaf by its SPKI hash
    (``--ignore-certificate-errors-spki-list``), which treats the connection as a valid
    certificate, so WebAuthn and the secure context behave as with a public certificate
    (docs/DECISIONS.md D-M6-03-6).
    """
    now = now or datetime.datetime.now(datetime.UTC)
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    ca_key = ec.generate_private_key(ec.SECP256R1())
    ca_name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "console-e2e test CA")])
    ca = (
        x509.CertificateBuilder()
        .subject_name(ca_name)
        .issuer_name(ca_name)
        .public_key(ca_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - datetime.timedelta(minutes=5))
        .not_valid_after(now + datetime.timedelta(days=1))
        .add_extension(x509.BasicConstraints(ca=True, path_length=0), critical=True)
        .sign(ca_key, hashes.SHA256())
    )
    leaf_key = ec.generate_private_key(ec.SECP256R1())
    leaf = (
        x509.CertificateBuilder()
        .subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "localhost")]))
        .issuer_name(ca_name)
        .public_key(leaf_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - datetime.timedelta(minutes=5))
        .not_valid_after(now + datetime.timedelta(days=1))
        .add_extension(
            x509.SubjectAlternativeName(
                [
                    x509.DNSName("localhost"),
                    x509.DNSName("api"),
                    x509.DNSName(RECEIVER),
                    x509.IPAddress(ipaddress.ip_address("127.0.0.1")),
                ]
            ),
            critical=False,
        )
        .add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True)
        .add_extension(
            x509.ExtendedKeyUsage([x509.oid.ExtendedKeyUsageOID.SERVER_AUTH]), critical=False
        )
        .sign(ca_key, hashes.SHA256())
    )
    pem = serialization.Encoding.PEM
    (directory / "ca.pem").write_bytes(ca.public_bytes(pem))
    (directory / "leaf.pem").write_bytes(leaf.public_bytes(pem) + ca.public_bytes(pem))
    key_file = directory / "leaf.key"
    key_file.write_bytes(
        leaf_key.private_bytes(pem, serialization.PrivateFormat.PKCS8, serialization.NoEncryption())
    )
    key_file.chmod(0o600)
    spki = leaf.public_key().public_bytes(
        serialization.Encoding.DER, serialization.PublicFormat.SubjectPublicKeyInfo
    )
    return Certs(
        directory=directory, spki_sha256=base64.b64encode(hashlib.sha256(spki).digest()).decode()
    )


Runner = Callable[..., subprocess.CompletedProcess[str]]


def _run(args: Sequence[str], **kwargs: Any) -> subprocess.CompletedProcess[str]:
    """Runs a command with text output; the caller decides about the exit code."""
    res: subprocess.CompletedProcess[str] = subprocess.run(
        list(args), text=True, check=False, **kwargs
    )
    return res


@dataclass
class Stack:
    """One Compose project of the test: its name, the files and the variables of the overlay."""

    project: str
    env: dict[str, str]
    run: Runner = _run
    files: list[Path] = field(
        default_factory=lambda: [
            COMPOSE_DIR / "compose.yaml",
            COMPOSE_DIR / "test" / "compose.smoke.yaml",
            COMPOSE_DIR / "test" / "compose.console-e2e.yaml",
        ]
    )

    # Images this run built and must remove again (the host image when its tag was missing).
    created_images: list[str] = field(default_factory=list)

    def args(self, *rest: str) -> list[str]:
        """The docker compose command line of the project with ``rest``."""
        out = [
            "docker",
            "compose",
            "--project-name",
            self.project,
            "--project-directory",
            str(COMPOSE_DIR),
        ]
        for f in self.files:
            out += ["--file", str(f)]
        out += [
            "--env-file",
            str(COMPOSE_DIR / "test" / "smoke.env"),
            "--env-file",
            str(COMPOSE_DIR / "test" / "console-e2e.env"),
        ]
        return out + list(rest)

    def compose(
        self, *rest: str, check: bool = True, **kwargs: object
    ) -> subprocess.CompletedProcess[str]:
        """Runs docker compose; a non-zero exit is a StepError when ``check`` is set."""
        res = self.run(self.args(*rest), env=self.env, **kwargs)
        if check and res.returncode != 0:
            raise StepError(f"docker compose {rest[0]} failed with exit code {res.returncode}")
        return res

    def down(self) -> None:
        """Removes every container, volume, network and local image of the project."""
        self.compose(
            "down",
            "--volumes",
            "--remove-orphans",
            "--rmi",
            "local",
            "--timeout",
            "60",
            check=False,
            capture_output=True,
        )

    def leftovers(self) -> list[str]:
        """Names of containers, volumes and networks that still carry the project label."""
        label = f"label=com.docker.compose.project={self.project}"
        left: list[str] = []
        for kind in (["ps", "-a"], ["volume", "ls"], ["network", "ls"]):
            res = self.run(
                [
                    "docker",
                    *kind,
                    "--filter",
                    label,
                    "--format",
                    "{{.Names}}" if kind[0] == "ps" else "{{.Name}}",
                ],
                env=self.env,
                capture_output=True,
            )
            left += [line for line in res.stdout.splitlines() if line.strip()]
        return left


def clean_env(base: dict[str, str]) -> dict[str, str]:
    """``base`` without BOTHY_*, COMPOSE_* and OTEL_* (the .env of an installation never counts)."""
    return {k: v for k, v in base.items() if not k.startswith(("BOTHY_", "COMPOSE_", "OTEL_"))}


def preflight(image: str, run: Runner = _run) -> None:
    """Docker, Compose, their versions and the server image; PreconditionError names the fix."""
    if shutil.which("docker") is None:
        raise PreconditionError(
            "docker is not on PATH; install Docker Engine 26 or newer with Compose 2.26 or newer"
        )
    engine = run(["docker", "version", "--format", "{{.Server.Version}}"], capture_output=True)
    if engine.returncode != 0:
        raise PreconditionError("no Docker runtime is reachable; start Docker or set DOCKER_HOST")
    if parse_version(engine.stdout.strip()) < MIN_ENGINE:
        raise PreconditionError(
            f"Docker Engine {engine.stdout.strip()} is too old; 26.0.0 or newer is needed"
        )
    comp = run(["docker", "compose", "version", "--short"], capture_output=True)
    if comp.returncode != 0 or parse_version(comp.stdout.strip()) < MIN_COMPOSE:
        raise PreconditionError("docker compose 2.26.0 or newer is needed (!reset, !override)")
    if run(["docker", "image", "inspect", image], capture_output=True).returncode != 0:
        raise PreconditionError(
            f"server image {image} not found; build it with make image or set E2E_SERVER_IMAGE"
        )


# The Python packages of the browser image: playwright and its dependencies (tools/uv.lock).
BROWSER_PACKAGES = ("playwright", "greenlet", "pyee", "typing-extensions")


def browser_requirements(exported: str, packages: Sequence[str] = BROWSER_PACKAGES) -> str:
    """The requirement entries of ``packages`` with their hashes, cut out of ``uv export`` output.

    ``uv export --format requirements.txt`` writes one entry per package: a line ``name==version``
    (optionally with a marker) followed by indented ``--hash=`` lines and ``# via`` comments.
    Raises PreconditionError when a package is missing or has no hash, so the browser image never
    installs an unpinned package.
    """
    blocks: dict[str, list[str]] = {}
    current: list[str] | None = None
    for line in exported.splitlines():
        if line and not line[0].isspace() and not line.startswith("#"):
            name = re.split(r"[=<>!~;\[ ]", line, maxsplit=1)[0].lower()
            current = blocks.setdefault(name, [])
            current.append(line.rstrip(" \\"))
        elif current is not None and line.strip().startswith("--hash="):
            current.append("    " + line.strip().rstrip(" \\"))
    out = []
    for name in packages:
        entry = blocks.get(name, [])
        if len(entry) < 2:
            raise PreconditionError(f"tools/uv.lock does not pin {name} with hashes")
        out.append(" \\\n".join(entry))
    return "\n".join(out) + "\n"


def report_seen(logs: str) -> bool:
    """Reports whether the api log holds an accepted violation report of the negative test.

    The negative test injects an inline script, so the report names script-src-elem with the
    blocked origin inline (instance/domain.CSPViolation).
    """
    for line in logs.splitlines():
        if REPORT_LOG not in line:
            continue
        if "script-src-elem" in line and "inline" in line:
            return True
    return False


def save_results(results: Path, target: str | None) -> None:
    """Copies the result files of the browsers to ``target`` (the CI artifact of a failure).

    The result files hold violations, foreign origins, policy names and versions, and of phase 2
    the test results, the console output of the page and a screenshot of each failed test; never
    the login of the seed, which stays in out/ and is removed with the temp directory.
    """
    if not target or not results.is_dir():
        return
    dest = Path(target)
    dest.mkdir(parents=True, exist_ok=True)
    for pattern in ("*.json", "*.png"):
        for f in results.glob(pattern):
            shutil.copy2(f, dest / f.name)


def bundle_build_args(tmp: Path, version: str) -> list[str]:
    """The build of the web bundle of the patrol suites.

    The target ``console-e2e-web`` of the server Dockerfile, exported by the local exporter to
    ``<tmp>/admin-bundle``.
    """
    return [
        "docker", "buildx", "build", "--quiet",
        "--file", str(ROOT / "services" / "server" / "Dockerfile"),
        "--target", "console-e2e-web",
        "--build-arg", f"VERSION={version}",
        "--output", f"type=local,dest={tmp / ADMIN_BUNDLE}",
        str(ROOT),
    ]  # fmt: skip


def share_bundle(bundle: Path) -> None:
    """Makes the exported bundle readable for the api container and checks it.

    The user of the api container (65532) mounts it read-only; the bundle must hold the index.html
    the server needs.
    """
    if not (bundle / "index.html").is_file():
        raise StepError("the admin bundle holds no index.html")
    bundle.chmod(0o755)
    for path in bundle.rglob("*"):
        path.chmod(0o755 if path.is_dir() else 0o644)


def build_bundle(stack: Stack, tmp: Path, version: str) -> None:
    """Builds the bundle of phase 2 into the temp directory; raises StepError on a failure."""
    res = stack.run(bundle_build_args(tmp, version), env=stack.env)
    if res.returncode != 0:
        raise StepError(f"the build of the admin bundle failed with exit code {res.returncode}")
    share_bundle(tmp / ADMIN_BUNDLE)


def system_roots(image: str, scratch: Path, run: Runner = _run) -> bytes:
    """The CA file of the server image, copied out of a container that never starts.

    ``docker create`` plus ``docker cp`` read the roots the image itself trusts (Debian's
    ca-certificates of the distroless base), so the trust bundle adds the test CA to exactly the
    roots of a release; the container is removed by its ID and the copy in ``scratch`` is read and
    deleted. Raises StepError when the container cannot be created or the file not copied.
    """
    created = run(["docker", "create", "--pull", "never", image], capture_output=True)
    cid = created.stdout.strip()
    if created.returncode != 0 or not cid:
        raise StepError(f"docker create of {image} failed with exit code {created.returncode}")
    copy = scratch / "image-roots.pem"
    try:
        res = run(["docker", "cp", "-L", f"{cid}:{IMAGE_ROOTS}", str(copy)], capture_output=True)
        if res.returncode != 0 or not copy.is_file():
            raise StepError(f"docker cp of the CA file {IMAGE_ROOTS} failed")
        return copy.read_bytes()
    finally:
        copy.unlink(missing_ok=True)
        run(["docker", "rm", cid], capture_output=True)


def write_trust_bundle(certs: Path, roots: bytes) -> Path:
    """Writes ``ca-bundle.pem``: the system roots followed by the test CA of this run.

    api and the worker read it as ``SSL_CERT_FILE`` (Go's crypto/x509 on Linux), so the webhook
    client trusts the receiver and every public root as before; their user (65532) reads the file,
    hence mode 0644. Raises StepError when ``roots`` holds no certificate.
    """
    if PEM_CERT not in roots:
        raise StepError("the CA file of the server image holds no certificate")
    bundle = certs / TRUST_BUNDLE
    bundle.write_bytes(roots.rstrip(b"\n") + b"\n" + (certs / "ca.pem").read_bytes())
    bundle.chmod(0o644)
    return bundle


@dataclass(frozen=True)
class Delivery:
    """One request the webhook receiver logged: method, path, ``webhook-id`` and the status."""

    method: str
    path: str
    webhook_id: str
    status: int


def deliveries(logs: str) -> list[Delivery]:
    """The requests in the JSON access log of the receiver (caddy), in their order.

    A line may carry a prefix of ``docker compose logs``; lines that are no access log entry are
    skipped. Header names are compared without case, as HTTP does (Go sends ``Webhook-Id``).
    """
    out: list[Delivery] = []
    for line in logs.splitlines():
        start = line.find("{")
        if start < 0:
            continue
        try:
            entry = json.loads(line[start:])
        except ValueError:
            continue
        request = entry.get("request") if isinstance(entry, dict) else None
        if not isinstance(request, dict):
            continue
        headers = request.get("headers") or {}
        ids: list[Any] = next(
            (list(v) for k, v in headers.items() if k.lower() == "webhook-id"), []
        )
        out.append(
            Delivery(
                method=str(request.get("method") or ""),
                path=str(request.get("uri") or ""),
                webhook_id=str(ids[0]) if ids else "",
                status=int(entry.get("status") or 0),
            )
        )
    return out


def check_deliveries(seen: Sequence[Delivery]) -> None:
    """Raises StepError unless the receiver answered 2xx to a POST with a ``webhook-id`` header."""
    if not any(d.method == "POST" and d.webhook_id and 200 <= d.status < 300 for d in seen):
        raise StepError("the receiver recorded no delivered POST with a webhook-id header")


def run_admin(stack: Stack) -> None:
    """Phase 2.

    The trust bundle, the webhook receiver, api with the admin overlay and the worker (it delivers
    the webhooks), the sidecar after api (it shares the network namespace of api, so it must
    follow the new container), the patrol suites, then the deliveries the receiver logged.
    """
    say("phase 2: trust bundle (the roots of the server image and the test CA)")
    certs = Path(stack.env["E2E_CERTS"])
    write_trust_bundle(certs, system_roots(stack.env["SMOKE_SERVER_IMAGE"], certs, stack.run))
    stack.files.append(ADMIN_OVERLAY)
    stack.env["E2E_MODE"] = "admin"
    say("phase 2: the webhook receiver")
    stack.compose("up", "--detach", "--no-deps", RECEIVER)
    say("phase 2: api with the admin bundle and a step-up of 3 s, the worker")
    stack.compose(
        "up", "--detach", "--wait", "--wait-timeout", "240", "--no-deps", "--force-recreate",
        "api", "worker",
    )  # fmt: skip
    say("phase 2: restart the TLS sidecar")
    stack.compose("up", "--detach", "--no-deps", "--force-recreate", "tls")
    say("phase 2: patrol suites of the console administration (chromium)")
    res = stack.compose("run", "--rm", "--no-deps", "console-browser", check=False)
    if res.returncode != 0:
        raise StepError("the patrol suites failed; see the lines above and out/results/admin.json")
    say("phase 2: deliveries at the webhook receiver")
    logs = stack.compose(
        "logs", "--no-color", "--no-log-prefix", RECEIVER, capture_output=True
    ).stdout
    seen = deliveries(logs)
    check_deliveries(seen)
    paths = sorted({d.path for d in seen if d.webhook_id})
    say(f"phase 2: the receiver logged {len(seen)} requests at {', '.join(paths)}")


def terminal_bundle_build_args(tmp: Path, version: str) -> list[str]:
    """The build of the web bundle of the terminal patrol test.

    The target ``console-e2e-terminal-web`` of the server Dockerfile, exported to
    ``<tmp>/terminal-bundle``.
    """
    return [
        "docker", "buildx", "build", "--quiet",
        "--file", str(ROOT / "services" / "server" / "Dockerfile"),
        "--target", "console-e2e-terminal-web",
        "--build-arg", f"VERSION={version}",
        "--output", f"type=local,dest={tmp / TERMINAL_BUNDLE}",
        str(ROOT),
    ]  # fmt: skip


def host_image_tag(dockerfile: Path = HOST_DOCKERFILE) -> str:
    """The tag of the host image as the terminal suite of M2-48 names it (stack.HostImage).

    ``termsuite-host:`` plus the first 12 hex of the SHA-256 of the Dockerfile.
    """
    return "termsuite-host:" + hashlib.sha256(dockerfile.read_bytes()).hexdigest()[:12]


def host_image_build_args(tag: str) -> list[str]:
    """The build of the host image with BuildKit into the local image store, as M2-48 does."""
    return [
        "docker", "buildx", "build", "--quiet", "--load", "--tag", tag,
        "--file", str(HOST_DOCKERFILE), str(HOST_DOCKERFILE.parent),
    ]  # fmt: skip


def host_bin_build_args(tmp: Path) -> list[str]:
    """The build of bothy-host and the Fake-claude from ``services/host``.

    It uses host-bin.Dockerfile and exports to ``<tmp>/host-bin``; the CI host of the job has
    Docker but no Go.
    """
    return [
        "docker", "buildx", "build", "--quiet",
        "--file", str(HOST_BIN_DOCKERFILE),
        "--output", f"type=local,dest={tmp / HOST_BIN}",
        str(ROOT / "services" / "host"),
    ]  # fmt: skip


_FINGERPRINT = re.compile(r"^fingerprint: (\S+)$", re.M)


def fingerprint_of(stdout: str) -> str:
    """The fingerprint of the output of a successful ``bothy-host pair``, or "".

    As M2-48's stack.Fingerprint reads it.
    """
    found = _FINGERPRINT.search(stdout)
    return found.group(1) if found else ""


_DURATION = re.compile(r"^(\d+(?:\.\d+)?)(us|ms|s)$")
_RATE = re.compile(r"^(\d+(?:\.\d+)?)(B|KB|KiB|MB|MiB|GB|GiB)/s$")
_DURATION_MS = {"us": 0.001, "ms": 1.0, "s": 1000.0}
_RATE_BYTES = {
    "B": 1,
    "KB": 1000,
    "KiB": 1024,
    "MB": 1000**2,
    "MiB": 1024**2,
    "GB": 1000**3,
    "GiB": 1024**3,
}


def parse_duration_ms(text: str) -> float:
    """Milliseconds of a duration value of the configuration reference (``16ms``, ``1s``)."""
    found = _DURATION.match(text)
    if not found:
        raise StepError(f"{text!r} is no duration of the configuration reference")
    return float(found.group(1)) * _DURATION_MS[found.group(2)]


def parse_byte_rate(text: str) -> float:
    """Bytes per second of a byte rate of the configuration reference (``1MiB/s``)."""
    found = _RATE.match(text)
    if not found:
        raise StepError(f"{text!r} is no byte rate of the configuration reference")
    return float(found.group(1)) * _RATE_BYTES[found.group(2)]


def desktop_targets(reference: str) -> dict[str, dict[str, Any]]:
    """The defaults of PERF_KEYS from the table of docs/operations/configuration.md.

    The row of a key names its default as the first code value after the key; each comes with its
    value in ms or bytes per second. Raises StepError when a key or its default is missing.
    """
    targets: dict[str, dict[str, Any]] = {}
    for key in PERF_KEYS:
        row = re.search(rf"^\| `{re.escape(key)}` \|(.*)$", reference, re.M)
        value = re.search(r"`([^`]+)`", row.group(1)) if row else None
        if value is None:
            raise StepError(f"the configuration reference names no default of {key}")
        text = value.group(1)
        if key.endswith("_p95"):
            targets[key] = {"value": text, "ms": parse_duration_ms(text)}
        else:
            targets[key] = {"value": text, "bytes_per_s": parse_byte_rate(text)}
    return targets


def locked_versions(lock: str, names: Sequence[str]) -> dict[str, str]:
    """The versions of ``names`` in the workspace's pubspec.lock; StepError names a missing one."""
    packages = (yaml.safe_load(lock) or {}).get("packages") or {}
    out: dict[str, str] = {}
    for name in names:
        version = (packages.get(name) or {}).get("version")
        if not version:
            raise StepError(f"pubspec.lock does not lock {name}")
        out[name] = str(version)
    return out


def finish_terminal_result(
    path: Path, targets: dict[str, dict[str, Any]], versions: dict[str, str]
) -> dict[str, Any]:
    """Checks the result of phase 3 and adds the desktop targets, the comparison and the versions.

    Writes the file again and returns it.

    Raises StepError when the file is missing or unreadable, the browser side failed, a check of
    TERMINAL_CHECKS is missing or failed. The measurement is no gate (24 12): a value outside a
    desktop target is recorded as False in ``within_desktop_targets`` only.
    """
    try:
        doc = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as e:
        raise StepError(f"no readable result of phase 3 ({type(e).__name__})") from e
    checks = doc.get("checks") if isinstance(doc, dict) else None
    if not isinstance(checks, dict) or doc.get("ok") is not True:
        raise StepError("the terminal suites failed; see out/results/terminal.json")
    failed = [n for n in TERMINAL_CHECKS if not (checks.get(n) or {}).get("ok")]
    if failed:
        raise StepError(f"terminal checks failed: {failed}")
    measured = doc.get("measurement") or {}
    p95_key, rate_key = PERF_KEYS
    p95 = measured.get("frame_to_paint_p95_ms")
    rate = measured.get("throughput_bytes_per_s")
    doc["desktop_targets"] = targets
    doc["within_desktop_targets"] = {
        p95_key: isinstance(p95, int | float) and p95 <= targets[p95_key]["ms"],
        rate_key: isinstance(rate, int | float) and rate >= targets[rate_key]["bytes_per_s"],
    }
    doc["versions"] = versions
    path.write_text(json.dumps(doc, indent=2), encoding="utf-8")
    result: dict[str, Any] = doc
    return result


def diagnose_services(stack: Stack) -> list[str]:
    """The services whose last log lines a failure prints; the host container in phase 3."""
    out = list(DIAGNOSE_SERVICES)
    if TERMINAL_OVERLAY in stack.files:
        out.append(HOST_SERVICE)
    return out


def host_exec(stack: Stack, *command: str, **kwargs: Any) -> subprocess.CompletedProcess[str]:
    """Runs ``command`` as root in the host container (compose exec, no TTY)."""
    return stack.compose("exec", "-T", HOST_SERVICE, *command, **kwargs)


def wait_boot(stack: Stack) -> None:
    """Polls ``systemctl is-system-running`` until systemd reports running or degraded."""
    deadline = time.monotonic() + BOOT_WAIT_S
    while True:
        res = host_exec(stack, "systemctl", "is-system-running", check=False, capture_output=True)
        if res.stdout.strip() in ("running", "degraded"):
            return
        if time.monotonic() > deadline:
            raise StepError("systemd of the host container did not start")
        time.sleep(POLL_S)


def provision_host(stack: Stack, tmp: Path) -> None:
    """The steps of M2-48's Host.Provision and Install.

    The binaries into HOST_BIN_DIR (root, 0755), the start log of the Fake-claude, the test CA
    (mounted by the overlay) into the system trust store, then ``bothy-host install`` with the
    session user of Alex.
    """
    host_exec(stack, "install", "-d", "-o", "root", "-g", "root", "-m", "0755", HOST_BIN_DIR)
    for name in ("bothy-host", "claude"):
        stack.compose("cp", str(tmp / HOST_BIN / name), f"{HOST_SERVICE}:{HOST_BIN_DIR}/{name}")
    host_exec(
        stack,
        "bash",
        "-c",
        f"chown root:root {HOST_BIN_DIR}/* && chmod 0755 {HOST_BIN_DIR}/*"
        f" && install -m 0666 /dev/null {CLAUDE_START_LOG}"
        " && update-ca-certificates >/dev/null",
    )
    host_exec(
        stack, f"{HOST_BIN_DIR}/bothy-host", "install", "--session-user", SESSION_USER,
        capture_output=True,
    )  # fmt: skip


def write_private(path: Path, doc: dict[str, str]) -> None:
    """Writes ``doc`` as JSON through a temporary file, so the reader never sees half of it."""
    part = path.with_suffix(".part")
    part.write_text(json.dumps(doc), encoding="utf-8")
    part.replace(path)


def pair_on_request(stack: Stack, out: Path, container: str) -> None:
    """Waits for the pairing request of the seed, pairs the host and answers with the fingerprint.

    Returns early when the browser container ended without a request.

    The code goes to bothy-host only as an argument of docker exec (as M2-48 passes it); the
    request file is removed before the pairing, the code is never printed.
    """
    request = out / PAIRING_FILE
    deadline = time.monotonic() + PAIRING_WAIT_S
    while not request.exists():
        state = stack.run(
            ["docker", "inspect", "--format", "{{.State.Running}}", container],
            env=stack.env,
            capture_output=True,
        )
        if state.stdout.strip() != "true":
            return
        if time.monotonic() > deadline:
            raise StepError("the seed of phase 3 never asked for the pairing")
        time.sleep(POLL_S)
    body = json.loads(request.read_text(encoding="utf-8"))
    request.unlink()
    say("phase 3: pair the host service (bothy-host pair)")
    res = host_exec(
        stack, HOST_SERVICE_BIN, "pair", "--server", HOST_SERVER_URL,
        "--code", str(body["pairing_code"]), check=False, capture_output=True,
    )  # fmt: skip
    fingerprint = fingerprint_of(res.stdout)
    if res.returncode != 0 or not fingerprint:
        raise StepError(f"bothy-host pair failed with exit code {res.returncode}")
    write_private(out / PAIRED_FILE, {"fingerprint": fingerprint})


def run_browser_detached(stack: Stack, out: Path) -> int:
    """Starts the terminal suites detached, pairs the host on request and waits for the end.

    Prints the output of the container and returns its exit code. The container is removed always.
    """
    container = f"{stack.project}-terminal-browser"
    stack.compose("run", "--detach", "--no-deps", "--name", container, "console-browser")
    try:
        pair_on_request(stack, out, container)
        try:
            waited = stack.run(
                ["docker", "wait", container],
                env=stack.env,
                capture_output=True,
                timeout=BROWSER_WAIT_S,
            )
        except subprocess.TimeoutExpired as e:
            raise StepError("the terminal suites did not end in time") from e
        stack.run(["docker", "logs", container], env=stack.env)
        code = waited.stdout.strip()
        return int(code) if waited.returncode == 0 and code.isdigit() else 1
    finally:
        stack.run(["docker", "rm", "--force", container], env=stack.env, capture_output=True)


def run_terminal(stack: Stack, tmp: Path, version: str) -> None:
    """Phase 3.

    The bundle, the host image and binaries, api with the terminal overlay, the TLS sidecar after
    it, the host container, then the terminal suites; raises StepError.
    """
    say("phase 3: build the terminal bundle, the host image and the host binaries")
    res = stack.run(terminal_bundle_build_args(tmp, version), env=stack.env)
    if res.returncode != 0:
        raise StepError(f"the build of the terminal bundle failed with exit code {res.returncode}")
    share_bundle(tmp / TERMINAL_BUNDLE)
    tag = host_image_tag()
    if stack.run(["docker", "image", "inspect", tag], capture_output=True).returncode != 0:
        stack.created_images.append(tag)
        if stack.run(host_image_build_args(tag), env=stack.env).returncode != 0:
            raise StepError("the build of the host image failed")
    if stack.run(host_bin_build_args(tmp), env=stack.env).returncode != 0:
        raise StepError("the build of the host binaries failed")
    if ADMIN_OVERLAY in stack.files:
        stack.files.remove(ADMIN_OVERLAY)
    stack.files.append(TERMINAL_OVERLAY)
    stack.env.update(E2E_MODE="terminal", E2E_HOST_IMAGE=tag)
    say("phase 3: api with the terminal bundle on the network of the host container")
    stack.compose(
        "up", "--detach", "--wait", "--wait-timeout", "240", "--no-deps", "--force-recreate", "api"
    )
    stack.compose("up", "--detach", "--no-deps", "--force-recreate", "tls")
    say("phase 3: the host service container (systemd, bothy-host install)")
    stack.compose("up", "--detach", "--no-deps", HOST_SERVICE)
    wait_boot(stack)
    provision_host(stack, tmp)
    say("phase 3: terminal suites (chromium); the output follows when they end")
    if run_browser_detached(stack, tmp / "out") != 0:
        raise StepError("the terminal suites failed; see the lines above and out/results")
    targets = desktop_targets(CONFIG_REFERENCE.read_text(encoding="utf-8"))
    versions = locked_versions(PUBSPEC_LOCK.read_text(encoding="utf-8"), VERSIONED_PACKAGES)
    doc = finish_terminal_result(tmp / "out" / "results" / TERMINAL_RESULT, targets, versions)
    # The temp directory goes at the teardown, so the numbers of 24 12 go to the output: numbers,
    # targets and versions only, never terminal content.
    summary = {
        "measurement": {k: v for k, v in (doc.get("measurement") or {}).items() if k != "ok"},
        "desktop_targets": doc["desktop_targets"],
        "within_desktop_targets": doc["within_desktop_targets"],
        "versions": doc["versions"],
    }
    say("phase 3: measurement " + json.dumps(summary, sort_keys=True))


def run_test(stack: Stack, tmp: Path, version: str) -> None:
    """The steps of the test on a stack whose variables are set; raises StepError."""
    say("validate")
    stack.compose("config", "--quiet")
    say("build and pull")
    stack.compose("build", "--quiet", "--build-arg", f"VERSION={version}", "smoke-probe")
    stack.compose("build", "--quiet", "console-browser")
    stack.compose("pull", "--quiet", "postgres", "tls")
    say("build the web bundle of the patrol suites")
    build_bundle(stack, tmp, version)
    say("start (secrets init, migrate up --bootstrap, api)")
    stack.compose("up", "--detach", "--wait", "--wait-timeout", "240", "api")
    say("bootstrap owner")
    setup_out = tmp / "out" / "setup.out"
    with (
        setup_out.open("w", encoding="utf-8") as out,
        (tmp / "logs" / "setup.err").open("w", encoding="utf-8") as err,
    ):
        setup_out.chmod(0o600)
        stack.compose(
            "exec",
            "-T",
            "api",
            "bothy",
            "setup",
            "bootstrap-owner",
            "--handle",
            "alex",
            "--display-name",
            "Alex",
            stdout=out,
            stderr=err,
        )
    say("seed (password and TOTP of the owner)")
    stack.compose("run", "--rm", "--no-deps", "--user", stack.env["E2E_UID_GID"], "console-seed")
    say("start the TLS sidecar")
    stack.compose("up", "--detach", "--no-deps", "tls")
    say("browsers (" + stack.env.get("E2E_BROWSERS", "chromium,firefox,webkit") + ")")
    res = stack.compose("run", "--rm", "--no-deps", "console-browser", check=False)
    if res.returncode != 0:
        raise StepError("the headless test failed; see the lines above and out/results")
    say("report reached the server")
    logs = stack.compose("logs", "--no-color", "api", capture_output=True).stdout
    if not report_seen(logs):
        raise StepError("no violation report of the negative test reached the endpoint")
    run_admin(stack)
    run_terminal(stack, tmp, version)


UV_EXPORT = (
    "uv", "export", "--project", str(ROOT / "tools"), "--locked", "--only-group", "dev",
    "--no-emit-project", "--no-header", "--format", "requirements.txt",
)  # fmt: skip


def workspace(environ: dict[str, str], run: Runner) -> Path:
    """Creates the temp directory of the run with the requirements of the browser image.

    Raises PreconditionError when tools/uv.lock cannot be exported; the directory is removed then.
    """
    tmp = Path(tempfile.mkdtemp(prefix="bothy-console-e2e.", dir=environ.get("TMPDIR")))
    for sub in ("secrets-input", "out", "logs", "browser"):
        (tmp / sub).mkdir(mode=0o700)
    # The browser container writes its results as the calling user, the build reads the context.
    (tmp / "out").chmod(0o777)
    (tmp / "browser").chmod(0o755)
    try:
        exported = run(list(UV_EXPORT), capture_output=True)
        if exported.returncode != 0:
            raise PreconditionError("uv export of tools/uv.lock failed; run make bootstrap-ci")
        requirements = browser_requirements(exported.stdout)
    except PreconditionError:
        shutil.rmtree(tmp, ignore_errors=True)
        raise
    (tmp / "browser" / "requirements.txt").write_text(requirements, encoding="utf-8")
    return tmp


def stack_env(environ: dict[str, str], tmp: Path, image: str, run_id: str) -> dict[str, str]:
    """Return the environment of docker compose.

    It is the caller's without installation variables, plus the inputs of the overlays and the
    certificates of this run.
    """
    certs = make_certs(tmp / "certs")
    # The sidecar reads the key as the calling user inside the container; a throwaway key.
    (tmp / "certs").chmod(0o755)
    (tmp / "certs" / "leaf.key").chmod(0o644)
    env = clean_env(environ)
    env.update(
        SMOKE_SERVER_IMAGE=image,
        SMOKE_RUN_ID=run_id,
        SMOKE_SECRETS_INPUT=str(tmp / "secrets-input"),
        SMOKE_OUT=str(tmp / "out"),
        SMOKE_TMP=str(tmp),
        SMOKE_CANARY="e2e" + secrets.token_hex(12),
        E2E_CERTS=str(certs.directory),
        E2E_BROWSER_CONTEXT=str(tmp / "browser"),
        E2E_SPKI=certs.spki_sha256,
        E2E_UID_GID=f"{os.getuid()}:{os.getgid()}",
        E2E_ADMIN_BUNDLE=str(tmp / ADMIN_BUNDLE),
        E2E_TERMINAL_BUNDLE=str(tmp / TERMINAL_BUNDLE),
    )
    env.pop("E2E_MODE", None)
    env.pop("E2E_HOST_IMAGE", None)
    return env


def cleanup(stack: Stack, tmp: Path, run_id: str) -> bool:
    """Removes the stack, its images and the temp directory; reports whether nothing is left."""
    say("cleanup")
    stack.down()
    for img in (f"bothy-smoke:{run_id}", f"bothy-console-browser:{run_id}", *stack.created_images):
        stack.run(["docker", "image", "rm", img], capture_output=True)
    shutil.rmtree(tmp, ignore_errors=True)
    left = stack.leftovers()
    if left:
        say("cleanup left objects behind: " + ", ".join(left))
    return not left


def main(
    argv: Sequence[str] | None = None, environ: dict[str, str] | None = None, run: Runner = _run
) -> int:
    """Run the test and return the exit code.

    The stack and the temp directory always go, unless E2E_KEEP=1 keeps them for debugging.
    """
    del argv
    environ = dict(os.environ if environ is None else environ)
    image = environ.get("E2E_SERVER_IMAGE") or default_image()
    try:
        preflight(image, run)
        tmp = workspace(environ, run)
    except PreconditionError as e:
        say(str(e))
        return 2
    run_id = secrets.token_hex(4)
    try:
        env = stack_env(environ, tmp, image, run_id)
    except Exception:
        shutil.rmtree(tmp, ignore_errors=True)
        raise
    stack = Stack(project=f"bothy-console-e2e-{run_id}", env=env, run=run)
    version = (ROOT / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    rc = 0
    try:
        run_test(stack, tmp, version)
        say("PASS")
    except StepError as e:
        rc = 1
        say(f"FAIL: {e}")
        stack.compose("ps", "--all", check=False)
        for svc in diagnose_services(stack):
            stack.compose("logs", "--no-color", "--tail", "100", svc, check=False)
        save_results(tmp / "out" / "results", environ.get("E2E_ARTIFACTS"))
    finally:
        if environ.get("E2E_KEEP") == "1":
            say(f"E2E_KEEP=1: the stack {stack.project} and {tmp} are kept")
        elif not cleanup(stack, tmp, run_id):
            rc = 1
    return rc


if __name__ == "__main__":
    sys.exit(main())
