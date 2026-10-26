# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/distribution/appdist.py: schema, placeholders, versions, update files."""

from __future__ import annotations

import base64
import copy
import re
import xml.etree.ElementTree as ET
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import appdist as dist
import pytest
import yaml
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey


@pytest.fixture
def template() -> dict[str, Any]:
    return dist.load(dist.TEMPLATE)


@pytest.fixture
def ed() -> Ed25519PrivateKey:
    return Ed25519PrivateKey.from_private_bytes(bytes(range(32)))


@pytest.fixture
def cfg(template: dict[str, Any], ed: Ed25519PrivateKey) -> dict[str, Any]:
    cfg = copy.deepcopy(template)
    cfg["apple"]["sparkle"]["public_ed_key"] = dist.public_key_text(ed)
    return cfg


# ---- schema and placeholders ----------------------------------------------------------------


def test_template_validates_against_the_schema(template: dict[str, Any]) -> None:
    assert template["schema_version"] == 1
    assert template["windows"]["publisher"] == "CN=Example Publisher"
    assert template["apple"]["sparkle"]["feed_url"] == "https://updates.example.com/appcast.xml"


@pytest.mark.parametrize(
    "key",
    [
        "bundle_id",
        "publisher.name",
        "apple.team_id",
        "apple.apns_topic",
        "apple.sparkle.feed_url",
        "apple.sparkle.download_url",
        "apple.sparkle.public_ed_key",
        "apple.sparkle.phased_rollout_interval",
        "windows.package_name",
        "windows.publisher",
        "windows.update_url",
        "windows.hours_between_update_checks",
        "android.package_name",
        "firebase.project_id",
    ],
)
def test_a_missing_value_fails_and_names_its_key(template: dict[str, Any], key: str) -> None:
    doc = copy.deepcopy(template)
    *parents, last = key.split(".")
    node = doc
    for part in parents:
        node = node[part]
    del node[last]
    with pytest.raises(
        dist.DistributionError, match=rf"^distribution: {re.escape(key)} is missing$"
    ):
        dist.validate(doc)


@pytest.mark.parametrize(
    ("key", "bad"),
    [
        ("apple.team_id", "abc"),
        ("apple.sparkle.feed_url", "http://updates.example.com/appcast.xml"),
        ("apple.sparkle.download_url", "https://updates.example.com/macos"),
        ("apple.sparkle.public_ed_key", "not a key"),
        ("windows.publisher", "Example Publisher"),
        ("windows.package_name", "a"),
        ("windows.hours_between_update_checks", 256),
        ("bundle_id", "chat"),
    ],
)
def test_a_wrong_value_fails_and_names_its_key(
    template: dict[str, Any], key: str, bad: object
) -> None:
    doc = copy.deepcopy(template)
    *parents, last = key.split(".")
    node = doc
    for part in parents:
        node = node[part]
    node[last] = bad
    with pytest.raises(dist.DistributionError, match=rf"^distribution: {re.escape(key)}: "):
        dist.validate(doc)


def test_unknown_keys_fail(template: dict[str, Any]) -> None:
    doc = copy.deepcopy(template)
    doc["apple"]["signing_password"] = "x"
    with pytest.raises(
        dist.DistributionError, match=r"^distribution: apple: Additional properties"
    ):
        dist.validate(doc)


def test_load_reports_missing_and_broken_files(tmp_path: Path) -> None:
    with pytest.raises(dist.DistributionError, match=r"distribution file .*missing.yaml"):
        dist.load(tmp_path / "missing.yaml")
    broken = tmp_path / "broken.yaml"
    broken.write_text("a: [", encoding="utf-8")
    with pytest.raises(dist.DistributionError, match="is no YAML"):
        dist.load(broken)
    empty = tmp_path / "empty.yaml"
    empty.write_text("", encoding="utf-8")
    with pytest.raises(dist.DistributionError, match=r"\(top level\)"):
        dist.load(empty)


def test_a_duplicate_key_fails(tmp_path: Path) -> None:
    text = dist.TEMPLATE.read_text(encoding="utf-8") + "bundle_id: org.acme.chat\n"
    path = tmp_path / "distribution.yaml"
    path.write_text(text, encoding="utf-8")
    with pytest.raises(dist.DistributionError, match="duplicate key 'bundle_id'"):
        dist.load(path)


def test_the_trial_address_on_loopback_is_allowed_only_there(template: dict[str, Any]) -> None:
    doc = copy.deepcopy(template)
    doc["apple"]["sparkle"]["feed_url"] = "http://127.0.0.1:4711/appcast.xml"
    doc["windows"]["update_url"] = "http://127.0.0.1:4711/windows/"
    dist.validate(doc)
    doc["apple"]["sparkle"]["feed_url"] = "http://192.0.2.1:4711/appcast.xml"
    with pytest.raises(dist.DistributionError, match=r"apple\.sparkle\.feed_url"):
        dist.validate(doc)


def test_every_template_value_is_a_placeholder(template: dict[str, Any]) -> None:
    assert dist.placeholders(template) == [ed for ed, _ in dist.PLACEHOLDERS]
    with pytest.raises(dist.DistributionError, match=r"apple\.team_id.*firebase\.project_id"):
        dist.require_publisher_values(template)


def test_publisher_values_pass(template: dict[str, Any], ed: Ed25519PrivateKey) -> None:
    real = copy.deepcopy(template)
    real["publisher"]["name"] = "Acme Chat"
    real["bundle_id"] = real["apple"]["apns_topic"] = "org.acme.chat"
    real["apple"]["team_id"] = "AB12CD34EF"
    real["apple"]["sparkle"]["feed_url"] = "https://updates.acme.test/appcast.xml"
    real["apple"]["sparkle"]["download_url"] = "https://updates.acme.test/macos/"
    real["apple"]["sparkle"]["public_ed_key"] = dist.public_key_text(ed)
    real["windows"]["package_name"] = "Acme.Chat"
    real["windows"]["publisher"] = 'CN=Acme Chat, O="Acme, Inc.", C=DE'
    real["windows"]["update_url"] = "https://updates.acme.test/windows/"
    real["android"]["package_name"] = "org.acme.chat"
    real["firebase"]["project_id"] = "acme-chat"
    dist.validate(real)
    assert dist.placeholders(real) == []
    dist.require_publisher_values(real)


def test_the_schema_file_is_valid_json_schema() -> None:
    import jsonschema

    jsonschema.Draft202012Validator.check_schema(yaml.safe_load(dist.SCHEMA.read_text("utf-8")))


# ---- versions and keys ----------------------------------------------------------------------


def test_app_versions() -> None:
    assert dist.app_version("0.1.0") == dist.AppVersion("0.1.0", 1000, "0.1.0.0")
    assert dist.app_version("12.34.56").build == 12_034_056
    for bad in ("1.2", "1.2.3-beta.1", "01.2.3", "1.1000.0", "1.2.1000", "2147.0.0"):
        with pytest.raises(dist.DistributionError):
            dist.app_version(bad)


def test_private_key_forms(ed: Ed25519PrivateKey) -> None:
    seed = dist.private_key_text(ed)
    assert dist.public_key_text(dist.private_key(seed)) == dist.public_key_text(ed)
    pub = base64.b64decode(dist.public_key_text(ed))
    old = base64.b64encode(base64.b64decode(seed) + pub).decode()
    assert dist.public_key_text(dist.private_key(old)) == dist.public_key_text(ed)
    with pytest.raises(dist.DistributionError, match="not base64"):
        dist.private_key("@@@")
    with pytest.raises(dist.DistributionError, match="32 or 64"):
        dist.private_key(base64.b64encode(b"short").decode())


def test_key_pair_check(cfg: dict[str, Any], ed: Ed25519PrivateKey) -> None:
    dist.check_key_pair(cfg, ed)
    other = Ed25519PrivateKey.from_private_bytes(bytes(32))
    with pytest.raises(dist.DistributionError, match="does not belong"):
        dist.check_key_pair(cfg, other)


# ---- macOS ----------------------------------------------------------------------------------


def test_info_plist_keys(cfg: dict[str, Any]) -> None:
    keys = dist.sparkle_info_plist(cfg)
    assert keys["SUFeedURL"] == "https://updates.example.com/appcast.xml"
    assert keys["SUPublicEDKey"] == cfg["apple"]["sparkle"]["public_ed_key"]
    assert keys["SURequireSignedFeed"] is True
    assert keys["SUEnableInstallerLauncherService"] is True
    assert dist.sparkle_entitlements("com.example.chat") == {
        "com.apple.security.temporary-exception.mach-lookup.global-name": [
            "com.example.chat-spks",
            "com.example.chat-spki",
        ]
    }


def _item(cfg: dict[str, Any], version: str, data: bytes, ed: Ed25519PrivateKey) -> ET.Element:
    archive = dist.Archive(
        name=f"ExampleChat-{version}.zip",
        length=len(data),
        signature=base64.b64encode(ed.sign(data)).decode(),
    )
    return dist.appcast_item(
        cfg,
        app_name="ExampleChat",
        version=dist.app_version(version),
        archive=archive,
        published=datetime(2026, 10, 26, 12, 0, tzinfo=UTC),
    )


def test_appcast_entry_is_beta_with_phased_rollout(
    cfg: dict[str, Any], ed: Ed25519PrivateKey
) -> None:
    feed = dist.add_items(dist.new_appcast("ExampleChat", cfg), [_item(cfg, "0.1.0", b"n", ed)], ed)
    content = dist.verify_feed(cfg, feed)
    root = ET.fromstring(content)  # noqa: S314
    item = root.find("channel/item")
    assert item is not None
    ns = {"sparkle": dist.SPARKLE_NS}
    assert item.findtext("sparkle:version", namespaces=ns) == "1000"
    assert item.findtext("sparkle:shortVersionString", namespaces=ns) == "0.1.0"
    assert item.findtext("sparkle:channel", namespaces=ns) == "beta"
    assert item.findtext("sparkle:phasedRolloutInterval", namespaces=ns) == "86400"
    assert item.findtext("sparkle:minimumSystemVersion", namespaces=ns) == "12.0"
    assert item.findtext("pubDate") == "Mon, 26 Oct 2026 12:00:00 +0000"
    enclosure = item.find("enclosure")
    assert enclosure is not None
    assert enclosure.get("url") == "https://updates.example.com/macos/ExampleChat-0.1.0.zip"
    assert enclosure.get("length") == "1"
    [entry] = dist.appcast_entries(feed)
    dist.verify_archive(cfg, b"n", entry.signature)
    with pytest.raises(dist.DistributionError, match="archive does not verify"):
        dist.verify_archive(cfg, b"m", entry.signature)


def test_no_phased_rollout_when_the_interval_is_zero(
    cfg: dict[str, Any], ed: Ed25519PrivateKey
) -> None:
    cfg["apple"]["sparkle"]["phased_rollout_interval"] = 0
    item = _item(cfg, "0.1.0", b"n", ed)
    assert item.find(f"{{{dist.SPARKLE_NS}}}phasedRolloutInterval") is None


def test_feed_signature_in_sparkles_format(cfg: dict[str, Any], ed: Ed25519PrivateKey) -> None:
    content = b"<rss><channel/></rss>\n"
    signed = dist.sign_feed(content, ed)
    block = signed[len(content) :].decode()
    assert block.startswith("<!-- sparkle-signatures:\nedSignature: ")
    assert block.endswith(f"\nlength: {len(content)}\n-->\n")
    assert dist.verify_feed(cfg, signed) == content
    assert dist.feed_content(signed) == content


def test_a_changed_feed_does_not_verify(cfg: dict[str, Any], ed: Ed25519PrivateKey) -> None:
    signed = dist.sign_feed(b"<rss><channel/></rss>\n", ed)
    with pytest.raises(dist.DistributionError, match="does not verify"):
        dist.verify_feed(cfg, signed.replace(b"channel", b"channeL", 1))
    with pytest.raises(dist.DistributionError, match="signed length"):
        dist.verify_feed(cfg, b" " + signed)
    with pytest.raises(dist.DistributionError, match="no signature block"):
        dist.verify_feed(cfg, b"<rss/>")
    other = Ed25519PrivateKey.from_private_bytes(bytes(32))
    with pytest.raises(dist.DistributionError, match="does not verify"):
        dist.verify_feed(cfg, dist.sign_feed(b"<rss/>", other))


def test_promotion_removes_only_the_channel(cfg: dict[str, Any], ed: Ed25519PrivateKey) -> None:
    feed = dist.add_items(
        dist.new_appcast("ExampleChat", cfg),
        [_item(cfg, "0.1.0", b"n", ed), _item(cfg, "0.1.1", b"n1", ed)],
        ed,
    )
    before = dist.appcast_entries(feed)
    promoted = dist.promote_appcast(feed, 1001, ed)
    dist.verify_feed(cfg, promoted)
    after = dist.appcast_entries(promoted)
    assert [e.channel for e in after] == ["beta", None]
    assert [(e.build, e.url, e.length, e.signature) for e in after] == [
        (e.build, e.url, e.length, e.signature) for e in before
    ]
    with pytest.raises(dist.DistributionError, match="not in the channel beta"):
        dist.promote_appcast(promoted, 1001, ed)
    with pytest.raises(dist.DistributionError, match="no entry with build 7"):
        dist.promote_appcast(promoted, 7, ed)


def test_merge_adds_the_entries_of_a_release(cfg: dict[str, Any], ed: Ed25519PrivateKey) -> None:
    published = dist.add_items(
        dist.new_appcast("ExampleChat", cfg), [_item(cfg, "0.1.0", b"n", ed)], ed
    )
    published = dist.promote_appcast(published, 1000, ed)
    release = dist.add_items(
        dist.new_appcast("ExampleChat", cfg), [_item(cfg, "0.1.1", b"n1", ed)], ed
    )
    merged = dist.merge_appcast(cfg, published, release, ed)
    dist.verify_feed(cfg, merged)
    assert [(e.build, e.channel) for e in dist.appcast_entries(merged)] == [
        ("1000", None),
        ("1001", "beta"),
    ]
    with pytest.raises(dist.DistributionError, match="already has build 1001"):
        dist.merge_appcast(cfg, merged, release, ed)


def test_not_an_appcast(ed: Ed25519PrivateKey) -> None:
    with pytest.raises(dist.DistributionError, match="no XML"):
        dist.appcast_entries(b"<rss")
    with pytest.raises(dist.DistributionError, match="no rss/channel"):
        dist.add_items(b"<feed/>", [], ed)
    with pytest.raises(dist.DistributionError, match="no enclosure"):
        dist.appcast_entries(b"<rss><channel><item/></channel></rss>")


# ---- Windows --------------------------------------------------------------------------------


def test_toast_clsid_matches_the_app() -> None:
    # LocalNotificationPresenter.guidFor('com.example.chat') of the app gives this value (run
    # once with flutter test when this vector was written); both follow D-M4-30-2.
    assert (
        dist.toast_activator_clsid("com.example.chat") == "{cfdd6c5c-92e7-5a82-b1f2-f206d6c7b5f5}"
    )
    assert dist.toast_activator_clsid("org.acme.chat") != dist.toast_activator_clsid(
        "com.example.chat"
    )


def test_appx_manifest(cfg: dict[str, Any]) -> None:
    text = dist.appx_manifest(
        cfg,
        version=dist.app_version("0.1.1"),
        app_name="ExampleChat",
        url_scheme="chat-example",
        executable="client.exe",
    )
    root = ET.fromstring(text)  # noqa: S314
    ns = {
        "m": "http://schemas.microsoft.com/appx/manifest/foundation/windows10",
        "uap": "http://schemas.microsoft.com/appx/manifest/uap/windows10",
        "desktop": "http://schemas.microsoft.com/appx/manifest/desktop/windows10",
        "com": "http://schemas.microsoft.com/appx/manifest/com/windows10",
    }
    identity = root.find("m:Identity", ns)
    assert identity is not None
    assert identity.attrib == {
        "Name": "com.example.chat",
        "Publisher": "CN=Example Publisher",
        "Version": "0.1.1.0",
        "ProcessorArchitecture": "x64",
    }
    clsid = dist.toast_activator_clsid("com.example.chat").strip("{}")
    toast = root.find(".//desktop:ToastNotificationActivation", ns)
    com = root.find(".//com:Class", ns)
    assert toast is not None
    assert toast.get("ToastActivatorCLSID") == clsid
    assert com is not None
    assert com.get("Id") == clsid
    protocol = root.find(".//uap:Protocol", ns)
    assert protocol is not None
    assert protocol.get("Name") == "chat-example"
    family = root.find("m:Dependencies/m:TargetDeviceFamily", ns)
    assert family is not None
    assert family.get("MinVersion") == "10.0.19041.0"
    app = root.find("m:Applications/m:Application", ns)
    assert app is not None
    assert app.get("Executable") == "client.exe"


def test_publisher_with_markup_is_escaped(cfg: dict[str, Any]) -> None:
    cfg["windows"]["publisher"] = 'CN=A "B" & <C>'
    text = dist.appinstaller(cfg, version=dist.app_version("0.1.0"), channel="beta")
    assert dist.appinstaller_target(text).publisher == 'CN=A "B" & <C>'


def test_appinstaller_files_and_promotion(cfg: dict[str, Any]) -> None:
    beta = dist.appinstaller(cfg, version=dist.app_version("0.1.1"), channel="beta")
    target = dist.appinstaller_target(beta)
    assert target == dist.AppInstallerTarget(
        uri="https://updates.example.com/windows/beta.appinstaller",
        version="0.1.1.0",
        package_name="com.example.chat",
        publisher="CN=Example Publisher",
        msix_uri="https://updates.example.com/windows/com.example.chat_0.1.1.0_x64.msix",
    )
    assert 'HoursBetweenUpdateChecks="24"' in beta
    stable = dist.promote_appinstaller(cfg, beta)
    promoted = dist.appinstaller_target(stable)
    assert promoted.uri == "https://updates.example.com/windows/stable.appinstaller"
    assert (promoted.version, promoted.msix_uri) == (target.version, target.msix_uri)
    with pytest.raises(dist.DistributionError, match="not the beta file"):
        dist.promote_appinstaller(cfg, stable)
    other = copy.deepcopy(cfg)
    other["windows"]["package_name"] = "Other.App"
    with pytest.raises(dist.DistributionError, match="another package"):
        dist.promote_appinstaller(
            cfg, dist.appinstaller(other, version=dist.app_version("0.1.1"), channel="beta")
        )
    other = copy.deepcopy(cfg)
    other["windows"]["publisher"] = "CN=Other"
    with pytest.raises(dist.DistributionError, match="another publisher"):
        dist.promote_appinstaller(
            cfg, dist.appinstaller(other, version=dist.app_version("0.1.1"), channel="beta")
        )
    with pytest.raises(dist.DistributionError, match="no AppInstaller/MainPackage"):
        dist.appinstaller_target("<AppInstaller/>")


# ---- command line ---------------------------------------------------------------------------


def _write_cfg(tmp_path: Path, cfg: dict[str, Any]) -> Path:
    path = tmp_path / "distribution.yaml"
    path.write_text(yaml.safe_dump(cfg), encoding="utf-8")
    return path


def test_cli_validate(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert dist.main(["validate", "--distribution", str(dist.TEMPLATE)]) == 0
    assert dist.main(["validate", "--distribution", str(dist.TEMPLATE), "--release"]) == 1
    assert "placeholder values of the template" in capsys.readouterr().err
    assert dist.main(["validate", "--distribution", str(tmp_path / "none.yaml")]) == 1


def test_cli_appcast_flow(
    tmp_path: Path, cfg: dict[str, Any], ed: Ed25519PrivateKey, monkeypatch: pytest.MonkeyPatch
) -> None:
    path = _write_cfg(tmp_path, cfg)
    key_file = tmp_path / "ed.key"
    key_file.write_text(dist.private_key_text(ed), encoding="utf-8")
    archives = tmp_path / "archives"
    archives.mkdir()
    (archives / "ExampleChat-0.1.0.zip").write_bytes(b"n")
    (archives / "ExampleChat-0.1.1.zip").write_bytes(b"n1")
    published = tmp_path / "appcast.xml"
    published.write_bytes(
        dist.add_items(dist.new_appcast("ExampleChat", cfg), [_item(cfg, "0.1.0", b"n", ed)], ed)
    )
    release = tmp_path / "release.xml"
    release.write_bytes(
        dist.add_items(dist.new_appcast("ExampleChat", cfg), [_item(cfg, "0.1.1", b"n1", ed)], ed)
    )
    common = ["--distribution", str(path), "--ed-key-file", str(key_file)]
    assert (
        dist.main(["appcast-merge", "--appcast", str(published), "--add", str(release), *common])
        == 0
    )
    import io

    monkeypatch.setattr("sys.stdin", io.StringIO(dist.private_key_text(ed)))
    assert (
        dist.main(
            [
                "promote-appcast",
                "--appcast",
                str(published),
                "--build",
                "1001",
                "--distribution",
                str(path),
                "--ed-key-file",
                "-",
            ]
        )
        == 0
    )
    assert [e.channel for e in dist.appcast_entries(published.read_bytes())] == ["beta", None]
    verify = ["verify-appcast", "--appcast", str(published), "--distribution", str(path)]
    assert dist.main([*verify, "--archives", str(archives)]) == 0
    (archives / "ExampleChat-0.1.1.zip").write_bytes(b"changed")
    assert dist.main([*verify, "--archives", str(archives)]) == 1
    other = tmp_path / "other.key"
    other.write_text(
        dist.private_key_text(Ed25519PrivateKey.from_private_bytes(bytes(32))), "utf-8"
    )
    assert (
        dist.main(
            [
                "promote-appcast",
                "--appcast",
                str(published),
                "--build",
                "1000",
                "--distribution",
                str(path),
                "--ed-key-file",
                str(other),
            ]
        )
        == 1
    )


def test_cli_promote_appinstaller(tmp_path: Path, cfg: dict[str, Any]) -> None:
    path = _write_cfg(tmp_path, cfg)
    beta = tmp_path / "beta.appinstaller"
    beta.write_text(
        dist.appinstaller(cfg, version=dist.app_version("0.1.1"), channel="beta"), "utf-8"
    )
    stable = tmp_path / "stable.appinstaller"
    args = [
        "promote-appinstaller",
        "--beta",
        str(beta),
        "--stable",
        str(stable),
        "--distribution",
        str(path),
    ]
    assert dist.main(args) == 0
    assert dist.appinstaller_target(stable.read_text("utf-8")).msix_uri.endswith(
        "_0.1.1.0_x64.msix"
    )


def test_merge_refuses_an_unsigned_or_stable_release(
    cfg: dict[str, Any], ed: Ed25519PrivateKey
) -> None:
    published = dist.add_items(dist.new_appcast("ExampleChat", cfg), [], ed)
    release = dist.add_items(
        dist.new_appcast("ExampleChat", cfg), [_item(cfg, "0.1.1", b"n1", ed)], ed
    )
    with pytest.raises(dist.DistributionError, match="no signature block"):
        dist.merge_appcast(cfg, published, dist.feed_content(release), ed)
    stable = dist.promote_appcast(release, 1001, ed)
    with pytest.raises(dist.DistributionError, match="build 1001 of the new appcast"):
        dist.merge_appcast(cfg, published, stable, ed)
