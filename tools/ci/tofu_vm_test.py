# SPDX-License-Identifier: Apache-2.0
"""Local VM test of the OpenTofu module generic-vm: `make tofu-vm-test` (brief M7-15).

Proves the host setup of the module on a real Linux host without any cloud cost (D-W-125,
D-W-127, OQ-09):

1. the product trial of `make release-trial` for the product on this host: the pinned registry
   on 127.0.0.1, the server images, then bundle and install.sh of the product, here also signed
   with the throwaway key of the trial (SBOMs, `.sigstore.json` per file, signed SHA256SUMS),
   served by a static HTTP server on 127.0.0.1;
2. the user data rendered by `tofu apply` of the test root infra/tofu/test/generic-vm with the
   download address and the public key of that trial;
3. a pinned Ubuntu cloud image (verified by SHA-256) booted in QEMU with user networking and a
   NoCloud seed from the same static server (KVM or HVF where available, TCG otherwise); a test
   only cloud-config part forwards 127.0.0.1:<registry port> and 127.0.0.1:<server port> inside
   the VM to the host (192.0.2.2: QEMU's user network moved to TEST-NET-1), so the bundle's
   registry and download address work unchanged inside the VM, and the Docker Engine there
   pulls from a loopback registry, which it accepts over plain HTTP without an
   insecure-registries entry;
4. after cloud-init: the smoke probe of M1-73 (`bothy-smoke run`) against the stack through an
   SSH port forward, with a fresh enrollment code of the owner;
5. a second delivery of the same setup over SSH (`provision_over_ssh = true`, `tofu apply`) that
   must change nothing on the host, then a plan without changes;
6. two release copies that must stop the setup before anything is unpacked: a bundle with a
   wrong checksum, and the same bundle with a SHA256SUMS that matches it but not its signature.

The VM, the SSH processes and the agent are stopped by PID, the registry and the builder by the
trial, and the temporary directory with the VM disk is removed at the end, also after a failure
(`--keep` keeps the directory). Only the verified base image stays in .cache/tofu-vm. The test
refuses to run under GitHub Actions.

Usage:
  tofu_vm_test.py --tofu <path> --cosign <path> --syft <path> [--platforms linux/arm64] [--keep]
Exit codes: 0 passed, 1 a step failed, 2 refused or a precondition is missing.
"""

from __future__ import annotations

import argparse
import base64
import email.mime.multipart
import email.mime.text
import functools
import hashlib
import http.server
import json
import os
import platform
import secrets
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request
from collections.abc import Iterator, Sequence
from contextlib import ExitStack, contextmanager
from dataclasses import dataclass, field
from pathlib import Path

import product_release
import release_chain as chain
import release_trial

REPO_ROOT = Path(__file__).resolve().parents[2]
TEST_ROOT = REPO_ROOT / "infra" / "tofu" / "test" / "generic-vm"
CACHE = REPO_ROOT / ".cache" / "tofu-vm"
# Ubuntu 24.04 LTS server cloud image of a dated release directory, pinned by SHA-256 from its
# SHA256SUMS (https://cloud-images.ubuntu.com/releases/noble/release-20261017/, retrieved
# 2026-10-20). A dated directory, not "current": the bytes behind the URL never change.
IMAGE_BASE = "https://cloud-images.ubuntu.com/releases/noble/release-20261017/"
CLOUD_IMAGES = {
    "arm64": (
        "ubuntu-24.04-server-cloudimg-arm64.img",
        "50e24e1fb79e89116e939d90aa7738f7af48c442fa1bc2510131e0b39c50348e",
    ),
    "amd64": (
        "ubuntu-24.04-server-cloudimg-amd64.img",
        "04e8f2461a16a44e78a370284471f29616cdd9baf1893df25807b71f985b6c94",
    ),
}
QEMU = {"arm64": "qemu-system-aarch64", "amd64": "qemu-system-x86_64"}
PUBLIC_HOSTNAME = "chat.example.com"
OWNER = "alex"
SSH_USER = "bothy"
STACK = "/srv/bothy"
# QEMU user networking with its network moved to the documentation range TEST-NET-1 (RFC 5737)
# instead of its default private range: the host's loopback as seen from the guest.
GUEST_NET = "192.0.2.0/24"
HOST_FROM_GUEST = "192.0.2.2"
VM_MEMORY_MB = 6144
VM_CPUS = 4
BOOT_TIMEOUT_S = 1800
STEP_TIMEOUT_S = 1800


class VMTestError(Exception):
    """A step of the VM test failed."""


class PreconditionError(VMTestError):
    """A tool or a host capability the test needs is missing."""


def guest_arch(platforms: Sequence[str]) -> str:
    """The architecture of the VM: the one platform of the trial, as Debian names it.

    Raises:
        PreconditionError: not exactly one platform, or one without a cloud image.
    """
    if len(platforms) != 1:
        raise PreconditionError("the VM test runs one platform; pass --platforms linux/<arch>")
    arch = platforms[0].removeprefix("linux/")
    if arch not in CLOUD_IMAGES:
        raise PreconditionError(f"no cloud image for {platforms[0]}")
    return arch


def host_platform(machine: str | None = None) -> str:
    """The Linux platform of this host's CPU, the default of --platforms."""
    m = (machine or platform.machine()).lower()
    return "linux/arm64" if m in ("arm64", "aarch64") else "linux/amd64"


def accelerator(arch: str, system: str | None = None, machine: str | None = None) -> str:
    """Pick hvf on macOS and kvm on Linux when the guest has the host's architecture, else tcg."""
    same = host_platform(machine) == f"linux/{arch}"
    system = system or platform.system()
    if same and system == "Darwin":
        return "hvf"
    if same and system == "Linux" and os.access("/dev/kvm", os.R_OK | os.W_OK):
        return "kvm"
    return "tcg"


def firmware(qemu: str, arch: str) -> tuple[Path, Path] | None:
    """UEFI code and variables template of aarch64 next to the QEMU binary; None for amd64.

    Raises:
        PreconditionError: the firmware of aarch64 is missing.
    """
    if arch != "arm64":
        return None
    share = Path(qemu).resolve().parents[1] / "share" / "qemu"
    code, varstore = share / "edk2-aarch64-code.fd", share / "edk2-arm-vars.fd"
    if not code.is_file() or not varstore.is_file():
        raise PreconditionError(f"UEFI firmware of aarch64 is missing in {share}")
    return code, varstore


def qemu_command(  # noqa: PLR0913, PLR0917 - one flag group per argument
    qemu: str,
    arch: str,
    accel: str,
    disk: Path,
    ssh_port: int,
    seed_url: str,
    console: Path,
    uefi: tuple[Path, Path] | None,
) -> list[str]:
    """The QEMU command line: user networking with SSH forwarded on 127.0.0.1 only."""
    cmd = [qemu, "-name", "bothy-tofu-vm", "-accel", accel]
    if arch == "arm64":
        cmd += ["-machine", "virt", "-cpu", "host" if accel != "tcg" else "max"]
    else:
        cmd += ["-machine", "q35", "-cpu", "host" if accel != "tcg" else "max"]
    if uefi is not None:
        cmd += [
            "-drive", f"if=pflash,format=raw,readonly=on,file={uefi[0]}",
            "-drive", f"if=pflash,format=raw,file={uefi[1]}",
        ]  # fmt: skip
    cmd += [
        "-smp", str(VM_CPUS), "-m", str(VM_MEMORY_MB),
        "-drive", f"if=virtio,format=qcow2,file={disk}",
        "-netdev", f"user,id=n0,net={GUEST_NET},host={HOST_FROM_GUEST},"
                   f"hostfwd=tcp:127.0.0.1:{ssh_port}-:22",
        "-device", "virtio-net-pci,netdev=n0",
        "-smbios", f"type=1,serial=ds=nocloud;s={seed_url}",
        "-display", "none", "-monitor", "none", "-serial", f"file:{console}",
    ]  # fmt: skip
    return cmd


def forward_config(ports: Sequence[int]) -> str:
    """The test-only cloud-config part: 127.0.0.1:<port> inside the VM goes to the host.

    route_localnet lets the kernel route a loopback destination after DNAT; the masquerade gives
    the packet the address of the VM, so the replies come back.
    """
    plist = ", ".join(str(p) for p in ports)
    table = (
        "table ip bothy_vm_test {\n"
        "  chain out { type nat hook output priority -100; "
        f"ip daddr 127.0.0.1 tcp dport {{ {plist} }} dnat to {HOST_FROM_GUEST}; }}\n"
        "  chain post { type nat hook postrouting priority 100; "
        'ip saddr 127.0.0.1 oifname != "lo" masquerade; }\n'
        "}\n"
    )
    doc = {
        "bootcmd": [
            ["sysctl", "-qw", "net.ipv4.conf.all.route_localnet=1"],
            ["sh", "-c", f"printf '%s' '{table}' | nft -f -"],
        ]
    }
    return "#cloud-config\n" + json.dumps(doc, indent=2) + "\n"


def seed_user_data(module_user_data: str, ports: Sequence[int]) -> str:
    """MIME multipart user data: the test part, then the module's user data unchanged."""
    msg = email.mime.multipart.MIMEMultipart()
    for text in (forward_config(ports), module_user_data):
        msg.attach(email.mime.text.MIMEText(text, "cloud-config", "utf-8"))
    return msg.as_string()


def snapshot_script(stack: str = STACK) -> str:
    """Shell commands that print the state a second setup run must not change.

    Contents are hashed, the secrets directory is listed by name, size, mode and time only, so no
    value is ever printed.
    """
    compose = f"{stack}/infra/compose"
    return "\n".join(
        [
            "set -e",
            "cd /",
            "sha256sum /etc/bothy/setup.env /etc/bothy/install-options /etc/bothy/firewall.nft"
            " /etc/bothy/release.pub /etc/systemd/system/bothy.service"
            " /etc/systemd/system/bothy-firewall.service /etc/apt/sources.list.d/docker.list"
            f" /etc/apt/keyrings/docker.asc {compose}/.env {stack}/.bothy-install"
            " /usr/local/lib/bothy/cosign-*",
            f"find {compose}/secrets -printf '%P %s %m %T@\\n' | sort",
            "dpkg-query -W -f '${Package} ${Version}\\n'"
            " docker-ce docker-compose-plugin containerd.io",
            f"id {SSH_USER}",
            "systemctl is-enabled bothy.service bothy-firewall.service",
            "nft list table inet bothy_edge",
            "docker volume ls --format '{{.Name}}' | sort",
            "docker ps -aq | xargs docker inspect --format"
            " '{{.Name}} {{.Id}} {{if .State.Running}}{{.State.StartedAt}}{{end}}' | sort",
        ]
    )


def snapshot_diff(before: str, after: str) -> list[str]:
    """Lines that differ between two snapshots ('-' before, '+' after)."""
    a, b = before.splitlines(), after.splitlines()
    return [f"- {x}" for x in a if x not in b] + [f"+ {x}" for x in b if x not in a]


def _connects(port: int) -> bool:
    """Whether 127.0.0.1:port accepts a TCP connection."""
    with socket.create_connection(("127.0.0.1", port), 5):
        return True


def free_port() -> int:
    """A free TCP port on 127.0.0.1 (released before QEMU or ssh binds it)."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def sha256_file(path: Path) -> str:
    """SHA-256 of a file, read in blocks."""
    digest = hashlib.sha256()
    with path.open("rb") as fh:
        for block in iter(lambda: fh.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def cloud_image(arch: str, cache: Path = CACHE) -> Path:
    """The pinned cloud image, downloaded once into the cache and verified on every use.

    Raises:
        VMTestError: the download does not match its pinned SHA-256.
    """
    name, want = CLOUD_IMAGES[arch]
    path = cache / name
    if path.is_file() and sha256_file(path) == want:
        return path
    cache.mkdir(parents=True, exist_ok=True)
    part = path.with_suffix(".part")
    print(f"tofu-vm-test: downloading {IMAGE_BASE}{name}")
    with urllib.request.urlopen(IMAGE_BASE + name, timeout=120) as resp, part.open("wb") as fh:  # noqa: S310  # nosec B310
        shutil.copyfileobj(resp, fh, 1 << 20)
    got = sha256_file(part)
    if got != want:
        part.unlink()
        raise VMTestError(f"{name} has SHA-256 {got}, pinned {want}")
    part.replace(path)
    return path


class _QuietHandler(http.server.SimpleHTTPRequestHandler):
    """Static files of the test (release copies and the seed) without a request log."""

    def log_message(self, format: str, *args: object) -> None:
        return


@contextmanager
def static_server(root: Path) -> Iterator[int]:
    """Serve `root` on 127.0.0.1 with a random port; yield the port."""
    handler = functools.partial(_QuietHandler, directory=str(root))
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield int(server.server_address[1])
    finally:
        server.shutdown()
        server.server_close()


@dataclass
class Processes:
    """Background processes of the test, stopped by PID at the end."""

    procs: list[subprocess.Popen[bytes]] = field(default_factory=list)

    def start(self, cmd: Sequence[str], log: Path, env: dict[str, str] | None = None) -> int:
        """Start a process with its output in `log`; return its PID."""
        with log.open("ab") as fh:
            proc = subprocess.Popen(  # nosec B603
                list(cmd), stdout=fh, stderr=subprocess.STDOUT, env=env
            )
        self.procs.append(proc)
        return proc.pid

    def stop(self) -> None:
        """Terminate every process (the newest first), then kill what is left."""
        for proc in reversed(self.procs):
            if proc.poll() is None:
                proc.terminate()
        for proc in reversed(self.procs):
            try:
                proc.wait(timeout=60)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=30)


@dataclass
class VM:
    """SSH access to the test VM."""

    key: Path
    port: int
    known_hosts: Path

    def ssh_args(self) -> list[str]:
        """Options of every ssh call: the throwaway key and known_hosts of this run only."""
        return [
            "-i", str(self.key), "-p", str(self.port),
            "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes",
            "-o", "StrictHostKeyChecking=accept-new",
            "-o", f"UserKnownHostsFile={self.known_hosts}",
            "-o", "ConnectTimeout=10", "-o", "LogLevel=ERROR",
        ]  # fmt: skip

    def run(
        self, script: str, *, check: bool = True, timeout: float = STEP_TIMEOUT_S, tty: bool = False
    ) -> subprocess.CompletedProcess[str]:
        """Run a shell script as root in the VM.

        The script travels base64 encoded in the command and runs from a file with standard
        input closed, so no command of it can read the rest of the script. On a failure only
        standard error is reported: standard output may carry a value (an enrollment code).
        """
        b64 = base64.b64encode(script.encode("utf-8")).decode("ascii")
        remote = (
            f's=$(mktemp) && printf %s {b64} | base64 -d >"$s" && sudo sh "$s" </dev/null;'
            ' rc=$?; rm -f "$s"; exit $rc'
        )
        proc = subprocess.run(  # nosec B603
            ["ssh", *self.ssh_args(), *(["-tt"] if tty else []), f"{SSH_USER}@127.0.0.1", remote],
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        if check and proc.returncode != 0:
            tail = proc.stderr.strip().splitlines()[-20:]
            raise VMTestError(f"in the VM (exit {proc.returncode}): " + "\n".join(tail))
        return proc


class Tofu:
    """The test root with a temporary data directory and state."""

    def __init__(self, tofu: str, workdir: Path) -> None:
        """Use the tofu binary `tofu` with its data and state under `workdir`."""
        self.tofu = tofu
        self.data = workdir / "tofu-data"
        self.state = workdir / "tofu.tfstate"
        self.data.mkdir()

    def __call__(
        self, *args: str, env: dict[str, str] | None = None, ok: Sequence[int] = (0,)
    ) -> subprocess.CompletedProcess[str]:
        """Run tofu in the test root; raise VMTestError on an exit code outside `ok`."""
        full = {**os.environ, "TF_DATA_DIR": str(self.data), "TF_IN_AUTOMATION": "1", **(env or {})}
        proc = subprocess.run(  # nosec B603
            [self.tofu, f"-chdir={TEST_ROOT}", *args],
            env=full,
            capture_output=True,
            text=True,
            check=False,
        )
        if proc.returncode not in ok:
            raise VMTestError(f"tofu {args[0]}: {(proc.stderr or proc.stdout).strip()[-2000:]}")
        return proc

    def apply(self, values: dict[str, object], env: dict[str, str] | None = None) -> str:
        """`tofu apply` with the values; return its output."""
        return self(
            "apply",
            "-auto-approve",
            "-input=false",
            "-no-color",
            f"-state={self.state}",
            *var_args(values),
            env=env,
        ).stdout

    def output(self, name: str) -> str:
        """One output of the state as raw text."""
        return self("output", f"-state={self.state}", "-raw", name).stdout

    def unchanged(self, values: dict[str, object], env: dict[str, str] | None = None) -> bool:
        """Whether a plan with the values has no changes (-detailed-exitcode 0)."""
        proc = self(
            "plan",
            "-detailed-exitcode",
            "-input=false",
            "-no-color",
            f"-state={self.state}",
            *var_args(values),
            env=env,
            ok=(0, 2),
        )
        return proc.returncode == 0


def var_args(values: dict[str, object]) -> list[str]:
    """-var arguments; lists, maps and booleans as JSON."""
    out = []
    for key, value in values.items():
        text = value if isinstance(value, str) else json.dumps(value)
        out += ["-var", f"{key}={text}"]
    return out


def tamper(good: Path, bad_sum: Path, bad_sig: Path, bundle: str) -> None:
    """Write two release copies that must fail.

    A changed bundle (checksum), and the changed bundle with a SHA256SUMS that matches it, under
    the old signature (signature).
    """
    for target in (bad_sum, bad_sig):
        shutil.copytree(good, target)
        with (target / bundle).open("ab") as fh:
            fh.write(b"\0")
    sums = bad_sig / chain.CHECKSUMS
    lines = []
    for line in sums.read_text(encoding="utf-8").splitlines():
        digest, name = line.split(None, 1)
        if name.lstrip("*") == bundle:
            digest = sha256_file(bad_sig / bundle)
        lines.append(f"{digest}  {name}\n")
    sums.write_text("".join(lines), encoding="utf-8")


def negative_probe(vm: VM, url: str, expect: str) -> str:
    """Run the setup with another download address and stack directory; it must stop.

    Returns the message of the setup. Raises VMTestError when it passes, names another reason,
    or leaves the stack directory behind (something was unpacked).
    """
    script = f"""set -u
d=$(mktemp -d)
for f in setup.env install-options firewall.nft release.pub; do cp "/etc/bothy/$f" "$d/"; done
sed -i "s|^SETUP_BASE_URL=.*|SETUP_BASE_URL='{url}'|" "$d/setup.env"
sed -i "s|^SETUP_STACK_DIR=.*|SETUP_STACK_DIR='/srv/bothy-neg'|" "$d/setup.env"
BOTHY_SETUP_DIR="$d" /usr/local/lib/bothy/bothy-setup >"$d/out" 2>&1
echo "rc=$?"
if [ -e /srv/bothy-neg ]; then echo stackdir=present; else echo stackdir=absent; fi
grep 'bothy-setup: verify' "$d/out" || true
rm -rf "$d"
"""
    out = vm.run(script).stdout
    if "rc=1" not in out or "stackdir=absent" not in out or expect not in out:
        raise VMTestError(f"the setup with {url} did not stop as expected: {out.strip()}")
    return next(line for line in out.splitlines() if "bothy-setup: verify" in line)


@dataclass
class Report:
    """Durations and results printed at the end."""

    lines: list[str] = field(default_factory=list)
    started: float = field(default_factory=time.monotonic)

    def step(self, text: str) -> None:
        """Record and print one result with the time since the start."""
        line = f"[{time.monotonic() - self.started:7.1f}s] {text}"
        self.lines.append(line)
        print(f"tofu-vm-test: {line}", flush=True)


def wait_for(what: str, probe: object, timeout_s: float, interval_s: float = 5) -> None:
    """Call probe until it returns true; raise VMTestError after the timeout."""
    deadline = time.monotonic() + timeout_s
    while True:
        try:
            if probe():  # type: ignore[operator]
                return
        except (subprocess.SubprocessError, OSError, VMTestError):
            pass
        if time.monotonic() > deadline:
            raise VMTestError(f"timed out after {timeout_s:.0f}s waiting for {what}")
        time.sleep(interval_s)


def preflight(arch: str, tofu: str) -> tuple[str, str]:
    """Check the tools of this host; return the QEMU and qemu-img paths.

    Raises:
        PreconditionError: a tool is missing.
    """
    found = {
        name: shutil.which(name)
        for name in (
            QEMU[arch],
            "qemu-img",
            "ssh",
            "ssh-keygen",
            "ssh-agent",
            "ssh-add",
            "docker",
            "go",
        )
    }
    missing = [name for name, path in found.items() if path is None]
    if missing:
        raise PreconditionError(f"missing on PATH: {', '.join(missing)} (QEMU: brew install qemu)")
    if not Path(tofu).is_file():
        raise PreconditionError(f"tofu not found at {tofu}")
    return str(found[QEMU[arch]]), str(found["qemu-img"])


def build_probe(workdir: Path) -> Path:
    """Build the smoke probe of M1-73 for this host."""
    out = workdir / "bothy-smoke"
    subprocess.run(  # nosec B603
        ["go", "build", "-o", str(out), "./test/smoke/cmd/bothy-smoke"],
        cwd=REPO_ROOT / "services" / "server",
        check=True,
    )
    return out


def release(
    trial: release_trial.Trial, tools: chain.Tools, www: Path, port: int, pub: Path
) -> tuple[str, list[Path]]:
    """Build, sign and verify bundle and install.sh into www/good.

    The build is the product trial of `make release-trial`, the signing uses the trial key like
    `release.yml` (job product-signing).

    Returns:
        The product version and the written files.
    """
    repo = trial.product_repo()
    good = www / "good"
    good.mkdir(parents=True)
    product_version = (repo / "VERSION").read_text(encoding="utf-8").strip()
    env = {
        **os.environ,
        "RELEASE_TRIAL_REGISTRY": trial.registry,
        "RELEASE_TRIAL_BASE_URL": f"http://127.0.0.1:{port}/good/",
    }
    env.pop("GITHUB_ACTIONS", None)
    try:
        written = product_release.build(repo, product_version, good, env, docker=tools.docker)
    except product_release.ProductReleaseError as exc:
        raise VMTestError(f"release-build of the product: {exc}") from exc
    chain.make_sboms(tools, good, product_version, [], written, [])
    chain.sign(tools, "private", good, product_version, [], written, [])
    chain.verify(tools, "private", good, [], key=pub)
    return product_version, written


@dataclass
class Context:
    """What the steps of one run share."""

    args: argparse.Namespace
    workdir: Path
    tools: chain.Tools
    procs: Processes
    report: Report
    arch: str
    port: int = 0
    registry: str = ""
    version: str = ""
    values: dict[str, object] = field(default_factory=dict)
    vm: VM | None = None

    def need_vm(self) -> VM:
        """The VM of the run (set by step_trial)."""
        if self.vm is None:
            raise VMTestError("the VM is not set up yet")
        return self.vm


def step_trial(ctx: Context, stack: ExitStack, www: Path) -> None:
    """The trial release with the product signed (Design note 2, first sentence)."""
    trial = release_trial.Trial(ctx.tools, ctx.workdir / "trial-work", ctx.args.platforms)
    trial.workdir.mkdir()
    ctx.registry = stack.enter_context(trial.local_registry())
    stack.enter_context(trial.builder_instance())
    ctx.report.step(f"trial: server {trial.version} for {ctx.arch}, registry {ctx.registry}")
    images = trial.build()
    pub = trial.sign_and_verify(images)
    trial.tag_and_check(images)
    ctx.version, written = release(trial, ctx.tools, www, ctx.port, pub)
    tamper(www / "good", www / "bad-sum", www / "bad-sig", product_release.bundle_name(ctx.version))
    ctx.report.step(f"trial: product {ctx.version} signed and verified, {len(written)} files")
    key = ctx.workdir / "id_ed25519"
    keygen = ["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "vm-test", "-f", str(key)]
    subprocess.run(keygen, check=True)  # nosec B603
    ctx.vm = VM(key, free_port(), ctx.workdir / "known_hosts")
    ctx.values = {
        "bothy_version": ctx.version,
        "image_registry": ctx.registry,
        "public_hostname": PUBLIC_HOSTNAME,
        "release_repository": "alex/bothy",
        "release_base_url": f"http://127.0.0.1:{ctx.port}/good/",
        "release_public_key": pub.read_text(encoding="utf-8"),
        "owner_handle": OWNER,
        "owner_name": "Alex",
        "virus_scan": "off",
        "admin_ssh_public_keys": [key.with_suffix(".pub").read_text(encoding="utf-8").strip()],
        "ssh_host": "127.0.0.1",
        "ssh_port": ctx.vm.port,
        # The forwarded port is not the port of sshd inside the VM, which the firewall admits.
        "sshd_port": 22,
    }


def step_boot(ctx: Context, tofu: Tofu, www: Path) -> None:
    """Render the user data, boot the pinned image with the seed, wait for cloud-init."""
    vm = ctx.need_vm()
    qemu, qemu_img = preflight(ctx.arch, ctx.args.tofu)
    accel = accelerator(ctx.arch)
    tofu("init", "-backend=false", "-input=false", "-no-color")
    tofu.apply(ctx.values)
    user_data = tofu.output("user_data")
    (ctx.workdir / "user-data.module.yaml").write_text(user_data, encoding="utf-8")
    seed = www / "seed"
    seed.mkdir()
    ports = [int(ctx.registry.rsplit(":", 1)[1]), ctx.port]
    (seed / "user-data").write_text(seed_user_data(user_data, ports), encoding="utf-8")
    meta = "instance-id: bothy-tofu-vm\nlocal-hostname: bothy-vm\n"
    (seed / "meta-data").write_text(meta, encoding="utf-8")
    (seed / "vendor-data").write_text("#cloud-config\n{}\n", encoding="utf-8")
    disk = ctx.workdir / "disk.qcow2"
    image = cloud_image(ctx.arch)
    create = [qemu_img, "create", "-q", "-f", "qcow2", "-F", "qcow2", "-b", str(image)]
    subprocess.run([*create, str(disk), "20G"], check=True)  # nosec B603
    uefi = firmware(qemu, ctx.arch)
    if uefi is not None:
        varstore = ctx.workdir / "uefi-vars.fd"
        shutil.copyfile(uefi[1], varstore)
        uefi = (uefi[0], varstore)
    seed_url = f"http://{HOST_FROM_GUEST}:{ctx.port}/seed/"
    console = ctx.workdir / "console.log"
    cmd = qemu_command(qemu, ctx.arch, accel, disk, vm.port, seed_url, console, uefi)
    boot = time.monotonic()
    pid = ctx.procs.start(cmd, ctx.workdir / "qemu.log")
    ctx.report.step(f"vm: QEMU {ctx.arch} with {accel}, pid {pid}, SSH 127.0.0.1:{vm.port}")
    wait_for("SSH of the VM", lambda: vm.run("true", timeout=30).returncode == 0, BOOT_TIMEOUT_S)
    ctx.report.step(f"vm: SSH up after {time.monotonic() - boot:.0f}s")
    status = vm.run("cloud-init status --wait --long", check=False, timeout=BOOT_TIMEOUT_S)
    log = vm.run("tail -n 80 /var/log/cloud-init-output.log", check=False).stdout
    (ctx.workdir / "cloud-init-output.tail").write_text(log, encoding="utf-8")
    if status.returncode not in (0, 2) or "status: done" not in status.stdout:
        print(log, file=sys.stderr)
        raise VMTestError(f"cloud-init: {status.stdout.strip()} {status.stderr.strip()}")
    full = vm.run("cat /var/log/cloud-init-output.log", check=False).stdout
    if "Enrollment code (shown once)" in full:
        raise VMTestError("the enrollment code reached the cloud-init log")
    ctx.report.step(f"vm: cloud-init done after {time.monotonic() - boot:.0f}s (setup ran)")


def step_smoke(ctx: Context) -> None:
    """The smoke probe of M1-73 through an SSH forward of the published port (edge direct)."""
    vm = ctx.need_vm()
    env_file = f"{STACK}/infra/compose/.env"
    publish = vm.run(f"sed -n 's/^BOTHY_HTTP_PUBLISH_PORT=//p' {env_file}").stdout.strip()
    local = free_port()
    forward = f"127.0.0.1:{local}:127.0.0.1:{publish or '8090'}"
    tunnel = ["ssh", *vm.ssh_args(), "-N", "-L", forward, f"{SSH_USER}@127.0.0.1"]
    ctx.procs.start(tunnel, ctx.workdir / "tunnel.log")
    out = ctx.workdir / "smoke-out"
    out.mkdir(mode=0o700)
    setup_out = out / "setup.out"
    setup_out.touch(mode=0o600)
    # A new code of the owner: install.sh withheld the first one from the log (D-M7-15-4).
    issue = (
        f"cd {STACK}/infra/compose && runuser -u {SSH_USER} -- docker compose exec -T api"
        f" bothy setup issue-enrollment-code --user {OWNER}"
    )
    setup_out.write_text(vm.run(issue).stdout, encoding="utf-8")
    probe = build_probe(ctx.workdir)
    wait_for("the SSH forward", lambda: _connects(local), 60, 1)
    cmd = [str(probe), "run", "--base-url", f"http://127.0.0.1:{local}"]
    cmd += ["--public-url", f"https://{PUBLIC_HOSTNAME}", "--out", str(out)]
    canary = {"SMOKE_CANARY": "vm" + secrets.token_hex(8)}
    subprocess.run(cmd, env={**os.environ, **canary}, check=True, timeout=600)  # nosec B603
    ctx.report.step("smoke: bothy-smoke run passed against the stack in the VM")


def step_second_delivery(ctx: Context, tofu: Tofu) -> str:
    """The same setup over SSH must change nothing; the next plan must be empty."""
    vm = ctx.need_vm()
    before = vm.run(snapshot_script()).stdout
    sock = ctx.workdir / "agent.sock"
    ctx.procs.start(["ssh-agent", "-D", "-a", str(sock)], ctx.workdir / "agent.log")
    wait_for("ssh-agent", sock.exists, 30, 0.5)
    agent = {"SSH_AUTH_SOCK": str(sock)}
    subprocess.run(["ssh-add", "-q", str(vm.key)], env={**os.environ, **agent}, check=True)
    again = {**ctx.values, "provision_over_ssh": True}
    (ctx.workdir / "ssh-apply.log").write_text(tofu.apply(again, env=agent), encoding="utf-8")
    diff = snapshot_diff(before, vm.run(snapshot_script()).stdout)
    if diff:
        raise VMTestError("the second delivery changed the host:\n" + "\n".join(diff))
    if not tofu.unchanged(again, env=agent):
        raise VMTestError("a plan after the second apply has changes")
    ctx.report.step("idempotence: the SSH delivery changed nothing; the next plan is empty")
    return before


TTY_PROBE = "sh -c 'if (: </dev/tty) 2>/dev/null; then echo TTY; else echo NOTTY; fi'"


def step_no_terminal(ctx: Context) -> None:
    """Check that install.sh runs without a terminal under an SSH pseudo terminal.

    Under the pseudo terminal of an SSH session (as remote-exec runs the delivery), the function
    `detached` of the setup leaves install.sh without a terminal, so the enrollment code cannot
    reach /dev/tty (D-M7-15-4); without it the probe sees the terminal.
    """
    vm = ctx.need_vm()
    script = (
        "eval \"$(sed -n '/^detached() {/,/^}/p' /usr/local/lib/bothy/bothy-setup)\"\n"
        f"echo plain=$({TTY_PROBE})\n"
        f"echo detached=$(detached {TTY_PROBE})\n"
    )
    out = vm.run(script, tty=True).stdout
    if "plain=TTY" not in out or "detached=NOTTY" not in out:
        raise VMTestError(f"the terminal probe did not show the detachment: {out.strip()}")
    ctx.report.step("no terminal: under an SSH PTY install.sh runs without /dev/tty")


def step_negative(ctx: Context, before: str) -> None:
    """A wrong checksum and a wrong signature stop the setup before anything is unpacked."""
    vm = ctx.need_vm()
    for name, expect in (("bad-sum", "checksum"), ("bad-sig", "signature of SHA256SUMS")):
        line = negative_probe(vm, f"http://127.0.0.1:{ctx.port}/{name}/", expect)
        ctx.report.step(f"negative: {name}: {line.strip()}")
    if snapshot_diff(before, vm.run(snapshot_script()).stdout):
        raise VMTestError("a stopped setup changed the host")
    ctx.report.step("negative: nothing unpacked, the running stack unchanged")


BOOT_ID = "cat /proc/sys/kernel/random/boot_id"


def step_reboot(ctx: Context) -> None:
    """Check that the units bring the host back after a reboot.

    The firewall table loads without an ordering cycle in the journal, and bothy.service starts
    the stack (discovery answers 200).
    """
    vm = ctx.need_vm()
    first = vm.run(BOOT_ID).stdout.strip()
    vm.run("systemctl reboot", check=False, timeout=30)
    wait_for("the VM after the reboot", lambda: vm.run(BOOT_ID, timeout=30).stdout.strip()
             not in ("", first), BOOT_TIMEOUT_S)  # fmt: skip
    env_file = f"{STACK}/infra/compose/.env"
    script = f"""set -u
publish=$(sed -n 's/^BOTHY_HTTP_PUBLISH_PORT=//p' {env_file})
url="http://127.0.0.1:${{publish:-8090}}/api/v1/instance/discovery"
code=none
for i in $(seq 1 90); do
  code=$(curl -s -o /dev/null -w '%{{http_code}}' "$url" || true)
  [ "$code" = 200 ] && break
  sleep 5
done
echo "discovery=$code"
echo "service=$(systemctl is-active bothy.service)"
echo "firewall=$(systemctl is-active bothy-firewall.service)"
nft list table inet bothy_edge >/dev/null 2>&1 && echo table=loaded
echo "cycles=$(journalctl -b 0 --no-pager | grep -ci 'ordering cycle')"
"""
    out = vm.run(script, timeout=STEP_TIMEOUT_S).stdout
    want = ("discovery=200", "service=active", "firewall=active", "table=loaded", "cycles=0")
    if not all(w in out for w in want):
        raise VMTestError(f"after the reboot: {out.strip()}")
    ctx.report.step("reboot: firewall loaded without an ordering cycle, the stack is back")


def run(args: argparse.Namespace) -> int:
    """The whole test; see the module documentation."""
    arch = guest_arch(args.platforms)
    preflight(arch, args.tofu)
    cloud_image(arch)
    workdir = Path(tempfile.mkdtemp(prefix="bothy-tofu-vm-"))
    tools = chain.Tools(cosign=args.cosign, syft=args.syft)
    docker_config = workdir / "docker-config"
    docker_config.mkdir()
    (docker_config / "config.json").write_text("{}\n", encoding="utf-8")
    # cosign and syft read registry credentials from DOCKER_CONFIG; an empty one keeps every
    # credential of this host out of the test, as in the trial.
    tools.env["DOCKER_CONFIG"] = str(docker_config)
    ctx = Context(args, workdir, tools, Processes(), Report(), arch)
    try:
        with ExitStack() as stack:
            stack.callback(ctx.procs.stop)
            www = workdir / "www"
            www.mkdir()
            ctx.port = stack.enter_context(static_server(www))
            step_trial(ctx, stack, www)
            tofu = Tofu(args.tofu, workdir)
            step_boot(ctx, tofu, www)
            step_smoke(ctx)
            before = step_second_delivery(ctx, tofu)
            step_no_terminal(ctx)
            step_negative(ctx, before)
            step_reboot(ctx)
            vm = ctx.need_vm()
            vm.run("poweroff", check=False, timeout=30)
    finally:
        if args.keep:
            print(f"tofu-vm-test: kept {workdir}")
        else:
            shutil.rmtree(workdir, ignore_errors=True)
    print("tofu-vm-test: passed")
    for line in ctx.report.lines:
        print(f"  {line}")
    return 0


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Local VM test of the module generic-vm.")
    parser.add_argument("--tofu", required=True)
    parser.add_argument("--cosign", required=True)
    parser.add_argument("--syft", required=True)
    parser.add_argument("--platforms", default=host_platform())
    parser.add_argument("--keep", action="store_true", help="keep the temporary directory")
    args = parser.parse_args(argv)
    try:
        release_trial.refuse_in_ci(os.environ)
        args.platforms = release_trial.parse_platforms(args.platforms)
    except release_trial.TrialError as exc:
        print(f"tofu-vm-test: {exc}", file=sys.stderr)
        return 2
    try:
        return run(args)
    except PreconditionError as exc:
        print(f"tofu-vm-test: {exc}", file=sys.stderr)
        return 2
    except (
        VMTestError,
        release_trial.TrialError,
        chain.ChainError,
        subprocess.SubprocessError,
    ) as exc:
        print(f"tofu-vm-test: FAIL: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
