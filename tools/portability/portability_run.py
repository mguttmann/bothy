# SPDX-License-Identifier: Apache-2.0
r"""Runner of the portability test: ``make portability`` with ``PORTABILITY_TAG`` (brief M7-14).

Installs a released product version like a stranger and verifies what runs
(docs/spec/16-operations.md 11.1). In this order, once with ``--storage fs`` (profiles
``direct,scan``) and once with ``--storage s3`` (profiles ``direct,scan,s3``):

1. the three variables of the interface are checked; an invalid value aborts here, before anything
   is downloaded, pulled or installed, and names the variable (exit code 2);
2. ``install.sh`` and the bundle ``bothy-compose-<version>.tar.gz`` come from the release URL of
   ``GITHUB_REPOSITORY`` and ``PORTABILITY_TAG`` (source ``url``) or from
   ``PORTABILITY_RELEASE_DIR`` (source ``release-download``);
3. ``BOTHY_REGISTRY`` and ``BOTHY_VERSION`` come from the bundle's ``.env.example``; every image its
   ``compose.yaml`` names with ``${BOTHY_VERSION}`` is pulled without a login and the signature of
   its digest is verified with cosign in a container pinned by digest: with the public key of the
   private phase from the repository or keyless with the identity of the release workflow (ADR
   0062 item 5); one unsigned image ends the run here, before ``install.sh``;
4. ``install.sh`` runs without a terminal with exactly the options of
   ``tests/portability/install-options`` (``--storage`` per run, ``--bundle`` for
   ``release-download``); the containers of the installation must run exactly the verified images;
5. the local observability stack is switched on with the overlay ``local``; the smoke scenario of
   M1-73 (owner from the setup wizard, space, channel, thread, message, WebSocket reception),
   then the scenario of this test (a run of the test adapter, an upload clean and with the EICAR
   test file, the search, the status page), then a backup and the restore test;
6. telemetry of the server arrived in Loki, Prometheus and Tempo, and none of their data nor any
   container log holds a canary of the run (chapter 17);
7. VZ-T13 (docs/spec/25-visualization.md section 12, brief M7-26): Grafana of the overlay starts
   with the plugin build of the bundle; through Grafana's HTTP API on its loopback (inside the
   container, the admin password read there from the secrets volume) the panel plugin
   ``bothy-live-panel`` is loaded and unsigned, Grafana reports no plugin error, the dashboard
   ``bothy-live`` is provisioned and every panel type it uses is installed; Grafana's log names
   only ``bothy-live-panel`` as unsigned and has no error of the dashboard provisioning;
8. the fs run checks the tailnet case without an account: in the network namespace of the
   ``tailscale`` sidecar the public listener listens on loopback only, so only the port
   ``tailscale serve`` exposes reaches the API (16 11.1, D-M7-14-3);
9. the digests of the verified images and the log lines of Grafana about unsigned plugins are
   reported.

On a failure the run prints ``docker compose ps --all`` and the end of the logs of exactly its
installation (its folder and project name) before it cleans up (16 11.1 rule 5). The cleanup
removes the containers, volumes and networks of the project, the folder, the probe image and the
temporary directory, and nothing else.

Trial-only variables (``make portability-trial``, D-W-127), refused when ``GITHUB_ACTIONS`` is set:
``PORTABILITY_TRIAL_PUBLIC_KEY`` (the public key of the trial release), and
``PORTABILITY_TRIAL_FAIL_AFTER_INSTALL=1`` with ``PORTABILITY_TRIAL_STORAGES`` (a forced failure
right after the installation, to prove the diagnosis), and ``PORTABILITY_TRIAL_KEEP=1`` (keep a
failed installation and the temporary directory for debugging).

Exit codes: 0 pass, 1 a step failed, 2 an invalid interface value or a missing precondition.
Progress lines go to stderr as ``portability: <step>``; no secret is ever printed.

Example::

    PORTABILITY_TAG=v0.1.0 PORTABILITY_INSTALL_SOURCE=url GITHUB_REPOSITORY=alex/bothy \\
        uv run --project tools python tools/portability/portability_run.py
"""

from __future__ import annotations

import io
import json
import os
import re
import secrets
import shlex
import shutil
import signal
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request
from collections.abc import Callable, Iterator, Mapping, Sequence
from contextlib import contextmanager
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools" / "ci"))
import product_release  # noqa: E402 - the bundle helpers of the release chain

OPTIONS_FILE = ROOT / "tests" / "portability" / "install-options"
ADAPTER = ROOT / "tests" / "portability" / "adapter.py"
REPO_KEY = ROOT / "quality" / "release" / "cosign-private-phase.pub"
# renovate: datasource=docker depName=ghcr.io/sigstore/cosign/cosign
COSIGN_IMAGE = (
    "ghcr.io/sigstore/cosign/cosign:v3.1.3"
    "@sha256:9e5c2f2edc34351160407ca3416c61855bdf9403c3c5936e0f0be7fc261611b8"
)
OIDC_ISSUER = "https://token.actions.githubusercontent.com"
TAG = re.compile(r"^v(?P<version>[0-9]+\.[0-9]+\.[0-9]+(-beta\.[0-9]+)?)$")
REPOSITORY = re.compile(r"^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$")
SOURCES = ("url", "release-download")
STORAGES = ("fs", "s3")
OWNER_HANDLE = "alex"
PUBLIC_URL = "https://chat.example.com"
SERVICE = "bothy-server"
# Values of the run (named here, the only place): the bound of every subprocess, the wait for the
# adapter files and the end of the logs printed on a failure.
INSTALL_TIMEOUT_S = 1800
STEP_TIMEOUT_S = 1800
ADAPTER_WAIT_S = 600
LOG_TAIL = 200
# Published ports of the installation, all set to 0 in the shell environment (Compose prefers it
# over .env): Docker picks a free port where it binds, so a run never meets a port in use, also
# when Docker runs in a VM whose ports differ from the host's.
PORT_VARIABLES = (
    "BOTHY_HTTP_PUBLISH_PORT",
    "BOTHY_OTLP_PUBLISH_PORT",
    "BOTHY_PROMETHEUS_PUBLISH_PORT",
    "BOTHY_ALERTMANAGER_PUBLISH_PORT",
    "BOTHY_GRAFANA_PUBLISH_PORT",
)
# The services of the overlay local (docs/observability/backends.md); Alertmanager and Grafana
# stay off, they play no part in the export and Alertmanager needs its webhook URLs first.
LOCAL_STACK = ("prometheus", "loki", "tempo", "otel-collector")
# VZ-T13: the one unsigned plugin and the dashboard that uses it (ruling T11, SEC-VZ-5).
GRAFANA_PLUGIN = "bothy-live-panel"
GRAFANA_DASHBOARD = "bothy-live"
GRAFANA_WAIT_S = 300
# The API call inside the container: the Basic credentials go through stdin (`curl -H @-`), so
# the password never appears in an argument list or the log; $1 is the path.
GRAFANA_API = (
    'printf "Authorization: Basic %s\\n" "$(printf "admin:%s" '
    '"$(cat /run/bothy/secrets/grafana_admin_password)" | base64 | tr -d "\\n")"'
    ' | curl -fsS -H @- "http://127.0.0.1:3000$1"'
)
_PLUGIN_ID_FIELD = re.compile(r'\b(?:id|pluginId|pluginID|plugin_id)="?([^\s"]+)"?')
ENROLLMENT = re.compile(r"en_[0-9A-HJKMNP-TV-Z]{4}(-[0-9A-HJKMNP-TV-Z]{4}){7}")


class InterfaceError(Exception):
    """An invalid value of the interface or a missing precondition (exit code 2)."""


class StepError(Exception):
    """A step of the test failed (exit code 1)."""


class KeptError(StepError):
    """A step failed and the trial keeps the installation for debugging."""


def say(message: str) -> None:
    """Write one progress line to stderr."""
    print(f"portability: {message}", file=sys.stderr, flush=True)


Runner = Callable[..., subprocess.CompletedProcess[str]]


def _run(args: Sequence[str], **kwargs: Any) -> subprocess.CompletedProcess[str]:
    """Run a command with text output; the caller decides about the exit code."""
    kwargs.setdefault("timeout", STEP_TIMEOUT_S)
    res: subprocess.CompletedProcess[str] = subprocess.run(
        list(args), text=True, check=False, **kwargs
    )
    return res


# ---- interface (16 11.1, "Interface of make portability") --------------------------------------


@dataclass(frozen=True)
class Interface:
    """The checked interface of one run.

    Attributes:
        tag: The product tag under test (``vX.Y.Z`` or ``vX.Y.Z-beta.N``).
        version: The tag without ``v``, the version in the bundle's name.
        source: ``url`` or ``release-download``.
        release_dir: The folder of ``gh release download`` (``release-download`` only).
        repository: ``GITHUB_REPOSITORY`` (``url`` only).
        trial_key: The trial's public key (trial only).
        storages: The storage runs, both unless the trial narrows them.
        fail_after_install: The trial's forced failure after the installation.
        keep: Keep the installation and the temporary directory after a failure (trial only, to
            debug a run).
    """

    tag: str
    version: str
    source: str
    release_dir: Path | None = None
    repository: str | None = None
    trial_key: Path | None = None
    storages: tuple[str, ...] = STORAGES
    fail_after_install: bool = False
    keep: bool = False


def read_interface(environ: Mapping[str, str]) -> Interface:
    """Check the variables of the interface; nothing is downloaded, pulled or installed yet.

    Args:
        environ: The environment of the run.

    Returns:
        The checked interface.

    Raises:
        InterfaceError: A value is missing or invalid; the message names the variable.
    """
    tag = environ.get("PORTABILITY_TAG", "")
    m = TAG.match(tag)
    if m is None:
        raise InterfaceError(f"PORTABILITY_TAG must be vX.Y.Z or vX.Y.Z-beta.N, not {tag!r}")
    source = environ.get("PORTABILITY_INSTALL_SOURCE", "")
    if source not in SOURCES:
        raise InterfaceError(
            f"PORTABILITY_INSTALL_SOURCE must be {' or '.join(SOURCES)}, not {source!r}"
        )
    version = m.group("version")
    release_dir = repository = None
    if source == "release-download":
        raw = environ.get("PORTABILITY_RELEASE_DIR", "")
        if not raw or not Path(raw).is_dir():
            raise InterfaceError(f"PORTABILITY_RELEASE_DIR must name a folder, not {raw!r}")
        release_dir = Path(raw).resolve()
        for name in ("install.sh", product_release.bundle_name(version)):
            if not (release_dir / name).is_file():
                raise InterfaceError(f"PORTABILITY_RELEASE_DIR has no {name}")
        # Optional here: it names the identity of the keyless signatures (ADR 0062 item 5).
        named = environ.get("GITHUB_REPOSITORY", "")
        if named and not REPOSITORY.match(named):
            raise InterfaceError(f"GITHUB_REPOSITORY must be <owner>/<repository>, not {named!r}")
        repository = named or None
    else:
        repository = environ.get("GITHUB_REPOSITORY", "")
        if not REPOSITORY.match(repository):
            raise InterfaceError(
                f"GITHUB_REPOSITORY must be <owner>/<repository> for the source url, "
                f"not {repository!r}"
            )
    trial = {k: v for k, v in environ.items() if k.startswith("PORTABILITY_TRIAL_") and v}
    if trial and environ.get("GITHUB_ACTIONS"):
        raise InterfaceError(f"{sorted(trial)[0]} is refused under GitHub Actions (D-W-127)")
    trial_key = None
    if "PORTABILITY_TRIAL_PUBLIC_KEY" in trial:
        trial_key = Path(trial["PORTABILITY_TRIAL_PUBLIC_KEY"]).resolve()
        if not trial_key.is_file():
            raise InterfaceError("PORTABILITY_TRIAL_PUBLIC_KEY must name a file")
    storages: tuple[str, ...] = STORAGES
    if "PORTABILITY_TRIAL_STORAGES" in trial:
        storages = tuple(trial["PORTABILITY_TRIAL_STORAGES"].split(","))
        if not storages or any(s not in STORAGES for s in storages):
            raise InterfaceError("PORTABILITY_TRIAL_STORAGES must list fs and/or s3")
    fail = trial.get("PORTABILITY_TRIAL_FAIL_AFTER_INSTALL", "")
    if fail not in ("", "1"):
        raise InterfaceError("PORTABILITY_TRIAL_FAIL_AFTER_INSTALL must be 1 or empty")
    keep = trial.get("PORTABILITY_TRIAL_KEEP", "")
    if keep not in ("", "1"):
        raise InterfaceError("PORTABILITY_TRIAL_KEEP must be 1 or empty")
    return Interface(
        tag, version, source, release_dir, repository, trial_key, storages, fail == "1", keep == "1"
    )


def release_base(repository: str, tag: str) -> str:
    """The release download address of the tag, from the repository, never from documentation.

    Example: ``release_base("alex/bothy", "v0.1.0")`` gives
    ``https://github.com/alex/bothy/releases/download/v0.1.0/`` (16 11.1 rule 3).
    """
    return f"https://github.com/{repository}/releases/download/{tag}/"


def read_options(path: Path = OPTIONS_FILE) -> list[str]:
    """The options of install.sh: the one line of the options file that is no comment.

    Raises:
        StepError: The file has not exactly one such line.
    """
    lines = [
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    if len(lines) != 1:
        raise StepError(f"{path.name} must hold exactly one line of options, has {len(lines)}")
    return shlex.split(lines[0])


def install_args(options: Sequence[str], storage: str, bundle: Path | None) -> list[str]:
    """The arguments of install.sh: the options with ``--storage`` set, ``--bundle`` appended.

    Raises:
        StepError: The options lack ``--storage`` or already name ``--bundle``.
    """
    args = list(options)
    if "--storage" not in args or args.index("--storage") + 1 >= len(args):
        raise StepError("the options file has no --storage <value>")
    if any(a == "--bundle" or a.startswith("--bundle=") for a in args):
        raise StepError("the options file must not name --bundle; the test adds it")
    args[args.index("--storage") + 1] = storage
    if bundle is not None:
        args += ["--bundle", str(bundle)]
    return args


# ---- the installation's configuration (16 5.1 item 3, brief requirement 17) -------------------

_ENV_ROW = re.compile(
    r"^\| `[a-z][a-z0-9_.]*` \| `(BOTHY_[A-Z0-9_]+)` \|.*\| (?P<status>[^|]+) \|\s*$"
)


def known_keys(configuration_md: str) -> set[str]:
    """The environment names of the keys this server version knows (status ``active``).

    A key with the status ``planned (<milestone>)`` is rejected as unknown at the start
    (docs/operations/configuration.md section 1).
    """
    out = set()
    for line in configuration_md.splitlines():
        m = _ENV_ROW.match(line)
        if m and m.group("status").strip() == "active":
            out.add(m.group(1))
    return out


def unknown_server_keys(compose_text: str, configuration_md: str) -> list[str]:
    """``BOTHY_*`` variables a server container of compose.yaml gets that no active key names.

    The server aborts its start on such a variable (16 5.1 item 3), so the installation must set
    only known keys. Variables without ``__`` are Compose variables, not server keys.
    """
    import yaml  # noqa: PLC0415 - only this check parses YAML

    doc = yaml.safe_load(compose_text)
    known = known_keys(configuration_md)
    unknown: set[str] = set()
    for svc in doc.get("services", {}).values():
        if "bothy-server:" not in str(svc.get("image", "")):
            continue
        env = svc.get("environment") or {}
        names = env.keys() if isinstance(env, dict) else [e.split("=", 1)[0] for e in env]
        for name in names:
            if name.startswith("BOTHY_") and "__" in name and name not in known:
                unknown.add(name)
    return sorted(unknown)


# ---- the release under test ------------------------------------------------------------------


@dataclass(frozen=True)
class Release:
    """install.sh and the bundle of the tag, and what the bundle pins.

    Attributes:
        script: install.sh as downloaded or from the release folder.
        bundle: The bundle file.
        registry: ``BOTHY_REGISTRY`` of the bundle.
        server: ``BOTHY_VERSION`` of the bundle (the server version).
        images: ``<name>:<tag>`` of every image compose.yaml names with ``${BOTHY_VERSION}``.
    """

    script: Path
    bundle: Path
    registry: str
    server: str
    images: tuple[str, ...]

    def refs(self) -> list[str]:
        """The full references the installation pulls."""
        return [f"{self.registry}/{img}" for img in self.images]


def bundle_values(data: bytes, version: str) -> tuple[str, str, tuple[str, ...]]:
    """Read registry, server version and the version images from the bundle bytes.

    Raises:
        StepError: A file or a value is missing in the bundle.
    """
    prefix = f"bothy-{version}/infra/compose/"
    try:
        with tarfile.open(fileobj=io.BytesIO(data)) as archive:
            env_text = _member(archive, prefix + ".env.example")
            compose_text = _member(archive, prefix + "compose.yaml")
    except tarfile.TarError as exc:
        raise StepError(f"the bundle is no tar archive: {exc}") from exc
    values = dict(re.findall(r"^(BOTHY_REGISTRY|BOTHY_VERSION)=(.*)$", env_text, re.M))
    registry, server = values.get("BOTHY_REGISTRY", ""), values.get("BOTHY_VERSION", "")
    if not registry or not server:
        raise StepError("the bundle's .env.example lacks BOTHY_REGISTRY or BOTHY_VERSION")
    images = tuple(product_release.compose_images(compose_text, server))
    if not images:
        raise StepError("the bundle's compose.yaml names no image with ${BOTHY_VERSION}")
    return registry, server, images


def _member(archive: tarfile.TarFile, name: str) -> str:
    """The text of one member of the bundle."""
    try:
        handle = archive.extractfile(name)
    except KeyError:
        handle = None
    if handle is None:
        raise StepError(f"the bundle has no {name}")
    return handle.read().decode("utf-8")


def _download(url: str, target: Path) -> None:
    """Download one release file over HTTPS (redirects to the asset store are followed)."""
    if not url.startswith("https://github.com/"):
        raise StepError(f"refusing a download outside the release address: {url}")
    with urllib.request.urlopen(url, timeout=300) as resp, target.open("wb") as fh:  # noqa: S310  # nosec B310
        shutil.copyfileobj(resp, fh)


def fetch_release(iface: Interface, workdir: Path) -> Release:
    """Fetch install.sh and the bundle from the source of the interface and read the bundle."""
    name = product_release.bundle_name(iface.version)
    if iface.release_dir is not None:
        script, bundle = iface.release_dir / "install.sh", iface.release_dir / name
    else:
        if iface.repository is None:
            raise StepError("the source url needs GITHUB_REPOSITORY")
        base = release_base(iface.repository, iface.tag)
        target = workdir / "release"
        target.mkdir()
        script, bundle = target / "install.sh", target / name
        _download(base + "install.sh", script)
        _download(base + name, bundle)
    registry, server, images = bundle_values(bundle.read_bytes(), iface.version)
    return Release(script, bundle, registry, server, images)


# ---- images: pull, verify, compare -------------------------------------------------------------


@dataclass(frozen=True)
class Verified:
    """One verified image: its reference, the signed digest and the local image ID."""

    ref: str
    digest: str
    image_id: str


def repository_of(ref: str) -> str:
    """The repository of an image reference, without tag or digest.

    Example: ``127.0.0.1:5000/bothy-server:1.0.0`` gives ``127.0.0.1:5000/bothy-server``.
    """
    if "@" in ref:
        return ref.split("@", 1)[0]
    head, _, last = ref.rpartition("/")
    return f"{head}/{last.split(':', 1)[0]}" if head else last.split(":", 1)[0]


_PULL_DIGEST = re.compile(r"^Digest: (sha256:[0-9a-f]{64})$", re.M)


def pulled_digest(ref: str, pull_output: str) -> str:
    """The digest ``docker pull`` reports for the tag (its line ``Digest: sha256:...``).

    Raises:
        StepError: The output names no digest or more than one.
    """
    found: set[str] = set(_PULL_DIGEST.findall(pull_output))
    if len(found) != 1:
        raise StepError(f"docker pull {ref} reported {len(found)} digests, want one")
    return found.pop()


def cosign_commands(
    ref_at_digest: str, key: Path | None, repository: str | None
) -> list[list[str]]:
    """The verifications that may pass a digest, in order: key, then keyless.

    Keyed: ``verify --key <key> --insecure-ignore-tlog`` (private phase, ADR 0062). Keyless: the
    identity of the server release workflow of the repository and the GitHub OIDC issuer. The
    key is mounted at ``/key.pub``.
    """
    commands = []
    if key is not None:
        commands.append(["verify", "--key", "/key.pub", "--insecure-ignore-tlog", ref_at_digest])
    if repository is not None:
        identity = (
            "^https://github\\.com/"
            + re.escape(repository)
            + "/\\.github/workflows/release\\.yml@refs/tags/server/v"
        )
        commands.append(
            [
                "verify",
                "--certificate-identity-regexp",
                identity,
                "--certificate-oidc-issuer",
                OIDC_ISSUER,
                ref_at_digest,
            ]
        )
    return commands


def verify_images(
    release: Release, key: Path | None, repository: str | None, run: Runner = _run
) -> list[Verified]:
    """Pull every image without a login and verify the signature of its digest.

    Raises:
        StepError: A pull fails or no verification passes for a digest.
    """
    if key is None and repository is None:
        raise StepError("no way to verify: neither a public key nor a repository identity")
    out = []
    for ref in release.refs():
        pulled = run(["docker", "pull", ref], capture_output=True)
        if pulled.returncode != 0:
            raise StepError(f"docker pull {ref} failed: {pulled.stderr.strip()[-400:]}")
        # The digest the registry served for the tag now; RepoDigests may also list a digest an
        # earlier pull left on this host, and that one would not be the image under test.
        digest = pulled_digest(ref, pulled.stdout)
        inspect = run(
            ["docker", "image", "inspect", "--format", "{{.Id}}", ref], capture_output=True
        )
        if inspect.returncode != 0:
            raise StepError(f"docker image inspect {ref} failed")
        image_id = inspect.stdout.strip()
        at = repository_of(ref) + "@" + digest
        mount = ["-v", f"{key}:/key.pub:ro"] if key is not None else []
        passed = False
        for cmd in cosign_commands(at, key, repository):
            res = run(
                ["docker", "run", "--rm", "--network", "host", *mount, COSIGN_IMAGE, *cmd],
                capture_output=True,
            )
            if res.returncode == 0:
                passed = True
                break
        if not passed:
            raise StepError(f"{at} carries no valid signature (cosign verify failed)")
        say(f"verified {at}")
        out.append(Verified(ref, digest, image_id.strip()))
    return out


def compare_running(
    verified: Sequence[Verified], containers: Mapping[str, tuple[str, str]]
) -> None:
    """Fail unless the containers run exactly the verified images.

    Args:
        verified: The verified images.
        containers: Service name to (configured image reference, image ID) of every container.

    Raises:
        StepError: A container of a verified repository runs another image ID, or a verified
            image runs in no container.
    """
    by_ref = {v.ref: v for v in verified}
    used: set[str] = set()
    for service, (ref, image_id) in sorted(containers.items()):
        if ref not in by_ref:
            if any(repository_of(ref) == repository_of(v.ref) for v in verified):
                raise StepError(f"{service} runs {ref}, which is not a verified reference")
            continue
        if image_id != by_ref[ref].image_id:
            raise StepError(f"{service} runs {image_id}, not the verified image of {ref}")
        used.add(ref)
    missing = sorted(set(by_ref) - used)
    if missing:
        raise StepError(f"no container runs the verified {', '.join(missing)}")


# ---- one installation ------------------------------------------------------------------------


def clean_env(environ: Mapping[str, str]) -> dict[str, str]:
    """The environment without BOTHY_*, COMPOSE_*, OTEL_* and TS_AUTHKEY (as smoke.sh does)."""
    drop = ("BOTHY_", "COMPOSE_", "OTEL_")
    return {k: v for k, v in environ.items() if not k.startswith(drop) and k != "TS_AUTHKEY"}


def enrollment_code(stdout: str) -> str:
    """The enrollment code install.sh printed once on stdout.

    Raises:
        StepError: There is not exactly one code (the message never repeats the output).
    """
    found = [m.group(0) for m in ENROLLMENT.finditer(stdout)]
    if len(set(found)) != 1:
        raise StepError(f"install.sh printed {len(set(found))} enrollment codes, want one")
    return found[0]


def env_value(env_file: Path, key: str) -> str:
    """The value of ``KEY=`` in an env file, or ""."""
    for line in env_file.read_text(encoding="utf-8").splitlines():
        if line.startswith(key + "="):
            return line.split("=", 1)[1]
    return ""


def write_secret(path: Path, text: str) -> None:
    """Write a file of mode 0600 that only the probe reads."""
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as fh:
        fh.write(text)


def listening(proc_net: str) -> list[tuple[str, int]]:
    """The listening sockets of ``/proc/net/tcp`` or ``tcp6`` text as (address, port).

    The address is hex as the kernel writes it: ``0100007F`` is 127.0.0.1, the IPv6 loopback
    ``00000000000000000000000001000000``.
    """
    out = []
    for line in proc_net.splitlines()[1:]:
        cols = line.split()
        if len(cols) > 3 and cols[3] == "0A":
            addr, _, port = cols[1].partition(":")
            out.append((addr, int(port, 16)))
    return out


def is_loopback(addr: str) -> bool:
    """Whether a hex address of /proc/net/tcp(6) is loopback: 127.0.0.0/8 or ::1.

    IPv4 is written as one little-endian word, so the first octet is the last byte
    (``0B00007F`` is 127.0.0.11, the embedded DNS of Docker).
    """
    if len(addr) == 8:
        return addr[6:8].upper() == "7F"
    return addr.upper() == "00000000000000000000000001000000"


# The server's listeners in the namespace of the tailscale sidecar (16 4.2): the public one on
# loopback only (api-tailscale sets server.listen_addr to 127.0.0.1:8080), the internal one on all
# addresses, because Prometheus and Alertmanager reach it over the Compose network; on a tailnet
# the tailnet policy of R-OP-03 admits port 443 only.
PUBLIC_PORT = 8080
INTERNAL_PORT = 8081


def check_tailnet(proc_tcp: str, public: int = PUBLIC_PORT, internal: int = INTERNAL_PORT) -> None:
    """In the namespace of the sidecar the public listener is loopback only.

    Every listener beyond loopback must be the internal listener; both listeners must exist.

    Raises:
        StepError: Another port listens beyond loopback, or a listener of the server is missing.
    """
    socks = listening(proc_tcp)
    exposed = sorted({p for a, p in socks if not is_loopback(a)} - {internal})
    if exposed:
        raise StepError(f"port(s) {exposed} listen beyond loopback in the tailscale namespace")
    if not any(p == public and is_loopback(a) for a, p in socks):
        raise StepError(f"the public listener does not listen on loopback port {public}")
    if not any(p == internal for _, p in socks):
        raise StepError(f"the internal listener does not listen on port {internal}")


@dataclass
class Installation:
    """One installation of the run: its folder, project and environment."""

    workdir: Path
    storage: str
    project: str
    env: dict[str, str]
    run: Runner = _run
    profiles: str = ""
    log: list[str] = field(default_factory=list)

    @property
    def folder(self) -> Path:
        """The folder install.sh installs into (its default ./bothy in the working directory)."""
        return self.workdir / "bothy"

    @property
    def compose_dir(self) -> Path:
        """The Compose folder of the bundle layout (16 3.1 "As built")."""
        return self.folder / "infra" / "compose"

    def compose(
        self, *args: str, check: bool = True, **kwargs: Any
    ) -> subprocess.CompletedProcess[str]:
        """``docker compose`` in the Compose folder with the project of this run."""
        env = dict(self.env)
        if self.profiles:
            env["COMPOSE_PROFILES"] = self.profiles
        res = self.run(["docker", "compose", *args], cwd=self.compose_dir, env=env, **kwargs)
        if check and res.returncode != 0:
            err = (res.stderr or "").strip()[-600:] if isinstance(res.stderr, str) else ""
            raise StepError(f"docker compose {args[0]} failed (exit {res.returncode}) {err}")
        return res

    def diagnose(self) -> None:
        """Print ``docker compose ps --all`` and the end of the logs of every service."""
        say(f"FAILED: state of the installation {self.project} in {self.compose_dir}")
        if not self.compose_dir.is_dir():
            say("the installation folder does not exist (the run failed before install.sh)")
            return
        print("docker compose ps --all:", file=sys.stderr, flush=True)
        self.compose("ps", "--all", check=False)
        print(f"last {LOG_TAIL} log lines of every service:", file=sys.stderr, flush=True)
        self.compose("logs", "--no-color", "--tail", str(LOG_TAIL), check=False)

    def down(self) -> list[str]:
        """Remove the containers, volumes and networks of the project; return leftovers.

        Every profile of compose.yaml is active for the removal: `down` removes only the volumes
        of the services it knows, and the run started services of several profiles (the overlay
        and the tailnet check).
        """
        # Without the folder install.sh never ran; the label check below still looks.
        if self.compose_dir.is_dir():
            listed = self.compose("config", "--profiles", check=False, capture_output=True)
            if listed.returncode == 0 and listed.stdout.split():
                self.profiles = ",".join(listed.stdout.split())
            self.compose(
                "down", "--volumes", "--remove-orphans", "--timeout", "60", check=False,
                capture_output=True,
            )  # fmt: skip
        label = f"label=com.docker.compose.project={self.project}"
        left = []
        for kind in ("ps -a", "volume ls", "network ls"):
            res = self.run(
                ["docker", *kind.split(), "--filter", label, "--format", "{{.Names}}"]
                if kind == "ps -a"
                else ["docker", *kind.split(), "--filter", label, "--format", "{{.Name}}"],
                capture_output=True,
            )
            left += [x for x in res.stdout.split() if x]
        return left

    def published_port(self) -> int:
        """The host port Docker chose for the public listener of ``api`` (published as 0).

        Raises:
            StepError: Compose names no port.
        """
        res = self.compose("port", "api", "8080", capture_output=True)
        port = res.stdout.strip().splitlines()[0].rpartition(":")[2] if res.stdout.strip() else ""
        if not port.isdigit():
            raise StepError("docker compose port api 8080 named no host port")
        return int(port)

    def running_images(self) -> dict[str, tuple[str, str]]:
        """Service name to (configured image, image ID) of every container of the project."""
        res = self.compose("ps", "--all", "--format", "json", capture_output=True)
        rows = _json_rows(res.stdout)
        out = {}
        for row in rows:
            cid = row.get("ID", "")
            inspect = self.run(
                ["docker", "inspect", "--format", "{{.Config.Image}} {{.Image}}", cid],
                capture_output=True,
            )
            if inspect.returncode != 0:
                raise StepError(f"docker inspect of {row.get('Service')} failed")
            ref, _, image_id = inspect.stdout.strip().partition(" ")
            out[f"{row.get('Service')}/{row.get('Name')}"] = (ref, image_id)
        return out


def _json_rows(text: str) -> list[dict[str, Any]]:
    """Rows of ``docker compose ps --format json``: one object per line or one array."""
    text = text.strip()
    if not text:
        return []
    if text.startswith("["):
        rows: list[dict[str, Any]] = json.loads(text)
        return rows
    return [json.loads(line) for line in text.splitlines() if line.strip()]


# ---- the run ---------------------------------------------------------------------------------


@dataclass
class Context:
    """What the steps of the whole test share."""

    iface: Interface
    workdir: Path
    release: Release
    probe: str
    key: Path | None
    environ: Mapping[str, str]
    run: Runner = _run
    canary: str = field(default_factory=lambda: "prt" + secrets.token_hex(12))
    reports: list[str] = field(default_factory=list)


def build_probe(run: Runner, tag: str) -> None:
    """Build the probe of M1-73 (target smoke of services/server/Dockerfile) as `tag`."""
    version = (ROOT / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    res = run(
        [
            "docker", "build", "--quiet", "--target", "smoke", "--file",
            "services/server/Dockerfile", "--build-arg", f"VERSION={version}", "--tag", tag, ".",
        ],
        cwd=ROOT, capture_output=True,
    )  # fmt: skip
    if res.returncode != 0:
        raise StepError(f"building the probe failed: {res.stderr.strip()[-800:]}")


def probe_args(ctx: Context, out: Path, *args: str, network: str = "host") -> list[str]:
    """``docker run`` of the probe with the run's output folder at /out and the canary."""
    return [
        "docker", "run", "--rm", "--network", network, "--user", f"{os.getuid()}:{os.getgid()}",
        "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
        "-v", f"{out}:/out", "-e", f"SMOKE_PUBLIC_URL={PUBLIC_URL}", "-e",
        f"SMOKE_CANARY={ctx.canary}", ctx.probe, *args,
    ]  # fmt: skip


def install(ctx: Context, inst: Installation) -> str:
    """Run install.sh as the source of the interface says; return the enrollment code.

    Raises:
        StepError: install.sh failed or printed not exactly one code.
    """
    bundle = ctx.release.bundle if ctx.iface.source == "release-download" else None
    args = install_args(read_options(), inst.storage, bundle)
    if ctx.iface.source == "url":
        if ctx.iface.repository is None:
            raise StepError("the source url needs GITHUB_REPOSITORY")
        url = release_base(ctx.iface.repository, ctx.iface.tag) + "install.sh"
        cmd = ["sh", "-c", 'curl -fsSL "$0" | sh -s -- "$@"', url, *args]
    else:
        cmd = ["sh", str(ctx.release.script), *args]
    say(f"install ({inst.storage}): install.sh {' '.join(args)}")
    stdout = inst.workdir / "install.out"
    fd = os.open(stdout, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as fh:
        res = inst.run(
            cmd, cwd=inst.workdir, env=inst.env, stdin=subprocess.DEVNULL, stdout=fh,
            timeout=INSTALL_TIMEOUT_S, start_new_session=True,
        )  # fmt: skip
    if res.returncode != 0:
        raise StepError(f"install.sh ended with exit code {res.returncode}")
    code = enrollment_code(stdout.read_text(encoding="utf-8"))
    stdout.unlink()
    return code


def enable_local_overlay(inst: Installation) -> None:
    """Switch the overlay local on: the observability profile and the exporter local.

    Only the shell environment of this run changes (Compose prefers it over .env), so the
    installation's files stay as install.sh wrote them. Traces are kept in full, so the scan
    sees every span of the run.
    """
    profiles = env_value(inst.compose_dir / ".env", "COMPOSE_PROFILES")
    inst.profiles = f"{profiles},observability"
    inst.env["BOTHY_OTEL_EXPORTER"] = "local"
    inst.env["BOTHY_TRACE_SAMPLE_PERCENT"] = "100"
    inst.compose(
        "up", "--detach", "--wait", "--wait-timeout", "600", *LOCAL_STACK, capture_output=True
    )


@contextmanager
def background(
    cmd: Sequence[str], log: Path, env: Mapping[str, str] | None = None
) -> Iterator[subprocess.Popen[bytes]]:
    """Start a process with its output in log; stop it by PID at the end."""
    with log.open("ab") as fh:
        proc = subprocess.Popen(  # nosec B603
            list(cmd), stdout=fh, stderr=subprocess.STDOUT, env=dict(env) if env else None,
            cwd=ROOT,
        )  # fmt: skip
    try:
        yield proc
    finally:
        if proc.poll() is None:
            proc.send_signal(signal.SIGTERM)
            try:
                proc.wait(timeout=60)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=30)


def wait_for(path: Path, proc: subprocess.Popen[bytes], seconds: float) -> None:
    """Wait until path exists, while proc runs (poll interval, the bound is seconds)."""
    deadline = time.monotonic() + seconds
    while not path.exists():
        if proc.poll() is not None:
            raise StepError(f"the probe ended (exit {proc.returncode}) before {path.name}")
        if time.monotonic() > deadline:
            raise StepError(f"{path.name} did not appear within {seconds:.0f}s")
        time.sleep(1)


def scenarios(ctx: Context, inst: Installation, code: str, port: int) -> None:
    """The smoke scenario of M1-73, then the portability scenario with the test adapter."""
    out = inst.workdir / "out"
    out.mkdir(mode=0o700)
    write_secret(out / "setup.out", code + "\n")
    base = f"http://127.0.0.1:{port}"
    say("smoke scenario of M1-73 (owner, space, channel, thread, message, WebSocket)")
    res = ctx.run(probe_args(ctx, out, "run", "--base-url", base, "--out", "/out"))
    if res.returncode != 0:
        raise StepError("the smoke scenario failed")
    issued = inst.compose(
        "exec", "-T", "api", "bothy", "setup", "issue-enrollment-code", "--user", OWNER_HANDLE,
        capture_output=True,
    )  # fmt: skip
    write_secret(out / "portability-setup.out", enrollment_code(issued.stdout) + "\n")
    word = "portability" + secrets.token_hex(6)
    say("portability scenario (test adapter run, upload clean and EICAR, search, status page)")
    logs = inst.workdir / "scan" / "logs"
    logs.mkdir(parents=True, mode=0o700)
    probe = probe_args(
        ctx, out, "portability", "--base-url", base, "--out", "/out", "--search-word", word
    )
    probe_log = inst.workdir / "probe.log"
    try:
        with background(probe, probe_log) as proc:
            wait_for(out / "adapter" / "agent.json", proc, ADAPTER_WAIT_S)
            adapter = [
                "uv", "run", "--project", "sdk/python", "--locked", "--quiet", "python",
                str(ADAPTER), "--dir", str(out / "adapter"), "--origin", base, "--public-url",
                PUBLIC_URL,
            ]  # fmt: skip
            env = {**ctx.environ, "PYTHONPATH": str(ROOT / "sdk" / "python" / "src")}
            with background(adapter, logs / "adapter.log", env):
                rc = proc.wait(timeout=STEP_TIMEOUT_S)
    finally:
        # The probe logs "ok <step>" and the failed step, never a value (M1-73); the adapter
        # logs no token and no content (SDK rule), so both are shown in full.
        for log in (probe_log, logs / "adapter.log"):
            if log.is_file():
                sys.stderr.write(f"--- {log.name}\n" + log.read_text(encoding="utf-8"))
    if rc != 0:
        raise StepError("the portability scenario failed")


def backup_kinds(storage: str) -> tuple[str, ...]:
    """The backups the run triggers before the restore test (16 8.2).

    With the driver ``s3`` the object backup is a mirror into the off-site bucket; the
    installation of the test has no off-site target (the options file names none), so there is no
    object backup to trigger, and the restore test skips its file sample (``objects_driver_s3``).
    """
    return ("pg_full", "logical", "objects") if storage == "fs" else ("pg_full", "logical")


def backup_and_restore(inst: Installation) -> None:
    """Trigger the backups of `backup_kinds`, then the restore test (16 8.3)."""
    for kind in backup_kinds(inst.storage):
        inst.compose("exec", "-T", "backup", "bothy-backupd", "backup", kind, capture_output=True)
        say(f"backup {kind}: ok")
    inst.compose("exec", "-T", "backup", "bothy-backupd", "restore-test", capture_output=True)
    say("restore test: every check passed or was skipped by rule")


def telemetry_scan(ctx: Context, inst: Installation, since: int) -> None:
    """Export the local stack, scan it with the container logs, require telemetry."""
    scan = inst.workdir / "scan"
    shutil.copytree(inst.workdir / "out", scan / "out", ignore=shutil.ignore_patterns("adapter"))
    logs = inst.compose("logs", "--no-color", "--timestamps", capture_output=True)
    (scan / "logs" / "compose.log").write_text(logs.stdout + logs.stderr, encoding="utf-8")
    config = json.loads(inst.compose("config", "--format", "json", capture_output=True).stdout)
    network = config["networks"]["internal"]["name"]
    say(f"telemetry through the overlay local and canary scan (network {network})")
    res = ctx.run(
        probe_args(ctx, scan, "local-scan", "--in", "/out", "--since", str(since), network=network)
    )
    if res.returncode != 0:
        raise StepError("the telemetry check or the canary scan failed")


def tailnet_check(inst: Installation) -> None:
    """The tailnet case without an account (Design note 3).

    The sidecar and the server start without a login; in their shared namespace the public
    listener listens on loopback only, so on a tailnet only the port that ``tailscale serve``
    exposes reaches the API; the internal listener (health and incoming webhooks of the stack) is
    the only other listener, and the tailnet policy of R-OP-03 admits port 443 only.
    """
    profiles = inst.profiles
    inst.profiles = profiles + ",tailscale"
    try:
        inst.compose(
            "up", "--detach", "--wait", "--wait-timeout", "300", "--no-deps", "tailscale",
            "api-tailscale", capture_output=True,
        )  # fmt: skip
        tcp = inst.compose(
            "exec", "-T", "tailscale", "cat", "/proc/net/tcp", "/proc/net/tcp6", capture_output=True
        )
        check_tailnet(tcp.stdout)
        say(
            "tailnet case: the public listener listens on loopback only, beyond loopback only the"
            " internal listener"
        )
    finally:
        inst.compose(
            "rm",
            "--stop",
            "--force",
            "tailscale",
            "api-tailscale",
            check=False,
            capture_output=True,
        )
        inst.profiles = profiles


def unsigned_log_lines(log: str) -> list[str]:
    """Lines of Grafana's log about an unsigned plugin: they say unsigned and name a plugin ID."""
    return [
        line
        for line in log.splitlines()
        if "unsigned" in line.lower() and _PLUGIN_ID_FIELD.search(line)
    ]


def _panel_types(panels: object) -> list[str]:
    """Every panel type of a dashboard, also the panels inside rows."""
    found: list[str] = []
    for panel in panels if isinstance(panels, list) else []:
        if not isinstance(panel, dict):
            continue
        if panel.get("type") != "row":
            found.append(str(panel.get("type")))
        found += _panel_types(panel.get("panels"))
    return found


def check_grafana(
    panel_plugins: Sequence[Mapping[str, Any]],
    errors: Sequence[Mapping[str, Any]],
    dashboard: Mapping[str, Any],
    log: str,
) -> list[str]:
    """VZ-T13 over the answers of Grafana's API and its log; return the unsigned log lines.

    Args:
        panel_plugins: the answer of ``/api/plugins?type=panel``.
        errors: the answer of ``/api/plugins/errors`` (plugins Grafana refused to load).
        dashboard: the answer of ``/api/dashboards/uid/bothy-live``.
        log: the log of the ``grafana`` service.

    Raises:
        StepError: the plugin is not loaded or not unsigned, Grafana reports a plugin error, the
            dashboard is not provisioned or uses a panel type that is not installed, the log names
            another plugin or none as unsigned, or the dashboard provisioning logged an error.
    """
    loaded = {str(p.get("id")): p for p in panel_plugins}
    plugin = loaded.get(GRAFANA_PLUGIN)
    if plugin is None:
        raise StepError(f"Grafana: {GRAFANA_PLUGIN} is not loaded")
    if plugin.get("signature") != "unsigned":
        raise StepError(
            f"Grafana: {GRAFANA_PLUGIN} has the signature {plugin.get('signature')!r}, not unsigned"
        )
    if errors:
        named = ", ".join(f"{e.get('pluginId')} ({e.get('errorCode')})" for e in errors)
        raise StepError(f"Grafana reports plugin errors: {named}")
    board = dashboard.get("dashboard") or {}
    if board.get("uid") != GRAFANA_DASHBOARD or not (dashboard.get("meta") or {}).get(
        "provisioned"
    ):
        raise StepError(f"Grafana: the dashboard {GRAFANA_DASHBOARD} is not provisioned")
    types = _panel_types(board.get("panels"))
    if not types:
        raise StepError(f"Grafana: the dashboard {GRAFANA_DASHBOARD} has no panel")
    missing = sorted({t for t in types if t not in loaded})
    if missing:
        raise StepError(f"Grafana: the dashboard {GRAFANA_DASHBOARD} uses {missing}, not loaded")
    lines = unsigned_log_lines(log)
    ids = {m.group(1) for line in lines for m in _PLUGIN_ID_FIELD.finditer(line)}
    if not ids:
        raise StepError("Grafana's log names no unsigned plugin")
    if ids != {GRAFANA_PLUGIN}:
        raise StepError(
            f"Grafana's log names other unsigned plugins: {sorted(ids - {GRAFANA_PLUGIN})}"
        )
    failed = [
        line
        for line in log.splitlines()
        if "level=error" in line and "logger=provisioning.dashboard" in line
    ]
    if failed:
        raise StepError(f"Grafana: provisioning.dashboard logged an error: {failed[0][:300]}")
    return lines


def grafana_check(inst: Installation) -> list[str]:
    """VZ-T13: start Grafana of the overlay, ask its API on loopback, read its log.

    Returns:
        The log lines about unsigned plugins (for the report).

    Raises:
        StepError: Grafana did not answer within ``GRAFANA_WAIT_S`` or `check_grafana` failed.
    """
    inst.compose(
        "up", "--detach", "--wait", "--wait-timeout", "300", "--no-deps", "grafana",
        capture_output=True,
    )  # fmt: skip

    def api(path: str, *, check: bool = True) -> subprocess.CompletedProcess[str]:
        return inst.compose(
            "exec", "-T", "grafana", "sh", "-c", GRAFANA_API, "sh", path,
            check=check, capture_output=True,
        )  # fmt: skip

    deadline = time.monotonic() + GRAFANA_WAIT_S
    while api("/api/health", check=False).returncode != 0:
        if time.monotonic() > deadline:
            raise StepError(f"Grafana did not answer within {GRAFANA_WAIT_S}s")
        time.sleep(2)

    def answer(path: str) -> Any:
        try:
            return json.loads(api(path).stdout)
        except ValueError as exc:
            raise StepError(f"Grafana {path} did not answer JSON") from exc

    panels = answer("/api/plugins?type=panel")
    errors = answer("/api/plugins/errors")
    dashboard = answer(f"/api/dashboards/uid/{GRAFANA_DASHBOARD}")
    api(f"/public/plugins/{GRAFANA_PLUGIN}/module.js")
    log = inst.compose("logs", "--no-color", "grafana", capture_output=True)
    return check_grafana(panels, errors, dashboard, log.stdout)


def run_storage(ctx: Context, storage: str) -> None:
    """One installation with storage: verify, install, compare, scenarios, backup, scan."""
    workdir = ctx.workdir / storage
    workdir.mkdir()
    project = f"bothy-portability-{secrets.token_hex(4)}"
    env = clean_env(ctx.environ)
    env["COMPOSE_PROJECT_NAME"] = project
    env.update(dict.fromkeys(PORT_VARIABLES, "0"))
    inst = Installation(workdir, storage, project, env, ctx.run)
    started = time.monotonic()
    kept = False
    try:
        say(f"run {storage}: pull and verify the images of {ctx.release.registry}")
        verified = verify_images(ctx.release, ctx.key, ctx.iface.repository, ctx.run)
        code = install(ctx, inst)
        if ctx.iface.fail_after_install:
            raise StepError(
                "forced failure after the installation (PORTABILITY_TRIAL_FAIL_AFTER_INSTALL)"
            )
        compare_running(verified, inst.running_images())
        say(f"run {storage}: the containers run exactly the verified images")
        enable_local_overlay(inst)
        since = int(time.time()) - 5
        compare_running(verified, inst.running_images())
        scenarios(ctx, inst, code, inst.published_port())
        backup_and_restore(inst)
        telemetry_scan(ctx, inst, since)
        unsigned = grafana_check(inst)
        say(f"VZ-T13: Grafana loaded {GRAFANA_PLUGIN} unsigned, dashboard {GRAFANA_DASHBOARD} ok")
        ctx.reports += [f"{storage}: grafana: {line}" for line in unsigned]
        if storage == "fs":
            tailnet_check(inst)
        took = time.monotonic() - started
        for v in verified:
            ctx.reports.append(f"{storage}: {repository_of(v.ref)}@{v.digest}")
        ctx.reports.append(f"{storage}: passed in {took:.0f}s")
    except (StepError, subprocess.SubprocessError, OSError) as exc:
        say(f"run {storage}: {exc}")
        inst.diagnose()
        if ctx.iface.keep:
            kept = True
            say(f"PORTABILITY_TRIAL_KEEP=1: kept {project} in {inst.compose_dir}")
            raise KeptError(f"run {storage} failed and was kept: {exc}") from exc
        raise StepError(f"run {storage} failed: {exc}") from exc
    finally:
        if not kept:
            left = inst.down()
            shutil.rmtree(workdir, ignore_errors=True)
            if left:
                raise StepError(f"cleanup left objects of {project} behind: {left}")


def preflight(run: Runner = _run) -> None:
    """Docker with Compose must be reachable; uv runs the test adapter."""
    for tool in ("docker", "uv", "curl", "sh"):
        if shutil.which(tool) is None:
            raise InterfaceError(f"{tool} is not on PATH")
    if run(["docker", "info", "--format", "{{.ServerVersion}}"], capture_output=True).returncode:
        raise InterfaceError("no Docker runtime is reachable; start Docker or set DOCKER_HOST")


def main(environ: Mapping[str, str] | None = None, run: Runner = _run) -> int:
    """Run the test; see the module documentation."""
    environ = os.environ if environ is None else environ
    try:
        iface = read_interface(environ)
        preflight(run)
    except InterfaceError as exc:
        say(str(exc))
        return 2
    key = iface.trial_key or (REPO_KEY if REPO_KEY.is_file() else None)
    workdir = Path(tempfile.mkdtemp(prefix="bothy-portability-"))
    probe = f"bothy-portability-probe:{secrets.token_hex(4)}"
    kept = False
    try:
        release = fetch_release(iface, workdir)
        names = ", ".join(release.images)
        say(f"{iface.tag}: server {release.server} from {release.registry}, images {names}")
        build_probe(run, probe)
        ctx = Context(iface, workdir, release, probe, key, environ, run)
        for storage in iface.storages:
            run_storage(ctx, storage)
    except KeptError as exc:
        kept = True
        say(f"FAIL: {exc}; the temporary directory {workdir} is kept")
        return 1
    except (StepError, subprocess.SubprocessError, OSError) as exc:
        say(f"FAIL: {exc}")
        return 1
    finally:
        if not kept:
            run(["docker", "image", "rm", "--force", probe], capture_output=True)
            shutil.rmtree(workdir, ignore_errors=True)
    say("PASS")
    for line in ctx.reports:
        say(f"  {line}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
