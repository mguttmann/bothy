# SPDX-License-Identifier: Apache-2.0
"""iOS release of the phone app (brief M7-13, ADR 0037, docs/spec/18-release.md 5 and 8).

`make -C apps/client release-ios VERSION=<X.Y.Z> OUT=<dir>` (release.yml job build-apple):

1. the secrets of the environment `release-apple` (IOS_SECRETS) and a distribution without
   placeholders; the version gives the build number `X * 1000000 + Y * 1000 + Z`
   (`appdist.app_version`);
2. the build branding with the publisher's bundle identifier, team (`apple.team_id`, so the
   App Group and the keychain groups carry it) and update path (`ios.update_url`), then
   `flutter build ipa --release --no-codesign`: the archive of the app and its two extensions
   (Notification Service, Share), unsigned;
3. the checks of the archive (`check_archive`): bundle identifiers, version and build number of
   the app and its extensions, and the entitlements the build gives them;
4. the signature for the App Store: the Apple Distribution identity in a keychain of this run,
   the App Store provisioning profiles of the app and its extensions embedded, every nested
   framework signed, then the extensions and the app with the entitlements of
   `distribution_entitlements` (`aps-environment` production, the team's application
   identifier); `codesign --verify --deep --strict` must pass and the app's signature must name
   the team, its bundle identifier and `aps-environment` production;
5. `Payload/<app>` zipped into `<bundle id>-<version>.ipa` in OUT, uploaded with
   `xcrun altool --upload-app` and the App Store Connect API key, and, once App Store Connect
   has processed it, added to the TestFlight group `beta` (`storeapi.AppStoreConnect`; 18 5).

`ios.py promote --build N` adds the same build to the TestFlight group `stable` (18 5 rule 1:
the same build, no rebuild).

`make -C apps/client trial-ios` runs steps 2 and 3 with a test publisher (D-W-127, brief Design
note 2): the template distribution, no signature and no upload, because signing and TestFlight
need an Apple Developer Program account (OQ-08). It writes `<out>/trial-ios.json`.

Usage:
  ios.py release --version X.Y.Z --out DIR [--distribution FILE] [--branding FILE]
  ios.py promote --build N [--distribution FILE]
  ios.py trial --out DIR [--branding FILE]
Exit codes: 0 done, 1 a step failed or this is no macOS host (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import base64
import binascii
import json
import os
import plistlib
import shutil
import sys
import tempfile
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import appdist
import storeapi
import yaml
from apple import Signer, import_identity, temporary_keychain
from common import (
    CLIENT,
    DEFAULT_BRANDING,
    Runner,
    StepError,
    branded_tree,
    branding_for,
    fail,
    require_env,
)

IOS_SECRETS = (
    "APPLE_DISTRIBUTION_P12",
    "APPLE_DISTRIBUTION_P12_PASSWORD",
    "APPLE_IOS_PROVISIONING_PROFILE",
    "APPLE_IOS_NOTIFICATION_PROFILE",
    "APPLE_IOS_SHARE_PROFILE",
    "APPLE_NOTARY_KEY",
    "APPLE_NOTARY_KEY_ID",
    "APPLE_NOTARY_ISSUER_ID",
)
# The TestFlight groups of the channels (18 5).
BETA_GROUP = "beta"
STABLE_GROUP = "stable"
TRIAL_VERSION = "0.1.0"
# The app extensions of the iOS app: the folder of their sources (with their entitlements) and
# the suffix of their bundle identifier (ios/<name>/<name>.xcconfig).
EXTENSIONS = ("NotificationService", "ShareExtension")
ARCHIVE = CLIENT / "build" / "ios" / "archive" / "Runner.xcarchive"
# The secret of each provisioning profile, by the bundle path relative to the app.
PROFILES = {
    "": "APPLE_IOS_PROVISIONING_PROFILE",
    "PlugIns/NotificationService.appex": "APPLE_IOS_NOTIFICATION_PROFILE",
    "PlugIns/ShareExtension.appex": "APPLE_IOS_SHARE_PROFILE",
}


@dataclass(frozen=True)
class Archive:
    """The unsigned build of one version.

    Attributes:
        app: the app bundle inside the archive.
        entitlements: the entitlements the build gives each bundle, by the bundle path
            relative to the app (`""` for the app, `PlugIns/<name>.appex` for an extension).
    """

    app: Path
    entitlements: dict[str, dict[str, Any]]


def require_macos() -> None:
    """Stop unless this is a macOS host (the build needs Xcode).

    Raises:
        StepError: on any other system.
    """
    if sys.platform != "darwin":
        raise StepError("release-ios and trial-ios need a macOS host with Xcode")


def bundle_ids(cfg: dict[str, Any]) -> dict[str, str]:
    """Return the bundle identifier of the app and of each extension, by bundle path."""
    ids = {"": cfg["bundle_id"]}
    for name in EXTENSIONS:
        ids[f"PlugIns/{name}.appex"] = f"{cfg['bundle_id']}.{name}"
    return ids


def expected_entitlements(cfg: dict[str, Any]) -> dict[str, dict[str, Any]]:
    """Return the entitlements a build for the team of `cfg` must give each bundle.

    The app: `aps-environment` (development in the build; the distribution signature says
    production), the App Group `group.<bundle id>` and the keychain groups of the app and the
    one it shares with its extensions; each extension the App Group and the shared keychain
    group (docs/decisions.d/M5-04.md, M5-06.md).
    """
    team, bundle = cfg["apple"]["team_id"], cfg["bundle_id"]
    shared = {
        "com.apple.security.application-groups": [f"group.{bundle}"],
    }
    result: dict[str, dict[str, Any]] = {
        "": {
            "aps-environment": "development",
            **shared,
            "keychain-access-groups": [f"{team}.{bundle}", f"{team}.{bundle}.shared"],
        }
    }
    for name in EXTENSIONS:
        result[f"PlugIns/{name}.appex"] = {
            **shared,
            "keychain-access-groups": [f"{team}.{bundle}.shared"],
        }
    return result


def distribution_entitlements(
    built: Mapping[str, Any], cfg: dict[str, Any], bundle_id: str
) -> dict[str, Any]:
    """Return the entitlements of a bundle signed for the App Store from those of its build.

    The team's application identifier and team identifier, which the App Store provisioning
    profile names, `get-task-allow` false, and `aps-environment` production (an App Store
    profile grants only that, docs/decisions.d/M5-04.md D-M5-04-10).
    """
    team = cfg["apple"]["team_id"]
    result = dict(built)
    result["application-identifier"] = f"{team}.{bundle_id}"
    result["com.apple.developer.team-identifier"] = team
    result["get-task-allow"] = False
    if "aps-environment" in result:
        result["aps-environment"] = "production"
    return result


def _plist(path: Path) -> dict[str, Any]:
    try:
        data = plistlib.loads(path.read_bytes())
    except (OSError, plistlib.InvalidFileException) as err:
        raise StepError(f"{path} is no property list") from err
    if not isinstance(data, dict):
        raise StepError(f"{path} is no dictionary")
    return data


# ---- build and checks -----------------------------------------------------------------------


def build_archive(
    run: Runner,
    cfg: dict[str, Any],
    version: appdist.AppVersion,
    branding: Path,
    workdir: Path,
) -> Archive:
    """Build the unsigned archive of `version` for the publisher of `cfg` (step 2).

    The entitlements are read from the files the build branding writes
    (`ios/<target>/Branding.entitlements`) while the tree carries it.

    Raises:
        StepError: the build fails or does not produce exactly one app.
    """
    release_branding = branding_for(branding, cfg, workdir / "branding.yaml", "ios")
    doc = yaml.safe_load(release_branding.read_text(encoding="utf-8"))
    doc["development_team"] = cfg["apple"]["team_id"]
    release_branding.write_text(yaml.safe_dump(doc, sort_keys=False), encoding="utf-8")
    shutil.rmtree(ARCHIVE, ignore_errors=True)
    with branded_tree(run, branding, release_branding):
        entitlements = {"": _plist(CLIENT / "ios" / "Runner" / "Branding.entitlements")}
        for name in EXTENSIONS:
            entitlements[f"PlugIns/{name}.appex"] = _plist(
                CLIENT / "ios" / name / "Branding.entitlements"
            )
        run(
            [
                "flutter",
                "build",
                "ipa",
                "--release",
                "--no-codesign",
                f"--build-name={version.short}",
                f"--build-number={version.build}",
                "--dart-define-from-file=build/branding/dart_defines.json",
            ],
            cwd=CLIENT,
        )
    apps = sorted((ARCHIVE / "Products" / "Applications").glob("*.app"))
    if len(apps) != 1:
        raise StepError(f"expected one app in {ARCHIVE}, found {len(apps)}")
    target = workdir / apps[0].name
    shutil.rmtree(target, ignore_errors=True)
    run(["ditto", apps[0], target])
    return Archive(app=target, entitlements=entitlements)


def check_archive(archive: Archive, cfg: dict[str, Any], version: appdist.AppVersion) -> list[str]:
    """Check the structure and entitlements of an unsigned archive (step 3); return the checks.

    Raises:
        StepError: naming the first finding.
    """
    checks: list[str] = []
    for relative, bundle_id in bundle_ids(cfg).items():
        bundle = archive.app / relative if relative else archive.app
        if not bundle.is_dir():
            raise StepError(f"the archive has no {relative}")
        info = _plist(bundle / "Info.plist")
        found = (
            info.get("CFBundleIdentifier"),
            info.get("CFBundleShortVersionString"),
            info.get("CFBundleVersion"),
        )
        want = (bundle_id, version.short, str(version.build))
        if found != want:
            raise StepError(
                f"{relative or archive.app.name} names {found}, the release builds {want}"
            )
        executable = bundle / str(info.get("CFBundleExecutable", ""))
        if not executable.is_file():
            raise StepError(f"{relative or archive.app.name} has no executable")
        checks.append(f"{bundle_id}: identifier, version {version.short}, build {version.build}")
    expected = expected_entitlements(cfg)
    for relative, want_entitlements in expected.items():
        got = archive.entitlements.get(relative, {})
        for key, value in want_entitlements.items():
            if got.get(key) != value:
                raise StepError(
                    f"{relative or 'the app'}: entitlement {key} is {got.get(key)!r}, "
                    f"the team needs {value!r}"
                )
        checks.append(
            f"{relative or 'the app'}: entitlements {', '.join(sorted(want_entitlements))}"
        )
    if (archive.app / "Frameworks" / "Flutter.framework").is_dir() is False:
        raise StepError("the app carries no Flutter.framework")
    return checks


# ---- signature ------------------------------------------------------------------------------


def _decode(text: str, name: str) -> bytes:
    try:
        return base64.b64decode(text, validate=True)
    except (binascii.Error, ValueError) as err:
        raise StepError(f"{name} is no base64") from err


def _api_key(text: str) -> str:
    """Return the `.p8` of the App Store Connect API key from its secret.

    The secret is base64, as for notarytool in release-apple (docs/operations/release.md 4.1).

    Raises:
        StepError: the secret is no base64 of a text.
    """
    try:
        return _decode(text, "APPLE_NOTARY_KEY").decode("utf-8")
    except UnicodeDecodeError as err:
        raise StepError("APPLE_NOTARY_KEY is no base64 of a .p8 key") from err


def profile_entitlements(run: Runner, profile: Path) -> dict[str, Any]:
    """Return the entitlements a provisioning profile grants (`security cms -D`)."""
    done = run(["security", "cms", "-D", "-i", profile], capture=True)
    try:
        data = plistlib.loads(done.stdout.encode("utf-8"))
    except plistlib.InvalidFileException as err:
        raise StepError(f"{profile.name} is no provisioning profile") from err
    entitlements = data.get("Entitlements") if isinstance(data, dict) else None
    if not isinstance(entitlements, dict):
        raise StepError(f"{profile.name} grants no entitlements")
    return entitlements


def code_items(app: Path) -> list[Path]:
    """Return the nested code of an iOS `app` to sign, the deepest first (inside out).

    Frameworks and dynamic libraries of the extensions and of the app, then the extensions; the
    app itself is signed last by the caller.
    """
    items = [
        p
        for p in app.rglob("*")
        if not p.is_symlink() and (p.suffix in (".framework", ".appex") or p.suffix == ".dylib")
    ]
    return sorted(items, key=lambda p: (-len(p.parts), str(p)))


def sign_app(
    run: Runner,
    archive: Archive,
    cfg: dict[str, Any],
    signer: Signer,
    profiles: Mapping[str, bytes],
    workdir: Path,
) -> None:
    """Embed the profiles and sign the app for the App Store (step 4).

    Raises:
        StepError: a profile names another application identifier, or codesign fails.
    """
    team = cfg["apple"]["team_id"]
    ids = bundle_ids(cfg)
    for relative, data in profiles.items():
        target = archive.app / relative if relative else archive.app
        profile = target / "embedded.mobileprovision"
        profile.write_bytes(data)
        granted = profile_entitlements(run, profile).get("application-identifier")
        if granted != f"{team}.{ids[relative]}":
            raise StepError(
                f"the profile of {relative or 'the app'} is for {granted}, "
                f"the app needs {team}.{ids[relative]}"
            )

    def sign(path: Path, entitlements: dict[str, Any] | None, name: str) -> None:
        cmd: list[str | Path] = [
            "codesign",
            "--force",
            "--keychain",
            signer.keychain,
            "--sign",
            signer.identity,
            "--timestamp=none",
        ]
        if entitlements is not None:
            ent = workdir / f"{name}.entitlements"
            ent.write_bytes(plistlib.dumps(entitlements))
            cmd += ["--entitlements", ent]
        run([*cmd, path])

    for index, item in enumerate(code_items(archive.app)):
        if item.suffix == ".appex":
            relative = item.relative_to(archive.app).as_posix()
            sign(
                item,
                distribution_entitlements(archive.entitlements[relative], cfg, ids[relative]),
                f"extension-{index}",
            )
        else:
            sign(item, None, f"code-{index}")
    sign(archive.app, distribution_entitlements(archive.entitlements[""], cfg, ids[""]), "app")


def verify_signature(run: Runner, app: Path, cfg: dict[str, Any]) -> None:
    """Check the distribution signature of `app` (step 4).

    Raises:
        StepError: the signature does not verify, names another team, or the app's entitlements
            lack the application identifier or `aps-environment` production.
    """
    run(["codesign", "--verify", "--deep", "--strict", "--verbose=2", app])
    team = cfg["apple"]["team_id"]
    details = run(["codesign", "-dv", "--verbose=2", app], capture=True)
    if f"TeamIdentifier={team}" not in details.stdout + details.stderr:
        raise StepError(f"{app.name} is not signed by the team {team}")
    done = run(["codesign", "-d", "--entitlements", "-", "--xml", app], capture=True)
    try:
        signed = plistlib.loads(done.stdout.strip().encode("utf-8"))
    except plistlib.InvalidFileException as err:
        raise StepError(f"{app.name} carries no entitlements") from err
    if signed.get("application-identifier") != f"{team}.{cfg['bundle_id']}":
        raise StepError(f"{app.name} is signed for {signed.get('application-identifier')}")
    if signed.get("aps-environment") != "production":
        raise StepError(f"{app.name}: aps-environment is {signed.get('aps-environment')!r}")


def make_ipa(run: Runner, app: Path, out: Path, name: str, workdir: Path) -> Path:
    """Zip `Payload/<app>` into `out/<name>`; return the IPA."""
    payload = workdir / "Payload"
    shutil.rmtree(payload, ignore_errors=True)
    payload.mkdir()
    run(["ditto", app, payload / app.name])
    out.mkdir(parents=True, exist_ok=True)
    ipa = out / name
    ipa.unlink(missing_ok=True)
    run(["ditto", "-c", "-k", "--sequesterRsrc", "--keepParent", payload, ipa])
    return ipa


def upload(run: Runner, ipa: Path, key: Path, key_id: str, issuer: str) -> None:
    """Upload `ipa` to App Store Connect with `xcrun altool` and the API key at `key`.

    altool reads the key from `API_PRIVATE_KEYS_DIR/AuthKey_<key id>.p8`.
    """
    run(
        [
            "xcrun",
            "altool",
            "--upload-app",
            "--type",
            "ios",
            "--file",
            ipa,
            "--apiKey",
            key_id,
            "--apiIssuer",
            issuer,
        ],
        env={"API_PRIVATE_KEYS_DIR": str(key.parent)},
    )


# ---- release, promotion, trial --------------------------------------------------------------


def release(
    run: Runner,
    *,
    distribution: Path,
    version: str,
    out: Path,
    branding: Path,
    environ: Mapping[str, str],
    http: storeapi.Http = storeapi.urllib_http,
) -> Path:
    """The iOS release step of release.yml with the publisher's accounts (module docstring).

    Raises:
        StepError: a secret is missing, a check, the signature or the upload fails.
        appdist.DistributionError: the distribution is not usable or holds placeholders.
    """
    require_macos()
    secret = require_env(IOS_SECRETS, environ, "release-apple")
    cfg = appdist.load(distribution)
    appdist.require_publisher_values(cfg)
    app_version = appdist.app_version(version)
    api_key = _api_key(secret["APPLE_NOTARY_KEY"])
    store = storeapi.AppStoreConnect(
        api_key, secret["APPLE_NOTARY_KEY_ID"], secret["APPLE_NOTARY_ISSUER_ID"], http=http
    )
    profiles = {rel: _decode(secret[name], name) for rel, name in PROFILES.items()}
    with tempfile.TemporaryDirectory(prefix="release-ios-") as tmp:
        workdir = Path(tmp)
        archive = build_archive(run, cfg, app_version, branding, workdir)
        check_archive(archive, cfg, app_version)
        p12 = workdir / "distribution.p12"
        p12.write_bytes(_decode(secret["APPLE_DISTRIBUTION_P12"], "APPLE_DISTRIBUTION_P12"))
        p12.chmod(0o600)
        with temporary_keychain(run, workdir) as (keychain, password):
            identity = import_identity(
                run, keychain, password, p12, secret["APPLE_DISTRIBUTION_P12_PASSWORD"]
            )
            signer = Signer(identity=identity, keychain=keychain, timestamp=False)
            sign_app(run, archive, cfg, signer, profiles, workdir)
        verify_signature(run, archive.app, cfg)
        ipa = make_ipa(
            run, archive.app, out, f"{cfg['bundle_id']}-{app_version.short}.ipa", workdir
        )
        keys = workdir / "keys"
        keys.mkdir()
        key = keys / f"AuthKey_{secret['APPLE_NOTARY_KEY_ID']}.p8"
        key.write_text(api_key, encoding="utf-8")
        key.chmod(0o600)
        upload(run, ipa, key, secret["APPLE_NOTARY_KEY_ID"], secret["APPLE_NOTARY_ISSUER_ID"])
    app_id = store.app_id(cfg["bundle_id"])
    build_id = store.processed_build(app_id, app_version.build)
    store.add_to_group(app_id, build_id, BETA_GROUP)
    return ipa


def promote(
    *,
    distribution: Path,
    build: int,
    environ: Mapping[str, str],
    http: storeapi.Http = storeapi.urllib_http,
) -> None:
    """Add the processed build `build` to the TestFlight group `stable` (18 5 rule 1).

    Raises:
        StepError: the API key is missing, or App Store Connect refuses.
        appdist.DistributionError: the distribution is not usable or holds placeholders.
    """
    names = ("APPLE_NOTARY_KEY", "APPLE_NOTARY_KEY_ID", "APPLE_NOTARY_ISSUER_ID")
    secret = require_env(names, environ, "release-apple")
    cfg = appdist.load(distribution)
    appdist.require_publisher_values(cfg)
    store = storeapi.AppStoreConnect(
        _api_key(secret["APPLE_NOTARY_KEY"]),
        secret["APPLE_NOTARY_KEY_ID"],
        secret["APPLE_NOTARY_ISSUER_ID"],
        http=http,
    )
    app_id = store.app_id(cfg["bundle_id"])
    store.add_to_group(app_id, store.processed_build(app_id, build), STABLE_GROUP)


def trial(run: Runner, *, out: Path, branding: Path) -> list[str]:
    """Build and check an unsigned archive with a test publisher (module docstring).

    Raises:
        StepError: a step or check fails.
    """
    require_macos()
    cfg = appdist.load(appdist.TEMPLATE)
    version = appdist.app_version(TRIAL_VERSION)
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="trial-ios-") as tmp:
        workdir = Path(tmp)
        archive = build_archive(run, cfg, version, branding, workdir)
        checks = check_archive(archive, cfg, version)
        signed = {
            rel: distribution_entitlements(archive.entitlements[rel], cfg, bundle_id)
            for rel, bundle_id in bundle_ids(cfg).items()
        }
    report = {
        "version": version.short,
        "build": version.build,
        "checks": checks,
        "distribution_entitlements": signed,
        "not_proven": (
            "the Apple Distribution signature, the upload to App Store Connect and TestFlight "
            "need the publisher's Apple Developer Program account (OQ-08)"
        ),
    }
    (out / "trial-ios.json").write_text(
        json.dumps(report, indent=2, default=str) + "\n", encoding="utf-8"
    )
    return checks


def main(argv: list[str] | None = None, run: Runner | None = None) -> int:
    """Run `release`, `promote` or `trial` (see the module docstring); returns the exit code."""
    parser = argparse.ArgumentParser(description="iOS release and trial of the phone app.")
    sub = parser.add_subparsers(dest="command", required=True)
    r = sub.add_parser("release", help="build, sign, upload one version to TestFlight")
    r.add_argument("--version", required=True)
    r.add_argument("--out", type=Path, required=True)
    r.add_argument("--distribution", type=Path, default=appdist.TEMPLATE)
    r.add_argument("--branding", type=Path, default=DEFAULT_BRANDING)
    p = sub.add_parser("promote", help="add a processed build to the TestFlight group stable")
    p.add_argument("--build", type=int, required=True)
    p.add_argument("--distribution", type=Path, default=appdist.TEMPLATE)
    t = sub.add_parser("trial", help="the unsigned archive and its checks with a test publisher")
    t.add_argument("--out", type=Path, required=True)
    t.add_argument("--branding", type=Path, default=DEFAULT_BRANDING)
    args = parser.parse_args(argv)
    runner = run or Runner()
    try:
        if args.command == "release":
            ipa = release(
                runner,
                distribution=args.distribution,
                version=args.version,
                out=args.out,
                branding=args.branding,
                environ=os.environ,
            )
            print(f"release-ios: {ipa.name} in TestFlight, group {BETA_GROUP}")
        elif args.command == "promote":
            promote(distribution=args.distribution, build=args.build, environ=os.environ)
            print(f"release-ios: build {args.build} in TestFlight, group {STABLE_GROUP}")
        else:
            checks = trial(runner, out=args.out, branding=args.branding)
            print(f"trial-ios: {len(checks)} checks passed")
    except (StepError, appdist.DistributionError) as err:
        return fail(str(err))
    return 0


if __name__ == "__main__":
    sys.exit(main())
