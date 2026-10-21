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
import os
import re
import secrets
import shutil
import subprocess
import sys
import tempfile
from collections.abc import Callable, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

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
DIAGNOSE_SERVICES = ("api", "migrate", "tls")
# The overlay of phase 2 (brief M6-04 Design note 4) and the directory of the bundle in the temp
# directory of the run.
ADMIN_OVERLAY = COMPOSE_DIR / "test" / "compose.console-e2e-admin.yaml"
ADMIN_BUNDLE = "admin-bundle"


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
    """Writes a test CA and a leaf for ``localhost``, ``api`` and 127.0.0.1 into ``directory``.

    The keys are P-256 and exist only for this run; the leaf is valid for one day. Chromium trusts
    the leaf by its SPKI hash (``--ignore-certificate-errors-spki-list``), which treats the
    connection as a valid certificate, so WebAuthn and the secure context behave as with a public
    certificate (docs/DECISIONS.md D-M6-03-6).
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
    """The build of the web bundle of the patrol suites: the target ``console-e2e-web`` of the
    server Dockerfile, exported by the local exporter to ``<tmp>/admin-bundle``."""
    return [
        "docker", "buildx", "build", "--quiet",
        "--file", str(ROOT / "services" / "server" / "Dockerfile"),
        "--target", "console-e2e-web",
        "--build-arg", f"VERSION={version}",
        "--output", f"type=local,dest={tmp / ADMIN_BUNDLE}",
        str(ROOT),
    ]  # fmt: skip


def share_bundle(bundle: Path) -> None:
    """Makes the exported bundle readable for the user of the api container (65532), which
    mounts it read-only, and checks that it holds the index.html the server needs."""
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


def run_admin(stack: Stack) -> None:
    """Phase 2: api with the admin overlay, the sidecar after it (it shares the network
    namespace of api, so it must follow the new container), then the patrol suites."""
    stack.files.append(ADMIN_OVERLAY)
    stack.env["E2E_MODE"] = "admin"
    say("phase 2: api with the admin bundle and a step-up of 3 s")
    stack.compose(
        "up", "--detach", "--wait", "--wait-timeout", "240", "--no-deps", "--force-recreate", "api"
    )
    say("phase 2: restart the TLS sidecar")
    stack.compose("up", "--detach", "--no-deps", "--force-recreate", "tls")
    say("phase 2: patrol suites of the console administration (chromium)")
    res = stack.compose("run", "--rm", "--no-deps", "console-browser", check=False)
    if res.returncode != 0:
        raise StepError("the patrol suites failed; see the lines above and out/results/admin.json")


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
    )
    env.pop("E2E_MODE", None)
    return env


def cleanup(stack: Stack, tmp: Path, run_id: str) -> bool:
    """Removes the stack, its images and the temp directory; reports whether nothing is left."""
    say("cleanup")
    stack.down()
    for img in (f"bothy-smoke:{run_id}", f"bothy-console-browser:{run_id}"):
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
        for svc in DIAGNOSE_SERVICES:
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
