# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/distribution/windows.py with a recording runner instead of the Windows tools.

The real chain (flutter, makeappx, signtool, App Installer) runs in the CI job
desktop-trial-windows; these tests hold the package layout, the commands and the update test.
"""

from __future__ import annotations

import io
import re
import subprocess
import xml.etree.ElementTree as ET
import zipfile
from collections.abc import Sequence
from pathlib import Path
from typing import Any

import appdist
import common
import pytest
import windows
from fakes import FakeRun, ok

NS = {"m": "http://schemas.microsoft.com/appx/manifest/foundation/windows10"}


@pytest.fixture
def cfg() -> dict[str, Any]:
    template = appdist.load(appdist.TEMPLATE)
    return appdist.trial_distribution(
        "http://127.0.0.1:4711/", template["apple"]["sparkle"]["public_ed_key"]
    )


@pytest.fixture
def sdk(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    root = tmp_path / "sdk"
    for version in ("10.0.19041.0", "10.0.26100.0", "10.0.22621.0"):
        for tool in ("makeappx.exe", "signtool.exe"):
            path = root / version / "x64" / tool
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"")
    (root / "notaversion" / "x64").mkdir(parents=True)
    (root / "notaversion" / "x64" / "signtool.exe").write_bytes(b"")
    monkeypatch.setattr(windows, "SDK_BIN", root)
    return root


def test_sdk_tool_takes_the_newest_sdk(sdk: Path) -> None:
    assert windows.sdk_tool("signtool.exe", sdk) == sdk / "10.0.26100.0" / "x64" / "signtool.exe"
    with pytest.raises(common.StepError, match=r"no Windows SDK with nothing\.exe"):
        windows.sdk_tool("nothing.exe", sdk)


def test_icon_png_takes_the_largest_png() -> None:
    png = windows.icon_png()
    assert png.startswith(b"\x89PNG\r\n\x1a\n")


def test_icon_without_png(tmp_path: Path) -> None:
    ico = tmp_path / "x.ico"
    ico.write_bytes(b"\x00\x00\x01\x00\x00\x00")
    with pytest.raises(common.StepError, match="holds no PNG"):
        windows.icon_png(ico)


def _build_output(root: Path, *exes: str) -> Path:
    output = root / "Release"
    (output / "data").mkdir(parents=True)
    for exe in exes:
        (output / exe).write_bytes(b"MZ")
    (output / "vcruntime140.dll").write_bytes(b"dll")
    return output


def test_stage_lays_out_the_package(tmp_path: Path, cfg: dict[str, Any]) -> None:
    output = _build_output(tmp_path, "client.exe")
    system32 = tmp_path / "System32"
    system32.mkdir()
    for dll in windows.VC_RUNTIME:
        (system32 / dll).write_bytes(b"system " + dll.encode())
    branding = {"app_name": "ExampleChat", "url_scheme": "chat-example"}
    package = windows.stage(
        output, cfg, appdist.app_version("0.1.1"), branding, tmp_path / "pkg", system32
    )
    assert (package / "vcruntime140.dll").read_bytes() == b"dll"
    assert (package / "msvcp140.dll").read_bytes() == b"system msvcp140.dll"
    for logo in windows.LOGOS:
        assert (package / "Assets" / logo).read_bytes().startswith(b"\x89PNG")
    root = ET.fromstring((package / "AppxManifest.xml").read_text("utf-8"))  # noqa: S314
    identity = root.find("m:Identity", NS)
    assert identity is not None
    assert identity.get("Version") == "0.1.1.0"
    app = root.find("m:Applications/m:Application", NS)
    assert app is not None
    assert app.get("Executable") == "client.exe"


def test_stage_needs_one_executable_and_the_runtime(tmp_path: Path, cfg: dict[str, Any]) -> None:
    branding = {"app_name": "A", "url_scheme": "a"}
    version = appdist.app_version("0.1.0")
    two = _build_output(tmp_path / "two", "a.exe", "b.exe")
    with pytest.raises(common.StepError, match="expected one executable"):
        windows.stage(two, cfg, version, branding, tmp_path / "p1", tmp_path)
    one = _build_output(tmp_path / "one", "a.exe")
    with pytest.raises(common.StepError, match=r"msvcp140\.dll is neither in the build"):
        windows.stage(one, cfg, version, branding, tmp_path / "p2", tmp_path / "none")


def test_sign_with_and_without_a_timestamp(tmp_path: Path, sdk: Path) -> None:
    run = FakeRun()
    cert = windows.Certificate(pfx=tmp_path / "c.pfx", password="pw")
    windows.sign(run, tmp_path / "a.msix", cert)
    windows.sign(run, tmp_path / "a.msix", windows.Certificate(cert.pfx, "pw", "http://ts.test"))
    assert run.calls[0][1:5] == ["sign", "/fd", "SHA256", "/f"]
    assert "/tr" not in run.calls[0]
    assert run.calls[1][-5:] == ["/tr", "http://ts.test", "/td", "SHA256", str(tmp_path / "a.msix")]
    windows.verify(run, tmp_path / "a.msix")
    assert run.calls[-1][1:4] == ["verify", "/pa", "/v"]


def test_release_step(
    tmp_path: Path, cfg: dict[str, Any], sdk: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    client = tmp_path / "client"
    monkeypatch.setattr(windows, "CLIENT", client)
    monkeypatch.setattr(common, "CLIENT", client)
    system32 = tmp_path / "System32"
    system32.mkdir()
    for dll in windows.VC_RUNTIME:
        (system32 / dll).write_bytes(b"dll")
    monkeypatch.setattr(windows, "SYSTEM32", system32)

    def flutter(args: list[str], _: dict[str, Any]) -> None:
        _build_output(client / "build" / "windows" / "x64" / "runner", "client.exe")

    def makeappx(args: list[str], _: dict[str, Any]) -> None:
        Path(args[-1]).write_bytes(b"msix")

    run = FakeRun({"flutter": flutter, "makeappx.exe": makeappx})
    branding = tmp_path / "branding.yaml"
    branding.write_text(
        "app_name: ExampleChat\nurl_scheme: chat-example\nbundle_id: com.example.chat\n"
        "default_server: https://chat.example.com\n",
        encoding="utf-8",
    )
    plan = windows.Plan(
        cfg=cfg,
        version=appdist.app_version("0.1.1"),
        out=tmp_path / "out",
        branding=branding,
        certificate=windows.Certificate(tmp_path / "c.pfx", "secret-pw"),
    )
    work = tmp_path / "work"
    work.mkdir()
    result = windows.release_step(run, plan, work)
    assert result.msix == tmp_path / "out" / "com.example.chat_0.1.1.0_x64.msix"
    target = appdist.appinstaller_target(result.appinstaller.read_text("utf-8"))
    assert target.msix_uri == "http://127.0.0.1:4711/com.example.chat_0.1.1.0_x64.msix"
    assert [Path(c[0]).name for c in run.calls] == [
        "dart",
        "flutter",
        "dart",
        "makeappx.exe",
        "signtool.exe",
        "signtool.exe",
    ]


def test_build_without_output_fails(
    tmp_path: Path, cfg: dict[str, Any], monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(windows, "CLIENT", tmp_path)
    monkeypatch.setattr(common, "CLIENT", tmp_path)
    branding = tmp_path / "b.yaml"
    branding.write_text("app_name: X\n", encoding="utf-8")
    with pytest.raises(common.StepError, match="wrote no"):
        windows.build_app(FakeRun(), cfg, appdist.app_version("0.1.0"), branding, tmp_path)


def test_release_stops_without_the_certificate(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(windows.sys, "platform", "win32")
    run = FakeRun()
    with pytest.raises(common.StepError, match="WINDOWS_SIGNING_PFX, WINDOWS_SIGNING_PFX_PASSWORD"):
        windows.release(
            run,
            distribution=appdist.TEMPLATE,
            version="0.1.0",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            environ={},
        )
    assert run.calls == []
    environ = dict.fromkeys(windows.WINDOWS_SECRETS, "eA==")
    with pytest.raises(appdist.DistributionError, match="placeholder values"):
        windows.release(
            run,
            distribution=appdist.TEMPLATE,
            version="0.1.0",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            environ=environ,
        )


def test_windows_only(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.setattr(windows.sys, "platform", "darwin")
    assert windows.main(["release", "--version", "0.1.0", "--out", str(tmp_path)], FakeRun()) == 1
    assert windows.main(["trial", "--out", str(tmp_path)], FakeRun()) == 1


# ---- the trial ------------------------------------------------------------------------------


class AppInstaller(FakeRun):
    """Plays PowerShell with the Appx module against the served files."""

    def __init__(self, www: Path, *, broken: bool = False) -> None:
        super().__init__()
        self.www = www
        self.installed = ""
        self.scripts: list[str] = []
        self.broken = broken
        self.certificates = 0

    def __call__(self, cmd: Sequence[str | Path], **kw: Any) -> subprocess.CompletedProcess[str]:
        args = [str(c) for c in cmd]
        if args[0] != "powershell.exe":
            return super().__call__(cmd, **kw)
        script = args[-1]
        self.scripts.append(script)
        if "Import-Certificate" in script:
            self.certificates += 1
            if self.broken:
                if kw.get("check", True):
                    raise common.StepError("Import-Certificate failed")
                return ok(code=1)
            return ok()
        if "Remove-Item -Path Cert:" in script:
            return ok()
        if "Add-AppxPackage -AppInstallerFile" in script:
            url = re.search(r"-AppInstallerFile '([^']+)'", script).group(1)  # type: ignore[union-attr]
            text = (self.www / url.rsplit("/", 1)[-1]).read_text("utf-8")
            target = appdist.appinstaller_target(text)
            data = (self.www / target.msix_uri.rsplit("/", 1)[-1]).read_bytes()
            if data != _msix(target.version):
                if kw.get("check", True):
                    raise common.StepError("Add-AppxPackage failed")
                return ok(code=1)
            self.installed = target.version
            return ok("installed")
        if "Remove-AppxPackage" in script:
            self.installed = ""
            return ok()
        if "Get-AppxPackage -Name" in script:
            return ok(self.installed)
        raise AssertionError(script)


def _msix(version: str) -> bytes:
    """A stand-in package; the same bytes for the same version (fixed entry times, so the fake
    App Installer's comparison does not depend on the second the test reaches it)."""
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as archive:
        for name, data in (
            ("AppxManifest.xml", version.encode()),
            ("AppxSignature.p7x", b"signature of " + version.encode()),
        ):
            archive.writestr(zipfile.ZipInfo(name, date_time=(2026, 1, 1, 0, 0, 0)), data)
    return buf.getvalue()


def test_the_stand_in_package_is_the_same_for_the_same_version() -> None:
    assert _msix("0.1.1.0") == _msix("0.1.1.0")


def test_corrupt_signature_changes_only_the_signature() -> None:
    good = _msix("0.1.1.0")
    bad = windows.corrupt_signature(good)
    assert len(bad) == len(good)
    with zipfile.ZipFile(io.BytesIO(bad)) as archive:
        assert archive.read("AppxManifest.xml") == b"0.1.1.0"
        with pytest.raises(zipfile.BadZipFile):
            archive.read("AppxSignature.p7x")
    plain = io.BytesIO()
    with zipfile.ZipFile(plain, "w") as archive:
        archive.writestr("a", b"b")
    with pytest.raises(common.StepError, match="no AppxSignature"):
        windows.corrupt_signature(plain.getvalue())


def _releases(tmp_path: Path, cfg: dict[str, Any]) -> list[windows.Release]:
    out = []
    for version in windows.TRIAL_VERSIONS:
        v = appdist.app_version(version)
        msix = tmp_path / appdist.msix_name(cfg, v)
        msix.write_bytes(_msix(v.msix))
        beta = tmp_path / f"beta-{version}.appinstaller"
        beta.write_text(appdist.appinstaller(cfg, version=v, channel="beta"), "utf-8")
        out.append(windows.Release(v, msix, beta))
    return out


def test_update_test_installs_updates_refuses_and_promotes(
    tmp_path: Path, cfg: dict[str, Any]
) -> None:
    www = tmp_path / "www"
    www.mkdir()
    old, new = _releases(tmp_path, cfg)
    run = AppInstaller(www)
    checks = windows.update_test(run, cfg, old, new, www, "http://127.0.0.1:4711/")
    assert len(checks) == 4
    assert run.installed == "0.1.1.0"
    stable = appdist.appinstaller_target((www / "stable.appinstaller").read_text("utf-8"))
    assert stable.msix_uri.endswith("com.example.chat_0.1.1.0_x64.msix")
    installs = re.findall(r"-AppInstallerFile '([^']+)'", "\n".join(run.scripts))
    assert installs == [
        "http://127.0.0.1:4711/beta.appinstaller",
        "http://127.0.0.1:4711/beta.appinstaller",
        "http://127.0.0.1:4711/beta.appinstaller",
        "http://127.0.0.1:4711/stable.appinstaller",
    ]


def test_the_trial_certificate_leaves_every_store(tmp_path: Path) -> None:
    run = AppInstaller(tmp_path)
    with windows.trial_certificate(run, "CN=Example Publisher", tmp_path) as cert:
        assert cert.pfx == tmp_path / "test-publisher.p12"
    removed = [s for s in run.scripts if "Remove-Item" in s]
    assert len(removed) == 2
    thumbprint = re.search(r"\\([0-9A-F]{40}) ", removed[0])
    assert thumbprint is not None
    assert all(thumbprint.group(1) in s for s in removed)


def test_a_failed_import_still_removes_the_certificate(tmp_path: Path) -> None:
    broken = AppInstaller(tmp_path, broken=True)
    with (
        pytest.raises(common.StepError, match="Import-Certificate failed"),
        windows.trial_certificate(broken, "CN=X", tmp_path),
    ):
        pass
    assert len([s for s in broken.scripts if "Remove-Item" in s]) == 2


def test_trial_runs_two_builds_and_the_update_test(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(windows.sys, "platform", "win32")
    built: list[str] = []

    def release_step(run: Any, plan: windows.Plan, workdir: Path) -> windows.Release:
        built.append(plan.version.short)
        assert plan.cfg["windows"]["update_url"].startswith("http://127.0.0.1:")
        plan.out.mkdir(parents=True)
        return windows.Release(plan.version, plan.out / "x.msix", plan.out / "beta.appinstaller")

    monkeypatch.setattr(windows, "release_step", release_step)
    monkeypatch.setattr(windows, "update_test", lambda *a: ["updated"])
    run = AppInstaller(tmp_path)
    assert windows.main(["trial", "--out", str(tmp_path / "out")], run) == 0
    assert built == ["0.1.0", "0.1.1"]
    assert "updated" in (tmp_path / "out" / "trial-windows.json").read_text("utf-8")
    assert any("Remove-AppxPackage" in s for s in run.scripts)
