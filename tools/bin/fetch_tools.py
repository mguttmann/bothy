# SPDX-License-Identifier: Apache-2.0
"""Fetch pinned release binaries for the repository gates.

The lock file tools/bin/tools.lock.json names version, release asset and sha256 per
platform. A binary is only installed after its archive matched the pinned sha256;
a cached binary is re-checked against a stamp of its own hash on every use.

An asset is a raw binary (empty `member`), a gzip- or xz-compressed tar archive or a zip
archive (recognised by its signature; Loki publishes only zip assets). In both archive
forms the `member` must exist and be a regular file; an archive that cannot be read is
an error (exit 1) like a wrong checksum, not a traceback.

Usage:
  fetch_tools.py path <tool>        ensure the tool is present, print its path
  fetch_tools.py fetch [<tool>...]  ensure tools are present (default: all)
  fetch_tools.py refresh <tool> <version>
                                    re-pin a tool from the GitHub release digests
  fetch_tools.py list               print tool, version, license
"""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
import platform
import re
import stat
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
import zlib
from pathlib import Path
from typing import Any

PLATFORMS = ("darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64")
REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_LOCK = REPO_ROOT / "tools" / "bin" / "tools.lock.json"
DEFAULT_CACHE = REPO_ROOT / ".cache" / "tools"
_SHA256 = re.compile(r"^[0-9a-f]{64}$")
_ZIP_SIGNATURE = b"PK\x03\x04"
_TIMEOUT_S = 120

Lock = dict[str, Any]


class LockError(Exception):
    """The lock file is malformed or the platform is unsupported."""


class FetchError(Exception):
    """A download did not match its pin or could not be unpacked."""


def platform_key(system: str | None = None, machine: str | None = None) -> str:
    system = (system or platform.system()).lower()
    machine = (machine or platform.machine()).lower()
    arch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(machine)
    if system not in ("darwin", "linux") or arch is None:
        raise LockError(
            f"unsupported platform {system}/{machine}; on Windows run the gates inside WSL"
        )
    return f"{system}-{arch}"


def load_lock(path: Path) -> Lock:
    data: Lock = json.loads(path.read_text(encoding="utf-8"))
    if data.get("schema") != 1 or not isinstance(data.get("tools"), dict):
        raise LockError(f"{path}: expected schema 1 with a 'tools' object")
    for name, tool in data["tools"].items():
        for key in ("version", "repo", "tag", "license", "binary", "assets"):
            if key not in tool:
                raise LockError(f"{name}: missing '{key}'")
        for plat, asset in tool["assets"].items():
            if plat not in PLATFORMS:
                raise LockError(f"{name}: unknown platform '{plat}'")
            if not _SHA256.match(str(asset.get("sha256", ""))):
                raise LockError(f"{name}/{plat}: missing or malformed sha256")
            member = str(asset.get("member", ""))
            if member.startswith("/") or ".." in Path(member).parts:
                raise LockError(f"{name}/{plat}: member must be a relative path inside the archive")
            if "asset" not in asset:
                raise LockError(f"{name}/{plat}: missing 'asset'")
    return data


def _render(template: str, version: str) -> str:
    return template.replace("{version}", version)


def asset_url(tool: dict[str, Any], plat: str) -> str:
    version = tool["version"]
    tag = _render(tool["tag"], version)
    asset = _render(tool["assets"][plat]["asset"], version)
    return f"https://github.com/{tool['repo']}/releases/download/{tag}/{asset}"


def download(url: str) -> bytes:
    if not url.startswith("https://"):
        raise FetchError(f"refusing non-https download: {url}")
    with urllib.request.urlopen(url, timeout=_TIMEOUT_S) as resp:  # noqa: S310  # nosec B310
        data: bytes = resp.read()
    return data


# What tarfile, zipfile, gzip and zlib raise for an archive that cannot be read (empty, cut
# off, corrupt or of another format); zipfile raises RuntimeError for an encrypted member and
# its subclass NotImplementedError for a compression method it lacks (Deflate64).
_UNREADABLE_ARCHIVE = (
    tarfile.TarError,
    zipfile.BadZipFile,
    EOFError,
    OSError,
    zlib.error,
    RuntimeError,
)


def _extract(data: bytes, member: str, name: str) -> bytes:
    """Return the bytes of the binary inside a downloaded release asset.

    Args:
        data: the downloaded asset, already checked against its pinned sha256.
        member: path of the binary inside the archive; empty for a raw binary asset.
        name: tool name for error messages.

    Returns:
        `data` itself for a raw binary, else the content of `member`. A zip archive
        (signature `PK\\x03\\x04`, Loki publishes nothing else) is read with zipfile, any
        other archive as gzip- or xz-compressed tar. Nothing is written to disk by member name,
        so no archive path can reach outside the cache; `load_lock` rejects absolute
        members and `..` anyway.

    Raises:
        FetchError: the member is missing or is not a regular file (directory, link), or
            the archive cannot be read (empty, cut off, corrupt or of another format).
    """
    if not member:
        return data
    try:
        if data.startswith(_ZIP_SIGNATURE):
            return _extract_zip(data, member, name)
        return _extract_tar(data, member, name)
    except _UNREADABLE_ARCHIVE as exc:
        raise FetchError(f"{name}: cannot read the archive ({type(exc).__name__}: {exc})") from exc


def _extract_tar(data: bytes, member: str, name: str) -> bytes:
    """Read `member` from a gzip- or xz-compressed tar archive (see `_extract`)."""
    # "r:*" lets tarfile detect the compression; ShellCheck publishes only .tar.xz assets.
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:*") as tf:
        try:
            info = tf.getmember(member)
        except KeyError as exc:
            raise FetchError(f"{name}: '{member}' not in archive") from exc
        if not info.isfile():
            raise FetchError(f"{name}: '{member}' is not a regular file")
        fh = tf.extractfile(info)
        if fh is None:
            raise FetchError(f"{name}: cannot read '{member}'")
        return fh.read()


def _extract_zip(data: bytes, member: str, name: str) -> bytes:
    """Read `member` from a zip archive with the same rules as for tar (see `_extract`)."""
    with zipfile.ZipFile(io.BytesIO(data)) as zf:
        try:
            info = zf.getinfo(member)
        except KeyError as exc:
            raise FetchError(f"{name}: '{member}' not in archive") from exc
        # The Unix mode sits in the high 16 bits. Zip tools of other systems leave its file
        # type bits empty; then only the trailing slash of a directory entry tells the type.
        file_type = stat.S_IFMT(info.external_attr >> 16)
        if info.is_dir() or (file_type and file_type != stat.S_IFREG):
            raise FetchError(f"{name}: '{member}' is not a regular file")
        return zf.read(info)


def _sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def ensure(lock: Lock, name: str, plat: str, cache: Path) -> Path:
    """Return the path of the pinned binary, downloading and verifying it if needed."""
    tool = lock["tools"][name]
    target_dir = cache / name / tool["version"]
    target: Path = target_dir / tool["binary"]
    stamp = target_dir / ".sha256"
    cached_ok = (
        target.is_file()
        and stamp.is_file()
        and _sha256(target.read_bytes()) == stamp.read_text(encoding="utf-8").strip()
    )
    if cached_ok:
        return target
    asset = tool["assets"][plat]
    data = download(asset_url(tool, plat))
    got = _sha256(data)
    if got != asset["sha256"]:
        raise FetchError(
            f"{name}: sha256 mismatch for {plat} (pinned {asset['sha256']}, got {got})"
        )
    binary = _extract(data, _render(asset.get("member", ""), tool["version"]), name)
    target_dir.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=target_dir)
    with os.fdopen(fd, "wb") as fh:
        fh.write(binary)
    Path(tmp).chmod(0o755)
    Path(tmp).replace(target)
    stamp.write_text(_sha256(binary) + "\n", encoding="utf-8")
    return target


def github_release(repo: str, tag: str) -> dict[str, Any]:
    url = f"https://api.github.com/repos/{repo}/releases/tags/{tag}"
    headers = {"Accept": "application/vnd.github+json"}
    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req, timeout=_TIMEOUT_S) as resp:  # noqa: S310  # nosec B310
        data: dict[str, Any] = json.load(resp)
    return data


def refresh(lock: Lock, name: str, version: str) -> None:
    """Re-pin `name` to `version` using the digests GitHub reports for the release assets."""
    tool = lock["tools"][name]
    release = github_release(tool["repo"], _render(tool["tag"], version))
    digests = {a["name"]: str(a.get("digest", "")) for a in release.get("assets", [])}
    new_assets = {}
    for plat, asset in tool["assets"].items():
        file_name = _render(asset["asset"], version)
        digest = digests.get(file_name, "")
        if not digest.startswith("sha256:"):
            raise FetchError(f"{name}: release {version} has no sha256 digest for {file_name}")
        new_assets[plat] = {**asset, "sha256": digest.removeprefix("sha256:")}
    tool["assets"] = new_assets
    tool["version"] = version


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--lock", type=Path, default=DEFAULT_LOCK)
    parser.add_argument("--cache", type=Path, default=DEFAULT_CACHE)
    parser.add_argument("--platform", default=None, help="override platform key (tests)")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_path = sub.add_parser("path")
    p_path.add_argument("tool")
    p_fetch = sub.add_parser("fetch")
    p_fetch.add_argument("tools", nargs="*")
    p_refresh = sub.add_parser("refresh")
    p_refresh.add_argument("tool")
    p_refresh.add_argument("version")
    sub.add_parser("list")
    args = parser.parse_args(argv)

    try:
        lock = load_lock(args.lock)
        names = (
            [args.tool]
            if args.cmd in ("path", "refresh")
            else (getattr(args, "tools", None) or sorted(lock["tools"]))
        )
        unknown = [n for n in names if n not in lock["tools"]]
        if unknown:
            print(f"unknown tool: {', '.join(unknown)}", file=sys.stderr)
            return 2
        if args.cmd == "list":
            for n in names:
                t = lock["tools"][n]
                print(f"{n}\t{t['version']}\t{t['license']}")
            return 0
        if args.cmd == "refresh":
            refresh(lock, args.tool, args.version)
            args.lock.write_text(json.dumps(lock, indent=2) + "\n", encoding="utf-8")
            print(f"{args.tool} pinned to {args.version}; review the diff before committing")
            return 0
        plat = args.platform or platform_key()
        for n in names:
            path = ensure(lock, n, plat, args.cache)
            if args.cmd == "path":
                print(path)
        return 0
    except (LockError, FetchError) as exc:
        print(f"fetch_tools: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
