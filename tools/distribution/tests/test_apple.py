# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/distribution/apple.py with a recording runner instead of the macOS tools.

The real chain (Xcode, codesign, notarytool, Sparkle's updater) runs in the CI job
desktop-trial-macos (`make -C apps/client trial-apple`); these tests hold the order of the steps,
what each command gets, and every stop before a signature.
"""

from __future__ import annotations

import base64
import io
import json
import plistlib
import subprocess
import tarfile
from collections.abc import Sequence
from pathlib import Path
from typing import Any

import appdist
import apple
import common
import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from fakes import MACHO, FakeRun, ok


@pytest.fixture
def ed() -> Ed25519PrivateKey:
    return Ed25519PrivateKey.from_private_bytes(bytes(range(32)))


@pytest.fixture
def cfg(ed: Ed25519PrivateKey) -> dict[str, Any]:
    return appdist.trial_distribution("http://127.0.0.1:4711/", appdist.public_key_text(ed))


def _plist(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(plistlib.dumps(data))


def make_app(root: Path, name: str = "ExampleChat", build: int = 1000) -> Path:
    """A bundle shaped like the Flutter build plus Sparkle: frameworks, an appex, a helper."""
    app = root / f"{name}.app"
    contents = app / "Contents"
    _plist(
        contents / "Info.plist",
        {
            "CFBundleIdentifier": "com.example.chat",
            "CFBundleExecutable": name,
            "CFBundleShortVersionString": appdist.app_version("0.1.0").short,
            "CFBundleVersion": str(build),
        },
    )
    (contents / "MacOS").mkdir(parents=True)
    (contents / "MacOS" / name).write_bytes(MACHO)
    for fw in ("FlutterMacOS", "Sparkle"):
        versions = contents / "Frameworks" / f"{fw}.framework" / "Versions"
        (versions / "A").mkdir(parents=True)
        (versions / "A" / fw).write_bytes(MACHO)
        (versions / "Current").symlink_to("A")
        (versions.parent / fw).symlink_to(f"Versions/Current/{fw}")
    sparkle = contents / "Frameworks" / "Sparkle.framework" / "Versions" / "A"
    (sparkle / "Autoupdate").write_bytes(MACHO)
    (sparkle / "XPCServices" / "Installer.xpc" / "Contents" / "MacOS").mkdir(parents=True)
    (sparkle / "Resources").mkdir()
    (sparkle / "Resources" / "readme.txt").write_text("not code", encoding="utf-8")
    appex = contents / "PlugIns" / "NotificationService.appex"
    _plist(
        appex / "Contents" / "Info.plist",
        {"CFBundleIdentifier": "com.example.chat.NotificationService", "CFBundleExecutable": "NS"},
    )
    (appex / "Contents" / "MacOS").mkdir(parents=True)
    (appex / "Contents" / "MacOS" / "NS").write_bytes(MACHO)
    (contents / "Frameworks" / "libextra.dylib").write_bytes(MACHO)
    return app


# ---- Sparkle --------------------------------------------------------------------------------


def _sparkle_archive() -> bytes:
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:xz") as tar:
        info = tarfile.TarInfo("Sparkle.framework/Versions/B/Sparkle")
        data = MACHO
        info.size = len(data)
        tar.addfile(info, io.BytesIO(data))
    return buf.getvalue()


def test_fetch_sparkle_checks_the_pinned_checksum(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    archive = _sparkle_archive()
    downloads: list[str] = []

    def download(url: str, target: Path) -> None:
        downloads.append(url)
        target.write_bytes(archive)

    with pytest.raises(common.StepError, match="checksum"):
        apple.fetch_sparkle(tmp_path, download)
    assert not (tmp_path / f"Sparkle-{apple.SPARKLE_VERSION}.tar.xz").exists()
    import hashlib

    monkeypatch.setattr(apple, "SPARKLE_SHA256", hashlib.sha256(archive).hexdigest())
    target = apple.fetch_sparkle(tmp_path, download)
    assert (target / "Sparkle.framework" / "Versions" / "B" / "Sparkle").read_bytes() == MACHO
    assert apple.fetch_sparkle(tmp_path, download) == target
    assert downloads == [apple.SPARKLE_URL, apple.SPARKLE_URL]


def test_the_sparkle_pin() -> None:
    assert apple.SPARKLE_URL.endswith(
        f"/{apple.SPARKLE_VERSION}/Sparkle-{apple.SPARKLE_VERSION}.tar.xz"
    )
    assert len(apple.SPARKLE_SHA256) == 64


# ---- bundle preparation ---------------------------------------------------------------------


def test_code_items_are_signed_inside_out(tmp_path: Path) -> None:
    app = make_app(tmp_path)
    items = [p.relative_to(app / "Contents").as_posix() for p in apple.code_items(app)]
    assert items == [
        "Frameworks/Sparkle.framework/Versions/A/XPCServices/Installer.xpc",
        "Frameworks/Sparkle.framework/Versions/A/Autoupdate",
        "Frameworks/FlutterMacOS.framework",
        "Frameworks/Sparkle.framework",
        "Frameworks/libextra.dylib",
        "PlugIns/NotificationService.appex",
    ]


def test_info_plist_gets_the_sparkle_keys(tmp_path: Path, cfg: dict[str, Any]) -> None:
    app = make_app(tmp_path)
    apple.configure_info_plist(app, cfg, appdist.app_version("0.1.0"))
    info = plistlib.loads((app / "Contents" / "Info.plist").read_bytes())
    assert info["SUFeedURL"] == "http://127.0.0.1:4711/appcast.xml"
    assert info["SURequireSignedFeed"] is True
    with pytest.raises(
        common.StepError, match=r"CFBundleShortVersionString is '0\.1\.0', expected '0\.1\.1'"
    ):
        apple.configure_info_plist(app, cfg, appdist.app_version("0.1.1"))
    other = dict(cfg, bundle_id="org.acme.chat")
    with pytest.raises(common.StepError, match="CFBundleIdentifier"):
        apple.configure_info_plist(app, other, appdist.app_version("0.1.0"))


def test_app_entitlements(cfg: dict[str, Any]) -> None:
    built = {
        "com.apple.security.app-sandbox": True,
        "com.apple.developer.aps-environment": "development",
        "keychain-access-groups": ["EXAMPLE000.com.example.chat"],
    }
    ent = apple.app_entitlements(built, cfg)
    assert ent["com.apple.developer.aps-environment"] == "production"
    assert ent["com.apple.application-identifier"] == "EXAMPLE000.com.example.chat"
    assert ent["com.apple.developer.team-identifier"] == "EXAMPLE000"
    assert ent["com.apple.security.temporary-exception.mach-lookup.global-name"] == [
        "com.example.chat-spks",
        "com.example.chat-spki",
    ]
    assert ent["com.apple.security.app-sandbox"] is True
    assert "com.apple.developer.aps-environment" not in apple.app_entitlements({}, cfg)


def _entitlements(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str]:
    if "--entitlements" in args and args[1] == "-d":
        target = args[-1]
        data = {"com.apple.security.app-sandbox": True}
        if target.endswith(".appex"):
            data["keychain-access-groups"] = ["EXAMPLE000.com.example.chat.shared"]
        if target.endswith("Installer.xpc"):
            return ok("")
        return ok(plistlib.dumps(data).decode())
    return ok()


def test_sign_app_signs_nested_code_first_with_their_entitlements(
    tmp_path: Path, cfg: dict[str, Any]
) -> None:
    app = make_app(tmp_path)
    run = FakeRun({"codesign": _entitlements})
    signer = apple.Signer(identity="A" * 40, keychain=tmp_path / "k.keychain-db", timestamp=False)
    apple.sign_app(run, app, cfg, signer, tmp_path, profiles={"": b"profile"})
    assert (app / "Contents" / "embedded.provisionprofile").read_bytes() == b"profile"
    signs = [c for c in run.calls if c[1] == "--force"]
    assert [Path(c[-1]).name for c in signs] == [
        "Installer.xpc",
        "Autoupdate",
        "FlutterMacOS.framework",
        "Sparkle.framework",
        "libextra.dylib",
        "NotificationService.appex",
        "ExampleChat.app",
    ]
    for call in signs:
        assert call[:9] == [
            "codesign",
            "--force",
            "--options",
            "runtime",
            "--keychain",
            str(signer.keychain),
            "--sign",
            "A" * 40,
            "--timestamp=none",
        ]
    assert "--preserve-metadata=entitlements" in signs[0]
    appex_ent = plistlib.loads(Path(signs[5][signs[5].index("--entitlements") + 1]).read_bytes())
    assert appex_ent["com.apple.application-identifier"] == (
        "EXAMPLE000.com.example.chat.NotificationService"
    )
    app_ent = plistlib.loads(Path(signs[-1][signs[-1].index("--entitlements") + 1]).read_bytes())
    assert "com.apple.security.temporary-exception.mach-lookup.global-name" in app_ent


def test_verify_signature_checks_the_team(tmp_path: Path) -> None:
    run = FakeRun(
        {
            "codesign -dv": lambda a, k: subprocess.CompletedProcess(
                a, 0, "", "TeamIdentifier=AB12CD34EF\n"
            )
        }
    )
    apple.verify_signature(run, tmp_path / "A.app", "AB12CD34EF")
    assert run.calls[0][:5] == ["codesign", "--verify", "--deep", "--strict", "--verbose=2"]
    with pytest.raises(common.StepError, match="not signed by the team EXAMPLE000"):
        apple.verify_signature(run, tmp_path / "A.app", "EXAMPLE000")
    apple.verify_signature(FakeRun(), tmp_path / "A.app", None)


def test_notarize(tmp_path: Path) -> None:
    notary = apple.Notary(key=tmp_path / "k.p8", key_id="KEY", issuer="ISSUER")
    run = FakeRun({"xcrun notarytool": lambda a, k: ok(json.dumps({"status": "Accepted"}))})
    apple.notarize(run, tmp_path / "A.app", notary, tmp_path)
    assert [c[1:3] for c in run.calls if c[0] == "xcrun"] == [
        ["notarytool", "submit"],
        ["stapler", "staple"],
        ["stapler", "validate"],
    ]
    run = FakeRun({"xcrun notarytool": lambda a, k: ok(json.dumps({"status": "Invalid"}))})
    with pytest.raises(common.StepError, match="ended with 'Invalid'"):
        apple.notarize(run, tmp_path / "A.app", notary, tmp_path)


def test_assess_reports_the_verdict(tmp_path: Path) -> None:
    run = FakeRun({"spctl": lambda a, k: subprocess.CompletedProcess(a, 3, "", "rejected\n")})
    assert apple.assess(run, tmp_path / "A.app") == (False, "rejected")
    assert apple.assess(FakeRun(), tmp_path / "A.app") == (True, "")


# ---- the release step -----------------------------------------------------------------------


def fake_tools(tmp_path: Path, client: Path, ed: Ed25519PrivateKey) -> FakeRun:
    """Handlers that act like flutter, ditto and sign_update on the fake bundle."""

    def flutter(args: list[str], kw: dict[str, Any]) -> None:
        build = int(next(a for a in args if a.startswith("--build-number=")).split("=")[1])
        short = next(a for a in args if a.startswith("--build-name=")).split("=")[1]
        products = client / "build" / "macos" / "Build" / "Products" / "Release"
        import shutil

        shutil.rmtree(products, ignore_errors=True)
        app = make_app(products, build=build)
        info = plistlib.loads((app / "Contents" / "Info.plist").read_bytes())
        info["CFBundleShortVersionString"] = short
        (app / "Contents" / "Info.plist").write_bytes(plistlib.dumps(info))
        assert kw["env"]["FLUTTER_XCODE_DEVELOPMENT_TEAM"] == "EXAMPLE000"

    def ditto(args: list[str], _: dict[str, Any]) -> None:
        import shutil

        if args[1] == "-c":
            Path(args[-1]).write_bytes(b"zip of " + Path(args[-2]).name.encode())
        elif args[1] == "-x":
            pass
        elif not Path(args[2]).exists():
            shutil.copytree(args[1], args[2], symlinks=True)

    def sign_update(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str]:
        data = Path(args[-1]).read_bytes()
        return ok(base64.b64encode(ed.sign(data)).decode() + "\n")

    return FakeRun(
        {
            "flutter": flutter,
            "ditto": ditto,
            "sign_update": sign_update,
            "codesign": _entitlements,
            "spctl": lambda a, k: subprocess.CompletedProcess(a, 3, "", "rejected"),
        }
    )


def _plan(
    cfg: dict[str, Any], tmp_path: Path, ed: Ed25519PrivateKey, version: str, **kw: Any
) -> apple.Plan:
    branding = tmp_path / "branding.yaml"
    branding.write_text(
        "app_name: ExampleChat\nurl_scheme: chat-example\nbundle_id: com.example.chat\n"
        "default_server: https://chat.example.com\n",
        encoding="utf-8",
    )
    return apple.Plan(
        cfg=cfg,
        version=appdist.app_version(version),
        out=tmp_path / "out" / version,
        branding=branding,
        signer=apple.Signer("B" * 40, tmp_path / "k", timestamp=False),
        ed_key=appdist.private_key_text(ed),
        **kw,
    )


def test_release_step_builds_signs_archives_and_writes_the_appcast(
    tmp_path: Path, cfg: dict[str, Any], ed: Ed25519PrivateKey, monkeypatch: pytest.MonkeyPatch
) -> None:
    client = tmp_path / "client"
    monkeypatch.setattr(apple, "CLIENT", client)
    monkeypatch.setattr(common, "CLIENT", client)
    run = fake_tools(tmp_path, client, ed)
    sparkle = tmp_path / "sparkle"
    work = tmp_path / "work"
    work.mkdir()
    first = apple.release_step(run, _plan(cfg, tmp_path, ed, "0.1.0"), sparkle, work)
    assert first.archive.name == "ExampleChat-0.1.0.zip"
    assert first.assessment == "rejected"
    entries = appdist.appcast_entries(first.appcast.read_bytes())
    assert [(e.build, e.channel) for e in entries] == [("1000", "beta")]
    appdist.verify_feed(cfg, first.appcast.read_bytes())
    programs = run.programs()
    assert programs.index("flutter build") < programs.index(
        "ditto " + str(sparkle / "Sparkle.framework")
    )
    assert run.stdin[programs.index("sign_update -p")] == appdist.private_key_text(ed)
    dart = [c for c in run.calls if c[0] == "dart"]
    assert {c[-1] for c in dart} == {"build/branding/input-branding.yaml"}
    assert (
        (client / "build" / "branding" / "input-branding.yaml")
        .read_text("utf-8")
        .startswith("app_name: ExampleChat")
    )

    second = apple.release_step(
        run, _plan(cfg, tmp_path, ed, "0.1.1", appcast=first.appcast), sparkle, work
    )
    assert [e.build for e in appdist.appcast_entries(second.appcast.read_bytes())] == [
        "1000",
        "1001",
    ]


def test_release_step_with_a_notary_requires_gatekeeper(
    tmp_path: Path, cfg: dict[str, Any], ed: Ed25519PrivateKey, monkeypatch: pytest.MonkeyPatch
) -> None:
    client = tmp_path / "client"
    monkeypatch.setattr(apple, "CLIENT", client)
    monkeypatch.setattr(common, "CLIENT", client)
    run = fake_tools(tmp_path, client, ed)
    run.handlers["codesign -dv"] = lambda a, k: subprocess.CompletedProcess(
        a, 0, "", "TeamIdentifier=EXAMPLE000"
    )
    run.handlers["xcrun notarytool"] = lambda a, k: ok(json.dumps({"status": "Accepted"}))
    notary = apple.Notary(tmp_path / "k.p8", "K", "I")
    work = tmp_path / "work"
    work.mkdir()
    with pytest.raises(common.StepError, match="spctl --assess rejects"):
        apple.release_step(run, _plan(cfg, tmp_path, ed, "0.1.0", notary=notary), tmp_path, work)


def test_release_step_refuses_a_foreign_key(
    tmp_path: Path, cfg: dict[str, Any], ed: Ed25519PrivateKey
) -> None:
    plan = _plan(cfg, tmp_path, ed, "0.1.0")
    plan.ed_key = appdist.private_key_text(Ed25519PrivateKey.from_private_bytes(bytes(32)))
    with pytest.raises(appdist.DistributionError, match="does not belong"):
        apple.release_step(FakeRun(), plan, tmp_path, tmp_path)


def test_build_needs_exactly_one_app(
    tmp_path: Path, cfg: dict[str, Any], monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(apple, "CLIENT", tmp_path / "client")
    monkeypatch.setattr(common, "CLIENT", tmp_path / "client")
    branding = tmp_path / "b.yaml"
    branding.write_text("app_name: X\n", encoding="utf-8")
    with pytest.raises(common.StepError, match="expected one app bundle"):
        apple.build_app(FakeRun(), cfg, appdist.app_version("0.1.0"), branding, tmp_path)


# ---- release and its stops ------------------------------------------------------------------


def test_release_stops_without_secrets_before_anything_runs(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(apple.sys, "platform", "darwin")
    run = FakeRun()
    with pytest.raises(
        common.StepError,
        match=r"APPLE_NOTARY_KEY, .*SPARKLE_ED_PRIVATE_KEY of the environment release-apple",
    ):
        apple.release(
            run,
            distribution=appdist.TEMPLATE,
            version="0.1.0",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            appcast=None,
            environ={"APPLE_DEVELOPER_ID_P12": "x"},
        )
    assert run.calls == []


def test_release_refuses_template_values(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(apple.sys, "platform", "darwin")
    environ = dict.fromkeys(apple.APPLE_SECRETS, "x")
    with pytest.raises(appdist.DistributionError, match="placeholder values"):
        apple.release(
            FakeRun(),
            distribution=appdist.TEMPLATE,
            version="0.1.0",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            appcast=None,
            environ=environ,
        )


def test_release_needs_macos(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.setattr(apple.sys, "platform", "linux")
    assert apple.main(["release", "--version", "0.1.0", "--out", str(tmp_path)], FakeRun()) == 1
    assert apple.main(["trial", "--out", str(tmp_path)], FakeRun()) == 1


# ---- keychain and test publisher ------------------------------------------------------------


def _security(args: list[str], _: dict[str, Any]) -> subprocess.CompletedProcess[str] | None:
    if args[1] == "list-keychains" and "-s" not in args:
        return ok('    "login.keychain-db"\n')
    if args[1] == "find-identity":
        return ok(f'  1) {"C" * 40} "Trial Test Publisher" (CSSMERR_TP_NOT_TRUSTED)\n')
    return None


def test_the_test_publisher_lives_in_a_temporary_keychain(tmp_path: Path) -> None:
    run = FakeRun({"security": _security})
    with apple.trial_publisher(run, tmp_path) as signer:
        assert signer.identity == "C" * 40
        assert signer.timestamp is False
        assert signer.keychain == tmp_path / "release.keychain-db"
    programs = [" ".join(c[:3]) for c in run.calls]
    assert programs[0].startswith("security create-keychain")
    assert any(
        p.startswith("sudo -n security") and "add-trusted-cert" in " ".join(c)
        for p, c in zip(programs, run.calls, strict=True)
    )
    restore = [c for c in run.calls if c[1] == "list-keychains" and "-s" in c][-1]
    assert restore[-1] == "login.keychain-db"
    assert str(signer.keychain) not in restore
    assert run.calls[-1][:2] == ["security", "delete-keychain"]
    assert any("remove-trusted-cert" in c for c in run.calls)
    assert (tmp_path / "test-publisher.p12").stat().st_size > 0


def test_import_without_identity_fails(tmp_path: Path) -> None:
    with pytest.raises(common.StepError, match="no code signing identity"):
        apple.import_identity(FakeRun(), tmp_path / "k", "pw", tmp_path / "x.p12", "pw")


# ---- the update test ------------------------------------------------------------------------


class HarnessRun(FakeRun):
    """Plays Sparkle's updater: compares the served feed with the installed bundle."""

    def __init__(self, cfg: dict[str, Any], www: Path) -> None:
        super().__init__({"ditto": self.ditto})
        self.cfg = cfg
        self.www = www
        self.archives: dict[str, Path] = {}

    def ditto(self, args: list[str], _: dict[str, Any]) -> None:
        make_app(Path(args[-1]), build=int(Path(args[-2]).read_bytes().decode().split()[-1]))

    def __call__(self, cmd: Sequence[str | Path], **kw: Any) -> subprocess.CompletedProcess[str]:
        args = [str(c) for c in cmd]
        if Path(args[0]).name != "update_harness":
            return super().__call__(cmd, **kw)
        self.calls.append(args)
        app = Path(args[1])
        channels = set(args[3::2])
        feed = (self.www / "appcast.xml").read_bytes()
        appdist.verify_feed(self.cfg, feed)
        current = int(apple.bundle_version(app))
        for entry in appdist.appcast_entries(feed):
            if int(entry.build) <= current or (entry.channel and entry.channel not in channels):
                continue
            data = (self.www / entry.url.rsplit("/", 1)[-1]).read_bytes()
            try:
                appdist.verify_archive(self.cfg, data, entry.signature)
            except appdist.DistributionError:
                return ok(code=1)
            info = app / "Contents" / "Info.plist"
            plist = plistlib.loads(info.read_bytes())
            plist["CFBundleVersion"] = entry.build
            info.write_bytes(plistlib.dumps(plist))
            return ok(code=apple.HARNESS_UPDATED)
        return ok(code=apple.HARNESS_NO_UPDATE)


def test_update_test_drives_every_check(
    tmp_path: Path, cfg: dict[str, Any], ed: Ed25519PrivateKey
) -> None:
    www = tmp_path / "www"
    www.mkdir()
    releases = []
    appcast = appdist.new_appcast("ExampleChat", cfg)
    for version in ("0.1.0", "0.1.1"):
        v = appdist.app_version(version)
        out = tmp_path / version
        out.mkdir()
        archive = out / f"ExampleChat-{version}.zip"
        archive.write_bytes(f"zip {v.build}".encode())
        signed = appdist.Archive(
            archive.name,
            archive.stat().st_size,
            base64.b64encode(ed.sign(archive.read_bytes())).decode(),
        )
        appcast = appdist.add_items(
            appcast,
            [appdist.appcast_item(cfg, app_name="ExampleChat", version=v, archive=signed)],
            ed,
        )
        if version == "0.1.0":
            appcast = appdist.promote_appcast(appcast, v.build, ed)
        (out / "appcast.xml").write_bytes(appcast)
        releases.append(apple.Release(v, out / "ExampleChat.app", archive, out / "appcast.xml"))
    run = HarnessRun(cfg, www)
    trial = apple.Trial(run=run, harness=tmp_path / "update_harness", install=tmp_path / "install")
    apple.update_test(trial, cfg, releases[0], releases[1], www, ed)
    assert len(trial.checks) == 7
    harness = [c for c in run.calls if Path(c[0]).name == "update_harness"]
    assert [c[2:] for c in harness] == [[], ["--channel", "beta"], ["--channel", "beta"], []]
    assert appdist.appcast_entries((www / "appcast.xml").read_bytes())[-1].channel is None


def test_a_failed_check_stops_the_trial(tmp_path: Path) -> None:
    trial = apple.Trial(run=FakeRun(), harness=tmp_path, install=tmp_path)
    with pytest.raises(common.StepError, match="trial check failed: x"):
        trial.expect(False, "x")


def test_bundle_version_while_the_installer_works(tmp_path: Path) -> None:
    assert apple.bundle_version(tmp_path / "gone.app") == ""
    assert apple.bundle_version(make_app(tmp_path, build=7)) == "7"


def test_release_with_secrets_signs_with_the_imported_identity(
    tmp_path: Path, ed: Ed25519PrivateKey, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(apple.sys, "platform", "darwin")
    monkeypatch.setattr(apple, "fetch_sparkle", lambda: tmp_path / "sparkle")
    seen: list[apple.Plan] = []

    def step(run: Any, plan: apple.Plan, sparkle: Path, workdir: Path) -> apple.Release:
        seen.append(plan)
        assert (workdir / "developer-id.p12").read_bytes() == b"p12"
        assert (workdir / "notary.p8").read_bytes() == b"p8"
        return apple.Release(plan.version, workdir, workdir / "a.zip", workdir / "appcast.xml")

    monkeypatch.setattr(apple, "release_step", step)
    monkeypatch.setattr(appdist, "require_publisher_values", lambda cfg: None)
    environ = dict.fromkeys(apple.APPLE_SECRETS, base64.b64encode(b"x").decode())
    environ["APPLE_DEVELOPER_ID_P12"] = base64.b64encode(b"p12").decode()
    environ["APPLE_NOTARY_KEY"] = base64.b64encode(b"p8").decode()
    run = FakeRun({"security": _security})
    monkeypatch.setattr(apple.os, "environ", environ)
    assert apple.main(["release", "--version", "1.2.3", "--out", str(tmp_path / "out")], run) == 0
    [plan] = seen
    assert plan.signer.timestamp is True
    assert plan.signer.identity == "C" * 40
    assert plan.notary is not None
    assert plan.notary.key_id == environ["APPLE_NOTARY_KEY_ID"]
    assert plan.version.build == 1_002_003
    assert set(plan.profiles or {}) == {"", "Contents/PlugIns/NotificationService.appex"}
    assert not any("add-trusted-cert" in c for c in run.calls)


def test_trial_builds_twice_promotes_and_runs_the_update_test(
    tmp_path: Path, ed: Ed25519PrivateKey, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(apple.sys, "platform", "darwin")
    monkeypatch.setattr(apple, "fetch_sparkle", lambda: tmp_path / "sparkle")
    built: list[tuple[str, bool]] = []

    def step(run: Any, plan: apple.Plan, sparkle: Path, workdir: Path) -> apple.Release:
        built.append((plan.version.short, plan.appcast is not None))
        key = appdist.private_key(plan.ed_key)
        current = plan.appcast.read_bytes() if plan.appcast else appdist.new_appcast("A", plan.cfg)
        archive = appdist.Archive("a.zip", 1, "c2ln")
        item = appdist.appcast_item(plan.cfg, app_name="A", version=plan.version, archive=archive)
        plan.out.mkdir(parents=True)
        (plan.out / "appcast.xml").write_bytes(appdist.add_items(current, [item], key))
        return apple.Release(
            plan.version, workdir, workdir / "a.zip", plan.out / "appcast.xml", "rejected"
        )

    tested: list[list[str | None]] = []

    def update(
        trial: apple.Trial,
        cfg: dict[str, Any],
        old: apple.Release,
        new: apple.Release,
        www: Path,
        key: Any,
    ) -> None:
        tested.append([e.channel for e in appdist.appcast_entries(new.appcast.read_bytes())])
        trial.expect(True, "updated")

    monkeypatch.setattr(apple, "release_step", step)
    monkeypatch.setattr(apple, "update_test", update)
    run = FakeRun({"security": _security})
    assert apple.main(["trial", "--out", str(tmp_path / "out")], run) == 0
    assert built == [("0.1.0", False), ("0.1.1", True)]
    assert tested == [[None, "beta"]]
    report = json.loads((tmp_path / "out" / "trial-apple.json").read_text("utf-8"))
    assert report["spctl"] == "rejected"
    assert report["sparkle"] == apple.SPARKLE_VERSION
    assert any(Path(c[0]).name == "xcrun" and c[1] == "swiftc" for c in run.calls)
