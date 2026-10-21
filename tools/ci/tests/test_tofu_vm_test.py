# SPDX-License-Identifier: Apache-2.0
"""Unit tests of the helpers of the local VM test (tools/ci/tofu_vm_test.py, brief M7-15).

The VM run itself is `make tofu-vm-test` on the build host; these tests pin the pieces that
decide what it proves: the QEMU command, the seed, the snapshot comparison and the tampered
release copies.
"""

from __future__ import annotations

import email
import hashlib
import io
import json
import subprocess
import sys
import urllib.request
from pathlib import Path
from typing import Any

import pytest
import tofu_vm_test as vt


def test_guest_arch_needs_one_known_platform() -> None:
    assert vt.guest_arch(["linux/arm64"]) == "arm64"
    assert vt.guest_arch(["linux/amd64"]) == "amd64"
    with pytest.raises(vt.PreconditionError):
        vt.guest_arch(["linux/amd64", "linux/arm64"])
    with pytest.raises(vt.PreconditionError):
        vt.guest_arch(["linux/s390x"])


def test_host_platform_and_accelerator() -> None:
    assert vt.host_platform("arm64") == "linux/arm64"
    assert vt.host_platform("aarch64") == "linux/arm64"
    assert vt.host_platform("x86_64") == "linux/amd64"
    assert vt.accelerator("arm64", "Darwin", "arm64") == "hvf"
    assert vt.accelerator("amd64", "Darwin", "arm64") == "tcg"
    assert vt.accelerator("amd64", "Windows", "x86_64") == "tcg"


def test_firmware_of_aarch64(tmp_path: Path) -> None:
    qemu = tmp_path / "bin" / "qemu-system-aarch64"
    qemu.parent.mkdir()
    qemu.write_text("")
    assert vt.firmware(str(qemu), "amd64") is None
    with pytest.raises(vt.PreconditionError):
        vt.firmware(str(qemu), "arm64")
    share = tmp_path / "share" / "qemu"
    share.mkdir(parents=True)
    (share / "edk2-aarch64-code.fd").write_text("")
    (share / "edk2-arm-vars.fd").write_text("")
    assert vt.firmware(str(qemu), "arm64") == (
        share / "edk2-aarch64-code.fd",
        share / "edk2-arm-vars.fd",
    )


def test_qemu_command_binds_ssh_on_loopback_only(tmp_path: Path) -> None:
    uefi = (tmp_path / "code.fd", tmp_path / "vars.fd")
    cmd = vt.qemu_command(
        "qemu", "arm64", "hvf", tmp_path / "d.qcow2", 40022, "http://192.0.2.2:1/seed/",
        tmp_path / "c.log", uefi,
    )  # fmt: skip
    joined = " ".join(cmd)
    assert "user,id=n0,net=192.0.2.0/24,host=192.0.2.2,hostfwd=tcp:127.0.0.1:40022-:22" in joined
    assert "-cpu host" in joined
    assert "type=1,serial=ds=nocloud;s=http://192.0.2.2:1/seed/" in cmd
    assert f"if=pflash,format=raw,readonly=on,file={uefi[0]}" in cmd
    x86 = vt.qemu_command("qemu", "amd64", "tcg", tmp_path / "d", 1, "u", tmp_path / "c", None)
    assert "q35" in x86
    assert "max" in x86
    assert not [a for a in x86 if "pflash" in a]


def test_seed_user_data_keeps_the_module_part_unchanged() -> None:
    module = "#cloud-config\nruncmd:\n- - /usr/local/lib/bothy/bothy-setup\n"
    msg = email.message_from_string(vt.seed_user_data(module, [5000, 8000]))
    parts = [p.get_payload(decode=True).decode("utf-8") for p in msg.walk() if not p.is_multipart()]
    assert [p.get_content_type() for p in msg.walk() if not p.is_multipart()] == [
        "text/cloud-config",
        "text/cloud-config",
    ]
    assert parts[1] == module
    forward = json.loads(parts[0].removeprefix("#cloud-config\n"))
    script = forward["bootcmd"][1][2]
    assert "tcp dport { 5000, 8000 } dnat to 192.0.2.2" in script
    assert forward["bootcmd"][0] == ["sysctl", "-qw", "net.ipv4.conf.all.route_localnet=1"]


def test_snapshot_lists_no_secret_values() -> None:
    script = vt.snapshot_script("/srv/bothy")
    assert "sha256sum" in script
    assert "find /srv/bothy/infra/compose/secrets -printf '%P %s %m %T@\\n'" in script
    assert "cat " not in script
    assert vt.snapshot_diff("a\nb\n", "a\nb\n") == []
    assert vt.snapshot_diff("a\nb\n", "a\nc\n") == ["- b", "+ c"]


def test_var_args() -> None:
    assert vt.var_args({"a": "x", "b": True, "c": ["k"], "d": 2}) == [
        "-var", "a=x", "-var", "b=true", "-var", 'c=["k"]', "-var", "d=2",
    ]  # fmt: skip


def test_tamper_breaks_checksum_and_signature(tmp_path: Path) -> None:
    good = tmp_path / "good"
    good.mkdir()
    (good / "b.tar.gz").write_bytes(b"bundle")
    (good / "install.sh").write_bytes(b"script")
    sums = "".join(
        f"{hashlib.sha256((good / n).read_bytes()).hexdigest()}  {n}\n"
        for n in ("b.tar.gz", "install.sh")
    )
    (good / "SHA256SUMS").write_text(sums)
    vt.tamper(good, tmp_path / "bad-sum", tmp_path / "bad-sig", "b.tar.gz")
    assert (tmp_path / "bad-sum" / "b.tar.gz").read_bytes() == b"bundle\0"
    assert (tmp_path / "bad-sum" / "SHA256SUMS").read_text() == sums
    changed = (tmp_path / "bad-sig" / "SHA256SUMS").read_text().splitlines()
    assert changed[0] == hashlib.sha256(b"bundle\0").hexdigest() + "  b.tar.gz"
    assert changed[1] == sums.splitlines()[1]
    assert (good / "b.tar.gz").read_bytes() == b"bundle"


def test_cloud_image_is_verified(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    data = b"image"
    name = "x.img"
    monkeypatch.setitem(vt.CLOUD_IMAGES, "arm64", (name, hashlib.sha256(data).hexdigest()))
    monkeypatch.setattr(urllib.request, "urlopen", lambda url, timeout: io.BytesIO(data))
    path = vt.cloud_image("arm64", tmp_path)
    assert path.read_bytes() == data
    assert vt.cloud_image("arm64", tmp_path) == path
    monkeypatch.setitem(vt.CLOUD_IMAGES, "arm64", ("y.img", "0" * 64))
    with pytest.raises(vt.VMTestError, match="pinned"):
        vt.cloud_image("arm64", tmp_path)
    assert not (tmp_path / "y.part").exists()


def test_static_server_serves_files(tmp_path: Path) -> None:
    (tmp_path / "f").write_text("ok")
    with vt.static_server(tmp_path) as port:
        assert vt._connects(port)
        with urllib.request.urlopen(f"http://127.0.0.1:{port}/f", timeout=5) as resp:
            assert resp.read() == b"ok"


def test_processes_stop_by_pid(tmp_path: Path) -> None:
    procs = vt.Processes()
    pid = procs.start([sys.executable, "-c", "import time; time.sleep(60)"], tmp_path / "log")
    assert pid > 0
    procs.stop()
    assert all(p.poll() is not None for p in procs.procs)


def test_wait_for() -> None:
    calls = []

    def probe() -> bool:
        calls.append(1)
        if len(calls) < 2:
            raise OSError("not yet")
        return True

    # The deadline is far beyond any load: the probe succeeds on its second call either way.
    vt.wait_for("x", probe, 3600, 0)
    assert len(calls) == 2
    with pytest.raises(vt.VMTestError, match="timed out"):
        vt.wait_for("y", lambda: False, 0.05, 0.01)


def test_preflight_names_missing_tools(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.setattr(vt.shutil, "which", lambda name: None if name == "qemu-img" else name)
    with pytest.raises(vt.PreconditionError, match="qemu-img"):
        vt.preflight("arm64", str(tmp_path / "tofu"))
    monkeypatch.setattr(vt.shutil, "which", lambda name: name)
    with pytest.raises(vt.PreconditionError, match="tofu"):
        vt.preflight("arm64", str(tmp_path / "tofu"))
    (tmp_path / "tofu").write_text("")
    assert vt.preflight("arm64", str(tmp_path / "tofu")) == ("qemu-system-aarch64", "qemu-img")


class FakeVM(vt.VM):
    """A VM whose run returns a fixed output."""

    def __init__(self, out: str) -> None:
        super().__init__(Path("k"), 1, Path("h"))
        self.out = out
        self.scripts: list[str] = []

    def run(self, script: str, *, check: bool = True, timeout: float = 1) -> Any:
        self.scripts.append(script)
        return subprocess.CompletedProcess([], 0, self.out, "")


def test_negative_probe_needs_a_stop_before_unpacking() -> None:
    line = "bothy-setup: verify: the checksum of b.tar.gz does not match SHA256SUMS"
    vm = FakeVM(f"rc=1\nstackdir=absent\n{line}\n")
    assert vt.negative_probe(vm, "http://127.0.0.1:1/bad-sum/", "checksum") == line
    assert "SETUP_BASE_URL='http://127.0.0.1:1/bad-sum/'" in vm.scripts[0]
    for out in ("rc=0\nstackdir=absent\n" + line, "rc=1\nstackdir=present\n" + line):
        with pytest.raises(vt.VMTestError):
            vt.negative_probe(FakeVM(out), "u", "checksum")
    with pytest.raises(vt.VMTestError):
        vt.negative_probe(FakeVM(f"rc=1\nstackdir=absent\n{line}\n"), "u", "signature")


def test_vm_ssh_options_use_the_run_key_only() -> None:
    args = vt.VM(Path("/k"), 2222, Path("/h")).ssh_args()
    assert args[:4] == ["-i", "/k", "-p", "2222"]
    assert "UserKnownHostsFile=/h" in args
    assert "IdentitiesOnly=yes" in args


def test_main_refuses_under_github_actions(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("GITHUB_ACTIONS", "true")
    assert vt.main(["--tofu", "t", "--cosign", "c", "--syft", "s"]) == 2


def test_main_reports_a_missing_tool(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("GITHUB_ACTIONS", raising=False)
    monkeypatch.setattr(vt.shutil, "which", lambda name: None)
    argv = ["--tofu", "t", "--cosign", "c", "--syft", "s", "--platforms", "linux/arm64"]
    assert vt.main(argv) == 2


def test_report_and_context() -> None:
    report = vt.Report()
    report.step("one")
    assert report.lines[0].endswith("one")
    ctx = vt.Context(None, Path("w"), None, vt.Processes(), report, "arm64")  # type: ignore[arg-type]
    with pytest.raises(vt.VMTestError):
        ctx.need_vm()
