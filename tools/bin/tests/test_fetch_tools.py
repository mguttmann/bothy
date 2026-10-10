# SPDX-License-Identifier: Apache-2.0
"""Tests for the pinned binary fetcher (no network: the download boundary is replaced)."""

from __future__ import annotations

import hashlib
import io
import json
import stat
import tarfile
from pathlib import Path

import fetch_tools
import pytest

REPO_LOCK = Path(__file__).resolve().parents[1] / "tools.lock.json"


def _tar_gz(members: dict[str, bytes]) -> bytes:
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tf:
        for name, data in members.items():
            info = tarfile.TarInfo(name)
            info.size = len(data)
            info.mode = 0o755
            tf.addfile(info, io.BytesIO(data))
    return buf.getvalue()


def _lock(asset: str, member: str, payload: bytes, version: str = "1.2.3") -> dict:
    return {
        "schema": 1,
        "tools": {
            "demo": {
                "version": version,
                "repo": "example/demo",
                "tag": "v{version}",
                "license": "MIT",
                "binary": "demo",
                "assets": {
                    "linux-amd64": {
                        "asset": asset,
                        "member": member,
                        "sha256": hashlib.sha256(payload).hexdigest(),
                    }
                },
            }
        },
    }


class FakeNet:
    """Replaces the network boundary: serves registered URLs and records calls."""

    def __init__(self) -> None:
        self.store: dict[str, bytes] = {}
        self.calls: list[str] = []

    def __setitem__(self, url: str, data: bytes) -> None:
        self.store[url] = data

    def download(self, url: str) -> bytes:
        self.calls.append(url)
        return self.store[url]


@pytest.fixture
def downloads(monkeypatch: pytest.MonkeyPatch) -> FakeNet:
    net = FakeNet()
    monkeypatch.setattr(fetch_tools, "download", net.download)
    return net


def test_repository_lock_is_valid() -> None:
    lock = fetch_tools.load_lock(REPO_LOCK)
    assert set(lock["tools"]) >= {"actionlint", "gitleaks", "lychee", "osv-scanner", "trivy"}
    for tool in lock["tools"].values():
        assert set(tool["assets"]) == set(fetch_tools.PLATFORMS)


def test_lock_rejects_missing_sha(tmp_path: Path) -> None:
    lock = _lock("demo.tar.gz", "demo", b"x")
    del lock["tools"]["demo"]["assets"]["linux-amd64"]["sha256"]
    path = tmp_path / "lock.json"
    path.write_text(json.dumps(lock))
    with pytest.raises(fetch_tools.LockError, match="sha256"):
        fetch_tools.load_lock(path)


def test_lock_rejects_member_with_parent_reference(tmp_path: Path) -> None:
    lock = _lock("demo.tar.gz", "../demo", b"x")
    path = tmp_path / "lock.json"
    path.write_text(json.dumps(lock))
    with pytest.raises(fetch_tools.LockError, match="member"):
        fetch_tools.load_lock(path)


@pytest.mark.parametrize(
    ("system", "machine", "expected"),
    [
        ("Darwin", "arm64", "darwin-arm64"),
        ("Darwin", "x86_64", "darwin-amd64"),
        ("Linux", "aarch64", "linux-arm64"),
        ("Linux", "x86_64", "linux-amd64"),
    ],
)
def test_platform_key(system: str, machine: str, expected: str) -> None:
    assert fetch_tools.platform_key(system, machine) == expected


def test_platform_key_rejects_windows() -> None:
    with pytest.raises(fetch_tools.LockError, match="WSL"):
        fetch_tools.platform_key("Windows", "AMD64")


def test_ensure_extracts_member_and_caches(tmp_path: Path, downloads: FakeNet) -> None:
    payload = _tar_gz({"demo-1.2.3/demo": b"#!/bin/sh\necho demo\n", "demo-1.2.3/README": b"r"})
    lock = _lock("demo-{version}.tar.gz", "demo-{version}/demo", payload)
    url = "https://github.com/example/demo/releases/download/v1.2.3/demo-1.2.3.tar.gz"
    downloads[url] = payload

    path = fetch_tools.ensure(lock, "demo", "linux-amd64", tmp_path)

    assert path.read_bytes() == b"#!/bin/sh\necho demo\n"
    assert path.stat().st_mode & stat.S_IXUSR
    assert path == tmp_path / "demo" / "1.2.3" / "demo"
    fetch_tools.ensure(lock, "demo", "linux-amd64", tmp_path)
    assert downloads.calls == [url]  # second call served from the cache


def test_ensure_raw_binary(tmp_path: Path, downloads: FakeNet) -> None:
    payload = b"\x7fELFbinary"
    lock = _lock("demo_linux_amd64", "", payload)
    downloads["https://github.com/example/demo/releases/download/v1.2.3/demo_linux_amd64"] = payload
    path = fetch_tools.ensure(lock, "demo", "linux-amd64", tmp_path)
    assert path.read_bytes() == payload


def test_ensure_rejects_checksum_mismatch(tmp_path: Path, downloads: FakeNet) -> None:
    lock = _lock("demo_linux_amd64", "", b"expected")
    downloads["https://github.com/example/demo/releases/download/v1.2.3/demo_linux_amd64"] = b"evil"
    with pytest.raises(fetch_tools.FetchError, match="sha256 mismatch"):
        fetch_tools.ensure(lock, "demo", "linux-amd64", tmp_path)
    assert not (tmp_path / "demo" / "1.2.3" / "demo").exists()


def test_ensure_rejects_missing_member(tmp_path: Path, downloads: FakeNet) -> None:
    payload = _tar_gz({"other": b"x"})
    lock = _lock("demo.tar.gz", "demo", payload)
    downloads["https://github.com/example/demo/releases/download/v1.2.3/demo.tar.gz"] = payload
    with pytest.raises(fetch_tools.FetchError, match="not in archive"):
        fetch_tools.ensure(lock, "demo", "linux-amd64", tmp_path)


def test_ensure_redownloads_when_cached_binary_was_tampered(
    tmp_path: Path, downloads: FakeNet
) -> None:
    payload = b"\x7fELFgood"
    lock = _lock("demo_linux_amd64", "", payload)
    url = "https://github.com/example/demo/releases/download/v1.2.3/demo_linux_amd64"
    downloads[url] = payload
    path = fetch_tools.ensure(lock, "demo", "linux-amd64", tmp_path)
    path.write_bytes(b"tampered")
    fetch_tools.ensure(lock, "demo", "linux-amd64", tmp_path)
    assert path.read_bytes() == payload
    assert downloads.calls == [url, url]


def test_refresh_updates_version_and_digests(monkeypatch: pytest.MonkeyPatch) -> None:
    lock = _lock("demo-{version}.tar.gz", "demo", b"old")
    release = {
        "tag_name": "v2.0.0",
        "assets": [
            {"name": "demo-2.0.0.tar.gz", "digest": "sha256:" + "a" * 64},
            {"name": "unrelated.txt", "digest": "sha256:" + "b" * 64},
        ],
    }
    seen: list[str] = []

    def fake_release(repo: str, tag: str) -> dict:
        seen.append(f"{repo}@{tag}")
        return release

    monkeypatch.setattr(fetch_tools, "github_release", fake_release)
    fetch_tools.refresh(lock, "demo", "2.0.0")
    entry = lock["tools"]["demo"]
    assert entry["version"] == "2.0.0"
    assert entry["assets"]["linux-amd64"]["sha256"] == "a" * 64
    assert seen == ["example/demo@v2.0.0"]


def test_refresh_fails_when_asset_missing(monkeypatch: pytest.MonkeyPatch) -> None:
    lock = _lock("demo-{version}.tar.gz", "demo", b"old")
    monkeypatch.setattr(
        fetch_tools, "github_release", lambda repo, tag: {"tag_name": tag, "assets": []}
    )
    with pytest.raises(fetch_tools.FetchError, match=r"demo-2\.0\.0\.tar\.gz"):
        fetch_tools.refresh(lock, "demo", "2.0.0")


def test_cli_path_prints_binary(
    tmp_path: Path, downloads: FakeNet, capsys: pytest.CaptureFixture[str]
) -> None:
    payload = b"\x7fELF"
    lock = _lock("demo_linux_amd64", "", payload)
    lock_path = tmp_path / "lock.json"
    lock_path.write_text(json.dumps(lock))
    downloads["https://github.com/example/demo/releases/download/v1.2.3/demo_linux_amd64"] = payload
    rc = fetch_tools.main(
        [
            "--lock",
            str(lock_path),
            "--cache",
            str(tmp_path / "c"),
            "--platform",
            "linux-amd64",
            "path",
            "demo",
        ]
    )
    assert rc == 0
    assert capsys.readouterr().out.strip() == str(tmp_path / "c" / "demo" / "1.2.3" / "demo")


def test_cli_unknown_tool(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    lock_path = tmp_path / "lock.json"
    lock_path.write_text(json.dumps(_lock("a", "", b"x")))
    rc = fetch_tools.main(["--lock", str(lock_path), "--platform", "linux-amd64", "path", "nope"])
    assert rc == 2
    assert "unknown tool" in capsys.readouterr().err


def test_download_refuses_plain_http() -> None:
    with pytest.raises(fetch_tools.FetchError, match="non-https"):
        fetch_tools.download("http://example.com/tool.tar.gz")


def _set_schema(lock: dict) -> None:
    lock["schema"] = 2


def _drop_repo(lock: dict) -> None:
    del lock["tools"]["demo"]["repo"]


def _add_windows(lock: dict) -> None:
    lock["tools"]["demo"]["assets"]["windows-amd64"] = {}


def _drop_asset(lock: dict) -> None:
    del lock["tools"]["demo"]["assets"]["linux-amd64"]["asset"]


@pytest.mark.parametrize(
    ("mutate", "message"),
    [
        (_set_schema, "schema 1"),
        (_drop_repo, "missing 'repo'"),
        (_add_windows, "unknown platform"),
        (_drop_asset, "missing 'asset'"),
    ],
)
def test_lock_validation_errors(tmp_path: Path, mutate, message: str) -> None:
    lock = _lock("demo.tar.gz", "demo", b"x")
    mutate(lock)
    path = tmp_path / "lock.json"
    path.write_text(json.dumps(lock))
    with pytest.raises(fetch_tools.LockError, match=message):
        fetch_tools.load_lock(path)


def test_cli_list_prints_versions_and_licenses(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    lock_path = tmp_path / "lock.json"
    lock_path.write_text(json.dumps(_lock("a", "", b"x")))
    assert fetch_tools.main(["--lock", str(lock_path), "list"]) == 0
    assert capsys.readouterr().out.strip() == "demo\t1.2.3\tMIT"


def test_cli_fetch_all(tmp_path: Path, downloads: FakeNet) -> None:
    payload = b"\x7fELF"
    lock_path = tmp_path / "lock.json"
    lock_path.write_text(json.dumps(_lock("demo_linux_amd64", "", payload)))
    downloads["https://github.com/example/demo/releases/download/v1.2.3/demo_linux_amd64"] = payload
    args = ["--lock", str(lock_path), "--cache", str(tmp_path / "c"), "--platform", "linux-amd64"]
    assert fetch_tools.main([*args, "fetch"]) == 0
    assert (tmp_path / "c" / "demo" / "1.2.3" / "demo").is_file()


def test_cli_refresh_rewrites_lock(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    lock_path = tmp_path / "lock.json"
    lock_path.write_text(json.dumps(_lock("demo-{version}.tar.gz", "demo", b"old")))
    release = {"assets": [{"name": "demo-3.0.0.tar.gz", "digest": "sha256:" + "c" * 64}]}
    monkeypatch.setattr(fetch_tools, "github_release", lambda repo, tag: release)
    assert fetch_tools.main(["--lock", str(lock_path), "refresh", "demo", "3.0.0"]) == 0
    written = json.loads(lock_path.read_text())["tools"]["demo"]
    assert written["version"] == "3.0.0"
    assert written["assets"]["linux-amd64"]["sha256"] == "c" * 64


def test_cli_reports_fetch_errors(
    tmp_path: Path, downloads: FakeNet, capsys: pytest.CaptureFixture[str]
) -> None:
    lock_path = tmp_path / "lock.json"
    lock_path.write_text(json.dumps(_lock("demo_linux_amd64", "", b"expected")))
    downloads["https://github.com/example/demo/releases/download/v1.2.3/demo_linux_amd64"] = b"bad"
    args = ["--lock", str(lock_path), "--cache", str(tmp_path / "c"), "--platform", "linux-amd64"]
    assert fetch_tools.main([*args, "path", "demo"]) == 1
    assert "sha256 mismatch" in capsys.readouterr().err
