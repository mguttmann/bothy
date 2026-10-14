# SPDX-License-Identifier: Apache-2.0
"""Tests for the contrast gate and the palette guard rules."""

import json
import unittest
from pathlib import Path

from tokenkit import accents, dtcg, model, rules

ROOT = Path(__file__).resolve().parents[3]


def _color(hex_value):
    return {"$type": "color", "$value": {"colorSpace": "srgb", "components": _components(hex_value)}}


def _components(hex_value):
    hex_value = hex_value.lstrip("#")
    return [int(hex_value[i : i + 2], 16) / 255 for i in (0, 2, 4)]


def _tokens(mapping):
    doc = {}
    for path, hex_value in mapping.items():
        node = doc
        parts = path.split(".")
        for part in parts[:-1]:
            node = node.setdefault(part, {})
        node[parts[-1]] = _color(hex_value)
    return dtcg.resolve(dtcg.flatten(doc))


class ContrastGateTest(unittest.TestCase):
    def test_text_pair_below_45_fails(self):
        tokens = _tokens({"fg": "#777777", "bg": "#ffffff"})
        rows = rules.check_contrast({"light": tokens}, [{"fg": "fg", "bg": "bg", "kind": "text"}])
        self.assertFalse(rows[0].passed)
        self.assertEqual(rows[0].required, 4.5)

    def test_ui_pair_needs_only_three(self):
        tokens = _tokens({"fg": "#949494", "bg": "#ffffff"})
        rows = rules.check_contrast({"light": tokens}, [{"fg": "fg", "bg": "bg", "kind": "ui"}])
        self.assertTrue(rows[0].passed)
        self.assertEqual(rows[0].required, 3.0)

    def test_every_theme_is_checked(self):
        light = _tokens({"fg": "#000000", "bg": "#ffffff"})
        dark = _tokens({"fg": "#222222", "bg": "#000000"})
        rows = rules.check_contrast({"light": light, "dark": dark}, [{"fg": "fg", "bg": "bg", "kind": "text"}])
        self.assertEqual([r.passed for r in rows], [True, False])

    def test_unknown_token_in_pair_is_an_error(self):
        tokens = _tokens({"fg": "#000000"})
        with self.assertRaises(rules.RuleError):
            rules.check_contrast({"light": tokens}, [{"fg": "fg", "bg": "nope", "kind": "text"}])

    def test_list_of_backgrounds_expands_to_one_row_each(self):
        tokens = _tokens({"fg": "#000000", "a": "#ffffff", "b": "#eeeeee"})
        rows = rules.check_contrast({"light": tokens}, [{"fg": "fg", "bg": ["a", "b"], "kind": "text"}])
        self.assertEqual([r.bg for r in rows], ["a", "b"])

    def test_list_of_foregrounds_expands_too(self):
        tokens = _tokens({"x": "#000000", "y": "#111111", "bg": "#ffffff"})
        rows = rules.check_contrast({"light": tokens}, [{"fg": ["x", "y"], "bg": "bg", "kind": "text"}])
        self.assertEqual([r.fg for r in rows], ["x", "y"])

    def test_unknown_kind_is_an_error(self):
        tokens = _tokens({"fg": "#000000", "bg": "#ffffff"})
        with self.assertRaises(rules.RuleError):
            rules.check_contrast({"light": tokens}, [{"fg": "fg", "bg": "bg", "kind": "huge"}])


class AccentContrastTest(unittest.TestCase):
    def test_every_accent_checked_light_and_dark(self):
        model_ = model.build(ROOT / "design" / "tokens" / "theme.resolver.json")
        data = json.loads((ROOT / "design" / "design-rules.json").read_text(encoding="utf-8"))
        themes = model_.full["theme"]
        variants = accents.derive(themes, data)
        rows = rules.check_accent_contrast(themes, data["contrast_pairs"], variants, "spruce")
        member = data["accent_variants"]
        others = [a for a in member["accents"] if a != member["default"]]
        self.assertEqual(len(others), 6)
        self.assertEqual({(r.theme, r.accent) for r in rows}, {(t, a) for t in ("light", "dark") for a in others})
        for theme in ("light", "dark"):
            for accent in others:
                swapped = {role for role, v in variants[(theme, accent)].items() if v.swapped}
                expected = {
                    (p["fg"], p["bg"])
                    for p in rules.expand_pairs(data["contrast_pairs"])
                    if p["fg"] in swapped or p["bg"] in swapped
                }
                got = {(r.fg, r.bg) for r in rows if r.theme == theme and r.accent == accent}
                self.assertEqual(got, expected, f"{theme} {accent}")
        failed = [r for r in rows if not r.passed]
        self.assertEqual(failed, [])

    def test_accent_rows_use_variant_colours(self):
        light = _tokens({"fg": "#000000", "bg": "#ffffff"})
        variant = accents.Variant(role="fg", scale="x", step="1", hex="#eeeeee", moved_from=None, swapped=True)
        rows = rules.check_accent_contrast(
            {"light": light}, [{"fg": "fg", "bg": "bg", "kind": "text"}], {("light", "x"): {"fg": variant}}, "spruce"
        )
        self.assertEqual([(r.accent, r.fg_hex, r.passed) for r in rows], [("x", "#eeeeee", False)])


class PaletteGuardTest(unittest.TestCase):
    def test_colour_too_close_to_forbidden_reference_is_flagged(self):
        tokens = _tokens({"palette.brand.500": "#5a66f0"})
        guard = {
            "scope": ["palette.brand."],
            "forbidden_colors": [{"name": "ref", "hex": "#5865f2", "min_delta_e_ok": 0.08}],
        }
        problems = rules.check_guards(tokens, [guard])
        self.assertEqual(len(problems), 1)

    def test_hue_window_flags_chromatic_colour_inside_window(self):
        tokens = _tokens({"palette.brand.500": "#c8643c"})
        guard = {
            "scope": ["palette.brand."],
            "forbidden_hue_ranges": [{"name": "terracotta", "from": 25, "to": 80, "min_chroma": 0.04}],
        }
        self.assertEqual(len(rules.check_guards(tokens, [guard])), 1)

    def test_neutral_must_carry_a_tint(self):
        tokens = _tokens({"palette.neutral.500": "#777777"})
        guard = {"scope": ["palette.neutral."], "min_chroma": 0.005}
        self.assertEqual(len(rules.check_guards(tokens, [guard])), 1)

    def test_scope_suffix_limits_the_guard_to_matching_paths(self):
        tokens = _tokens({"color.warning.subtle": "#fef3ea", "color.warning.fg": "#fef3ea"})
        guard = {
            "scope": ["color."],
            "scope_suffix": [".subtle"],
            "forbidden_hue_ranges": [{"name": "cream", "from": 30, "to": 115, "min_chroma": 0.0, "max_chroma": 0.03}],
        }
        problems = rules.check_guards(tokens, [guard])
        self.assertEqual(len(problems), 1)
        self.assertIn("color.warning.subtle", problems[0])

    def test_max_chroma_lets_a_clear_signal_tint_pass(self):
        tokens = _tokens({"color.warning.subtle": "#fee6d4", "color.attention.subtle": "#fef5aa"})
        guard = {
            "scope": ["color."],
            "scope_suffix": [".subtle"],
            "forbidden_hue_ranges": [{"name": "cream", "from": 30, "to": 115, "min_chroma": 0.0, "max_chroma": 0.03}],
        }
        self.assertEqual(rules.check_guards(tokens, [guard]), [])

    def test_compliant_palette_has_no_problems(self):
        tokens = _tokens({"palette.brand.500": "#0b7a75"})
        guard = {
            "scope": ["palette.brand."],
            "forbidden_colors": [{"name": "ref", "hex": "#5865f2", "min_delta_e_ok": 0.08}],
            "forbidden_hue_ranges": [{"name": "warm", "from": 25, "to": 80, "min_chroma": 0.04}],
            "min_chroma": 0.005,
        }
        self.assertEqual(rules.check_guards(tokens, [guard]), [])


if __name__ == "__main__":
    unittest.main()
