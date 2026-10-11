# SPDX-License-Identifier: Apache-2.0
"""Tests of the preview page sections and of `tokens.py previews --view`."""

import re
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import yaml

import tokens
from tokenkit import emit_preview, model

HERE = Path(__file__).resolve().parents[1]


class PreviewSectionsTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.html = emit_preview.render(model.build(tokens.RESOLVER), tokens.VENDOR, "test")

    def test_every_preview_view_has_a_section(self):
        for view in {v for v, _, _, _ in tokens.PREVIEWS}:
            with self.subTest(view=view):
                self.assertIn(f'<section class="pv" id="{view}"', self.html)

    def test_terminal_section_uses_terminal_roles_only_for_its_colours(self):
        section = self.html.split('<section class="pv" id="terminal"', 1)[1].split("</section>", 1)[0]
        for role in ("bg", "fg", "cursor", "selection-bg", "ansi-red", "ansi-bright-black"):
            with self.subTest(role=role):
                self.assertIn(f"--ds-color-terminal-{role}", self.html)
        self.assertIsNone(re.search(r"#[0-9a-fA-F]{6}\b", section), "no literal colour in the terminal section")

    def test_run_states_show_words_not_enum_values(self):
        components = (HERE.parents[1] / "docs" / "design" / "components.md").read_text(encoding="utf-8")
        for state in ("queued", "running", "waiting_input", "completed", "failed", "cancelled"):
            with self.subTest(state=state):
                self.assertNotIn(f'aria-label="{state}"', self.html)
                self.assertNotIn(f"</svg>{state}</span>", self.html)
                row = re.search(rf"^\| `{state}` \|.*\| \"([^\"]+)\"", components, re.M)
                self.assertIsNotNone(row, f"components.md has no word for {state}")
                self.assertIn(f'aria-label="{row.group(1)}"', self.html)

    def test_viz_section_uses_series_and_axis_roles(self):
        for role in ("series-1", "series-8", "axis", "grid", "area-fill"):
            with self.subTest(role=role):
                self.assertIn(f"--ds-color-viz-{role}", self.html)

    def test_delivery_target_comes_from_the_slo_file(self):
        """The preview never invents a target: objective and threshold of the Sloth file (K2-15)."""
        text = emit_preview.SLO_FILE.read_text(encoding="utf-8")
        slo = next(e for e in yaml.safe_load(text)["slos"] if e["name"] == "delivery-latency")
        threshold = re.search(r'le="([0-9.]+)"', slo["sli"]["events"]["error_query"]).group(1)
        self.assertEqual(emit_preview.delivery_slo(), (float(slo["objective"]), float(threshold)))
        section = self.html.split('<section class="pv" id="viz"', 1)[1].split("</section>", 1)[0]
        objective = f"{float(slo['objective']):.10g}".replace(".", ",")
        self.assertIn(f"Ziel {objective} %", section)
        self.assertIn(f"Anteil innerhalb {float(threshold) * 1000:.10g} ms", section)
        self.assertIn('stroke-dasharray="6 4"', section)
        self.assertNotIn("250 ms", section)

    def test_delivery_slo_follows_the_file_and_fails_without_it(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "sync.yaml"
            path.write_text(
                emit_preview.SLO_FILE.read_text(encoding="utf-8")
                .replace("objective: 99.5", "objective: 99.9")
                .replace('le="0.5"', 'le="0.25"'),
                encoding="utf-8",
            )
            self.assertEqual(emit_preview.delivery_slo(path), (99.9, 0.25))
            path.write_text('slos:\n  - name: "reconnect"\n    objective: 99.0\n', encoding="utf-8")
            with self.assertRaises(ValueError):
                emit_preview.delivery_slo(path)


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
        code, files = self._run("--view", "terminal")
        self.assertEqual(code, 0)
        self.assertEqual(files, ["terminal-light.png", "terminal-dark.png"])

    def test_view_can_repeat(self):
        _, files = self._run("--view", "terminal", "--view", "viz")
        self.assertEqual(files, ["terminal-light.png", "terminal-dark.png", "viz-light.png", "viz-dark.png"])

    def test_unknown_view_is_rejected(self):
        with self.assertRaises(SystemExit) as ctx, mock.patch("sys.stderr"):
            tokens.main(["previews", "--view", "nope"])
        self.assertEqual(ctx.exception.code, 2)


if __name__ == "__main__":
    unittest.main()
