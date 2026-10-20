# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/bin/tofu_providers.py: the offline provider mirror of the OpenTofu gates."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

import pytest
import tofu_providers

ZIP = b"PK\x03\x04 fake provider archive"
ZIP_SHA = hashlib.sha256(ZIP).hexdigest()

LOCK = f"""# This file is maintained automatically by "tofu init".
# Manual edits may be lost in future updates.

provider "registry.opentofu.org/hashicorp/aws" {{
  version     = "6.69.1"
  constraints = "6.69.1"
  hashes = [
    "h1:AAAA=",
    "zh:{"0" * 64}",
    "zh:{ZIP_SHA}",
  ]
}}

provider "registry.opentofu.org/hetznercloud/hcloud" {{
  version = "1.70.0"
  hashes = [
    "zh:{ZIP_SHA}",
  ]
}}
"""


def write_lock(directory: Path, text: str = LOCK) -> Path:
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / ".terraform.lock.hcl"
    path.write_text(text, encoding="utf-8")
    return path


class FakeNet:
    """Stands in for the registry and the release downloads."""

    def __init__(self, payload: bytes = ZIP, shasum: str = ZIP_SHA) -> None:
        self.payload = payload
        self.shasum = shasum
        self.urls: list[str] = []

    def __call__(self, url: str) -> bytes:
        self.urls.append(url)
        if "/v1/providers/" in url:
            parts = url.split("/")
            ns, name, version, os_name, arch = parts[5], parts[6], parts[7], parts[9], parts[10]
            doc: dict[str, Any] = {
                "filename": f"terraform-provider-{name}_{version}_{os_name}_{arch}.zip",
                "download_url": f"https://github.com/{ns}/terraform-provider-{name}/{version}.zip",
                "shasum": self.shasum,
            }
            return json.dumps(doc).encode()
        return self.payload


def test_parse_lock_reads_version_and_zip_hashes(tmp_path: Path) -> None:
    providers = tofu_providers.parse_lock(write_lock(tmp_path))
    assert [(p.source, p.version) for p in providers] == [
        ("hashicorp/aws", "6.69.1"),
        ("hetznercloud/hcloud", "1.70.0"),
    ]
    assert providers[0].zip_hashes == {"0" * 64, ZIP_SHA}


def test_mirror_downloads_in_the_packed_layout(tmp_path: Path) -> None:
    net = FakeNet()
    mirror = tofu_providers.ensure_mirror(
        [write_lock(tmp_path / "root")], tmp_path / "cache", "darwin-arm64", fetch=net
    )
    zip_path = (
        mirror
        / "registry.opentofu.org/hashicorp/aws/terraform-provider-aws_6.69.1_darwin_arm64.zip"
    )
    assert zip_path.read_bytes() == ZIP
    assert (
        net.urls[0]
        == "https://registry.opentofu.org/v1/providers/hashicorp/aws/6.69.1/download/darwin/arm64"
    )
    # A second call finds both archives and downloads nothing.
    again = FakeNet()
    tofu_providers.ensure_mirror(
        [tmp_path / "root/.terraform.lock.hcl"], tmp_path / "cache", "darwin-arm64", fetch=again
    )
    assert again.urls == []


def test_registry_hash_not_in_the_lock_file_is_refused(tmp_path: Path) -> None:
    net = FakeNet(shasum="f" * 64)
    with pytest.raises(tofu_providers.ProviderError, match="not in the lock file"):
        tofu_providers.ensure_mirror(
            [write_lock(tmp_path)], tmp_path / "cache", "linux-amd64", fetch=net
        )


def test_download_with_another_hash_is_refused(tmp_path: Path) -> None:
    net = FakeNet(payload=b"tampered")
    with pytest.raises(tofu_providers.ProviderError, match="sha256"):
        tofu_providers.ensure_mirror(
            [write_lock(tmp_path)], tmp_path / "cache", "linux-amd64", fetch=net
        )
    assert not list((tmp_path / "cache").rglob("*.zip"))


def test_tampered_cache_is_downloaded_again(tmp_path: Path) -> None:
    lock = write_lock(tmp_path / "root")
    cache = tmp_path / "cache"
    tofu_providers.ensure_mirror([lock], cache, "linux-amd64", fetch=FakeNet())
    [cached, _] = sorted(cache.rglob("*.zip"))
    cached.write_bytes(b"changed")
    net = FakeNet()
    tofu_providers.ensure_mirror([lock], cache, "linux-amd64", fetch=net)
    assert cached.read_bytes() == ZIP
    assert len(net.urls) == 2


def test_two_lock_files_with_different_versions_are_refused(tmp_path: Path) -> None:
    other = LOCK.replace('version     = "6.69.1"', 'version     = "6.70.0"')
    locks = [write_lock(tmp_path / "a"), write_lock(tmp_path / "b", other)]
    with pytest.raises(tofu_providers.ProviderError, match="hashicorp/aws"):
        tofu_providers.ensure_mirror(locks, tmp_path / "cache", "linux-amd64", fetch=FakeNet())


def test_lock_file_without_providers_is_refused(tmp_path: Path) -> None:
    with pytest.raises(tofu_providers.ProviderError, match="no provider"):
        tofu_providers.parse_lock(write_lock(tmp_path, "# empty\n"))


def test_cli_config_names_the_mirror_and_the_cache(tmp_path: Path) -> None:
    text = tofu_providers.cli_config(tmp_path / "mirror", tmp_path / "plugins")
    assert f'path = "{tmp_path / "mirror"}"' in text
    assert f'plugin_cache_dir = "{tmp_path / "plugins"}"' in text
    assert "direct" not in text


def test_default_locks_are_the_test_roots() -> None:
    names = sorted(p.parent.name for p in tofu_providers.default_locks())
    assert names == ["cloud", "validate"]


def test_main_prints_the_mirror(
    tmp_path: Path, capsys: pytest.CaptureFixture[str], monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(tofu_providers, "_fetch", FakeNet())
    lock = write_lock(tmp_path / "root")
    assert (
        tofu_providers.main(["mirror", "--lock", str(lock), "--cache", str(tmp_path / "cache")])
        == 0
    )
    assert capsys.readouterr().out.strip() == str(tmp_path / "cache")


def test_main_reports_an_error(
    tmp_path: Path, capsys: pytest.CaptureFixture[str], monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(tofu_providers, "_fetch", FakeNet(shasum="f" * 64))
    lock = write_lock(tmp_path / "root")
    assert (
        tofu_providers.main(["mirror", "--lock", str(lock), "--cache", str(tmp_path / "cache")])
        == 1
    )
    assert "tofu_providers: " in capsys.readouterr().err
