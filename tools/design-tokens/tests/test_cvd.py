# SPDX-License-Identifier: Apache-2.0
"""Tests of the colour vision deficiency simulation (tokenkit/cvd.py) and its gate rows (rules.check_cvd)."""

import json
import unittest
from pathlib import Path

import tokens
from tests.test_gate import GateTreeMixin
from tokenkit import cvd, dtcg, model, rules

ROOT = Path(__file__).resolve().parents[3]

# Machado, Oliveira and Fernandes 2009, matrices for severity 1.0 (supplementary table of the paper).
# Applied to a primary, a matrix gives its column; these are the published numbers, listed here a
# second time so that a transposed or mistyped matrix in cvd.py fails.
REFERENCE_COLUMNS = {
    "protanopia": [(0.152286, 0.114503, -0.003882), (1.052583, 0.786281, -0.048116), (-0.204868, 0.099216, 1.051998)],
    "deuteranopia": [(0.367322, 0.280085, -0.011820), (0.860646, 0.672501, 0.042940), (-0.227968, 0.047413, 0.968881)],
    "tritanopia": [(1.255528, -0.078411, 0.004733), (-0.076749, 0.930809, 0.691367), (-0.178779, 0.147602, 0.303900)],
}


def _tokens(mapping):
    doc = {}
    for path, hex_value in mapping.items():
        node = doc
        parts = path.split(".")
        for part in parts[:-1]:
            node = node.setdefault(part, {})
        comps = [int(hex_value.lstrip("#")[i : i + 2], 16) / 255 for i in (0, 2, 4)]
        node[parts[-1]] = {"$type": "color", "$value": {"colorSpace": "srgb", "components": comps}}
    return dtcg.resolve(dtcg.flatten(doc))


def _rules(entries, minimum=0.05):
    return {"cvd_min_delta_e_ok": {"value": minimum, "source": "test", "reason": "test"}, "cvd_pairs": entries}


SERIES = [f"s.{i}" for i in range(1, 9)]
SERIES_HEX = ["#197fc3", "#ae6416", "#136f69", "#c53a88", "#8d7716", "#063e1a", "#d43538", "#748486"]


class MatrixTest(unittest.TestCase):
    def test_matrices_match_reference(self):
        primaries = ((1.0, 0.0, 0.0), (0.0, 1.0, 0.0), (0.0, 0.0, 1.0))
        for deficiency, columns in REFERENCE_COLUMNS.items():
            for primary, expected in zip(primaries, columns, strict=True):
                with self.subTest(deficiency=deficiency, primary=primary):
                    got = cvd.apply_matrix(primary, deficiency)
                    for g, e in zip(got, expected, strict=True):
                        self.assertAlmostEqual(g, e, places=6)
            with self.subTest(deficiency=deficiency, color="white"):
                for v in cvd.apply_matrix((1.0, 1.0, 1.0), deficiency):
                    self.assertAlmostEqual(v, 1.0, places=5)
                self.assertEqual(cvd.simulate_hex("#ffffff", deficiency), "#ffffff")
        # Sample colours worked by hand from the table: sRGB red decodes to linear (1, 0, 0), so
        # protanopia gives the first column, clipped and encoded; #ff8000 decodes to (1, 0.2159, 0).
        for hex_value, deficiency, expected in (
            ("#ff0000", "protanopia", (0.426608, 0.372654, 0.0)),
            ("#ff8000", "deuteranopia", (0.769305, 0.683791, 0.0)),
        ):
            with self.subTest(sample=hex_value):
                got = cvd.simulate(hex_value, deficiency)
                for g, e in zip(got, expected, strict=True):
                    self.assertAlmostEqual(g, e, delta=1e-3)

    def test_simulation_clips_and_roundtrips(self):
        for deficiency in cvd.DEFICIENCIES:
            for hex_value in ("#ff0000", "#00ff00", "#0000ff", "#ffff00", "#00ffff", "#ff00ff"):
                with self.subTest(deficiency=deficiency, color=hex_value):
                    for v in cvd.simulate(hex_value, deficiency):
                        self.assertGreaterEqual(v, 0.0)
                        self.assertLessEqual(v, 1.0)
            for grey in ("#000000", "#808080", "#3a3a3a", "#ffffff"):
                with self.subTest(deficiency=deficiency, grey=grey):
                    self.assertEqual(cvd.simulate_hex(grey, deficiency), grey)

    def test_distance_is_zero_for_equal_colours(self):
        self.assertEqual(cvd.distance("#197fc3", "#197fc3", "tritanopia"), 0.0)


class CvdRulesTest(unittest.TestCase):
    def setUp(self):
        self.themes = {
            "light": _tokens(dict(zip(SERIES, SERIES_HEX, strict=True))),
            "dark": _tokens(dict(zip(SERIES, SERIES_HEX, strict=True))),
        }

    def test_adjacent_pairs_checked_with_wrap(self):
        entry = {"usage": "series", "deficiencies": list(cvd.DEFICIENCIES), "sequence": SERIES, "wrap": True}
        rows = rules.check_cvd(self.themes, _rules([entry]))
        self.assertEqual(len(rows), 8 * 3 * 2)
        for theme in ("light", "dark"):
            for deficiency in cvd.DEFICIENCIES:
                pairs = [(r.a, r.b) for r in rows if r.theme == theme and r.deficiency == deficiency]
                self.assertEqual(pairs, [(SERIES[i], SERIES[(i + 1) % 8]) for i in range(8)])
        self.assertIn(("s.8", "s.1"), {(r.a, r.b) for r in rows})

    def test_explicit_pairs(self):
        entry = {"usage": "light", "deficiencies": ["tritanopia"], "pairs": [["s.1", "s.2"], ["s.1", "s.3"]]}
        rows = rules.check_cvd({"light": self.themes["light"]}, _rules([entry]))
        self.assertEqual([(r.a, r.b) for r in rows], [("s.1", "s.2"), ("s.1", "s.3")])

    def test_failing_pair_fails_gate(self):
        themes = {"light": _tokens({"a": "#197fc3", "b": "#1a7fc3"})}
        entry = {"usage": "twins", "deficiencies": ["deuteranopia"], "pairs": [["a", "b"]]}
        rows = rules.check_cvd(themes, _rules([entry]))
        self.assertEqual(len(rows), 1)
        self.assertFalse(rows[0].passed)
        self.assertLess(rows[0].distance, 0.05)
        self.assertEqual(rows[0].required, 0.05)

    def test_unknown_role_or_deficiency_is_rule_error(self):
        for entry in (
            {"usage": "x", "deficiencies": ["deuteranopia"], "pairs": [["s.1", "nope"]]},
            {"usage": "x", "deficiencies": ["achromatopsia"], "pairs": [["s.1", "s.2"]]},
            {"usage": "x", "deficiencies": [], "pairs": [["s.1", "s.2"]]},
            {"usage": "x", "deficiencies": ["deuteranopia"]},
            {"usage": "x", "deficiencies": ["deuteranopia"], "sequence": ["s.1"]},
        ):
            with self.subTest(entry=entry), self.assertRaises(rules.RuleError):
                rules.check_cvd(self.themes, _rules([entry]))
        with self.assertRaises(rules.RuleError):
            rules.check_cvd(self.themes, {"cvd_pairs": []})


class RealDesignTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.model = model.build(tokens.RESOLVER)
        cls.rule_data = json.loads(tokens.RULES.read_text(encoding="utf-8"))
        cls.themes = cls.model.full["theme"]

    def test_real_tokens_pass(self):
        rows = rules.check_cvd(self.themes, self.rule_data)
        failed = [r for r in rows if not r.passed]
        self.assertEqual(failed, [])
        self.assertEqual({r.theme for r in rows}, {"light", "dark"})
        self.assertEqual({r.deficiency for r in rows}, set(cvd.DEFICIENCIES))
        contrast = rules.check_contrast(self.themes, self.rule_data["contrast_pairs"])
        self.assertEqual([r for r in contrast if not r.passed], [])
        minimum = self.rule_data["cvd_min_delta_e_ok"]
        self.assertTrue(minimum["source"] and minimum["reason"])

    def test_real_pairs_cover_series_wrap_and_traffic_light(self):
        rows = rules.check_cvd(self.themes, self.rule_data)
        pairs = {(r.a, r.b) for r in rows}
        series = [f"color.viz.series.{i}" for i in range(1, 9)]
        for i in range(8):
            self.assertIn((series[i], series[(i + 1) % 8]), pairs)
        for pair in (
            ("color.success.fg", "color.warning.fg"),
            ("color.warning.fg", "color.danger.fg"),
            ("color.success.fg", "color.danger.fg"),
        ):
            self.assertIn(pair, pairs)
        self.assertEqual(len(rows), 11 * 3 * 2)

    def test_accents_do_not_touch_checked_roles(self):
        # The check runs once per theme because no accent swaps a checked role (D-M6-10-4); a
        # future accent role among them must add per-accent rows first.
        checked = {role for r in rules.check_cvd(self.themes, self.rule_data) for role in (r.a, r.b)}
        self.assertEqual(checked & set(self.rule_data["accent_variants"]["roles"]), set())

    def test_contrast_md_lists_cvd_rows(self):
        outputs, problems = tokens.generate()
        self.assertEqual(problems, [])
        text = outputs[tokens.DOCS / "contrast.md"]
        self.assertIn("## Color vision deficiency", text)
        self.assertIn("cvd_min_delta_e_ok", text)
        rows = rules.check_cvd(self.themes, self.rule_data)
        for r in rows:
            with self.subTest(row=(r.theme, r.deficiency, r.a, r.b)):
                self.assertEqual(text.count(f"| {r.theme} | {r.deficiency} | `{r.a}` | `{r.b}` |"), 1)
        resolved = json.loads(outputs[tokens.BUILD / "tokens.resolved.json"])
        self.assertEqual(len(resolved["cvd"]["rows"]), len(rows))


class CvdGateTest(GateTreeMixin, unittest.TestCase):
    def test_failing_pair_fails_check_with_message(self):
        def twins(data):
            data["color"]["viz"]["series"]["4"]["$value"] = data["color"]["viz"]["series"]["3"]["$value"]

        self._edit_json("design/tokens/color.dark.tokens.json", twins)
        proc = self._check()
        self.assertEqual(proc.returncode, 1)
        self.assertIn(
            "gate: [dark, deuteranopia] color.viz.series.3 and color.viz.series.4: deltaE_ok 0.000 < ", proc.stderr
        )


if __name__ == "__main__":
    unittest.main()
