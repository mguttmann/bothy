# SPDX-License-Identifier: Apache-2.0
"""End-to-end tests of `tokens.py check` (the `make tokens` gate) on a copy of the real design tree."""

import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import tokens

HERE = Path(__file__).resolve().parents[1]
ROOT = HERE.parents[1]


def _sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class GateTreeMixin:
    """Copies tools/design-tokens, design/ and docs/design/ to a temporary root; `_check` runs the gate there."""

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory(prefix="design-tokens-gate-")
        self.root = Path(self._tmp.name)
        ignore = shutil.ignore_patterns("__pycache__", "*.pyc")
        shutil.copytree(HERE, self.root / "tools" / "design-tokens", ignore=ignore)
        shutil.copytree(ROOT / "design", self.root / "design", ignore=ignore)
        shutil.copytree(ROOT / "docs" / "design", self.root / "docs" / "design", ignore=ignore)
        package_copy = Path("packages", "design_system", "lib", "src", "generated")
        shutil.copytree(ROOT / package_copy, self.root / package_copy, ignore=ignore)

    def tearDown(self):
        self._tmp.cleanup()

    def _check(self):
        return subprocess.run(
            [sys.executable, str(self.root / "tools" / "design-tokens" / "tokens.py"), "check"],
            capture_output=True,
            text=True,
            check=False,
        )

    def _edit_json(self, relative, change):
        path = self.root / relative
        data = json.loads(path.read_text(encoding="utf-8"))
        change(data)
        path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


class CheckGateTest(GateTreeMixin, unittest.TestCase):
    """Each test copies tools/design-tokens, design/ and docs/design/ and runs the gate there."""

    def test_repository_state_passes(self):
        proc = self._check()
        self.assertEqual(proc.returncode, 0, proc.stderr)

    def test_changed_token_without_rebuild_fails(self):
        def bump(data):
            data["space"]["16"]["$value"]["value"] = 17

        self._edit_json("design/tokens/space.tokens.json", bump)
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn("stale: design/build/tokens.css", proc.stderr)

    def test_changed_series_without_rebuild_fails(self):
        def darker_series(data):
            data["color"]["viz"]["series"]["1"]["$value"] = "{palette.loch.700}"

        self._edit_json("design/tokens/color.light.tokens.json", darker_series)
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn("stale: design/build/tokens.css", proc.stderr)

    def test_series_breaking_the_cvd_rule_fails(self):
        def twin_series(data):
            series = data["color"]["viz"]["series"]
            series["2"]["$value"] = {"colorSpace": "srgb", "components": [0x1A / 255, 0x7F / 255, 0xC3 / 255]}
            series["1"]["$value"] = {"colorSpace": "srgb", "components": [0x19 / 255, 0x7F / 255, 0xC3 / 255]}

        self._edit_json("design/tokens/color.light.tokens.json", twin_series)
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn(
            "gate: [light, deuteranopia] color.viz.series.1 and color.viz.series.2: deltaE_ok 0.00", proc.stderr
        )

    def test_contrast_pair_below_aa_fails(self):
        def text_like_canvas(data):
            data["color"]["text"]["primary"]["$value"] = data["color"]["surface"]["canvas"]["$value"]

        self._edit_json("design/tokens/color.light.tokens.json", text_like_canvas)
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn("gate: [light] color.text.primary on color.surface.canvas: 1.00 < 4.5", proc.stderr)

    def test_check_fails_when_accent_output_stale(self):
        dart = self.root / "design" / "build" / "dart" / "design_tokens.g.dart"
        text = dart.read_text(encoding="utf-8")
        head, _, tail = text.partition("  static const DsAccent lightLoch = DsAccent(")
        self.assertTrue(tail, "lightLoch constant missing")
        first = tail.index("Color(0xFF") + len("Color(0xFF")
        changed = tail[:first] + ("000000" if tail[first : first + 6] != "000000" else "FFFFFF") + tail[first + 6 :]
        dart.write_text(head + "  static const DsAccent lightLoch = DsAccent(" + changed, encoding="utf-8")
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn("stale: design/build/dart/design_tokens.g.dart", proc.stderr)

    def test_check_fails_when_package_copy_differs(self):
        copy = self.root / "packages" / "design_system" / "lib" / "src" / "generated" / "design_tokens.g.dart"
        copy.write_text(copy.read_text(encoding="utf-8") + "// drift\n", encoding="utf-8")
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn("stale: packages/design_system/lib/src/generated/design_tokens.g.dart", proc.stderr)
        self.assertNotIn("stale: design/build/dart/design_tokens.g.dart", proc.stderr)

    def test_build_writes_the_package_copy_with_the_same_bytes(self):
        copy = self.root / "packages" / "design_system" / "lib" / "src" / "generated" / "design_tokens.g.dart"
        copy.unlink()
        proc = subprocess.run(
            [sys.executable, str(self.root / "tools" / "design-tokens" / "tokens.py"), "build"],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(proc.returncode, 0, proc.stderr)
        source = self.root / "design" / "build" / "dart" / "design_tokens.g.dart"
        self.assertEqual(copy.read_bytes(), source.read_bytes())

    def test_build_fails_below_aa_for_accent(self):
        # A flat mid-grey scale as an extra accent: no step reaches 4.5:1 against light or dark text.
        def add_flat_scale(data):
            grey = {"colorSpace": "srgb", "components": [0.55, 0.55, 0.55]}
            data["palette"]["fog"] = {step: {"$value": grey} for step in ("50", "300", "500", "700", "900", "950")}

        def add_accent(data):
            data["accent_variants"]["accents"].append("fog")

        self._edit_json("design/tokens/color.palette.tokens.json", add_flat_scale)
        self._edit_json("design/design-rules.json", add_accent)
        before = {p: p.read_bytes() for p in (self.root / "design" / "build").rglob("*") if p.is_file()}
        before |= {p: p.read_bytes() for p in (self.root / "docs" / "design").glob("*.md")}
        proc = subprocess.run(
            [sys.executable, str(self.root / "tools" / "design-tokens" / "tokens.py"), "build"],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(proc.returncode, 1, proc.stderr)
        self.assertIn("accent fog, theme light: no step of fog meets ", proc.stderr)
        self.assertNotIn("[light] ", proc.stderr)
        after = {p: p.read_bytes() for p in before}
        self.assertEqual(after, before)

    def test_changed_vendor_file_fails(self):
        # The incident class: a line-ending rewrite of a vendored text file.
        license_file = self.root / "tools" / "design-tokens" / "vendor" / "phosphor" / "LICENSE"
        license_file.write_bytes(license_file.read_bytes().replace(b"\n", b"\r\n"))
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn("vendor: phosphor/LICENSE: sha256 differs from MANIFEST.json", proc.stderr)


class VendorManifestTest(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory(prefix="design-tokens-vendor-")
        self.vendor = Path(self._tmp.name)
        (self.vendor / "phosphor").mkdir()
        (self.vendor / "phosphor" / "LICENSE").write_bytes(b"MIT\n")
        self._manifest({"phosphor/LICENSE": _sha256(self.vendor / "phosphor" / "LICENSE")})

    def tearDown(self):
        self._tmp.cleanup()

    def _manifest(self, sha256):
        (self.vendor / "MANIFEST.json").write_text(json.dumps({"sha256": sha256}), encoding="utf-8")

    def test_matching_manifest_has_no_problems(self):
        self.assertEqual(tokens.vendor_problems(self.vendor), [])

    def test_unlisted_and_missing_files_are_problems(self):
        (self.vendor / "phosphor" / "extra.svg").write_text("<svg/>\n", encoding="utf-8")
        self._manifest({"phosphor/LICENSE": _sha256(self.vendor / "phosphor" / "LICENSE"), "fonts/x.woff2": "0"})
        self.assertEqual(
            tokens.vendor_problems(self.vendor),
            [
                "phosphor/extra.svg: not listed in MANIFEST.json",
                "fonts/x.woff2: listed in MANIFEST.json but missing",
            ],
        )

    def test_text_is_stored_with_lf_and_binary_unchanged(self):
        self.assertEqual(tokens.as_stored(b"a\r\nb\r\n"), b"a\nb\n")
        self.assertEqual(tokens.as_stored(b"wOF2\0\r\n"), b"wOF2\0\r\n")


if __name__ == "__main__":
    unittest.main()
