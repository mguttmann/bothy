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


class CheckGateTest(unittest.TestCase):
    """Each test copies tools/design-tokens, design/ and docs/design/ and runs the gate there."""

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory(prefix="design-tokens-gate-")
        self.root = Path(self._tmp.name)
        ignore = shutil.ignore_patterns("__pycache__", "*.pyc")
        shutil.copytree(HERE, self.root / "tools" / "design-tokens", ignore=ignore)
        shutil.copytree(ROOT / "design", self.root / "design", ignore=ignore)
        shutil.copytree(ROOT / "docs" / "design", self.root / "docs" / "design", ignore=ignore)
        # The preview reads the delivery SLO (emit_preview.SLO_FILE).
        slo = self.root / "infra" / "observability" / "slo"
        slo.mkdir(parents=True)
        shutil.copy2(ROOT / "infra" / "observability" / "slo" / "sync.yaml", slo / "sync.yaml")

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

    def test_contrast_pair_below_aa_fails(self):
        def text_like_canvas(data):
            data["color"]["text"]["primary"]["$value"] = data["color"]["surface"]["canvas"]["$value"]

        self._edit_json("design/tokens/color.light.tokens.json", text_like_canvas)
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn("gate: [light] color.text.primary on color.surface.canvas: 1.00 < 4.5", proc.stderr)

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
