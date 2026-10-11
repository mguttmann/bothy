# SPDX-License-Identifier: Apache-2.0
"""Tests for the model split and the CSS and Dart emitters."""

import json
import tempfile
import unittest
from pathlib import Path

from tokenkit import dtcg, emit_css, emit_dart, emit_docs, emit_mermaid, model

REPO_RESOLVER = Path(__file__).resolve().parents[3] / "design" / "tokens" / "theme.resolver.json"


def _write(path: Path, data) -> None:
    path.write_text(json.dumps(data), encoding="utf-8")


def _color(hex_value, alpha=None):
    hex_value = hex_value.lstrip("#")
    value = {"colorSpace": "srgb", "components": [int(hex_value[i : i + 2], 16) / 255 for i in (0, 2, 4)]}
    if alpha is not None:
        value["alpha"] = alpha
    return {"$value": value}


class ModelTest(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        d = Path(self._tmp.name)
        _write(
            d / "base.tokens.json",
            {
                "palette": {"$type": "color", "a": _color("#ffffff"), "b": _color("#000000")},
                "space": {"$type": "dimension", "4": {"$value": {"value": 4, "unit": "px"}}},
                "elevation": {
                    "$type": "shadow",
                    "menu": {
                        "$value": {
                            "color": "{color.shadow}",
                            "offsetX": {"value": 0, "unit": "px"},
                            "offsetY": {"value": 2, "unit": "px"},
                            "blur": {"value": 6, "unit": "px"},
                            "spread": {"value": 0, "unit": "px"},
                        }
                    },
                },
            },
        )
        _write(
            d / "light.tokens.json",
            {"color": {"$type": "color", "bg": {"$value": "{palette.a}"}, "shadow": _color("#000000", 0.5)}},
        )
        _write(
            d / "dark.tokens.json",
            {"color": {"$type": "color", "bg": {"$value": "{palette.b}"}, "shadow": _color("#000000", 0.8)}},
        )
        _write(
            d / "r.resolver.json",
            {
                "version": "2025.10",
                "sets": {"base": {"sources": [{"$ref": "base.tokens.json"}]}},
                "modifiers": {
                    "theme": {
                        "contexts": {"light": [{"$ref": "light.tokens.json"}], "dark": [{"$ref": "dark.tokens.json"}]},
                        "default": "light",
                    }
                },
                "resolutionOrder": [{"$ref": "#/sets/base"}, {"$ref": "#/modifiers/theme"}],
            },
        )
        self.model = model.build(d / "r.resolver.json")

    def tearDown(self):
        self._tmp.cleanup()

    def test_theme_tokens_are_split_from_base(self):
        self.assertIn("space.4", self.model.base)
        self.assertNotIn("color.bg", self.model.base)
        self.assertEqual(self.model.contexts["theme"]["dark"]["color.bg"].hex, "#000000")

    def test_base_token_that_aliases_a_theme_token_becomes_theme_dependent(self):
        self.assertNotIn("elevation.menu", self.model.base)
        self.assertIn("elevation.menu", self.model.contexts["theme"]["light"])

    def test_token_reference_lists_dimension_values_and_skips_colours(self):
        text = emit_docs.tokens_md(self.model, "HEADER")
        self.assertIn("| `space.4` | dimension | 4px |", text)
        self.assertNotIn("`palette.a`", text)


class CssTest(unittest.TestCase):
    def test_variable_name_drops_root_segment(self):
        self.assertEqual(emit_css.var_name("color.accent.$root"), "--ds-color-accent")
        self.assertEqual(emit_css.var_name("color.surface.sidebar-hover"), "--ds-color-surface-sidebar-hover")

    def test_font_family_quotes_named_families_only(self):
        self.assertEqual(
            emit_css.font_family(["Atkinson Hyperlegible Next", "system-ui", "sans-serif"]),
            '"Atkinson Hyperlegible Next", system-ui, sans-serif',
        )

    def test_translucent_colour_uses_rgb_with_alpha(self):
        self.assertEqual(emit_css.color_value("#08191c", 0.48), "rgb(8 25 28 / 0.48)")

    def test_cubic_bezier(self):
        self.assertEqual(emit_css.cubic([0.2, 0, 0, 1]), "cubic-bezier(0.2, 0, 0, 1)")


class DartTest(unittest.TestCase):
    def test_identifier_is_lower_camel_case_without_group_prefix(self):
        self.assertEqual(emit_dart.identifier("color.surface.sidebar-hover", drop="color"), "surfaceSidebarHover")
        self.assertEqual(emit_dart.identifier("palette.spruce.50", drop="palette"), "spruce50")

    def test_opaque_colour_literal(self):
        self.assertEqual(emit_dart.color_literal("#0c5450", 1.0), "Color(0xFF0C5450)")

    def test_translucent_colour_literal(self):
        self.assertEqual(emit_dart.color_literal("#08191c", 0.48), "Color(0x7A08191C)")


class MermaidTest(unittest.TestCase):
    def setUp(self):
        self.model = model.build(REPO_RESOLVER)

    def test_every_variable_is_the_light_theme_colour_of_its_token(self):
        cfg = emit_mermaid.config(self.model)
        self.assertEqual(cfg["theme"], "base")
        light = self.model.full["theme"]["light"]
        for name, path in emit_mermaid.THEME_VARIABLES.items():
            self.assertEqual(cfg["themeVariables"][name], light[path].hex, name)

    def test_font_is_the_sans_family_of_the_design_system(self):
        font = emit_mermaid.config(self.model)["themeVariables"]["fontFamily"]
        self.assertTrue(font.startswith('"Atkinson Hyperlegible Next", '), font)

    def test_site_css_has_one_block_per_colour_scheme(self):
        css = emit_mermaid.material_css(self.model, "HEADER")
        self.assertIn('[data-md-color-scheme="default"] {', css)
        self.assertIn('[data-md-color-scheme="slate"] {', css)
        dark = self.model.full["theme"]["dark"]["color.accent.bg"].hex
        self.assertIn(f"--md-primary-fg-color: {dark};", css.split("slate")[1])

    def test_unknown_token_is_an_error(self):
        original = dict(emit_mermaid.THEME_VARIABLES)
        emit_mermaid.THEME_VARIABLES["lineColor"] = "color.does.not-exist"
        try:
            with self.assertRaises(dtcg.TokenError):
                emit_mermaid.config(self.model)
        finally:
            emit_mermaid.THEME_VARIABLES.clear()
            emit_mermaid.THEME_VARIABLES.update(original)


if __name__ == "__main__":
    unittest.main()
