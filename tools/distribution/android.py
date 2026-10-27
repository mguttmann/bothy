# SPDX-License-Identifier: Apache-2.0
"""Android release of the phone app (brief M7-13, ADR 0037, docs/spec/18-release.md 5 and 8).

`make -C apps/client release-android VERSION=<X.Y.Z> OUT=<dir>` (release.yml job build-android):

1. the secrets of the environment `release-android` (ANDROID_SECRETS) and a distribution
   without placeholders; the version `X.Y.Z` gives the version code `X * 1000000 + Y * 1000 + Z`
   (`appdist.app_version`);
2. the upload key store of the publisher into the temporary directory of the run, and the
   Firebase configuration of the instance's push (`ANDROID_GOOGLE_SERVICES_JSON`), which must
   name `firebase.project_id` and `android.package_name`, as `android/app/google-services.json`
   for the build only;
3. the build branding with the application ID and the update path of the distribution, then
   `flutter build appbundle --release` with the version, signed by Gradle with the upload key
   (the properties `uploadStoreFile`, `uploadStorePassword`, `uploadKeyAlias`,
   `uploadKeyPassword` of android/app/build.gradle.kts);
4. the checks (`verify_bundle`): `jarsigner -verify` passes and the bundle's signer is the upload
   key (SHA-256 of the certificate), `bundletool validate`, the manifest names the application ID,
   version code and version name, and the universal APK that `bundletool build-apks` makes from
   the bundle with the same key passes `apksigner verify` with the same certificate;
5. the bundle `<application ID>-<version>.aab` in OUT, and its release on the internal test track
   of Google Play (the channel beta, 18 5) through the service account
   (`storeapi.PlayPublisher`).

`android.py promote --version-code N` releases the same build on `android.stable_track` (the
channel stable, 18 5 rule 1: no rebuild).

`make -C apps/client trial-android` runs steps 3 and 4 with a test publisher (D-W-127, brief
Design note 2): a throwaway upload key made with `keytool` in the temporary directory of the
run, the template distribution, no Firebase configuration and no upload. It writes
`<out>/trial-android.json`.

Usage:
  android.py release --version X.Y.Z --out DIR [--distribution FILE] [--branding FILE]
  android.py promote --version-code N [--distribution FILE]
  android.py trial --out DIR [--branding FILE]
Exit codes: 0 done, 1 a step failed (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import base64
import binascii
import contextlib
import hashlib
import json
import os
import re
import secrets
import shutil
import sys
import tempfile
import urllib.request
import zipfile
from collections.abc import Iterator, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import appdist
import storeapi
from common import (
    CLIENT,
    DEFAULT_BRANDING,
    REPO,
    Runner,
    StepError,
    branded_tree,
    branding_for,
    fail,
    require_env,
)

# The pinned bundletool release: version, file and its SHA-256 (brief M7-13 rule 9 of M7-12).
BUNDLETOOL_VERSION = "1.18.3"
BUNDLETOOL_URL = (
    "https://github.com/google/bundletool/releases/download/"
    f"{BUNDLETOOL_VERSION}/bundletool-all-{BUNDLETOOL_VERSION}.jar"
)
BUNDLETOOL_SHA256 = "a099cfa1543f55593bc2ed16a70a7c67fe54b1747bb7301f37fdfd6d91028e29"
CACHE = REPO / ".cache" / "bundletool"

ANDROID_SECRETS = (
    "ANDROID_UPLOAD_KEYSTORE",
    "ANDROID_UPLOAD_KEYSTORE_PASSWORD",
    "ANDROID_UPLOAD_KEY_ALIAS",
    "ANDROID_UPLOAD_KEY_PASSWORD",
    "ANDROID_GOOGLE_SERVICES_JSON",
    "PLAY_SERVICE_ACCOUNT_JSON",
)
# The Play track of the channel beta (18 5); stable is `android.stable_track`.
BETA_TRACK = "internal"
TRIAL_VERSION = "0.1.0"
GOOGLE_SERVICES = CLIENT / "android" / "app" / "google-services.json"
BUNDLE = CLIENT / "build" / "app" / "outputs" / "bundle" / "release" / "app-release.aab"


@dataclass(frozen=True)
class UploadKey:
    """The upload key of a build: the key store file of the run and its names and passwords."""

    store: Path
    store_password: str
    alias: str
    key_password: str


@dataclass(frozen=True)
class Checked:
    """What the checks of a bundle found.

    Attributes:
        sha256: the SHA-256 of the signer's certificate, lower-case hex.
        package: the application ID of the manifest.
        version_code: the version code of the manifest.
        version_name: the version name of the manifest.
    """

    sha256: str
    package: str
    version_code: int
    version_name: str


# ---- tools ----------------------------------------------------------------------------------


def _download(url: str, target: Path) -> None:  # pragma: no cover - network
    if not url.startswith("https://github.com/google/bundletool/"):
        raise StepError(f"refusing a download from {url}")
    with urllib.request.urlopen(url, timeout=300) as resp:  # noqa: S310  # nosec B310
        target.write_bytes(resp.read())


def fetch_bundletool(cache: Path = CACHE, download: Any = _download) -> Path:
    """Return the pinned bundletool jar, downloading it once; checked on every use.

    Raises:
        StepError: the file does not match BUNDLETOOL_SHA256.
    """
    jar = cache / f"bundletool-all-{BUNDLETOOL_VERSION}.jar"
    if not jar.is_file():
        cache.mkdir(parents=True, exist_ok=True)
        partial = jar.with_suffix(".partial")
        download(BUNDLETOOL_URL, partial)
        partial.replace(jar)
    digest = hashlib.sha256(jar.read_bytes()).hexdigest()
    if digest != BUNDLETOOL_SHA256:
        jar.unlink()
        raise StepError(
            f"bundletool {BUNDLETOOL_VERSION}: checksum {digest}, pinned {BUNDLETOOL_SHA256}"
        )
    return jar


def java_tool(name: str, environ: Mapping[str, str] = os.environ) -> str:
    """Return `name` (`java`, `keytool`, `jarsigner`) of JAVA_HOME, else the one on PATH."""
    home = environ.get("JAVA_HOME")
    if home:
        candidate = Path(home) / "bin" / name
        if candidate.is_file():
            return str(candidate)
    return name


def apksigner(environ: Mapping[str, str] = os.environ) -> str:
    """Return `apksigner` of the newest build tools of the Android SDK.

    Raises:
        StepError: neither ANDROID_HOME nor ANDROID_SDK_ROOT names an SDK with build tools.
    """
    for var in ("ANDROID_HOME", "ANDROID_SDK_ROOT"):
        root = environ.get(var)
        if not root:
            continue
        tools = sorted(
            (p for p in (Path(root) / "build-tools").glob("*") if (p / "apksigner").is_file()),
            key=lambda p: [int(x) if x.isdigit() else 0 for x in re.split(r"[.-]", p.name)],
        )
        if tools:
            return str(tools[-1] / "apksigner")
    raise StepError("apksigner not found: set ANDROID_HOME to an Android SDK with build tools")


def _fingerprint(text: str) -> str:
    """Return the first `SHA256:` fingerprint of keytool's output, lower-case hex."""
    match = re.search(r"SHA-?256:\s*([0-9A-Fa-f:]{95})", text)
    if match is None:
        raise StepError("keytool printed no SHA-256 fingerprint")
    return match.group(1).replace(":", "").lower()


def key_fingerprint(run: Runner, key: UploadKey) -> str:
    """Return the SHA-256 of the certificate of the upload key."""
    out = run(
        [
            java_tool("keytool"),
            "-list",
            "-v",
            "-keystore",
            key.store,
            "-alias",
            key.alias,
            "-storepass:env",
            "BOTHY_UPLOAD_STORE_PASSWORD",
        ],
        env={"BOTHY_UPLOAD_STORE_PASSWORD": key.store_password},
        capture=True,
    )
    return _fingerprint(out.stdout)


# ---- build ----------------------------------------------------------------------------------


def check_google_services(text: str, cfg: dict[str, Any]) -> None:
    """Refuse a Firebase configuration of another project or app.

    Raises:
        StepError: no JSON, another `project_id` than `firebase.project_id`, or no client for
            `android.package_name`.
    """
    try:
        doc = json.loads(text)
        project = doc["project_info"]["project_id"]
        packages = {c["client_info"]["android_client_info"]["package_name"] for c in doc["client"]}
    except (ValueError, KeyError, TypeError) as err:
        raise StepError("ANDROID_GOOGLE_SERVICES_JSON is no google-services.json") from err
    if project != cfg["firebase"]["project_id"]:
        raise StepError(
            f"ANDROID_GOOGLE_SERVICES_JSON is of the Firebase project {project}, "
            f"the distribution names {cfg['firebase']['project_id']}"
        )
    if cfg["android"]["package_name"] not in packages:
        raise StepError(f"ANDROID_GOOGLE_SERVICES_JSON has no app {cfg['android']['package_name']}")


@contextlib.contextmanager
def firebase_config(text: str | None, target: Path | None = None) -> Iterator[None]:
    """Put the Firebase configuration in place for the build and remove it afterwards.

    `target` is `android/app/google-services.json` of the app unless given. Without a
    configuration the app builds without push (docs/decisions.d/M5-05.md D-M5-05-5). A file that
    is already there is refused: it would change the build without the release asking for it.

    Raises:
        StepError: `target` exists before the build.
    """
    target = target or GOOGLE_SERVICES
    if target.exists():
        raise StepError(f"{target} exists; remove it, the release writes its own")
    if text is not None:
        target.write_text(text, encoding="utf-8")
    try:
        yield
    finally:
        target.unlink(missing_ok=True)


def build_bundle(
    run: Runner,
    cfg: dict[str, Any],
    version: appdist.AppVersion,
    branding: Path,
    key: UploadKey,
    workdir: Path,
) -> Path:
    """Build the signed App Bundle of `version` for the publisher of `cfg`; return it.

    Raises:
        StepError: the build fails or writes no bundle.
    """
    release_branding = branding_for(branding, cfg, workdir / "branding.yaml", "android")
    BUNDLE.unlink(missing_ok=True)
    with branded_tree(run, branding, release_branding):
        run(
            [
                "flutter",
                "build",
                "appbundle",
                "--release",
                f"--build-name={version.short}",
                f"--build-number={version.build}",
                "--dart-define-from-file=build/branding/dart_defines.json",
            ],
            cwd=CLIENT,
            env={
                "ORG_GRADLE_PROJECT_uploadStoreFile": str(key.store),
                "ORG_GRADLE_PROJECT_uploadStorePassword": key.store_password,
                "ORG_GRADLE_PROJECT_uploadKeyAlias": key.alias,
                "ORG_GRADLE_PROJECT_uploadKeyPassword": key.key_password,
            },
        )
    if not BUNDLE.is_file():
        raise StepError(f"the build wrote no bundle at {BUNDLE}")
    target = workdir / BUNDLE.name
    shutil.copyfile(BUNDLE, target)
    return target


def _manifest(run: Runner, bundletool: list[str], bundle: Path, xpath: str) -> str:
    out = run(
        [*bundletool, "dump", "manifest", f"--bundle={bundle}", f"--xpath={xpath}"],
        capture=True,
    )
    return out.stdout.strip()


def verify_signer(run: Runner, bundle: Path, key: UploadKey) -> str:
    """Check that `jarsigner` verifies `bundle` and that `key` signed it; return the SHA-256.

    Raises:
        StepError: the signature does not verify or is another key's.
    """
    want = key_fingerprint(run, key)
    verified = run([java_tool("jarsigner"), "-verify", bundle], capture=True)
    if "jar verified." not in verified.stdout:
        raise StepError(f"jarsigner does not verify {bundle.name}: {verified.stdout.strip()}")
    signer = _fingerprint(
        run([java_tool("keytool"), "-printcert", "-jarfile", bundle], capture=True).stdout
    )
    if signer != want:
        raise StepError(f"{bundle.name} is signed by {signer}, the upload key is {want}")
    return want


def verify_bundle(
    run: Runner,
    bundle: Path,
    key: UploadKey,
    cfg: dict[str, Any],
    version: appdist.AppVersion,
    workdir: Path,
    bundletool_jar: Path,
) -> Checked:
    """Check the signature, structure and identity of `bundle` (step 4 of the module docstring).

    Raises:
        StepError: a check fails; the message names it.
    """
    want = verify_signer(run, bundle, key)
    bundletool = [java_tool("java"), "-jar", str(bundletool_jar)]
    run([*bundletool, "validate", f"--bundle={bundle}"], capture=True)
    package = _manifest(run, bundletool, bundle, "/manifest/@package")
    code = _manifest(run, bundletool, bundle, "/manifest/@android:versionCode")
    name = _manifest(run, bundletool, bundle, "/manifest/@android:versionName")
    expected = (cfg["android"]["package_name"], str(version.build), version.short)
    if (package, code, name) != expected:
        raise StepError(
            f"{bundle.name} names {package} {code} {name}, the release builds " + " ".join(expected)
        )
    passwords = workdir / "apks-passwords"
    passwords.mkdir(exist_ok=True)
    (passwords / "store").write_text(key.store_password, encoding="utf-8")
    (passwords / "key").write_text(key.key_password, encoding="utf-8")
    apks = workdir / "universal.apks"
    apks.unlink(missing_ok=True)
    try:
        run(
            [
                *bundletool,
                "build-apks",
                f"--bundle={bundle}",
                f"--output={apks}",
                "--mode=universal",
                f"--ks={key.store}",
                f"--ks-key-alias={key.alias}",
                f"--ks-pass=file:{passwords / 'store'}",
                f"--key-pass=file:{passwords / 'key'}",
            ],
        )
    finally:
        shutil.rmtree(passwords, ignore_errors=True)
    with zipfile.ZipFile(apks) as archive:
        universal = workdir / "universal.apk"
        universal.write_bytes(archive.read("universal.apk"))
    out = run([apksigner(), "verify", "--print-certs", universal], capture=True).stdout
    match = re.search(r"Signer #1 certificate SHA-256 digest: ([0-9a-f]{64})", out)
    if match is None or match.group(1) != want:
        raise StepError(f"apksigner: the universal APK is not signed by the upload key: {out}")
    return Checked(sha256=want, package=package, version_code=int(code), version_name=name)


# ---- release --------------------------------------------------------------------------------


def _keystore(workdir: Path, text: str) -> Path:
    path = workdir / "upload.keystore"
    try:
        path.write_bytes(base64.b64decode(text, validate=True))
    except (binascii.Error, ValueError) as err:
        raise StepError("ANDROID_UPLOAD_KEYSTORE is no base64 key store") from err
    path.chmod(0o600)
    return path


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
    """The release step of release.yml with the publisher's accounts (module docstring).

    Raises:
        StepError: a secret is missing, a check or the upload fails.
        appdist.DistributionError: the distribution is not usable or holds placeholders.
    """
    secret = require_env(ANDROID_SECRETS, environ, "release-android")
    cfg = appdist.load(distribution)
    appdist.require_publisher_values(cfg)
    app_version = appdist.app_version(version)
    check_google_services(secret["ANDROID_GOOGLE_SERVICES_JSON"], cfg)
    play = storeapi.PlayPublisher(
        secret["PLAY_SERVICE_ACCOUNT_JSON"], cfg["android"]["package_name"], http=http
    )
    jar = fetch_bundletool()
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="release-android-") as tmp:
        workdir = Path(tmp)
        key = UploadKey(
            store=_keystore(workdir, secret["ANDROID_UPLOAD_KEYSTORE"]),
            store_password=secret["ANDROID_UPLOAD_KEYSTORE_PASSWORD"],
            alias=secret["ANDROID_UPLOAD_KEY_ALIAS"],
            key_password=secret["ANDROID_UPLOAD_KEY_PASSWORD"],
        )
        with firebase_config(secret["ANDROID_GOOGLE_SERVICES_JSON"]):
            bundle = build_bundle(run, cfg, app_version, branding, key, workdir)
        verify_bundle(run, bundle, key, cfg, app_version, workdir, jar)
        target = out / f"{cfg['android']['package_name']}-{app_version.short}.aab"
        shutil.copyfile(bundle, target)
    play.upload(
        target.read_bytes(),
        version_code=app_version.build,
        name=app_version.short,
        track=BETA_TRACK,
    )
    return target


def promote(
    *,
    distribution: Path,
    version_code: int,
    environ: Mapping[str, str],
    http: storeapi.Http = storeapi.urllib_http,
) -> str:
    """Release the uploaded build `version_code` on `android.stable_track`; return the track.

    Raises:
        StepError: the service account is missing or Play refuses.
        appdist.DistributionError: the distribution is not usable or holds placeholders.
    """
    secret = require_env(("PLAY_SERVICE_ACCOUNT_JSON",), environ, "release-android")
    cfg = appdist.load(distribution)
    appdist.require_publisher_values(cfg)
    major, rest = divmod(version_code, 1_000_000)
    minor, patch = divmod(rest, 1_000)
    track = str(cfg["android"]["stable_track"])
    storeapi.PlayPublisher(
        secret["PLAY_SERVICE_ACCOUNT_JSON"], cfg["android"]["package_name"], http=http
    ).promote(version_code=version_code, name=f"{major}.{minor}.{patch}", track=track)
    return track


# ---- trial ----------------------------------------------------------------------------------


def trial_key(run: Runner, workdir: Path) -> UploadKey:
    """Make a throwaway upload key with keytool in `workdir` (D-W-127): RSA 2048, two days."""
    password = secrets.token_urlsafe(18)
    key = UploadKey(
        store=workdir / "trial-upload.p12",
        store_password=password,
        alias="trial-upload",
        key_password=password,
    )
    run(
        [
            java_tool("keytool"),
            "-genkeypair",
            "-keystore",
            key.store,
            "-storetype",
            "PKCS12",
            "-alias",
            key.alias,
            "-keyalg",
            "RSA",
            "-keysize",
            "2048",
            "-validity",
            "2",
            "-dname",
            "CN=Test Publisher",
            "-storepass:env",
            "BOTHY_UPLOAD_STORE_PASSWORD",
        ],
        env={"BOTHY_UPLOAD_STORE_PASSWORD": password},
    )
    return key


def trial(run: Runner, *, out: Path, branding: Path) -> Checked:
    """Build and check a bundle with a test publisher (module docstring); nothing is uploaded.

    Raises:
        StepError: a step or check fails.
    """
    cfg = appdist.load(appdist.TEMPLATE)
    version = appdist.app_version(TRIAL_VERSION)
    jar = fetch_bundletool()
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="trial-android-") as tmp:
        workdir = Path(tmp)
        key = trial_key(run, workdir)
        with firebase_config(None):
            bundle = build_bundle(run, cfg, version, branding, key, workdir)
        checked = verify_bundle(run, bundle, key, cfg, version, workdir, jar)
        # The probe that must fail: another upload key does not pass the signer check.
        other = workdir / "other"
        other.mkdir()
        try:
            verify_signer(run, bundle, trial_key(run, other))
        except StepError:
            pass
        else:
            raise StepError("the signer check accepted a key that did not sign the bundle")
    report = {
        "bundletool": BUNDLETOOL_VERSION,
        "version": version.short,
        "version_code": checked.version_code,
        "package": checked.package,
        "signer_sha256": checked.sha256,
        "checks": [
            "jarsigner -verify passes and the signer is the upload key",
            "another upload key is refused by the signer check",
            "bundletool validate passes",
            "the manifest names the application ID, version code and version name",
            "the universal APK of bundletool build-apks passes apksigner verify with the key",
        ],
        "not_proven": "the upload to Google Play needs the publisher's account (OQ-08)",
    }
    (out / "trial-android.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return checked


def main(argv: list[str] | None = None, run: Runner | None = None) -> int:
    """Run `release`, `promote` or `trial` (see the module docstring); returns the exit code."""
    parser = argparse.ArgumentParser(description="Android release and trial of the phone app.")
    sub = parser.add_subparsers(dest="command", required=True)
    r = sub.add_parser("release", help="build, sign, check and upload one version")
    r.add_argument("--version", required=True)
    r.add_argument("--out", type=Path, required=True)
    r.add_argument("--distribution", type=Path, default=appdist.TEMPLATE)
    r.add_argument("--branding", type=Path, default=DEFAULT_BRANDING)
    p = sub.add_parser("promote", help="release an uploaded build on the stable track")
    p.add_argument("--version-code", type=int, required=True)
    p.add_argument("--distribution", type=Path, default=appdist.TEMPLATE)
    t = sub.add_parser("trial", help="the build and its checks with a test publisher")
    t.add_argument("--out", type=Path, required=True)
    t.add_argument("--branding", type=Path, default=DEFAULT_BRANDING)
    args = parser.parse_args(argv)
    runner = run or Runner()
    try:
        if args.command == "release":
            target = release(
                runner,
                distribution=args.distribution,
                version=args.version,
                out=args.out,
                branding=args.branding,
                environ=os.environ,
            )
            print(f"release-android: {target.name} on the Play track {BETA_TRACK}")
        elif args.command == "promote":
            track = promote(
                distribution=args.distribution,
                version_code=args.version_code,
                environ=os.environ,
            )
            print(f"release-android: version code {args.version_code} on the Play track {track}")
        else:
            checked = trial(runner, out=args.out, branding=args.branding)
            print(
                f"trial-android: {checked.package} {checked.version_name} "
                f"({checked.version_code}) signed by {checked.sha256}"
            )
    except (StepError, appdist.DistributionError) as err:
        return fail(str(err))
    return 0


if __name__ == "__main__":
    sys.exit(main())
