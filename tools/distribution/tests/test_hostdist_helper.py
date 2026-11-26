# SPDX-License-Identifier: Apache-2.0
"""Tests of the computer helper steps of tools/distribution/hostdist.py (brief CU-02).

The helpers are signed and packaged on the owner's own machines (D-CU-01-29, D-CU-02-10). These
tests hold the signing commands, the comparison of the designated requirement, the package with
the optional helper component and the script that sets `host_service.computer_use` when the
component is selected. The native CI jobs of host-platform.yml install the packages for real.
"""

from __future__ import annotations

import os
import plistlib
import re
import shutil
import subprocess
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Any

import common
import hostdist
import pytest
import windows
from fakes import FakeRun, ok

VERSION = hostdist.component_version()
SET_SCRIPT = hostdist.INFRA / "set-computer-use.sh"
SET_SCRIPT_PS1 = hostdist.INFRA / "windows" / "set-computer-use.ps1"
DR = 'designated => identifier "com.example.bothy-computer-helper" and anchor apple generic'


# ---- the configuration script ---------------------------------------------------------------


def _set(tmp_path: Path, text: str | None) -> tuple[int, str]:
    cfg = tmp_path / "config.yaml"
    if text is not None:
        cfg.write_text(text, encoding="utf-8")
    done = subprocess.run(
        ["sh", str(SET_SCRIPT), str(cfg)], capture_output=True, text=True, check=False
    )
    return done.returncode, cfg.read_text(encoding="utf-8") if cfg.exists() else ""


CASES = [
    # (before, after); None: the file does not exist yet.
    (None, "host_service:\n  computer_use: true\n"),
    ("", "host_service:\n  computer_use: true\n"),
    (
        "host_service:\n    session_users: [alex]\n",
        "host_service:\n    computer_use: true\n    session_users: [alex]\n",
    ),
    (
        "# settings\nhost_service:  # the host\n  # users\n  session_users:\n    - alex\n",
        "# settings\nhost_service:  # the host\n  computer_use: true\n  # users\n"
        "  session_users:\n    - alex\n",
    ),
    (
        "host_service:\n  computer_use: false\n  scrollback_bytes: 1024\n",
        "host_service:\n  computer_use: true\n  scrollback_bytes: 1024\n",
    ),
    (
        "host_service:\n  profiles:\n    - name: x\n      computer_use: no\n"
        "  computer_use: false\n",
        "host_service:\n  profiles:\n    - name: x\n      computer_use: no\n  computer_use: true\n",
    ),
    ("host_service:\n", "host_service:\n  computer_use: true\n"),
    ("# only a comment\n", "# only a comment\nhost_service:\n  computer_use: true\n"),
]


@pytest.mark.parametrize(("before", "after"), CASES)
def test_set_computer_use(tmp_path: Path, before: str | None, after: str) -> None:
    code, text = _set(tmp_path, before)
    assert code == 0
    assert text == after
    # A second run changes nothing.
    code, again = _set(tmp_path, text)
    assert code == 0
    assert again == after


def test_set_computer_use_keeps_the_mode_of_an_existing_file(tmp_path: Path) -> None:
    cfg = tmp_path / "config.yaml"
    cfg.write_text("host_service:\n  session_users: [alex]\n", encoding="utf-8")
    cfg.chmod(0o640)
    assert _set(tmp_path, None)[0] == 0
    assert cfg.stat().st_mode & 0o777 == 0o640


def test_set_computer_use_refuses_a_flow_mapping(tmp_path: Path) -> None:
    before = "host_service: {session_users: [alex]}\n"
    code, text = _set(tmp_path, before)
    assert code != 0
    assert text == before


@pytest.mark.skipif(shutil.which("pwsh") is None, reason="PowerShell is not installed here")
@pytest.mark.parametrize(("before", "after"), CASES)
def test_the_powershell_script_does_the_same(
    tmp_path: Path, before: str | None, after: str
) -> None:
    cfg = tmp_path / "config.yaml"
    if before is not None:
        cfg.write_bytes(before.encode("utf-8"))
    done = subprocess.run(
        ["pwsh", "-NoProfile", "-File", str(SET_SCRIPT_PS1), "-Config", str(cfg)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert done.returncode == 0, done.stderr
    assert cfg.read_bytes().decode("utf-8") == after


# ---- macOS signing --------------------------------------------------------------------------


def _sign_run(
    *, runtime: bool = True, authority: str = "Developer ID Application", dr: str = DR
) -> FakeRun:
    flags = "flags=0x10000(runtime)" if runtime else "flags=0x0(none)"
    details = f"Identifier=com.example.bothy-computer-helper\n{flags}\nAuthority={authority}: x\n"
    return FakeRun(
        {
            "codesign -dv": lambda a, o: subprocess.CompletedProcess(a, 0, "", details),
            "codesign -d": lambda a, o: ok(f"Executable=/x\n{dr}\n"),
        }
    )


def test_helper_sign_macos_signs_verifies_and_records_the_requirement(tmp_path: Path) -> None:
    app = tmp_path / hostdist.HELPER_APP
    req = tmp_path / "state" / "requirement.txt"
    run = _sign_run()
    checks = hostdist.helper_sign_macos(
        run, app=app, identity="Developer ID Application: X", requirement_file=req
    )
    assert run.programs() == [
        "codesign --force",
        "codesign --verify",
        "codesign -dv",
        "codesign -d",
    ]
    sign = run.calls[0]
    assert sign[sign.index("--options") + 1] == "runtime"
    assert sign[sign.index("--sign") + 1] == "Developer ID Application: X"
    assert "--timestamp=none" in sign
    assert "--deep" not in sign
    assert sign[-1] == str(app)
    assert "--strict" in run.calls[1]
    assert req.read_text(encoding="utf-8") == DR + "\n"
    assert any("recorded" in c for c in checks)
    # The same requirement again passes.
    checks = hostdist.helper_sign_macos(_sign_run(), app=app, identity="X", requirement_file=req)
    assert any("unchanged" in c for c in checks)


def test_helper_sign_macos_stops_on_a_changed_requirement(tmp_path: Path) -> None:
    req = tmp_path / "requirement.txt"
    req.write_text(DR + "\n", encoding="utf-8")
    other = 'designated => identifier "com.example.bothy-computer-helper" and cdhash H"00"'
    with pytest.raises(common.StepError, match="designated requirement changed"):
        hostdist.helper_sign_macos(
            _sign_run(dr=other), app=tmp_path / "a.app", identity="X", requirement_file=req
        )
    assert req.read_text(encoding="utf-8") == DR + "\n"


@pytest.mark.parametrize(
    ("kwargs", "match"),
    [
        ({"runtime": False}, "no hardened runtime"),
        ({"authority": "Apple Development"}, "not signed by a Developer ID"),
        ({"dr": "nothing"}, "no designated requirement"),
    ],
)
def test_helper_sign_macos_failures(tmp_path: Path, kwargs: dict[str, Any], match: str) -> None:
    with pytest.raises(common.StepError, match=match):
        hostdist.helper_sign_macos(
            _sign_run(**kwargs),
            app=tmp_path / "a.app",
            identity="X",
            requirement_file=tmp_path / "r",
        )
    assert not (tmp_path / "r").exists()


# ---- macOS package --------------------------------------------------------------------------


def _fake_app(tmp_path: Path) -> Path:
    app = tmp_path / "build" / hostdist.HELPER_APP
    exe = app / "Contents" / "MacOS" / hostdist.HELPER_EXE_MACOS
    exe.parent.mkdir(parents=True)
    exe.write_bytes(b"helper")
    exe.chmod(0o755)
    (app / "Contents" / "Info.plist").write_text("<plist/>", encoding="utf-8")
    return app


def _package_run() -> FakeRun:
    def go(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        out = Path(args[args.index("-o") + 1])
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_bytes(b"binary")
        return ok()

    return FakeRun({"go build": go})


def test_helper_package_macos(tmp_path: Path) -> None:
    app = _fake_app(tmp_path)
    run = _package_run()
    work = tmp_path / "work"
    pkg = hostdist.helper_package_macos(
        run, version=VERSION, out=tmp_path / "out", app=app, arch="arm64", workdir=work
    )
    assert pkg == tmp_path / "out" / f"bothy-host-computer_{VERSION}_darwin_arm64.pkg"
    build = next(c for c in run.calls if c[:2] == ["go", "build"])
    assert build[build.index("-ldflags") + 1].endswith("PackageKind=pkg")
    pkgbuilds = [c for c in run.calls if c[0] == "pkgbuild"]
    assert [c[c.index("--identifier") + 1] for c in pkgbuilds] == [
        hostdist.PKG_IDENTIFIER,
        hostdist.HELPER_PKG_IDENTIFIER,
    ]
    assert all(
        c[c.index("--install-location") + 1] == "/Library/Application Support" for c in pkgbuilds
    )
    helper = pkgbuilds[1]
    scripts = Path(helper[helper.index("--scripts") + 1])
    assert os.access(scripts / "postinstall", os.X_OK)
    assert (scripts / "set-computer-use.sh").read_bytes() == SET_SCRIPT.read_bytes()
    component = plistlib.loads(Path(helper[helper.index("--component-plist") + 1]).read_bytes())
    assert component == [
        {
            "RootRelativeBundlePath": f"bothy-host/bin/{hostdist.HELPER_APP}",
            "BundleIsRelocatable": False,
            "BundleIsVersionChecked": False,
            "BundleHasStrictIdentifier": True,
            "BundleOverwriteAction": "upgrade",
        }
    ]
    root = Path(helper[helper.index("--root") + 1])
    copied = root / "bothy-host" / "bin" / hostdist.HELPER_APP
    assert (copied / "Contents" / "MacOS" / hostdist.HELPER_EXE_MACOS).read_bytes() == b"helper"
    agent = root / "bothy-host" / "share" / hostdist.HELPER_LAUNCH_AGENT.name
    assert agent.read_bytes() == (hostdist.INFRA / "macos" / agent.name).read_bytes()
    assert agent.stat().st_mode & 0o777 == 0o644
    product = next(c for c in run.calls if c[0] == "productbuild")
    assert "--sign" not in product
    # The file the step under test wrote, not foreign input.
    dist = ET.parse(product[product.index("--distribution") + 1]).getroot()  # noqa: S314
    choices = {c.get("id"): c for c in dist.iter("choice")}
    assert choices["computer-helper"].get("start_selected") == "false"
    assert choices["host"].get("enabled") == "false"
    assert choices["host"].get("start_selected") == "true"
    refs = {c.get("id"): [r.get("id") for r in c.iter("pkg-ref")] for c in dist.iter("choice")}
    assert refs == {
        "host": [hostdist.PKG_IDENTIFIER],
        "computer-helper": [hostdist.HELPER_PKG_IDENTIFIER],
    }


def test_the_launch_agent_starts_the_installed_helper_in_the_gui_session() -> None:
    agent = plistlib.loads(
        (hostdist.INFRA / "macos" / hostdist.HELPER_LAUNCH_AGENT.name).read_bytes()
    )
    assert agent["Label"] == hostdist.HELPER_BUNDLE_ID
    assert agent["ProgramArguments"] == [
        str(
            hostdist.DARWIN_HELPER_DIR
            / hostdist.HELPER_APP
            / "Contents"
            / "MacOS"
            / hostdist.HELPER_EXE_MACOS
        )
    ]
    assert agent["LimitLoadToSessionType"] == "Aqua"
    assert agent["RunAtLoad"] is True
    assert agent["KeepAlive"] is True


def test_the_postinstall_sets_the_key_and_loads_the_agent() -> None:
    text = (hostdist.INFRA / "macos" / "helper-postinstall").read_text(encoding="utf-8")
    assert '"/Library/Application Support/bothy-host/config.yaml"' in text
    assert "set-computer-use.sh" in text
    assert "/Library/LaunchAgents/" in text
    assert "launchctl bootstrap gui/" in text


# ---- Windows --------------------------------------------------------------------------------


def test_helper_cert_windows_creates_a_machine_certificate(monkeypatch: pytest.MonkeyPatch) -> None:
    run = FakeRun({"powershell.exe": lambda a, o: ok("ABCDEF0123456789ABCDEF0123456789ABCDEF01\n")})
    thumb = hostdist.helper_cert_windows(run, subject="CN=bothy-computer-helper (local)")
    assert thumb == "ABCDEF0123456789ABCDEF0123456789ABCDEF01"
    script = run.calls[0][-1]
    for part in (
        "New-SelfSignedCertificate",
        "-Type CodeSigningCert",
        "-KeyExportPolicy NonExportable",
        "Cert:\\LocalMachine\\My",
        "Cert:\\LocalMachine\\Root",
        "Cert:\\LocalMachine\\TrustedPublisher",
        "$env:CERT_SUBJECT",
    ):
        assert part in script


def test_helper_cert_windows_needs_a_thumbprint() -> None:
    run = FakeRun({"powershell.exe": lambda a, o: ok("\n")})
    with pytest.raises(common.StepError, match="thumbprint"):
        hostdist.helper_cert_windows(run, subject="CN=x")


def test_helper_sign_windows(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(windows, "sdk_tool", lambda name, sdk_bin=None: Path(name))
    run = FakeRun()
    exe = tmp_path / hostdist.HELPER_EXE_WINDOWS
    hostdist.helper_sign_windows(run, exe=exe, thumbprint="AB" * 20)
    sign, verify = run.calls
    assert sign[:2] == ["signtool.exe", "sign"]
    assert sign[sign.index("/sha1") + 1] == "AB" * 20
    assert "/sm" in sign
    assert sign[sign.index("/fd") + 1] == "SHA256"
    assert sign[-1] == str(exe)
    assert verify[:3] == ["signtool.exe", "verify", "/pa"]
    with pytest.raises(common.StepError, match="thumbprint"):
        hostdist.helper_sign_windows(FakeRun(), exe=exe, thumbprint="not hex")


@pytest.mark.parametrize("thumbprint", ["CD" * 20, None])
def test_helper_package_windows(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, thumbprint: str | None
) -> None:
    monkeypatch.setattr(windows, "sdk_tool", lambda name, sdk_bin=None: Path(name))
    run = _package_run()
    helper = tmp_path / hostdist.HELPER_EXE_WINDOWS
    msi = hostdist.helper_package_windows(
        run,
        version=VERSION,
        out=tmp_path / "out",
        helper=helper,
        thumbprint=thumbprint,
        wix=tmp_path / "wix.dll",
        workdir=tmp_path / "work",
    )
    assert msi == tmp_path / "out" / f"bothy-host-computer_{VERSION}_windows_amd64.msi"
    wix = next(c for c in run.calls if c[0] == "dotnet")
    defines = [wix[i + 1] for i, a in enumerate(wix) if a == "-d"]
    assert f"HelperPath={helper}" in defines
    assert f"HelperScript={SET_SCRIPT_PS1}" in defines
    signed = [c[-1] for c in run.calls if c[:2] == ["signtool.exe", "sign"]]
    if thumbprint:
        assert signed == [str(tmp_path / "work" / "msi-bin" / "bothy-host.exe"), str(msi)]
    else:
        assert signed == []


def test_the_msi_has_the_helper_only_with_its_define() -> None:
    wxs = (hostdist.INFRA / "windows" / "bothy-host.wxs").read_text(encoding="utf-8")
    start = wxs.index("<?ifdef HelperPath?>")
    end = wxs.index("<?endif?>", start)
    block = wxs[start:end]
    # Everything of the helper sits inside the block, so a release build carries none of it.
    outside = re.sub(r"<!--.*?-->", "", wxs[:start] + wxs[end:], flags=re.DOTALL)
    for part in ("ComputerHelper", "bothy-computer-helper.exe", "set-computer-use.ps1"):
        assert part in block
        assert part not in outside
    assert 'Level="1000"' in block
    assert "CurrentVersion\\Run" in block
    assert 'Name="bothy-computer-helper"' in block
    assert 'Impersonate="no"' in block
    assert "&amp;ComputerHelper=3" in block


# ---- command line ---------------------------------------------------------------------------


def test_the_make_targets_call_the_steps() -> None:
    make = (hostdist.HOST / "Makefile").read_text(encoding="utf-8")
    for target, step in (
        ("helper-sign-macos:", "helper-sign-macos"),
        ("helper-package-macos:", "helper-package-macos"),
        ("helper-package-windows:", "helper-package-windows"),
    ):
        body = make[make.index(target) :].split("\n\n", 1)[0]
        assert f"$(HOSTDIST) {step}" in body
    body = make[make.index("helper-macos:") :].split("\n\n", 1)[0]
    assert "desktop/macos/bundle.sh" in body


# ---- the requirement the host service checks (D-CU-03-7) -----------------------------------


def _requirement_flag(run: FakeRun) -> str | None:
    build = next(c for c in run.calls if c[:2] == ["go", "build"])
    flags = build[build.index("-ldflags") + 1]
    marker = f"-X '{hostdist.MODULE}/internal/buildinfo.ComputerHelperRequirement="
    if marker not in flags:
        return None
    return flags.split(marker, 1)[1].rsplit("'", 1)[0]


def test_the_macos_package_builds_the_host_with_the_recorded_requirement(tmp_path: Path) -> None:
    req = tmp_path / "requirement.txt"
    req.write_text(DR + "\n", encoding="utf-8")
    run = _package_run()
    hostdist.helper_package_macos(
        run,
        version=VERSION,
        out=tmp_path / "out",
        app=_fake_app(tmp_path),
        arch="arm64",
        workdir=tmp_path / "work",
        requirement_file=req,
    )
    assert _requirement_flag(run) == DR.removeprefix("designated => ")


def test_the_macos_package_without_a_requirement_is_a_development_build(tmp_path: Path) -> None:
    run = _package_run()
    hostdist.helper_package_macos(
        run,
        version=VERSION,
        out=tmp_path / "out",
        app=_fake_app(tmp_path),
        arch="arm64",
        workdir=tmp_path / "work",
    )
    assert _requirement_flag(run) is None


def test_a_requirement_with_a_single_quote_is_refused() -> None:
    with pytest.raises(common.StepError, match="quote"):
        hostdist.ldflags("1.2.3", "pkg", requirement="identifier 'x'")


@pytest.mark.parametrize(
    ("thumbprint", "want"), [("CD" * 20, hostdist.HELPER_CERT_SUBJECT), (None, None)]
)
def test_the_windows_package_builds_the_host_with_the_publisher(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, thumbprint: str | None, want: str | None
) -> None:
    monkeypatch.setattr(windows, "sdk_tool", lambda name, sdk_bin=None: Path(name))
    run = _package_run()
    hostdist.helper_package_windows(
        run,
        version=VERSION,
        out=tmp_path / "out",
        helper=tmp_path / hostdist.HELPER_EXE_WINDOWS,
        thumbprint=thumbprint,
        wix=tmp_path / "wix.dll",
        workdir=tmp_path / "work",
    )
    assert _requirement_flag(run) == want


def test_helper_sign_macos_names_one_keychain(tmp_path: Path) -> None:
    # Never the search list: another project's keychain in it asked the owner for its password.
    run = _sign_run()
    hostdist.helper_sign_macos(
        run, app=tmp_path / "a.app", identity="X", requirement_file=tmp_path / "r"
    )
    sign = run.calls[0]
    assert sign[sign.index("--keychain") + 1] == str(hostdist.LOGIN_KEYCHAIN)
    other = tmp_path / "k.keychain-db"
    run = _sign_run()
    hostdist.helper_sign_macos(
        run, app=tmp_path / "a.app", identity="X", requirement_file=tmp_path / "r", keychain=other
    )
    assert run.calls[0][run.calls[0].index("--keychain") + 1] == str(other)
