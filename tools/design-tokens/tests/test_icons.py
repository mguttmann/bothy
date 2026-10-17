# SPDX-License-Identifier: Apache-2.0
"""Tests of the icon assets (`tokenkit/icon_assets.py`, `tokens.py icons`).

The unit tests use a fake compiler (one line per SVG) so they check the tool's own logic; the
determinism test and the gate test run the real `vector_graphics_compiler` of the workspace lock.
"""

import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from tests.test_gate import GateTreeMixin
from tokenkit import emit_dart, icon_assets, model

HERE = Path(__file__).resolve().parents[1]
ROOT = HERE.parents[1]
PACKAGE = ROOT / "packages" / "design_system"
PHOSPHOR = HERE / "vendor" / "phosphor"
HEADER = "generated for a test"


def _icons() -> dict:
    return emit_dart.icon_extension(model.build(ROOT / "design" / "tokens" / "theme.resolver.json"))


def _fake_compile(svgs: dict[str, bytes]) -> dict[str, bytes]:
    return {key: b"VEC " + hashlib.sha256(data).hexdigest().encode() for key, data in svgs.items()}


def _sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


class IconEntriesTest(unittest.TestCase):
    def test_every_mapped_glyph_in_its_weight_and_fill(self):
        icons = _icons()
        keys = {e.key for e in icon_assets.entries(icons)}
        for semantic, glyph in icons["glyphs"].items():
            with self.subTest(semantic=semantic):
                self.assertIn(f"{glyph['weight']}/{glyph['name']}", keys)
                self.assertIn(f"fill/{glyph['name']}", keys)
        for name in icons["space-icons"]:
            self.assertIn(f"regular/{name}", keys)

    def test_vendor_needs_the_fill_svgs(self):
        needed = set(icon_assets.needed_svgs({"glyphs": {"a.b": {"name": "x", "weight": "bold"}}}))
        self.assertEqual(needed, {("x", "bold"), ("x", "fill")})

    def test_asset_paths_stable(self):
        icons = {"glyphs": {"a.one": {"name": "check", "weight": "regular"}}, "space-icons": ["robot"]}
        before = {e.key: e.asset for e in icon_assets.entries(icons)}
        icons["glyphs"]["a.two"] = {"name": "bell", "weight": "bold"}
        after = {e.key: e.asset for e in icon_assets.entries(icons)}
        for key, asset in before.items():
            self.assertEqual(after[key], asset)
        self.assertEqual(after["bold/bell"], "assets/icons/bold/bell.vec")
        self.assertEqual(list(after), sorted(after))


class IconBuildTest(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory(prefix="design-tokens-icons-")
        self.phosphor = Path(self._tmp.name) / "phosphor"
        shutil.copytree(PHOSPHOR, self.phosphor)

    def tearDown(self):
        self._tmp.cleanup()

    def _build(self, icons=None):
        return icon_assets.build(icons or _icons(), self.phosphor, HEADER, _fake_compile, "1.0.0")

    def test_missing_svg_fails_naming_glyph(self):
        (self.phosphor / "circle-notch-bold.svg").unlink()
        with self.assertRaises(icon_assets.IconError) as ctx:
            self._build()
        message = str(ctx.exception)
        self.assertIn("trigger.working", message)
        self.assertIn("'circle-notch'", message)
        self.assertIn("'bold'", message)

    def test_manifest_hashes_match(self):
        out = self._build()
        manifest = json.loads(out[icon_assets.MANIFEST_FILE])
        self.assertEqual(manifest["package"], "@phosphor-icons/core@2.1.1")
        self.assertEqual(manifest["license_sha256"], _sha256(out[icon_assets.LICENSE_FILE]))
        assets = {path for path in out if path.endswith(".vec")}
        self.assertEqual({g["asset"] for g in manifest["glyphs"].values()}, assets)
        for key, entry in manifest["glyphs"].items():
            with self.subTest(key=key):
                self.assertEqual(entry["asset_sha256"], _sha256(out[entry["asset"]]))
                weight, name = key.split("/")
                svg = self.phosphor / icon_assets.svg_file(name, weight)
                self.assertEqual(entry["source_sha256"], _sha256(svg.read_bytes()))

    def test_lookup_names_every_asset_and_semantic_icon(self):
        icons = _icons()
        lookup = self._build(icons)[icon_assets.LOOKUP_FILE].decode()
        for entry in icon_assets.entries(icons):
            self.assertIn(f"'{entry.key}': '{entry.asset}',", lookup)
        for semantic in icons["glyphs"]:
            self.assertIn(f"'{semantic}': DsIcons.", lookup)

    def test_compiler_output_must_cover_every_svg(self):
        with self.assertRaises(icon_assets.IconError):
            icon_assets.build(_icons(), self.phosphor, HEADER, lambda svgs: {}, "1.0.0")


class RealCompilerTest(unittest.TestCase):
    def test_build_is_deterministic(self):
        icons = _icons()
        compile_ = icon_assets.vector_compiler(PACKAGE)
        version = icon_assets.compiler_version(ROOT / "pubspec.lock")
        first = icon_assets.build(icons, PHOSPHOR, HEADER, compile_, version)
        second = icon_assets.build(icons, PHOSPHOR, HEADER, compile_, version)
        self.assertEqual(first, second)
        self.assertTrue(all(first[p] for p in first if p.endswith(".vec")))


class IconGateTest(GateTreeMixin, unittest.TestCase):
    """`tokens.py build` and `check` on a copy of the tree, with the real compiler of the workspace."""

    def _run(self, command):
        return subprocess.run(
            [sys.executable, str(self.root / "tools" / "design-tokens" / "tokens.py"), command],
            capture_output=True,
            text=True,
            check=False,
        )

    def test_check_fails_on_changed_asset(self):
        asset = self.root / "packages" / "design_system" / "assets" / "icons" / "bold" / "circle-notch.vec"
        asset.write_bytes(asset.read_bytes() + b"\0")
        proc = self._run("check")
        self.assertEqual(proc.returncode, 1)
        self.assertIn("stale: packages/design_system/assets/icons/bold/circle-notch.vec", proc.stderr)

    def test_check_fails_on_an_extra_asset(self):
        extra = self.root / "packages" / "design_system" / "assets" / "icons" / "regular" / "extra.vec"
        extra.write_bytes(b"x")
        proc = self._run("check")
        self.assertEqual(proc.returncode, 1)
        self.assertIn("stale: packages/design_system/assets/icons/regular/extra.vec (not generated", proc.stderr)

    def test_missing_svg_fails_build_and_check(self):
        (self.root / "tools" / "design-tokens" / "vendor" / "phosphor" / "x-bold.svg").unlink()
        for command in ("build", "check"):
            with self.subTest(command=command):
                proc = self._run(command)
                self.assertNotEqual(proc.returncode, 0)
                self.assertIn("action.dismiss", proc.stderr)
                self.assertIn("'x' in weight 'bold'", proc.stderr)


if __name__ == "__main__":
    unittest.main()
