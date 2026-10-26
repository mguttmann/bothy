# SPDX-License-Identifier: Apache-2.0
"""Publisher values of the apps and the update files built from them (brief M7-12, ADR 0037).

`apps/distribution/distribution.yaml` holds every value a publisher needs (docs/spec/18-release.md
8, R-RL-05); this module loads it against `distribution.schema.json`, refuses the placeholders of
the template for a real release, and renders the files the release steps of
tools/distribution/apple.py and windows.py write: the Sparkle keys of the macOS `Info.plist`, the
Sparkle appcast with its entries, the MSIX manifest and the `.appinstaller` file per channel. It
also promotes a build from beta to stable without a rebuild (18 5 rule 1): the appcast entry
loses its `sparkle:channel` element, `stable.appinstaller` points at the MSIX of
`beta.appinstaller`.

The appcast is signed as a whole with the EdDSA key of the publisher in Sparkle's format (the
content, then a trailing comment `<!-- sparkle-signatures:` with `edSignature:` and `length:`),
so the app (SURequireSignedFeed) accepts no changed feed; every change here signs it again.

Usage:
  appdist.py validate --distribution FILE [--release]
  appdist.py appcast-merge --appcast FILE --add FILE --ed-key-file FILE|- --distribution FILE
  appdist.py promote-appcast --appcast FILE --build N --ed-key-file FILE|- --distribution FILE
  appdist.py promote-appinstaller --beta FILE --stable FILE --distribution FILE
  appdist.py verify-appcast --appcast FILE --distribution FILE [--archives DIR]
Exit codes: 0 fine, 1 a finding (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import base64
import email.utils
import hashlib
import re
import sys
import uuid

# Only update files this tool wrote or the publisher's own; expat resolves no external entities.
import xml.etree.ElementTree as ET  # nosec B405
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from string import Template
from typing import Any
from xml.sax.saxutils import escape  # nosec B406 - escaping only

import jsonschema  # type: ignore[import-untyped]
import yaml
from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey, Ed25519PublicKey

REPO = Path(__file__).resolve().parents[2]
DIST_DIR = REPO / "apps" / "distribution"
TEMPLATE = DIST_DIR / "distribution.yaml"
SCHEMA = DIST_DIR / "distribution.schema.json"
TEMPLATES = DIST_DIR / "templates"

SPARKLE_NS = "http://www.andymatuschak.org/xml-namespaces/sparkle"
APPINSTALLER_NS = "http://schemas.microsoft.com/appx/appinstaller/2018"
# The oldest macOS and Windows the apps support (docs/spec/15-client-architecture.md 2: macOS
# 12, Windows 10 2004); the MSIX names the version it was tested with as the newest.
MINIMUM_MACOS = "12.0"
MINIMUM_WINDOWS = "10.0.19041.0"
MAX_WINDOWS_TESTED = "10.0.26100.0"
# The channel of a new build; promotion removes it (18 5).
BETA = "beta"

# Values of the template that a real release refuses: the key path and a test for the value.
_EXAMPLE_HOST = re.compile(r"^https?://([^/]+\.)?example\.(com|org|net)(/|$)")
PLACEHOLDERS: tuple[tuple[str, Any], ...] = (
    ("publisher.name", lambda v: v == "Example Publisher"),
    ("bundle_id", lambda v: v.startswith("com.example.")),
    ("apple.team_id", lambda v: v == "EXAMPLE000"),
    ("apple.apns_topic", lambda v: v.startswith("com.example.")),
    ("apple.sparkle.feed_url", lambda v: bool(_EXAMPLE_HOST.match(v))),
    ("apple.sparkle.download_url", lambda v: bool(_EXAMPLE_HOST.match(v))),
    ("apple.sparkle.public_ed_key", lambda v: base64.b64decode(v) == bytes(32)),
    ("windows.package_name", lambda v: v.startswith("com.example.")),
    ("windows.publisher", lambda v: v == "CN=Example Publisher"),
    ("windows.update_url", lambda v: bool(_EXAMPLE_HOST.match(v))),
    ("android.package_name", lambda v: v.startswith("com.example.")),
    ("firebase.project_id", lambda v: v.startswith("example-")),
)

# Sparkle's trailing signature block of a signed feed (SPUExtractSignedFeed.m of Sparkle 2.10).
_FEED_SIGNATURE_PREFIX = b"<!-- sparkle-signatures:\n"
_FEED_SIGNATURE = re.compile(
    rb"<!-- sparkle-signatures:\nedSignature: ([A-Za-z0-9+/=]+)\nlength: ([0-9]+)\n-->\s*\Z"
)


class DistributionError(Exception):
    """A distribution file, an update file or a key is not usable; the message names why."""


@dataclass(frozen=True)
class AppVersion:
    """The version of one app build on every platform (18 5 rule 1: no channel in it).

    Attributes:
        short: `X.Y.Z`, the version users see (CFBundleShortVersionString, flutter --build-name).
        build: the build number, `X * 1000000 + Y * 1000 + Z`, so it grows with the version
            (CFBundleVersion and sparkle:version, flutter --build-number).
        msix: the four-part MSIX version `X.Y.Z.0`.
    """

    short: str
    build: int
    msix: str


def app_version(version: str) -> AppVersion:
    """Return the platform versions of the app version `version`.

    Raises:
        DistributionError: not `X.Y.Z`, a pre-release (apps carry no channel in the version,
            18 5 rule 1), or a part too large for the build number or MSIX.

    Example: `app_version("1.2.3").build` is `1002003`.
    """
    match = re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", version)
    if match is None:
        raise DistributionError(
            f"app version {version!r} is not X.Y.Z: apps carry no channel or pre-release in the "
            "version (docs/spec/18-release.md 5 rule 1)"
        )
    major, minor, patch = (int(p) for p in match.groups())
    if minor > 999 or patch > 999 or major > 2146:
        raise DistributionError(f"app version {version!r}: minor and patch must stay below 1000")
    return AppVersion(
        short=version, build=major * 1_000_000 + minor * 1_000 + patch, msix=f"{version}.0"
    )


def _key_path(path: Any) -> str:
    return ".".join(str(p) for p in path)


def validate(doc: Any) -> dict[str, Any]:
    """Return `doc` when it matches distribution.schema.json.

    Raises:
        DistributionError: the first finding, naming the key (`apple.sparkle.feed_url is
            missing`, `windows.publisher: ... does not match ...`).
    """
    schema = yaml.safe_load(SCHEMA.read_text(encoding="utf-8"))
    validator = jsonschema.Draft202012Validator(schema)
    errors = sorted(validator.iter_errors(doc), key=lambda e: (list(e.absolute_path), e.message))
    if errors:
        error = errors[0]
        if error.validator == "required":
            missing = re.match(r"'([^']+)' is a required property", error.message)
            name = missing.group(1) if missing else "?"
            key = _key_path([*error.absolute_path, name])
            raise DistributionError(f"distribution: {key} is missing")
        where = _key_path(error.absolute_path) or "(top level)"
        raise DistributionError(f"distribution: {where}: {error.message}")
    if not isinstance(doc, dict):  # pragma: no cover - the schema requires an object
        raise DistributionError("distribution: not a mapping")
    return doc


def load(path: Path) -> dict[str, Any]:
    """Read and validate the distribution file at `path` (see `validate`).

    Raises:
        DistributionError: the file is missing, no YAML, or does not match the schema.
    """
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as err:
        raise DistributionError(f"distribution file {path}: {err.strerror}") from err
    try:
        doc = yaml.safe_load(text)
    except yaml.YAMLError as err:
        raise DistributionError(f"distribution file {path} is no YAML: {err}") from err
    return validate(doc)


def value(cfg: dict[str, Any], key: str) -> Any:
    """Return the value at the dotted `key` of `cfg` (`apple.sparkle.feed_url`)."""
    node: Any = cfg
    for part in key.split("."):
        node = node[part]
    return node


def placeholders(cfg: dict[str, Any]) -> list[str]:
    """Return the keys of `cfg` that still hold a placeholder of the template (PLACEHOLDERS)."""
    return [key for key, test in PLACEHOLDERS if test(value(cfg, key))]


def require_publisher_values(cfg: dict[str, Any]) -> None:
    """Refuse a real release with placeholder values.

    Raises:
        DistributionError: naming every key that still holds a value of the template.
    """
    found = placeholders(cfg)
    if found:
        raise DistributionError(
            "distribution: placeholder values of the template in "
            + ", ".join(found)
            + "; set the publisher's own values (apps/distribution/README.md)"
        )


def trial_distribution(base_url: str, public_key: str) -> dict[str, Any]:
    """Return the template with the trial's update host `base_url` and EdDSA `public_key`.

    The trials (D-W-127) keep every other value of the template, so they prove the template
    itself; `base_url` is `http://127.0.0.1:<port>/`, the only plain HTTP address the schema
    allows.
    """
    cfg = load(TEMPLATE)
    sparkle = cfg["apple"]["sparkle"]
    sparkle["feed_url"] = f"{base_url}appcast.xml"
    sparkle["download_url"] = base_url
    sparkle["public_ed_key"] = public_key
    cfg["windows"]["update_url"] = base_url
    return validate(cfg)


# ---- keys ---------------------------------------------------------------------------------


def private_key(text: str) -> Ed25519PrivateKey:
    """Return the EdDSA key of a Sparkle private key file (base64 of the 32-byte seed).

    Sparkle's `generate_keys -x` writes the seed; older keys hold the seed followed by the
    public key (64 bytes), of which the seed is used.

    Raises:
        DistributionError: not base64 of 32 or 64 bytes.
    """
    try:
        raw = base64.b64decode(text.strip(), validate=True)
    except ValueError as err:
        raise DistributionError("the EdDSA private key is not base64") from err
    if len(raw) not in (32, 64):
        raise DistributionError("the EdDSA private key is not 32 or 64 bytes")
    return Ed25519PrivateKey.from_private_bytes(raw[:32])


def public_key_text(key: Ed25519PrivateKey) -> str:
    """Return the base64 public key of `key`, the form of SUPublicEDKey."""
    raw = key.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
    return base64.b64encode(raw).decode("ascii")


def private_key_text(key: Ed25519PrivateKey) -> str:
    """Return the base64 seed of `key`, the form of Sparkle's private key file."""
    raw = key.private_bytes(
        serialization.Encoding.Raw,
        serialization.PrivateFormat.Raw,
        serialization.NoEncryption(),
    )
    return base64.b64encode(raw).decode("ascii")


def check_key_pair(cfg: dict[str, Any], key: Ed25519PrivateKey) -> None:
    """Make sure the private key belongs to `apple.sparkle.public_ed_key` of `cfg`.

    Raises:
        DistributionError: the keys differ, so the apps would refuse every update.
    """
    if public_key_text(key) != cfg["apple"]["sparkle"]["public_ed_key"]:
        raise DistributionError(
            "the EdDSA private key does not belong to apple.sparkle.public_ed_key; the apps "
            "would refuse every update signed with it"
        )


def _public_key(cfg: dict[str, Any]) -> Ed25519PublicKey:
    raw = base64.b64decode(cfg["apple"]["sparkle"]["public_ed_key"])
    return Ed25519PublicKey.from_public_bytes(raw)


def verify_archive(cfg: dict[str, Any], data: bytes, signature: str) -> None:
    """Check the EdDSA `signature` (base64) of an archive with the public key of `cfg`.

    Raises:
        DistributionError: the signature does not verify.
    """
    try:
        _public_key(cfg).verify(base64.b64decode(signature), data)
    except (InvalidSignature, ValueError) as err:
        raise DistributionError("the EdDSA signature of the archive does not verify") from err


# ---- macOS ----------------------------------------------------------------------------------


def sparkle_info_plist(cfg: dict[str, Any]) -> dict[str, Any]:
    """Return the Sparkle keys of the app's Info.plist for the publisher of `cfg`.

    Automatic checks are on without asking; the feed must be signed (SURequireSignedFeed) and an
    archive is checked before it is extracted. The installer runs through Sparkle's XPC service,
    because the app keeps its sandbox (docs/decisions.d/M7-12.md).
    """
    sparkle = cfg["apple"]["sparkle"]
    return {
        "SUFeedURL": sparkle["feed_url"],
        "SUPublicEDKey": sparkle["public_ed_key"],
        "SUEnableAutomaticChecks": True,
        "SURequireSignedFeed": True,
        "SUVerifyUpdateBeforeExtraction": True,
        "SUEnableInstallerLauncherService": True,
    }


def sparkle_entitlements(bundle_id: str) -> dict[str, Any]:
    """Return the entitlements a sandboxed app needs for Sparkle's installer service.

    Sparkle's documentation for sandboxed apps: the app talks to the installer it launched over
    the Mach services `<bundle id>-spks` and `<bundle id>-spki`.
    """
    return {
        "com.apple.security.temporary-exception.mach-lookup.global-name": [
            f"{bundle_id}-spks",
            f"{bundle_id}-spki",
        ]
    }


def _render(name: str, values: dict[str, Any], *, attribute: bool = False) -> str:
    text = (TEMPLATES / name).read_text(encoding="utf-8")
    quoted = {
        k: (escape(str(v), {'"': "&quot;"}) if attribute else escape(str(v)))
        for k, v in values.items()
    }
    return Template(text).substitute(quoted)


def _parse(data: bytes, what: str) -> ET.Element:
    parser = ET.XMLParser(target=ET.TreeBuilder(insert_comments=True))  # noqa: S314
    try:
        return ET.fromstring(data, parser=parser)  # noqa: S314  # nosec B314
    except ET.ParseError as err:
        raise DistributionError(f"{what} is no XML: {err}") from err


def feed_content(data: bytes) -> bytes:
    """Return a feed without Sparkle's trailing signature block (the signed content)."""
    at = data.rfind(_FEED_SIGNATURE_PREFIX)
    return data if at < 0 else data[:at]


def sign_feed(content: bytes, key: Ed25519PrivateKey) -> bytes:
    """Return `content` with Sparkle's signature block over it appended.

    The block is the one `sign_update` writes and Sparkle's SPUExtractSignedFeed reads: the
    EdDSA signature of every byte before the block and their number.
    """
    signature = base64.b64encode(key.sign(content)).decode("ascii")
    block = f"<!-- sparkle-signatures:\nedSignature: {signature}\nlength: {len(content)}\n-->\n"
    return content + block.encode("ascii")


def verify_feed(cfg: dict[str, Any], data: bytes) -> bytes:
    """Check the signature block of a feed with the public key of `cfg`; return the content.

    Raises:
        DistributionError: no signature block, a length that is not the content's, or a
            signature that does not verify.
    """
    match = _FEED_SIGNATURE.search(data)
    content = feed_content(data)
    if match is None:
        raise DistributionError("the appcast carries no signature block")
    if int(match.group(2)) != len(content):
        raise DistributionError("the signed length of the appcast is not its length")
    try:
        _public_key(cfg).verify(base64.b64decode(match.group(1)), content)
    except (InvalidSignature, ValueError) as err:
        raise DistributionError("the signature of the appcast does not verify") from err
    return content


def new_appcast(app_name: str, cfg: dict[str, Any]) -> bytes:
    """Return an appcast without entries for the publisher of `cfg` (templates/appcast.xml)."""
    text = _render(
        "appcast.xml", {"app_name": app_name, "feed_url": cfg["apple"]["sparkle"]["feed_url"]}
    )
    return text.encode("utf-8")


@dataclass(frozen=True)
class Archive:
    """One signed update archive of the macOS app.

    Attributes:
        name: file name below `apple.sparkle.download_url`.
        length: size in bytes.
        signature: base64 EdDSA signature from Sparkle's `sign_update`.
    """

    name: str
    length: int
    signature: str


def appcast_item(
    cfg: dict[str, Any],
    *,
    app_name: str,
    version: AppVersion,
    archive: Archive,
    published: datetime | None = None,
) -> ET.Element:
    """Return the appcast entry of a new build in the channel beta (templates/appcast-item.xml).

    Args:
        cfg: the distribution.
        app_name: the name of the app in the entry's title.
        version: the version of the build.
        archive: the signed archive below `apple.sparkle.download_url`.
        published: the publication date (now by default).

    Returns:
        The `item` element with `sparkle:channel` beta and the phased rollout interval of
        `cfg` (none when it is 0).
    """
    sparkle = cfg["apple"]["sparkle"]
    when = published or datetime.now(UTC)
    text = _render(
        "appcast-item.xml",
        {
            "app_name": app_name,
            "pub_date": email.utils.format_datetime(when),
            "build": version.build,
            "short_version": version.short,
            "minimum_system_version": MINIMUM_MACOS,
            "phased_rollout_interval": sparkle["phased_rollout_interval"],
            "archive_url": sparkle["download_url"] + archive.name,
            "archive_length": archive.length,
            "archive_signature": archive.signature,
        },
        attribute=True,
    )
    item = _parse(text.encode("utf-8"), "the appcast entry template")
    if sparkle["phased_rollout_interval"] == 0:
        phased = item.find(f"{{{SPARKLE_NS}}}phasedRolloutInterval")
        if phased is not None:
            item.remove(phased)
    return item


def _serialize(root: ET.Element) -> bytes:
    ET.register_namespace("sparkle", SPARKLE_NS)
    ET.indent(root, space="  ")
    body = ET.tostring(root, encoding="unicode")
    return ('<?xml version="1.0" encoding="utf-8"?>\n' + body + "\n").encode("utf-8")


def _channel(root: ET.Element) -> ET.Element:
    channel = root.find("channel")
    if root.tag != "rss" or channel is None:
        raise DistributionError("the appcast has no rss/channel element")
    return channel


def _builds(channel: ET.Element) -> list[str]:
    return [
        (item.findtext(f"{{{SPARKLE_NS}}}version") or "").strip()
        for item in channel.findall("item")
    ]


def add_items(appcast: bytes, items: list[ET.Element], key: Ed25519PrivateKey) -> bytes:
    """Return `appcast` with `items` appended to its channel, signed with `key`.

    Raises:
        DistributionError: the appcast is no appcast, or it already has an entry with the build
            number of one of `items` (a build is published once; 18 10: corrected forward).
    """
    root = _parse(feed_content(appcast), "the appcast")
    channel = _channel(root)
    present = set(_builds(channel))
    for item in items:
        build = (item.findtext(f"{{{SPARKLE_NS}}}version") or "").strip()
        if build in present:
            raise DistributionError(f"the appcast already has build {build}")
        present.add(build)
        channel.append(item)
    return sign_feed(_serialize(root), key)


def merge_appcast(cfg: dict[str, Any], appcast: bytes, new: bytes, key: Ed25519PrivateKey) -> bytes:
    """Return the published `appcast` with every entry of the release's appcast `new` added.

    Raises:
        DistributionError: `new` is not signed with the key of `cfg`, or one of its entries is
            not in the channel beta (a release publishes into beta; stable is a promotion).
    """
    content = verify_feed(cfg, new)
    items = list(_channel(_parse(content, "the new appcast")).findall("item"))
    for item in items:
        if (item.findtext(f"{{{SPARKLE_NS}}}channel") or "").strip() != BETA:
            build = (item.findtext(f"{{{SPARKLE_NS}}}version") or "").strip()
            raise DistributionError(f"build {build} of the new appcast is not in the channel beta")
    return add_items(appcast, items, key)


def promote_appcast(appcast: bytes, build: int, key: Ed25519PrivateKey) -> bytes:
    """Move build `build` from beta to stable: remove its `sparkle:channel`; sign again.

    The entry, the archive and its signature stay as they are, so stable gets the very build
    beta tested (18 5 rule 1, R-RL-03).

    Raises:
        DistributionError: no entry with this build number, or it is not in the channel beta.
    """
    root = _parse(feed_content(appcast), "the appcast")
    for item in _channel(root).findall("item"):
        if (item.findtext(f"{{{SPARKLE_NS}}}version") or "").strip() != str(build):
            continue
        channel = item.find(f"{{{SPARKLE_NS}}}channel")
        if channel is None or (channel.text or "").strip() != BETA:
            raise DistributionError(f"build {build} is not in the channel {BETA}")
        item.remove(channel)
        return sign_feed(_serialize(root), key)
    raise DistributionError(f"the appcast has no entry with build {build}")


@dataclass(frozen=True)
class FeedEntry:
    """One entry of an appcast as the update test needs it."""

    build: str
    channel: str | None
    url: str
    length: int
    signature: str


def appcast_entries(data: bytes) -> list[FeedEntry]:
    """Return the entries of an appcast (signed or not) in file order."""
    entries = []
    for item in _channel(_parse(feed_content(data), "the appcast")).findall("item"):
        enclosure = item.find("enclosure")
        if enclosure is None:
            raise DistributionError("an appcast entry has no enclosure")
        channel = item.findtext(f"{{{SPARKLE_NS}}}channel")
        entries.append(
            FeedEntry(
                build=(item.findtext(f"{{{SPARKLE_NS}}}version") or "").strip(),
                channel=channel.strip() if channel else None,
                url=enclosure.get("url", ""),
                length=int(enclosure.get("length", "0")),
                signature=enclosure.get(f"{{{SPARKLE_NS}}}edSignature", ""),
            )
        )
    return entries


# ---- Windows --------------------------------------------------------------------------------


def toast_activator_clsid(bundle_id: str) -> str:
    """Return the toast activation GUID of the app with `bundle_id`, in braces.

    The same derivation as `LocalNotificationPresenter.guidFor` of the app (D-M4-30-2): the first
    16 bytes of SHA-256 over `notifications:<bundle id>` with the version and variant bits of a
    name-based UUID, so the package registers the GUID the app initializes the plugin with.

    Example: `toast_activator_clsid("com.example.chat")`.
    """
    raw = bytearray(hashlib.sha256(f"notifications:{bundle_id}".encode()).digest()[:16])
    raw[6] = (raw[6] & 0x0F) | 0x50
    raw[8] = (raw[8] & 0x3F) | 0x80
    return "{" + str(uuid.UUID(bytes=bytes(raw))) + "}"


def appx_manifest(
    cfg: dict[str, Any], *, version: AppVersion, app_name: str, url_scheme: str, executable: str
) -> str:
    """Return the MSIX manifest of a build (templates/AppxManifest.xml).

    Args:
        cfg: the distribution (package name, publisher, publisher name, bundle identifier).
        version: the version of the build.
        app_name: the display name (build branding).
        url_scheme: the URL scheme of deep links (build branding).
        executable: the file name of the app's executable in the package.
    """
    return _render(
        "AppxManifest.xml",
        {
            "package_name": cfg["windows"]["package_name"],
            "publisher": cfg["windows"]["publisher"],
            "publisher_name": cfg["publisher"]["name"],
            "msix_version": version.msix,
            "app_name": app_name,
            "url_scheme": url_scheme,
            "executable": executable,
            "toast_clsid": toast_activator_clsid(cfg["bundle_id"]).strip("{}"),
            "minimum_windows_version": MINIMUM_WINDOWS,
            "max_version_tested": MAX_WINDOWS_TESTED,
        },
        attribute=True,
    )


def msix_name(cfg: dict[str, Any], version: AppVersion) -> str:
    """Return the file name of the MSIX of `version` (`<package>_<X.Y.Z.0>_x64.msix`)."""
    return f"{cfg['windows']['package_name']}_{version.msix}_x64.msix"


def appinstaller(cfg: dict[str, Any], *, version: AppVersion, channel: str) -> str:
    """Return `<channel>.appinstaller` pointing at the MSIX of `version`.

    Both live below `windows.update_url`; App Installer checks the file at every start of the
    app at most every `windows.hours_between_update_checks` hours.
    """
    base = cfg["windows"]["update_url"]
    return _appinstaller(cfg, channel, version.msix, base + msix_name(cfg, version))


def _appinstaller(cfg: dict[str, Any], channel: str, msix_version: str, msix_url: str) -> str:
    return _render(
        "channel.appinstaller",
        {
            "appinstaller_url": f"{cfg['windows']['update_url']}{channel}.appinstaller",
            "msix_version": msix_version,
            "package_name": cfg["windows"]["package_name"],
            "publisher": cfg["windows"]["publisher"],
            "msix_url": msix_url,
            "hours_between_update_checks": cfg["windows"]["hours_between_update_checks"],
        },
        attribute=True,
    )


@dataclass(frozen=True)
class AppInstallerTarget:
    """What an `.appinstaller` file points at."""

    uri: str
    version: str
    package_name: str
    publisher: str
    msix_uri: str


def appinstaller_target(text: str) -> AppInstallerTarget:
    """Return what the `.appinstaller` file `text` points at.

    Raises:
        DistributionError: no App Installer file with a MainPackage.
    """
    root = _parse(text.encode("utf-8"), "the .appinstaller file")
    package = root.find(f"{{{APPINSTALLER_NS}}}MainPackage")
    if root.tag != f"{{{APPINSTALLER_NS}}}AppInstaller" or package is None:
        raise DistributionError("the .appinstaller file has no AppInstaller/MainPackage")
    return AppInstallerTarget(
        uri=root.get("Uri", ""),
        version=root.get("Version", ""),
        package_name=package.get("Name", ""),
        publisher=package.get("Publisher", ""),
        msix_uri=package.get("Uri", ""),
    )


def promote_appinstaller(cfg: dict[str, Any], beta: str) -> str:
    """Return `stable.appinstaller` for the MSIX `beta.appinstaller` points at (18 5 rule 1).

    The MSIX is the same file, with the same version and signature; only the file's own address
    becomes the stable one.

    Raises:
        DistributionError: `beta` is not the beta file of this publisher.
    """
    target = appinstaller_target(beta)
    base = cfg["windows"]["update_url"]
    if target.uri != f"{base}{BETA}.appinstaller":
        raise DistributionError(f"not the beta file of {base}: {target.uri}")
    if target.package_name != cfg["windows"]["package_name"]:
        raise DistributionError(f"the beta file installs another package: {target.package_name}")
    if target.publisher != cfg["windows"]["publisher"]:
        raise DistributionError(f"the beta file names another publisher: {target.publisher}")
    return _appinstaller(cfg, "stable", target.version, target.msix_uri)


# ---- command line ---------------------------------------------------------------------------


def _read_key(source: str) -> Ed25519PrivateKey:
    text = sys.stdin.read() if source == "-" else Path(source).read_text(encoding="utf-8")
    return private_key(text)


def main(argv: list[str] | None = None) -> int:
    """Run one subcommand (see the module docstring); returns the exit code."""
    parser = argparse.ArgumentParser(description="Publisher values and update files of the apps.")
    sub = parser.add_subparsers(dest="command", required=True)
    v = sub.add_parser("validate", help="check a distribution file")
    v.add_argument("--distribution", type=Path, required=True)
    v.add_argument("--release", action="store_true", help="refuse template placeholders too")
    m = sub.add_parser("appcast-merge", help="add the entries of a release's appcast")
    m.add_argument("--appcast", type=Path, required=True)
    m.add_argument("--add", type=Path, required=True)
    m.add_argument("--ed-key-file", required=True)
    m.add_argument("--distribution", type=Path, required=True)
    p = sub.add_parser("promote-appcast", help="move a build of the appcast from beta to stable")
    p.add_argument("--appcast", type=Path, required=True)
    p.add_argument("--build", type=int, required=True)
    p.add_argument("--ed-key-file", required=True)
    p.add_argument("--distribution", type=Path, required=True)
    w = sub.add_parser("promote-appinstaller", help="point stable.appinstaller at the beta MSIX")
    w.add_argument("--beta", type=Path, required=True)
    w.add_argument("--stable", type=Path, required=True)
    w.add_argument("--distribution", type=Path, required=True)
    c = sub.add_parser("verify-appcast", help="check the feed and archive signatures")
    c.add_argument("--appcast", type=Path, required=True)
    c.add_argument("--distribution", type=Path, required=True)
    c.add_argument("--archives", type=Path, help="directory with the archives of the entries")
    args = parser.parse_args(argv)
    try:
        message = _command(args, load(args.distribution))
    except (DistributionError, OSError) as err:
        print(f"distribution: {err}", file=sys.stderr)
        return 1
    print(message)
    return 0


def _command(args: argparse.Namespace, cfg: dict[str, Any]) -> str:
    """Run the subcommand of `args` with the distribution `cfg`; return its result line."""
    if args.command == "validate":
        if args.release:
            require_publisher_values(cfg)
        return f"{args.distribution}: valid"
    if args.command in ("appcast-merge", "promote-appcast"):
        key = _read_key(args.ed_key_file)
        check_key_pair(cfg, key)
        current = args.appcast.read_bytes()
        verify_feed(cfg, current)
        if args.command == "appcast-merge":
            result = merge_appcast(cfg, current, args.add.read_bytes(), key)
        else:
            result = promote_appcast(current, args.build, key)
        args.appcast.write_bytes(result)
        return f"{args.appcast}: written and signed"
    if args.command == "promote-appinstaller":
        beta = args.beta.read_text(encoding="utf-8")
        args.stable.write_text(promote_appinstaller(cfg, beta), encoding="utf-8")
        return f"{args.stable}: points at the MSIX of {args.beta}"
    data = args.appcast.read_bytes()
    verify_feed(cfg, data)
    for entry in appcast_entries(data):
        if args.archives is not None:
            archive = args.archives / entry.url.rsplit("/", 1)[-1]
            verify_archive(cfg, archive.read_bytes(), entry.signature)
    return f"{args.appcast}: signatures verify"


if __name__ == "__main__":
    sys.exit(main())
