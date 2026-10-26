# SPDX-License-Identifier: Apache-2.0
"""Release ZIP of the Grafana panel plugin (tools/ci/plugin_release.py, brief M7-26).

`pack` turns the webpack output `dist/` into `bothy-live-panel-<version>.zip` with the one top
folder Grafana expects, reproducibly; `read_zip` is the check the product bundle runs before it
unpacks the ZIP into the Compose tree. Every refusal names the problem and writes nothing.
"""

from __future__ import annotations

import io
import json
import os
import stat
import subprocess
import sys
import zipfile
from pathlib import Path

import plugin_release
import pytest

REPO = Path(__file__).resolve().parents[3]
TOOL = REPO / "tools" / "ci" / "plugin_release.py"
VERSION = "1.2.3"


def make_dist(root: Path, version: str = VERSION, **plugin: object) -> Path:
    """A minimal webpack output: plugin.json, module.js, a logo and a font in a subfolder."""
    dist = root / "dist"
    (dist / "img").mkdir(parents=True)
    (dist / "fonts").mkdir()
    doc: dict[str, object] = {
        "type": "panel",
        "name": "Live views",
        "id": "bothy-live-panel",
        "info": {"version": version, "updated": "2026-10-26"},
    }
    doc.update(plugin)
    (dist / "plugin.json").write_text(json.dumps(doc), encoding="utf-8")
    (dist / "module.js").write_text("define([],function(){})\n", encoding="utf-8")
    (dist / "img" / "logo.svg").write_text("<svg/>\n", encoding="utf-8")
    (dist / "fonts" / "a.woff2").write_bytes(b"\0\1\2")
    return dist


def zip_bytes(entries: dict[str, bytes], *, symlink: str | None = None) -> bytes:
    """A ZIP with the given members; `symlink` marks one member as a symbolic link."""
    out = io.BytesIO()
    with zipfile.ZipFile(out, "w") as archive:
        for name, data in entries.items():
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            mode = stat.S_IFLNK | 0o777 if name == symlink else stat.S_IFREG | 0o644
            info.external_attr = mode << 16
            archive.writestr(info, data)
    return out.getvalue()


def plugin_json(version: str = VERSION, **extra: object) -> bytes:
    doc: dict[str, object] = {
        "type": "panel",
        "id": "bothy-live-panel",
        "info": {"version": version},
    }
    doc.update(extra)
    return json.dumps(doc).encode()


# ---- pack ----------------------------------------------------------------------------------


def test_pack_writes_one_top_folder_with_every_file(tmp_path: Path) -> None:
    dist = make_dist(tmp_path)
    written = plugin_release.pack(dist, VERSION, tmp_path / "out")
    assert written == tmp_path / "out" / "bothy-live-panel-1.2.3.zip"
    with zipfile.ZipFile(written) as archive:
        names = archive.namelist()
        assert names == sorted(names)
        assert names == [
            "bothy-live-panel/fonts/a.woff2",
            "bothy-live-panel/img/logo.svg",
            "bothy-live-panel/module.js",
            "bothy-live-panel/plugin.json",
        ]
        for info in archive.infolist():
            assert info.date_time == (1980, 1, 1, 0, 0, 0)
            assert (info.external_attr >> 16) == stat.S_IFREG | 0o644


def test_pack_is_reproducible(tmp_path: Path) -> None:
    dist = make_dist(tmp_path)
    first = plugin_release.pack(dist, VERSION, tmp_path / "a").read_bytes()
    os.utime(dist / "module.js", (1, 1))
    assert plugin_release.pack(dist, VERSION, tmp_path / "b").read_bytes() == first


@pytest.mark.parametrize(
    ("plugin", "named"),
    [
        ({"id": "other-panel"}, "other-panel"),
        ({"type": "app"}, "app"),
        ({"info": {"version": "0.0.0"}}, "0.0.0"),
    ],
)
def test_pack_refuses_another_plugin(tmp_path: Path, plugin: dict[str, object], named: str) -> None:
    dist = make_dist(tmp_path, **plugin)
    with pytest.raises(plugin_release.PluginReleaseError, match=named):
        plugin_release.pack(dist, VERSION, tmp_path / "out")
    assert not (tmp_path / "out").exists()


def test_pack_refuses_a_missing_module(tmp_path: Path) -> None:
    dist = make_dist(tmp_path)
    (dist / "module.js").unlink()
    with pytest.raises(plugin_release.PluginReleaseError, match=r"module\.js"):
        plugin_release.pack(dist, VERSION, tmp_path / "out")


def test_pack_refuses_a_symbolic_link(tmp_path: Path) -> None:
    dist = make_dist(tmp_path)
    (dist / "link.js").symlink_to(dist / "module.js")
    with pytest.raises(plugin_release.PluginReleaseError, match=r"link\.js"):
        plugin_release.pack(dist, VERSION, tmp_path / "out")


def test_pack_refuses_a_version_that_is_not_semver(tmp_path: Path) -> None:
    dist = make_dist(tmp_path, version="v1.2")
    with pytest.raises(plugin_release.PluginReleaseError, match=r"v1\.2"):
        plugin_release.pack(dist, "v1.2", tmp_path / "out")


# ---- read_zip --------------------------------------------------------------------------------


def test_read_zip_returns_the_files_below_the_top_folder(tmp_path: Path) -> None:
    data = plugin_release.pack(make_dist(tmp_path), VERSION, tmp_path / "out").read_bytes()
    files = plugin_release.read_zip(data, VERSION)
    assert sorted(files) == ["fonts/a.woff2", "img/logo.svg", "module.js", "plugin.json"]
    assert files["fonts/a.woff2"] == b"\0\1\2"


@pytest.mark.parametrize(
    ("entries", "named"),
    [
        ({"other/plugin.json": plugin_json(), "other/module.js": b""}, "other/plugin.json"),
        ({"bothy-live-panel/../x": b"", "bothy-live-panel/plugin.json": plugin_json()}, ".."),
        ({"/bothy-live-panel/plugin.json": plugin_json()}, "/bothy-live-panel"),
        (
            {
                "bothy-live-panel/plugin.json": plugin_json("9.9.9"),
                "bothy-live-panel/module.js": b"",
            },
            "9.9.9",
        ),
        (
            {
                "bothy-live-panel/plugin.json": plugin_json(id="evil-panel"),
                "bothy-live-panel/module.js": b"",
            },
            "evil-panel",
        ),
        ({"bothy-live-panel/plugin.json": plugin_json()}, r"module\.js"),
        ({"bothy-live-panel/module.js": b""}, r"plugin\.json"),
        ({"bothy-live-panel\\module.js": b"", "bothy-live-panel/plugin.json": b"{}"}, "\\\\"),
    ],
)
def test_read_zip_refuses(entries: dict[str, bytes], named: str) -> None:
    with pytest.raises(plugin_release.PluginReleaseError, match=named):
        plugin_release.read_zip(zip_bytes(entries), VERSION)


def test_read_zip_refuses_a_symbolic_link() -> None:
    entries = {
        "bothy-live-panel/plugin.json": plugin_json(),
        "bothy-live-panel/module.js": b"/etc/passwd",
    }
    with pytest.raises(plugin_release.PluginReleaseError, match="symbolic link"):
        plugin_release.read_zip(zip_bytes(entries, symlink="bothy-live-panel/module.js"), VERSION)


def test_read_zip_refuses_what_is_not_a_zip() -> None:
    with pytest.raises(plugin_release.PluginReleaseError, match="not a ZIP"):
        plugin_release.read_zip(b"not a zip", VERSION)


# ---- unpack ----------------------------------------------------------------------------------


def test_unpack_replaces_the_folder_but_keeps_its_gitignore(tmp_path: Path) -> None:
    data = plugin_release.pack(make_dist(tmp_path), VERSION, tmp_path / "out").read_bytes()
    dest = tmp_path / "plugins" / "bothy-live-panel"
    (dest / "img").mkdir(parents=True)
    (dest / ".gitignore").write_text("*\n", encoding="utf-8")
    (dest / "stale.js").write_text("old\n", encoding="utf-8")
    (dest / "img" / "old.svg").write_text("old\n", encoding="utf-8")
    plugin_release.unpack(data, VERSION, dest)
    found = sorted(p.relative_to(dest).as_posix() for p in dest.rglob("*") if p.is_file())
    assert found == [".gitignore", "fonts/a.woff2", "img/logo.svg", "module.js", "plugin.json"]


def test_unpack_writes_nothing_for_a_bad_zip(tmp_path: Path) -> None:
    dest = tmp_path / "bothy-live-panel"
    dest.mkdir()
    (dest / "module.js").write_text("kept\n", encoding="utf-8")
    with pytest.raises(plugin_release.PluginReleaseError):
        plugin_release.unpack(b"not a zip", VERSION, dest)
    assert (dest / "module.js").read_text(encoding="utf-8") == "kept\n"


# ---- command line ----------------------------------------------------------------------------


def test_cli_packs_and_names_the_file(tmp_path: Path) -> None:
    dist = make_dist(tmp_path)
    result = subprocess.run(
        [sys.executable, str(TOOL), "pack", "--dist", str(dist), "--version", VERSION, "--out",
         str(tmp_path / "out")],
        capture_output=True, text=True, check=False,
    )  # fmt: skip
    assert result.returncode == 0, result.stderr
    assert "bothy-live-panel-1.2.3.zip" in result.stdout


def test_cli_refusal_exits_1_and_names_the_value(tmp_path: Path) -> None:
    dist = make_dist(tmp_path, id="other-panel")
    result = subprocess.run(
        [sys.executable, str(TOOL), "pack", "--dist", str(dist), "--version", VERSION, "--out",
         str(tmp_path / "out")],
        capture_output=True, text=True, check=False,
    )  # fmt: skip
    assert result.returncode == 1
    assert "other-panel" in result.stderr
