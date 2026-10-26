# SPDX-License-Identifier: Apache-2.0
"""Release ZIP of the Grafana panel plugin `bothy-live-panel` (component `grafana-bothy-live`).

`make -C plugins/grafana-bothy-live release-build VERSION=<v> OUT=<dir>` builds the plugin with
webpack and then runs `pack`: the files of `dist/` go into `bothy-live-panel-<v>.zip` below the
one top folder `bothy-live-panel/`, the layout of a Grafana plugin ZIP, so the ZIP unpacks
straight into Grafana's plugin directory. The ZIP is reproducible: sorted members, one fixed
time and mode, no directory entries, so the same `dist/` always gives the same bytes.

`read_zip` is the check before anything unpacks the ZIP (the product bundle carries the plugin,
docs/spec/25-visualization.md section 8.3, docs/decisions.d/M7-26.md): every member lies below
the top folder, no absolute path, no `..`, no backslash, no symbolic link, and `plugin.json`
names exactly this plugin, type `panel` and the expected version, next to a `module.js`.

The plugin stays unsigned (ruling T11): the ZIP carries no `MANIFEST.txt`; the release signs the
ZIP itself with cosign like every release file (tools/ci/release_chain.py).

`unpack` puts a checked ZIP into a folder, for `make -C plugins/grafana-bothy-live stack-install`
(the plugin for the Compose stack of a checkout).

Usage:
  plugin_release.py pack --dist <webpack output> --version <v> --out <dir>
  plugin_release.py unpack --zip <file> --version <v> --dest <dir>
Exit codes: 0 written, 1 refused (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import io
import json
import re
import stat
import sys
import tempfile
import zipfile
from pathlib import Path, PurePosixPath

PLUGIN_ID = "bothy-live-panel"
PLUGIN_TYPE = "panel"
# One fixed time for every member: the earliest a ZIP can hold, so a rebuild of the same files
# yields the same bytes.
ZIP_TIME = (1980, 1, 1, 0, 0, 0)
FILE_MODE = stat.S_IFREG | 0o644
_SEMVER = re.compile(r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$")


class PluginReleaseError(Exception):
    """The plugin ZIP cannot be built or is not the expected plugin; nothing was written."""


def zip_name(version: str) -> str:
    """Return the file name of the release ZIP, for example `bothy-live-panel-1.0.0.zip`."""
    return f"{PLUGIN_ID}-{version}.zip"


def check_manifest(data: bytes, version: str, where: str) -> None:
    """Check that a `plugin.json` names this plugin, type panel and `version`.

    Args:
        data: the bytes of plugin.json.
        version: the expected `info.version`.
        where: the path named in a finding.

    Raises:
        PluginReleaseError: not JSON, or another ID, type or version (the value is named).
    """
    try:
        doc = json.loads(data)
    except ValueError as exc:
        raise PluginReleaseError(f"{where} is not JSON: {exc}") from exc
    if not isinstance(doc, dict):
        raise PluginReleaseError(f"{where} is not a JSON object")
    raw_info = doc.get("info")
    info: dict[str, object] = raw_info if isinstance(raw_info, dict) else {}
    found = {
        "id": (doc.get("id"), PLUGIN_ID),
        "type": (doc.get("type"), PLUGIN_TYPE),
        "info.version": (info.get("version"), version),
    }
    for key, (got, want) in found.items():
        if got != want:
            raise PluginReleaseError(f"{where}: {key} is {got!r}, expected {want!r}")


def _check_version(version: str) -> None:
    if not _SEMVER.match(version):
        raise PluginReleaseError(f"version '{version}' is not SemVer without 'v'")


def pack(dist: Path, version: str, out: Path) -> Path:
    """Write `out/bothy-live-panel-<version>.zip` from the webpack output `dist`.

    Args:
        dist: the plugin build (`npm run build` writes `plugins/grafana-bothy-live/dist`).
        version: the component version; `dist/plugin.json` must carry it.
        out: the output directory (created if missing).

    Returns:
        The path of the written ZIP.

    Raises:
        PluginReleaseError: the version is not SemVer, `plugin.json` names another plugin, type
            or version, `module.js` is missing, or `dist` holds a symbolic link or anything but
            files and folders; nothing is written then.

    Example:
        `pack(Path("dist"), "1.0.0", Path("out"))` writes `out/bothy-live-panel-1.0.0.zip`.
    """
    _check_version(version)
    manifest = dist / "plugin.json"
    if not manifest.is_file():
        raise PluginReleaseError(f"{manifest} is missing (run npm run build first)")
    check_manifest(manifest.read_bytes(), version, str(manifest))
    if not (dist / "module.js").is_file():
        raise PluginReleaseError(f"{dist / 'module.js'} is missing")
    files = []
    for path in sorted(dist.rglob("*")):
        rel = path.relative_to(dist).as_posix()
        if path.is_symlink():
            raise PluginReleaseError(f"{rel} in {dist} is a symbolic link")
        if path.is_file():
            files.append((rel, path))
        elif not path.is_dir():
            raise PluginReleaseError(f"{rel} in {dist} is not a regular file")
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for rel, path in sorted(files):
            info = zipfile.ZipInfo(f"{PLUGIN_ID}/{rel}", date_time=ZIP_TIME)
            info.external_attr = FILE_MODE << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, path.read_bytes())
    out.mkdir(parents=True, exist_ok=True)
    final = out / zip_name(version)
    with tempfile.NamedTemporaryFile(dir=out, prefix=".plugin-zip-", delete=False) as tmp:
        tmp.write(buffer.getvalue())
    Path(tmp.name).replace(final)
    final.chmod(0o644)
    return final


def read_zip(data: bytes, version: str) -> dict[str, bytes]:
    """Check a plugin ZIP and return its files below the top folder.

    Args:
        data: the bytes of the ZIP.
        version: the version `plugin.json` must carry.

    Returns:
        Path relative to `bothy-live-panel/` (POSIX form) to content, for example
        `{"module.js": b"...", "plugin.json": b"..."}`.

    Raises:
        PluginReleaseError: not a ZIP; a member outside `bothy-live-panel/`, absolute, with `..`
            or a backslash, a symbolic link or a duplicate; `plugin.json` or `module.js`
            missing; `plugin.json` names another plugin, type or version.
    """
    try:
        archive = zipfile.ZipFile(io.BytesIO(data))
    except zipfile.BadZipFile as exc:
        raise PluginReleaseError(f"the plugin file is not a ZIP: {exc}") from exc
    files: dict[str, bytes] = {}
    with archive:
        for info in archive.infolist():
            name = info.filename
            if "\\" in name:
                raise PluginReleaseError(f"member {name!r} contains a backslash (\\)")
            pure = PurePosixPath(name)
            if pure.is_absolute() or ".." in pure.parts:
                raise PluginReleaseError(f"member {name} is absolute or contains ..")
            if not pure.parts or pure.parts[0] != PLUGIN_ID or len(pure.parts) < 2:
                raise PluginReleaseError(f"member {name} is not below {PLUGIN_ID}/")
            if info.is_dir():
                continue
            if stat.S_ISLNK(info.external_attr >> 16):
                raise PluginReleaseError(f"member {name} is a symbolic link")
            rel = PurePosixPath(*pure.parts[1:]).as_posix()
            if rel in files:
                raise PluginReleaseError(f"member {name} appears twice")
            files[rel] = archive.read(info)
    for required in ("plugin.json", "module.js"):
        if required not in files:
            raise PluginReleaseError(f"the plugin ZIP has no {PLUGIN_ID}/{required}")
    check_manifest(files["plugin.json"], version, f"{PLUGIN_ID}/plugin.json")
    return files


def unpack(data: bytes, version: str, dest: Path) -> list[Path]:
    """Check a plugin ZIP with `read_zip` and unpack its files into `dest`.

    Everything in `dest` but its `.gitignore` is removed first, so no file of an older build
    stays behind (the folder of a checkout is `infra/observability/grafana/plugins/
    bothy-live-panel`, which Compose mounts into Grafana).

    Returns:
        The written files.

    Raises:
        PluginReleaseError: as `read_zip`; `dest` is unchanged then.
    """
    files = read_zip(data, version)
    dest.mkdir(parents=True, exist_ok=True)
    for path in sorted(dest.rglob("*"), reverse=True):
        if path.relative_to(dest).as_posix() == ".gitignore":
            continue
        if path.is_dir() and not path.is_symlink():
            path.rmdir()
        else:
            path.unlink()
    written = []
    for rel, content in sorted(files.items()):
        target = dest / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(content)
        target.chmod(0o644)
        written.append(target)
    return written


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Release ZIP of the Grafana panel plugin.")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_pack = sub.add_parser("pack")
    p_pack.add_argument("--dist", type=Path, required=True)
    p_pack.add_argument("--version", required=True)
    p_pack.add_argument("--out", type=Path, required=True)
    p_unpack = sub.add_parser("unpack")
    p_unpack.add_argument("--zip", type=Path, required=True)
    p_unpack.add_argument("--version", required=True)
    p_unpack.add_argument("--dest", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        if args.cmd == "pack":
            print(f"  release-build: wrote {pack(args.dist, args.version, args.out)}")
        else:
            unpack(args.zip.read_bytes(), args.version, args.dest)
            print(f"  unpacked {args.zip.name} into {args.dest}")
    except PluginReleaseError as exc:
        print(f"plugin release: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
