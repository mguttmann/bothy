# SPDX-License-Identifier: Apache-2.0
"""Tests of the preview page sections and of `tokens.py previews --view`."""

import unittest
from pathlib import Path
from unittest import mock

import tokens
from tokenkit import emit_preview, model


class PreviewSectionsTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.html = emit_preview.render(model.build(tokens.RESOLVER), tokens.VENDOR, "test")

    def test_every_preview_view_has_a_section(self):
        for view in {v for v, _, _, _ in tokens.PREVIEWS}:
            with self.subTest(view=view):
                self.assertIn(f'<section class="pv" id="{view}"', self.html)

    def test_viz_section_uses_series_and_axis_roles(self):
        section = self.html.split('<section class="pv" id="viz"', 1)[1].split("</section>", 1)[0]
        for k in range(1, 9):
            with self.subTest(series=k):
                self.assertIn(f"--ds-color-viz-series-{k}", section)
        for role in ("axis", "grid", "area-fill"):
            with self.subTest(role=role):
                self.assertIn(f"--ds-color-viz-{role}", self.html)


class PreviewsCommandTest(unittest.TestCase):
    def _run(self, *argv):
        calls = []
        with (
            mock.patch.object(tokens.shutil, "which", return_value="/usr/bin/npx"),
            mock.patch.object(tokens.subprocess, "run", side_effect=lambda cmd, check: calls.append(cmd)),
            mock.patch("builtins.print"),
        ):
            code = tokens.main(["previews", *argv])
        return code, [Path(cmd[-1]).name for cmd in calls]

    def test_without_view_renders_every_preview(self):
        code, files = self._run()
        self.assertEqual(code, 0)
        self.assertEqual(files, [f"{stem}.png" for _, _, _, stem in tokens.PREVIEWS])

    def test_view_renders_only_that_view(self):
        code, files = self._run("--view", "viz")
        self.assertEqual(code, 0)
        self.assertEqual(files, ["viz-light.png", "viz-dark.png"])

    def test_view_can_repeat(self):
        _, files = self._run("--view", "mobile", "--view", "viz")
        self.assertEqual(files, ["mobile-light.png", "mobile-dark.png", "viz-light.png", "viz-dark.png"])

    def test_unknown_view_is_rejected(self):
        with self.assertRaises(SystemExit) as ctx, mock.patch("sys.stderr"):
            tokens.main(["previews", "--view", "nope"])
        self.assertEqual(ctx.exception.code, 2)


if __name__ == "__main__":
    unittest.main()
