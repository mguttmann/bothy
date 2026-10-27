# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/distribution/ios.py with a recording runner instead of Xcode and codesign.

The real build runs in `make -C apps/client trial-ios` and the CI job `mobile trial (iOS)`;
signing and TestFlight need the publisher's Apple account (OQ-08). These tests hold the checks
of an archive, the entitlements of each bundle, the signing order, the upload and its stops.
"""

from __future__ import annotations

import base64
import json
import plistlib
from pathlib import Path
from typing import Any

import appdist
import common
import ios
import pytest
import yaml
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec
from fakes import FakeRun, ok
from test_android import cfg as publisher_cfg  # noqa: F401 - the fixture of a real publisher
from test_storeapi import FakeHttp, answer

TEAM = "AB12CD34EF"
BUNDLE = "org.acme.chat"


@pytest.fixture
def cfg(publisher_cfg: dict[str, Any]) -> dict[str, Any]:  # noqa: F811
    return publisher_cfg


def _plist(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(plistlib.dumps(data))


def make_app(
    root: Path, bundle: str = BUNDLE, version: str = "1.2.3", build: int = 1002003
) -> Path:
    """An iOS app bundle as `flutter build ipa` lays it out, with both extensions."""
    app = root / "Runner.app"
    _plist(
        app / "Info.plist",
        {
            "CFBundleIdentifier": bundle,
            "CFBundleShortVersionString": version,
            "CFBundleVersion": str(build),
            "CFBundleExecutable": "Runner",
        },
    )
    (app / "Runner").write_bytes(b"macho")
    for fw in ("Flutter", "App"):
        (app / "Frameworks" / f"{fw}.framework").mkdir(parents=True)
    for name in ios.EXTENSIONS:
        ext = app / "PlugIns" / f"{name}.appex"
        _plist(
            ext / "Info.plist",
            {
                "CFBundleIdentifier": f"{bundle}.{name}",
                "CFBundleShortVersionString": version,
                "CFBundleVersion": str(build),
                "CFBundleExecutable": name,
            },
        )
        (ext / name).write_bytes(b"macho")
    (app / "PlugIns" / "ShareExtension.appex" / "Frameworks" / "libx.dylib").parent.mkdir()
    (app / "PlugIns" / "ShareExtension.appex" / "Frameworks" / "libx.dylib").write_bytes(b"m")
    return app


def built_entitlements(team: str = TEAM, bundle: str = BUNDLE) -> dict[str, dict[str, Any]]:
    """What branding_defines.dart writes for a team (the files the build signs with)."""
    groups = {"com.apple.security.application-groups": [f"group.{bundle}"]}
    result: dict[str, dict[str, Any]] = {
        "": {
            "aps-environment": "development",
            **groups,
            "keychain-access-groups": [f"{team}.{bundle}", f"{team}.{bundle}.shared"],
        }
    }
    for name in ios.EXTENSIONS:
        result[f"PlugIns/{name}.appex"] = {
            **groups,
            "keychain-access-groups": [f"{team}.{bundle}.shared"],
        }
    return result


# ---- identities and entitlements ------------------------------------------------------------


def test_bundle_ids_of_the_app_and_its_extensions(cfg: dict[str, Any]) -> None:
    assert ios.bundle_ids(cfg) == {
        "": BUNDLE,
        "PlugIns/NotificationService.appex": f"{BUNDLE}.NotificationService",
        "PlugIns/ShareExtension.appex": f"{BUNDLE}.ShareExtension",
    }


def test_expected_entitlements_match_what_the_branding_writes(cfg: dict[str, Any]) -> None:
    assert ios.expected_entitlements(cfg) == built_entitlements()


def test_distribution_entitlements(cfg: dict[str, Any]) -> None:
    app = ios.distribution_entitlements(built_entitlements()[""], cfg, BUNDLE)
    assert app["application-identifier"] == f"{TEAM}.{BUNDLE}"
    assert app["com.apple.developer.team-identifier"] == TEAM
    assert app["get-task-allow"] is False
    assert app["aps-environment"] == "production"
    assert app["keychain-access-groups"] == [f"{TEAM}.{BUNDLE}", f"{TEAM}.{BUNDLE}.shared"]
    ext = ios.distribution_entitlements(
        built_entitlements()["PlugIns/ShareExtension.appex"], cfg, f"{BUNDLE}.ShareExtension"
    )
    assert "aps-environment" not in ext
    assert ext["application-identifier"] == f"{TEAM}.{BUNDLE}.ShareExtension"


# ---- archive --------------------------------------------------------------------------------


def test_check_archive_passes_a_correct_build(tmp_path: Path, cfg: dict[str, Any]) -> None:
    archive = ios.Archive(app=make_app(tmp_path), entitlements=built_entitlements())
    checks = ios.check_archive(archive, cfg, appdist.app_version("1.2.3"))
    assert checks[0] == f"{BUNDLE}: identifier, version 1.2.3, build 1002003"
    assert len(checks) == 6


@pytest.mark.parametrize(
    ("change", "message"),
    [
        (
            lambda app, ent: _plist(app / "Info.plist", {"CFBundleIdentifier": "x"}),
            "Runner.app names",
        ),
        (
            lambda app, ent: _plist(
                app / "PlugIns" / "ShareExtension.appex" / "Info.plist",
                {
                    "CFBundleIdentifier": f"{BUNDLE}.ShareExtension",
                    "CFBundleShortVersionString": "1.2.3",
                    "CFBundleVersion": "1",
                },
            ),
            "PlugIns/ShareExtension.appex names",
        ),
        (lambda app, ent: (app / "Runner").unlink(), "has no executable"),
        (
            lambda app, ent: __import__("shutil").rmtree(
                app / "PlugIns" / "NotificationService.appex"
            ),
            "has no PlugIns/NotificationService.appex",
        ),
        (lambda app, ent: ent[""].pop("aps-environment"), "aps-environment is None"),
        (
            lambda app, ent: ent["PlugIns/ShareExtension.appex"].update(
                {"keychain-access-groups": ["x"]}
            ),
            "keychain-access-groups",
        ),
        (
            lambda app, ent: __import__("shutil").rmtree(app / "Frameworks" / "Flutter.framework"),
            "no Flutter",
        ),
    ],
)
def test_check_archive_refuses(
    tmp_path: Path, cfg: dict[str, Any], change: Any, message: str
) -> None:
    app = make_app(tmp_path)
    entitlements = built_entitlements()
    change(app, entitlements)
    with pytest.raises(common.StepError, match=message):
        ios.check_archive(
            ios.Archive(app=app, entitlements=entitlements), cfg, appdist.app_version("1.2.3")
        )


@pytest.fixture
def tree(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    client = tmp_path / "client"
    monkeypatch.setattr(common, "CLIENT", client)
    monkeypatch.setattr(ios, "CLIENT", client)
    monkeypatch.setattr(ios, "ARCHIVE", client / "build" / "ios" / "archive" / "Runner.xcarchive")
    return client


def build_tools(seen: dict[str, Any]) -> dict[str, Any]:
    """`dart run tool/branding_defines.dart` writes the entitlements of the branding it gets;
    `flutter build ipa` the archive; `ditto` copies."""

    def dart(args: list[str], opts: dict[str, Any]) -> None:
        branding = yaml.safe_load((common.CLIENT / args[-1]).read_text(encoding="utf-8"))
        seen.setdefault("brandings", []).append(branding)
        team = branding.get("development_team", "")
        for relative, ent in built_entitlements(team, branding["bundle_id"]).items():
            folder = "Runner" if not relative else relative.split("/")[1].removesuffix(".appex")
            _plist(common.CLIENT / "ios" / folder / "Branding.entitlements", ent if team else {})

    def flutter(args: list[str], opts: dict[str, Any]) -> None:
        seen["flutter"] = args
        make_app(ios.ARCHIVE / "Products" / "Applications")

    def ditto(args: list[str], opts: dict[str, Any]) -> None:
        if args[1] == "-c":
            Path(args[-1]).write_bytes(b"IPA")
            return
        __import__("shutil").copytree(args[1], args[2], symlinks=True)

    return {"dart run": dart, "flutter": flutter, "ditto": ditto}


def test_build_archive_with_the_publishers_team_and_update_path(
    tmp_path: Path, tree: Path, cfg: dict[str, Any]
) -> None:
    seen: dict[str, Any] = {}
    work = tmp_path / "work"
    work.mkdir()
    archive = ios.build_archive(
        FakeRun(build_tools(seen)), cfg, appdist.app_version("1.2.3"), common.DEFAULT_BRANDING, work
    )
    assert archive.app == work / "Runner.app"
    assert archive.entitlements == built_entitlements()
    release = seen["brandings"][0]
    assert release["bundle_id"] == BUNDLE
    assert release["development_team"] == TEAM
    assert release["update_url"] == cfg["ios"]["update_url"]
    # The tree gets the given branding back afterwards.
    assert "development_team" not in seen["brandings"][1]
    assert seen["flutter"][1:5] == ["build", "ipa", "--release", "--no-codesign"]
    assert "--build-number=1002003" in seen["flutter"]


def test_build_needs_exactly_one_app(tmp_path: Path, tree: Path, cfg: dict[str, Any]) -> None:
    seen: dict[str, Any] = {}
    handlers = build_tools(seen)
    handlers["flutter"] = lambda args, opts: None
    with pytest.raises(common.StepError, match="expected one app"):
        ios.build_archive(
            FakeRun(handlers), cfg, appdist.app_version("1.2.3"), common.DEFAULT_BRANDING, tmp_path
        )


# ---- signature ------------------------------------------------------------------------------


def profile(team: str, bundle: str) -> str:
    return plistlib.dumps({"Entitlements": {"application-identifier": f"{team}.{bundle}"}}).decode()


def signing_tools(seen: dict[str, Any], team: str = TEAM) -> dict[str, Any]:
    def security(args: list[str], opts: dict[str, Any]) -> Any:
        if args[1] == "cms":
            data = Path(args[-1]).read_bytes().decode()
            return ok(data)
        if args[1] == "find-identity":
            return ok('  1) 0123456789ABCDEF0123456789ABCDEF01234567 "Apple Distribution: Acme"\n')
        if args[1] == "list-keychains" and len(args) == 4:
            return ok('    "/login.keychain-db"\n')
        return None

    def codesign(args: list[str], opts: dict[str, Any]) -> Any:
        if "-dv" in args:
            return ok(f"TeamIdentifier={team}\n")
        if "--entitlements" in args and "-d" in args:
            return ok(
                plistlib.dumps(
                    {"application-identifier": f"{TEAM}.{BUNDLE}", "aps-environment": "production"}
                ).decode()
            )
        if "--sign" in args:
            ent = args[args.index("--entitlements") + 1] if "--entitlements" in args else None
            seen.setdefault("signed", []).append(
                (Path(args[-1]).name, plistlib.loads(Path(ent).read_bytes()) if ent else None)
            )
        return None

    return {"security": security, "codesign": codesign}


def profiles(team: str = TEAM) -> dict[str, bytes]:
    return {
        rel: profile(team, bundle).encode()
        for rel, bundle in {
            "": BUNDLE,
            "PlugIns/NotificationService.appex": f"{BUNDLE}.NotificationService",
            "PlugIns/ShareExtension.appex": f"{BUNDLE}.ShareExtension",
        }.items()
    }


def test_sign_app_embeds_the_profiles_and_signs_inside_out(
    tmp_path: Path, cfg: dict[str, Any]
) -> None:
    seen: dict[str, Any] = {}
    archive = ios.Archive(app=make_app(tmp_path / "a"), entitlements=built_entitlements())
    signer = ios.Signer(identity="0123", keychain=tmp_path / "k", timestamp=False)
    ios.sign_app(FakeRun(signing_tools(seen)), archive, cfg, signer, profiles(), tmp_path)
    names = [name for name, _ in seen["signed"]]
    assert names[-1] == "Runner.app"
    assert names.index("libx.dylib") < names.index("ShareExtension.appex")
    assert {"Flutter.framework", "App.framework", "NotificationService.appex"} <= set(names)
    ents = dict(seen["signed"])
    assert ents["Flutter.framework"] is None
    assert ents["Runner.app"]["aps-environment"] == "production"
    assert ents["Runner.app"]["application-identifier"] == f"{TEAM}.{BUNDLE}"
    assert ents["NotificationService.appex"]["application-identifier"] == (
        f"{TEAM}.{BUNDLE}.NotificationService"
    )
    for rel in profiles():
        assert ((archive.app / rel) / "embedded.mobileprovision").is_file()


def test_sign_app_refuses_a_profile_of_another_app(tmp_path: Path, cfg: dict[str, Any]) -> None:
    archive = ios.Archive(app=make_app(tmp_path / "a"), entitlements=built_entitlements())
    signer = ios.Signer(identity="0123", keychain=tmp_path / "k", timestamp=False)
    wrong = profiles()
    wrong["PlugIns/ShareExtension.appex"] = profile(TEAM, "org.other.app").encode()
    with pytest.raises(
        common.StepError, match=r"ShareExtension\.appex is for AB12CD34EF\.org\.other"
    ):
        ios.sign_app(FakeRun(signing_tools({})), archive, cfg, signer, wrong, tmp_path)


def test_verify_signature_checks_team_identifier_and_push(
    tmp_path: Path, cfg: dict[str, Any]
) -> None:
    app = make_app(tmp_path)
    ios.verify_signature(FakeRun(signing_tools({})), app, cfg)
    with pytest.raises(common.StepError, match="not signed by the team"):
        ios.verify_signature(FakeRun(signing_tools({}, team="ZZZZZZZZZZ")), app, cfg)
    handlers = signing_tools({})
    base = handlers["codesign"]

    def development(args: list[str], opts: dict[str, Any]) -> Any:
        if "--entitlements" in args and "-d" in args:
            return ok(
                plistlib.dumps(
                    {"application-identifier": f"{TEAM}.{BUNDLE}", "aps-environment": "development"}
                ).decode()
            )
        return base(args, opts)

    handlers["codesign"] = development
    with pytest.raises(common.StepError, match="aps-environment is 'development'"):
        ios.verify_signature(FakeRun(handlers), app, cfg)


def test_upload_reads_the_key_from_its_folder(tmp_path: Path) -> None:
    seen: dict[str, Any] = {}

    def xcrun(args: list[str], opts: dict[str, Any]) -> None:
        seen["args"], seen["env"] = args, opts["env"]

    key = tmp_path / "keys" / "AuthKey_K1.p8"
    ios.upload(FakeRun({"xcrun altool": xcrun}), tmp_path / "a.ipa", key, "K1", "issuer")
    assert seen["args"][1:5] == ["altool", "--upload-app", "--type", "ios"]
    assert seen["args"][-4:] == ["--apiKey", "K1", "--apiIssuer", "issuer"]
    assert seen["env"] == {"API_PRIVATE_KEYS_DIR": str(tmp_path / "keys")}


# ---- release, promotion, trial --------------------------------------------------------------


@pytest.fixture
def p8() -> str:
    return (
        ec.generate_private_key(ec.SECP256R1())
        .private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        )
        .decode()
    )


def environ(p8: str) -> dict[str, str]:
    profile_b64 = {rel: base64.b64encode(data).decode() for rel, data in profiles().items()}
    return {
        "APPLE_DISTRIBUTION_P12": base64.b64encode(b"p12").decode(),
        "APPLE_DISTRIBUTION_P12_PASSWORD": "pw",
        "APPLE_IOS_PROVISIONING_PROFILE": profile_b64[""],
        "APPLE_IOS_NOTIFICATION_PROFILE": profile_b64["PlugIns/NotificationService.appex"],
        "APPLE_IOS_SHARE_PROFILE": profile_b64["PlugIns/ShareExtension.appex"],
        "APPLE_NOTARY_KEY": base64.b64encode(p8.encode()).decode(),
        "APPLE_NOTARY_KEY_ID": "K1",
        "APPLE_NOTARY_ISSUER_ID": "issuer",
    }


API = ios.storeapi.ASC_API


def asc_routes() -> dict[str, Any]:
    return {
        f"GET {API}/apps": answer({"data": [{"id": "6400", "attributes": {"bundleId": BUNDLE}}]}),
        f"GET {API}/builds": answer(
            {"data": [{"id": "b1", "attributes": {"processingState": "VALID"}}]}
        ),
        f"GET {API}/apps/6400/betaGroups": answer(
            {
                "data": [
                    {"id": "g-beta", "attributes": {"name": "beta"}},
                    {"id": "g-stable", "attributes": {"name": "stable"}},
                ]
            }
        ),
        f"POST {API}/betaGroups/g-beta/relationships/builds": answer({}),
        f"POST {API}/betaGroups/g-stable/relationships/builds": answer({}),
    }


def write_cfg(tmp_path: Path, cfg: dict[str, Any]) -> Path:
    path = tmp_path / "distribution.yaml"
    path.write_text(yaml.safe_dump(cfg), encoding="utf-8")
    return path


def test_release_builds_signs_uploads_and_adds_the_build_to_beta(
    tmp_path: Path, tree: Path, cfg: dict[str, Any], p8: str, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(ios.sys, "platform", "darwin")
    seen: dict[str, Any] = {}
    run = FakeRun({**build_tools(seen), **signing_tools(seen), "xcrun altool": lambda a, o: None})
    http = FakeHttp(asc_routes())
    ipa = ios.release(
        run,
        distribution=write_cfg(tmp_path, cfg),
        version="1.2.3",
        out=tmp_path / "out",
        branding=common.DEFAULT_BRANDING,
        environ=environ(p8),
        http=http,
    )
    assert ipa == tmp_path / "out" / f"{BUNDLE}-1.2.3.ipa"
    programs = run.programs()
    assert programs.index("flutter build") < programs.index("security import")
    assert programs.index("codesign --verify") < programs.index("xcrun altool")
    # The keychain of the run is gone at the end; the build went into the group beta.
    assert programs.index("security delete-keychain") < programs.index("xcrun altool")
    assert http.calls[-1][1] == f"{API}/betaGroups/g-beta/relationships/builds"
    # altool reads the decoded key from the folder of the run.
    altool = next(c for c in run.calls if c[:2] == ["xcrun", "altool"])
    assert altool[altool.index("--apiKey") + 1] == "K1"


def test_release_stops_without_secrets_before_anything_runs(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(ios.sys, "platform", "darwin")
    run = FakeRun()
    with pytest.raises(
        common.StepError,
        match=r"APPLE_DISTRIBUTION_P12_PASSWORD, .*APPLE_NOTARY_ISSUER_ID of the environment "
        r"release-apple",
    ):
        ios.release(
            run,
            distribution=appdist.TEMPLATE,
            version="1.2.3",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            environ={"APPLE_DISTRIBUTION_P12": "x"},
        )
    assert run.calls == []


def test_release_refuses_template_values_and_needs_macos(
    tmp_path: Path, p8: str, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(ios.sys, "platform", "darwin")
    with pytest.raises(appdist.DistributionError, match="placeholder values"):
        ios.release(
            FakeRun(),
            distribution=appdist.TEMPLATE,
            version="1.2.3",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            environ=environ(p8),
        )
    monkeypatch.setattr(ios.sys, "platform", "linux")
    with pytest.raises(common.StepError, match="need a macOS host"):
        ios.trial(FakeRun(), out=tmp_path, branding=common.DEFAULT_BRANDING)


def test_a_secret_that_is_no_base64_is_named(tmp_path: Path) -> None:
    with pytest.raises(common.StepError, match="APPLE_IOS_SHARE_PROFILE is no base64"):
        ios._decode("not base64!", "APPLE_IOS_SHARE_PROFILE")
    with pytest.raises(common.StepError, match=r"no base64 of a \.p8 key"):
        ios._api_key(base64.b64encode(b"\xff\xfe").decode())


def test_promote_adds_the_build_to_stable(tmp_path: Path, cfg: dict[str, Any], p8: str) -> None:
    http = FakeHttp(asc_routes())
    ios.promote(
        distribution=write_cfg(tmp_path, cfg),
        build=1002003,
        environ={k: v for k, v in environ(p8).items() if k.startswith("APPLE_NOTARY")},
        http=http,
    )
    assert http.calls[-1][1] == f"{API}/betaGroups/g-stable/relationships/builds"


def test_trial_checks_the_unsigned_archive_of_the_template(
    tmp_path: Path, tree: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(ios.sys, "platform", "darwin")
    template = appdist.load(appdist.TEMPLATE)
    seen: dict[str, Any] = {}
    handlers = build_tools(seen)

    def flutter(args: list[str], opts: dict[str, Any]) -> None:
        seen["flutter"] = args
        make_app(
            ios.ARCHIVE / "Products" / "Applications",
            bundle=template["bundle_id"],
            version=ios.TRIAL_VERSION,
            build=appdist.app_version(ios.TRIAL_VERSION).build,
        )

    handlers["flutter"] = flutter
    run = FakeRun(handlers)
    checks = ios.trial(run, out=tmp_path / "out", branding=common.DEFAULT_BRANDING)
    assert len(checks) == 6
    report = json.loads((tmp_path / "out" / "trial-ios.json").read_text(encoding="utf-8"))
    team = template["apple"]["team_id"]
    assert report["distribution_entitlements"][""]["application-identifier"] == (
        f"{team}.{template['bundle_id']}"
    )
    # A trial neither signs nor uploads.
    assert not any(c[0] in ("codesign", "security", "xcrun") for c in run.calls)


def test_main_reports_failures_with_exit_code_1(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setattr(ios.sys, "platform", "linux")
    assert ios.main(["release", "--version", "1.2.3", "--out", str(tmp_path)], run=FakeRun()) == 1
    assert "macOS" in capsys.readouterr().err
    assert ios.main(["promote", "--build", "1"], run=FakeRun()) == 1
    assert ios.main(["trial", "--out", str(tmp_path)], run=FakeRun()) == 1


def test_main_runs_the_trial(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setattr(ios, "trial", lambda run, out, branding: ["a", "b"])
    assert ios.main(["trial", "--out", str(tmp_path)], run=FakeRun()) == 0
    assert "trial-ios: 2 checks passed" in capsys.readouterr().out
