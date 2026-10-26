# SPDX-License-Identifier: Apache-2.0
"""macOS release of the desktop app with Sparkle 2 (brief M7-12, ADR 0037, 18-release.md 8).

`make -C apps/client release-apple VERSION=<X.Y.Z> OUT=<dir>` (release.yml job build-apple):

1. the build branding with the publisher's bundle identifier (`branding_for`), then
   `flutter build macos --release` with the version and build number of `appdist.app_version`
   and the team of `apple.team_id` (so the App Group and keychain groups carry its prefix);
2. Sparkle.framework of the pinned release (SPARKLE_VERSION, checked against SPARKLE_SHA256)
   into `Contents/Frameworks`, the Sparkle keys into `Info.plist` (`appdist.sparkle_info_plist`);
3. the provisioning profiles, then a signature of every nested code item from the inside out
   and of the app with the hardened runtime, the entitlements of the build plus Sparkle's
   installer service, `aps-environment` production and the team's application identifier;
   `codesign --verify --deep --strict` must pass and the signature must name `apple.team_id`;
4. notarization with `xcrun notarytool submit --wait` and `xcrun stapler staple`, then
   `spctl --assess` as a required check;
5. the archive (`ditto`, zip), its EdDSA signature from Sparkle's `sign_update`, and the signed
   appcast with the new entry in the channel beta (`appdist.add_items`), appended to `--appcast`
   when given.

Every secret comes from the environment `release-apple` (APPLE_SECRETS); without one the step
stops before the build with the names of the missing ones. Template placeholders in the
distribution stop it too.

`make -C apps/client trial-apple` runs the same steps with a test publisher instead of the
owner's accounts (D-W-127, brief Design note 1 and 2): a self-signed code signing identity in a
temporary keychain (trusted for code signing for the run only), a throwaway EdDSA key, the
update addresses on 127.0.0.1; no notarization, and `spctl --assess` runs and its expected
rejection is recorded. It builds 0.1.0 and 0.1.1, promotes 0.1.0 to stable, and then drives
Sparkle's own updater (tools/distribution/macos/update_harness.swift, built against the pinned
framework) against an installed 0.1.0:

- the default channel offers nothing (0.1.1 is beta only);
- a changed 0.1.1 archive is refused and 0.1.0 stays;
- the channel beta replaces 0.1.0 with 0.1.1, whose signature verifies;
- after promoting 0.1.1, the default channel replaces a fresh 0.1.0 with the same build.

The result is written to `<out>/trial-apple.json`.

Usage:
  apple.py release --version X.Y.Z --out DIR [--distribution FILE] [--branding FILE]
                   [--appcast FILE]
  apple.py trial --out DIR [--branding FILE]
Exit codes: 0 done, 1 a step failed (named on stderr), 2 usage error or not a macOS host.
"""

from __future__ import annotations

import argparse
import base64
import contextlib
import datetime
import hashlib
import json
import os
import plistlib
import secrets
import shutil
import sys
import tarfile
import tempfile
import urllib.request
from collections.abc import Iterator, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import appdist
import yaml
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
    serve,
    wait_for,
)
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import pkcs12
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID

# The pinned Sparkle release: version, archive and its SHA-256 (brief M7-12 rule 9).
SPARKLE_VERSION = "2.10.1"
SPARKLE_URL = (
    "https://github.com/sparkle-project/Sparkle/releases/download/"
    f"{SPARKLE_VERSION}/Sparkle-{SPARKLE_VERSION}.tar.xz"
)
SPARKLE_SHA256 = "1a856d2a9edb8e628de88121d7be04cf0e2167f8641650b78fca00b14244cf27"
CACHE = REPO / ".cache" / "sparkle"
HARNESS = Path(__file__).resolve().parent / "macos" / "update_harness.swift"

# The secrets of the environment release-apple (docs/operations/release.md 4).
APPLE_SECRETS = (
    "APPLE_DEVELOPER_ID_P12",
    "APPLE_DEVELOPER_ID_P12_PASSWORD",
    "APPLE_PROVISIONING_PROFILE",
    "APPLE_EXTENSION_PROVISIONING_PROFILE",
    "APPLE_NOTARY_KEY",
    "APPLE_NOTARY_KEY_ID",
    "APPLE_NOTARY_ISSUER_ID",
    "SPARKLE_ED_PRIVATE_KEY",
)
# Versions of the trial: an older build and the one that replaces it.
TRIAL_VERSIONS = ("0.1.0", "0.1.1")
# The bound of the removal of the test certificate's trust (it can wait for a dialog).
REMOVE_TRUST_PATIENCE_SECONDS = 60.0
# Exit codes of update_harness.swift.
HARNESS_UPDATED = 0
HARNESS_NO_UPDATE = 4
# Suffixes of nested code that is signed as a bundle.
_BUNDLE_SUFFIXES = (".app", ".appex", ".framework", ".xpc", ".bundle")
_MACHO_MAGIC = {
    b"\xcf\xfa\xed\xfe",
    b"\xce\xfa\xed\xfe",
    b"\xca\xfe\xba\xbe",
    b"\xbe\xba\xfe\xca",
}


@dataclass
class Signer:
    """A code signing identity in a keychain of this run.

    Attributes:
        identity: SHA-1 hash of the certificate (`security find-identity`).
        keychain: the temporary keychain that holds it.
        timestamp: ask Apple's timestamp service (a Developer ID signature needs it; a test
            publisher's does not get one).
    """

    identity: str
    keychain: Path
    timestamp: bool


@dataclass
class Notary:
    """The App Store Connect API key notarytool uses (files of this run)."""

    key: Path
    key_id: str
    issuer: str


@dataclass
class Release:
    """What one macOS release step produced."""

    version: appdist.AppVersion
    app: Path
    archive: Path
    appcast: Path
    assessment: str = ""
    checks: list[str] = field(default_factory=list)


def require_macos() -> None:
    """Stop unless this is a macOS host (the build needs Xcode, signing needs codesign).

    Raises:
        StepError: on any other system.
    """
    if sys.platform != "darwin":
        raise StepError("release-apple and trial-apple need a macOS host with Xcode")


# ---- Sparkle --------------------------------------------------------------------------------


def _download(url: str, target: Path) -> None:  # pragma: no cover - network
    if not url.startswith("https://github.com/sparkle-project/"):
        raise StepError(f"refusing a download from {url}")
    with urllib.request.urlopen(url, timeout=120) as resp:  # noqa: S310  # nosec B310
        target.write_bytes(resp.read())


def fetch_sparkle(cache: Path = CACHE, download: Any = _download) -> Path:
    """Return the directory of the pinned Sparkle release, downloading it once.

    The archive is checked against SPARKLE_SHA256 before it is unpacked; a cached copy is
    checked again on every use.

    Raises:
        StepError: the archive does not match the pinned checksum.
    """
    archive = cache / f"Sparkle-{SPARKLE_VERSION}.tar.xz"
    target = cache / SPARKLE_VERSION
    if not archive.is_file():
        cache.mkdir(parents=True, exist_ok=True)
        partial = archive.with_suffix(".partial")
        download(SPARKLE_URL, partial)
        partial.replace(archive)
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    if digest != SPARKLE_SHA256:
        archive.unlink()
        raise StepError(f"Sparkle {SPARKLE_VERSION}: checksum {digest}, pinned {SPARKLE_SHA256}")
    if not (target / "Sparkle.framework").is_dir():
        shutil.rmtree(target, ignore_errors=True)
        target.mkdir(parents=True)
        with tarfile.open(archive) as tar:
            tar.extractall(target, filter="tar")
    return target


# ---- build ----------------------------------------------------------------------------------


def build_app(
    run: Runner,
    cfg: dict[str, Any],
    version: appdist.AppVersion,
    branding: Path,
    workdir: Path,
) -> Path:
    """Build the macOS app of `version` for the publisher of `cfg`; return the bundle.

    Raises:
        StepError: the build fails or does not produce exactly one app bundle.
    """
    release_branding = branding_for(branding, cfg, workdir / "branding.yaml")
    with branded_tree(run, branding, release_branding):
        run(
            [
                "flutter",
                "build",
                "macos",
                "--release",
                f"--build-name={version.short}",
                f"--build-number={version.build}",
                "--dart-define-from-file=build/branding/dart_defines.json",
            ],
            cwd=CLIENT,
            # Flutter passes FLUTTER_XCODE_* to xcodebuild: the team gives the App Group and the
            # keychain groups their prefix; the build itself stays signed to run locally and
            # is signed for distribution afterwards.
            env={
                "FLUTTER_XCODE_DEVELOPMENT_TEAM": cfg["apple"]["team_id"],
                "FLUTTER_XCODE_CODE_SIGN_STYLE": "Manual",
                "FLUTTER_XCODE_CODE_SIGN_IDENTITY": "-",
                "FLUTTER_XCODE_PROVISIONING_PROFILE_SPECIFIER": "",
            },
        )
    products = CLIENT / "build" / "macos" / "Build" / "Products" / "Release"
    apps = sorted(products.glob("*.app"))
    if len(apps) != 1:
        raise StepError(f"expected one app bundle in {products}, found {len(apps)}")
    target = workdir / apps[0].name
    shutil.rmtree(target, ignore_errors=True)
    run(["ditto", apps[0], target])
    return target


def embed_sparkle(run: Runner, app: Path, sparkle: Path) -> None:
    """Copy Sparkle.framework into the app (ditto keeps its symbolic links)."""
    frameworks = app / "Contents" / "Frameworks"
    frameworks.mkdir(parents=True, exist_ok=True)
    run(["ditto", sparkle / "Sparkle.framework", frameworks / "Sparkle.framework"])


def configure_info_plist(app: Path, cfg: dict[str, Any], version: appdist.AppVersion) -> None:
    """Write the Sparkle keys into the app's Info.plist and check identity and version.

    Raises:
        StepError: the bundle identifier or the version of the build is not the expected one.
    """
    path = app / "Contents" / "Info.plist"
    info = plistlib.loads(path.read_bytes())
    expected = {
        "CFBundleIdentifier": cfg["bundle_id"],
        "CFBundleShortVersionString": version.short,
        "CFBundleVersion": str(version.build),
    }
    for key, want in expected.items():
        if info.get(key) != want:
            raise StepError(f"{path}: {key} is {info.get(key)!r}, expected {want!r}")
    info.update(appdist.sparkle_info_plist(cfg))
    path.write_bytes(plistlib.dumps(info))


# ---- signing --------------------------------------------------------------------------------


def entitlements_of(run: Runner, path: Path) -> dict[str, Any]:
    """Return the entitlements a code item was signed with (empty when it has none)."""
    done = run(["codesign", "-d", "--entitlements", "-", "--xml", path], capture=True)
    text = done.stdout.strip()
    if not text:
        return {}
    data = plistlib.loads(text.encode("utf-8"))
    return data if isinstance(data, dict) else {}


def _is_macho(path: Path) -> bool:
    with path.open("rb") as f:
        return f.read(4) in _MACHO_MAGIC


def _bundle_executable(path: Path) -> bool:
    """Return whether `path` is the main executable of the bundle around it."""
    parent = path.parent
    if parent.name == "MacOS" and parent.parent.name == "Contents":
        info = parent.parent / "Info.plist"
        if info.is_file():
            executable = plistlib.loads(info.read_bytes()).get("CFBundleExecutable")
            return bool(executable == path.name)
    framework = parent.parent.parent if parent.parent.name == "Versions" else None
    return framework is not None and framework.name == f"{path.name}.framework"


def code_items(app: Path) -> list[Path]:
    """Return the nested code of `app` to sign, the deepest first (signing goes inside out).

    Bundles (frameworks, apps, app extensions, XPC services) are signed as a whole; any other
    Mach-O file (a helper tool such as Sparkle's Autoupdate, a dylib) on its own. Symbolic links
    (a framework's `Versions/Current`) are skipped, so nothing is signed twice.
    """
    items: list[Path] = []
    for root, dirs, files in os.walk(app / "Contents"):
        here = Path(root)
        for name in dirs:
            if name.endswith(_BUNDLE_SUFFIXES) and not (here / name).is_symlink():
                items.append(here / name)
        for name in files:
            path = here / name
            if path.is_symlink() or not path.is_file() or _bundle_executable(path):
                continue
            if _is_macho(path):
                items.append(path)
    return sorted(items, key=lambda p: (-len(p.parts), str(p)))


def _with_identity(entitlements: dict[str, Any], team: str, bundle_id: str) -> dict[str, Any]:
    result = dict(entitlements)
    result["com.apple.application-identifier"] = f"{team}.{bundle_id}"
    result["com.apple.developer.team-identifier"] = team
    return result


def app_entitlements(built: dict[str, Any], cfg: dict[str, Any]) -> dict[str, Any]:
    """Return the entitlements of the distributed app from those of the build.

    The build's entitlements (Release.entitlements with the team's prefixes) plus Sparkle's
    installer service, `aps-environment` production (a Developer ID profile is a distribution
    profile, docs/decisions.d/M5-04.md D-M5-04-10) and the team's application identifier, which
    the provisioning profile names.
    """
    result = _with_identity(built, cfg["apple"]["team_id"], cfg["bundle_id"])
    result.update(appdist.sparkle_entitlements(cfg["bundle_id"]))
    if "com.apple.developer.aps-environment" in result:
        result["com.apple.developer.aps-environment"] = "production"
    return result


def _sign(run: Runner, signer: Signer, path: Path, entitlements: Path | None) -> None:
    cmd: list[str | Path] = [
        "codesign",
        "--force",
        "--options",
        "runtime",
        "--keychain",
        signer.keychain,
        "--sign",
        signer.identity,
        "--timestamp" if signer.timestamp else "--timestamp=none",
    ]
    if entitlements is None:
        cmd.append("--preserve-metadata=entitlements")
    else:
        cmd += ["--entitlements", entitlements]
    run([*cmd, path])


def sign_app(
    run: Runner,
    app: Path,
    cfg: dict[str, Any],
    signer: Signer,
    workdir: Path,
    profiles: Mapping[str, bytes] | None = None,
) -> None:
    """Sign `app` and its nested code for distribution.

    Args:
        run: the command runner.
        app: the app bundle.
        cfg: the distribution.
        signer: the identity.
        workdir: where the entitlement files of this run go.
        profiles: provisioning profiles by bundle path relative to `app` (`""` for the app,
            `Contents/PlugIns/<name>.appex` for an extension); none for the test publisher.
    """
    team = cfg["apple"]["team_id"]
    for relative, data in (profiles or {}).items():
        target = (app / relative if relative else app) / "Contents" / "embedded.provisionprofile"
        target.write_bytes(data)
    built = entitlements_of(run, app)
    for index, item in enumerate(code_items(app)):
        if item.suffix == ".appex":
            bundle_id = plistlib.loads((item / "Contents" / "Info.plist").read_bytes())[
                "CFBundleIdentifier"
            ]
            ent = workdir / f"extension-{index}.entitlements"
            ent.write_bytes(
                plistlib.dumps(_with_identity(entitlements_of(run, item), team, bundle_id))
            )
            _sign(run, signer, item, ent)
        else:
            _sign(run, signer, item, None)
    ent = workdir / "app.entitlements"
    ent.write_bytes(plistlib.dumps(app_entitlements(built, cfg)))
    _sign(run, signer, app, ent)


def verify_signature(run: Runner, app: Path, team: str | None) -> None:
    """Run `codesign --verify --deep --strict`; with `team`, check the signing team.

    Raises:
        StepError: the signature does not verify or names another team.
    """
    run(["codesign", "--verify", "--deep", "--strict", "--verbose=2", app])
    if team is not None:
        details = run(["codesign", "-dv", "--verbose=2", app], capture=True)
        if f"TeamIdentifier={team}" not in details.stdout + details.stderr:
            raise StepError(f"{app.name} is not signed by the team {team}")


def notarize(run: Runner, app: Path, notary: Notary, workdir: Path) -> None:
    """Notarize `app` with notarytool and staple the ticket.

    Raises:
        StepError: notarytool does not report the submission as Accepted.
    """
    upload = workdir / f"{app.stem}-notarize.zip"
    run(["ditto", "-c", "-k", "--keepParent", app, upload])
    done = run(
        [
            "xcrun",
            "notarytool",
            "submit",
            upload,
            "--key",
            notary.key,
            "--key-id",
            notary.key_id,
            "--issuer",
            notary.issuer,
            "--wait",
            "--output-format",
            "json",
        ],
        capture=True,
    )
    status = json.loads(done.stdout or "{}").get("status")
    if status != "Accepted":
        raise StepError(f"notarization of {app.name} ended with {status!r}")
    run(["xcrun", "stapler", "staple", app])
    run(["xcrun", "stapler", "validate", app])


def assess(run: Runner, app: Path) -> tuple[bool, str]:
    """Run Gatekeeper's `spctl --assess` on `app`; return whether it accepts and its output."""
    done = run(
        ["spctl", "--assess", "--type", "execute", "--verbose=4", app], capture=True, check=False
    )
    return done.returncode == 0, (done.stdout + done.stderr).strip()


def make_archive(
    run: Runner, app: Path, out: Path, version: appdist.AppVersion, sparkle: Path, key: str
) -> appdist.Archive:
    """Zip `app` into `out` and sign the archive with Sparkle's `sign_update`.

    The private key goes to `sign_update` on its standard input, never on a command line.
    """
    name = f"{app.stem}-{version.short}.zip"
    archive = out / name
    run(["ditto", "-c", "-k", "--sequesterRsrc", "--keepParent", app, archive])
    done = run(
        [sparkle / "bin" / "sign_update", "-p", "--ed-key-file", "-", archive],
        stdin=key,
        capture=True,
    )
    return appdist.Archive(name=name, length=archive.stat().st_size, signature=done.stdout.strip())


# ---- the release step -----------------------------------------------------------------------


@dataclass
class Plan:
    """Inputs of one release step."""

    cfg: dict[str, Any]
    version: appdist.AppVersion
    out: Path
    branding: Path
    signer: Signer
    ed_key: str
    appcast: Path | None = None
    notary: Notary | None = None
    profiles: Mapping[str, bytes] | None = None


def release_step(run: Runner, plan: Plan, sparkle: Path, workdir: Path) -> Release:
    """Build, sign, notarize (with a notary), archive and add the appcast entry of one version.

    Without a notary (the trial) `spctl --assess` only records its verdict; with one it must
    accept the app.

    Raises:
        StepError: a step fails.
    """
    plan.out.mkdir(parents=True, exist_ok=True)
    key = appdist.private_key(plan.ed_key)
    appdist.check_key_pair(plan.cfg, key)
    app = build_app(run, plan.cfg, plan.version, plan.branding, workdir)
    embed_sparkle(run, app, sparkle)
    configure_info_plist(app, plan.cfg, plan.version)
    sign_app(run, app, plan.cfg, plan.signer, workdir, plan.profiles)
    real = plan.notary is not None
    verify_signature(run, app, plan.cfg["apple"]["team_id"] if real else None)
    if plan.notary is not None:
        notarize(run, app, plan.notary, workdir)
    accepted, assessment = assess(run, app)
    if real and not accepted:
        raise StepError(f"spctl --assess rejects {app.name}: {assessment}")
    archive = make_archive(run, app, plan.out, plan.version, sparkle, plan.ed_key)
    appdist.verify_archive(plan.cfg, (plan.out / archive.name).read_bytes(), archive.signature)
    if plan.appcast is not None:
        current = plan.appcast.read_bytes()
        appdist.verify_feed(plan.cfg, current)
    else:
        current = appdist.new_appcast(app.stem, plan.cfg)
    item = appdist.appcast_item(plan.cfg, app_name=app.stem, version=plan.version, archive=archive)
    feed = appdist.add_items(current, [item], key)
    appcast = plan.out / "appcast.xml"
    appcast.write_bytes(feed)
    appdist.verify_feed(plan.cfg, feed)
    return Release(plan.version, app, plan.out / archive.name, appcast, assessment)


# ---- keychain and identities ----------------------------------------------------------------


@contextlib.contextmanager
def temporary_keychain(run: Runner, workdir: Path) -> Iterator[tuple[Path, str]]:
    """Create an unlocked keychain for this run, in the search list; yield path and password.

    The search list gets the keychain in front of the user's own and its previous content back
    at the end, when the keychain is deleted, also after a failure. Nothing goes into a login
    keychain (brief M7-12 Boundaries).
    """
    keychain = workdir / "release.keychain-db"
    password = secrets.token_urlsafe(24)
    run(["security", "create-keychain", "-p", password, keychain], redact=[password])
    previous = run(["security", "list-keychains", "-d", "user"], capture=True).stdout
    listed = [line.strip().strip('"') for line in previous.splitlines() if line.strip()]
    try:
        run(["security", "set-keychain-settings", keychain])
        run(["security", "unlock-keychain", "-p", password, keychain], redact=[password])
        run(["security", "list-keychains", "-d", "user", "-s", keychain, *listed])
        yield keychain, password
    finally:
        run(["security", "list-keychains", "-d", "user", "-s", *listed], check=False)
        run(["security", "delete-keychain", keychain], check=False)


def import_identity(
    run: Runner, keychain: Path, password: str, p12: Path, p12_password: str
) -> str:
    """Import a PKCS#12 identity into `keychain` for codesign; return its SHA-1 hash.

    Raises:
        StepError: the keychain holds no code signing identity afterwards.
    """
    run(
        ["security", "import", p12, "-k", keychain, "-P", p12_password, "-T", "/usr/bin/codesign"],
        redact=[p12_password],
    )
    run(
        [
            "security",
            "set-key-partition-list",
            "-S",
            "apple-tool:,apple:,codesign:",
            "-s",
            "-k",
            password,
            keychain,
        ],
        capture=True,
        redact=[password],
    )
    listing = run(["security", "find-identity", "-p", "codesigning", keychain], capture=True)
    for line in listing.stdout.splitlines():
        parts = line.split()
        if len(parts) >= 2 and parts[0].endswith(")") and len(parts[1]) == 40:
            return parts[1]
    raise StepError("the keychain holds no code signing identity")


def trial_identity(workdir: Path, name: str) -> tuple[Path, Path, str]:
    """Create a self-signed code signing certificate; return the PKCS#12, the PEM, the password.

    The PKCS#12 uses the legacy algorithms `security import` reads.
    """
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, name)])
    now = datetime.datetime.now(datetime.UTC)
    delta = datetime.timedelta
    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(subject)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - delta(minutes=5))
        .not_valid_after(now + delta(days=2))
        .add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True)
        .add_extension(
            x509.KeyUsage(
                digital_signature=True,
                content_commitment=False,
                key_encipherment=False,
                data_encipherment=False,
                key_agreement=False,
                key_cert_sign=False,
                crl_sign=False,
                encipher_only=False,
                decipher_only=False,
            ),
            critical=True,
        )
        .add_extension(x509.ExtendedKeyUsage([ExtendedKeyUsageOID.CODE_SIGNING]), critical=True)
        .sign(key, hashes.SHA256())
    )
    password = secrets.token_urlsafe(16)
    encryption = (
        serialization.PrivateFormat.PKCS12.encryption_builder()
        .kdf_rounds(50000)
        .key_cert_algorithm(pkcs12.PBES.PBESv1SHA1And3KeyTripleDESCBC)
        .hmac_hash(hashes.SHA1())  # noqa: S303 - the PKCS#12 MAC macOS `security import` reads
        .build(password.encode())
    )
    p12 = workdir / "test-publisher.p12"
    p12.write_bytes(
        pkcs12.serialize_key_and_certificates(b"test publisher", key, cert, None, encryption)
    )
    pem = workdir / "test-publisher.pem"
    pem.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
    return p12, pem, password


@contextlib.contextmanager
def trial_publisher(run: Runner, workdir: Path) -> Iterator[Signer]:
    """A self-signed code signing identity for this run only (brief Design note 1).

    codesign uses an identity only when its certificate is trusted for code signing, so the
    certificate gets that trust in the admin trust settings (`sudo -n`, as on the CI runner) and
    loses it at the end, with the keychain.
    """
    p12, pem, p12_password = trial_identity(workdir, "Trial Test Publisher")
    with temporary_keychain(run, workdir) as (keychain, password):
        identity = import_identity(run, keychain, password, p12, p12_password)
        run(
            [
                "sudo",
                "-n",
                "security",
                "add-trusted-cert",
                "-d",
                "-r",
                "trustRoot",
                "-p",
                "codeSign",
                "-k",
                keychain,
                pem,
            ]
        )
        try:
            yield Signer(identity=identity, keychain=keychain, timestamp=False)
        finally:
            # remove-trusted-cert can wait for an authorization dialog on a runner; the trust
            # then stays for a certificate whose key went with the keychain, which signs nothing.
            run(
                ["sudo", "-n", "security", "remove-trusted-cert", "-d", pem],
                check=False,
                patience=REMOVE_TRUST_PATIENCE_SECONDS,
                optional=True,
            )


def _b64file(workdir: Path, name: str, text: str) -> Path:
    path = workdir / name
    path.write_bytes(base64.b64decode(text))
    path.chmod(0o600)
    return path


def release(
    run: Runner,
    *,
    distribution: Path,
    version: str,
    out: Path,
    branding: Path,
    appcast: Path | None,
    environ: Mapping[str, str],
) -> Release:
    """The release step of release.yml with the publisher's accounts (see the module docstring).

    Raises:
        StepError: a secret is missing, the distribution holds placeholders, or a step fails.
        appdist.DistributionError: the distribution or a key is not usable.
    """
    require_macos()
    secret = require_env(APPLE_SECRETS, environ, "release-apple")
    cfg = appdist.load(distribution)
    appdist.require_publisher_values(cfg)
    app_version = appdist.app_version(version)
    sparkle = fetch_sparkle()
    with tempfile.TemporaryDirectory(prefix="release-apple-") as tmp:
        workdir = Path(tmp)
        p12 = _b64file(workdir, "developer-id.p12", secret["APPLE_DEVELOPER_ID_P12"])
        notary = Notary(
            key=_b64file(workdir, "notary.p8", secret["APPLE_NOTARY_KEY"]),
            key_id=secret["APPLE_NOTARY_KEY_ID"],
            issuer=secret["APPLE_NOTARY_ISSUER_ID"],
        )
        profiles = {
            "": base64.b64decode(secret["APPLE_PROVISIONING_PROFILE"]),
            "Contents/PlugIns/NotificationService.appex": base64.b64decode(
                secret["APPLE_EXTENSION_PROVISIONING_PROFILE"]
            ),
        }
        with temporary_keychain(run, workdir) as (keychain, password):
            identity = import_identity(
                run, keychain, password, p12, secret["APPLE_DEVELOPER_ID_P12_PASSWORD"]
            )
            plan = Plan(
                cfg=cfg,
                version=app_version,
                out=out,
                branding=branding,
                signer=Signer(identity=identity, keychain=keychain, timestamp=True),
                ed_key=secret["SPARKLE_ED_PRIVATE_KEY"],
                appcast=appcast,
                notary=notary,
                profiles=profiles,
            )
            return release_step(run, plan, sparkle, workdir)


# ---- the trial ------------------------------------------------------------------------------


def build_harness(run: Runner, sparkle: Path, workdir: Path) -> Path:
    """Compile update_harness.swift against the pinned Sparkle.framework; return the binary."""
    binary = workdir / "update_harness"
    run(
        [
            "xcrun",
            "swiftc",
            "-swift-version",
            "5",
            "-O",
            "-F",
            sparkle,
            "-framework",
            "Sparkle",
            "-Xlinker",
            "-rpath",
            "-Xlinker",
            sparkle,
            "-o",
            binary,
            HARNESS,
        ]
    )
    return binary


def bundle_version(app: Path) -> str:
    """Return CFBundleVersion of an installed app ("" while the installer is replacing it)."""
    try:
        return str(
            plistlib.loads((app / "Contents" / "Info.plist").read_bytes())["CFBundleVersion"]
        )
    except (OSError, KeyError, plistlib.InvalidFileException):
        return ""


@dataclass
class Trial:
    """State of one trial run: the runner, the harness and where builds get installed."""

    run: Runner
    harness: Path
    install: Path
    checks: list[str] = field(default_factory=list)

    def install_app(self, archive: Path, name: str) -> Path:
        """Install the app of `archive` fresh (as a user unpacks the download)."""
        shutil.rmtree(self.install, ignore_errors=True)
        self.install.mkdir(parents=True)
        self.run(["ditto", "-x", "-k", archive, self.install])
        return self.install / name

    def update(self, app: Path, channels: list[str]) -> int:
        """Run Sparkle's updater against `app` with `channels`; return the harness's exit code."""
        cmd: list[str | Path] = [self.harness, app]
        for channel in channels:
            cmd += ["--channel", channel]
        return self.run(cmd, check=False).returncode

    def expect(self, ok: bool, check: str) -> None:
        """Record `check` as passed, or fail the trial with it.

        Raises:
            StepError: `ok` is false.
        """
        if not ok:
            raise StepError(f"trial check failed: {check}")
        print(f"trial: ok: {check}", flush=True)
        self.checks.append(check)


def update_test(
    trial: Trial, cfg: dict[str, Any], old: Release, new: Release, www: Path, ed: Ed25519PrivateKey
) -> None:
    """The update test of brief Design note 2 against the served `www` (module docstring).

    Raises:
        StepError: a check fails.
    """
    name = old.app.name
    want_old, want_new = str(old.version.build), str(new.version.build)
    feed = www / "appcast.xml"
    shutil.copy2(new.appcast, feed)
    for release_ in (old, new):
        shutil.copy2(release_.archive, www / release_.archive.name)
    appdist.verify_feed(cfg, feed.read_bytes())
    for entry in appdist.appcast_entries(feed.read_bytes()):
        archive = www / entry.url.rsplit("/", 1)[-1]
        appdist.verify_archive(cfg, archive.read_bytes(), entry.signature)
    trial.expect(True, "the appcast and both archives verify with the public EdDSA key")

    app = trial.install_app(old.archive, name)
    code = trial.update(app, [])
    trial.expect(
        code == HARNESS_NO_UPDATE and bundle_version(app) == want_old,
        f"the default channel offers no beta build (exit {code})",
    )

    served = www / new.archive.name
    good = served.read_bytes()
    served.write_bytes(good[:-1] + bytes([good[-1] ^ 0xFF]))
    code = trial.update(app, [appdist.BETA])
    trial.expect(
        code not in (HARNESS_UPDATED, HARNESS_NO_UPDATE) and bundle_version(app) == want_old,
        f"a changed archive is refused and {old.version.short} stays (exit {code})",
    )
    served.write_bytes(good)

    code = trial.update(app, [appdist.BETA])
    trial.expect(code == HARNESS_UPDATED, f"the channel beta finds {new.version.short}")
    wait_for(f"{new.version.short} installed", lambda: bundle_version(app) == want_new)
    trial.run(["codesign", "--verify", "--deep", "--strict", app])
    trial.expect(True, f"the channel beta replaced {old.version.short} by {new.version.short}")

    feed.write_bytes(appdist.promote_appcast(feed.read_bytes(), new.version.build, ed))
    app = trial.install_app(old.archive, name)
    code = trial.update(app, [])
    trial.expect(
        code == HARNESS_UPDATED, "after promotion the default channel finds the same build"
    )
    wait_for(f"{new.version.short} installed from stable", lambda: bundle_version(app) == want_new)
    trial.expect(True, f"the promoted build {new.version.short} replaced {old.version.short}")


def trial(run: Runner, *, out: Path, branding: Path) -> list[str]:
    """Run the trial of the macOS chain with a test publisher; return the passed checks.

    Raises:
        StepError: a step or check fails.
    """
    require_macos()
    sparkle = fetch_sparkle()
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="trial-apple-") as tmp:
        workdir = Path(tmp)
        www = workdir / "www"
        www.mkdir()
        with serve(www) as base_url:
            key = Ed25519PrivateKey.generate()
            cfg = appdist.trial_distribution(base_url, appdist.public_key_text(key))
            (workdir / "distribution.yaml").write_text(yaml.safe_dump(cfg), encoding="utf-8")
            harness = build_harness(run, sparkle, workdir)
            ed_key = appdist.private_key_text(key)
            with trial_publisher(run, workdir) as signer:
                releases: list[Release] = []
                appcast: Path | None = None
                for version in TRIAL_VERSIONS:
                    plan = Plan(
                        cfg=cfg,
                        version=appdist.app_version(version),
                        out=out / version,
                        branding=branding,
                        signer=signer,
                        ed_key=ed_key,
                        appcast=appcast,
                    )
                    step = workdir / f"build-{version}"
                    step.mkdir()
                    releases.append(release_step(run, plan, sparkle, step))
                    # The older build is proven and promoted before the next one is published.
                    promoted = (
                        appdist.promote_appcast(
                            releases[-1].appcast.read_bytes(), plan.version.build, key
                        )
                        if version == TRIAL_VERSIONS[0]
                        else None
                    )
                    if promoted is not None:
                        appcast = workdir / "appcast-stable.xml"
                        appcast.write_bytes(promoted)
                session = Trial(run=run, harness=harness, install=workdir / "install")
                session.expect(True, "codesign --verify --deep --strict passes for both builds")
                update_test(session, cfg, releases[0], releases[1], www, key)
                assessment = releases[1].assessment
    report = {
        "sparkle": SPARKLE_VERSION,
        "versions": list(TRIAL_VERSIONS),
        "checks": session.checks,
        "spctl": assessment,
        "spctl_note": "a test publisher is no Developer ID; Gatekeeper rejects it by design",
    }
    (out / "trial-apple.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(f"trial-apple: spctl --assess (expected rejection): {assessment}", flush=True)
    return session.checks


def main(argv: list[str] | None = None, run: Runner | None = None) -> int:
    """Run `release` or `trial` (see the module docstring); returns the exit code."""
    parser = argparse.ArgumentParser(description="macOS release and trial of the desktop app.")
    sub = parser.add_subparsers(dest="command", required=True)
    r = sub.add_parser("release", help="build, sign, notarize and publish files for one version")
    r.add_argument("--version", required=True)
    r.add_argument("--out", type=Path, required=True)
    r.add_argument("--distribution", type=Path, default=appdist.TEMPLATE)
    r.add_argument("--branding", type=Path, default=DEFAULT_BRANDING)
    r.add_argument("--appcast", type=Path, help="the published appcast to add the entry to")
    t = sub.add_parser("trial", help="the whole chain with a test publisher")
    t.add_argument("--out", type=Path, required=True)
    t.add_argument("--branding", type=Path, default=DEFAULT_BRANDING)
    args = parser.parse_args(argv)
    runner = run or Runner()
    try:
        if args.command == "release":
            result = release(
                runner,
                distribution=args.distribution,
                version=args.version,
                out=args.out,
                branding=args.branding,
                appcast=args.appcast,
                environ=os.environ,
            )
            print(f"release-apple: {result.archive.name} and {result.appcast.name} in {args.out}")
        else:
            checks = trial(runner, out=args.out, branding=args.branding)
            print(f"trial-apple: {len(checks)} checks passed")
    except (StepError, appdist.DistributionError) as err:
        return fail(str(err))
    return 0


if __name__ == "__main__":
    sys.exit(main())
