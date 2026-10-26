# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/distribution/hostdist.py with a recording runner instead of the platform tools.

The real chains run in `make release-trial COMPONENT=host` (Linux packages in containers) and the
CI jobs host-trial-macos and host-trial-windows; these tests hold the build commands, the order
of signing and packaging, the stops without secrets and the checks of an installed package.
"""

from __future__ import annotations

import hashlib
import io
import subprocess
import zipfile
from pathlib import Path
from typing import Any

import apple
import common
import hostdist
import pytest
import windows
from fakes import FakeRun, ok

VERSION = hostdist.component_version()


def _ldflags(call: list[str]) -> str:
    return call[call.index("-ldflags") + 1]


def _fake_nfpm(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str]:
    Path(args[args.index("--target") + 1]).write_bytes(b"package")
    return ok()


def test_names_and_linker_flags() -> None:
    assert hostdist.binary_name("1.2.3", "windows", "amd64") == "bothy-host_1.2.3_windows_amd64.exe"
    assert hostdist.binary_name("1.2.3", "linux", "arm64") == "bothy-host_1.2.3_linux_arm64"
    assert hostdist.package_name("1.2.3", "darwin", "arm64", "pkg") == (
        "bothy-host_1.2.3_darwin_arm64.pkg"
    )
    flags = hostdist.ldflags("1.2.3", "deb")
    assert f"-X {hostdist.MODULE}/internal/buildinfo.Version=1.2.3 " in flags
    assert flags.endswith(f"-X {hostdist.MODULE}/internal/buildinfo.PackageKind=deb")


def test_build_writes_binaries_and_linux_packages(tmp_path: Path) -> None:
    def go(args: list[str], opts: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        out = Path(args[args.index("-o") + 1])
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_bytes(b"binary")
        return ok()

    run = FakeRun({"go build": go, "nfpm": _fake_nfpm})
    written = hostdist.build(run, VERSION, tmp_path, "nfpm")
    names = [p.name for p in written]
    assert names[:5] == [hostdist.binary_name(VERSION, o, a) for o, a in hostdist.PLATFORMS]
    assert names[5:] == [
        hostdist.package_name(VERSION, "linux", a, k) for k, a in hostdist.LINUX_PACKAGES
    ]
    builds = [c for c in run.calls if c[:2] == ["go", "build"]]
    assert len(builds) == 9
    for call in builds:
        assert "-trimpath" in call
        assert "-buildvcs=false" in call
    kinds = [_ldflags(c).rsplit("=", 1)[1] for c in builds]
    assert kinds == ["", "", "", "", "", "deb", "deb", "rpm", "rpm"]
    packagers = [c[c.index("--packager") + 1] for c in run.calls if c[0] == "nfpm"]
    assert packagers == ["deb", "deb", "rpm", "rpm"]


def test_build_refuses_another_version(tmp_path: Path) -> None:
    run = FakeRun()
    with pytest.raises(common.StepError, match=r"not 9\.9\.9"):
        hostdist.build(run, "9.9.9", tmp_path, "nfpm")
    with pytest.raises(common.StepError, match="not SemVer"):
        hostdist.check_version("v1.0.0")
    assert run.calls == []


def test_build_stops_when_nfpm_writes_nothing(tmp_path: Path) -> None:
    run = FakeRun()
    with pytest.raises(common.StepError, match="nfpm wrote no"):
        hostdist.build(run, VERSION, tmp_path, "nfpm")


@pytest.mark.parametrize(
    ("version", "msi"),
    [("1.2.3", "1.2.3"), ("1.2.3-beta.4", "1.2.3"), ("0.0.0+build.5", "0.0.0")],
)
def test_msi_version_drops_the_pre_release(version: str, msi: str) -> None:
    assert hostdist.msi_version(version) == msi


def test_msi_version_range() -> None:
    with pytest.raises(common.StepError, match="MSI version range"):
        hostdist.msi_version("256.0.0")


def _nupkg() -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as archive:
        archive.writestr("tools/net6.0/any/wix.dll", b"dll")
    return buf.getvalue()


def test_fetch_wix_checks_the_digest(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    data = _nupkg()
    monkeypatch.setattr(hostdist, "WIX_SHA256", hashlib.sha256(data).hexdigest())
    calls: list[str] = []

    def download(url: str, target: Path) -> None:
        calls.append(url)
        target.write_bytes(data)

    dll = hostdist.fetch_wix(tmp_path, download)
    assert dll.read_bytes() == b"dll"
    assert hostdist.fetch_wix(tmp_path, download) == dll
    assert calls == [hostdist.WIX_URL]


def test_fetch_wix_refuses_another_package(tmp_path: Path) -> None:
    def download(url: str, target: Path) -> None:
        target.write_bytes(_nupkg())

    with pytest.raises(common.StepError, match="SHA-256"):
        hostdist.fetch_wix(tmp_path, download)
    assert not (tmp_path / f"wix.{hostdist.WIX_VERSION}.nupkg").exists()


# ---- macOS ----------------------------------------------------------------------------------


def _apple_run(runtime: bool = True, team: str = "TEAM123456") -> FakeRun:
    def go(args: list[str], opts: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        out = Path(args[args.index("-o") + 1])
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_bytes(b"macho")
        return ok()

    def codesign(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        if args[1] == "-dv":
            flags = "flags=0x10000(runtime)" if runtime else "flags=0x0(none)"
            return subprocess.CompletedProcess(args, 0, "", f"{flags}\nTeamIdentifier={team}\n")
        return ok()

    def productbuild(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        Path(args[-1]).write_bytes(b"pkg")
        return ok()

    return FakeRun(
        {
            "go build": go,
            "codesign": codesign,
            "productbuild": productbuild,
            "xcrun notarytool": lambda a, o: ok('{"status": "Accepted"}'),
        }
    )


def _plan(tmp_path: Path, notary: apple.Notary | None = None) -> hostdist.ApplePlan:
    signer = apple.Signer(identity="A" * 40, keychain=tmp_path / "k", timestamp=notary is not None)
    installer = apple.Signer(identity="B" * 40, keychain=tmp_path / "k", timestamp=True)
    return hostdist.ApplePlan(
        version=VERSION,
        out=tmp_path / "out",
        signer=signer,
        installer=installer,
        team="TEAM123456" if notary else None,
        notary=notary,
    )


def test_apple_packages_sign_package_and_notarize(tmp_path: Path) -> None:
    run = _apple_run()
    notary = apple.Notary(key=tmp_path / "n.p8", key_id="K", issuer="I")
    pkgs = hostdist.apple_packages(run, _plan(tmp_path, notary), tmp_path / "work")
    assert [p.name for p in pkgs] == [
        hostdist.package_name(VERSION, "darwin", a, "pkg") for a in hostdist.APPLE_ARCHES
    ]
    builds = [c for c in run.calls if c[:2] == ["go", "build"]]
    assert all(_ldflags(c).endswith("PackageKind=pkg") for c in builds)
    progs = run.programs()
    # Per architecture: build, sign with the runtime and a timestamp, verify strictly, two
    # components, the signed product, notarization, stapling and Gatekeeper.
    first = progs[: progs.index("spctl --assess") + 1]
    assert first == [
        "go build",
        "codesign --force",
        "codesign --verify",
        "codesign -dv",
        "pkgbuild --root",
        "pkgbuild --root",
        "productbuild --package",
        "xcrun notarytool",
        "xcrun stapler",
        "xcrun stapler",
        "spctl --assess",
    ]
    sign = next(c for c in run.calls if c[:2] == ["codesign", "--force"])
    assert "runtime" in sign
    assert "--timestamp" in sign
    assert "--strict" in next(c for c in run.calls if c[:2] == ["codesign", "--verify"])
    pkgbuilds = [c for c in run.calls if c[0] == "pkgbuild"]
    locations = [c[c.index("--install-location") + 1] for c in pkgbuilds[:2]]
    assert locations == ["/Library/Application Support", "/Library/LaunchDaemons"]
    assert all(c[c.index("--ownership") + 1] == "recommended" for c in pkgbuilds)
    assert "--scripts" in pkgbuilds[1]
    assert "--scripts" not in pkgbuilds[0]
    product = next(c for c in run.calls if c[0] == "productbuild")
    assert product[product.index("--sign") + 1] == "B" * 40
    root = tmp_path / "work" / "pkg-arm64"
    assert (root / "binary" / "bothy-host" / "bin" / "bothy-host").stat().st_mode & 0o777 == 0o755
    plist = root / "launchd" / "bothy-host.plist"
    assert plist.read_bytes() == (hostdist.INFRA / "macos" / "bothy-host.plist").read_bytes()


def test_apple_binary_without_runtime_or_with_another_team_stops(tmp_path: Path) -> None:
    notary = apple.Notary(key=tmp_path / "n.p8", key_id="K", issuer="I")
    with pytest.raises(common.StepError, match="no hardened runtime"):
        hostdist.apple_packages(_apple_run(runtime=False), _plan(tmp_path, notary), tmp_path / "a")
    with pytest.raises(common.StepError, match="not signed by the team"):
        hostdist.apple_packages(_apple_run(team="OTHER"), _plan(tmp_path, notary), tmp_path / "b")


def test_notarization_must_be_accepted_and_assessed(tmp_path: Path) -> None:
    notary = apple.Notary(key=tmp_path / "n.p8", key_id="K", issuer="I")
    run = FakeRun({"xcrun notarytool": lambda a, o: ok('{"status": "Invalid"}')})
    with pytest.raises(common.StepError, match="Invalid"):
        hostdist.notarize_pkg(run, tmp_path / "x.pkg", notary)
    run = FakeRun(
        {
            "xcrun notarytool": lambda a, o: ok('{"status": "Accepted"}'),
            "spctl": lambda a, o: ok("rejected", code=3),
        }
    )
    with pytest.raises(common.StepError, match="spctl rejects"):
        hostdist.notarize_pkg(run, tmp_path / "x.pkg", notary)


def test_the_trial_plan_has_no_notarization(tmp_path: Path) -> None:
    run = _apple_run()
    hostdist.apple_packages(run, _plan(tmp_path), tmp_path / "work")
    assert "xcrun notarytool" not in run.programs()
    sign = next(c for c in run.calls if c[:2] == ["codesign", "--force"])
    assert "--timestamp=none" in sign


def test_apple_release_without_secrets_builds_nothing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(apple, "require_macos", lambda: None)
    run = FakeRun()
    with pytest.raises(common.StepError, match="APPLE_INSTALLER_P12"):
        hostdist.apple_release(
            run,
            version=VERSION,
            out=tmp_path,
            distribution=tmp_path / "d.yaml",
            environ={"APPLE_DEVELOPER_ID_P12": "x"},
        )
    assert run.calls == []


def test_installer_identity_is_the_new_identity(tmp_path: Path) -> None:
    listings = iter(
        [
            '  1) AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA "app"\n',
            '  1) AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA "app"\n'
            '  2) BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB "installer"\n',
        ]
    )
    run = FakeRun({"security find-identity": lambda a, o: ok(next(listings))})
    ident = hostdist.installer_identity(run, tmp_path / "k", "pw", tmp_path / "i.p12", "p12pw")
    assert ident == "B" * 40
    imported = next(c for c in run.calls if c[:2] == ["security", "import"])
    assert "/usr/bin/productbuild" in imported


def _installed_apple(stats: dict[str, str], loaded: int = 113, disabled: str = "") -> FakeRun:
    def stat(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        return ok(stats.get(args[-1], "root:wheel 755") + "\n")

    def launchctl(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        if args[1] == "print":
            return ok("", code=loaded)
        return ok(disabled or '\tdisabled services = {\n\t\t"bothy-host" => disabled\n\t}\n')

    return FakeRun(
        {
            "stat": stat,
            "launchctl": launchctl,
            "bothy-host": lambda a, o: ok(VERSION + "\n"),
        }
    )


def test_check_installed_apple(tmp_path: Path) -> None:
    run = _installed_apple({str(hostdist.DARWIN_PLIST): "root:wheel 644"})
    checks = hostdist.check_installed_apple(run, VERSION)
    assert any("launchd has not loaded" in c for c in checks)
    assert next(c for c in run.calls if c[0] == "codesign")[:4] == [
        "codesign",
        "--verify",
        "--strict",
        "--verbose=2",
    ]


@pytest.mark.parametrize(
    ("stats", "loaded", "disabled", "match"),
    [
        ({str(hostdist.DARWIN_PLIST): "root:staff 664"}, 113, "", "want root:wheel 644"),
        ({str(hostdist.DARWIN_PLIST): "root:wheel 644"}, 0, "", "loaded the host service"),
        ({str(hostdist.DARWIN_PLIST): "root:wheel 644"}, 113, "nothing", "not disabled"),
    ],
)
def test_check_installed_apple_failures(
    stats: dict[str, str], loaded: int, disabled: str, match: str
) -> None:
    with pytest.raises(common.StepError, match=match):
        hostdist.check_installed_apple(_installed_apple(stats, loaded, disabled), VERSION)


# ---- Windows --------------------------------------------------------------------------------


def _windows_run(tmp_path: Path, ps: dict[str, str] | None = None) -> FakeRun:
    answers = ps or {}

    def go(args: list[str], opts: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        out = Path(args[args.index("-o") + 1])
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_bytes(b"pe")
        return ok()

    def powershell(args: list[str], opts: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        script = args[-1]
        for key, answer in answers.items():
            if key in script:
                return ok(answer + "\n")
        return ok("")

    return FakeRun(
        {
            "go build": go,
            "powershell.exe": powershell,
            "bothy-host.exe": lambda a, o: ok(VERSION + "\n"),
        }
    )


def test_windows_package_signs_binary_then_msi(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(windows, "sdk_tool", lambda name, sdk_bin=None: Path(name))
    monkeypatch.setattr(hostdist, "fetch_wix", lambda: tmp_path / "wix.dll")
    run = _windows_run(tmp_path)
    cert = windows.Certificate(pfx=tmp_path / "c.pfx", password="secret")
    msi = hostdist.windows_package(run, "1.2.3-beta.1", tmp_path / "out", cert, tmp_path)
    assert msi.name == "bothy-host_1.2.3-beta.1_windows_amd64.msi"
    progs = run.programs()
    assert progs == [
        "go build",
        "signtool.exe sign",
        "signtool.exe verify",
        "dotnet " + str(tmp_path / "wix.dll"),
        "signtool.exe sign",
        "signtool.exe verify",
    ]
    build = next(c for c in run.calls if c[0] == "go")
    assert _ldflags(build).endswith("PackageKind=msi")
    wix = next(c for c in run.calls if c[0] == "dotnet")
    assert "ProductVersion=1.2.3" in wix
    assert wix[wix.index("-arch") + 1] == "x64"
    verify = next(c for c in run.calls if c[1] == "verify")
    assert "/pa" in verify


def test_windows_release_without_secrets_builds_nothing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(windows, "require_windows", lambda: None)
    run = FakeRun()
    with pytest.raises(common.StepError, match="WINDOWS_SIGNING_PFX"):
        hostdist.windows_release(
            run, version=VERSION, out=tmp_path, distribution=tmp_path / "d.yaml", environ={}
        )
    assert run.calls == []


GOOD = {
    ".Owner": "BUILTIN\\Administrators",
    "Win32_Service": "Manual Stopped NT SERVICE\\bothy-host",
}


def test_check_installed_windows(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(windows, "sdk_tool", lambda name, sdk_bin=None: Path(name))
    run = _windows_run(tmp_path, GOOD)
    checks = hostdist.check_installed_windows(run, VERSION, tmp_path / "bothy-host")
    assert len(checks) == 5
    assert any("start type demand" in c for c in checks)


@pytest.mark.parametrize(
    ("answers", "match"),
    [
        ({**GOOD, ".Owner": "WIN\\runner"}, "not to a trusted owner"),
        ({**GOOD, "FileSystemRights": "BUILTIN\\Users Modify"}, "writable for"),
        ({**GOOD, "Win32_Service": "Auto Running LocalSystem"}, "want Manual Stopped"),
    ],
)
def test_check_installed_windows_failures(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, answers: dict[str, str], match: str
) -> None:
    monkeypatch.setattr(windows, "sdk_tool", lambda name, sdk_bin=None: Path(name))
    with pytest.raises(common.StepError, match=match):
        hostdist.check_installed_windows(_windows_run(tmp_path, answers), VERSION, tmp_path)


def test_msiexec_accepts_only_success(tmp_path: Path) -> None:
    for code, good in (("0", True), ("3010", True), ("1603", False)):

        def answer(
            args: list[str], opts: dict[str, Any], c: str = code
        ) -> subprocess.CompletedProcess[str]:
            return ok(c + "\n")

        run = FakeRun({"powershell.exe": answer})
        if good:
            hostdist._msiexec(run, "i", tmp_path / "x.msi", tmp_path / "log")
        else:
            with pytest.raises(common.StepError, match="1603"):
                hostdist._msiexec(run, "i", tmp_path / "x.msi", tmp_path / "log")


def test_main_usage_and_failure(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    with pytest.raises(SystemExit) as exc:
        hostdist.main([])
    assert exc.value.code == 2
    assert hostdist.main(["build", "--version", "9.9.9", "--out", str(tmp_path)], FakeRun()) == 1
    assert "not 9.9.9" in capsys.readouterr().err
